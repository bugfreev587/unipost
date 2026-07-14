# Platform Documentation Production Alignment Design

## Objective

Audit UniPost's complete public platform-information surface against current production behavior, correct every stale or contradictory claim, and add regression coverage for facts that appear across multiple pages.

The finished documentation must describe what UniPost production supports today. It must not present a capability exposed by an upstream social platform as available unless the production UniPost path actually implements and enables it.

## Scope

The audit covers:

- The platform overview and all nine platform detail pages: X, LinkedIn, Instagram, Threads, TikTok, YouTube, Pinterest, Bluesky, and Facebook.
- Quickstart and Dashboard Quickstart documentation.
- Connect Sessions guides and API reference pages.
- Platform Credentials overview, API reference, and platform-specific credential guides.
- Hosted Connect / White-label overview and platform-specific setup guides.
- Existing platform marketing pages for X, LinkedIn, Instagram, Threads, TikTok, YouTube, Pinterest, and Bluesky.
- Shared SEO content that makes platform, connection-mode, publishing, analytics, inbox, pricing, or approval claims used by the pages above.

Unrelated blog posts, competitor pages, changelog entries, launch collateral, and historical design documents are outside this change unless a scoped public page imports their content directly.

## Source-of-truth hierarchy

Every claim will be evaluated using this order:

1. Current production API behavior and production responses, including the public platform-capabilities endpoint.
2. The latest fetched `origin/main` implementation, tests, routes, connector registration, runtime gates, plan gates, and analytics adapters.
3. Current production deployment configuration when a capability depends on a registered connector or runtime setting and that state can be verified safely.
4. Official upstream platform documentation for native limits and approval requirements.
5. Existing UniPost public prose only as an audit target, never as independent proof.

When sources differ, the public documentation will state the narrower behavior actually available through UniPost production. Environment- or approval-dependent behavior will be labeled explicitly.

## Audit model

The audit will maintain one evidence matrix with a row per platform and these claim groups:

- Connection modes: workspace-owned account connection, Connect Sessions, shared Quickstart credentials, workspace Platform Credentials, and Bluesky app-password behavior.
- Plan and approval constraints: paid-plan gates, custom-platform slots, upstream app review, sandbox restrictions, and public-use requirements.
- Publishing surfaces: text, image, video, carousel, Story, Reel, Short, thread, first comment, link, playlist, board, and platform-specific options.
- Media limits: counts, formats, file sizes, durations, aspect ratios, and mixed-media rules.
- Scheduling and delivery behavior.
- Analytics: post metrics, account metrics, dedicated analytics explorer support, required reconnect scopes, and environment limitations.
- Inbox: comments, replies, direct messages, and reply support.
- Operational limits: UniPost safety caps and any remaining runtime gates.

Each corrected statement must be traceable to at least one production or `origin/main` source. Official platform documentation will be used for native limits only when the production adapter does not impose a narrower value.

## Implementation approach

Use a targeted alignment rather than a broad page rewrite.

- Preserve the existing page structure, visual layout, and routing.
- Correct data tables, summaries, badges, limitations, setup notes, examples, FAQs, and marketing copy where evidence shows drift.
- Remove references to deleted feature flags or obsolete rollout phases.
- Use the same terminology across pages: `Quickstart` for UniPost shared credentials, `Connect Sessions` for customer-owned account onboarding, `Hosted Connect branding` for the UniPost-hosted pre-OAuth page, and `Platform Credentials` for the upstream OAuth app identity.
- Keep environment-dependent caveats beside the affected capability instead of hiding them in generic footnotes.
- Avoid centralizing all platform content in this task. Only extract a shared fact or helper when multiple scoped pages currently encode the same high-risk claim and the extraction materially prevents recurrence.

## Consistency safeguards

Add a focused static regression test for cross-page invariants that have already drifted or are likely to drift. The test will inspect source content rather than browser-rendered wording so it remains fast and deterministic.

Initial invariants include:

- X is not described as lacking the shared Quickstart path while production Connect Sessions can resolve the registered shared Twitter connector with `allow_quickstart_creds=true`.
- All OAuth platforms supported by production Connect Sessions use compatible shared-credential and Platform Credentials terminology.
- Bluesky remains identified as app-password based and is not described as using OAuth app credentials.
- Public pages do not reference removed runtime feature flags such as `FEATURE_FACEBOOK_REELS`.
- YouTube analytics copy includes the currently deployed analytics surface instead of the former limited metric set.
- Platform counts and platform names remain consistent across scoped overview and marketing surfaces.

The test should fail with a message naming the stale claim and affected source file.

## Verification

Local verification will include:

1. Run the new consistency test in its failing state before production-content edits.
2. Make the smallest evidence-backed documentation changes needed for the test to pass.
3. Run the focused consistency test and any existing documentation tests.
4. Run `npm run build` from `dashboard/`.
5. Run `npm run test:regression:dashboard` when Playwright browsers are installed, because the change affects the shared documentation shell and routing surface.
6. Inspect the final diff for unsupported claims, accidental layout changes, and unrelated files.

After task-branch validation, merge into an updated local `dev`, rerun the required checks, and push local `dev` to `origin/dev`. Monitor all triggered checks and development deployments until completion. Then verify the changed pages on `https://dev.unipost.dev` in a real browser, including the platform overview, X, Facebook, YouTube, Connect Sessions, Platform Credentials, White-label, and representative marketing pages.

## Completion criteria

The task is complete only when:

- Every scoped page has been checked against the evidence matrix.
- All identified stale or contradictory claims are corrected.
- Cross-page terminology is internally consistent.
- New and existing local validation passes on both the task branch and updated local `dev`.
- The push to `origin/dev` and all triggered deployments/checks succeed.
- The real development documentation renders correctly and matches the approved expected outcome.
