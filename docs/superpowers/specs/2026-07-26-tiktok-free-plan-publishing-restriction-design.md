# TikTok Free-Plan Publishing Restriction Design

**Status:** Confirmed product design; awaiting written-spec review
**Date:** 2026-07-26
**Owner areas:** Publishing, Admin, Dashboard, Media Retention, Lifecycle Email
**Branch:** `codex/staging-tiktok-free-publishing-restriction`
**Base:** `origin/staging` at `c9b3727d8e527e3cd88738d9608b8c010e195c0e`

## Summary

UniPost needs an operator-controlled, database-backed Publishing Restrictions center that can temporarily stop a selected plan from publishing to a selected platform. The first restriction is TikTok publishing for Free workspaces when TikTok capacity is constrained. Paid plans and every other platform remain available.

The restriction is a publishing policy, not an Unleash feature flag. The backend remains authoritative at every point where publish work is admitted or dispatched. A Free/TikTok target that is blocked by the current policy never reaches the TikTok adapter and never receives an automatic retry. Mixed-platform posts continue for unrestricted targets and fail only the restricted TikTok result.

The feature also gives Super Admins an independent, explicitly manual email-campaign workflow for restriction and recovery notices. Changing the restriction never sends email and never republishes failed content. Customers retain control: after the restriction is lifted or the workspace becomes Paid, they may retry an individual failed TikTok result through the existing result Retry action.

## Goals

1. Provide one extensible Admin center for platform-and-plan publishing restrictions.
2. Restrict TikTok publishing only for the Free plan while the initial restriction is enabled.
3. Apply one centralized policy decision to HTTP admission, immediate dispatch, scheduled dispatch, and manual result retry.
4. Recheck policy in the delivery worker immediately before any TikTok adapter call.
5. Preserve partial success for mixed-platform posts.
6. Give every blocked API response and result the same stable error contract and exact customer message.
7. Preserve policy-failed media for 60 days through the existing `media_post_usages` business ledger.
8. Keep failed TikTok results customer-retryable after the policy no longer applies, without automatic or Admin bulk retry.
9. Expose clear Composer, Posts, Calendar, and Admin states.
10. Support manual, durable, idempotent restriction and recovery email campaigns with recipient preview and failed-recipient retry.

## Non-goals

- This is not an Unleash flag and does not use `/v1/me/features` as its policy authority.
- No paid plan is restricted by the initial data.
- No non-TikTok platform is restricted by the initial data.
- No customer email is triggered by deployment, migration, enable, or disable.
- No post is automatically retried when a restriction is disabled or a workspace upgrades.
- No Admin bulk retry or Admin impersonation of the customer Retry action.
- No Cloudflare R2 Object Lifecycle rule for policy-retained media.
- No deletion or disconnection of TikTok accounts, drafts, saved posts, or historical results.
- No production release, deployment, or external email send is implied by implementing this design.

## Selected Approach

### Chosen: a dedicated database policy service

Add a focused `publishingrestrictions` domain with a PostgreSQL store, policy evaluator, Admin service, and email-campaign worker. Handlers and delivery workers depend on the evaluator interface instead of reading tables directly. This gives every publishing path the same decision rules while keeping Admin state transitions, campaign orchestration, and UI projections independently testable.

The evaluator reads the persisted platform restriction and the workspace's current plan. The platform is resolved from the server-owned social-account record, never trusted from request JSON. There is no cross-request in-memory cache in the first version; request-local reuse is allowed, but each independent worker dispatch performs a fresh database-backed evaluation so an operator change takes effect without cache-expiry ambiguity.

### Rejected: extending the feature-flag system

The existing feature-flag system models product rollout availability. This feature is an operational publishing policy with restricted plans, cycles, affected counts, customer-facing failure records, media retention, and manual communication campaigns. Treating it as a flag would conflate two authorities and would not model the required audit or campaign lifecycle.

### Rejected: UI-only disabling or a TikTok-adapter-only check

UI-only disabling would not protect API clients, schedules, or retries. An adapter-only check would detect the restriction too late to provide correct HTTP 402 admission, deterministic mixed-result behavior, or media retention. The policy must be checked at admission and again at dispatch.

## Core Policy Model

### Decision inputs

