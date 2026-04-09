"use client";

import { useState, useCallback } from "react";
import Link from "next/link";

// ── Method badge ──
const METHOD_COLORS: Record<string, { bg: string; text: string }> = {
  GET: { bg: "#10b98118", text: "#10b981" },
  POST: { bg: "#3b82f618", text: "#3b82f6" },
  PUT: { bg: "#f59e0b18", text: "#f59e0b" },
  PATCH: { bg: "#f59e0b18", text: "#f59e0b" },
  DELETE: { bg: "#ef444418", text: "#ef4444" },
};

export function MethodBadge({ method }: { method: string }) {
  const c = METHOD_COLORS[method] || METHOD_COLORS.GET;
  return (
    <span style={{ display: "inline-flex", padding: "4px 10px", borderRadius: 6, fontSize: 12, fontWeight: 700, fontFamily: "var(--mono)", background: c.bg, color: c.text, letterSpacing: ".04em" }}>
      {method}
    </span>
  );
}

// ── Endpoint header ──
export function EndpointHeader({ method, path, description, badges }: {
  method: string; path: string; description: string; badges?: string[];
}) {
  return (
    <div style={{ background: "#0a0a0a", border: "1px solid #1a1a1a", borderRadius: 12, padding: "24px 28px", marginBottom: 32 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 8 }}>
        <MethodBadge method={method} />
        <code style={{ fontSize: 16, fontWeight: 600, color: "#f0f0f0", fontFamily: "var(--mono)" }}>{path}</code>
      </div>
      <p style={{ fontSize: 14.5, color: "#999", lineHeight: 1.6, margin: 0 }}>{description}</p>
      {badges && badges.length > 0 && (
        <div style={{ display: "flex", gap: 6, marginTop: 12 }}>
          {badges.map(b => (
            <span key={b} style={{ fontSize: 11, padding: "3px 8px", borderRadius: 5, background: "#ffffff08", color: "#888", border: "1px solid #222", fontFamily: "var(--mono)", fontWeight: 600 }}>{b}</span>
          ))}
        </div>
      )}
    </div>
  );
}

