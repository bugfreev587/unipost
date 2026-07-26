# TikTok Free-Plan Publishing Restrictions PRD

**Status:** Confirmed design; self-reviewed after external-review disposition; awaiting user approval for implementation planning
**Date:** 2026-07-26
**Canonical source of truth:** This document supersedes all duplicated design notes for this feature.
**Owner areas:** Publishing, Admin, Dashboard, Media Retention, Lifecycle Email
**Branch:** `codex/staging-tiktok-free-publishing-restriction`
**Base:** `origin/staging` at `c9b3727d8e527e3cd88738d9608b8c010e195c0e`

## 1. Executive summary

UniPost must add a generic, database-backed Admin Publishing Restrictions center. Its first policy temporarily blocks TikTok publishing for Free workspaces while TikTok's app-level active-creator capacity is constrained. Paid plans and other platforms remain available.

The restriction is an operational publishing policy with richer state than UniPost's existing database-backed boolean feature flags. The backend is authoritative when publish work is admitted and again immediately before a platform call. A blocked Free/TikTok target never reaches TikTok and never receives an automatic retry. Mixed-platform posts continue for every unrestricted target and fail only the TikTok result.

The same policy failure is visible in the Composer, Posts List, and Calendar. Policy-failed media remains in R2 for 60 days through UniPost's existing `media_post_usages` business-retention ledger. After the restriction is disabled or the workspace becomes Paid, the customer may personally retry only the failed TikTok result through the existing Retry endpoint and UI, provided the media still exists. Admin never bulk-retries and disabling the restriction never replays old posts.

Restriction and recovery emails are separate manual Admin campaigns and a separately deliverable implementation milestone within this PRD. A toggle, migration, deployment, or recovery transition never sends email. Each campaign requires preview, recipient count, and a second irreversible confirmation, then proceeds through a durable, idempotent worker with failed-recipient retry. The implementation must reuse UniPost's existing audited transactional-email infrastructure and must not grow into a general marketing-campaign system.

## 2. Investigation context and capacity rationale

### 2.1 TikTok capacity model

TikTok's Direct Post guidelines impose a 24-hour active-creator cap on each API client. TikTok separately applies a per-creator posting cap, typically around 15 posts per creator per day; that separate per-creator cap is not the policy addressed by this PRD. See [TikTok Direct Post API developer guidelines](https://developers.tiktok.com/doc/content-sharing-guidelines).

The completed UniPost investigation confirmed this against production shared-client traffic: repeated `reached_active_user_cap` failures occurred with exactly **100 distinct successful TikTok creators in the preceding rolling 24 hours**. At the target failure, those occupied slots comprised 94 Team accounts and 6 Free accounts. Connected-account demand at investigation time was:

- Team: 135 accounts across 2 workspaces;
- Free: 35 accounts across 26 workspaces;
- Basic: 4 accounts across 3 workspaces.

In this PRD, “100 rolling active users” means UniPost's observed/current app-level allocation. It is not a universal TikTok limit for every API client. The evidence establishes that the capacity was real, production-facing, shared, and binding; the PRD makes no unverified claim about UniPost's TikTok audit or sandbox status.

### 2.2 Why restrict Free publishing

The capacity is shared across UniPost workspaces. Each distinct TikTok creator that publishes through the UniPost API client can consume one of the rolling active-creator slots. When capacity is exhausted, additional TikTok publishing cannot be admitted reliably even though connected accounts, media, saved posts, and other platforms remain healthy.

UniPost has asked TikTok to increase the allocation. Until TikTok grants more capacity, the operational policy prioritizes the limited publishing capacity for paying customers by temporarily blocking TikTok publishing on Free. Based on the observed target-failure mix, this could free the 6 slots occupied by Free creators. It cannot guarantee all paid publishing because Team plus Basic connected demand alone (139 accounts at investigation time) can exceed the observed 100-creator allocation. A TikTok capacity increase therefore remains necessary in parallel; the restriction is a prioritization control, not a complete capacity fix or a promise that Paid publishing always succeeds.

**Review disposition:** the production failure evidence resolves the review's concern that the capacity premise might be hypothetical. The reviewer was correct that restricting Free cannot guarantee enough capacity for all Paid demand, and that limitation is now explicit.

### 2.3 Investigation conclusions carried into the design

- The problem is platform capacity, not invalid customer content or an account disconnect.
- The restriction must be reversible without a deployment.
- The restriction must be enforced server-side for API, scheduled, and retry traffic.
- Existing scheduled content must never wait silently and surprise-publish after recovery.
- Paid plans and non-TikTok destinations must continue.
- Customer accounts and saved content must remain intact.
- Customer communication must be explicit and manually controlled, never coupled to the toggle.

## 3. Goals

