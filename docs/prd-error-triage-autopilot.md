# PRD - Admin Error Triage Autopilot

**Status:** Planning
**Owner:** Admin / Support / Publishing / Notifications
**Created:** 2026-06-07
**Target:** Daily admin error report, AI triage, bug-fix planning, and one-click Loops user email delivery

---

## Problem

UniPost already exposes cross-tenant publishing failures at `/admin/errors`, and the backend already stores useful diagnostic data in `social_post_results`, `post_failures`, integration logs, and Loops lifecycle events. The missing product layer is a daily operator workflow:

- Admins must manually open `/admin/errors` and scan individual failures.
- Repeated failures are not grouped into a support-ready or engineering-ready summary.
- There is no durable daily report that says whether the last 24 hours were healthy.
- User-actionable failures do not produce a reviewed email draft that can be sent from the admin surface.
- UniPost platform bug candidates do not produce a structured repair plan for approval.

The current manual process is workable for a small volume of errors, but it does not scale with customer usage. UniPost needs a formal internal feature that turns daily failures into reviewed actions without bypassing human approval.

## Product Direction

Build an admin-only Error Triage Autopilot that runs every day at 12:00 AM PT and analyzes the previous 24 hours of production publishing failures.

The scheduled run should generate:

1. A daily summary report.
2. Grouped failure buckets with impact and evidence.
3. Classifications for each actionable bucket.
4. Bug-fix plans for UniPost platform bug candidates.
5. User-facing Loops email drafts for user-actionable issues.
6. A clear "no action needed" report when there are no actionable issues.

The scheduled job must not automatically email customers and must not automatically create code changes. Admins review the generated output in a new admin page and explicitly click to send a prepared user email or approve a bug-fix plan.

## Goals

1. Add a new admin left-nav entry for Error Triage.
2. Run a daily triage job at 12:00 AM America/Los_Angeles.
3. Analyze production failures from the previous 24-hour PT window.
4. Group failures into coherent buckets by platform, error code, account state, source, message pattern, and affected customers.
5. Classify buckets as UniPost bug, user action needed, upstream platform issue, transient/no action, known duplicate, or needs human review.
6. Generate admin-readable daily reports with clear evidence and recommended next actions.
7. Generate bug-fix plans for likely UniPost platform bugs, including suspected surface, reproduction clues, risk, validation plan, and rollout notes.
8. Generate safe user email drafts for user-actionable issues.
9. Let admins send a generated email to the affected dashboard customer through Loops with one click.
10. Store triage runs, item classifications, email drafts, send events, and admin actions for auditability.

## Non-goals

- No automatic customer email sending from the scheduled run.
- No automatic code changes, branch creation, commits, pull requests, or deployment.
- No direct dashboard-browser connection to Loops, OpenAI, Anthropic, or any other third-party secret-bearing service.
- No exposure of raw debug curls, request payloads, access tokens, refresh tokens, provider payloads, or internal stack traces in customer emails.
- No support for emailing managed end-users through `external_user_email` in v1. The recipient is the UniPost dashboard customer or workspace owner represented by `user_email`.
- No replacement of `/admin/errors`; the new page summarizes and acts on failures, while `/admin/errors` remains the raw inspection surface.
- No production release automation. Bug plans may later become development tasks after explicit approval.

## Current Codebase Findings

### Admin errors

- Dashboard route: `dashboard/src/app/admin/errors/page.tsx`.
- Admin menu: `dashboard/src/app/admin/_components/admin-ui.tsx`.
- Existing menu already has `Errors` under `System`.
- Current admin failure list calls `GET /v1/admin/post-failures`.
- The current list supports `search`, `platform`, `source`, `days`, and `limit`.
- The dashboard currently loads at most 100 failures, while the backend caps list responses at 200.

### Backend failure data

- `post_failures` table exists with `platform`, `failure_stage`, `error_code`, `platform_error_code`, `message`, `raw_error`, `is_retriable`, and `created_at`.
- `AdminHandler.ListPostFailures` currently builds admin failure rows by joining `social_posts`, `social_post_results`, `social_accounts`, `workspaces`, and `users`.
- `social_post_results.debug_curl` may contain important evidence, and server-side code documents that it is redacted before admin exposure.
- `post_failures` is a better long-term triage source than scraping `/admin/errors`, because it is structured, indexed, and independent of UI rendering.

