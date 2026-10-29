# Platform incident circuit breaker

Use this runbook to isolate one provider without taking UniPost's other platforms offline. The operational control is independent of plan-based publishing restrictions.

## Safety properties

- `active`: connect, publish, retry/resume, token refresh, and provider reads are allowed.
- `publishing_disabled`: connect and provider reads remain available; new and resumed publishing is blocked.
- `fully_disabled`: connect, publish/retry/resume, token refresh, analytics, creator-info, board, and other authenticated provider calls are blocked.
- A control lookup error fails closed before a provider call.
- Mutations require the current version and write an audit event.
- Never paste `DATABASE_URL`, OAuth secrets, access tokens, or refresh tokens into logs, tickets, or chat.

## TikTok incident recovery order

Keep the API, delivery worker, scheduler, and background provider workers offline until step 5 succeeds.

1. Confirm a recoverable production database backup exists.
2. Deploy the commit containing migration `138_platform_operational_controls.sql` while provider-calling services remain offline.
3. Apply migrations using the normal Railway migration command and confirm migration 138 completed.
4. Read TikTok's current version:

   ```sql
   SELECT platform, state, version, reason_code, state_changed_at
   FROM platform_operational_controls
   WHERE platform = 'tiktok';
   ```

5. From a trusted Railway shell with `DATABASE_URL` already injected, set TikTok to `fully_disabled`. Replace `1` with the version returned above:

   ```bash
   go run ./cmd/platform-control \
     --platform tiktok \
     --state fully_disabled \
     --expected-version 1 \
     --reason security_incident_2026_10_29
   ```

   The command must print only `platform`, `state`, `version`, and `changed`.

6. Verify both the persisted state and its audit event:

   ```sql
   SELECT platform, state, version, reason_code, user_message, state_changed_at
   FROM platform_operational_controls
   WHERE platform = 'tiktok';

   SELECT platform, event_type, expected_version, resulting_version, request_id, created_at
   FROM platform_operational_control_events
   WHERE platform = 'tiktok'
   ORDER BY created_at DESC
   LIMIT 5;
   ```

7. Verify every other platform remains independently `active` unless an incident decision says otherwise:

   ```sql
   SELECT platform, state, version
   FROM platform_operational_controls
   ORDER BY platform;
   ```

## Staged service restoration

1. Restore only the API service.
2. Confirm `/health` is healthy and profiles/accounts remain visible.
3. Using an authenticated operator account, verify TikTok OAuth/Hosted Connect, new publish, retry, creator-info, and forced analytics refresh return `PLATFORM_OPERATION_DISABLED`.
4. Verify LinkedIn or another approved active platform can still load its connect entry and accept a non-provider-destructive publish validation path.
5. Inspect logs: no TikTok provider-start, token-refresh, analytics, or connector exchange event may appear after the control was activated.
6. Restore scheduler and delivery workers only after the rejection probes pass.
7. Inspect pending/running/retrying delivery jobs. A claimed TikTok job, including one with a stored `publish_token`, must finalize with:
   - `error_code = platform_operation_disabled`
   - `failure_stage = platform_operational_control`
   - `next_action = retry_after_platform_recovery`
8. Restore remaining background workers and continue monitoring for TikTok outbound calls.

## Rollback and recovery

If another platform was disabled accidentally, read its current version and use the same command with `--state active` and an explicit reason. Never bypass optimistic version checking with direct SQL during normal operations.

TikTok may return to `active` only after all of the following are true:

- TikTok client secret rotation/revocation is complete and independently verified.
- All compromised TikTok tokens are revoked or expired, and reconnect is required where appropriate.
- Root cause and malicious publishing path are identified and closed.
- Production rejection and audit evidence has been retained.
- A controlled staging reconnect, publish, retry/resume, token refresh, and analytics test passes with the replacement credentials.
- The incident owner explicitly approves recovery.

To recover, read the current TikTok version, run the command with `--state active`, verify the audit event, restore only the API, perform a controlled canary, and then restore workers. If the canary fails or any unexpected provider call appears, set TikTok back to `fully_disabled` using the new version and stop restoration.
