# Admin Email Filters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a complete recipient-email dropdown and inclusive attempted-date range to `/admin/email` while retaining all existing filters.

**Architecture:** The Go admin handler will reuse the unified email-notification CTE for both paginated filtering and a new distinct-email options endpoint. The Next.js client will keep local filter state, convert browser-local calendar dates into RFC 3339 half-open boundaries, and send all filters to the server. Existing admin UI primitives and native form controls remain in use.

**Tech Stack:** Go 1.x, chi, pgx/PostgreSQL, Next.js 16, React 19, TypeScript, Playwright source regression tests.

---

## File structure

- `api/internal/handler/admin.go`: extend email list query/validation and add the filter-options query and handler.
- `api/internal/handler/admin_test.go`: test SQL predicates, range validation, option query, and route registration.
- `api/cmd/api/main.go`: register the new authenticated admin filter-options route.
- `dashboard/src/lib/api.ts`: add filter parameter types, query serialization, options response type, and options request.
- `dashboard/src/app/admin/email/filters.ts`: isolate browser-local date-boundary conversion and range validation.
- `dashboard/src/app/admin/email/page.tsx`: add option loading, Email/Date controls, pagination resets, and inline range errors.
- `dashboard/tests/regression/dashboard.spec.ts`: enforce the frontend filter contract through the existing source regression suite.

### Task 1: Backend list filters and date validation

**Files:**
- Modify: `api/internal/handler/admin_test.go`
- Modify: `api/internal/handler/admin.go`

- [ ] **Step 1: Write failing backend tests**

Add tests asserting that:

```go
func TestAdminEmailNotificationsSQLFiltersRecipientAndAttemptedRange(t *testing.T) {
	sql := adminEmailNotificationsWhereSQL
	for _, want := range []string{
		"LOWER(email) = LOWER($7)",
		"attempted_at >= $8",
		"attempted_at < $9",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin email notifications filter missing %q:\n%s", want, sql)
		}
	}
}

func TestParseAdminEmailNotificationRange(t *testing.T) {
	start, end, err := parseAdminEmailNotificationRange(
		"2026-07-01T07:00:00Z",
		"2026-07-03T07:00:00Z",
	)
	if err != nil || start == nil || end == nil {
		t.Fatalf("valid range = %v/%v/%v", start, end, err)
	}
	if _, _, err := parseAdminEmailNotificationRange("bad", ""); err == nil {
		t.Fatal("malformed start_at should fail")
	}
	if _, _, err := parseAdminEmailNotificationRange(
		"2026-07-03T07:00:00Z",
		"2026-07-03T07:00:00Z",
	); err == nil {
		t.Fatal("zero-length range should fail")
	}
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestAdminEmailNotificationsSQLFiltersRecipientAndAttemptedRange|TestParseAdminEmailNotificationRange'
```

Expected: FAIL because the new predicates and parser do not exist.

- [ ] **Step 3: Implement the minimal backend list behavior**

Extend `adminEmailNotificationsQuery` with:

```go
Email   string
StartAt *time.Time
EndAt   *time.Time
```

Extend `adminEmailNotificationsWhereSQL` with exact recipient email and half-open attempted-time clauses:

```sql
AND ($7::TEXT = '' OR LOWER(email) = LOWER($7))
AND ($8::TIMESTAMPTZ IS NULL OR attempted_at >= $8)
AND ($9::TIMESTAMPTZ IS NULL OR attempted_at < $9)
```

Append `Email`, `StartAt`, and `EndAt` to the shared query args and move pagination placeholders to `$10` and `$11`.

Add a strict helper:

```go
func parseAdminEmailNotificationRange(startRaw, endRaw string) (*time.Time, *time.Time, error) {
	parse := func(name, raw string) (*time.Time, error) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, nil
		}
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, fmt.Errorf("%s must be an RFC3339 timestamp", name)
		}
		return &value, nil
	}
	start, err := parse("start_at", startRaw)
	if err != nil {
		return nil, nil, err
	}
	end, err := parse("end_at", endRaw)
	if err != nil {
		return nil, nil, err
	}
	if start != nil && end != nil && !end.After(*start) {
		return nil, nil, fmt.Errorf("end_at must be after start_at")
	}
	return start, end, nil
}
```

Validate the range in `ListEmailNotifications` before querying; return `422 VALIDATION_ERROR` with the helper error when invalid. Pass `email`, `start_at`, and `end_at` into `adminEmailNotificationsQuery`.

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run the same focused Go test command. Expected: PASS.

