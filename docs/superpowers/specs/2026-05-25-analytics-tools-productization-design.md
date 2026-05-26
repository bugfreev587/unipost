# Analytics Tools Productization Design

## Context

`/tools/tiktok-analytics` is already receiving meaningful visitor traffic, but it was originally shipped as a temporary public preview for the TikTok analytics dashboard. UniPost now supports platform analytics surfaces for TikTok, Instagram, Threads, and Pinterest, so the public tools section should turn that accidental demand into a deliberate product and content surface.

## Goals

- Productize public analytics tool pages for TikTok, Instagram, Threads, and Pinterest.
- Add those analytics pages to `/tools` as live tools.
- Remove the current coming-soon tools from `/tools`.
- Publish one UniPost Analytics blog post that explains Posts Overview and Platform Analytics.
- Keep the work frontend/content only. Do not add a feature flag.

## Non-Goals

- No backend analytics API changes.
- No new authenticated dashboard behavior.
- No Facebook, YouTube, X, LinkedIn, or Bluesky analytics pages in this first batch.
- No platform-specific blog cluster yet; use traffic data from this launch to decide later articles.

## Public Tool Pages

Create four public routes:

- `/tools/tiktok-analytics`
- `/tools/instagram-analytics`
- `/tools/threads-analytics`
- `/tools/pinterest-analytics`

Each page should use a shared public analytics page pattern rather than duplicating four bespoke layouts. The page should feel like a productized tool/preview, not an internal dashboard screenshot.

Each page should include:

- Platform-specific title and SEO metadata.
- Short explanation of what UniPost analytics shows for that platform.
- A sample analytics surface based on the existing dashboard analytics view and safe sample data.
- Supported permission/scope summary.
- Key metrics and content tables relevant to the platform.
- CTA links to start building and read analytics docs.
- Cross-links to the other analytics tool pages.

Platform content:

- TikTok: profile, follower/following stats, total likes, public videos, UniPost-published TikTok post performance.
- Instagram: Business profile, follower/following/media counts, recent media, reach, likes, comments, shares, saves, UniPost-published post performance.
- Threads: profile, follower count, views, replies, reposts, quotes, recent Threads posts, UniPost-published post performance.
- Pinterest: connected boards, published Pins, impressions, saves, outbound clicks, comments, UniPost-published Pin performance.

## Tools Index

Update `/tools` so it only shows live tools:

- AgentPost
- Character Counter
- TikTok Analytics
- Instagram Analytics
- Threads Analytics
- Pinterest Analytics

Remove the current coming-soon cards:

- Thread Splitter
- Caption Generator

## Blog Post

Add one blog post introducing UniPost Analytics. Suggested slug:

`/blog/social-media-analytics-api`

The post should explain:

- Why publishing APIs need analytics, not only posting.
- Posts Overview: cross-platform post performance in one normalized surface.
- Platform Analytics: native platform drilldowns for TikTok, Instagram, Threads, and Pinterest.
- What metrics differ by platform and why UniPost shows unavailable metrics clearly instead of inventing numbers.
- How developers can start with UniPost API and connect accounts.

The blog should link to:

- `/tools/tiktok-analytics`
- `/tools/instagram-analytics`
- `/tools/threads-analytics`
- `/tools/pinterest-analytics`
- `/docs/api/analytics`
- `/docs/api/analytics/posts`
- `/docs/api/accounts/metrics`

## Architecture

Prefer a small config-driven public analytics tool layer:

- A shared component renders the marketing/tool wrapper and platform-specific sections.
- Platform config owns labels, scopes, metric names, sample numbers, table rows, and SEO copy.
- Existing dashboard analytics components can inform the visual and content model, but public pages should not require Clerk or live account data.

Use existing UniPost styling patterns from `/tools` and dashboard analytics. Avoid a new design system or new dependencies.

## SEO

Each analytics tool page should have a unique title and description:

- TikTok analytics API / dashboard preview keywords.
- Instagram analytics API / Business account insights keywords.
- Threads analytics API / profile and post insights keywords.
- Pinterest analytics API / Pin analytics keywords.

Add the new public routes and blog post to the sitemap through existing sitemap behavior.

## Testing

Run dashboard/frontend validation for the changed surface:

- `npm run build` from `dashboard/`.
- If Playwright browsers are available, run `npm run test:regression:dashboard` from `dashboard/`.

Manual checks:

- `/tools` shows only live tools and no coming-soon cards.
- All four analytics tool pages load without authentication.
- Blog article renders, links resolve, and appears in `/blog`.
- The TikTok page no longer says "local preview" or "dashboard preview" in public-facing metadata/copy.
