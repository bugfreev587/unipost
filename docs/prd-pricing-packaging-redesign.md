# UniPost — Pricing & Packaging Redesign PRD
**Shift UniPost from pure post-volume pricing to product-tier pricing that matches API + dashboard + Inbox + Analytics**
Version 1.0 | May 2026

---

## 1. Background

### 1.1 The product problem

UniPost no longer behaves like a narrow "social posting API only" product.

Today, the product includes:

- multi-platform publishing API
- dashboard-based manual posting
- scheduling
- webhooks
- MCP server / AI-agent workflows
- Inbox
- Analytics
- white-label / native mode

That product shape is materially closer to Zernio than to PostForMe.

However, our current pricing still communicates the opposite.

On the live pricing page, the primary message is:

- "All plans include every feature. The only difference is how many posts you need per month."
  - source: [dashboard/src/app/pricing/page.tsx](/Users/xiaoboyu/unipost/dashboard/src/app/pricing/page.tsx:129)

Our own comparison pages repeat the same packaging claim:

- "UniPost's pricing is per post volume only. Every plan includes all features — no feature tiers, no upgrade pressure."
  - source: [dashboard/src/app/alternatives/[competitor]/page.tsx](/Users/xiaoboyu/unipost/dashboard/src/app/alternatives/[competitor]/page.tsx:289)

And our competitor metadata encodes UniPost as:

- `pricingModel: "Per post volume"`
  - source: [dashboard/src/data/competitors/unipost.ts](/Users/xiaoboyu/unipost/dashboard/src/data/competitors/unipost.ts:13)

This creates a packaging mismatch:

- customers evaluating the API see a usage-based product
- customers using the dashboard manually may never come close to their monthly post cap
- higher-value product surfaces like Inbox and Analytics are not monetized as differentiators
- plan upgrades feel arbitrary because the only visible step-up is "more posts"

### 1.2 Competitor reality as of May 1, 2026

We checked current public pricing pages on `2026-05-01`.

#### PostForMe

PostForMe currently positions itself as a lightweight, predictable developer API:

- starts at `$10/mo`
- emphasizes `1,000 successful posts per month`
- no visible product-tier packaging around Inbox or collaboration surfaces
- page focuses on API access and basic social features rather than an operations dashboard

Source:

