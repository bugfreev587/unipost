// UniPost's own data for comparison tables
// Last verified: April 2026
// Source: unipost.dev/pricing

export const UNIPOST = {
  name: "UniPost",
  slug: "unipost",
  tagline: "Unified social media API for developers",
  pricing: {
    freeTier: true,
    freePostsPerMonth: 100,
    startingPrice: 10,
    pricingModel: "Per post volume",
    enterprisePlan: true,
    tiers: [
      { label: "Free", price: 0, posts: "100/mo" },
      { label: "Starter", price: 10, posts: "1,000/mo" },
      { label: "Growth", price: 25, posts: "2,500/mo" },
      { label: "Pro", price: 50, posts: "5,000/mo" },
      { label: "Scale", price: 75, posts: "10,000/mo" },
      { label: "Business", price: 150, posts: "20,000/mo" },
      { label: "Enterprise", price: null, posts: "Custom" },
    ],
  },
  platforms: {
    total: 7,
    x: true,
    bluesky: true,
    linkedin: true,
    instagram: true,
    threads: true,
    tiktok: true,
    youtube: true,
    facebook: "coming",
    pinterest: "coming",
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
