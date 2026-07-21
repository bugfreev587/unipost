# Managed-User Inbox Isolation Hotfix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Confine every Inbox HTTP, mutation, sync, X, and realtime request to an explicit managed user while preserving an owner/admin workspace aggregate and preventing Connect from silently reassigning provider accounts.

**Architecture:** Authentication continues to derive workspace and role from Clerk or the workspace API key. A new canonical Inbox scope is resolved once per request and passed explicitly into SQL, X confirmation, WebSocket, and sync paths; managed mode adds a social_accounts.external_user_id predicate while workspace mode remains owner/admin-only. Connect ownership uses a transaction-scoped advisory lock and workspace-wide provider-identity lookup, with no destructive data migration or automatic historical repair.

**Tech Stack:** Go 1.x, chi, pgx/pgxpool, sqlc, PostgreSQL advisory locks and LISTEN/NOTIFY, Clerk, workspace API keys, Next.js/TypeScript documentation, Node source-contract tests, GitHub Actions, Railway, Vercel.

**Approved design:** docs/superpowers/specs/2026-07-20-managed-user-inbox-isolation-hotfix-design.md

**Progress:** 0/21 tasks complete.

---

## File map

**Create**

- api/internal/auth/token_auth.go — reusable Clerk/API-key token verification for HTTP and WebSocket.
- api/internal/auth/token_auth_test.go — creator-bound, legacy, revoked, and membership tests.
- api/internal/inboxaccess/scope.go — canonical scope type, request resolver, context helpers, and stable errors.
- api/internal/inboxaccess/scope_test.go — API-key/Clerk authorization matrix.
- api/internal/handler/inbox_scope.go — chi middleware that resolves scope once for all Inbox routes.
- api/internal/handler/inbox_scope_test.go — route-wide fail-closed tests.
- api/internal/connectownership/store.go — advisory-lock ownership transaction and reconnect decision.
- api/internal/connectownership/store_test.go — reconnect/conflict/concurrency decision tests.
- scripts/inbox-scope-preflight.sql — read-only creatorless-key and ambiguous-account audit.
- scripts/inbox-scope-preflight.test.mjs — proves the audit cannot write.
- scripts/inbox-scope-acceptance.mjs — deployed HTTP/WebSocket isolation acceptance using UniPost-owned fixtures.

**Modify**

- api/internal/auth/dualauth.go, api/internal/auth/unkey.go — call shared token auth and expose creator attribution.
- api/internal/db/queries/inbox.sql, api/internal/db/queries/social_accounts.sql — explicit managed/workspace predicates and ownership queries.
- api/internal/db/inbox.sql.go, api/internal/db/social_accounts.sql.go — sqlc-generated output only.
- api/internal/db/inbox_tenant_isolation_contract_test.go — source and transactional managed-user matrix.
- api/internal/handler/inbox.go, api/internal/handler/inbox_x_confirmation.go, api/internal/handler/inbox_x_outbound.go — consume scope on all user-facing paths.
- api/internal/handler/connect_callback.go, api/internal/handler/connect_bluesky.go — replace profile-only reconnect with ownership store.
- api/internal/handler/connect_sessions_test.go — legitimate reconnect and ownership-conflict coverage.
- api/internal/ws/handler.go, api/internal/ws/hub.go, api/internal/ws/pgnotify.go — API-key handshake and scoped delivery.
- api/internal/ws/handler_test.go, api/internal/ws/hub_subscribe_test.go — handshake and delivery matrix.
- api/internal/handler/meta_webhook.go, api/internal/worker/inbox_sync.go — emit internally derived external-user ownership.
- api/cmd/api/main.go — mount the Inbox scope middleware and inject Connect ownership store.
- dashboard/src/app/docs/api/inbox/page.tsx and the list/reply/sync detail pages — explicit server-side scope examples.
- dashboard/src/app/docs/guides/x/comments/page.tsx, dashboard/src/app/docs/guides/x/direct-messages/page.tsx, dashboard/src/app/docs/guides/x/reconnect-permissions/page.tsx — scoped X examples.
- dashboard/tests/x-inbox-docs-source.test.mjs, scripts/smoke-test.sh, docs/sdk-api-coverage-matrix.md — contract and smoke coverage.

**Explicitly not created**

- No database migration that rewrites Inbox rows.
- No workspace-level unique index in this hotfix.
- No feature flag.
- No customer-account test fixture.

---

### Task 1: Re-establish the owned hotfix baseline

**Files:**

- Inspect only: repository and remote branch state.

- [ ] **Step 1: Verify the exclusive worktree before fetching**

Run:

~~~bash
pwd
test "$(pwd)" = "/Users/xiaoboyu/.config/superpowers/worktrees/unipost/hotfix-inbox-comments-idor"
test "$(git branch --show-current)" = "hotfix-inbox-comments-idor"
git status --short
~~~

Expected: the exact owned path and branch; no uncommitted files except this committed plan. Any mismatch is an immediate stop.

- [ ] **Step 2: Fetch and audit staging without changing files**

Run:

~~~bash
git fetch origin
git log --oneline --decorate -20 origin/staging
git log --oneline origin/staging..HEAD
git diff --name-status origin/staging...HEAD
git merge-tree --write-tree HEAD origin/staging
~~~

Expected: the merge-tree check reports no conflicts. If it reports any conflict, stop and ask the user; do not switch, stash, reset, or resolve unrelated state.

- [ ] **Step 3: Bring the owned branch to the latest staging base**

Run only after repeating the Step 1 worktree/branch verification:

~~~bash
git merge --no-edit origin/staging
~~~

Expected: clean merge or already up to date. If a merge commit is created, list its files and confirm they came only from origin/staging.

- [ ] **Step 4: Capture the baseline SHA and required test health**

Run:

~~~bash
git rev-parse HEAD
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/auth ./internal/db ./internal/handler ./internal/ws
~~~

Expected: all four packages PASS. Any failure is a hard stop before implementation.

---

### Task 2: Share API-key authentication and preserve creator attribution

**Files:**

- Create: api/internal/auth/token_auth.go
- Create: api/internal/auth/token_auth_test.go
- Modify: api/internal/auth/dualauth.go
- Modify: api/internal/auth/unkey.go
- Test: api/internal/auth/dualauth_api_key_test.go

- [ ] **Step 1: Write failing tests for reusable token auth**

Add tests that assert a creator-bound key sets workspace, key ID, current role, and CreatorBound=true; a creatorless key keeps legacy RoleOwner for unrelated APIs but sets CreatorBound=false; removed creators fail authentication.

~~~go
func TestAuthenticateAPIKeyTokenExposesCreatorAttribution(t *testing.T) {
	ctx, failure := AuthenticateAPIKeyToken(context.Background(), db.New(&apiKeyAuthTestDB{
		creatorUserID: "user_admin",
		membershipRole: RoleAdmin,
	}), apiKeyAuthTestToken)
	if failure != nil {
		t.Fatalf("AuthenticateAPIKeyToken failure = %+v", failure)
	}
	if !GetAPIKeyCreatorBound(ctx) {
		t.Fatal("creator-bound key was marked legacy")
	}
	if GetRole(ctx) != RoleAdmin {
		t.Fatalf("role = %q, want admin", GetRole(ctx))
	}
}

func TestAuthenticateAPIKeyTokenMarksLegacyKeyUnattributed(t *testing.T) {
	ctx, failure := AuthenticateAPIKeyToken(context.Background(), db.New(&apiKeyAuthTestDB{}), apiKeyAuthTestToken)
	if failure != nil {
		t.Fatalf("AuthenticateAPIKeyToken failure = %+v", failure)
	}
	if GetAPIKeyCreatorBound(ctx) {
		t.Fatal("creatorless key must not be aggregate-capable")
	}
	if GetRole(ctx) != RoleOwner {
		t.Fatalf("legacy compatibility role = %q, want owner", GetRole(ctx))
	}
}
~~~

- [ ] **Step 2: Run the tests and confirm the missing API**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/auth -run 'TestAuthenticateAPIKeyToken' -count=1
~~~

Expected: FAIL because AuthenticateAPIKeyToken and GetAPIKeyCreatorBound do not exist.

- [ ] **Step 3: Implement the shared result and context marker**

Create the reusable API with this contract, moving the existing revocation, expiry, last-used, membership, and role logic without weakening it:

~~~go
type TokenAuthFailure struct {
	Status  int
	Code    string
	Message string
}

const apiKeyCreatorBoundKey contextKey = "apiKeyCreatorBound"

func GetAPIKeyCreatorBound(ctx context.Context) bool {
	value, _ := ctx.Value(apiKeyCreatorBoundKey).(bool)
	return value
}

