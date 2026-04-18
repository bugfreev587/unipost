"use client";

import { useAuth } from "@clerk/nextjs";
import { useCallback, useEffect, useMemo, useState } from "react";

import { listAdminBilling, type AdminBillingRow } from "@/lib/api";

import { AdminShell, StatCard, fmtCents, fmtNumber, fmtRelative } from "../_components/admin-ui";

const DAY_OPTIONS = [30, 90, 180] as const;

type PlanBreakdown = {
  plan_id: string;
  plan_name: string;
  workspaces: number;
  active_workspaces: number;
  mrr_cents: number;
  at_risk_cents: number;
};

export default function AdminMRRPage() {
  const { getToken } = useAuth();
  const [rows, setRows] = useState<AdminBillingRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [days, setDays] = useState<(typeof DAY_OPTIONS)[number]>(180);

  const loadBilling = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const res = await listAdminBilling(token, { days, limit: 200 });
      setRows(res.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, [days, getToken]);

  useEffect(() => {
    loadBilling();
  }, [loadBilling]);

  const activePaidRows = useMemo(
    () => rows.filter((row) => row.status === "active" && row.price_cents > 0),
    [rows],
  );
  const paidRows = useMemo(() => rows.filter((row) => row.price_cents > 0), [rows]);
  const atRiskRows = useMemo(
    () => rows.filter((row) => row.price_cents > 0 && (row.status === "past_due" || row.cancel_at_period_end)),
    [rows],
  );
  const totalMrrCents = useMemo(
    () => activePaidRows.reduce((sum, row) => sum + row.price_cents, 0),
    [activePaidRows],
  );
  const atRiskMrrCents = useMemo(
    () => atRiskRows.reduce((sum, row) => sum + row.price_cents, 0),
    [atRiskRows],
  );
  const freeWorkspaceCount = useMemo(
    () => rows.filter((row) => row.price_cents === 0).length,
    [rows],
  );
  const paidWorkspaceCount = activePaidRows.length;

  const planBreakdown = useMemo<PlanBreakdown[]>(() => {
    const map = new Map<string, PlanBreakdown>();
    rows.forEach((row) => {
      const existing = map.get(row.plan_id) ?? {
        plan_id: row.plan_id,
        plan_name: row.plan_name,
        workspaces: 0,
        active_workspaces: 0,
        mrr_cents: 0,
        at_risk_cents: 0,
      };
      existing.workspaces += 1;
      if (row.status === "active") {
        existing.active_workspaces += 1;
      }
      if (row.status === "active" && row.price_cents > 0) {
        existing.mrr_cents += row.price_cents;
      }
      if (row.price_cents > 0 && (row.status === "past_due" || row.cancel_at_period_end)) {
        existing.at_risk_cents += row.price_cents;
      }
      map.set(row.plan_id, existing);
    });
    return [...map.values()].sort((a, b) => b.mrr_cents - a.mrr_cents || b.workspaces - a.workspaces);
  }, [rows]);

  const mostRecentChange = useMemo(() => {
    if (rows.length === 0) return null;
    return rows.reduce<string | null>((latest, row) => {
      if (!latest) return row.updated_at;
      return new Date(row.updated_at).getTime() > new Date(latest).getTime() ? row.updated_at : latest;
    }, null);
  }, [rows]);

  return (
    <AdminShell title="MRR" loading={loading} onRefresh={loadBilling}>
      {error && (
        <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
          {error}
        </div>
      )}

      <div className="ad-section-header">
        <div className="ad-section-title">Revenue structure</div>
        <div className="ad-section-meta">Active and at-risk recurring revenue across the last {days} days</div>
      </div>

      <div className="ad-stat-grid">
        <StatCard label="Active MRR" value={fmtCents(totalMrrCents)} valueColor="accent" sub={`${fmtNumber(paidWorkspaceCount)} paid active workspaces`} />
        <StatCard label="At-Risk MRR" value={fmtCents(atRiskMrrCents)} sub={`${fmtNumber(atRiskRows.length)} subscriptions`} subColor={atRiskMrrCents > 0 ? "down" : undefined} />
        <StatCard label="Free vs Paid" value={`${fmtNumber(freeWorkspaceCount)} / ${fmtNumber(paidWorkspaceCount)}`} sub="free vs active paid workspaces" />
        <StatCard label="Latest Billing Change" value={mostRecentChange ? fmtRelative(mostRecentChange) : "—"} sub={mostRecentChange ? new Date(mostRecentChange).toLocaleDateString("en-US") : "—"} />
      </div>

      <div className="ad-filter-bar">
        <select value={days} onChange={(e) => setDays(Number(e.target.value) as typeof days)}>
          {DAY_OPTIONS.map((value) => (
            <option key={value} value={value}>
              Last {value} days
            </option>
          ))}
        </select>
      </div>

      <div className="ad-section-header" style={{ marginTop: 8 }}>
        <div className="ad-section-title">Plan breakdown</div>
        <div className="ad-section-meta">Revenue concentration by plan</div>
      </div>

      <div className="ad-tbl-wrap ad-tbl-static">
        <table>
          <thead>
            <tr>
              <th>Plan</th>
              <th>Workspaces</th>
              <th>Active</th>
              <th>MRR</th>
              <th>At Risk</th>
              <th>Share</th>
            </tr>
          </thead>
          <tbody>
            {loading && planBreakdown.length === 0 ? (
              <tr><td colSpan={6} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>Loading…</td></tr>
            ) : planBreakdown.length === 0 ? (
              <tr><td colSpan={6} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>No billing data yet.</td></tr>
            ) : (
              planBreakdown.map((plan) => {
                const share = totalMrrCents > 0 ? (plan.mrr_cents / totalMrrCents) * 100 : 0;
                return (
                  <tr key={plan.plan_id}>
                    <td>
                      <div style={{ fontWeight: 500 }}>{plan.plan_name}</div>
                      <div className="ad-mono">{plan.plan_id}</div>
                    </td>
                    <td>{fmtNumber(plan.workspaces)}</td>
                    <td>{fmtNumber(plan.active_workspaces)}</td>
                    <td>{fmtCents(plan.mrr_cents)}</td>
                    <td style={{ color: plan.at_risk_cents > 0 ? "var(--warning)" : "var(--dmuted)" }}>
                      {fmtCents(plan.at_risk_cents)}
                    </td>
                    <td>
                      <div style={{ fontSize: 11.5 }}>{share.toFixed(0)}%</div>
                      <div className="ad-usage-bar" style={{ width: 88, marginTop: 5 }}>
                        <div className="ad-usage-fill ad-uf-g" style={{ width: `${share}%` }} />
                      </div>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      <div className="ad-section-header" style={{ marginTop: 18 }}>
        <div className="ad-section-title">Subscriptions at risk</div>
        <div className="ad-section-meta">Past due or set to cancel at period end</div>
      </div>

      <div className="ad-tbl-wrap ad-tbl-static">
        <table>
          <thead>
            <tr>
              <th>Workspace</th>
              <th>User</th>
              <th>Plan</th>
              <th>Status</th>
              <th>Revenue</th>
              <th>Period End</th>
            </tr>
          </thead>
          <tbody>
            {atRiskRows.length === 0 ? (
              <tr><td colSpan={6} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>No at-risk subscriptions in the current window.</td></tr>
            ) : (
              atRiskRows.map((row) => (
                <tr key={row.workspace_id}>
                  <td>
                    <div style={{ fontWeight: 500 }}>{row.workspace_name}</div>
                    <div className="ad-mono">{row.workspace_id.slice(0, 16)}</div>
                  </td>
                  <td>{row.user_email}</td>
                  <td>{row.plan_name}</td>
                  <td>
                    <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
                      <span
                        className="ad-badge"
                        style={row.status === "past_due"
                          ? { background: "var(--danger-soft)", color: "var(--danger)", border: "1px solid color-mix(in srgb, var(--danger) 20%, transparent)" }
                          : { background: "var(--warning-soft)", color: "var(--warning)", border: "1px solid color-mix(in srgb, var(--warning) 20%, transparent)" }}
                      >
                        {row.status}
                      </span>
                      {row.cancel_at_period_end ? <span className="ad-badge ad-b-gray">cancel at end</span> : null}
                    </div>
                  </td>
                  <td>{fmtCents(row.price_cents)}</td>
                  <td>{row.current_period_end ? fmtRelative(row.current_period_end) : "—"}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </AdminShell>
  );
}
