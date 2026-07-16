# UniPost X Credits, Comments, and DMs PRD

**Status:** Draft — strategic sequencing decision required before implementation planning

**Date:** 2026-07-15

**Owner:** UniPost Product + Engineering

**Target:** Managed X API usage, X comments/replies, legacy X DMs, billing, and developer documentation

## 1. Executive summary

UniPost will add a workspace-scoped X Credits system to pay for variable X API usage without making the existing subscription plans unpredictable. Each paid plan receives a monthly X Credit allowance. Customers can buy persistent top-up credits when the included allowance is exhausted.

The approved commercial model is:

- **1 X Credit represents $0.001 of upstream X API cost for internal cost accounting.**
- Included credits reset each billing cycle and do not roll over.
- Purchased top-up credits never expire; they remain until consumed or reversed because of a refund or chargeback.
- Included credits are consumed before top-up credits.
- The base Top-up rate is **$1 = 400 X Credits**, with progressively larger bonus-credit percentages for larger purchases.
- The launch ladder is **$5 / 2,000**, **$10 / 4,200**, **$25 / 11,000**, **$50 / 23,000**, and **$100 / 48,000 Credits**.
- The ladder creates a theoretical gross-margin range of **60% down to 52%** before Stripe fees, taxes, infrastructure, support, refunds, and future X pricing changes.

X Credits are separate from the existing UniPost posts-per-month allowance. An X publish can consume both one UniPost post unit and X Credits. X Credits apply only when UniPost's managed X developer app pays the upstream API bill. Calls made through a customer's own X Platform Credentials are billed by X to that customer and do not consume UniPost X Credits.

The product rollout has two messaging phases:

1. **MVP:** X public comments/replies and legacy Direct Message APIs in UniPost Inbox.
2. **Later beta:** XChat support only after an engineering proof of concept validates its encrypted key lifecycle, official SDK readiness, webhook behavior, and production supportability.

This PRD also requires new API Reference pages and task-oriented Guidance pages. Every new Reference page must link to its relevant Guidance page, and every Guidance page must link back to the exact API Reference endpoints it uses.

This document specifies the complete target state. Section 2.3 records a required product decision about whether the full Credits ledger ships before X Inbox or is deferred until bounded Inbox usage produces real cost data.

## 2. Background

### 2.1 Current UniPost behavior

As of this PRD:

- UniPost supports X publishing, scheduling, media posts, threads, and `first_comment` as a self-reply.
- UniPost can expose X reply counts through Analytics.
- UniPost Inbox does not ingest X comments/replies or X DMs.
- The current X OAuth 2.0 flow requests publishing and read scopes, but not DM scopes.
- X publishing and new X connections require a paid UniPost plan.
- X publishing already has a per-connected-account safety cap of 20 successful publishes per UTC day.
- Inbox requires Basic or higher.
- UniPost has monthly post quotas and Stripe subscription billing, but no customer-facing variable-cost credit ledger or one-time top-up product.

### 2.2 Why credits are required

X charges for API resources rather than providing a predictable flat allowance. Current published examples include charges for creating posts, reading posts, reading DM events, sending DMs, and receiving private activity events. A post containing a URL is materially more expensive than a normal post.

The existing paid-plan gate and 20-publishes-per-account-per-day safety cap materially bound today's outbound X publishing exposure. The immediate unbounded risk introduced by this project is inbound Activity/webhook traffic: a viral post can generate paid reply or DM events without the customer explicitly initiating each cost.

The full credit model remains useful at larger scale because it makes variable usage visible, keeps the base plans simple, and creates a controlled Top-up margin. It is not assumed to be the only safe first release strategy.

### 2.3 Required sequencing decision

Product must approve one of these approaches before an implementation plan is written:

#### Option A — Full Credits foundation first

- Build the ledger, included grants, reservations, manual Top-ups, reconciliation, and customer surfaces before launching X comments and DMs.
- Strongest long-term accounting and monetization model.
- Highest initial engineering cost and longest time before customers receive X Inbox value.

#### Option B — Bounded X Inbox first (recommended for approval)

- Launch X comments/replies and legacy DMs with a billing-grade per-plan monthly weighted X-usage counter and a hard per-workspace daily inbound-spend cap.
- Reuse existing quota/gate patterns where practical, but make the new counter atomic rather than copying the current best-effort publish safety tracker.
- Do not launch purchased Top-ups or Auto top-up in this stage.
- Collect real per-operation cost, webhook volume, block rate, and customer-demand data.
- Promote the full ledger and Top-up system only when measured usage, customer requests, or margin thresholds justify it.

Option B is the review recommendation because the current outbound exposure is already bounded. This PRD retains the complete Credits design so it is implementation-ready if Option A is chosen or when Option B reaches its promotion gate. The product owner must explicitly choose; the document does not silently override the previously approved Credits direction.

Suggested promotion triggers from bounded usage to the full ledger are any of:

- Managed X cost exceeds 15% of paid subscription revenue for two consecutive months.
- At least five paying workspaces hit an X usage cap in one month and request additional capacity.
- At least three paying workspaces request purchasable Top-ups or Auto top-up.
- Manual support exceptions for X capacity exceed two hours per month.

### 2.4 Official X references

The implementation and final public copy must be checked against the then-current official documentation before launch:

- [X API pay-per-use pricing](https://docs.x.com/x-api/getting-started/pricing)
- [OAuth 2.0 Authorization Code with PKCE](https://docs.x.com/fundamentals/authentication/oauth-2-0/authorization-code)
- [Direct Messages manage quickstart](https://docs.x.com/x-api/direct-messages/manage/quickstart)
- [Direct Messages lookup integration](https://docs.x.com/x-api/direct-messages/lookup/integrate)
- [Conversation IDs and replies](https://docs.x.com/x-api/fundamentals/conversation-id)
- [Filtered Stream](https://docs.x.com/x-api/posts/filtered-stream/introduction)
- [X Activity API](https://docs.x.com/x-api/activity/introduction)
- [Webhooks](https://docs.x.com/x-api/webhooks/introduction)
- [Developer Console](https://docs.x.com/fundamentals/developer-portal)
- [XChat conversation API](https://docs.x.com/x-api/chat/get-chat-conversations)
- [XChat send message API](https://docs.x.com/x-api/chat/send-chat-message)
- [XChat encryption behavior](https://help.x.com/en/using-x/about-chat)
- [Official XChat bot sample](https://github.com/xdevplatform/xchat-bot-python)

X pricing and access rules are external dependencies. The values in this PRD are the launch catalog, not a permanent promise that X's upstream rates will never change.

## 3. Goals

1. Protect UniPost's gross margin when it pays X API usage on behalf of customers.
2. Give every paid plan a useful included X allowance.
3. Let customers continue using X through profitable, transparent top-ups.
4. Support X public comments/replies in UniPost Inbox.
5. Support legacy X DMs in UniPost Inbox, including inbound sync and outbound replies.
6. Apply deterministic, idempotent charging so a retry or duplicate webhook never double-charges a customer.
7. Give customers a clear balance, transaction history, cost estimate, and low-balance warning.
8. Obtain and configure the new X developer permissions and webhook subscriptions required by the feature.
9. Publish complete API Reference and Guidance documentation with bidirectional links.
10. Reconcile UniPost credit consumption against X's actual API usage and invoice data.

## 4. Non-goals

- Building a general-purpose credit system for every social platform in the first release.
- Replacing the existing posts-per-month allowance.
- Passing X's invoice through to customers at a 1:1 cost ratio.
- Supporting X Inbox on Free or API plans; the existing Basic-or-higher Inbox gate remains.
- Offering customer-to-customer credit transfers, cash withdrawals, or a secondary market.
- Promising XChat general availability before the technical proof of concept passes.
- Supporting X reply moderation, hide/unhide, block, mute, or abuse reporting in the MVP.
- Importing an account's unlimited historical X conversations.
- Charging UniPost X Credits for calls made with a customer's own X developer app credentials.

## 5. Terminology

| Term | Definition |
|---|---|
| X Credit | Publicly, an abstract UniPost usage unit for managed X operations. It is not currency and has no cash value. The internal cost-accounting mapping is confidential. |
| Included credits | Credits granted by a paid plan for the current billing period. They expire at the end of that period and do not roll over. |
| Top-up credits | Credits purchased separately through manual or automatic Top-up. They have no time-based expiration and remain until consumed or reversed because of a refund or chargeback. |
| Managed X app | UniPost's shared X developer app. UniPost pays X for its usage. |
| BYO X app | A customer's X app configured through UniPost Platform Credentials. The customer pays X directly. |
| X reply | A public X post that replies to another post. This is normalized as an Inbox comment workflow. |
| Legacy X DM | A Direct Message accessed through X's Direct Message Event APIs. |
| XChat | X's newer encrypted chat product and `/2/chat/*` APIs. It is a separate later-phase integration. |
| Pricing catalog | Versioned mapping from a UniPost X operation to X Credits consumed. |

### 5.1 Public versus internal pricing information

The PRD contains confidential unit-economics fields for finance and engineering. Public Pricing, Billing, API Reference, Guidance, receipts, marketing copy, metadata, and examples must not disclose:

- X's upstream dollar price per resource.
- The internal `$0.001` reference cost represented by one Credit.
- UniPost gross profit or gross-margin percentages.
- Maximum upstream cost represented by a plan allowance.

Public surfaces may disclose plan allowances, Credits consumed by a UniPost operation, Top-up price, base/bonus/total Credits, non-expiration, and conservative examples of how many operations a balance can support. Internal-only tables must be clearly labeled and excluded from public source data or generated customer-facing artifacts.

## 6. Commercial model

### 6.1 Included credits by plan

| UniPost plan | Monthly price | Existing post capacity | Included X Credits / billing cycle | Inbox eligibility |
|---|---:|---:|---:|---|
| Free | $0 | 100 | 0 | No |
| API | $10 | 1,000 | 1,500 | No |
| Basic | $19 | 2,500 | 4,000 | Yes |
| Growth | $59 | 7,500 | 12,000 | Yes |
| Team | $149 | Unlimited | 30,000 | Yes |
| Enterprise | Custom | Custom | Contract-defined | Contract-defined |

**Internal-only unit economics:** these allowances represent a maximum reference upstream X cost near 15% to 21% of self-serve subscription revenue while still providing meaningful trial capacity. Do not include this sentence or its derivation in public copy.

Included X Credits are granted at the start of the Stripe subscription billing period, not on the first day of the calendar month. Free has no X Credits because it cannot newly connect or publish to X.

### 6.2 What each plan can do with its included credits

The public Pricing page must translate abstract credit balances into concrete X operations. Display the following table under the plan cards and before the full feature-comparison matrix, with the heading **What your included X Credits can do**.

| Plan | Included X Credits | Normal posts without a URL | Posts containing a URL | Complete comment interactions | Complete DM interactions |
|---|---:|---:|---:|---:|---:|
| Free | 0 | Not available | Not available | Not available | Not available |
| API | 1,500 | 100 | 7 | Inbox not included | Inbox not included |
| Basic | 4,000 | 266 | 20 | 200 | 160 |
| Growth | 12,000 | 800 | 60 | 600 | 480 |
| Team | 30,000 | 2,000 | 150 | 1,500 | 1,200 |
| Enterprise | Custom | Custom | Custom | Custom | Custom |

Calculation definitions:

- **Normal post:** 15 credits for one X Post Create without a URL.
- **Post containing a URL:** 200 credits for one X Post Create with URL.
- **Complete comment interaction:** 5 credits to receive/read one public reply plus 15 credits to send one normal reply, or 20 credits total.
- **Complete DM interaction:** 10 credits to receive/read one DM event plus 15 credits to send one DM, or 25 credits total.
- Every displayed maximum is `floor(included credits / operation cost)`.
- Each column assumes the workspace spends its entire included balance on that one operation type. Credits are one shared balance, not separate post, comment, and DM allowances.
- The table uses the conservative normal-reply price. A reply that qualifies for X's lower summoned-reply price can consume fewer credits.
- Actual capacity can be lower when a workflow also performs paid user enrichment, backfill, or other billable X reads.
- Comments and DMs remain unavailable on API even though the plan includes X Credits, because UniPost Inbox starts on Basic.
- BYO X Platform Credentials do not consume UniPost X Credits, so this table applies only to UniPost-managed X connections.

On desktop, use the comparison table above. On mobile, render one compact card per plan with the same values rather than forcing a horizontally compressed table. If the signed-in workspace plan is known, visually mark the current plan without hiding the other plans.

The Pricing page, Plans and limits docs, Billing UI, and API Reference must derive these values from one versioned pricing-catalog configuration or a generated artifact. Do not independently hard-code operation costs and calculated capacities in multiple UI files. Source tests must fail if the displayed table drifts from the approved included allowances or operation catalog.

### 6.3 Top-up conversion

**Internal only:** the conversion and margin calculations in Sections 6.3 and the finance columns of 6.4 are confidential. Public surfaces use the pack price, base Credits, bonus, total Credits, non-expiration, and operation examples only.

The base conversion before volume bonus is:

```text
$1.00 customer price = 400 base X Credits
1 base X Credit = $0.0025 customer price
1,000 base X Credits = $2.50 customer price
```

Larger purchases receive bonus Credits. The effective Credits per dollar increase by SKU while the reference upstream cost represented by one Credit remains $0.001. Therefore there is no single effective customer price per Credit across all Top-up packs.

For the $5 base pack, the theoretical economics are:

```text
Revenue per 1,000 credits:       $2.50
Reference upstream cost:         $1.00
Gross profit before other costs: $1.50
Reference gross margin:          60%
```

The volume discount intentionally reduces theoretical gross margin as purchase size increases, but the self-serve launch ladder must not fall below 52% before payment and operating costs. This margin funds payment processing, taxes, infrastructure, observability, support, credit card disputes, refunds, X invoice variance, and the risk that X changes its pricing.

### 6.4 Top-up SKUs

**Internal finance table — do not publish the cost, profit, or margin columns:**

| SKU | Price | Base Credits | Bonus | Total Credits | Reference upstream cost | Reference gross profit | Theoretical gross margin |
|---|---:|---:|---:|---:|---:|---:|---:|
| `topup_5` | $5 | 2,000 | 0% / 0 | 2,000 | $2.00 | $3.00 | 60% |
| `topup_10` | $10 | 4,000 | 5% / 200 | 4,200 | $4.20 | $5.80 | 58% |
| `topup_25` | $25 | 10,000 | 10% / 1,000 | 11,000 | $11.00 | $14.00 | 56% |
| `topup_50` | $50 | 20,000 | 15% / 3,000 | 23,000 | $23.00 | $27.00 | 54% |
| `topup_100` | $100 | 40,000 | 20% / 8,000 | 48,000 | $48.00 | $52.00 | 52% |

The self-serve minimum is $5 and maximum single purchase is $100. A customer can make another purchase after the first completes. Larger Enterprise commitments require a custom quote and explicit margin review. No self-serve pack may exceed a 20% bonus or reduce theoretical gross margin below 52% without a new product decision.

The customer-facing pack selector contains only: price, base Credits, bonus percentage/amount, total Credits, and `Never expires`.

The Billing and Pricing surfaces should also translate each pack into conservative, single-operation examples:

| Top-up | Normal posts | URL posts | Complete comment interactions | Complete DM interactions |
|---:|---:|---:|---:|---:|
| $5 / 2,000 | 133 | 10 | 100 | 80 |
| $10 / 4,200 | 280 | 21 | 210 | 168 |
| $25 / 11,000 | 733 | 55 | 550 | 440 |
| $50 / 23,000 | 1,533 | 115 | 1,150 | 920 |
| $100 / 48,000 | 3,200 | 240 | 2,400 | 1,920 |

These values use the same definitions and floor rounding as Section 6.2. Each column assumes all purchased Credits are spent on one operation type. Comment and DM examples apply only to Basic and higher because purchasing a Top-up does not unlock Inbox on API.

### 6.5 Bucket lifecycle

1. Included credits are created as a billing-period grant.
2. Included credits expire when that subscription billing period ends.
3. Unused included credits do not roll over.
4. Top-up credits never expire and do not reset each month. A Top-up bucket always stores `expires_at = null`; the database must reject a time-based expiration on this bucket type.
5. Consumption uses the earliest-expiring included bucket first, then top-up buckets oldest-first.
6. An upgrade immediately grants the positive difference between the old and new plan allowance for the remainder of the current billing period. It does not regrant the entire new allowance if the workspace has already consumed included credits.
7. A downgrade changes the allowance on the next billing period. It does not remove purchased top-up credits.
8. If a subscription becomes Free, top-up credits remain on the workspace but managed X operations stay unavailable until a paid plan is restored.
9. Credits are workspace-scoped and non-transferable. Refunds, chargebacks, and account deletion can create reversing/removal entries, but they are not expiration events and must not be described as expiration in the UI or API.

### 6.6 Managed app versus BYO app

| Connection mode | Who pays X | UniPost X Credits | UniPost plan and post quota |
|---|---|---|---|
| UniPost Quickstart / managed X app | UniPost | Required | Still enforced |
| Customer X Platform Credentials | Customer | Not charged | Still enforced |

The connection mode must be stored on every X account and copied into every X usage ledger entry. A customer must never be charged UniPost X Credits for BYO activity.

Included and Top-up Credits form one workspace balance usable by both Dashboard and API operations on managed X connections. Buying Credits does not unlock a plan feature: API can spend them on permitted X publishing but still cannot use Inbox, Basic and above can spend them on shipped X Inbox operations, and Free cannot use managed X operations even if retained Top-up Credits are present.

## 7. Launch pricing catalog

The launch catalog uses the official X resource prices available when this PRD was written. Engineering must verify the catalog immediately before production release. The upstream-resource and effective-price columns below are internal-only; public docs expose the UniPost operation and Credits consumed, not X's dollar price or UniPost margin.

| UniPost operation | Upstream X resource | Managed-app credits | Effective customer price across $5-$100 packs | Notes |
|---|---|---:|---:|---|
| Create normal X post | Post Create | 15 | $0.0313-$0.0375 | Includes a public reply when X classifies it as normal create. |
| Create post containing URL | Post Create with URL | 200 | $0.4167-$0.5000 | Use the conservative URL classifier in Section 7.2 and reserve 200 before the call. |
| Create summoned reply | Post Create summoned | 10 | $0.0208-$0.0250 | Use only when the request qualifies under X's definition. |
| Read public post | Post Read | 5 / newly billable resource | $0.0104-$0.0125 | Local daily deduplication applies. |
| Read app-owner post | Owned Post Read | 1 / newly billable resource | $0.0021-$0.0025 | Use only when X's ownership conditions are conclusively met. Otherwise charge Post Read. |
| Read user profile | User Read | 10 / newly billable resource | $0.0208-$0.0250 | Avoid enrichment reads unless the product needs them. |
| Read legacy DM event | DM Event Read | 10 / newly billable event | $0.0208-$0.0250 | Applies to lookup/backfill reads. |
| Send legacy DM | DM Interaction Create | 15 | $0.0313-$0.0375 | Reserve before calling X. |
| Receive legacy DM webhook | `dm.received` | 10 | $0.0208-$0.0250 | Do not also charge a duplicate lookup read for the same event. |
| Receive XChat webhook | `chat.received` | 10 | $0.0208-$0.0250 | Later-phase only. |
| Send XChat message | Chat send operation | 15 | $0.0313-$0.0375 | Provisional until the XChat proof of concept confirms upstream accounting. |
| Receive public-post webhook | `post.create` | 5 | $0.0104-$0.0125 | Charge once per unique upstream event/resource/day. |
| Delete or other optional interaction | Current X priced resource | Catalog-defined | Catalog-defined | Not exposed in MVP. |

### 7.1 Catalog rules

- The catalog is versioned, for example `x-credits-2026-07-15-v1`.
- Every reservation and ledger transaction stores the catalog version and operation key.
- Historical entries are never recalculated when a future catalog changes.
- X-owned deduplication is described as a soft guarantee. UniPost therefore provides its own deterministic customer charging contract instead of mirroring an eventual X invoice line by line.
- If X adds or changes a resource price, UniPost can publish a new catalog. Customer-facing changes require updated pricing/docs and reasonable notice unless an emergency upstream change would otherwise create immediate material loss.
- Existing purchased credits retain their numeric balance. The number of credits charged by a future operation can change only through a new published catalog version.

### 7.2 X URL-post classification and reservation

UniPost must never reserve 15 Credits and later discover it owes the 200-Credit URL operation. Before a create-post or public-reply request:

1. Parse the final text after templates, variables, tracking parameters, and link rewriting are applied.
2. Classify any `http://` or `https://` token, `www.` URL, `t.co` URL, shortened/redirect URL, or X post/profile URL included in the text as a URL candidate.
3. A quote-post link included in text is a URL candidate. Mentions, hashtags, uploaded-media source URLs, and UniPost's internal media IDs are not text URLs by themselves.
4. If parsing is ambiguous or a redirect/link expander fails, choose the 200-Credit URL operation.
5. Reserve 200 Credits for a URL candidate and 15 only when the final text is conclusively URL-free.
6. Settle using X's actual billed classification when available. Release the difference to the originating bucket. If actual classification is not available synchronously, settle the conservative amount and let reconciliation release a verified over-reservation; never add a surprise debit that makes available balance negative.

The classifier is versioned and covered by fixtures for plain URLs, Unicode/punctuation boundaries, `t.co`, common shorteners, X quote links, media-only posts, and rewritten tracking URLs.

## 8. Charging and ledger design

### 8.1 Data model

Use an immutable ledger plus materialized balances. Suggested logical entities:

#### `x_credit_accounts`

- `workspace_id`
- `included_available`
- `topup_available`
- `reserved_total`
- `recovery_debt`: non-spendable Credits owed after a refund or chargeback exceeds the remaining purchased balance
- `billing_period_start`
- `billing_period_end`
- `plan_id`
- `updated_at`

`x_credit_accounts` is a materialized cache for fast reads. It is not the accounting authority.

#### `x_credit_buckets`

- `id`
- `workspace_id`
- `bucket_type`: `monthly_grant` or `topup`
- `granted_credits`
- `available_credits`
- `reserved_credits`
- `status`: `active`, `expired`, or `closed`
- `starts_at`
- `expires_at`: required for `monthly_grant`; always `NULL` for `topup`, enforced by a database constraint
- `source_id`: subscription period, Stripe Checkout Session, manual adjustment, or refund
- `created_at`

The immutable ledger plus bucket rows are authoritative. `x_credit_accounts` is rebuilt from them and checked by an automated consistency job. A mismatch blocks new managed-X reservations for that workspace, emits an alert, and is repaired from the ledger rather than trusting the cache.

#### `x_credit_ledger`

- `id`
- `workspace_id`
- `social_account_id`
- `connection_mode`: `managed` or `platform_credentials`
- `bucket_id`
- `entry_type`: `grant`, `reserve`, `settle`, `release`, `topup`, `refund_reversal`, `recovery_debt_open`, `recovery_debt_offset`, `expire`, or `adjustment`
- `operation_key`
- `catalog_version`
- `credits`
- `idempotency_key`
- `upstream_resource_type`
- `upstream_resource_id`
- `request_id`
- `metadata`
- `created_at`

#### `x_credit_daily_resources`

- Unique key on `(workspace_id, social_account_id, upstream_resource_type, upstream_resource_id, utc_date)`
- Records whether a read or inbound event was already charged under UniPost's daily deduplication contract.

#### `x_credit_inbound_daily_usage`

- Unique key on `(workspace_id, utc_date)`
- `credits_consumed`
- `credits_limit`
- `events_accepted`
- `events_suppressed`
- Used for the hard inbound-spend cap in Section 8.3.

#### `x_credit_topups`

- `id`
- `workspace_id`
- `sku`
- `base_credits`
- `bonus_percent`
- `bonus_credits`
- `total_credits`
- `amount_cents`
- `purchase_mode`: `manual` or `auto`
- `stripe_checkout_session_id`
- `stripe_payment_intent_id`
- `idempotency_key`
- `status`
- `credited_at`
- `created_at`

### 8.2 Write charging

1. Resolve connection mode. BYO returns a zero-credit estimate and bypasses the credit ledger.
2. Determine the operation key and maximum credit cost before the upstream request. URL candidates follow Section 7.2.
3. For immediate writes, atomically reserve credits before calling X.
4. Call X with a stable request idempotency key when supported.
5. On success, settle the reservation exactly once.
6. On a confirmed failure where X did not accept the resource, release the reservation.
7. On an unknown timeout, mark the reservation `pending_unknown` and start the reservation TTL workflow below.
8. Duplicate API retries with the same UniPost idempotency key return the original result and do not create a second charge.

For scheduled posts, UniPost returns an estimate when the schedule is created but reserves credits only when the worker is ready to publish. If the workspace lacks credits at execution time, the X target fails with normalized code `x_credits_exhausted`, other platform targets continue, and UniPost sends a low-balance/failure notification with a top-up link.

Reservation TTL and reconciliation SLA:

- Normal in-flight reservation TTL: 15 minutes.
- Reconcile unknown outcomes after approximately 1, 5, and 15 minutes using request idempotency, stored response identifiers, and any available X lookup.
- At 15 minutes, a confirmed success settles and a confirmed failure releases.
- If still inconclusive, release the customer-facing reservation so it cannot lock the balance indefinitely, record an internal `unbilled_pending` exposure, and complete finance reconciliation within 24 hours.
- Do not retroactively make the customer balance negative. Repeated inconclusive outcomes on the same account trip an operational circuit breaker and pause new managed-X writes for review.
- Expired or released reservations are excluded before evaluating Auto top-up.

### 8.3 Read and webhook charging

- A public post, user, or DM event is charged only when it is a newly billable resource under the UniPost daily deduplication key.
- Receiving a webhook and later encountering the same resource during backfill must not charge twice.
- Re-reading an item from UniPost's database never consumes X Credits.
- Pagination must expose an estimate before an optional manual backfill that could consume a large number of credits.
- Automated background sync stops before it would make the balance negative.
- Customer-facing ledger entries identify whether consumption came from a webhook, polling, backfill, publish, or reply.

Inbound cost protection is mandatory before enabling X Activity subscriptions:

- Each workspace has a hard UTC-day inbound-spend cap measured in Credits across `post.mention.create`, `dm.received`, later `chat.received`, and paid recovery reads caused by those events.
- Default cap for an Inbox-eligible self-serve plan is `max(100, floor(plan_monthly_included_credits * 10%))`: Basic 400, Growth 1,200, Team 3,000. Enterprise is contract-defined. Free and API have an effective cap of 0 because Inbox is unavailable.
- The cap is independent of total balance. A large Top-up cannot be silently drained by a viral post unless the owner explicitly raises the inbound cap.
- Billing lets an Owner/Admin lower the cap or raise it up to the current available balance after acknowledging the estimated exposure.
- At 80% of the inbound cap, notify the workspace. Define a subscription-removal safety buffer of `max(20 Credits, 10% of the daily cap)`; when remaining cap reaches that buffer, stop billable polling/backfill and begin removing or pausing paid Activity subscriptions before the nominal cap is exhausted. At 100%, suppress all optional downstream paid work and send the cap-reached notification.
- Events suppressed after the cap are counted without storing private bodies where feasible. Restoring delivery requires the next UTC day or an explicit cap increase; any paid backfill requires a displayed estimate and confirmation.
- Keep an operational buffer and alert because subscription removal is not instantaneous and X may continue billing events during propagation.

### 8.4 Balance invariants

- Available balance can never be negative.
- A successful managed-app write has exactly one settlement.
- A failed pre-upstream validation has no reservation and no charge.
- A released reservation returns Credits to the same originating bucket only while that bucket remains active. If an included bucket's billing period has ended, release converts directly to an `expire` entry and does not become spendable.
- At a billing-period boundary, expire only unreserved included Credits. Reserved Credits move to an `expired_pending_settlement` state until they settle or release under the TTL. A successful settlement consumes them; a release expires them immediately.
- A refund or chargeback first reverses unspent Credits from the purchased bucket. If the purchased Credits were already consumed, keep spendable balance at zero and record the shortfall in `recovery_debt` rather than violating the non-negative invariant.
- While `recovery_debt > 0`, managed-X operations are blocked. Future purchased or included grants offset the debt before becoming spendable, unless an audited admin adjustment waives it. The UI and API show a payment/review state without describing the debt as credit expiration.
- Top-up webhook handling is idempotent on Stripe Checkout Session or Payment Intent.
- Granting or consuming credits uses a database transaction and row-level locking or an equivalent atomic mechanism.
- Admin adjustments require an actor, reason, and audit log entry.

### 8.5 Option B bounded-usage implementation

Option B does not implement buckets, purchased balances, refunds, reservations, or a customer transaction ledger. It implements the smallest billing-grade control needed to launch bounded X Inbox safely:

- `x_usage_periods`: one row per workspace and subscription billing period with `weighted_units_used`, `weighted_units_limit`, period boundaries, and timestamps.
- Operation weights reuse the managed-app Credit catalog numbers so collected data remains comparable with the later ledger.
- An atomic conditional increment succeeds only when `used + requested <= limit`; concurrency across workers/replicas cannot overspend the plan allowance.
- Inbound events also pass the Section 8.3 daily inbound-spend counter before the monthly increment.
- BYO X Platform Credentials bypass the managed-X usage counter because the customer pays X directly.
- At the period boundary, create a new usage row; do not mutate historical periods.
- Expose usage/limit/reset information in Billing and API, but do not call the value a purchased balance and do not offer Top-up Checkout.
- Return `x_monthly_usage_limit_exceeded` with upgrade/contact guidance when the hard monthly limit is reached.

This counter is intentionally not presented as the final ledger. Migration to the full system seeds the first included bucket from remaining period allowance and preserves historical usage rows for analytics.

## 9. Customer experience

### 9.1 Billing page

Add an **X Credits** section to workspace Billing showing:

- Total available credits.
- Included balance and top-up balance separately.
- Current plan's monthly allowance.
- Billing-period reset date.
- Existing X publish safety usage: successful publishes today out of 20 for each connected X account, resetting at 00:00 UTC.
- Workspace inbound-spend usage and daily cap, including controls to lower or explicitly raise the cap.
- Estimated examples such as normal posts, URL posts, DM sends, and post reads.
- Last 30 days of consumption by operation.
- Top-up cards for all five SKUs, showing base Credits, bonus percentage, bonus Credits, final Credits, and `Never expires`.
- Link to full transaction history.
- Auto top-up settings when that fast-follow is enabled.
- Link to the X Credits Guidance page.

Do not display a dollar-equivalent cash balance because credits are a product usage unit, not stored money.

Non-expiration must be visible without opening terms or a tooltip:

- Above the pack selector: **Buy more, get up to 20% bonus credits. Purchased credits never expire.**
- On every pack card: final Credits, bonus percentage, and **Never expires**.
- In the order summary: for example, **11,000 X Credits · Includes 10% bonus · Never expires**.
- On the payment-success page and email receipt: final base/bonus breakdown and **Purchased credits never expire**.
- On the balance card: **Included credits — resets on {date}** and **Top-up credits — never expire** as separate labels.
- When the workspace is Free: **Your purchased Credits are still yours and never expire, but managed X operations require a paid plan. Upgrade to use this balance.**

The non-expiration promise requires legal review for supported jurisdictions before launch because purchased, non-expiring usage units can create stored-value or gift-card questions even when they are non-transferable, non-refundable by default, and have no cash value.

### 9.2 Low-balance states

- All low-balance evaluation uses `available_to_spend = included + topup - active reservations`; included-bucket consumption alone never triggers a warning while the workspace still has a large Top-up balance.
- Dashboard notice when `available_to_spend` falls to or below 20% of the current plan's monthly included grant.
- Email and dashboard warning when it falls to or below 5% of the current plan's monthly included grant.
- At `available_to_spend = 0`: blocking warning, Top-up CTA, and API error documentation link.
- Notifications are deduplicated per workspace, billing period, and threshold crossing. A balance increase above a threshold rearms that threshold.
- Low-balance notices state which scheduled X posts are at risk before their execution time.
- Inbound-spend-cap warnings are separate from balance warnings so a workspace can understand “Credits available, but today's inbound protection limit is reached.”

### 9.3 Top-up Checkout

- Use one-time Stripe Checkout in the same live/test mode selection already used by UniPost subscription billing.
- Use the five fixed server-side SKUs in Section 6.4; do not accept arbitrary amounts in the first release.
- Attach `workspace_id`, `sku`, `base_credits`, `bonus_credits`, `total_credits`, and an internal top-up id to Stripe metadata generated by the server.
- Grant credits only after a verified successful Stripe webhook.
- The success page polls the UniPost balance until the idempotent webhook grant is visible.
- Payment failure or Checkout cancellation grants no credits.
- Refunds create reversing ledger entries. They must never silently delete ledger history.
- Manual and Auto top-ups use the same SKU, bonus, and non-expiration rules. Margin remains an internal finance metric.
- A paid workspace can use the resulting balance from Dashboard or API operations allowed by its plan. Top-up purchase never bypasses the Free X gate or the Basic Inbox gate.

### 9.4 Auto top-up — explicitly post-MVP

Auto top-up is not part of MVP. It may ship only after manual Top-up, ledger reconciliation, refund recovery, and duplicate-charge monitoring are stable in production. The data model and API must not prevent the later phase. When enabled, the workspace owner chooses:

- A custom trigger threshold in whole Credits. UniPost may recommend a value based on the plan, but the recommendation must remain editable.
- One of the approved SKUs.
- Monthly top-up spend cap.
- Payment method through Stripe.

Auto top-up is off by default and requires explicit owner/admin consent. Stripe must collect and authorize a reusable payment method through a SetupIntent or equivalent compliant flow before UniPost can make off-session charges. A payment requiring 3DS or other customer action pauses Auto top-up, sends an actionable notification, and does not grant Credits until Stripe confirms success. A downgrade to Free or subscription cancellation disables future automatic charges without deleting existing Top-up Credits.

Define `available_to_spend` as included credits plus top-up credits minus active reservations. Auto top-up evaluates against this spendable balance, not against either bucket independently.

Trigger rules:

1. Trigger when `available_to_spend` crosses from above the user-defined threshold to at or below it.
2. Before a managed X reservation, trigger early if the pending operation would take `available_to_spend` to or below the threshold.
3. Apply a new billing-period included grant before evaluating Auto top-up at a subscription reset, so the reset cannot cause an unnecessary charge.
4. If Auto top-up is enabled while the current balance is already at or below the chosen threshold, the UI must warn that a purchase will start immediately; the API response must return `will_trigger_immediately: true`.
5. Only one Auto top-up may be in flight for a workspace.
6. A successful purchase cannot immediately trigger another purchase in a chain. The trigger rearms only after the balance is observed above the threshold and later crosses it again.
7. If the selected SKU would not restore the current balance above the configured threshold, warn the user before saving the configuration.
8. A failed Auto top-up disables further automatic attempts until the owner resolves payment and explicitly re-enables it. It never grants credits.
9. Reaching the user-defined monthly spend cap pauses Auto top-up until the next monthly cap period and sends a notification.
10. Auto top-up uses the same volume bonus as a manual purchase of the selected SKU; it never receives a hidden lower or higher conversion.

The Billing UI uses an editable numeric field rather than a fixed plan threshold:

```text
Automatically top up when available X Credits fall to: [ 800 ]
Top-up package:                                      [ $25 ]
Maximum automatic spend per month:                  [ $100 ]
```

The confirmation copy must summarize the exact user-defined rule and the non-expiration promise:

> When your available balance falls to 800 X Credits, UniPost will automatically purchase the selected package, up to $100 per month. Purchased credits never expire.

## 10. Public API contract

### 10.1 X Credits endpoints

#### `GET /v1/billing/x-credits`

Returns the current workspace balance and catalog summary.

Minimum response fields:

- `data.total_available`
- `data.included_available`
- `data.topup_available`
- `data.topup_expires_at`: always `null`
- `data.reserved`
- `data.available_to_spend`
- `data.recovery_debt`
- `data.plan_monthly_grant`
- `data.billing_period_start`
- `data.billing_period_end`
- `data.catalog_version`
- `data.inbound_daily_usage`
- `data.inbound_daily_limit`
- `data.connection_mode_note`
- `request_id`

#### `GET /v1/billing/x-credits/transactions`

Returns paginated ledger entries. Filters:

- `entry_type`
- `operation_key`
- `social_account_id`
- `from`
- `to`
- `cursor`
- `limit`

No sensitive X DM body or post body is returned in billing transactions.

#### `POST /v1/billing/x-credits/top-up/checkout`

Owner/admin or an explicitly `billing:write`-scoped API key only. Accepts one enum SKU and returns a Stripe Checkout URL. It supports API-initiated purchase but still requires the customer to complete Stripe Checkout; the first release does not allow an ordinary publishing API key to charge a saved card directly.

```json
{
  "sku": "topup_25"
}
```

The endpoint requires `Idempotency-Key`. The API derives price, base Credits, bonus, and total Credits from the server-side SKU catalog. Internal margin is not returned. It must never trust client-supplied amount or credit quantity.

```json
{
  "data": {
    "topup_id": "xtu_123",
    "sku": "topup_25",
    "amount_cents": 2500,
    "base_credits": 10000,
    "bonus_percent": 10,
    "bonus_credits": 1000,
    "total_credits": 11000,
    "expires_at": null,
    "checkout_url": "https://checkout.stripe.com/..."
  },
  "request_id": "req_123"
}
```

Credits become spendable only after a verified Stripe success webhook grants the idempotent Top-up ledger entry. A Free workspace cannot create a Checkout Session for managed X Credits; return the existing plan-gate error with an upgrade link.

#### `POST /v1/billing/x-credits/auto-top-up/payment-method/setup`

Fast-follow endpoint. Owner/admin only. Creates the Stripe SetupIntent or equivalent session required to collect explicit consent and a reusable payment method for off-session Auto top-up. No Credits are granted by this endpoint.

#### `PATCH /v1/billing/x-credits/auto-top-up`

Fast-follow endpoint. Owner/admin or an explicitly `billing:write`-scoped API key only. Enables, updates, or disables the user-defined threshold, SKU, and monthly spend cap. The threshold is not inferred or locked by the workspace plan.

```json
{
  "enabled": true,
  "threshold_credits": 800,
  "sku": "topup_25",
  "monthly_spend_cap_cents": 10000
}
```

The endpoint requires `Idempotency-Key`. The response returns the saved configuration, current `available_to_spend`, `will_trigger_immediately`, the selected package's total credits, and `expires_at: null` for the credits that a successful Auto top-up will grant. Amount, bonus, and final Credits are derived from the server-side SKU catalog and cannot be supplied by the client.

### 10.2 Estimates and actual charges

- `POST /v1/posts/validate` returns `estimated_x_credits` per managed X target.
- `POST /v1/posts` returns the estimate at acceptance, not a false claim of final charge.
- The final platform result stores `x_credits_charged`, `x_credit_operation`, and `x_credit_catalog_version`.
- `POST /v1/inbox/{id}/reply` returns the settled charge for synchronous managed X replies/DMs.
- BYO results return `x_credits_charged: 0` and `x_credit_billing_mode: customer_x_app`.

### 10.3 Errors

| Scenario | HTTP | Normalized code | Behavior |
|---|---:|---|---|
| Total X balance is insufficient before an immediate write | 402 | `x_credits_exhausted` | No upstream request is made. |
| Existing 20/day X publish safety cap reached | 429 | `per_platform_daily_cap_exceeded` | No X request and no Credit reservation; reset at 00:00 UTC. |
| Option B monthly weighted X allowance reached | 402 | `x_monthly_usage_limit_exceeded` | No paid managed-X work; return billing-period reset and upgrade/contact guidance. |
| Workspace X inbound-spend cap reached | 429 | `x_inbound_daily_cap_exceeded` | Pause paid inbound work and provide reset/cap-management guidance. |
| Refund/chargeback recovery debt exists | 402 | `x_credit_recovery_required` | Block managed-X spend until the debt is offset or waived. |
| Top-up SKU is invalid | 422 | `validation_error` | No Checkout Session is created. |
| Stripe top-up is unavailable | 503 | `billing_unavailable` | Existing balance remains unchanged. |
| Auto top-up requires customer action | 409 | `auto_topup_action_required` | Pause automatic charges and return the Stripe recovery URL. |
| Auto top-up monthly cap reached | 402 | `auto_topup_monthly_cap_reached` | Do not charge; notify the owner and use the remaining balance until exhausted. |
| Auto top-up payment failed | 402 | `auto_topup_payment_failed` | Grant no Credits, disable automatic retries, and return manual recovery guidance. |
| X account lacks new scopes | 409 | `x_reconnect_required` | Return reconnect URL and missing scopes. |
| X Inbox unavailable on plan | 402 | `plan_feature_not_available` | Existing Basic gate semantics. |
| XChat is not enabled for the account | 422 | `x_chat_not_supported` | Legacy DM support remains unaffected. |

All new errors follow the existing UniPost error envelope and include `request_id`.

### 10.4 Gate ordering and coexistence with existing limits

X Credits do not replace the existing per-account 20-successful-publishes-per-UTC-day safety cap.

For an outbound managed-X target, apply gates in this order:

1. Plan/platform eligibility.
2. Payload and scope validation, including URL classification.
3. Existing per-account daily publish safety cap.
4. Recovery-debt check.
5. Credit reservation.
6. Upstream X request.

A target rejected by steps 1-4 consumes no Credits. The final result and Billing UI can therefore show `Credits available` and `20/20 X publishes used today` at the same time. The Dashboard must display both constraints rather than implying that purchasing Credits overrides platform-safety limits.

For inbound activity, apply account/plan eligibility, event deduplication, workspace inbound-spend cap, and available balance before any optional paid enrichment or backfill. Webhook receipt itself must respond within X's required deadline even when downstream ingestion is suppressed.

## 11. X comments/replies

### 11.1 MVP scope

- Ingest replies and mentions addressed to a connected X account.
- Group public replies by X `conversation_id`.
- Normalize inbound items as source `x_reply`.
- Display the parent post, author, text, timestamps, reply tree, read state, assignment, and resolution state using the existing Inbox model.
- Reply through `POST /v1/inbox/{id}/reply` using X's post-create reply operation.
- Backfill replies to UniPost-published X posts within the X API's supported recent-search window.
- Preserve public post URLs and external ids for opening the conversation on X.

### 11.2 Ingestion strategy

1. Prefer X Activity `post.mention.create` events for low-latency inbound delivery.
2. Use filtered stream or recent-search/conversation lookup only where needed for recovery and controlled backfill.
3. Deduplicate by upstream post id before creating an Inbox row or charging credits.
4. Avoid repeated author enrichment reads when the author already exists in UniPost's cache.
5. Store the upstream `conversation_id`, replied-to post id, and root post id.

### 11.3 Outbound replies

- Use the connected account's OAuth token.
- Reserve credits before the write for managed-app connections.
- A URL in the reply can select the higher URL-post catalog operation.
- Persist outbound replies as `is_own=true` Inbox items after X confirms success.
- Use the target Inbox item id plus client idempotency key to prevent duplicate replies.

## 12. Legacy X DMs

### 12.1 MVP scope

- Normalize legacy DMs as source `x_dm`.
- Receive new DMs through X Activity when available.
- Perform a controlled initial/backfill lookup within X's available DM history window, currently documented as 30 days.
- Show one Inbox thread per X DM conversation.
- Reply from the Dashboard and `POST /v1/inbox/{id}/reply`.
- Store outbound messages locally so UniPost does not need an avoidable paid read to rediscover them.

### 12.2 Privacy and security

- Encrypt X OAuth tokens at rest using the existing UniPost token encryption path.
- Do not log DM bodies, access tokens, webhook payload bodies, or encryption secrets.
- Verify webhook signatures and reject stale or invalid callbacks.
- Apply workspace authorization to every Inbox read and reply.
- Honor account disconnect and workspace deletion by removing subscriptions and applying the existing Inbox data-deletion policy.
- Make DM backfill opt-in when the estimated credit cost exceeds a safe threshold.
- Document that X's API history window is not the same as UniPost's data-retention period.

## 13. XChat later-phase gate

XChat must not be marketed as generally available with the legacy DM MVP. Before implementation is promoted beyond an internal beta, engineering must prove:

1. The official production SDK or protocol is stable enough for supported server-side use.
2. The user-key, PIN, and encrypted key material lifecycle can be implemented without UniPost exposing plaintext secrets or making unrecoverable account states common.
3. Inbound Activity events and send APIs work reliably for a representative set of accounts.
4. XChat and legacy DM conversations can be normalized without silently merging unrelated threads.
5. Credit accounting for `chat.received`, chat reads, and sends reconciles with X billing.
6. Disconnect, key rotation, recovery, and deletion flows are documented and tested.
7. X's current terms permit UniPost's intended multi-tenant Inbox use case.

Passing the gate adds source `x_chat`; failing it leaves the shipped product at public replies plus legacy `x_dm`.

## 14. X developer account, permissions, and application work

### 14.1 Managed UniPost X app setup

Before development-environment testing:

1. Confirm the UniPost developer account is approved and the production app belongs to a Project.
2. Enable the required pay-per-use access and fund the X API account.
3. Set an X-side spending limit and operational alerts.
4. Configure the OAuth 2.0 client as a confidential Web App / Automated App.
5. Register every callback URL exactly for development, staging, and production.
6. Verify website, privacy-policy, terms-of-service, and support URLs.
7. Keep client secret and app-only bearer token in the correct environment's secret store.
8. Register the HTTPS webhook endpoint, complete CRC validation, and verify `x-twitter-webhooks-signature` on callbacks.
9. Subscribe only to the minimum events needed for the active phase.
10. Record the developer-account owner, billing owner, secret-rotation owner, X plan, subscription capacity, and escalation path in the operations runbook.

Register both UniPost callback families in every environment:

| Environment | OAuth callback | Hosted Connect callback |
|---|---|---|
| Development | `https://dev-api.unipost.dev/v1/oauth/callback/twitter` | `https://dev-api.unipost.dev/v1/connect/callback/twitter` |
| Staging | `https://staging-api.unipost.dev/v1/oauth/callback/twitter` | `https://staging-api.unipost.dev/v1/connect/callback/twitter` |
| Production | `https://api.unipost.dev/v1/oauth/callback/twitter` | `https://api.unipost.dev/v1/connect/callback/twitter` |

Where the Developer Console exposes a separate app-permission selector in addition to OAuth 2.0 scopes, configure the app for read, write, and Direct Message access. The granted OAuth scope set on the returned token remains the runtime source of truth.

### 14.2 OAuth scopes

The managed app currently needs the existing publishing scopes and must add DM permissions for the MVP.

| Scope | MVP use | Status |
|---|---|---|
| `tweet.read` | Read posts/replies and DM-linked post context required by X | Existing / required |
| `tweet.write` | Publish posts and public replies | Existing / required |
| `users.read` | Resolve connected account and message/reply authors | Existing / required |
| `offline.access` | Refresh access without repeated interactive login | Existing / required |
| `media.write` | Upload media for X posts | Existing / required |
| `dm.read` | Read legacy DM events and conversations | **New / required** |
| `dm.write` | Send legacy DMs | **New / required** |
| `tweet.moderate.write` | Hide/unhide replies | Not requested for MVP |

Use least privilege. Do not request `tweet.moderate.write` until the product actually supports moderation.

XChat-specific scopes and key permissions must be verified against the current official XChat documentation during its proof of concept. They are not assumed to be covered by `dm.read` and `dm.write`.

### 14.3 Existing-account reconnect

Adding scopes does not retroactively update previously granted OAuth tokens.

- Mark managed X connections missing `dm.read` or `dm.write` as `reconnect_required` for DM capability only.
- Keep existing publishing working if its current scopes remain valid.
- Show the exact missing scopes and a reconnect CTA in Accounts, Inbox, and Billing/X Credits context.
- Do not repeatedly prompt API-plan customers for DM scopes because their plan cannot use Inbox.
- Basic, Growth, Team, and eligible Enterprise accounts receive the reconnect prompt.
- After reconnect, verify granted scopes before enabling DM sync.
- BYO customers must update their own X app permissions before reconnecting.

### 14.4 Activity and webhook subscriptions

For MVP, request the minimum applicable private activity subscriptions:

- `post.mention.create`
- `dm.received`

Use the app-only bearer token for webhook registration and the connected user's OAuth authorization for private, account-scoped event delivery. Record the returned subscription ids so disconnect, zero-balance pause, and workspace deletion can remove the exact subscriptions idempotently.

Do not subscribe to `dm.sent` if UniPost already stores all of its own outbound messages and the event would add cost without filling a product gap. Add it only if reconciliation testing proves it is necessary.

For XChat beta, evaluate:

- `chat.received`
- `chat.sent` only if required for cross-client consistency

X currently documents self-serve Activity subscription capacity limits. The MVP requests two event subscriptions per eligible connected account (`post.mention.create` and `dm.received`), so a 1,500-subscription limit represents about 750 fully subscribed accounts. Alert before 70%, 85%, and 95% and pursue higher capacity before onboarding would exceed it. This is a capacity-monitoring requirement, not an initial launch blocker at current scale. If later XChat adds a third subscription, capacity falls to about 500 accounts and must be recalculated in the runbook.

### 14.5 Developer access request narrative

Use the following substance in any X access, capacity, or support application:

> UniPost is a multi-tenant social publishing and customer-engagement platform. A customer explicitly connects an X account through OAuth and authorizes UniPost to publish, receive replies or messages addressed to that account, and send replies on the account owner's behalf. UniPost uses X data only to provide the connected customer's publishing, Inbox, support, and analytics workflows. We do not sell X content, build surveillance profiles, or use private messages to train public models. Access is workspace-scoped, tokens are encrypted, webhook signatures are verified, sensitive message bodies are excluded from logs, disconnect removes activity subscriptions, and workspace deletion follows our documented data-deletion process. We apply least-privilege scopes, usage limits, abuse controls, and customer-visible cost accounting.

The actual application must include current traffic estimates, projected connected X accounts, event volume, retention policy, privacy URL, terms URL, security contact, and the requested Activity subscription capacity.

### 14.6 Verified repository prerequisites and implementation traps

The implementation plan must include these current-code prerequisites verified against `origin/main` on 2026-07-15:

1. **Two X OAuth implementations must stay aligned.** Hosted Connect uses `api/internal/connect/twitter.go` and already uses S256 PKCE. Platform Credentials/native OAuth uses `api/internal/platform/twitter.go`, where `DefaultOAuthConfig.Scopes` and `GetAuthURL` currently duplicate scope values. Add `dm.read` and `dm.write` to both connection modes and make `GetAuthURL` derive the scope parameter from `config.Scopes` instead of another hard-coded string.
2. **Upgrade Platform Credentials PKCE.** `api/internal/platform/twitter.go` currently uses `code_challenge_method=plain` and derives the verifier from state. Replace it with a cryptographically random stored verifier and RFC 7636 S256 challenge before granting DM permissions. Do not weaken the already-correct Hosted Connect S256 path.
3. **Expand Inbox account queries.** `api/internal/db/queries/inbox.sql` currently limits background/sync account selection to `instagram`, `threads`, and `facebook`. Add `twitter` to the relevant query paths and regenerate sqlc.
4. **Expand normalized source handling.** Add `x_reply` and `x_dm` across backend dispatch, response models, Dashboard types, grouping, icons, filters, unread counts, websocket validation, and reply routing. “Reuse the existing Inbox model” does not mean adapter-only work.
5. **Correct existing Inbox documentation drift.** Before adding X, `/docs/api/inbox` must describe the sources actually implemented in production: `ig_comment`, `ig_dm`, `threads_reply`, `fb_comment`, and `fb_dm`. Remove `youtube_comment` from the documented supported set unless backend ingestion is implemented and verified; add the missing Facebook sources.
6. **Create one plan/catalog source of truth.** Plan prices, quotas, feature gates, and future X allowance/catalog values are currently repeated across DB migrations and multiple Dashboard/docs files. The implementation includes a plan/catalog registry or generated artifact consumed by Pricing, Plans and limits, Billing, API Reference, operation-capacity tables, and source tests. Database plan rows remain runtime authority; generated public data must be checked against them.
7. **Fix Enterprise plan serialization.** Enterprise currently stores `price_cents=0`, and `GET /v1/plans` serializes that as `$0` semantics. Add an explicit custom/contact-sales representation such as `price_cents: null` or `pricing_model: "custom"` before using the endpoint as the shared Pricing source.
8. **Preserve existing daily safety behavior.** The 20/day X publish cap is a best-effort account-safety belt today, not a billing-grade counter. Ensure its rejection occurs before Credit reservation, and do not claim Credits allow customers to bypass it.

## 15. Documentation requirements

Documentation is part of the feature definition, not a launch follow-up.

### 15.1 New API Reference pages

Create these pages using the existing API Reference components and navigation:

| Route | Purpose | Must link to |
|---|---|---|
| `/docs/api/x-credits` | X Credits overview, public operation-to-Credits catalog, managed vs BYO behavior; no upstream dollar cost or margin | X Credits guide; Plans and limits |
| `/docs/api/x-credits/balance` | `GET /v1/billing/x-credits` request/response | X Credits guide |
| `/docs/api/x-credits/transactions` | `GET /v1/billing/x-credits/transactions` filters and ledger fields | X Credits guide |
| `/docs/api/x-credits/top-up` | Manual Top-up Checkout, five SKUs, bonus fields, idempotency, and non-expiration | X Credits guide |
| `/docs/api/x-credits/auto-top-up` | Post-MVP payment-method setup and user-configured Auto top-up API; publish only when the feature ships | X Credits guide |
| `/docs/api/inbox/list` | Dedicated `GET /v1/inbox` contract including `x_reply` and `x_dm` | X comments guide; X DMs guide |
| `/docs/api/inbox/reply` | Dedicated `POST /v1/inbox/{id}/reply` contract and X credit result fields | X comments guide; X DMs guide |
| `/docs/api/inbox/sync` | Dedicated `POST /v1/inbox/sync`, estimates, backfill, and credit behavior | X comments guide; X DMs guide |

Also update:

- `/docs/api/inbox` first to correct its production baseline to `ig_comment`, `ig_dm`, `threads_reply`, `fb_comment`, and `fb_dm`; remove the unsupported documented `youtube_comment`. Then add `x_reply`, `x_dm`, and later `x_chat` only when each phase ships.
- `/docs/api/posts/create` and `/docs/api/posts/validate` for estimates, actual-charge fields, BYO behavior, and `x_credits_exhausted`.
- `/docs/api/billing` to link to the X Credits API family.
- `/docs/api/errors` with every normalized error in Section 10.3.
- The API Reference sidebar, index, search index, sitemap, and structured metadata.

### 15.2 New Guidance pages

Create task-oriented Guidance pages:

| Route | User task | Must link back to |
|---|---|---|
| `/docs/guides/x/credits-and-top-ups` | Estimate Credits, inspect balance, buy discounted non-expiring Top-ups, and handle exhaustion; add Auto top-up instructions only in its post-MVP release | Balance, transactions, manual Top-up, later Auto top-up, create-post, and validate references |
| `/docs/guides/x/comments` | Connect X, receive comments/replies, list threads, and reply | Inbox list, reply, and sync references |
| `/docs/guides/x/direct-messages` | Request scopes, reconnect, sync DMs, and send a reply | Inbox list, reply, sync, and X Credits references |
| `/docs/guides/x/reconnect-permissions` | Add DM scopes to managed or BYO X apps and reconnect existing accounts | Account connect/capabilities, Inbox, and X Credits references |

The Guides sidebar gains an **X Guides** section. Each guide must be procedural, include complete code examples, explain managed versus BYO billing, and state plan eligibility.

### 15.3 Existing X pages to update

- `/docs/platforms/twitter`
  - Change Inbox support from `none` to the shipped capability.
  - Add X Credits examples and limits.
  - Link to all four X Guidance pages and the X Credits API Reference.
- `/docs/platform-credentials/twitter`
  - Add `dm.read` and `dm.write` setup.
  - Explain that BYO X app usage is billed directly by X and does not consume UniPost X Credits.
  - Link to reconnect guidance and the X Inbox guides.
- `/docs/pricing`
  - Add included credits by plan, reset behavior, the five-pack Top-up ladder, 0%-20% bonus, non-expiration, and separate post/X-credit enforcement.
  - Mirror the operation-capacity table and calculation definitions from Section 6.2.
  - Mirror the Top-up operation examples from Section 6.4 and explain that each example spends the whole pack on one operation type.
  - Do not disclose X's upstream dollar prices, the internal cost represented by one Credit, gross profit, gross margin, or maximum upstream cost represented by a plan.
- `/pricing`
  - Add one concise X Credits allowance line per plan and a Top-up FAQ covering the five packs, volume bonus, manual/API purchase, and non-expiration. Add Auto top-up copy only when that post-MVP feature ships.
  - Add **What your included X Credits can do** directly below the plan cards, showing normal posts, URL posts, complete comment interactions, and complete DM interactions for every plan.
  - Show `Inbox not included` for API comments and DMs rather than displaying a misleading theoretical capacity.
  - Explain that each maximum assumes the full shared balance is spent on one operation type and that mixed usage reduces every individual maximum.
  - Render a readable desktop table and equivalent mobile plan cards.
  - Generate displayed capacities from the versioned allowance/catalog data and cover the values with source tests.
  - Display **Buy more, get up to 20% bonus credits. Purchased credits never expire.** near the Top-up ladder.

### 15.4 Bidirectional linking acceptance rule

The documentation build is incomplete unless all of the following are true:

1. Every new API Reference page contains a visible **Guides** or **Next steps** link to at least one relevant X Guidance page.
2. Every new X Guidance page contains a visible **API Reference** section linking to each endpoint used by the guide.
3. `/docs/platforms/twitter` links to both the X Guidance group and X Credits Reference.
4. `/docs/platform-credentials/twitter` links to reconnect Guidance and the relevant account/Inbox Reference pages.
5. The X Credits Guidance page links to `/docs/pricing`; `/docs/pricing` links back to the X Credits Reference or Guidance page.
6. Automated source tests assert both directions for every required pair and fail on missing routes.
7. Search indexing and sitemap generation include all new pages.
8. No page describes XChat as shipped until its later-phase acceptance gate passes.
9. Public documentation and marketing source tests fail if upstream X dollar prices, internal Credit cost mapping, gross profit, gross margin, or maximum upstream plan cost appear outside explicitly internal documentation.

### 15.5 Required link matrix

| Reference page | Guidance page | Direction |
|---|---|---|
| X Credits overview/balance/transactions/manual Top-up/Auto top-up | X Credits and top-ups | Both directions |
| Inbox list/reply/sync | X comments | Both directions |
| Inbox list/reply/sync | X direct messages | Both directions |
| Account connect/capabilities | X reconnect permissions | Both directions |
| Create/validate posts | X Credits and top-ups | Both directions |

## 16. Operations and reconciliation

### 16.1 Monitoring

Track and alert on:

- X upstream cost by resource type, app, environment, and day.
- UniPost credits settled by operation and catalog version.
- Daily upstream-cost-to-ledger variance.
- Negative or inconsistent materialized balances; expected value is zero occurrences.
- Pending reservations approaching or exceeding the 15-minute customer-facing TTL.
- Reservations reaching the defined 15-minute TTL, `unbilled_pending` exposure, and the 24-hour finance-reconciliation deadline.
- Ledger/bucket authority versus `x_credit_accounts` cache mismatches and repair outcomes.
- Refund/chargeback recovery debt and time to resolution.
- Duplicate webhook event attempts and deduplication rate.
- Activity subscription capacity at 70%, 85%, and 95%.
- Low-balance blocks and successful top-ups after a block.
- Inbound daily-cap usage, 80%/100% notifications, suppressed events, subscription-removal latency, and paid backfill confirmations.
- Manual and automatic Top-up attempts, failures, customer-action requirements, monthly-cap pauses, and duplicate-prevention outcomes.
- DM/reply webhook latency, backfill lag, and reply success rate.
- Gross margin for included allowance and Top-ups separately, broken down by SKU and manual versus automatic purchase mode.

### 16.2 Reconciliation

- Run a daily reconciliation comparing X usage exports/console data with UniPost ledger aggregation.
- Flag material variance by operation, not only total dollars.
- Do not retroactively charge customers for an internal UniPost bug without an explicit reviewed policy.
- Undercharging caused by a catalog or integration bug is absorbed until a corrected catalog is published.
- Overcharging is corrected with an auditable credit adjustment.
- Pending timeout reservations follow the 1/5/15-minute workflow. Any `unbilled_pending` item unresolved at 24 hours enters an admin review queue and pages the billing owner.

### 16.3 Cost controls

- Set X-side spending limits per environment.
- Development and staging must use non-production apps or separately bounded credentials where possible.
- Preserve the existing 20-successful-X-publishes-per-account-per-UTC-day safety cap independently of Credits.
- Enforce the workspace inbound daily-spend cap before optional enrichment or backfill can consume Credits.
- Stop optional polling before essential outbound writes when a workspace balance is low.
- Unsubscribe or pause paid private events when the inbound cap or workspace balance is exhausted, if X supports doing so without losing required account state.
- Restore subscriptions and offer bounded backfill after balance is replenished.
- Cache public data and authors within allowed policy windows.

## 17. Rollout plan

No feature flag is required by default. Use normal environment promotion, plan gates, capability checks, and explicit beta eligibility. Phase order depends on the Section 2.3 decision.

### Common Phase 0 — prerequisites

- Confirm current X pricing internally without publishing the upstream dollar amounts or UniPost margin.
- Add X API funds and spending limits.
- Complete the OAuth, PKCE, Inbox-source, documentation-baseline, plan/catalog-source, and Enterprise serialization prerequisites in Section 14.6.
- Configure scopes, callbacks, webhook, CRC, and signatures.
- Validate the 20/day outbound safety-cap interaction and the new inbound daily-spend cap.
- Submit capacity/access request only when projections approach the monitored threshold.
- Validate developer, staging, and production app separation.

### Option A sequence — full Credits first

1. Ledger, buckets, grants, reservation TTL, refund recovery, settlement, and reconciliation.
2. Balance/transaction APIs, Billing UI, and manual Stripe Top-ups.
3. Managed-vs-BYO branching, estimates, low-balance states, and public documentation without internal unit economics.
4. X public comments/replies.
5. Legacy X DMs and reconnect flow.

### Option B sequence — bounded Inbox first (recommended for approval)

1. Add an atomic per-workspace monthly weighted X-usage counter with the approved operation weights, but no purchased buckets, Stripe Top-ups, or customer ledger.
2. Add the hard per-workspace daily inbound-spend cap, notifications, suppression, and bounded backfill.
3. Ship X public comments/replies and legacy X DMs with plan allowance enforcement.
4. Measure cost, inbound volume, cap hits, and demand against the Section 2.3 promotion triggers.
5. Build the full Credits ledger and manual Top-ups only after a trigger or explicit product decision.

### Post-MVP — Auto top-up

- Explicit opt-in and Stripe reusable-payment-method setup.
- User-defined threshold, selected SKU, and required monthly spend cap.
- Off-session payment, 3DS/customer-action recovery, notifications, and anti-loop behavior.
- Auto top-up API Reference and updated X Credits Guidance.
- Auto top-up is not part of either MVP sequence.

### Later — XChat proof of concept

- Internal-only technical validation.
- Security review and cost reconciliation.
- Beta decision based on the Section 13 gate.

## 18. Effort estimate

The estimate assumes one backend engineer and one frontend/full-stack engineer working in parallel, with product/design review and QA support.

| Workstream | Estimated engineering effort |
|---|---:|
| Credits ledger, grants, reservation/settlement, reconciliation | 3-4 engineer-weeks |
| Stripe top-ups, billing APIs, Billing UI, notifications | 2-3 engineer-weeks |
| Auto top-up, saved payment method, off-session recovery, and configurable threshold | 1-2 engineer-weeks as fast-follow |
| Atomic bounded-usage counter and inbound daily-spend protection | 1.5-2.5 engineer-weeks |
| X developer setup, webhooks, Activity subscriptions, reconnect scopes | 1-2 engineer-weeks plus external X approval time |
| X comments/replies ingestion, normalization, UI, outbound replies | 2-3 engineer-weeks |
| Legacy X DM sync, UI integration, outbound replies, privacy hardening | 2-3 engineer-weeks |
| API Reference, Guidance, links, tests, and launch copy | 1-1.5 engineer-weeks |
| Plan/catalog source-of-truth refactor and Enterprise serialization fix | 1.25-2.5 engineer-weeks |
| End-to-end QA, billing reconciliation, and rollout hardening | 1.5-2 engineer-weeks |

**Option A full Credits-first MVP:** approximately **13.75-21 engineer-weeks**, or roughly **8-11 calendar weeks** with two engineers and no external X approval delay.

**Option B bounded-Inbox-first MVP:** approximately **7.75-13.25 engineer-weeks**, or roughly **4-7 calendar weeks** with two engineers. The later ledger/manual-Top-up phase adds approximately **6-9 engineer-weeks**, informed by real usage data.

**Auto top-up post-MVP:** add approximately **1-2 engineer-weeks** after manual Top-up stability criteria are met.

**XChat later phase:** add approximately **4-8+ engineer-weeks** after official tooling and permission feasibility are proven. This range is intentionally separate because encrypted key management and evolving XChat APIs are the largest uncertainty.

## 19. Success metrics

- Managed X gross margin stays within the approved floor by plan and in aggregate.
- Top-up theoretical gross margin matches the approved SKU ladder and never falls below 52% before payment and operating costs under the launch catalog.
- Option B, if selected, reports monthly weighted-usage utilization, inbound-cap hits, suppressed events, and customer requests needed for the Section 2.3 promotion decision.
- Paid inbound work beyond a workspace's hard daily cap: zero confirmed incidents, excluding documented X subscription-removal propagation exposure.
- Duplicate customer charges: zero confirmed incidents.
- Successful top-up grant after verified Stripe payment: at least 99.9%.
- X reply and legacy DM outbound success rate: at least 98%, excluding platform policy/rate-limit failures.
- P95 inbound webhook-to-Inbox latency: under 60 seconds.
- Daily X invoice-to-ledger variance: below the finance-defined materiality threshold, with all larger variances explained.
- At least 90% of `x_credits_exhausted` responses include an actionable balance/top-up link.
- Documentation bidirectional-link tests pass in CI.
- Customer-facing source tests report zero disclosures of upstream X dollar prices or UniPost margin.

## 20. Acceptance criteria

### 20.1 Option B bounded X Inbox MVP

If Option B is approved, its MVP is complete only when:

1. X public replies and legacy DMs are available on Basic, Growth, Team, and eligible Enterprise workspaces through Dashboard and Inbox API.
2. The monthly weighted X-usage counter and daily inbound-spend cap are atomic across workers/replicas and hard-block before additional paid work.
3. A viral inbound event burst cannot consume beyond the configured daily inbound cap without explicit Owner/Admin action.
4. The existing 20/day per-account X publish safety cap remains independent, visible, and checked before any X usage reservation/count increment.
5. OAuth scope updates, Platform Credentials S256 PKCE, reconnect behavior, Inbox query/source expansion, and real dev-environment verification pass.
6. `/docs/api/inbox` correctly documents the production baseline sources and the shipped X sources, with no unsupported `youtube_comment` claim.
7. Pricing and docs describe included operation capacity and hard caps, but do not advertise purchased Top-ups before they ship.
8. The Section 15 Reference/Guidance links for the shipped Inbox surface pass in both directions.
9. Cost, cap-hit, suppression, backfill, and customer-demand metrics required by the Section 2.3 promotion gate are live.

### 20.2 Full Credits and manual Top-ups

If Option A is approved, or when Option B is promoted to the full system, the release is complete only when:

1. Paid plans receive exactly the approved included credit amounts at the correct Stripe billing-period boundary.
2. Manual Top-ups grant exactly 2,000, 4,200, 11,000, 23,000, or 48,000 Credits for the five approved SKUs after a verified successful payment and never before it. The ledger and API preserve base, bonus, and total Credits separately.
3. Included credits are consumed before Top-ups. Included credits expire at the billing-period boundary, while purchased Top-up buckets always have `expires_at = null` and never expire because of time, plan changes, or subscription cancellation.
4. Managed X operations charge the versioned catalog; BYO X operations charge zero UniPost X Credits.
5. Immediate writes cannot start without enough available credits.
6. Retries, duplicate webhooks, and worker redelivery do not create duplicate charges.
7. Failed pre-upstream requests and confirmed upstream failures release reservations.
8. Customers can view balance and paginated transaction history through Dashboard and API.
9. Basic, Growth, Team, and eligible Enterprise workspaces can receive and reply to `x_reply` and `x_dm` items after granting the required permissions.
10. Existing X publishing remains functional when an account has not yet reconnected for DM scopes.
11. X developer callbacks, scopes, webhook CRC/signature verification, subscription limits, spending alerts, and ownership runbook are verified per environment.
12. The Pricing page and Plans and limits docs accurately show allowances, the five-pack graduated Top-up ladder, 0%-20% bonus, non-expiration, separate quota behavior, and the complete Section 6.2 and 6.4 operation-capacity tables. Desktop tables and mobile cards are generated from versioned allowance/catalog data, use floor rounding, label API Inbox operations unavailable, and pass source tests for every displayed number.
13. All new API Reference and Guidance pages in Section 15 exist, are in navigation/search/sitemap, and pass the bidirectional linking tests.
14. XChat is not presented as generally available.
15. Local CI-equivalent checks, development deployment, and real development-environment acceptance pass before the implementation is reported complete.
16. `POST /v1/billing/x-credits/top-up/checkout` requires Owner/Admin or explicit `billing:write`, requires `Idempotency-Key`, returns server-derived base/bonus/total fields with `expires_at: null`, and never grants Credits before the verified Stripe webhook.
17. Billing, Checkout summary, payment success, email receipt, Pricing, and Plans and limits surfaces visibly state that purchased Credits never expire; this promise is not hidden only in a tooltip or legal terms.
18. Public pages and APIs expose Credits and operation examples without exposing X upstream dollar pricing, internal Credit cost mapping, gross profit, gross margin, or maximum upstream plan cost.
19. Refunds and chargebacks that exceed remaining purchased Credits create `recovery_debt` while spendable balance remains non-negative; subsequent grants offset the debt according to Section 8.4.
20. URL candidate posts reserve 200 Credits under the versioned Section 7.2 classifier; final settlement never produces a surprise negative balance.
21. Included buckets with in-flight reservations cross billing periods using `expired_pending_settlement`; released Credits from an ended period expire rather than reappearing as spendable.
22. The ledger and bucket rows are authoritative, `x_credit_accounts` is a rebuildable cache, and consistency mismatch tests block new reservations and repair from the ledger.
23. Unknown write reservations follow the 1/5/15-minute workflow, release customer-facing balance after the TTL if still inconclusive, and enter the 24-hour internal reconciliation path.
24. The existing 20/day publish safety cap and the inbound daily-spend cap coexist with Credits using the Section 10.4 gate order and distinct error/UI states.
25. Legal review approves the non-expiring, non-transferable, no-cash-value Top-up terms and the Free-plan retained-balance copy for launch jurisdictions.
26. Enterprise is represented as custom/contact-sales rather than `$0`, and public plan/catalog values are generated from a checked source of truth.

### 20.3 Auto top-up post-MVP

Auto top-up has a separate release gate and cannot be marked complete merely because manual Top-up ships. It requires at least 30 days of stable manual-payment reconciliation with zero unresolved duplicate-charge incidents, explicit opt-in, saved-payment-method consent, user-defined threshold, mandatory monthly cap, one in-flight payment, rearm behavior, 3DS recovery, and dedicated Reference/Guidance documentation.

## 21. Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| X changes prices without long notice | Margin compression | Versioned catalog, spending cap, daily reconciliation, emergency operations pause. |
| URL posts cost far more than normal posts | Customer surprise | Preflight estimate, clear examples, validation response, Billing breakdown. |
| Duplicate events or retries | Double charge and trust loss | Immutable ledger, unique idempotency keys, daily resource deduplication. |
| Viral post drains balance through unsolicited inbound events | Unexpected spend and trust loss | Hard workspace daily inbound-spend cap, 80%/100% warnings, rapid subscription pause, and opt-in estimated backfill. |
| Scheduled post executes after balance is depleted | Failed scheduled X target | Estimate at schedule time, low-balance warnings, execution-time reserve, top-up CTA. |
| Unknown reservations lock balance indefinitely | Silent capacity loss and false Auto top-up triggers | 15-minute TTL, 1/5/15-minute reconciliation, customer-facing release, and 24-hour internal review SLA. |
| New DM scopes break existing X connections | Publishing regression | Capability-specific reconnect; keep existing publishing scopes operational. |
| Activity subscription ceiling is reached | New accounts lack real-time Inbox | Capacity monitoring and early higher-capacity application. |
| DM content appears in logs | Privacy/security incident | Payload redaction, log tests, token encryption, restricted support tooling. |
| XChat encryption/API instability | Delayed or unreliable feature | Separate POC and GA gate; ship legacy DM independently. |
| Top-up refunds create inconsistent balance | Financial mismatch | Reversing ledger entries and admin review when balance is already consumed. |
| Non-expiring purchased Credits trigger stored-value obligations or customer confusion on Free | Legal or chargeback exposure | Legal review, non-transferable/no-cash-value terms, explicit Free-plan usability copy, and retained `expires_at = null`. |
| Customers assume Credits override the existing 20/day X safety cap | Support burden and failed publishes | Distinct usage meters, gate ordering, normalized errors, and Pricing/Billing copy that both limits apply. |
| Public copy exposes UniPost unit economics | Pricing and negotiation harm | Internal/public data separation and source tests banning upstream dollar costs and margin from customer-facing surfaces. |
| Auto top-up repeatedly charges a customer | Financial and trust incident | Explicit opt-in, user-defined threshold, required monthly cap, one in-flight payment, threshold-crossing rearm, and no chained purchases. |
| Docs claim more than production supports | Customer confusion | Environment acceptance and source tests; phase-aware XChat copy. |

## 22. Product decisions recorded

The upstream-cost and margin bullets in this section are confidential internal decisions and must not be copied into public surfaces.

- Package model: **included monthly allowance plus purchasable top-ups**.
- Credit accounting unit: **1 credit represents $0.001 upstream X reference cost**.
- Included credits: **0 / 1,500 / 4,000 / 12,000 / 30,000 / custom** for Free / API / Basic / Growth / Team / Enterprise.
- Top-up base conversion: **$1 = 400 base Credits**, before volume bonus.
- Top-up SKUs: **$5 / 2,000**, **$10 / 4,200**, **$25 / 11,000**, **$50 / 23,000**, and **$100 / 48,000**.
- Top-up bonus ladder: **0% / 5% / 10% / 15% / 20%** as purchase size increases.
- Top-up theoretical margin range: **60% down to a 52% floor before payment and operating costs**.
- Included credits do not roll over; purchased Top-up Credits never expire.
- Included credits are consumed before top-ups.
- Manual and Auto top-ups use the same SKU bonus and non-expiration rules.
- Top-up purchase is available through Billing UI and API with `billing:write`, but does not unlock Free X access or API-plan Inbox.
- Auto top-up uses a user-defined editable threshold and required monthly spend cap; no plan-locked threshold is permitted.
- Auto top-up is explicitly post-MVP and ships only after manual Top-up stability gates pass.
- X Credits are separate from posts/month.
- Managed app consumes X Credits; BYO X app does not.
- X comments/replies and legacy DMs are the MVP.
- XChat is a separate gated beta.
- API Reference and Guidance pages are both required, with bidirectional links.

## 23. Open product decision

- **Implementation sequence:** approve Option A (full Credits foundation first) or Option B (recommended bounded-Inbox-first sequence) from Section 2.3. Auto top-up is no longer an open MVP question; it is post-MVP in both options.
