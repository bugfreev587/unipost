# Staging Trial Dialog and Sandbox Upgrade Fix Design

## Goal

Replace the Admin Billing trial grant and revoke browser confirmations with UniPost's existing confirmation modal, and restore Stripe sandbox plan upgrades so a successful staging Checkout is projected onto the workspace subscription without leaving the replaced paid subscription active.

## Scope

This change covers only:

- Admin Billing `Grant Trial` confirmation.
- Admin Billing `Revoke` confirmation.
- Stripe webhook plan resolution for live and sandbox subscription prices.
- Paid-plan Checkout handoff from the replaced subscription to the newly paid subscription.
- Prorated Stripe customer-balance credit for unused time on the replaced subscription.
- Local, CI, staging deployment, regression, and staging sandbox Checkout verification.

Pricing and user Settings Billing trial confirmations remain unchanged. Production is not merged; PR #265 remains the user-reviewed staging-to-main promotion PR.

## Confirmed Root Cause

`billing.Manager` intentionally keeps separate live and sandbox price maps. At API startup, `syncStripePriceIDs` writes only live Stripe price IDs into `plans.stripe_price_id`, explicitly describing that column as a legacy live-price cache.

The managed-trial webhook projection added direct calls to `GetPlanByStripePriceID` in both Checkout binding validation and subscription projection. A staging superadmin creates Checkout with a sandbox price ID, which is not present in the live-only database column. The webhook therefore rejects the successfully paid sandbox subscription before updating the local subscription plan. Before the managed-trial projector rewrite, ordinary Checkout used the validated `plan_id` metadata and did not perform this incompatible lookup.

The fix must preserve the stronger subscription-price validation introduced by the projector while resolving the price in the Stripe mode that signed the webhook.

There is a third direct `GetPlanByStripePriceID` call in the legacy helper `resolvePlanIDFromStripeSubscription`. Current staging has no caller for this helper: `handleSubscriptionUpdated` builds a mode-tagged snapshot and calls the shared projector directly. The helper became dead code during the managed-trial projector rewrite. It will be removed so a future caller cannot accidentally reintroduce live-only sandbox reconciliation.

Staging acceptance exposed a second billing defect after the mode-aware resolver was deployed. Stripe Checkout in `subscription` mode always creates a new subscription. A paid Basic workspace therefore received a new Growth subscription while the original Basic subscription remained active. Updating the local row to Growth fixed entitlements but did not complete the Stripe billing handoff and could charge both subscriptions on future renewals.

The paid Checkout path must explicitly identify and retire the subscription it replaces. This is not applicable to Free-to-paid activation because Free workspaces have no paid Stripe subscription to replace.

## Admin Confirmation Dialogs

`dashboard/src/app/admin/billing/page.tsx` will import and reuse `ConfirmModal` from `dashboard/src/components/confirm-modal.tsx`.

`GrantTrialForm` will keep a local pending-confirmation state with two variants:

- `grant`: captures the validated plan, duration, and proposed timeline shown when the action was opened.
- `revoke`: captures the current revocable grant identity and plan shown when the action was opened.

Clicking an action opens the modal but does not mutate billing state. Confirming closes the modal and calls the existing async mutation using the captured values. Canceling or pressing Escape closes the modal without issuing an API request. Existing mutation locks remain authoritative, and the confirm button is disabled while a mutation is in progress.

The Grant modal uses the default variant, title `Grant trial`, confirm label `Grant Trial`, and the same workspace, plan, duration, start, end, and post-trial billing facts currently shown by the browser confirmation. The Revoke modal uses the danger variant, title `Revoke trial offer`, confirm label `Revoke`, and states that the user will no longer be able to activate the offer.

## Mode-Aware Stripe Plan Resolution

`api/internal/billing/manager.go` will expose a read-only reverse lookup on `Mode`:

```go
func (m *Mode) PlanID(priceID string) string
```

The method trims the requested price ID, scans the mode's configured plan-to-price map, and returns the matching internal plan ID or an empty string. The map is immutable after manager construction, so no additional synchronization is required.

`StripeWebhookHandler` will centralize subscription price resolution:

1. Resolve the verified mode from the snapshot's `StripeMode`.
2. Reverse-map the Stripe price through that mode.
3. Load the canonical plan row by internal plan ID.
4. If no configured mode mapping matches, fall back to `GetPlanByStripePriceID` for legacy live events and historical prices.
5. Return a hard error if neither source resolves the price.

Checkout subscription binding and the shared subscription projector will use this resolver. This keeps Checkout metadata as a cross-check instead of making it the billing authority. Both ordinary sandbox upgrades and managed-trial sandbox subscriptions use the same projection path.

`customer.subscription.updated` already enters the shared projector through `subscriptionWebhookSnapshot(mode.Name, &sub)`, so its sandbox behavior is covered by the same mode-aware resolver. The unused `resolvePlanIDFromStripeSubscription` helper is deleted instead of being expanded with a new mode parameter.

## Paid Checkout Subscription Handoff

When an ordinary Checkout is created for a workspace whose local billing row points to an existing paid Stripe subscription, the Checkout Session and its future Subscription metadata include the exact subscription ID and plan being replaced. Free-to-paid Checkout omits these fields. The metadata is a claim to verify, not authority to cancel an arbitrary Stripe object.

On `checkout.session.completed`, after retrieving the new subscription and completing the existing environment, workspace, customer, price, plan, and Checkout metadata checks, the handler:

1. Loads the workspace's current local subscription.
2. Accepts the replacement claim only when the claimed old ID is the local Stripe subscription on the first delivery, or when the local row already points to the verified new subscription on a retry.
3. Retrieves the claimed old subscription from the same verified Stripe mode.
4. Requires the old subscription customer to equal the new subscription customer. Its workspace and environment metadata must match when present; legacy empty metadata is accepted only because the exact old subscription ID and customer were already bound in the local billing row.
5. Projects the verified new subscription onto the workspace before retiring the old subscription. This prevents a concurrently delivered deletion event from downgrading the workspace after the replacement is known.
6. Immediately cancels the old subscription with Stripe proration and final invoicing enabled so its unused paid time becomes customer-balance credit under Stripe's invoice rules.
7. Returns HTTP 500 if retrieval or cancellation is indeterminate. Stripe retry resumes the same handoff without creating another Checkout or subscription.
8. Continues trial supersession, plan-change notifications, and quota reconciliation only through their existing idempotent paths. The validated replaced-plan metadata preserves the original plan-change source on a retry after the local row already points to the new plan.

The cancellation operation is idempotent at the application boundary. A retry that finds the old subscription already canceled treats retirement as complete. It never cancels the verified new subscription, a subscription owned by another customer or workspace, or an ID supplied only by browser input.

`customer.subscription.deleted` for the replaced subscription may arrive after the local row points to the new subscription. The deletion handler acknowledges that event as a stale superseded-subscription deletion only when the event is for the same verified environment, workspace, and customer and the local row contains a different non-empty Stripe subscription ID. It must not project the old paid plan, cancel the new subscription, or downgrade the workspace to Free. Deletion of the current local subscription keeps the existing cancellation behavior.

Scheduled or active managed trials do not enter ordinary Checkout. The existing `TRIAL_PLAN_CHANGE_REQUIRED` response routes those users to `/v1/billing/change-plan`, which mutates the current Subscription or Subscription Schedule directly, forfeits the trial, and starts the selected plan's billing immediately. That path remains unchanged and its existing regression coverage must stay green.

## Failure Handling and Idempotency

- Unknown price IDs continue to fail closed and return a retryable webhook error.
- A configured sandbox price cannot be mistaken for the corresponding live price because the verified webhook mode selects the reverse map.
- Existing environment, workspace, customer, subscription, metadata, period monotonicity, and trial-grant guards remain unchanged.
- Replayed Checkout and subscription events remain idempotent through the existing projection and lifecycle-event keys.
- A failed old-subscription cancellation returns HTTP 500 after the new subscription projection; replay retries retirement and cannot create another subscription.
- A delayed deletion event for the replaced subscription is an acknowledged no-op after the new subscription is locally authoritative.
- Free-to-paid Checkout never enters subscription replacement or proration logic.
- Modal cancellation creates no network request or mutation lock.
- If the confirmed mutation fails, the existing inline error and refresh-required behavior remains unchanged.

## Test Strategy

Tests are written before production changes and must fail for the expected missing behavior.

Backend tests cover:

- `Mode.PlanID` resolves a sandbox price whose value differs from the database live price.
- Checkout completion signed by the sandbox webhook accepts the sandbox price and projects the requested plan.
- `customer.subscription.updated` in sandbox resolves the same price and projects the plan.
- Unknown mode-specific prices still fail closed.
- Existing live and managed-trial webhook tests remain green.
- A paid Basic-to-Growth Checkout projects Growth and requests immediate prorated cancellation of the exact old Basic subscription.
- Cancellation uses final invoicing plus proration so unused Basic time is credited through Stripe.
- Cancellation failure returns HTTP 500; replay completes the same handoff without a second plan mutation or duplicate cancellation side effect.
- A previously canceled old subscription is accepted as an idempotent successful retirement.
- A delayed `customer.subscription.deleted` event for the old Basic subscription does not downgrade the current Growth subscription.
- A deletion event for the current Growth subscription still follows the normal cancellation-to-Free path.
- A foreign customer, workspace, environment, or unbound replacement ID fails closed without cancellation.
- Free-to-paid Checkout performs zero replacement cancellation calls.

Dashboard contract tests cover:

- Admin Billing imports and renders `ConfirmModal` for grant and revoke.
- The `GrantTrialForm` implementation no longer calls `window.confirm` or bare `confirm` for grant or revoke.
- Grant uses the default modal treatment and Revoke uses the danger treatment.
- Confirmation text retains plan, duration, timing, and activation consequences.

The source assertion is scoped between `GrantTrialForm` and `PlanFlipMenu`. The separate Admin plan-flip QA control intentionally retains its existing browser confirmation because it was explicitly excluded from this change.

Validation includes full API tests, Dashboard source contract tests, Dashboard build, and local dashboard regression.

## Staging Delivery and Acceptance

The task stays on the conversation-owned `dev-admin-configurable-free-trial` branch and worktree. A focused PR targets `staging`. Per explicit user authorization, Preview Acceptance is skipped for this fix; local CI-equivalent checks and GitHub CI remain required.

After merge, all staging Railway and Vercel deployments must succeed for the exact staging SHA. Deployed acceptance will verify:

- Grant Trial opens the UniPost modal and Cancel produces no mutation.
- Revoke opens the UniPost danger modal and Cancel produces no mutation.
- The existing staging admin account completes a Basic-to-Growth Stripe sandbox Checkout.
- Returning to Settings Billing shows Growth after webhook reconciliation.
- Stripe shows the new Growth subscription as active and the replaced Basic subscription as canceled; no second active paid subscription remains.
- The prorated unused Basic amount is represented by Stripe's final invoice/customer-balance credit.
- Dashboard staging regression passes.
- PR #265 points to the new validated staging head and remains open for user review.

The first post-deployment regression attempt produced 58 passes, one Monaco editor timeout, and one serially skipped test because jsDelivr module requests remained incomplete. The trace showed the external CDN dependency rather than a billing regression. The user explicitly authorized treating that observed CDN incident as an environment exception, but the final replacement SHA must still run the complete 60-test suite again and pass before this work is considered complete.
