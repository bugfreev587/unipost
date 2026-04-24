// Zernio competitor data
// Last verified: April 2026
// Source: zernio.com/pricing
// Verify quarterly before updating

export const ZERNIO = {
  name: "Zernio",
  slug: "zernio",
  tagline: "Social media management API platform",
  pricing: {
    freeTier: false,
    freePostsPerMonth: 0,
    startingPrice: 39,
    pricingModel: "Per feature tier",
    enterprisePlan: true,
    tiers: [
      { label: "Starter", price: 39, posts: "Included" },
      { label: "Growth", price: 99, posts: "Higher volume" },
      { label: "Enterprise", price: null, posts: "Custom" },
    ],
  },
  platforms: {
    total: 14,
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
    mcpServer: true,
    firstComment: false,
    nativeMode: true,
    quickstartMode: false,
  },
  developerExperience: {
    restApi: true,
    sdk: true,
    docsQuality: 4,
    mcpServer: true,
    openSource: false,
  },
  compliance: {
    soc2: true,
    gdpr: true,
  },
  heroTitle: "UniPost vs Zernio — Which\nsocial API is right for you?",
  heroSub: "Comparing Zernio alternatives? UniPost offers a permanent free tier, simpler per-post pricing, and developer-first design for solo devs and small teams.",
  verdict: {
    chooseUs: [
      "You want a free tier to start",
      "You prefer simple per-post pricing",
      "You're a solo dev or indie hacker",
      "You want Bluesky support",
    ],
    chooseThem: [
      "You need 14+ platform coverage",
      "You have enterprise clients (ClickUp, PwC-level)",
      "You need a mature, battle-tested product",
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
    { q: "Can I use UniPost if I'm already using Zernio?", a: "Yes. You can run both in parallel during migration. UniPost's free tier lets you test without any financial commitment." },
    { q: "Does UniPost support all the platforms Zernio supports?", a: "UniPost currently supports 7 platforms. Zernio supports 14. If you need Facebook, Pinterest, or other platforms, check our roadmap or contact us." },
    { q: "Both have MCP — what's the difference?", a: "UniPost's MCP Server is a core feature designed for developer API workflows. Zernio offers MCP as an add-on. Both let AI agents post on behalf of users." },
    { q: "Does UniPost have a free trial?", a: "Yes — the Free plan (100 posts/month) is permanent, not a time-limited trial. No credit card required." },
    { q: "Is Zernio better for enterprise?", a: "Zernio has more enterprise features (SOC 2, larger platform coverage, enterprise clients). UniPost is better suited for solo developers and small teams who want simple pricing and a free tier." },
    { q: "How long does migration take?", a: "Most developers complete the switch in under an hour. The main change is the endpoint URL and field names." },
  ],
  seo: {
    title: "UniPost vs Zernio — Social API Comparison (2026) | UniPost",
    description: "Comparing Zernio alternatives? UniPost offers a free tier, 7 platforms, simple pricing, and native MCP Server. See the full feature comparison.",
    keywords: ["zernio alternative", "zernio competitor", "zernio vs unipost", "social media api comparison"],
    ogTitle: "UniPost vs Zernio — Social Media API Comparison",
    ogDescription: "Free tier. 7 platforms. Simpler pricing. Compare UniPost and Zernio.",
  },
};
