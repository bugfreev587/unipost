"use client";

import Link from "next/link";
import { ApiReferencePage, MethodBadge } from "../../_components/doc-components";

const ENDPOINTS = [
  {
    label: "Get TikTok profile",
    href: "/docs/api/analytics/tiktok/profile",
    path: "/v1/accounts/{id}/tiktok/profile",
    description: "Profile fields unlocked by user.info.profile: display name, username, avatar, bio, profile links, and verification state.",
  },
  {
    label: "Get TikTok account metrics",
    href: "/docs/api/analytics/tiktok/account-metrics",
    path: "/v1/accounts/{id}/metrics",
    description: "Follower, following, public video count, total likes, and video_count fields unlocked by user.info.stats.",
  },
  {
    label: "List TikTok public videos",
    href: "/docs/api/analytics/tiktok/videos",
    path: "/v1/accounts/{id}/tiktok/videos",
    description: "Owned public video inventory and engagement counters unlocked by video.list.",
  },
  {
    label: "Read UniPost-published post analytics",
    href: "/docs/api/analytics/posts",
    path: "/v1/posts/{post_id}/analytics",
    description: "Normalized analytics for TikTok posts published through UniPost, including views, likes, comments, shares, and native IDs.",
  },
];

export default function TikTokAnalyticsDocsPage() {
  return (
    <ApiReferencePage
      breadcrumbItems={[
        { label: "API Reference", href: "/docs/api" },
        { label: "Analytics", href: "/docs/api/analytics/summary" },
        { label: "TikTok Analytics" },
      ]}
      section="analytics"
      title="TikTok Analytics"
      description="TikTok Analytics combines profile, account statistics, public video inventory, and UniPost-published post performance. TikTok has approved the required analytics scopes for production use; access is controlled by the tiktok.analytics_scopes feature flag."
    >
      <div style={{ display: "grid", gap: 18 }}>
        <section style={{ border: "1px solid var(--docs-border)", borderRadius: 8, padding: 18, background: "var(--docs-bg-elevated)" }}>
          <h2 style={{ margin: 0, fontSize: 16, color: "var(--docs-text)", fontWeight: 720 }}>Production readiness</h2>
          <p style={{ margin: "8px 0 0", color: "var(--docs-text-soft)", fontSize: 13.5, lineHeight: 1.65 }}>
            These endpoints are public-ready: TikTok approved <code>user.info.profile</code>, <code>user.info.stats</code>, and <code>video.list</code> for the production app. Enable <code>tiktok.analytics_scopes</code> in production to request those scopes and serve TikTok-specific analytics. When the flag is off, the endpoints return <code>FEATURE_DISABLED</code> as the rollback path.
          </p>
        </section>

        <section style={{ display: "grid", gap: 10 }}>
          {ENDPOINTS.map((endpoint) => (
            <Link
              key={endpoint.href}
              href={endpoint.href}
              style={{
                display: "grid",
                gridTemplateColumns: "minmax(210px, 0.38fr) minmax(0, 1fr)",
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
                <MethodBadge method="GET" />
                <span style={{ fontSize: 13.5, color: "var(--docs-text)", fontWeight: 650, overflowWrap: "anywhere" }}>{endpoint.label}</span>
              </span>
              <span style={{ display: "grid", gap: 4, minWidth: 0 }}>
                <code style={{ fontFamily: "var(--docs-mono)", fontSize: 12.5, color: "var(--docs-text-soft)", overflowWrap: "anywhere" }}>{endpoint.path}</code>
                <span style={{ fontSize: 12.5, lineHeight: 1.5, color: "var(--docs-text-faint)" }}>{endpoint.description}</span>
              </span>
            </Link>
          ))}
        </section>
      </div>
    </ApiReferencePage>
  );
}
