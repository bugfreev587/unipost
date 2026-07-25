# Social Connection and Profile Bindings Design

**Date:** 2026-07-24
**Status:** Proposed for written review; conversational decisions approved
**Target branch:** `dev-social-account-profile-isolation`

## 1. Summary

UniPost currently stores OAuth credentials, provider identity, connection state,
and Profile ownership in one `social_accounts` row. This prevents the same
physical provider account from being used by more than one Profile in the same
Workspace.

This design separates the physical provider connection from its Profile-scoped
account bindings:

- `social_connections` is the Workspace-scoped physical connection. It owns
  credentials, provider identity, refresh state, shared account status, and
  webhook routing identity.
- `social_accounts` remains the public, Profile-scoped account resource. Each
  row binds one Profile to one `social_connection` and keeps its existing public
  `account_id`.
- One connection may be bound to one or more Profiles in the same Workspace.
- One UniPost Post may use at most one Profile binding for a given physical
  connection. Selecting two different account IDs backed by the same connection
  in one Post is a validation error.

The Post API remains backward compatible. Clients continue to discover accounts
by Profile and publish with `platform_posts[].account_id`. No `profile_id` or
`connection_id` is added to the publish request.

## 2. Goals

1. Allow a Workspace to bind one physical provider account to multiple Profiles.
2. Store and refresh one authoritative credential set per physical provider
   account.
3. Preserve all existing public `social_account_id` values and Profile-first
   account discovery flows.
4. Keep each Post, post result, and post analytics record attributed to exactly
   one selected Profile binding per physical connection.
5. Prevent duplicate physical publishing caused by selecting the same connection
   through multiple Profiles in one Post.
6. Keep Workspace-level subscription, quota, credits, API keys, and billing
   unchanged.
7. Treat inbox data and account-level metrics as shared physical-connection data.

## 3. Non-goals

1. Multiple Workspaces under an account-level subscription or shared quota.
2. A single Post attributed to multiple Profiles for the same physical
   connection.
3. Silent merging or first-item-wins behavior for duplicate connection targets.
4. Moving the existing public `account_id` to Workspace scope.
5. Exposing `connection_id` as a required public API identifier in V1.
6. Provider-level duplicate-content detection across separate Post requests.
7. Independent credentials, inboxes, or account metrics per Profile binding.

## 4. Current behavior

Today:

```text
Workspace
└── Profile
    └── social_accounts row
        ├── public account ID
        ├── OAuth tokens
        ├── provider identity
        └── connection status
```

`social_accounts.profile_id` gives each account one Profile owner. Account lists
filter directly by `profile_id`. Post creation accepts `account_id` and derives
`social_posts.profile_ids` from the selected account rows.

Normal connection flows also deduplicate at Workspace scope:

- dashboard OAuth finds an existing `(platform, external_account_id)` anywhere
  in the Workspace and refreshes that row;
- direct connect returns `ACCOUNT_ALREADY_CONNECTED`;
- managed Connect returns an ownership conflict when the matching account belongs
  to another Profile.

The database has some Profile-scoped uniqueness, but the supported product
behavior is still one physical provider account in one Profile per Workspace.

## 5. Chosen architecture

```text
Workspace
├── SocialConnection: Twitter @unipost
│   ├── credentials and refresh state
│   ├── provider identity and webhook routing
│   └── shared connection status
│
├── Profile: Development
│   └── SocialAccount sa_twitter_dev ───────┐
├── Profile: Staging                        ├── same SocialConnection
│   └── SocialAccount sa_twitter_staging ───┤
└── Profile: Production                     │
    └── SocialAccount sa_twitter_prod ──────┘
```

### 5.1 SocialConnection responsibilities

The physical connection owns:

- `workspace_id`;
- platform and canonical provider identity;
- encrypted access and refresh tokens;
- token expiry and last refresh time;
- granted scopes;
- connection type and provider-app mode;
- provider account name, avatar, and shared metadata;
- active, reconnect-required, or disconnected status;
- provider webhook identity and subscription lifecycle;
- physical-account rate-limit and serialization identity.

### 5.2 SocialAccount responsibilities

The public Profile binding owns:

