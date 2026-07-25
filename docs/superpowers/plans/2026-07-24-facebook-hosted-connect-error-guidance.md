# Facebook Hosted Connect Error Guidance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace misleading Facebook Hosted Connect failures with safe, actionable Page guidance and make every attributable Hosted Connect attempt synchronously write one searchable Workspace result-log attempt.

**Architecture:** The Facebook connector returns a bounded typed failure that never contains provider bodies or credential-bearing URLs. The OAuth callback and Bluesky form share a response-writer-backed outcome recorder: after Session and Workspace attribution, the first HTTP response write synchronously attempts one integration-log insert using the final safe outcome. API and Dashboard presentation map the same fixed Facebook reason codes, while Workspace Logs add exact metadata-ID search without a schema migration.

**Tech Stack:** Go 1.25, Chi, pgx/sqlc, PostgreSQL JSONB, Next.js 16, React 19, TypeScript, Node 22 test runner, Playwright.

---

## File map

### New files

- `api/internal/connect/facebook_errors.go` — bounded Facebook failure type, public reason constants, safe Meta error parsing.
- `api/internal/handler/connect_outcome.go` — shared synchronous Hosted Connect result recorder and response-writer wrapper.
- `api/internal/handler/connect_outcome_test.go` — exactly-once attempt, success/failure/cancelled, and fail-open tests.
- `api/internal/db/integration_logs_search_contract_test.go` — generated SQL contract for exact Connect Session/external user metadata lookup.
- `dashboard/src/lib/log-search.ts` — shared history/live-tail log-search predicate.
- `dashboard/tests/hosted-connect-errors.test.mjs` — executable Facebook presentation tests against the TypeScript helper.
- `dashboard/tests/hosted-connect-logs.test.mjs` — executable exact metadata-ID search tests.

### Modified files

- `api/internal/connect/facebook.go` — use bounded typed failures and distinguish Page states.
- `api/internal/connect/facebook_test.go` — Page classification and four provider-body leakage regressions.
- `api/internal/integrationlogs/logger.go` — store interface and bounded synchronous `WriteSync` path.
- `api/internal/integrationlogs/logger_test.go` — synchronous insert, notification, and failure propagation tests.
- `api/internal/integrationlogs/normalize.go` — add the cancellation action constant.
- `api/internal/handler/connect_callback.go` — safe Facebook public reasons and shared outcome recording for every attributable OAuth path.
- `api/internal/handler/connect_sessions_test.go` — OAuth result-log matrix, Facebook redirects/HTML, pending state, and no-leak assertions.
- `api/internal/handler/connect_bluesky.go` — attribute before provider validation and use the shared outcome recorder.
- `api/internal/handler/connect_bluesky_test.go` — Bluesky success/failure result-log and password-exclusion coverage.
- `api/cmd/api/main.go` — inject the integration logger into the Bluesky handler.
- `api/internal/db/queries/integration_logs.sql` — exact `connect_session_id` and `external_user_id` metadata search.
- `api/internal/db/integration_logs.sql.go` — regenerated sqlc output.
- `api/internal/handler/logs_test.go` — Workspace-scoped query propagation contract.
- `dashboard/src/lib/connect-errors.ts` — fixed Facebook title/body presentation map.
- `dashboard/src/app/connect/[platform]/page.tsx` — render the mapped title and body.
- `dashboard/src/app/(dashboard)/projects/[id]/logs/page.tsx` — reuse exact metadata-aware live search.
- `dashboard/package.json` — focused Hosted Connect test script.
- `dashboard/src/app/docs/api/logs/page.tsx` — document Hosted Connect outcome actions and search.
- `dashboard/src/app/docs/api/logs/list/page.tsx` — document Facebook error-code and exact ID query examples.

## Task 1: Add typed, bounded Facebook failures

**Files:**

- Create: `api/internal/connect/facebook_errors.go`
- Modify: `api/internal/connect/facebook.go`
- Test: `api/internal/connect/facebook_test.go`

- [ ] **Step 1: Write failing Page-state tests**

Add tests that drive the real `FacebookConnector.ExchangeCode` through an `httptest.Server` and inspect the wished-for typed API:

```go
func TestFacebookConnectorExchangeClassifiesPageAvailability(t *testing.T) {
	tests := []struct {
		name             string
		pages            []map[string]any
		wantCode         FacebookConnectFailureCode
		wantPageCount    int
		wantPublishable  int
	}{
		{
			name:            "no accessible pages",
			pages:           []map[string]any{},
			wantCode:        FacebookPageNotAvailable,
			wantPageCount:   0,
			wantPublishable: 0,
		},
		{
			name: "pages lack publishing tasks",
			pages: []map[string]any{{
				"id": "page_1", "name": "Read only", "access_token": "page-token",
				"tasks": []string{"ADVERTISE"},
			}},
			wantCode:        FacebookPagePermissionRequired,
			wantPageCount:   1,
			wantPublishable: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector, closeServer := newFacebookExchangeTestConnector(t, tt.pages)
			defer closeServer()

			_, err := connector.ExchangeCode(context.Background(), SessionView{}, "code")
			var failure *FacebookConnectFailure
			if !errors.As(err, &failure) {
				t.Fatalf("error = %v, want *FacebookConnectFailure", err)
			}
			if failure.Code != tt.wantCode || failure.PageCount != tt.wantPageCount || failure.PublishablePageCount != tt.wantPublishable {
				t.Fatalf("failure = %+v", failure)
			}
		})
	}
}
```

