"use client";

import Link from "next/link";
import { CodeBlock, CodeTabs as SharedCodeTabs, codeBlockStyles } from "../../_components/code-block";

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
  { match: /^POST \/v1\/social-posts\/validate$/i, href: "/docs/api/posts/validate" },
  { match: /^POST \/v1\/social-posts\/[^/]+\/publish$/i, href: "/docs/api/posts/drafts" },
  { match: /^POST \/v1\/social-posts(?:\/bulk)?$/i, href: "/docs/api/posts/create" },
  { match: /^GET \/v1\/social-posts\/[^/]+\/analytics$/i, href: "/docs/api/analytics" },
  { match: /^GET \/v1\/social-accounts(?:\/[^/]+\/health)?$/i, href: "/docs/api/accounts/list" },
  { match: /^POST \/v1\/connect\/sessions/i, href: "/docs/api/connect/sessions" },
  { match: /^POST \/v1\/webhooks\/[^/]+\/rotate$/i, href: "/docs/api/webhooks" },
  { match: /^GET \/v1\/webhooks\/[^/]+$/i, href: "/docs/api/webhooks" },
  { match: /^POST \/v1\/media$/i, href: "/docs/api/media" },
  { match: /^GET \/v1\/users/i, href: "/docs/api/users" },
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
      <code className="docs-api-inline docs-api-inline-static">
        {content}
      </code>
    );
  }

  return (
    <Link href={resolvedHref} className="docs-api-inline">
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
      <SharedCodeTabs snippets={snippets} />
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