- the existing public `social_account_id`;
- `profile_id`;
- `connection_id`;
- binding status and binding timestamps;
- optional Profile-specific display overrides added by future features.

An active binding is unique by `(profile_id, connection_id)`.

### 5.3 Ownership invariants

1. A SocialConnection belongs to exactly one Workspace.
2. A SocialAccount binding belongs to exactly one Profile.
3. The binding's Profile must belong to the same Workspace as the connection.
4. A Profile may have at most one active binding to a connection.
5. A connection may have multiple active bindings within its Workspace.
6. A connection may not be rebound across Workspaces through this feature.

The same provider identity may still exist in another Workspace only under the
existing cross-Workspace plan and sharing rules. This design does not change
those rules.

## 6. Data model

### 6.1 `social_connections`

Add a Workspace-scoped table containing the credential and physical connection
columns moved from `social_accounts`. Store a non-null canonical
`provider_identity` resolved from verified provider data. It is intentionally
separate from the provider account ID used for display because Instagram webhook
identity currently differs from its display account ID.

Required constraints:

- primary key on `id`;
- foreign key `workspace_id -> workspaces(id) ON DELETE CASCADE`;
- one active connection per `(workspace_id, platform, provider_identity)` across
  connection types;
- indexes for token refresh scans, status scans, provider webhook lookup, and
  Workspace account counts.

Uniqueness must use the same verified provider identity used by the existing
Connect ownership store. It must not trust caller-supplied account IDs.

### 6.2 `social_accounts`

Add a non-null `connection_id` foreign key after backfill. Keep `id` and
`profile_id` unchanged.

Credential and shared-status columns move to `social_connections` after all
read and write paths have switched. They may remain temporarily during a staged
dual-read migration, but `social_connections` becomes authoritative before the
old columns are removed.

Required constraint:

```text
UNIQUE(profile_id, connection_id)
```

Unbind is a state transition on this stable binding identity. Rebinding the same
connection to the same Profile reactivates the existing account ID instead of
creating another historical binding row.

### 6.3 Existing dependent rows

The following references remain binding-scoped and keep their existing
`social_account_id` foreign keys:

- `social_post_results`;
- `post_delivery_jobs`;
- post failures and integration logs;
- connect session completion IDs;
- post analytics through `social_post_results`.

The delivery worker resolves credentials through
`social_account -> social_connection`.

Physical-account state that must not be duplicated across bindings moves or is
keyed by `connection_id`, including token refresh ownership, provider rate-limit
serialization, per-physical-account caps, webhook subscriptions, and account
health.

Inbox records and account-level analytics require a connection-scoped ownership
path. Public responses may continue to include a compatible binding account ID,
but storage and deduplication must not duplicate one inbound provider event per
Profile binding.

## 7. Migration and backward compatibility

The migration preserves every existing public account ID.

1. Create `social_connections` in a backward-compatible state.
2. Add nullable `social_accounts.connection_id`.
3. For every existing `social_accounts` row, create one connection containing
   its current physical fields.
4. Set the existing row's `connection_id` to the new connection ID.
5. Verify every account has exactly one connection and that Workspace ownership
   matches through the Profile.
6. Make `connection_id` non-null and add binding uniqueness.
7. Switch connection, publish, refresh, webhook, health, and metrics code to the
   new authority.
8. Remove obsolete Profile-scoped provider-identity uniqueness and credential
   columns only after deployed verification confirms no old readers or writers.

Compatibility guarantees:

- existing `social_account_id` values do not change;
- existing Post payloads do not change;
- `GET /v1/accounts?profile_id=...` continues to return Profile-scoped accounts;
- existing Post results and webhook payloads keep their current primary account
  identifier;
- customers do not need to update stored account IDs;
- old clients that know nothing about shared connections keep current behavior.

## 8. Account connection and binding flows

### 8.1 First connection in a Workspace

1. Complete the existing OAuth, direct-connect, or managed Connect verification.
2. Create the SocialConnection with verified provider identity and encrypted
   credentials.
3. Create the SocialAccount binding for the requested Profile.
4. Return the new binding's existing-style `social_account_id`.

### 8.2 Connecting the same provider account to another Profile