Also split the accepted-task coverage into a table for `CREATE_CONTENT`, `MANAGE`, and `MODERATE`, and assert that a publishable Page without `access_token` returns `FacebookAuthorizationFailed`.

- [ ] **Step 2: Run the Page-state tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/connect -run 'TestFacebookConnectorExchange(ClassifiesPageAvailability|AcceptsPublishingTasks|MissingPageToken)' -count=1
```

Expected: compilation fails because `FacebookConnectFailure`, its codes, and the new helper do not exist.

- [ ] **Step 3: Write failing provider-body and URL leakage tests**

Cover short token, long token, `/me/accounts`, and `/me` non-200 responses with a unique marker such as `raw-provider-secret`. For each operation, assert:

```go
var failure *FacebookConnectFailure
if !errors.As(err, &failure) {
	t.Fatalf("error = %v, want typed Facebook failure", err)
}
for _, forbidden := range []string{"raw-provider-secret", "oauth-code-secret", "user-token-secret", "client-secret"} {
	if strings.Contains(err.Error(), forbidden) {
		t.Fatalf("error leaked %q: %v", forbidden, err)
	}
}
if failure.RemoteStatusCode != http.StatusBadRequest {
	t.Fatalf("remote status = %d", failure.RemoteStatusCode)
}
```

Add a transport-error test whose returned `url.Error` contains a credential-bearing URL, and assert the typed error does not wrap or stringify that URL.

- [ ] **Step 4: Run leakage tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/connect -run 'TestFacebookConnector.*(ProviderBody|TransportError)' -count=1
```

Expected: FAIL because current errors contain `string(body)` or the raw transport error.

- [ ] **Step 5: Implement the bounded failure type**

Create `facebook_errors.go` with the exact public contract:

```go
package connect

import (
	"encoding/json"
	"fmt"
)

type FacebookConnectFailureCode string

const (
	FacebookPageNotAvailable       FacebookConnectFailureCode = "facebook_page_not_available"
	FacebookPagePermissionRequired FacebookConnectFailureCode = "facebook_page_permission_required"
	FacebookAuthorizationFailed    FacebookConnectFailureCode = "facebook_authorization_failed"
)

type FacebookConnectFailure struct {
	Code                 FacebookConnectFailureCode
	Stage                string
	RemoteStatusCode     int
	MetaCode             int
	MetaSubcode          int
	PageCount            int
	PublishablePageCount int
}

func (e *FacebookConnectFailure) Error() string {
	if e == nil {
		return "facebook connect failed"
	}
	return fmt.Sprintf("facebook connect failed at %s", e.Stage)
}

func newFacebookProviderFailure(stage string, status int, body []byte) *FacebookConnectFailure {
	var envelope struct {
		Error struct {
			Code    int `json:"code"`
			Subcode int `json:"error_subcode"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &envelope)
	return &FacebookConnectFailure{
		Code:             FacebookAuthorizationFailed,
		Stage:            stage,
		RemoteStatusCode: status,
		MetaCode:         envelope.Error.Code,
		MetaSubcode:      envelope.Error.Subcode,
	}
}
```

Do not add an `Unwrap` method or store a raw cause: request URLs contain secrets in query parameters.

- [ ] **Step 6: Replace unsafe Facebook errors and distinguish Page states**

In all Facebook request, transport, non-200, empty-token, and decode failures, return `FacebookConnectFailure` with a fixed stage. Never format `body`, `req.URL`, or the raw `http.Client.Do` error.

Replace the current selection block with:

```go
if len(pages) == 0 {
	return nil, &FacebookConnectFailure{
		Code: FacebookPageNotAvailable, Stage: "page_discovery",
		PageCount: 0, PublishablePageCount: 0,
	}
}

publishableCount := 0
for _, candidate := range pages {
	if facebookPageHasPublishTask(candidate.Tasks) {
		publishableCount++
	}
}
page, ok := firstPublishableFacebookPage(pages)
if !ok {
	return nil, &FacebookConnectFailure{
		Code: FacebookPagePermissionRequired, Stage: "page_permission",
		PageCount: len(pages), PublishablePageCount: publishableCount,
	}
}
if page.AccessToken == "" {
	return nil, &FacebookConnectFailure{
		Code: FacebookAuthorizationFailed, Stage: "page_access_token",
		PageCount: len(pages), PublishablePageCount: publishableCount,
	}
}
```

- [ ] **Step 7: Run connector tests and verify GREEN**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/connect -count=1
```

Expected: PASS with no leaked marker in failure output.

- [ ] **Step 8: Commit Task 1**

```bash
git add api/internal/connect/facebook.go api/internal/connect/facebook_errors.go api/internal/connect/facebook_test.go
git commit -m "fix: classify Facebook Page connection failures"
```

## Task 2: Add safe Facebook error presentation on both response paths

**Files:**

- Modify: `api/internal/handler/connect_callback.go`
- Test: `api/internal/handler/connect_sessions_test.go`
- Modify: `dashboard/src/lib/connect-errors.ts`
- Modify: `dashboard/src/app/connect/[platform]/page.tsx`
- Create: `dashboard/tests/hosted-connect-errors.test.mjs`
- Modify: `dashboard/package.json`

- [ ] **Step 1: Write failing API presentation tests**

Add table tests for the three public reasons. The direct HTML assertions are:

```go
tests := []struct {
	reason, title, body string
}{
	{
		"facebook_page_not_available",
		"Facebook Page unavailable",
		"We couldn’t find a Facebook Page this account can manage or has allowed UniPost to access.",
	},
	{
		"facebook_page_permission_required",
		"Facebook Page permission required",
		"Your Facebook account can access a Page, but it doesn’t have permission to publish content.",
	},
	{
		"facebook_authorization_failed",
		"Connection failed",
		"Facebook authorization couldn’t be completed.",
	},
}
```

Call the reason renderer through `redirectWithStatus` with an empty `return_url`. Assert title/body are present and the reason token plus `token_exchange_failed` are absent from the HTML. Add a redirect test that asserts `reason=<stable-code>` and excludes raw provider text.

- [ ] **Step 2: Run API presentation tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestConnectRedirectFacebookPresentation' -count=1
```

Expected: FAIL because the template title is fixed to `Connect` and the reason is rendered raw.

- [ ] **Step 3: Write failing executable Dashboard tests**

Create `hosted-connect-errors.test.mjs` and import the TypeScript module directly under Node 22:

```js
import test from "node:test";
import assert from "node:assert/strict";
import { getConnectErrorPresentation } from "../src/lib/connect-errors.ts";

test("Facebook Hosted Connect reasons have actionable presentations", () => {
  assert.deepEqual(
    getConnectErrorPresentation("facebook_page_not_available", "facebook"),
    {
      title: "Facebook Page unavailable",
      body: "We couldn’t find a Facebook Page this account can manage or has allowed UniPost to access. Make sure this Facebook account manages a Page and that UniPost is allowed to access it, or ask a Page admin to grant you access. Then open the original connection link and try again.",
    },
  );
  assert.equal(
    getConnectErrorPresentation("unexpected-provider-body", "facebook").body,
    "Facebook authorization couldn’t be completed. Please try again later or contact the developer who sent you the link.",
  );
});

test("non-Facebook unknown reasons preserve existing behavior", () => {
  assert.equal(getConnectErrorPresentation("legacy_reason", "linkedin").body, "legacy_reason");
});
```

- [ ] **Step 4: Run Dashboard test and verify RED**

Run:

```bash
cd dashboard
node --test tests/hosted-connect-errors.test.mjs
```

Expected: FAIL because `getConnectErrorPresentation` is not exported.

- [ ] **Step 5: Implement API title/body presentation**

Change the error template to render `{{.Title}}` and add a fixed presentation lookup:

```go
type connectErrorPresentation struct {
	Title string
	Body  string
}

func presentationForConnectReason(reason string) (connectErrorPresentation, bool) {
	switch reason {
	case string(connect.FacebookPageNotAvailable):
		return connectErrorPresentation{Title: "Facebook Page unavailable", Body: facebookPageNotAvailableMessage}, true
	case string(connect.FacebookPagePermissionRequired):
		return connectErrorPresentation{Title: "Facebook Page permission required", Body: facebookPagePermissionRequiredMessage}, true
	case string(connect.FacebookAuthorizationFailed):
		return connectErrorPresentation{Title: "Connection failed", Body: facebookAuthorizationFailedMessage}, true
	default:
		return connectErrorPresentation{}, false
	}
}
```

Keep `renderConnectError` as the legacy wrapper with title `Connect`. In `redirectWithStatus`, use the mapped presentation only for known Facebook reasons; keep other platform behavior unchanged.

- [ ] **Step 6: Implement Dashboard presentation**

Export:

```ts
export type ConnectErrorPresentation = { title: string; body: string };

export function getConnectErrorPresentation(
  raw?: string | null,
  platform?: string,
): ConnectErrorPresentation {
  const reason = (raw || "").trim();
  if (reason === "facebook_page_not_available") {
    return { title: "Facebook Page unavailable", body: FACEBOOK_PAGE_NOT_AVAILABLE_BODY };
  }
  if (reason === "facebook_page_permission_required") {
    return { title: "Facebook Page permission required", body: FACEBOOK_PAGE_PERMISSION_REQUIRED_BODY };
  }
  if (reason === "facebook_authorization_failed" || platform === "facebook") {
    return { title: "Connection failed", body: FACEBOOK_AUTHORIZATION_FAILED_BODY };
  }
  if (
    reason.includes("Free plan workspaces cannot share the same connected social account")
    || reason.includes("ACCOUNT_NOT_AVAILABLE_ON_FREE_PLAN")
  ) {
    return { title: "Connection failed", body: FREE_PLAN_ACCOUNT_UNAVAILABLE_BODY };
  }
  if (reason.includes("ACCOUNT_ALREADY_CONNECTED")) {
    return { title: "Connection failed", body: "This social account is already connected in this workspace." };
  }
  return {
    title: "Connection failed",
    body: reason || "Failed to connect. Please try again.",
  };
}

export function humanizeConnectError(raw?: string | null): string {
  return getConnectErrorPresentation(raw).body;
}
```

Update the Hosted Connect page to compute once:

```tsx
const presentation = getConnectErrorPresentation(reason, platform);
return <ErrorPage title={presentation.title} body={presentation.body} />;
```

Add `test:hosted-connect` to `package.json`:

```json
"test:hosted-connect": "node --test tests/hosted-connect-errors.test.mjs tests/hosted-connect-logs.test.mjs"
```

- [ ] **Step 7: Run presentation tests and verify GREEN**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestConnectRedirectFacebookPresentation' -count=1
cd ../dashboard
node --test tests/hosted-connect-errors.test.mjs
```

Expected: PASS.

- [ ] **Step 8: Commit Task 2**

