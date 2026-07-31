# Social Connection and Profile Bindings Design

**Date:** 2026-07-24
**Status:** Revised after code-mapped review; ready for written re-review
**Target branch:** `codex/social-account-profile-bindings` from `origin/staging`

## 1. Summary

UniPost currently stores OAuth credentials, provider identity, connection state,
and Profile ownership in one `social_accounts` row. This prevents the same
physical provider account from being used by more than one Profile in the same
Workspace.

This design separates the physical provider connection from its Profile-scoped
account bindings:

- `social_connections` is the Workspace-scoped physical connection. It owns
  credentials, provider identity, refresh state, shared account status, and
  webhook routing identity. It also owns the nullable managed-user identity
  (`external_user_id`); one physical connection has exactly one managed-user
  owner or is owner/BYO-scoped.
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
7. Store inbox data and account-level metrics once per physical connection while
   preserving the mandatory `(workspace_id, external_user_id)` managed-user
   authorization boundary on every read, mutation, sync, and realtime path.

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

- dashboard OAuth uses `FindSocialAccountByExternalID` to find an existing
  provider account in the Workspace and silently refresh/reactivate that row;
- direct connect performs a Workspace lookup and returns the public
  `ACCOUNT_ALREADY_CONNECTED` conflict;
- managed Connect uses a provider-identity advisory lock plus an ownership
  decision. `profile_mismatch`, `managed_user_mismatch`, BYO ownership, and
  ambiguous matches are internal conflict classes; callers receive the existing
  HTTP 409 HTML error page rather than those classes as public API codes.

Workspace-level deduplication is application-enforced today, not a database
constraint. The database uniqueness that exists is Profile-scoped, including
`social_accounts_managed_unique_idx` and
`social_accounts_active_external_unique_idx`. Historical data can therefore
contain more than one row for the same Workspace, platform, and provider
identity, including rows owned by different managed users.

`social_posts.profile_ids` is a lazily populated derived cache. Draft and
scheduled rows created before or outside the publish/claim paths may have an
empty value. Binding authorization, duplicate-connection validation, and
attribution must resolve from the selected `social_account_id` rows and result
rows, never from `social_posts.profile_ids` as an authority.

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
- nullable `external_user_id` managed-user ownership and managed-user email;
- active, reconnect-required, or disconnected status;
- provider webhook identity and subscription lifecycle;
- physical-account rate-limit and serialization identity.

### 5.2 SocialAccount responsibilities

The public Profile binding owns:

- the existing public `social_account_id`;
- `profile_id`;
- `connection_id`;
- binding status and binding timestamps;
- monotonically increasing binding `version` for publish/unbind race checks;
- optional Profile-specific display overrides added by future features.

There is exactly one stable binding row per `(profile_id, connection_id)`, not
one row per active period. Unbind and rebind change status and increment
`version`; they do not create another historical row.

### 5.3 Ownership invariants

1. A SocialConnection belongs to exactly one Workspace.
2. A SocialAccount binding belongs to exactly one Profile.
3. The binding's Profile must belong to the same Workspace as the connection.
4. A Profile may have at most one active binding to a connection.
5. A connection may have multiple active bindings within its Workspace.
6. A connection may not be rebound across Workspaces through this feature.
7. A managed SocialConnection has exactly one non-null `external_user_id`.
8. A binding never overrides or broadens its connection's managed-user owner.
9. A managed connection may be reused in another Profile only inside the same
   authenticated Workspace and the same selected `external_user_id` scope.

The same provider identity may still exist in another Workspace only under the
existing cross-Workspace plan and sharing rules. This design does not change
those rules.

## 6. Data model

### 6.1 `social_connections`

Add a Workspace-scoped table containing the credential and physical connection
columns moved from `social_accounts`. It contains nullable `external_user_id`
and `external_user_email` ownership fields. Managed connections require a
non-null, immutable `external_user_id`; owner/BYO connections require it to be
null. Changing managed-user ownership is not an update operation: it is an
ownership conflict requiring explicit remediation.

Store a canonical `provider_identity` resolved from verified provider data. It
is intentionally separate from the provider account ID used for display because
Instagram webhook identity differs from its application-domain account ID.
Normal reusable connections require a non-null canonical identity. Temporary
`migration_conflict` records may have no canonical identity, cannot publish or
be matched by connect flows, and are excluded from the new authority until
remediated.

