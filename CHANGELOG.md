# UniPost Changelog

All notable changes to the UniPost API + MCP server are documented here.

The format is loosely based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
this project does not yet follow strict semantic versioning across deploys —
each section heading below corresponds to a sprint, not a published version.

## [Unreleased]

### Changed — Workspace + Profile Architecture Refactor

- **Breaking**: `Project` concept split into `Workspace` (security boundary,
  API keys, billing, posts) and `Profile` (lightweight brand grouping, social
  accounts). Enables cross-profile posting — one post can target accounts from
  multiple profiles within the same workspace.
- **Database**: New `workspaces` table; `projects` renamed to `profiles`;
  all FK columns updated (`project_id` → `workspace_id` or `profile_id`).
  Migration 025 handles the full schema change.
- **API routes**: Dashboard routes split into `/v1/workspaces/{id}/...`
  (workspace-scoped) and `/v1/profiles/{id}/...` (profile-scoped).
  `/v1/projects` removed; use `/v1/profiles` for profile CRUD.
- **Error codes**: `account_not_in_project` → `account_not_in_workspace`,
  `media_id_not_in_project` → `media_id_not_in_workspace`.
- **Stripe metadata**: `project_id` → `workspace_id` in checkout session metadata.
- **Dashboard**: All "Project" UI text → "Profile"; API client updated for new endpoints.
- **Admin**: `active_projects` → `active_workspaces`, `project_count` → `workspace_count`.

## Sprint 5 — Post-Launch Hardening

### Added — UniPost API

- **Analytics rollup endpoint** — `GET /v1/analytics/rollup` returns
  time-bucketed publish metrics (total / succeeded / failed) with
  configurable granularity (`day`, `week`, `month`) and GROUP BY
  dimensions (`platform`, `social_account_id`, `external_user_id`,
  `status`). Max range 366 days. Dynamic SQL uses an allowlist-based
  GROUP BY (not user input interpolation) to prevent injection — two
  lock tests pin the allowlists so a future refactor can't silently
  expand the SQL surface. (PR1)
- **Per-account monthly publish quota** — new `per_account_monthly_limit`
  column on `projects`. When set, the publish path counts each social
  account's successful posts in the current calendar month (UTC) and
  refuses dispatch when the count reaches the cap. Enforcement is
  wired into both the immediate publish path and the scheduler, using
  a per-request `PerAccountTracker` with an atomic check-and-decrement
  mutex so parallel dispatch groups within a single request can't
  over-publish. `NULL` = unlimited (existing behavior); `0` = emergency
  lockout. `PATCH /v1/projects/:id` accepts the field with a
  `**int32` shape so absent/null/number map correctly. 7 tests under
  `-race`. (PR2)
- **Instagram Connect** (feature-flagged) — `InstagramConnector`
  implements the managed-user OAuth flow using "Instagram API with
  Instagram Login" (graph.instagram.com, no Facebook required). Two-
  step token swap (short-lived → long-lived); long-lived failure is
  fatal (no 1-hour fallback). Gated behind `CONNECT_INSTAGRAM_ENABLED`
  so the code ships to production before Meta App Review. 7 tests. (PR3)
- **Threads Connect** (feature-flagged) — `ThreadsConnector` for the
  same managed-user flow. Structurally identical to Instagram (same
  Meta two-step swap), with `th_` grant-type prefix (not `ig_`),
  `threads_profile_picture_url` avatar field, and authorize URL on
  `threads.net` (not `graph.threads.net`). Gated behind
  `CONNECT_THREADS_ENABLED`. 7 tests pin the per-platform deltas
  against IG→Threads port typos. (PR4)
- **MCP server v0.6.0** with two new tools:
  `unipost_get_analytics_rollup` (dimensional rollup with
  day/week/month granularity + dynamic GROUP BY),
  `unipost_update_project_quota` (set/clear the per-account cap from
  MCP). `unipost_create_connect_session` platform enum expanded to
  include `instagram` and `threads`. Tool count: 20 (from 18). (PR9)
- **/tools landing page** at `unipost.dev/tools` — tool gallery with
  reusable `ToolCard` component. AgentPost is the first card;
  Connect Widget and Analytics Explorer are "coming soon" placeholders.
  **/tools/agentpost** product page with hero, terminal demo mockup,
  feature cards, LLM provider showcase, and CTA. Both routes live in
  the existing Next.js app — no new domain. (PR8)

