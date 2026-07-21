# Managed-User Inbox Authorization Hotfix Design

**Date:** 2026-07-20
**Status:** Approved direction; written-spec review pending
**Incident:** Inbox comments and DMs visible across managed users in one owner workspace

## Context

The first production hotfix stopped Meta webhook fan-out, removed arbitrary account fallbacks, hardened workspace ownership joins, and quarantined ambiguous historical Instagram rows. Production now has zero cross-managed-user duplicate event groups for the affected workspace, and new webhook events route only to exact provider account matches.

That containment did not add a managed-user authorization boundary. `GET /v1/inbox` still lists by authenticated workspace, and the other Inbox object and mutation routes authorize at workspace granularity. In the affected production workspace, current live Inbox rows belong to more than one `social_accounts.external_user_id`, so a workspace-scoped API response can contain multiple managed users' items even though every row is now assigned to the correct social account.

The customer's workspace API key is stored only on the customer's backend. Managed users do not receive the key or call UniPost directly. The customer backend authenticates its own app users and knows their `external_user_id`. Workspace owners and admins must retain an aggregate view across all managed users in both the UniPost Dashboard and the API.

The API key remains a privileged workspace credential: UniPost cannot independently prove that an `external_user_id` chosen by a valid key matches the customer app's current end user. Managed-user isolation therefore depends on the customer backend deriving that value from its authenticated app session and never exposing the key. UniPost's responsibility is to make the chosen scope explicit and prevent every object lookup, mutation, sync, and realtime path from escaping it.

## Goal

Make `(authenticated workspace_id, authorized external_user_id)` the mandatory authorization boundary for managed-user Inbox access while preserving an explicit workspace-wide aggregate scope for workspace owners and admins.

The boundary must cover comments and DMs on every supported Inbox platform and every read, mutation, sync, outbound-status, and realtime path. Guessing an Inbox item ID, social account ID, operation ID, or WebSocket subscription must never cross the selected managed-user scope.

## Non-goals

- Managed users will not receive UniPost workspace API keys.
- This hotfix will not treat a bare `external_user_id` supplied by an untrusted browser as authentication.
- It will not redesign customer-app login, issue end-user JWTs, or add a feature flag.
- It will not blanket-restore quarantined Instagram rows.
- It will not change exact webhook routing back to workspace enumeration or fallback behavior.

## Approaches considered

### 1. Mandatory explicit scope on the existing Inbox API — selected

Every API-key-authenticated Inbox request declares either workspace scope or managed-user scope. The authenticated workspace always comes from the API key; the caller never supplies it. Managed-user scope additionally supplies `external_user_id`, which the customer backend derives from its authenticated app session.

This is the smallest production-safe change because it reuses the existing routes and data model, fails closed when scope is missing, and supports both aggregate owner/admin access and managed-user access.

### 2. Duplicate the API below `/v1/users/{external_user_id}/inbox`

Nested routes make the resource hierarchy visible, but they duplicate every list, item, reply, mutation, sync, outbound-status, and WebSocket route. A path parameter still is not authentication, so the same authorization middleware would remain necessary. This adds surface area without strengthening the boundary.

### 3. Mint short-lived managed-user access tokens

A delegated token bound to `(workspace_id, external_user_id)` provides the strongest future browser/mobile model. It is unnecessary for the current server-to-server architecture and would expand this emergency change into token issuance, signing, rotation, revocation, and client migration. It remains a possible follow-up if managed users later call UniPost directly.

## Authorization contract

### Canonical scope

The API will construct one canonical scope after normal Clerk/API-key authentication:

```go
type InboxAccessScope struct {
	WorkspaceID    string
	Mode           InboxScopeMode // workspace or managed_user
	ExternalUserID string         // required only for managed_user
}
```

`WorkspaceID` is always derived from the authenticated Clerk session or API key. No Inbox handler or SQL query may accept a caller-provided workspace ID.

### API-key requests

