# TikTok Managed Account Identity Accuracy PRD

**Status:** Investigation complete; proposed fix ready for product and engineering review

**Date:** 2026-07-31

**Branch:** `dev-tiktok-stale-account-investigation`

**Production baseline investigated:** `origin/main` at `a3a72b9a170547c92efc6d3071afac697eb33700`

**Owner areas:** Hosted Connect, TikTok integration, Accounts API

**Intended release path:** task branch → Preview Acceptance → `dev` → `staging` → `main`

**Feature flag:** None

## 1. Executive summary

A customer reported that reconnecting TikTok repeatedly appeared to fall back to an old or corrupted account. The TikTok consent screen showed `robyn6608`, while UniPost continued to return the same managed account ID and the name `Robyn` from `/accounts`.

The investigation found no evidence of a stale `/accounts` cache or failed OAuth credential replacement. Production recorded successful OAuth callbacks, replaced the account credentials and provider identity fields, and updated the current row. The current row's display name and avatar match the public `@robyn6608` profile.

The confirmed defect is an identity-field mapping error. TikTok distinguishes the stable username/handle (`username`, such as `robyn6608`) from the editable profile name (`display_name`, such as `Robyn`). UniPost requests only `open_id`, `display_name`, and `avatar_url`, then assigns `display_name` to both its internal `Username` and `DisplayName` fields. The callback consequently stores `Robyn` in both `metadata.username` and `metadata.display_name`, and uses it as `account_name`.

The user experience is made more misleading by two intentional but poorly exposed behaviors:

1. Managed reconnects preserve the UniPost account ID for the same profile, platform, and external user.
2. `/accounts` returns the original `connected_at` but not the latest connection time, explicit TikTok username, or display name.

The selected fix preserves stable managed-account identity and existing foreign-key references. It corrects the TikTok profile request and mapping, adds explicit identity fields to the Accounts API, exposes a true last-connection timestamp, and marks legacy TikTok rows that need one post-fix reconnect before their username can be trusted.

## 2. Incident reference

- Platform: TikTok
- `external_user_id`: `2b5208fd-804f-4fdb-97e5-51023d9535b3`
- `unipostManagedAccountId`: `58d724de-ce6c-487d-826b-507b7de1720f`
- Expected TikTok username: `robyn6608`
- TikTok display name observed by UniPost: `Robyn`
- Investigated Workspace: `ae267ee2-298d-4fa8-b6a0-c386000b17af`
- Investigated Profile: `4ec8ee48-9119-40ad-b4ca-99992e965316`

No OAuth code, access token, refresh token, API key, email address, complete signed avatar URL, or database credential may be copied into this PRD, implementation, tests, logs, screenshots, or release artifacts.

## 3. User problem

### 3.1 Reported behavior

The customer:

1. Revoked UniPost access in TikTok and browser sessions.
2. Logged out of the old TikTok account.
3. Logged in directly as `robyn6608` and confirmed it was active.
4. Started UniPost Hosted Connect.
5. Saw `robyn6608` on the TikTok OAuth screen.
6. Completed authorization successfully.
7. Received the same UniPost managed account ID and the name `Robyn` from `/accounts`.

This made it appear that UniPost had ignored the new authorization and restored a cached or corrupted account.

### 3.2 Customer impact

- Customers cannot confidently tell which TikTok account will receive a post.
- Repeating revocation and reconnection does not change the misleading fields.
- The stable UniPost account ID appears to prove that no switch occurred, even though ID reuse is intentional.
- Support cannot compare historical TikTok `open_id` values because only the current provider identity is stored.
- A correct token replacement can still be perceived as a security or account-selection failure.

The trust impact is high even when publishing would target the newly authorized account. A social publishing product must make the selected destination unambiguous before a customer sends content.

## 4. Confirmed production evidence

### 4.1 Production source state

The investigation fetched `origin` and used `origin/main` at:

```text
a3a72b9a170547c92efc6d3071afac697eb33700
```

