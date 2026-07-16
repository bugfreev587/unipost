# Paid Soft Overage and Scheduling Circuit Breaker Design

## Problem

UniPost currently treats API, Basic, and Growth monthly post limits as soft overage. Usage is recorded and surfaced through warnings, but publishing continues beyond the plan limit. This protects active customer integrations from sudden interruption, yet the current behavior has three gaps:

- Paid workspaces can continue adding unlimited scheduled workload after reaching their monthly plan quota.
- Paid workspaces do not receive deterministic quota emails.
- UniPost has no structured escalation or follow-up path for sustained paid-plan overage.

The result is unpredictable scheduled workload, limited upgrade guidance, and no reliable warning before scheduling becomes unsustainable. Because immediate publishing remains available under soft overage, this policy provides a bounded scheduling control rather than a complete cost cap.

## Outcome

Keep immediate publishing available for finite paid self-serve plans, while introducing a scheduling circuit breaker and deterministic email escalation:

- API, Basic, and Growth continue using soft overage for immediate publishing.
- Monthly effective usage includes successful publishes plus committed scheduled platform units.
- New scheduled quota units are accepted only while projected effective usage remains at or below 100% of the target month's limit.
- Existing scheduled posts continue executing after the workspace reaches 100%.
- Quota emails are evaluated at 80%, 90%, 100%, 105%, 110%, 115%, and 120%.
- Warning emails below 100% respect the user's quota-email preference.
- Alerts at and above 100% are required service notices.
- A 120% threshold creates an administrative follow-up item.
- A plan downgrade revalidates future scheduled work against the new plan and places excess work on quota hold.
- No automatic overage charge or automatic immediate-publishing shutdown is introduced.

## Scope

### Included plans

- API
- Basic
- Growth

### Excluded plans

- Free retains its existing hard monthly quota, active scheduled-post cap, and Free quota email service. A paid-to-Free transition may use downgrade quota hold before the existing Free policies become authoritative.
- Team remains unlimited for monthly UniPost posts.
- Enterprise follows contract-defined quota behavior and is not opted into this policy by default.

### Non-goals

- Do not introduce metered overage billing.
- Do not automatically suspend immediate publishing at or above 120%.
- Do not stop, cancel, or rewrite existing scheduled posts merely because normal usage crosses a threshold. Plan-downgrade quota hold is the explicit exception.
- Do not merge the existing Free quota service into the paid policy in this change.
- Do not add a feature flag.
- Do not change platform rate limits, queue limits, abuse controls, or upstream platform quotas.

## Recommended Architecture

Refactor the existing quota core so Free and paid policy share one monthly usage snapshot and projected-usage calculation.

`quota.Checker` already calculates completed usage, scheduled reservations, and the Free projected gate. Extract a plan-agnostic monthly snapshot and `WouldExceed(additionalUnits)` primitive from that implementation. The existing Free wrappers continue using the shared primitive to hard-block publish admission. A paid quota policy coordinator uses the same primitive only for schedule-increasing writes.

The paid coordinator owns plan eligibility, downgrade quota hold, threshold evaluation, and notification orchestration. It must not implement a second effective-usage query or arithmetic path.

Paid notification delivery uses a separate ledger modeled after the existing Free quota reminder ledger because the paid sequence requires preference skips, superseded thresholds, leases, and multiple retries that the Free table does not support. Both paths reuse the email registry, audited Loops client, email preferences, and Admin email reporting.

```text
Schedule write request
    |
    v
Shared quota.Checker monthly snapshot
    |
    v
Paid schedule policy admission
    |-- allowed --> commit schedule change --> evaluate affected periods
    |
    `-- denied  --> HTTP 402 quota error; no data change; no threshold email

Successful publish / cancellation / reschedule / plan change
    |
    v
Recalculate affected monthly snapshot
    |
    v
Threshold ledger --> asynchronous email worker --> Loops
                         |
                         `--> unified email_send_attempts audit
```

This structure keeps one source of truth for completed and committed scheduled units while preserving different product behavior: Free hard-blocks publish admission, whereas paid plans block only schedule-increasing writes.

