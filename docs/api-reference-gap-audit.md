# API Reference Gap Audit

Last reviewed: May 26, 2026

This audit compares the public API routes registered in [api/cmd/api/main.go](/Users/xiaoboyu/unipost/api/cmd/api/main.go) against the dedicated endpoint pages currently present under [dashboard/src/app/docs/api](/Users/xiaoboyu/unipost/dashboard/src/app/docs/api).

## Newly covered in this pass

- `GET /v1/profiles`
- `POST /v1/profiles`
- `GET /v1/profiles/{id}`
- `PATCH /v1/profiles/{id}`
- `DELETE /v1/profiles/{id}`
- `POST /v1/accounts/connect`
- `DELETE /v1/accounts/{id}`
- `GET /v1/accounts/{id}/capabilities`
- `GET /v1/accounts/{id}/tiktok/creator-info`
- `GET /v1/analytics/posts`
- `GET /v1/analytics/posts/export`
- `GET /v1/analytics/rollup`
- `GET /v1/analytics/platforms`
- `GET /v1/analytics/platforms/{platform}`
- `POST /v1/analytics/refresh`

## Still missing dedicated API Reference pages

### Core and discovery

- `GET /v1/workspace`
- `PATCH /v1/workspace`
- `GET /v1/platforms/capabilities`
- `GET /v1/plans`
- `GET /v1/usage`

### Media

- `DELETE /v1/media/{id}`

### Publishing

- `DELETE /v1/posts/{id}`
- `POST /v1/posts/{id}/results/{resultID}/retry`
- `GET /v1/posts/{id}/queue`
- `POST /v1/posts/{id}/preview-link`

### Delivery jobs

- `GET /v1/post-delivery-jobs`
- `GET /v1/post-delivery-jobs/summary`
- `POST /v1/post-delivery-jobs/{jobID}/retry`
- `POST /v1/post-delivery-jobs/{jobID}/cancel`

### Analytics

- `GET /v1/analytics/trend`
- `GET /v1/analytics/by-platform`

### Webhooks

- `DELETE /v1/webhooks/{id}` does not have a dedicated page yet. The route exists, but the current docs only cover create, list, get, update, and rotate.

## Intentional exclusions

- Legacy aliases such as `POST /v1/posts/{id}/archive`, `POST /v1/posts/{id}/restore`, `POST /v1/posts/{id}/cancel`, and `POST /v1/post-delivery-jobs/{jobID}/retry-now` are intentionally excluded from the missing-pages list because the canonical docs now point users to `PATCH /v1/posts/{id}` and `POST /v1/post-delivery-jobs/{jobID}/retry`.
- Dashboard-only routes such as `/v1/dashboard/profiles`, `/v1/profiles/{profileID}/social-accounts`, and `/v1/workspaces/{workspaceID}/...` are also excluded because the API Reference targets the public API-key surface, not the internal dashboard session API.
