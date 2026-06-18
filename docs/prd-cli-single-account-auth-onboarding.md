# PRD - CLI Single-Account Auth And Onboarding Cleanup

**Status:** Draft
**Owner:** Developer Experience / CLI
**Created:** 2026-06-18
**Target:** Make UniPost CLI auth obvious, single-account, and agent-friendly

---

## Problem

UniPost CLI currently has two auth entry points that look equivalent in `--help`:

```bash
unipost auth login --setup-token <token> [--client <client>] [--json]
unipost auth login --api-key <key> [--json]
```

They are not equivalent.

- `--setup-token` is a Dashboard-driven bootstrap flow. Users only get a setup token after clicking a Dashboard action, and the token is useful only for that first local CLI setup.
- `--api-key` validates an existing API key and records redacted metadata only in `0.2.0`; it does not store the secret in keychain, so follow-up commands still fail unless `UNIPOST_API_KEY` is set.
- `unipost init` does not currently provide a smooth "I already have an API key" path.
- Users who want to replace an old account/workspace binding do not have an obvious single-account rebind workflow.

This creates avoidable confusion:

- Users see "Logged in" after `auth login --api-key` but then `doctor` says no API key is found.
- Setup-token appears in top-level help even though users cannot obtain one without Dashboard context.
- AI agents receive unclear instructions and may ask users to paste production API keys into chat or shell history.
- A user whose old account was deleted needs a clear way to unlink and bind a new account.

## Product Direction

UniPost CLI should use a **single-account local binding model**:

- One local UniPost workspace/account binding at a time.
- `auth login` replaces the existing binding after confirmation.
- `auth logout` fully unbinds the current local credential.
- No multi-account switcher, no account list, and no hidden active-account ambiguity.

The primary human path should be:

```bash
unipost init
```

The primary non-interactive path should be:

```bash
unipost auth login --api-key <key> --json
```

The Dashboard setup-token path should still exist, but it should be presented as a Dashboard-generated onboarding command, not as a normal command users are expected to invent.

## Goals

1. Let a user who already has a UniPost API key configure or reconfigure CLI in one obvious flow.
2. Make `auth login --api-key` keychain-backed by default so follow-up commands work immediately.
3. Keep setup-token support for Dashboard copy-paste onboarding, but remove it from the top-level help's primary command list.
4. Support replacing an old account/workspace binding with a new one without multi-account complexity.
5. Make `doctor`, `init`, and `auth status` explain the exact fix when local auth is missing or metadata-only.
6. Reduce API key exposure in AI-agent workflows by steering first-time agent setup through Dashboard setup tokens where available.
7. Preserve non-interactive automation support with stable JSON output and explicit confirmation flags.
8. Update docs, help text, and skills so users and agents share the same mental model.

## Non-goals

- No multi-account support.
- No `auth list` / `auth use` account switcher as a promoted workflow.
- No browser OAuth/device-code login in this PRD.
- No Dashboard UI implementation details beyond the CLI contract it should display.
- No change to server API-key permission semantics.
- No change to logs/workspace authorization scope.
- No migration that attempts to recover previously discarded API-key plaintext from metadata-only records.

## Current Behavior

### `auth login --setup-token`

Current intended behavior:

- Exchanges a Dashboard-generated setup token for a named CLI API key.
- Stores the returned API key in OS keychain.
- Stores redacted credential metadata in local config.
- Follow-up authenticated commands work without `UNIPOST_API_KEY`.

This is the better onboarding behavior, but users only know the token when Dashboard gives them a command to copy.

### `auth login --api-key`

Current `0.2.0` behavior:

- Validates the API key against the API.
- Records redacted metadata only.
- Does not store the API key in OS keychain.
- Prints a message telling the user to set `UNIPOST_API_KEY`.
- Follow-up commands such as `doctor` fail unless the user exports `UNIPOST_API_KEY` or passes `--api-key`.

This is technically safe but product-confusing because the command says "Logged in" while the CLI cannot authenticate later.

### `init`

Current behavior when no API key is available:

- Reports that no UniPost API key was found.
- Tells the user to generate a setup token from the dashboard.

It does not give an obvious path for the common case "I already have an API key and want to configure this machine."

## Target UX

### First-time setup with an existing API key

Command:

```bash
unipost init
```

Expected interactive flow:

1. CLI detects no active keychain credential.
2. CLI asks how the user wants to connect:
   - Paste an API key.
   - Paste a Dashboard setup command/token.
   - Open Dashboard setup instructions.
3. If the user pastes an API key:
   - Validate key against the selected/default base URL.
   - Store the key in OS keychain.
   - Store only redacted metadata in config.
   - Print workspace name/id and next commands.
4. `unipost doctor --json` works immediately afterward.

