# UniPost - Posts Calendar View PRD
**将 Posts 从 legacy list 扩展为默认 Calendar 工作台**
Status: Planning
Owner: Dashboard / Publishing
Created: 2026-05-30

---

## 1. Background

当前 UniPost Dashboard 的 Posts 页面以 list view 展示所有 published 和 scheduled content。这个视图适合批量管理、查看状态、展开 post 详情、archive/delete/reschedule，但它不适合做内容排期。

用户需要一个更接近日历的排期体验：

- 默认进入 Posts 时看到 calendar view，而不是 list。
- 可以按月份理解所有 profile 的内容节奏。
- 可以按 Profile、Platform、Status 过滤。
- 同一个 Profile 的 posts 在 calendar 中使用一致颜色。
- Calendar 视觉和交互参考 Apple Calendar，支持当前 Dashboard 的 light/dark theme。
- 现有 list view 不删除，作为 legacy list view 保留。

这个需求会改变 Posts 页面的主要信息架构，因此必须通过 feature flag 控制 rollout，生产环境默认关闭。

---

## 2. Product Goals

1. 将 `/posts` 默认体验从列表管理升级为 calendar-first 内容排期视图。
2. 保留 legacy list view 中已有的管理能力，避免破坏 archive/delete/reschedule/expanded details 等现有工作流。
3. 让用户能在一个 calendar 中看到 workspace 下所有 profiles 的所有 posts。
4. 用 Profile 颜色统一 sidebar 和 calendar event，让内容归属一眼可见。
5. 支持 Profile、Platform、Status 三类过滤，默认展示完整数据。
6. 复用现有 `CreatePostDrawer`，让用户在 calendar 中点击紧凑 `Create +` 后继续使用熟悉的创建流程。
7. 通过 `posts.calendar_view_v1` feature flag 控制 dashboard route、入口和 backend feature surface。

---

## 3. Non-goals

- v1 不实现 day view。
- v1 不实现 week view。
- v1 不实现 drag-and-drop reschedule。
- v1 不实现点击空日期自动预填 schedule time。
- v1 不重做 `CreatePostDrawer` 的 profile/account 选择逻辑。
- v1 不重做 post detail drawer 或 list row expanded details。
- v1 不改变 posts API 的创建、发布、调度、归档、删除语义。
- v1 不让 frontend 直接连接 Unleash 或接收 Unleash token。
- v1 不支持自定义 profile 颜色编辑；颜色先由前端稳定分配。

---

## 4. Feature Flag

### 4.1 Flag key

```text
posts.calendar_view_v1
```

### 4.2 Owner area

Dashboard / Posts / Publishing UX.

### 4.3 Defaults

```text
development: on after flag is created and backend fallback is safe
production: off
fallback: off in production
```

### 4.4 Rollback

Emergency rollback is to disable `posts.calendar_view_v1` in the production Unleash environment.

When disabled:

- `/projects/:id/posts` renders the current legacy list view.
- Calendar entry points are hidden.
- `/projects/:id/posts/list` redirects to `/projects/:id/posts`.
- No data migration or API rollback is required.

### 4.5 Backend contract

Backend must expose the flag through existing:

```text
GET /v1/me/features
```

Dashboard must use the existing feature flag hook and must not connect to Unleash directly.

---

## 5. Route and Navigation Requirements

### 5.1 Flag on

```text
/projects/:id/posts
```

Shows the new calendar view by default.

```text
/projects/:id/posts/list
```

Shows the existing legacy list view.

Calendar page top-right contains:

- `List View` button/link -> `/projects/:id/posts/list`
- compact `Create +` button -> opens existing create post drawer

Legacy list view contains:

- `Calendar View` button/link near the current `All platforms` filter -> `/projects/:id/posts`

### 5.2 Flag off

```text
/projects/:id/posts
```

Shows the existing legacy list view.

Calendar UI and `Calendar View` entry point are not visible.

If a user visits:

```text
/projects/:id/posts/list
```

Redirect to `/projects/:id/posts`. This keeps a single legacy entry point while the flag is off and avoids exposing an inactive calendar/list split.

### 5.3 Dashboard sidebar

The Dashboard sidebar keeps the existing `Posts` nav item. No separate sidebar item is needed for Calendar or List.

---

## 6. Calendar View UX Requirements

### 6.1 Overall page shape

