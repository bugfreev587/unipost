// PostForMe competitor data
// Last verified: April 2026
// Source: postforme.dev
// Verify quarterly before updating

export const POSTFORME = {
  name: "PostForMe",
  slug: "postforme",
  tagline: "Open-source social media posting API",
  pricing: {
    freeTier: false,
    freePostsPerMonth: 0,
    startingPrice: 19,
    pricingModel: "Per feature tier",
    enterprisePlan: false,
    tiers: [
      { label: "Starter", price: 19, posts: "Included" },
      { label: "Pro", price: 49, posts: "Higher volume" },
    ],
  },
  platforms: {
    total: 6,
    x: false,
    bluesky: true,
    linkedin: true,
    instagram: true,
    threads: true,
    tiktok: true,
    youtube: true,
    facebook: false,
    pinterest: false,
  } as Record<string, boolean | string | number>,
  features: {
    scheduledPosts: true,
    postAnalytics: false,
    webhooks: false,
    mediaUpload: true,
    twitterThreads: false,
    bulkPublishing: false,
    mcpServer: false,
    firstComment: false,
    nativeMode: false,
    quickstartMode: true,
  },
  developerExperience: {
    restApi: true,
    sdk: false,
    docsQuality: 3,
    mcpServer: false,
    openSource: true,
  },
  compliance: {
    soc2: false,
    gdpr: true,
  },
  heroTitle: "UniPost vs PostForMe —\nA developer's comparison",
  heroSub: "Evaluating PostForMe alternatives? UniPost offers more platforms, a free tier, analytics, webhooks, and native MCP Server support — all without self-hosting.",
  verdict: {
    chooseUs: [
      "You want a free tier with no self-hosting",
      "You need X/Twitter support",
      "You need analytics and webhooks",
      "You're building AI agents (MCP)",
    ],
    chooseThem: [
      "You want to self-host and own the code",
      "You prefer open-source solutions",
      "You have a simple posting-only use case",
    ],
  },
  migrationEndpoint: {
    from: "api.postforme.dev/v1/publish",
    to: "api.unipost.dev/v1/social-posts",
  },
  migrationFields: {
    from: 'networks: ["instagram"]',
    to: 'account_ids: ["sa_instagram_xxx"]',
  },
  faqs: [
    { q: "Can I use UniPost if I'm already using PostForMe?", a: "Yes. You can run both in parallel during migration. UniPost's free tier lets you test without any financial commitment." },
    { q: "PostForMe is open-source. Is UniPost?", a: "No. UniPost is a managed SaaS product. You get reliability, uptime, and zero infrastructure management. If you need open-source, PostForMe is a good choice." },
    { q: "Does UniPost support X/Twitter?", a: "Yes. UniPost supports X/Twitter along with 6 other platforms. PostForMe does not currently support X/Twitter." },
    { q: "Does UniPost have a free trial?", a: "Yes — the Free plan (100 posts/month) is permanent, not a time-limited trial. No credit card required." },
    { q: "Is PostForMe really free?", a: "PostForMe is open-source but their hosted version requires a paid plan starting at $19/month. Self-hosting is free but requires your own infrastructure." },
    { q: "How long does migration take?", a: "Most developers complete the switch in under an hour. The main change is the endpoint URL and field names." },
  ],
  seo: {
    title: "UniPost vs PostForMe — Social API Comparison (2026) | UniPost",
    description: "Comparing PostForMe alternatives? UniPost offers 7 platforms, a free tier, analytics, webhooks, and native MCP Server. See the full comparison.",
    keywords: ["postforme alternative", "postforme competitor", "postforme vs unipost", "social media api comparison"],
    ogTitle: "UniPost vs PostForMe — Social Media API Comparison",
    ogDescription: "Free tier. 7 platforms. More features. Compare UniPost and PostForMe.",
  },
};
