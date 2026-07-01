# Remove Feature Flags and Unleash Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove UniPost's Unleash-backed feature flag system, stop using flags to release product behavior, and carry the cleanup through the standard release flow.

**Architecture:** Convert shipped flag-controlled behavior into direct product behavior, keep plan gates as packaging controls, and delete the Unleash provider, flag registry, dashboard flag gate components, and flag documentation. Runtime configuration remains only for credentials and providers such as Loops, TikTok, Stripe, Clerk, and AI providers; it must not decide whether a product feature is released.

**Tech Stack:** Go API with chi/sqlc/pgx, Next.js dashboard, Clerk auth, Railway API deployments, Vercel dashboard deployments, GitHub Actions, Playwright dashboard regression.

---

## Current Flag Inventory

Unleash-backed flags currently registered in `api/internal/featureflags/flags.go`:

- `tiktok.analytics_scopes`: controls TikTok analytics OAuth scopes and TikTok analytics endpoints.
- `attribution.utm_signup_binding_v1`: controls landing UTM capture and signed-in user binding.
- `email.loops_integration_v1`: controls Loops contact sync and lifecycle events.
- `billing.free_plan_hard_post_quota`: controls hard blocking when Free workspaces exceed monthly quota.
- `app_review.autopilot_v1`: controls App Review Autopilot dashboard/API/review-session access.
- `app_review.ai_agent_v1`: controls AI-guided App Review agent commands and agent next-action API.
- `posts.calendar_view_v1`: controls Posts calendar view and `/posts/list` route split.

Related but not Unleash-backed:

- `inbox`: already removed from feature flags and enforced by plan gates (`plans.allow_inbox`). Keep this.
- `FEATURE_FACEBOOK_REELS`: still an env switch in `api/internal/platform/validate.go`. Treat this as a residual feature switch. Include it only if the user confirms this cleanup means all feature switches, not just Unleash-backed flags.
- `FEATURES_IN_DEV` in `dashboard/src/lib/features-in-dev.ts`: super-admin development visibility for unfinished dashboard surfaces. Keep this unless the user asks to remove internal development visibility.

## Required Product Decisions Before Code Changes

Do not start implementation until the user confirms these target outcomes:

1. App Review Autopilot target state:
   - Recommended path: remove the Unleash flag and make the dashboard/API available to authenticated workspaces through the existing Developer submenu.
   - Safer alternative: remove the Unleash flag but keep the dashboard entry admin-only through existing `is_admin`/super-admin checks.
   - Removal alternative: delete the App Review Autopilot product surface and its API routes.
2. AI-guided App Review agent target state:
   - Recommended path: remove the flag and let the existing AI provider configuration decide availability. If no provider is configured, keep returning `AI_NOT_CONFIGURED`.
   - Safer alternative: keep scripted agent commands only and remove AI-guided next-action usage.
3. `FEATURE_FACEBOOK_REELS` scope:
   - Recommended path for this release: leave it out of the Unleash cleanup and create a separate follow-up if all env feature switches must go away too.
   - Full cleanup path: remove the env switch and make Facebook Reels behavior a normal capability or remove the Reels path.

## File Structure

API files to modify or delete:

- Delete: `api/internal/featureflags/flags.go`
- Delete: `api/internal/featureflags/unleash.go`
- Delete: `api/internal/featureflags/flags_test.go`
- Modify: `api/go.mod`
- Modify: `api/go.sum`
- Modify: `api/.env.example`
- Modify: `api/cmd/api/main.go`
- Modify: `api/internal/connect/tiktok.go`
- Modify: `api/internal/connect/tiktok_test.go`
- Modify: `api/internal/platform/tiktok.go`
- Modify: `api/internal/platform/tiktok_test.go`
- Modify: `api/internal/handler/tiktok_analytics.go`
- Modify: `api/internal/handler/social_account_metrics.go`
- Modify: `api/internal/handler/landing_attribution.go`
- Modify: `api/internal/handler/me.go`
- Modify: `api/internal/handler/plan_gate.go`
- Modify: `api/internal/handler/review.go`
- Modify: `api/internal/handler/review_test.go`
- Modify: `api/internal/handler/connect_sessions_test.go`
- Modify: `api/internal/handler/social_posts_quota_test.go`
- Modify: `api/internal/quota/checker.go`
- Modify: `api/internal/quota/checker_test.go`
- Modify: `api/internal/loops/syncer.go`
- Modify: `api/internal/loops/syncer_test.go`

Dashboard files to modify or delete:

- Delete: `dashboard/src/lib/feature-flags.ts`
- Delete: `dashboard/src/components/feature-flag-gate.tsx`
- Modify: `dashboard/src/lib/use-feature-flags.ts`
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/components/dashboard/shell.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/analytics/platforms/platform-analytics-list.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/analytics/platforms/tiktok/page.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/posts/page.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/posts/list/page.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/accounts/app-review/page.tsx`
- Modify: `dashboard/tests/posts-calendar-routing-source.test.mjs`
- Modify: `dashboard/tests/tiktok-analytics-docs-source.test.mjs`
- Modify: `dashboard/tests/docs-analytics-guides-source.test.mjs`

Docs and operational files:

- Replace: `docs/feature-flags-unleash.md` with a short decommission note, or delete it if no other docs link to it.
- Modify docs that mention the old flag process only when they describe current product behavior, not historical PRDs.

---

### Task 0: Confirm Target Outcomes and Prepare the Standard Release Branch

**Files:**
- No source files.

- [ ] **Step 1: Ask for the required product decisions**

Ask the user this exact question before implementation:

```text
Before I remove the flag system, please confirm:
1. Should App Review Autopilot become generally available, admin-only, or be removed?
2. Should AI-guided App Review become config-controlled, scripted-only, or be removed?
3. Is FEATURE_FACEBOOK_REELS included in this cleanup, or should this release focus only on Unleash-backed flags?
```

Expected: user gives concrete choices for all three items.

- [ ] **Step 2: Start from latest `origin/dev`**

Run:

```bash
git status --short --branch
git fetch origin
git checkout -B dev-remove-feature-flags origin/dev
```

Expected: branch is `dev-remove-feature-flags` and tracks current `origin/dev`. If uncommitted user changes would be overwritten, stop and ask the user.

- [ ] **Step 3: Rename the Codex thread**

Rename the thread to:

```text
dev-remove-feature-flags
```

Expected: thread title matches the branch name exactly.

---

### Task 1: Update API Tests for Flagless Behavior

**Files:**
- Modify: `api/internal/connect/tiktok_test.go`
- Modify: `api/internal/platform/tiktok_test.go`
- Modify: `api/internal/quota/checker_test.go`
- Modify: `api/internal/handler/social_posts_quota_test.go`
- Modify: `api/internal/handler/review_test.go`
- Modify: `api/internal/loops/syncer_test.go`

- [ ] **Step 1: Change TikTok connect scope tests**

In `api/internal/connect/tiktok_test.go`, replace env-off/env-on expectations with direct expectations:

```go
func TestTikTokConnectScopesIncludeAnalyticsScopes(t *testing.T) {
	scopes := tiktokConnectScopes()
	for _, scope := range []string{"user.info.profile", "user.info.stats", "video.list"} {
		if !slices.Contains(scopes, scope) {
			t.Fatalf("expected TikTok connect scopes to include %s, got %v", scope, scopes)
		}
	}
}

func TestTikTokAppReviewSessionKeepsContentPostingOnlyScopes(t *testing.T) {
	scopes := tiktokConnectScopesForSession(SessionView{ExternalUserID: "app-review:workspace"})
	for _, scope := range []string{"user.info.profile", "user.info.stats", "video.list"} {
		if slices.Contains(scopes, scope) {
			t.Fatalf("app-review session should not include analytics scope %s, got %v", scope, scopes)
		}
	}
}
```

Add `slices` to imports if the file does not already import it:

```go
import "slices"
```

- [ ] **Step 2: Change TikTok platform OAuth tests**

In `api/internal/platform/tiktok_test.go`, remove `TIKTOK_ANALYTICS_SCOPES_ENABLED` setup and assert analytics scopes are always present:

```go
func TestTikTokOAuthScopesIncludeAnalyticsScopes(t *testing.T) {
	scopes := tiktokOAuthScopes()
	for _, scope := range []string{"user.info.profile", "user.info.stats", "video.list"} {
		if !slices.Contains(scopes, scope) {
			t.Fatalf("expected TikTok OAuth scopes to include %s, got %v", scope, scopes)
		}
	}
}
```

- [ ] **Step 3: Change quota tests**

In `api/internal/quota/checker_test.go`, remove `featureflags.SetProvider(...)` and all `FEATURE_BILLING_FREE_PLAN_HARD_POST_QUOTA` setup. Replace the env-off test with this behavior test:

```go
func TestFreePlanHardBlockGateAlwaysEnabledForFreePlan(t *testing.T) {
	checker := newTestChecker(t, "free", 100)
	gate := checker.FreePlanHardBlockGateForPeriod(context.Background(), "workspace_1", "2026-07")

	if !gate.enabled {
		t.Fatal("expected Free plan hard quota gate to be enabled without a feature flag")
	}
	if gate.planID != "free" {
		t.Fatalf("expected free plan id, got %q", gate.planID)
	}
}
```

Keep paid-plan soft-overage tests intact.

- [ ] **Step 4: Change social post quota handler tests**

In `api/internal/handler/social_posts_quota_test.go`, remove the `featureflags` import, remove provider setup, and delete `t.Setenv("FEATURE_BILLING_FREE_PLAN_HARD_POST_QUOTA", "true")`.

Expected: existing hard-block assertions still pass because the behavior is unconditional for Free plans.

- [ ] **Step 5: Change App Review tests according to the confirmed target**

If App Review Autopilot becomes generally available or admin-only, remove these lines from `api/internal/handler/review_test.go`:

```go
t.Setenv("FEATURE_APP_REVIEW_AUTOPILOT_V1", "true")
t.Setenv("FEATURE_APP_REVIEW_AI_AGENT_V1", "true")
featureflags.SetProvider(featureflags.EnvProvider{})
t.Cleanup(func() { featureflags.SetProvider(featureflags.EnvProvider{}) })
```

Also remove the `featureflags` import. For AI-guided behavior, keep tests that expect `AI_NOT_CONFIGURED` when no AI provider is configured.

- [ ] **Step 6: Change Loops syncer tests**

In `api/internal/loops/syncer_test.go`, remove tests that prove `email.loops_integration_v1` can disable Loops. Keep tests that prove Loops does nothing when the client is nil or disabled.

- [ ] **Step 7: Run targeted API tests and confirm failures before implementation**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/connect ./internal/platform ./internal/quota ./internal/handler ./internal/loops -count=1
```