func AuthenticateAPIKeyToken(
	ctx context.Context,
	queries *db.Queries,
	token string,
) (context.Context, *TokenAuthFailure) {
	ak, err := queries.GetAPIKeyByHash(ctx, apikey.Hash(token))
	if err != nil {
		return nil, &TokenAuthFailure{Status: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: "Invalid API key"}
	}
	if ak.RevokedAt.Valid {
		return nil, &TokenAuthFailure{Status: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: "API key has been revoked"}
	}
	if ak.ExpiresAt.Valid && ak.ExpiresAt.Time.Before(time.Now()) {
		return nil, &TokenAuthFailure{Status: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: "API key has expired"}
	}
	go func(keyID string) {
		if updateErr := queries.UpdateAPIKeyLastUsedAt(context.Background(), keyID); updateErr != nil {
			slog.Error("failed to update last_used_at", "key_id", keyID, "error", updateErr)
		}
	}(ak.ID)
	result := context.WithValue(ctx, WorkspaceIDKey, ak.WorkspaceID)
	result = context.WithValue(result, APIKeyIDKey, ak.ID)
	result = context.WithValue(result, apiKeyCreatorBoundKey, ak.CreatedByUserID != "")
	role := RoleOwner
	if ak.CreatedByUserID != "" {
		membership, membershipErr := queries.GetMembership(ctx, db.GetMembershipParams{
			WorkspaceID: ak.WorkspaceID,
			UserID: ak.CreatedByUserID,
		})
		if membershipErr != nil || membership.Status != "active" {
			return nil, &TokenAuthFailure{Status: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: "API key is no longer authorized"}
		}
		role = membership.Role
	}
	return context.WithValue(result, RoleKey, role), nil
}
~~~

The implementation must retain the existing detached last_used_at update and must never log the token.

- [ ] **Step 4: Make both HTTP middleware paths call the shared function**

dualauth.go and unkey.go should format TokenAuthFailure through their existing JSON writer:

~~~go
ctx, failure := AuthenticateAPIKeyToken(r.Context(), queries, token)
if failure != nil {
	writeJSON(w, failure.Status, map[string]any{
		"error": map[string]any{"code": failure.Code, "message": failure.Message},
	})
	return
}
next.ServeHTTP(w, r.WithContext(ctx))
~~~

- [ ] **Step 5: Verify and commit**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/auth -count=1
cd ..
git diff --check
git add api/internal/auth/token_auth.go api/internal/auth/token_auth_test.go api/internal/auth/dualauth.go api/internal/auth/unkey.go api/internal/auth/dualauth_api_key_test.go
git commit -m "refactor(auth): share api key token authentication"
~~~

Expected: PASS and one focused commit.

---

### Task 3: Define and resolve the canonical Inbox scope

**Files:**

- Create: api/internal/inboxaccess/scope.go
- Create: api/internal/inboxaccess/scope_test.go
- Modify: api/internal/db/queries/social_accounts.sql
- Generated: api/internal/db/social_accounts.sql.go

- [ ] **Step 1: Add the managed-user existence query and failing resolver matrix**

Add this non-status-filtered ownership check so disconnected history retains its owner:

~~~sql
-- name: InboxManagedUserExists :one
SELECT EXISTS (
  SELECT 1
  FROM social_accounts sa
  JOIN profiles p ON p.id = sa.profile_id
  WHERE p.workspace_id = @workspace_id
    AND sa.connection_type = 'managed'
    AND sa.external_user_id = @external_user_id
);
~~~

Add table tests for API-key missing scope, contradictory params, managed A, unknown A, admin workspace, editor workspace, legacy workspace, Clerk admin default, and Clerk editor.

~~~go
func TestResolveAPIKeyMissingScopeFailsClosed(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/inbox", nil)
	ctx := auth.SetAPIKeyID(auth.SetRole(auth.SetWorkspaceID(request.Context(), "ws_1"), auth.RoleAdmin), "key_1")
	request = request.WithContext(auth.SetAPIKeyCreatorBound(ctx, true))
	_, failure := Resolve(request, db.New(&scopeTestDB{}))
	if failure == nil || failure.Status != 400 || failure.Code != "INBOX_SCOPE_REQUIRED" {
		t.Fatalf("failure = %+v", failure)
	}
}
~~~

- [ ] **Step 2: Generate sqlc and run the failing package**

Run:

~~~bash
cd api
sqlc generate
GOCACHE=/tmp/unipost-go-build go test ./internal/inboxaccess -count=1
~~~

Expected: FAIL because the inboxaccess package and auth.SetAPIKeyCreatorBound are not implemented.

- [ ] **Step 3: Implement scope, context, and stable errors**

Use an explicit enum; an empty external ID never means workspace mode:

~~~go
type Mode string

const (
	ModeWorkspace   Mode = "workspace"
	ModeManagedUser Mode = "managed_user"
)

type Scope struct {
	WorkspaceID    string
	Mode           Mode
	ExternalUserID string
}

func (s Scope) WorkspaceWide() bool {
	return s.Mode == ModeWorkspace
}

type Failure struct {
	Status  int
	Code    string
	Message string
}

func FromContext(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(scopeContextKey{}).(Scope)
	return scope, ok
}

func WithContext(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeContextKey{}, scope)
}
~~~

Resolve rules:

1. Workspace always comes from auth.GetWorkspaceID.
2. API-key requests require exactly one inbox_scope value.
3. Clerk owner/admin requests default to workspace when inbox_scope is absent.
4. API-key managed mode accepts exactly one trimmed external_user_id only after InboxManagedUserExists.
5. Workspace mode rejects external_user_id, roles below admin, and creatorless API keys.
6. Clerk roles below admin receive 403 for both modes.
7. Unknown managed users receive 404.

- [ ] **Step 4: Make creator attribution settable only for tests/shared auth**

Add the setter used by shared auth and tests:

~~~go
func SetAPIKeyCreatorBound(ctx context.Context, bound bool) context.Context {
	return context.WithValue(ctx, apiKeyCreatorBoundKey, bound)
}
~~~

- [ ] **Step 5: Verify and commit**

Run:

~~~bash
cd api
sqlc generate
GOCACHE=/tmp/unipost-go-build go test ./internal/auth ./internal/inboxaccess -count=1
cd ..
git diff --check
git add api/internal/auth/token_auth.go api/internal/inboxaccess api/internal/db/queries/social_accounts.sql api/internal/db/social_accounts.sql.go
git commit -m "feat(inbox): resolve explicit access scope"
~~~

Expected: resolver matrix PASS; generated file has no manual edits.

---

### Task 4: Enforce scope middleware on every HTTP Inbox route

**Files:**

- Create: api/internal/handler/inbox_scope.go
- Create: api/internal/handler/inbox_scope_test.go
- Modify: api/cmd/api/main.go

- [ ] **Step 1: Write a failing route contract test**

The test mounts two sentinel handlers under the Inbox route and proves a missing API-key scope never reaches either:

~~~go
func TestRequireInboxAccessScopeBlocksEveryRoute(t *testing.T) {
	called := 0
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called++ })
	request := httptest.NewRequest(http.MethodGet, "/v1/inbox", nil)
	ctx := auth.SetWorkspaceID(request.Context(), "ws_1")
	ctx = auth.SetRole(ctx, auth.RoleAdmin)
	ctx = auth.SetAPIKeyID(ctx, "key_1")
	ctx = auth.SetAPIKeyCreatorBound(ctx, true)
	recorder := httptest.NewRecorder()
	RequireInboxAccessScope(db.New(&scopeHandlerDB{}))(next).ServeHTTP(recorder, request.WithContext(ctx))
	if recorder.Code != http.StatusBadRequest || called != 0 {
		t.Fatalf("status=%d called=%d", recorder.Code, called)
	}
}
~~~

- [ ] **Step 2: Run and confirm failure**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run TestRequireInboxAccessScope -count=1
~~~

Expected: FAIL because RequireInboxAccessScope does not exist.

- [ ] **Step 3: Implement middleware with the shared error envelope**

~~~go
func RequireInboxAccessScope(queries *db.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scope, failure := inboxaccess.Resolve(r, queries)
			if failure != nil {
				slog.Warn("Inbox scope rejected",
					"event", "inbox_scope_rejected",
					"reason", failure.Code,
					"workspace_id", auth.GetWorkspaceID(r.Context()),
				)
				writeError(w, failure.Status, failure.Code, failure.Message)
				return
			}
			next.ServeHTTP(w, r.WithContext(inboxaccess.WithContext(r.Context(), scope)))
		})
	}
}
~~~

- [ ] **Step 4: Mount it once above all eleven HTTP routes**

The route order must be:

~~~go
r.Route("/v1/inbox", func(r chi.Router) {
	r.Use(handler.RequireInboxAccessScope(queries))
	r.Use(handler.RequirePlanInbox(quotaChecker))
	r.Get("/", inboxHandler.List)
	r.Get("/unread-count", inboxHandler.UnreadCount)
	r.Get("/x-outbound-operations/{requestID}", inboxHandler.XOutboundStatus)
	r.Post("/mark-all-read", inboxHandler.MarkAllRead)
	r.Post("/sync", inboxHandler.Sync)
	r.Get("/{id}", inboxHandler.Get)
	r.Get("/{id}/media-context", inboxHandler.MediaContext)
	r.Post("/{id}/read", inboxHandler.MarkRead)
	r.Post("/{id}/reply", inboxHandler.Reply)
	r.Post("/{id}/thread-state", inboxHandler.UpdateThreadState)
})
~~~

- [ ] **Step 5: Verify and commit**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestRequireInboxAccessScope|TestRequirePlanInbox' -count=1
cd ..
git diff --check
git add api/internal/handler/inbox_scope.go api/internal/handler/inbox_scope_test.go api/cmd/api/main.go
git commit -m "feat(inbox): require scope on http routes"
~~~

Expected: scope errors occur before plan/provider work.

---

### Task 5: Scope list, unread count, and object reads in SQL

**Files:**

- Modify: api/internal/db/queries/inbox.sql
- Modify: api/internal/db/inbox_tenant_isolation_contract_test.go
- Generated: api/internal/db/inbox.sql.go

- [ ] **Step 1: Extend the SQL contract tests before queries**

Add ListInboxItemsByWorkspace, CountUnreadByWorkspace, and GetInboxItem expectations for this exact predicate:

~~~go
for _, name := range []string{
	"ListInboxItemsByWorkspace",
	"CountUnreadByWorkspace",
	"GetInboxItem",
} {
	query := inboxTenantIsolationQuery(t, source, name)
	for _, want := range []string{
		"sqlc.arg('workspace_scope')::boolean",
		"sa.external_user_id = sqlc.arg('external_user_id')::text",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("%s missing managed-user predicate %q", name, want)
		}
	}
}
~~~

Extend the transactional fixture to create managed users A/B and one BYO NULL account in one workspace; assert A sees only A, B sees only B, and workspace mode sees A+B+BYO.

- [ ] **Step 2: Run and confirm the contract fails**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestInboxTenantIsolationAuthenticatedQueriesDeriveWorkspace|TestInboxManagedUser' -count=1
~~~

Expected: FAIL because the three queries do not accept scope.

- [ ] **Step 3: Add explicit scope predicates**

Add the following condition to each query after both workspace predicates:

~~~sql
AND (
  sqlc.arg('workspace_scope')::BOOLEAN
  OR sa.external_user_id = sqlc.arg('external_user_id')::TEXT
)
~~~

Do not use COALESCE and do not infer workspace mode from an empty value. Keep current active/disconnected filters on list and unread count; GetInboxItem keeps history semantics.

- [ ] **Step 4: Generate and update transactional calls**

Every test call passes both fields:

~~~go
items, err := queries.ListInboxItemsByWorkspace(ctx, ListInboxItemsByWorkspaceParams{
	WorkspaceID: "inbox-isolation-workspace-1",
	Limit: 20,
	WorkspaceScope: false,
	ExternalUserID: "managed-a",
})
~~~

- [ ] **Step 5: Verify and commit**

Run:

~~~bash
cd api
sqlc generate
GOCACHE=/tmp/unipost-go-build go test ./internal/db -count=1
cd ..
git diff --check
git add api/internal/db/queries/inbox.sql api/internal/db/inbox.sql.go api/internal/db/inbox_tenant_isolation_contract_test.go
git commit -m "fix(inbox): scope reads by managed user"
~~~

Expected: SQL source and transactional matrix PASS.

---

### Task 6: Scope read-state and thread mutations in SQL

**Files:**

- Modify: api/internal/db/queries/inbox.sql
- Modify: api/internal/db/inbox_tenant_isolation_contract_test.go
- Generated: api/internal/db/inbox.sql.go

- [ ] **Step 1: Write failing mutation assertions**

Add managed A/B mutation cases for MarkInboxItemRead, MarkAllInboxItemsRead, and UpdateInboxThreadState. Each B-target operation under A scope must affect zero rows; A and workspace scope must retain their intended behavior.

~~~go
updated, err := queries.MarkAllInboxItemsRead(ctx, MarkAllInboxItemsReadParams{
	WorkspaceID: "inbox-isolation-workspace-1",
	WorkspaceScope: false,
	ExternalUserID: "managed-a",
	ExcludeXDms: false,
})
if err != nil || updated != 1 {
	t.Fatalf("managed A marked=%d error=%v", updated, err)
}
~~~

- [ ] **Step 2: Run and confirm failure**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestInboxManagedUserMutations|TestInboxTenantIsolationAuthenticatedQueriesDeriveWorkspace' -count=1
~~~

Expected: FAIL because mutation params/predicates are absent.

- [ ] **Step 3: Add the explicit predicate to all three mutations**

For UPDATE ... FROM queries, add:

~~~sql
AND (
  sqlc.arg('workspace_scope')::BOOLEAN
  OR sa.external_user_id = sqlc.arg('external_user_id')::TEXT
)
~~~

Keep both stored and derived workspace predicates. UpdateInboxThreadState must not rely only on the preceding Get call.

- [ ] **Step 4: Generate and verify zero-row behavior**

Run:

~~~bash
cd api
sqlc generate
GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestInboxManagedUserMutations|TestInboxTenantIsolationAuthenticatedQueriesDeriveWorkspace' -count=1
~~~

Expected: PASS; B stays unchanged and no mutation reports an internal error for a cross-scope object.

- [ ] **Step 5: Commit**

Run:

~~~bash
cd ..
git diff --check
git add api/internal/db/queries/inbox.sql api/internal/db/inbox.sql.go api/internal/db/inbox_tenant_isolation_contract_test.go
git commit -m "fix(inbox): scope read and thread mutations"
~~~

---

### Task 7: Thread scope through list, get, media, read, reply, and thread handlers

**Files:**

- Modify: api/internal/handler/inbox.go
- Modify: api/internal/handler/inbox_test.go
- Modify: api/internal/handler/inbox_x_outbound.go

- [ ] **Step 1: Write the failing handler authorization matrix**

Create request contexts with inboxaccess.Scope for A and fake DB rows for B. Assert Get, MediaContext, MarkRead, Reply, and UpdateThreadState return 404 before account token decryption, provider adapters, outbound claims, or mutation queries.

~~~go
func managedInboxRequest(method, target string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	ctx := auth.SetWorkspaceID(request.Context(), "ws_1")
	ctx = inboxaccess.WithContext(ctx, inboxaccess.Scope{
		WorkspaceID: "ws_1",
		Mode: inboxaccess.ModeManagedUser,
		ExternalUserID: "managed-a",
	})
	return request.WithContext(ctx)
}
~~~

- [ ] **Step 2: Run and confirm failures/compile errors**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestInboxManagedScope' -count=1
~~~

Expected: FAIL because handlers do not pass the new SQL parameters.

- [ ] **Step 3: Add one helper and use it on every scoped query**

~~~go
func inboxQueryScope(ctx context.Context) (bool, string) {
	scope, ok := inboxaccess.FromContext(ctx)
	if !ok {
		return false, ""
	}
	return scope.WorkspaceWide(), scope.ExternalUserID
}
~~~

List, UnreadCount, Get, MarkRead, MarkAllRead, MediaContext, Reply, XOutboundStatus, and UpdateThreadState must pass WorkspaceScope and ExternalUserID. Every reload and idempotent-reply lookup that calls GetInboxItem must pass the same scope.

- [ ] **Step 4: Make cross-scope mutations return 404 without provider work**

MarkRead and UpdateThreadState must keep the scoped Get first. Reply must load the target through scoped Get before decrypting account tokens or claiming X idempotency. XOutboundStatus must validate its target with the request scope before returning operation state.

All scoped object misses emit one sanitized inbox_scope_object_rejected event containing only workspace ID, route class, and scope mode. Do not perform an unscoped existence query merely to distinguish a guessed cross-scope ID from a nonexistent ID, and do not log the object ID, external_user_id, email, body, or token.

- [ ] **Step 5: Verify and commit**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestInboxManagedScope|TestMediaContext|TestXOutboundStatus' -count=1
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -count=1
cd ..
git diff --check
git add api/internal/handler/inbox.go api/internal/handler/inbox_test.go api/internal/handler/inbox_x_outbound.go
git commit -m "fix(inbox): enforce scope in object handlers"
~~~