### Added — AgentPost

- **OpenAI + Gemini LLM providers** — refactored the single-provider
  Claude path into `src/lib/llm/` with per-provider SDKs (Anthropic,
  OpenAI, Google Gemini). The prompt is shared; the SDK call differs
  per file. `llm_provider` in config.json is the new switch (default
  "anthropic" for backward compat). `agentpost init` walks through
  provider choice and asks only for the matching key. The shared
  parser (`parse.ts`) absorbs all provider quirks: markdown fence
  stripping (Gemini), prose-prefix recovery (Gemini), unknown
  account_id rejection, empty-caption rejection, active-account
  coverage check. 11 tests. (PR5)
- **rss-bridge example** — `examples/rss-bridge/` polls any RSS or
  Atom feed and posts new items as per-platform social posts via
  UniPost. First-run safety: only the most recent item is published
  on the first run. State tracked via a single guid in `state.json`,
  committed back to the repo by the included GitHub Action. (PR6)

### Changed

- **Migration numbering** — `022_analytics_rollup_index.sql` was
  renumbered to `023` (and PR2's quota migration to `024`) because
  an external commit had already used version 022 for
  `022_default_and_last_project.sql`. Goose panicked on boot with
  "duplicate version 22 detected" until the renumber shipped.
- **AgentPost `requireConfig()`** relaxed — no longer hard-checks
  `anthropic_api_key` at startup. Provider key validation is lazy
  (via `requireProviderKey()` in `llm/index.ts`) so a config with
  multiple providers configured can switch between them without
  re-running init.

### Fixed

- **Migration 022 collision** — `022_analytics_rollup_index.sql`
  collided with the pre-existing `022_default_and_last_project.sql`,
  causing a goose panic on Railway deploy. Renumbered to 023 in
  commit `1ba40b5`. (PR1 hotfix)

## Sprint 4 — Launch Sprint (AgentPost v0.1 + Polish)

This is the first sprint whose deliverable is **a launch**, not just
code. AgentPost v0.1 ships as an open-source CLI on npm
(`@unipost/agentpost`), Show HN goes live on Tuesday April 28 2026
9am PT, and the UniPost Connect surface lands four polish items
that close the gaps from the Sprint 3 review.

### Added — UniPost API

- **Managed Twitter media support** — the validator no longer rejects
  posts with images on managed (Connect-flow) Twitter accounts. The
  Connect OAuth flow now requests `media.write` and the BYO Twitter
  adapter passes the `media_category=tweet_image` form field on
  uploads. Three latent bugs surfaced and were fixed during smoke
  testing (empty `media.write` scope on both Connect + BYO paths,
  missing `media_category` field on the Twitter `/2/media/upload`
  endpoint).
- **Bulk publish endpoint** — `POST /v1/social-posts/bulk` accepts
  up to 50 single-post bodies in one request and returns a per-post
  result array. Partial-success semantics: HTTP 200 as long as the
  request itself parses, with per-post failures landing in each
  entry's `error` field. Per-post idempotency keys still work; the
  natural retry pattern is "re-send the same batch with the same
  keys."
- **First-comment support** — every `platform_posts[]` entry now
  accepts an optional `first_comment` string. The handler publishes
  the main post, captures the external_id, then dispatches the
  first comment via a new `FirstCommentAdapter` interface that
  Twitter (self-reply), LinkedIn (UGC comment), and Instagram
  (Graph API comments) implement. Bluesky and Threads strict-reject
  the field with `first_comment_unsupported` — they have native
  thread support, use `thread_position` instead. First-comment
  failure is recorded as a `warnings[]` entry on the parent result;
  the main post is never rolled back.
- **White-label Connect branding** — `projects` gains three optional
  columns (`branding_logo_url`, `branding_display_name`,
  `branding_primary_color`) that the hosted Connect page renders
  when set. Customers can show their own logo + name + primary
  button color on the page their end users see. The "Powered by
  UniPost" footer is always visible (full-label removal is Sprint 5+).
- **Managed Users view** — `GET /v1/users` and the dashboard
  `/projects/{id}/users` page show one row per end user
  (`external_user_id`) onboarded via Connect, grouped on the fly
  via SQL aggregation over `social_accounts`. Each row carries
  per-platform account counts and a reconnect-needed flag. Detail
  view at `GET /v1/users/{external_user_id}` returns the full
  per-account list with disconnect buttons.
- **Meta data-deletion endpoint** — `POST /v1/meta/data-deletion`
  verifies Meta's signed_request HMAC-SHA256, extracts the user_id,
  deletes matching `social_accounts` rows, returns the
  `{url, confirmation_code}` response Meta requires. Mandatory for
  the eventual Meta App Review submission. Returns 503
  NOT_CONFIGURED until `META_APP_SECRET` is set in Railway.
- **MCP server v0.5.0** with three new tools:
  `unipost_bulk_create_posts`, `unipost_list_managed_users`,
  `unipost_get_managed_user`. The existing `unipost_create_post`
  schema documents the new `first_comment` field on
  `platform_posts[]` entries.

### Added — AgentPost (new repo at github.com/unipost-dev/agentpost)

- **AgentPost v0.1 CLI** — `npm install -g @unipost/agentpost`
  installs the `agentpost` binary. Three commands: `init` (sets up
  UniPost + Anthropic API keys at `~/.agentpost/config.json`),
  `accounts` (lists connected social accounts), and the headline
  bare-positional form `agentpost "<message>"` that uses Claude to
  generate per-platform posts, renders them in an Ink TUI with
  color-coded character counters, and publishes on confirmation.
- **changelog-bot example** — `examples/changelog-bot/` reads
  `CHANGELOG.md`, finds the most recent release section, asks
  Claude to translate user-facing changes into platform-perfect
  launch posts, and publishes via UniPost's bulk endpoint. Drop-in
  GitHub Action workflow lets users add `UNIPOST_API_KEY` +
  `ANTHROPIC_API_KEY` as repo secrets and tag a release to get
  automatic multi-platform launch posts.

### Changed

- **Capabilities schema** `1.2 → 1.4` (additive, two bumps in one
  sprint). 1.3 dropped the managed-Twitter media restriction.
  1.4 added `FirstCommentCapability.MaxLength` and flipped
  `twitter.first_comment.supported` to `true`.
- **`projects` schema** gains `branding_logo_url`,
  `branding_display_name`, `branding_primary_color`. All three
  optional; `PATCH /v1/projects/{id}` accepts them with hex color
  + HTTPS URL validation.
- **OAuth scopes** — both the BYO Twitter adapter and the Sprint 3
  Connect Twitter connector now request `media.write` in addition
  to the previous `tweet.read tweet.write users.read offline.access`.
  Existing tokens minted before Sprint 4 don't carry the new scope
  and need a re-Connect to upgrade.

### Fixed

- **Sprint 3 PR3 latent bug**: validator codes
  `first_comment_unsupported` and `first_comment_too_long` were
  added to `validate.go` but missed from the handler's
  `fatalErrorCodes` allowlist, so the strict-reject contract for
  Bluesky/Threads silently failed and the publish loop went ahead
  and posted the parent. Audit also surfaced the same gap for the
  Sprint 2 thread codes (`threads_unsupported`,
  `thread_positions_not_contiguous`, `thread_mixed_with_single`)
  and media-library codes (`media_id_not_found`,
  `media_id_not_in_project`, `media_not_uploaded`) — all are now
  registered as fatal. New `fatal_codes_test.go` locks the
  allowlist so a regression of this exact form can't recur.
- **Connect dashboard page rendering** — the `/connect/[platform]`
  page was inheriting the dashboard's dark `#080808` body
  background, making the brand-name span next to the logo nearly
  invisible. New `dashboard/src/app/connect/layout.tsx` wraps
  /connect routes in a fixed-position `#fafafa` container so the
  hosted page is fully decoupled from the dashboard chrome.
- **Managed users SQL** — `MAX(external_user_email) FILTER (...)`
  returned NULL when no row in the group had an email, but sqlc
  inferred the column as plain `string` (the `::TEXT` cast hides
  the nullability). Wrap in `COALESCE(..., '')` so the result is
  always non-null.

### Deprecated

Nothing.

### Breaking

None.

## Sprint 3 — UniPost Connect

### Added

- **UniPost Connect** — multi-tenant hosted OAuth flow that lets
  customers onboard their end users' social accounts without
  touching OAuth credentials or running token refresh themselves.
  Three platforms in v1: **Twitter, LinkedIn, Bluesky**. Meta /
  Google / TikTok deferred to Sprint 4 pending App Review.
  - `POST /v1/connect/sessions` creates a 30-minute hosted-page
    link the customer emails to their end user.
  - `GET /v1/connect/sessions/{id}` (API key) for polling.
  - `GET /v1/public/connect/sessions/{id}?state=…` (no auth,
    oauth_state-protected) for the hosted page to read.
  - Hosted dashboard page at
    `app.unipost.dev/connect/<platform>?session=<id>&state=<state>`
    with platform-specific UIs (Authorize button for OAuth,
    native HTML form for Bluesky).
  - `GET /v1/connect/callback/{platform}` is the OAuth provider
    redirect target; runs token exchange + profile lookup, upserts
    the managed `social_accounts` row, fires `account.connected`,
    and 302s back to the customer's `return_url`.
  - `POST /v1/public/connect/sessions/{id}/bluesky` accepts a
    cross-origin native HTML form (handle + app password) so the
    password never lives in dashboard JS.
- **`account.connected` webhook event** — fires when a Connect flow
  completes successfully (any of the three platforms).
- **Managed token refresh worker** — runs every 5 min, refreshes
  managed-flow tokens within 30 min of expiry, uses
  `FOR UPDATE SKIP LOCKED` so concurrent API instances pick disjoint
  slices and never double-refresh. Success path is silent; failure
  flips `status='reconnect_required'` and fires
  `account.disconnected` with `reason='refresh_failed'`.
- **Bluesky thread support** — `bluesky.text.supports_threads`
  flips to `true`. The runDispatchGroup orchestrator now plumbs
  per-platform thread state (`thread_root_uri/cid` +
  `thread_parent_uri/cid` for Bluesky; `in_reply_to_tweet_id` for
  Twitter) so adapters stay decoupled. Capabilities schema bumps
  `1.1 → 1.2` (additive).
- **Reschedule + cancel for scheduled posts.** `PATCH
  /v1/social-posts/{id}` extends to allow `scheduled_at`-only
  edits when the row is in `status='scheduled'`. New endpoint
  `POST /v1/social-posts/{id}/cancel` flips drafts and scheduled
  posts to `status='cancelled'` under the same optimistic-lock
  pattern Sprint 2 used for draft publish.
- **`GET /v1/social-accounts` filters** — optional
  `?external_user_id=…&platform=…` query params let customers
  look up the row created by a Connect flow without scanning
  the whole project.
- **MCP server v0.4.0** with three new tools:
  `unipost_create_connect_session`, `unipost_reschedule_post`,
  `unipost_cancel_post`.
- **Per-platform character counters** on the hosted preview page.
  Twitter URLs collapse to 23 chars (t.co weighting), Bluesky uses
  `Intl.Segmenter` for grapheme counts, others use UTF-16 code
  units. Hand-rolled to avoid the 200KB `twitter-text` dependency.

### Changed

- **`social_accounts` schema gains six columns:** `status`
  (active | reconnect_required), `connection_type` (byo |
  managed), `connect_session_id`, `external_user_id`,
  `external_user_email`, `last_refreshed_at`. Plus partial unique
  index on `(project_id, platform, external_user_id)` for the
  re-connect upsert path (excludes Bluesky, which upserts on
  `external_account_id` instead because one user may legitimately
  own multiple handles).
- **Capabilities schema** `1.1 → 1.2`. Additive only.
- Re-connecting the same `external_user_id` reuses the existing
  `social_accounts` row (preserves historical post_results FK
  references) instead of creating a duplicate.

### Security / Validation

- **Managed Twitter is text-only in v1.** The OAuth flow does NOT
  request `media.write`. The validator rejects any media on a
  managed Twitter account with the new
  `media_unsupported_for_managed_twitter` fatal error code so
  callers fail fast instead of getting a 403 from Twitter.
- Bluesky form rate-limited at 10/min/IP, OAuth callback at
  60/min/IP — both as defense against credential stuffing /
  callback floods.

### Deprecated

Nothing.

### Breaking

None.

## Sprint 2 (commits `9d89c00..8198902`)

### Added

- **Media library API** (`POST/GET/DELETE /v1/media`). Two-step
  upload: `POST /v1/media` returns a presigned PUT URL the client
  uploads bytes to directly; subsequent publishes reference the
  returned `media_id`. R2-backed (Cloudflare) reusing the bucket
  set up in Sprint 1 for the TikTok PULL_FROM_URL workaround. The
  abandoned-upload sweeper folds into the existing analytics
  refresh worker (no new goroutine) and hard-deletes pending media
  older than 7 days. Per-platform size caps from the capabilities
  table apply on top of a 25 MB global hard ceiling.
- **`platform_posts[].media_ids`** — references rows in the new
  media library. Resolved server-side at adapter dispatch time
  to a fresh 15-minute presigned download URL, then merged with
  the existing `media_urls` list. Adapters see only URLs — zero
  adapter code changes for the new field.
- **Drafts API** — `POST /v1/social-posts` accepts `status="draft"`
  to persist without dispatching. New endpoints:
  - `POST /v1/social-posts/{id}/publish` — atomic draft → publish
    transition with optimistic locking (409 on concurrent publish
    races). Routes through the same publish loop the immediate
    path uses so quota counting / event emission / per-result
    caption persistence stay in one place.
  - `PATCH /v1/social-posts/{id}` — replace draft content. Refuses
    to touch non-draft rows.
  - `DELETE /v1/social-posts/{id}` — already existed; gracefully
    handles drafts (zero results = no platform calls).
- **Hosted preview links** — new `POST /v1/social-posts/{id}/preview-link`
  returns a 24h JWT-signed URL the user can share without exposing
  the API key. The public route `GET /v1/public/drafts/{id}?token=...`
  serves the draft + resolved media URLs to the new dashboard
  preview page at `/preview/[id]` (one column per platform with
  approximate caption count). JWT signed with the existing
  `ENCRYPTION_KEY` value plus an `aud:"preview"` claim — no new
  env var.
- **Twitter threads** — `platform_posts[].thread_position` (1-indexed)
  declares a multi-tweet thread. Posts in the same thread group
  dispatch sequentially with the previous tweet's external_id
  threaded through `opts["in_reply_to_tweet_id"]` so the adapter
  can chain via the v2 reply object. Mid-thread failure stops the
  chain and marks remaining tweets as `failed` with an
  `upstream thread post failed at thread_position N` error.
  Standalone posts and other thread groups still run in parallel.
  Twitter only in Sprint 2; Bluesky / Threads land in Sprint 3.
- **`list_posts` filters + cursor pagination** — `GET /v1/social-posts`
  accepts `?status=draft,published&from=...&to=...&limit=...&cursor=...`.
  Cursor is base64url(`unix_nanos|id`) — keyset pagination on the
  new `(project_id, created_at DESC, id DESC)` index. Stable across
  inserts. Response shape changes from the legacy `{ data, meta }`
  envelope to `{ data, next_cursor }`. Clients loop until
  `next_cursor` is empty. `account_id` and `platform` filters from
  the PRD are deferred to Sprint 3 — they need EXISTS subqueries
  against `social_post_results` and a separate index.
- **Account health endpoint** — `GET /v1/social-accounts/{id}/health`
  returns `{status, last_successful_post_at, last_error?, token_expires_at?}`.
  Status derived from the account's last 10 results (no new
  background workers, no active probing): `disconnected` if the
  account row is flagged, `degraded` if any of the 10 failed,
  otherwise `ok`. `last_error` is categorized via substring match
  (`token_expired`, `rate_limited`, `media_too_large`,
  `url_unverified`, `unknown`).