Expected before implementation: tests fail to compile or fail assertions because code still imports `featureflags`.

---

### Task 2: Remove the API Feature Flag Engine

**Files:**
- Delete: `api/internal/featureflags/flags.go`
- Delete: `api/internal/featureflags/unleash.go`
- Delete: `api/internal/featureflags/flags_test.go`
- Modify: `api/cmd/api/main.go`
- Modify: `api/internal/handler/plan_gate.go`
- Modify: `api/go.mod`
- Modify: `api/go.sum`

- [ ] **Step 1: Remove provider startup from `api/cmd/api/main.go`**

Delete the block that initializes `featureflags.NewProviderFromEnv()`, calls `featureflags.SetProvider`, defers `featureflags.Close`, and logs the provider. Remove the `featureflags` import.

Expected replacement near startup:

```go
slog.Info("runtime environment detected", "env", runtimeenv.Current(), "production", runtimeenv.IsProduction())
```

- [ ] **Step 2: Remove App Review route middleware**

In `api/cmd/api/main.go`, change:

```go
r.Route("/v1/review", func(r chi.Router) {
	r.Use(handler.RequireFeatureFlag(featureflags.AppReviewAutopilotV1))
```

to:

```go
r.Route("/v1/review", func(r chi.Router) {
```

- [ ] **Step 3: Remove `RequireFeatureFlag` middleware**

In `api/internal/handler/plan_gate.go`, delete the `RequireFeatureFlag` function and remove unused imports:

```go
import (
	"net/http"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)
```

- [ ] **Step 4: Delete the featureflags package**

Run:

```bash
rm api/internal/featureflags/flags.go api/internal/featureflags/unleash.go api/internal/featureflags/flags_test.go
```

- [ ] **Step 5: Remove Unleash dependency**

Run:

```bash
cd api
go mod tidy
```

Expected: `github.com/Unleash/unleash-go-sdk/v6` is removed from `api/go.mod` and `api/go.sum`.

---

### Task 3: Convert Backend Flagged Behavior to Direct Behavior

**Files:**
- Modify: `api/internal/connect/tiktok.go`
- Modify: `api/internal/platform/tiktok.go`
- Modify: `api/internal/handler/tiktok_analytics.go`
- Modify: `api/internal/handler/social_account_metrics.go`
- Modify: `api/internal/handler/landing_attribution.go`
- Modify: `api/internal/quota/checker.go`
- Modify: `api/internal/loops/syncer.go`
- Modify: `api/internal/handler/review.go`

- [ ] **Step 1: Make TikTok analytics scopes normal scopes**

In `api/internal/connect/tiktok.go`, remove `context` and `featureflags` imports if they are only used for scopes. Replace `tiktokConnectScopesForSession` with:

```go
func tiktokConnectScopesForSession(session SessionView) []string {
	scopes := append([]string(nil), tiktokConnectBaseScopes...)
	if isTikTokAppReviewSession(session) {
		return scopes
	}
	return append(scopes, tiktokConnectAnalyticsScopes...)
}
```

