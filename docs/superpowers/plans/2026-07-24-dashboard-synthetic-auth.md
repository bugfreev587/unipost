# Dashboard Synthetic Authentication Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the skipped email/password Dashboard regression with a required, ticket-authenticated disposable Clerk user that leaves no Clerk or UniPost records behind.

**Architecture:** A reusable JavaScript helper owns environment validation, passwordless Clerk user creation, Clerk's email-to-ticket Playwright sign-in, explicit UniPost bootstrap, and cleanup. The existing `DELETE /v1/me` handler becomes synchronous for both Clerk and local database deletion, allowing Preview environments to clean up without Clerk webhooks. Public/local, deployed authenticated, and Preview Playwright suites remain separate so every selected test is required and missing secrets fail instead of skip.

**Tech Stack:** Go 1.24, chi/sqlc/pgx, Next.js 16, Node.js test runner, Playwright 1.60, Clerk Testing 2.2.10, GitHub Actions.

---

### Task 1: Make account deletion synchronously remove UniPost data

**Files:**
- Create: `api/internal/handler/me_delete_test.go`
- Modify: `api/internal/handler/me.go:26-44,527-560`

- [ ] **Step 1: Write the failing handler tests**

Add tests that construct `MeHandler` with an injected Clerk deletion function and a recording `db.DBTX`. The success test must assert this exact order and outcome:

```go
calls := []string{}
h.deleteClerkUser = func(_ context.Context, userID string) error {
    calls = append(calls, "clerk:"+userID)
    return nil
}

h.Delete(rec, authenticatedDeleteRequest("user_synthetic"))

if rec.Code != http.StatusNoContent {
    t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
}
if diff := cmp.Diff([]string{"clerk:user_synthetic", "database:user_synthetic"}, calls); diff != "" {
    t.Fatalf("delete order mismatch (-want +got):\n%s", diff)
}
```

Add a database-failure test that expects `500`, proves Clerk deletion ran first, and proves no `204` is returned when local cleanup is unconfirmed.

- [ ] **Step 2: Run the focused tests and verify RED**

Run from `api/`:

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestMeDelete' -count=1
```

Expected: compile failure because `MeHandler` has no injectable Clerk deleter and `Delete` does not synchronously call `DeleteUser`.

- [ ] **Step 3: Add the minimal synchronous implementation**

Add a `deleteClerkUser func(context.Context, string) error` field with this production default:

```go
func deleteClerkUser(ctx context.Context, userID string) error {
    clerk.SetKey(os.Getenv("CLERK_SECRET_KEY"))
    _, err := clerkuser.Delete(ctx, userID)
    return err
}
```

Initialize it in `NewMeHandler`. In `Delete`, call the injected function, then call `h.queries.DeleteUser(r.Context(), userID)`. Return `500` on either failure and return `204` only after both calls succeed. Update the comment to describe the webhook as an idempotent fallback.

- [ ] **Step 4: Run focused and package tests and verify GREEN**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestMeDelete' -count=1
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -count=1
```

Expected: all selected tests pass with zero failures.

- [ ] **Step 5: Commit the API cleanup change**

```bash
git add api/internal/handler/me.go api/internal/handler/me_delete_test.go
git commit -m "fix: make account cleanup synchronous"
```

### Task 2: Build the synthetic Clerk authentication helper

**Files:**
- Create: `dashboard/tests/regression/support/synthetic-auth.mjs`
- Create: `dashboard/tests/dashboard-synthetic-auth.test.mjs`
- Modify: `dashboard/package.json`

- [ ] **Step 1: Write failing configuration and identity tests**

Test these public contracts with `node:test` and injected `fetch`/clock/random functions:

```js
assert.throws(
  () => loadSyntheticAuthConfig({
    DASHBOARD_BASE_URL: "https://app.unipost.dev",
    DASHBOARD_TEST_CLERK_SECRET_KEY: "sk_test_wrong",
  }),
  /production.*sk_live_/i,
);

const identity = await createSyntheticClerkUser(config, fakeFetch, {
  now: () => new Date("2026-07-24T12:00:00Z"),
  randomUUID: () => "00000000-0000-4000-8000-000000000001",
});
assert.match(identity.email, /^codex-dashboard-regression-/);
assert.equal(requestBody.skip_password_requirement, true);
assert.equal(Object.hasOwn(requestBody, "password"), false);
```

Also test Development/Preview `sk_test_` acceptance, production `sk_live_` acceptance, publishable-key discovery, exact tracked-user deletion, and redacted Clerk API errors.

- [ ] **Step 2: Run the Node tests and verify RED**

```bash
node --test tests/dashboard-synthetic-auth.test.mjs
```

