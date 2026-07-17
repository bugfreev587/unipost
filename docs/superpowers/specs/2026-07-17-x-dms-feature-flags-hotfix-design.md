# X DMs Feature Flags Hotfix Design

**Status:** Approved for production hotfix implementation

**Date:** 2026-07-17

**Owner:** UniPost Product and Engineering

## 1. Outcome

UniPost will add a small, database-backed feature flag system and a Super Admin-only page at `/admin/feature-flags`. The first registered flags are:

- `x_dms_v1`
- `x_credits_billing_v1`

The hotfix prevents ordinary workspaces from accessing X Direct Message functionality while the integration is not production-ready. It also lets UniPost keep X Credits accounting internal-only until Product opens the public balance experience. X Comments and X Post publishing remain available.

The system does not use Unleash or another external flag provider.

## 2. Flag semantics

Every registered flag has one persisted public-release boolean:

- `enabled_for_users = false`: the feature is available only to workspaces owned by a Super Admin.
- `enabled_for_users = true`: the feature is available to ordinary users after all existing plan, account, permission, and capability gates pass.

There is no environment field, percentage rollout, user allowlist, workspace allowlist, schedule, or variant support. Each deployed environment reads the same flag key and semantics from its own database. A missing row, database error, or unknown key fails closed for ordinary users.

The Super Admin workspace bypass applies consistently to Clerk Dashboard requests, workspace API keys, scheduled/background work, and managed users associated with that workspace. It is based on the owning workspace, not on an untrusted client claim.

The initial migration registers both `x_dms_v1` and `x_credits_billing_v1` with `enabled_for_users = false`.

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

## 4. X Credits behavior

### 4.1 Flag off

When `x_credits_billing_v1` is closed for an ordinary workspace:

- managed X API calls do not create, increment, finalize, reverse, or reconcile customer X Credits usage events;
- managed X calls are not blocked by monthly X Credits allowance;
- X Credits Billing balance/allowance controls and operation-capacity UI are hidden from ordinary users;
- customer-facing responses do not claim an X Credits charge;
- BYO X app behavior remains unchanged;
- the existing per-account 20-successful-X-publishes-per-UTC-day safety cap remains enforced;
- the internal workspace inbound-cost safety cap remains enforced to prevent uncontrolled upstream X API cost.

The internal inbound-cost cap is a safety control while customer accounting is closed. It is not shown as a customer X Credits balance and does not activate the public X Credits feature.

Super Admin-owned workspaces continue exercising the complete X Credits accounting path while the public flag is closed, including Dashboard, API-key, scheduled/background, and managed-user operations.

### 4.2 Flag on

When `x_credits_billing_v1` is open:

- ordinary managed-X operations use the existing weighted monthly X Credits allowance;
- usage events, monthly blocking, Billing surfaces, API response fields, and operation-capacity UI become available;
- the 20/day publish safety cap and inbound-cost cap remain independent controls.

The initial production state is closed. Product intends to open the public X Credits balance experience in a later month after production review.

## 5. Backend architecture

Create `api/internal/featureflags` as the single backend authority.

The package contains:

- a compile-time registry of supported keys and their descriptions;
- a database store for public-release state;
- evaluation methods for authenticated Super Admin users and non-interactive/API-key requests;
- fail-closed behavior for unknown keys, missing rows, and read errors.

The initial keys are:

```text
x_dms_v1
x_credits_billing_v1
```

The persistence table stores:

- `key` as the primary key;
- `enabled_for_users`;
- `updated_by`;
- `updated_at`.

No arbitrary key creation or deletion is exposed through the Admin API. Adding a new flag requires registering the key in code and adding or seeding its database row. This prevents typographical or orphaned production flags.

## 6. APIs and authorization

### 6.1 Admin APIs

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

### 6.2 User evaluation

Restore `/v1/me/features` as the authenticated rollout-feature surface while retaining `plan_gates` compatibility:

```json
{
  "data": {
    "environment": "production",
    "provider": "database",
    "flags": {
      "x_dms_v1": false,
      "x_credits_billing_v1": false
    },
    "plan_gates": {
      "inbox": true,
      "audit_log": false
    }
  }
}
```

For a member of a Super Admin-owned workspace, both flags evaluate to `true` even when `enabled_for_users` is false. For an ordinary workspace they equal the persisted public-release state. API-key, scheduled/background, and managed-user requests resolve the same workspace decision.

## 7. Enforcement points

### 7.1 X DMs

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

### 7.2 X Credits

The hotfix enforces `x_credits_billing_v1` at:

1. immediate X publishing usage admission and settlement;
2. scheduled/background X publishing;
3. X Inbox inbound and outbound customer-accounting paths;
4. X Credits Billing and API surfaces;
5. Dashboard and public operation-capacity presentation.