- **MCP server v0.3.0** with five new / two upgraded tools:
  - **NEW** `unipost_upload_media` — accepts EITHER `base64_data`
    (≤4 MB after inflation, for Claude Desktop local files) OR
    `url` (already-hosted files). Wraps the two-step
    `POST /v1/media` + presigned PUT flow.
  - **NEW** `unipost_create_draft` — alias for `create_post` with
    `status="draft"`.
  - **NEW** `unipost_publish_draft` — wraps the new
    publish-from-draft endpoint.
  - **NEW** `unipost_get_account_health` — wraps the new health
    endpoint.
  - `unipost_create_post` and `unipost_validate_post` gain
    `media_ids` and `thread_position` pass-through.
  - `unipost_list_posts` gains the new filter + cursor params.

### Changed

- **Capabilities schema bumped to `1.1`** with one additive field:
  `text.supports_threads`. Twitter is the only `true` value in
  Sprint 2. Old `1.0` consumers ignore the unknown field — the
  bump is purely additive.
- **`internal/mediaproxy` renamed to `internal/storage`**. Same R2
  client, same env vars. Existing TikTok call site updated. The
  package now houses both `UploadFromURL` (TikTok PULL_FROM_URL
  staging, formerly `Upload`) and the new `PresignPut` / `Head` /
  `PresignGet` / `Delete` helpers for the media library.
