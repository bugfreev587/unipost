# X Inbox Webhook Hotfix PRD

**Status:** Approved design; external review findings resolved; implementation plan pending

**Date:** 2026-07-23

**Branch:** `hotfix-x-inbox-webhooks`

**Base:** latest `origin/staging` at hotfix creation (`0babd5cb312ce0cc4f2d3fc995ce8dad0bd2facf`)

**Owner area:** X Inbox

**Release path:** `hotfix-x-inbox-webhooks` → `staging` → `main` → sync the same hotfix branch to `dev`

## 1. Executive summary

UniPost production currently reports that X Comments/Replies and X DMs are available for the user-owned managed account `@unipostdev`, but no real-time provider resources exist for that app. The production X Developer Console has no Filtered Stream rule, no webhook, and no X Activity subscription. The production delivery record is in `error` because the delivery runtime lacks the managed app bearer token and consumer secret.

The code contains the ingestion, signature verification, parser, routing, and provider client needed for real-time delivery. The blocking code path is the X Inbox delivery worker: an earlier rollback hard-coded `dmsDesired := false` and removed webhook plus `dm.received` subscription provisioning after X returned HTTP 403 during a development attempt. The worker also treats the consumer secret as a shared prerequisite for both public Comments and private DMs, so a missing webhook-signing credential disables Comments even though Filtered Stream does not need it.

This hotfix restores self-healing delivery with independent Comments and DM desired states. DM provisioning is protected by two required gates: the existing backend `x_dms_v1` workspace evaluator and an operational exact-account canary allowlist. It uses app bearer authentication for current X webhook and X Activity subscription management, admits legacy unencrypted `dm.received` events, explicitly excludes encrypted XChat `chat.received`, and preserves managed-user isolation by routing only to an exact social account or an unambiguous exact provider X user ID.

The rollout is deliberately bounded. Staging must prove one user-owned canary before production. Production remains globally flag-off and permits only the user-owned production social account during acceptance. Under that global-off state, the existing evaluator returns true only while the fixture workspace owner is a Super Admin, so that evaluator result is an explicit pre-canary gate rather than an assumed property. Any required check failure, timeout, skipped result, provider 403, ambiguous route, or different-SHA validation is a hard stop.

## 2. Incident statement

### 2.1 User impact

- New X replies/comments do not arrive in UniPost Inbox in real time.
- New legacy unencrypted X DMs do not arrive in UniPost Inbox in real time.
- The API capability surface may report Comments and DMs as enabled from account scopes and feature eligibility even when delivery resources are absent.
- Manual polling or DM backfill is not an acceptable substitute for this hotfix because it is not real time and can incur provider cost.

### 2.2 Confirmed production fixture

| Field | Value |
| --- | --- |
| UniPost profile | `16202f3f-0c3c-4b92-afae-177f279c692a` |
| Managed user | `sdk-inbox-x` |
| X account | `@unipostdev` |
| Social account ID | `bc507960-aed6-4ae7-8568-27ad63cf5c58` |
| Provider X user ID | `2039562772455809024` |
| X app | `UniPost.dev` (`32804437`) |
| X developer account | `2046271311652200448` |

The account is active, plan-eligible for Inbox, connected through the UniPost-managed X app, and has `offline.access`, `dm.read`, `dm.write`, `tweet.read`, `tweet.write`, `users.read`, and `media.write`.

### 2.3 Independently re-verified evidence

The following facts were checked before any application code or environment change:

- Fresh `origin/main` is `daf30712cef932282d419854edb05d86db653d10`.
- Fresh `origin/staging` at hotfix creation is `0babd5cb312ce0cc4f2d3fc995ce8dad0bd2facf`.
- Production X app `32804437` has zero event subscriptions, zero webhooks, and zero streaming rules.
- Production X app `32804437` has no generated App-Only Bearer Token. Its existing user authorization for `@unipostdev` includes the required read/write/DM scopes.
- Production database state for the exact social account has no rule ID, DM subscription ID, or activity route generation.
- Production delivery status is `error` with missing `TWITTER_CONSUMER_SECRET` and `TWITTER_BEARER_TOKEN` configuration errors.
- The `x_dms_v1` global flag is `false`.
- With the global flag off, `featureflags.Evaluator.ForWorkspace` can return true only when the workspace owner resolves as a Super Admin. The fixture's current evaluator result must be re-verified immediately before staging and production canary activation; it is not inferred from account scopes or capability output.
- `api/internal/worker/x_inbox_delivery.go` hard-codes `dmsDesired := false` and has no active webhook/subscription creation path.
- `api/internal/xinbox/subscriptions.go` already implements `/2/webhooks` and `/2/activity/subscriptions` using app bearer authentication and stable account tags.
- `api/internal/xinbox/ingest.go` already parses current `dm.received` and legacy `direct_message_events` payloads.
- `api/cmd/api/main.go` already evaluates `x_dms_v1` at DM ingestion and user-facing capability boundaries, but does not pass the evaluator into the delivery worker.
- The legacy fallback query currently matches either `social_accounts.external_user_id` or `social_accounts.external_account_id`. A provider X user ID must never be treated as a customer-managed user ID; this fallback is broader than the required isolation boundary.

### 2.4 Current official provider contract

- X Webhook management endpoints require an OAuth 2.0 App-Only Bearer Token: <https://docs.x.com/x-api/webhooks/introduction>.
- X Activity subscription creation supports `dm.received`: <https://docs.x.com/x-api/activity/create-x-activity-subscription>.
- X added legacy DM event types to the X Activity API on 2026-02-23: <https://docs.x.com/changelog>.
- XChat `chat.received` is a distinct encrypted chat product with user-context authorization and decryption requirements: <https://docs.x.com/xchat/real-time-events>.
- Filtered Stream is the near-real-time public Post channel used for replies/comments: <https://docs.x.com/x-api/posts/filtered-stream/introduction>.

