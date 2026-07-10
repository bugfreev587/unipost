# UniPost - Instagram OAuth 190 Reconnect Required PRD
**Turn Meta login-challenge publish failures into actionable reconnect-required failures**
Status: Review
Owner: Publishing / Platform Adapters / Admin
Created: 2026-07-09

---

## 1. Background

On 2026-07-09, UniPost investigated a customer account shown in Admin Users with a high red error count:

- Email: `virusokellen2020@gmail.com`
- User ID: `user_3Ffa0cG0jkMcMAdM5fKdQH6bIxI`
- Workspace: `longhornmedia kft`
- Plan: Basic
- Admin Users metrics at investigation time:
  - `724 / 2,500` posts used this month
  - `19` scheduled posts
  - `99` failed posts this month

The red `99` in Admin Users is `failed_posts_this_month`. It counts distinct posts created this month where either:

- `social_posts.status = 'failed'`, or
- at least one related `social_post_results.status = 'failed'`.

It does not count every internal failure event. The related `post_failures` table had `404` events this month because retries, worker stale recovery, container processing timeouts, and other intermediate attempts are recorded separately.

The investigated customer is a high-volume API customer with many connected managed social accounts. The error count is therefore amplified by batch publishing across many accounts.

---

## 2. Failure Summary

### 2.1 Overall publish volume

Current-month post distribution for the customer:

| Status | Source | Posts |
| --- | --- | ---: |
| `published` | `api` | 1,265 |
| `failed` | `api` | 99 |
| `scheduled` | `api` | 19 |
| `publishing` | `api` | 4 |

All relevant failed posts came from API-created posts.

### 2.2 Failed result breakdown

The `99` failed post results this month break down as:

| Platform | Failed results | Distinct accounts |
| --- | ---: | ---: |
| Instagram | 82 | 16 |
| TikTok | 17 | 3 |

Recent failed posts accelerated in the last three UTC days:

| UTC day | Failed posts |
| --- | ---: |
| 2026-07-07 | 21 |
| 2026-07-08 | 20 |
| 2026-07-09 | 28 |

### 2.3 Main Instagram failure

The dominant Instagram failure is currently stored as:

```text
error_code=platform_error
failure_stage=dispatch
message=failed to get Instagram user ID
next_action=contact_support
```

However, redacted debug curls show the real upstream response:

```text
GET https://graph.instagram.com/v21.0/me?fields=id
HTTP 400
{
  "error": {
    "message": "Error validating access token: You cannot access the app till you log in to www.instagram.com and follow the instructions given.",
    "type": "OAuthException",
    "code": 190,
    "error_subcode": 0
  }
}
```

This means the affected Instagram accounts need the account owner to log in to Instagram and complete the platform-required instructions. It is an account/token action, not a generic UniPost platform error.

Observed scope:

- `68` Instagram failed results had `failed to get Instagram user ID`.
- All `68` had redacted debug payloads consistent with Meta OAuth code `190` login challenge.
- The remaining `14` Instagram failed results were other categories, including disconnected accounts, container/media processing errors, and temporary platform errors. The expected direct impact of this PRD is the `68` OAuth code `190` user-ID lookup failures and future failures of the same shape.
- The largest repeated account buckets included:
  - `paigeturner199908`: 25 failures
  - `haileyadams972026`: 8 failures
  - `leoparker11003`: 8 failures
  - `kiss84635`: 5 failures
  - several other Instagram accounts with 3-4 failures each

### 2.4 Main TikTok failures

TikTok failures are separate from this PRD's primary fix:

| Failure | Count | Interpretation |
| --- | ---: | --- |
| `auth_token_invalid`, refresh returned `invalid_grant` | 8 | One TikTok account has an invalid or expired refresh token and needs reconnect. |
| `platform_error`, `spam_risk` | 4 | TikTok rejected the content/account as spam risk. |
| older `access_token_invalid` upload-init failures | 4 | Token was invalid for earlier attempts. |
| upload chunk HTTP 500 | 1 | Upstream TikTok upload failure. |

The existing TikTok invalid-token path already maps to `auth_token_invalid` and marks accounts as reconnect required. This PRD does not change TikTok taxonomy, but the reconnect-required validation gate proposed below should also prevent newly created posts from targeting TikTok accounts already marked `reconnect_required`.

---

## 3. Problem

