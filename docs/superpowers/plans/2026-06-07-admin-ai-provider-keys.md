# Admin AI Provider Keys Implementation Plan

## Objective

Implement `docs/prd-admin-ai-provider-api-keys.md` without a feature flag. Add a super-admin-only `/admin/ai-keys` dashboard page, encrypted global AI provider key storage, per-surface routing, TokenGate/OpenAI/Anthropic request-time provider resolution, validation, admin-safe event history, and deployed dev acceptance.

## Constraints

- Work on `dev-admin-ai-provider-keys` from latest `origin/dev`.
- Do not touch unrelated untracked files.
- Do not add a feature flag; the user explicitly said no flag.
- Preserve environment fallback behavior for OpenAI and Anthropic.
- Do not return, log, or persist plaintext provider keys outside encrypted storage.
- Current `audit_log` is workspace-scoped, so global AI provider events will use a dedicated global admin event table instead of weakening the existing audit table.

## Phase 1 - Database And SQLC

1. Add migration `081_admin_ai_provider_keys.sql`.
2. Create `admin_ai_provider_keys` with encrypted key ciphertext, key tail, provider defaults, validation metadata, timestamps, and admin actor ids.
3. Create `ai_surface_routing` with `surface TEXT PRIMARY KEY` to enforce one provider per surface.
4. Create `admin_ai_provider_events` for global admin-safe AI provider events.
5. Add SQLC queries for providers, routing, and events.
6. Run `sqlc generate` from `api/`.

## Phase 2 - Backend Provider Registry

1. Add `api/internal/aiproviders`.
2. Define providers, surfaces, client kinds, validation statuses, event actions, and admin-safe response DTOs.
3. Implement config normalization:
   - provider validation
   - base URL trimming
   - default base URLs
   - surface/client-kind compatibility
   - model fallback order
4. Implement encrypted create/update/rotate/disable behavior.
5. Implement route/unroute behavior with DB-enforced uniqueness.
6. Implement request-time effective resolution:
   - routed admin provider
   - environment fallback
   - existing deterministic/stub fallback by caller
7. Implement validation probes:
   - `/models` for TokenGate/OpenAI-compatible providers
   - provider error mapping for 400/401/403/429/503/504/timeouts
   - redaction before persistence or response
8. Implement chat-completions and messages HTTP clients:
   - TokenGate/OpenAI chat uses `Authorization: Bearer`
   - TokenGate messages uses `Authorization: Bearer`
   - native Anthropic messages uses `x-api-key` and `anthropic-version`
9. Add unit tests before implementation for normalization, redaction, storage semantics, routing, resolution, validation mapping, and header selection.

## Phase 3 - Admin API

1. Add `api/internal/handler/ai_providers.go`.
2. Add super-admin-only routes under `/v1/admin`:
   - `GET /ai-providers`
   - `PUT /ai-providers/{provider}`
   - `POST /ai-providers/{provider}/test`
   - `POST /ai-providers/{provider}/disable`
   - `PUT /ai-provider-routing/{surface}`
   - `DELETE /ai-provider-routing/{surface}`
   - `GET /ai-providers/events`
3. Mount routes in `api/cmd/api/main.go` behind `auth.RequireSuperAdmin`.
4. Add handler tests for response shape, auth guard expectations where practical, no-secret responses, stored-key test behavior, and route mutation behavior.

## Phase 4 - Runtime AI Surface Migration

1. Migrate AI Post Assist:
   - inject provider registry into `AIPostAssistHandler`
   - resolve `post_assist` on each request
   - keep current stub fallback behavior on provider failure
2. Migrate Error Triage:
   - add a run-scoped analyzer hook so provider/model are resolved at run time
   - preserve deterministic fallback
   - write `error_triage_runs.model` from the actual provider/model used for the run
3. Migrate App Review AI:
   - inject dynamic messages planner
   - keep native Anthropic env fallback
   - support TokenGate Messages only through explicit routing and validation
4. Add or update tests for each surface's fallback and dynamic resolution behavior.

## Phase 5 - Dashboard API And UI

1. Add typed API helpers in `dashboard/src/lib/api.ts`.
2. Add `AI Keys` nav item under `System` in `dashboard/src/app/admin/_components/admin-ui.tsx`.
3. Create `dashboard/src/app/admin/ai-keys/page.tsx` with `AdminShell title="AI Keys" requireSuperAdmin`.
4. Build compact operational UI:
   - provider status table
   - effective routing policy table
   - provider editor with password key input
   - routing controls with optional model override
   - validation result and recent events
5. Ensure the UI never renders full API keys and clears password input after save.

## Phase 6 - Local Verification

1. Run backend tests:
   - `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`
2. Run dashboard build:
   - `cd dashboard && npm run build`
3. Run dashboard regression tests if Playwright browsers are available:
   - `cd dashboard && npm run test:regression:dashboard`
4. Fix any failures and rerun the relevant validation.

## Phase 7 - Dev Integration And Acceptance

1. Check status before switching branches.
2. Update local `dev` from `origin/dev`.
3. Merge `dev-admin-ai-provider-keys` into local `dev`.
4. Rerun required validation on local `dev`.
5. Push local `dev` to `origin/dev`.
6. Monitor triggered checks/deployments until complete.
7. Open development environment:
   - `https://dev-app.unipost.dev/admin/ai-keys`
   - `https://dev-api.unipost.dev`
8. Verify acceptance:
   - page appears for super admin
   - provider statuses load
   - env fallbacks show without secrets
   - TokenGate can be saved/tested if dashboard access/key is available
   - `post_assist` routing can be set to TokenGate only after validation
   - disable/unroute restores fallback behavior