Every policy decision uses:

- the current database restriction row for the resolved platform;
- the current `subscriptions.plan_id` for the workspace, with a missing subscription treated conservatively as `free` in the same way existing plan checks do;
- the workspace and social-account ownership relationship;
- the current restriction `cycle_id` and `version` for evidence.

The decision is restricted only when all of these are true:

1. the restriction row is enabled;
2. the resolved platform is listed by that row, initially `tiktok`;
3. the current workspace plan is in `restricted_plan_ids`, initially only `free`.

Paid plans bypass the initial restriction at admission and dispatch. A workspace that upgrades after a policy failure becomes eligible for manual Retry without waiting for the restriction to be disabled.

### Policy service boundary

The domain exposes one read decision for publishing paths and separate Admin/campaign operations:

```go
type Decision struct {
    Restricted    bool
    Platform      string
    PlanID        string
    ReasonCode    string
    CycleID       string
    Version       int64
    UserMessage   string
    NextAction    string
}

type Evaluator interface {
    Evaluate(ctx context.Context, workspaceID, platform string) (Decision, error)
}
```

Publishing callers consume `Decision`; they do not reproduce plan-list, enabled-state, message, or cycle logic. Admin mutations live behind a service that owns optimistic version checks and audit events. Campaign creation and delivery live behind a separate service/worker so a policy toggle cannot accidentally send mail.

### Evaluator failure behavior

If the evaluator cannot establish policy state, the caller must not call TikTok:

- an HTTP admission path returns a retryable 503/internal-service error;
- a delivery worker leaves the attempt in the existing infrastructure retry path with a UniPost/internal failure, rather than classifying it as a policy restriction;
- no customer policy email or policy-retention deadline is created from an evaluator infrastructure error.

This is distinct from a successful `Restricted=true` decision, which is deterministic, non-automatically-retriable, and uses the policy error contract below.

## Canonical Error Contract

### Constants

- API code: `PLAN_PLATFORM_PUBLISHING_RESTRICTED`
- Normalized and persisted result code: `plan_platform_publishing_restricted`
- HTTP status for a fully blocked publish request: `402 Payment Required`
- Failure stage: `publishing_policy`
- Error source: `unipost`
- Error temporality: `temporary`
- Next action: `upgrade_or_wait_then_retry`
- User message, exactly:

> TikTok publishing is temporarily unavailable on the Free plan due to platform capacity limits. We’re working with TikTok to increase capacity. Upgrade your plan or try again after the restriction is lifted.

### Fully blocked HTTP response

