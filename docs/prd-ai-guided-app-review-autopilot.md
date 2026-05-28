# PRD - AI-guided App Review Autopilot

**Status:** Planning
**Owner:** White-label / App Review / Review Agent
**Created:** 2026-05-28
**Primary platform:** TikTok
**Feature flag:** `app_review.autopilot_v1` for existing surfaces; add `app_review.ai_agent_v1` before implementation

---

## Problem

UniPost's first App Review Autopilot implementation is too deterministic for real TikTok review recordings. The local agent follows a fixed script of selectors and clicks. That works only when the browser state, login state, timing, window layout, and TikTok OAuth behavior exactly match the happy path.

The generated TailTales TikTok posting videos exposed the core failure modes:

- The recording began from the review publish page instead of a clean customer app login and account connection journey.
- The browser window and capture region were not reliably tied together, so other windows could cover the recorded region.
- OAuth handoff could appear in a different browser window and contaminate the recording.
- The script advanced too quickly for a human reviewer to understand the evidence.
- The agent did not understand the page; it only clicked fixed selectors.
- When the page was in the wrong state, the agent kept going and produced unusable video.

TikTok app review videos are judged by whether a reviewer can clearly see why each requested API scope is needed. A brittle selector script is not enough. The recording system needs page understanding, state verification, and safe recovery.

## Product Direction

Upgrade App Review Autopilot from a fixed script recorder into an AI-guided, evidence-gated recording agent.

The system should use three layers:

1. **Review Plan**
   Defines the required scope evidence, canonical step order, expected surfaces, and acceptance criteria.

2. **AI-guided Executor**
   Observes the current browser page and chooses the next safe action from a closed action allowlist.

3. **Evidence Gate**
   Verifies that each required step actually appeared in the recording before the agent proceeds.

The AI should not be a free-form browser operator. It should be a constrained planner inside a deterministic state machine.

## AI Provider Configuration

### Where the Anthropic key should live

The Anthropic API key must be configured on the UniPost API server only.

Use:

```text
ANTHROPIC_API_KEY=<your Claude Console key>
ANTHROPIC_MODEL=claude-sonnet-4-20250514
```

Local development:

```text
/Users/xiaoboyu/unipost/api/.env
```

Railway development API service:

```text
ANTHROPIC_API_KEY
ANTHROPIC_MODEL
```

Railway production API service:

```text
ANTHROPIC_API_KEY
ANTHROPIC_MODEL
```

Do not put this key in:

- `dashboard/.env.local`
- any `NEXT_PUBLIC_*` variable
- the browser
- the local review CLI command
- a customer workspace setting
- a committed file

### Why backend-only

The review agent will send browser observations to the UniPost API. The API will call Anthropic and return a constrained action. This keeps AI credentials centralized, rate-limited, logged, and revocable.

The local CLI should never receive the Anthropic key. Customers should not need to bring their own AI key for v1.

### Existing codebase context

UniPost currently has server-side OpenAI usage for compose assist through:

```text
OPENAI_API_KEY
OPENAI_MODEL
```

The new review agent should use an Anthropic-specific client rather than browser-side Anthropic calls. The existing AgentPost public tool stores an Anthropic key in the browser as a demo pattern; that pattern must not be reused for App Review Autopilot.

## Goals

1. Generate TikTok app review demo videos that a reviewer can understand without extra explanation.
2. Support TikTok content posting scopes:
   - `user.info.basic`
   - `video.upload`
   - `video.publish`
3. Support TikTok analytics scopes in the same architecture:
   - `user.info.profile`
   - `user.info.stats`
   - `video.list` only when the customer's TikTok app and sandbox can safely authorize it.
