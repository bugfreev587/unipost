# PRD - Admin AI Provider API Keys

**Status:** Planning
**Owner:** Admin / Platform / AI
**Created:** 2026-06-07
**Target:** Admin-managed AI provider keys for OpenAI, Anthropic, and TokenGate

---

## Problem

UniPost currently uses server-side environment variables for AI providers:

- `OPENAI_API_KEY` and related model/base URL variables power AI Post Assist and Error Triage.
- `ANTHROPIC_API_KEY` powers App Review AI planner flows.
- There is no admin UI for seeing which AI provider is active, validating a new key, rotating keys safely, or moving AI traffic through TokenGate.

This makes provider changes operationally expensive. Rotating a key requires deploy-time environment edits, and admins cannot tell from the dashboard whether AI failures are caused by missing credentials, invalid keys, model mismatch, provider outage, or code defects.

## Product Direction

Add a super-admin-only admin page for AI provider credentials. The page lives in the admin left sidebar and becomes the operational source of truth for UniPost AI provider configuration.

The first supported providers are:

1. TokenGate
2. OpenAI
3. Anthropic

The backend should prefer an enabled admin-managed provider routed to a surface when present, then fall back to existing environment variables. This lets UniPost add TokenGate without breaking the existing OpenAI and Anthropic deployments.

## Goals

1. Add a new admin sidebar entry under `System`:

```text
AI Keys -> /admin/ai-keys
```

2. Let super admins view AI provider status without exposing full secrets.
3. Let super admins create, update, test, route, unroute, disable, and rotate provider keys.
4. Support TokenGate as an OpenAI-compatible gateway for `/chat/completions`.
5. Support TokenGate as an Anthropic-compatible gateway for `/messages` when a calling surface needs Anthropic Messages format.
6. Keep existing `OPENAI_API_KEY` and `ANTHROPIC_API_KEY` as read-only environment fallbacks.
7. Centralize AI provider resolution so AI Post Assist, Error Triage, and App Review AI can move off scattered environment reads.
8. Persist audit events for every credential mutation and validation test.
9. Never return plaintext API keys to the browser after save.
10. Provide clear admin-facing errors for invalid keys, missing models, rate limits, and provider outages.

## Non-goals

- No customer BYOK in v1. This is an internal UniPost admin setting, not a workspace setting.
- No direct browser calls to TokenGate, OpenAI, Anthropic, or any other secret-bearing AI provider.
- No automatic creation or revocation of remote TokenGate dashboard keys in v1 unless TokenGate exposes a stable key-management API. V1 stores and validates keys that were created in the TokenGate dashboard.
- No exposure of prompts, social post drafts, diagnostic evidence, provider responses, or token usage to non-admin users.
- No immediate removal of existing environment variables.
- No production traffic migration without development validation.

## Current Codebase Findings

### UniPost admin

- Admin shell and sidebar live in `dashboard/src/app/admin/_components/admin-ui.tsx`.
- Current System entries include `Logs`, `Errors`, `Triage`, and `Settings`.
- `AdminShell` supports `requireSuperAdmin`, already used by `/admin/logs`.
- Admin APIs are mounted under `/v1/admin/*` in `api/cmd/api/main.go` and gated by `auth.AdminMiddleware`.
- Super-admin-only backend routes can use `auth.RequireSuperAdmin`.

### Existing AI usage

- `api/internal/handler/ai_post_assist.go` reads `OPENAI_API_KEY` and calls `https://api.openai.com/v1/chat/completions` directly.
- `api/internal/errortriage/ai_analyzer.go` reads `OPENAI_API_KEY`, prefers `OPENAI_ERROR_TRIAGE_MODEL` over `OPENAI_MODEL`, and supports optional `OPENAI_ERROR_TRIAGE_URL`.
- `api/internal/reviewai/anthropic.go` uses `ANTHROPIC_API_KEY` with Anthropic Messages.
- `api/cmd/api/main.go` wires App Review AI with `reviewai.NewAnthropicClient(os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("ANTHROPIC_MODEL"), "", nil)`.
- `api/cmd/api/main.go` constructs Error Triage and App Review AI clients at process startup. The implementation must replace these static AI clients with request-time provider resolution so admin routing changes and key rotation work without restarting the API process.

### Existing secret patterns

