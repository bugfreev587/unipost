# PRD — Pinterest publishing (Phase 1)

**Status:** Planning  
**Owner:** TBD  
**Target:** Pinterest GA milestone  
**Created:** 2026-04-26

---

## Problem

UniPost currently supports seven public publishing platforms in the product surface and capability map:

- X / Twitter
- LinkedIn
- Bluesky
- Instagram
- Threads
- TikTok
- YouTube

Pinterest is already listed as `coming` in the dashboard competitor data, but UniPost does not yet support:

- connecting Pinterest accounts
- selecting a Pinterest board as the publish target
- creating Pins through the publish API
- showing Pinterest in the dashboard composer, posts list, or docs

This leaves a visible platform-gap versus competitors and blocks a meaningful customer segment:

- ecommerce brands
- creators who repurpose visual content
- agencies that need evergreen traffic channels
- SaaS users comparing "how many platforms are supported"

Pinterest is a good next platform because its official developer platform explicitly supports creating and managing Pins and Boards, including Standard Pins and Video Pins, and Pinterest's developer guidelines explicitly allow scheduling / content-marketing style integrations when the user chooses each action.

## Goals

1. Ship Pinterest as a first-class publish target in UniPost.
2. Support the core scheduling use case: user connects a Pinterest account, picks a board, and publishes a Pin with caption + media + optional destination link.
3. Reuse UniPost's existing `platform_posts[]`, media library, scheduler, queue, and result model instead of adding a Pinterest-specific write path.
4. Expose Pinterest through the same public capability and validation surfaces as the other platforms.
5. Keep the first release narrow enough to launch quickly and reliably.

## Non-goals

- Pinterest Shopping / catalogs
- ads management
- Idea Pins, Product Pins, or Collections
- board creation from UniPost
- analytics parity on day 1
- comment / inbox support
- bulk board sync beyond the connected account's own boards
- per-Pin advanced metadata beyond the minimal publish fields

## Product decision

Phase 1 should ship **Standard Pins and Video Pins only**.

This means:

- UniPost treats Pinterest as a visual publish surface, not a full Pinterest management suite.
- Every Pinterest post requires media.
- Every Pinterest post targets exactly one board.
- One UniPost `platform_post` becomes one Pinterest Pin.
- The board is selected at compose time or supplied via `platform_options.pinterest.board_id`.

This is the smallest release that:

- closes the visible competitive gap
- fits UniPost's current post/result model
- serves the most common customer workflow
- avoids Pinterest Shopping and catalog complexity

## User stories

1. As a marketer, I can connect a Pinterest account to UniPost using OAuth.
2. As a marketer, I can choose one of my Pinterest boards before publishing.
3. As a marketer, I can publish an image Pin with a title, description, and destination URL.
4. As a marketer, I can publish a video Pin with a title, description, and destination URL.
5. As a scheduler user, I can schedule the Pin through UniPost's scheduler just like other platforms.
6. As an API user, I can publish to Pinterest through `platform_posts[]` without a separate Pinterest-only endpoint.
7. As a user, I get clear validation errors when media is missing, board selection is missing, or the media mix is unsupported.

## Current codebase fit

Pinterest fits the current architecture well:

- OAuth/connect flow already exists per platform in [api/internal/handler/oauth.go](/Users/xiaoboyu/unipost/api/internal/handler/oauth.go).
- Publish adapters plug into the existing registry in [api/internal/platform](/Users/xiaoboyu/unipost/api/internal/platform).
- Static capability metadata already ships from [api/internal/platform/capabilities.go](/Users/xiaoboyu/unipost/api/internal/platform/capabilities.go) via `GET /v1/platforms/capabilities`.
- Validation already supports platform-specific media rules in [api/internal/platform/validate.go](/Users/xiaoboyu/unipost/api/internal/platform/validate.go).
- The product already accepts per-platform options through `platform_posts[].platform_options`.

So Pinterest does not need a new publish architecture. It needs:

- a new connector
- a new adapter
- Pinterest capability metadata
- Pinterest-specific validation
- dashboard composer and account UI wiring
- docs and rollout support

## Scope

### In scope for Phase 1

- OAuth connection for Pinterest
- board listing for connected accounts
- publish Standard Pins with one image
- publish Video Pins with one video
- optional title
- caption / description
- optional destination link
- scheduling through UniPost's existing scheduler
- public capability metadata
- validation endpoint support
- dashboard composer support
- docs updates

### Out of scope for Phase 1

- multi-image carousels
- mixed media posts
- Idea Pins
- Product Pins
- Collections
- analytics ingestion
- board creation / editing
- inbox / comments / replies
- first comment equivalent
- thread support

## Official platform assumptions

This PRD assumes the current official Pinterest platform position as of April 26, 2026:

- Pinterest's developer platform publicly supports creating and managing organic content, including Pins and Boards.
- Pinterest's content use-case page explicitly lists Standard Pins and Video Pins among supported creation surfaces.
- Pinterest's developer guidelines allow scheduling / publishing integrations, but require that the user explicitly chooses each action and that the integration does not automate engagement or act without user intent.

