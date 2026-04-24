# SDK API Coverage Matrix

This matrix tracks UniPost's current public API surface against the local JS, Python, and Go SDKs, plus whether the `scripts/sdk-validation` live tests cover each area.

Legend:

- `Yes` = implemented in SDK or covered by validation
- `Conditional` = live validation runs it only when a safe fixture exists, or accepts a known gated/negative-path outcome
- `No` = not currently implemented / not currently validated
- `Helper` = SDK convenience method, not a 1:1 backend route

## Route coverage

| Public API surface | JS SDK | Python SDK | Go SDK | Live validation |
| --- | --- | --- | --- | --- |
| `GET /v1/platforms/capabilities` | Yes | Yes | Yes | Yes |
| `GET /v1/plans` | Yes | Yes | Yes | Yes |
| `GET /v1/workspace` | Yes | Yes | Yes | Yes |
| `PATCH /v1/workspace` | Yes | Yes | Yes | Yes |
| `GET /v1/profiles` | Yes | Yes | Yes | Yes |
| `POST /v1/profiles` | Yes | Yes | Yes | Yes |
| `GET /v1/profiles/{id}` | Yes | Yes | Yes | Yes |
| `PATCH /v1/profiles/{id}` | Yes | Yes | Yes | Yes |
| `DELETE /v1/profiles/{id}` | Yes | Yes | Yes | Yes |
| `GET /v1/accounts` | Yes | Yes | Yes | Yes |
| `POST /v1/accounts/connect` | Yes | Yes | Yes | Yes, negative path |
| `DELETE /v1/accounts/{id}` | Yes | Yes | Yes | No direct live delete |
| `GET /v1/accounts/{id}/capabilities` | Yes | Yes | Yes | Yes |
| `GET /v1/accounts/{id}/health` | Yes | Yes | Yes | Yes |
| `GET /v1/accounts/{id}/tiktok/creator-info` | Yes | Yes | Yes | Conditional |
| `GET /v1/accounts/{id}/facebook/page-insights` | Yes | Yes | Yes | Conditional |
| `POST /v1/media` | Yes | Yes | Yes | Yes |
| `GET /v1/media/{id}` | Yes | Yes | Yes | Yes |
| `DELETE /v1/media/{id}` | Yes | Yes | Yes | Cleanup path |
| `POST /v1/connect/sessions` | Yes | Yes | Yes | Yes |
| `GET /v1/connect/sessions/{id}` | Yes | Yes | Yes | Yes |
| `POST /v1/workspaces/{workspaceID}/platform-credentials` | Yes | Yes | Yes | Conditional, plan-gated |
| `GET /v1/workspaces/{workspaceID}/platform-credentials` | Yes | Yes | Yes | Conditional, plan-gated |
| `DELETE /v1/workspaces/{workspaceID}/platform-credentials/{platform}` | Yes | Yes | Yes | Conditional, plan-gated |
| `GET /v1/posts` | Yes | Yes | Yes | Yes |
| `POST /v1/posts` | Yes | Yes | Yes | Yes |
| `POST /v1/posts/validate` | Yes | Yes | Yes | Yes |
| `GET /v1/posts/{id}` | Yes | Yes | Yes | Yes |
| `GET /v1/posts/{id}/analytics` | Yes | Yes | Yes | Yes |
| `POST /v1/posts/{id}/archive` | Yes | Yes | Yes | Yes |
| `POST /v1/posts/{id}/restore` | Yes | Yes | Yes | Yes |
| `DELETE /v1/posts/{id}` | Yes | Yes | Yes | Cleanup path |
| `POST /v1/posts/{id}/results/{resultID}/retry` | Yes | Yes | Yes | Conditional |
| `GET /v1/posts/{id}/queue` | Yes | Yes | Yes | Yes |
| `GET /v1/post-delivery-jobs` | Yes | Yes | Yes | Yes |
| `GET /v1/post-delivery-jobs/summary` | Yes | Yes | Yes | Yes |
| `POST /v1/post-delivery-jobs/{jobID}/retry` | Yes | Yes | Yes | Conditional |
| `POST /v1/post-delivery-jobs/{jobID}/cancel` | Yes | Yes | Yes | Conditional |
| `POST /v1/posts/{id}/publish` | Yes | Yes | Yes | Conditional, opt-in |
| `PATCH /v1/posts/{id}` | Yes | Yes | Yes | Yes |
| `POST /v1/posts/{id}/cancel` | Yes | Yes | Yes | Yes |
| `POST /v1/posts/bulk` | Yes | Yes | Yes | Yes |
| `GET /v1/users` | Yes | Yes | Yes | Yes |
| `GET /v1/users/{external_user_id}` | Yes | Yes | Yes | Conditional |
| `POST /v1/posts/{id}/preview-link` | Yes | Yes | Yes | Yes |
| `GET /v1/analytics/summary` | Yes | Yes | Yes | Yes |
| `GET /v1/analytics/trend` | Yes | Yes | Yes | Yes |
| `GET /v1/analytics/by-platform` | Yes | Yes | Yes | Yes |
| `GET /v1/analytics/rollup` | Yes | Yes | Yes | Yes |
| `POST /v1/webhooks` | Yes | Yes | Yes | Yes |
| `GET /v1/webhooks` | Yes | Yes | Yes | Yes |
| `GET /v1/webhooks/{id}` | Yes | Yes | Yes | Yes |
| `PATCH /v1/webhooks/{id}` | Yes | Yes | Yes | Yes |
| `DELETE /v1/webhooks/{id}` | Yes | Yes | Yes | Cleanup path |
| `POST /v1/webhooks/{id}/rotate` | Yes | Yes | Yes | Yes |
| `GET /v1/oauth/connect/{platform}` | Yes | Yes | Yes | Yes, accepts current `unauthorized` backend behavior |
| `GET /v1/usage` | Yes | Yes | Yes | Yes |

## SDK-only helpers

These are convenience methods that exist in the SDK surface but are not 1:1 public backend routes:

| Helper | JS | Python | Go | Validation |
| --- | --- | --- | --- | --- |
| `accounts.get(id)` | Yes | Yes | Yes | Yes |

`accounts.get(id)` is implemented client-side by listing accounts and selecting the matching item because the public API does not currently expose `GET /v1/accounts/{id}`.

## Notes

- `oauth.connect` is implemented in all three SDKs because the route is public, but current backend behavior still appears to require profile context and often returns `unauthorized` on the API-key path. Validation treats that response as a known server-side limitation rather than an SDK defect.
- `facebook/page-insights`, `platform-credentials`, `retryResult`, `deliveryJobs.retry`, `deliveryJobs.cancel`, and live publish are environment-sensitive. Validation covers them conditionally or via safe negative-path checks so the scripts remain runnable in normal workspaces.