## 3. Root cause

### 3.1 Primary code cause

Commit `59875223` intentionally disabled private X Activity provisioning after earlier 403 responses. It:

- replaced the calculated DM desired state with `dmsDesired := false`;
- removed stale route-generation replacement;
- removed `EnsureWebhook` and `EnsureDMSubscription` calls;
- changed tests to require that DM provisioning never occurs.

That rollback was safe for the provider behavior known at the time, but it remained in production after the X Activity API contract changed to include legacy DM events.

### 3.2 Runtime configuration cause

The production Railway service does not have the app bearer token or consumer secret required by the current worker and webhook signature resolver. The X production app also lacks an App-Only Bearer Token, so the missing bearer cannot be resolved only by copying an existing value.

### 3.3 Coupling cause affecting Comments

`deliveryCredentialError` treats a consumer secret as mandatory for the whole account. The consumer secret is required for webhook CRC and signature verification, but not for Filtered Stream rule management or the stream connection. This coupling prevents Comments from recovering independently.

### 3.4 Isolation weakness in legacy fallback

Current X Activity events carry a stable subscription tag from which UniPost can recover the exact social account ID. Legacy Account Activity envelopes can omit the tag and provide only `for_user_id`, which is a provider X user ID. The current fallback query also compares this provider value with the customer-controlled `social_accounts.external_user_id`. That comparison is semantically incorrect and can become ambiguous or cross-route if a customer-managed identifier happens to equal an X numeric user ID.

## 4. Goals

1. Restore near-real-time X Comments/Replies through Filtered Stream.
2. Restore near-real-time legacy unencrypted DMs through X Activity `dm.received` webhooks.
3. Keep Comments and DMs independently eligible, provisioned, cleaned up, and observable.
4. Make `x_dms_v1` the backend authority for the DM lifecycle, including worker provisioning.
5. Add an exact-account operational canary boundary in addition to the feature flag.
6. Fail closed on flag evaluation errors, provider authorization failures, malformed events, ambiguous routing, and missing DM credentials without disabling eligible Comments.
7. Preserve stable tags, stable webhook route generations, idempotent replacement, and exact cleanup.
8. Preserve managed-user isolation and the existing owner/admin/API-key aggregate contract.
9. Prove the change in staging, production, and dev through the repository's hotfix flow.
10. Avoid DM backfill and unapproved billable provider calls.

## 5. Non-goals

- Encrypted XChat events such as `chat.received`, `chat.sent`, or `chat.conversation_join`.
- XChat decryption, key exchange, or XDK integration.
- DM history backfill or replay jobs.
- Polling as a replacement for real-time delivery.
- Changes to publishing, scheduling, analytics computation, or unrelated platforms.
- A global production rollout of `x_dms_v1`.
- New browser/mobile managed-user authentication. The existing server-to-server managed Inbox scope remains the contract.
- Rotation of existing X credentials that are still valid.
- A new customer-facing feature flag.

## 6. Safety and product invariants

1. `x_dms_v1=false` always means no DM subscription is desired, regardless of canary configuration.
2. A social account absent from the canary allowlist never receives a new DM subscription.
3. Missing or invalid canary configuration fails closed for DMs and does not affect Comments.
4. A feature evaluator error fails closed for DMs, surfaces an error, and does not affect Comments.
5. Comments never require a consumer secret or webhook URL.
6. DMs require both provider-management and webhook-signing credentials.
7. A provider payload cannot choose `workspace_id`, customer `external_user_id`, or an arbitrary social account.
8. Tagged events route to exactly one social account through the app-specific route and subscription tag.
9. Untagged legacy events route only through an unambiguous exact provider X user ID within the app route.
10. Zero or multiple legacy candidates result in no Inbox write.
11. `chat.*` events never become `x_dm` Inbox items.
12. No token, consumer secret, webhook route secret, signature, private DM body, or previously pasted UniPost API key is logged or included in artifacts. The pasted production UniPost API key is treated as exposed and must be revoked or rotated independently and promptly; it is not reused for hotfix acceptance.
13. Provider resources are never considered removed unless the provider confirms deletion or returns a documented idempotent-not-found response.
14. A failed, timed-out, cancelled, skipped, or different-SHA check blocks promotion.

## 7. Approved solution

### 7.1 Architecture choice

Use the existing reconciliation worker and provider client as the single resource-lifecycle authority. Add:

- independent Comments and DM desired-state calculation;
- the existing `x_dms_v1` evaluator at the worker boundary;
- an exact `social_account_id` canary allowlist;
- credential requirements scoped to each delivery source;
- restoration of idempotent Webhook and `dm.received` subscription provisioning;
- status-aware HTTP 403 classification and a persisted retry latch;
- exact legacy routing by provider X user ID only;
- explicit parser tests rejecting XChat admission.

This is preferred over a one-shot provisioning script because provider state must remain self-healing after route rotation, account disconnection, scope loss, plan change, spend pause, flag change, or credential replacement.

### 7.2 Component boundaries

