# Facebook Hosted Connect Error Guidance and Outcome Logging PRD

**Status:** Approved design; external review findings resolved; written PRD ready for user review

**Date:** 2026-07-24

**Branch:** `hotfix-facebook-connect-error-guidance`

**Base:** latest `origin/staging` at hotfix creation (`2c488080a2598d1b88134a1d265c1ae5a0352dba`)

**Owner areas:** Hosted Connect, Facebook integration, Developer Logs

**Release path:** `hotfix-facebook-connect-error-guidance` → `staging` → `main` → sync the same hotfix branch to `dev`

## 1. Executive summary

A production Facebook Hosted Connect attempt for a third-party Facebook user completed the OAuth token exchange but found no Facebook Page that the user could publish to. The API recorded the real provider-stage cause internally, then collapsed every `ExchangeCode` failure into the public reason `token_exchange_failed`. The no-`return_url` Hosted Connect page rendered that internal reason directly as `Connection failed: token_exchange_failed`.

This behavior is misleading: the user is told that authorization failed even though authorization succeeded and the actionable problem is Page availability or Page permissions. It also leaks an internal implementation code instead of telling the user how to recover.

This hotfix introduces Facebook-only, typed Page discovery failures and stable public error codes. It distinguishes an empty Page list from a non-empty Page list with no publishable Page, renders approved user guidance on both Hosted Connect response paths, keeps retryable sessions pending, and prevents raw connector errors from reaching the browser.

The hotfix also establishes a platform-wide Hosted Connect outcome logging invariant. For every OAuth callback or Bluesky form submission that can be attributed to a valid Connect Session and Workspace, UniPost makes exactly one synchronous attempt to persist a user-visible success, failure, or cancellation result before responding. Under normal log-database availability that attempt produces exactly one result row; explicit fail-open behavior applies when the insert fails. Users can find persisted results in Workspace Logs by account-connection category, platform, status, request ID, Connect Session ID, or external user ID.

The Facebook error experience remains deliberately scoped to Facebook. The outcome-log completeness rule applies to every Hosted Connect platform, including the non-OAuth Bluesky form, because a partial platform history would be inconsistent for users.

## 2. Incident statement

### 2.1 User impact

- A Facebook user with no accessible Page, or no Page publishing permission, sees `Connection failed: token_exchange_failed`.
- The message implies a token or system failure and provides no recovery instructions.
- Customer support and integrators cannot reliably distinguish Page availability problems from genuine Meta OAuth or API failures using the browser result.
- Hosted Connect outcomes are already logged on some paths, but several attributable callback failures only reach service logs. Workspace users therefore cannot rely on Developer Logs as a complete record of connection attempts.

### 2.2 Confirmed production evidence

The investigated attempt had these properties:

- Platform: Facebook.
- Hosted Connect callback reached the production API.
- Short-lived and long-lived Facebook user token exchange succeeded.
- The subsequent Page discovery stage returned no Page with an accepted publishing task.
- The production integration log stored the internal detail `facebook connect found no Pages with publish permissions`.
- The callback logged and redirected with `error_code=token_exchange_failed`.
- The server-rendered Hosted Connect page displayed `Connection failed: token_exchange_failed`.
- The Connect Session remained `pending`, allowing a retry while the original link remained valid.

No credential, OAuth code, access token, API key, or user email from the investigation may be copied into this PRD, implementation, tests, fixtures, logs, screenshots, or release artifacts.

### 2.3 Current Meta readiness context

The application is published. `pages_show_list` and `pages_read_engagement` are ready to publish. `pages_messaging` is unrelated to Page discovery and Page publishing for this flow and remains outside this hotfix. This incident is not evidence that the Facebook App failed token exchange or that Facebook Login is globally unavailable.

## 3. Current behavior and root cause

### 3.1 Connector collapses Page states

`api/internal/connect/facebook.go` performs these operations inside `ExchangeCode`:

1. Exchange the OAuth code for a short-lived user token.
2. Exchange it for a long-lived user token.
3. Request `/me/accounts` with Page fields and tasks.
4. Select the first Page with `CREATE_CONTENT`, `MANAGE`, or `MODERATE`.
5. Return a generic Go error when no publishable Page is selected.

The current selection result does not distinguish:

- Meta returned zero Pages; and
- Meta returned one or more Pages, but none had a publishing task.

### 3.2 Callback erases the real stage

