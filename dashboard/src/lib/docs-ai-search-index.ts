import { PLATFORM_METRICS } from "@/lib/platform-capabilities";

export type DocsAiConfidence = "high" | "medium" | "low" | "none";

export type DocsAiChunk = {
  id: string;
  title: string;
  path: string;
  section_id: string;
  primary_nav: "Guides" | "API Reference" | "Platforms" | "Resources" | "Overview";
  section_title: string;
  content: string;
  product_area: "analytics" | "accounts" | "platforms" | "publishing";
  tags: string[];
  endpoint_aliases: string[];
  platforms: string[];
  last_indexed_at: string;
};

export type DocsAiSearchHit = {
  chunk: DocsAiChunk;
  score: number;
  matchedTerms: string[];
};

export type DocsAiSearchResult = {
  hits: DocsAiSearchHit[];
  confidence: DocsAiConfidence;
};

export type DocsAiSource = {
  id: string;
  title: string;
  path: string;
  section_title: string;
  primary_nav: string;
  excerpt: string;
};

export type GroundedDocsAnswer = {
  answer: string;
  steps: string[];
  confidence: DocsAiConfidence;
  sources: DocsAiSource[];
  related: DocsAiSource[];
  generated_by: "ai" | "extractive" | "fallback";
};

const LAST_INDEXED_AT = "2026-06-18T00:00:00.000Z";

function chunk(input: Omit<DocsAiChunk, "last_indexed_at">): DocsAiChunk {
  return { ...input, last_indexed_at: LAST_INDEXED_AT };
}

const platformCapabilitySummary = Object.entries(PLATFORM_METRICS)
  .map(([platform, metrics]) => {
    const supported = Object.entries(metrics)
      .filter(([, supportedMetric]) => supportedMetric)
      .map(([metric]) => metric)
      .join(", ");
    const unsupported = Object.entries(metrics)
      .filter(([, supportedMetric]) => !supportedMetric)
      .map(([metric]) => metric)
      .join(", ");

    return `${platform}: supported ${supported || "none"}; unsupported ${unsupported || "none"}.`;
  })
  .join("\n");

