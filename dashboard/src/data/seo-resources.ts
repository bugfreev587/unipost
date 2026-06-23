export type SeoResource = {
  slug: string;
  title: string;
  description: string;
  eyebrow: string;
  h1: string;
  summary: string;
  lastVerified: string;
  sections: Array<{
    title: string;
    body: string;
    rows: Array<{
      label: string;
      value: string;
      note: string;
    }>;
  }>;
  faqs: Array<{
    q: string;
    a: string;
  }>;
};

export const SEO_RESOURCES: SeoResource[] = [
  {
    slug: "social-media-api-platform-requirements",
    title: "Social Media API Platform Requirements Matrix | UniPost",
    description:
      "A practical matrix of platform requirements for social media APIs, including OAuth, app review, media, analytics, and webhook considerations.",
    eyebrow: "Platform Matrix",
    h1: "Social media API platform requirements matrix",
    summary:
      "Use this matrix to plan which native platform requirements still matter when you build on top of a unified social media API.",
    lastVerified: "2026-06-23",
    sections: [
      {
        title: "Requirements by platform",
        body: "Every social network has a different mix of OAuth, app review, media, and analytics requirements.",
        rows: [
          { label: "X", value: "Paid API and OAuth requirements", note: "Expect stricter access and rate limits than most networks." },
          { label: "Instagram", value: "Meta app review", note: "Business or creator account requirements can affect publish eligibility." },
          { label: "LinkedIn", value: "LinkedIn app permissions", note: "Organization posting and member posting may require different scopes." },
          { label: "TikTok", value: "App review and content scopes", note: "Video publishing and analytics are permission-sensitive." },
          { label: "YouTube", value: "Google OAuth scopes", note: "Uploads, Shorts, and analytics can require separate permissions." },
        ],
      },
    ],
    faqs: [
      {
        q: "Does a unified API remove platform review?",
        a: "No. A unified API reduces integration work, but native platforms still control app review, scopes, rate limits, and policy enforcement.",
      },
      {
        q: "How should I use this matrix?",
        a: "Use it to decide which platforms can start in quickstart mode and which ones need customer-owned credentials or native review before launch.",
      },
    ],
  },
  {
    slug: "platform-posting-constraints",
    title: "Platform Posting Constraints by Network | UniPost",
    description:
      "A reference for platform posting constraints across social networks, including media type, caption, scheduling, and post-type differences.",
    eyebrow: "Posting Constraints",
    h1: "Platform posting constraints by network",
    summary:
      "Unified APIs help normalize the workflow, but product teams still need to respect each network's media, caption, and post-type rules.",
    lastVerified: "2026-06-23",
    sections: [
      {
        title: "Common constraint categories",
        body: "Most publish failures come from media shape, post type, account eligibility, or permission scope mismatches.",
        rows: [
          { label: "Images", value: "Aspect ratio and count limits", note: "Carousels and multi-image posts vary by platform." },
          { label: "Video", value: "Duration, size, codec, and thumbnail limits", note: "Short-form and feed video paths may differ." },
          { label: "Captions", value: "Length, mentions, hashtags, and link previews", note: "Some platforms rewrite or suppress previews." },
          { label: "Scheduling", value: "Immediate vs future publish support", note: "Some networks support scheduling through the API, others need provider-side queues." },
          { label: "Account type", value: "Business, creator, page, or organization", note: "Eligibility can differ even inside one platform." },
        ],
      },
    ],
    faqs: [
      {
        q: "Can UniPost make every network accept the same payload?",
        a: "UniPost normalizes the API shape, but each native network still decides what content and account types are valid.",
      },
      {
        q: "Where should validation happen?",
        a: "Validate early in your product where possible, then rely on UniPost status and webhooks for final delivery outcomes.",
      },
    ],
  },
  {
    slug: "social-media-oauth-app-review",
    title: "Social Media OAuth and App Review Requirements | UniPost",
    description:
      "A guide to OAuth and app review requirements for products that connect customer social accounts through a unified API.",
    eyebrow: "OAuth and Review",
    h1: "OAuth and app review requirements by platform",
    summary:
      "Account connection is usually the hardest part of social publishing. This guide explains what to plan for before production launch.",
    lastVerified: "2026-06-23",
    sections: [
      {
        title: "Planning checklist",
        body: "Plan OAuth and app review alongside product design, not after the first customer asks for a production connection.",
        rows: [
          { label: "Redirect URLs", value: "Register production and dev callback URLs", note: "Keep app, staging, and dev domains explicit." },
          { label: "Scopes", value: "Request only what the workflow needs", note: "Over-broad scopes slow review and create trust issues." },
          { label: "Demo content", value: "Prepare review videos and screenshots", note: "Platforms often need proof of how permissions are used." },
          { label: "Token storage", value: "Keep native tokens out of browsers", note: "UniPost handles this in hosted connection flows." },
          { label: "Fallback path", value: "Use quickstart while native review is pending", note: "Do this only where quickstart mode fits the customer experience." },
        ],
      },
    ],
    faqs: [
      {
        q: "Can I use UniPost credentials forever?",
        a: "Quickstart credentials are useful for testing and some hosted flows, but white-label production often needs your own approved platform credentials.",
      },
      {
        q: "Who owns platform review?",
        a: "The app owner usually owns review artifacts and platform approval. UniPost provides the connection and publishing infrastructure around that process.",
      },
    ],
  },
  {
    slug: "media-upload-limits",
    title: "Social Media API Media Upload Limits and Workflow | UniPost",
    description:
      "Compare media upload workflows for unified social media APIs, including public URLs, reserved media IDs, direct upload, and platform validation.",
    eyebrow: "Media Workflow",
    h1: "Media upload limits and workflow comparison",
    summary:
      "Media is where social API integrations get messy. A unified API should centralize upload, validation, publish, and failure feedback.",
    lastVerified: "2026-06-23",
    sections: [
      {
        title: "Workflow options",
        body: "Choose the workflow that matches how your product stores and approves media.",
        rows: [
          { label: "Public URL", value: "Fastest integration", note: "Works when your assets are already available to UniPost." },
          { label: "Reserved media ID", value: "Best for app-owned uploads", note: "Upload once, reuse in publish calls." },
          { label: "Direct platform upload", value: "Native complexity", note: "Usually requires separate SDKs, retries, and format checks." },
          { label: "Per-platform variant", value: "Best quality control", note: "Use when networks need different crops, thumbnails, or captions." },
          { label: "Webhook outcome", value: "Operational visibility", note: "Record when media validation or publish delivery fails." },
        ],
      },
    ],
    faqs: [
      {
        q: "Should I upload media to UniPost or use URLs?",
        a: "Use URLs for quick integration and media IDs when your product needs stronger control over upload lifecycle and reuse.",
      },
      {
        q: "Can one video work everywhere?",
        a: "Sometimes, but duration, aspect ratio, file size, and content type constraints vary by platform.",
      },
    ],
  },
  {
    slug: "unified-api-cost-calculator",
    title: "Unified API vs Native Social API Engineering Cost Calculator | UniPost",
    description:
      "Estimate the engineering cost of building native social API integrations versus using a unified social media publishing API.",
    eyebrow: "Cost Calculator",
    h1: "Unified API vs native API engineering cost calculator",
    summary:
      "Use this calculator model to estimate whether your team should build native platform integrations or buy a unified API.",
    lastVerified: "2026-06-23",
    sections: [
      {
        title: "Cost model",
        body: "The core tradeoff is not only first build cost. It is also review, maintenance, retries, policy changes, and support work.",
        rows: [
          { label: "Initial integration", value: "2-6 weeks per major platform", note: "Depends on OAuth, media, scheduling, and analytics depth." },
          { label: "App review", value: "Days to weeks", note: "Requires demos, test users, screenshots, and policy explanations." },
          { label: "Maintenance", value: "Ongoing", note: "Native APIs change scopes, limits, and behaviors over time." },
          { label: "Support burden", value: "Customer-facing", note: "Every platform error becomes a support and product education problem." },
          { label: "Unified API", value: "One integration plus vendor cost", note: "Best when publishing is important but not your core infrastructure advantage." },
        ],
      },
    ],
    faqs: [
      {
        q: "When should I build native integrations?",
        a: "Build native integrations when platform depth is your core product advantage or when you need features a unified API does not expose.",
      },
      {
        q: "When should I use a unified API?",
        a: "Use a unified API when your goal is to ship reliable social publishing, not maintain every platform integration yourself.",
      },
    ],
  },
];

export function getSeoResource(slug: string) {
  return SEO_RESOURCES.find((resource) => resource.slug === slug);
}
