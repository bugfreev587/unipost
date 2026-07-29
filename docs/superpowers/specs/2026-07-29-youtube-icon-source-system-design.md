# YouTube Icon Source System Design

## Goal

Improve the appearance and consistency of YouTube icons across the authenticated UniPost Dashboard without recreating the source-attribution confusion identified in YouTube's June 1, 2026 ToS Violations Report.

The design establishes one Dashboard-wide semantic system:

- real YouTube channel or content identity may use an unmodified official YouTube source mark with a compliant link;
- UniPost-owned actions, states, counts, capabilities, scheduling, and health information use a refined neutral destination glyph;
- the UI explicitly labels which information originates from YouTube and which information is managed or calculated by UniPost.

The product owner approved the selected product direction on July 29, 2026. This written PRD was subsequently revised to incorporate code-review findings about Inbox coverage, disconnect sentinels, and Dashboard-wide guardrails; the revised PRD still requires final product-owner approval before implementation planning begins.

## Compliance Basis

The official audit did not identify an incorrect icon URL as the violation. It cited YouTube Developer Policies III.F.2a and III.F.2b and stated that UniPost did not clearly differentiate content that did not originate from YouTube, causing confusion about its origin.

The report highlighted official red YouTube icons near UniPost-owned interface elements including profile grouping, account health, total account counts, capability summaries, and publishing controls. The remediation must therefore address semantic attribution, not only link destinations.

The supplied report's written finding and screenshots concern authenticated UniPost Dashboard surfaces. It does not identify a public marketing or documentation page. That evidence supports limiting this remediation to the authenticated Dashboard, but absence from the report is not a compliance approval for public surfaces. Public uses of `PlatformIcon` remain a separately recorded branding-review risk and must not be represented as covered by this remediation.

The implementation must also comply with the separate YouTube Branding Guidelines:

- an official YouTube logo or icon must be clickable;
- it must link to YouTube content, a YouTube page, or a clearly identified YouTube component;
- it must use an approved, unmodified official asset;
- its color, shape, proportions, visibility, minimum size, and clear space must be preserved;
- it must not be the most prominent element on the page;
- it must not imply that UniPost-generated content or functionality originates from or is endorsed by YouTube.

Authoritative references:

- https://developers.google.com/youtube/terms/developer-policies
- https://developers.google.com/youtube/terms/branding-guidelines

## Scope

### In Scope

- Establish a semantic distinction between YouTube source identity and UniPost operational destinations.
- Add a focused YouTube channel identity component using the existing account name, avatar URL, external channel ID, and status.
- Add a focused official YouTube source-link component that owns all Dashboard use of the official asset.
- Replace the current generic video-camera glyph with a refined neutral rounded-video-frame and play-triangle glyph for UniPost operational surfaces.
- Adopt the channel identity treatment on authenticated Dashboard surfaces that display a real connected YouTube channel, including Accounts, managed-user identity views, and YouTube Analytics.
- Migrate the existing official-red YouTube treatments in authenticated Analytics, Inbox, and Admin user-summary surfaces according to the semantic matrix below.
- Permit the official source mark on real YouTube content or API-data surfaces only when the corresponding source disclosure and compliant destination link are present.
- Extend the current localized operational-surface checks into a Dashboard-wide default-deny policy with one strict official-asset component allowlist.
- Add visual, link, accessibility, and source-attribution regression coverage.
- Document the usage matrix and preserve an audit evidence package for future YouTube reviews.

### Explicit Non-Goals

- Do not change YouTube OAuth, publishing behavior, scopes, token storage, or revocation.
- Do not change the API response model or database schema.
- Do not add a feature flag.
- Do not redesign other platform icons.
- Do not change public marketing or documentation-site platform branding in this task. Inventory those uses and record them as a separate follow-up risk; do not claim that this Dashboard remediation certifies them.
- Do not restore the official YouTube icon to every place that currently uses `AccountDestinationIcon`.
- Do not promote the task to staging or production unless the user later explicitly requests the standard release flow.

## Alternatives Considered

### A. Channel avatar plus independent source mark - selected

The channel avatar is the primary identity. A separate official YouTube source link provides attribution and opens the real channel. UniPost operational surfaces retain a neutral destination glyph.

