"use client";

import Link from "next/link";
import { ApiReferencePage, MethodBadge } from "../_components/doc-components";

const ENDPOINTS = [
  { label: "List logs", method: "GET" as const, path: "/v1/logs", href: "/docs/api/logs/list", description: "Cursor-paginated search and backfill across retained logs." },
  { label: "Get log", method: "GET" as const, path: "/v1/logs/{id}", href: "/docs/api/logs/get", description: "Fetch one log, including redacted request and response payloads." },
  { label: "Stream logs", method: "GET" as const, path: "/v1/logs/stream", href: "/docs/api/logs/stream", description: "Server-Sent Events stream for near real-time ingestion, with replay." },
];

const RETENTION = [
  ["free", "1 day"],
  ["api", "7 days"],
  ["basic", "14 days"],
  ["growth", "30 days"],
  ["team", "90 days"],
  ["enterprise", "180 days"],
];

function Card({ children }: { children: React.ReactNode }) {
  return (
    <div style={{ border: "1px solid var(--docs-border)", borderRadius: 12, background: "var(--docs-bg-elevated)", padding: "18px 20px" }}>
      {children}
    </div>
  );
}

export default function LogsOverviewPage() {
  return (
    <ApiReferencePage
      breadcrumbItems={[{ label: "API Reference", href: "/docs/api" }, { label: "Logs" }]}
      section="logs"
      title="Logs"
      description="Developer Logs give every workspace an observability surface for API, publishing, OAuth, webhook, and worker activity. Query them over REST, backfill with cursor pagination, or stream them in near real time over SSE."
    >
      <div style={{ display: "grid", gap: 24 }}>
        <div style={{ display: "grid", gap: 8 }}>
          {ENDPOINTS.map((endpoint) => (
            <Link
              key={endpoint.path}
              href={endpoint.href}
              style={{
                display: "grid",
                gridTemplateColumns: "minmax(160px, 0.32fr) minmax(0, 1fr)",
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
                <span style={{ fontSize: 13.5, color: "var(--docs-text)", fontWeight: 650 }}>{endpoint.label}</span>
              </span>
              <span style={{ display: "grid", gap: 4, minWidth: 0 }}>
                <code style={{ fontFamily: "var(--docs-mono)", fontSize: 12.5, color: "var(--docs-text-soft)" }}>{endpoint.path}</code>
                <span style={{ fontSize: 12.5, lineHeight: 1.5, color: "var(--docs-text-faint)" }}>{endpoint.description}</span>
              </span>
            </Link>
          ))}
        </div>

        <Card>
          <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 10 }}>Scope and isolation</div>
          <ul style={{ margin: 0, paddingLeft: 18, display: "grid", gap: 8, fontSize: 14, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
            <li>Logs are always scoped to the workspace that owns the API key or dashboard session. The normal logs API never accepts a <code>workspace_id</code> parameter.</li>
            <li>Requesting a log that belongs to another workspace returns <code>404 NOT_FOUND</code>.</li>
            <li>Admin logs are a separate, super-admin-only surface and are never available through these endpoints.</li>
          </ul>
        </Card>

        <Card>
          <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 10 }}>Redaction and payloads</div>
          <ul style={{ margin: 0, paddingLeft: 18, display: "grid", gap: 8, fontSize: 14, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
            <li>The list endpoint never returns raw payloads. The detail endpoint includes redacted <code>request_payload</code> and <code>response_payload</code>.</li>
            <li>Redaction runs before logs are persisted. Any object key whose lowercased name contains <code>token</code>, <code>secret</code>, <code>authorization</code>, <code>cookie</code>, <code>password</code>, <code>refresh_token</code>, <code>access_token</code>, or <code>client_secret</code> is replaced with <code>[REDACTED]</code>.</li>
            <li>Payloads are truncated to keep log rows bounded.</li>
            <li>Use the <code>request_id</code> field to correlate API responses with their logs.</li>
          </ul>
        </Card>

        <Card>
          <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 10 }}>Retention by plan</div>
          <div style={{ fontSize: 14, lineHeight: 1.7, color: "var(--docs-text-soft)", marginBottom: 12 }}>
            Backfill returns every log still retained for your workspace, not all logs ever created. Retention depends on plan:
          </div>
          <div style={{ display: "grid", gridTemplateColumns: "max-content max-content", gap: "6px 24px", fontSize: 13.5 }}>
            <div style={{ fontWeight: 700, color: "var(--docs-text)" }}>Plan</div>
            <div style={{ fontWeight: 700, color: "var(--docs-text)" }}>Retention</div>
            {RETENTION.map(([plan, window]) => (
              <div key={plan} style={{ display: "contents" }}>
                <code style={{ fontFamily: "var(--docs-mono)", color: "var(--docs-text-soft)" }}>{plan}</code>
                <span style={{ color: "var(--docs-text-soft)" }}>{window}</span>
              </div>
            ))}
          </div>
        </Card>

        <Card>
          <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 10 }}>Choosing an access pattern</div>
          <div style={{ fontSize: 14, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
            Use <strong style={{ color: "var(--docs-text)" }}>REST list</strong> with <code>cursor</code> for polling and backfill. Use the <strong style={{ color: "var(--docs-text)" }}>SSE stream</strong> for near real-time ingestion. Keep using <Link href="/docs/api/webhooks" style={{ color: "var(--docs-accent)" }}>webhooks</Link> for post and account lifecycle events.
          </div>
        </Card>
      </div>
    </ApiReferencePage>
  );
}
