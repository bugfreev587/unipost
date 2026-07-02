package emailregistry

type DeliveryClass string

const (
	CriticalTransactional DeliveryClass = "critical_transactional"
	ServiceAlert          DeliveryClass = "service_alert"
	Lifecycle             DeliveryClass = "lifecycle"
	Marketing             DeliveryClass = "marketing"
	Test                  DeliveryClass = "test"
)

type PreferenceCategory string

const (
	EssentialAccountBilling PreferenceCategory = "essential_account_billing"
	PublishingFailures      PreferenceCategory = "publishing_failures"
	AccountConnectionAlerts PreferenceCategory = "account_connection_alerts"
	UsageQuotaAlerts        PreferenceCategory = "usage_quota_alerts"
	SupportFollowUps        PreferenceCategory = "support_follow_ups"
	OnboardingTips          PreferenceCategory = "onboarding_tips"
	ProductUpdates          PreferenceCategory = "product_updates"
	TestEmails              PreferenceCategory = "test_emails"
)

type FooterPolicy string

const (
	FooterUnsubscribe            FooterPolicy = "unsubscribe"
	FooterManagePreferences      FooterPolicy = "manage_preferences"
	FooterRequiredNotice         FooterPolicy = "required_notice"
	FooterRequiredNoticeNoManage FooterPolicy = "required_notice_no_manage"
	FooterTestNotice             FooterPolicy = "test_notice"
)

type EmailPreferenceCategory struct {
	Key            PreferenceCategory `json:"category_key"`
	Label          string             `json:"label"`
	Description    string             `json:"description"`
	DefaultEnabled bool               `json:"default_enabled"`
	Locked         bool               `json:"locked"`
}

type Event struct {
	Key                 string
	Domain              string
	TriggerSource       string
	Provider            string
	TemplateEnv         string
	LoopsEventName      string
	ExternalLoopsConfig string
	DeliveryClass       DeliveryClass
	PreferenceCategory  PreferenceCategory
	CanUnsubscribe      bool
	PreferenceGated     bool
	RequiredReason      string
	FooterPolicy        FooterPolicy
	RecipientPolicy     string
	IdempotencyPolicy   string
	RequiredVariables   []string
	AuditPolicy         string
	FallbackPolicy      string
	RetentionPolicy     string
	OwnerArea           string
}

func (e Event) CanManagePreferences() bool {
	category, ok := lookupPreferenceCategory(e.PreferenceCategory)
	return ok && !category.Locked
}

var preferenceCategories = []EmailPreferenceCategory{
	{
		Key:            EssentialAccountBilling,
		Label:          "Essential account and billing emails",
		Description:    "Invites, billing, account security, subscription, and account deletion messages.",
		DefaultEnabled: true,
		Locked:         true,
	},
	{
		Key:            PublishingFailures,
		Label:          "Publishing failure alerts",
		Description:    "Emails when a post cannot be delivered to a selected platform.",
		DefaultEnabled: true,
		Locked:         false,
	},
	{
		Key:            AccountConnectionAlerts,
		Label:          "Account connection alerts",
		Description:    "Emails when a connected social account needs attention before UniPost can publish.",
		DefaultEnabled: true,
		Locked:         false,
	},
	{
		Key:            UsageQuotaAlerts,
		Label:          "Usage and quota alerts",
		Description:    "Emails about free-plan usage thresholds and posting limits.",
		DefaultEnabled: true,
		Locked:         false,
	},
	{
		Key:            SupportFollowUps,
		Label:          "Support follow-ups",
		Description:    "Emails sent after UniPost support reviews an issue that affects your account.",
		DefaultEnabled: true,
		Locked:         false,
	},
	{
		Key:            OnboardingTips,
		Label:          "Onboarding tips",
		Description:    "Product guidance and setup tips for getting started with UniPost.",
		DefaultEnabled: true,
		Locked:         false,
	},
	{
		Key:            ProductUpdates,
		Label:          "Product updates",
		Description:    "Announcements, launches, education, and promotional campaigns.",
		DefaultEnabled: false,
		Locked:         false,
	},
	{
		Key:            TestEmails,
		Label:          "Test emails",
		Description:    "Emails sent only when you explicitly test your notification channel.",
		DefaultEnabled: true,
		Locked:         true,
	},
}