| Component | Responsibility | Required change |
| --- | --- | --- |
| `featureflags.Evaluator` | Workspace DM eligibility | Reuse unchanged through a worker callback/interface |
| X Inbox delivery worker | Desired state and lifecycle | Split Comments/DM logic, apply flag and allowlist, restore provisioning, add 403 latch |
| X provider client | Webhook/subscription API | Preserve status in sanitized errors so 403 is machine-detectable |
| X Inbox delivery store | Durable state | Add and load a dedicated nullable `dm_subscription_forbidden_fingerprint`; keep `last_error` out of control flow |
| X webhook handler | CRC, signature, parse, acknowledge | Keep current security behavior; admit only recognized legacy DM events |
| X ingestion store | Provider route to local account | Make legacy fallback exact and unambiguous |
| X ingestion service | Account-derived Inbox item | Preserve account/workspace ownership and feature gate |
| `main.go` wiring | Runtime dependencies | Pass evaluator and parsed canary configuration into worker |

## 8. Desired-state model

### 8.1 Common account eligibility

The worker begins with these common facts:

- account is connected and `status='active'`;
- plan allows Inbox;
- app mode is `unipost_managed_app` or `workspace_x_app`;
- provider spend safety does not report a known pause.

`legacy_unknown` is never eligible.

### 8.2 Comments desired state

Comments are desired when all of the following are true:

- common account eligibility is true;
- scopes contain `tweet.read`, `tweet.write`, and `users.read`;
- an app bearer token can be resolved;
- a stable app identity/route key can be resolved for stream ownership;
- provider spend safety does not report a known pause.

Comments do not require:

- `x_dms_v1`;
- DM scopes;
- the DM canary allowlist;
- a consumer secret;
- a webhook URL;
- an Activity subscription.

### 8.3 DM desired state

DMs are desired when all of the following are true:

- common account eligibility is true;
- scopes contain `dm.read`, `dm.write`, and `users.read`;
- `featureflags.Evaluator.ForWorkspace(ctx, workspaceID, featureflags.XDMSV1)` returns `(true, nil)`;
- the exact `social_account_id` is in `X_INBOX_DM_CANARY_SOCIAL_ACCOUNT_IDS`;
- an app bearer token can be resolved;
- a consumer secret is configured for CRC and signature verification;
- a stable webhook route key is available;
- `X_INBOX_WEBHOOK_URL` is a valid absolute HTTPS base URL;
- provider spend safety does not report a known pause.

DM desired state does not depend on Comments desired state or tweet scopes.

When `x_dms_v1` is globally off, the existing evaluator makes workspace-owner Super Admin status a required input: the fixture is DM-eligible only while `ForWorkspace(ctx, workspaceID, XDMSV1)` returns `(true, nil)`. Before any staging or production provider mutation, a read-only pre-canary check must prove that exact evaluator result for the fixture workspace while the global flag remains off. A false result or evaluation error is a normal fail-closed stop: no DM ensure call occurs, any exact stored DM subscription is removed through the existing cleanup contract, and eligible Comments remain unaffected. Losing Super Admin status after activation therefore converges DMs to off rather than leaving an orphaned desired subscription.

### 8.4 Decision matrix

| Condition | Comments | DMs | Required action |
| --- | --- | --- | --- |
| Account inactive/disconnected | Off | Off | Delete stored rule/subscription by exact IDs |
| Plan disallows Inbox | Off | Off | Delete both; mark `paused_plan` |
| Known spend pause | Off | Off | Delete both; mark the existing cap/allowance status |
| Spend evaluation error | No new start/create | No new create | Preserve provider IDs for retry, stop starting the local stream for the incomplete cycle, save error |
| Missing tweet scopes only | Off | Independently evaluated | Remove rule only |
| Missing DM scopes only | Independently evaluated | Off | Remove subscription only |
| App bearer missing | Off | Off | Save credential error; do not call provider |
| Consumer secret missing | Independently evaluated | Off | Keep Comments; save DM credential error |
| Webhook URL missing/invalid | Independently evaluated | Off | Keep Comments; save DM configuration error |
| Flag false | Independently evaluated | Off | Remove subscription; clear any previous 403 latch |
| Global flag off and owner is not Super Admin | Independently evaluated | Off | Same as flag false; never infer eligibility from scopes or canary membership |
| Flag evaluation error | Independently evaluated | Off | Remove subscription, retain Comments, return/surface evaluator error |
| Not canary-allowlisted | Independently evaluated | Off | Remove subscription; this is a normal gated state |
| Provider returns 403 while provisioning DM | Independently evaluated | Off/latched | Preserve Comments, save forbidden latch, stop unchanged retries |
| All source-specific requirements satisfied | On | On | Ensure each resource independently and save exact IDs |

## 9. Canary allowlist contract

### 9.1 Configuration

Add the non-secret runtime variable:

`X_INBOX_DM_CANARY_SOCIAL_ACCOUNT_IDS`

Contract:

- comma-separated exact social account UUIDs;
- trim whitespace;
- remove duplicates;
- reject malformed UUID entries;
- a missing or entirely empty configuration yields an empty effective allowlist;
- any malformed or empty entry inside a non-empty list invalidates the whole list; do not partially apply the remaining valid entries;
- invalid configuration therefore yields an empty effective allowlist for the entire process configuration;
- invalid configuration produces one sanitized startup/reconciliation error and never broadens access;
- the allowlist is an additional operational safety boundary, not a replacement for `x_dms_v1`.

### 9.2 Environment policy

- Initial staging deployment: empty allowlist.
- Staging canary: exactly one user-owned staging fixture after code/deployment health is proven.
- Initial production deployment: empty allowlist until exact-SHA deployment health is proven.
- Production acceptance: exactly `bc507960-aed6-4ae7-8568-27ad63cf5c58`.
- `x_dms_v1` remains globally off in production. Super Admin workspace eligibility is evaluated through the existing backend evaluator.
- Dev sync: empty by default unless a user-owned dev fixture is explicitly selected.