Expected non-interactive equivalent:

```bash
unipost auth login --api-key <key> --yes --json
```

### First-time setup from Dashboard

Dashboard should show a copyable command like:

```bash
unipost auth login --setup-token ust_... --client codex
```

Expected behavior:

- The user or agent copies the command exactly.
- CLI exchanges the setup token for a named CLI API key.
- CLI stores the returned key in OS keychain.
- CLI prints success and suggests `unipost doctor --json`.

The user should not need to discover or type `--setup-token` from top-level help.

### Reconfigure when already bound

Command:

```bash
unipost init
```

If a keychain credential exists, show:

```text
UniPost CLI is currently linked to:
Workspace: Xiaobo Yu's Workspace (51fdbeec-df42-4d66-918f-a0cf7cc44b26)
API key: Claude Code CLI (up_live_3S5h...)
Base URL: https://api.unipost.dev

What would you like to do?
1. Keep current binding
2. Replace with a new API key
3. Replace with a Dashboard setup token
4. Log out
```

If the user chooses replace:

- Validate the new credential first.
- If validation succeeds, delete the old keychain item and write the new one.
- Update config metadata to the new workspace/key.
- Print clear success.

### Rebind after old account deletion

Recommended user flow:

```bash
unipost auth logout
unipost init
```

Or one-step replacement:

```bash
unipost auth login --api-key <new_key>
```

If an old binding exists, `auth login --api-key` should prompt:

```text
UniPost CLI is currently linked to workspace <old_workspace>.
Logging in with this API key will replace the local binding.
Continue? [y/N]
```

With `--yes`, non-interactive replacement is allowed:

```bash
unipost auth login --api-key <new_key> --yes --json
```

### Metadata-only mode

The old metadata-only behavior should be retained only as an explicit advanced mode:

```bash
unipost auth login --api-key <key> --metadata-only
```

This mode must not say "Logged in" without qualification. It should say:

```text
Recorded API key metadata only. Authenticated commands still require UNIPOST_API_KEY or --api-key.
```

JSON output must include:

```json
{
  "data": {
    "storage": "metadata_only",
    "authenticated_commands_ready": false
  }
}
```

## Command Semantics

### `unipost init`

Role:

- Main user-facing setup and reconfiguration wizard.
- Safe to recommend in docs and support replies.

Behavior:

- Reads current config and keychain state.
- Validates active credential if present.
- Offers replacement when a credential already exists.
- Supports API-key and setup-token input.
- Stores secrets only in OS keychain.
- Never prints full API keys.
- Does not create social posts or mutate workspace data beyond optional CLI API-key creation through setup-token exchange.

### `unipost auth login --api-key <key>`

Role:

- Direct credential binding for users who already have an API key.
- Recommended for CI-like non-interactive setup only when the key is already available in a secure context.

Target default behavior:

- Validate key.
- Store full key in OS keychain.
- Store redacted metadata in config.
- Replace any existing binding after confirmation.
- Follow-up commands work without environment variables.

Flags:

- `--yes`: allow replacement without interactive confirmation.
- `--metadata-only`: keep old no-secret-storage behavior.
- `--json`: stable envelope.
- `--base-url`: explicit environment override.

### `unipost auth login --setup-token <token> --client <client>`

Role:

- Dashboard-generated bootstrap command.
- Optimized for onboarding AI agents and first-time local setup without exposing long-lived API keys.

Target behavior:

- Keep command support.
- Keep JSON support.
- Keep `--client` because Dashboard can generate client-specific named keys and audit metadata.
- Move out of the top-level help's primary command list.
- Document as "Dashboard setup flow" in detailed auth help and docs.

### `unipost auth logout`

Role:

- Fully unbind the one local account.

Target behavior:

- Delete current keychain secret if present.
- Remove credential metadata from config.
- Preserve unrelated config such as display preferences only if currently supported and safe.
- After logout, `doctor` should clearly report no API key found and suggest `unipost init`.

### `unipost auth status`

Role:

- Explain whether authenticated commands are ready.

Target states:

| State | Meaning | Suggested action |
| --- | --- | --- |
| `keychain_ready` | Secret exists and validates | None |
| `metadata_only` | Metadata exists, no stored secret | Run `unipost auth login --api-key <key>` or set `UNIPOST_API_KEY` |
| `missing` | No local auth | Run `unipost init` |
| `invalid` | Stored key fails validation | Run `unipost auth logout` then `unipost init`, or replace with `auth login --api-key <key>` |
| `keychain_unavailable` | OS keychain cannot be used | Use `UNIPOST_API_KEY` or pass `--api-key` |

## Help And Documentation Changes

### Top-level `unipost --help`

The primary command list should show the common path:

```bash
unipost init [--json]
unipost auth login --api-key <key> [--json]
unipost auth logout [--json]
unipost auth status [--json]
```

Move setup-token to a separate lower section:

```bash
Dashboard setup flow:
  unipost auth login --setup-token <token> --client <client> [--json]
```

The section should say:

```text
Setup tokens are generated by the UniPost Dashboard. You normally copy the full command from Dashboard instead of typing it manually.
```

### `unipost auth login --help`

Detailed auth help should explain both flows:

- Use `--api-key` when you already have a UniPost API key and want this machine to remember it.
- Use `--setup-token` only when the Dashboard gives you a setup token or a full setup command.
- Use `--metadata-only` only when you intentionally do not want to store a secret locally.

### Public docs

Docs should recommend:

```bash
unipost init
```

for most local setup.

Docs should show:

```bash
unipost auth login --api-key <key>
```

for users who already created a key.

Docs should mention setup-token only in:

- Dashboard setup flow.
- AI-assisted debugging / agent setup.
- Troubleshooting local CLI auth.

### Agent skills

Codex and Claude Code skills should say:

1. First run `unipost auth status --json`.
2. If auth is missing, ask the user to run `unipost init` or paste the Dashboard-generated setup command.
3. Do not ask the user to paste a production API key into chat unless they explicitly choose the API-key path.
4. If `metadata_only`, explain that authenticated commands are not ready and recommend rebinding.

## Config And Storage Model

### Local config

`~/.unipost/config.json` should contain only non-secret metadata:

- `base_url`
- `default_workspace_id`
- current credential metadata:
  - workspace id/name
  - API key id/name if known
  - key prefix/fingerprint
  - keychain service/account
  - storage mode
  - authenticated_at

### OS keychain

The full API key should be stored only in OS keychain for keychain-backed modes:

- setup-token exchange
- api-key login default
- interactive `init` API-key entry

If keychain is unavailable:

- Interactive flows must explain that secure local storage is unavailable.
- CLI may offer `UNIPOST_API_KEY` fallback.
- CLI must not silently write plaintext API keys to config.

### Base URL

Keep the existing precedence:

```text
--base-url > UNIPOST_BASE_URL > config base_url > default production API
```

`init` and `auth login` should show the target base URL before validating a credential.

For production live keys, warn if the base URL is dev/staging. For test/dev keys, warn if the base URL is production.

## Error Handling

### `doctor`

When no API key is found:

```text
No API key found.
Run `unipost init` to configure this machine, or set UNIPOST_API_KEY for one-off commands.
```

When metadata-only credential exists:

```text
This machine has API key metadata but no stored secret.
Run `unipost auth login --api-key <key>` to store it in keychain, or set UNIPOST_API_KEY for one-off commands.
```

### `init`

When user chooses replacement but new credential fails validation:

- Keep the old binding unchanged.
- Show the normalized API error.
- Include request id.

### `auth login --api-key`

When existing binding differs from the new key's workspace:

- Require confirmation unless `--yes`.
- Show old workspace and new workspace.
- Do not delete the old keychain item until the new key validates.

### `auth logout`

When keychain deletion fails:

- Report partial logout.
- Remove config metadata only if keychain state is known safe, or show exact recovery instructions.

## Security And Privacy

- Never print a full API key after input.
- Avoid encouraging users to paste production keys into AI chat.
- Prefer Dashboard setup-token flow for agent onboarding because it can create a named, revocable CLI key without exposing a long-lived secret.
- API-key login may accept the key from terminal input; docs should warn that shell history can capture inline command arguments.
- Interactive `init` should prefer hidden input for pasted API keys.
- JSON output must redact key material.
- Support bundles must continue to redact API keys and authorization headers.

## Backward Compatibility

Existing metadata-only configs from `0.2.0` should not break.

On next `auth status`, `doctor`, or `init`, CLI should detect:

```json
"storage": "metadata_only"
```

and report that authenticated commands are not ready.

Users can repair by running:

```bash
unipost auth login --api-key <key>
```

No automatic migration can restore the key because the previous flow intentionally did not store it.

If existing `auth list` / `auth use` commands are present, stop promoting them. Options:

1. Keep as compatibility aliases but mark hidden/deprecated in help.
2. Make them operate on the single current credential only and return a deprecation warning.
3. Remove in a future major CLI version.

Preferred V1: keep compatibility, hide from top-level help, and add deprecation warnings.

## API / Backend Requirements

No new backend endpoint is required for the API-key path.

Setup-token flow continues to require existing Dashboard-issued token exchange behavior:

- Token is short-lived.
- Token can create or return a named CLI API key once.
- CLI stores returned key in OS keychain.

Dashboard should generate commands with explicit client metadata:

```bash
unipost auth login --setup-token ust_... --client codex
```

## Telemetry And Audit

CLI telemetry should remain default-off unless product policy changes elsewhere.

If telemetry is enabled, capture only non-secret auth flow events:

- `init_started`
- `auth_login_api_key_started`
- `auth_login_api_key_success`
- `auth_login_setup_token_success`
- `auth_rebind_confirmed`
- `auth_logout_success`
- failure code categories

Never capture key value, full token, workspace secret fields, or request/response payloads.

Backend audit/logging for setup-token-created CLI keys should include:

- key name
- client
- source `cli_setup_token`
- workspace id
- creator user id

## Acceptance Criteria

1. A user with an existing API key can run `unipost auth login --api-key <key>` and then `unipost doctor --json` without setting `UNIPOST_API_KEY`.
2. A user can run `unipost init`, paste an API key, and end with a keychain-backed local binding.
3. If an old binding exists, `init` and `auth login --api-key` clearly ask before replacing it.
4. Replacing a binding validates the new key before deleting the old keychain secret.
5. `auth logout` removes the single local binding; `auth status` then reports missing auth and recommends `unipost init`.
6. `--metadata-only` preserves the old no-secret behavior and says authenticated commands are not ready.
7. `unipost --help` no longer presents setup-token as a normal first-class login option.
8. `unipost auth login --help` explains setup-token as Dashboard-generated.
9. Public docs recommend `unipost init` for local setup and mention setup-token only in Dashboard/agent setup contexts.
10. Codex and Claude Code skills tell agents to prefer `auth status`, `init`, or Dashboard setup commands before asking users for long-lived API keys.
11. Existing metadata-only configs produce actionable repair messages rather than ambiguous "logged in" state.
12. Tests cover keychain-backed API-key login, replacement confirmation, metadata-only mode, help text, init reconfiguration, and logout.

## Test Plan

### CLI unit tests

- `auth login --api-key` stores a keychain-backed credential by default.
- `auth login --api-key --metadata-only` does not store the secret and reports `authenticated_commands_ready: false`.
- Existing binding replacement requires confirmation.
- `--yes` allows non-interactive replacement.
- Replacement does not delete the old keychain item when new validation fails.
- `auth logout` deletes keychain item and config metadata.
- `auth status` distinguishes `keychain_ready`, `metadata_only`, `missing`, `invalid`, and `keychain_unavailable`.
- `doctor` emits actionable findings for missing and metadata-only auth.
- `init` can complete first-time API-key setup.
- `init` can replace an existing binding.
- Top-level help hides setup-token from the primary command list.
- Auth help documents setup-token as Dashboard-generated.

### Integration smoke tests

- Fresh temporary HOME, API-key login, `doctor --json` succeeds.
- Fresh temporary HOME, metadata-only login, `doctor --json` fails with repair hint.
- Existing binding to workspace A, login with workspace B key and confirmation, `config show` reflects workspace B.
- Base URL mismatch warning appears for live key against dev/staging URL.

### Documentation checks

- CLI reference page matches generated help.
- AI-assisted debugging page does not instruct users to paste production keys into chat as the default agent path.
- README and public docs say `unipost init` is the normal setup command.

## Rollout Plan

### Phase 1: CLI auth behavior

- Make `auth login --api-key` keychain-backed by default.
- Add `--metadata-only`.
- Add replacement confirmation and `--yes`.
- Improve `auth status`, `doctor`, and `logout` messages.

### Phase 2: `init` reconfiguration wizard

- Add first-time API-key setup.
- Add existing-binding detection.
- Add replace/logout choices.
- Keep setup-token input available but framed as Dashboard-provided.

### Phase 3: Help/docs/skills cleanup

- Update top-level help.
- Update `auth login --help`.
- Update CLI docs, README, AI-assisted debugging docs, Codex skill, and Claude Code skill.
- Hide or de-emphasize multi-account commands.

### Phase 4: Dashboard alignment

- Ensure Dashboard setup flow copies the full setup-token command.
- Label setup-token as short-lived and Dashboard-generated.
- Prefer client-specific command snippets for Codex and Claude Code.

## Open Questions

1. Should inline `auth login --api-key <key>` remain supported, or should docs prefer hidden interactive input because shell history can capture the key?
2. Should `init` create a named API key through Dashboard setup-token only, while API-key login uses the already-created key as-is?
3. Should `auth list` and `auth use` remain hidden compatibility commands or be removed in the next CLI minor version?
4. Should `auth logout` preserve `base_url`, or should it reset to production default to avoid stale dev/staging bindings?
5. Should replacement confirmation be required when the workspace is the same but the API key id differs?
6. Should setup-token commands expire after first successful exchange or after a short time window even if unused?
