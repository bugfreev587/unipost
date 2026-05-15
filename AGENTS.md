# UniPost Agent Workflow

## Default branch flow

- Treat `dev` as the default integration branch for all development work.
- Do not develop directly on `dev` unless the user explicitly asks for it.
- At the start of a new conversation, before creating any development branch, update the local `dev` branch first:
  1. Fetch `origin`.
  2. Switch to local `dev`.
  3. Pull the latest `dev` from remote with `git pull --ff-only origin dev`.
- For code or documentation changes, start from that freshly updated local `dev` branch:
  1. Create a short-lived branch from `dev` named `dev-<task-slug>`.
  2. Rename the conversation/thread to exactly match the new branch name.
- Do all implementation and local testing on the `dev-<task-slug>` branch.
- After implementation is complete and tests pass, merge the task branch back into local `dev`.
- Run the relevant validation again on local `dev`.
- If validation passes, push local `dev` to `origin/dev`.
- Create a pull request from `dev` to `main`.

## Safety rules

- Before switching branches, inspect `git status`.
- Never overwrite, reset, checkout away, or stash user changes unless the user explicitly approves it.
- If unrelated local changes prevent switching branches or merging, stop and ask the user how to handle them.
- Keep commits focused on the requested change. Do not include `artifacts/` or unrelated generated files unless the user asks.
- If the user explicitly requests a different branch, direct push, hotfix, or production change, follow that latest instruction instead of the default flow.

## Feature flag and production isolation rules

- Any new feature that can affect production users, third-party OAuth scopes, billing, writes, background jobs, external API calls, user-visible workflow changes, or data access must be protected by a feature flag before it is merged to `main`.
- Use Unleash as the remote feature flag provider. The UniPost backend is the authority for sensitive decisions; the frontend may hide or show UI from `/v1/me/features`, but it must not connect to Unleash directly or receive Unleash tokens.
- Production defaults must be conservative. New risky flags should be `off` in `production`, and may be `on` in `development` only after the backend fallback is safe.
- Backend checks must go through `api/internal/featureflags` or the existing shared feature flag API. Do not add scattered environment-variable reads for individual features unless they are part of the provider fallback.
- Frontend checks must use the backend feature surface, currently `GET /v1/me/features`, so browser behavior matches backend-evaluated flags.
- When adding a flag, document its key, owner area, production default, rollback action, and any third-party approval dependency in `docs/feature-flags-unleash.md`.
- For high-risk flags, verify both paths locally or in dev: flag on enables the new behavior, and flag off preserves the old production-safe behavior.
- Emergency rollback should be a flag toggle in Unleash production. Code rollback should be the fallback only when the flag cannot contain the issue.
