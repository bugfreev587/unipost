export type SeoFaq = {
  q: string;
  a: string;
};

export type SeoLink = {
  label: string;
  href: string;
};

export type SeoSection = {
  title: string;
  body: string;
  bullets: string[];
};

export type SeoGrowthPage = {
  slug: string;
  path: string;
  title: string;
  description: string;
  eyebrow: string;
  h1: string;
  primaryQuery: string;
  summary: string;
  proofPoints: string[];
  codeExample: string;
  workflow: string[];
  sections: SeoSection[];
  limitations: string[];
  faqs: SeoFaq[];
  primaryCta: SeoLink;
  secondaryCta: SeoLink;
  relatedLinks: SeoLink[];
};

const SUPPORTED_PLATFORMS =
  "X, Instagram, Facebook, Threads, TikTok, YouTube, LinkedIn, Pinterest, and Bluesky";

const SHARED_WORKFLOW = [
  "Connect customer accounts through hosted OAuth or customer-owned credentials.",
  "Upload or reference media before publishing so platform validation is centralized.",
  "Publish or schedule posts with POST /v1/posts using account_ids for each destination.",
  "Track results through post status, analytics, and webhooks.",
  "Handle webhooks and errors with platform-specific messages instead of generic failures.",
];

const SHARED_RELATED_LINKS: SeoLink[] = [
  { label: "Create post API", href: "/docs/api/posts/create" },
  { label: "Pricing", href: "/pricing" },
  { label: "Compare vendors", href: "/compare/social-media-apis" },
  { label: "Platform requirements matrix", href: "/resources/social-media-api-platform-requirements" },
];

