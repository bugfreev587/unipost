package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/loops"
)

func TestWebhookHandlerSyncLoopsDashboardUserBestEffort(t *testing.T) {
	syncer := &fakeLoopsSyncer{err: errors.New("provider unavailable")}
	h := NewWebhookHandler(nil, nil, "").SetLoopsSyncer(syncer)

	h.syncLoopsDashboardUser(context.Background(), "user.created", clerkUserData{
		ID:        "user_123",
		FirstName: "Alex",
		LastName:  "Smith",
	}, "alex@example.com", "Alex Smith", "ws_123", "Alex Workspace")

	if syncer.calls != 1 {
		t.Fatalf("calls = %d, want 1", syncer.calls)
	}
	got := syncer.lastUser
	if got.ID != "user_123" {
		t.Fatalf("user id = %q", got.ID)
	}
	if got.Email != "alex@example.com" {
		t.Fatalf("email = %q", got.Email)
	}
	if got.FirstName != "Alex" || got.LastName != "Smith" {
		t.Fatalf("name = %q %q", got.FirstName, got.LastName)
	}
	if got.WorkspaceID != "ws_123" || got.WorkspaceName != "Alex Workspace" {
		t.Fatalf("workspace = %q/%q", got.WorkspaceID, got.WorkspaceName)
	}
	if got.Event != "user.created" {
		t.Fatalf("event = %q", got.Event)
	}
}

func TestWebhookHandlerSendWelcomeEmailUsesLoopsTemplateWhenConfigured(t *testing.T) {
	mailer := &recordingMailer{}
	sender := &recordingTransactionalSender{}
	h := NewWebhookHandler(nil, mailer, "https://dev-app.unipost.dev").
		SetWelcomeEmailSender(sender, "tmpl_user_welcome")

	h.sendWelcomeEmail(context.Background(), "user_123", "alex@example.com", "Alex Smith", "ws_123", "Alex Workspace")

	if sender.sent != 1 {
		t.Fatalf("Loops transactional sends = %d, want 1", sender.sent)
	}
	if mailer.sent != 0 {
		t.Fatalf("Resend mailer sends = %d, want 0", mailer.sent)
	}
	email := sender.last
	if email.TransactionalID != "tmpl_user_welcome" {
		t.Fatalf("transactional ID = %q, want tmpl_user_welcome", email.TransactionalID)
	}
	if email.Email != "alex@example.com" {
		t.Fatalf("email = %q", email.Email)
	}
	if email.UserID != "user_123" {
		t.Fatalf("user ID = %q, want user_123", email.UserID)
	}
	if email.IdempotencyKey != "user_welcome:user_123" {
		t.Fatalf("idempotency key = %q, want user_welcome:user_123", email.IdempotencyKey)
	}
	assertDataVariable(t, email.DataVariables, "recipient_name", "Alex Smith")
	assertDataVariable(t, email.DataVariables, "workspace_name", "Alex Workspace")
	assertDataVariable(t, email.DataVariables, "app_url", "https://dev-app.unipost.dev")
	assertDataVariable(t, email.DataVariables, "connect_url", "https://dev-app.unipost.dev/projects")
	assertDataVariable(t, email.DataVariables, "discord_url", "https://discord.gg/HDBAhYpuQu")
	if email.Audit.EventKey != "email.user.welcome.v1" {
		t.Fatalf("audit event key = %q, want email.user.welcome.v1", email.Audit.EventKey)
	}
	if email.Audit.WorkspaceID != "ws_123" {
		t.Fatalf("audit workspace = %q, want ws_123", email.Audit.WorkspaceID)
	}
}

func TestWebhookHandlerSendWelcomeEmailDoesNotFallbackToResendWithoutLoopsTemplate(t *testing.T) {
	mailer := &recordingMailer{}
	h := NewWebhookHandler(nil, mailer, "https://dev-app.unipost.dev")

	h.sendWelcomeEmail(context.Background(), "user_123", "alex@example.com", "Alex Smith", "ws_123", "Alex Workspace")

	if mailer.sent != 0 {
		t.Fatalf("Resend mailer sends = %d, want 0", mailer.sent)
	}
}

type fakeLoopsSyncer struct {
	calls    int
	lastUser loops.DashboardUser
	err      error
}

func (f *fakeLoopsSyncer) SyncDashboardUser(_ context.Context, user loops.DashboardUser) error {
	f.calls++
	f.lastUser = user
	return f.err
}
