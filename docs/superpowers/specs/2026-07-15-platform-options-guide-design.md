# Platform Options Examples Guide Design

## Problem

The Create Post API reference correctly describes `platform_posts[].platform_options` as a flat destination options object, but it does not show common platform-specific examples. The existing platform guides contain useful examples, yet several use the legacy `account_ids` request shape with nested `platform_options.<platform>` objects. Developers can therefore miss required options, assume the wrong defaults, or copy a legacy nested object into the recommended `platform_posts[]` shape.

The recent YouTube support case demonstrates the consequence: the caller omitted `privacy_status`, so the API used its `private` default even though the source video otherwise qualified as a YouTube Short.

## Outcome

Create one task-focused guidance page at `/docs/guides/platform-options` that gives copyable, recommended `platform_posts[]` examples for the five platforms whose option shapes are most error-prone:

- YouTube
- Instagram
- TikTok
- Facebook
- Pinterest

Add a direct link to this guide in the `platform_posts[].platform_options` request field on the Create Post API reference. Also expose the guide through the Guides index, documentation sidebar, and documentation search index.

## Non-goals

- Do not change API behavior, validation, defaults, or SDK types.
- Do not add large platform-specific examples directly to the Create Post API reference.
- Do not document every supported platform in this first version.
- Do not make the legacy `account_ids` request shape the primary workflow.
- Do not introduce a new visual system or new third-party dependencies.

## Information Architecture

### Route and navigation

- Route: `/docs/guides/platform-options`
- Page title: `Platform options examples`
- Eyebrow: `Publishing Guides`
- Add a `Platform options examples` card to `/docs/guides`.
- Add a `Platform options examples` item to the `Publishing Guides` sidebar group.
- Add a search-index entry that answers queries about `platform_options`, YouTube Shorts visibility, Instagram Stories and Reels, TikTok privacy controls, Facebook Reels, and Pinterest boards.

### Create Post API reference link

Keep the `platform_posts[].platform_options` field concise. Extend its description with a direct link to `/docs/guides/platform-options`, identifying the covered platforms. The API reference remains an endpoint contract; the guide owns the copyable examples and troubleshooting detail.

## Page Structure

### 1. Request-shape callout

Open with a prominent callout explaining:

- In the recommended `platform_posts[]` shape, each destination is already platform-scoped.
- `platform_posts[].platform_options` must therefore be a flat object.
- `platform_posts[].platform_options.youtube`, `.instagram`, or another nested platform key is invalid.
- The nested `platform_options.<platform>` form belongs only to the legacy top-level `account_ids` request shape.

Include a compact comparison table and code tabs for:

- Recommended flat shape
- Legacy nested shape
- Invalid mixed shape

### 2. Validate-first workflow

Recommend sending the exact payload to `POST /v1/posts/validate` before publishing. Explain the distinction between a valid request that returns `data.valid: false` and a malformed mixed request that returns HTTP 422.

### 3. YouTube examples

Cover:

- Public long-form video
- Public YouTube Short
- Native YouTube scheduled publication using `privacy_status: "private"` with `publish_at`

State explicitly:

- API requests default to `private` when `privacy_status` is omitted.
- The Dashboard defaults to `public`; this does not change the API default.
- Callers that expect a publicly visible video or Short must send `privacy_status: "public"`.
- `title` and `made_for_kids` are required.
- `shorts: true` adds the Shorts hint used by the current adapter; it does not resize, crop, or guarantee classification.
- YouTube automatically classifies eligible square or vertical videos of no more than three minutes as Shorts.

### 4. Instagram examples

Cover Feed, Reels, and Stories using flat `mediaType` values. Emphasize that `platform_options.instagram.mediaType` is invalid inside `platform_posts[]`, and that Stories accept exactly one media asset.

### 5. TikTok examples

Cover:

- Required privacy selection
- Comment, duet, and stitch controls
- Commercial disclosure fields

Use provider-compatible option names and explain that interaction settings should be chosen explicitly rather than assumed.

### 6. Facebook examples

Cover:

- Feed media post
- Reel video
- Link-only Feed post

Explain that Reels require video and cannot be combined with a link attachment.

### 7. Pinterest examples

Cover the required `board_id` plus optional `title` and `link`. Explain that `board_id` must identify the destination board and that Pinterest publishing requires media.

### 8. Common mistakes and reference links

End with a table mapping common mistakes to fixes, including:

- Nested platform key inside `platform_posts[].platform_options`
- Missing YouTube `privacy_status`
- Missing YouTube `made_for_kids`
- Missing TikTok privacy selection
- Instagram Story with multiple media assets
- Facebook Reel with link attachment
- Missing Pinterest `board_id`

Link to the Create Post reference, Validate Post reference, Publishing guide, and each covered platform guide.

## Presentation

Use existing documentation components and styles:

- `DocsPage` for the page shell
- `DocsCodeTabs` for copyable JSON examples
- `DocsTable` for request-shape and troubleshooting comparisons
- Existing docs callout classes for warnings and recommendations
- Existing next-step cards for reference links

The page should be static and server-rendered. It requires no new client state, animation, icon package, or dependency.

## Testing

Add a focused Node source regression test that fails before the guide exists and verifies:

- The new guide route and title exist.
- The request-shape callout distinguishes flat, legacy, and invalid mixed shapes.
- All five platforms have copyable examples.
- YouTube documents the API `private` default, explicit `public` resolution, Shorts eligibility, and scheduled-private exception.
- The Guides index links to the page.
- The Publishing Guides sidebar links to the page.
- The Create Post request field links to the page.
- The documentation search index contains the new guide entry and key discovery terms.

Run the focused test first for the red-green cycle, then the relevant documentation source suite, `npm run build`, and the dashboard Playwright regression suite when browsers are available.

## Acceptance Criteria

- A developer reading the Create Post request fields can reach the guide directly.
- A developer can copy a correct flat `platform_posts[].platform_options` example for each covered platform.
- The guide makes it impossible to reasonably infer that omitted YouTube visibility becomes public.
- The guide clearly distinguishes recommended, legacy, and invalid mixed request shapes.
- The YouTube section explains how to publish a public Short without implying that `shorts: true` converts the media.
- The page is discoverable from Guides navigation and documentation search.
- Existing documentation pages continue to build and pass regression checks.
