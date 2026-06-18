"use client";

import Link from "next/link";
import { ApiReferencePage, MethodBadge } from "./_components/doc-components";

type Endpoint = {
  label: string;
  method: "GET" | "POST" | "PATCH" | "DELETE";
  path: string;
  href: string;
  description: string;
};

type EndpointGroup = {
  title: string;
  description: string;
  endpoints: Endpoint[];
};

const ENDPOINT_GROUPS: EndpointGroup[] = [
  {
    title: "Profiles",
    description: "Create and manage UniPost profiles that organize connected accounts and published content.",
    endpoints: [
      { label: "List profiles", method: "GET", path: "/v1/profiles", href: "/docs/api/profiles/list", description: "Fetch profiles in the workspace." },
      { label: "Create profile", method: "POST", path: "/v1/profiles", href: "/docs/api/profiles/create", description: "Create a profile for posting and analytics." },
      { label: "Get profile", method: "GET", path: "/v1/profiles/{id}", href: "/docs/api/profiles/get", description: "Fetch one profile by ID." },
      { label: "Update profile", method: "PATCH", path: "/v1/profiles/{id}", href: "/docs/api/profiles/update", description: "Update profile metadata." },
      { label: "Delete profile", method: "DELETE", path: "/v1/profiles/{id}", href: "/docs/api/profiles/delete", description: "Remove a profile." },
    ],
  },
  {
    title: "Accounts",
    description: "Connect social accounts, inspect account health, and query account-level platform capabilities.",
    endpoints: [
      { label: "List accounts", method: "GET", path: "/v1/accounts", href: "/docs/api/accounts/list", description: "List connected social accounts." },
      { label: "Connect account", method: "POST", path: "/v1/accounts/connect", href: "/docs/api/accounts/connect", description: "Create an OAuth connection request." },
      { label: "OAuth connect", method: "POST", path: "/v1/oauth/connect", href: "/docs/api/accounts/oauth-connect", description: "Start an OAuth flow from API clients." },
      { label: "Disconnect account", method: "DELETE", path: "/v1/accounts/{id}", href: "/docs/api/accounts/disconnect", description: "Disconnect a social account." },
      { label: "Account capabilities", method: "GET", path: "/v1/accounts/{id}/capabilities", href: "/docs/api/accounts/capabilities", description: "Inspect publish and media support." },
      { label: "Account health", method: "GET", path: "/v1/accounts/{id}/health", href: "/docs/api/accounts/health", description: "Read connection health and reconnect state." },
      { label: "Account metrics", method: "GET", path: "/v1/accounts/{id}/metrics", href: "/docs/api/accounts/metrics", description: "Read platform account metrics." },
      { label: "TikTok creator info", method: "GET", path: "/v1/accounts/{id}/tiktok/creator-info", href: "/docs/api/accounts/tiktok-creator-info", description: "Fetch TikTok publishing limits." },
    ],
  },
  {
    title: "Publishing",
    description: "Create, schedule, validate, update, and inspect posts across connected destinations.",
    endpoints: [
      { label: "Create post", method: "POST", path: "/v1/posts", href: "/docs/api/posts/create", description: "Create or schedule a post." },
      { label: "List posts", method: "GET", path: "/v1/posts", href: "/docs/api/posts/list", description: "List posts in a workspace." },
      { label: "Get post", method: "GET", path: "/v1/posts/{id}", href: "/docs/api/posts/get", description: "Fetch a post and delivery results." },
      { label: "Update post", method: "PATCH", path: "/v1/posts/{id}", href: "/docs/api/posts/update", description: "Edit post content or status." },
      { label: "Validate post", method: "POST", path: "/v1/posts/validate", href: "/docs/api/posts/validate", description: "Validate platform-specific constraints." },
      { label: "Create draft", method: "POST", path: "/v1/posts/drafts", href: "/docs/api/posts/drafts/create", description: "Create a draft post." },
      { label: "Publish draft", method: "POST", path: "/v1/posts/{id}/publish", href: "/docs/api/posts/drafts/publish", description: "Publish a draft." },
      { label: "Reserve media upload", method: "POST", path: "/v1/media", href: "/docs/api/media/reserve", description: "Reserve upload storage for media." },
      { label: "Get media", method: "GET", path: "/v1/media/{id}", href: "/docs/api/media/get", description: "Fetch uploaded media metadata." },
    ],
  },
  {
    title: "Analytics",
    description: "Build reporting tables, platform summaries, exports, and refresh workflows across Instagram, Threads, Pinterest, TikTok, and Facebook Page.",
    endpoints: [
      { label: "Workspace summary", method: "GET", path: "/v1/analytics/summary", href: "/docs/api/analytics/summary", description: "Read workspace-level analytics cards." },
      { label: "Post analytics", method: "GET", path: "/v1/posts/{post_id}/analytics", href: "/docs/api/analytics/posts", description: "Inspect one post's platform results." },
      { label: "List analytics posts", method: "GET", path: "/v1/analytics/posts", href: "/docs/api/analytics/posts-list", description: "List normalized post-level analytics rows." },
      { label: "Export analytics posts", method: "GET", path: "/v1/analytics/posts/export", href: "/docs/api/analytics/posts/export", description: "Download normalized post analytics as CSV." },
      { label: "Analytics rollup", method: "GET", path: "/v1/analytics/rollup", href: "/docs/api/analytics/rollup", description: "Read time-bucketed metrics grouped by platform, account, or status." },
      { label: "Analytics platforms", method: "GET", path: "/v1/analytics/platforms", href: "/docs/api/analytics/platforms", description: "List platform availability and analytics health." },
      { label: "Get analytics platform", method: "GET", path: "/v1/analytics/platforms/{platform}", href: "/docs/api/analytics/platforms/detail", description: "Read platform summary, trend, accounts, and top posts." },
      { label: "Request analytics refresh", method: "POST", path: "/v1/analytics/refresh", href: "/docs/api/analytics/refresh", description: "Mark matching rows stale for the refresh worker." },
    ],
  },
  {
    title: "Instagram Analytics",
    description: "Read Instagram Business profile details, account metrics, recent media insights, and UniPost-published Instagram post performance.",
    endpoints: [
      { label: "Instagram Analytics overview", method: "GET", path: "/v1/accounts/{id}/instagram/*", href: "/docs/api/analytics/instagram", description: "Understand Instagram analytics scopes and endpoint choices." },
      { label: "Get Instagram profile", method: "GET", path: "/v1/accounts/{id}/instagram/profile", href: "/docs/api/analytics/instagram/profile", description: "Fetch Instagram profile fields from instagram_business_basic." },
      { label: "Get Instagram account metrics", method: "GET", path: "/v1/accounts/{id}/metrics", href: "/docs/api/analytics/instagram/account-metrics", description: "Fetch follower, following, and media counts for Instagram accounts." },
      { label: "List Instagram media analytics", method: "GET", path: "/v1/accounts/{id}/instagram/media", href: "/docs/api/analytics/instagram/media", description: "Fetch recent media rows with reach, likes, comments, shares, and saves." },
      { label: "Instagram post analytics", method: "GET", path: "/v1/posts/{post_id}/analytics", href: "/docs/api/analytics/posts", description: "Read normalized metrics for Instagram posts published through UniPost." },
    ],
  },
  {
    title: "Threads Analytics",
    description: "Read Threads profile details, account metrics, recent post insights, and UniPost-published Threads post performance.",
    endpoints: [
      { label: "Threads Analytics overview", method: "GET", path: "/v1/accounts/{id}/threads/*", href: "/docs/api/analytics/threads", description: "Understand Threads analytics scopes and endpoint choices." },
      { label: "Get Threads profile", method: "GET", path: "/v1/accounts/{id}/threads/profile", href: "/docs/api/analytics/threads/profile", description: "Fetch Threads profile fields from threads_basic." },
      { label: "Get Threads account metrics", method: "GET", path: "/v1/accounts/{id}/metrics", href: "/docs/api/analytics/threads/account-metrics", description: "Fetch account insights from threads_manage_insights." },
      { label: "List Threads post analytics", method: "GET", path: "/v1/accounts/{id}/threads/posts", href: "/docs/api/analytics/threads/posts", description: "Fetch recent Threads posts with views, likes, replies, reposts, and quotes." },
      { label: "Threads post analytics", method: "GET", path: "/v1/posts/{post_id}/analytics", href: "/docs/api/analytics/posts", description: "Read normalized metrics for Threads posts published through UniPost." },
    ],
  },
  {
    title: "Pinterest Analytics",
    description: "Read Pinterest board inventory and UniPost-published Pin analytics.",
    endpoints: [
      { label: "Pinterest Analytics overview", method: "GET", path: "/v1/accounts/{id}/pinterest/*", href: "/docs/api/analytics/pinterest", description: "Understand Pinterest analytics scopes and endpoint choices." },
      { label: "List Pinterest boards", method: "GET", path: "/v1/accounts/{id}/pinterest/boards", href: "/docs/api/analytics/pinterest/boards", description: "Fetch Pinterest boards available to the connected account." },
      { label: "Pinterest post analytics", method: "GET", path: "/v1/posts/{post_id}/analytics", href: "/docs/api/analytics/pinterest/post-analytics", description: "Read normalized Pin metrics for Pinterest posts published through UniPost." },
    ],
  },
  {
    title: "TikTok Analytics",
    description: "Read TikTok profile fields, account statistics, public video inventory, and UniPost-published TikTok post performance.",
    endpoints: [
      { label: "TikTok Analytics overview", method: "GET", path: "/v1/accounts/{id}/tiktok/*", href: "/docs/api/analytics/tiktok", description: "Understand TikTok analytics scopes, production gating, and endpoint choices." },
      { label: "Get TikTok profile", method: "GET", path: "/v1/accounts/{id}/tiktok/profile", href: "/docs/api/analytics/tiktok/profile", description: "Fetch profile fields from user.info.profile." },
      { label: "Get TikTok account metrics", method: "GET", path: "/v1/accounts/{id}/metrics", href: "/docs/api/analytics/tiktok/account-metrics", description: "Fetch followers, following, public video count, and total likes from user.info.stats." },
      { label: "List TikTok public videos", method: "GET", path: "/v1/accounts/{id}/tiktok/videos", href: "/docs/api/analytics/tiktok/videos", description: "Fetch public video inventory and engagement counters from video.list." },
      { label: "TikTok post analytics", method: "GET", path: "/v1/posts/{post_id}/analytics", href: "/docs/api/analytics/posts", description: "Read normalized metrics for TikTok posts published through UniPost." },
    ],
  },
  {
    title: "Facebook Page Analytics",
    description: "Read Facebook Page profile data, Page Insights, recent Page posts, and UniPost-published Facebook post performance.",
    endpoints: [
      { label: "Facebook Page Analytics overview", method: "GET", path: "/v1/accounts/{id}/facebook/*", href: "/docs/api/analytics/facebook", description: "Understand Facebook Page analytics scopes and endpoint choices." },
      { label: "Get Facebook Page analytics", method: "GET", path: "/v1/accounts/{id}/facebook/page-analytics", href: "/docs/api/analytics/facebook/page-analytics", description: "Fetch Page profile, Page Insights, recent Page posts, and per-post engagement." },
      { label: "Get Facebook Page insights", method: "GET", path: "/v1/accounts/{id}/facebook/page-insights", href: "/docs/api/analytics/facebook/page-insights", description: "Fetch Page-level follows, impressions, views, and post engagements." },
      { label: "Facebook Page post analytics", method: "GET", path: "/v1/posts/{post_id}/analytics", href: "/docs/api/analytics/facebook/post-analytics", description: "Read normalized metrics for Facebook posts published through UniPost." },
    ],
  },
  {
    title: "API Metrics",
    description: "Inspect API-key traffic volume, latency, and status-code health for workspace Developer API calls.",
    endpoints: [
      { label: "Overall", method: "GET", path: "/v1/api-metrics/overall", href: "/docs/api/api-metrics/overall", description: "Read aggregate API latency, volume, and error totals." },
      { label: "Summary", method: "GET", path: "/v1/api-metrics/summary", href: "/docs/api/api-metrics/summary", description: "List per-endpoint API latency and error rows." },
      { label: "Trend", method: "GET", path: "/v1/api-metrics/trend", href: "/docs/api/api-metrics/trend", description: "Read hourly or daily API metrics buckets." },
      { label: "Status-Code", method: "GET", path: "/v1/api-metrics/status-codes", href: "/docs/api/api-metrics/status-codes", description: "Read exact status-code distribution by endpoint." },
    ],
  },
  {
    title: "Logs",
    description: "Query, backfill, and stream workspace integration logs for API, publishing, OAuth, webhook, and worker activity.",
    endpoints: [
      { label: "List logs", method: "GET", path: "/v1/logs", href: "/docs/api/logs/list", description: "Cursor-paginated log search and backfill." },
      { label: "Get log", method: "GET", path: "/v1/logs/{id}", href: "/docs/api/logs/get", description: "Fetch one log with redacted payloads." },
      { label: "Stream logs", method: "GET", path: "/v1/logs/stream", href: "/docs/api/logs/stream", description: "Real-time SSE log stream with replay." },
    ],
  },
  {
    title: "Connect Sessions",
    description: "Create branded account-connection sessions for embedded and white-label onboarding flows.",
    endpoints: [
      { label: "Create connect session", method: "POST", path: "/v1/connect/sessions", href: "/docs/api/connect/sessions/create", description: "Create a hosted connection session." },
      { label: "Get connect session", method: "GET", path: "/v1/connect/sessions/{id}", href: "/docs/api/connect/sessions/get", description: "Fetch session status." },
    ],
  },
  {
    title: "Workspace, Users, And Webhooks",
    description: "Inspect workspace settings, sync managed users, and subscribe external systems to UniPost events.",
    endpoints: [
      { label: "Get workspace", method: "GET", path: "/v1/workspace", href: "/docs/api/workspace/get", description: "Fetch workspace metadata." },
      { label: "Update workspace", method: "PATCH", path: "/v1/workspace", href: "/docs/api/workspace/update", description: "Update workspace metadata." },
      { label: "Platform credentials", method: "POST", path: "/v1/platform-credentials", href: "/docs/api/platform-credentials", description: "Save workspace-owned platform OAuth credentials." },
      { label: "List users", method: "GET", path: "/v1/users", href: "/docs/api/users/list", description: "List managed users." },
      { label: "Get user", method: "GET", path: "/v1/users/{external_user_id}", href: "/docs/api/users/get", description: "Fetch one managed user." },
      { label: "Create webhook", method: "POST", path: "/v1/webhooks", href: "/docs/api/webhooks/create", description: "Create a webhook subscription." },
      { label: "List webhooks", method: "GET", path: "/v1/webhooks", href: "/docs/api/webhooks/list", description: "List webhook subscriptions." },
      { label: "Get webhook", method: "GET", path: "/v1/webhooks/{id}", href: "/docs/api/webhooks/get", description: "Fetch one webhook subscription." },
      { label: "Update webhook", method: "PATCH", path: "/v1/webhooks/{id}", href: "/docs/api/webhooks/update", description: "Update webhook subscription settings." },
      { label: "Rotate webhook secret", method: "POST", path: "/v1/webhooks/{id}/rotate", href: "/docs/api/webhooks/rotate", description: "Rotate a webhook signing secret." },
    ],
  },
];

