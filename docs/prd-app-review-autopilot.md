# PRD - App Review Autopilot

**Status:** Planning
**Owner:** UniPost Product / Platform Engineering
**Target:** White-label onboarding and TikTok Content Posting API review
**Created:** 2026-05-26

---

## Problem

White-label customers want their users to connect social accounts and publish posts under the customer's own brand. For official platform access, many platforms require the customer to submit an app review package, including a demo video that shows:

- the customer's website or app domain
- the customer's brand and OAuth app identity
- the OAuth consent flow
- every requested product, permission, or scope in real product usage
- the feature that publishes, uploads media, or reads analytics through the requested API

This is painful for UniPost customers because they must understand each platform's app review rules, build a temporary demo flow, prepare test content, record the right browser screens, and explain each scope. The customer may already have a landing page and a domain, but they may not have a polished social posting UI or review-specific demo environment yet.

UniPost can turn this into a product advantage by doing more of the review preparation work for the customer: host a customer-branded review app, automate DNS setup, guide platform app configuration, run a local recording agent, and generate the submission package.

## Goals

1. Let a white-label customer prepare a platform app review demo from the UniPost dashboard.
2. Host the review demo under the customer's own review subdomain, such as `review.customer.com`.
3. Automate DNS record creation whenever the customer's DNS provider supports it.
4. Verify all prerequisites before recording: domain, DNS, TLS, landing page, brand, platform credentials, redirect URI, scopes, and demo flow readiness.
5. Let the customer start recording from the dashboard, then run one local CLI command to launch the recording agent.
6. Use the customer's own platform OAuth app credentials, never UniPost quickstart credentials, for review recordings.
7. Record a browser-window video that shows the customer's domain, OAuth flow, TikTok creator information, test post creation, publishing, and publish result.
8. Generate review artifacts: MP4 video, scope evidence, submission notes, and checklist.
9. Keep sensitive platform login actions under the customer's control. UniPost must not receive platform passwords, 2FA codes, or long-lived local browser sessions.
10. Make first-run local permission requirements explicit, especially macOS Screen Recording permission.

## Non-goals

- Supporting every platform in the first release
- Automatically submitting the app review form in TikTok Developer Portal
- Building a macOS or Windows desktop GUI app
- Asking customers for platform account passwords
- Recording the customer's full desktop
- Replacing the customer's real product UI
- Supporting apex-domain hosting in v1
- Supporting arbitrary path proxy hosting such as `customer.com/review/tiktok` in v1
- Supporting TikTok analytics scopes in the MVP recording flow
- Hiding real review requirements or creating fake demos

## Technical Feasibility Risks

The MVP is feasible, but several pieces are deeper than a normal dashboard feature. These risks must be made explicit before implementation estimates.

### 1. Browser-window recording is the largest engineering risk

The app review video should show the customer domain in the browser address bar. Playwright's built-in video recording is not enough because it captures only the page viewport.

Cross-platform browser-window capture is not solved by bundling `ffmpeg-static` alone:

- macOS: basic ffmpeg display capture can capture a display or device, not a specific browser window. Reliable per-window capture likely requires ScreenCaptureKit or a native helper. A display-crop fallback must track the browser window rectangle and can be invalidated by window movement or overlap.
- Windows: reliable per-window capture likely requires Windows Graphics Capture or a native helper.
- Linux: X11 and Wayland behave differently, and Wayland may restrict capture heavily.

Implementation must prototype macOS capture first before committing to the final cross-platform architecture. The first private beta may support macOS only if that is the fastest path to validate customer demand.

### 2. macOS Screen Recording permission breaks a pure "one command" promise

On macOS, Screen Recording permission attaches to the launching app, usually Terminal, iTerm, or the Node runtime. It cannot be granted silently from an `npx` package. A first run may require:

1. run the CLI
2. see a permission failure or preflight warning
3. open System Settings
4. grant Screen Recording permission to the terminal app
5. restart the terminal
6. run the command again

