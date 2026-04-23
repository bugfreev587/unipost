# UniPost API Standardization Audit
**Current inconsistencies, risks, and recommended migration path**
Version 1.0 | April 23, 2026

---

## 1. Executive summary

UniPost already has a strong API foundation, but the public surface is not yet governed by one shared design system.

The biggest issue is not any single endpoint. The biggest issue is that multiple API styles currently coexist:

- public API-key API
- dashboard/session API
- public callback and hosted-flow endpoints
- internal/admin-style operational endpoints

Because these styles live side-by-side, the API currently shows inconsistency in:

- route structure
- naming
- query parameters
- action semantics
- pagination
- error code conventions
- async contract clarity

This is still fixable with low risk because UniPost does not yet have external API customers.

---

## 2. High-priority findings

### 2.1 Public and dashboard endpoints are mixed in the same versioned space

Observed patterns in [main.go](/Users/xiaoboyu/unipost/api/cmd/api/main.go):

- public API-style routes:
  - `/v1/social-posts`
  - `/v1/media`
  - `/v1/webhooks`
- dashboard/session routes:
  - `/v1/me/...`
  - `/v1/dashboard/profiles/...`
  - `/v1/workspaces/{workspaceID}/billing`
- public system routes:
  - `/v1/public/...`
  - `/v1/oauth/...`
  - `/v1/connect/callback/...`
- admin routes:
  - `/v1/admin/...`

Risk:
- docs and SDK boundaries become blurry
- auth expectations are harder to reason about
- future compatibility promises become unclear

Recommendation:
- treat these as separate products immediately
- no new public endpoint should be added without an explicit surface label

### 2.2 The same capability exists in both workspace-explicit and workspace-implicit forms

Examples:

- `/v1/social-posts`
- `/v1/workspaces/{workspaceID}/social-posts`

- `/v1/media`
- `/v1/workspaces/{workspaceID}/media`

- `/v1/users`
- `/v1/profiles/{profileID}/users`

Risk:
- duplicated maintenance
- inconsistent SDK shape
- docs become harder to explain

Recommendation:
- public API should standardize on workspace-implicit routes
- dashboard routes may keep explicit workspace IDs where the UI needs them
- legacy aliases can remain temporarily, but the product should present one canonical form

### 2.3 Query parameter conventions are inconsistent

Observed examples in handlers:

- analytics summary/trend uses `start_date` and `end_date`
- API metrics uses `from` and `to`
- analytics refresh uses `refresh=1`
- some lists use `offset`
- posts list already supports cursor semantics in parts of the code

Risk:
- poor developer experience
- harder SDK abstraction
- accidental breaking changes when teams normalize later

Recommendation:
- time range: `from`, `to`
- booleans: `true`, `false`
- pagination: `limit`, `cursor`

### 2.4 Action endpoints have grown organically

Observed action routes:

- `/archive`
- `/restore`
- `/publish`
- `/cancel`
- `/retry`
- `/retry-now`
- `/dismiss`
- `/complete`
- `/reopen`
- `/rotate`

Risk:
- commands are not clearly distinguished from updates
- naming style drifts across modules

Recommendation:
- keep command routes only where they represent real workflows
- prefer `PATCH` for normal state/property updates
- define one naming rule for commands

### 2.5 Pagination envelope does not reflect actual pagination semantics

Current helper in [response.go](/Users/xiaoboyu/unipost/api/internal/handler/response.go) hardcodes:

- `page: 1`
- `per_page: 20`

Risk:
- metadata can be misleading
- clients may infer pagination behavior that is not real

Recommendation:
- move to cursor-oriented metadata for public APIs
- only include pagination fields that are actually meaningful

### 2.6 Error code taxonomy is only partially standardized

Current backend already uses:

- generic codes such as `VALIDATION_ERROR`, `NOT_FOUND`, `INTERNAL_ERROR`
- domain codes such as publish or queue-related conflicts

Risk:
- clients depend on message text instead of stable codes
- the same failure class may get represented differently across handlers

Recommendation:
- define a formal public error code registry
- move toward lowercase stable codes
- reserve domain-specific codes for real client branching needs

---

## 3. Current route inventory by API surface

### 3.1 Public Developer API

Examples:

- `/v1/workspace`
- `/v1/profiles`
- `/v1/social-accounts`
- `/v1/media`
- `/v1/connect/sessions`
- `/v1/social-posts`
- `/v1/users`
- `/v1/analytics/*`
- `/v1/webhooks`
- `/v1/usage`

Assessment:
- good candidate for a stable SDK-friendly API
- should become the canonical public surface

### 3.2 Dashboard API

Examples:

- `/v1/me`
- `/v1/me/bootstrap`
- `/v1/me/activation/*`
- `/v1/me/notifications/*`
- `/v1/me/tutorials/*`
- `/v1/dashboard/profiles/*`
- `/v1/workspaces/{workspaceID}/api-keys`
- `/v1/workspaces/{workspaceID}/billing/*`
- `/v1/workspaces/{workspaceID}/api-metrics/*`

Assessment:
- valid first-party API
- should not define public conventions by accident

### 3.3 Public System Endpoints

Examples:

- `/v1/public/drafts/{id}`
- `/v1/public/connect/sessions/{id}`
- `/v1/public/connect/sessions/{id}/authorize`
- `/v1/public/connect/sessions/{id}/bluesky`
- `/v1/oauth/callback/{platform}`
- `/v1/connect/callback/{platform}`

Assessment:
- these are fine as system endpoints
- they should be documented separately from the developer API

### 3.4 Admin and operations

Examples:

- `/v1/admin/stats`
- `/v1/admin/billing`
- `/v1/admin/posts`

Assessment:
- valid operational surface
- should remain clearly internal

---

## 4. Recommended target model

### 4.1 Public API

Canonical public routes:

- `/v1/workspace`
- `/v1/profiles`
- `/v1/profiles/{id}`
- `/v1/social-accounts`
- `/v1/social-accounts/{id}`
- `/v1/media`
- `/v1/media/{id}`
- `/v1/social-posts`
- `/v1/social-posts/{id}`
- `/v1/social-posts/{id}/queue`
- `/v1/social-posts/{id}/results/{result_id}/retry`
- `/v1/social-posts/{id}/publish`
- `/v1/social-posts/{id}/cancel`
- `/v1/social-posts/bulk`
- `/v1/connect/sessions`
- `/v1/connect/sessions/{id}`
- `/v1/users`
- `/v1/users/{external_user_id}`
- `/v1/analytics/summary`
- `/v1/analytics/trend`
- `/v1/analytics/by-platform`
- `/v1/analytics/rollup`
- `/v1/webhooks`
- `/v1/webhooks/{id}`
- `/v1/webhooks/{id}/rotate`

### 4.2 Dashboard API

Canonical rule:
- all new first-party-only endpoints should be introduced under `/v1/dashboard/...`

That avoids leaking dashboard-specific shapes into the public contract.

### 4.3 System endpoints

Canonical rule:
- public callbacks and hosted-flow endpoints remain outside the resource CRUD model
- document them as integration flow endpoints, not as SDK resources

---

## 5. Parameter inconsistencies to fix

### 5.1 Time range

Current mix:
- `start_date`
- `end_date`
- `from`
- `to`

Target:
- `from`
- `to`

Migration:
- add `from/to` support everywhere first
- keep `start_date/end_date` as deprecated aliases temporarily
- update docs and SDKs to emit only `from/to`

### 5.2 Boolean flags

Current example:
- `refresh=1`

Target:
- `refresh=true`

Migration:
- accept both during migration
- document only `true/false`

### 5.3 Pagination

Current mix:
- fixed meta defaults
- `offset`
- cursor-style code paths

Target:
- `limit`
- `cursor`

Migration:
- update public list endpoints first
- keep offset pagination internal where needed

---

## 6. Response inconsistencies to fix

### 6.1 Pagination metadata

Current helper returns page-style metadata even when the endpoint is not actually page-based.

Target:

```json
{
  "data": [],
  "meta": {
    "has_more": true,
    "next_cursor": "abc"
  },
  "request_id": "req_123"
}
```

### 6.2 Request IDs

Current public response helper does not include `request_id`.

Target:
- every public response should include `request_id`

Benefit:
- easier support
- easier tracing across API, jobs, and webhooks

### 6.3 Async response semantics

Current publish create endpoint often returns `201 Created` with async execution already happening in practice.

