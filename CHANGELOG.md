# UniPost Changelog

All notable changes to the UniPost API + MCP server are documented here.

The format is loosely based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
this project does not yet follow strict semantic versioning across deploys —
each section heading below corresponds to a sprint, not a published version.

## [Unreleased]

### Added

- **`POST /v1/social-posts` accepts `platform_posts[]`** — a per-account
  request shape with its own `caption`, `media_urls`, `platform_options`,
  and (future) `in_reply_to`. Use it whenever you want to tailor content
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