### Loops

- `api/internal/loops/client.go` supports contact upsert, event send, and transactional email send.
- `api/internal/loops/syncer.go` gates Loops lifecycle behavior through `email.loops_integration_v1`.
- Current transactional IDs include plan changed, account canceled, and post failed.
- Existing post-failed Loops events are customer notification oriented; this feature needs a separate admin-approved support email path with its own idempotency and audit trail.

### AI generation

- The backend already has server-side AI patterns in `api/internal/handler/ai_post_assist.go`.
- This feature should use a server-side AI interface with structured JSON output and no frontend API keys.
- AI output must be persisted so repeated page loads do not regenerate reports or email drafts.

## User Experience

### Admin entry point

Add a new admin sidebar item under `System`:

```text
Error Triage -> /admin/error-triage
```

The page should show the latest daily run by default.

Primary page areas:

- Run status card: `Completed`, `Running`, `Failed`, `No actionable issues`, or `Needs review`.
- Summary cards: failures analyzed, affected users, affected workspaces, platform bug candidates, user email drafts, needs-review items.
- Daily report: concise natural-language summary.
- Buckets table: classification, confidence, platform, users affected, latest error, recommended action, and status.
- Email draft queue: generated customer emails waiting for admin send.
- Bug plan queue: UniPost bug candidates waiting for admin approval.
- Run history: previous daily runs with date, status, and counts.

### Daily scheduled run

At 12:00 AM PT, the backend creates a triage run for the previous PT day:

```text
window_start = previous local midnight in America/Los_Angeles
window_end = current local midnight in America/Los_Angeles
```

If the scheduled job starts late, it still uses the intended PT window. If a run already exists for the same window, the scheduler should not create a duplicate.

The run should:

1. Acquire a database advisory lock so multiple API replicas cannot run the same triage concurrently.
2. Load failures for the window.
3. Normalize and redact diagnostic inputs.
4. Group related failures into buckets.
5. Ask the AI classifier to produce structured triage output per bucket.
6. Persist the report and item-level outputs.
7. Mark the run completed, completed with review needed, or failed.

### Manual run

Admins can click `Run now` for the last 24 hours. Manual runs are useful after deploys or after fixing a classifier prompt. Manual runs must be labeled as manual and must not replace the canonical scheduled run for the PT day unless the admin explicitly chooses `Re-run daily report`.

### No-issue day

If there are no failures or only non-actionable transient failures, the page shows:

- `No actionable issues found`
- the number of failures inspected
- any transient or already-retried buckets
- no pending email drafts
- no pending bug plans

This is still stored as a completed triage run so admins can see that the job ran.

## Classification Taxonomy

Each bucket receives exactly one primary classification:

```text
unipost_bug
user_action_needed
upstream_platform_issue
transient_no_action
known_duplicate
needs_human_review
```

### `unipost_bug`

Use when the evidence indicates UniPost code, configuration, queueing, validation, upload behavior, retry behavior, OAuth setup, or integration handling caused the failure.

Examples:

- UniPost sent invalid parameters to a provider.
- UniPost accepted an unsupported media shape that should have been rejected by validation.
- A retryable error was marked terminal.
- A provider credential or redirect URI is misconfigured by UniPost.

Required output:

- title
- impact
- evidence
- suspected code area
- reproduction clues
- proposed fix
- validation plan
- rollout and rollback notes
- confidence

### `user_action_needed`

Use when the customer can resolve the issue without a UniPost code change.

Examples:

- expired or revoked social account connection
- missing platform permission
- media or caption violates platform rules
- quota or account-level restriction controlled by the customer or provider
- invalid board/page/channel/account selection

Required output:

- user-facing explanation
- customer action
- support-safe evidence
- email subject
- email body
- CTA URL
- confidence

### `upstream_platform_issue`

Use when the provider appears degraded or is returning errors outside UniPost or customer control.

Required output:

- provider
- observed symptoms
- affected count
- whether retry is expected
- admin monitoring recommendation

### `transient_no_action`

Use when UniPost has already retried or will retry, and no admin or user action is currently needed.

Required output:

- reason no action is needed
- retry status if available
- when to re-open if failures continue

### `known_duplicate`

Use when the same root cause already has an open triage item or known issue from a previous run.

Required output:

- duplicate-of item ID
- latest evidence
- whether impact increased

### `needs_human_review`

Use when evidence is insufficient, confidence is low, or the proposed customer email could be unsafe.

Required output:

- why review is needed
- what evidence is missing
- recommended next inspection path

## Data Requirements

### New tables

Create persistent storage for triage runs and actions.

```text
error_triage_runs
- id
- run_type: scheduled | manual
- status: running | completed | failed
- window_start
- window_end
- timezone
- started_at
- completed_at
- model
- prompt_version
- failures_analyzed
- affected_users
- affected_workspaces
- summary
- error_message
- created_by_admin_id
- created_at
- updated_at
```

```text
error_triage_items
- id
- run_id
- dedupe_key
- classification
- status: pending_review | email_ready | email_sent | bug_plan_pending | bug_plan_approved | dismissed | no_action | needs_human_review
- confidence
- platform
- source
- error_code
- platform_error_code
- failure_stage
- affected_user_count
- affected_workspace_count
- affected_post_count
- latest_failure_at
- evidence_json
- ai_summary
- admin_notes
- bug_plan_json
- email_draft_json
- cta_url
- duplicate_of_item_id
- created_at
- updated_at
```

```text
error_triage_item_failures
- id
- item_id
- post_id
- social_post_result_id
- post_failure_id
- workspace_id
- user_id
- user_email
- platform
- created_at
```

```text
error_triage_email_sends
- id
- item_id
- recipient_scope_key
- recipient_user_id
- recipient_email
- loops_event_name
- loops_transactional_id
- idempotency_key
- subject_snapshot
- body_snapshot
- sent_by_admin_id
- sent_at
- provider_status
- provider_error
```

For user-actionable buckets that affect multiple dashboard customers, the triage item may represent the shared root cause, but email delivery is per recipient. The stored draft may be generated from a shared template plus recipient-specific variables, and each recipient gets a separate send row, idempotency key, subject snapshot, and body snapshot.

### Dedupe model

Use a stable `dedupe_key` per bucket so duplicate issues can be linked across runs. The key should be derived from normalized fields such as:

- classification candidate
- platform
- source
- error code
- platform error code
- failure stage
- normalized message fingerprint
- suspected code area when available

The key must not include raw user content or secrets.

## AI Input and Output

### Input

The AI classifier receives only sanitized diagnostic data:

- post failure fields
- platform
- source
- timestamps
- redacted error messages
- redacted `debug_curl` snippets
- retry metadata when available
- user/workspace counts
- captions only as truncated context when necessary

The prompt must explicitly forbid leaking raw provider payloads, tokens, or private customer content into emails.

### Output schema

The model response must be validated as structured JSON. Invalid output fails the item into `needs_human_review`.

Required top-level fields:

```json
{
  "classification": "user_action_needed",
  "confidence": 0.87,
  "summary": "The connected Threads account appears to need reconnection.",
  "evidence": ["Provider returned an auth-shaped error", "Failures affect one workspace"],
  "recommended_action": "Ask the user to reconnect Threads and retry the post.",
  "email_draft": {
    "subject": "Action needed: reconnect Threads in UniPost",
    "body": "..."
  },
  "bug_plan": null,
  "safety": {
    "contains_sensitive_content": false,
    "requires_human_review": false
  }
}
```

## Loops Email Requirements

### Delivery path

Add an admin-only backend endpoint:

```text
POST /v1/admin/error-triage/items/{id}/send-email
```

The endpoint should:

1. Require admin access.
2. Load the triage item and email draft.
3. Verify the item is `user_action_needed` and `email_ready`.
4. Verify the recipient is the dashboard customer or workspace owner from the stored failure data.
5. Use a deterministic idempotency key, such as `error_triage:{item_id}:{recipient_scope_key}`.
6. Send through Loops server-side.
7. Store a row in `error_triage_email_sends`.
8. Move the recipient send state to `sent`; move the item to `email_sent` only after every intended recipient has been sent or dismissed.

