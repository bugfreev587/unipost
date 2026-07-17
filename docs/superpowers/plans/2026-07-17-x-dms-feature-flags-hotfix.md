# X DMs and X Credits Feature Flags Hotfix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two UniPost-managed production feature flags, default both off, so regular users cannot use unfinished X DMs or see/use X Credits accounting while Super Admin-owned workspaces retain both paths for testing.

**Architecture:** PostgreSQL is the source of truth for a small global feature registry. A shared backend evaluator combines the global value with the workspace owner's Super Admin status, and every sensitive API/worker path uses that evaluator. The Admin page edits only allowlisted flags through Super Admin-only endpoints. The dashboard consumes backend-evaluated workspace flags; a public read-only endpoint controls public pricing presentation. OAuth 2.0 remains unchanged, and automated private DM Activity subscriptions are disabled.

**Tech Stack:** Go 1.24, chi, pgx/PostgreSQL, Clerk authentication, Next.js/React/TypeScript, existing AdminShell and design tokens, Go tests, Next.js build, Playwright regression tests.

---

## Task 1: Persist and evaluate the two allowlisted flags

**Files:**
- Create: `api/internal/db/migrations/118_feature_flags.sql`
- Create: `api/internal/featureflags/featureflags.go`
- Create: `api/internal/featureflags/postgres.go`
- Test: `api/internal/featureflags/featureflags_test.go`
- Test: `api/internal/db/feature_flags_migration_test.go`

- [ ] Write failing tests proving:
  - only `x_dms_v1` and `x_credits_billing_v1` are registered;
  - both default to `false`;
  - an enabled global flag is available to a regular workspace;
  - a disabled global flag is still available when the workspace owner is a Super Admin;
  - a disabled global flag is unavailable to a regular workspace;
  - updates reject unknown keys;
  - the migration creates `feature_flags` and immutable `feature_flag_changes` rows with actor and timestamp.

- [ ] Run the focused tests and observe the expected failures:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/featureflags ./internal/db
```

- [ ] Add migration `118_feature_flags.sql`:
  - `feature_flags(key PRIMARY KEY, enabled, description, updated_by, updated_at)`;
  - `feature_flag_changes(id, flag_key, previous_enabled, enabled, changed_by, changed_at)`;
  - seed both allowlisted keys as `false` using `ON CONFLICT DO NOTHING`;
  - constrain keys to the two registered values.

- [ ] Implement:
  - constants `XDMSV1` and `XCreditsBillingV1`;
  - metadata registry for labels/descriptions/owner areas;
  - `Store.List`, `Store.Set`, and `Store.GlobalEnabled`;
  - `Evaluator.ForWorkspace(ctx, workspaceID, key)` that resolves the workspace owner and grants a disabled flag only when `auth.IsSuperAdmin` is true;
  - `Evaluator.Public(ctx, key)` that returns only the global value.

- [ ] Re-run the focused tests until green.

- [ ] Commit:

```bash
git add api/internal/db/migrations/118_feature_flags.sql api/internal/db/feature_flags_migration_test.go api/internal/featureflags
git commit -m "feat: add internal feature flag registry"
```

## Task 2: Expose safe admin, workspace, and public flag APIs

**Files:**
- Create: `api/internal/handler/feature_flags.go`
- Test: `api/internal/handler/feature_flags_test.go`
- Modify: `api/internal/handler/me.go`
- Modify: `api/internal/handler/me_features_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] Write failing handler tests proving:
  - `GET /v1/admin/feature-flags` returns the two flags and metadata;
  - `PATCH /v1/admin/feature-flags/{key}` validates a boolean body and unknown keys;
  - normal users cannot reach the admin routes through `RequireSuperAdmin`;
  - `/v1/me/features` returns backend-evaluated `x_dms_v1` and `x_credits_billing_v1`;
  - `/v1/public/features` returns only global booleans and no actor/audit metadata.