```bash
git add api/internal/handler/connect_callback.go api/internal/handler/connect_sessions_test.go dashboard/src/lib/connect-errors.ts dashboard/src/app/connect/'[platform]'/page.tsx dashboard/tests/hosted-connect-errors.test.mjs dashboard/package.json
git commit -m "fix: show actionable Facebook Connect errors"
```

## Task 3: Add synchronous integration-log writes and the shared outcome recorder

**Files:**

- Modify: `api/internal/integrationlogs/logger.go`
- Create: `api/internal/integrationlogs/logger_test.go`
- Modify: `api/internal/integrationlogs/normalize.go`
- Create: `api/internal/handler/connect_outcome.go`
- Create: `api/internal/handler/connect_outcome_test.go`

- [ ] **Step 1: Write failing `WriteSync` tests**

Define a fake store implementing `InsertIntegrationLog` and test normalization, after-write notification, and error propagation:

```go
type fakeIntegrationLogStore struct {
	params db.InsertIntegrationLogParams
	row    db.IntegrationLog
	err    error
}

func (f *fakeIntegrationLogStore) InsertIntegrationLog(_ context.Context, params db.InsertIntegrationLogParams) (db.IntegrationLog, error) {
	f.params = params
	return f.row, f.err
}

func TestLoggerWriteSyncPersistsBeforeReturning(t *testing.T) {
	store := &fakeIntegrationLogStore{row: db.IntegrationLog{ID: 42}}
	notified := false
	logger := NewLogger(store, func(context.Context, db.IntegrationLog) { notified = true })
	err := logger.WriteSync(context.Background(), Event{WorkspaceID: "ws_1", Action: "account.connect.callback_succeeded"})
	if err != nil || store.params.WorkspaceID != "ws_1" || !notified {
		t.Fatalf("err=%v params=%+v notified=%v", err, store.params, notified)
	}
}
```

The failure test returns `errors.New("insert failed")` and asserts `WriteSync` returns that error without calling `afterWrite`.

- [ ] **Step 2: Run logger tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/integrationlogs -run 'TestLoggerWriteSync' -count=1
```

Expected: compilation fails because the constructor requires `*db.Queries` and `WriteSync` does not exist.

- [ ] **Step 3: Implement the store interface and `WriteSync`**

Use:

```go
type integrationLogStore interface {
	InsertIntegrationLog(context.Context, db.InsertIntegrationLogParams) (db.IntegrationLog, error)
}

func (l *Logger) WriteSync(ctx context.Context, e Event) error {
	if l == nil || l.queries == nil || e.WorkspaceID == "" || e.Action == "" {
		return errors.New("integration logger unavailable")
	}
	opCtx, cancel := context.WithTimeout(ctx, defaultWriteTimeout)
	defer cancel()
	row, err := l.queries.InsertIntegrationLog(opCtx, Normalize(e))
	if err != nil {
		l.failures.Add(1)
		return err
	}
	if l.afterWrite != nil {
		l.afterWrite(opCtx, row)
	}
	return nil
}
```

Refactor the async worker to call a shared insert helper without changing queue behavior. Add `ActionAccountConnectCallbackCancelled = "account.connect.callback_cancelled"`.

- [ ] **Step 4: Run logger tests and verify GREEN**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/integrationlogs -count=1
```

Expected: PASS.

- [ ] **Step 5: Write failing shared recorder tests**

Create a fake synchronous writer and exercise the real response wrapper:

```go
type recordingOutcomeWriter struct {
	events []integrationlogs.Event
	err    error
}

func (w *recordingOutcomeWriter) WriteSync(_ context.Context, event integrationlogs.Event) error {
	w.events = append(w.events, event)
	return w.err
}

func TestHostedConnectOutcomeResponseWriterWritesOnceBeforeResponse(t *testing.T) {
	writer := &recordingOutcomeWriter{}
	outcome := newHostedConnectOutcome(context.Background(), writer, hostedConnectAttempt{
		WorkspaceID: "ws_1", ProfileID: "pr_1", Platform: "facebook",
		SessionID: "cs_1", ExternalUserID: "managed_1", RequestID: "req_1",
	})
	outcome.Fail("facebook_page_not_available", "Hosted Connect failed: no manageable Facebook Page was found.", map[string]any{"page_count": 0})
	base := httptest.NewRecorder()
	w := wrapHostedConnectResponse(base, outcome)
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte("body"))
	if len(writer.events) != 1 {
		t.Fatalf("events = %d", len(writer.events))
	}
}
```

Add success, cancellation, default bounded failure, metadata, and writer-error tests. Assert the writer error does not alter the HTTP status/body and the event contains no response payload.

- [ ] **Step 6: Run recorder tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestHostedConnectOutcome' -count=1
```

Expected: compilation fails because the recorder does not exist.

- [ ] **Step 7: Implement the shared recorder**

Implement a guarded state object with `Success`, `Fail`, and `Cancel`. Its base metadata is exactly:

```go
map[string]any{
	"connect_session_id": attempt.SessionID,
	"external_user_id":   attempt.ExternalUserID,
	"callback_status":    "error",
	"connection_type":    "managed",
}
```

`persist` calls `WriteSync` once using `sync.Once`. On error, increment an atomic failure counter and emit only safe identifiers:

```go
slog.Error("hosted_connect_outcome_log_write_failed",
	"workspace_id", attempt.WorkspaceID,
	"platform", attempt.Platform,
	"action", event.Action,
	"request_id", attempt.RequestID,
	"total_failures", hostedConnectOutcomeWriteFailures.Add(1),
)
```

The response writer calls `persist` before delegating from both `WriteHeader` and `Write`.

- [ ] **Step 8: Run recorder and integration-log tests and verify GREEN**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/integrationlogs ./internal/handler -run 'Test(LoggerWriteSync|HostedConnectOutcome)' -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit Task 3**

```bash
git add api/internal/integrationlogs/logger.go api/internal/integrationlogs/logger_test.go api/internal/integrationlogs/normalize.go api/internal/handler/connect_outcome.go api/internal/handler/connect_outcome_test.go
git commit -m "feat: persist Hosted Connect outcomes synchronously"
```

## Task 4: Route every attributable OAuth callback through the outcome recorder

**Files:**

- Modify: `api/internal/handler/connect_callback.go`
- Test: `api/internal/handler/connect_sessions_test.go`

- [ ] **Step 1: Extend the fake connector and writer**

Extend `fakeOAuthConnector` without changing existing defaults:

```go
type fakeOAuthConnector struct {
	platform    string
	profile     *connect.Profile
	exchangeErr error
	profileErr  error
}

