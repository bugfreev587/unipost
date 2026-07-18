# Preview Environment Guardrails Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every UniPost task branch pass an isolated Railway-and-Vercel preview deployment and fail-closed regression gate before it can merge into `dev`.

**Architecture:** A Draft PR from a conversation-owned `dev-*` branch triggers existing local CI plus a new `Preview Acceptance` workflow. Railway creates a PR environment from a sanitized, zero-replica `preview-base`; the workflow resolves the API deployment for the exact PR SHA, builds the `unipost-dev` Vercel preview against that API, and runs browser acceptance. GitHub branch protection requires the checks, while `AGENTS.md` makes worktree ownership, preview acceptance, failure reporting, and promotion diff review non-negotiable.

**Tech Stack:** Git worktrees, GitHub Actions and branch protection, Node.js built-in tests, Railway PR Environments, Vercel CLI 50.26.1, Next.js 16, Playwright 1.60, Go/chi CORS.

---

## File map

- Modify `AGENTS.md`: highest-priority session isolation, preview-first development, hard failure, and promotion audit rules.
- Modify `.github/workflows/ci.yml`: run on `staging`, remove production URLs from PR CI, and validate preview contracts.
- Create `.github/workflows/preview-acceptance.yml`: orchestrate Railway resolution, Vercel build/deploy, browser regression, failure evidence, and cleanup-safe concurrency.
- Create `scripts/preview/railway-deployments.mjs`: pure selection logic plus GitHub deployment polling for the exact Railway PR API SHA.
- Create `scripts/preview/railway-deployments.test.mjs`: unit tests for success, SHA mismatch, missing URL, failed status, and timeout behavior.
- Create `scripts/preview/release-guardrails.test.mjs`: static contract tests for `AGENTS.md` and workflow fail-closed invariants.
- Create `scripts/preview/write-manifest.mjs`: write the non-secret preview manifest served only by the preview build.
- Create `scripts/preview/write-manifest.test.mjs`: validate URL/SHA normalization and reject production targets.
- Create `dashboard/playwright.preview.config.ts`: deployed preview-only Playwright configuration with zero retries.
- Create `dashboard/tests/regression/preview-environment.spec.ts`: verify preview identity, public frontend health, preview API health, and browser CORS.
- Modify `dashboard/package.json`: add `test:regression:preview`.
- Modify `api/cmd/api/cors_test.go`: prove environment-configured Vercel preview origins work.
- Modify `docs/ci-gates.md`: document new required gates, Preview environment, and failure-report format.

### Task 1: Lock the repository workflow contract

**Files:**
- Create: `scripts/preview/release-guardrails.test.mjs`
- Modify: `AGENTS.md`
- Modify: `.github/workflows/ci.yml`
- Modify: `docs/ci-gates.md`

- [ ] **Step 1: Write the failing guardrail contract test**

Create `scripts/preview/release-guardrails.test.mjs` with tests that read repository files and assert the required language and CI triggers:

```js
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";

const read = (path) => readFile(new URL(`../../${path}`, import.meta.url), "utf8");

test("AGENTS enforces exclusive branch and worktree ownership", async () => {
  const agents = await read("AGENTS.md");
  assert.match(agents, /one exclusive branch and one exclusive worktree/i);
  assert.match(agents, /must never use another conversation's branch or worktree/i);
  assert.match(agents, /verify the absolute worktree path and current branch/i);
});

test("AGENTS requires preview acceptance before dev", async () => {
  const agents = await read("AGENTS.md");
  assert.match(agents, /Draft pull request.*to `dev`/s);
  assert.match(agents, /Railway PR Environment/);
  assert.match(agents, /Vercel Preview/);
  assert.match(agents, /must not merge.*`dev`.*Preview Acceptance.*success/is);
});

test("AGENTS treats every non-success regression result as a hard stop", async () => {
  const agents = await read("AGENTS.md");
  for (const result of ["fails", "errors", "times out", "is cancelled", "is skipped", "cannot start"]) {
    assert.ok(agents.includes(result), `missing hard-stop result: ${result}`);
  }
  assert.match(agents, /exact failure message and relevant log excerpt/i);
});

test("CI covers every integration branch without production runtime targets", async () => {
  const workflow = await read(".github/workflows/ci.yml");
  for (const branch of ["dev", "staging", "main"]) assert.ok(workflow.includes(`- ${branch}`));
  assert.doesNotMatch(workflow, /https:\/\/api\.unipost\.dev/);
  assert.doesNotMatch(workflow, /pk_live_/);
  assert.match(workflow, /NEXT_PUBLIC_API_URL: http:\/\/localhost:8080/);
  assert.match(workflow, /NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY:.*NEXT_PUBLIC_CLERK_DEVELOPMENT_PUBLISHABLE_KEY/);
});
```