- [ ] **Step 5: Commit the backend list filters**

```bash
git add api/internal/handler/admin.go api/internal/handler/admin_test.go
git commit -m "feat: filter admin emails by recipient and date"
```

### Task 2: Backend Email options endpoint

**Files:**
- Modify: `api/internal/handler/admin_test.go`
- Modify: `api/internal/handler/admin.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing options endpoint tests**

Add a test that checks the options SQL for the unified CTE, exclusion of blank email addresses, case-insensitive deduplication, and deterministic sorting. Extend the route test to require:

```go
`r.Get("/v1/admin/email-notifications/filter-options", adminHandler.ListEmailNotificationFilterOptions)`
```

- [ ] **Step 2: Run the focused tests and verify RED**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestAdminEmailNotificationFilterOptions|TestAdminEmailNotificationsRouteIsRegistered'
```

Expected: FAIL because the query, handler, and route are missing.

- [ ] **Step 3: Implement the options query and handler**

Build the query from `adminEmailNotificationsCTESQL`:

```sql
SELECT email
FROM (
  SELECT DISTINCT ON (LOWER(email)) email
  FROM email_notifications
  WHERE BTRIM(email) <> ''
  ORDER BY LOWER(email), email
) distinct_emails
ORDER BY LOWER(email), email
```

Add `queryEmailNotificationFilterOptions`, scan all rows into `[]string`, and return:

```go
writeSuccess(w, map[string]any{"emails": emails})
```

Register the GET route inside the existing Clerk/admin middleware group.

- [ ] **Step 4: Run focused and full backend tests**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler
GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit the options endpoint**

```bash
git add api/internal/handler/admin.go api/internal/handler/admin_test.go api/cmd/api/main.go
git commit -m "feat: list admin email filter options"
```

### Task 3: Dashboard API and local-date helper

**Files:**
- Modify: `dashboard/tests/regression/dashboard.spec.ts`
- Modify: `dashboard/src/lib/api.ts`
- Create: `dashboard/src/app/admin/email/filters.ts`

- [ ] **Step 1: Write a failing frontend contract test**

Extend the admin email regression test to require:

```ts
expect(apiSource).toContain("AdminEmailNotificationFilterOptions");
expect(apiSource).toContain("listAdminEmailNotificationFilterOptions");
expect(apiSource).toContain('qs.set("email", params.email)');
expect(apiSource).toContain('qs.set("start_at", params.start_at)');
expect(apiSource).toContain('qs.set("end_at", params.end_at)');
expect(pageSource).toContain("buildAttemptedDateRange");
expect(pageSource).toContain("All emails");
expect(pageSource).toContain('type="date"');
```

- [ ] **Step 2: Run the regression case and verify RED**

```bash
cd dashboard
npx playwright test --config=playwright.regression.config.ts tests/regression/dashboard.spec.ts --grep "admin email notifications"
```

Expected: FAIL because the API and page wiring are missing.

- [ ] **Step 3: Add API types and serialization**

Extend `AdminEmailNotificationListParams`:

```ts
email?: "all" | string;
start_at?: string;
end_at?: string;
```

Add:

```ts
export interface AdminEmailNotificationFilterOptions {
  emails: string[];
}

export async function listAdminEmailNotificationFilterOptions(
  token: string,
): Promise<ApiResponse<AdminEmailNotificationFilterOptions>> {
  return request("/v1/admin/email-notifications/filter-options", token);
}
```

Serialize `email` only when it is not `all`; serialize non-empty timestamp bounds.

- [ ] **Step 4: Add the browser-local date helper**

Create `filters.ts` with:

```ts
export type AttemptedDateRange = {
  start_at?: string;
  end_at?: string;
  error?: string;
};

function localMidnight(date: string): Date {
  const [year, month, day] = date.split("-").map(Number);
  return new Date(year, month - 1, day);
}

export function buildAttemptedDateRange(startDate: string, endDate: string): AttemptedDateRange {
  if (startDate && endDate && endDate < startDate) {
    return { error: "End date must be on or after start date." };
  }
  const range: AttemptedDateRange = {};
  if (startDate) range.start_at = localMidnight(startDate).toISOString();
  if (endDate) {
    const endExclusive = localMidnight(endDate);
    endExclusive.setDate(endExclusive.getDate() + 1);
    range.end_at = endExclusive.toISOString();
  }
  return range;
}
```

- [ ] **Step 5: Run TypeScript/build validation**

