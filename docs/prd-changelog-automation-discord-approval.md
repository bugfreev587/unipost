# Changelog Automation With Discord Approval PRD
Status: Planning
Owner: Developer Experience / Release Engineering
Created: 2026-06-18
Updated: 2026-06-18

---

## 1. Background

UniPost now has a public `/changelog` page that works best when it stays sparse, factual, and verified. Manually maintaining that page is reliable but easy to forget after a busy release day. The goal of this automation is to turn yesterday's shipped work into a daily review prompt, while keeping final publishing under human approval.

The user wants a daily morning process that:

1. Reads yesterday's commits and release activity.
2. Uses AI to summarize changelog-worthy contributions.
3. Sends a Discord message with links and three actions:
   - Publish
   - Save for later
   - Discard
4. If the user clicks Publish, automatically runs the full release chain to production.

This is intentionally not a fully autonomous changelog writer. AI drafts; the human approves; automation executes a guarded release pipeline.

---

## 2. Product Goals

1. Reduce manual changelog maintenance by producing a daily AI-generated candidate.
2. Preserve the public changelog's trust value by requiring human approval before publication.
3. Make approval lightweight inside Discord.
4. After approval, automatically publish through the normal chain:
   - `dev`
   - `staging`
   - `main`
   - production verification
5. Stop safely if any check, deploy, or verification step fails.
6. Report the final result back to Discord with useful links.

---

## 3. Non-goals

- Do not let AI directly edit production without a human click.
- Do not publish every commit.
- Do not turn `/changelog` into a daily activity feed.
- Do not publish unverified SDK versions.
- Do not bypass the existing `dev -> staging -> main` release flow.
- Do not require a Discord bot for v1 if signed web links are enough.
- Do not build a CMS in v1.
- Do not expose GitHub, Discord, or AI provider secrets to the browser.

---

## 4. Success Criteria

1. A scheduled workflow runs every morning in `America/Los_Angeles`.
2. If yesterday has no changelog-worthy shipped work, Discord receives a clear "no candidate" message or no message, depending on configuration.
3. If a candidate exists, Discord receives a message containing:
   - candidate title
   - candidate summary
   - category and impact
   - source links
   - confidence
   - Publish / Save for later / Discard links
4. Clicking Publish creates a changelog update, validates it, merges it through `dev`, `staging`, and `main`, and verifies production.
5. Clicking Save for later stores the candidate without publishing it.
6. Clicking Discard suppresses the same candidate from future daily prompts.
7. Failures stop the release and send a Discord failure report with the failed stage and relevant logs/PR/deployment links.

---

## 5. User Experience

### 5.1 Daily Discord Candidate

Example Discord message:

```text
UniPost changelog candidate for 2026-06-17

Candidate: Multi-language SDKs 0.4.1
Area: SDK
Impact: Improved
Confidence: High

Summary:
JavaScript, Python, Go, and Java SDKs reached 0.4.1 with coverage for Developer Logs and updated analytics surfaces.

Verified sources:
- PR #72
- npm @unipost/sdk 0.4.1
- PyPI unipost 0.4.1
- Go tag v0.4.1
- Maven dev.unipost:sdk-java:0.4.1

Actions:
Publish | Save for later | Discard
```

Discord incoming webhooks can post the message. The action controls should be link buttons or plain links. The links point back to a UniPost-owned signed action endpoint. Webhooks are not responsible for processing the click.

### 5.2 Publish Result Message

On success:

```text
Published changelog update to production.

Entry: Multi-language SDKs 0.4.1
Production: https://unipost.dev/changelog#sdk-0-4-1
Dev PR: ...
Staging PR: ...
Production PR: ...
Commit: ...
```

On failure:

```text
Changelog publish stopped.

Stage: production verification
Reason: /changelog did not contain the expected release id.
Run: ...
PR: ...
Next step: inspect production deployment logs and rerun after fixing.
```

---

## 6. Recommended Architecture

### 6.1 Components