- [ ] **Step 2: Run the contract test and verify RED**

Run:

```bash
node --test scripts/preview/release-guardrails.test.mjs
```

Expected: FAIL because `AGENTS.md` does not contain the exclusive-worktree and preview-first rules, `ci.yml` omits `staging`, and PR CI still points at production API/Clerk configuration.

- [ ] **Step 3: Add the absolute `AGENTS.md` rules**

Insert a new section immediately after `# UniPost Agent Workflow` with these normative requirements:

```md
## Absolute session isolation and CI/CD stop rules

These rules are absolute and override every conflicting instruction below. A violation requires Codex to stop immediately and report it to the user.

### One conversation, one exclusive branch and one exclusive worktree

- Every development conversation owns exactly one dedicated branch and one dedicated worktree.
- Codex must never use another conversation's branch or worktree for reading task state, editing, testing, committing, merging, pushing, deploying, or releasing.
- Before every write, test, commit, merge, push, deployment, or promotion, Codex must verify the absolute worktree path and current branch and prove that both belong to the current conversation.
- If ownership is missing, ambiguous, or mismatched, Codex must stop and ask the user. It must not switch branches, stash, reset, discard, overwrite, or include the unrelated state.
- Shared `dev`, `staging`, and `main` checkouts are integration-only. Normal implementation must not occur in them.

### Preview acceptance before `dev`

- A normal task must push only its own `dev-<task-slug>` branch and open a Draft pull request to `dev`.
- The Draft pull request must deploy an isolated Railway PR Environment and a Vercel Preview wired to that PR API.
- Codex must not merge into `dev` until local CI, Preview Acceptance, deployed regression, and Codex browser acceptance all succeed on the exact pull request head SHA.
- Unfinished or unaccepted work must remain outside `dev`.

### Regression failure is a hard stop

- A required test is failed when it fails, errors, times out, is cancelled, is skipped, cannot start, produces no result, or validates a different commit.
- On any such result, Codex must stop before merge, push to an integration branch, promotion, or deployment to the next environment.
- Codex must report the environment, branch, SHA, workflow, job, suite, test case, exact failure message and relevant log excerpt, run URL, artifact URLs, and whether any deployment or promotion already occurred.
- A suspected flaky result remains failed. Work resumes only after the cause is fixed and the complete required suite passes on the replacement SHA, unless the user explicitly authorizes an exception after reviewing the evidence.

### Promotion content audit

- Before every merge or promotion, Codex must list the exact commits and changed files unique to the source branch.
- Any unrelated, unidentified, unfinished, or unaccepted change is a hard blocker.
```

Replace the existing direct-push-to-`dev` default flow with Draft PR, Preview Acceptance, merge, persistent `dev` deployment, and browser acceptance. Keep `dev -> staging -> main` for cross-environment promotion.

- [ ] **Step 4: Remove production runtime targets from PR CI and include staging**

Update `.github/workflows/ci.yml` so both `pull_request.branches` and `push.branches` contain `dev`, `staging`, and `main`. Set:

```yaml
NEXT_PUBLIC_API_URL: http://localhost:8080
NEXT_PUBLIC_APP_URL: http://localhost:3000
NEXT_PUBLIC_BASE_URL: http://localhost:3000
NEXT_PUBLIC_LANDING_URL: http://localhost:3000
NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY: ${{ vars.NEXT_PUBLIC_CLERK_DEVELOPMENT_PUBLISHABLE_KEY }}
```