This provides the clearest ownership hierarchy and the strongest visual improvement while keeping the official mark away from UniPost-generated state.

### B. Official YouTube mark as the primary account icon

This would be simpler and more immediately recognizable, but it gives the official mark more visual weight and makes source confusion more likely when the same row includes UniPost profile, connection, or health data.

### C. Refined neutral icon everywhere

This would be the most conservative audit posture, but it would continue to underrepresent real YouTube channel identity and would not materially improve the source-attribution experience.

## Verified Current-State Inventory

The implementation plan must start from these code-verified facts rather than treating the current rule as a global ban:

- `AccountDestinationIcon` currently renders a Lucide `Video` glyph for YouTube using `currentColor`; replacing it with the refined neutral SVG remains valid.
- `PlatformIcon` contains an inline official-red YouTube treatment (`fill="#ff0000"`).
- `youtube-compliance-ui-source.test.mjs` checks only a named set of operational components and the absence of the red fill literal inside `AccountDestinationIcon`. It does not cover Analytics or Inbox.
- Authenticated Analytics still renders the red `PlatformIcon` in the platform navigation card, the YouTube Analytics page heading, aggregate platform tables, post rows, and result cards.
- Authenticated Inbox still renders a dynamic `PlatformIcon` at the conversation-list identity and original-post context. Both can resolve to YouTube while appearing beside UniPost unread, thread-status, publish-status, and other managed state.
- Authenticated Admin user summaries also render dynamic `PlatformIcon` collections that can resolve to YouTube beside UniPost-owned user and account state.
- The only current direct YouTube channel URL construction is in `youtube-analytics-view.tsx`; it does not reject disconnected sentinel IDs.
- Disconnect writes `external_account_id = 'disconnected:' || id`, clears `account_name` and `account_avatar_url`, and sets status to `disconnected`. It does not clear the external-account-ID field.

## Semantic Usage Model

### 1. YouTube channel identity

Use a composed channel identity when the UI represents a specific connected YouTube channel.

The composition contains:

- the real channel avatar as the primary visual;
- a neutral initials fallback when the avatar is unavailable;
- the channel name;
- the disclosure `YouTube channel · Source: YouTube`;
- an independent official YouTube source link that opens the real channel.

The official source mark does not overlap the avatar and is not used as the avatar fallback.

Eligible surfaces:

- the Accounts table account cell;
- managed-user account detail rows that represent a specific channel;
- managed-user channel lists when an individual account identity is available;
- YouTube Analytics channel headers;
- other authenticated account-identity views that contain a concrete `SocialAccount`.

When a row also contains UniPost health, connection, or management state, the channel identity and its source link must occupy an identity cell or region separate from the status cell or region. The source disclosure must say `Source: YouTube`; the operational region must say `UniPost-managed` or use an equally explicit UniPost-owned label. The official source mark must never be merged into the health/status cluster.

### 2. UniPost operational destination

Use the neutral destination glyph when the UI represents a UniPost action, filter, state, derived capability, count, or workflow destination.

The glyph is a rounded video frame with an inset play triangle. It uses `currentColor`, follows the Dashboard theme, and never uses YouTube red.

Required neutral surfaces include:

- the platform Connect picker;
- account selection and removal controls in Create Post;
- platform editor headings;
- publishing destination chips;
- calendar filters, events, and inspectors;
- connection-health and account-count summaries;
- capability summaries;
- source-platform counts calculated by UniPost;
- post-list filters and other operational badges;
- Analytics navigation cards, aggregate platform tables, post rows, and result-status clusters;
- Inbox conversation-list rows, unread counts, thread-status controls, and original-post status metadata;
- Admin user platform summaries and account-state badges;
- disconnected-account states.

These surfaces may use the word `YouTube` as a destination label, but they must not use the official source asset.

### 3. Real YouTube content or API data

The official source mark may appear next to a real YouTube channel, published YouTube video, or YouTube API data only when all of the following are true:

- the element clearly represents YouTube-origin content or data;
- the mark is rendered by the approved source-link component;
- the mark opens the corresponding channel, video, YouTube API component, or YouTube home page;
- nearby copy says `View on YouTube`, `Source: YouTube`, or `Data from YouTube` as appropriate;
- API-derived metrics display their freshness or fetch time when the surface supports it;
- UniPost-derived information on the same surface is separately labeled as UniPost-owned.

Inbox is intentionally conservative. A YouTube Inbox conversation row uses the neutral destination glyph because that row is also a UniPost-managed unread/thread-status control. It shows visible text such as `Source: YouTube` rather than using the official mark as the source signal. The original-post or comment context may expose `YouTubeSourceLink` only when the item has a validated YouTube content URL, and that link must live in a separate action region from Resolve/Re-open, unread, publish status, reply, and other UniPost controls. Without a valid YouTube URL, Inbox uses the neutral glyph and text disclosure only.

## Surface Matrix

| Dashboard surface | Visual treatment | Official asset allowed | Required disclosure or behavior |
| --- | --- | --- | --- |
| Accounts: connected channel identity | Channel avatar plus source link | Yes | `YouTube channel · Source: YouTube`; link to channel |
| Accounts: Connect platform picker | Neutral destination glyph | No | Platform name only; OAuth remains a UniPost flow |
| Connection stats and health | Neutral destination glyph | No | `UniPost-managed account health` and `Source platform` |
| Managed-user individual channel identity | Channel avatar plus source link | Yes | Channel disclosure and channel link |
| Managed-user aggregated platform counts | Neutral destination glyph | No | Count remains visibly UniPost-generated |
| Create Post account selector and editor | Neutral destination glyph | No | `YouTube channel` as destination label; capability copy remains attributed to UniPost |
| Calendar, filters, and scheduled-post UI | Neutral destination glyph | No | State and scheduling remain attributed to UniPost |
| Analytics platform navigation and aggregate tables | Neutral destination glyph | No | Navigation, aggregation, and counts remain visibly UniPost-generated |
| Analytics post rows and result-status cluster | Neutral destination glyph | No | Keep platform destination separate from UniPost status |
| Published YouTube result with valid URL | Official source link | Yes | `View on YouTube`; link to actual result |
| Result without a valid YouTube URL | Neutral destination glyph | No | Do not imply that a usable YouTube content link exists |
| YouTube Analytics channel header | Channel avatar plus source link | Yes | `Data from YouTube`; show fetch time where applicable |
| YouTube Analytics UniPost-derived state | Neutral destination glyph or no icon | No | Label as UniPost-generated or managed |
| Inbox YouTube conversation list and thread state | Neutral destination glyph | No | Visible `Source: YouTube`; unread and status remain `UniPost-managed` and visually separate |
| Inbox YouTube content context with validated content URL | Neutral context identity plus separate official source link | Yes, link region only | `View on YouTube`; keep the link separate from Resolve/Re-open, reply, unread, and publish status |
| Inbox YouTube content context without validated URL | Neutral destination glyph | No | `Source: YouTube`; no official mark or `View on YouTube` claim |
| Admin user platform and account summaries | Neutral destination glyph | No | Platform membership and account state remain visibly UniPost-managed |

## Component Architecture

### `YouTubeSourceLink`

This is the only authenticated Dashboard component permitted to import or render the new official YouTube source asset. The existing shared `PlatformIcon` may retain its current public and marketing behavior because those surfaces are outside this task, but authenticated Dashboard consumers must not use `PlatformIcon` in any expression that can resolve to YouTube.

Responsibilities:

- render the approved, unmodified official asset;
- preserve its original color, proportions, visibility, clear space, and solid-background requirements;
- use a conservative visual height of at least 24 CSS pixels unless the current downloaded official asset specifies a larger minimum;
- provide at least a 44-by-44 CSS pixel interactive target;
- render a keyboard-focusable anchor rather than a decorative span;
- open the resolved YouTube destination in a new tab;
- set `rel="noopener noreferrer"`;
- set an accessible label such as `Open Xiaobo Yu on YouTube` or the fallback `Open YouTube`;
- expose a visible focus state in light and dark themes;
- never adopt UniPost health, success, warning, selection, or disabled colors.
- remain visually subordinate to the channel avatar, content thumbnail/title, or page heading that establishes the primary identity.