In `api/internal/platform/tiktok.go`, replace `tiktokOAuthScopes` with:

```go
func tiktokOAuthScopes() []string {
	scopes := append([]string(nil), tiktokLegacyScopes...)
	return append(scopes, tiktokAnalyticsScopes...)
}
```

- [ ] **Step 2: Remove TikTok analytics feature-disabled checks**

In `api/internal/handler/tiktok_analytics.go`, delete:

```go
if !tiktokAnalyticsScopesEnabled(r) {
	writeError(w, http.StatusForbidden, "FEATURE_DISABLED", "TikTok analytics is not enabled in this environment.")
	return nil, nil, "", false
}
```

Delete the `tiktokAnalyticsScopesEnabled` helper if no other handler uses it.

In `api/internal/handler/social_account_metrics.go`, remove the TikTok-specific `FEATURE_DISABLED` branch. If a helper is still useful for another condition, replace it with direct account/platform validation only.

- [ ] **Step 3: Make UTM attribution capture and binding always active**

In `api/internal/handler/landing_attribution.go`, replace:

```go
utmEnabled := featureflags.Enabled(r.Context(), featureflags.AttributionUTMSignupBindingV1, featureflags.Target{
	SessionID: sessionID,
})

attribution := map[string]string{}
rawQuery := ""
sourceCode := h.resolveSource(body.Source, referer)
if utmEnabled {
	attribution = sanitizeLandingAttribution(body.Attribution)
	rawQuery = sanitizeLandingText(body.RawQuery, 1024)
	sourceCode = h.resolveSourceWithAttribution(body.Source, referer, attribution)
}
```

with:

```go
attribution := sanitizeLandingAttribution(body.Attribution)
rawQuery := sanitizeLandingText(body.RawQuery, 1024)
sourceCode := h.resolveSourceWithAttribution(body.Source, referer, attribution)
```

In `BindSessionToUser`, delete the early return that checks `AttributionUTMSignupBindingV1`.

- [ ] **Step 4: Make Free plan hard post quota always active**

In `api/internal/quota/checker.go`, replace:

```go
if !featureflags.Enabled(ctx, featureflags.FreePlanHardPostQuota, featureflags.Target{
	WorkspaceID: workspaceID,
}) {
	return gate
}
gate.enabled = true
```

with:

```go
gate.enabled = true
```

Remove the `featureflags` import.

- [ ] **Step 5: Make Loops controlled only by Loops client/configuration**

In `api/internal/loops/syncer.go`, replace the default `enabled` function with:

```go
if enabled == nil {
	enabled = func(context.Context, DashboardUser) bool {
		return true
	}
}
```

Remove the `featureflags` import. Keep existing `s.client == nil || !s.client.Enabled()` checks; those are provider configuration checks, not feature flags.

- [ ] **Step 6: Remove App Review feature-disabled checks**

In `api/internal/handler/review.go`, delete checks against `AppReviewAutopilotV1` in `authenticateReviewSession` and `authenticateReviewAgent`.

For `CreateJob`, set `aiGuided` according to the confirmed target. If the recommended config-controlled path is chosen, use:

```go
aiGuided := h.aiPlanner != nil
```

Keep this check in `NextAgentAction`:

```go
if h.aiPlanner == nil {
	writeError(w, http.StatusServiceUnavailable, "AI_NOT_CONFIGURED", "AI-guided review agent is not configured.")
	return
}
```

Do not return `FEATURE_DISABLED` for AI-guided review after this cleanup.

- [ ] **Step 7: Run targeted API tests**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/connect ./internal/platform ./internal/quota ./internal/handler ./internal/loops -count=1
```

Expected: targeted API tests pass.

---

### Task 4: Simplify `/v1/me/features` into `/v1/me/plan-gates`

**Files:**
- Modify: `api/internal/handler/me.go`
- Modify: `api/cmd/api/main.go`
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/lib/use-feature-flags.ts`
- Modify: `dashboard/src/components/dashboard/shell.tsx`

- [ ] **Step 1: Rename the backend response shape**

In `api/internal/handler/me.go`, replace `featureFlagsResponse` with:

```go
type planGatesResponse struct {
	PlanGates map[string]bool `json:"plan_gates"`
}
```

Replace `Features` with:

```go
func (h *MeHandler) PlanGates(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}

	workspaceID := ""
	if mem, err := h.queries.GetActiveMembership(r.Context(), userID); err == nil {
		workspaceID = mem.WorkspaceID
	} else if workspaces, wsErr := h.queries.ListWorkspacesByUser(r.Context(), userID); wsErr == nil && len(workspaces) > 0 {
		workspaceID = workspaces[0].ID
	}

	planGates := map[string]bool{"inbox": false}
	if workspaceID != "" {
		planGates["inbox"] = h.quotaChecker == nil || h.quotaChecker.PlanAllowsInbox(r.Context(), workspaceID)
	}

	writeSuccess(w, planGatesResponse{PlanGates: planGates})
}
```

