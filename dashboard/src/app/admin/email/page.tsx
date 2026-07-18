"use client";

import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { useCallback, useEffect, useMemo, useState } from "react";

import {
  listAdminEmailNotificationFilterOptions,
  listAdminEmailNotifications,
  retryAdminPaidQuotaEmailNotification,
  type AdminEmailNotificationListParams,
  type AdminEmailNotificationRow,
  type AdminEmailNotificationStatus,
} from "@/lib/api";

import { AdminShell, StatCard, fmtDate, fmtNumber, fmtRelative } from "../_components/admin-ui";
import { SearchHistoryInput } from "../_components/search-history-input";
import { buildAttemptedDateRange } from "./filters";

const STATUS_OPTIONS = [
  "all",
  "failed",
  "retry_wait",
  "processing",
  "pending",
  "sent",
  "skipped",
  "skipped_superseded",
  "skipped_preference_disabled",
  "skipped_missing_recipient",
] as const;
const PROVIDER_OPTIONS = ["all", "loops", "notification_system", "resend_legacy"] as const;
const THRESHOLD_OPTIONS = ["all", 80, 85, 90, 95, 100, 105, 110, 115, 120] as const;
const LIMIT_OPTIONS = [50, 100, 200, 500] as const;

function currentPeriod() {
  return new Date().toISOString().slice(0, 7);
}

function usagePercent(row: AdminEmailNotificationRow) {
  if (row.post_limit <= 0) return null;
  return Math.round((row.effective_usage / row.post_limit) * 100);
}

