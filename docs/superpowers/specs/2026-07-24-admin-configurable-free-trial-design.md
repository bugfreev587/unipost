# Admin-Granted Configurable Free Trials

**Status:** Revised after integration review; awaiting written-spec re-review
**Date:** 2026-07-24
**Owner areas:** Billing, Admin, Dashboard, Lifecycle Email
**Branch:** `dev-admin-configurable-free-trial`
**Base:** `origin/staging` at `2c488080a2598d1b88134a1d265c1ae5a0352dba`

## Summary

UniPost administrators need to grant a workspace a manually approved, time-limited trial of a self-serve paid plan. The administrator chooses the duration from 1 through 730 whole days. Trial behavior depends on the workspace's current billing state:

- A Free workspace receives a pending offer for a selected API, Basic, Growth, or Team plan. The trial starts only after the user chooses that exact plan from either the public Pricing page or Settings → Billing and completes Stripe Checkout with a payment method.
- An actively paying workspace receives a future trial of its current plan only. The paid period continues unchanged, the trial begins when that period ends, and normal billing of the same plan resumes when the trial ends.

All trial states are visible in Admin Billing. Pending, Checkout-in-progress, scheduled, and active states are also visible on the matching plan card on both user upgrade surfaces. Settings → Billing includes a durable Trial History. Stripe owns the billing timeline; UniPost owns grant authorization, product presentation, audit history, and lifecycle email delivery.

## Goals

1. Let an administrator grant a configurable 1–730 day trial.
2. Support API, Basic, Growth, and Team; do not support Enterprise.
3. Preserve the current paid period before inserting a same-plan paid-user trial.
4. Require a Free user to select the granted plan and provide a payment method before trial activation.
5. Start billing automatically when a trial ends unless the user cancels renewal.
6. Let users change plans while a trial is scheduled or active; doing so forfeits the remaining trial and starts a new paid billing period immediately.
7. Send one trial-ending billing email three days before the end, or as soon as possible for trials shorter than three days.
8. Show current trial state consistently on Pricing and Settings → Billing, and show Trial History on Settings → Billing.
9. Preserve a complete admin audit trail and make all Stripe and webhook operations idempotent.

## Non-goals

- User-discoverable or automatically granted trials.
- Trials without administrator approval.
- Trials for Enterprise or a custom/offline contract.
- Upgrading or downgrading a paid user as part of the grant itself.
- Carrying unused trial time to another plan.
- Crediting or prorating unused trial time when a user changes plans.
- A global feature flag. This feature is manual-admin-only by default and follows the repository rule not to add a flag unless explicitly requested.
- Supporting multiple workspaces per account as a separate product concept. Grants use the same workspace dimension as Billing.

## Product Rules

### Eligible plans and duration

- `plan_id` must be one of `api`, `basic`, `growth`, or `team`.
- `duration_days` must be an integer from 1 through 730 inclusive.
- Enterprise is excluded because it is not a self-serve Stripe card plan.
- A workspace can have only one non-terminal trial grant at a time.
- A workspace can receive another grant after an earlier grant reaches a terminal state. Repeated grants are admin-only and remain fully audited.

### Free workspace

1. The administrator selects the target paid plan and duration.
2. UniPost creates a `pending_activation` grant. The workspace remains Free and the trial clock does not start.
3. Pricing and Settings → Billing show the offer only on the granted target plan.
4. The user selects that plan and completes Stripe Checkout with a payment method.
5. Checkout creates a Stripe subscription with the approved trial duration and metadata linking the subscription to the grant.
6. The successful Stripe event moves the grant to `active`, records exact start/end timestamps, and applies the target plan's entitlements.
7. At trial end, Stripe starts a normal paid billing period for the target plan unless renewal was canceled.
8. If the user buys a different plan before activating the offer, the pending grant becomes `superseded` after the other purchase succeeds.
9. An unactivated grant does not expire automatically. It remains pending until activated, revoked by an administrator, or superseded by another successful purchase.

### Actively paying workspace

