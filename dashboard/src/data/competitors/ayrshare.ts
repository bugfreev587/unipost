// Ayrshare competitor data
// Last verified: April 2026
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
  heroSub: "Looking for an Ayrshare alternative? UniPost offers unified social media API with a free tier, simpler pricing, and native MCP Server support for AI agents.",
  verdict: {
    chooseUs: [
      "You want a free tier to start",
      "You're building AI agents (MCP)",
      "You prefer simple per-post pricing",
      "You're a solo dev or small team",
    ],
    chooseThem: [
      "You need 15+ platform coverage",
      "You have enterprise compliance needs",
      "You need white-label reseller plan",
    ],
  },
  migrationEndpoint: {
    from: "api.ayrshare.com/api/post",
    to: "api.unipost.dev/v1/social-posts",
  },
  migrationFields: {
    from: 'platforms: ["instagram"]',
    to: 'account_ids: ["sa_instagram_xxx"]',
  },
  faqs: [
    { q: "Can I use UniPost if I'm already using Ayrshare?", a: "Yes. You can run both in parallel during migration. UniPost's free tier lets you test without committing." },
    { q: "Does UniPost support all the platforms Ayrshare supports?", a: "UniPost currently supports 7 platforms. Ayrshare supports 15+. If you need Facebook, Pinterest, or other platforms, check our roadmap or contact us." },
    { q: "Is UniPost's API compatible with Ayrshare?", a: "The API is similar in concept but not drop-in compatible. Field names and endpoint paths differ. See the migration guide above for the exact changes needed." },
    { q: "Does UniPost have a free trial?", a: "Yes — the Free plan (100 posts/month) is permanent, not a time-limited trial. No credit card required." },
    { q: "What about Ayrshare's white-label features?", a: "UniPost supports White-label (Native mode) on all paid plans, which gives your users a branded OAuth experience. Custom-domain white-labeling is on the roadmap." },
    { q: "How long does migration take?", a: "Most developers complete the switch in under an hour. The main change is the endpoint URL and field names." },
  ],
  seo: {
    title: "Best Ayrshare Alternative for Developers (2026) | UniPost",
    description: "Looking for an Ayrshare alternative? UniPost offers unified social media API with a free tier, 7 platforms, and native MCP Server. Compare features and pricing.",
    keywords: ["ayrshare alternative", "ayrshare competitor", "ayrshare vs unipost", "social media api alternative", "unified social media api"],
    ogTitle: "UniPost vs Ayrshare — Social Media API Comparison",
    ogDescription: "Free tier. 7 platforms. Native MCP Server. Compare UniPost and Ayrshare.",
  },
};