export const DOCS_AI_INDEX: DocsAiChunk[] = [
  chunk({
    id: "analytics-guide-tiktok-followers",
    title: "Get TikTok followers",
    path: "/docs/guides/analytics/tiktok-followers",
    section_id: "answer",
    primary_nav: "Guides",
    section_title: "Direct answer",
    product_area: "analytics",
    tags: ["analytics", "tiktok", "followers", "account metrics", "scopes"],
    endpoint_aliases: [
      "GET /v1/accounts/{account_id}/metrics",
      "GET /v1/accounts/{id}/metrics",
      "GET /v1/accounts/:account_id/metrics",
      "GET /v1/accounts/:id/metrics",
      "/v1/accounts/{account_id}/metrics",
      "/v1/accounts/:account_id/metrics",
    ],
    platforms: ["tiktok"],
    content:
      "TikTok followers use the unified UniPost account metrics API: GET /v1/accounts/{account_id}/metrics. The approved TikTok scope is user.info.stats. Read data.follower_count from the response. video.list is for public videos and post-level TikTok video inventory, not follower count. user.info.profile powers profile fields.",
  }),
  chunk({
    id: "analytics-guide-account-metrics",
    title: "Get account metrics across platforms",
    path: "/docs/guides/analytics/account-metrics",
    section_id: "fields",
    primary_nav: "Guides",
    section_title: "Fields to read",
    product_area: "analytics",
    tags: ["analytics", "accounts", "followers", "following", "post count", "metrics"],
    endpoint_aliases: [
      "GET /v1/accounts/{account_id}/metrics",
      "GET /v1/accounts/{id}/metrics",
      "GET /v1/accounts/:account_id/metrics",
      "GET /v1/accounts/:id/metrics",
    ],
    platforms: ["instagram", "threads", "tiktok", "twitter"],
    content:
      "Use GET /v1/accounts/{account_id}/metrics for account-level metrics such as data.follower_count, data.following_count, data.post_count, and data.platform_specific. List accounts with GET /v1/accounts, choose the connected account id, then call account metrics. Unsupported platforms return NOT_SUPPORTED instead of an empty success response.",
  }),
  chunk({
    id: "api-reference-account-metrics",
    title: "Get account metrics",
    path: "/docs/api/accounts/metrics",
    section_id: "endpoint",
    primary_nav: "API Reference",
    section_title: "GET account metrics",
    product_area: "accounts",
    tags: ["api reference", "account metrics", "followers", "account_id"],
    endpoint_aliases: [
      "GET /v1/accounts/{account_id}/metrics",
      "GET /v1/accounts/{id}/metrics",
      "GET /v1/accounts/:account_id/metrics",
      "GET /v1/accounts/:id/metrics",
    ],
    platforms: ["instagram", "threads", "tiktok", "twitter"],
    content:
      "GET /v1/accounts/{account_id}/metrics returns normalized account metrics for one connected social account. The response includes data.social_account_id, data.platform, data.follower_count, data.following_count, data.post_count, data.platform_specific, and data.fetched_at.",
  }),
  chunk({
    id: "analytics-guide-export-posts",
    title: "Export post analytics rows",
    path: "/docs/guides/analytics/export-post-analytics",
    section_id: "steps",
    primary_nav: "Guides",
    section_title: "Steps",
    product_area: "analytics",
    tags: ["analytics", "export", "csv", "posts", "reporting", "bi"],
    endpoint_aliases: [
      "GET /v1/analytics/posts/export",
      "/v1/analytics/posts/export",
    ],
    platforms: ["instagram", "threads", "pinterest", "tiktok", "facebook", "linkedin", "twitter", "youtube", "bluesky"],
    content:
      "Use GET /v1/analytics/posts/export when your app needs normalized post-level analytics rows as CSV across multiple UniPost-published posts. Choose from and to, add optional platform, account_id, profile_id, post_id, status, and sort filters, then save the CSV response.",
  }),
  chunk({
    id: "analytics-guide-post-analytics",
    title: "Get post analytics",
    path: "/docs/guides/analytics/post-analytics",
    section_id: "overview",
    primary_nav: "Guides",
    section_title: "Overview",
    product_area: "analytics",
    tags: ["analytics", "posts", "post analytics", "likes", "comments", "shares", "views"],
    endpoint_aliases: [
      "GET /v1/posts/{post_id}/analytics",
      "GET /v1/posts/:post_id/analytics",
      "/v1/posts/{post_id}/analytics",
    ],
    platforms: ["instagram", "threads", "pinterest", "tiktok", "facebook", "linkedin", "twitter", "youtube", "bluesky"],
    content:
      "Use GET /v1/posts/{post_id}/analytics for normalized post-level analytics on a single UniPost-published post. The response includes destination results, normalized analytics fields such as likes, comments, shares, saves, clicks, video views, engagement rate where available, platform_specific data, and fetched_at.",
  }),
  chunk({
    id: "analytics-platform-capabilities",
    title: "Platform analytics capabilities",
    path: "/docs/api/analytics/platforms",
    section_id: "capabilities",
    primary_nav: "API Reference",
    section_title: "Supported metrics by platform",
    product_area: "platforms",
    tags: ["analytics", "platforms", "capabilities", "metrics", "support matrix"],
    endpoint_aliases: [
      "GET /v1/analytics/platforms",
      "/v1/analytics/platforms",
    ],
    platforms: Object.keys(PLATFORM_METRICS),
    content:
      "Platform analytics capabilities come from PLATFORM_METRICS, the shared product source of truth for supported post metrics. Current snapshot:\n" +
      platformCapabilitySummary,
  }),
  chunk({
    id: "analytics-guide-reconnect-scopes",
    title: "Reconnect accounts for analytics scopes",
    path: "/docs/guides/analytics/reconnect-analytics-scopes",
    section_id: "steps",
    primary_nav: "Guides",
    section_title: "Steps",
    product_area: "analytics",
    tags: ["analytics", "scopes", "reconnect", "oauth", "permissions"],
    endpoint_aliases: [
      "GET /v1/accounts/{account_id}/health",
      "GET /v1/accounts/:account_id/health",
      "GET /v1/accounts",
    ],
    platforms: ["instagram", "threads", "pinterest", "tiktok", "facebook"],
    content:
      "If an account was connected before analytics scopes were granted, reconnect it so the token includes the new platform scopes. TikTok analytics scopes include user.info.profile, user.info.stats, and video.list. Use account health or account listing state to identify accounts that need reconnect before relying on live analytics metrics.",
  }),
  chunk({
    id: "analytics-tiktok-native-drilldown",
    title: "TikTok native analytics drilldowns",
    path: "/docs/api/analytics/tiktok",
    section_id: "overview",
    primary_nav: "API Reference",
    section_title: "Overview",
    product_area: "analytics",
    tags: ["analytics", "tiktok", "profile", "videos", "native drilldown"],
    endpoint_aliases: [
      "GET /v1/accounts/{account_id}/tiktok/profile",
      "GET /v1/accounts/{account_id}/tiktok/videos",
      "GET /v1/accounts/{account_id}/metrics",
    ],
    platforms: ["tiktok"],
    content:
      "TikTok native drilldowns are optional when a product needs profile details or public video inventory. user.info.profile powers TikTok profile fields, user.info.stats powers account stats, and video.list powers public videos. For follower count, prefer the unified GET /v1/accounts/{account_id}/metrics endpoint.",
  }),
];

