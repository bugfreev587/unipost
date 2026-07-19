# Public Canonical Coverage Design

## Goal

Add a unique, self-referencing production canonical URL to the 19 public UniPost pages that currently omit one, without changing page content, UI, routing, authentication, sitemap membership, or CiteLoop content.

## Affected routes

The exact production routes are:

- Docs: `/docs`, `/docs/api/inbox/list`, `/docs/api/inbox/reply`, `/docs/api/inbox/sync`, `/docs/guides/x/comments`, `/docs/guides/x/reconnect-permissions`
- Legal: `/privacy`, `/terms`
- Platform landing pages: `/instagram-api`, `/linkedin-api`, `/twitter-api`, `/tiktok-api`, `/youtube-api`, `/pinterest-api`, `/bluesky-api`, `/threads-api`
- Tools: `/tools`, `/tools/agentpost`, `/tools/character-counter`

Each route must render exactly one canonical link whose absolute URL is `https://unipost.dev` plus the route path.

## Architecture

### Platform metadata

The eight platform page modules currently duplicate the same `Metadata` structure. Add a focused metadata builder beside the platform configuration. It accepts a `PlatformConfig`, derives `https://unipost.dev/${config.slug}-api`, and returns the existing title, description, keywords, and Open Graph fields plus `alternates.canonical`. Each platform route exports the builder result. Page rendering and JSON-LD remain unchanged.

### Docs, legal, and tools metadata

The six affected docs modules export a minimal `Metadata` object containing their fixed production canonical. Next.js will merge this route metadata with the existing layout metadata.

The legal and tools pages already export `Metadata`; add an `alternates.canonical` field to those five objects. Do not introduce a request-dependent canonical in the root layout because that could affect authenticated dashboard routes.

## Regression protection

Extend `dashboard/tests/seo-public-pages-source.test.mjs` with a table of all 19 route modules and expected canonical URLs. The test must fail on the current staging source, then pass only when every route is wired to the exact production canonical. Platform tests also exercise the shared metadata builder contract so a future platform addition follows the same pattern.

After the source test passes, run the complete dashboard SEO/source regressions, dashboard build, and dashboard Playwright regression. The existing authenticated local smoke and local landing-host exceptions remain limited to local validation; deployed staging and production acceptance must verify all 19 canonical tags directly.

## Release and acceptance

Use the hotfix flow from the latest `origin/staging`:

1. Push `hotfix-public-canonical-20260719` and open a PR to `staging`.
2. Require exact-head GitHub CI, Railway Preview, staging Vercel/Railway deployment, and browser acceptance.
3. Verify the 19 staging paths are HTTP 200, contain no `noindex`, and render the expected production canonical.
4. Promote `staging` to `main` only after a fresh content audit and all gates pass.
5. Verify the same 19 production paths, homepage SEO metadata, API health, and sitemap health.
6. Sync the owned hotfix branch to the latest `origin/dev`, complete Preview Acceptance, merge to `dev`, and verify the development deployment. Stop on conflicts or any failed required gate.

## Non-goals

- No sitemap additions or removals
- No title, description, Open Graph, Twitter, page-copy, or UI changes
- No authenticated app routing or Clerk changes
- No CiteLoop-generated content or automation changes
- No canonical changes outside the 19 audited public routes
