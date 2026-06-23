# PRD - SEO/GEO Search Growth

**Status:** Planning
**Owner:** Marketing / Landing / Developer Experience
**Created:** 2026-06-23
**Target:** Improve UniPost visibility for unified social media API, social posting API, and AI search discovery

---

## Problem

UniPost is not consistently visible for high-intent Google searches such as:

- unified API posting social platforms
- unified social media API
- social media posting API
- social media publishing API
- post to Instagram LinkedIn TikTok API

The current SEO motion is too blog-heavy. Blog posts help with long-tail discovery, but competitors rank with product-led search surfaces: category pages, solution pages, integration pages, comparison pages, about/entity pages, docs, and dense internal linking.

The business signal is already strong enough to justify a search-growth push. Organic traffic has produced 50+ registrations, 2 paid users, and roughly one third active users. That means search traffic is not just vanity traffic; it is already bringing users who can activate and pay.

Competitor observations:

- `postforme.dev/about` appears in multiple Google Search results. The page is not only a company story; it targets the category with "Unified Social Media API for Developers" language, explains founder-market fit, links to integrations, solutions, resources, comparisons, docs, GitHub, and Discord, and reinforces entity trust.
- PostForMe appears to run Google Search ads for relevant terms.
- Zernio appears to run paid Google Search ads and also ranks with category/comparison content.
- Bundle.social, Outstand, Ayrshare, PostForMe, and Zernio all place category language directly in titles, headings, above-the-fold copy, code examples, FAQ, and internal links.

Google's own guidance treats GEO/AEO as an extension of SEO, not a separate hack. AI search visibility still depends on being crawlable, indexed, useful, structured, and trusted. UniPost needs a search architecture that helps both classic Google results and AI-generated search experiences understand what UniPost is, who it is for, and why it is a credible source.

## Product Direction

Build a product-led SEO/GEO system for UniPost:

1. Fix crawl/indexing basics first.
2. Repair existing commercial pages that already match target keyword clusters but currently lack server-rendered metadata.
3. Add an `/about` entity-trust page inspired by PostForMe's successful pattern.
4. Add high-intent money pages that directly target developer search queries after the existing pages are rank-eligible.
5. Turn docs, platform pages, alternatives, and tools into a connected topical authority graph.
6. Add original, quotable assets that AI search and external writers can cite.
7. Run a small Google Search ads experiment to validate keyword economics while organic rankings mature.

This work should not become a page farm. Every page must answer a real developer or buyer question and connect to a product action: read docs, compare options, start building, or contact UniPost.

## Goals

1. Improve indexed visibility for "unified social media API", "social media posting API", and related commercial-intent queries.
2. Make UniPost easier for Google and AI search systems to understand as an entity: a developer-first unified social media publishing API.
3. Increase qualified signups from search while preserving or improving activation rate.
4. Create a repeatable page system for category pages, platform pages, solution pages, comparison pages, and trust pages.
5. Add paid-search learning without becoming dependent on ads.
6. Build a measurement loop from query -> landing page -> signup -> activation -> paid conversion.

## Non-goals

- No mass-generated SEO pages with thin or duplicate content.
- No fake mentions, fake reviews, or inauthentic backlinks.
- No production release, staging promotion, or main-branch promotion as part of this PRD.
- No feature flag unless future implementation touches protected API-layer or dashboard behavior.
- No immediate guarantee of Google ranking. Ranking depends on indexing, competition, backlinks, freshness, and user behavior.
- No `llms.txt` or special AI-only markup as a primary ranking strategy. It may be added later for ecosystem expectations, but it is not the core lever.

## Users and Search Intent

| Persona | Search intent | Need |
| --- | --- | --- |
| SaaS founder | "social media posting API" | Ship posting without building every platform integration. |
| Developer | "post to Instagram LinkedIn TikTok API" | Find exact API surface, code examples, auth flow, and platform constraints. |
| AI product builder | "AI agent social media posting API" | Let an agent publish through a safe, documented API. |
| Social scheduler builder | "unified social media API" | Compare infrastructure options before committing engineering time. |
| Buyer comparing vendors | "Ayrshare alternative", "PostForMe alternative", "Zernio alternative" | Understand pricing, platform coverage, white-label, docs, limits, and tradeoffs. |

## Keyword Clusters

### Primary commercial cluster

- unified social media API
- social media posting API
- social media publishing API
- unified social media posting API
- one API to post to social media
- post to multiple social platforms API

### Developer workflow cluster

- post to Instagram API
- post to TikTok API
- post to LinkedIn API
- post to YouTube Shorts API
- upload media social media API
- social media webhook API
- social media OAuth API