Add this step after checkout in the Dashboard job:

```yaml
- name: Validate release guardrails
  working-directory: ${{ github.workspace }}
  run: node --test scripts/preview/*.test.mjs
```

- [ ] **Step 5: Document the new gates**

Update `docs/ci-gates.md` to define:

- `API tests`, `Dashboard build`, and `Preview Acceptance`;
- task PRs target `dev`, promotion PRs target `staging` and `main`;
- preview failures are fail-closed;
- preview regression uses ephemeral URLs and non-production credentials;
- the mandatory failure report fields from `AGENTS.md`.

- [ ] **Step 6: Verify GREEN**

Run:

```bash
node --test scripts/preview/release-guardrails.test.mjs
```

Expected: PASS.

- [ ] **Step 7: Commit the workflow contract**

```bash
git add AGENTS.md .github/workflows/ci.yml docs/ci-gates.md scripts/preview/release-guardrails.test.mjs
git commit -m "chore: enforce isolated preview workflow"
```

### Task 2: Resolve the exact Railway PR API deployment

**Files:**
- Create: `scripts/preview/railway-deployments.mjs`
- Create: `scripts/preview/railway-deployments.test.mjs`

- [ ] **Step 1: Write failing deployment-selection tests**

Test pure selection with fixtures containing an API deployment, worker deployments without URLs, a Vercel deployment, and mismatched SHAs:

```js
import assert from "node:assert/strict";
import { test } from "node:test";
import { selectReadyRailwayAPI } from "./railway-deployments.mjs";

const sha = "a".repeat(40);

test("selects the successful Railway API deployment for the exact SHA", () => {
  const result = selectReadyRailwayAPI([
    { id: 1, sha, environment: "pr-42", statuses: [{ state: "success", environment_url: "https://api-pr-42.up.railway.app" }] },
    { id: 2, sha, environment: "pr-42-worker", statuses: [{ state: "success", environment_url: "" }] },
    { id: 3, sha, environment: "Preview", statuses: [{ state: "success", environment_url: "https://unipost.vercel.app" }] },
  ], sha);
  assert.equal(result.apiURL, "https://api-pr-42.up.railway.app");
});

test("rejects a successful Railway URL attached to another SHA", () => {
  assert.throws(() => selectReadyRailwayAPI([
    { id: 1, sha: "b".repeat(40), environment: "pr-42", statuses: [{ state: "success", environment_url: "https://api-pr-42.up.railway.app" }] },
  ], sha), /exact head SHA/);
});

test("rejects failed, permanent, and missing Railway URLs", () => {
  assert.throws(() => selectReadyRailwayAPI([
    { id: 1, sha, environment: "dev", statuses: [{ state: "success", environment_url: "https://dev-api.unipost.dev" }] },
    { id: 2, sha, environment: "pr-42", statuses: [{ state: "failure", environment_url: "https://api-pr-42.up.railway.app" }] },
  ], sha), /ready Railway PR API/);
});
```

- [ ] **Step 2: Run and verify RED**

```bash
node --test scripts/preview/railway-deployments.test.mjs
```

Expected: FAIL with `ERR_MODULE_NOT_FOUND`.

- [ ] **Step 3: Implement selection and polling**

Implement:

```js
const permanentHosts = new Set([
  "dev-api.unipost.dev",
  "staging-api.unipost.dev",
  "api.unipost.dev",
  "unipost-dev.up.railway.app",
  "unipost-staging.up.railway.app",
  "unipost-production.up.railway.app",
]);

export function selectReadyRailwayAPI(deployments, expectedSHA) {
  const exact = deployments.filter((deployment) => deployment.sha === expectedSHA);
  const ready = exact.flatMap((deployment) =>
    deployment.statuses.map((status) => ({ deployment, status })))
    .filter(({ status }) => status.state === "success" && status.environment_url)
    .filter(({ status }) => {
      const host = new URL(status.environment_url).hostname;
      return host.endsWith(".up.railway.app") && !permanentHosts.has(host);
    });
  if (ready.length !== 1) {
    const mismatch = deployments.some((deployment) => deployment.sha !== expectedSHA);
    throw new Error(mismatch && exact.length === 0
      ? "No Railway deployment matches the exact head SHA"
      : "Expected exactly one ready Railway PR API deployment");
  }
  return {
    apiURL: ready[0].status.environment_url.replace(/\/+$/, ""),
    deploymentId: ready[0].deployment.id,
    environment: ready[0].deployment.environment,
    sha: expectedSHA,
  };
}
```