The relevant production blobs were also verified to be identical to `origin/dev` at investigation time:

- `api/internal/connect/tiktok.go`
- `api/internal/handler/connect_callback.go`
- `api/internal/db/queries/social_accounts.sql`

The conclusions in this PRD are based on `origin/main`, not on an assumed development-only implementation.

### 4.2 OAuth attempts and persistence

Production contained nine completed TikTok Connect Sessions for the external user, including eight successful callbacks on 2026-07-31. The most recent attempt had:

- Connect Session: `4d56f58b-7990-47a4-911c-14c9158b09fa`
- Status: `completed`
- Completed account: `58d724de-ce6c-487d-826b-507b7de1720f`
- Completed at: `2026-07-31 20:42:26.933592 UTC`

The account row showed:

- Status: `active`
- Connection type: `managed`
- No `disconnected_at` value
- `connect_session_id` equal to the latest completed session
- `last_refreshed_at`: `2026-07-31 20:42:26.920012 UTC`
- Token expiry based on the latest authorization
- Granted `user.info.profile` scope

This proves that the latest callback reached the persistence path and replaced the current row's credentials and profile fields. It is inconsistent with a static database snapshot or a callback that silently failed before save.

### 4.3 Current provider identity alignment

The current database row stored:

- `account_name`: `Robyn`
- `metadata.username`: `Robyn`
- `metadata.display_name`: `Robyn`
- Avatar asset fingerprint: `ad2765eb617aa830c1c2eb9855ae2234`

The public `https://www.tiktok.com/@robyn6608` profile exposed:

- `uniqueId`: `robyn6608`
- `nickname`: `Robyn`
- The same avatar asset fingerprint: `ad2765eb617aa830c1c2eb9855ae2234`

The public identity and current database profile therefore align on nickname and avatar. TikTok `open_id` is app-specific and cannot be mapped to a public profile without calling TikTok with the current access token, so this comparison is not a cryptographic proof of token ownership. It is strong corroborating evidence that the current row contains `robyn6608` profile data rather than an unrelated stale snapshot.

### 4.4 TikTok field contract

TikTok's current User Info documentation defines:

- `open_id`: unique user identity for the current application, available through `user.info.basic`.
- `display_name`: the user's profile name or nickname, available through `user.info.basic`.
- `username`: the user's TikTok username, available through `user.info.profile`.

Reference: <https://developers.tiktok.com/doc/tiktok-api-v2-get-user-info>

The production account had `user.info.profile`, so UniPost was authorized to request `username`.

The production code also confirms that this scope is part of every current standard TikTok Connect request. `user.info.profile` is included in `tiktokConnectAnalyticsScopes`, and `tiktokConnectScopesForSession` unconditionally appends that set to the base publishing scopes. The existing authorization tests assert the complete scope string. A post-fix reconnect will therefore request the permission needed for `username`; older stored tokens may still lack it until reconnect.

### 4.5 Verification results

The existing relevant Go packages were run without the Go test result cache:

```text
go test -count=1 ./internal/connect ./internal/connectownership ./internal/handler
```

All three packages passed. The current TikTok connector test explicitly expects the request to contain only `open_id,display_name,avatar_url` and expects the returned UniPost `Username` to equal TikTok `display_name`. The test suite therefore confirms and currently locks in the defective field contract.

## 5. Root cause

### 5.1 Primary root cause: TikTok username is not requested

`api/internal/connect/tiktok.go` sends:

```text
fields=open_id,display_name,avatar_url
```

It does not request `username` even though the OAuth flow grants `user.info.profile`.

### 5.2 Primary root cause: display name is assigned to username

The same connector returns:

```text
ExternalAccountID = open_id
Username          = display_name
DisplayName       = display_name
AvatarURL         = avatar_url
```

The callback then:

- writes `profile.Username` to `metadata.username`;
- writes `profile.DisplayName` to `metadata.display_name`; and
- selects `profile.Username` first for `account_name`.

For TikTok, all three values become `Robyn`, while `robyn6608` is discarded.