Remove `featureflags` and `runtimeenv` imports from `me.go`.

- [ ] **Step 2: Add `/v1/me/plan-gates` and temporarily keep `/v1/me/features` compatible**

In `api/cmd/api/main.go`, replace:

```go
r.Get("/v1/me/features", meHandler.Features)
```

with:

```go
r.Get("/v1/me/plan-gates", meHandler.PlanGates)
r.Get("/v1/me/features", meHandler.PlanGates)
```

This keeps old deployed dashboard bundles working during the dev and staging rollouts.

- [ ] **Step 3: Update dashboard API types**

In `dashboard/src/lib/api.ts`, replace `FeatureFlagsResponse` and `getFeatureFlags` with:

```ts
export interface PlanGatesResponse {
  plan_gates: Record<string, boolean>;
}

export async function getPlanGates(token: string): Promise<ApiResponse<PlanGatesResponse>> {
  return request("/v1/me/plan-gates", token);
}
```

- [ ] **Step 4: Convert `use-feature-flags.ts` into `use-plan-gates.ts`**

Rename `dashboard/src/lib/use-feature-flags.ts` to `dashboard/src/lib/use-plan-gates.ts` and replace its contents with:

```ts
"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { getPlanGates } from "@/lib/api";

export function usePlanGates() {
  const { getToken } = useAuth();
  const [planGates, setPlanGates] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    (async () => {
      try {
        setLoading(true);
        const token = await getToken();
        if (!token) {
          if (!cancelled) setPlanGates({});
          return;
        }
        const res = await getPlanGates(token);
        if (!cancelled) setPlanGates(res.data.plan_gates || {});
      } catch {
        if (!cancelled) setPlanGates({});
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [getToken]);

  return { planGates, loading };
}
```

- [ ] **Step 5: Update dashboard shell**

In `dashboard/src/components/dashboard/shell.tsx`, remove `FEATURE_FLAG_KEYS` and `useFeatureFlags` imports. Import `usePlanGates`.

Change:

```ts
const { flags: backendFeatureFlags, planGates } = useFeatureFlags();
```

to:

```ts
const { planGates } = usePlanGates();
```

Remove `backendFlag` and `backendFlagsAny` from the `NavItem` and `NavSubItem` types and from `filterNavItems`. Keep admin visibility handling.

Change the App Review nav item to:

```ts
{ href: "/accounts/app-review", label: "App Review" },
```

Change:

```ts
const navItems = filterNavItems(backendFeatureFlags, isAdmin);
```

to:

```ts
const navItems = filterNavItems(isAdmin);
```

- [ ] **Step 6: Run API compile check**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -count=1
```

Expected: handler tests pass.

---

### Task 5: Remove Dashboard Feature Flag Gates

**Files:**
- Delete: `dashboard/src/lib/feature-flags.ts`
- Delete: `dashboard/src/components/feature-flag-gate.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/analytics/platforms/platform-analytics-list.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/analytics/platforms/tiktok/page.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/posts/page.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/posts/list/page.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/accounts/app-review/page.tsx`

- [ ] **Step 1: Make TikTok analytics card always visible**

In `platform-analytics-list.tsx`, remove `FEATURE_FLAG_KEYS`, `useFeatureFlags`, `loading`, and `tiktokEnabled`.

Replace:

```tsx
{!loading && tiktokEnabled ? (
  <Link href={`/projects/${profileId}/analytics/platforms/tiktok`}>
    ...
  </Link>
) : null}
```

with:

```tsx
<Link href={`/projects/${profileId}/analytics/platforms/tiktok`}>
  ...
</Link>
```

- [ ] **Step 2: Remove TikTok page flag gate**

In `dashboard/src/app/(dashboard)/projects/[id]/analytics/platforms/tiktok/page.tsx`, replace the component body with:

```tsx
import { TikTokAnalyticsView } from "@/components/analytics/tiktok-analytics-view";

export default async function TikTokPlatformAnalyticsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <TikTokAnalyticsView profileId={id} />;
}
```

- [ ] **Step 3: Make Posts calendar the default page**

In `dashboard/src/app/(dashboard)/projects/[id]/posts/page.tsx`, replace the file with:

```tsx
import { PostsCalendarView } from "@/components/posts/calendar/posts-calendar-view";

