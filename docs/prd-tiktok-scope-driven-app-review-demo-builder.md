# PRD - TikTok Scope-Driven App Review Demo Builder

**Status:** Planning
**Owner:** UniPost Product / Platform Engineering
**Target:** White-label TikTok app review recordings
**Created:** 2026-05-27

---

## Problem

White-label customers who apply for official TikTok API access must submit demo videos that prove every requested scope is used in a real product flow. The hard part is not only recording a screen. The customer must know which product screen to show, which field proves each scope, how to expose the TikTok OAuth consent screen, how to satisfy TikTok's Content Posting API guidance, and how to keep each uploaded video under TikTok's review file-size limit.

UniPost can make this dramatically easier by turning TikTok app review into a scope-driven recording workflow. The customer selects the scopes they requested in TikTok Developer Portal, and UniPost generates the exact review steps, recording segments, subtitles, test data, and local-agent script needed to produce reviewer-friendly demo videos.

## Goals

1. Let a white-label customer select the TikTok API scopes they are applying for.
2. Generate a deterministic demo plan that maps every selected scope to visible product evidence.
3. Include the TikTok account connection flow and TikTok OAuth consent page in the recording.
4. Record real UniPost-powered customer flows, especially the create post drawer for Content Posting API scopes and TikTok Analytics for analytics scopes.
5. Produce clear 1080p demo videos with large section subtitles and browser address bar visible.
6. Split recordings into multiple files when needed so every final MP4 is under 50 MB.
7. Require an OAuth reset preflight so TikTok shows the authorization scope screen during recording.
8. Keep sensitive actions under the customer's control: TikTok login, 2FA, mobile confirmation, and account authorization.
9. Create a repeatable template system that can later support other platforms and scopes.

## Non-Goals

- Automatically submitting the TikTok app review form.
- Asking customers for TikTok passwords, 2FA codes, or mobile device access.
- Faking scope evidence or recording artificial API test pages as the primary demo.
- Supporting every TikTok product in the first version.
- Building native desktop apps for recording.
- Guaranteeing TikTok approval; UniPost improves evidence quality but cannot control reviewer decisions.
- Recording videos above TikTok's upload size limit and asking customers to manually compress them.

## Product Principle

The feature should not be framed as "screen recording automation." It should be framed as:

> Select scopes. UniPost generates reviewer-visible evidence for every scope.

This matters because TikTok reviewers evaluate whether the requested scopes are necessary and visibly used by the product. A successful recording must show a user journey, the TikTok OAuth authorization page, and product UI that makes each requested permission understandable.

## User Flow

### 1. Select TikTok App and Scopes

The customer opens:

```text
White-label > App Review > TikTok
```

They select or confirm:

- TikTok developer app
- app client key and secret status
- review domain
- redirect URI status
- TikTok API scopes requested in TikTok Developer Portal
- use case preset, such as Content Posting API or TikTok Analytics

The scope picker must make the selected scopes explicit. Presets can preselect common combinations, but the customer can adjust the final scope set to match TikTok Developer Portal.

### 2. Review Generated Demo Plan

UniPost generates a demo plan from the selected scopes. The page shows:

- required recording segments
- steps inside each segment
- which scopes each step proves
- required user actions
- required test data
- whether each segment will use create post drawer, account connection, analytics page, or external TikTok pages
- whether the segment is expected to publish a `SELF_ONLY` review video

The customer should be able to review the plan before starting recording. The plan is not free-form AI output. It is generated from versioned templates.

### 3. Complete Preflight

The page blocks recording until required setup is complete:

- TikTok credentials saved.
- Review domain is ready.
- Redirect URI has been copied into TikTok Developer Portal.
- Test TikTok account is available.
- Sample video is available.
- OAuth consent reset is confirmed.
- The customer has reviewed the generated recording plan.

Redirect URI cannot be fully verified before OAuth. The UI must present it as an attestation:

```text
I added this redirect URI in TikTok Developer Portal.
```

### 4. Reset TikTok Authorization Before Recording

To show TikTok's OAuth authorization scope screen, the customer must remove existing app access before recording if the TikTok account has previously authorized the app.

The page should show these instructions:

1. Open the TikTok mobile app.
2. Go to Settings and privacy.
3. Go to Security and permissions.
4. Open Apps and services permissions.
5. Find UniPost or the customer's TikTok app name.
6. Remove access.
7. Return to UniPost and confirm the reset is done.

The customer must check:

```text
I removed existing TikTok app authorization for this test account.
```

If the recording agent starts OAuth and TikTok skips the consent screen, the job should fail with a clear message:

