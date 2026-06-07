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
5. Classify buckets as UniPost bug, user action needed, upstream platform issue, transient/no action, or needs human review.
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
- `social_post_results.debug_curl` may contain important evidence. It is intended to be written through `internal/debugrt`, which redacts sensitive headers and query params before storage; admin APIs currently pass the stored value through. Phase 1 must verify that every `debug_curl` value eligible for triage was captured through this redaction path, including older rows and any non-debugrt writers.
- `post_failures` is a better long-term triage source than scraping `/admin/errors`, because it is structured, indexed, and independent of UI rendering.

### Loops

- `api/internal/loops/client.go` supports contact upsert, event send, and transactional email send.
- `api/internal/loops/syncer.go` gates Loops lifecycle behavior through `email.loops_integration_v1`.
- Current transactional IDs include plan changed, account canceled, and post failed.
- Existing post-failed Loops events are customer notification oriented; this feature needs a separate admin-approved support email path with its own idempotency and audit trail.
- The triage page must account for existing `post_failed` customer notifications so admin-approved follow-up emails do not read like a duplicate first notification.

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

- Run status card: `Completed`, `Running`, `Failed`, `No actionable issues`, or `Needs review`. The stored run status remains a small execution-state enum; `No actionable issues` and `Needs review` are UI health verdicts derived from item counts.
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
7. Mark the run `completed` or `failed`, with review state exposed through the derived `health_status`.

Storage uses `status=completed` for both clean and review-needed runs. The admin API returns a derived `health_status` such as `no_actionable_issues`, `actionable_items`, or `needs_review` so the UI does not overload the run execution status.

### Manual run

Admins can click `Run now` for the trailing 24 hours. Manual runs are useful after deploys or after fixing a classifier prompt. Manual runs must be labeled as manual and must not replace the canonical scheduled run for the PT day unless the admin explicitly chooses `Re-run daily report`.

API mapping:

- `POST /v1/admin/error-triage/runs` creates a manual trailing-24-hour run.
- `POST /v1/admin/error-triage/runs/{id}/rerun` re-runs the same window and run type as an existing run, preserving a link to the superseded run.

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

### `needs_human_review`

Use when evidence is insufficient, confidence is low, or the proposed customer email could be unsafe.

Required output:

- why review is needed
- what evidence is missing
- recommended next inspection path

### Duplicate handling

Known-duplicate detection must be deterministic, not model-generated. Before invoking AI for a bucket, the triage service computes a `dedupe_key` and checks previous triage items. If an open or recently resolved item with the same key exists, the new item links `duplicate_of_item_id` to that prior item and stores the latest evidence. The AI may receive duplicate context as supporting information, but it must not invent `duplicate_of_item_id` values.

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
- supersedes_run_id
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

Add a database-level duplicate guard for scheduled runs:

```text
UNIQUE (window_start) WHERE run_type = 'scheduled'
```

The advisory lock prevents duplicate work across API replicas; the unique index prevents duplicate scheduled rows if a process crashes or two schedulers race.

```text
error_triage_items
- id
- run_id
- dedupe_key
- classification: unipost_bug | user_action_needed | upstream_platform_issue | transient_no_action | needs_human_review
- action_kind: none | email | bug_plan | monitor | review
- workflow_status: pending_review | ready | partially_completed | completed | dismissed | failed
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

`classification` describes what kind of problem the bucket is. `workflow_status` describes operator progress. They must not duplicate each other. For example, a `user_action_needed` item can move from `ready` to `partially_completed` to `completed`, while a `transient_no_action` item is usually created as `completed` with `action_kind=none`.

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
error_triage_item_recipients
- id
- item_id
- recipient_scope_key
- workspace_id
- recipient_user_id
- email_snapshot
- status: pending | sent | dismissed | send_failed
- latest_send_attempt_id
- dismissed_by_admin_id
- dismissed_at
- dismiss_reason
- created_at
- updated_at
```

```text
error_triage_email_sends
- id
- item_id
- recipient_id
- recipient_scope_key
- recipient_user_id
- recipient_email
- attempt_number
- loops_event_name
- loops_transactional_id
- idempotency_key
- subject_snapshot
- body_snapshot
- sent_by_admin_id
- sent_at
- provider_status
- provider_error
- created_at
```

