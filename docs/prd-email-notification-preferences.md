# PRD - Email notification preferences and unsubscribe policy

**Status:** Draft
**Owner:** Growth lifecycle / Notifications / Billing / Support
**Created:** 2026-07-02
**Target:** Give users clear, self-service control over UniPost email notifications while preserving essential transactional delivery.

---

## Problem

UniPost now sends user-facing emails through several paths, but the user-facing unsubscribe and preference model is not explicit enough.

The current admin email page shows a unified operational view of email send attempts, but the actual emails come from different sources:

- Loops transactional sends recorded in `email_send_attempts`.
- Free-plan quota reminders recorded in `free_plan_quota_email_reminders`.
- Admin-reviewed support follow-ups recorded in `error_triage_email_sends`.
- Legacy notification email rows in `unipost_notification_deliveries`, including skipped rows when Loops owns the email path.

This creates three user experience risks:

1. Users may see no clear way to stop optional product alerts, lifecycle emails, or marketing-style messages.
2. Users may believe every email can be unsubscribed through Loops, even though Loops transactional emails ignore marketing subscription state and do not include Loops unsubscribe links by default.
3. A user can currently disable an email checkbox for some notification events, but Loops-owned email paths such as `post.failed` and `account.disconnected` do not consistently check that preference before sending.

UniPost needs one understandable email preference model that maps each email to the right user control:

- true unsubscribe for marketing and lifecycle emails
- UniPost notification preferences for optional service alerts
- explanatory footer and manage-preferences link for critical transactional emails

## Product Direction

Use UniPost as the product-level authority for email preferences. Use Loops for template rendering, transactional delivery, lifecycle workflows, and marketing subscription state where appropriate.

The guiding principle is:

```text
Users manage what UniPost is allowed to send.
Loops handles how the selected email is rendered and delivered.
```

Do not treat Loops unsubscribe as the single source of truth for all email. Loops documentation states that the `subscribed` contact property does not affect transactional emails, and transactional emails do not include an unsubscribe link by default. Therefore, UniPost must make its own send/no-send decisions before calling Loops for service-alert categories.

Resolved product decision: category-level UniPost email preferences are the authoritative source for UniPost email send eligibility. The existing `unipost_notification_subscriptions` matrix should continue to control Slack and Discord delivery, but email should be migrated out of that matrix for Loops-owned notification events. A one-time backfill may seed the new category preferences from existing email-channel subscriptions; after that, the category preference is the only email on/off state.

## Goals

1. Add a clear email footer policy for every user-facing UniPost email.
2. Let users unsubscribe from emails that are genuinely unsubscribable, such as marketing and optional lifecycle sequences.
3. Route service-alert management to UniPost notification preferences, not Loops global unsubscribe.
4. Explain why critical transactional emails cannot be disabled.
5. Add durable backend preference checks before optional service-alert sends.
6. Make `/settings/notifications` the primary user-facing email preference center.
7. Keep Loops mailing lists or subscription state in sync only for lifecycle and marketing categories.
8. Preserve auditability in `/admin/email` so support can explain why a user received a message.
9. Fix the current correctness bug where disabling email delivery for `post.failed` or `account.disconnected` can fail to stop the Loops email path.

## Non-Goals

- Do not let users disable critical account, workspace, billing, security, or legal emails.
- Do not use Loops `subscribed=false` as the enforcement mechanism for transactional or service-alert emails.
- Do not build a full in-app notification center in this project.
- Do not migrate Clerk-owned authentication emails such as verification, password reset, or magic link emails.
- Do not add feature flags by default. Per UniPost rules, add flags only if explicitly requested.
- Do not change the copy or trigger logic of every template in one release unless required for footer compliance.
- Do not send marketing or promotional content inside emails classified as critical transactional.

## Current Codebase Findings

### Admin Email Visibility

`dashboard/src/app/admin/email/page.tsx` renders `/admin/email` as an operational audit page. It loads `GET /v1/admin/email-notifications`, displays email sends, and labels the page as "User-facing email attempts, Loops audit rows, and migration skip records."

`api/internal/handler/admin.go` builds the admin email list from:

- `email_send_attempts`
- `free_plan_quota_email_reminders`
- `error_triage_email_sends`
- `unipost_notification_deliveries` email-channel rows

