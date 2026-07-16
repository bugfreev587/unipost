# Paid Soft Overage and Scheduling Circuit Breaker Design

## Problem

UniPost currently treats API, Basic, and Growth monthly post limits as soft overage. Usage is recorded and surfaced through warnings, but publishing continues beyond the plan limit. This protects active customer integrations from sudden interruption, yet the current behavior has three gaps:

- Paid workspaces can continue adding unlimited scheduled workload after reaching their monthly plan quota.
- Paid workspaces do not receive deterministic quota emails.
- UniPost has no structured escalation or follow-up path for sustained paid-plan overage.

The result is weak cost control, limited upgrade guidance, and no reliable warning before scheduling becomes unsustainable.

## Outcome

Keep immediate publishing available for finite paid self-serve plans, while introducing a scheduling circuit breaker and deterministic email escalation:

- API, Basic, and Growth continue using soft overage for immediate publishing.
- Monthly effective usage includes successful publishes plus active scheduled platform units.
- New scheduled quota units are accepted only while projected effective usage remains at or below 100% of the target month's limit.
- Existing scheduled posts continue executing after the workspace reaches 100%.
- Quota emails are evaluated at 80%, 90%, 100%, 105%, 110%, 115%, and 120%.
- Warning emails below 100% respect the user's quota-email preference.
- Alerts at and above 100% are required service notices.
- A 120% threshold creates an administrative follow-up item.
- No automatic overage charge or automatic immediate-publishing shutdown is introduced.

## Scope

### Included plans

- API
- Basic
- Growth

### Excluded plans

- Free retains its existing hard monthly quota, active scheduled-post cap, and Free quota email service.
- Team remains unlimited for monthly UniPost posts.
- Enterprise follows contract-defined quota behavior and is not opted into this policy by default.

### Non-goals

- Do not introduce metered overage billing.
- Do not automatically suspend immediate publishing at or above 120%.
- Do not stop, cancel, or rewrite existing scheduled posts when the workspace crosses a threshold.
- Do not merge the existing Free quota service into the paid policy in this change.
- Do not add a feature flag.
- Do not change platform rate limits, queue limits, abuse controls, or upstream platform quotas.

## Recommended Architecture

Create an independent paid quota policy service. It owns paid-plan usage snapshots, schedule admission decisions, threshold evaluation, and the paid quota notification ledger.

The existing Free quota implementation remains separate. Both paths may reuse shared database queries, the email registry, the audited Loops client, email preferences, and Admin email reporting.

```text
Schedule write request
    |
    v
Paid quota policy admission
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

This separation keeps paid soft-overage policy distinct from Free hard-cap semantics and avoids turning the existing Free quota email service into a multi-plan rules engine.

## Monthly Usage Model

### Effective usage

For one workspace and UTC calendar month:

```text
effective_usage(period)
  = completed_posts(period)
  + active_scheduled_units(period)
```

- `completed_posts` is the existing successful platform-publish usage count for the actual UTC month in which publishing succeeded.
- `active_scheduled_units` counts undeleted platform destinations whose parent post remains scheduled for that UTC month.
- One parent post scheduled to Instagram, TikTok, and LinkedIn consumes three scheduled units.
- Draft, cancelled, deleted, failed, partial, and already-published parent posts do not reserve scheduled units.
- A scheduled unit is associated with the UTC month of `scheduled_at`.

When a scheduled result publishes successfully, it stops being an active scheduled unit and becomes completed usage. If delayed execution crosses a UTC month boundary, the reservation leaves the scheduled month and the completion is charged to the actual successful-publish month.

### Percentage

Threshold comparisons use exact integer arithmetic:

```text
effective_usage * 100 >= threshold * post_limit
```

Display percentages may be rounded for UI copy, but rounding must not affect admission or threshold decisions.

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
- Bulk schedule creation
- Draft-to-scheduled transition
- Immediate-to-scheduled transition
- Adding platform destinations to a scheduled post
- Moving a scheduled post into another month
- MCP and CLI flows that call the same API
- Future schedule-writing entry points

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

After acquiring the lock, the transaction recalculates completed and scheduled usage before making the final decision. Bulk requests are atomic for quota admission: if the accepted request would exceed the target month's quota, none of its scheduled units are created.

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
    "code": "PLAN_SCHEDULED_POST_QUOTA_EXCEEDED",
    "normalized_code": "plan_scheduled_post_quota_exceeded",
    "message": "Monthly scheduling quota reached. Upgrade your plan, cancel existing scheduled posts, or wait until the quota resets. Immediate publishing remains available.",
    "details": {
      "completed_posts": 2488,
      "scheduled_posts": 12,
      "effective_usage": 2500,
      "post_limit": 2500,
      "requested_posts": 1,
      "period": "2026-07",
      "resets_at": "2026-08-01T00:00:00Z",
      "upgrade_recommended": true,
      "immediate_publishing_allowed": true
    }
  }
}
```

