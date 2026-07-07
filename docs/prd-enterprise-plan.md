# PRD: Enterprise Plan

Date: 2026-07-07
Owner area: Pricing, Billing, Infrastructure, Customer Success
Status: Draft

## Problem

UniPost's public pricing now positions Team as the highest self-serve plan:

- Team is `$149/mo`.
- Team includes unlimited monthly UniPost posts.
- Team includes unlimited profiles and unlimited users.
- Team includes RBAC, per-member API keys, audit log, and priority support.

This is a strong self-serve offer for agencies, internal marketing teams, and
multi-operator workspaces. It is not enough for customers whose cost or
operational risk comes from high connected-account count, heavy platform usage,
large scheduled queues, high-volume Inbox or Analytics workflows, procurement,
security review, or SLA requirements.

The current Enterprise positioning is too shallow. It appears as custom terms,
SLA, dedicated support, security review, and contract flexibility, but it does
not clearly explain why an unlimited Team customer should upgrade. Without a
clear Enterprise boundary, a high-usage customer can reasonably expect `$149/mo`
to cover usage patterns that may require reserved infrastructure, manual quota
planning, or custom commercial terms.

## Goals

- Preserve Team as a simple and strong self-serve plan with unlimited monthly
  UniPost posts.
- Define Enterprise as a contracted capacity and support plan, not as "more
  posts than Team."
- Give high-usage customers a clear upgrade path before their usage creates
  platform quota, infrastructure, support, or gross-margin pressure.
- Make the public pricing page explain the boundary between unlimited Team usage
  and Enterprise-grade guarantees.
- Keep UniPost differentiated from connected-account-tax pricing on self-serve
  plans while still allowing Enterprise contracts to account for active account
  volume.
- Give Customer Success and Sales concrete internal triggers for starting an
  Enterprise conversation with high-usage Team workspaces.
- Create a PRD that can drive a later implementation of pricing page copy,
  billing/admin surfaces, and sales qualification.

## Non-Goals

- Do not remove unlimited monthly posts from Team.
- Do not convert self-serve pricing to per-connected-account pricing.
- Do not add automatic usage-based billing in the first Enterprise rollout.
- Do not promise to bypass third-party platform rate limits, quota limits, spam
  controls, or review requirements.
- Do not add a feature flag for the packaging and pricing-page copy work. A
  later runtime capacity rollout may still need its own rollout guard.
- Do not build a full `/enterprise` landing page in the first pricing-page
  rollout unless the user explicitly expands scope.

## Current Product Context

### Existing plans

The current plan ladder is:

- Free
- API
- Basic
- Growth
- Team
- Enterprise

Team is currently the visible top self-serve tier on the public pricing page.
Enterprise exists in product and database concepts, but the public pricing page
treats it as a sales banner rather than as a fully defined commercial package.

### Existing Enterprise runtime behavior

Enterprise already appears in parts of the backend as a plan id. In several
runtime areas it follows Team defaults:

- media retention follows Team windows
- integration log retention is longer than lower plans
- runtime rate-limit envelopes currently carry over from Team unless manually
  tuned
- plan gates generally include Enterprise wherever Growth or Team unlocks a
  paid feature

This means the first Enterprise PRD can focus on packaging and commercial
boundaries before requiring a full runtime architecture change.

## Product Positioning

### Team

Team is the self-serve plan for collaboration and normal production scale.

Public promise:

- unlimited monthly UniPost posts
- unlimited profiles
- unlimited users
- RBAC and per-member API keys
- audit log
- priority support
- shared UniPost infrastructure

Important boundary:

- Team has no monthly UniPost post quota, but platform safety limits,
  third-party API quotas, abuse controls, and shared-infrastructure fairness
  still apply.
- Team does not include reserved capacity, contractual throughput guarantees,
  custom platform-volume commitments, security review, procurement support, or
  a custom SLA.

### Enterprise

Enterprise is the contracted plan for customers who need guarantees, custom
capacity, and commercial flexibility.

