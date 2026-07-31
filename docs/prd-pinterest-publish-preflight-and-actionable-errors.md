# PRD — Pinterest Publishing Hardening

**Status:** Approved for implementation

**Owner:** Publishing / API Platform / Dashboard

**Created:** 2026-07-30

**Target:** Pinterest publishing reliability hardening

**Supporting dependency:** Secure Remote Media Fetcher

---

## 1. Summary

UniPost currently accepts a syntactically valid Pinterest `board_id` and sends it directly to Pinterest without confirming that the board exists, belongs to the connected account, or is visible in the active Pinterest environment. It also passes externally hosted media to Pinterest without first proving that Pinterest can fetch a valid image or video.

When Pinterest rejects one of these resources, UniPost often stores a generic `platform_error` and tells the customer to contact support. The same response contains enough provider evidence to give a specific action, but the current taxonomy does not understand Pinterest's error vocabulary.

The primary product scope of this PRD is Pinterest-specific hardening that:

1. validates the destination board immediately before provider dispatch;
2. converts known Pinterest failures into stable, actionable UniPost errors;
3. prevents Dashboard users from submitting stale or arbitrary board identifiers;
4. preserves structured provider evidence for API clients and operators;
5. keeps permanent resource failures out of the automatic retry queue;
6. isolates Pinterest Sandbox and Production destination identities.

The incident also exposed a dead customer-supplied media URL. Safely downloading and staging arbitrary external media is not implemented as Pinterest-specific downloader logic. It is a separate platform dependency, **Secure Remote Media Fetcher**, with its own interface, security review, tests, and acceptance gate. Pinterest is its first consumer.

The two workstreams are related but independently deliverable:

- **Workstream A — Pinterest Publishing Hardening:** errors, board preflight, environment isolation, Dashboard behavior, retries, and Pinterest observability.
- **Workstream B — Secure Remote Media Fetcher:** safe external URL retrieval and controlled storage integration, followed by Pinterest media-preflight adoption.

Workstream A can ship without Workstream B. Workstream B must not block the Pinterest board/error fixes, and it must not ship without its security gate.

---

## 2. Incident that motivates this PRD

On 2026-07-30, Pinterest rejected an API-originated publish attempt:

```text
POST https://api.pinterest.com/v5/pins
HTTP 403
{"code":29,"message":"You are not permitted to access that resource."}
```

The request used:

```json
{
  "board_id": "1131529543818288706",
  "description": "Hello from SocialHub 🚀",
  "media_source": {
    "source_type": "image_url",
    "url": "https://images.unsplash.com/photo-1517336714739-489689fd1ca8?w=1200"
  },
  "title": ""
}
```

Production-safe, read-only diagnostics established the following:

- the stored OAuth token successfully authenticated the expected Pinterest Business account;
- `GET /v5/user_account` returned `200`;
- `GET /v5/boards` returned `200` with an empty board list and `board_count=0`;
- `GET /v5/boards/1131529543818288706` returned `404`, Pinterest code `40`, `Board not found`;
- `GET /v5/pins` returned `200`;
- the submitted Unsplash URL returned `404` when fetched directly;
- UniPost classified the publish failure as `platform_error`, `is_retriable=false`, `next_action=contact_support`;
- the delivery job had `max_attempts=5` but correctly stopped after one attempt because the failure was non-retriable.

The evidence rules out an expired token as the primary cause. The destination board was not available to the connected account. The media URL was an additional independent defect that would have remained after fixing the board.

---

## 3. Problem

### 3.1 Board validation is only syntactic

The current request validator checks that a Pinterest board ID:

- is present; and
- contains only digits.

It does not check that the board:

- exists;
- belongs to or is writable by the connected Pinterest account;
- is still available after being selected;
- came from the same Pinterest Production or Sandbox environment used for publishing.

An arbitrary, stale, deleted, cross-account, or cross-environment numeric ID therefore reaches `POST /v5/pins`.

### 3.2 Remote media can be dead before dispatch

The Pinterest adapter stages URLs only when they look ephemeral, such as presigned object-storage URLs or known temporary-file hosts. A normal-looking third-party URL is passed through unchanged even when it returns `404`, redirects to non-media content, or is inaccessible to Pinterest.

### 3.3 Known Pinterest failures become generic support errors

The failure taxonomy recognizes generic words such as `permission`, but Pinterest's code `29` message says `not permitted`. The classifier extracts the numeric provider code but leaves the public classification as generic `platform_error`.

As a result:

- `next_action` becomes `contact_support` instead of `select_valid_target`;
- `error_temporality` remains `unknown` instead of `permanent`;
- `provider_error` is empty because Pinterest has no structured provider-error extractor;
- Dashboard and API clients cannot distinguish an invalid board from an internal UniPost failure.

### 3.4 Pinterest 403 responses are over-associated with reconnects

The board handlers currently treat any Pinterest `403` as an authentication error. A `403` may instead mean that a valid token cannot access a specific board. Reconnecting the same account does not make a foreign, deleted, or cross-environment board valid.

