# UniPost Agent Workflow

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

## Three-environment release flow

- Use this promotion chain for all normal releases: `dev-<task-slug>` -> `dev` -> `staging` -> `main`.
- Environment mapping:
  1. `dev` deploys to Vercel `unipost-dev`, Railway `dev`, and Clerk Development.
  2. `staging` deploys to Vercel `unipost-staging`, Railway `staging`, and Clerk Development.
  3. `main` deploys to Vercel `unipost`, Railway `production`, and Clerk Production.
- Treat `dev` as the integration environment for active work. It may receive frequent changes.
- Treat `staging` as pre-production. Only promote changes to `staging` when they are intended to become a release candidate.
- Treat `main` as production. Only promote from `staging` to `main` after staging validation passes and the user explicitly approves a production release or PR.
- Do not merge a feature branch directly into `staging` or `main` during normal development. The only exception is a user-approved hotfix flow.
- Do not create a pull request from `dev` to `main`. Production PRs must be from `staging` to `main` unless the user explicitly approves a hotfix exception.
- Use pull requests for cross-environment promotions:
  1. Release candidate: PR or merge `dev` -> `staging`.
  2. Production release: PR `staging` -> `main`.
- Before creating any explicitly requested promotion pull request, run the local CI-equivalent checks for the changed surface:
  1. Backend/API changes: from `api/`, run `GOCACHE=/tmp/unipost-go-build go test ./...`.
  2. Dashboard/frontend/docs changes: from `dashboard/`, run `npm run build`.
  3. Dashboard routing, auth, onboarding, analytics, posting, account connection, docs shell, or shared UI shell changes: from `dashboard/`, run `npm run test:regression:dashboard` when Playwright browsers are installed.
- If the user explicitly asks for a PR and a required check cannot be run, report the skipped check and why before creating the PR.
- After promoting to `staging`, validate the deployed staging environment before promoting to `main`.
- After promoting to `main`, verify the production health check and any changed critical user flow.

## Hotfix flow

- Use this flow only for urgent production fixes or when the user explicitly asks for a hotfix.
- Start hotfixes from the latest `main`, not from `dev`:
  1. Fetch `origin`.
  2. Switch to local `main`.
  3. Pull the latest `main` from remote with `git pull --ff-only origin main`.
  4. Create `hotfix-<task-slug>` from `main`.
- Keep the hotfix branch narrowly scoped to the production issue.
- Run the relevant local CI-equivalent checks before creating or merging the hotfix PR.
- Merge the hotfix to `main` only after the required validation passes and the user approves the production change.
- After the hotfix reaches `main`, immediately backport it to both `staging` and `dev` by merge or cherry-pick. Do not leave production-only hotfix commits absent from `staging` or `dev`, because the next normal release could otherwise reintroduce the bug.
- If the backport has conflicts or cannot be applied cleanly, stop and ask the user how to proceed.

## Push and PR rules

- Normal task branches may be pushed to `origin/dev-<task-slug>` for review or backup.
- `dev` may be pushed after task validation passes.
- `staging` may be pushed only as a release-candidate promotion from `dev`, unless the user explicitly approves another source.
- `main` may be pushed or merged only as a production release from `staging` or as a user-approved hotfix from `hotfix-<task-slug>`.
- Never bypass `staging` for normal production releases.
- Never use production domains for dev-environment validation, and never use dev domains for production release validation.

## Completion monitoring rules

- After any direct push, pull request merge, merge request merge, or branch promotion, Codex must monitor the triggered remote checks until they finish. This includes GitHub Actions, Vercel deployments, Railway deployments, and any other required or visibly triggered test/deploy checks.
- A push, merge, or promotion is not complete while any required or triggered check is queued, pending, running, or waiting for deployment.
- After local code changes are pushed to `origin/dev`, Codex must wait for the development deployment to complete, then personally open the relevant development domain in a browser and verify that the change works before stopping the task or reporting final results.
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

- Before starting implementation for API-layer changes or Dashboard-layer changes, proactively ask the user whether the change should be protected by a feature flag.
- Add a feature flag only when the user explicitly says the change needs one, including when they answer yes to the pre-implementation flag prompt. Otherwise, do not add a feature flag.
- When the user says a feature flag is needed, create the flag in Unleash at `https://flags.unipost.dev` before wiring the flag into code. If you cannot access or log in to `flags.unipost.dev`, ask the user for help.
- Use Unleash as the remote feature flag provider. The UniPost backend is the authority for sensitive decisions; the frontend may hide or show UI from `/v1/me/features`, but it must not connect to Unleash directly or receive Unleash tokens.
- Production defaults must be conservative. New flags should be `off` in `production`, and may be `on` in `development` only after the backend fallback is safe.
- Backend checks must go through `api/internal/featureflags` or the existing shared feature flag API. Do not add scattered environment-variable reads for individual features unless they are part of the provider fallback.
- Frontend checks must use the backend feature surface, currently `GET /v1/me/features`, so browser behavior matches backend-evaluated flags.
- When adding a flag, document its key, owner area, production default, rollback action, and any third-party approval dependency in `docs/feature-flags-unleash.md`.
- For high-risk flags, verify both paths locally or in dev: flag on enables the new behavior, and flag off preserves the old production-safe behavior.
- Emergency rollback should be a flag toggle in Unleash production. Code rollback should be the fallback only when the flag cannot contain the issue.
