# PRD - White-label Profile Branding and Logo Upload

**Status:** Planning
**Owner:** UniPost Product / Platform Engineering
**Target:** Accounts -> White-label dashboard setup, hosted Connect branding, R2-backed logo assets
**Created:** 2026-05-30

---

## Problem

White-label customers currently have two related setup jobs:

1. Upload their own platform credentials so OAuth uses the customer's app and quota.
2. Configure the brand identity their end users see when connecting social accounts.

The first job exists in the dashboard under `Accounts -> White-label Credentials`. The second job mostly exists only through the Profile API:

- `branding_logo_url`
- `branding_display_name`
- `branding_primary_color`
- `branding_hide_powered_by`

The dashboard only exposes the attribution toggle today. That leaves customers with an awkward split: they can paste platform `client_id` / `client_secret` in the White-label page, but must call the API manually to make the hosted Connect page show their logo, name, and color.

This is especially confusing because the hosted Connect page is the moment where the customer's end user decides whether to authorize access. The page should say, visually and textually:

```text
<Customer Logo>
Customer Product wants to publish posts to your TikTok account on your behalf.
```

Not:

```text
UniPost wants to publish posts...
```

White-label should feel like one setup surface, not a credentials page plus an undocumented API task.

## Goals

1. Add a **Hosted Connect Branding** section to `Accounts -> White-label`.
2. Let customers upload a logo image directly from the dashboard.
3. Store uploaded logo assets in Cloudflare R2 as long-lived branding assets.
4. Let customers edit the Profile display name and primary color from the same page.
5. Keep the existing Profile API fields as the source of truth for hosted Connect rendering.
6. Show a live preview of the hosted Connect entry page using the selected logo, display name, color, and attribution state.
7. Keep plan gates consistent:
   - Basic and up can brand hosted Connect.
   - Growth and Team can hide `Powered by UniPost`.
8. Preserve the existing Quickstart and white-label runtime flow: Connect Sessions, `managed_account_id`, publish API, MCP tools, webhooks, and account status behavior do not change.

## Non-goals

- Do not change how platform OAuth credentials are uploaded.
- Do not change the platform OAuth consent screen itself. That screen is controlled by the platform and shows the app configured in the customer's developer console.
- Do not add custom domain support in this PRD.
- Do not build a domain/app-review assistant in this PRD.
- Do not expose arbitrary CSS or full page theming.
- Do not reuse the general `/v1/media` upload lifecycle for logos. Post media can be cleaned up after publish; branding logos are long-lived account configuration.
- Do not require a new feature flag unless implementation owners explicitly decide to stage rollout. The feature is naturally gated by existing plan limits.

## Product Decision

White-label setup should be presented as two adjacent controls on the same page:

```text
Accounts -> White-label

1. Hosted Connect Branding
   - Profile display name
   - Logo upload
   - Primary color
   - Powered by UniPost attribution toggle
   - Live Connect preview

2. Platform Credentials
   - Meta / Facebook
   - LinkedIn
   - TikTok
   - YouTube
   - X / Twitter
   - Pinterest
```

The mental model:

- **Branding** controls what the end user sees on UniPost-hosted Connect pages.
- **Credentials** control which platform app and quota are used behind OAuth.

These two are related but not identical. A customer can configure branding before platform credentials are complete. A customer can also upload credentials before polishing branding. The page should make both setup states visible.

## User Stories

1. As a white-label customer, I can open `Accounts -> White-label` and see whether my hosted Connect branding is complete.
2. As a white-label customer, I can upload my logo without preparing a public CDN URL.
3. As a white-label customer, I can set the display name shown to my end users during Connect.
4. As a white-label customer, I can choose the primary button color used on hosted Connect.
5. As a Growth or Team customer, I can hide the `Powered by UniPost` attribution from hosted Connect.
6. As a Basic customer, I can brand the page but cannot hide UniPost attribution.
7. As a Free or API customer, I can see the branding controls in a locked state with clear upgrade copy.
8. As an end user receiving a Connect Session, I see the customer's logo/name before authorizing the platform connection.
9. As UniPost support, I can inspect a profile and know whether its logo is external or R2-managed.

## UX Requirements

### Placement

Primary placement:

