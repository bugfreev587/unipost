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
			"domain":             event.Domain,
			"provider":           event.Provider,
			"template_env":       event.TemplateEnv,
			"delivery_class":     string(event.DeliveryClass),
			"recipient_policy":   event.RecipientPolicy,
			"idempotency_policy": event.IdempotencyPolicy,
			"audit_policy":       event.AuditPolicy,
			"fallback_policy":    event.FallbackPolicy,
			"retention_policy":   event.RetentionPolicy,
			"owner_area":         event.OwnerArea,
		}
		for name, value := range required {
			if strings.TrimSpace(value) == "" {
				t.Fatalf("%s missing %s", event.Key, name)
			}
		}
		if len(event.RequiredVariables) == 0 {
			t.Fatalf("%s missing required variables", event.Key)
		}
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