If any of these assumptions change during implementation, update this PRD and the capability map before launch.

## API surface

### Final request shape

Pinterest should use the existing `platform_posts[]` contract.

```json
{
  "platform_posts": [
    {
      "account_id": "sa_pinterest_1",
      "caption": "5 desk setup ideas for your home office",
      "media_ids": ["med_pinterest_1"],
      "platform_options": {
        "pinterest": {
          "board_id": "987654321",
          "title": "Home Office Desk Setup Ideas",
          "link": "https://example.com/blog/desk-setup"
        }
      }
    }
  ],
  "scheduled_at": "2026-05-01T18:00:00Z"
}
```

### Field behavior

- `caption` maps to the Pin description.
- `platform_options.pinterest.title` is optional, but strongly recommended.
- `platform_options.pinterest.link` is optional.
- `platform_options.pinterest.board_id` is required.
- exactly one media item is required
- media must be either one image or one video
- no text-only Pinterest posts
- no multi-item carousels in Phase 1

### Platform option schema

```json
{
  "pinterest": {
    "board_id": "string, required",
    "title": "string, optional",
    "link": "string, optional"
  }
}
```

Possible future fields, intentionally not in Phase 1:

- `alt_text`
- `dominant_color`
- `parent_pin_id`
- shopping / product metadata

## Capability map changes

Add `pinterest` to [api/internal/platform/capabilities.go](/Users/xiaoboyu/unipost/api/internal/platform/capabilities.go).

Recommended Phase 1 capability shape:

```go
"pinterest": {
    DisplayName: "Pinterest",
    Text: TextCapability{
        MaxLength: 800,
        MinLength: 0,
        Required:  false,
        SupportsThreads: false,
    },
    Media: MediaCapability{
        RequiresMedia: true,
        AllowMixed:    false,
        Images: ImageCapability{
            MaxCount: 1,
            AllowedFormats: []string{"jpg", "jpeg", "png", "webp"},
        },
        Videos: VideoCapability{
            MaxCount: 1,
            AllowedFormats: []string{"mp4", "mov"},
        },
    },
    Thread:       ThreadCapability{Supported: false},
    Scheduling:   SchedulingCapability{Supported: true},
    FirstComment: FirstCommentCapability{Supported: false},
}
```

Notes:

- The exact length and format limits must be confirmed against current Pinterest docs during implementation.
- UniPost's scheduler should remain `supported=true` even if Pinterest does not offer native future scheduling through the same API shape, because UniPost already owns the scheduling layer.

## Validation rules

Add Pinterest-specific validation in [api/internal/platform/validate.go](/Users/xiaoboyu/unipost/api/internal/platform/validate.go).

### Required checks

1. `board_id` is required for every Pinterest post.
2. Exactly one media item is required.
3. Caption-only posts are rejected.
4. Mixed media is rejected.
5. More than one image is rejected in Phase 1.
6. More than one video is rejected.
7. `in_reply_to` is rejected because Pinterest threads are unsupported.
8. `first_comment` is rejected because Pinterest has no matching UniPost behavior.
9. Unsupported image/video formats are rejected using the static capability map.
10. Invalid `link` must return a clear validation error if we already validate URL shape elsewhere.

### New validation codes

Recommended new error codes:

- `pinterest_board_required`
- `pinterest_requires_media`
- `pinterest_single_media_only`
- `pinterest_mixed_media_unsupported`
- `invalid_pinterest_link`

## OAuth and account connection

Pinterest needs a new connector following the same pattern as Instagram / Threads / LinkedIn.

### New connector

Add:

- [api/internal/connect/pinterest.go](/Users/xiaoboyu/unipost/api/internal/connect/pinterest.go)

Responsibilities:

- build the Pinterest authorize URL
- exchange authorization code for tokens
- fetch the user's Pinterest account profile
- persist refresh/access token material in the existing `social_accounts` model

### Account metadata

Persist enough account metadata to support UI and board fetching:

- external account ID
- display name
- avatar URL if available
- raw metadata JSON for future board/account details

### Board selection requirement

Pinterest differs from most current UniPost platforms because the target destination is not just the account; it is a board within the account.

So Phase 1 needs a board-selection surface:

- at compose time in the dashboard
- in API requests through `platform_options.pinterest.board_id`

### Board list endpoint

Add a lightweight endpoint to list boards for a connected Pinterest account.

Recommended shape:

`GET /v1/social-accounts/{id}/pinterest/boards`

Response:

```json
{
  "boards": [
    { "id": "123", "name": "Home Decor" },
    { "id": "456", "name": "Spring Launch" }
  ]
}
```

This should be a read-through API call to Pinterest rather than a permanently mirrored local table in Phase 1.

## Adapter implementation

Add a new adapter:

- [api/internal/platform/pinterest.go](/Users/xiaoboyu/unipost/api/internal/platform/pinterest.go)

### Adapter responsibilities

1. Accept UniPost `caption`, media, and `platform_options.pinterest`.
2. Validate the final media set defensively before making the Pinterest call.
3. Create an image Pin or video Pin through Pinterest's content creation APIs.
4. Return a standard `platform.PostResult`.
5. Populate the public Pin URL when Pinterest returns enough information to construct it.

