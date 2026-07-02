package handler

import (
	"context"
	"strings"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestNotificationHandlerSendEmailTestUsesLoopsTemplateWhenConfigured(t *testing.T) {
	mailer := &recordingMailer{}
	sender := &recordingTransactionalSender{}
	handler := NewNotificationHandler(nil, mailer, "https://dev-app.unipost.dev").
		SetNotificationTestEmailSender(sender, "tmpl_notification_test")

	err := handler.sendTestChannel(context.Background(), db.UnipostNotificationChannel{
		ID:     "ch_123",
		UserID: "user_123",
		Kind:   "email",
		Config: []byte(`{"address":"alex@example.com"}`),
	})
	if err != nil {
		t.Fatalf("sendTestChannel returned error: %v", err)
	}

	if sender.sent != 1 {
		t.Fatalf("Loops transactional sends = %d, want 1", sender.sent)
	}
	if mailer.sent != 0 {
		t.Fatalf("Resend mailer sends = %d, want 0", mailer.sent)
	}
	email := sender.last
	if email.TransactionalID != "tmpl_notification_test" {
		t.Fatalf("transactional ID = %q, want tmpl_notification_test", email.TransactionalID)
	}
	if email.Email != "alex@example.com" {
		t.Fatalf("email = %q", email.Email)
	}
	if email.UserID != "user_123" {
		t.Fatalf("user ID = %q, want user_123", email.UserID)
	}
	if email.IdempotencyKey != "" {
		t.Fatalf("idempotency key = %q, want empty for repeated user-initiated tests", email.IdempotencyKey)
	}
	assertDataVariable(t, email.DataVariables, "recipient_name", "there")
	assertDataVariable(t, email.DataVariables, "settings_url", "https://dev-app.unipost.dev/settings/notifications")
	assertDataVariable(t, email.DataVariables, "footer_policy", "test_notice")
	assertDataVariable(t, email.DataVariables, "manage_preferences_url", "https://dev-app.unipost.dev/settings/notifications")
}

func TestNotificationHandlerSendEmailTestDoesNotFallbackToResendWithoutLoopsTemplate(t *testing.T) {
	mailer := &recordingMailer{}
	handler := NewNotificationHandler(nil, mailer, "https://dev-app.unipost.dev")

	err := handler.sendTestChannel(context.Background(), db.UnipostNotificationChannel{
		ID:     "ch_123",
		UserID: "user_123",
		Kind:   "email",
		Config: []byte(`{"address":"alex@example.com"}`),
	})
	if err == nil || !strings.Contains(err.Error(), "Loops notification test email is not configured") {
		t.Fatalf("error = %v, want Loops configuration error", err)
	}
	if mailer.sent != 0 {
		t.Fatalf("Resend mailer sends = %d, want 0", mailer.sent)
	}
}