Every API-key-authenticated `/v1/inbox` HTTP or WebSocket request must explicitly choose one of these forms:

```text
?inbox_scope=managed_user&external_user_id=<opaque-customer-user-id>
?inbox_scope=workspace
```

Rules:

- Missing `inbox_scope` fails with `400 INBOX_SCOPE_REQUIRED`.
- `managed_user` without exactly one nonempty `external_user_id` fails with `400 INVALID_INBOX_SCOPE`.
- `workspace` combined with `external_user_id` fails with `400 INVALID_INBOX_SCOPE`.
- An `external_user_id` with no ownership record in the authenticated workspace returns `404` without exposing another workspace's identifiers.
- A disconnected social account follows the existing Inbox-history retention policy; disconnection must not transfer or erase its managed-user ownership boundary.
- Workspace scope is allowed only when the API key creator's current workspace role is `owner` or `admin`.
- The customer's backend must derive scope from its authenticated app session. It must not forward an arbitrary browser query parameter unchanged.
- Scope omission never falls back to workspace-wide access.

### Clerk Dashboard requests

- Active workspace owners and admins default to workspace scope and retain the aggregate Inbox.
- Owners and admins may explicitly select managed-user scope to filter the Dashboard to one managed user.
- Workspace members without owner/admin role do not gain aggregate Inbox access through this hotfix.
- Dashboard and API use the same `InboxAccessScope` and the same database queries after scope construction.

## Data model and query boundary

`social_accounts.external_user_id` is the managed-user ownership key. One managed user may own several platform accounts, so access is not limited to one `social_account_id`.

Every Inbox query must derive ownership through:

```text
inbox_items.social_account_id
  -> social_accounts.id
  -> social_accounts.external_user_id
  -> profiles.workspace_id
```

Managed-user mode requires all of:

```sql
p.workspace_id = authenticated_workspace_id
AND i.workspace_id = authenticated_workspace_id
AND sa.external_user_id = authorized_external_user_id
```

Workspace mode requires the two workspace predicates and an owner/admin scope already authorized by middleware.

SQL receives an explicit scope mode; a null or empty external-user value must never mean workspace-wide access. Cross-scope item lookups return `404`, including when the item ID exists in the same workspace.

## Covered Inbox surfaces

The same scope is mandatory for:

- list and pagination;
- unread count;
- get item;
- media context;
- mark one read;
- mark all read;
- reply to comment or DM;
- update thread state;
- manual Inbox sync;
- X backfill, reply, DM, outbound operation, and outbound status;
- WebSocket connection and event delivery.

Replies and mutations must first load the target item through the scoped query and then load its social account through the same workspace and external-user boundary. Knowing another managed user's object ID must not authorize an operation.

Manual sync must enumerate only accounts whose `external_user_id` matches managed-user scope. Workspace scope may enumerate all accounts only after owner/admin authorization.

## Realtime isolation

The current WebSocket hub is workspace-oriented, so database query isolation alone is insufficient. A managed-user WebSocket subscription must be keyed by both workspace and external user.

Inbound notifications will carry or derive the owning `external_user_id` from the exact `social_account_id`. The hub will deliver:

- every workspace event to authorized owner/admin workspace subscriptions;
- only exact external-user events to managed-user subscriptions.

Sync-complete and count-update events must use the same scope. A managed-user connection must not receive event metadata, counts, or refresh signals caused solely by another managed user.

## Ingestion and write ownership

The deployed ingress hotfix remains authoritative:

- Instagram resolves only exact `instagram_webhook_user_id` matches.
- Facebook and Threads resolve only exact provider account IDs.
- X ingress remains exact-account routed.
- Unmatched events fail closed and create no Inbox row.
- Periodic recovery sync uses each social account's own provider token.

Exact routing no longer fans out across a workspace. It may still find more than one local connection for the same real provider account. The connection flow must reject assigning the same `(workspace, platform, provider account identity)` to two different nonempty `external_user_id` values. This check must run transactionally or under an account-identity advisory lock so concurrent Connect callbacks cannot create ambiguous managed ownership.

