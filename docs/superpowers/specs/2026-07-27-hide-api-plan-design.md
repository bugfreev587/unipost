# Hide API Plan from New Users

## Goal

Temporarily remove the API plan from self-serve frontend selection surfaces while preserving the existing API plan subscriptions and product behavior.

## Scope

- Add one shared frontend visibility rule that treats `api` as unavailable to new users.
- Apply that rule to every Pricing plan-list render site:
  - the `TIERS` card loop;
  - the comparison table's hardcoded plan header and column-key loop;
  - both desktop and mobile loops over `X_CREDIT_PLANS`.
- Keep the hardcoded Enterprise sales block and the Enterprise X Credits row/card visible. Only API is hidden.
- Update Pricing sales copy and FAQ entries that present API as a currently sold plan. General references to the UniPost API remain.
- Apply the shared rule to Dashboard Billing upgrade cards, except that a current API subscriber continues to see API as the current plan.
- Keep the Dashboard Billing plan-name summary aligned with the cards each user can see.

## Explicit Non-Goals

- Do not change the API, database, plan catalog, Stripe Product, or Stripe Price.
- Do not block direct Checkout API calls or the existing `?upgrade=api` deep link.
- Do not scrub legacy API plan references from documentation or marketing routes outside the public Pricing page.
- Do not change current subscriptions, entitlements, renewals, quotas, or billing webhooks.
- Do not change how sandbox, admin, or super-admin subscriptions are counted in reporting.

## Frontend Behavior

The public Pricing page presents the currently sold self-serve plans: Free, Basic, Growth, and Team. Enterprise remains visible through its existing sales-led block and X Credits capacity entry. API remains in the local frontend data so restoring it later does not require reconstructing plan details, but the shared visibility rule filters it from rendered lists.

The comparison data retains its existing `api` properties. The rendered header and cell loops use the visible comparison-plan IDs, so hidden data does not appear while existing plan contracts and a future restoration remain intact.

Pricing copy that explicitly presents API as sold is updated or removed, including the plan-ladder FAQ, quota and retention explanations, the API-versus-Basic FAQ, competitor price comparison, and paid-plan starting price. General product-API language is not changed.

Dashboard Billing filters the local plan catalog according to the current subscription:

- If the current plan is API, show Free, API, Basic, Growth, and Team so the subscriber can still identify the current plan.
- Otherwise, show Free, Basic, Growth, and Team.

The change does not add new styling. Pricing's card, comparison-header, and comparison-row grids change from five self-serve columns to four across their base and responsive CSS declarations. Dashboard Billing already uses an auto-flowing three-column grid and needs no CSS change.

## Verification

- A focused source-level regression test proves the shared API visibility rule is applied to the Pricing card loop, comparison header/cell loops, both X Credits loops, and Dashboard Billing cards.
- The test proves Enterprise remains visible while API is filtered.
- The test proves Dashboard Billing conditionally keeps API only for a current API subscriber.
- The test proves new-user Pricing sales copy no longer advertises API as a purchasable plan.
- The focused regression test fails before the implementation and passes afterward.
- `team-plan-contract-source.test.mjs` is rerun to prove retained hidden `api` comparison data still satisfies the Team contract; it is changed only if the implementation genuinely changes that contract.
- `enterprise-pricing-source.test.mjs` is rerun to prove the Enterprise block ordering and Enterprise exclusion from `TIERS` remain intact.
- The Dashboard production build passes.
- The Dashboard browser regression suite runs and browser acceptance verifies Pricing and Billing because shared plan-selection surfaces change.

## Accepted Trade-off

This is intentionally a frontend-only sales-hiding measure. A user who knows an old Checkout API path or upgrade deep link may still initiate an API plan subscription. The product owner explicitly accepts that risk in exchange for the smallest implementation.
