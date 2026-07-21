# Inbox Comments and DMs Tenant-Isolation Hotfix Design

## Incident and goal

Instagram webhook deliveries are currently copied into every active Instagram social account instead of only the account identified by the webhook entry. This assigns one managed user's comments and DMs to other managed users, and can also cross workspace boundaries.

Production evidence collected without reading message bodies confirms the defect for the workspace owned by `guyhass02@gmail.com`: five Instagram comment event groups and twenty-eight Instagram DM event groups were stored under two different managed users. Across the production database, a single Instagram event has been copied to as many as sixty-five social accounts in fifteen workspaces.

The hotfix must stop new cross-account writes, remove already-exposed suspect rows from every user-visible Inbox surface without destroying the evidence, restore only account-owned data from the upstream APIs, and complete the repository's hotfix promotion flow.

## Root cause

`MetaWebhookHandler.handleInstagramEntry` ignores the Instagram user ID in `entry.id`. It calls `FindAllActiveAccountsByPlatform("instagram")` and writes every comment and DM to every returned account. The uniqueness constraint on `(social_account_id, external_id)` prevents duplicates only within one account, so it does not prevent the same event from being copied across accounts or workspaces.

Threads and Facebook webhook routing also contain an unsafe fallback that assigns an unmatched entry to an arbitrary active account. Although the confirmed incident rows are Instagram rows, this fallback has the same isolation failure mode and must be removed in the same narrowly scoped hotfix.

The working reference is X Inbox ingestion: it resolves an event to an exact external user/app account set and does not choose an unrelated account when routing data is missing.

## Containment design

### Exact webhook routing

All Meta Inbox entry routing will use the existing indexed lookup by `(platform, external_account_id)` and the webhook's `entry.id`.

- Instagram will resolve all active Instagram accounts whose stored `external_account_id` exactly equals `entry.id`. It will no longer enumerate all Instagram accounts.
- Threads will use the same exact lookup and will no longer fall back to an arbitrary Threads account.
- Facebook will retain its exact multi-account lookup for the Page ID and will no longer fall back to an arbitrary Facebook account.
- An unmatched entry will be acknowledged to Meta after emitting a structured warning containing only the platform and unmatched external account ID. No Inbox row or WebSocket event will be created.
- Multiple exact matches remain supported because the same real social account may be intentionally connected in more than one workspace. Exact matches all refer to the same upstream account identity; matches with a different `external_account_id` are never accepted.

The periodic per-account Inbox sync remains the recovery path for an event whose webhook identifier cannot be matched. Losing real-time delivery for an unmatched account is safer than assigning private content to an unrelated tenant.

### Recoverable quarantine of existing leaked rows

The code deployment and data cleanup are intentionally ordered to avoid the old webhook process recreating leaked rows during deployment:

1. Deploy and verify exact routing first.
2. Create an incident quarantine table that stores the original Inbox row as JSONB together with its original row ID, source, social account ID, incident key, and quarantine timestamp.
3. In one transaction, identify suspect Instagram rows where the same `(source, external_id)` exists under more than one distinct `social_accounts.external_account_id`.
4. Copy every suspect row to the quarantine table, then delete those rows from `inbox_items` in the same transaction.
5. The operation is idempotent through a unique constraint on `(incident_key, original_inbox_item_id)` and produces before/moved/remaining counts for the release record.

Instagram comment and DM external IDs identify one upstream event. The same event appearing under different Instagram account IDs is therefore evidence of the fan-out defect. Every copy is quarantined because the stored rows do not retain enough trustworthy routing evidence to identify the one correct account locally. Quarantining all copies fails closed and preserves a reversible record.

The quarantine must run only after the fixed API deployment is serving traffic. It will first run in staging, then in production after the production deployment passes health checks. No message body will be printed in command output or release evidence.

### Account-scoped restoration

After quarantine, the existing per-account Inbox sync worker will fetch comments and DMs separately using each social account's own access token. Rows can therefore be recreated only beneath the account that the upstream API authorizes.

Acceptance will compare counts by `(source, external_id)` and distinct `external_account_id`; no restored Instagram event may span different Instagram accounts. Quarantined rows that are outside the providers' available sync window remain quarantined rather than being guessed back into an account. Restoration will never copy a quarantined row directly back to the live table without independent upstream ownership evidence.

## Defense-in-depth read checks

Inbox list and direct-object queries will continue to enforce the authenticated workspace boundary. Tests will additionally prove that an Inbox row cannot be returned or mutated when its social account resolves through its profile to a different workspace, even if the denormalized `inbox_items.workspace_id` is corrupt. Unauthorized direct-object requests must return the existing not-found response so object existence is not disclosed.

This hotfix does not treat a caller-supplied `external_user_id` as authentication. A managed user identity is not currently encoded in UniPost API-key authentication, so trusting a query parameter would only move the IDOR. The immediate incident is contained at the authoritative account-routing layer.

## Tests

Test-driven implementation will add failing tests before production code changes.

- An Instagram entry routes only to accounts with the exact external account ID.
- An Instagram entry with no exact account match creates no Inbox writes.
- Threads and Facebook unmatched entries create no Inbox writes and never call an arbitrary-account fallback.
- A matching event may fan out only to duplicate connections that share the same exact external account ID.
- Comment and DM routing receive the same isolated account set.
- Workspace list, get, mark-read, mark-all-read, media-context, reply, thread-state, sync, X outbound status, and WebSocket paths retain their existing authorization behavior; direct-object tests cover a mismatched social-account workspace.
- The quarantine query selects cross-account Instagram duplicates, does not select copies tied to the same exact external account ID, is idempotent, preserves the full original row in JSONB, and leaves no selected row live.
- The complete backend suite runs with `GOCACHE=/tmp/unipost-go-build go test ./...`.

## Deployment and acceptance

The owned `hotfix-inbox-comments-idor` branch is based on the latest `origin/staging`. After local validation:

1. Push the hotfix branch and open a pull request to `staging`.
2. Audit the exact commits and files unique to the branch.
3. Wait for every required GitHub, Railway, and Vercel check on the exact head SHA.
4. Merge only after all checks pass, wait for staging deployment, and verify exact routing plus the quarantine dry run in staging.
5. Execute the staging quarantine, wait for account-scoped sync, and verify no cross-account event groups remain.
6. Open `staging` to `main`, re-audit promotion content, and wait for all production checks and deployments.
7. Verify the fixed API version is serving production before executing the production quarantine.
8. Record quarantine counts without bodies, wait for account-scoped restoration, verify the affected workspace and the global duplicate query, and exercise the critical Inbox flow in production.
9. Sync the owned hotfix branch with the latest `origin/dev`, rerun the required suite, open the hotfix pull request to `dev`, complete Preview Acceptance, merge, wait for development deployment, and verify the development environment.

Any failed, skipped, cancelled, timed-out, or wrong-SHA check stops the promotion. No feature flag is added because the user did not request one and a tenant-isolation control must not be optional.

## Rollback

Code rollback restores the previous webhook behavior and is therefore unsafe once the defect is confirmed. If the exact matcher unexpectedly drops legitimate webhook deliveries, keep the exact matcher in place and rely on per-account sync while investigating identifier normalization.

The data quarantine is reversible from the preserved JSONB records, but restoration requires independently verified account ownership. A blanket restore is prohibited because it would recreate the disclosure.
