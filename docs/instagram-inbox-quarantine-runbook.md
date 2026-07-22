# Instagram Inbox Tenant-Isolation Quarantine Runbook

This runbook moves locally ambiguous Instagram comments and DMs out of every user-visible Inbox after the exact routing deployment is serving the target environment. It preserves each complete Inbox row in `inbox_item_quarantine` before deleting the live copy. It never guesses which tenant owns an event.

The operation must run in staging first. In production, stop after the dry run and obtain explicit production approval before applying it.

## Safety properties

- The script defaults to `apply=false`, opens a repeatable-read transaction, and rolls back.
- The candidate set is limited to Instagram `ig_comment` and `ig_dm` rows whose `(source, external_id)` appears under more than one distinct Instagram `external_account_id`.
- An advisory transaction lock prevents two quarantine operators from running concurrently.
- Candidate Inbox rows are row-locked while the transaction is open.
- The dry run prints only candidate count and candidate digest. No message bodies, authors, access tokens, or preserved JSON are printed.
- Apply requires the exact candidate count and candidate digest from the immediately preceding dry run plus `recovery_ready=true`.
- Every original row is preserved as JSONB before any live row is deleted.
- Preserved, deleted, and remaining counts must all match before commit.
- `(incident_key, original_inbox_item_id)` prevents duplicate evidence records.
- A blanket restoration is unsafe and prohibited.

## Preconditions

1. The exact routing deployment has completed in the target environment and the deployed API SHA is recorded.
2. Required CI, deployment health, and critical Inbox acceptance checks are successful on that SHA.
3. Instagram mapping coverage has been checked after the existing-account backfill worker ran.
4. Recent Instagram webhook entry IDs resolve only through exact `instagram_webhook_user_id` mappings. Missing mappings remain fail-closed and must be investigated before cleanup for the affected account.
5. A database snapshot exists or PITR is confirmed usable for the target environment. Record the recovery evidence without exposing `DATABASE_URL`.
6. An incident key has been chosen, for example `instagram-inbox-idor-2026-07-20`.
7. The operator is authorized for the target environment.

Do not print, paste into tickets, screenshots, CI logs, or commit the value of `DATABASE_URL`. No message bodies or quarantine JSON may appear in release evidence.

## Mapping coverage check

Run this content-free aggregate after the fixed API and backfill worker are active:

```sql
SELECT
  COUNT(*) AS active_instagram_accounts,
  COUNT(*) FILTER (
    WHERE NULLIF(BTRIM(sa.metadata->>'instagram_webhook_user_id'), '') IS NOT NULL
  ) AS mapped_accounts,
  COUNT(*) FILTER (
    WHERE NULLIF(BTRIM(sa.metadata->>'instagram_webhook_user_id'), '') IS NULL
  ) AS missing_mappings
FROM social_accounts sa
WHERE sa.platform = 'instagram'
  AND sa.status = 'active'
  AND sa.disconnected_at IS NULL;
```

Record only aggregate counts. Attempt the account-scoped backfill for missing mappings and investigate failures without restoring fan-out or arbitrary fallback behavior.

## Dry run

Run from the repository root against the checked-in SHA:

```bash
psql "$DATABASE_URL" \
  -v incident_key=instagram-inbox-idor-2026-07-20 \
  -v apply=false \
  -f api/ops/instagram_inbox_quarantine.sql
```

Record the candidate count and candidate digest. Reconcile the result with an independently reviewed duplicate-count query. An unexpectedly empty, materially larger, or changed candidate set must be investigated before apply.

The dry run always rolls back and makes no persistent database change.

## Staging apply

Confirm the staging snapshot or PITR evidence, then supply the unchanged incident key and the exact values from the immediately preceding staging dry run:

```bash
psql "$DATABASE_URL" \
  -v incident_key=instagram-inbox-idor-2026-07-20 \
  -v apply=true \
  -v recovery_ready=true \
  -v expected_count=33 \
  -v expected_digest=0123456789abcdef0123456789abcdef \
  -f api/ops/instagram_inbox_quarantine.sql
```

The example count and digest are placeholders. Never reuse them. The transaction commits only when the current locked candidate set exactly matches both supplied values and every row is preserved before deletion.

After staging apply:

1. Verify `candidate_count = preserved_count = deleted_count` and `remaining_count = 0` from the script output.
2. Wait for account-scoped Inbox sync to run with each account's own token.
3. Verify no restored `(source, external_id)` spans distinct Instagram external account IDs.
4. Exercise Inbox list, item, media context, mark-read, thread-state, comment reply, and DM reply flows.
5. Confirm no message content was written to operator or deployment logs.

## Production gate and apply

After the exact fixed SHA is deployed and accepted in production:

1. Run the mapping coverage check.
2. Confirm the production snapshot or PITR recovery evidence.
3. Run only the production dry run.
4. Present the production candidate count, candidate digest, mapping coverage, deployed SHA, and recovery evidence to the user.
5. Obtain explicit production approval for the exact apply command and candidate set.
6. Only then run the apply command with the unchanged incident key, exact count, exact digest, and `recovery_ready=true`.

Any changed count or digest invalidates the approval. Run a new dry run, explain the difference, and request approval again. Never adjust an expected value merely to make the transaction commit.

## Restoration and verification

The existing account-scoped sync is the normal restoration path. It calls the provider using each social account's own token, so upstream authorization supplies independent upstream ownership evidence.

Quarantined rows outside the provider sync window may remain unavailable. They must not blanket-restore from `original_row`, because that would recreate the cross-tenant disclosure. A targeted restoration requires independent upstream ownership evidence for each original event and a separately reviewed procedure.

Verify both the affected workspace and the global condition without selecting message bodies:

```sql
SELECT COUNT(*) AS remaining_cross_account_groups
FROM (
  SELECT i.source, i.external_id
  FROM inbox_items i
  JOIN social_accounts sa ON sa.id = i.social_account_id
  WHERE sa.platform = 'instagram'
    AND i.source IN ('ig_comment', 'ig_dm')
  GROUP BY i.source, i.external_id
  HAVING COUNT(DISTINCT sa.external_account_id) > 1
) duplicate_groups;
```

Completion requires `remaining_cross_account_groups = 0`, healthy account-scoped sync, no new cross-account groups, and successful production Inbox acceptance.

## Stop conditions

Stop without further mutation when:

- the exact routing deployment or mapping coverage cannot be verified;
- snapshot/PITR readiness is missing;
- the count or digest differs from the approved dry run;
- preserved, deleted, or remaining counts do not match;
- a required check fails, times out, is cancelled, or validates another SHA;
- staging restoration or Inbox acceptance fails;
- explicit production approval is absent.
