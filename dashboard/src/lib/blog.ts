import fs from "node:fs";
import path from "node:path";

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

export const staticBlogPosts: BlogPost[] = [
  {
    slug: "social-media-analytics-api",
    title: "Social Media Analytics API: Posts Overview and Platform Insights in UniPost",
    seoTitle: "Social Media Analytics API for TikTok, Instagram, Threads, Pinterest",
    description:
      "How UniPost combines cross-platform post analytics with native platform analytics for TikTok, Instagram, Threads, and Pinterest.",
    excerpt:
      "UniPost analytics gives developers one place to inspect published post performance and platform-native metrics across TikTok, Instagram, Threads, and Pinterest.",
    publishedAt: "2026-05-25",
    updatedAt: "2026-05-25",
    readingTime: "6 min read",
    category: "Analytics",
    author: "UniPost",
    keywords: [
      "social media analytics API",
      "TikTok analytics API",
      "Instagram analytics API",
      "Threads analytics API",
      "Pinterest analytics API",
      "social media post analytics",
      "cross-platform analytics API",
    ],
    blocks: [
      {
        type: "lead",
        text:
          "A social media API should not stop at publishing. Once your app sends posts to TikTok, Instagram, Threads, Pinterest, and the rest of your social stack, users immediately ask what happened next. UniPost Analytics gives developers a normalized Posts Overview plus platform-specific drilldowns for the networks that expose deeper native metrics.",
      },
      {
        type: "summary",
        title: "Key takeaways",
        items: [
          "Posts Overview shows cross-platform performance for content published through UniPost, including views, reach, likes, comments, shares, saves, and clicks where each platform supports them.",
          "Platform Analytics gives native drilldowns for [TikTok](/tools/tiktok-analytics), [Instagram](/tools/instagram-analytics), [Threads](/tools/threads-analytics), and [Pinterest](/tools/pinterest-analytics).",
          "UniPost normalizes the analytics shape without pretending every platform exposes the same metrics.",
          "Developers can start with the [Analytics API docs](/docs/api/analytics), post-level analytics in [POST analytics](/docs/api/analytics/posts), and account metrics in [Account Metrics](/docs/api/accounts/metrics).",
        ],
      },
      {
        type: "paragraph",
        text:
          "Most teams begin with a publish button. The second feature request is usually a reporting screen: did the post publish, which platform performed best, and what should the product show when one network exposes reach while another exposes views or outbound clicks?",
      },
      {
        type: "heading",
        text: "Posts Overview: one place for published content performance",
      },
      {
        type: "paragraph",
        text:
          "Posts Overview is the normalized layer. It answers how each piece of content performed after UniPost published it, regardless of whether the destination was a short video, an image post, a Thread, or a Pin. This is the right surface for product dashboards, customer reports, campaign tables, and agent workflows that need a reliable result after publishing.",
      },
      {
        type: "list",
        items: [
          "Use [GET /v1/posts/:post_id/analytics](/docs/api/analytics/posts) to inspect one post across the accounts it was published to.",
          "Use [GET /v1/analytics/summary](/docs/api/analytics/summary) for account-wide performance snapshots.",
          "Use [GET /v1/analytics/by-platform](/docs/api/analytics) when your UI needs to compare platforms inside the same reporting period.",
          "Keep unavailable metrics explicit. A platform that does not expose saves should show unavailable data, not a fabricated zero.",
        ],
      },
      {
        type: "heading",
        text: "Platform Analytics: native metrics when the platform has more to say",
      },
      {
        type: "paragraph",
        text:
          "Platform Analytics is the drilldown layer. It keeps the normalized analytics model, then adds the native fields a specific provider exposes. This is useful when users want to inspect the platform itself: TikTok public videos, Instagram Business media, Threads replies and reposts, or Pinterest board and Pin performance.",
      },
      {
        type: "table",
        caption: "Current platform analytics surfaces in UniPost",
        headers: ["Platform", "Native surface", "Useful metrics", "Preview"],
        rows: [
          ["TikTok", "Profile, account stats, public videos", "Followers, likes, videos, views, comments, shares", "[TikTok Analytics](/tools/tiktok-analytics)"],
          ["Instagram", "Business profile and recent media", "Reach, likes, comments, shares, saves, media count", "[Instagram Analytics](/tools/instagram-analytics)"],
          ["Threads", "Profile and recent posts", "Views, likes, replies, reposts, quotes", "[Threads Analytics](/tools/threads-analytics)"],
          ["Pinterest", "Boards and published Pins", "Impressions, saves, outbound clicks, comments", "[Pinterest Analytics](/tools/pinterest-analytics)"],
        ],
      },
      {
        type: "heading",
        text: "Why platform metrics do not all match",
      },
      {
        type: "paragraph",
        text:
          "Social platforms do not expose a universal analytics schema. Instagram Business accounts can expose reach and saves for media. TikTok's approved analytics scopes focus on profile stats and public video metrics. Threads centers conversation metrics like replies, reposts, and quotes. Pinterest cares about impressions, saves, and outbound clicks. UniPost keeps those differences visible so your product can make honest UI decisions.",
      },
      {
        type: "note",
        title: "A normalized API should not flatten away meaning",
        text:
          "The point of a unified analytics API is not to force every platform into the same numbers. It is to give your app one integration pattern while preserving the metrics users expect on each network.",
      },
      {
        type: "heading",
        text: "How a developer workflow fits together",
      },
      {
        type: "paragraph",
        text:
          "A typical UniPost analytics workflow is: connect social accounts, publish posts, read delivery status, then fetch analytics for the published content. The same connected account IDs used for publishing are used when the dashboard drills into platform analytics.",
      },
      {
        type: "code",
        language: "bash",
        code: `curl "https://api.unipost.dev/v1/posts/post_abc123/analytics?refresh=true" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"

curl "https://api.unipost.dev/v1/analytics/by-platform?range=30d" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
      },
      {
        type: "paragraph",
        text:
          "For account-level metrics, use [Account Metrics](/docs/api/accounts/metrics). For post-level results, start with [Post Analytics](/docs/api/analytics/posts). If you want to see the productized platform surfaces first, compare the public previews for [TikTok](/tools/tiktok-analytics), [Instagram](/tools/instagram-analytics), [Threads](/tools/threads-analytics), and [Pinterest](/tools/pinterest-analytics).",
      },
      {
        type: "heading",
        text: "FAQ",
      },
      {
        type: "faq",
        items: [
          {
            question: "Is UniPost Analytics only for posts published through UniPost?",
            answer:
              "The normalized Posts Overview is centered on content published through UniPost because UniPost can reliably map each post to the connected account and platform result. Some platform drilldowns can also show native account or content inventory when the platform exposes it through approved scopes.",
          },
          {
            question: "Why do some platforms show reach while others show views or impressions?",
            answer:
              "Each social network exposes different analytics fields. UniPost normalizes the API pattern and keeps platform-specific metric names visible so your UI can avoid misleading comparisons.",
          },
          {
            question: "Which platform analytics pages are available now?",
            answer:
              "The first public analytics previews cover TikTok, Instagram, Threads, and Pinterest. They mirror the platform analytics surfaces available in the UniPost dashboard.",
          },
          {
            question: "Do I need separate integrations for publishing and analytics?",
            answer:
              "No. UniPost uses the same account connection and post model for publishing, delivery status, and analytics. Your app calls one API surface instead of maintaining a separate provider integration for each network.",
          },
        ],
      },
    ],
  },
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
  {
    slug: "ai-agent-social-media-posting",
    title: "How to Give Claude the Ability to Post to Social Media",
    seoTitle: "How to Give Claude (and Any AI Agent) the Ability to Post to Social Media",
    description:
      "Why AI agents can draft a tweet but cannot publish it, and how to wire Claude, Cursor, or any MCP-compatible agent into one social publishing API that handles OAuth, media uploads, and per-platform delivery.",
    excerpt:
      "LLMs can write the post. They cannot send it—OAuth, scopes, and media uploads are not model problems. Here is the MCP setup that closes the gap.",
    publishedAt: "2026-05-20",
    updatedAt: "2026-05-20",
    readingTime: "6 min read",
    category: "AI & Agents",
    author: "UniPost",
    keywords: [
      "AI agent post to social media",
      "Claude post to Twitter",
      "MCP social media",
      "Claude Desktop MCP server",
      "LLM publish to LinkedIn",
      "social media API for AI agents",
      "@unipost/mcp-server",
    ],
    blocks: [
      {
        type: "lead",
        text:
          "An MCP server for social media publishing lets AI agents draft, schedule, and publish posts to networks like X, LinkedIn, TikTok, and Instagram without exposing OAuth tokens or per-platform delivery logic to the agent itself. The Model Context Protocol (MCP) standardizes how an agent discovers and calls external tools; a unified publishing API behind that protocol gives the agent a single tool worth calling.",
      },
      {
        type: "summary",
        title: "Key takeaways",
        items: [
          "AI agents can draft social posts but rarely publish them—the bottleneck is OAuth, media validation, and per-platform delivery, not text generation.",
          "MCP standardizes how an agent discovers and calls tools, but it does not solve the authentication and delivery work behind those tools. The MCP server has to.",
          "UniPost exposes an MCP server that turns nine social platforms into a handful of `unipost_*` tools, including `unipost_create_draft` and `unipost_publish_draft`.",
          "OAuth stays user-initiated through a [hosted Connect Session](/docs/api/connect/sessions/create) the agent never touches; after completion, your app gives the agent the returned account id (`completed_social_account_id`, also exposed as `managed_account_id`).",
        ],
      },
      {
        type: "paragraph",
        text:
          "This is useful for Claude Desktop and Cursor users whose AI assistant should actually publish drafts, for AI content products embedding a publish step in their workflow, and for autonomous agents that schedule and post on a user's behalf. Teams that want the same publish surface without the agent layer can read [the multi-platform publishing guide](/blog/social-media-publishing-api) instead.",
      },
      {
        type: "heading",
        text: "Why can't AI agents already post to social media?",
      },
      {
        type: "paragraph",
        text:
          "The hard part is not writing the post. A modern LLM can produce a publication-ready X thread in seconds and tune it to nine networks in a minute. The hard part is finishing the workflow. Three things sit between draft and published that do not belong to the model.",
      },
      {
        type: "list",
        items: [
          "Authorizing the user's social account. OAuth is a browser-mediated, user-initiated handshake. The agent cannot grant access on the user's behalf, and any pattern that lets it try is a security incident in the making.",
          "Translating one piece of content into nine platform shapes. [Instagram](/instagram-api) requires Business or Creator accounts, [TikTok](/tiktok-api) gates publishing scopes behind app review, and [LinkedIn](/linkedin-api) wants different payloads for text, image, and document posts.",
          "Reporting delivery back to the agent. Publishing is asynchronous—the platform accepts the request, then succeeds, fails with a platform-specific error, or sits in a processing state. The agent needs to know which, without pretending failed posts succeeded.",
        ],
      },
      {
        type: "paragraph",
        text:
          "None of this is a model failure. The work has to live somewhere, and that somewhere should not be inside the tool-use loop.",
      },
      {
        type: "heading",
        text: "What does MCP standardize, and what does it leave to the server?",
      },
      {
        type: "paragraph",
        text:
          "The Model Context Protocol standardizes how an agent discovers and calls external tools. Claude Desktop, Cursor, Cline, and Continue.dev all speak it today. MCP does not solve the authentication geometry behind those tools. If the tool definition is “post to TikTok,” MCP does not say where the OAuth token comes from, how it gets refreshed, or what happens when the user revokes scopes. That work lives in the MCP server.",
      },
      {
        type: "paragraph",
        text:
          "UniPost's MCP server sits in exactly that gap. The flow is `Agent → MCP → UniPost API → 9 social platforms`. The agent calls a flat `unipost_*` tool, UniPost handles the per-platform shape, and OAuth tokens stay on the UniPost side—bound to the account id the user created out of band.",
      },
      {
        type: "heading",
        text: "How do I wire Claude into UniPost?",
      },
      {
        type: "paragraph",
        text:
          "Use UniPost's hosted Streamable HTTP MCP endpoint in Claude Desktop. That keeps Claude on the current tool surface without running a local stdio proxy.",
      },
      {
        type: "code",
        language: "json",
        code: `{
  "mcpServers": {
    "unipost": {
      "url": "https://mcp.unipost.dev/mcp",
      "headers": {
        "Authorization": "Bearer up_live_..."
      }
    }
  }
}`,
      },
      {
        type: "paragraph",
        text:
          "Restart Claude Desktop and the `unipost_list_accounts`, `unipost_create_draft`, `unipost_publish_draft`, `unipost_create_connect_session`, and other current `unipost_*` tools become available in the next conversation. Cursor, Cline, and Continue.dev have their own MCP config files; use the same URL and authorization header in their MCP server entry.",
      },
      {
        type: "heading",
        text: "What does an AI publish workflow look like in practice?",
      },
      {
        type: "paragraph",
        text:
          "With the server connected, a publish task looks like a normal Claude turn—except the side effects are real.",
      },
      {
        type: "code",
        language: "text",
        code: `You: Draft a launch announcement for our new analytics dashboard
     and publish it to X and LinkedIn.

Claude: I'll list your connected accounts first to pick the right ones.
        [tool: unipost_list_accounts]
        → sa_x_kCqz        (account: @yourhandle)
        → sa_linkedin_4dQp (account: Your Company)

        Now I'll create a draft you can review.
        [tool: unipost_create_draft
          platform_posts=[
            { account_id: "sa_x_kCqz",        caption: "..." },
            { account_id: "sa_linkedin_4dQp", caption: "..." }
          ]]
        → draft_id: dr_8H2k

        Want to publish, or edit captions first?

You: Publish.

Claude: [tool: unipost_publish_draft id="dr_8H2k"]
        ✓ Published to X:        https://x.com/yourhandle/status/...
        ✓ Published to LinkedIn: https://www.linkedin.com/feed/update/...`,
      },
      {
        type: "paragraph",
        text:
          "The agent saw three tool calls. The user saw a single confirmation step. Under the hood, `unipost_create_draft` creates a draft with [POST /v1/posts](/docs/api/posts/create) using `status: \"draft\"` and two `platform_posts[]` entries. `unipost_publish_draft` then publishes that draft through [POST /v1/posts/:post_id/publish](/docs/api/posts/drafts/publish), where UniPost dispatches the per-platform delivery to X and LinkedIn behind the scenes.",
      },
      {
        type: "heading",
        text: "How should AI agents handle OAuth?",
      },
      {
        type: "paragraph",
        text:
          "They should not. The account id an agent uses (`sa_x_kCqz` above) is created earlier through a UniPost hosted Connect Session, and that step belongs to the user, not the agent. Your app—or even a one-time CLI run—opens the session, the user clicks through in a browser, and UniPost exposes the completed account as `completed_social_account_id` (`managed_account_id` for hosted Connect callers). That is the id the agent uses from that point on.",
      },
      {
        type: "code",
        language: "bash",
        code: `curl -X POST "https://api.unipost.dev/v1/connect/sessions" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform": "twitter",
    "external_user_id": "user_123",
    "allow_quickstart_creds": true,
    "return_url": "https://yourapp.com/connected"
  }'

# Response → send the user to the returned \`url\`.
# After completion, read \`completed_social_account_id\` (or \`managed_account_id\`)
# from the session response and give that account id to the agent.`,
      },
      {
        type: "paragraph",
        text:
          "Why route OAuth through a hosted URL instead of letting the agent collect credentials? Because the agent is not authorized to grant access on a user's behalf, and any architecture that lets it try will eventually leak tokens, post to the wrong account, or fail a Meta App Review. The MCP server can list, draft, publish, schedule, and pull analytics. Authorizing a new account stays a deliberate, browser-mediated, user-driven step—by design.",
      },
      {
        type: "heading",
        text: "What kinds of agents can I build with this?",
      },
      {
        type: "list",
        items: [
          "Draft-and-publish agents: Claude composes, the user confirms in one turn, the post ships.",
          "Scheduled content agents: a daily routine drafts and queues posts to the right accounts without anyone watching.",
          "Reply-to-comment agents: pair the MCP publishing tools with direct [Inbox API](/docs/api/inbox) calls, or your own MCP wrapper, so an agent can read supported DMs and comments and draft replies under the same authorization model.",
          "Multi-account broadcast from one prompt: same intent, per-platform variants, one tool call.",
        ],
      },
      {
        type: "heading",
        text: "Build the integrations directly or use the MCP server?",
      },
      {
        type: "paragraph",
        text:
          "Most of the cost of giving an AI agent publishing powers is not the tool definition. It is the surrounding work—OAuth, refresh tokens, media specs, delivery status, and webhooks—that you would otherwise rebuild for every platform you add.",
      },
      {
        type: "table",
        caption: "Where the work lives when you give an AI agent publishing access",
        headers: ["Concern", "Build per platform yourself", "UniPost MCP server"],
        rows: [
          ["OAuth + token refresh", "9 user-mediated flows in your app", "Hosted Connect Session, one per platform"],
          ["Media upload + validation", "Per-platform format and size rules", "Built in before publish"],
          ["Per-platform delivery shape", "Custom request builder per network", "One `platform_posts[]` array, all 9 platforms"],
          ["Asynchronous result reporting", "Custom queue + per-platform parsers", "Signed webhooks, one status model"],
          ["New platform support", "New OAuth + adapter + media path", "Available when the provider ships it"],
          ["Time from idea to first AI publish", "Weeks to months", "Hours"],
        ],
      },
      {
        type: "note",
        title: "Open-source reference",
        text:
          "[AgentPost](https://agentpost.dev) is an MIT-licensed AI-native CLI and web frontend built on UniPost. Clone it as a starting point for an agent that drafts, previews, and publishes—or read the source for how the MCP tools are sequenced.",
      },
      {
        type: "heading",
        text: "FAQ",
      },
      {
        type: "faq",
        items: [
          {
            question: "Can my AI agent post without the user's explicit authorization?",
            answer:
              "No, and it should not try. OAuth happens through a hosted Connect Session the user opens in a browser. After the session completes, the agent only receives the resulting account id (`completed_social_account_id` / `managed_account_id`) and uses it for subsequent publish calls.",
          },
          {
            question: "Does this only work with Claude?",
            answer:
              "MCP is an open protocol. Claude Desktop, Cursor, Cline, and Continue.dev support it today. Agents that do not speak MCP can call the [UniPost HTTP API](/docs/api/posts/create) directly with the same publish, draft, and account endpoints.",
          },
          {
            question: "Which UniPost tools does the MCP server expose?",
            answer:
              "`unipost_list_accounts`, `unipost_create_post`, `unipost_create_draft`, `unipost_publish_draft`, `unipost_create_connect_session`, `unipost_upload_media`, `unipost_get_analytics`, `unipost_reschedule_post`, `unipost_cancel_post`, and several others—covering the full publish, draft, schedule, and analytics surface area.",
          },
          {
            question: "What stops an AI agent from spamming?",
            answer:
              "Per-post idempotency keys deduplicate retries, and the underlying platform's rate limits pass through to the agent as normal error responses. UniPost does not suppress legitimate rate-limit errors—the agent sees them and can back off.",
          },
          {
            question: "How do I audit what the agent actually posted?",
            answer:
              "Every publish emits a signed webhook and shows up in the workspace logs with the same `request_id` the agent saw in its tool response. Subscribe via [Webhooks](/docs/api/webhooks/create) to receive state changes in real time.",
          },
        ],
      },
    ],
  },
];

