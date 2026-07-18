# UniPost internal feature flags

UniPost no longer uses Unleash. Rollout flags are stored in the UniPost database and evaluated by the backend so Dashboard, API-key traffic, managed users, background workers, and public marketing surfaces share one authority.

## Admin control

- Page: `/admin/feature-flags`
- API: `GET /v1/admin/feature-flags` and `PATCH /v1/admin/feature-flags/{key}`
- Access: Super Admin only
- Scope: one global value per deployed environment; the data model has no test/production variant dimension
- Audit: actual state changes append an immutable row to `feature_flag_changes`

The Admin UI uses these semantics:

- **ON:** the feature is available to regular users.
- **OFF:** the feature is unavailable to regular users. Workspaces owned by a Super Admin remain enabled for acceptance testing.

The frontend may hide customer UI using `GET /v1/me/features` or `GET /v1/public/features`, but sensitive behavior must remain backend-enforced.

## Registered flags

| Key | Owner area | Default | OFF behavior | Rollback action | Third-party dependency |
|---|---|---:|---|---|---|
| `x_dms_v1` | X Inbox | OFF | Regular workspaces cannot list, sync, or send `x_dm`; DM-only missing scopes do not require reconnect. X comments and publishing remain available. | Turn OFF in Admin; the backend stops DM access and removes any stale DM delivery intent. | X OAuth 2.0 supports direct DM reads/writes, but private Activity subscription creation is not production-ready. |
| `x_credits_billing_v1` | Billing | OFF | Managed X calls do not count against or block on the customer monthly X Credits balance. The independent 20 X publishes/account/day limit and internal inbound cost-safety cap remain active. | Turn OFF in Admin; customer monthly accounting and UI stop immediately while safety accounting continues. | X pay-per-use pricing and UniPost cost reconciliation. |

## Reconnect behavior

The OAuth 2.0 connection request continues to include `tweet.read`, `tweet.write`, `users.read`, `offline.access`, `media.write`, `dm.read`, and `dm.write`.

- Turning `x_dms_v1` OFF does not revoke already-granted scopes.
- Turning it ON does not require another reconnect for accounts that already granted the DM scopes.
- Accounts missing `dm.read` or `dm.write` reconnect once after the flag is ON.
- X comments require their existing read/write scopes and are not gated by `x_dms_v1`.

## Production isolation

Both seeded flags are OFF. A migration, deploy, or process restart must not turn them ON. The only supported global mutation is the Super Admin API/UI, and all customer enforcement must use the workspace evaluator rather than reading a frontend value or an environment variable.
