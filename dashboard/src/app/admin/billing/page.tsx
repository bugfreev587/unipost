"use client";

import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { useCallback, useEffect, useMemo, useState } from "react";

import { listAdminBilling, type AdminBillingListParams, type AdminBillingRow } from "@/lib/api";

import { AdminShell, StatCard, fmtCents, fmtNumber, fmtRelative } from "../_components/admin-ui";

const STATUS_OPTIONS = ["all", "active", "past_due", "canceled", "trialing"] as const;
const PLAN_OPTIONS = ["all", "free", "pro", "business"] as const;
const DAY_OPTIONS = [30, 90, 180] as const;

export default function AdminBillingPage() {
  const { getToken } = useAuth();
  const [rows, setRows] = useState<AdminBillingRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState<(typeof STATUS_OPTIONS)[number]>("all");
  const [plan, setPlan] = useState<(typeof PLAN_OPTIONS)[number]>("all");
  const [days, setDays] = useState<(typeof DAY_OPTIONS)[number]>(90);
  const limit = 100;

  const loadBilling = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const params: AdminBillingListParams = {
        search: search || undefined,
        status: status !== "all" ? status : undefined,
        plan: plan !== "all" ? plan : undefined,
        days,
        limit,
      };
      const res = await listAdminBilling(token, params);
      setRows(res.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, [days, getToken, plan, search, status]);

  useEffect(() => {
    loadBilling();
  }, [loadBilling]);

  useEffect(() => {
    const timer = setTimeout(() => setSearch(searchInput), 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  const activeCount = useMemo(() => rows.filter((row) => row.status === "active").length, [rows]);
  const paidCount = useMemo(() => rows.filter((row) => row.price_cents > 0).length, [rows]);
  const pastDueCount = useMemo(() => rows.filter((row) => row.status === "past_due").length, [rows]);
  const cancelAtEndCount = useMemo(() => rows.filter((row) => row.cancel_at_period_end).length, [rows]);
  const totalMrrCents = useMemo(() => rows.filter((row) => row.status === "active").reduce((sum, row) => sum + row.price_cents, 0), [rows]);

  return (
    <AdminShell title="Billing" loading={loading} onRefresh={loadBilling}>
      {error && (
        <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
          {error}
        </div>
      )}

      <div className="ad-section-header">
        <div className="ad-section-title">Subscriptions</div>
        <div className="ad-section-meta">Workspace billing state across the last {days} days</div>
      </div>

      <div className="ad-stat-grid">
        <StatCard label="Active Subs" value={fmtNumber(activeCount)} sub={`${fmtNumber(rows.length)} total rows`} />
        <StatCard label="Paid Workspaces" value={fmtNumber(paidCount)} sub={rows.length > 0 ? `${((paidCount / rows.length) * 100).toFixed(0)}% of current set` : "—"} />
        <StatCard label="MRR" value={fmtCents(totalMrrCents)} valueColor="accent" sub="active subscriptions only" />
        <StatCard label="At Risk" value={fmtNumber(pastDueCount + cancelAtEndCount)} sub={`${fmtNumber(pastDueCount)} past_due · ${fmtNumber(cancelAtEndCount)} cancel at period end`} subColor={pastDueCount > 0 ? "down" : undefined} />
      </div>

      <div className="ad-filter-bar">
        <input
          className="ad-search"
          placeholder="Search by user, workspace, or plan..."
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          style={{ width: 320 }}
        />
        <select value={status} onChange={(e) => setStatus(e.target.value as typeof status)}>
          {STATUS_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All Statuses" : `Status: ${value}`}
            </option>
          ))}
        </select>
        <select value={plan} onChange={(e) => setPlan(e.target.value as typeof plan)}>
          {PLAN_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All Plans" : `Plan: ${value}`}
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

      <div className="ad-tbl-wrap ad-tbl-static">
        <table>
          <thead>
            <tr>
              <th>Workspace</th>
              <th>User</th>
              <th>Plan</th>
              <th>Status</th>
              <th>Usage</th>
              <th>Renewal</th>
              <th>Stripe</th>
              <th>Updated</th>
            </tr>
          </thead>
          <tbody>
            {loading && rows.length === 0 ? (
              <tr><td colSpan={8} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>Loading…</td></tr>
            ) : rows.length === 0 ? (
              <tr><td colSpan={8} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>No billing rows matched the current filters.</td></tr>
            ) : (
              rows.map((row) => {
                const usagePct = row.post_limit > 0 ? Math.min(100, (row.posts_used / row.post_limit) * 100) : 0;
                const usageClass = usagePct >= 90 ? "ad-uf-r" : usagePct >= 70 ? "ad-uf-a" : "ad-uf-g";
                return (
                  <tr key={row.workspace_id}>
                    <td>
                      <div style={{ fontWeight: 500 }}>{row.workspace_name}</div>
                      <div className="ad-mono">{row.workspace_id.slice(0, 16)}</div>
                    </td>
                    <td>
                      <Link href={`/admin/users?user=${row.user_id}`} className="ad-link">
                        {row.user_email}
                      </Link>
                    </td>
                    <td>
                      <div style={{ fontWeight: 500 }}>{row.plan_name}</div>
                      <div className="ad-mono">{row.plan_id} · {fmtCents(row.price_cents)}/mo</div>
                    </td>
                    <td>
                      <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
                        <span
                          className="ad-badge"
                          style={row.status === "active"
                            ? { background: "var(--success-soft)", color: "var(--success)", border: "1px solid color-mix(in srgb, var(--success) 20%, transparent)" }
                            : row.status === "past_due"
                              ? { background: "var(--danger-soft)", color: "var(--danger)", border: "1px solid color-mix(in srgb, var(--danger) 20%, transparent)" }
                              : { background: "var(--surface2)", color: "var(--dmuted)", border: "1px solid var(--dborder2)" }}
                        >
                          {row.status}
                        </span>
                        {row.cancel_at_period_end ? <span className="ad-badge ad-b-gray">cancel at end</span> : null}
                        {!row.trial_used ? <span className="ad-badge ad-b-blue">trial eligible</span> : null}
                      </div>
                    </td>
                    <td>
                      <div style={{ fontSize: 11.5 }}>{fmtNumber(row.posts_used)} / {fmtNumber(row.post_limit)}</div>
                      <div className="ad-usage-bar" style={{ width: 88, marginTop: 5 }}>
                        <div className={`ad-usage-fill ${usageClass}`} style={{ width: `${usagePct}%` }} />
                      </div>
                    </td>
                    <td>
                      {row.current_period_end ? (
                        <div>
                          <div>{fmtRelative(row.current_period_end)}</div>
                          <div className="ad-mono" style={{ marginTop: 3 }}>
                            {new Date(row.current_period_end).toLocaleDateString("en-US")}
                          </div>
                        </div>
                      ) : (
                        <span style={{ color: "var(--dmuted2)", fontSize: 11 }}>—</span>
                      )}
                    </td>
                    <td>
                      <div className="ad-mono">{row.stripe_customer_id?.slice(0, 12) || "—"}</div>
                      <div className="ad-mono" style={{ marginTop: 3 }}>{row.stripe_subscription_id?.slice(0, 12) || "—"}</div>
                    </td>
                    <td style={{ whiteSpace: "nowrap" }}>{fmtRelative(row.updated_at)}</td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </AdminShell>
  );
}
