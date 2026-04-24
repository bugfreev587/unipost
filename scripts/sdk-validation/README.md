# SDK Validation Tests

Live API tests for the current UniPost SDK surfaces.

All three validation scripts now use the local SDK packages checked into this repo so unreleased SDK changes can be verified against the live UniPost API before publishing:
- JavaScript: `/Users/xiaoboyu/unipost/sdk/javascript`
- Go: `/Users/xiaoboyu/unipost/sdk/go`
- Python: `/Users/xiaoboyu/unipost/sdk/python`

## JavaScript / TypeScript

```bash
cd js
npm install
UNIPOST_API_KEY=up_live_xxx node unipost-sdk-test.mjs
UNIPOST_API_KEY=up_live_xxx TEST_ACCOUNT_ID=<id> node unipost-sdk-test.mjs
```

What it covers now:
- Public catalogs: platforms capabilities and plans
- Workspace get/update
- Profiles list/create/get/update/delete
- Accounts list/get/health/capabilities
- TikTok creator info and Facebook page insights when available
- Connect account negative-path validation
- Media upload/get/delete cleanup
- Connect session create/get
- Users list/get when managed users exist
- Webhook signature verification plus webhook CRUD/rotate
- Platform credentials create/list/delete when the workspace plan allows it
- Posts validate/list/get/queue/analytics
- Draft create/update/preview/archive/restore
- Scheduled create/update/cancel
- Bulk create
- Delivery job list/summary and conditional retry/cancel
- Analytics summary/trend/by-platform/rollup
- Usage get
- OAuth connect known-path validation

## Python

```bash
cd python
pip install -r requirements.txt
UNIPOST_API_KEY=up_live_xxx python unipost_sdk_test.py
UNIPOST_API_KEY=up_live_xxx TEST_ACCOUNT_ID=<id> python unipost_sdk_test.py
```

Coverage matches the JavaScript suite above, using the local Python SDK package.

## Go

```bash
cd go
go mod tidy
UNIPOST_API_KEY=up_live_xxx go run main.go
UNIPOST_API_KEY=up_live_xxx TEST_ACCOUNT_ID=<id> go run main.go
```

Coverage matches the JavaScript suite above, plus Go-specific page-meta checks such as `Accounts.ListPage()` and `Webhooks.ListPage()`.

## Flags

| Env var | Purpose |
|---------|---------|
| `UNIPOST_API_KEY` | Required. API key from app.unipost.dev |
| `TEST_ACCOUNT_ID` | Optional. Enables post create/schedule/cancel tests |
| `TEST_PUBLISH_NOW=true` | Optional. Actually publishes a post (irreversible) |

## Coverage notes

- The live suites cover the entire public SDK surface for normal routes.
- A small subset remains conditional rather than guaranteed on every run:
  - `users.get()` only runs when the workspace has managed users.
  - `posts.publish()` only runs when `TEST_PUBLISH_NOW=true`.
  - `posts.retryResult()` only runs when a safe failed result already exists.
  - `deliveryJobs.retry()/cancel()` only runs when a retryable job already exists.
- Direct destructive calls such as deleting an arbitrary pre-existing account are intentionally not forced in validation. Cleanup paths still verify delete behavior for resources the scripts create themselves, such as posts, media, webhooks, and temporary profiles.
