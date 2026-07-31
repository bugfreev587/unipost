# Social Connection Profile Bindings Hardening Design

**Status:** Approved for implementation on 2026-07-31
**Scope:** Supplement to `2026-07-24-social-connection-profile-bindings-design.md`
**Pull request:** #299

## 1. Purpose

PR #299 separates a physical provider connection from its Profile-scoped public
account bindings. This supplement closes the remaining merge blockers without
changing the public Post payload or exposing `connection_id`.

The implementation must:

1. make “one logical Post selects only one public binding for a physical
   connection” a database-enforced invariant while still allowing a thread to
   create multiple delivery jobs for the same binding;
2. deploy without running a strict connection-authority migration before old
   application pods have drained;
3. provide deterministic migration preflight, cutover reconciliation, and
   conflict recovery;
4. preserve existing `social_account_id` values whenever a quarantined row can
   be recovered unambiguously;
5. enforce Profile/connection Workspace equality at the database and read
   boundaries; and
6. stop describing a state-changing cutover as a reversible Goose migration.

The task branch and PR may be updated, but this work does not authorize merging
the PR into `dev`.

## 2. Non-goals

- The database cannot infer that two different, nonempty provider identifiers
  are aliases for the same real provider account. Provider-specific verified
  identity remains the canonical source, and the preflight report states this
  limitation explicitly.
- V1 does not expose `connection_id` to API clients.
- V1 does not permit one logical Post to use sibling bindings merely because
  their captions or platform options differ.
- This supplement does not add an application feature flag. Rollout phase is
  durable database state controlled by an explicit cutover operation.
- This supplement does not merge or promote PR #299.

## 3. Chosen rollout architecture

The feature is delivered by one code PR but enabled in two operational phases.

### 3.1 Expand phase

Railway continues to run `./bin/api migrate` before starting the new runtime.
All automatic Goose migrations in PR #299 must therefore be compatible with
old pods that may continue serving during a rolling deployment.

Expand migrations may:

- create `social_connections`, migration evidence tables, rollout-state tables,
  and nullable connection snapshot columns;
- create connection-aware indexes whose legacy branch remains valid when
  `connection_id` is null;
- install a rollout-aware compatibility trigger;
- create the physical-target reservation table and enforcement trigger; and
- create the daily reservation and Inbox schemas in a legacy-compatible state.

Expand migrations must not:

- classify all historical accounts;
- change historical account status to `reconnect_required`;
- attach historical bindings to a connection;
- deduplicate historical Inbox rows by a not-yet-populated connection;
- rewrite historical delivery snapshots to a connection;
- consolidate legacy daily quota counters; or
- enable the strict “all new bindings require `connection_id`” write gate.

New application code writes a connection and binding atomically. An old pod may
still insert a legacy row with null `connection_id`; that row remains isolated
and is classified at cutover.

For an existing non-null binding updated by an old pod during Expand, the
compatibility trigger mirrors authoritative credential, ownership, routing, and
connection-health changes into `social_connections`. An explicit Profile
unbind is not mirrored as a physical disconnect. Platform, managed owner, and
Workspace identity cannot be changed incompatibly.

### 3.2 Explicit cutover phase

After the new version is healthy, an operator runs:

```text
api social-connections-preflight --mode=cutover
api social-connections-cutover
```

Cutover is not part of `preDeployCommand`. It uses the existing Railway backup
gate, records the application SHA and Railway environment identifiers, acquires
a global PostgreSQL advisory lock, drains delivery, verifies the deployed
runtime inventory, and performs reconciliation transactionally. It does not
trust a human-only "old pods drained" assertion.

The cutover command scripts the operational sequence:

1. change rollout state from `expand` to `draining`;
2. let the database claim guard refuse every new delivery-job lease, including
   claims issued by old workers that do not know about rollout state;
3. wait for existing running/retrying jobs and provider-I/O leases to reach zero;
4. query Railway with the environment-scoped migration token and verify that
   every active API and delivery-worker deployment uses the requested commit
   SHA and that no older deployment remains active;
5. inspect database application sessions and reject unrecognized or older
   UniPost runtime sessions;
6. rerun cutover preflight and create the exact-environment backup; and
7. execute reconciliation. On a pre-reconciliation failure, restore phase
   `expand` so claims resume safely.