The response includes `provider`, `delivery_class`, `event_key`, `transactional_id`, `trigger_source`, `trigger_reference_id`, and recipient snapshots. This is the right foundation for support visibility, but it does not yet expose whether the email was preference-gated, unsubscribable, or critical.

### Email Registry

`api/internal/emailregistry/registry.go` already defines the main email taxonomy:

- `critical_transactional`
- `service_alert`
- `lifecycle`
- `test`

Current registry events include:

| Event key | Current class | Current expected control |
|---|---|---|
| `email.user.welcome.v1` | `lifecycle` | Lifecycle unsubscribe / manage preferences |
| `email.workspace.member_invited.v1` | `critical_transactional` | Cannot unsubscribe |
| `email.billing.plan_changed.v1` | `critical_transactional` | Cannot unsubscribe |
| `email.billing.payment_failed.v1` | `critical_transactional` | Cannot unsubscribe |
| `email.billing.payment_recovered.v1` | `critical_transactional` | Cannot unsubscribe |
| `email.billing.subscription_canceled.v1` | `critical_transactional` | Cannot unsubscribe |
| `email.quota.free_plan_reminder.v1` | `service_alert` | Manage in UniPost if made optional; critical threshold copy may remain required |
| `email.account.disconnected.v1` | `service_alert` | Manage in UniPost notification preferences |
| `email.post.failed.v1` | `service_alert` | Manage in UniPost notification preferences |
| `email.support.error_triage_follow_up.v1` | `service_alert` | Manage support/contact preferences; admin-send safety remains required |
| `email.notification.test.v1` | `test` | User-triggered; manage preferences link only |
| `email.user.account_canceled.v1` | `critical_transactional` | Cannot unsubscribe |

The registry does not yet define preference category, unsubscribe behavior, footer policy, or send gating policy. This PRD adds those concepts.

The registry should also become the single source of truth for delivery class. Today `api/internal/loops/audit.go` re-derives delivery class from Loops lifecycle event names in a separate switch. That duplicate mapping must be consolidated into registry lookups so send gating, audit rows, and admin display cannot disagree about an event.

### Loops Sending

`api/internal/loops/client.go` sends transactional emails to Loops with:

- `transactionalId`
- `email`
- `dataVariables`
- optional idempotency key header

The backend does not pass a UniPost unsubscribe URL, manage-preferences URL, footer policy, or category metadata to templates today.

### Notification Preferences

`api/internal/handler/notifications.go` exposes user notification settings for:

- `post.failed`
- `account.disconnected`

New users receive a verified account-level email channel and default-on subscriptions through `ensureDefaultNotifications`.

The notification dispatcher now skips email rows for `post.failed` and `account.disconnected` because email delivery moved to Loops transactional templates. Slack and Discord notification delivery remain in the notification system.

Primary correctness gap: the Loops email paths for `post.failed` and `account.disconnected` are not gated by the same notification subscription state before sending. Users can toggle the email checkbox in the settings UI today, but the current Loops send paths can still send those emails unconditionally. This PRD treats that as the first behavior to fix.

The settings page currently renders a channel-by-event subscriptions matrix. Email appears as one of the channels beside Slack and Discord. The target product model changes that split:

- Email preferences move to category rows owned by UniPost email preferences.
- Slack and Discord stay in the channel-by-event matrix.
- The built-in email channel can remain visible in the Channels section for identity and test-send purposes, but it should not be an independent source of truth for service-alert email eligibility.

### Quota Reminders

`api/internal/quotaemail/service.go` sends free-plan quota reminders directly through Loops transactional email. It writes to its own quota reminder ledger and does not currently attach `loops.EmailAudit` metadata to the send. The existing quota PRD intentionally did not honor the dormant `billing.usage_80pct` notification preference because 95 percent and 100 percent warnings warn about imminent or active posting interruption.

This PRD should not blindly make every quota reminder optional. It should distinguish:

- warning and upgrade-nudge portions that can point to preferences
- block or critical limit notices that may remain required as service-critical account status notices

### Error Triage Follow-Ups

`api/internal/errortriage/send.go` sends admin-reviewed support follow-ups through Loops transactional email. These are explicitly admin-approved and user-specific. The send should continue to require admin review and safety checks, but the email should include a manage-preferences/help link so recipients can understand why they were contacted.

## Email Control Model

### Delivery Classes

