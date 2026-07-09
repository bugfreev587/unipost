import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";
import { ApiInlineLink } from "../../api/_components/doc-components";

const STORY_SNIPPETS = [
  {
    label: "cURL",
    lang: "bash",
    code: `# Step 1: Validate the exact payload you plan to publish.
curl -X POST "https://api.unipost.dev/v1/posts/validate" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform_posts": [
      {
        "account_id": "sa_instagram_123",
        "media_urls": ["https://cdn.example.com/story.mp4"],
        "platform_options": {
          "mediaType": "story"
        }
      }
    ]
  }'

# Step 2: Publish the same strict platform_posts shape.
curl -X POST "https://api.unipost.dev/v1/posts" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform_posts": [
      {
        "account_id": "sa_instagram_123",
        "media_urls": ["https://cdn.example.com/story.mp4"],
        "platform_options": {
          "mediaType": "story"
        }
      }
    ]
  }'`,
  },
  {
    label: "Node.js",
    lang: "javascript",
    code: `const apiBase = "https://api.unipost.dev";

const payload = {
  platform_posts: [
    {
      account_id: "sa_instagram_123",
      media_urls: ["https://cdn.example.com/story.mp4"],
      platform_options: {
        mediaType: "story",
      },
    },
  ],
};

const validateResponse = await fetch(apiBase + "/v1/posts/validate", {
  method: "POST",
  headers: {
    Authorization: "Bearer " + process.env.UNIPOST_API_KEY,
    "Content-Type": "application/json",
  },
  body: JSON.stringify(payload),
});

if (validateResponse.status === 422) {
  throw new Error("Request shape is invalid: " + await validateResponse.text());
}

const validation = await validateResponse.json();
if (!validation.data.valid) {
  throw new Error(validation.data.errors[0]?.message || "Story payload is not valid");
}

const publishResponse = await fetch(apiBase + "/v1/posts", {
  method: "POST",
  headers: {
    Authorization: "Bearer " + process.env.UNIPOST_API_KEY,
    "Content-Type": "application/json",
  },
  body: JSON.stringify(payload),
});

if (!publishResponse.ok) {
  throw new Error("Publish failed: " + await publishResponse.text());
}`,
  },
  {
    label: "Python",
    lang: "python",
    code: `import os

import requests

api_base = "https://api.unipost.dev"
headers = {
    "Authorization": f"Bearer {os.environ['UNIPOST_API_KEY']}",
    "Content-Type": "application/json",
}

payload = {
    "platform_posts": [
        {
            "account_id": "sa_instagram_123",
            "media_urls": ["https://cdn.example.com/story.mp4"],
            "platform_options": {
                "mediaType": "story",
            },
        }
    ]
}

validate_response = requests.post(
    f"{api_base}/v1/posts/validate",
    headers=headers,
    json=payload,
)

if validate_response.status_code == 422:
    raise RuntimeError(f"Request shape is invalid: {validate_response.text}")

validation = validate_response.json()
if not validation["data"]["valid"]:
    raise RuntimeError(validation["data"]["errors"][0]["message"])

publish_response = requests.post(
    f"{api_base}/v1/posts",
    headers=headers,
    json=payload,
)
publish_response.raise_for_status()`,
  },
];

const SHAPE_EXAMPLES = [
  {
    label: "Recommended new shape",
    lang: "json",
    code: `{
  "platform_posts": [
    {
      "account_id": "sa_instagram_123",
      "media_urls": ["https://cdn.example.com/story.jpg"],
      "platform_options": {
        "mediaType": "story"
      }
    }
  ]
}`,
  },
  {
    label: "Legacy shape",
    lang: "json",
    code: `{
  "account_ids": ["sa_instagram_123"],
  "media_urls": ["https://cdn.example.com/story.jpg"],
  "platform_options": {
    "instagram": {
      "mediaType": "story"
    }
  }
}`,
  },
  {
    label: "Invalid mixed shape",
    lang: "json",
    code: `{
  "platform_posts": [
    {
      "account_id": "sa_instagram_123",
      "media_urls": ["https://cdn.example.com/story.jpg"],
      "platform_options": {
        "instagram": {
          "mediaType": "story"
        }
      }
    }
  ]
}`,
  },
];