1. The workspace must have an active self-serve Stripe subscription, must not be set to cancel, and must not have an unrelated Stripe Subscription Schedule.
2. The administrator can choose only the current paid plan and the trial duration.
3. UniPost attaches a Stripe Subscription Schedule to the current subscription:
   - the existing paid phase continues through `current_period_end`;
   - a same-price trial phase starts at `current_period_end` and ends after the approved number of days;
   - a paid phase of the same plan follows the trial;
   - the schedule releases the subscription after the resumed paid phase so ordinary recurring billing continues.
4. No proration or invoice is created when the administrator schedules the trial.
5. The grant is `scheduled` until Stripe enters the trial phase, then `active`, then `completed` when normal paid billing resumes.

Stripe Subscription Schedules are the selected mechanism because phases preserve the already-paid period and represent the future trial and paid resumption explicitly. Directly setting `trial_end` on the existing subscription was rejected because it can make the subscription appear trialing immediately rather than after the paid period.

### Existing billing compatibility requirements

The implementation must deliberately change two verified behaviors in the existing Stripe webhook projection:

1. `isTrialCancellation` currently treats every `trialing && cancel_at_period_end` subscription as an abandoned legacy trial, immediately calls `CancelSubscription`, and downgrades the workspace to Free. That behavior must become grant-aware. When the Stripe subscription carries a matching `trial_grant_id`, the grant is `active`, and Stripe's `trial_end` matches the grant, `cancel_at_period_end` means **renewal canceled, access retained through `ends_at`**. The webhook updates the subscription/grant projection but must not immediately cancel or downgrade it. The existing immediate-downgrade behavior remains only for trialing subscriptions that are not owned by this grant system.
2. `checkout.session.completed` currently writes local subscription status as `active` without reading the created Stripe subscription. Trial Checkout creates a `trialing` subscription. The handler must retrieve the referenced Stripe subscription using the already-verified Stripe mode and project its real `status`, price, period, `trial_start`, `trial_end`, cancellation fields, and metadata. A retrieval failure returns a retryable webhook error; the handler must never invent `active`.

The shared Stripe subscription projector must also accept `customer.subscription.created` and `customer.subscription.updated`. It correlates by `trial_grant_id` metadata first, then indexed Stripe IDs, and can recover when those events arrive before or after `checkout.session.completed`.

Entitlement gates have been verified to use the workspace subscription's `plan_id`, not `status`. Therefore an admin-managed subscription may correctly remain `trialing` while receiving the target paid plan's entitlements. Tests must lock in that behavior. The legacy `subscriptions.trial_used` field currently has no runtime billing-policy caller and remains compatibility-only.

### Plan changes while a grant is scheduled or active

- Users may upgrade or downgrade from either Pricing or Settings → Billing.
- A plan change during `scheduled` or `active` forfeits the trial immediately.
- For `free_to_paid`, UniPost makes one Subscription update that replaces the item price, sets `trial_end=now`, sets `billing_cycle_anchor=now`, disables proration, and requires successful immediate payment before applying the update.
- For `paid_same_plan`, UniPost does **not** release the attached schedule first. It makes one Subscription Schedule update that replaces the remaining schedule with a paid target-plan phase starting now, sets `billing_cycle_anchor=phase_start`, removes the trial, disables proration, and keeps `end_behavior=release`. Stripe recommends modifying an attached schedule through the Schedule API because direct Subscription changes can otherwise be overwritten by a later phase.
- The user is charged for a complete new billing period of the selected plan. There is no credit, proration, or transfer of remaining trial days.
- The grant becomes `superseded` only after Stripe webhook projection confirms the target plan and the immediate billing transition. If the single Stripe mutation fails, the existing trial timeline remains authoritative and the endpoint returns a retryable error. If Stripe succeeds but the local response/update fails, metadata-driven webhook reconciliation completes the transition.
- Both plan-selection surfaces must show a confirmation warning: changing plans ends the trial and starts billing immediately.

### Cancellation

- A user can cancel renewal while a trial is scheduled or active.
- Cancellation preserves access through the approved trial end and prevents the post-trial paid phase.
- At trial end the Stripe subscription cancels and UniPost returns the workspace to Free.
- Cancellation dispatches by grant kind:
  - `free_to_paid`: set the trialing Subscription to cancel at the trial-period end. The grant-aware webhook guard records canceled renewal without immediately downgrading access.
  - `paid_same_plan`: update the Subscription Schedule, preserve the current paid/trial phases, remove the resumed paid phase, and set schedule `end_behavior=cancel`. Do not emulate this path with Subscription `cancel_at_period_end`.