| Component | Responsibility |
| --- | --- |
| Daily GitHub Actions workflow | Triggers the morning digest and runs collector/summarizer scripts. |
| Commit/PR collector | Finds yesterday's relevant merged PRs and commits. |
| AI summarizer | Converts candidate activity into strict JSON. |
| Candidate validator | Enforces changelog rules and verifies SDK registry versions. |
| Candidate store | Saves pending/saved/discarded/published candidate state. |
| Discord notifier | Sends the review message and final status messages. |
| Signed action endpoint | Receives Publish / Save for later / Discard clicks. |
| Release orchestrator | Runs guarded `dev -> staging -> main` publishing. |
| Verification runner | Opens dev, staging, and production URLs and checks expected content. |

### 6.2 Recommended Shape

Use GitHub Actions for the daily summarizer and release orchestration, with a small UniPost backend endpoint for signed action handling.

Why:

- GitHub Actions already has repo permissions, branch operations, CI context, and scheduled workflow support.
- The backend endpoint is better for click handling because Discord links need a stable HTTPS destination.
- The release chain can be audited through PRs and workflow runs.

---

## 7. Daily Candidate Generation

### 7.1 Schedule

Run once per morning:

```yaml
on:
  schedule:
    - cron: "0 8 * * *"
      timezone: "America/Los_Angeles"
  workflow_dispatch:
```

The workflow should live on the default branch and should support manual dispatch for backfills and testing.

### 7.2 Source Of Truth

For public changelog candidates, the default source should be `main`, not `dev`.

Reason:

- `/changelog` is public and production-facing.
- Public changelog entries should describe shipped work.
- `dev` may contain work that is not yet intended for production.

Optional future setting:

- `mode=dev-preview` can summarize `dev` into an internal-only Discord digest, but it should not create a public changelog entry until the work reaches production.

### 7.3 Collection Window

The daily workflow should evaluate the previous local calendar day in `America/Los_Angeles`.

Example:

- Run time: `2026-06-18 08:00 America/Los_Angeles`
- Collection window: `2026-06-17 00:00:00` through `2026-06-17 23:59:59 America/Los_Angeles`

The collector should persist the exact window in the candidate payload.

### 7.4 Inputs To AI

The AI summarizer should receive:

- merged PR titles and bodies
- commit messages
- changed file paths
- diff stat
- source links
- existing `changelogReleases`
- SDK registry versions when relevant
- exclusion policy from this PRD

The AI should not receive secrets, raw environment variables, API keys, or private customer data.

### 7.5 AI Output Contract

The summarizer must output JSON only:

```json
{
  "hasCandidate": true,
  "candidate": {
    "id": "sdk-0-4-1",
    "date": "2026-06-17",
    "displayDate": "June 17, 2026",
    "title": "Multi-language SDKs 0.4.1",
    "summary": "Official JavaScript, Python, Go, and Java SDKs reached 0.4.1 with updated Developer Logs coverage.",
    "category": "sdk",
    "impact": "improved",
    "isBreaking": false,
    "sdkVersions": [],
    "links": [],
    "sourceLinks": [],
    "confidence": "high",
    "whyUserVisible": "Customers can upgrade SDKs and access newly documented Developer Logs helpers.",
    "excludedCommits": []
  }
}
```

If no candidate is appropriate:

```json
{
  "hasCandidate": false,
  "reason": "Only internal refactors and CI maintenance changed yesterday.",
  "excludedCommits": []
}
```

---

## 8. Candidate Validation Rules

The validator must reject a candidate if any of these are true:

1. It has no `sourceLinks`.
2. It duplicates an existing release id.
3. Its title or summary contains unsupported future-looking claims.
4. It summarizes internal-only refactors, dependency bumps, CI changes, or copy tweaks as product releases.
5. It includes an SDK version without registry verification.
6. It includes `@unipost/sdk-js` as a package name. The JavaScript npm package is `@unipost/sdk`.
7. It includes AgentPost as an SDK release.
8. It lacks a user-visible rationale.
9. It has low confidence.

The validator should allow a "no candidate" result and should treat that as success.

### 8.1 SDK Verification

For SDK entries, verify each ecosystem before allowing Publish:

| Ecosystem | Package | Verification source |
| --- | --- | --- |
| npm | `@unipost/sdk` | npm registry |
| pip | `unipost` | PyPI JSON or release page |
| Go | `github.com/unipost-dev/sdk-go` | Go module proxy or Git tag |
| Maven | `dev.unipost:sdk-java` | Maven Central metadata or artifact path |