The implementation must use an official downloadable asset rather than copying or redrawing the current inline SVG path. The source asset must be kept in one documented location under the Dashboard's static brand assets.

### `YouTubeChannelIdentity`

This focused composition owns a concrete YouTube `SocialAccount` identity.

Inputs:

- `account_name`;
- `account_avatar_url`;
- `external_account_id`;
- `status`;
- density or size variant when needed by a list.

Responsibilities:

- render the channel avatar or initials fallback;
- render the channel name or `YouTube channel` fallback;
- render the visible source disclosure;
- render `YouTubeSourceLink` only when the account state permits it;
- avoid taking over the row's disconnect, selection, or editing action;
- render in an identity cell or region separate from `UniPost-managed` health, status, selection, and editing controls;
- support light, dark, desktop, and mobile layouts.

This is intentionally YouTube-specific. The task does not introduce a generalized all-platform identity framework.

### `AccountDestinationIcon`

Keep this existing component for UniPost operational surfaces.

For `platform === "youtube"`, replace the current Lucide video-camera icon with a small internal neutral SVG primitive consisting of a rounded video frame and play triangle. This avoids a new dependency and prevents the neutral glyph from imitating or reusing an official YouTube brand asset.

The component remains decorative and `aria-hidden` when surrounding text already supplies the accessible name.

### Channel URL builder

Add a focused helper that converts a valid YouTube channel ID into:

`https://www.youtube.com/channel/{encodedChannelId}`

The helper must reject empty IDs and disconnected sentinels such as `disconnected:{accountId}`. It must not accept a caller-provided arbitrary URL.

Replace the single existing direct channel URL construction in `youtube-analytics-view.tsx` with this sentinel-aware helper. The purpose is centralized validation and safe fallback behavior, not deduplication of multiple existing callers.

## Data and Fallback Behavior

The existing Dashboard `SocialAccount` already provides the required fields, so no API change is needed.

### Valid active channel ID

- Link the official source mark to the exact channel page.
- Use the account name in the accessible label.

### Missing or invalid channel ID on an otherwise active account

- Link the source mark to `https://www.youtube.com/` because the Branding Guidelines explicitly permit the YouTube home page.
- Change the accessible label to `Open YouTube` rather than claiming a channel-specific destination.
- emit a non-sensitive monitoring signal identifying the UniPost account ID and missing-channel-ID condition without logging credentials or tokens;
- keep the visible disclosure `YouTube channel · Source: YouTube`.

### Missing or broken avatar

- Fall back to a neutral initials avatar derived from the channel name.
- If the name is also missing, use the neutral destination glyph inside the avatar; do not invent a `YT` abbreviation or fall back to the official YouTube mark.
- Do not fall back to the official YouTube icon as the avatar.

### Disconnected account

- Do not render the official source link. Disconnect replaces the external-account-ID field with the non-channel sentinel `disconnected:{accountId}`, clears the stored channel name and avatar, and sets status to `disconnected`.
- The URL helper must reject that sentinel before any URL is built; never create `youtube.com/channel/disconnected:...`.
- Use the neutral destination glyph or initials avatar.
- Label the identity `Disconnected YouTube channel` and keep UniPost status visually separate.

### Published result without a valid external URL

- Keep the neutral destination glyph.
- Do not render `View on YouTube` or an official source link until a valid result URL exists.

## Visual Specification

### Channel identity

- Standard avatar: 40 to 42 CSS pixels.
- Compact avatar: 32 CSS pixels only where the surrounding list cannot support the standard size.
- Avatar shape: 10 to 12 CSS pixel corner radius; no circular social-logo treatment.
- Channel name: existing Dashboard type scale and weight; no oversized platform branding.
- Source copy: existing muted small-text token, with normal capitalization and no all-caps brand styling.

### Official source link

- Use the official asset as downloaded at implementation time.
- Visual height: at least 24 CSS pixels, or the larger current official minimum.
- Click target: at least 44 by 44 CSS pixels.
- Background: a solid neutral surface with sufficient contrast in both themes.
- Border: subtle Dashboard border token; no red outline or glow.
- Motion: optional one-pixel upward transform on hover and a short 120-180 ms transition; no perpetual animation.
- Focus: visible high-contrast focus ring that is not dependent on YouTube red alone.