- During a scheduled or active trial, the Billing UI routes cancellation through a trial-aware UniPost endpoint. A portal session shown for those users must not expose a competing subscription-cancellation path that could cancel before the approved trial end.
- Create a dedicated Stripe Customer Portal configuration for admin-managed trials. It keeps payment-method management and invoice history enabled but disables both subscription cancellation and subscription updates. `CreatePortal` selects this configuration whenever a grant is `scheduled` or `active`; ordinary customers continue using the default configuration. The configuration IDs are environment-specific secrets/configuration, validated at startup where practical.
- A Free user can ignore a pending offer; no cancellation is needed because no subscription or trial clock exists.

## Architecture

### System of record boundaries

- **Stripe:** subscription price, payment method, paid period, trial timing, scheduled phase transitions, invoices, cancellation, and final billing state.
- **UniPost trial grant:** administrative authorization, intended duration/plan, grant lifecycle, display state, failure information, and audit history.
- **UniPost subscription row:** current entitlement plan and Stripe subscription projection, updated by existing Stripe webhooks.
- **Loops/email audit:** trial-ending communication and delivery evidence.

Stripe webhook events are authoritative for remote state transitions. A successful API call records intent, but the grant does not claim `active`, `completed`, or `canceled` until the corresponding Stripe event is processed.

### New trial-grant table

Add an additive migration for `workspace_trial_grants` with this logical shape:

| Column | Purpose |
|---|---|
| `id` | Stable UUID used for Stripe idempotency and metadata |
| `workspace_id` | Billing/entitlement owner |
| `kind` | `free_to_paid` or `paid_same_plan` |
| `plan_id` | Approved trial plan |
| `duration_days` | Integer from 1 through 730 |
| `status` | Grant lifecycle state |
| `granted_by_user_id` | Admin actor |
| `stripe_customer_id` | Stripe correlation snapshot |
| `stripe_subscription_id` | Stripe subscription correlation |
| `stripe_schedule_id` | Subscription Schedule correlation when used |
| `stripe_checkout_session_id` | The one Checkout session currently allowed to claim a Free grant |
| `granted_at` | Grant creation time |
| `scheduled_start_at` | Expected paid-period boundary for a paid-user trial |
| `started_at` | Actual Stripe trial start |
| `ends_at` | Actual Stripe trial end |
| `activated_at` | Free-user Checkout activation time |
| `canceled_at` | Renewal cancellation time |
| `revoked_at` | Admin revocation time for a pending offer |
| `superseded_at` | Plan-change/purchase replacement time |
| `superseded_by_plan_id` | Replacement paid plan, when applicable |
| `completed_at` | Successful normal billing resumption time |
| `failure_code` / `failure_message` | Admin-only provisioning/reconciliation diagnostics |
| `created_at` / `updated_at` | Standard timestamps |

Use a partial unique index to allow at most one grant per workspace in `provisioning`, `pending_activation`, `checkout_pending`, `scheduled`, or `active`. Add unique/indexed lookups for Checkout Session ID, Stripe subscription ID, Stripe schedule ID, status/end time, and workspace history.

The existing `subscriptions.trial_used` column is not suitable for repeatable, audited grants. It remains in place for migration compatibility but is not the policy authority for this feature.

### State machine

Non-terminal states:

- `provisioning`: an admin request exists but the required Stripe operation has not been confirmed.
- `pending_activation`: a Free workspace has an offer but has not completed Checkout.
- `checkout_pending`: one open Stripe Checkout Session has atomically claimed a Free grant but has not completed. `checkout.session.expired` returns it to `pending_activation`.
- `scheduled`: a paying workspace has a future trial phase.
- `active`: Stripe reports the subscription is in the approved trial.

Terminal states:

- `completed`: trial ended and the expected paid plan resumed.
- `canceled`: renewal was canceled and the subscription ended at trial end.
- `revoked`: an administrator withdrew an unactivated Free offer.
- `superseded`: another plan purchase/change forfeited the grant.
- `failed`: provisioning could not be completed safely.

