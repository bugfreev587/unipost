# UniPost integration: common root causes and fixes

Each row maps a `doctor diagnose` finding to a fix. The CLI provides the
evidence and the recommended action; you apply the repository-specific edit.

## Auth and environment

| Finding | Likely cause | Fix |
| --- | --- | --- |
| `finding_api_key_missing` | `UNIPOST_API_KEY` not set | Set the env var or run `unipost auth login`. |
| `finding_api_key_prefix_unknown` | Key is not a UniPost key | Use a key starting with `up_live_` or `up_test_`. |
| `finding_auth_unauthorized` | Missing `Bearer` prefix, wrong/revoked key | Send `Authorization: Bearer <UNIPOST_API_KEY>`; confirm the key is active. |
| `finding_live_key_non_production_base` | Live key against dev/staging base URL | Match key to base URL (`up_live_` → `https://api.unipost.dev`). |
| `finding_test_key_production_base` | Test key against production | Use a live key for production, or point `--base-url` at dev/staging. |
| `finding_api_unreachable` | Network/base-URL problem | Check connectivity and the configured base URL. |

## Workspace, profile, account

| Finding | Likely cause | Fix |
| --- | --- | --- |
| `finding_no_profile` | Workspace has no profile | Create a profile in the dashboard. |
| `finding_no_connected_account` | No social account connected | Connect an account in the dashboard, then re-verify. |
| `finding_account_unhealthy` | Token expired / disconnected | Reconnect or refresh the account in the dashboard. |

## Logs and request correlation

| Finding | What to do |
| --- | --- |
| `finding_recent_failed_requests` | Inspect with `unipost doctor explain --request-id <id> --json` or `unipost logs get <id> --json`. |

## Verifying a fix

Always run `unipost doctor verify --json` after a patch. It performs only
non-destructive checks (auth probe, workspace read, account/health read, logs
read). It never publishes. `data.status: "passed"` confirms the fix.
