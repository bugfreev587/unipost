# YouTube Dashboard Source System

## Remediation record

The June 1, 2026 YouTube ToS Violations Report cited Developer Policies III.F.2a and III.F.2b. Its finding was source confusion: official red YouTube graphics appeared beside UniPost-owned profile grouping, health, counts, capabilities, and publishing controls. The finding was not that the icon hyperlink merely needed to point to `youtube.com`.

On July 29, 2026, implementation review confirmed that the current YouTube brand site specifies a 100px minimum digital YouTube Icon height. The API Branding Guidelines defer to current YouTube brand guidance. A 100px official icon is not compatible with the compact authenticated Dashboard surfaces and would become disproportionately prominent. The product owner therefore approved A2: the authenticated Dashboard renders no official YouTube graphic.

Authoritative references:

- [YouTube Developer Policies](https://developers.google.com/youtube/terms/developer-policies)
- [YouTube API Branding Guidelines](https://developers.google.com/youtube/terms/branding-guidelines)
- [YouTube Icon brand guidance](https://brand.youtube/youtube-icon)

## Authenticated Dashboard rules

| Meaning | Treatment |
| --- | --- |
| Concrete connected channel | Real channel avatar, channel name, and validated `Source: YouTube` or `Data from YouTube` text link |
| UniPost action, health, count, status, capability, filter, or destination | Neutral `AccountDestinationIcon`; never official red or official artwork |
| Published YouTube result or Inbox context with a valid URL | Separate validated `View on YouTube` text link |
| Missing or invalid result URL | No external source link |
| Disconnected account | Neutral fallback; no source link and no sentinel-derived channel URL |

`YouTubeChannelIdentity` contains identity only. Health, connection status, management controls, and other UniPost-owned state remain in a separate cell or labeled region. `YouTubeSourceLink` accepts only HTTPS destinations on permitted YouTube hosts and uses a neutral external-link glyph.

## Automated enforcement

`npm run test:youtube-compliance` recursively checks authenticated Dashboard, Admin, and shared component source. It rejects:

- official YouTube red (`#ff0000` or `#f00`) outside the legacy public icon definition;
- the legacy official path fingerprint beginning `M23.498 6.186`;
- YouTube icon/logo image asset references;
- authenticated `<PlatformIcon platform="youtube">` use;
- unclassified dynamic `PlatformIcon` rendering;
- source links that omit URL validation, safe new-tab attributes, accessible naming, or the approved disclosure text.

The CI Dashboard job runs this contract before building the application.

## Public-surface follow-up inventory

The supplied audit evidence concerns authenticated Dashboard surfaces. The following public or shared legacy consumers were inventoried but are not remediated or certified by this task:

- `dashboard/src/app/marketing/page.tsx`
- `dashboard/src/app/about/page.tsx`
- `dashboard/src/app/blog/_components/blog-cover.tsx`
- `dashboard/src/app/tools/_components/public-analytics-tool.tsx`
- `dashboard/src/components/tools/ToolCard.tsx`
- `dashboard/src/components/platform-icons.tsx`

This exclusion is not a statement that YouTube approved those public uses. They require a separate branding review.

## Acceptance evidence to attach to the pull request

- exact task-branch head SHA and isolated Preview URLs;
- successful local and remote `test:youtube-compliance`, build, and regression results;
- light, dark, keyboard-focus, and 390px-width screenshots for Accounts, Analytics, Inbox, Admin, Create Post, and published results;
- proof that text source links open the intended YouTube destination;
- confirmation that the Preview and persistent dev deployment run the accepted SHA.

Do not commit the private audit PDF or screenshots containing private account data. Use synthetic audit accounts or redact identifying data.