func (f fakeOAuthConnector) ExchangeCode(context.Context, connect.SessionView, string) (*connect.TokenSet, error) {
	if f.exchangeErr != nil {
		return nil, f.exchangeErr
	}
	return &connect.TokenSet{
		AccessToken: "access-token", RefreshToken: "refresh-token",
		ExpiresAt: time.Now().Add(time.Hour), Scopes: []string{"user.info.basic"},
	}, nil
}

func (f fakeOAuthConnector) FetchProfile(context.Context, string) (*connect.Profile, error) {
	if f.profileErr != nil {
		return nil, f.profileErr
	}
	if f.profile != nil {
		return f.profile, nil
	}
	profile := &connect.Profile{
		ExternalAccountID: "platform-account-new", Username: "Robyn", DisplayName: "Robyn",
	}
	if f.platform == "instagram" {
		profile.WebhookAccountID = "instagram-professional-user-new"
	}
	return profile, nil
}
```

Allow tests to inject `recordingOutcomeWriter` through `SetIntegrationLogger` by changing the handler field to the shared writer interface.

- [ ] **Step 2: Write failing Facebook classification callback tests**

For each typed connector failure, assert:

- redirect or direct HTML uses the correct stable public reason/presentation;
- Session status remains pending;
- one result-log insert attempt occurs;
- `ErrorCode`, message, Page counts, request ID, Session ID, and external user ID are correct;
- the raw marker is absent from event payload, redirect, and HTML.

Use request context populated with the request ID middleware helper or its context key through the actual middleware.

- [ ] **Step 3: Write failing all-platform outcome matrix**

Create table cases for every registered OAuth Hosted Connect platform:

```go
for _, platformName := range []string{
	"twitter", "linkedin", "youtube", "tiktok", "instagram", "threads", "facebook", "pinterest",
} {
	// success case: one callback_succeeded event
	// exchange failure case: one callback_failed event
}
```

Add focused cases for provider cancellation, connector resolution failure, profile failure, provider identity failure, ownership conflict, plan failure, encryption failure, save failure, Instagram subscription failure, completion-claim failure, repeat callback, and success. Each attributable path must make one attempt; invalid/missing state and unresolved Workspace must make zero.

- [ ] **Step 4: Run callback tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestConnectCallback_(FacebookFailures|OutcomeMatrix|OutcomeFailurePaths)' -count=1
```

Expected: FAIL because several paths produce zero logs, existing paths use the async writer, and Facebook reasons collapse to `token_exchange_failed`.

- [ ] **Step 5: Attribute the attempt before terminal checks**

After resolving a Session, resolve its profile/Workspace before platform/status/expiry terminal responses. If Workspace resolution fails, remain in the documented unattributable exception. Once attribution succeeds:

```go
outcome := newHostedConnectOutcome(r.Context(), h.ilog, hostedConnectAttempt{
	WorkspaceID: workspaceID,
	ProfileID: session.ProfileID,
	Platform: session.Platform,
	SessionID: session.ID,
	ExternalUserID: session.ExternalUserID,
	RequestID: appmw.GetRequestID(r.Context()),
})
w = wrapHostedConnectResponse(w, outcome)
```

Default failure state is a fixed `connect_callback_failed`; every known return updates it before rendering.

- [ ] **Step 6: Map every OAuth outcome**

Use fixed codes/messages. At minimum:

| Path | Code | Result message |
| --- | --- | --- |
| provider cancel | `access_denied` | `Hosted Connect was cancelled by the user.` |
| provider error | bounded provider code or `authorization_failed` | `Hosted Connect failed during authorization.` |
| connector resolution | `connector_resolution_failed` | `Hosted Connect failed during authorization.` |
| exchange, non-Facebook | `token_exchange_failed` | `Hosted Connect failed during authorization.` |
| exchange, unexpected Facebook | `facebook_authorization_failed` | `Hosted Connect failed during authorization.` |
| Facebook Page empty | `facebook_page_not_available` | `Hosted Connect failed: no manageable Facebook Page was found.` |
| Facebook no publish task | `facebook_page_permission_required` | `Hosted Connect failed: Facebook Page publishing permission is required.` |
| profile | `profile_fetch_failed` | `Hosted Connect failed during authorization.` |
| ownership | `account_ownership_conflict` or bounded lookup code | `Hosted Connect failed while verifying account ownership.` |
| plan | `managed_account_limit_reached` | `Hosted Connect failed because the workspace account limit was reached.` |
| encryption | `credential_encryption_failed` | `Hosted Connect failed while securing credentials.` |
| save | `account_save_failed` | `Hosted Connect failed while saving the account.` |
| Instagram webhook failure | `webhook_subscription_failed` | `Instagram webhook subscription failed.` |
| Instagram containment failure | `webhook_subscription_containment_failed` | `Instagram webhook subscription failed and reconnect containment could not be confirmed.` |
| completion | `session_completion_failed` | `Hosted Connect failed while completing the session.` |
| success | no code | `Hosted Connect completed successfully.` |