The product promise should be "one local command, with a one-time recording permission setup when required", not "one frictionless command." The agent's `doctor` checks must be folded into `run` as a mandatory preflight, and the dashboard should warn macOS users before they leave the browser.

### 3. Redirect URI configuration cannot be fully pre-verified

UniPost cannot read the customer's TikTok developer app configuration. The readiness page cannot truthfully show "redirect URI verified" before the OAuth flow runs.

The readiness gate should collect an attestation:

```text
I added this redirect URI in TikTok Developer Portal.
```

The first real verification happens during the recording-time OAuth round trip. Redirect mismatch must be a first-class failure with copy-paste remediation.

### 4. Existing dashboard components may need decoupling

The review app should reuse TikTok posting logic, but dashboard components may depend on Clerk, dashboard layout, workspace context, CSS variables, or dashboard-only assumptions. Implementation should verify component dependencies before estimating reuse.

The likely path is to extract shared review-safe logic and smaller controls from the dashboard compose UI rather than importing the full drawer into the public review shell.

### 5. TikTok review publishes may land as `SELF_ONLY`

During app review, the customer's TikTok app may be unaudited. TikTok posting can still be demonstrated, but the actual publish result may land as `SELF_ONLY` or use the adapter's self-only retry path. This is expected for review/sandbox-style flows and should not be treated as a broken publish.

The review UI should still show the privacy selector, creator information, interaction controls, disclosure controls, and publish result. The submission notes should explain that the app demonstrates Content Posting API use with review-safe visibility.

## MVP Decision

The MVP should support **TikTok Content Posting API review only**.

The first supported demo flow is:

1. Open the customer's review domain.
2. Connect TikTok through the customer's TikTok OAuth app.
3. Show the TikTok OAuth consent flow.
4. Return to the customer-branded review app.
5. Fetch and display TikTok `creator_info`.
6. Show available privacy options, interaction settings, disclosure controls, and max video duration.
7. Create a test TikTok post using a prepared video.
8. Publish through UniPost's TikTok adapter.
9. Show processing or published result.
10. Generate an evidence package for TikTok review.

This release should not attempt TikTok analytics review. The architecture should leave room for a v1.1 analytics review flow that reuses the TikTok analytics components and live API endpoints.

## Product Assumptions

- The customer has a landing page and domain.
- The customer is willing to delegate a review subdomain to UniPost through DNS.
- The customer can create a TikTok developer app and provide its client key and client secret to UniPost.
- The customer can log in to TikTok locally during recording.
- The customer is comfortable running one local CLI command.
- The review video must show the browser address bar so reviewers can see the customer's domain.
- The review app must use customer branding and customer-owned platform credentials.

## User Stories

1. As a white-label customer, I can enter my landing page and domain so UniPost can prepare a branded review environment.
2. As a white-label customer, I can connect my DNS provider and let UniPost automatically add the required DNS records.
3. As a white-label customer, I can see exactly which prerequisites are incomplete before recording.
4. As a white-label customer, I can upload TikTok client credentials and get clear redirect URI instructions.
5. As a white-label customer, I can click "Start recording" only after the readiness checks pass.
6. As a white-label customer, I can run one CLI command locally to start the review agent.
7. As a white-label customer, I can complete TikTok login and 2FA myself while the agent waits.
8. As a white-label customer, I can preview the final demo video before using it in a platform submission.
9. As UniPost support, I can inspect non-sensitive execution logs when a recording fails.

## App Review Autopilot Page

Add a dashboard page under white-label setup:

```text
White-label > App Review Autopilot
```

For the MVP, the page can route directly into the TikTok setup:

```text
White-label > App Review Autopilot > TikTok
```

The page is a readiness gate. It should not feel like a documentation page. It should show the customer:

- what is ready
- what UniPost can fix automatically
- what the customer must authorize
- why "Start recording" is disabled

### Sections

