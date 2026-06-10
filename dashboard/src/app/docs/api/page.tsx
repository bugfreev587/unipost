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
    description: "Build reporting tables, platform summaries, exports, and refresh workflows across Instagram, Threads, Pinterest, and TikTok.",
    endpoints: [
      { label: "Workspace summary", method: "GET", path: "/v1/analytics/summary", href: "/docs/api/analytics/summary", description: "Read workspace-level analytics cards." },
      { label: "Post analytics", method: "GET", path: "/v1/posts/{post_id}/analytics", href: "/docs/api/analytics/posts", description: "Inspect one post's platform results." },
      { label: "List analytics posts", method: "GET", path: "/v1/analytics/posts", href: "/docs/api/analytics/posts-list", description: "List normalized post-level analytics rows." },
      { label: "Export analytics posts", method: "GET", path: "/v1/analytics/posts/export", href: "/docs/api/analytics/posts/export", description: "Download normalized post analytics as CSV." },
      { label: "Analytics rollup", method: "GET", path: "/v1/analytics/rollup", href: "/docs/api/analytics/rollup", description: "Read time-bucketed metrics grouped by platform, account, or status." },
      { label: "Analytics platforms", method: "GET", path: "/v1/analytics/platforms", href: "/docs/api/analytics/platforms", description: "List platform availability and analytics health." },
      { label: "Get analytics platform", method: "GET", path: "/v1/analytics/platforms/{platform}", href: "/docs/api/analytics/platforms/detail", description: "Read platform summary, trend, accounts, and top posts." },
      { label: "Request analytics refresh", method: "POST", path: "/v1/analytics/refresh", href: "/docs/api/analytics/refresh", description: "Mark matching rows stale for the refresh worker." },
      { label: "API metrics", method: "GET", path: "/v1/api-metrics/overall", href: "/docs/api/api-metrics", description: "Read workspace API latency, volume, and status metrics." },
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