### Loops configuration

Introduce a dedicated Loops transactional email template for admin-approved triage emails:

```text
LOOPS_ERROR_TRIAGE_USER_ACTION_TRANSACTIONAL_ID
```

If the transactional ID is missing but Loops is enabled, the send endpoint should return a clear admin-facing configuration error instead of silently falling back to a generic event. This feature needs predictable email rendering and audit snapshots.

### Email content rules

Generated emails must:

- be short, calm, and action-oriented
- identify the affected platform and post when useful
- explain what the customer can do next
- include a dashboard CTA when available
- avoid raw provider JSON
- avoid debug curls
- avoid stack traces
- avoid access tokens, refresh tokens, API keys, cookies, request signatures, or authorization headers
- avoid blaming the customer
- avoid claiming UniPost fixed something unless it actually did

The admin page must show the exact subject and body snapshot before sending.

### Recipient rule

For v1, send to the UniPost dashboard customer email associated with the affected workspace owner. Do not send to `external_user_email` or third-party managed end users until a separate PRD covers customer-specific consent, support ownership, and privacy boundaries.

When one root-cause bucket affects several workspaces, the admin UI should show one row per intended recipient inside the item. Admins can send each recipient email independently. Bulk send is out of scope for v1.

## Admin API Requirements

Add admin endpoints:

```text
GET /v1/admin/error-triage/runs
GET /v1/admin/error-triage/runs/{id}
POST /v1/admin/error-triage/runs
POST /v1/admin/error-triage/runs/{id}/rerun
PATCH /v1/admin/error-triage/items/{id}
POST /v1/admin/error-triage/items/{id}/send-email
POST /v1/admin/error-triage/items/{id}/approve-bug-plan
```

`PATCH /items/{id}` supports admin notes, status changes, and dismissal.

`approve-bug-plan` marks a bug candidate as approved for implementation planning. It does not create a branch, commit, pull request, or deployment by itself.

## Admin UI Requirements

### Run list

Show recent runs with:

- run window
- run type
- status
- failures analyzed
- actionable item count
- email draft count
- bug candidate count
- completed time

### Run detail

Show:

- report summary
- health verdict
- top platforms
- top normalized error buckets
- user-action-needed queue
- UniPost bug queue
- needs-review queue
- no-action bucket summary

### Item detail

Show:

- classification and confidence
- evidence list
- affected users/workspaces/posts
- latest failure timestamp
- sample redacted error messages
- link to `/admin/errors` with matching filters
- generated email draft if present
- generated bug plan if present
- admin notes
- audit history

### One-click send

For `email_ready` items:

- primary button: `Send via Loops`
- confirmation modal: show recipient, subject, and body
- success state: `Sent`
- failure state: provider error with retry option

The button must be disabled if Loops is not configured, if the draft requires human review, if the item has already been sent to that recipient, or if the recipient email is missing.

Bulk email send is intentionally not included in v1. The one-click action applies to one previewed recipient at a time.

## Scheduler Requirements

The scheduler should run in the backend environment, not in the dashboard browser.

Preferred implementation:

- a backend worker started by the API process
- PT timezone calculation using `America/Los_Angeles`
- Postgres advisory lock around scheduled execution
- one canonical scheduled run per PT day
- manual admin-triggered runs supported separately

The scheduler must be safe with multiple replicas. If the hosting layer later provides a dedicated cron service, the same run creation service should be reused so the behavior remains identical.

## Safety and Privacy

1. Store raw diagnostic evidence only in admin-only tables.
2. Store email subject/body snapshots exactly as sent.
3. Redact secrets before AI input.
4. Redact secrets before admin display where possible.
5. Never include debug curls or raw provider payloads in customer emails.
6. Require admin access for all triage endpoints.
7. Require explicit click before Loops email delivery.
8. Use idempotency keys to prevent duplicate sends.
9. Persist model name and prompt version for audit.
10. If classification confidence is below the configured threshold, mark the item `needs_human_review`.

