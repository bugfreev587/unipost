"use client";

import { useAuth } from "@clerk/nextjs";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  getAdminLandingVisitors,
  type AdminLandingVisitorRow,
  type AdminLandingVisitorsResponse,
  type AdminLandingVisitorTrendRow,
} from "@/lib/api";
import { countryDisplay, countryNameFromCode } from "@/lib/countries";

import { CountryDonut, SourceDonut } from "../_components/country-donut";
import { AdminShell, StatCard, fmtDate, fmtNumber, fmtRelative } from "../_components/admin-ui";

const RANGE_OPTIONS = [7, 30, 90, 180] as const;
const LIMIT_OPTIONS = [50, 100, 200, 500] as const;

function fmtPct(value: number) {
  if (!Number.isFinite(value) || value <= 0) return "—";
  return `${(value * 100).toFixed(1)}%`;
}

function shortSession(sessionId: string) {
  if (sessionId.length <= 14) return sessionId;
  return `${sessionId.slice(0, 8)}…${sessionId.slice(-5)}`;
}

function referrerHost(referrer: string) {
  if (!referrer) return "direct";
  try {
    return new URL(referrer).hostname;
  } catch {
    return referrer;
  }
}

function fillTrend(rows: AdminLandingVisitorTrendRow[], days: number) {
  const byDate = new Map(rows.map((row) => [row.date, row]));
  const today = new Date();
  today.setUTCHours(0, 0, 0, 0);
  return Array.from({ length: days }, (_, i) => {
    const d = new Date(today);
    d.setUTCDate(today.getUTCDate() - (days - 1 - i));
    const key = d.toISOString().slice(0, 10);
    return byDate.get(key) ?? { date: key, visits: 0, unique_visitors: 0, signups: 0 };
  });
}

function AttributionBadges({ row }: { row: AdminLandingVisitorRow }) {
  const { attribution } = row;
  return (
    <div className="av-badges">
      <span className="ad-badge ad-b-gray">s:{attribution.utm_source || row.source_code}</span>
      {attribution.utm_medium ? <span className="ad-badge ad-b-gray">m:{attribution.utm_medium}</span> : null}
      {attribution.utm_campaign ? <span className="ad-badge ad-b-blue">c:{attribution.utm_campaign}</span> : null}
      {attribution.r ? <span className="ad-badge ad-b-gray">r:{attribution.r}</span> : null}
    </div>
  );
}

function CountryBadge({ code }: { code?: string | null }) {
  const name = countryNameFromCode(code);
  if (!name) return <span style={{ color: "var(--dmuted2)", fontSize: 11 }}>—</span>;
  return <span className="ad-badge ad-b-gray" title={countryDisplay(code)}>{name}</span>;
}

function VisitorsTrend({ rows }: { rows: AdminLandingVisitorTrendRow[] }) {
  const maxVisits = Math.max(1, ...rows.map((row) => row.visits));
  return (
    <div className="av-trend">
      {rows.map((row) => {
        const height = Math.max(3, Math.round((row.visits / maxVisits) * 92));
        return (
          <div className="av-trend-col" key={row.date} title={`${row.date}: ${row.visits} visits, ${row.unique_visitors} visitors, ${row.signups} signups`}>
            <div className="av-trend-bars">
              {row.signups > 0 ? <span className="av-trend-signups" style={{ height: Math.max(4, Math.round((row.signups / maxVisits) * 92)) }} /> : null}
              <span className="av-trend-visits" style={{ height }} />
            </div>
            <span className="av-trend-date">{row.date.slice(5)}</span>
          </div>
        );
      })}
    </div>
  );
}

function VisitorDetail({ row, onClose }: { row: AdminLandingVisitorRow; onClose: () => void }) {
  return (
    <aside className="ad-detail-panel">
      <div className="ad-panel-header">
        <div>
          <div className="ad-panel-title">{row.label}</div>
          <div className="ad-mono">{fmtDate(row.created_at)} · {fmtRelative(row.created_at)}</div>
        </div>
        <button className="ad-close-btn" onClick={onClose}>×</button>
      </div>

      <div className="ad-panel-section">
        <div className="ad-panel-section-title">Attribution</div>
        <div className="av-detail-grid">
          <span>source</span><strong>{row.attribution.utm_source || row.source_code}</strong>
          <span>medium</span><strong>{row.attribution.utm_medium || "—"}</strong>
          <span>campaign</span><strong>{row.attribution.utm_campaign || "—"}</strong>
          <span>r</span><strong>{row.attribution.r || "—"}</strong>
        </div>
      </div>

      <div className="ad-panel-section">
        <div className="ad-panel-section-title">Visitor</div>
        <div className="av-detail-grid">
          <span>session</span><strong>{row.session_id}</strong>
          <span>country</span><strong>{countryDisplay(row.country_code)}</strong>
          <span>user</span><strong>{row.user_email || row.user_id || "not bound"}</strong>
          <span>path</span><strong>{row.path}</strong>
        </div>
      </div>

      <div className="ad-panel-section">
        <div className="ad-panel-section-title">Raw</div>
        <div className="av-raw">{row.raw_query || "—"}</div>
        <div className="av-raw" style={{ marginTop: 8 }}>{row.referrer || "No referrer"}</div>
      </div>
    </aside>
  );
}

