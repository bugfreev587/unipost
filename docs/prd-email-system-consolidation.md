# PRD - UniPost Email System Consolidation

**Owner:** Growth lifecycle / Notifications / Billing / Support
**Date:** 2026-06-26
**Status:** Draft
**Target:** Consolidate UniPost user-facing email events, triggers, templates, and provider ownership around Loops, while keeping Resend as a low-level sending rail and emergency fallback.

## Summary

UniPost currently has user emails split across three paths:

1. Backend-rendered `mail.Message` emails sent through `mail.Mailer`, which is backed by Resend when `RESEND_API_KEY` is configured.
2. Loops contact sync, events, and transactional emails for lifecycle paths.
3. The user notification system, which renders email templates in backend code and also supports Slack and Discord webhook channels.

This creates unclear ownership, inconsistent templates, possible duplicate sends, and fragmented subscription semantics. The target state is:

- Loops is the system of record for UniPost user contacts, lifecycle workflows, and user-facing transactional email templates.
- The UniPost backend remains the authority for trigger conditions, recipient resolution, idempotency, audit logs, and sensitive business decisions.
- Resend remains available as a low-level mail rail for non-user operational sends or emergency fallback, but should not be the normal template system for user lifecycle, billing, quota, support, or product emails.
- Contacts should not be synced from Loops to Resend unless UniPost later chooses Resend as a marketing audience system. Today, that would create duplicate contact and unsubscribe state without solving a product need.

## External Product References

The recommended direction follows common SaaS email architecture:

- Loops documents SaaS lifecycle workflows for acquisition, onboarding, retention, re-engagement, dunning, and reactivation emails, with workflows triggered by contact properties or events: https://loops.so/docs/guides/lifecycle-emails
- Loops transactional email is API-triggered and template-based, with data variables supplied by the application: https://loops.so/docs/transactional
- Customer.io describes transactional messages as messages users implicitly expect, such as receipts and password resets, and separates those from broader marketing preferences: https://docs.customer.io/journeys/send/transactional/api/
- Postmark's transactional email examples include welcome, invitation, password reset, receipts, payment failure, and notification emails as core SaaS/application patterns: https://postmarkapp.com/blog/transactional-email-examples
- Resend's Send Email API sends directly to recipient addresses and does not require a contact record for simple email delivery: https://resend.com/docs/api-reference/emails/send-email
- Resend audiences are a separate marketing/contact-management surface; UniPost should not create a second audience source unless product ownership moves there: https://resend.com/docs/dashboard/audiences/introduction

## Goals

1. Produce one source-of-truth inventory of all user-facing emails, events, triggers, providers, templates, and gaps.
2. Define a target event taxonomy that separates lifecycle, transactional, billing, quota, product-alert, support, and notification-channel concerns.
3. Move user-facing email rendering into Loops templates wherever practical.
4. Keep backend-owned trigger logic, idempotency, auditability, and safety checks in UniPost.
5. Remove or prevent duplicate-capable email paths, especially for `post.failed`, welcome, and plan upgrade events where duplicate user sends depend on Resend, Loops template IDs, and Loops dashboard workflow configuration.
6. Define missing events that should be added for a complete SaaS email lifecycle.
7. Provide a phased migration plan that can ship safely through the normal UniPost dev -> staging -> production flow.

## Non-Goals

- Do not migrate Clerk-owned authentication emails such as email verification, password reset, or magic links.
- Do not sync Loops contacts into Resend.
- Do not build a new in-app notification center in this project.
- Do not replace Slack or Discord notification-channel delivery with Loops.
- Do not use Loops workflows for sensitive business decisions such as quota eligibility, plan entitlement, payment status, or support-email safety checks.
- Do not add feature flags by default. Per UniPost rules, add flags only when explicitly requested.

## Current Provider Roles

### Resend

Current code path:

- `api/internal/mail/resend.go` implements `ResendMailer`.
- `api/cmd/api/main.go` initializes `mail.Mailer` from `RESEND_API_KEY`.
- Backend handlers render HTML/text directly and call `mailer.Send`.
- The notification worker renders event-specific email copy in `api/internal/worker/notification.go` and sends it through `mail.Mailer`.

Current role:

- Low-level delivery rail for backend-rendered emails.
- No user contact sync.
- No centralized user email templates.
- No lifecycle workflow ownership.

### Loops

Current code path:

- `api/internal/loops/client.go` supports contact upsert, event send, and transactional email send.
- `api/internal/loops/syncer.go` gates lifecycle sync through `email.loops_integration_v1`.
- `api/cmd/api/main.go` wires template IDs for plan changed, account canceled, post failed, free-plan quota reminders, and admin error triage sends.

Current role:

- Contact system for dashboard users.
- Lifecycle events such as `user_signed_up`.
- Transactional emails for selected lifecycle, quota, and admin-support paths.

### Notification System

Current code path:

- `api/internal/events/bus.go` defines product/webhook event names.
- `api/internal/handler/notifications.go` exposes user settings and supported notification events.
- `api/internal/worker/notification.go` fans out notifications to email, Slack webhook, and Discord webhook channels.

Current role:

- Multi-channel alerting for configured notification events.
- Email channel currently uses backend-rendered templates through Resend.
- Slack and Discord are legitimate non-email channels and should remain in this system.

## Current Email Inventory

| Email / Event | Trigger | Current Platform | Current Template Source | Current Status | Target Owner |
|---|---|---:|---|---|---|
| Welcome email | Clerk `user.created` webhook after workspace creation | Resend | Backend `renderWelcomeEmail` | Active | Loops workflow or transactional template |
| Loops signup event | Clerk `user.created` / `user.updated` webhook | Loops | Event `user_signed_up`; workflow depends on Loops config | Active contact/event sync | Loops lifecycle workflow |
| Workspace invite | UniPost workspace admin creates a `workspace_invites` row | Resend | Backend `sendInviteEmail` | Active; UniPost-owned invite path, not Clerk organization invite mail | Loops transactional template |
| Paid activation | Stripe checkout from free or no plan to paid plan | Resend | Backend `renderPaidActivationEmail` | Active | Merge into Loops `billing.plan_changed` or a dedicated paid activation template |
| Plan changed | Stripe checkout/subscription update with changed plan | Loops if template ID exists; else Loops event | `LOOPS_PLAN_CHANGED_TRANSACTIONAL_ID` or event `plan_changed` | Active, but can duplicate paid activation | Loops transactional template |
| Account canceled | User deletes UniPost account | Loops if template ID exists; else Loops event | `LOOPS_ACCOUNT_CANCELED_TRANSACTIONAL_ID` or event `user_account_canceled` | Active | Loops transactional template, with policy review |
| Free plan quota reminder | Publish, schedule, or block path evaluates quota thresholds | Loops | `LOOPS_FREE_PLAN_QUOTA_REMINDER_TRANSACTIONAL_ID` | Active | Keep Loops transactional, add registry/audit consistency |
| Post failed notification | Publish/worker failure reaches terminal failure path | Resend notification system plus Loops `post_failed` lifecycle path when configured | Backend `renderEmail("post.failed")`; `LOOPS_POST_FAILED_TRANSACTIONAL_ID` or Loops event/workflow | Duplicate-capable depending on env vars and Loops dashboard config | Single Loops email path; notification system keeps Slack/Discord only |
| Account disconnected notification | Manual disconnect or token refresh failure | Resend notification system for email; Slack/Discord also supported | Backend `renderEmail("account.disconnected")` | Active | Loops email template plus notification Slack/Discord |
| Billing usage 80 percent | Notification catalog includes `billing.usage_80pct` | Resend if published | Backend `renderEmail("billing.usage_80pct")` | Dormant / legacy, superseded by quota reminder service | Remove or hide email event; map usage emails to quota service |
| Billing payment failed | Stripe `invoice.payment_failed` | Resend notification system | Backend `renderEmail("billing.payment_failed")` | Active | Loops dunning transactional or workflow-backed template |
| Notification test email | User tests email channel in settings | Resend | Backend `TestChannel` email | Active | Loops transactional test template or email gateway test path |
| Admin error triage follow-up | Admin explicitly sends reviewed customer email | Loops | `LOOPS_ERROR_TRIAGE_USER_ACTION_TRANSACTIONAL_ID` | Active/design-backed | Keep Loops transactional and audited |

## Main Problems

