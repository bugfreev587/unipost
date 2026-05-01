// Ayrshare competitor data
// Last verified: 2026-05-01
// Source: ayrshare.com/pricing
// Verify quarterly before updating

export const AYRSHARE = {
  name: "Ayrshare",
  slug: "ayrshare",
  tagline: "Social media API for posting, scheduling, and analytics",
  pricing: {
    freeTier: false,
    freePostsPerMonth: 0,
    startingPrice: 29,
    pricingModel: "Per post volume",
    enterprisePlan: true,
    tiers: [
      { label: "Premium", price: 29, posts: "Included" },
      { label: "Business", price: 89, posts: "Higher volume" },
      { label: "Enterprise", price: null, posts: "Custom" },
    ],
  },
  platforms: {
    total: "15+",
    x: true,
    bluesky: false,
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
    postAnalytics: true,
    webhooks: true,
    mediaUpload: true,
    twitterThreads: true,
    bulkPublishing: true,
    mcpServer: false,
    firstComment: false,
    nativeMode: true,
    quickstartMode: true,
    inbox: true,
    dashboard: true,
  },
  developerExperience: {
    restApi: true,
    sdk: true,
    docsQuality: 5,
    mcpServer: false,
    openSource: false,
  },
  compliance: {
    soc2: true,
    gdpr: true,
  },
  heroTitle: "The best Ayrshare alternative\nfor developers in 2026",
  heroSub: "Looking for an Ayrshare alternative? UniPost has a permanent free tier, native MCP for AI agents, and bundles Inbox + Analytics into Basic at $19/mo.",
  verdict: {
    chooseUs: [
      "You want a permanent free tier (Ayrshare starts at $29/mo)",
      "You're building AI agents (UniPost ships native MCP)",
      "You want a $10/mo API-only plan (UniPost API tier)",
      "You're a solo dev or small team",
    ],
    chooseThem: [
      "You need 15+ platform coverage",
      "You have enterprise compliance needs",
      "You need a white-label reseller plan",
    ],
  },
  migrationEndpoint: {
    from: "api.ayrshare.com/api/post",
    to: "api.unipost.dev/v1/posts",
  },
  migrationFields: {
    from: 'platforms: ["instagram"]',
    to: 'account_ids: ["sa_instagram_xxx"]',
  },
  faqs: [
    { q: "Can I use UniPost if I'm already using Ayrshare?", a: "Yes. You can run both in parallel during migration. UniPost's Free plan lets you test without committing." },
    { q: "Does UniPost support all the platforms Ayrshare supports?", a: "UniPost currently supports 9 platforms (X, Instagram, Facebook, Threads, TikTok, YouTube, LinkedIn, Pinterest, Bluesky). Ayrshare supports 15+. If you need a network beyond UniPost's nine, check our roadmap or contact us." },
    { q: "Is UniPost's API compatible with Ayrshare?", a: "The API is similar in concept but not drop-in compatible. Field names and endpoint paths differ. See the migration guide above for the exact changes needed." },
    { q: "Does UniPost have a free trial?", a: "Yes — the Free plan (100 posts/month) is permanent, not a time-limited trial. No credit card required. X publishing requires any paid plan; the other 8 platforms are available on Free." },
    { q: "What about Ayrshare's white-label features?", a: "UniPost supports White-label (Native mode) starting at the Growth tier ($59/mo), which gives your users a branded OAuth experience. Custom-domain white-labeling is on the roadmap." },
    { q: "How long does migration take?", a: "Most developers complete the switch in under an hour. The main change is the endpoint URL and field names." },
  ],
  seo: {
    title: "Best Ayrshare Alternative for Developers (2026) | UniPost",
    description: "Looking for an Ayrshare alternative? UniPost offers a permanent free tier, native MCP Server, and bundles Inbox + Analytics. Compare features and pricing.",
    keywords: ["ayrshare alternative", "ayrshare competitor", "ayrshare vs unipost", "social media api alternative", "unified social media api"],
    ogTitle: "UniPost vs Ayrshare — Social Media API Comparison",
    ogDescription: "Permanent free tier. Native MCP. Bundled Inbox + Analytics. Compare UniPost and Ayrshare.",
  },
};
