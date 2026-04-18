# AgentPost — Design Handoff for UniPost

> **Audience:** Claude Code working inside the UniPost codebase.
> **Goal:** Give you everything you need to (a) verify the assumptions below against the real codebase, (b) confirm or correct the gap analysis, and (c) propose a concrete sprint plan to make UniPost ready for AgentPost.
> **Author:** drafted in a Cowork session with the UniPost founder (Xiaobo). Treat this doc as a *proposal*, not a final spec — your job is to ground it in real code.

---

## 0. How to use this document

1. Read §1–§4 to understand what AgentPost is and why we're proposing it.
2. Read §5 carefully — these are **assumptions** about UniPost's current API surface based only on what is exposed via the public MCP tools (`unipost_create_post`, `unipost_list_accounts`, `unipost_list_posts`, `unipost_get_post`, `unipost_get_analytics`). **Verify each assumption against the real codebase before trusting any later section.**
3. Read §6 (gap analysis) and correct it based on your verification.
4. Read §7 (P0 API specs) and propose concrete diffs / file paths in the UniPost repo.
5. Answer the open questions in §10.
6. Produce a sprint plan (§9 has a starting point).

---

## 1. TL;DR

**AgentPost** is a proposed open-source companion project to UniPost. It is *the first AI-native social media manager*: instead of opening a "compose" UI and writing a post, the user describes what happened in plain language, and an LLM (Claude or GPT) decides which platforms to post to, rewrites the copy for each platform's tone and limits, shows a preview, and publishes via the UniPost API.

```bash
$ agentpost "shipped webhook support in my MCP server today after 3 days of debugging 🎉"
🤖 Drafting posts for 4 platforms...
[per-platform previews shown]
? What now? › Post all
📤 Posting...
  ✓ Twitter   → https://x.com/...
  ✓ LinkedIn  → https://linkedin.com/...
  ✓ Threads   → https://threads.net/...
  ✓ Bluesky   → https://bsky.app/...
Done in 4.2s.
```

**Why build it:**

1. **Strategic moat for UniPost.** UniPost's differentiator is "MCP-native unified social API." AgentPost is the canonical demo of that promise. Competitors (Zernio, Post for Me) don't have anything comparable.
2. **Top-of-funnel.** AgentPost is open source, MIT-licensed, with 5 built-in examples. Each example is a separate SEO / blog / HN post entry point. Every install routes the user back to UniPost for credentials.
3. **Dogfooding.** Building AgentPost will surface every rough edge in UniPost's API. The gaps in §6 below are the first batch.
4. **Differentiation.** AgentPost is the kind of product that goes viral on Hacker News and X among indie hackers — exactly the audience UniPost's free tier is trying to capture.

**What this doc is NOT:** a marketing brief, a launch plan, or a finalized spec. It is a *technical scoping document* whose purpose is to figure out what UniPost needs to build first.

---

## 2. Target users

| Persona | Description | Entry point |
|---|---|---|
| **Lin** | Indie hacker, builds in public, lives in the terminal. Posts daily snippets to 4–5 platforms manually today. | CLI (`npx agentpost`) |
| **Sara** | Non-developer marketer at a small SaaS. Writes weekly product updates. Won't touch a CLI. | Web app + Slack bot |
| **AI agent author** | Building a Claude-powered assistant that needs to post on behalf of users. | UniPost MCP server (no AgentPost UI at all) |

AgentPost serves the first two. UniPost's MCP server already serves the third.

---

## 3. User journeys

### 3.1 CLI (Lin)

**Install + first run:**

```bash
$ npm install -g agentpost
$ agentpost init
Welcome to AgentPost!

? Which LLM do you want to use? › Claude
? Paste your Anthropic API key: › sk-ant-...
? Open browser to sign in to UniPost? (Y/n) › Y
[browser opens unipost.dev/auth/cli?token=xxx]
[user signs in, connects accounts]
✓ Connected to UniPost (5 accounts: Twitter, LinkedIn, Threads, Bluesky, IG)
✓ Saved config to ~/.agentpost/config.json
```

**Daily use:**

