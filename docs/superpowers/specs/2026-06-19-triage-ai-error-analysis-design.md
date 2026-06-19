# Triage AI Error Analysis Design

## Goal

Improve the admin Error Triage page so a `needs_human_review` bucket explains what the error is, why it likely happened, how an admin can resolve or investigate it, and which concrete failure records support the bucket.

## Scope

- Add structured operator-facing analysis to Error Triage item evidence:
  - `what_is_this_error`
  - `why_it_happened`
  - `how_to_resolve`
  - `missing_evidence`
  - `next_inspection_path`
- Include concrete failure identifiers in item evidence samples:
  - `post_failure_id`
  - `post_id`
  - `social_post_result_id`
- Show this analysis and failure sample evidence on `/admin/error-triage`.
- Add links from failure samples to `/admin/errors` so admins can open the raw failure context.
- Update `AGENTS.md` so feature flags are opt-in by explicit user request, not a default pre-implementation question.

## Non-Goals

- Do not add a feature flag for this change.
- Do not rewrite the Error Triage page layout.
- Do not expose secrets, raw tokens, or unredacted provider payloads.
- Do not change customer email sending behavior.
- Do not create production or staging promotion changes.

## Backend Design

The existing `errortriage.ItemDraft` persists `Evidence` into `error_triage_items.evidence_json`, which makes it the smallest safe extension point for richer review information. The deterministic analyzer will add a `review_analysis` object to evidence for every item, with especially useful fallback content for `needs_human_review` buckets such as `platform_error` at `worker_timeout`.

The OpenAI analyzer will request the same structured fields from AI output. When the model returns safe review analysis, `mergeAISuggestion` will persist it into the draft evidence. If AI is unavailable, low confidence, unsafe, or asks for human review, deterministic fallback analysis still exists so the UI never collapses to only a generic sentence.

Evidence samples will add the database identifiers needed to connect the bucket back to raw failures. Existing sanitization remains in place for messages and debug curls.

## Dashboard Design

The Triage item card will keep the current facts row and existing bug/email sections. It will add:

- an `AI analysis` section with compact What / Why / How rows, plus optional missing evidence and next inspection path rows;
- a `Failure samples` section with up to five evidence samples, sanitized message/debug excerpt, and an `Open raw error` link.

The raw error link will target `/admin/errors` with the best available query identifier. If a sample has `post_failure_id`, that is preferred; otherwise the link falls back to `post_id`.

## Testing

- Add backend tests proving deterministic review analysis is present and includes concrete identifiers.
- Add backend tests proving AI review analysis is merged when provided and deterministic analysis survives human-review safety downgrades.
- Add frontend type helpers and rendering helpers that are covered by TypeScript build.
- Run:
  - `GOCACHE=/tmp/unipost-go-build go test ./internal/errortriage`
  - `GOCACHE=/tmp/unipost-go-build go test ./...`
  - `npm run build` from `dashboard/`

## Acceptance Criteria

- A `needs_human_review` bucket for Instagram/Youtube platform errors no longer shows only generic review text.
- The card answers what the error is, why it happened, and how to resolve or investigate it.
- Admins can see concrete supporting failure samples and navigate to raw error inspection.
- Feature flag guidance in `AGENTS.md` defaults to no feature flag unless the user explicitly requests one.
