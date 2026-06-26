package handler

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/loops"
	mailpkg "github.com/xiaoboyu/unipost-api/internal/mail"
)

func TestMembersHandlerSendInviteEmailUsesLoopsTemplateWhenConfigured(t *testing.T) {
	mailer := &recordingMailer{}
	sender := &recordingTransactionalSender{}
	handler := NewMembersHandler(nil, nil, mailer, "https://dev-app.unipost.dev").
		SetInviteEmailSender(sender, "tmpl_workspace_invite")

	expiresAt := time.Date(2026, 7, 3, 12, 30, 0, 0, time.UTC)
	handler.sendInviteEmail(context.Background(), db.WorkspaceInvite{
		ID:          "inv_123",
		WorkspaceID: "ws_123",
		Email:       "teammate@example.com",
		Role:        "admin",
		ExpiresAt:   pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}, "Acme Workspace", "https://dev-app.unipost.dev/invite/tok_123")

	if sender.sent != 1 {
		t.Fatalf("Loops transactional sends = %d, want 1", sender.sent)
	}
	if mailer.sent != 0 {
		t.Fatalf("Resend mailer sends = %d, want 0", mailer.sent)
	}
	email := sender.last
	if email.TransactionalID != "tmpl_workspace_invite" {
		t.Fatalf("transactional ID = %q, want tmpl_workspace_invite", email.TransactionalID)
	}
	if email.Email != "teammate@example.com" {
		t.Fatalf("email = %q", email.Email)
	}
	if email.IdempotencyKey != "workspace_invite:inv_123" {
		t.Fatalf("idempotency key = %q", email.IdempotencyKey)
	}
	assertDataVariable(t, email.DataVariables, "workspace_name", "Acme Workspace")
	assertDataVariable(t, email.DataVariables, "role", "Admin")
	assertDataVariable(t, email.DataVariables, "accept_url", "https://dev-app.unipost.dev/invite/tok_123")
	assertDataVariable(t, email.DataVariables, "expires_at", expiresAt.Format(time.RFC3339))
}

func TestMembersHandlerSendInviteEmailDoesNotFallbackToResendWithoutLoopsTemplate(t *testing.T) {
	mailer := &recordingMailer{}
	handler := NewMembersHandler(nil, nil, mailer, "https://dev-app.unipost.dev")

	handler.sendInviteEmail(context.Background(), db.WorkspaceInvite{
		ID:          "inv_123",
		WorkspaceID: "ws_123",
		Email:       "teammate@example.com",
		Role:        "editor",
		ExpiresAt:   pgtype.Timestamptz{Time: time.Date(2026, 7, 3, 12, 30, 0, 0, time.UTC), Valid: true},
	}, "Acme Workspace", "https://dev-app.unipost.dev/invite/tok_123")

	if mailer.sent != 0 {
		t.Fatalf("Resend mailer sends = %d, want 0", mailer.sent)
	}
}

type recordingMailer struct {
	sent int
	last mailpkg.Message
}

func (m *recordingMailer) Send(_ context.Context, msg mailpkg.Message) error {
	m.sent++
	m.last = msg
	return nil
}

type recordingTransactionalSender struct {
	sent int
	last loops.TransactionalEmail
}

func (s *recordingTransactionalSender) SendTransactional(_ context.Context, email loops.TransactionalEmail) error {
	s.sent++
	s.last = email
	return nil
}

func assertDataVariable(t *testing.T, vars map[string]any, key string, want any) {
	t.Helper()
	if got := vars[key]; got != want {
		t.Fatalf("data variable %s = %#v, want %#v", key, got, want)
	}
}