1. Resolve the verified provider identity.
2. Find the existing SocialConnection in the Workspace.
3. Refresh its authoritative credentials and shared metadata.
4. If the target Profile already has an active binding, reconnect and return it.
5. Otherwise create a new Profile binding with a new public account ID.
6. Do not return `ACCOUNT_ALREADY_CONNECTED` or `profile_mismatch` merely because
   another Profile owns a binding to the same Workspace connection.

Managed Connect ownership still validates the managed external user. Binding a
connection owned by another managed user remains an ownership conflict.

### 8.3 Listing accounts

Account listing remains Profile-first. Each Profile sees its binding ID.

The response may add backward-compatible fields:

```json
{
  "id": "sa_twitter_dev",
  "profile_id": "pr_dev",
  "platform": "twitter",
  "account_name": "@unipost",
  "shared_connection": true,
  "bound_profile_ids": ["pr_dev", "pr_staging", "pr_production"]
}
```

V1 does not require clients to receive or submit `connection_id`.

## 9. Unbind, disconnect, and Profile deletion

The product must distinguish two operations.

### 9.1 Unbind from Profile

Unbind disables or removes one SocialAccount binding. It does not revoke tokens,
disconnect the provider account, remove other bindings, or delete shared inbox
and account-metric data.

### 9.2 Disconnect physical connection

Disconnect revokes or invalidates the SocialConnection and makes every binding
unavailable. The UI must identify every affected Profile and require explicit
confirmation before this operation.

### 9.3 Delete Profile

Deleting a Profile removes its bindings through the existing Profile cascade.
It must not delete a SocialConnection that still has another active binding.
An unbound connection with no remaining references may be cleaned up only after
provider webhook cleanup and credential-lifecycle requirements have completed.

## 10. Post API contract

### 10.1 Request compatibility

The request shape remains unchanged:

```json
{
  "platform_posts": [
    {
      "account_id": "sa_twitter_dev",
      "caption": "v1.4 is live"
    }
  ]
}
```

`account_id` resolves both:

- the selected Profile attribution through the SocialAccount binding; and
- the physical credentials through its SocialConnection.

The API does not require `profile_id` or `connection_id`.

The legacy `account_ids` request shape remains supported under the same rule.

### 10.2 One binding per connection per Post

After resolving all account IDs, the validator groups distinct binding IDs by
`connection_id`.

- one binding for a connection: allowed;
- the same binding repeated for an explicitly valid platform sequence such as a
  Twitter thread: allowed under the existing sequence rules;
- two or more different binding IDs for the same connection: reject the entire
  Post request.

This rule applies to immediate posts, scheduled posts, drafts, draft publish,
each logical Post inside a bulk request, retries that revalidate destinations,
and any future Post entry point. Existing bulk response semantics may continue
across separate logical Posts, but one conflicting Post item must have no partial
destination dispatch.

### 10.3 No silent merge or first-item-wins

The validator rejects duplicate connection bindings even when caption, media,
reply target, thread fields, first comment, and platform options are identical.

It must never:

- publish only the first array item;
- merge bindings silently;
- attribute one physical result to multiple Profiles;
- publish valid destinations while skipping the conflicting destinations.

Array order must not affect the outcome.

### 10.4 Validation error contract

Return HTTP `422 Unprocessable Entity` before any Post insert, quota reservation,
delivery job, media-side effect, credit mutation, or provider API call.

Use one stable code for matching and conflicting payloads:

```json
{
  "error": {
    "code": "DUPLICATE_SOCIAL_CONNECTION",
    "message": "The same physical social connection is selected through multiple profiles. Choose one account binding.",
    "details": {
      "platform": "twitter",
      "account_ids": ["sa_twitter_dev", "sa_twitter_staging"],
      "profile_ids": ["pr_dev", "pr_staging"],
      "platform_post_indexes": [0, 1],
      "payloads_match": true
    }
  }
}
```

When delivery payloads differ, return the same code with
`payloads_match: false` and tell the caller to choose one Profile or create
separate Post requests. Do not expose internal connection IDs in the error.

If a request contains multiple connection conflicts, return the first conflict
after sorting by platform and then by the sorted public account IDs. This keeps
the confirmed error details shape stable and prevents request array order from
choosing a different physical connection as the reported conflict. The returned
`platform_post_indexes` still identify the original request positions.