Changing the allowlist is an environment configuration change and must be monitored like a deployment. It must not be logged with unrelated account identifiers.

## 10. Provider resource lifecycle

### 10.1 Comments rule

- Stable tag: existing `unipost:x:account:<social_account_id>` format.
- Rule value continues to target replies/comments for the connected handle.
- If the stored rule ID is absent and Comments are desired, ensure the rule and persist its exact ID before starting the stream.
- If Comments are not desired, delete only the exact stored rule ID.
- Comments failures do not cause DM deletion unless a shared common eligibility condition is false.

Filtered Stream connection ownership remains app-wide, not account-wide. Reconciliation first evaluates every account, then deduplicates desired streams by stable app identity. One shared connection stays running while at least one Comments-desired account for that app has a persisted rule; disabling or deleting one account's rule must not restart or stop that shared connection while another desired rule remains. When the last Comments-desired account for the app drops, the worker stops that one app stream exactly once. DM-only gate, subscription, latch, or credential changes never start, stop, or restart Filtered Stream. If a reconciliation cycle cannot determine the complete desired app set, the worker preserves the current stream set and does not apply a partial start/stop decision.

### 10.2 App webhook

- URL is derived through `AppWebhookURL(X_INBOX_WEBHOOK_URL, webhookRouteKey)`.
- The route key is stable across consumer-secret rotation.
- `EnsureWebhook` lists app webhooks, reuses an exact URL match, revalidates an invalid exact match, or creates one when absent.
- A webhook is app-level and can be shared by multiple account subscriptions. Turning one account's DM desired state off does not delete the webhook.
- Webhook creation/revalidation must pass CRC before it is considered valid.
- Before a canary allowlist becomes non-empty, probe the already-deployed app-specific callback with a fresh non-secret synthetic `crc_token`. Require HTTP 200 and independently verify that `response_token` equals the expected HMAC-SHA256 result using the configured consumer secret without printing either value. This route-health probe is required before the first provider webhook create/revalidate call and does not substitute for the provider's own synchronous CRC validation.

### 10.3 DM subscription

- Event type is exactly `dm.received`.
- Filter `user_id` is exactly `social_accounts.external_account_id`, never the customer-managed `external_user_id`.
- Stable tag is `unipost:x:dm:<social_account_id>`.
- The subscription references the exact ensured webhook ID.
- If an existing stable-tag subscription matches event type, provider user ID, webhook ID, and route generation, reuse it.
- If any of those fields differ, delete the old subscription by exact ID, persist the cleared local state, then create and persist the replacement.
- Reconciliation remains idempotent after restarts and repeated cycles.

### 10.4 Cleanup

- Existing cleanup-intent leasing, retry budget, app token generation, and exact ID deletion remain authoritative.
- Cleanup uses the app bearer associated with the generation that created the resource.
- A delete 404/documented missing-resource response is success.
- A delete 403, timeout, or ambiguous provider response is failure and the local ID remains pending cleanup.
- No cleanup path enumerates and bulk-deletes resources belonging to another app or account.

## 11. HTTP 403 handling and retry latch

### 11.1 Status-aware errors

Provider client errors used by webhook and Activity subscription management must expose, without secrets:

- HTTP method;
- provider path template;
- HTTP status;
- sanitized provider error code/title when present.

Authorization headers, tokens, signatures, request bodies containing private data, and raw secret-bearing URLs are never included.

### 11.2 Persistent latch

Add an additive nullable column to `x_inbox_delivery_resources`:

`dm_subscription_forbidden_fingerprint TEXT`

This dedicated field is the only durable control-flow state for the DM creation latch. Load it into the worker account model and update it through the delivery-resource store. `last_error` remains a human-readable, sanitized account-level summary of the most recent actionable reconciliation error and is never parsed or compared to decide whether a provider call is allowed. This prevents a simultaneous Comments error from clearing the DM retry latch and prevents a DM error from erasing the only durable evidence needed to bound retries.

When a DM ensure/list/create operation returns HTTP 403, save the SHA-256 fingerprint of the non-secret desired configuration in `dm_subscription_forbidden_fingerprint`. Save a sanitized DM 403 summary in `last_error`, but do not depend on that text for latching.

The fingerprint covers:

- app mode;
- app/route identity;
- social account ID;
- provider X user ID;
- webhook URL;
- desired event type.

It does not include bearer tokens, consumer secrets, DM content, or signatures.

If the same account remains DM-desired with the same fingerprint, subsequent reconciliation cycles:

- do not call X subscription provisioning again;
- keep eligible Comments active;
- keep delivery status `error` with the latch reason;
- emit only bounded structured diagnostics rather than one provider call per minute.

If a Comments error and a DM latch are simultaneously live, the dedicated fingerprint continues to suppress unchanged DM provider calls. `last_error` may contain whichever actionable failure was most recently persisted; bounded source-specific logs and metrics must preserve both source outcomes for observability. The hotfix intentionally retains one aggregate delivery status and does not pretend that `last_error` is a complete per-source error ledger.

The latch resets when:

- `x_dms_v1` evaluates false;
- the account is removed from the canary allowlist;
- required DM configuration changes and therefore produces a new fingerprint;
- an operator deliberately runs an off→on gate cycle after correcting provider access.

Resetting means persisting `NULL` to `dm_subscription_forbidden_fingerprint`; overwriting or clearing `last_error` alone never resets the latch.

A 403 during deletion is not converted into a creation latch. It remains a cleanup failure and blocks promotion.

## 12. Webhook security and event admission

The existing security sequence remains:

1. Resolve the app-specific consumer secret from the opaque route key.
2. Read a bounded body.
3. Verify `x-twitter-webhooks-signature` over the exact raw bytes.
4. Parse only after signature verification.
5. Reject malformed recognized events with a non-2xx response.
6. Acknowledge and discard verified stale events outside the allowed time window.
7. Deduplicate accepted events by `(social_account_id, external_id)` through the existing Inbox upsert contract.

CRC remains:

- app-specific;
- rate-limited;
- HMAC-SHA256 using the correct consumer secret;
- unavailable when the route cannot resolve exactly one secret.

## 13. Event format boundary

### 13.1 Admitted current X Activity event

Only `data.event_type == "dm.received"` is admitted from the current X Activity envelope. It must include:

- a stable account tag;
- filter provider user ID;
- event ID;
- sender;
- timestamp;
- conversation ID or a derivable participant pair.

### 13.2 Admitted legacy event

Only `direct_message_events[].type == "message_create"` is admitted from the legacy Account Activity envelope. It must include:

- `for_user_id` provider X user ID;
- event ID;
- sender and recipient provider IDs;
- valid timestamp.

### 13.3 Explicitly excluded events

The parser returns no `ActivityEvent` for:

- `chat.received`;
- `chat.sent`;
- `chat.conversation_join`;
- `dm.sent`;
- `dm.read`;
- `dm.indicate_typing`;
- any other unrecognized Activity event.

No `chat.*` payload can be persisted with source `x_dm`, even if it contains fields resembling a legacy DM.

## 14. Exact-account routing and managed-user isolation

### 14.1 Tagged routing

For current X Activity events:

1. Parse the exact social account ID from `unipost:x:dm:<social_account_id>`.
2. Resolve that account through the webhook route key.
3. Require an active X account whose app mode belongs to that route.
4. Use the database account's workspace and customer-managed owner for insertion and notification.
5. Ignore any payload field that attempts to supply workspace or customer ownership.

### 14.2 Legacy fallback routing

For legacy envelopes without a subscription tag:

1. Treat `for_user_id` only as a provider X user ID.
2. Query only `social_accounts.external_account_id` within the resolved app route.
3. Do not compare it with `social_accounts.external_user_id`.
4. Keep the database query as `:many`, return every active exact provider-ID candidate in deterministic order, and do not add `LIMIT 1`.
5. Convert every returned row strictly; a malformed candidate is an error, not a row that may be silently skipped and turn an ambiguous set into one apparent match.
6. Enforce cardinality in the ingestion service before eligibility, admission, persistence, or notification: exactly one candidate is required.
7. Zero candidates return the existing not-found behavior without a write.
8. Multiple candidates return an explicit ambiguity error without a write or notification; no first-row-wins behavior is permitted.

This fail-closed rule is required because an untagged legacy payload cannot prove which duplicate local connection owns the event.

### 14.3 Downstream access isolation

Insertion always derives:

- `social_account_id` from the exact resolved database account;
- `workspace_id` from that account's profile;
- notification `external_user_id` from the database account, not the payload.

Existing Inbox authorization remains:

- managed-user scope is confined to `(authenticated workspace, connection_type='managed', external_user_id)`;
- another managed user cannot list, read, mutate, sync, or receive WebSocket notifications for the item;
- workspace owner/admin scope retains the intended aggregate view;
- a workspace API key can use explicit managed-user scope or authorized workspace aggregate according to the existing server-to-server contract.

## 15. Delivery status semantics

The existing single account delivery status is retained for this hotfix.

- `active`: at least one desired source is active and no desired source has an unresolved error.
- `pending`: eligible resources have not yet completed initial reconciliation.
- `paused_plan`, `paused_cap`, `paused_allowance`: existing common pause meanings.
- `error`: a desired source has a credential, evaluator, provider, routing, or persistence failure.

An `error` status does not imply both sources are down. Source-specific resource IDs, the dedicated DM forbidden fingerprint, and bounded source-specific logs distinguish the failing path. `last_error` is only the most recently persisted sanitized summary, so it may be overwritten by a later Comments or DM failure without changing latch behavior. For example, a DM 403 leaves the Comments rule and stream running while the account-level delivery status reports an actionable error and the dedicated fingerprint continues to suppress unchanged DM retries.

## 16. Configuration and secrets

### 16.1 Required managed-app values

| Variable | Comments | DMs | Handling |
| --- | --- | --- | --- |
| `TWITTER_BEARER_TOKEN` | Required | Required | Generate only if absent; store only in Railway secret configuration |
| `TWITTER_CONSUMER_SECRET` | Not required | Required | Reveal existing value if valid; do not rotate unnecessarily |
| `TWITTER_CLIENT_ID` | Required for stable app identity | Required | Existing non-secret app identifier |
| `X_INBOX_WEBHOOK_ROUTE_SECRET` | Stable app route identity | Required | Generate only if missing; never derive from consumer secret |
| `X_INBOX_WEBHOOK_URL` | Not required | Required | Environment-specific HTTPS base path |
| `X_INBOX_DM_CANARY_SOCIAL_ACCOUNT_IDS` | Not used | Required additional gate | Exact account allowlist; empty is fail closed |

### 16.2 Environment URLs

- Staging webhook base: `https://staging-api.unipost.dev/v1/webhooks/twitter`
- Production webhook base: `https://api.unipost.dev/v1/webhooks/twitter`
- Dev webhook base: `https://dev-api.unipost.dev/v1/webhooks/twitter`

The opaque route key is appended by the application and is never written into this document, logs, or PR text.

### 16.3 Credential operation policy