### Product/solution cluster

- social media scheduler API
- social media API for SaaS
- AI agent social posting
- social media API for developers
- embedded social publishing
- white label social media API

### Comparison cluster

- Ayrshare alternative
- PostForMe alternative
- Zernio alternative
- Upload-Post alternative
- best unified social media APIs
- social media API pricing

## V1 Scope

### P0: Existing commercial page SEO repair

The highest-leverage first step is to fix commercial pages that already exist and already map to the PRD's keyword clusters.

Verified codebase state:

- `/pricing`, `/compare`, `/solutions`, and `/alternatives/[competitor]` are currently client components.
- These routes do not export per-page metadata from a server component.
- `/alternatives/[competitor]` uses `useParams` and does not define `generateStaticParams`.
- `/compare` targets the "best unified social media APIs" intent but inherits generic root metadata.
- `/alternatives/*` targets competitor-alternative searches but lacks server-rendered title, description, canonical, Open Graph, and route-level static params.

P0 implementation requirements:

- Convert `/pricing`, `/compare`, `/solutions`, and `/alternatives/[competitor]` to server-rendered pages where practical, or split each page into a server wrapper plus client island for interactive pieces.
- Add per-page `metadata` or `generateMetadata`.
- Add canonical URLs and Open Graph metadata.
- Add `generateStaticParams` for `/alternatives/[competitor]`.
- Move or emit JSON-LD from server-rendered page code so crawlers receive it in the initial HTML.
- Keep existing comparison data source discipline, including last-verified dates for third-party claims.

This P0 work should happen before new money pages because it is cheaper and uses pages UniPost already has.

### P0: Crawl and indexing repair

Fix technical issues that can block or dilute search discovery:

- Add a public `robots.txt` that returns `200` on `https://unipost.dev/robots.txt`.
- Ensure `robots.txt` allows public marketing/docs pages and points to `https://unipost.dev/sitemap.xml`.
- Update `dashboard/src/proxy.ts` public-route handling so `robots.txt` is not redirected from the landing host to `app.unipost.dev`.
- Add `dashboard/src/app/robots.ts` or an equivalent static public `robots.txt`.
- Verify the production host behavior directly because local build validation cannot prove the apex-domain proxy path.
- Ensure sitemap includes all public SEO pages, including `/about`, money pages, platform pages, alternatives, tools, docs, and blog posts.
- Fix known sitemap gaps: add `pinterest-api`, `/compare`, `/alternatives/*`, `/about` when built, and the new money pages when built.
- Verify canonical URLs use the preferred host.
- Decide whether `www.unipost.dev` should 301 to `unipost.dev`; prefer 301 if operationally safe.

### P0: Homepage metadata and category alignment

Revise homepage metadata so Google clearly sees the core category:

- Homepage query ownership: homepage owns brand plus "one API for every social platform".
- `/social-media-api` query ownership: the dedicated category page owns the head term "unified social media API".
- Title target to test: `UniPost | Unified Social Media Posting API for Developers`
- Description target: explain account connection, media upload, multi-platform publishing, webhooks/status, and supported platforms.
- H1 should keep product clarity while including category terms naturally.
- Above-the-fold copy should include "social media API", "posting API", and "publishing API" only where it reads naturally.
- The current title `UniPost | Ship Social Publishing in Days` is punchier. Treat the title change as a ranking/CTR tradeoff to review after Search Console data, not as a guaranteed improvement.

### P0: `/about` entity-trust page

Add an `/about` page modeled after PostForMe's effective pattern, adapted for UniPost:

- Title: `About UniPost | Unified Social Media API for Developers`
- H1: `A developer-first social media publishing API`
- Explain why UniPost exists: developers should not have to rebuild OAuth, media handling, publishing, status tracking, webhooks, and analytics for every social platform.
- Explain who UniPost serves: SaaS products, social schedulers, AI content tools, workflow automation, internal tools, and agents.
- Include founder/product credibility without exaggeration.
- Include trust modules: docs, pricing, changelog, supported platforms, API quickstart, comparisons, support/contact, and status if available.
- Add strong internal links to platform pages, solution pages, comparison pages, docs quickstart, and signup.
- Add `Organization`, `SoftwareApplication`, and `BreadcrumbList` structured data when implementation starts.

### P1: Core money pages

Create or strengthen pages that directly match commercial search intent:

| Page | Primary query | Purpose |
| --- | --- | --- |
| `/social-media-api` | unified social media API | Category page for the whole product. |
| `/social-media-posting-api` | social media posting API | Posting-specific API page with code, media, scheduling, status, and webhooks. |
| `/social-media-publishing-api` | social media publishing API | Publishing workflow page for product teams building social features. |

