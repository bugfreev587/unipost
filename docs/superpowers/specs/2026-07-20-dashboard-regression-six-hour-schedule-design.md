# Dashboard Regression Six-Hour Schedule Design

## Goal

Reduce the scheduled production Dashboard Regression from once per hour to once every six hours, while preserving the existing manual trigger and regression behavior.

## Schedule

Change the GitHub Actions cron expression in `.github/workflows/dashboard-regression.yml` from:

```text
15 * * * *
```

to:

```text
15 */6 * * *
```

The scheduled runs therefore occur daily at `00:15`, `06:15`, `12:15`, and `18:15` UTC. GitHub Actions may start a scheduled run later than its nominal time when runners are delayed.

## Scope

Only the cron expression changes. The following behavior remains unchanged:

- `workflow_dispatch` continues to permit manual runs.
- The regression suite and Playwright configuration remain unchanged.
- `DASHBOARD_BASE_URL` continues to target the existing configured environment.
- Failure notification and artifact upload behavior remain unchanged.

## Verification

- Parse the workflow as valid YAML.
- Assert that the configured schedule is exactly `15 */6 * * *`.
- Confirm the diff contains no unrelated workflow or application changes.