For user-actionable buckets that affect multiple dashboard customers, the triage item may represent the shared root cause, but email delivery is per recipient. The stored draft may be generated from a shared template plus recipient-specific variables. Each recipient gets a row in `error_triage_item_recipients`, and each send attempt gets a row in `error_triage_email_sends`.

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

### Provider and model

V1 should reuse the existing server-side AI integration pattern unless implementation discovers a strong reason to introduce a shared provider abstraction first. The current codebase already has OpenAI JSON-output usage in `api/internal/handler/ai_post_assist.go`; Error Triage should follow the same server-only secret model and make model name configurable through environment.

The PRD does not require frontend access to AI providers. The admin page only calls UniPost backend APIs.

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

Input limits must be explicit and logged:

- default maximum buckets sent to AI per run: 100
- default maximum representative failures per bucket: 25
- default maximum serialized prompt input per bucket: 40,000 characters
- if a bucket exceeds the cap, summarize deterministic aggregates and sample the newest and most frequent representative failures
- persist a `truncated=true` signal in `evidence_json` when any cap is applied
- never drop aggregate counts just because samples are capped

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
POST /v1/admin/error-triage/items/{id}/recipients/{recipient_id}/send-email
```

The endpoint should:

1. Require admin access.
2. Load the triage item and email draft.
3. Require an explicit `recipient_id`.
4. Verify the item is `classification=user_action_needed`, `action_kind=email`, and `workflow_status=ready` or `partially_completed`.
5. Verify the recipient row is `pending` or `send_failed`.
6. Reload the recipient user's current email from `users` at send time, and keep `email_snapshot` only as audit context.
7. Verify the recipient is the dashboard customer or workspace owner from the stored failure data.
8. Create a new `error_triage_email_sends` attempt row.
9. Use a deterministic provider idempotency key, such as `error_triage:{item_id}:{recipient_scope_key}`.
10. Send through Loops server-side.
11. Mark the attempt `succeeded` or `failed`.
12. If delivery succeeds, move the recipient status to `sent`.
13. If delivery fails, move the recipient status to `send_failed` and keep the retry button available.
14. Move the item workflow status to `completed` only after every intended recipient has been sent or dismissed.

### Retry semantics

Retries after a failed provider attempt reuse the same Loops idempotency key for that item/recipient. This prevents duplicate customer emails if Loops accepted the original request but UniPost received a timeout or ambiguous error.

`error_triage_email_sends` stores one row per attempt. Failed attempts do not disable the send button. The UI disables sending only when the recipient status is `sent` or `dismissed`, when the item is no longer send-eligible, when Loops is not configured, or when the recipient email cannot be resolved.

If an admin edits or regenerates the email draft after a failed send, the backend should create a new draft version and a new idempotency key suffix, such as:

```text
error_triage:{item_id}:{recipient_scope_key}:draft:{draft_version}
```

Draft editing is optional in v1, but the retry semantics must leave room for it.

### Loops configuration

Introduce a dedicated Loops transactional email template for admin-approved triage emails:

```text
LOOPS_ERROR_TRIAGE_USER_ACTION_TRANSACTIONAL_ID
```

If the transactional ID is missing but Loops is enabled, the send endpoint should return a clear admin-facing configuration error instead of silently falling back to a generic event. This feature needs predictable email rendering and audit snapshots.

### Email content rules

Generated emails must:

- be short, calm, and action-oriented
- be framed as an admin-reviewed follow-up when the customer has already received the automatic `post_failed` notification
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

The item detail should show whether UniPost already sent the customer an automatic `post_failed` notification for the same post or bucket when that can be determined from existing notification delivery data or Loops audit data. If this cannot be determined, the page should display `Prior notification unknown` rather than assume no notification was sent.

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
POST /v1/admin/error-triage/items/{id}/recipients/{recipient_id}/send-email
POST /v1/admin/error-triage/items/{id}/approve-bug-plan
```

`PATCH /items/{id}` supports admin notes, status changes, and dismissal.

`approve-bug-plan` marks a bug candidate as approved for implementation planning. It does not create a branch, commit, pull request, or deployment by itself.

`POST /runs` maps to the admin `Run now` action. `POST /runs/{id}/rerun` maps to `Re-run daily report` or `Re-run this manual window`, depending on the source run.

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

