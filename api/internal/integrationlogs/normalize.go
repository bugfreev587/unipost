package integrationlogs

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type Level string
type Status string
type Category string
type Source string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

const (
	StatusSuccess Status = "success"
	StatusWarning Status = "warning"
	StatusError   Status = "error"
)

const (
	CategoryPublishing Category = "publishing"
	CategoryAPIRequest Category = "api_request"
	CategoryOAuth      Category = "oauth"
	CategoryWebhook    Category = "webhook"
	CategorySystem     Category = "system"
)

const (
	SourceAPI       Source = "api"
	SourceDashboard Source = "dashboard"
	SourceWorker    Source = "worker"
	SourceWebhook   Source = "webhook"
	SourceOAuth     Source = "oauth"
)

const (
	ActionPostValidateStarted          = "post.validate.started"
	ActionPostValidateFailed           = "post.validate.failed"
	ActionPostPublishQueued            = "post.publish.queued"
	ActionPostPublishStarted           = "post.publish.started"
	ActionPostPublishFailedPreDispatch = "post.publish.failed_pre_dispatch"
	ActionPostPublishPlatformStarted   = "post.publish.platform_started"
	ActionPostPublishPlatformSucceeded = "post.publish.platform_succeeded"
	ActionPostPublishPlatformFailed    = "post.publish.platform_failed"
	ActionPostPublishCompleted         = "post.publish.completed"

	ActionAPIRequestSucceeded        = "api.request.succeeded"
	ActionAPIRequestFailed           = "api.request.failed"
	ActionAPIRequestValidationFailed = "api.request.validation_failed"
	ActionAPIRequestRateLimited      = "api.request.rate_limited"

	ActionAccountConnectSessionCreated = "account.connect.session_created"
	ActionAccountConnectCallbackOK     = "account.connect.callback_succeeded"
	ActionAccountConnectCallbackFailed = "account.connect.callback_failed"
	ActionAccountRefreshTokenFailed    = "account.refresh_token_failed"

	ActionWebhookDeliveryStarted   = "webhook.delivery.started"
	ActionWebhookDeliverySucceeded = "webhook.delivery.succeeded"
	ActionWebhookDeliveryFailed    = "webhook.delivery.failed"
	ActionWebhookRetryScheduled    = "webhook.delivery.retry_scheduled"

	ActionJobStarted   = "job.started"
	ActionJobSucceeded = "job.succeeded"
	ActionJobFailed    = "job.failed"
)

type Event struct {
	WorkspaceID string
	TS          time.Time

	Level    Level
	Status   Status
	Category Category
	Action   string
	Source   Source
	Message  string

	RequestID string
	TraceID   string

	ActorUserID   string
	ActorAPIKeyID string

	ProfileID       string
	SocialAccountID string
	PostID          string
	PlatformPostID  string
	Platform        string

	Endpoint         string
	Method           string
	HTTPStatusCode   *int
	RemoteStatusCode *int
	DurationMS       *int

	ErrorCode string

	Metadata        any
	RequestPayload  any
	ResponsePayload any
}

func Normalize(e Event) db.InsertIntegrationLogParams {
	ts := e.TS
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	level := string(e.Level)
	if level == "" {
		level = string(LevelInfo)
	}

	status := string(e.Status)
	if status == "" {
		status = string(StatusSuccess)
	}

	category := string(e.Category)
	if category == "" {
		category = string(CategorySystem)
	}

	source := string(e.Source)
	if source == "" {
		source = string(SourceWorker)
	}

	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = strings.TrimSpace(e.Action)
	}
	if message == "" {
		message = "integration log event"
	}

	return db.InsertIntegrationLogParams{
		WorkspaceID:      e.WorkspaceID,
		Ts:               pgtype.Timestamptz{Time: ts, Valid: true},
		Level:            level,
		Status:           status,
		Category:         category,
		Action:           strings.TrimSpace(e.Action),
		Source:           source,
		Message:          message,
		RequestID:        pgText(strings.TrimSpace(e.RequestID)),
		TraceID:          pgText(strings.TrimSpace(e.TraceID)),
		ActorUserID:      pgText(strings.TrimSpace(e.ActorUserID)),
		ActorApiKeyID:    pgText(strings.TrimSpace(e.ActorAPIKeyID)),
		ProfileID:        pgText(strings.TrimSpace(e.ProfileID)),
		SocialAccountID:  pgText(strings.TrimSpace(e.SocialAccountID)),
		PostID:           pgText(strings.TrimSpace(e.PostID)),
		PlatformPostID:   pgText(strings.TrimSpace(e.PlatformPostID)),
		Platform:         pgText(normalizeToken(e.Platform)),
		Endpoint:         pgText(strings.TrimSpace(e.Endpoint)),
		Method:           pgText(strings.ToUpper(strings.TrimSpace(e.Method))),
		HTTPStatusCode:   pgInt4(e.HTTPStatusCode),
		RemoteStatusCode: pgInt4(e.RemoteStatusCode),
		DurationMs:       pgInt4(e.DurationMS),
		ErrorCode:        pgText(normalizeToken(e.ErrorCode)),
		Metadata:         ensureJSONObject(e.Metadata),
		RequestPayload:   RedactJSON(e.RequestPayload),
		ResponsePayload:  RedactJSON(e.ResponsePayload),
	}
}

func pgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func pgInt4(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

func normalizeToken(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	v = strings.ReplaceAll(v, " ", "_")
	return v
}

func ensureJSONObject(v any) []byte {
	if v == nil {
		return []byte(`{}`)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return []byte(`{}`)
	}
	if len(out) == 0 {
		return []byte(`{}`)
	}
	return out
}