export default function ApiReferenceIndexPage() {
  return (
    <ApiReferencePage
      breadcrumbItems={[{ label: "API Reference" }]}
      section="api"
      title="API Reference"
      description="Explore the public UniPost API by resource. Each endpoint page includes authentication, parameters, responses, and runnable examples."
    >
      <div style={{ display: "grid", gap: 26 }}>
        {ENDPOINT_GROUPS.map((group) => (
          <section key={group.title} aria-labelledby={`api-group-${group.title.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`}>
            <div style={{ display: "grid", gridTemplateColumns: "minmax(180px, 0.32fr) minmax(0, 1fr)", gap: 18, alignItems: "start" }}>
              <div>
                <h2 id={`api-group-${group.title.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`} style={{ fontSize: 18, lineHeight: 1.25, margin: 0, color: "var(--docs-text)", fontWeight: 720 }}>
                  {group.title}
                </h2>
                <p style={{ fontSize: 13.5, lineHeight: 1.65, color: "var(--docs-text-soft)", margin: "8px 0 0" }}>{group.description}</p>
              </div>
              <div style={{ display: "grid", gap: 8 }}>
                {group.endpoints.map((endpoint) => (
                  <Link
                    key={`${endpoint.method} ${endpoint.path}`}
                    href={endpoint.href}
                    style={{
                      display: "grid",
                      gridTemplateColumns: "minmax(178px, 0.35fr) minmax(0, 1fr)",
                      gap: 14,
                      alignItems: "center",
                      padding: "13px 14px",
                      border: "1px solid var(--docs-border)",
                      borderRadius: 8,
                      background: "var(--docs-bg-elevated)",
                      textDecoration: "none",
                      color: "inherit",
                    }}
                  >
                    <span style={{ display: "flex", alignItems: "center", gap: 10, minWidth: 0 }}>
                      <MethodBadge method={endpoint.method} />
                      <span style={{ fontSize: 13.5, color: "var(--docs-text)", fontWeight: 650, overflowWrap: "anywhere" }}>{endpoint.label}</span>
                    </span>
                    <span style={{ display: "grid", gap: 4, minWidth: 0 }}>
                      <code style={{ fontFamily: "var(--docs-mono)", fontSize: 12.5, color: "var(--docs-text-soft)", overflowWrap: "anywhere" }}>{endpoint.path}</code>
                      <span style={{ fontSize: 12.5, lineHeight: 1.5, color: "var(--docs-text-faint)" }}>{endpoint.description}</span>
                    </span>
                  </Link>
                ))}
              </div>
            </div>
          </section>
        ))}
      </div>
    </ApiReferencePage>
  );
}
