package handler

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/postfailures"
)

func TestBuildLoopsPlanChangedEvent(t *testing.T) {
	event := buildLoopsPlanChangedEvent(
		db.User{ID: "user_123", Email: "alex@example.com", Name: pgtype.Text{String: "Alex Smith", Valid: true}},
		db.Workspace{ID: "ws_123", Name: "Alex Workspace"},
		"free",
		"basic",
		"upgrade",
		"plan_changed:checkout:cs_123",
		"https://app.unipost.dev",
	)

	if event.EventName != "plan_changed" {
		t.Fatalf("event name = %q, want plan_changed", event.EventName)
	}
	if event.UserID != "user_123" || event.Email != "alex@example.com" {
		t.Fatalf("user = %q/%q", event.UserID, event.Email)
	}
	if event.PlanID != "basic" {
		t.Fatalf("plan id = %q, want basic", event.PlanID)
	}
	assertLifecycleProp(t, event.Properties, "old_plan_id", "free")
	assertLifecycleProp(t, event.Properties, "new_plan_id", "basic")
	assertLifecycleProp(t, event.Properties, "change_type", "upgrade")
	assertLifecycleProp(t, event.Properties, "billing_url", "https://app.unipost.dev/settings/billing")
}

func TestBuildLoopsAccountCanceledEventSkipsContact(t *testing.T) {
	canceledAt := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)

	event := buildLoopsAccountCanceledEvent(
		db.User{ID: "user_123", Email: "alex@example.com", Name: pgtype.Text{String: "Alex Smith", Valid: true}},
		db.Workspace{ID: "ws_123", Name: "Alex Workspace"},
		canceledAt,
	)

	if event.EventName != "user_account_canceled" {
		t.Fatalf("event name = %q, want user_account_canceled", event.EventName)
	}
	if !event.SkipContact {
		t.Fatal("expected account canceled event to skip contact upsert")
	}
	if event.IdempotencyKey != "user_account_canceled:user_123" {
		t.Fatalf("idempotency key = %q", event.IdempotencyKey)
	}
	assertLifecycleProp(t, event.Properties, "canceled_at", "2026-05-25T12:00:00Z")
}

func TestBuildLoopsPostFailedEvent(t *testing.T) {
	failure := postfailures.BuildParams(
		"post_123",
		"result_123",
		"ws_123",
		"acct_123",
		"youtube",
		"dispatch",
		"quota exceeded",
		"quota exceeded",
	)

	event := buildLoopsPostFailedEvent(
		db.User{ID: "user_123", Email: "alex@example.com", Name: pgtype.Text{String: "Alex Smith", Valid: true}},
		db.Workspace{ID: "ws_123", Name: "Alex Workspace"},
		db.SocialPost{ID: "post_123", WorkspaceID: "ws_123", ProfileIds: []string{"profile_123"}},
		db.SocialPostResult{ID: "result_123", SocialAccountID: "acct_123"},
		db.PostDeliveryJob{ID: "job_123", Attempts: 1, MaxAttempts: 5},
		failure,
		"https://app.unipost.dev",
	)

	if event.EventName != "post_failed" {
		t.Fatalf("event name = %q, want post_failed", event.EventName)
	}
	if event.IdempotencyKey != "post_failed:job_123:1" {
		t.Fatalf("idempotency key = %q", event.IdempotencyKey)
	}
	assertLifecycleProp(t, event.Properties, "post_id", "post_123")
	assertLifecycleProp(t, event.Properties, "result_id", "result_123")
	assertLifecycleProp(t, event.Properties, "social_account_id", "acct_123")
	assertLifecycleProp(t, event.Properties, "platform", "youtube")
	assertLifecycleProp(t, event.Properties, "error_code", "quota_exceeded")
	assertLifecycleProp(t, event.Properties, "dashboard_url", "https://app.unipost.dev/projects/profile_123/logs?post_id=post_123")
}

func assertLifecycleProp(t *testing.T, props map[string]any, key string, want any) {
	t.Helper()
	if got := props[key]; got != want {
		t.Fatalf("property %s = %#v, want %#v", key, got, want)
	}
}
