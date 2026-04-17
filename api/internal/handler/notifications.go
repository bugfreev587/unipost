package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/mail"
)

// NotificationHandler exposes the user-facing settings API for the
// notification system (migration 040). All routes are account-scoped
// by the authenticated user — no workspace context needed.
type NotificationHandler struct {
	queries    *db.Queries
	mailer     mail.Mailer
	httpClient *http.Client
	appBaseURL string
}

func NewNotificationHandler(queries *db.Queries, mailer mail.Mailer, appBaseURL string) *NotificationHandler {
	if appBaseURL == "" {
		appBaseURL = "https://app.unipost.dev"
	}
	return &NotificationHandler{
		queries:    queries,
		mailer:     mailer,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		appBaseURL: strings.TrimRight(appBaseURL, "/"),
	}
}

// Static catalog of events the settings UI knows how to show. Kept in
// sync with worker/notification.go renderEmail — if an event doesn't
// have a template here, don't advertise it in the UI.
//
// New entries need a default_channel kind ("email" for now) and a
// default_on flag that drives the auto-provisioning path.
type eventDescriptor struct {
	Type        string `json:"event_type"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // "critical" | "high" | "medium" | "low"
	DefaultOn   bool   `json:"default_on"`
}

// SupportedNotificationEvents is the source of truth for "which events
// can a user subscribe to". Also consumed by the default-provisioning
// path in Bootstrap to seed subscriptions on user creation.
var SupportedNotificationEvents = []eventDescriptor{
	{Type: "post.failed", Label: "Post failed to publish", Description: "Get notified when a post can't be delivered to a platform.", Severity: "high", DefaultOn: true},
	{Type: "account.disconnected", Label: "Account disconnected", Description: "A connected social account lost its token and can't post until reconnected.", Severity: "high", DefaultOn: true},
	{Type: "billing.usage_80pct", Label: "Usage at 80% of plan limit", Description: "Heads-up before you hit the monthly post cap.", Severity: "medium", DefaultOn: true},
	{Type: "billing.payment_failed", Label: "Payment failed", Description: "Your Stripe subscription charge didn't go through.", Severity: "critical", DefaultOn: true},
}

// ── Channels ─────────────────────────────────────────────────────────

type channelResponse struct {
	ID         string                 `json:"id"`
	Kind       string                 `json:"kind"`
	Label      string                 `json:"label,omitempty"`
	Config     map[string]any         `json:"config"`
	Verified   bool                   `json:"verified"`
	CreatedAt  string                 `json:"created_at"`
}

func toChannelResponse(c db.NotificationChannel) channelResponse {
	var cfg map[string]any
	_ = json.Unmarshal(c.Config, &cfg)
	if cfg == nil {
		cfg = map[string]any{}
	}
	return channelResponse{
		ID:        c.ID,
		Kind:      c.Kind,
		Label:     c.Label.String,
		Config:    cfg,
		Verified:  c.VerifiedAt.Valid,
		CreatedAt: c.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// ListChannels — GET /v1/me/notifications/channels
func (h *NotificationHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	rows, err := h.queries.ListNotificationChannelsByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load channels")
		return
	}
	out := make([]channelResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, toChannelResponse(c))
	}
	writeSuccess(w, out)
}

// CreateChannel — POST /v1/me/notifications/channels
//
// CreateChannel — POST /v1/me/notifications/channels
//
// Accepts three channel kinds:
//   - email:           {kind, address, label?}. Auto-verified if address
//                      matches Clerk signup email; otherwise unverified.
//   - slack_webhook:   {kind, url, label?}. Auto-verified (URL ownership
//                      is implicit — only workspace admin has it).
//   - discord_webhook: {kind, url, label?}. Same as Slack.
func (h *NotificationHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	var body struct {
		Kind    string `json:"kind"`
		Address string `json:"address"` // email
		URL     string `json:"url"`     // slack_webhook, discord_webhook
		Label   string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	body.Kind = strings.ToLower(strings.TrimSpace(body.Kind))
	body.Address = strings.TrimSpace(body.Address)
	body.URL = strings.TrimSpace(body.URL)

	var cfgBytes []byte
	var verified pgtype.Timestamptz

	switch body.Kind {
	case "email":
		if body.Address == "" || !strings.Contains(body.Address, "@") {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "A valid email address is required")
			return
		}
		cfgBytes, _ = json.Marshal(map[string]string{"address": body.Address})
		if user, err := h.queries.GetUser(r.Context(), userID); err == nil {
			if strings.EqualFold(user.Email, body.Address) {
				verified = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
			}
		}

	case "slack_webhook":
		if body.URL == "" || !strings.HasPrefix(body.URL, "https://hooks.slack.com/") {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "A valid Slack webhook URL is required (https://hooks.slack.com/...)")
			return
		}
		cfgBytes, _ = json.Marshal(map[string]string{"url": body.URL})
		verified = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}

	case "discord_webhook":
		if body.URL == "" || !strings.HasPrefix(body.URL, "https://discord.com/api/webhooks/") {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "A valid Discord webhook URL is required (https://discord.com/api/webhooks/...)")
			return
		}
		cfgBytes, _ = json.Marshal(map[string]string{"url": body.URL})
		verified = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}

	default:
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Supported kinds: email, slack_webhook, discord_webhook")
		return
	}

	created, err := h.queries.CreateNotificationChannel(r.Context(), db.CreateNotificationChannelParams{
		UserID:      userID,
		WorkspaceID: pgtype.Text{},
		Kind:        body.Kind,
		Config:      cfgBytes,
		Label:       pgtype.Text{String: body.Label, Valid: body.Label != ""},
		VerifiedAt:  verified,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create channel")
		return
	}
	writeSuccess(w, toChannelResponse(created))
}

// DeleteChannel — DELETE /v1/me/notifications/channels/{id}
func (h *NotificationHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Channel id required")
		return
	}
	if err := h.queries.SoftDeleteNotificationChannel(r.Context(), db.SoftDeleteNotificationChannelParams{
		ID:     id,
		UserID: userID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete channel")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TestChannel — POST /v1/me/notifications/channels/{id}/test
// Sends a one-off test message through the actual provider path.
func (h *NotificationHandler) TestChannel(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Channel id required")
		return
	}

	channel, err := h.queries.GetNotificationChannel(r.Context(), db.GetNotificationChannelParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Channel not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load channel")
		return
	}

	if err := h.sendTestChannel(r.Context(), channel); err != nil {
		slog.Warn("notifications: test send failed", "channel_id", channel.ID, "kind", channel.Kind, "error", err)
		writeError(w, http.StatusBadGateway, "DELIVERY_FAILED", err.Error())
		return
	}

	writeSuccess(w, map[string]any{
		"id":      channel.ID,
		"kind":    channel.Kind,
		"message": testChannelSuccessMessage(channel.Kind),
	})
}

// ── Subscriptions ────────────────────────────────────────────────────

type subscriptionResponse struct {
	ID        string `json:"id"`
	EventType string `json:"event_type"`
	ChannelID string `json:"channel_id"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

// ListSubscriptions — GET /v1/me/notifications/subscriptions
func (h *NotificationHandler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	rows, err := h.queries.ListNotificationSubscriptionsByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load subscriptions")
		return
	}
	out := make([]subscriptionResponse, 0, len(rows))
	for _, s := range rows {
		out = append(out, subscriptionResponse{
			ID:        s.ID,
			EventType: s.EventType,
			ChannelID: s.ChannelID,
			Enabled:   s.Enabled,
			CreatedAt: s.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	writeSuccess(w, out)
}

// UpsertSubscription — PUT /v1/me/notifications/subscriptions
//
// Body: {event_type, channel_id, enabled}. Creates the row if missing
// or updates enabled/filter if present (ON CONFLICT in the query).
// Used by the settings matrix UI — each checkbox flip hits this endpoint.
func (h *NotificationHandler) UpsertSubscription(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	var body struct {
		EventType string `json:"event_type"`
		ChannelID string `json:"channel_id"`
		Enabled   bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if !isSupportedEvent(body.EventType) {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Unsupported event type: "+body.EventType)
		return
	}
	if body.ChannelID == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "channel_id required")
		return
	}

	// Validate the channel belongs to the user — defense-in-depth even
	// though the FK prevents cross-user inserts (a malicious client
	// would get a 500 on FK violation without this).
	if _, err := h.queries.GetNotificationChannel(r.Context(), db.GetNotificationChannelParams{
		ID: body.ChannelID, UserID: userID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Channel not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to verify channel")
		return
	}

	sub, err := h.queries.CreateNotificationSubscription(r.Context(), db.CreateNotificationSubscriptionParams{
		UserID:      userID,
		WorkspaceID: pgtype.Text{}, // account-level
		EventType:   body.EventType,
		ChannelID:   body.ChannelID,
		Enabled:     body.Enabled,
		Filter:      nil,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to save subscription")
		return
	}
	writeSuccess(w, subscriptionResponse{
		ID:        sub.ID,
		EventType: sub.EventType,
		ChannelID: sub.ChannelID,
		Enabled:   sub.Enabled,
		CreatedAt: sub.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// DeleteSubscription — DELETE /v1/me/notifications/subscriptions/{id}
func (h *NotificationHandler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Subscription id required")
		return
	}
	if err := h.queries.DeleteNotificationSubscription(r.Context(), db.DeleteNotificationSubscriptionParams{
		ID: id, UserID: userID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete subscription")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Events catalog ───────────────────────────────────────────────────

// ListEvents — GET /v1/me/notifications/events
// Returns the static catalog so the settings UI can label each row.
func (h *NotificationHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	writeSuccess(w, SupportedNotificationEvents)
}

func isSupportedEvent(t string) bool {
	for _, e := range SupportedNotificationEvents {
		if e.Type == t {
			return true
		}
	}
	return false
}

func (h *NotificationHandler) sendTestChannel(ctx context.Context, c db.NotificationChannel) error {
	switch c.Kind {
	case "email":
		var cfg struct {
			Address string `json:"address"`
		}
		if err := json.Unmarshal(c.Config, &cfg); err != nil || cfg.Address == "" {
			return fmt.Errorf("invalid email channel config")
		}
		return h.mailer.Send(ctx, mail.Message{
			To:      cfg.Address,
			Subject: "[UniPost] Notification channel test",
			HTML: fmt.Sprintf(`<p>This is a test notification from UniPost.</p>
<p>Your email channel is connected and can receive alerts.</p>
<p><a href="%s/settings/notifications">Manage notification settings →</a></p>`, h.appBaseURL),
			Text: fmt.Sprintf("This is a test notification from UniPost.\n\nYour email channel is connected and can receive alerts.\n\nManage settings: %s/settings/notifications\n", h.appBaseURL),
		})
	case "slack_webhook":
		var cfg struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(c.Config, &cfg); err != nil || cfg.URL == "" {
			return fmt.Errorf("invalid slack_webhook channel config")
		}
		body, _ := json.Marshal(map[string]string{
			"text": fmt.Sprintf("UniPost test notification\nThis Slack channel is connected and ready to receive alerts.\n%s/settings/notifications", h.appBaseURL),
		})
		return h.postTestWebhook(ctx, cfg.URL, body)
	case "discord_webhook":
		var cfg struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(c.Config, &cfg); err != nil || cfg.URL == "" {
			return fmt.Errorf("invalid discord_webhook channel config")
		}
		body, _ := json.Marshal(map[string]any{
			"content":  fmt.Sprintf("UniPost test notification\nThis Discord channel is connected and ready to receive alerts.\n%s/settings/notifications", h.appBaseURL),
			"username": "UniPost",
		})
		return h.postTestWebhook(ctx, cfg.URL, body)
	default:
		return fmt.Errorf("unsupported channel kind: %s", c.Kind)
	}
}

func (h *NotificationHandler) postTestWebhook(ctx context.Context, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: http: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("webhook: %d", resp.StatusCode)
}

func testChannelSuccessMessage(kind string) string {
	switch kind {
	case "email":
		return "Test email sent."
	case "slack_webhook":
		return "Test Slack message sent."
	case "discord_webhook":
		return "Test Discord message sent."
	default:
		return "Test notification sent."
	}
}
