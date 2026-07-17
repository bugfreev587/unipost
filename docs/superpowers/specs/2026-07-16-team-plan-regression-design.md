# Team Plan Regression Coverage Design

## Objective

Protect every customer-facing Team Plan entitlement with deterministic automated tests, fix any defects those tests reveal, and verify the complete behavior in staging and production. Production verification must use a dedicated disposable workspace and must not mutate existing customer workspaces.

## Release path

The source branch is `hotfix-team-plan-coverage`, created from the latest `origin/staging`. After local verification, the branch is merged into `staging`, staging deployments are monitored to completion, and the real staging environment is accepted. A `staging` to `main` pull request then promotes the same commit set to production. After production acceptance, the resulting changes are synchronized to `dev`, validated, deployed, and accepted in the development environment.

## Coverage model

Use three complementary layers:

1. Go unit and handler tests verify plan decisions, authorization boundaries, state changes, audit writes, cleanup behavior, and error contracts without browser flakiness.
2. Dashboard source and Playwright tests verify Team-specific feature gating, labels, navigation, enabled actions, and important error states.
3. A reusable environment acceptance runner exercises the deployed API and UI against a dedicated Team workspace. It creates resources with a unique run prefix, records every resource ID, and deletes all temporary data in a `finally` cleanup path.

The deployed acceptance runner must be environment-configurable. Staging uses the staging API and app domains; production uses the production domains. It must reject mismatched domain/environment combinations before making a request.

## Backend coverage

### Team entitlement bundle

Add direct tests for Team plan values returned by the quota and limits surfaces:

- unlimited monthly posts and scheduling capacity;
- unlimited profiles, members, API keys, webhooks, managed accounts, and managed users;
- Inbox, Analytics, X, white-label, Hosted Connect branding, and attribution removal enabled;
- all-platform custom credential capacity;
- Team X Credit allowance and inbound daily allowance.

Tests must distinguish unlimited from fail-open database errors so a missing plan row cannot accidentally masquerade as a valid Team entitlement.

### Profiles and members

Add handler-level tests proving that Team can create profiles and invite members after lower-plan thresholds would have been exceeded. Add finite-plan counterexamples proving the limit is still enforced elsewhere.

Cover member lifecycle and authorization:

- owner and permitted admin invitations;
- invitation acceptance and duplicate/expired invitation handling;
- owner/admin/editor role boundaries;
- role changes, member removal, and ownership transfer;
- prevention of removing or demoting the last owner;
- prevention of self-escalation and cross-workspace access;
- stable error codes for forbidden and conflicting operations.

### Per-member API keys

Test API key creation through the real handler/service path. A key must carry the effective role of its creator, and authorization performed with that key must enforce the same role boundary. Role changes and member removal must not leave a key with privileges that exceed the current membership state. Revoked keys must fail authentication. Keys and memberships from one workspace must never authorize another workspace.

Authentication currently resolves the creator's membership on every request, but incorrectly defaults to Owner when that membership is missing, inactive, or cannot be loaded. This is a critical fail-open privilege escalation: removing an editor can turn the editor's surviving key into an Owner key. Tests must reproduce missing, inactive, and lookup-error states. A key created by a user must authenticate only while that creator has an active membership in the key's workspace; otherwise authentication is denied. Role changes must take effect immediately. Legacy keys without a creator retain their existing compatibility behavior until a separate migration can assign ownership safely.

### Audit log

Add tests for list filtering, pagination/limits, workspace isolation, and Team plan availability. Each covered mutation must assert its audit category, action, actor, target, workspace, and redacted metadata. Secrets, raw API keys, invitation tokens, and platform credentials must never appear in audit metadata.

Audit writes are an explicit best-effort contract in `internal/audit`: logging failures must never roll back or fail the primary mutation. Tests must inject audit-write failures and prove the mutation still succeeds while preventing secrets from being emitted to logs or response bodies.

The pricing contract declares Audit Log as Team-only, while the current endpoint permits every authenticated plan. Add a backend plan gate and matching dashboard gate so Free, API, Basic, and Growth receive the normal upgrade-required experience and Team remains available. The gate must fail closed when the plan cannot be resolved; a database lookup error must not grant Audit Log access.

API key and platform-credential audit action constants already exist but their mutation handlers do not emit events. Add best-effort, secret-redacted events for API key creation/revocation and platform credential creation/deletion. The audit record may contain a key ID, key name, platform, and actor identity, but never the raw key, credential secret, OAuth secret, invite token, or encrypted payload.

### Media retention

