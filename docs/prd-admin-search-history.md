# PRD - Admin Search History

**Status:** Planning
**Owner:** Admin / Dashboard / API
**Created:** 2026-06-18
**Target:** Cross-device search history for high-frequency admin search and text-filter inputs

---

## Problem

UniPost admins repeatedly search the same operational identifiers while debugging users, posts, API traffic, and integration failures:

- workspace IDs
- user emails
- request or post identifiers
- error snippets
- captions or post IDs

The current admin dashboard treats each search box as ephemeral state. If an admin leaves a page, signs in on another browser, or uses another computer, previously typed searches are gone. Browser-native autocomplete is unreliable because the dashboard uses different inputs, different domains per environment, and privacy settings vary by device.

Admins need a first-party, account-scoped search history that follows them across devices after sign-in.

## Product Direction

Add server-backed admin search history for selected admin text inputs. The browser renders a compact dropdown when the input receives focus, showing the current admin user's recent values for that exact field. Selecting a row fills the input and applies the filter using the page's existing behavior.

Search history is scoped to the signed-in admin user. It is not shared across admins, workspaces, or customers.

Do not use `localStorage` as the source of truth. A small in-memory client cache is allowed for responsiveness, but persistence must go through the backend so history syncs across browsers and computers.

No feature flag is required for this work.

## V1 Scope

Implement search history for exactly these admin inputs:

| Page | Route | Field | Field key |
| --- | --- | --- | --- |
| Admin logs | `/admin/logs` | `Search logs...` | `admin.logs.q` |
| Admin logs | `/admin/logs` | `Workspace ID` | `admin.logs.workspace_id` |
| Admin logs | `/admin/logs` | `User email` | `admin.logs.owner_email` |
| Admin errors | `/admin/errors` | search box | `admin.errors.search` |
| Admin API metrics | `/admin/api-metrics` | `workspace_id` | `admin.api_metrics.workspace_id` |
| Admin posts | `/admin/posts` | search box | `admin.posts.search` |
| Admin users | `/admin/users` | search box | `admin.users.search` |

V1 should not add history to dropdown selects, date/time ranges, workspace dashboard logs, public docs search, post list search, inbox search, or marketing/public pages.

## Goals

1. Let an admin reuse recent search values from any signed-in browser or computer.
2. Keep history private to the signed-in admin user.
3. Avoid saving blank values, whitespace-only values, or duplicate rows.
4. Keep the UI fast and unobtrusive: focus shows history, typing filters it, click or keyboard selection applies it.
5. Preserve existing page filtering behavior and URL sync behavior.
6. Avoid introducing new frontend dependencies.
7. Add backend and frontend tests that prove history is scoped, deduped, capped, and selectable.

## Non-goals

- No customer-facing search history.
- No search history for normal workspace dashboard pages in V1.
- No organization-wide shared admin history.
- No browser-only `localStorage` persistence.
- No full saved-search feature with named filters, compound filters, or alerts.
- No automatic history import from browser autocomplete.
- No feature flag.

## Users and Permissions

Only authenticated UniPost admins can use this feature.

The backend must mount the API under the existing admin middleware:

```text
/v1/admin/search-history
```

The caller must never provide a user ID. The backend derives the admin user ID from the authenticated Clerk session and reads or writes only rows owned by that user.

`/admin/logs` remains super-admin-only. Because the search-history API is mounted under the broader admin middleware, the backend must also enforce the existing super-admin checker for `admin.logs.*` field keys. Do not rely on UI reachability for this distinction.

The backend should validate every `field_key` against the V1 allowlist so unsupported fields cannot be used as arbitrary storage.

## Data Model

Add a table for per-admin search history:

```sql
admin_search_history
- id text primary key default gen_random_uuid()::text
- admin_user_id text not null
- field_key text not null
- value text not null
- value_normalized text not null
- usage_count integer not null default 1
- first_used_at timestamptz not null default now()
- last_used_at timestamptz not null default now()
- created_at timestamptz not null default now()
- updated_at timestamptz not null default now()
```

Constraints and indexes:

```sql
unique (admin_user_id, field_key, value_normalized)
index (admin_user_id, field_key, last_used_at desc, usage_count desc)
```

Normalization rules:

- Trim leading and trailing whitespace.
- Collapse internal runs of whitespace to a single space for `value_normalized`.
- Lowercase `value_normalized` for case-insensitive dedupe.
- Preserve the trimmed original casing in `value` for display.
- Reject values longer than 512 characters.
- Reject values shorter than 2 characters after trimming. V1 has no per-field minimum-length exceptions.

Retention and caps:

