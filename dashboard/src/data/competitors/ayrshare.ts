// Ayrshare competitor data
// Last verified: 2026-05-01
// Source: ayrshare.com/pricing + ayrshare.com/docs/* (supported networks, MCP, webhooks, messenger APIs)
// Verify quarterly before updating

export const AYRSHARE = {
  name: "Ayrshare",
  slug: "ayrshare",
  tagline: "Social media API for posting, scheduling, and analytics",
  pricing: {
    freeTier: true, // Basic plan: ~20 posts/mo with "sent with free plan" branding watermark
    freePostsPerMonth: 20,
    startingPrice: 149, // Premium tier — Free Basic exists but is watermarked
    pricingModel: "Per post volume + per-profile",
    enterprisePlan: true,
    tiers: [
      { label: "Basic (Free)", price: 0,    posts: "~20/mo (watermarked)" },
      { label: "Premium",      price: 149,  posts: "Included" },
      { label: "Business",     price: 499,  posts: "Higher volume" },
      { label: "Enterprise",   price: null, posts: "Custom" },
    ],
  },
  platforms: {
    total: 13, // docs list 13: Bluesky, Facebook, Google Business Profile, Instagram, LinkedIn, Pinterest, Reddit, Snapchat, Telegram, Threads, TikTok, X, YouTube
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
      "You want a $10/mo API-only plan (Ayrshare Premium starts at $149/mo)",
      "You want Inbox + Analytics bundled into a $19/mo plan",
      "You're a solo dev or small team",
    ],
    chooseThem: [
      "You need 13 platforms including Reddit, Snapchat, Telegram, and Google Business",
      "You have enterprise compliance needs and want a longer track record",
      "You need a white-label reseller plan (Ayrshare Business)",
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
    { q: "Can I use UniPost if I'm already using Ayrshare?", a: "Yes. You can run both in parallel during migration. UniPost's Free plan (100 posts/mo, no watermarks) lets you test without committing." },
    { q: "Does UniPost support all the platforms Ayrshare supports?", a: "UniPost currently supports 9 platforms (X, Instagram, Facebook, Threads, TikTok, YouTube, LinkedIn, Pinterest, Bluesky). Ayrshare supports 13, including Reddit, Snapchat, Telegram, and Google Business Profile. If you need a network beyond UniPost's nine, check our roadmap or contact us." },
    { q: "Is UniPost's API compatible with Ayrshare?", a: "The API is similar in concept but not drop-in compatible. Field names and endpoint paths differ. See the migration guide above for the exact changes needed." },
    { q: "Does UniPost have a free plan?", a: "Yes — the Free plan includes 100 posts/month, no credit card, no time limit, and no branding watermarks. Ayrshare's Basic plan is also free but is limited to ~20 posts/mo and adds a 'sent with free plan' watermark to every post." },
    { q: "Both have MCP — what's the difference?", a: "Ayrshare ships an official MCP server with 34 tools, and so does UniPost. Both work with Claude, Cursor, and other MCP clients. The deciding factor is usually pricing and the rest of the feature stack — UniPost API at $10/mo is cheaper than Ayrshare Premium at $149/mo." },
    { q: "What about Ayrshare's white-label features?", a: "UniPost supports White-label (Native mode) starting at the Growth tier ($59/mo). Ayrshare's white-label is on the Business plan ($499/mo). Custom-domain white-labeling is on UniPost's roadmap." },
    { q: "How long does migration take?", a: "Most developers complete the switch in under an hour. The main change is the endpoint URL and field names." },
  ],
  seo: {
    title: "Best Ayrshare Alternative for Developers (2026) | UniPost",
    description: "Looking for an Ayrshare alternative? UniPost starts at $10/mo (vs Ayrshare's $149) with Inbox + Analytics bundled at $19/mo. Compare features and pricing.",
    keywords: ["ayrshare alternative", "ayrshare competitor", "ayrshare vs unipost", "social media api alternative", "unified social media api"],
    ogTitle: "UniPost vs Ayrshare — Social Media API Comparison",
    ogDescription: "Cheaper entry tier. Bundled Inbox + Analytics. Compare UniPost and Ayrshare.",
  },
};
