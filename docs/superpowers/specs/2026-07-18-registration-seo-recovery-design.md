# Registration and SEO Recovery Design

## Objective

Recover qualified registrations by correcting UniPost's homepage search intent, improving indexability, and replacing the current visitor-derived signup proxy with a trustworthy acquisition funnel.

The primary outcome is more real and activated registrations. Total visits and Google clicks are diagnostic metrics only; a decline in irrelevant or automated traffic is acceptable.

## Evidence baseline

The production review used the latest fetched `origin/main`, the production database, the production visitor records, and Google Search Console data available on July 18, 2026.

### Registrations

- Production `users.created_at` contains 29 registrations in June and 6 registrations from July 1 through July 18.
- The same-window comparison is 22 registrations from June 1 through June 18 versus 6 from July 1 through July 18, a decline of about 73%.
- June contained Google-attributed registrations; the July comparison window contained none.
- The current `Bound Signups` visitor metric is not a reliable registration count. It counts sessions that later became associated with a user, including returning users, rather than users whose accounts were created on the displayed day.
- The `users` table is also not an immutable historical ledger because deleting a Clerk user deletes the corresponding application row.

### Visitor quality

- The raw June and July visitor totals are dominated by repeated automated-looking requests.
- Roughly 96% of the `Other` sessions in each month share the same Windows/Chrome user agent and US location while repeatedly scanning pricing, tools, and analytics routes.
- The visitor recorder does not cover every public navigation path, recognizes only a narrow set of Google referrer hosts, and allows internal referrers to appear as acquisition sources.
- Raw visitor totals therefore cannot be used to explain registration movement without traffic classification and a separate registration source of truth.

### Search Console

For July 1–17 compared with June 1–17:

- Clicks increased from 11 to 72.
- Impressions increased from 502 to 1,434.
- CTR increased from 2.2% to 5.0%.
- Average position improved slightly from 7.7 to 7.4.
- The homepage produced 70 of the 72 July clicks.
- Italy produced 44 July clicks, compared with none in the June window.
- The exact `unipost` query produced 59 July clicks; Italy produced 42 of those clicks.

This is not a broad organic visibility collapse. Search visibility increased, but the new traffic was concentrated in a geographically and semantically mismatched brand query.

### Homepage metadata regression

- A July 8 production change used the descriptive title `UniPost | Social Media Posting API for Developers`.
- On July 9, CiteLoop opened a sequence of four commits with the same `Rewrite homepage title and meta description for query relevance` subject. Two were no-ops; two changed `dashboard/src/app/marketing/page.tsx`.
- The first changing commit, `8cb2b639`, incorrectly set `HOMEPAGE_TITLE` to the commit-message text `Rewrite homepage title and meta description for query relevance`.
- The later changing commit, `ff988e56`, replaced that invalid title with the generic title `Unipost` and a generic multi-channel publishing description.
- Those commits were single-parent commits on the CiteLoop PR branch, not merge commits themselves. They entered production through pull request #175.
- After the generic title change, impressions and clicks for the exact `unipost` query increased sharply, driven overwhelmingly by Italy.
- The Italian postal and financial-services brand at `uniposte.it` creates a credible query-intent collision.

The metadata change is the strongest code-correlated explanation for the irrelevant traffic shift. It does not by itself prove causality, but it is specific, reversible, and consistent with the query and country data.

### CiteLoop production-change audit

The audit found 12 commits on `origin/main` authored by `citeloop[bot]` or `CiteLoop` between June 17 and July 18:

- Eight commits changed repository content and four were no-ops.
- Three changing commits published new generated articles on or after July 7.
- Two changing commits caused the homepage metadata regression described above.
- A July 14 Doctor Site Fix shortened one generated article's display title.
- A second July 14 Doctor Site Fix changed `dashboard/src/app/sitemap.ts` so generated `blogPosts` are eligible for sitemap inclusion.
- CiteLoop remained active after the metadata incident, publishing articles on July 16 and July 18. At the time of the audit, the latest `origin/main` commit was the July 18 CiteLoop publish commit `fca9033d`.

The service therefore had continuing authority to propose and merge production SEO/GEO changes; the homepage regression was not an isolated manual edit. The user has now stopped all CiteLoop changes to UniPost until a separate CiteLoop remediation is approved. That operational stop is a prerequisite for this recovery. If CiteLoop resumes touching UniPost before its remediation is approved, implementation and promotion must stop.

