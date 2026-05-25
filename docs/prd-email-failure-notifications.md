# PRD - Failed-log AI Debug and unified email templates

**Status:** Planning
**Owner:** Notifications / Publishing / Logs
**Created:** 2026-05-25
**Target:** Dashboard Logs AI Debug and failure email quality upgrade

---

## Problem

UniPost already records rich failed publishing logs and sends user-facing notification emails for `post.failed`, but the product splits the recovery path awkwardly:

- The email only says a post failed and sends the user back to the dashboard.
- The failed log contains the evidence needed for diagnosis, including metadata and redacted provider request/response payloads.
- There is no user-facing `AI Debug` action in Logs that turns the failed log JSON into a root cause analysis and recommended solution.

The May 25 Threads failure investigation showed the desired product shape. The failed log already contained enough detail to identify an expired Threads session token. The user should get a clean email notification, then click into the exact failed log where AI can analyze the raw log JSON and explain the root cause and recommended fix in context.

## Product direction

Use email for notification. Use Logs for diagnosis.

For failed publishing errors:

1. Send the user an email that a publish failed.
2. Include an `AI Debug` link in that email.
3. The link opens Dashboard -> Logs -> the exact failed log row.
4. The log detail is expanded automatically.
5. The `AI Debug` panel is opened or ready to run.
6. AI receives the raw failed log JSON as the diagnostic input.
7. AI returns root cause analysis and recommended solution inside Logs, not inside the email.

This keeps email short and readable while making Logs the durable debugging workspace.

## Goals

1. Add an `AI Debug` feature to user Logs for failed log rows.
2. Send the raw failed log JSON to AI so it can produce root cause analysis and recommended solution.
3. Deep link failure emails to the exact dashboard failed log with the detail drawer expanded.
4. Keep failed-error user notifications email-only in v1.
5. Standardize all UniPost transactional emails around one reusable layout, tone, and visual system.
6. Preserve plain-text email alternatives for deliverability and accessibility.
7. Prevent emails from leaking debug curls, tokens, provider payloads, or internal worker details.
8. Keep notification delivery and AI debugging best-effort so publishing never blocks on email or AI.

## Non-goals

- No AI diagnosis embedded directly in failure emails.
- No Slack or Discord delivery for failed publishing errors in v1.
- No marketing newsletter system.
- No unsubscribe/preferences redesign beyond linking to existing notification settings.
- No direct browser connection to OpenAI or any LLM service.
- No frontend access to platform access tokens, refresh tokens, or unredacted provider secrets.
- No automatic reconnect on behalf of a user; users must explicitly reconnect OAuth accounts.
- No full Resend-hosted template migration in the first implementation unless the team chooses it later.

## Current codebase findings

### Existing notification flow

- `SupportedNotificationEvents` includes `post.failed`, `account.disconnected`, `billing.usage_80pct`, and `billing.payment_failed`.
- New users get a verified account-level email channel and default-on subscriptions through `ensureDefaultNotifications`.
- `NotificationDispatcher` fans matching events into `notification_deliveries`.
- `NotificationDeliveryWorker` drains `notification_deliveries` and sends email, Slack, or Discord webhook messages.
- The async publish queue calls `refreshParentPostStatusContext`, which publishes `post.failed` when the parent post becomes terminally failed.
- Delivery worker startup uses `ResendMailer` when `RESEND_API_KEY` is configured and `NoopMailer` otherwise.

### Existing logs flow

- User logs are served by `GET /v1/logs` and `GET /v1/logs/{id}`.
- The dashboard user Logs page lives at `/projects/{profile_id}/logs`.
- The API already has workspace-scoped `GetIntegrationLog`, so AI Debug can enforce the same workspace access boundary as log detail.
- Publishing failures are logged as integration log action `post.publish.platform_failed`.
- The failed log includes fields such as `message`, `metadata`, `response_payload`, `post_id`, `social_account_id`, `platform`, `error_code`, and timestamps.

### Current email limitation

`renderEmail` currently hardcodes each template inside `api/internal/worker/notification.go`. The `post.failed` email includes:

- generic subject
- generic body
- truncated caption
- dashboard link

It does not include:

- a link to the exact failed log
- an expanded log detail state
- an `AI Debug` entry point
- a unified email layout shared with other UniPost emails

### Existing diagnostic inputs

UniPost already stores useful failure evidence:

- `integration_logs.metadata`
- `integration_logs.request_payload`
- `integration_logs.response_payload`
- `social_post_results.error_message`
- `social_post_results.debug_curl`
- `post_failures.error_code`
- `post_failures.failure_stage`
- `post_failures.is_retriable`
- post delivery job metadata such as `attempts`, `max_attempts`, `retriable`, and `another_attempt`

For AI Debug v1, the primary source of truth should be the integration log JSON returned by `GET /v1/logs/{id}` with payloads included. The AI request should use the exact failed log JSON object after existing token redaction has been applied.

## External design references

Use these as constraints, not as pixel-perfect designs:

- Resend supports stored templates with variables and can import HTML or React Email files; this is useful if UniPost later wants provider-managed versioning instead of only backend-rendered HTML ([Resend Templates](https://resend.com/docs/dashboard/templates/introduction)).
- React Email provides email-safe components and Tailwind support; its Tailwind docs call out email client style limitations and the pixel-based preset for compatibility ([React Email Tailwind](https://react.email/docs/components/tailwind)).
- React Email also offers composable primitives like containers, sections, text, links, and buttons that fit a shared transactional layout ([React Email Components](https://react.email/components)).
- Mailchimp's transactional guidance emphasizes concise, scannable copy, clear labels/buttons, and user-focused next steps ([Mailchimp transactional best practices](https://mailchimp.com/resources/transactional-email-examples/)).
- Mailchimp's HTML email guidance recommends multipart plain text, simple code, public absolute assets, email width around 600px or fluid, inline CSS, and rendering tests across clients ([Mailchimp HTML email guide](https://mailchimp.com/help/about-html-email/)).

## User experience

### User story: expired Threads token

When a Threads post fails because the access token is expired:

1. The user receives a short failure email.
2. The email subject is clear but not overly technical: `Action needed: a Threads post failed`.
3. The email says the post could not be delivered and offers one primary CTA: `Open AI Debug`.
4. The CTA opens:

```text
/projects/{profile_id}/logs?log_id={integration_log_id}&expand=1&ai_debug=1
```

5. Dashboard Logs loads the failed log row, opens the detail drawer, and shows the `AI Debug` panel.
6. If `ai_debug=1`, the panel may auto-run once the log loads, or show a focused `Run AI Debug` button if the product wants explicit user action.
7. AI receives the raw failed log JSON and returns:

- root cause analysis
- recommended solution
- confidence
- supporting evidence from the log
- what to do next

The email must not expose debug curls, raw provider JSON, or root cause analysis. That belongs in Logs.

### User story: transient platform outage

When UniPost has a failed log caused by a temporary platform issue:

- The email remains a short notification with `Open AI Debug`.
- Logs AI Debug can explain whether UniPost already retried, will retry, or needs user action.
- If the failed log shows `another_attempt=true`, the AI response should say no action is needed yet unless retries are exhausted.

### User story: validation or media issue

When the failed log points to validation or media problems:

- The email still only links to AI Debug.
- Logs AI Debug explains the likely field/media issue and recommends editing the post or media before retrying.

## Product requirements

### 1. Email-only failed-error notification

For failed publishing errors, v1 should notify the user by email only.

Requirements:

- `post.failed` notification deliveries should target verified `email` channels only.
- Slack and Discord webhook notification channels should not receive failed publishing error notifications in v1.
- Existing non-failure notification events may keep their current channel behavior unless separately changed.
- The email should be short, calm, and focused on the deep link to Logs.

Implementation options:

1. Filter `post.failed` fanout to email channels in `NotificationDispatcher`.
2. Or add an event descriptor setting such as `AllowedChannelKinds: ["email"]` and apply it during target resolution.

The second option is preferred because it keeps channel policy declarative and easier to extend.

### 2. Failure email deep link

Failure emails must include a stable `AI Debug` link to the exact failed log.

Required URL shape:

```text
{APP_BASE_URL}/projects/{profile_id}/logs?log_id={integration_log_id}&expand=1&ai_debug=1
```

Requirements:

- `profile_id` must point to a dashboard project that can view the failed log.
- `integration_log_id` must identify the exact failed log row.
- `expand=1` tells the Logs page to open the detail drawer.
- `ai_debug=1` tells the Logs page to focus the AI Debug panel.
- If the log is unavailable or expired, the dashboard should show a friendly empty state and a link back to Logs filtered by `status=error`.

The email may include a secondary `View logs` text link, but the primary CTA should be `Open AI Debug`.

### 3. Capture integration log ID for email payloads

The failure email cannot deep link to the exact failed log unless the notification payload knows the log ID.

Requirements:

- The publishing failure logging path should return or expose the inserted `integration_logs.id`.
- The terminal `post.failed` notification payload should include:

```json
{
  "failed_log": {
    "id": 51597,
    "action": "post.publish.platform_failed",
    "status": "error",
    "level": "error",
    "platform": "threads",
    "post_id": "b04def14-ded7-42f7-8916-fee42cdd9db7",
    "profile_id": "profile_123"
  }
}
```

- If several platform logs failed for the same post, choose the newest terminal failed log with the highest severity for the primary email link.
- Include a compact count such as `2 platforms failed` in the email only when multiple failed logs exist.

### 4. Logs AI Debug action

Add an `AI Debug` action to failed log details in the user dashboard.

Location:

```text
Dashboard -> Project -> Logs -> failed log detail drawer
```

Interaction requirements:

- Show `AI Debug` only for logs where `status=error` or `level=error`.
- The control should be an action button in the detail drawer header or an adjacent section, not a large marketing panel.
- The drawer should keep raw payload sections accessible, but the AI Debug result should appear above raw JSON once generated.
- Loading state should use a compact skeleton or inline progress state.
- Error state should say the AI analysis could not be generated and keep the raw log visible.
- The button label should be `AI Debug`.
- Do not use decorative AI-purple styling; use the dashboard's existing neutral/action styling.

### 5. AI Debug API

Add a workspace-scoped endpoint that runs AI Debug for a single log.

Recommended endpoint:

```http
POST /v1/logs/{id}/ai-debug
```

Input:

- No request body is required.
- The server loads the log by `id` and current workspace access.

Output:

```json
{
  "data": {
    "log_id": 51597,
    "analysis_id": "laid_123",
    "status": "completed",
    "root_cause": "Threads rejected the request because the connected account token expired.",
    "recommended_solution": "Reconnect Threads in UniPost, then retry the failed post.",
    "confidence": "high",
    "evidence": [
      "response_payload.error contains 'Session has expired'",
      "provider returned OAuthException code 190",
      "metadata.retriable is false"
    ],
    "next_steps": [
      "Open account settings and reconnect Threads",
      "Return to the failed post and retry after reconnecting"
    ],
    "generated_at": "2026-05-25T19:12:00Z"
  }
}
```

The endpoint should be idempotent for the same log. If a completed analysis already exists, return it unless the caller requests regeneration.

### 6. Raw JSON AI input

AI Debug should pass the raw failed log JSON to AI.

Raw means the complete log object available to the authenticated user from `GET /v1/logs/{id}`, including request and response payload fields when present. It does not mean decrypted tokens or unredacted provider secrets.

Requirements:

- Use the same redacted payload shape shown in the Logs UI.
- Do not add decrypted access tokens, refresh tokens, Authorization headers, or internal secrets.
- Preserve original field names and nested structure so the AI can reason from the real log.
- Include the log category, action, status, metadata, request payload, response payload, and relevant IDs.
- Include an explicit system instruction that the model must not ask the user for secrets.
- Include an explicit system instruction that it should cite fields from the provided JSON as evidence.

Prompt contract:

```text
You are UniPost AI Debug. Analyze the raw failed integration log JSON.
Return root cause analysis and recommended solution for the end user.
Use only evidence present in the JSON. Do not request secrets or tokens.
Do not reveal raw access tokens, Authorization headers, or debug curl text.
```

### 7. AI output schema and validation

The AI output must fit a strict JSON schema:

```json
{
  "root_cause": "string",
  "recommended_solution": "string",
  "confidence": "high | medium | low",
  "evidence": ["string"],
  "next_steps": ["string"]
}
```

Validation requirements:

- `root_cause`: 40-700 characters.
- `recommended_solution`: 40-700 characters.
- `confidence`: one of `high`, `medium`, `low`.
- `evidence`: 1-6 items.
- `next_steps`: 1-5 items.
- Reject outputs containing `access_token=`, `Authorization: Bearer`, `refresh_token`, or raw debug curl command text.
- Reject outputs that ask users to paste secrets into UniPost.
- On validation failure, return a safe generic response and log the validation error server-side.

### 8. AI Debug persistence

Persist generated AI Debug results so users and support see consistent answers.

Recommended table:

```text
integration_log_ai_debug
```

Suggested fields:

```text
id
workspace_id
integration_log_id
model
input_hash
root_cause
recommended_solution
confidence
evidence_json
next_steps_json
created_at
updated_at
regenerated_at
```

Requirements:

- One current result per `(workspace_id, integration_log_id, input_hash)`.
- If the underlying log JSON changes or renderer shape changes, a new input hash can produce a new result.
- Store AI output, not full AI prompt input.
- Never store unredacted secrets.

### 9. Unified email template system

Replace ad hoc string templates with a small shared template layer.

Recommended package:

```text
api/internal/mail/templates
```

Core API:

```go
type TemplateData struct {
    Preheader string
    Title     string
    Intro     string
    Severity  string
    Sections  []Section
    PrimaryCTA *CTA
    SecondaryCTA *CTA
    FooterLinks []Link
}

func RenderTransactional(data TemplateData) mail.Message
```

All transactional emails should use this shared layout:

- welcome email
- paid activation email
- notification test email
- failed publishing email
- `account.disconnected`
- `billing.usage_80pct`
- `billing.payment_failed`

The first migration can keep routing and event semantics unchanged while moving rendering into the shared template package.

### 10. Failed publishing email content

The failed publishing email should be intentionally lightweight.

It should include:

- a clear subject
- a short summary that a post failed
- post caption preview
- platform when known
- account name when known
- time of failure
- primary CTA: `Open AI Debug`
- secondary link: `View logs`
- footer with notification settings and support links

It should not include:

- AI root cause analysis
- recommended solution
- raw provider payload
- debug curl
- worker/job metadata
- access tokens or provider secrets

Example:

```text
Subject: Action needed: a Threads post failed
Preheader: Open AI Debug to review the failed log and recommended fix.

A Threads post failed

UniPost could not deliver one of your posts to Threads. Open AI Debug to review the failed log, root cause analysis, and recommended solution in your dashboard.

[Open AI Debug]

Post: What nobody tells you about lower middle market investing
Platform: Threads
Account: Natu
Failed at: May 25, 2026 11:30 AM PDT

You received this because publishing alerts are enabled for this workspace.
Manage notifications | Contact support
```

Plain text should carry the same content and include the full AI Debug URL.

### 11. Email visual and copy standard

All UniPost transactional emails should follow these rules:

- 600px max-width centered content.
- Single-column layout.
- White content surface on a quiet light-gray page background.
- No hero images for operational alerts.
- Inline CSS only in final HTML.
- System sans-serif stack.
- Comfortable spacing: 24px outer padding, 20-24px section spacing, 16px body text.
- One primary CTA button.
- Optional secondary text link.
- A short preheader.
- Clear event label near the top, such as `Publishing alert` or `Billing alert`.
- No emojis in email content.
- No debug payloads in visible copy.
- Plain-text version for every email.
- Footer includes UniPost name, why the user got the email, settings link, and support link.

Suggested palette:

```text
Page background: #f6f7f9
Surface:         #ffffff
Text:            #17202a
Muted text:      #5f6b7a
Border:          #e6eaf0
Primary CTA:     #1769aa
Warning accent:  #b7791f
Critical accent: #b42318
Success accent:  #087f5b
```

The tone should be calm and useful:

- Prefer: `Open AI Debug to review the failed log`
- Avoid: `Platform delivery job failed`
- Prefer: `A Threads post failed`
- Avoid: `OAuthException code 190`

### 12. Deep-link behavior in dashboard Logs

The Logs page must understand these query params:

```text
log_id={integration_log_id}
expand=1
ai_debug=1
```

Requirements:

- Load or fetch the target log if it is not in the current paginated list.
- Highlight the target row.
- Open the detail drawer when `expand=1`.
- Focus the AI Debug section when `ai_debug=1`.
- If the target log cannot be loaded, show a non-blocking message and keep the user in Logs.
- Keep browser back/forward behavior sane by preserving or clearing query params intentionally when the drawer closes.

### 13. Safety and privacy

Email content must never include:

- raw `debug_curl`
- access tokens
- refresh tokens
- Authorization headers
- provider request URLs containing token query params
- raw provider JSON
- stack traces
- internal-only worker IDs unless framed as a support code

AI Debug may inspect the raw failed log JSON that is already visible to the authenticated user, but it must not reveal hidden secrets or introduce sensitive data not present in that visible log JSON.

Support codes are allowed if short and stable:

```text
Support code: failed_log:threads:51597
```

### 14. Feature flags

Because this touches dashboard logs, AI calls, and notification routing, ship behind backend and frontend-visible feature flags.

Primary flag:

```text
logs.ai_debug_v1
```

Defaults:

```text
development: on
production: off
fallback: off
```

Email deep-link flag:

```text
notifications.failed_email_ai_debug_link_v1
```

Defaults:

```text
development: on
production: off
fallback: off
```

Owner area: Notifications / Publishing / Logs. Production rollback is disabling the flags, which returns failed publishing notifications to the existing generic dashboard link and hides the Logs AI Debug action.

### 15. Observability

Add structured logs or counters for:

- AI Debug requested
- AI Debug completed
- AI Debug failed
- AI output validation failed
- cached AI Debug result returned
- failed email rendered with AI Debug link
- failed email missing log ID fallback used
- notification delivery status

Do not log full email bodies, full AI prompts, or raw debug curls.

### 16. Acceptance criteria

1. Failed publishing errors send user notification email only in v1.
2. Failure emails include an `Open AI Debug` CTA.
3. The CTA opens `/projects/{profile_id}/logs?log_id={integration_log_id}&expand=1&ai_debug=1`.
4. Dashboard Logs opens the exact failed log drawer from the email link.
5. Logs shows an `AI Debug` action for failed/error logs.
6. `POST /v1/logs/{id}/ai-debug` sends the raw visible failed log JSON to AI.
7. AI Debug returns root cause analysis and recommended solution in a validated schema.
8. AI Debug results are persisted or cached so refreshing the drawer does not regenerate unnecessarily.
9. Failure emails do not include AI analysis, recommended solution, raw provider payload, or debug curl.
10. All existing transactional email types render through one shared email layout.
11. Every email has HTML and text output.
12. Unit tests assert no rendered failure email includes `access_token`, `Authorization`, raw `debug_curl`, or provider JSON.
13. Unit tests cover AI output validation, forbidden secret patterns, cache hit behavior, and deep-link URL generation.
14. Dashboard regression coverage verifies the Logs deep link opens and expands the target failed log when Playwright browsers are available.
15. Production can disable AI Debug and email deep links independently with feature flags.

## Rollout plan

### Phase 1: email deep link foundation

- Make publishing failure logging return or expose `integration_log_id`.
- Add failed log metadata to `post.failed` notification payloads.
- Restrict failed publishing notifications to email channels.
- Render failed publishing email through the unified template with `Open AI Debug` CTA.
- Add deep-link URL generation tests.

### Phase 2: Logs deep-link behavior

- Teach dashboard Logs to parse `log_id`, `expand`, and `ai_debug`.
- Fetch the target log if needed.
- Open and highlight the failed log detail drawer.
- Add the initial AI Debug panel shell behind `logs.ai_debug_v1`.

### Phase 3: AI Debug API and UI

- Add `POST /v1/logs/{id}/ai-debug`.
- Pass the raw visible failed log JSON to AI.
- Validate and persist AI output.
- Render root cause analysis, recommended solution, evidence, confidence, and next steps in the log detail drawer.

### Phase 4: unified email templates

- Move notification test email, account disconnected, billing usage, and billing payment failure into the shared template.
- Move welcome and paid activation emails into the shared template.
- Add golden tests for HTML and text rendering.

## Open questions

1. Should `ai_debug=1` auto-run AI Debug, or should it open the panel with a focused `Run AI Debug` button for explicit user consent?
2. Should AI Debug be available for all error logs or only publishing failures in v1?
3. Should workspace admins receive all workspace failure emails, or only the workspace owner/account-level notification user as today?
4. Should the AI Debug endpoint support `force_regenerate=true` for support/admin users?
5. Should Slack and Discord failed-error notifications be disabled only for `post.failed`, or for every notification event with `status=error`?

## Implementation notes

- Keep the rendering code independent from Resend. `mail.Mailer` should still accept a fully rendered `mail.Message`.
- If React Email is adopted for authoring, render generated HTML into Go fixtures or a build artifact rather than adding a Node runtime dependency to email delivery.
- Prefer deterministic Go template rendering for v1 because current delivery runs inside the Go API worker.
- Add a small email preview command or test fixture output so design reviews do not require sending real emails.
- Treat raw log JSON as the visible, redacted log object returned to the authenticated user, not as a reason to attach unredacted secrets.
- Keep the Logs AI Debug panel compact and task-focused: root cause, recommended solution, evidence, and next steps.