#### 1. Brand and website

Inputs:

- landing page URL
- legal company or product name
- support email
- privacy policy URL
- terms URL

Automated checks:

- landing page is reachable
- domain matches the requested review domain's base domain
- page title and brand name can be detected
- logo can be detected or uploaded
- privacy policy URL is reachable
- terms URL is reachable
- support email is present

Output:

- resolved brand name
- resolved logo
- resolved primary color if available
- preview of the review app header

#### 2. Review domain

Inputs:

- root domain, such as `customer.com`
- desired review subdomain, defaulting to `review.customer.com`

UniPost should prefer subdomain hosting:

```text
review.customer.com CNAME review.unipost.dev
```

UniPost should also require a verification TXT record:

```text
_unipost-review.customer.com TXT unipost-review=rv_xxx
```

The TXT record proves customer control. The CNAME delegates the review app host to UniPost.

Automated checks:

- root domain syntax is valid
- subdomain is not already bound to another workspace
- DNS provider can be detected from nameservers or provider APIs
- CNAME resolves to UniPost
- TXT verification token resolves
- TLS certificate is issued
- `https://review.customer.com` loads the UniPost-hosted review app

DNS and TLS readiness should be asynchronous. Verification and certificate issuance can take minutes or longer, especially when DNS propagation, CAA records, or managed certificate issuance slow the path. The customer should be able to close the page and return later. UniPost should persist the readiness state and send an in-app or email notification when the review domain becomes ready.

#### 3. DNS automatic setup

The primary experience should be automatic.

Flow:

1. Customer enters `customer.com`.
2. UniPost proposes `review.customer.com`.
3. UniPost detects the DNS provider.
4. Page shows:

```text
DNS provider detected: Cloudflare
[Connect DNS automatically]
```

5. Customer clicks the button.
6. Customer is sent through an authorized DNS setup flow.
7. The provider asks the customer to confirm adding the CNAME and TXT records.
8. Customer confirms.
9. Customer returns to UniPost.
10. UniPost starts background DNS and TLS readiness checks.
11. UniPost notifies the customer when the domain is ready, instead of requiring the customer to stare at a spinner.

Recommended implementation strategy:

- First choice: integrate a managed DNS automation provider such as Entri Connect.
- Second choice: support Domain Connect where the provider supports it.
- Fallback: manual DNS instructions.

Manual fallback must be shown only when automation is unavailable or fails.

Manual fallback copy should be concise:

```text
Add these records at your DNS provider:

Type: CNAME
Name: review
Value: review.unipost.dev

Type: TXT
Name: _unipost-review
Value: unipost-review=rv_xxx
```

The page should explain that DNS automation never asks UniPost to know or store the customer's DNS password. The customer authorizes the DNS change with their provider.

#### 4. TikTok app credentials

Inputs:

- TikTok client key
- TikTok client secret
- selected TikTok products and scopes

Checks:

- client key is present
- client secret is present and stored encrypted
- TikTok platform credential row exists for the workspace
- customer has attested that the OAuth redirect URI was copied into the TikTok developer app
- selected MVP scopes include `user.info.basic`, `video.publish`, and `video.upload`
- review flow does not use UniPost quickstart credentials

The MVP should use the existing API-domain OAuth callback path because UniPost already serves token exchange there. The review app should pass a customer-domain return URL so the browser returns to the customer review domain after the callback completes.

The page should show the exact MVP redirect URI to copy:

```text
https://api.unipost.dev/v1/connect/callback/tiktok
```

The review flow should redirect back to:

```text
https://review.customer.com/tiktok/posting
```

Future versions may support a custom-domain callback such as:

```text
https://review.customer.com/oauth/callback/tiktok
```

That future path requires either custom-domain reverse proxying to the existing callback handler or first-class callback handling under the review domain. It is not required for the MVP.

The readiness page cannot verify the TikTok developer app settings directly. If the redirect URI is missing or wrong, the OAuth flow should fail during recording with an error that shows the exact URI to add and a "Copy redirect URI" action.

