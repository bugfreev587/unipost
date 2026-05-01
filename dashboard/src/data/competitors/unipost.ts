// UniPost's own data for comparison tables
// Last verified: 2026-05-01 against unipost.dev/pricing (after the
// product-tier pricing redesign — migration 058)

export const UNIPOST = {
  name: "UniPost",
  slug: "unipost",
  tagline: "Unified social media API + dashboard + Inbox + Analytics",
  pricing: {
    freeTier: true,
    freePostsPerMonth: 100,
    startingPrice: 10,
    pricingModel: "Product tier + monthly capacity",
    enterprisePlan: true,
    tiers: [
      { label: "Free",       price: 0,    posts: "100/mo" },
      { label: "API",        price: 10,   posts: "1,000/mo" },
      { label: "Basic",      price: 19,   posts: "2,500/mo" },
      { label: "Growth",     price: 59,   posts: "7,500/mo" },
      { label: "Team",       price: 149,  posts: "25,000/mo" },
      { label: "Enterprise", price: null, posts: "Custom" },
    ],
  },
  platforms: {
    total: 9,
    x: true,
    bluesky: true,
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
    firstComment: true,
    nativeMode: true,
    quickstartMode: true,
    inbox: true,
    dashboard: true,
  },
  developerExperience: {
    restApi: true,
    sdk: "coming",
    docsQuality: 4,
    mcpServer: true,
    openSource: false,
  },
  compliance: {
    soc2: "coming",
    gdpr: true,
  },
};
