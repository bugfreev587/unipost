# PRD - Free plan quota reminder emails

**Status:** Planning
**Owner:** Billing / Growth lifecycle / Notifications
**Created:** 2026-06-22
**Target:** Automated Free plan quota reminder emails delivered through Loops

---

## Problem

Free plan workspaces can now be hard-blocked once monthly post quota is exhausted, but the customer warning path is too thin.

Today, users can discover the limit only when they look at billing usage or when a publish request is rejected. That creates three problems:

- Users who are approaching the quota do not get enough warning to upgrade before work is interrupted.
- Users whose scheduled posts reserve quota may be blocked earlier than they expect from completed-post usage alone.
- Users who have already crossed the quota need a clear explanation that posting is blocked until the monthly quota resets or they upgrade.

UniPost needs a calm, deterministic quota email sequence for Free plan workspaces that nudges upgrades before interruption and explains the block once it happens.

## Product direction

UniPost should own quota evaluation and delivery idempotency. Loops should own the customer-facing email rendering and delivery.

The backend is the source of truth for:

- workspace plan
- current quota period
- completed usage
- scheduled quota reservations
- hard-block state
- whether a threshold email has already been sent

Loops is the system of record for:

- email template content
- rendered transactional email
- send metrics and provider-level delivery status

This should not be a pure Loops segmentation workflow where Loops infers quota state from contact properties. UniPost must compute the state and trigger Loops only when a threshold becomes eligible.

## Goals

1. Notify Free plan workspace owners when quota usage reaches 80%.
2. Notify again as usage increases by each additional 5 percentage points after 80%.
3. Send a stronger 95% warning that the workspace is about to be blocked.
4. Send a warning-style 100% email explaining that new posting is blocked and the quota resets on the first day of the next month.
5. Include an upgrade recommendation and pricing page link in every quota reminder email.
6. Avoid duplicate emails for the same workspace, quota period, and threshold.
7. Account for scheduled post reservations when determining whether the user is near block.
8. Keep paid plan users out of this sequence.
9. Keep the system safe to roll back with a feature flag.

## Non-goals

- No change to Free plan quota size.
- No change to paid plan soft-overage behavior.
- No automatic upgrade, billing checkout, or payment flow changes.
- No SMS, Slack, Discord, or in-app notification delivery in v1.
- No marketing campaign blasts to all Free users.
- No pure Loops-side quota calculation.
- No production release from this PRD alone.

## Current codebase findings

### Quota

The quota package already exposes Free plan quota state and hard-block logic.

- `api/internal/quota/checker.go` calculates usage percentage for a workspace and period.
- Latest `origin/dev` includes `Reserved` scheduled quota units in `QuotaStatus`.
- Free plan hard block uses `usage + reserved + additionalPosts > limit`.
- Quota periods are UTC calendar months in `YYYY-MM` format.

The reminder system should use the same effective usage concept as the hard-block gate:

```text
effective_usage = completed_post_usage + scheduled_reserved_quota_units
effective_percentage = effective_usage / monthly_post_limit * 100
```

Emails may show both:

- posted usage, for familiarity
- scheduled reserved usage, when it materially affects the warning

### Notifications

UniPost already has notification infrastructure for `billing.usage_80pct`, but the searched code path did not show an active publisher for that event.

The existing notification worker currently renders billing usage email copy in backend code and sends through the configured `mail.Mailer`.

### Loops

UniPost already has a Loops client that can:

- upsert contacts
- send events
- send transactional emails
- attach an idempotency key

The existing Loops lifecycle syncer is behind `email.loops_integration_v1` and already handles lifecycle-style user emails.

## Trigger model

### Eligible users

Send quota reminder emails only when all of the following are true:

- The workspace's current plan is `free`.
- The workspace has a monthly post limit greater than zero.
- The workspace owner has a usable dashboard email.
- The quota reminder feature flag is enabled for the target environment.
- Loops is configured for the environment.
- The workspace has not already received the same threshold email in the same quota period.

Do not send quota reminder emails when:

- The workspace is on any paid plan.
- The workspace is unlimited.
- The workspace owner email is missing.
- The workspace has been deleted or is otherwise not send-eligible.
- The threshold was already sent for the current period.

### Thresholds

For v1, the threshold sequence is:

```text
80%, 85%, 90%, 95%, 100%
```

The 80%, 85%, and 90% emails use a "heads up" tone.

The 95% email uses a stronger "almost blocked" tone.

The 100% email uses a warning-style "blocked now" tone.

### Crossing behavior

Threshold eligibility is based on crossing into a threshold band.

Examples:

| Previous effective percentage | New effective percentage | Email to send |
| --- | --- | --- |
| 76% | 80% | 80% |
| 82% | 86% | 85% |
| 86% | 91% | 90% |
| 91% | 96% | 95% |
| 96% | 100% | 100% |
| 70% | 92% | 90% only |
| 78% | 101% | 100% only |

