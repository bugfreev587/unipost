# X Account Read SDKs and Guidance Design

**Date:** 2026-07-31

**Status:** Revised after external review; pending final user approval

## 1. Summary

UniPost has released public OpenAPI endpoints for reading the live profile and authored post history of a connected X account:

- `GET /v1/accounts/{account_id}/profile`
- `GET /v1/accounts/{account_id}/posts`

The production API is usable through raw HTTP, but the JavaScript, Python, Go, and Java SDKs do not yet expose these operations. The public API Reference currently documents the raw endpoints, but it does not provide complete SDK examples or a task-oriented integration guide.

This project will publish a backward-compatible `0.7.0` release of all four SDKs, complete the related API Reference coverage, and add a public guide at `/docs/guides/x/profile-and-post-history`.

The X Credits billing rollout remains controlled by the existing `x_credits_billing_v1` Feature Flag. The account-read endpoints remain available while the flag is disabled; only customer Credits accounting and Credits-specific documentation are conditional.

## 2. Goals

1. Add first-class profile and authored-post-history operations to all four official SDKs.
2. Preserve the complete response envelope so applications can use pagination, idempotency replay state, request IDs, and Credits settlement metadata.
3. Publish all four SDKs as version `0.7.0` without breaking existing applications.
4. Add JavaScript, Python, Go, and Java examples to the affected API Reference pages.
5. Add a task-oriented guide for reading an X profile and walking authored post history safely.
6. Keep Credits documentation and Credits-only examples aligned with the server-authoritative Feature Flag.
7. Validate the source SDKs, the published packages, the documentation Preview, and the deployed development environment before declaring completion.

## 3. Non-goals

- Changing the production OpenAPI endpoint behavior or billing state machine.
- Looking up arbitrary X users that are not represented by an authorized connected UniPost account.
- Hiding the profile or post-history endpoints when X Credits billing is disabled.
- Adding local Unleash access or Feature Flag caching to any SDK.
- Automatically aggregating multiple X pages into one unbounded SDK request.
- Building a background synchronization framework, scheduler, database schema, or webhook feed for account history.
- Redesigning existing SDK resource organization or converting all dynamic SDK responses to strongly typed models.
- Changing existing SDK runtime requirements, dependencies, timeout defaults, retry defaults, authentication behavior, or error classification. The release does add read-only access to server error metadata that the current SDKs discard.

## 4. Compatibility Contract

Version `0.7.0` is an additive feature release. A user application must be able to replace its current SDK version with `0.7.0`, rebuild, and continue using every existing method without source changes.

The release must satisfy these constraints:

- No existing public method is renamed or removed.
- No existing method gains a new required parameter.
- No existing method changes its return type or unwrapping behavior.
- No SDK constructor gains a new required argument.
- Default authentication, timeout, retry, redirect, and error classification remains unchanged.
- Existing error classes, constructors, status values, and code-selection rules remain compatible. Additive error surfaces expose read-only `details`, `is_retriable`, and server `Retry-After` metadata so new account-read recovery flows are usable without parsing raw bodies; Go uses a wrapper for new operations instead of expanding its existing public `APIError` struct.
- New profile, post-history, and Credits types use new names rather than changing unrelated existing types.
- Existing `capabilities` return types remain unchanged:
  - JavaScript: `Record<string, unknown>`
  - Python: `dict[str, Any]`
  - Go: `JSONMap`
  - Java: `JsonNode`
- The additive `x_account_reads` capability namespace is documented, but does not force an incompatible return-type migration.
- Existing Go structs are not expanded merely to expose this feature. This avoids breaking callers that use positional, unkeyed struct literals.
- Java adds methods only to the existing concrete `AccountsResource`; it does not change public interfaces or constructors.
- The SDK release does not introduce new mandatory runtime dependencies or raise supported runtime versions.

Each SDK repository will include a compatibility test that compiles or executes representative pre-`0.7.0` calls against the new source.

## 5. SDK Resource Design

The new operations belong to the existing Accounts resource because both reads are scoped to one connected account. A separate X-only top-level resource would fragment account operations and introduce unnecessary navigation differences across SDKs.

### 5.1 Public method names