## Monthly Usage Model

### Effective usage

For one workspace and UTC calendar month:

```text
effective_usage(period)
  = completed_posts(period)
  + committed_scheduled_units(period)
```

- `completed_posts` is the existing successful platform-publish usage count for the actual UTC month in which publishing succeeded.
- `committed_scheduled_units` counts undeleted platform destinations whose parent post is `scheduled`, scheduled-origin `publishing`, or `quota_hold` for that UTC month.
- One parent post scheduled to Instagram, TikTok, and LinkedIn consumes three scheduled units.
- A `publishing` post with a non-null `scheduled_at` remains reserved until it reaches a terminal state. This closes the current gap between claiming a scheduled post and incrementing completed usage.
- `quota_hold` units remain committed for percentage, notification, and admission purposes even though the delivery worker cannot execute them.
- Draft, cancelled, deleted, failed, partial, and already-published parent posts do not reserve scheduled units. Immediate-origin `publishing` posts with no `scheduled_at` also do not reserve units.
- A scheduled unit is associated with the UTC month of `scheduled_at`.
- Scheduled-unit counting continues to honor the existing disconnected-account filtering and administrative scheduled-quota reset baseline.

When a scheduled result publishes successfully, it stops being a committed scheduled unit and becomes completed usage. If execution crosses a UTC month boundary, the reservation remains in the scheduled month while the parent is `publishing`, then the successful platform call is charged to the actual publish month. No special exemption is added for delayed execution because the existing usage ledger records real provider work in the month when it occurs.

### Percentage

Threshold comparisons use exact integer arithmetic:

```text
effective_usage * 100 >= threshold * post_limit
```

`percentage` and `effective_percentage` are JSON number display values and may contain fractional percentages. Clients must not depend on whether a whole-number value is serialized with a decimal point. Rounding must not affect admission or threshold decisions.

### Cross-month behavior

A scheduled post consumes quota in its planned UTC publication month:

- A July post consumes July scheduled quota.
- An August post consumes August scheduled quota even if created in July.
- Moving a scheduled post from July to August releases July units and consumes August units.
- Moving a post into a month that would exceed the target month's scheduling capacity is rejected atomically.

This permits customers who have exhausted the current month to prepare future-month content.

## Schedule Admission Policy

All operations that can increase scheduled units must use one shared admission service:

- Dashboard schedule creation
- Public API schedule creation
- Draft-to-scheduled transition
- Immediate-to-scheduled transition
- Adding platform destinations to a scheduled post
- Moving a scheduled post into another month
- MCP and CLI flows that call the same API
- Future schedule-writing entry points

The current Bulk API supports immediate publishing only. It remains outside schedule admission and retains paid soft-overage behavior.

For the target month:

```text
projected_usage
  = current_effective_usage
  - units_released_by_the_same_atomic_edit
  + requested_new_units
```

Decision:

- `projected_usage <= post_limit`: allow the operation.
- `projected_usage > post_limit`: reject the entire operation.
- An operation that reaches exactly 100% is allowed.
- Once current effective usage is at or above 100%, any operation that adds net scheduled units is rejected.

The policy does not block:

- Existing scheduled posts from executing.
- Immediate publishing.
- Cancelling or deleting scheduled posts.
- Removing scheduled platform destinations.
- Editing copy or media without increasing scheduled units.
- Moving a post when the atomic release and reservation leave the target month within quota.

### Atomicity and concurrency

Admission and the related schedule mutation must run in one database transaction.

Use a PostgreSQL transaction advisory lock keyed by workspace and UTC period. Reuse the repository's established `pg_advisory_xact_lock` pattern with a distinct namespace for paid monthly schedule quota. Operations affecting two periods acquire both locks in sorted period order to prevent deadlocks.

After acquiring the lock, the transaction recalculates completed and committed scheduled usage before making the final decision. A multi-destination scheduled request is atomic for quota admission: if the whole request would exceed the target month's quota, none of its scheduled units are created.

This prevents concurrent requests at 99% from both observing the same stale capacity and exceeding 100%.

