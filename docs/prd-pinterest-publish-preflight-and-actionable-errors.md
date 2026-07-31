# PRD — Pinterest Publish Preflight and Actionable Errors

**Status:** Review

**Owner:** Publishing / API Platform / Dashboard

**Created:** 2026-07-30

**Target:** Pinterest publishing reliability hardening

---

## 1. Summary

UniPost currently accepts a syntactically valid Pinterest `board_id` and sends it directly to Pinterest without confirming that the board exists, belongs to the connected account, or is visible in the active Pinterest environment. It also passes externally hosted media to Pinterest without first proving that Pinterest can fetch a valid image or video.

When Pinterest rejects one of these resources, UniPost often stores a generic `platform_error` and tells the customer to contact support. The same response contains enough provider evidence to give a specific action, but the current taxonomy does not understand Pinterest's error vocabulary.

This PRD defines a Pinterest-specific reliability layer that:

1. validates the destination board immediately before provider dispatch;
2. validates and, when necessary, stages remote media on a UniPost-controlled public URL;
3. converts known Pinterest failures into stable, actionable UniPost errors;
4. prevents Dashboard users from submitting stale or arbitrary board identifiers;
5. preserves structured provider evidence for API clients and operators;
6. keeps permanent resource failures out of the automatic retry queue.

This is a Pinterest-specific change. It does not introduce a generalized cross-platform preflight framework.

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

1. Reject unavailable or unwritable Pinterest boards before `POST /v5/pins`.
2. Reject invalid remote media before Pinterest attempts to fetch it.
3. Ensure known Pinterest provider responses produce stable, actionable UniPost error fields.
4. Preserve correct retry behavior: permanent resource failures must not retry automatically.
5. Make Dashboard board selection safe and understandable, including the zero-board state.
6. Make API usage deterministic: list or create a board, submit its returned ID, and receive a specific error if it is no longer valid.
7. Preserve backward compatibility for existing post request and result shapes.
8. Add sufficient structured telemetry to distinguish board, media, token, rate-limit, and unexpected provider failures.

---

## 5. Non-goals

- Do not build a generic preflight framework for every social platform.
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

### 6.2 Prefer controlled media delivery

Pinterest fetches `image_url` and `video_url` from its own network. UniPost should give Pinterest a URL whose availability, type, and lifetime UniPost controls.

### 6.3 Return actions, not provider prose

Customers should branch on stable UniPost fields. Pinterest messages remain diagnostic evidence, not the primary product contract.

### 6.4 Do not confuse authorization with resource ownership

A valid token can be unable to access one board. Token validation, required-scope validation, and target-resource validation are separate decisions.

### 6.5 Keep permanent failures terminal

A deleted board or dead media URL will not heal through repeated delivery attempts. The customer must change the request or destination.

---

## 7. Chosen approach

Use real-time provider preflight plus controlled media staging.

Alternatives considered:

1. **Dashboard/cache-only validation:** lower latency and fewer provider calls, but does not protect API posts, scheduled posts, deleted boards, or stale selections.
2. **Provider-call-only with better error mapping:** smaller implementation, but still spends a Pinterest write request on a known-invalid target and cannot detect dead media deterministically.
3. **Real-time preflight plus staging:** one additional board read and possible media transfer, but provides the strongest correctness and the clearest failure contract.

Option 3 is selected because publishing correctness is more important than avoiding one read request, and Pinterest's read and write rate-limit categories are separate.

---

## 8. Required publish flow