When closed, these paths bypass customer X Credits usage accounting but continue applying the 20/day publish cap and internal inbound-cost cap.

## 8. Admin page

Add `/admin/feature-flags` and place its navigation entry directly below Object Storage in the Admin `Overview` section.

The page uses the existing Admin shell and visual language. It is a compact operational list rather than a card grid. It initially shows rows for X DMs and X Credits billing. Each row shows:

- feature name and stable key;
- concise description;
- current state, `Internal only` or `Available to users`;
- last update time and actor when present;
- one accessible toggle labeled `Available to ordinary users`.

The page includes matching loading, empty, error, saving, and success states. A public-enablement change requires an in-app confirmation dialog describing the impact. It must use UniPost's existing `Dialog` primitive rather than `window.confirm` or another browser-native prompt.

The confirmation dialog:

- appears centered over a dimmed page overlay;
- identifies the feature and the requested `ON` or `OFF` target state;
- explains whether the feature will become available or unavailable to regular users;
- reminds the operator that Super Admin-owned workspaces retain acceptance access while the flag is `OFF`;
- offers explicit `Cancel` and `Turn ON` or `Turn OFF` actions;
- supports Escape, focus containment, and focus restoration through the shared dialog primitive;
- keeps the confirm action disabled and visibly loading while persistence is pending;
- remains open with a scoped error when the update fails;
- closes only after the backend confirms persistence and the row state has updated.

Only one pending flag change can exist at a time. Opening the dialog does not mutate server state. Canceling or dismissing it leaves the flag unchanged.

The page and APIs require Super Admin access.

## 9. Documentation

Update:

- `docs/feature-flags-unleash.md` to document the new internal database-backed system and state that Unleash remains decommissioned;
- `docs/prd-x-credits-dms-comments.md` to record the OAuth 2.0-only manual DM scope and the feature flag launch gate;
- X DM API Reference and Guidance pages to state `Manual sync`, controlled availability, and no real-time subscription claim;
- X Credits Reference, Guidance, Billing, and Pricing copy to state that customer accounting is controlled by `x_credits_billing_v1`;
- `docs/x-inbox-operations.md` with flag verification, rollback, and incident procedures.

The flag documentation records:

- keys: `x_dms_v1` and `x_credits_billing_v1`;
- owner areas: X Inbox and X Billing;
- production default: internal only;
- rollback: set `enabled_for_users` to false;
- external dependency: X OAuth 2.0 support for private Activity subscriptions before real-time DMs can be reconsidered.

## 10. Testing

Backend tests prove:

- missing/unknown/read-error states fail closed for ordinary users;
- Super Admin evaluation remains enabled;
- Admin routes reject non-Super Admin users;
- updates persist and create an audit record;
- account capabilities suppress DM access and reconnect prompts while closed;
- list, sync, reply, unread/count, and websocket paths do not expose X DMs while closed;
- Super Admin-owned workspace API keys, background operations, and managed users receive the same internal bypass;
- ordinary workspace API keys, background operations, and managed users remain closed;
- Comments remain enabled;
- delivery reconciliation never calls private DM subscription creation.
- X Credits accounting and monthly blocking are bypassed for ordinary workspaces while closed;
- X Credits accounting remains active for Super Admin-owned workspaces while closed;
- the 20/day publish cap and internal inbound-cost cap remain active in both states.

Dashboard tests prove:

- Feature Flags appears below Object Storage;
- the page is Super Admin-only;
- loading, error, empty, saving, confirmation, and success states exist;
- the confirmation is an accessible centered in-app dialog and no `window.confirm` call remains;
- canceling the dialog performs no update, while confirming shows a pending state and closes only after success;
- ordinary users do not see X DM controls while closed;
- a Super Admin can still test X DMs;
- ordinary users do not see X Credits Billing/allowance/capacity UI while closed;
- a Super Admin-owned workspace can still test the X Credits path;
- Comments remain visible.

Run the complete backend suite, Dashboard build, and Dashboard regression suite before promotion.

## 11. Release and rollback

Follow the repository hotfix flow:

1. Implement on `hotfix-x-dms-feature-flags` from latest `origin/staging`.
2. Merge into local `staging`, validate, and push `origin/staging`.
3. Wait for staging checks and deployments, then verify both internal-only and public-enabled paths.
4. Return both flags to internal-only before production promotion.
5. Create and merge the `staging` to `main` production PR.
6. Wait for production checks and deployments.
7. Verify production ordinary users cannot access X DMs or customer X Credits accounting, Super Admin-owned workspaces can test both, the 20/day and inbound safety caps remain active, and Comments/Post publishing remain healthy.
8. Sync the hotfix back to `dev`, validate, deploy, and verify development.

Emergency rollback is changing either flag to internal-only from `/admin/feature-flags`. Code rollback is used only if the Admin or evaluation path is unhealthy.