Public promise:

- priority support and capacity planning
- custom platform-volume planning
- SLA and dedicated support
- security review and procurement support
- custom commercial terms

Enterprise should feel like a different buying motion from Team, not the sixth
card in the same self-serve grid.

Dedicated or bring-your-own platform credentials should remain a sales-only
conversation in v1. They are platform- and app-review-dependent, so they should
not appear as a first-run public pricing-page promise.

## Target Customers

Enterprise is for customers with at least one of these needs:

- A SaaS or embedded product with many end customers connecting social accounts.
- A brand group, agency, or media operation with a large number of active
  managed accounts.
- Sustained high-volume publishing, scheduling, Inbox sync, or Analytics refresh
  usage.
- Need for reserved queue capacity or predictable throughput during campaigns.
- Need for dedicated support during onboarding or incidents.
- Need for security review, DPA, vendor review, procurement, invoicing, or
  custom contract terms.
- Need to plan around X, LinkedIn, Meta, TikTok, YouTube, or other platform
  quota constraints.

## Pricing Strategy

Enterprise is sales-led and contract-based in the first rollout.

Recommended public label:

- `Custom`

Recommended internal sales floor:

- Target contracts should normally start at `$999/mo+`.
- Exception handling below that floor should be explicit and should usually be
  reserved for strategic logos, design partners, or annual prepay.
- The `$999/mo+` floor is intentionally about 6.7x Team. It creates room for
  onboarding, support, quota investigation, incident response, and manual
  contract handling without turning every large Team workspace into a
  low-margin support account.

Pricing dimensions:

- Base platform fee: access to Enterprise support, contract terms, and account
  management.
- Capacity commitment tier: publishing throughput, worker priority, queue
  priority, or other operational guarantees, used only when Operations can
  actually fulfill the commitment.
- Active account/profile band: a negotiated band based on active managed
  customer profiles, connected accounts, or workspace scale.
- Premium platform usage: X, high-volume DMs, Analytics refresh, historical
  sync, high-frequency polling, or other usage with direct third-party quota or
  cost pressure.
- Support and compliance tier: dedicated Slack or email channel, security
  review, onboarding, procurement, DPA, and incident response expectations.

The public pricing page should not expose this formula in full. It should expose
the buying reason and route customers to sales.

## Cost Drivers

Enterprise pricing should be anchored to real operational cost drivers, even
when the first rollout stays manual.

Primary cost drivers:

- Premium platform calls: X, YouTube uploads, high-volume Meta/Instagram,
  LinkedIn, TikTok, and other platform paths where quota or paid access can
  become the binding constraint.
- Provider reads: Inbox sync, comment/DM polling, Analytics refresh, historical
  sync, and any workflow that spends platform quota without creating a post.
- Worker capacity: queue depth, retry volume, video/media processing, high
  campaign concurrency, and scheduler load.
- Storage and retention: uploaded media size, failed-post troubleshooting
  windows, and long scheduled backlogs.
- Support cost: onboarding, platform credential diagnosis, security review,
  incident response, custom reporting, and procurement work.

The first internal pricing model should not need exact cost accounting per
request. It should classify accounts into bands:

- `Standard Team`: normal self-serve use, no outreach required.
- `Enterprise candidate`: high usage or procurement/security need, start a
  sales conversation.
- `Enterprise required before guarantee`: customer asks for SLA, custom capacity
  terms, custom platform-volume terms, or campaign throughput commitments.

## Recent Customer Signal

A recent support case showed the target pattern: a trialing customer connected
many accounts, scheduled densely, and explicitly asked about moving to the
largest package after trial. This is not enough to make Enterprise a near-term
volume revenue engine by itself, because current signup and paid-conversion
counts are still small. It is enough to validate the need for:

- internal high-usage lead triggers
- clear Team fair-use language
- a sales-led Enterprise path before a customer expects `$149/mo` to cover
  custom capacity and support

## Fair-Use Boundary