## Feature Flag Decision

This feature touches backend automation, admin UI, AI generation, and customer email delivery. Before implementation starts, ask for explicit approval to create feature flags according to the repo workflow.

Recommended flags if approved:

```text
ops.error_triage_autopilot_v1
ops.error_triage_loops_send_v1
```

Recommended defaults:

```text
development: on
staging: on after local validation
production: off until the first reviewed launch
fallback: off in production
```

Rollback:

- Disable `ops.error_triage_autopilot_v1` to stop scheduled generation and hide the admin page.
- Disable `ops.error_triage_loops_send_v1` to keep reports visible but prevent Loops sends.

If feature flags are not approved, implement the feature with conservative production defaults, but keep Loops sends admin-click-only and idempotent.

## Observability

Log and expose:

- scheduled run started
- scheduled run skipped because a run already exists
- advisory lock acquisition failure
- failure query count
- AI classification success/failure
- invalid AI output
- run completed
- run failed
- Loops email send requested
- Loops email send succeeded
- Loops email send failed
- bug plan approved
- item dismissed

Metrics should include:

- triage run duration
- failures analyzed per run
- classification counts
- email drafts generated
- emails sent
- Loops send failures
- model errors
- needs-human-review rate

## Validation Plan

### Backend

- Unit test PT window calculation across DST transitions.
- Unit test dedupe key generation.
- Unit test classifier output validation.
- Unit test secret redaction before AI input and email output.
- Unit test Loops send idempotency.
- Unit test admin permission checks.
- Integration test run creation from seeded failures.
- Integration test no-failure run.
- Integration test duplicate scheduled run prevention.

### Dashboard

- Build the admin page.
- Test run list, run detail, item detail, empty state, running state, failed state, and sent state.
- Test one-click send disabled states.
- Test responsive admin layout using existing `AdminShell` conventions.

### End-to-end

- Seed a user-action-needed failure and verify a draft is generated.
- Send the draft through a Loops test template in development.
- Seed a UniPost-bug-like failure and verify a bug plan is generated but no code action happens.
- Seed a no-action transient failure and verify no email draft is generated.
- Verify repeated send attempts do not duplicate Loops emails.

## Acceptance Criteria

1. `/admin/error-triage` appears in the admin sidebar under `System`.
2. A scheduled triage run is created once per PT day at 12:00 AM PT.
3. The scheduled run analyzes the previous PT day only.
4. A no-error or no-action day still produces a completed report.
5. Failure buckets are classified into the defined taxonomy.
6. User-actionable items generate safe customer email drafts.
7. UniPost bug candidates generate structured repair plans.
8. The daily job does not automatically email customers.
9. The daily job does not automatically create branches, commits, pull requests, or deployments.
10. Admins can send an eligible email draft via Loops with one click after reviewing the preview.
11. Every Loops send is idempotent and audited.
12. Customer emails never include debug curls, raw provider payloads, tokens, stack traces, or hidden internal notes.
13. Admins can dismiss items and add notes.
14. Admins can approve bug plans for later implementation planning.
15. All required backend tests pass.
16. `npm run build` passes for dashboard changes.
17. Development deployment is validated against the new admin page and Loops send flow before reporting implementation complete.

## Phased Rollout

### Phase 1 - Data and report foundation

- Add triage tables.
- Add daily run service.
- Add failure grouping and deterministic no-AI summary fallback.
- Add admin APIs for runs and items.

### Phase 2 - AI classification

- Add structured AI classifier.
- Persist prompt version, model, report, and item output.
- Add low-confidence handling.
- Add redaction tests.

### Phase 3 - Admin UI

- Add `/admin/error-triage`.
- Add sidebar entry.
- Add run list, run detail, item detail, notes, dismiss, and bug-plan approval.

### Phase 4 - Loops send

- Add dedicated Loops transactional ID configuration.
- Add send endpoint.
- Add one-click send preview and audit states.
- Verify delivery in development with a Loops test template.

### Phase 5 - Production launch

- Enable report generation first.
- Review several daily runs without sending emails.
- Enable Loops send after draft quality is acceptable.
- Keep automatic email sending out of scope.