### 5.3 Contributing factor: stable managed-account ID is not explained

`UpsertManagedSocialAccount` conflicts on:

```text
(profile_id, platform, external_user_id)
```

On conflict it updates tokens, `external_account_id`, name, avatar, metadata, scope, and current Connect Session while preserving the row ID. This is required to preserve posts, analytics, delivery history, and other foreign-key references.

The stable ID is correct system behavior, but the Accounts API does not explain or visibly prove that the provider identity behind the ID changed.

### 5.4 Contributing factor: `/accounts` exposes insufficient identity data

The current Accounts API returns:

- `id`
- `account_name`
- `external_account_id`
- original `connected_at`
- connection and ownership fields

It does not return:

- explicit `username`
- explicit `display_name`
- latest successful connection time
- current Connect Session completion time

The opaque `external_account_id` may change, but customers normally cannot interpret a TikTok app-scoped `open_id`. The other visible fields can remain unchanged, creating the appearance of a stale response.

### 5.5 Contributing factor: connection time and token refresh time are conflated

`connected_at` records row creation and intentionally remains unchanged during an upsert. `last_refreshed_at` changes both during a Hosted Connect save and during background token refresh. It cannot safely be renamed or exposed as `last_connected_at` because a token refresh is not a user-authorized account switch.

The data model lacks a dedicated timestamp for the latest successful user connection.

### 5.6 Observability gap: no provider-identity transition history

The latest row contains only the current `external_account_id`. Existing Connect Session and integration-log records prove that callbacks completed, but they do not retain the previous and next provider identity in a bounded audit record. Support cannot retrospectively prove whether each reconnect changed `open_id`.

This gap did not cause the incident, but it increased investigation uncertainty.

### 5.7 Contributing factor: `account_name` is a legacy cross-platform identifier

The callback prefers `Profile.Username` when computing `account_name`, but connectors do not give `Username` one universal product meaning:

- X, Instagram, Threads, and Pinterest use a provider username or handle.
- YouTube prefers a channel handle or custom URL and falls back to the channel title.
- Facebook uses a Page name because the current connector does not obtain a separate Page username.
- LinkedIn currently uses email while retaining the human name separately as `DisplayName`.

`account_name` must therefore remain documented as a legacy resolved identifier, not a guaranteed friendly label. TikTok must populate it with the true username to correct the existing connector intent, while new explicit `username` and `display_name` fields provide stable presentation semantics.

### 5.8 Contributing factor: stored TikTok avatars are expiring provider URLs

The TikTok connector stores the raw `avatar_url` returned by TikTok. The investigated value was a signed TikTok CDN URL with provider-controlled expiry parameters; it was not copied to or served from UniPost-managed storage.

Returning that stored value as a new `/accounts` contract would create broken-image risk after expiry and would not provide durable identity evidence. P0 therefore does not add `account_avatar_url` to `/accounts`. A future avatar contract requires either authenticated live profile retrieval or an approved proxy/hosting and retention design.

## 6. Approaches considered

### 6.1 Correct identity fields while preserving the managed row — selected

Request TikTok `username`, map it correctly, continue updating the existing managed row, and expose explicit identity and last-connection fields.

Benefits:

- Fixes the root cause at the provider boundary.
- Preserves posts, analytics, deliveries, and account references.
- Makes the current destination human-verifiable.
- Is additive for API consumers except for correcting the ambiguous legacy `account_name` value.

### 6.2 Create a new UniPost account row for every TikTok identity switch — rejected

This would make the ID visibly change, but it would split account history and create ambiguous ownership of existing posts, analytics, scheduled content, and delivery jobs. It would also contradict the current one-managed-account-per-external-user-and-platform contract.

### 6.3 Add cache invalidation or force browser logout — rejected

Production evidence shows that the callback persisted new credentials and profile data. The misleading response is reproducible from current database values and source mapping. Cache invalidation would not turn `display_name=Robyn` into `username=robyn6608` and would leave the API contract defective.

## 7. Goals