Expected: all handler tests PASS and fake provider call counts remain zero for cross-scope IDs.

---

### Task 8: Restrict manual sync and X account selection

**Files:**

- Modify: api/internal/db/queries/inbox.sql
- Generated: api/internal/db/inbox.sql.go
- Modify: api/internal/handler/inbox.go
- Modify: api/internal/handler/inbox_test.go
- Modify: api/internal/worker/inbox_sync.go

- [ ] **Step 1: Write failing account-enumeration tests**

Extend the SQL transaction fixture with Instagram, Facebook, Threads, and X accounts for A, B, and BYO. Managed A must enumerate only A; workspace mode must include all supported accounts.

~~~go
accounts, err := queries.FindInboxAccountsByWorkspace(ctx, FindInboxAccountsByWorkspaceParams{
	WorkspaceID: "inbox-isolation-workspace-1",
	WorkspaceScope: false,
	ExternalUserID: "managed-a",
})
if err != nil {
	t.Fatal(err)
}
for _, account := range accounts {
	if !account.ExternalUserID.Valid || account.ExternalUserID.String != "managed-a" {
		t.Fatalf("escaped account scope: %+v", account)
	}
}
~~~

Add handler tests proving an x_backfill.account_id belonging to B is not found under A and no X adapter read occurs.

- [ ] **Step 2: Run and confirm failure**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/handler -run 'TestInboxManagedUserAccountEnumeration|TestInboxManagedScopeSync' -count=1
~~~

Expected: FAIL because FindInboxAccountsByWorkspace is workspace-only.

- [ ] **Step 3: Scope account enumeration**

Change FindInboxAccountsByWorkspace to accept and enforce:

~~~sql
AND (
  sqlc.arg('workspace_scope')::BOOLEAN
  OR sa.external_user_id = sqlc.arg('external_user_id')::TEXT
)
~~~

Retain p.workspace_id, active/disconnected, and platform predicates.

- [ ] **Step 4: Pass scope into both normal Sync and syncXBackfill**

The top-level handler resolves scope once and both paths call the same scoped account query:

~~~go
workspaceWide, externalUserID := inboxQueryScope(r.Context())
accounts, err := h.queries.FindInboxAccountsByWorkspace(r.Context(), db.FindInboxAccountsByWorkspaceParams{
	WorkspaceID: workspaceID,
	WorkspaceScope: workspaceWide,
	ExternalUserID: externalUserID,
})
~~~

Background worker discovery is internal and must pass workspace mode explicitly wherever generated signatures change. It must never manufacture managed scope from an empty value.

- [ ] **Step 5: Generate, verify, and commit**

Run:

~~~bash
cd api
sqlc generate
GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/handler ./internal/worker -count=1
cd ..
git diff --check
git add api/internal/db/queries/inbox.sql api/internal/db/inbox.sql.go api/internal/handler/inbox.go api/internal/handler/inbox_test.go api/internal/worker/inbox_sync.go
git commit -m "fix(inbox): scope manual and x sync accounts"
~~~

Expected: A never invokes provider reads for B.

---

### Task 9: Bind X confirmation and outbound operation access to scope

**Files:**

- Modify: api/internal/db/queries/inbox.sql
- Generated: api/internal/db/inbox.sql.go
- Modify: api/internal/handler/inbox_x_confirmation.go
- Modify: api/internal/handler/inbox.go
- Modify: api/internal/handler/inbox_test.go

- [ ] **Step 1: Add failing scope-ownership tests for operation snapshots**

Add a query contract and transaction test for account-ID ownership:

~~~sql
-- name: CountInboxAccountsInScope :one
SELECT COUNT(*)::INTEGER
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE p.workspace_id = @workspace_id
  AND sa.id = ANY(@account_ids::TEXT[])
  AND (
    sqlc.arg('workspace_scope')::BOOLEAN
    OR sa.external_user_id = sqlc.arg('external_user_id')::TEXT
  );
~~~

Add a confirmation test where a valid A token is replayed under B. Expected: scope error, status remains pending, and execution_owner stays NULL.

- [ ] **Step 2: Run and confirm failure**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/db -run 'TestXBackfillConfirmationRejectsCrossManagedScope|TestCountInboxAccountsInScope' -count=1
~~~

Expected: FAIL because beginXBackfillConfirmationOperation checks only workspace.

- [ ] **Step 3: Validate scope inside the confirmation transaction before state transition**

Change the begin signature to receive inboxaccess.Scope. After decoding account snapshots and before expiring, leasing, or updating the row:

~~~go
accountIDs := make([]string, 0, len(operation.Accounts))
for _, account := range operation.Accounts {
	accountIDs = append(accountIDs, account.ID)
}
owned, ownershipErr := db.New(tx).CountInboxAccountsInScope(ctx, db.CountInboxAccountsInScopeParams{
	WorkspaceID: scope.WorkspaceID,
	AccountIds: accountIDs,
	WorkspaceScope: scope.WorkspaceWide(),
	ExternalUserID: scope.ExternalUserID,
})
if ownershipErr != nil || int(owned) != len(accountIDs) {
	return xBackfillConfirmationOperation{}, errors.New("X backfill confirmation operation is outside Inbox scope")
}
~~~

Legacy pending operations remain usable only when their stored account IDs reconcile to the current explicit scope.

- [ ] **Step 4: Keep X outbound status and reply replay scoped**

Every user-request call to GetInboxItem passes the current scope. Internal recovery workers pass WorkspaceScope=true with the outbound row's stored workspace; they never accept caller identifiers.

- [ ] **Step 5: Generate, verify, and commit**

Run:

~~~bash
cd api
sqlc generate
GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/handler -run 'TestXBackfill|TestXOutbound|TestInboxManagedScope' -count=1
cd ..
git diff --check
git add api/internal/db/queries/inbox.sql api/internal/db/inbox.sql.go api/internal/handler/inbox_x_confirmation.go api/internal/handler/inbox.go api/internal/handler/inbox_test.go
git commit -m "fix(inbox): bind x operations to access scope"
~~~

Expected: cross-scope token use cannot consume or mutate an operation.

---

### Task 10: Add an API-key Inbox WebSocket handshake

**Files:**

- Modify: api/internal/ws/handler.go
- Modify: api/internal/ws/handler_test.go
- Modify: api/cmd/api/main.go
- Modify: api/internal/auth/token_auth.go

- [ ] **Step 1: Write failing handshake-choice tests**

Inject token authenticators in tests and cover: Clerk token only, Authorization API key only, both credentials, neither credential, API key in query, missing API-key scope, creatorless workspace scope, editor workspace scope, and managed A.

~~~go
func TestInboxWebSocketRejectsBothCredentialForms(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/inbox/ws?token=clerk-token&inbox_scope=workspace", nil)
	request.Header.Set("Authorization", "Bearer up_test_key")
	recorder := httptest.NewRecorder()
	handler := newInboxWebSocketTestHandler()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", recorder.Code)
	}
}
~~~

- [ ] **Step 2: Run and confirm current Clerk-only behavior fails the matrix**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/ws -run 'TestInboxWebSocket' -count=1
~~~

Expected: FAIL for the API-key path and dual-credential rejection.

- [ ] **Step 3: Add an Inbox-only scoped-auth mode**

Keep logs WebSocket Clerk-only. Configure only the Inbox handler:

~~~go
inboxWSHandler := ws.NewHandler(inboxHub, queries).
	WithInboxPlanGate(quotaChecker).
	WithInboxScopeAuth()
~~~

When scoped mode is enabled:

1. Authorization must be exactly Bearer plus an API-key prefix, or token query must contain the Clerk JWT.
2. Both/neither is 401.
3. An API key in a URL parameter is rejected.
4. API-key verification calls auth.AuthenticateAPIKeyToken.
5. Clerk verification resolves current workspace membership and role.
6. inboxaccess.Resolve runs before websocket.Accept and before plan gating.

The customer backend WebSocket client sends Authorization: Bearer $UNIPOST_WORKSPACE_API_KEY on the HTTP upgrade. The API key is never encoded in the URL.

- [ ] **Step 4: Preserve browser Dashboard behavior**

Clerk owner/admin with no inbox_scope resolves workspace mode. Clerk editor is denied. API-key managed mode requires both query parameters while the secret stays in the upgrade Authorization header.

- [ ] **Step 5: Verify and commit**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/auth ./internal/ws -count=1
cd ..
git diff --check
git add api/internal/ws/handler.go api/internal/ws/handler_test.go api/cmd/api/main.go api/internal/auth/token_auth.go
git commit -m "feat(inbox): authenticate scoped api key websockets"
~~~

Expected: the logs handler tests remain unchanged and PASS.

---

### Task 11: Partition the WebSocket hub by workspace and managed user