When a single usage update crosses multiple unsent thresholds, send only the highest crossed threshold. This prevents a batch publish or scheduled reservation update from generating several emails at once.

### Monthly reset behavior

Threshold state resets with the monthly quota period.

Because the quota period is `YYYY-MM` in UTC, the 100% email should say:

```text
Your Free plan quota resets on the first day of the next month.
```

If the product later wants local-time precision, add a separate PRD to choose a timezone contract and update the quota period implementation.

### Upgrade behavior

All quota reminder emails must include:

- a recommendation to upgrade
- the pricing page link
- a dashboard billing link when available

Use this production pricing URL in customer emails:

```text
https://unipost.dev/pricing
```

Use environment-specific app links for dashboard billing:

```text
{APP_BASE_URL}/settings/billing
```

## Delivery architecture

### Recommended v1 architecture

Use UniPost backend threshold detection plus Loops transactional email.

Flow:

1. A publish, scheduled publish reservation, or scheduled quota scan recalculates Free plan quota state.
2. Backend computes effective usage percentage.
3. Backend maps the percentage to the highest eligible threshold.
4. Backend checks the threshold send ledger for `(workspace_id, period, threshold)`.
5. Backend creates a pending ledger row before provider delivery.
6. Backend sends a Loops transactional email with deterministic idempotency.
7. Backend marks the ledger row as `sent` or `failed`.
8. Failed sends remain retryable without sending duplicate emails after an ambiguous provider timeout.

### Why transactional email instead of a Loops workflow

Quota reminders are service notices tied to hard-block behavior, not nurture campaigns.

Transactional send is preferred because it gives UniPost:

- deterministic threshold choice
- a clear transactional template ID
- backend-controlled idempotency keys
- simple retry semantics
- predictable behavior even if Loops workflow branching changes

Loops workflows may still be useful later for broader lifecycle campaigns, but v1 should keep the quota warning path deterministic.

### Loops configuration

Add a dedicated transactional template:

```text
LOOPS_FREE_PLAN_QUOTA_REMINDER_TRANSACTIONAL_ID
```

The template should support these data variables:

```text
workspace_name
threshold_percent
usage_percent
posts_used
posts_reserved
posts_limit
remaining_posts
reset_date_label
pricing_url
billing_url
email_variant
headline
body
cta_label
secondary_cta_label
```

The backend should pass fully written `headline`, `body`, and CTA labels so the Loops template can remain a stable visual shell. That keeps behavioral copy versioned in code and makes tests practical.

If the Loops transactional ID is missing while the feature flag is enabled, the backend should fail closed for sends and log a clear configuration error. It should not silently fall back to a marketing workflow.

## Data and idempotency

Add a ledger table or equivalent durable store for quota reminder sends.

Suggested table:

```text
free_plan_quota_email_notifications
```

Suggested fields:

- `id`
- `workspace_id`
- `owner_user_id`
- `recipient_email`
- `period`
- `threshold_percent`
- `effective_usage`
- `completed_usage`
- `reserved_usage`
- `limit`
- `status`: `pending`, `sent`, `failed`, `skipped`
- `provider`: `loops`
- `provider_idempotency_key`
- `provider_error`
- `sent_at`
- `created_at`
- `updated_at`

Required unique constraint:

```text
UNIQUE (workspace_id, period, threshold_percent)
```

Provider idempotency key:

```text
free_plan_quota:{workspace_id}:{period}:{threshold_percent}
```

This is shorter than Loops' 100-character idempotency-key limit if workspace IDs are UUIDs. If a future workspace ID format grows too long, hash the workspace ID portion.

## Email content requirements

### Tone

Emails should be:

- short
- plain-spoken
- calm
- action-oriented
- clear that Free plan posting is constrained by monthly quota
- helpful rather than punitive

Do not:

- blame the user
- use dark-pattern urgency
- imply paid plans are required to keep existing data
- claim reset timing more precisely than the quota system supports
- mention internal provider details like Loops, Railway, Vercel, or feature flags

### Shared structure

Each email should have:

1. Subject
2. Short headline
3. One paragraph explaining usage state
4. One paragraph recommending upgrade
5. Primary CTA to pricing
6. Secondary link to billing or usage
7. Plain-text fallback

### Variant: 80%, 85%, 90%

Use this variant for threshold emails below 95%.

Subject pattern:

```text
[UniPost] You've used {threshold_percent}% of your Free plan posts
```

Headline:

```text
You're at {threshold_percent}% of your Free plan quota
```

Body:

```text
Your workspace has used {posts_used} of {posts_limit} Free plan posts this month.
```

If reserved scheduled posts are present:

```text
You also have {posts_reserved} scheduled posts reserved, so your effective usage is {usage_percent}%.
```

Upgrade paragraph:

```text
Upgrade when you're ready to keep publishing without the Free plan monthly cap.
```

Primary CTA:

```text
View plans
```

Secondary CTA:

```text
Review usage
```

Plain text example:

```text
You're at {threshold_percent}% of your Free plan quota.

Your workspace has used {posts_used} of {posts_limit} Free plan posts this month.
{reserved_line}

Upgrade to keep publishing without the Free plan monthly cap:
{pricing_url}

Review usage:
{billing_url}
```

### Variant: 95%

Subject:

```text
[UniPost] You're almost at your Free plan post limit
```

Headline:

```text
You're close to being blocked
```

Body:

```text
Your workspace has reached {usage_percent}% of its Free plan monthly post quota. Once it reaches 100%, new publish requests will be blocked until next month's reset unless you upgrade.
```

Upgrade paragraph:

```text
Upgrade now to avoid an interruption and keep scheduled publishing moving.
```

Primary CTA:

```text
Upgrade before you're blocked
```

Secondary CTA:

```text
Review usage
```

Plain text example:

```text
You're close to being blocked.

Your workspace has reached {usage_percent}% of its Free plan monthly post quota. Once it reaches 100%, new publish requests will be blocked until next month's reset unless you upgrade.

Upgrade now to avoid an interruption:
{pricing_url}

Review usage:
{billing_url}
```

### Variant: 100%

Subject:

```text
[UniPost] Warning: Free plan posting is now blocked
```

Headline:

```text
Free plan posting is blocked
```

Body:

```text
Your workspace has reached 100% of its Free plan monthly post quota. New publish requests are blocked for this workspace until your quota resets on the first day of next month.
```

If scheduled reservations contributed to the block:

```text
Scheduled posts can reserve quota before they publish, so your effective usage may include upcoming scheduled posts.
```

Upgrade paragraph:

```text
Upgrade to resume publishing immediately and get a higher monthly post limit.
```

Primary CTA:

```text
Upgrade to resume publishing
```

Secondary CTA:

```text
Review usage
```

Plain text example:

```text
Warning: Free plan posting is blocked.

Your workspace has reached 100% of its Free plan monthly post quota. New publish requests are blocked for this workspace until your quota resets on the first day of next month.
{reserved_block_line}

Upgrade to resume publishing immediately and get a higher monthly post limit:
{pricing_url}

Review usage:
{billing_url}
```

## Backend requirements

### Threshold detector

Add a backend component that can be called from quota-changing paths and from a scheduled safety scan.

The detector should accept:

- workspace ID
- quota period
- completed usage
- reserved scheduled usage
- limit
- plan ID
- owner user ID and email

The detector should return:

- no-op
- threshold send candidate
- skipped reason

### Trigger locations

V1 should evaluate reminders in at least these places:

1. After completed post usage increments.
2. After scheduled posts create or update quota reservations.
3. In a periodic worker scan as a safety net for any missed event path.

The periodic worker should only send missing threshold emails for the current period and should obey the same idempotency ledger.

### Send behavior

The send path should:

1. Resolve workspace plan and owner email at send time.
2. Recompute quota status before sending.
3. Skip if workspace is no longer Free.
4. Skip if threshold no longer applies.
5. Insert or claim the ledger row before provider send.
6. Send Loops transactional email with idempotency key.
7. Mark success or failure.
8. Keep failed sends retryable.

Provider failures must not block publishing. They should be logged and retried by the worker.

### Feature flag

Add a feature flag:

```text
billing.free_plan_quota_email_reminders_v1
```

Recommended defaults:

```text
development: on after Loops test template is configured
production: off until delivery is verified
fallback: off
```

Owner area:

```text
Billing / Growth lifecycle / Notifications
```

Rollback:

```text
Disable billing.free_plan_quota_email_reminders_v1. The backend should stop creating new reminder sends while preserving existing ledger rows.
```

Third-party dependency:

```text
Loops API availability and LOOPS_FREE_PLAN_QUOTA_REMINDER_TRANSACTIONAL_ID configuration.
```

## Dashboard requirements

No new dashboard page is required in v1.

However, emails link to existing billing or usage surfaces:

- pricing page: `https://unipost.dev/pricing`
- dashboard billing: `{APP_BASE_URL}/settings/billing`

If the current billing route is workspace-specific in the final implementation, the backend should use that route instead of the account-level fallback.

## Analytics and observability

Emit structured logs for:

- threshold detected
- threshold skipped
- ledger insert conflict
- Loops send requested
- Loops send succeeded
- Loops send failed
- retry scheduled

Suggested event names:

```text
free_plan_quota_email_detected
free_plan_quota_email_sent
free_plan_quota_email_failed
free_plan_quota_email_skipped
```