1. Store and return the TikTok username authorized by the user.
2. Keep TikTok username and display name as separate values.
3. Make the selected TikTok destination recognizable in `/accounts` without interpreting `open_id`.
4. Preserve the existing managed account ID during a same-slot reconnect.
5. Expose the time of the latest successful Hosted Connect separately from row creation and token refresh.
6. Add regression coverage for a TikTok account whose username differs from its display name.
7. Improve support evidence for future provider-identity transitions without recording credentials.
8. Preserve current ownership, Workspace isolation, and plan-limit behavior.

## 8. Non-goals

- Changing the managed-account uniqueness key.
- Creating a new account ID when the same external user reconnects TikTok.
- Deleting or rewriting historical posts, analytics, or delivery results.
- Changing TikTok OAuth scopes; `user.info.profile` is already granted.
- Adding a feature flag.
- Redesigning Hosted Connect UI.
- Treating `display_name` as a stable account identifier.
- Storing OAuth codes, raw tokens, provider response bodies, or signed avatar URLs in new logs.
- Adding stored TikTok avatar URLs to `/accounts` before a durable proxy or live-fetch contract exists.
- Solving provider account selection inside TikTok's own consent UI.
- Applying the TikTok field change to unrelated platforms without separate evidence.

## 9. Product and API requirements

### 9.1 TikTok profile request

The managed TikTok connector must request:

```text
open_id,username,display_name,avatar_url
```

The request remains authorized by the existing scopes.

If TikTok omits `username` despite a successful response:

- the connection may complete using `display_name` as a backward-compatible fallback;
- the connector must leave internal `Username` empty so the shared callback falls back to `DisplayName` only for `account_name` and success messaging;
- the stored metadata must omit the verified username, set `username_missing=true`, and avoid falsely claiming that the display name is a username; and
- a sanitized diagnostic must record `username_missing=true` without token or raw-response data.

An empty `open_id` remains a hard provider-identity failure.

### 9.2 Internal profile mapping

The connector must map:

```text
ExternalAccountID = open_id
Username          = username, or empty when TikTok omits it
DisplayName       = display_name
AvatarURL         = avatar_url
```

The connector must not copy `DisplayName` into `Username` as a fallback. The existing callback may resolve the customer-facing legacy `account_name` from `Username` and then `DisplayName`, but verified username metadata and API fields must remain empty when TikTok omits `username`. Tests must distinguish a true username from the display-name compatibility value.

### 9.3 Persistence behavior

On TikTok connect or reconnect, the existing managed row must update:

- encrypted access token;
- encrypted refresh token;
- token expiry;
- `external_account_id` from `open_id`;
- `account_name` from true username, falling back to display name only when necessary;
- account avatar;
- `metadata.username` from true username when present;
- `metadata.display_name` from display name;
- `metadata.username_missing=true` when TikTok omits username, and no `username_missing` marker when it is present;
- `metadata.tiktok_identity_schema_version=2` whenever the post-fix TikTok profile mapping completes, including the explicit missing-username path;
- scopes;
- current Connect Session ID;
- status and disconnection state; and
- dedicated latest connection time.

The existing row ID must remain unchanged for the same `(profile_id, platform, external_user_id)` slot.

### 9.4 Dedicated latest connection time

Add a nullable `last_connected_at TIMESTAMPTZ` to `social_accounts`.

Rules:

- A successful OAuth or Bluesky connect save sets `last_connected_at` to the callback completion time.
- Background token refresh must not change `last_connected_at`.
- New BYO accounts may leave it null unless their connection flow has an equivalent user-authorized completion point.
- Backfill managed accounts from the latest completed Connect Session that references the account.
- If no completed session exists, backfill managed accounts from `connected_at` and mark the value as historical fallback in migration documentation.

`connected_at` retains its existing meaning: when the UniPost account row was created.

### 9.5 `/accounts` response contract

Add these fields to account list responses:

```json
{
  "account_name": "robyn6608",
  "username": "robyn6608",
  "display_name": "Robyn",
  "identity_refresh_required": false,
  "connected_at": "2026-06-17T21:42:27Z",
  "last_connected_at": "2026-07-31T20:42:26Z"
}
```

Rules:

- `username`, `display_name`, and `last_connected_at` are additive nullable fields.
- `identity_refresh_required` is an additive boolean and is `true` only for active managed TikTok rows that have not completed the post-fix identity fetch and therefore do not carry `metadata.tiktok_identity_schema_version=2`.
- TikTok `account_name` becomes the true username when present, matching the existing callback intent to prefer `Profile.Username`.
- `display_name` remains available for friendly UI presentation.
- A legacy TikTok row without the v2 marker returns `username=null`, preserves its existing `account_name` as a compatibility value, and returns `identity_refresh_required=true`; the API must not present the legacy duplicated metadata value as a verified username.
- A post-fix row with the v2 marker and `username_missing=true` returns `username=null` and `identity_refresh_required=false`; it keeps the display-name fallback for `account_name` and must not repeatedly prompt the customer to reconnect solely for the same provider omission.
- The transition detector must not use `username == display_name`, because valid TikTok accounts can intentionally use the same value for both fields.
- P0 does not add `account_avatar_url` to the list response. Existing documentation that shows it in a List Accounts example must be corrected to match the production backend contract.
- Existing `external_account_id` remains unchanged in the contract.
- Profile-nested and Workspace-wide account list routes must return the same identity semantics.
- OpenAPI definitions, generated clients, SDK types, and documentation examples must be updated together.

### 9.6 Dashboard identity presentation and transition state

For a v2 TikTok identity, Dashboard surfaces that represent a publishing destination must render:

- primary label: `display_name`, falling back to `username`, then the legacy `account_name`;
- secondary label: `@username` when a verified username exists; and
- connection recency from `last_connected_at`, while preserving the original `connected_at` label where row age matters.

For a legacy managed TikTok row with `identity_refresh_required=true`:

- keep the existing friendly label so the account remains recognizable;
- show concise inline guidance: `Reconnect TikTok to refresh the account username`;
- provide the existing reconnect action rather than inventing a second OAuth entry point; and
- do not mark the account disconnected or block otherwise-valid publishing solely because identity metadata is legacy.

The Accounts page, account selectors used before publishing, and managed-user account detail must consume the same shared label rule. MySuperX and SDK consumers receive the explicit fields and decide their own layout, but documentation must recommend display name as the friendly label and username as the handle.

This is a low-density account-row treatment, not a new warning card or modal. The reconnect guidance must remain adjacent to the affected account identity and must not obscure connection status.

### 9.7 Avatar provenance and P0 boundary

The stored TikTok `account_avatar_url` remains internal provider profile data and may expire. It is not a UniPost-proxied asset and is not added to `/accounts` in this release.

Existing live TikTok profile and creator-info endpoints may continue returning provider avatar URLs under their current contracts. Any future durable avatar field in `/accounts` requires a separate decision covering refresh cadence, proxy or object-storage ownership, retention, authorization, provider terms, and failure fallback.

### 9.8 Safe identity-transition diagnostics

On a successful managed reconnect, the callback should record safe bounded metadata:

- `account_username`
- `account_display_name`
- `identity_changed` boolean
- `connect_session_id`
- `social_account_id`

Do not add access tokens, refresh tokens, OAuth codes, raw provider response bodies, or complete signed avatar URLs.

If recording provider identifiers is necessary for internal audit, store a one-way keyed digest of the previous and next provider identity rather than exposing raw `open_id` values in user-visible logs. Key management and retention must reuse an approved internal security mechanism; otherwise defer the digests and keep only `identity_changed`.

## 10. Current and target data flow

### 10.1 Current flow

```text
TikTok consent shows @robyn6608
  → token exchange succeeds
  → User Info returns open_id + display_name=Robyn + avatar
  → connector sets Username=Robyn
  → managed upsert preserves UniPost account ID
  → /accounts returns account_name=Robyn + original connected_at
  → customer concludes old account was restored
```