### Duplicate or Competing Sends

`post.failed` is not an unconditional duplicate send in source code. The backend notification path publishes `post.failed` and can send a Resend-backed notification email. Separately, the Loops lifecycle path emits `post_failed` and sends a Loops transactional email when `LOOPS_POST_FAILED_TRANSACTIONAL_ID` is configured, or may trigger a Loops-dashboard workflow when configured there. This makes `post.failed` duplicate-capable depending on environment and Loops dashboard configuration.

Welcome has a similar risk: the backend sends `renderWelcomeEmail` through Resend after `user.created`, while the Loops path also emits `user_signed_up`, which can power a Loops onboarding workflow. Plan upgrades can send a Resend paid activation email and a Loops plan changed email when both paths are configured. These need one authoritative email-send decision per event plus a documented Loops-side workflow audit.

### Templates Are Scattered

Some user emails are hardcoded in handlers, some are hardcoded in the notification worker, and some are Loops templates. This makes copy changes, QA, localization, and brand consistency harder than necessary.

### Event Names Are Not Unified

UniPost currently has:

- Public webhook events such as `post.failed`.
- Notification setting events such as `billing.usage_80pct`.
- Loops event names such as `post_failed` and `plan_changed`.
- Environment variables such as `LOOPS_POST_FAILED_TRANSACTIONAL_ID`.

These are related, but not governed by a single email registry.

### Subscription Semantics Are Mixed

Some emails are transactional and expected even if marketing is unsubscribed. Some are product alerts that should respect UniPost notification settings. Some lifecycle emails should respect marketing subscription state in Loops. The current implementation does not make this distinction explicit.

## Target Architecture

### 1. Email Event Registry

Introduce a backend-owned registry for all user-facing emails. This can start as code, not a database table.

Each registry entry should define:

- `event_key`: canonical email event key, for example `email.billing.payment_failed.v1`.
- `domain`: `user`, `workspace`, `billing`, `quota`, `publishing`, `account`, `support`, `notification`.
- `trigger_source`: webhook, handler, worker, scheduled job, admin action, or Loops workflow.
- `provider`: default `loops`.
- `template_id_env`: the expected Loops transactional template ID environment variable when applicable.
- `external_loops_config`: expected Loops workflow or transactional-template behavior configured outside the repo.
- `delivery_class`: `critical_transactional`, `service_alert`, `lifecycle`, `marketing`, or `test`.
- `recipient_policy`: workspace owner, invited email, acting user, affected user, or admin-selected recipient.
- `idempotency_policy`: deterministic key format.
- `data_contract`: required and optional variables sent to Loops.
- `audit_policy`: whether to write a durable send attempt row.
- `fallback_policy`: none, retry, or Resend emergency fallback.
- `retention_policy`: how long subject snapshots, variable snapshots, and provider errors are retained.

### 2. Loops as User Email Template Layer

Use Loops for:

- Transactional templates that require exact API-triggered sends.
- Lifecycle workflows where Loops can own timing and branching.
- Contact properties that power segmentation and workflows.

The backend must still compute sensitive conditions before calling Loops. Examples:

- Whether a quota threshold was crossed.
- Whether a payment failure should notify the workspace owner.
- Whether a post failure is terminal.
- Whether a support follow-up is safe and admin-approved.

### 3. Resend as Low-Level Rail Only

Keep Resend for:

- Emergency fallback if a critical transactional template is unavailable and fallback is explicitly enabled.
- Non-user operational delivery if needed.
- Future provider flexibility behind an internal mail interface.

Do not use Resend contacts/audiences unless UniPost intentionally moves marketing/audience ownership from Loops to Resend.

### 4. Notification System Split

Keep the notification system for:

- Slack webhook channel.
- Discord webhook channel.
- User-configurable multi-channel alert subscriptions.

Move user email rendering out of `worker/notification.go`. Email deliveries for notification events should call the email registry and Loops template path, or be disabled when the same event already has a lifecycle email path.

## Target Event Taxonomy

### Transactional Templates

