"use client";

import { useMemo, useRef, useState, type WheelEvent } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Check, ChevronRight, Copy, Play, X } from "lucide-react";
import { CodeBlock, CodeTabs as SharedCodeTabs, codeBlockStyles } from "../../_components/code-block";
import { DocsContentBreadcrumb } from "../../_components/docs-content-breadcrumb";
import { JsonMonacoViewer } from "./json-monaco-viewer";

// ── Method badge ──
const METHOD_COLORS: Record<string, { bg: string; text: string }> = {
  GET: { bg: "#10b98118", text: "#10b981" },
  POST: { bg: "#3b82f618", text: "#3b82f6" },
  PUT: { bg: "#f59e0b18", text: "#f59e0b" },
  PATCH: { bg: "#f59e0b18", text: "#f59e0b" },
  DELETE: { bg: "#ef444418", text: "#ef4444" },
};

function renderBooleanGlyph(value: boolean) {
  if (value) {
    return <span style={{ display: "inline-flex", alignItems: "center", color: "#22c55e", fontWeight: 700 }}>✓</span>;
  }

  return <span style={{ display: "inline-flex", alignItems: "center", color: "#ef4444", fontWeight: 700 }}>X</span>;
}

export function MethodBadge({ method }: { method: string }) {
  const c = METHOD_COLORS[method] || METHOD_COLORS.GET;
  return (
    <span style={{ display: "inline-flex", padding: "4px 10px", borderRadius: 6, fontSize: 12, fontWeight: 700, fontFamily: "var(--docs-mono)", background: c.bg, color: c.text, letterSpacing: ".04em" }}>
      {method}
    </span>
  );
}

const ENDPOINT_DOC_LINKS: Array<{ match: RegExp; href: string }> = [
  { match: /^POST \/v1\/(?:posts|social-posts)\/validate$/i, href: "/docs/api/posts/validate" },
  { match: /^POST \/v1\/(?:posts|social-posts)\/[^/]+\/publish$/i, href: "/docs/api/posts/drafts/publish" },
  { match: /^PATCH \/v1\/(?:posts|social-posts)\/[^/]+$/i, href: "/docs/api/posts/update" },
  { match: /^POST \/v1\/(?:posts|social-posts)$/i, href: "/docs/api/posts/create" },
  { match: /^GET \/v1\/(?:posts|social-posts)\/[^/]+\/queue$/i, href: "/docs/api/posts/get" },
  { match: /^GET \/v1\/(?:posts|social-posts)\/[^/]+\/analytics$/i, href: "/docs/api/analytics/posts" },
  { match: /^GET \/v1\/analytics\/posts$/i, href: "/docs/api/analytics/posts-list" },
  { match: /^GET \/v1\/analytics\/posts\/export$/i, href: "/docs/api/analytics/posts/export" },
  { match: /^GET \/v1\/analytics\/rollup$/i, href: "/docs/api/analytics/rollup" },
  { match: /^GET \/v1\/analytics\/platforms$/i, href: "/docs/api/analytics/platforms" },
  { match: /^GET \/v1\/analytics\/platforms\/[^/]+$/i, href: "/docs/api/analytics/platforms/detail" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/instagram\/profile$/i, href: "/docs/api/analytics/instagram/profile" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/instagram\/media$/i, href: "/docs/api/analytics/instagram/media" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/threads\/profile$/i, href: "/docs/api/analytics/threads/profile" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/threads\/posts$/i, href: "/docs/api/analytics/threads/posts" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/pinterest\/boards$/i, href: "/docs/api/analytics/pinterest/boards" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/youtube\/analytics\/summary$/i, href: "/docs/api/analytics/youtube/summary" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/youtube\/analytics\/trend$/i, href: "/docs/api/analytics/youtube/trend" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/youtube\/analytics\/videos$/i, href: "/docs/api/analytics/youtube/videos" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/facebook\/page-analytics$/i, href: "/docs/api/analytics/facebook/page-analytics" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/facebook\/page-insights$/i, href: "/docs/api/analytics/facebook/page-insights" },
  { match: /^POST \/v1\/analytics\/refresh$/i, href: "/docs/api/analytics/refresh" },
  { match: /^POST \/v1\/platform-credentials$/i, href: "/docs/api/platform-credentials/create" },
  { match: /^GET \/v1\/platform-credentials$/i, href: "/docs/api/platform-credentials/list" },
  { match: /^DELETE \/v1\/platform-credentials\/[^/]+$/i, href: "/docs/api/platform-credentials/delete" },
  { match: /^GET \/v1\/(?:posts|social-posts)\/[^/]+$/i, href: "/docs/api/posts/get" },
  { match: /^GET \/v1\/(?:posts|social-posts)$/i, href: "/docs/api/posts/list" },
  { match: /^GET \/v1\/workspace$/i, href: "/docs/api/workspace/get" },
  { match: /^PATCH \/v1\/workspace$/i, href: "/docs/api/workspace/update" },
  { match: /^GET \/v1\/api-keys$/i, href: "/docs/api/api-keys/list" },
  { match: /^POST \/v1\/api-keys$/i, href: "/docs/api/api-keys/create" },
  { match: /^DELETE \/v1\/api-keys\/[^/]+$/i, href: "/docs/api/api-keys/delete" },
  { match: /^GET \/v1\/api-metrics\/overall$/i, href: "/docs/api/api-metrics/overall" },
  { match: /^GET \/v1\/api-metrics\/summary$/i, href: "/docs/api/api-metrics/summary" },
  { match: /^GET \/v1\/api-metrics\/trend$/i, href: "/docs/api/api-metrics/trend" },
  { match: /^GET \/v1\/api-metrics\/status-codes$/i, href: "/docs/api/api-metrics/status-codes" },
  { match: /^GET \/v1\/api-metrics$/i, href: "/docs/api/api-metrics/overall" },
  { match: /^GET \/v1\/billing\/x-credits$/i, href: "/docs/api/x-credits" },
  { match: /^GET \/v1\/inbox$/i, href: "/docs/api/inbox/list" },
  { match: /^POST \/v1\/inbox\/[^/]+\/reply$/i, href: "/docs/api/inbox/reply" },
  { match: /^POST \/v1\/inbox\/sync$/i, href: "/docs/api/inbox/sync" },
  { match: /^GET \/v1\/logs$/i, href: "/docs/api/logs/list" },
  { match: /^GET \/v1\/logs\/stream$/i, href: "/docs/api/logs/stream" },
  { match: /^GET \/v1\/logs\/[^/]+$/i, href: "/docs/api/logs/get" },
  { match: /^GET \/v1\/profiles$/i, href: "/docs/api/profiles/list" },
  { match: /^POST \/v1\/profiles$/i, href: "/docs/api/profiles/create" },
  { match: /^GET \/v1\/profiles\/[^/]+$/i, href: "/docs/api/profiles/get" },
  { match: /^PATCH \/v1\/profiles\/[^/]+$/i, href: "/docs/api/profiles/update" },
  { match: /^DELETE \/v1\/profiles\/[^/]+$/i, href: "/docs/api/profiles/delete" },
  { match: /^POST \/v1\/oauth\/connect$/i, href: "/docs/api/accounts/oauth-connect" },
  { match: /^GET \/v1\/profiles\/[^/]+\/oauth\/connect\/[^/]+$/i, href: "/docs/api/accounts/oauth-connect" },
  { match: /^GET \/v1\/workspaces\/[^/]+\/api-keys$/i, href: "/docs/api/api-keys/list" },
  { match: /^POST \/v1\/workspaces\/[^/]+\/api-keys$/i, href: "/docs/api/api-keys/create" },
  { match: /^DELETE \/v1\/workspaces\/[^/]+\/api-keys\/[^/]+$/i, href: "/docs/api/api-keys/delete" },
  { match: /^POST \/v1\/(?:accounts|social-accounts)\/connect$/i, href: "/docs/api/accounts/connect" },
  { match: /^DELETE \/v1\/(?:accounts|social-accounts)\/[^/]+$/i, href: "/docs/api/accounts/disconnect" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/capabilities$/i, href: "/docs/api/accounts/capabilities" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/health$/i, href: "/docs/api/accounts/health" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/metrics$/i, href: "/docs/api/accounts/metrics" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/tiktok\/creator-info$/i, href: "/docs/api/accounts/tiktok-creator-info" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/tiktok\/profile$/i, href: "/docs/api/analytics/tiktok/profile" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/tiktok\/videos$/i, href: "/docs/api/analytics/tiktok/videos" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)$/i, href: "/docs/api/accounts/list" },
  { match: /^(?:POST|GET) \/v1\/connect\/sessions(?:\/[^/]+)?$/i, href: "/docs/api/connect/sessions" },
  { match: /^POST \/v1\/webhooks$/i, href: "/docs/api/webhooks/create" },
  { match: /^GET \/v1\/webhooks$/i, href: "/docs/api/webhooks/list" },
  { match: /^GET \/v1\/webhooks\/[^/]+$/i, href: "/docs/api/webhooks/get" },
  { match: /^PATCH \/v1\/webhooks\/[^/]+$/i, href: "/docs/api/webhooks/update" },
  { match: /^DELETE \/v1\/webhooks\/[^/]+$/i, href: "/docs/api/webhooks/get" },
  { match: /^POST \/v1\/webhooks\/[^/]+\/rotate$/i, href: "/docs/api/webhooks/rotate" },
  { match: /^POST \/v1\/media\/audio-overlays$/i, href: "/docs/api/media/audio-overlays" },
  { match: /^(?:POST|GET) \/v1\/media\/gif-conversions(?:\/[^/]+)?$/i, href: "/docs/api/media/gif-conversions" },
  { match: /^POST \/v1\/media$/i, href: "/docs/api/media" },
  { match: /^GET \/v1\/users/i, href: "/docs/api/users" },
  // Legacy workspace-scoped Platform Credentials paths.
  { match: /^POST \/v1\/workspaces\/[^/]+\/platform-credentials$/i, href: "/docs/api/platform-credentials/create" },
  { match: /^GET \/v1\/workspaces\/[^/]+\/platform-credentials$/i, href: "/docs/api/platform-credentials/list" },
  { match: /^DELETE \/v1\/workspaces\/[^/]+\/platform-credentials\/[^/]+$/i, href: "/docs/api/platform-credentials/delete" },
];