1. Provide one extensible Admin center for platform-and-plan publishing restrictions.
2. Restrict TikTok publishing only for the Free plan while the initial restriction is enabled.
3. Apply one centralized policy to API create/admission, immediate posting, scheduled dispatch, and manual result Retry.
4. Recheck policy in the delivery worker immediately before any TikTok call.
5. Preserve partial success for mixed-platform posts.
6. Use one exact error contract and one exact customer message everywhere.
7. Deterministically fail existing scheduled Free/TikTok posts that become due during the restriction.
8. Allow only customer-triggered, result-level Retry after policy or plan eligibility changes.
9. Retain policy-failed media for 60 days through `media_post_usages` with race-safe cleanup behavior.
10. Provide manual restriction and recovery campaigns with exact copy, correct audience, durable progress, and idempotency.
11. Default to disabled and introduce no email or publishing side effect at deployment.

## 4. Scope and non-goals

### 4.1 In scope

- Generic platform restriction data and Admin page, initialized with TikTok plus restricted plan Free.
- Database policy evaluation and optimistic Admin state mutation.
- API and result error contracts.
- Immediate, scheduled, queued-worker, mixed-platform, and manual Retry behavior.
- Composer, Posts List, Calendar, shared result-details, and Admin interactions.
- Policy-specific R2 business retention through `media_post_usages`.
- Separate manual restriction and recovery email campaigns.
- Audit events, concurrency controls, metrics, tests, and a safe disabled rollout.

### 4.2 Non-goals

- Do not represent the restriction as another boolean row in the existing `feature_flags` table; its plan, reason, cycle, operational, and communication state requires a dedicated domain while reusing existing Admin patterns.
- No initial restriction for Paid plans or non-TikTok platforms.
- No TikTok account disconnect, credential mutation, saved-post deletion, or draft deletion.
- No Cloudflare R2 Object Lifecycle policy for this retention rule.
- No automatic email from deployment, migration, enable, or disable.
- No automatic post replay when the restriction is disabled or a workspace upgrades.
- No Admin bulk Retry, Admin impersonated Retry, or recovery campaign that enqueues posts.
- No change to the separate TikTok per-creator posting cap.
- No general-purpose marketing-campaign framework, arbitrary campaign builder, audience segmentation product, campaign scheduling suite, A/B testing, or cross-channel messaging system.
- No production release, deployment, or external email send implied by implementation.

## 5. Product policy

### 5.1 Restriction decision

A target is restricted only when all conditions are true:

1. the database restriction row for the resolved platform is enabled;
2. the server-resolved platform is `tiktok` for the initial rule;
3. the workspace's current plan is in `restricted_plan_ids`, initially only `free`.

The evaluator reads the current `subscriptions.plan_id`. A missing subscription is treated conservatively as Free, matching existing plan-policy behavior. Platform identity comes from the workspace-owned social-account row, never from untrusted request JSON.

Paid plans bypass the initial policy. A workspace that upgrades after a policy failure becomes eligible for customer Retry even if the global Free/TikTok restriction remains enabled.

### 5.2 Central policy service

Create a focused `publishingrestrictions` domain with:

- a PostgreSQL store;
- one evaluator used by publishing paths;
- an Admin mutation service that owns versioning and audit events;
- a separate campaign service and worker.

Handlers and workers consume a decision containing restricted state, platform, plan, reason, cycle, version, exact message, and next action. They do not duplicate plan arrays or message logic.

There is no cross-request in-memory cache in the first version. Request-local reuse is allowed, but every independent delivery-worker attempt performs a fresh database-backed evaluation so operator changes do not depend on cache expiry.

### 5.3 Relationship to the existing feature-flag system

Verified `origin/staging` at `c9b3727d8e527e3cd88738d9608b8c010e195c0e` uses PostgreSQL `feature_flags` and append-only `feature_flag_changes`; mutations lock the flag row, record the actor and before/after boolean transition in one transaction, and Admin access uses the existing `SuperAdminChecker`. The current feature-flag schema and API do **not** have an explicit version column or `expected_version` conflict contract.

Publishing restrictions require restricted-plan arrays, reason/message state, activation cycles, timestamps, affected-workspace/account metrics, and correlation to the two confirmed email types. Extending a boolean flag row with all of that would make the feature-flag domain less coherent. Use a dedicated restriction schema, but reuse the established PostgreSQL transaction/row-lock approach, append-only change-audit approach, `SuperAdminChecker`, Admin handler conventions, and Admin UI confirmation patterns. Add the explicit restriction `version`/`expected_version` contract because this operational control needs visible optimistic concurrency; do not imply that this field already exists on `feature_flags`.

**Review disposition:** the stale provider comparison is removed. The design now compares against the DB-backed system that actually exists and identifies both what is reused and why the richer policy state remains separate.

### 5.4 Policy-read failure

If policy state cannot be read, TikTok must not be called:

- HTTP admission returns a retryable 503/internal-service response;
- a worker uses the existing infrastructure retry path for the internal read failure;
- the failure is not mislabeled as a publishing restriction;
- no policy email or 60-day policy deadline is created from a policy-read error.