- Treat the production UniPost API key previously pasted into chat as exposed. Revoke or rotate it promptly on its own security timeline, before production acceptance, and do not reuse it for this hotfix. This action is independent of X credential configuration and does not authorize printing, recovering, or handling the old value.
- Do not show, copy, generate, regenerate, revoke, or rotate X credentials before the code/design gates require them.
- Generate the missing App-Only Bearer Token only after staging code gates are ready for canary configuration.
- Prefer revealing and storing the existing consumer secret over regenerating it.
- If a required credential already exists in Railway, do not replace it without evidence it is invalid.
- Never expose a secret in terminal output, screenshots, Git, CI logs, artifacts, PR text, or this task.

## 17. Observability

Add bounded structured logs for reconciliation decisions and failures with:

- environment/process mode;
- app mode;
- hashed app identity;
- social account ID;
- workspace ID where already allowed by existing internal logs;
- source (`comments` or `dm`);
- action (`ensure`, `reuse`, `replace`, `delete`, `skip`, `latch`);
- sanitized error class and HTTP status;
- whether flag, canary, scope, credential, plan, or spend eligibility blocked the source.
- whether a DM forbidden latch is absent, newly set, unchanged, or cleared, without logging the full fingerprint.

Do not log:

- access/bearer tokens;
- consumer secrets;
- webhook route secrets or full opaque callback URLs;
- webhook signatures;
- DM bodies;
- authorization headers;
- the previously pasted UniPost production API key.

The final acceptance evidence must record exact branch, commit SHA, workflow/job, deployment URL, provider resource IDs only where they are non-secret, and the relevant sanitized log excerpt.

## 18. TDD requirements

Implementation begins with failing tests. At minimum the suite must prove the following.

### 18.1 Worker desired-state tests

- Comments can be desired while DMs are not desired.
- DMs can be desired while Comments are not desired.
- Missing consumer secret disables DMs without disabling Comments.
- Missing webhook URL disables DMs without disabling Comments.
- DM provisioning occurs only when account active, plan allowed, DM scopes present, `x_dms_v1` true, exact account allowlisted, app bearer present, consumer secret present, webhook URL valid, and spend safety allows.
- With the global flag off, a Super Admin-owned workspace can evaluate `x_dms_v1=true`; a non-Super-Admin-owned workspace evaluates false and performs no DM ensure call.
- Losing Super Admin eligibility removes/does not recreate the DM subscription without deleting an eligible Comments rule.
- Flag false removes/does not create DM subscriptions without deleting the Comments rule.
- Flag evaluator error removes/does not create DM subscriptions, returns an error, and preserves Comments.
- Missing/invalid/empty allowlist removes/does not create DM subscriptions and preserves Comments.
- One malformed or empty entry in a non-empty multi-account allowlist invalidates the whole list; no valid subset is partially enabled.
- Known spend pause prevents both paid sources according to existing policy.
- Spend evaluation error creates neither source and does not start an unproven stream cycle.
- Route generation mismatch deletes and replaces the DM subscription in order.
- A healthy exact subscription is reused without creation or deletion.
- Cleanup remains exact-ID, leased, retryable, and idempotent.
- Two Comments-desired accounts sharing one app identity use one shared stream; disabling one keeps it running, disabling the last stops it once, and DM-only state changes do not churn it.
- An incomplete desired-state cycle preserves the existing shared stream set rather than applying a partial app aggregate.

### 18.2 Provider-client tests

- Webhook list/create/revalidate uses app bearer authentication.
- Activity list/create/delete uses app bearer authentication.
- Stable-tag matching checks event type, provider user ID, and webhook ID.
- Pagination still finds stable tags beyond the first page within the existing bound.
- Direct, wrapped, and array response shapes remain accepted where currently supported.
- HTTP 403 is exposed as a sanitized status-aware error.
- Tokens and authorization headers never appear in error text.

### 18.3 403 latch tests

- First unchanged 403 saves the stable latch fingerprint.
- A second reconcile with the same desired fingerprint does not call provider provisioning.
- Comments remain active during the DM latch.
- A later Comments error may overwrite `last_error` but cannot clear or bypass `dm_subscription_forbidden_fingerprint`.
- A later DM error may overwrite `last_error` without losing Comments failure diagnostics from source-specific logs/metrics.
- Flag off clears the latch and removes any stored DM subscription.
- Removing the account from the canary clears the latch.
- A deliberate off→on cycle after corrected configuration permits exactly one new attempt.
- App mode, route identity, webhook URL, or provider-user changes produce a new fingerprint and permit one controlled retry.
- A bearer-token or consumer-secret replacement alone does not enter the fingerprint; after correcting secret-only authorization, an operator must deliberately run the documented off→on gate cycle.
- Delete 403 remains a cleanup error and is not treated as successful removal.

### 18.4 Routing and ingestion tests

- Tagged `dm.received` routes to the exact social account from the tag.
- The payload cannot override the database workspace or managed owner.
- Legacy `for_user_id` matches exact provider `external_account_id`, not customer `external_user_id`.
- Zero legacy candidates produce no write.
- Multiple legacy candidates fail closed and produce no write or notification.
- The store returns all exact provider-ID candidates without `LIMIT 1`, and a malformed returned candidate fails closed instead of being skipped.
- The exact account's customer-managed owner is used for WebSocket notification.
- `chat.received` and all other `chat.*` events produce no `x_dm` event/item.
- Current `dm.received` and legacy `direct_message_events` fixtures remain accepted.
- Feature-off ingestion drops the private body before admission or persistence.

### 18.5 Managed-user isolation tests

- The receiving managed user can list and read the new X reply and DM.
- A second managed user in the same workspace cannot list, read, mutate, sync, or receive WebSocket delivery for them.
- Workspace owner/admin aggregate scope can see the intended items.
- Workspace API-key aggregate behavior remains unchanged.
- Cross-workspace and forged `workspace_id` access remains impossible.

