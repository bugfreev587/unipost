# UniPost X Credits, Comments, and DMs PRD

**Status:** Draft for product review

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

## 2. Background

### 2.1 Current UniPost behavior

As of this PRD:

- UniPost supports X publishing, scheduling, media posts, threads, and `first_comment` as a self-reply.
- UniPost can expose X reply counts through Analytics.
- UniPost Inbox does not ingest X comments/replies or X DMs.
- The current X OAuth 2.0 flow requests publishing and read scopes, but not DM scopes.
- X publishing and new X connections require a paid UniPost plan.
- Inbox requires Basic or higher.
- UniPost has monthly post quotas and Stripe subscription billing, but no customer-facing variable-cost credit ledger or one-time top-up product.

### 2.2 Why credits are required

X charges for API resources rather than providing a predictable flat allowance. Current published examples include charges for creating posts, reading posts, reading DM events, sending DMs, and receiving private activity events. A post containing a URL is materially more expensive than a normal post.

If UniPost hides all X variable cost inside a flat subscription, a small number of X-heavy customers can make lower-priced plans unprofitable. If UniPost passes through X's invoice at cost, it has no room for payment fees, infrastructure, reconciliation variance, support, bad debt, refunds, or pricing changes.

The proposed credit model makes cost visible, keeps the base plans simple, and creates a controlled top-up margin.

### 2.3 Official X references

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
| X Credit | UniPost usage unit for managed X API operations. It is not currency and has no cash value. |
| Included credits | Credits granted by a paid plan for the current billing period. They expire at the end of that period and do not roll over. |
| Top-up credits | Credits purchased separately through manual or automatic Top-up. They have no time-based expiration and remain until consumed or reversed because of a refund or chargeback. |
| Managed X app | UniPost's shared X developer app. UniPost pays X for its usage. |
| BYO X app | A customer's X app configured through UniPost Platform Credentials. The customer pays X directly. |
| X reply | A public X post that replies to another post. This is normalized as an Inbox comment workflow. |
| Legacy X DM | A Direct Message accessed through X's Direct Message Event APIs. |
| XChat | X's newer encrypted chat product and `/2/chat/*` APIs. It is a separate later-phase integration. |
| Pricing catalog | Versioned mapping from a UniPost X operation to X Credits consumed. |

## 6. Commercial model

### 6.1 Included credits by plan

| UniPost plan | Monthly price | Existing post capacity | Included X Credits / billing cycle | Maximum upstream cost represented | Inbox eligibility |
|---|---:|---:|---:|---:|---|
| Free | $0 | 100 | 0 | $0.00 | No |
| API | $10 | 1,000 | 1,500 | $1.50 | No |
| Basic | $19 | 2,500 | 4,000 | $4.00 | Yes |
| Growth | $59 | 7,500 | 12,000 | $12.00 | Yes |
| Team | $149 | Unlimited | 30,000 | $30.00 | Yes |
| Enterprise | Custom | Custom | Contract-defined | Contract-defined | Contract-defined |

These allowances keep the maximum represented upstream X cost near 15% to 21% of self-serve subscription revenue while still providing meaningful trial capacity.

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

| SKU | Price | Base Credits | Bonus | Total Credits | Reference upstream cost | Reference gross profit | Theoretical gross margin |
|---|---:|---:|---:|---:|---:|---:|---:|
| `topup_5` | $5 | 2,000 | 0% / 0 | 2,000 | $2.00 | $3.00 | 60% |
| `topup_10` | $10 | 4,000 | 5% / 200 | 4,200 | $4.20 | $5.80 | 58% |
| `topup_25` | $25 | 10,000 | 10% / 1,000 | 11,000 | $11.00 | $14.00 | 56% |
| `topup_50` | $50 | 20,000 | 15% / 3,000 | 23,000 | $23.00 | $27.00 | 54% |
| `topup_100` | $100 | 40,000 | 20% / 8,000 | 48,000 | $48.00 | $52.00 | 52% |

The self-serve minimum is $5 and maximum single purchase is $100. A customer can make another purchase after the first completes. Larger Enterprise commitments require a custom quote and explicit margin review. No self-serve pack may exceed a 20% bonus or reduce theoretical gross margin below 52% without a new product decision.

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

The launch catalog uses the official X resource prices available when this PRD was written. Engineering must verify the catalog immediately before production release.

