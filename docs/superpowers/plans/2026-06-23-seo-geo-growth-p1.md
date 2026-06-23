# SEO/GEO Growth P1 Execution Plan

## Goal

Execute the remaining SEO/GEO PRD work after the completed P0 batch, merge it through `dev`, push `origin/dev`, and verify the deployed development environment.

## Scope

1. Add source-level SEO tests for the remaining public search surfaces.
2. Ship the three commercial money pages:
   - `/social-media-api`
   - `/social-media-posting-api`
   - `/social-media-publishing-api`
3. Ship solution detail pages:
   - `/solutions/social-media-scheduler-api`
   - `/solutions/ai-agent-social-posting`
   - `/solutions/saas-social-publishing`
   - `/solutions/white-label-social-media-api`
4. Strengthen comparison surfaces:
   - `/compare/social-media-apis`
   - competitor best-fit sections for PostForMe, Zernio, and Ayrshare
5. Add original GEO resource pages:
   - platform requirements matrix
   - posting constraints
   - OAuth/app review requirements
   - media upload workflow limits
   - unified API cost calculator
6. Add execution docs for ads, measurement, and distribution items that require external accounts or ongoing work.

## Validation

Run on the task branch:

```bash
cd dashboard
npm run test:seo
npm run build
```

Then merge to local `dev`, rerun the same validation, push `origin/dev`, monitor triggered remote checks/deployments, and verify the changed public pages on `https://dev.unipost.dev`.