```text
Dashboard -> Profile -> Accounts -> White-label
```

The existing Profile settings page may keep or link to the same branding controls, but the White-label page should be the primary self-serve setup path.

### Section 1: Hosted Connect Branding

Fields:

- **Display name**
  - Maps to `profiles.branding_display_name`.
  - Required for a "complete" branding state.
  - Max 60 characters, same as existing API validation.
  - Helper copy: "Shown to end users on hosted Connect pages."

- **Logo**
  - Upload button with drag-and-drop support.
  - Accept PNG and JPEG in MVP.
  - Max 2 MB.
  - Recommended square asset, at least 256 x 256.
  - Show current logo preview.
  - Provide "Replace" and "Remove" actions.

- **Primary color**
  - Maps to `profiles.branding_primary_color`.
  - Hex color input plus color swatch.
  - Same server validation: `#RRGGBB`.

- **Hide Powered by UniPost**
  - Maps to `profiles.branding_hide_powered_by`.
  - Disabled unless the plan allows it.
  - Copy:
    - Basic: "Available on Growth and Team."
    - Growth/Team: normal toggle.

### Live preview

The preview should render a compact hosted Connect card:

```text
[logo] Customer Product

Connect TikTok
Customer Product wants to publish posts to your TikTok account on your behalf.

[Authorize TikTok]

Powered by UniPost
```

Requirements:

- Use the selected primary color for the authorize button.
- Update instantly as form values change.
- Do not use a marketing-style hero. This is a utilitarian setup page.
- Keep the preview visually close to the real `/connect/[platform]` page.
- On mobile, stack form and preview vertically.

### Section 2: Platform Credentials

Keep the existing credential rows. Add a short bridge sentence above them:

```text
After branding is set, upload platform credentials so OAuth uses your app and your platform quota.
```

Credential rows continue to link to platform docs and developer portals.

## End-user Connect Behavior

The hosted Connect page already reads optional branding from the public session response and falls back to UniPost defaults.

After this feature, the normal white-label end-user flow should be:

1. Customer creates a Connect Session for an end user.
2. End user opens `app.unipost.dev/connect/{platform}?session=...`.
3. Hosted Connect page shows the customer's logo, display name, primary color, and optional attribution state.
4. End user clicks the authorize button.
5. Platform OAuth screen opens.
6. If customer platform credentials exist, the platform OAuth screen shows the customer's platform app identity.
7. UniPost receives the OAuth callback, stores/refreshes tokens, and returns `managed_account_id` as today.

Important wording distinction:

- Hosted Connect branding controls the UniPost-hosted pre-OAuth page.
- Platform app credentials control the platform-owned OAuth consent page.

Both are part of the customer's perceived white-label experience.

## API Requirements

### Existing Profile update API

Keep `PATCH /v1/profiles/{id}` as the source of truth for text/color/toggle fields:

```json
{
  "branding_display_name": "Customer Product",
  "branding_primary_color": "#2563eb",
  "branding_hide_powered_by": false
}
```

The API should continue returning:

```json
{
  "branding_logo_url": "https://...",
  "branding_display_name": "Customer Product",
  "branding_primary_color": "#2563eb",
  "branding_hide_powered_by": false
}
```

### New logo upload endpoint

Add a dedicated endpoint instead of overloading `/v1/media`:

```text
POST /v1/profiles/{id}/branding/logo
Content-Type: multipart/form-data
Field: file
```

Response:

```json
{
  "data": {
    "profile": {
      "id": "pr_123",
      "branding_logo_url": "https://pub-xxx.r2.dev/branding/ws_123/pr_123/logo_abc.png",
      "branding_display_name": "Customer Product",
      "branding_primary_color": "#2563eb",
      "branding_hide_powered_by": false
    }
  }
}
```

Why API-mediated upload instead of presigned PUT:

- Logo files are small.
- The API can atomically validate, store, and update the profile.
- R2-managed branding objects are retained indefinitely; replacing a logo updates the profile pointer to a new immutable object and leaves the old object in R2.
- The dashboard avoids a two-step "upload then commit URL" state.
- Orphaned branding objects are acceptable because they are small, customer profile assets and must not be lifecycle-deleted.

### Remove logo endpoint

Add:

```text
DELETE /v1/profiles/{id}/branding/logo
```

Behavior:

- Sets `branding_logo_url = NULL`.
- Clears `branding_logo_storage_key`.
- Does not delete the R2 object. Profile branding objects are retained indefinitely.
- Does not change display name, primary color, or attribution.

### Auth and plan gates

Use the same workspace/profile authorization rules as `PATCH /v1/profiles/{id}`.

Plan behavior:

- Free/API:
  - May read current branding fields.
  - Cannot save logo/display/color changes for hosted Connect branding.
  - Return `402 PLAN_FEATURE_NOT_ALLOWED` or existing plan-gate shape.
- Basic:
  - May save logo/display/color.
  - Cannot set `branding_hide_powered_by = true`.
- Growth/Team:
  - May save all branding fields.

## Data Model

Current profile fields remain:

- `branding_logo_url`
- `branding_display_name`
- `branding_primary_color`
- `branding_hide_powered_by`

Add an optional internal storage key:

```sql
ALTER TABLE profiles
  ADD COLUMN branding_logo_storage_key TEXT;
```

Semantics:

- `branding_logo_url` is the public URL used by hosted Connect and returned by APIs.
- `branding_logo_storage_key` is set only when UniPost uploaded the asset to R2.
- If a profile uses an externally supplied HTTPS logo URL through the API, `branding_logo_storage_key` is `NULL`.
- When replacing an R2-managed logo, update the profile to point at the new storage key and retain the previous object in R2.

## R2 Storage Requirements

Use the existing R2 client configuration:

- `R2_ACCOUNT_ID`
- `R2_ACCESS_KEY_ID`
- `R2_SECRET_ACCESS_KEY`
- `R2_BUCKET_NAME`
- `R2_PUBLIC_DOMAIN`

Add storage helpers for branding assets:

```go
func BrandingLogoKey(workspaceID, profileID, ext string) string
func (c *Client) PutObject(ctx context.Context, key string, body io.Reader, contentType string, cacheControl string) error
```

Key shape:

```text
branding/{workspace_id}/{profile_id}/logo_{uuid}.{ext}
```

Cache policy:

```text
public, max-age=31536000, immutable
```

Because replacement uses a new UUID key, old cached assets do not need cache busting.

Lifecycle invariant:

- Objects under `branding/` are profile configuration assets and must be retained indefinitely.
- Application cleanup workers must not delete `branding/` objects.
- R2 bucket lifecycle rules, if configured outside this repo, must not expire or delete the `branding/` prefix. Any automatic R2 cleanup must be prefix-scoped to temporary media paths such as `media/` or pull-through staging paths, never `branding/`.

Accepted MIME types in MVP:

- `image/png`
- `image/jpeg`

Rejected in MVP:

- SVG, because inline/scriptable SVG behavior differs across browsers and CDNs.
- GIF, because animated logos add visual noise to hosted Connect.
- WebP, unless implementation adds reliable server-side validation.

Limits:

- Max size: 2 MB.
- Empty file rejected.
- Decode image config server-side before storing.
- Recommended minimum: 256 x 256.
- Hard maximum dimensions: 4096 x 4096.

## Dashboard Implementation Requirements

### API client

Add:

```ts
uploadProfileLogo(token, profileId, file)
deleteProfileLogo(token, profileId)
```

Keep using `updateProfile(...)` for display name, color, and attribution.

### White-label page state

The page should load:

- current profile
- API limits / plan gates
- platform credentials

The page should track independent saving states:

- `brandingSaving`
- `logoUploading`
- `credentialSaving`

Credential save failures should not erase branding form state.
Branding save failures should not erase credential form state.

### Completion states

Show a concise setup summary:

```text
Branding: Complete / Needs logo / Needs display name / Locked by plan
Credentials: 2 of 7 platforms configured
```

This helps users understand why a white-label flow still looks incomplete.

## Error Handling

Logo upload errors:

- `STORAGE_NOT_CONFIGURED`: "Logo upload storage is not configured. Try again later or contact support."
- `PLAN_FEATURE_NOT_ALLOWED`: "Hosted Connect branding starts on Basic."
- `VALIDATION_ERROR`: show exact size/type/dimension issue.
- `PAYLOAD_TOO_LARGE`: "Logo must be 2 MB or smaller."

