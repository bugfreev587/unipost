# PRD: R2 Media Retention and Free Scheduled Post Cap

Date: 2026-07-06
Owner area: Publishing API, Media Storage, Pricing
Status: Implemented in `dev-r2-media-retention`; review fixes in `dev-r2-retention-review-fixes`

## Problem

The `unipost` R2 bucket is growing quickly after the Cloudflare R2 lifecycle cleanup rule was disabled. R2 lifecycle rules are unsafe for UniPost because scheduled posts can point to media that must remain available for weeks or months. Cleanup must be driven by UniPost business state instead of object age.

## Goals

- Clean uploaded media from R2 only after UniPost knows the parent post has reached a terminal status.
- Keep scheduled, draft, queued, publishing, and processing media available until the post finishes.
- Apply different media retention windows by workspace plan.
- Keep failed, partial, and cancelled posts for longer troubleshooting windows.
- Add a Free plan active scheduled-post cap to reduce long-lived media backlog.
- Remove the legacy code-level retention policy that scheduled large media cleanup after a fixed 2 hour window.
- Document the new behavior on the pricing page, comparison chart, and API docs.

## Non-Goals

- Do not rely on Cloudflare R2 lifecycle rules for UniPost uploaded media cleanup.
- Do not cap active scheduled posts for paid plans.
- Do not add a feature flag.
- Do not change monthly post quota semantics except for documenting how they interact with the Free scheduled cap.

## Product Rules

### Media Retention

Retention starts after the parent post reaches a terminal status.

| Plan | Published/success retention | Failed/partial/cancelled retention |
| --- | ---: | ---: |
| Free | 1 day | 2 days |
| API | 2 days | 4 days |
| Basic | 4 days | 8 days |
| Growth | 15 days | 30 days |
| Team | 30 days | 60 days |
| Enterprise | 30 days | 60 days |

Rules:

- `published` uses the success window.
- `failed`, `partial`, and `cancelled` use the failed window.
- `scheduled`, `draft`, `queued`, `publishing`, `processing`, and any other non-terminal status do not get a cleanup deadline.
- A retry moves the media usage back to an in-flight state so cleanup does not race the retry.
- Shared media is deleted only when every post usage for that media is due. Any usage with no cleanup deadline or a future cleanup deadline blocks deletion.
- Cleanup deletes both the source R2 object and the derived pull-copy object when present.

### Scheduled Post Cap

Free plan:

- Existing monthly quota remains 100 posts/month.
- Active scheduled parent posts cannot exceed 50.
- Active scheduled means parent posts in `scheduled` status with `deleted_at IS NULL`; overdue scheduled rows still count until the scheduler claims or cancels them.
- Published, failed, partial, draft, and cancelled posts do not count toward the active scheduled cap.

Paid plans:

- API, Basic, Growth, Team, and Enterprise do not cap active scheduled posts.

## Technical Design

### Retention Ledger

Add `media_post_usages` as the business-owned retention ledger:

- `media_id`
- `post_id`
- `workspace_id`
- `post_status`
- `cleanup_after_at`

The ledger is updated whenever a parent post is created, edited, queued, retried, or reaches a final aggregate status.

The migration backfills this ledger for existing v2 posts that are already in terminal statuses (`published`, `partial`, `failed`, `cancelled`) by expanding `social_posts.metadata.platform_posts[].media_ids`, joining owned `media` rows, and applying the current workspace plan retention window. This backfill runs before the legacy `media.cleanup_after_at` deadlines are cleared.

Because the initial retention migration may already be applied in development, a follow-up idempotent migration (`098_media_retention_review_fixes.sql`) repeats the terminal-post backfill and rebuilds the due index with `cancelled` included.

Media can be reused across multiple posts. The cleanup query must require at least one due usage and no blocking usage for the same `media_id`; otherwise one finished post could delete media still needed by a scheduled or in-flight post.

### Worker

The media cleanup worker runs:

- once on startup
- once every 24 hours after that

The worker processes due ledger rows in batches of 500. For each due media item, it deletes the source object and derived pull-copy object from R2, then deletes the media row so dependent usage rows cascade.

If a full batch makes zero deletion progress, the worker exits the current sweep and retries on the next startup/tick. This prevents a bad R2 credential, outage, or stuck object from causing an infinite tight loop over the same 500 rows.

### Legacy Policy Removal

Remove the old retention policy that scheduled large media cleanup after a fixed 2 hour window with `media.cleanup_after_at`. The migration backfills terminal media usage rows first, then clears existing `media.cleanup_after_at` values so the old deadline cannot delete media after the new ledger deploys.

## API Behavior

When a Free workspace attempts to create a scheduled post that would exceed the active scheduled cap, UniPost returns:

- HTTP `402`
- `code`: `PLAN_SCHEDULED_POST_LIMIT_EXCEEDED`
- `normalized_code`: `plan_scheduled_post_limit_exceeded`

The monthly quota error remains separate:

- HTTP `402`
- `code`: `PLAN_POST_QUOTA_EXCEEDED`

The Free active scheduled cap must be enforced atomically at scheduled-post creation time. UniPost uses a workspace-scoped Postgres advisory transaction lock inside the insert statement so concurrent requests cannot both pass a count-then-insert race.

## Documentation Requirements

- Pricing plan cards mention per-plan media retention.
- Pricing comparison chart includes:
  - Active scheduled posts
  - Media retention after success
  - Media retention after failed/partial/cancelled
- Pricing FAQ explains Free scheduled cap and media retention by plan.
- Docs pricing page includes retention and active scheduled cap in the plan ladder and usage controls.
- Media upload and get-media API docs explain status-driven retention.
- Create-post API docs explain the Free active scheduled cap and its 402 error.

## Acceptance Criteria

- Manual prerequisite: UniPost credentials can delete specific R2 objects in both development and production buckets.
- Backend tests cover:
  - retention matrix by plan/status
  - Free active scheduled cap behavior
  - atomic scheduled cap insert query
  - retention ledger migration
  - media usage upsert deadline calculation
  - worker 24 hour cadence, batch cleanup, and no-progress exit
  - removal of legacy cleanup query paths
- `GOCACHE=/tmp/unipost-go-build go test ./...` passes from `api/`.
- `npm run build` passes from `dashboard/`.
- Development deployment completes after pushing `origin/dev`.
- Development self-acceptance verifies:
  - pricing page exposes retention and active scheduled cap
  - docs expose retention and scheduled-cap behavior
  - backend is healthy after migration/deploy