const STOP_WORDS = new Set([
  "a",
  "an",
  "and",
  "api",
  "are",
  "can",
  "do",
  "does",
  "for",
  "from",
  "get",
  "how",
  "i",
  "is",
  "it",
  "of",
  "on",
  "or",
  "the",
  "to",
  "use",
  "what",
  "which",
  "with",
]);

function normalize(value: string) {
  return value.toLowerCase().replace(/[\u2018\u2019]/g, "'").replace(/[\u201c\u201d]/g, "\"");
}

function tokenize(query: string) {
  return Array.from(new Set(
    normalize(query)
      .split(/[^a-z0-9_./:{}-]+/)
      .map((token) => token.trim())
      .filter((token) => token.length > 1 && !STOP_WORDS.has(token)),
  ));
}

function scoreChunk(chunkToScore: DocsAiChunk, query: string, terms: string[]) {
  const normalizedQuery = normalize(query);
  const title = normalize(chunkToScore.title);
  const content = normalize(chunkToScore.content);
  const tags = chunkToScore.tags.map(normalize).join(" ");
  const aliases = chunkToScore.endpoint_aliases.map(normalize).join(" ");
  const path = normalize(chunkToScore.path);
  const platforms = chunkToScore.platforms.map(normalize).join(" ");
  const haystack = `${title} ${content} ${tags} ${aliases} ${path} ${platforms}`;
  let score = 0;
  const matchedTerms: string[] = [];

  for (const term of terms) {
    if (!haystack.includes(term)) continue;

    matchedTerms.push(term);
    score += 5;
    if (title.includes(term)) score += 9;
    if (tags.includes(term)) score += 7;
    if (aliases.includes(term)) score += 11;
    if (platforms.includes(term)) score += 5;
    if (path.includes(term)) score += 4;
  }

  if (normalizedQuery && aliases.includes(normalizedQuery)) score += 44;
  if (normalizedQuery && title.includes(normalizedQuery)) score += 36;
  if (normalizedQuery.includes("follower") && chunkToScore.id.includes("followers")) score += 26;
  if (normalizedQuery.includes("tiktok") && chunkToScore.platforms.includes("tiktok")) score += 18;
  if (normalizedQuery.includes("video.list") && content.includes("video.list")) score += 18;
  if (normalizedQuery.includes("export") && path.includes("export")) score += 24;
  if (normalizedQuery.includes("csv") && content.includes("csv")) score += 15;

  const isTaskQuery = /\b(how|which|what|where|followers?|export|reconnect|field|fields)\b/i.test(query);
  if (isTaskQuery && chunkToScore.primary_nav === "Guides") score += 10;
  if (/\bendpoint|reference|path|route\b/i.test(query) && chunkToScore.primary_nav === "API Reference") score += 10;

  return { score, matchedTerms };
}

function confidenceForScore(score: number): DocsAiConfidence {
  if (score >= 62) return "high";
  if (score >= 34) return "medium";
  if (score >= 16) return "low";
  return "none";
}