## API Contract

### Rejection status

Return the standard HTTP status:

```http
402 Payment Required
```

Use a plan-specific application error rather than inventing a nonstandard HTTP status:

```json
{
  "error": {
    "code": "PLAN_MONTHLY_SCHEDULING_CAPACITY_EXCEEDED",
    "normalized_code": "plan_monthly_scheduling_capacity_exceeded",
    "message": "Monthly scheduling quota reached. Upgrade your plan, cancel existing scheduled posts, or wait until the quota resets. Immediate publishing remains available.",
    "details": {
      "completed_posts": 2488,
      "scheduled_posts": 12,
      "quota_hold_posts": 0,
      "effective_usage": 2500,
      "post_limit": 2500,
      "requested_units": 1,
      "period": "2026-07",
      "resets_at": "2026-08-01T00:00:00Z",
      "upgrade_recommended": true,
      "immediate_publishing_allowed": true
    }
  }
}
```

This name is intentionally distinct from `PLAN_SCHEDULED_POST_LIMIT_EXCEEDED`, which represents the Free plan's active scheduled parent-post count cap.

### Response headers

Preserve the current meaning of `X-UniPost-Usage` as completed usage. Add:

```http
X-UniPost-Usage: 2488/2500
X-UniPost-Scheduled-Usage: 12
X-UniPost-Quota-Hold-Usage: 0
X-UniPost-Effective-Usage: 2500/2500
X-UniPost-Warning: scheduled_quota_reached
```

Normal paid quota responses use:

- `approaching_limit` from 80% through below 100%.
- `scheduled_quota_reached` when effective usage is at or above 100% and schedule admission is unavailable.
- Existing completed-usage `over_limit` behavior may remain for surfaces that do not calculate scheduled usage, but schedule-writing and billing surfaces must expose the effective status.

### Usage endpoint

Extend `GET /v1/usage` without removing existing fields:

```json
{
  "period": "2026-07",
  "post_count": 2488,
  "scheduled_count": 12,
  "quota_hold_count": 0,
  "effective_usage": 2500,
  "post_limit": 2500,
  "percentage": 99.52,
  "effective_percentage": 100.0,
  "warning": "scheduled_quota_reached",
  "scheduling_allowed": false,
  "resets_at": "2026-08-01T00:00:00Z",
  "plan": "basic"
}
```

`scheduled_count` is the total committed scheduled-unit count, including any `quota_hold` units. `quota_hold_count` is its held subset so the Dashboard can explain why some committed work will not execute. `effective_usage` equals `post_count + scheduled_count`.

## Email Threshold Policy

| Threshold | Severity | Preference behavior | Required message |
|---|---|---|---|
| 80% | Warning | Respect `usage_quota_alerts` | Usage is rising; review current usage and plan |
| 90% | Warning | Respect `usage_quota_alerts` | Scheduling capacity is nearly exhausted |
| 100% | Alert | Required service notice | New scheduled units are unavailable |
| 105% | Alert | Required service notice | Workspace is continuing above plan quota |
| 110% | Alert | Required service notice | Serious sustained overage; upgrade recommended |
| 115% | Alert | Required service notice | High sustained overage |
| 120% | Critical Alert | Required service notice | Highest escalation and administrative follow-up |

Required alerts contain only quota status, operational consequences, recovery options, and relevant upgrade guidance. They must not include unrelated promotional content.

The recipient is the workspace owner's primary email address. A future billing-contact feature may take precedence, but it is outside this change.

### Email registry policy

Register two paid quota email events that may share one Loops transactional template:

- `email.quota.paid_plan_warning.v1` for 80% and 90%. It uses `usage_quota_alerts`, is preference-gated, uses a manage-preferences footer, and resolves `LOOPS_PAID_PLAN_QUOTA_TRANSACTIONAL_ID`.
- `email.quota.paid_plan_alert.v1` for 100% through 120%. It is not preference-gated, records the required-service reason, uses the required-notice footer, and resolves the same transactional template ID.