```bash
$ agentpost "finally got webhooks working in my MCP server, 3 days of hair pulling"

🤖 Drafting posts for 4 platforms...

┌─ Twitter ─────────────────────────────────────┐
│ 3 days of hair pulling later... webhooks are  │
│ alive in my MCP server 🎉                     │
│ #buildinpublic                                │
└───────────────────────────────────────────────┘
┌─ LinkedIn ────────────────────────────────────┐
│ Shipped webhook support in my MCP server      │
│ today after three days of debugging.          │
│ ...                                           │
└───────────────────────────────────────────────┘
┌─ Threads ─────────────────────────────────────┐
│ webhooks: 1                                   │
│ me: 0                                         │
│ but they work now 🥲                          │
└───────────────────────────────────────────────┘
┌─ Bluesky ─────────────────────────────────────┐
│ shipped webhooks in my MCP server today       │
│ after 3 days of debugging                     │
└───────────────────────────────────────────────┘

? What now?
  ❯ Post all
    Edit one
    Skip a platform
    Regenerate
    Cancel

[Lin presses Enter]
📤 Posting...
  ✓ Twitter   → https://x.com/lin/status/...
  ✓ LinkedIn  → https://linkedin.com/posts/...
  ✓ Threads   → https://threads.net/@lin/post/...
  ✓ Bluesky   → https://bsky.app/profile/lin.bsky.social/...
Done in 4.2s.
```

### 3.2 Web (Sara)

`agentpost.dev` (a static Next.js site, deployed by the open-source maintainer) — sign in with the same UniPost account, type a sentence, see preview cards, click "Post all". Same engine, just rendered as cards in a browser.

### 3.3 Slack bot

```
Sara: /post we just hit 100 paying customers 🎉
AgentPost: Here's what I'll post on 4 platforms... [card previews]
            [Approve all] [Edit] [Cancel]
Sara: [Approve all]
AgentPost: ✓ Posted! Links: ...
```

### 3.4 Built-in examples (5 turnkey "agents")

Each is a separate npm package, separate README, separate blog post:

1. `agentpost-github-release-bot` — listens for GitHub releases via webhook → posts to all platforms
2. `agentpost-rss-bridge` — RSS / Substack / podcast feed → multi-platform
3. `agentpost-changelog-bot` — parses `CHANGELOG.md` updates → "what's new" posts
4. `agentpost-daily-standup` — Slack input → build-in-public daily log
5. `agentpost-thread-replay` — long article → Twitter thread + LinkedIn long-form

These are AgentPost's real distribution channels.

---

## 4. Architecture

```
                  ┌─────────────┐
   user input ──→ │  AgentPost  │
                  │ (CLI / Web /│
                  │   Slack)    │
                  └──────┬──────┘
                         │
                ① LLM generates N drafts
                         │
                         ▼
                  ┌─────────────┐
                  │  Preview UI │  ← user reviews + edits + confirms
                  │ (rendered   │
                  │  locally)   │
                  └──────┬──────┘
                         │
                ② user clicks "Post"
                         │
                         ▼
                  ┌─────────────┐
                  │   UniPost   │  ← only does the "send"
                  │     API     │
                  └──────┬──────┘
                         │
              ┌──────────┼──────────┐
              ▼          ▼          ▼
            Twitter  LinkedIn   Threads ...
```

**Boundary principle:**
- **AgentPost = the brain.** It owns: LLM prompting, per-platform copy generation, preview UI, user interaction, scheduling logic.
- **UniPost = the hands.** It owns: OAuth, token refresh, media upload, platform-specific quirks, the actual `POST /accounts/:id/posts` HTTP call.

Anything that requires understanding *what* to post belongs to AgentPost. Anything that requires understanding *how* to talk to a specific platform belongs to UniPost.

This boundary is important because it means **future agents besides AgentPost can be built on top of UniPost** (a chatbot, an automation tool, a Zapier-style integration) without UniPost having to absorb their concerns.

### 4.1 Preview: where does it live?

We considered three options:

| Option | Where preview lives | Pros | Cons |
|---|---|---|---|
| **A** | Inside AgentPost (CLI/Web/Slack renders it locally) | Single-context UX; UniPost stays simple | AgentPost has to implement 3 UIs |
| **B** | UniPost hosts a `/draft/:id` page; AgentPost redirects | UniPost gets the traffic; pixel-perfect platform previews | Breaks CLI flow; UniPost has to build draft + review UI |
| **C** | Hybrid: A by default, B with `--web` flag | Best of both; CLI is fast, mobile-friendly review available | Two code paths to maintain |

**Decision: Phase 1 = Option A only. Phase 2 = add Option B as `--web`. The Slack bot in Phase 2 will use `--web` by default because Slack blocks can't render real platform previews.**

This decision drives the gap analysis: a **Drafts API** (§6 P1 #6) is *not* needed for Phase 1.

---

## 5. Assumptions about UniPost's current state — PLEASE VERIFY

> ⚠️ **The following is what I know from outside the codebase, via the published MCP tools only.** Before relying on §6 or §7, please open the UniPost repo and confirm or correct each statement. Mark each one ✅ correct / ⚠️ partially correct / ❌ wrong, and where you mark anything other than ✅ please explain.

| # | Assumption | Verify |
|---|---|---|
| A1 | The post creation endpoint accepts a single `caption` field that is sent as-is to every platform in `account_ids`. | |
| A2 | There is no `platform_posts` / per-account-caption structure in the request body. | |
| A3 | Media can be supplied as `media_urls` (publicly fetchable URLs) or as `media_ids` created via `POST /v1/media` + presigned upload. There is no raw multipart file body on `create_post`; local files go through the media library first. | |
| A4 | The 6 supported platforms today are: Instagram, TikTok, YouTube, Threads, LinkedIn, Bluesky. X (Twitter), Facebook, Pinterest, Reddit are **not** supported. | |
| A5 | There is one `UniPost account` per developer. All connected social accounts belong to that single owner. There is no notion of "App" or "end user" — i.e. no Stripe-Connect-style multi-tenant model. | |
| A6 | API keys are user-scoped, generated in a dashboard, and passed as `Authorization: Bearer <key>` (or similar). There is no OAuth-for-third-party-apps flow. | |
| A7 | There is no public endpoint that returns per-platform capability metadata (max caption length, required media, supported aspect ratios, etc.). Such limits, if enforced, are enforced server-side at publish time. | |
| A8 | There is no validation / pre-flight endpoint. The only way to know if a post will be accepted is to call `create_post` and read the result. | |
| A9 | There is no webhook system for post status updates. Callers must poll `get_post` to learn about async failures. | |
| A10 | There is no `drafts` resource. A post either exists (queued/published/failed) or it doesn't. | |
| A11 | `create_post` does not support a `in_reply_to` field. There is no concept of a thread or a reply chain. | |
| A12 | `list_posts` returns the most recent N posts in reverse chronological order with no filtering by status, platform, date range, or account. | |
| A13 | There is no per-account "health check" endpoint. Token expiry is only discovered when a publish fails with `account is disconnected`. | |
| A14 | There is no `usage` / `quota` endpoint. Rate limits, if any, are not exposed to the caller. | |
| A15 | The TikTok integration uses TikTok's "pull from URL" / "photo init" flows. Tokens expire frequently and the only recovery is for the user to re-authorize via the dashboard. | |
| A16 | Scheduling is supported via a `scheduled_at` ISO timestamp on `create_post`. There is no separate `schedules` resource. | |

**For each ❌ or ⚠️ above, please provide:** the file path(s) in the codebase that contradict the assumption, and a short note describing the actual behavior.

---

## 6. Gap analysis — what UniPost needs to add

> Each gap is tagged P0 / P1 / P2. P0 = AgentPost cannot ship without it (or ships with bad UX). P1 = AgentPost ships, but a competitor with this feature would beat us. P2 = nice to have, second release.
>
> **Please re-prioritize after verifying §5.** Some of these gaps may already be partially solved in the codebase.

### 🔴 P0 — required for AgentPost v0.1

#### G1. Per-platform caption in a single request

**Problem.** AgentPost generates a different caption per platform (Twitter 280 chars, LinkedIn 1200 chars, Threads short and casual, Bluesky community-toned). Today it would have to call `create_post` N times — once per account — losing atomicity, complicating error handling, and making "one logical post" hard to track in `list_posts`.

**Proposed shape.** Extend `POST /posts` to accept either the existing flat shape *or* a new `platform_posts` array:

```json
POST /posts
{
  "platform_posts": [
    {
      "account_id": "8558370d-...",
      "caption": "3 days of hair pulling later... 🎉",
      "media_urls": ["https://..."]
    },
    {
      "account_id": "dae7cb19-...",
      "caption": "Shipped webhook support today after three days of debugging...",
      "media_urls": ["https://..."]
    }
  ],
  "idempotency_key": "agentpost-2026-04-07-abc123"
}
```

The legacy `{caption, account_ids, media_urls}` shape stays supported (it just expands server-side into N identical `platform_posts`).

**Response:** existing shape, with one `results` entry per `platform_posts` entry. The top-level `id` represents the *logical group*; each result has its own `external_id`.

**Why P0:** Without this, AgentPost's central feature ("rewrite for each platform") is hostile to use.

#### G2. Platform capability metadata API

**Problem.** AgentPost's LLM prompts need to know each platform's hard limits *before* generating copy, otherwise it generates a 400-char tweet and we discover the failure at publish time.

**Proposed shape.**

```
GET /platforms/capabilities
→ {
    "twitter": {
      "text": {"max_length": 280, "supports_links": true, "supports_hashtags": true},
      "media": {
        "images": {"max_count": 4, "max_bytes": 5_000_000, "formats": ["jpg","png","gif","webp"]},
        "videos": {"max_count": 1, "max_bytes": 512_000_000, "formats": ["mp4","mov"], "max_duration_seconds": 140}
      },
      "thread": {"supported": true, "max_posts": 25},
      "scheduling": {"supported": true, "min_lead_seconds": 0}
    },
    "instagram": {
      "text": {"max_length": 2200, ...},
      "media": {"required": true, "images": {"min_count": 1, ...}, ...},
      ...
    },
    ...
  }
```

**Implementation note.** This can be a static JSON file in the repo, served as a 200 response. It does *not* need to query each platform's API at runtime. The schema must be **stable and versioned** because AgentPost will ship hard-coded TypeScript types based on it.

**Why P0:** Without this, every AgentPost upgrade has to chase platform changes by reading docs.

#### G3. Pre-flight validation API

**Problem.** AgentPost wants to tell the user "this draft will fail because X" *before* clicking Post. Today the only way to find out is to actually publish.

**Proposed shape.**

```
POST /posts/validate
{
  "platform_posts": [...same shape as G1...]
}
→ {
    "valid": false,
    "errors": [
      {"account_id": "...", "platform": "twitter", "field": "caption", "code": "exceeds_max_length", "message": "Caption is 312 chars; max is 280", "actual": 312, "limit": 280},
      {"account_id": "...", "platform": "instagram", "field": "media_urls", "code": "missing_required", "message": "Instagram requires at least one image"}
    ],
    "warnings": [
      {"account_id": "...", "platform": "linkedin", "code": "no_first_comment", "message": "Posts with a first comment historically perform better"}
    ]
  }
```

**Implementation note.** Validation logic should be pure / deterministic — reuse the same code that the publish endpoint runs server-side, just stop before the actual outbound HTTP call. **Do not** call the platform API here (no network round-trip, no token use).

**Why P0:** Massive UX win for the cost of refactoring publish-side validation into a callable function.

#### G4 (deferred). Multi-tenant OAuth / "UniPost App" model

**Status: NOT in P0 for v0.1.** Originally proposed as P0, but we have a hack that lets us defer it:

> **The hack:** For AgentPost v0.1, every end user signs up at unipost.dev themselves and pastes their personal API key into `~/.agentpost/config.json`. AgentPost is a thin client of the user's *own* UniPost account. No multi-tenancy needed.

This is acceptable for indie hackers (the v0.1 audience) but **must** be revisited before AgentPost has a real Web/Slack experience for non-developers (Sara persona). Treat G4 as P1 below.

### 🟡 P1 — needed for AgentPost v0.5+ / competitive parity

#### G5. Multi-tenant App / Connect model

(See G4 above.) Stripe-Connect-style: developers register an "App" in the UniPost dashboard, get `client_id` / `client_secret`, run end users through a UniPost-hosted OAuth flow, receive a per-end-user token, and call all post APIs with that token. UniPost meters and bills per-App.

This is the single largest piece of work in this whole document. It is correctly **deferred** until AgentPost has product-market fit.

#### G6. Webhooks for post status

```
POST /webhooks
{ "url": "https://agentpost.dev/hooks/unipost", "events": ["post.published","post.failed"] }
```

UniPost POSTs the URL when a post finishes. Standard signature header (`X-Unipost-Signature: t=...,v1=...`). Replaces the polling AgentPost would otherwise have to do for async failures (TikTok in particular often fails minutes after the API returns 200).

#### G7. Drafts API (enables Option B / `--web`)

```
POST /drafts                → {draft_id, preview_url, expires_at}
GET /drafts/:id
PATCH /drafts/:id           (edit a single platform_post)
POST /drafts/:id/publish    → standard post-creation response
DELETE /drafts/:id
```

`preview_url` is a UniPost-hosted page that renders pixel-accurate previews (using the same capability data from G2). Unlocks the `agentpost --web` flag and the Slack bot's primary flow.

#### G8. Media upload API (binary, not URL)

```
POST /media (multipart/form-data; up to ~50 MB)
→ {media_id, url, expires_at}
```

`media_id` can then be passed in a `media_ids` array of `create_post` (or callers can keep using `media_urls` for already-hosted assets). Removes the requirement that AgentPost users host their own image storage.

Optional: a multipart resumable variant for video uploads (TUS protocol) — only if real users complain.

#### G9. Reply / thread support

Add `in_reply_to` to `create_post` (or `platform_posts[]` entries). Required for Twitter threads, LinkedIn comments, Threads chains. Enables the `agentpost-thread-replay` example.

#### G10. Quota / rate limit visibility

Either:
- Add `X-RateLimit-Remaining`, `X-RateLimit-Limit`, `X-RateLimit-Reset` headers on every response, **or**
- Add `GET /usage` returning `{period, posts_used, posts_limit, by_platform: {...}}`.

AgentPost's CLI will use this to warn users approaching their cap.

### 🟢 P2 — second release / nice-to-have

| # | Gap | Notes |
|---|---|---|
| G11 | X (Twitter), Facebook Pages, Pinterest as supported platforms | Highest user-demand additions. X is now pay-per-use (~$5/mo for 500 posts), so cheap to add. |
| G12 | Reddit support | Skip until enterprise-priced ($12k/yr API floor). Add a "Coming soon — request access" waitlist landing page instead. |
| G13 | Bulk publish (`POST /posts/bulk` with N posts) | Used by `rss-bridge` example. Otherwise N round-trips. |
| G14 | `list_posts` filters: `status`, `platform`, `from`, `to`, `account_id`, pagination cursor | Enables an "AgentPost stats" subcommand. |
| G15 | Per-account health check (`GET /accounts/:id/health`) | Surfaces token expiry before publish failure. |
| G16 | Aggregated analytics (`GET /analytics/summary?from=...&group_by=platform`) | Powers an "insight" feature in AgentPost. |
| G17 | First comment / pinned comment per platform | Performance booster; some competitors have this. |
| G18 | Mentions / hashtag autocomplete API | Lets AgentPost suggest @mentions while drafting. |

**Explicitly NOT recommended:**

| Gap | Why not |
|---|---|
| Built-in image generation (DALL-E / Imagen passthrough) | Scope creep. Let AgentPost or the user supply images. |
| Built-in LLM rewriting | Same — that's AgentPost's job, not UniPost's. |
| Inbox / DM / comment management | Huge separate product area, requires Meta App Review, GDPR work. Address in a separate handoff doc. |

---

## 7. Detailed P0 API specs

> Below are concrete request/response shapes for G1, G2, G3. Treat them as a starting point; please reconcile them with UniPost's existing conventions (auth header name, error envelope, snake_case vs camelCase, etc.).

### 7.1 Per-platform post creation (G1)

**Endpoint:** `POST /v1/posts`

**Request — new shape:**

```json
{
  "platform_posts": [
    {
      "account_id": "8558370d-b957-450c-a399-e2c0838a441a",
      "caption": "string, required, max length per platform G2",
      "media_urls": ["https://...", "..."],
      "media_ids": ["med_..."],
      "scheduled_at": "2026-04-08T10:00:00Z",
      "in_reply_to": {"unipost_post_id": "..."}
    }
  ],
  "idempotency_key": "client-supplied UUID, optional but recommended"
}
```

**Request — legacy shape (still supported):**

```json
{
  "caption": "string",
  "account_ids": ["...", "..."],
  "media_urls": ["..."],
  "scheduled_at": "..."
}
```

The server expands the legacy shape into `platform_posts` internally so there is exactly one code path downstream.

**Response:**

```json
{
  "id": "post_grp_4514d94b-...",
  "created_at": "2026-04-07T18:29:34Z",
  "status": "published | partial | failed | scheduled",
  "results": [
    {
      "social_account_id": "...",
      "platform": "twitter",
      "account_name": "lin",
      "status": "published | failed | scheduled",
      "caption": "the caption that was actually sent (post-truncation if any)",
      "external_id": "1234567890",
      "external_url": "https://x.com/lin/status/1234567890",
      "published_at": "2026-04-07T18:29:43Z",
      "error_code": null,
      "error_message": null
    }
  ]
}
```

**Error envelope:**

Top-level 4xx if the *request* is malformed (bad UUIDs, invalid JSON). Top-level 200 with `status: "failed"` or `"partial"` if the *publish* failed at one or more platforms — never bury platform errors inside a 4xx.

**Idempotency:** if `idempotency_key` is supplied and matches a previous request from the same caller within 24h, return the prior response unchanged.

**Open question:** should `platform_posts` be an array or an object keyed by `account_id`? Array is simpler and supports same-account-twice (for a thread). Recommend array.

### 7.2 Platform capabilities (G2)

**Endpoint:** `GET /v1/platforms/capabilities`

**Auth:** none required (or accept anonymous). It's static data.

**Response:**

```json
{
  "schema_version": "1.0",
  "generated_at": "2026-04-07T00:00:00Z",
  "platforms": {
    "twitter": {
      "display_name": "X (Twitter)",
      "text": {
        "max_length": 280,
        "min_length": 1,
        "supports_links": true,
        "supports_hashtags": true,
        "supports_mentions": true,
        "newlines_allowed": true
      },
      "media": {
        "required": false,
        "images": {
          "min_count": 0,
          "max_count": 4,
          "max_bytes": 5000000,
          "formats": ["jpg", "jpeg", "png", "webp", "gif"],
          "max_dimensions": {"width": 4096, "height": 4096}
        },
        "videos": {
          "min_count": 0,
          "max_count": 1,
          "max_bytes": 512000000,
          "formats": ["mp4", "mov"],
          "min_duration_seconds": 0.5,
          "max_duration_seconds": 140,
          "aspect_ratios": ["1:1", "16:9", "9:16"]
        },
        "image_video_mutually_exclusive": true
      },
      "thread": {"supported": true, "max_posts": 25},
      "scheduling": {"supported": true, "min_lead_seconds": 0, "max_lead_days": 365},
      "first_comment": {"supported": false}
    },
    "instagram": { ... },
    "tiktok": { ... },
    "youtube": { ... },
    "threads": { ... },
    "linkedin": { ... },
    "bluesky": { ... }
  }
}
```

**Scoped variant (also expose):** `GET /v1/accounts/:id/capabilities` — same data, narrowed to just the platform of that account, plus any per-account quirks (e.g., business vs personal account differences).

### 7.3 Pre-flight validation (G3)

**Endpoint:** `POST /v1/posts/validate`

**Request:** identical to `POST /v1/posts` (G1).

**Response:**

```json
{
  "valid": false,
  "errors": [
    {
      "platform_post_index": 0,
      "account_id": "...",
      "platform": "twitter",
      "field": "caption",
      "code": "exceeds_max_length",
      "message": "Caption is 312 characters; Twitter limit is 280",
      "actual": 312,
      "limit": 280,
      "severity": "error"
    },
    {
      "platform_post_index": 1,
      "account_id": "...",
      "platform": "instagram",
      "field": "media_urls",
      "code": "missing_required",
      "message": "Instagram requires at least one image or video",
      "severity": "error"
    }
  ],
  "warnings": [
    {
      "platform_post_index": 2,
      "account_id": "...",
      "platform": "linkedin",
      "code": "no_link_preview",
      "message": "LinkedIn posts with a link preview historically get 1.7x more impressions",
      "severity": "warning"
    }
  ]
}
```

**Implementation rule:** validation must be a **pure function** of the request body + capability data + account state (which platform, business/personal). It must not call any external platform's API. It must complete in < 50 ms p95.

**Error code list to define centrally** (so AgentPost can map them to UI strings):
`exceeds_max_length`, `below_min_length`, `missing_required`, `unsupported_format`, `file_too_large`, `dimensions_out_of_range`, `aspect_ratio_unsupported`, `duration_out_of_range`, `account_disconnected`, `account_token_expired`, `quota_exceeded`, `scheduled_too_soon`, `scheduled_too_far`, `unsupported_in_reply_to`, `unknown`.

---

## 8. Sketch: G6 webhooks (P1, included for completeness)

**Endpoint to register:**

```
POST /v1/webhooks
{
  "url": "https://agentpost.dev/hooks/unipost",
  "events": ["post.published", "post.failed", "post.partial", "account.disconnected"],
  "active": true
}
→ { "id": "wh_...", "secret": "whsec_..." }
```

**Outbound delivery format:**

```
POST https://agentpost.dev/hooks/unipost
X-Unipost-Signature: t=1712512345,v1=hex(hmac_sha256(secret, "t.body"))
Content-Type: application/json

{
  "id": "evt_...",
  "type": "post.published",
  "created_at": "2026-04-07T18:29:43Z",
  "data": { ... full post object ... }
}
```

**Retry policy:** 5 attempts with exponential backoff (10s, 1m, 10m, 1h, 6h). After 5 failures, mark webhook unhealthy and email the developer.

---

## 9. Suggested phasing

> A starting proposal. **Please rewrite this after you verify §5 — sequencing depends on which gaps are already partially built.**

### Sprint 1 (week 1–2): the "AgentPost-ready" sprint
- G2: capabilities API (1 day; static JSON)
- G3: validation API (3 days; refactor existing publish-side checks into a pure function)
- G1: per-platform `platform_posts` request shape (4–5 days; biggest piece, requires DB schema + downstream worker changes)
- Tests + docs + a `CHANGELOG.md` entry

**Exit criterion:** AgentPost v0.1 can be built end-to-end against the new APIs. UniPost dogfoods the new endpoints by migrating its own (hypothetical) dashboard to use them.

### Sprint 2 (week 3–4): the AgentPost launch enabler
- G8: media upload (binary)
- G9: reply / `in_reply_to`
- Bug-fixes from AgentPost dogfooding
- Public API docs + Postman collection

### Sprint 3 (week 5–6): observability + AgentPost v0.5
- G6: webhooks
- G10: quota / rate-limit headers
- G15: account health check
- G14: `list_posts` filters

### Sprint 4 (week 7–9): platform expansion
- G11: X / Twitter integration
- G11: Facebook Pages integration
- G11: Pinterest integration

### Sprint 5+ (week 10+): the multi-tenant rewrite
- G5 / G7: App + Drafts model (the big one)
- AgentPost v1.0 with Web + Slack frontends

### Out of scope for this doc
- Inbox / DM (separate handoff)
- Reddit (deferred, see G12)
- LLM passthrough, image-gen passthrough (out)

---

## 10. Open questions

Please answer these (or flag them as needing the founder's input) when you reply.

1. **§5 verification.** Which assumptions A1–A16 are wrong? Provide file paths where the actual code differs.
2. **Existing internals.** Does UniPost already have an internal "publish job" abstraction that could be repurposed for `platform_posts` without a schema migration, or will G1 require a new posts/post_results table split?
3. **Caption storage.** Today, is the `caption` stored once per post or once per account? G1's response shape needs to surface the actual caption sent to each platform (after any server-side truncation/sanitization).
4. **Idempotency.** Is there an existing idempotency mechanism for `create_post`? If yes, what's the key format and TTL?
5. **Capability source of truth.** For G2, can you find a place in the codebase where per-platform limits are already encoded (e.g., in publish handlers)? If yes, recommend extracting them to a single static config file.
6. **Validation reuse.** For G3, is the publish-time validation a single function or scattered across handlers? G3 is much cheaper if it's already centralized.
7. **Rate limiting.** Does UniPost currently rate-limit anything? If yes, where and how — so that G10 surfaces real numbers.
8. **TikTok auth fragility.** The session this doc was drafted in showed TikTok tokens dying repeatedly. Is there a refresh strategy in code, or does every TikTok account need manual reconnection? This affects G15's design.
9. **Naming convention.** snake_case or camelCase in JSON bodies? Plural or singular resource names? (`/v1/posts` vs `/v1/post`?) Match existing.
10. **Versioning.** Is the API versioned (`/v1/...`) or unversioned today? G1's shape change is backward-compatible, but G2/G3 are new endpoints — should they live under `/v1/` or unversioned?
11. **MCP server alignment.** The MCP tools currently exposed (`unipost_create_post` etc.) take the legacy single-caption shape. Should they be updated to expose `platform_posts` too, or is it OK to leave the MCP layer behind the REST API on this?
12. **Hosting for `--web` previews (Phase 2).** Will the draft preview page be part of the main `unipost.dev` app, a subdomain, or a separate service? Affects G7 design.

---

## 11. Appendix: how AgentPost will bill / monetize (for context only)

AgentPost itself is **free, open source, MIT licensed**. There is no AgentPost paid plan. Monetization happens entirely through UniPost: AgentPost users sign up for UniPost (free tier), then upgrade to a paid UniPost plan when they outgrow free quota. AgentPost is pure top-of-funnel.

This is why AgentPost's design relentlessly preserves the "Powered by UniPost" linkage in CLI output, README, docs, and example projects. Every successful AgentPost install is a UniPost lead.

---

## 12. Appendix: competitive context (for context only)

AgentPost competes with:
- **Buffer / Hootsuite:** old-world UI-first social schedulers. AgentPost wins on "describe, don't compose."
- **Postiz, Mixpost (open source schedulers):** AgentPost wins on AI-native rewriting and CLI/agent ergonomics.
- **Zernio's `ZernFlow` (their open-source ManyChat alternative):** Zernio is also using open source as a UniPost-style funnel. AgentPost's differentiator over ZernFlow is the agent / LLM angle, where Zernio is "visual chatbot builder."
- **Post for Me's MCP server:** they have an MCP server but no AgentPost-equivalent demo. Building AgentPost first is how UniPost wins the MCP-native narrative.

There is no current competitor doing exactly "natural language → multi-platform AI rewriter → publish." This is the wedge.

---

## 13. Final instructions for Claude Code

When you reply to the founder:

1. **Verify §5 first.** Walk the codebase and produce a corrected version of the table.
2. **Re-prioritize §6** based on what's already built.
3. **Reconcile §7's API specs** with UniPost's existing conventions. Propose concrete file paths and diffs (don't apply them yet — produce a written proposal first).
4. **Answer §10's open questions** to the extent the codebase reveals the answers.
5. **Produce a revised §9 sprint plan** using real estimates based on the actual code.
6. **Flag anything in this doc that is wrong, naive, or missing.** This doc was drafted from outside the codebase by someone who only sees the public MCP tools — your job is to be the reality check.

End with a recommendation: **"go" or "no-go" on Sprint 1**, and if "go", a list of issues / PRs you'd open this week.

---

*End of handoff document.*