Every transition must be monotonic or explicitly allow a documented reconciliation correction. Duplicate events are no-ops. An older event must not move a terminal or later state backward.

The one documented non-monotonic transition is `checkout_pending -> pending_activation` after Stripe confirms `checkout.session.expired`. It does not consume trial time or create a subscription.

## Backend Interfaces

### Admin APIs

`POST /v1/admin/workspaces/{workspaceID}/trials`

```json
{
  "plan_id": "growth",
  "duration_days": 30
}
```

Behavior:

- Validates workspace, plan, duration, current billing state, existing grants, Stripe cancellation state, and unrelated schedules.
- For Free, creates a pending offer without calling Stripe.
- For paid, creates a provisioning record, creates/updates the Stripe schedule with an idempotency key derived from the grant ID, then marks the grant scheduled.
- Returns the complete admin projection, including exact expected dates.

`POST /v1/admin/workspaces/{workspaceID}/trials/{trialID}/revoke`

- For `pending_activation`, revoke immediately in the local transaction.
- For `checkout_pending`, first ask Stripe to expire the recorded Checkout Session. Stripe permits expiry only while the Session is still `open`. Mark the grant `revoked` only after expiry succeeds. If Stripe reports that the Session is already complete or otherwise not expireable, return 409 and let the completion webhook win; never leave a live trialing subscription attached to a terminal `revoked` grant.
- Preserves the row and records actor/time rather than deleting it.

`GET /v1/admin/billing`

- Extends each billing row with the current/open Trial summary.
- Supports displaying the current state, dates, plan, duration, and provisioning failure.

Financial mutations remain behind the existing admin authentication boundary and must record the authenticated actor, request metadata, before/after values, and Stripe identifiers in the audit log.

### User APIs

`GET /v1/billing`

- Adds a compact `trial` projection for the current `pending_activation`, `checkout_pending`, `scheduled`, or `active` grant.
- Includes fields needed by both Pricing and Settings → Billing: ID, status, plan, days, scheduled/actual dates, post-trial price, and whether changing plans forfeits the trial.

`GET /v1/billing/trials`

- Returns user-safe Trial History newest first.
- Excludes administrator identity and raw failure details.
- Includes plan, duration, status, relevant timestamps, and a normalized user-visible terminal reason.

`POST /v1/billing/checkout`

- Keeps the existing request contract.
- If the workspace has a matching pending grant, attaches the approved trial duration and `trial_grant_id` metadata to the Checkout subscription data.
- Requires payment-method collection for Trial Checkout.
- Before creating Stripe Checkout, atomically change `pending_activation -> checkout_pending`. Only one Checkout Session may claim the grant. Persist the returned Session ID; if Stripe creation fails, compare-and-swap back to `pending_activation`.
- A retry while the recorded Session is still open returns that Session's URL instead of creating another Session. If Stripe reports it expired, reconcile back to pending before creating a replacement.
- `checkout.session.expired` returns an abandoned claim to `pending_activation`, allowing another attempt. A concurrent admin revoke can only win by successfully expiring the still-open Session.
- A mismatched plan starts ordinary Checkout; after successful purchase, webhook handling marks the pending grant superseded.
- Checkout completion and revocation are therefore serialized by Stripe's Session state: completed Checkout is honored; successfully expired Checkout is revoked and can never complete later.

`POST /v1/billing/change-plan`

- Handles immediate plan changes for an existing scheduled or active Trial subscription.
- Dispatches by grant kind and performs one authoritative Stripe mutation: Subscription update for `free_to_paid`, Subscription Schedule update for `paid_same_plan`.
- Ends/removes the trial, updates the price, starts a new full billing cycle, and marks the grant superseded after Stripe confirms the change.
- Uses idempotency keys and returns the updated billing projection.

`POST /v1/billing/trials/{trialID}/cancel-renewal`

- Dispatches by grant kind. `free_to_paid` sets Subscription cancellation at trial end; `paid_same_plan` removes the resumed paid phase and sets Schedule `end_behavior=cancel`.
- Preserves the current/scheduled trial through `ends_at` and prevents any paid phase after it.
- Returns the updated trial and billing projection.