const GENERATED_BLOG_CANDIDATES = [
  path.join(/* turbopackIgnore: true */ process.cwd(), "content", "citeloop", "blog"),
  path.join(/* turbopackIgnore: true */ process.cwd(), "..", "content", "citeloop", "blog"),
];

const unsafeGeneratedPatterns = [/<\s*script\b/i, /^\s*import\s+/m, /\son[a-z]+\s*=/i];

export function loadGeneratedBlogPosts(): BlogPost[] {
  for (const dir of GENERATED_BLOG_CANDIDATES) {
    if (fs.existsSync(dir)) {
      return loadGeneratedBlogPostsFromDirectory(dir);
    }
  }
  return [];
}

export function loadGeneratedBlogPostsFromDirectory(dir: string): BlogPost[] {
  if (!fs.existsSync(dir)) {
    return [];
  }
  return fs
    .readdirSync(dir, { withFileTypes: true })
    .filter((entry) => entry.isFile() && /\.mdx?$/i.test(entry.name))
    .map((entry) => {
      const filePath = path.join(dir, entry.name);
      return parseGeneratedBlogPostFromSource(filePath, fs.readFileSync(filePath, "utf8"));
    })
    .filter((post): post is BlogPost => Boolean(post));
}

export function parseGeneratedBlogPostFromSource(filePath: string, source: string): BlogPost | null {
  if (unsafeGeneratedPatterns.some((pattern) => pattern.test(source))) {
    return null;
  }
  const parsed = splitFrontmatter(source);
  if (!parsed) {
    return null;
  }
  const meta = parseFrontmatter(parsed.frontmatter);
  const fileSlug = path.basename(filePath).replace(/\.mdx?$/i, "");
  const slug = normalizeSlug(stringValue(meta.slug) || fileSlug);
  const title = stringValue(meta.title) || stringValue(meta.h1) || slug;
  const description = stringValue(meta.description) || stringValue(meta.excerpt) || title;
  const publishedAt = stringValue(meta.published_at) || stringValue(meta.publishedAt) || new Date().toISOString().slice(0, 10);
  const updatedAt = stringValue(meta.updated_at) || stringValue(meta.updatedAt) || publishedAt;
  const blocks = parseMarkdownBlocks(parsed.body, title);

  if (!slug || blocks.length === 0) {
    return null;
  }

  const post: BlogPost = {
    slug,
    title,
    seoTitle: stringValue(meta.seo_title) || stringValue(meta.seoTitle) || title,
    description,
    excerpt: stringValue(meta.excerpt) || description,
    publishedAt,
    updatedAt,
    readingTime: "1 min read",
    category: stringValue(meta.category) || "Engineering",
    author: stringValue(meta.author) || "UniPost",
    keywords: arrayValue(meta.keywords),
    blocks,
  };
  post.readingTime = `${Math.max(1, Math.ceil(countBlogWords(post) / 220))} min read`;
  return post;
}

