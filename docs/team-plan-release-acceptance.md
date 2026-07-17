# Team Plan release acceptance

`dashboard/scripts/team-plan-acceptance.mjs` validates the deployed Team contract with synthetic Clerk identities and a disposable workspace. It is a release gate for development, staging, and production; it is not a customer-workspace diagnostic.

## Safety boundary

The runner refuses domains that do not exactly match `TEAM_ACCEPTANCE_ENV`. It also refuses identity emails whose local value does not start with `codex-team-acceptance-`. After bootstrap, it reads the workspace back from the API and refuses to proceed unless the workspace name starts with the same prefix.

The run creates three Clerk users (owner, admin, editor), one workspace, 25 additional profiles, two API keys, two accepted invitations/memberships, one Bluesky credential containing a non-functional test value, and the associated audit events. It never connects a social account or publishes content. Audit rows intentionally exist until the disposable workspace is deleted because the product does not expose audit deletion.

Cleanup runs in `finally` semantics after success and failure. It first attempts normal API cleanup, revokes every short-lived Clerk session, deletes all three Clerk users, then uses the environment database connection to remove any synthetic user rows left behind by delayed webhooks. The run fails if the cleanup ledger is non-empty or any workspace, profile, API key, or invitation with the run prefix remains.

## Required environment variables

- `TEAM_ACCEPTANCE_ENV`: `development`, `staging`, or `production`.
- `TEAM_ACCEPTANCE_API_URL`: the exact API domain for that environment.
- `TEAM_ACCEPTANCE_APP_URL`: the exact app domain for that environment.
- `TEAM_ACCEPTANCE_DATABASE_URL`: the public Postgres URL for that environment (Railway `DATABASE_PUBLIC_URL`, not the `.railway.internal` runtime URL). Never reuse one environment's URL for another.
- `TEAM_ACCEPTANCE_CLERK_SECRET_KEY`: the environment's Clerk secret (`sk_test_` for development/staging; `sk_live_` for production).
- `TEAM_ACCEPTANCE_OWNER_EMAIL`, `TEAM_ACCEPTANCE_ADMIN_EMAIL`, `TEAM_ACCEPTANCE_EDITOR_EMAIL`: three unique disposable addresses beginning with `codex-team-acceptance-`. The runner creates and deletes these Clerk users.

`psql` and the Playwright Chromium browser must be installed on the release workstation. Do not put secret values in shell history; load them from the deployment provider or a protected temporary environment file, then remove that file after the run.

## Commands

Run the contract tests before any deployed acceptance:

```bash
cd dashboard
npm run test:team-plan-acceptance
```

Staging:

```bash
TEAM_ACCEPTANCE_ENV=staging \
TEAM_ACCEPTANCE_API_URL=https://staging-api.unipost.dev \
TEAM_ACCEPTANCE_APP_URL=https://staging-app.unipost.dev \
npm run acceptance:team-plan
```

Production:

```bash
TEAM_ACCEPTANCE_ENV=production \
TEAM_ACCEPTANCE_API_URL=https://api.unipost.dev \
TEAM_ACCEPTANCE_APP_URL=https://app.unipost.dev \
npm run acceptance:team-plan
```

Development uses `https://dev-api.unipost.dev` and `https://dev-app.unipost.dev` with `TEAM_ACCEPTANCE_ENV=development`.

## Assertions

The runner verifies:

- the complete `/v1/limits` Team entitlement bundle and unlimited resource caps;
- Inbox and Audit Log plan gates;
- profile creation beyond Growth's 25-profile cap;
- admin/editor invitation acceptance and resolved roles;
- editor denials, owner self-removal/self-demotion safeguards, and role changes;
- owner/admin API-key creation, immediate role downgrade enforcement, and key access;
- platform credential create/delete with secret-safe responses and audit entries;
- membership, API-key, role, and credential audit actions without raw secrets;
- authenticated Team Dashboard routes for profiles, Analytics, Inbox, API Keys, Credentials, Members, and Audit Log without 5xx or application-error states;
- complete cleanup of all removable artifacts.

## Interrupted-run cleanup

If the process is interrupted before its cleanup handler finishes:

1. Revoke or delete the three synthetic users in the matching Clerk instance. This revokes their sessions.
2. Revoke API keys whose names start with the run prefix.
3. Delete the synthetic workspace owner from the matching environment database; foreign-key cascades remove the workspace and its profiles, memberships, keys, credentials, invitations, subscriptions, and audit entries.
4. Confirm no removable artifacts remain with a read-only query using the exact interrupted prefix:

```sql
SELECT 'workspace', id FROM workspaces WHERE name LIKE 'codex-team-acceptance-<run>%'
UNION ALL
SELECT 'profile', id FROM profiles WHERE name LIKE 'codex-team-acceptance-<run>%'
UNION ALL
SELECT 'api_key', id FROM api_keys WHERE name LIKE 'codex-team-acceptance-<run>%'
UNION ALL
SELECT 'invite', id FROM workspace_invites WHERE email LIKE 'codex-team-acceptance-<run>%';
```

Any cleanup error is a release blocker. Record the environment, run prefix, remaining resource IDs, and the failed cleanup operation before retrying.