### 10.2 Target flow

```text
TikTok consent shows @robyn6608
  → token exchange succeeds
  → User Info returns open_id + username=robyn6608 + display_name=Robyn + avatar
  → connector keeps username and display name separate
  → managed upsert preserves UniPost account ID, records identity schema v2, and records last_connected_at
  → /accounts returns username=robyn6608, display_name=Robyn, and latest connection time
  → customer can verify the destination despite the stable UniPost ID
```

## 11. Implementation plan

### Phase 1: Reproduce and lock the correct contract

1. Add a failing connector test with `username=robyn6608` and `display_name=Robyn`.
2. Assert that the User Info request includes `username`.
3. Assert that internal `Username` and `DisplayName` remain distinct.
4. Add callback persistence coverage proving that distinct fields reach `account_name` and metadata correctly.

### Phase 2: Correct TikTok identity ingestion

1. Extend the TikTok User Info response model with `username`.
2. Request `username` in `FetchProfile`.
3. Map username and display name separately.
4. Implement the explicit missing-username path without copying display name into the verified `Username` field.
5. Update callback event and success-message fallbacks so an empty verified username still uses the resolved display name for customer-facing compatibility.
6. Persist `tiktok_identity_schema_version=2` whenever the corrected mapping completes and persist `username_missing=true` only when applicable.

### Phase 3: Add connection-time semantics

1. Add the nullable `last_connected_at` migration.
2. Backfill managed rows using completed Connect Sessions, with documented fallback.
3. Update managed create, reconnect, and upsert queries to set the field.
4. Confirm the token refresh worker does not update it.
5. Regenerate sqlc output and update query-contract tests.

### Phase 4: Extend Accounts API and clients

1. Add explicit username, display name, identity-refresh state, and last connection fields to the API response.
2. Apply the same contract to Profile and Workspace account lists.
3. Update OpenAPI, generated SDKs, examples, and relevant Dashboard types.
4. Correct the current List Accounts documentation example, which shows `account_avatar_url` even though the production response omits it.
5. Ensure existing consumers of `account_name` tolerate the TikTok correction from display name to username.
6. Implement one shared TikTok label rule: display name as the primary label and `@username` as the secondary handle.
7. Add the inline reconnect guidance and existing reconnect action for `identity_refresh_required=true` without blocking publishing.

### Phase 5: Add safe diagnostics

1. Detect whether the provider identity changed before applying the managed upsert.
2. Record the safe fields defined in §9.8.
3. Avoid raw identifiers in user-visible logs unless separately approved.
4. Document how support verifies a future identity switch.

### Phase 6: Preview and deployed acceptance

1. Push only the owned task branch and open a Draft PR to `dev`.
2. Complete local CI, Railway PR Environment deployment, Vercel Preview, deployed regression, and Codex browser acceptance on the exact PR head SHA.
3. Verify the target scenario in Preview using a TikTok account whose username differs from its display name.
4. Merge to `dev` only after every Preview gate passes.
5. Wait for the development deployment and repeat acceptance on the official development domains.
6. Promote to staging and production only when the user separately authorizes the standard release flow.

## 12. Test requirements

### 12.1 Unit tests

- TikTok authorization continues to request the approved scopes.
- Every standard TikTok Connect Session includes `user.info.profile` in its authorization request.
- User Info requests `open_id,username,display_name,avatar_url`.
- `username=robyn6608` and `display_name=Robyn` remain distinct.
- Missing username follows the documented fallback.
- Missing username leaves internal `Username` empty while retaining `DisplayName`.
- Missing `open_id` fails closed.
- No error includes an access token or raw provider response containing credentials.

### 12.2 Persistence and ownership tests

