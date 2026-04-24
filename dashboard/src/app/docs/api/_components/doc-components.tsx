"use client";

import { useMemo, useRef, useState } from "react";
import Link from "next/link";
import { Check, ChevronRight, Copy, Play, X } from "lucide-react";
import { CodeBlock, CodeTabs as SharedCodeTabs, codeBlockStyles } from "../../_components/code-block";
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
  { match: /^POST \/v1\/(?:posts|social-posts)\/bulk$/i, href: "/docs/api/posts/bulk" },
  { match: /^PATCH \/v1\/(?:posts|social-posts)\/[^/]+$/i, href: "/docs/api/posts/update" },
  { match: /^POST \/v1\/(?:posts|social-posts)$/i, href: "/docs/api/posts/create" },
  { match: /^GET \/v1\/(?:posts|social-posts)\/[^/]+\/queue$/i, href: "/docs/api/posts/get" },
  { match: /^GET \/v1\/(?:posts|social-posts)\/[^/]+\/analytics$/i, href: "/docs/api/analytics/posts" },
  { match: /^GET \/v1\/(?:posts|social-posts)\/[^/]+$/i, href: "/docs/api/posts/get" },
  { match: /^GET \/v1\/(?:posts|social-posts)$/i, href: "/docs/api/posts/list" },
  { match: /^GET \/v1\/profiles$/i, href: "/docs/api/profiles/list" },
  { match: /^POST \/v1\/profiles$/i, href: "/docs/api/profiles/create" },
  { match: /^GET \/v1\/profiles\/[^/]+$/i, href: "/docs/api/profiles/get" },
  { match: /^PATCH \/v1\/profiles\/[^/]+$/i, href: "/docs/api/profiles/update" },
  { match: /^DELETE \/v1\/profiles\/[^/]+$/i, href: "/docs/api/profiles/delete" },
  { match: /^POST \/v1\/(?:accounts|social-accounts)\/connect$/i, href: "/docs/api/accounts/connect" },
  { match: /^DELETE \/v1\/(?:accounts|social-accounts)\/[^/]+$/i, href: "/docs/api/accounts/disconnect" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/capabilities$/i, href: "/docs/api/accounts/capabilities" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/health$/i, href: "/docs/api/accounts/health" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)\/[^/]+\/tiktok\/creator-info$/i, href: "/docs/api/accounts/tiktok-creator-info" },
  { match: /^GET \/v1\/(?:accounts|social-accounts)$/i, href: "/docs/api/accounts/list" },
  { match: /^(?:POST|GET) \/v1\/connect\/sessions(?:\/[^/]+)?$/i, href: "/docs/api/connect/sessions" },
  { match: /^POST \/v1\/webhooks$/i, href: "/docs/api/webhooks/create" },
  { match: /^GET \/v1\/webhooks$/i, href: "/docs/api/webhooks/list" },
  { match: /^GET \/v1\/webhooks\/[^/]+$/i, href: "/docs/api/webhooks/get" },
  { match: /^PATCH \/v1\/webhooks\/[^/]+$/i, href: "/docs/api/webhooks/update" },
  { match: /^DELETE \/v1\/webhooks\/[^/]+$/i, href: "/docs/api/webhooks/get" },
  { match: /^POST \/v1\/webhooks\/[^/]+\/rotate$/i, href: "/docs/api/webhooks/rotate" },
  { match: /^POST \/v1\/media$/i, href: "/docs/api/media" },
  { match: /^GET \/v1\/users/i, href: "/docs/api/users" },
  // White-label group.
  {
    match: /^(?:POST|GET|DELETE) \/v1\/workspaces\/[^/]+\/platform-credentials(?:\/[^/]+)?$/i,
    href: "/docs/api/white-label/credentials",
  },
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
            <span key={b} style={{ fontSize: 11, padding: "3px 8px", borderRadius: 5, background: "var(--docs-tech-chip)", color: "var(--docs-tech-muted)", border: "1px solid rgba(255,255,255,.08)", fontFamily: "var(--docs-mono)", fontWeight: 600 }}>{b}</span>
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
      <div style={{ border: "1px solid var(--docs-border)", borderRadius: 10, overflow: "hidden", background: "var(--docs-bg-elevated)" }}>
        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13.5 }}>
          <thead>
            <tr style={{ background: "var(--docs-bg-muted)" }}>
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
          `,
        }}
      />
      <div className="docs-api-code-tabs" style={{ minWidth: 0 }}>
        <SharedCodeTabs snippets={snippets} />
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
    <div style={{ background: "color-mix(in srgb, var(--docs-link) 7%, var(--docs-bg-elevated))", border: "1px solid color-mix(in srgb, var(--docs-link) 18%, var(--docs-border))", borderRadius: 8, padding: "14px 18px", margin: "16px 0", fontSize: 13.5, lineHeight: 1.6, color: "var(--docs-text-soft)" }}>
      {children}
    </div>
  );
}

// ── Related endpoints ──
export function RelatedEndpoints({ items }: { items: { method: string; path: string; label: string; href: string }[] }) {
  return (
    <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
      {items.map(item => (
        <Link key={item.href} href={item.href} style={{
          display: "flex", alignItems: "center", gap: 10, padding: "14px 16px", background: "var(--docs-bg-elevated)", border: "1px solid var(--docs-border)",
          borderRadius: 10, textDecoration: "none", color: "var(--docs-text-soft)", fontSize: 13, transition: "all .15s",
        }}>
          <MethodBadge method={item.method} />
          <span>{item.label}</span>
        </Link>
      ))}
    </div>
  );
}

// ── Error codes table ──
export interface ErrorCodeRow { code: string; http: number; description: string }

export function ErrorTable({ errors }: { errors: ErrorCodeRow[] }) {
  return (
    <div style={{ border: "1px solid var(--docs-border)", borderRadius: 10, overflow: "hidden", background: "var(--docs-bg-elevated)" }}>
      <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13.5 }}>
        <thead>
          <tr style={{ background: "var(--docs-bg-muted)" }}>
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

function buildRequestBodyTemplate(fields: ApiFieldItem[]) {
  const template: Record<string, unknown> = {};

  for (const field of fields) {
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

          return (
            <div
              key={`${section}-${field.name}`}
              style={{
                border: "1px solid var(--docs-border)",
                borderRadius: 14,
                padding: "12px 12px 10px",
                background: "var(--docs-bg-muted)",
              }}
            >
              <div style={{ display: "flex", alignItems: "baseline", gap: 8, flexWrap: "wrap", marginBottom: 8 }}>
                <label htmlFor={inputId} style={{ fontFamily: "var(--docs-mono)", fontSize: 12.5, fontWeight: 600, color: "var(--docs-text)" }}>
                  {normalized.label}
                </label>
                {field.type ? <span style={{ fontFamily: "var(--docs-mono)", fontSize: 11.5, color: "var(--docs-text-muted)" }}>{field.type}</span> : null}
                <span
                  style={{
                    fontSize: 10.5,
                    fontWeight: 600,
                    color: normalized.optional ? "var(--docs-text-faint)" : "var(--docs-text)",
                    fontFamily: "var(--docs-mono)",
                    textTransform: "uppercase",
                    letterSpacing: ".04em",
                  }}
                >
                  {normalized.optional ? "Optional" : "Required"}
                </span>
                {field.meta ? <span style={{ fontSize: 12, color: "var(--docs-text-faint)" }}>{field.meta}</span> : null}
              </div>
              <input
                id={inputId}
                type={section === "auth" ? "password" : "text"}
                placeholder={buildFieldPlaceholder(field, section)}
                spellCheck={false}
                autoComplete="off"
                value={values[fieldKey] || ""}
                onChange={(event) => onValueChange(fieldKey, event.target.value)}
                style={{
                  width: "100%",
                  borderRadius: 10,
                  border: "1px solid var(--docs-border)",
                  background: "var(--docs-bg-elevated)",
                  color: "var(--docs-text)",
                  fontSize: 12.5,
                  lineHeight: 1.5,
                  padding: "9px 11px",
                  outline: "none",
                  fontFamily: "var(--docs-mono)",
                }}
              />
              <div style={{ fontSize: 12, lineHeight: 1.55, color: "var(--docs-text-soft)", marginTop: 8 }}>
                {field.description}
              </div>
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
      <div className="api-request-config-panel">
        <div
          style={{
            border: "1px solid var(--docs-border)",
            borderRadius: 14,
            padding: 12,
            background: "var(--docs-bg-muted)",
          }}
        >
          <div style={{ display: "flex", alignItems: "baseline", gap: 8, flexWrap: "wrap", marginBottom: 8 }}>
            <span style={{ fontFamily: "var(--docs-mono)", fontSize: 12.5, fontWeight: 500, color: "var(--docs-text-soft)" }}>body</span>
            <span style={{ fontFamily: "var(--docs-mono)", fontSize: 11.5, color: "var(--docs-text-muted)" }}>application/json</span>
          </div>
          <textarea
            value={value}
            onChange={(event) => onChange(event.target.value)}
            spellCheck={false}
            style={{
              width: "100%",
              minHeight: 180,
              resize: "vertical",
              borderRadius: 12,
              border: "1px solid var(--docs-border)",
              background: "var(--docs-tech-bg)",
              color: "var(--docs-tech-text)",
              fontSize: 12.5,
              lineHeight: 1.55,
              padding: "12px 13px",
              outline: "none",
              fontFamily: "var(--docs-mono)",
            }}
          />
          <div style={{ display: "grid", gap: 8, marginTop: 10 }}>
            {fields.map((field) => {
              const normalized = normalizeConfigFieldName(field.name);
              return (
                <div key={`body-${field.name}`}>
                  <div style={{ display: "flex", alignItems: "baseline", gap: 8, flexWrap: "wrap", marginBottom: 4 }}>
                    <span style={{ fontFamily: "var(--docs-mono)", fontSize: 12, color: "var(--docs-text)" }}>{normalized.label}</span>
                    {field.type ? <span style={{ fontFamily: "var(--docs-mono)", fontSize: 11, color: "var(--docs-text-muted)" }}>{field.type}</span> : null}
                    <span style={{ fontSize: 10.5, color: "var(--docs-text-faint)", fontFamily: "var(--docs-mono)", textTransform: "uppercase", letterSpacing: ".04em" }}>
                      {normalized.optional ? "Optional" : "Required"}
                    </span>
                  </div>
                  <div style={{ fontSize: 12, lineHeight: 1.55, color: "var(--docs-text-soft)" }}>{field.description}</div>
                </div>
              );
            })}
          </div>
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
                padding:0 16px 16px;
                display:grid;
                gap:10px;
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
            color: "var(--docs-text-faint)",
            textAlign: "center",
          }}
        >
          {baseUrl}
        </div>
        <div style={{ padding: "14px 18px", borderBottom: "1px solid var(--docs-border)", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 16 }}>
          <div style={{ minWidth: 0, display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
            <span style={{ fontFamily: "var(--docs-mono)", fontSize: 13, fontWeight: 700, color: METHOD_COLORS[method]?.text || "var(--docs-text)" }}>{method}</span>
            <code style={{ fontFamily: "var(--docs-mono)", fontSize: 13, color: "var(--docs-text-soft)", wordBreak: "break-all" }}>{pathPreview}</code>
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
  section,
  title,
  description,
  children,
}: {
  section: string;
  title: string;
  description: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <article className="docs-page docs-page-api" style={{ width: "100%" }}>
      <div style={{ padding: "10px 0 22px", borderBottom: "1px solid var(--docs-border)", marginBottom: 26 }}>
        <div style={{ fontSize: 14, fontWeight: 700, color: "#f04d23", marginBottom: 18 }}>{section}</div>
        <h1 style={{ fontSize: 42, lineHeight: 1.06, letterSpacing: "-0.045em", fontWeight: 740, margin: 0, color: "var(--docs-text)" }}>{title}</h1>
        <div style={{ fontSize: 17, lineHeight: 1.75, color: "var(--docs-text-soft)", marginTop: 18, maxWidth: "96ch" }}>{description}</div>
      </div>
      {children}
    </article>
  );
}

export function ApiReferenceGrid({
  left,
  right,
}: {
  left: React.ReactNode;
  right: React.ReactNode;
}) {
  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: "minmax(0, 1.18fr) minmax(320px, 0.82fr)",
        gap: 28,
        alignItems: "start",
      }}
      className="api-reference-grid"
    >
      <style dangerouslySetInnerHTML={{ __html: "@media (max-width: 1080px){.api-reference-grid{grid-template-columns:1fr!important;}}" }} />
      <div style={{ minWidth: 0 }}>{left}</div>
      <div style={{ minWidth: 0 }}>{right}</div>
    </div>
  );
}

export function ApiEndpointCard({
  method,
  path,
  children,
}: {
  method: string;
  path: string;
  children: React.ReactNode;
}) {
  return (
    <div style={{ border: "1px solid var(--docs-border)", borderRadius: 20, background: "var(--docs-bg-elevated)", boxShadow: "var(--docs-card-shadow)", overflow: "hidden" }}>
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
    <div>
      {title ? <h3 style={{ fontSize: 16, fontWeight: 700, color: "var(--docs-text)", margin: "0 0 14px" }}>{title}</h3> : null}
      <div style={{ display: "grid", gap: 14 }}>
        {items.map((item) => (
          <div key={item.name}>
            <div style={{ display: "flex", alignItems: "baseline", gap: 10, flexWrap: "wrap", marginBottom: 6 }}>
              <span style={{ fontFamily: "var(--docs-mono)", fontSize: 15, fontWeight: 700, color: "#f04d23" }}>{item.name}</span>
              {item.type ? <span style={{ fontFamily: "var(--docs-mono)", fontSize: 13, color: "var(--docs-text-muted)" }}>{item.type}</span> : null}
              {item.meta ? <span style={{ fontSize: 12.5, color: "var(--docs-text-faint)" }}>{item.meta}</span> : null}
            </div>
            <div style={{ fontSize: 15, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>{item.description}</div>
          </div>
        ))}
      </div>
    </div>
  );
}
