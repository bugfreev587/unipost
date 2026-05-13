package handler

import (
	"strings"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestShouldSendPaidActivationEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		previous *db.Subscription
		planID   string
		want     bool
	}{
		{
			name:   "new paid subscription sends email",
			planID: "growth",
			want:   true,
		},
		{
			name:     "free to paid sends email",
			previous: &db.Subscription{PlanID: "free"},
			planID:   "team",
			want:     true,
		},
		{
			name:     "paid to paid does not resend",
			previous: &db.Subscription{PlanID: "growth"},
			planID:   "team",
			want:     false,
		},
		{
			name:     "free plan never sends",
			previous: &db.Subscription{PlanID: "free"},
			planID:   "free",
			want:     false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldSendPaidActivationEmail(tc.previous, tc.planID); got != tc.want {
				t.Fatalf("shouldSendPaidActivationEmail(%+v, %q) = %v, want %v", tc.previous, tc.planID, got, tc.want)
			}
		})
	}
}

func TestRenderPaidActivationEmail(t *testing.T) {
	t.Parallel()

	msg := renderPaidActivationEmail("founder@example.com", "Acme", "Growth", "https://app.unipost.dev")
	if msg.To != "founder@example.com" {
		t.Fatalf("unexpected recipient: %q", msg.To)
	}
	if msg.Subject != "[UniPost] Welcome to Growth" {
		t.Fatalf("unexpected subject: %q", msg.Subject)
	}
	if !strings.Contains(msg.HTML, "Acme") || !strings.Contains(msg.HTML, "/settings/billing") {
		t.Fatalf("html body missing expected content: %s", msg.HTML)
	}
	if !strings.Contains(msg.Text, "Open billing: https://app.unipost.dev/settings/billing") {
		t.Fatalf("text body missing billing link: %s", msg.Text)
	}
}
