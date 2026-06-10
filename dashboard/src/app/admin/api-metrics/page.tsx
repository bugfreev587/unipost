"use client";

import { useAuth } from "@clerk/nextjs";
import { useCallback, useEffect, useState } from "react";

import {
  getAdminAPIMetricsOverall,
  getAdminAPIMetricsStatusCodes,
  getAdminAPIMetricsSummary,
  getAdminAPIMetricsTrend,
  getAdminAPIMetricsWorkspaces,
  type AdminAPIMetricsWorkspaceRow,
  type APIMetricsOverall,
  type APIMetricsQueryParams,
  type APIMetricsStatusCodeRow,
  type APIMetricsSummaryRow,
  type APIMetricsTrendRow,
} from "@/lib/api";

import { AdminShell, StatCard, fmtNumber } from "../_components/admin-ui";

const DAY_OPTIONS = [1, 7, 30, 90] as const;
const METHOD_OPTIONS = ["all", "GET", "POST", "PUT", "PATCH", "DELETE"] as const;
const STATUS_OPTIONS = ["all", "2xx", "3xx", "4xx", "5xx"] as const;
const SORT_OPTIONS = [
  "total_calls_desc",
  "p95_ms_desc",
  "p99_ms_desc",
  "server_errors_desc",
  "rate_limited_desc",
] as const;