### 3.5 Dashboard state is not authoritative

Dashboard board selection improves the common path, but a selected board may be deleted, become inaccessible, or come from stale component state before a scheduled post dispatches. The worker must remain the final authority even when the UI originally supplied the ID.

---

## 4. Goals

### 4.1 Workstream A — Pinterest Publishing Hardening

1. Reject unavailable or unwritable Pinterest boards before `POST /v5/pins`.
2. Ensure known Pinterest provider responses produce stable, actionable UniPost error fields.
3. Preserve correct retry behavior: permanent resource failures must not retry automatically.
4. Make Dashboard board selection safe and understandable, including the zero-board state.
5. Make API usage deterministic: list or create a board, submit its returned ID, and receive a specific error if it is no longer valid.
6. Preserve backward compatibility for existing post request and result shapes.
7. Add sufficient structured telemetry to distinguish board, token, rate-limit, and unexpected provider failures.
8. Prevent Sandbox board identifiers from being dispatched in Production.

### 4.2 Workstream B — Secure Remote Media Fetcher

1. Provide one backend-owned interface for safely retrieving an arbitrary customer-supplied `http` or `https` media URL.
2. Reject SSRF, DNS rebinding, redirect, type-confusion, timeout, and oversized-response attacks before storage.
3. Stage verified bytes on an existing UniPost-controlled public publishing URL.
4. Integrate Pinterest media preflight with that interface without embedding network-security logic in the Pinterest adapter.
5. Reject invalid remote media before Pinterest attempts to fetch it.

---

## 5. Non-goals

- Do not build a generic preflight framework for every social platform.
- Do not turn Pinterest board preflight or Pinterest provider-error mapping into a generalized cross-platform framework.
- Do not put safe-fetch networking policy inside the Pinterest adapter.
- Do not change Pinterest OAuth scopes or the OAuth authorization flow.
- Do not require users to reconnect when the current token is valid.
- Do not add Pinterest board editing, deletion, bulk sync, sections, or board analytics.
- Do not change Pinterest Pin types or add carousel, Product Pin, Collection, or shopping support.
- Do not change video processing or asynchronous video status behavior.
- Do not automatically retry invalid boards or invalid media.
- Do not expose raw access tokens, authorization headers, signed URLs, or unsafe provider payloads.
- Do not add a feature flag. This API and Dashboard hardening should ship through the normal Preview Acceptance flow.

---

## 6. Product principles

### 6.1 Validate at the last responsible moment

Syntactic validation remains at request admission. Authoritative resource validation happens in the delivery worker immediately before provider dispatch, after token refresh and before media staging or `POST /pins`.

This minimizes the time-of-check/time-of-use window and also protects scheduled posts whose destination changed after creation.

### 6.2 Prefer controlled media delivery through Workstream B

Pinterest fetches `image_url` and `video_url` from its own network. UniPost should give Pinterest a URL whose availability, type, and lifetime UniPost controls.

### 6.3 Return actions, not provider prose

Customers should branch on stable UniPost fields. Pinterest messages remain diagnostic evidence, not the primary product contract.

### 6.4 Do not confuse authorization with resource ownership

A valid token can be unable to access one board. Token validation, required-scope validation, and target-resource validation are separate decisions.

### 6.5 Keep permanent failures terminal

A deleted board or dead media URL will not heal through repeated delivery attempts. The customer must change the request or destination.

---

## 7. Chosen approach

Deliver two bounded workstreams through explicit interfaces.

Alternatives considered:

1. **One Pinterest-only implementation:** keeps ownership in one adapter but mixes provider semantics with security-sensitive network fetching and makes the Pinterest fix unnecessarily large.
2. **One generalized publishing-preflight framework:** maximizes reuse but expands scope across unrelated platforms before a second consumer exists.
3. **Pinterest hardening plus a narrow safe-fetch dependency:** keeps Pinterest behavior provider-specific while isolating reusable network-security policy behind one backend interface.

Option 3 is selected.

### 7.1 Workstream A ownership

The Pinterest adapter owns Pinterest API calls, board semantics, provider-error normalization, and the decision to proceed to `POST /v5/pins`. Dashboard owns Pinterest board selection and customer guidance. Generic social-post handlers remain platform-agnostic.

### 7.2 Workstream B ownership

The Secure Remote Media Fetcher owns URL parsing, DNS/IP validation, connection policy, redirect handling, bounded streaming, and detected media metadata. Existing storage owns persisted objects and lifecycle cleanup. The Pinterest adapter supplies a URL and consumes either verified staged media or a typed failure; it does not implement those controls itself.

The current generic remote-upload implementation is not safe for arbitrary untrusted URLs: it follows redirects by default, does not reject private or reserved network destinations, does not defend against DNS rebinding, and reads the response without a streaming byte ceiling. It must not be reused for external `media_urls` until Workstream B passes the section 15 security gate.

### 7.3 Delivery independence

