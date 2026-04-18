"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  getAdminLandingSources,
  getAdminStats,
  type AdminLandingSourceRow,
  type AdminLandingSourcesResponse,
  type AdminStats,
} from "@/lib/api";
import { useAuth } from "@clerk/nextjs";

import { AdminShell, StatCard, fmtCents, fmtDate, fmtNumber, fmtRelative } from "./_components/admin-ui";

function LandingSourceRowView({ row }: { row: AdminLandingSourceRow }) {
  return (
    <tr>
      <td>
        <div style={{ fontWeight: 500 }}>{row.label}</div>
      </td>
      <td>
        <span className="ad-badge ad-b-gray">{row.source_code}</span>
      </td>
      <td>{fmtNumber(row.visits)}</td>
      <td>{fmtNumber(row.unique_visitors)}</td>
      <td style={{ color: "var(--dmuted)", fontSize: 11.5 }}>{fmtRelative(row.last_visit_at)}</td>
    </tr>
  );
}

export default function AdminDashboardPage() {
  const { getToken } = useAuth();
  const [stats, setStats] = useState<AdminStats | null>(null);
  const [landingSources, setLandingSources] = useState<AdminLandingSourcesResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadAll = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const [statsRes, landingSourcesRes] = await Promise.all([
        getAdminStats(token),
        getAdminLandingSources(token),
      ]);
      setStats(statsRes.data);
      setLandingSources(landingSourcesRes.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, [getToken]);

  useEffect(() => {
    loadAll();
  }, [loadAll]);

  const conversionPct = useMemo(() => {
    if (!stats || stats.total_users === 0) return 0;
    return (stats.paid_users / stats.total_users) * 100;
  }, [stats]);

  const failedPct = useMemo(() => {
    if (!stats || stats.posts_this_month === 0) return 0;
    return (stats.posts_failed_this_month / stats.posts_this_month) * 100;
  }, [stats]);

  const signups7dDelta = useMemo(() => {
    if (!stats || stats.prev_signups_7d === 0) return null;
    return ((stats.new_signups_7d - stats.prev_signups_7d) / stats.prev_signups_7d) * 100;
  }, [stats]);

  const topLandingSource = useMemo(() => landingSources?.rows[0] ?? null, [landingSources]);
  const latestLandingVisit = useMemo(() => {
    if (!landingSources?.rows.length) return null;
    return landingSources.rows.reduce<string | null>((latest, row) => {
      if (!row.last_visit_at) return latest;
      if (!latest) return row.last_visit_at;
      return new Date(row.last_visit_at).getTime() > new Date(latest).getTime() ? row.last_visit_at : latest;
    }, null);
  }, [landingSources]);

  return (
    <AdminShell title="Dashboard" loading={loading} onRefresh={loadAll}>
      {error && (
        <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
          {error}
        </div>
      )}

      <div className="ad-section-header">
        <div className="ad-section-title">Growth</div>
        <div className="ad-section-meta">Cross-tenant overview</div>
      </div>
      <div className="ad-stat-grid">
        <StatCard
          label="Total Users"
          value={stats ? fmtNumber(stats.total_users) : "—"}
          sub={stats && stats.new_users_this_month > 0 ? `↑ ${stats.new_users_this_month} this month` : "—"}
          subColor="up"
        />
        <StatCard
          label="Paid Users"
          value={stats ? fmtNumber(stats.paid_users) : "—"}
          sub={stats ? `${conversionPct.toFixed(1)}% conversion` : "—"}
        />
        <StatCard
          label="Active Workspaces"
          value={stats ? fmtNumber(stats.active_workspaces) : "—"}
          sub={stats && stats.total_users > 0 ? `avg ${(stats.active_workspaces / stats.total_users).toFixed(1)} / user` : "—"}
        />
        <StatCard
          label="New Signups (7d)"
          value={stats ? fmtNumber(stats.new_signups_7d) : "—"}
          sub={signups7dDelta != null ? `${signups7dDelta >= 0 ? "↑" : "↓"} ${Math.abs(signups7dDelta).toFixed(0)}% vs prev week` : "—"}
          subColor={signups7dDelta != null && signups7dDelta >= 0 ? "up" : "down"}
        />
      </div>

      <div className="ad-section-header">
        <div className="ad-section-title">Revenue</div>
        <div className="ad-section-meta">Current billing snapshot</div>
      </div>
      <div className="ad-stat-grid">
        <StatCard label="MRR" value={stats ? fmtCents(stats.mrr_cents) : "—"} valueColor="accent" sub="—" />
        <StatCard label="Churn (30d)" value={stats ? fmtNumber(stats.churn_30d) : "—"} sub="last 30 days" subColor={stats && stats.churn_30d > 0 ? "down" : undefined} />
        <StatCard label="Platform Connections" value={stats ? fmtNumber(stats.platform_connections) : "—"} sub="active social accounts" />
        <StatCard label="Posts This Month" value={stats ? fmtNumber(stats.posts_this_month) : "—"} sub={stats ? <>Failed rate: <span style={{ color: failedPct > 5 ? "var(--danger)" : "var(--warning)" }}>{failedPct.toFixed(1)}%</span></> : "—"} />
      </div>

      <div className="ad-section-header" style={{ marginTop: 8 }}>
        <div className="ad-section-title">Landing Sources</div>
        <div className="ad-section-meta">Last {landingSources?.range_days ?? 30} days</div>
      </div>

      <div className="ad-stat-grid" style={{ marginBottom: 14 }}>
        <StatCard label="Landing Visits" value={landingSources ? fmtNumber(landingSources.total_visits) : "—"} sub="Tracked on marketing page" />
        <StatCard label="Unique Visitors" value={landingSources ? fmtNumber(landingSources.unique_visitors) : "—"} sub={landingSources && landingSources.total_visits > 0 ? `${((landingSources.unique_visitors / landingSources.total_visits) * 100).toFixed(0)}% unique rate` : "—"} />
        <StatCard label="Top Source" value={topLandingSource?.label ?? "—"} sub={topLandingSource ? `${fmtNumber(topLandingSource.visits)} visits` : "—"} valueColor="accent" />
        <StatCard label="Most Recent" value={latestLandingVisit ? fmtRelative(latestLandingVisit) : "—"} sub={latestLandingVisit ? fmtDate(latestLandingVisit) : "—"} />
      </div>

      <div className="ad-tbl-wrap ad-tbl-static">
        <table>
          <thead>
            <tr>
              <th>Source</th>
              <th>Code</th>
              <th>Visits</th>
              <th>Unique Visitors</th>
              <th>Last Visit</th>
            </tr>
          </thead>
          <tbody>
            {landingSources && landingSources.rows.length > 0 ? (
              landingSources.rows.map((row) => <LandingSourceRowView key={row.source_code} row={row} />)
            ) : (
              <tr>
                <td colSpan={5} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>
                  {loading ? "Loading…" : "No landing source data yet"}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </AdminShell>
  );
}
