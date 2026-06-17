# Regression Monitoring

This repo now includes a scheduled regression monitor for the public UniPost API and the released UniPost SDK packages.

## What runs

The GitHub Actions workflow at `/Users/xiaoboyu/unipost/.github/workflows/regression-monitor.yml` runs every hour and on manual dispatch.

It executes six suites:

- `smoke`: `/Users/xiaoboyu/unipost/scripts/smoke-test.sh`
- `sdk-js`: published-package regression via `/Users/xiaoboyu/unipost/scripts/sdk-published-regression/run-suite.sh`
- `sdk-python`: published-package regression via `/Users/xiaoboyu/unipost/scripts/sdk-published-regression/run-suite.sh`
- `sdk-go`: published-package regression via `/Users/xiaoboyu/unipost/scripts/sdk-published-regression/run-suite.sh`
- `sdk-java`: published-package regression via `/Users/xiaoboyu/unipost/scripts/sdk-published-regression/run-suite.sh`
- `ai-provider`: TokenGate provider regression via `/Users/xiaoboyu/unipost/scripts/ai-provider-monitor.sh`

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
- `TOKENGATE_REGRESSION_API_KEY`
  - Required for the `ai-provider` suite.
  - Use a dedicated TokenGate key for synthetic monitoring.

Add these repository variables:

- `UNIPOST_API_BASE_URL`
  - Optional.
  - Defaults to `https://api.unipost.dev` when unset.
- `TOKENGATE_REGRESSION_BASE_URL`
  - Optional.
  - Defaults to `https://gateway.mytokengate.com/v1` when unset.
- `TOKENGATE_REGRESSION_EXPECTED_MODELS`
  - Optional but recommended.
  - Comma-separated model IDs that must be present in TokenGate `/models`.
- `TOKENGATE_REGRESSION_CHAT_MODEL`
  - Required when `AI_PROVIDER_MONITOR_CHAT` is unset or true.
  - The model used for the synthetic `/chat/completions` call.
- `AI_PROVIDER_MONITOR_CHAT`
  - Optional.
  - Defaults to `true`; set to `false` to check only TokenGate `/models`.

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
- TokenGate API availability through `/models`
- TokenGate chat-completions availability when `TOKENGATE_REGRESSION_CHAT_MODEL` is configured

The AI provider monitor intentionally calls TokenGate directly instead of the super-admin UniPost AI provider endpoints. The UniPost admin provider test routes require a Clerk super-admin session, while the public regression monitor is designed to run from GitHub Actions with scoped synthetic secrets.

It does not yet synthetically verify:

- outbound webhook delivery to a real public receiver
- the dashboard `/admin/ai-keys` UI
- the super-admin `/v1/admin/ai-providers/{provider}/test` route
- that `/v1/ai/post-assist` is currently routed to TokenGate instead of falling back to OpenAI or deterministic stub output

Those remaining gaps are intentional for now. Webhook delivery validation needs a stable externally reachable endpoint. UniPost-routed AI validation needs either a safe service credential for super-admin API access or a dedicated backend health endpoint that does not expose provider secrets.