**Files:**

- Modify: api/internal/ws/hub.go
- Modify: api/internal/ws/hub_subscribe_test.go
- Modify: api/internal/ws/handler.go

- [ ] **Step 1: Write the failing fan-out matrix**

Register A, B, and aggregate connections in one workspace and an aggregate connection in another workspace. Broadcast an A event and assert only A plus same-workspace aggregate receive it.

~~~go
func TestHubBroadcastInboxPartitionsManagedUsers(t *testing.T) {
	hub := NewHub()
	workspace := inboxaccess.Scope{WorkspaceID: "ws_1", Mode: inboxaccess.ModeWorkspace}
	userA := inboxaccess.Scope{WorkspaceID: "ws_1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: "a"}
	userB := inboxaccess.Scope{WorkspaceID: "ws_1", Mode: inboxaccess.ModeManagedUser, ExternalUserID: "b"}
	aggregate := hub.SubscribeScope(workspace)
	a := hub.SubscribeScope(userA)
	b := hub.SubscribeScope(userB)
	hub.BroadcastInbox("ws_1", "a", []byte("event-a"))
	assertReceives(t, aggregate.C(), "event-a")
	assertReceives(t, a.C(), "event-a")
	assertDoesNotReceive(t, b.C())
}
~~~

- [ ] **Step 2: Run and confirm workspace-only fan-out fails**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/ws -run 'TestHubBroadcastInboxPartitionsManagedUsers' -count=1
~~~

Expected: FAIL because SubscribeScope and BroadcastInbox do not exist.

- [ ] **Step 3: Use a comparable structured key**

~~~go
type scopeKey struct {
	workspaceID    string
	mode           inboxaccess.Mode
	externalUserID string
}

func keyForScope(scope inboxaccess.Scope) scopeKey {
	return scopeKey{
		workspaceID: scope.WorkspaceID,
		mode: scope.Mode,
		externalUserID: scope.ExternalUserID,
	}
}
~~~

Store Inbox connections/subscriptions under this key. Keep the existing workspace-only Broadcast and Subscribe behavior for logs.

- [ ] **Step 4: Deliver exact events to two keys only**

~~~go
func (h *Hub) BroadcastInbox(workspaceID, externalUserID string, message []byte) {
	h.broadcastKey(scopeKey{workspaceID: workspaceID, mode: inboxaccess.ModeWorkspace}, message)
	if externalUserID != "" {
		h.broadcastKey(scopeKey{
			workspaceID: workspaceID,
			mode: inboxaccess.ModeManagedUser,
			externalUserID: externalUserID,
		}, message)
	}
}
~~~

ServeHTTP must call ServeScopedConn with the resolved scope. BYO NULL events go only to aggregate.

- [ ] **Step 5: Verify and commit**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/ws -count=1
cd ..
git diff --check
git add api/internal/ws/hub.go api/internal/ws/hub_subscribe_test.go api/internal/ws/handler.go
git commit -m "fix(inbox): partition realtime hub by managed user"
~~~

---

### Task 12: Carry internally derived ownership through PostgreSQL notifications

**Files:**

- Modify: api/internal/ws/pgnotify.go
- Create or modify: api/internal/ws/pgnotify_test.go
- Modify: api/internal/db/queries/inbox.sql
- Generated: api/internal/db/inbox.sql.go
- Modify: api/internal/handler/meta_webhook.go
- Modify: api/internal/handler/meta_webhook_test.go
- Modify: api/internal/handler/inbox.go
- Modify: api/internal/worker/inbox_sync.go
- Modify: api/internal/worker/inbox_sync_test.go

- [ ] **Step 1: Write failing envelope and producer tests**

Assert notification JSON contains externally derived external_user_id, PGListener forwards through BroadcastInbox, and an empty owner/BYO identity reaches aggregate only.

~~~go
type inboxEnvelope struct {
	Type           string `json:"type"`
	WorkspaceID    string `json:"workspace_id"`
	ExternalUserID string `json:"external_user_id,omitempty"`
}
~~~

Add producer tests where a Meta routing row owns managed A and the inbound payload cannot override that value.

- [ ] **Step 2: Run and confirm failure**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/ws ./internal/handler ./internal/worker -run 'Test.*ScopedNotification|Test.*ManagedSyncNotification' -count=1
~~~

Expected: FAIL because the envelope has only workspace_id.

- [ ] **Step 3: Extend exact routing projections with ownership**

FindAllActiveInstagramAccountsByWebhookUserID, FindAllSocialAccountsByPlatformAndExternalID, and ListAllInboxAccounts must select sa.external_user_id. The handler passes the generated value, never the request payload:

~~~go
externalUserID := ""
if route.ExternalUserID.Valid {
	externalUserID = route.ExternalUserID.String
}
h.notify(ctx, route.WorkspaceID, externalUserID, item)
~~~

- [ ] **Step 4: Split scoped and workspace control events**

Use these contracts:

~~~go
func Notify(ctx context.Context, pool *pgxpool.Pool, workspaceID, externalUserID string, item any)
func NotifyEvent(ctx context.Context, pool *pgxpool.Pool, workspaceID, externalUserID string, event map[string]any)
func NotifyWorkspaceEvent(ctx context.Context, pool *pgxpool.Pool, workspaceID string, event map[string]any)
~~~

Manual and background sync group new counts by external_user_id and send one managed event per nonempty owner plus one aggregate event. X backfill uses its already scoped account list. No message body or email enters routing metadata.

Every sync producer uses the exact event type inbox.sync_complete; do not introduce a second spelling or wrap it under item.

- [ ] **Step 5: Generate, verify, and commit**

Run:

~~~bash
cd api
sqlc generate
GOCACHE=/tmp/unipost-go-build go test ./internal/ws ./internal/handler ./internal/worker ./internal/db -count=1
cd ..
git diff --check
git add api/internal/ws api/internal/db/queries/inbox.sql api/internal/db/inbox.sql.go api/internal/handler/meta_webhook.go api/internal/handler/meta_webhook_test.go api/internal/handler/inbox.go api/internal/worker/inbox_sync.go api/internal/worker/inbox_sync_test.go
git commit -m "fix(inbox): route realtime events by managed owner"
~~~

Expected: A events reach A+aggregate, never B.

---

### Task 13: Implement the workspace-level Connect ownership transaction

**Files:**

- Create: api/internal/connectownership/store.go
- Create: api/internal/connectownership/store_test.go
- Modify: api/internal/db/queries/social_accounts.sql
- Generated: api/internal/db/social_accounts.sql.go

- [ ] **Step 1: Write failing pure-decision and SQL contract tests**

Cover: no match=create; same profile/same nonempty user=reconnect; different user=conflict; BYO NULL=conflict; same user/different profile=conflict; multiple matches=conflict.

~~~go
func TestDecideOwnershipRejectsSilentReassignment(t *testing.T) {
	matches := []db.SocialAccount{{
		ID: "account-b",
		ProfileID: "profile-1",
		ExternalUserID: pgtype.Text{String: "managed-b", Valid: true},
	}}
	decision := decide(matches, "profile-1", "managed-a")
	if decision.Kind != Conflict {
		t.Fatalf("decision=%+v, want conflict", decision)
	}
}
~~~

The SQL source test must require profiles.workspace_id, all profiles, active rows, managed and BYO rows, and FOR UPDATE.

- [ ] **Step 2: Run and confirm failure**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/connectownership ./internal/db -run 'TestDecideOwnership|TestConnectOwnershipQuery' -count=1
~~~

Expected: FAIL because package and query do not exist.

- [ ] **Step 3: Add canonical workspace identity lookup**

~~~sql
-- name: ListActiveAccountsByWorkspaceProviderIdentity :many
SELECT sa.*
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE p.workspace_id = @workspace_id
  AND sa.platform = @platform
  AND sa.status = 'active'
  AND sa.disconnected_at IS NULL
  AND (
    (
      @platform = 'instagram'
      AND sa.metadata->>'instagram_webhook_user_id' = @provider_identity
    )
    OR (
      @platform <> 'instagram'
      AND sa.external_account_id = @provider_identity
    )
  )
ORDER BY sa.connected_at DESC, sa.id
FOR UPDATE OF sa;
~~~

- [ ] **Step 4: Implement a transaction-scoped store**

Define the complete input and decision surface:

~~~go
var ErrOwnershipConflict = errors.New("ACCOUNT_OWNERSHIP_CONFLICT")

type DecisionKind string

const (
	Create    DecisionKind = "create"
	Reconnect DecisionKind = "reconnect"
	Conflict  DecisionKind = "conflict"
)

type Decision struct {
	Kind      DecisionKind
	AccountID string
}

type OwnershipKey struct {
	WorkspaceID      string
	ProfileID        string
	Platform         string
	ProviderIdentity string
	ExternalUserID   string
}

type SaveRequest struct {
	WorkspaceID     string
	ProfileID       string
	Platform        string
	ProviderIdentity string
	ExternalUserID  string
	Refresh         db.RefreshConnectedSocialAccountParams
	Upsert          db.UpsertManagedSocialAccountParams
	Create          db.CreateManagedSocialAccountParams
}

type Store struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, queries: db.New(pool)}
}

