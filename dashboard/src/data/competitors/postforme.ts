// PostForMe competitor data
// Last verified: 2026-05-01 against postforme.dev/pricing
// Verify quarterly before updating — also re-verify whenever the live
// pricing copy on UniPost references a specific PostForMe fact.

export const POSTFORME = {
  name: "PostForMe",
  slug: "postforme",
  tagline: "Lightweight, API-only social posting",
  pricing: {
    freeTier: false,
    freePostsPerMonth: 0,
    startingPrice: 10,
    pricingModel: "Single per-post-volume tier",
    enterprisePlan: true,
    tiers: [
      { label: "Standard", price: 10, posts: "1,000/mo" },
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
    webhooks: false, // not advertised on the public pricing page
    mediaUpload: true,
    twitterThreads: false,
    bulkPublishing: false,
    mcpServer: false,
    firstComment: false,
    nativeMode: false,
    quickstartMode: true,
    inbox: false, // PostForMe is API-only — no DM/comments inbox
    dashboard: false, // API-first product, no dashboard UI
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
  heroTitle: "UniPost vs PostForMe —\nWhen you outgrow API-only",
  heroSub: "PostForMe is a lightweight posting API. UniPost is the same API plus a dashboard, Inbox, Analytics, and team workflows — and the API tier matches PostForMe at $10/mo.",
  verdict: {
    chooseUs: [
      "You want a dashboard for manual posting",
      "You need an Inbox for DMs and comments",
      "You're building AI agents (UniPost ships native MCP)",
      "You want a permanent free tier without a credit card",
    ],
    chooseThem: [
      "You want to self-host and own the code",
      "You only need a publishing API, no dashboard",
      "You prefer open-source solutions",
    ],
  },
  migrationEndpoint: {
    from: "api.postforme.dev/v1/publish",
    to: "api.unipost.dev/v1/posts",
  },
  migrationFields: {
    from: 'networks: ["instagram"]',
    to: 'account_ids: ["sa_instagram_xxx"]',
  },
  faqs: [
    { q: "Can I use UniPost if I'm already using PostForMe?", a: "Yes. You can run both in parallel during migration. UniPost's permanent Free tier lets you test without any financial commitment." },
    { q: "PostForMe is open-source. Is UniPost?", a: "No. UniPost is a managed SaaS product. You get reliability, uptime, and zero infrastructure management. If you need open-source, PostForMe is a good choice." },
    { q: "Does UniPost support X/Twitter?", a: "Yes — on paid plans (API $10/mo and up). UniPost supports all 9 platforms PostForMe supports plus a dashboard, Inbox, and full Analytics. PostForMe also supports X." },
    { q: "Does UniPost have a free trial?", a: "Yes — the Free plan (100 posts/month) is permanent, not a time-limited trial. No credit card required. X publishing is reserved for paid plans." },
    { q: "How does UniPost API ($10) compare to PostForMe ($10)?", a: "Same price, same 1,000-post quota, same 9 platforms (UniPost is on 9 too). UniPost API also includes a native MCP server for AI-agent workflows, which PostForMe does not have. Step up to UniPost Basic ($19/mo) when you also want a dashboard, Inbox, and full Analytics — features PostForMe doesn't ship." },
    { q: "How long does migration take?", a: "Most developers complete the switch in under an hour. The main change is the endpoint URL and field names." },
  ],
  seo: {
    title: "UniPost vs PostForMe — Social API Comparison (2026) | UniPost",
    description: "Comparing PostForMe alternatives? UniPost API matches PostForMe at $10/mo and adds a dashboard, Inbox, Analytics, and native MCP server on Basic and up.",
    keywords: ["postforme alternative", "postforme competitor", "postforme vs unipost", "social media api comparison"],
    ogTitle: "UniPost vs PostForMe — Social Media API Comparison",
    ogDescription: "Same API price. Plus dashboard, Inbox, Analytics, MCP. Compare UniPost and PostForMe.",
  },
};
