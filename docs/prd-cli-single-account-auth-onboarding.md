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
- The current secure-store implementation is macOS-only. It shells out to `/usr/bin/security`; Linux and Windows currently report keychain unavailable.
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
- No promoted multi-account switcher, no account-list workflow, and no hidden active-account ambiguity.

The primary human path should be:

```bash
unipost init
```

The primary non-interactive path should be:

```bash
unipost auth login --api-key <key> --json
```

This path is keychain-backed on macOS in V1. On Linux and Windows, V1 must not silently pretend that local authenticated commands are ready. It should fail with a clear `KEYCHAIN_UNAVAILABLE` error unless the user explicitly chooses `--metadata-only`, passes `--api-key` per command, or uses `UNIPOST_API_KEY`.

The Dashboard setup-token path should still exist, but it should be presented as a Dashboard-generated onboarding command, not as a normal command users are expected to invent.

## Product Decisions

1. **Secret persistence is macOS-only in V1.** The CLI will use the current macOS keychain implementation for locally remembered secrets. Linux libsecret and Windows Credential Manager support are deferred to a future cross-platform storage phase.
2. **No silent metadata-only fallback.** When keychain-backed login is requested on a platform without secure local storage, the command should return `KEYCHAIN_UNAVAILABLE` with actionable alternatives. Users can choose `--metadata-only` explicitly, export `UNIPOST_API_KEY`, or pass `--api-key` for one-off commands.
3. **Hidden input is the default human path.** Docs and `init` should prefer hidden interactive key entry. Inline `auth login --api-key <key>` remains supported for CI and controlled non-interactive contexts.
4. **`auth list` and `auth use` stay compatibility-only.** These commands already exist but do not provide true multi-account support. V1 should hide or de-emphasize them, make them operate only on the single current binding, and add deprecation messaging.
5. **`auth logout` resets environment binding state.** Logout should remove credential metadata and remove stored `base_url` so the CLI returns to the production default unless `UNIPOST_BASE_URL` or `--base-url` is supplied.

## Goals

1. Let a user who already has a UniPost API key configure or reconfigure CLI in one obvious flow.
2. Make `auth login --api-key` keychain-backed by default on macOS so follow-up commands work immediately, and make non-macOS secure-storage limitations explicit.
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
- No Linux libsecret or Windows Credential Manager implementation in V1.

## Current Behavior

### Secure store

Current behavior:

- macOS uses `/usr/bin/security` to read/write the login keychain.
- Non-macOS platforms use a stub secure store that throws a keychain-unavailable error.
- The CLI therefore cannot currently persist API-key secrets on Linux, Windows, or most CI runners.

This platform constraint must be visible in user-facing behavior. A command must not say authenticated local commands are ready when the platform cannot store the secret.

### `auth login --setup-token`

Current intended behavior:

- Exchanges a Dashboard-generated setup token for a named CLI API key.
- Stores the returned API key in macOS keychain when secure local storage is available.
- Stores redacted credential metadata in local config.
- Follow-up authenticated commands work without `UNIPOST_API_KEY`.
- If a valid local binding already exists, returns `already_configured` and does not consume the setup token unless the user explicitly requests replacement with `--replace-key`, `--reauth`, or `--yes`.

This is the better onboarding behavior, but users only know the token when Dashboard gives them a command to copy.

### `auth login --api-key`

Current `0.2.0` behavior:

- Validates the API key against the API.
- Records redacted metadata only.
- Does not store the API key in any secure local store.
- Prints a message telling the user to set `UNIPOST_API_KEY`.
- Follow-up commands such as `doctor` fail unless the user exports `UNIPOST_API_KEY` or passes `--api-key`.

This is technically safe but product-confusing because the command says "Logged in" while the CLI cannot authenticate later.

### `init`

Current behavior when no API key is available:

- Reports that no UniPost API key was found.
- Tells the user to generate a setup token from the dashboard.

It does not give an obvious path for the common case "I already have an API key and want to configure this machine."

### `auth status`

Current `0.2.0` behavior:

