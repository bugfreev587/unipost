# Regression Monitoring

This repo now includes a scheduled regression monitor for the public UniPost API and the released UniPost SDK packages.

## What runs

The GitHub Actions workflow at `/Users/xiaoboyu/unipost/.github/workflows/regression-monitor.yml` runs every hour and on manual dispatch.

It executes four suites:

- `smoke`: `/Users/xiaoboyu/unipost/scripts/smoke-test.sh`
- `sdk-js`: published-package regression via `/Users/xiaoboyu/unipost/scripts/sdk-published-regression/run-suite.sh`
- `sdk-python`: published-package regression via `/Users/xiaoboyu/unipost/scripts/sdk-published-regression/run-suite.sh`
- `sdk-go`: published-package regression via `/Users/xiaoboyu/unipost/scripts/sdk-published-regression/run-suite.sh`

The source-SDK validation flow is separate and lives under `/Users/xiaoboyu/unipost/scripts/sdk-source-validation/run-suite.sh`. That flow validates unreleased SDK source from `/Users/xiaoboyu/unipost-dev` and should be used before tagging new SDK releases.

Each regression suite is wrapped by `/Users/xiaoboyu/unipost/scripts/regression/run-suite.sh`, which standardizes env vars and writes a dedicated log file into `artifacts/regression/`.

If any suite fails, the workflow sends a webhook alert through `/Users/xiaoboyu/unipost/scripts/regression/send-alert.sh`.

## Required GitHub configuration

Add these repository secrets:

- `UNIPOST_REGRESSION_API_KEY`
  - Required.
  - Use a real API key for a stable workspace dedicated to regression testing.
- `REGRESSION_TEST_ACCOUNT_ID`
  - Optional but strongly recommended.
  - Enables account-specific flows inside the smoke test and SDK suites.
- `REGRESSION_ALERT_WEBHOOK_URL`
  - Optional but strongly recommended.
  - Supports Slack incoming webhooks and Discord webhooks.

Add this repository variable if you want to test a non-default environment:

- `UNIPOST_API_BASE_URL`
  - Optional.
  - Defaults to `https://api.unipost.dev` when unset.

## Recommended test workspace setup

Use a dedicated workspace with:

- at least one healthy connected social account
- a stable API key that is not reused for customer traffic
- safe test media and draft content
- a known `social_account_id` stored in `REGRESSION_TEST_ACCOUNT_ID`

This keeps the hourly checks deterministic and makes failures easier to diagnose.

## Coverage and current limits

The hourly monitor currently verifies:

- public REST API smoke coverage
- released JavaScript, Python, Go, and Java SDK compatibility against the live API
- whether the packages customers actually install still work against production

It does not yet synthetically verify outbound webhook delivery to a real public receiver.

That remaining gap is intentional for now because delivery validation needs a stable externally reachable endpoint. If you want to close it later, the clean next step is a dedicated synthetic receiver plus a fifth regression suite that asserts real event delivery and signature validation end to end.