The current repository contains 16 files under `content/citeloop/blog`. The production loader:

- Ignores the `canonical` values in generated frontmatter and constructs the rendered canonical as `https://unipost.dev/blog/<slug>`.
- Excludes the `citeloop-dev-verification` fixture from production blog data.
- Rejects generated sources that fail its parser or safety checks.

Therefore, incorrect `dev.unipost.dev` or `/blogs/` canonical strings in generated source frontmatter are not currently emitted as production canonical metadata and are not part of the emergency fix. Generated content still requires a later factual-quality and query-cannibalization review as part of CiteLoop remediation.

### Google systems

- Google's May 2026 core update ran from May 21 to June 2.
- Google's June 2026 spam update ran from June 24 to June 26 and applied globally.
- Google reported no official July broad core update and no broad July crawling, indexing, or serving incident during the reviewed period.
- Search Console reported no manual action.

The spam update is a possible secondary influence, but the production evidence does not support treating a Google algorithm penalty as the primary cause.

### Index coverage

- Search Console reported 13 indexed pages and 38 non-indexed pages at its latest July 9 update.
- The non-indexed set included expected exclusions such as `noindex`, redirects, 401 responses, and 404 responses, plus 9 crawled-but-not-indexed URLs requiring review.
- No sitemap had been submitted in Search Console.
- The current production sitemap returned 50 URLs during the July 18 audit. Every listed URL returned HTTP 200 without a redirect.
- Only three `/blog/` entries were present in the live sitemap during that audit, despite the source implementation allowing generated `blogPosts`. This is an observed source/deployment-output mismatch to test, not evidence that invalid generated URLs are currently in the sitemap.

## Diagnosis

Four separate problems are currently being conflated:

1. **Automation governance:** CiteLoop retained permission to change production SEO/GEO surfaces after producing an invalid title and then a harmful generic title. Restoring metadata without containing that writer would be temporary.
2. **Search intent:** the generic homepage metadata encouraged Google to interpret `Unipost` as an ambiguous brand query, producing a surge of likely irrelevant Italian traffic.
3. **Sitemap submission and regression protection:** the live sitemap is currently clean at the HTTP-status level, but it has not been submitted to Search Console and lacks a deployed-output contract that would prevent future invalid entries or source/runtime drift.
4. **Measurement:** visitor bindings, mutable user rows, incomplete referrer detection, and automated scans prevent the current dashboard from reporting a trustworthy acquisition funnel.

The implementation must address these independently so a movement in one layer does not masquerade as success or failure in another.

## Goals

- Restore homepage metadata that explicitly identifies UniPost as a social-media posting API for developers.
- Reduce ambiguity with unrelated `UniPost`/`UniPoste` brand intent.
- Prevent automated changes from silently reducing the homepage title to a generic brand or arbitrary task text.
- Submit the existing sitemap and enforce that its deployed entries remain public, indexable 200 responses without redirect or conflicting canonical metadata.
- Establish an immutable registration fact independent of user deletion and later visitor activity.
- Capture first-touch acquisition once without allowing return visits to overwrite it.
- Distinguish new signups, attributed signups, activated signups, bound users, and qualified visitors.
- Preserve raw visitor evidence while excluding known and suspected automation from default decision-making.
- Add tests and deployed acceptance that make future metadata and funnel regressions visible.

## Non-goals

- Redesigning the homepage or dashboard.
- Changing pricing, onboarding, registration UX, or CTA copy in this work.
- Blocking countries or suppressing Italian users.
- Treating traffic volume as a success target.
- Hard-coding a country exclusion into attribution.
- Building a general-purpose bot-defense or WAF product.
- Rewriting all marketing content.
- Redesigning CiteLoop, its prompt logic, permissions, or approval workflow. That is a separate remediation project.
- Bulk deleting or rewriting CiteLoop-generated articles without page-level search and factual evidence.
- Adding homepage JSON-LD as part of the emergency metadata repair.
- Adding a feature flag. The approved design uses independent, reversible changes instead.
- Promoting beyond `dev` without explicit user authorization.

## Delivery architecture

Work is divided into three sequential batches:

1. P0 restores and protects homepage search intent.
2. P1 submits the existing sitemap and adds a deployed-output regression contract.
3. P2 establishes the trustworthy registration and attribution funnel.

Each batch has its own focused diff, tests, Preview Acceptance, and rollback boundary. The conversation continues to use only its exclusive `dev-registration-seo-analysis` branch and worktree. If multiple pull requests are needed, they are opened sequentially from that same owned branch after synchronizing it with the newly merged `origin/dev`; no second task branch or worktree is introduced.

CiteLoop's UniPost writer is already stopped by the user. This recovery adds repository-level regression protection, but the separate CiteLoop remediation remains responsible for permanently restricting its target files and correcting its SEO/GEO decision logic.

## Online-user safety contract

The change must not alter the behavior or availability of existing customer workflows:

- Authentication and authorization decisions.
- Account and social-channel connection.
- Post creation, scheduling, publishing, and delivery jobs.
- Existing public API request or response contracts.
- Billing, plan enforcement, and API-key validation.

P0 changes only public-page metadata. P1 changes only public discovery metadata, sitemap output, and canonical behavior. Neither batch may change authenticated application routing or API behavior.

P2 adds observability around registration and admin reporting. Its writes must be **failure-open** for customer-facing flows:

- A signup-event insert failure must not reject, roll back, or delay a valid Clerk signup.
- A fallback attribution or analytics failure must not fail authenticated API bootstrap.
- The Clerk webhook may return a retryable failure for its own event processing, but that failure must not undo or disable the account already created by Clerk.
- Database operations must have bounded timeouts and idempotent retries.
- Reporting endpoints may show delayed or temporarily incomplete analytics when the new data path is degraded; customer product operations must continue normally.
- Errors must be observable so missing analytics are repaired rather than silently accepted.

Database changes are additive:

- Create new tables and indexes without destructive modification of existing customer tables.
- Keep new relationships nullable where necessary for anonymization and safe rollout.
- Perform historical backfill in bounded batches that do not hold long locks on active user or posting tables.
- Do not drop or rewrite existing visitor data.
- Roll back application reads and writes without dropping the new tables.

Production promotion is outside the current authorization. If it is authorized later, the release must monitor signup success, authenticated API error rate and latency, connection flows, publishing flows, and background-job health. A regression in any existing customer workflow is a hard stop and rollback condition even if the new analytics appear correct.

## P0: Homepage search-intent recovery

### Root-cause containment

The user-confirmed CiteLoop stop is the immediate containment control. P0 must also add a repository regression test covering the homepage metadata contract so any future PR, including an automated PR, fails CI when it:

- Uses a standalone generic `Unipost` title.
- Copies a task, prompt, or commit-message phrase into the title.
- Removes the developer/API positioning.
- Produces inconsistent title or description values across primary, Open Graph, and Twitter metadata.

The test must run in the normal dashboard CI path. Making the check required and preventing administrative bypass are repository-governance actions to verify before CiteLoop is ever re-enabled; they do not replace the upstream CiteLoop remediation.

### Metadata

Use the descriptive homepage title:

`UniPost | Social Media Posting API for Developers`

The description should state that UniPost is an API for publishing and managing social-media content across supported platforms. It should lead with the developer and API use case rather than generic brand or channel-management language.

The same product positioning must be reflected in:

- Next.js metadata title and description.
- Open Graph title and description.
- A newly added Twitter summary-card title and description.

The page must retain one canonical URL for `https://unipost.dev`.

The current homepage has no Twitter metadata and no homepage JSON-LD. P0 adds Twitter metadata and its regression assertions. It does not add JSON-LD; structured data can be designed separately when there is a supported schema and a measurable purpose.

### Disambiguation

Use descriptive product terms rather than the standalone title `Unipost`. Do not add homepage structured data in P0, and do not manufacture reviews, ratings, company facts, or other unsupported rich-result fields.

No country-specific blocking or `UniPoste` keyword manipulation is included. Search intent is corrected by clearly describing the actual product.

### P0 acceptance

- The server-rendered homepage contains the approved title, description, canonical, Open Graph, and Twitter values.
- The public page and signed-out registration CTA remain functional.
- A regression test fails for the standalone brand title, the observed CiteLoop commit-message title, missing developer/API intent, or inconsistent primary/social metadata.
- The dashboard production build succeeds.
- Preview browser inspection confirms the rendered head and normal homepage behavior on the exact PR SHA.

