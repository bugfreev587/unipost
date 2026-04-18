"use client";

import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { useCallback, useEffect, useMemo, useState } from "react";

import {
  listAdminPostFailures,
  type AdminPostFailureListParams,
  type AdminUserPostFailure,
} from "@/lib/api";

import { AdminShell, StatCard, fmtNumber, fmtRelative } from "../_components/admin-ui";

const PLATFORM_OPTIONS = ["all", "twitter", "linkedin", "instagram", "threads", "tiktok", "youtube", "bluesky"] as const;
const SOURCE_OPTIONS = ["all", "dashboard", "api", "mcp"] as const;
const DAY_OPTIONS = [7, 30, 90] as const;

export default function AdminErrorsPage() {
  const { getToken } = useAuth();
  const [failures, setFailures] = useState<AdminUserPostFailure[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [platform, setPlatform] = useState<(typeof PLATFORM_OPTIONS)[number]>("all");
  const [source, setSource] = useState<(typeof SOURCE_OPTIONS)[number]>("all");
  const [days, setDays] = useState<(typeof DAY_OPTIONS)[number]>(30);
  const limit = 100;

  const loadFailures = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const params: AdminPostFailureListParams = {
        search: search || undefined,
        platform: platform !== "all" ? platform : undefined,
        source: source !== "all" ? source : undefined,
        days,
        limit,
      };
      const res = await listAdminPostFailures(token, params);
      setFailures(res.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, [days, getToken, platform, search, source]);

  useEffect(() => {
    loadFailures();
  }, [loadFailures]);

  useEffect(() => {
    const timer = setTimeout(() => setSearch(searchInput), 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  const uniqueUsers = useMemo(() => new Set(failures.map((item) => item.user_id)).size, [failures]);
  const uniqueWorkspaces = useMemo(() => new Set(failures.map((item) => item.workspace_id)).size, [failures]);
  const byPlatform = useMemo(() => {
    const counts = new Map<string, number>();
    failures.forEach((item) => {
      const key = item.platform || "parent";
      counts.set(key, (counts.get(key) || 0) + 1);
    });
    return [...counts.entries()].sort((a, b) => b[1] - a[1])[0] ?? null;
  }, [failures]);
  const bySource = useMemo(() => {
    const counts = new Map<string, number>();
    failures.forEach((item) => counts.set(item.source, (counts.get(item.source) || 0) + 1));
    return [...counts.entries()].sort((a, b) => b[1] - a[1])[0] ?? null;
  }, [failures]);

  return (
    <AdminShell title="Errors" loading={loading} onRefresh={loadFailures}>
      {error && (
        <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
          {error}
        </div>
      )}

      <div className="ad-section-header">
        <div className="ad-section-title">Publishing failures</div>
        <div className="ad-section-meta">Cross-tenant errors from the last {days} days</div>
      </div>

      <div className="ad-stat-grid">
        <StatCard label="Failures" value={fmtNumber(failures.length)} sub="current filtered set" />
        <StatCard label="Affected Users" value={fmtNumber(uniqueUsers)} sub="distinct customers" />
        <StatCard label="Affected Workspaces" value={fmtNumber(uniqueWorkspaces)} sub="distinct workspaces" />
        <StatCard
          label="Top Bucket"
          value={byPlatform ? byPlatform[0] : "—"}
          sub={byPlatform ? `${fmtNumber(byPlatform[1])} failures` : bySource ? `${bySource[0]} source` : "—"}
          valueColor="accent"
        />
      </div>

      <div className="ad-filter-bar">
        <input
          className="ad-search"
          placeholder="Search by user, workspace, caption, or error..."
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          style={{ width: 320 }}
        />
        <select value={platform} onChange={(e) => setPlatform(e.target.value as typeof platform)}>
          {PLATFORM_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All Platforms" : `Platform: ${value}`}
            </option>
          ))}
        </select>
        <select value={source} onChange={(e) => setSource(e.target.value as typeof source)}>
          {SOURCE_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All Sources" : `Source: ${value}`}
            </option>
          ))}
        </select>
        <select value={days} onChange={(e) => setDays(Number(e.target.value) as typeof days)}>
          {DAY_OPTIONS.map((value) => (
            <option key={value} value={value}>
              Last {value} days
            </option>
          ))}
        </select>
      </div>

      <div className="ad-stack">
        {loading && failures.length === 0 ? (
          <div className="ad-failure-card" style={{ color: "var(--dmuted)", textAlign: "center" }}>Loading…</div>
        ) : failures.length === 0 ? (
          <div className="ad-failure-card" style={{ color: "var(--dmuted)", textAlign: "center" }}>
            No failures matched the current filters.
          </div>
        ) : (
          failures.map((failure, idx) => {
            const message = failure.error_message || failure.error_summary || "No error message recorded.";
            return (
              <article
                key={`${failure.post_id}-${failure.platform || "parent"}-${idx}`}
                className="ad-failure-card"
              >
                <div className="ad-failure-head">
                  <div>
                    <div className="ad-failure-meta">
                      <span className="ad-badge ad-b-gray">{failure.platform || failure.post_status}</span>
                      <span className="ad-badge ad-b-blue">{failure.source}</span>
                      {failure.account_name ? <span style={{ fontSize: 11, color: "var(--dmuted)" }}>@{failure.account_name}</span> : null}
                    </div>
                    <div className="ad-failure-title" style={{ marginTop: 6 }}>
                      <Link href={`/admin/users?user=${failure.user_id}`} className="ad-link">
                        {failure.user_email}
                      </Link>
                      <span style={{ color: "var(--dmuted2)" }}> · </span>
                      <span>{failure.workspace_name}</span>
                    </div>
                  </div>
                  <div style={{ textAlign: "right" }}>
                    <div style={{ fontSize: 11.5, color: "var(--dmuted)" }}>{fmtRelative(failure.created_at)}</div>
                    <div className="ad-mono" style={{ marginTop: 4 }}>{failure.post_id.slice(0, 16)}</div>
                  </div>
                </div>

                {failure.caption ? <div className="ad-failure-caption">{failure.caption}</div> : null}
                <div className="ad-failure-message">{message}</div>

                <div style={{ display: "flex", justifyContent: "space-between", gap: 12, marginTop: 8, alignItems: "center", flexWrap: "wrap" }}>
                  <div className="ad-mono">
                    workspace {failure.workspace_id.slice(0, 12)} · user {failure.user_id.slice(0, 12)}
                  </div>
                  <Link href={`/admin/users?user=${failure.user_id}`} className="ad-link" style={{ fontSize: 12 }}>
                    Inspect user →
                  </Link>
                </div>
              </article>
            );
          })
        )}
      </div>
    </AdminShell>
  );
}