4. Begin the content posting demo from a clean customer app flow, not from an already-connected publish page.
5. Show TikTok OAuth scope authorization clearly in the recording.
6. Upload and preview a real MP4 before publishing.
7. Show caption, visibility, interaction controls, content disclosure, music usage confirmation, branded content policy, and final publish result.
8. Keep every generated video segment under TikTok's 50 MB upload limit.
9. Use 1080p output whenever possible.
10. Add visible section titles and step pauses so reviewers can follow the demo.
11. Detect bad recording states and stop instead of producing unusable artifacts.
12. Keep passwords, 2FA, QR scans, cookies, and tokens out of AI prompts and logs.

## Non-goals

- No fully autonomous TikTok login.
- No CAPTCHA bypass.
- No AI access to passwords, verification codes, cookies, refresh tokens, or OAuth secrets.
- No arbitrary JavaScript or shell execution from model output.
- No frontend direct calls to Anthropic.
- No customer-provided AI key in v1.
- No promise that the agent can overcome TikTok account risk controls.
- No support for all social platforms in the first AI-guided rollout.

## Target User Experience

### Dashboard setup

The user opens:

```text
White-label -> App Review Autopilot
```

The page asks for:

- customer app name, for example `TailTales`
- customer domain, for example `tiktok-review.tailtales.ai`
- selected TikTok scopes
- TikTok developer app credentials
- OAuth redirect URI attestation
- confirmation that TikTok app access has been removed from the mobile app before recording
- optional sample video override; default is UniPost-provided test video

The readiness panel checks:

- feature flag enabled
- TikTok credentials saved
- review domain configured and reachable
- review session token can be minted
- sample video is available
- selected scopes map to a known review plan
- local agent command can be generated

### Start recording

The user clicks `Start recording`.

UniPost creates a review job and shows a command:

```bash
npx --yes --package <pinned-agent-package> unipost-review-agent run \
  --token <single-use-token> \
  --session-token <review-session-token> \
  --api-url https://dev-api.unipost.dev
```

The local agent opens a controlled browser window and records only that window.

### Canonical TikTok content posting demo

The generated video should follow this order:

1. Open a clean browser window.
2. Navigate to the TailTales review domain.
3. Show TailTales login or review-session entry.
4. Complete TailTales session setup.
5. Navigate to TailTales account connection.
6. Click `Connect TikTok`.
7. Show TikTok OAuth authorization page.
8. Show requested scopes:
   - `user.info.basic`
   - `video.upload`
   - `video.publish`
9. User manually completes TikTok login, QR scan, 2FA, or consent if needed.
10. Return to TailTales.
11. Show TikTok account connected.
12. Open the publish post page.
13. Load and display TikTok creator info from TikTok's creator info response.
14. Upload the review MP4.
15. Show video preview.
16. Show the video stored in UniPost media storage or equivalent upload-ready state.
17. Enter caption.
18. Show visibility options returned by TikTok:
   - follower visibility
   - mutual follow friends
   - self only
19. Choose `SELF_ONLY` for unaudited sandbox app safety.
20. Show comment, duet, and stitch controls, including unavailable/disabled states.
21. Show content disclosure controls.
22. Show TikTok Music Usage Confirmation.
23. Open the Music Usage Confirmation link long enough to read.
24. Return to publish page.
25. Show TikTok Branded Content Policy.
26. Open the Branded Content Policy link long enough to read.
27. Return to publish page.
28. Preview final post.
29. Click publish.
30. Show publish result and TikTok publish id/status.
31. Show UniPost post record or delivery result.

### Segment expectations

Part 1: Account connection, OAuth, creator info, upload, preview, caption

Part 2: Visibility, interaction controls, disclosure, music and branded content policies

Part 3: Final preview, publish action, publish result, UniPost delivery record

Each part must be independently understandable and under 50 MB.

## Architecture

```text
Dashboard
  -> creates review kit/job
  -> shows local command and live status

Local Review Agent
  -> opens controlled browser
  -> captures browser window
  -> observes page state
  -> executes allowed actions
  -> records artifacts

UniPost API
  -> owns review plan
  -> owns review session auth
  -> owns AI orchestration
  -> calls Anthropic
  -> verifies evidence
  -> stores job events and artifacts

Anthropic
  -> receives redacted page observations
  -> returns constrained JSON actions only
```