export default function InstagramStoriesGuidePage() {
  return (
    <DocsPage
      eyebrow="Publishing Guides"
      title="Publish Instagram Stories"
      lead="Use this guide when your app needs to publish a single Instagram Story through the UniPost API without accidentally sending it as a normal feed post."
      className="docs-page-wide"
    >
      <div className="docs-callout docs-callout-tip">
        <strong>Recommended shape:</strong> use <code>platform_posts[]</code> and put <code>mediaType: "story"</code>{" "}
        directly inside that destination's flat <code>platform_options</code> object.
      </div>

      <h2 id="when-to-use">When to use this guide</h2>
      <p>
        Instagram Stories are a separate Instagram publishing surface. Use <code>mediaType: "story"</code> when the
        post should appear as ephemeral full-screen story content instead of a feed post or Reel.
      </p>
      <p>
        Stories accept exactly one media item: one image or one video. If your user selects multiple assets, route the
        publish as a feed carousel instead, or ask the user to choose one asset for the Story.
      </p>

      <h2 id="request-shape">Choose one request shape</h2>
      <p>
        UniPost supports both the recommended <code>platform_posts[]</code> request shape and the older{" "}
        <code>account_ids</code> shape. Do not combine their <code>platform_options</code> formats in one request.
      </p>
      <DocsTable
        columns={["Shape", "Where mediaType goes", "Notes"]}
        rows={[
          [
            "New platform_posts[]",
            <code key="new">platform_posts[].platform_options.mediaType</code>,
            "Recommended for new integrations. The platform post is already scoped to one destination, so options are flat.",
          ],
          [
            "Legacy account_ids",
            <code key="legacy">platform_options.instagram.mediaType</code>,
            "Only use this with top-level account_ids. Keep it strict if an older integration still depends on this shape.",
          ],
          [
            "Mixed",
            <code key="mixed">platform_posts[].platform_options.instagram.mediaType</code>,
            "Invalid. UniPost returns 422 because the request mixes legacy platform-scoped options into the new shape.",
          ],
        ]}
      />
      <DocsCodeTabs snippets={SHAPE_EXAMPLES} />

      <h2 id="validate-first">Validate before publishing</h2>
      <p>
        Call <ApiInlineLink endpoint="POST /v1/posts/validate" href="/docs/api/posts/validate" /> with the exact payload
        you plan to publish. A valid request shape always returns <code>200</code>, even when <code>data.valid</code> is{" "}
        <code>false</code> because the account, media, or platform constraints need attention.
      </p>
      <p>
        If the request shape is mixed or otherwise outside the API contract, UniPost returns <code>422</code> with a
        remediation hint and <code>docs_url</code>. Fix that before retrying validation or publish.
      </p>

      <h2 id="example">Example workflow</h2>
      <DocsCodeTabs snippets={STORY_SNIPPETS} />

      <h2 id="media-rules">Story media rules</h2>
      <DocsTable
        columns={["Field", "Required", "Limits", "Notes"]}
        rows={[
          ["media_urls or media_ids", "Yes", "Exactly 1 asset", "Use a public hosted URL or a UniPost media_id that has finished uploading."],
          ["media type", "Yes", "JPEG image, MP4 video, or MOV video", "Stories do not accept text-only posts."],
          ["caption", "No", "Not rendered as a story text overlay", "If the user needs visible text, bake it into the image or video before publishing."],
          ["platform_options.mediaType", "Yes", "story", "Use this flat key in the recommended platform_posts[] shape."],
        ]}
      />

      <h2 id="common-errors">Common errors</h2>
      <DocsTable
        columns={["Code", "What it means", "How to fix it"]}
        rows={[
          [
            "VALIDATION_ERROR",
            "The request shape is invalid, often because platform_posts[] contains legacy platform_options.instagram.",
            <>
              Move <code>mediaType</code> directly under <code>platform_posts[].platform_options</code>, or switch fully
              to the legacy <code>account_ids</code> shape.
            </>,
          ],
          [
            "instagram_story_single_media_only",
            "The Story request contains zero media items or more than one media item.",
            "Send exactly one image or one video for the Instagram destination.",
          ],
          [
            "invalid_instagram_media_type",
            "The Instagram mediaType value is not one of the supported surfaces.",
            <>
              Use <code>feed</code>, <code>reels</code>, or <code>story</code>.
            </>,
          ],
          [
            "media_not_uploaded",
            "A supplied media_id is still pending.",
            <>
              Upload bytes to the <code>upload_url</code>, then poll{" "}
              <ApiInlineLink endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" /> until the media is uploaded.
            </>,
          ],
        ]}
      />

      <h2 id="reference">Reference</h2>
      <div className="docs-next-grid">
        <Link href="/docs/api/posts/validate" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Validate post</div>
          <div className="docs-next-body">Check the exact Story payload and distinguish 200 validation failures from 422 shape errors.</div>
        </Link>
        <Link href="/docs/api/posts/create" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Create post</div>
          <div className="docs-next-body">Publish the same platform_posts[] payload after validation passes.</div>
        </Link>
        <Link href="/docs/platforms/instagram" className="docs-next-card">
          <div className="docs-next-kicker">Platform</div>
          <div className="docs-next-title">Instagram</div>
          <div className="docs-next-body">Review Instagram media specs, surface options, and platform-specific validation codes.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