- `api/internal/crypto/aes.go` provides AES-256-GCM encryption using `ENCRYPTION_KEY`.
- `platform_credentials` currently stores workspace platform credentials, but this new feature should use a global admin table because AI provider keys affect platform runtime globally.
- `audit_log` exists and supports config/security events through `api/internal/audit`.

### CiteLoop reference

CiteLoop already implements the relevant pattern:

- Environment keys: `TOKENGATE_API_KEY`, `TOKENGATE_BASE_URL`, `TOKENGATE_MODEL`.
- TokenGate preferred when configured; Anthropic is fallback.
- OpenAI-compatible client posts to `{base_url}/chat/completions`.
- Admin credentials are global singleton settings, not project-scoped.
- Admin status returns provider, configured boolean, key tail, base URL, and updated time, but never returns the full key.
- UI lets the admin choose `tokengate`, `openai`, or `claude`, paste a password key, edit base URL for OpenAI-compatible providers, save, and refresh status.

UniPost should borrow the security and UX shape, but should not copy CiteLoop's old default TokenGate base URL. TokenGate's current docs list:

```text
https://gateway.mytokengate.com/v1
```

## TokenGate Findings

TokenGate's docs describe:

- OpenAI-compatible API:

```text
POST https://gateway.mytokengate.com/v1/chat/completions
Authorization: Bearer YOUR_API_KEY
```

- Anthropic-compatible API:

```text
POST https://gateway.mytokengate.com/v1/messages
Authorization: Bearer YOUR_API_KEY
```

- Model list endpoint:

```text
GET https://gateway.mytokengate.com/v1/models
Authorization: Bearer YOUR_API_KEY
```

- Common errors:

```text
400 invalid parameters
401 invalid or missing API key
403 insufficient permissions
429 rate limit exceeded
503/504 provider unavailable or overloaded
```

- Security guidance:
  - Never hardcode keys.
  - Use environment variables or server-side storage.
  - Rotate keys regularly, recommended every 90 days.
  - Use separate keys per environment.
  - Monitor key usage for anomalies.

Reference links:

- `https://docs.mytokengate.com/en/wiki/api-docs`
- `https://docs.mytokengate.com/en/wiki/api-docs/chat-completions`
- `https://docs.mytokengate.com/en/wiki/api-docs/messages`
- `https://docs.mytokengate.com/en/wiki/api-docs/models`
- `https://docs.mytokengate.com/en/wiki/faq/error-codes`
- `https://docs.mytokengate.com/en/wiki/security`

## User Experience

### Admin entry point

Add this left-nav item under `System`, between `Triage` and `Settings`:

```text
AI Keys -> /admin/ai-keys
```

Use a key-shaped or circuit/provider icon from `lucide-react`. The page should require super admin access in the dashboard and backend.

### Page layout

The page title is `AI Keys`.

Primary sections:

- Provider status table
- Active routing policy
- Provider detail editor
- Validation and recent events

### Provider status table

Rows:

- TokenGate
- OpenAI
- Anthropic

Columns:

- Provider
- Status: `Active`, `Configured`, `Env fallback`, `Disabled`, `Validation failed`
- Key source: `Admin-managed`, `Environment`, or `Not configured`
- Key tail: last 4 characters only
- Base URL
- Default model
- Routed surfaces: `Post Assist`, `Error Triage`, `App Review`
- Last validated
- Last rotated
- Actions

Actions:

- `Configure`
- `Test`
- `Route surface`
- `Disable`
- `Rotate`

`Disable` only disables the UniPost stored provider config. It does not revoke the key in TokenGate/OpenAI/Anthropic.

### Provider detail editor

When configuring a provider, show a side panel or inline editor using the existing admin visual system:

- Provider selector: TokenGate, OpenAI, Anthropic
- API key password input
- Base URL input:
  - TokenGate default: `https://gateway.mytokengate.com/v1`
  - OpenAI default: `https://api.openai.com/v1`
  - Anthropic default: `https://api.anthropic.com/v1`
- Chat completions model input for OpenAI-compatible surfaces
- Messages model input for Anthropic-compatible surfaces
- Surface routing controls:
  - Post Assist
  - Error Triage
  - App Review AI
- Optional model override per routed surface
- `Test connection`
- `Save`
- `Cancel`