This error must remain distinct from `PLAN_SCHEDULED_POST_LIMIT_EXCEEDED`, which represents the Free plan's active scheduled parent-post count cap.

### Response headers

Preserve the current meaning of `X-UniPost-Usage` as completed usage. Add:

```http
X-UniPost-Usage: 2488/2500
X-UniPost-Scheduled-Usage: 12
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
  "effective_usage": 2500,
  "post_limit": 2500,
  "percentage": 99.52,
  "effective_percentage": 100,
  "warning": "scheduled_quota_reached",
  "scheduling_allowed": false,
  "resets_at": "2026-08-01T00:00:00Z",
  "plan": "basic"
}
```

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

### Threshold evaluation events

Evaluate affected periods after:

- A successful platform publish increments completed usage.
- A scheduled post or platform unit is created.
- A scheduled post is cancelled or deleted.
- A scheduled post changes month.
- Platform destinations are added or removed.
- A draft becomes scheduled.
- A subscription changes plan.
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

A unique constraint on `(workspace_id, period, threshold_percent)` enforces the monthly decision rule.

The ledger owns threshold state and retry scheduling. The unified `email_send_attempts` table remains the provider-send audit surface.

## Rollout and Reconciliation

The schedule admission policy becomes authoritative as soon as the production code is active. It does not cancel existing scheduled posts, including workspaces whose current effective usage is already above the plan limit.

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

Run the same reconciliation at the start of each UTC month so scheduled units created before the month began are evaluated even when no schedule mutation occurs at the boundary. A daily idempotent reconciliation may also run as a safety net for missed business-event evaluations; it must produce no additional email after a threshold has a final decision.

## Email Delivery

### Template

Use one Loops transactional template for the paid quota sequence. Variables control:

- Subject and preview text
- Warning, Alert, or Critical Alert presentation
- Workspace and owner names
- Plan name
- Threshold reached
- Completed, scheduled, effective, limit, percentage, and remaining scheduling capacity
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
- Threshold time
- Recent activity context needed for support or sales outreach
- Open and resolved timestamps

Crossing above 120% sends no additional quota-threshold email.

Upgrading to a plan whose recalculated effective usage is below 100% automatically resolves the open follow-up. Otherwise the follow-up remains available for manual resolution.

The follow-up does not itself restrict immediate publishing. Any stronger account action requires an explicit administrative decision outside this policy.

## Plan Changes and Resets

### Upgrade

- Recalculate against the new limit immediately.
- Restore schedule admission when effective usage falls below 100%.
- Preserve all existing scheduled posts and notification history.
- Resolve the 120% follow-up if the upgraded quota brings usage below 100%.

### Downgrade

- Recalculate against the new limit immediately.
- If effective usage is at or above 100%, prevent new scheduled units immediately.
- If the change crosses multiple unsatisfied thresholds, send only the highest reached and mark lower ones `skipped_superseded`.
- If the new percentage is at or above 120%, send the 120% Critical Alert and create the follow-up.

### Administrative completed-usage reset

