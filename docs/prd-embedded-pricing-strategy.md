# UniPost - Embedded Pricing Strategy PRD
**Differentiate UniPost from per-connected-account pricing and tighten free/API plan boundaries**

Version 1.1 | June 21, 2026

---

Execution note:

- See `docs/prd-embedded-pricing-strategy-execution-notes.md` for Phase 1 status, existing backend gate conventions, and Phase 2/3 implementation blockers.

---

## 1. Background

UniPost's current public pricing is product-tier based:

- Free - `$0/mo`, `100 posts/mo`
- API - `$10/mo`, `1,000 posts/mo`
- Basic - `$19/mo`, `2,500 posts/mo`
- Growth - `$59/mo`, `7,500 posts/mo`
- Team - `$149/mo`, `25,000 posts/mo`

The current ladder packages UniPost by product stage and monthly posting capacity, not by the number of connected social accounts.

Zernio's public pricing pages now show a different model. As of `2026-06-21`, those pages price by connected social account:

- first 2 connected accounts are free
- accounts 3-10 are `$6/account/mo`
- accounts 11-100 are `$3/account/mo`
- accounts 101-2,000 are `$1/account/mo`
- 2,001+ is custom

Source:

- [Zernio pricing](https://docs.zernio.com/pricing)
- [Zernio public pricing page](https://zernio.com/pricing)

Evidence note:

- The public Zernio pages above were checked on `2026-06-21` and currently show pay-per-connected-account pricing.
- UniPost's repository still stores Zernio's older tier/add-on positioning in `dashboard/src/data/competitors/zernio.ts` and old Zernio add-on copy in `dashboard/src/app/pricing/page.tsx`.
- Before any public UniPost page publishes the `$418/mo` comparison, capture a dated screenshot or archive of Zernio's current pricing page and update all stored competitor data in the same change. Treat specific competitor prices as external claims that require evidence.

This creates a sharp positioning opportunity for UniPost. A customer building their own app may have many end users, each connecting one or more social accounts. Under per-connected-account pricing, that customer's infrastructure cost grows linearly with connected accounts, even when posting volume is low.

Example:

- 100 app end users
- 2 social accounts per end user
- 200 connected social accounts total

Zernio monthly cost:

```text
8 x $6 + 90 x $3 + 100 x $1 = $418/mo
```

The same customer on UniPost should choose primarily by:

- monthly successful posts
- whether they need hosted branding or platform credentials
- whether they need Inbox, Analytics, team features, and higher operational limits

They should not be charged a self-serve line item for every connected social account.

---

## 2. Definitions

This PRD uses four terms that must stay distinct in product copy and docs.

### 2.1 Workspace user

A person who can log in to the UniPost workspace.

Examples:

- founder
- marketer
- engineer
- agency teammate

This is the "Users" row on the pricing page. It is not the customer's app user count.

### 2.2 Managed user

An end user inside a UniPost customer's own product, identified by `external_user_id`.

Examples:

- a creator in a scheduling SaaS
- a seller in an ecommerce marketing tool
- a franchise location manager in a brand portal

Managed users appear in the UniPost API through `/v1/users` and related managed-account flows.

### 2.3 Connected social account

A social account, page, channel, organization, or profile connected to UniPost.

Examples:

- one Instagram account
- one Facebook Page
- one LinkedIn organization
- one YouTube channel
- one X account

Zernio bills directly on this dimension. UniPost should not use it as the primary self-serve billing meter.

### 2.4 Profile

A UniPost grouping for a brand, app surface, client, region, or project inside a workspace.

Profiles are not the same thing as managed users. A customer with 10,000 app users should not need 10,000 UniPost profiles.

---

## 3. Problem

The current UniPost pricing strategy is directionally right, but it does not yet fully claim the strongest competitive advantage created by Zernio's per-account model.

Today, the pricing page says:

- product-tier plus monthly capacity
- Free/API/Basic/Growth/Team
- profiles and workspace users
- Inbox, Analytics, white-label, and team unlocks

But it does not clearly say:

- UniPost is not priced per connected social account on self-serve plans.
- UniPost is a better fit for embedded apps where end users connect their own social accounts.
- "Users" on the pricing page means workspace team members, not app end users.
- Free is for evaluation, not for running an embedded customer-facing product.
- API is for paid developer usage and light production, not the main embedded SaaS plan.

The result is that customers comparing UniPost to Zernio may miss the most important economic difference.

---

## 4. Goals

1. Position UniPost as the predictable pricing option for embedded apps with many connected social accounts.
2. Preserve the current product-tier pricing direction instead of copying Zernio's connected-account meter.
3. Make Growth the obvious self-serve plan for customer-facing embedded social account connection.
4. Retain the API plan as a low-friction paid developer tier, while preventing it from becoming the cheap production plan for embedded SaaS.
5. Tighten Free so it remains useful for evaluation but cannot replace a paid plan.
6. Clarify the difference between workspace users, managed users, connected social accounts, and profiles.
7. Update competitor positioning so `/alternatives/zernio`, `/compare`, `/pricing`, and stored Zernio competitor metadata reflect the same verified Zernio pricing model.

---

## 5. Non-goals

- Do not switch UniPost self-serve pricing to per-connected-account billing.
- Do not remove the Free plan.
- Do not remove the API plan.
- Do not introduce automatic overage billing in this PRD.
- Do not create a full add-on marketplace in this PRD.
- Do not change production pricing immediately without a migration and communication plan.
- Do not retroactively disconnect existing accounts when a workspace exceeds a new limit.
- Do not treat managed users as billable seats.

---

## 6. Strategic decision

UniPost should keep its core self-serve pricing model:

> product tier + monthly post capacity + feature/operational limits

UniPost should explicitly avoid positioning around:

> price per connected social account

Connected social accounts are still a cost and risk driver. They affect token refresh, storage, webhooks, analytics sync, inbox sync, and support burden. But they should be handled through:

- plan packaging
- feature gates
- fair-use policy
- enterprise/custom terms at high scale
- operational safety limits

They should not become a visible per-account self-serve meter.

This is the core differentiation against Zernio.

---

## 7. Customer positioning

### 7.1 Primary message

Recommended public positioning:

> Build embedded social publishing without a connected-account tax.

Supporting copy:

> UniPost plans are based on product stage and monthly posting capacity. Your app can let users connect social accounts without every connected account becoming a new self-serve billing line item.

### 7.2 Example comparison

Recommended pricing page example:

| Scenario | Zernio | UniPost |
| --- | ---: | --- |
| 100 end users x 2 connected accounts | `$418/mo` | Growth is `$59/mo` if total usage stays within `7,500 posts/mo` and Growth feature limits |

Supporting copy:

> If your app has many low-volume users, per-connected-account pricing can become expensive before your own revenue catches up. For example, 200 connected accounts with total monthly usage under `7,500` posts would fit UniPost Growth at `$59/mo`, while the same connected-account count is `$418/mo` under Zernio's current graduated account pricing. UniPost is designed for embedded products where account count and posting volume do not always grow together.

This example should always state its assumption: it compares low-frequency connected accounts where the customer's total monthly posting volume and required features fit Growth.

### 7.3 Audience fit

UniPost should be positioned as strongest for:

- SaaS products embedding social account connection
- AI tools that publish on behalf of users
- creator tools with many low- or medium-frequency users
- vertical SaaS products where each customer connects a few social accounts
- products that need hosted Connect, `external_user_id`, and managed-account APIs

Zernio remains stronger for:

- customers needing more platform coverage today
- customers needing Ads as a first-class product surface
- customers comfortable paying per connected account
- high-volume accounts where unlimited posts matter more than connected-account count

---

## 8. Plan strategy

### 8.1 Free

Free should remain permanent, useful, and credit-card-free.

Free should not be powerful enough to run a customer-facing embedded app.

Recommended Free positioning:

> Validate the API and dashboard with a real connected account.

Recommended Free limits:

| Limit | Recommendation |
| --- | --- |
| Monthly posts | Keep `100 posts/mo` hard cap |
| Workspace users | Keep `1` |
| Profiles | Keep `1` |
| Connected social accounts | Add a low cap, recommended `2` managed connected accounts |
| Managed users | Add a low cap, recommended `3` distinct completed `external_user_id` values |
| Connect sessions | Do not hard-cap create attempts; monitor for abuse/rate-limit patterns |
| API keys | Add a low cap, recommended `1 active key` |
| Webhook endpoints | Add a low cap, recommended `1 endpoint` |
| X publishing | Keep unavailable |
| Inbox | Keep unavailable |
| Analytics | Keep unavailable |
| Platform credentials | Keep unavailable |
| Hosted Connect branding | Keep unavailable |

Rationale:

- Free still proves the product works.
- Free supports real API testing, dashboard testing, MCP testing, webhooks, and scheduling.
- Free no longer lets a customer quietly run a small embedded SaaS.

Implementation note:

- Existing accounts should not be deleted or disconnected if a workspace later exceeds the new cap.
- New successful managed connections should be blocked once the connected-account or managed-user limit is reached.
- Connect Session creation attempts should not be blocked by a monthly product cap. Developers often create several sessions while integrating and debugging before a user successfully connects an account.
- Connect Session creation may still reject requests that would clearly exceed an already-reached managed-user or managed-account cap, such as a new `external_user_id` on a Free workspace that already has `3` managed users. That is cap enforcement on the resulting successful connection, not metering session attempts.
- Enforce caps again at successful connection time so pending sessions cannot bypass the limit.
- Abuse protection and operational rate limits may still apply to Connect Session creation, but those should be framed as anti-abuse/runtime safety controls rather than plan packaging.
- Public copy should frame the Free connected-account cap as an evaluation boundary, not as a pricing dimension. The message is "Free is for trying the product," not "UniPost charges by account."
- Phase 2 Free-plan enforcement will ship without a feature flag, per product decision. Rollback is a code revert if dev validation exposes an issue.

### 8.2 API

The API plan should be retained.

Recommended API positioning:

> Paid developer access for API-first publishing and light production usage.

Why keep API:

- `$10/mo` is a strong developer entry point.
- It preserves a clean alternative to PostForMe-style API pricing.
- It gives X publishing a low paid entry tier.
- It reduces the jump from Free to Basic.
- It captures users who do not need Inbox or full dashboard workflows.

Recommended API limits:

| Limit | Recommendation |
| --- | --- |
| Monthly posts | Keep `1,000 posts/mo` |
| Workspace users | Keep `1` |
| Profiles | Keep `2` |
| Connected social accounts | Allow enough for real developer usage; recommended soft/fair-use cap rather than public per-account billing |
| Managed users | Permit testing/light usage, but make Growth the plan for customer-facing embedded deployment; exact enforcement model requires product decision before implementation |
| Connect sessions | Do not hard-cap create attempts by packaging; rely on abuse/rate-limit monitoring unless usage data shows a need for a paid allowance |
| X publishing | Include |
| Analytics | Keep read-only Analytics API |
| Inbox | Keep unavailable |
| Platform credentials | Keep unavailable |
| Hosted Connect branding | Keep unavailable |

Implementation note:

- API managed-user and managed-account allowances are new packaging/enforcement decisions, not existing limit knobs.
- API Connect Session creation should remain developer-friendly and should not be metered as a normal plan allowance unless abuse or cost data justifies it.
- API should not introduce visible per-connected-account billing. If account count is constrained, present it as fair-use or operational safety guidance.

Rationale:

- API should not become "Growth but cheaper."
- API customers should be able to build, test, and run small workloads.
- Customer-facing embedded products should graduate to Growth.

Recommended copy:

> API is for developers who need publishing infrastructure. If your own users are connecting social accounts inside your app, Growth is the embedded plan.

### 8.3 Basic

Basic should remain the operating-console plan.

Recommended Basic positioning:

> Run your own social workflow with Inbox, Analytics, and one-platform custom credentials.

Recommended adjustments:

- Keep Inbox and full Analytics.
- Keep one-platform platform credentials if that is the intended current packaging.
- Keep `5 profiles`.
- Keep `1 workspace user`.
- Do not position Basic as the main embedded SaaS plan.

Copy note:

- Avoid saying "full white-label" for Basic.
- Say "one-platform custom credentials" or "one-platform native OAuth" if that is the product reality.

### 8.4 Growth

Growth should become the flagship plan for the Zernio comparison.

Recommended Growth positioning:

> Embed UniPost into your own product without per-connected-account billing.

Recommended Growth package:

- all supported platform credentials
- hosted Connect branding
- optional removal of "Powered by UniPost"
- high managed-user and managed-account limits, with the exact enforcement surface defined before implementation
- Connect Session creation that is not hard-capped by packaging, while still protected by abuse/rate-limit monitoring
- `25 profiles`
- `3 workspace users`
- `7,500 posts/mo`

Implementation note:

- Growth managed-user and connected-account limits are new packaging surfaces unless they can reuse existing operational rate limits for queue protection.
- Growth Connect Session creation should stay frictionless for legitimate onboarding flows; use abuse/rate-limit monitoring rather than a normal monthly packaging cap.

Growth should be the first plan where the pricing page explicitly says "built for your users connecting accounts inside your app."

### 8.5 Team

Team should stay focused on collaboration and operational scale.

Recommended Team positioning:

> Multi-operator publishing and embedded account operations.

Recommended Team package:

- unlimited profiles
- unlimited workspace users
- RBAC
- per-member API keys
- audit log
- priority support
- higher operational limits

---

## 9. Competitor positioning

### 9.1 Zernio comparison page

The current Zernio comparison data and pages should be updated because the repo still references Zernio's older tier/add-on model while Zernio's public pricing pages now show pay-per-connected-account pricing.

Phase 1 must update these surfaces together so public messaging does not contradict itself:

- `dashboard/src/data/competitors/zernio.ts`
- `/alternatives/zernio`
- `/compare`
- `dashboard/src/app/pricing/page.tsx` FAQ copy that references Zernio's old `$9/mo` add-ons
- SEO metadata or comparison snippets fed by the stored Zernio competitor data

Publishing prerequisite:

- Capture a dated screenshot or archive of Zernio's current pricing page before shipping public competitor claims.

New hero direction:

> UniPost vs Zernio - Embedded publishing without the connected-account tax

New subhead direction:

> Zernio now prices by connected social account. UniPost self-serve plans are based on product stage and monthly posting capacity, making UniPost a stronger fit for apps whose users connect many low-volume accounts.

Recommended "Choose UniPost if" bullets:

- You are building an app where your own users connect social accounts.
- You need predictable self-serve pricing as connected accounts grow.
- Your users are low- or medium-volume, so unlimited posts are less valuable than account-count flexibility.
- You want hosted Connect, `external_user_id`, managed users, and API-first account onboarding.
- You want a low-cost API tier and a clear embedded Growth tier.

Recommended "Choose Zernio if" bullets:

- You need platforms UniPost does not support yet.
- You need Ads management as a first-class product surface.
- You prefer unlimited posts and are comfortable paying by connected account.
- Your connected-account count is low enough that per-account billing is predictable.

### 9.2 Pricing page

Add a short "Built for embedded apps" section below the plan cards or near the Growth card.

Suggested copy:

> No connected-account tax on self-serve plans.
>
> If your app has 100 users and each connects 2 social accounts, that is 200 connected accounts. UniPost does not turn each one into a separate self-serve billing line. Pick your plan by product stage, monthly post capacity, and whether you need embedded onboarding features.

### 9.3 Docs

Docs should clarify:

- `external_user_id` is for the customer's own end users.
- workspace users are people who log in to UniPost.
- profiles are brand/project groupings.
- connected social accounts are the accounts that can publish.
- self-serve plans are not priced per connected social account.

Recommended docs surfaces:

- `/docs/connect-sessions`
- `/docs/api/users/list`
- `/docs/api/accounts/list`
- `/docs/pricing`
- `/docs/white-label`
- `/docs/platform-credentials`

---

## 10. Product gates and limits

### 10.1 Free plan gates

Keep these existing gates:

- Free cannot publish to X.
- Free cannot use Inbox.
- Free cannot use Analytics.
- Free cannot create platform credentials.
- Free cannot customize hosted Connect branding.
- Free hard-blocks monthly posts once quota would be exceeded.

Add these new enforcement systems:

- Free blocks new managed connected social accounts above its account cap.
- Free blocks new managed users above its evaluation cap of `3` distinct completed `external_user_id` values.
- Free blocks new active API keys above its API-key cap.
- Free blocks new webhook endpoints above its webhook cap.

Connect Session creation attempts should not have a plan-packaging cap. These Free API-key, webhook, connected-account, and managed-user caps do not currently exist as simple configuration values. They require count queries, admission checks on creation/connect routes, user-facing limit state, and tests.

### 10.2 API plan gates

Keep these existing gates:

- API can publish to X.
- API can use read-only Analytics API.
- API cannot use Inbox.
- API cannot create platform credentials.
- API cannot customize hosted Connect branding.

Add or define these new packaging rules:

- API should have enough managed-account capacity for testing and light production.
- API should point embedded production customers toward Growth.

### 10.3 Growth gates

Keep these existing gates:

- Growth can create platform credentials for all supported platforms.
- Growth can customize hosted Connect branding.
- Growth can hide "Powered by UniPost".

Add or define these new packaging rules:

- Growth has materially higher managed-user, managed-account, request, and queue limits than API.
- Growth should not hard-cap Connect Session creation attempts as a normal packaging dimension; monitor for abuse and operational pressure instead.
- Growth should remain the first self-serve plan explicitly positioned for embedded customer-facing account connection.

### 10.4 Existing-user behavior

New caps should be enforced on new actions, not destructive cleanup.

If a workspace already has more connected accounts, managed users, profiles, or webhooks than the new plan limit:

- keep existing records
- block new creation
- show upgrade guidance
- avoid deleting or disconnecting anything automatically

---

## 11. Billing and fair-use policy

UniPost should not bill self-serve customers per connected account, but it should reserve the right to move very large or operationally expensive deployments to Enterprise.

Recommended fair-use language:

> Self-serve plans are not billed per connected social account. Extremely large account counts, high-frequency token refresh workloads, heavy inbox/analytics sync, or unusual automation patterns may require Enterprise terms so we can size infrastructure and support correctly.

This protects UniPost from cost leakage while preserving the marketable difference from Zernio.

---

## 12. Implementation phases

### Phase 1 - Positioning and docs

Scope:

- Capture dated evidence of Zernio's current public pricing before publishing specific Zernio comparison claims.
- Update `dashboard/src/data/competitors/zernio.ts` to reflect Zernio's current per-connected-account pricing.
- Update `/alternatives/zernio` to reflect Zernio's current per-connected-account pricing.
- Update `/compare` so its main comparison table uses the same Zernio model.
- Update `dashboard/src/app/pricing/page.tsx` to remove the stale Zernio `$9/mo` add-on claim.
- Add an embedded-app pricing section to `/pricing`.
- Add docs clarification for workspace users vs managed users vs connected social accounts vs profiles.
- Update FAQ copy for API, Growth, and Free.

No backend changes required except copy/data.

### Phase 2 - Free plan enforcement

Scope:

- Build new backend enforcement for the Free managed connected-account cap.
- Build new backend enforcement for the Free managed-user cap of `3` distinct completed `external_user_id` values.
- Do not add a plan-packaging cap for Connect Session creation attempts; keep abuse/rate-limit monitoring separate.
- Build new backend enforcement for the Free API-key cap.
- Build new backend enforcement for the Free webhook endpoint cap.
- Add count queries, route-level admission checks, dashboard/API limit state, and tests for each new cap.
- Audit existing plan-gate error patterns for Inbox, Analytics, X publishing, and platform credentials before choosing status codes. New gates should follow the existing UniPost convention rather than introducing a second convention.
- Do not add a feature flag for Phase 2 Free-plan enforcement.

### Phase 3 - API vs Growth differentiation

Scope:

- Define API and Growth managed-user and managed-account allowances before implementation.
- Build any required backend enforcement for those allowances if they are hard limits.
- Surface those limits in `/v1/limits`.
- Add dashboard upgrade copy when customers approach embedded usage.
- Add Growth-specific messaging in Connect Session and platform credential docs.

This phase is product packaging plus new enforcement work. It should not be estimated as a configuration-only change unless Phase 2 creates reusable limit primitives first.

### Phase 4 - Enterprise escalation

Scope:

- Add internal heuristics to flag accounts with unusual connected-account count, sync volume, or queue pressure.
- Add admin reporting for managed users, connected accounts, and Connect Session volume by plan.
- Add sales/support playbook for high-scale self-serve customers.

---

## 13. Metrics

Measure:

- number of connected accounts by plan
- number of managed users by plan
- Connect Sessions created by plan
- number of Free workspaces that hit each new cap
- number of upgrades after hitting each cap
- absolute Free to API upgrades
- absolute Free to Growth upgrades
- absolute API to Growth upgrades
- monthly posts by plan
- gross margin proxy by plan
- Zernio alternative page conversion rate
- pricing page CTA click-through from embedded-app section
- support tickets related to "users", "profiles", and "accounts" confusion

Conversion rates should be secondary until the volume is large enough to make percentages meaningful. At current low paid conversion volume, absolute counts are more useful for decision-making.

Success signals:

- higher Growth conversion from embedded-app traffic
- fewer confused questions about whether app end users count as workspace users
- fewer Free workspaces running production-like managed-user and connected-account flows
- `/alternatives/zernio` becomes a stronger acquisition page for customers with many connected accounts

---

## 14. Risks

### 14.1 Free becomes less generous

Risk:

- Free plan restrictions may reduce signups or experiments.

Mitigation:

- Keep 100 posts/month.
- Keep API, dashboard, MCP, scheduling, and webhooks available.
- Make limits explicit before users hit them.
- Keep upgrade path to API at `$10/mo`.

### 14.2 Account-count cost leakage

Risk:

- A customer can connect many accounts but rarely post, creating background token refresh, support, and storage costs without much revenue.

Mitigation:

- Add fair-use language.
- Add Enterprise escalation heuristics.
- Gate heavy sync features by plan.
- Keep Inbox and full Analytics off Free/API.

### 14.3 API plan cannibalizes Growth

Risk:

- Embedded customers may choose API and avoid Growth.

Mitigation:

- Keep API unbranded and limited for managed-user scale.
- Put hosted Connect branding and platform credentials on Basic/Growth as intended.
- Make Growth the plan named in embedded-app copy.

### 14.4 Messaging becomes too negative

Risk:

- Overemphasizing Zernio's pricing could feel reactive.

Mitigation:

- Lead with UniPost's positive fit: predictable embedded pricing.
- Use Zernio comparison only where appropriate.
- Avoid hostile copy.

---

## 15. Open questions

Recommended default answers for Phase 2 are captured in `docs/prd-embedded-pricing-strategy-execution-notes.md`.

1. What exact Free connected-account cap should ship: `2`, `3`, or `5`? Current recommendation: `2` managed connected accounts.
2. Free managed-user limit basis is decided for Phase 2: `3` distinct completed `external_user_id` values. Connect Sessions created should not count toward this cap.
3. Should API have a hard managed-user cap, a soft warning, or only fair-use language?
4. Should Basic continue to include one-platform platform credentials, or should all platform credentials move to Growth?
5. Should Growth include unlimited managed users publicly, or should it keep a high fair-use threshold with Enterprise escalation?
6. What exact API and Growth managed-user and connected-account allowances should ship?
7. Should the pricing page show the 200-account Zernio example directly, keep it on the Zernio alternative page only, or show a lighter version on pricing and the full calculation on the alternative page?

---

## 16. Recommended decisions

1. Keep UniPost self-serve pricing based on product tier and monthly post capacity.
2. Do not add per-connected-account billing to self-serve plans.
3. Retain the API plan at `$10/mo`.
4. Reposition API as paid developer/light production, not embedded SaaS.
5. Reposition Growth as the embedded SaaS plan.
6. Tighten Free with low account/user caps while keeping Connect Session creation developer-friendly.
7. Add "no connected-account tax" messaging to pricing and Zernio comparison pages.
8. Update Zernio comparison data immediately because the current page reflects Zernio's older pricing model.

---

## 17. Acceptance criteria

This PRD is accepted when:

- The team agrees whether Free should cap connected accounts and managed users, and agrees that Connect Session creation attempts should not be hard-capped by normal plan packaging.
- The team agrees to retain or remove the API plan. Recommended: retain.
- The team agrees that Growth is the primary embedded-app plan.
- The team agrees whether Basic keeps one-platform platform credentials.
- The implementation plan can be split into positioning-only work and backend enforcement work.
- Updated public copy can explain the difference between workspace users, managed users, profiles, and connected social accounts without ambiguity.

Phase-level acceptance criteria:

- Phase 1 is complete when the stored Zernio competitor data, `/alternatives/zernio`, `/compare`, and `/pricing` no longer contain the old Zernio tier/add-on claim, and the PR or internal notes include dated evidence for the current Zernio pricing claim.
- Phase 2 is complete when a Free workspace cannot create a third managed connected account if the cap is `2`, cannot exceed `3` distinct completed managed users, cannot create more than the allowed API keys or webhook endpoints, does not hard-block Connect Session creation attempts by monthly packaging count, receives error responses that match existing UniPost plan-gate conventions, and keeps existing over-limit records intact.
- Phase 3 is complete when API and Growth embedded-usage allowances are defined, surfaced through the chosen limits interface, enforced where intended, and accompanied by upgrade guidance from API to Growth.
