package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/mail"
)

// ── Dispatcher ───────────────────────────────────────────────────────

// NotificationDispatcher implements events.EventBus by fanning events
// out to notification_deliveries rows. One row per matching
// subscription. Best-effort: never blocks, panics are recovered, all
// errors are logged.
//
// This is the parallel of WebhookDeliveryWorker's role on the webhook
// side. Compose both under events.MultiBus so one handler.Publish
// feeds both systems.
type NotificationDispatcher struct {
	queries *db.Queries
}

func NewNotificationDispatcher(queries *db.Queries) *NotificationDispatcher {
	return &NotificationDispatcher{queries: queries}
}

// Publish satisfies events.EventBus.
func (d *NotificationDispatcher) Publish(ctx context.Context, workspaceID, event string, data any) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("notifications: publish panic recovered", "event", event, "workspace_id", workspaceID, "panic", r)
		}
	}()
	if err := d.fanout(ctx, workspaceID, event, data); err != nil {
		slog.Warn("notifications: fanout failed", "event", event, "workspace_id", workspaceID, "error", err)
	}
}

func (d *NotificationDispatcher) fanout(ctx context.Context, workspaceID, event string, data any) error {
	// Resolve subscriptions that should hear this event. The query
	// already joins workspaces to validate ownership, so account-level
	// (workspace_id NULL) subs only match workspaces the user owns.
	targets, err := d.queries.ResolveNotificationTargets(ctx, db.ResolveNotificationTargetsParams{
		EventType: event,
		ID:        workspaceID,
	})
	if err != nil {
		return fmt.Errorf("resolve targets: %w", err)
	}
	if len(targets) == 0 {
		return nil
	}

	// One event_id per Publish call — identical across all fanout rows
	// so a receiver can correlate, and the UNIQUE (event_id, channel_id)
	// constraint makes concurrent dispatchers idempotent.
	eventID := uuid.NewString()
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	for _, t := range targets {
		if err := d.queries.CreateNotificationDelivery(ctx, db.CreateNotificationDeliveryParams{
			SubscriptionID: t.SubscriptionID,
			ChannelID:      t.ChannelID,
			EventType:      event,
			EventID:        eventID,
			Payload:        payload,
		}); err != nil {
			slog.Error("notifications: create delivery failed",
				"subscription_id", t.SubscriptionID,
				"event", event,
				"error", err)
			// Keep going — don't let one bad row block the rest.
		}
	}
	return nil
}

// ── Delivery worker ──────────────────────────────────────────────────

// NotificationDeliveryWorker polls notification_deliveries and routes
// pending rows to their channel implementation. Mirrors
// WebhookDeliveryWorker's retry schedule so operators can reason about
// both systems the same way.
type NotificationDeliveryWorker struct {
	queries *db.Queries
	mailer  mail.Mailer
	// appBaseURL is the user-facing dashboard URL (e.g.
	// https://app.unipost.dev) used to build deep links in email
	// bodies. Falls back to a generic marker in local dev.
	appBaseURL string
}

func NewNotificationDeliveryWorker(queries *db.Queries, mailer mail.Mailer, appBaseURL string) *NotificationDeliveryWorker {
	if appBaseURL == "" {
		appBaseURL = "https://app.unipost.dev"
	}
	return &NotificationDeliveryWorker{
		queries:    queries,
		mailer:     mailer,
		appBaseURL: strings.TrimRight(appBaseURL, "/"),
	}
}

