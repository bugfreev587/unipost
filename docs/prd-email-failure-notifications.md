# PRD - Failure-aware notifications and unified email templates

**Status:** Planning
**Owner:** Notifications / Publishing
**Created:** 2026-05-25
**Target:** Dashboard and API notification quality upgrade

---

## Problem

UniPost already sends user-facing notification emails for important events such as `post.failed`, but failure emails are too generic. A user who sees a platform failure like an expired Threads token gets an email that says a post failed and asks them to open the dashboard. The email does not explain what failed, why it likely failed, or what the user should do next.

The May 25 Threads failure investigation showed this exact gap:

- The platform delivery job failed while calling Threads Graph API.
- The concrete upstream reason was an expired Threads session token.
- UniPost recorded enough diagnostic evidence in integration logs and result payloads to understand the issue.
- The user notification delivery was sent, but the rendered email only included a generic failure notice and caption.

This is a trust problem. Users do not just need to know "something failed"; they need calm, specific guidance that helps them recover without asking support.

## Goals

1. Send post failure emails that include a user-safe diagnosis and recommended fix.
2. Standardize all UniPost transactional emails around one reusable layout, tone, and visual system.
3. Keep the first version deterministic for common failures, with optional AI analysis only when rules cannot produce a confident answer.
4. Make the email comfortable to read on mobile and desktop.
5. Preserve plain-text alternatives for deliverability and accessibility.
6. Avoid leaking debug curls, access tokens, raw provider payloads, or internal-only IDs in user emails.
7. Keep notifications best-effort so publishing and bootstrap flows never block on email rendering or delivery.

## Non-goals

- No marketing newsletter system.
- No unsubscribe/preferences redesign beyond linking to existing notification settings.
- No direct browser connection to OpenAI or any LLM service.
- No feature that lets the frontend receive raw platform access tokens or provider debug payloads.
- No automatic reconnect on behalf of a user; users must explicitly reconnect OAuth accounts.
- No full Resend-hosted template migration in the first implementation unless the team chooses it as a later operational step.

## Current codebase findings

### Existing notification flow

- `SupportedNotificationEvents` already includes `post.failed`, `account.disconnected`, `billing.usage_80pct`, and `billing.payment_failed`.
- New users get a verified account-level email channel and default-on subscriptions through `ensureDefaultNotifications`.
- `NotificationDispatcher` fans matching events into `notification_deliveries`.
- `NotificationDeliveryWorker` drains `notification_deliveries` and sends email, Slack, or Discord webhook messages.
- The async publish queue calls `refreshParentPostStatusContext`, which publishes `post.failed` when the parent post becomes terminally failed.
- Delivery worker startup uses `ResendMailer` when `RESEND_API_KEY` is configured and `NoopMailer` otherwise.

### Current email limitation

`renderEmail` currently hardcodes each template inside `api/internal/worker/notification.go`. The `post.failed` email includes:

- generic subject
- generic body
- truncated caption
- dashboard link

It does not include:

- platform name
- account name
- result-level error
- classified failure code
- retry status
- recommended recovery action
- support-safe diagnostic summary

### Existing diagnostic inputs

UniPost already stores useful failure evidence:

- `social_post_results.error_message`
- `social_post_results.debug_curl`
- `post_failures.error_code`
- `post_failures.failure_stage`
- `post_failures.is_retriable`
- integration log `response_payload.error`
- integration log `response_payload.debug_curl`
- post delivery job metadata such as `attempts`, `max_attempts`, `retriable`, and `another_attempt`

The notification payload currently carries `socialPostResponse` data. That includes per-result `error_message`, `platform`, `account_name`, `status`, and `social_account_id`, but does not include the richer `post_failures` taxonomy or recommended action.

## External design references

Use these as constraints, not as pixel-perfect designs:

- Resend supports stored templates with variables and can import HTML or React Email files; this is useful if UniPost later wants provider-managed versioning instead of only backend-rendered HTML ([Resend Templates](https://resend.com/docs/dashboard/templates/introduction)).
- React Email provides email-safe components and Tailwind support; its Tailwind docs call out email client style limitations and the pixel-based preset for compatibility ([React Email Tailwind](https://react.email/docs/components/tailwind)).
- React Email also offers composable primitives like containers, sections, text, links, and buttons that fit a shared transactional layout ([React Email Components](https://react.email/components)).
- Mailchimp's transactional guidance emphasizes concise, scannable copy, clear labels/buttons, and user-focused next steps ([Mailchimp transactional best practices](https://mailchimp.com/resources/transactional-email-examples/)).
- Mailchimp's HTML email guidance recommends multipart plain text, simple code, public absolute assets, email width around 600px or fluid, inline CSS, and rendering tests across clients ([Mailchimp HTML email guide](https://mailchimp.com/help/about-html-email/)).

## User experience

### User story: expired Threads token

When a Threads post fails because the access token is expired, the user receives a clear email:

- Subject: `Action needed: reconnect Threads to publish your post`
- Header: `Threads needs to be reconnected`
- Summary: `Your post could not be delivered because Threads says the account authorization expired.`
- Recommended fix: `Reconnect Threads in UniPost, then retry the post.`
- CTA: `Reconnect Threads`
- Secondary CTA: `View failed post`
- Details: platform, account name if available, post caption preview, failure time, whether UniPost will retry.

The email must not expose debug curls or the raw provider JSON.

### User story: transient platform outage

When a retriable failure happens and another attempt is already scheduled, the email should avoid panic:

- Subject: `Heads up: UniPost is retrying a failed Instagram delivery`
- Summary: `Instagram returned a temporary error. UniPost will retry automatically.`
- Recommended fix: `No action needed yet. Check the post if it still fails after retries.`
- CTA: `View delivery status`

### User story: validation or media issue

When the post is blocked by user-correctable content or media:

- Subject: `Action needed: update your post before retrying`
- Recommended fix explains what field/media must change.
- CTA goes to the failed post detail, not a reconnect page.

## Product requirements

### 1. Failure explainer

Create a backend failure explainer that converts internal failure evidence into a user-safe explanation.

Recommended package:

```text
api/internal/notifications/explainer
```

Core output:

```go
type FailureExplanation struct {
    Title             string
    Summary           string
    RecommendedAction string
    PrimaryCTA        NotificationCTA
    SecondaryCTA      *NotificationCTA
    Severity          string // info | warning | action_required | critical
    UserActionNeeded  bool
    AutoRetryStatus   string // none | scheduled | exhausted | unknown
    Confidence        string // high | medium | low
    SupportCode       string
}
```

The explainer must accept structured inputs instead of raw string-only payloads:

```go
type FailureContext struct {
    EventType          string
    PostID             string
    WorkspaceID        string
    Platform           string
    AccountName        string
    SocialAccountID    string
    CaptionPreview     string
    ErrorMessage       string
    FailureStage       string
    ErrorCode          string
    PlatformErrorCode  string
    Retriable          bool
    AnotherAttempt     bool
    Attempts           int
    MaxAttempts        int
    DebugCurl          string
    OccurredAt         time.Time
}
```

The explainer may inspect `DebugCurl`, but only to derive safe labels. The returned explanation must never include raw debug curl content.

### 2. Deterministic rules first

Implement deterministic mappings for common failure classes before any AI fallback.

Required v1 rules:

| Condition | User-facing diagnosis | Recommended action |
|---|---|---|
| Meta / Threads / Instagram / Facebook `OAuthException`, code `190`, or token expired/invalid | Account authorization expired | Reconnect the account, then retry the post |
| `auth_token_invalid` | Account authorization expired | Reconnect the account |
| `account_reconnect_required` | Account needs reconnect | Reconnect the account |
| `missing_permission` or scope-related provider text | Permission missing | Reconnect and approve requested permissions |
| `rate_limit` | Platform rate limit | Wait and retry later; if automatic retry is scheduled, no action yet |
| `temporary_platform_error` | Platform temporary issue | UniPost will retry automatically if attempts remain |
| `quota_exceeded` | Platform or plan quota reached | Review quota/billing or wait for quota reset |
| `validation_error` | Post settings need changes | Open the post, edit the highlighted issue, retry |
| `media_error` | Media could not be uploaded or processed | Check media format, size, duration, and source availability |
| `target_not_found` | Target account/post could not be found | Reconnect or choose another account |
| unknown | Delivery failed | Open the post for details or contact support |

### 3. Optional AI fallback

AI analysis should be optional and conservative.

Use AI only when:

- deterministic rules produce `Confidence=low`
- the error is terminal or support-worthy
- `OPENAI_API_KEY` and model configuration are available
- the event is safe to summarize without sending secrets

AI must receive a sanitized object:

```json
{
  "event_type": "post.failed",
  "platform": "threads",
  "failure_stage": "dispatch",
  "error_code": "platform_error",
  "safe_error_summary": "Threads returned OAuthException code 190: access token expired",
  "retriable": false,
  "another_attempt": false
}
```

The AI output must fit a strict JSON schema:

```json
{
  "title": "Threads needs to be reconnected",
  "summary": "Threads says this account authorization expired.",
  "recommended_action": "Reconnect Threads, then retry the post.",
  "confidence": "medium"
}
```

AI output must pass server-side validation:

- max lengths for every field
- no raw tokens or URLs with `access_token`
- no blamey language
- no instructions that ask for secrets
- no unsupported provider actions

If validation fails, fall back to the generic deterministic unknown message.

### 4. Notification payload enrichment

The `post.failed` notification payload should include a `failure_summary` object so renderers do not need to query multiple tables during delivery.

Example:

```json
{
  "id": "post_123",
  "status": "failed",
  "caption": "Launch post...",
  "failure_summary": {
    "platform": "threads",
    "account_name": "Natu",
    "error_code": "auth_token_invalid",
    "failure_stage": "dispatch",
    "retriable": false,
    "another_attempt": false,
    "title": "Threads needs to be reconnected",
    "summary": "Threads says this account authorization expired.",
    "recommended_action": "Reconnect Threads, then retry the post.",
    "primary_cta": {
      "label": "Reconnect Threads",
      "url": "https://dev.unipost.dev/accounts?account=..."
    },
    "secondary_cta": {
      "label": "View failed post",
      "url": "https://dev.unipost.dev/posts/..."
    }
  }
}
```

If a multi-platform post has several failed results, choose the highest-severity failure as the email headline and include a compact list of failed platforms below it.

### 5. Unified email template system

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
- `post.failed`
- `account.disconnected`
- `billing.usage_80pct`
- `billing.payment_failed`

The first migration can keep routing and event semantics unchanged while moving rendering into the shared template package.

### 6. Email visual and copy standard

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

- Prefer: `Threads needs to be reconnected`
- Avoid: `Platform delivery job failed`
- Prefer: `Reconnect Threads, then retry this post`
- Avoid: `OAuthException code 190`

### 7. Required email template structure

Every HTML email should follow this structure:

1. Hidden preheader.
2. Outer page background.
3. Content container.
4. Small product header: `UniPost`.
5. Event label.
6. Main title.
7. One-paragraph summary.
8. Recommended action block when action is needed.
9. Primary CTA.
10. Context details table/list.
11. Secondary link if useful.
12. Footer with settings and support links.

Example failure email copy:

```text
Subject: Action needed: reconnect Threads to publish your post
Preheader: Threads says this account authorization expired.

Threads needs to be reconnected

Your post could not be delivered because Threads says the account authorization expired.

Recommended fix
Reconnect Threads in UniPost, then retry this post.

[Reconnect Threads]

Post: What nobody tells you about lower middle market investing
Platform: Threads
Account: Natu
Retry status: UniPost will not retry automatically because reconnect is required.

You received this because publishing alerts are enabled for this workspace.
Manage notifications | Contact support
```

Plain text should carry the same content without layout-only wording.

### 8. Dashboard alignment

The dashboard should use the same explanation object where possible:

- failed post detail
- queue detail row
- retry modal or retry confirmation
- integration log detail panel

This avoids one message in email and a different message in the product.

### 9. Safety and privacy

Email content must never include:

- raw `debug_curl`
- access tokens
- refresh tokens
- Authorization headers
- provider request URLs containing token query params
- raw provider JSON if it includes identifiers that are not useful to the user
- stack traces
- internal-only worker IDs unless framed as a support code

Support codes are allowed if short and stable:

```text
Support code: post_failed:threads:auth_token_invalid
```

### 10. Feature flag

Because this touches notification content and optional AI calls, ship behind a backend feature flag:

```text
notifications.failure_explainer_v1
```

Defaults:

```text
development: on
production: off
fallback: off
```

Owner area: Notifications / Publishing. Production rollback is disabling the flag, which returns post failure notifications to the existing generic template.

If the implementation includes AI fallback, gate it separately:

```text
notifications.failure_explainer_ai
```

Defaults:

```text
development: off
production: off
fallback: off
```

The deterministic explainer must work without AI.

### 11. Observability

Add structured logs or counters for:

- explanation rule selected
- explanation confidence
- whether AI fallback was attempted
- whether AI fallback passed validation
- email template key rendered
- notification delivery status

Do not log full email bodies or raw debug curls.

### 12. Acceptance criteria

1. A Threads expired-token failure email says the account authorization expired and recommends reconnecting Threads before retrying.
2. A temporary platform failure email says UniPost is retrying automatically when another attempt exists.
3. A validation/media failure email points the user to edit the post before retrying.
4. All existing email event types render through one shared transactional layout.
5. Every email has HTML and text output.
6. Unit tests cover deterministic explanations for token expiry, reconnect, missing permissions, rate limit, temporary error, validation, media, and unknown failures.
7. Unit tests assert no rendered failure email includes `access_token`, `Authorization`, or raw `debug_curl`.
8. Snapshot or golden tests cover the normalized HTML and text templates.
9. `RESEND_API_KEY` unset still uses `NoopMailer`; rendering must still be testable without network.
10. Production can disable the new failure explainer with `notifications.failure_explainer_v1`.

## Rollout plan

### Phase 1: deterministic explainer

- Add the explainer package.
- Add rule-based classifications.
- Add tests for common publishing failure classes.
- Enrich `post.failed` payloads with `failure_summary`.
- Render `post.failed` email through the new shared template behind `notifications.failure_explainer_v1`.

### Phase 2: unified email templates

- Move notification test email, account disconnected, billing usage, and billing payment failure into the shared template.
- Move welcome and paid activation emails into the shared template.
- Add golden tests for HTML and text rendering.

### Phase 3: dashboard reuse

- Expose `failure_summary` in post detail and queue responses.
- Update failed post/queue UI to show the same diagnosis and recommended fix.

### Phase 4: optional AI fallback

- Add sanitized AI fallback behind `notifications.failure_explainer_ai`.
- Validate outputs strictly.
- Log rule/AI choice without sensitive payloads.

## Open questions

1. Should `Reconnect Threads` link to a filtered account settings page, or should we add a one-click reconnect route for a specific social account?
2. Should workspace admins receive all workspace failure emails, or only the workspace owner/account-level notification user as today?
3. Should `post.partial` also get a notification when one platform succeeds and another requires action?
4. Should Resend-hosted templates be adopted for operational versioning, or should UniPost keep rendering HTML in Go for deterministic deployments?
5. Should Slack and Discord notification payloads also receive the same failure explanation copy in v1?

## Implementation notes

- Keep the rendering code independent from Resend. `mail.Mailer` should still accept a fully rendered `mail.Message`.
- If React Email is adopted for authoring, render generated HTML into Go fixtures or a build artifact rather than adding a Node runtime dependency to email delivery.
- Prefer deterministic Go template rendering for v1 because current delivery runs inside the Go API worker.
- Add a small email preview command or test fixture output so design reviews do not require sending real emails.
- Keep debug data available in dashboard/admin surfaces, but summarize it for emails.
