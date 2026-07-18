# CI gates

UniPost uses two layers of CI so development stays fast while production remains protected.

## Pull request gate

`.github/workflows/ci.yml` runs on:

- pull requests targeting `dev`
- pull requests targeting `staging`
- pull requests targeting `main`
- pushes to `dev`
- pushes to `staging`
- pushes to `main`
- manual `workflow_dispatch`

The required checks should be:

- `API tests`
- `Dashboard build`
- `Preview Acceptance` for task pull requests targeting `dev`

This covers the minimum production-safety bar before merging:

- backend compile, Go tests, and Go statement coverage artifacts with `go test ./... -coverprofile=coverage.out`
- dashboard dependency install and production `next build`
- dashboard local smoke regression against the built app with Playwright

The current backend statement coverage baseline measured during setup is 9.7%. The required CI floor starts at 9.0% so the gate blocks major regressions without pretending the codebase is already well covered. Raise `API_COVERAGE_MIN` in `.github/workflows/ci.yml` as focused tests are added. The biggest current backend gaps are generated DB query wrappers, worker loops, billing/webhook integration edges, and platform adapters that depend on third-party APIs.

The local dashboard smoke in CI uses the public GitHub Actions variable `NEXT_PUBLIC_CLERK_DEVELOPMENT_PUBLISHABLE_KEY` so public pages can initialize Clerk with a valid non-production frontend key. It pairs that with a dummy `CLERK_SECRET_KEY` placeholder because `clerkMiddleware` requires the variable at server startup, but public-page smoke coverage does not authenticate or call Clerk server APIs. Authenticated dashboard regression still requires real Clerk test credentials and stays in the deployed regression workflow.

Pull request CI must use `NEXT_PUBLIC_CLERK_DEVELOPMENT_PUBLISHABLE_KEY` and local non-production origins. It must never build a pull request with the production API URL or Clerk Production key.

## Preview acceptance gate

Every normal `dev-*` task branch opens a Draft pull request to `dev`; a `hotfix-*` branch syncing an approved production fix back to `dev` uses the same gate. Railway creates an isolated PR Environment from the sanitized `preview-base`, and Vercel builds the exact pull request SHA against that ephemeral API. The required `Preview Acceptance` check verifies the API health, frontend identity manifest, browser-visible API CORS, and public frontend health before the pull request may merge.

Preview Acceptance requires the repository secrets `VERCEL_TOKEN`, `VERCEL_AUTOMATION_BYPASS_SECRET`, and `RAILWAY_API_TOKEN`, plus the repository variable `RAILWAY_PROJECT_ID`. Each run uploads its prebuilt Vercel output as a compressed archive and creates a unique Vercel alias for signed-in human inspection. Playwright targets the immutable Vercel deployment URL and sends the project-scoped `x-vercel-protection-bypass` header, which is Vercel's supported E2E access method; the Preview remains protected from ordinary unauthenticated visitors and concurrent or repeated runs cannot overwrite one another. Rotate the bypass in Vercel and GitHub together, and revoke the superseded Vercel bypass after the replacement run passes. `RAILWAY_API_TOKEN` must be a dedicated Railway workspace token so the workflow can resolve the ephemeral environment ID from Railway's GitHub Deployment record, trigger that environment's `preview-api` when Railway skips an unchanged monorepo path, and read its domain and exact deployed commit. Do not use a production project token or a personal account-wide token.

The Railway `preview-base` service `preview-api` must use the repository-wide watch path `**/*`. This intentionally rebuilds the isolated API for every PR head, including frontend-only and documentation-only commits, so Preview Acceptance can prove that both deployed surfaces correspond to the same exact SHA.

Any failed, errored, timed-out, cancelled, skipped, missing, unable-to-start, or SHA-mismatched required result is a hard stop. The failure report must include the environment, branch, SHA, workflow, job, suite, test case, exact message and relevant log excerpt, run URL, artifact URLs, and whether any deployment or promotion already occurred.

Dashboard lint is intentionally not a required check yet because the existing codebase currently has lint errors. Once those are cleaned up, add `npm run lint` to the dashboard job and make it required.

Before any agent creates a pull request, it should run the same local CI equivalent:

- `GOCACHE=/tmp/unipost-go-build go test ./...` from `api/`
- `npm run build` from `dashboard/`
- `npm run test:regression:dashboard` from `dashboard/` when Playwright browsers are installed or the change touches dashboard routing, docs, auth, onboarding, analytics, posting, account connection, or shared UI shell code

## Deployed regression monitor

`.github/workflows/regression-monitor.yml` and `.github/workflows/dashboard-regression.yml` are deployed-environment monitors. They are useful for catching broken production or dev deployments, but they should not be the only PR gate because they depend on external accounts, live secrets, and third-party services.

Configure these GitHub repository secrets and variables for deployed regression:

- `UNIPOST_REGRESSION_API_KEY`
- `REGRESSION_TEST_ACCOUNT_ID`
- `REGRESSION_ALERT_WEBHOOK_URL`
- `UNIPOST_API_BASE_URL`
- `DASHBOARD_BASE_URL`
- `DASHBOARD_TEST_EMAIL`
- `DASHBOARD_TEST_PASSWORD`
- `DASHBOARD_TEST_PROFILE_ID`

## Branch protection

Recommended GitHub branch protection:

- For `dev`, require PRs from `dev-*` branches and require `API tests`, `Dashboard build`, and `Preview Acceptance`.
- For `staging`, require promotion PRs from `dev` and require `API tests` plus `Dashboard build`.
- For `main`, require production PRs from `staging` and require `API tests` plus `Dashboard build`.
- Require branches to be up to date before merging when GitHub offers the option.
- Keep direct pushes to `dev`, `staging`, and `main` disabled. Emergency exceptions require the user's explicit authorization after the blocking evidence is reported.
