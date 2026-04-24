# UniPost API Style Guide
**Unified design rules for all external APIs**
Version 1.0 | April 23, 2026

---

## 1. Why this exists

UniPost is at the right stage to standardize its API before external customers depend on inconsistent behavior.

Today the codebase already has a lot of good building blocks:

- a common JSON success envelope
- a common JSON error envelope
- API-key auth for public integrations
- session auth for dashboard flows
- strong async foundations for publishing and webhooks

What is missing is a single design contract that every new endpoint must follow.

This document defines that contract.

---

## 2. API surfaces

UniPost has multiple kinds of HTTP APIs. They must not be designed as if they are the same product.

### 2.1 Public Developer API

Audience:
- external developers
- SDK users
- automation agents

Auth:
- API key

Namespace:
- `/v1/...`

Characteristics:
- long-term compatibility matters
- must be documented in public docs
- must be exposed in SDKs

Examples:
- posts
- media
- analytics
- webhooks
- connect sessions

### 2.2 Dashboard API

Audience:
- UniPost web app

Auth:
- user session

Namespace:
- `/v1/dashboard/...`

Characteristics:
- optimized for first-party product iteration
- may include UI convenience endpoints
- should not leak into SDKs or public docs as customer-facing API

Examples:
- onboarding
- tutorials
- notification channel management
- admin and settings helpers

### 2.3 Public System Endpoints

Audience:
- platform callbacks
- hosted public flows
- preview links

Auth:
- signed tokens, platform verification, or explicit public access

Namespace:
- `/v1/public/...`
- `/v1/oauth/...`
- `/v1/connect/callback/...`

Characteristics:
- not general-purpose developer resources
- should be modeled as system entrypoints, not customer CRUD APIs

Examples:
- OAuth callbacks
- hosted preview pages
- public connect session completion endpoints

### 2.4 Rule

No new endpoint should be added until its API surface is identified first.

If the caller is an SDK user, it belongs in the Public Developer API.
If the caller is the UniPost dashboard, it belongs in the Dashboard API.
If the caller is a third-party platform or a hosted browser flow, it belongs in Public System Endpoints.

---

## 3. Resource model

UniPost should be resource-first, not handler-first.

### 3.1 Primary boundary

The top-level public security boundary is the workspace.

That means public API resources should be workspace-scoped by default, either:

- explicitly in the path, or
- implicitly through the API key, but still modeled as workspace-owned resources

Recommended public model:

- `/v1/workspace`
- `/v1/profiles`
- `/v1/accounts`
- `/v1/media`
- `/v1/posts`
- `/v1/connect/sessions`
- `/v1/users`
- `/v1/analytics/...`
- `/v1/webhooks`

Rule:
- if the API key is already bound to exactly one workspace, top-level public resource collections may omit `workspaces/{workspace_id}` for ergonomics
- dashboard-only routes may use explicit workspace IDs when the UI needs cross-workspace navigation

This means UniPost should pick one public style and stick to it:

- public API: workspace-implicit
- dashboard API: workspace-explicit when needed

Do not keep both styles for the same public capability long-term.

### 3.2 Nouns over verbs

Path segments should represent resources, not internal functions.

Good:
- `/v1/posts`
- `/v1/posts/{id}`
- `/v1/posts/{id}/queue`
- `/v1/webhooks/{id}`

Allowed when there is a real command:
- `/v1/posts/{id}/publish`
- `/v1/posts/{id}/cancel`
- `/v1/webhooks/{id}/rotate`

Avoid:
- ad hoc command names when a normal update or subresource would do
- function-like paths that expose implementation rather than domain meaning

### 3.3 Naming

Use plural nouns for collections and singular path params for identifiers.

Examples:
- `/v1/posts`
- `/v1/posts/{post_id}`
- `/v1/posts/{post_id}/results/{result_id}`

Prefer stable path parameter names:
- `workspace_id`
- `profile_id`
- `post_id`
- `result_id`
- `job_id`
- `webhook_id`

If the router uses `{id}` internally, docs and SDKs may still normalize the semantic name.

---

## 4. Method semantics

Use HTTP methods consistently.

### 4.1 Standard CRUD

- `GET /resources`
  - list or search
- `GET /resources/{id}`
  - fetch one
- `POST /resources`
  - create
- `PATCH /resources/{id}`
  - partial update
- `DELETE /resources/{id}`
  - delete

### 4.2 Commands

Use `POST` for commands that are not clean partial updates.

Examples:
- publish a draft
- cancel an asynchronous operation
- rotate a secret
- retry a failed delivery

Rule:
- if the action changes a resource field directly, prefer `PATCH`
- if the action triggers a workflow or side effect, `POST /.../{id}/action` is acceptable

### 4.3 Bulk operations

Bulk operations should be explicit.

Preferred shape:
- `POST /v1/posts/bulk`

Bulk endpoints must document:
- per-item success semantics
- idempotency behavior
- maximum batch size
- partial failure behavior