| Class | User-facing meaning | User control | Enforcement owner | Footer policy |
|---|---|---|---|---|
| `critical_transactional` | Essential account, workspace, billing, security, invite, or legal email | Cannot unsubscribe | UniPost backend | Explain why it was sent; link to manage optional notifications |
| `service_alert` | Product alert tied to workspace operation or support | Can manage by category unless explicitly marked required | UniPost backend | Explain category; link to manage notifications |
| `lifecycle` | Onboarding, activation, retention, churn, education | Can unsubscribe from lifecycle/marketing category | UniPost + Loops | Include unsubscribe link and manage preferences link |
| `marketing` | Product updates, launches, newsletters, promotions | Must be unsubscribable | Loops mailing list or subscription state, mirrored in UniPost where needed | Include unsubscribe link and manage preferences link |
| `test` | Explicit user-triggered test email | No unsubscribe needed | UniPost backend | Explain user action; link to notification settings |

`marketing` is a forward-looking delivery class. No current registry event maps to it. Product update or newsletter sends must be added as separate registry events before the marketing unsubscribe path can be tested end to end.

### New Registry Fields

Extend the email registry conceptually with these fields:

| Field | Purpose |
|---|---|
| `preference_category` | Stable product category shown to users, such as `publishing_failures` or `account_connection_alerts`. |
| `can_unsubscribe` | Whether the email footer should show a one-click unsubscribe action. |
| `preference_gated` | Whether the backend must check UniPost preferences before sending. |
| `required_reason` | Short human explanation for emails that cannot be disabled. |
| `footer_policy` | One of `unsubscribe`, `manage_preferences`, `required_notice`, `required_notice_no_manage`, `test_notice`. |
| `footer_copy_policy` | Whether UniPost provides pre-rendered footer copy, a no-footer override, or template-side rendering instructions. |
| `loops_mailing_list_key` | Optional Loops mailing list or audience mapping for lifecycle/marketing categories. |
| `settings_url_scope` | Deep link target in UniPost settings. |

## Preference Categories

User-facing category names should be calm and concrete. Avoid internal event keys.

| Category key | Display label | Includes | Default | User control |
|---|---|---|---|---|
| `essential_account_billing` | Essential account and billing emails | invites, payment failures, plan changes, cancellations, account deletion | On, locked | Cannot turn off |
| `publishing_failures` | Publishing failure alerts | `email.post.failed.v1` | On | Toggle email on/off |
| `account_connection_alerts` | Account connection alerts | `email.account.disconnected.v1` | On | Toggle email on/off |
| `usage_quota_alerts` | Usage and quota alerts | quota threshold reminders | On | Toggle if product chooses optional; critical block notices may be locked |
| `support_follow_ups` | Support follow-ups | admin-reviewed triage emails | On | Manage contact/support preference; no blanket suppression of safety-critical support |
| `onboarding_tips` | Onboarding tips | welcome, activation nudges, first account/post lifecycle workflows | On | Unsubscribe |
| `product_updates` | Product updates | launches, changelog, education, promotional campaigns | Off or marketing-consent dependent | Unsubscribe |
| `test_emails` | Test emails | notification test email | User action only | Not shown as a persistent toggle |

## User Experience

### Email Footer Rules

Every UniPost-authored user-facing email should include one of these footer patterns.

#### Unsubscribable lifecycle or marketing email

Use when `footer_policy = unsubscribe`.

```text
You are receiving this because you signed up for UniPost updates and product guidance.
Unsubscribe from these emails or manage all email preferences.
```

Requirements:

- "Unsubscribe from these emails" must be a signed, no-login, one-click link scoped to the category.
- "Manage all email preferences" should open `/settings/notifications`, requiring login if needed.
- If the category is backed by Loops mailing lists, the one-click action must update Loops and UniPost state.

#### Service alert email

Use when `footer_policy = manage_preferences`.

```text
You are receiving this because email alerts are enabled for this UniPost workspace.
Manage notification preferences.
```

Requirements:

- Do not label this as a Loops unsubscribe.
- Link to the relevant category in `/settings/notifications`.
- The backend must check the UniPost preference before future optional sends.

#### Critical transactional email

Use when `footer_policy = required_notice`.

```text
This is an essential account, workspace, or billing email from UniPost and cannot be turned off.
You can manage optional notification emails in UniPost settings.
```

Requirements:

- Do not provide a one-click unsubscribe that could suppress critical emails.
- Keep the body strictly transactional.
- Link to optional notification settings for transparency.
- For `email.user.account_canceled.v1`, omit the manage-preferences link because the user likely can no longer access account settings after deletion. Use only the essential-email explanation.

#### Test email

Use when `footer_policy = test_notice`.

```text
You received this because you sent a test email from UniPost.
Manage notification preferences.
```

Requirements:

- No unsubscribe link is required.
- Link to `/settings/notifications`.

### Notification Settings Page

Update `/settings/notifications` so email controls are clear and category-based.

The page should contain:

- A locked section for essential account and billing emails.
- A section for workspace/service alerts with email toggles.
- A section for lifecycle and product emails with unsubscribe/manage controls.
- Existing Slack and Discord channel controls remain in the event-channel matrix.
- The email column is removed from the event-channel matrix for Loops-owned events once category preferences ship.
- The built-in email channel remains in the Channels section for display and test-send, but it does not provide a second toggle for `post.failed` or `account.disconnected`.

Phase 2 should include a wireframe before implementation because this is larger than changing labels in the current settings page. The current UI couples email, Slack, and Discord inside one matrix; the target UI separates email category preferences from Slack/Discord event delivery.

Example structure:

| Section | Rows |
|---|---|
| Essential | Account, workspace, and billing emails - locked on |
| Workspace alerts | Publishing failures, account connection issues, usage/quota alerts |
| Product emails | Onboarding tips, product updates |
| Channels | Built-in email identity/test, Slack webhook, Discord webhook |
| Slack and Discord subscriptions | Event-by-channel matrix without email as a competing on/off state |

When a row is locked:

- The toggle should be disabled.
- The row should explain the reason in short plain language.

When a row is optional:

- The toggle should persist immediately.
- A confirmation toast can say "Email preference updated."
- If the preference syncs to Loops, failure should show a warning but preserve UniPost as the source of truth.

### Unsubscribe Landing Page

Add a signed unsubscribe route for no-login category unsubscribe:

```text
/email/preferences/unsubscribe?t=<signed-token>
```

The page should:

- Decode recipient, category, event key, and expiry from the signed token.
- Show the category being unsubscribed.
- Confirm the change after one click, or perform immediate unsubscribe and show confirmation.
- Offer "Manage all email preferences" as the next action.
- Avoid exposing whether arbitrary email addresses exist in UniPost.

Token requirements:

- Signed with server secret.
- Scoped to recipient email or user ID, category, and event key.
- Short enough to fit email clients comfortably.
- Expiring, with a reasonable lifetime such as 30 days.
- Idempotent.

## Backend Requirements

### Preference Storage

Introduce durable email preference state that can handle category-level decisions independent of notification channels.

Recommended model:

```text
email_preferences
- id
- user_id nullable
- email normalized
- category_key
- enabled
- source
- updated_at
- created_at
```

Notes:

- Prefer `user_id` when the recipient is a UniPost dashboard user.
- Preserve `email` for invited users or recipients before user creation.
- Store one row per category.
- Keep a separate audit row for unsubscribe actions if useful for support.

The existing notification subscription tables should continue to power Slack and Discord. They may keep historical email-channel rows for audit and migration safety, but they should no longer be the authoritative source for Loops-owned email send decisions after Phase 2.

Conflict rule:

- During migration, if a category preference exists, it wins over any email-channel subscription row.
- If no category preference exists, the system may fall back to the existing email-channel subscription for a one-release compatibility window.
- After backfill and UI migration, the fallback should be removed or limited to a read-only migration path.

This prevents `email_preferences` and `unipost_notification_subscriptions` from drifting into two independent sources of truth for the same email.

### Registry-Aware Email Policy Sender

Introduce one registry-aware email policy layer before adding send gating to individual services.

This layer should:

1. Resolve event metadata from `emailregistry`.
2. Resolve footer copy and links.
3. Check UniPost email preferences when the event is preference-gated.
4. Decide whether to send, skip, or record a required-send decision.
5. Add standard footer variables to Loops `dataVariables`.
6. Write or return enough audit metadata for the source-specific ledger.

All user-facing Loops email paths should call this layer rather than each service hand-rolling preference and footer behavior:

- Loops lifecycle transactionals such as `post_failed` and `account_disconnected`.
- `quotaemail.Service`, which currently calls a `loops.Sender` directly and writes its own ledger.
- `errortriage.EmailSendService`, which currently sends through its own sender interface and writes `error_triage_email_sends`.
- Direct transactional senders for welcome, invites, notification tests, and account/billing events.

### Send Gating

Before sending an email, resolve:

1. `event_key`
2. `delivery_class`
3. `preference_category`
4. `footer_policy`
5. recipient user/email
6. whether the category is enabled

Rules:

- `critical_transactional`: send regardless of optional preference state.
- `test`: send only because the user explicitly requested it.
- `service_alert`: check UniPost preference unless the registry marks the specific event as required.
- `lifecycle`: check UniPost lifecycle preference and Loops subscription/mailing list state where configured.
- `marketing`: check marketing consent and Loops mailing list/subscription state.

If a send is skipped due to preference:

- Do not call Loops.
- Record an audit row with `status = skipped` and a reason such as `preference_disabled`.
- Show the skipped row in `/admin/email`.

### Footer Variables

All Loops templates should receive standard footer variables:

```text
manage_preferences_url
unsubscribe_url
footer_policy
footer_reason
preference_category_label
footer_text
footer_html
```

Rules:

- `unsubscribe_url` is populated only when one-click unsubscribe is valid.
- `manage_preferences_url` is populated for UniPost-authored emails when the footer includes a settings action. It may be omitted for event-specific exceptions such as account deletion, where the user likely can no longer access settings.
- UniPost should provide pre-rendered `footer_text` and `footer_html` variables as the default. Loops templates should insert these variables rather than reimplement branching logic in the Loops dashboard.
- Template-side `footer_policy` branching is allowed only as a temporary migration fallback, because Loops template edits are manual, external to the repo, and not covered by CI.

### Loops Template Rollout Dependency

Footer rendering depends on Loops dashboard template edits for each active transactional template. That work is manual and out of band.

Safe rollout order:

1. Ship backend variables while old templates ignore them.
2. Update Loops templates to render `footer_html` and plain-text equivalent content.
3. Send test messages for every active template in development or staging-equivalent Loops configuration.
4. Only then rely on footer policy acceptance criteria.

Do not make implementation correctness depend on complex template-side conditional logic. UniPost should prepare the final footer copy server-side.

### Loops Sync

Use Loops mailing lists or `subscribed` state only for lifecycle and marketing preferences.

Do not update Loops global `subscribed=false` when a user disables service alerts such as publishing failures. That could suppress lifecycle/product emails unexpectedly while still not preventing transactional sends.

Recommended mapping:

| UniPost category | Loops mapping |
|---|---|
| `onboarding_tips` | Lifecycle mailing list or workflow audience |
| `product_updates` | Product updates mailing list |
| `publishing_failures` | No Loops mailing list; UniPost gating only |
| `account_connection_alerts` | No Loops mailing list; UniPost gating only |
| `usage_quota_alerts` | No Loops mailing list in v1; UniPost gating if optional |
| `essential_account_billing` | No Loops unsubscribe mapping |

### Admin Visibility

Extend `/admin/email` data over time with:

- `preference_category`
- `footer_policy`
- `preference_decision`: `sent`, `skipped_preference_disabled`, `required`, `user_initiated`
- `unsubscribe_token_issued`: boolean or timestamp if useful

This is not one schema change because the admin email page unions heterogeneous sources. Scope the admin visibility work per source:

| Source | Required treatment |
|---|---|
| `email_send_attempts` | Add or derive preference metadata from registry and audit fields. |
| `free_plan_quota_email_reminders` | Add preference decision fields to the quota ledger or derive them from threshold/category policy. |
| `error_triage_email_sends` | Add preference decision fields to support follow-up attempts without bypassing admin safety checks. |
| `unipost_notification_deliveries` | Preserve legacy and migration skipped rows; do not treat these rows as the future source of Loops email preference decisions. |

The admin table should help support answer:

- Why did this user receive the email?
- Could the user have turned it off?
- Where can the user manage this category?
- Was it skipped because of preferences?

## Event-by-Event Policy

