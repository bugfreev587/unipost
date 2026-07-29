# YouTube Source Identity System Design — A2

## Decision

Authenticated UniPost Dashboard surfaces will not render an official YouTube logo or icon.

The approved A2 system uses:

- the real channel avatar as the primary YouTube account identity;
- a keyboard-accessible text link such as `Source: YouTube`, `Data from YouTube`, or `View on YouTube` for source attribution;
- a refined neutral video/play glyph for UniPost-owned actions, status, counts, navigation, and workflow destinations;
- explicit visual and textual separation between YouTube-origin identity/data and UniPost-managed state.

This A2 decision supersedes the earlier proposal to place a 24px official YouTube icon in a 44px link target.

## Why the design changed

The supplied June 1, 2026 audit cited YouTube Developer Policies III.F.2a and III.F.2b: UniPost did not clearly differentiate content that did not originate from YouTube, causing source confusion. The violation was not simply an incorrect icon URL.

During implementation planning on July 29, 2026, the current official YouTube brand site was checked. Its YouTube Icon guidance states that digital icon height must not be smaller than 100px. The YouTube API Branding Guidelines require clients to follow the current brand-site minimum-size rules.

A 100px official icon is incompatible with the approved compact Accounts, Analytics, Inbox, and Admin layouts. Shrinking it would violate the current size rule; rendering it at 100px would make it disproportionately prominent and conflict with the requirement that YouTube branding not be the page's most prominent element. The product owner therefore approved A2: no official graphic in authenticated Dashboard UI.

Authoritative references:

- https://developers.google.com/youtube/terms/developer-policies
- https://developers.google.com/youtube/terms/branding-guidelines
- https://brand.youtube/youtube-icon

## Scope

### In scope

- Accounts and connected-channel identity.
- Developer App managed-user details and aggregate counts.
- YouTube Analytics, Analytics aggregates, post rows, and result cards.
- Inbox conversation rows and original-content context.
- Admin user/platform summaries.
- Create Post, Calendar, Connection Stats, capability summaries, platform counts, filters, and publishing destinations.
- Published YouTube results with validated YouTube URLs.
- Static compliance guards, focused tests, CI, browser acceptance, and audit evidence.

### Non-goals

- No YouTube OAuth, API, scope, token, publishing, database, or API-response changes.
- No feature flag.
- No redesign of other platform icons.
- No public marketing, blog, documentation, or public-tool visual change. Those `PlatformIcon` consumers remain an inventoried, uncertified follow-up risk.
- No staging or production promotion unless explicitly requested later.

## Verified current state

- `AccountDestinationIcon` currently uses Lucide `Video` for YouTube.
- `PlatformIcon` contains an inline red YouTube graphic with `fill="#ff0000"`.
- The current compliance test covers named operational files only; it does not cover Analytics, Inbox, or Admin.
- Authenticated Analytics, Inbox, and Admin contain `PlatformIcon` expressions that can render the red YouTube graphic.
- Accounts and managed-user aggregate surfaces already use neutral `AccountDestinationIcon`, but concrete channel rows do not yet show channel avatars or linked source text.
- The only direct YouTube channel URL constructor is in `youtube-analytics-view.tsx` and does not reject disconnected sentinels.
- Disconnect stores `external_account_id = 'disconnected:' || id`, clears `account_name` and `account_avatar_url`, and sets `status = 'disconnected'`.

## Semantic usage model

### Concrete YouTube channel identity

When a concrete `SocialAccount` represents a YouTube channel, render `YouTubeChannelIdentity`:

- 40px standard or 32px compact channel avatar as the primary visual;
- neutral initials fallback when the avatar is missing or broken;
- neutral destination glyph fallback when both avatar and name are missing;
- channel name;
- text source link `Source: YouTube` or `Data from YouTube`.

The identity component does not render health, status, selection, disconnect, capability, or editing controls. Those remain in a separate cell or labeled region.