Target:
- `202 Accepted` when final publish outcome is still pending

Migration note:
- because this is technically breaking for some clients, do it before public launch or behind a compatibility switch

---

## 7. Error inconsistencies to fix

### 7.1 Generic vs domain-specific

Current pattern:
- broad use of `VALIDATION_ERROR`
- mixed use of more specific domain errors

Target:
- stable generic taxonomy for top-level classes
- explicit domain-specific codes only where client branching matters

### 7.2 Case format

Current pattern:
- uppercase snake case

Target:
- lowercase snake case for public API

Migration:
- before public launch, switch docs/SDKs/backend together
- if backward compatibility is later needed, expose aliases internally but only document one format

---

## 8. Async workflow consistency

UniPost's biggest product strength is async publish. It should also become one of the clearest parts of the API.

### 8.1 What is already good

- parent post model exists
- per-platform result model exists
- queue/job model exists
- webhook delivery model exists

### 8.2 What needs formalization

- consistent initial status
- clear terminal states
- clear `execution_mode`
- standard `202 Accepted` usage
- one documented polling pattern
- one documented webhook completion pattern

### 8.3 Target contract

For async publish:

- create post returns queued or publishing state
- `GET /v1/social-posts/{id}` returns aggregate state
- `GET /v1/social-posts/{id}/queue` returns execution detail
- webhook emits terminal aggregate outcome

This is already close in implementation and mostly needs formal standardization.

---

## 9. Recommended migration phases

### Phase 1: Freeze the design

Do now:

- adopt [api-style-guide.md](/Users/xiaoboyu/unipost/docs/api-style-guide.md) as the rulebook
- stop adding new public endpoints that do not follow it
- label every route by API surface

### Phase 2: Fix public contract mismatches before users exist

Do next:

- standardize public query params to `from/to`, `limit/cursor`, `true/false`
- standardize public error codes
- standardize public response metadata
- decide whether public async create returns `201` or `202`

### Phase 3: Separate dashboard conventions from public conventions

Do after that:

- move all newly added first-party-only routes under `/v1/dashboard/...`
- document dashboard-only vs public resources explicitly

### Phase 4: Remove duplication

Do once docs and SDKs are aligned:

- deprecate duplicate workspace-explicit public aliases
- keep one canonical public route per capability

### Current rollout status

As of April 23, 2026, the repository implementation and the deployed public API are temporarily out of sync for some standardized list responses.

Repository state:

- public list responses standardize on `meta.total` and `meta.limit`
- public errors include `error.normalized_code`
- async publish flows use `202 Accepted`
- legacy action routes emit deprecation headers where canonical replacements exist

Live validation against `https://api.unipost.dev` on April 23, 2026 found that some deployed endpoints still do not return the new list metadata shape:

- `GET /v1/social-accounts`
- `GET /v1/webhooks`

Observed SDK validation failures:

- JS validation failed on `listAccounts()` and `webhooks.list()` because `meta.limit` was missing
- Python validation failed on the same two endpoints for the same reason
- Go validation failed on `Accounts.ListPage()` and `Webhooks.ListPage()` because `meta.total` / `meta.limit` were missing

Interpretation:

- the local repo contract is internally consistent
- the SDKs are aligned with the local repo contract
- the deployment target still needs the latest backend rollout before live validation can pass cleanly

Recommended rollout sequence:

1. deploy the current backend changes
2. rerun JS, Go, and Python SDK validation against production
3. only then mark phase one and phase two as externally complete

---

## 10. Recommended immediate backlog

If we want to start implementation right away, the first concrete tasks should be:

1. Replace the pagination helper with a real cursor-capable meta shape for public endpoints.
2. Add `request_id` to all public responses and errors.
3. Normalize analytics endpoints to `from` and `to`.
4. Normalize boolean query parsing such as `refresh=true`.
5. Define and codify the public error code registry.
6. Decide whether async create endpoints will move to `202 Accepted` before launch.
7. Mark canonical public routes in docs and SDKs, and tag duplicates as legacy/internal.

---

## 11. Final assessment

UniPost does not need a full rewrite.

It needs:
- one public contract
- one naming scheme
- one error scheme
- one pagination scheme
- one async story

That is a very manageable refactor at this stage, and the current codebase is already close enough that a disciplined cleanup will pay off quickly.
