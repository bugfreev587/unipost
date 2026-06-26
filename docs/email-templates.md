# UniPost Email Template Contracts

**Owner:** Growth lifecycle / Notifications / Billing / Support
**Status:** Phase 1 registry contract
**Source registry:** `api/internal/emailregistry`

This document is the human-readable contract for UniPost user-facing email templates. It records the canonical email event key, Loops transactional template environment variable, required variables, idempotency policy, delivery class, preference behavior, and external Loops workflow audit requirement for each email event.

The backend remains the authority for trigger conditions, recipient resolution, idempotency, and audit records. Loops owns rendering and lifecycle workflow orchestration where explicitly documented here. Resend contacts must not be synced for these templates.

## Delivery Classes

| Class | Meaning |
|---|---|
| `critical_transactional` | The user expects this message as part of account, workspace, billing, or security behavior. It may be sent regardless of marketing subscription when legally and product-appropriate. |
| `service_alert` | Product or support alert tied to the user's workspace. Default-on, but should respect UniPost notification settings where feasible. |
| `lifecycle` | Onboarding, activation, retention, or churn messaging. Should respect Loops subscription status and lifecycle audience rules. |
| `test` | Explicit user-triggered delivery check. Repeated clicks should produce repeated test emails. |

## Template Contracts

### email.user.welcome.v1

- Template env: `LOOPS_USER_WELCOME_TRANSACTIONAL_ID`
- Provider: Loops
- Delivery class: `lifecycle`
- Owner area: Growth lifecycle
- Trigger: Clerk `user.created` webhook after UniPost creates the default workspace.
- Recipient policy: new dashboard user.
- Required variables: `recipient_name`, `workspace_name`, `app_url`, `connect_url`, `discord_url`
- Idempotency policy: `user_welcome:{user_id}`
- Audit policy: record one send attempt per user welcome.
- Fallback policy: none by default.
- Retention policy: retain metadata and variable snapshots for 13 months; redact support-free text if added later.
- External Loops workflow audit: audit workflows listening to `user_signed_up` before enabling this template. Exactly one welcome/onboarding entry email should send.

### email.workspace.member_invited.v1

- Template env: `LOOPS_WORKSPACE_MEMBER_INVITED_TRANSACTIONAL_ID`
- Provider: Loops
- Delivery class: `critical_transactional`
- Owner area: Workspace collaboration
- Trigger: UniPost workspace invite created from a `workspace_invites` row.
- Recipient policy: invited email address.
- Required variables: `workspace_name`, `role`, `accept_url`, `expires_at`
- Idempotency policy: `workspace_invite:{invite_id}`
- Audit policy: record one send attempt per invite id.
- Fallback policy: none by default.
- Retention policy: retain metadata and variable snapshots for 13 months.
- External Loops workflow audit: no Loops workflow should listen to workspace member invite events unless this transactional template is retired.

### email.billing.plan_changed.v1

- Template env: `LOOPS_PLAN_CHANGED_TRANSACTIONAL_ID`
- Provider: Loops
- Delivery class: `critical_transactional`
- Owner area: Billing
- Trigger: Stripe checkout or subscription update changes plan.
- Recipient policy: workspace owner.
- Required variables: `workspace_name`, `old_plan_id`, `new_plan_id`, `change_type`, `billing_url`
- Idempotency policy: existing plan_changed Stripe event key
- Audit policy: record one send attempt per plan transition idempotency key.
- Fallback policy: none by default.
- Retention policy: retain metadata and variable snapshots for 13 months.
- External Loops workflow audit: audit workflows listening to `plan_changed` before enabling paid activation copy. Exactly one paid activation or plan changed email should send for the same transition.

### email.billing.payment_failed.v1

- Template env: `LOOPS_BILLING_PAYMENT_FAILED_TRANSACTIONAL_ID`
- Provider: Loops
- Delivery class: `critical_transactional`
- Owner area: Billing
- Trigger: Stripe `invoice.payment_failed`.
- Recipient policy: workspace owner.
- Required variables: `workspace_name`, `plan_id`, `billing_url`, `retry_message`, `attempt_count`, `next_payment_attempt`
- Idempotency policy: `billing_payment_failed:{invoice_id}:{attempt_count}`
- Audit policy: record one send attempt per invoice collection attempt.
- Fallback policy: none by default.
- Retention policy: retain metadata and variable snapshots for 13 months.
- External Loops workflow audit: no Loops workflow should send additional dunning email for the same invoice attempt.

### email.billing.payment_recovered.v1

- Template env: `LOOPS_BILLING_PAYMENT_RECOVERED_TRANSACTIONAL_ID`
- Provider: Loops
- Delivery class: `critical_transactional`
- Owner area: Billing
- Trigger: Stripe payment succeeds after UniPost observed a prior `past_due` or failed state.
- Recipient policy: workspace owner.
- Required variables: `workspace_name`, `plan_id`, `billing_url`
- Idempotency policy: `billing_payment_recovered:{invoice_id}`
- Audit policy: record one send attempt per recovered invoice.
- Fallback policy: none by default.
- Retention policy: retain metadata and variable snapshots for 13 months.
- External Loops workflow audit: no Loops workflow should send recovery email without backend recovery audit.

### email.billing.subscription_canceled.v1

- Template env: `LOOPS_BILLING_SUBSCRIPTION_CANCELED_TRANSACTIONAL_ID`
- Provider: Loops
- Delivery class: `critical_transactional`
- Owner area: Billing
- Trigger: Stripe subscription canceled or user cancels paid plan.
- Recipient policy: workspace owner.
- Required variables: `workspace_name`, `plan_id`, `effective_at`, `billing_url`
- Idempotency policy: `billing_subscription_canceled:{subscription_id}:{effective_at}`
- Audit policy: record one send attempt per subscription cancellation effective date.
- Fallback policy: none by default.
- Retention policy: retain metadata and variable snapshots for 13 months.
- External Loops workflow audit: no overlapping cancellation workflow should send for the same subscription cancellation.