The generated `sdkVersions` objects must include:

- `ecosystem`
- `packageName`
- `version`
- `href`
- `installCommand`

---

## 9. Candidate Store

V1 can store candidates as GitHub issues, GitHub artifacts, or a small backend table.

Recommended v1: backend table.

Suggested table shape:

| Field | Purpose |
| --- | --- |
| `id` | Candidate id / idempotency key |
| `window_start` | Collection window start |
| `window_end` | Collection window end |
| `status` | `pending`, `saved`, `discarded`, `publishing`, `published`, `failed` |
| `payload_json` | Candidate JSON |
| `source_hash` | Hash of commit SHAs and PR ids |
| `discord_message_id` | Optional Discord message reference |
| `created_at` | Audit timestamp |
| `updated_at` | Audit timestamp |
| `acted_at` | Human action timestamp |
| `acted_by_hint` | Optional audit label if known |

The `source_hash` prevents repeated prompts for the same work.

---

## 10. Discord Actions

### 10.1 Signed URLs

Action URLs should look like:

```text
https://api.unipost.dev/internal/changelog-actions?action=publish&candidate_id=...&expires=...&signature=...
```

The signature should cover:

- candidate id
- action
- expiration timestamp
- source hash

Links must be single-use. Expired, reused, or invalid signatures should return a safe HTML response and send no release action.

### 10.2 Actions

| Action | Behavior |
| --- | --- |
| Publish | Start release orchestration. |
| Save for later | Mark candidate as saved and optionally include it in the next daily digest. |
| Discard | Mark candidate as discarded and suppress repeats for the same source hash. |

### 10.3 User Feedback

The HTTP response should be a simple confirmation page:

- "Publish started"
- "Saved for later"
- "Discarded"
- "Link expired"
- "Already handled"

The Discord message should also be updated or followed up where possible.

---

## 11. Publish Orchestration

### 11.1 High-level Flow

When Publish is clicked:

1. Mark candidate `publishing`.
2. Create branch `dev-changelog-auto-YYYY-MM-DD`.
3. Update `dashboard/src/app/changelog/releases.ts`.
4. Run local/source validation in the workflow.
5. Merge or PR into `dev`.
6. Wait for `origin/dev` checks and deployments.
7. Verify dev environment.
8. Promote `dev -> staging`.
9. Wait for staging checks and deployments.
10. Verify staging environment.
11. Promote `staging -> main`.
12. Wait for production checks and deployments.
13. Verify production environment.
14. Mark candidate `published`.
15. Send Discord success report.

### 11.2 Branch And PR Policy

Even though the user wants one-click production release, the automation should still preserve PR audit records.

Recommended:

- Create a PR into `dev` for the changelog commit.
- Auto-merge it only after required checks pass.
- Create a PR `dev -> staging`.
- Auto-merge it only after required checks pass and dev verification passes.
- Create a PR `staging -> main`.
- Auto-merge it only after required checks pass and staging verification passes.

This gives complete auditability while still satisfying "one click to production".

### 11.3 Files The Automation May Edit

Allowed:

- `dashboard/src/app/changelog/releases.ts`
- `dashboard/tests/changelog-source.test.mjs` only if test fixtures need static source coverage for a new schema shape.
- Optional generated candidate metadata under `docs/changelog-candidates/` if backend table is deferred.

Disallowed:

- API behavior files
- dashboard runtime components unrelated to changelog
- workflow files during a normal daily publish
- feature flag configuration
- secrets or env files

### 11.4 Validation Before Dev Merge

Minimum:

- `node --test tests/changelog-source.test.mjs` from `dashboard/`
- `npm run build` from `dashboard/`
- candidate schema validation
- SDK registry validation when `sdkVersions` is present

If the generated changelog entry changes page layout risk, run:

- `npm run test:regression:dashboard`

### 11.5 Environment Verification

Dev:

- Open `https://dev.unipost.dev/changelog`.
- Confirm the release id exists.
- Confirm title, date, category, source links, and SDK pills if applicable.
- Confirm no browser page errors.

Staging:

- Open `https://staging.unipost.dev/changelog`.
- Run the same checks.

Production:

- Open `https://unipost.dev/changelog`.
- Run the same checks.
- Check `https://api.unipost.dev/health`.

---