// ── Breadcrumbs ──
export function Breadcrumbs({ items }: { items: { label: string; href?: string }[] }) {
  return (
    <nav style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 12.5, color: "#555", marginBottom: 24, fontFamily: "var(--mono)" }}>
      {items.map((item, i) => (
        <span key={i} style={{ display: "flex", alignItems: "center", gap: 6 }}>
          {i > 0 && <span style={{ color: "#333" }}>/</span>}
          {item.href ? (
            <Link href={item.href} style={{ color: "#888", textDecoration: "none" }}>{item.label}</Link>
          ) : (
            <span style={{ color: "#ccc" }}>{item.label}</span>
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
      <h3 style={{ fontSize: 20, fontWeight: 700, letterSpacing: "-.3px", color: "#f0f0f0", marginBottom: 16, scrollMarginTop: 80 }}>{title}</h3>
      {children}
    </section>
  );
}

// ── Param table ──
export interface ParamRow {
  name: string;
  type: string;
  required: boolean;
  description: string;
}

export function ParamTable({ params, title }: { params: ParamRow[]; title?: string }) {
  return (
    <div style={{ marginBottom: 24 }}>
      {title && <div style={{ fontSize: 13, fontWeight: 700, color: "#888", marginBottom: 10, fontFamily: "var(--mono)" }}>{title}</div>}
      <div style={{ border: "1px solid #1a1a1a", borderRadius: 10, overflow: "hidden" }}>
        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13.5 }}>
          <thead>
            <tr style={{ background: "#0a0a0a" }}>
              <th style={{ textAlign: "left", padding: "10px 14px", color: "#666", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid #1a1a1a" }}>Parameter</th>
              <th style={{ textAlign: "left", padding: "10px 14px", color: "#666", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid #1a1a1a" }}>Type</th>
              <th style={{ textAlign: "left", padding: "10px 14px", color: "#666", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid #1a1a1a" }}>Required</th>
              <th style={{ textAlign: "left", padding: "10px 14px", color: "#666", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid #1a1a1a" }}>Description</th>
            </tr>
          </thead>
          <tbody>
            {params.map((p, i) => (
              <tr key={p.name} style={{ borderBottom: i < params.length - 1 ? "1px solid #111" : undefined }}>
                <td style={{ padding: "10px 14px", fontFamily: "var(--mono)", color: "#f0f0f0", fontWeight: 500 }}>{p.name}</td>
                <td style={{ padding: "10px 14px", fontFamily: "var(--mono)", color: "#10b981", fontSize: 12 }}>{p.type}</td>
                <td style={{ padding: "10px 14px", color: p.required ? "#f59e0b" : "#555" }}>{p.required ? "Yes" : "No"}</td>
                <td style={{ padding: "10px 14px", color: "#999", lineHeight: 1.5 }}>{p.description}</td>
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
  const [active, setActive] = useState(0);
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(snippets[active].code);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }, [active, snippets]);

  return (
    <div style={{ borderRadius: 10, overflow: "hidden", border: "1px solid #1a1a1a", marginBottom: 20 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", background: "#0a0a0a", borderBottom: "1px solid #1a1a1a", padding: "8px 14px" }}>
        <div style={{ display: "flex", gap: 2 }}>
          {snippets.map((s, i) => (
            <button key={s.lang} onClick={() => setActive(i)} style={{
              padding: "5px 12px", borderRadius: 6, fontSize: 12, fontWeight: 500, fontFamily: "var(--mono)",
              cursor: "pointer", border: "1px solid transparent", transition: "all .1s",
              background: i === active ? "#1a1a1a" : "transparent",
              color: i === active ? "#f0f0f0" : "#666",
              borderColor: i === active ? "#242424" : "transparent",
            }}>{s.label}</button>
          ))}
        </div>
        <button onClick={handleCopy} style={{
          padding: "4px 10px", borderRadius: 5, fontSize: 11, fontWeight: 600, fontFamily: "var(--mono)",
          cursor: "pointer", border: "1px solid #242424", background: "#111", color: copied ? "#10b981" : "#888",
          transition: "color .15s",
        }}>{copied ? "Copied!" : "Copy"}</button>
      </div>
      <pre style={{ margin: 0, padding: "18px 20px", background: "#0f0f0f", fontSize: 13, lineHeight: 1.7, fontFamily: "var(--mono)", color: "#cdd6f4", overflowX: "auto" }}>
        {snippets[active].code}
      </pre>
    </div>
  );
}

// ── JSON response block ──
export function ResponseBlock({ title, code }: { title: string; code: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = () => { navigator.clipboard.writeText(code); setCopied(true); setTimeout(() => setCopied(false), 1500); };

  return (
    <div style={{ borderRadius: 10, overflow: "hidden", border: "1px solid #1a1a1a", marginBottom: 20 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", background: "#0a0a0a", borderBottom: "1px solid #1a1a1a", padding: "8px 14px" }}>
        <span style={{ fontSize: 12, fontWeight: 600, fontFamily: "var(--mono)", color: "#888" }}>{title}</span>
        <button onClick={handleCopy} style={{ padding: "4px 10px", borderRadius: 5, fontSize: 11, fontWeight: 600, fontFamily: "var(--mono)", cursor: "pointer", border: "1px solid #242424", background: "#111", color: copied ? "#10b981" : "#888" }}>{copied ? "Copied!" : "Copy"}</button>
      </div>
      <pre style={{ margin: 0, padding: "18px 20px", background: "#0f0f0f", fontSize: 13, lineHeight: 1.7, fontFamily: "var(--mono)", color: "#cdd6f4", overflowX: "auto" }}>{code}</pre>
    </div>
  );
}

// ── Info box ──
export function InfoBox({ children }: { children: React.ReactNode }) {
  return (
    <div style={{ background: "#0ea5e908", border: "1px solid #0ea5e920", borderRadius: 8, padding: "14px 18px", margin: "16px 0", fontSize: 13.5, lineHeight: 1.6, color: "#aaa" }}>
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
          display: "flex", alignItems: "center", gap: 10, padding: "14px 16px", background: "#0a0a0a", border: "1px solid #1a1a1a",
          borderRadius: 10, textDecoration: "none", color: "#ccc", fontSize: 13, transition: "all .15s",
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
    <div style={{ border: "1px solid #1a1a1a", borderRadius: 10, overflow: "hidden" }}>
      <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13.5 }}>
        <thead>
          <tr style={{ background: "#0a0a0a" }}>
            <th style={{ textAlign: "left", padding: "10px 14px", color: "#666", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid #1a1a1a" }}>Code</th>
            <th style={{ textAlign: "left", padding: "10px 14px", color: "#666", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid #1a1a1a" }}>HTTP</th>
            <th style={{ textAlign: "left", padding: "10px 14px", color: "#666", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid #1a1a1a" }}>Description</th>
          </tr>
        </thead>
        <tbody>
          {errors.map((e, i) => (
            <tr key={e.code} style={{ borderBottom: i < errors.length - 1 ? "1px solid #111" : undefined }}>
              <td style={{ padding: "10px 14px", fontFamily: "var(--mono)", color: "#ef4444", fontSize: 12, fontWeight: 500 }}>{e.code}</td>
              <td style={{ padding: "10px 14px", fontFamily: "var(--mono)", color: "#f59e0b", fontSize: 12 }}>{e.http}</td>
              <td style={{ padding: "10px 14px", color: "#999", lineHeight: 1.5 }}>{e.description}</td>
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
        <span style={{ fontSize: 13, fontWeight: 700, fontFamily: "var(--mono)", color: "#10b981" }}>{version}</span>
        <span style={{ fontSize: 12, color: "#555" }}>({date})</span>
      </div>
      <ul style={{ margin: 0, paddingLeft: 16 }}>
        {items.map((item, i) => <li key={i} style={{ fontSize: 13, color: "#999", lineHeight: 1.6, marginBottom: 2 }}>{item}</li>)}
      </ul>
    </div>
  );
}