The CLI portion will parse `--repo`, `--sha`, `--token`, `--output`, and `--manifest`; poll GitHub Deployments for up to 25 minutes; fetch statuses for every deployment; call `selectReadyRailwayAPI`; verify `${apiURL}/health`; write `api_url`, `railway_deployment_id`, and `railway_environment` to `$GITHUB_OUTPUT`; and write `artifacts/preview/railway.json` without secrets.

- [ ] **Step 4: Verify GREEN**

```bash
node --test scripts/preview/railway-deployments.test.mjs
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/preview/railway-deployments.mjs scripts/preview/railway-deployments.test.mjs
git commit -m "ci: resolve Railway preview deployment"
```

### Task 3: Add preview identity and browser acceptance

**Files:**
- Create: `scripts/preview/write-manifest.mjs`
- Create: `scripts/preview/write-manifest.test.mjs`
- Create: `dashboard/playwright.preview.config.ts`
- Create: `dashboard/tests/regression/preview-environment.spec.ts`
- Modify: `dashboard/package.json`
- Modify: `api/cmd/api/cors_test.go`

- [ ] **Step 1: Write failing manifest tests**

Test that the manifest accepts only a 40-character SHA, an ephemeral Railway URL, and an HTTPS Vercel branch host, and rejects all persistent environment hosts.

```js
import assert from "node:assert/strict";
import { test } from "node:test";
import { createPreviewManifest } from "./write-manifest.mjs";

test("creates a non-secret manifest for one preview SHA", () => {
  assert.deepEqual(createPreviewManifest({
    sha: "a".repeat(40),
    branch: "dev-preview-environment-guardrails",
    apiURL: "https://api-pr-42.up.railway.app",
  }), {
    sha: "a".repeat(40),
    branch: "dev-preview-environment-guardrails",
    apiURL: "https://api-pr-42.up.railway.app",
  });
});

test("rejects persistent API targets", () => {
  for (const apiURL of ["https://api.unipost.dev", "https://dev-api.unipost.dev", "https://staging-api.unipost.dev"]) {
    assert.throws(() => createPreviewManifest({
      sha: "a".repeat(40),
      branch: "dev-preview-environment-guardrails",
      apiURL,
    }), /ephemeral Railway/);
  }
});
```

- [ ] **Step 2: Run and verify RED**

```bash
node --test scripts/preview/write-manifest.test.mjs
```

Expected: FAIL with `ERR_MODULE_NOT_FOUND`.

- [ ] **Step 3: Implement manifest validation and CLI output**

`write-manifest.mjs` will export `createPreviewManifest`, parse `--sha`, `--branch`, `--api-url`, and `--output`, and write JSON to `dashboard/public/__unipost-preview.json`. It must reject production, staging, persistent development, non-HTTPS, non-Railway, and malformed SHA values.

- [ ] **Step 4: Add preview Playwright configuration and test**

Add the package script:

```json
"test:regression:preview": "playwright test --config=playwright.preview.config.ts"
```

Configure `playwright.preview.config.ts` with one Chromium worker, zero retries, an externally supplied `DASHBOARD_BASE_URL`, and HTML/JUnit reports.

The preview spec must:

```ts
test("frontend and API are the same preview SHA pair", async ({ page }) => {
  const manifest = await page.request.get("/__unipost-preview.json");
  expect(manifest.ok()).toBeTruthy();
  const data = await manifest.json();
  expect(data.sha).toBe(process.env.EXPECTED_PREVIEW_SHA);
  expect(data.apiURL).toBe(process.env.EXPECTED_PREVIEW_API_URL);

  await page.goto("/docs", { waitUntil: "domcontentloaded" });
  await expect(page.locator("article").first()).toContainText(/UniPost|API/);

  const health = await page.evaluate(async (apiURL) => {
    const response = await fetch(`${apiURL}/health`, { credentials: "include" });
    return { ok: response.ok, status: response.status };
  }, data.apiURL);
  expect(health).toEqual({ ok: true, status: 200 });
});
```

