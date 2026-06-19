# Docs AI Search Full PRD Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the remaining Docs AI Search PRD so natural-language docs questions retrieve the right docs, answer only with source coverage, collect feedback, and pass offline evals before release.

**Architecture:** Keep API Reference pages endpoint-shaped. Move answer quality into a curated content-level docs chunk registry, deterministic intent-aware retrieval, optional AI refinement after source coverage, and eval tests that exercise real retrieval behavior. Use deterministic answers for core workflows so production quality does not depend on model availability.

**Tech Stack:** Next.js App Router, Vercel AI SDK, Node `node:test`, TypeScript docs registry.

---

### Task 1: Add Failing Docs AI Evals

**Files:**
- Create: `dashboard/tests/docs-ai-search-evals.test.mjs`
- Modify: `dashboard/package.json`

- [ ] **Step 1: Write evals that execute real retrieval behavior**

Create `dashboard/tests/docs-ai-search-evals.test.mjs` with Node tests that load the docs AI TypeScript module through a lightweight esbuild bundle, then assert:

- `how to connect tiktok to unipost with API?` answers with `POST /v1/connect/sessions`, sources the Connect Sessions guide, and does not source TikTok followers first.
- `How do I get TikTok followers?` answers with `GET /v1/accounts/{account_id}/metrics`, `user.info.stats`, and `data.follower_count`.
- `Does video.list give followers?` answers no and points followers to account metrics.
- `POST /v1/connect/sessions` ranks the API Reference create-session page first.
- An unsupported question returns confidence `none` and no answer sources.

- [ ] **Step 2: Add a script**

Add `test:docs-ai` to `dashboard/package.json`:

```json
"test:docs-ai": "node --test tests/docs-ai-search-evals.test.mjs tests/docs-ai-search-implementation-source.test.mjs tests/docs-analytics-guides-source.test.mjs"
```

- [ ] **Step 3: Verify red**

Run:

```bash
cd dashboard && npm run test:docs-ai
```

Expected: FAIL because the current analytics-only index routes TikTok connect questions to TikTok followers and lacks connect-session chunks.

### Task 2: Build the Content-Level Registry

**Files:**
- Modify: `dashboard/src/lib/docs-ai-search-index.ts`

- [ ] **Step 1: Extend the chunk schema**

Add `intent_tags` and broaden `product_area` to cover `connect`, `credentials`, `auth`, `posting`, `analytics`, `accounts`, `platforms`, `resources`, and `publishing`.

- [ ] **Step 2: Add curated chunks for PRD-critical docs**

Add chunks for Connect Sessions guide sections, create/get Connect Session API reference pages, TikTok platform guide, TikTok Platform Credentials guide, API auth, profiles, accounts, publishing, and the existing Analytics Guides/API Reference chunks. Every chunk must have `title`, `path`, `section_id`, `primary_nav`, `section_title`, `content`, `product_area`, `tags`, `intent_tags`, `endpoint_aliases`, `platforms`, and `last_indexed_at`.

- [ ] **Step 3: Keep analytics capability grounding**

Keep the `PLATFORM_METRICS` summary chunk for analytics capability claims.

### Task 3: Replace Naive Matching With Intent-Aware Retrieval

**Files:**
- Modify: `dashboard/src/lib/docs-ai-search-index.ts`

- [ ] **Step 1: Add intent detection**

Classify queries into task intent (`connect`, `analytics`, `posting`, `credentials`, `auth`, `reference`, or `unknown`) using verbs, endpoint patterns, platform names, and known aliases.

- [ ] **Step 2: Score exact endpoint and phrase matches**

Normalize endpoint aliases, prefer exact endpoint matches, and avoid substring-only matches that let generic words such as `api` or `tiktok` dominate.

- [ ] **Step 3: Enforce source coverage**

If the top source does not cover the detected product intent, return confidence `none` with related docs instead of writing a “closest documented path” answer.

- [ ] **Step 4: Preserve guide-first behavior**

For task-shaped queries, rank guides first and attach API Reference as supporting sources. For exact endpoint queries, rank API Reference first.

### Task 4: Add Deterministic Grounded Answers

**Files:**
- Modify: `dashboard/src/lib/docs-ai-search-index.ts`

- [ ] **Step 1: Add connect-session answer templates**

Add deterministic answers for customer-owned account connection questions, especially TikTok connect questions, with `POST /v1/connect/sessions`, required request fields, `data.url`, webhook completion, and polling fallback.

- [ ] **Step 2: Keep analytics answer templates**

Keep and broaden the TikTok followers, account metrics fields, export, and `video.list` clarification answers.

- [ ] **Step 3: Remove misleading generic answer**

Delete the fallback that says “The closest documented path is ...”. Generic fallback should state that source coverage is insufficient and list related docs only.

### Task 5: Strengthen Feedback and Operations Signals

**Files:**
- Modify: `dashboard/src/app/api/docs/answer/route.ts`
- Modify: `dashboard/src/app/api/docs/feedback/route.ts`
- Modify: `dashboard/src/app/docs/_components/docs-shell.tsx`

- [ ] **Step 1: Log answer telemetry**

Log `docs_ai_answer` with query length, confidence, generated_by, source ids, related ids, and coverage reason.

- [ ] **Step 2: Expand feedback payload**

Include answer confidence, generated_by, sources, related docs, and current docs path in feedback events so missing-doc reports can be reviewed from logs.

- [ ] **Step 3: Keep UI shape stable**

Only adjust copy/example questions if needed; keep Ask and Classic search modes intact.

### Task 6: Remove Public TikTok Flag Leakage From Docs Sources

**Files:**
- Modify: `dashboard/src/app/docs/api/analytics/tiktok/page.tsx`
- Modify: `dashboard/src/app/docs/api/analytics/tiktok/profile/page.tsx`
- Modify: `dashboard/src/app/docs/api/analytics/tiktok/account-metrics/page.tsx`
- Modify: `dashboard/src/app/docs/api/analytics/tiktok/videos/page.tsx`
- Modify: `dashboard/src/app/docs/platforms/[platform]/_data.tsx`
- Modify: `dashboard/tests/tiktok-analytics-docs-source.test.mjs`

- [ ] **Step 1: Replace flag references with public-ready wording**

State that TikTok approved `user.info.profile`, `user.info.stats`, and `video.list`; connected accounts may need reconnect to receive the approved scopes.

- [ ] **Step 2: Update source tests**

Assert that public docs do not mention `tiktok.analytics_scopes` or `FEATURE_TIKTOK_ANALYTICS_SCOPES`.

### Task 7: Validate and Release

**Files:**
- No new source files beyond tasks above.

- [ ] **Step 1: Local validation on task branch**

Run:

```bash
cd dashboard && npm run test:docs-ai
cd dashboard && npm run build
cd dashboard && npm run test:regression:dashboard
```

- [ ] **Step 2: Merge to local dev and validate again**

Fetch `origin`, fast-forward local `dev`, merge `dev-docs-ai-search-full-prd`, and rerun required validation.

- [ ] **Step 3: Push `origin/dev` and verify dev**

Push local `dev`, monitor checks/deployments, then verify `https://dev.unipost.dev/api/docs/answer` for TikTok connect, TikTok followers, and an unsupported query.

- [ ] **Step 4: Promote to staging and production**

Create and merge `dev` -> `staging`, verify staging, then create and merge `staging` -> `main`, verify production health and the same Docs AI answer flows.