| SDK | Profile method | Post-history method |
| --- | --- | --- |
| JavaScript | `client.accounts.getProfile(accountId, params)` | `client.accounts.listPosts(accountId, params)` |
| Python sync | `client.accounts.get_profile(account_id, ...)` | `client.accounts.list_posts(account_id, ...)` |
| Python async | `await client.accounts.get_profile(account_id, ...)` | `await client.accounts.list_posts(account_id, ...)` |
| Go | `client.Accounts.Profile(ctx, accountID, params)` | `client.Accounts.ListPosts(ctx, accountID, params)` |
| Java | `client.accounts().profile(accountId, params)` | `client.accounts().listPosts(accountId, params)` |

The names follow each SDK's current conventions while retaining the same conceptual API.

### 5.2 Profile request

The profile method accepts:

- required connected `account_id` path value;
- required `external_user_id` managed-user selector;
- required `idempotency_key`, serialized as the `Idempotency-Key` header.

The SDK validates nonblank `account_id`, `external_user_id`, and `idempotency_key` values before sending the new request. Workspace ownership, platform, managed-user scope, account authorization, Credits admission, and provider behavior remain server-authoritative.

### 5.3 Post-history request

The post-history method accepts:

- required connected `account_id` path value;
- required `external_user_id`;
- required `limit`, from 5 through 100;
- required `idempotency_key` for that logical page request;
- optional opaque `cursor`;
- optional `start_time` inclusive lower bound;
- optional `end_time` exclusive upper bound;
- optional `exclude_reposts`;
- optional `exclude_replies_to_others`.

The SDK validates the documented `limit` range and rejects a locally supplied `end_time` that is not after `start_time`. The server repeats those checks. `start_time` is inclusive and `end_time` is exclusive under the upstream X timeline contract; UniPost passes both values to `GET /2/users/{id}/tweets` without changing their boundary semantics.

The SDK sends one bounded API request per method call. It does not automatically traverse `next_cursor`, reduce `limit`, or generate replacement idempotency keys.

### 5.4 Idempotency behavior

The SDK exposes the required idempotency key as an explicit named option. The caller owns the logical key lifecycle:

- Retry the exact same logical profile or page request with the same key.
- Use a new key for a different account, selector, filter set, time range, limit, or continuation page.
- When the API supplies `error.details.retry_cursor`, retry with that cursor and the same key after the delay in the HTTP `Retry-After` header.

SDKs must not silently generate a key because doing so would prevent a process restart from safely replaying the same logical operation.

### 5.5 Response envelopes

Both new SDK methods return the complete API envelope instead of only `data`.

The profile response exposes:

- normalized X profile data;
- `meta.credits`;
- `meta.replayed` when present;
- `request_id`.

The post-history response exposes:

- normalized authored posts;
- `meta.limit`;
- `meta.scanned_count`;
- `meta.returned_count`;
- `meta.has_more`;
- `meta.next_cursor`;
- `meta.cursor_expires_at`;
- `meta.credits`;
- `meta.replayed` when present;
- `request_id`.

Credits metadata includes the operation ID, settlement status, `accounting_enabled`, billing mode, `bypass_reason`, operation, estimated amount, reserved amount, charged amount, released amount, and catalog version. Fields that the API may omit are optional in SDK types. `bypass_reason` distinguishes at least `feature_disabled` from `customer_x_app` when customer accounting is bypassed.

The server omits `meta.replayed` when it is false. SDK response types therefore make it optional and define absence as `false`.

### 5.6 Language-specific typing

- JavaScript exports interfaces for profile requests, post-history requests, normalized profile data, normalized authored posts, media, public metrics, thread metadata, Credits settlement metadata, and both full response envelopes.
- Python adds dataclasses or the repository's established typed response representation for the same concepts and implements both sync and async paths.
- Go adds new request, data, metadata, and envelope structs in the Accounts surface. Existing structs remain unchanged.
- Java follows the current SDK architecture: request options are represented by explicit parameter objects or maps consistent with existing resources, and responses remain `JsonNode`/`Page`-compatible unless the repository already has an established typed model for the same shape. The implementation must not introduce an isolated object-mapping framework solely for this feature.

All new public model names use the `XAccount` prefix so they cannot collide with the existing workspace `Profile` types. Representative names are `XAccountProfileParams`, `XAccountProfile`, `XAccountProfileResponse`, `XAccountPostsParams`, `XAccountPost`, `XAccountPostsMeta`, and `XAccountPostsResponse`, translated only for each language's normal casing conventions.

### 5.7 Additive error metadata