Calendar view must occupy the entire Posts content area.

Do not render legacy list view elements:

- no `Posts` header
- no subtitle
- no legacy tabs row
- no search row
- no bulk archive/delete controls
- no large green `Create` button from the list page
- no legacy bordered page card wrapper inside the calendar surface

The page should feel like a first-class calendar application embedded in the dashboard.

### 6.2 Apple Calendar reference

The visual target is Apple Calendar month view:

- left sidebar with grouped filters
- month title in the main header
- compact top controls
- thin grid lines
- dense but readable day cells
- colored event pills
- dark mode and light mode parity
- no marketing-style hero, no explanatory copy, no decorative cards

The design should match UniPost theme variables and dashboard typography rather than copying Apple system colors directly.

### 6.3 Top bar

Calendar top bar contains:

- current month title, for example `May 2026`
- segmented control: `Day`, `Week`, `Month`
- `Month` is active
- `Day` and `Week` are visible but disabled or inert in v1
- previous month button
- `Today` button
- next month button
- `List View` button
- compact `Create +` button

Behavior:

- Previous/next changes the visible month.
- Today returns to the month containing today.
- `List View` navigates to legacy list view.
- `Create +` opens `CreatePostDrawer`.
- Day/week should not pretend to work. They should either be disabled with clear affordance or remain visually present but non-interactive.

### 6.4 Sidebar

The left sidebar contains three filter groups.

#### Profiles

Show all profiles in the workspace.

Requirements:

- Each profile has a checkbox.
- Each profile has a stable color swatch.
- Default: all profiles selected.
- Unchecking a profile hides that profile's posts from the calendar.
- Profile color in sidebar must match event pill color in the month grid.
- If a post belongs to multiple profiles, use the first known profile for the primary event color in v1.
- If a post has no known `profile_ids`, use a neutral fallback color and include it when all profiles are selected.

#### Platforms

Show platforms from the loaded posts and connected accounts.

Requirements:

- Each platform has a checkbox.
- Default: all platforms selected.
- Unchecking a platform hides posts that target only that platform.
- A post targeting multiple platforms remains visible if at least one selected platform matches.
- Platform labels should use existing platform naming conventions.
- Platform icons may be shown if they fit cleanly and do not add clutter.

#### Status

Show status filter on the calendar sidebar.

Requirements:

- Default: `All Status`.
- Status filter is single-select in v1.
- Available options:
  - `All Status`
  - `Published`
  - `Scheduled`
  - `In Progress`
  - `Failed`
  - `Drafts`
  - `Cancelled`
  - `Archived`
- `Published` includes `published`.
- `Scheduled` includes `scheduled`.
- `In Progress` includes `queued`, `dispatching`, `retrying`, and `processing`.
- `Failed` includes both `failed` and `partial`, matching legacy list behavior.
- `Drafts` includes `draft`.
- `Cancelled` includes `cancelled`.
- `Archived` includes posts with `archived_at`.
- Non-archived status filters should exclude archived posts unless `Archived` is selected.
- `All Status` includes every returned post, including archived posts and in-flight statuses, because calendar is a complete workspace timeline.

### 6.5 Mini month navigator

If space allows, sidebar bottom should include a small month navigator similar to Apple Calendar.

Requirements:

- Shows current visible month.
- Highlights today.
- Highlights selected/visible month days subtly.
- Clicking a date is optional in v1.
- This must not crowd the primary filters on smaller screens.

The mini navigator can be deferred if it creates layout risk, but the main sidebar filter groups are required.

---

## 7. Month Grid Requirements

### 7.1 Date range

Month view displays a full 6-week grid when needed, including leading and trailing days from adjacent months.

Requirements:

- Week starts on Sunday for v1, matching the Apple Calendar screenshot and current US locale expectation.
- Day headers: `Sun`, `Mon`, `Tue`, `Wed`, `Thu`, `Fri`, `Sat`.
- Adjacent month days are visually muted.
- Today is highlighted.
- The current visible month title updates when navigating.

### 7.2 Post placement date

Determine the calendar date for each post:

1. If `status === "scheduled"` and `scheduled_at` exists, use `scheduled_at`.
2. Else if `published_at` exists, use `published_at`.
3. Else use `created_at`.

Rationale:

- Scheduled posts should appear where they are planned.
- Published posts should appear where they went live.
- Drafts, failed posts, and records without publish time still need a visible date.

