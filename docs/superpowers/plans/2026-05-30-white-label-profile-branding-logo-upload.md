# White-label Profile Branding Logo Upload Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let white-label customers configure hosted Connect profile branding, including direct R2-backed logo upload, from `Accounts -> White-label`.

**Architecture:** Keep Profile branding fields as the source of truth. Add a long-lived `branding/` R2 storage path and profile `branding_logo_storage_key` column for uploaded logos, with dedicated upload/delete handlers on the Profile API. Extend the existing White-label dashboard page with branding form state, upload controls, and a live Connect preview above the platform credentials section.

**Tech Stack:** Go/chi/pgx/sqlc, Cloudflare R2 via the existing S3-compatible storage client, Next.js App Router, React client components, existing dashboard CSS utilities, lucide-react icons.

---

## File Structure

- Modify `api/internal/storage/media.go`
  - Add `BrandingLogoKey`, `PublicURL`, and `PutObject` helpers for long-lived branding assets.
- Modify `api/internal/storage/media_test.go`
  - Add storage key and nil-client tests for the new helpers.
- Add `api/internal/db/migrations/078_profile_branding_logo_storage_key.sql`
  - Add nullable `profiles.branding_logo_storage_key`.
- Modify `api/internal/db/queries/profiles.sql`
  - Return `branding_logo_storage_key` in profile selects.
  - Add `UpdateProfileBrandingLogo` and `ClearProfileBrandingLogo`.
- Regenerate sqlc output in `api/internal/db/models.go` and `api/internal/db/profiles.sql.go`.
- Modify `api/internal/handler/projects.go`
  - Add branding logo upload/delete handler methods.
  - Add upload validation helpers.
  - Preserve existing JSON branding update behavior.
- Modify `api/internal/handler/projects_branding_test.go`
  - Add helper tests for accepted/rejected logo uploads and safe delete key scoping.
- Modify `api/cmd/api/main.go`
  - Wire storage into `ProfileHandler`.
  - Mount `POST/DELETE /v1/profiles/{id}/branding/logo`.
- Modify `dashboard/src/lib/api.ts`
  - Add `uploadProfileLogo` and `deleteProfileLogo`.
- Modify `dashboard/src/app/(dashboard)/projects/[id]/accounts/native/page.tsx`
  - Load the current profile.
  - Add Hosted Connect Branding section above platform credentials.
  - Add file upload, remove logo, display name, primary color, attribution toggle, and live preview.

---

### Task 1: R2 Branding Storage and Database Shape

**Files:**
- Modify: `api/internal/storage/media.go`
- Modify: `api/internal/storage/media_test.go`
- Add: `api/internal/db/migrations/078_profile_branding_logo_storage_key.sql`
- Modify: `api/internal/db/queries/profiles.sql`
- Generated: `api/internal/db/models.go`
- Generated: `api/internal/db/profiles.sql.go`

- [ ] **Step 1: Write failing storage tests**

Add to `api/internal/storage/media_test.go`:

```go
func TestBrandingLogoKey(t *testing.T) {
	cases := []struct {
		workspaceID string
		profileID   string
		ext         string
		wantPrefix  string
		wantSuffix  string
	}{
		{"ws_123", "pr_456", ".png", "branding/ws_123/pr_456/logo_", ".png"},
		{"ws_123", "pr_456", "jpg", "branding/ws_123/pr_456/logo_", ".jpg"},
	}
	for _, c := range cases {
		got := BrandingLogoKey(c.workspaceID, c.profileID, c.ext)
		if !strings.HasPrefix(got, c.wantPrefix) || !strings.HasSuffix(got, c.wantSuffix) {
			t.Fatalf("BrandingLogoKey(%q,%q,%q) = %q, want prefix %q suffix %q", c.workspaceID, c.profileID, c.ext, got, c.wantPrefix, c.wantSuffix)
		}
	}
}

func TestNilClientBrandingHelpers(t *testing.T) {
	var c *Client
	if err := c.PutObject(context.TODO(), "branding/ws/pr/logo.png", strings.NewReader("x"), "image/png", "public, max-age=1"); err != ErrNotConfigured {
		t.Errorf("PutObject on nil: want ErrNotConfigured, got %v", err)
	}
	if got := c.PublicURL("branding/ws/pr/logo.png"); got != "" {
		t.Errorf("PublicURL on nil = %q, want empty string", got)
	}
}
```