| UniPost operation | Upstream X resource | Managed-app credits | Effective customer price across $5-$100 packs | Notes |
|---|---|---:|---:|---|
| Create normal X post | Post Create | 15 | $0.0313-$0.0375 | Includes a public reply when X classifies it as normal create. |
| Create post containing URL | Post Create with URL | 200 | $0.4167-$0.5000 | Determine URL classification before reservation. |
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

## 8. Charging and ledger design

### 8.1 Data model

Use an immutable ledger plus materialized balances. Suggested logical entities:

#### `x_credit_accounts`

- `workspace_id`
- `included_available`
- `topup_available`
- `reserved_total`
- `billing_period_start`
- `billing_period_end`
- `plan_id`
- `updated_at`

#### `x_credit_buckets`

- `id`
- `workspace_id`
- `bucket_type`: `monthly_grant` or `topup`
- `granted_credits`
- `available_credits`
- `starts_at`
- `expires_at`: required for `monthly_grant`; always `NULL` for `topup`, enforced by a database constraint
- `source_id`: subscription period, Stripe Checkout Session, manual adjustment, or refund
- `created_at`

#### `x_credit_ledger`

- `id`
- `workspace_id`
- `social_account_id`
- `connection_mode`: `managed` or `platform_credentials`
- `bucket_id`
- `entry_type`: `grant`, `reserve`, `settle`, `release`, `topup`, `refund`, `expire`, or `adjustment`
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
2. Determine the operation key and maximum credit cost before the upstream request.
3. For immediate writes, atomically reserve credits before calling X.
4. Call X with a stable request idempotency key when supported.
5. On success, settle the reservation exactly once.
6. On a confirmed failure where X did not accept the resource, release the reservation.
7. On an unknown timeout, keep the reservation pending and reconcile against X before settling or releasing it.
8. Duplicate API retries with the same UniPost idempotency key return the original result and do not create a second charge.

For scheduled posts, UniPost returns an estimate when the schedule is created but reserves credits only when the worker is ready to publish. If the workspace lacks credits at execution time, the X target fails with normalized code `x_credits_exhausted`, other platform targets continue, and UniPost sends a low-balance/failure notification with a top-up link.

### 8.3 Read and webhook charging

- A public post, user, or DM event is charged only when it is a newly billable resource under the UniPost daily deduplication key.
- Receiving a webhook and later encountering the same resource during backfill must not charge twice.
- Re-reading an item from UniPost's database never consumes X Credits.
- Pagination must expose an estimate before an optional manual backfill that could consume a large number of credits.
- Automated background sync stops before it would make the balance negative.
- Customer-facing ledger entries identify whether consumption came from a webhook, polling, backfill, publish, or reply.

### 8.4 Balance invariants

- Available balance can never be negative.
- A successful managed-app write has exactly one settlement.
- A failed pre-upstream validation has no reservation and no charge.
- A released reservation returns credits to the same originating bucket.
- Top-up webhook handling is idempotent on Stripe Checkout Session or Payment Intent.
- Granting or consuming credits uses a database transaction and row-level locking or an equivalent atomic mechanism.
- Admin adjustments require an actor, reason, and audit log entry.

## 9. Customer experience

### 9.1 Billing page

Add an **X Credits** section to workspace Billing showing:

- Total available credits.
- Included balance and top-up balance separately.
- Current plan's monthly allowance.
- Billing-period reset date.
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

### 9.2 Low-balance states

- At 80% of included allowance consumed: dashboard notice and in-product usage indicator.
- At 95%: email and dashboard warning.
- At 100% of total available balance: blocking warning, top-up CTA, and API error documentation link.
- Notifications are deduplicated per workspace, billing period, and threshold.
- Low-balance notices state which scheduled X posts are at risk before their execution time.

### 9.3 Top-up Checkout

- Use one-time Stripe Checkout in the same live/test mode selection already used by UniPost subscription billing.
- Use the five fixed server-side SKUs in Section 6.4; do not accept arbitrary amounts in the first release.
- Attach `workspace_id`, `sku`, `base_credits`, `bonus_credits`, `total_credits`, and an internal top-up id to Stripe metadata generated by the server.
- Grant credits only after a verified successful Stripe webhook.
- The success page polls the UniPost balance until the idempotent webhook grant is visible.
- Payment failure or Checkout cancellation grants no credits.
- Refunds create reversing ledger entries. They must never silently delete ledger history.
- Manual and Auto top-ups use the same SKU, bonus, margin, and non-expiration rules.
- A paid workspace can use the resulting balance from Dashboard or API operations allowed by its plan. Top-up purchase never bypasses the Free X gate or the Basic Inbox gate.

### 9.4 Auto top-up fast-follow