- Recalculate the affected period after the reset.
- Restore scheduling if effective usage falls below 100%.
- Do not delete or reopen threshold decisions already finalized for that month.

## Dashboard Experience

### Billing and usage

Show the components of effective usage separately:

- Successfully published
- Currently scheduled
- Effective usage
- Plan limit

At 80% through below 100%, show a yellow warning. At or above 100%, show a red alert:

> New scheduled posts are paused for this month. Existing scheduled posts and immediate publishing remain available.

### Schedule actions

The Dashboard may proactively disable schedule actions using the usage response, but the backend remains authoritative.

On quota rejection, offer:

- Upgrade plan
- Manage or cancel scheduled posts
- Publish immediately

Editing actions that do not add net scheduled units remain available.

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
- Open paid quota follow-ups

## Error Handling

- Quota snapshot failures fail closed for schedule-increasing paid-plan mutations because accepting unbounded scheduled work would violate the new policy.
- Cancellation, deletion, and net quota-releasing edits must remain available when possible; if their transaction cannot safely calculate the change, return a normal internal error without partially mutating data.
- Email evaluation or delivery failure is logged and audited but does not roll back the completed business mutation.
- Missing owner email produces an auditable terminal skip or failure reason rather than blocking quota policy.
- Unknown, unlimited, or contract-controlled plans do not enter the paid self-serve notification sequence.

## Testing

### Unit tests

- Plan eligibility for API, Basic, Growth, Free, Team, and Enterprise
- Effective usage arithmetic
- Exact 80/90/100/105/110/115/120 comparisons without display rounding
- Exactly 100% accepted and any projected value above 100% rejected
- Immediate publishing remains allowed at 100% and 120%
- Net-zero and quota-releasing edits remain allowed
- Cross-month release and reservation
- Highest-threshold-only selection
- Monthly deduplication after usage falls and rises
- Warning preference skips and required alerts
- Plan upgrade, downgrade, and administrative reset decisions

### Database and concurrency tests

- Advisory-lock contract for workspace and period
- Concurrent schedule requests cannot exceed the target month's limit
- Multi-period locks are acquired in stable order
- Bulk admission is atomic
- Unique threshold constraint prevents duplicate decisions
- Worker claims are disjoint across instances
- Expired processing leases recover safely

### Handler and contract tests

- Dashboard/API schedule creation
- Bulk schedule creation
- Draft-to-scheduled
- Immediate-to-scheduled
- Adding scheduled destinations
- Removing destinations
- Same-month edits
- Cross-month rescheduling
- Error code, normalized code, details, and headers
- Additive `GET /v1/usage` response fields
- Existing Free hard-cap and active-scheduled-cap contracts remain unchanged

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

- Billing displays completed, scheduled, and effective usage
- Warning and Alert states render correctly
- Schedule actions disable proactively at 100%
- Quota error offers upgrade, schedule management, and immediate publish actions
- Quota-releasing edits remain accessible

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
- Dashboard Warning and Alert behavior
- Loops notification audit and Admin visibility

The task is not complete until the development deployment succeeds and the expected behavior is personally verified on the development domains.

## Acceptance Criteria

- API, Basic, and Growth effective usage equals completed plus active scheduled platform units for each UTC month.
- New scheduled units cannot make projected effective usage exceed the target month's plan limit.
- Existing scheduled posts and immediate publishing continue at and above 100%.
- All schedule-writing entry points return the same 402 error contract.
- 80% and 90% Warning emails respect the quota-email preference.
- 100%, 105%, 110%, 115%, and 120% alerts are required service notices.
- A single state change sends only the highest newly reached threshold.
- Each workspace, period, and threshold has one final decision.
- Email delivery is asynchronous, audited, idempotent, and retried without affecting business requests.
- Reaching 120% creates one administrative follow-up and no later threshold email.
- Plan changes and cancellations immediately recalculate schedule availability without removing existing scheduled content.
- Dashboard, API, Bulk API, MCP, and CLI behavior remain consistent.
- Free, Team, and Enterprise behavior remains outside the new paid self-serve policy.
