"use client";

import Link from "next/link";
import { ApiReferencePage, MethodBadge } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";
import type { PlatformAnalyticsDoc, PlatformAnalyticsEndpointDoc, PlatformAnalyticsEndpointId } from "../_data/platform-analytics-docs";

function platformBreadcrumb(platform: PlatformAnalyticsDoc, leaf?: string) {
  return [
    { label: "API Reference", href: "/docs/api" },
    { label: "Analytics", href: "/docs/api/analytics/summary" },
    { label: platform.label, href: leaf ? `/docs/api/analytics/${platform.slug}` : undefined },
    ...(leaf ? [{ label: leaf }] : []),
  ];
}

function endpointPathHint(endpoint: PlatformAnalyticsEndpointDoc) {
  return endpoint.path.replace(":account_id", "{id}").replace(":post_id", "{post_id}");
}

export function PlatformAnalyticsOverviewPage({ platform }: { platform: PlatformAnalyticsDoc }) {
  return (
    <ApiReferencePage
      breadcrumbItems={platformBreadcrumb(platform)}
      section="analytics"
      title={platform.title}
      description={platform.description}
    >
      <div style={{ display: "grid", gap: 18 }}>
        <section style={{ border: "1px solid var(--docs-border)", borderRadius: 8, padding: 18, background: "var(--docs-bg-elevated)" }}>
          <h2 style={{ margin: 0, fontSize: 16, color: "var(--docs-text)", fontWeight: 720 }}>Production readiness</h2>
          <p style={{ margin: "8px 0 0", color: "var(--docs-text-soft)", fontSize: 13.5, lineHeight: 1.65 }}>
            {platform.productionReadiness} These are public API docs for supported <code>{platform.platformName}</code> Analytics surfaces.
          </p>
          <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 12 }}>
            {platform.scopes.map((scope) => (
              <code key={scope} style={{ border: "1px solid var(--docs-border)", borderRadius: 6, padding: "5px 8px", color: "var(--docs-text)", background: "var(--docs-bg)" }}>
                {scope}
              </code>
            ))}
          </div>
        </section>

        <section style={{ display: "grid", gap: 10 }}>
          {platform.endpoints.map((endpoint) => (
            <Link
              key={endpoint.href}
              href={endpoint.href}
              style={{
                display: "grid",
                gridTemplateColumns: "minmax(230px, 0.38fr) minmax(0, 1fr)",
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
                <code style={{ fontFamily: "var(--docs-mono)", fontSize: 12.5, color: "var(--docs-text-soft)", overflowWrap: "anywhere" }}>{endpointPathHint(endpoint)}</code>
                <span style={{ fontSize: 12.5, lineHeight: 1.5, color: "var(--docs-text-faint)" }}>{endpoint.description}</span>
              </span>
            </Link>
          ))}
        </section>
      </div>
    </ApiReferencePage>
  );
}

export function PlatformAnalyticsEndpointPage({
  platform,
  endpointId,
}: {
  platform: PlatformAnalyticsDoc;
  endpointId: PlatformAnalyticsEndpointId;
}) {
  const endpoint = platform.endpoints.find((item) => item.id === endpointId);

  if (!endpoint) {
    throw new Error(`Missing ${platform.slug} analytics endpoint docs for ${endpointId}`);
  }

  return (
    <SingleEndpointReferencePage
      breadcrumbItems={platformBreadcrumb(platform, endpoint.label)}
      section="analytics"
      title={endpoint.label}
      description={`${endpoint.description} ${endpoint.scopeNote}`}
      method={endpoint.method}
      path={endpoint.path}
      requestSections={endpoint.requestSections}
      responses={endpoint.responses}
      snippets={endpoint.snippets}
      responseSnippets={endpoint.responseSnippets}
    />
  );
}