Workstream A ships first and retains existing media behavior until Workstream B is accepted. Workstream B then adds deterministic external-media validation and staging for Pinterest. The final PRD outcome requires both workstreams, but failure or delay in Workstream B must not hold back accepted Pinterest error and destination fixes.

---

## 8. Required publish flow

```text
request admission
  → local Pinterest syntax validation
  → enqueue delivery job
  → load account and decrypt/refresh token
  → Pinterest account/token check when required
  → authoritative Pinterest board preflight            [Workstream A]
  → remote media validation and staging, when enabled  [Workstream B]
  → POST /v5/pins
  → normalize Pinterest provider response              [Workstream A]
  → persist result, failure, retry decision, and telemetry
```

### 8.1 Request admission

Keep the existing local checks:

- exactly one Pinterest media item;
- `board_id` is required;
- `board_id` contains only digits;
- title, description, link, media count, and media type satisfy Pinterest capability rules.

Request admission must not call Pinterest. The public validation endpoint remains deterministic and does not gain provider-network dependency.

### 8.2 Token handling

Before board preflight, use the same token refresh path used by publishing.

- A successful Pinterest user or board read proves that the token is authenticated for that operation.
- `401` or Pinterest code `2` maps to `auth_token_invalid` and `reconnect_account`.
- A `403` must not automatically imply token invalidity.
- Missing required scopes maps to `missing_permission` and `reconnect_or_update_permissions` only when the provider evidence refers to account-level scope or when even the connected account's own board collection is forbidden.

### 8.3 Board preflight

Immediately before media staging, fetch the selected board using the active access token. Direct `GET /v5/boards/{board_id}` is the primary path; a board-list lookup is a bounded fallback, not the default request.

The preflight must prove:

1. the board exists in the active Pinterest environment;
2. the board is visible to the operation user represented by the token;
3. Pinterest reports a documented ownership or writability signal that proves Pin creation is allowed, or the board is present in the operation user's authoritative owned-board list;
4. the selected board ID exactly matches the submitted ID.

Visibility alone is not proof of writability. If direct board lookup succeeds but its response is ambiguous about ownership or write access, the adapter must compare the submitted ID against a cached, bounded traversal of the operation user's owned-board collection from `GET /v5/boards`. The implementation must not treat numeric shape, a successful direct read, or visibility of a shared board as proof of write access.

The owned-board fallback must obey all of the following limits:

- cache by social account ID and Pinterest environment;
- use a short TTL of 60 seconds;
- invalidate on board creation, account reconnect, account disconnect, or Pinterest environment change;
- request the maximum supported page size, currently `250`;
- stop after 8 pages or 2,000 distinct boards, whichever comes first;
- stop immediately when the submitted board is found;
- detect repeated bookmarks and duplicate-page loops;
- if ownership remains ambiguous or the cap is reached before the collection is exhausted, fail with `temporary_platform_error` and `retry_later` rather than silently accepting or permanently rejecting the board.

The 8-page/2,000-board ceiling is an explicit UniPost worker safety bound, not an assumption that provider pagination is finite. If legitimate supported accounts require a higher ceiling, the product limit, latency budget, implementation, and tests must be revised together before raising it.

Board preflight outcomes:

| Provider evidence | UniPost `error_code` | `platform_error_code` | `next_action` | Retriable |
|---|---|---|---|---|
| Board lookup `404`, code `40` | `target_not_found` | `40` | `select_valid_target` | No |
| Board lookup/list `403`, code `29`, while token otherwise works | `target_not_found` | `29` | `select_valid_target` | No |
| Account/board collection forbidden due to scope | `missing_permission` | provider code | `reconnect_or_update_permissions` | No |
| Token `401`, code `2` | `auth_token_invalid` | `2` | `reconnect_account` | No |
| Pinterest `429` | `rate_limit` | provider code if present | `wait_and_retry` | Yes |
| Pinterest timeout or 5xx during preflight | `temporary_platform_error` | provider code if present | `retry_later` | Yes |
| Ownership is ambiguous or board-list traversal exceeds its safety cap | `temporary_platform_error` | provider code if present | `retry_later` | Yes |

An invalid-board preflight must not call `POST /v5/pins`.

### 8.4 Environment isolation

Board responses already expose `sandbox_mode`. This environment identity must remain attached to the board-selection state in Dashboard and to the internal post metadata used by scheduled delivery.

- Production publishing accepts only boards fetched or created through the Production Pinterest base URL.
- Sandbox publishing accepts only boards fetched or created through the Sandbox base URL.
- Dashboard clears the selected board when account or environment identity changes.
- The worker remains authoritative and rejects cross-environment IDs even when a client bypasses Dashboard.
- At request creation, the server stores an internal `pinterest_environment` marker with the selected destination. This is internal metadata, not a new public request field.
- Before a runtime changes from Pinterest Sandbox to Production, operators must audit queued and scheduled Pinterest posts carrying the Sandbox marker. Those posts must be held and surfaced for board reselection or rejected with `select_valid_target`; board IDs must never be translated across environments.
- Legacy queued rows without an environment marker receive live board preflight in the active environment and are never assumed to match it.