func (w *NotificationDeliveryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	slog.Info("notification delivery worker started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("notification delivery worker stopped")
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

// retryDelays mirrors worker/webhook.go so operators see consistent
// retry behavior across both systems. After 3 attempts we mark dead.
var notifRetryDelays = []time.Duration{1 * time.Minute, 5 * time.Minute, 30 * time.Minute}

func (w *NotificationDeliveryWorker) tick(ctx context.Context) {
	deliveries, err := w.queries.GetPendingNotificationDeliveries(ctx)
	if err != nil {
		slog.Error("notifications: get pending failed", "error", err)
		return
	}
	for _, d := range deliveries {
		w.deliverOne(ctx, d)
	}
}

func (w *NotificationDeliveryWorker) deliverOne(ctx context.Context, d db.GetPendingNotificationDeliveriesRow) {
	sendErr := w.dispatchByKind(ctx, d)
	if sendErr == nil {
		if err := w.queries.MarkNotificationDeliverySent(ctx, d.ID); err != nil {
			slog.Error("notifications: mark sent failed", "delivery_id", d.ID, "error", err)
		}
		return
	}

	attempts := int(d.Attempts) + 1
	slog.Warn("notifications: delivery failed",
		"delivery_id", d.ID,
		"event", d.EventType,
		"kind", d.ChannelKind,
		"attempts", attempts,
		"error", sendErr)

	if attempts >= len(notifRetryDelays)+1 {
		if err := w.queries.MarkNotificationDeliveryDead(ctx, db.MarkNotificationDeliveryDeadParams{
			ID:        d.ID,
			LastError: pgtype.Text{String: truncate(sendErr.Error(), 500), Valid: true},
		}); err != nil {
			slog.Error("notifications: mark dead failed", "delivery_id", d.ID, "error", err)
		}
		return
	}

	nextRetry := time.Now().Add(notifRetryDelays[attempts-1])
	if err := w.queries.ScheduleNotificationDeliveryRetry(ctx, db.ScheduleNotificationDeliveryRetryParams{
		ID:          d.ID,
		NextRetryAt: pgtype.Timestamptz{Time: nextRetry, Valid: true},
		LastError:   pgtype.Text{String: truncate(sendErr.Error(), 500), Valid: true},
	}); err != nil {
		slog.Error("notifications: schedule retry failed", "delivery_id", d.ID, "error", err)
	}
}

func (w *NotificationDeliveryWorker) dispatchByKind(ctx context.Context, d db.GetPendingNotificationDeliveriesRow) error {
	switch d.ChannelKind {
	case "email":
		return w.sendEmail(ctx, d)
	case "slack_webhook", "sms", "in_app":
		// Modeled in the schema; not wired in MVP. Treat as no-op so
		// the row moves to sent and doesn't pile up on every tick.
		slog.Info("notifications: skipping unwired channel kind", "kind", d.ChannelKind, "delivery_id", d.ID)
		return nil
	default:
		return fmt.Errorf("unknown channel kind: %s", d.ChannelKind)
	}
}

// ── Email rendering ──────────────────────────────────────────────────

func (w *NotificationDeliveryWorker) sendEmail(ctx context.Context, d db.GetPendingNotificationDeliveriesRow) error {
	var cfg struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(d.ChannelConfig, &cfg); err != nil || cfg.Address == "" {
		return fmt.Errorf("invalid email channel config: %w", err)
	}

	msg := renderEmail(d.EventType, d.Payload, w.appBaseURL)
	msg.To = cfg.Address
	return w.mailer.Send(ctx, msg)
}

// renderEmail turns an event into a Message. One small template per
// supported event. New events add a case here plus a subscription
// created in bootstrap.
func renderEmail(eventType string, payloadJSON []byte, appBaseURL string) mail.Message {
	var payload map[string]any
	_ = json.Unmarshal(payloadJSON, &payload)
	getStr := func(k string) string {
		if v, ok := payload[k].(string); ok {
			return v
		}
		return ""
	}

	switch eventType {
	case "post.failed":
		caption := truncate(getStr("caption"), 80)
		if caption == "" {
			caption = "(no caption)"
		}
		return mail.Message{
			Subject: "[UniPost] A post failed to publish",
			HTML: fmt.Sprintf(`<p>One of your scheduled posts didn't go out.</p>
<p><strong>Caption:</strong> %s</p>
<p>Check the post detail for the error reason and retry options.</p>
<p><a href="%s">Open dashboard →</a></p>`, htmlEscape(caption), appBaseURL),
			Text: fmt.Sprintf("A UniPost post failed to publish.\n\nCaption: %s\n\nOpen the dashboard to see the error and retry: %s\n", caption, appBaseURL),
		}

	case "account.disconnected":
		name := getStr("account_name")
		plat := getStr("platform")
		if name == "" {
			name = "A social account"
		}
		return mail.Message{
			Subject: fmt.Sprintf("[UniPost] %s was disconnected", name),
			HTML: fmt.Sprintf(`<p><strong>%s</strong> (%s) is no longer connected to UniPost. New posts to this account will fail until it's reconnected.</p>
<p><a href="%s">Reconnect in dashboard →</a></p>`, htmlEscape(name), htmlEscape(plat), appBaseURL),
			Text: fmt.Sprintf("%s (%s) was disconnected from UniPost. New posts will fail until it's reconnected.\n\n%s\n", name, plat, appBaseURL),
		}

	case "billing.usage_80pct":
		return mail.Message{
			Subject: "[UniPost] You've used 80% of this month's post quota",
			HTML: fmt.Sprintf(`<p>Heads up — you've used 80%% of your monthly post quota. If you run out, new posts will be rejected until the quota resets or you upgrade.</p>
<p><a href="%s/settings/billing">Review plan & usage →</a></p>`, appBaseURL),
			Text: fmt.Sprintf("You've used 80%% of your UniPost monthly post quota.\n\nReview plan: %s/settings/billing\n", appBaseURL),
		}

	case "billing.payment_failed":
		return mail.Message{
			Subject: "[UniPost] Your subscription payment failed",
			HTML: fmt.Sprintf(`<p>Your latest UniPost subscription payment failed. We'll retry automatically, but you may want to update your card to avoid any service interruption.</p>
<p><a href="%s/settings/billing">Update payment method →</a></p>`, appBaseURL),
			Text: fmt.Sprintf("Your UniPost subscription payment failed. Update your card to avoid interruption:\n\n%s/settings/billing\n", appBaseURL),
		}
	}

	// Unknown event type — fall back to a generic email so the row
	// doesn't pile up as a retry. Log loudly so we notice.
	slog.Warn("notifications: no template for event", "event", eventType)
	return mail.Message{
		Subject: fmt.Sprintf("[UniPost] %s", eventType),
		Text:    fmt.Sprintf("UniPost event: %s\n\n%s\n", eventType, appBaseURL),
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Minimal HTML-escaping for values interpolated into template strings.
// We control the templates themselves so this just protects against
// user-supplied captions / account names breaking the markup.
var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&#39;",
)

func htmlEscape(s string) string { return htmlEscaper.Replace(s) }