Auto top-up is not required for the first public release, but the data model and API must not prevent it. When enabled, the workspace owner chooses:

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
- `data.plan_monthly_grant`
- `data.billing_period_start`
- `data.billing_period_end`
- `data.catalog_version`
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

The endpoint requires `Idempotency-Key`. The API derives price, base Credits, bonus, total Credits, and margin from the server-side SKU catalog. It must never trust client-supplied amount or credit quantity.

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
| Top-up SKU is invalid | 422 | `validation_error` | No Checkout Session is created. |
| Stripe top-up is unavailable | 503 | `billing_unavailable` | Existing balance remains unchanged. |
| Auto top-up requires customer action | 409 | `auto_topup_action_required` | Pause automatic charges and return the Stripe recovery URL. |
| Auto top-up monthly cap reached | 402 | `auto_topup_monthly_cap_reached` | Do not charge; notify the owner and use the remaining balance until exhausted. |
| Auto top-up payment failed | 402 | `auto_topup_payment_failed` | Grant no Credits, disable automatic retries, and return manual recovery guidance. |
| X account lacks new scopes | 409 | `x_reconnect_required` | Return reconnect URL and missing scopes. |
| X Inbox unavailable on plan | 402 | `plan_feature_not_available` | Existing Basic gate semantics. |
| XChat is not enabled for the account | 422 | `x_chat_not_supported` | Legacy DM support remains unaffected. |

All new errors follow the existing UniPost error envelope and include `request_id`.

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

X currently documents self-serve Activity subscription capacity limits. If each connected account needs multiple subscriptions, UniPost must alert before 70%, 85%, and 95% of the app limit and pursue higher capacity before onboarding would exceed it. At three subscriptions per account, a 1,500-subscription limit represents only about 500 fully subscribed accounts.

### 14.5 Developer access request narrative

Use the following substance in any X access, capacity, or support application:

> UniPost is a multi-tenant social publishing and customer-engagement platform. A customer explicitly connects an X account through OAuth and authorizes UniPost to publish, receive replies or messages addressed to that account, and send replies on the account owner's behalf. UniPost uses X data only to provide the connected customer's publishing, Inbox, support, and analytics workflows. We do not sell X content, build surveillance profiles, or use private messages to train public models. Access is workspace-scoped, tokens are encrypted, webhook signatures are verified, sensitive message bodies are excluded from logs, disconnect removes activity subscriptions, and workspace deletion follows our documented data-deletion process. We apply least-privilege scopes, usage limits, abuse controls, and customer-visible cost accounting.

The actual application must include current traffic estimates, projected connected X accounts, event volume, retention policy, privacy URL, terms URL, security contact, and the requested Activity subscription capacity.

## 15. Documentation requirements

Documentation is part of the feature definition, not a launch follow-up.

### 15.1 New API Reference pages

Create these pages using the existing API Reference components and navigation:

| Route | Purpose | Must link to |
|---|---|---|
| `/docs/api/x-credits` | X Credits overview, catalog, managed vs BYO behavior | X Credits guide; Plans and limits |
| `/docs/api/x-credits/balance` | `GET /v1/billing/x-credits` request/response | X Credits guide |
| `/docs/api/x-credits/transactions` | `GET /v1/billing/x-credits/transactions` filters and ledger fields | X Credits guide |
| `/docs/api/x-credits/top-up` | Manual Top-up Checkout, five SKUs, bonus fields, idempotency, and non-expiration | X Credits guide |
| `/docs/api/x-credits/auto-top-up` | Payment-method setup and user-configured Auto top-up API | X Credits guide |
| `/docs/api/inbox/list` | Dedicated `GET /v1/inbox` contract including `x_reply` and `x_dm` | X comments guide; X DMs guide |
| `/docs/api/inbox/reply` | Dedicated `POST /v1/inbox/{id}/reply` contract and X credit result fields | X comments guide; X DMs guide |
| `/docs/api/inbox/sync` | Dedicated `POST /v1/inbox/sync`, estimates, backfill, and credit behavior | X comments guide; X DMs guide |

Also update:

- `/docs/api/inbox` to list `x_reply`, `x_dm`, and later `x_chat` only when that phase ships.
- `/docs/api/posts/create` and `/docs/api/posts/validate` for estimates, actual-charge fields, BYO behavior, and `x_credits_exhausted`.
- `/docs/api/billing` to link to the X Credits API family.
- `/docs/api/errors` with every normalized error in Section 10.3.
- The API Reference sidebar, index, search index, sitemap, and structured metadata.