## Stripe Integration

### Metadata

Attach at least these values to Checkout, Subscription, and Schedule/phase metadata where supported:

- `workspace_id`
- `plan_id`
- `trial_grant_id`
- `trial_kind`
- `unipost_environment`

The environment marker remains mandatory so dev/staging/production webhooks cannot apply an event created for another environment.

### Idempotency and failure handling

- Derive Stripe idempotency keys from `trial_grant_id` plus the operation name.
- Create a local `provisioning` record before a paid-user Stripe mutation.
- If Stripe rejects the request, mark the record `failed` with a safe code/message and return an error; do not show the user a scheduled trial.
- If Stripe succeeds but the local update fails, webhook reconciliation must be able to recover the row from Stripe metadata.
- A plan change must never use `release schedule -> update subscription`. The Schedule path is one idempotent Schedule update, so a failed request leaves the prior schedule intact instead of stranding the subscription between remote steps.
- Never overwrite an unrelated Stripe schedule. Return HTTP 409 with a specific admin-facing message.
- Never silently undo `cancel_at_period_end`; require the subscription to be restored before granting a paid-user trial.

### Required webhook coverage

Extend the current Stripe webhook handler for:

- `checkout.session.completed`
- `checkout.session.expired`
- `customer.subscription.created`
- `customer.subscription.updated`
- `customer.subscription.deleted`
- `customer.subscription.trial_will_end`
- Subscription Schedule lifecycle events needed to confirm scheduled, released, completed, canceled, or aborted schedules
- Invoice events already used for payment-failed/recovered behavior at trial exit

Webhook handling must tolerate duplicate delivery and out-of-order events. Grant ID metadata is the primary correlation; Stripe subscription/schedule IDs are indexed fallbacks.

`checkout.session.completed` must treat the Checkout Session as correlation, not as the subscription status authority. It retrieves the referenced Subscription in the verified Stripe mode and then invokes the same subscription projector used for `customer.subscription.created/updated`. In particular, it must persist `trialing`, never the existing hard-coded `active` value.

For `customer.subscription.updated` with `status=trialing` and `cancel_at_period_end=true`, the projector checks for a matching active admin grant before invoking the legacy immediate-cancellation path. A matching managed grant stays on the paid `plan_id`, records renewal cancellation, and remains entitled until Stripe sends the end-of-trial deletion/cancellation event.

### Customer Portal isolation

Provision one dedicated trial-safe Billing Portal configuration in every Stripe mode used by UniPost:

- `features.subscription_cancel.enabled=false`
- `features.subscription_update.enabled=false`
- payment-method update enabled
- invoice history enabled

Store its ID as `STRIPE_SANDBOX_BILLING_PORTAL_TRIAL_CONFIGURATION_ID` for Sandbox/development/staging and `STRIPE_BILLING_PORTAL_TRIAL_CONFIGURATION_ID` for Live/production. `CreatePortal` chooses that ID only for a `scheduled` or `active` admin grant. The UniPost plan-change and cancel-renewal endpoints remain the only subscription mutation paths during those states.

## Lifecycle Email

Add `email.billing.trial_ending.v1` to `api/internal/emailregistry` and `docs/email-templates.md`.

- Provider: Loops
- Template env: `LOOPS_BILLING_TRIAL_ENDING_TRANSACTIONAL_ID`
- Delivery class: `critical_transactional`
- Preference category: essential account/billing; not unsubscribe-gated
- Recipient: workspace owner
- Primary trigger: Stripe `customer.subscription.trial_will_end`
- Normal timing: three days before `ends_at`
- Short-trial fallback: for a one-, two-, or three-day trial, send once immediately after activation if Stripe cannot provide a full three-day lead
- Required variables: `workspace_name`, `plan_id`, `plan_name`, `trial_end`, `days_remaining`, `post_trial_price`, `billing_url`, `cancel_url`
- Idempotency key: `billing_trial_ending:{trial_grant_id}:{trial_end}`
- Audit: one `email_send_attempts` row per idempotency key