### Phase 1 publish matrix

- One image + optional title + optional link: supported
- One video + optional title + optional link: supported
- Text only: rejected
- Multiple images: rejected
- Multiple videos: rejected
- Mixed image + video: rejected

### Async behavior

If Pinterest video creation is asynchronous, the adapter should reuse UniPost's existing pattern:

- return `Status="processing"` when the post is accepted but not yet fully ready
- let the status worker or poll-on-read path finalize the row later

If Pinterest responds synchronously for Phase 1 image Pins, those can remain immediate `published`.

## Worker and post status

If the Pinterest create flow for video is asynchronous, add a Pinterest status worker similar in spirit to the Facebook video worker, but only if needed by the actual API behavior.

Decision:

- image Pins should not add a new worker
- video Pins may add a worker if Pinterest requires polling

Do not add a Pinterest-specific worker until the real API requires it.

## Dashboard requirements

### Accounts

- Add Pinterest to the social-account connect list.
- Show connected Pinterest accounts with the same account-card UI as other platforms.

### Composer

- Add Pinterest as a selectable destination.
- When a Pinterest account is selected, require board selection.
- Show fields:
  - Board
  - Title
  - Destination link
- Reuse the main caption box for description.
- Reuse existing media picker.

### Composer constraints

- Disable publish if no board is selected.
- Disable publish if no media is attached.
- Reject multi-media selection for Pinterest in Phase 1.
- If multiple platforms are selected and only Pinterest is invalid, show a scoped per-platform validation state instead of blocking the whole composer with an unclear generic message.

### Posts UI

- Show Pinterest platform badge anywhere other platforms are displayed.
- Show the selected board name if available in detail views.
- Preserve the Pin URL in the results UI when available.

## Docs requirements

Update:

- public API docs
- quickstart examples
- platform capabilities docs
- marketing surface that enumerates supported platforms
- competitor comparison pages that currently say Pinterest is `coming`

Documentation should clearly state:

- Pinterest requires media
- Pinterest requires a board
- Phase 1 supports only one image or one video per post

## Analytics

Phase 1 should not block on Pinterest analytics.

Decision:

- publish support ships first
- analytics is a follow-up PRD

If the adapter interface requires a no-op position, leave analytics unimplemented for Pinterest initially.

## Rollout plan

1. Land static capability entry and validation rules behind tests.
2. Ship OAuth connector and board-list endpoint in staging.
3. Ship Pinterest adapter for image Pins first.
4. Add video Pin support in the same milestone if API behavior is straightforward; otherwise gate video behind a feature flag and launch images first.
5. Wire dashboard composer and account UI.
6. Update docs and marketing pages.
7. Release to an internal workspace first.
8. Run live publish smoke tests against:
   - one image Pin
   - one video Pin
   - scheduled image Pin
   - invalid request with missing board
   - invalid request with missing media
9. Enable for all workspaces.

## Success criteria

This project is successful if:

1. A user can connect a Pinterest account in the dashboard.
2. A user can list their boards and choose one at publish time.
3. A user can publish a Standard Pin with one image.
4. A user can schedule that Pin through UniPost.
5. The API `platform_posts[]` flow works for Pinterest without introducing a Pinterest-only write endpoint.
6. Validation catches the most common Pinterest failures before dispatch.
7. The dashboard and marketing site can truthfully say UniPost supports Pinterest.

## Risks

1. Pinterest app review / access approval may slow launch even if the code is ready.
2. Board selection adds a new destination-selection concept that other platforms do not need.
3. Video Pin creation may turn out to be more asynchronous or operationally fragile than image Pins.
4. Exact content limits may drift, so the static capability map must be checked against current docs before release.
5. If Pinterest requires business-account constraints or scope approvals not captured in the current PRD, the connect flow may need a staged rollout.

## Open questions

1. Should Phase 1 launch with image Pins only if video introduces polling complexity?
2. Should UniPost require `title` for Pinterest even if Pinterest itself allows it to be optional, in order to improve post quality?
3. Do we want to expose board selection only in `platform_options.pinterest.board_id`, or also add a first-class compose field in the public API docs examples everywhere Pinterest appears?
4. Should Pinterest account metadata cache the latest fetched boards for UX speed, or is live fetch on compose good enough for Phase 1?
5. Do we want a feature flag such as `CONNECT_PINTEREST_ENABLED` for staged rollout, mirroring how other gated platforms are handled?

## Recommended implementation order

1. Connector + OAuth callback wiring
2. Board list read endpoint
3. Capability map entry
4. Validation rules + tests
5. Adapter for image Pins
6. Dashboard composer / account UI
7. Docs and marketing updates
8. Video Pin support if low-risk

## Sources

- [Pinterest Developers](https://developers.pinterest.com/)
- [Pinterest Content Use Case](https://developers.pinterest.com/usecase/content/)
- [Pinterest Developer Guidelines](https://policy.pinterest.com/developer-guidelines)