| Event Key | Trigger | Recipient | Loops Template Env Var | Required Variables | Idempotency Key |
|---|---|---|---|---|---|
| `email.user.welcome.v1` | Clerk `user.created` after workspace creation | New user | `LOOPS_USER_WELCOME_TRANSACTIONAL_ID` | `recipient_name`, `workspace_name`, `app_url`, `connect_url`, `discord_url` | `user_welcome:{user_id}` |
| `email.workspace.member_invited.v1` | Workspace invite created | Invited email | `LOOPS_WORKSPACE_MEMBER_INVITED_TRANSACTIONAL_ID` | `workspace_name`, `role`, `accept_url`, `expires_at` | `workspace_invite:{invite_id}` |
| `email.billing.plan_changed.v1` | Stripe checkout/subscription update changes plan | Workspace owner | `LOOPS_PLAN_CHANGED_TRANSACTIONAL_ID` | `workspace_name`, `old_plan_id`, `new_plan_id`, `change_type`, `billing_url` | Existing plan change key |
| `email.billing.payment_failed.v1` | Stripe `invoice.payment_failed` | Workspace owner | `LOOPS_BILLING_PAYMENT_FAILED_TRANSACTIONAL_ID` | `workspace_name`, `plan_id`, `billing_url`, `retry_message`, `attempt_count`, `next_payment_attempt` | `billing_payment_failed:{invoice_id}:{attempt_count}` |
| `email.billing.payment_recovered.v1` | Stripe payment succeeds after past-due status | Workspace owner | `LOOPS_BILLING_PAYMENT_RECOVERED_TRANSACTIONAL_ID` | `workspace_name`, `plan_id`, `billing_url` | `billing_payment_recovered:{invoice_id}` |
| `email.billing.subscription_canceled.v1` | Stripe subscription canceled or user cancels paid plan | Workspace owner | `LOOPS_BILLING_SUBSCRIPTION_CANCELED_TRANSACTIONAL_ID` | `workspace_name`, `plan_id`, `effective_at`, `billing_url` | `billing_subscription_canceled:{subscription_id}:{effective_at}` |
| `email.quota.free_plan_reminder.v1` | Free workspace crosses 80/85/90/95/100 percent threshold | Workspace owner | `LOOPS_FREE_PLAN_QUOTA_REMINDER_TRANSACTIONAL_ID` | Existing quota variables | Existing quota key |
| `email.account.disconnected.v1` | Manual disconnect or token refresh permanently fails | Workspace owner | `LOOPS_ACCOUNT_DISCONNECTED_TRANSACTIONAL_ID` | `workspace_name`, `platform`, `account_name`, `reconnect_url`, `reason` | `account_disconnected:{social_account_id}:{event_source}` |
| `email.post.failed.v1` | Terminal publish failure after retry policy | Workspace owner | `LOOPS_POST_FAILED_TRANSACTIONAL_ID` | `workspace_name`, `post_id`, `platform`, `error_code`, `dashboard_url`, `retriable` | Existing post failed key, one send per terminal failure |
| `email.support.error_triage_follow_up.v1` | Admin clicks send after reviewing AI-generated draft | Admin-selected affected dashboard user | `LOOPS_ERROR_TRIAGE_USER_ACTION_TRANSACTIONAL_ID` | `subject`, `body`, `cta_url` | Existing triage key |
| `email.notification.test.v1` | User tests email channel | Authenticated user | `LOOPS_NOTIFICATION_TEST_TRANSACTIONAL_ID` | `recipient_name`, `settings_url` | No suppression idempotency; allow repeated user-initiated tests |

Billing dunning intent:

- `email.billing.payment_failed.v1` should send at most once per Stripe invoice collection attempt, not once per webhook replay and not only once per subscription lifetime. Use Stripe invoice ID plus invoice attempt count when available. If attempt count is unavailable, use invoice ID plus `next_payment_attempt` or Stripe event ID as a fallback and document the behavior before implementation.
- `email.billing.payment_recovered.v1` should send once per invoice after UniPost observes recovery from a prior failed or `past_due` state. A replayed Stripe success webhook must not resend the recovery email.
- `email.billing.plan_changed.v1` should suppress overlapping paid activation copy. If the Loops template includes paid activation content, remove the Resend paid activation email for that transition.
- `email.notification.test.v1` should not use a coarse time bucket that blocks repeat tests. It is explicitly user-initiated, so repeated clicks should produce repeated test emails while still logging attempts.