No new public request field is required. The server knows the active Pinterest environment from runtime configuration and records that identity when accepting the post.

### 8.5 Media validation and staging

This section is the Pinterest integration contract for Workstream B. It is not implemented as ad hoc fetching inside Workstream A. Until Workstream B passes its security gate and is integrated, Workstream A retains existing media behavior and does not claim deterministic rejection of dead external URLs.

After Workstream B is enabled for Pinterest, run media work only after board preflight succeeds.

For `media_ids` already backed by UniPost-controlled storage:

- use the existing media metadata and public publishing URL;
- enforce Pinterest media type and size limits;
- do not create a duplicate staged object unless the existing URL is unsuitable for provider fetch.

For external `media_urls`, use a new hardened safe-fetch path. The current `UploadFromURL` behavior is not approved for arbitrary untrusted URLs and must not be extended for this flow until section 15's security gate passes.

After the security gate passes:

1. parse and fetch through the approved server-side safe-fetch abstraction;
2. reject initial or redirected private, loopback, link-local, metadata-service, or otherwise prohibited network destinations;
3. require a successful media response rather than relying on `HEAD` alone;
4. verify the detected bytes and MIME type agree with a supported Pinterest image or video type;
5. enforce maximum download size and existing Pinterest media limits;
6. stage the verified bytes to the existing UniPost public R2 publishing path;
7. send the staged UniPost URL to Pinterest.

The staged object must reuse the existing publishing-object retention and cleanup policy. This PRD does not create a Pinterest-specific retention period.

Media failure outcomes:

| Failure | `error_code` | `next_action` | Retriable |
|---|---|---|---|
| Source URL returns 4xx or no media body | `media_error` | `fix_media` | No |
| Unsupported or mismatched media type | `media_error` | `fix_media` | No |
| Media exceeds supported size/format limits | `media_error` | `fix_media` | No |
| Temporary source timeout or 5xx | `temporary_platform_error` | `retry_later` | Yes |
| R2 staging temporarily fails | `temporary_platform_error` | `retry_later` | Yes |

### 8.6 Create Pin

Only call `POST /v5/pins` after board and media preflight pass.

If Pinterest returns code `29` or `40` despite successful preflight, classify it using the same destination-target contract. This covers access changes between preflight and write. Preserve the provider response and mark the result permanent; do not retry the unchanged request.

---

## 9. Public API contract

### 9.1 Request compatibility

The existing post request remains unchanged:

```json
{
  "platform_posts": [
    {
      "account_id": "sa_pinterest_1",
      "caption": "Five workspace ideas",
      "media_urls": ["https://example.com/workspace.jpg"],
      "platform_options": {
        "board_id": "123456789012345678"
      }
    }
  ]
}
```

No new required fields are introduced.

### 9.2 Board discovery workflow

API clients should use one of the existing account-scoped endpoints before publishing:

```text
GET  /v1/accounts/{id}/pinterest/boards
POST /v1/accounts/{id}/pinterest/boards
```

Profile-scoped equivalents remain supported:

```text
GET  /v1/profiles/{profileID}/accounts/{accountID}/pinterest/boards
POST /v1/profiles/{profileID}/accounts/{accountID}/pinterest/boards
```

The list response remains the authoritative source for selectable board IDs at request creation time. Successful listing does not replace worker preflight.

### 9.3 Failed result example: invalid board

```json
{
  "status": "failed",
  "error_message": "The selected Pinterest board is unavailable for this connected account.",
  "error_code": "target_not_found",
  "failure_stage": "destination_preflight",
  "platform_error_code": "40",
  "is_retriable": false,
  "next_action": "select_valid_target",
  "error_source": "platform",
  "error_temporality": "permanent",
  "provider_error": {
    "provider": "pinterest",
    "http_status": 404,
    "code": "40",
    "reason": "board_not_found"
  }
}
```

### 9.4 Failed result example: inaccessible board

```json
{
  "status": "failed",
  "error_message": "The selected Pinterest board does not belong to or is not accessible by this connected account.",
  "error_code": "target_not_found",
  "failure_stage": "destination_preflight",
  "platform_error_code": "29",
  "is_retriable": false,
  "next_action": "select_valid_target",
  "error_source": "platform",
  "error_temporality": "permanent",
  "provider_error": {
    "provider": "pinterest",
    "http_status": 403,
    "code": "29",
    "reason": "board_not_accessible"
  }
}
```

### 9.5 Failed result example: invalid media (Workstream B)

```json
{
  "status": "failed",
  "error_message": "Pinterest media could not be downloaded from the supplied URL.",
  "error_code": "media_error",
  "failure_stage": "media_preflight",
  "is_retriable": false,
  "next_action": "fix_media",
  "error_source": "unipost",
  "error_temporality": "permanent"
}
```

For this v1 contract, `error_source=unipost` means UniPost detected and rejected the request before Pinterest dispatch; it does not assign fault or blame to UniPost. The existing enum reserves customer-originated attribution for a future contract change, so this Pinterest-specific PRD does not introduce `customer_request`. Internal events must additionally record `failure_reason=customer_media_unreachable` (or the corresponding normalized media reason) so product analytics and reliability reporting can distinguish customer input from UniPost infrastructure failures.