`api/internal/handler/connect_callback.go` treats every `ExchangeCode` error as `token_exchange_failed`. It writes that error code to integration logs and sends the same reason to the response path. Page discovery happens after token exchange but is therefore presented as a token exchange failure.

### 3.3 Hosted Connect exposes internal reasons

When no `return_url` is configured, the API renders `Connection failed: <reason>` directly. When a `return_url` is configured, the Dashboard Hosted Connect page passes unknown values through its current error humanizer. Neither path has a safe Facebook-specific presentation contract.

### 3.4 Outcome logging is path-dependent and asynchronous

The callback already emits:

- `account.connect.callback_succeeded` on the normal success path; and
- `account.connect.callback_failed` on some provider, token, profile, persistence, and subscription failures.

However, connector resolution, ownership, plan, encryption, session completion, and other attributable failures can return without a Workspace integration log. Existing callback logging also uses the general asynchronous log queue, which can drop events when full. The result is not a dependable user-facing connection history.

### 3.5 Raw Meta response bodies enter existing error channels

The current Facebook connector embeds `string(body)` in errors for four non-200 provider responses:

- short-lived token exchange;
- long-lived token exchange;
- `/me/accounts` Page discovery; and
- `/me` Page profile fetch.

The callback then logs the error through structured service logging and, on exchange/profile failures, serializes `err.Error()` into `integration_logs.response_payload.error`. Redaction by JSON key cannot make an arbitrary provider error string safe. The current implementation can therefore persist or emit the complete Meta response body even when the public redirect reason is generic.

Removing this existing leak is a mandatory part of the hotfix, not an incidental benefit of the new outcome recorder. The connector must stop constructing errors from raw response bodies, and Hosted Connect outcome events must stop serializing unbounded `err.Error()` values into user-visible payloads.

## 4. Goals

1. Tell a Facebook Hosted Connect user whether UniPost found no accessible Page or found Pages without publishing permission.
2. Give the user clear recovery instructions without claiming more than Meta's response proves.
3. Keep genuine Facebook OAuth, network, or Meta API failures separate from Page eligibility failures.
4. Prevent internal error strings and provider response bodies from reaching public pages or redirect URLs.
5. Preserve the current successful Facebook Page selection behavior.
6. Keep retryable Facebook Page discovery and authorization failures in `pending` state.
7. Ensure every attributable Hosted Connect OAuth callback or Bluesky form submission makes exactly one synchronous result-log insert attempt and, under normal log-database availability, persists exactly one user-visible outcome row.
8. Make a specific connection attempt findable in Workspace Logs using its Connect Session ID or external user ID.
9. Preserve Workspace isolation and existing log retention rules.
10. Release through staging and production with negative and positive Facebook acceptance coverage.

## 5. Non-goals

- Requesting or reviewing new Meta permissions.
- Changing `pages_messaging`, Facebook Messenger behavior, Inbox behavior, or webhook subscriptions.
- Connecting Facebook personal profiles; UniPost continues to connect Facebook Pages.
- Adding Page selection when multiple publishable Pages are returned. The first publishable Page remains the current contract.
- Adding a retry button to the error page.
- Extending typed, user-actionable connector errors to Instagram, Threads, X, LinkedIn, or other platforms in this release.
- Redesigning the Hosted Connect page or Workspace Logs page.
- Creating a database outbox or a new Connect Attempt table.
- Treating a browser close before the provider callback as a failed attempt.
- Changing plan retention periods or Workspace Logs authorization.
- Adding a feature flag.

## 6. Product behavior

### 6.1 User-facing Facebook outcomes

| Provider result | Public reason | Page title | User message |
| --- | --- | --- | --- |
| `/me/accounts` returns an empty list | `facebook_page_not_available` | `Facebook Page unavailable` | `We couldn’t find a Facebook Page this account can manage or has allowed UniPost to access. Make sure this Facebook account manages a Page and that UniPost is allowed to access it, or ask a Page admin to grant you access. Then open the original connection link and try again.` |
| One or more Pages are returned, but none has `CREATE_CONTENT`, `MANAGE`, or `MODERATE` | `facebook_page_permission_required` | `Facebook Page permission required` | `Your Facebook account can access a Page, but it doesn’t have permission to publish content. Ask a Page admin to grant you Facebook content-management access, then open the original connection link and try again.` |
| Any other Facebook token, network, response-decode, Page endpoint, or profile-stage authorization failure covered by this presentation | `facebook_authorization_failed` | `Connection failed` | `Facebook authorization couldn’t be completed. Please try again later or contact the developer who sent you the link.` |

