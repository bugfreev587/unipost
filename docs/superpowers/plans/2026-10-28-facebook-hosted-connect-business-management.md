# Facebook Hosted Connect Business Management Implementation Plan

> Execute in the conversation-owned `dev-facebook-business-management` worktree. Keep the production change limited to the Hosted Connect Facebook scope list.

**Goal:** Make Facebook Hosted Connect request `business_management` so Meta can return Business Portfolio-managed Pages through the existing `/me/accounts` flow.

**Architecture:** Preserve the current OAuth and Page discovery pipeline. Extend its canonical Hosted Connect scope slice by one permission and lock both authorization and token metadata behavior with focused tests.

**Tech stack:** Go, `net/url`, `testing`, Facebook Graph API OAuth

---

## Task 1: Add failing authorization-scope coverage

**Files:**

- Modify: `api/internal/connect/facebook_test.go`

1. Extend `TestFacebookConnectorAuthorizeURL` to parse the generated URL and assert that its space-delimited `scope` value contains `business_management` exactly once.
2. Run `GOCACHE=/tmp/unipost-go-build go test ./internal/connect -run TestFacebookConnectorAuthorizeURL -count=1` from `api/`.
3. Confirm the test fails because the current Hosted Connect scope list omits `business_management`.

## Task 2: Add failing TokenSet scope coverage

**Files:**

- Modify: `api/internal/connect/facebook_test.go`

1. Extend the successful exchange test to assert `TokenSet.Scopes` contains `business_management` exactly once while retaining existing Page token, zero-expiry, and empty-refresh-token assertions.
2. Run `GOCACHE=/tmp/unipost-go-build go test ./internal/connect -run TestFacebookConnectorExchangeSelectsFirstPublishablePage -count=1`.
3. Confirm the test fails for the missing scope and not for token-selection behavior.

## Task 3: Implement the minimal production change

**Files:**

- Modify: `api/internal/connect/facebook.go`

1. Add `business_management` once to `facebookConnectScopes`.
2. Run `gofmt` on the touched Go files.
3. Rerun both focused tests and confirm they pass.

## Task 4: Verify compatibility and repository state

**Files:**

- Verify: `api/internal/connect/facebook.go`
- Verify: `api/internal/connect/facebook_test.go`

1. Run `GOCACHE=/tmp/unipost-go-build go test ./internal/connect ./internal/platform -count=1`.
2. Run `GOCACHE=/tmp/unipost-go-build go test ./... -count=1`.
3. Run `git diff --check` and audit all commits and changed files relative to `origin/main`.
4. Confirm no database, callback, Page-selection, token-refresh, feature-flag, or frontend behavior changed.

## Task 5: Commit the focused implementation

1. Commit the test and implementation changes separately from the approved design documentation.
2. Do not push, open a pull request, deploy, or promote without a subsequent explicit user request.
