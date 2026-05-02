// Zernio competitor data
// Last verified: 2026-05-01 against zernio.com (homepage, pricing) + docs.zernio.com
// Total platform count and Free-tier presence are user-confirmed.
// Per-platform flags below are best-effort — Zernio's site advertises
// the count but not always an itemized list, so flags are based on the
// major networks Zernio publicly supports. Verify before quoting in marketing.

export const ZERNIO = {
  name: "Zernio",
  slug: "zernio",
  tagline: "Social media platform with tiered features and add-ons",
  pricing: {
    freeTier: true,
    freePostsPerMonth: 20, // Zernio Free: 2 social sets, 20 posts/mo, no Tools API
    startingPrice: 19, // Build tier: $19/mo monthly billing ($16/mo billed yearly)
    pricingModel: "Tier + add-ons (Analytics, Comments+DMs, Ads each charged separately)",
    enterprisePlan: true,
    tiers: [
      { label: "Free",        price: 0,    posts: "20/mo" },
      { label: "Build",       price: 19,   posts: "120/mo" },
      { label: "Accelerate",  price: 49,   posts: "Unlimited" },
      { label: "Unlimited",   price: 833,  posts: "Unlimited" },
    ],
    addOns: [
      "Analytics ($9/mo on Build, $42/mo on Accelerate, $833/mo on Unlimited)",
      "Comments + DMs (same per-tier pricing as Analytics)",
      "Ads (same per-tier pricing as Analytics)",
    ],
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
    postAnalytics: true, // available as a paid add-on, not bundled
    webhooks: true,
    mediaUpload: true,
    twitterThreads: true,
    bulkPublishing: true,
    mcpServer: true, // mcp.zernio.com — full MCP-protocol server, 280+ auto-generated tools
    firstComment: false,
    nativeMode: false, // only OAuth 2.1 verified; no white-label / branded OAuth advertised
    quickstartMode: false,
    inbox: true, // Comments + DMs add-on
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
  heroTitle: "UniPost vs Zernio —\nThe full stack without the add-on tax",
  heroSub: "Zernio sells Analytics and Comments+DMs as separate paid add-ons on top of its tier price. UniPost bundles them into Basic at $19/mo.",
  verdict: {
    chooseUs: [
      "You want Inbox and Analytics included, not as add-ons",
      "You want native MCP support without an upgrade or Tools API gating",
      "You're a solo dev or indie hacker who wants more headroom on the free tier (100 posts vs Zernio's 20)",
      "You want a branded OAuth (white-label) experience — UniPost ships this on Growth ($59/mo)",
    ],
    chooseThem: [
      "You need 15 platforms and use ones UniPost doesn't have yet (Reddit, WhatsApp, Telegram, Discord, Snapchat, Google Business)",
      "You need Ads management as a first-class feature",
      "You have enterprise-scale volume needs (Unlimited tier at $833/mo)",
    ],
  },
  migrationEndpoint: {
    from: "api.zernio.com/v1/posts",
    to: "api.unipost.dev/v1/posts",
  },
  migrationFields: {
    from: 'channels: ["instagram"]',
    to: 'account_ids: ["sa_instagram_xxx"]',
  },
  faqs: [
    { q: "Can I use UniPost if I'm already using Zernio?", a: "Yes. You can run both in parallel during migration. UniPost's Free plan (100 posts/mo) gives you more headroom than Zernio's Free (20 posts/mo) to evaluate." },
    { q: "What's the real-money difference at the entry tier?", a: "Zernio Build is $19/mo and excludes Inbox and Analytics — you add those as $9/mo each, bringing the realistic total to ~$37/mo. UniPost Basic is $19/mo with both bundled in. Same price, more value." },
    { q: "Does UniPost support all the platforms Zernio supports?", a: "UniPost supports 9 platforms (X, Instagram, Facebook, Threads, TikTok, YouTube, LinkedIn, Pinterest, Bluesky). Zernio advertises 15. If you need a network beyond UniPost's nine, check our roadmap or contact us." },
    { q: "Both have MCP — what's the difference?", a: "UniPost ships MCP as a core feature on every plan including Free. Zernio's MCP server (Tools API) is gated by tier. Same protocol, different packaging." },
    { q: "Does UniPost have a free trial?", a: "Yes — the Free plan (100 posts/month) is permanent, not a time-limited trial. No credit card required. Paid plans (Basic and up) include a 14-day free trial when you upgrade." },
    { q: "Does UniPost support white-label OAuth?", a: "Yes — UniPost ships white-label (Native mode) on the Growth tier ($59/mo) so your users see your app name on the OAuth screen. Zernio's docs only describe standard OAuth 2.1, not branded white-label." },
    { q: "How long does migration take?", a: "Most developers complete the switch in under an hour. The main change is the endpoint URL and field names." },
  ],
  seo: {
    title: "UniPost vs Zernio — Social API Comparison (2026) | UniPost",
    description: "Comparing Zernio alternatives? UniPost Basic ($19/mo) bundles Inbox and Analytics — Zernio sells them as $9/mo add-ons each. See the full comparison.",
    keywords: ["zernio alternative", "zernio competitor", "zernio vs unipost", "social media api comparison"],
    ogTitle: "UniPost vs Zernio — Social Media API Comparison",
    ogDescription: "Inbox and Analytics included. No add-on tax. Compare UniPost and Zernio.",
  },
};
