# TikTok Scope Demo Builder Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the TikTok Scope-Driven App Review Demo Builder described in `docs/prd-tiktok-scope-driven-app-review-demo-builder.md`.

**Architecture:** Add a deterministic TikTok review-template registry in the API, expose scope-template and demo-plan endpoints, then make review kit/job/script creation consume the generated plan. The dashboard App Review page becomes a scope picker plus generated-plan preview and OAuth-reset preflight, while the local agent continues to execute a closed action script and reports richer segment events.

**Tech Stack:** Go API with chi/sqlc/pgx, PostgreSQL migrations, Next.js dashboard with TypeScript React, Node review-agent with Playwright and native capture.

---

## File Structure

- Create `api/internal/reviewtemplate/tiktok.go`: deterministic TikTok scope templates, evidence matrix, segment definitions, validation, and plan builder.
- Create `api/internal/reviewtemplate/tiktok_test.go`: unit tests for posting, analytics, mixed scopes, unsupported scopes, and OAuth prelude requirements.
- Modify `api/internal/handler/review.go`: add template/demo-plan responses, support `content_posting`, `analytics`, and `mixed` kit use cases, store generated plan metadata in `brand_snapshot`, include plan metadata in kit/job/session responses, build scripts from the plan.
- Modify `api/cmd/api/main.go`: register `GET /v1/review/tiktok/scope-templates` and `POST /v1/review/tiktok/demo-plan` behind `app_review.autopilot_v1`.
- Modify `dashboard/src/lib/api.ts`: add review template/demo-plan types and client helpers, broaden `createReviewKit` use case typing.
- Modify `dashboard/src/app/(dashboard)/projects/[id]/accounts/app-review/page.tsx`: add scope picker, presets, generated-plan preview, OAuth reset preflight, and pass selected scopes/plan metadata into kit creation.
- Modify `api/internal/reviewscript/script.go`: build review-agent steps from plan segment keys while preserving the closed action enum.
- Modify `review-agent/src/runner.js`: add explicit `oauth_consent_seen` / `oauth_consent_skipped` reporting and keep manual-pause overlays out of final reviewer artifacts when possible.
- Add migration `api/internal/db/migrations/078_review_scope_demo_builder.sql` only if we decide to promote plan fields out of `brand_snapshot`; first slice should avoid this unless querying plan fields becomes necessary.

## Task 1: Backend TikTok Template Registry

**Files:**
- Create: `api/internal/reviewtemplate/tiktok.go`
- Create: `api/internal/reviewtemplate/tiktok_test.go`

- [ ] **Step 1: Write failing tests**

Create tests covering:

```go
func TestBuildTikTokDemoPlanContentPosting(t *testing.T) {
	input := reviewtemplate.TikTokDemoPlanInput{Scopes: []string{"user.info.basic", "video.upload", "video.publish"}}
	plan, err := reviewtemplate.BuildTikTokDemoPlan(input)
	if err != nil {
		t.Fatalf("BuildTikTokDemoPlan: %v", err)
	}
	if plan.UseCase != "content_posting" {
		t.Fatalf("use case = %q", plan.UseCase)
	}
	if !plan.OAuthPrelude.Required || len(plan.Segments) != 3 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	assertSegment(t, plan, "posting_part_1", []string{"user.info.basic", "video.upload"})
	assertSegment(t, plan, "posting_part_2", []string{"user.info.basic", "video.publish"})
	assertSegment(t, plan, "posting_part_3", []string{"video.publish"})
}

func TestBuildTikTokDemoPlanAnalytics(t *testing.T) {
	plan, err := reviewtemplate.BuildTikTokDemoPlan(reviewtemplate.TikTokDemoPlanInput{Scopes: []string{"user.info.profile", "user.info.stats"}})
	if err != nil {
		t.Fatalf("BuildTikTokDemoPlan: %v", err)
	}
	if plan.UseCase != "analytics" || len(plan.Segments) != 2 {
		t.Fatalf("unexpected analytics plan: %+v", plan)
	}
	assertSegment(t, plan, "analytics_part_1", []string{"user.info.profile", "user.info.stats"})
	assertSegment(t, plan, "analytics_part_2", []string{"user.info.profile", "user.info.stats"})
}

func TestBuildTikTokDemoPlanRejectsUnsupportedScope(t *testing.T) {
	_, err := reviewtemplate.BuildTikTokDemoPlan(reviewtemplate.TikTokDemoPlanInput{Scopes: []string{"comment.list"}})
	if err == nil || !strings.Contains(err.Error(), "unsupported TikTok scope") {
		t.Fatalf("expected unsupported scope error, got %v", err)
	}
}
```

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/reviewtemplate`

Expected: fail because package does not exist.

- [ ] **Step 2: Implement template registry**

Implement `BuildTikTokDemoPlan`, `ListTikTokScopeTemplates`, scope normalization, deterministic ordering, and segment generation. The registry must include:

- `user.info.basic`
- `video.upload`
- `video.publish`
- `user.info.profile`
- `user.info.stats`
- `video.list`

Each generated plan must include:

- `template_version`
- `platform`
- `use_case`
- `requested_scopes`
- `oauth_prelude`
- `recording`
- `segments`
- `scope_coverage`
- `warnings`

- [ ] **Step 3: Run tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/reviewtemplate`

Expected: pass.

- [ ] **Step 4: Commit**

Run:

```bash
git add api/internal/reviewtemplate
git commit -m "feat: add tiktok review scope templates"
```

## Task 2: API Demo Plan Endpoints

**Files:**
- Modify: `api/internal/handler/review.go`
- Modify: `api/internal/handler/review_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Add handler tests**

Add tests that call:

- `GET /v1/review/tiktok/scope-templates`
- `POST /v1/review/tiktok/demo-plan`
- `POST /v1/review/kits` with analytics scopes

The demo-plan test should assert:

```go
if env.Data.Platform != "tiktok" { t.Fatal(...) }
if !env.Data.OAuthPrelude.Required { t.Fatal(...) }
if len(env.Data.Segments) == 0 { t.Fatal(...) }
```

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestReviewTikTok.*Plan|TestReviewCreateKit'`

Expected: fail before handlers are implemented.

- [ ] **Step 2: Implement handlers**

Add:

- `GetTikTokScopeTemplates`
- `CreateTikTokDemoPlan`

Update `CreateKit` to:

- validate selected scopes through `reviewtemplate.BuildTikTokDemoPlan`
- allow `content_posting`, `analytics`, and `mixed`
- store the generated plan under `brand_snapshot.review_plan`
- store `scope_template_version`
- store `oauth_reset_required: true`
- persist `RequiredScopes` from the selected plan, not the old fixed posting scope set

- [ ] **Step 3: Register routes**

Inside `/v1/review` route:

```go
r.Get("/tiktok/scope-templates", reviewHandler.GetTikTokScopeTemplates)
r.Post("/tiktok/demo-plan", reviewHandler.CreateTikTokDemoPlan)
```

- [ ] **Step 4: Run tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler`

Expected: pass.

- [ ] **Step 5: Commit**

Run:

```bash
git add api/internal/handler/review.go api/internal/handler/review_test.go api/cmd/api/main.go
git commit -m "feat: expose tiktok review demo plans"
```

## Task 3: Dashboard Scope Picker and Plan Preview

**Files:**
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/accounts/app-review/page.tsx`

- [ ] **Step 1: Add API client types**

Add `TikTokScopeTemplate`, `TikTokDemoPlan`, `getTikTokReviewScopeTemplates`, and `createTikTokReviewDemoPlan`.

- [ ] **Step 2: Add scope state and presets**

Replace the fixed `REQUIRED_TIKTOK_SCOPES` with selected scope state. Presets:

- Content Posting API: `user.info.basic`, `video.upload`, `video.publish`
- Analytics Basic: `user.info.profile`, `user.info.stats`
- Analytics + Video List: `user.info.profile`, `user.info.stats`, `video.list`

- [ ] **Step 3: Add generated plan panel**

Show:

- selected scopes
- generated segments
- section titles
- scopes covered by each segment
- 1080p and `<50MB` constraints
- OAuth prelude required

- [ ] **Step 4: Add OAuth reset preflight**

Add a required checkbox:

```text
I removed existing TikTok app authorization from TikTok mobile settings for this test account.
```

Disable Create review kit until this is checked.

- [ ] **Step 5: Wire kit creation**

Pass selected scopes, generated plan metadata, and profile id to `createReviewKit`.

- [ ] **Step 6: Run dashboard validation**

Run: `cd dashboard && npm run build`

Expected: pass.

- [ ] **Step 7: Commit**

Run:

```bash
git add dashboard/src/lib/api.ts 'dashboard/src/app/(dashboard)/projects/[id]/accounts/app-review/page.tsx'
git commit -m "feat: add tiktok review scope picker"
```

## Task 4: Review Script From Generated Plan

**Files:**
- Modify: `api/internal/reviewscript/script.go`
- Modify: `api/internal/reviewscript/script_test.go`
- Modify: `api/internal/handler/review.go`

- [ ] **Step 1: Add script tests**

Assert content posting scripts include:

- OAuth prelude
- creator info
- upload/select video
- privacy
- disclosure
- compliance links
- publish result

Assert analytics scripts include:

- OAuth prelude
- analytics navigation
- profile/stats evidence
- `video.list` only when selected

- [ ] **Step 2: Implement plan-aware script builder**

Add `BuildTikTokScriptFromPlan` and use plan metadata from the kit when present. Fall back to the old posting script for older kits without plan metadata.

- [ ] **Step 3: Run API tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/reviewscript ./internal/handler`

Expected: pass.

- [ ] **Step 4: Commit**

Run:

```bash
git add api/internal/reviewscript api/internal/handler/review.go api/internal/handler/review_test.go
git commit -m "feat: build review scripts from demo plans"
```

## Task 5: Agent Event and Artifact Metadata

**Files:**
- Modify: `review-agent/src/runner.js`
- Modify: `review-agent/tests/runner-video.test.js`
- Modify: `review-agent/tests/script-contract.test.js`

- [ ] **Step 1: Add tests**

Cover:

- script metadata produces segment markers
- OAuth consent skipped failure gets reported with a reset message
- completion artifacts include segment key, duration, capture mode, address-bar flag, and scope coverage when provided by script

- [ ] **Step 2: Implement reporter events**

Emit:

- `oauth_consent_seen`
- `oauth_consent_skipped`
- `segment_started`
- `segment_completed`

- [ ] **Step 3: Run review-agent tests**

Run: `cd review-agent && npm test`

Expected: pass.

- [ ] **Step 4: Commit**

Run:

```bash
git add review-agent/src review-agent/tests
git commit -m "feat: enrich review agent segment evidence"
```

## Task 6: End-to-End Validation

**Files:**
- No new files unless test fixes are needed.

- [ ] **Step 1: API validation**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`

Expected: pass.

- [ ] **Step 2: Dashboard validation**

Run: `cd dashboard && npm run build`

Expected: pass.

- [ ] **Step 3: Review-agent validation**

Run: `cd review-agent && npm test`

Expected: pass.

- [ ] **Step 4: PRD traceability check**

Search for these terms and verify they exist in code or UI:

```bash
rg "oauth_reset|OAuth reset|1080p|50 MB|user.info.stats|video.list|posting_part_1|analytics_part_1" api dashboard review-agent
```

Expected: each PRD-critical concept is represented.

- [ ] **Step 5: Merge to dev only after validation**

Follow `AGENTS.md`: merge the task branch into local `dev`, rerun relevant checks, and push `origin/dev`.

