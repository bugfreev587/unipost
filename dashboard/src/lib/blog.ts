export type BlogBlock =
  | { type: "lead"; text: string }
  | { type: "summary"; title?: string; items: string[] }
  | { type: "paragraph"; text: string }
  | { type: "heading"; text: string }
  | { type: "list"; items: string[] }
  | { type: "code"; language: string; code: string }
  | { type: "note"; title: string; text: string }
  | { type: "table"; caption?: string; headers: string[]; rows: string[][] }
  | { type: "faq"; items: { question: string; answer: string }[] };

export type BlogPost = {
  slug: string;
  title: string;
  seoTitle: string;
  description: string;
  excerpt: string;
  publishedAt: string;
  updatedAt: string;
  readingTime: string;
  category: string;
  author: string;
  keywords: string[];
  blocks: BlogBlock[];
};

export const blogPosts: BlogPost[] = [
  {
    slug: "social-media-publishing-api",
    title: "How to Add Social Media Publishing to Your App Without Building 9 Integrations",
    seoTitle: "Social Media Publishing API: Post to X, LinkedIn, Instagram + 6 More",
    description:
      "A practical guide to using one social media publishing API to post to X, LinkedIn, Instagram, TikTok, Threads, YouTube, Facebook, Pinterest, and Bluesky.",
    excerpt:
      "How developers can post to X, LinkedIn, Instagram, TikTok, Threads, YouTube, Facebook, Pinterest, and Bluesky through one API.",
    publishedAt: "2026-05-18",
    updatedAt: "2026-05-18",
    readingTime: "5 min read",
    category: "Engineering",
    author: "UniPost",
    keywords: [
      "social media publishing API",
      "social posting API",
      "post to multiple social platforms API",
      "unified social media API",
      "social media API for AI agents",
    ],
    blocks: [
      {
        type: "lead",
        text:
          "A social media publishing API lets your app post to multiple social platforms without building and maintaining every platform integration yourself. Instead of separate publishing logic for X, LinkedIn, Instagram, TikTok, Threads, YouTube, Facebook, Pinterest, and Bluesky, your product can connect accounts once and publish through one API.",
      },
      {
        type: "summary",
        title: "Key takeaways",
        items: [
          "A unified social media publishing API replaces nine per-platform OAuth, media, and delivery integrations with one HTTP surface.",
          "The real cost of building it yourself is not the POST request—it is OAuth flows, media validation, retries, delivery status, and webhooks across nine platforms.",
          "UniPost exposes one [POST /v1/posts](/docs/api/posts/create) call that accepts a `platform_posts[]` array so each account can carry its own caption and media.",
          "A unified API is the right pick when social publishing is a feature inside your product, not the core product itself.",
        ],
      },
      {
        type: "paragraph",
        text:
          "This is useful for scheduling tools, creator platforms, marketing SaaS products, AI content apps, and internal automation workflows where social publishing is important, but not the core engineering project your team wants to own.",
      },
      {
        type: "heading",
        text: "Why is publishing to multiple social platforms hard?",
      },
      {
        type: "paragraph",
        text:
          "The hard part is not sending one HTTP request. The hard part is that all nine major networks have different OAuth scopes, media rules, rate limits, post formats, error shapes, and delivery states. A single publish action on the user side fans out into very different work on the platform side.",
      },
      {
        type: "list",
        items: [
          "OAuth and account connection work differently per platform—[Instagram](/instagram-api) requires Business or Creator accounts, [TikTok](/tiktok-api) requires app review for posting scopes, [Threads](/threads-api) gates on a linked Instagram account.",
          "Images, videos, thumbnails, and aspect ratios have different requirements (e.g. [TikTok](/tiktok-api) wants 1080×1920 vertical video, [LinkedIn](/linkedin-api) supports landscape and square).",
          "Rate limits and permission rules vary by account type and recent activity.",
          "Publish responses need to be normalized into one status model your product can render.",
          "Webhooks and retries become necessary once customers depend on delivery succeeding asynchronously.",
        ],
      },
      {
        type: "paragraph",
        text:
          "That is why a simple publish button often turns into a full integration layer. Teams start with one platform, then quickly need account management, media upload, validation, retries, delivery tracking, and support tooling.",
      },
      {
        type: "heading",
        text: "What should a social media publishing API include?",
      },
      {
        type: "paragraph",
        text:
          "A good social posting API should do more than expose one generic endpoint. It should cover the workflow around publishing, from connecting customer accounts to tracking final delivery.",
      },
      {
        type: "list",
        items: [
          "Hosted OAuth so users can connect social accounts without your team handling tokens.",
          "Media upload for images and videos, with platform-appropriate validation before publish.",
          "Per-account payloads so each destination can have its own caption, media, or platform options.",
          "One API request that fans out to multiple connected accounts.",
          "Delivery status, retries, and webhook events that match a single normalized state model.",
        ],
      },
      {
        type: "heading",
        text: "Build it yourself or use a unified publishing API?",
      },
      {
        type: "paragraph",
        text:
          "Most of the cost is not the publish endpoint itself. It is the surrounding plumbing—OAuth, refresh tokens, media specs, retries, and webhooks—that you would otherwise rebuild for every platform you add.",
      },
      {
        type: "table",
        caption: "Where the engineering work actually lives",
        headers: ["Concern", "Build it yourself", "Unified publishing API"],
        rows: [
          ["OAuth + token refresh", "Implement and maintain 9 flows", "Hosted, one flow per platform"],
          ["Media validation", "Per-platform rules and edge cases", "Built in before publish"],
          ["Retries + delivery status", "Custom queue + state machine", "Built in, one status model"],
          ["Webhooks", "Subscribe and parse per platform", "One subscription surface"],
          ["New platform support", "Engineering project, weeks", "Available as the provider ships"],
          ["Time to first real publish", "Weeks to months", "Hours to days"],
        ],
      },
      {
        type: "heading",
        text: "How do I publish to multiple platforms with one API call?",
      },
      {
        type: "paragraph",
        text:
          "With UniPost, your app connects customer accounts through hosted OAuth, stores the returned connected account IDs (`sa_*`), and sends one publish request with a `platform_posts[]` array. UniPost handles platform differences—media validation, formatting, retries—behind the API.",
      },
      {
        type: "code",
        language: "bash",
        code: `curl -X POST "https://api.unipost.dev/v1/posts" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "status": "published",
    "platform_posts": [
      {
        "account_id": "sa_twitter_1",
        "caption": "We just shipped social publishing from one API."
      },
      {
        "account_id": "sa_linkedin_1",
        "caption": "Engineering note: shipping multi-platform posting with one API call.",
        "media": [{ "type": "image", "url": "https://example.com/launch.png" }]
      }
    ]
  }'`,
      },
      {
        type: "paragraph",
        text:
          "Each entry in `platform_posts[]` can carry its own caption and media, so the same call can publish a short post on X and a longer post on LinkedIn without you maintaining two send paths. The full schema is in the [Create Post API reference](/docs/api/posts/create).",
      },
      {
        type: "note",
        title: "Build vs. buy",
        text:
          "Build directly if one platform integration is your core product (for example, an X-only client). Use a unified API if social publishing is a feature your customers expect inside a larger product.",
      },
      {
        type: "heading",
        text: "What are the best use cases for a unified social publishing API?",
      },
      {
        type: "list",
        items: [
          "Social media schedulers that need multi-platform posting without owning every OAuth flow.",
          "AI content tools that generate captions or videos and need a safe publish path.",
          "Creator platforms that onboard accounts and publish on behalf of users.",
          "Marketing SaaS products adding social distribution alongside their core workflow.",
          "Internal tools that automate announcements, launches, and campaign posts.",
        ],
      },
      {
        type: "heading",
        text: "Start with one publishing workflow",
      },
      {
        type: "paragraph",
        text:
          "The simplest architecture is: connect accounts, upload media, publish content, and listen for delivery status. UniPost gives developers that workflow through one social media publishing API, so teams can ship social features in days instead of maintaining nine separate integrations. The [Quickstart](/docs/quickstart) walks through the first publish end to end.",
      },
      {
        type: "heading",
        text: "FAQ",
      },
      {
        type: "faq",
        items: [
          {
            question: "What is a social media publishing API?",
            answer:
              "A social media publishing API is a single HTTP interface that lets your app connect customer accounts and publish posts to multiple social networks—such as X, LinkedIn, Instagram, TikTok, Threads, YouTube, Facebook, Pinterest, and Bluesky—without integrating each platform separately.",
          },
          {
            question: "How is it different from a workflow tool like Zapier?",
            answer:
              "Workflow tools are aimed at no-code users connecting apps. A social media publishing API is aimed at developers embedding publishing inside their own product, with per-account payloads, validation, delivery status, and webhooks that your application code can rely on.",
          },
          {
            question: "Which platforms does UniPost support today?",
            answer:
              "UniPost supports publishing to X, LinkedIn, Instagram, TikTok, Threads, YouTube, Facebook, Pinterest, and Bluesky. Each platform has its own connection requirements—Instagram needs a Business or Creator account, TikTok requires app review for posting scopes, and Threads requires a linked Instagram account.",
          },
          {
            question: "Can AI agents use it to publish content?",
            answer:
              "Yes. The same API surface that products use is suitable for AI agents. Validation, account scoping, and structured delivery responses help agents publish safely without spraying the same caption across every platform.",
          },
          {
            question: "Do I need to handle media uploads myself?",
            answer:
              "No. You can send a publicly hosted media URL, or reserve a UniPost upload with POST /v1/media and publish using the returned media IDs. UniPost validates aspect ratio, format, and platform constraints before publish.",
          },
          {
            question: "What happens when a publish fails on one platform?",
            answer:
              "Each entry in `platform_posts[]` resolves independently. UniPost returns per-account delivery status, retries transient errors, and emits webhook events so your product can show clear per-platform outcomes to the user.",
          },
        ],
      },
    ],
  },
];

export function getBlogPost(slug: string) {
  return blogPosts.find((post) => post.slug === slug);
}

export function countBlogWords(post: BlogPost): number {
  let words = 0;
  for (const block of post.blocks) {
    if (block.type === "lead" || block.type === "paragraph") {
      words += block.text.split(/\s+/).filter(Boolean).length;
    } else if (block.type === "heading") {
      words += block.text.split(/\s+/).filter(Boolean).length;
    } else if (block.type === "list" || block.type === "summary") {
      for (const item of block.items) {
        words += item.split(/\s+/).filter(Boolean).length;
      }
    } else if (block.type === "note") {
      words += block.title.split(/\s+/).filter(Boolean).length;
      words += block.text.split(/\s+/).filter(Boolean).length;
    } else if (block.type === "table") {
      for (const row of block.rows) {
        for (const cell of row) {
          words += cell.split(/\s+/).filter(Boolean).length;
        }
      }
    } else if (block.type === "faq") {
      for (const item of block.items) {
        words += item.question.split(/\s+/).filter(Boolean).length;
        words += item.answer.split(/\s+/).filter(Boolean).length;
      }
    }
  }
  return words;
}