- A new managed TikTok account stores username when present, display name, avatar, identity schema v2, and latest connection time.
- A post-fix TikTok response without username stores `username_missing=true`, does not duplicate display name into verified username metadata, and still completes with a display-name `account_name` fallback.
- Reconnecting the same TikTok `open_id` preserves the account ID and updates credentials and profile data.
- Reconnecting a different TikTok `open_id` for the same external-user slot preserves the account ID and replaces the provider identity fields.
- Provider-identity ownership conflicts continue to fail closed.
- A token refresh does not modify `last_connected_at`.
- A disconnect does not erase the last successful connection time.

### 12.3 API contract tests

- Profile-scoped and Workspace-scoped lists return the new fields.
- TikTok `account_name` and `username` return `robyn6608` while `display_name` returns `Robyn`.
- A legacy TikTok row returns `username=null` and `identity_refresh_required=true` instead of claiming that duplicated display-name metadata is a verified username.
- A corrected TikTok row returns `identity_refresh_required=false`.
- A corrected TikTok row with `username_missing=true` returns `username=null` and `identity_refresh_required=false`.
- List Accounts does not add an expiring raw TikTok avatar URL.
- Existing fields retain their names and types.
- Managed User and Workspace isolation remain unchanged.
- Disconnected-account filtering remains unchanged.

### 12.4 Migration tests

- The migration is forward- and rollback-safe.
- Backfill chooses the latest completed Connect Session for each managed account.
- Accounts without a completed session use the documented fallback.
- The migration does not modify tokens, ownership, status, or provider identity.

### 12.5 Dashboard regression tests

- A corrected TikTok account renders `Robyn` as its primary label and `@robyn6608` as its secondary handle.
- A legacy TikTok account renders inline reconnect guidance based on `identity_refresh_required`, not on username/display-name equality.
- A post-fix TikTok account with `username_missing=true` keeps its display-name label and does not render the legacy reconnect guidance.
- The reconnect control reuses the existing Connect flow.
- A legacy identity remains publishable when its connection status is active.
- Account selectors and managed-user account detail use the same identity-label helper.

### 12.6 Required local validation

From `api/`:

```text
GOCACHE=/tmp/unipost-go-build go test ./...
```

If Dashboard types or rendering change, from `dashboard/`:

```text
npm run build
npm run test:regression:dashboard
```

## 13. Acceptance criteria

The fix is accepted only when all of the following are true on the same deployed SHA:

1. TikTok OAuth shows and authorizes `robyn6608`.
2. The completed Connect Session references the expected managed account.
3. The UniPost managed account ID remains stable across reconnect.
4. The stored `external_account_id` comes from the latest TikTok `open_id`.
5. The stored and returned username is `robyn6608`.
6. The stored and returned display name is `Robyn`.
7. The corrected row carries `tiktok_identity_schema_version=2` and returns `identity_refresh_required=false`.
8. A pre-fix legacy row returns `username=null`, `identity_refresh_required=true`, and the approved reconnect guidance.
9. A post-fix response that omits username returns `username=null` and `identity_refresh_required=false` without creating an endless reconnect prompt.
10. Dashboard renders `Robyn` as the friendly label and `@robyn6608` as the handle after reconnect.
11. `last_connected_at` advances after the successful reconnect.
12. `connected_at` remains the original row-creation time.
13. A background token refresh does not advance `last_connected_at`.
14. `/accounts` exposes enough human-readable data to distinguish the destination without interpreting `open_id`.
15. `/accounts` does not newly expose an expiring raw TikTok avatar URL.
16. Existing posts, analytics, delivery history, and scheduled content still reference the stable account ID.
17. No credential or raw provider response appears in logs, responses, fixtures, or artifacts.
18. All required local, Preview, deployed regression, and browser acceptance checks pass on the exact PR head SHA before merge.

## 14. Rollout and rollback

### 14.1 Rollout