```text
request admission
  → local Pinterest syntax validation
  → enqueue delivery job
  → load account and decrypt/refresh token
  → Pinterest account/token check when required
  → authoritative board preflight
  → remote media validation and staging
  → POST /v5/pins
  → normalize provider response
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

Immediately before media staging, fetch the selected board using the active access token.

The preflight must prove:

1. the board exists in the active Pinterest environment;
2. the board is visible to the operation user represented by the token;
3. Pinterest reports enough ownership/access information to permit creation, or the board is present in the operation user's authoritative board list;
4. the selected board ID exactly matches the submitted ID.

If direct board lookup does not provide sufficient ownership fields, the adapter may compare the submitted ID against the complete paginated result of `GET /v5/boards`. The implementation must not treat numeric shape alone as proof of ownership.

Board preflight outcomes:

| Provider evidence | UniPost `error_code` | `platform_error_code` | `next_action` | Retriable |
|---|---|---|---|---|
| Board lookup `404`, code `40` | `target_not_found` | `40` | `select_valid_target` | No |
| Board lookup/list `403`, code `29`, while token otherwise works | `target_not_found` | `29` | `select_valid_target` | No |
| Account/board collection forbidden due to scope | `missing_permission` | provider code | `reconnect_or_update_permissions` | No |
| Token `401`, code `2` | `auth_token_invalid` | `2` | `reconnect_account` | No |
| Pinterest `429` | `rate_limit` | provider code if present | `wait_and_retry` | Yes |
| Pinterest timeout or 5xx during preflight | `temporary_platform_error` | provider code if present | `retry_later` | Yes |

An invalid-board preflight must not call `POST /v5/pins`.

### 8.4 Environment isolation

Board responses already expose `sandbox_mode`. This environment identity must remain attached to the board-selection state in Dashboard.

- Production publishing accepts only boards fetched or created through the Production Pinterest base URL.
- Sandbox publishing accepts only boards fetched or created through the Sandbox base URL.
- Dashboard clears the selected board when account or environment identity changes.
- The worker remains authoritative and rejects cross-environment IDs even when a client bypasses Dashboard.

No new public request field is required. The server knows the active Pinterest environment from runtime configuration.

### 8.5 Media validation and staging

Run media work only after board preflight succeeds.

For `media_ids` already backed by UniPost-controlled storage:

- use the existing media metadata and public publishing URL;
- enforce Pinterest media type and size limits;
- do not create a duplicate staged object unless the existing URL is unsuitable for provider fetch.

For external `media_urls`:

1. fetch through the existing server-side media ingestion controls;
2. reject redirects to private, loopback, link-local, or otherwise prohibited network destinations;
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

### 9.5 Failed result example: invalid media

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

Add structured events or fields for:

- `pinterest_destination_preflight_started`;
- `pinterest_destination_preflight_succeeded`;
- `pinterest_destination_preflight_failed`;
- `pinterest_media_preflight_failed`;
- `pinterest_media_staged`;
- `pinterest_create_pin_failed`.

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
- failure stage;
- retriable decision;
- duration.

Never log or emit:

- access or refresh tokens;
- authorization headers;
- workspace credential secrets;
- signed media query parameters;
- full provider request bodies containing sensitive URLs.

Operational metrics:

- destination-preflight success and failure counts;
- failures by Pinterest code and normalized reason;
- media-source rejection count by status/type;
- media staging success/failure and duration;
- create-Pin requests prevented by preflight;
- Pinterest publish success rate after preflight;
- count of known Pinterest errors that incorrectly fall back to `contact_support`.

---

## 14. Backward compatibility and migration

- No database migration is required solely for the public result shape because the existing failure model already stores error source, temporality, provider error, stage, provider code, retryability, and next action.
- Existing rows are not rewritten in bulk.
- Legacy Pinterest failure strings may be re-derived through the compatibility classifier when read through internal tools.
- Existing post requests remain valid.
- Existing board-list and board-create endpoints remain unchanged.
- Existing clients that only understand `platform_error` continue to receive a terminal failed result, but new failures use the more specific stable classification.
- Scheduled Pinterest posts already waiting in the queue receive the new worker preflight when their delivery begins.

---

## 15. Security and abuse controls

Remote media staging must use the existing hardened server-side fetch path or add equivalent protections before launch:

- deny private and local network destinations;
- validate redirects at every hop;
- enforce connection, response, and total-operation timeouts;
- enforce download-size limits while streaming;
- detect media type from bytes rather than trusting only the header or extension;
- avoid returning fetched response bodies in public errors;
- clean staged objects through the existing lifecycle policy.

Board preflight must use only the connected account's decrypted token inside the trusted backend. Board identifiers are not secrets, but they must not be accepted as proof of ownership.

---

## 16. Testing requirements

### 16.1 Unit tests

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

### 16.2 Adapter and handler tests

- valid board preflight proceeds to create Pin;
- board `404/code 40` prevents create Pin;
- board `403/code 29` with otherwise valid token prevents create Pin without marking reconnect;
- account `401/code 2` prevents create Pin and marks reconnect-required according to existing account policy;
- paginated board-list fallback finds the submitted board;
- Sandbox and Production environment identity remains distinct;
- board-list and board-create handlers distinguish token failure from resource-level denial.

### 16.3 Media tests

- external image returning `200` and valid bytes is staged and published using the staged URL;
- source `404` fails before create Pin;
- redirect to a private address is rejected;
- content-type/byte mismatch is rejected;
- unsupported media type is rejected;
- oversized media stops streaming at the configured limit;
- temporary source 5xx/timeout is retriable;
- temporary R2 failure is retriable;
- UniPost-controlled media does not receive unnecessary duplicate staging.

### 16.4 Dashboard regression tests

- Pinterest account selection loads boards;
- zero boards disables submission and exposes Create Board;
- creating a board refreshes and selects it;
- switching accounts clears the prior board;
- changing environment identity clears the prior board;
- invalid-board failure renders board-selection guidance;
- media-preflight failure renders media-replacement guidance;
- token failure alone renders reconnect guidance.

### 16.5 Deployed acceptance

In the isolated PR environment and then the official development environment:

1. a valid connected Pinterest test account can list or create a board;
2. a valid board plus a stable supported image publishes successfully;
3. a nonexistent numeric board fails before `POST /pins` with `target_not_found`;
4. a board owned by another account fails with `select_valid_target` and does not mark the connection invalid;
5. a `404` media URL fails during media preflight with `fix_media`;
6. no known code `2`, `29`, `40`, or `429` failure returns generic `contact_support`;
7. permanent failures do not enqueue an automatic retry;
8. temporary provider and staging failures follow the existing retry policy.

---

## 17. Acceptance criteria

This PRD is complete when all of the following are true:

1. Every Pinterest publish performs authoritative board validation immediately before provider write.
2. A board that is missing, deleted, cross-account, or cross-environment never reaches `POST /v5/pins` when preflight can identify the condition.
3. Every external Pinterest `media_url` is either validated and staged on a UniPost-controlled publishing URL or rejected before provider write.
4. Known Pinterest codes `2`, `29`, `40`, and HTTP `429` produce the documented structured classifications and next actions.
5. Resource-level `403` responses no longer automatically imply that the Pinterest account must reconnect.
6. Dashboard users select boards only from live account-scoped results and receive a usable zero-board path.
7. API users receive `select_valid_target`, `fix_media`, `reconnect_account`, or retry guidance as appropriate without parsing provider prose.
8. Invalid board and invalid media failures are terminal and do not consume further automatic attempts.
9. Valid Pinterest image and video publishing behavior remains compatible with the existing request contract.
10. Logs and public responses contain no credentials or unsafe media URLs.

---

## 18. Implementation surfaces

The implementation plan is expected to touch these existing areas without requiring a new publishing architecture:

- `api/internal/platform/pinterest.go`
  - structured provider errors;
  - board lookup/preflight;
  - media staging behavior;
- `api/internal/platform/validate.go`
  - retain local syntax and capability validation;
- `api/internal/handler/social_posts.go`
  - remain platform-agnostic;
  - persist typed adapter failure stages only if the current error interface cannot carry `destination_preflight` and `media_preflight` cleanly;
- `api/internal/handler/pinterest_boards.go`
  - distinguish token, scope, and resource errors;
- `api/internal/postfailures`
  - Pinterest provider extraction and stable mapping;
- `dashboard/src/components/posts/create-post/platform-fields/pinterest-fields.tsx`
  - live selection, zero-board state, and board creation flow;
- Dashboard post-result error presentation;
- public API documentation and SDK error examples;
- backend, Dashboard, and deployed regression coverage.

Pinterest-specific preflight belongs in the Pinterest adapter rather than a Pinterest branch in the generic social-post handler. The implementation plan must identify the smallest cohesive diff across these surfaces and must not include unrelated platform refactoring.

---

## 19. Release requirements

Follow the normal UniPost task-branch flow:

1. implement on the conversation-owned `dev-<task-slug>` branch and worktree;
2. run backend tests for the changed API surface;
3. run Dashboard build and relevant regression tests for Dashboard changes;
4. push only the task branch and open a Draft PR to `dev`;
5. complete Railway PR Environment and Vercel Preview Acceptance on the exact head SHA;
6. merge to `dev` only after every required check and deployed acceptance passes;
7. verify the official development environment after merge.

Promotion to staging or production is outside this PRD's implementation task unless explicitly requested as a release.
