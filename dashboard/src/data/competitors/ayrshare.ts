// Ayrshare competitor data
// Last verified: 2026-06-23
// Source: ayrshare.com homepage + pricing + docs/additional/mcp-server
// Verify quarterly before updating

export const AYRSHARE = {
  name: "Ayrshare",
  slug: "ayrshare",
  tagline: "Social media API for posting, scheduling, and analytics",
  pricing: {
    freeTier: false,
    freePostsPerMonth: 0,
    startingPrice: 599, // Business monthly; annual public rate starts at $499/mo
    pricingModel: "Business/Enterprise profile-based pricing",
    enterprisePlan: true,
    tiers: [
      { label: "Business monthly", price: 599, posts: "First 30 profiles" },
      { label: "Business annual", price: 499, posts: "First 30 profiles" },
      { label: "Additional profiles", price: 8.99, posts: "Up to 100 profiles" },
      { label: "Enterprise", price: null, posts: "Custom" },
    ],
  },
  platforms: {
    total: "13+", // homepage advertises 13+ platforms
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
    mcpServer: true, // official Ayrshare MCP server with 34 tools (Claude / Cursor compatible)
    firstComment: false, // not advertised
    nativeMode: true, // White-label on Business plan
    quickstartMode: false, // term not used by Ayrshare
    inbox: true, // Messenger + Comments APIs
    dashboard: true, // app.ayrshare.com
  },
  developerExperience: {
    restApi: true,
    sdk: true, // Official Node.js + Python SDKs
    docsQuality: 5,
    mcpServer: true,
    openSource: false, // SDK wrappers on GitHub but core platform proprietary
  },
  compliance: {
    soc2: false, // not advertised on public pages
    gdpr: true, // explicitly GDPR-ready per DPA page
  },
  heroTitle: "The best Ayrshare alternative\nfor developers in 2026",
  heroSub: "Looking for an Ayrshare alternative? UniPost starts at $10/mo (Ayrshare Premium is $149) and bundles Inbox + Analytics into Basic at $19/mo.",
  verdict: {
    chooseUs: [
      "You want a real free tier without watermarks (UniPost Free: 100 posts/mo, no branding)",
      "You want a $10/mo API-only plan instead of a Business profile-based plan",
      "You want Inbox + Analytics bundled into a $19/mo plan",
      "You're a solo dev or small team",
    ],
    chooseThem: [
      "You need 13 platforms including Reddit, Snapchat, Telegram, and Google Business",
      "You have enterprise compliance needs and want a longer track record",
      "You need a white-label reseller plan (Ayrshare Business)",
    ],
  },
  bestFit: {
    unipost: "Developers and small SaaS teams that want a lower self-serve entry point, free tier, and native MCP inside UniPost.",
    competitor: "Mature SaaS teams that prefer Ayrshare's profile-based Business plan, 13+ platforms, and enterprise support motion.",
  },
  sourceLinks: [
    { label: "Ayrshare homepage", url: "https://www.ayrshare.com/" },
    { label: "Ayrshare pricing", url: "https://www.ayrshare.com/pricing/" },
    { label: "Ayrshare Documentation MCP", url: "https://www.ayrshare.com/docs/additional/mcp-server" },
  ],
  migrationEndpoint: {
    from: "api.ayrshare.com/api/post",
    to: "api.unipost.dev/v1/posts",
  },
  migrationFields: {
    from: 'platforms: ["instagram"]',
    to: 'account_ids: ["sa_instagram_xxx"]',
  },
  faqs: [
    { q: "Can I use UniPost if I'm already using Ayrshare?", a: "Yes. You can run both in parallel during migration. UniPost's Free plan (100 posts/mo, no watermarks) lets you test without committing." },
    { q: "Does UniPost support all the platforms Ayrshare supports?", a: "UniPost currently supports 9 platforms (X, Instagram, Facebook, Threads, TikTok, YouTube, LinkedIn, Pinterest, Bluesky). Ayrshare supports 13, including Reddit, Snapchat, Telegram, and Google Business Profile. If you need a network beyond UniPost's nine, check our roadmap or contact us." },
    { q: "Is UniPost's API compatible with Ayrshare?", a: "The API is similar in concept but not drop-in compatible. Field names and endpoint paths differ. See the migration guide above for the exact changes needed." },
    { q: "Does UniPost have a free plan?", a: "Yes — the Free plan includes 100 posts/month, no credit card, no time limit, and no branding watermarks. Ayrshare's current public pricing emphasizes Business and Enterprise profile-based pricing with a 14-day free trial." },
    { q: "Both have MCP — what's the difference?", a: "Ayrshare publishes MCP documentation and UniPost ships native MCP support. The deciding factor is usually packaging: UniPost has a lower self-serve entry point, while Ayrshare is oriented around Business and Enterprise profile pricing." },
    { q: "What about Ayrshare's white-label features?", a: "UniPost supports White-label (Native mode) starting at the Growth tier ($59/mo). Ayrshare's Business plan is profile-based, with public pricing currently listed for the first 30 profiles." },
    { q: "How long does migration take?", a: "Most developers complete the switch in under an hour. The main change is the endpoint URL and field names." },
  ],
  seo: {
    title: "Best Ayrshare Alternative for Developers (2026) | UniPost",
    description: "Looking for an Ayrshare alternative? UniPost starts at $10/mo with a free tier and native MCP, while Ayrshare is oriented around Business and Enterprise profile pricing.",
    keywords: ["ayrshare alternative", "ayrshare competitor", "ayrshare vs unipost", "social media api alternative", "unified social media api"],
    ogTitle: "UniPost vs Ayrshare — Social Media API Comparison",
    ogDescription: "Cheaper entry tier. Bundled Inbox + Analytics. Compare UniPost and Ayrshare.",
  },
};