### 10.5 Separate Post requests

The restriction is scoped to one UniPost Post. A caller may deliberately create
separate Posts through different Profile bindings. Those requests produce
separate real provider posts and consume quota or credits independently.

Cross-request content deduplication is outside this design. Existing idempotency
keys continue to protect retries of the same logical request.

## 11. Publishing, queueing, and provider limits

Delivery records keep the selected Profile binding ID so existing post history
and post analytics remain Profile-scoped. Credential loading joins to the
connection.

All physical-account concurrency controls must use `connection_id`, not binding
ID. Otherwise multiple Profile bindings could bypass provider serialization,
daily caps, refresh locks, or rate limits.

Quota and credit units reflect real provider operations. Because one Post cannot
select two bindings for the same connection, existing per-destination accounting
remains unambiguous.

Retrying a failed result uses the original binding and connection. Rebinding or
unbinding after Post creation must not silently retarget historical Posts to
another Profile.

## 12. Data visibility and analytics

### 12.1 Profile-scoped data

The selected SocialAccount binding owns:

- Post visibility;
- social post results;
- post analytics and exports;
- Profile post counts;
- Post-related integration logs;
- drafts and schedules.

A Post made with `sa_twitter_dev` appears only in the Development Profile, even
though the connection is also bound to Staging and Production.

### 12.2 Connection-scoped data

The physical connection owns:

- inbox messages, comments, and conversation state;
- follower and other account-level metrics;
- account health and reconnect-required state;
- provider webhook receipts and deduplication;
- provider rate limits and usage controls.

All Profiles bound to the connection may view the same connection-scoped data,
subject to existing Workspace role and managed-user access rules. Storage must
not clone inbound events per binding. Workspace rollups must count one physical
metric source once.

## 13. Dashboard behavior

1. Connections management shows which Profiles are bound to a connection.
2. Users may bind or unbind Profiles without re-entering credentials when the
   current authorization remains valid.
3. Reconnect and physical disconnect actions clearly state that they affect all
   bound Profiles.
4. The Post composer lists Profile-scoped account bindings as it does today.
5. After one binding is selected, sibling bindings backed by the same connection
   are disabled for the same Post with this explanation:

   > This is the same connected account already selected through another Profile.

6. The backend remains authoritative and applies the same validation to every
   API caller.

## 14. Security and privacy

1. Resolve connection ownership from authenticated Workspace context and Profile
   membership; never trust a caller-supplied Workspace ID.
2. Verify every binding and connection share the same Workspace.
3. Keep tokens only on the physical connection and encrypted under the existing
   encryption service.
4. Preserve managed-user isolation. A shared connection does not grant a second
   managed user access merely because they share a Workspace.
5. Webhook routing must use verified provider identity plus the correct
   provider-app or route identity; it must not fan one event into every binding.
6. Audit bind, unbind, reconnect, and physical disconnect separately with actor,
   Workspace, connection-safe identity, affected Profile IDs, and affected public
   account IDs.

## 15. Error handling

- Duplicate bindings for one connection in a Post: atomic HTTP 422 with
  `DUPLICATE_SOCIAL_CONNECTION`.
- Binding belongs to another Workspace: existing not-found or ownership error;
  never reveal foreign identifiers.
- Profile is not bound to the connection during a bind reuse request: create the
  binding only after verified provider identity and ownership checks pass.
- Shared connection is reconnect-required or disconnected: every binding reports
  the same unavailable state and publishing fails before dispatch.
- Unbind races with publish: transactional validation or a binding version check
  ensures a disabled binding cannot enqueue a new delivery.
- Concurrent binding creation: database uniqueness makes one request succeed and
  the other return the existing binding idempotently.

## 16. Testing strategy

### 16.1 Migration tests

- every existing account receives exactly one connection;
- public account IDs and dependent foreign keys remain unchanged;
- connection Workspace matches binding Profile Workspace;
- no token, provider identity, scope, or status data is lost;
- rollback or staged compatibility behavior is explicitly validated before the
  destructive old-column removal migration.

### 16.2 Connection and binding tests