## AI Orchestration Contract

### Request from local agent to API

```json
{
  "job_id": "rvjob_123",
  "step_key": "connect_tiktok",
  "goal": "Connect the TikTok account and show the OAuth authorization page.",
  "current_url": "https://tiktok-review.tailtales.ai/connect",
  "page_title": "TailTales",
  "visible_text": "...redacted visible text...",
  "dom_hints": [
    {
      "role": "button",
      "text": "Connect TikTok",
      "selector_hint": "[data-review-step='connect-tiktok']"
    }
  ],
  "screenshot_ref": "review-observations/rvjob_123/step_04.png",
  "allowed_actions": ["click", "type", "upload_file", "scroll", "wait", "assert", "pause_for_user", "navigate"]
}
```

### Response from API to local agent

```json
{
  "action": "click",
  "target": {
    "selector": "[data-review-step='connect-tiktok']",
    "description": "Connect TikTok button"
  },
  "reason": "The page is on the account connection surface and the TikTok connect button is visible.",
  "expected_evidence": {
    "url_contains": "tiktok.com",
    "visible_text_any": ["Authorize", "Permissions", "Continue"]
  },
  "hold_ms_after_action": 2000
}
```

### Closed action allowlist

The local agent may execute only:

- `navigate`
- `click`
- `type`
- `upload_file`
- `scroll`
- `wait`
- `assert`
- `pause_for_user`
- `open_link`
- `return_to_review_page`

The agent must reject:

- arbitrary JavaScript
- shell commands
- network requests supplied by the model
- cookie or localStorage reads requested by the model
- password or 2FA extraction
- model-generated file paths outside the approved upload file

## Evidence Gates

Every review step must define evidence before recording starts.

Examples:

### OAuth scope page

Required evidence:

- URL host includes `tiktok.com`
- visible text indicates authorization or consent
- visible requested scopes or TikTok permission language
- screenshot captured during the consent page
- timestamp marker in the video

If TikTok skips the authorization page, the job must fail with:

```text
TikTok skipped the authorization page. Remove the customer app from TikTok mobile app -> Settings and privacy -> Security and permissions -> Apps and services permissions, then record again.
```

### Creator info

Required evidence:

- TailTales review page visible
- TikTok account identity visible
- privacy options loaded from TikTok creator info
- max duration or creator capability evidence visible

### Video upload

Required evidence:

- selected MP4 visible
- upload-ready state visible
- preview player visible
- caption field visible

### Compliance

Required evidence:

- music usage confirmation link visible
- Music Usage Confirmation page opened and readable
- branded content policy link visible
- Branded Content Policy page opened and readable

### Publish

Required evidence:

- final preview visible
- publish button clicked
- result visible
- publish id/status visible

## Recording Rules

The recording system must prioritize reviewer readability over raw automation speed.

Defaults:

```text
resolution: 1920x1080 output
fps: 30
max file size: 50 MB per segment
section title hold: 2000-3000 ms
action hold: 1500-2500 ms
policy link hold: 4000-6000 ms
post-publish result hold: 4000-6000 ms
```

Browser rules:

- Use one controlled browser window for the recorded flow.
- Prefer a dedicated browser profile controlled by the local agent.
- Do not allow manual OAuth windows to cover the recorded browser.
- If manual login requires a separate real browser, do not record that browser; record the review page with an explicit pause overlay.
- Before recording starts, set and verify the actual browser window bounds.
- Capture the actual browser window bounds returned by the OS, not just the requested size.
- If the recorded region includes large black bars, stop and classify as `capture_region_invalid`.

## Local Agent Behavior

The local agent should run a state machine:

```text
prepare_browser
prepare_capture
open_customer_app
establish_review_session
connect_tiktok
oauth_consent
return_connected
open_publish
creator_info
upload_video
caption_and_preview
privacy_and_interactions
compliance_links
publish
verify_result
export_segments
upload_artifacts
complete
```