For user-action-needed items with a `pending` or `send_failed` recipient:

- primary button: `Send via Loops`
- confirmation modal: show recipient, subject, and body
- success state: `Sent`
- failure state: provider error with retry option

The button must be disabled if Loops is not configured, if the draft requires human review, if the recipient status is `sent` or `dismissed`, if the item workflow status is not send-eligible, or if the recipient email is missing.

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

### Redaction preflight

Phase 1 must verify the actual `debug_curl` redaction path before any `debug_curl` value is sent to AI. The implementation should confirm:

- adapters that write `debug_curl` use `internal/debugrt` or an equivalent redaction function
- legacy rows that cannot be proven redacted are excluded from AI input or redacted again
- redaction covers authorization headers, bearer tokens, cookies, token-like query params, request signatures, and common provider secrets
- tests fail if a rendered AI input or customer email contains secret-shaped values

### Data retention

Persist enough history for support and product quality, but do not keep PII-heavy evidence forever.

Recommended retention:

- keep full `evidence_json`, copied `user_email`, and sampled failure payloads for 180 days
- keep run summaries, aggregate counts, classifications, bug plans, and email send audit snapshots for 13 months
- after the evidence retention window, redact or delete PII-heavy fields while preserving aggregate reporting
- never delete `error_triage_email_sends` audit rows before the configured support/audit retention period

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
- Unit test `debug_curl` redaction preflight behavior.
- Unit test recipient workflow state transitions.
- Unit test Loops send idempotency and failed-attempt retry behavior.
- Unit test admin permission checks.
- Integration test run creation from seeded failures.
- Integration test no-failure run.
- Integration test duplicate scheduled run prevention.
- Integration test scheduled run unique-index collision handling.
- Integration test deterministic duplicate linking by `dedupe_key`.

### Dashboard

- Build the admin page.
- Test run list, run detail, item detail, empty state, running state, failed state, and sent state.
- Test one-click send disabled states.
- Test per-recipient pending, sent, dismissed, and send-failed states.
- Test prior-post-failed-notification display states.
- Test responsive admin layout using existing `AdminShell` conventions.

### End-to-end

- Seed a user-action-needed failure and verify a draft is generated.
- Send the draft through a Loops test template in development.
- Seed a UniPost-bug-like failure and verify a bug plan is generated but no code action happens.
- Seed a no-action transient failure and verify no email draft is generated.
- Verify repeated send attempts do not duplicate Loops emails.
- Verify a failed Loops attempt leaves the recipient retryable.

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
13. Multi-recipient buckets store and display independent recipient states.
14. Failed Loops attempts can be retried without permanently disabling that recipient.
15. Scheduled runs have both advisory-lock protection and a database-level uniqueness guard.
16. Duplicate items are linked deterministically by `dedupe_key`, not by AI invention.
17. Admins can dismiss items and add notes.
18. Admins can approve bug plans for later implementation planning.
19. All required backend tests pass.
20. `npm run build` passes for dashboard changes.
21. Development deployment is validated against the new admin page and Loops send flow before reporting implementation complete.

## Phased Rollout

### Phase 1 - Data and report foundation

- Add triage tables.
- Add scheduled-run uniqueness guard.
- Verify and enforce `debug_curl` redaction eligibility before AI input.
- Add daily run service.
- Add failure grouping and deterministic no-AI summary fallback.
- Add deterministic duplicate linking by `dedupe_key`.
- Add admin APIs for runs and items.

### Phase 2 - AI classification

- Add structured AI classifier.
- Persist prompt version, model, report, and item output.
- Enforce AI input caps and persist truncation metadata.
- Add low-confidence handling.
- Add redaction tests.

### Phase 3 - Admin UI

- Add `/admin/error-triage`.
- Add sidebar entry.
- Add run list, run detail, item detail, notes, dismiss, and bug-plan approval.

### Phase 4 - Loops send

- Add dedicated Loops transactional ID configuration.
- Add recipient table, send-attempt audit table, and send endpoint.
- Add one-click send preview and audit states.
- Add failed-send retry semantics.
- Verify delivery in development with a Loops test template.

### Phase 5 - Production launch

- Enable report generation first.
- Review several daily runs without sending emails.
- Enable Loops send after draft quality is acceptable.
- Keep automatic email sending out of scope.