Required constraints:

- primary key on `id`;
- foreign key `workspace_id -> workspaces(id) ON DELETE CASCADE`;
- one reusable canonical connection per
  `(workspace_id, platform, provider_identity)` across connection types and
  across active, reconnect-required, and disconnected states;
- a partial unique index over rows whose `provider_identity IS NOT NULL` and
  `status <> 'migration_conflict'`; disconnected rows remain inside this index
  so reconnect must reuse their `connection_id` rather than create a new one;
- checks requiring `external_user_id IS NOT NULL` for managed connections and
  `external_user_id IS NULL` for owner/BYO connections;
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
creating another historical binding row. Add
`version BIGINT NOT NULL DEFAULT 1`; every bind-state or connection-target
transition increments it. Delivery admission snapshots both `connection_id` and
`version` and verifies them before enqueue/dispatch.

### 6.3 Existing dependent rows

The following references remain binding-scoped and keep their existing
`social_account_id` foreign keys where a foreign key currently exists:

- `social_post_results`;
- `post_delivery_jobs`;
- post failures;
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

`integration_logs.social_account_id` is a plain `TEXT` column, not a foreign
key. It remains a binding identifier by convention and must be explicitly
audited in migration/read-path tests; no database cascade or automatic repoint
can be assumed.

## 7. Migration and backward compatibility

The migration preserves every existing public account ID, but it must not assume
that current rows are already unique at Workspace scope. Backfill is a classified
deduplication process, not a mechanical one-row-to-one-connection copy.

### 7.1 Canonical identity mapping

Backfill derives identity from stored provider-verified fields, never from a new
caller value:

| Platform | Canonical `provider_identity` source |
| --- | --- |
| Instagram | nonempty `metadata->>'instagram_webhook_user_id'` |
| All other currently supported platforms | nonempty `external_account_id` |

Instagram's `external_account_id` is the application-domain ID (`raw.ID`), while
webhook routing and current ownership matching use the professional user ID
(`raw.UserID`) persisted as `instagram_webhook_user_id`. They are not
interchangeable. An Instagram row without the webhook identity, or any active
row without its platform's canonical identity, is an unresolved migration
conflict. The migration must not guess, fall back to an account name, or use the
Instagram application-domain ID as the canonical webhook identity.

Disconnected historical rows with intentionally scrubbed identities may be
recorded as `migration_conflict`, but they are not eligible for automatic
matching, credential authority, publishing, or reconnect until provider
reverification supplies the canonical identity.

### 7.2 Preflight grouping and merge eligibility

Before creating authoritative connections, inventory all rows grouped by
`(workspace_id, platform, canonical_provider_identity)`. For each group, record
the source account IDs, Profile IDs, statuses, connection types, app/route modes,
and managed ownership.

Automatic collapse into one connection is allowed only when every source row:

1. has the same ownership class (all managed or all owner/BYO);
2. for managed rows, has the same nonempty `external_user_id`;
3. has compatible provider-app mode and webhook-route identity;
4. has one unambiguous credential winner according to an explicit precedence
   rule: active beats disconnected, then latest verified refresh timestamp,
   then latest connected timestamp, then stable account ID as the tie-breaker;
5. contains no contradictory provider identity, scope, or token metadata that
   would make the winner unsafe.

Eligible groups create one SocialConnection and retain every source
`social_accounts.id` as a separate Profile binding. The chosen credential source
and every discarded duplicate field value are written to the migration audit.

A group crossing two `external_user_id` values, crossing managed and owner/BYO
ownership, lacking canonical identity, or having incompatible routing/credential
state must never be silently merged. It is written to a
`social_connection_migration_conflicts` audit/operations table (or equivalent
durable migration report) and remains on the legacy authority path. No binding
in that group receives an authoritative reusable connection until the conflict
is explicitly resolved or quarantined.

### 7.3 Staged rollout gates

1. Create `social_connections` and the migration-conflict audit surface in a
   backward-compatible state.
2. Add nullable `social_accounts.connection_id` and binding `version`.
3. Run the identity inventory and conflict report without changing authority.
4. Backfill only eligible groups, preserving every public account ID.
5. Re-run the inventory under the same provider-identity advisory lock used by
   connect flows so a concurrent connection cannot escape classification.