At each state:

1. Observe page.
2. Send redacted observation to API.
3. Receive constrained action.
4. Execute action.
5. Wait for reviewer-readable hold.
6. Verify evidence.
7. Add timeline marker.
8. Continue or pause/fail.

The local agent must not decide final success by itself. The API should store job evidence and determine whether the job is complete.

## Dashboard UX

The App Review Autopilot page should show:

- selected scopes
- generated review plan
- required video parts
- readiness checks
- local command
- current live step
- current screenshot preview
- AI action being attempted
- manual action required state
- failure reason
- re-record button
- artifact download links

For manual TikTok login:

```text
Waiting for TikTok login and consent.
Complete this in the opened browser window. UniPost cannot see or store your password, QR scan, or verification code.
```

If the agent is stuck:

```text
The page does not match the expected review step.
Expected: TikTok OAuth authorization page.
Current: TikTok login retry limit page.
Recommended action: wait and retry with a different TikTok account or browser profile.
```

## Data Model Additions

Add or extend:

### `review_jobs`

- `execution_mode`: `scripted` | `ai_guided`
- `current_state`
- `failure_code`
- `failure_detail`
- `ai_provider`
- `ai_model`
- `started_at`
- `completed_at`

### `review_job_events`

Add event types:

- `observation_captured`
- `ai_action_requested`
- `ai_action_selected`
- `ai_action_rejected`
- `evidence_gate_passed`
- `evidence_gate_failed`
- `manual_action_required`
- `capture_region_invalid`

### `review_job_artifacts`

Support:

- observation screenshots
- final video segments
- execution evidence JSON
- optional contact sheet PNG

## API Endpoints

### Existing endpoints to preserve

- create review kit
- create review job
- fetch job environment
- upload artifact
- complete/fail job

### New endpoints

```text
POST /v1/review/jobs/{job_id}/observe
```

Uploads current page observation metadata and optional screenshot artifact reference.

```text
POST /v1/review/jobs/{job_id}/next-action
```

Returns the next constrained action for the current state.

```text
POST /v1/review/jobs/{job_id}/evidence
```

Submits evidence for a state and receives pass/fail classification.

```text
POST /v1/review/jobs/{job_id}/manual-action
```

Marks a state as waiting for user action.

## Prompting Requirements

The system prompt must include:

- You are controlling a browser only through a closed action schema.
- Follow the provided review plan exactly.
- Do not invent product behavior.
- Do not request secrets.
- Do not continue if required evidence is missing.
- Prefer waiting or pausing over clicking unknown controls.
- Return strict JSON only.

The user prompt should include:

- current review state
- step goal
- allowed actions
- redacted visible text
- DOM hints
- previous failed attempts
- evidence gate requirements

The model output must be parsed as structured JSON and validated before execution.

## Security and Privacy

Do not send these to Anthropic:

- cookies
- localStorage
- OAuth access tokens
- refresh tokens
- client secrets
- passwords
- verification codes
- full unredacted HTML
- hidden inputs unless explicitly allowlisted

Send only:

- URL origin and path
- visible text with obvious secrets redacted
- accessibility/DOM hints for visible controls
- screenshot where sensitive regions are not expected
- review plan state

Retention:

- Store AI action logs for debugging.
- Store screenshots as review artifacts.
- Do not store raw passwords or login form contents.
- Add a redaction pass before writing observations to permanent storage.

## Feature Flag and Rollout

Use existing `app_review.autopilot_v1` for the dashboard entry point.

Add a new high-risk flag:

```text
app_review.ai_agent_v1
```

Production default:

```text
off
```

Development default:

```text
on only after the backend fallback is safe
```

Rollback:

- Turn off `app_review.ai_agent_v1`.
- Dashboard falls back to scripted/manual review kit generation.
- Existing completed artifacts remain accessible.

Third-party dependency:

- Anthropic API availability and model behavior.
- TikTok OAuth/review portal availability.