### email.quota.free_plan_reminder.v1

- Template env: `LOOPS_FREE_PLAN_QUOTA_REMINDER_TRANSACTIONAL_ID`
- Provider: Loops
- Delivery class: `service_alert`
- Owner area: Billing / Growth lifecycle
- Trigger: free workspace crosses quota threshold during publish, schedule, or block path.
- Recipient policy: workspace owner.
- Required variables: `subject`, `preview_text`, `headline`, `recipient_name`, `workspace_name`, `body`, `usage_percent`, `posts_limit`, `pricing_url`
- Idempotency policy: `free_plan_quota:{workspace_id}:{period}:{threshold_percent}`
- Audit policy: use `free_plan_quota_email_reminders` ledger.
- Fallback policy: none by default.
- Retention policy: retain ledger and variable snapshots for 13 months.
- External Loops workflow audit: no Loops workflow should independently calculate quota thresholds.

### email.account.disconnected.v1

- Template env: `LOOPS_ACCOUNT_DISCONNECTED_TRANSACTIONAL_ID`
- Provider: Loops
- Delivery class: `service_alert`
- Owner area: Accounts / Notifications
- Trigger: manual disconnect or token refresh permanently fails.
- Recipient policy: workspace owner.
- Required variables: `workspace_name`, `platform`, `account_name`, `reconnect_url`, `reason`
- Idempotency policy: `account_disconnected:{social_account_id}:{event_source}`
- Audit policy: record one send attempt per disconnect source.
- Fallback policy: none by default.
- Retention policy: retain metadata and variable snapshots for 13 months.
- External Loops workflow audit: no overlapping reconnect workflow should send immediate duplicate email for the same disconnect event.

### email.post.failed.v1

- Template env: `LOOPS_POST_FAILED_TRANSACTIONAL_ID`
- Provider: Loops
- Delivery class: `service_alert`
- Owner area: Publishing / Notifications
- Trigger: terminal publish failure after retry policy.
- Recipient policy: workspace owner.
- Required variables: `workspace_name`, `post_id`, `platform`, `error_code`, `dashboard_url`, `retriable`
- Idempotency policy: existing post_failed job attempt key; one send per terminal failure
- Audit policy: record one send attempt per terminal failure idempotency key.
- Fallback policy: none by default.
- Retention policy: retain metadata and redacted variable snapshots for 13 months.
- External Loops workflow audit: audit workflows listening to `post_failed` before enabling this template. Exactly one email should be sent for the same terminal publish failure.

### email.support.error_triage_follow_up.v1

- Template env: `LOOPS_ERROR_TRIAGE_USER_ACTION_TRANSACTIONAL_ID`
- Provider: Loops
- Delivery class: `service_alert`
- Owner area: Support / Admin
- Trigger: admin sends reviewed error triage draft.
- Recipient policy: admin-selected affected dashboard user.
- Required variables: `subject`, `body`, `cta_url`
- Idempotency policy: `error_triage_email:{item_id}:{recipient_scope_key}:v{draft_version}`
- Audit policy: use `error_triage_email_sends` ledger.
- Fallback policy: none.
- Retention policy: follow error triage support/audit retention policy.
- External Loops workflow audit: no workflow should auto-send this support follow-up; admin click is required.

### email.notification.test.v1

- Template env: `LOOPS_NOTIFICATION_TEST_TRANSACTIONAL_ID`
- Provider: Loops
- Delivery class: `test`
- Owner area: Notifications
- Trigger: user tests email channel.
- Recipient policy: authenticated user.
- Required variables: `recipient_name`, `settings_url`
- Idempotency policy: no suppression idempotency; allow repeated user-initiated tests
- Audit policy: record each explicit test attempt.
- Fallback policy: none by default.
- Retention policy: retain metadata for 90 days.
- External Loops workflow audit: no workflow should listen to notification test events.

### email.user.account_canceled.v1

- Template env: `LOOPS_ACCOUNT_CANCELED_TRANSACTIONAL_ID`
- Provider: Loops
- Delivery class: `critical_transactional`
- Owner area: Growth lifecycle / Account
- Trigger: user deletes UniPost account.
- Recipient policy: canceling user.
- Required variables: `canceled_at`
- Idempotency policy: `user_account_canceled:{user_id}`
- Audit policy: record one send attempt per account cancellation.
- Fallback policy: none by default.
- Retention policy: retain metadata and variable snapshots for 13 months.
- External Loops workflow audit: no cancellation workflow should send in addition to the backend account-canceled template.

## Loops Dashboard Audit Checklist

Before migrating or enabling a template, audit the Loops dashboard for workflows and transactional templates that listen to the same event or contact state.

Required audit keys:

- `user_signed_up`: confirm whether a welcome/onboarding workflow already sends the first welcome email. If yes, disable the backend Resend welcome email before enabling `email.user.welcome.v1`, or keep the workflow as the owner and do not enable the transactional welcome template.
- `post_failed`: confirm whether a workflow or transactional template already sends post failure email. Backend tests cannot see Loops dashboard workflows, so this audit must be recorded before moving `post.failed` email ownership.
- `plan_changed`: confirm whether paid activation or plan-change copy exists in a Loops workflow. The Resend paid activation email must not overlap with a Loops plan changed email.

For every audit, record:

- Loops workflow or template name.
- Event or contact property trigger.
- Whether it sends email.
- Whether it remains enabled after migration.
- Owner who approved the final single-email path.