The webhook must acknowledge valid Stripe events even if Loops is unavailable. Missing configuration or provider failures are recorded and visible on `/admin/email`. Before enabling the template in any environment, audit Loops workflows so no second workflow sends a duplicate trial-ending message.

## User Experience

### Admin Billing

Add a `Grant Trial` action to each eligible workspace row.

- Free row: editable Plan select plus Days input.
- Paid row: current Plan shown read-only plus Days input.
- Labels sit above inputs; helper text states the 1–730 day limit and when the clock begins.
- The confirmation summary shows the calculated flow before submission:
  - Free: `Pending until Checkout → N-day Plan trial → Plan billing`.
  - Paid: `Current period ends DATE → N-day current-plan trial → billing resumes DATE`.
- Loading disables duplicate submission.
- Validation and Stripe failures appear inline without discarding input.
- Rows show compact Pending, Activation in progress, Scheduled, Active, or Failed status with plan and dates.
- Pending offers expose Revoke. Terminal history remains accessible through audit data; no row is deleted.

### Pricing

The signed-in Pricing page reads the same billing/trial projection as Settings → Billing.

- A Free user's granted target card displays `N-day trial available` and a `Start N-day free trial` CTA.
- While that offer is `checkout_pending`, the same card displays `Activation in progress` and a `Continue checkout` CTA that resumes the one open Session.
- A paid user's current-plan card displays `N-day trial scheduled` with start/end dates, then `Trial active` with end date/remaining days after the phase starts.
- Other plan cards remain actionable. When a grant is scheduled or active, they warn that changing plans forfeits the trial and starts billing immediately.
- No unrelated plan card shows the grant.
- Unsigned users continue through normal registration; registration does not activate or consume a trial.
- Update FAQ copy so it no longer claims that paid plans never have time-limited trials. Copy must state that time-limited trials are available only when specifically granted.

### Settings → Billing

- Show a prominent but non-blocking Trial summary near the current subscription.
- For a scheduled paid trial, render the complete timeline: current paid-period end, trial start, trial end, and billing resumption.
- For an active trial, show plan, end date, remaining time, and post-trial price.
- For a pending Free offer, show the target plan and activation CTA.
- For `checkout_pending`, show activation in progress and let the user resume the existing Checkout Session.
- Upgrade/downgrade remains available with the forfeiture/immediate-charge confirmation.
- Cancel renewal remains available through the trial-aware cancellation flow.

Add `Trial History` below the existing Billing content:

- Newest first.
- Shows plan, duration, normalized status, grant date, scheduled/actual start, end, and terminal reason.
- Does not expose administrator identity or internal failure text.
- Desktop may use the existing billing table language; mobile collapses to a single-column information group with no horizontal dependence.
- Provide layout-matched loading skeletons, an informative empty state, and inline retryable error state.

Pricing and Billing must derive badges and copy from shared status-formatting logic so `pending_activation`, `checkout_pending`, `scheduled`, and `active` never disagree between pages.

## Error Responses and Edge Cases

- Invalid plan or duration: 422 with a field-specific validation message.
- Existing non-terminal grant: 409 with the current grant summary.
- Paid workspace without an active Stripe self-serve subscription: 409.
- Paid subscription already set to cancel: 409.
- Paid subscription with unrelated schedule: 409.
- Enterprise: 422 and no Stripe call.
- Revoke after activation: 409; use cancel-renewal instead.
- Revoke while Checkout is open: expire the Session, then revoke. Revoke after Checkout has completed: 409 and honor the Stripe subscription.
- Checkout for a stale/revoked/superseded grant: ordinary paid Checkout, never a trial.
- Multiple Checkout attempts for the same pending grant: the atomic `checkout_pending` claim permits only one open Session. Expiration returns the grant to pending; completion consumes it. Later/stale events cannot create a second entitlement.
- Trial-exit payment failure: the grant still leaves `active` because the free period ended; the existing Stripe subscription/payment-failure policy decides `past_due` access and email behavior.
- Time values are stored in UTC and rendered in the viewer's local timezone.

## Observability and Audit

Audit events must cover:

- grant created
- grant provisioning failed
- pending grant revoked
- Free grant activated by Checkout
- paid grant scheduled
- trial started
- renewal canceled
- trial superseded by plan change/purchase
- trial completed and paid billing resumed
- Stripe schedule released/canceled unexpectedly

Logs include workspace ID, trial grant ID, Stripe subscription/schedule IDs, plan, duration, transition, request/event ID, and environment. Logs and user-visible errors must not contain Stripe secrets, payment-method data, or raw webhook payloads.

## Implementation Surfaces

The implementation plan must explicitly cover these verified integration points:

- `api/internal/handler/stripe_webhook.go`: replace the unconditional `isTrialCancellation` branch with grant-aware handling; stop hard-coding Checkout-created subscriptions as `active`; add created/expired/trial/schedule events and one shared Stripe subscription projector.
- `api/internal/handler/billing.go`: attach trial data to matching Checkout, coordinate `checkout_pending`, resume an open Session, dispatch plan-change/cancel by grant kind, and select the trial-safe Portal configuration.
- `api/internal/billing/manager.go`: expose narrow mode-aware Stripe operations for Subscription retrieval/update, Checkout expiry, Subscription Schedule create/update, and Portal configuration selection with deterministic fakes in tests.
- `api/internal/db/migrations`, `api/internal/db/queries`, and generated models: add grant lifecycle persistence, indexes, compare-and-swap transitions, history, and correlation lookups.
- `api/internal/handler/admin.go` and `api/cmd/api/main.go`: add authenticated admin grant/revoke routes, user trial/history routes, required webhook event wiring, and environment configuration.
- `api/internal/quota/checker.go` and plan-gate tests: preserve plan-based entitlements and prove a `trialing` target plan is unlocked without being counted as active paid revenue.
- `api/internal/emailregistry` and `docs/email-templates.md`: register and audit the one trial-ending email.
- `dashboard/src/app/admin/billing/page.tsx`, `dashboard/src/app/pricing/pricing-page-client.tsx`, and `dashboard/src/app/(dashboard)/settings/billing/page.tsx`: implement the approved admin form, shared status cards, trial-aware mutations, and Trial History.
- `dashboard/src/lib/api.ts`: add typed grant, history, checkout-resume, cancel-renewal, and plan-change contracts shared by both user surfaces.

## Testing Strategy

### Database and domain tests

- Migration up/down and schema-model synchronization.
- Partial unique index allows repeated terminal grants but blocks concurrent open grants.
- State transition table accepts valid transitions and rejects backward/duplicate-invalid transitions.
- User projection removes admin identity and raw failures.

### Backend unit/handler tests

- Free grant validation and pending creation without a Stripe call.
- Paid same-plan enforcement and exact schedule phase dates.
- 1-day and 730-day boundaries; reject 0, 731, fractions, and unknown plans.
- Reject Enterprise, non-active subscriptions, canceling subscriptions, and unrelated schedules.
- Matching Checkout receives trial metadata/duration and requires a payment method.
- Trial Checkout atomically claims the grant, rejects a second open Session, returns to pending on expiry, and serializes revoke-versus-complete correctly.
- `checkout.session.completed` retrieves and projects the actual Stripe Subscription as `trialing`; it never hard-codes `active`.
- `customer.subscription.created` before/after Checkout completion converges on the same grant/subscription projection.
- Mismatched Checkout does not receive a trial and supersedes only after success.
- Free-trial plan change uses one Subscription update; paid-trial plan change uses one Schedule update and never release-then-update. Both start a full paid cycle and record `superseded` only after confirmation.
- Cancellation dispatches by grant kind, preserves access through trial end, and prevents the resumed paid phase.
- A managed `trialing + cancel_at_period_end` webhook does not invoke the legacy immediate downgrade; an unmanaged legacy trial still does.
- Trialing subscriptions receive target-plan entitlements because gates remain plan-based; paid/MRR reporting does not count them as active revenue.
- Scheduled/active grants receive the trial-safe Portal configuration; the default portal remains unchanged for ordinary subscriptions.
- Duplicate and out-of-order webhooks are idempotent.
- Trial-ending email is sent once with correct variables; short trials use immediate fallback.
- Loops failure records an email attempt and still returns webhook success.