User-initiated writes are separately protected by `InboxAccessScope`; the webhook fix alone does not authorize mark-read, reply, thread-state, sync, or outbound operations.

## Error handling and telemetry

- Scope parse failures return stable 400 error codes without account details.
- Cross-scope objects and unknown managed users return 404.
- Unauthorized workspace aggregate requests return 403.
- Logs may contain internal workspace/account IDs and error classes, but not message bodies, access tokens, external-user emails, or raw provider payloads.
- Metrics distinguish missing scope, rejected cross-scope object access, and invalid managed ownership without logging the external-user value.

## Compatibility and rollout

This is intentionally fail closed and is a breaking change for API-key Inbox callers that omit scope.

- Clerk owner/admin Dashboard behavior remains workspace-wide by default.
- Customer-backend managed-user calls add `inbox_scope=managed_user` and `external_user_id` to every Inbox request, including WebSocket and object routes.
- Customer-backend owner/admin aggregate calls add `inbox_scope=workspace`.
- No feature flag is used; tenant isolation must not be optional.
- Existing correctly owned Inbox rows do not require migration or deletion. The authorization boundary applies immediately after deployment.

The documentation and generated API examples must clearly state that the workspace API key remains server-side and that the backend derives `external_user_id` from its own authenticated session.

## Test strategy

### Unit and SQL contract tests

- Scope parser rejects missing, contradictory, empty, and unknown scope values.
- API-key workspace scope requires current owner/admin role.
- Clerk owner/admin defaults to workspace scope.
- Every Inbox SQL query contains both derived workspace ownership and explicit scope predicates.
- Connection ownership rejects one provider account assigned to different managed users in one workspace.

### Handler authorization matrix

Create two managed users, A and B, in one workspace with separate social accounts and Inbox items:

- A list/count returns only A.
- B list/count returns only B.
- Owner/admin workspace scope returns A and B.
- A cannot get, read, reply to, update, sync, or inspect media for B's item.
- A cannot retrieve B's X outbound operation.
- Missing API-key scope fails closed.
- Guessed IDs return 404 and do not invoke provider adapters.

### Realtime tests

- A managed-user WebSocket receives A events and never B events.
- B receives B events and never A events.
- Owner/admin workspace WebSocket receives both.
- Sync-complete and unread-count notifications follow the same partition.

### Deployed acceptance

- Preview and staging use synthetic managed users A and B under one test workspace.
- Production verification uses a UniPost-owned test workspace or content-free aggregate queries; it must not log in as or click through a customer's managed-user account.
- The affected production workspace must retain zero cross-managed-user duplicate events, zero workspace ownership mismatches, and scoped query counts that reconcile to the owner/admin aggregate.
- Required CI, Railway, Vercel, deployed regression, and browser acceptance must pass on the exact promoted SHA.

## Recovery and rollback

The safe rollback is to keep scope enforcement and fix the failing scoped path. Reverting to implicit workspace-wide API-key Inbox access would reopen the disclosure and is prohibited.

Because the change does not rewrite existing Inbox rows, database rollback is limited to additive indexes or helper structures introduced for scoped lookup. Historical quarantine evidence remains untouched. If a customer integration has not supplied scope, it receives an explicit error rather than overbroad data.

## Acceptance criteria

The hotfix is complete only when:

1. API-key Inbox requests cannot omit scope.
2. Managed-user scope is enforced on every HTTP, mutation, sync, X, and WebSocket path.
3. Owner/admin aggregate behavior is identical in Dashboard and API.
4. Managed users A and B in one workspace cannot observe or mutate each other's Inbox data.
5. Webhook and periodic sync writes remain exact-account routed.
6. The same provider account cannot be assigned to two managed users in one workspace without an explicit ownership conflict.
7. Preview, staging, and production acceptance pass without using a customer managed-user login.