Key input behavior:

- Empty key keeps the existing stored key when editing the same provider.
- Changing provider or rotating a key requires a new key.
- After saving, clear the input and show only the key tail.
- Never prefill the key field.

Test behavior:

- If a new key is present in the editor, `Test connection` validates the unsaved candidate config and does not persist it.
- If the key field is empty for an already configured provider, `Test connection` validates the stored key.
- If the key field is empty for an unconfigured provider, `Test connection` returns `AI_PROVIDER_KEY_REQUIRED`.

### Routing policy

V1 should support one simple policy:

```text
Use the admin-managed routed provider for the surface when configured. Otherwise use environment fallback.
```

The page should display the effective provider for each AI surface:

- Post Assist: TokenGate/OpenAI-compatible chat completions preferred, environment OpenAI fallback.
- Error Triage: TokenGate/OpenAI-compatible chat completions preferred, deterministic analyzer fallback if no key.
- App Review AI: Anthropic Messages preferred. TokenGate Messages may be selected only if the provider is configured for Anthropic-compatible messages.

Future policy options such as provider failover chains and cost-based routing are out of scope for v1. Per-surface model selection is in scope because Error Triage already supports `OPENAI_ERROR_TRIAGE_MODEL` separately from the general `OPENAI_MODEL` fallback.

### Validation states

`Test connection` should run server-side validation and show:

- Success: provider reachable, auth accepted, model available.
- Auth failure: key invalid or missing.
- Model failure: key valid but model unavailable.
- Rate limit: provider returned 429.
- Provider failure: provider returned 503/504 or timed out.
- Config failure: invalid base URL or unsupported provider mode.

Do not log or render plaintext keys in validation output.

## Backend Requirements

### Provider registry

Add a shared backend package, for example:

```text
api/internal/aiproviders
```

Responsibilities:

- Load admin-managed provider configs.
- Decrypt keys only inside the backend process.
- Resolve the effective provider per AI surface.
- Resolve providers at request time. DB reads may use a short TTL cache or write-time invalidation, but routing changes, disable actions, and key rotation must take effect without an API process restart.
- Build OpenAI-compatible chat-completions clients.
- Build Anthropic-compatible messages clients.
- Run validation probes.
- Normalize provider errors into admin-safe error codes.

The package should expose small interfaces so existing AI surfaces do not know where keys came from:

```text
ChatCompletionsClient
MessagesClient
ProviderResolver
```

### Resolution order

For each AI request:

1. If an enabled admin-managed provider is routed to the requested surface, use it.
2. Else use environment fallback:
   - `OPENAI_API_KEY` for OpenAI-compatible chat-completions surfaces.
   - `ANTHROPIC_API_KEY` for Anthropic Messages surfaces.
3. Else use the existing deterministic/stub fallback if the surface already has one.
4. Else return a clear `AI_PROVIDER_NOT_CONFIGURED` error.

Model resolution is per surface:

1. Use `ai_surface_routing.model_override` when present.
2. Else use the provider default for the requested client kind:
   - `chat_model` for chat-completions surfaces.
   - `messages_model` for messages surfaces.
3. Else use the existing environment model fallback for that surface:
   - Post Assist: `OPENAI_MODEL`, then `gpt-4.1-mini`.
   - Error Triage: `OPENAI_ERROR_TRIAGE_MODEL`, then `OPENAI_MODEL`, then `gpt-4.1-mini`.
   - App Review AI: `ANTHROPIC_MODEL`, then the existing Anthropic default.

Run metadata must record the provider and model actually used. For Error Triage, `error_triage_runs.model` must be written from request-time resolution for that run, not from a startup-time analyzer snapshot.

### TokenGate integration

TokenGate config:

```text
provider = tokengate
base_url = https://gateway.mytokengate.com/v1
api_key = <stored encrypted>
chat_model = configurable, default selected by admin
messages_model = configurable, default selected by admin
```

OpenAI-compatible calls:

```text
POST {base_url}/chat/completions
Authorization: Bearer <api_key>
Content-Type: application/json
```

Anthropic-compatible calls:

```text
POST {base_url}/messages
Authorization: Bearer <api_key>
Content-Type: application/json
```

Native Anthropic calls use a different header scheme:

```text
POST https://api.anthropic.com/v1/messages
x-api-key: <api_key>
anthropic-version: 2023-06-01
Content-Type: application/json
```

The `MessagesClient` implementation must switch headers by provider. TokenGate Messages should use `Authorization: Bearer` as documented by TokenGate. Native Anthropic should keep `x-api-key` and `anthropic-version`. If TokenGate also accepts `anthropic-version`, sending it is optional and must not be required for native Anthropic compatibility.

Model list validation:

```text
GET {base_url}/models
Authorization: Bearer <api_key>
```

The implementation must trim trailing slashes from `base_url` before composing endpoint URLs.

### Admin API

Add super-admin-only endpoints:

```text
GET /v1/admin/ai-providers
PUT /v1/admin/ai-providers/{provider}
POST /v1/admin/ai-providers/{provider}/test
PUT /v1/admin/ai-provider-routing/{surface}
DELETE /v1/admin/ai-provider-routing/{surface}
POST /v1/admin/ai-providers/{provider}/disable
GET /v1/admin/ai-providers/events
```

Endpoint semantics:

- `PUT /v1/admin/ai-providers/{provider}` creates or updates provider key config.
- `POST /v1/admin/ai-providers/{provider}/test` validates either the submitted candidate config or the stored provider config.
- `PUT /v1/admin/ai-provider-routing/{surface}` upserts the single active provider for that surface and optional model override.
- `DELETE /v1/admin/ai-provider-routing/{surface}` removes admin-managed routing for that surface, returning it to environment fallback.
- `POST /v1/admin/ai-providers/{provider}/disable` disables the stored provider config and removes any routing rows pointing to it.
- `GET /v1/admin/ai-providers/events` is a paginated read-only audit view over `audit_log` where `resource_type = 'ai_provider_key'`.

Supported event query params:

```text
limit
cursor
provider
action
```

`GET /v1/admin/ai-providers` returns effective status and fallbacks:

```json
{
  "data": {
    "providers": [
      {
        "provider": "tokengate",
        "configured": true,
        "enabled": true,
        "source": "admin",
        "key_tail": "9abc",
        "base_url": "https://gateway.mytokengate.com/v1",
        "chat_model": "gpt-4o",
        "messages_model": "claude-sonnet-4-6",
        "last_validated_at": "2026-06-07T19:30:00Z",
        "last_validation_status": "ok",
        "last_rotated_at": "2026-06-07T19:30:00Z",
        "updated_at": "2026-06-07T19:30:00Z"
      }
    ],
    "effective": {
      "post_assist": {
        "provider": "tokengate",
        "source": "admin",
        "client_kind": "chat_completions",
        "model": "gpt-4o"
      },
      "error_triage": {
        "provider": "openai",
        "source": "env",
        "client_kind": "chat_completions",
        "model": "gpt-4.1-mini"
      },
      "app_review_ai": {
        "provider": "anthropic",
        "source": "env",
        "client_kind": "messages",
        "model": "claude-sonnet-4-20250514"
      }
    }
  }
}
```

`PUT /v1/admin/ai-providers/{provider}` accepts:

```json
{
  "api_key": "sk-...",
  "base_url": "https://gateway.mytokengate.com/v1",
  "chat_model": "gpt-4o",
  "messages_model": "claude-sonnet-4-6",
  "enabled": true
}
```

The response must never include `api_key`.

`PUT /v1/admin/ai-provider-routing/{surface}` accepts:

```json
{
  "provider": "tokengate",
  "client_kind": "chat_completions",
  "model_override": "gpt-4o"
}
```

Allowed surfaces:

```text
post_assist
error_triage
app_review_ai
```

Allowed client kinds:

```text
chat_completions
messages
```

Surface/client-kind compatibility:

- `post_assist`: `chat_completions`
- `error_triage`: `chat_completions`
- `app_review_ai`: `messages`

### Mutation and rotation semantics

Credential writes have deterministic audit behavior:

- First `PUT` for a provider with a non-empty key creates the row, sets `last_rotated_at`, and writes `AI_PROVIDER_KEY.CREATED`.
- Later `PUT` for the same provider with a non-empty key replaces the encrypted key, updates `key_tail` and `last_rotated_at`, and writes `AI_PROVIDER_KEY.ROTATED`.
- Later `PUT` for the same provider with an empty key keeps the existing encrypted key, updates non-secret config, and writes `AI_PROVIDER_KEY.UPDATED`.
- A provider change or first save without a key is rejected with `AI_PROVIDER_KEY_REQUIRED`.
- `POST /test` writes `AI_PROVIDER_KEY.TESTED` with redacted validation status.
- Routing changes write `AI_PROVIDER_KEY.ACTIVATED` with provider, surface, client kind, and model metadata.
- Disabling a provider writes `AI_PROVIDER_KEY.DISABLED` and deletes routing rows that pointed at that provider.

Concurrent edits are last-write-wins in v1. An optional `updated_at` precondition may be added during implementation if the UI needs conflict detection.

### Data model

Add a global table:

```sql
CREATE TABLE admin_ai_provider_keys (
  provider TEXT PRIMARY KEY CHECK (provider IN ('tokengate','openai','anthropic')),
  enabled BOOLEAN NOT NULL DEFAULT false,
  api_key_ciphertext TEXT NOT NULL,
  key_tail TEXT NOT NULL,
  base_url TEXT NOT NULL,
  chat_model TEXT NOT NULL DEFAULT '',
  messages_model TEXT NOT NULL DEFAULT '',
  last_validated_at TIMESTAMPTZ,
  last_validation_status TEXT,
  last_validation_error TEXT,
  last_rotated_at TIMESTAMPTZ,
  created_by_admin_id TEXT,
  updated_by_admin_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Add a separate routing table so "one active provider per surface" is enforced by the database, not by array inspection in application code:

```sql
CREATE TABLE ai_surface_routing (
  surface TEXT PRIMARY KEY CHECK (surface IN ('post_assist','error_triage','app_review_ai')),
  provider TEXT NOT NULL REFERENCES admin_ai_provider_keys(provider) ON DELETE RESTRICT,
  client_kind TEXT NOT NULL CHECK (client_kind IN ('chat_completions','messages')),
  model_override TEXT NOT NULL DEFAULT '',
  created_by_admin_id TEXT,
  updated_by_admin_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Notes:

- Use AES-256-GCM through `api/internal/crypto`.
- Store only ciphertext and key tail.
- Do not store API keys in `audit_log.before_json`, `audit_log.after_json`, or metadata.
- `ai_surface_routing.surface` is the hard uniqueness constraint for active routing.
- A provider can be enabled but not routed to any surface.
- Disabling a provider must remove or block all routing rows that point to it before future AI requests resolve.
- `last_validation_error` must be stored in redacted admin-safe form. It must not contain prompts, API keys, Authorization headers, ciphertext, request bodies, or provider-returned sensitive payloads.

### Audit

Add audit action constants:

```text
AI_PROVIDER_KEY.CREATED
AI_PROVIDER_KEY.ROTATED
AI_PROVIDER_KEY.UPDATED
AI_PROVIDER_KEY.TESTED
AI_PROVIDER_KEY.ACTIVATED
AI_PROVIDER_KEY.DISABLED
```

Use:

```text
resource_type = "ai_provider_key"
category = "config"
```

Audit payloads may include provider, base URL, models, routed surface, client kind, model override, key tail, validation status, and source. They must not include plaintext keys or ciphertext.

## Frontend Requirements

### Dashboard API client

Add typed helpers in `dashboard/src/lib/api.ts`:

```text
listAdminAIProviders
updateAdminAIProvider
testAdminAIProvider
routeAdminAIProviderSurface
deleteAdminAIProviderSurfaceRoute
disableAdminAIProvider
listAdminAIProviderEvents
```

### Admin page

Create:

```text
dashboard/src/app/admin/ai-keys/page.tsx
```

Use:

```text
<AdminShell title="AI Keys" requireSuperAdmin>
```

The page must handle:

- loading state
- empty/not configured state
- env fallback state
- validation success
- validation failure
- save success
- save failure
- disabled provider
- mobile sidebar behavior through existing `AdminShell`

Do not add visible implementation instructions or explanatory onboarding copy. Keep the page operational and compact.

## Security Requirements

1. Require admin access plus super admin access for all credential reads and writes.
2. Keep all provider calls server-side.
3. Store keys encrypted with `ENCRYPTION_KEY`.
4. Return only `key_tail`, never plaintext or ciphertext.
5. Redact provider errors before returning them to the browser.
6. Avoid logging Authorization headers, request bodies containing keys, or decrypted configs.
7. Write audit events for all mutations and validation tests.
8. Separate TokenGate keys by environment:

```text
unipost-dev
unipost-staging
unipost-production
```

9. Use development TokenGate keys only on development domains.
10. Use production TokenGate keys only after staging validation and explicit production release approval.
11. Store provider validation errors only after redaction.
12. `ENCRYPTION_KEY` rotation and re-encryption are out of scope for v1. If `ENCRYPTION_KEY` is rotated before re-key tooling exists, admins must re-enter AI provider keys.

## AI Data Handling Requirements

AI provider routing changes where prompt data is processed. Activating TokenGate for a surface means TokenGate can receive the prompt payload for that surface.

Surface data sensitivity:

- Post Assist sends customer draft content, selected destination context, media metadata, and validation issues.
- Error Triage sends redacted operational diagnostics, failure evidence, and potentially customer/account context that was safe enough for internal admin analysis.
- App Review AI sends redacted browser observations and review goals.

Requirements:

1. The admin UI must show a concise data-processing notice before routing a surface to TokenGate for the first time.
2. TokenGate routing should initially be enabled only for `post_assist` in development.
3. `error_triage` must remain on OpenAI environment fallback or deterministic fallback until the team confirms TokenGate data retention, no-training, and support-access policies are acceptable for diagnostic data.
4. `app_review_ai` must remain on native Anthropic until TokenGate Messages validation confirms header compatibility and model behavior for constrained action JSON.
5. Existing redaction tests for Error Triage and App Review AI remain mandatory before any third-party gateway routing is enabled.
6. The PRD accepts TokenGate support for all three surfaces as a product direction, but rollout is per surface and explicit, not automatic.

## Feature Flag Decision

Before implementation starts, ask the user whether this dashboard/API-layer change should be protected by a feature flag, per repo workflow.

If approved, create:

```text
admin.ai_provider_keys_v1
```

Recommended defaults:

```text
development: on
production: off
fallback: off in production
```

Owner area: Admin / Platform / AI.

Rollback action: disable `admin.ai_provider_keys_v1` to hide the admin page and block admin mutation endpoints. Existing AI provider config remains stored but ignored by UI entry points. Backend AI runtime should keep environment fallback behavior unless the implementation explicitly gates runtime provider resolution behind the same flag.

Third-party dependency: TokenGate API and dashboard availability.

If a feature flag is not approved, implement without a new flag but keep production behavior conservative: existing environment variables remain the fallback and no production TokenGate traffic is routed until a super admin explicitly routes a surface after validation.

## TokenGate Setup Checklist

1. Create a TokenGate API key in the TokenGate dashboard for the target environment.
2. Name the key with environment and date, for example:

```text
unipost-dev-2026-06-07
```

3. Use a separate key for development, staging, and production.
4. Paste the key into `/admin/ai-keys` for the matching environment.
5. Set base URL to:

```text
https://gateway.mytokengate.com/v1
```

6. Choose initial models for chat-completions and messages.
7. Click `Test connection`.
8. Route a surface to TokenGate only after validation passes.
9. Run the relevant AI workflow in the development environment.
10. Rotate the key every 90 days or immediately after suspected exposure.

Do not store TokenGate plaintext keys in docs, tickets, Slack, logs, or screenshots.

## Rollout Plan

### Phase 1 - Admin storage and status

- Add DB migration for `admin_ai_provider_keys` and `ai_surface_routing`.
- Add backend provider config load/save/test endpoints.
- Add backend surface routing endpoints.
- Add audit actions.
- Add dashboard API client methods.
- Add `/admin/ai-keys` page and sidebar item.
- Show environment fallback status for OpenAI and Anthropic.
- Validate TokenGate using `/models`.

### Phase 2 - Shared provider registry

- Add `api/internal/aiproviders`.
- Migrate AI Post Assist to resolve through the registry on each request.
- Migrate Error Triage analyzer to resolve through the registry for each run.
- Keep deterministic/stub fallback behavior unchanged.
- Add unit tests for provider resolution order, cache invalidation, and per-surface model selection.
- Ensure `error_triage_runs.model` records the actual provider/model used for that run.

### Phase 3 - App Review AI compatibility

- Add Anthropic-compatible messages support through the registry.
- Keep native Anthropic as the safe default.
- Allow TokenGate Messages only after validation confirms the selected model works.
- Migrate App Review AI planner to request-time registry resolution.

### Phase 4 - Development deployment validation

- Push through the normal `dev` flow.
- Wait for the development deployment.
- Validate `/admin/ai-keys` on `https://dev-app.unipost.dev`.
- Save and validate a development TokenGate key.
- Route only `post_assist` to TokenGate first.
- Run one low-risk AI Post Assist request in the development app.
- Confirm Error Triage remains on its configured fallback until diagnostic-data policy approval.
- Run one Error Triage test path or manual run only after explicit surface routing approval.

## Testing Requirements

Backend:

- Unit test provider normalization and base URL trimming.
- Unit test encrypted storage never returns plaintext.
- Unit test empty key keeps the existing key only when editing the same provider.
- Unit test provider change requires a new key.
- Unit test TokenGate validation success using a mock `/models` response.
- Unit test 401, 403, 429, 503, and timeout mapping.
- Unit test audit payloads exclude plaintext and ciphertext.
- Unit test provider resolution order: routed admin provider, env fallback, deterministic/stub fallback.
- Unit test `ai_surface_routing.surface` uniqueness prevents double routing for one surface.
- Unit test dynamic request-time resolution reflects a routing/key update without process restart.
- Unit test model resolution: route override, provider default, then per-surface env fallback.
- Unit test native Anthropic uses `x-api-key` plus `anthropic-version`, while TokenGate Messages uses `Authorization: Bearer`.
- Unit test persisted validation errors are redacted.
- Unit test super-admin-only route enforcement.

Frontend:

- Unit test API normalizers for provider status.
- Component test empty, configured, env fallback, validation failed, and routed states.
- Verify password input clears after save.
- Verify key tail is displayed and full key is never rendered.

Manual/deployed:

- Validate development admin page loads for super admins.
- Validate non-super admins get the existing 403 super-admin-only state.
- Validate TokenGate test connection with a real development key.
- Validate Post Assist can generate through TokenGate in dev.
- Validate disabling TokenGate or deleting the `post_assist` route falls back to OpenAI environment key or existing deterministic fallback.

## Acceptance Criteria

1. `/admin/ai-keys` appears in the admin sidebar under `System`.
2. The page is visible only to users in `SUPER_ADMINS`.
3. The backend admin endpoints require admin and super-admin access.
4. TokenGate, OpenAI, and Anthropic appear in the provider status table.
5. Existing OpenAI and Anthropic env keys appear as configured environment fallbacks without exposing values.
6. A super admin can save a TokenGate key with base URL `https://gateway.mytokengate.com/v1`.
7. The saved key is encrypted at rest and never returned to the browser.
8. The UI shows only the key tail and timestamps after save.
9. `Test connection` validates TokenGate server-side and maps common provider errors to admin-safe messages.
10. Routing `post_assist` to TokenGate sends Post Assist chat-completions calls through TokenGate without restarting the API process.
11. The database prevents two admin-managed providers from being routed to the same surface at the same time.
12. Per-surface model selection preserves the current Error Triage `OPENAI_ERROR_TRIAGE_MODEL` behavior.
13. Native Anthropic and TokenGate Messages use the correct provider-specific auth headers.
14. Disabling TokenGate or deleting a surface route restores existing fallback behavior.
15. All credential mutations write audit log events without plaintext or ciphertext.
16. AI Post Assist continues to work when only `OPENAI_API_KEY` is configured.
17. Error Triage records the actual provider/model used for each run.
18. App Review AI continues to work when only `ANTHROPIC_API_KEY` is configured.
19. Development deployment is validated on `https://dev-app.unipost.dev` before implementation is reported complete.

## Open Implementation Notes

- The PRD recommends `AI Keys` as the sidebar label. `AI Providers` is also acceptable if implementation discovers the page will emphasize provider routing more than credential rotation.
- TokenGate remote key creation is intentionally manual in v1. If TokenGate later publishes an admin key-management API, add a separate PRD section for remote key lifecycle management.
- If provider usage/cost reporting becomes important, add a later metrics table fed by the AI registry. V1 should focus on secure configuration and routing.