Existing clients that ignore the newer structured fields continue to receive the established result object and HTTP behavior.

---

## 10. Pinterest provider error normalization

Add Pinterest as a first-class provider in structured error extraction.

The normalized provider object may contain:

```json
{
  "provider": "pinterest",
  "http_status": 403,
  "code": "29",
  "reason": "board_not_accessible",
  "is_transient": false
}
```

Requirements:

- parse the provider response as structured JSON at the adapter boundary;
- preserve HTTP status and Pinterest numeric code;
- derive a stable UniPost reason from request context and preflight stage;
- do not rely only on English provider message fragments;
- keep legacy string parsing as a fallback for older stored failures;
- redact tokens, authorization headers, signed query parameters, and unsafe URLs from public responses and logs;
- keep the full sanitized provider body available only in existing internal debug surfaces.

Known mapping:

| Context | Pinterest evidence | Normalized reason |
|---|---|---|
| User/account request | 401, code 2 | `token_invalid` |
| Board preflight | 404, code 40 | `board_not_found` |
| Board preflight or create Pin | 403, code 29 | `board_not_accessible` |
| Provider request | 429 | `rate_limited` |
| Provider request | timeout or 5xx | `provider_temporary_failure` |

Unknown Pinterest errors remain honest:

```text
error_code=platform_error
error_temporality=unknown
next_action=contact_support
```

Only unrecognized errors should reach that fallback.

---

## 11. Dashboard requirements

### 11.1 Board selection

- Load boards from the profile-scoped Pinterest boards endpoint whenever a Pinterest account becomes selected.
- Clear the selected board when the account changes, disconnects, reconnects, or the returned environment identity changes.
- Render board choices from returned IDs; do not expose a free-form board-ID input.
- Refresh the board list before final submission when the current list is no longer fresh in component state.
- Keep the worker preflight as the final authority.

### 11.2 Zero-board state

When the board list is empty:

- disable Pinterest submission for that account;
- explain that Pinterest requires a destination board;
- expose the existing Create Board action;
- refresh and select the newly created board after successful creation;
- do not tell the user to reconnect unless the board endpoint returns a true token or scope error.

Recommended copy:

```text
This Pinterest account has no available boards. Create a board before publishing a Pin.
```

### 11.3 Failure presentation

For `target_not_found` with Pinterest code `29` or `40`:

```text
The selected Pinterest board is no longer available for this account. Choose another board and publish again.
```

For `media_error` from `media_preflight`:

```text
Pinterest could not use this media. Replace it with a publicly available supported image or video.
```

For `auth_token_invalid`:

```text
Pinterest rejected this connection. Reconnect the account, then try again.
```

The Dashboard must use structured fields, not parse `error_message`.

---

## 12. Retry behavior

| Failure class | Automatic retry | Manual retry with unchanged request |
|---|---|---|
| Board not found/inaccessible | No | No |
| Unsupported or missing media | No | No |
| Token invalid | No | No, reconnect first |
| Missing scope | No | No, reconnect/update permission first |
| Pinterest rate limit | Yes | Not while an automatic retry is pending |
| Pinterest timeout or 5xx | Yes | Follow existing retry policy |
| Temporary media-source or R2 failure | Yes | Follow existing retry policy |

The delivery job may retain its configured maximum attempt count. A permanent preflight failure transitions terminal after its first attempt, as it does today. The customer-facing reason and action must explain why.

---

## 13. Observability

Workstream A adds:

- `pinterest_destination_preflight_started`;
- `pinterest_destination_preflight_succeeded`;
- `pinterest_destination_preflight_failed`;
- `pinterest_create_pin_failed`.

Workstream B adds after Pinterest adoption:

- `pinterest_media_preflight_failed`;
- `pinterest_media_staged`.

Each event should include safe identifiers and dimensions:

- post ID;
- social post result ID;
- workspace ID;
- social account ID;
- sanitized board ID or a one-way fingerprint consistent with existing logging policy;
- Pinterest environment (`production` or `sandbox`);
- HTTP status;
- Pinterest error code;
- normalized reason;
- normalized failure reason, including `customer_media_unreachable` for a dead customer-supplied URL;
- failure stage;
- retriable decision;
- duration.

Never log or emit:

- access or refresh tokens;
- authorization headers;
- workspace credential secrets;
- signed media query parameters;
- full provider request bodies containing sensitive URLs.

Workstream A operational metrics:

- destination-preflight success and failure counts;
- failures by Pinterest code and normalized reason;
- create-Pin requests prevented by preflight;
- Pinterest publish success rate after preflight;
- count of known Pinterest errors that incorrectly fall back to `contact_support`.

Workstream B operational metrics:

- media-source rejection count by status/type;
- media staging success/failure and duration.