The ledger stores the selected event key for every threshold decision. This avoids dynamically changing one registry event between optional and required delivery.

The existing Free event remains unchanged in this feature. Its current preference behavior is not used as the implementation model for paid Warning emails.

### Threshold evaluation events

Evaluate affected periods after:

- A successful platform publish increments completed usage.
- A scheduled post or platform unit is created.
- A scheduled post is cancelled or deleted.
- A scheduled post changes month.
- Platform destinations are added or removed.
- A draft becomes scheduled.
- A subscription changes plan.
- A downgrade places posts on quota hold or a later capacity reconciliation releases them.
- An administrative quota reset changes completed usage.

Email failure never changes the result of the triggering business operation.

### Crossing multiple thresholds

When one committed state change crosses several unsatisfied thresholds, send only the highest threshold reached.

Example:

```text
79% -> 106%
```

The service creates the 105% notification as sendable and records 80%, 90%, and 100% as `skipped_superseded` unless a final decision already exists for those thresholds.

This prevents bursts of multiple emails while preserving an auditable decision for every crossed threshold.

### No email for rejected projections

A rejected schedule request does not change effective usage. Therefore:

- A request at 99% that would project usage to 101% returns HTTP 402.
- Effective usage remains 99%.
- No 100% email is generated.

Threshold emails are based only on committed usage state.

### Monthly idempotency

Each workspace, UTC period, and threshold receives one final decision:

```text
paid_quota:{workspace_id}:{period}:{threshold}
```

Usage may drop after cancellation and later cross the same threshold again, but the email is not repeated. A new UTC month starts a new threshold sequence.

## Notification Ledger

Add a `paid_plan_quota_notifications` ledger, separate from the Free quota reminder table.

Each row records:

- Workspace and owner user
- Recipient email snapshot
- Plan ID
- UTC period
- Threshold percent
- Severity
- Registry event key
- Completed, scheduled, effective, and limit snapshots
- Transactional template ID
- Idempotency key
- Status
- Attempt count
- Next attempt time
- Lease/processing timestamps
- Last provider error
- Attempted, sent, created, and updated timestamps

Allowed status values:

- `pending`
- `processing`
- `sent`
- `retry_wait`
- `failed`
- `skipped_superseded`
- `skipped_preference_disabled`
- `skipped_missing_recipient`

A unique constraint on `(workspace_id, period, threshold_percent)` enforces the monthly decision rule.

The ledger owns threshold state and retry scheduling. The unified `email_send_attempts` table remains the provider-send audit surface.

## Rollout and Reconciliation

The schedule admission policy becomes authoritative as soon as the production code is active. Crossing a usage threshold during normal operation does not cancel or hold existing scheduled posts, including workspaces whose current effective usage is already above the plan limit. The separate downgrade transition policy may hold future excess work when a lower plan becomes effective.

After deployment, run one idempotent reconciliation for the current UTC month across eligible API, Basic, and Growth workspaces:

- Recalculate completed, scheduled, effective, and limit values.
- Select only the highest threshold currently reached.
- Create that threshold as sendable when it has no existing final decision.
- Record all lower reached thresholds without existing decisions as `skipped_superseded`.
- Apply `usage_quota_alerts` preference handling when the highest selected threshold is 80% or 90%.
- Treat 100% and higher as required alerts.
- Create the 120% administrative follow-up when applicable.

Examples:

- Current usage of 99% sends the 90% Warning and records 80% as superseded.
- Current usage of 108% sends the 105% Alert and records 80%, 90%, and 100% as superseded.
- Current usage of 125% sends the 120% Critical Alert, records all lower thresholds as superseded, and creates the follow-up.

The unique ledger constraint and stable idempotency keys make reconciliation safe to rerun. Reconciliation creates database decisions only; the asynchronous worker performs provider sends.

Run an idempotent reconciliation daily at 00:15 UTC for the current UTC month. This includes the start-of-month evaluation for scheduled units created before the month began and acts as a safety net for missed business-event evaluations. It must produce no additional email after a threshold has a final decision.

## Email Delivery

### Template

