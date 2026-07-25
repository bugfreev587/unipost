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

This change affects only Dashboard regression test support, the deployed regression workflow, and CI documentation. It does not change the UniPost product authentication configuration, customer accounts, production feature behavior, or feature flags.

## Components

### Synthetic identity helper

A focused helper will own the complete lifecycle:

1. Validate the configured base URL and Clerk secret-key type so a test key cannot be used against production and a live key cannot be used against development.
2. Discover the deployed Clerk publishable key from a public page.
3. Create a uniquely named Clerk user with a generated email and random strong password through Clerk's Backend API.
4. Initialize `@clerk/testing/playwright` with the environment's publishable and secret keys.
5. Establish a browser session with `clerk.signIn({ page, emailAddress })`, bypassing the disabled password UI without bypassing Clerk authentication.
6. Return the disposable user identity to the caller for cleanup.

The helper will expose small functions for configuration validation, user creation, browser sign-in, and cleanup so their contracts can be tested without making live requests.

### Authenticated regression

The authenticated smoke test will require `DASHBOARD_TEST_CLERK_SECRET_KEY` instead of email and password. It will create the synthetic identity in setup, sign in through Clerk's testing helper, wait for UniPost bootstrap to produce the default workspace/profile, run the existing route assertions, and delete the Clerk user in teardown.

The suite must not silently skip when the Clerk key is absent. Missing required authentication configuration will fail the authenticated smoke with a direct setup error.

### GitHub Actions

`.github/workflows/dashboard-regression.yml` will pass only `DASHBOARD_TEST_CLERK_SECRET_KEY` for authentication. `DASHBOARD_TEST_EMAIL` and `DASHBOARD_TEST_PASSWORD` will be removed. `DASHBOARD_TEST_PROFILE_ID` may remain as an optional override, but the normal path will discover the synthetic user's default profile.

The GitHub secret must contain the Clerk secret key matching `DASHBOARD_BASE_URL`:

- `https://dev-app.unipost.dev` and `https://staging-app.unipost.dev` require `sk_test_`.
- `https://app.unipost.dev` requires `sk_live_`.

## Cleanup and Failure Handling

Cleanup runs in `finally` semantics after success, assertion failure, navigation failure, or partial setup. A created Clerk user is always deleted. A cleanup failure fails the run and reports the Clerk user ID without printing the secret key or generated password.

If both the acceptance and cleanup fail, the result preserves both errors. Generated credentials remain in memory only and are never logged, committed, stored as artifacts, or written to the browser report.

If Clerk user creation succeeds but UniPost bootstrap has not completed yet, the test will retry only the profile discovery within a bounded timeout. It will not reuse a customer profile or select an arbitrary existing user.

## Testing

The implementation will add source-level/unit coverage that proves:

- missing or environment-mismatched Clerk configuration fails clearly;
- generated identities use a reserved synthetic prefix and random password;
- the Backend API request creates and deletes only the tracked synthetic user;
- Clerk browser sign-in receives the generated email and no password;
- cleanup runs after an acceptance failure and cleanup failures are surfaced;
- the workflow no longer references `DASHBOARD_TEST_EMAIL` or `DASHBOARD_TEST_PASSWORD`;
- the authenticated Playwright suite no longer contains a credential-based skip.

Local verification will include the focused unit tests, Dashboard build, and the complete Dashboard regression command. Deployed verification will run on the exact Draft PR head SHA in its isolated Preview environment before any merge to `dev`.

## Acceptance Criteria

- No Dashboard regression path depends on a Google account password or Google UI automation.
- The authenticated smoke is a required test and cannot report skipped because credentials are absent.
- Each run uses only its own disposable Clerk user and removes it after the run.
- Secrets and generated passwords never appear in logs or artifacts.
- The existing authenticated route coverage still verifies real Clerk session behavior against the deployed Dashboard and API.