- [ ] **Step 5: Add the failing CORS test**

Extend `api/cmd/api/cors_test.go` with a test that sets `CORS_ALLOWED_ORIGINS=https://*.vercel.app`, sends a preflight from `https://unipost-dev-git-dev-example-xiaobo-yus-projects.vercel.app`, and expects the same origin in `Access-Control-Allow-Origin`.

- [ ] **Step 6: Verify browser and CORS tests**

Run:

```bash
node --test scripts/preview/write-manifest.test.mjs
GOCACHE=/tmp/unipost-go-build go test ./cmd/api -run CORS
```

Expected: PASS. The Playwright test is exercised against the deployed Preview in Task 6.

- [ ] **Step 7: Commit**

```bash
git add scripts/preview dashboard/package.json dashboard/playwright.preview.config.ts dashboard/tests/regression/preview-environment.spec.ts api/cmd/api/cors_test.go
git commit -m "test: add deployed preview acceptance"
```

### Task 4: Add the fail-closed Preview Acceptance workflow

**Files:**
- Create: `.github/workflows/preview-acceptance.yml`
- Modify: `scripts/preview/release-guardrails.test.mjs`

- [ ] **Step 1: Extend the static test and verify RED**

Require the workflow to:

- run only for PRs targeting `dev`;
- include opened, synchronize, reopened, ready-for-review, labeled, and unlabeled events;
- reject forks and branches not starting with `dev-`;
- use `pull_request.head.sha`;
- pin Vercel CLI to `50.26.1`;
- run `test:regression:preview`;
- use `if: always()` artifact upload and `if: failure()` failure summary;
- include a `preview-failure-drill` label gate;
- contain no production domain or production Clerk key.

Run:

```bash
node --test scripts/preview/release-guardrails.test.mjs
```

Expected: FAIL because the workflow does not exist.

- [ ] **Step 2: Create the workflow**

Create a job named `Preview Acceptance` with:

```yaml
name: Preview Acceptance

on:
  pull_request:
    branches: [dev]
    types: [opened, synchronize, reopened, ready_for_review, labeled, unlabeled]

permissions:
  contents: read
  deployments: read
  pull-requests: read

concurrency:
  group: preview-${{ github.event.pull_request.number }}
  cancel-in-progress: true
```

The job must:

1. Check out `${{ github.event.pull_request.head.sha }}`.
2. Run all `scripts/preview/*.test.mjs`.
3. Poll Railway through `railway-deployments.mjs`.
4. Create `dashboard/.vercel/project.json` from `VERCEL_ORG_ID` and `VERCEL_PROJECT_ID`.
5. Pull Vercel Preview environment variables.
6. Compute the branch alias host and write `__unipost-preview.json`.
7. Build with the Railway API URL and preview branch URL injected into all `NEXT_PUBLIC_*` origin variables.
8. Deploy prebuilt output with GitHub branch/SHA metadata.
9. Run `test:regression:preview` with expected SHA and API URL.
10. Fail intentionally with an exact message when the PR has `preview-failure-drill`.
11. Always upload `artifacts/preview`, Playwright results, and the HTML report.
12. On failure, append branch, SHA, Railway URL, Vercel URL, failed stage, and the Actions run URL to `$GITHUB_STEP_SUMMARY`.

Every credential comes from GitHub secrets or Vercel Preview variables. No secret value is echoed.

- [ ] **Step 3: Verify GREEN and workflow syntax**

Run:

```bash
node --test scripts/preview/*.test.mjs
ruby -e 'require "yaml"; YAML.load_file(".github/workflows/preview-acceptance.yml", aliases: true); puts "valid"'
```