export default function AdminVisitorsPage() {
  const { getToken } = useAuth();
  const [days, setDays] = useState<(typeof RANGE_OPTIONS)[number]>(30);
  const [limit, setLimit] = useState<(typeof LIMIT_OPTIONS)[number]>(100);
  const [source, setSource] = useState("");
  const [campaign, setCampaign] = useState("");
  const [data, setData] = useState<AdminLandingVisitorsResponse | null>(null);
  const [selected, setSelected] = useState<AdminLandingVisitorRow | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadVisitors = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const res = await getAdminLandingVisitors(token, { days, source: source || undefined, campaign: campaign || undefined, limit });
      setData(res.data);
      setSelected((current) => current ? res.data.rows.find((row) => row.id === current.id) ?? null : null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load visitors");
    } finally {
      setLoading(false);
    }
  }, [campaign, days, getToken, limit, source]);

  useEffect(() => {
    loadVisitors();
  }, [loadVisitors]);

  const trend = useMemo(() => fillTrend(data?.trend ?? [], days), [data?.trend, days]);
  const signupRate = data && data.unique_visitors > 0 ? data.signups / data.unique_visitors : 0;
  const latest = data?.rows[0]?.created_at ?? null;

  return (
    <AdminShell title="Visitors" loading={loading} onRefresh={loadVisitors}>
      <style>{visitorsCss}</style>
      {error && (
        <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
          {error}
        </div>
      )}

      <div className="ad-section-header">
        <div>
          <div className="ad-section-title">Visitors</div>
          <div className="ad-section-meta">UTM records, source aliases, and session-to-user bindings</div>
        </div>
        <div className="ad-filter-bar" style={{ marginBottom: 0 }}>
          <select value={days} onChange={(e) => setDays(Number(e.target.value) as typeof days)}>
            {RANGE_OPTIONS.map((option) => <option key={option} value={option}>{option}d</option>)}
          </select>
          <select value={source} onChange={(e) => setSource(e.target.value)}>
            <option value="">All sources</option>
            {(data?.source_options ?? []).map((option) => <option key={option} value={option}>{option}</option>)}
          </select>
          <select value={campaign} onChange={(e) => setCampaign(e.target.value)}>
            <option value="">All campaigns</option>
            {(data?.campaign_options ?? []).map((option) => <option key={option} value={option}>{option}</option>)}
          </select>
          <select value={limit} onChange={(e) => setLimit(Number(e.target.value) as typeof limit)}>
            {LIMIT_OPTIONS.map((option) => <option key={option} value={option}>{option} rows</option>)}
          </select>
        </div>
      </div>

      <div className="ad-stat-grid">
        <StatCard label="Visits" value={data ? fmtNumber(data.total_visits) : "—"} sub={`Last ${data?.range_days ?? days} days`} />
        <StatCard label="Unique Visitors" value={data ? fmtNumber(data.unique_visitors) : "—"} sub={data && data.total_visits > 0 ? `${fmtPct(data.unique_visitors / data.total_visits)} unique rate` : "—"} />
        <StatCard label="Bound Signups" value={data ? fmtNumber(data.signups) : "—"} sub={fmtPct(signupRate)} valueColor="accent" />
        <StatCard label="Latest Visit" value={latest ? fmtRelative(latest) : "—"} sub={latest ? fmtDate(latest) : "—"} />
      </div>

      <div className="av-panel av-trend-panel">
        <div className="ad-section-header">
          <div className="ad-section-title">Daily trend</div>
          <div className="ad-section-meta">bars show visits; green ticks indicate bound signups</div>
        </div>
        <VisitorsTrend rows={trend} />
      </div>

      <div className="av-breakdown-grid">
        <CountryDonut
          title="Visitor countries"
          subtitle={`Last ${data?.range_days ?? days} days`}
          rows={data?.countries ?? []}
          loading={loading}
          valueLabel="visitors"
        />
        <SourceDonut
          title="Visitor sources"
          subtitle={`Last ${data?.range_days ?? days} days`}
          rows={data?.sources ?? []}
          loading={loading}
          valueLabel="visitors"
        />
      </div>

      <div className="ad-section-header" style={{ marginTop: 18 }}>
        <div className="ad-section-title">Recent visits</div>
        <div className="ad-section-meta">Click a row to inspect raw UTM and referrer data</div>
      </div>

      <div className="ad-tbl-wrap">
        <table>
          <thead>
            <tr>
              <th>Time</th>
              <th>Source</th>
              <th>Country</th>
              <th>UTM</th>
              <th>Path</th>
              <th>User</th>
              <th>Session</th>
            </tr>
          </thead>
          <tbody>
            {data && data.rows.length > 0 ? (
              data.rows.map((row) => (
                <tr key={row.id} onClick={() => setSelected(row)}>
                  <td>
                    <div>{fmtRelative(row.created_at)}</div>
                    <div className="ad-mono">{new Date(row.created_at).toLocaleString()}</div>
                  </td>
                  <td>
                    <div style={{ fontWeight: 600 }}>{row.label}</div>
                    <span className="ad-badge ad-b-gray">{row.source_code}</span>
                    <div className="ad-mono" style={{ marginTop: 4 }}>utm: {row.attribution.utm_source || "—"}</div>
                  </td>
                  <td><CountryBadge code={row.country_code} /></td>
                  <td><AttributionBadges row={row} /></td>
                  <td>
                    <div className="av-path">{row.path}</div>
                    <div className="ad-mono">{referrerHost(row.referrer)}</div>
                  </td>
                  <td>{row.user_email ? <span className="ad-mono">{row.user_email}</span> : <span style={{ color: "var(--dmuted)" }}>—</span>}</td>
                  <td><span className="ad-mono">{shortSession(row.session_id)}</span></td>
                </tr>
              ))
            ) : (
              <tr>
                <td colSpan={7} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>
                  {loading ? "Loading…" : "No visitor data matched this filter."}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {selected ? <VisitorDetail row={selected} onClose={() => setSelected(null)} /> : null}
    </AdminShell>
  );
}

const visitorsCss = `
.av-trend-panel {
  margin-bottom: 10px;
}
.av-breakdown-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 18px;
}
.av-panel {
  background: var(--surface);
  border: 1px solid var(--dborder);
  border-radius: 8px;
  padding: 14px 16px 16px;
  min-height: 280px;
}
.av-trend {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(22px, 1fr));
  align-items: end;
  gap: 5px;
  min-height: 132px;
}
.av-trend-col {
  display: grid;
  grid-template-rows: 96px 18px;
  gap: 6px;
  min-width: 0;
}
.av-trend-bars {
  position: relative;
  display: flex;
  align-items: end;
  justify-content: center;
  height: 96px;
  border-bottom: 1px solid var(--dborder);
}
.av-trend-visits {
  width: 100%;
  max-width: 18px;
  border-radius: 4px 4px 0 0;
  background: color-mix(in srgb, var(--daccent) 72%, var(--surface3));
}
.av-trend-signups {
  position: absolute;
  right: 1px;
  bottom: 0;
  width: 4px;
  border-radius: 4px 4px 0 0;
  background: var(--success);
}
.av-trend-date {
  font-family: var(--font-geist-mono), monospace;
  font-size: 9.5px;
  color: var(--dmuted2);
  text-align: center;
  overflow: hidden;
  white-space: nowrap;
}
.av-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  max-width: 360px;
}
.av-path {
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}
.av-detail-grid {
  display: grid;
  grid-template-columns: 74px minmax(0, 1fr);
  gap: 7px 10px;
  font-size: 12px;
}
.av-detail-grid span {
  color: var(--dmuted);
}
.av-detail-grid strong {
  min-width: 0;
  color: var(--dtext);
  font-family: var(--font-geist-mono), monospace;
  font-size: 11px;
  font-weight: 500;
  word-break: break-all;
}
.av-raw {
  background: var(--surface2);
  border: 1px solid var(--dborder);
  border-radius: 6px;
  color: var(--dtext);
  font-family: var(--font-geist-mono), monospace;
  font-size: 11px;
  line-height: 1.55;
  padding: 9px 10px;
  word-break: break-all;
}
@media (max-width: 1100px) {
  .ad-stat-grid { grid-template-columns: repeat(2, 1fr); }
  .av-breakdown-grid { grid-template-columns: 1fr; }
}
`;
