"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { useWorkspaceId } from "@/lib/use-workspace-id";
import {
  getAPIMetricsSummary,
  getAPIMetricsTrend,
  getAPIMetricsOverall,
  type APIMetricsSummaryRow,
  type APIMetricsTrendRow,
  type APIMetricsOverall,
} from "@/lib/api";
import { Activity, CheckCircle2, AlertTriangle, Clock, TrendingUp } from "lucide-react";

const TIME_RANGES = [
  { label: "24h", days: 1 },
  { label: "7d", days: 7 },
  { label: "30d", days: 30 },
  { label: "90d", days: 90 },
];

export default function APIMetricsPage() {
  useParams<{ id: string }>();
  const workspaceId = useWorkspaceId();
  const { getToken } = useAuth();

  const [range, setRange] = useState(7);
  const [overall, setOverall] = useState<APIMetricsOverall | null>(null);
  const [summary, setSummary] = useState<APIMetricsSummaryRow[]>([]);
  const [trend, setTrend] = useState<APIMetricsTrendRow[]>([]);
  const [loading, setLoading] = useState(true);

  const loadData = useCallback(async () => {
    if (!workspaceId) return;
    setLoading(true);
    try {
      const token = await getToken();
      if (!token) return;
      const to = new Date().toISOString();
      const from = new Date(Date.now() - range * 24 * 60 * 60 * 1000).toISOString();
      const [o, s, t] = await Promise.all([
        getAPIMetricsOverall(token, workspaceId, from, to),
        getAPIMetricsSummary(token, workspaceId, from, to),
        getAPIMetricsTrend(token, workspaceId, from, to),
      ]);
      setOverall(o.data);
      setSummary(s.data || []);
      setTrend(t.data || []);
    } catch (err) {
      console.error("Failed to load API metrics:", err);
    } finally {
      setLoading(false);
    }
  }, [workspaceId, getToken, range]);

  useEffect(() => { loadData(); }, [loadData]);

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 24 }}>
        <div>
          <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>
            API Metrics
          </div>
          <div style={{ fontSize: 14, color: "var(--dmuted)", marginTop: 6 }}>
            Usage, performance, and reliability of your API endpoints.
          </div>
        </div>
        <div style={{ display: "flex", gap: 4, background: "var(--surface1)", borderRadius: 6, padding: 2, border: "1px solid var(--dborder)" }}>
          {TIME_RANGES.map((t) => (
            <button
              key={t.days}
              onClick={() => setRange(t.days)}
              className={`dbtn ${range === t.days ? "dbtn-primary" : "dbtn-ghost"}`}
              style={{ padding: "4px 12px", fontSize: 12 }}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <div style={{ color: "var(--dmuted)", padding: 40, textAlign: "center" }}>Loading metrics...</div>
      ) : !overall || overall.total_calls === 0 ? (
        <div style={{ textAlign: "center", padding: 60, color: "var(--dmuted)" }}>
          <Activity style={{ width: 40, height: 40, margin: "0 auto 12px", opacity: 0.3 }} />
          <div style={{ fontSize: 15, fontWeight: 600, color: "var(--dtext)", marginBottom: 6 }}>No API calls yet</div>
          <div style={{ fontSize: 13 }}>API metrics will appear here once your API keys are used.</div>
        </div>
      ) : (
        <>
          {/* Overview cards */}
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))", gap: 12, marginBottom: 24 }}>
            <MetricCard
              icon={<Activity className="w-4 h-4" />}
              label="Total Calls"
              value={overall.total_calls.toLocaleString()}
              color="var(--dtext)"
            />
            <MetricCard
              icon={<CheckCircle2 className="w-4 h-4" />}
              label="Success Rate"
              value={`${overall.reliability_pct.toFixed(1)}%`}
              color={overall.reliability_pct >= 99 ? "var(--success)" : overall.reliability_pct >= 95 ? "var(--warning)" : "var(--danger)"}
            />
            <MetricCard
              icon={<AlertTriangle className="w-4 h-4" />}
              label="Errors"
              value={`${overall.client_error_count + overall.server_error_count}`}
              sub={`${overall.client_error_count} client · ${overall.server_error_count} server`}
              color={overall.server_error_count > 0 ? "var(--danger)" : "var(--warning)"}
            />
            <MetricCard
              icon={<Clock className="w-4 h-4" />}
              label="Latency (p50 / p95 / p99)"
              value={`${overall.p50_ms}ms`}
              sub={`p95: ${overall.p95_ms}ms · p99: ${overall.p99_ms}ms`}
              color="var(--dtext)"
            />
          </div>

          {/* Trend mini chart (text-based for now) */}
          {trend.length > 0 && (
            <div style={{
              padding: 16,
              background: "var(--surface1)",
              border: "1px solid var(--dborder)",
              borderRadius: 10,
              marginBottom: 24,
            }}>
              <div style={{ fontSize: 11, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.06em", color: "var(--dmuted2)", marginBottom: 12 }}>
                <TrendingUp style={{ width: 12, height: 12, display: "inline", marginRight: 4, verticalAlign: "middle" }} />
                API Calls Over Time
              </div>
              <div style={{ display: "flex", alignItems: "flex-end", gap: 2, height: 80 }}>
                {trend.map((t, i) => {
                  const maxCalls = Math.max(...trend.map((r) => r.total_calls), 1);
                  const height = (t.total_calls / maxCalls) * 100;
                  const errorPct = t.total_calls > 0 ? (t.error_count / t.total_calls) : 0;
                  return (
                    <div
                      key={i}
                      title={`${new Date(t.bucket).toLocaleString()}: ${t.total_calls} calls (${t.error_count} errors)`}
                      style={{
                        flex: 1,
                        height: `${Math.max(height, 2)}%`,
                        background: errorPct > 0.1 ? "var(--warning)" : "var(--success)",
                        borderRadius: "2px 2px 0 0",
                        minWidth: 3,
                        opacity: 0.8,
                        transition: "height 0.2s",
                      }}
                    />
                  );
                })}
              </div>
              <div style={{ display: "flex", justifyContent: "space-between", marginTop: 6, fontSize: 10, color: "var(--dmuted2)" }}>
                <span>{trend.length > 0 ? new Date(trend[0].bucket).toLocaleDateString() : ""}</span>
                <span>{trend.length > 0 ? new Date(trend[trend.length - 1].bucket).toLocaleDateString() : ""}</span>
              </div>
            </div>
          )}

          {/* Per-endpoint table */}
          <div style={{ fontSize: 11, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.06em", color: "var(--dmuted2)", marginBottom: 8 }}>
            Per-Endpoint Breakdown
          </div>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Endpoint</th>
                  <th style={{ textAlign: "right" }}>Calls</th>
                  <th style={{ textAlign: "right" }}>Success</th>
                  <th style={{ textAlign: "right" }}>4xx</th>
                  <th style={{ textAlign: "right" }}>5xx</th>
                  <th style={{ textAlign: "right" }}>Error %</th>
                  <th style={{ textAlign: "right" }}>p50</th>
                  <th style={{ textAlign: "right" }}>p95</th>
                  <th style={{ textAlign: "right" }}>p99</th>
                </tr>
              </thead>
              <tbody>
                {summary.map((row) => {
                  const errorRate = row.total_calls > 0
                    ? ((row.client_error_count + row.server_error_count) / row.total_calls * 100)
                    : 0;
                  return (
                    <tr key={`${row.method}-${row.path}`}>
                      <td>
                        <span style={{ fontSize: 10, fontWeight: 600, color: methodColor(row.method), marginRight: 6, fontFamily: "var(--font-geist-mono), monospace" }}>
                          {row.method}
                        </span>
                        <span style={{ fontFamily: "var(--font-geist-mono), monospace", fontSize: 12.5 }}>
                          {row.path}
                        </span>
                      </td>
                      <td style={{ textAlign: "right", fontFamily: "var(--font-geist-mono), monospace" }}>{row.total_calls}</td>
                      <td style={{ textAlign: "right", color: "var(--success)", fontFamily: "var(--font-geist-mono), monospace" }}>{row.success_count}</td>
                      <td style={{ textAlign: "right", color: row.client_error_count > 0 ? "var(--warning)" : "var(--dmuted)", fontFamily: "var(--font-geist-mono), monospace" }}>{row.client_error_count}</td>
                      <td style={{ textAlign: "right", color: row.server_error_count > 0 ? "var(--danger)" : "var(--dmuted)", fontFamily: "var(--font-geist-mono), monospace" }}>{row.server_error_count}</td>
                      <td style={{ textAlign: "right", color: errorRate > 10 ? "var(--danger)" : errorRate > 5 ? "var(--warning)" : "var(--dmuted)", fontFamily: "var(--font-geist-mono), monospace" }}>
                        {errorRate.toFixed(1)}%
                      </td>
                      <td style={{ textAlign: "right", fontFamily: "var(--font-geist-mono), monospace", color: "var(--dmuted)" }}>{row.p50_ms}ms</td>
                      <td style={{ textAlign: "right", fontFamily: "var(--font-geist-mono), monospace", color: row.p95_ms > 1000 ? "var(--warning)" : "var(--dmuted)" }}>{row.p95_ms}ms</td>
                      <td style={{ textAlign: "right", fontFamily: "var(--font-geist-mono), monospace", color: row.p99_ms > 2000 ? "var(--danger)" : "var(--dmuted)" }}>{row.p99_ms}ms</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </>
      )}
    </>
  );
}

function MetricCard({ icon, label, value, sub, color }: {
  icon: React.ReactNode;
  label: string;
  value: string;
  sub?: string;
  color: string;
}) {
  return (
    <div style={{
      padding: "14px 16px",
      background: "var(--surface1)",
      border: "1px solid var(--dborder)",
      borderRadius: 10,
    }}>
      <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 6 }}>
        <span style={{ color: "var(--dmuted)" }}>{icon}</span>
        <span style={{ fontSize: 10, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.06em", color: "var(--dmuted2)" }}>{label}</span>
      </div>
      <div style={{ fontSize: 22, fontWeight: 700, color, fontFamily: "var(--font-geist-mono), monospace" }}>
        {value}
      </div>
      {sub && (
        <div style={{ fontSize: 11, color: "var(--dmuted)", marginTop: 2, fontFamily: "var(--font-geist-mono), monospace" }}>
          {sub}
        </div>
      )}
    </div>
  );
}

function methodColor(method: string): string {
  switch (method) {
    case "GET": return "var(--success)";
    case "POST": return "var(--info)";
    case "PUT": return "var(--warning)";
    case "PATCH": return "var(--warning)";
    case "DELETE": return "var(--danger)";
    default: return "var(--dmuted)";
  }
}
