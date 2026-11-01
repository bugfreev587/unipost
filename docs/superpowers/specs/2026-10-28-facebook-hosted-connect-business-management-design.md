# Facebook Hosted Connect Business-Managed Page Discovery Design

**Status:** Approved

**Date:** 2026-10-28

**Branch:** `hotfix-facebook-business-management`

**Base:** `origin/staging` at `fad8ab530e55db91fc7223d6ee96313fbe52516c`

**Release path:** `hotfix-facebook-business-management` → `staging`

**Owner areas:** Hosted Connect, Facebook integration

## 1. Problem

Facebook Hosted Connect requests Page permissions without requesting `business_management`. Meta can therefore return an empty `/me/accounts` result for a Page that the connecting user manages through a Business Portfolio, even when the user has full control and UniPost is allowed in Business Integrations. The connector reports this observable result as `facebook_page_not_available` with `page_count: 0`.

The Dashboard Facebook authorization path already requests `business_management`; Hosted Connect maintains a separate scope list and has drifted from that behavior.

## 2. Goal

Allow new and reauthorized Facebook Hosted Connect sessions to discover Business Portfolio-owned Pages through the existing `/me/accounts` flow, while preserving all behavior for already-connected accounts and personally owned Pages.

## 3. Design

Add `business_management` to `facebookConnectScopes` in `api/internal/connect/facebook.go`.

The existing authorization URL will then request the permission alongside the current Page scopes. The existing token exchange, `/me/accounts` request, publish-task checks, first-publishable-Page selection, Page token storage, expiry behavior, profile fetch, error contract, and session lifecycle remain unchanged.

Tests will verify both externally observable uses of the scope list:

- the Facebook authorization URL requests `business_management`; and
- the returned `TokenSet.Scopes` records `business_management` after a successful exchange.

Implementation follows test-driven development: add the assertions first and observe them fail against the current code, then add the single production scope entry and rerun the relevant and full backend suites.

## 4. Backward compatibility

- Existing connected Facebook accounts keep their stored Page access tokens and are not disconnected or reauthorized automatically.
- Personal Page connections continue through the same `/me/accounts` and Page-selection logic.
- New connections and explicit reconnections receive the additional Meta consent scope.
- No database migration, feature flag, environment variable, callback change, or API contract change is required.
- A customer affected by the previous empty Page result must reconnect after deployment so Meta can issue a user token containing the newly requested permission.

## 5. Alternatives considered

### Direct Business Portfolio enumeration

Query `/me/businesses`, then each Business Portfolio's `owned_pages` and `client_pages` edges. This was rejected for this fix because Hosted Connect ultimately needs Page access tokens and publishing tasks, not just asset IDs. Adding fallback enumeration would broaden token acquisition, pagination, deduplication, and Page-selection behavior and create more regression risk than the confirmed scope omission requires.

### Refactor all Facebook scopes into one shared package

Centralizing the Dashboard and Hosted Connect scope lists would prevent future drift, but it introduces cross-package refactoring unrelated to the customer incident. The minimal Hosted Connect change is safer for current users; consolidation can be evaluated separately.

## 6. Validation

- Prove the new tests fail before the production change.
- Run `go test ./internal/connect` after the change.
- Run `go test ./internal/connect ./internal/platform` to cover both Facebook authorization paths.
- Run `go test ./...` before claiming backend completion.
- Confirm the branch diff changes only the approved design, implementation, and focused tests.

The initial full-suite run exposed an existing date-sensitive handler test whose
hard-coded reschedule target had passed. Stabilize that test with future month
boundaries and derived period expectations. This is test-only release hygiene;
the production scheduling validation remains unchanged.

## 7. Operational dependency

The Meta app must have publishable access to `business_management`. The `unipost-dev` Meta app currently shows the permission as `Ready to publish`, so this dependency is not blocking development-environment validation.

The first `dev` Draft PR passed ordinary CI but failed Preview Acceptance because
Railway did not create a PR deployment for its exact head SHA. After reviewing the
failure evidence, the user explicitly authorized bypassing that Preview gate and
requested a staging-based hotfix PR with direct staging verification.