- [ ] Run and observe failures:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler
```

- [ ] Implement a `FeatureFlagsHandler`, wire the Postgres store/evaluator in `main.go`, and register:
  - authenticated `GET /v1/me/features`;
  - public `GET /v1/public/features`;
  - Super Admin-only `GET /v1/admin/feature-flags`;
  - Super Admin-only `PATCH /v1/admin/feature-flags/{key}`.

- [ ] Keep the existing `/v1/me/features` response contract (`flags` plus `plan_gates`) and change provider from `removed` to `unipost`.

- [ ] Re-run focused handler tests until green.

- [ ] Commit:

```bash
git add api/internal/handler api/cmd/api/main.go
git commit -m "feat: expose internal feature flag APIs"
```

## Task 3: Put X DMs behind the backend authority and disable private subscriptions

**Files:**
- Modify: `api/internal/xinbox/capabilities.go`
- Modify: `api/internal/xinbox/capabilities_test.go`
- Modify: `api/internal/handler/inbox.go`
- Modify: `api/internal/handler/inbox_x_outbound.go`
- Modify: `api/internal/handler/platforms.go`
- Modify: `api/internal/handler/inbox_test.go`
- Modify: `api/internal/handler/platforms_x_inbox_test.go`
- Modify: `api/internal/db/queries/inbox.sql`
- Regenerate/modify: `api/internal/db/inbox.sql.go`
- Modify: `api/internal/worker/inbox_sync.go`
- Modify: `api/internal/worker/inbox_sync_subscription_test.go`
- Modify: `api/internal/worker/x_inbox_delivery.go`
- Modify: `api/internal/worker/x_inbox_delivery_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] Write failing tests proving that when `x_dms_v1` is off for an ordinary workspace:
  - X capabilities report DMs disabled and missing DM scopes alone do not set reconnect required;
  - list/count/unread/get endpoints cannot expose `x_dm`;
  - `source=x_dm`, manual X DM sync, and X DM send return a stable `FEATURE_NOT_AVAILABLE` response;
  - background/manual DM ingestion does not persist private messages;
  - comments/replies remain available;
  - the delivery reconciler never calls `EnsureDMSubscription`.

- [ ] Add companion tests proving a Super Admin-owned workspace can still use the OAuth 2.0 manual DM read/send path while the global flag is off.

- [ ] Run the focused tests and observe failures:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/xinbox ./internal/handler ./internal/worker ./internal/db
```

- [ ] Inject the shared evaluator into the platform, inbox, sync, and delivery components.

- [ ] Add query-level `x_dm` exclusion so pagination, unread counts, and item lookup cannot leak hidden DMs.

- [ ] Keep the existing OAuth 2.0 requested scopes unchanged. When the flag is off, capability/reconnect evaluation ignores DM-only missing scopes. When turned on, accounts that already granted DM scopes do not reconnect; accounts missing them reconnect once.

- [ ] Remove automatic calls to `EnsureDMSubscription` for all environments. Preserve comment Filtered Stream subscriptions and existing stored cleanup metadata; do not add OAuth 1.0a.

- [ ] Re-run focused tests until green.

- [ ] Commit:

```bash
git add api/internal/xinbox api/internal/handler api/internal/worker api/internal/db api/cmd/api/main.go
git commit -m "fix: gate unfinished X direct messages"
```

## Task 4: Gate customer X Credits accounting without weakening safety controls

**Files:**
- Create: `api/internal/xcredits/rollout.go`
- Test: `api/internal/xcredits/rollout_test.go`
- Modify: `api/internal/xcredits/service.go`
- Modify: `api/internal/xcredits/postgres.go`
- Modify: `api/internal/xcredits/inbound_service_test.go`
- Modify: `api/internal/handler/billing.go`
- Modify: `api/internal/handler/x_credits_test.go`
- Modify: `api/cmd/api/main.go`
- Regression test: existing X publish daily-cap tests under `api/internal/handler`

- [ ] Write failing tests proving that when `x_credits_billing_v1` is off for an ordinary workspace:
  - outbound managed-X `Reserve` returns `bypassed` and creates no monthly usage event;
  - finalize/reverse tolerate a bypassed empty event;
  - inbound processing still performs atomic message mutation and enforces the existing internal daily cost cap;
  - inbound safety-only admission does not increment `x_usage_periods` or `x_usage_events` and cannot fail due to monthly allowance;
  - `/v1/billing/x-credits` returns `FEATURE_NOT_AVAILABLE`;
  - the independent 20 X publishes/account/UTC day limit remains enforced.

- [ ] Add companion tests proving a Super Admin-owned workspace follows the existing full accounting and monthly-limit path while the global flag is off.

- [ ] Run and observe failures:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/xcredits ./internal/handler
```

- [ ] Add a rollout-aware service facade used by all existing X usage call sites:
  - resolve the flag by workspace owner;
  - delegate unchanged when enabled;
  - bypass customer monthly reserve/exposure accounting when disabled;
  - route inbound work through a new atomic safety-only Postgres method that keeps receipt deduplication, daily safety counters/cap, notifications, and mutation atomicity while omitting monthly usage rows/events and monthly-limit suppression.

- [ ] Gate X Credits customer billing endpoints through the same evaluator. Do not alter the separate 20/day X publish limiter.

- [ ] Re-run focused tests until green.

- [ ] Commit:

```bash
git add api/internal/xcredits api/internal/handler/billing.go api/internal/handler/x_credits_test.go api/cmd/api/main.go
git commit -m "fix: gate X Credits customer accounting"
```

## Task 5: Add the Admin page and hide unavailable customer surfaces