- Return at most 8 rows per field to the dashboard.
- Keep at most 25 rows per admin user and field.
- After each upsert, delete older rows beyond the newest 25 for that `(admin_user_id, field_key)` pair.
- Delete rows with `last_used_at < now() - interval '180 days'` opportunistically from write paths. V1 does not need a scheduled cleanup job.
- Store `usage_count` for future ranking and auditing. V1 list queries should sort by `last_used_at desc, usage_count desc` so repeated searches break timestamp ties predictably.

## API Design

### List history

```http
GET /v1/admin/search-history?field_key=admin.logs.q&limit=8
```

Response:

```json
{
  "data": [
    {
      "id": "6fd5f2c3-2d83-4e42-8ec0-565260a06156",
      "field_key": "admin.logs.q",
      "value": "quota_exceeded",
      "usage_count": 4,
      "last_used_at": "2026-06-18T17:12:42Z"
    }
  ],
  "request_id": "req_123"
}
```

Rules:

- `field_key` is required.
- `field_key` must be in the V1 allowlist.
- `limit` defaults to 8 and is capped at 8.
- Rows are sorted by `last_used_at desc, usage_count desc`.

### Save history value

```http
POST /v1/admin/search-history
Content-Type: application/json

{
  "field_key": "admin.logs.workspace_id",
  "value": "wk_abc123"
}
```

Response:

```json
{
  "data": {
    "id": "6fd5f2c3-2d83-4e42-8ec0-565260a06156",
    "field_key": "admin.logs.workspace_id",
    "value": "wk_abc123",
    "usage_count": 2,
    "last_used_at": "2026-06-18T17:15:03Z"
  },
  "request_id": "req_123"
}
```

Rules:

- Upsert by `(admin_user_id, field_key, value_normalized)`.
- On duplicate, increment `usage_count`, update `value` to the latest trimmed display value, and update `last_used_at`.
- Empty or invalid values return `400 VALIDATION_ERROR`.
- Unsupported field keys return `400 VALIDATION_ERROR`.

Implementation should use the existing response helpers and envelope shape from `api/internal/handler/response.go`: `writeSuccess(w, data)` or the matching metadata helper for successful responses, and `writeError(...)` for errors. Do not add a `success` boolean to admin search-history responses.

### Delete one history row

```http
DELETE /v1/admin/search-history/{id}
```

Rules:

- The row must belong to the current admin user.
- Deleting a missing or foreign row should return `404`.
- V1 UI should expose this as a small remove button in each dropdown row.

## Frontend UX

Create a reusable dashboard component:

```text
dashboard/src/app/admin/_components/search-history-input.tsx
```

The component should support:

- Controlled `value` and `onChange`.
- Optional `onCommit` callback for saving history.
- Existing `className` and inline `style` so it can replace both `.ad-search` inputs and custom inline-styled log inputs.
- Optional leading icon slot for `/admin/logs`.
- `fieldKey` required.
- `placeholder`, `aria-label`, and disabled state passthrough.

Interaction behavior:

1. On focus, fetch and show recent history for that field.
2. While typing, filter the already fetched history client-side by substring match.
3. On click or keyboard selection, set the input value and invoke the page's existing setter.
4. Save a value when the admin presses Enter, blurs the input with a non-empty value, or selects a history row.
5. Do not save on every keypress.
6. Close on Escape, outside click, or selection.
7. Support ArrowUp, ArrowDown, Enter, Escape, and Tab.
8. Show no dropdown when there are no history rows.
9. Include a per-row remove button that deletes the history row without applying it.

Visual direction:

- Match the current admin UI: compact height, 6-10px radius, `var(--surface-raised)` or `var(--surface2)`, `var(--dborder)`, `var(--dtext)`, `var(--dmuted2)`.
- The dropdown should be absolutely positioned under the input wrapper.
- It must not push filter bars or tables down.
- It must fit narrow mobile widths without horizontal overflow.
- It must not use emoji or add a new icon package.

## Page Integration

### `/admin/logs`

Replace these inputs:

- main `query` input, field key `admin.logs.q`
- `workspaceFilter`, field key `admin.logs.workspace_id`
- `ownerEmailFilter`, field key `admin.logs.owner_email`

Preserve:

- existing `Search` icon on the main query input
- existing URL sync for `workspace_id` and `owner_email`
- existing active filter chips
- existing immediate refresh behavior

### `/admin/errors`

Replace `searchInput` with `SearchHistoryInput` using `admin.errors.search`.

Preserve the existing 300ms debounce from `searchInput` to `search`.

### `/admin/api-metrics`

Replace `workspaceID` input with `SearchHistoryInput` using `admin.api_metrics.workspace_id`.

Preserve existing immediate reload behavior driven by `workspaceID`.

### `/admin/posts`

Replace `searchInput` with `SearchHistoryInput` using `admin.posts.search`.

Preserve the existing 300ms debounce from `searchInput` to `search`.

### `/admin/users`

Replace `searchInput` with `SearchHistoryInput` using `admin.users.search`.

Preserve:

- existing 300ms debounce from `searchInput` to `search`
- `setOffset(0)` when the search input changes

## Implementation Notes

Backend files likely involved:

- `api/internal/db/migrations/084_admin_search_history.sql`
- `api/internal/db/queries/admin_search_history.sql`
- generated `api/internal/db/admin_search_history.sql.go`
- `api/internal/handler/admin_search_history.go`
- `api/cmd/api/main.go`
- `api/internal/handler/admin_search_history_test.go`

Frontend files likely involved:

- `dashboard/src/lib/api.ts`
- `dashboard/src/app/admin/_components/search-history-input.tsx`
- `dashboard/src/app/admin/logs/page.tsx`
- `dashboard/src/app/admin/errors/page.tsx`
- `dashboard/src/app/admin/api-metrics/page.tsx`
- `dashboard/src/app/admin/posts/page.tsx`
- `dashboard/src/app/admin/users/page.tsx`

Test files likely involved:

- `api/internal/handler/admin_search_history_test.go`
- `dashboard/tests/admin-search-history-source.test.mjs`
- optional Playwright regression coverage in `dashboard/tests/regression/dashboard-nav.spec.ts` or a new focused admin search history spec if test auth fixtures already support admin pages.

## Security and Privacy

Search history can contain customer emails, workspace IDs, post IDs, request IDs, and error text. Treat it as admin-only operational data.

Requirements:

- Never expose one admin's history to another admin.
- Never allow client-supplied `admin_user_id`.
- Enforce super-admin access for `admin.logs.*` field keys in the backend handler.
- Do not save empty values.
- Do not save values longer than 512 characters.
- Use allowlisted field keys only.
- Do not log search history request bodies in application logs.
- Do not expose history through customer workspace APIs.
- Keep production and development histories separated by their existing environment databases.

## Error Handling

Frontend behavior:

- If list-history fetch fails, keep the input usable and hide the dropdown.
- If save-history fails, do not block the search.
- If delete-history fails, keep the row visible and allow retry.
- Avoid visible error banners for history failures unless the failure blocks the primary search, which it should not.

Backend behavior:

- Validation failures return `400 VALIDATION_ERROR`.
- Unauthorized users return the existing admin middleware response.
- Foreign or missing deletes return `404`.
- Unexpected persistence failures return `500 INTERNAL_ERROR`.

## Acceptance Criteria

1. A super admin searches `/admin/logs` by log query, workspace ID, and user email; each value appears in that exact field's dropdown on another signed-in browser after refresh.
2. Admin history does not cross fields. A workspace ID saved in `/admin/api-metrics` does not appear in `/admin/users`.
3. Admin history does not cross users. Admin A cannot see Admin B's history.
4. Reusing the same value dedupes the row and moves it to the top instead of creating duplicates.
5. Each field returns no more than 8 visible suggestions.
6. Each field stores no more than 25 retained suggestions per admin user.
7. Selecting a suggestion applies the existing filter for that page.
8. Removing a suggestion deletes it from the dropdown and from the backend for that admin user.
9. If the history API is unavailable, all search boxes still work as they do today.
10. No feature flag is added.

## Validation Plan

Backend:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
```

Dashboard:

```bash
cd dashboard
npm run build
```

Regression, when Playwright browsers are installed:

```bash
cd dashboard
npm run test:regression:dashboard
```

Manual local checks:

1. Sign in as an admin.
2. Enter values in each V1 field.
3. Refresh the page and confirm the dropdown appears on focus.
4. Use keyboard navigation to select an item.
5. Delete one item and confirm it no longer appears after refresh.
6. Use a second browser profile signed in as the same admin and confirm history syncs.
7. Use a different admin account and confirm history is not visible.

Development deployment self-acceptance after pushing `origin/dev`:

1. Wait for the development deployment to finish.
2. Open the relevant development admin domains, not production domains.
3. Verify the same V1 fields against the deployed development backend.
4. Confirm cross-browser sync in development for the same admin user.

## Rollout

This is an admin-only feature with no feature flag.

Rollout sequence:

1. Ship database migration and backend API.
2. Ship dashboard component and page integrations.
3. Validate locally.
4. Merge to `dev`, rerun required checks, push `origin/dev`.
5. Monitor triggered checks and development deployments.
6. Perform deployed development self-acceptance.

Rollback:

- If frontend history UX breaks but search still works, revert dashboard integrations to plain inputs.
- If backend API causes errors, remove dashboard calls and leave the table unused until a fix ships.
- If migration must be reverted, drop `admin_search_history` only after confirming no runtime code depends on it.

## Open Questions

1. Should V2 replace opportunistic 180-day cleanup with a scheduled maintenance job if table growth becomes meaningful?
2. Should V2 add named saved searches that include dropdown filters and date ranges?
3. Should V2 let super admins optionally share a search with another admin for incident handoff?