The pricing page and docs should use a fair-use boundary that preserves Team's
marketing strength without implying unlimited third-party capacity.

Recommended copy:

> Team has no monthly UniPost post quota. Platform safety limits, third-party
> API quotas, abuse controls, and shared-infrastructure fairness still apply.
> Customers needing capacity planning, SLA, or custom platform-volume terms
> should use Enterprise.

Rules:

- Do not describe Team as "unlimited except..." in the card headline. Keep the
  card simple.
- Put the boundary in a nearby pricing note and in FAQ/docs where a buyer is
  already thinking about limits.
- Do not make Enterprise sound punitive. It is the plan for guarantees and
  custom operating terms.
- Do not imply that Enterprise can guarantee delivery when a third-party
  platform blocks, rate-limits, rejects, or requires review for a customer.

### Internal Team Fair-Use Review Thresholds

These thresholds are internal qualification triggers. They are not public caps,
do not change Team's unlimited monthly UniPost post promise, and should not
automatically block legitimate customers.

Customer Success should review a Team workspace for Enterprise outreach when it
hits any of these signals:

- Active social accounts: `100+` connected social accounts used in the last 30
  days.
- Active profiles: `50+` profiles with posting, scheduling, Inbox, or Analytics
  activity in the last 30 days.
- Sustained publishing: `1,000+` successful platform posts in a day for at
  least `3` days in a rolling `7` day window.
- Monthly publishing: `25,000+` successful platform posts in a rolling `30` day
  window.
- Scheduled backlog: `5,000+` active scheduled parent posts, or `2,000+`
  scheduled posts due in the next `24` hours.
- Provider quota pressure: repeated provider quota or rate-limit errors on the
  same workspace, platform, or app identity in a rolling `7` day window.
- Support pressure: repeated support escalations caused by platform quota,
  credential ownership, campaign timing, or high-volume retry behavior.
- Commercial pressure: requests for SLA, security review, DPA, invoicing,
  procurement, dedicated support, or campaign throughput guarantees.

Abuse, spam, and platform-safety enforcement remain separate from Enterprise
qualification. A workspace can be blocked for abuse even if it is not an
Enterprise candidate.

## Pricing Page Requirements

### Layout

The pricing page should keep the self-serve card grid focused on:

- Free
- API
- Basic
- Growth
- Team

Enterprise should not be a sixth same-size card in the self-serve grid.

Instead, add a distinct Enterprise section after the self-serve cards or after
the usage/fair-use note. The section should visually read as a sales-led
contract option.

### Enterprise section content

Recommended title:

- `Enterprise`

Recommended subtitle:

- `Priority support, capacity planning, and custom platform-volume terms for high-scale teams.`

Recommended price label:

- `Custom`

Recommended CTA:

- `Contact sales`

Recommended content blocks:

- `Capacity planning`
  - High-volume planning and priority support for large workspaces.
- `Platform-volume planning`
  - Contracted terms for high connected-account or premium platform usage.
- `SLA and security`
  - Dedicated support, SLA, security review, procurement, and DPA support.

Public copy must not promise `reserved capacity` until the runtime capability is
implemented or contract-specific manual operations can actually deliver it. In
Phase 2, use `capacity planning`, `priority support`, or `custom
platform-volume terms`. Reserve the phrase `reserved capacity` for Phase 4 or
for signed contracts where Operations has approved a concrete fulfillment path.

### Team copy update

Team should remain a self-serve plan with:

- `Unlimited posts/mo`
- `Unlimited profiles and unlimited users`
- `Priority support`

Add a nearby explanatory note, not a downgrade in the card:

- Team is unlimited for monthly UniPost post quota.
- Enterprise is for capacity planning, SLA, and custom platform-volume terms.

### FAQ updates

Add or update FAQ entries:

- `What does unlimited Team usage mean?`
- `When do I need Enterprise instead of Team?`
- `Can Enterprise increase third-party platform quotas?`

Required answer semantics:

- Team has no monthly UniPost post quota.
- Platform safety caps and third-party quotas still apply.
- Enterprise can help plan, isolate, or contract around usage, but it
  cannot override platform-owned limits or review processes.
- `Custom` on Enterprise means contract-defined terms. It is a superset of
  Team's unlimited monthly UniPost post promise, not a smaller quota.

## Docs Requirements

Update the developer pricing docs so buyer-facing and developer-facing semantics
match.

Required docs changes:

- Plans and limits should list Enterprise as `Custom / Contract`.
- Monthly posts for Enterprise should remain `Custom`, not "unlimited."
- Docs should explain that `Custom` means the monthly post terms are defined by
  contract and may include no UniPost monthly post quota, custom capacity terms,
  or other account-specific guarantees.
- API behavior should say Enterprise contracts may include custom operational
  terms, but platform safety caps and third-party quotas still apply.
- Media retention may remain Team-equivalent in v1 unless a contract says
  otherwise.
- Runtime rate limits may remain Team-equivalent in v1 unless manually tuned for
  a contract.

## Billing and Admin Requirements

First rollout:

- Enterprise remains sales-led.
- No self-serve Stripe checkout for Enterprise.
- Billing dashboard should route Enterprise interest to contact sales.
- Admin should be able to identify Enterprise workspaces by
  `plan_id = enterprise`.

Later rollout:

- Add internal admin notes for contracted capacity, account band, support tier,
  SLA terms, and premium platform usage terms.
- Add a simple internal workspace contract profile if manual notes become
  operationally risky.
- Add usage reporting that helps Customer Success identify Team customers who
  should be contacted for Enterprise.

### Existing Enterprise Workspaces

If there are no active Enterprise contracts at rollout time, no migration is
required. If any workspace already has `plan_id = enterprise`, preserve its
current runtime behavior and support expectations until Customer Success
explicitly records a contract profile. Existing Enterprise workspaces should not
be downgraded, forced into self-serve billing, or assigned new capacity terms
silently.

## Upgrade Qualification

The product should recommend Enterprise when one or more signals appear:

- Very high number of active connected accounts or profiles.
- Sustained high monthly publish volume on Team.
- Frequent platform quota or rate-limit incidents.
- Need for dedicated support during launch or campaigns.
- Need for procurement, security review, DPA, invoicing, or contract terms.
- Need for predictable campaign throughput.
- Need for premium platform quota planning.

Internal lead triggers:

- Team workspace exceeds an internal active-account threshold.
- Team workspace sustains high daily publishing volume for multiple days.
- Team workspace has repeated provider quota errors.
- Team workspace has repeated support escalations tied to platform limits.
- Team workspace requests SLA, security review, or invoicing.

These triggers should be available to Sales and Customer Success before any
automatic metered billing work. They can start as a manual admin query or
dashboard report.

## Implementation Phases

### Phase 1: Packaging and PRD

- Write this PRD.
- Align internal understanding that Enterprise is about guarantees and custom
  terms, not more posts than Team.

### Phase 2: Public pricing page and docs

- Keep five self-serve pricing cards.
- Add a stronger standalone Enterprise section.
- Add Team fair-use boundary copy.
- Add Enterprise FAQs.
- Update developer pricing docs.
- Validate pricing page on desktop and mobile.
- Use `capacity planning` and `priority support` language until Operations can
  fulfill reserved capacity in a contract.
- Explain that Enterprise `Custom` is contract-defined and can include terms
  equal to or broader than Team's unlimited monthly UniPost post promise.

### Phase 3: Internal sales and admin workflow

- Define Enterprise qualification thresholds.
- Add internal notes or admin fields for contract terms if needed.
- Add support/sales routing for Contact Sales submissions.
- Add reporting for high-usage Team accounts that may need Enterprise outreach.

### Phase 4: Runtime capacity controls

- Add contract-aware rate-limit overrides only when needed.
- Add reserved or priority queue behavior only when there is a signed use case.
- Add contract-specific platform usage reporting.
- Keep default Enterprise runtime behavior Team-equivalent until the contract
  requires more.
