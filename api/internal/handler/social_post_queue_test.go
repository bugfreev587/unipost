package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
)

func TestRetryDeliveryJobNowMarksDeprecated(t *testing.T) {
	h := &SocialPostHandler{}
	req := httptest.NewRequest(http.MethodPost, "/v1/post-delivery-jobs//retry-now", nil)
	rr := httptest.NewRecorder()

	h.RetryDeliveryJobNow(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when job id is missing", rr.Code)
	}
	if got := rr.Header().Get("Deprecation"); got != "true" {
		t.Fatalf("Deprecation header = %q, want true", got)
	}
	if got := rr.Header().Get("Sunset"); got == "" {
		t.Fatal("expected Sunset header on legacy retry-now alias")
	}
	if got := rr.Header().Get("Link"); got == "" {
		t.Fatal("expected Link header pointing to canonical retry route")
	}
}

func TestWorkerPublishingEventSourceIsWorker(t *testing.T) {
	event := workerPublishingEvent(integrationlogs.Event{
		Action: integrationlogs.ActionPostPublishPlatformFailed,
	})

	if event.Source != integrationlogs.SourceWorker {
		t.Fatalf("source = %q, want %q", event.Source, integrationlogs.SourceWorker)
	}
}

func TestResolvePublishingEventSourcePreservesExplicitSource(t *testing.T) {
	got := resolvePublishingEventSource(context.Background(), integrationlogs.Event{
		Source: integrationlogs.SourceWorker,
	})

	if got != integrationlogs.SourceWorker {
		t.Fatalf("source = %q, want %q", got, integrationlogs.SourceWorker)
	}
}

func TestResolvePublishingEventSourceUsesAPIWhenAPIKeyPresent(t *testing.T) {
	ctx := context.WithValue(context.Background(), auth.APIKeyIDKey, "api_key_123")

	got := resolvePublishingEventSource(ctx, integrationlogs.Event{})

	if got != integrationlogs.SourceAPI {
		t.Fatalf("source = %q, want %q", got, integrationlogs.SourceAPI)
	}
}

func TestResolvePublishingEventSourceDefaultsToDashboard(t *testing.T) {
	got := resolvePublishingEventSource(context.Background(), integrationlogs.Event{})

	if got != integrationlogs.SourceDashboard {
		t.Fatalf("source = %q, want %q", got, integrationlogs.SourceDashboard)
	}
}

func TestPostFailureShouldMarkReconnectRequired(t *testing.T) {
	arg := db.CreatePostFailureParams{
		ErrorCode:       "account_reconnect_required",
		SocialAccountID: pgtype.Text{String: "acc_threads", Valid: true},
	}
	if !postFailureShouldMarkReconnectRequired(arg) {
		t.Fatal("expected account_reconnect_required with account id to mark reconnect required")
	}

	arg.ErrorCode = "missing_permission"
	if postFailureShouldMarkReconnectRequired(arg) {
		t.Fatal("missing_permission should not mark reconnect required")
	}

	arg.ErrorCode = "account_reconnect_required"
	arg.SocialAccountID = pgtype.Text{}
	if postFailureShouldMarkReconnectRequired(arg) {
		t.Fatal("missing account id should not mark reconnect required")
	}
}
