# Publish GIFs Guidance Design

## Goal

Add a task-oriented documentation page that explains how to publish GIF posts to X and Facebook through UniPost.

The page must also give readers an accurate platform-wide status summary:

- X and Facebook support direct GIF publishing through UniPost today.
- LinkedIn and Threads have official upstream GIF capabilities, but UniPost support is coming soon.
- Instagram, TikTok, Pinterest, YouTube, and Bluesky do not have a direct GIF file publishing path that UniPost exposes. UniPost will offer a GIF-to-MP4 conversion option in a future feature.

This task documents the current and planned behavior only. It does not implement GIF-to-MP4 conversion or add GIF publishing to another platform.

## Audience

The primary audience is a developer who:

- already has or plans to connect X and Facebook accounts to UniPost;
- has a hosted GIF URL or a local `.gif` file;
- wants an end-to-end API workflow rather than an endpoint contract alone;
- needs to understand why the same GIF cannot currently be sent unchanged to every UniPost destination.

## Route and Navigation

Create a new Publishing Guide at:

`/docs/guides/publish-gifs`

Add the guide to:

- the Guides index;
- the Publishing Guides section in the documentation sidebar;
- related-guide links shown on the API Reference pages used by the workflow.

Use the visible title:

`Publish GIFs to X and Facebook`

Use a short navigation label:

`Publish GIFs`

## Page Structure

### 1. Platform support matrix

Place the matrix immediately below the page introduction so readers can determine platform support before reading the workflow.

Use these columns:

| Column | Meaning |
| --- | --- |
| Platform | UniPost destination |
| Official GIF support | Whether the upstream publishing API has a GIF publishing capability |
| UniPost status | `Supported` or `Coming soon` |
| Recommended action | What the integrator should do today or expect from the planned feature |

Use these rows:

| Platform | Official GIF support | UniPost status | Recommended action |
| --- | --- | --- | --- |
| X / Twitter | Yes, direct GIF media upload | Supported | Publish the GIF directly |
| Facebook Page | Yes, GIF photo post | Supported | Publish the GIF directly |
| LinkedIn | Yes, through LinkedIn image APIs | Coming soon | Wait for native UniPost GIF support |
| Threads | Yes, through provider-backed GIF attachments | Coming soon | Wait for UniPost GIF attachment support |
| Instagram | No direct GIF publishing surface | Coming soon | Convert the GIF to MP4 through the planned UniPost conversion option |
| TikTok | No direct GIF publishing surface | Coming soon | Convert the GIF to MP4 through the planned UniPost conversion option |
| Pinterest | No direct animated GIF publishing surface in the supported organic Pin API flow | Coming soon | Convert the GIF to MP4 through the planned UniPost conversion option |
| YouTube | No GIF post type; publishing requires video media | Coming soon | Convert the GIF to MP4 through the planned UniPost conversion option |
| Bluesky | No stable direct GIF file publishing path exposed by UniPost | Coming soon | Convert the GIF to MP4 through the planned UniPost conversion option |

The matrix must avoid claiming that every upstream platform rejects GIFs. LinkedIn and Threads are explicitly identified as UniPost integration gaps. The other five platforms are described as conversion candidates rather than direct GIF destinations.

### 2. Current supported workflow

State that direct GIF publishing currently targets X and Facebook Page accounts.

Explain the two accepted media sources:

1. A publicly accessible GIF URL in `platform_posts[].media_urls`.
2. A local GIF uploaded to the UniPost media library and referenced through `platform_posts[].media_ids`.

Recommend the `platform_posts[]` request shape for both single-platform and multi-platform publishing.

### 3. Publish a hosted GIF

Provide a copyable example that publishes one publicly hosted GIF to both an X account and a Facebook Page account in one `POST /v1/posts` request.

The two destination entries should share the same GIF URL while retaining independent captions and account IDs.

The example must:

- use `https://api.unipost.dev`;
- use `$UNIPOST_API_KEY`;
- use placeholder account IDs for X and Facebook;
- include only one GIF per destination;
- avoid Facebook link options and other media;
- avoid scheduling so the example works with Facebook media publishing.

### 4. Publish a local GIF

Document the complete workflow:

1. List or identify the connected destination accounts.
2. Reserve the media upload with `POST /v1/media`, using `content_type: "image/gif"`.
3. PUT the raw GIF bytes to the returned `upload_url` with `Content-Type: image/gif`.
4. Poll `GET /v1/media/:media_id` until the status is `uploaded` or `attached`.
5. Optionally validate the final destination payload with `POST /v1/posts/validate`.
6. Publish with `POST /v1/posts` and the returned `media_id`.
7. Poll `GET /v1/posts/:post_id` until each destination result is final.

Provide one complete cURL workflow. It may use `jq` to extract `media_id`, `upload_url`, and the created post ID.

The workflow should reuse one uploaded UniPost media ID for both destination entries.

### 5. Platform-specific limits

Show a compact comparison:

| Rule | X / Twitter | Facebook Page |
| --- | --- | --- |
| GIF count | Exactly one GIF | Exactly one GIF |
| UniPost file-size cap | 5 MB | 10 MB |
| Mixed media | Do not combine the GIF with images or video | Do not combine the GIF with other media |
| Links | Caption URLs follow X post behavior | Do not combine Facebook link options with media |
| Scheduling | UniPost scheduling is supported | Scheduled Facebook media publishing is not currently supported |