Expected: failure because `tests/regression/support/synthetic-auth.mjs` does not exist.

- [ ] **Step 3: Implement the minimal helper contracts**

Implement these exact exports: `loadSyntheticAuthConfig(env = process.env)`, `loadClerkPublishableKey(config, fetchImpl = fetch)`, `createSyntheticClerkUser(config, fetchImpl = fetch, runtime)`, `deleteSyntheticClerkUser(config, userID, fetchImpl = fetch)`, `withClerkSecret(secretKey, operation)`, `readActiveClerkSession(page)`, `apiRequest(config, token, path, options)`, and `runWithCleanup(cleanup, acceptance)`.

`loadSyntheticAuthConfig` maps stable app hosts to their API hosts and requires `EXPECTED_PREVIEW_API_URL` for Vercel previews. Clerk requests must never interpolate the secret into errors. User creation must send `skip_password_requirement: true`, `skip_legal_checks: true`, no password, and reserved synthetic names.

- [ ] **Step 4: Add ticket sign-in and cleanup contract tests**

Inject `clerkSetup` and `clerk.signIn` and assert:

```js
await signInSyntheticUser(page, config, identity, { clerkSetup, signIn });
assert.deepEqual(signInOptions, { page, emailAddress: identity.email });
assert.equal(process.env.CLERK_SECRET_KEY, originalValue);
```

Test explicit `GET /v1/me/bootstrap`, profile resolution, authenticated `DELETE /v1/me`, fallback Clerk deletion before bootstrap, and aggregate preservation when acceptance and cleanup both fail.

- [ ] **Step 5: Implement sign-in, bootstrap, and cleanup**

Add the helper exports `signInSyntheticUser(page, config, identity, dependencies)`, `bootstrapSyntheticUser(page, config)`, and `cleanupSyntheticUser(page, config, identity, state, dependencies)`.

The sign-in implementation must use `clerk.signIn({ page, emailAddress })`; Clerk Testing 2.2.10 creates the short-lived ticket internally. Bootstrap must read `window.Clerk.session.getToken()`, call `/v1/me/bootstrap`, then read `/v1/profiles`. Cleanup must call `/v1/me` after bootstrap and otherwise delete only the exact tracked Clerk user.

- [ ] **Step 6: Add and run the focused script**

Add:

```json
"test:dashboard-synthetic-auth": "node --test tests/dashboard-synthetic-auth.test.mjs"
```

Run:

```bash
npm run test:dashboard-synthetic-auth
```

Expected: all synthetic-auth unit tests pass with zero failures.

- [ ] **Step 7: Commit the helper**

```bash
git add dashboard/package.json dashboard/tests/dashboard-synthetic-auth.test.mjs dashboard/tests/regression/support/synthetic-auth.mjs
git commit -m "test: add synthetic Clerk auth helper"
```

### Task 3: Replace the skipped authenticated smoke

**Files:**
- Create: `dashboard/tests/regression/authenticated-dashboard.spec.ts`
- Create: `dashboard/playwright.authenticated.config.ts`
- Modify: `dashboard/tests/regression/dashboard.spec.ts:1-7,222-258`
- Modify: `dashboard/playwright.regression.config.ts`
- Modify: `dashboard/playwright.preview.config.ts`
- Modify: `dashboard/package.json`

- [ ] **Step 1: Add a failing source contract**

Extend `dashboard/tests/dashboard-synthetic-auth.test.mjs` to assert:

```js
assert.equal(dashboardSource.includes("DASHBOARD_TEST_EMAIL"), false);
assert.equal(dashboardSource.includes("DASHBOARD_TEST_PASSWORD"), false);
assert.equal(dashboardSource.includes("test.skip"), false);
assert.match(authenticatedSource, /runWithCleanup/);
assert.match(authenticatedSource, /bootstrapSyntheticUser/);
```

Assert the public regression config excludes `authenticated-dashboard.spec.ts`, while the authenticated and Preview configs select it explicitly.

- [ ] **Step 2: Run the focused test and verify RED**

```bash
npm run test:dashboard-synthetic-auth
```

Expected: failure because the old credential-based skipped smoke remains and the new authenticated spec/config do not exist.

- [ ] **Step 3: Move the authenticated smoke to its required suite**

Remove `DASHBOARD_TEST_EMAIL`, `DASHBOARD_TEST_PASSWORD`, `signIn`, and the authenticated describe block from `dashboard.spec.ts`. Create `authenticated-dashboard.spec.ts` that:

1. Loads config at module startup so a selected suite fails on a missing key.
2. Creates one disposable identity.
3. Loads a public Clerk page and signs in with the email/ticket helper.
4. Explicitly bootstraps and resolves the synthetic default profile.
5. Runs the existing project, accounts, posts, analytics, settings, TikTok, and YouTube assertions.
6. Runs cleanup in `finally` while preserving an earlier acceptance error.

- [ ] **Step 4: Separate public and authenticated Playwright commands**

Make the public/local config ignore the authenticated spec. Add `playwright.authenticated.config.ts` with one Chromium worker, no retries locally, retained failure evidence, and the authenticated spec as its only match. Add:

```json
"test:regression:dashboard:authenticated": "playwright test --config=playwright.authenticated.config.ts"
```

Include the authenticated spec in `playwright.preview.config.ts` so exact-SHA Preview Acceptance exercises the real Clerk/API flow.

- [ ] **Step 5: Run source/unit verification and verify GREEN**

```bash
npm run test:dashboard-synthetic-auth
```

Expected: all synthetic-auth source and helper tests pass.

- [ ] **Step 6: Commit the Playwright split**

```bash
git add dashboard/package.json dashboard/playwright.authenticated.config.ts dashboard/playwright.preview.config.ts dashboard/playwright.regression.config.ts dashboard/tests/dashboard-synthetic-auth.test.mjs dashboard/tests/regression/authenticated-dashboard.spec.ts dashboard/tests/regression/dashboard.spec.ts
git commit -m "test: require ticket-authenticated dashboard smoke"
```

### Task 4: Wire environment-specific Clerk secrets into GitHub Actions

**Files:**
- Modify: `.github/workflows/dashboard-regression.yml`
- Modify: `.github/workflows/preview-acceptance.yml`
- Modify: `dashboard/tests/dashboard-synthetic-auth.test.mjs`
- Modify: `docs/ci-gates.md`

- [ ] **Step 1: Add failing workflow contract tests**

Assert:

```js
assert.doesNotMatch(deployedWorkflow, /DASHBOARD_TEST_(EMAIL|PASSWORD)/);
assert.match(deployedWorkflow, /DASHBOARD_TEST_CLERK_PRODUCTION_SECRET_KEY/);
assert.match(deployedWorkflow, /test:regression:dashboard:authenticated/);
assert.match(previewWorkflow, /DASHBOARD_TEST_CLERK_DEVELOPMENT_SECRET_KEY/);
assert.match(previewWorkflow, /DASHBOARD_TEST_CLERK_SECRET_KEY/);
```

Assert the documentation names both environment-specific repository secrets and says missing secrets fail the selected authenticated suite.

- [ ] **Step 2: Run the contract test and verify RED**

```bash
npm run test:dashboard-synthetic-auth
```

Expected: workflow/documentation assertions fail against the old email/password configuration.

- [ ] **Step 3: Update deployed and Preview workflows**

In the scheduled deployed workflow, pass:

```yaml
DASHBOARD_TEST_CLERK_SECRET_KEY: ${{ secrets.DASHBOARD_TEST_CLERK_PRODUCTION_SECRET_KEY }}
```

and run the public and authenticated scripts as separate required steps. In Preview Acceptance, pass:

```yaml
DASHBOARD_TEST_CLERK_SECRET_KEY: ${{ secrets.DASHBOARD_TEST_CLERK_DEVELOPMENT_SECRET_KEY }}
EXPECTED_PREVIEW_API_URL: ${{ steps.railway.outputs.api_url }}
```

to the Preview Playwright step. Never print either secret.

- [ ] **Step 4: Update CI documentation**

Replace the email/password secret list with:

- `DASHBOARD_TEST_CLERK_DEVELOPMENT_SECRET_KEY` (`sk_test_`)
- `DASHBOARD_TEST_CLERK_PRODUCTION_SECRET_KEY` (`sk_live_`)

Document passwordless disposable users, ticket sign-in, explicit bootstrap, synchronous `/v1/me` cleanup, and the separation between public/local and authenticated deployed suites.

- [ ] **Step 5: Run workflow contracts and YAML parsing**

```bash
npm run test:dashboard-synthetic-auth
ruby -e 'require "yaml"; YAML.parse_file("../.github/workflows/dashboard-regression.yml"); YAML.parse_file("../.github/workflows/preview-acceptance.yml"); puts "workflow yaml valid"'
```

Expected: unit/contract tests pass and both workflows parse.

- [ ] **Step 6: Commit workflow wiring**

```bash
git add .github/workflows/dashboard-regression.yml .github/workflows/preview-acceptance.yml dashboard/tests/dashboard-synthetic-auth.test.mjs docs/ci-gates.md
git commit -m "ci: run dashboard smoke with Clerk tickets"
```

### Task 5: Configure Clerk and GitHub without exposing secrets

