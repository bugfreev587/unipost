package emailpolicy

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/emailregistry"
)

func TestPrepareSkipsDisabledPreferenceGatedEmail(t *testing.T) {
	reader := &fakePreferenceReader{
		prefs: map[emailregistry.PreferenceCategory]Preference{
			emailregistry.PublishingFailures: {Enabled: false},
		},
	}
	service := NewService(reader, "https://dev-app.unipost.dev")

	decision, err := service.Prepare(context.Background(), Request{
		EventKey: "email.post.failed.v1",
		UserID:   "user_123",
		Email:    "alex@example.com",
		DataVariables: map[string]any{
			"workspace_name": "Alex Workspace",
		},
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}

	if decision.ShouldSend {
		t.Fatal("ShouldSend = true, want false")
	}
	if decision.SkipReason != SkipReasonPreferenceDisabled {
		t.Fatalf("SkipReason = %q, want %q", decision.SkipReason, SkipReasonPreferenceDisabled)
	}
	if reader.calls != 1 {
		t.Fatalf("preference calls = %d, want 1", reader.calls)
	}
}

func TestPrepareAllowsDefaultOnPreferenceAndAddsFooterVariables(t *testing.T) {
	reader := &fakePreferenceReader{err: ErrPreferenceNotFound}
	service := NewService(reader, "https://dev-app.unipost.dev/")

	decision, err := service.Prepare(context.Background(), Request{
		EventKey: "email.account.disconnected.v1",
		UserID:   "user_123",
		Email:    "alex@example.com",
		DataVariables: map[string]any{
			"workspace_name": "Alex Workspace",
		},
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}

	if !decision.ShouldSend {
		t.Fatalf("ShouldSend = false, reason %q", decision.SkipReason)
	}
	assertDecisionVariable(t, decision, "footer_policy", string(emailregistry.FooterManagePreferences))
	assertDecisionVariable(t, decision, "preference_category_key", string(emailregistry.AccountConnectionAlerts))
	assertDecisionVariable(t, decision, "preference_category_label", "Account connection alerts")
	assertDecisionVariable(t, decision, "manage_preferences_url", "https://dev-app.unipost.dev/settings/notifications")
	footerText := stringDecisionVariable(t, decision, "footer_text")
	if !strings.Contains(footerText, "Account connection alerts") || !strings.Contains(footerText, "settings") {
		t.Fatalf("footer_text = %q, want category and settings copy", footerText)
	}
	if _, ok := decision.DataVariables["unsubscribe_url"]; ok {
		t.Fatal("unsubscribe_url present while phase 5 one-click unsubscribe is out of scope")
	}
}

func TestPrepareDoesNotGateCriticalEmailAndAddsRequiredReason(t *testing.T) {
	reader := &fakePreferenceReader{
		prefs: map[emailregistry.PreferenceCategory]Preference{
			emailregistry.EssentialAccountBilling: {Enabled: false},
		},
	}
	service := NewService(reader, "https://dev-app.unipost.dev")

	decision, err := service.Prepare(context.Background(), Request{
		EventKey: "email.billing.payment_failed.v1",
		UserID:   "user_123",
		Email:    "alex@example.com",
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}

	if !decision.ShouldSend {
		t.Fatalf("critical email should send, reason %q", decision.SkipReason)
	}
	if reader.calls != 0 {
		t.Fatalf("critical email should not read preferences, calls=%d", reader.calls)
	}
	assertDecisionVariable(t, decision, "footer_policy", string(emailregistry.FooterRequiredNotice))
	reason := stringDecisionVariable(t, decision, "footer_reason")
	if !strings.Contains(strings.ToLower(reason), "billing") {
		t.Fatalf("footer_reason = %q, want billing explanation", reason)
	}
}

func TestPreparePropagatesPreferenceLookupErrors(t *testing.T) {
	service := NewService(&fakePreferenceReader{err: errors.New("db down")}, "https://dev-app.unipost.dev")

	_, err := service.Prepare(context.Background(), Request{
		EventKey: "email.post.failed.v1",
		UserID:   "user_123",
		Email:    "alex@example.com",
	})
	if err == nil || !strings.Contains(err.Error(), "db down") {
		t.Fatalf("error = %v, want db down", err)
	}
}

type fakePreferenceReader struct {
	prefs map[emailregistry.PreferenceCategory]Preference
	err   error
	calls int
}

func (f *fakePreferenceReader) EmailPreference(_ context.Context, userID, email string, category emailregistry.PreferenceCategory) (Preference, error) {
	f.calls++
	if userID != "user_123" {
		return Preference{}, errors.New("unexpected user id")
	}
	if email != "alex@example.com" {
		return Preference{}, errors.New("unexpected email")
	}
	if f.err != nil {
		return Preference{}, f.err
	}
	if pref, ok := f.prefs[category]; ok {
		return pref, nil
	}
	return Preference{}, ErrPreferenceNotFound
}

func assertDecisionVariable(t *testing.T, decision Decision, key string, want any) {
	t.Helper()
	if got := decision.DataVariables[key]; got != want {
		t.Fatalf("%s = %#v, want %#v", key, got, want)
	}
}

func stringDecisionVariable(t *testing.T, decision Decision, key string) string {
	t.Helper()
	value, ok := decision.DataVariables[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		t.Fatalf("%s = %#v, want non-empty string", key, decision.DataVariables[key])
	}
	return value
}