---

## 5. Query, headers, and body

These three channels must have distinct responsibilities.

### 5.1 Query parameters

Use query params only for:
- filtering
- pagination
- sorting
- time ranges
- inclusion of optional expansions

Examples:
- `?status=published`
- `?platform=twitter`
- `?limit=20`
- `?cursor=abc123`
- `?from=2026-04-01T00:00:00Z&to=2026-04-23T00:00:00Z`

Do not use query params for:
- authentication
- secrets
- signatures
- idempotency
- large structured payloads

### 5.2 Headers

Use headers for request metadata and protocol control.

Standard request headers:
- `Authorization: Bearer <api_key>`
- `Idempotency-Key: <key>`
- `Content-Type: application/json`
- `Accept: application/json`

Standard response headers:
- `X-Request-Id`

Allowed specialized response headers:
- quota or warning headers when they are supplemental and documented

Do not invent new custom headers unless they provide true protocol-level value.

### 5.3 Request body

Use JSON body for:
- create payloads
- updates
- command inputs

Body shape should remain domain-oriented, not implementation-oriented.

---

## 6. Query parameter standards

UniPost should stop inventing new parameter names for the same concept.

### 6.1 Time range

Use:
- `from`
- `to`

Format:
- RFC3339 timestamp

Do not use:
- `start_date`
- `end_date`
- mixed date-only and timestamp formats on similar endpoints

### 6.2 Pagination

Public APIs should standardize on cursor pagination.

Use:
- `limit`
- `cursor`

Response:
- `meta.next_cursor`
- `meta.has_more`

Avoid for public APIs:
- `offset`
- page-number pagination

Offset pagination may remain internal in dashboard-only endpoints where compatibility pressure is low, but it should not be the long-term public standard.

### 6.3 Sorting

Use:
- `sort`
- `order`

Examples:
- `?sort=created_at&order=desc`

### 6.4 Filters

Use exact field names where possible:
- `status`
- `platform`
- `profile_id`
- `external_user_id`

For repeated values:
- prefer repeated query params or comma-separated lists, but pick one rule and document it globally

Recommended standard:
- repeated params for SDK clarity
- example: `?status=queued&status=publishing`

### 6.5 Booleans

Use:
- `true`
- `false`

Do not use:
- `1`
- `0`

Example:
- `?refresh=true`

---

## 7. Response shape

All public API responses should share one envelope shape.

### 7.1 Success

```json
{
  "data": {},
  "meta": {},
  "request_id": "req_123"
}
```

Rules:
- `data` is always present on success
- `meta` is optional
- `request_id` should always be present for support/debugging

### 7.2 Single resource response

```json
{
  "data": {
    "id": "post_123",
    "status": "queued"
  },
  "request_id": "req_123"
}
```

### 7.3 List response

```json
{
  "data": [],
  "meta": {
    "has_more": true,
    "next_cursor": "cursor_abc"
  },
  "request_id": "req_123"
}
```

Do not hardcode fake pagination fields such as `page: 1` and `per_page: 20` if the endpoint is not actually paginating that way.

### 7.4 Created resource

`POST` create endpoints should return:
- `201 Created` when a resource is created synchronously
- `202 Accepted` when the request creates or schedules an asynchronous operation

### 7.5 Deletes

Preferred delete response:

```json
{
  "data": {
    "id": "wh_123",
    "deleted": true
  },
  "request_id": "req_123"
}
```

Alternative:
- `204 No Content` for simple internal endpoints

Public API should prefer explicit JSON confirmation for consistency.

---

## 8. Error shape

All public API errors should use one structure.

### 8.1 Standard error envelope

```json
{
  "error": {
    "code": "validation_error",
    "message": "Invalid `from` parameter",
    "details": {
      "field": "from"
    }
  },
  "request_id": "req_123"
}
```

### 8.2 Error code rules

Error codes should be:
- lowercase snake_case
- stable over time
- machine-meaningful

Examples:
- `validation_error`
- `unauthorized`
- `forbidden`
- `not_found`
- `conflict`
- `rate_limited`
- `internal_error`

Domain-specific codes are allowed, but only when the generic code is not enough.

Examples:
- `quota_exceeded`
- `platform_account_unhealthy`
- `idempotency_conflict`
- `post_already_terminal`

### 8.3 HTTP status mapping

- `400 Bad Request`
  - malformed JSON or invalid primitive format
- `401 Unauthorized`
  - missing or invalid auth
- `403 Forbidden`
  - authenticated but not allowed
- `404 Not Found`
  - resource absent in this scope
- `409 Conflict`
  - state conflict, optimistic lock, idempotency mismatch
- `422 Unprocessable Entity`
  - structurally valid request but business rule violation
- `429 Too Many Requests`
  - rate limit or quota throttle
- `500 Internal Server Error`
  - unexpected server failure
- `502/503/504`
  - upstream/transient infrastructure failures when appropriate