6. Resolve or explicitly quarantine every unresolved active conflict. The
   authority switch is blocked while an active row has no safe connection.
7. Verify each migrated account has exactly one connection, the connection
   Workspace matches its Profile Workspace, and managed ownership is unchanged.
8. Add the normal-connection uniqueness index and
   `UNIQUE(profile_id, connection_id)`, then make `connection_id` non-null only
   after the unresolved-active-conflict count is zero.
9. Switch connect, publish, refresh, webhook, Inbox, health, and metrics code to
   the connection authority. Dual-read is compatibility-only; writes have one
   declared authority in each rollout phase.
10. Remove obsolete Profile-scoped credential/ownership columns and uniqueness
    only after deployed verification confirms no old readers or writers.

This process intentionally fails closed instead of merging two managed users.
Operational remediation may reconnect one verified owner, leave the other row
quarantined, or correct bad historical metadata, but must be auditable and must
not reassign `external_user_id` implicitly.

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

### 8.2 Existing identity: preserve each flow's contract

All flows resolve the same verified provider identity and the same connection,
but they keep their current external duplicate behavior unless this design
explicitly changes it:

- **Dashboard OAuth:** refresh/reactivate the existing SocialConnection. Reuse
  the target Profile's stable binding when present; otherwise create a new
  binding for the OAuth-selected Profile. This replaces the current behavior
  that refreshes the one old `social_accounts` row without creating a binding
  for a different OAuth target Profile.
- **Direct connect:** preserve the public hard HTTP 409
  `ACCOUNT_ALREADY_CONNECTED` response for a repeated physical identity. A
  caller that wants another Profile binding uses the explicit bind operation,
  not a duplicate credential submission.
- **Managed Connect:** preserve the advisory lock and ownership decision. A
  matching connection with the same nonempty `external_user_id` may reuse the
  connection and create/reactivate the target Profile binding; a different
  `external_user_id`, owner/BYO connection, or ambiguous match remains a 409
  ownership conflict. `profile_mismatch` stops being a conflict only for this
  same-owner reuse case. The external response remains the current HTML error
  page; conflict classes remain structured logs/internal decisions rather than
  newly exposed public codes.

The normal transaction order is: resolve verified identity, acquire the
Workspace/platform/identity advisory lock, load the canonical connection,
validate connection ownership, refresh connection credentials when the flow
permits it, and create/reactivate the binding idempotently.

### 8.3 Explicitly bind an existing connection

The Dashboard may offer a bind operation using an existing public
`social_account_id` as the source and a target Profile ID. V1 does not need to
expose `connection_id`. The backend resolves the source binding, verifies the
authenticated Workspace, role, target Profile, and managed-user scope, then
creates or reactivates `(target_profile_id, connection_id)`. Possession of a
source account ID alone is never authorization.

### 8.4 Listing accounts

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

`bound_profile_ids` is optional and must be filtered through the caller's
current Workspace membership, role, and managed-user scope. A managed-user
request must not learn Profile IDs visible only through another
`external_user_id`; omitting the field is preferable to leaking hidden bindings.

## 9. Unbind, disconnect, and Profile deletion

The product must distinguish two operations.

### 9.1 Unbind from Profile

Unbind disables or removes one SocialAccount binding. It does not revoke tokens,
disconnect the provider account, remove other bindings, or delete shared inbox
and account-metric data. The preferred implementation is a status transition
that increments binding `version`, preserving the public account ID for a later
idempotent rebind.

### 9.2 Disconnect physical connection

Disconnect revokes or invalidates the SocialConnection and makes every binding
unavailable. The UI must identify every affected Profile and require explicit
confirmation before this operation.

A normal disconnect preserves the canonical connection row and identity. A later
verified reconnect must acquire the identity lock, reuse the same
`connection_id`, update its authoritative credentials/status in place, and
reactivate existing bindings without changing their public IDs. Connect code
must not create a second normal connection merely because the old one has
`status = 'disconnected'`.

Historical post/result rows remain binding-scoped; queued or retried delivery
resolves only through the binding's current stable connection after the version
check. A provider-mandated hard erasure that makes identity reuse impossible is
a separate destructive lifecycle requiring explicit tombstone and retry policy;
it is not the normal disconnect path described here.