Profile branding errors:

- Display name too long.
- Invalid color.
- Attribution hide attempted on a plan that does not allow it.

Public hosted Connect fallback:

- If `branding_logo_url` is missing or the image fails to load, the page still renders display name and text.
- Broken logo should not block OAuth.

## Security and Abuse Controls

1. Only authenticated workspace members who can update the profile can upload or remove the logo.
2. Validate that the profile belongs to the caller's workspace.
3. Restrict logo upload to image MIME types and decoded image formats.
4. Reject SVG in MVP.
5. Do not accept user-controlled R2 object keys.
6. Use UUID-based object names to avoid cache poisoning and overwrites.
7. Do not expose R2 credentials to the browser.
8. Deleting/replacing a logo must not delete R2 objects. It only clears or replaces the profile's DB pointer.
9. The unauthenticated Connect page must treat logo URL as data only, rendered in an `img` element.

## Migration and Backward Compatibility

Existing customers with `branding_logo_url` set to an external HTTPS URL continue to work.

The new storage key column is optional. Existing rows get `NULL`.

The dashboard upload flow will create R2-managed logos going forward.

The existing API behavior remains valid:

- API callers can still set `branding_logo_url` directly if that is already supported.
- The dashboard prefers upload because it is easier and safer for self-serve users.

## Rollout

1. Ship backend storage key migration and logo upload/delete endpoints.
2. Add dashboard API client helpers.
3. Add the Hosted Connect Branding section above Platform Credentials.
4. Verify Basic, Growth, and Free/API plan states locally.
5. Deploy to development.
6. Test against `https://dev.unipost.dev` and `https://dev-api.unipost.dev`.
7. Validate a real Connect Session loads the uploaded logo and display name.

Feature flag:

- No new Unleash flag is proposed in this PRD.
- The implementation remains protected by plan gates and can be rolled back by reverting the dashboard section and endpoints.
- If implementation owners want a staged rollout, ask for explicit approval before adding an Unleash flag.

## Acceptance Criteria

1. A Basic-or-higher customer can upload a PNG/JPEG logo from `Accounts -> White-label`.
2. The uploaded logo is stored in R2 under the `branding/` prefix.
3. The profile's `branding_logo_url` updates to the public R2 URL.
4. Old R2-managed profile logo objects are retained indefinitely after replacement or removal.
5. The customer can remove the logo and hosted Connect falls back cleanly.
6. The customer can edit display name and primary color from the White-label page.
7. Growth/Team customers can hide attribution from the same section.
8. Basic customers see the attribution toggle disabled with upgrade copy.
9. Free/API customers see branding controls locked with upgrade copy.
10. A newly created Connect Session renders the uploaded logo, display name, primary color, and attribution state.
11. Platform credential upload behavior remains unchanged.
12. Existing API-set external `branding_logo_url` values continue rendering.

## Test Plan

Backend:

- Unit test accepted MIME types and size limits.
- Unit test rejected SVG/GIF/WebP in MVP.
- Unit test profile ownership enforcement.
- Unit test plan gate behavior for Free/API/Basic/Growth.
- Unit test profile branding storage keys are never considered deletable by cleanup logic.
- Handler test upload success updates profile fields.
- Handler test delete logo clears URL and storage key.

Dashboard:

- Form validation for display name and color.
- Logo preview after file selection.
- Locked Free/API state.
- Basic state with attribution toggle disabled.
- Growth state with attribution toggle enabled.
- Save branding without changing credentials.
- Save credentials without losing branding form state.

End-to-end:

- Upload logo in dashboard.
- Create Connect Session.
- Open hosted Connect URL.
- Verify logo, display name, button color, and attribution render.
- Replace logo and verify new Connect page uses the new URL.
- Remove logo and verify fallback.

## Open Questions

1. Should API callers still be allowed to set arbitrary external `branding_logo_url`, or should new writes require R2 upload while old values remain grandfathered?
2. Should MVP allow WebP if we add server-side WebP decode support?
3. Should the Profile settings page embed the same component or simply deep-link to `Accounts -> White-label`?
4. Should the White-label page be renamed to `Branding & Credentials` once this ships?
