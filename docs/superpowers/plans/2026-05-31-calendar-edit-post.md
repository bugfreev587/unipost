# Calendar Edit Post Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users edit draft and scheduled posts directly from Calendar instead of jumping to List View.

**Architecture:** Extend the existing post composer into reusable create/edit modes, then render edit mode from an anchored Calendar inspector. Backend `PATCH /v1/posts/{id}` will accept the same publish payload for both drafts and scheduled posts while preserving lifecycle patches and optimistic state guards.

**Tech Stack:** Go API with sqlc/pgx, Next.js 16 App Router, React 19, existing dashboard CSS/Tailwind utilities, lucide-react icons.

---

## File Structure

- Modify `api/internal/db/queries/social_posts.sql`: widen the content update query to permit both `draft` and `scheduled` rows and update `profile_ids`.
- Regenerate or manually update `api/internal/db/social_posts.sql.go`: reflect the query and `ProfileIds` parameter used by handlers.
- Modify `api/internal/handler/social_posts_drafts.go`: route scheduled posts through full content update, share canonical payload conversion, and keep lifecycle patches intact.
- Modify `api/internal/handler/social_posts_patch_test.go`: add failing tests for editable status classification and SQL parameter construction.
- Modify `dashboard/src/lib/api.ts`: add `updateSocialPost`, expose metadata needed to hydrate edit mode, and keep `rescheduleSocialPost` compatibility if still used by list view.
- Modify `dashboard/src/components/posts/create-post/use-create-post-form.ts`: add an initializer that can hydrate form state from a saved post and accounts.
- Modify `dashboard/src/components/posts/create-post/create-post-drawer.tsx`: extract a reusable composer body or add mode props with a non-sheet render path for Calendar edit.
- Modify `dashboard/src/components/posts/calendar/posts-calendar-view.tsx`: replace `Open in List` with `Edit`, open the anchored editor for editable posts, and refresh data after save.

---

### Task 1: Backend Scheduled Content Edit

**Files:**
- Modify: `api/internal/handler/social_posts_patch_test.go`
- Modify: `api/internal/handler/social_posts_drafts.go`
- Modify: `api/internal/db/queries/social_posts.sql`
- Modify: `api/internal/db/social_posts.sql.go`

- [ ] **Step 1: Write the failing tests**

Add tests that define the desired pure behavior before changing production code:

```go
func TestCanEditSocialPostContent(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: "draft", want: true},
		{status: "scheduled", want: true},
		{status: "publishing", want: false},
		{status: "published", want: false},
		{status: "failed", want: false},
		{status: "cancelled", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := canEditSocialPostContent(tt.status); got != tt.want {
				t.Fatalf("canEditSocialPostContent(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestBuildContentUpdateParamsIncludesProfilesForScheduledPost(t *testing.T) {
	scheduledAt := time.Date(2026, 6, 1, 18, 30, 0, 0, time.UTC)
	posts := []platform.PlatformPostInput{{
		AccountID: "acct_linkedin",
		Caption:   "updated caption",
		MediaURLs: []string{"https://cdn.example/image.jpg"},
	}}
	metadata, err := platform.EncodePostMetadata(posts)
	if err != nil {
		t.Fatal(err)
	}

	params := buildSocialPostContentUpdateParams("post_1", "ws_1", posts, metadata, &scheduledAt, []string{"prof_1"})

	if params.ID != "post_1" || params.WorkspaceID != "ws_1" {
		t.Fatalf("unexpected ids: %#v", params)
	}
	if !params.Caption.Valid || params.Caption.String != "updated caption" {
		t.Fatalf("caption = %#v, want updated caption", params.Caption)
	}
	if len(params.MediaUrls) != 1 || params.MediaUrls[0] != "https://cdn.example/image.jpg" {
		t.Fatalf("media urls = %#v", params.MediaUrls)
	}
	if !bytes.Equal(params.Metadata, metadata) {
		t.Fatalf("metadata mismatch")
	}
	if !params.ScheduledAt.Valid || !params.ScheduledAt.Time.Equal(scheduledAt) {
		t.Fatalf("scheduled_at = %#v, want %s", params.ScheduledAt, scheduledAt)
	}
	if len(params.ProfileIds) != 1 || params.ProfileIds[0] != "prof_1" {
		t.Fatalf("profile_ids = %#v, want prof_1", params.ProfileIds)
	}
}
```