The 30-day successful and 60-day failed Team policy values already have direct unit coverage. Do not duplicate those assertions. Extend the cleanup lifecycle coverage to exact eligibility boundaries, all terminal statuses, active statuses, terminal-status transitions, repeated cleanup, object deletion failures, and retry eligibility. Active or in-flight posts must never lose media. Cleanup must be idempotent and must not delete objects belonging to another workspace/post. The current policy and media-usage ledger are the source of truth; obsolete comments in migration 052 do not define runtime behavior.

### Hosted Connect and credentials

Test Team access to every supported custom platform slot, branding updates, attribution hiding, and credential creation/update. Add downgrade tests proving existing configuration is preserved while additions outside the downgraded plan are rejected. Verify credential and OAuth secrets are redacted from responses, logs, and audit metadata.

## Dashboard coverage

Add a Team-specific authenticated Playwright suite driven by dedicated test credentials. It verifies:

- Billing and API Limits display Team and Unlimited consistently;
- Members shows invitation and role-management controls only to authorized roles;
- API Keys can be created/revoked and present role-safe messaging;
- Audit Log renders resulting member/key/config events;
- Analytics and Inbox load real data or a stable empty state without indefinite loading;
- Credentials and Hosted Connect branding controls are enabled for Team;
- forbidden roles see a clear disabled/forbidden state rather than a broken page.

Pricing copy must use one support contract. Team retains the card's published `Priority support` entitlement. Enterprise copy must distinguish `Dedicated support`, SLA, security review, procurement, and capacity planning instead of claiming that ordinary priority support requires Enterprise. A source-level regression test must keep the Team card, Team FAQ, and Enterprise explanation consistent.

The suite must fail rather than skip when explicitly enabled for release acceptance. Generic CI may continue skipping authenticated deployment tests when credentials are absent, but the release workflow must provide the credentials and enforce execution.

## Environment acceptance and cleanup

Staging and production acceptance use a dedicated workspace whose name starts with `codex-team-acceptance-<timestamp>`. The runner records created users, invitations, profiles, API keys, branding values, and audit targets. It performs the following sequence:

1. confirm environment and Team entitlement bundle;
2. create profiles beyond a lower-plan cap;
3. invite admin and editor test identities;
4. verify role allow/deny behavior and ownership safeguards;
5. create role-bound API keys and verify effective permissions;
6. update branding/credentials with non-secret test values;
7. confirm corresponding audit entries and secret redaction;
8. load Team dashboard routes and assert stable rendered states;
9. revoke keys, remove members, delete profiles/configuration, and delete or neutralize the workspace;
10. query by the unique run prefix and fail if removable artifacts remain.

Cleanup runs after both success and failure. Cleanup errors are release blockers and must be reported with the remaining resource IDs.

## Defect handling

Every discovered defect follows a strict red-green cycle:

1. capture a deterministic failing test or acceptance assertion;
2. trace the request through handler, service, database, and UI boundaries to identify the root cause;
3. implement the smallest root-cause fix;
4. rerun the focused test, related package tests, full API tests, Dashboard build, and Dashboard regression suite;
5. retain the regression test permanently.

No production behavior is changed solely to satisfy a mock. Tests use real handlers and stores where practical; fakes are limited to external providers and failure injection.

## Release gates

The change may advance only when all applicable gates are green:

- `GOCACHE=/tmp/unipost-go-build go test ./...` from `api/`;
- `npm run build` from `dashboard/`;
- `npm run test:regression:dashboard` from `dashboard/`;
- Team-specific local tests and red-green evidence for every fix;
- `node scripts/generate-x-credits-catalog.mjs --check` so the JSON catalog and generated Go/TypeScript artifacts cannot drift; published Team allowance tests remain explicit contract assertions;
- code review with all critical and important findings resolved;
- all GitHub, Vercel, Railway, and other triggered staging checks complete successfully;
- staging acceptance completes with cleanup verification;
- `staging` to `main` PR checks complete successfully;
- production acceptance completes with cleanup verification;
- production health remains normal;
- the same fix is synchronized to `dev`, deployed, and accepted.

Before the `staging` to `main` pull request is merged, inspect both the GitHub changed-file list and a local merge-result simulation. The current branches contain patch-equivalent changelog promotion commits with different hashes and production-only CiteLoop content. The release must preserve all production-only files and must not promote unrelated staging drift.

## Out of scope

Priority support delivery is an operational service commitment and cannot be proven by application regression tests. Coverage is limited to keeping its pricing declaration internally consistent and testing any support-routing UI or API that exists. The work will not redesign pricing, introduce feature flags, or alter unrelated Team entitlements unless a failing test proves current behavior contradicts the published contract.