Remove all callback `ResponsePayload: {"error": err.Error()}` result writes. Do not include `error_description` in outcome metadata. For Facebook provider errors, public reason is always `facebook_authorization_failed` unless the user cancelled.

Bound provider-supplied error codes before storing them:

```go
var connectErrorCodePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,64}$`)

func boundedConnectErrorCode(raw, fallback string) string {
	raw = strings.TrimSpace(raw)
	if !connectErrorCodePattern.MatchString(raw) {
		return fallback
	}
	return strings.ToLower(raw)
}
```

Before the final redirect call:

```go
outcome.Success(saved.ID, profile.Username)
```

- [ ] **Step 7: Run callback tests and verify GREEN**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestConnectCallback_|TestConnectRedirectFacebookPresentation' -count=1
```

Expected: PASS. Existing concurrency and ownership tests must remain green.

- [ ] **Step 8: Run complete handler tests**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit Task 4**

```bash
git add api/internal/handler/connect_callback.go api/internal/handler/connect_sessions_test.go
git commit -m "feat: record every OAuth Connect outcome"
```

## Task 5: Route Bluesky form attempts through the shared recorder

**Files:**

- Modify: `api/internal/handler/connect_bluesky.go`
- Test: `api/internal/handler/connect_bluesky_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing Bluesky outcome tests**

Add success and invalid-credential tests using `recordingOutcomeWriter`. Assert:

```go
if len(writer.events) != 1 { t.Fatalf("events = %d", len(writer.events)) }
event := writer.events[0]
if event.Action != integrationlogs.ActionAccountConnectCallbackFailed || event.ErrorCode != "bluesky_credentials_rejected" {
	t.Fatalf("event = %+v", event)
}
encoded, _ := json.Marshal(event)
if bytes.Contains(encoded, []byte("app-password-secret")) {
	t.Fatal("outcome leaked Bluesky app password")
}
```

Cover ownership, plan, encryption, save, completion, repeat Session, and success. Missing/invalid state is zero events. A valid Session with missing handle/password is attributable and produces one bounded failure.

- [ ] **Step 2: Run Bluesky tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestConnectBluesky.*Outcome' -count=1
```

Expected: FAIL because the Bluesky handler has no logger or recorder.

- [ ] **Step 3: Add logger injection and early attribution**

Add:

```go
func (h *ConnectBlueskyHandler) SetIntegrationLogger(logger hostedConnectOutcomeWriter) *ConnectBlueskyHandler {
	h.ilog = logger
	return h
}
```

After form parsing, require only Session ID/state before lookup. Resolve Session and profile/Workspace before validating handle/password or calling Bluesky. Wrap the response immediately after attribution. Never place the app password in metadata, errors, or service logs.

- [ ] **Step 4: Map Bluesky outcomes and inject production logger**

Use this complete Bluesky outcome map before rendering each response:

| Path | Error code | Result message |
| --- | --- | --- |
| required form fields missing | `bluesky_credentials_required` | `Hosted Connect failed because Bluesky credentials are required.` |
| connector unavailable | `bluesky_connector_unavailable` | `Hosted Connect failed during authorization.` |
| credentials rejected | `bluesky_credentials_rejected` | `Hosted Connect failed during authorization.` |
| provider identity missing | `provider_identity_missing` | `Hosted Connect failed during authorization.` |
| ownership lookup/conflict | `account_ownership_failed` / `account_ownership_conflict` | `Hosted Connect failed while verifying account ownership.` |
| plan or sharing limit | `managed_account_limit_reached` / `account_unavailable` | `Hosted Connect failed because the account is unavailable to this workspace.` |
| encryption | `credential_encryption_failed` | `Hosted Connect failed while securing credentials.` |
| save | `account_save_failed` | `Hosted Connect failed while saving the account.` |
| completion claim | `session_completion_failed` | `Hosted Connect failed while completing the session.` |
| success | omitted | `Hosted Connect completed successfully.` |

Call `outcome.Success(savedID, connectResult.AccountName)` before the success HTML or redirect. Update main wiring:

```go
connectBlueskyHandler := handler.NewConnectBlueskyHandler(
	queries, encryptor, eventBus, connectOwnershipStore,
).SetIntegrationLogger(integrationLogger)
```

- [ ] **Step 5: Run Bluesky and API wiring tests and verify GREEN**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestConnectBluesky' -count=1
GOCACHE=/tmp/unipost-go-build go test ./cmd/api -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 5**

```bash
git add api/internal/handler/connect_bluesky.go api/internal/handler/connect_bluesky_test.go api/cmd/api/main.go
git commit -m "feat: record Bluesky Hosted Connect outcomes"
```

## Task 6: Make Connect outcomes searchable by exact metadata IDs

**Files:**

