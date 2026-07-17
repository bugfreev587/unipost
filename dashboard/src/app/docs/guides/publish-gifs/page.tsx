import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";
import { ApiInlineLink } from "../../api/_components/doc-components";

const HOSTED_GIF_SNIPPETS = [
  {
    label: "cURL",
    lang: "bash",
    code: `set -euo pipefail

GIF_PAYLOAD='{
  "platform_posts": [
    {
      "account_id": "sa_twitter_123",
      "caption": "The launch flow in 12 seconds.",
      "media_urls": ["https://cdn.example.com/launch.gif"]
    },
    {
      "account_id": "sa_facebook_123",
      "caption": "The launch flow in 12 seconds.",
      "media_urls": ["https://cdn.example.com/launch.gif"]
    }
  ]
}'

# Validate the exact destination payload before publishing.
VALIDATION=$(curl -fSs -X POST "https://api.unipost.dev/v1/posts/validate" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d "$GIF_PAYLOAD")

if ! echo "$VALIDATION" | jq -e '.data.valid == true' >/dev/null; then
  echo "Hosted GIF validation failed" >&2
  echo "$VALIDATION" | jq >&2
  exit 1
fi

# Publish one hosted GIF to X and Facebook.
curl -fSs -X POST "https://api.unipost.dev/v1/posts" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d "$GIF_PAYLOAD"`,
  },
  {
    label: "Node.js",
    lang: "javascript",
    code: `const apiBase = "https://api.unipost.dev";
const headers = {
  Authorization: "Bearer " + process.env.UNIPOST_API_KEY,
  "Content-Type": "application/json",
};

const payload = {
  platform_posts: [
    {
      account_id: "sa_twitter_123",
      caption: "The launch flow in 12 seconds.",
      media_urls: ["https://cdn.example.com/launch.gif"],
    },
    {
      account_id: "sa_facebook_123",
      caption: "The launch flow in 12 seconds.",
      media_urls: ["https://cdn.example.com/launch.gif"],
    },
  ],
};

const validationResponse = await fetch(apiBase + "/v1/posts/validate", {
  method: "POST",
  headers,
  body: JSON.stringify(payload),
});
const validation = await validationResponse.json();

if (!validationResponse.ok || !validation.data.valid) {
  throw new Error(JSON.stringify(validation, null, 2));
}

const publishResponse = await fetch(apiBase + "/v1/posts", {
  method: "POST",
  headers,
  body: JSON.stringify(payload),
});
const post = await publishResponse.json();

if (!publishResponse.ok) {
  throw new Error(JSON.stringify(post, null, 2));
}

console.log(post.data.id);`,
  },
];

const LOCAL_GIF_SNIPPETS = [
  {
    label: "cURL",
    lang: "bash",
    code: `set -euo pipefail

# Step 1: Find the connected X and Facebook account IDs.
curl -fSs "https://api.unipost.dev/v1/accounts" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"

# Step 2: Reserve a UniPost media upload for the local GIF.
RESERVATION=$(curl -fSs -X POST "https://api.unipost.dev/v1/media" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "filename": "launch.gif",
    "content_type": "image/gif"
  }')

MEDIA_ID=$(echo "$RESERVATION" | jq -er '.data.id')
UPLOAD_URL=$(echo "$RESERVATION" | jq -er '.data.upload_url')

# Step 3: Upload the raw GIF bytes to the presigned storage URL.
curl -fsS -X PUT "$UPLOAD_URL" \\
  -H "Content-Type: image/gif" \\
  --data-binary "@launch.gif"

# Step 4: Poll for up to five minutes.
MEDIA_DEADLINE=$((SECONDS + 300))
while (( SECONDS < MEDIA_DEADLINE )); do
  MEDIA=$(curl -fSs "https://api.unipost.dev/v1/media/$MEDIA_ID" \\
    -H "Authorization: Bearer $UNIPOST_API_KEY")
  MEDIA_STATUS=$(echo "$MEDIA" | jq -er '.data.status')

  case "$MEDIA_STATUS" in
    uploaded|attached) break ;;
    pending) sleep 1 ;;
    *) echo "Unexpected media status: $MEDIA_STATUS" >&2; echo "$MEDIA" | jq >&2; exit 1 ;;
  esac
done
if [ "$MEDIA_STATUS" != "uploaded" ] && [ "$MEDIA_STATUS" != "attached" ]; then
  echo "Timed out waiting for the GIF upload" >&2
  exit 1
fi

GIF_PAYLOAD=$(jq -n --arg media_id "$MEDIA_ID" '{
  platform_posts: [
    {
      account_id: "sa_twitter_123",
      caption: "The launch flow in 12 seconds.",
      media_ids: [$media_id]
    },
    {
      account_id: "sa_facebook_123",
      caption: "The launch flow in 12 seconds.",
      media_ids: [$media_id]
    }
  ]
}')

# Step 5: Validate the final destination payload.
VALIDATION=$(curl -fSs -X POST "https://api.unipost.dev/v1/posts/validate" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d "$GIF_PAYLOAD")

if [ "$(echo "$VALIDATION" | jq -er '.data.valid')" != "true" ]; then
  echo "$VALIDATION" >&2
  exit 1
fi

# Step 6: Publish the uploaded GIF to X and Facebook.
POST=$(curl -fSs -X POST "https://api.unipost.dev/v1/posts" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d "$GIF_PAYLOAD")

POST_ID=$(echo "$POST" | jq -er '.data.id')

# Step 7: Poll the asynchronous results for up to 15 minutes.
POST_DEADLINE=$((SECONDS + 900))
while (( SECONDS < POST_DEADLINE )); do
  RESULT=$(curl -fSs "https://api.unipost.dev/v1/posts/$POST_ID" \\
    -H "Authorization: Bearer $UNIPOST_API_KEY")
  STATUS=$(echo "$RESULT" | jq -er '.data.status')

  case "$STATUS" in
    published)
      echo "$RESULT" | jq
      exit 0
      ;;
    partial|failed|cancelled) echo "$RESULT" | jq >&2; exit 1 ;;
    queued|dispatching|retrying|publishing|scheduled) sleep 2 ;;
    *) echo "Unexpected post status: $STATUS" >&2; echo "$RESULT" | jq >&2; exit 1 ;;
  esac
done
echo "Timed out waiting for publishing results" >&2
exit 1`,
  },
];

