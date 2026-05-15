# Feature Flags and Unleash Rollout

UniPost deploys backend code from Railway and frontend code from Vercel. Merging to `main` deploys code, but feature release is controlled separately through feature flags. The backend remains the final authority for risky behavior such as OAuth scopes, third-party API calls, billing changes, and writes.

## Phase Order

1. Keep `UNIPOST_ENV` set on every Railway API environment and `NEXT_PUBLIC_UNIPOST_ENV` set on Vercel.
2. Route all new backend feature checks through `api/internal/featureflags` instead of reading scattered environment variables.
3. Run with `FEATURE_FLAGS_PROVIDER=env` until the Unleash service is deployed.
4. Deploy Unleash as an independent Railway service backed by its own PostgreSQL database.
5. Use Unleash environments for `development` and `production`. Add `preview` later only if the Unleash edition and release process need a separate preview target.
6. Switch the API to `FEATURE_FLAGS_PROVIDER=unleash` environment by environment.
7. Add a read-only UniPost admin status page at `/admin/features` after backend and frontend checks both use the shared API surface.

## Railway Unleash Service

Use the official Railway Unleash template or create a new Railway service from the official Docker image:

```text
unleashorg/unleash-server:latest
```

Provision a dedicated Railway PostgreSQL database for Unleash. Do not reuse the UniPost application database. The Unleash service needs:

```text
DATABASE_HOST=${{Postgres.PGHOST}}
DATABASE_PORT=${{Postgres.PGPORT}}
DATABASE_NAME=${{Postgres.PGDATABASE}}
DATABASE_USERNAME=${{Postgres.PGUSER}}
DATABASE_PASSWORD=${{Postgres.PGPASSWORD}}
DATABASE_SSL=true
DATABASE_SSL_REJECT_UNAUTHORIZED=false
UNLEASH_URL=https://flags.unipost.dev
PORT=4242
LOG_LEVEL=info
```

After the first login, immediately rotate the default admin password in the Unleash UI.

## UniPost API Configuration

Start with the env provider:

```text
FEATURE_FLAGS_PROVIDER=env
FEATURE_TIKTOK_ANALYTICS_SCOPES=false
```

After Unleash is live and the backend token is created:

```text
FEATURE_FLAGS_PROVIDER=unleash
UNLEASH_URL=https://flags.unipost.dev/api
UNLEASH_SERVER_TOKEN=<backend-token>
UNLEASH_APP_NAME=unipost-api
UNLEASH_ENVIRONMENT=production
```

Backend SDKs evaluate flags locally from cached definitions. If Unleash is unreachable at startup, UniPost falls back to the env provider rather than blocking API boot.

## Frontend Contract

The dashboard should not connect to Unleash directly in the first rollout. It should call:

```text
GET /v1/me/features
```

The response is intentionally simple:

```json
{
  "data": {
    "environment": "production",
    "provider": "unleash",
    "flags": {
      "tiktok.analytics_scopes": false
    }
  }
}
```

The browser can use this to show or hide UI affordances. The backend must still enforce any sensitive behavior in the API handler or platform adapter that performs the action.

## Initial Flags

Create this flag in Unleash:

```text
tiktok.analytics_scopes
```

Recommended defaults:

```text
development: on
production: off
fallback: off in production
```

This flag controls whether TikTok OAuth requests include:

```text
user.info.profile
user.info.stats
video.list
```

Production should stay off until TikTok approves those scopes for the production app. The emergency rollback is to disable `tiktok.analytics_scopes` in the production environment.

## Admin Status Page

UniPost admins can inspect evaluated flag state from:

```text
/admin/features
```

This page calls the backend `GET /v1/me/features` endpoint and never connects to Unleash directly. It shows the runtime environment, active provider, enabled flag count, and the current value for each registered UniPost flag. The page is read-only by design; operational changes still happen in Unleash so emergency rollback remains a single production flag toggle.