This differs from a successful restricted decision, which is deterministic and receives no automatic retry.

## 6. Exact error contract

### 6.1 Canonical values

- API code: `PLAN_PLATFORM_PUBLISHING_RESTRICTED`
- Normalized and persisted result code: `plan_platform_publishing_restricted`
- HTTP status when every request target is blocked: `402 Payment Required`
- Failure stage: `publishing_policy`
- Error source: `unipost`
- Error temporality: `temporary`
- Automatic retriability: `false`
- Next action: `upgrade_or_wait_then_retry`
- Result title: `Publishing restricted`
- User message, exactly:

> TikTok publishing is temporarily unavailable on the Free plan due to platform capacity limits. We’re working with TikTok to increase capacity. Upgrade your plan or try again after the restriction is lifted.

HTTP 402 is intentional because the fully blocked request is plan-dependent and offers upgrade as one valid next action; the stable error code and message, rather than the status phrase alone, communicate that this is a temporary capacity policy rather than a billing failure.

### 6.2 Fully blocked response

If every publish target is restricted, create no parent post, result, delivery job, quota usage, or media-retention transition. Return this contract:

```json
{
  "error": {
    "code": "PLAN_PLATFORM_PUBLISHING_RESTRICTED",
    "normalized_code": "plan_platform_publishing_restricted",
    "message": "TikTok publishing is temporarily unavailable on the Free plan due to platform capacity limits. We’re working with TikTok to increase capacity. Upgrade your plan or try again after the restriction is lifted.",
    "next_action": "upgrade_or_wait_then_retry",
    "is_retriable": false,
    "error_source": "unipost",
    "error_temporality": "temporary",
    "details": {
      "platform": "tiktok",
      "plan_id": "free"
    }
  },
  "request_id": "req_8f3c1c2e"
}
```

`is_retriable=false` prevents automatic retries. A persisted failed result later exposes conditional manual eligibility through its dynamic `retry_policy`. The cycle ID is not customer-facing; keep it only in internal logs, `post_failures` safe metadata, restriction audit events, and campaign correlation.

### 6.3 Persisted policy failure

A policy-failed TikTok result has:

- `status=failed`;
- the exact message in `error_message`;
- `error_code=plan_platform_publishing_restricted`;
- `failure_stage=publishing_policy`;
- `error_source=unipost`;
- `error_temporality=temporary`;
- `is_retriable=false`;
- `next_action=upgrade_or_wait_then_retry`;
- no `provider_error`, `platform_error_code`, external ID, publish token, URL, or debug curl.

If a delivery job exists, it becomes terminal without a retry job. `post_failures` stores the same code, message, stage, and safe cycle metadata. Policy failures are not classified as TikTok/provider errors because no provider call occurred.

## 7. Publishing flows

### 7.1 Drafts

Saving a draft remains allowed because it does not admit publish work. Publishing an existing draft runs the same policy gate as a new publish. Existing draft content stays intact even when its TikTok target is currently unavailable.

### 7.2 User-safe policy projection

Add `GET /v1/publishing-restrictions`. It returns active restrictions applicable to the current workspace plan, including platform, plan, reason, exact message, and next action. The Dashboard uses it for preemptive disabling. It is advisory only; server admission and worker checks remain authoritative if the projection is stale or bypassed.

Posts List and Calendar both use the shared `CreatePostDrawer`, whose existing data effects are keyed to the drawer's `open` state. Fetch the projection whenever the drawer mounts/opens and again on window focus while it remains open. A successful focus refresh immediately updates disabled selections and Retry guidance. Submit always remains authoritative: a stale or failed projection fetch cannot permit a TikTok call, and the structured API error is rendered inline.

### 7.3 New immediate publish

After structural validation and server-side account resolution, partition targets into allowed and restricted sets.

- All restricted: return the exact HTTP 402 and persist nothing.
- Mixed: create the parent; create failed results immediately for restricted TikTok targets; create normal pending results/jobs only for allowed targets; return all result states with `202 Accepted`, matching the existing immediate-create contract for admitted queued work.
- None restricted: keep the current queue path.

Policy failures do not consume publish quota. Existing successful-publish quota accounting remains unchanged.

### 7.4 New scheduled publish

- TikTok-only Free schedule while active: HTTP 402 and no stored post.
- Mixed schedule: keep allowed destinations schedulable and retain the original per-target metadata needed for due-time results; return `201 Created`, matching the existing scheduled-create contract.
- At due time, the scheduler evaluates every target. A restricted TikTok target receives a failed result and no delivery job; allowed targets receive normal jobs.
- If the restriction is disabled or the workspace becomes Paid before due time, TikTok is allowed.

Due-time failure starts policy media retention at the actual failure time rather than schedule creation, so a far-future schedule cannot use up its 60-day retention before it becomes due.

### 7.5 Existing schedules

