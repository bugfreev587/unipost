"use client";

import { useAuth } from "@clerk/nextjs";
import { useCallback, useEffect, useState } from "react";

import {
  getAdminObjectStorage,
  type AdminObjectStoragePeriod,
  type AdminObjectStorageResponse,
} from "@/lib/api";

import { AdminShell, StatCard, fmtBytes, fmtDate, fmtNumber, fmtRelative } from "../_components/admin-ui";
import { ObjectStorageDailyChart } from "./object-storage-daily-chart";

const PERIOD_OPTIONS: Array<{ value: AdminObjectStoragePeriod; label: string }> = [
  { value: "yesterday", label: "Yesterday" },
  { value: "last_7_days", label: "Last 7 days" },
  { value: "last_month", label: "Last month" },
  { value: "this_week", label: "This week" },
  { value: "this_month", label: "This month" },
  { value: "this_year", label: "This year" },
];

export default function AdminObjectStoragePage() {
  const { getToken } = useAuth();
  const [period, setPeriod] = useState<AdminObjectStoragePeriod>("last_7_days");
  const [data, setData] = useState<AdminObjectStorageResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadStorage = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const res = await getAdminObjectStorage(token, period);
      setData(res.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load object storage metrics");
    } finally {
      setLoading(false);
    }
  }, [getToken, period]);

  useEffect(() => {
    loadStorage();
  }, [loadStorage]);

  const current = data?.current;
  const worker = data?.worker;
  const metrics = data?.period_metrics;
  const backlog = data?.backlog;

  return (
    <AdminShell title="Object Storage" loading={loading} onRefresh={loadStorage}>
      <style>{objectStorageCss}</style>
      {error && (
        <div className="aos-error">
          {error}
        </div>
      )}

      <div className="ad-section-header">
        <div>
          <div className="ad-section-title">Tracked R2 usage</div>
          <div className="ad-section-meta">Confirmed object size, cleanup worker health, and retention backlog</div>
        </div>
        <div className="ad-filter-bar aos-filter">
          <select value={period} onChange={(event) => setPeriod(event.target.value as AdminObjectStoragePeriod)} aria-label="Object storage period">
            {PERIOD_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>
        </div>
      </div>

      <div className="ad-stat-grid aos-stat-grid">
        <StatCard label="Confirmed tracked size" value={fmtBytes(current?.confirmed_tracked_bytes ?? 0)} sub={`${fmtNumber(current?.uploaded_objects ?? 0)} uploaded objects`} valueColor="accent" />
        <StatCard label="Tracked objects" value={fmtNumber(current?.tracked_objects ?? 0)} sub={`${fmtNumber(current?.pending_objects ?? 0)} pending / ${fmtNumber(current?.referenced_objects ?? 0)} referenced`} />
        <StatCard label="Worker last run" value={worker?.last_run_finished_at ? fmtRelative(worker.last_run_finished_at) : "Never"} sub={worker?.last_run_status || "No cleanup runs recorded yet"} subColor={worker?.last_failed_objects ? "down" : undefined} />
        <StatCard label="Estimated next run" value={worker?.estimated_next_run_at ? fmtRelative(worker.estimated_next_run_at) : "Unknown"} sub={worker?.estimated_next_run_at ? formatDateTime(worker.estimated_next_run_at) : "Runs every 24h plus startup"} />
        <StatCard label="Active cleanup run" value={worker?.active_run_started_at ? fmtRelative(worker.active_run_started_at) : "Idle"} sub={`${fmtNumber(worker?.stale_running_runs ?? 0)} Stale running runs`} subColor={(worker?.stale_running_runs ?? 0) > 0 ? "down" : undefined} />
        <StatCard label="Added objects" value={fmtNumber(metrics?.added_objects ?? 0)} sub={`${fmtBytes(metrics?.added_confirmed_bytes ?? 0)} confirmed in period`} />
        <StatCard label="Deleted in period" value={fmtNumber(metrics?.deleted_objects ?? 0)} sub={fmtBytes(metrics?.deleted_bytes ?? 0)} valueColor={(metrics?.deleted_objects ?? 0) > 0 ? "accent" : undefined} />
        <StatCard label="Failed object count" value={fmtNumber(metrics?.failed_object_count ?? 0)} sub={`${fmtNumber(metrics?.failed_run_count ?? 0)} Failed run count`} subColor={(metrics?.failed_object_count ?? 0) > 0 ? "down" : undefined} />
        <StatCard label="Due cleanup" value={fmtNumber(backlog?.due_objects ?? 0)} sub={`${fmtBytes(backlog?.due_bytes ?? 0)} due now`} subColor={(backlog?.due_objects ?? 0) > 0 ? "down" : undefined} />
      </div>

      <div className="aos-period-note">
        <span>Window</span>
        <strong>{data ? `${formatDateTime(data.period.from)} to ${formatDateTime(data.period.to)}` : "Loading..."}</strong>
        <span>Next cleanup deadline</span>
        <strong>{backlog?.next_cleanup_deadline_at ? formatDateTime(backlog.next_cleanup_deadline_at) : "No future deadline"}</strong>
      </div>

      <section className="aos-chart-card" aria-labelledby="daily-storage-movement-title">
        <div className="ad-section-header aos-chart-header">
          <div>
            <div id="daily-storage-movement-title" className="ad-section-title">Daily storage movement</div>
            <div className="ad-section-meta">Confirmed R2 uploads and completed cleanup deletions by UTC day</div>
          </div>
        </div>
        {loading && !data ? (
          <div className="aos-chart-skeleton" aria-label="Loading daily storage movement"><i /><i /><i /><i /><i /><i /><i /></div>
        ) : data ? (
          <ObjectStorageDailyChart rows={data?.daily_activity ?? []} />
        ) : (
          <div className="aos-chart-empty">Daily activity is unavailable until object storage metrics load.</div>
        )}
      </section>

      <div className="ad-section-header" style={{ marginTop: 22 }}>
        <div className="ad-section-title">Buckets</div>
        <div className="ad-section-meta">Application-tracked rows, not Cloudflare billing inventory</div>
      </div>
      <div className="ad-tbl-wrap ad-tbl-static">
        <table>
          <thead>
            <tr>
              <th>Bucket</th>
              <th>Confirmed tracked size</th>
              <th>Tracked objects</th>
              <th>Pending</th>
              <th>Uploaded</th>
              <th>Referenced</th>
              <th>Due cleanup</th>
            </tr>
          </thead>
          <tbody>
            {loading && !data ? (
              <tr><td colSpan={7} className="aos-empty-cell">Loading object storage metrics...</td></tr>
            ) : !data || data.buckets.length === 0 ? (
              <tr><td colSpan={7} className="aos-empty-cell">No tracked object storage rows found.</td></tr>
            ) : (
              data.buckets.map((bucket) => (
                <tr key={bucket.bucket_name}>
                  <td>
                    <div style={{ fontWeight: 600 }}>{bucket.bucket_name}</div>
                    <div className="ad-mono">R2_BUCKET_NAME</div>
                  </td>
                  <td>{fmtBytes(bucket.confirmed_tracked_bytes)}</td>
                  <td>{fmtNumber(bucket.tracked_objects)}</td>
                  <td>{fmtNumber(bucket.pending_objects)}</td>
                  <td>{fmtNumber(bucket.uploaded_objects)}</td>
                  <td>{fmtNumber(bucket.referenced_objects)}</td>
                  <td>
                    <div>{fmtNumber(bucket.due_objects)}</div>
                    <div className="ad-mono">{fmtBytes(bucket.due_bytes)}</div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className="aos-two-col">
        <section>
          <div className="ad-section-header">
            <div className="ad-section-title">Content types</div>
            <div className="ad-section-meta">Confirmed uploaded bytes</div>
          </div>
          <div className="ad-tbl-wrap ad-tbl-static">
            <table>
              <thead>
                <tr>
                  <th>Type</th>
                  <th>Objects</th>
                  <th>Size</th>
                </tr>
              </thead>
              <tbody>
                {data?.content_types.length ? data.content_types.map((row) => (
                  <tr key={row.content_type || "unknown"}>
                    <td className="ad-mono">{row.content_type || "unknown"}</td>
                    <td>{fmtNumber(row.tracked_objects)}</td>
                    <td>{fmtBytes(row.confirmed_tracked_bytes)}</td>
                  </tr>
                )) : (
                  <tr><td colSpan={3} className="aos-empty-cell">No confirmed uploaded objects yet.</td></tr>
                )}
              </tbody>
            </table>
          </div>
        </section>

        <section>
          <div className="ad-section-header">
            <div className="ad-section-title">Status breakdown</div>
            <div className="ad-section-meta">Pending counts do not inflate bytes</div>
          </div>
          <div className="ad-tbl-wrap ad-tbl-static">
            <table>
              <thead>
                <tr>
                  <th>Status</th>
                  <th>Objects</th>
                  <th>Confirmed size</th>
                </tr>
              </thead>
              <tbody>
                {data?.status_breakdown.length ? data.status_breakdown.map((row) => (
                  <tr key={row.status || "unknown"}>
                    <td><span className="ad-badge ad-b-gray">{row.status || "unknown"}</span></td>
                    <td>{fmtNumber(row.tracked_objects)}</td>
                    <td>{fmtBytes(row.confirmed_tracked_bytes)}</td>
                  </tr>
                )) : (
                  <tr><td colSpan={3} className="aos-empty-cell">No active tracked statuses.</td></tr>
                )}
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <div className="ad-section-header" style={{ marginTop: 22 }}>
        <div className="ad-section-title">Recent cleanup runs</div>
        <div className="ad-section-meta">Latest finished runs only; stale running rows do not hide completed work</div>
      </div>
      <div className="ad-tbl-wrap ad-tbl-static">
        <table>
          <thead>
            <tr>
              <th>Status</th>
              <th>Started</th>
              <th>Finished</th>
              <th>Deleted</th>
              <th>Deleted size</th>
              <th>Failed</th>
              <th>Summary</th>
            </tr>
          </thead>
          <tbody>
            {data?.recent_runs.length ? data.recent_runs.map((run) => (
              <tr key={run.id}>
                <td><span className="ad-badge ad-b-gray" style={runStatusStyle(run.status)}>{run.status}</span></td>
                <td>{run.started_at ? fmtRelative(run.started_at) : "Unknown"}</td>
                <td>{run.finished_at ? fmtRelative(run.finished_at) : "Unknown"}</td>
                <td>{fmtNumber(run.deleted_objects)}</td>
                <td>{fmtBytes(run.deleted_bytes)}</td>
                <td style={{ color: run.failed_objects > 0 ? "var(--danger)" : "var(--dmuted)" }}>{fmtNumber(run.failed_objects)}</td>
                <td className="ad-mono">{run.error_summary || "-"}</td>
              </tr>
            )) : (
              <tr><td colSpan={7} className="aos-empty-cell">No cleanup runs recorded yet.</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </AdminShell>
  );
}

function formatDateTime(iso: string) {
  return `${fmtDate(iso)} ${new Date(iso).toLocaleTimeString("en-US", { hour: "2-digit", minute: "2-digit" })}`;
}

function runStatusStyle(status: string) {
  if (status === "completed") {
    return {
      background: "var(--success-soft)",
      color: "var(--success)",
      borderColor: "color-mix(in srgb, var(--success) 20%, transparent)",
    };
  }
  if (status === "failed" || status === "completed_with_errors") {
    return {
      background: "var(--danger-soft)",
      color: "var(--danger)",
      borderColor: "color-mix(in srgb, var(--danger) 20%, transparent)",
    };
  }
  return undefined;
}

const objectStorageCss = `
.aos-error {
  background: var(--danger-soft);
  border: 1px solid color-mix(in srgb, var(--danger) 22%, transparent);
  border-radius: 8px;
  padding: 12px;
  margin-bottom: 16px;
  color: var(--danger);
  font-size: 13px;
}
.aos-filter { margin: 0; }
.aos-stat-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); }
.aos-period-note {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
  border: 1px solid var(--dborder);
  border-radius: 8px;
  background: var(--surface1);
  padding: 10px 12px;
  margin-bottom: 18px;
  font-size: 12px;
}
.aos-period-note span {
  color: var(--dmuted);
  text-transform: uppercase;
  letter-spacing: .06em;
  font-size: 10px;
  font-weight: 700;
}
.aos-period-note strong {
  font-family: var(--font-geist-mono), monospace;
  font-size: 11px;
  margin-right: 10px;
}
.aos-chart-card {
  border: 1px solid var(--dborder);
  border-radius: 10px;
  background: var(--surface1);
  padding: 14px 16px 12px;
  margin-bottom: 22px;
}
.aos-chart-header { margin-bottom: 12px; }
.aos-chart {
  min-width: 0;
}
.aos-chart-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin-bottom: 12px;
  color: var(--dmuted);
  font-size: 11px;
  font-weight: 600;
}
.aos-chart-legend span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.aos-chart-legend i {
  width: 8px;
  height: 8px;
  border-radius: 2px;
}
.aos-chart-confirm { background: #ef4444; }
.aos-chart-delete { background: #22c55e; }
.aos-chart-scroll {
  display: grid;
  grid-template-columns: 52px minmax(520px, 1fr);
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 2px;
}
.aos-chart-axis {
  display: grid;
  grid-template-rows: repeat(5, 1fr);
  min-height: 220px;
  color: var(--dmuted);
  font-family: var(--font-geist-mono), monospace;
  font-size: 10px;
  text-align: right;
}
.aos-chart-axis span {
  transform: translateY(-7px);
  white-space: nowrap;
}
.aos-chart-axis span:last-child { transform: translateY(5px); }
.aos-chart-plot {
  position: relative;
  min-height: 220px;
}
.aos-chart-gridlines {
  position: absolute;
  inset: 0 0 32px;
  display: grid;
  grid-template-rows: repeat(4, 1fr);
  pointer-events: none;
}
.aos-chart-gridlines i { border-top: 1px solid color-mix(in srgb, var(--dborder) 85%, transparent); }
.aos-chart-gridlines i:last-child { border-bottom: 1px solid color-mix(in srgb, var(--dborder) 85%, transparent); }
.aos-chart-groups {
  position: relative;
  z-index: 1;
  display: grid;
  height: 220px;
  gap: 6px;
}
.aos-chart-group {
  display: grid;
  grid-template-rows: minmax(0, 1fr) 24px;
  gap: 8px;
  min-width: 0;
}
.aos-chart-bars {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-items: end;
  gap: 3px;
  min-height: 0;
}
.aos-chart-bar {
  width: 100%;
  min-height: 0;
  align-self: end;
  border: 0;
  border-radius: 3px 3px 0 0;
  cursor: pointer;
  transform-origin: bottom;
  transition: transform .15s ease, filter .15s ease;
}
.aos-chart-bar:hover,
.aos-chart-bar:focus-visible {
  filter: brightness(1.12);
  transform: scaleX(.94);
  outline: 2px solid var(--dtext);
  outline-offset: 2px;
}
.aos-chart-bar[data-empty="true"] {
  min-height: 12px;
  background: transparent;
}
.aos-chart-date {
  overflow: hidden;
  color: var(--dmuted);
  font-family: var(--font-geist-mono), monospace;
  font-size: 10px;
  line-height: 1.25;
  text-align: center;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.aos-chart-tooltip {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 10px;
  margin-top: 10px;
  color: var(--dmuted);
  font-family: var(--font-geist-mono), monospace;
  font-size: 11px;
}
.aos-chart-tooltip strong { color: var(--dtext); }
.aos-chart-empty {
  display: grid;
  min-height: 220px;
  place-items: center;
  border: 1px dashed color-mix(in srgb, var(--dborder) 85%, transparent);
  border-radius: 8px;
  color: var(--dmuted);
  font-size: 12px;
  text-align: center;
}
.aos-chart-skeleton {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  align-items: end;
  gap: 10px;
  min-height: 220px;
  padding: 20px 12px 32px 64px;
}
.aos-chart-skeleton i {
  display: block;
  min-height: 28px;
  border-radius: 3px 3px 0 0;
  background: color-mix(in srgb, var(--dborder) 60%, transparent);
}
.aos-chart-skeleton i:nth-child(2n) { min-height: 76px; }
.aos-chart-skeleton i:nth-child(3n) { min-height: 118px; }
.aos-two-col {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(0, .9fr);
  gap: 16px;
  margin-top: 22px;
}
.aos-empty-cell {
  padding: 24px;
  color: var(--dmuted);
  text-align: center;
}
@media (max-width: 1080px) {
  .aos-stat-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .aos-two-col { grid-template-columns: 1fr; }
}
@media (max-width: 640px) {
  .aos-stat-grid { grid-template-columns: 1fr; }
  .aos-filter { width: 100%; }
  .aos-filter select { width: 100%; }
  .aos-chart-card { padding: 14px 12px 12px; }
  .aos-chart-scroll { grid-template-columns: 44px minmax(480px, 1fr); }
  .aos-chart-axis { font-size: 9px; }
}
`;
