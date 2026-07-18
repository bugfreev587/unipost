# Preview Environment and Release Guardrails Design

## Summary

UniPost will validate every normal development task in an isolated, branch-specific preview stack before the task can enter the shared `dev` integration branch. Each Codex conversation owns one branch and one worktree. A Draft pull request from that branch to `dev` becomes the unit that triggers local CI, an ephemeral Railway backend environment, a Vercel frontend preview wired to that backend, deployed regression, and Codex browser acceptance.

No failed, incomplete, cancelled, timed-out, skipped, or indeterminate required check may be bypassed. A failed gate stops the workflow before merge or promotion and produces a concrete failure report.

## Goals

1. Prevent one conversation from modifying, committing, testing, deploying, or releasing another conversation's work.
2. Keep unfinished features out of `dev`, `staging`, and `main`.
3. Give each task a real deployed environment for backend, frontend, database, and browser acceptance before merge.
4. Make regression failure a fail-closed release gate.
5. Preserve the existing environment promotion chain after a task has passed preview acceptance:
   `task preview -> dev -> staging -> main`.
6. Make the exact commit SHA and changed-file set visible at every merge and promotion boundary.

## Non-goals

- Preview environments are not production-like sources of real customer data.
- Preview environments do not send real outbound posts, emails, webhooks, scheduled jobs, or third-party mutations by default.
- This design does not replace the persistent `dev`, `staging`, or production environments.
- This design does not introduce feature flags automatically.
- This design does not permit feature branches to promote directly to `staging` or `main`.

## Session isolation contract

Every Codex development conversation must:

1. Fetch `origin`.
2. Create a new branch directly from the required base ref.
3. Create and remain inside a dedicated worktree for that branch.
4. Rename the Codex task to exactly match the branch name.
5. Verify the absolute worktree path and current branch before any write, test, commit, push, merge, deployment, or promotion.

A worktree and branch are owned exclusively by the conversation that created them. Codex must stop if ownership cannot be established or if unrelated changes appear. Codex may not stash, reset, discard, overwrite, or include another conversation's changes.

Shared checkouts of `dev`, `staging`, and `main` may be used only as integration checkouts when their ownership and cleanliness are proven. Normal implementation never occurs in them.

## Branch and pull request lifecycle

The normal task lifecycle is:

1. Create `dev-<task-slug>` from the latest `origin/dev` in a dedicated worktree.
2. Implement and run the local CI-equivalent checks.
3. Push only `origin/dev-<task-slug>`.
4. Open a Draft pull request from `dev-<task-slug>` to `dev`.
5. Wait for local CI and the complete preview stack.
6. Run deployed regression and Codex browser acceptance against the preview URLs.
7. Mark the pull request ready only after preview acceptance passes.
8. Merge to `dev` only when the task is complete, release-eligible, and every required check is successful.
9. Wait for the persistent development deployments and repeat acceptance on the official development domains.

Closing or merging the pull request destroys its ephemeral Railway resources. The task branch remains reviewable until normal branch cleanup.

## Preview architecture

### Railway backend

Railway native PR Environments will be enabled for the UniPost project with:

- base environment: a persistent `preview-base` derived from `dev` but stripped of outbound and production-capable credentials before its first deployment;
- automatic PR Environments enabled;
- Bot PR Environments enabled;
- Focused PR Environments enabled where service dependency behavior is proven;
- Railway-provided domains retained on the development API so PR API domains are created automatically.

The ephemeral environment must deploy the pull request head SHA. The API service must expose a unique HTTPS URL and pass `/health` before frontend preview construction begins.

The preview environment uses isolated Postgres and Redis services. It must never point to the persistent development, staging, or production databases or queues.

`preview-base` is configuration infrastructure, not a shared feature-validation target. Its sanitized API keeps one sleep-enabled replica so Railway PR Environments inherit a deployable web service; MCP and worker replicas remain at zero. PR Environments inherit its sanitized variables and create the task-specific runtime. This avoids the unsafe interval that would occur if a PR environment copied live development credentials and only sanitized them after services had already started.