export default function AdminAPIMetricsPage() {
  const { getToken } = useAuth();
  const [overall, setOverall] = useState<APIMetricsOverall | null>(null);
  const [summary, setSummary] = useState<APIMetricsSummaryRow[]>([]);
  const [trend, setTrend] = useState<APIMetricsTrendRow[]>([]);
  const [workspaces, setWorkspaces] = useState<AdminAPIMetricsWorkspaceRow[]>([]);
  const [statusCodes, setStatusCodes] = useState<APIMetricsStatusCodeRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [days, setDays] = useState<(typeof DAY_OPTIONS)[number]>(7);
  const [method, setMethod] = useState<(typeof METHOD_OPTIONS)[number]>("all");
  const [statusClass, setStatusClass] = useState<(typeof STATUS_OPTIONS)[number]>("all");
  const [sort, setSort] = useState<(typeof SORT_OPTIONS)[number]>("p95_ms_desc");
  const [workspaceID, setWorkspaceID] = useState("");

  const buildParams = useCallback((): APIMetricsQueryParams => {
    const to = new Date().toISOString();
    const from = new Date(Date.now() - days * 24 * 60 * 60 * 1000).toISOString();
    return {
      from,
      to,
      interval: days > 7 ? "day" : "hour",
      method: method === "all" ? undefined : method,
      status_class: statusClass === "all" ? undefined : statusClass,
      workspace_id: workspaceID.trim() || undefined,
      sort,
      limit: 100,
      min_calls: 1,
    };
  }, [days, method, sort, statusClass, workspaceID]);

  const loadMetrics = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const params = buildParams();
      const [overallRes, summaryRes, trendRes, workspacesRes, statusRes] = await Promise.all([
        getAdminAPIMetricsOverall(token, params),
        getAdminAPIMetricsSummary(token, params),
        getAdminAPIMetricsTrend(token, params),
        getAdminAPIMetricsWorkspaces(token, params),
        getAdminAPIMetricsStatusCodes(token, params),
      ]);
      setOverall(overallRes.data);
      setSummary(summaryRes.data || []);
      setTrend(trendRes.data || []);
      setWorkspaces(workspacesRes.data || []);
      setStatusCodes(statusRes.data || []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load API metrics");
    } finally {
      setLoading(false);
    }
  }, [buildParams, getToken]);

  useEffect(() => {
    loadMetrics();
  }, [loadMetrics]);

  const latestP95 = trend.length > 0 ? trend[trend.length - 1].p95_ms : 0;
  const maxCalls = Math.max(...trend.map((row) => row.total_calls), 1);

  return (
    <AdminShell title="API Metrics" loading={loading} onRefresh={loadMetrics}>
      {error && (
        <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
          {error}
        </div>
      )}

      <div className="ad-section-header">
        <div className="ad-section-title">Developer API health</div>
        <div className="ad-section-meta">API-key traffic across all workspaces for the last {days} day{days === 1 ? "" : "s"}</div>
      </div>

      <div className="ad-stat-grid">
        <StatCard label="Total Calls" value={fmtNumber(overall?.total_calls ?? 0)} sub="API-key requests" />
        <StatCard label="Reliability" value={`${(overall?.reliability_pct ?? 0).toFixed(1)}%`} sub={`${fmtNumber(overall?.server_error_count ?? 0)} server failures`} valueColor={(overall?.server_error_count ?? 0) > 0 ? undefined : "accent"} subColor={(overall?.server_error_count ?? 0) > 0 ? "down" : undefined} />
        <StatCard label="p95 Latency" value={`${overall?.p95_ms ?? 0}ms`} sub={`p99 ${overall?.p99_ms ?? 0}ms / latest ${latestP95}ms`} />
        <StatCard label="Rate Limited" value={fmtNumber(overall?.rate_limited_count ?? 0)} sub="HTTP 429 responses" subColor={(overall?.rate_limited_count ?? 0) > 0 ? "down" : undefined} />
      </div>

      <div className="ad-filter-bar">
        <select value={days} onChange={(e) => setDays(Number(e.target.value) as typeof days)}>
          {DAY_OPTIONS.map((value) => (
            <option key={value} value={value}>Last {value}d</option>
          ))}
        </select>
        <select value={method} onChange={(e) => setMethod(e.target.value as typeof method)}>
          {METHOD_OPTIONS.map((value) => (
            <option key={value} value={value}>{value === "all" ? "All methods" : value}</option>
          ))}
        </select>
        <select value={statusClass} onChange={(e) => setStatusClass(e.target.value as typeof statusClass)}>
          {STATUS_OPTIONS.map((value) => (
            <option key={value} value={value}>{value === "all" ? "All statuses" : value}</option>
          ))}
        </select>
        <select value={sort} onChange={(e) => setSort(e.target.value as typeof sort)}>
          {SORT_OPTIONS.map((value) => (
            <option key={value} value={value}>{sortLabel(value)}</option>
          ))}
        </select>
        <input
          className="ad-search"
          placeholder="workspace_id"
          value={workspaceID}
          onChange={(e) => setWorkspaceID(e.target.value)}
          style={{ width: 220 }}
        />
      </div>

      <div style={{ border: "1px solid var(--dborder)", borderRadius: 8, padding: 14, marginBottom: 18, background: "var(--surface1)" }}>
        <div className="ad-section-title" style={{ marginBottom: 10 }}>Traffic trend</div>
        {trend.length === 0 ? (
          <div style={{ color: "var(--dmuted)", fontSize: 13, padding: 18, textAlign: "center" }}>No API traffic in this window.</div>
        ) : (
          <div style={{ display: "flex", alignItems: "flex-end", gap: 3, height: 86 }}>
            {trend.map((row, idx) => {
              const height = Math.max(3, (row.total_calls / maxCalls) * 100);
              const errorRate = row.total_calls > 0 ? row.error_count / row.total_calls : 0;
              return (
                <div
                  key={`${row.bucket}-${idx}`}
                  title={`${new Date(row.bucket).toLocaleString()}: ${row.total_calls} calls, p95 ${row.p95_ms}ms`}
                  style={{
                    flex: 1,
                    minWidth: 3,
                    height: `${height}%`,
                    borderRadius: "2px 2px 0 0",
                    background: errorRate > 0.1 ? "var(--warning)" : "var(--success)",
                    opacity: 0.84,
                  }}
                />
              );
            })}
          </div>
        )}
      </div>

      <div className="ad-section-header">
        <div className="ad-section-title">Endpoints</div>
        <div className="ad-section-meta">Sorted by {sortLabel(sort).toLowerCase()}</div>
      </div>
      <div className="ad-tbl-wrap ad-tbl-static">
        <table>
          <thead>
            <tr>
              <th>Endpoint</th>
              <th>Calls</th>
              <th>4xx</th>
              <th>429</th>
              <th>5xx</th>
              <th>Error %</th>
              <th>p95</th>
              <th>p99</th>
            </tr>
          </thead>
          <tbody>
            {loading && summary.length === 0 ? (
              <tr><td colSpan={8} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>Loading...</td></tr>
            ) : summary.length === 0 ? (
              <tr><td colSpan={8} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>No endpoints matched the current filters.</td></tr>
            ) : (
              summary.map((row) => (
                <tr key={`${row.method}-${row.path}`}>
                  <td>
                    <span className="ad-mono" style={{ color: methodColor(row.method), marginRight: 8 }}>{row.method}</span>
                    <span className="ad-mono">{row.path}</span>
                  </td>
                  <td>{fmtNumber(row.total_calls)}</td>
                  <td>{fmtNumber(row.client_error_count)}</td>
                  <td style={{ color: row.rate_limited_count > 0 ? "var(--warning)" : "var(--dmuted)" }}>{fmtNumber(row.rate_limited_count)}</td>
                  <td style={{ color: row.server_error_count > 0 ? "var(--danger)" : "var(--dmuted)" }}>{fmtNumber(row.server_error_count)}</td>
                  <td>{row.error_rate_pct.toFixed(1)}%</td>
                  <td>{row.p95_ms}ms</td>
                  <td>{row.p99_ms}ms</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className="ad-section-header" style={{ marginTop: 18 }}>
        <div className="ad-section-title">Workspace impact</div>
        <div className="ad-section-meta">Workspaces with the most API pressure or slow endpoints</div>
      </div>
      <div className="ad-tbl-wrap ad-tbl-static">
        <table>
          <thead>
            <tr>
              <th>Workspace</th>
              <th>Calls</th>
              <th>429</th>
              <th>Failure %</th>
              <th>p95</th>
              <th>p99</th>
              <th>Slowest endpoint</th>
            </tr>
          </thead>
          <tbody>
            {workspaces.length === 0 ? (
              <tr><td colSpan={7} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>No workspace metrics in this window.</td></tr>
            ) : (
              workspaces.map((row) => (
                <tr key={row.workspace_id}>
                  <td>
                    <div style={{ fontWeight: 500 }}>{row.workspace_name || "Unnamed workspace"}</div>
                    <div className="ad-mono">{row.workspace_id}</div>
                  </td>
                  <td>{fmtNumber(row.total_calls)}</td>
                  <td style={{ color: row.rate_limited_count > 0 ? "var(--warning)" : "var(--dmuted)" }}>{fmtNumber(row.rate_limited_count)}</td>
                  <td>{row.server_failure_rate_pct.toFixed(1)}%</td>
                  <td>{row.p95_ms}ms</td>
                  <td>{row.p99_ms}ms</td>
                  <td>
                    <div className="ad-mono">{row.slowest_endpoint || "-"}</div>
                    <div style={{ fontSize: 11, color: "var(--dmuted)" }}>p95 {row.slowest_endpoint_p95_ms}ms</div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className="ad-section-header" style={{ marginTop: 18 }}>
        <div className="ad-section-title">Status codes</div>
        <div className="ad-section-meta">Exact HTTP status distribution by endpoint</div>
      </div>
      <div className="ad-tbl-wrap ad-tbl-static">
        <table>
          <thead>
            <tr>
              <th>Status</th>
              <th>Endpoint</th>
              <th>Calls</th>
            </tr>
          </thead>
          <tbody>
            {statusCodes.length === 0 ? (
              <tr><td colSpan={3} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>No status-code data in this window.</td></tr>
            ) : (
              statusCodes.slice(0, 24).map((row) => (
                <tr key={`${row.status_code}-${row.method}-${row.path}`}>
                  <td className="ad-mono" style={{ color: statusColor(row.status_code) }}>{row.status_code}</td>
                  <td>
                    <span className="ad-mono" style={{ color: methodColor(row.method), marginRight: 8 }}>{row.method}</span>
                    <span className="ad-mono">{row.path}</span>
                  </td>
                  <td>{fmtNumber(row.total_calls)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </AdminShell>
  );
}

function sortLabel(sort: (typeof SORT_OPTIONS)[number]) {
  switch (sort) {
    case "p95_ms_desc": return "Highest p95";
    case "p99_ms_desc": return "Highest p99";
    case "server_errors_desc": return "Most 5xx";
    case "rate_limited_desc": return "Most 429";
    default: return "Most calls";
  }
}

function methodColor(method: string) {
  switch (method) {
    case "GET": return "var(--success)";
    case "POST": return "var(--info)";
    case "PUT":
    case "PATCH": return "var(--warning)";
    case "DELETE": return "var(--danger)";
    default: return "var(--dmuted)";
  }
}

function statusColor(status: number) {
  if (status >= 500) return "var(--danger)";
  if (status >= 400) return "var(--warning)";
  if (status >= 200 && status < 400) return "var(--success)";
  return "var(--dmuted)";
}
