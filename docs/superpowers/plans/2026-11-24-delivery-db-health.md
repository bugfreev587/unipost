# Delivery database resilience and account health hotfix

Goal: bound database connection/queue operations, expose pending age, and stop cancelled/historical failures from misrepresenting account health.

Base: origin/staging (993a0ce28f048cad0abdca932c100a57c8407419). This chat exclusively owns hotfix-delivery-db-health in this worktree.

Design: configure pgx connection establishment to take at most 5 seconds. Bound dispatch/retry recovery and claim operations independently to 10 seconds, without attaching those deadlines to provider execution or leases. Worker /health probes Postgres with a 2-second deadline and returns 503 on failure. Add workspace-scoped oldest_pending_at and oldest_pending_age_ms to the existing queue summary; future retries do not count as ready pending work. Derive account status from the latest non-cancelled terminal result, using the delivery completion time with post creation fallback; preserve last successful timestamp, clear historical last_error after success, return an error rather than false healthy on query failure. No schema migration or feature flag.

- [x] Add regression tests for connection timeout, bounded worker database calls, readiness failure/recovery, pending age SQL, and account cancellation/success/failure ordering.
- [x] Observe expected test failures, then implement minimal changes and regenerate sqlc output.
- [x] Run API CI-equivalent unit/coverage tests and relevant disposable PostgreSQL integration tests.
- [ ] Review exact commits/files, push only owned hotfix branch, open PR to staging, monitor every triggered check. Preview Acceptance is explicitly waived by the user; cancel the automatic Preview run.
- [ ] Merge only with successful checks; monitor staging API/worker/MCP and Vercel deploys; browser/API acceptance on staging.
- [ ] Audit staging-to-main unique commits/files, promote with PR and checks, monitor production deploys and browser/API acceptance.
- [ ] Sync owned hotfix with origin/dev after production verification; Preview Acceptance is waived by the user for this hotfix. Stop on conflicts.

Limits: this hardens failure handling and diagnostics; it does not claim to fix an unproven Railway networking defect. Never publish to customer accounts during acceptance.