#### 5. Demo flow readiness

Checks:

- review app can create a TikTok connect session using customer credentials
- test media asset is available
- TikTok posting UI can show privacy, interaction, and disclosure controls
- post publish endpoint is available
- publishing status view is available
- scope evidence mapping exists for each requested scope

The page should list the planned recording steps before starting:

1. Open review domain
2. Connect TikTok
3. Show OAuth consent
4. Fetch creator information
5. Choose privacy and interaction settings
6. Upload/select test video
7. Publish
8. Show result

#### 6. Recording agent

Checks:

- review job can be created
- one-time agent token can be minted
- CLI command can be generated
- upload URL can be generated

For macOS customers, the page should show a preflight warning before displaying the CLI command:

```text
First run on macOS may ask you to enable Screen Recording for your terminal app. If permission is denied, enable it in System Settings, restart your terminal, then run the command again.
```

When all prior sections pass, enable:

```text
[Start recording]
```

## Start Recording Flow

When the customer clicks "Start recording":

1. UniPost creates a `review_job`.
2. UniPost creates a short-lived one-time agent token.
3. UniPost mints a signed review-session token for the review app.
4. UniPost displays a version-pinned command:

```bash
npx --yes @unipost/review-agent@0.1.0 run --token revtok_xxx
```

The dashboard may also offer "use latest agent" for support/debugging, but the default command must pin the agent version tested with the generated script. A bad `@latest` publish should not immediately affect every customer.

5. Customer runs the command locally.
6. Agent connects to UniPost and claims the job.
7. Dashboard updates in real time:

```text
Agent connected
Checking local environment
Opening https://review.customer.com/tiktok/posting
Recording browser window
Waiting for TikTok login
Publishing test video
Uploading demo video
Complete
```

The dashboard is the source of truth for the customer. Terminal output can mirror progress, but important instructions must appear in the dashboard and in the controlled browser window.

The CLI is the primary MVP execution path. A desktop app is explicitly out of scope.

## UX Flow Requirements

The journey crosses multiple surfaces: dashboard, DNS provider, TikTok Developer Portal, terminal, the automated browser, and on macOS, System Settings. The product must actively reduce attention switching.

### Dashboard as command center

The dashboard should always tell the customer where to look next:

- "Return to your DNS provider and confirm the records."
- "Open your terminal and run this command."
- "Switch to the browser window the agent opened."
- "Complete TikTok login in the browser."
- "Return here to preview your video."

The customer should not need to infer status from terminal logs.

### Manual-login pause overlay

When the agent reaches TikTok login, 2FA, captcha, or OAuth consent, it must inject or display a prominent pause overlay in the controlled browser window:

```text
Action needed: log in to TikTok

Complete TikTok login and approve access in this browser window.
UniPost cannot see or store your password or verification code.
The recording will continue automatically after TikTok sends you back.
```

The dashboard should show the same state:

```text
Waiting for TikTok login
```

MVP decision: keep the overlay in the recorded MP4. It helps reviewers understand why the flow pauses and makes the customer's control over login explicit. Future video post-processing can trim or chapter this pause if needed.

### Re-record loop

Customers will often need to re-record because of login mistakes, unexpected provider screens, or a distracting pause. Re-recording should not redo setup.

Flow:

1. Customer previews the video.
2. Customer clicks "Record again".
3. UniPost creates a new `review_job` under the same `review_kit`.
4. Existing domain, brand, TikTok credentials, and scope mapping remain unchanged.
5. UniPost mints a fresh one-time agent token and displays the new command.

Failed jobs should show a "Try again" action that generates the correct run or resume command. The customer should not need to remember a job ID or choose between `run` and `resume`.

## Local Review Agent

The agent is a local CLI package:

```text
@unipost/review-agent
```

Supported commands:

```bash
unipost-review-agent run --token revtok_xxx
unipost-review-agent doctor
unipost-review-agent resume --job rev_xxx
```

The `npx` command should be the default path shown in the dashboard. The installed binary commands are for repeat runs and support debugging.

The `run` command must always execute `doctor` preflight checks before starting a recording. `doctor` as a standalone command is only for support and troubleshooting.

### Agent responsibilities

The agent must:

- validate the one-time token
- fetch the review script
- launch a headed browser
- use a clean temporary browser profile
- set a stable browser window size
- open the customer review domain
- add the signed review-session token to the browser context without exposing it in the recorded address bar
- record the browser window so the address bar is visible
- automate review app interactions using stable selectors
- pause when TikTok login, 2FA, captcha, or consent requires user input
- show a visible pause overlay during user-controlled login steps
- resume after the user completes the external authorization step
- publish the test TikTok post through the review app
- capture screenshots at milestone steps
- upload the final MP4 and execution metadata to UniPost
- clean up temporary browser profile data after completion

The agent must not:

- ask the customer for platform passwords
- upload platform cookies to UniPost
- record the full desktop by default
- keep a persistent browser profile unless the user explicitly asks for a debug run
- run arbitrary scripts from the review job
- access workspace data unrelated to the current review job

### Agent implementation recommendation

MVP technology:

- Node.js + TypeScript
- Playwright for browser automation
- native capture helper prototype for browser-window recording, starting on macOS
- ffmpeg only as a display-crop fallback where reliable enough for private beta
- WebSocket or SSE for progress updates to the dashboard
- signed upload URLs for MP4 and evidence artifacts
- bundled CLI build with `ncc`, `tsup`, or an equivalent packager

Important recording requirement:

- Playwright page video alone is not sufficient because it records only the page viewport, not the browser address bar.
- App review videos should show the customer domain in the browser chrome.
- The agent should record the browser window or a tightly scoped screen region containing the browser window.
- The phase 0 prototype must prove the recording path on macOS before broader rollout.

## Review App

The review app is hosted by UniPost but appears under the customer's review domain.

Example:

```text
https://review.customer.com/tiktok/posting
```

It must:

- use customer branding
- show the customer's domain in the address bar
- use customer platform credentials
- avoid the UniPost dashboard shell
- avoid showing "Quickstart" branding
- avoid using UniPost quickstart credentials
- show only the steps needed for app review
- expose stable `data-review-step` selectors for the local agent
- show a visible evidence panel or step labels that explain which scope is demonstrated

### Review app session auth

The review app is public on the internet, but the recording flow must not be publicly usable by anyone who guesses the URL. The app needs a signed, short-lived review-session token.

Recommended behavior:

- `review_jobs` mint a derived review-session JWT or opaque session token.
- The local agent receives that token as part of the review script.
- The agent injects it into the browser context as an HttpOnly cookie or equivalent first-party session bootstrap before opening the review page.
- The token is scoped to one `review_job`, one `review_kit`, one workspace, one platform, and one review domain.
- The token cannot access normal dashboard APIs or arbitrary workspace data.
- The token expires quickly and is invalidated when the job completes or fails.

If someone opens `https://review.customer.com/tiktok/posting` without an active signed session, the app should show a safe "No active review session" state with no credentials, no account data, and no publish controls.

Example selectors:

```html
<button data-review-step="connect-tiktok">Connect TikTok</button>
<section data-review-step="creator-info">...</section>
<button data-review-step="publish-tiktok">Publish test video</button>
<section data-review-step="publish-result">...</section>
```

## Reuse of Existing UniPost Surfaces

The review app should reuse underlying UniPost capabilities but not directly reuse the normal dashboard pages.

### TikTok posting

Reuse:

- TikTok platform adapter
- media upload and publish API
- TikTok `creator_info` endpoint
- TikTok privacy, interaction, disclosure, and max-duration logic from the dashboard compose components

Do not reuse as-is:

- normal dashboard shell
- general Create Post drawer layout
- Quickstart tutorial page
- quickstart credentials

The review flow must render a narrower review-specific UI around the same posting logic.

The implementation should expect a small refactor:

- move TikTok creator-info loading, privacy option mapping, disclosure validation, and duration checks into review-safe shared modules where possible
- keep dashboard-specific layout, Clerk assumptions, and drawer state outside the review app
- render review-specific controls that are optimized for the script and the recorded video

### TikTok analytics

Out of scope for MVP.

Future v1.1 can reuse:

- TikTok analytics API endpoints
- profile, account metrics, videos, and post analytics components

But v1.1 must still render those capabilities inside a customer-domain, review-specific shell rather than the normal UniPost dashboard route.

## Review Artifacts

Each completed review job should produce:

- `demo-video.mp4`
- milestone screenshots
- execution log with sensitive values redacted
- scope evidence JSON
- `submission-notes.md`
- `review-checklist.md`

Example scope evidence:

```json
{
  "platform": "tiktok",
  "recording_started_at": "2026-05-26T20:30:00Z",
  "scopes": [
    {
      "scope": "user.info.basic",
      "demonstrated_by": "creator_info account identity panel",
      "video_marker": "00:42",
      "event_elapsed_ms": 42000
    },
    {
      "scope": "video.publish",
      "demonstrated_by": "Publish test video action",
      "video_marker": "01:35",
      "event_elapsed_ms": 95000
    },
    {
      "scope": "video.upload",
      "demonstrated_by": "Test video upload and publish flow",
      "video_marker": "01:14",
      "event_elapsed_ms": 74000
    }
  ]
}
```

Video markers require the recorder and step runner to share a clock. The agent should emit `recording_started_at` and per-step elapsed milliseconds so UniPost can correlate script events to MP4 timestamps.

## Data Model

Recommended new tables or equivalent persisted models:

### `review_domains`

Tracks customer review domain delegation.

Fields:

- `id`
- `workspace_id`
- `domain`
- `provider`
- `status`
- `verification_token`
- `cname_target`
- `dns_verified_at`
- `tls_status`
- `tls_issued_at`
- `created_at`
- `updated_at`

### `review_kits`

Tracks a platform review configuration.

Fields:

- `id`
- `workspace_id`
- `platform`
- `use_case`
- `review_domain_id`
- `brand_snapshot`
- `required_scopes`
- `status`
- `created_at`
- `updated_at`

### `review_jobs`

Tracks one recording run.

Fields:

- `id`
- `review_kit_id`
- `workspace_id`
- `platform`
- `status`
- `started_at`
- `completed_at`
- `failed_at`
- `failure_reason`
- `agent_version`
- `review_session_token_id`
- `video_file_id`
- `artifacts_json`
- `created_at`
- `updated_at`

### `review_job_events`

Tracks progress and support-debuggable events.

Fields:

- `id`
- `review_job_id`
- `event_type`
- `message`
- `metadata`
- `elapsed_ms`
- `created_at`

### `review_sessions`

Tracks a short-lived session that allows the review app to load during a specific recording job.

Fields:

- `id`
- `review_job_id`
- `review_kit_id`
- `workspace_id`
- `platform`
- `review_domain`
- `token_hash`
- `expires_at`
- `claimed_at`
- `revoked_at`
- `created_at`

## API Surface

Recommended internal API surface:

```http
POST /v1/review/domains
GET /v1/review/domains/{id}
POST /v1/review/domains/{id}/dns-automation
POST /v1/review/domains/{id}/verify

POST /v1/review/kits
GET /v1/review/kits/{id}
POST /v1/review/kits/{id}/readiness

POST /v1/review/jobs
GET /v1/review/jobs/{id}
GET /v1/review/jobs/{id}/script
POST /v1/review/jobs/{id}/events
POST /v1/review/jobs/{id}/complete
POST /v1/review/jobs/{id}/fail
```