Rule:
- do not overload `VALIDATION_ERROR` for every problem in the system

---

## 9. Idempotency

All public create or command endpoints with side effects should define idempotency behavior.

### 9.1 Header

Use:
- `Idempotency-Key`

### 9.2 Required scope

The idempotency key should be scoped by:
- workspace
- endpoint
- request fingerprint

### 9.3 Behavior

If the same idempotency key is replayed with the same effective request:
- return the original result

If replayed with a different request body:
- return `409 Conflict`
- error code: `idempotency_conflict`

### 9.4 Where required

Strongly recommended for:
- `POST /v1/posts`
- `POST /v1/posts/bulk`
- `POST /v1/media`
- `POST /v1/connect/sessions`
- `POST /v1/webhooks`

---

## 10. Async operation rules

UniPost is an async-first API for important workflows like publishing and webhook delivery.

This should be formalized.

### 10.1 When to use async

Use asynchronous execution when:
- the operation fans out to multiple platforms
- third-party APIs can be slow
- retries are expected
- the final state may take seconds or minutes

### 10.2 Response semantics

Async create/command endpoints should return:
- `202 Accepted` when final work is pending
- a resource containing initial status

Example:

```json
{
  "data": {
    "id": "post_123",
    "status": "queued",
    "execution_mode": "async"
  },
  "request_id": "req_123"
}
```

### 10.3 Status model

Each async domain should define:
- pending states
- in-progress states
- terminal success states
- terminal failure states

For posts, recommended top-level rule:
- `draft`
- `scheduled`
- `queued`
- `publishing`
- `published`
- `partial`
- `failed`
- `canceled`
- `archived`

### 10.4 Completion access

Every async workflow should support at least one of:
- polling a resource
- receiving a developer webhook

Prefer both.

---

## 11. Webhook rules

Developer webhooks are part of the public API contract, not a side feature.

### 11.1 Subscription resource

Webhook subscriptions are a first-class resource:
- `POST /v1/webhooks`
- `GET /v1/webhooks`
- `GET /v1/webhooks/{id}`
- `PATCH /v1/webhooks/{id}`
- `DELETE /v1/webhooks/{id}`
- `POST /v1/webhooks/{id}/rotate`

### 11.2 Signature

Use:
- `X-UniPost-Signature`
- `X-UniPost-Event`

Signature format should be documented and SDKs must verify it consistently.

### 11.3 Event naming

Event names should be:
- noun-oriented
- past-tense outcome oriented when event means a state transition

Examples:
- `post.published`
- `post.partial`
- `post.failed`
- `account.connected`

### 11.4 Event payloads

Event payload should be one canonical schema per event family.

Do not let SDK event types drift from backend event reality.

---

## 12. Versioning and deprecation

UniPost should treat `v1` as a contract, not just a folder name.

### 12.1 Versioning

Major breaking changes require:
- a new version namespace, or
- an explicitly managed compatibility plan

### 12.2 Deprecation

When replacing an endpoint:
- mark old endpoint as deprecated in docs
- keep old and new behavior in parallel for a defined migration window
- log internal usage of deprecated endpoints

### 12.3 Compatibility rule

Do not make silent breaking changes in:
- response shape
- enum values
- query parameter semantics
- idempotency behavior

---

## 13. SDK alignment rules

If an endpoint is public, it should be judged by whether it can be represented cleanly in SDKs.

Before a public endpoint is considered complete:
- docs must exist
- SDK method must exist or be intentionally deferred
- validation script should cover it

No public API should remain in a state where:
- backend supports it
- docs only partially describe it
- SDK types are outdated

---

## 14. Design checklist for new endpoints

Before shipping a new endpoint, confirm:

1. Which API surface does it belong to?
2. Is the path resource-oriented?
3. Are query params only used for filtering/pagination/sorting/time range?
4. Are headers only used for metadata/auth/protocol control?
5. Does it use the standard response envelope?
6. Does it use the standard error envelope?
7. Is pagination consistent with the public standard?
8. Is idempotency defined?
9. Is async behavior explicit?
10. Is it documented and represented in SDKs?

If any answer is no, the endpoint is not ready.

---

## 15. Immediate standardization decisions for UniPost

These decisions should be adopted now:

1. Public API stays workspace-implicit under `/v1/...`.
2. Dashboard API moves conceptually under `/v1/dashboard/...` for all new first-party-only endpoints.
3. Time range params standardize on `from` and `to`.
4. Public pagination standardizes on `limit` and `cursor`.
5. Public errors move toward lowercase stable codes.
6. Async publish endpoints should return `202 Accepted` once compatibility risk is manageable.
7. Every public capability must be reflected in docs, SDKs, and validation scripts together.

---

## 16. Non-goals

This guide does not force:
- strict REST purity for every workflow
- immediate removal of all legacy endpoints
- identical internals across dashboard and public handlers

It does require:
- one public contract
- one naming system
- one error system
- one response system

That consistency is the product.