Expected: all tests pass and YAML prints `valid`.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/preview-acceptance.yml scripts/preview/release-guardrails.test.mjs
git commit -m "ci: add isolated preview acceptance gate"
```

### Task 5: Configure provider credentials and sanitized Railway base

**External state:**
- GitHub repository `bugfreev587/unipost`
- Railway project `9f45a577-d452-43e1-992c-ff23d5f2293c`
- Vercel team `team_flEmxSwX7kS3AxbasYtviStA`
- Vercel project `prj_M8zHunydbwLF1hxLeSvIqypgt9pK`

- [ ] **Step 1: Install GitHub provider variables and secrets without printing them**

Run:

```bash
gh variable set VERCEL_ORG_ID --body "team_flEmxSwX7kS3AxbasYtviStA"
gh variable set VERCEL_PROJECT_ID --body "prj_M8zHunydbwLF1hxLeSvIqypgt9pK"
gh variable set VERCEL_TEAM_SLUG --body "xiaobo-yus-projects"
jq -r '.token' "$HOME/Library/Application Support/com.vercel.cli/auth.json" | gh secret set VERCEL_TOKEN
```

Verify only names and timestamps with `gh secret list --app actions` and `gh variable list`.

- [ ] **Step 2: Copy Clerk Development values into Vercel Preview securely**

Link a protected temporary directory to `unipost-staging`, pull its production environment, assert `NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY` begins with `pk_test_` and `CLERK_SECRET_KEY` begins with `sk_test_`, pipe those values into the `unipost-dev` Preview environment, then delete the temporary file. Never print either value.

Set these non-secret Preview values:

```text
NEXT_PUBLIC_UNIPOST_ENV=development
```

The workflow overrides API and frontend origins per branch at build time.

- [ ] **Step 3: Make the orchestrated Vercel Preview authoritative**

Set the `unipost-dev` Ignored Build Step so Git-triggered `dev-*` feature previews are skipped while the persistent `dev` deployment continues:

```bash
if [[ "$VERCEL_GIT_COMMIT_REF" == dev-* ]]; then exit 0; else exit 1; fi
```

Vercel treats exit `0` as “skip this Git build.” The GitHub workflow's local `vercel build` plus `vercel deploy --prebuilt` remains the only authoritative task preview.

- [ ] **Step 4: Create `preview-base` without starting unsafe services**

In Railway:

1. Duplicate `dev` as `preview-base` but leave its changes staged.
2. Keep one sleep-enabled API replica so PR Environments inherit a deployable web service; set post-delivery-worker, media-worker, and MCP replicas to zero before applying.
3. Keep isolated Postgres and Redis service references.
4. Set `UNIPOST_ENV=preview`.
5. Set `CORS_ALLOWED_ORIGINS=https://*.vercel.app`.
6. Set `POST_DELIVERY_WORKER_DISABLE_API_DELIVERY=true`.
7. Set `MEDIA_PROCESSING_WORKER_DISABLE_API_PROCESSING=true`.
8. Set `FEATURE_EMAIL_LOOPS_INTEGRATION_V1=false`.
9. Set `CHANGELOG_PUBLISH_DRY_RUN=true`.
10. Remove outbound/provider credentials from the preview API and workers: Anthropic, BetterStack, changelog automation/signing, Clerk webhook, Facebook/Instagram/Threads/LinkedIn/Pinterest/TikTok/X/YouTube credentials, Loops, Resend, R2 account and access credentials, Stripe secrets/webhook secrets, Unleash server token, and X Inbox webhook secret.
11. Retain only the Clerk Development server key, encryption key, admin identities, isolated database/Redis references, and non-secret feature/configuration values needed for boot.
12. Apply once, verify only the sanitized API can run, and confirm no deployment with outbound-capable credentials started.

- [ ] **Step 5: Enable Railway PR Environments**

Set:

- enabled: true;
- base environment: `preview-base`;
- Bot PR Environments: true;
- Focused PR Environments: false for initial proof, so every task receives an API and isolated data services.

Focused mode may be enabled only in a later change after dependency behavior is verified.

- [ ] **Step 6: Correct environment source mappings**

Change Railway MCP source branches:

- `dev / mcp` -> `dev`;
- `staging / mcp` -> `staging`.