Time zone:

- v1 calculates calendar day boundaries in the viewer's browser local timezone.
- This matches the current dashboard pattern for client-side date bucketing and avoids inventing workspace/profile timezone semantics that do not exist in the current API.
- Event popovers must display the formatted date/time and the timezone used, preferably the IANA timezone from `Intl.DateTimeFormat().resolvedOptions().timeZone`.
- Future workspace/profile timezone support can replace this rule only after a timezone field exists in the backend contract.

### 7.3 Event pill content

Each calendar event pill should show:

- short caption text, with fallback `(no caption)`
- profile color
- compact status signal when useful
- optional platform icon/count if it stays readable

Event pills must:

- fit inside day cells without changing grid dimensions
- truncate long captions
- preserve accessible title/label text
- work in dark and light themes

Profile color and status signal must be layered, not overloaded:

- Profile color is the primary event ownership signal, shown as the pill tint and/or left accent rail.
- Status is a separate compact text label or chip inside the pill, not just a color change.
- Recommended compact labels:
  - `PUB` for `published`
  - `SCH` for `scheduled`
  - `RUN` for `queued`, `dispatching`, `retrying`, `processing`
  - `FAIL` for `failed` and `partial`
  - `DRFT` for `draft`
  - `CNCL` for `cancelled`
  - `ARCH` for posts with `archived_at`
- Archived posts should also be visually muted, but the `ARCH` label must remain present so color is not the only signal.
- Failed/partial posts may use a stronger border or status chip treatment, but profile color remains the event ownership color.

### 7.4 Overflow behavior

If a day has more posts than can fit:

- show the first visible posts in a stable order
- show a `+N more` affordance
- v1 may open a compact popover or simply expand the day cell area only if it does not break the month grid

Recommended v1:

- fixed day cell height
- show `+N more` text after the visible pills
- no complex popover unless implementation remains low-risk

### 7.5 Ordering

Within a day, order posts by:

1. scheduled/published/created timestamp ascending
2. status priority if timestamps are identical
3. caption as final stable tie-breaker

### 7.6 Event interaction

Clicking a calendar event opens a compact event popover in the calendar view.

The popover should show:

- full caption
- profile name
- status
- scheduled/published/created time used for calendar placement, including timezone
- target platforms
- `Open in List` action

The `Open in List` action navigates to legacy list view with enough context to reveal or focus the selected post.

The popover should not duplicate the full legacy expanded result grid in v1. Detailed per-platform result troubleshooting remains owned by the legacy list view.

---

## 8. Data Requirements

### 8.1 Existing frontend data

The current Posts page already loads:

- `listSocialPosts(token)` workspace-wide posts
- `listProfiles(token)` workspace profiles
- `listSocialAccounts(token, profileId)` current profile accounts

Calendar requires workspace-wide profile context and profile/platform filters.

### 8.2 Profiles

Use `listProfiles(token)` for sidebar Profiles.

Required fields:

- `id`
- `name`

Optional fields:

- `branding_primary_color` may be used as an input to profile color selection if it is suitable.

### 8.3 Posts

Use `listSocialPosts(token)` for calendar events.

Required fields:

- `id`
- `caption`
- `status`
- `created_at`
- `scheduled_at`
- `published_at`
- `archived_at`
- `profile_ids`
- `target_platforms`
- `results`

### 8.4 Platforms

Platform filter values should derive from:

- `post.results[].platform`
- `post.target_platforms`
- active connected accounts when available

The filter should not hardcode only the existing legacy list platforms if the API returns newer platforms.

### 8.5 Create drawer accounts

Calendar `Create +` should reuse `CreatePostDrawer`.

The drawer currently supports profile switching and loads accounts for the selected profile after opening. In v1, Calendar does not need to pre-load all accounts across all profiles for the drawer.

Required behavior:

- Opening from Calendar works.
- User can select a Profile inside the drawer.
- User can select connected accounts for that Profile.
- On successful create, calendar data refreshes silently.

---

## 9. Profile Color Requirements

### 9.1 Color palette

Use an Apple Calendar-inspired set of distinct, accessible colors, tuned for UniPost themes.

Suggested base palette:

```text
blue
green
orange
pink
purple
teal
yellow
red
indigo
mint
```

Design constraints:

- Avoid a one-note purple/blue palette.
- Keep saturation controlled so dark mode does not glow.
- Event background should be tinted; text must remain readable.
- Sidebar swatch and event pill must use the same profile color token.

### 9.2 Stable assignment

Profile color should be stable across renders.

Recommended rule:

- If `profile.branding_primary_color` exists and passes contrast/saturation constraints, use it.
- Otherwise derive a color index from `profile.id`.

Do not store profile color in backend in v1.

---

## 10. Status Display Requirements

Calendar status display must cover every status known to the current Posts list and backend queue flow.

Status groups:

| Calendar filter | Raw post status / field |
| --- | --- |
| `All Status` | all returned posts, including archived |
| `Published` | `status === "published"` |
| `Scheduled` | `status === "scheduled"` |
| `In Progress` | `status` in `queued`, `dispatching`, `retrying`, `processing` |
| `Failed` | `status` in `failed`, `partial` |
| `Drafts` | `status === "draft"` |
| `Cancelled` | `status === "cancelled"` |
| `Archived` | `archived_at` is present |

Notes:

- `Archived` is not a raw `status` value. It is a calendar filter category derived from `archived_at`.
- If a post has `archived_at`, it should appear in `All Status` and `Archived`.
- If a post has `archived_at`, it should not appear in specific non-archived status filters.
- Unknown future statuses should remain visible under `All Status` and use a generic status label until explicitly mapped.

---

## 11. Legacy List View Requirements

Legacy list view must remain functionally equivalent to the current page.

Keep:

- status tabs
- search
- platform filter
- bulk select
- archive/delete/restore
- row expansion
- platform results grid
- retry actions
- reschedule dialog
- existing create drawer entry
- activation/tutorial query handling

Add:

- `Calendar View` button/link near `All platforms`.

Feature flag behavior:

- Only show `Calendar View` link when `posts.calendar_view_v1` is enabled.
- If flag is disabled, legacy list remains exactly the `/posts` experience.

---

## 12. Migration and Regression Risk

The legacy Posts page is large and stateful. The implementation must treat preserving legacy behavior as a first-class requirement, not as incidental cleanup.

Required implementation strategy:

- Use a minimal-risk route split.
- Move the existing legacy page behavior into a `PostsLegacyListView` component with the smallest practical mechanical change.
- Keep the legacy component's existing state, handlers, drawer behavior, query handling, row expansion, reschedule dialog, and action flows intact.
- Let `/projects/:id/posts/list` render `PostsLegacyListView` only when `posts.calendar_view_v1` is enabled.
- Let `/projects/:id/posts` render either calendar or `PostsLegacyListView` based on the flag.
- Do not introduce a broad shared posts hook in v1 unless it is needed to remove direct duplication and does not alter legacy behavior.
- Do not combine the calendar implementation with unrelated cleanup of the legacy list.

Regression acceptance:

- Legacy list must pass manual regression for tabs, search, platform filter, bulk select, archive/delete/restore, row expansion, retry, reschedule, create drawer, and activation/tutorial query handling.
- `npm run test:regression:dashboard` is required when Playwright browsers are installed.
- If Playwright browsers are not installed, the skipped regression check and reason must be reported before implementation is considered ready.

---

## 13. States and Empty Cases

### 13.1 Loading

Calendar should show a skeleton that matches the calendar layout:

- sidebar skeleton rows
- month grid skeleton blocks

Avoid a generic spinner as the only loading state.

### 13.2 No posts

If there are no posts in the workspace:

- show empty calendar grid
- keep filters visible
- allow `Create +`
- do not show a large marketing-style empty state

### 13.3 Filters hide all posts

If posts exist but current filters hide all posts:

- keep the calendar grid visible
- show a subtle in-grid empty indicator
- user can adjust filters from sidebar

### 13.4 No profiles

If no profiles exist:

- sidebar Profiles group shows an empty state.
- `Create +` should still follow existing drawer behavior, which may guide the user through profile/account setup if supported.

### 13.5 API failure

If posts/profiles fail to load:

- show inline error inside calendar shell
- keep route usable
- allow retry if practical
- do not fail into a blank page

---

## 14. Responsive Requirements

### 14.1 Desktop

Primary target is desktop dashboard.

Desktop layout:

- fixed-width left sidebar
- main calendar grid fills remaining width
- top bar remains single row where practical

### 14.2 Tablet and small desktop

Requirements:

- sidebar may narrow
- top controls can wrap into two rows
- event text truncates cleanly
- month grid remains readable

### 14.3 Mobile

The dashboard is not primarily mobile-first, but the page must not break.

Minimum requirements:

- no horizontal overflow outside intended scroll containers
- sidebar can stack above calendar or become a collapsible filter panel
- top controls remain tappable
- text must not overlap

---

## 15. Accessibility Requirements

- All buttons must have accessible names.
- Disabled Day/Week controls must expose disabled state.
- Checkbox filters must use real inputs or equivalent accessible controls.
- Calendar event pills must expose full caption, status, profile, platform, date, and timezone in accessible text.
- Keyboard users must be able to:
  - navigate to Create
  - navigate to List View
  - change filters
  - use previous/next/today
- Color cannot be the only status signal.

---

## 16. Analytics and Observability

No new analytics pipeline is required for v1.

Useful optional client events, if UniPost already has a dashboard analytics pattern:

- calendar view opened
- list view opened from calendar
- calendar create clicked
- calendar month changed
- profile filter changed
- platform filter changed
- status filter changed

Do not add a new analytics dependency only for this feature.

---

## 17. Acceptance Criteria

### 17.1 Flag and routing

- With `posts.calendar_view_v1` off, `/projects/:id/posts` shows the current legacy list view.
- With `posts.calendar_view_v1` off, `/projects/:id/posts/list` redirects to `/projects/:id/posts`.
- With `posts.calendar_view_v1` on, `/projects/:id/posts` shows calendar view by default.
- With flag on, `/projects/:id/posts/list` shows legacy list view.
- Calendar has `List View` button that navigates to list view.
- Legacy list has `Calendar View` button near `All platforms`.

### 17.2 Calendar layout

- Calendar page does not render legacy Posts title/subtitle/tabs/search/bulk toolbar.
- Calendar fills the Posts content area.
- Month grid renders current month with adjacent month days.
- Previous/next/today controls work.
- Day/week controls are visible but not active in v1.

### 17.3 Data display

- Calendar shows posts from all workspace profiles.
- Calendar includes all returned posts, including published, scheduled, queued, dispatching, retrying, processing, failed, partial, draft, cancelled, archived, and unknown future statuses.
- Archived is treated as an `archived_at`-derived filter category, not a raw status value.
- Scheduled posts use `scheduled_at`.
- Published posts use `published_at`.
- Other posts use `created_at`.
- Calendar date placement uses the viewer's browser local timezone in v1.
- Profile colors match sidebar and event pills.
- Status signal is shown as a separate compact label/chip, not only as color.

### 17.4 Filters

- Profiles default all selected.
- Platforms default all selected.
- Status defaults to `All Status`.
- Profile filter hides/shows matching posts.
- Platform filter hides/shows matching posts.
- Status filter matches legacy semantics, including `Failed = failed + partial`.
- `In Progress` includes queued, dispatching, retrying, and processing posts.
- `Cancelled` includes cancelled posts.
- `All Status` includes archived posts because calendar is the complete workspace timeline.
- Specific non-archived status filters exclude archived posts.
- `Archived` shows only archived posts.

### 17.5 Legacy list regression

- Legacy list tabs, search, platform filter, bulk select, archive/delete/restore, row expansion, platform results grid, retry, reschedule, create drawer, and activation/tutorial query handling remain functionally equivalent.
- The implementation uses the minimal-risk route split and avoids broad legacy refactors.
- Dashboard regression tests are run when Playwright browsers are installed.

### 17.6 Create flow

- `Create +` opens existing `CreatePostDrawer`.
- User can select profile/accounts in drawer.
- After successful create, calendar refreshes and the new post appears on the correct date.

### 17.7 Event interaction

- Clicking a calendar event opens a compact popover.
- Popover shows full caption, profile, status, date/time, timezone, and target platforms.
- Popover includes an `Open in List` action.
- Full platform troubleshooting remains in legacy list view.

### 17.8 Theme and polish

- Light theme and dark theme both match Dashboard variables.
- No text overlap in month grid, sidebar, or top bar.
- Event pills truncate cleanly.
- Calendar remains usable at common desktop widths.

---

## 18. QA Plan

### 18.1 Local build

From `dashboard/`:

```text
npm run build
```

### 18.2 Regression checks

If Playwright browsers are installed:

```text
npm run test:regression:dashboard
```