- Modify: `api/internal/db/queries/integration_logs.sql`
- Modify (generated): `api/internal/db/integration_logs.sql.go`
- Create: `api/internal/db/integration_logs_search_contract_test.go`
- Modify: `api/internal/handler/logs_test.go`
- Create: `dashboard/src/lib/log-search.ts`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/logs/page.tsx`
- Create: `dashboard/tests/hosted-connect-logs.test.mjs`

- [ ] **Step 1: Write failing SQL contract test**

```go
func TestListIntegrationLogsSearchesExactHostedConnectMetadataIDs(t *testing.T) {
	query := strings.ToLower(listIntegrationLogs)
	for _, predicate := range []string{
		"metadata->>'connect_session_id' =",
		"metadata->>'external_user_id' =",
	} {
		if !strings.Contains(query, predicate) {
			t.Fatalf("query missing %q", predicate)
		}
	}
	for _, forbidden := range []string{
		"metadata->>'connect_session_id' ilike",
		"metadata->>'external_user_id' ilike",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("query contains substring metadata search %q", forbidden)
		}
	}
}
```

- [ ] **Step 2: Run SQL contract test and verify RED**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/db -run TestListIntegrationLogsSearchesExactHostedConnectMetadataIDs -count=1
```

Expected: FAIL because the predicates are absent.

- [ ] **Step 3: Add exact SQL predicates and regenerate**

Inside the existing `query` OR group add:

```sql
OR metadata->>'connect_session_id' = sqlc.arg('query')::TEXT
OR metadata->>'external_user_id' = sqlc.arg('query')::TEXT
```

Run:

```bash
cd api
sqlc generate
```

Only expected generated database files may change; audit the generated diff.

- [ ] **Step 4: Write failing Dashboard exact-search tests**

Create a pure helper test:

```js
import test from "node:test";
import assert from "node:assert/strict";
import { integrationLogMatchesSearch } from "../src/lib/log-search.ts";

const log = {
  message: "Hosted Connect failed during authorization.",
  action: "account.connect.callback_failed",
  metadata: { connect_session_id: "cs_exact", external_user_id: "customer_42" },
};

test("matches complete Hosted Connect metadata IDs", () => {
  assert.equal(integrationLogMatchesSearch(log, "cs_exact"), true);
  assert.equal(integrationLogMatchesSearch(log, "customer_42"), true);
  assert.equal(integrationLogMatchesSearch(log, "cs_ex"), false);
});
```

- [ ] **Step 5: Run Dashboard search test and verify RED**

```bash
cd dashboard
node --test tests/hosted-connect-logs.test.mjs
```

Expected: FAIL because the helper does not exist.

- [ ] **Step 6: Implement and reuse the search helper**

Create:

```ts
import type { IntegrationLog } from "@/lib/api";

export function integrationLogMatchesSearch(
  log: Pick<IntegrationLog, "message" | "action" | "request_id" | "post_id" | "error_code" | "metadata">,
  rawQuery: string,
): boolean {
  const query = rawQuery.trim().toLowerCase();
  if (!query) return true;
  const metadata = log.metadata || {};
  const exactIDs = [metadata.connect_session_id, metadata.external_user_id]
    .filter((value): value is string => typeof value === "string")
    .map((value) => value.toLowerCase());
  if (exactIDs.includes(query)) return true;
  return [log.message, log.action, log.request_id, log.post_id, log.error_code]
    .filter((value): value is string => Boolean(value))
    .some((value) => value.toLowerCase().includes(query));
}
```

Use this helper in `liveMatchesFilters`. Keep server history and live-tail semantics aligned.

- [ ] **Step 7: Run search tests and handler log tests**

Add a handler contract assertion to `api/internal/handler/logs_test.go` so the API layer cannot silently drop the complete ID query before it reaches the SQL layer:

```go
func TestLogsList_ForwardsHostedConnectIdentityQuery(t *testing.T) {
	store := &fakeLogsStore{}
	h := NewLogsHandler(store)
	r := newLogsRequest("/v1/logs?q=cs_exact", "ws_authed")
	w := httptest.NewRecorder()

	h.List(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if store.listParams.Query != "cs_exact" {
		t.Fatalf("query = %q, want cs_exact", store.listParams.Query)
	}
}
```

