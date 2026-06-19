# UniPost Agent Workflow

## Non-negotiable development rule

- This rule applies to every development task and must not be skipped: if the task's expected final outcome is unclear, ask the user for the target outcome before starting implementation. Do not infer, invent, or assume the final goal.
- After any change is pushed to `origin/dev`, Codex must wait for the triggered development deployment to finish, then perform self-acceptance in the real dev environment against the expected outcome. Codex may report the task complete only after the real dev environment matches the expected outcome.

## Default branch flow

- Treat `dev` as the default integration branch for all normal development work.
- Do not develop directly on `dev` unless the user explicitly asks for it.
- At the start of every new conversation, before writing code or documentation changes, fetch `origin` and create a new short-lived branch directly from the latest `origin/dev`.
  1. Run `git fetch origin`.
  2. Create a branch from `origin/dev` named `dev-<task-slug>`.
  3. Rename the conversation/thread to exactly match the new branch name.
- Do not base new development work on the current local `dev` branch unless the user explicitly asks for it.
- Do all implementation and local testing on the `dev-<task-slug>` branch.
- For every code change, start from the latest `origin/dev`, create a clean `dev-<task-slug>` branch, and work only on that new branch until the change is ready.
- After code changes are complete, pull the latest `origin/dev` into local `dev`, merge the task branch into local `dev`, and rerun all required tests before considering the task complete.
- After implementation is complete and tests pass on both the task branch and the updated local `dev`, push local `dev` directly to `origin/dev` unless the user explicitly asks for a pull request instead.
- Run the relevant validation again on local `dev` before pushing `dev`.
- If validation passes, push local `dev` to `origin/dev`.
- Pushing or merging to `dev` deploys the development environment only. Do not promote from `dev` to `staging` or `main` unless the user explicitly asks for a release, staging promotion, production release, or PR.
- If the user says to run the standard release flow, follow the `Standard release flow` section after completing the normal task-branch work.

## Standard release flow

- Use this flow when the user says to run the standard release flow, walk the standard release flow, or `走标准发布流程`.
- That instruction means Codex should carry the change through development, staging, and production unless the user explicitly narrows the target environment.
- Use this promotion chain for all standard releases: `dev-<task-slug>` -> `origin/dev` -> PR to `staging` -> PR to production (`main`).
- Environment mapping:
  1. `dev` deploys to Vercel `unipost-dev`, Railway `dev`, and Clerk Development.
  2. `staging` deploys to Vercel `unipost-staging`, Railway `staging`, and Clerk Development.
  3. `main` deploys to Vercel `unipost`, Railway `production`, and Clerk Production.
- Start every standard release from a clean branch based on the latest `origin/dev`:
  1. Run `git fetch origin`.
  2. Create `dev-<task-slug>` directly from `origin/dev`.
  3. Rename the conversation/thread to exactly match `dev-<task-slug>`.
  4. Implement and validate the change on `dev-<task-slug>`.
- After implementation is complete, update local `dev` from `origin/dev`, merge `dev-<task-slug>` into local `dev`, rerun the required validation, then push local `dev` directly to `origin/dev`.
- After pushing `origin/dev`, wait for all triggered checks and development deployments to finish, then verify the expected outcome in the real development environment.
- After development verification passes, create a promotion PR from `dev` to `staging`. Merge the PR after its checks pass, wait for the staging redeployment to finish, then verify the expected outcome in the real staging environment.
- After staging verification passes, create a production PR from `staging` to `main`. Merge the PR after its checks pass, wait for the production redeployment to finish, then verify production health and the changed critical user flow in the real production environment.
- Do not merge a feature branch directly into `staging` or `main` during the standard release flow.
- Do not create a pull request from `dev` to `main`. Production PRs must be from `staging` to `main`.
- Before creating any promotion pull request, run the local CI-equivalent checks for the changed surface:
  1. Backend/API changes: from `api/`, run `GOCACHE=/tmp/unipost-go-build go test ./...`.
  2. Dashboard/frontend/docs changes: from `dashboard/`, run `npm run build`.
  3. Dashboard routing, auth, onboarding, analytics, posting, account connection, docs shell, or shared UI shell changes: from `dashboard/`, run `npm run test:regression:dashboard` when Playwright browsers are installed.
- If the user explicitly asks for a PR and a required check cannot be run, report the skipped check and why before creating the PR.
- If any check, deployment, or environment verification fails, inspect the failure, make the needed fix in the correct source branch for that stage, rerun validation, and continue the same release flow only after the failure is resolved.

## Hotfix flow

- Use this flow only for urgent production fixes or when the user explicitly asks for a hotfix.
- Start hotfixes from the latest `origin/staging`, not from `dev` or `main`:
  1. Fetch `origin`.
  2. Create `hotfix-<task-slug>` directly from `origin/staging`.
  3. Rename the conversation/thread to exactly match `hotfix-<task-slug>`.