- No feature flag is required because the fix corrects an existing field contract and the database change is additive.
- Deploy backend and schema changes before or with any consumer that relies on the new fields.
- The username correction is prospective. Existing TikTok rows never stored a trustworthy username and cannot be safely relabeled from database state alone.
- Every affected managed TikTok account needs one successful post-fix reconnect before `username` and `account_name` become authoritative under identity schema v2.
- Until that reconnect, `/accounts` returns `identity_refresh_required=true` and Dashboard offers the existing reconnect action with inline guidance. Publishing remains available when the underlying connection is active.
- Do not infer legacy state from `username == display_name`; use the explicit schema marker.
- Monitor TikTok Hosted Connect success and failure rates, profile-fetch failures, and missing-username fallbacks.
- Support should communicate that a stable managed account ID is expected and verify the explicit username and latest connection time.

### 14.2 Rollback

- Application code can roll back while leaving the additive nullable column in place.
- Do not restore the incorrect `display_name → username` mapping as a permanent workaround.
- If TikTok unexpectedly rejects the `username` field despite granted scope, use the documented fallback and record sanitized diagnostics while investigating provider behavior.
- Do not delete or recreate the customer's managed account as a rollback action.

## 15. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Some TikTok tokens omit `username` | Explicit fallback plus sanitized diagnostic and test coverage |
| TikTok continues omitting username after a post-fix reconnect | Mark the completed v2 fetch separately from `username_missing` so Dashboard does not create an endless reconnect loop |
| Existing consumers expect `account_name` to be a display name | Add explicit `display_name`; update Dashboard to render display name plus handle; document `account_name` as a legacy resolved identifier |
| Existing accounts still show legacy identity after deployment | Mark pre-v2 rows explicitly and provide one post-fix reconnect path without blocking publishing |
| Username and display name happen to be equal | Detect legacy state with an explicit schema version, never value equality |
| Stored TikTok avatar URL expires | Do not add it to `/accounts`; defer durable avatar delivery to a separate proxy or live-fetch design |
| Migration misstates historical connection time | Prefer completed Connect Session; document `connected_at` fallback |
| New diagnostics expose provider identifiers | Log only safe names and `identity_changed`; use keyed digests only after security approval |
| Stable ID remains confusing | Return explicit identity and `last_connected_at`; document managed-slot semantics |
| Fix is mistaken for a cache change | Preserve investigation evidence and test the provider-field mapping directly |

## 16. Support disposition for the investigated account

The investigated account should not be deleted, recreated, or manually reassigned based on the evidence currently available.

Support can state:

- The latest TikTok OAuth callback completed successfully.
- UniPost refreshed the managed account at `2026-07-31 20:42:26 UTC`.
- The current profile data matches the public `@robyn6608` nickname and avatar.
- The unchanged UniPost managed account ID is expected.
- The misleading value `Robyn` is TikTok's display name being presented as the username.
- Engineering has identified an Accounts API identity-field defect; reconnects performed before deployment cannot repair the missing username.
- After the fix reaches the customer's environment, the account needs one successful reconnect so UniPost can fetch `username=robyn6608`, mark the identity as v2, and clear the reconnect guidance.
- The customer should not delete the UniPost managed account; the normal reconnect flow updates the existing row and preserves its history.

## 17. Open questions for implementation review

The review resolves two prior questions:

- Missing username uses the explicit display-name fallback and does not fail an otherwise valid publishing connection.
- Identity-transition digests are deferred unless an approved keyed-digest facility already exists.

One release-planning question remains:

1. Which published SDK versions must receive the additive Accounts API fields in the same release?

## 18. Decision summary

- This is not primarily a cache incident.
- The confirmed root cause is `display_name` being used as TikTok username.
- The current production account is strongly corroborated as `@robyn6608` by nickname and avatar alignment.
- Stable managed account ID reuse is correct and remains unchanged.
- The selected fix corrects identity ingestion, exposes explicit API fields, marks legacy rows for one post-fix reconnect, adds true last-connection semantics, and improves safe diagnostics.
- Dashboard presents display name as the friendly label and username as the handle; it does not silently replace one with the other.
- Raw TikTok avatar URLs remain outside the new `/accounts` contract until UniPost defines durable proxy or live-fetch semantics.
- Implementation and release remain pending user review and authorization.