export function searchDocsIndex(query: string, options: { limit?: number } = {}): DocsAiSearchResult {
  const trimmed = query.trim();
  if (!trimmed) {
    return { hits: [], confidence: "none" };
  }

  const terms = tokenize(trimmed);
  const hits = DOCS_AI_INDEX
    .map((chunkItem) => {
      const scored = scoreChunk(chunkItem, trimmed, terms);
      return { chunk: chunkItem, score: scored.score, matchedTerms: scored.matchedTerms };
    })
    .filter((hit) => hit.score > 0)
    .sort((a, b) => b.score - a.score || a.chunk.title.localeCompare(b.chunk.title))
    .slice(0, options.limit ?? 5);

  return {
    hits,
    confidence: confidenceForScore(hits[0]?.score ?? 0),
  };
}

function sourceFromHit(hit: DocsAiSearchHit): DocsAiSource {
  const excerpt = hit.chunk.content.length > 240
    ? `${hit.chunk.content.slice(0, 237).trimEnd()}...`
    : hit.chunk.content;

  return {
    id: hit.chunk.id,
    title: hit.chunk.title,
    path: hit.chunk.path,
    section_title: hit.chunk.section_title,
    primary_nav: hit.chunk.primary_nav,
    excerpt,
  };
}

function isInsufficientCoverage(search: DocsAiSearchResult) {
  // no answer without source coverage: low-confidence matches can be shown as related docs,
  // but the answer must explicitly say the docs do not contain enough support.
  return search.confidence === "none" || search.hits.length === 0;
}

function answerForKnownAnalyticsTask(query: string, search: DocsAiSearchResult) {
  const normalizedQuery = normalize(query);

  if (normalizedQuery.includes("tiktok") && /follower|fans?/.test(normalizedQuery)) {
    return {
      answer:
        "Use the unified UniPost account metrics endpoint, not a TikTok-native followers endpoint. Call GET /v1/accounts/{account_id}/metrics for the TikTok account and read data.follower_count. The TikTok scope behind that field is user.info.stats; video.list is for public videos and post-level inventory.",
      steps: [
        "List accounts with GET /v1/accounts and choose the TikTok account id.",
        "If the account predates analytics scope approval, reconnect it so user.info.stats is on the token.",
        "Call GET /v1/accounts/{account_id}/metrics.",
        "Read data.follower_count from the response.",
      ],
    };
  }

  if (normalizedQuery.includes("export") || normalizedQuery.includes("csv")) {
    return {
      answer:
        "Use GET /v1/analytics/posts/export when you need normalized post analytics rows as CSV across multiple UniPost-published posts. Add date filters, then optionally filter by platform, account_id, profile_id, post_id, status, or sort.",
      steps: [
        "Choose the reporting window with from and to.",
        "Add optional filters such as platform, account_id, profile_id, or status.",
        "Call GET /v1/analytics/posts/export and save the CSV response.",
      ],
    };
  }

  if (normalizedQuery.includes("account") && normalizedQuery.includes("field")) {
    return {
      answer:
        "The account metrics response is normalized around data.social_account_id, data.platform, data.follower_count, data.following_count, data.post_count, data.platform_specific, and data.fetched_at.",
      steps: [
        "Call GET /v1/accounts/{account_id}/metrics for the connected account.",
        "Read normalized fields first.",
        "Use data.platform_specific only for provider-native additions or upstream diagnostics.",
      ],
    };
  }

  const top = search.hits[0]?.chunk;
  if (!top) return null;

  return {
    answer: `The closest documented path is ${top.title}. ${top.content}`,
    steps: [
      `Open ${top.path}.`,
      "Use the cited source before wiring the API call into production.",
    ],
  };
}

export function buildGroundedDocsAnswer(query: string, search = searchDocsIndex(query)): GroundedDocsAnswer {
  const sources = search.hits.slice(0, 3).map(sourceFromHit);
  const related = search.hits.slice(3, 5).map(sourceFromHit);

  if (isInsufficientCoverage(search)) {
    return {
      answer:
        "I could not find enough source coverage in the docs to answer that confidently. Try the related docs below or file a missing-docs report from this search panel.",
      steps: [],
      confidence: "none",
      sources: [],
      related: sources,
      generated_by: "fallback",
    };
  }

  const taskAnswer = answerForKnownAnalyticsTask(query, search);

  return {
    answer: taskAnswer?.answer ?? "The docs contain relevant source coverage, but no task answer template matched this query. Use the cited sources below.",
    steps: taskAnswer?.steps ?? [],
    confidence: search.confidence,
    sources,
    related,
    generated_by: "extractive",
  };
}