- Keep the hotfix branch narrowly scoped to the production issue.
- Implement and validate the fix on `hotfix-<task-slug>`.
- Merge the hotfix into local `staging`, rerun the required validation, then push `staging` to `origin/staging`.
- After pushing `origin/staging`, wait for all triggered checks and the staging redeployment to finish, then verify the fix in the real staging environment.
- After staging verification passes, create a production PR from `staging` to `main`. Merge the PR after its checks pass, wait for the production redeployment to finish, then verify production health and the fixed user flow in the real production environment.
- After production verification passes, sync the same change back to `dev` by merging or cherry-picking it into local `dev`, rerun required validation, push `dev` to `origin/dev`, wait for the development deployment, and verify the development environment.
- If the sync to `dev` has conflicts or cannot be applied cleanly, stop and ask the user how to proceed.

## Push and PR rules

- Normal task branches may be pushed to `origin/dev-<task-slug>` for review or backup.
- `dev` may be pushed after task validation passes.
- Use pull requests for cross-environment promotions in the standard release flow: `dev` -> `staging`, then `staging` -> `main`.
- Use pull requests for production hotfix promotion: `staging` -> `main`.
- `staging` may be updated only by a standard release promotion from `dev` or by a hotfix branch that started from `origin/staging`.
- `main` may be updated only by a production release or hotfix PR from `staging`.
- Never bypass `staging` for normal production releases.
- Never use production domains for dev-environment validation, and never use dev domains for production release validation.

## Completion monitoring rules

- After any direct push, pull request merge, merge request merge, or branch promotion, Codex must monitor the triggered remote checks until they finish. This includes GitHub Actions, Vercel deployments, Railway deployments, and any other required or visibly triggered test/deploy checks.
- A push, merge, or promotion is not complete while any required or triggered check is queued, pending, running, or waiting for deployment.
- After any changes are pushed to `origin/dev`, Codex must wait for the development deployment to complete, then personally open the relevant development domain in a browser and verify that the change works before stopping the task or reporting final results.
- Codex may report interim status, but must not claim the task is finished until all required or triggered checks have completed successfully and the push or merge itself is confirmed successful.
- If any required or triggered check fails, Codex must inspect the failure logs, identify the cause, make the needed fix when it is within scope, rerun local validation, push the fix, and monitor the checks again.
- If a failure requires credentials, external approval, paid-service access, or a product decision that Codex cannot safely resolve, Codex must stop, report the exact blocker, and ask the user for the missing permission or decision.

## Safety rules

- Before switching branches, inspect `git status`.
- Never overwrite, reset, checkout away, or stash user changes unless the user explicitly approves it.
- If unrelated local changes prevent switching branches or merging, stop and ask the user how to handle them.
- Keep commits focused on the requested change. Do not include `artifacts/` or unrelated generated files unless the user asks.
- If the user explicitly requests a different branch, direct push, hotfix, or production change, follow that latest instruction instead of the default flow, while still preserving user changes and reporting any skipped validation.

## Environment domains

- Development backend API: `https://dev-api.unipost.dev`
- Development landing frontend: `https://dev.unipost.dev`
- Development app frontend: `https://dev-app.unipost.dev`
- Staging backend API: `https://staging-api.unipost.dev`
- Staging landing frontend: `https://staging.unipost.dev`
- Staging app frontend: `https://staging-app.unipost.dev`
- Production backend API: `https://api.unipost.dev`
- Production landing frontend: `https://unipost.dev`
- Production app frontend: `https://app.unipost.dev`
- When testing deployed development changes, use the development domains above. Do not use production domains for dev-environment validation.
- When testing deployed staging changes, use the staging domains above. Do not use production domains for staging validation unless the user is explicitly validating production.

## UI design skill

For frontend, dashboard, landing page, component styling, layout, typography, or visual polish work, use the installed `design-taste-frontend` / `taste-skill` Codex skill before designing or editing UI.

## Feature flag and production isolation rules

- Default to no feature flag for API-layer changes and Dashboard-layer changes. Do not proactively ask whether a change needs a feature flag.
- Add a feature flag only when the user explicitly says the change needs one. Otherwise, do not add a feature flag.
- When the user says a feature flag is needed, create the flag in Unleash at `https://flags.unipost.dev` before wiring the flag into code. If you cannot access or log in to `flags.unipost.dev`, ask the user for help.
- Use Unleash as the remote feature flag provider. The UniPost backend is the authority for sensitive decisions; the frontend may hide or show UI from `/v1/me/features`, but it must not connect to Unleash directly or receive Unleash tokens.
- Production defaults must be conservative. New flags should be `off` in `production`, and may be `on` in `development` only after the backend fallback is safe.
- Backend checks must go through `api/internal/featureflags` or the existing shared feature flag API. Do not add scattered environment-variable reads for individual features unless they are part of the provider fallback.
- Frontend checks must use the backend feature surface, currently `GET /v1/me/features`, so browser behavior matches backend-evaluated flags.
- When adding a flag, document its key, owner area, production default, rollback action, and any third-party approval dependency in `docs/feature-flags-unleash.md`.
- For high-risk flags, verify both paths locally or in dev: flag on enables the new behavior, and flag off preserves the old production-safe behavior.
- Emergency rollback should be a flag toggle in Unleash production. Code rollback should be the fallback only when the flag cannot contain the issue.