UniPost currently loses the actionable Meta error details at the Instagram user-ID lookup boundary.

`api/internal/platform/instagram.go` calls:

```text
GET https://graph.instagram.com/v21.0/me?fields=id
```

When Meta returns HTTP 400 OAuth code `190`, the adapter decodes the body into `{ id }`, sees no `id`, and returns:

```text
failed to get Instagram user ID
```

That generic message has three bad effects:

1. `postfailures.Classify` cannot see the Meta OAuth code `190`.
2. The failure is persisted as `platform_error` instead of `account_reconnect_required`.
3. `recordPostFailure` does not call `MarkSocialAccountReconnectRequired`, so the account can remain `active` and continue receiving publish attempts that are likely to fail the same way.

There is also a second, blocking product gap: marking an account `reconnect_required` currently does not stop new publish attempts from selecting it. The publish validators build `ValidateAccount.Disconnected` from `disconnected_at` only. `MarkSocialAccountReconnectRequired` sets `status='reconnect_required'` and leaves `disconnected_at` empty, so the account is still treated as valid publish inventory by newly created posts.

That means diagnostics and account marking alone do not deliver churn prevention. The implementation must also make `status='reconnect_required'` block new publish validation in the same places that `disconnected_at` blocks it today.

The admin-facing error is less useful than the evidence UniPost already has in debug curl logs, and the current validation gate allows the same account state to keep producing new failures.

---

## 4. Goals

1. Preserve sanitized Meta error evidence from Instagram user-ID lookup failures.
2. Classify Meta OAuth code `190` from Instagram user-ID lookup as `account_reconnect_required`.
3. Automatically mark affected active social accounts as `reconnect_required`.
4. Make Admin Errors show `account_reconnect_required` and `next_action=reconnect_account` for new matching failures.
5. Avoid repeated new-post publish churn against accounts that are already marked `reconnect_required`.
6. Keep the change additive and backward-compatible for API consumers.

---

## 5. Non-goals

- Do not automatically reconnect Instagram accounts on behalf of users.
- Do not disconnect accounts; mark them `reconnect_required` using the existing account status path.
- Do not alter Admin Users failed-post counting.
- Do not modify historical failure rows in production.
- Do not add a feature flag. This is a narrow backend classification and diagnostics fix; UniPost's default rule is no feature flag unless explicitly requested.
- Do not change TikTok failure taxonomy in this PRD.
- Do not guarantee that posts already accepted into an in-flight batch or queue snapshot will be stopped after one account in that same batch is newly marked `reconnect_required`.
- Do not expose access tokens, signed URLs, Authorization headers, or unredacted provider payloads.

---

## 6. Proposed Behavior

### 6.1 Current behavior

For an Instagram OAuth code `190` response during user-ID lookup:

```text
stored error_code: platform_error
stored failure_stage: dispatch
stored message: failed to get Instagram user ID
stored platform_error_code: empty
stored next_action: contact_support
social_accounts.status: remains active
```

### 6.2 Target behavior

For the same upstream response:

```text
stored error_code: account_reconnect_required
stored failure_stage: dispatch
stored message: instagram get user id failed (400): {"error":{...,"code":190,...}}
stored platform_error_code: 190
stored next_action: reconnect_account
social_accounts.status: reconnect_required
social_accounts.metadata.reconnect_required_at: set
future newly validated posts to that account: rejected as account disconnected/reconnect required
```

The public and admin response should remain structurally compatible. Existing fields are reused:

- `error_message`
- `error_code`
- `failure_stage`
- `platform_error_code`
- `is_retriable`
- `next_action`

If `provider_error` is already derived by the existing taxonomy, it should identify:

```json
{
  "provider": "meta",
  "http_status": 400,
  "code": "190",
  "type": "OAuthException"
}
```

---

## 7. Implementation Requirements

### 7.1 Preserve Instagram user-ID lookup response body

Modify `api/internal/platform/instagram.go`.

Current issue:

- `getIGUserID` does not check `resp.StatusCode`.
- It decodes the response as `{ "id": "..." }`.
- If `id` is empty, it returns `failed to get Instagram user ID`.

Required behavior:

- Read the response body.
- If `resp.StatusCode != http.StatusOK`, return an error containing:
  - adapter operation: `instagram get user id failed`
  - HTTP status
  - sanitized response body only
