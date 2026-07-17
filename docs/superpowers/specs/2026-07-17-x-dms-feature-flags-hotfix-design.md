# X DMs Feature Flags Hotfix Design

**Status:** Approved for production hotfix implementation

**Date:** 2026-07-17

**Owner:** UniPost Product and Engineering

## 1. Outcome

UniPost will add a small, database-backed feature flag system and a Super Admin-only page at `/admin/feature-flags`. The first registered flag is `x_dms_v1`.

The hotfix prevents ordinary Dashboard users, API keys, and managed users from accessing X Direct Message functionality while the integration is not production-ready. X Comments and X Post publishing remain available.

The system does not use Unleash or another external flag provider.

## 2. Flag semantics

Every registered flag has one persisted public-release boolean:

- `enabled_for_users = false`: the feature is available only to Super Admin users.
- `enabled_for_users = true`: the feature is available to ordinary users after all existing plan, account, permission, and capability gates pass.

There is no environment field, percentage rollout, user allowlist, workspace allowlist, schedule, or variant support. Each deployed environment reads the same flag key and semantics from its own database. A missing row, database error, or unknown key fails closed for ordinary users.

The initial migration registers `x_dms_v1` with `enabled_for_users = false`.

## 3. X DMs behavior

### 3.1 Flag off

When `x_dms_v1` is closed:

- Super Admin users can test X DM list, manual sync, and outbound replies.
- Ordinary Dashboard users cannot list, sync, or reply to `x_dm` items.
- API-key and managed-user requests cannot list, sync, or reply to `x_dm` items.
- X DM controls, filters, empty states, reconnect prompts, and capability claims are hidden from ordinary users.
- Existing X DM rows are not returned through ordinary-user list, grouping, unread-count, or websocket surfaces.
- X Comments and Post publishing continue unchanged.

The backend is authoritative. Direct API calls cannot bypass the flag.

### 3.2 Flag on

When `x_dms_v1` is open:

- Ordinary users receive X DM access only if their plan includes Inbox and the connected X account has the required OAuth scopes.
- Accounts with `dm.read` and `dm.write` become usable immediately.
- Accounts missing either DM scope receive the existing reconnect-required state and reconnect guidance.

### 3.3 OAuth scopes

The feature flag does not change the OAuth authorization request. New X connections and reconnects continue requesting:

```text
tweet.read tweet.write users.read offline.access media.write dm.read dm.write
```

While the flag is closed, an account missing DM scopes is not prompted to reconnect solely for DMs. If that account reconnects for another reason, it can obtain the complete scope set. When the flag later opens:

- an account that already has the DM scopes does not reconnect again;
- an account that still lacks the DM scopes reconnects once.

Comments continue using `tweet.read`, `tweet.write`, and `users.read`.

### 3.4 Delivery mode

`x_dms_v1` controls the existing OAuth 2.0 DM list, bounded manual sync, and send/reply functions. It does not enable real-time private DM subscriptions.

The X Inbox delivery worker stops automatically creating `dm.received` Activity subscriptions. It continues managing the Filtered Stream used for X Comments. Existing webhook parsing and forward-compatible subscription storage remain in place so X real-time DM delivery can be revisited after X supports the required OAuth 2.0 subscription flow.

The customer-facing DM delivery description is `Manual sync`; the product must not claim that real-time X DMs are available.

## 4. Backend architecture

Create `api/internal/featureflags` as the single backend authority.

The package contains:

- a compile-time registry of supported keys and their descriptions;
- a database store for public-release state;
- evaluation methods for authenticated Super Admin users and non-interactive/API-key requests;
- fail-closed behavior for unknown keys, missing rows, and read errors.

The first key is:

```text
x_dms_v1
```

The persistence table stores:

- `key` as the primary key;
- `enabled_for_users`;
- `updated_by`;
- `updated_at`.

No arbitrary key creation or deletion is exposed through the Admin API. Adding a new flag requires registering the key in code and adding or seeding its database row. This prevents typographical or orphaned production flags.

## 5. APIs and authorization

### 5.1 Admin APIs

Add:

- `GET /v1/admin/feature-flags`
- `PATCH /v1/admin/feature-flags/{key}`

Both routes require Clerk authentication, the existing Admin gate, and the existing Super Admin gate.

The PATCH body is:

```json
{
  "enabled_for_users": true
}
```