The empty-list message intentionally covers the observable cause without claiming which Meta configuration produced it. An empty `/me/accounts` response does not prove that the person owns no Facebook Page; it can also mean the Facebook user has no Page role or did not grant UniPost effective access to any Page. The hotfix does not claim that one specific permission checkbox was declined because the empty list alone cannot prove that.

### 6.2 Response-path parity

The same reason must have the same meaning on both Hosted Connect response paths:

1. **No `return_url`:** the API renders the approved title and message directly.
2. **With `return_url`:** the API appends only the stable public `reason`; UniPost's Hosted Connect frontend maps that reason to the approved title and message.

An external customer-provided `return_url` receives the same stable reason code but remains responsible for its own presentation.

### 6.3 Retry and Session state

- `facebook_page_not_available`, `facebook_page_permission_required`, and `facebook_authorization_failed` leave the Connect Session `pending`.
- The user can correct Page access and reopen the original link while it is still valid.
- If the original link has expired, the customer must create a new Connection Session.
- No retry button is added in this release because the response page does not consistently retain a safe, reusable original Hosted Connect URL on every return path.
- User cancellation remains terminal and marks the Session `cancelled`.
- Successful connection marks the Session `completed` through the existing completion claim.

## 7. Facebook error contract

### 7.1 Typed connector failures

The Facebook connector must return programmatically identifiable errors for the two actionable Page discovery states. The callback must use typed classification, such as `errors.Is` or `errors.As`, rather than matching error text.

The stable classifications are:

- `facebook_page_not_available`
- `facebook_page_permission_required`

The connector must retain the current selection rule: the first Page containing `CREATE_CONTENT`, `MANAGE`, or `MODERATE` is selected. A Page with an accepted publishing task but no Page access token is not a permission-required result; it is an unexpected provider-contract failure and is presented publicly as `facebook_authorization_failed`.

### 7.2 Public and internal classification

The callback maintains two layers:

- **Public reason:** safe, stable, documented, and suitable for a URL or user interface.
- **Internal diagnostic class:** suitable for service diagnosis and user-visible Developer Logs after sanitization.

For Page discovery failures, the public reason and Developer Logs `error_code` are the Facebook-specific stable codes. For unexpected Facebook exchange-stage failures, the public reason is `facebook_authorization_failed`; internal service telemetry may retain a more specific safe stage such as `short_token_exchange`, `long_token_exchange`, `page_fetch`, or `profile_fetch`.

The callback must never place `err.Error()`, a Meta response body, or another unbounded connector string in a redirect URL or HTML response.

Existing `ResponsePayload: {"error": err.Error()}` writes on Hosted Connect exchange, profile, and persistence failures must be removed from result events or replaced with fixed, bounded fields. A new public reason allowlist is not sufficient while an equivalent raw value remains in Workspace Logs or service logs.

### 7.3 Safe Page diagnostics

Actionable Page errors may add only these counts to the user-visible result log:

- `page_count`
- `publishable_page_count`

Page IDs, Page names, Page access tokens, the long-lived user token, and the complete `/me/accounts` response are excluded.

For other Facebook provider errors, the connector must parse a bounded safe provider-error type. Safe diagnostics may include the operation/stage, remote HTTP status, and parsed numeric Meta error code/subcode. Its `Error()` output must not contain the raw body or any token. Raw response bodies and authorization material are excluded from the returned error, structured service logs, integration-log payloads, and fallback telemetry.

## 8. Unified Hosted Connect outcome logging

### 8.1 Invariant

Under normal integration-log database availability, every Hosted Connect OAuth callback or Bluesky form submission that can be attributed to a valid Connect Session and Workspace must make one synchronous result-log write:

> Attempt exactly one synchronous result-event insert for that connection attempt before returning HTML or redirecting the browser; when the insert succeeds, exactly one user-visible result row exists.

The result event is per attempt, not per Session. A retry using the same still-pending Session produces another result event with the same `connect_session_id` and a different `request_id`. A database write failure follows the explicit fail-open behavior in §8.6 and is not described as a persisted result. The existing `account.connect.callback_*` action names are retained for compatibility and also apply to the Bluesky form even though that platform has no OAuth callback.

