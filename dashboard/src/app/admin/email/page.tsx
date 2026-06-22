"use client";

import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { useCallback, useEffect, useMemo, useState } from "react";

import {
  listAdminEmailNotifications,
  type AdminEmailNotificationListParams,
  type AdminEmailNotificationRow,
  type AdminEmailNotificationStatus,
} from "@/lib/api";

import { AdminShell, StatCard, fmtDate, fmtNumber, fmtRelative } from "../_components/admin-ui";
import { SearchHistoryInput } from "../_components/search-history-input";

const STATUS_OPTIONS = ["sent", "pending", "failed", "all"] as const;
const THRESHOLD_OPTIONS = ["all", 80, 85, 90, 95, 100] as const;
const LIMIT_OPTIONS = [50, 100, 200, 500] as const;

function currentPeriod() {
  return new Date().toISOString().slice(0, 7);
}

function usagePercent(row: AdminEmailNotificationRow) {
  if (row.post_limit <= 0) return null;
  return Math.round((row.effective_usage / row.post_limit) * 100);
}

function triggerLabel(row: AdminEmailNotificationRow) {
  if (row.threshold_percent === 100) return "Usage blocked";
  if (row.threshold_percent === 95) return "Block warning";
  return `Usage ${row.threshold_percent}%`;
}

function statusStyle(status: AdminEmailNotificationStatus) {
  if (status === "sent") {
    return {
      background: "var(--success-soft)",
      color: "var(--success)",
      borderColor: "color-mix(in srgb, var(--success) 20%, transparent)",
    };
  }
  if (status === "failed") {
    return {
      background: "var(--danger-soft)",
      color: "var(--danger)",
      borderColor: "color-mix(in srgb, var(--danger) 20%, transparent)",
    };
  }
  return undefined;
}