func applyDecision(
	ctx context.Context,
	queries *db.Queries,
	decision Decision,
	request SaveRequest,
) (db.SocialAccount, error) {
	switch decision.Kind {
	case Reconnect:
		request.Refresh.ID = decision.AccountID
		return queries.RefreshConnectedSocialAccount(ctx, request.Refresh)
	case Create:
		if request.Platform == "bluesky" {
			return queries.CreateManagedSocialAccount(ctx, request.Create)
		}
		return queries.UpsertManagedSocialAccount(ctx, request.Upsert)
	default:
		return db.SocialAccount{}, ErrOwnershipConflict
	}
}
~~~

Add a read-only Check method for early conflict/quota classification. Save must repeat the decision under the advisory lock, so Check is never authoritative under concurrency:

~~~go
func (s *Store) Check(ctx context.Context, key OwnershipKey) (Decision, error) {
	matches, err := s.queries.ListActiveAccountsByWorkspaceProviderIdentity(ctx, db.ListActiveAccountsByWorkspaceProviderIdentityParams{
		WorkspaceID: key.WorkspaceID,
		Platform: key.Platform,
		ProviderIdentity: key.ProviderIdentity,
	})
	if err != nil {
		return Decision{}, err
	}
	return decide(matches, key.ProfileID, key.ExternalUserID), nil
}
~~~

The save transaction then acquires the lock before its authoritative lookup:

~~~go
func (s *Store) Save(ctx context.Context, request SaveRequest) (db.SocialAccount, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.SocialAccount{}, err
	}
	defer tx.Rollback(ctx)
	lockValue := request.WorkspaceID + "\x00" + request.Platform + "\x00" + request.ProviderIdentity
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", lockValue); err != nil {
		return db.SocialAccount{}, err
	}
	queries := db.New(tx)
	matches, err := queries.ListActiveAccountsByWorkspaceProviderIdentity(ctx, db.ListActiveAccountsByWorkspaceProviderIdentityParams{
		WorkspaceID: request.WorkspaceID,
		Platform: request.Platform,
		ProviderIdentity: request.ProviderIdentity,
	})
	if err != nil {
		return db.SocialAccount{}, err
	}
	decision := decide(matches, request.ProfileID, request.ExternalUserID)
	account, err := applyDecision(ctx, queries, decision, request)
	if err != nil {
		return db.SocialAccount{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.SocialAccount{}, err
	}
	return account, nil
}
~~~

ACCOUNT_OWNERSHIP_CONFLICT is typed and contains no email, token, or provider payload. Multiple matches never trigger an update.

- [ ] **Step 5: Generate, verify, and commit**

Run:

~~~bash
cd api
sqlc generate
GOCACHE=/tmp/unipost-go-build go test ./internal/connectownership ./internal/db -count=1
cd ..
git diff --check
git add api/internal/connectownership api/internal/db/queries/social_accounts.sql api/internal/db/social_accounts.sql.go
git commit -m "fix(connect): serialize managed account ownership"
~~~

Expected: decision and SQL contract tests PASS; no migration exists.

---

### Task 14: Replace silent reconnect reassignment in every managed Connect completion

**Files:**

- Modify: api/internal/handler/connect_callback.go
- Modify: api/internal/handler/connect_bluesky.go
- Modify: api/internal/handler/connect_sessions_test.go
- Modify: api/cmd/api/main.go

- [ ] **Step 1: Add failing callback tests**

Test OAuth and Bluesky paths for same-user reconnect success, different-user 409, BYO collision 409, cross-profile 409, and no RefreshConnectedSocialAccount call on conflict.

~~~go
func TestConnectCallbackRejectsCrossManagedUserReassignment(t *testing.T) {
	writer := &fakeOwnershipWriter{saveErr: connectownership.ErrOwnershipConflict}
	handler := newConnectCallbackTestHandler(t).WithOwnershipWriter(writer)
	recorder := httptest.NewRecorder()
	handler.Callback(recorder, connectCallbackRequestFor("managed-a"))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status=%d, want 409", recorder.Code)
	}
	if writer.refreshCalls != 0 {
		t.Fatalf("refresh calls=%d, want 0", writer.refreshCalls)
	}
}
~~~

- [ ] **Step 2: Run and confirm existing code silently refreshes**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestConnect.*Ownership|TestConnectCallbackRejectsCrossManagedUserReassignment' -count=1
~~~

Expected: FAIL; current path finds by profile/provider and overwrites external_user_id.

- [ ] **Step 3: Derive the canonical identity after provider verification**

~~~go
providerIdentity := profile.ExternalAccountID
if platformName == "instagram" {
	providerIdentity = strings.TrimSpace(profile.WebhookAccountID)
}
if providerIdentity == "" {
	renderConnectError(w, http.StatusBadGateway, "Provider account identity is missing.")
	return
}
~~~

Bluesky uses its verified DID. The external_user_id always comes from the signed Connect session, never callback query parameters.

- [ ] **Step 4: Route save through the ownership store**

Remove the profile-only FindActiveManagedSocialAccountByExternalAccount branch from both handlers. Call ownershipStore.Check after provider identity resolution; return 409 immediately for conflicts and use its create/reconnect result for quota classification. After quota checks, call Save, which repeats the decision under the transaction lock. On either conflict return 409 without publishing account.connected, subscribing Instagram webhooks, completing the Connect session, or modifying tokens.

Emit a sanitized managed_account_ownership_conflict event containing workspace ID, platform, conflict class, and number of matches. Do not log provider identity, external_user_id, email, access token, or callback payload.

Production constructors receive one store backed by the API pool:

~~~go
ownershipStore := connectownership.NewStore(pool)
connectCallbackHandler := handler.NewConnectCallbackHandler(
	queries, encryptor, webhookWorker, connectRegistry, apiBaseURL, superAdminChecker, ownershipStore,
)
connectBlueskyHandler := handler.NewConnectBlueskyHandler(
	queries, encryptor, webhookWorker, ownershipStore,
)
~~~

- [ ] **Step 5: Verify and commit**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/connectownership ./internal/handler -run 'TestConnect|TestManaged' -count=1
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -count=1
cd ..
git diff --check
git add api/internal/handler/connect_callback.go api/internal/handler/connect_bluesky.go api/internal/handler/connect_sessions_test.go api/cmd/api/main.go
git commit -m "fix(connect): reject ambiguous managed ownership"
~~~

Expected: legitimate reconnects pass; conflict paths have zero writes and zero external side effects.

---

### Task 15: Publish the contract and add non-destructive acceptance tooling

**Files:**

- Create: scripts/inbox-scope-preflight.sql
- Create: scripts/inbox-scope-preflight.test.mjs
- Create: scripts/inbox-scope-acceptance.mjs
- Modify: dashboard/src/app/docs/api/inbox/page.tsx
- Modify: dashboard/src/app/docs/api/inbox/list/page.tsx
- Modify: dashboard/src/app/docs/api/inbox/reply/page.tsx
- Modify: dashboard/src/app/docs/api/inbox/sync/page.tsx
- Modify: dashboard/src/app/docs/guides/x/comments/page.tsx
- Modify: dashboard/src/app/docs/guides/x/direct-messages/page.tsx
- Modify: dashboard/src/app/docs/guides/x/reconnect-permissions/page.tsx
- Modify: dashboard/tests/x-inbox-docs-source.test.mjs
- Modify: scripts/smoke-test.sh
- Modify: docs/sdk-api-coverage-matrix.md

- [ ] **Step 1: Write failing documentation and audit-safety tests**

The docs source test must require both scope forms and the server-side-key warning. The SQL test rejects every data-definition or data-modification statement and requires a read-only transaction.

~~~js
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

test("Inbox API docs require explicit server-side scope", () => {
  const source = readFileSync("src/app/docs/api/inbox/page.tsx", "utf8");
  assert.match(source, /inbox_scope=managed_user/);
  assert.match(source, /external_user_id=user_123/);
  assert.match(source, /inbox_scope=workspace/);
  assert.match(source, /server-side/i);
});