An existing scheduled Free/TikTok post that becomes due while the restriction is active fails deterministically with the canonical policy code. It never remains in a waiting state for automatic publication after the restriction is lifted. Other targets on the parent continue.

### 7.6 Already-enqueued work and worker recheck

A TikTok job may have been queued before an Admin enables the restriction or before a workspace plan changes. The delivery worker therefore evaluates policy after loading the current job, post, result, and account, and immediately before the platform-start marker and `publishOneContext`/TikTok adapter call.

If restricted, the worker marks the policy result/job terminal and returns without calling TikTok. Existing double-publish guards remain ahead of this check. A TikTok HTTP call that already began before the committed policy was observed is not canceled; the Admin confirmation states this boundary.

### 7.7 Mixed-platform parent state

- Allowed results continue independently.
- While allowed results are active, the parent stays publishing.
- An allowed success plus restricted TikTok failure produces `partial`.
- Stored legacy/due-time cases with only failed results produce `failed`.
- A fresh request with every target restricted never creates a parent.

## 8. Customer-triggered manual Retry

### 8.1 Existing endpoint and UI

Reuse without replacement:

`POST /v1/posts/{id}/results/{resultID}/retry`

Reuse the existing result-level Retry UI in Posts and Calendar. Retry remains a customer action. Admin never retries a result for the customer.

### 8.2 Eligibility

All existing requirements remain:

- result belongs to the workspace and parent;
- result is `failed`;
- no pending/running/retrying job exists for the result;
- original target and media can be resolved.

For the policy error:

- Free plus active restriction: `manual_retry_allowed=false`, `retry_state=blocked`, `reason=publishing_restriction_active`; API returns the unified policy error.
- Restriction disabled: manual Retry becomes available.
- Workspace upgraded to a non-restricted plan: manual Retry becomes available even while Free remains restricted.
- Media expired or missing: manual Retry is unavailable with `reason=media_reupload_required`; API creates no job and tells the customer to upload again.

### 8.3 Atomic retry enqueue

In one database transaction:

1. lock and recheck the failed result;
2. recheck existing active jobs and preserve the current unique-active-job rule;
3. re-evaluate current policy and plan;
4. verify referenced media still exists and is uploaded;
5. create exactly one retry job;
6. return the parent media usage to active/no cleanup deadline.

### 8.4 Dispatch and no automatic replay

The worker evaluates policy again immediately before TikTok. If policy applies again, the attempt becomes a new terminal policy failure, no automatic retry is created, and the 60-day media window restarts from the new failure time.

Disabling a restriction, upgrading a plan, sending a recovery notice, or deploying code never creates a job. The customer personally reviews and retries only the failed TikTok result.

## 9. R2 and media retention

### 9.1 Business ledger, not Cloudflare lifecycle

Use `media_post_usages`, which already protects shared objects through business references and `cleanup_after_at`. Do not add or change a Cloudflare R2 Object Lifecycle rule.

Add non-null `retention_reason` and backfill existing rows without changing their deadlines. Initial reasons:

- `active_post`: non-terminal or customer Retry active; `cleanup_after_at=NULL`;
- `plan_status`: normal plan/status retention;
- `publishing_restriction`: policy failure; failure time plus 60 days.

### 9.2 Policy failure deadline

At the actual policy-failure time, update every `media_post_usages` row for the parent to:

- the current derived post status;
- `retention_reason=publishing_restriction`;
- `cleanup_after_at=policy_failed_at + interval '60 days'`.

This overrides normal Free failed/partial retention. Another policy failure restarts the complete 60-day window. Result details expose the effective deadline as `media_retained_until`.

Current Free failed/partial retention is 48 hours, so 60 days is approximately a 30× extension of object lifetime for affected media and has a corresponding R2 storage-cost impact. Track retained object count, bytes, and projected cost by restriction cycle. The longer parent-wide deadline on a mixed/partial post intentionally overrides the successful non-TikTok result's shorter normal retention: retaining longer is safe, preserves one coherent parent media set for result-level Retry, and shared-media cleanup still waits for every usage.

### 9.3 Retry and terminal transitions

- Retry enqueue atomically sets `retention_reason=active_post` and clears the cleanup deadline.
- Retry success recalculates normal retention from success time using the workspace plan then in effect and sets `plan_status`.
- A non-policy terminal retry failure uses normal plan/status retention.
- Another policy failure sets a fresh 60-day policy deadline.

### 9.4 Cleanup race and shared media

The cleanup worker may claim an object only when every post/processing usage is due and no null/future deadline remains. Retry enqueue increments the existing `media.usage_version` while clearing the post-usage deadline. Cleanup claim compares that media-row version and locks the `media` row. Do not add a second `usage_version` to `media_post_usages`.

Therefore:

- retry wins: the object becomes active and an old cleanup snapshot cannot delete it;
- cleanup wins: retry detects deleted/missing media and returns `media_reupload_required` without creating a job;
- shared media remains protected by any other active or retained usage.

