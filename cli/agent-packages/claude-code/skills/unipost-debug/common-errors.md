# UniPost integration: common root causes and fixes

Each row maps a `doctor diagnose` finding to a fix. The CLI provides the
evidence and the recommended action; you apply the repository-specific edit.

## Auth and environment

| Finding | Likely cause | Fix |
| --- | --- | --- |
| `finding_api_key_missing` | `UNIPOST_API_KEY` not set | Set the env var or run `unipost auth login`. |
| `finding_api_key_prefix_unknown` | Key is not a UniPost key | Use a key starting with `up_live_` or `up_test_`. |
| `finding_auth_unauthorized` | Missing `Bearer` prefix, wrong/revoked key | Send `Authorization: Bearer <UNIPOST_API_KEY>`; confirm the key is active. |
| `finding_local_auth_header_missing_bearer` | Local code sends the raw API key or `X-API-Key` | Patch the file in `recommended_actions[].target_files` to send `Authorization: Bearer <UNIPOST_API_KEY>`. |
| `finding_local_hardcoded_api_key` | A source/example file contains a literal UniPost key | Move the key to a local secret or env var, rotate it if committed, and keep examples as `UNIPOST_API_KEY=` only. |
| `finding_local_env_var_name_mismatch` | Local examples use `UNIPOST_KEY` instead of `UNIPOST_API_KEY` | Rename the documented env var or map app config consistently. Ask before changing a real `.env`. |
| `finding_live_key_non_production_base` | Live key against dev/staging base URL | Match key to base URL (`up_live_` → `https://api.unipost.dev`). |
| `finding_test_key_production_base` | Test key against production | Use a live key for production, or point `--base-url` at dev/staging. |
| `finding_api_unreachable` | Network/base-URL problem | Check connectivity and the configured base URL. |

## Workspace, profile, account

| Finding | Likely cause | Fix |
| --- | --- | --- |
| `finding_no_profile` | Workspace has no profile | Create a profile in the dashboard. |
| `finding_no_connected_account` | No social account connected | Connect an account in the dashboard, then re-verify. |
| `finding_account_unhealthy` | Token expired / disconnected | Reconnect or refresh the account in the dashboard. |

## Payload and media

| Finding | Likely cause | Fix |
| --- | --- | --- |
| `finding_local_payload_shape` | Local request body uses singular `account_id` or local media paths | Use `account_ids: [<account_id>]`; upload local media first or pass public media URLs. Run `unipost posts validate --json`. |

## Logs and request correlation

| Finding | What to do |
| --- | --- |
| `finding_recent_failed_requests` | Inspect with `unipost doctor explain --request-id <id> --json` or `unipost logs get <id> --json`. |

## Webhooks

| Finding | Likely cause | Fix |
| --- | --- | --- |
| `finding_local_webhook_raw_body_risk` | Webhook route parses JSON before verifying the UniPost signature | Verify against the raw body first, or mount a raw-body parser only for the webhook route. |

## Verifying a fix

Always run `unipost doctor verify --json` after a patch. It performs only
non-destructive checks (auth probe, workspace read, account/health read, logs
read). It never publishes. `data.status: "passed"` confirms the fix.
