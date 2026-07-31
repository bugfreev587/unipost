# Pinterest publishing: boards, preflight, and actionable failures

This document describes UniPost's Pinterest destination behavior. It covers board discovery, board creation, the unchanged post request shape, delivery-time board preflight, structured failure fields, and retry behavior.

It does **not** describe arbitrary external-media staging. Remote media hardening is a separate workstream; until that work is accepted, clients should continue to prefer media uploaded through UniPost.

## Post request shape

Pinterest uses the existing `platform_posts[].platform_options` object. There is no public environment field and no free-form environment override.

```json
{
  "platform_posts": [
    {
      "account_id": "account_pinterest_123",
      "caption": "A new Pin",
      "media_ids": ["media_123"],
      "platform_options": {
        "board_id": "1131529543818288706",
        "title": "Optional title",
        "link": "https://example.com/optional-destination"
      }
    }
  ]
}
```

`platform_options.board_id` remains required for Pinterest and must be a numeric Pinterest board ID. Syntax validation happens when the request is admitted. The delivery worker performs authoritative destination validation immediately before it attempts to create the Pin.

## Discover boards

Dashboard clients should use the profile-scoped endpoint:

```http
GET /v1/profiles/{profile_id}/accounts/{account_id}/pinterest/boards
Authorization: Bearer <session-token>
```

The account-scoped compatibility route is also available:

```http
GET /v1/accounts/{account_id}/pinterest/boards
```

Success response:

```json
{
  "data": {
    "boards": [
      {
        "id": "1131529543818288706",
        "name": "Product launches"
      }
    ],
    "sandbox_mode": false
  }
}
```

The returned list contains only boards that UniPost can prove belong to the connected Pinterest operation user. `sandbox_mode` is part of the selection identity: a board selected from Sandbox must not be reused in Production, or vice versa.

The Dashboard treats a board snapshot as fresh for 60 seconds. It clears a selected board when the connected account changes, the returned environment changes, or the selected ID is absent from a refreshed list. The delivery worker remains authoritative even when a client bypasses the Dashboard.

## Create a board

```http
POST /v1/profiles/{profile_id}/accounts/{account_id}/pinterest/boards
Authorization: Bearer <session-token>
Content-Type: application/json

{
  "name": "Product launches"
}
```

The account-scoped compatibility route is also available at `POST /v1/accounts/{account_id}/pinterest/boards`.

Success response:

```json
{
  "data": {
    "board": {
      "id": "1131529543818288706",
      "name": "Product launches"
    }
  }
}
```

After creation, clients should refresh the board list and select the created ID only when it appears in that same account-and-environment response. Creating a board invalidates UniPost's short-lived board cache.

## Delivery-time board preflight

Before media staging or `POST /v5/pins`, the worker:

1. loads the connected Pinterest account and its current token;
2. verifies that the post's internal environment marker matches the active Pinterest runtime;
3. looks up the selected board directly;
4. proves that the board belongs to the current Pinterest operation user;
5. when direct proof is insufficient, traverses the owned-board collection with a hard limit of 8 pages and 2,000 distinct boards;
6. creates the Pin only after destination proof succeeds.

The owned-board proof cache is keyed by social account, Pinterest environment, and a one-way token fingerprint. Its TTL is 60 seconds and does not slide when a cached proof is read. Board creation, reconnect, disconnect, token changes, and environment changes invalidate or bypass stale entries.

If UniPost cannot complete bounded proof because Pinterest repeats bookmarks/pages or exceeds the traversal cap, the result is temporary and retriable. UniPost never treats incomplete proof as authorization to write.

## Internal environment isolation

For new Pinterest posts, UniPost stores the active `production` or `sandbox` identity in internal post metadata. This is not a public request field.

- Scheduled and queued delivery retain the environment selected at request creation.
- A runtime environment mismatch fails at `destination_preflight` before any provider write.
- Legacy queued rows without the marker receive live board preflight in the active environment and are not assumed to match it.
- Operators must audit queued Pinterest posts before a Sandbox-to-Production cutover; board IDs are never translated between environments.

## Structured delivery failures

Clients should branch on `platform`, `error_code`, `failure_stage`, `platform_error_code`, `is_retriable`, and `next_action`. Do not parse Pinterest's prose.

### Board missing or inaccessible

Pinterest code `29` (not accessible) and code `40` (not found), plus board ownership failures found by preflight, use the destination contract:

```json
{
  "status": "failed",
  "platform": "pinterest",
  "error_message": "The selected Pinterest board is unavailable for this connected account.",
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

Recommended customer copy:

```text
The selected Pinterest board is no longer available for this account. Choose another board and publish again.
```

This failure is terminal for the current delivery job. UniPost does not automatically retry an unchanged invalid destination.

### Invalid or expired token

Pinterest HTTP `401` or code `2` uses:

```text
error_code=auth_token_invalid
failure_stage=destination_preflight
is_retriable=false
next_action=reconnect_account
```

Reconnect guidance is reserved for token/account failures. A resource-level `403` with Pinterest code `29` does not mark the account reconnect-required.

### Missing permission

When Pinterest denies an account-level board operation because the token lacks a required scope:

```text
error_code=missing_permission
failure_stage=destination_preflight
is_retriable=false
next_action=reconnect_or_update_permissions
```

### Temporary provider failure

Pinterest `429`, provider `5xx`, network timeout, or a bounded-proof safety stop uses `destination_preflight` when it occurs during board verification.

```text
error_code=rate_limit | temporary_platform_error
failure_stage=destination_preflight
is_retriable=true
next_action=wait_and_retry | retry_later
```

The existing delivery queue schedules another attempt and keeps the result in a processing state while a retry remains. When retry attempts are exhausted, the retry job becomes terminal.

### Unknown provider failure

Unrecognized errors remain explicit rather than being misclassified:

```text
error_code=platform_error
error_temporality=unknown
is_retriable=false
next_action=contact_support
```

## Board endpoint errors

Board discovery and creation return stable Dashboard-facing codes:

| Code | HTTP status | Meaning |
| --- | --- | --- |
| `NEEDS_RECONNECT` | `409` | Pinterest rejected the token/account. |
| `MISSING_PERMISSION` | `409` | A required Pinterest board permission is missing. |
| `PINTEREST_BOARD_UNAVAILABLE` | `409` | The requested board is missing or not writable by this account. |
| `PINTEREST_TEMPORARY` | `503` | Pinterest or ownership proof is temporarily unavailable. |
| `PINTEREST_ERROR` | `502` | The failure could not be classified more specifically. |

## Safe observability

Pinterest delivery emits these integration-log actions:

- `pinterest_destination_preflight_started`
- `pinterest_destination_preflight_succeeded`
- `pinterest_destination_preflight_failed`
- `pinterest_create_pin_failed`

The events may contain post/result/account/job identifiers, environment, normalized status/code/reason, failure stage, retry decision, duration, and a truncated SHA-256 fingerprint of the board ID. They must not contain access or refresh tokens, the raw board ID, raw request/response bodies, or Pinterest API URLs.