export function mergeBlogPosts(existing: BlogPost[], generated: BlogPost[]): BlogPost[] {
  const seen = new Set(existing.map((post) => post.slug));
  const merged = [...existing];
  for (const post of generated) {
    if (!seen.has(post.slug)) {
      seen.add(post.slug);
      merged.push(post);
    }
  }
  return merged.sort((a, b) => Date.parse(b.publishedAt) - Date.parse(a.publishedAt));
}

export const blogPosts: BlogPost[] = mergeBlogPosts(staticBlogPosts, loadGeneratedBlogPosts());

export function getBlogPost(slug: string) {
  return blogPosts.find((post) => post.slug === slug);
}

function splitFrontmatter(source: string): { frontmatter: string; body: string } | null {
  const normalized = source.replace(/^\uFEFF/, "");
  const match = normalized.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n?([\s\S]*)$/);
  if (!match) {
    return null;
  }
  return { frontmatter: match[1], body: match[2] };
}

function parseFrontmatter(frontmatter: string): Record<string, string | string[]> {
  const values: Record<string, string | string[]> = {};
  for (const line of frontmatter.split(/\r?\n/)) {
    const match = line.match(/^([A-Za-z0-9_-]+):\s*(.*)$/);
    if (!match) {
      continue;
    }
    values[match[1]] = parseFrontmatterValue(match[2].trim());
  }
  return values;
}