```text
TikTok skipped the authorization page because this account is already authorized. Remove app access in TikTok mobile settings, then record again.
```

### 5. Start Recording

The customer clicks Start recording. UniPost creates a review job and shows a pinned CLI command. The local agent:

- launches or controls a browser window at a fixed recording size
- uses a signed review-session token
- records the browser window with address bar visible
- overlays large section titles for each step
- pauses for TikTok login, 2FA, QR confirmation, or authorization clicks
- resumes automatically when the expected page or callback is reached
- uploads final MP4 artifacts back to UniPost

## Scope Template Registry

The builder uses deterministic templates. Each template declares:

- supported platform
- supported scopes
- required evidence blocks
- required UI surfaces
- required external pages
- recording segment plan
- title overlays
- validation selectors
- success criteria
- expected failure states

AI may help draft readable labels or submission notes, but it must not decide the required evidence blocks for a scope.

Example shape:

```ts
type ReviewScopeTemplate = {
  platform: "tiktok";
  scopes: string[];
  useCase: "content_posting" | "analytics";
  requiredEvidenceBlocks: string[];
  recordingSegments: string[];
  outputConstraints: {
    resolution: "1080p";
    maxFileSizeMB: 50;
  };
};
```

## TikTok Scope Evidence Matrix

| Scope | Required visible evidence | Primary surface |
| --- | --- | --- |
| `user.info.basic` | Connected TikTok account identity, posting account, account name/avatar, creator info-driven post settings | Connection flow and create post drawer |
| `video.upload` | User selects/uploads a video, UniPost validates it, preview is visible before publish | Create post drawer |
| `video.publish` | User clicks publish, UniPost shows publish status, TikTok profile or post page confirms result | Create post drawer, posts list, TikTok profile |
| `user.info.profile` | TikTok profile card, avatar, username/display name, profile data used in analytics UI | TikTok Analytics |
| `user.info.stats` | Followers, following, likes, video count, or equivalent account stats | TikTok Analytics |
| `video.list` | Public video list from TikTok, compared with TikTok profile page | TikTok Analytics, only if requested |

## Required OAuth Prelude

Every full TikTok demo plan must start from a disconnected TikTok account state. The recording should show:

1. Customer-branded UniPost or white-label dashboard.
2. TikTok account is not connected.
3. User clicks Connect TikTok.
4. Browser navigates to TikTok OAuth.
5. TikTok shows the customer's app name/logo and requested access.
6. User authorizes access.
7. Browser returns to the customer domain through the configured callback.
8. UniPost shows TikTok connected.

This prelude proves that the requested scopes are user-authorized through TikTok, not silently assumed by UniPost.

## Content Posting API Demo Plan

When the selected scopes include `user.info.basic`, `video.upload`, or `video.publish`, UniPost should generate a three-part recording plan modeled after approved TikTok review demos.

### Posting Part 1 - Creator Info, Upload, and Content Details

Section title:

```text
1. Retrieve Creator Info
```

Required actions:

- Open the create post drawer.
- Select the connected TikTok social account.
- Show "posting as" account identity.
- Show privacy options, interaction controls, max video duration, and account capabilities.
- Show that these controls are driven by TikTok creator information.

Section title:

```text
2. User Uploads Video And Enters Post Details
```

Required actions:

- Upload or select a review-safe video.
- In UniPost production flows, the video should be uploaded through the same media path used by normal create post, including R2 when applicable.
- Show video selected and previewable.
- Enter a caption.
- Select explicit TikTok visibility, preferably `SELF_ONLY` during review/sandbox flows.
- Show validation if the user has not selected required TikTok fields.

Section title:

```text
3a. Content Disclosure Settings
```

Required actions:

- Show `Disclose video content`.
- Show `Your Brand` and `Branded Content` choices.
- Show validation or helper text when disclosure choices are incomplete.
- Show the completed disclosure state.

### Posting Part 2 - Privacy Management and Compliance

Section title:

```text
3b. Privacy Management
```

Required actions:

- Open the TikTok visibility selector.
- Show available visibility options from TikTok account capabilities.
- Select review-safe visibility.
- Show comment, duet, and stitch controls.
- If duet or stitch are unavailable, show disabled states and keep them visible enough for reviewer context.

Section title:

```text
4. Compliance Requirements
```

Required actions:

- Show Music Usage Confirmation link.
- Open the TikTok music usage or policy page.
- Return to the create post drawer.
- Show Branded Content Policy or relevant TikTok policy link.
- Open the policy page.
- Return and complete required confirmation.

### Posting Part 3 - Preview, Publish, and Verification

Section title:

```text
5. Preview And Publish
```

Required actions:

- Show final post preview.
- Click Publish.
- Show publishing progress.
- Show publish result or post id.
- Show the post in UniPost posts list with published status.
- Open the TikTok profile or TikTok page and show the published review video where possible.
- Return to UniPost and show the final record.

Notes:

- Review/sandbox publishes may use or fall back to `SELF_ONLY`. This is acceptable when the submission notes explain the review-safe visibility.
- The demo should use the real create post drawer or a review mode of the real drawer, not a simplified API test page.

## TikTok Analytics Demo Plan

When selected scopes include `user.info.profile`, `user.info.stats`, or `video.list`, UniPost should generate analytics recording segments.

### Analytics Part 1 - Login, OAuth, and Navigation

Section title:

```text
Login To Customer App
```

Required actions:

- Log in to the customer-branded UniPost workspace.
- Open Connections, Quickstart, or the account connection area.
- Show TikTok disconnected.
- Click Connect TikTok.

Section title:

```text
Authorize Access To Customer App
```

Required actions:

- Show TikTok OAuth consent screen.
- Show requested analytics scopes.
- Authorize access.
- Return to the customer domain.
- Show TikTok connected.
- Navigate to Analytics > Platforms > TikTok Analytics.
- Show analytics loading state.

### Analytics Part 2 - Scope Evidence

Section title:

```text
1. user.info.profile
```

Required actions:

- Show TikTok profile card.
- Show username, display name, avatar, or profile identity fields used by the product.

Section title:

```text
2. user.info.stats
```

Required actions:

- Show followers, following, likes, video count, or supported stats.
- Make the metric labels readable at 1080p.

Optional section title:

```text
3. video.list
```

Required only when `video.list` is selected:

- Show TikTok videos list in UniPost analytics.
- Open TikTok profile or public page to compare videos.
- Return to UniPost analytics.

If the customer did not request `video.list`, the builder must not include this section.

## Recording Quality Requirements

Final MP4 artifacts must satisfy:

- 1080p target resolution.
- Browser address bar visible.
- Customer review domain visible during product steps.
- TikTok domain visible during OAuth and external policy/profile checks.
- 30 fps target.
- Each final file under 50 MB.
- Large section titles visible for every scope group or required step.
- No sensitive password, 2FA code, recovery code, or secret should be visible in the final output.

If the recording exceeds 50 MB, UniPost should automatically split it into multiple ordered parts rather than asking the customer to edit video files manually.

Recommended segment naming:

```text
tiktok-content-posting-part-1.mp4
tiktok-content-posting-part-2.mp4
tiktok-content-posting-part-3.mp4
tiktok-analytics-part-1.mp4
tiktok-analytics-part-2.mp4
```

## App Review Page Requirements

### Scope Selection

The page should include:

- TikTok scope checklist.
- Presets for Content Posting API and Analytics.
- Warning when a selected scope has no supported template.
- Warning when a selected scope is known to cause sandbox OAuth issues.
- Summary of selected scopes and generated evidence coverage.

### Generated Plan Preview

The page should show:

- ordered recording segments
- section titles
- user actions
- scopes proved by each section
- estimated duration
- expected output files
- required manual pauses

The user should not need to read TikTok docs to understand what the video will contain.

### Preflight

The page should check or ask for:

- TikTok credentials
- review domain
- redirect URI attestation
- test TikTok account
- sample video
- OAuth reset confirmation
- local recording readiness

### Start and Resume

The page should:

- create a review job
- show a pinned CLI command
- show live job state
- classify failures
- provide exact resume or re-record commands
- keep existing setup when re-recording

## Local Agent Requirements

The local agent should:

- run from a pinned package version
- reject unknown script actions through a closed enum
- launch a browser window at a controlled size
- record at 1080p where the local display allows it
- show instructional overlays during manual pauses
- fail clearly when TikTok skips OAuth consent
- upload final artifacts to the review job
- report segment timings and step events

Manual pause overlays should be obvious to the user:

```text
Log in to TikTok. UniPost will continue automatically after authorization.
```

Final reviewer artifacts should not include instructional pause overlays unless they are unavoidable. The preferred UX is to show pause instructions in the dashboard, terminal, or a non-recorded control window while the recorded browser shows only the customer product, TikTok OAuth, TikTok policy pages, and TikTok profile verification. If an in-browser overlay is used during beta, UniPost should trim or exclude that portion before final artifact upload.

## Data Model Additions

Existing `review_kits`, `review_jobs`, and `review_sessions` can be extended. Suggested additions:

### `review_kits`

- `requested_scopes`
- `scope_template_version`
- `use_case`
- `generated_plan_json`
- `oauth_reset_required`