### 9.3 Delete Profile

Deleting a Profile removes its bindings through the existing Profile cascade.
It must not delete a SocialConnection that still has another active binding.
An unbound connection with no remaining references may be cleaned up only after
provider webhook cleanup and credential-lifecycle requirements have completed.

The implementation must preserve or deliberately replace existing delete side
effects, including the `BEFORE DELETE ON social_accounts` X Inbox cleanup-route
trigger introduced by migrations 110/111/113. Moving route/credential ownership
to `social_connections` must not cause Profile cascade deletion to erase the
route material required by pending cleanup intents. Trigger behavior, ordering,
and orphan-connection cleanup require migration tests before old columns are
removed.

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

The concrete validator input must carry this relationship. Extend
`platform.ValidateAccount` and the Workspace account-loading query behind
`loadValidateAccounts`/`ListSocialAccountsByWorkspace` with `ConnectionID`.
`uniqueAccountIDs` remains useful for database loading but is not sufficient for
this invariant: after accounts are loaded, `ValidatePlatformPosts` groups the
distinct public account IDs by `ValidateAccount.ConnectionID`.

The duplicate-connection issue is a new request-fatal class. It must be checked
and handled before the current draft early-return and before the current
soft-failure path for `account_disconnected` and
`account_not_in_workspace`. Adding it only to `filterFatalIssues` is
insufficient because normal single-post validation currently writes fatal issue
lists as HTTP 400 and draft creation intentionally tolerates other validation
errors.

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

This is a dedicated error mapping, not the current generic
`writeValidationErrors` response. Add `DUPLICATE_SOCIAL_CONNECTION` to the
normalized error map and return the repository's standard `ErrorResponse`
envelope with:

- `error.code = "DUPLICATE_SOCIAL_CONNECTION"`;
- `error.normalized_code = "duplicate_social_connection"`;
- the stable message below;
- one fatal `duplicate_social_connection` validator issue for each conflicting
  request entry in `error.issues[]`; and
- the public conflict data in `error.details`.

Use one stable code for matching and conflicting payloads:

```json
{
  "error": {
    "code": "DUPLICATE_SOCIAL_CONNECTION",
    "normalized_code": "duplicate_social_connection",
    "message": "The same physical social connection is selected through multiple profiles. Choose one account binding.",
    "issues": [
      {
        "platform_post_index": 0,
        "account_id": "sa_twitter_dev",
        "platform": "twitter",
        "field": "platform_posts.account_id",
        "code": "duplicate_social_connection",
        "message": "This physical social connection is selected through multiple account bindings.",
        "severity": "error"
      },
      {
        "platform_post_index": 1,
        "account_id": "sa_twitter_staging",
        "platform": "twitter",
        "field": "platform_posts.account_id",
        "code": "duplicate_social_connection",
        "message": "This physical social connection is selected through multiple account bindings.",
        "severity": "error"
      }
    ],
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

For one conflicting connection, `account_ids`, `profile_ids`, and
`platform_post_indexes` contain every selected sibling binding/index, including
groups of three or more; they are never truncated. Account and Profile IDs are
deduplicated and sorted, while indexes are unique ascending integers. The
details never expose `connection_id` or unauthorized Profile IDs.

In bulk, this atomicity boundary is one `posts[]` logical item, matching the
existing per-item success model. A conflicting item returns its own 422 result
and performs no insertion, quota acceptance, credit mutation, job creation, or
provider call; independent bulk items may still succeed. Validation and this
return must occur before `countPublishQuotaUnits`, quota acceptance, and
`executeImmediatePost` for that item.

### 10.5 Separate Post requests

The restriction is scoped to one UniPost Post. A caller may deliberately create
separate Posts through different Profile bindings. Those requests produce
separate real provider posts and consume quota or credits independently.

Cross-request content deduplication is outside this design. Existing idempotency
keys continue to protect retries of the same logical request.

Neither validation nor attribution may use `social_posts.profile_ids` to infer
the selected binding. That field remains a derived/lazily populated cache; the
authoritative inputs are `platform_posts[].account_id`, the loaded binding rows,
and persisted result/job binding IDs.

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

Admission persists the selected `social_account_id`, `connection_id`, and
binding `version` on the delivery boundary (job columns or an equivalently
immutable snapshot). Immediately before enqueue/dispatch, the worker verifies
that the binding is active and still has the same connection/version. A mismatch
fails before provider I/O; it must not follow a newly assigned credential path.

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

Storage is connection-scoped, but visibility is not granted merely by having a
Profile binding. Every Inbox read, item lookup, mutation, reply, sync, outbound
status, and realtime subscription must preserve the existing canonical
`InboxAccessScope` boundary:

```text
(authenticated workspace_id, selected external_user_id)
```

For managed-user scope, the selected `external_user_id` must equal
`social_connections.external_user_id`; a sibling binding cannot broaden it.
Workspace aggregate scope remains available only to the already-authorized
owner/admin path. Owner/BYO connections with null `external_user_id` remain
outside managed-user scope. Object misses across scopes return the existing
non-disclosing response.

Storage must not clone inbound events per binding. A connection-owned inbox item
may return a compatible account ID selected only from a binding visible inside
the caller's scope. Workspace rollups count one physical metric source once,
while managed-user projections remain confined to that connection owner.

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
4. Preserve managed-user isolation. Resolve it from
   `social_connections.external_user_id`; a shared connection does not grant a
   second managed user access merely because they share a Workspace or Profile.
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
- A managed-user mismatch, owner/BYO mismatch, ambiguous legacy group, or missing
  canonical identity never falls back to Profile or Workspace sharing; it
  remains a conflict and emits no credentials or hidden Profile identifiers.

## 16. Testing strategy

### 16.1 Migration tests

- identity mapping uses Instagram `instagram_webhook_user_id` and uses
  `external_account_id` for every other current platform;
- an active Instagram row missing webhook identity becomes an unresolved
  conflict and is never matched through its application-domain ID;
- same-identity rows with the same managed `external_user_id` collapse to one
  connection while preserving every binding ID;
- same-identity rows with different `external_user_id` values never collapse and
  block the authority switch until explicitly remediated/quarantined;
- managed and owner/BYO rows with the same identity never collapse;
- ambiguous credential/app-route groups follow the conflict path rather than a
  nondeterministic winner;
- after the conflict gate is clear, every existing account receives exactly one
  connection;
- public account IDs and dependent foreign keys remain unchanged;
- connection Workspace matches binding Profile Workspace;
- managed `external_user_id` moves to the connection without ownership change;
- no token, provider identity, scope, or status data is lost;
- `integration_logs.social_account_id` remains a tested binding identifier even
  though it has no foreign key;
- existing X Inbox delete-trigger cleanup still captures route credentials after
  the ownership move;
- rollback or staged compatibility behavior is explicitly validated before the
  destructive old-column removal migration.

### 16.2 Connection and binding tests

- first connection creates one connection and one binding;
- reconnect in the same Profile reuses both;
- Dashboard OAuth, same-owner managed Connect, and explicit bind in another
  Profile reuse the connection and create/reactivate the target binding;
- managed Connect same-owner Profile mismatch becomes bind/reuse, while
  managed-user mismatch, owner/BYO mismatch, and ambiguous matches remain 409;
- direct connect preserves its hard `ACCOUNT_ALREADY_CONNECTED` duplicate
  contract and the explicit bind operation handles multi-Profile reuse;
- dashboard OAuth refreshes one connection and creates/reactivates the correct
  target binding;
- disconnect followed by verified reconnect reuses the same `connection_id` and
  public binding IDs;
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
- a conflict involving three or more sibling bindings returns every authorized
  account/Profile/index in deterministic order without truncation;
- the 422 response includes both `code` and `normalized_code` plus the expected
  `issues[]` and `details` shape;
- an explicit thread using one binding remains valid;
- a thread using sibling bindings for one connection is rejected;
- immediate, scheduled, draft, draft-publish, bulk, and retry paths enforce the
  same invariant;
- drafts reject this new fatal conflict even though they continue to tolerate
  unrelated validation issues;
- a conflicting bulk item has zero side effects while independent bulk items may
  still succeed;
- empty/lazily populated `social_posts.profile_ids` never bypasses validation or
  changes attribution;
- unbind between validation and dispatch is caught by connection/version check.

### 16.4 Shared-state tests

- token refresh occurs once per connection and updates every binding's observed
  health;
- reconnect-required state is visible through every binding;
- provider rate limits and per-account caps aggregate by connection;
- one inbound webhook produces one stored event;
- owner/admin Workspace scope can read the connection-owned aggregate Inbox;
- managed-user scope can read only connections whose connection-level
  `external_user_id` matches the selected scope;
- two Profile bindings do not allow one managed user to read another managed
  user's inbox item, mutation target, sync, outbound status, or realtime event;
- `bound_profile_ids` returns only caller-visible Profiles;
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

The rollout safety control is phase gating rather than a runtime flag that could
silently weaken authorization or duplicate-publish protection:

1. inventory-only migration;
2. eligible-row backfill while legacy columns remain authoritative;
3. zero-unresolved-active-conflict gate;
4. connection-authority switch with old columns retained for rollback reads;
5. enable multi-Profile bind creation only after Inbox scope and publish guards
   pass deployed acceptance;
6. destructive old-column removal in a later release.

Emergency containment disables creation of new sibling bindings and physical
disconnect operations while keeping the duplicate-connection publish rejection
and managed-user Inbox boundary enabled. Rollback must never restore unsafe
cross-owner merging or allow duplicate physical publishing. A remote feature
flag/kill switch can be added only by an explicit product decision; if added,
its safe-off behavior is “no new sharing,” not “skip security validation.”

Structured logs and metrics should cover:

- connection created, reused, refreshed, and disconnected;
- binding created, reused, unbound, and rejected;
- duplicate-connection Post validation counts by platform;
- token refresh attempts per connection;
- binding count per connection;
- webhook matches per verified provider identity;
- any query that resolves more than one connection for an identity that should
  be unique;
- unresolved migration conflicts by reason, platform, ownership class, and age;
- managed-scope access denials caused by connection-owner mismatch;
- binding-version mismatches caught before enqueue or provider dispatch.

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
inbound data at connection scope and project visibility through the canonical
Workspace/managed-user access scope, not through all bindings.

### Managed-user isolation regression

Moving Inbox storage to connection scope could reintroduce cross-managed-user
IDOR if binding membership were treated as authorization. Mitigation:
`external_user_id` has one authoritative home on the connection; cross-owner
rows never merge; every Inbox surface enforces `(workspace_id,
external_user_id)`; hidden Profile IDs are filtered from responses.

### Accidental duplicate publishing

Two sibling bindings in one Post could publish twice. Mitigation: atomic request
rejection before side effects; never first-item-wins or silently merge.

### Migration drift

Dual credential authorities could diverge during rollout. Mitigation: staged
backfill, explicit authority switch, consistency audits, and delayed old-column
removal.

### Historical identity conflicts

Existing Profile-scoped rows can duplicate one provider identity across managed
owners or lack Instagram webhook identity. Mitigation: preflight grouping,
durable conflict records, no automatic cross-owner merge, zero-conflict rollout
gate, and explicit quarantine/remediation.

### Reconnect identity drift

Creating a new connection after disconnect could strand bindings and make queued
work resolve obsolete credentials. Mitigation: disconnected canonical rows remain
inside the uniqueness key, reconnect updates the same connection, and workers
verify snapshotted connection/version before dispatch.

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
10. Store credentials, health, inbox, and account metrics at connection scope,
    but authorize managed Inbox access by the connection's single
    `external_user_id`, never by all bound Profiles.
11. Move managed-user ownership to `social_connections`; prohibit implicit
    ownership reassignment and cross-owner connection sharing.
12. Backfill only identity groups that are safe to collapse. Missing Instagram
    webhook identity, cross-owner duplicates, and ambiguous routing/credentials
    are blocking migration conflicts, not merge candidates.
13. Keep disconnected canonical connections reusable and require reconnect to
    preserve `connection_id` and existing binding IDs.
14. Preserve the distinct external contracts of direct connect, Dashboard OAuth,
    and managed Connect while changing the managed same-owner Profile-mismatch
    decision into connection reuse plus binding creation.
15. Treat `social_posts.profile_ids` as a non-authoritative cache and
    `integration_logs.social_account_id` as a non-FK binding reference.
16. Use binding `version` plus snapshotted `connection_id` to close
    unbind/rebind-versus-publish races.