This is a characterization test for the existing request propagation and must pass before and after the SQL search change.

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/handler -run 'Test(ListIntegrationLogsSearchesExactHostedConnectMetadataIDs|LogsList_)' -count=1
cd ../dashboard
node --test tests/hosted-connect-logs.test.mjs
```

Expected: PASS.

- [ ] **Step 8: Commit Task 6**

```bash
git add api/internal/db/queries/integration_logs.sql api/internal/db/integration_logs.sql.go api/internal/db/integration_logs_search_contract_test.go api/internal/handler/logs_test.go dashboard/src/lib/log-search.ts dashboard/src/app/'(dashboard)'/projects/'[id]'/logs/page.tsx dashboard/tests/hosted-connect-logs.test.mjs
git commit -m "feat: search Connect outcomes by session identity"
```

## Task 7: Update Developer Logs documentation

**Files:**

- Modify: `dashboard/src/app/docs/api/logs/page.tsx`
- Modify: `dashboard/src/app/docs/api/logs/list/page.tsx`
- Test: `dashboard/tests/hosted-connect-logs.test.mjs`

- [ ] **Step 1: Add failing documentation assertions**

Extend the Node test to read both documentation pages and assert the corpus contains:

```js
for (const expected of [
  "account.connect.callback_succeeded",
  "account.connect.callback_failed",
  "account.connect.callback_cancelled",
  "facebook_page_not_available",
  "facebook_page_permission_required",
  "facebook_authorization_failed",
  "connect_session_id",
  "external_user_id",
]) {
  assert.match(corpus, new RegExp(expected));
}
```

- [ ] **Step 2: Run documentation test and verify RED**

```bash
cd dashboard
node --test tests/hosted-connect-logs.test.mjs
```

Expected: FAIL because the new actions and reason codes are not documented.

- [ ] **Step 3: Add safe examples and search guidance**

Document:

- `category=oauth&platform=facebook&status=error` filtering;
- exact `q=<connect_session_id>` and `q=<external_user_id>` matching;
- success, failed, and cancelled actions;
- three Facebook public error codes;
- list payload omission and detail redaction behavior.

Use synthetic IDs only. Do not include production emails, API keys, session IDs, OAuth codes, Page IDs, or Meta response bodies.

- [ ] **Step 4: Run focused Dashboard tests and verify GREEN**

```bash
cd dashboard
npm run test:hosted-connect
```

Expected: all Hosted Connect Node tests pass.

- [ ] **Step 5: Commit Task 7**

```bash
git add dashboard/src/app/docs/api/logs/page.tsx dashboard/src/app/docs/api/logs/list/page.tsx dashboard/tests/hosted-connect-logs.test.mjs dashboard/package.json
git commit -m "docs: document Hosted Connect outcome logs"
```

## Task 8: Full local verification and branch audit

**Files:** All files changed by Tasks 1–7.

- [ ] **Step 1: Run formatting**

```bash
cd api
gofmt -w internal/connect/facebook.go internal/connect/facebook_errors.go internal/connect/facebook_test.go internal/integrationlogs/logger.go internal/integrationlogs/logger_test.go internal/integrationlogs/normalize.go internal/handler/connect_outcome.go internal/handler/connect_outcome_test.go internal/handler/connect_callback.go internal/handler/connect_sessions_test.go internal/handler/connect_bluesky.go internal/handler/connect_bluesky_test.go internal/db/integration_logs_search_contract_test.go
```

Expected: command exits 0. Review any formatting diff before proceeding.

- [ ] **Step 2: Run complete API suite**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: PASS with zero failed, skipped, cancelled, timed-out, or empty packages required by the repository.

- [ ] **Step 3: Run focused Dashboard tests**

```bash
cd dashboard
npm run test:hosted-connect
```

Expected: PASS.

- [ ] **Step 4: Build Dashboard**

```bash
cd dashboard
npm run build
```

Expected: Next.js production build exits 0.

- [ ] **Step 5: Run Dashboard regression**

```bash
cd dashboard
npm run test:regression:dashboard
```

Expected: all Playwright regression tests pass. If browsers are unavailable, this is a hard stop before PR creation unless the user explicitly approves an exception after reviewing the evidence.

- [ ] **Step 6: Run security and formatting scans**

```bash
git diff --check origin/staging...HEAD
! rg -n 'string\(body\)|ResponsePayload:.*err\.Error\(\)' api/internal/connect/facebook.go api/internal/handler/connect_callback.go
! rg -n 'up_live_|oauth-code-secret|raw-provider-secret|app-password-secret' --glob '!**/*_test.go' --glob '!docs/superpowers/**' .
```

Expected:

- `git diff --check` exits 0;
- the unsafe Facebook/callback scan returns no matches;
- secret markers occur only in negative tests where explicitly expected.

- [ ] **Step 7: Audit branch-only commits and files**

```bash
git log --oneline origin/staging..HEAD
git diff --name-status origin/staging...HEAD
git status --short
```

Expected: only the PRD, implementation plan, and files listed in this plan; worktree clean; no unrelated artifacts.

- [ ] **Step 8: Commit any verification-only corrections**

If formatting or verification required tracked corrections, stage only those exact files and commit:

```bash
git commit -m "test: complete Hosted Connect regression coverage"
```

Do not create an empty commit.

## Task 9: Hotfix pull request and deployed acceptance

**Files:** No new implementation files unless a failed gate requires a tested correction.

- [ ] **Step 1: Push only the owned hotfix branch**

```bash
git push -u origin hotfix-facebook-connect-error-guidance
```

Expected: remote branch points to the locally verified head SHA.

- [ ] **Step 2: Open the staging pull request**

Open a PR from `hotfix-facebook-connect-error-guidance` to `staging` with the PRD, test evidence, security remediation, and explicit deployment acceptance checklist. Do not merge while any check is pending.

- [ ] **Step 3: Monitor every check on the exact head SHA**

Wait for GitHub Actions and every triggered Railway/Vercel check. Any failed, errored, skipped, cancelled, timed-out, non-starting, or different-SHA result is a hard stop.

- [ ] **Step 4: Merge to staging and wait for deployments**

Before merge, re-audit `origin/staging..HEAD` commits and files. After merge, wait for staging API and Dashboard deployments to succeed.

- [ ] **Step 5: Perform staging acceptance**

On staging domains, verify:

- Facebook account with zero accessible Pages;
- Facebook account with visible Page but no publishing task;
- Facebook account with a publishable Page;
- representative non-Facebook OAuth outcome;
- Bluesky form outcome;
- one Workspace result row per attributable attempt;
- exact Session/external-user search;
- no secrets or raw provider bodies;
- deployed SHA equals the accepted SHA.

- [ ] **Step 6: Promote staging to main**

Only after staging acceptance passes, open and merge `staging` → `main`, monitor all checks/deployments, and repeat bounded production acceptance from the PRD.

- [ ] **Step 7: Sync the hotfix to dev**

After production verification, sync the same owned branch with latest `origin/dev`, stop on conflicts, rerun required validation, open the PR to `dev`, complete Preview Acceptance, merge, wait for development deployment, and verify development domains.