### 9.5 Expiry

After 60 days and successful cleanup, result history remains visible with `Publishing restricted`, the exact message, and expired `media_retained_until`. Because the existing Retry endpoint has no replacement-media payload, the customer must upload media again in a new or edited post.

## 10. Customer frontend

### 10.1 Composer locations and behavior

In both the standard Composer and Calendar Composer, while the current workspace is Free and the restriction is active:

- show the exact persistent inline notice directly beneath the TikTok account-selection area;
- disable active TikTok account choices and keep them unselected;
- exclude TikTok from Toggle All, preselection, and replay restoration;
- disable submit if stale local state still contains a TikTok selection;
- keep Paid and other-platform choices available;
- preserve existing drafts and content.

The notice is persistent, not a dismissible toast. Disabled controls use native disabled behavior, visible reason text, and `aria-describedby`; color is not the only signal.

If server policy changes after the client projection loads, a fully blocked submit shows the same exact message inline in the submit-error area. A mixed response keeps allowed progress visible and shows only the TikTok failure.

### 10.2 Posts List and Calendar result details

Both surfaces use the shared result-details behavior and show:

- title `Publishing restricted`;
- the full exact message;
- next action for upgrade or wait;
- `media_retained_until` when present;
- no automatic-retry language;
- disabled Retry with `Publishing restriction active` while blocked;
- enabled existing Retry after disable or Paid upgrade if media remains;
- `Upload media again to retry` after expiry.

## 11. Admin Publishing Restrictions center

### 11.1 Route, access, and extensibility

Add `/admin/publishing-restrictions` plus an Admin navigation item. Render rows from API data so future platforms can be added without building another page.

The page and every API are Super Admin-only. Enforce both route middleware and the existing server-side Super Admin checker. Ordinary Admins receive 403 and no recipient details.

### 11.2 Operational content

For each restriction show:

- platform and reason;
- enabled/disabled state;
- restricted plans;
- current/most recent cycle ID;
- affected Free workspace count;
- affected active account count;
- enabled, disabled, created, and updated timestamps;
- latest actor and version;
- recent audit events;
- restriction/recovery campaign status and progress.

Affected account counts require `platform=tiktok`, `status=active`, `disconnected_at IS NULL`, and a Free workspace. Counts are live and separate from snapshotted campaign counts.

Use existing Admin shell/tokens, divided operational sections, labels above controls, inline loading/error/empty states, keyboard focus, and reduced-motion-safe feedback. Do not introduce a new animation library or decorative design system.

### 11.3 APIs and optimistic versioning

- `GET /v1/admin/publishing-restrictions`
- `PATCH /v1/admin/publishing-restrictions/{platform}`

Mutation body:

```json
{
  "enabled": true,
  "expected_version": 4
}
```

The service locks the row, compares `expected_version`, increments version, and writes the audit event in one transaction. Version mismatch returns 409 with current state; the UI reloads and shows an inline conflict instead of overwriting.

- Disabled to enabled creates a new cycle ID, stamps enable time, and clears disable time.
- Enabled to disabled preserves the cycle for recovery correlation and stamps disable time.
- Same-state requests are idempotent no-ops without a new cycle/event.

### 11.4 Confirmation copy requirements

Enable confirmation states:

- Free TikTok admission/dispatch will be blocked;
- Paid and other platforms remain available;
- already in-flight calls are not canceled;
- no customer email is sent;
- no post is automatically retried.

Disable confirmation states:

- new eligible customer actions may publish again;
- previous failures will not publish automatically;
- customers must use result-level Retry;
- no recovery email is sent unless separately previewed and confirmed.

## 12. Data model

### 12.1 `platform_publishing_restrictions`

One row per platform:

| Column | Purpose |
|---|---|
| `id` | Stable identifier |
| `platform` | Unique normalized key; initial `tiktok` |
| `enabled` | Current state; initial `false` |
| `restricted_plan_ids` | Plan array; initial `['free']` |
| `reason_code` | Initial `platform_capacity_limit` |
| `user_message` | Canonical customer message snapshot |
| `cycle_id` | Current/most recent activation cycle |
| `version` | Monotonic optimistic-lock version |
| `enabled_at` / `disabled_at` | Latest transition times |
| `created_at` / `updated_at` | Record times |
| `updated_by_user_id` | Latest Super Admin actor |

The migration inserts TikTok disabled. Deployment changes no publish behavior and sends no email.

### 12.2 `platform_publishing_restriction_events`

Append-only global audit events store:

- restriction/platform and cycle;
- `enabled` or `disabled` event type;
- actor;
- expected/resulting versions;
- before/after JSON;
- request ID and safe IP/user-agent metadata;
- timestamp.

Use a dedicated table because existing `audit_log` requires a workspace while this policy is global.

### 12.3 `platform_publishing_restriction_email_campaigns`