### Vercel frontend

The Vercel `unipost-dev` project will remain the frontend preview project. A GitHub Actions preview orchestrator will build and deploy the exact pull request head SHA through the Vercel CLI after the Railway API is healthy.

The build receives:

- the ephemeral Railway API URL as `NEXT_PUBLIC_API_URL`;
- Clerk Development credentials, never Clerk Production credentials;
- preview-safe application, landing, and callback origins;
- Vercel Git metadata for the exact repository, branch, and commit SHA.

The resulting Vercel deployment must expose:

- a commit-specific URL for immutable evidence;
- a branch-specific URL for the latest task preview.

Automatic duplicate Vercel Git previews should be disabled or ignored for `dev-*` branches once the orchestrated preview is proven, so one SHA has one authoritative acceptance deployment.

### Preview coordination

The orchestrator must not guess which backend deployment belongs to a frontend build. It must correlate:

- repository;
- pull request number;
- source branch;
- head commit SHA;
- Railway environment and API service;
- Vercel deployment metadata.

If the Railway API URL, deployment SHA, or service identity is missing or mismatched, the preview gate fails closed.

## Preview runtime safety

Railway PR Environments copy configuration from a base environment, so `preview-base` must be sanitized before it is allowed to deploy or become the PR Environment source. Preview defaults must:

- use an isolated Postgres database and Redis instance;
- use Clerk Development;
- disable scheduled execution;
- disable post-delivery and media workers unless the task explicitly requires them;
- disable real outbound email, posting, webhook delivery, billing mutation, and destructive cleanup;
- avoid production tokens and production callback domains;
- use dedicated test accounts or no-op providers for third-party integrations;
- prevent preview migrations or jobs from touching persistent environment data.

If a task requires a normally disabled side effect, the expected outcome must identify the exact preview-only dependency and acceptance procedure before it is enabled.

## CI and deployed regression

### Pull request CI

The existing `CI` workflow remains the first required gate for pull requests targeting `dev`. It runs:

- backend Go tests and coverage;
- dashboard source regression;
- dashboard production build;
- local Playwright dashboard smoke.

The workflow must stop using production API and Clerk configuration. Pull request CI will use explicit non-production test configuration.

### Preview deployment gate

A new required `Preview Acceptance` workflow will:

1. Resolve the open pull request and exact head SHA.
2. Wait for the Railway API deployment for that SHA.
3. Verify API health and record the Railway deployment URL.
4. Build and deploy the Vercel preview against that API.
5. Verify the Vercel deployment reaches a terminal `READY` state for the same SHA.
6. Run API smoke tests that are safe for an isolated preview.
7. Run dashboard Playwright regression against the Vercel preview.
8. Upload logs, Playwright reports, and a machine-readable preview manifest.
9. Publish a GitHub job summary containing the PR, branch, SHA, Railway URL, Vercel URL, and every gate result.

Authenticated or third-party regression that cannot safely run in preview must be explicitly classified and must remain a required persistent-`dev` acceptance step after merge. It may not silently appear as passed or skipped.

### Failure semantics

The following results are all failures:

- failed;
- errored;
- timed out;
- cancelled;
- skipped when required;
- unable to start;
- missing result;
- deployment SHA mismatch;
- deployment URL missing;
- health verification inconclusive.

On failure, Codex and CI must:

1. stop before merge, push to an integration branch, promotion, or next-environment deployment;
2. preserve and upload available logs and artifacts;
3. report environment, branch, commit SHA, workflow, job, suite, test case, exact failure, relevant log excerpt, run link, artifact links, and whether any deployment already occurred;
4. require a fix and a complete successful rerun on the exact replacement SHA.

Suspected flakiness is still a failure. Only the user may authorize an exception after receiving the failure evidence.

## Merge and promotion protection

Before merging a task into `dev`, Codex must report:

- pull request number;
- accepted head SHA;
- commits in `origin/dev..HEAD`;
- changed files in `origin/dev...HEAD`;
- preview URLs;
- required check conclusions.