### Attribution and prominence separation

- A source identity and UniPost health/status must use separate cells on table layouts or separate labeled regions on cards and mobile layouts.
- The source region uses `Source: YouTube`, `Data from YouTube`, or `View on YouTube`; the operational region uses `UniPost-managed` or other explicit UniPost-owned wording.
- The official mark's visible artwork is never larger or more visually dominant than the primary channel avatar, content thumbnail/title, or page heading. The 44-by-44 click target does not increase the artwork size.
- No success, warning, unread, selected, or health color may be applied to the official artwork or its container.

### Neutral destination glyph

- Standard visual size: 18 CSS pixels.
- Compact visual size: 14 CSS pixels.
- Stroke: 1.8 CSS pixels at the 24-unit source view box.
- Color: `currentColor` using existing muted Dashboard tokens.
- Container: existing operational icon containers may remain; do not add red backgrounds or brand gradients.

## Accessibility and Interaction

- Every official source mark is a real anchor and keyboard accessible.
- The clickable region is at least 44 by 44 CSS pixels even if the official asset is visually smaller.
- Accessible names describe the action and destination rather than the icon shape.
- The source link does not intercept selection, disconnect, editing, or row-navigation controls.
- Decorative neutral destination glyphs remain hidden from assistive technology when adjacent text names the platform.
- Source disclosure remains visible text and is not encoded only by color or iconography.
- Light and dark themes must maintain sufficient contrast for text, borders, hover states, and focus indicators.
- Mobile layouts must not create horizontal overflow or reduce the source-link target below 44 CSS pixels.

## Automated Compliance Guardrails

The current `dashboard/tests/youtube-compliance-ui-source.test.mjs` is a localized operational-surface rule, not a global ban. It names several operational components, forbids `PlatformIcon` in those files, and checks only that `AccountDestinationIcon` lacks the `fill="#ff0000"` literal. Analytics and Inbox are not covered. The implementation must expand this into a stronger Dashboard-wide default-deny rule while preserving the existing operational checks.

Required automated checks:

1. The downloaded official YouTube source asset is imported or referenced only by `YouTubeSourceLink` within authenticated Dashboard code. Verify the approved asset by a recorded content hash so a renamed copy does not evade the rule.
2. A brand-feature scan covers authenticated Dashboard routes and shared Dashboard components. Outside `YouTubeSourceLink`, it rejects official-red fill variants, the official SVG path/signature, copied inline official artwork, and references to the approved asset. The legacy shared `PlatformIcon` definition is an explicit public-surface containment exception, not an authenticated Dashboard allowlist.
3. An authenticated-surface inventory fails on any unclassified `PlatformIcon` expression that is literal YouTube or can dynamically resolve to YouTube. This explicitly covers `app/(dashboard)`, `app/admin`, Analytics, and Inbox as well as the original operational components.
4. `AccountDestinationIcon` contains the neutral YouTube destination glyph and no official YouTube fill, path, copied artwork, or asset reference.
5. Operational surfaces including Connect, Create Post, Calendar, Connection Stats, capability summaries, Analytics navigation/aggregates/status clusters, Inbox list/status regions, Admin user summaries, and aggregate platform badges use the neutral treatment and never `YouTubeSourceLink`.
6. Identity surfaces use `YouTubeChannelIdentity` only when rendering a concrete YouTube account. Component tests assert that its source region and UniPost health/status region are separate and carry the required disclosures.
7. Every `YouTubeSourceLink` renders an anchor with a validated permitted YouTube destination, `target="_blank"`, `rel="noopener noreferrer"`, and an accessible name.
8. Channel URL tests cover valid IDs, empty IDs, disconnected sentinels, and encoding behavior.
9. Avatar tests cover valid images, image failure, missing name, and disconnected state.
10. Published-result and Inbox tests prove the official source link appears only when a validated YouTube content URL exists and never inside the status/control cluster.
11. Light and dark browser acceptance captures Accounts identity, Analytics, Inbox, operational destination treatment, keyboard focus, and mobile layout.
12. Browser acceptance records the visible artwork dimensions and surrounding hierarchy, confirming that each official mark is subordinate to the avatar, content, or page heading and is not the page's most prominent element.
13. The original audit surfaces are rechecked: Create Post, Quickstart/Accounts, profile grouping, capability summary, account health, and platform counts.