- **`SocialPostHandler` constructor takes a `*storage.Client`** for
  resolving `media_ids` to presigned download URLs at dispatch time.
- **`scheduler.go` data race fixed** in PR6 — pre-existing append
  from goroutines was already replaced with a fixed-size outcomes
  slice. PR6 (this sprint) also added per-platform routing logs to
  both publish paths for smoke-test correlation.
- **`runPublishLoop`** refactored to dispatch by group (standalone
  posts in parallel, thread groups serial within / parallel across)
  via the new `groupForDispatch` + `runDispatchGroup` helpers.

### Deprecated

- **`media_urls` on the top-level legacy shape** — still works,
  will warn in v0.4. Use `platform_posts[].media_urls` or
  `platform_posts[].media_ids` for new integrations.

### Breaking

- None in Sprint 2.

### Migration notes

- New env vars: none. Sprint 2 reuses `ENCRYPTION_KEY` for the
  preview JWT signing key (with an audience claim for domain
  separation) and reuses the `R2_*` env vars set up in Sprint 1.
- New DB migration `019_media_and_list_index.sql` adds the `media`
  table and the `(project_id, created_at DESC, id DESC)` index for
  cursor pagination. Safe to run on existing data — no DDL on
  existing tables.

## Sprint 1 (commits `ade0b7e..f006b28`)