- [ ] **Step 2: Run tests and confirm failure**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestCanEditSocialPostContent|TestBuildContentUpdateParamsIncludesProfilesForScheduledPost' -count=1
```

Expected: FAIL because `canEditSocialPostContent` and `buildSocialPostContentUpdateParams` do not exist yet.

- [ ] **Step 3: Implement backend helpers and route**

Add helpers in `api/internal/handler/social_posts_drafts.go`:

```go
func canEditSocialPostContent(status string) bool {
	return status == "draft" || status == "scheduled"
}

func buildSocialPostContentUpdateParams(postID, workspaceID string, posts []platform.PlatformPostInput, metaJSON []byte, scheduledAt *time.Time, profileIDs []string) db.UpdateDraftContentParams {
	canonicalCaption := pgtype.Text{}
	canonicalMedia := []string{}
	if len(posts) > 0 {
		if posts[0].Caption != "" {
			canonicalCaption = pgtype.Text{String: posts[0].Caption, Valid: true}
		}
		if posts[0].MediaURLs != nil {
			canonicalMedia = posts[0].MediaURLs
		}
	}
	scheduledAtParam := pgtype.Timestamptz{}
	if scheduledAt != nil {
		scheduledAtParam = pgtype.Timestamptz{Time: *scheduledAt, Valid: true}
	}
	return db.UpdateDraftContentParams{
		ID:          postID,
		WorkspaceID: workspaceID,
		Caption:     canonicalCaption,
		MediaUrls:   canonicalMedia,
		Metadata:    metaJSON,
		ScheduledAt: scheduledAtParam,
		ProfileIds:  profileIDs,
	}
}
```

Change `UpdateDraft` so scheduled posts no longer branch to `reschedulePost` for content payloads. Only lifecycle-style payloads keep using lifecycle handling; otherwise `draft` and `scheduled` both parse the full publish body and call `UpdateDraftContent`.

- [ ] **Step 4: Widen SQL guard**

Change `UpdateDraftContent`:

```sql
UPDATE social_posts
SET caption = $3,
    media_urls = $4,
    metadata = $5,
    scheduled_at = $6,
    profile_ids = $7
WHERE id = $1 AND workspace_id = $2 AND status IN ('draft', 'scheduled')
RETURNING *;
```

Update the generated Go wrapper parameter struct and QueryRow call to include `ProfileIds []string`.

- [ ] **Step 5: Run backend test target**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestCanEditSocialPostContent|TestBuildContentUpdateParamsIncludesProfilesForScheduledPost|TestParseSocialPostLifecyclePatch' -count=1
```

Expected: PASS.

---

### Task 2: Dashboard API and Form Hydration