Stripe logic should sit behind a narrow interface so handler/domain tests can use deterministic fakes rather than network calls.

### Frontend tests

- Admin form varies correctly between Free and Paid.
- Validation, loading, error, success, and revoke states.
- Matching Pricing card shows Pending, Checkout-in-progress, Scheduled, and Active states.
- Settings → Billing shows the same state and exact timeline.
- Other plan selection shows the forfeiture/immediate-charge confirmation.
- Trial History renders every terminal state, empty/loading/error states, and mobile layout.
- FAQ copy no longer contradicts the feature.

### End-to-end acceptance

Using Stripe Sandbox/Test Clocks where practical:

1. Free → pending → activate from Pricing → active → reminder → paid invoice.
2. Free → pending → activate from Settings → Billing → active → reminder → paid invoice.
3. Paid → current paid period → scheduled same-plan trial → active → paid resumption.
4. Scheduled and active trials → change plan → superseded → immediate new-plan charge.
5. Trial → cancel renewal → retain access through end → Free with no paid invoice.
6. Pending → admin revoke → no Trial CTA on either surface.
7. Repeated eligible grant after terminal state.
8. Duplicate webhook replay produces no duplicate transition or email.
9. Open Checkout ↔ admin revoke race: only successful expiry may revoke; completed Checkout is honored and activates exactly one trial.

Run backend full tests, dashboard production build, dashboard regression suite, exact-head CI, Railway PR environment, Vercel Preview wired to that PR API, deployed regression, and browser acceptance before integration. A skipped, canceled, timed-out, missing, or wrong-SHA result is a failure under repository policy.

## Deployment and External Configuration

1. Apply the additive database migration.
2. Deploy backend webhook/API support before exposing the admin action.
3. Ensure Stripe webhook endpoints subscribe to trial and schedule lifecycle events in Sandbox and Live modes.
4. Create one trial-safe Customer Portal configuration per Stripe mode, disable subscription update/cancel in it, and set the environment-specific configuration ID.
5. Create/audit the Loops transactional template and set `LOOPS_BILLING_TRIAL_ENDING_TRANSACTIONAL_ID` per environment.
6. Deploy Pricing, Admin Billing, and Settings → Billing together so state presentation stays consistent.
7. Validate on Preview with Stripe Sandbox before any real grant.

There is no feature flag. Production exposure remains conservative because no trial exists until an authenticated administrator explicitly grants one. If code must be rolled back after schedules exist, do not delete or blindly release them: Stripe schedules continue the safe billing timeline, while the prior subscription webhook path can still project ordinary subscription updates. Additive trial rows remain for audit and can be reconciled after the forward fix.

## Acceptance Criteria

1. Admin can grant any integer duration from 1 through 730 days to an eligible workspace.
2. Free grants require the selected target plan and payment method before the clock starts.
3. Pricing and Settings → Billing both activate the same pending offer.
4. Paid grants preserve the current paid period, trial only the current plan, and resume that plan's billing.
5. Pricing and Billing matching cards consistently show Pending, Checkout-in-progress, Scheduled, and Active states with dates.
6. Users can change plans during Scheduled/Active; the trial is forfeited and a new full paid period starts immediately.
7. Users can cancel renewal and keep access through trial end without a following charge.
8. Trial-ending email sends once three days before end, or immediately for a shorter trial.
9. Settings → Billing shows complete user-safe Trial History.
10. Admin Billing and audit logs show complete operational history and failures.
11. Duplicate/out-of-order Stripe events do not duplicate grants, transitions, charges, or emails.
12. Enterprise and ineligible Stripe states are rejected without mutation.
13. Managed trial cancellation never triggers the legacy immediate-downgrade guard; access remains through trial end.
14. Trial Checkout projects the real Stripe `trialing` status and grants paid-plan entitlements without counting trial revenue as active MRR.
15. Trial users cannot mutate or cancel subscriptions through Customer Portal; payment-method management and invoice history remain available.
16. Revoke-versus-Checkout completion cannot produce a terminal revoked grant with a live trialing Stripe subscription.
17. All required local, CI, Preview, deployed regression, and browser acceptance checks pass on the exact PR head SHA.