### Added

- **`POST /v1/social-posts` accepts `platform_posts[]`** — a per-account
  request shape with its own `caption`, `media_urls`, `platform_options`,
  and `in_reply_to`. Use it whenever you want to tailor content
  per platform (terse on Twitter, long-form on LinkedIn). The legacy
  `caption + account_ids` shape continues to work and is expanded
  server-side into the same internal representation. (PR5)
- **`idempotency_key` on `POST /v1/social-posts`** — pass the same
  key + project within 24h and the original response is returned
  unchanged (no duplicate platform posts). Replays carry an
  `Idempotent-Replay: true` response header. Backed by a partial
  unique index on `(project_id, idempotency_key)` so the lookup
  index stays small. (PR4 + PR5)
- **`social_post_results.caption` column** — every per-account result
  row now persists the exact caption the platform received, so
  analytics, reads, and the dashboard see the truth instead of the
  parent post's single caption. Existing rows are backfilled from
  `social_posts.caption` during the migration. (PR4 + PR5 + PR6)
- **`GET /v1/platforms/capabilities`** — public, cacheable, no-auth
  endpoint returning the per-platform publish-side capability map
  (caption length, image / video count caps, file size hints,
  threading, scheduling, first-comment support). LLM clients call
  this BEFORE drafting so they can size content correctly without
  burning a publish round-trip. Schema versioned (`schema_version: "1.0"`).