New runtimes set a versioned PostgreSQL `application_name` containing process
kind, service ID, deployment ID, and commit SHA. Railway deployment inventory
is authoritative for pod version; database sessions and a quiet legacy-write
window are defense-in-depth. If deployment verification is unavailable,
cutover fails closed. A fresh disposable PR environment may use its existing
fresh-environment proof because it has no preceding deployment or data.

The cutover transaction:

1. changes rollout state from `draining` to `cutting_over`;
2. locks the affected account, job, Inbox, result, and daily-ledger write
   surfaces;
3. reruns the same classifier used by preflight;
4. promotes only safe canonical groups and preserves every source account ID;
5. quarantines ambiguous groups as non-publishable;
6. reconciles delivery snapshots and Post physical-target reservations;
7. reconciles Inbox canonical rows and supersession evidence;
8. consolidates current-day physical quota reservations and operation rows;
9. verifies every publishable binding has one same-Workspace connection;
10. verifies no nonterminal delivery is left with an invalid binding snapshot;
11. changes rollout state to `cutover`; and
12. commits the evidence and authority switch together.

Any failed verification rolls the transaction and rollout phase back to
`expand`. The command is idempotent: a completed cutover reports the recorded
run instead of repeating data mutations.

## 4. Rollout state and audit

`social_connection_rollout_state` contains one well-known row:

```text
id                         = 'global'
phase                      = 'expand' | 'draining' | 'cutting_over' | 'cutover'
cutover_application_sha    nullable text
cutover_environment_id     nullable text
cutover_completed_at       nullable timestamptz
updated_at                 timestamptz
```

`social_connection_cutover_runs` records attempts and their non-secret report:

```text
id
application_sha
environment_id
phase_before
status                     = 'started' | 'succeeded' | 'failed'
report                     jsonb
started_at
completed_at
```

Failed attempts are logged by the command even when the reconciliation
transaction rolls back. Secrets and raw access or refresh tokens are never
included in reports.

## 5. Thread-compatible physical-target invariant

### 5.1 Why a delivery-job unique index is incorrect

A Post thread legitimately creates multiple delivery jobs for one public
`social_account_id`. A partial unique index on
`(post_id, COALESCE(connection_id, social_account_id))` would reject those
threads. It would also fail to identify two incorrectly aliased connection IDs
as one provider account.

### 5.2 Durable target selection

Add `social_post_physical_targets`:

```text
post_id                     text, references social_posts on delete cascade
physical_target_key         text
connection_id               nullable text, references social_connections
selected_social_account_id  nullable text
status                      = 'active' | 'migration_conflict'
conflict_details            jsonb
created_at
updated_at

primary key (post_id, physical_target_key)
```

Physical keys are namespaced:

- migrated: `connection:<connection_id>`;
- Expand-phase legacy: `legacy-account:<social_account_id>`.

For `status = 'active'`, `selected_social_account_id` is required. For
`migration_conflict`, it is null and no delivery may start.

A database trigger on every `post_delivery_jobs` insert atomically inserts or
locks the target row:

- no row: reserve the target for the job’s public account ID;
- same selected account: allow another job, including another thread item or a
  retry;
- different selected account: reject the insert;
- migration-conflict target: reject the insert.

The primary key serializes concurrent first inserts. The target row survives
terminal delivery jobs, so a later retry or forgotten enqueue path cannot
select a sibling after the first job finishes.

The application-level `DUPLICATE_SOCIAL_CONNECTION` validation remains for a
clear HTTP 422 response before any result or delivery mutation; this admission
layer provides the normal no-side-effect guarantee. The database trigger is a
per-insert fail-closed backstop: it prevents a second sibling job and therefore
prevents duplicate provider I/O, but it cannot roll back a first job that an
internal caller already committed in a separate transaction. All first-party
batch enqueue paths must reserve their full physical-target set in one
transaction before inserting jobs, or roll the batch back on a database
rejection, and translate that rejection to the same normalized error.

### 5.3 Cutover of legacy Post targets

Cutover rekeys legacy target groups using the newly assigned connection:

- one selected account per `(post, connection)`: create an active connection
  target and preserve all same-binding thread jobs;
- multiple selected accounts with nonterminal work: fail cutover and report the
  exact Post, accounts, results, and jobs that must be drained or resolved;
