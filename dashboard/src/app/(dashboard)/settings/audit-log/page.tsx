"use client";

import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { listAuditLog, type AuditLogEntry } from "@/lib/api";

// /settings/audit-log — RBAC Phase 6 dashboard view.
//
// Reads from GET /v1/audit-log with optional filters (action,
// category, days). Shows the most recent N events newest-first with
// a compact category-colored badge per row. Click a row to expand
// the before / after JSON snapshots.

const CATEGORY_OPTIONS = ["all", "membership", "billing", "config", "auth"] as const;
const DAY_OPTIONS = [7, 30, 90, 180] as const;

export default function AuditLogPage() {
  const { getToken } = useAuth();
  const [rows, setRows] = useState<AuditLogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [category, setCategory] = useState<(typeof CATEGORY_OPTIONS)[number]>("all");
  const [days, setDays] = useState<(typeof DAY_OPTIONS)[number]>(30);
  const [expanded, setExpanded] = useState<number | null>(null);

  const load = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const res = await listAuditLog(token, {
        category: category !== "all" ? category : undefined,
        days,
        limit: 200,
      });
      setRows(res.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load audit log");
    } finally {
      setLoading(false);
    }
  }, [getToken, category, days]);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16, maxWidth: 980 }}>
      <p style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6, margin: 0 }}>
        Records every membership change, plan change, API key creation/revocation, and
        platform credential change in this workspace. Useful for compliance reviews and for
        tracing back when a setting changed.
      </p>

      <div style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
        <select
          value={category}
          onChange={(e) => setCategory(e.target.value as (typeof CATEGORY_OPTIONS)[number])}
          style={selectStyle}
        >
          {CATEGORY_OPTIONS.map((c) => (
            <option key={c} value={c}>
              Category: {c}
            </option>
          ))}
        </select>
        <select
          value={days}
          onChange={(e) => setDays(Number(e.target.value) as (typeof DAY_OPTIONS)[number])}
          style={selectStyle}
        >
          {DAY_OPTIONS.map((d) => (
            <option key={d} value={d}>
              Last {d} days
            </option>
          ))}
        </select>
        <button onClick={load} className="dbtn dbtn-ghost" style={{ fontSize: 12, padding: "6px 12px" }}>
          Refresh
        </button>
        <span style={{ fontSize: 12, color: "var(--dmuted)", marginLeft: "auto" }}>
          {loading ? "Loading…" : `${rows.length} event${rows.length === 1 ? "" : "s"}`}
        </span>
      </div>

      {error && (
        <div
          style={{
            background: "color-mix(in srgb, var(--danger) 8%, transparent)",
            border: "1px solid color-mix(in srgb, var(--danger) 25%, transparent)",
            borderRadius: 8,
            padding: "10px 14px",
            fontSize: 13,
            color: "var(--danger)",
          }}
        >
          {error}
        </div>
      )}

      {rows.length === 0 && !loading && !error ? (
        <div
          style={{
            padding: "32px 16px",
            border: "1px dashed var(--dborder)",
            borderRadius: 8,
            color: "var(--dmuted)",
            fontSize: 13,
            textAlign: "center",
          }}
        >
          No audit events in this window. Events are recorded as members get invited,
          plans change, API keys rotate, etc.
        </div>
      ) : (
        <div style={{ border: "1px solid var(--dborder)", borderRadius: 8, overflow: "hidden" }}>
          {rows.map((row) => (
            <Row
              key={row.id}
              row={row}
              expanded={expanded === row.id}
              onToggle={() => setExpanded(expanded === row.id ? null : row.id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function Row({
  row,
  expanded,
  onToggle,
}: {
  row: AuditLogEntry;
  expanded: boolean;
  onToggle: () => void;
}) {
  return (
    <div style={{ borderTop: "1px solid var(--dborder)" }}>
      <button
        onClick={onToggle}
        style={{
          width: "100%",
          padding: "10px 14px",
          display: "grid",
          gridTemplateColumns: "auto 1fr auto auto",
          alignItems: "center",
          gap: 14,
          background: "none",
          border: "none",
          cursor: "pointer",
          textAlign: "left",
          color: "var(--dtext)",
        }}
      >
        <CategoryBadge category={row.category} />
        <div>
          <div style={{ fontFamily: "var(--font-mono, ui-monospace)", fontSize: 12.5, fontWeight: 600 }}>
            {row.action}
          </div>
          <div style={{ fontSize: 11.5, color: "var(--dmuted)", marginTop: 2 }}>
            {row.resource_type}
            {row.resource_id ? ` · ${row.resource_id.slice(0, 16)}` : ""}
            {row.actor_user_id ? ` · by ${row.actor_user_id.slice(0, 16)}` : ""}
            {row.actor_api_key_id ? ` · via api key` : ""}
          </div>
        </div>
        <div style={{ fontSize: 11.5, color: "var(--dmuted)", whiteSpace: "nowrap" }}>
          {new Date(row.created_at).toLocaleString()}
        </div>
        <div style={{ fontSize: 11, color: "var(--dmuted)" }}>{expanded ? "▾" : "▸"}</div>
      </button>
      {expanded && (
        <div style={{ padding: "0 14px 14px 60px", background: "var(--surface2, transparent)" }}>
          {row.before != null && (
            <Diff label="Before" value={row.before} />
          )}
          {row.after != null && (
            <Diff label="After" value={row.after} />
          )}
          {row.metadata != null && (
            <Diff label="Metadata" value={row.metadata} />
          )}
        </div>
      )}
    </div>
  );
}

function Diff({ label, value }: { label: string; value: unknown }) {
  return (
    <div style={{ marginTop: 8 }}>
      <div style={{ fontSize: 10.5, fontWeight: 700, color: "var(--dmuted)", textTransform: "uppercase", letterSpacing: "0.04em" }}>
        {label}
      </div>
      <pre
        style={{
          fontSize: 11.5,
          margin: "4px 0 0",
          padding: 8,
          background: "var(--dbg)",
          border: "1px solid var(--dborder)",
          borderRadius: 4,
          overflow: "auto",
          maxHeight: 200,
          fontFamily: "var(--font-mono, ui-monospace)",
        }}
      >
        {JSON.stringify(value, null, 2)}
      </pre>
    </div>
  );
}

function CategoryBadge({ category }: { category: string }) {
  const colorMap: Record<string, { bg: string; fg: string }> = {
    membership: { bg: "color-mix(in srgb, var(--daccent) 12%, transparent)", fg: "var(--daccent)" },
    billing: { bg: "color-mix(in srgb, var(--warning) 14%, transparent)", fg: "var(--warning)" },
    config: { bg: "color-mix(in srgb, var(--dmuted) 18%, transparent)", fg: "var(--dmuted)" },
    auth: { bg: "color-mix(in srgb, var(--danger) 12%, transparent)", fg: "var(--danger)" },
    publishing: { bg: "color-mix(in srgb, var(--daccent) 12%, transparent)", fg: "var(--daccent)" },
  };
  const c = colorMap[category] ?? { bg: "var(--surface2)", fg: "var(--dmuted)" };
  return (
    <span
      style={{
        fontSize: 10.5,
        fontFamily: "var(--font-mono, ui-monospace)",
        padding: "3px 8px",
        background: c.bg,
        color: c.fg,
        borderRadius: 4,
        whiteSpace: "nowrap",
        textTransform: "uppercase",
        letterSpacing: "0.04em",
        fontWeight: 600,
      }}
    >
      {category}
    </span>
  );
}

const selectStyle: React.CSSProperties = {
  padding: "6px 10px",
  fontSize: 12,
  border: "1px solid var(--dborder)",
  borderRadius: 6,
  background: "var(--dbg)",
  color: "var(--dtext)",
};
