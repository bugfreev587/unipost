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
- A July 9 automated content change replaced it with the generic title `Unipost` and a generic multi-channel publishing description.
- After the generic title change, impressions and clicks for the exact `unipost` query increased sharply, driven overwhelmingly by Italy.
- The Italian postal and financial-services brand at `uniposte.it` creates a credible query-intent collision.

The metadata change is the strongest code-correlated explanation for the irrelevant traffic shift. It does not by itself prove causality, but it is specific, reversible, and consistent with the query and country data.

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

## Diagnosis

Three separate problems are currently being conflated:

1. **Search intent:** the homepage metadata encouraged Google to interpret `Unipost` as a generic brand query, producing a surge of likely irrelevant Italian traffic.
2. **Indexing hygiene:** the site has no submitted sitemap and has unresolved canonical/index-status cases, limiting confidence in page discovery and consolidation.
3. **Measurement:** visitor bindings, mutable user rows, incomplete referrer detection, and automated scans prevent the current dashboard from reporting a trustworthy acquisition funnel.

The implementation must address these independently so a movement in one layer does not masquerade as success or failure in another.

## Goals

- Restore homepage metadata that explicitly identifies UniPost as a social-media posting API for developers.
- Reduce ambiguity with unrelated `UniPost`/`UniPoste` brand intent.
- Ensure the submitted sitemap contains only canonical, indexable, public 200 responses.
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
- Adding a feature flag. The approved design uses independent, reversible changes instead.
- Promoting beyond `dev` without explicit user authorization.

## Delivery architecture

Work is divided into three sequential batches:

1. P0 restores homepage search intent.
2. P1 corrects sitemap, canonical, and indexability fundamentals.
3. P2 establishes the trustworthy registration and attribution funnel.

Each batch has its own focused diff, tests, Preview Acceptance, and rollback boundary. The conversation continues to use only its exclusive `dev-registration-seo-analysis` branch and worktree. If multiple pull requests are needed, they are opened sequentially from that same owned branch after synchronizing it with the newly merged `origin/dev`; no second task branch or worktree is introduced.

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

### Metadata

Use the descriptive homepage title:

`UniPost | Social Media Posting API for Developers`

The description should state that UniPost is an API for publishing and managing social-media content across supported platforms. It should lead with the developer and API use case rather than generic brand or channel-management language.

The same product positioning must be reflected in:

- Next.js metadata title and description.
- Open Graph title and description.
- Twitter title and description.
- Any homepage JSON-LD product/application description that currently repeats the generic wording.

The page must retain one canonical URL for `https://unipost.dev`.

### Disambiguation

Use descriptive product terms rather than the standalone title `Unipost`. Structured data may identify the product as a software application or service only when the existing page and schema support those claims. It must not manufacture reviews, ratings, company facts, or other unsupported rich-result fields.

No country-specific blocking or `UniPoste` keyword manipulation is included. Search intent is corrected by clearly describing the actual product.

### P0 acceptance

- The server-rendered homepage contains the approved title, description, canonical, Open Graph, and Twitter values.
- The public page and signed-out registration CTA remain functional.
- A regression test fails if the homepage title is reduced to the standalone brand name.
- The dashboard production build succeeds.
- Preview browser inspection confirms the rendered head and normal homepage behavior on the exact PR SHA.

### P0 rollback

P0 is a metadata-only rollback. Reverting its focused commit restores the preceding values without a data migration.

## P1: Indexing hygiene

### Sitemap contract

The generated sitemap may contain only URLs that are:

- Publicly accessible without authentication.
- Canonical.
- Intended to be indexed.
- Expected to return HTTP 200.
- Not hidden behind a runtime or feature condition.

The sitemap must exclude login/app routes, 401 responses, redirects, 404s, `noindex` pages, duplicated host variants, and conditional content that is unavailable in production.

### Canonical host

`https://unipost.dev` is the canonical marketing host. `www` and other duplicate variants must consistently redirect or canonicalize to it. The implementation should use the existing routing and metadata mechanisms rather than introducing competing canonical declarations.

### Coverage review

Review the 9 crawled-but-not-indexed URLs individually:

- Confirm whether each URL is intended to rank.
- Fix accidental thin, duplicate, non-canonical, or unavailable responses.
- Leave intentionally excluded pages excluded.
- Do not force every discovered URL into the index.

The existing `noindex`, 401, redirect, and 404 groups should be sampled to confirm they are intentional before changing them.

### Search Console action

After the deployed sitemap is verified, submit `https://unipost.dev/sitemap.xml` in Search Console. This is a separate external-state action and must be announced immediately before execution. A successful submission does not guarantee indexing.

### P1 acceptance

- Every sitemap URL resolves to an indexable canonical 200 response in the target environment.
- No authenticated, redirecting, missing, or explicitly excluded URL is present.
- The canonical host is consistent in rendered metadata and sitemap output.
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
- A guard against the standalone homepage title `Unipost`.
- `npm run build` from `dashboard/`.
- Preview browser verification of rendered metadata, homepage load, and the signed-out registration CTA.

### P1 tests

- A focused sitemap test that enumerates all generated entries and rejects non-canonical, private, redirecting, missing, or `noindex` routes.
- Canonical-host assertions.
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

- At least 10 real new signups.
- At least 3 Google first-touch signups.
- New-signup and attribution counts reconcile with the authoritative event tables.
- The Italian share of exact-`unipost` clicks moves materially downward, with an initial target below 25%.
- Developer/API-related non-brand impressions or clicks do not fall by more than 20%.

### Twenty-eight-day outcome

- Registration pace and activated-signup rate show sustained recovery rather than a one-day spike.
- Search Console traffic is less concentrated in the Italian ambiguous-brand query.
- Qualified organic conversion can be evaluated without automated scans dominating the denominator.

Total clicks may decline without failing the change. A decline caused by removing irrelevant Italian brand traffic is acceptable when qualified registrations and activation improve.

## Decision rules after observation

- If Google retains the old title after re-crawl, inspect duplicate metadata, canonical selection, rendering, and title rewriting before changing copy again.
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

## Completion criteria

The design is implemented only when:

- All three batches satisfy their local and exact-SHA Preview gates.
- Existing signup, authentication, account connection, publishing, billing, and public API behavior passes regression acceptance.
- Homepage search intent is explicit and protected by regression coverage.
- Sitemap and canonical behavior pass the indexability contract.
- Registration events are idempotent and independent of return visits.
- First-touch attribution is frozen and tested.
- Automated traffic is classified without deleting evidence or blocking users.
- Dashboard terminology and calculations distinguish every approved funnel stage.
- Required development deployments complete successfully and are personally verified.
- Any later production release is separately authorized and verified before the business observation clock begins.
