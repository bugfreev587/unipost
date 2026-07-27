# Hide API Plan from New Users

## Goal

Temporarily remove the API plan from self-serve frontend selection surfaces while preserving the existing API plan subscriptions and product behavior.

## Scope

- Hide the API plan card from the public Pricing page.
- Hide the API plan column from the public Pricing comparison table.
- Hide the API plan row/card from the optional X Credits comparison on the public Pricing page.
- Hide the API plan from the Dashboard Billing upgrade cards for users whose current plan is not API.
- Keep the API plan visible as the current plan in Dashboard Billing for an existing API subscriber.
- Keep the visible plan-name summary aligned with the cards each user can see.

## Explicit Non-Goals

- Do not change the API, database, plan catalog, Stripe Product, or Stripe Price.
- Do not block direct Checkout API calls or the existing `?upgrade=api` deep link.
- Do not remove legacy API plan references from product documentation or other marketing copy.
- Do not change current subscriptions, entitlements, renewals, quotas, or billing webhooks.
- Do not change how sandbox, admin, or super-admin subscriptions are counted in reporting.

## Frontend Behavior

The public Pricing page always presents the currently sold self-serve plans: Free, Basic, Growth, and Team. Its card grid, comparison table, and X Credits capacity presentation use the same visible plan set.

Dashboard Billing filters the local plan catalog according to the current subscription:

- If the current plan is API, show Free, API, Basic, Growth, and Team so the subscriber can still identify the current plan.
- Otherwise, show Free, Basic, Growth, and Team.

The change does not add new styling. Existing grids should be adjusted only as needed to fit four public plan columns cleanly.

## Verification

- A source-level regression test proves the public Pricing page excludes API from all three plan-list render paths.
- The same test proves Dashboard Billing conditionally keeps API only for a current API subscriber.
- The focused regression test fails before the implementation and passes afterward.
- The Dashboard production build passes.
- The Dashboard browser regression suite runs because the shared public Pricing and Billing surfaces change.

## Accepted Trade-off

This is intentionally a frontend-only sales-hiding measure. A user who knows an old Checkout API path or upgrade deep link may still initiate an API plan subscription. The product owner explicitly accepts that risk in exchange for the smallest implementation.