### Lifecycle Workflows

Loops workflows should own these non-critical lifecycle sequences after the backend updates contact properties or sends product events:

| Workflow | Backend Trigger | Loops Trigger | Purpose |
|---|---|---|---|
| New user onboarding | Contact created and `user_signed_up` sent | Contact added or `user_signed_up` event | Welcome/onboarding sequence over first 30 days |
| Activation nudge | No connected account within N days | Contact property `activation_state=needs_account` | Encourage first account connection |
| First account connected | `account.connected` for user's first account | Event `first_account_connected` | Reinforce next step: create first post |
| First post published | First successful post | Event `first_post_published` | Reinforce value and suggest scheduling/API usage |
| Upgrade upsell | Free plan usage or repeated activation milestone | Contact property or event | Promote paid plan at natural moments |
| Reconnect reminder | Account disconnected and not reconnected after delay | Contact property/event plus workflow delay | Remind user to reconnect without spamming |
| Churn feedback | Account or subscription cancellation | Contact property `subscriptionStatus=Canceled` or event | Ask for feedback and offer help |
| Re-engagement | Inactive user after N days | Contact property `last_active_at` or segment | Bring dormant users back |

## Missing Events to Add

### High Priority

1. `email.user.welcome.v1`
   - Current welcome email is Resend/backend-rendered.
   - Move to Loops so copy and onboarding can be managed in one place.
   - Migration must audit and disable overlapping Loops `user_signed_up` welcome workflows or the backend Resend welcome path before enabling the new Loops welcome template.

2. `email.workspace.member_invited.v1`
   - Current invite email is Resend/backend-rendered.
   - This is a classic transactional email and should use a Loops template.
   - The current invite path is UniPost-owned (`workspace_invites` plus an accept URL). If Clerk organization invitations are introduced later, this event must be revisited to avoid double invite emails.

3. `email.billing.payment_failed.v1`
   - Current payment failure is a Resend notification email.
   - Move to Loops and model it as dunning.

4. `email.account.disconnected.v1`
   - Current email is rendered through notification worker.
   - Move email to Loops, preserve Slack/Discord notification channels.

5. `email.post.failed.v1` single-owner decision
   - Keep only one customer email path.
   - Recommended: Loops transactional template for email, notification system for Slack/Discord.
   - Audit Loops dashboard workflows and transactional-template configuration for `post_failed`; backend tests alone cannot prove that an external Loops workflow will not also send.

6. `email.billing.payment_recovered.v1`
   - Missing today.
   - Useful after a failed payment is resolved so the customer knows no action is needed.

### Medium Priority

7. `email.billing.subscription_canceled.v1`
   - Distinguish paid subscription cancellation from full UniPost account deletion.
   - Current `user_account_canceled` only covers user deletion behavior.

8. `lifecycle.first_account_connected`
   - Missing as a lifecycle event.
   - Useful to drive activation workflows in Loops.

9. `lifecycle.first_post_published`
   - Missing as a lifecycle event.
   - Reinforces activation and can prompt advanced features.

10. `lifecycle.onboarding_incomplete`
   - Missing today.
   - Can be implemented with contact properties such as `has_connected_account`, `has_published_post`, and `last_active_at`.

11. `email.notification.test.v1`
   - Current test email is backend-rendered through Resend.
   - Migrating it verifies the real Loops transactional path for user emails.

### Lower Priority

12. Security-sensitive account events
   - Examples: API key created, API key deleted, webhook secret rotated, workspace member role changed.
   - These are valuable but should be designed separately because they may need stronger audit and security copy.

13. Scheduled digest
   - Examples: weekly publishing summary or account health digest.
   - Loops has a scheduled digest pattern, but UniPost should only add this after core transactional consolidation.

14. Product update broadcasts
   - Use Loops campaigns/broadcasts, not transactional emails.
   - Requires clear marketing consent and unsubscribe behavior.

## Subscription and Preference Policy