- Calls the authenticated workspace check before classifying local auth state.
- Fails when no API key is available.
- Cannot currently report `missing`, `metadata_only`, or `keychain_unavailable` as first-class local states.

The target design requires `auth status` to inspect local config and secure-store availability before making any authenticated API call.

### `auth list` and `auth use`

Current behavior:

- Both commands already exist.
- They do not implement true multi-account switching.
- `auth list` reports at most the current active credential.
- `auth use` sets the default workspace id and should not be presented as account switching.

V1 should keep these as compatibility commands only, hide them from prominent help, and add deprecation or clarification text.

## Target UX

### First-time setup with an existing API key on macOS

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
   - Store the key in macOS keychain.
   - Store only redacted metadata in config.
   - Print workspace name/id and next commands.
4. `unipost doctor --json` works immediately afterward.

Expected non-interactive equivalent:

```bash
unipost auth login --api-key <key> --yes --json
```

### First-time setup with an existing API key on Linux/Windows

Command:

```bash
unipost init
```

Expected interactive flow:

1. CLI detects that secure local secret storage is unavailable.
2. CLI still lets the user validate an API key against the selected/default base URL.
3. CLI does not write the full key to local config.
4. CLI explains the available choices:
   - Set `UNIPOST_API_KEY` in the current shell or local environment.
   - Pass `--api-key` for one-off commands.
   - Record redacted metadata only with an explicit metadata-only choice.
5. CLI reports authenticated local commands as not ready unless a usable key is available from env or command flags.

Non-interactive behavior:

```bash
unipost auth login --api-key <key> --json
```

On non-macOS V1, this should fail with `KEYCHAIN_UNAVAILABLE` unless `--metadata-only` is present. The error should include exact recovery commands.

### First-time setup from Dashboard

Dashboard should show a copyable command like:

```bash
unipost auth login --setup-token ust_... --client codex
```

Expected behavior:

- The user or agent copies the command exactly.
- CLI exchanges the setup token for a named CLI API key.
- CLI stores the returned key in macOS keychain when secure storage is available.
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
- Stores secrets only in the configured secure store. In V1 this means macOS keychain only.
- On non-macOS, explains secure storage is unavailable and offers env-var, one-off `--api-key`, or explicit metadata-only paths.
- Never prints full API keys.
- Does not create social posts or mutate workspace data beyond optional CLI API-key creation through setup-token exchange.

### `unipost auth login --api-key <key>`

Role:

- Direct credential binding for users who already have an API key.
- Recommended for CI-like non-interactive setup only when the key is already available in a secure context.

Target default behavior:

- Validate key.
- Store full key in macOS keychain when available.
- Store redacted metadata in config.
- Replace any existing binding after confirmation.
- Follow-up commands work without environment variables.
- On non-macOS V1, fail with `KEYCHAIN_UNAVAILABLE` unless `--metadata-only` is present. Do not silently fall back to metadata-only.

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
- On non-macOS V1, preflight secure-store availability before exchanging the token. If secure storage is unavailable, return `KEYCHAIN_UNAVAILABLE` and do not create a new API key that the CLI would immediately discard.
- If an existing local keychain binding validates successfully, return `already_configured` and leave the setup token unused unless replacement was explicitly requested.

### `unipost auth logout`

Role:

- Fully unbind the one local account.

Target behavior:

- Delete current keychain secret if present.
- Remove credential metadata from config.
- Remove stored `base_url` so the CLI falls back to the production default unless `UNIPOST_BASE_URL` or `--base-url` is set.
- Preserve unrelated config such as display preferences only if currently supported and safe.
- After logout, `doctor` should clearly report no API key found and suggest `unipost init`.

### `unipost auth status`

Role:

- Explain whether authenticated commands are ready.

Required behavior:

- Must run without an API key.
- Must classify local config, metadata, and secure-store availability before attempting any API request.
- Should only call `/v1/workspace` when it has a usable secret from keychain, `UNIPOST_API_KEY`, or `--api-key`.
- Should return a stable JSON state even when auth is missing or the keychain is unavailable.

Target states:

| State | Meaning | Suggested action |
| --- | --- | --- |
| `keychain_ready` | Secret exists and validates | None |
| `metadata_only` | Metadata exists, no stored secret | Run `unipost auth login --api-key <key>` or set `UNIPOST_API_KEY` |
| `missing` | No local auth | Run `unipost init` |
| `invalid` | Stored key fails validation | Run `unipost auth logout` then `unipost init`, or replace with `auth login --api-key <key>` |
| `keychain_unavailable` | Secure local store cannot be used | Use `UNIPOST_API_KEY` or pass `--api-key` |

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

for users who already created a key and are running on macOS, or for CI-style contexts where the key is already available and the user understands the local storage behavior.

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

### Secure local store

The full API key should be stored only in a secure OS credential store for keychain-backed modes. In V1, the only supported secure local store is macOS keychain via `/usr/bin/security`.

- setup-token exchange
- api-key login default
- interactive `init` API-key entry

If keychain is unavailable on Linux, Windows, or CI:

- Interactive flows must explain that secure local storage is unavailable.
- CLI may offer `UNIPOST_API_KEY` fallback.
- CLI may offer explicit `--metadata-only` fallback.
- Non-interactive keychain-backed login must fail with `KEYCHAIN_UNAVAILABLE` instead of silently recording metadata-only state.
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

When secure local storage is unavailable:

```text
Secure local credential storage is not available on this platform.
Use UNIPOST_API_KEY for authenticated commands, pass --api-key for one-off commands, or rerun with --metadata-only to record redacted metadata only.
```

JSON output should include:

```json
{
  "ok": false,
  "error": {
    "code": "KEYCHAIN_UNAVAILABLE",
    "message": "Secure local credential storage is not available on this platform."
  }
}
```

### `auth logout`

When keychain deletion fails:

- Report partial logout.
- Remove config metadata only if keychain state is known safe, or show exact recovery instructions.

## Security And Privacy

- Never print a full API key after input.
- Avoid encouraging users to paste production keys into AI chat.
- Prefer Dashboard setup-token flow for agent onboarding because it can create a named, revocable CLI key without exposing a long-lived secret.
- API-key login may accept the key from terminal input; docs should warn that shell history can capture inline command arguments.
- Setup-token commands also appear in shell history and process arguments when pasted inline. This is mitigated by short expiry, one-time exchange, revocability, and generated client-specific keys, but docs should still avoid presenting inline tokens as a generic manual command.
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

`auth list` and `auth use` already exist, but they should not be promoted as multi-account tools.

V1 behavior:

- Keep them as compatibility commands.
- Hide them from top-level help and docs.
- Make `auth list` report only the single current binding.
- Make `auth use` operate only on the current workspace metadata and return a clarification or deprecation warning.
- Consider removal only in a future major CLI version.

## API / Backend Requirements

No new backend endpoint is required for the API-key path.

Setup-token flow continues to require existing Dashboard-issued token exchange behavior:

- Token is short-lived.
- Token can create or return a named CLI API key once.
- CLI stores returned key in macOS keychain when secure storage is available.
- CLI does not exchange the token when a valid local binding is already present, so users can safely paste a Dashboard command twice without creating duplicate CLI keys.

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

