# Inbox Comments and DMs Tenant-Isolation Hotfix Design

## Incident and goal

Instagram webhook deliveries are currently copied into every active Instagram social account instead of only the account identified by the webhook entry. This assigns one managed user's comments and DMs to other managed users, and can also cross workspace boundaries.

Production evidence collected without reading message bodies confirms the defect for the workspace owned by `guyhass02@gmail.com`: five Instagram comment event groups and twenty-eight Instagram DM event groups were stored under two different managed users. Across the production database, a single Instagram event has been copied to as many as sixty-five social accounts in fifteen workspaces.

The hotfix must stop new cross-account writes, remove already-exposed suspect rows from every user-visible Inbox surface without destroying the evidence, restore only account-owned data from the upstream APIs, and complete the repository's hotfix promotion flow.

## Root cause

`MetaWebhookHandler.handleInstagramEntry` ignores the Instagram user ID in `entry.id`. It calls `FindAllActiveAccountsByPlatform("instagram")` and writes every comment and DM to every returned account. The uniqueness constraint on `(social_account_id, external_id)` prevents duplicates only within one account, so it does not prevent the same event from being copied across accounts or workspaces.

There is also an Instagram identifier mismatch. The connection flows request and store Meta's app-scoped `id`, while Instagram webhook `entry.id` is the professional account `user_id`. Production logs contained two recent webhook entry IDs, and neither matched any active account's stored `external_account_id`. Replacing the fan-out with a lookup on the currently stored value would fail closed, but it would also stop real-time delivery for every observed Instagram webhook. The hotfix therefore has to persist the correct webhook routing identifier before exact routing can be considered functional.

Threads and Facebook webhook routing also contain an unsafe fallback that assigns an unmatched entry to an arbitrary active account. Although the confirmed incident rows are Instagram rows, this fallback has the same isolation failure mode and must be removed in the same narrowly scoped hotfix.

The working reference is X Inbox ingestion: it resolves an event to an exact external user/app account set and does not choose an unrelated account when routing data is missing.

## Containment design

### Exact webhook routing

All Meta Inbox entry routing will use a persisted, provider-defined webhook routing identifier and the webhook's `entry.id`.

- Instagram will resolve all active Instagram accounts whose trusted `instagram_webhook_user_id` exactly equals `entry.id`. It will no longer enumerate all Instagram accounts.
- Threads will use the same exact lookup and will no longer fall back to an arbitrary Threads account.
- Facebook will retain its exact multi-account lookup for the Page ID and will no longer fall back to an arbitrary Facebook account.
- An unmatched entry will be acknowledged to Meta after emitting a structured warning containing only the platform and unmatched external account ID. No Inbox row or WebSocket event will be created.
- Multiple exact matches remain supported because the same real social account may be intentionally connected in more than one workspace. Exact matches all refer to the same upstream account identity; matches with a different `external_account_id` are never accepted.

The periodic per-account Inbox sync remains the recovery path for an event whose webhook identifier cannot be matched. Losing real-time delivery for an unmatched account is safer than assigning private content to an unrelated tenant.

### Instagram webhook identifier mapping

Meta's Instagram Login contract exposes both `id` and `user_id`: `id` is app-scoped, while `user_id` is the professional account identifier delivered as webhook `entry.id`. The hotfix will preserve the existing `external_account_id` to avoid changing account identity, and store the separately fetched `user_id` in social-account metadata as `instagram_webhook_user_id`.

- Managed Instagram connection and native Instagram OAuth will request both `id` and `user_id` and persist the webhook identifier with the account.
- The Instagram webhook subscription/sync worker will refresh `user_id` with that account's own access token and persist it before declaring the subscription ready. This safely backfills existing active connections.
- A partial expression index on the active Instagram metadata value will support exact webhook routing. A separate active `(platform, external_account_id)` index will support exact Threads and Facebook routing.
- The Instagram router may use `external_account_id` only as a validated compatibility fallback when it is already equal to the fetched `user_id`; it may never guess, enumerate, or choose the newest account.
- Until an existing account has been backfilled, its unmatched real-time webhook is dropped with content-free structured telemetry and its account-scoped periodic sync remains the recovery path.