export default function PublishGifsGuidePage() {
  return (
    <DocsPage
      eyebrow="Publishing Guides"
      title="Publish GIFs to X and Facebook"
      lead="Publish a hosted or local GIF to X and Facebook, compare official platform support with UniPost support, and convert GIFs for destinations that require video to preserve GIF animation."
      className="docs-page-wide"
    >
      <h2 id="platform-support">Platform support</h2>
      <p>
        Direct GIF publishing is available in UniPost today for X and Facebook Pages. Other destinations fall into two
        groups: LinkedIn and Threads have upstream GIF capabilities that UniPost has not connected yet, while the
        remaining platforms need a video-based workflow instead of an unchanged GIF file.
      </p>
      <DocsTable
        columns={["Platform", "Official GIF support", "UniPost status", "Recommended action"]}
        rows={[
          [
            "X / Twitter",
            "Yes — direct GIF media upload",
            "Supported",
            "Publish the GIF directly.",
          ],
          [
            "Facebook Page",
            "Yes — GIF photo post",
            "Supported",
            "Publish the GIF directly.",
          ],
          [
            "LinkedIn",
            "Yes — through LinkedIn image APIs",
            "Coming soon",
            "Wait for native UniPost GIF support.",
          ],
          [
            "Threads",
            "Yes — through provider-backed GIF attachments",
            "Coming soon",
            "Wait for UniPost GIF attachment support.",
          ],
          [
            "Instagram",
            "No direct GIF publishing surface",
            "GIF-to-MP4 conversion available; destination-specific publishing guidance coming soon",
            "Convert the GIF to MP4, then use the destination's video workflow.",
          ],
          [
            "TikTok",
            "No direct GIF publishing surface",
            "GIF-to-MP4 conversion available; destination-specific publishing guidance coming soon",
            "Convert the GIF to MP4, then use the destination's video workflow.",
          ],
          [
            "Pinterest",
            "No direct animated GIF publishing surface in the supported organic Pin API flow",
            "GIF-to-MP4 conversion available; destination-specific publishing guidance coming soon",
            "Convert the GIF to MP4, then use the destination's video workflow.",
          ],
          [
            "YouTube",
            "No GIF post type; publishing requires video media",
            "GIF-to-MP4 conversion available; destination-specific publishing guidance coming soon",
            "Convert the GIF to MP4, then use the destination's video workflow.",
          ],
          [
            "Bluesky",
            "No direct GIF media type in the documented image and video embed APIs",
            "GIF-to-MP4 conversion available; destination-specific publishing guidance coming soon",
            "Convert the GIF to MP4, then use the destination's video workflow.",
          ],
        ]}
      />

      <div className="docs-callout docs-callout-tip">
        <strong>Current direct destinations:</strong> send GIF files only to connected X and Facebook Page accounts.
        LinkedIn and Threads direct GIF paths remain coming soon. For destinations that require video to preserve GIF
        animation, convert the GIF first; do not send the unchanged GIF and wait for the upstream platform to reject it.
      </div>

      <h2 id="supported-workflow">Choose the media source</h2>
      <p>
        Use the recommended <code>platform_posts[]</code> request shape so each destination has its own account,
        caption, and media fields. UniPost accepts either of these GIF sources:
      </p>
      <DocsTable
        columns={["Source", "Request field", "Workflow"]}
        rows={[
          [
            "Public hosted GIF",
            <code key="hosted">platform_posts[].media_urls</code>,
            "Send a direct, publicly reachable URL whose path ends in .gif. Query strings are allowed.",
          ],
          [
            "Local .gif file",
            <code key="local">platform_posts[].media_ids</code>,
            "Reserve an upload, PUT the file bytes, wait for the media row, then publish its media_id.",
          ],
        ]}
      />
      <p>
        Find destination IDs with <ApiInlineLink endpoint="GET /v1/accounts" />. If the accounts are not connected yet,
        complete the relevant X or Facebook connection flow before creating the post.
      </p>

      <h2 id="hosted-gif">Publish a hosted GIF</h2>
      <p>
        If the GIF already has a stable public URL, send it directly in <code>media_urls</code>. The same URL can be
        reused for the X and Facebook destinations, while each platform post can keep a different caption.
      </p>
      <p>
        The URL path must end in <code>.gif</code>; Query strings are allowed. If your CDN provides an extensionless URL,
        upload it and use <code>media_ids</code> instead so UniPost can identify the stored content as{" "}
        <code>image/gif</code>.
      </p>
      <p>
        Validate with <ApiInlineLink endpoint="POST /v1/posts/validate" href="/docs/api/posts/validate" />, then publish
        the identical payload with <ApiInlineLink endpoint="POST /v1/posts" href="/docs/api/posts/create" />.
      </p>
      <div className="docs-callout docs-callout-warning">
        <strong>Hosted URL validation:</strong> the validate endpoint does not download hosted media bytes. It can
        validate the URL shape and destination rules, but cannot confirm the hosted file&apos;s actual MIME type,
        dimensions, or size. Keep the URL public and confirm that it returns the expected GIF before publishing.
      </div>
      <DocsCodeTabs snippets={HOSTED_GIF_SNIPPETS} />

      <h2 id="local-gif">Publish a local GIF</h2>
      <p>
        Local files use the media library. Reserve the upload with{" "}
        <ApiInlineLink endpoint="POST /v1/media" href="/docs/api/media/reserve" />, upload the raw bytes to the returned{" "}
        <code>upload_url</code>, and poll{" "}
        <ApiInlineLink endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" /> until the status is{" "}
        <code>uploaded</code> or <code>attached</code>.
      </p>
      <p>
        After publishing, the create response is asynchronous. Poll{" "}
        <ApiInlineLink endpoint="GET /v1/posts/:post_id" href="/docs/api/posts/get" /> and inspect each result row for
        the final X and Facebook outcome.
      </p>
      <DocsCodeTabs snippets={LOCAL_GIF_SNIPPETS} />

      <h2 id="platform-limits">X and Facebook limits</h2>
      <DocsTable
        columns={["Rule", "X / Twitter", "Facebook Page"]}
        rows={[
          ["GIF count", "Exactly one GIF", "Exactly one GIF"],
          ["UniPost file-size cap", "5 MB", "10 MB"],
          ["Mixed media", "Do not combine the GIF with images or video", "Do not combine the GIF with other media"],
          ["Links", "Caption URLs follow normal X post behavior", "Do not combine Facebook link options with media"],
          ["Scheduling", "UniPost scheduling is supported", "Publish the Facebook GIF immediately"],
        ]}
      />
      <div className="docs-callout docs-callout-warning">
        <strong>Publishing to both platforms:</strong> the GIF must be 5 MB or smaller because one cross-platform
        request must satisfy the stricter X limit. Do not include <code>scheduled_at</code> when the request contains a
        Facebook GIF.
      </div>
      <div className="docs-callout docs-callout-warning">
        <strong>X validation boundary:</strong> <code>POST /v1/posts/validate</code> does not currently enforce X&apos;s
        one-GIF-only rule or reject a GIF combined with another image. Send exactly one GIF as the only X media item;
        otherwise X can reject the post after UniPost queues it.
      </div>

      <h2 id="convert-to-mp4">Convert a GIF for video destinations</h2>
      <p>
        LinkedIn and Threads require native integrations with their upstream GIF capabilities. UniPost will add those
        destination-specific paths without pretending that the platforms themselves reject GIFs.
      </p>
      <p>
        To preserve animation on Instagram, TikTok, Pinterest, YouTube, and Bluesky, convert the GIF to video media.
        Upload the GIF with <ApiInlineLink endpoint="POST /v1/media" href="/docs/api/media/reserve" />, create a job
        with <ApiInlineLink endpoint="POST /v1/media/gif-conversions" href="/docs/api/media/gif-conversions" />, and
        poll <code>GET /v1/media/gif-conversions/&#123;id&#125;</code>. A successful job returns an MP4{" "}
        <code>output_media_id</code>.
      </p>
      <p>
        Conversion does not publish, edit a draft, or replace the original GIF. Use the returned Media ID in a normal
        video publishing request after validating the selected destination. Destination-specific publishing guides and
        the Dashboard conversion control are still coming soon.
      </p>
      <div className="docs-callout docs-callout-tip">
        <strong>Output profile:</strong> UniPost creates a silent H.264 MP4 at 30 FPS, composites transparency over
        white by default, preserves complete animation cycles, avoids upscaling, and limits the longest edge to 1920
        pixels. Static GIFs and animations shorter than five seconds produce at least five seconds of video; short
        animations repeat complete cycles. TikTok requires both output dimensions to be at least 360 pixels; validation
        rejects smaller outputs before dispatch.
      </div>

      <h2 id="common-errors">Common errors</h2>
      <DocsTable
        columns={["Code or state", "What it means", "How to fix it"]}
        rows={[
          [
            "unsupported_format",
            "The GIF targets a destination that UniPost does not currently support for direct GIF publishing.",
            "Target X or Facebook directly, or convert the GIF to MP4 before using a video destination.",
          ],
          [
            "file_too_large",
            "The GIF exceeds a destination's UniPost file-size limit.",
            "Use 5 MB or smaller when one request targets both X and Facebook.",
          ],
          [
            "media_not_uploaded",
            "A local media_id is still pending.",
            <>
              PUT the GIF bytes, then poll{" "}
              <ApiInlineLink key="media-error-link" endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" />{" "}
              until the media is ready.
            </>,
          ],
          [
            "mixed_media_unsupported",
            "The request combines image-family media with video.",
            "Do not combine the GIF with video. X and Facebook each require the GIF to be the only media item.",
          ],
          [
            "facebook_scheduled_media_unsupported",
            "The Facebook GIF request includes scheduled_at.",
            "Remove scheduled_at and publish the Facebook GIF immediately.",
          ],
          [
            "Asynchronous destination failure",
            "UniPost accepted and queued the post, but X or Facebook later rejected delivery.",
            <>
              Poll <ApiInlineLink key="post-error-link" endpoint="GET /v1/posts/:post_id" href="/docs/api/posts/get" />{" "}
              and inspect the failed destination's structured error fields.
            </>,
          ],
        ]}
      />

      <h2 id="reference">API Reference</h2>
      <div className="docs-next-grid">
        <Link href="/docs/api/accounts/list" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">List accounts</div>
          <div className="docs-next-body">Find the connected X and Facebook account IDs used in platform_posts[].</div>
        </Link>
        <Link href="/docs/api/media/reserve" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Reserve media upload</div>
          <div className="docs-next-body">Create a media row and receive the presigned upload URL for a local GIF.</div>
        </Link>
        <Link href="/docs/api/media/get" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Get media</div>
          <div className="docs-next-body">Wait until a local GIF is uploaded or attached before publishing it.</div>
        </Link>
        <Link href="/docs/api/media/gif-conversions" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Convert GIF to MP4</div>
          <div className="docs-next-body">Create and poll an asynchronous universal MP4 conversion job.</div>
        </Link>
        <Link href="/docs/api/posts/validate" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Validate post</div>
          <div className="docs-next-body">Check media format, size, account state, and platform constraints.</div>
        </Link>
        <Link href="/docs/api/posts/create" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Create post</div>
          <div className="docs-next-body">Queue the X and Facebook GIF destinations for asynchronous publishing.</div>
        </Link>
        <Link href="/docs/api/posts/get" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Get post</div>
          <div className="docs-next-body">Read the final aggregate status and per-destination publishing results.</div>
        </Link>
        <Link href="/docs/platforms/twitter" className="docs-next-card">
          <div className="docs-next-kicker">Platform</div>
          <div className="docs-next-title">X / Twitter</div>
          <div className="docs-next-body">Review X media, caption, thread, scheduling, and plan requirements.</div>
        </Link>
        <Link href="/docs/platforms/facebook" className="docs-next-card">
          <div className="docs-next-kicker">Platform</div>
          <div className="docs-next-title">Facebook Page</div>
          <div className="docs-next-body">Review Facebook feed media limits and unsupported combinations.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
