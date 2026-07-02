package emailregistry

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestRegistryContainsRequiredEmailEvents(t *testing.T) {
	events := byKey(t, Registry())

	expected := map[string]string{
		"email.user.welcome.v1":                   "LOOPS_USER_WELCOME_TRANSACTIONAL_ID",
		"email.workspace.member_invited.v1":       "LOOPS_WORKSPACE_MEMBER_INVITED_TRANSACTIONAL_ID",
		"email.billing.plan_changed.v1":           "LOOPS_PLAN_CHANGED_TRANSACTIONAL_ID",
		"email.billing.payment_failed.v1":         "LOOPS_BILLING_PAYMENT_FAILED_TRANSACTIONAL_ID",
		"email.billing.payment_recovered.v1":      "LOOPS_BILLING_PAYMENT_RECOVERED_TRANSACTIONAL_ID",
		"email.billing.subscription_canceled.v1":  "LOOPS_BILLING_SUBSCRIPTION_CANCELED_TRANSACTIONAL_ID",
		"email.quota.free_plan_reminder.v1":       "LOOPS_FREE_PLAN_QUOTA_REMINDER_TRANSACTIONAL_ID",
		"email.account.disconnected.v1":           "LOOPS_ACCOUNT_DISCONNECTED_TRANSACTIONAL_ID",
		"email.post.failed.v1":                    "LOOPS_POST_FAILED_TRANSACTIONAL_ID",
		"email.support.error_triage_follow_up.v1": "LOOPS_ERROR_TRIAGE_USER_ACTION_TRANSACTIONAL_ID",
		"email.notification.test.v1":              "LOOPS_NOTIFICATION_TEST_TRANSACTIONAL_ID",
		"email.user.account_canceled.v1":          "LOOPS_ACCOUNT_CANCELED_TRANSACTIONAL_ID",
	}

	for key, env := range expected {
		event, ok := events[key]
		if !ok {
			t.Fatalf("registry missing %s", key)
		}
		if event.TemplateEnv != env {
			t.Fatalf("%s TemplateEnv = %q, want %q", key, event.TemplateEnv, env)
		}
	}
}

func TestRegistryEntriesHaveRequiredContractFields(t *testing.T) {
	seen := map[string]bool{}
	for _, event := range Registry() {
		if strings.TrimSpace(event.Key) == "" {
			t.Fatal("registry contains event with empty key")
		}
		if seen[event.Key] {
			t.Fatalf("duplicate event key %q", event.Key)
		}
		seen[event.Key] = true

		required := map[string]string{
			"domain":              event.Domain,
			"provider":            event.Provider,
			"template_env":        event.TemplateEnv,
			"delivery_class":      string(event.DeliveryClass),
			"recipient_policy":    event.RecipientPolicy,
			"idempotency_policy":  event.IdempotencyPolicy,
			"audit_policy":        event.AuditPolicy,
			"fallback_policy":     event.FallbackPolicy,
			"retention_policy":    event.RetentionPolicy,
			"owner_area":          event.OwnerArea,
			"preference_category": string(event.PreferenceCategory),
			"footer_policy":       string(event.FooterPolicy),
		}
		for name, value := range required {
			if strings.TrimSpace(value) == "" {
				t.Fatalf("%s missing %s", event.Key, name)
			}
		}
		if len(event.RequiredVariables) == 0 {
			t.Fatalf("%s missing required variables", event.Key)
		}
		if event.PreferenceGated && !event.CanManagePreferences() {
			t.Fatalf("%s is preference gated but not manageable", event.Key)
		}
		if event.DeliveryClass == CriticalTransactional && strings.TrimSpace(event.RequiredReason) == "" {
			t.Fatalf("%s critical email missing required reason", event.Key)
		}
	}
}

func TestRegistryDefinesPolicyForServiceAlertPreferences(t *testing.T) {
	events := byKey(t, Registry())

	for _, tc := range []struct {
		key      string
		category PreferenceCategory
		loops    string
	}{
		{key: "email.post.failed.v1", category: PublishingFailures, loops: "post_failed"},
		{key: "email.account.disconnected.v1", category: AccountConnectionAlerts, loops: "account_disconnected"},
	} {
		event := events[tc.key]
		if event.DeliveryClass != ServiceAlert {
			t.Fatalf("%s class = %q, want service_alert", tc.key, event.DeliveryClass)
		}
		if event.PreferenceCategory != tc.category {
			t.Fatalf("%s category = %q, want %q", tc.key, event.PreferenceCategory, tc.category)
		}
		if !event.PreferenceGated {
			t.Fatalf("%s should be preference gated", tc.key)
		}
		if event.FooterPolicy != FooterManagePreferences {
			t.Fatalf("%s footer policy = %q, want manage_preferences", tc.key, event.FooterPolicy)
		}
		if event.LoopsEventName != tc.loops {
			t.Fatalf("%s loops event = %q, want %q", tc.key, event.LoopsEventName, tc.loops)
		}
	}
}