The existing `account.connect.session_created` event is a lifecycle event and is not counted as a callback result.

### 8.2 Result actions

| Outcome | Action | Status | Level | Error code |
| --- | --- | --- | --- | --- |
| Connection completed | `account.connect.callback_succeeded` | `success` | `info` | omitted |
| Connection attempt failed | `account.connect.callback_failed` | `error` | `error` | stable failure code |
| User denied/cancelled authorization | `account.connect.callback_cancelled` | `warning` | `warning` | safe provider cancellation code, normally `access_denied` |

### 8.3 Required fields

Every attributable result includes:

- `workspace_id`
- `category=oauth`
- `source=oauth`
- `action`
- `status`
- `level`
- `message`
- `request_id`
- `profile_id`
- `platform`
- `metadata.connect_session_id`
- `metadata.external_user_id`
- `metadata.callback_status`
- `metadata.connection_type=managed`

A success event additionally includes:

- top-level `social_account_id`
- safe `metadata.account_name` when available

A failure or cancellation event additionally includes:

- top-level `error_code`
- safe, error-specific metadata allowed by this PRD

The event must not include the third-party user's email.

### 8.4 User-visible messages

Result messages are concise and safe to show in Workspace Logs:

- `Hosted Connect completed successfully.`
- `Hosted Connect failed: no manageable Facebook Page was found.`
- `Hosted Connect failed: Facebook Page publishing permission is required.`
- `Hosted Connect failed during authorization.`
- `Hosted Connect was cancelled by the user.`

Other platforms retain their existing stable error codes, but all attributable paths use a safe, bounded result message instead of an unbounded internal error.

### 8.5 Central outcome recorder

The OAuth callback and Bluesky form handler must replace path-by-path result writes with a shared attempt outcome recorder:

1. Resolve a valid Session and Workspace without recording submitted credentials.
2. Initialize one attempt outcome bound to the request ID, Session, Workspace, profile, and platform.
3. Each subsequent return path sets the final outcome classification and safe metadata.
4. Before the response or redirect is sent, the recorder synchronously inserts one integration log row.
5. The recorder prevents a second result write for the same in-process attempt.

Existing non-result service diagnostics may remain, but they must not create additional `account.connect.callback_*` result rows for the same attempt.

### 8.6 Synchronous persistence behavior

Hosted Connect result events use a synchronous integration-log write path rather than the bounded asynchronous queue. This path must return an error to the caller and must use a bounded database timeout. Both the OAuth callback handler and Bluesky form handler use this path.

If the result-log insert fails:

- an already successful account connection is not rolled back or reported as a connection failure;
- the intended connection outcome is still returned to the browser;
- a structured service error and metric identify `hosted_connect_outcome_log_write_failed` with safe Workspace, platform, action, and request identifiers;
- no token, OAuth code, raw provider response, or API key appears in the fallback service log.

This provides an application-level persistence guarantee under normal database availability, not an absolute delivery guarantee during database failure. A transactional outbox is explicitly deferred.

Success, failure, and cancellation all use the synchronous path. Keeping success asynchronous would preserve the existing queue-full loss mode and would violate the approved requirement that users can reliably find both successful and failed Hosted Connect attempts. Callback latency from the bounded insert must be measured and included in staging regression review.

### 8.7 Attribution exceptions

No Workspace result log can be safely guaranteed when:

- `state` is absent or invalid;
- no Connect Session can be resolved;
- the Session cannot be associated with a Workspace;
- rate limiting rejects the request before safe attribution;
- the browser closes or abandons the provider flow before callback or form submission;
- the database is unavailable for both business state and log persistence.

These paths emit sanitized service telemetry only. They must not guess a Workspace from browser-controlled input.

## 9. Developer Logs discovery and API behavior

### 9.1 Existing filters

Users can locate outcome events using:

- category `oauth`, shown in the Dashboard as `Account connection`;
- platform;
- status;
- source `oauth`;
- request ID;
- error code;
- social account after a successful connection.

### 9.2 Connect Session and external user search

The existing `q` search contract is extended to exactly match the safe ID metadata fields when the complete ID is supplied:

- `metadata.connect_session_id`
- `metadata.external_user_id`

This requires no schema migration. The list query compares these two JSON object fields using exact equality while retaining its existing free-text behavior for message, action, request ID, post ID, and error code. Substring `ILIKE` matching is not added for either metadata ID.

