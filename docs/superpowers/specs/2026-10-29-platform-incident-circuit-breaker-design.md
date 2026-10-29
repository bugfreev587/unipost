# Platform Incident Circuit Breaker Design

**Date:** 2026-10-29  
**Status:** Approved in incident conversation  
**Branch:** `main-security-incident-investigation-20261029`  
**Base:** `origin/main` at `762bf793b9bda4fa7f4abc93c30a6abb5e57fdaf`

## Goal

Restore UniPost production service while keeping TikTok completely isolated until TikTok rotates the application secret and revokes existing tokens. The operator must be able to change each supported platform independently without redeploying.

## Decision

Keep the existing plan-scoped publishing restriction unchanged for commercial and capacity policy. Add a separate incident circuit breaker whose meaning is operational, not billing-related.

Each supported platform has one state:

- `active`: normal behavior.
- `publishing_disabled`: account connection and non-publishing reads may continue, but no new or resumed provider publish call may start.
- `fully_disabled`: no OAuth or Hosted Connect flow, publishing call, retry/resume, analytics request, token refresh, or other authenticated outbound provider call may start.

The initial production state after migration is conservative but non-disruptive: all seeded platforms default to `active`. TikTok is switched to `fully_disabled` through an audited administrative mutation before the API or delivery worker is restored. Facebook, Instagram, and Threads remain controlled independently because their application lifecycle and reconnect readiness differ from TikTok.

## Supported platforms

Seed one row for every registered production connector:

- `twitter`
- `linkedin`
- `bluesky`
- `youtube`
- `tiktok`
- `instagram`
- `threads`
- `facebook`
- `pinterest`

Unknown platforms fail closed at mutation time. Runtime evaluation of an unseeded platform returns `fully_disabled` so a newly registered connector cannot silently bypass incident controls.

## Data model and audit

Create `platform_operational_controls` with platform, state, reason code, operator-safe user message, version, timestamps, and the last actor. Create an append-only event table recording before/after state, actor, request metadata, and optimistic-lock versions.

Administrative changes require the current version. A stale version returns conflict and makes no change. Runtime reads are direct database reads; correctness is preferred over caching during an incident.

## Enforcement

A shared evaluator returns an operation-specific decision for `connect`, `publish`, or `read`.

- OAuth authorization start and callback enforce `connect` before generating an authorization URL or exchanging a code.
- Hosted Connect session creation, authorization, and callback enforce `connect`.
- Post creation, draft publish, bulk publish, scheduling, retry creation, dispatch, and retry execution enforce `publish`.
- The delivery worker checks immediately before every provider call. The global circuit breaker overrides the existing publish-token resume exception; a stored TikTok `publish_id` or Instagram container ID cannot bypass it.
- Analytics, metrics refresh, token refresh, and other authenticated provider operations enforce `read`.
- Webhooks may be accepted and recorded because they are inbound and do not expose credentials or initiate provider actions.

When a publishing operation is blocked, no provider adapter method is called. New API requests receive a stable `PLATFORM_OPERATION_DISABLED` response with the platform and current state. Already queued jobs are finalized as temporarily failed with a retry-after-recovery next action; they are not left running or silently dropped.

If evaluation fails because the database cannot be read, authenticated outbound platform operations fail closed. Ordinary UniPost data reads that do not contact a provider remain available.

## Administration

Expose super-admin-only list and update endpoints next to the existing publishing-restriction administration. The list reports every supported platform and current state. Updates use optimistic locking and create an audit event.

This incident does not require a new dashboard UI. The API and an explicit database-safe operational command are sufficient for the first production recovery. UI work is deferred.

## Recovery sequence

1. Deploy the schema and code while API and delivery worker remain offline.
2. Set TikTok to `fully_disabled` and verify the persisted state and audit event.
3. Verify all other intended platforms are `active`.
4. Restore the API first and verify profiles, projects, and non-provider data reads.
5. Verify TikTok OAuth and publish probes are rejected without provider calls.
6. Restore the delivery worker and verify a synthetic/non-provider-safe job path plus queue health.
7. Restore Meta platforms only according to their separate app publication and account reconnect readiness.

## Testing and acceptance

Tests must be written before implementation and demonstrate:

- state/operation decision matrix and fail-closed unknown-platform behavior;
- optimistic-lock update and append-only audit event;
- OAuth and Hosted Connect rejection in `fully_disabled`;
- API publish rejection in both disabled states;
- worker rejection immediately before dispatch, including a job with a persisted publish token;
- analytics/read rejection only in `fully_disabled`;
- unaffected platforms remain functional;
- database evaluation errors never cause an outbound provider call.

The complete backend suite must pass. Production recovery additionally requires deployment success, persisted TikTok state verification, API health, rejection probes, queue inspection, and browser verification of profile visibility.

## Non-goals

- Rotating provider credentials through this feature.
- Deleting social accounts or tokens.
- Replacing the existing Free-plan TikTok capacity restriction.
- Automatically replaying posts blocked during the incident.
- Building a new operator dashboard in this hotfix.