- [PostForMe pricing](https://www.postforme.dev/pricing)

#### Zernio

Zernio currently positions itself as a broader social product platform:

- `Free / Build / Accelerate / Unlimited`
- product-tier packaging rather than a single "posts-only" ladder
- `Analytics`, `Comments + DMs`, and `Ads` sold as add-ons to paid tiers
- dashboard/product posture much closer to UniPost's direction

Source:

- [Zernio pricing](https://zernio.com/pricing)

### 1.3 What is broken in the current UniPost model

The current model overweights one dimension:

- monthly successful post count

And underweights the dimensions customers actually perceive:

- whether they can use UniPost as a lightweight API trial
- whether they get operational visibility
- whether they get engagement workflows
- whether they can embed UniPost into their own product
- whether they can collaborate as a team

This is especially awkward for dashboard-led users because:

- they often post manually
- they value workflow surfaces more than raw post throughput
- they can sit on a paid plan and still use only a tiny fraction of the quota

In short:

- Post volume is still a useful guardrail.
- Post volume should no longer be the core packaging story.

---

## 2. Goals

1. Reposition UniPost pricing so it matches the real product shape: API + dashboard + Inbox + Analytics.
2. Preserve a strong permanent free tier for trial, hobby, and early integration use cases.
3. Make paid plans feel meaningfully better because they unlock operational value, not just higher quota.
4. Use Inbox and Analytics as clear upgrade drivers from Free to Paid.
5. Keep the pricing simple enough for self-serve adoption.
6. Preserve monthly post limits as a cost-control guardrail, but demote them from primary package identity to secondary plan attribute.

---

## 3. Non-goals

- We are **not** changing the underlying monthly quota infrastructure in this PRD.
- We are **not** switching to automatic overage billing.
- We are **not** introducing a complicated add-on marketplace in v1 of the redesign.
- We are **not** building enterprise-only packaging first and backfilling self-serve later.
- We are **not** removing the free plan.
- We are **not** changing the current soft-limit quota behavior in this PRD.

---

## 4. Product packaging decision

### 4.1 Core decision

UniPost should move from:

- **per-post-volume-first pricing**

to:

- **product-tier-first pricing with post-volume guardrails**

That means plan names and upgrade logic should primarily communicate:

- who the plan is for
- what workflow unlocks
- what level of product maturity it supports

And only secondarily communicate:

- monthly post allowance

### 4.2 Why this is the right packaging model

Because UniPost is not only selling "message delivery."

It is selling:

- a publishing layer
- an operations surface
- an engagement surface
- a developer integration layer

If we continue to package everything as "same product, more posts," then:

- dashboard users feel misfit
- higher-value surfaces are under-monetized
- competitors like Zernio look more mature even when our self-serve story is simpler
- competitors like PostForMe anchor the comparison toward "just another posting API"

### 4.3 Pricing philosophy

The new pricing should follow four principles:

1. **Start free, not trial-only.**
   - Free remains permanent and credit-card-free.
2. **Paid plans unlock visibility and workflow, not only throughput.**
   - Inbox and Analytics become paid-plan value.
3. **Each step-up should correspond to a clearer customer stage.**
   - individual builder → growing product → small team → larger organization
4. **Keep packaging simple.**
   - One clear self-serve ladder is better than tier-plus-many-add-ons in v1.

---

## 5. Recommended plan structure

## 5.1 The ladder

Recommended public self-serve plans:

- `Free`
- `Basic`
- `Growth`
- `Team`
- `Enterprise`

This is intentionally close to the user's preferred mental model and closer to how Zernio frames product maturity.

### 5.2 Recommended pricing and positioning

#### Free — `$0/mo`

For:

- solo developers evaluating UniPost
- hobby projects
- early prototypes
- low-volume manual posting

Core promise:

- start building and publishing without a credit card

Recommended limits:

- `100 posts/month`
- `1 workspace`
- X publishing remains paid-only

Included:

- dashboard
- publishing API
- scheduling
- webhooks
- MCP server
- core platform coverage excluding paid-only X publishing
- quickstart mode

Excluded:

- Inbox
- Analytics
- White-label / native mode

Rationale:

- Free should be enough to validate the product
- but not enough to replace paid operational workflows

#### Basic — `$19/mo`

For:

- indie hackers
- small SaaS builders
- creators or operators who want a serious day-to-day tool

Core promise:

- upgrade from posting tool to operating console

Recommended limits:

- `1,000 posts/month`
- `1 workspace`

Included:

- everything in Free
- X publishing
- Inbox
- Analytics

Excluded:

- White-label / native mode

Rationale:

- This is the key monetization step.
- The value story is not "10x more posts."
- The value story is "now you can run your workflow from UniPost."

#### Growth — `$59/mo`

For:

- product teams embedding UniPost into their own app
- customers with heavier API usage
- customers who need branded onboarding

Core promise:

- turn UniPost into customer-facing infrastructure

Recommended limits:

- `5,000 posts/month`
- multiple workspaces or higher account/team allowances

Included:

- everything in Basic
- White-label / native mode
- higher operational limits
- higher queue / rate / usage thresholds where relevant

Rationale:

- White-label is a major packaging step and belongs above Basic.
- Growth should feel like the "real SaaS integration" plan.

#### Team — `$149/mo`

For:

- small teams
- agencies
- internal marketing / ops teams
- multi-operator workflows

Core promise:

- collaboration, throughput, and operational scale

Recommended limits:

- `20,000 posts/month`
- more generous workspace / account / usage ceilings

Included:

- everything in Growth
- priority support
- stronger collaboration positioning
- room for future team workflow features

Rationale:

- Team should be anchored on scale and collaboration, not just quota.

#### Enterprise — custom

For:

- larger customers with procurement, SLA, compliance, or custom-volume requirements

Core promise:

- custom commercial terms and support

Likely includes:

- custom limits
- SLA
- dedicated support
- contract flexibility
- optional deployment / security accommodations if offered

---

## 6. Feature gating recommendation

### 6.1 Inbox and Analytics

Recommendation:

- `Inbox` and `Analytics` should be **paid-plan-only**
- specifically: unavailable on `Free`, included on `Basic` and above

### 6.2 Why not make them separate add-ons in self-serve

Do **not** copy Zernio's add-on pattern in the first UniPost redesign.

Reason:

- Zernio uses add-ons because it is packaging a larger enterprise-style platform
- UniPost still benefits more from a simple self-serve ladder
- asking the customer to make two upgrade decisions is unnecessary friction:
  - first "Should I pay?"
  - then "Should I pay extra for visibility?"

For UniPost, Inbox and Analytics should do one clean job:

- make the first paid plan obviously better than free

### 6.3 White-label / native mode

Recommendation:

- keep White-label above Basic
- include it in Growth and Team

Reason:

- it maps cleanly to a more advanced product stage
- it is a stronger fit for embedded / SaaS-facing use cases than for casual manual-posting customers

### 6.4 X publishing

Recommendation:

- keep X publishing as paid-only

This already aligns with the current product direction and existing PRD:

- [docs/prd-x-paid-only-and-per-platform-daily-caps.md](/Users/xiaoboyu/unipost/docs/prd-x-paid-only-and-per-platform-daily-caps.md)

---

## 7. Recommended packaging table

This is the target public packaging matrix for v1.

| Capability | Free | Basic | Growth | Team | Enterprise |
| --- | --- | --- | --- | --- | --- |
| Dashboard posting | Yes | Yes | Yes | Yes | Yes |
| Posts API | Yes | Yes | Yes | Yes | Yes |
| Scheduling | Yes | Yes | Yes | Yes | Yes |
| Webhooks | Yes | Yes | Yes | Yes | Yes |
| MCP server | Yes | Yes | Yes | Yes | Yes |
| X publishing | No | Yes | Yes | Yes | Yes |
| Inbox | No | Yes | Yes | Yes | Yes |
| Analytics | No | Yes | Yes | Yes | Yes |
| White-label / native mode | No | No | Yes | Yes | Yes |
| Monthly posts | 100 | 1,000 | 5,000 | 20,000 | Custom |

Notes:

- exact workspace/account/member ceilings can be tuned during implementation
- the key packaging decision is the gating pattern, not the exact number in every quota field

---

## 8. Quota and overage strategy

### 8.1 Current real behavior

As of `2026-05-01`, the backend monthly quota system is a soft limit:

- the checker explicitly says `Never blocks — soft limit only`
  - source: [api/internal/quota/checker.go](/Users/xiaoboyu/unipost/api/internal/quota/checker.go:32)
- publish handlers set `X-UniPost-Usage` and `X-UniPost-Warning` headers but do not block on quota
  - sources:
    - [api/internal/handler/social_posts.go](/Users/xiaoboyu/unipost/api/internal/handler/social_posts.go:318)
    - [api/internal/handler/social_posts_bulk.go](/Users/xiaoboyu/unipost/api/internal/handler/social_posts_bulk.go:110)

Current warning thresholds:

- `>= 80%` => `approaching_limit`
- `>= 100%` => `over_limit`

### 8.2 Recommendation

Keep the existing monthly overage behavior:

- no hard stop exactly at the quota boundary
- no automatic overage fees
- no surprise billing
- sustained overuse becomes a sales / upgrade motion, not an instant product interruption

### 8.3 Why this fits the new pricing

This model works especially well with product-tier pricing because:

- the customer is not buying an exact transactional meter
- they are buying a package of workflows and capacity
- soft limits reduce fear during onboarding and product rollout
- it preserves trust with small builders and growing teams

### 8.4 Required messaging cleanup

There is currently some copy drift.

Pricing and terms mostly describe the true soft-limit behavior:

- [dashboard/src/app/pricing/page.tsx](/Users/xiaoboyu/unipost/dashboard/src/app/pricing/page.tsx:56)
- [dashboard/src/app/pricing/page.tsx](/Users/xiaoboyu/unipost/dashboard/src/app/pricing/page.tsx:57)
- [dashboard/src/app/terms/page.tsx](/Users/xiaoboyu/unipost/dashboard/src/app/terms/page.tsx:112)

But the billing UI contains a harder-sounding line:

- "Monthly limit exceeded. Upgrade to continue posting."
  - source: [dashboard/src/app/(dashboard)/settings/billing/page.tsx](/Users/xiaoboyu/unipost/dashboard/src/app/(dashboard)/settings/billing/page.tsx:280)

That should be revised to match actual product behavior before or during rollout.

Recommended replacement:

- `Monthly limit exceeded. Posting continues for now, but sustained overage will require an upgrade.`

---

## 9. Pricing page messaging

### 9.1 Current problem

The current hero copy frames UniPost as pure usage-based infrastructure:

- "All plans include every feature. The only difference is how many posts you need per month."
  - source: [dashboard/src/app/pricing/page.tsx](/Users/xiaoboyu/unipost/dashboard/src/app/pricing/page.tsx:129)

That line must change. It directly conflicts with the intended pricing redesign.

### 9.2 New messaging direction

The pricing page should communicate:

- start free
- pay for visibility, workflow, and product maturity
- use post counts as plan capacity, not as plan identity

### 9.3 Recommended hero copy

Suggested headline:

- `Start free. Upgrade for visibility, collaboration, and scale.`

Suggested subhead:

- `Use UniPost as an API or dashboard. Paid plans unlock Inbox, Analytics, X publishing, and white-label.`

### 9.4 Recommended card framing

Do not use plan labels like:

- `1,000 Social Posts`
- `2,500 Social Posts`

Instead use:

- `Free`
- `Basic`
- `Growth`
- `Team`

And show post allowance as a supporting line item inside each card.

### 9.5 Recommended comparison framing

The comparison section should highlight:

- operational visibility
- engagement workflow
- branding / embedding
- platform access
- capacity

Not only:

- post volume

---

## 10. Competitor-page implications

### 10.1 UniPost metadata

Update our own competitor/comparison data so UniPost is no longer described as:

- `Per post volume`

Recommended new label:

- `Product tier + monthly capacity`

Files affected:

- [dashboard/src/data/competitors/unipost.ts](/Users/xiaoboyu/unipost/dashboard/src/data/competitors/unipost.ts:13)

### 10.2 Zernio comparison

The Zernio alternative page should shift from:

- "we are simpler because all features are included"

to:

- "we are simpler because paid plans include Inbox and Analytics without forcing self-serve add-on decisions"

### 10.3 PostForMe comparison

The PostForMe alternative page should emphasize:

- PostForMe is lighter and more API-only
- UniPost is the better fit when the customer wants:
  - dashboard workflows
  - Inbox
  - Analytics
  - X support
  - MCP-native operations

---

## 11. Implementation scope

This PRD is about packaging and pricing presentation first.

### 11.1 Must-change surfaces in v1

- marketing pricing page
  - [dashboard/src/app/pricing/page.tsx](/Users/xiaoboyu/unipost/dashboard/src/app/pricing/page.tsx)
- pricing docs overview
  - [dashboard/src/app/docs/pricing/page.tsx](/Users/xiaoboyu/unipost/dashboard/src/app/docs/pricing/page.tsx)
- billing page messaging
  - [dashboard/src/app/(dashboard)/settings/billing/page.tsx](/Users/xiaoboyu/unipost/dashboard/src/app/(dashboard)/settings/billing/page.tsx)
- competitor/comparison metadata
  - [dashboard/src/data/competitors/unipost.ts](/Users/xiaoboyu/unipost/dashboard/src/data/competitors/unipost.ts)
  - [dashboard/src/data/competitors/zernio.ts](/Users/xiaoboyu/unipost/dashboard/src/data/competitors/zernio.ts)
  - [dashboard/src/data/competitors/postforme.ts](/Users/xiaoboyu/unipost/dashboard/src/data/competitors/postforme.ts)
- competitor comparison page copy
  - [dashboard/src/app/alternatives/[competitor]/page.tsx](/Users/xiaoboyu/unipost/dashboard/src/app/alternatives/[competitor]/page.tsx)

### 11.2 Product gating needed to make the pricing true

If the public pricing says Inbox and Analytics are paid-only, the product must enforce that.

Minimum required gating:

- Free workspaces cannot access Inbox
- Free workspaces cannot access Analytics
- Free workspaces continue to access core publishing/dashboard/API surfaces

This is separate implementation work and may be split into a follow-up engineering PRD if needed.

### 11.3 Not required for the first pricing rollout

- advanced billing migration logic
- grandfathering automation
- add-on billing architecture
- annual billing redesign
- metered overage billing

---

## 12. Migration and rollout guidance

### 12.1 Existing free users

If there are existing free users already using Inbox or Analytics, we need one of two rollout modes:

1. **Hard cutover**
   - simplest implementation
   - may frustrate active users
2. **Grandfathered preview**
   - existing free workspaces keep access temporarily
   - new free workspaces are gated immediately

Recommended default:

- grandfather existing free workspaces for a short transition window if usage is meaningful
- otherwise hard cutover is acceptable

This depends on how much real free-plan usage those surfaces already have.

### 12.2 Existing paid users

Existing paid users should map cleanly:

- current low paid plans => `Basic`
- current mid-tier plans with white-label usage => `Growth`
- current larger plans => `Team` or enterprise case-by-case

Because the current system is mostly quota-based, the key migration concern is:

- preserving access, not preserving exact plan names

### 12.3 Rollout sequence

Recommended order:

1. finalize public pricing structure and copy
2. implement feature gating required to make that copy true
3. update comparison pages and docs
4. announce the new pricing model
5. optionally migrate plan IDs / billing naming later if the backend still uses legacy IDs

---

## 13. Risks

### 13.1 Risk: too much complexity too early

If we introduce too many plan distinctions at once, the page becomes harder to understand.

Mitigation:

- keep the ladder to four self-serve plans
- do not add self-serve add-ons in v1

### 13.2 Risk: free plan becomes too weak

If Free loses too much, acquisition may slow.

Mitigation:

- keep API access, dashboard posting, scheduling, webhooks, and MCP in Free
- only gate the clearly higher-value operational surfaces

### 13.3 Risk: pricing copy outruns product truth

If the site says Inbox / Analytics are paid-only but the app still exposes them to Free, packaging credibility drops.

Mitigation:

- ship feature gating with or before pricing-page rollout

### 13.4 Risk: dashboard/manual users still do not know why they should pay

If the plan cards still overemphasize post counts, the redesign will not solve the original problem.

Mitigation:

- use persona-based plan names
- use workflow unlocks as card headlines
- move post counts into supporting copy

---

## 14. Success criteria

This pricing redesign is successful if:

1. UniPost no longer reads like "just another post-metered API."
2. The first paid tier has a clear, product-led reason to exist.
3. Dashboard/manual-posting customers can understand why they should upgrade even at low post volume.
4. Inbox and Analytics become visible upgrade drivers.
5. The site, app, docs, and comparison pages all tell the same packaging story.

---

## 15. Final recommendation

Adopt this structure:

- Free
- Basic
- Growth
- Team
- Enterprise

With these core packaging rules:

- Free keeps core API/dashboard access
- Inbox is paid-only
- Analytics is paid-only
- X stays paid-only
- White-label starts at Growth
- monthly post limits remain as soft capacity guardrails, not the primary identity of the plans

The single most important product-message change is this:

- UniPost should stop selling "more posts"
- UniPost should start selling "more operating power"