export default function AdminEmailPage() {
  const { getToken } = useAuth();
  const [rows, setRows] = useState<AdminEmailNotificationRow[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState<(typeof STATUS_OPTIONS)[number]>("sent");
  const [threshold, setThreshold] = useState<(typeof THRESHOLD_OPTIONS)[number]>("all");
  const [period, setPeriod] = useState("");
  const [limit, setLimit] = useState<(typeof LIMIT_OPTIONS)[number]>(100);
  const [offset, setOffset] = useState(0);

  const loadNotifications = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const params: AdminEmailNotificationListParams = {
        search: search || undefined,
        status,
        threshold,
        period: period || undefined,
        limit,
        offset,
      };
      const res = await listAdminEmailNotifications(token, params);
      setRows(res.data);
      setTotal(res.meta?.total ?? res.data.length);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load email notifications");
    } finally {
      setLoading(false);
    }
  }, [getToken, limit, offset, period, search, status, threshold]);

  useEffect(() => {
    loadNotifications();
  }, [loadNotifications]);

  useEffect(() => {
    const timer = setTimeout(() => {
      setSearch(searchInput);
      setOffset(0);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    setOffset(0);
  }, [limit, period, status, threshold]);

  const visibleRange = useMemo(() => {
    if (rows.length === 0) return "0";
    return `${offset + 1}-${offset + rows.length}`;
  }, [offset, rows.length]);

  const sentOnPage = useMemo(() => rows.filter((row) => row.status === "sent").length, [rows]);
  const blockedOnPage = useMemo(() => rows.filter((row) => row.threshold_percent === 100).length, [rows]);
  const latestAttempt = rows[0]?.attempted_at ?? null;
  const canPageBack = offset > 0;
  const canPageForward = offset + rows.length < total;

  return (
    <AdminShell title="Email" loading={loading} onRefresh={loadNotifications}>
      <style>{emailCss}</style>
      {error && (
        <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
          {error}
        </div>
      )}

      <div className="ad-section-header">
        <div>
          <div className="ad-section-title">Email notifications</div>
          <div className="ad-section-meta">Free plan quota reminders sent through Loops, newest first</div>
        </div>
        <div className="ae-period-actions">
          <button type="button" className="ad-btn ad-btn-ghost" onClick={() => setPeriod(currentPeriod())}>
            This month
          </button>
          <button type="button" className="ad-btn ad-btn-ghost" onClick={() => setPeriod("")}>
            All periods
          </button>
        </div>
      </div>

      <div className="ad-filter-bar" style={{ marginBottom: 16 }}>
        <SearchHistoryInput
          fieldKey="admin.email.search"
          className="ad-search"
          placeholder="Search email, workspace, event, or id..."
          value={searchInput}
          onChange={setSearchInput}
          style={{ width: 320 }}
        />
        <select value={status} onChange={(e) => setStatus(e.target.value as typeof status)}>
          {STATUS_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All statuses" : `Status: ${value}`}
            </option>
          ))}
        </select>
        <select value={threshold} onChange={(e) => setThreshold(e.target.value === "all" ? "all" : Number(e.target.value) as typeof threshold)}>
          {THRESHOLD_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All triggers" : `Trigger: ${value}%`}
            </option>
          ))}
        </select>
        <input
          className="ad-search ae-period-input"
          placeholder="Period YYYY-MM"
          value={period}
          onChange={(event) => setPeriod(event.target.value.trim())}
          aria-label="Filter by period"
        />
        <select value={limit} onChange={(e) => setLimit(Number(e.target.value) as typeof limit)}>
          {LIMIT_OPTIONS.map((value) => (
            <option key={value} value={value}>{value} rows</option>
          ))}
        </select>
      </div>

      <div className="ad-stat-grid">
        <StatCard label="Matching Emails" value={fmtNumber(total)} sub={`Showing ${visibleRange}`} />
        <StatCard label="Sent On Page" value={fmtNumber(sentOnPage)} sub={`${fmtNumber(rows.length)} loaded`} valueColor="accent" />
        <StatCard label="Blocked Events" value={fmtNumber(blockedOnPage)} sub="100% threshold on page" subColor={blockedOnPage > 0 ? "down" : undefined} />
        <StatCard label="Latest Attempt" value={latestAttempt ? fmtRelative(latestAttempt) : "—"} sub={latestAttempt ? fmtDate(latestAttempt) : "—"} />
      </div>

      <div className="ad-section-header" style={{ marginTop: 24 }}>
        <div className="ad-section-title" style={{ fontSize: 14 }}>Notification events</div>
        <div className="ad-section-meta">Recipient email is the snapshot used for the Loops send</div>
      </div>

      <div className="ad-tbl-wrap ad-tbl-static">
        <table>
          <thead>
            <tr>
              <th>Trigger Event</th>
              <th>Email</th>
              <th>Status</th>
              <th>Usage</th>
              <th>Workspace</th>
              <th>Period</th>
              <th>Attempted</th>
              <th>Result</th>
            </tr>
          </thead>
          <tbody>
            {loading && rows.length === 0 ? (
              <tr>
                <td colSpan={8} className="ae-empty-cell">Loading email notifications...</td>
              </tr>
            ) : rows.length === 0 ? (
              <tr>
                <td colSpan={8} className="ae-empty-cell">No email notifications match the current filters.</td>
              </tr>
            ) : (
              rows.map((row) => {
                const pct = usagePercent(row);
                return (
                  <tr key={row.id}>
                    <td style={{ minWidth: 180 }}>
                      <div style={{ fontWeight: 600 }}>{triggerLabel(row)}</div>
                      <div className="ad-mono" style={{ marginTop: 3 }}>
                        {row.trigger_event}
                      </div>
                    </td>
                    <td style={{ minWidth: 220 }}>
                      <Link href={`/admin/users?user=${row.user_id}`} className="ad-link">
                        {row.email}
                      </Link>
                      {row.owner_email !== row.email ? (
                        <div className="ad-mono" style={{ marginTop: 3 }}>owner: {row.owner_email}</div>
                      ) : null}
                    </td>
                    <td>
                      <span className="ad-badge ad-b-gray" style={statusStyle(row.status)}>
                        {row.status}
                      </span>
                    </td>
                    <td style={{ minWidth: 150 }}>
                      <div className="ae-usage-line">
                        <span>{fmtNumber(row.effective_usage)}</span>
                        <span>/</span>
                        <span>{fmtNumber(row.post_limit)}</span>
                        {pct != null ? <strong>{pct}%</strong> : null}
                      </div>
                      <div className="ad-mono" style={{ marginTop: 3 }}>
                        done {fmtNumber(row.completed_usage)} + reserved {fmtNumber(row.reserved_usage)}
                      </div>
                    </td>
                    <td style={{ minWidth: 190 }}>
                      <div>{row.workspace_name || "Unnamed workspace"}</div>
                      <div className="ad-mono" style={{ marginTop: 3 }}>{row.workspace_id}</div>
                    </td>
                    <td><span className="ad-badge ad-b-gray">{row.period}</span></td>
                    <td style={{ whiteSpace: "nowrap" }}>
                      <div>{fmtRelative(row.attempted_at)}</div>
                      <div className="ad-mono" style={{ marginTop: 3 }}>{fmtDate(row.attempted_at)}</div>
                    </td>
                    <td style={{ minWidth: 180 }}>
                      {row.status === "failed" ? (
                        <div className="ae-failure">{row.failure_reason || "No failure reason captured"}</div>
                      ) : row.sent_at ? (
                        <>
                          <div>{fmtRelative(row.sent_at)}</div>
                          <div className="ad-mono" style={{ marginTop: 3 }}>sent {fmtDate(row.sent_at)}</div>
                        </>
                      ) : (
                        <span style={{ color: "var(--dmuted2)", fontSize: 11 }}>Waiting</span>
                      )}
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      <div className="ae-pager" aria-label="Email notification pagination">
        <button
          type="button"
          className="ad-btn ad-btn-ghost"
          disabled={!canPageBack}
          onClick={() => setOffset(Math.max(0, offset - limit))}
        >
          Previous
        </button>
        <span className="ad-mono">Rows {visibleRange} of {fmtNumber(total)}</span>
        <button
          type="button"
          className="ad-btn ad-btn-ghost"
          disabled={!canPageForward}
          onClick={() => setOffset(offset + limit)}
        >
          Next
        </button>
      </div>
    </AdminShell>
  );
}

const emailCss = `
.ae-period-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.ae-period-input {
  width: 132px;
}
.ae-empty-cell {
  padding: 24px;
  color: var(--dmuted);
  text-align: center;
}
.ae-usage-line {
  display: flex;
  align-items: baseline;
  gap: 5px;
  font-family: var(--font-geist-mono), monospace;
  font-size: 12px;
}
.ae-usage-line strong {
  margin-left: 4px;
  color: var(--daccent);
  font-size: 11px;
}
.ae-failure {
  max-width: 320px;
  color: var(--danger);
  font-size: 11.5px;
  line-height: 1.45;
}
.ae-pager {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
  margin-top: 14px;
  color: var(--dmuted);
  font-size: 12px;
}
@media (max-width: 720px) {
  .ae-period-actions {
    width: 100%;
    justify-content: flex-start;
    flex-wrap: wrap;
  }
  .ae-period-input {
    width: 100%;
  }
  .ae-pager {
    justify-content: flex-start;
    flex-wrap: wrap;
  }
}
`;