- **`GET /v1/social-accounts/{id}/capabilities`** — same shape but
  scoped to one account. Returns 404 for accounts not in the calling
  project. (PR1)
- **`POST /v1/social-posts/validate`** — pure pre-flight that runs
  the same checks `Create()` runs but writes nothing and calls no
  external APIs. Returns `{ valid, errors[], warnings[] }`. p95 < 1ms
  in benchmarks. (PR2 + PR3)
- **Webhook events fire on publish** — `post.published`, `post.partial`,
  and `post.failed` are now enqueued by both the synchronous publish
  path (`POST /v1/social-posts`) and the scheduler when a scheduled
  post fires. `account.disconnected` fires when an account is
  disconnected via `DELETE /v1/social-accounts/{id}`. Best-effort: a
  failed enqueue NEVER blocks or fails the publish path. (PR7)
- **New webhook management endpoints**: `GET /v1/webhooks/{id}`,
  `PATCH /v1/webhooks/{id}`, `DELETE /v1/webhooks/{id}`,
  `POST /v1/webhooks/{id}/rotate`. (PR8)
- **MCP server v0.2.0** with three new / upgraded tools:
  - `unipost_create_post` now accepts `platform_posts[]` alongside the
    legacy `caption + account_ids` shape. The tool description tells
    Claude / GPT to prefer `platform_posts[]` for multi-platform
    fan-out where the message should differ across networks.
  - `unipost_validate_post` (NEW) — preflight a draft against the
    capability map without publishing.
  - `unipost_get_capabilities` (NEW) — fetch the static
    per-platform capability map.
  (PR9)