### P0 rollback

P0 is a metadata-only rollback. Reverting its focused commit restores the preceding values without a data migration.

## P1: Sitemap submission and deployed-output contract

### Current-state constraint

Do not rebuild or broadly refactor the sitemap. The July 18 production sample contained 50 entries and all returned HTTP 200 without redirect. P1 preserves the existing curated and data-backed route construction unless a deployed test exposes a specific invalid entry.

The deployed sitemap contract is that every emitted URL is:

- Publicly accessible without authentication.
- Free of a declared canonical that points to a different page or host.
- Intended to be indexed.
- Returning HTTP 200 without redirect.
- Not hidden behind a runtime or feature condition.

The contract rejects authenticated routes, redirects, 404s, explicit `noindex`, conflicting canonical declarations, duplicated host variants, and unavailable conditional content. An otherwise valid page's lack of an explicit canonical is not, by itself, a P1 emergency failure.

### Canonical host

`https://unipost.dev` is the canonical marketing host. `www` and other duplicate variants must consistently redirect or canonicalize to it. The implementation should use the existing routing and metadata mechanisms rather than introducing competing canonical declarations.

### Coverage review

Review the 9 crawled-but-not-indexed URLs individually:

- Confirm whether each URL is intended to rank.
- Fix accidental thin, duplicate, non-canonical, or unavailable responses.
- Leave intentionally excluded pages excluded.
- Do not force every discovered URL into the index.

The existing `noindex`, 401, redirect, and 404 groups should be sampled to confirm they are intentional before changing them.

This is a Search Console diagnosis task, not evidence that the current sitemap contains those URLs. No page is changed solely to increase the indexed count.

### Search Console action

After the deployed sitemap is verified, submit `https://unipost.dev/sitemap.xml` in Search Console. This is a separate external-state action and must be announced immediately before execution. A successful submission does not guarantee indexing.

### P1 acceptance

- A deployed check fetches the generated sitemap and then requests every entry.
- Every sitemap URL resolves directly to an indexable 200 response.
- No authenticated, redirecting, missing, or explicitly excluded URL is present.
- Any declared canonical uses the production apex host and does not conflict with the sitemap URL.
- The source/runtime discrepancy in generated-blog sitemap inclusion is explained or removed; the test validates deployed output rather than assuming source enumeration.
- Search Console accepts the sitemap without a parsing or fetch error after production deployment.

### P1 rollback

Code changes can be reverted independently. Search Console submission is not destructive; if a sitemap becomes invalid, the correct response is to restore a valid sitemap or remove the submitted property entry with explicit authorization, not to conceal errors.

## P2: Trustworthy acquisition funnel

### Registration fact model

Add a `signup_events` table whose purpose is to record the registration fact, not mutable account state.

Conceptual fields:

- `id`: internal event identifier.
- `identity_key`: a deterministic non-email key used only for idempotency.
- `user_id`: nullable link to the current application user.
- `registered_at`: Clerk's original account creation time.
- `captured_at`: when UniPost recorded the event.
- `capture_method`: `clerk_webhook`, `api_bootstrap`, or `historical_backfill`.
- `anonymized_at`: nullable timestamp set when the user link is removed.

The final migration must follow the repository's identifier types and privacy conventions. It must not copy email address, name, full Clerk payload, or unnecessary personal data into the event.

Creation rules:

- The Clerk `user.created` webhook is the primary writer.
- The authenticated API bootstrap path provides an idempotent fallback.
- A uniqueness constraint on the identity key prevents duplicate events when both paths execute.
- `registered_at` comes from the identity provider's creation time, not webhook receipt time.
- Repeated login, return visits, and visitor binding cannot change `registered_at`.
- User deletion nulls or anonymizes the user association without deleting the aggregate registration fact, subject to the product's privacy-retention policy.

### Attribution model

Add a one-to-one `signup_attributions` record associated with a signup event.

Conceptual fields:

- `signup_event_id`.
- `landing_visit_id`.
- `source`.
- `landing_path`.
- `referrer_host`.
- UTM source, medium, campaign, term, and content where present.
- Country code where already collected.
- `traffic_class`.
- `attribution_method`.
- `attribution_confidence`.
- `attributed_at`.

The attribution row is a snapshot. It does not depend on a mutable join at reporting time.

