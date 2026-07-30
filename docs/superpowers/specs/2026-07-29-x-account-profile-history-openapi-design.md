# X Account Profile and Post History OpenAPI Design

**Date:** 2026-07-29
**Status:** Revised after design review; product decisions approved; implementation plan pending
**Primary consumer:** MySuperX through UniPost Hosted Connect

## Context

MySuperX connects each managed user's X account through UniPost Hosted Connect. After connection, MySuperX needs the account's profile and recent authored posts so that MySuperX can generate a Writing DNA in its own system.

UniPost already requests the X OAuth scopes needed for profile and post reads, stores the connected X account identity and tokens, exposes workspace-scoped account APIs, and has an X Credits reservation and settlement ledger. It does not currently expose a public account-level API for a live X profile or for posts authored directly on X.

The original request proposed an asynchronous snapshot capped at 200 posts and included MySuperX-specific concepts such as a 20-post minimum sample. Product review changed that boundary: UniPost will expose a general, synchronous, cursor-paginated OpenAPI and account for the X resources it reads. The caller decides how many pages to request and whether the returned sample is sufficient for its application.

## Decision summary

P0 adds these public capabilities:

1. Account capability discovery for profile and owned-post-history reads.
2. Live retrieval and normalization of a connected X account's profile.
3. Synchronous, cursor-paginated retrieval and normalization of posts authored by that account.
4. Feature-flag-controlled X Credits preauthorization, actual-usage settlement, idempotent replay, and ambiguous-outcome reconciliation.
5. Workspace X Credits balance and transaction-ledger visibility.
6. Workspace, account, and managed-user isolation on every profile or post-content read while preserving the existing Workspace-only capability-discovery contract.

P0 does not add asynchronous snapshots, webhooks, Writing DNA analysis, sample-size policy, or inferred thread positions and totals.

## Goals

- Let a server-side OpenAPI consumer read the current profile of an X account connected through UniPost.
- Let the consumer scan that account's recent authored posts in reverse chronological order using an opaque cursor.
- Return stable, provider-neutral fields while preserving the X identifiers needed for caller-side deduplication and thread reconstruction.
- Fail closed across Workspace and Managed User boundaries.
- Deny a billable read before calling X when the workspace cannot afford the request's maximum possible Credits charge.
- Charge for the X resources actually read, including resources later removed by local filters.
- Make every cost-bearing upstream read idempotent and auditable, even when customer accounting is bypassed.
- Reuse and generalize the existing X Credits exposure reservation and settlement machinery rather than create a second billing state machine.
- Avoid retaining a reusable corpus of historical posts in UniPost.

## Non-goals

- Generate, score, or store Writing DNA.
- Decide whether 20, 50, 200, or another number of posts is enough for a downstream use case.
- Fetch multiple X pages inside one public API request.
- Create or retain historical-content snapshots.
- Notify completion through webhooks.
- Guarantee a complete historical archive beyond the range X makes available.
- Infer `thread.position` or `thread.total_posts` without a complete, verified conversation.
- Fetch an arbitrary X user's profile or timeline from a caller-supplied X user ID.
- Include quoted users' text in the connected user's authored text.
- Use the returned content to train a public model.

## Approaches considered

### 1. Synchronous cursor pagination — selected

Each `/posts` call maps to at most one X user-timeline page. The caller chooses a `limit` from 5 through 100 and follows an opaque `next_cursor` until it has enough data or `has_more` is false.

This is the most general OpenAPI contract. It bounds request duration and charge, keeps the caller in control, and avoids putting MySuperX's 200-post or sample-sufficiency policy into UniPost.

### 2. Asynchronous snapshots

Snapshots are useful for large background exports and webhook-driven workflows, but they add job state, retention, deletion, cache reuse, partial-completion semantics, and a second content store. They are deferred to a future batch API if demonstrated demand warrants them.

### 3. One request that internally fetches up to 200 posts

This would hide pagination from the consumer, but it creates long-running requests, larger preauthorizations, more complicated partial failures, and an application-specific cap. It is rejected for P0.

## Existing foundations to reuse

- The X OAuth connection already requests `tweet.read`, `users.read`, and `offline.access`.
- The account records already contain the X user ID, username, display name, avatar, encrypted access token, encrypted refresh token, scopes, connection mode, Workspace ownership, and optional `external_user_id`.
- The public API already uses `{data, meta, request_id}` success envelopes and structured error envelopes.
- The account capability route already exists and can be extended additively without making its current Workspace-only callers supply a new parameter.
- `api/internal/xcredits` already supports atomic reservation, finalization, reversal, monthly allowances, catalog versions, and duplicate idempotency detection.
- The X Credits catalog already defines `user.read`, `post.read`, and `post.read_owned` operations.
- Existing Inbox code provides a model for explicit Managed User scoping and for ambiguous X-read reconciliation.

## Validated upstream constraints

The design relies on the current official X API contract and must be rechecked during implementation because provider limits and pricing can change:

- [`GET /2/users/:id/tweets`](https://docs.x.com/x-api/users/get-posts) is the canonical authored-post timeline endpoint.
- X documents a maximum of 100 results per page and access to as many as 3,200 of the user's most recent posts, subject to account history, access tier, and deletion or availability constraints.
- Pagination uses an upstream token; UniPost wraps that token in its own scoped opaque cursor.
- The current X pricing documentation lists Post Read and User Read as per-resource operations. UniPost's customer-facing Credits remain governed by the versioned UniPost catalog, not a hard-coded copy of X's dollar price.
- X's discounted owned-resource category refers to the developer app owner's resources, not simply any post authored by an end user connected to the app. UniPost therefore uses the conservative `post.read` operation until provider billing evidence proves otherwise.

References: [X timelines introduction](https://docs.x.com/x-api/posts/timelines/introduction), [user-post timeline integration](https://docs.x.com/x-api/posts/timelines/integrate), [X pricing](https://docs.x.com/x-api/getting-started/pricing), and [X rate limits](https://docs.x.com/x-api/fundamentals/rate-limits).

## Authentication and authorization contract

### Credential boundary

All endpoints in this design use UniPost's existing Workspace API-key authentication. The API key stays on the customer's backend and must not be sent to a managed user's browser.

The authenticated `workspace_id` always comes from the API key. The caller cannot supply or override it.

### Managed User selector and compatibility

The two endpoints that return profile or post content require exactly one nonempty `external_user_id` query parameter:

```text
GET /v1/accounts/{id}/profile?external_user_id=user_123
GET /v1/accounts/{id}/posts?external_user_id=user_123&limit=100
```

The already-deployed capability endpoint keeps `external_user_id` optional:

```text
GET /v1/accounts/{id}/capabilities
GET /v1/accounts/{id}/capabilities?external_user_id=user_123
```

Without the selector it preserves the current Workspace-only discovery behavior. When the selector is present, it validates exact Managed User ownership. This is safe because the endpoint returns capability and billing-policy metadata, not profile or post content, and avoids breaking existing non-Hosted-Connect callers.

Authorization requires all of the following:

```text
API key workspace
  == account profile workspace

request external_user_id
  == social_accounts.external_user_id
```

Rules:

- An account outside the authenticated Workspace returns `404 ACCOUNT_NOT_FOUND`.
- For `/profile` and `/posts`, an account in the Workspace whose `external_user_id` differs from the request returns `403 ACCOUNT_ACCESS_DENIED`.
- For `/profile` and `/posts`, a missing or empty `external_user_id` returns `400 VALIDATION_ERROR`.
- When `/capabilities` receives `external_user_id`, a mismatch also returns `403 ACCOUNT_ACCESS_DENIED`.
- An account with a null `external_user_id` remains discoverable through the existing Workspace-only `/capabilities` call but is not readable through `/profile` or `/posts`.
- A non-X account returns its existing platform capability response without `x_account_reads`; `/profile` and `/posts` return `400 WRONG_PLATFORM`.
- The caller cannot supply an external X user ID. UniPost always derives the X user ID from the authorized account record.
- Account disconnect or deletion immediately prevents new live reads.

This provides confinement within the selected Managed User. As with the existing managed-user server-to-server model, UniPost cannot independently prove that a valid Workspace API-key holder selected the customer application's currently authenticated user. The customer backend remains responsible for deriving `external_user_id` from its own authenticated session.

## API contract

All new success responses use the existing top-level envelope:

```json
{
  "data": {},
  "meta": {},
  "request_id": "req_xxx"
}
```

### Account capabilities

```text
GET /v1/accounts/{id}/capabilities
GET /v1/accounts/{id}/capabilities?external_user_id=user_123
```

This endpoint does not call X and does not consume Credits. The optional selector behaves as defined above. It increments the capability schema from `1.7` to `1.8` and adds an `x_account_reads` namespace alongside the existing `capability` and `x_inbox` fields:

```json
{
  "data": {
    "schema_version": "1.8",
    "account_id": "sa_xxx",
    "platform": "twitter",
    "capability": {},
    "x_account_reads": {
      "profile_read": {
        "supported": true,
        "authorized": true,
        "reconnect_required": false,
        "credits": {
          "accounting_enabled": true,
          "billing_mode": "unipost_managed_app",
          "bypass_reason": null,
          "operation": "user.read",
          "catalog_credits_per_resource": 10,
          "effective_credits_per_resource": 10
        }
      },
      "own_post_history_read": {
        "supported": true,
        "authorized": true,
        "reconnect_required": false,
        "min_page_size": 5,
        "max_page_size": 100,
        "credits": {
          "accounting_enabled": true,
          "billing_mode": "unipost_managed_app",
          "bypass_reason": null,
          "operation": "post.read",
          "catalog_credits_per_resource": 5,
          "effective_credits_per_resource": 5
        }
      }
    }
  },
  "request_id": "req_xxx"
}
```

`authorized` is based on connection state, the stored granted scopes, and whether usable refresh credentials are present. A successful connection alone must not imply that live history is readable.

When `x_credits_billing_v1` is disabled for the Workspace, `accounting_enabled` is false, `effective_credits_per_resource` is zero, and `bypass_reason` is `feature_disabled`. For a Workspace-owned X app, the effective price is also zero and `bypass_reason` is `customer_x_app`. The nominal catalog operation and weight remain visible so callers can understand the policy that will apply if customer accounting is later enabled.

### Account profile

```text
GET /v1/accounts/{id}/profile?external_user_id=user_123
Idempotency-Key: <caller-generated-key>
```

`Idempotency-Key` is required because the request has an upstream cost and may have an ambiguous network outcome, even when customer X Credits accounting is bypassed.

UniPost performs a live X user lookup for the connected account and returns:

```json
{
  "data": {
    "account_id": "sa_xxx",
    "platform": "twitter",
    "external_account_id": "123456789",
    "username": "victoria",
    "display_name": "Victoria",
    "description": "Founder building AI products...",
    "profile_image_url": "https://...",
    "location": "San Francisco",
    "website_url": "https://...",
    "account_created_at": "2020-01-01T00:00:00Z",
    "verified": false,
    "public_metrics": {
      "followers": 1200,
      "following": 350,
      "posts": 4200,
      "listed": 12
    },
    "retrieved_at": "2026-07-29T10:00:00Z"
  },
  "meta": {
    "credits": {
      "operation_id": "xro_xxx",
      "status": "finalized",
      "accounting_enabled": true,
      "billing_mode": "unipost_managed_app",
      "bypass_reason": null,
      "operation": "user.read",
      "estimated": 10,
      "reserved": 10,
      "charged": 10,
      "released": 0,
      "catalog_version": "x-credits-2026-07-16-v1"
    }
  },
  "request_id": "req_xxx"
}
```

Required P0 profile fields are `external_account_id`, `username`, `display_name`, `description`, `profile_image_url`, `account_created_at`, and `retrieved_at`. Optional upstream fields are omitted or returned as null when X does not provide them; their absence does not fail an otherwise valid profile response.

There is no reusable profile cache in P0. The only retained successful response is the 24-hour idempotency receipt described below.

### Authored post history

```text
GET /v1/accounts/{id}/posts
  ?external_user_id=user_123
  &limit=100
  &cursor=<opaque-cursor>
  &start_time=2025-07-29T00:00:00Z
  &end_time=2026-07-29T00:00:00Z
  &exclude_reposts=true
  &exclude_replies_to_others=true
Idempotency-Key: <caller-generated-key-for-this-page>
```

Parameters:

- `external_user_id` is required.
- `limit` is required and must be between 5 and 100 inclusive.
- `cursor` is optional on the first page and required to continue a scan.
- `start_time` and `end_time` are optional RFC 3339 timestamps and must form a valid range accepted by X.
- `exclude_reposts` defaults to `false` in the general API.
- `exclude_replies_to_others` defaults to `false` in the general API.
- `Idempotency-Key` is required and must be unique per logical page request.

`limit` is the maximum number of upstream X post resources scanned on that page. It is not a promise that the same number of posts will remain after local filtering.

Example response:

```json
{
  "data": [
    {
      "external_post_id": "1900000000000000000",
      "account_id": "sa_xxx",
      "text": "The actual post text...",
      "created_at": "2026-07-20T09:30:00Z",
      "language": "en",
      "conversation_id": "1900000000000000000",
      "content_type": "original_post",
      "reply_to_external_post_id": null,
      "is_reply": false,
      "is_self_reply": false,
      "is_repost": false,
      "is_quote": false,
      "thread": {
        "thread_id": "1900000000000000000"
      },
      "media": [{"type": "image"}],
      "public_metrics": {
        "likes": 120,
        "replies": 14,
        "reposts": 22,
        "quotes": 5,
        "impressions": 8000
      }
    }
  ],
  "meta": {
    "limit": 100,
    "scanned_count": 100,
    "returned_count": 64,
    "has_more": true,
    "next_cursor": "opaque-value",
    "cursor_expires_at": "2026-08-05T10:00:00Z",
    "credits": {
      "operation_id": "xro_xxx",
      "status": "finalized",
      "accounting_enabled": true,
      "billing_mode": "unipost_managed_app",
      "bypass_reason": null,
      "operation": "post.read",
      "estimated": 500,
      "reserved": 500,
      "charged": 500,
      "released": 0,
      "catalog_version": "x-credits-2026-07-16-v1"
    }
  },
  "request_id": "req_xxx"
}
```

An account with no matching posts returns `200`, an empty `data` array, `returned_count: 0`, and the correct `has_more` value. It is not an error.

## Post normalization

P0 requests the X fields needed to produce:

- `id`
- `text`
- `created_at`
- `lang`
- `conversation_id`
- `in_reply_to_user_id`
- `referenced_tweets`
- optional attachments and public metrics

Normalization rules:

- `external_post_id` is the X post ID and is the caller's stable deduplication key.
- `conversation_id` comes directly from X.
- `thread.thread_id` equals `conversation_id`.
- `reply_to_external_post_id` is the referenced post whose relation type is `replied_to`, when present.
- `is_reply` is true when the post has a reply relationship.
- `is_self_reply` is true only when `is_reply` is true and `in_reply_to_user_id` equals the connected account's X user ID.
- `is_repost` is true when the post has a `retweeted` reference.
- `is_quote` is true when the post has a `quoted` reference.
- `content_type` is `repost`, `reply`, `quote`, or `original_post`, evaluated in that precedence order.
- `exclude_replies_to_others=true` removes replies for which `is_reply=true` and `is_self_reply=false`; it preserves self-replies used in threads.
- `exclude_reposts=true` removes posts for which `is_repost=true`.
- A Quote Post remains the connected user's own text with `is_quote=true`. P0 does not merge the quoted user's text into `text`.
- Missing optional media or metrics never prevents the core post from being returned.

P0 deliberately does not return `thread.position` or `thread.total_posts`. Those values cannot be verified from a single reverse-chronological timeline page. The caller can group by `conversation_id`, order by `created_at`, and merge additional pages. A future endpoint may provide a separately billed complete-conversation read:

```text
GET /v1/accounts/{id}/threads/{conversation_id}
```

That endpoint is not part of this design's P0 scope.

## Cursor contract

The public cursor is opaque, authenticated, and encrypted. Its payload binds:

- Workspace ID;
- account ID;
- `external_user_id`;
- canonical filter values;
- requested time range;
- upstream X pagination token;
- issuance and expiration timestamps.

The UniPost cursor expires after seven days and every page response returns `cursor_expires_at`. A modified, expired, cross-account, cross-user, or filter-mismatched cursor returns `400 INVALID_CURSOR`. UniPost must not expose the upstream X pagination token directly. X may impose a shorter undocumented validity period on its own token; UniPost cannot promise continuation after X rejects that token.

Each continuation page requires a new `Idempotency-Key`. Replaying a completed page with the same key returns the stored response during the 24-hour receipt period. If UniPost returns a retriable error whose `Retry-After` would extend beyond the cursor's remaining lifetime, the error details include a newly signed `retry_cursor` wrapping the same upstream token. If the caller simply allows a cursor to expire, restarting the scan is a new billable operation; UniPost does not refund caller-caused expiry.

UniPost does not intentionally duplicate a post within a response. Callers must still upsert by `external_post_id` because a live timeline can change between page requests.

## X Credits policy

### Existing feature flag and app-mode matrix

All customer X Credits accounting introduced by these endpoints goes through the existing `x_credits_billing_v1` Workspace evaluation and the existing `xcredits.RolloutService`. The flag is currently globally off; the evaluator can still enable it for an eligible super-admin-owned Workspace according to the existing rollout behavior.

The flag controls customer accounting, not availability of `/profile` or `/posts`. Reads remain usable while customer accounting is bypassed, subject to authentication, X authorization, operational rate limits, and internal cost-safety controls.

| X app mode | `x_credits_billing_v1` | Customer reservation and charge | Receipt metadata |
|---|---|---|---|
| UniPost managed | enabled for Workspace | Reserve, enforce 402, settle actual usage | `status=finalized`, `accounting_enabled=true` |
| UniPost managed | disabled for Workspace | Bypass customer balance and 402 | `status=bypassed`, `bypass_reason=feature_disabled` |
| Workspace-owned X app | either | Always bypass customer X Credits | `status=bypassed`, `bypass_reason=customer_x_app` |

When accounting is bypassed, the operation still requires an idempotency receipt and still participates in internal cost-safety and request-rate controls. Its response contains a Credits block with `estimated`, `reserved`, `charged`, and `released` all equal to zero rather than omitting the block. This makes the behavior explicit and keeps one stable response shape.

If feature-flag evaluation fails, UniPost fails closed before calling X. The handler must not assume that a flag-evaluation error means disabled accounting.

### Catalog operations

- Profile lookup uses `user.read` and currently costs 10 Credits per user resource.
- Timeline reads use `post.read` and currently cost 5 Credits per upstream post resource.
- Catalog weights always come from the versioned X Credits catalog, not handler constants.
- `post.read_owned` must not be used merely because the connected user authored the post. It is permitted only if UniPost has verified that X classifies the read as an app-owner read for the UniPost developer app. Until then, P0 defaults to `post.read`.

**Product decision, 2026-07-29:** P0 uses `post.read` at the current catalog weight of 5 Credits per resource when customer accounting is enabled. If later X invoice evidence proves these reads qualify for `post.read_owned`, UniPost will publish a new catalog version and apply it only to requests created after that version becomes active. Historical usage will not be recomputed, refunded, or back-charged.

### Preauthorization

Before any customer-billable X call, UniPost calculates the maximum charge:

```text
profile estimate = 1 * weight(user.read)
posts estimate   = limit * weight(post.read)
```

When customer accounting is enabled, UniPost atomically reserves that amount against the Workspace's monthly allowance. Pending reservations count against available balance so concurrent reads cannot overspend. When accounting is bypassed by feature flag or app mode, the customer reservation is zero and insufficient customer balance cannot produce a 402.

If the reservation cannot be made, UniPost returns `402 INSUFFICIENT_X_CREDITS` and performs zero X calls. Its error details contain:

```json
{
  "estimated_credits": 500,
  "available_credits": 240,
  "max_affordable_limit": 48
}
```

For `/posts`, `max_affordable_limit` is `floor(available_credits / current_post_read_weight)`, capped at 100. Values below the minimum page size are returned as `0`. UniPost does not silently reduce the requested limit.

Both `available_credits` and `max_affordable_limit` remain in the error. Although mathematically related, the latter applies the active catalog weight and public page-size bounds so clients do not need to duplicate UniPost admission policy.

A separate estimate endpoint is intentionally omitted. A standalone estimate can become stale before the actual reservation; the estimate and atomic reservation belong in the real request.

### Settlement

After a successful X response:

- Profile final charge is the number of X user resources read, normally one.
- Posts final charge is `scanned_count * current post-read weight`.
- `scanned_count` is the number of upstream post resources admitted for normalization before local filtering, capped at the requested `limit`.
- Filtering 100 scanned posts down to 20 returned posts still charges for 100 resources.
- Any unused reservation is released atomically during finalization.
- The response reports estimated, reserved, charged, and released Credits.

The customer charge can never exceed the preauthorized amount. If X violates its contract and returns more than `limit`, UniPost admits and returns at most `limit`, charges at most the reservation, records the larger raw count only in sanitized internal telemetry, alerts operators, and absorbs any excess provider cost.

A definite failure before X returns billable resources reverses the reservation. An X rate-limit response with no returned post resources does not consume the reservation.

### Idempotency

`Idempotency-Key` is mandatory on `/profile` and `/posts`.

The request fingerprint includes the authenticated Workspace, account, Managed User, HTTP method, route, and canonical query parameters. Behavior is:

- Same key and same fingerprint while completed: replay the stored response, with no new X call and no new charge.
- Same key and same fingerprint while executing: return `409 READ_IN_PROGRESS` with `Retry-After`; do not start a concurrent X call.
- Same key and same fingerprint while reconciling an ambiguous outcome: return `409 READ_SETTLEMENT_PENDING` with `Retry-After`; do not start a concurrent X call.
- Same key and different fingerprint: return `409 IDEMPOTENCY_CONFLICT`.
- Receipt expiration after 24 hours permits a later request to be treated as new and billed again.

### Ambiguous upstream outcomes

If UniPost sends the X request but a timeout, connection interruption, or truncated response makes the resource count unknowable:

1. The read operation enters `outcome_unknown`; its Credits remain reserved.
2. The client receives `409 READ_SETTLEMENT_PENDING`, `is_retriable: true`, and `Retry-After`.
3. Identical client retries observe the same operation rather than creating another concurrent read.
4. A background reconciler retries the exact logical request with bounded backoff and records every attempt.
5. A successful reconciliation finalizes actual usage and stores the replayable normalized response.
6. A definite no-resource failure releases the reservation.
7. If UniPost still cannot establish the outcome after 24 hours, it releases the user's reservation, records an internal accounting anomaly, and alerts operators.
8. UniPost never back-charges the customer after that 24-hour release. Any later provider cost is absorbed by UniPost.

This avoids both indefinite customer holds and unsupported maximum-charge billing.

All retries remain one customer operation and can produce at most one customer charge. If X bills UniPost more than once while UniPost resolves an ambiguous read, the additional provider cost is not passed through as another customer charge.

## Credits visibility

### Workspace balance

The existing endpoint remains the authoritative Workspace snapshot:

```text
GET /v1/billing/x-credits
```

It remains non-billable and is extended additively to distinguish:

- finalized usage;
- pending reservations;
- effective usage (`finalized + pending`);
- monthly allowance;
- available balance;
- billing-period boundaries;
- catalog version.

Existing fields remain backward compatible. `monthly_remaining` must use effective usage so it matches admission decisions.

The current aggregate counter already includes provisional reservations. Splitting finalized, pending, and effective values is not a field rename: implementation requires a new status-aggregated `x_usage_events` query and corresponding snapshot tests. This work must be a distinct implementation-plan task.

This endpoint retains its existing `x_credits_billing_v1` availability behavior. For an ordinary Workspace while the flag is off, the billing endpoint remains unavailable even though `/profile` and `/posts` may execute with customer accounting bypassed.

### Transaction ledger

P0 adds:

```text
GET /v1/billing/x-credits/events
```

This endpoint is Workspace scoped, does not call X, and consumes no X Credits. It uses the same `x_credits_billing_v1` availability gate as the existing Workspace balance endpoint. It supports cursor pagination and optional filters for:

- `account_id`;
- `external_user_id`;
- operation;
- status;
- start and end time.

Each item contains:

- public `operation_id`;
- account ID and Managed User ID;
- operation key and catalog version;
- estimated, reserved, charged, and released Credits;
- state such as `reserved`, `finalized`, `released`, or `reconciliation_pending`;
- created, updated, finalized, and expiry timestamps as applicable.

The ledger never returns an OAuth token, response body, raw X payload, post text, raw upstream pagination token, or raw `Idempotency-Key`.

## Settlement architecture, data model, and retention

### Reuse of the existing exposure state machine

The existing `xcredits` exposure flow already implements the important financial transitions:

```text
reserved -> read_started -> finalize_pending -> finalized
                         -> release_pending  -> released
                         -> needs_reconciliation
```

P0 must generalize and reuse this machinery as the sole authority for customer reservation, final charge, release, and reconciliation state. It must not create a parallel Credits state machine in the account-read handler.

Directly calling the current Inbox-specific implementation without refactoring is insufficient because it is coupled to `x_inbox_backfill_exposure_reservations`, applies the inbound daily cap, allows partial-resource admission, uses a fixed 30-minute deadline, and does not retain a replayable normalized response or enough request data to retry an upstream read. The implementation plan must therefore extract or extend a generic X read-exposure abstraction while preserving current Inbox behavior.

The generalized exposure layer must support:

- a source or purpose discriminator such as `x_account_profile_read` or `x_account_post_history_read`;
- strict whole-request admission for account reads, with `minimum_resources == requested_resources`;
- customer-accounting enabled or bypassed based on app mode and `x_credits_billing_v1`;
- internal cost-safety enforcement that remains active when customer accounting is bypassed;
- a source-specific reconciliation deadline, 24 hours for these account reads;
- Workspace, social account, and `external_user_id` ownership;
- one shared recovery worker and one authoritative deadline per exposure;
- source-specific upstream-resolution callbacks without duplicating financial settlement logic.

Existing Inbox exposure behavior and its 30-minute deadline must remain unchanged unless a separately reviewed migration intentionally changes them.

### Idempotency receipt

P0 adds only the missing request and replay layer, conceptually `x_read_receipts`, with:

- public operation ID;
- Workspace, social account, and `external_user_id` ownership;
- endpoint and canonical request fingerprint;
- a keyed hash of the caller's idempotency key rather than the raw key;
- generalized exposure or usage-event reference;
- requested resource limit and operation key;
- upstream execution state and attempt metadata;
- the exact sanitized request data needed for bounded retry, with provider pagination state encrypted at rest;
- normalized response JSON only after success;
- sanitized failure class;
- created, updated, completed, and expires-at timestamps.

The receipt owns upstream execution and replay states such as `executing`, `outcome_unknown`, `succeeded`, and `failed`. It does not own or duplicate authoritative financial states, Credits counters, or a second reconciliation deadline.

The exact migration and query layout belongs in the implementation plan, but the following invariants are mandatory:

- One Workspace and idempotency-key hash maps to at most one logical read receipt during the receipt lifetime.
- The receipt and generalized X Credits exposure are linkable for audit.
- Workspace, account, and Managed User ownership are persisted before the upstream call.
- State transitions use conditional updates or row locking so only one caller owns execution or reconciliation.
- Exposure admission and receipt admission are committed before X is called.
- Final settlement and terminal receipt state cannot diverge silently; failed finalization remains recoverable through the shared exposure recovery path.
- A bypassed operation still has a receipt, but its customer reservation, charge, and release values remain zero.

The normalized response is retained for at most 24 hours solely for idempotent replay. It is not reused across different idempotency keys or callers as a business cache. Expired receipts and response bodies are deleted by a recurring cleanup worker.

Workspace deletion, Managed User deletion, or permanent account deletion deletes related receipts and response bodies immediately through database ownership rules or explicit cleanup. Account disconnect blocks new reads but does not need to destroy an unexpired receipt that the same authorized scope already created; permanent deletion does.

## Failure and HTTP contract

Errors use UniPost's existing error envelope and top-level `request_id`:

```json
{
  "error": {
    "code": "RATE_LIMITED",
    "message": "X temporarily limited post-history retrieval.",
    "is_retriable": true,
    "details": {
      "retry_after_seconds": 900
    }
  },
  "request_id": "req_xxx"
}
```

Stable P0 cases:

| HTTP | Code | Meaning |
|---:|---|---|
| 400 | `VALIDATION_ERROR` | Missing or invalid parameter, including out-of-range `limit` |
| 400 | `IDEMPOTENCY_KEY_REQUIRED` | A billable or cost-bearing read omitted `Idempotency-Key` |
| 400 | `INVALID_CURSOR` | Cursor is invalid, expired, modified, or bound to a different request scope |
| 400 | `WRONG_PLATFORM` | The authorized account is not an X account |
| 401 | `UNAUTHORIZED` | API key is missing or invalid |
| 402 | `INSUFFICIENT_X_CREDITS` | Maximum request charge cannot be reserved |
| 403 | `ACCOUNT_ACCESS_DENIED` | Account exists in the Workspace but belongs to another Managed User |
| 403 | `FEATURE_NOT_AVAILABLE` | A Credits balance or ledger endpoint is hidden while `x_credits_billing_v1` is disabled |
| 404 | `ACCOUNT_NOT_FOUND` | Account is not in the authenticated Workspace or does not exist |
| 409 | `IDEMPOTENCY_CONFLICT` | The key was already used for a different request fingerprint |
| 409 | `READ_IN_PROGRESS` | The same logical idempotent read is currently executing |
| 409 | `READ_SETTLEMENT_PENDING` | The same logical read has an ambiguous outcome under reconciliation |
| 422 | `ACCOUNT_REAUTHORIZATION_REQUIRED` | Account is disconnected, token refresh failed permanently, or required scope is absent |
| 429 | `RATE_LIMITED` | UniPost or X temporarily rate-limited the read |
| 502 | `X_UPSTREAM_ERROR` | X returned a definite non-rate-limit failure or invalid response |
| 503 | `UPSTREAM_UNAVAILABLE` | X cannot currently serve the request and the outcome is known to be non-billable |

Errors include `is_retriable`, a safe `message`, and a `retry_policy` or retry timing where applicable. They never include raw provider bodies or credentials.

## Page-level success semantics

Because one public request fetches no more than one upstream page, P0 does not expose snapshot-style partial success:

- A page succeeds and is returned as a whole.
- A page definitely fails and returns no page data.
- An ambiguous page remains pending until reconciliation produces a replayable response or releases the reservation.
- Pages successfully returned before a later page fails remain valid in the caller's system.

This replaces the original `is_partial` snapshot requirement. The caller resumes from the last successful opaque cursor with the same idempotency key for the failed logical page.

## Rate limits and concurrency

Customer Credits availability, when enabled, does not bypass operational limits. P0 applies bounded per-Workspace and per-account read concurrency and request-rate controls independently of the customer monthly allowance and keeps internal cost-safety controls active when customer accounting is bypassed.

- Only one execution or reconciliation owner may run for a logical idempotent request.
- X `429` responses are normalized to `RATE_LIMITED` and preserve the provider's retry timing when safe.
- UniPost rate limits also use `RATE_LIMITED` with a UniPost-derived retry time.
- A rate-limit rejection before an X resource response releases the request reservation.
- Request timeouts, response-size bounds, and sanitized error-body limits follow existing X HTTP-client safety patterns.

Exact numerical UniPost concurrency and rate limits are operational configuration, not OpenAPI guarantees, and will be selected in the implementation plan.

## Observability and audit

Every cost-bearing upstream request, including customer-accounting bypasses, emits structured fields sufficient to follow the operation without logging content:

- request ID and operation ID;
- Workspace, social account, and Managed User internal identifiers;
- endpoint and request-fingerprint hash;
- X operation and catalog version;
- requested limit, scanned count, and returned count;
- estimated, reserved, charged, and released Credits;
- state transitions, retry count, reconciliation deadline, and terminal reason;
- upstream status class and latency;
- cursor issuance and validation outcome without logging its plaintext.

Logs must not contain OAuth tokens, refresh tokens, raw idempotency keys, post text, profile bio, raw X response bodies, or upstream pagination tokens.

Metrics include:

- requests and latency by endpoint/outcome;
- X responses by status class;
- reservation denials;
- reserved, finalized, and released Credits by operation;
- idempotent replays and conflicts;
- ambiguous reads, reconciliation age, 24-hour forced releases, and internal accounting anomalies;
- scanned-to-returned post ratios;
- account and Workspace rate-limit rejections.

An alert fires when ambiguous reads approach their 24-hour deadline, forced releases occur, reconciliation backlog exceeds its threshold, or Credits settlement fails repeatedly.

## Security and privacy

- Provider access and refresh tokens remain encrypted at rest and are never returned.
- Cursor payloads are authenticated and encrypted with a rotating server-side key.
- Idempotency keys are stored only as keyed hashes suitable for equality checks.
- Raw X response bodies are processed in memory and not logged.
- The 24-hour normalized response receipt is the only post-content persistence introduced by P0.
- Billing ledger responses exclude content and provider identifiers not required for audit.
- Cross-Workspace account probing returns 404.
- The X user ID always comes from the authorized account record, preventing arbitrary public-user reads.
- All provider-returned URLs and text are treated as untrusted response data and JSON encoded normally.

## Compatibility and rollout

The new profile, posts, and Credits-event routes are additive. The capability response and existing X Credits snapshot are extended additively. `/capabilities` keeps its current no-selector behavior, and only validates Managed User ownership when the optional selector is supplied.

The existing `x_credits_billing_v1` flag is mandatory at every new account-read accounting call site. No new feature flag is introduced.

For an ordinary Workspace while the flag remains off, a UniPost-managed X account read succeeds with customer accounting bypassed and zero customer Credits. When the flag is enabled for the Workspace, the same request performs preauthorization, can return 402, and settles actual usage. Workspace-owned X app reads bypass customer accounting in both cases. A Workspace for which the flag cannot be evaluated fails closed before X is called.

Implementation and rollout must update `docs/feature-flags-unleash.md` to list the new profile and post-history accounting call sites, the bypass behavior, the internal safety behavior that remains active, and the rollback effect. Turning the flag off is the customer-accounting rollback: new reads continue but stop reserving or charging customer X Credits.

OpenAPI documentation must explain:

- the optional selector on `/capabilities` and required `external_user_id` boundary on `/profile` and `/posts`;
- server-side API-key handling;
- `limit` as scanned-resource capacity rather than promised result count;
- per-page idempotency;
- `x_credits_billing_v1`, app-mode bypass, Credits reservation, and settlement;
- caller-side pagination and `external_post_id` deduplication;
- why thread position and sample sufficiency are not returned.

## Test strategy

### Authorization tests

Create two Workspaces, with two Managed Users inside the first Workspace:

- Account reads succeed only for the API-key Workspace and exact `external_user_id`.
- Cross-Workspace account IDs return 404.
- Same-Workspace cross-user account IDs return `403 ACCOUNT_ACCESS_DENIED` without invoking X.
- `/capabilities` without `external_user_id` preserves the current Workspace-only success path.
- `/capabilities` with a mismatched `external_user_id` returns 403.
- `/profile` and `/posts` with a missing `external_user_id`, null-owned accounts, and non-X accounts fail before invoking X.
- Disconnect, missing scope, expired access with successful refresh, and permanently failed refresh follow the documented capability and error behavior.

### Profile and normalization tests

- Core profile fields map correctly and optional metrics can be absent.
- Original, reply-to-other, self-reply, Quote Post, and Repost fixtures produce the documented flags and content types.
- `exclude_replies_to_others` preserves self-replies and removes only replies to others.
- `exclude_reposts` removes Reposts.
- Quoted content does not replace or contaminate the connected user's own `text`.
- Missing media or metrics does not drop a valid post.
- Empty timelines return `200` with an empty array.

### Cursor tests

- The first page returns a continuation cursor only when X reports another page.
- Every continuation response exposes a seven-day `cursor_expires_at`.
- Continuation preserves the exact account, Managed User, time range, and filters.
- Modified, expired, cross-account, cross-user, and filter-mismatched cursors fail closed.
- A retriable delay that would outlive the current cursor returns a refreshed `retry_cursor` without exposing the X token.
- The upstream pagination token never appears in logs or response JSON.

### Credits and idempotency tests

- With `x_credits_billing_v1` enabled on a UniPost-managed account, Profile preauthorizes one `user.read` unit.
- With the flag enabled on a UniPost-managed account, Posts preauthorize `limit * post.read` weight.
- With the flag enabled, insufficient Credits returns 402 and the fake X client observes zero calls.
- With the flag disabled, the same UniPost-managed reads succeed without customer reservation or charge and return `status=bypassed`, `bypass_reason=feature_disabled`.
- Workspace-owned X app reads bypass customer accounting whether the flag is enabled or disabled and return `bypass_reason=customer_x_app`.
- A feature-flag evaluation failure performs zero X calls.
- A 100-resource X page filtered to 20 posts charges for 100 resources.
- An anomalous response containing more than `limit` resources never charges above the reservation and triggers telemetry.
- A shorter final X page releases unused reservation.
- A definite pre-resource failure releases the reservation.
- Concurrent requests with the same key and fingerprint execute one X call and create one charge.
- A completed same-key replay returns identical normalized data and Credits receipt without another X call.
- The same key with a different fingerprint returns 409.
- An ambiguous result remains reserved, exposes pending status, and is owned by one reconciler.
- Successful reconciliation finalizes actual usage and makes the result replayable.
- An unresolved 24-hour operation releases the reservation and cannot later back-charge the customer.
- Workspace snapshot totals equal the sum of finalized and pending ledger events.
- Finalized, pending, and effective snapshot fields come from status-aggregated ledger data and preserve the existing meaning of `monthly_used`.
- Ledger filters cannot escape the authenticated Workspace and never expose response content or raw idempotency keys.

### Failure tests

- X 429 maps to `RATE_LIMITED`, preserves safe retry timing, and does not charge when no resource body was returned.
- Definite X failures map to stable sanitized errors.
- Timeouts and truncated bodies enter ambiguous reconciliation rather than being silently released or fully charged.
- Credits-finalization failure leaves durable recoverable state and alerts rather than reporting a false terminal success.
- After a successful reservation followed by a finalization failure, shared reconciliation eventually applies the delta so `weighted_units_used` converges to the final charge and does not indefinitely consume the customer's available balance.
- Reconciliation leases prevent two replicas from retrying the same operation concurrently.

### Deployed acceptance

On the exact pull-request head SHA in the isolated Railway PR Environment and Vercel Preview:

1. Connect or use a dedicated X acceptance account through Hosted Connect.
2. Confirm capability discovery with the correct and incorrect Managed User.
3. Read the live profile and verify username, display name, bio, avatar, and retrieval time.
4. Read at least two post pages and verify cursor continuity, filters, thread primitives, and no token exposure.
5. With `x_credits_billing_v1` enabled for the acceptance Workspace, verify same-key replay creates no second X call or Credits charge using the ledger.
6. Exercise an insufficient-Credits fixture or controlled allowance and prove zero upstream calls.
7. Disable the flag and prove the same reads remain available with zero customer Credits and an explicit feature-disabled bypass receipt.
8. Verify a Workspace-owned X app remains bypassed in both flag states.
9. Verify the Workspace X Credits snapshot and event ledger match the completed operations while the flag is enabled.
10. Inspect sanitized logs for request IDs and operation IDs without profile text, post text, tokens, or raw cursors.

All required local tests, Preview Acceptance, deployed regression, and browser acceptance must pass on the exact PR head SHA before merge to `dev`. After merge, the official development API must be revalidated before the task is complete.

## Revised acceptance criteria

P0 is accepted when:

1. A correctly scoped caller can retrieve the connected X profile's required fields.
2. A correctly scoped caller can retrieve one live post-history page of 5 through 100 scanned resources and continue through opaque cursors.
3. Reply-to-other and Repost filters behave as documented while self-reply threads remain available.
4. Posts expose verifiable conversation and relationship primitives without unverified positions or totals.
5. Zero matching posts returns a valid empty response; UniPost does not judge sample sufficiency.
6. Prior successful pages remain usable when a later page is rate-limited or fails.
7. Missing scopes or unusable authorization produces a stable reauthorization error and capability state.
8. Cross-Workspace and cross-Managed User reads fail closed before X is called.
9. When `x_credits_billing_v1` is enabled for a UniPost-managed account, insufficient Credits denies the complete page before X is called.
10. When customer accounting is enabled, successful reads settle against the admitted upstream resources scanned, never above the reservation.
11. Idempotent replay never causes a second charge, while mismatched reuse is rejected.
12. Ambiguous outcomes are reconciled or released within 24 hours without late customer back-charge.
13. Workspace Credits balance and transaction events explain every operation.
14. Responses, logs, ledger APIs, and metrics never expose X credentials or retained post content beyond the authorized live response and 24-hour idempotency receipt.
15. With the Credits flag disabled or a Workspace-owned X app, live reads return an explicit zero-charge bypass without weakening idempotency, authorization, rate limits, or internal cost safety.

## Deferred follow-ups

- Optional asynchronous batch snapshots for consumers that need long-running exports.
- Snapshot retention, deletion, cache reuse, and completion webhooks.
- A separately billed full-conversation endpoint that can verify thread ordering and totals.
- Longer-lived customer-controlled content storage, if a future product explicitly requires it.
- Provider-cost reconciliation improvements if X later exposes authoritative per-request billing receipts.
