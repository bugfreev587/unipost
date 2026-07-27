# Trial Dialog and Stripe Plan Change Fix Design

## Goal

Replace the Admin Billing trial grant and revoke browser confirmations with UniPost's existing confirmation modal, restore mode-aware Stripe webhook projection, and make paid plan changes in both Stripe sandbox and live update the existing subscription with correct proration instead of creating a second subscription.

## Scope

This change covers only:

- Admin Billing `Grant Trial` confirmation.
- Admin Billing `Revoke` confirmation.
- Stripe webhook plan resolution for live and sandbox subscription prices.
- Free-to-paid Checkout for initial subscription activation.
- Paid-to-paid plan changes through a Stripe Customer Portal `subscription_update_confirm` flow.
- Dedicated, environment-specific plan-change Portal configurations.
- Local, CI, staging deployment, regression, and staging sandbox plan-change verification.

Pricing and user Settings Billing trial confirmations remain unchanged. The implementation is shared by sandbox and live modes, but external configuration and deployment remain environment-gated. Production is not changed during staging work; PR #265 remains the user-reviewed staging-to-main promotion PR.

## Confirmed Root Cause

`billing.Manager` intentionally keeps separate live and sandbox price maps. At API startup, `syncStripePriceIDs` writes only live Stripe price IDs into `plans.stripe_price_id`, explicitly describing that column as a legacy live-price cache.

The managed-trial webhook projection added direct calls to `GetPlanByStripePriceID` in both Checkout binding validation and subscription projection. A staging superadmin creates Checkout with a sandbox price ID, which is not present in the live-only database column. The webhook therefore rejects the successfully paid sandbox subscription before updating the local subscription plan. Before the managed-trial projector rewrite, ordinary Checkout used the validated `plan_id` metadata and did not perform this incompatible lookup.

The fix must preserve the stronger subscription-price validation introduced by the projector while resolving the price in the Stripe mode that signed the webhook.

There is a third direct `GetPlanByStripePriceID` call in the legacy helper `resolvePlanIDFromStripeSubscription`. Current staging has no caller for this helper: `handleSubscriptionUpdated` builds a mode-tagged snapshot and calls the shared projector directly. The helper became dead code during the managed-trial projector rewrite. It will be removed so a future caller cannot accidentally reintroduce live-only sandbox reconciliation.

Staging acceptance exposed a second billing defect after the mode-aware resolver was deployed. Stripe Checkout in `subscription` mode always creates a new subscription. A paid Basic workspace therefore received a new Growth subscription while the original Basic subscription remained active. Updating the local row to Growth fixed entitlements but left two renewable subscriptions and produced no proration credit.

The duplicate-subscription state is avoidable. Stripe Customer Portal's `subscription_update_confirm` flow updates one existing subscription, previews the upcoming invoice and proration, and handles payment failures and 3D Secure in Stripe-hosted UI. Ordinary paid plan changes must use that flow. Subscription Checkout remains only for Free workspaces that need their first paid subscription and payment method.

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

Checkout subscription binding and the shared subscription projector will use this resolver. This keeps Checkout metadata as a cross-check instead of making it the billing authority. Free-to-paid Checkout, paid Portal plan changes, and managed-trial subscriptions all reach the same mode-aware subscription projector.

`customer.subscription.updated` already enters the shared projector through `subscriptionWebhookSnapshot(mode.Name, &sub)`, so its sandbox behavior is covered by the same mode-aware resolver. The unused `resolvePlanIDFromStripeSubscription` helper is deleted instead of being expanded with a new mode parameter.

## Paid Plan Change Portal Flow

The billing API separates initial purchase from plan change:

- `POST /v1/billing/checkout` accepts only a Free workspace with no active paid Stripe subscription. It creates the first paid subscription through Checkout.
- `POST /v1/billing/plan-change-session` accepts an owner-authenticated target plan and returns `{ "url": "..." }` for a Stripe Customer Portal Session using `flow_data.type=subscription_update_confirm`.
- A paid workspace can never fall back from a missing or rejected Portal configuration to Subscription Checkout.