## 12. Failure Handling

The orchestrator must stop at the first failed stage.

Examples:

| Stage | Failure | Required behavior |
| --- | --- | --- |
| Candidate validation | SDK version not found | Mark failed, tell Discord no publish happened. |
| Dev PR checks | Dashboard build failed | Leave PR open, send check link. |
| Dev deployment | Vercel failed | Stop before staging, send deployment link. |
| Staging verification | Entry not visible | Stop before main, send verification details. |
| Production verification | Entry not visible | Mark failed, send production deployment and page links. |

The automation must not retry blindly more than once per stage. A manual rerun should be explicit.

---

## 13. Security And Permissions

Required secrets:

- `CHANGELOG_AI_API_KEY` or use the existing server-side AI provider path if implemented there.
- `CHANGELOG_DISCORD_WEBHOOK_URL`
- `CHANGELOG_ACTION_SIGNING_SECRET`
- GitHub token with permission to create branches, PRs, and merge PRs.

Rules:

- Never send secrets to AI.
- Never include secret-bearing URLs in Discord.
- Signed action URLs should expire.
- The release orchestrator should only allow known actions and known candidate ids.
- The action endpoint should be rate limited.
- Production release automation should be restricted to candidates generated by the daily workflow or an explicit manual workflow dispatch.

---

## 14. Observability

Every run should produce a durable audit trail:

- collection window
- included PRs and commits
- excluded PRs and commits
- AI prompt version
- AI model/provider
- validator decisions
- Discord message URL if available
- selected action
- PR URLs
- deployment URLs
- verification results
- final status

The Discord final message should include enough links for a human to inspect the release without searching GitHub or Vercel manually.

---

## 15. Rollout Plan

### Phase 1: Dry Run Digest

- Scheduled workflow collects yesterday's production commits.
- AI generates candidate JSON.
- Validator runs.
- Discord receives message with candidate, but action links are disabled or point to no-op pages.

### Phase 2: Save And Discard Actions

- Add signed action endpoint.
- Implement Save for later and Discard.
- Do not implement Publish yet.

### Phase 3: Publish To Dev Only

- Publish creates changelog branch and PR into `dev`.
- Auto-merge after checks.
- Verify dev.
- Stop and report to Discord.

### Phase 4: Full Release

- Extend Publish to promote `dev -> staging -> main`.
- Verify staging and production.
- Send final success/failure reports.

### Phase 5: Hardening

- Add idempotency dashboard or admin view.
- Add manual rerun for saved candidates.
- Add "edit before publish" if needed.

---

## 16. Open Decisions

1. Candidate store:
   - backend table preferred
   - GitHub issue/artifact acceptable for a lighter v1
2. AI provider:
   - reuse existing backend AI provider routing if the action endpoint owns summarization
   - use GitHub Actions secret if summarization stays in Actions
3. Publish PR behavior:
   - recommended: create PRs and auto-merge after checks
   - alternative: direct branch merges, less auditable
4. Discord message update:
   - use follow-up messages in v1
   - edit original webhook message if message id/token handling is straightforward

---

## 17. Acceptance Criteria

1. Daily workflow can be manually dispatched.
2. Daily workflow sends a Discord candidate for a test commit window.
3. Candidate JSON passes schema validation.
4. SDK versions are verified before any SDK candidate is publishable.
5. Publish action rejects invalid, expired, reused, or tampered links.
6. Save for later changes candidate status and prevents loss of the candidate.
7. Discard suppresses the same source hash from future prompts.
8. Publish creates a changelog update and runs validations before merging to `dev`.
9. Publish waits for dev deployment and verifies `https://dev.unipost.dev/changelog`.
10. Publish promotes to staging only after dev verification passes.
11. Publish promotes to main only after staging verification passes.
12. Production verification checks `https://unipost.dev/changelog` and `https://api.unipost.dev/health`.
13. Any failed stage stops the pipeline and sends Discord failure details.
14. Successful production publication sends Discord success details with links.

---

## 18. References

- GitHub Actions scheduled workflows: https://docs.github.com/actions/using-workflows/workflow-syntax-for-github-actions
- Discord webhook resource: https://docs.discord.com/developers/resources/webhook
- Discord message components: https://docs.discord.com/developers/components/using-message-components