**Files:**
- No repository files.

- [ ] **Step 1: Read existing secret names without reading values**

```bash
gh secret list
```

Expected: determine whether the two new secret names already exist; GitHub never returns values.

- [ ] **Step 2: Retrieve the Development and Production Clerk secret keys through the authorized Chrome session**

Use the signed-in Clerk Dashboard UI for the UniPost application. Select Development before copying the `sk_test_` key and Production before copying the `sk_live_` key. Keep values out of terminal output, commentary, screenshots, and artifacts.

- [ ] **Step 3: Store both GitHub Actions secrets securely**

Pipe each value directly to `gh secret set` without command-line interpolation or echoed output:

```bash
gh secret set DASHBOARD_TEST_CLERK_DEVELOPMENT_SECRET_KEY
gh secret set DASHBOARD_TEST_CLERK_PRODUCTION_SECRET_KEY
```

Expected: both commands report the secret name was set; values remain unreadable.

- [ ] **Step 4: Verify only secret presence**

```bash
gh secret list | rg 'DASHBOARD_TEST_CLERK_(DEVELOPMENT|PRODUCTION)_SECRET_KEY'
```

Expected: both names are listed with update timestamps.

### Task 6: Complete local verification and publish a Draft PR

**Files:**
- Modify only files needed to address verification failures attributable to this branch.

- [ ] **Step 1: Run complete API CI**

```bash
GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: all API tests pass with zero failures.

- [ ] **Step 2: Run Dashboard unit/contracts and build**

```bash
npm run test:dashboard-synthetic-auth
npm run build
```

Expected: tests and Next.js production build succeed.

- [ ] **Step 3: Run the public Dashboard regression**

```bash
npm run test:regression:dashboard
```

Expected: all selected public tests pass with zero skipped authenticated tests because that suite is intentionally separate.

- [ ] **Step 4: Run the authenticated regression against development**

Provide the Development Clerk secret through a protected temporary environment only, set `DASHBOARD_BASE_URL=https://dev-app.unipost.dev`, and run:

```bash
npm run test:regression:dashboard:authenticated
```

Expected: the authenticated smoke passes, creates a disposable Development Clerk user, and cleanup returns `204` with no skipped test.

- [ ] **Step 5: Review the exact diff and content audit**

```bash
git diff --check
git status --short
git log --oneline origin/dev..HEAD
git diff --name-status origin/dev...HEAD
```

Expected: only the approved spec, plan, API cleanup, Dashboard test support, workflows, and CI documentation are present.

- [ ] **Step 6: Request code review and address all Critical/Important findings**

Review the complete diff against the design, with special attention to secret leakage, environment mix-ups, customer-data selection, cleanup confirmation, and test selection. Re-run affected verification after any fix.

- [ ] **Step 7: Push the owned branch and open a Draft PR to `dev`**

```bash
git push -u origin dev-dashboard-test-auth
gh pr create --draft --base dev --head dev-dashboard-test-auth --title "test: authenticate Dashboard regression without passwords" --body "Replaces the skipped email/password smoke with disposable Clerk ticket authentication, synchronous UniPost cleanup, and exact-SHA Preview coverage. Test plan: API suite, Dashboard helper contracts, production build, public Playwright regression, authenticated Development regression, and Preview Acceptance."
```

Expected: one Draft PR from the owned task branch to `dev`.

### Task 7: Monitor exact-SHA Preview Acceptance and perform browser acceptance

**Files:**
- No repository files unless a verified failure requires a scoped fix.

- [ ] **Step 1: Record the exact PR head SHA and monitor all checks**

```bash
git rev-parse HEAD
gh pr checks --watch
```

Expected: API tests, Dashboard build, Railway PR Environment, Vercel Preview, public Preview checks, and authenticated synthetic smoke all succeed on the same SHA.

- [ ] **Step 2: Inspect artifacts and stop on any non-success**

Any failure, error, timeout, cancellation, skip, missing result, or SHA mismatch is a hard stop. Capture workflow/job/suite/case, exact message, relevant log excerpt, run URL, artifact URLs, and deployment state before fixing.

- [ ] **Step 3: Verify the exact Preview in Chrome**

Open the immutable Vercel Preview for the exact head SHA in the authorized Chrome profile. Confirm public health, authenticated Dashboard navigation, and that the browser is not sent through a password or Google OAuth UI.

- [ ] **Step 4: Audit commits and files before any merge decision**

```bash
git log --oneline origin/dev..HEAD
git diff --name-status origin/dev...HEAD
```

Expected: no unrelated, unidentified, unfinished, or unaccepted changes. Leave the PR Draft and do not merge to `dev` without explicit completion of all repository gates.
