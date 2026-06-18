---
name: unipost-debug
description: Diagnose and repair a broken UniPost API integration using the installed `unipost` CLI. Use when the user says something like "fix my UniPost integration", "UniPost posting is failing", or "use UniPost debug".
---

# UniPost Agent Debug Kit

You are debugging the user's own UniPost API integration from inside their
project repository. The `unipost` CLI exposes deterministic, machine-readable
diagnostics. Your job is to run them, act on the recommended actions safely,
verify the fix, and escalate only when you cannot resolve it.

## Closed loop

```
diagnose -> explain -> patch local code/config -> verify -> escalate only if needed
```

## Steps

1. Run `unipost auth status --json`. If `data.state` is `missing`, ask the user
   to run `unipost init` or paste the Dashboard-generated setup command. If
   `data.state` is `metadata_only`, explain that authenticated commands are not
   ready and ask the user to rebind with `unipost auth login --api-key <key>` or
   set `UNIPOST_API_KEY`.
2. Run `unipost doctor diagnose --json`. Parse `data.schema_version` (`doctor.v1`),
   `data.status`, `data.findings`, and `data.local_project`. Never scrape the
   human text.
3. For each finding, read `recommended_actions`. The `safety` field decides what
   you may do automatically:
   - `read_only` / `safe_to_execute_without_user` — apply directly (e.g. patch
     application code, fix the `Authorization: Bearer <key>` header).
   - `needs_user_approval` — explain and ask before changing it.
   - `manual_only` — a real product step (e.g. connect an account in the
     dashboard). Tell the user what to do; do not fake it.
4. Use `data.local_project.code_hints` to target local edits. Hints use relative
   paths and line numbers; prefer the files listed in `recommended_actions[].target_files`.
   The CLI intentionally does not read real `.env` contents.
5. To understand a specific failure, run
   `unipost doctor explain --request-id <id> --json` or
   `unipost logs list --status error --since 2h --json`.
6. When patching configuration files, prefer this order:
   1. Patch application code.
   2. Patch `.env.example`.
   3. Suggest changes to `.env`.
   4. Modify a real `.env` only after explicit user approval.
7. After every patch, run `unipost doctor verify --json`. A `data.status` of
   `passed` means the fix worked.
8. If verification still fails after a few bounded attempts, run
   `unipost doctor support-bundle --json`. It writes a redacted
   `unipost-debug-report.md`. If the user explicitly approves sending it to
   UniPost support, run `unipost doctor support-bundle --upload --json` and
   share only the returned bundle id. Do not paste secrets anywhere.

## Hard safety rules

- Never print full API keys, OAuth access/refresh tokens, cookies, webhook
  secrets, platform app secrets, or full `Authorization` headers. The CLI
  already masks keys as `up_live_9BWr...vhzHk`; keep them masked.
- Never run a live publish as verification. `unipost doctor verify` is
  non-destructive by default. Only the user may opt into
  `--allow-live-publish`, and only when they explicitly ask.
- Never disconnect/delete accounts, rotate webhook secrets, or revoke API keys
  as part of debugging.
- Upload a support bundle only after explicit user approval. The upload is
  redacted and workspace-scoped, but it is still support escalation.

## Reference

- Common root causes and fixes: `common-errors.md` (next to this file).
- Docs: https://unipost.dev/docs/cli/agent-debug
