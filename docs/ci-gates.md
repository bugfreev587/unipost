# CI gates

UniPost uses two layers of CI so development stays fast while production remains protected.

## Pull request gate

`.github/workflows/ci.yml` runs on:

- pull requests targeting `dev`
- pull requests targeting `main`
- pushes to `dev`
- pushes to `main`
- manual `workflow_dispatch`

The required checks should be:

- `API tests`
- `Dashboard build`

This covers the minimum production-safety bar before merging:

- backend compile and Go tests with `go test ./...`
- dashboard dependency install and production `next build`

Dashboard lint is intentionally not a required check yet because the existing codebase currently has lint errors. Once those are cleaned up, add `npm run lint` to the dashboard job and make it required.

## Deployed regression monitor

`.github/workflows/regression-monitor.yml` and `.github/workflows/dashboard-regression.yml` are deployed-environment monitors. They are useful for catching broken production or dev deployments, but they should not be the only PR gate because they depend on external accounts, live secrets, and third-party services.

Configure these GitHub repository secrets and variables for deployed regression:

- `UNIPOST_REGRESSION_API_KEY`
- `REGRESSION_TEST_ACCOUNT_ID`
- `REGRESSION_ALERT_WEBHOOK_URL`
- `UNIPOST_API_BASE_URL`
- `DASHBOARD_BASE_URL`
- `DASHBOARD_TEST_EMAIL`
- `DASHBOARD_TEST_PASSWORD`
- `DASHBOARD_TEST_PROFILE_ID`

## Branch protection

Recommended GitHub branch protection:

- For `dev`, require PRs from `dev-*` branches and require `API tests` plus `Dashboard build`.
- For `main`, require PRs from `dev` and require the same checks.
- Require branches to be up to date before merging when GitHub offers the option.
- Keep direct pushes to `main` disabled except for emergency admin override.