The Dashboard Workspace Logs search and live-tail client filter must use the same searchable fields so a result does not disappear when live mode is active.

The initial exact metadata predicates remain unindexed and are acceptable only within the existing Workspace, time-range, and retention bounds at current volume. Query latency must be observed in staging. Expression indexes or dedicated columns are a future scaling option and are not added by this hotfix.

### 9.3 Documentation

Developer Logs documentation must include:

- success, failure, and cancellation actions;
- examples of filtering `category=oauth`, `platform=facebook`, and `status=error`;
- the three public Facebook reason/error codes;
- Connect Session ID and external user ID search behavior;
- the fact that list endpoints omit request and response payloads while the detail endpoint returns only redacted payloads.

## 10. Security and privacy invariants

1. OAuth code, access token, refresh token, Page token, API key, client secret, and authorization headers never enter public reasons, user-visible metadata, payloads, service fallback logs, tests, or artifacts.
2. Raw Meta response bodies never enter connector error strings, Hosted Connect result logs, or structured service logs. All four current Facebook `string(body)` error constructions are removed or replaced by bounded parsed provider errors.
3. Facebook redirect reasons come from a fixed allowlist; browser-controlled Facebook `error_description` is not reflected verbatim. Other platforms keep their current public presentation in this release, while their Workspace result logs still use bounded messages and safe error codes.
4. A Connect Session ID may be logged only after server-side Session resolution and Workspace attribution. OAuth state is never logged.
5. Third-party user email is not included in result events.
6. Workspace Logs remain scoped exclusively by the authenticated Workspace context. A caller-supplied Workspace ID is ignored.
7. Search over metadata remains within the existing Workspace predicate.
8. Synchronous logging failure never exposes database details to the Hosted Connect user.
9. Previously exposed or pasted credentials are not reused in acceptance fixtures and must be rotated independently.

## 11. Failure and state model

| Stage | Example failure | Session state | Public result | Workspace result log |
| --- | --- | --- | --- | --- |
| Provider consent | User selects Cancel | `cancelled` | cancelled | warning/cancelled |
| Provider consent | Non-cancellation provider error | `pending` | safe authorization failure | error/failed |
| Short or long token exchange | Meta/network failure | `pending` | `facebook_authorization_failed` | error/failed |
| Page discovery | Empty list | `pending` | `facebook_page_not_available` | error/failed |
| Page permission validation | No publishable Page | `pending` | `facebook_page_permission_required` | error/failed |
| Page/profile provider contract | Missing token or invalid response | `pending` | `facebook_authorization_failed` | error/failed |
| Bluesky credential validation | Provider rejects the handle or app password | `pending` | existing safe Bluesky form error | error/failed |
| Ownership/plan/persistence | Existing stable callback errors | existing behavior | existing safe response | error/failed |
| Completion claim | Completed by current request | `completed` | success | success/succeeded |
| Repeat callback | Session already terminal | unchanged | already-used failure | error/failed when Workspace attribution succeeds |

## 12. Test strategy

### 12.1 Facebook connector unit tests

- Empty `data` returns the typed Page-not-available classification.
- A non-empty list with only unrelated tasks returns the typed Page-permission-required classification.
- A list with unavailable Pages followed by one publishable Page still selects the first publishable Page.
- Each accepted task (`CREATE_CONTENT`, `MANAGE`, `MODERATE`) is covered.
- A publishable Page without an access token is classified as an unexpected Facebook authorization/provider failure, not a permission error.
- Each of the four existing non-200 paths proves that raw bodies, tokens, and provider messages do not appear in the returned error, service-log fields, result-log payload, public reason, redirect, or HTML.
- Invalid JSON responses do not leak raw bodies or credentials into the public classification.

### 12.2 OAuth callback handler tests

- Each Facebook typed error produces the approved public reason.
- Unexpected Facebook exchange errors produce `facebook_authorization_failed`.
- No-`return_url` responses contain the approved title and message and exclude internal errors.
- `return_url` redirects contain only the stable reason and exclude provider text.
- Page and authorization failures leave the Session pending.
- Cancellation marks the Session cancelled.
- Success marks the Session completed.
- Every attributable success, failure, and cancellation path makes exactly one synchronous result-event insert attempt.
- Multiple internal failure checks in one attempt cannot emit duplicate result events.
- Result-log insertion occurs before response/redirect completion.
- A result-log write failure does not change the established business outcome and emits sanitized service telemetry.
- A forced result-log write failure produces no false assertion that a user-visible result row was persisted.
- Non-attributable callbacks do not write to a guessed Workspace.
- Every registered OAuth Hosted Connect platform is covered by a table-driven success/failure outcome-log contract test.
- Other Hosted Connect platforms retain their callback presentation behavior while gaining complete outcome logging.