export default function PostsPage() {
  return <PostsCalendarView />;
}
```

- [ ] **Step 4: Keep legacy list route available**

In `dashboard/src/app/(dashboard)/projects/[id]/posts/list/page.tsx`, replace the file with:

```tsx
import { PostsLegacyListView } from "@/components/posts/list/posts-legacy-list-view";

export default function PostsListPage() {
  return <PostsLegacyListView showCalendarLink />;
}
```

- [ ] **Step 5: Remove App Review page flag gate**

In `dashboard/src/app/(dashboard)/projects/[id]/accounts/app-review/page.tsx`, remove `FeatureFlagGate` and `FEATURE_FLAG_KEYS` imports. Replace `AppReviewAutopilotPage` with:

```tsx
export default function AppReviewAutopilotPage() {
  return <AppReviewAutopilotContent />;
}
```

If the user confirmed admin-only App Review, wrap `AppReviewAutopilotContent` with the existing admin check pattern from `dashboard/src/components/dashboard/shell.tsx` instead of a feature flag.

- [ ] **Step 6: Delete unused flag files**

Run:

```bash
rm dashboard/src/lib/feature-flags.ts dashboard/src/components/feature-flag-gate.tsx
```

- [ ] **Step 7: Check no dashboard flag imports remain**

Run:

```bash
rg -n "FEATURE_FLAG_KEYS|FeatureFlagGate|useFeatureFlags|feature-flags" dashboard/src dashboard/tests
```

Expected: no matches except historical text in tests that will be changed in Task 6.

---

### Task 6: Update Dashboard Source Tests and Docs Checks

**Files:**
- Modify: `dashboard/tests/posts-calendar-routing-source.test.mjs`
- Modify: `dashboard/tests/tiktok-analytics-docs-source.test.mjs`
- Modify: `dashboard/tests/docs-analytics-guides-source.test.mjs`

- [ ] **Step 1: Update Posts calendar source test**

In `dashboard/tests/posts-calendar-routing-source.test.mjs`, replace assertions that expect `useFeatureFlags` and `FEATURE_FLAG_KEYS.postsCalendarViewV1` with:

```js
assert.match(postsPage, /PostsCalendarView/);
assert.doesNotMatch(postsPage, /useFeatureFlags|FEATURE_FLAG_KEYS|PostsLegacyListView/);
assert.match(postsListPage, /PostsLegacyListView/);
assert.doesNotMatch(postsListPage, /router\.replace|useFeatureFlags|FEATURE_FLAG_KEYS/);
```

- [ ] **Step 2: Update TikTok analytics docs tests**

In `dashboard/tests/tiktok-analytics-docs-source.test.mjs`, remove `docs/feature-flags-unleash.md` from the files under test if that doc is deleted or replaced by a decommission note. Keep public docs assertions that public pages do not mention internal rollout keys.

Expected public-doc assertion:

```js
assert.doesNotMatch(page, /tiktok\.analytics_scopes|FEATURE_TIKTOK_ANALYTICS_SCOPES|feature flag/i);
```

- [ ] **Step 3: Keep analytics guide flag leak tests**

In `dashboard/tests/docs-analytics-guides-source.test.mjs`, keep assertions that public docs do not mention internal flag names. No product docs should instruct customers to toggle feature flags after this cleanup.

- [ ] **Step 4: Run Node source tests**

Run:

```bash
cd dashboard
node --test tests/posts-calendar-routing-source.test.mjs tests/tiktok-analytics-docs-source.test.mjs tests/docs-analytics-guides-source.test.mjs
```

Expected: source tests pass.

---

### Task 7: Clean Environment Examples and Flag Documentation

**Files:**
- Modify: `api/.env.example`
- Replace or delete: `docs/feature-flags-unleash.md`

- [ ] **Step 1: Remove Unleash and env fallback variables from `api/.env.example`**

Delete this feature flag section:

```dotenv
# -- Feature flags -----------------------------------------------------
FEATURE_FLAGS_PROVIDER=env
FEATURE_TIKTOK_ANALYTICS_SCOPES=false
UNLEASH_URL=
UNLEASH_SERVER_TOKEN=
UNLEASH_APP_NAME=unipost-api
UNLEASH_ENVIRONMENT=development
FEATURE_EMAIL_LOOPS_INTEGRATION_V1=false
```

Also delete:

```dotenv
FEATURE_APP_REVIEW_AI_AGENT_V1=false
```

Update the Loops comment to:

```dotenv
# When set, Clerk user.created/user.updated webhooks can sync dashboard users
# to Loops contacts and emit signup/lifecycle events.
LOOPS_API_KEY=
```

- [ ] **Step 2: Replace `docs/feature-flags-unleash.md` with a decommission note**

If no docs link requires the old rollout guide, replace the file with:

```markdown
# Feature Flags and Unleash Decommission