Account-read recovery depends on structured fields that the server already returns but the current SDKs do not consistently expose. Version `0.7.0` adds optional read-only metadata to each language's error surface without changing existing constructors or code classification:

- raw structured `details` for `retry_cursor`, `retry_cursor_expires_at`, `estimated_credits`, `available_credits`, and `max_affordable_limit`;
- `is_retriable` from the response body;
- `retry_after` in seconds, sourced from the HTTP `Retry-After` header and available for retriable `409` and `429` responses.

JavaScript and Python add optional properties to their existing base error classes. Go does not add fields to the existing public `APIError`, because doing so would break applications that construct it with an unkeyed literal. New account-read methods instead return an `XAccountReadError` wrapper containing structured details and header-derived retry metadata while implementing `Unwrap() error` to expose the existing `*APIError` to `errors.As`; existing endpoint errors remain unchanged. Java preserves its current constructor and adds an overload plus read-only accessors; `getResponseBody()` remains available.

HTTP layers pass response headers into error parsing. Existing automatic `429` retries retain their current retry count and cap behavior, while the terminal error reports the actual sanitized server delay rather than a body default. The SDK does not automatically retry the `READ_IN_PROGRESS` or `READ_SETTLEMENT_PENDING` `409` states; callers inspect `is_retriable` and `retry_after` and replay with the same idempotency key.

## 6. X Credits SDK Surface

The account-read response types always contain Credits metadata because the server returns the accounting mode even when customer accounting is bypassed.

The `0.7.0` release will also expose the two already-public Credits inspection endpoints through the SDKs:

- `GET /v1/billing/x-credits`
- `GET /v1/billing/x-credits/events`

These methods are additive and grouped under a billing/X Credits resource that follows each SDK's existing resource conventions. They do not evaluate Feature Flags locally.

When `x_credits_billing_v1` is disabled, the server remains authoritative and returns HTTP `403 FEATURE_NOT_AVAILABLE` for Credits inspection. The SDK preserves that stable API error code through its existing error mechanism. The account profile and post-history methods remain usable and return `meta.credits.accounting_enabled=false` plus the applicable `bypass_reason` for bypassed customer accounting.

## 7. Feature Flag and Documentation Behavior

The existing `x_credits_billing_v1` flag controls customer X Credits accounting and public Credits documentation. It does not control account-read endpoint availability.

### 7.1 Always available

- `GET /v1/accounts/{account_id}/profile` API Reference.
- `GET /v1/accounts/{account_id}/posts` API Reference.
- The new `/docs/guides/x/profile-and-post-history` guide.
- SDK methods and response types for profile and post history.
- Documentation explaining `meta.credits.accounting_enabled` as the source of truth for that response.

### 7.2 Available only when the flag is enabled

- `/docs/api/x-credits`.
- `/docs/guides/x/credits`.
- Navigation and search-index entries for Credits-only pages.
- Credits balance checks, allowance planning, hard-limit handling, and `402 INSUFFICIENT_X_CREDITS` examples in the new guide.
- Credits-only related links on the profile and post-history API Reference pages.

When the flag is disabled, the new guide shows a short neutral note that customer Credits accounting is not enabled and directs readers to inspect `meta.credits.accounting_enabled`. It must not show balance-management steps, quota calculations, or a link to a hidden Credits page.

The implementation reuses the existing documentation controls rather than introducing a second flag system:

- `getPublicDocsFeatureFlags` for server-rendered conditional sections;
- `usePublicDocsFeatureFlags` for client-rendered API Reference sections;
- `DOCS_PATH_FEATURES` for whole Credits-only pages;
- `required_feature: "x_credits_billing_v1"` for Credits-only AI search chunks inside an otherwise public guide.

The SDK never connects to Unleash. The UniPost backend remains the authority for all sensitive and billable decisions.

## 8. API Reference Completion

The affected API Reference pages will be completed as follows:

### 8.1 Account capabilities

The `x_account_reads` namespace, authorization state, page-size range, accounting state, and `bypass_reason` are already published. This project only corrects any discovered inaccuracies and updates the existing four-language capability examples to show how an application checks the new reads before calling them. It does not rewrite the shipped capability reference.

### 8.2 X account profile

Add JavaScript, Python, Go, and Java SDK tabs alongside cURL. Document:

- required account and managed-user scope;
- required idempotency key;
- complete normalized profile fields;
- replay metadata;
- Credits accounting metadata;
- stable errors and retry behavior.

Credits-only examples and related links are conditional on the public Feature Flag.

### 8.3 X authored posts

Add JavaScript, Python, Go, and Java SDK tabs alongside cURL. Document:

- bounded `limit` behavior;
- time bounds and filters;
- the difference between `scanned_count` and `returned_count`;
- opaque cursor scope and expiry;
- one new idempotency key per logical page;
- same-key recovery with `retry_cursor`;
- normalized content, thread, media, and public metrics fields.

Credits-only examples and related links are conditional on the public Feature Flag.

### 8.4 X Credits

When the flag is enabled, add four-language SDK examples for balance and event inspection. The pages remain unavailable through public docs routing, navigation, and search while the flag is disabled.

## 9. Guidance Page

Create `/docs/guides/x/profile-and-post-history` with the title **Read X profiles and post history**.

The page is task-oriented and contains:

1. Prerequisites: API key, an active connected X account explicitly bound to an owning `external_user_id`, and a capability check. A workspace-owned account without a Managed User binding is not accessible through these endpoints.
2. Reading the current profile with an explicit idempotency key.
3. Reading the first authored-post page with a bounded limit.
4. Persisting and following `next_cursor` before `cursor_expires_at`.
5. Generating a new idempotency key for each new page while reusing the key for exact retries.
6. Filtering reposts and replies to others while retaining self-replies used in threads.
7. Deduplicating stored posts by `external_post_id`.
8. Handling `INVALID_CURSOR`, provider rate limits, `READ_IN_PROGRESS`, `READ_SETTLEMENT_PENDING`, `retry_cursor`, and cursor expiry.
9. Conditional Credits admission and insufficient-balance handling when the Feature Flag is enabled.
10. Complete JavaScript, Python, Go, Java, and cURL examples.

The guide links to the profile, post-history, capabilities, errors, authentication, `/docs/guides/x/reconnect-permissions`, and conditionally available Credits references. It explains that profile reads require `users.read`, post-history reads require `users.read` and `tweet.read`, and both require `offline.access` plus a valid persisted X app identity. It is added to the X Guides navigation, X platform guide, API Reference related-guide cards, guide index, and docs AI search index.

The page follows the existing DocsPage guide layout and design system. It does not introduce a new page shell or visual system.

## 10. Repository and Release Flow

### 10.1 UniPost documentation repository

Documentation changes remain on the conversation-owned task branch, are pushed to a Draft pull request targeting `dev`, and complete Preview Acceptance on the exact head SHA. After merge, the official development deployment is verified in a browser before any release promotion.

### 10.2 SDK repositories

The live repository state was rechecked on 2026-07-31:

- `sdk-js` main and latest tag are `0.6.2` / `v0.6.2`.
- `sdk-python`, `sdk-go`, and `sdk-java` main and latest tags are `0.6.0` / `v0.6.0`.
- The historical JavaScript managed-user branch is behind main and its pull request is already merged. Although that branch history temporarily prepared `0.7.0`, no `v0.7.0` tag or npm `0.7.0` artifact exists.
- npm, PyPI, the Go module proxy, and Maven Central expose no `0.7.0` artifact, so the version is available for this coordinated release.
- `github.com/unipost-dev/sdk-go` exists, resolves publicly, and already publishes `v0.6.0`; the unresolved-Go note in `docs/sdk-release.md` is stale and will be corrected in this project.

Each SDK repository uses a dedicated task branch for the feature implementation. Changes are reviewed and validated through that repository's CI before merging to its default branch. After all four feature merges, the existing `scripts/release/create-sdk-release.sh 0.7.0 --push` workflow performs the lockstep version bump, source-validation hard gate, release commits, tags, and pushes. It runs against clean, conversation-owned SDK clones supplied through `UNIPOST_DEV_ROOT`, not shared SDK checkouts from another task.

Release tags are created only after the feature merge commits and source-validation gates are present on each default branch:

- JavaScript: `v0.7.0`, published as `@unipost/sdk@0.7.0`.
- Python: `v0.7.0`, published as `unipost==0.7.0`.
- Go: `v0.7.0`, published from `github.com/unipost-dev/sdk-go`.
- Java: `v0.7.0`, published as the repository's existing Maven artifact.