### 15.2 New Guidance pages

Create task-oriented Guidance pages:

| Route | User task | Must link back to |
|---|---|---|
| `/docs/guides/x/credits-and-top-ups` | Estimate X cost, inspect balance, buy discounted non-expiring Top-ups, configure Auto top-up, and handle exhaustion | Balance, transactions, manual/Auto top-up, create-post, and validate references |
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
  - Add included credits by plan, reset behavior, the five-pack Top-up ladder, 0%-20% bonus, 60%-52% theoretical margin model, non-expiration, and separate post/X-credit enforcement.
  - Mirror the operation-capacity table and calculation definitions from Section 6.2.
  - Mirror the Top-up operation examples from Section 6.4 and explain that each example spends the whole pack on one operation type.
- `/pricing`
  - Add one concise X Credits allowance line per plan and a Top-up FAQ covering the five packs, volume bonus, manual/API purchase, Auto top-up, and non-expiration.
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
- Pending reservations older than the reconciliation SLA.
- Duplicate webhook event attempts and deduplication rate.
- Activity subscription capacity at 70%, 85%, and 95%.
- Low-balance blocks and successful top-ups after a block.
- Manual and automatic Top-up attempts, failures, customer-action requirements, monthly-cap pauses, and duplicate-prevention outcomes.
- DM/reply webhook latency, backfill lag, and reply success rate.
- Gross margin for included allowance and Top-ups separately, broken down by SKU and manual versus automatic purchase mode.

### 16.2 Reconciliation

- Run a daily reconciliation comparing X usage exports/console data with UniPost ledger aggregation.
- Flag material variance by operation, not only total dollars.
- Do not retroactively charge customers for an internal UniPost bug without an explicit reviewed policy.
- Undercharging caused by a catalog or integration bug is absorbed until a corrected catalog is published.
- Overcharging is corrected with an auditable credit adjustment.
- Pending timeout reservations reconcile automatically; unresolved cases enter an admin review queue.

### 16.3 Cost controls

- Set X-side spending limits per environment.
- Development and staging must use non-production apps or separately bounded credentials where possible.
- Stop optional polling before essential outbound writes when a workspace balance is low.
- Unsubscribe or pause paid private events when a workspace has zero balance and no viable top-up, if X supports doing so without losing required account state.
- Restore subscriptions and offer bounded backfill after balance is replenished.
- Cache public data and authors within allowed policy windows.

## 17. Rollout plan

No feature flag is required by default. Use normal environment promotion, plan gates, capability checks, and explicit beta eligibility.

### Phase 0 — X access and pricing validation

- Confirm current X pricing.
- Add X API funds and spending limits.
- Configure scopes, callbacks, webhook, CRC, and signatures.
- Submit capacity/access request if projected subscriptions exceed self-serve limits.
- Validate developer, staging, and production app separation.

### Phase 1 — Credits foundation

- Ledger, buckets, grants, reservations, settlement, reconciliation.
- Balance and transaction APIs.
- Billing UI and manual Stripe top-ups.
- Managed-vs-BYO branching.
- Publish estimate and actual-charge metadata.
- Low-balance notifications and hard block.
- API Reference and X Credits Guidance pages.

### Phase 1B — Auto top-up fast-follow

- Explicit opt-in and Stripe reusable-payment-method setup.
- User-defined threshold, selected SKU, and required monthly spend cap.
- Off-session payment, 3DS/customer-action recovery, notifications, and anti-loop behavior.
- Auto top-up API Reference and updated X Credits Guidance.
- This phase is not included in the MVP total unless product explicitly promotes Auto top-up to launch scope.

### Phase 2 — X public comments/replies

- Activity/webhook ingestion and backfill.
- `x_reply` normalization and Inbox UI.
- Outbound public replies.
- Dedicated Inbox Reference pages and X comments Guidance.

### Phase 3 — Legacy X DMs

- OAuth reconnect for new scopes.
- DM subscriptions, initial sync, and outbound replies.
- `x_dm` Inbox source.
- X DMs and reconnect Guidance.

### Phase 4 — XChat proof of concept

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
| X developer setup, webhooks, Activity subscriptions, reconnect scopes | 1-2 engineer-weeks plus external X approval time |
| X comments/replies ingestion, normalization, UI, outbound replies | 2-3 engineer-weeks |
| Legacy X DM sync, UI integration, outbound replies, privacy hardening | 2-3 engineer-weeks |
| API Reference, Guidance, links, tests, and launch copy | 1-1.5 engineer-weeks |
| End-to-end QA, billing reconciliation, and rollout hardening | 1.5-2 engineer-weeks |

