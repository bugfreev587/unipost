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
          "UniPost ships an `@unipost/mcp-server` package that turns nine social platforms into a handful of `unipost_*` tools, including `unipost_create_draft` and `unipost_publish_draft`.",
          "OAuth stays user-initiated through a [hosted Connect Session](/docs/api/connect/sessions/create) the agent never touches; the agent only receives the resulting `social_account_id`.",
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
          "UniPost's MCP server sits in exactly that gap. The flow is `Agent → MCP → UniPost API → 9 social platforms`. The agent calls a flat `unipost_*` tool, UniPost handles the per-platform shape, and OAuth tokens stay on the UniPost side—bound to a `social_account_id` the user created out of band.",
      },
      {
        type: "heading",
        text: "How do I wire Claude into UniPost?",
      },
      {
        type: "paragraph",
        text:
          "`@unipost/mcp-server` is published on npm and runs on stdio, so it drops into Claude Desktop's `claude_desktop_config.json` like any other MCP server.",
      },
      {
        type: "code",
        language: "json",
        code: `{
  "mcpServers": {
    "unipost": {
      "command": "npx",
      "args": ["-y", "@unipost/mcp-server"],
      "env": {
        "UNIPOST_API_KEY": "sk_live_..."
      }
    }
  }
}`,
      },
      {
        type: "paragraph",
        text:
          "Restart Claude Desktop and the `unipost_list_accounts`, `unipost_create_draft`, `unipost_publish_draft`, `unipost_create_connect_session`, and a dozen other `unipost_*` tools become available in the next conversation. Cursor, Cline, and Continue.dev use the same config shape with their own MCP host paths.",
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
          "The agent saw three tool calls. The user saw a single confirmation step. UniPost saw a [POST /v1/posts](/docs/api/posts/create) with two `platform_posts[]` entries and dispatched the per-platform delivery to X and LinkedIn behind the scenes.",
      },
      {
        type: "heading",
        text: "How should AI agents handle OAuth?",
      },
      {
        type: "paragraph",
        text:
          "They should not. The `social_account_id` an agent uses (`sa_x_kCqz` above) is created earlier through a UniPost hosted Connect Session, and that step belongs to the user, not the agent. Your app—or even a one-time CLI run—opens the session, the user clicks through in a browser, and UniPost returns the `social_account_id` the agent uses from that point on.",
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
# When they return, the \`completed_social_account_id\` is what the agent uses.`,
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
          "Reply-to-comment agents: with the [Inbox API](/docs/api/inbox), an agent can read DMs and comments and draft replies under the same authorization model.",
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
              "No, and it should not try. OAuth happens through a hosted Connect Session the user opens in a browser. The agent only receives the resulting `social_account_id` and uses it for subsequent publish calls.",
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