## Acceptance Criteria

### Functional

- User can create an AI-guided TikTok content posting review job.
- Local CLI opens a controlled browser and begins recording.
- Agent starts from customer app/review session entry, not directly from connected publish state.
- Agent shows or pauses for TikTok OAuth consent.
- Agent uploads the configured MP4.
- Agent shows creator info, preview, caption, visibility, interaction controls, compliance links, publish result, and UniPost delivery evidence.
- Agent exports three MP4 segments under 50 MB each.
- Dashboard shows final artifact links.

### Quality

- Output videos are 1080p unless the local display cannot support it.
- Every section title is visible for at least 2 seconds.
- Every important user action has a visible pause before and after.
- Policy pages are held long enough to read.
- No large black bars or wrong windows in the final artifacts.
- Generated evidence JSON maps every TikTok scope to one or more video timestamps.

### Safety

- Anthropic key is server-only.
- Local CLI never receives Anthropic key.
- AI output is rejected if outside the action allowlist.
- Sensitive login input is never sent to AI.
- Manual login state is clearly marked.

## Testing Plan

### Unit tests

- Anthropic client parses valid JSON.
- Anthropic client rejects non-JSON or invalid action schema.
- Observation redaction removes password fields, tokens, cookies, and secrets.
- Evidence gate passes/fails each TikTok step correctly.
- Local agent rejects unsupported actions.
- Segment export stays under configured byte limit.
- Browser bounds use actual OS-confirmed dimensions after window sizing.

### Integration tests

- Mock AI returns a connect action and the local agent executes it.
- Mock AI returns invalid action and agent fails safely.
- Mock page starts in wrong state and agent recovers or pauses.
- Manual OAuth pause resumes after review page shows connected state.
- Recording timeline includes all required markers.

### Manual acceptance

Use the previously approved TikTok videos as reference:

```text
/Users/xiaoboyu/Movies/UniPost/TikTok-demo/white-label
```

Compare:

- opening flow
- pacing
- readability
- scope evidence
- OAuth scope display
- upload and preview
- compliance policy display
- publish result

The new TailTales videos should match or exceed the approved videos' clarity.

## Implementation Phases

### Phase 1 - PRD and configuration

- Add this PRD.
- Add `ANTHROPIC_API_KEY` and `ANTHROPIC_MODEL` to API configuration docs/example.
- Add feature flag documentation for `app_review.ai_agent_v1`.

### Phase 2 - Backend AI orchestrator

- Add Anthropic client in API.
- Add action schema and validator.
- Add observation redaction.
- Add `next-action` endpoint.
- Add job event logging for AI actions.

### Phase 3 - Local agent AI mode

- Add `--ai-guided` mode.
- Add page observation capture.
- Add screenshot capture.
- Execute only validated actions.
- Add action holds and section holds.
- Validate actual capture bounds.

### Phase 4 - Evidence gates

- Define evidence requirements per TikTok review state.
- Add evidence endpoint.
- Generate timestamped evidence JSON.
- Fail fast on missing OAuth page, wrong window, black bars, or skipped required scopes.

### Phase 5 - Dashboard live UX

- Show AI-guided plan.
- Show live step and screenshots.
- Show manual action state.
- Show re-record flow.
- Show final artifact links and evidence coverage.

### Phase 6 - TailTales validation

- Configure Anthropic key in development API.
- Run TailTales recording.
- Compare against approved videos.
- Iterate until the videos are reviewer-readable.
- Push to `origin/dev` only after checks pass.

## Open Questions

1. Should the clean TailTales login step be simulated through a signed review session, or should it show a real customer login page?
2. Should manual TikTok login be recorded in the same browser window or represented by a pause overlay while the user logs in elsewhere?
3. Should UniPost generate a contact sheet automatically for each video part to make review QA faster?
4. Should we support BYOK for enterprise customers later, or keep UniPost-hosted AI only?
5. Which Anthropic model should be the production default after checking current Anthropic model guidance during implementation?