Update imports to include `strings`.

- [ ] **Step 2: Run storage tests and verify red**

Run:

```bash
cd api && go test ./internal/storage -run 'TestBrandingLogoKey|TestNilClientBrandingHelpers'
```

Expected: fails because `BrandingLogoKey`, `PutObject`, or `PublicURL` is undefined.

- [ ] **Step 3: Implement storage helpers**

Add to `api/internal/storage/media.go`:

```go
const BrandingPrefix = "branding/"

func BrandingLogoKey(workspaceID, profileID, ext string) string {
	if ext != "" && ext[0] != '.' {
		ext = "." + ext
	}
	return path.Join("branding", workspaceID, profileID, "logo_"+uuid.NewString()+ext)
}

func (c *Client) PublicURL(key string) string {
	if c == nil {
		return ""
	}
	return c.publicBase + "/" + strings.TrimLeft(key, "/")
}

func (c *Client) PutObject(ctx context.Context, key string, body io.Reader, contentType string, cacheControl string) error {
	if c == nil {
		return ErrNotConfigured
	}
	if cacheControl == "" {
		cacheControl = "public, max-age=31536000, immutable"
	}
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(c.bucket),
		Key:          aws.String(key),
		Body:         body,
		ContentType:  aws.String(contentType),
		CacheControl: aws.String(cacheControl),
	})
	if err != nil {
		return fmt.Errorf("storage: put object: %w", err)
	}
	return nil
}
```

Also add imports for `io`, `strings`, and `github.com/google/uuid` if missing.

- [ ] **Step 4: Run storage tests and verify green**

Run:

```bash
cd api && go test ./internal/storage -run 'TestBrandingLogoKey|TestNilClientBrandingHelpers'
```

Expected: tests pass.

- [ ] **Step 5: Add database migration and sqlc queries**

Create `api/internal/db/migrations/078_profile_branding_logo_storage_key.sql`:

```sql
-- +goose Up
--
-- R2-backed hosted Connect logo uploads. External logo URLs remain
-- represented by branding_logo_url with this key left NULL.

ALTER TABLE profiles
  ADD COLUMN branding_logo_storage_key TEXT;

-- +goose Down

ALTER TABLE profiles
  DROP COLUMN IF EXISTS branding_logo_storage_key;
```

Update `api/internal/db/queries/profiles.sql` so every profile SELECT/RETURNING includes `branding_logo_storage_key`. Add:

```sql
-- name: UpdateProfileBrandingLogo :one
UPDATE profiles
SET branding_logo_url = $2,
    branding_logo_storage_key = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING id, name, created_at, updated_at, branding_logo_url, branding_display_name, branding_primary_color, workspace_id, branding_hide_powered_by, branding_logo_storage_key;

-- name: ClearProfileBrandingLogo :one
UPDATE profiles
SET branding_logo_url = NULL,
    branding_logo_storage_key = NULL,
    updated_at = NOW()
WHERE id = $1
RETURNING id, name, created_at, updated_at, branding_logo_url, branding_display_name, branding_primary_color, workspace_id, branding_hide_powered_by, branding_logo_storage_key;
```

- [ ] **Step 6: Regenerate sqlc**

Run:

```bash
cd api && sqlc generate
```

Expected: `models.go` includes `BrandingLogoStorageKey pgtype.Text`; `profiles.sql.go` includes the two new query methods.

---

