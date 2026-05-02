// PostForMe competitor data
// Last verified: 2026-05-01 against postforme.dev (pricing, homepage, developers, about, faq)
// Verify quarterly before updating — also re-verify whenever the live
// pricing copy on UniPost references a specific PostForMe fact.

export const POSTFORME = {
  name: "PostForMe",
  slug: "postforme",
  tagline: "Open-source social posting API + dashboard",
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
    webhooks: true, // advertised on homepage and developers page (real-time account + post-status events)
    mediaUpload: true,
    twitterThreads: false, // not advertised on public pages
    bulkPublishing: false, // not advertised on public pages
    mcpServer: false,
    firstComment: false, // not advertised on public pages
    nativeMode: true, // "White Label Project" — own developer credentials, branded OAuth
    quickstartMode: true,
    inbox: false, // no DM/comments inbox surface
    dashboard: true, // hosted dashboard at app.postforme.dev (composer, scheduling, analytics)
  },
  developerExperience: {
    restApi: true,
    sdk: true, // JS, TypeScript, Python, Ruby, Go (5 official SDKs)
    docsQuality: 3,
    mcpServer: false,
    openSource: true, // AGPL-3.0, github.com/DayMoonDevelopment/post-for-me
  },
  compliance: {
    soc2: false,
    gdpr: false, // not explicitly advertised on FAQ/pricing/about/homepage
  },
  heroTitle: "UniPost vs PostForMe —\nManaged SaaS vs self-host",
  heroSub: "PostForMe is an open-source posting API with its own dashboard and white-label OAuth. UniPost is a managed SaaS alternative with a permanent free tier, native MCP server, and an Inbox for DMs and comments.",
  verdict: {
    chooseUs: [
      "You want a permanent free tier (100 posts/mo, no credit card)",
      "You need an Inbox for DMs and comments",
      "You're building AI agents (UniPost ships native MCP)",
      "You'd rather not run and maintain infrastructure yourself",
    ],
    chooseThem: [
      "You want to self-host and own the code (AGPL-3.0)",
      "You prefer open-source solutions",
      "You're comfortable running your own infrastructure",
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
    { q: "Can I use UniPost if I'm already using PostForMe?", a: "Yes. You can run both in parallel during migration. UniPost's permanent Free tier (100 posts/mo) lets you test without any financial commitment — PostForMe has no free tier." },
    { q: "PostForMe is open-source. Is UniPost?", a: "No. UniPost is a managed SaaS product. You get reliability, uptime, and zero infrastructure management. If you need open-source and want to self-host, PostForMe is a good choice (AGPL-3.0)." },
    { q: "Does UniPost support X/Twitter?", a: "Yes — on paid plans (API $10/mo and up). UniPost supports the same 9 platforms PostForMe supports." },
    { q: "Does UniPost have a free trial?", a: "Yes — the Free plan (100 posts/month) is permanent, not a time-limited trial. No credit card required. X publishing is reserved for paid plans. PostForMe has no free tier — its lowest plan is $10/mo." },
    { q: "How does UniPost API ($10) compare to PostForMe ($10)?", a: "Same price, same 1,000-post quota, same 9 platforms. UniPost API also includes a native MCP server for AI-agent workflows, which PostForMe does not have. Step up to UniPost Basic ($19/mo) if you also want an Inbox for DMs and comments — PostForMe doesn't ship an inbox." },
    { q: "How long does migration take?", a: "Most developers complete the switch in under an hour. The main change is the endpoint URL and field names." },
  ],
  seo: {
    title: "UniPost vs PostForMe — Managed vs Self-host (2026) | UniPost",
    description: "Comparing PostForMe alternatives? UniPost is a managed SaaS with a permanent free tier, native MCP server, and an Inbox — versus PostForMe's open-source self-host model.",
    keywords: ["postforme alternative", "postforme competitor", "postforme vs unipost", "social media api comparison"],
    ogTitle: "UniPost vs PostForMe — Managed SaaS vs Open Source",
    ogDescription: "Free tier. Native MCP. Inbox for DMs. Compare UniPost and PostForMe.",
  },
};