### 18.6 Wiring tests

- `main.go` passes the same `featureFlagEvaluator` used by ingestion/capability into the delivery worker.
- `main.go` passes only parsed canary membership, not raw environment reads scattered through account reconciliation.
- The managed webhook handler routes remain registered.
- No user OAuth token is reintroduced into X Activity subscription management.

### 18.7 Migration and callback readiness tests

- The additive migration creates nullable `dm_subscription_forbidden_fingerprint` and the generated query/model/store paths round-trip it without changing existing resource IDs.
- Existing delivery-resource rows migrate with a null latch and retain their Comments/DM IDs, route generation, status, and `last_error`.
- A synthetic CRC probe receives HTTP 200 only when the route resolves the correct app-specific consumer secret, and its returned HMAC matches independently.
- A failed CRC probe blocks canary activation before any provider webhook create/revalidate call.

## 19. Local validation

Before the first push and again before every promotion PR:

1. Verify absolute worktree path and branch ownership.
2. Fetch origin and confirm the source branch contents.
3. From `api/`, run:

   `GOCACHE=/tmp/unipost-go-build go test ./...`

4. Run focused worker, provider-client, ingestion, handler, feature-flag, and tenant-isolation tests during TDD.
5. If dashboard/docs code changes unexpectedly become necessary, run the corresponding dashboard build/regression gates; otherwise keep the hotfix API-only.
6. List exact commits and files unique to the source branch.

Any failed, errored, timed-out, cancelled, skipped, or missing result is a hard stop.

## 20. Staging canary plan

### 20.1 Pre-merge

- Push only `hotfix-x-inbox-webhooks`.
- Open a PR from the owned hotfix branch to `staging`.
- Confirm local CI and all GitHub checks succeed on the exact head SHA.
- Audit all unique commits and files before merge.
- Do not configure a non-empty staging allowlist before the empty-gate build is deployed and healthy.

### 20.2 Empty-gate deployment

- Deploy with `X_INBOX_DM_CANARY_SOCIAL_ACCOUNT_IDS` empty.
- Confirm Comments behavior is unaffected by the empty DM gate.
- Confirm no DM subscription is created for any account.
- Confirm the evaluator and allowlist diagnostics contain no secrets.
- With the allowlist still empty, probe the exact deployed app-specific callback using a fresh synthetic CRC token; require HTTP 200 and independently verify the response HMAC before any provider webhook create/revalidate call is allowed.

### 20.3 Fixture selection

Use only a user-owned staging X test account. Resolve its exact staging `social_account_id`, provider X user ID, scopes, owner, and plan through read-only checks immediately before canary activation. While the staging/global `x_dms_v1` state is off, call the same backend evaluator used by the worker and require `(true, nil)` for that exact workspace; canary membership or Super Admin assumptions are not substitutes for this result.

If no suitable user-owned staging fixture exists, stop and ask the user to connect or designate one. Never substitute a customer account.

### 20.4 Provider-call cost gate

Before the first staging provider mutation, verify current X pricing/credit behavior for:

- Filtered Stream rule management and delivery;
- webhook creation/revalidation;
- X Activity subscription creation and delivered events.

If any call may consume credits, report the maximum bounded canary cost and obtain explicit approval. No DM backfill or replay is permitted.

### 20.5 Active canary

- Reconfirm the exact fixture workspace still evaluates `x_dms_v1=true` immediately before the configuration change.
- Reconfirm the synthetic CRC probe passes on the exact deployed SHA and app-specific route.
- Set the allowlist to exactly the staging fixture.
- Monitor the first reconciliation cycle and all deployments/checks.
- Verify one exact Comments rule where Comments are eligible.
- Verify one valid app-specific webhook.
- Verify one exact `dm.received` subscription for the fixture.
- Ask for fresh provider-side test events only at the acceptance step.
- Verify the reply and DM enter only the fixture's managed Inbox scope.
- Verify another managed user cannot access them.
- Verify owner/admin aggregate access.

### 20.6 Staging 403 hard stop

If any required X webhook or Activity operation returns 403:

1. Treat the canary as failed.
2. Confirm the persistent latch prevents repeated unchanged calls.
3. Clear the staging canary allowlist.
4. Confirm Comments remain healthy and no DM subscription is active.
5. Record environment, branch, SHA, workflow/job, exact operation, sanitized error, timestamps, deployment/run URLs, and whether a webhook was already created.
6. Do not promote to production.
7. Resume only after the authorization/capability cause is fixed and the full staging suite passes on a replacement SHA/configuration.

## 21. Production rollout and acceptance

### 21.1 Promotion

- After complete staging acceptance, create the production PR from `staging` to `main`.
- Do not merge the feature branch directly to `main`.
- Re-run required local CI-equivalent checks and audit all unique promotion commits/files.
- Merge only after every check succeeds.
- Wait for all production deployments and checks on the exact production SHA.

### 21.2 Credential configuration

Only after code and staging gates pass:

- generate the missing production App-Only Bearer Token once;
- reveal the existing production consumer secret only if Railway lacks it;
- do not regenerate valid OAuth keys or user tokens;
- configure missing Railway secrets without printing their values;
- configure the production webhook base URL;
- initially keep the production canary allowlist empty;
- confirm `x_dms_v1` global state remains off.
- before using any UniPost API key for acceptance, confirm the previously exposed production key has been revoked or rotated and use only a replacement through secret-safe handling.
- while the allowlist remains empty, run the synthetic CRC probe against the exact production app-specific route and require a valid independently checked response.

