# YouTube Icon Source System Design

## Goal

Improve the appearance and consistency of YouTube icons across the authenticated UniPost Dashboard without recreating the source-attribution confusion identified in YouTube's June 1, 2026 ToS Violations Report.

The design establishes one Dashboard-wide semantic system:

- real YouTube channel or content identity may use an unmodified official YouTube source mark with a compliant link;
- UniPost-owned actions, states, counts, capabilities, scheduling, and health information use a refined neutral destination glyph;
- the UI explicitly labels which information originates from YouTube and which information is managed or calculated by UniPost.

The product owner approved this design on July 29, 2026.

## Compliance Basis

The official audit did not identify an incorrect icon URL as the violation. It cited YouTube Developer Policies III.F.2a and III.F.2b and stated that UniPost did not clearly differentiate content that did not originate from YouTube, causing confusion about its origin.

The report highlighted official red YouTube icons near UniPost-owned interface elements including profile grouping, account health, total account counts, capability summaries, and publishing controls. The remediation must therefore address semantic attribution, not only link destinations.

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
- Adopt the channel identity treatment on authenticated Dashboard surfaces that display a real connected YouTube channel.
- Permit the official source mark on real YouTube content or API-data surfaces only when the corresponding source disclosure and compliant destination link are present.
- Replace the current blanket source-level ban on the official YouTube icon with a strict component allowlist.
- Add visual, link, accessibility, and source-attribution regression coverage.
- Document the usage matrix and preserve an audit evidence package for future YouTube reviews.

### Explicit Non-Goals

- Do not change YouTube OAuth, publishing behavior, scopes, token storage, or revocation.
- Do not change the API response model or database schema.
- Do not add a feature flag.
- Do not redesign other platform icons.
- Do not change public marketing or documentation-site platform branding in this task.
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
| Published YouTube result with valid URL | Official source link | Yes | `View on YouTube`; link to actual result |
| Result without a valid YouTube URL | Neutral destination glyph | No | Do not imply that a usable YouTube content link exists |
| YouTube Analytics channel header | Channel avatar plus source link | Yes | `Data from YouTube`; show fetch time where applicable |
| YouTube Analytics UniPost-derived state | Neutral destination glyph or no icon | No | Label as UniPost-generated or managed |

## Component Architecture

### `YouTubeSourceLink`

This is the only authenticated Dashboard component permitted to import or render the new official YouTube source asset. The existing shared `PlatformIcon` may retain its current public and marketing behavior because those surfaces are outside this task, but authenticated Dashboard consumers must not use `PlatformIcon` to render YouTube.

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
- support light, dark, desktop, and mobile layouts.

This is intentionally YouTube-specific. The task does not introduce a generalized all-platform identity framework.

### `AccountDestinationIcon`

Keep this existing component for UniPost operational surfaces.

For `platform === "youtube"`, replace the current Lucide video-camera icon with a small internal neutral SVG primitive consisting of a rounded video frame and play triangle. This avoids a new dependency and prevents the neutral glyph from imitating or reusing an official YouTube brand asset.

The component remains decorative and `aria-hidden` when surrounding text already supplies the accessible name.

### Channel URL builder

Add or reuse a focused helper that converts a valid YouTube channel ID into:

`https://www.youtube.com/channel/{encodedChannelId}`

The helper must reject empty IDs and disconnected sentinels such as `disconnected:{accountId}`. It must not accept a caller-provided arbitrary URL.

The same helper should be reused by YouTube Analytics instead of duplicating channel URL construction.

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

- Do not render the official source link because UniPost clears the stored YouTube external account ID on disconnect.
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

The current `dashboard/tests/youtube-compliance-ui-source.test.mjs` enforces a blanket ban by proving the operational destination component does not contain the official red fill. Replace that blanket rule with a stronger semantic allowlist.

Required automated checks:

1. The new official YouTube source asset is imported or referenced only by `YouTubeSourceLink` within authenticated Dashboard code. Existing public or marketing `PlatformIcon` behavior is outside this task and remains a separate allowlist entry.
2. `AccountDestinationIcon` contains the neutral YouTube destination glyph and no official YouTube fill, path, or asset reference.
3. Operational surfaces including Connect, Create Post, Calendar, Connection Stats, capability summaries, and aggregate platform badges use `AccountDestinationIcon` and never `YouTubeSourceLink`.
4. Identity surfaces use `YouTubeChannelIdentity` only when rendering a concrete YouTube account.
5. Every `YouTubeSourceLink` renders an anchor with a permitted YouTube destination, `target="_blank"`, `rel="noopener noreferrer"`, and an accessible name.
6. Channel URL tests cover valid IDs, empty IDs, disconnected sentinels, and encoding behavior.
7. Avatar tests cover valid images, image failure, missing name, and disconnected state.
8. Published-result tests prove the official source link appears only when a valid YouTube result URL exists.
9. Light and dark browser acceptance captures the Accounts identity treatment, operational destination treatment, focus state, and mobile layout.
10. The original audit surfaces are rechecked: Create Post, Quickstart/Accounts, profile grouping, capability summary, account health, and platform counts.

The static guard should fail if a future developer imports the official asset directly into another Dashboard component.

## Verification

Local verification for the implementation must include:

- focused source and component tests for the semantic allowlist;
- focused URL and fallback tests;
- `npm run build` from `dashboard/`;
- `npm run test:regression:dashboard` when Playwright browsers are installed;
- light-theme, dark-theme, keyboard, and mobile browser acceptance.

Preview Acceptance must run against the exact Draft pull request head SHA and must include:

- GitHub CI success;
- Vercel Preview success;
- Railway PR Environment success, even though the task is frontend-only, because it is a required repository gate;
- deployed regression success;
- Codex browser acceptance on the isolated Preview;
- screenshots of the original audit-equivalent screens and the newly permitted source-link contexts;
- confirmation that every official source mark opens the intended YouTube destination.

After the task pull request is accepted and merged into `dev`, wait for the persistent development deployments and repeat acceptance on `https://dev-app.unipost.dev` and the corresponding development API. Staging and production promotion are not part of this design unless explicitly requested later.

## Audit Evidence Package

Preserve a small, reviewable evidence set with the implementation:

- the semantic surface matrix from this design;
- links to the current YouTube Developer Policies and Branding Guidelines;
- the exact implementation commit SHA and Preview URL;
- light and dark Accounts screenshots;
- Create Post, Calendar, Connection Stats, and published-result screenshots;
- automated allowlist, link-contract, build, and regression results;
- a short explanation that YouTube-origin identity uses the official linked source mark while UniPost-owned actions and metrics use neutral glyphs.

Do not commit the private audit PDF or screenshots containing private account data unless the user explicitly approves their inclusion. Redact channel names or use dedicated audit accounts when artifacts are persisted.

## Definition of Done

- The Accounts page is visually improved through real channel avatars and a clear source hierarchy.
- Every official YouTube source mark in the authenticated Dashboard is rendered by the single approved source-link component.
- Every official source mark is unmodified, large enough, fully visible, keyboard accessible, and linked to a permitted YouTube destination.
- No official YouTube source mark appears next to UniPost-generated health, counts, capabilities, scheduling, profile grouping, or operational controls.
- All original audit-equivalent screens clearly distinguish YouTube-origin data from UniPost-owned information.
- Missing IDs, broken avatars, disconnected accounts, and missing result URLs degrade without false YouTube attribution.
- Focused tests, the Dashboard build, required regression, Preview Acceptance, and real development acceptance all pass on the exact applicable SHAs.
- The audit evidence package is complete and contains no unapproved private user data.