### First-touch selection

For a signup event:

1. Consider eligible external landing visits in the 30 days before `registered_at`.
2. Select the earliest eligible visit.
3. Ignore internal UniPost referrers as acquisition sources.
4. Preserve an identified external source when a later visit is Direct.
5. Normalize Google referrers across supported Google country domains instead of matching only `.com`.
6. If the signup is captured before the client binds its landing session, allow the valid session-binding path to create the missing attribution once.
7. Once an attribution row exists, later visits cannot replace it.
8. If no eligible visit exists, report the signup as `Unattributed`; absence of an attribution row is not absence of a signup.

The implementation must define and test any small clock-skew tolerance. It must not attribute visits that clearly occurred after registration.

### Historical backfill

Backfill one signup event for each currently retained user using the existing creation timestamp and mark it `historical_backfill`.

Where an eligible historical landing visit exists, create an inferred attribution and label its confidence accordingly. The backfill cannot recover users that were already deleted or visits that were never recorded. Reports and documentation must disclose this limitation rather than presenting the historical series as complete.

### Traffic classification

Retain raw visitor rows and classify them as:

- `human_or_unknown`
- `known_bot`
- `suspected_automation`

Known-bot classification uses maintained crawler and automation signatures. Suspected automation may use conservative combinations of:

- Repeated identical user-agent/network characteristics.
- Burst rate.
- Repeated scanning of a fixed route set.
- Other existing server-observable request characteristics.

A single unfamiliar user agent or country must not be enough to classify a visitor as automated. Classification affects analytics defaults, not authentication, rate limits, or access control. The current Chrome 148 scanning pattern should be covered by a regression fixture based on its behavior, not by a permanent version-only ban.

### Reporting model

Rename and separate the dashboard metrics:

- **New Signups:** count of `signup_events.registered_at`.
- **Attributed Signups:** new signups with a frozen first-touch attribution.
- **Activated Signups:** new signups that meet the approved activation definition within seven days.
- **Bound Users:** visitor sessions that became linked to any user, retained only as a diagnostic.
- **Qualified Visitors:** visitor sessions classified as `human_or_unknown`.

The default dashboard excludes `known_bot` and `suspected_automation` from qualified-visitor metrics but provides an explicit way to inspect raw totals. Labels and API response fields must not continue calling bound users “signups.”

### Activation definition

A signup is activated when, within seven days of registration, the user completes at least one of:

- Creates an API key.
- Connects one social account or channel.
- Makes a successful core API call or successfully publishes content.

The implementation should derive these facts from existing authoritative records when possible. It should not add duplicate client-only analytics events as the activation source of truth.

### GA4

Emit GA4's `sign_up` event after confirmed registration where consent and the existing analytics loader permit it. GA4 is a secondary cross-check because consent refusal, blocking, or client failure can prevent collection. `signup_events` remains authoritative.

## Verification strategy

### P0 tests

- Metadata unit/static regression coverage for the approved title, description, canonical, Open Graph, and Twitter fields.
- Guards against the standalone title `Unipost`, the observed CiteLoop task-text title, missing developer/API intent, and inconsistent primary/social metadata.
- `npm run build` from `dashboard/`.
- Preview browser verification of rendered metadata, homepage load, and the signed-out registration CTA.

### P1 tests

- A focused source-level test for stable sitemap construction and accidental inclusion of known private route classes.
- A deployed Preview check that fetches the actual sitemap, requests every entry without following redirects, and rejects non-200, redirecting, private, or `noindex` results.
- A deployed check rejects a declared off-host or conflicting canonical but does not fail solely because a page omits an explicit canonical.
- `npm run build` from `dashboard/`.
- Browser or HTTP verification against the deployed Preview for representative sitemap URLs.
- Search Console validation only after an explicitly authorized production deployment.

### P2 tests

