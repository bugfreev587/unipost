# SEO/GEO Search Execution

Last updated: 2026-06-23

This document turns the PRD's paid search, measurement, and distribution items into execution checklists. Launching ads, submitting Search Console, and posting in communities require external account access and should be treated as external account required steps.

## Google Search Ads Test

### Campaign 1: Unified Social Media API

Landing page: `/social-media-api`

Keyword seed:

- "unified social media api"
- "unified social posting api"
- "one api social media posting"
- "social media api developers"

Ad angle:

- Build social publishing with one API.
- Connect accounts, upload media, publish, and track status.
- Free plan available.

### Campaign 2: Social Media Posting API

Landing page: `/social-media-posting-api`

Keyword seed:

- "social media posting api"
- "post to multiple social platforms api"
- "social media publishing api"
- "api to post to instagram linkedin tiktok"

Ad angle:

- POST /v1/posts for multi-platform publishing.
- Media, scheduling, status, and webhooks.
- Developer docs and free starter plan.

### Campaign 3: Competitor Alternatives

Landing pages:

- `/alternatives/ayrshare`
- `/alternatives/zernio`
- `/alternatives/postforme`

Keyword seed:

- "ayrshare alternative"
- "zernio alternative"
- "postforme alternative"
- "social media api pricing"

Policy note: confirm Google Ads trademark policy and account rules before using competitor names in ad copy.

## Negative Keyword Seed

Start with exact or phrase match. Add this negative keyword seed before launch:

- free instagram followers
- social media manager jobs
- social media marketing course
- social media captions
- social media templates
- hootsuite login
- buffer login
- facebook api jobs
- instagram scraper
- twitter bot spam
- buy followers
- influencer marketplace
- social media calendar template

## UTM Naming

Use one naming system for every ad:

```text
utm_source=google
utm_medium=cpc
utm_campaign=seo_geo_search_p1
utm_term={keyword}
utm_content={campaign}_{adgroup}_{creative}
```

Example:

```text
/social-media-api?utm_source=google&utm_medium=cpc&utm_campaign=seo_geo_search_p1&utm_term=unified_social_media_api&utm_content=unified_api_exact_api_copy_a
```

## Measurement Checklist

Track before launch:

- Baseline organic registrations: 50+
- Baseline paid users from organic: 2
- Baseline active share: roughly one third
- Search Console indexed pages and target query impressions

Track after launch:

- signup count by landing page
- docs clicks from each SEO page
- pricing clicks from each SEO page
- quickstart or API reference clicks from each SEO page
- checkout starts from search traffic
- aggregate search-sourced activation
- aggregate search-sourced paid conversion
- cost per qualified signup
- activated workspace to paid conversion
- query terms that produce high-intent behavior

Do not over-interpret per-page activation or paid conversion until each page has enough volume. At current volume, absolute signup counts and Search Console impressions are more reliable than page-level conversion rates.

## Distribution Checklist

Developer assets:

- Update SDK README links to `/social-media-api`, `/social-media-posting-api`, and `/docs/api/posts/create`.
- Add example snippets that show `POST /v1/posts`, `account_ids`, media, and webhooks.
- Link platform pages from examples when a sample is platform-specific.

Directories and API marketplaces:

- Submit UniPost to credible developer directories and API marketplaces that allow product-led API listings.
- Use `/social-media-api` as the default category landing page.
- Use `/compare/social-media-apis` when the directory asks for category comparison context.

Recipe distribution:

- Prepare n8n, Make, and Zapier-style recipe pages only when the workflow is supported or explicitly planned.
- Do not publish fake integration recipes for unsupported automations.

Community participation:

- Share original resources only when they answer a real thread or question.
- Prefer the platform requirements matrix, OAuth/app review guide, or cost calculator over a generic product pitch.
- Record any earned links or referral traffic in the growth tracker.

## External Account Required

These steps cannot be completed from the repo alone:

- Google Ads campaign creation and budget approval.
- Google Search Console sitemap submission or indexing request.
- Analytics dashboard configuration for signup -> activation -> paid attribution.
- Directory submissions that require founder accounts or payment.
- Community posts that should come from a real founder or company account.