- first connection creates one connection and one binding;
- reconnect in the same Profile reuses both;
- connect in another Profile reuses the connection and creates a new binding;
- managed-user mismatch remains a conflict;
- cross-Workspace reuse still follows existing plan rules;
- unbind preserves other bindings and credentials;
- physical disconnect disables every binding;
- Profile deletion preserves a still-referenced connection.

### 16.3 Post validation tests

- one binding publishes normally with an unchanged request and response;
- two different bindings to one connection with identical payloads return 422;
- two different bindings to one connection with different captions return 422;
- differences in media, first comment, reply target, or platform options also
  return 422;
- array order does not change the error;
- the first of multiple conflicts is returned deterministically;
- a valid request mixed with one conflict causes zero Post rows, jobs, provider
  calls, quota reservations, and credit mutations;
- an explicit thread using one binding remains valid;
- a thread using sibling bindings for one connection is rejected;
- immediate, scheduled, draft, draft-publish, bulk, and retry paths enforce the
  same invariant.

### 16.4 Shared-state tests

- token refresh occurs once per connection and updates every binding's observed
  health;
- reconnect-required state is visible through every binding;
- provider rate limits and per-account caps aggregate by connection;
- one inbound webhook produces one stored event;
- bound Profiles can read the shared inbox and account metrics under authorized
  scopes;
- Workspace analytics do not multiply account-level values by binding count;
- Post analytics remain visible only under the selected binding's Profile.

### 16.5 Deployed acceptance

In the isolated PR environment and then the real development environment:

1. Create two Profiles in one Workspace.
2. Connect one disposable provider account to the first Profile.
3. Bind the same connection to the second Profile.
4. Confirm each Profile lists a distinct public account ID.
5. Publish using each binding in separate requests and confirm two real provider
   posts with correct Profile history.
6. Submit one request containing both binding IDs and confirm an atomic 422 with
   no provider post and no quota or credit change.
7. Repeat with different captions and confirm the same atomic rejection.
8. Verify reconnect, unbind, physical disconnect, inbox, account metrics, and
   post analytics semantics.

## 17. Rollout and observability

No feature flag is added unless explicitly requested. Rollout relies on staged
migrations, local validation, isolated Preview Acceptance, deployed regression,
and development-environment acceptance.

Structured logs and metrics should cover:

- connection created, reused, refreshed, and disconnected;
- binding created, reused, unbound, and rejected;
- duplicate-connection Post validation counts by platform;
- token refresh attempts per connection;
- binding count per connection;
- webhook matches per verified provider identity;
- any query that resolves more than one connection for an identity that should
  be unique.

Logs must not include plaintext access or refresh tokens.

## 18. Risks and mitigations

### Shared credential blast radius

A token failure affects every binding. Mitigation: show shared state explicitly,
centralize refresh, and make reconnect/disconnect confirmation list affected
Profiles.

### Provider-rate-limit bypass

Profile binding IDs could look like separate accounts. Mitigation: serialize and
cap physical operations by connection ID.

### Inbound duplication

Fan-out by binding would duplicate inbox and metrics. Mitigation: store and dedupe
inbound data at connection scope and project visibility to authorized bindings.

### Accidental duplicate publishing

Two sibling bindings in one Post could publish twice. Mitigation: atomic request
rejection before side effects; never first-item-wins or silently merge.

### Migration drift

Dual credential authorities could diverge during rollout. Mitigation: staged
backfill, explicit authority switch, consistency audits, and delayed old-column
removal.

### Client compatibility

Moving public IDs would break stored customer configuration. Mitigation: preserve
all binding IDs and keep the Post request contract unchanged.

## 19. Final decisions

1. Use one Workspace and keep Workspace-level subscription, quota, and credits.
2. Introduce a Workspace-scoped physical SocialConnection.
3. Keep public SocialAccounts as Profile-scoped bindings with stable IDs.
4. Allow one connection to bind multiple Profiles.
5. Keep the Post API request shape unchanged; `account_id` selects one binding.
6. Permit only one binding per connection in one Post.
7. Reject sibling bindings atomically with HTTP 422, even when payloads match.
8. Never silently publish only the first item and never silently merge items.
9. Keep Post data and post analytics on the selected Profile binding.
10. Share credentials, health, inbox, and account metrics at connection scope.