**Files:**
- Create: `dashboard/src/app/admin/feature-flags/page.tsx`
- Create: `dashboard/src/app/admin/feature-flags/feature-flags.css`
- Modify: `dashboard/src/app/admin/_components/admin-ui.tsx`
- Modify: `dashboard/src/lib/api.ts`
- Create: `dashboard/src/lib/use-feature-flags.ts`
- Modify: `dashboard/src/app/(dashboard)/settings/billing/page.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/inbox/page.tsx`
- Modify: `dashboard/src/app/pricing/pricing-page-client.tsx`
- Test: add focused source/contract tests alongside existing dashboard tests

- [ ] Write failing frontend contract tests proving:
  - Feature Flags appears directly below Object Storage;
  - the page lists exactly X DMs and X Credits billing with clear ON/OFF meaning;
  - toggling requires confirmation, persists through the admin PATCH endpoint, reports errors, and refreshes server state;
  - the page uses `AdminShell` with `requireSuperAdmin`;
  - ordinary workspace UI hides X DM controls/items when off;
  - ordinary workspace Billing and public Pricing hide X Credits sections when off;
  - Super Admin workspace flags returned by `/v1/me/features` keep internal test UI available.

- [ ] Run the focused tests and observe failures using the existing dashboard test runner identified for these files.

- [ ] Implement the page using the existing AdminShell, existing design tokens, Lucide icons already installed in the project, a compact status badge, native switch semantics, explicit confirmation, loading state, and inline error state.

- [ ] Add typed API functions and a shared authenticated hook for `/v1/me/features`. Fetch `/v1/public/features` for public pricing display.

- [ ] Gate the relevant customer surfaces without relying on the frontend for security; the backend remains authoritative.

- [ ] Run:

```bash
cd dashboard
npm run build
```

- [ ] Commit:

```bash
git add dashboard/src
git commit -m "feat: add admin feature flag controls"
```

## Task 6: Update operational and product documentation

**Files:**
- Modify: `docs/feature-flags-unleash.md` (rename content/title to internal feature flags; rename file if references permit)
- Modify: `docs/prd-x-credits-dms-comments.md`
- Modify: `docs/x-inbox-operations.md`
- Modify: relevant Dashboard X DM/X Credits API Reference and Guidance pages

- [ ] Document for each flag:
  - key and owner area;
  - default OFF;
  - ON/OFF semantics;
  - Super Admin workspace override;
  - rollback action (turn OFF in `/admin/feature-flags`);
  - X OAuth 2.0 scope/reconnect behavior;
  - X private Activity subscription dependency and current disabled status;
  - X Credits OFF still preserves the independent 20/day publish cap and internal inbound cost cap.

- [ ] Link X DM API Reference ↔ Guidance and X Credits API Reference ↔ Guidance in both directions.

- [ ] State clearly that no top-up/auto-top-up or OAuth 1.0a work is included in this hotfix.

- [ ] Verify docs links through the dashboard build.

- [ ] Commit:

```bash
git add docs dashboard/src/app/docs dashboard/src/lib/docs-ai-search-index.ts
git commit -m "docs: document X rollout controls"
```

## Task 7: Full local verification

- [ ] Run backend CI-equivalent checks:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
```

- [ ] Run dashboard CI-equivalent checks:

```bash
cd dashboard
npm run build
npm run test:regression:dashboard
```

- [ ] Inspect the diff and status; verify no unrelated files or generated artifacts are included.

- [ ] Perform an implementation review against every item in the approved design spec.

## Task 8: Hotfix promotion and live acceptance

- [ ] Fetch origin, update local `staging`, merge `hotfix-x-dms-feature-flags`, rerun backend and dashboard checks, and push `staging` to `origin/staging`.

- [ ] Monitor every triggered GitHub, Vercel, and Railway check until terminal success.

- [ ] Verify on the real staging domains:
  - Admin navigation and Feature Flags page;
  - both flags default OFF;
  - Super Admin can toggle and regular-user behavior changes;
  - X DM OFF does not prompt reconnect for DM-only scopes and cannot list/sync/send DMs;
  - comments and X publishing still work;
  - X Credits OFF hides customer surfaces and does not decrement monthly balance;
  - 20/day and inbound safety caps remain active.

- [ ] Create and merge the required `staging` → `main` production PR after checks pass.

- [ ] Monitor production checks/deployments and verify the same critical flows at `app.unipost.dev` and `api.unipost.dev`.

- [ ] Sync the exact hotfix back to local `dev`, rerun validation, push `origin/dev`, monitor the dev deployments, and verify on `dev-app.unipost.dev` and `dev-api.unipost.dev`.

- [ ] Do not report completion until staging, production, and development acceptance all pass.