### Task 2: Backend Upload/Delete Handlers

**Files:**
- Modify: `api/internal/handler/projects.go`
- Modify: `api/internal/handler/projects_branding_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing handler helper tests**

Add tests to `api/internal/handler/projects_branding_test.go` for:

```go
func TestValidateBrandingLogoUploadAcceptsPNGAndJPEG(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []byte
		wantContentType string
		wantExt string
	}{
		{"png", testPNGBytes(t), "image/png", ".png"},
		{"jpeg", testJPEGBytes(t), "image/jpeg", ".jpg"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateBrandingLogoUpload(tc.body)
			if err != nil {
				t.Fatalf("validateBrandingLogoUpload: %v", err)
			}
			if got.contentType != tc.wantContentType || got.ext != tc.wantExt {
				t.Fatalf("got %#v, want content type %q ext %q", got, tc.wantContentType, tc.wantExt)
			}
		})
	}
}

func TestValidateBrandingLogoUploadRejectsSVGAndOversize(t *testing.T) {
	if _, err := validateBrandingLogoUpload([]byte(`<svg></svg>`)); err == nil {
		t.Fatal("svg should be rejected")
	}
	if _, err := validateBrandingLogoUpload(make([]byte, brandingLogoMaxBytes+1)); err == nil {
		t.Fatal("oversize logo should be rejected")
	}
}

func TestCanDeleteBrandingLogoStorageKeyScopesDeletes(t *testing.T) {
	if !canDeleteBrandingLogoStorageKey("branding/ws_1/pr_1/logo_a.png", "ws_1", "pr_1") {
		t.Fatal("expected matching branding key to be deletable")
	}
	for _, key := range []string{
		"branding/ws_2/pr_1/logo_a.png",
		"branding/ws_1/pr_2/logo_a.png",
		"media/file.png",
		"../branding/ws_1/pr_1/logo.png",
	} {
		if canDeleteBrandingLogoStorageKey(key, "ws_1", "pr_1") {
			t.Fatalf("key %q should not be deletable", key)
		}
	}
}
```

Include small `testPNGBytes` and `testJPEGBytes` helpers using `image/png` and `image/jpeg`.

- [ ] **Step 2: Run handler tests and verify red**

Run:

```bash
cd api && go test ./internal/handler -run 'TestValidateBrandingLogoUpload|TestCanDeleteBrandingLogoStorageKey'
```

Expected: fails because helpers/constants are undefined.

- [ ] **Step 3: Add handler dependencies and validation helpers**

In `api/internal/handler/projects.go`:

- Add a narrow `brandingLogoStore` interface with `PutObject`, `Delete`, and `PublicURL`.
- Add `store brandingLogoStore` to `ProfileHandler`.
- Add `SetBrandingLogoStore(store brandingLogoStore) *ProfileHandler`.
- Add constants:

```go
const brandingLogoMaxBytes = 2 * 1024 * 1024
const brandingLogoCacheControl = "public, max-age=31536000, immutable"
```

- Add `validateBrandingLogoUpload([]byte)` and `canDeleteBrandingLogoStorageKey(...)` helpers.

- [ ] **Step 4: Run handler tests and verify green**

Run:

```bash
cd api && go test ./internal/handler -run 'TestValidateBrandingLogoUpload|TestCanDeleteBrandingLogoStorageKey'
```

Expected: tests pass.

- [ ] **Step 5: Implement upload/delete methods**

Add:

```go
func (h *ProfileHandler) UploadBrandingLogo(w http.ResponseWriter, r *http.Request)
func (h *ProfileHandler) DeleteBrandingLogo(w http.ResponseWriter, r *http.Request)
```

Behavior:

- Resolve `workspaceID` from `auth.GetWorkspaceID`.
- Load profile by id and verify `profile.WorkspaceID == workspaceID`.
- Enforce `PlanAllowsHostedConnectBranding`.
- Return `503 STORAGE_NOT_CONFIGURED` if store is nil.
- Read multipart `file` with a 2 MB cap.
- Validate PNG/JPEG and dimensions.
- Upload to `storage.BrandingLogoKey(profile.WorkspaceID, profile.ID, ext)`.
- Update profile through `UpdateProfileBrandingLogo`.
- Delete old R2 key only when `canDeleteBrandingLogoStorageKey` returns true.
- Delete endpoint clears DB via `ClearProfileBrandingLogo`, then deletes old key best-effort.

- [ ] **Step 6: Wire routes**

In `api/cmd/api/main.go`:

```go
profileHandler := handler.NewProfileHandler(queries, quotaChecker).SetBrandingLogoStore(storageClient)
```

Add routes in the workspace-scoped profiles section:

```go
r.Post("/v1/profiles/{id}/branding/logo", profileHandler.UploadBrandingLogo)
r.Delete("/v1/profiles/{id}/branding/logo", profileHandler.DeleteBrandingLogo)
```

- [ ] **Step 7: Run backend focused tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/storage ./internal/handler
```

