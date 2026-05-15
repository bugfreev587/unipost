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