### `review_jobs`

- `recording_profile`
- `segment_status_json`
- `selected_output_segments`

### `review_job_events`

Add event types for:

- `oauth_consent_seen`
- `oauth_consent_skipped`
- `manual_pause_started`
- `manual_pause_completed`
- `segment_started`
- `segment_completed`
- `video_split_completed`

### Review Artifacts

Each uploaded video artifact should store:

- segment key
- filename
- duration
- file size
- resolution
- scopes covered
- subtitle titles included

## API Requirements

Suggested endpoints:

```text
GET  /v1/review/tiktok/scope-templates
POST /v1/review/tiktok/demo-plan
POST /v1/review/kits
POST /v1/review/jobs
GET  /v1/review/jobs/{id}
POST /v1/review/jobs/{id}/events
POST /v1/review/jobs/{id}/artifacts
```

`POST /v1/review/tiktok/demo-plan` should be deterministic for the same input scopes and template version.

## Feature Flag

Use the existing App Review Autopilot flag for the initial protected release:

```text
app_review.autopilot_v1
```

If the scope-driven builder is released independently from the current autopilot MVP, add a child flag:

```text
app_review.scope_demo_builder_v1
```

Production default must stay off until the TikTok flow has been validated end to end.

## Error Handling

### OAuth consent skipped

Cause:

- TikTok account already authorized the app.

Resolution:

- Show TikTok mobile app removal instructions.
- Require re-record.

### Redirect URI mismatch

Cause:

- Customer did not add the callback URL correctly in TikTok Developer Portal.

Resolution:

- Show exact redirect URI.
- Show copy button.
- Ask customer to update TikTok Developer Portal.
- Re-run from OAuth step.

### Video over 50 MB

Cause:

- Recording segment is too long or bitrate is too high.

Resolution:

- Automatically split or recompress.
- Preserve 1080p readability as the priority.

### Missing sample video

Cause:

- Posting demo requires an uploadable video.

Resolution:

- Provide a default review-safe sample video or let customer upload one.

### TikTok login blocked or CAPTCHA

Cause:

- TikTok risk controls.

Resolution:

- Pause recording.
- Let customer complete the challenge.
- Do not attempt to bypass TikTok security controls.

## Success Metrics

- Percentage of generated plans with complete scope coverage.
- Percentage of jobs producing all required MP4 files.
- Median time from selecting scopes to final artifacts.
- Number of customer interventions per successful recording.
- Percentage of recordings where OAuth consent screen was captured.
- Percentage of final videos under 50 MB without manual editing.
- TikTok approval rate for customers using generated demo plans.

## Rollout Plan

### Phase 1 - Internal TikTok Content Posting Template

- Support `user.info.basic`, `video.upload`, `video.publish`.
- Reuse the create post drawer in review mode.
- Generate three posting video segments.
- Validate with TailTales and UniPost-owned examples.

### Phase 2 - TikTok Analytics Template

- Support `user.info.profile` and `user.info.stats`.
- Support `video.list` only when requested.
- Reuse TikTok Analytics page.
- Generate two analytics video segments.

### Phase 3 - Customer Beta

- Enable for selected white-label customers.
- Add guided OAuth reset checklist.
- Add dashboard plan preview and re-record loop.
- Track real TikTok review outcomes.

### Phase 4 - Template Expansion

- Add other TikTok products only after the template system is stable.
- Generalize the scope template registry for Meta, YouTube, LinkedIn, or Pinterest review packages.

## Open Questions

1. Should customer-facing recordings show UniPost branding at all, or only the customer's brand?
2. Should manual pause overlays be visible in final MP4 files or trimmed during post-processing?
3. Should the customer upload their own review sample video, or should UniPost provide a platform-safe default?
4. Should the scope picker import scopes from TikTok Developer Portal manually via copy/paste, or rely on user selection only?
5. Should analytics scopes be included in the first customer beta or kept behind an internal-only template until Content Posting is stable?

## Acceptance Criteria

1. A customer can select TikTok scopes and see a generated plan mapping every supported scope to evidence steps.
2. The generated plan includes the OAuth connection and authorization scope screen prelude.
3. The generated posting plan uses the real create post drawer or a review mode of it.
4. The generated analytics plan uses the TikTok Analytics page.
5. The plan includes explicit section titles aligned with TikTok review guidance.
6. The agent can produce ordered 1080p MP4 artifacts under 50 MB each.
7. If TikTok skips OAuth consent, the job fails with reset instructions instead of producing weak evidence.
8. Re-recording does not require redoing domain, credentials, or brand setup.
9. The final artifact metadata shows which scopes each video segment covers.