Suggested log fields:

- `workspace_id`
- `owner_user_id`
- `period`
- `threshold_percent`
- `completed_usage`
- `reserved_usage`
- `effective_usage`
- `limit`
- `provider`
- `error`

## Privacy and compliance

Quota reminder emails may include:

- workspace name
- usage counts
- quota limit
- reset timing
- pricing and billing links

Quota reminder emails must not include:

- API keys
- access tokens
- refresh tokens
- social account identifiers beyond user-facing names
- post content
- provider payloads
- internal worker or deployment details

These are service emails related to product usage and account limits. They should still include a link to notification settings if UniPost's global email footer requires it, but they should not be treated as marketing blasts.

## Rollout plan

### Phase 1 - PRD and template approval

- Approve this PRD.
- Approve final email copy.
- Create the Loops transactional template in the development Loops environment.
- Configure `LOOPS_FREE_PLAN_QUOTA_REMINDER_TRANSACTIONAL_ID` in development.

### Phase 2 - Backend implementation behind flag

- Add the send ledger.
- Add the threshold detector.
- Add Loops transactional send support for this template.
- Add tests for threshold crossing and idempotency.
- Add the scheduled safety scan.
- Document the feature flag in `docs/feature-flags-unleash.md`.

### Phase 3 - Development validation

- Enable the flag in development.
- Use test workspaces to cross 80%, 85%, 90%, 95%, and 100%.
- Verify Loops receives one send per workspace, period, and threshold.
- Verify a jump from below 80% to above 95% sends only the highest crossed threshold.
- Verify the 100% email explains block and reset.
- Verify paid workspaces do not receive quota reminders.

### Phase 4 - Production rollout

- Configure the production Loops transactional ID.
- Keep production flag off.
- Run a dry-run scan to count eligible recipients by threshold.
- Review sample recipients before enabling.
- Enable for a small production cohort.
- Monitor sends, failures, and support replies.
- Expand rollout if delivery and copy quality are acceptable.

## Testing requirements

### Unit tests

Add tests for:

- 79% to 80% sends 80%.
- 80% to 84% sends nothing new.
- 84% to 85% sends 85%.
- 89% to 90% sends 90%.
- 94% to 95% sends 95%.
- 99% to 100% sends 100%.
- 70% to 92% sends 90% only.
- 78% to 101% sends 100% only.
- Re-running the detector for the same workspace, period, and threshold does not create a second send.
- Paid plans are skipped.
- Missing owner email is skipped.
- Reserved scheduled posts are included in effective percentage.
- Failed provider attempts can retry without creating a second threshold row.

### Integration tests

Add tests for:

- Loops transactional payload includes required variables.
- Loops idempotency key is deterministic and under the provider length limit.
- Missing transactional ID returns a configuration error in the send worker.
- The scheduled safety scan does not duplicate sends already created by publish-time detection.

### Manual development acceptance

In the development environment:

1. Create or seed a Free workspace at 79%.
2. Cross 80% and verify the 80% email.
3. Cross 85% and verify the 85% email.
4. Cross 90% and verify the 90% email.
5. Cross 95% and verify the "almost blocked" copy.
6. Cross 100% and verify the warning-style blocked copy.
7. Confirm pricing link opens the pricing page.
8. Confirm billing link opens the development dashboard billing page.
9. Confirm no duplicate email is sent after refreshing or retrying the detector.
10. Confirm a paid workspace does not receive reminders.

## Acceptance criteria

1. Free plan quota reminder emails are sent at 80%, 85%, 90%, 95%, and 100%.
2. The 95% email clearly says the workspace is close to being blocked.
3. The 100% email clearly says Free plan posting is blocked and resets on the first day of the next month.
4. Every email recommends upgrading and links to `https://unipost.dev/pricing`.
5. Emails are sent at most once per workspace, period, and threshold.
6. Usage jumps send only the highest crossed threshold email.
7. Scheduled reserved quota units count toward effective usage for warning eligibility.
8. Paid workspaces do not receive Free plan quota reminders.
9. Provider failures do not block publishing.
10. The feature can be disabled through `billing.free_plan_quota_email_reminders_v1`.
11. Development deployment is validated with real Loops sends or Loops test recipients before reporting implementation complete.

## V1 decisions

1. Email copy uses "posts" instead of "publish requests" because customer-facing pricing is post-oriented.
2. Dashboard billing links should use the most accurate existing billing route available at implementation time. If a workspace-specific billing route exists, prefer it over account-level `/settings/billing`.
3. The 100% email should be sent when effective usage reaches 100%, even if scheduled reservations are what push the workspace to the block threshold.
4. V1 sends quota reminders to the workspace owner only.
5. A downgrade from paid to Free mid-period should immediately evaluate quota reminders using the current period's effective usage.