Agent tokens must be scoped to:

- one `review_job_id`
- one workspace
- one platform
- read-only script fetch
- event append
- artifact upload
- job completion/failure

They must not permit arbitrary workspace API actions.

Review-session tokens are separate from agent tokens. The agent token lets the CLI fetch scripts, append events, and upload artifacts. The review-session token lets the browser load the public review app for the active job.

## Review Script Contract

The agent should not infer the whole flow from natural language. UniPost should generate a structured review script.

The script action set must be a closed enum. The agent must reject any unknown action.

Allowed MVP actions:

- `goto`
- `click`
- `fill`
- `assert_visible`
- `assert_url_contains`
- `manual_pause`
- `wait_for_navigation`
- `wait_for_network_idle`
- `screenshot`
- `emit_marker`

No arbitrary JavaScript execution is allowed in MVP review scripts.

Example:

```json
{
  "job_id": "revjob_123",
  "platform": "tiktok",
  "agent_version": "0.1.0",
  "start_url": "https://review.customer.com/tiktok/posting",
  "review_session": {
    "delivery": "cookie",
    "cookie_name": "__unipost_review_session",
    "expires_at": "2026-05-26T21:00:00Z"
  },
  "recording": {
    "window_width": 1440,
    "window_height": 1000,
    "show_address_bar": true
  },
  "steps": [
    {
      "id": "open_review_app",
      "action": "goto",
      "url": "https://review.customer.com/tiktok/posting",
      "marker": "Open customer review domain"
    },
    {
      "id": "connect_tiktok",
      "action": "click",
      "selector": "[data-review-step='connect-tiktok']",
      "marker": "Start TikTok OAuth"
    },
    {
      "id": "wait_for_oauth",
      "action": "manual_pause",
      "resume_when_url_contains": "/tiktok/posting",
      "overlay": "Log in to TikTok and approve access. UniPost cannot see or store your password or verification code.",
      "marker": "Customer completes TikTok login and consent"
    },
    {
      "id": "assert_creator_info",
      "action": "assert_visible",
      "selector": "[data-review-step='creator-info']",
      "marker": "Show TikTok creator_info"
    },
    {
      "id": "publish",
      "action": "click",
      "selector": "[data-review-step='publish-tiktok']",
      "marker": "Publish test video"
    },
    {
      "id": "assert_result",
      "action": "assert_visible",
      "selector": "[data-review-step='publish-result']",
      "marker": "Show publish result"
    }
  ]
}
```

## Security and Privacy

Security requirements:

- Review agent tokens expire quickly, recommended TTL 30 minutes.
- Tokens are single-use after a job is claimed.
- Tokens are scoped only to the current review job.
- Review-session tokens are separate from agent tokens and expire/revoke with the job.
- Platform credentials remain server-side and encrypted at rest.
- Platform passwords, 2FA codes, cookies, and browser storage are not uploaded.
- The agent records only the controlled browser window or explicit capture region.
- The customer can preview the video before using it.
- Review artifacts have configurable retention and deletion controls.
- Logs redact OAuth codes, tokens, secrets, cookies, authorization headers, and platform identifiers where appropriate.

Privacy UX requirements:

- Before the CLI starts recording, it must clearly say what will be recorded.
- During manual login pauses, it must say UniPost cannot see or store the customer's password.
- The dashboard should show artifact retention and deletion controls.

## Error Handling

The readiness page should block recording when:

- DNS is not verified
- TLS is not issued
- review domain is unreachable
- TikTok credentials are missing
- redirect URI attestation is missing
- required scopes are missing from the planned review kit
- test media is unavailable
- review app health check fails

The agent should fail with actionable errors when:

- token expired
- job already claimed
- browser cannot launch
- recording permission is unavailable
- review domain cannot load
- review-session token is missing, expired, or rejected
- OAuth never returns to the review app
- TikTok rejects the OAuth flow because the redirect URI was not added correctly
- TikTok creator_info fails
- publish action fails
- upload of MP4 fails