| Delivery Class | Examples | User Preference Behavior | Provider Surface |
|---|---|---|---|
| `critical_transactional` | Invite, payment failed, quota blocked, account disconnected, account security | Sent regardless of marketing subscription when legally/product-appropriate | Loops transactional |
| `service_alert` | Post failed, quota warning, reconnect reminder | Default on; should respect UniPost notification settings where possible | Loops transactional plus UniPost settings |
| `lifecycle` | Welcome sequence, onboarding nudges, activation, churn feedback | Respect Loops subscription status and lifecycle audience rules | Loops workflow |
| `marketing` | Product updates, launch campaigns, newsletters | Requires marketing subscription and unsubscribe support | Loops campaign/broadcast |
| `test` | Notification test email | Explicit user action, not a marketing message | Loops transactional |

Transactional classification must not be used to send promotional content. Upgrade prompts can appear in quota emails when directly related to avoiding service interruption, but broader promotional upsell should live in lifecycle/marketing workflows.

## Data and Audit Requirements

Create or evolve an email send audit model that can eventually cover all user-facing emails.

Minimum fields:

- `id`
- `event_key`
- `recipient_user_id`
- `recipient_email`
- `workspace_id`
- `provider`
- `provider_template_id`
- `idempotency_key`
- `delivery_class`
- `status`: `pending`, `sent`, `failed`, `skipped`
- `subject_snapshot`
- `data_variables_snapshot`
- `trigger_source`
- `trigger_reference_id`
- `attempt_count`
- `last_error`
- `created_at`
- `attempted_at`
- `sent_at`

Existing ledgers such as `free_plan_quota_email_reminders`, `error_triage_email_sends`, and `notification_deliveries` can remain initially. The PRD target is a consistent audit pattern, not necessarily a single table in the first migration.

Snapshots can contain PII such as recipient names, workspace names, support context, subject lines, and CTA URLs. Implementations must define retention and redaction rules before storing broad `data_variables_snapshot` payloads. Support and triage email bodies should follow the stricter retention policy already planned for error triage artifacts.

## Template Contract Requirements

Each Loops transactional template must have a documented contract:

- Template ID environment variable.
- Event key.
- Owning product area.
- Required variables.
- Optional variables.
- Subject ownership: Loops template subject or backend-supplied subject.
- CTA URL source.
- Idempotency key format.
- Whether the template may include marketing content.
- Whether a user can disable this class of email.
- Production rollback action.
- External Loops workflow dependencies, including whether a workflow listens for the same event and could send an additional email.

All template IDs should be documented in `docs/feature-flags-unleash.md` or a new `docs/email-templates.md`. Since these are configuration dependencies rather than feature flags, a dedicated `docs/email-templates.md` is cleaner.

## Migration Plan

### Phase 1 - Registry and Inventory

1. Add a backend email event registry with the target event keys and template ID environment names.
2. Add `docs/email-templates.md` with each template contract.
3. Add tests that fail if a configured event is missing a template contract or owner.
4. Add tests that fail if a `LOOPS_*_TRANSACTIONAL_ID` variable wired in `api/cmd/api/main.go` has no registry entry or template contract.
5. Add an explicit Loops dashboard configuration audit checklist for workflows that listen to `user_signed_up`, `post_failed`, `plan_changed`, or other lifecycle events.
6. Do not change sending behavior yet.

### Phase 2 - Move Simple Resend Emails to Loops

1. Migrate welcome email to `email.user.welcome.v1`.
2. Migrate workspace invite email to `email.workspace.member_invited.v1`.
3. Migrate notification test email to `email.notification.test.v1`.
4. Keep Resend fallback off by default unless explicitly configured.

### Phase 3 - Billing Consolidation

1. Merge paid activation and plan changed behavior into one Loops-owned flow.
2. Add `email.billing.payment_failed.v1`.
3. Add `email.billing.payment_recovered.v1`.
4. Add `email.billing.subscription_canceled.v1`.
5. Ensure Stripe-triggered payment failure emails are idempotent by invoice attempt, while recovery emails are idempotent by invoice recovery.

### Phase 4 - Product Alert De-Duplication

Prerequisite: the normalized email audit path from Phase 6, or an event-specific durable ledger with equivalent fields, must exist before disabling email fanout from `notification_deliveries`. Otherwise UniPost loses per-recipient email observability while moving the send from the notification worker to Loops.