function normalizeEndpointReference(value: string) {
  return value.trim().replace(/\{[^}]+\}/g, ":id");
}

function resolveEndpointDocHref(endpoint: string) {
  const normalized = normalizeEndpointReference(endpoint);
  return ENDPOINT_DOC_LINKS.find((item) => item.match.test(normalized))?.href;
}

export function ApiInlineLink({
  endpoint,
  href,
}: {
  endpoint: string;
  href?: string;
}) {
  const resolvedHref = href || resolveEndpointDocHref(endpoint);
  const trimmed = endpoint.trim();
  const [method, ...rest] = trimmed.split(" ");
  const path = rest.join(" ");
  const isPathOnly = trimmed.startsWith("/v1/");
  const methodClassName = !isPathOnly && method ? `docs-api-inline-${method.toLowerCase()}` : "";
  const content = (
    <>
      <span className="docs-api-inline-glow" />
      <span className="docs-api-inline-label">
        {isPathOnly ? (
          <span className="docs-api-inline-path docs-api-inline-path-only">{trimmed}</span>
        ) : (
          <>
            <span className="docs-api-inline-method">{method}</span>
            {path ? <span className="docs-api-inline-path">{path}</span> : null}
          </>
        )}
      </span>
    </>
  );

  if (!resolvedHref) {
    return (
      <code className={`docs-api-inline docs-api-inline-static ${methodClassName}`.trim()}>
        {content}
      </code>
    );
  }

  return (
    <Link href={resolvedHref} className={`docs-api-inline ${methodClassName}`.trim()}>
      {content}
    </Link>
  );
}

// ── Endpoint header ──
export function EndpointHeader({ method, path, description, badges }: {
  method: string; path: string; description: string; badges?: string[];
}) {
  return (
    <div style={{ background: "var(--docs-tech-bg)", border: "1px solid var(--docs-tech-border)", borderRadius: 12, padding: "24px 28px", marginBottom: 32 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 8 }}>
        <MethodBadge method={method} />
        <code style={{ fontSize: 16, fontWeight: 600, color: "var(--docs-tech-text)", fontFamily: "var(--docs-mono)" }}>{path}</code>
      </div>
      <p style={{ fontSize: 14.5, color: "var(--docs-tech-text-soft)", lineHeight: 1.6, margin: 0 }}>{description}</p>
      {badges && badges.length > 0 && (
        <div style={{ display: "flex", gap: 6, marginTop: 12 }}>
          {badges.map(b => (
            <span key={b} style={{ fontSize: 11, padding: "3px 8px", borderRadius: 5, background: "var(--docs-tech-chip)", color: "var(--docs-tech-muted)", border: "1px solid var(--docs-tech-border)", fontFamily: "var(--docs-mono)", fontWeight: 600 }}>{b}</span>
          ))}
        </div>
      )}
    </div>
  );
}

// ── Breadcrumbs ──
export function Breadcrumbs({ items }: { items: { label: string; href?: string }[] }) {
  return (
    <nav style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 12.5, color: "var(--docs-text-faint)", marginBottom: 24, fontFamily: "var(--docs-mono)" }}>
      {items.map((item, i) => (
        <span key={i} style={{ display: "flex", alignItems: "center", gap: 6 }}>
          {i > 0 && <span style={{ color: "var(--docs-text-faint)" }}>/</span>}
          {item.href ? (
            <Link href={item.href} style={{ color: "var(--docs-text-muted)", textDecoration: "none" }}>{item.label}</Link>
          ) : (
            <span style={{ color: "var(--docs-text-soft)" }}>{item.label}</span>
          )}
        </span>
      ))}
    </nav>
  );
}

// ── Section heading ──
export function DocSection({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section id={id} style={{ marginBottom: 48 }}>
      <h3 style={{ fontSize: 20, fontWeight: 700, letterSpacing: "-.3px", color: "var(--docs-text)", marginBottom: 16, scrollMarginTop: 80 }}>{title}</h3>
      {children}
    </section>
  );
}

// ── Param table ──
export interface ParamRow {
  name: string;
  type: string;
  required: boolean;
  description: React.ReactNode;
}