export const MONEY_PAGES: SeoGrowthPage[] = [
  {
    slug: "social-media-api",
    path: "/social-media-api",
    title: "Unified Social Media API for Developers | UniPost",
    description:
      "Use UniPost as a unified social media API for account connection, media upload, multi-platform publishing, webhooks, and status tracking across nine networks.",
    eyebrow: "Unified Social Media API",
    h1: "A unified social media API for product teams",
    primaryQuery: "unified social media API",
    summary:
      "Build one integration for social account connection, media handling, posting, scheduling, delivery status, and webhooks instead of maintaining separate native APIs for every platform.",
    proofPoints: [
      "9 supported platforms",
      "REST API plus hosted OAuth",
      "SDKs for JS, Python, Go, and Java",
      "Native MCP server for AI agents",
    ],
    codeExample: `await fetch("https://api.unipost.dev/v1/posts", {
  method: "POST",
  headers: {
    "Authorization": \`Bearer \${UNIPOST_API_KEY}\`,
    "Content-Type": "application/json"
  },
  body: JSON.stringify({
    caption: "Launching today",
    account_ids: ["sa_x_123", "sa_linkedin_456"],
    media_ids: ["media_abc"],
    scheduled_at: "2026-07-01T16:00:00Z"
  })
});`,
    workflow: SHARED_WORKFLOW,
    sections: [
      {
        title: "One API surface for social publishing",
        body: `UniPost supports ${SUPPORTED_PLATFORMS}. Your product stores connected account IDs and sends publish requests to a single API surface.`,
        bullets: [
          "Use account_ids to route one post to one or many destinations.",
          "Send text, links, media IDs, and per-platform options in one request.",
          "Use the same API key and error model across supported networks.",
        ],
      },
      {
        title: "OAuth, tokens, and account connection",
        body: "Hosted Connect moves OAuth and token refresh out of your roadmap while still letting customers connect their own accounts.",
        bullets: [
          "Use UniPost credentials for quickstart mode.",
          "Use customer-owned credentials for white-label workflows.",
          "Handle account lifecycle events through webhooks.",
        ],
      },
      {
        title: "Media, webhooks, and platform constraints",
        body: "Media upload, publish status, webhooks, and platform constraints are handled close to the API so your product can stay focused on user experience.",
        bullets: [
          "Reserve or reference media before publishing.",
          "Receive status updates instead of polling every platform.",
          "Show honest validation messages when a platform rejects a post.",
        ],
      },
    ],
    limitations: [
      "Native networks still control OAuth approval, rate limits, media rules, and final content policy decisions.",
      "Some platforms require customer-owned developer credentials for production white-label flows.",
      "Analytics and inbox depth vary by platform because each native API exposes different data.",
    ],
    faqs: [
      {
        q: "What is a unified social media API?",
        a: "It is one API layer for connecting accounts, uploading media, publishing posts, tracking status, and handling webhooks across multiple social networks.",
      },
      {
        q: "Does UniPost replace native social APIs completely?",
        a: "UniPost handles the common publishing workflow, but native platform review, rate limits, and content policy still apply.",
      },
      {
        q: "Which platforms are supported?",
        a: `UniPost supports ${SUPPORTED_PLATFORMS}.`,
      },
    ],
    primaryCta: { label: "Start building", href: "/welcome" },
    secondaryCta: { label: "Read the API docs", href: "/docs/api/posts/create" },
    relatedLinks: SHARED_RELATED_LINKS,
  },
  {
    slug: "social-media-posting-api",
    path: "/social-media-posting-api",
    title: "Social Media Posting API for Developers | UniPost",
    description:
      "Add a social media posting API to your app with one POST /v1/posts call for text, media, scheduling, post status, webhooks, and platform-specific options.",
    eyebrow: "Social Media Posting API",
    h1: "A social media posting API built for shipping",
    primaryQuery: "social media posting API",
    summary:
      "Use one posting endpoint for text, images, videos, carousels, scheduled posts, and platform-specific options while UniPost handles destination mapping and publish outcomes.",
    proofPoints: [
      "POST /v1/posts",
      "Immediate or scheduled publishing",
      "Media upload workflow",
      "Status and webhook callbacks",
    ],
    codeExample: `await fetch("https://api.unipost.dev/v1/posts", {
  method: "POST",
  headers: {
    "Authorization": \`Bearer \${UNIPOST_API_KEY}\`,
    "Content-Type": "application/json"
  },
  body: JSON.stringify({
    caption: "New feature is live",
    account_ids: ["sa_instagram_123", "sa_threads_456"],
    media_urls: ["https://cdn.example.com/launch.mp4"],
    platform_options: {
      instagram: { type: "reel" },
      threads: { reply_control: "everyone" }
    }
  })
});`,
    workflow: SHARED_WORKFLOW,
    sections: [
      {
        title: "Post once, route to many accounts",
        body: "The posting API is designed around connected account IDs, not hard-coded platform branches in your app.",
        bullets: [
          "Send one payload to multiple account_ids.",
          "Keep per-platform settings in platform_options.",
          "Use the returned post ID for status and retry workflows.",
        ],
      },
      {
        title: "Media upload before delivery",
        body: "UniPost lets your system upload or reference media before publishing, then applies platform validation during delivery.",
        bullets: [
          "Use hosted media IDs when your app controls uploads.",
          "Use public URLs when assets already live in your CDN.",
          "Surface platform constraints before users wonder why a post failed.",
        ],
      },
      {
        title: "webhooks for publish results",
        body: "webhooks let your backend react to posted, scheduled, failed, and account-change events without building a polling loop.",
        bullets: [
          "Update your UI when delivery status changes.",
          "Trigger retries or support workflows on failure.",
          "Record platform-specific post IDs for analytics.",
        ],
      },
    ],
    limitations: [
      "Each platform has platform constraints for video length, image ratio, captions, hashtags, and link previews.",
      "Some post types, such as Stories or Shorts, may require different fields than standard feed posts.",
      "Publishing to customer-owned accounts can require native app review before production traffic.",
    ],
    faqs: [
      {
        q: "Can I schedule posts with the posting API?",
        a: "Yes. Add a scheduled_at timestamp to create scheduled posts when your plan and connected accounts support it.",
      },
      {
        q: "Can I send videos and images?",
        a: "Yes. Use media IDs or public media URLs. UniPost still validates platform-specific media rules before delivery.",
      },
      {
        q: "How do I know whether a post succeeded?",
        a: "Use the returned post object, status endpoints, and webhooks to track each destination outcome.",
      },
    ],
    primaryCta: { label: "Create your first post", href: "/welcome" },
    secondaryCta: { label: "View POST /v1/posts", href: "/docs/api/posts/create" },
    relatedLinks: SHARED_RELATED_LINKS,
  },
  {
    slug: "social-media-publishing-api",
    path: "/social-media-publishing-api",
    title: "Social Media Publishing API for SaaS Products | UniPost",
    description:
      "A social media publishing API for SaaS teams that need account connection, media workflows, scheduling, approvals, status tracking, and webhooks.",
    eyebrow: "Social Media Publishing API",
    h1: "A social media publishing API for embedded workflows",
    primaryQuery: "social media publishing API",
    summary:
      "Embed social publishing into your product with the primitives product teams need: customer account connection, media handling, scheduling, status tracking, and reliable delivery feedback.",
    proofPoints: [
      "Embedded SaaS workflows",
      "Hosted Connect and white-label modes",
      "Approval-friendly publishing primitives",
      "Docs and pricing CTAs on every path",
    ],
    codeExample: `await fetch("https://api.unipost.dev/v1/posts", {
  method: "POST",
  headers: {
    "Authorization": \`Bearer \${UNIPOST_API_KEY}\`,
    "Content-Type": "application/json"
  },
  body: JSON.stringify({
    caption: "Approved by the customer",
    account_ids: ["sa_facebook_123", "sa_linkedin_456"],
    publish_mode: "scheduled",
    scheduled_at: "2026-07-02T13:30:00Z",
    metadata: { source: "customer_approval_flow" }
  })
});`,
    workflow: SHARED_WORKFLOW,
    sections: [
      {
        title: "Designed for embedded publishing",
        body: "UniPost works for SaaS products that want social publishing inside their own workflow instead of sending customers to another dashboard.",
        bullets: [
          "Create account connection sessions from your backend.",
          "Store account_ids against your own users or workspaces.",
          "Publish after your own approval, billing, or content generation flow.",
        ],
      },
      {
        title: "Status tracking for product teams",
        body: "Publishing is an operational workflow. UniPost exposes status and webhooks so your UI can explain what happened.",
        bullets: [
          "Show pending, scheduled, posted, and failed states.",
          "Record platform IDs for downstream analytics.",
          "Use webhook events to sync your own activity feed.",
        ],
      },
      {
        title: "Honest platform constraints",
        body: "A publishing API should make platform constraints visible instead of hiding native requirements until production.",
        bullets: [
          "Different platforms expose different media and analytics features.",
          "Native app review can be required for customer-owned credentials.",
          "Some networks change policy or rate limits without much notice.",
        ],
      },
    ],
    limitations: [
      "UniPost cannot bypass native platform review, permissions, or content policy.",
      "White-label OAuth depends on each customer or tenant having approved platform credentials where required.",
      "Some advanced native features may need dedicated platform-specific support before they become unified fields.",
    ],
    faqs: [
      {
        q: "Is a publishing API different from a posting API?",
        a: "Posting is the API call. Publishing includes the broader workflow: account connection, media, scheduling, approvals, status, analytics, and webhooks.",
      },
      {
        q: "Can I embed this inside my SaaS?",
        a: "Yes. UniPost is designed for products that want customers to connect accounts and publish without leaving the product experience.",
      },
      {
        q: "Can I remove UniPost branding?",
        a: "Growth and higher plans support native or white-label flows for supported platforms where your own credentials are approved.",
      },
    ],
    primaryCta: { label: "Start free", href: "/welcome" },
    secondaryCta: { label: "See pricing", href: "/pricing" },
    relatedLinks: SHARED_RELATED_LINKS,
  },
];