Required because this touches dashboard routing, shared shell behavior, and Posts UI.

### 18.3 Manual QA

Run Dashboard locally and verify:

- flag off -> `/posts` legacy list
- flag off -> `/posts/list` redirects to `/posts`
- flag on -> `/posts` calendar
- `Calendar View` from list
- `List View` from calendar
- legacy list workflows still work after route split
- Create drawer opens from calendar
- post creation refreshes calendar
- profile filters work
- platform filters work
- status filter works
- viewer-local timezone is used for day placement
- event popover shows timezone
- empty workspace does not break
- dark theme visual QA
- light theme visual QA
- narrow viewport does not overlap text

### 18.4 Dev environment QA

When deployed to development:

- use `https://dev.unipost.dev`
- verify `posts.calendar_view_v1` on in development
- verify calendar with real workspace profiles/accounts/posts
- do not validate this feature against production domains during development rollout

---

## 19. Rollout Plan

1. Create `posts.calendar_view_v1` in Unleash.
2. Add backend flag definition and expose through `/v1/me/features`.
3. Add dashboard flag key.
4. Build calendar view behind flag.
5. Keep legacy list route available.
6. Verify locally with flag on/off.
7. Deploy to development with flag on.
8. QA on `https://dev.unipost.dev`.
9. Keep production flag off until product review passes.
10. Enable production for internal/admin users or limited workspace segment first if Unleash targeting is available.
11. Roll back by disabling production flag.

---

## 20. Implementation Notes

These are guidance notes, not final engineering plan.

Recommended structure:

- Move current list page logic into a reusable legacy list component with minimal mechanical changes.
- Add a calendar page/component for month view.
- Use a small shared posts data hook only if it reduces duplication without turning into a broad refactor or changing legacy behavior.
- Put calendar date math in a small utility with focused unit coverage for month boundaries, leading/trailing days, today highlighting, and local-time day bucketing.
- Keep calendar-specific CSS scoped to calendar classes.
- Prefer CSS grid for the month grid.
- Use existing `lucide-react` icons.
- Avoid adding new date libraries unless native date helpers become too error-prone.

Potential route structure:

```text
dashboard/src/app/(dashboard)/projects/[id]/posts/page.tsx
dashboard/src/app/(dashboard)/projects/[id]/posts/list/page.tsx
dashboard/src/components/posts/calendar/posts-calendar-view.tsx
dashboard/src/components/posts/list/posts-legacy-list-view.tsx
```

The exact file layout can change during implementation if it better matches the existing codebase.

---

## 21. V1 Product Decisions

1. `All Status` includes archived posts by default.
   - The calendar is a complete workspace timeline.
   - Legacy list keeps archived separated in its own tab for management workflows.

2. Clicking a calendar event opens a compact calendar popover.
   - The popover gives quick context.
   - Deep per-platform troubleshooting remains in legacy list view.
   - `Open in List` provides the bridge to legacy details.

3. Calendar search is not included in v1.
   - Sidebar filters cover the primary calendar use case.
   - Legacy list retains search.

4. Profile colors may use `branding_primary_color` only when it passes contrast and visual constraints.
   - Otherwise use the stable built-in palette.
   - Do not persist calendar colors to backend in v1.

5. Calendar day placement uses the viewer's browser local timezone.
   - Workspace/profile timezone fields do not exist in the current API.
   - The popover must show the timezone used.

6. `/posts/list` redirects to `/posts` when the calendar flag is off.
   - This keeps flag-off behavior simple and avoids exposing an inactive split route.

7. Legacy migration uses a minimal route split.
   - Move legacy behavior with minimal mechanical changes.
   - Avoid shared-hook refactors unless strictly necessary.

---

## 22. Product Decision Summary

Build a feature-flagged Apple Calendar-inspired month view for Posts. Make calendar the default `/posts` experience only when `posts.calendar_view_v1` is enabled. Preserve the current list as legacy list view at `/posts/list` while the flag is on; redirect `/posts/list` back to `/posts` while the flag is off. Calendar shows all workspace profiles, all statuses/status-derived categories, and supports Profile, Platform, and Status filters from the left sidebar. Profile colors unify sidebar and event pills, while status remains a separate compact text signal. Calendar date placement uses the viewer's browser local timezone in v1. Creation reuses the existing create drawer. Production rollout is controlled entirely through Unleash.