One row per `(cycle_id, campaign_type)`:

- `campaign_type`: `restriction_notice` or `recovery_notice`;
- status: `queued`, `running`, `completed`, `completed_with_failures`, `failed`;
- immutable subject/body snapshots;
- restriction version;
- previewed and snapshotted recipient counts;
- pending/sent/failed/skipped progress counts or equivalent derived projection;
- created/confirmed actors;
- snapshot/start/completion/create/update timestamps.

The unique cycle/type constraint makes repeat confirmation idempotent.

### 12.4 `platform_publishing_restriction_email_recipients`

Each recipient work item stores:

- campaign ID;
- canonical owner user ID;
- original and normalized email;
- first-name snapshot;
- represented eligible workspace IDs;
- status `pending`, `sending`, `sent`, `failed`, or `skipped_ineligible`;
- attempt count, next attempt, and safe last error;
- stable idempotency key;
- shared `email_send_attempts` reference when available;
- sent/create/update timestamps.

Unique campaign/user and campaign/normalized-email constraints enforce dedupe. If multiple user rows normalize to one email, choose a deterministic canonical recipient and retain all represented workspaces for evidence.

### 12.5 `media_post_usages` and existing `media.usage_version`

Add non-null `retention_reason` only to `media_post_usages`. Backfill existing rows to `plan_status` without changing existing deadlines. Policy and retry transitions follow Section 9. Reuse and increment the existing `media.usage_version` for cleanup/retry concurrency; no version column is added to `media_post_usages`.

## 13. Manual email campaigns

### 13.1 Independent delivery milestone and strict separation

Email remains confirmed product scope, but implementation planning must split it into an independently deliverable **Communications milestone** after the core Policy Enforcement/Admin/Frontend/Media milestone. This separation reduces coupling and lets the core be validated without enabling or sending email; it is not a deferral or removal of the approved email requirements, and the PRD is not complete until both milestones meet their acceptance criteria.

Email is a separate Admin action. Toggle transactions, migrations, deployments, and policy recovery create no campaign or recipient work.

Register only the two fixed service-notice events in `api/internal/emailregistry` and email template documentation. Send through the existing `loops.AuditedClient`, using stable provider idempotency keys and the existing `email_send_attempts` unique provider/key audit trail for every attempt. Correlate attempts to campaign/recipient work through stable trigger references. Missing provider/template configuration becomes a durable failure; do not use an untracked fallback.

The existing audited client and attempt ledger satisfy per-send provider idempotency/audit, but they do not snapshot a confirmed audience, represent the irreversible second confirmation, aggregate durable campaign progress, enforce one campaign per cycle/type, or select only failed recipients for Admin retry. New campaign and recipient persistence is permitted only for those confirmed orchestration guarantees. Do not duplicate provider-attempt audit payloads or build general campaign capabilities.

**Review disposition:** the recommendation to remove or replace approved emails with an ad hoc one-off send conflicts with the confirmed product decision. The delivery split is accepted; a general batch-marketing subsystem is explicitly rejected, and existing audited email infrastructure is mandatory.

### 13.2 APIs and two confirmations

- `POST /v1/admin/publishing-restrictions/{platform}/email-campaigns/preview`
- `POST /v1/admin/publishing-restrictions/{platform}/email-campaigns`
- `GET /v1/admin/publishing-restrictions/{platform}/email-campaigns`
- `POST /v1/admin/publishing-restrictions/{platform}/email-campaigns/{campaignID}/retry-failed`

Preview computes the current deduped recipient count and returns exact copy plus a short-lived signed token containing platform, cycle, type, restriction version, count, and expiry. Preview persists no recipients and cannot send.

The UI displays copy/count, then a second modal labels Send as irreversible. Confirmation includes the token and explicit confirmation value. The server revalidates token, cycle, version, campaign preconditions, and snapshots current recipients in one transaction. Show preview count and final snapshot count if they differ.

### 13.3 Restriction audience

Restriction notice is available only while that cycle is enabled. Recipients are current Free workspace owners with at least one active connected TikTok account:

- active workspace membership with role `owner`;
- plan `free`;
- TikTok account `status=active` and `disconnected_at IS NULL`;
- non-empty normalized email.

Deduplicate by user and normalized email. Immediately before send, recheck that at least one represented workspace remains eligible. Otherwise mark `skipped_ineligible` and do not send.

### 13.4 Recovery audience

Recovery notice is available only when:

- the same cycle is disabled;
- the cycle has a restriction campaign;
- at least one restriction recipient is `sent`;
- no recovery campaign exists for the cycle.

Candidates are only successfully notified users from that cycle who are still Free with active TikTok at send time. Upgraded/disconnected recipients are skipped. Re-enabling invalidates an unconfirmed recovery preview; recovery cannot be confirmed while restricted again.

