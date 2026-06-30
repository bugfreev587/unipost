# Admin Users Failed Posts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a current-month failed-posts column to the admin users table, link the count to a filtered admin errors view, and restrict user detail opening to the `View` button.

**Architecture:** Backend admin handlers provide the new aggregate and exact user/month failure filters. Dashboard API types pass those fields and params through. The users and errors pages make small UI changes that follow the existing admin table and filter patterns.

**Tech Stack:** Go `net/http` handlers with pgx SQL, Next.js 16 App Router, React 19, TypeScript, existing admin CSS classes, Node source tests, Go tests.

---

## File Structure

- Modify `api/internal/handler/admin.go`: add `failed_posts_this_month`, expose `user_id` and `period=this_month` on the post failures endpoint, and filter SQL accordingly.
- Modify `api/internal/handler/admin_test.go`: add source tests for the new aggregate and filters.
- Modify `dashboard/src/lib/api.ts`: add `failed_posts_this_month`, `user_id`, and `period` types and query params.
- Modify `dashboard/src/app/admin/users/page.tsx`: add the Failed table column/link and remove row click behavior.
- Modify `dashboard/src/app/admin/errors/page.tsx`: parse and apply `user_id` and `period=this_month` filters from the URL.
- Modify `dashboard/tests/admin-users-scheduled-source.test.mjs`: extend existing admin users source assertions.
- Create `dashboard/tests/admin-errors-user-filter-source.test.mjs`: assert Errors page/API user and month filter wiring.

---

### Task 1: Backend Failing Tests

**Files:**
- Modify: `api/internal/handler/admin_test.go`

- [ ] **Step 1: Add failing backend source tests**

Add these tests near the existing admin user and post failure source tests:

```go
func TestAdminUsersListSQLIncludesFailedPostsThisMonth(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin.go: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"FailedPostsThisMonth int64",
		"`json:\"failed_posts_this_month\"`",
		"AS failed_posts_this_month",
		"sp.created_at >= date_trunc('month', NOW())",
		"spr.status = 'failed'",
		"COUNT(DISTINCT sp.id)::bigint",
		"&u.FailedPostsThisMonth",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin users list should include this-month failed posts %q", want)
		}
	}
}

func TestAdminPostFailuresSQLSupportsExactUserAndThisMonthFilters(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin.go: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"Period string",
		`strings.TrimSpace(q.Get("user_id"))`,
		`normalizeAdminPostFailurePeriod(q.Get("period"))`,
		"period == \"this_month\"",
		"sp.created_at >= date_trunc('month', NOW())",
		"pf.created_at >= date_trunc('month', NOW())",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin post failures should support exact user/month filter %q", want)
		}
	}
}
```

- [ ] **Step 2: Run backend tests and verify RED**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestAdminUsersListSQLIncludesFailedPostsThisMonth|TestAdminPostFailuresSQLSupportsExactUserAndThisMonthFilters'
```

Expected: FAIL because `failed_posts_this_month`, `Period string`, and period normalization are not implemented yet.

---

### Task 2: Dashboard Failing Tests

**Files:**
- Modify: `dashboard/tests/admin-users-scheduled-source.test.mjs`
- Create: `dashboard/tests/admin-errors-user-filter-source.test.mjs`

- [ ] **Step 1: Extend admin users source tests**

Add assertions to `dashboard/tests/admin-users-scheduled-source.test.mjs`:

```js
test("admin users API row exposes failed posts this month", () => {
  const api = source("src/lib/api.ts");

  assert.match(api, /export interface AdminUserRow/);
  assert.match(api, /failed_posts_this_month: number;/);
});

test("admin users table links failed counts and only View opens detail", () => {
  const page = source("src/app/admin/users/page.tsx");
  const scheduledHeader = page.indexOf("<th>Scheduled</th>");
  const failedHeader = page.indexOf("<th>Failed</th>");
  const postsUsedHeader = page.indexOf("<th>Posts Used</th>");

  assert.ok(failedHeader > scheduledHeader, "Failed should appear after Scheduled");
  assert.ok(failedHeader < postsUsedHeader, "Failed should appear before Posts Used");
  assert.match(page, /failed_posts_this_month/);
  assert.match(page, /adminUserFailedPostsHref\(u\.id\)/);
  assert.match(page, /period=this_month/);
  assert.match(page, /ad-tbl-wrap ad-tbl-static/);
  assert.doesNotMatch(page, /<tr key=\{u\.id\} onClick=/);
  assert.match(page, /colSpan=\{13\}/);
});
```