Before creating the Portal Session, the API loads the local billing row and retrieves its subscription in the Stripe mode selected for the authenticated workspace owner. It requires one subscription item, matching customer, matching workspace and environment metadata when present, a configured current price, and a configured target price. The Portal flow specifies the exact subscription ID, item ID, and target price. Browser input supplies only the target internal plan ID and never a Stripe object ID.

The Portal configuration applies these billing rules:

- Increasing the recurring amount is an immediate upgrade with `proration_behavior=always_invoice`. Stripe applies unused value from the current plan against the new plan's prorated charge on the same plan-change invoice.
- Decreasing the recurring amount is scheduled for the current period end using the `decreasing_item_amount` condition. The current plan and entitlements remain authoritative until Stripe applies the scheduled change.
- Price is the only allowed customer-selected update. Quantity and promotion-code changes are disabled.
- The flow redirects to Settings Billing only after Stripe completes the hosted confirmation. Canceling the flow makes no billing change.

Stripe emits `customer.subscription.updated` for an applied upgrade and for a downgrade when it becomes effective. The existing shared projector remains the only local plan authority; the Portal-return redirect does not optimistically update entitlements. Payment failure or 3D Secure is handled in the Stripe-hosted flow and does not create a second subscription.

Scheduled or active managed trials do not enter the ordinary paid Portal flow. The existing `TRIAL_PLAN_CHANGE_REQUIRED` response routes those users to `/v1/billing/change-plan`, which mutates the current Subscription or Subscription Schedule directly, forfeits the trial, and starts the selected plan's billing immediately. That path remains unchanged and its existing regression coverage must stay green.

## Portal Configuration and Environment Isolation

Live and sandbox currently have only their default Portal configurations; subscription updates are disabled and proration is `none`. The plan-change flow therefore uses dedicated configuration IDs carried by `billing.Mode`: `STRIPE_PLAN_CHANGE_PORTAL_CONFIGURATION_ID` for live and `STRIPE_SANDBOX_PLAN_CHANGE_PORTAL_CONFIGURATION_ID` for sandbox. The configuration contains only the API, Basic, Growth, and Team product/price pairs for that Stripe mode and uses the billing rules above. Configuration provisioning derives those entries from the same mode-specific price IDs loaded by `billing.Mode`; release verification retrieves the configuration and requires an exact price-set match before the environment variable is accepted.

Staging implementation provisions only the sandbox configuration and stores only its ID in the staging environment. No live Stripe configuration is created or changed during staging work. Production configuration is a separate release action that requires production authorization. If the selected mode lacks a plan-change configuration ID, the endpoint fails closed and does not create Checkout.

The duplicate Basic subscription created during staging acceptance is one-time sandbox data cleanup. After the new flow is deployed, the old Basic subscription is canceled with proration and the verified Growth subscription remains active. This cleanup is not part of the runtime production architecture.

The only current production paid workspace upgraded from Basic to Team through the legacy Checkout flow. The user has already canceled its old subscription and manually adjusted the prorated customer balance. The user confirmed there are no other paid workspaces, so a production database or Stripe-wide duplicate-subscription audit is explicitly out of scope.

Production ordering is a hard release gate: before PR #265 may merge into `main`, create the dedicated live plan-change Portal configuration, verify its live product/price allowlist and billing rules against `billing.Mode`, and set `STRIPE_PLAN_CHANGE_PORTAL_CONFIGURATION_ID` in the production environment. If any step is missing or mismatched, PR #265 must remain open because paid production upgrades would fail closed. None of those live Stripe or production environment actions are authorized by this staging task; they require explicit production-release authorization.

Pending-downgrade display in UniPost is intentionally deferred. Stripe's hosted confirmation shows the scheduled change, while Settings Billing continues to show the current effective plan until Stripe applies the downgrade and emits the authoritative webhook. Adding a separate local pending-downgrade projection is a follow-up UX feature, not part of this billing-correctness fix.

## Failure Handling and Idempotency