export function ParamTable({ params, title }: { params: ParamRow[]; title?: string }) {
  return (
    <div style={{ marginBottom: 24 }}>
      {title && <div style={{ fontSize: 13, fontWeight: 700, color: "var(--docs-text-muted)", marginBottom: 10, fontFamily: "var(--docs-mono)" }}>{title}</div>}
      <div style={{ border: "none", borderRadius: 0, overflow: "visible", background: "transparent" }}>
        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13.5 }}>
          <thead>
            <tr style={{ background: "transparent" }}>
              <th style={{ textAlign: "left", padding: "10px 14px", color: "var(--docs-text-faint)", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid var(--docs-border)" }}>Parameter</th>
              <th style={{ textAlign: "left", padding: "10px 14px", color: "var(--docs-text-faint)", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid var(--docs-border)" }}>Type</th>
              <th style={{ textAlign: "left", padding: "10px 14px", color: "var(--docs-text-faint)", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid var(--docs-border)" }}>Required</th>
              <th style={{ textAlign: "left", padding: "10px 14px", color: "var(--docs-text-faint)", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid var(--docs-border)" }}>Description</th>
            </tr>
          </thead>
          <tbody>
            {params.map((p, i) => (
              <tr key={p.name} style={{ borderBottom: i < params.length - 1 ? "1px solid var(--docs-border)" : undefined }}>
                <td style={{ padding: "10px 14px", fontFamily: "var(--docs-mono)", color: "var(--docs-text)", fontWeight: 500 }}>{p.name}</td>
                <td style={{ padding: "10px 14px", fontFamily: "var(--docs-mono)", color: "var(--docs-accent)", fontSize: 12 }}>{p.type}</td>
                <td style={{ padding: "10px 14px" }}>{renderBooleanGlyph(p.required)}</td>
                <td style={{ padding: "10px 14px", color: "var(--docs-text-soft)", lineHeight: 1.5 }}>{p.description}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ── Code tabs with copy ──
export function CodeTabs({ snippets }: { snippets: { lang: string; label: string; code: string }[] }) {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: codeBlockStyles() }} />
      <style
        dangerouslySetInnerHTML={{
          __html: `
            .docs-api-code-tabs .docs-code-tabs{
              margin:0;
              width:100%;
              min-width:0;
            }
            .docs-api-code-tabs{
              --docs-code-frame-bg: var(--docs-tech-bg);
              --docs-code-header-bg: var(--docs-tech-bg);
              --docs-code-tab-bg: var(--docs-bg-elevated);
            }
            .docs-api-code-tabs ::selection,
            .docs-api-code-tabs ::-moz-selection{
              background: rgba(96, 165, 250, 0.24) !important;
              background-color: rgba(96, 165, 250, 0.24) !important;
              color: inherit !important;
            }
            .docs-api-code-tabs .monaco-editor{
              --vscode-editor-selectionBackground: rgba(96, 165, 250, 0.32) !important;
              --vscode-editor-inactiveSelectionBackground: rgba(96, 165, 250, 0.18) !important;
              --vscode-editor-selectionHighlightBackground: rgba(96, 165, 250, 0.16) !important;
            }
            html.dark .docs-api-code-tabs .monaco-editor{
              --vscode-editor-selectionBackground: rgba(124, 178, 255, 0.36) !important;
              --vscode-editor-inactiveSelectionBackground: rgba(124, 178, 255, 0.2) !important;
              --vscode-editor-selectionHighlightBackground: rgba(124, 178, 255, 0.18) !important;
            }
            .docs-api-code-tabs .monaco-editor .focused .selected-text,
            .docs-api-code-tabs .monaco-editor .selected-text{
              background-color: rgba(96, 165, 250, 0.32) !important;
            }
            html.dark .docs-api-code-tabs .monaco-editor .focused .selected-text,
            html.dark .docs-api-code-tabs .monaco-editor .selected-text{
              background-color: rgba(124, 178, 255, 0.36) !important;
            }
          `,
        }}
      />
      <div className="docs-api-code-tabs" style={{ minWidth: 0 }}>
        <SharedCodeTabs snippets={snippets} viewerMaxHeight={10000} themeVariant="api" />
      </div>
    </>
  );
}

// ── JSON response block ──
export function ResponseBlock({ title, code }: { title: string; code: string }) {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: codeBlockStyles() }} />
      <CodeBlock code={code} language="json" title={title} />
    </>
  );
}

// ── Info box ──
export function InfoBox({ children }: { children: React.ReactNode }) {
  return (
    <div className="docs-callout docs-callout-compact">
      {children}
    </div>
  );
}

// ── Error codes table ──
export interface ErrorCodeRow { code: string; http: number; description: string }

export function ErrorTable({ errors }: { errors: ErrorCodeRow[] }) {
  return (
    <div style={{ border: "none", borderRadius: 0, overflow: "visible", background: "transparent" }}>
      <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13.5 }}>
        <thead>
          <tr style={{ background: "transparent" }}>
            <th style={{ textAlign: "left", padding: "10px 14px", color: "var(--docs-text-faint)", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid var(--docs-border)" }}>Code</th>
            <th style={{ textAlign: "left", padding: "10px 14px", color: "var(--docs-text-faint)", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid var(--docs-border)" }}>HTTP</th>
            <th style={{ textAlign: "left", padding: "10px 14px", color: "var(--docs-text-faint)", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid var(--docs-border)" }}>Description</th>
          </tr>
        </thead>
        <tbody>
          {errors.map((e, i) => (
            <tr key={e.code} style={{ borderBottom: i < errors.length - 1 ? "1px solid var(--docs-border)" : undefined }}>
              <td style={{ padding: "10px 14px", fontFamily: "var(--docs-mono)", color: "#ef4444", fontSize: 12, fontWeight: 500 }}>{e.code}</td>
              <td style={{ padding: "10px 14px", fontFamily: "var(--docs-mono)", color: "#f59e0b", fontSize: 12 }}>{e.http}</td>
              <td style={{ padding: "10px 14px", color: "var(--docs-text-soft)", lineHeight: 1.5 }}>{e.description}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ── Changelog entry ──
export function ChangelogEntry({ version, date, items }: { version: string; date: string; items: string[] }) {
  return (
    <div style={{ marginBottom: 16 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 6 }}>
        <span style={{ fontSize: 13, fontWeight: 700, fontFamily: "var(--docs-mono)", color: "var(--docs-accent)" }}>{version}</span>
        <span style={{ fontSize: 12, color: "var(--docs-text-faint)" }}>({date})</span>
      </div>
      <ul style={{ margin: 0, paddingLeft: 16 }}>
        {items.map((item, i) => <li key={i} style={{ fontSize: 13, color: "var(--docs-text-soft)", lineHeight: 1.6, marginBottom: 2 }}>{item}</li>)}
      </ul>
    </div>
  );
}

export interface ApiFieldItem {
  name: string;
  type?: string;
  description: React.ReactNode;
  meta?: string;
  defaultValue?: React.ReactNode;
  optional?: boolean;
}

export type ApiGuideLink = {
  label: string;
  href: string;
};

export function EnumValues({
  values,
  label = "Values",
}: {
  values: string[];
  label?: string;
}) {
  return (
    <div style={{ marginTop: 8, display: "flex", flexWrap: "wrap", alignItems: "center", gap: 8 }}>
      <span
        style={{
          fontSize: 10.5,
          fontWeight: 700,
          letterSpacing: ".08em",
          textTransform: "uppercase",
          color: "var(--docs-text-faint)",
          fontFamily: "var(--docs-mono)",
        }}
      >
        {label}
      </span>
      <span
        style={{
          fontSize: 12.5,
          fontWeight: 700,
          color: "var(--docs-accent)",
          fontFamily: "var(--docs-mono)",
          lineHeight: 1.6,
        }}
      >
        {values.join(" | ")}
      </span>
    </div>
  );
}

function normalizeConfigFieldName(name: string) {
  if (name.endsWith("?")) {
    return {
      label: name.slice(0, -1),
      optional: true,
    };
  }

  return {
    label: name,
    optional: false,
  };
}

function buildFieldPlaceholder(field: ApiFieldItem, section: "auth" | "path" | "query") {
  if (section === "auth") {
    return field.type ? `Enter ${field.type}` : "Enter value";
  }
  if (field.type === "string[]") {
    return "Comma-separated values";
  }
  if (field.type === "integer" || field.type === "number") {
    return "Enter number";
  }
  return "Enter value";
}

function buildFieldKey(name: string) {
  return normalizeConfigFieldName(name).label;
}

function buildJsonTemplateValue(type?: string) {
  const normalized = (type || "").trim().toLowerCase();

  if (!normalized) {
    return "";
  }
  if (normalized.includes('"draft"')) {
    return "draft";
  }
  if (normalized.includes("boolean")) {
    return false;
  }
  if (normalized.includes("integer") || normalized.includes("number")) {
    return 0;
  }
  if (normalized.includes("string[]")) {
    return [];
  }
  if (normalized === "array" || normalized.includes("[]")) {
    return [];
  }
  if (normalized.includes("object")) {
    return {};
  }
  if (normalized.includes("null")) {
    return null;
  }
  return "";
}

function shouldRenderBodyInput(type?: string) {
  const normalized = (type || "").trim().toLowerCase();
  return !(
    normalized === "array"
    || normalized.includes("[]")
    || normalized.includes("object")
  );
}

function readBodyFieldValue(value: string, key: string) {
  try {
    const parsed = JSON.parse(value || "{}");
    const fieldValue = parsed?.[key];
    if (fieldValue === undefined || fieldValue === null) return "";
    if (typeof fieldValue === "string") return fieldValue;
    return String(fieldValue);
  } catch {
    return "";
  }
}

function writeBodyFieldValue(value: string, key: string, nextValue: string, type?: string) {
  let parsed: Record<string, unknown> = {};
  try {
    parsed = JSON.parse(value || "{}");
  } catch {
    parsed = {};
  }

  const normalized = (type || "").trim().toLowerCase();
  if (normalized.includes("boolean")) {
    parsed[key] = nextValue === "true";
  } else if (normalized.includes("integer") || normalized.includes("number")) {
    parsed[key] = nextValue.trim() ? Number(nextValue) : 0;
  } else {
    parsed[key] = nextValue;
  }

  return JSON.stringify(parsed, null, 2);
}

function readBodyComplexFieldValue(value: string, key: string, type?: string) {
  try {
    const parsed = JSON.parse(value || "{}");
    const fieldValue = parsed?.[key];
    if (fieldValue === undefined || fieldValue === null) {
      return (type || "").includes("[]") ? "" : JSON.stringify(buildJsonTemplateValue(type), null, 2);
    }
    if (Array.isArray(fieldValue) && (type || "").includes("[]")) {
      return fieldValue.join(", ");
    }
    return JSON.stringify(fieldValue, null, 2);
  } catch {
    return "";
  }
}

function writeBodyComplexFieldValue(value: string, key: string, nextValue: string, type?: string) {
  let parsed: Record<string, unknown> = {};
  try {
    parsed = JSON.parse(value || "{}");
  } catch {
    parsed = {};
  }

  if ((type || "").includes("[]")) {
    parsed[key] = nextValue
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean);
    return JSON.stringify(parsed, null, 2);
  }

  try {
    parsed[key] = nextValue.trim() ? JSON.parse(nextValue) : buildJsonTemplateValue(type);
  } catch {
    parsed[key] = nextValue;
  }

  return JSON.stringify(parsed, null, 2);
}

function buildRequestBodyTemplate(fields: ApiFieldItem[]) {
  const template: Record<string, unknown> = {};

  for (const field of fields) {
    if (normalizeConfigFieldName(field.name).optional) {
      continue;
    }
    const key = buildFieldKey(field.name);
    template[key] = buildJsonTemplateValue(field.type);
  }

  return JSON.stringify(template, null, 2);
}

function RequestConfigSection({
  title,
  fields,
  section,
  values,
  onValueChange,
}: {
  title: string;
  fields: ApiFieldItem[];
  section: "auth" | "path" | "query";
  values: Record<string, string>;
  onValueChange: (key: string, value: string) => void;
}) {
  const [expandedOptionalFields, setExpandedOptionalFields] = useState<Record<string, boolean>>({});

  if (fields.length === 0) {
    return null;
  }

  return (
    <details className="api-request-config-section">
      <summary
        className="api-request-config-summary"
        style={{
          listStyle: "none",
          cursor: "pointer",
          padding: "15px 18px",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 16,
        }}
      >
        <span style={{ fontSize: 13, fontWeight: 500, color: "var(--docs-text-soft)", letterSpacing: ".01em" }}>{title}</span>
        <ChevronRight className="api-accordion-chevron" strokeWidth={2.2} />
      </summary>
      <div className="api-request-config-panel">
        {fields.map((field) => {
          const normalized = normalizeConfigFieldName(field.name);
          const inputId = `request-config-${section}-${normalized.label}`;
          const fieldKey = buildFieldKey(field.name);
          const expanded = !normalized.optional || Boolean(expandedOptionalFields[fieldKey]);

          if (normalized.optional && !expanded) {
            return (
              <div key={`${section}-${field.name}`} className="api-playground-field-row-wrap">
                <button
                  type="button"
                  className="api-playground-field-row"
                  onClick={() => setExpandedOptionalFields((current) => ({ ...current, [fieldKey]: true }))}
                  aria-expanded={false}
                >
                  <span className="api-playground-row-name">
                    <ChevronRight size={16} strokeWidth={2.2} />
                    {normalized.label}
                  </span>
                  <span className="api-playground-field-type">{field.meta ? `${field.meta} · ` : ""}{field.type || "string"}</span>
                </button>
              </div>
            );
          }

          return (
            <div
              key={`${section}-${field.name}`}
              className="api-playground-field"
            >
              <div className="api-playground-field-heading">
                <label htmlFor={inputId} className="api-playground-field-name">
                  {normalized.label}
                  {!normalized.optional ? <span className="api-playground-required">*</span> : null}
                </label>
                <span className="api-playground-field-type">{field.meta ? `${field.meta} · ` : ""}{field.type || "string"}</span>
              </div>
              <input
                id={inputId}
                type={section === "auth" ? "password" : "text"}
                placeholder={buildFieldPlaceholder(field, section)}
                spellCheck={false}
                autoComplete="off"
                value={values[fieldKey] || ""}
                onChange={(event) => onValueChange(fieldKey, event.target.value)}
                className="api-playground-input"
              />
            </div>
          );
        })}
      </div>
    </details>
  );
}

function RequestBodySection({
  fields,
  value,
  onChange,
}: {
  fields: ApiFieldItem[];
  value: string;
  onChange: (value: string) => void;
}) {
  const [editorOpen, setEditorOpen] = useState(false);
  const [expandedRows, setExpandedRows] = useState<Record<string, boolean>>({});

  if (fields.length === 0) {
    return null;
  }

  return (
    <details className="api-request-config-section">
      <summary
        className="api-request-config-summary"
        style={{
          listStyle: "none",
          cursor: "pointer",
          padding: "15px 18px",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 16,
        }}
      >
        <span style={{ fontSize: 13, fontWeight: 500, color: "var(--docs-text-soft)", letterSpacing: ".01em" }}>Request Body</span>
        <ChevronRight className="api-accordion-chevron" strokeWidth={2.2} />
      </summary>
      <div className="api-request-config-panel api-request-body-panel">
        <button type="button" className="api-playground-json-button" onClick={() => setEditorOpen((current) => !current)}>
          {editorOpen ? "Close JSON Editor" : "Open JSON Editor"}
        </button>
        {editorOpen ? (
          <textarea
            value={value}
            onChange={(event) => onChange(event.target.value)}
            spellCheck={false}
            className="api-playground-json-editor"
          />
        ) : null}
        <div className="api-playground-body-grid">
          {fields.map((field) => {
            const normalized = normalizeConfigFieldName(field.name);
            const fieldKey = buildFieldKey(field.name);
            const inputId = `request-config-body-${normalized.label}`;
            const canInput = shouldRenderBodyInput(field.type);
            const expanded = !normalized.optional || Boolean(expandedRows[fieldKey]);

            if (!canInput) {
              const isStringArray = (field.type || "").includes("[]");

              return (
                <div key={`body-${field.name}`} className="api-playground-field-row-wrap">
                  <button
                    type="button"
                    className={`api-playground-field-row${expanded ? " expanded" : ""}`}
                    onClick={() => setExpandedRows((current) => ({ ...current, [fieldKey]: !current[fieldKey] }))}
                    aria-expanded={expanded}
                  >
                    <span className="api-playground-row-name">
                      <ChevronRight size={16} strokeWidth={2.2} />
                      {normalized.label}
                    </span>
                    <span className="api-playground-field-type">{field.type || "object"}</span>
                  </button>
                  {expanded ? (
                    <div className="api-playground-row-editor">
                      {isStringArray ? (
                        <input
                          type="text"
                          placeholder="value_one, value_two"
                          spellCheck={false}
                          autoComplete="off"
                          value={readBodyComplexFieldValue(value, fieldKey, field.type)}
                          onChange={(event) => onChange(writeBodyComplexFieldValue(value, fieldKey, event.target.value, field.type))}
                          className="api-playground-input"
                        />
                      ) : (
                        <textarea
                          value={readBodyComplexFieldValue(value, fieldKey, field.type)}
                          onChange={(event) => onChange(writeBodyComplexFieldValue(value, fieldKey, event.target.value, field.type))}
                          spellCheck={false}
                          className="api-playground-json-editor compact"
                        />
                      )}
                    </div>
                  ) : null}
                </div>
              );
            }

            if (normalized.optional && !expanded) {
              return (
                <div key={`body-${field.name}`} className="api-playground-field-row-wrap">
                  <button
                    type="button"
                    className="api-playground-field-row"
                    onClick={() => setExpandedRows((current) => ({ ...current, [fieldKey]: true }))}
                    aria-expanded={false}
                  >
                    <span className="api-playground-row-name">
                      <ChevronRight size={16} strokeWidth={2.2} />
                      {normalized.label}
                    </span>
                    <span className="api-playground-field-type">{field.type || "string"}</span>
                  </button>
                </div>
              );
            }

            return (
              <div key={`body-${field.name}`} className="api-playground-field">
                <div className="api-playground-field-heading">
                  <label htmlFor={inputId} className="api-playground-field-name">
                    {normalized.label}
                    {!normalized.optional ? <span className="api-playground-required">*</span> : null}
                  </label>
                  <span className="api-playground-field-type">{field.type || "string"}</span>
                </div>
                <input
                  id={inputId}
                  type={field.type?.includes("boolean") ? "text" : field.type?.includes("integer") ? "number" : "text"}
                  placeholder="Enter value"
                  spellCheck={false}
                  autoComplete="off"
                  value={readBodyFieldValue(value, fieldKey)}
                  onChange={(event) => onChange(writeBodyFieldValue(value, fieldKey, event.target.value, field.type))}
                  className="api-playground-input"
                />
              </div>
            );
          })}
        </div>
      </div>
    </details>
  );
}

export function ApiRequestConfigCard({
  method,
  path,
  requestPathTemplate,
  baseUrl = "https://api.unipost.dev",
  authFields = [],
  pathFields = [],
  queryFields = [],
  bodyFields = [],
  useMonacoForJsonResponse = false,
}: {
  method: string;
  path: string;
  requestPathTemplate?: string;
  baseUrl?: string;
  authFields?: ApiFieldItem[];
  pathFields?: ApiFieldItem[];
  queryFields?: ApiFieldItem[];
  bodyFields?: ApiFieldItem[];
  useMonacoForJsonResponse?: boolean;
}) {
  const [authValues, setAuthValues] = useState<Record<string, string>>({});
  const [pathValues, setPathValues] = useState<Record<string, string>>({});
  const [queryValues, setQueryValues] = useState<Record<string, string>>({});
  const [bodyValue, setBodyValue] = useState(() => buildRequestBodyTemplate(bodyFields));
  const [isRunning, setIsRunning] = useState(false);
  const [responseOpen, setResponseOpen] = useState(false);
  const [responseStatus, setResponseStatus] = useState<number | null>(null);
  const [responseBody, setResponseBody] = useState("{}");
  const [responseCopied, setResponseCopied] = useState(false);
  const [modalPosition, setModalPosition] = useState({ x: 80, y: 80 });
  const [modalSize, setModalSize] = useState({ width: 720, height: 560 });
  const dragState = useRef<{ offsetX: number; offsetY: number; dragging: boolean }>({
    offsetX: 0,
    offsetY: 0,
    dragging: false,
  });
  const resizeState = useRef<{ startX: number; startY: number; startWidth: number; startHeight: number; resizing: boolean }>({
    startX: 0,
    startY: 0,
    startWidth: 720,
    startHeight: 560,
    resizing: false,
  });

  const pathPreview = requestPathTemplate || path;
  const queryParamKeys = useMemo(() => queryFields.map((field) => buildFieldKey(field.name)), [queryFields]);
  const hasBody = bodyFields.length > 0;

  const requestUrl = useMemo(() => {
    const resolvedPath = pathFields.reduce((acc, field) => {
      const key = buildFieldKey(field.name);
      const value = pathValues[key]?.trim();
      return acc.replace(`:${key}`, value || `:${key}`);
    }, path);

    const params = new URLSearchParams();
    for (const key of queryParamKeys) {
      const value = queryValues[key]?.trim();
      if (value) {
        params.set(key, value);
      }
    }

    const queryString = params.toString();
    return `${baseUrl}${resolvedPath}${queryString ? `?${queryString}` : ""}`;
  }, [baseUrl, path, pathFields, pathValues, queryParamKeys, queryValues]);

  async function handleRun() {
    try {
      setIsRunning(true);

      const headers: Record<string, string> = {};
      for (const field of authFields) {
        const key = buildFieldKey(field.name);
        const rawValue = authValues[key]?.trim();
        if (!rawValue) {
          continue;
        }

        if (key.toLowerCase() === "authorization") {
          headers.Authorization = /^Bearer\s+/i.test(rawValue) ? rawValue : `Bearer ${rawValue}`;
          continue;
        }

        headers[key] = rawValue;
      }

      if (hasBody && !headers["Content-Type"]) {
        headers["Content-Type"] = "application/json";
      }

      let requestBody: string | undefined;
      if (hasBody) {
        requestBody = bodyValue.trim() ? JSON.stringify(JSON.parse(bodyValue), null, 2) : "{}";
      }

      const response = await fetch(requestUrl, {
        method,
        headers,
        body: requestBody,
      });

      const text = await response.text();
      let formatted = text;
      try {
        formatted = JSON.stringify(JSON.parse(text), null, 2);
      } catch {
        formatted = JSON.stringify({ raw: text || "" }, null, 2);
      }

      setResponseStatus(response.status);
      setResponseBody(formatted);
      setResponseOpen(true);
    } catch (error) {
      const message = error instanceof Error ? error.message : "Unknown request error";
      setResponseStatus(null);
      setResponseBody(JSON.stringify({ error: message }, null, 2));
      setResponseOpen(true);
    } finally {
      setIsRunning(false);
    }
  }

  async function handleCopyResponse() {
    await navigator.clipboard.writeText(responseBody);
    setResponseCopied(true);
    window.setTimeout(() => setResponseCopied(false), 1600);
  }

  function startDrag(event: React.PointerEvent<HTMLDivElement>) {
    dragState.current = {
      dragging: true,
      offsetX: event.clientX - modalPosition.x,
      offsetY: event.clientY - modalPosition.y,
    };

    const move = (moveEvent: PointerEvent) => {
      if (!dragState.current.dragging) {
        return;
      }
      setModalPosition({
        x: Math.max(16, moveEvent.clientX - dragState.current.offsetX),
        y: Math.max(16, moveEvent.clientY - dragState.current.offsetY),
      });
    };

    const stop = () => {
      dragState.current.dragging = false;
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", stop);
    };

    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", stop);
  }

  function startResize(event: React.PointerEvent<HTMLButtonElement>) {
    event.stopPropagation();
    resizeState.current = {
      resizing: true,
      startX: event.clientX,
      startY: event.clientY,
      startWidth: modalSize.width,
      startHeight: modalSize.height,
    };

    const move = (moveEvent: PointerEvent) => {
      if (!resizeState.current.resizing) {
        return;
      }
      const nextWidth = resizeState.current.startWidth + (moveEvent.clientX - resizeState.current.startX);
      const nextHeight = resizeState.current.startHeight + (moveEvent.clientY - resizeState.current.startY);
      setModalSize({
        width: Math.min(Math.max(nextWidth, 420), window.innerWidth - 32),
        height: Math.min(Math.max(nextHeight, 280), window.innerHeight - 32),
      });
    };

    const stop = () => {
      resizeState.current.resizing = false;
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", stop);
    };

    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", stop);
  }

  const sections = [
    authFields.length > 0 ? <RequestConfigSection key="auth" title="Authorization" fields={authFields} section="auth" values={authValues} onValueChange={(key, value) => setAuthValues((current) => ({ ...current, [key]: value }))} /> : null,
    pathFields.length > 0 ? <RequestConfigSection key="path" title="Path" fields={pathFields} section="path" values={pathValues} onValueChange={(key, value) => setPathValues((current) => ({ ...current, [key]: value }))} /> : null,
    queryFields.length > 0 ? <RequestConfigSection key="query" title="Query" fields={queryFields} section="query" values={queryValues} onValueChange={(key, value) => setQueryValues((current) => ({ ...current, [key]: value }))} /> : null,
    bodyFields.length > 0 ? <RequestBodySection key="body" fields={bodyFields} value={bodyValue} onChange={setBodyValue} /> : null,
  ].filter(Boolean);

  if (sections.length === 0) {
    return null;
  }

  return (
    <>
      <ApiEndpointCard method="" path="">
        <style
          dangerouslySetInnerHTML={{
            __html: `
              .api-request-config-section > summary::-webkit-details-marker{display:none}
              .api-request-config-section .api-accordion-chevron{
                width:18px;
                height:18px;
                color:var(--docs-text-muted);
                flex-shrink:0;
                transition:transform .18s ease,color .18s ease;
                transform:rotate(0deg);
                transform-origin:50% 50%;
              }
              .api-request-config-section[open] .api-accordion-chevron{
                transform:rotate(90deg);
                color:var(--docs-text-faint);
              }
              .api-request-config-section .api-request-config-summary:hover .api-accordion-chevron{
                color:var(--docs-text);
              }
              .api-request-config-section + .api-request-config-section{
                border-top:1px solid var(--docs-border);
              }
              .api-request-config-section .api-request-config-panel{
                padding:0 18px 18px;
                display:grid;
                gap:14px;
              }
              .api-request-body-panel{
                padding-top:2px!important;
              }
              .api-playground-field{
                min-width:0;
              }
              .api-playground-field-heading{
                display:flex;
                align-items:baseline;
                justify-content:space-between;
                gap:12px;
                margin-bottom:8px;
              }
              .api-playground-field-name,
              .api-playground-row-name{
                display:inline-flex;
                align-items:center;
                gap:6px;
                min-width:0;
                color:var(--docs-text);
                font-family:var(--docs-mono);
                font-size:13px;
                font-weight:680;
                line-height:1.25;
              }
              .api-playground-required{
                color:#ff6b6b;
                font-family:var(--docs-ui);
                font-weight:760;
              }
              .api-playground-field-type{
                flex-shrink:0;
                color:var(--docs-text-muted);
                font-family:var(--docs-mono);
                font-size:12px;
                font-weight:560;
                line-height:1.25;
              }
              .api-playground-input,
              .api-playground-json-editor{
                width:100%;
                border:1px solid var(--docs-border);
                border-radius:8px;
                background:color-mix(in srgb, var(--docs-bg-muted) 58%, var(--docs-bg-elevated));
                color:var(--docs-text);
                font-family:var(--docs-mono);
                font-size:13px;
                line-height:1.5;
                outline:none;
              }
              .api-playground-input{
                height:42px;
                padding:0 12px;
              }
              .api-playground-input:focus,
              .api-playground-json-editor:focus{
                border-color:color-mix(in srgb, #3b82f6 44%, var(--docs-border));
                box-shadow:0 0 0 3px color-mix(in srgb, #3b82f6 12%, transparent);
              }
              .api-playground-input::placeholder{
                color:var(--docs-text-faint);
              }
              .api-playground-body-grid{
                display:grid;
                grid-template-columns:repeat(2,minmax(0,1fr));
                gap:18px 20px;
              }
              .api-playground-field-row{
                display:flex;
                align-items:center;
                justify-content:space-between;
                gap:16px;
                width:100%;
                padding:4px 0;
                border:0;
                background:transparent;
                color:inherit;
                text-align:left;
                cursor:pointer;
              }
              .api-playground-field-row-wrap{
                grid-column:1/-1;
                display:grid;
                gap:10px;
              }
              .api-playground-field-row.expanded svg{
                transform:rotate(90deg);
              }
              .api-playground-field-row svg{
                color:var(--docs-text-muted);
                transition:transform .16s ease;
              }
              .api-playground-row-editor{
                padding-left:26px;
              }
              .api-playground-json-button{
                justify-self:start;
                border:1px solid var(--docs-border);
                border-radius:8px;
                background:var(--docs-bg-muted);
                color:var(--docs-text);
                font-family:var(--docs-mono);
                font-size:12px;
                font-weight:680;
                line-height:1;
                padding:10px 12px;
                cursor:pointer;
              }
              .api-playground-json-button:hover{
                background:var(--docs-bg-elevated);
              }
              .api-playground-json-editor{
                grid-column:1/-1;
                min-height:160px;
                resize:vertical;
                padding:12px;
              }
              .api-playground-json-editor.compact{
                min-height:112px;
              }
              @media (max-width: 720px){
                .api-playground-body-grid{
                  grid-template-columns:1fr;
                }
              }
            `,
          }}
        />
        <div
          style={{
            padding: "8px 18px",
            borderBottom: "1px solid var(--docs-border)",
            background: "var(--docs-bg-muted)",
            fontSize: 11.5,
            fontFamily: "var(--docs-mono)",
            color: "var(--docs-text-muted)",
            textAlign: "center",
          }}
        >
          {baseUrl}
        </div>
        <div style={{ padding: "14px 18px", borderBottom: "1px solid var(--docs-border)", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 16 }}>
          <div style={{ minWidth: 0, display: "flex", alignItems: "center", gap: 12, flexWrap: "wrap" }}>
            <span style={{ fontFamily: "var(--docs-mono)", fontSize: 15, fontWeight: 700, color: METHOD_COLORS[method]?.text || "var(--docs-text)" }}>{method}</span>
            <code style={{ fontFamily: "var(--docs-mono)", fontSize: 15, color: "var(--docs-text)", wordBreak: "break-all" }}>{pathPreview}</code>
          </div>
          <button
            type="button"
            onClick={handleRun}
            disabled={isRunning}
            style={{
              flexShrink: 0,
              display: "inline-flex",
              alignItems: "center",
              gap: 6,
              border: "1px solid color-mix(in srgb, #3b82f6 26%, var(--docs-border))",
              background: isRunning ? "color-mix(in srgb, #3b82f6 14%, var(--docs-bg-muted))" : "color-mix(in srgb, #3b82f6 18%, var(--docs-bg-elevated))",
              color: "#2563eb",
              borderRadius: 10,
              padding: "8px 11px",
              fontSize: 12,
              fontWeight: 600,
              cursor: isRunning ? "wait" : "pointer",
              fontFamily: "var(--docs-mono)",
            }}
          >
            <Play size={13} />
            {isRunning ? "Running" : "Run"}
          </button>
        </div>
        <div>{sections}</div>
      </ApiEndpointCard>

      {responseOpen ? (
        <div
          style={{
            position: "fixed",
            inset: 0,
            zIndex: 120,
            pointerEvents: "none",
          }}
        >
          <div
            style={{
              position: "absolute",
              left: modalPosition.x,
              top: modalPosition.y,
              width: `min(${modalSize.width}px, calc(100vw - 32px))`,
              height: `min(${modalSize.height}px, calc(100vh - 32px))`,
              background: "var(--docs-bg-elevated)",
              border: "1px solid var(--docs-border)",
              borderRadius: 18,
              boxShadow: "var(--docs-card-shadow)",
              overflow: "hidden",
              pointerEvents: "auto",
            }}
          >
            <div
              onPointerDown={startDrag}
              style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: 12,
                padding: "12px 14px",
                borderBottom: "1px solid var(--docs-border)",
                background: "color-mix(in srgb, var(--docs-bg-muted) 74%, #d7dde9)",
                cursor: "grab",
              }}
            >
              <div style={{ display: "flex", alignItems: "center", gap: 10, minWidth: 0 }}>
                <div style={{ fontSize: 12.5, fontWeight: 600, color: "var(--docs-text-soft)" }}>Response</div>
                {responseStatus !== null ? (
                  <span style={{ fontFamily: "var(--docs-mono)", fontSize: 11.5, color: "var(--docs-text-faint)" }}>
                    HTTP {responseStatus}
                  </span>
                ) : null}
              </div>
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <button
                  type="button"
                  onClick={handleCopyResponse}
                  style={{
                    width: 32,
                    height: 32,
                    display: "inline-flex",
                    alignItems: "center",
                    justifyContent: "center",
                    borderRadius: 9,
                    border: "1px solid var(--docs-border)",
                    background: "var(--docs-bg-elevated)",
                    color: "var(--docs-text-muted)",
                    cursor: "pointer",
                  }}
                  aria-label="Copy response"
                >
                  {responseCopied ? <Check size={15} /> : <Copy size={15} />}
                </button>
                <button
                  type="button"
                  onClick={() => setResponseOpen(false)}
                  style={{
                    width: 32,
                    height: 32,
                    display: "inline-flex",
                    alignItems: "center",
                    justifyContent: "center",
                    borderRadius: 9,
                    border: "1px solid var(--docs-border)",
                    background: "var(--docs-bg-elevated)",
                    color: "var(--docs-text-muted)",
                    cursor: "pointer",
                  }}
                  aria-label="Close response"
                >
                  <X size={15} />
                </button>
              </div>
            </div>
            <div style={{ padding: 14, overflow: "auto", height: "calc(100% - 58px)" }}>
              {useMonacoForJsonResponse ? (
                <JsonMonacoViewer code={responseBody} height={Math.max(modalSize.height - 92, 180)} />
              ) : (
                <CodeBlock code={responseBody} language="json" bare />
              )}
            </div>
            <button
              type="button"
              onPointerDown={startResize}
              aria-label="Resize response"
              style={{
                position: "absolute",
                right: 6,
                bottom: 6,
                width: 18,
                height: 18,
                border: "none",
                background: "transparent",
                cursor: "nwse-resize",
                pointerEvents: "auto",
                padding: 0,
              }}
            >
              <span
                style={{
                  position: "absolute",
                  right: 1,
                  bottom: 1,
                  width: 12,
                  height: 12,
                  background:
                    "linear-gradient(135deg, transparent 0 43%, color-mix(in srgb, var(--docs-text-faint) 78%, transparent) 43% 54%, transparent 54% 64%, color-mix(in srgb, var(--docs-text-faint) 78%, transparent) 64% 75%, transparent 75%)",
                  opacity: 0.8,
                }}
              />
            </button>
          </div>
        </div>
      ) : null}
    </>
  );
}

export function ApiReferencePage({
  breadcrumbItems,
  section: _section,
  title,
  description,
  guideLinks = [],
  children,
}: {
  breadcrumbItems?: { label: string; href?: string }[];
  section: string;
  title: string;
  description: React.ReactNode;
  guideLinks?: ApiGuideLink[];
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const resolvedBreadcrumbItems = breadcrumbItems ?? buildApiReferenceBreadcrumb(pathname, title);

  return (
    <article className="docs-page docs-page-api" style={{ width: "100%" }}>
      <DocsContentBreadcrumb items={resolvedBreadcrumbItems} />
      <div className="api-reference-page-header" style={{ padding: "10px 0 22px", borderBottom: "1px solid var(--docs-border)", marginBottom: 26 }}>
        <style
          dangerouslySetInnerHTML={{
            __html: `
              .api-reference-guide-links{
                display:flex;
                flex-wrap:wrap;
                gap:8px;
                margin-top:14px;
              }
              .api-reference-guide-link{
                display:inline-flex;
                align-items:center;
                gap:8px;
                max-width:100%;
                border:1px solid color-mix(in srgb, #0f766e 26%, var(--docs-border));
                border-radius:999px;
                background:color-mix(in srgb, #0f766e 10%, var(--docs-bg-elevated));
                color:color-mix(in srgb, #0f766e 86%, var(--docs-text))!important;
                padding:5px 10px 5px 8px;
                font-size:12.5px;
                line-height:1.1;
                font-weight:650;
                text-decoration:none!important;
                transition:background .16s ease,border-color .16s ease,transform .16s ease;
              }
              .api-reference-guide-link:hover{
                background:color-mix(in srgb, #0f766e 15%, var(--docs-bg-elevated));
                border-color:color-mix(in srgb, #0f766e 38%, var(--docs-border));
                transform:translateY(-1px);
              }
              .api-reference-guide-kicker{
                border-radius:999px;
                background:color-mix(in srgb, #0f766e 18%, transparent);
                color:color-mix(in srgb, #0f766e 88%, var(--docs-text));
                padding:2px 6px;
                font-family:var(--docs-mono);
                font-size:10.5px;
                font-weight:760;
                letter-spacing:0;
                text-transform:uppercase;
              }
              .api-reference-guide-label{
                overflow:hidden;
                text-overflow:ellipsis;
                white-space:nowrap;
              }
              html.dark .api-reference-guide-link{
                border-color:color-mix(in srgb, #34d399 30%, var(--docs-border));
                background:color-mix(in srgb, #34d399 10%, var(--docs-bg-elevated));
                color:color-mix(in srgb, #a7f3d0 78%, var(--docs-text))!important;
              }
            `,
          }}
        />
        <h1 style={{ fontSize: 42, lineHeight: 1.06, letterSpacing: "-0.045em", fontWeight: 740, margin: 0, color: "var(--docs-text)" }}>{title}</h1>
        <div style={{ fontSize: 17, lineHeight: 1.75, color: "var(--docs-text-soft)", marginTop: 18, maxWidth: "96ch" }}>{description}</div>
        {guideLinks.length > 0 ? (
          <div className="api-reference-guide-links" aria-label="Related guides">
            {guideLinks.map((guide) => (
              <Link key={guide.href} href={guide.href} className="api-reference-guide-link">
                <span className="api-reference-guide-kicker">Guide</span>
                <span className="api-reference-guide-label">{guide.label}</span>
                <ChevronRight size={14} strokeWidth={2.2} aria-hidden="true" />
              </Link>
            ))}
          </div>
        ) : null}
      </div>
      {children}
    </article>
  );
}

function buildApiReferenceBreadcrumb(pathname: string, title: string) {
  const items: Array<{ label: string; href?: string }> = [];
  const sectionItem = getApiSectionBreadcrumb(pathname);

  if (sectionItem && sectionItem.label.toLowerCase() !== title.trim().toLowerCase()) {
    items.push(sectionItem);
  }

  items.push({ label: title });
  return items;
}

function getApiSectionBreadcrumb(pathname: string) {
  const matches: Array<{ prefix: string; label: string; href?: string }> = [
    { prefix: "/docs/api/connect/sessions/", label: "Connect", href: "/docs/api/connect/sessions" },
    { prefix: "/docs/api/profiles/", label: "Profiles", href: "/docs/api/profiles" },
    { prefix: "/docs/api/accounts/", label: "Accounts", href: "/docs/api/accounts/list" },
    { prefix: "/docs/api/users/", label: "Users", href: "/docs/api/users" },
    { prefix: "/docs/api/api-keys/", label: "API keys", href: "/docs/api/api-keys" },
    { prefix: "/docs/api/posts/drafts/", label: "Drafts", href: "/docs/api/posts/drafts" },
    { prefix: "/docs/api/posts/", label: "Posts", href: "/docs/api/posts/list" },
    { prefix: "/docs/api/media/", label: "Media", href: "/docs/api/media" },
    { prefix: "/docs/api/api-metrics", label: "API Metrics", href: "/docs/api/api-metrics/overall" },
    { prefix: "/docs/api/logs", label: "Logs", href: "/docs/api/logs" },
    { prefix: "/docs/api/analytics/", label: "Analytics", href: "/docs/api/analytics" },
    { prefix: "/docs/api/webhooks/", label: "Webhooks", href: "/docs/api/webhooks" },
    { prefix: "/docs/api/workspace/", label: "Workspace", href: "/docs/api/workspace/get" },
    { prefix: "/docs/api/platform-credentials", label: "Platform Credentials", href: "/docs/api/platform-credentials" },
    { prefix: "/docs/api/white-label/", label: "White-label", href: "/docs/api/white-label/branding" },
    { prefix: "/docs/api/inbox", label: "Inbox" },
  ];

  return matches.find((match) => pathname.startsWith(match.prefix));
}

export function ApiReferenceGrid({
  left,
  right,
}: {
  left: React.ReactNode;
  right: React.ReactNode;
}) {
  const rightColumnRef = useRef<HTMLDivElement | null>(null);

  function handleRightColumnWheelCapture(event: WheelEvent<HTMLDivElement>) {
    const element = rightColumnRef.current;
    if (!element) return;

    const deltaY = event.deltaY;
    if (Math.abs(deltaY) <= Math.abs(event.deltaX)) return;

    const maxScrollTop = element.scrollHeight - element.clientHeight;
    if (maxScrollTop <= 1) return;

    const canScrollUp = element.scrollTop > 0;
    const canScrollDown = element.scrollTop < maxScrollTop - 1;
    const shouldScrollRightColumn = (deltaY < 0 && canScrollUp) || (deltaY > 0 && canScrollDown);

    if (!shouldScrollRightColumn) return;

    event.preventDefault();
    event.stopPropagation();
    element.scrollTop = Math.max(0, Math.min(maxScrollTop, element.scrollTop + deltaY));
  }

  return (
    <div className="api-reference-grid-shell">
      <style
        dangerouslySetInnerHTML={{
          __html: `
            .api-reference-grid-shell{
              width:100%;
              min-width:0;
              container-type:inline-size;
            }
            .api-reference-grid{
              display:grid;
              grid-template-columns:minmax(420px, 1fr) minmax(340px, 496px);
              gap:clamp(22px, 2.4cqi, 32px);
              align-items:start;
              min-width:0;
            }
            .api-reference-grid-right{
              position:sticky;
              top:96px;
              align-self:start;
              min-width:0;
            }
            .api-reference-grid-left{
              min-width:0;
            }
            @container (max-width: 960px){
              .api-reference-grid{
                grid-template-columns:1fr!important;
              }
              .api-reference-grid-right{
                position:static!important;
                top:auto!important;
                max-height:none!important;
                overflow:visible!important;
                padding-right:0!important;
              }
            }
            @container (max-width: 620px){
              .api-reference-grid{
                gap:22px;
              }
              .api-reference-grid-right .docs-code-tabs-header{
                padding:12px 12px 0;
              }
              .api-reference-grid-right .docs-code-tabs > .docs-code-tabs-header .docs-code-tab-list{
                padding-right:0;
              }
              .api-reference-grid-right .docs-code-tabs > .docs-code-tabs-header .docs-copy-button,
              .api-reference-grid-right .docs-code-tabs > .docs-code-tabs-header .docs-expand-button{
                opacity:1;
                transform:none;
              }
              .api-reference-grid-right .docs-code-tabs > .docs-code-tabs-header .docs-copy-button{
                right:50px;
              }
              .api-reference-grid-right .docs-code-tabs > .docs-code-tabs-header .docs-expand-button{
                right:10px;
              }
            }
          `,
        }}
      />
      <div className="api-reference-grid">
        <div className="api-reference-grid-left">{left}</div>
        <div
          ref={rightColumnRef}
          className="api-reference-grid-right"
          onWheelCapture={handleRightColumnWheelCapture}
        >
          {right}
        </div>
      </div>
    </div>
  );
}

export function ApiEndpointCard({
  method: _method,
  path: _path,
  children,
}: {
  method: string;
  path: string;
  children: React.ReactNode;
}) {
  void _method;
  void _path;

  return (
    <div className="api-endpoint-card" style={{ border: "1px solid var(--docs-border)", borderRadius: 8, background: "var(--docs-bg-elevated)", boxShadow: "var(--docs-card-shadow)", overflow: "hidden" }}>
      <div>{children}</div>
    </div>
  );
}

export function ApiAccordion({
  title,
  defaultOpen = false,
  children,
}: {
  title: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}) {
  return (
    <details open={defaultOpen} className="api-accordion">
      <style
        dangerouslySetInnerHTML={{
          __html: `
            .api-accordion > summary::-webkit-details-marker{display:none}
            .api-accordion .api-accordion-chevron{
              width:18px;
              height:18px;
              color:var(--docs-text-muted);
              flex-shrink:0;
              transition:transform .18s ease,color .18s ease;
              transform:rotate(0deg);
              transform-origin:50% 50%;
            }
            .api-accordion[open] .api-accordion-chevron{
              transform:rotate(90deg);
              color:var(--docs-text-faint);
            }
            .api-accordion .api-accordion-summary:hover .api-accordion-chevron{
              color:var(--docs-text);
            }
            .api-accordion .api-accordion-panel{
              padding:2px 18px 18px 44px;
            }
          `,
        }}
      />
      <summary
        className="api-accordion-summary"
        style={{
          listStyle: "none",
          cursor: "pointer",
          padding: "15px 18px",
          fontSize: 15,
          fontWeight: 700,
          color: "var(--docs-text)",
          display: "flex",
          alignItems: "center",
          gap: 10,
          fontFamily: "var(--docs-mono)",
          letterSpacing: ".01em",
        }}
      >
        <ChevronRight className="api-accordion-chevron" strokeWidth={2.2} />
        <span>{title}</span>
      </summary>
      <div className="api-accordion-panel">{children}</div>
    </details>
  );
}

export function ApiFieldList({
  title,
  items,
}: {
  title?: string;
  items: ApiFieldItem[];
}) {
  return (
    <div className="api-field-list">
      {title ? <h3 style={{ fontSize: 16, fontWeight: 700, color: "var(--docs-text)", margin: "0 0 14px" }}>{title}</h3> : null}
      <div className="api-field-list-items" style={{ display: "grid", gap: 14 }}>
        {items.map((item) => {
          const normalized = normalizeConfigFieldName(item.name);
          const isOptional = item.optional ?? normalized.optional;

          return (
            <div key={item.name} className="api-field-row">
              <div className="api-field-row-heading" style={{ display: "flex", alignItems: "baseline", gap: 10, flexWrap: "wrap", marginBottom: 6 }}>
                <span className="api-field-name" style={{ fontFamily: "var(--docs-mono)", fontSize: 15, fontWeight: 700, color: "#f04d23" }}>{normalized.label}</span>
                {isOptional ? <span className="api-field-chip">Optional</span> : null}
                {item.defaultValue !== undefined ? <span className="api-field-chip api-field-chip-default">Default: {item.defaultValue}</span> : null}
                {item.type ? <span style={{ fontFamily: "var(--docs-mono)", fontSize: 13, color: "var(--docs-text-muted)" }}>{item.type}</span> : null}
                {item.meta ? <span style={{ fontSize: 12.5, color: "var(--docs-text-faint)" }}>{item.meta}</span> : null}
              </div>
              <div className="api-field-description" style={{ fontSize: 15, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>{item.description}</div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