Reliability metrics must segment `failure_stage=media_preflight` by normalized failure reason. Customer-input rejections such as `customer_media_unreachable` are reported as product-quality/input metrics and excluded from the numerator of UniPost infrastructure reliability SLOs, even though the v1 public contract uses `error_source=unipost` for pre-dispatch enforcement.

---

## 14. Backward compatibility and migration

- No database migration is required solely for the public result shape because the existing failure model already stores error source, temporality, provider error, stage, provider code, retryability, and next action.
- The implementation must first prove that the existing post/platform metadata persists `pinterest_environment` through scheduling. If it does not, a narrowly scoped internal schema migration is required; silently deriving the creation environment at dispatch is not acceptable.
- Existing rows are not rewritten in bulk.
- Legacy Pinterest failure strings may be re-derived through the compatibility classifier when read through internal tools.
- Existing post requests remain valid.
- Existing board-list and board-create endpoints remain unchanged.
- Existing clients that only understand `platform_error` continue to receive a terminal failed result, but new failures use the more specific stable classification.
- Scheduled Pinterest posts already waiting in the queue receive the new worker preflight when their delivery begins.
- New posts record the internal Pinterest environment marker without changing the public request schema.
- Legacy scheduled rows without the marker are handled by live preflight; they are not backfilled with an assumed environment.

---

## 15. Supporting platform dependency — Secure Remote Media Fetcher

Workstream B is a bounded platform capability, not a Pinterest feature implementation. Its public backend interface accepts an external media URL plus an explicit byte/type policy and returns either:

- verified, bounded media content and detected metadata suitable for storage; or
- a typed failure that the calling platform adapter maps into its own product error contract.

The interface must not contain Pinterest board logic, Pinterest error codes, or Pinterest-specific customer copy. Pinterest is the first consumer; adding another consumer requires its own product requirements and adapter mapping rather than expanding this PRD.

### 15.1 Security contract

Remote media staging is a net-new security-sensitive capability. The current generic `UploadFromURL` path is not hardened for arbitrary URLs and must not be used as the implementation baseline without first extracting or replacing it with an approved safe-fetch abstraction.

The safe fetcher must:

- accept only `http` and `https`, reject URL userinfo, and reject malformed or non-web schemes;
- reject loopback, private, link-local, multicast, unspecified, reserved, and other non-public IPv4 and IPv6 destinations;
- explicitly block common cloud metadata addresses and hostnames;
- resolve the hostname before connection and reject the request if any returned A or AAAA record is prohibited;
- connect through a custom dial path that pins a validated public IP and verifies the actual peer, while preserving TLS certificate and hostname verification for the original host, rather than resolving the hostname again through the default transport;
- disable automatic redirect following and manually validate every `Location` hop;
- re-resolve and reapply the full hostname/IP policy on every redirect, including cross-host redirects;
- reject redirects to non-HTTP schemes, HTTPS-to-HTTP downgrades, redirect loops, and more than 5 hops;
- prevent DNS rebinding between validation and connection and between redirect hops;
- enforce connection, response-header, idle-read, and total-operation timeouts;
- stream the response through a hard byte ceiling and terminate immediately when the configured maximum is exceeded; unbounded `io.ReadAll` is prohibited;
- detect media type from a bounded byte sample and verify it against allowed Pinterest types rather than trusting the response header, extension, or query string;
- redact query strings, credentials, response bodies, and unsafe redirect targets from public errors and routine logs;
- clean staged objects through the existing lifecycle policy.

AppSec review and negative security tests are a release-blocking gate for arbitrary external URL staging. The feature must not be enabled in Preview, development, or later environments until this gate passes. Board preflight and Pinterest error normalization may ship independently before media staging if their own acceptance gates pass.

### 15.2 Storage and adapter boundary

- The safe fetcher validates and streams bytes; it does not decide object retention or expose a public URL.
- Existing UniPost storage persists verified bytes and applies the existing publishing-object lifecycle.
- The Pinterest adapter receives the staged public URL and selects Pinterest's `image_url` or `video_url` media-source shape.
- Typed fetch failures remain provider-neutral until the Pinterest adapter maps them to `media_error`, `temporary_platform_error`, `fix_media`, or `retry_later`.
- No caller may bypass the safe fetcher and pass arbitrary external URLs into the existing unhardened `UploadFromURL` path.

### 15.3 Delivery-control decision

This PRD does not add a feature flag or an environment-variable bypass. A fail-open preflight switch would silently restore invalid destination writes, while an unsafe media-staging switch would not substitute for the required security gate. Repository policy also requires an explicit product request before adding a feature flag.

Risk is reduced by keeping Workstream A and Workstream B independently reviewable and reversible. A feature flag may be reconsidered only through an explicit product decision with a defined owner, production default, and rollback contract.

---

## 16. Testing requirements

### 16.1 Workstream A — error-normalization unit tests