- **Per-platform routing log** at INFO level on every adapter
  dispatch in both the immediate (`publish: dispatching to adapter`)
  and scheduled (`scheduler: dispatching to adapter`) flows, showing
  `account_id`, `platform`, and `caption_preview` (first 40 runes +
  ellipsis) so operators can verify per-platform routing in Railway
  logs without instrumenting each adapter individually. (PR6)

### Changed

- **`POST /v1/social-posts/validate` and `POST /v1/social-posts` share
  one validation path** via `platform.ValidatePlatformPosts`. The
  publish path filters to STRUCTURAL errors only (caption length,
  media count, mixing, schedule, threading) so legacy partial-success
  semantics for disconnected/not-in-project accounts are preserved.
- **`Create` validation errors return as a structured `error.issues`
  array** in addition to the existing `error.code = "VALIDATION_ERROR"`.
  Existing error-code switching keeps working; new clients can read
  `error.issues[]` for richer per-field detail.
- **`social_posts.metadata` is now a versioned JSON blob** with
  `schema_version: 2` carrying the full `platform_posts[]` shape.
  Existing rows (v1: `account_ids` + `platform_options`) continue to
  read correctly via the `DecodePostMetadata` v1 fallback path. (PR5 + PR6)
- **Scheduler `publishPost` rewrite** — reads v2 metadata to dispatch
  per-account captions, fixes a pre-existing data race on the `results`
  slice (each goroutine now writes to a fixed-size slot indexed by
  input position), and persists `social_post_results.caption` for every
  result. (PR6)

### Breaking

- **`POST /v1/webhooks` no longer accepts a client-provided `secret`.**
  The signing secret is generated server-side as `whsec_` + 32 hex
  chars and returned in the Create response **exactly once** (and once
  more on `POST /v1/webhooks/{id}/rotate`). Subsequent reads via
  `GET /v1/webhooks` and `GET /v1/webhooks/{id}` return only a
  `secret_preview` (`whsec_xx…`) instead of the plaintext.

  Migration for existing webhooks: existing `webhooks.secret` rows are
  preserved as-is — your existing subscribers continue to verify
  signatures using whatever secret was originally set. To get a
  preview-only response from the API, call `POST /v1/webhooks/{id}/rotate`
  to generate a fresh secret.

  Callers passing `{ "secret": "..." }` to `POST /v1/webhooks` will
  now receive an HTTP 422:

      {
        "error": {
          "code": "VALIDATION_ERROR",
          "message": "secret is generated server-side; do not provide one ..."
        }
      }

  Update SDKs / scripts to drop the `secret` field and capture the
  one-shot value from the Create response instead.

### Fixed

- **Scheduler data race** on the `results` slice — pre-existing bug
  where multiple goroutines did `results = append(...)` without
  synchronization. Replaced with a fixed-size `outcomes` slice
  indexed by input position. (PR6)
- **Per-platform captions silently dropped on scheduled posts** —
  before this sprint, the scheduler always used the parent post's
  single caption regardless of how the post was created. After PR6
  it reads the per-account caption from the persisted v2 metadata. (PR6)