Every failure should be visible in the dashboard with a suggested next action. Redirect URI failures should show the exact URI to add in TikTok Developer Portal and a "Copy redirect URI" action.

Failed jobs should expose recovery actions in the dashboard:

- "Retry from beginning" for clean re-records
- "Resume recording" only when the agent can safely continue from a known checkpoint
- "Copy run command" or "Copy resume command" without requiring the customer to remember a job ID

## Success Metrics

Product success:

- percentage of customers who reach "Ready to record"
- percentage of recording jobs completed successfully
- median time from creating review kit to completed MP4
- number of manual DNS setups avoided through automation
- customer support tickets per review kit
- customer-reported app review approval rate

Technical success:

- DNS verification latency
- TLS issuance latency
- agent install/start success rate
- agent preflight success rate
- macOS recording permission completion rate
- recording upload success rate
- publish demo success rate
- percentage of failures with actionable error classification

## Rollout Plan

### Phase 0 - Internal prototype

- Hardcode one UniPost-controlled test domain.
- Generate a TikTok review script.
- Run local CLI agent manually on macOS first.
- Prove browser-window recording with the address bar visible.
- Document the first-run Screen Recording permission flow.
- Produce a demo MP4 for UniPost's own TikTok app review flow.

### Phase 1 - Private beta

- Support one customer review subdomain per workspace.
- Support manual DNS setup and verification.
- Support TikTok Content Posting API review flow.
- Support local CLI recording on the proven OS target from phase 0.
- Use the existing API-domain OAuth callback and redirect back to the customer review domain.
- Generate MP4 and submission notes.

### Phase 2 - DNS automation

- Add managed DNS automation, preferably through Entri Connect or a similar provider.
- Keep manual fallback.
- Add DNS provider detection and better verification states.

### Phase 3 - Broader review kits

- Add LinkedIn posting review.
- Add YouTube upload review.
- Add TikTok analytics review.
- Add Meta review flow only after the simpler platforms are stable.

## Acceptance Criteria

MVP is complete when:

1. A customer can configure `review.customer.com` for a workspace.
2. UniPost verifies CNAME, TXT ownership, TLS, and review app reachability.
3. A customer can upload TikTok app credentials.
4. UniPost can create a TikTok review kit that uses customer credentials.
5. "Start recording" stays disabled until all required readiness checks pass.
6. The dashboard treats redirect URI setup as customer attestation and handles mismatch as a recording-time failure.
7. The dashboard shows a version-pinned `npx` command for the local review agent.
8. The agent preflight explains macOS Screen Recording permission when required.
9. The local review agent opens a headed browser at the customer review domain.
10. The review app requires a signed active review-session token and shows a safe inactive state without one.
11. The recorded video includes the browser address bar with the customer domain.
12. The agent pauses for user-controlled TikTok login and OAuth consent.
13. The pause state is visible in the browser and dashboard.
14. The review flow fetches and displays TikTok creator information.
15. The review flow publishes a prepared TikTok test video through UniPost, with review-safe `SELF_ONLY` behavior treated as expected.
16. The agent uploads a completed MP4 and execution evidence with elapsed-time markers.
17. The dashboard shows the final video and downloadable review artifacts.
18. Re-recording creates a fresh job/token without redoing domain, brand, or credential setup.
19. No platform password, 2FA code, or cookie is uploaded to UniPost.

## Open Questions

1. Which DNS automation provider should UniPost choose first: Entri Connect, Domain Connect, or direct integrations with top DNS providers?
2. What artifact retention period should apply by default: 30 days, 90 days, or until customer deletion?
3. Should UniPost support user-uploaded test videos in MVP, or use only a UniPost-provided neutral test video?
4. Should the generated video include automated captions in MVP, or should captions be a v1.1 enhancement?
5. Which OS should private beta officially support first after the macOS recording prototype is complete?