**MVP total:** approximately **12.5-18.5 engineer-weeks**, or roughly **7-10 calendar weeks** with two engineers and no external X approval delay.

**Auto top-up fast-follow:** add approximately **1-2 engineer-weeks**. If promoted into MVP, add this range to the MVP total and release schedule.

**XChat later phase:** add approximately **4-8+ engineer-weeks** after official tooling and permission feasibility are proven. This range is intentionally separate because encrypted key management and evolving XChat APIs are the largest uncertainty.

## 19. Success metrics

- Managed X gross margin stays within the approved floor by plan and in aggregate.
- Top-up theoretical gross margin matches the approved SKU ladder and never falls below 52% before payment and operating costs under the launch catalog.
- Duplicate customer charges: zero confirmed incidents.
- Successful top-up grant after verified Stripe payment: at least 99.9%.
- X reply and legacy DM outbound success rate: at least 98%, excluding platform policy/rate-limit failures.
- P95 inbound webhook-to-Inbox latency: under 60 seconds.
- Daily X invoice-to-ledger variance: below the finance-defined materiality threshold, with all larger variances explained.
- At least 90% of `x_credits_exhausted` responses include an actionable balance/top-up link.
- Documentation bidirectional-link tests pass in CI.

## 20. Acceptance criteria

The MVP is complete only when:

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
12. The Pricing page and Plans and limits docs accurately show allowances, the five-pack graduated Top-up ladder, 0%-20% bonus, 60%-52% theoretical margin range, non-expiration, separate quota behavior, and the complete Section 6.2 and 6.4 operation-capacity tables. Desktop tables and mobile cards are generated from versioned allowance/catalog data, use floor rounding, label API Inbox operations unavailable, and pass source tests for every displayed number.
13. All new API Reference and Guidance pages in Section 15 exist, are in navigation/search/sitemap, and pass the bidirectional linking tests.
14. XChat is not presented as generally available.
15. Local CI-equivalent checks, development deployment, and real development-environment acceptance pass before the implementation is reported complete.
16. `POST /v1/billing/x-credits/top-up/checkout` requires Owner/Admin or explicit `billing:write`, requires `Idempotency-Key`, returns server-derived base/bonus/total fields with `expires_at: null`, and never grants Credits before the verified Stripe webhook.
17. Billing, Checkout summary, payment success, email receipt, Pricing, and Plans and limits surfaces visibly state that purchased Credits never expire; this promise is not hidden only in a tooltip or legal terms.

## 21. Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| X changes prices without long notice | Margin compression | Versioned catalog, spending cap, daily reconciliation, emergency operations pause. |
| URL posts cost far more than normal posts | Customer surprise | Preflight estimate, clear examples, validation response, Billing breakdown. |
| Duplicate events or retries | Double charge and trust loss | Immutable ledger, unique idempotency keys, daily resource deduplication. |
| Scheduled post executes after balance is depleted | Failed scheduled X target | Estimate at schedule time, low-balance warnings, execution-time reserve, top-up CTA. |
| New DM scopes break existing X connections | Publishing regression | Capability-specific reconnect; keep existing publishing scopes operational. |
| Activity subscription ceiling is reached | New accounts lack real-time Inbox | Capacity monitoring and early higher-capacity application. |
| DM content appears in logs | Privacy/security incident | Payload redaction, log tests, token encryption, restricted support tooling. |
| XChat encryption/API instability | Delayed or unreliable feature | Separate POC and GA gate; ship legacy DM independently. |
| Top-up refunds create inconsistent balance | Financial mismatch | Reversing ledger entries and admin review when balance is already consumed. |
| Auto top-up repeatedly charges a customer | Financial and trust incident | Explicit opt-in, user-defined threshold, required monthly cap, one in-flight payment, threshold-crossing rearm, and no chained purchases. |
| Docs claim more than production supports | Customer confusion | Environment acceptance and source tests; phase-aware XChat copy. |

## 22. Product decisions recorded

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
- X Credits are separate from posts/month.
- Managed app consumes X Credits; BYO X app does not.
- X comments/replies and legacy DMs are the MVP.
- XChat is a separate gated beta.
- API Reference and Guidance pages are both required, with bidirectional links.

## 23. Open product decision

- **Auto top-up launch phase:** the design is complete and currently scheduled as Phase 1B fast-follow. Product must explicitly decide whether to promote it into MVP; until then, it is not included in the 12.5-18.5 engineer-week MVP estimate.