UniPost no longer uses Unleash-backed feature flags for product rollout.

Current release controls:

- Product packaging is enforced through plan gates, such as `plans.allow_inbox`, `plans.allow_analytics`, and `plans.white_label`.
- Third-party integrations are controlled by credentials and provider configuration, such as `LOOPS_API_KEY`, TikTok OAuth credentials, and AI provider routes.
- New feature rollout follows the standard development, staging, and production release flow instead of remote flag toggles.

Operational cleanup after this code ships:

1. Remove `FEATURE_FLAGS_PROVIDER`, `UNLEASH_URL`, `UNLEASH_SERVER_TOKEN`, `UNLEASH_APP_NAME`, and `UNLEASH_ENVIRONMENT` from Railway environments.
2. Confirm `/v1/me/plan-gates` returns only plan gates.
3. Shut down the Unleash Railway service after development, staging, and production verification pass.
4. Remove DNS for `flags.unipost.dev` after the service is shut down.
```

- [ ] **Step 3: Scan docs for current Unleash instructions**

Run:

```bash
rg -n "Unleash|FEATURE_FLAGS_PROVIDER|UNLEASH_|feature flag|feature flags" docs api dashboard --glob '!docs/prd-*' --glob '!docs/superpowers/plans/*'
```

Expected: matches are either the decommission note, historical PRDs excluded from current product docs, or non-Unleash plan-gate language.

---

### Task 8: Full Local Validation on Task Branch

**Files:**
- No source files.

- [ ] **Step 1: Run API tests**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: all API tests pass and there are no imports of `api/internal/featureflags`.

- [ ] **Step 2: Run dashboard build**

Run:

```bash
cd dashboard
npm run build
```

Expected: build passes.

- [ ] **Step 3: Run dashboard regression if browsers are installed**

Run:

```bash
cd dashboard
npm run test:regression:dashboard
```

Expected: Playwright regression passes. If browsers are missing, install Chromium with `npx playwright install chromium` and rerun.

- [ ] **Step 4: Run final source scans**

Run:

```bash
rg -n "github.com/Unleash|FEATURE_FLAGS_PROVIDER|UNLEASH_|featureflags\\.|FeatureFlagGate|FEATURE_FLAG_KEYS|useFeatureFlags|/v1/me/features" api dashboard docs --glob '!docs/prd-*' --glob '!docs/superpowers/plans/*'
```

Expected: no active code references. `/v1/me/features` may remain only in `api/cmd/api/main.go` as a temporary compatibility alias during one release.

- [ ] **Step 5: Commit the task branch**

Run:

```bash
git status --short
git add api dashboard docs
git commit -m "chore: remove Unleash feature flags"
```

Expected: commit includes only the flag cleanup files and no unrelated untracked artifacts.

---

### Task 9: Merge to `dev`, Push, Monitor, and Verify Development

**Files:**
- No source files unless conflict fixes are needed.

- [ ] **Step 1: Update local `dev` and merge task branch**

Run:

```bash
git fetch origin
git checkout dev
git pull --ff-only origin dev
git merge --no-ff dev-remove-feature-flags
```

Expected: local `dev` contains the cleanup commit. If conflicts occur, resolve only cleanup-related files and rerun validation.

- [ ] **Step 2: Rerun required local validation on `dev`**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
cd ../dashboard
npm run build
npm run test:regression:dashboard
```

Expected: all required checks pass.

- [ ] **Step 3: Push `dev`**

Run:

```bash
git push origin dev
```

Expected: push succeeds and triggers development checks/deployments.

- [ ] **Step 4: Monitor remote checks and deployments**

Monitor GitHub Actions, Vercel `unipost-dev`, and Railway `dev` until all triggered checks and deployments complete.

Expected: no queued, pending, running, or failed checks remain.

- [ ] **Step 5: Self-accept in development**

Use development domains only:

```text
https://dev-api.unipost.dev
https://dev-app.unipost.dev
https://dev.unipost.dev
```

Verify:

- `GET https://dev-api.unipost.dev/v1/me/plan-gates` works for a signed-in dashboard session and returns `plan_gates`.
- Dashboard loads without calling `/v1/me/features` from current bundles.
- Posts page defaults to the calendar view.
- `/projects/:id/posts/list` still shows the legacy list.
- TikTok analytics card/page are visible without a flag gate.
- Inbox unread work still respects `plan_gates.inbox`.
- App Review behavior matches the confirmed target.
- Loops behavior is controlled by Loops configuration and does not depend on Unleash.

Expected: real development environment matches the confirmed target outcome.

