# SDK Source Validation Tests

Live API tests for unreleased UniPost SDK source changes.

All three validation scripts use the local SDK packages from the public unipost-dev workspace, so unreleased SDK changes can be verified against the live UniPost API before publishing:
- JavaScript: `/Users/xiaoboyu/unipost-dev/sdk-js` (`dist/index.mjs`)
- Go: `/Users/xiaoboyu/unipost-dev/sdk-go` (wired via a `replace` directive in `go/go.mod`)
- Python: `/Users/xiaoboyu/unipost-dev/sdk-python` (added to `sys.path`)
- Java: `/Users/xiaoboyu/unipost-dev/sdk-java` (published to `mavenLocal()` before the validation app runs)

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
- Platform credentials create/list/delete only when `TEST_PLATFORM_CREDENTIALS_PLATFORM` explicitly names a supported platform that is safe for the test workspace to overwrite
- API keys list, plus a create/revoke round-trip (mint a test key, verify the prefix, revoke it)
- Posts validate/list/get/queue/analytics
- Draft create/update/preview/archive/restore
- Scheduled create/update/cancel
- Bulk create
- Delivery job list/summary and conditional retry/cancel
- Analytics summary/trend/by-platform/rollup
- Developer logs list/get and conditional SSE replay stream
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

## Java

```bash
cd /Users/xiaoboyu/unipost-dev/sdk-java
./gradlew test
./gradlew publishToMavenLocal

cd /Users/xiaoboyu/unipost/scripts/sdk-validation/java
UNIPOST_API_KEY=up_live_xxx ./gradlew run -PunipostJavaSdkVersion=0.4.0 -PuseLocalSdk=true
UNIPOST_API_KEY=up_live_xxx TEST_ACCOUNT_ID=<id> ./gradlew run -PunipostJavaSdkVersion=0.4.0 -PuseLocalSdk=true
```

Coverage broadly matches the JavaScript / Python / Go live suites, including public catalogs, workspace and profile CRUD, account health/capabilities, connect sessions, media, managed users, webhooks, platform credentials, API keys, posts, delivery jobs, analytics, developer logs, usage, and OAuth connect URL generation.

## Flags

| Env var | Purpose |
|---------|---------|
| `UNIPOST_API_KEY` | Required. API key from app.unipost.dev |
| `TEST_ACCOUNT_ID` | Optional. Enables post create/schedule/cancel tests |
| `TEST_PUBLISH_NOW=true` | Optional. Actually publishes a post (irreversible) |
| `TEST_PLATFORM_CREDENTIALS_PLATFORM` | Optional. Enables the destructive Platform Credentials create/list/delete round-trip for a supported platform (`twitter`, `linkedin`, `bluesky`, `youtube`, `tiktok`, `instagram`, `threads`, `facebook`, or `pinterest`). Leave unset for production regression keys unless that workspace is dedicated to credential overwrite testing. |

## Coverage notes

- The live suites cover the entire public SDK surface for normal routes.
- A small subset remains conditional rather than guaranteed on every run:
  - `users.get()` only runs when the workspace has managed users.
  - `posts.publish()` only runs when `TEST_PUBLISH_NOW=true`.
  - `posts.retryResult()` only runs when a safe failed result already exists.
  - `deliveryJobs.retry()/cancel()` only runs when a retryable job already exists.
  - `logs.get()` and `logs.stream()` only run when `logs.list()` returns at least one retained log.
  - `platform credentials create/list/delete` only runs when `TEST_PLATFORM_CREDENTIALS_PLATFORM` is set to a supported platform that is safe to overwrite.
- Direct destructive calls such as deleting an arbitrary pre-existing account are intentionally not forced in validation. Cleanup paths still verify delete behavior for resources the scripts create themselves, such as posts, media, webhooks, and temporary profiles.

## Running the source-validation suites

From repo root:

```bash
scripts/sdk-source-validation/run-suite.sh sdk-js
scripts/sdk-source-validation/run-suite.sh sdk-python
scripts/sdk-source-validation/run-suite.sh sdk-go
scripts/sdk-source-validation/run-suite.sh sdk-java
```

If your SDK checkout root is not `/Users/xiaoboyu/unipost-dev`, override it with:

```bash
UNIPOST_DEV_ROOT=/path/to/unipost-dev scripts/sdk-source-validation/run-suite.sh sdk-js
```

## Published-package regression

Published-package regression is intentionally separate:

- source validation uses unreleased SDK source from `/Users/xiaoboyu/unipost-dev`
- regression monitoring uses the released packages from npm, PyPI, and Go module resolution

Run the published-package suites with:

```bash
scripts/sdk-published-regression/run-suite.sh sdk-js
scripts/sdk-published-regression/run-suite.sh sdk-python
scripts/sdk-published-regression/run-suite.sh sdk-go
scripts/sdk-published-regression/run-suite.sh sdk-java
```