The static guard must fail both when a future developer imports or copies the official asset into another authenticated Dashboard component and when a new authenticated `PlatformIcon` use can render YouTube without classification. Import ownership alone is insufficient because inline SVG artwork could otherwise bypass it.

## Verification

Local verification for the implementation must include:

- focused source and component tests for the semantic allowlist;
- focused URL and fallback tests;
- `npm run build` from `dashboard/`;
- `npm run test:regression:dashboard` when Playwright browsers are installed;
- light-theme, dark-theme, keyboard, and mobile browser acceptance.
- a complete authenticated-surface inventory proving that Analytics, Inbox, and Admin user summaries no longer render the legacy red `PlatformIcon` for YouTube;
- a read-only public-surface `PlatformIcon` inventory recorded as follow-up risk, without changing those surfaces in this task.

Preview Acceptance must run against the exact Draft pull request head SHA and must include:

- GitHub CI success;
- Vercel Preview success;
- Railway PR Environment success, even though the task is frontend-only, because it is a required repository gate;
- deployed regression success;
- Codex browser acceptance on the isolated Preview;
- screenshots of the original audit-equivalent screens and the newly permitted source-link contexts;
- screenshots of Analytics, Inbox, and Admin user summaries showing source/UniPost-state separation;
- confirmation that every official source mark opens the intended YouTube destination.
- confirmation by visual review and measured artwork dimensions that no official mark is the most prominent element on its page or card.

After the task pull request is accepted and merged into `dev`, wait for the persistent development deployments and repeat acceptance on `https://dev-app.unipost.dev` and the corresponding development API. Staging and production promotion are not part of this design unless explicitly requested later.

## Audit Evidence Package

Preserve a small, reviewable evidence set with the implementation:

- the semantic surface matrix from this design;
- links to the current YouTube Developer Policies and Branding Guidelines;
- the exact implementation commit SHA and Preview URL;
- light and dark Accounts screenshots;
- Create Post, Calendar, Connection Stats, Analytics, Inbox, Admin user summary, and published-result screenshots;
- automated allowlist, link-contract, build, and regression results;
- a short explanation that YouTube-origin identity uses the official linked source mark while UniPost-owned actions and metrics use neutral glyphs.
- the public marketing/documentation `PlatformIcon` inventory and an explicit statement that those surfaces were not remediated or certified by this task.

Do not commit the private audit PDF or screenshots containing private account data unless the user explicitly approves their inclusion. Redact channel names or use dedicated audit accounts when artifacts are persisted.

## Definition of Done

- The Accounts page is visually improved through real channel avatars and a clear source hierarchy.
- Every official YouTube source mark in the authenticated Dashboard is rendered by the single approved source-link component.
- Every official source mark is unmodified, large enough, fully visible, keyboard accessible, and linked to a permitted YouTube destination.
- Every official source mark is visually subordinate to the channel avatar, content, or page heading and is not the most prominent element on the page or card.
- No official YouTube source mark is placed inside or visually grouped with UniPost-generated health, counts, capabilities, scheduling, profile grouping, or operational controls.
- When source identity and UniPost status share a row or card, they occupy separate labeled cells or regions (`Source: YouTube` versus `UniPost-managed`) and the official mark is absent from the status/control region.
- Authenticated Analytics, Inbox, and Admin user summaries have no remaining legacy red `PlatformIcon` path that can resolve to YouTube; Inbox uses neutral source treatment unless a validated YouTube content URL enables a separate official source link.
- All original audit-equivalent screens clearly distinguish YouTube-origin data from UniPost-owned information.
- Missing IDs, broken avatars, disconnected accounts, and missing result URLs degrade without false YouTube attribution.
- Focused tests, the Dashboard build, required regression, Preview Acceptance, and real development acceptance all pass on the exact applicable SHAs.
- The audit evidence package is complete and contains no unapproved private user data.
- Public marketing/documentation uses are inventoried as out-of-scope follow-up risk and are not described as certified by this Dashboard remediation.