Each money page must include:

- Clear H1 and metadata for the target query.
- One concrete API example.
- Supported platforms.
- Account connection/OAuth explanation.
- Media upload and platform-specific options.
- Webhooks/status tracking.
- Honest limitations and platform constraints.
- FAQ.
- Internal links to docs, platform pages, alternatives, pricing, and signup.

### P1: Solution pages

Add solution pages for real buyer contexts:

- `/solutions/social-media-scheduler-api`
- `/solutions/ai-agent-social-posting`
- `/solutions/saas-social-publishing`
- `/solutions/white-label-social-media-api`

Each solution page should map a user workflow to UniPost primitives:

- Connect customer accounts.
- Upload or reference media.
- Publish or schedule posts.
- Track results.
- Handle webhooks/errors.
- Show platform-specific differences without pushing users to native APIs.

### P1: Comparison and alternative pages

Keep building comparison pages, but make them more useful and defensible:

- Update `/alternatives/postforme` using the latest verified PostForMe positioning, including about-page/entity strategy, open-source/self-host angle, and pricing.
- Update `/alternatives/zernio` and `/alternatives/ayrshare` with clear "best fit" sections.
- Add `/compare/social-media-apis` or strengthen `/compare` to target "best unified social media APIs" without pretending to be neutral.

Comparison rules:

- State who UniPost is best for.
- State who should choose the competitor.
- Include last-verified date.
- Link to source pages where appropriate.
- Do not make unverifiable claims.

### P1: Original GEO assets

Create original assets that AI search, competitors' comparison writers, and developers can cite:

- Social Media API Platform Requirements Matrix.
- Platform posting constraints by network.
- OAuth and app review requirements by platform.
- Media upload limits and workflow comparison.
- Unified API vs native API engineering cost calculator.

These assets should be internally linked from money pages and docs. They should be written as practical references, not marketing fluff.

### P1: Google Search ads experiment

Run a small search ads test to learn which commercial terms convert before organic rankings fully mature.

Initial test structure:

- Campaign 1: `Unified Social Media API`
- Campaign 2: `Social Media Posting API`
- Campaign 3: competitor alternatives where allowed by policy

Landing pages:

- `social media api` queries -> `/social-media-api`
- `posting api` queries -> `/social-media-posting-api`
- competitor queries -> relevant `/alternatives/*`

Ads must use UTMs and should not send broad traffic to the homepage by default.

Initial success metrics:

- Cost per qualified signup.
- Signup to activated workspace.
- Activated workspace to paid conversion.
- Query terms that produce high-intent behavior.

### P2: Distribution and backlinks

Build real, credible distribution:

- GitHub examples and SDK READMEs linking to docs and money pages.
- n8n/Make/Zapier-style recipes if supported or planned.
- Developer directories and API marketplaces.
- Launch posts for major pages/assets.
- Reddit/Hacker News participation only where the content genuinely answers the question.
- Partner/customer case studies when available.

## Content Architecture

The site should become a topic graph, not a pile of independent pages:

```text
Homepage
  -> /about
  -> /social-media-api
  -> /social-media-posting-api
  -> /social-media-publishing-api
  -> /solutions/*
  -> /{platform}-api
  -> /compare and /alternatives/*
  -> /docs/quickstart and API reference
  -> /tools/*
  -> /blog/*
```

Every page should have a clear next step:

- Developer intent: docs quickstart or API reference.
- Buyer intent: pricing, comparison, or contact.
- Evaluation intent: alternatives, platform matrix, or limitations.
- Ready-to-try intent: app signup.

## Structured Data Requirements

Add JSON-LD where it matches visible page content:

- `Organization` sitewide or on `/about`.
- `SoftwareApplication` or `Product` on homepage and money pages.
- `BreadcrumbList` on public pages where breadcrumbs are visible or logically represented.
- `Article` on blog posts.
- `FAQPage` only where FAQ content is visible and stable.

Do not add structured data for facts that are not visible to users.

## Measurement Requirements

Track the full growth path:

- Search query / landing page when available through Search Console.
- UTM source, medium, campaign, term, and content.
- Signup count by landing page.
- Aggregate search-sourced activation and paid conversion.
- Per-landing-page activation and paid conversion only once each page has enough volume to avoid noisy rates.
- Docs clicks and quickstart clicks from SEO pages.
- Pricing clicks and checkout starts from SEO pages.

Baseline to record before rollout:

- Organic has already produced 50+ registrations.
- Organic has produced 2 paid users.
- Roughly one third of those users are active.

30-day targets after P0/P1 launch:

- All P0 pages indexed.
- Search Console impressions grow for target primary keywords.
- At least one target query shows UniPost in top 20.
- Absolute organic signups continue growing versus the pre-rollout baseline.
- Record landing-page signup counts without over-interpreting per-page conversion rates.
- Paid-search experiment identifies at least one keyword/ad/landing-page combination worth continuing or proves ads are uneconomic.

90-day targets:

- Top 10 visibility for at least one primary or near-primary commercial query.
- More than 2 paid customers attributed to organic or paid search.
- Aggregate organic activation remains directionally healthy relative to the current roughly one-third active-user signal.
- At least one original asset earns external links or recurring referral traffic.

## Implementation Plan

### Phase 0: Same-day PRD and P0 technical prep

- Write this PRD.
- Fix existing commercial page SEO eligibility for `/pricing`, `/compare`, `/solutions`, and `/alternatives/[competitor]`.
- Fix `robots.txt` and proxy handling, then validate on the real production landing host.
- Add missing existing routes to sitemap: `pinterest-api`, `/compare`, and `/alternatives/*`.
- Add `/about` to sitemap and navigation/footer plan when implemented.
- Update homepage metadata plan and define query ownership between homepage and `/social-media-api`.
- Confirm current Search Console ownership and sitemap submission status.

### Phase 1: Entity and category pages

- Implement `/about`.
- Implement or strengthen the three money pages.
- Add structured data for `/about`, homepage, and money pages.
- Update sitemap and footer/internal links.
- Validate with local build and deployed dev environment.

### Phase 2: Search experiment

- Prepare ad keyword groups, negative keyword seed list, landing pages, and UTM naming.
- Launch low-budget Google Search ads.
- Review query terms and conversions after enough clicks for directional learning.

### Phase 3: Topic authority expansion

- Add solution pages.
- Strengthen comparison pages.
- Publish first original platform matrix.
- Connect docs, platform pages, tools, and blog posts with internal links.

### Phase 4: Distribution

- Publish example repos or SDK examples.
- Submit to credible directories.
- Share original assets in developer communities where useful.
- Collect customer quotes or case studies when available.

## Acceptance Criteria

### P0 technical

- `https://unipost.dev/robots.txt` returns `200`.
- `robots.txt` includes `Sitemap: https://unipost.dev/sitemap.xml`.
- `https://unipost.dev/sitemap.xml` returns `200` and includes all P0 public SEO pages, including `pinterest-api`, `/compare`, and `/alternatives/*`.
- Public SEO pages are not redirected to `app.unipost.dev`.
- Public SEO pages are crawlable without authentication.
- `/pricing`, `/compare`, `/solutions`, and `/alternatives/[competitor]` return page-specific metadata in initial HTML.
- `/alternatives/[competitor]` has static params for all supported competitor slugs.

### P0 content

- `/about` exists with category-aware title/H1, founder-market-fit explanation, trust signals, and internal links.
- Homepage metadata clearly supports brand plus "one API for every social platform" intent without fighting `/social-media-api` for the same head term.
- Three money pages are explicitly scheduled with owner, target query, and acceptance criteria.
- Each implemented page has at least one docs or quickstart CTA and one signup/pricing CTA.

### Measurement

- Search Console is checked for indexing and impressions.
- Ads use UTMs and do not point all high-intent terms to the homepage.
- Signup counts can be segmented by landing page or campaign.
- Activation and paid conversion are tracked in aggregate for search traffic until per-page sample sizes are large enough.

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Thin pages hurt quality | Require code examples, workflow details, limitations, and links to docs. |
| Pages compete with each other | Assign one primary query per page and use canonical/internal links intentionally. |
| Ads spend without learning | Start small, use exact/phrase match, add negative keywords, review query terms frequently. |
| Claims become stale | Add last-verified dates to comparisons and platform matrices. |
| AI search ignores pages | Focus on crawlable, structured, citable, non-commodity pages; avoid AI-only hacks. |
| Competitors outrank with backlinks | Build original assets and developer distribution instead of relying only on onsite content. |

## Open Questions

- What is the monthly Google Search ads budget for the first experiment?
- Who should be listed as UniPost's public founder/team contact on `/about`?
- Does UniPost have public GitHub repositories, SDKs, Discord, status page, or changelog links that should be first-class trust signals?
- Should `www.unipost.dev` be permanently redirected to `unipost.dev` in the same implementation batch?
- Which analytics source is authoritative for signup -> activation -> paid attribution across organic and paid search?