- After this phase exists, public pricing may use `reserved capacity` only for
  terms that the runtime and Operations process can actually honor.

## Success Metrics

- Enterprise CTA click-through rate from the pricing page.
- Number of Enterprise-qualified leads.
- Conversion rate from high-usage Team workspaces to Enterprise conversations.
- Gross margin improvement for high-usage customers.
- Reduction in support escalations caused by unclear Team vs Enterprise
  expectations.
- Reduction in cases where a customer expects Team to include custom SLA,
  reserved capacity, or unlimited third-party platform quota.

Because current traffic and paid conversion volume are small, Phase 2 should
treat these as directional and qualitative signals rather than hard numerical
targets. Early success can be one or two correctly qualified Enterprise
conversations, clearer support routing, and fewer ambiguous "largest package"
questions.

## Risks and Mitigations

### Risk: Team unlimited feels weakened

Mitigation:

- Keep `Unlimited posts/mo` in the Team card.
- Explain boundaries in notes and FAQ rather than in the headline.
- Frame Enterprise as guarantees and custom terms, not as a penalty for usage.

### Risk: Enterprise feels vague

Mitigation:

- Use concrete Enterprise value pillars: capacity planning, platform-volume
  planning, SLA/security, dedicated support.
- Use `Custom` publicly, but keep an internal sales floor so the plan has a real
  commercial anchor.

### Risk: Public copy outruns runtime capability

Mitigation:

- Do not use `reserved capacity` in public Phase 2 copy.
- Use `capacity planning`, `priority support`, and `custom platform-volume
  terms` until runtime or Operations can fulfill reserved capacity.
- Reintroduce `reserved capacity` only after Phase 4 or an approved
  contract-specific operating plan.

### Risk: Enterprise Custom reads weaker than Team Unlimited

Mitigation:

- Explain that Enterprise `Custom` means contract-defined usage terms.
- State in FAQ/docs that Custom can include no UniPost monthly post quota,
  custom capacity terms, SLA, or account-specific guarantees.
- Do not present Enterprise as a lower quota than Team.

### Risk: Customers expect Enterprise to bypass platform limits

Mitigation:

- Explicitly state that Enterprise cannot override third-party platform-owned
  rate limits, quota, review, spam controls, or content policy enforcement.
- Sell quota planning, isolation, platform credential strategy, and support
  coordination instead.

### Risk: First rollout overbuilds billing infrastructure

Mitigation:

- Keep v1 sales-led and manual.
- Add contract-aware runtime controls only when there is a signed customer need.

### Risk: Internal floor price feels arbitrary

Mitigation:

- Tie the `$999/mo+` floor to cost-driver bands rather than to posts alone.
- Review the floor after the first qualified Enterprise conversations and
  support cases.
- Discount below the floor only for explicit strategic reasons.

## Acceptance Criteria

- A PRD exists at `docs/prd-enterprise-plan.md`.
- The PRD defines Enterprise as a contracted capacity and support plan.
- The PRD preserves Team unlimited monthly UniPost posts.
- The PRD states that Team does not include reserved capacity, custom SLA, or
  unlimited third-party platform quota.
- The PRD recommends a standalone Enterprise section on the pricing page instead
  of a sixth self-serve card.
- The PRD includes public pricing copy direction, FAQ semantics, docs
  requirements, and implementation phases.
- The PRD defines internal Team fair-use review thresholds for Sales and
  Customer Success.
- The PRD prevents public `reserved capacity` language before Phase 4 or before
  a signed contract has an approved fulfillment path.
- The PRD explains that Enterprise `Custom` is contract-defined and not weaker
  than Team unlimited.
- The PRD includes a rough cost-driver model for Enterprise pricing.
- Phase 2 implementation should render a standalone Enterprise section, update
  FAQ/docs semantics, and pass desktop and mobile visual checks.
- The PRD contains no unresolved placeholders.