- multiple selected accounts with terminal history only: create a
  `migration_conflict` target and durable evidence. Historical results remain
  unchanged, but no sibling retry can publish until an operator explicitly
  selects the surviving binding.

This protects future provider I/O without rewriting history or pretending an
already duplicated historical delivery did not occur.

## 6. Preflight and reconciliation

`social-connections-preflight` is read-only. It supports human-readable output
and `--json`, and exits nonzero only for hard cutover blockers.

The report contains:

- total account rows and candidate canonical groups;
- missing provider identity by platform, including Instagram rows without
  `instagram_webhook_user_id`;
- incompatible credentials, scopes, routing metadata, app modes, ownership,
  managed owner, and duplicate Profile binding groups;
- publishable active rows with null `connection_id`;
- Profile/connection Workspace or platform mismatches;
- running or retrying jobs that prevent a safe cutover;
- pending jobs that need snapshot reconciliation;
- Post targets that would collapse sibling bindings;
- Inbox rows that require connection-level supersession;
- current-day quota rows and operations that require consolidation;
- relevant table row counts and relation sizes for the maintenance-window
  decision; and
- heuristic alias warnings when two candidate connections in one
  Workspace/platform have different canonical identities but overlap on a
  non-secret stable secondary identifier. For Instagram this includes
  cross-row overlap between the application-scoped `external_account_id` and
  stored professional/webhook identifiers.

The report proves consistency only for stored canonical identities. It cannot
prove that two different provider identifiers are aliases. Production cutover
requires separate provider-specific investigation when heuristic evidence
suggests canonical identity corruption. Heuristic matches are warnings and
never authorize an automatic merge. Cutover does not call provider APIs: doing
so would add credential, permission, rate-limit, and network availability to
the maintenance-window authority decision.

## 7. Conflict recovery and stable public IDs

Quarantined source rows keep their original `social_account_id`, Profile,
history, and non-secret migration evidence. They are non-publishable until a
verified reconnect resolves them.

Reconnect flows may carry an optional server-validated
`reconnect_account_id`. The target account must:

- belong to the authenticated Workspace and expected Profile;
- be null-connection and `reconnect_required`;
- appear in unresolved migration evidence;
- match the verified platform; and
- satisfy managed-owner isolation.

After provider verification, recovery finds or creates the canonical
connection and updates the same binding row to that connection, increments
`binding_version`, restores active status, and records the evidence resolution.
The strict write trigger permits null-to-non-null recovery only when the new
authority matches the target Profile Workspace, platform, ownership, verified
provider identity, and connection projection.

If `(profile_id, connection_id)` already has another binding, or multiple
quarantined rows are indistinguishable, recovery fails closed. An operator must
select the surviving public binding; the system does not silently delete,
renumber, or merge IDs. This is the only case in which preserving every
historical ID is impossible under the one-binding-per-Profile invariant.

Untargeted OAuth keeps the normal connection behavior. The Dashboard reconnect
action for a quarantined row must use targeted reconnect so a new public ID is
not accidentally created.

## 8. Workspace isolation

Workspace equality is enforced in three places:

1. the binding write trigger loads the target Profile and connection and
   rejects different Workspace IDs;
2. connection-aware reads require:

   ```sql
   sa.connection_id IS NULL OR sc.workspace_id = @workspace_id
   ```

3. cutover verifies and reports any preexisting mismatch before enabling strict
   authority.

The trigger also requires binding and connection platforms to match. Managed
ownership remains connection-level and cannot be reassigned by binding or
recovery operations.

Bind and unbind handlers no longer derive Workspace authority from an arbitrary
Profile when authenticated Workspace context is missing. Workspace-scoped
routes fail closed if middleware did not provide it.

Availability helpers treat `binding_status <> 'active'` as unavailable even
though the final delivery guard continues to validate the binding snapshot
atomically immediately before provider I/O.

## 9. Migration safety and rollback

Goose migrations 136–139 are revised so their automatic Up path is Expand-only.
Their Down paths may reverse schema only when no connection-owned runtime data
or cutover evidence exists. They do not claim to restore a completed authority
cutover.

The state-changing cutover is explicitly forward-only:

- it is registered with the existing Railway backup gate;
- its affected-row preflight is shown before backup and execution;
- the backup is bound to the exact Railway project, environment, Postgres
  service, volume instance, application service, and commit SHA;
- rollback means restore the verified pre-cutover database backup and deploy
  the Expand-compatible application artifact, not run `goose down`.

Migration safety markers, the irreversible-operation registry, and the safety
manifest must agree in tests.

## 10. Operational sequence

1. Deploy the Expand migrations and application to the isolated PR environment.
2. Run local CI and Preview Acceptance on the exact PR head SHA.
3. After eventual merge authorization, deploy Expand to the target environment.
4. Wait for the new API and worker version to become healthy.
5. Run the cutover command; it atomically enters drain mode and database guards
   stop new claims from old and new workers.
6. Let the command verify zero active provider-I/O leases, the Railway
   deployment inventory, versioned database sessions, and the legacy-write
   quiet window.
7. Let the command rerun preflight, retain its JSON report, and confirm a
   Railway backup for the exact environment and SHA.
8. Let the command execute cutover and restore delivery claims only after
   reconciliation succeeds.
9. Verify only the new runtime is serving after cutover.
10. Verify account listing, targeted reconnect, sibling binding, thread
    publishing, duplicate sibling rejection, Inbox visibility, quota, and
    delivery behavior.

PR #299 stops before merge. This sequence is documentation and executable
capability, not authorization to update `dev`.

## 11. Required tests

### 11.1 Physical target

- two sibling bindings in one Post are rejected by application validation;
- bypassing application validation is rejected by the database trigger;
- repeated jobs for the same binding and Post are allowed for threads;
- retry after a terminal job is allowed only for the same binding;
- a sibling retry after terminal completion is rejected;
- concurrent sibling inserts produce exactly one selected binding;
- a migration-conflict target permits no job;
- a large same-binding thread creates one target row without rejecting jobs;
- target-trigger latency and lock wait remain within the delivery enqueue
  performance budget under concurrent thread insertion.

### 11.2 Rolling deployment

- Expand permits an old-style null-connection insert;
- Expand mirrors an old-style token refresh for an already classified binding;
- Expand mirrors an old-style physical disconnect but not a Profile unbind;
- drain mode prevents old and new workers from acquiring a new delivery lease;
- cutover rejects an active Railway deployment on an older SHA;
- cutover rejects an unrecognized or older UniPost database session;
- cutover restores Expand and resumes claims when drain verification fails;
- Cutover rejects new null-connection inserts;
- Cutover allows only constructively valid quarantined-row recovery.

### 11.3 Preflight and cutover

- every classifier reason is reported deterministically;
- IG missing webhook identity is counted and quarantined;
- overlapping secondary provider identifiers produce warnings without merging
  connections;
- cutover refuses running/retrying delivery work;
- pending snapshots reconcile without changing the selected public binding;
- Inbox duplicates retain supersession evidence;
- daily counters consolidate by physical connection without losing operations;
- rerunning a successful cutover is a no-op with the same recorded result;
- a failed verification rolls back authority and rollout state.

### 11.4 Stable ID recovery and isolation

- targeted reconnect reuses the quarantined `social_account_id`;
- ambiguous recovery fails without creating a replacement binding;
- cross-Workspace connection attachment fails at the database trigger;
- resolved credential reads fail closed on a Workspace mismatch;
- Bind and Unbind reject missing authenticated Workspace context;
- unbound bindings are unavailable in admission and delivery helpers.

### 11.5 Regression

- backend package and PostgreSQL integration suites pass;
- existing thread behavior remains valid;
- existing public Post API payloads and account IDs remain unchanged;
- Dashboard build and account/posting regression suites pass;
- Preview Acceptance validates the exact pushed PR head SHA.

## 12. Acceptance criteria

The hardening is complete only when:

- automatic deployment performs Expand only;
- explicit cutover is backup-gated and idempotent;
- no current or future enqueue path can select sibling bindings for one
  connection and Post while same-binding threads still work;
- all publishable bindings after cutover have a same-Workspace connection;
- conflicts are reported, quarantined, and recoverable without silently
  replacing public IDs;
- migration safety labels describe actual rollback behavior;
- required local and remote checks pass on the exact PR head; and
- PR #299 remains unmerged for user review.