- Migration and repository tests for idempotent signup creation.
- Concurrent webhook/bootstrap insertion produces one event.
- Signup-event or attribution write failure does not fail the customer-facing signup/bootstrap path.
- Analytics database timeouts are bounded and do not materially add to customer-facing request latency.
- User deletion removes the live association without erasing the anonymous aggregate event.
- Attribution cases for earliest touch, Direct after known source, internal referrer, Google country domains, UTM precedence, no matching visit, delayed binding, and post-registration rejection.
- Traffic-classification fixtures for known bots, the observed scanner pattern, high-frequency legitimate-looking edge cases, and normal browsers.
- API tests that independently validate new signups, attributed signups, activated signups, bound users, and qualified visitors.
- Frontend tests for metric labels and filters.
- `GOCACHE=/tmp/unipost-go-build go test ./...` from `api/`.
- `npm run build` from `dashboard/`.
- `npm run test:regression:dashboard` when Playwright browsers are installed because the change affects dashboard analytics.

### Preview Acceptance

For every batch:

- Local required checks must pass.
- The branch is pushed only to its owned remote task branch.
- A Draft PR targets `dev`.
- GitHub CI, Railway PR Environment, Vercel Preview, deployed regression, and Codex browser acceptance must all succeed on the exact PR head SHA.
- A missing, skipped, cancelled, timed-out, or mismatched result is a failure and stops the flow.
- Before merge, audit the exact unique commits and changed files.
- After merge to `dev`, wait for development deployments and verify the behavior on the official development domains.

No staging or production promotion is part of this design unless the user explicitly requests the standard release flow or another permitted release action.

## Success metrics

The measurement clock starts when Google has re-crawled and adopted the corrected homepage metadata, not merely when code is deployed.

### Seven-day confirmation

- Google has re-crawled the homepage or Search Console inspection confirms the current rendered metadata.
- The deployed title, description, canonical, and sitemap remain technically correct.

### Fourteen-day leading indicators

- Directional target: approximately 10 real new signups.
- Directional target: approximately 3 Google first-touch signups.
- New-signup and attribution counts reconcile with the authoritative event tables.
- The Italian share of exact-`unipost` clicks moves materially downward, with an initial target below 25%.
- Developer/API-related non-brand impressions or clicks do not fall by more than 20%.

The signup counts are observation targets, not release gates or automatic rollback thresholds. At the current traffic volume, normal day-to-day variance is too large for a 14-day count to establish causality by itself.

### Twenty-eight-day outcome

- Registration pace and activated-signup rate show sustained recovery rather than a one-day spike.
- Search Console traffic is less concentrated in the Italian ambiguous-brand query.
- Qualified organic conversion can be evaluated without automated scans dominating the denominator.

Total clicks may decline without failing the change. A decline caused by removing irrelevant Italian brand traffic is acceptable when qualified registrations and activation improve.

## Decision rules after observation

- If Google retains the old title after re-crawl, inspect duplicate metadata, canonical selection, rendering, and title rewriting before changing copy again.
- If CiteLoop resumes modifying UniPost before its separate remediation is approved, stop the recovery flow and restore the containment boundary.
- If ambiguous Italian brand traffic remains dominant after 28 days, refine product disambiguation and relevant structured data using new query evidence; do not block Italy.
- If qualified developer traffic remains healthy but registration conversion stays low, begin a separate CTA, pricing, and onboarding investigation.
- If registrations increase but activation does not, treat acquisition quality or onboarding as the next problem rather than declaring SEO success.
- Avoid repeated daily metadata changes. Search evaluation requires stable observation windows.

## Rollback and data safety

- P0 is reverted through its focused metadata commit.
- P1 code can be reverted independently; a submitted sitemap should be corrected or explicitly removed rather than hidden.
- P2 uses additive, backward-compatible migrations.
- A P2 application rollback leaves the new tables in place and stops new reads/writes through code; it does not drop captured events.
- No destructive down migration is used as the normal rollback path.
- Existing raw visitor rows remain available for audit.
- Re-enabling CiteLoop is not a rollback mechanism and is outside this recovery.

## Completion criteria

The design is implemented only when:

- All three batches satisfy their local and exact-SHA Preview gates.
- Existing signup, authentication, account connection, publishing, billing, and public API behavior passes regression acceptance.
- Homepage search intent is explicit and protected by regression coverage.
- The deployed sitemap passes its HTTP/indexability contract and is ready for an explicitly authorized Search Console submission.
- Registration events are idempotent and independent of return visits.
- First-touch attribution is frozen and tested.
- Automated traffic is classified without deleting evidence or blocking users.
- Dashboard terminology and calculations distinguish every approved funnel stage.
- Required development deployments complete successfully and are personally verified.
- Any later production release is separately authorized and verified before the business observation clock begins.