test("preflight is read only", () => {
  const scriptDirectory = dirname(fileURLToPath(import.meta.url));
  const source = readFileSync(join(scriptDirectory, "inbox-scope-preflight.sql"), "utf8");
  assert.match(source, /SET TRANSACTION READ ONLY/i);
  assert.match(source, /ROLLBACK/i);
  assert.doesNotMatch(source, /\b(INSERT|UPDATE|DELETE|MERGE|ALTER|DROP|CREATE|TRUNCATE)\b/i);
});
~~~

- [ ] **Step 2: Run and confirm failure**

Run:

~~~bash
cd dashboard
node --test tests/x-inbox-docs-source.test.mjs ../scripts/inbox-scope-preflight.test.mjs
~~~

Expected: FAIL because examples and scripts are absent.

- [ ] **Step 3: Add a strictly read-only production preflight**

~~~sql
\set ON_ERROR_STOP on
BEGIN;
SET TRANSACTION READ ONLY;

SELECT COUNT(*) AS active_creatorless_api_keys
FROM api_keys
WHERE BTRIM(created_by_user_id) = ''
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > NOW());

WITH provider_ownership AS (
  SELECT
    p.workspace_id,
    sa.platform,
    CASE
      WHEN sa.platform = 'instagram'
        THEN NULLIF(sa.metadata->>'instagram_webhook_user_id', '')
      ELSE NULLIF(sa.external_account_id, '')
    END AS provider_identity,
    COUNT(*) AS active_rows,
    COUNT(DISTINCT COALESCE(sa.external_user_id, '__owner_byo__')) AS owner_count
  FROM social_accounts sa
  JOIN profiles p ON p.id = sa.profile_id
  WHERE sa.status = 'active'
    AND sa.disconnected_at IS NULL
  GROUP BY p.workspace_id, sa.platform, provider_identity
)
SELECT workspace_id, platform, MD5(provider_identity) AS provider_identity_hash, active_rows, owner_count
FROM provider_ownership
WHERE provider_identity IS NOT NULL
  AND (active_rows > 1 OR owner_count > 1)
ORDER BY workspace_id, platform, provider_identity;

ROLLBACK;
~~~

No execution path may add a repair statement to this file.

- [ ] **Step 4: Add deployed acceptance and update every example**

scripts/inbox-scope-acceptance.mjs requires these environment variables and exits nonzero if any is missing:

~~~js
const required = [
  "INBOX_ACCEPT_API_URL",
  "INBOX_ACCEPT_API_KEY",
  "INBOX_ACCEPT_EXTERNAL_USER_A",
  "INBOX_ACCEPT_EXTERNAL_USER_B",
  "INBOX_ACCEPT_ITEM_A",
  "INBOX_ACCEPT_ITEM_B",
];
for (const name of required) {
  if (!process.env[name]) {
    throw new Error("missing required acceptance input: " + name);
  }
}
~~~

The script must assert:

1. A list contains no B item.
2. B list contains no A item.
3. A GET/read/reply/thread-state attempts against B return 404. The B fixture is already read, requests its current thread state, and has a deliberately non-deliverable test adapter credential so even a regression cannot send a real provider reply.
4. workspace scope returns both fixtures only when the fixture key creator is owner/admin.
5. missing scope returns 400 INBOX_SCOPE_REQUIRED.
6. managed-user WebSocket A never receives a synthetic B event, while aggregate receives A and B.

Documentation examples use managed scope for customer-backend calls:

~~~text
?inbox_scope=managed_user&external_user_id=user_123
~~~

and workspace scope only for owner/admin aggregate:

~~~text
?inbox_scope=workspace
~~~

Update the smoke endpoint to /v1/inbox/unread-count?inbox_scope=workspace.

- [ ] **Step 5: Verify and commit**

Run:

~~~bash
cd dashboard
node --test tests/x-inbox-docs-source.test.mjs ../scripts/inbox-scope-preflight.test.mjs
npm run build
cd ..
git diff --check
git add scripts/inbox-scope-preflight.sql scripts/inbox-scope-preflight.test.mjs scripts/inbox-scope-acceptance.mjs dashboard/src/app/docs/api/inbox dashboard/src/app/docs/guides/x dashboard/tests/x-inbox-docs-source.test.mjs scripts/smoke-test.sh docs/sdk-api-coverage-matrix.md
git commit -m "docs(inbox): publish managed scope contract"
~~~

Expected: docs tests and build PASS. The preflight script contains SELECT only.

---

### Task 16: Run the complete local CI-equivalent gate

**Files:**

- Verify all changed files; do not edit during the first run.

- [ ] **Step 1: Verify worktree, branch, and clean generated output**

Run:

~~~bash
pwd
test "$(pwd)" = "/Users/xiaoboyu/.config/superpowers/worktrees/unipost/hotfix-inbox-comments-idor"
test "$(git branch --show-current)" = "hotfix-inbox-comments-idor"
git status --short
cd api
sqlc generate
cd ..
git status --short
git diff --check
~~~

Expected: sqlc produces no unexpected diff and the worktree is clean. Any generated diff must be reviewed and committed before continuing.

- [ ] **Step 2: Run all backend tests**

Run:

~~~bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
~~~

Expected: every package PASS; no skip, timeout, cancellation, or package with no result is accepted.

- [ ] **Step 3: Run Dashboard build and required regression**

Run:

~~~bash
cd ../dashboard
npm run build
npm run test:regression:dashboard
~~~

Expected: both PASS. Missing Playwright browsers is a failure under the repository rules, not an allowed skip.

- [ ] **Step 4: Run source, smoke-contract, and race-focused tests**

Run:

~~~bash
cd ..
node --test scripts/inbox-scope-preflight.test.mjs
cd api
GOCACHE=/tmp/unipost-go-build go test -race ./internal/inboxaccess ./internal/ws ./internal/connectownership
~~~

Expected: PASS with no race report.

- [ ] **Step 5: Audit unique commits and files**

Run:

~~~bash
cd ..
git fetch origin
git log --format='%h %s' origin/staging..HEAD
git diff --name-status origin/staging...HEAD
git status --short
~~~

Expected: only the original approved hotfix history, the approved design/plan, and managed-user isolation implementation are unique. Any unrelated or unidentified file is a hard blocker.

---

### Task 17: Push the owned hotfix branch and open a Draft PR to staging

**Files:**

- Remote branch: origin/hotfix-inbox-comments-idor
- Pull request base: staging

- [ ] **Step 1: Recheck staging movement and mergeability**

Run:

~~~bash
pwd
test "$(pwd)" = "/Users/xiaoboyu/.config/superpowers/worktrees/unipost/hotfix-inbox-comments-idor"
test "$(git branch --show-current)" = "hotfix-inbox-comments-idor"
git status --short
git fetch origin
git merge-tree --write-tree HEAD origin/staging
~~~

Expected: clean. If origin/staging moved, merge it into the owned branch, rerun all of Task 16 on the replacement SHA, and stop on any conflict.

- [ ] **Step 2: Record the exact candidate SHA and content audit**

Run:

~~~bash
candidate_sha=$(git rev-parse HEAD)
printf '%s\n' "$candidate_sha"
git log --format='%H %s' origin/staging..HEAD
git diff --name-status origin/staging...HEAD
~~~

Expected: candidate_sha identifies exactly the locally accepted content.

- [ ] **Step 3: Push only the owned branch**

Run:

~~~bash
git push --set-upstream origin hotfix-inbox-comments-idor
~~~

Expected: origin/hotfix-inbox-comments-idor points to candidate_sha. Do not push staging, main, or dev.

- [ ] **Step 4: Open a Draft PR to staging**

Run:

~~~bash
gh pr create --draft --base staging --head hotfix-inbox-comments-idor --title "hotfix: isolate Inbox by managed user" --body "Adds explicit managed-user/workspace Inbox scope across HTTP, mutations, sync, X, WebSocket, and Connect ownership. No destructive data migration. Production promotion requires customer-backend scope readiness and read-only preflight."
~~~

Expected: one Draft PR with base staging.

- [ ] **Step 5: Confirm PR head and triggered checks**

Run:

~~~bash
gh pr view --json number,url,isDraft,baseRefName,headRefName,headRefOid,statusCheckRollup
~~~

Expected: base=staging, head=hotfix-inbox-comments-idor, headRefOid=candidate_sha, and every expected check/deployment is present or queued.

---

### Task 18: Complete PR-environment acceptance and merge to staging

**Files:**

- No source changes unless a failing gate is fixed on the owned branch.

- [ ] **Step 1: Monitor all checks on the exact head SHA**

Run:

~~~bash
gh pr checks --watch --interval 15
gh pr view --json headRefOid,statusCheckRollup
~~~

Expected: every GitHub, Railway PR Environment, Vercel Preview, and deployed regression check is SUCCESS on the same head SHA. Failure, error, timeout, cancellation, skip, or missing result is a hard stop.