function parseFrontmatterValue(value: string): string | string[] {
  if (value === "[]") {
    return [];
  }
  if (value.startsWith("[") && value.endsWith("]")) {
    return value
      .slice(1, -1)
      .split(",")
      .map((item) => unquote(item.trim()))
      .filter(Boolean);
  }
  return unquote(value);
}

function unquote(value: string): string {
  if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
    return value.slice(1, -1).replace(/\\"/g, '"');
  }
  return value;
}

function stringValue(value: string | string[] | undefined): string {
  return typeof value === "string" ? value.trim() : "";
}

function arrayValue(value: string | string[] | undefined): string[] {
  if (Array.isArray(value)) {
    return value;
  }
  if (typeof value === "string" && value.trim()) {
    return [value.trim()];
  }
  return [];
}

function normalizeSlug(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 96);
}

function parseMarkdownBlocks(markdown: string, title: string): BlogBlock[] {
  const lines = markdown.replace(/\r\n/g, "\n").split("\n");
  const blocks: BlogBlock[] = [];
  let paragraph: string[] = [];
  let firstParagraph = true;

  const flushParagraph = () => {
    const text = paragraph.join(" ").replace(/\s+/g, " ").trim();
    paragraph = [];
    if (!text) {
      return;
    }
    blocks.push({ type: firstParagraph ? "lead" : "paragraph", text });
    firstParagraph = false;
  };

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const trimmed = line.trim();

    if (!trimmed) {
      flushParagraph();
      continue;
    }

    const codeMatch = trimmed.match(/^```(\w+)?\s*$/);
    if (codeMatch) {
      flushParagraph();
      const code: string[] = [];
      i++;
      while (i < lines.length && !lines[i].trim().startsWith("```")) {
        code.push(lines[i]);
        i++;
      }
      blocks.push({ type: "code", language: codeMatch[1] || "text", code: code.join("\n").trimEnd() });
      continue;
    }

    const headingMatch = trimmed.match(/^(#{1,3})\s+(.+)$/);
    if (headingMatch) {
      flushParagraph();
      const headingText = headingMatch[2].trim();
      if (!(headingMatch[1] === "#" && headingText.toLowerCase() === title.toLowerCase())) {
        blocks.push({ type: "heading", text: headingText });
      }
      continue;
    }

    if (/^[-*]\s+/.test(trimmed)) {
      flushParagraph();
      const items: string[] = [];
      while (i < lines.length && /^[-*]\s+/.test(lines[i].trim())) {
        items.push(lines[i].trim().replace(/^[-*]\s+/, ""));
        i++;
      }
      i--;
      blocks.push({ type: "list", items });
      continue;
    }

    if (trimmed.startsWith("|")) {
      flushParagraph();
      const tableLines: string[] = [];
      while (i < lines.length && lines[i].trim().startsWith("|")) {
        tableLines.push(lines[i].trim());
        i++;
      }
      i--;
      const table = parseMarkdownTable(tableLines);
      if (table) {
        blocks.push(table);
      } else {
        paragraph.push(...tableLines);
      }
      continue;
    }

    paragraph.push(trimmed);
  }
  flushParagraph();
  return blocks;
}

function parseMarkdownTable(lines: string[]): BlogBlock | null {
  if (lines.length < 2 || !/^\|?\s*:?-{3,}/.test(lines[1])) {
    return null;
  }
  const headers = splitTableRow(lines[0]);
  const rows = lines.slice(2).map(splitTableRow).filter((row) => row.length === headers.length);
  if (headers.length === 0 || rows.length === 0) {
    return null;
  }
  return { type: "table", headers, rows };
}

function splitTableRow(line: string): string[] {
  return line
    .replace(/^\|/, "")
    .replace(/\|$/, "")
    .split("|")
    .map((cell) => cell.trim())
    .filter(Boolean);
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