**Files:**
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/components/posts/create-post/use-create-post-form.ts`

- [ ] **Step 1: Add client update API**

Add this client helper:

```ts
export async function updateSocialPost(
  token: string,
  postId: string,
  data: CreateSocialPostPayload
): Promise<ApiResponse<SocialPost>> {
  return request(`/v1/posts/${postId}`, token, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}
```

Extend `SocialPost` with an optional metadata-derived post list if the backend response already exposes it; otherwise hydrate from `caption`, `media_urls`, and target platforms as a best-effort fallback.

- [ ] **Step 2: Add form initializer**

Add a `hydrateFromPost` action to `useCreatePostForm`:

```ts
const hydrateFromPost = useCallback((post: SocialPost, availableAccounts: SocialAccount[]) => {
  const accountIds = deriveEditableAccountIds(post, availableAccounts);
  setSelectedAccountIds(new Set(accountIds));
  setMainContent(post.caption || "");
  setPublishMode(post.status === "scheduled" ? "schedule" : "draft");
  setScheduledAt(post.scheduled_at ? toDateTimeLocalValue(post.scheduled_at) : "");
  setOverrides(deriveOverridesFromPost(post, availableAccounts));
  setMediaItems(deriveExistingMediaItems(post));
  setUploadCache(deriveExistingMediaCache(post));
}, []);
```

Add local helper functions in the same file for account IDs, overrides, date conversion, and existing media placeholders.

- [ ] **Step 3: Run dashboard build once hydration compiles**

Run:

```bash
cd dashboard
npm run build
```

Expected: PASS. If it fails on missing `File` placeholders for existing media, adjust the edit payload builder to preserve existing media IDs without constructing fake `File` instances.

---

### Task 3: Shared Composer Edit Mode

**Files:**
- Modify: `dashboard/src/components/posts/create-post/create-post-drawer.tsx`

- [ ] **Step 1: Add edit props**

Extend props:

```ts
type ComposerMode = "create" | "edit";

interface CreatePostDrawerProps {
  mode?: ComposerMode;
  editPost?: SocialPost;
  renderMode?: "sheet" | "inline";
  onSaved?: (postId?: string) => void | Promise<void>;
}
```

Use `mode ?? "create"` and `renderMode ?? "sheet"` so current create flows remain unchanged.

- [ ] **Step 2: Hydrate edit state when opened**

When `mode === "edit"` and `editPost` is present, call `form.hydrateFromPost(editPost, allLoadedAccounts)` after accounts are loaded. Guard with a ref keyed by `editPost.id` so the user is not rehydrated while typing.

- [ ] **Step 3: Use PATCH for edit save**

In `handleSubmit`, call:

```ts
const response = mode === "edit" && editPost
  ? await updateSocialPost(token, editPost.id, payload)
  : await createSocialPost(token, payload);
await (onSaved || onCreated)(response.data.id);
```

Change labels in edit mode to `Save changes`, `Saving...`, and header title `Edit post`.

- [ ] **Step 4: Add inline render path**

For `renderMode === "inline"`, render the composer shell content without `<Sheet>` and without the right-side fixed drawer assumptions. Preserve the same form sections, footer, validation, media preview dialog, and discard confirmation.

---

### Task 4: Calendar Anchored Editor

**Files:**
- Modify: `dashboard/src/components/posts/calendar/posts-calendar-view.tsx`

- [ ] **Step 1: Replace action copy**

Remove the `Link` to `/posts/list?post=...` from `EventPopover` and render a button:

```tsx
<button type="button" className="posts-calendar-open-list" onClick={onEdit} disabled={!editable}>
  {editable ? "Edit" : "View only"}
</button>
```

- [ ] **Step 2: Track edit target**

Add state:

```ts
const [editingPostTarget, setEditingPostTarget] = useState<SelectedPostTarget | null>(null);
```

Clicking `Edit` sets `editingPostTarget` from the selected target and keeps the selected post ID.

- [ ] **Step 3: Render anchored editor**

Add `CalendarEditInspector` that uses the same placement helper with a larger fallback size and renders the composer with:

```tsx
<CreatePostDrawer
  open
  renderMode="inline"
  mode="edit"
  editPost={post}
  accounts={accountsForPost}
  workspaceId={workspaceId}
  profileName={profile?.name}
  getToken={getToken}
  onOpenChange={(open) => { if (!open) closeEditor(); }}
  onCreated={handleCreated}
  onSaved={handleEdited}
/>
```

- [ ] **Step 4: Refresh after save**

`handleEdited` calls `loadData()`, clears edit state, and closes the compact popover.

---

### Task 5: Verification, Merge, Push, Remote Check

**Files:**
- No source files beyond previous tasks.

- [ ] **Step 1: Local backend verification**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: PASS.

- [ ] **Step 2: Local dashboard verification**

Run:

```bash
cd dashboard
npm run build
```

Expected: PASS.

- [ ] **Step 3: Merge to local dev**

Run:

```bash
git fetch origin
git switch dev
git pull --ff-only origin dev
git merge --no-ff dev-calendar-edit-post
```

- [ ] **Step 4: Re-run validation on local dev**

Run the same backend and dashboard commands from Steps 1 and 2.

- [ ] **Step 5: Push and monitor**

Run:

```bash
git push origin dev
```

Monitor GitHub Actions, Vercel, and Railway triggered by the push until they finish.

- [ ] **Step 6: Verify dev deployment**

Open and test:

- `https://dev.unipost.dev`
- `https://dev-app.unipost.dev`
- `https://dev-api.unipost.dev`

Confirm the Calendar edit flow works end to end. If any issue appears, fix it on `dev-calendar-edit-post`, repeat validation, merge to `dev`, push, and monitor again.
