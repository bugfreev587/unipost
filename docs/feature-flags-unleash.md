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
      "tiktok.analytics_scopes": false,
      "inbox": true
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

It also controls the dashboard TikTok platform analytics surface under `Analytics -> Platforms -> TikTok` and the backend endpoints that fetch TikTok profile, account metrics, and public video inventory. Production should stay off until TikTok approves those scopes for the production app. The emergency rollback is to disable `tiktok.analytics_scopes` in the production environment.

Hosted Connect Sessions are enabled directly in development and production. They are not gated by Unleash; platform readiness is handled by configured OAuth credentials, provider approval, and normal upstream failure handling.

Create this flag in Unleash:

```text
attribution.utm_signup_binding_v1
```

Recommended defaults:

```text
development: on
production: off
fallback: off in production
```

Owner area: growth analytics / Admin. This flag controls lightweight UTM capture on landing visits and authenticated binding from a landing `session_id` to the signed-in user. When disabled, UniPost keeps the existing `r` / referrer landing-source tracking behavior and skips new session-user binding writes. Production can roll back by disabling `attribution.utm_signup_binding_v1`; existing attribution rows remain readable, but no new UTM JSONB values or bindings are recorded while the flag is off.

Create this flag in Unleash:

```text
inbox
```

Recommended defaults:

```text
development: on
production: on
fallback: on
```

Owner area: Inbox / Dashboard. This flag controls the UniPost Inbox surface, including dashboard navigation, unread-count polling, the Inbox WebSocket, and `/v1/inbox/*` API routes. Inbox is already a supported product surface for Instagram comments and DMs, Threads comments, and YouTube comments; the flag exists for rollout control and emergency shutdown rather than for a new hidden feature. Production rollback is to disable `inbox` in the production environment, which hides the dashboard entry point and blocks the backend Inbox API while preserving stored inbox data.

Create this flag in Unleash:

```text
email.loops_integration_v1
```

Recommended defaults:

```text
development: on
production: off
fallback: off in production
```

Owner area: Growth lifecycle / Backend API. This flag controls Loops contact sync, welcome lifecycle events, and transactional lifecycle notifications for UniPost dashboard users. When enabled and `LOOPS_API_KEY` is configured, Clerk `user.created` and `user.updated` webhooks upsert dashboard users into Loops contacts, and `user.created` emits a `user_signed_up` event. Plan changes, account cancellation, and post failures use Loops transactional emails when their template IDs are configured with `LOOPS_PLAN_CHANGED_TRANSACTIONAL_ID`, `LOOPS_ACCOUNT_CANCELED_TRANSACTIONAL_ID`, and `LOOPS_POST_FAILED_TRANSACTIONAL_ID`; if a template ID is missing, the backend falls back to the matching Loops event. Production rollback is to disable `email.loops_integration_v1`; the backend then keeps normal signup/webhook behavior but stops making outbound Loops calls. Third-party dependency: Loops API availability and account configuration.

Create this flag in Unleash:

```text
billing.free_plan_hard_post_quota
```

Recommended defaults:

```text
development: on
production: off
fallback: off in production
```

Owner area: Billing / Publishing API. This flag controls whether Free plan workspaces are hard-blocked from creating new publish requests once the request would exceed the monthly post quota. Paid plans deliberately keep soft-overage behavior, with usage warnings and upgrade guidance rather than immediate interruption. There is no third-party approval dependency. Production rollback is to disable `billing.free_plan_hard_post_quota`; Free workspaces then return to the historical soft-overage behavior while the dashboard and pricing copy can be updated independently if needed.

## Admin Status Page

UniPost admins can inspect evaluated flag state from:

```text
/admin/features
```

This page calls the backend `GET /v1/me/features` endpoint and never connects to Unleash directly. It shows the runtime environment, active provider, enabled flag count, and the current value for each registered UniPost flag. The page is read-only by design; operational changes still happen in Unleash so emergency rollback remains a single production flag toggle.
