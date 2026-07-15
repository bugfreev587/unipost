import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";
import { ApiInlineLink } from "../../api/_components/doc-components";

const REQUEST_SHAPES = [
  {
    label: "Recommended platform_posts[]",
    lang: "json",
    code: `{
  "platform_posts": [
    {
      "account_id": "sa_youtube_123",
      "media_ids": ["media_123"],
      "platform_options": {
        "title": "Launch update",
        "made_for_kids": false,
        "privacy_status": "public"
      }
    }
  ]
}`,
  },
  {
    label: "Legacy account_ids",
    lang: "json",
    code: `{
  "account_ids": ["sa_youtube_123"],
  "media_ids": ["media_123"],
  "platform_options": {
    "youtube": {
      "title": "Launch update",
      "made_for_kids": false,
      "privacy_status": "public"
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
      "account_id": "sa_youtube_123",
      "media_ids": ["media_123"],
      "platform_options": {
        "youtube": {
          "title": "Launch update",
          "made_for_kids": false
        }
      }
    }
  ]
}`,
  },
];

const YOUTUBE_EXAMPLES = [
  {
    label: "Public video",
    lang: "json",
    code: `{
  "platform_posts": [
    {
      "account_id": "sa_youtube_123",
      "media_ids": ["media_video_123"],
      "caption": "Full launch walkthrough.",
      "platform_options": {
        "title": "Launch update",
        "made_for_kids": false,
        "privacy_status": "public"
      }
    }
  ]
}`,
  },
  {
    label: "Public Short",
    lang: "json",
    code: `{
  "platform_posts": [
    {
      "account_id": "sa_youtube_123",
      "media_ids": ["media_short_123"],
      "caption": "The launch in 30 seconds.",
      "platform_options": {
        "title": "Launch update",
        "made_for_kids": false,
        "privacy_status": "public",
        "shorts": true
      }
    }
  ]
}`,
  },
  {
    label: "YouTube native schedule",
    lang: "json",
    code: `{
  "platform_posts": [
    {
      "account_id": "sa_youtube_123",
      "media_ids": ["media_video_123"],
      "platform_options": {
        "title": "Launch update",
        "made_for_kids": false,
        "privacy_status": "private",
        "publish_at": "2026-07-20T17:00:00Z"
      }
    }
  ]
}`,
  },
];

const INSTAGRAM_EXAMPLES = [
  {
    label: "Feed post",
    lang: "json",
    code: `{
  "platform_options": {
    "mediaType": "feed"
  }
}`,
  },
  {
    label: "Reel",
    lang: "json",
    code: `{
  "platform_options": {
    "mediaType": "reels"
  }
}`,
  },
  {
    label: "Story",
    lang: "json",
    code: `{
  "media_ids": ["media_story_123"],
  "platform_options": {
    "mediaType": "story"
  }
}`,
  },
];

const TIKTOK_EXAMPLES = [
  {
    label: "Public video",
    lang: "json",
    code: `{
  "platform_options": {
    "privacy_level": "PUBLIC_TO_EVERYONE",
    "disable_comment": false,
    "disable_duet": true,
    "disable_stitch": true,
    "brand_organic_toggle": false,
    "brand_content_toggle": false
  }
}`,
  },
  {
    label: "Branded content",
    lang: "json",
    code: `{
  "platform_options": {
    "privacy_level": "PUBLIC_TO_EVERYONE",
    "disable_comment": false,
    "disable_duet": true,
    "disable_stitch": true,
    "brand_organic_toggle": false,
    "brand_content_toggle": true
  }
}`,
  },
];

const FACEBOOK_EXAMPLES = [
  {
    label: "Feed media",
    lang: "json",
    code: `{
  "media_ids": ["media_image_123"],
  "platform_options": {
    "mediaType": "feed"
  }
}`,
  },
  {
    label: "Reel",
    lang: "json",
    code: `{
  "media_ids": ["media_video_123"],
  "platform_options": {
    "mediaType": "reel"
  }
}`,
  },
  {
    label: "Link only",
    lang: "json",
    code: `{
  "caption": "Read the launch notes",
  "platform_options": {
    "link": "https://example.com/launch"
  }
}`,
  },
];

const PINTEREST_EXAMPLE = [
  {
    label: "Create Pin",
    lang: "json",
    code: `{
  "media_ids": ["media_pin_123"],
  "platform_options": {
    "board_id": "1234567890",
    "title": "Launch inspiration",
    "link": "https://example.com/launch"
  }
}`,
  },
];

