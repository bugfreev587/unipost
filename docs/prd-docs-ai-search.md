# PRD - Docs AI Search

**Status:** Draft
**Owner:** Product / Developer Experience
**Target:** Docs AI search planning
**Created:** 2026-06-18

---

## Problem

UniPost's docs currently do two jobs:

- API Reference documents exact endpoints, parameters, responses, and errors.
- Guides explain how to complete real workflows.

The API Reference is good for developers who already know which endpoint they need, but it is weak for task-shaped questions such as "How do I get TikTok followers?" A user may find TikTok scopes, native analytics pages, account metrics, and post analytics, but still miss that the UniPost answer is the unified `GET /v1/accounts/{account_id}/metrics` endpoint.

Docs AI Search should bridge that gap without turning API Reference pages into FAQ pages. API Reference remains the Reference layer: stable, endpoint-shaped, and easy to scan. Task answers should live in Guides and be discoverable by AI search.

## Goals

1. Add an AI search experience that can answer task-shaped developer questions from UniPost docs.
2. Make answers grounded in indexed docs pages with citation links for every factual claim.
3. Use a guide-first retrieval strategy for workflow questions, then link to API Reference for exact endpoint contracts.
4. Preserve API Reference as endpoint reference only, without FAQ blocks or broad task walkthroughs.
5. Build the knowledge base around small docs chunks with title, path, section id, primary nav, product area, endpoint aliases, and platform tags.
6. Support direct questions such as "How do I get TikTok followers?" by answering with the UniPost API path, prerequisites, response field, and related references.
7. Collect feedback signals so unclear questions and missing docs become a docs backlog.

## Non-goals

- Answer from private workspace data, logs, customer account data, or dashboard state.
- Execute API calls, create accounts, reconnect accounts, or mutate customer data from the search box.
- Replace Support or App Review workflows.
- Generate ungrounded answers from model knowledge when the docs do not contain the answer.
- Change public API shapes as part of the AI search launch.

## Personas

- **API integrator:** Wants to finish a concrete job quickly, often starting with a natural-language question.
- **Platform operator:** Needs to confirm scopes, reconnect requirements, and supported analytics fields.
- **Internal support / sales engineer:** Needs a fast way to link customers to the correct docs page and endpoint.

## UX Requirements

### Search entry

- Add a docs-level search box labeled around "Ask UniPost Docs" or equivalent.
- Keep classic keyword search available for users who want a page list.
- Let users ask natural-language questions without needing endpoint names.

### Answer shape

Each AI answer must include:

- A direct answer in one short paragraph.
- Numbered steps when the question is procedural.
- The primary UniPost API endpoint when relevant.
- Required scopes or account prerequisites when relevant.
- Response fields to read when relevant.
- Related links, with guide links before API Reference links for task questions.
- A "not found in docs" response when retrieval confidence is low.

### Source behavior

- Every answer must be grounded and include citations to docs pages.
- There must be no answer without source coverage.
- If sources disagree or the docs are incomplete, the answer must say what is missing and show the closest docs links.

## Content Model

The index should store docs chunks rather than entire pages. Each docs chunk should include:

- `title`
- `path`
- `section_id`
- `primary_nav`
- `section_title`
- `content`
- `product_area`
- `tags`
- `endpoint_aliases`
- `platforms`
- `last_indexed_at`

Guides should carry task tags such as `analytics`, `tiktok`, `followers`, `account metrics`, and `scopes`. API Reference chunks should carry endpoint aliases such as `GET /v1/accounts/{account_id}/metrics`, `GET /v1/accounts/:account_id/metrics`, and normalized SDK method names when available.

## Retrieval Strategy

Use guide-first retrieval for task queries:

1. Detect task intent from verbs and user phrases such as "how do I", "get followers", "export analytics", "reconnect scopes", or "which API".
2. Rank matching Guides above API Reference for the first answer source.
3. Attach API Reference chunks as supporting sources for exact request and response contracts.
4. Prefer platform-specific guide chunks only when they explain how to use UniPost's unified API for that platform.
5. Avoid presenting native platform drilldowns as the first path when a unified UniPost API exists.

For exact endpoint queries, rank API Reference first and attach Guides as related context.

## Guardrails

- Do not answer without citations from indexed docs.
- Do not invent scopes, fields, endpoint paths, pricing, or availability.
- Do not claim a platform supports a metric unless a docs source says so.
- Do not expose feature flags, internal rollout keys, private comments, or branch names in public answers.
- Do not advise users to call provider-native APIs when UniPost offers a unified public API for the same task.
- Return a fallback answer when the docs are stale or ambiguous.

## Analytics Guides Dependency

Analytics Guides should ship before Docs AI Search so the model has task-shaped knowledge. The first guide set should cover:

- Which API gets TikTok followers.
- How to read normalized account metrics across platforms.
- How to get analytics for UniPost-published posts.
- How to export analytics rows.
- How to reconnect accounts when analytics scopes are missing.

This gives AI search a clean source of truth for common questions while keeping API Reference clean.

## Technical Approach

### Phase 1: Content foundation

- Add a `Guides` docs section outside API Reference.
- Create Analytics Guides for high-frequency user questions.
- Ensure existing keyword search indexes the new guide pages through docs navigation.

### Phase 2: Static docs index

- Add an extraction script that walks docs routes and/or structured source metadata.
- Emit chunked JSON with stable ids based on path and heading.
- Include endpoint aliases and platform tags.
- Validate that every indexed chunk maps to a public docs URL.

### Phase 3: AI answer API

- Add a server-side docs answer route.
- Retrieve candidate chunks using hybrid keyword and vector matching.
- Generate answers with strict source grounding.
- Return answer text, citations, related pages, and confidence state.

### Phase 4: Feedback and operations

- Track query, clicked result, answer helpfulness, fallback rate, and missing-doc reports.
- Add an internal review queue for low-confidence queries.
- Add regression prompts for core docs questions before release.

## Initial Example Questions

| Question | Expected primary source | Expected answer |
| --- | --- | --- |
| How do I get TikTok followers? | Analytics Guide: TikTok followers | Call `GET /v1/accounts/{account_id}/metrics`, ensure `user.info.stats`, read `data.follower_count`. |
| Which endpoint exports analytics? | Analytics Guide: Export analytics rows | Use `GET /v1/analytics/posts/export`; link the exact API Reference page. |
| Does `video.list` give followers? | Analytics Guide: TikTok followers | No. `video.list` is for public videos; followers come from `user.info.stats` through account metrics. |
| What fields are returned by account metrics? | API Reference: Get account metrics | Link the endpoint contract and list the normalized response fields. |

## Acceptance Criteria

- A user can ask "How do I get TikTok followers?" and receive the unified UniPost endpoint, required TikTok scope, response field, and related docs links.
- Answer generation refuses or falls back when retrieved docs do not support the answer.
- Every generated answer includes citations.
- Guide pages are preferred over API Reference for task questions.
- API Reference remains endpoint-shaped and does not gain broad FAQ/task walkthrough content.
- Analytics Guides exist before the AI search launch and are included in the docs index.
- Internal feedback can identify docs gaps from unanswered or low-confidence questions.

## Rollout

1. Ship Analytics Guides and index them in existing docs search.
2. Build the static docs chunk index and run offline answer evaluations.
3. Release AI search to internal users.
4. Expand to public docs once citation quality and fallback behavior pass acceptance.

## Open Questions

- Which model and embedding provider should be used for production AI search?
- Should search history be stored per workspace, anonymous session, or not stored initially?
- What retention window is acceptable for raw user questions?
- Should docs feedback create support tickets, Linear issues, or an internal dashboard queue?