Use one Loops transactional template for both paid quota registry events. Variables control:

- Subject and preview text
- Warning, Alert, or Critical Alert presentation
- Workspace and owner names
- Plan name
- Threshold reached
- Completed, committed scheduled, held, effective, limit, percentage, and remaining scheduling capacity
- UTC period and reset time
- Whether scheduling is currently available
- Upgrade, billing, and scheduled-post management URLs

Using one template avoids seven independently drifting designs while keeping threshold-specific idempotency and audit records.

### Asynchronous worker

The evaluator records notification decisions after the business mutation commits. A database-backed worker then:

1. Claims eligible `pending` or due `retry_wait` rows with a lease.
2. Writes or updates the audited Loops send attempt.
3. Sends the transactional email with the stable quota idempotency key.
4. Marks the notification `sent`, schedules another retry, or marks it `failed`.

Provider failure must not fail publishing, cancellation, plan changes, or schedule admission.

### Retry policy

Use one initial attempt and up to three retries:

1. Initial attempt immediately
2. Retry after 5 minutes
3. Retry after 1 hour
4. Retry after 6 hours

All provider requests reuse the same Loops idempotency key. After the fourth total provider request fails, mark the notification `failed`.

The worker uses `FOR UPDATE SKIP LOCKED` or the repository's established leased-job pattern so multiple API instances claim disjoint work. Expired `processing` leases are recoverable.

## Administrative Follow-up

Reaching 120% creates one row in `paid_quota_follow_ups` per workspace and period. It contains:

- Workspace and owner
- Current plan
- Completed, scheduled, effective, and limit snapshots
- Held-unit snapshot
- Threshold time
- Recent activity context needed for support or sales outreach
- Open and resolved timestamps

Crossing above 120% sends no additional quota-threshold email.

Upgrading to a plan whose recalculated effective usage is below 100% automatically resolves the open follow-up. Otherwise the follow-up remains available for manual resolution.

The follow-up does not itself restrict immediate publishing. Any stronger account action requires an explicit administrative decision outside this policy.

## Downgrade Quota Hold

When a downgrade becomes effective, revalidate future committed scheduled work against the new finite plan limit before allowing the delivery worker to continue normally.

This check runs in the effective plan-change transaction or in an idempotent follow-up job keyed by workspace and plan-change reference. It covers every UTC month within the existing 90-day scheduling horizon.

Rules:

- Posts whose `scheduled_at` is earlier than the downgrade effective timestamp are grandfathered, are not rewritten, and consume executable capacity before later scheduled work.
- Completed usage, grandfathered scheduled units, and already-claimed `publishing` units consume executable capacity first.
- Remaining `scheduled` parent posts are considered in deterministic order by `scheduled_at`, `created_at`, then `id`.
- A parent post and all its platform units are allocated atomically. The system does not execute only part of a multi-destination parent because of a downgrade.
- A parent that fits entirely remains `scheduled`.
- A parent that does not fit entirely changes to `quota_hold`; it is not deleted and the delivery worker must not claim it.
- Held units continue to count in `scheduled_count` and `effective_usage`, preserving truthful overage percentages and preventing new work from bypassing the hold.
- Any target month with one or more held posts rejects new schedule-increasing writes, even if a smaller new request could fit unused executable capacity before a larger held parent.

Capacity reconciliation runs after an upgrade, cancellation, deletion, destination removal, completed-usage reset, or other capacity-releasing event. It considers held parents in the same deterministic order:

- Future-dated parents that now fit return to `scheduled`.
- A held parent whose `scheduled_at` has already passed remains held and requires the user to select a new scheduled time or publish immediately. It must not publish late without explicit user action.
- If only some held parents fit, the remaining parents stay held.

For downgrades into API, Basic, or Growth, threshold evaluation uses completed plus all committed scheduled units, including held units, and selects the highest newly reached paid threshold. For cancellation or downgrade into Free, the transition may place excess future work on hold, but notification delivery uses the existing Free quota policy rather than the paid alert sequence.

The Dashboard and API expose held status, reason, original scheduled time, and actions to upgrade, cancel, reduce destinations, reschedule, or publish immediately.