Wait for both deployments, verify terminal success, and record the deployed SHA and branch. Do not alter production MCP.

### Task 6: Push the task branch and prove both failure and success

**External state:**
- Branch `dev-preview-environment-guardrails`
- Draft PR to `dev`

- [ ] **Step 1: Run complete local validation**

Run:

```bash
node --test scripts/preview/*.test.mjs
GOCACHE=/tmp/unipost-go-build go test ./...
npm run build
npm run test:regression:dashboard
```

Run Go commands from `api/` and npm commands from `dashboard/`. Expected: all required checks pass; authenticated dashboard regression may not be silently skipped when its credentials are configured.

- [ ] **Step 2: Audit and push only the owned branch**

Verify:

```bash
pwd
git branch --show-current
git status --short
git log --oneline origin/dev..HEAD
git diff --name-status origin/dev...HEAD
```

Push:

```bash
git push -u origin dev-preview-environment-guardrails
```

- [ ] **Step 3: Open a Draft PR to `dev`**

Create a Draft PR titled `chore: isolate branch preview deployments` and include the exact expected outcome, local validation, preview architecture, and rollback.

- [ ] **Step 4: Prove hard-stop failure reporting**

Create and apply the `preview-failure-drill` label. Wait for `Preview Acceptance` to fail. Verify:

- the PR is not mergeable;
- the failure message names the deliberate failure;
- the Actions run summary includes branch, SHA, Railway URL, Vercel URL, and run URL;
- preview manifest and Playwright artifacts are retained.

Remove the label and wait for the complete workflow to rerun.

- [ ] **Step 5: Prove successful preview**

Verify:

- Railway PR environment uses `preview-base`;
- API deployment SHA equals the PR head SHA;
- `/health` passes;
- Vercel Preview metadata SHA equals the PR head SHA;
- `__unipost-preview.json` names the ephemeral Railway URL;
- browser CORS fetch succeeds;
- no production API or Clerk Production configuration appears in the Preview;
- all GitHub checks are successful.

- [ ] **Step 6: Install branch protection**

Protect:

- `dev`: require PR, strict up-to-date branch, `API tests`, `Dashboard build`, `Preview Acceptance`, enforce admins, disallow force push/deletion;
- `staging`: require PR, strict up-to-date branch, `API tests`, `Dashboard build`, enforce admins, disallow force push/deletion;
- `main`: require PR, strict up-to-date branch, `API tests`, `Dashboard build`, enforce admins, disallow force push/deletion.

Use zero required approvals so a solo-maintainer release is possible, but do not permit direct pushes.

- [ ] **Step 7: Mark ready, merge, and verify persistent development**

Mark the PR ready only after all checks succeed. Merge to `dev`, wait for GitHub Actions, Vercel `unipost-dev`, Railway `dev`, and corrected `dev/mcp` deployments. Verify the changed workflow documentation on the exact merged SHA and open the relevant development frontend/API domains for health acceptance.

- [ ] **Step 8: Verify cleanup**

Confirm the PR Railway environment is deleted after merge and no orphan `pr-*` environment remains.

### Task 7: Final verification and evidence

- [ ] **Step 1: Re-run repository validation on updated `dev`**

```bash
node --test scripts/preview/*.test.mjs
GOCACHE=/tmp/unipost-go-build go test ./...
npm run build
npm run test:regression:dashboard
```

- [ ] **Step 2: Verify provider inventory**

Record without secret values:

- GitHub required checks for `dev`, `staging`, and `main`;
- Railway branch mappings for all persistent environments;
- `preview-base` application replica counts and sanitized variable names;
- Vercel Preview variable names and environments;
- the Vercel Git ignored-build rule that suppresses duplicate automatic `dev-*` previews while retaining the orchestrated CLI Preview;
- successful preview and persistent development deployment IDs/URLs/SHAs;
- failed drill Actions run URL and retained artifacts.

- [ ] **Step 3: Report completion**

Report the task complete only after every required check, deployment, cleanup, and real development-environment acceptance is successful. Include exact failures if any gate remains non-successful; do not claim partial configuration is complete.