| Event key | Delivery class | Preference category | Footer policy | Send gating |
|---|---|---|---|---|
| `email.user.welcome.v1` | `lifecycle` | `onboarding_tips` | `unsubscribe` | UniPost + Loops lifecycle preference |
| `email.workspace.member_invited.v1` | `critical_transactional` | `essential_account_billing` | `required_notice` | Always send when invite is valid |
| `email.billing.plan_changed.v1` | `critical_transactional` | `essential_account_billing` | `required_notice` | Always send for valid billing transition |
| `email.billing.payment_failed.v1` | `critical_transactional` | `essential_account_billing` | `required_notice` | Always send for valid payment attempt |
| `email.billing.payment_recovered.v1` | `critical_transactional` | `essential_account_billing` | `required_notice` | Always send for valid recovery |
| `email.billing.subscription_canceled.v1` | `critical_transactional` | `essential_account_billing` | `required_notice` | Always send for valid cancellation |
| `email.quota.free_plan_reminder.v1` | `service_alert` | `usage_quota_alerts` | `manage_preferences` or `required_notice` by threshold | 80-95 optional if product chooses; 100 block notice may be required |
| `email.account.disconnected.v1` | `service_alert` | `account_connection_alerts` | `manage_preferences` | Check UniPost preference |
| `email.post.failed.v1` | `service_alert` | `publishing_failures` | `manage_preferences` | Check UniPost preference |
| `email.support.error_triage_follow_up.v1` | `service_alert` | `support_follow_ups` | `manage_preferences` | Check support preference, admin safety, and review state |
| `email.notification.test.v1` | `test` | `test_emails` | `test_notice` | Explicit user action only |
| `email.user.account_canceled.v1` | `critical_transactional` | `essential_account_billing` | `required_notice_no_manage` | Always send when account deletion succeeds |

## API Requirements

### User Preferences

Add APIs under the existing notifications route group:

```text
GET /v1/me/notifications/email-preferences
PUT /v1/me/notifications/email-preferences/{category_key}
```

`GET` should return:

```json
{
  "data": [
    {
      "category_key": "publishing_failures",
      "label": "Publishing failure alerts",
      "description": "Emails when a post cannot be delivered.",
      "enabled": true,
      "locked": false,
      "delivery_class": "service_alert"
    }
  ]
}
```

`PUT` should:

- reject locked categories with a clear validation error
- persist the UniPost preference
- sync Loops only for mapped lifecycle/marketing categories
- return the updated category

### Signed Unsubscribe

Add public routes:

```text
GET /v1/email-preferences/unsubscribe/preview?t=...
POST /v1/email-preferences/unsubscribe?t=...
```

The dashboard can wrap these routes in a simple public page.

## Rollout Plan

### Phase 1 - Registry and Footer Policy

- Add preference metadata to the email registry.
- Make `emailregistry` the single source of truth for delivery class and policy metadata.
- Replace or wrap hardcoded delivery-class/event-key derivation in Loops auditing with registry lookups.
- Introduce the registry-aware email policy sender.
- Generate server-rendered footer variables for Loops transactional sends.
- Ship backend variables before relying on Loops template rendering.
- Keep send behavior unchanged except for footer content.

### Phase 2 - Preferences UI and Storage

- Add category-level email preferences API.
- Add a settings-page wireframe for the split between email category preferences and Slack/Discord delivery settings.
- Update `/settings/notifications` to display category-based email controls.
- Remove email as an independent on/off column for Loops-owned events in the subscriptions matrix.
- Keep existing Slack and Discord notification settings behavior.
- Backfill default preferences for existing users from existing notification subscription state where possible.

### Phase 3 - Send Gating

- Gate `email.post.failed.v1` and `email.account.disconnected.v1` using UniPost preferences before calling Loops.
- Add skipped audit rows when preference disables a send.
- Verify `/admin/email` shows skipped-by-preference rows clearly for `email_send_attempts` and legacy notification migration rows.

### Phase 4 - Admin Visibility and Source-Specific Ledgers

- Extend admin email output per source rather than assuming one common table.
- Add or derive `preference_category`, `footer_policy`, and `preference_decision` for `email_send_attempts`.
- Decide whether quota and error-triage ledgers need new columns or derived registry metadata for v1.
- Preserve historical notification-delivery rows without making them the future source of email preference truth.

### Phase 5 - One-Click Unsubscribe

- Add signed unsubscribe URLs for lifecycle and marketing categories.
- Sync lifecycle/marketing opt-outs to Loops mailing lists or subscription state.
- Add user-facing confirmation page.
- Treat marketing as forward-looking until a `product_updates` registry event exists.