- Unknown price IDs continue to fail closed and return a retryable webhook error.
- A configured sandbox price cannot be mistaken for the corresponding live price because the verified webhook mode selects the reverse map.
- Existing environment, workspace, customer, subscription, metadata, period monotonicity, and trial-grant guards remain unchanged.
- Replayed Checkout and subscription events remain idempotent through the existing projection and lifecycle-event keys.
- Paid workspaces cannot enter Subscription Checkout, including two concurrent paid Checkout attempts.
- Missing configuration, foreign ownership, multiple subscription items, unknown prices, Stripe retrieval failure, and Portal Session creation failure all fail closed before any plan mutation.
- Portal completion is projected only from the verified `customer.subscription.updated` webhook; redirect timing cannot grant entitlements.
- A duplicate or delayed subscription webhook cannot regress the current billing period or duplicate the plan-change lifecycle event.
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
- Free-to-paid creates Subscription Checkout and does not require a plan-change Portal configuration.
- Paid Basic-to-Growth creates a `subscription_update_confirm` Portal Session for the exact existing subscription item and sandbox Growth price.
- Paid plan-change Session creation performs zero Checkout Session calls.
- Paid Checkout is rejected before Stripe mutation, so concurrent attempts cannot create parallel subscriptions.
- Missing Portal configuration, multiple items, foreign customer, workspace, or environment, and unknown current or target prices fail closed.
- Portal Session parameters use redirect-after-completion and never accept Stripe IDs from the request body.
- Sandbox `customer.subscription.updated` projects the upgraded Growth price without changing the subscription ID.
- Replayed and out-of-order subscription events preserve period monotonicity and lifecycle idempotency.
- Existing managed-trial `/change-plan` tests remain green.

Dashboard contract tests cover:

- Admin Billing imports and renders `ConfirmModal` for grant and revoke.
- The `GrantTrialForm` implementation no longer calls `window.confirm` or bare `confirm` for grant or revoke.
- Grant uses the default modal treatment and Revoke uses the danger treatment.
- Confirmation text retains plan, duration, timing, and activation consequences.
- Settings Billing sends Free workspaces to Checkout, ordinary paid workspaces to `/v1/billing/plan-change-session`, and scheduled or active managed trials to the existing `/v1/billing/change-plan` action.
- Pricing-page redirects reach the same Settings Billing dispatcher rather than calling a second billing implementation.
- Portal success and cancellation return to Settings Billing without optimistic plan mutation.

The source assertion is scoped between `GrantTrialForm` and `PlanFlipMenu`. The separate Admin plan-flip QA control intentionally retains its existing browser confirmation because it was explicitly excluded from this change.

Validation includes full API tests, Dashboard source contract tests, Dashboard build, and local dashboard regression.

## Staging Delivery and Acceptance

The task stays on the conversation-owned `dev-admin-configurable-free-trial` branch and worktree. A focused PR targets `staging`. Per explicit user authorization, Preview Acceptance is skipped for this fix; local CI-equivalent checks and GitHub CI remain required.

After merge, all staging Railway and Vercel deployments must succeed for the exact staging SHA. Deployed acceptance will verify:

- Grant Trial opens the UniPost modal and Cancel produces no mutation.
- Revoke opens the UniPost danger modal and Cancel produces no mutation.
- The one-time duplicate Basic sandbox subscription is canceled with proration, leaving the verified current subscription as the only active paid subscription.
- The existing staging admin account completes an immediate paid upgrade through Stripe's hosted update-confirmation flow. Basic-to-Growth is preferred; if the verified current plan is already Growth, Growth-to-Team is used.
- The hosted page displays the prorated invoice impact and completes without creating a second subscription.
- Returning to Settings Billing shows the webhook-projected target plan and the same Stripe subscription ID.
- Stripe shows exactly one active paid subscription for the customer.
- Dashboard staging regression passes.
- PR #265 points to the new validated staging head and remains open for user review.

The first post-deployment regression attempt produced 58 passes, one Monaco editor timeout, and one serially skipped test because jsDelivr module requests remained incomplete. The trace showed the external CDN dependency rather than a billing regression. The user explicitly authorized treating that observed CDN incident as an environment exception, but the final replacement SHA must still run the complete 60-test suite again and pass before this work is considered complete.