- numeric board syntax remains accepted locally;
- non-numeric and missing board IDs remain validation errors;
- Pinterest code `2` maps to token invalid;
- Pinterest code `29` in board context maps to board inaccessible;
- Pinterest code `40` maps to board not found;
- Pinterest `429` maps to rate limit and retriable;
- Pinterest 5xx and timeouts map to temporary platform error;
- the word `permitted` no longer falls through to generic `platform_error` in known Pinterest context;
- structured `provider_error` includes provider, status, code, and normalized reason;
- board resource `403` does not automatically mark an account reconnect-required.

### 16.2 Workstream A — Pinterest adapter and handler tests

- valid board preflight proceeds to create Pin;
- direct board lookup is used before board-list fallback;
- a visible board with no documented ownership/writability proof does not proceed on visibility alone;
- board `404/code 40` prevents create Pin;
- board `403/code 29` with otherwise valid token prevents create Pin without marking reconnect;
- account `401/code 2` prevents create Pin and marks reconnect-required according to existing account policy;
- paginated board-list fallback finds the submitted board;
- board-list fallback uses the account-and-environment cache and invalidates it on board creation, reconnect, disconnect, and environment change;
- repeated bookmarks, duplicate pages, more than 8 pages, or more than 2,000 boards stop traversal safely;
- a traversal cap or unresolved ownership produces a retriable temporary failure, not a permanent false negative;
- Sandbox and Production environment identity remains distinct;
- new posts retain their Pinterest environment marker through scheduling and dispatch;
- a Sandbox-marked post is held or rejected after a Production cutover and never reuses or translates the board ID;
- legacy scheduled posts without a marker receive live preflight in the active environment;
- board-list and board-create handlers distinguish token failure from resource-level denial.

### 16.3 Workstream B — safe-fetch and Pinterest media-integration tests

- external image returning `200` and valid bytes is staged and published using the staged URL;
- source `404` fails before create Pin;
- redirect to a private address is rejected;
- direct IPv4 and IPv6 loopback, private, link-local, multicast, unspecified, reserved, and metadata-service targets are rejected;
- hostnames with mixed public and prohibited A/AAAA answers are rejected;
- DNS rebinding between validation and connection cannot change the dialed destination;
- the connected peer must match a validated pinned address;
- every redirect hop repeats DNS and IP validation, including a public URL redirecting to a prohibited destination;
- redirect loops, more than 5 redirects, URL userinfo, non-HTTP schemes, HTTPS-to-HTTP downgrades, and scheme-changing redirects to non-web protocols are rejected;
- content-type/byte mismatch is rejected;
- unsupported media type is rejected;
- oversized media stops streaming at the configured limit;
- the oversize path terminates without buffering the complete response;
- temporary source 5xx/timeout is retriable;
- temporary R2 failure is retriable;
- UniPost-controlled media does not receive unnecessary duplicate staging.

### 16.4 Workstream A — Dashboard regression tests

- Pinterest account selection loads boards;
- zero boards disables submission and exposes Create Board;
- creating a board refreshes and selects it;
- switching accounts clears the prior board;
- changing environment identity clears the prior board;
- a Sandbox-to-Production environment transition requires board reselection before submission;
- invalid-board failure renders board-selection guidance;
- token failure alone renders reconnect guidance.

### 16.5 Workstream B — Dashboard regression tests

- media-preflight failure renders media-replacement guidance from structured fields.

### 16.6 Workstream A — deployed acceptance

In the isolated PR environment and then the official development environment:

1. a valid connected Pinterest test account can list or create a board;
2. a valid board plus a stable supported image publishes successfully;
3. a nonexistent numeric board fails before `POST /pins` with `target_not_found`;
4. a board owned by another account fails with `select_valid_target` and does not mark the connection invalid;
5. no known code `2`, `29`, `40`, or `429` failure returns generic `contact_support`;
6. permanent destination failures do not enqueue an automatic retry;
7. temporary Pinterest provider failures follow the existing retry policy;
8. Workstream A passes deployed acceptance while Workstream B remains unavailable.

### 16.7 Workstream B — security and deployed acceptance

1. AppSec approves the exact safe-fetch implementation and negative-test evidence before external URL staging is enabled;
2. a valid external image is staged and publishes successfully through Pinterest;
3. a `404` media URL fails during media preflight with `fix_media`;
4. permanent media failures do not enqueue an automatic retry;
5. temporary source and staging failures follow the existing retry policy;
6. observability distinguishes customer media input failures from UniPost infrastructure failures and excludes the former from the infrastructure SLO numerator.

---

## 17. Acceptance criteria

### 17.1 Workstream A — Pinterest Publishing Hardening

1. Every Pinterest publish performs authoritative board validation immediately before provider write.
2. A board that is missing, deleted, cross-account, or cross-environment never reaches `POST /v5/pins` when preflight can identify the condition.
3. Known Pinterest codes `2`, `29`, `40`, and HTTP `429` produce the documented structured classifications and next actions.
4. Resource-level `403` responses no longer automatically imply that the Pinterest account must reconnect.
5. Dashboard users select boards only from live account-scoped results and receive a usable zero-board path.
6. API users receive `select_valid_target`, `reconnect_account`, or retry guidance as appropriate without parsing provider prose.
7. Invalid board failures are terminal and do not consume further automatic attempts.
8. Valid Pinterest image and video publishing behavior remains compatible with the existing request contract.
9. Board ownership fallback is bounded to 8 pages/2,000 boards, cached by account and environment, and fails temporarily when it cannot prove writability.
10. Sandbox-to-Production cutover cannot dispatch a scheduled post using a board ID selected in the old environment.
11. Workstream A is accepted and may ship independently of Workstream B.