func TestLookupByLoopsEventNameUsesRegistryDeliveryClass(t *testing.T) {
	event, ok := LookupByLoopsEventName("post_failed")
	if !ok {
		t.Fatal("post_failed missing from Loops event lookup")
	}
	if event.Key != "email.post.failed.v1" {
		t.Fatalf("event key = %q, want email.post.failed.v1", event.Key)
	}
	if event.DeliveryClass != ServiceAlert {
		t.Fatalf("delivery class = %q, want service_alert", event.DeliveryClass)
	}

	event, ok = LookupByLoopsEventName("billing_payment_failed")
	if !ok {
		t.Fatal("billing_payment_failed missing from Loops event lookup")
	}
	if event.Key != "email.billing.payment_failed.v1" {
		t.Fatalf("event key = %q, want email.billing.payment_failed.v1", event.Key)
	}
	if event.DeliveryClass != CriticalTransactional {
		t.Fatalf("delivery class = %q, want critical_transactional", event.DeliveryClass)
	}
}

func TestEmailPreferenceCategoriesExposeUserControls(t *testing.T) {
	categories := EmailPreferenceCategories()
	byCategory := map[PreferenceCategory]EmailPreferenceCategory{}
	for _, category := range categories {
		if strings.TrimSpace(string(category.Key)) == "" {
			t.Fatal("category missing key")
		}
		if strings.TrimSpace(category.Label) == "" {
			t.Fatalf("%s missing label", category.Key)
		}
		byCategory[category.Key] = category
	}

	if !byCategory[PublishingFailures].DefaultEnabled || byCategory[PublishingFailures].Locked {
		t.Fatalf("publishing failures should default on and be user-manageable: %+v", byCategory[PublishingFailures])
	}
	if !byCategory[AccountConnectionAlerts].DefaultEnabled || byCategory[AccountConnectionAlerts].Locked {
		t.Fatalf("account connection alerts should default on and be user-manageable: %+v", byCategory[AccountConnectionAlerts])
	}
	if !byCategory[EssentialAccountBilling].Locked {
		t.Fatalf("essential account/billing category should be locked")
	}
}

func TestRegistryCoversTemplateLinkVariables(t *testing.T) {
	events := byKey(t, Registry())
	expected := map[string][]string{
		"email.user.welcome.v1":                   {"app_url", "connect_url", "discord_url"},
		"email.workspace.member_invited.v1":       {"accept_url"},
		"email.billing.payment_failed.v1":         {"billing_url"},
		"email.billing.payment_recovered.v1":      {"billing_url"},
		"email.billing.subscription_canceled.v1":  {"billing_url"},
		"email.quota.free_plan_reminder.v1":       {"pricing_url", "billing_url"},
		"email.account.disconnected.v1":           {"reconnect_url"},
		"email.support.error_triage_follow_up.v1": {"cta_url"},
		"email.notification.test.v1":              {"settings_url"},
	}

	for key, vars := range expected {
		event, ok := events[key]
		if !ok {
			t.Fatalf("registry missing %s", key)
		}
		required := map[string]bool{}
		for _, name := range event.RequiredVariables {
			required[name] = true
		}
		for _, name := range vars {
			if !required[name] {
				t.Fatalf("%s required variables missing template link variable %q", key, name)
			}
		}
	}
}

func TestCurrentlyWiredLoopsTransactionalIDsHaveRegistryEntries(t *testing.T) {
	mainSource, err := os.ReadFile("../../cmd/api/main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	re := regexp.MustCompile(`LOOPS_[A-Z0-9_]+_TRANSACTIONAL_ID`)
	wired := map[string]bool{}
	for _, match := range re.FindAllString(string(mainSource), -1) {
		wired[match] = true
	}
	if len(wired) == 0 {
		t.Fatal("no LOOPS transactional IDs found in api/cmd/api/main.go")
	}

	registryEnvs := map[string]string{}
	for _, event := range Registry() {
		registryEnvs[event.TemplateEnv] = event.Key
	}
	for env := range wired {
		if _, ok := registryEnvs[env]; !ok {
			t.Fatalf("%s is wired in main.go but missing from email registry", env)
		}
	}
}

func TestTemplateContractsDocumentRegistryAndLoopsDashboardAudits(t *testing.T) {
	doc, err := os.ReadFile("../../../docs/email-templates.md")
	if err != nil {
		t.Fatalf("read docs/email-templates.md: %v", err)
	}
	body := string(doc)

	for _, event := range Registry() {
		for _, want := range []string{event.Key, event.TemplateEnv, event.IdempotencyPolicy} {
			if !strings.Contains(body, want) {
				t.Fatalf("docs/email-templates.md missing %q for %s", want, event.Key)
			}
		}
	}
	for _, auditKey := range []string{"user_signed_up", "post_failed", "plan_changed"} {
		if !strings.Contains(body, auditKey) {
			t.Fatalf("docs/email-templates.md missing Loops dashboard audit key %q", auditKey)
		}
	}
}

func byKey(t *testing.T, events []Event) map[string]Event {
	t.Helper()
	out := map[string]Event{}
	for _, event := range events {
		out[event.Key] = event
	}
	return out
}