function triggerLabel(row: AdminEmailNotificationRow) {
  if (row.threshold_percent <= 0) return row.event_key.replace(/^email\./, "");
  if (row.threshold_percent >= 120) return "Critical paid quota alert";
  if (row.threshold_percent >= 100) return "Paid scheduling alert";
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
  if (status.startsWith("skipped")) {
    return {
      background: "color-mix(in srgb, var(--dmuted) 12%, transparent)",
      color: "var(--dmuted)",
      borderColor: "color-mix(in srgb, var(--dmuted) 18%, transparent)",
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
  const [emailOptions, setEmailOptions] = useState<string[]>([]);
  const [emailOptionsLoading, setEmailOptionsLoading] = useState(true);
  const [filterOptionsError, setFilterOptionsError] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [email, setEmail] = useState("all");
  const [status, setStatus] = useState<(typeof STATUS_OPTIONS)[number]>("all");
  const [provider, setProvider] = useState<(typeof PROVIDER_OPTIONS)[number]>("all");
  const [eventKey, setEventKey] = useState("");
  const [threshold, setThreshold] = useState<(typeof THRESHOLD_OPTIONS)[number]>("all");
  const [period, setPeriod] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [limit, setLimit] = useState<(typeof LIMIT_OPTIONS)[number]>(100);
  const [offset, setOffset] = useState(0);
  const [retryingId, setRetryingId] = useState<string | null>(null);
  const range = useMemo(
    () => buildAttemptedDateRange(startDate, endDate),
    [endDate, startDate],
  );

  const loadFilterOptions = useCallback(async () => {
    setEmailOptionsLoading(true);
    setFilterOptionsError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const res = await listAdminEmailNotificationFilterOptions(token);
      setEmailOptions(res.data.emails);
    } catch (e) {
      setFilterOptionsError(
        e instanceof Error ? e.message : "Failed to load email filter options",
      );
    } finally {
      setEmailOptionsLoading(false);
    }
  }, [getToken]);

  const loadNotifications = useCallback(async () => {
    if (range.error) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const params: AdminEmailNotificationListParams = {
        search: search || undefined,
        status,
        provider,
        event_key: eventKey || undefined,
        email,
        threshold,
        period: period || undefined,
        start_at: range.start_at,
        end_at: range.end_at,
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
  }, [email, eventKey, getToken, limit, offset, period, provider, range.end_at, range.error, range.start_at, search, status, threshold]);

  const refreshPage = useCallback(async () => {
    await Promise.all([loadNotifications(), loadFilterOptions()]);
  }, [loadFilterOptions, loadNotifications]);

  const retryNotification = useCallback(async (notificationId: string) => {
    setRetryingId(notificationId);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      await retryAdminPaidQuotaEmailNotification(token, notificationId);
      await loadNotifications();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to retry paid quota notification");
    } finally {
      setRetryingId(null);
    }
  }, [getToken, loadNotifications]);

  useEffect(() => {
    loadNotifications();
  }, [loadNotifications]);

  useEffect(() => {
    loadFilterOptions();
  }, [loadFilterOptions]);

  useEffect(() => {
    const timer = setTimeout(() => {
      setSearch(searchInput);
      setOffset(0);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    setOffset(0);
  }, [email, endDate, eventKey, limit, period, provider, startDate, status, threshold]);

  const visibleRange = useMemo(() => {
    if (rows.length === 0) return "0";
    return `${offset + 1}-${offset + rows.length}`;
  }, [offset, rows.length]);

  const sentOnPage = useMemo(() => rows.filter((row) => row.status === "sent").length, [rows]);
  const failedOnPage = useMemo(() => rows.filter((row) => row.status === "failed").length, [rows]);
  const skippedOnPage = useMemo(() => rows.filter((row) => row.status.startsWith("skipped")).length, [rows]);
  const latestAttempt = rows[0]?.attempted_at ?? null;
  const canPageBack = offset > 0;
  const canPageForward = offset + rows.length < total;

  return (
    <AdminShell title="Email" loading={loading} onRefresh={refreshPage}>
      <style>{emailCss}</style>
      {error && (
        <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
          {error}
        </div>
      )}
      {filterOptionsError && (
        <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
          Email filter options: {filterOptionsError}
        </div>
      )}

      <div className="ad-section-header">
        <div>
          <div className="ad-section-title">Email sends</div>
          <div className="ad-section-meta">User-facing email attempts, Loops audit rows, and migration skip records</div>
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

      <div className="ae-primary-filters">
        <label className="ae-filter-field ae-email-filter">
          <span>Email</span>
          <select
            value={email}
            disabled={emailOptionsLoading}
            aria-label="Filter by recipient email"
            aria-busy={emailOptionsLoading}
            onChange={(event) => setEmail(event.target.value)}
          >
            <option value="all">All emails</option>
            {emailOptions.map((option) => (
              <option key={option.toLowerCase()} value={option}>
                {option}
              </option>
            ))}
          </select>
        </label>
        <label className="ae-filter-field ae-status-filter">
          <span>Status</span>
          <select
            value={status}
            aria-label="Filter by status"
            onChange={(event) => setStatus(event.target.value as typeof status)}
          >
            {STATUS_OPTIONS.map((value) => (
              <option key={value} value={value}>
                {value === "all" ? "All statuses" : value}
              </option>
            ))}
          </select>
        </label>
        <label className="ae-filter-field ae-date-filter">
          <span>Attempted from</span>
          <input
            type="date"
            value={startDate}
            aria-label="Attempted from"
            onChange={(event) => setStartDate(event.target.value)}
          />
        </label>
        <label className="ae-filter-field ae-date-filter">
          <span>Attempted through</span>
          <input
            type="date"
            value={endDate}
            aria-label="Attempted through"
            onChange={(event) => setEndDate(event.target.value)}
          />
        </label>
      </div>
      {range.error ? (
        <div className="ae-range-error" role="alert">
          {range.error}
        </div>
      ) : null}

      <div className="ad-filter-bar" style={{ marginBottom: 16 }}>
        <SearchHistoryInput
          fieldKey="admin.email.search"
          className="ad-search"
          placeholder="Search email, workspace, event, or id..."
          value={searchInput}
          onChange={setSearchInput}
          style={{ width: 320 }}
        />
        <select value={provider} onChange={(e) => setProvider(e.target.value as typeof provider)}>
          {PROVIDER_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All providers" : `Provider: ${value}`}
            </option>
          ))}
        </select>
        <input
          className="ad-search ae-event-key-input"
          placeholder="Event key"
          value={eventKey}
          onChange={(event) => setEventKey(event.target.value.trim())}
          aria-label="Filter by email event key"
        />
        <select value={threshold} onChange={(e) => setThreshold(e.target.value === "all" ? "all" : Number(e.target.value) as typeof threshold)}>
          {THRESHOLD_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All quota triggers" : `Quota: ${value}%`}
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
        <StatCard label="Matching Sends" value={fmtNumber(total)} sub={`Showing ${visibleRange}`} />
        <StatCard label="Sent On Page" value={fmtNumber(sentOnPage)} sub={`${fmtNumber(rows.length)} loaded`} valueColor="accent" />
        <StatCard label="Failed On Page" value={fmtNumber(failedOnPage)} sub="Provider or delivery errors" subColor={failedOnPage > 0 ? "down" : undefined} />
        <StatCard label="Skipped On Page" value={fmtNumber(skippedOnPage)} sub="Legacy email fanout suppressed" />
        <StatCard label="Latest Attempt" value={latestAttempt ? fmtRelative(latestAttempt) : "—"} sub={latestAttempt ? fmtDate(latestAttempt) : "—"} />
      </div>

      <div className="ad-section-header" style={{ marginTop: 24 }}>
        <div className="ad-section-title" style={{ fontSize: 14 }}>Recent email activity</div>
        <div className="ad-section-meta">Recipient email is the snapshot used for the provider send</div>
      </div>

      <div className="ad-tbl-wrap ad-tbl-static">
        <table>
          <thead>
            <tr>
              <th>Trigger Event</th>
              <th>Email</th>
              <th>Status</th>
              <th>Provider</th>
              <th>Audit</th>
              <th>Workspace</th>
              <th>Attempted</th>
              <th>Result</th>
            </tr>
          </thead>
          <tbody>
            {loading && rows.length === 0 ? (
              <tr>
                <td colSpan={8} className="ae-empty-cell">Loading email sends...</td>
              </tr>
            ) : rows.length === 0 ? (
              <tr>
                <td colSpan={8} className="ae-empty-cell">No email sends match the current filters.</td>
              </tr>
            ) : (
              rows.map((row) => {
                const pct = usagePercent(row);
                return (
                  <tr key={row.id}>
                    <td style={{ minWidth: 180 }}>
                      <div style={{ fontWeight: 600 }}>{triggerLabel(row)}</div>
                      <div className="ad-mono" style={{ marginTop: 3 }}>
                        {row.event_key}
                      </div>
                      <div className="ad-mono" style={{ marginTop: 3 }}>
                        ref {row.trigger_event}
                      </div>
                    </td>
                    <td style={{ minWidth: 220 }}>
                      {row.user_id ? (
                        <Link href={`/admin/users?user=${row.user_id}`} className="ad-link">
                          {row.email}
                        </Link>
                      ) : (
                        <span>{row.email}</span>
                      )}
                      {row.owner_email && row.owner_email !== row.email ? (
                        <div className="ad-mono" style={{ marginTop: 3 }}>owner: {row.owner_email}</div>
                      ) : null}
                    </td>
                    <td>
                      <span className="ad-badge ad-b-gray" style={statusStyle(row.status)}>
                        {row.status}
                      </span>
                      {row.severity ? (
                        <div className="ad-mono" style={{ marginTop: 4 }}>{row.severity}</div>
                      ) : null}
                      {row.attempt_count > 0 ? (
                        <div className="ad-mono" style={{ marginTop: 3 }}>attempt {fmtNumber(row.attempt_count)}</div>
                      ) : null}
                    </td>
                    <td style={{ minWidth: 150 }}>
                      <div>{row.provider || "unknown"}</div>
                      <div className="ad-mono" style={{ marginTop: 3 }}>{row.transactional_id || "no template id"}</div>
                    </td>
                    <td style={{ minWidth: 150 }}>
                      {pct != null ? (
                        <>
                          <div className="ae-usage-line">
                            <span>{fmtNumber(row.effective_usage)}</span>
                            <span>/</span>
                            <span>{fmtNumber(row.post_limit)}</span>
                            <strong>{pct}%</strong>
                          </div>
                          <div className="ad-mono" style={{ marginTop: 3 }}>
                            done {fmtNumber(row.completed_usage)} + scheduled {fmtNumber(row.reserved_usage)} + held {fmtNumber(row.quota_hold_usage)}
                          </div>
                          <div className="ad-mono" style={{ marginTop: 3 }}>
                            {row.preference_category || "no preference category"}
                          </div>
                          <div className="ad-mono" style={{ marginTop: 3 }}>
                            {row.preference_decision || row.footer_policy || "no policy decision"}
                          </div>
                        </>
                      ) : (
                        <>
                          <div>{row.delivery_class || "unclassified"}</div>
                          <div className="ad-mono ae-idempotency" style={{ marginTop: 3 }}>
                            {row.idempotency_key || "repeatable test send"}
                          </div>
                          <div className="ad-mono" style={{ marginTop: 3 }}>
                            {row.preference_category || "no preference category"}
                          </div>
                          <div className="ad-mono" style={{ marginTop: 3 }}>
                            {row.preference_decision || row.footer_policy || "no policy decision"}
                          </div>
                        </>
                      )}
                    </td>
                    <td style={{ minWidth: 190 }}>
                      <div>{row.workspace_name || "Unnamed workspace"}</div>
                      <div className="ad-mono" style={{ marginTop: 3 }}>{row.workspace_id}</div>
                    </td>
                    <td style={{ whiteSpace: "nowrap" }}>
                      <div>{fmtRelative(row.attempted_at)}</div>
                      <div className="ad-mono" style={{ marginTop: 3 }}>{fmtDate(row.attempted_at)}</div>
                    </td>
                    <td style={{ minWidth: 180 }}>
                      {row.status === "failed" ? (
                        <div>
                          <div className="ae-failure">{row.failure_reason || "No failure reason captured"}</div>
                          {row.retryable ? (
                            <button
                              type="button"
                              className="ad-btn ad-btn-ghost ae-retry-button"
                              disabled={retryingId === row.id}
                              onClick={() => void retryNotification(row.id)}
                            >
                              {retryingId === row.id ? "Queuing..." : "Retry"}
                            </button>
                          ) : null}
                        </div>
                      ) : row.status.startsWith("skipped") ? (
                        <div className="ad-mono">
                          {row.preference_decision === "skipped_preference_disabled"
                            ? "Skipped by user email preference"
                            : "Skipped by migration policy"}
                        </div>
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
.ae-primary-filters {
  display: flex;
  align-items: flex-end;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 10px;
}
.ae-filter-field {
  display: grid;
  gap: 5px;
}
.ae-filter-field > span {
  color: var(--dmuted);
  font-size: 10px;
  font-weight: 650;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}
.ae-filter-field select,
.ae-filter-field input {
  min-height: 31px;
  background: var(--surface2);
  border: 1px solid var(--dborder2);
  border-radius: 6px;
  color: var(--dtext);
  font-family: inherit;
  font-size: 12px;
  outline: none;
  padding: 5px 10px;
}
.ae-filter-field select {
  cursor: pointer;
}
.ae-filter-field select:disabled {
  cursor: wait;
  opacity: 0.65;
}
.ae-filter-field select:focus,
.ae-filter-field input:focus {
  border-color: color-mix(in srgb, var(--primary) 32%, transparent);
  box-shadow: 0 0 0 3px var(--focus-ring);
}
.ae-email-filter {
  flex: 1 1 280px;
  max-width: 400px;
}
.ae-status-filter {
  width: 190px;
}
.ae-date-filter {
  width: 158px;
}
.ae-range-error {
  margin: -2px 0 12px;
  color: var(--danger);
  font-size: 11.5px;
}
.ae-period-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.ae-period-input {
  width: 132px;
}
.ae-event-key-input {
  width: 240px;
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
.ae-retry-button {
  margin-top: 8px;
}
.ae-idempotency {
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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
  .ae-primary-filters {
    align-items: stretch;
    flex-direction: column;
  }
  .ae-email-filter,
  .ae-status-filter,
  .ae-date-filter {
    width: 100%;
    max-width: none;
  }
  .ae-period-actions {
    width: 100%;
    justify-content: flex-start;
    flex-wrap: wrap;
  }
  .ae-period-input {
    width: 100%;
  }
  .ae-event-key-input {
    width: 100%;
  }
  .ae-pager {
    justify-content: flex-start;
    flex-wrap: wrap;
  }
}
`;
