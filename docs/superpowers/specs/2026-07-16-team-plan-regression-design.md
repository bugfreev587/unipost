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

If the implementation currently snapshots a role into the key, tests must expose whether that snapshot becomes stale. The safe behavior is to intersect key scope with the creator's current active membership rather than preserve elevated access after demotion or removal.

### Audit log

Add tests for list filtering, pagination/limits, workspace isolation, and Team plan availability. Each covered mutation must assert its audit category, action, actor, target, workspace, and redacted metadata. Secrets, raw API keys, invitation tokens, and platform credentials must never appear in audit metadata.

Audit write failure behavior must be explicit: the primary mutation may succeed only when the existing product contract treats audit as best-effort; otherwise the mutation must fail atomically. Tests will document and preserve the chosen existing contract rather than silently changing it.

### Media retention

Extend retention tests to cover exact 30-day and 60-day boundaries, all terminal statuses, active statuses, terminal-status transitions, repeated cleanup, object deletion failures, and retry eligibility. Active or in-flight posts must never lose media. Cleanup must be idempotent and must not delete objects belonging to another workspace/post.

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
- code review with all critical and important findings resolved;
- all GitHub, Vercel, Railway, and other triggered staging checks complete successfully;
- staging acceptance completes with cleanup verification;
- `staging` to `main` PR checks complete successfully;
- production acceptance completes with cleanup verification;
- production health remains normal;
- the same fix is synchronized to `dev`, deployed, and accepted.

## Out of scope

Priority support is an operational service commitment and cannot be proven by application regression tests. Coverage is limited to confirming the pricing declaration and any support-routing UI or API that exists. The work will not redesign pricing, introduce feature flags, or alter Team entitlements unless a failing test proves current behavior contradicts the published contract.
