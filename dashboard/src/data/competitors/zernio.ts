// Zernio competitor data
// Last verified: 2026-06-23 against zernio.com/social-media-api + zernio.com/blog/pay-per-account-pricing
// Pricing evidence: docs/competitive-evidence/zernio-pricing-2026-06-21.md
// Per-platform flags below are best-effort. Zernio's public pricing page
// lists the major social, messaging, and ads networks it supports.
// Verify current coverage before quoting platform-specific gaps in marketing.

export const ZERNIO = {
  name: "Zernio",
  slug: "zernio",
  tagline: "Pay-per-account social API with every feature bundled",
  pricing: {
    freeTier: true,
    freePostsPerMonth: 0, // Account-based free tier; use freeTierLabel for display.
    freeTierLabel: "first 2 connected accounts free",
    startingPrice: 6, // First paid band after the 2-account credit: $6/account/mo.
    pricingModel: "Pay per connected social account; first 2 free, then graduated rates",
    enterprisePlan: true,
    tiers: [
      { label: "First 2 accounts", price: 0, posts: "Unlimited", priceDisplay: "Free" },
      { label: "Accounts 3-10", price: 6, posts: "Unlimited", priceDisplay: "$6/account/mo" },
      { label: "Accounts 11-100", price: 3, posts: "Unlimited", priceDisplay: "$3/account/mo" },
      { label: "Accounts 101-2,000", price: 1, posts: "Unlimited", priceDisplay: "$1/account/mo" },
      { label: "2,001+ accounts", price: null, posts: "Unlimited", priceDisplay: "Custom" },
    ],
    addOns: [],
  },
  platforms: {
    total: 15, // homepage advertises 15 (Instagram, TikTok, YouTube, X, LinkedIn, Facebook, Threads, Pinterest, Reddit, Bluesky, WhatsApp, Telegram, Discord, Snapchat, Google Business)
    x: true,
    bluesky: true, // confirmed via homepage + MCP docs
    linkedin: true,
    instagram: true,
    threads: true,
    tiktok: true,
    youtube: true,
    facebook: true,
    pinterest: true,
  } as Record<string, boolean | string | number>,
  features: {
    scheduledPosts: true,
    postAnalytics: true, // included with connected accounts per current public pricing
    webhooks: true,
    mediaUpload: true,
    twitterThreads: true,
    bulkPublishing: true,
    mcpServer: true, // mcp.zernio.com — full MCP-protocol server, 280+ auto-generated tools
    firstComment: false,
    nativeMode: false, // only OAuth 2.1 verified; no white-label / branded OAuth advertised
    quickstartMode: false,
    inbox: true, // included with connected accounts per current public pricing
    dashboard: true,
  },
  developerExperience: {
    restApi: true,
    sdk: true, // 8 official SDKs: Node.js, Python, Go, Ruby, Java, PHP, .NET, Rust
    docsQuality: 4,
    mcpServer: true,
    openSource: false,
  },
  compliance: {
    soc2: false, // no public security/compliance/trust page on zernio.com
    gdpr: false, // no explicit public GDPR claim
  },
  heroTitle: "UniPost vs Zernio —\nEmbedded publishing without the connected-account tax",
  heroSub: "Zernio's public pricing is pay per connected social account. UniPost self-serve plans are based on product tier and monthly post capacity, so embedded apps with many low-volume users can avoid account-meter pricing.",
  verdict: {
    chooseUs: [
      "You are building an app where your own users connect social accounts",
      "You want self-serve pricing based on product tier and monthly post capacity, not connected-account count",
      "Your users are low- or medium-volume, so Growth can be more predictable than paying per connected account",
      "You want branded OAuth through Hosted Connect — UniPost ships this on Growth ($59/mo)",
    ],
    chooseThem: [
      "You need 15 platforms and use ones UniPost doesn't have yet (Reddit, WhatsApp, Telegram, Discord, Snapchat, Google Business)",
      "You need Ads management as a first-class feature",
      "You want unlimited posts and every feature bundled with each connected account",
      "Your connected-account count is low enough that per-account billing is predictable",
    ],
  },
  bestFit: {
    unipost: "Embedded apps that want product-tier pricing instead of a bill that grows with every connected social account.",
    competitor: "Teams that need 15 platforms, unlimited posts, ads, and a pay-per-connected social account model.",
  },
  sourceLinks: [
    { label: "Zernio social media API", url: "https://zernio.com/social-media-api" },
    { label: "Zernio pay-per-account pricing", url: "https://zernio.com/blog/pay-per-account-pricing" },
    { label: "Zernio docs", url: "https://docs.zernio.com/" },
  ],
  migrationEndpoint: {
    from: "api.zernio.com/v1/posts",
    to: "api.unipost.dev/v1/posts",
  },
  migrationFields: {
    from: 'channels: ["instagram"]',
    to: 'account_ids: ["sa_instagram_xxx"]',
  },
  faqs: [
    { q: "Can I use UniPost if I'm already using Zernio?", a: "Yes. You can run both in parallel during migration. UniPost's Free plan gives you 100 posts/month to test the API and dashboard, while paid plans are organized around product stage and monthly post capacity." },
    { q: "What's the real-money difference for an embedded app?", a: "If your app has 100 end users and each connects 2 social accounts, that is 200 connected accounts. Under Zernio's current graduated account pricing, 200 full-month accounts is $418/mo. If the same app fits under 7,500 posts/mo and Growth features, UniPost Growth is $59/mo." },
    { q: "Does UniPost support all the platforms Zernio supports?", a: "UniPost supports 9 platforms (X, Instagram, Facebook, Threads, TikTok, YouTube, LinkedIn, Pinterest, Bluesky). Zernio advertises 15. If you need a network beyond UniPost's nine, check our roadmap or contact us." },
    { q: "Both have MCP — what's the difference?", a: "Both ship MCP support. The deciding factor is usually packaging: UniPost self-serve plans are not priced per connected social account, while Zernio's public pricing uses an account meter." },
    { q: "Does UniPost have a free plan?", a: "Yes — the Free plan includes 100 posts/month, no credit card, and no time limit. Paid plans do not include a separate time-limited trial." },
    { q: "Does UniPost support white-label OAuth?", a: "Yes — UniPost ships white-label (Native mode) on the Growth tier ($59/mo) so your users see your app name on the OAuth screen. Zernio's public docs describe standard OAuth flows; verify directly with Zernio if branded OAuth is a requirement." },
    { q: "How long does migration take?", a: "Most developers complete the switch in under an hour. The main change is the endpoint URL and field names." },
  ],
  seo: {
    title: "UniPost vs Zernio — Embedded Social API Comparison (2026) | UniPost",
    description: "Comparing Zernio alternatives? UniPost self-serve plans are priced by product stage and monthly post capacity, not per connected social account.",
    keywords: ["zernio alternative", "zernio competitor", "zernio vs unipost", "social media api comparison"],
    ogTitle: "UniPost vs Zernio — Embedded Social API Comparison",
    ogDescription: "No connected-account tax on self-serve plans. Compare UniPost and Zernio.",
  },
};