- If status is 200 but decode fails, return a decode-specific error.
- If status is 200 but `id` is empty, return an error that includes the body or a clear "missing id" diagnostic.
- Do not include the request URL in the error string because `getIGUserID` currently sends the Instagram access token in the query string.
- Add or reuse defense-in-depth redaction for persisted delivery errors so accidental `access_token=...`, `Authorization: Bearer ...`, and similar token material is removed before storage.

Example target error:

```text
instagram get user id failed (400): {"error":{"message":"Error validating access token: ...","type":"OAuthException","code":190,"error_subcode":0}}
```

### 7.2 Reuse existing taxonomy classification

Modify or verify `api/internal/postfailures/taxonomy.go`.

Existing classifier behavior already treats Meta OAuth code `190` as reconnect-required when the raw error string contains:

- `"code":190`, or
- `"code": 190`, or
- `OAuthException` with token validation/expiry language.

Requirement:

- Add an explicit test for the new Instagram user-ID lookup error shape.
- Confirm classification:

```text
ErrorCode: account_reconnect_required
ErrorSource: platform
ErrorTemporality: permanent
PlatformErrorCode: 190
ProviderError.Provider: meta
ProviderError.Code: 190
ProviderError.HTTPStatus: 400
IsRetriable: false
NextAction: reconnect_account
```

### 7.3 Confirm account status transition

No new account-status API should be added.

Existing behavior in `api/internal/handler/social_posts.go`:

- `recordPostFailure` persists structured failure details.
- If `ErrorCode` is `account_reconnect_required` or `auth_token_invalid`, it calls `MarkSocialAccountReconnectRequired`.

Existing DB behavior:

```sql
UPDATE social_accounts
SET status = 'reconnect_required',
    metadata = COALESCE(metadata, '{}'::jsonb) || jsonb_build_object('reconnect_required_at', NOW()::TEXT)
WHERE id = $1
  AND status = 'active'
```

Requirement:

- Ensure the new Instagram OAuth code `190` path reaches this existing code path.
- Do not introduce a second reconnect-required mechanism.

### 7.4 Gate new publishes on reconnect-required status

Modify the publish validation account builders so `status='reconnect_required'` is treated the same as disconnected for newly validated publish requests.

Relevant paths:

- `api/internal/handler/social_posts_validate.go`
  - `loadValidateAccounts`
  - currently sets `Disconnected: a.DisconnectedAt.Valid`
- `api/internal/handler/social_post_queue.go`
  - `EnqueueScheduledPost`
  - currently sets `Disconnected: ok && acc.DisconnectedAt.Valid`

Required behavior:

```text
Disconnected = disconnected_at is set OR status == reconnect_required
```

This should be implemented as a small shared helper if the surrounding code supports it cleanly, or duplicated directly in the two account builders if that is the narrower change.

The gate must apply to all platforms that use the existing `reconnect_required` status, not only Instagram. That means it should also reduce future new-post churn for TikTok accounts that were previously marked reconnect-required by existing `auth_token_invalid` handling.

Known limitation:

- In the immediate publish path, `accountMap` is loaded once for the request before dispatch work begins. If a single large request targets the same failing account multiple times and the account is marked reconnect-required during that same request, other already-created results in that same snapshot may still run. The gate prevents subsequent new validations after the status is marked; it does not mutate the already-loaded in-memory account map.

### 7.5 Scope for non-publish Instagram callers

`getIGUserID` is shared by Instagram operations beyond post publishing, such as media, DM, and conversation helpers. The adapter diagnostics improvement benefits those callers too because the returned error will include status/body evidence. However, only publish failures that route through `recordPostFailure` will automatically mark a social account as `reconnect_required`.

Non-publish callers should keep their existing behavior unless a later PRD explicitly extends reconnect marking to those surfaces.

---

## 8. Acceptance Criteria

1. A Meta OAuth code `190` response from Instagram `/me?fields=id` no longer persists as generic `platform_error`.
2. New matching failures persist with `error_code=account_reconnect_required`.
3. New matching failures expose `platform_error_code=190` when available.
4. New matching failures expose `next_action=reconnect_account`.
5. The affected active `social_accounts` row is marked `status='reconnect_required'`.
6. New publish validation treats `status='reconnect_required'` as unavailable/disconnected.
7. A follow-up new publish attempt to the same account is rejected before adapter dispatch unless the account is reconnected.
8. Existing TikTok invalid-token classification continues to pass tests and now benefits from the same reconnect-required validation gate for future new posts.
9. Existing Instagram successful publish behavior is unchanged.
10. Existing Instagram transient container timeout and media-error classification is unchanged.
11. No sensitive token material appears in persisted `message`, `raw_error`, `debug_curl`, logs, or API responses.