Before `dev -> staging` or `staging -> main`, Codex must report the commits and files unique to the source environment branch. Any unrelated, unidentified, unfinished, or unaccepted change is a hard release blocker.

Branch protection should require:

- pull requests for `dev`, `staging`, and `main`;
- `API tests`;
- `Dashboard build`;
- `Preview Acceptance` for task pull requests targeting `dev`;
- up-to-date branches before merge;
- no direct push to `staging` or `main`;
- no administrator bypass except an explicitly authorized emergency.

## Provider configuration corrections

The current deployment inventory contains configuration that must be corrected as part of rollout:

- Railway `dev` API and workers track `dev`, but the `dev` MCP service currently tracks `main`.
- Railway `staging` API and post-delivery worker track `staging`, but the `staging` MCP service currently tracks `main`.
- GitHub Actions currently exposes a production Clerk publishable key to pull request CI.
- Pull request Dashboard CI currently sets `NEXT_PUBLIC_API_URL` to the production API.

The Railway MCP source branches must be corrected to match their environments. Pull request and preview workflows must use only non-production variables.

## Secrets and configuration

GitHub Actions will require:

- `VERCEL_TOKEN`;
- `VERCEL_ORG_ID`;
- `VERCEL_PROJECT_ID` for `unipost-dev`;
- Clerk Development preview credentials;
- preview-only regression credentials where safe.

Secret values must never be committed, printed, included in artifacts, or copied into `AGENTS.md`. Public identifiers may be GitHub Actions variables rather than secrets.

## Cleanup and cost controls

- Railway PR environments are deleted when a PR closes or merges.
- Codex verifies that Railway removes the task PR Environment when the pull request closes or merges.
- Vercel preview retention follows project policy; branch previews cease updating after branch deletion.
- Focused PR Environments are enabled only after verifying that API, worker, database, and Redis dependencies are neither omitted nor unnecessarily replicated.
- Preview workers remain disabled by default to avoid duplicate background processing and third-party side effects.

## Acceptance criteria

The rollout is complete only when a test pull request proves all of the following:

1. The task runs in its own branch and worktree.
2. The PR cannot merge while any required check is non-successful.
3. Railway creates an isolated PR environment from `dev`.
4. The preview API deployment contains the PR head SHA and passes `/health`.
5. Vercel deploys the same head SHA and calls the preview API rather than persistent dev or production.
6. Preview uses Clerk Development and contains no production runtime credentials.
7. API smoke and Dashboard Playwright regression pass against the preview stack.
8. A deliberately failing regression blocks the workflow and produces the required failure report and artifacts.
9. Closing the test PR removes the Railway PR environment.
10. Railway `dev` and `staging` MCP services track `dev` and `staging`, respectively.
11. After merge, persistent `dev` deployments complete and Codex verifies the expected behavior on the official development domains.

## Rollout order

1. Add the absolute rules to `AGENTS.md`.
2. Add static tests for the workflow and rule invariants.
3. Add the preview workflow and reporting scripts.
4. Add GitHub variables and secrets without exposing values.
5. Create and sanitize `preview-base`, then enable Railway PR Environments with `preview-base` as their source.
6. Correct Railway MCP branch mappings.
7. Configure Vercel Preview variables and authoritative orchestration.
8. Push the task branch and open a Draft PR to `dev`.
9. Exercise one successful preview and one intentional regression failure.
10. Merge only after successful preview acceptance, then verify persistent `dev`.

## References

- [Railway PR Environments](https://docs.railway.com/guides/preview-deployments-with-pr-environments)
- [Railway GitHub Actions PR environments](https://docs.railway.com/guides/github-actions-pr-environment)
- [Railway post-deployment GitHub Actions](https://docs.railway.com/guides/github-actions-post-deploy)
- [Vercel Git deployments](https://vercel.com/docs/git)
- [Vercel environment variables](https://vercel.com/docs/environment-variables)