This intentionally leaves a coverage gap: a user who becomes an eligible Free/TikTok owner after the restriction audience was snapshotted did not receive the restriction notice and therefore does not receive recovery email. They learn that publishing is restored through the refreshed Composer projection. Support and Admin campaign reporting must not describe recovery as reaching every currently eligible Free/TikTok owner.

### 13.5 Durable progress, retry, and idempotency

The worker claims recipient rows in bounded batches with locks/leases, marks sending, rechecks eligibility, and calls Loops with a stable key derived from cycle, type, and canonical recipient. The same key is used by `email_send_attempts`.

Worker crash, repeat confirmation, and Admin failed-recipient retry cannot duplicate an accepted send. Automatic transport retries are bounded. Exhausted rows become failed and the campaign becomes `completed_with_failures`. `Retry failed recipients` resets only failed rows; it does not add recipients, resnapshot audience, change copy, or resend sent/skipped rows.

### 13.6 Exact restriction email

Subject:

> Temporary TikTok publishing restriction for Free plans

Body:

> Hi {{first_name}},
>
> We’re sorry for the disruption.
>
> TikTok applies a daily capacity limit to publishing through UniPost. Because our current capacity has been reached, TikTok publishing is temporarily unavailable for Free-plan users.
>
> Your connected TikTok accounts, saved posts, and other publishing platforms are not affected. Paid plans will continue to have access to TikTok publishing while this restriction is active.
>
> We have asked TikTok to increase our publishing capacity and will let you know as soon as TikTok publishing becomes available again on the Free plan.
>
> Thank you for your patience and understanding.
>
> The UniPost Team

### 13.7 Exact recovery email

Subject:

> TikTok publishing is available again on the Free plan

Body:

> Hi {{first_name}},
>
> TikTok publishing is now available again on the UniPost Free plan.
>
> You can create and publish new TikTok posts from your connected accounts. Posts that previously failed because of the temporary capacity restriction will not publish automatically; please review and publish them again when you’re ready.
>
> Thank you for your patience while we worked to increase publishing capacity.
>
> The UniPost Team

`first_name` is snapshotted from current user data; if absent, render `there`. Subject and body snapshots are immutable after confirmation.

## 14. Audit, concurrency, security, and observability

### 14.1 Policy concurrency

- Row lock plus `expected_version` serializes Admin changes.
- Version conflicts return 409 with current data.
- Audit event and policy mutation commit atomically.
- Worker recheck covers policy/plan changes after admission or queueing.

### 14.2 Retry and cleanup concurrency

- Existing one-active-job-per-result unique index remains authoritative.
- Retry job creation and media activation commit atomically.
- Existing `media.usage_version` plus row locking prevents stale cleanup snapshots from deleting newly active media; `media_post_usages` receives no duplicate version field.

### 14.3 Campaign concurrency

- Unique cycle/type prevents duplicate campaigns.
- User/email uniqueness prevents duplicate snapshots.
- Row claims/leases prevent parallel workers from sending one recipient concurrently.
- Stable provider idempotency prevents duplicate accepted sends after uncertain failures.

### 14.4 Security and privacy

- All restriction/campaign Admin APIs require Super Admin server-side.
- Responses and logs never expose tokens or raw provider payloads.
- Recipient data remains on the Admin surface and follows existing Admin privacy controls.
- Logs exclude email bodies, credentials, and raw media URLs.

### 14.5 Metrics/logs

Record:

- admission, scheduler, worker, and manual-Retry policy blocks;
- policy evaluator errors;
- platform, plan, cycle/version, post/result/job IDs, and failure stage;
- campaign sent, failed, skipped, and retry counts.

## 15. Testing requirements

### 15.1 Policy and Admin

- Migration seeds TikTok disabled/Free and creates no campaign/send.
- Evaluator blocks Free/TikTok only; Paid and other platforms bypass.
- Plan is re-read after upgrade/downgrade.
- Super Admin authorization is enforced.
- Version conflict returns 409 without overwrite.
- Enable creates one cycle/event; repeat enable is no-op.
- Disable preserves cycle/audits actor; repeat disable is no-op.

### 15.2 API, immediate, scheduled, and mixed

- Fully blocked immediate and scheduled requests return the exact 402 envelope and persist nothing.
- The public error details omit cycle ID while internal logs/failures/audit retain it.
- Mixed immediate returns 202, creates only the TikTok policy failure, and queues allowed results.
- Mixed scheduled create returns 201 and retains allowed target metadata for due-time dispatch.
- Draft save works; draft publish is gated.
- Paid TikTok follows normal dispatch.
- Existing due Free/TikTok schedule fails deterministically.
- Mixed due schedule continues allowed destinations.
- Disable/upgrade before due permits TikTok.
- Already-enqueued job is stopped by worker before platform start/call.
- Fake TikTok adapter call count remains zero for every restricted path.
- Result, job, and `post_failures` fields exactly match contract and create no automatic retry.

### 15.3 Manual Retry