### 21.3 Production canary activation

Before changing the allowlist, prove through the worker's backend evaluator that the exact acceptance workspace returns `x_dms_v1=true` while the global flag remains off. Confirm its owner currently resolves as a Super Admin and that the production synthetic CRC probe passed on the exact deployed SHA. A false/error evaluator result or CRC mismatch is a hard stop and no provider mutation is attempted.

After the empty-gate production deployment is healthy, set:

`X_INBOX_DM_CANARY_SOCIAL_ACCOUNT_IDS=bc507960-aed6-4ae7-8568-27ad63cf5c58`

Then verify through read-only provider and database inspection:

- exactly one production Filtered Stream rule for `@unipostdev`;
- exactly one valid production webhook on the app-specific opaque UniPost route;
- exactly one `dm.received` subscription tagged for the exact social account and filtered to provider user `2039562772455809024`;
- local delivery state stores the exact rule/subscription IDs and current route generation;
- no other production account receives a new DM subscription.

### 21.4 Fresh-event acceptance

At this exact step, ask the user to send:

1. one fresh X reply/comment targeting `@unipostdev`;
2. one fresh legacy unencrypted DM between the user-owned test accounts.

Do not expect pre-subscription events to arrive and do not backfill them.

Verify:

- both events enter `sdk-inbox-x` with the exact social account ID;
- the DM source is `x_dm` and the reply source is `x_reply`;
- no `chat.*` event is present;
- a different managed user cannot list/read/mutate either item;
- `sdk-inbox-x` managed scope can access its items;
- owner/admin/API-key workspace aggregate can see the intended aggregate;
- WebSocket ownership notification uses `sdk-inbox-x` from the database.

### 21.5 Non-Inbox smoke checks

- Publishing: perform a safe read-only capability/health check and, only if an existing non-billable smoke fixture is available, the established minimal publishing smoke. Do not create unrelated production content without explicit authorization.
- Analytics: verify existing analytics endpoints/dashboard load for the fixture without mutation.
- Confirm no publishing, scheduling, or analytics code changed in the promotion audit.

## 22. Production rollback

Primary rollback is fail-closed configuration:

1. Clear `X_INBOX_DM_CANARY_SOCIAL_ACCOUNT_IDS` for the affected account.
2. If necessary, keep or set global `x_dms_v1` off.
3. Wait for reconciliation to remove the exact DM subscription.
4. Confirm Comments rule and stream remain active when independently eligible.
5. Confirm no new DM Inbox items arrive after subscription removal.

If the worker cannot remove the provider subscription:

- treat it as a failed rollback;
- stop promotion/completion;
- report the exact provider resource and sanitized deletion failure;
- use the X Developer Console only for the exact known subscription after user-visible evidence and authorization, never bulk deletion.

Code rollback is secondary and must follow the repository PR workflow. Credential rotation is not the default rollback because it can disrupt Comments and unrelated X product functions.

## 23. Sync back to dev

After production acceptance succeeds:

1. Fetch origin.
2. Sync the same owned `hotfix-x-inbox-webhooks` branch with the latest `origin/dev`.
3. If conflicts occur, stop and ask the user; do not resolve by discarding either side.
4. Re-run required local tests.
5. Push the same hotfix branch and open a PR to `dev`.
6. Complete Preview Acceptance on the exact head SHA.
7. Merge only after all Preview gates pass.
8. Wait for development deployment.
9. Verify the official dev domains and an explicitly user-owned dev fixture if one is available.

Dev configuration remains DM-fail-closed by default when no exact dev canary is designated.

## 24. Release blockers

The release stops immediately on any of the following:

- branch/worktree ownership mismatch;
- unrelated or unidentified commit/file in promotion content;
- required test failure, error, timeout, cancellation, skip, or missing result;
- validation against a SHA different from the proposed head;
- X 403 during required canary provisioning;
- fixture workspace does not evaluate `x_dms_v1=true` through the backend evaluator immediately before canary activation;
- invalid webhook CRC or signature verification;
- ambiguous legacy provider-user routing;
- managed-user isolation failure;
- missing required secret that cannot be configured safely;
- previously exposed production UniPost API key remains active or would need to be reused for acceptance;
- provider call with unknown/possibly billable cost before approval;
- no user-owned staging fixture;
- deployment/check still pending;
- publishing or analytics regression;
- conflict while syncing the hotfix to `dev`.

## 25. Definition of done

The hotfix is complete only when all of the following are true:

- PRD and implementation plan are approved.
- TDD tests cover every required desired-state, Super Admin evaluator precondition, whole-list canary parsing, dedicated 403 latch, shared-stream lifecycle, CRC readiness, routing cardinality, isolation, and XChat exclusion behavior.
- Full local API tests pass.
- The hotfix PR merges to staging after all exact-SHA checks pass.
- Staging canary and managed-user isolation pass with no 403.
- The staging-to-main production PR passes and deploys successfully.
- Production has exactly one intended rule, webhook, and DM subscription for the acceptance fixture.
- The exact production workspace evaluates `x_dms_v1=true` with the global flag off immediately before activation, and the deployed app-specific CRC probe passes before provider webhook creation.
- Fresh production reply and legacy DM arrive only in `sdk-inbox-x`.
- Another managed user cannot access them.
- Owner/admin/API-key aggregate behavior remains correct.
- Publishing and analytics smoke checks show no regression.
- The same hotfix is synced to dev through PR and dev acceptance passes.
- All triggered CI, Vercel, Railway, regression, and browser checks finish successfully.
- The production UniPost API key previously pasted into chat has been revoked or rotated before production acceptance; the exposed value is never repeated or reused.