The server validates the key against the registry, writes the new state atomically, records the actor and timestamp, and creates an audit-log event containing only the flag key and old/new boolean state.

### 5.2 User evaluation

Restore `/v1/me/features` as the authenticated rollout-feature surface while retaining `plan_gates` compatibility:

```json
{
  "data": {
    "environment": "production",
    "provider": "database",
    "flags": {
      "x_dms_v1": false
    },
    "plan_gates": {
      "inbox": true,
      "audit_log": false
    }
  }
}
```

For a Super Admin, `flags.x_dms_v1` is `true` even when `enabled_for_users` is false. For an ordinary user it equals the persisted public-release state.

API-key and managed-user requests do not receive a Super Admin bypass.

## 6. X DM enforcement points

The hotfix enforces `x_dms_v1` at every relevant boundary:

1. X account capability calculation.
2. Inbox list and unread/count responses.
3. X manual sync and confirmation execution.
4. X DM outbound reply.
5. Websocket delivery of X DM items.
6. Dashboard source filters, conversation rendering, sync controls, and reconnect prompts.
7. X delivery reconciliation, where automatic private DM subscription creation remains disabled regardless of flag state.

The normalized error for a direct ordinary-user request while closed is:

```text
HTTP 403
code: FEATURE_NOT_AVAILABLE
message: X Direct Messages are not available yet.
```

The error does not disclose internal rollout configuration.

## 7. Admin page

Add `/admin/feature-flags` and place its navigation entry directly below Object Storage in the Admin `Overview` section.

The page uses the existing Admin shell and visual language. It is a compact operational list rather than a card grid. Each row shows:

- feature name and stable key;
- concise description;
- current state, `Internal only` or `Available to users`;
- last update time and actor when present;
- one accessible toggle labeled `Available to ordinary users`.

The page includes matching loading, empty, error, saving, and success states. A public-enablement change requires a confirmation dialog describing the impact. The toggle is disabled during the request and updates only after the backend confirms persistence.

The page and APIs require Super Admin access.

## 8. Documentation

Update:

- `docs/feature-flags-unleash.md` to document the new internal database-backed system and state that Unleash remains decommissioned;
- `docs/prd-x-credits-dms-comments.md` to record the OAuth 2.0-only manual DM scope and the feature flag launch gate;
- X DM API Reference and Guidance pages to state `Manual sync`, controlled availability, and no real-time subscription claim;
- `docs/x-inbox-operations.md` with flag verification, rollback, and incident procedures.

The flag documentation records:

- key: `x_dms_v1`;
- owner area: X Inbox;
- production default: internal only;
- rollback: set `enabled_for_users` to false;
- external dependency: X OAuth 2.0 support for private Activity subscriptions before real-time DMs can be reconsidered.

## 9. Testing

Backend tests prove:

- missing/unknown/read-error states fail closed for ordinary users;
- Super Admin evaluation remains enabled;
- Admin routes reject non-Super Admin users;
- updates persist and create an audit record;
- account capabilities suppress DM access and reconnect prompts while closed;
- list, sync, reply, unread/count, and websocket paths do not expose X DMs while closed;
- API-key and managed-user paths never receive the Super Admin bypass;
- Comments remain enabled;
- delivery reconciliation never calls private DM subscription creation.

Dashboard tests prove:

- Feature Flags appears below Object Storage;
- the page is Super Admin-only;
- loading, error, empty, saving, confirmation, and success states exist;
- ordinary users do not see X DM controls while closed;
- a Super Admin can still test X DMs;
- Comments remain visible.

Run the complete backend suite, Dashboard build, and Dashboard regression suite before promotion.

## 10. Release and rollback

Follow the repository hotfix flow:

1. Implement on `hotfix-x-dms-feature-flags` from latest `origin/staging`.
2. Merge into local `staging`, validate, and push `origin/staging`.
3. Wait for staging checks and deployments, then verify both internal-only and public-enabled paths.
4. Return the flag to internal-only before production promotion.
5. Create and merge the `staging` to `main` production PR.
6. Wait for production checks and deployments.
7. Verify production ordinary users cannot access X DMs, Super Admin can access the Admin page, and Comments/Post publishing remain healthy.
8. Sync the hotfix back to `dev`, validate, deploy, and verify development.

Emergency rollback is changing `x_dms_v1` to internal-only from `/admin/feature-flags`. Code rollback is used only if the Admin or evaluation path is unhealthy.