---

### Task 10: Promote to Staging

**Files:**
- No source files unless staging verification exposes a defect.

- [ ] **Step 1: Create PR from `dev` to `staging`**

Run the local CI-equivalent checks first, then create the PR:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
cd ../dashboard
npm run build
npm run test:regression:dashboard
```

Expected: local checks pass before PR creation.

- [ ] **Step 2: Merge after checks pass**

Monitor PR checks. Merge only after checks pass.

Expected: `staging` receives the cleanup through a `dev` to `staging` promotion PR.

- [ ] **Step 3: Wait for staging deployments**

Monitor Vercel `unipost-staging`, Railway `staging`, and GitHub checks until complete.

Expected: all triggered staging checks and deployments succeed.

- [ ] **Step 4: Self-accept in staging**

Use staging domains only:

```text
https://staging-api.unipost.dev
https://staging-app.unipost.dev
https://staging.unipost.dev
```

Repeat the development acceptance checklist against staging.

Expected: staging matches the confirmed target outcome.

---

### Task 11: Promote to Production

**Files:**
- No source files unless production PR checks expose a defect.

- [ ] **Step 1: Create PR from `staging` to `main`**

Do not create a PR from `dev` to `main`.

Expected: production PR source is `staging`, target is `main`.

- [ ] **Step 2: Merge after checks pass**

Monitor PR checks. Merge only after checks pass.

Expected: `main` receives exactly the staging-verified cleanup.

- [ ] **Step 3: Wait for production deployments**

Monitor Vercel `unipost`, Railway `production`, and GitHub checks until complete.

Expected: all triggered production checks and deployments succeed.

- [ ] **Step 4: Self-accept in production**

Use production domains only:

```text
https://api.unipost.dev
https://app.unipost.dev
https://unipost.dev
```

Verify:

- API health is normal.
- Dashboard loads for the test account.
- Posts calendar and legacy list route work.
- TikTok analytics page is reachable.
- Inbox remains plan-gated.
- App Review behavior matches the confirmed target.
- No customer-facing page references Unleash or feature flags as a rollout mechanism.

Expected: production is healthy and matches the confirmed target outcome.

---

### Task 12: Decommission Unleash Infrastructure After Production Verification

**Files:**
- No repository files unless infra documentation is updated.

- [ ] **Step 1: Remove Railway environment variables**

After production verification passes, remove these variables from API environments where they exist:

```text
FEATURE_FLAGS_PROVIDER
UNLEASH_URL
UNLEASH_SERVER_TOKEN
UNLEASH_APP_NAME
UNLEASH_ENVIRONMENT
FEATURE_TIKTOK_ANALYTICS_SCOPES
FEATURE_ATTRIBUTION_UTM_SIGNUP_BINDING_V1
FEATURE_EMAIL_LOOPS_INTEGRATION_V1
FEATURE_BILLING_FREE_PLAN_HARD_POST_QUOTA
FEATURE_APP_REVIEW_AUTOPILOT_V1
FEATURE_APP_REVIEW_AI_AGENT_V1
FEATURE_POSTS_CALENDAR_VIEW_V1
TIKTOK_ANALYTICS_SCOPES_ENABLED
```

Expected: API remains healthy because code no longer reads these variables.

- [ ] **Step 2: Shut down Unleash service**

Stop the Railway Unleash service only after dev, staging, and production all pass self-acceptance.

Expected: no UniPost API or dashboard behavior changes after the service is stopped.

- [ ] **Step 3: Remove `flags.unipost.dev` DNS**

Remove the DNS record after the service is stopped and no monitoring points to it.

Expected: no active UniPost runtime depends on `flags.unipost.dev`.

- [ ] **Step 4: Watch post-decommission health**

For at least one normal deployment cycle, watch:

- API logs for missing env or provider initialization errors.
- Dashboard errors for `/v1/me/features` calls.
- TikTok analytics OAuth/connect errors.
- Loops delivery errors.
- Free plan quota blocking behavior.

Expected: no Unleash-related errors appear.

---

## Self-Review

- Spec coverage: the plan removes Unleash SDK/provider, backend flag registry, dashboard flag gates, env fallback examples, rollout docs, and includes the required standard release flow through dev, staging, and production.
- Product decision gap: App Review Autopilot, AI-guided App Review, and `FEATURE_FACEBOOK_REELS` require explicit user confirmation before implementation.
- Type consistency: frontend plan renames feature flag API usage to plan gates; backend keeps `/v1/me/features` as a temporary compatibility alias returning the same `plan_gates` response.
- Validation coverage: API full tests, dashboard build, dashboard regression, source scans, and deployed environment self-acceptance are included.