```bash
cd dashboard
npm run build
```

Expected: PASS.

- [ ] **Step 6: Commit the API and helper**

```bash
git add dashboard/src/lib/api.ts dashboard/src/app/admin/email/filters.ts dashboard/tests/regression/dashboard.spec.ts
git commit -m "feat: add admin email filter client contract"
```

### Task 4: Admin Email page controls and behavior

**Files:**
- Modify: `dashboard/src/app/admin/email/page.tsx`
- Modify: `dashboard/tests/regression/dashboard.spec.ts`

- [ ] **Step 1: Strengthen the failing frontend contract test**

Require the page source to include:

```ts
expect(pageSource).toContain("listAdminEmailNotificationFilterOptions");
expect(pageSource).toContain('aria-label="Filter by recipient email"');
expect(pageSource).toContain('aria-label="Attempted from"');
expect(pageSource).toContain('aria-label="Attempted through"');
expect(pageSource).toContain("range.error");
expect(pageSource).toContain("setOffset(0)");
```

Run the focused Playwright test and confirm it fails for the missing page behavior.

- [ ] **Step 2: Add independent option-loading state**

Add `emailOptions`, `emailOptionsLoading`, and `filterOptionsError`. Fetch the new endpoint once on mount and again from the manual refresh callback. Preserve already loaded list rows if option loading fails.

- [ ] **Step 3: Add Email and attempted-date state**

Add:

```ts
const [email, setEmail] = useState("all");
const [startDate, setStartDate] = useState("");
const [endDate, setEndDate] = useState("");
const range = useMemo(
  () => buildAttemptedDateRange(startDate, endDate),
  [startDate, endDate],
);
```

Skip list requests while `range.error` exists. Otherwise pass `email`, `range.start_at`, and `range.end_at` to `listAdminEmailNotifications`.

- [ ] **Step 4: Render accessible native controls**

Add a labeled Email select with `All emails`, a disabled loading state, and all server-returned options. Add labeled start/end `type="date"` controls and an inline range error. Use compact CSS classes that wrap at mobile widths and keep all existing controls.

- [ ] **Step 5: Reset pagination and wire refresh**

Include Email and both dates in the pagination-reset effect. Replace the shell refresh callback with a function that reloads the options and list together.

- [ ] **Step 6: Run focused regression and dashboard build**

```bash
cd dashboard
npx playwright test --config=playwright.regression.config.ts tests/regression/dashboard.spec.ts --grep "admin email notifications"
npm run build
```

Expected: PASS.

- [ ] **Step 7: Commit the page behavior**

```bash
git add dashboard/src/app/admin/email/page.tsx dashboard/tests/regression/dashboard.spec.ts
git commit -m "feat: add admin email page filters"
```

### Task 5: Full verification and integration

**Files:**
- Verify all changed files.

- [ ] **Step 1: Run all required local validation on the task branch**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
cd ../dashboard
npm run build
npm run test:regression:dashboard
```

Expected: PASS. If Playwright browsers are not installed, record the exact skipped failure.

- [ ] **Step 2: Review the diff and repository state**

```bash
git diff origin/dev...HEAD --check
git diff origin/dev...HEAD --stat
git status --short --branch
```

Confirm only the design, plan, backend, dashboard API, page, helper, and relevant tests changed. Do not add existing `.superpowers/`, `artifacts/`, or unrelated plan files.

- [ ] **Step 3: Update and merge local dev**

```bash
git fetch origin
git switch dev
git pull --ff-only origin dev
git merge --no-ff dev-admin-email-filters
```

Stop if unrelated user changes prevent the branch switch.

- [ ] **Step 4: Rerun required validation on local dev**

Run the same backend tests, dashboard build, and dashboard regression suite. Expected: PASS.

- [ ] **Step 5: Push and monitor**

```bash
git push origin dev
```

Monitor all triggered GitHub Actions, Vercel, and Railway deployments until terminal success.

- [ ] **Step 6: Real development acceptance**

Open `https://dev-app.unipost.dev/admin/email` in an authenticated browser and verify:

1. `All emails` is the default.
2. The Email options are complete beyond the current page.
3. Selecting an Email narrows rows exactly.
4. Status and every retained filter still combine correctly.
5. Start-only, end-only, and same-day ranges filter Attempted timestamps correctly.
6. A reversed range shows an inline error and does not send an invalid request.
7. Refresh reloads the list and Email options.

Only report completion after the development deployment and this acceptance pass.