### UniPost operational destination

Use the neutral rounded-video-frame/play glyph for:

- Connect and Create Post controls;
- Calendar, filters, scheduled posts, and destination chips;
- health, counts, capabilities, profile grouping, and aggregate badges;
- Analytics navigation, aggregate platform tables, post rows, and result-status clusters;
- Inbox list rows, unread and thread status, and original-post status metadata;
- Admin user platform summaries;
- disconnected-account states.

The glyph uses `currentColor` and never YouTube red, an official path, or official trade dress.

### YouTube-origin content and data

Use a text source link only when the destination can be validated:

- channel identity: exact channel page when a real channel ID exists;
- active account missing a valid channel ID: YouTube home page with accessible label `Open YouTube` and a non-sensitive monitoring warning;
- published content: exact validated YouTube/YouTu.be URL;
- Inbox context: exact validated YouTube URL when the API item provides one.

No valid URL means no `View on YouTube` link. The neutral operational glyph and visible source copy remain.

## Surface matrix

| Authenticated surface | Visual treatment | Source-link behavior | UniPost-state separation |
| --- | --- | --- | --- |
| Accounts concrete YouTube account | Avatar + channel name + `Source: YouTube` text link | Exact channel, or home fallback for an active missing ID | Status remains in `UniPost status` column |
| Managed-user concrete YouTube account | Avatar + channel name + `Source: YouTube` text link | Exact channel, or home fallback | Status and disconnect/dismiss remain sibling regions |
| Managed-user aggregate counts | Neutral destination glyph | No source link | Counts remain UniPost-generated |
| Connect/Create Post/Calendar | Neutral destination glyph | No source link | Action/capability/schedule remains UniPost-owned |
| Analytics navigation and aggregates | Neutral destination glyph | No source link | Navigation/counts remain UniPost-generated |
| YouTube Analytics concrete channel | Avatar + `Data from YouTube` text link | Exact channel, or home fallback | Readiness/freshness/reconnect state remains separate |
| Published YouTube result with valid URL | Neutral result/status glyph + separate `View on YouTube` text link | Exact validated content URL | Link is outside the status cluster |
| Result without valid URL | Neutral destination glyph | No link | No false source claim |
| Inbox YouTube conversation row | Neutral destination glyph + visible `Source: YouTube` | No row-level source link | Unread/thread status remains separate |
| Inbox content context with valid URL | Neutral context glyph + separate `View on YouTube` text link | Exact validated content URL | Link is outside Resolve/Reply/status controls |
| Admin user/platform summaries | Neutral destination glyph | No source link | Membership and status remain UniPost-managed |

## Component architecture

### `youtube-source.ts`

Provides two pure policies:

- `buildYouTubeChannelUrl` trims and encodes a valid channel ID and rejects empty IDs or `disconnected:` sentinels;
- `normalizeYouTubeContentUrl` accepts HTTPS URLs only for `youtube.com`, `www.youtube.com`, `m.youtube.com`, and `youtu.be`.

### `YouTubeSourceLink`

This component renders no YouTube graphic. It owns authenticated YouTube source-link behavior:

- visible text disclosure supplied from a closed union: `Source: YouTube`, `Data from YouTube`, or `View on YouTube`;
- a neutral `ExternalLink` glyph using `currentColor`;
- at least a 44px interactive height;
- validated YouTube destination only;
- `target="_blank"` and `rel="noopener noreferrer"`;
- action-oriented accessible label;
- visible light/dark focus state that does not depend on red.

### `YouTubeChannelIdentity`

Inputs are `id`, `account_name`, `account_avatar_url`, `external_account_id`, and `status` from `SocialAccount`, plus density and disclosure.

It renders avatar/fallback, name, and `YouTubeSourceLink`. It never renders UniPost health or controls. Disconnected accounts render `Disconnected YouTube channel`, no source link, and a neutral fallback.

