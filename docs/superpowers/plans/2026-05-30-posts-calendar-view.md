# Posts Calendar View Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a feature-flagged Apple Calendar style month view for Posts, keep the current Posts page as the legacy list view, and make calendar the default when `posts.calendar_view_v1` is enabled.

**Architecture:** Keep legacy risk low by moving the current Posts page into a reusable `PostsLegacyListView` component, then add small route entry points for `/projects/[id]/posts` and `/projects/[id]/posts/list`. Put pure calendar date/status/profile-color logic in a small model utility with tests, and keep the visual calendar component separate from routing and flag gates.

**Tech Stack:** Next.js App Router, React client components, Clerk auth token, existing UniPost API client, Go backend feature flag registry, Node `node:test`, Go `testing`, `npm run build`, Playwright regression when credentials/browsers are available.

---

## File Structure

- `api/internal/featureflags/flags_test.go`: backend TDD coverage for the new feature flag definition and defaults.
- `api/internal/featureflags/flags.go`: `posts.calendar_view_v1` feature flag constant and definition.
- `docs/feature-flags-unleash.md`: flag key, owner, defaults, rollback action, and dependency notes.
- `dashboard/tests/posts-calendar-model.test.mts`: Node unit tests for month grid, date bucketing, status grouping, and color fallback behavior.
- `dashboard/src/components/posts/calendar/calendar-model.ts`: pure calendar/date/status/color helpers used by the view.
- `dashboard/src/components/posts/calendar/posts-calendar-view.tsx`: Apple Calendar style month UI, filters, popover, and CreatePostDrawer integration.
- `dashboard/src/components/posts/list/posts-legacy-list-view.tsx`: mechanically moved legacy list page with a `Calendar View` link and optional focused-post behavior.
- `dashboard/src/app/(dashboard)/projects/[id]/posts/page.tsx`: feature-flagged default entry point, calendar when enabled, legacy when disabled.
- `dashboard/src/app/(dashboard)/projects/[id]/posts/list/page.tsx`: legacy list route, redirecting back to `/posts` when the flag is disabled.
- `dashboard/src/lib/feature-flags.ts`: frontend feature flag key registration.
- `dashboard/src/app/globals.css`: Posts calendar full-height frame adjustment inside the existing dashboard shell.

---

### Task 1: Remote Flag And Backend Definition

**Files:**
- Modify: `api/internal/featureflags/flags_test.go`
- Modify: `api/internal/featureflags/flags.go`
- Modify: `docs/feature-flags-unleash.md`

- [ ] **Step 1: Create or confirm the Unleash flag**

Create this flag in `https://flags.unipost.dev` before wiring it into code:

```text
posts.calendar_view_v1
```

Required configuration:

```text
development: on
production: off
fallback: off in production, on outside production
owner area: Dashboard / Posts
rollback: disable posts.calendar_view_v1 in production
third-party dependency: none
```

- [ ] **Step 2: Write the failing backend test**

Add `TestPostsCalendarViewFlagDefinition` to `api/internal/featureflags/flags_test.go`. It must assert that `PostsCalendarViewV1` is registered, uses `FEATURE_POSTS_CALENDAR_VIEW_V1`, defaults off in production, and defaults on in development.

- [ ] **Step 3: Run the backend test and verify it fails**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/featureflags -run TestPostsCalendarViewFlagDefinition -count=1
```

Expected: fail with `undefined: PostsCalendarViewV1`.

- [ ] **Step 4: Add the backend flag definition**

Add this constant:

```go
PostsCalendarViewV1 Flag = "posts.calendar_view_v1"
```

Add this definition:

```go
PostsCalendarViewV1: {
	Flag:        PostsCalendarViewV1,
	EnvVar:      "FEATURE_POSTS_CALENDAR_VIEW_V1",
	Description: "Controls the Apple Calendar style Posts month view and the /posts/list legacy route split.",
	DefaultEnabled: func(target Target) bool {
		return !isProduction(target.Env)
	},
},
```

- [ ] **Step 5: Run the backend test and verify it passes**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/featureflags -run TestPostsCalendarViewFlagDefinition -count=1
```

Expected: pass.

- [ ] **Step 6: Document the flag**

Add a `posts.calendar_view_v1` section to `docs/feature-flags-unleash.md` with the defaults and rollback action from Step 1.

---

### Task 2: Calendar Model TDD

**Files:**
- Create: `dashboard/tests/posts-calendar-model.test.mts`
- Create: `dashboard/src/components/posts/calendar/calendar-model.ts`

- [ ] **Step 1: Write the failing model tests**

Create `dashboard/tests/posts-calendar-model.test.mts` to cover:

```text
six-week Sunday-first month grid
local date bucketing from scheduled_at, then published_at, then created_at
in_progress status grouping for queued, dispatching, retrying, processing
failed status grouping for failed and partial
archived_at overriding raw status for Archived
All Status including archived while non-archived filters exclude archived
profile branding colors with stable palette fallback
```

- [ ] **Step 2: Run the model tests and verify they fail**

Run:

```bash
cd dashboard
node --test tests/posts-calendar-model.test.mts
```

Expected: fail because `calendar-model.ts` does not exist.

- [ ] **Step 3: Implement the pure model helpers**

Create `dashboard/src/components/posts/calendar/calendar-model.ts` exporting:

```ts
export type CalendarStatusFilter = "all" | "published" | "scheduled" | "in_progress" | "failed" | "draft" | "cancelled" | "archived";
export type CalendarStatusGroup = Exclude<CalendarStatusFilter, "all">;
export type CalendarModelPost = { status: string; scheduled_at?: string | null; published_at?: string | null; created_at?: string | null; archived_at?: string | null };
export type CalendarModelProfile = { id: string; name: string; branding_primary_color?: string | null };
```

Implement `buildMonthGrid`, `bucketPostByLocalDay`, `getPostStatusGroup`, `shouldShowPostForStatusFilter`, `getProfileCalendarColor`, and `formatLocalDateKey` with local browser timezone date math.

- [ ] **Step 4: Run the model tests and verify they pass**

Run:

```bash
cd dashboard
node --test tests/posts-calendar-model.test.mts
```

Expected: pass.

---

### Task 3: Legacy Route Split

**Files:**
- Move: `dashboard/src/app/(dashboard)/projects/[id]/posts/page.tsx` to `dashboard/src/components/posts/list/posts-legacy-list-view.tsx`
- Create: `dashboard/src/app/(dashboard)/projects/[id]/posts/page.tsx`
- Create: `dashboard/src/app/(dashboard)/projects/[id]/posts/list/page.tsx`
- Modify: `dashboard/src/lib/feature-flags.ts`

- [ ] **Step 1: Move the legacy page into a reusable component**

Run:

```bash
mkdir -p dashboard/src/components/posts/list
git mv "dashboard/src/app/(dashboard)/projects/[id]/posts/page.tsx" "dashboard/src/components/posts/list/posts-legacy-list-view.tsx"
```

Then change `export default function PostsPage()` to:

```tsx
type PostsLegacyListViewProps = {
  showCalendarLink?: boolean;
};

export function PostsLegacyListView({ showCalendarLink = false }: PostsLegacyListViewProps) {
```

- [ ] **Step 2: Add the legacy Calendar View link**

Render this near the legacy platform select when `showCalendarLink` is true:

```tsx
{showCalendarLink ? (
  <Link className="posts-view-switch" href={`/projects/${profileId}/posts`}>
    <Calendar size={16} />
    Calendar View
  </Link>
) : null}
```

- [ ] **Step 3: Add focused post behavior for calendar popover deep links**

Read `post` from `useSearchParams()` and expand/scroll the matching row after posts load.

- [ ] **Step 4: Register and wire the frontend feature flag**

Add this key to `dashboard/src/lib/feature-flags.ts`:

```ts
postsCalendarViewV1: "posts.calendar_view_v1",
```

Create a `/posts` client route that renders legacy while loading or disabled, and `PostsCalendarView` when enabled. Create `/posts/list` as the legacy list route that redirects to `/posts` when disabled.

---

### Task 4: Calendar View UI

**Files:**
- Create: `dashboard/src/components/posts/calendar/posts-calendar-view.tsx`
- Modify: `dashboard/src/app/globals.css`

- [ ] **Step 1: Build the data-loading shell**

Use `useAuth`, `useParams`, `useWorkspaceId`, `listSocialPosts`, `listProfiles`, and `listSocialAccounts`. Load posts workspace-wide, load all profiles, and load connected accounts per profile for the Profile and Platforms filters.

- [ ] **Step 2: Add Apple Calendar style layout**

Render one full-height surface with:

```text
left sidebar: Profiles, Platforms, Status
top bar: month title, Day Week Month segmented control, prev Today next, List View, Create +
main: 7-column month grid, six rows, weekday header
```

Do not render the legacy Posts header, subheader, tabs, search, bulk actions, or large Create button in this component.

- [ ] **Step 3: Render posts as profile-colored calendar pills**

Use the first `profile_ids` entry to choose the profile color. Use that color for a left rail and translucent background, and render a separate compact status chip using:

```text
PUB, SCH, RUN, FAIL, DRFT, CNCL, ARCH
```

- [ ] **Step 4: Add filters, Create drawer, and event popover**

Profiles and Platforms are multi-select checkboxes. Status is a single-select list. `Create +` opens the existing `CreatePostDrawer`. Event click opens a compact popover with caption, profile, status, platforms, local time plus timezone, and an `Open in List` link to `/projects/:id/posts/list?post=:postId`.

- [ ] **Step 5: Add full-height dashboard frame styling**

Add a global style hook for `.posts-calendar-fullheight` matching the existing inbox/logs full-height frame pattern, without changing other dashboard pages.

---

### Task 5: Verification

**Files:**
- All changed files

- [ ] **Step 1: Run focused backend tests**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/featureflags -count=1
```

- [ ] **Step 2: Run focused dashboard model tests**

Run:

```bash
cd dashboard
node --test tests/posts-calendar-model.test.mts
```

- [ ] **Step 3: Run dashboard build**

Run:

```bash
cd dashboard
npm run build
```

- [ ] **Step 4: Run backend package tests if backend code changed**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
```

- [ ] **Step 5: Run dashboard regression when possible**

Run:

```bash
cd dashboard
npm run test:regression:dashboard
```

If Playwright browsers or dashboard credentials are unavailable, report the skipped scope and run the public route subset that does not require credentials.