export const SOLUTION_PAGES: SeoGrowthPage[] = [
  {
    slug: "social-media-scheduler-api",
    path: "/solutions/social-media-scheduler-api",
    title: "Social Media Scheduler API | UniPost",
    description:
      "Build a social media scheduler API with account connection, media upload, scheduled posts, delivery status, and webhook-based retry workflows.",
    eyebrow: "Solution",
    h1: "Social media scheduler API infrastructure",
    primaryQuery: "social media scheduler API",
    summary:
      "Use UniPost to power calendars, queues, approvals, retries, and scheduled publishing without rebuilding native social APIs.",
    proofPoints: ["Calendars", "Queues", "Scheduled posts", "Webhook status"],
    codeExample: MONEY_PAGES[1].codeExample,
    workflow: SHARED_WORKFLOW,
    sections: [
      {
        title: "Connect customer accounts",
        body: "A scheduler starts with account connection. Hosted Connect gives each customer a path to authorize destinations.",
        bullets: ["Store account_ids in your scheduler.", "Group destinations by customer workspace.", "Refresh and account events arrive through webhooks."],
      },
      {
        title: "Upload or reference media",
        body: "Media can be uploaded or referenced before a calendar item is published.",
        bullets: ["Validate asset readiness early.", "Preserve platform-specific variants.", "Avoid re-uploading unchanged assets."],
      },
      {
        title: "Publish or schedule posts",
        body: "Publish immediately or schedule future delivery through POST /v1/posts.",
        bullets: ["Attach scheduled_at to queued content.", "Use status updates to keep the calendar honest.", "Handle webhooks and errors in your queue worker."],
      },
    ],
    limitations: [
      "platform-specific differences still matter for video length, thumbnails, captions, and link rendering.",
      "Recurring queue logic belongs in your product unless you choose to model each occurrence as a UniPost post.",
    ],
    faqs: [
      { q: "Can UniPost power a calendar UI?", a: "Yes. UniPost handles publishing primitives while your product owns the calendar and approval UX." },
      { q: "Can I retry failed scheduled posts?", a: "Yes. Use status and webhook events to decide whether to retry, edit, or notify the customer." },
    ],
    primaryCta: { label: "Start building", href: "/welcome" },
    secondaryCta: { label: "Read posting docs", href: "/docs/api/posts/create" },
    relatedLinks: SHARED_RELATED_LINKS,
  },
  {
    slug: "ai-agent-social-posting",
    path: "/solutions/ai-agent-social-posting",
    title: "AI Agent Social Posting API | UniPost",
    description:
      "Let AI agents publish safely through a social posting API with connected account controls, MCP support, media handling, webhooks, and audit-friendly status.",
    eyebrow: "Solution",
    h1: "Social posting infrastructure for AI agents",
    primaryQuery: "AI agent social posting",
    summary:
      "Give agents a narrow publishing tool instead of raw social credentials. UniPost exposes posting primitives over REST and MCP so agent workflows can stay controllable.",
    proofPoints: ["Native MCP", "REST API", "Connected account controls", "Webhook audit trail"],
    codeExample: `// Agent tool calls your backend, your backend calls UniPost
await unipost.posts.create({
  caption: generatedDraft,
  account_ids: approvedAccountIds,
  media_ids: approvedMediaIds,
  metadata: { source: "agent_workflow" }
});`,
    workflow: SHARED_WORKFLOW,
    sections: [
      {
        title: "Connect customer accounts",
        body: "Keep OAuth and account ownership outside the agent. Your product decides which account_ids an agent may use.",
        bullets: ["Restrict agent tools to approved destinations.", "Use workspaces for tenant isolation.", "Avoid exposing native social tokens to the agent."],
      },
      {
        title: "Upload or reference media",
        body: "Agents can generate copy, but media should still pass through your approval and upload workflow.",
        bullets: ["Reference approved media IDs.", "Attach source metadata to generated posts.", "Validate media before publish time."],
      },
      {
        title: "Track results",
        body: "Track results through UniPost status, analytics, and webhooks so the agent can learn from real outcomes.",
        bullets: ["Log posted and failed events.", "Feed analytics back into planning.", "Handle webhooks and errors before the next autonomous action."],
      },
    ],
    limitations: [
      "AI agents should not receive native platform tokens or unrestricted posting permissions.",
      "platform-specific differences can change what an agent is allowed to publish on each network.",
    ],
    faqs: [
      { q: "Does UniPost support MCP?", a: "Yes. UniPost ships a native MCP server for agent-friendly posting workflows." },
      { q: "Should agents publish without review?", a: "That depends on your product risk model. UniPost provides the API primitives; your product should decide approvals and permissions." },
    ],
    primaryCta: { label: "Connect an agent", href: "/welcome" },
    secondaryCta: { label: "Open MCP docs", href: "/docs" },
    relatedLinks: SHARED_RELATED_LINKS,
  },
  {
    slug: "saas-social-publishing",
    path: "/solutions/saas-social-publishing",
    title: "Social Publishing API for SaaS | UniPost",
    description:
      "Embed social publishing in your SaaS product with hosted account connection, media workflows, scheduling, status tracking, and webhooks.",
    eyebrow: "Solution",
    h1: "Embedded social publishing for SaaS products",
    primaryQuery: "social media API for SaaS",
    summary:
      "Let customers connect social accounts and publish from your app while UniPost handles the cross-platform publishing infrastructure.",
    proofPoints: ["Embedded workflows", "Customer accounts", "Product-tier pricing", "White-label path"],
    codeExample: MONEY_PAGES[2].codeExample,
    workflow: SHARED_WORKFLOW,
    sections: [
      {
        title: "Connect customer accounts",
        body: "Create hosted connection sessions from your backend and attach returned account_ids to your own users.",
        bullets: ["Support multi-tenant workspaces.", "Keep customer account ownership clear.", "Use webhook events for disconnects and token issues."],
      },
      {
        title: "Publish or schedule posts",
        body: "A SaaS workflow often includes drafts, approvals, billing checks, and scheduled publishing before the final API call.",
        bullets: ["Publish only after your product approves the content.", "Attach metadata for audit trails.", "Track results in your own activity feed."],
      },
      {
        title: "Handle webhooks and errors",
        body: "Handle webhooks and errors centrally so support teams can explain platform-specific failures without reading native docs.",
        bullets: ["Show user-friendly failure reasons.", "Keep retry decisions in your backend.", "Record platform-specific differences in customer history."],
      },
    ],
    limitations: [
      "Your SaaS still owns tenant permissions, billing gates, and content approval policy.",
      "Native platform app review may be required when using customer-owned credentials.",
    ],
    faqs: [
      { q: "Can I embed account connection?", a: "Yes. Hosted Connect is designed for customer account connection inside SaaS products." },
      { q: "Can I bill my own customers separately?", a: "Yes. UniPost is infrastructure; your product owns customer packaging and pricing." },
    ],
    primaryCta: { label: "Build a SaaS workflow", href: "/welcome" },
    secondaryCta: { label: "See pricing", href: "/pricing" },
    relatedLinks: SHARED_RELATED_LINKS,
  },
  {
    slug: "white-label-social-media-api",
    path: "/solutions/white-label-social-media-api",
    title: "White Label Social Media API | UniPost",
    description:
      "Use a white-label social media API path with customer-owned credentials, hosted account connection, media upload, publishing, webhooks, and status tracking.",
    eyebrow: "Solution",
    h1: "White-label social media API for embedded products",
    primaryQuery: "white label social media API",
    summary:
      "Move from quickstart credentials to customer-owned or product-owned platform credentials so your users see your brand in the account connection flow.",
    proofPoints: ["Hosted Connect", "Native mode", "Platform credentials", "Attribution controls"],
    codeExample: `await fetch("https://api.unipost.dev/v1/connect/sessions", {
  method: "POST",
  headers: {
    "Authorization": \`Bearer \${UNIPOST_API_KEY}\`,
    "Content-Type": "application/json"
  },
  body: JSON.stringify({
    platform: "linkedin",
    mode: "native",
    redirect_url: "https://yourapp.example.com/connect/callback"
  })
});`,
    workflow: SHARED_WORKFLOW,
    sections: [
      {
        title: "Connect customer accounts",
        body: "White-label starts with the account connection screen. UniPost supports native mode for supported platforms when credentials are approved.",
        bullets: ["Bring your own developer credentials.", "Use Hosted Connect to complete OAuth.", "Store account_ids after callback."],
      },
      {
        title: "Upload or reference media",
        body: "Your product keeps the branded UX while UniPost still handles media upload workflow and validation.",
        bullets: ["Keep media in your app or CDN.", "Send media IDs or URLs to UniPost.", "Explain platform-specific differences in your UI."],
      },
      {
        title: "Track results",
        body: "Track results through status endpoints and webhooks while your users stay inside your product.",
        bullets: ["Handle webhooks and errors in your backend.", "Show posted, failed, and scheduled states.", "Link users to support paths when credentials need review."],
      },
    ],
    limitations: [
      "White-label depends on native platform app review and credential approval.",
      "Some platforms may still show platform-owned consent wording that cannot be fully customized.",
      "Quickstart mode is faster for testing; native mode is the production path for branded OAuth.",
    ],
    faqs: [
      { q: "Does white-label mean no platform review?", a: "No. White-label usually means your approved credentials are used, and native platform rules still apply." },
      { q: "Can I start before my own credentials are approved?", a: "Yes. Quickstart mode helps you test while you prepare native app review." },
    ],
    primaryCta: { label: "Start native mode", href: "/welcome" },
    secondaryCta: { label: "Compare vendors", href: "/compare/social-media-apis" },
    relatedLinks: SHARED_RELATED_LINKS,
  },
];

export function getMoneyPage(slug: string) {
  return MONEY_PAGES.find((page) => page.slug === slug);
}

export function getSolutionPage(slug: string) {
  return SOLUTION_PAGES.find((page) => page.slug === slug);
}