export default function PlatformOptionsGuidePage() {
  return (
    <DocsPage
      eyebrow="Publishing Guides"
      title="Platform options examples"
      lead="Copy safe platform-specific options for the publishing destinations that most often need extra configuration."
      className="docs-page-wide"
    >
      <div className="docs-callout docs-callout-tip">
        <strong>Use flat options:</strong> inside <code>platform_posts[]</code>, each <code>platform_options</code> object is
        already scoped to one destination. Put fields such as <code>privacy_status</code> or <code>mediaType</code>{" "}
        directly inside it.
      </div>

      <h2 id="request-shape">Choose one request shape</h2>
      <p>
        The recommended <code>platform_posts[]</code> shape uses a flat <code>platform_options</code> object. The Legacy
        account_ids shape uses a top-level object nested by platform name. Do not combine those formats.
      </p>
      <DocsTable
        columns={["Shape", "Options path", "Use"]}
        rows={[
          [
            "Recommended",
            <code key="recommended">platform_posts[].platform_options.privacy_status</code>,
            "Use for new integrations and per-account overrides.",
          ],
          [
            "Legacy account_ids",
            <code key="legacy">platform_options.youtube.privacy_status</code>,
            "Use only with the older top-level account_ids request shape.",
          ],
          [
            "Invalid mixed shape",
            <code key="mixed">platform_posts[].platform_options.youtube.privacy_status</code>,
            "Do not nest the platform name inside platform_posts[].",
          ],
        ]}
      />
      <DocsCodeTabs snippets={REQUEST_SHAPES} />

      <h2 id="validate-first">Validate before publishing</h2>
      <p>
        Send the exact payload to{" "}
        <ApiInlineLink endpoint="POST /v1/posts/validate" href="/docs/api/posts/validate" /> before calling{" "}
        <ApiInlineLink endpoint="POST /v1/posts" href="/docs/api/posts/create" />. Validation catches request-shape,
        account, media, and platform constraints without publishing the post.
      </p>

      <h2 id="youtube">YouTube</h2>
      <p>
        YouTube requires a <code>title</code> and explicit <code>made_for_kids</code> selection. API requests default to <code>private</code>{" "}
        when <code>privacy_status</code> is omitted, so set it to <code>public</code> when the upload
        should be public. Google may still force uploads from an unverified API project to private.
      </p>
      <p>
        For Shorts, supply an eligible square or vertical video no longer than three minutes. <code>shorts: true</code>{" "}
        adds a Shorts hint, but it does not resize, crop, or guarantee that YouTube classifies the upload as a Short.
        YouTube native scheduling requires <code>privacy_status: private</code> with <code>publish_at</code>.
      </p>
      <DocsCodeTabs snippets={YOUTUBE_EXAMPLES} />

      <h2 id="instagram">Instagram</h2>
      <p>
        Set <code>mediaType</code> to <code>feed</code>, <code>reels</code>, or <code>story</code>. A Story accepts exactly
        one image or one video; use a feed carousel for multiple assets.
      </p>
      <DocsCodeTabs snippets={INSTAGRAM_EXAMPLES} />
      <p>
        For a complete Story workflow, see <Link href="/docs/guides/instagram-stories">Publish Instagram Stories</Link>.
      </p>

      <h2 id="tiktok">TikTok</h2>
      <p>
        Make privacy and interaction controls explicit. If a post promotes a third-party brand, set{" "}
        <code>brand_content_toggle</code> to <code>true</code>; use <code>brand_organic_toggle</code> for your own brand.
        The duet and stitch fields apply to videos; omit them for photo carousels.
      </p>
      <DocsCodeTabs snippets={TIKTOK_EXAMPLES} />

      <h2 id="facebook">Facebook</h2>
      <p>
        A Reel requires exactly one video and cannot include a link. For a link-only feed post, omit{" "}
        <code>media_ids</code> and <code>media_urls</code>; Facebook rejects requests that combine link and media.
      </p>
      <DocsCodeTabs snippets={FACEBOOK_EXAMPLES} />

      <h2 id="pinterest">Pinterest</h2>
      <p>
        Every Pin requires a numeric <code>board_id</code> and exactly one image or video. <code>title</code> and{" "}
        <code>link</code> are optional.
      </p>
      <DocsCodeTabs snippets={PINTEREST_EXAMPLE} />

      <h2 id="common-mistakes">Common mistakes</h2>
      <DocsTable
        columns={["Symptom", "Likely cause", "Resolution"]}
        rows={[
          [
            "Options appear to be ignored",
            "A platform name is nested inside platform_posts[].platform_options.",
            "Remove the platform-name wrapper and keep the destination options flat.",
          ],
          [
            "YouTube upload is private",
            "privacy_status was omitted, so the API defaulted to private.",
            "Send privacy_status: public and verify that the Google API project is approved for public uploads.",
          ],
          [
            "Instagram Story publishes as another surface",
            "mediaType is missing or is not story.",
            "Send mediaType: story with exactly one media asset.",
          ],
          [
            "Facebook request fails with link and media",
            "Facebook does not accept both in the same destination post.",
            "Publish either a link-only feed post or a media post.",
          ],
          [
            "Pinterest validation fails",
            "board_id is missing or the Pin has the wrong number of assets.",
            "Send a numeric board_id and exactly one image or video.",
          ],
        ]}
      />

      <h2 id="references">References</h2>
      <p>
        Use the <Link href="/docs/api/posts/create">Create Post API Reference</Link> for the complete endpoint contract,
        and check the <Link href="/docs/platforms">platform pages</Link> for media limits and supported capabilities.
      </p>
    </DocsPage>
  );
}