### 12.3 Bluesky Hosted Connect tests

- Every attributable Bluesky form success and failure makes exactly one synchronous result-event insert attempt through the shared recorder.
- Invalid credentials, ownership conflicts, plan failures, encryption failures, save failures, completion-claim failures, and success receive safe stable classifications.
- The submitted handle may be represented only through an approved safe account identifier; the app password is absent from every event, fallback service log, response payload, and test diagnostic.
- Form submissions that cannot resolve a Session or Workspace do not write to a guessed Workspace.
- Existing credential-validation, ownership, completion-claim, and no-password-echo behavior remains intact.

### 12.4 Integration-log tests

- The synchronous write path persists normalized and redacted events.
- Workspace scope remains mandatory for list and detail queries.
- `q` exactly matches complete `connect_session_id` and `external_user_id` values inside metadata only within the authenticated Workspace; partial ID fragments do not match these metadata fields.
- List responses continue to omit request and response payloads.
- Detail responses contain only redacted payloads.
- WebSocket/SSE delivery carries the persisted result event.

### 12.5 Dashboard tests

- Each Facebook public reason maps to the approved title and body.
- Unknown Facebook reasons map to the generic Facebook authorization message instead of echoing the input.
- Success and cancellation pages continue to render correctly.
- Non-Facebook Hosted Connect presentation remains unchanged.
- Workspace Logs free-text search matches Connect Session and external user metadata for loaded and live events.
- Account-connection category, platform, status, source, request ID, and error-code filters continue to work.

### 12.6 Required local validation

Before the hotfix PR is eligible to merge:

1. From `api/`: `GOCACHE=/tmp/unipost-go-build go test ./...`
2. From `dashboard/`: `npm run build`
3. From `dashboard/`: `npm run test:regression:dashboard` when Playwright browsers are installed
4. Any new focused Dashboard source/contract tests

A failed, timed-out, cancelled, skipped, non-starting, empty, or different-SHA required check is a failure and blocks promotion.

## 13. Acceptance criteria

### 13.1 Staging browser acceptance

Use controlled test accounts for three scenarios:

1. **No accessible Page**
   - Complete Facebook consent with an account for which `/me/accounts` is empty.
   - Observe `Facebook Page unavailable` and the approved recovery text.
   - Confirm the Session remains pending.
   - Confirm one failed Workspace Log with `facebook_page_not_available`, `page_count=0`, and `publishable_page_count=0`.

2. **Page visible without publishing permission**
   - Complete Facebook consent with an account that can see a Page but has none of the accepted publishing tasks.
   - Observe `Facebook Page permission required` and the approved recovery text.
   - Confirm the Session remains pending.
   - Confirm one failed Workspace Log with `facebook_page_permission_required`, a positive `page_count`, and `publishable_page_count=0`.

3. **Publishable Page**
   - Complete Facebook consent with a Page-publishing account.
   - Observe the existing success page or customer return redirect.
   - Confirm the Session is completed and the social account is active.
   - Confirm one success Workspace Log with the resulting `social_account_id`.

For all scenarios:

- the exact log is searchable by Connect Session ID and external user ID;
- the log has the connection attempt request ID;
- no OAuth code, token, API key, email, or raw Meta body appears in URL, HTML, page source, Workspace Logs, or captured service output;
- refreshing or retrying produces a separate attempt log without duplicating a single attempt;
- non-Facebook Hosted Connect smoke coverage proves no presentation regression.

Unified outcome-log staging acceptance additionally covers:

- one representative non-Facebook OAuth success or controlled failure, proving the shared OAuth recorder on a deployed service;
- one Bluesky form success or controlled invalid-credential failure, proving the non-OAuth recorder path;
- exactly one result row per attributable attempt in both cases;
- automated handler coverage for every remaining registered OAuth Hosted Connect platform.

### 13.2 Production acceptance

After production deployment:

- verify API and Dashboard health;
- create fresh, bounded Facebook Hosted Connect sessions with non-exposed credentials;
- verify at least the no-accessible-Page negative path and a publishable-Page success path;
- verify both outcomes appear once in the owning Workspace Logs and are searchable by Connect Session ID;
- confirm the deployed SHA matches the promoted production SHA;
- confirm no unrelated integration-log volume or error-rate regression.

## 14. Rollout and release plan

### 14.1 Hotfix branch to staging

1. Implement only on `hotfix-facebook-connect-error-guidance`, based on the recorded `origin/staging` SHA.
2. Run all required local validation.
3. Audit the exact commits and files unique to the branch.
4. Push the owned hotfix branch and open a pull request to `staging`.
5. Wait for every required and visibly triggered check to succeed on the exact head SHA.
6. Merge only after the audit and checks pass.
7. Wait for staging Railway and Vercel deployments to finish.
8. Complete all staging acceptance scenarios on staging domains.

### 14.2 Staging to production

1. Audit the exact commits and changed files in `staging` relative to `main`.
2. Open a production PR from `staging` to `main`; never bypass staging.
3. Wait for all required checks to succeed on the exact promotion SHA.
4. Merge, wait for production deployment, and perform production acceptance.
5. Stop immediately on any required failure or SHA mismatch.

### 14.3 Sync back to development

1. After production acceptance succeeds, sync the same owned hotfix branch with the latest `origin/dev`.
2. Stop and ask for direction if conflicts prevent a clean sync.
3. Rerun required validation.
4. Open a PR from the hotfix branch to `dev`.
5. Complete Preview Acceptance on the exact PR head SHA.
6. Merge only after all Preview gates pass.
7. Wait for the official development deployment and verify the same outcomes on development domains.

## 15. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Empty Page list is misrepresented as proof that no Page exists | Use wording that covers missing Page and missing Page access |
| Error classification regresses successful Page selection | Preserve the selection algorithm and add mixed-list tests |
| Internal provider details leak through fallback paths | Fixed public allowlist, bounded log fields, negative leakage assertions |
| Central result recorder creates duplicate logs | Single in-process finalizer and exactly-one tests across return paths |
| Synchronous log writes add callback latency | One bounded insert only after attribution; record duration and alert on failures |
| Exact metadata lookup has no expression index | Keep equality predicates inside Workspace/time/retention bounds, measure staging latency, and defer indexing until volume justifies it |
| Log database failure occurs after a successful connection | Do not reverse business success; emit sanitized service telemetry and metric |
| All-platform logging work causes presentation changes | Separate platform-neutral outcome recording from Facebook-only public presentation |
| Existing external consumers depend on `token_exchange_failed` for Facebook | Document the replacement public reasons and preserve safe internal diagnostic stages |

## 16. Implementation surfaces

The implementation plan is expected to touch these areas, subject to code-level verification after this PRD is approved:

- Facebook connector classification and tests.
- Hosted Connect callback outcome handling and tests.
- Bluesky Hosted Connect form outcome handling and tests.
- A shared synchronous Hosted Connect outcome recorder used by both handler families.
- API server-rendered Connect error presentation.
- Dashboard Hosted Connect error presentation and tests.
- Integration logger synchronous write interface and tests.
- Integration-log search query and generated database code.
- Workspace Logs live search behavior.
- Developer Logs and Hosted Connect documentation.

No database schema migration or feature flag is expected.

## 17. Definition of done

The hotfix is complete only when:

- all approved Facebook messages and public reason codes are implemented;
- Page availability and Page permission failures are distinct and retryable;
- internal errors are absent from public responses;
- every attributable Hosted Connect OAuth callback and Bluesky form submission makes exactly one synchronous result-log insert attempt; under normal log-database availability exactly one Workspace result row is persisted, while forced write failure follows the documented fail-open telemetry path;
- users can find attempts by Connect Session ID and external user ID;
- automated tests and local CI-equivalent checks pass;
- staging negative and positive Facebook acceptance passes on the exact deployed SHA;
- production acceptance passes on the exact deployed SHA;
- the change is synced back to `dev` and verified in the development environment;
- no unrelated commits or files are promoted;
- no credential or sensitive provider payload appears in code, logs, test artifacts, or documentation.

## 18. Open questions

None. Product behavior, Facebook-only error scope, no-retry-button scope, all-platform outcome logging including Bluesky, synchronous persistence, searchability, testing, and hotfix release path were approved during design review.