- [ ] **Step 2: Add Errors page filter source test**

Create `dashboard/tests/admin-errors-user-filter-source.test.mjs`:

```js
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";

function source(path) {
  return readFileSync(resolve(path), "utf8");
}

test("admin errors API supports exact user and this-month filters", () => {
  const api = source("src/lib/api.ts");

  assert.match(api, /user_id\?: string;/);
  assert.match(api, /period\?: "this_month";/);
  assert.match(api, /qs\.set\("user_id", params\.user_id\)/);
  assert.match(api, /qs\.set\("period", params\.period\)/);
});

test("admin errors page reads user_id and period from URL", () => {
  const page = source("src/app/admin/errors/page.tsx");

  assert.match(page, /params\.get\("user_id"\)/);
  assert.match(page, /params\.get\("period"\) === "this_month"/);
  assert.match(page, /user_id: userIdFilter \|\| undefined/);
  assert.match(page, /period: range === "this_month" \? "this_month" : undefined/);
  assert.match(page, /value="this_month"/);
});
```

- [ ] **Step 3: Run dashboard source tests and verify RED**

Run:

```bash
cd dashboard && node --test tests/admin-users-scheduled-source.test.mjs tests/admin-errors-user-filter-source.test.mjs
```

Expected: FAIL because the UI/API source does not yet include the failed column or exact user/month filters.

---

### Task 3: Backend Implementation

**Files:**
- Modify: `api/internal/handler/admin.go`

- [ ] **Step 1: Add the user-list aggregate**

Update `adminUserRow`, `ListUsers` SQL, and the scan order:

```go
FailedPostsThisMonth int64 `json:"failed_posts_this_month"`
```

SQL expression:

```sql
COALESCE((SELECT COUNT(DISTINCT sp.id)::bigint
   FROM social_posts sp
   JOIN workspaces w ON w.id = sp.workspace_id
   WHERE w.user_id = u.id
     AND sp.deleted_at IS NULL
     AND sp.created_at >= date_trunc('month', NOW())
     AND (
       sp.status = 'failed'
       OR EXISTS (
         SELECT 1
         FROM social_post_results spr
         WHERE spr.post_id = sp.id
           AND spr.status = 'failed'
       )
     )), 0) AS failed_posts_this_month,
```

Scan target:

```go
&u.PostsUsed, &u.ScheduledPosts, &u.FailedPostsThisMonth, &u.PostLimit,
```

- [ ] **Step 2: Add period support to post-failure queries**

Add a field and normalizer:

```go
Period string
```

```go
func normalizeAdminPostFailurePeriod(raw string) string {
	period := strings.TrimSpace(strings.ToLower(raw))
	if period == "this_month" {
		return period
	}
	return ""
}
```

Use dynamic date filter snippets inside `queryPostFailures`:

```go
postDateFilterSQL := "sp.created_at >= NOW() - ($2::INT * INTERVAL '1 day')"
failureEventDateFilterSQL := "pf.created_at >= NOW() - ($2::INT * INTERVAL '1 day')"
if opts.Period == "this_month" {
	postDateFilterSQL = "sp.created_at >= date_trunc('month', NOW())"
	failureEventDateFilterSQL = "pf.created_at >= date_trunc('month', NOW())"
}
```

Replace the three hard-coded date filters in the SQL string with those snippets.

- [ ] **Step 3: Wire route params**

In `ListUserPostFailures`, pass:

```go
Period: normalizeAdminPostFailurePeriod(r.URL.Query().Get("period")),
```

In `ListPostFailures`, pass:

```go
UserID: strings.TrimSpace(q.Get("user_id")),
Period: normalizeAdminPostFailurePeriod(q.Get("period")),
```

- [ ] **Step 4: Run backend tests and verify GREEN**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestAdminUsersListSQLIncludesFailedPostsThisMonth|TestAdminPostFailuresSQLSupportsExactUserAndThisMonthFilters'
```

Expected: PASS.

---

### Task 4: Dashboard Implementation

**Files:**
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/app/admin/users/page.tsx`
- Modify: `dashboard/src/app/admin/errors/page.tsx`

- [ ] **Step 1: Update API types and params**

Add to `AdminUserRow`:

```ts
failed_posts_this_month: number;
```

Add to `AdminPostFailureListParams`:

```ts
user_id?: string;
period?: "this_month";
```

Add to `listAdminPostFailures`:

```ts
if (params?.user_id) qs.set("user_id", params.user_id);
if (params?.period) qs.set("period", params.period);
```