- Active Free restriction blocks existing Retry endpoint with unified error.
- Dynamic retry policy shows blocked.
- Disable enables only customer result Retry.
- Paid upgrade enables Retry while Free restriction stays active.
- Existing active-job uniqueness prevents duplicate Retry.
- Re-enable between enqueue/dispatch blocks worker with zero TikTok calls.
- Disable and email campaigns create no jobs.
- Missing/expired media requires re-upload and creates no job.

### 15.4 Media

- Policy failure sets exactly failure time plus 60 days and reason.
- Policy retention overrides the current 48-hour Free failed/partial retention, with metrics for the approximately 30× lifetime/storage-cost increase.
- Mixed/partial parent media intentionally receives the longer policy deadline.
- Result exposes `media_retained_until`.
- Retry job/media activation is atomic.
- Success recalculates normal current-plan retention from success time.
- Another policy failure restarts 60 days.
- Cleanup/retry race is safe in both winners.
- Shared media stays protected by another usage.

### 15.5 Email

- Toggle/deployment/migration never creates or sends email.
- Restriction campaign only while enabled.
- Recovery only after disabled and a successfully sent restriction notice in the cycle.
- Snapshot audience is correct and deduped by user/email.
- Send-time recheck skips upgraded/disconnected/ineligible owners.
- Recovery recipients are a subset of successful restriction recipients.
- Cycle/type/recipient idempotency prevents duplicates across crashes, confirmations, and retries.
- Copy and recipient snapshot remain immutable.
- Progress survives restart.
- Retry-failed touches failed recipients only.
- Exact subjects, bodies, and first-name fallback are covered.
- Every attempted send uses `AuditedClient`/`email_send_attempts`; new campaign data is limited to snapshot, confirmation, progress, cycle dedupe, and failed-recipient retry.
- Recovery tests assert the intentional exclusion of eligible users who were not successfully sent the cycle's restriction notice.

### 15.6 Frontend

- Both composers load policy, show exact persistent notice, and disable only Free TikTok.
- Shared Composer projection refreshes on mount/open and window focus; authoritative submit handling covers stale/failed projections.
- Toggle All/preselection excludes restricted TikTok.
- Stale fully blocked submit shows inline exact error.
- Mixed result shows only TikTok as `Publishing restricted`.
- Posts/Calendar share title, full message, deadline, and Retry state.
- Retry changes after disable, Paid upgrade, and media expiry.
- Admin renders generic rows and every loading/empty/error/conflict state.
- Toggle confirmations say no email and no auto-retry.
- Campaign preview, second confirmation, progress, and retry-failed are accessible.

### 15.7 Required validation

- `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`
- `cd dashboard && npm run build`
- `cd dashboard && npm run test:regression:dashboard` when Playwright browsers are installed
- Browser acceptance for Composer, Posts, Calendar, and Admin on the exact deployed head SHA and only the user-requested environment.

A failed, skipped, canceled, timed-out, missing, or wrong-SHA required result is a hard stop.

## 16. Rollout and operational defaults

Implementation and review use two independently deliverable milestones:

1. **Core milestone:** policy schema/evaluator, Admin restriction control, admission/scheduler/worker/Retry enforcement, customer UI, and media retention.
2. **Communications milestone:** the two fixed manual campaigns, exact copy, audience snapshot/recheck, second confirmation, durable progress, failed-recipient retry, and audited/idempotent sends.

Both milestones remain disabled and side-effect-free at deployment. Operational rollout then proceeds as follows:

1. Ship additive schema/code with TikTok row disabled.
2. Deployment sends no email and changes no TikTok publishing behavior.
3. Verify disabled Admin state, affected counts, empty customer projection, and unchanged publishing.
4. A user explicitly enables the restriction after reading confirmation.
5. A user separately previews and irreversibly confirms each email campaign.
6. Disabling restores eligibility for new posts and customer Retry but never replays old results.
7. Production release, deployment, or promotion occurs only when separately requested and after repository branch/CI/Preview/deployment/browser gates.

## 17. Acceptance criteria

The feature is accepted only when:

- Admin has a generic, Super Admin-only, optimistic, audited restriction center;
- initial TikTok/Free state is disabled and side-effect-free;
- active restriction blocks all Free/TikTok admission/dispatch/retry calls before TikTok while Paid/other platforms continue;
- exact HTTP/result codes, next action, title, and full message are consistent;
- mixed posts fail only TikTok;
- due schedules fail deterministically and never surprise-publish later;
- customer Retry is available only after policy/plan/media/job conditions allow it;
- no automatic or Admin bulk replay exists;
- policy-failed media receives race-safe 60-day ledger retention and shared media is protected;
- Composer, Posts, Calendar, and Admin show consistent accessible states;
- emails require a separate preview plus second confirmation, exact copy, correct deduped/rechecked audience, durable progress, and stable idempotency;
- toggle, deployment, disable, and recovery email never automatically send mail or enqueue posts.