### `AccountDestinationIcon`

For YouTube, replace Lucide `Video` with a small internal outline SVG: rounded video frame plus inset play triangle. It remains decorative when adjacent text supplies the accessible name. Other platform behavior remains unchanged.

## Data and fallback rules

- Valid active channel ID: exact channel text link.
- Active missing/invalid channel ID: home-page text link labeled `Open YouTube`; warn once per non-sensitive UniPost account ID.
- Missing/broken avatar: initials; missing name too: neutral destination glyph.
- Disconnected sentinel/status: no source link, no channel URL, no official or text source action.
- Invalid/missing published-result or Inbox URL: no `View on YouTube` link.
- Never construct `youtube.com/channel/disconnected:...`.

## Visual and accessibility requirements

- Avatar remains the strongest account-identity visual.
- Source text uses existing muted small-text tokens; no red brand background, glow, or badge.
- Neutral external-link glyph is 14px and subordinate to the source text.
- Source link has at least 44px interactive height without forcing a 44px graphic.
- Focus ring has sufficient light/dark contrast and does not depend on red.
- Mobile width 390px has no horizontal overflow.
- Source identity and `UniPost-managed` state use separate cells or labeled regions.
- Visible text, not color or an icon alone, communicates source.

## Automated guardrails

The updated compliance suite must:

1. Scan `src/app/(dashboard)`, `src/app/admin`, and shared authenticated components.
2. Reject official red variants, the legacy YouTube SVG path/signature, copied official artwork, and any authenticated reference to a YouTube brand asset.
3. Reject authenticated literal YouTube `PlatformIcon` calls and dynamic `PlatformIcon` calls in Dashboard/Admin routes.
4. Assert `YouTubeSourceLink` contains no image, official path, official fill, or brand-asset reference.
5. Assert every source link validates the destination, renders a real anchor, uses the required target/rel/accessibility contract, and shows approved text.
6. Assert operational surfaces use `AccountDestinationIcon` and concrete identity surfaces use `YouTubeChannelIdentity`.
7. Cover valid IDs, empty IDs, sentinels, URL hosts, broken avatars, missing names, disconnected state, and invalid result URLs.
8. Explicitly cover Analytics, Inbox, Admin, Accounts, managed-user detail, Create Post, Calendar, Connection Stats, capabilities, counts, and published results.
9. Inventory public `PlatformIcon` consumers as out of scope and never describe them as certified.

## Verification and release

Local implementation verification:

- focused URL/source/compliance tests;
- existing Analytics source tests;
- `npm run build`;
- `npm run test:regression:dashboard`;
- authenticated fixture-backed browser acceptance when required credentials are available.

The task branch must be pushed and opened as a Draft PR to `dev`. GitHub CI, Vercel Preview, Railway PR Environment, deployed regression, and Codex browser acceptance must all succeed on the exact head SHA before merge. After merge, wait for the persistent development deployments and repeat acceptance on `https://dev-app.unipost.dev` with `https://dev-api.unipost.dev`.

Do not create a staging or production PR.

## Definition of done

- No authenticated Dashboard surface can render the legacy red YouTube `PlatformIcon` or any other official YouTube graphic.
- Accounts and managed-user detail use avatar-first channel identity with a compliant text source link.
- Analytics, Inbox, Admin, Create Post, Calendar, health, counts, and capabilities use neutral operational treatment.
- Valid published results and content contexts use separate validated `View on YouTube` text links; invalid URLs produce no link.
- Disconnected sentinels never become channel URLs.
- Source identity and UniPost status remain visibly and semantically separate.
- Static guards fail on official red/path/asset regressions and unclassified authenticated `PlatformIcon` use.
- Focused tests, build, regressions, Preview Acceptance, and real dev acceptance pass on their exact SHAs.
- The evidence package records the official 100px constraint, the A2 decision, screenshots, URLs, SHAs, and the out-of-scope public inventory without private user data.