### Phase 6 - Quota and Support Policy Refinement

- Decide whether 80, 85, 90, and 95 percent quota reminders are optional.
- Keep 100 percent blocked-state notice required if product/legal agrees.
- Add support follow-up preference behavior without bypassing admin safety checks.

## Acceptance Criteria

1. Every UniPost-authored email in the registry has a footer policy.
2. Emails with `footer_policy = unsubscribe` include a working one-click unsubscribe link.
3. Emails with `footer_policy = manage_preferences` include a manage-preferences link and do not claim to be Loops-unsubscribable.
4. Emails with `footer_policy = required_notice` or `required_notice_no_manage` explain that the email is essential and cannot be turned off.
5. `email.post.failed.v1` respects the UniPost `publishing_failures` preference before sending.
6. `email.account.disconnected.v1` respects the UniPost `account_connection_alerts` preference before sending.
7. Disabled service-alert sends are recorded as skipped audit rows.
8. `/settings/notifications` shows locked essential emails and editable optional email categories.
9. The settings subscriptions matrix no longer exposes a competing email toggle for Loops-owned events after category preferences ship.
10. `/admin/email` can show why an email was sent or skipped for each source included in the implementation phase.
11. `emailregistry` is the single source for delivery class and email policy metadata.
12. Registry-aware send policy is shared by Loops lifecycle, quota, and error-triage email paths before those paths add preference gating.
13. Loops templates render server-provided footer copy rather than requiring template-side policy branching.
14. Loops unsubscribe or mailing list sync is used only for lifecycle and marketing categories.
15. Transactional billing, invite, and account emails are not suppressible through marketing unsubscribe.

## Open Product Decisions

1. Should 80, 85, 90, and 95 percent quota reminders be user-disableable, while 100 percent block notices remain required?
2. Should support follow-ups have a separate "Do not email me about support issues" preference, or should support requests be handled manually by admins?
3. Should product updates be enabled by default only for users who explicitly consent during signup/onboarding?
4. Should one-click unsubscribe require a confirmation click, or should opening the signed link unsubscribe immediately and show a confirmation page?
5. Should admin email visibility include preference state at send time in v1, or only after send gating ships?

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Users think a critical email should have an unsubscribe link | Confusion or spam complaints | Use explicit required-email footer copy and keep content strictly transactional |
| Service alerts continue to send after a user disables email | Trust loss | Enforce UniPost preference before Loops send; audit skipped rows |
| Loops global unsubscribe is used for service alerts | Preferences do not work as expected | Keep service-alert gating in UniPost and avoid Loops mailing list mapping for those categories |
| Email has two independent preference stores | User toggles drift and sends become hard to explain | Make category email preferences authoritative; remove email from the event-channel matrix for Loops-owned events |
| Registry and audit disagree on delivery class | Wrong footer or send-gating behavior | Consolidate audit and send policy lookups through `emailregistry` |
| Loops template edits lag backend deploy | Emails ship without required footer copy | Server-render footer variables, ship variables first, manually verify every active template before acceptance |
| Marketing unsubscribe suppresses lifecycle unexpectedly | Reduced onboarding effectiveness | Use category-specific mailing lists where possible |
| One-click unsubscribe tokens leak preference access | Unauthorized preference changes | Signed, scoped, expiring tokens; idempotent actions; no broad account access |
| Locked essential category feels hostile | User frustration | Explain why it is locked and link to optional notification controls |

## Implementation Notes

- The existing `emailregistry` package is the natural home for policy metadata.
- The Loops client should not need to know product policy; a higher-level email sender or registry-aware wrapper should resolve policy and footer variables before calling Loops.
- For quota reminders, `quotaemail.Service` currently sends directly through `loops.Client` and writes its own ledger; it should call the registry-aware policy layer before sending.
- For error triage, `errortriage.EmailSendService` currently sends through its own transactional sender interface and writes `error_triage_email_sends`; it should receive footer variables and preference decisions without weakening admin safety gates.
- The current `NotificationDispatcher` skipped-row behavior for Loops-owned events should be preserved for migration audit, but send gating must happen in the Loops paths too.
- Existing docs to update after implementation: `docs/email-templates.md` and `docs/prd-email-system-consolidation.md`.