Before quarantine in each environment, a dry run must report mapping coverage for active Instagram accounts and prove that recently observed webhook IDs resolve only to exact mapped accounts. Missing mappings do not permit fan-out and block the data-cleanup step for affected accounts until the account-scoped backfill has been attempted and its result recorded.

### Recoverable quarantine of existing leaked rows

The code deployment and data cleanup are intentionally ordered to avoid the old webhook process recreating leaked rows during deployment:

1. Deploy and verify exact routing first.
2. Create an incident quarantine table that stores the original Inbox row as JSONB together with its original row ID, source, social account ID, incident key, and quarantine timestamp.
3. In one transaction, identify suspect Instagram rows where the same `(source, external_id)` exists under more than one distinct `social_accounts.external_account_id`.
4. Copy every suspect row to the quarantine table, then delete those rows from `inbox_items` in the same transaction.
5. The operation is idempotent through a unique constraint on `(incident_key, original_inbox_item_id)` and produces before/moved/remaining counts for the release record.

The schema migration is additive and performs no Inbox data mutation. The operator script defaults to a rolled-back dry run. An apply requires an explicit flag, the exact candidate count and candidate-set digest from the immediately preceding dry run, confirmation that the environment has a usable recovery snapshot/PITR, and explicit user approval for production. It locks the captured rows, preserves their complete JSONB before deletion, verifies preserved/deleted/remaining counts inside one transaction, and rolls back on any mismatch. Migration rollback must refuse to drop a non-empty evidence table.

Instagram comment and DM external IDs identify one upstream event. The same event appearing under different Instagram account IDs is therefore evidence of the fan-out defect. Every copy is quarantined because the stored rows do not retain enough trustworthy routing evidence to identify the one correct account locally. Quarantining all copies fails closed and preserves a reversible record.

The quarantine must run only after the fixed API deployment is serving traffic. It will first run in staging, then in production after the production deployment passes health checks. No message body will be printed in command output or release evidence.

### Account-scoped restoration

After quarantine, the existing per-account Inbox sync worker will fetch comments and DMs separately using each social account's own access token. Rows can therefore be recreated only beneath the account that the upstream API authorizes.

Acceptance will compare counts by `(source, external_id)` and distinct `external_account_id`; no restored Instagram event may span different Instagram accounts. Quarantined rows that are outside the providers' available sync window remain quarantined rather than being guessed back into an account. Restoration will never copy a quarantined row directly back to the live table without independent upstream ownership evidence.

## Defense-in-depth read checks

Inbox list and direct-object SQL queries will be changed to derive tenant ownership through `inbox_items.social_account_id -> social_accounts.profile_id -> profiles.workspace_id`, rather than trusting only the denormalized `inbox_items.workspace_id`.

- List, count, mark-all-read, get, mark-read, and thread-state mutations will require the derived profile workspace to equal the authenticated workspace.
- Media-context account loading will use the workspace-scoped social-account lookup after the Inbox item passes the same derived ownership check.
- Reply, sync, X outbound-status, and WebSocket paths will be regression-tested through their existing authenticated entry points; any path that directly loads an Inbox item will use the hardened query.
- Unauthorized direct-object requests will retain the existing not-found response so object existence is not disclosed.

These checks protect against a corrupt or forged denormalized workspace stamp. They are defense in depth, not a substitute for correct ingestion: a wrongly selected social account genuinely belongs to its own workspace, so only exact webhook routing prevents that bad write.

This hotfix does not treat a caller-supplied `external_user_id` as authentication. A managed user identity is not currently encoded in UniPost API-key authentication, so trusting a query parameter would only move the IDOR. The immediate incident is contained at the authoritative account-routing layer.

## Tests

Test-driven implementation will add failing tests before production code changes.