func EmailPreferenceCategories() []EmailPreferenceCategory {
	out := make([]EmailPreferenceCategory, len(preferenceCategories))
	copy(out, preferenceCategories)
	return out
}

func LookupPreferenceCategory(key PreferenceCategory) (EmailPreferenceCategory, bool) {
	return lookupPreferenceCategory(key)
}

func lookupPreferenceCategory(key PreferenceCategory) (EmailPreferenceCategory, bool) {
	for _, category := range preferenceCategories {
		if category.Key == key {
			return category, true
		}
	}
	return EmailPreferenceCategory{}, false
}

var events = []Event{
	{
		Key:                 "email.user.welcome.v1",
		Domain:              "user",
		TriggerSource:       "clerk user.created webhook after workspace creation",
		Provider:            "loops",
		TemplateEnv:         "LOOPS_USER_WELCOME_TRANSACTIONAL_ID",
		ExternalLoopsConfig: "Audit workflows listening to user_signed_up before enabling this template.",
		DeliveryClass:       Lifecycle,
		PreferenceCategory:  OnboardingTips,
		CanUnsubscribe:      true,
		PreferenceGated:     false,
		FooterPolicy:        FooterUnsubscribe,
		RecipientPolicy:     "new dashboard user",
		IdempotencyPolicy:   "user_welcome:{user_id}",
		RequiredVariables:   []string{"recipient_name", "workspace_name", "app_url", "connect_url", "discord_url"},
		AuditPolicy:         "record one send attempt per user welcome",
		FallbackPolicy:      "none by default",
		RetentionPolicy:     "retain metadata and variable snapshots for 13 months; redact support-free text if added later",
		OwnerArea:           "Growth lifecycle",
	},
	{
		Key:                 "email.workspace.member_invited.v1",
		Domain:              "workspace",
		TriggerSource:       "workspace invite created",
		Provider:            "loops",
		TemplateEnv:         "LOOPS_WORKSPACE_MEMBER_INVITED_TRANSACTIONAL_ID",
		ExternalLoopsConfig: "No Loops workflow should listen to workspace member invite events unless this template is retired.",
		DeliveryClass:       CriticalTransactional,
		PreferenceCategory:  EssentialAccountBilling,
		CanUnsubscribe:      false,
		PreferenceGated:     false,
		RequiredReason:      "Workspace invites are required so members can access the workspace they were invited to join.",
		FooterPolicy:        FooterRequiredNotice,
		RecipientPolicy:     "invited email address",
		IdempotencyPolicy:   "workspace_invite:{invite_id}",
		RequiredVariables:   []string{"workspace_name", "role", "accept_url", "expires_at"},
		AuditPolicy:         "record one send attempt per invite id",
		FallbackPolicy:      "none by default",
		RetentionPolicy:     "retain metadata and variable snapshots for 13 months",
		OwnerArea:           "Workspace collaboration",
	},
	{
		Key:                 "email.billing.plan_changed.v1",
		Domain:              "billing",
		TriggerSource:       "Stripe checkout or subscription update changes plan",
		Provider:            "loops",
		TemplateEnv:         "LOOPS_PLAN_CHANGED_TRANSACTIONAL_ID",
		ExternalLoopsConfig: "Audit workflows listening to plan_changed before enabling paid activation copy.",
		DeliveryClass:       CriticalTransactional,
		LoopsEventName:      "plan_changed",
		PreferenceCategory:  EssentialAccountBilling,
		CanUnsubscribe:      false,
		PreferenceGated:     false,
		RequiredReason:      "Billing emails are required to keep you informed about your UniPost subscription.",
		FooterPolicy:        FooterRequiredNotice,
		RecipientPolicy:     "workspace owner",
		IdempotencyPolicy:   "existing plan_changed Stripe event key",
		RequiredVariables:   []string{"workspace_name", "old_plan_id", "new_plan_id", "change_type", "billing_url"},
		AuditPolicy:         "record one send attempt per plan transition idempotency key",
		FallbackPolicy:      "none by default",
		RetentionPolicy:     "retain metadata and variable snapshots for 13 months",
		OwnerArea:           "Billing",
	},
	{
		Key:                 "email.billing.payment_failed.v1",
		Domain:              "billing",
		TriggerSource:       "Stripe invoice.payment_failed",
		Provider:            "loops",
		TemplateEnv:         "LOOPS_BILLING_PAYMENT_FAILED_TRANSACTIONAL_ID",
		ExternalLoopsConfig: "No Loops workflow should send additional dunning email for the same invoice attempt.",
		DeliveryClass:       CriticalTransactional,
		LoopsEventName:      "billing_payment_failed",
		PreferenceCategory:  EssentialAccountBilling,
		CanUnsubscribe:      false,
		PreferenceGated:     false,
		RequiredReason:      "Billing emails are required to keep you informed about failed payments and account status.",
		FooterPolicy:        FooterRequiredNotice,
		RecipientPolicy:     "workspace owner",
		IdempotencyPolicy:   "billing_payment_failed:{invoice_id}:{attempt_count}",
		RequiredVariables:   []string{"workspace_name", "plan_id", "billing_url", "retry_message", "attempt_count", "next_payment_attempt"},
		AuditPolicy:         "record one send attempt per invoice collection attempt",
		FallbackPolicy:      "none by default",
		RetentionPolicy:     "retain metadata and variable snapshots for 13 months",
		OwnerArea:           "Billing",
	},
	{
		Key:                 "email.billing.payment_recovered.v1",
		Domain:              "billing",
		TriggerSource:       "Stripe payment succeeds after prior past_due or failed state",
		Provider:            "loops",
		TemplateEnv:         "LOOPS_BILLING_PAYMENT_RECOVERED_TRANSACTIONAL_ID",
		ExternalLoopsConfig: "No Loops workflow should send recovery email without backend recovery audit.",
		DeliveryClass:       CriticalTransactional,
		LoopsEventName:      "billing_payment_recovered",
		PreferenceCategory:  EssentialAccountBilling,
		CanUnsubscribe:      false,
		PreferenceGated:     false,
		RequiredReason:      "Billing emails are required to keep you informed about payment and subscription status.",
		FooterPolicy:        FooterRequiredNotice,
		RecipientPolicy:     "workspace owner",
		IdempotencyPolicy:   "billing_payment_recovered:{invoice_id}",
		RequiredVariables:   []string{"workspace_name", "plan_id", "billing_url"},
		AuditPolicy:         "record one send attempt per recovered invoice",
		FallbackPolicy:      "none by default",
		RetentionPolicy:     "retain metadata and variable snapshots for 13 months",
		OwnerArea:           "Billing",
	},
	{
		Key:                 "email.billing.subscription_canceled.v1",
		Domain:              "billing",
		TriggerSource:       "Stripe subscription canceled or user cancels paid plan",
		Provider:            "loops",
		TemplateEnv:         "LOOPS_BILLING_SUBSCRIPTION_CANCELED_TRANSACTIONAL_ID",
		ExternalLoopsConfig: "No overlapping cancellation workflow should send for the same subscription cancellation.",
		DeliveryClass:       CriticalTransactional,
		LoopsEventName:      "billing_subscription_canceled",
		PreferenceCategory:  EssentialAccountBilling,
		CanUnsubscribe:      false,
		PreferenceGated:     false,
		RequiredReason:      "Billing emails are required to confirm subscription changes and cancellation status.",
		FooterPolicy:        FooterRequiredNotice,
		RecipientPolicy:     "workspace owner",
		IdempotencyPolicy:   "billing_subscription_canceled:{subscription_id}:{effective_at}",
		RequiredVariables:   []string{"workspace_name", "plan_id", "effective_at", "billing_url"},
		AuditPolicy:         "record one send attempt per subscription cancellation effective date",
		FallbackPolicy:      "none by default",
		RetentionPolicy:     "retain metadata and variable snapshots for 13 months",
		OwnerArea:           "Billing",
	},
	{
		Key:                 "email.quota.free_plan_reminder.v1",
		Domain:              "quota",
		TriggerSource:       "free workspace crosses quota threshold during publish, schedule, or block path",
		Provider:            "loops",
		TemplateEnv:         "LOOPS_FREE_PLAN_QUOTA_REMINDER_TRANSACTIONAL_ID",
		ExternalLoopsConfig: "No Loops workflow should independently calculate quota thresholds.",
		DeliveryClass:       ServiceAlert,
		PreferenceCategory:  UsageQuotaAlerts,
		CanUnsubscribe:      false,
		PreferenceGated:     false,
		FooterPolicy:        FooterManagePreferences,
		RecipientPolicy:     "workspace owner",
		IdempotencyPolicy:   "free_plan_quota:{workspace_id}:{period}:{threshold_percent}",
		RequiredVariables:   []string{"subject", "preview_text", "headline", "recipient_name", "workspace_name", "body", "usage_percent", "posts_limit", "pricing_url", "billing_url"},
		AuditPolicy:         "use free_plan_quota_email_reminders ledger",
		FallbackPolicy:      "none by default",
		RetentionPolicy:     "retain ledger and variable snapshots for 13 months",
		OwnerArea:           "Billing / Growth lifecycle",
	},
	{
		Key:                 "email.account.disconnected.v1",
		Domain:              "account",
		TriggerSource:       "manual disconnect or token refresh permanently fails",
		Provider:            "loops",
		TemplateEnv:         "LOOPS_ACCOUNT_DISCONNECTED_TRANSACTIONAL_ID",
		ExternalLoopsConfig: "No overlapping reconnect workflow should send immediate duplicate email for the same disconnect event.",
		DeliveryClass:       ServiceAlert,
		LoopsEventName:      "account_disconnected",
		PreferenceCategory:  AccountConnectionAlerts,
		CanUnsubscribe:      false,
		PreferenceGated:     true,
		FooterPolicy:        FooterManagePreferences,
		RecipientPolicy:     "workspace owner",
		IdempotencyPolicy:   "account_disconnected:{social_account_id}:{event_source}",
		RequiredVariables:   []string{"workspace_name", "platform", "account_name", "reconnect_url", "reason"},
		AuditPolicy:         "record one send attempt per disconnect source",
		FallbackPolicy:      "none by default",
		RetentionPolicy:     "retain metadata and variable snapshots for 13 months",
		OwnerArea:           "Accounts / Notifications",
	},
	{
		Key:                 "email.post.failed.v1",
		Domain:              "publishing",
		TriggerSource:       "terminal publish failure after retry policy",
		Provider:            "loops",
		TemplateEnv:         "LOOPS_POST_FAILED_TRANSACTIONAL_ID",
		ExternalLoopsConfig: "Audit workflows listening to post_failed before enabling this template.",
		DeliveryClass:       ServiceAlert,
		LoopsEventName:      "post_failed",
		PreferenceCategory:  PublishingFailures,
		CanUnsubscribe:      false,
		PreferenceGated:     true,
		FooterPolicy:        FooterManagePreferences,
		RecipientPolicy:     "workspace owner",
		IdempotencyPolicy:   "existing post_failed job attempt key; one send per terminal failure",
		RequiredVariables:   []string{"workspace_name", "post_id", "platform", "error_code", "dashboard_url", "retriable"},
		AuditPolicy:         "record one send attempt per terminal failure idempotency key",
		FallbackPolicy:      "none by default",
		RetentionPolicy:     "retain metadata and redacted variable snapshots for 13 months",
		OwnerArea:           "Publishing / Notifications",
	},
	{
		Key:                 "email.support.error_triage_follow_up.v1",
		Domain:              "support",
		TriggerSource:       "admin sends reviewed error triage draft",
		Provider:            "loops",
		TemplateEnv:         "LOOPS_ERROR_TRIAGE_USER_ACTION_TRANSACTIONAL_ID",
		ExternalLoopsConfig: "No workflow should auto-send this support follow-up; admin click is required.",
		DeliveryClass:       ServiceAlert,
		PreferenceCategory:  SupportFollowUps,
		CanUnsubscribe:      false,
		PreferenceGated:     false,
		FooterPolicy:        FooterManagePreferences,
		RecipientPolicy:     "admin-selected affected dashboard user",
		IdempotencyPolicy:   "error_triage_email:{item_id}:{recipient_scope_key}:v{draft_version}",
		RequiredVariables:   []string{"subject", "body", "cta_url"},
		AuditPolicy:         "use error_triage_email_sends ledger",
		FallbackPolicy:      "none",
		RetentionPolicy:     "follow error triage support/audit retention policy",
		OwnerArea:           "Support / Admin",
	},
	{
		Key:                 "email.notification.test.v1",
		Domain:              "notification",
		TriggerSource:       "user tests email channel",
		Provider:            "loops",
		TemplateEnv:         "LOOPS_NOTIFICATION_TEST_TRANSACTIONAL_ID",
		ExternalLoopsConfig: "No workflow should listen to notification test events.",
		DeliveryClass:       Test,
		PreferenceCategory:  TestEmails,
		CanUnsubscribe:      false,
		PreferenceGated:     false,
		FooterPolicy:        FooterTestNotice,
		RecipientPolicy:     "authenticated user",
		IdempotencyPolicy:   "no suppression idempotency; allow repeated user-initiated tests",
		RequiredVariables:   []string{"recipient_name", "settings_url"},
		AuditPolicy:         "record each explicit test attempt",
		FallbackPolicy:      "none by default",
		RetentionPolicy:     "retain metadata for 90 days",
		OwnerArea:           "Notifications",
	},
	{
		Key:                 "email.user.account_canceled.v1",
		Domain:              "user",
		TriggerSource:       "user deletes UniPost account",
		Provider:            "loops",
		TemplateEnv:         "LOOPS_ACCOUNT_CANCELED_TRANSACTIONAL_ID",
		ExternalLoopsConfig: "No cancellation workflow should send in addition to the backend account-canceled template.",
		DeliveryClass:       CriticalTransactional,
		LoopsEventName:      "user_account_canceled",
		PreferenceCategory:  EssentialAccountBilling,
		CanUnsubscribe:      false,
		PreferenceGated:     false,
		RequiredReason:      "Account emails are required to confirm important account-status changes.",
		FooterPolicy:        FooterRequiredNotice,
		RecipientPolicy:     "canceling user",
		IdempotencyPolicy:   "user_account_canceled:{user_id}",
		RequiredVariables:   []string{"canceled_at"},
		AuditPolicy:         "record one send attempt per account cancellation",
		FallbackPolicy:      "none by default",
		RetentionPolicy:     "retain metadata and variable snapshots for 13 months",
		OwnerArea:           "Growth lifecycle / Account",
	},
}

func Registry() []Event {
	out := make([]Event, len(events))
	copy(out, events)
	return out
}

func Lookup(key string) (Event, bool) {
	for _, event := range events {
		if event.Key == key {
			return event, true
		}
	}
	return Event{}, false
}

func LookupByLoopsEventName(eventName string) (Event, bool) {
	for _, event := range events {
		if event.LoopsEventName == eventName {
			return event, true
		}
	}
	return Event{}, false
}