- [ ] **Step 2: Add users-table Failed column and remove row clicks**

In `dashboard/src/app/admin/users/page.tsx`, import `Link` from `next/link`, add:

```ts
function adminUserFailedPostsHref(userId: string) {
  const params = new URLSearchParams({ user_id: userId, period: "this_month" });
  return `/admin/errors?${params.toString()}`;
}
```

Use `className="ad-tbl-wrap ad-tbl-static"`, update empty-state `colSpan` to `13`, add `<th>Failed</th>` after Scheduled, remove `<tr key={u.id} onClick={() => openUser(u.id)}>` in favor of `<tr key={u.id}>`, and add:

```tsx
<td>
  {u.failed_posts_this_month > 0 ? (
    <Link href={adminUserFailedPostsHref(u.id)} className="ad-link au-failed-link">
      {fmtNumber(u.failed_posts_this_month)}
    </Link>
  ) : (
    <span className="au-failed-zero">0</span>
  )}
</td>
```

Add CSS:

```css
.au-failed-link {
  color: var(--danger);
  font-weight: 650;
}
.au-failed-zero {
  color: var(--dmuted2);
}
```

- [ ] **Step 3: Add Errors URL filters**

In `dashboard/src/app/admin/errors/page.tsx`, replace numeric `days` state with a range state:

```ts
const RANGE_OPTIONS = ["this_month", "7", "30", "90"] as const;
type FailureRange = typeof RANGE_OPTIONS[number];
```

Parse:

```ts
const periodIsThisMonth = params.get("period") === "this_month";
const userId = params.get("user_id") || "";
```

Apply:

```ts
user_id: userIdFilter || undefined,
period: range === "this_month" ? "this_month" : undefined,
days: range !== "this_month" ? Number(range) : undefined,
```

Render a select option with `value="this_month"` and preserve the existing 7/30/90 options.

- [ ] **Step 4: Run dashboard source tests and verify GREEN**

Run:

```bash
cd dashboard && node --test tests/admin-users-scheduled-source.test.mjs tests/admin-errors-user-filter-source.test.mjs
```

Expected: PASS.

---

### Task 5: Full Validation, Merge, Push, and Dev Acceptance

**Files:**
- No new source files beyond earlier tasks.

- [ ] **Step 1: Run task-branch backend validation**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run task-branch dashboard validation**

Run:

```bash
cd dashboard && npm run build
```

Expected: PASS.

- [ ] **Step 3: Run dashboard regression when browsers are installed**

Run:

```bash
cd dashboard && npm run test:regression:dashboard
```

Expected: PASS, or report the exact missing-browser/tooling blocker.

- [ ] **Step 4: Commit task branch**

Run:

```bash
git add api/internal/handler/admin.go api/internal/handler/admin_test.go dashboard/src/lib/api.ts dashboard/src/app/admin/users/page.tsx dashboard/src/app/admin/errors/page.tsx dashboard/tests/admin-users-scheduled-source.test.mjs dashboard/tests/admin-errors-user-filter-source.test.mjs docs/superpowers/specs/2026-06-30-admin-users-failed-posts-design.md docs/superpowers/plans/2026-06-30-admin-users-failed-posts.md
git commit -m "feat: add admin user failed post counts"
```

- [ ] **Step 5: Merge into local dev and rerun required validation**

Run:

```bash
git switch dev
git pull --ff-only origin dev
git merge --no-ff dev-admin-users-failed-posts
cd api && GOCACHE=/tmp/unipost-go-build go test ./...
cd ../dashboard && npm run build
```

Run dashboard regression again because dashboard routing/admin UI changed:

```bash
cd dashboard && npm run test:regression:dashboard
```

- [ ] **Step 6: Push development branch**

Run:

```bash
git push origin dev
```

- [ ] **Step 7: Monitor remote checks and development deployment**

Wait until GitHub checks, Vercel dev deployment, Railway dev deployment, and visible triggered deployments finish successfully. Inspect logs and fix in the correct branch if any triggered check or deployment fails.

- [ ] **Step 8: Verify real development environment**

Open:

```text
https://dev-app.unipost.dev/admin/users
```

Verify:

- The `Failed` column appears after `Scheduled`.
- Clicking empty space in a user row does not open detail.
- Clicking `View` opens detail.
- Clicking a nonzero failed count opens `/admin/errors` with `user_id=<that user>` and `period=this_month`.
- The Errors page results match that exact user and current-month failure window.
