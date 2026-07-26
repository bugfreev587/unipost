# Staging Trial Dialog and Sandbox Upgrade Fix Design

## Goal

Replace the Admin Billing trial grant and revoke browser confirmations with UniPost's existing confirmation modal, and restore Stripe sandbox plan upgrades so a successful staging Checkout is projected onto the workspace subscription.

## Scope

This change covers only:

- Admin Billing `Grant Trial` confirmation.
- Admin Billing `Revoke` confirmation.
- Stripe webhook plan resolution for live and sandbox subscription prices.
- Local, CI, staging deployment, regression, and staging sandbox Checkout verification.

Pricing and user Settings Billing trial confirmations remain unchanged. Production is not merged; PR #265 remains the user-reviewed staging-to-main promotion PR.

## Confirmed Root Cause

`billing.Manager` intentionally keeps separate live and sandbox price maps. At API startup, `syncStripePriceIDs` writes only live Stripe price IDs into `plans.stripe_price_id`, explicitly describing that column as a legacy live-price cache.

The managed-trial webhook projection added direct calls to `GetPlanByStripePriceID` in both Checkout binding validation and subscription projection. A staging superadmin creates Checkout with a sandbox price ID, which is not present in the live-only database column. The webhook therefore rejects the successfully paid sandbox subscription before updating the local subscription plan. Before the managed-trial projector rewrite, ordinary Checkout used the validated `plan_id` metadata and did not perform this incompatible lookup.

The fix must preserve the stronger subscription-price validation introduced by the projector while resolving the price in the Stripe mode that signed the webhook.

There is a third direct `GetPlanByStripePriceID` call in the legacy helper `resolvePlanIDFromStripeSubscription`. Current staging has no caller for this helper: `handleSubscriptionUpdated` builds a mode-tagged snapshot and calls the shared projector directly. The helper became dead code during the managed-trial projector rewrite. It will be removed so a future caller cannot accidentally reintroduce live-only sandbox reconciliation.

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

## Failure Handling and Idempotency

- Unknown price IDs continue to fail closed and return a retryable webhook error.
- A configured sandbox price cannot be mistaken for the corresponding live price because the verified webhook mode selects the reverse map.
- Existing environment, workspace, customer, subscription, metadata, period monotonicity, and trial-grant guards remain unchanged.
- Replayed Checkout and subscription events remain idempotent through the existing projection and lifecycle-event keys.
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
- Dashboard staging regression passes.
- PR #265 points to the new validated staging head and remains open for user review.