When every publishing target in a request is restricted, the request creates no parent post, result, delivery job, quota usage, or media-retention transition and returns:

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
      "plan_id": "free",
      "restriction_cycle_id": "<current-cycle-id>"
    }
  },
  "request_id": "<request-id>"
}
```

`is_retriable=false` means UniPost must not automatically retry the blocked operation. The conditional manual-retry state is expressed by the result's dynamic `retry_policy` after a result exists.

### Persisted result and job failure

A policy-failed TikTok result has:

- `status=failed`;
- the exact user message in `error_message`;
- `error_code=plan_platform_publishing_restricted`;
- `failure_stage=publishing_policy`;
- `error_source=unipost`;
- `error_temporality=temporary`;
- `is_retriable=false` so no automatic delivery retry is scheduled;
- `next_action=upgrade_or_wait_then_retry`;
- no `provider_error`, `platform_error_code`, external ID, publish token, URL, or debug curl.

The associated delivery job, if one already exists, becomes terminal without creating a retry job. The corresponding `post_failures` record stores the same normalized code, cycle ID in safe metadata, and the exact message. The result presentation title is `Publishing restricted`.

## Publishing Flow Semantics

### Drafts and validation

Saving a draft remains allowed because it does not admit publish work. Existing accounts, drafts, saved posts, and content stay intact. Publishing a draft runs the same policy gate as a new publish request.

`GET /v1/publishing-restrictions` gives the signed-in Dashboard a user-safe workspace projection for preemptive UI disabling. It returns only active restrictions that apply to the current workspace plan:

```json
{
  "data": {
    "restrictions": [
      {
        "platform": "tiktok",
        "plan_id": "free",
        "reason_code": "platform_capacity_limit",
        "message": "TikTok publishing is temporarily unavailable on the Free plan due to platform capacity limits. We’re working with TikTok to increase capacity. Upgrade your plan or try again after the restriction is lifted.",
        "next_action": "upgrade_or_wait_then_retry"
      }
    ]
  }
}
```

This projection improves UX but is never authoritative. A stale or bypassed client still encounters the server admission and worker gates.

### New immediate publish

After structural validation and server-side account resolution, the admission gate partitions targets into allowed and restricted sets.

- All targets restricted: return the canonical HTTP 402 and persist nothing.
- Mixed targets: create the parent post; immediately create a failed result for each restricted TikTok target; create normal pending results and delivery jobs only for allowed targets. The response contains both outcomes.
- No targets restricted: follow the current queue path unchanged.

Quota accounting counts only targets that actually publish successfully under existing rules. A policy failure does not consume publish quota.

### New scheduled publish

- A scheduled request containing only restricted TikTok targets is fully blocked with HTTP 402 and is not stored.
- A mixed scheduled request remains schedulable for its allowed destinations. The metadata retains all original per-target content so the due-time policy decision remains attributable to the original result.
- The scheduler evaluates every target when the scheduled time arrives. It creates a failed TikTok result and no delivery job for any target restricted at that time, while enqueueing the allowed targets.
- If the restriction was disabled or the workspace became Paid before the due time, TikTok is allowed and schedules normally.

This due-time failure rule ensures the policy-retention clock starts at the actual scheduled policy failure, not at schedule creation, and prevents a 60-day retention window from expiring before a far-future scheduled time.

### Existing scheduled posts and queued jobs

An existing Free/TikTok scheduled post that becomes due while the restriction is active fails deterministically with the policy result contract. It does not remain pending for later automatic publication. Other platform results from the same parent continue.

A TikTok delivery job that was already created before the restriction was enabled is protected by the worker gate. The worker evaluates policy after loading the current job, post, result, and account, and immediately before `MarkPostDeliveryJobPlatformStarted` and `publishOneContext`. A restricted decision marks the result/job as the terminal policy failure and returns without invoking the TikTok adapter. The existing job and result double-publish guards remain in front of this gate.

Enabling a restriction does not attempt to cancel a TikTok HTTP call that already began before the committed restriction was observed. It prevents subsequent adapter calls. The Admin confirmation describes this boundary without promising cancellation of in-flight provider work.

### Mixed-platform parent status

Parent status continues to derive from all results:

- allowed results in progress keep the parent publishing;
- at least one allowed published result plus a restricted failed result yields `partial`;
- all results restricted/failed yields `failed` only for stored legacy/due-time cases, because a fresh all-restricted request is rejected before persistence.

No restricted TikTok failure prevents allowed targets from being queued, dispatched, or finalized.

## Manual Result Retry

The customer uses the existing endpoint and existing result-level Retry UI:

`POST /v1/posts/{postId}/results/{resultId}/retry`

### Retry eligibility

The result must still satisfy all existing requirements:

- it belongs to the workspace and parent post;
- it is `failed`;
- no pending, running, or retrying job exists for that result;
- the original target and media can be resolved.

For `plan_platform_publishing_restricted`, the retry policy adds these rules:

- while the current workspace is still Free and the TikTok restriction is enabled: `manual_retry_allowed=false`, `retry_state=blocked`, `reason=publishing_restriction_active`;
- after the restriction is disabled: manual Retry becomes available;
- after the workspace upgrades to a non-restricted plan: manual Retry becomes available even if the restriction remains enabled;
- after retained media has expired or is missing: `manual_retry_allowed=false`, `reason=media_reupload_required` and the API returns a specific media-reupload error rather than queueing a job.

The retry endpoint performs the policy and media checks again inside the enqueue transaction. UI eligibility alone never authorizes dispatch.

### Retry enqueue and dispatch

The existing partial unique index allowing one active delivery job per result remains the final database guard. In one transaction, retry enqueue:

1. locks/rechecks the failed result and current active-job state;
2. rechecks policy and current workspace plan;
3. verifies every referenced media object is still uploaded;
4. creates exactly one retry job;
5. moves the post's media usages back to active retention with no cleanup deadline.

The worker rechecks policy again before any TikTok call. If the restriction reappeared or the workspace returned to Free before dispatch, the retry fails with the policy code, creates no automatic retry, and restarts the 60-day retention window from this new policy-failure time.

An Admin cannot invoke, batch, or schedule this customer Retry operation. Disabling a restriction and completing a recovery email campaign never enqueue delivery jobs.

## Media Retention and Cleanup Safety

### Ledger extension

Extend `media_post_usages` with a non-null `retention_reason`. Existing rows backfill to `plan_status`. Supported values initially are:

- `active_post`: the post is non-terminal or a customer retry is active; `cleanup_after_at=NULL`;
- `plan_status`: the normal plan/status retention policy applies;
- `publishing_restriction`: at least one result on the parent has the policy failure; `cleanup_after_at=policy_failed_at + 60 days`.

The migration is additive and does not rewrite existing cleanup deadlines other than assigning their reason. No R2 lifecycle configuration changes.

### Policy failure

At the exact time a TikTok result becomes policy-failed, all `media_post_usages` rows for that parent post are updated to:

- the current parent/result-derived status;
- `retention_reason=publishing_restriction`;
- `cleanup_after_at=policy_failed_at + interval '60 days'`.

This overrides the normal Free failed/partial retention deadline. A later policy failure restarts the full 60 days from that later failure. The API result projection exposes the effective common deadline as `media_retained_until`.

### Retry and success transitions

- Customer Retry start atomically sets `retention_reason=active_post` and `cleanup_after_at=NULL` in the same transaction that creates the active retry job.
- Successful retry recalculates the ordinary plan/status deadline from the success time using the workspace plan then in effect and sets `retention_reason=plan_status`.
- A non-policy terminal retry failure uses the existing normal plan/status retention calculation.
- Another policy failure sets `publishing_restriction` and a new failure-time-plus-60-days deadline.

### Shared media and cleanup race

The existing cleanup eligibility rule remains: an object cannot be claimed while any `media_post_usages` or media-processing usage has a null/future deadline. The retry transaction updates the parent media row's `usage_version` while returning usage to active. Cleanup claims compare that version and lock the media row, so a concurrent retry either protects the object before claim or detects that the object was already deleted and returns `media_reupload_required`; it never queues a TikTok retry against an object being deleted.

Shared media stays protected by every usage row. Expiry of one policy-retained post never deletes an object still active or retained for another post or processing job.

### After expiry

After the 60-day deadline and successful cleanup, the historical result remains visible, including `Publishing restricted`, the message, and the expired `media_retained_until`. Manual Retry is unavailable because the existing endpoint has no replacement-media payload. The customer must create/edit a post with a new upload before publishing again.

## Database Design

### `platform_publishing_restrictions`

One row per platform:

| Column | Purpose |
|---|---|
| `id` | Stable UUID/text identifier |
| `platform` | Unique normalized platform key, initially `tiktok` |
| `enabled` | Current policy state; seeded `false` |
| `restricted_plan_ids` | Non-empty plan ID array when enabled; initially `['free']` |
| `reason_code` | Stable operator reason, initially `platform_capacity_limit` |
| `user_message` | Snapshotted canonical customer message |
| `cycle_id` | Current/most recent activation cycle identifier |
| `version` | Monotonic optimistic-lock version |
| `enabled_at` / `disabled_at` | Latest transition timestamps |
| `created_at` / `updated_at` | Record timestamps |
| `updated_by_user_id` | Latest Super Admin actor |

The migration inserts the TikTok row disabled. Deployment alone changes no publish behavior and sends no email.

### `platform_publishing_restriction_events`

Append-only global audit events record:

- restriction ID/platform and cycle ID;
- `enabled` or `disabled` event type;
- actor user ID;
- expected and resulting versions;
- before/after JSON snapshots;
- request ID, IP/user-agent metadata where available;
- event timestamp.

A dedicated table is used because the existing `audit_log` requires a workspace ID while this policy is global. Events are never updated or deleted by the Admin workflow.

### `platform_publishing_restriction_email_campaigns`

One durable campaign per `(cycle_id, campaign_type)`:

| Column | Purpose |
|---|---|
| `id` | Campaign ID |
| `restriction_id` / `cycle_id` | Exact restriction cycle |
| `campaign_type` | `restriction_notice` or `recovery_notice` |
| `status` | `queued`, `running`, `completed`, `completed_with_failures`, or `failed` |
| `subject_snapshot` / `body_snapshot` | Immutable confirmed copy |
| `restriction_version` | Version confirmed by Admin |
| `previewed_recipient_count` | Count shown before confirmation |
| `snapshotted_recipient_count` | Count actually inserted at confirmation |
| `pending_count` / `sent_count` / `failed_count` / `skipped_count` | Durable progress counters or query-derived projection |
| `created_by_user_id` / `confirmed_by_user_id` | Admin actors |
| `snapshot_at` / `started_at` / `completed_at` | Lifecycle timestamps |
| `created_at` / `updated_at` | Record timestamps |

The unique cycle/type key makes repeated confirmation idempotent and prevents duplicate campaigns.

### `platform_publishing_restriction_email_recipients`

One recipient snapshot per campaign and normalized email:

| Column | Purpose |
|---|---|
| `id` | Recipient work item ID |
| `campaign_id` | Owning campaign |
| `recipient_user_id` | Canonical owner user |
| `recipient_email` / `normalized_email` | Send address and dedupe key |
| `first_name_snapshot` | Confirmed personalization value |
| `eligible_workspace_ids` | All eligible workspaces represented by the deduped recipient |
| `status` | `pending`, `sending`, `sent`, `failed`, or `skipped_ineligible` |
| `attempt_count` / `next_attempt_at` | Durable worker progress |
| `last_error` | Safe provider/eligibility error |
| `idempotency_key` | Stable per-cycle/type/recipient key |
| `email_send_attempt_id` | Link to shared audit when available |
| `sent_at` / `created_at` / `updated_at` | Timestamps |

Unique constraints on `(campaign_id, recipient_user_id)` and `(campaign_id, normalized_email)` enforce user and email dedupe. When multiple owners normalize to the same email, the snapshot chooses a deterministic canonical user and retains all represented workspace IDs for evidence. The provider idempotency key stays stable across failed-recipient retries.

## Admin Publishing Restrictions Center

### Route and access

Add standalone `/admin/publishing-restrictions` and an Admin navigation item. The page and every API are Super Admin-only, using both route middleware and the existing server-side Super Admin checker. Ordinary Admins receive 403 and never receive campaign recipient details.

The page is extensible by rendering restriction rows from API data rather than hard-coding a TikTok-only page.

### Page content

For each platform row, show:

- platform and reason;
- current enabled/disabled state;
- restricted plan badges;
- current cycle ID;
- affected Free workspace count;
- affected active TikTok account count;
- enabled, disabled, and last-updated timestamps;
- latest actor;
- current optimistic version;
- recent audit events;
- restriction and recovery email campaign status/progress.

Counts are live operational counts, not email-recipient counts. Affected accounts require `platform=tiktok`, `status=active`, `disconnected_at IS NULL`, and a workspace on Free.

The UI follows the existing Admin shell and design tokens. Use divided operational sections rather than decorative cards, labels above controls, inline loading/error/empty states, keyboard-focus styles, and reduced-motion-safe feedback. No new animation library or visual language is introduced.

### Toggle API and optimistic locking

Admin APIs:

- `GET /v1/admin/publishing-restrictions`
- `PATCH /v1/admin/publishing-restrictions/{platform}`

Mutation body:

```json
{
  "enabled": true,
  "expected_version": 4
}
```

The service locks the row and compares `expected_version`. A mismatch returns 409 with the current projection. A successful mutation increments `version` and appends an audit event in the same transaction.

- `false -> true` creates a new `cycle_id`, stamps `enabled_at`, clears `disabled_at`, and uses the configured restricted plan list.
- `true -> false` preserves the cycle ID for recovery-campaign correlation and stamps `disabled_at`.
- requesting the already-current state is an idempotent no-op that does not create another cycle or audit event.

### Toggle confirmations

Enable confirmation explicitly states:

- Free-plan TikTok admission and dispatch will be blocked;
- paid plans and other platforms remain available;
- already in-flight TikTok calls are not canceled;
- no customer email will be sent;
- no post will be retried automatically.

Disable confirmation explicitly states:

- new eligible customer actions may publish to TikTok again;
- previously failed results will not publish automatically;
- customers must use result-level Retry themselves;
- no recovery email will be sent unless an Admin separately creates and confirms it.

The toggle is disabled while a mutation is pending. A 409 reloads the row and shows an inline version-conflict message rather than overwriting another Admin's change.

## Customer Dashboard Experience

### Composer and Calendar composer

When the current workspace projection contains the TikTok/Free restriction:

- show the exact persistent inline notice directly beneath the TikTok account-selection area;
- render active TikTok account choices disabled and unselected with an accessible reason;
- exclude disabled TikTok accounts from Toggle All and preselection/replay behavior;
- disable submission if stale local state still contains a TikTok selection;
- keep paid-plan and other-platform selections available;
- keep existing drafts and their content intact.

The notice remains visible while the restriction applies; it is not a dismissible toast. Disabled account buttons use native `disabled`, visible text, and `aria-describedby` so color is not the only signal.

If the restriction changes after the client projection loads, the server response remains authoritative. A fully blocked submit shows the exact message inline in the submit-error area. A mixed response keeps allowed-result progress visible and renders the TikTok result failure normally.

### Posts and Calendar result details

The shared result-details model recognizes `plan_platform_publishing_restricted` and renders:

- title: `Publishing restricted`;
- the full exact customer message;
- next-action label reflecting upgrade or wait;
- `media_retained_until` when present;
- no automatic-retry language;
- Retry disabled with `Publishing restriction active` while blocked;
- Retry enabled through the existing control once policy/plan/media conditions allow it;
- `Upload media again to retry` after media expiry.

Posts List and Calendar use the same shared result component, so status, message, retention deadline, and Retry behavior cannot drift.

## Manual Email Campaigns

### Separation from the policy toggle

Email is a distinct Admin action. Toggle transactions never create campaigns or recipient rows. Migration and deployment never create campaigns. Campaign workers process only rows created by an Admin's second confirmation.

Register two service-notice events in `api/internal/emailregistry` and the email template documentation. Delivery uses the existing audited Loops transactional client, stable idempotency keys, and shared `email_send_attempts` audit. Provider/template configuration failure is durable campaign failure, never a reason to send through an untracked fallback.

### Preview and irreversible confirmation

Admin APIs:

- `POST /v1/admin/publishing-restrictions/{platform}/email-campaigns/preview`
- `POST /v1/admin/publishing-restrictions/{platform}/email-campaigns`
- `GET /v1/admin/publishing-restrictions/{platform}/email-campaigns`
- `POST /v1/admin/publishing-restrictions/{platform}/email-campaigns/{campaignID}/retry-failed`

Preview accepts a campaign type, computes the eligible deduped count, and returns the exact subject/body plus a short-lived signed preview token containing platform, cycle ID, type, restriction version, count, and expiry. It does not persist recipients and cannot send.

The UI first shows copy and recipient count. The Send action opens a second confirmation explicitly labeled irreversible. Confirmation posts the preview token and an explicit confirmation value. The server revalidates token, cycle, version, campaign preconditions, and then snapshots the current recipient set in one transaction. The fresh snapshot count may differ from preview; the response shows both counts. Only committed recipient work items are deliverable.

### Restriction-notice eligibility

Restriction notice can be confirmed only while the restriction is enabled for that cycle. Eligible recipients are current owners of Free workspaces with at least one active connected TikTok account:

- membership role `owner` and status `active`;
- subscription plan `free`;
- at least one social account with `platform=tiktok`, `status=active`, and `disconnected_at IS NULL`;
- a non-empty normalized email.

Snapshot candidates are deduped by user and normalized email. Immediately before each send, the worker rechecks that at least one represented workspace is still Free with an active TikTok account. Ineligible recipients become `skipped_ineligible` and are not sent.

### Recovery-notice eligibility

Recovery notice can be confirmed only when:

- the same cycle is disabled;
- the cycle has a restriction-notice campaign;
- at least one recipient in that notice campaign is `sent`;
- no recovery campaign already exists for the cycle.

Candidates are only users successfully sent the restriction notice in that cycle. The worker rechecks that they are still Free with at least one active TikTok account. Upgraded or disconnected recipients are skipped. A later re-enable invalidates an unconfirmed recovery preview; recovery cannot be confirmed while the platform is restricted again.

### Durable delivery and retries

The campaign worker claims recipient rows with row locking/leases in bounded batches, marks each `sending`, rechecks eligibility, and sends with a stable key derived from cycle, campaign type, and canonical recipient key. Provider calls and `email_send_attempts` use that key, so worker crashes and Admin retry cannot duplicate an accepted send.

Automatic worker retry is bounded for transport failures. Exhausted recipients become `failed`, and the campaign becomes `completed_with_failures`. The Admin `Retry failed recipients` action changes only failed rows back to pending; it does not resnapshot eligibility, add recipients, or change copy. Sent and skipped rows never resend. Progress survives API restarts and is visible by counts and per-recipient failure summary.

### Restriction email copy

Subject, exactly:

> Temporary TikTok publishing restriction for Free plans

Body, exactly:

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

`first_name` uses the current user name at snapshot time; when no usable first name exists, it renders `there` so the greeting remains grammatical. Subject/body snapshots are immutable after confirmation.

### Recovery email copy

Subject, exactly:

> TikTok publishing is available again on the Free plan

Body, exactly:

> Hi {{first_name}},
>
> TikTok publishing is now available again on the UniPost Free plan.
>
> You can create and publish new TikTok posts from your connected accounts. Posts that previously failed because of the temporary capacity restriction will not publish automatically; please review and publish them again when you’re ready.
>
> Thank you for your patience while we worked to increase publishing capacity.
>
> The UniPost Team

## Security, Privacy, and Observability

- Restriction mutation, preview, confirmation, progress, and retry-failed APIs require Super Admin authorization server-side.
- API responses never expose account tokens or raw provider payloads.
- Audit events record actors and safe before/after policy data.
- Campaign recipient data is limited to the Admin page and follows existing Admin privacy controls where applicable.
- Logs include platform, plan, restriction cycle/version, post/result/job IDs, decision, and failure stage, but not email bodies, tokens, or raw media URLs.
- Metrics distinguish admission blocks, scheduler blocks, worker blocks, manual-retry blocks, campaign sent/failed/skipped counts, and policy evaluator errors.
- Policy failures are not reported as TikTok/provider failures because no provider call occurred.

## Testing Strategy

### Policy and database tests

- Migration seeds TikTok disabled with restricted plan `free` and sends/queues nothing.
- Evaluator restricts Free/TikTok and bypasses Paid/TikTok, Free/non-TikTok, and disabled state.
- Current plan is re-read after upgrade/downgrade.
- Toggle optimistic version conflict returns 409 and cannot overwrite state.
- Enable creates one cycle and audit event; idempotent enable does not.
- Disable preserves the cycle and records actor/timestamps; idempotent disable does not.
- Super Admin authorization protects all Admin APIs.

### API admission and dispatch tests

- Fully blocked immediate and scheduled requests return exact HTTP 402 envelope and persist no post/result/job.
- Mixed immediate request creates only the TikTok policy failure and queues allowed targets.
- Draft save remains allowed; draft publish enforces policy.
- Paid plan bypasses admission and reaches normal dispatch.
- Scheduled Free/TikTok due while active fails deterministically and creates no TikTok job/call.
- Mixed scheduled post continues other destinations.
- Restriction lifted before due permits TikTok.
- Already-enqueued TikTok job is blocked by the worker recheck before platform-start/adaptor invocation.
- Worker adapter mock proves zero TikTok calls for every restricted path.
- Policy result/job/post-failure fields match the canonical contract and schedule no automatic retry.

### Manual Retry tests

- Existing endpoint rejects manual Retry while Free restriction is active with the exact unified error.
- Dynamic retry policy is blocked while active.
- Disabling restriction enables customer Retry for that result only.
- Upgrading to Paid enables Retry while global restriction remains enabled.
- Existing active-job uniqueness prevents duplicate Retry.
- Policy re-enable between enqueue and dispatch fails at worker recheck with no TikTok call.
- Disabling/recovery campaign never creates retry jobs.
- Expired or missing media returns media-reupload-required and creates no job.

### Media retention tests

- Policy failure sets failure time plus exactly 60 days and `publishing_restriction`.
- Policy retention overrides normal Free failed/partial retention.
- `media_retained_until` is exposed on affected result details.
- Retry enqueue and active/no-deadline transition are atomic.
- Successful retry recalculates normal current-plan retention from success time.
- Another policy failure restarts 60 days.
- Cleanup-versus-retry concurrency cannot delete media after active retry wins.
- Retry detects already-deleted media when cleanup wins.
- Shared media remains protected by another active/future usage.

### Email campaign tests

- Toggle and migration never create campaigns or send mail.
- Restriction preview/confirm is allowed only while enabled.
- Recovery preview/confirm is allowed only after disabled and only for a cycle with successful restriction notices.
- Recipient snapshot includes current Free owners with active TikTok, deduped by user and normalized email.
- Worker recheck skips upgraded, disconnected, or no-longer-owner recipients.
- Recovery recipients are a subset of successfully notified restriction recipients.
- Stable cycle/type/recipient idempotency prevents duplicate sends across crash, repeat confirmation, and failed-recipient retry.
- Copy and recipient snapshot stay immutable.
- Durable progress survives worker restart.
- Failed-recipient retry touches failed recipients only.
- Exact subjects, bodies, first-name fallback, and campaign counts are covered.

### Dashboard tests

- Composer and Calendar composer load workspace restrictions.
- Persistent notice uses the exact message.
- Free TikTok choices are disabled and excluded from Toggle All/preselection while other choices work.
- Stale fully blocked submit renders inline error.
- Mixed response shows only TikTok as `Publishing restricted`.
- Posts and Calendar share the same result title, full message, retention deadline, and Retry state.
- Retry becomes available after disable or Paid upgrade and becomes unavailable after media expiry.
- Admin page renders multiple platform rows from data, not a TikTok-only hard-coded layout.
- Toggle confirmations state no email and no auto-retry.
- Version conflict, loading, empty, error, campaign preview, second confirmation, progress, and retry-failed states are accessible.

### Required validation

- From `api/`: `GOCACHE=/tmp/unipost-go-build go test ./...`
- From `dashboard/`: `npm run build`
- From `dashboard/`: `npm run test:regression:dashboard` when Playwright browsers are installed
- Focused worker tests use a fake TikTok adapter and assert call count zero.
- Browser acceptance covers Composer, Posts, Calendar, and `/admin/publishing-restrictions` on the exact deployed head SHA at the environment explicitly requested by the user.

Any failed, skipped, timed-out, canceled, missing, or wrong-SHA required result remains a hard stop under the repository workflow.

## Rollout and Operational Safety

1. Deploy the additive schema and code with the initial TikTok restriction disabled.
2. Verify the Admin row is disabled, counts load, customer projection is empty, and ordinary TikTok publishing is unchanged.
3. No email template or campaign is activated by deployment.
4. A user must explicitly enable the restriction from Admin after reviewing the confirmation.
5. A user must separately preview and irreversibly confirm each email campaign.
6. Disabling the restriction restores eligibility for new publishing and manual result Retry but does not enqueue any old result.
7. Environment promotion or production release occurs only when separately requested and after the repository's required branch, CI, Preview, deployment, and browser-acceptance gates.

## Acceptance Criteria

The design is complete when all of the following are true on the implementation's accepted target environment:

- the Admin restriction row defaults disabled and can be changed only with optimistic, audited Super Admin mutations;
- enabling Free/TikTok restriction blocks every new or queued restricted adapter call while Paid and other-platform publishing continue;
- exact HTTP/result error contracts and customer copy are used everywhere;
- mixed posts fail only their TikTok result;
- due scheduled posts never surprise-publish later;
- manual Retry is customer-controlled and dynamically available only when policy, plan, job, and media state allow it;
- policy-failed media receives race-safe 60-day business retention and shared media remains protected;
- Composer, Posts, Calendar, and Admin show consistent accessible states;
- email remains a separate two-confirmation manual operation with correct dedupe, eligibility recheck, durable progress, and idempotency;
- no toggle, deployment, recovery, or Admin action bulk-retries a post or automatically sends an email.