---

## 9. Validation Plan

### 9.1 Unit tests

From `api/`, run:

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/platform ./internal/postfailures ./internal/handler
```

Required test coverage:

1. `api/internal/platform/instagram_test.go`
   - Simulate `/me?fields=id` returning HTTP 400 with Meta OAuth code `190`.
   - Assert the adapter error includes:
     - `instagram get user id failed (400)`
     - `"code":190`
     - no raw access token.

2. `api/internal/postfailures/taxonomy_test.go`
   - Classify the new Instagram adapter error string.
   - Assert:
     - `account_reconnect_required`
     - `platform_error_code=190`
     - provider `meta`
     - HTTP status `400`
     - non-retriable.

3. `api/internal/handler` tests
   - Confirm a `CreatePostFailureParams` with `account_reconnect_required` and a valid `social_account_id` marks an active social account as `reconnect_required`.
   - Preserve existing tests for TikTok `auth_token_invalid`.

4. Publish validation tests
   - Confirm `loadValidateAccounts` treats an account with `status='reconnect_required'` and empty `disconnected_at` as disconnected/unavailable.
   - Confirm scheduled enqueue validation uses the same reconnect-required gate.
   - Confirm ordinary active accounts with empty `disconnected_at` still validate normally.

5. Delivery error redaction tests
   - Confirm persisted delivery errors redact accidental `access_token=...` query values.
   - Confirm persisted delivery errors redact accidental `Authorization: Bearer ...` header text.

### 9.2 Local integration sanity check

Use a local or mocked publish flow where the Instagram adapter returns:

```json
{
  "error": {
    "message": "Error validating access token: You cannot access the app till you log in to www.instagram.com and follow the instructions given.",
    "type": "OAuthException",
    "code": 190,
    "error_subcode": 0
  }
}
```

Expected result:

- The social post result fails.
- The failure fields are structured as reconnect-required.
- The account status changes to `reconnect_required`.
- No retry job is scheduled for the permanent reconnect-required failure.
- A subsequent new publish validation for the same account rejects the target before adapter dispatch.
- A second account in the same already-loaded batch snapshot may still run if it was accepted before the account status changed; this is an acknowledged limitation, not a failed validation.

### 9.3 Development deployment verification

After implementation is merged to local `dev`, validation passes, and `dev` is pushed to `origin/dev`, wait for the development deployment to finish. Then verify in the real development environment:

- Development backend API: `https://dev-api.unipost.dev`
- Development app frontend: `https://dev-app.unipost.dev`

Verification steps:

1. Trigger or simulate an Instagram publish failure that returns Meta OAuth code `190`.
2. Open Admin Errors in the development app.
3. Confirm the failure row shows:
   - `account_reconnect_required`
   - `dispatch`
   - `190` where platform/provider code is surfaced
   - reconnect-style next action.
4. Confirm the corresponding social account in dev DB has:
   - `status='reconnect_required'`
   - `metadata.reconnect_required_at` set.
5. Confirm a follow-up new publish attempt to that account is rejected before Instagram adapter dispatch.
6. Reconnect the account or use a healthy active account and confirm normal publish validation still works.

---

## 10. Rollout Notes

- This should be shipped through the normal `dev` deployment flow first.
- No production data migration is needed.
- Historical rows will continue to show `platform_error` unless a separate backfill is explicitly approved.
- The change is safe to roll forward because it preserves more error detail, reuses existing reconnect-required behavior, and applies the reconnect-required status consistently during new publish validation.
- Rollback is a code rollback; there is no feature flag for this narrow backend behavior.

---

## 11. Open Questions

1. Should Admin Errors add a friendlier display label for `account_reconnect_required`, such as "Reconnect Instagram account", or is the existing badge sufficient for v1?
2. Should Admin Users eventually distinguish final failed posts from intermediate failure events to reduce confusion between `99` failed posts and `404` failure events?
3. Should we create a follow-up cleanup task to improve historical analytics by backfilling recent Instagram OAuth code `190` rows, or leave history immutable?
