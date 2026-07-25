# Dashboard Synthetic Authentication Design

## Problem

The deployed Dashboard regression has one authenticated smoke test, but it only runs when `DASHBOARD_TEST_EMAIL` and `DASHBOARD_TEST_PASSWORD` are set. UniPost enables Google OAuth for normal Clerk signup and sign-in, so there is no product-managed password to supply. The required smoke test is therefore skipped, which is a failed release gate under the repository rules.

## Decision

Use a disposable Clerk user and Clerk's official Playwright test helper for each authenticated regression run. The workflow will hold one environment-matched Clerk secret key. It will not store a user email or password, automate Google OAuth, or enable password login in the product.

Alternatives considered:

- A permanent email/password account would be easy to understand but would add a long-lived password, exercise a login method customers do not use, and require account maintenance.
- Google OAuth browser automation would match the customer entry point but would be unstable under MFA, consent, CAPTCHA, and provider risk controls.
- A disposable Clerk user avoids both problems and follows the synthetic-session pattern already used by `dashboard/scripts/team-plan-acceptance.mjs`.

## Scope

This change affects Dashboard regression test support, the deployed regression workflow, CI documentation, and the existing account-deletion handler used for deterministic synthetic cleanup. It does not change the UniPost product authentication configuration, customer sign-in behavior, or feature flags. The account-deletion adjustment makes the existing deletion contract synchronous instead of depending solely on an asynchronous Clerk webhook.

## Components

### Synthetic identity helper

A focused helper will own the complete lifecycle:

1. Validate the configured base URL and Clerk secret-key type so a test key cannot be used against production and a live key cannot be used against development.
2. Read the environment-matched Clerk publishable key from an explicit public CI variable; do not scrape deployment HTML.
3. Create a uniquely named passwordless Clerk user through Clerk's Backend API using `skip_password_requirement: true`. No generated password is needed or stored.
4. Initialize `@clerk/testing/playwright` with the environment's publishable and secret keys. Because the email overload reads `CLERK_SECRET_KEY`, set it only for the helper's lifetime and restore the process environment in `finally`.
5. Establish a browser session with the supported `clerk.signIn({ page, emailAddress })` overload. In `@clerk/testing` 2.2.10 this overload resolves the user through Clerk's Backend API, creates a 300-second sign-in token, and activates the session with Clerk's `ticket` strategy. The test never submits a password or drives the Google OAuth UI.
6. Return the disposable user identity to the caller for cleanup.

The helper will expose small functions for configuration validation, user creation, browser sign-in, and cleanup so their contracts can be tested without making live requests.

### Authenticated regression

The authenticated smoke test will require `DASHBOARD_TEST_CLERK_SECRET_KEY` instead of email and password. It will create the synthetic identity in setup, sign in through Clerk's ticket-based testing helper, read the active browser session token, call `GET /v1/me/bootstrap` explicitly, resolve the default profile, run the existing route assertions, and delete the account in teardown. Profile discovery may retry only after the explicit bootstrap call and only within a bounded timeout.

The suite must not silently skip when the Clerk key is absent. Missing required authentication configuration will fail the authenticated smoke with a direct setup error.

### GitHub Actions

`.github/workflows/dashboard-regression.yml` will pass only `DASHBOARD_TEST_CLERK_SECRET_KEY` for authentication. `DASHBOARD_TEST_EMAIL` and `DASHBOARD_TEST_PASSWORD` will be removed. `DASHBOARD_TEST_PROFILE_ID` may remain as an optional override, but the normal path will discover the synthetic user's default profile.

The GitHub secret must contain the Clerk secret key matching `DASHBOARD_BASE_URL`:

- `https://dev-app.unipost.dev` and `https://staging-app.unipost.dev` require `sk_test_`.
- `https://app.unipost.dev` requires `sk_live_`.

The scheduled workflow currently defaults to `https://app.unipost.dev`, so it deliberately creates a reserved synthetic identity in the production Clerk instance and production database. The identity is never confused with a customer account because its email and name use the `codex-dashboard-regression-` prefix, and the run is successful only when its Clerk and UniPost records are deleted. Development, staging, and Preview runs use the matching Development Clerk instance and their own API/database environment.

### Deterministic UniPost cleanup

The existing authenticated `DELETE /v1/me` endpoint will keep deleting the current Clerk user, then synchronously call the existing `DeleteUser` database query before returning `204`. The database deletion cascades through the user's workspace, profiles, subscriptions, accounts, keys, and posts through the existing foreign keys. The later `user.deleted` webhook remains a safe idempotent fallback, but the test no longer depends on that webhook being configured or delivered.

This is required for isolated Railway Preview environments, where `CLERK_WEBHOOK_SECRET` is intentionally absent. It also strengthens normal account deletion: a successful `204` now confirms that both Clerk and UniPost deletion completed.

## Cleanup and Failure Handling

Cleanup runs in `finally` semantics after success, assertion failure, navigation failure, or partial setup. When a browser session and UniPost user exist, teardown calls authenticated `DELETE /v1/me`; a `204` confirms synchronous Clerk and UniPost cleanup. If setup failed before UniPost bootstrap, teardown calls Clerk's Backend API directly for the exact tracked user. If authenticated deletion fails after bootstrap, the direct Clerk delete remains an idempotent fallback, but the run stays failed and reports the tracked user ID because local cleanup was not confirmed. No failed cleanup may be reported as successful.

If both the acceptance and cleanup fail, the result preserves both errors. Secret keys and sign-in tickets remain in memory only and are never logged, committed, stored as artifacts, or written to the browser report.

If Clerk user creation succeeds but UniPost bootstrap has not completed yet, the test will retry only the profile discovery within a bounded timeout. It will not reuse a customer profile or select an arbitrary existing user.

## Testing

The implementation will add source-level/unit coverage that proves:

- missing or environment-mismatched Clerk configuration fails clearly;
- generated identities use a reserved synthetic prefix and explicitly omit a password;
- the Backend API request creates and deletes only the tracked synthetic user;
- Clerk browser sign-in uses the supported email overload, which creates a short-lived sign-in ticket and receives no password;
- the test explicitly bootstraps UniPost before resolving a profile;
- `DELETE /v1/me` synchronously deletes the local user record after Clerk deletion, while repeat webhook deletion remains harmless;
- cleanup runs after an acceptance failure and cleanup failures are surfaced;
- the workflow no longer references `DASHBOARD_TEST_EMAIL` or `DASHBOARD_TEST_PASSWORD`;
- the authenticated Playwright suite no longer contains a credential-based skip.

Local verification will include the focused unit tests, Dashboard build, and the complete Dashboard regression command. Deployed verification will run on the exact Draft PR head SHA in its isolated Preview environment before any merge to `dev`.

## Acceptance Criteria

- No Dashboard regression path depends on a Google account password or Google UI automation.
- The authenticated smoke is a required test and cannot report skipped because credentials are absent.
- Each run uses only its own disposable Clerk user and a short-lived ticket, then synchronously removes its Clerk and UniPost records.
- Preview cleanup does not depend on Clerk webhook delivery.
- Secrets and sign-in tickets never appear in logs or artifacts.
- The existing authenticated route coverage still verifies real Clerk session behavior against the deployed Dashboard and API.