Clarify that one cross-platform request must satisfy the strictest shared file-size limit. A GIF sent to both X and Facebook must therefore be 5 MB or smaller.

### 6. Coming-soon behavior

Explain two distinct roadmap paths:

- LinkedIn and Threads: future native integration with their upstream GIF capabilities.
- Instagram, TikTok, Pinterest, YouTube, and Bluesky: a future UniPost option that converts a GIF to MP4 before publishing to a video-capable destination.

Do not promise an API field name, delivery date, pricing behavior, media-retention policy, conversion quality, or supported conversion limits. Those decisions belong in the follow-up GIF-to-MP4 PRD.

### 7. Common errors

Include the likely UniPost validation or lifecycle failures:

| Error or state | Meaning | Resolution |
| --- | --- | --- |
| `unsupported_format` | GIF was targeted to a platform that UniPost does not currently support for direct GIF publishing | Target X or Facebook, or wait for the documented coming-soon path |
| `file_too_large` | The GIF exceeds the destination limit | Reduce the GIF below the platform limit; use 5 MB or less when targeting both X and Facebook |
| `media_not_uploaded` | A local media upload is still pending | PUT the bytes and poll the media record until it is ready |
| `mixed_media_unsupported` | GIF was combined with another media kind | Send the GIF as the only media item |
| Facebook scheduled-media validation failure | Facebook media was submitted with `scheduled_at` | Publish the Facebook GIF immediately |
| Asynchronous destination failure | UniPost accepted the post, but an upstream platform later rejected delivery | Poll the post result and inspect the per-destination error details |

Use the actual error code already exposed by UniPost where a stable code exists. For the Facebook scheduling row, use the current documented code if the implementation exposes it consistently; otherwise describe it without inventing a new code.

### 8. API Reference links

Link inline at the step where each endpoint is first used:

- `GET /v1/accounts`
- `POST /v1/media`
- `GET /v1/media/:media_id`
- `POST /v1/posts/validate`
- `POST /v1/posts`
- `GET /v1/posts/:post_id`

End the page with reference cards for:

- List accounts
- Reserve media upload
- Get media
- Validate post
- Create post
- Get post

Also link to the X and Facebook platform documentation pages for complete platform specifications.

## API Reference Backlinks

Add `Publish GIFs` as a related guide on these endpoint reference pages:

- `GET /v1/accounts`
- `POST /v1/media`
- `GET /v1/media/:media_id`
- `POST /v1/posts/validate`
- `POST /v1/posts`
- `GET /v1/posts/:post_id`

Use the shared endpoint-to-guide mapping in the single-endpoint page component where possible, rather than adding bespoke markup to every endpoint page.

Existing related guides must remain present.

## Guides Index and Sidebar

Add a Guides index card with:

- title: `Publish GIFs`
- description: `Publish a hosted or local GIF to X and Facebook, compare platform support, and prepare for upcoming conversion workflows.`

Add `Publish GIFs` to the Publishing Guides sidebar group near the other publishing workflow guides.

Update the Guides index publishing-workflows copy so the new guide is discoverable from task language such as “publish a GIF.”

## Presentation and Components

Follow existing documentation patterns:

- `DocsPage` for the page shell;
- `DocsTable` for support and limits matrices;
- `DocsCodeTabs` for code examples;
- `ApiInlineLink` for endpoint references;
- existing callout and next-card classes for status notes and final reference links.

No new visual component or CSS system is required. The support matrix is the primary visual summary.

## Testing

Add a source-level documentation regression test before implementing the page. The test should fail until all required surfaces exist.

Verify at least:

- the route file exists and uses the approved page title;
- the top-level matrix contains all nine platforms;
- X and Facebook are marked supported;
- LinkedIn and Threads are marked coming soon without being described as officially unsupported;
- the five conversion destinations mention GIF-to-MP4 conversion as coming soon;
- the page links all six workflow API Reference pages;
- Guides index and sidebar include the new route;
- related-guide mapping adds the guide to all six API endpoints without removing existing guides;
- examples use `platform_posts[]`, `image/gif`, and both an X and Facebook account;
- the cross-platform 5 MB effective limit and Facebook immediate-publish constraint are documented.

Run the targeted source test first, then the dashboard build. Run the dashboard regression suite when Playwright browsers are available, because the docs shell and navigation are changed.

## Acceptance Criteria

The task is complete when:

1. `/docs/guides/publish-gifs` is reachable from the Guides index and sidebar.
2. The support matrix accurately distinguishes official platform capability from UniPost implementation status.
3. A developer can copy either the hosted-URL or local-file workflow to publish one GIF to X and Facebook.
4. All six used API Reference pages link back to the guide.
5. The page does not claim that GIF-to-MP4 conversion already exists.
6. Local dashboard validation passes.
7. The change is merged into local `dev`, pushed to `origin/dev`, all triggered checks and deployments pass, and the live development documentation matches this design.

## Follow-up

After this Guidance is complete, create a separate PRD for the GIF-to-MP4 conversion feature. That PRD must define the product surface, API contract, conversion lifecycle, limits, storage behavior, failure handling, cost or quota implications, and destination-specific video validation. The conversion feature is not part of this Guidance implementation.