Expected: pass.

---

### Task 3: Dashboard Branding API and White-label UI

**Files:**
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/accounts/native/page.tsx`

- [ ] **Step 1: Add API client helpers**

In `dashboard/src/lib/api.ts`, add:

```ts
export async function uploadProfileLogo(
  token: string,
  profileId: string,
  file: File
): Promise<ApiResponse<Profile>> {
  const form = new FormData();
  form.append("file", file);
  return request(`/v1/profiles/${profileId}/branding/logo`, token, {
    method: "POST",
    body: form,
  });
}

export async function deleteProfileLogo(
  token: string,
  profileId: string
): Promise<ApiResponse<Profile>> {
  return request(`/v1/profiles/${profileId}/branding/logo`, token, {
    method: "DELETE",
  });
}
```

If `request` always forces JSON headers, update it so `FormData` bodies do not set `Content-Type`.

- [ ] **Step 2: Extend White-label page state**

In `native/page.tsx`:

- Capture `profileId` from `useParams`.
- Load `getProfile(token, profileId)` alongside credentials and limits.
- Add state for `profile`, `displayName`, `primaryColor`, `hidePoweredBy`, `brandingSaving`, `logoUploading`, `brandingError`, `logoError`.

- [ ] **Step 3: Add Hosted Connect Branding section**

Render before platform credentials:

- locked plan copy when branding is not allowed
- display name input
- logo preview + upload input + replace/remove controls
- primary color swatch/input
- hide attribution checkbox
- save button
- live preview card

Use existing dashboard tokens and `settings-section` classes. Do not add nested cards or marketing-style hero layout.

- [ ] **Step 4: Wire form actions**

Implement:

- `handleBrandingSave` calls `updateProfile` with display/color/hide attribution.
- `handleLogoUpload` calls `uploadProfileLogo`.
- `handleLogoDelete` calls `deleteProfileLogo`.
- State updates from returned `Profile`.

- [ ] **Step 5: Run dashboard build**

Run:

```bash
cd dashboard && npm run build
```

Expected: build passes.

---

### Task 4: Final Verification and Commit

**Files:**
- All changed files.

- [ ] **Step 1: Run backend validation**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: pass.

- [ ] **Step 2: Run dashboard validation**

Run:

```bash
cd dashboard && npm run build
```

Expected: pass.

- [ ] **Step 3: Inspect diff**

Run:

```bash
git status --short
git diff --stat
```

Expected: only PRD, plan, backend branding/storage/db, API routing, and dashboard White-label files are changed.

- [ ] **Step 4: Commit**

Run:

```bash
git add docs/prd-white-label-profile-branding-logo-upload.md docs/superpowers/plans/2026-05-30-white-label-profile-branding-logo-upload.md api dashboard
git commit -m "Add white-label profile branding upload"
```