1. Move account disconnected email to Loops.
2. Move post failed email to one Loops path.
3. Preserve Slack/Discord notification behavior in the notification system.
4. Disable or remove email channel fanout for events whose email path has moved to Loops.
5. Hide or relabel legacy `billing.usage_80pct` in notification settings.

### Phase 5 - Lifecycle Workflows

1. Add backend contact properties for activation state.
2. Emit Loops events for first account connected and first post published.
3. Create Loops workflows for onboarding, activation nudges, reconnect reminders, and churn feedback.
4. Keep lifecycle workflows out of critical service delivery paths.

### Phase 6 - Audit and Admin Visibility

1. Normalize send audit rows across migrated email events.
2. Add admin views or filters for recent email sends by event key, recipient, provider, and status.
3. Include provider failure reasons and idempotency keys for support debugging.

## Rollout and Validation

For each migrated email:

1. Create and publish the Loops transactional template in development.
2. Configure the development template ID.
3. Trigger the email locally or in development using a test user.
4. Verify the email renders with correct variables and links.
5. Verify idempotency by repeating the trigger.
6. Verify no duplicate Resend email is sent for the same event.
7. Verify Loops dashboard workflows do not send a second email for the same canonical event.
8. Promote through staging and production using the standard UniPost release flow when implementation begins.

## Acceptance Criteria

1. Every user-facing email has a documented event key, trigger, provider, template, recipient policy, idempotency policy, and preference class.
2. No user-facing email is hardcoded in arbitrary handlers after migration, except for explicit fallback paths.
3. Welcome, invite, billing, quota, post failure, account disconnected, and support follow-up emails use Loops templates.
4. Resend contacts are not synced.
5. `post.failed` has exactly one email owner after migration, and both repository configuration and Loops dashboard configuration are documented so it cannot send both Resend and Loops emails.
6. Paid plan activation has exactly one email owner after migration, and Loops template/workflow configuration is audited so it cannot send overlapping paid activation and plan changed emails.
7. Slack and Discord notification channels continue to work for supported events.
8. Legacy `billing.usage_80pct` does not create duplicate quota emails.
9. All migrated emails have deterministic idempotency keys.
10. Stripe dunning sends are idempotent by invoice attempt, while Stripe recovery sends are idempotent by invoice recovery.
11. Notification email migration does not remove per-recipient delivery observability; either a normalized audit row or equivalent event ledger exists before legacy email fanout is disabled.
12. Admin/support can inspect failed user-email sends with provider error details.

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Missing Loops template ID disables a critical email | Users miss important notices | Fail closed with clear logs/admin errors before production enablement; keep explicit fallback only for critical templates |
| Duplicate sends during migration | Users receive confusing emails | Migrate one event at a time and add tests for disabled legacy path |
| Marketing copy leaks into transactional templates | Compliance and trust risk | Classify each template and review content before production |
| Loops unsubscribe semantics are misunderstood | Users receive unexpected lifecycle emails | Separate transactional, service alert, lifecycle, and marketing classes |
| Notification settings no longer match email behavior | User confusion | Update settings UI/API labels as each event migrates |
| Provider outage | Critical emails delayed | Retry with idempotency; define explicit emergency fallback for critical transactional classes |

## Implementation Notes

- The first implementation should be a narrow registry and documentation pass before moving sends.
- Avoid large refactors of notification channels until the email registry exists.
- Keep Loops workflow sends and backend transactional sends separate in naming and documentation.
- Add tests around `post.failed` and paid activation to prevent duplicate email sends.
- Do not introduce a feature flag unless explicitly requested.

## Source Code References

- Resend mailer: `api/internal/mail/resend.go`
- Mailer wiring: `api/cmd/api/main.go`
- Loops client: `api/internal/loops/client.go`
- Loops syncer: `api/internal/loops/syncer.go`
- Welcome email: `api/internal/handler/webhooks.go`
- Workspace invite email: `api/internal/handler/members.go`
- Paid activation email: `api/internal/handler/stripe_webhook.go`
- Notification catalog: `api/internal/handler/notifications.go`
- Notification email renderer: `api/internal/worker/notification.go`
- Free plan quota email service: `api/internal/quotaemail/service.go`
- Error triage email sender: `api/internal/errortriage/send.go`
- Public/internal event names: `api/internal/events/bus.go`