The JavaScript built `dist` output is regenerated and included according to its existing release policy. `docs/sdk-release.md` is updated to describe the live repositories and the `UNIPOST_DEV_ROOT` isolation requirement. No tag is moved or reused. A failed publication is repaired with a new commit and, when the registry forbids replacement, a new version rather than overwriting an existing artifact.

## 11. Testing and Acceptance

### 11.1 SDK unit and compatibility tests

Each SDK must test:

- correct profile path, query, and `Idempotency-Key` header;
- correct post-history query serialization for every optional parameter;
- preservation of `false` boolean filters;
- complete response-envelope parsing;
- Credits metadata parsing for enabled and bypassed accounting;
- pagination metadata and opaque cursor preservation;
- stable API error propagation for `IDEMPOTENCY_CONFLICT`, `READ_IN_PROGRESS`, `READ_SETTLEMENT_PENDING`, `INSUFFICIENT_X_CREDITS`, `RATE_LIMITED`, `ACCOUNT_REAUTHORIZATION_REQUIRED`, `X_UPSTREAM_ERROR`, `INVALID_CURSOR`, `IDEMPOTENCY_KEY_REQUIRED`, `WRONG_PLATFORM`, `ACCOUNT_ACCESS_DENIED`, and HTTP `403 FEATURE_NOT_AVAILABLE`;
- structured parsing of `details`, `is_retriable`, and header-derived `retry_after`, including terminal errors after the existing automatic `429` retry loop;
- compatibility of representative pre-`0.7.0` calls;
- package version and User-Agent alignment.

Python tests cover sync and async clients. JavaScript validates generated declaration files and the packaged `dist`. Go validates the public module. Java validates tests, Javadocs, and local Maven publication.

### 11.2 Source validation against development

The UniPost source-validation suite is extended for all four SDKs. Using an approved, non-customer connected X fixture, each SDK performs:

- capability read;
- one live profile read;
- one live post-history read with the minimum safe limit;
- one exact idempotent replay that does not create a second charge;
- one continuation request when a safe next cursor exists;
- assertion that response metadata and account scope are correct.

Provider reads remain bounded. The fixture account and API key are never logged.

### 11.3 Feature Flag acceptance

The documentation and API behavior are verified in both states:

- Flag off: account reads remain available, customer accounting is bypassed, `accounting_enabled=false`, Credits-only docs are unavailable, and the general guide omits Credits-only instructions.
- Flag on in an approved development fixture: Credits-specific guide content appears, Credits inspection methods succeed, estimates are enforced before provider access, settlement metadata is returned, and insufficient balance is denied before the provider call.

### 11.4 Documentation acceptance

Required checks include:

- dashboard production build;
- dashboard regression suite for docs routing and shared shell changes;
- source tests for navigation, related links, SDK tabs, AI search chunks, and Feature Flag filtering;
- Preview browser acceptance on desktop and narrow viewport;
- official development browser acceptance after merge.

### 11.5 Published-package acceptance

After registry publication, clean temporary consumers install the exact `0.7.0` artifact from npm, PyPI, the Go module proxy/source repository, and Maven. Each consumer compiles and runs a safe request against the development API. Validation must exercise the published artifact rather than local source.

## 12. Rollout and Rollback

The new SDK methods are inert until an application calls them, so publishing is low risk to existing users. The documentation is released after Preview Acceptance and may precede or coincide with package availability only if version labels and install commands are accurate.

Rollback rules:

- Existing SDK versions remain available; registries are not destructively overwritten.
- A package defect is fixed in a new patch release rather than deleting or replacing `0.7.0`.
- Credits customer accounting can be contained immediately by disabling `x_credits_billing_v1` in the affected environment.
- Disabling the Credits flag must not disable profile or post-history reads.
- Documentation Credits pages and conditional sections disappear when the flag is disabled.

## 13. Completion Criteria

The project is complete only when:

- all four SDK repositories contain the approved additive API and compatibility tests;
- all four repository CI suites pass on their exact release commits;
- `0.7.0` is available from all four public distribution channels;
- published-package regression passes for all four exact artifacts;
- the API Reference contains complete four-language examples;
- `/docs/guides/x/profile-and-post-history` is deployed and linked from the intended documentation surfaces;
- both Feature Flag documentation paths are verified;
- the official development environment has passed browser acceptance;
- no existing SDK public call requires application changes after upgrade.