### 17.2 Workstream B — Secure Remote Media Fetcher and Pinterest adoption

1. Arbitrary external URL staging cannot ship until the safe fetcher passes AppSec review and negative tests for private networks, IPv6, metadata endpoints, redirects, DNS rebinding, peer pinning, timeouts, and streaming size limits.
2. The safe fetcher exposes no Pinterest-specific board, provider-code, retry, or customer-copy logic.
3. Every external Pinterest `media_url` is either validated and staged on a UniPost-controlled publishing URL or rejected before provider write.
4. Invalid media failures are terminal; temporary source or storage failures follow the existing retry policy.
5. Pinterest API users receive `fix_media` or retry guidance without parsing provider prose.
6. Customer-supplied dead media retains the v1 `error_source=unipost` contract but is separately attributed in telemetry and reliability metrics.
7. Logs and public responses contain no credentials or unsafe media URLs.

The full PRD outcome is complete only when both workstreams meet their respective acceptance criteria. Workstream A completion must be reported separately and must not imply that Workstream B is available.

---

## 18. Implementation surfaces

### 18.1 Workstream A — Pinterest Publishing Hardening

The Pinterest implementation plan is expected to touch these areas without requiring a new publishing architecture:

- `api/internal/platform/pinterest.go`
  - structured provider errors;
  - direct board lookup plus bounded, cached owned-board fallback;
  - environment-aware destination preflight;
- `api/internal/platform/validate.go`
  - retain local syntax and capability validation;
- `api/internal/handler/social_posts.go`
  - remain platform-agnostic;
  - persist typed adapter failure stages only if the current error interface cannot carry `destination_preflight` cleanly;
- `api/internal/handler/pinterest_boards.go`
  - distinguish token, scope, and resource errors;
- `api/internal/postfailures`
  - Pinterest provider extraction and stable mapping;
- post/platform internal metadata persistence
  - record Pinterest environment at request creation and retain it through scheduled dispatch;
- `dashboard/src/components/posts/create-post/platform-fields/pinterest-fields.tsx`
  - live selection, zero-board state, and board creation flow;
- Dashboard post-result error presentation;
- public API documentation and SDK error examples;
- Pinterest backend, Dashboard, and deployed regression coverage.

Pinterest-specific preflight belongs in the Pinterest adapter rather than a Pinterest branch in the generic social-post handler. The implementation plan must identify the smallest cohesive diff across these surfaces and must not include unrelated platform refactoring.

### 18.2 Workstream B — Secure Remote Media Fetcher

The platform dependency implementation plan is expected to touch:

- a shared backend safe-fetch module, location chosen during implementation design
  - one provider-neutral typed interface;
  - SSRF, redirect, DNS rebinding, address pinning, timeout, MIME, and streaming-size enforcement;
  - negative security tests reusable by any future server-side remote-ingestion consumer;
- `api/internal/storage/r2.go`
  - accept verified bytes/streams from the safe-fetch path;
  - never expose the current unhardened `UploadFromURL` path to arbitrary customer URLs;
- `api/internal/platform/pinterest.go`
  - call the approved safe-fetch/storage interface after destination preflight;
  - map provider-neutral fetch failures into Pinterest's media failure contract;
- media observability and deployed Pinterest integration coverage.

Workstream B must remain narrow: it provides safe retrieval and Pinterest adoption, not a new general media-processing service.

---

## 19. Implementation sequence and completion reporting

Execute the work in this order:

1. **A1 — Pinterest error normalization:** structured provider errors and correct `403`/reconnect semantics.
2. **A2 — Pinterest destination hardening:** bounded board preflight, environment metadata, Dashboard behavior, retries, and observability.
3. **A checkpoint acceptance:** complete local, Preview regression, and browser acceptance for the exact Workstream A checkpoint SHA, and preserve the evidence before adding Workstream B commits. Do not report Workstream A deployed or complete unless it is merged and verified as a separate task.
4. **B1 — Secure Remote Media Fetcher:** provider-neutral interface, network controls, streaming bounds, storage handoff, negative security tests, and AppSec review.
5. **B2 — Pinterest media adoption:** call the approved interface, map typed media failures, and add Pinterest integration and deployed acceptance.
6. **Full acceptance:** report the PRD complete only after both Workstream A and Workstream B pass their gates.

For execution as one task, keep A and B in separate focused commit groups on the same conversation-owned branch and Draft PR. The repository's normal task-branch, Preview Acceptance, and development verification rules govern delivery but are not product scope in this PRD. Promotion to staging or production requires a separate explicit release instruction.