## Plan Changes and Resets

### Upgrade

- Recalculate against the new limit immediately.
- Reconcile held posts against the new capacity.
- Restore schedule admission only when effective usage is below 100% and the target month has no held posts.
- Preserve all existing scheduled posts and notification history.
- Resolve the 120% follow-up if the upgraded quota brings usage below 100%.

### Downgrade

- Apply the new plan when the Stripe subscription change becomes effective, preserving the repository's existing period-end downgrade timing.
- Recalculate against the new limit immediately at that effective time.
- Apply the deterministic downgrade quota-hold allocation to future scheduled parents.
- If effective usage is at or above 100%, prevent new scheduled units immediately.
- If the change crosses multiple unsatisfied thresholds, send only the highest reached and mark lower ones `skipped_superseded`.
- If the new percentage is at or above 120%, send the 120% Critical Alert and create the follow-up.

### Administrative completed-usage reset

- Recalculate the affected period after the reset.
- Reconcile held posts, then restore scheduling only when effective usage is below 100% and no held posts remain for the period.
- Do not delete or reopen threshold decisions already finalized for that month.

## Dashboard Experience

### Billing and usage

Show the components of effective usage separately:

- Successfully published
- Committed scheduled units
- Units on quota hold
- Effective usage
- Plan limit

At 80% through below 100%, show a yellow warning. At or above 100%, show a red alert:

> New scheduled posts are paused for this month. Existing scheduled posts and immediate publishing remain available.

When `quota_hold_count` is greater than zero, replace the generic sentence with explicit held-post copy:

> Some future scheduled posts are on hold after your plan changed. They will not publish unless capacity is restored or you reschedule or publish them manually.

### Schedule actions

The Dashboard may proactively disable schedule actions using the usage response, but the backend remains authoritative.

On quota rejection, offer:

- Upgrade plan
- Manage or cancel scheduled posts
- Publish immediately

Editing actions that do not add net scheduled units remain available.

Held posts have a distinct status treatment and explanation. They must not appear as normally scheduled or imply that delivery will occur at the original time.

## Observability and Admin Email Reporting

Extend the existing Admin Email Notifications view to include paid quota notifications with:

- Threshold and severity
- UTC period
- Completed, scheduled, effective, and limit snapshots
- Status and attempt count
- Last provider error
- Attempted and sent timestamps
- Manual retry for terminal `failed` rows

Logs must identify workspace, period, threshold, notification ID, and status without exposing Loops credentials or unrelated user content.

Operational metrics should include:

- Schedule admission rejections by plan
- Notifications created, sent, skipped, retried, and failed by threshold
- Workspaces at or above 100% and 120%
- Held posts and held units by plan and target month
- Open paid quota follow-ups

## Error Handling

- Entitlement lookups such as optional feature access retain the repository's existing fail-open posture where documented.
- Monthly capacity admission is intentionally fail-closed for schedule-increasing mutations because it runs inside a bounded write transaction and accepting unknown committed workload would violate the scheduling policy.
- Cancellation, deletion, and net quota-releasing edits must remain available when possible; if their transaction cannot safely calculate the change, return a normal internal error without partially mutating data.
- Email evaluation or delivery failure is logged and audited but does not roll back the completed business mutation.
- Missing owner email produces an auditable terminal skip or failure reason rather than blocking quota policy.
- Unknown, unlimited, or contract-controlled plans do not enter the paid self-serve notification sequence.

## Testing

### Unit tests

- Plan eligibility for API, Basic, Growth, Free, Team, and Enterprise
- Effective usage arithmetic
- Scheduled-origin `publishing` and `quota_hold` units remain committed; immediate-origin `publishing` does not
- Exact 80/90/100/105/110/115/120 comparisons without display rounding
- Exactly 100% accepted and any projected value above 100% rejected
- Immediate publishing remains allowed at 100% and 120%
- Net-zero and quota-releasing edits remain allowed
- Cross-month release and reservation
- Highest-threshold-only selection
- Monthly deduplication after usage falls and rises
- Warning preference skips and required alerts
- Warning and Alert registry events use the same template with different preference and footer policies
- Plan upgrade, downgrade, and administrative reset decisions
- Deterministic downgrade allocation and quota-hold promotion
- Past-due held posts require explicit rescheduling or immediate publishing