- Managed and native Instagram connection flows persist the provider's `user_id` separately from its app-scoped `id`.
- Existing Instagram accounts backfill the webhook `user_id` using only their own decrypted token.
- An Instagram entry routes only to active accounts with the exact mapped webhook `user_id`.
- An Instagram entry with no exact account match creates no Inbox writes.
- Threads and Facebook unmatched entries create no Inbox writes and never call an arbitrary-account fallback.
- A matching event may fan out only to duplicate connections that share the same exact provider webhook account ID.
- Comment and DM routing receive the same isolated account set.
- Workspace list, count, get, mark-read, mark-all-read, media-context, reply, thread-state, sync, X outbound status, and WebSocket paths retain their existing authorization behavior; SQL tests prove a mismatched social-account workspace cannot be read or mutated even when `inbox_items.workspace_id` is forged to match the caller.
- The active Instagram webhook-ID expression index and the active platform/external-account index support their exact lookup predicates.
- The quarantine query selects cross-account Instagram duplicates, does not select copies tied to the same exact external account ID, is idempotent, preserves the full original row in JSONB, and leaves no selected row live.
- The complete backend suite runs with `GOCACHE=/tmp/unipost-go-build go test ./...`.

## Deployment and acceptance

The owned `hotfix-inbox-comments-idor` branch is based on the latest `origin/staging`. After local validation:

1. Push the hotfix branch and open a pull request to `staging`.
2. Audit the exact commits and files unique to the branch.
3. Wait for every required GitHub, Railway, and Vercel check on the exact head SHA.
4. Merge only after all checks pass, wait for staging deployment, verify Instagram identifier backfill and exact routing, then run the quarantine dry run in staging.
5. Execute the staging quarantine only after mapping coverage, snapshot readiness, and dry-run count/digest are recorded; wait for account-scoped sync and verify no cross-account event groups remain.
6. Open `staging` to `main`, re-audit promotion content, and wait for all production checks and deployments.
7. Verify the fixed API version is serving production, run and record the Instagram identifier-mapping backfill/coverage check, confirm recovery readiness, and present the production dry-run count/digest for explicit user approval before executing the production quarantine.
8. Record quarantine counts without bodies, wait for account-scoped restoration, verify the affected workspace and the global duplicate query, and exercise the critical Inbox flow in production.
9. Sync the owned hotfix branch with the latest `origin/dev`, rerun the required suite, open the hotfix pull request to `dev`, complete Preview Acceptance, merge, wait for development deployment, and verify the development environment.

Any failed, skipped, cancelled, timed-out, or wrong-SHA check stops the promotion. No feature flag is added because the user did not request one and a tenant-isolation control must not be optional.

## Customer impact and incident response

Quarantine intentionally removes every locally ambiguous copy, including the potentially correct copy, because local rows do not retain trustworthy ownership evidence. Provider sync windows may not return older events, so some historical comments or DMs may remain unavailable while preserved in the incident table. This temporary loss of availability is an explicit tradeoff for stopping cross-tenant disclosure; it must be included in the operator/customer communication and release record.

The confirmed cross-workspace exposure must be preserved as a security incident and separately assessed by the authorized privacy/legal owner for notification, retention, and evidence-handling obligations. This hotfix will produce content-free counts and identifiers needed for that assessment, but will not send customer or regulatory notifications without explicit authority.

## Out of scope

The broad WebSocket origin configuration and wider authorization architecture deserve separate security follow-up, but they are not causal to this ingestion fan-out and will not be changed in this narrowly scoped production hotfix. Preview Acceptance for the required development sync-back remains mandatory under the repository workflow.

## Rollback

Code rollback restores the previous webhook behavior and is therefore unsafe once the defect is confirmed. If identifier mapping or exact matching unexpectedly drops legitimate webhook deliveries, keep exact fail-closed routing in place and rely on per-account sync while investigating. The global fan-out and arbitrary-account fallback must never be restored as a rollback mechanism.

The data quarantine is reversible from the preserved JSONB records, but restoration requires independently verified account ownership. A blanket restore is prohibited because it would recreate the disclosure.
