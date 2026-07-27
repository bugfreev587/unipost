# Scheduled idempotency index rollout

The `quota_hold` idempotency change must ship in two deployments. Do not widen
the partial unique index during the rolling deployment that introduces the new
lookup behavior.

## Phase A — application compatibility

PR #270 ships only the application-side compatibility layer:

- scheduled idempotency lookup includes both `scheduled` and `quota_hold`;
- keyed scheduled creates acquire the workspace/key transaction advisory lock,
  then re-check replay or conflict before quota and active-scheduled gates;
- migration 071 remains authoritative, so
  `social_posts_workspace_scheduled_idempotency_uniq` continues to cover only
  `status = 'scheduled'`;
- the required schema version remains 126.

This ordering keeps old instances compatible during Railway's rolling deploy.
An old instance still uses its historical scheduled-only lookup and index; the
predeploy step does not introduce a new `23505` path that the old instance
cannot replay.

## Phase B — index widening after Phase A is universal

Create a separate pull request and migration only after both staging and
production confirm that every API instance is running the Phase A binary and
no old binary remains as a rollback target.

Before applying Phase B in each environment:

1. Follow the Railway pre-migration backup gate and retain the backup evidence.
2. Audit active keys with:

   ```sql
   SELECT workspace_id, idempotency_key, COUNT(*)
   FROM social_posts
   WHERE idempotency_key IS NOT NULL
     AND status IN ('scheduled', 'quota_hold')
     AND deleted_at IS NULL
   GROUP BY workspace_id, idempotency_key
   HAVING COUNT(*) > 1;
   ```

3. If the audit returns any row, stop. Do not delete, rewrite, or choose a
   winner automatically; resolve each duplicate with an explicit product and
   operations decision.
4. If the audit is empty, apply a reversible migration that replaces the
   migration-071 predicate with `status IN ('scheduled', 'quota_hold')`.
5. Verify same-payload replay, different-payload conflict, migration Down, and
   exact-schema checks before promoting to the next environment.

Phase B does not require planned downtime, but it must not share the Phase A
rolling deployment. If an old instance or rollback binary is still present,
defer Phase B.