- [ ] **Step 2: Run the read-only preflight in the isolated PR database**

Run through the PR environment shell without printing the connection URI:

~~~bash
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/inbox-scope-preflight.sql
~~~

Expected: active_creatorless_api_keys=0 and no ambiguous identity rows for the synthetic workspace. The script ends with ROLLBACK.

- [ ] **Step 3: Run deployed synthetic A/B acceptance**

Set the required INBOX_ACCEPT variables only from UniPost-owned PR fixtures, then run:

~~~bash
node scripts/inbox-scope-acceptance.mjs
~~~

Expected: all HTTP, mutation, X, and WebSocket assertions PASS. No customer login or customer API key is used.

- [ ] **Step 4: Mark ready, re-audit, and merge**

Run:

~~~bash
gh pr ready
git fetch origin
git log --format='%H %s' origin/staging..HEAD
git diff --name-status origin/staging...HEAD
gh pr merge --merge --delete-branch=false
~~~

Expected: audit is unchanged and the PR merges into staging. If staging moved or the merge SHA differs from the accepted content, stop and rerun required checks.

- [ ] **Step 5: Capture merge evidence**

Run:

~~~bash
gh pr view --json state,mergedAt,mergeCommit,url
git fetch origin
git rev-parse origin/staging
~~~

Expected: merged state and an origin/staging SHA containing the accepted head.

---

### Task 19: Verify the real staging deployment

**Files:**

- Staging API: https://staging-api.unipost.dev
- Staging app: https://staging-app.unipost.dev

- [ ] **Step 1: Monitor every staging deployment**

Use the PR checks, Railway deployment view, and Vercel deployment view until all triggered staging deployments finish.

Expected: API and app deployments are SUCCESS for the exact origin/staging SHA. A pending deployment means this task remains incomplete.

- [ ] **Step 2: Verify public health without authentication**

Run:

~~~bash
curl --fail --silent --show-error https://staging-api.unipost.dev/health
~~~

Expected: HTTP 200 and healthy response.

- [ ] **Step 3: Run staging read-only preflight**

Run inside the staging environment:

~~~bash
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/inbox-scope-preflight.sql
~~~

Expected: no creatorless active test key and no ambiguous synthetic ownership. Any row blocks production promotion; do not update or delete it automatically.

- [ ] **Step 4: Run staging A/B acceptance and browser acceptance**

Run the acceptance script against https://staging-api.unipost.dev with UniPost-owned staging fixtures. Then open https://staging-app.unipost.dev in the browser with a UniPost-owned owner/admin account and verify aggregate Inbox, unread count, reply controls, and WebSocket reconnect.

Expected: script PASS, aggregate UI shows both synthetic users, and no console/API errors.

- [ ] **Step 5: Record exact staging evidence**

Record staging SHA, Railway/Vercel deployment URLs, acceptance output, browser result, and timestamp in the PR/release notes. Production PR creation is forbidden until all are successful.

---

### Task 20: Promote staging to production and verify isolation

**Files:**

- Promotion source: staging
- Promotion target: main
- Production API: https://api.unipost.dev
- Production app: https://app.unipost.dev

- [ ] **Step 1: Enforce the customer-readiness and read-only data gates**

Before creating the production PR, obtain explicit confirmation that the affected customer backend sends inbox_scope plus external_user_id on every managed-user HTTP request and uses polling until API-key WebSocket staging acceptance is adopted.

Run the production preflight in a read-only transaction:

~~~bash
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/inbox-scope-preflight.sql
~~~

Expected: active_creatorless_api_keys=0 and no unresolved ambiguous provider identity. Any result is reported and blocks promotion; no attribution, delete, merge, or reassignment is performed without separate authorization.

- [ ] **Step 2: Audit and open the staging-to-main PR**

Run:

~~~bash
git fetch origin
git log --format='%H %s' origin/main..origin/staging
git diff --name-status origin/main...origin/staging
gh pr create --base main --head staging --title "hotfix: isolate Inbox by managed user" --body "Promotes the staging-accepted managed-user Inbox isolation hotfix. Customer scoped-call readiness and production read-only preflight are confirmed."
~~~

Expected: only accepted staging content is included. Any unrelated/unaccepted commit is a hard blocker.

- [ ] **Step 3: Monitor checks, merge, and wait for production deployments**

Run:

~~~bash
production_pr=$(gh pr view staging --json number --jq .number)
gh pr checks "$production_pr" --watch --interval 15
gh pr merge "$production_pr" --merge
gh pr view "$production_pr" --json state,mergeCommit,url
~~~

Expected: all checks SUCCESS before merge. After merge, wait for Railway production and Vercel production to complete on the exact origin/main SHA.

- [ ] **Step 4: Run production health and UniPost-owned A/B acceptance**

Run:

~~~bash
curl --fail --silent --show-error https://api.unipost.dev/health
node scripts/inbox-scope-acceptance.mjs
~~~

Set acceptance inputs only from a UniPost-owned production test workspace. Do not log in as guyhass02@gmail.com or any customer managed user.

Expected: health 200; A/B HTTP, mutation, X, and WebSocket isolation PASS.

- [ ] **Step 5: Verify the affected workspace without customer interaction**

Use content-free, read-only reconciliation queries to verify:

1. zero stored/derived workspace mismatches;
2. zero cross-managed-user duplicate event groups;
3. every live Inbox item joins to exactly one social_account external_user_id or intentional BYO NULL;
4. per-external-user counts sum to the owner/admin aggregate.

Expected: all invariants hold. Record main SHA, deployment URLs, health, acceptance, and reconciliation output. No customer UI click, reply, read-state mutation, or provider call is allowed.

---

### Task 21: Sync the hotfix back to dev through Preview Acceptance

**Files:**

- Source branch: hotfix-inbox-comments-idor
- Target branch: dev
- Development API: https://dev-api.unipost.dev
- Development app: https://dev-app.unipost.dev

- [ ] **Step 1: Fetch and perform a conflict-only audit**

Run:

~~~bash
pwd
test "$(pwd)" = "/Users/xiaoboyu/.config/superpowers/worktrees/unipost/hotfix-inbox-comments-idor"
test "$(git branch --show-current)" = "hotfix-inbox-comments-idor"
git status --short
git fetch origin
git merge-tree --write-tree HEAD origin/dev
~~~

Expected: clean merge. This branch previously showed conflicts against dev in dashboard Playwright configs, marketing, SEO, and preview scripts; if any conflict remains, stop and ask the user exactly as required by the hotfix flow. Do not resolve, discard, or overwrite unrelated dev work.

- [ ] **Step 2: Merge dev only when the audit is clean and rerun local CI**

Run:

~~~bash
git merge --no-edit origin/dev
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
cd ../dashboard
npm run build
npm run test:regression:dashboard
~~~

Expected: all PASS on the new head SHA. Any failure stops push/PR.

- [ ] **Step 3: Push the updated owned branch and open a Draft PR to dev**

Run:

~~~bash
cd ..
git push origin hotfix-inbox-comments-idor
gh pr create --draft --base dev --head hotfix-inbox-comments-idor --title "hotfix: sync managed-user Inbox isolation to dev" --body "Backports the production-verified Inbox isolation hotfix to dev. Requires Preview Acceptance on this exact head SHA."
~~~

Expected: Draft PR base=dev, exact current head.

- [ ] **Step 4: Complete Preview Acceptance and merge to dev**

Monitor all GitHub, Railway PR Environment, Vercel Preview, deployed regression, and browser checks. Run the read-only preflight and scripts/inbox-scope-acceptance.mjs against Preview with UniPost fixtures. Mark ready and merge only after every result is SUCCESS on the exact head SHA.

Expected: Preview Acceptance complete with no skipped or missing test.

- [ ] **Step 5: Wait for dev deployment and personally accept it**

Wait for every persistent dev deployment, then verify https://dev-api.unipost.dev/health and run A/B acceptance against dev. Open https://dev-app.unipost.dev in the browser with a UniPost-owned owner/admin account and verify aggregate Inbox and realtime.

Expected: dev API/app match production behavior. Only now may the hotfix be reported fully complete.

---

## Final completion evidence

The implementation is complete only when all 21 tasks are checked and the final report contains:

- exact task branch, staging, main, and dev SHAs;
- unique commit and changed-file audits for each promotion;
- local, Preview, staging, production, and dev test/deployment results;
- Railway, Vercel, GitHub run, and PR URLs;
- read-only preflight and affected-workspace invariant results;
- confirmation that no customer account was used, no customer data was mutated, and no production row was deleted or rewritten by audit/remediation;
- explicit confirmation that HTTP, mutation, sync, X, and WebSocket A/B isolation all passed.