1. On macOS, a user with an existing API key can run `unipost auth login --api-key <key>` and then `unipost doctor --json` without setting `UNIPOST_API_KEY`.
2. On macOS, a user can run `unipost init`, paste an API key through hidden input, and end with a keychain-backed local binding.
3. If an old binding exists, `init` and `auth login --api-key` clearly ask before replacing it.
4. Replacing a binding validates the new key before deleting the old keychain secret.
5. `auth logout` removes the single local binding, removes stored `base_url`, and `auth status` then reports missing auth and recommends `unipost init`.
6. `--metadata-only` preserves the old no-secret behavior and says authenticated commands are not ready.
7. On non-macOS, `auth login --api-key <key>` returns `KEYCHAIN_UNAVAILABLE` unless `--metadata-only` is passed, and it gives env-var and one-off alternatives.
8. `auth status --json` runs without an API key and reports `missing`, `metadata_only`, `keychain_ready`, `invalid`, or `keychain_unavailable`.
9. `unipost --help` no longer presents setup-token as a normal first-class login option.
10. `unipost auth login --help` explains setup-token as Dashboard-generated and short-lived.
11. Public docs recommend `unipost init` for local setup and mention setup-token only in Dashboard/agent setup contexts.
12. Codex and Claude Code skills tell agents to prefer `auth status`, `init`, or Dashboard setup commands before asking users for long-lived API keys.
13. Existing metadata-only configs produce actionable repair messages rather than ambiguous "logged in" state.
14. `auth list` and `auth use` are hidden/de-emphasized compatibility commands and do not imply multi-account switching.
15. Tests cover macOS keychain-backed API-key login, non-macOS keychain-unavailable behavior, replacement confirmation, metadata-only mode, help text, init reconfiguration, status without a key, and logout.

## Test Plan

### CLI unit tests

- On macOS or with a fake available secure store, `auth login --api-key` stores a keychain-backed credential by default.
- With a fake unavailable secure store, `auth login --api-key` fails with `KEYCHAIN_UNAVAILABLE` unless `--metadata-only` is passed.
- `auth login --api-key --metadata-only` does not store the secret and reports `authenticated_commands_ready: false`.
- Existing binding replacement requires confirmation.
- `--yes` allows non-interactive replacement.
- Replacement does not delete the old keychain item when new validation fails.
- `auth logout` deletes keychain item, config metadata, and stored `base_url`.
- `auth status` runs without a key and distinguishes `keychain_ready`, `metadata_only`, `missing`, `invalid`, and `keychain_unavailable`.
- `doctor` emits actionable findings for missing and metadata-only auth.
- `init` can complete first-time API-key setup.
- `init` uses hidden input for pasted API keys.
- `init` can replace an existing binding.
- Top-level help hides setup-token from the primary command list.
- Auth help documents setup-token as Dashboard-generated.
- `auth list` and `auth use` remain compatibility commands and do not expose multi-account semantics.

### Integration smoke tests

- Fresh temporary HOME on macOS or fake secure-store environment, API-key login, `doctor --json` succeeds.
- Fresh temporary HOME, metadata-only login, `doctor --json` fails with repair hint.
- Fresh temporary HOME with unavailable secure store, API-key login returns `KEYCHAIN_UNAVAILABLE` and leaves no plaintext key in config.
- Existing binding to workspace A, login with workspace B key and confirmation, `config show` reflects workspace B.
- Base URL mismatch warning appears for live key against dev/staging URL.

### Documentation checks

- CLI reference page matches generated help.
- AI-assisted debugging page does not instruct users to paste production keys into chat as the default agent path.
- README and public docs say `unipost init` is the normal setup command.

## Rollout Plan

### Phase 1: CLI auth behavior

- Make `auth login --api-key` keychain-backed by default on macOS.
- Add explicit `KEYCHAIN_UNAVAILABLE` behavior for Linux, Windows, and CI environments without secure storage.
- Add `--metadata-only`.
- Add replacement confirmation and `--yes`.
- Make `auth status` classify local state without requiring an API key.
- Improve `doctor` and `logout` messages.
- Make `auth logout` remove stored `base_url`.
- Keep `auth list` and `auth use` as hidden/de-emphasized compatibility commands.

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

### Future Phase: Cross-platform secure storage

- Evaluate Linux libsecret support.
- Evaluate Windows Credential Manager support.
- Preserve the V1 no-plaintext-config invariant.
- Update acceptance criteria so `auth login --api-key` can be keychain-backed by default beyond macOS only when those secure stores exist.

## Open Questions

1. Should `init` create a named API key through Dashboard setup-token only, while API-key login uses the already-created key as-is?
2. Should replacement confirmation be required when the workspace is the same but the API key id differs?
3. Should setup-token commands expire after first successful exchange or after a short time window even if unused?
4. Should cross-platform secure storage move into the next CLI minor release, or remain deferred until macOS behavior has been validated in production?
