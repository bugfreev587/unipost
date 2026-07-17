# Media worker operations

The UniPost API binary supports a dedicated Media Worker process for Audio
Overlay and Media object cleanup:

```text
UNIPOST_PROCESS=media-worker
```

The worker uses the same Railway build and start command as the API. It serves
only `GET /health`; public API routes are not mounted. Configure
`MEDIA_PROCESSING_WORKER_DATABASE_MAX_CONNS` to override its database pool
limit (default `8`).

## Deployment A rollout

1. Deploy the new schema and application while API-mode Media workers remain
   enabled (the default).
2. Create the Railway Media Worker service with
   `UNIPOST_PROCESS=media-worker`, the shared API environment, and `/health`
   as its health check.
3. Wait for the worker to become healthy and for every older API instance to
   drain. Confirm Audio Overlay claims are kind-specific.
4. Set `MEDIA_PROCESSING_WORKER_DISABLE_API_PROCESSING=true` on the API
   service and redeploy it.
5. Confirm the dedicated worker remains healthy and Audio Overlay plus hourly
   abandoned-upload/daily retention cleanup continue to run.

If the dedicated worker becomes unhealthy, remove or set
`MEDIA_PROCESSING_WORKER_DISABLE_API_PROCESSING=false` on the API service and
redeploy the API before investigating further.