### Database and concurrency tests

- Advisory-lock contract for workspace and period
- Concurrent schedule requests cannot exceed the target month's limit
- Multi-period locks are acquired in stable order
- Multi-destination schedule admission is atomic
- Unique threshold constraint prevents duplicate decisions
- Worker claims are disjoint across instances
- Expired processing leases recover safely

### Handler and contract tests

- Dashboard/API schedule creation
- Draft-to-scheduled
- Immediate-to-scheduled
- Adding scheduled destinations
- Removing destinations
- Same-month edits
- Cross-month rescheduling
- Error code, normalized code, details, and headers
- Additive `GET /v1/usage` response fields
- Existing Free limits and error contracts remain unchanged while the shared reservation query gains in-flight undercount coverage
- Existing immediate-only Bulk API remains outside schedule admission
- Effective downgrade applies and releases quota holds idempotently

### Email tests

- One transactional template receives the correct severity variables
- 79% to 106% sends only 105% and records lower thresholds as superseded
- Disabled Warning preference creates a skipped decision
- Required alerts ignore the optional Warning preference
- Initial send plus three retry intervals
- Stable provider idempotency key across retries
- Terminal failure appears in Admin and supports manual retry
- 120% creates only one follow-up and higher usage sends no further threshold email

### Dashboard regression tests

- Billing displays completed, committed scheduled, held, and effective usage
- Warning and Alert states render correctly
- Schedule actions disable proactively at 100%
- Quota error offers upgrade, schedule management, and immediate publish actions
- Quota-releasing edits remain accessible
- Held posts are clearly distinguished and cannot be mistaken for executable schedules

## Validation and Release

Before promotion:

- From `api/`, run `GOCACHE=/tmp/unipost-go-build go test ./...`.
- From `dashboard/`, run `npm run build`.
- Run `npm run test:regression:dashboard` when Playwright browsers are installed.

After pushing `origin/dev`, wait for all GitHub Actions, Railway, and Vercel development deployments to finish. Then verify in the real development environment:

- Usage endpoint values and headers
- Exactly-100% admission
- Above-100% schedule rejection
- Immediate publishing after the schedule breaker
- Cancellation restoring capacity
- Cross-month scheduling
- Scheduled-to-`publishing` transitions do not temporarily release capacity
- Downgrade quota hold and later upgrade/cancellation recovery
- Dashboard Warning and Alert behavior
- Loops notification audit and Admin visibility

The task is not complete until the development deployment succeeds and the expected behavior is personally verified on the development domains.

## Acceptance Criteria

- API, Basic, and Growth effective usage equals completed plus committed `scheduled`, scheduled-origin `publishing`, and `quota_hold` platform units for each UTC month.
- New scheduled units cannot make projected effective usage exceed the target month's plan limit.
- Existing scheduled posts and immediate publishing continue when normal usage crosses 100%; an effective plan downgrade may hold future work that exceeds the new plan capacity.
- All schedule-writing entry points return the same 402 error contract.
- 80% and 90% Warning emails respect the quota-email preference.
- 100%, 105%, 110%, 115%, and 120% alerts are required service notices.
- A single state change sends only the highest newly reached threshold.
- Each workspace, period, and threshold has one final decision.
- Email delivery is asynchronous, audited, idempotent, and retried without affecting business requests.
- Reaching 120% creates one administrative follow-up and no later threshold email.
- Plan changes and cancellations recalculate schedule availability without deleting scheduled content; excess future work is preserved on quota hold.
- Dashboard, schedule API, MCP, and CLI behavior remain consistent. The immediate-only Bulk API retains paid soft-overage behavior.
- Free steady-state, Team, and Enterprise behavior remains outside the new paid self-serve policy, apart from safe paid-to-Free transition holding.
