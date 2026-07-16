"use client";

import { useAuth } from "@clerk/nextjs";
import { useCallback, useEffect, useMemo, useState } from "react";

import {
  listAdminPaidQuotaFollowUps,
  updateAdminPaidQuotaFollowUp,
  type AdminPaidQuotaFollowUpRow,
} from "@/lib/api";

import { AdminShell, StatCard, fmtDate, fmtNumber, fmtRelative } from "../_components/admin-ui";

const STATUS_OPTIONS = ["all", "open", "contacted", "resolved", "dismissed"] as const;

export default function AdminPaidQuotaPage() {
  const { getToken } = useAuth();
  const [rows, setRows] = useState<AdminPaidQuotaFollowUpRow[]>([]);
  const [status, setStatus] = useState<(typeof STATUS_OPTIONS)[number]>("open");
  const [loading, setLoading] = useState(true);
  const [savingId, setSavingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const response = await listAdminPaidQuotaFollowUps(token, { status, limit: 200 });
      setRows(response.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load paid quota follow-ups");
    } finally {
      setLoading(false);
    }
  }, [getToken, status]);

  useEffect(() => {
    void load();
  }, [load]);

  const openCount = useMemo(() => rows.filter((row) => row.status === "open").length, [rows]);
  const highestUsage = useMemo(
    () => rows.reduce((highest, row) => Math.max(highest, row.effective_usage), 0),
    [rows],
  );

  async function updateRow(row: AdminPaidQuotaFollowUpRow, nextStatus: AdminPaidQuotaFollowUpRow["status"]) {
    setSavingId(row.id);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      await updateAdminPaidQuotaFollowUp(token, row.id, {
        status: nextStatus,
        assignee_user_id: row.assignee_user_id,
        notes: row.notes,
      });
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to update follow-up");
    } finally {
      setSavingId(null);
    }
  }

  return (
    <AdminShell title="Paid Quota" loading={loading} onRefresh={load}>
      <style>{paidQuotaCss}</style>
      {error ? <div className="apq-error">{error}</div> : null}

      <div className="ad-section-header">
        <div>
          <div className="ad-section-title">120% follow-up queue</div>
          <div className="ad-section-meta">
            Workspaces that crossed the critical paid-plan quota threshold and may need direct outreach.
          </div>
        </div>
        <select value={status} onChange={(event) => setStatus(event.target.value as typeof status)}>
          {STATUS_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All statuses" : `Status: ${value}`}
            </option>
          ))}
        </select>
      </div>

      <div className="ad-stat-grid">
        <StatCard label="Loaded Follow-ups" value={fmtNumber(rows.length)} sub={`Filter: ${status}`} />
        <StatCard label="Open On Page" value={fmtNumber(openCount)} sub="Awaiting review" subColor={openCount > 0 ? "down" : undefined} />
        <StatCard label="Highest Effective Usage" value={fmtNumber(highestUsage)} sub="Completed plus committed schedule" valueColor="accent" />
      </div>

      <div className="ad-tbl-wrap ad-tbl-static apq-table">
        <table>
          <thead>
            <tr>
              <th>Workspace</th>
              <th>Usage</th>
              <th>Breakdown</th>
              <th>Owner</th>
              <th>Created</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {loading && rows.length === 0 ? (
              <tr><td colSpan={6} className="apq-empty">Loading follow-ups...</td></tr>
            ) : rows.length === 0 ? (
              <tr><td colSpan={6} className="apq-empty">No paid quota follow-ups match this filter.</td></tr>
            ) : rows.map((row) => {
              const percentage = row.post_limit > 0
                ? Math.round((row.effective_usage / row.post_limit) * 100)
                : 0;
              return (
                <tr key={row.id}>
                  <td>
                    <div style={{ fontWeight: 650 }}>{row.workspace_name || "Unnamed workspace"}</div>
                    <div className="ad-mono">{row.workspace_id}</div>
                    <div className="ad-mono">{row.plan_id} · {row.period}</div>
                  </td>
                  <td>
                    <div className="apq-usage">{fmtNumber(row.effective_usage)} / {fmtNumber(row.post_limit)}</div>
                    <span className="ad-badge ad-b-red">{percentage}%</span>
                  </td>
                  <td className="ad-mono">
                    <div>published {fmtNumber(row.completed_usage)}</div>
                    <div>scheduled {fmtNumber(row.scheduled_usage)}</div>
                    <div>held {fmtNumber(row.quota_hold_usage)}</div>
                  </td>
                  <td>
                    <div>{row.owner_email || "No owner email"}</div>
                    <div className="ad-mono">{row.owner_user_id || "No owner user"}</div>
                  </td>
                  <td>
                    <div>{fmtRelative(row.created_at)}</div>
                    <div className="ad-mono">{fmtDate(row.created_at)}</div>
                  </td>
                  <td>
                    <select
                      value={row.status}
                      disabled={savingId === row.id}
                      onChange={(event) => void updateRow(row, event.target.value as AdminPaidQuotaFollowUpRow["status"])}
                    >
                      {STATUS_OPTIONS.filter((value) => value !== "all").map((value) => (
                        <option key={value} value={value}>{value}</option>
                      ))}
                    </select>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </AdminShell>
  );
}

const paidQuotaCss = `
.apq-error {
  margin-bottom: 16px;
  padding: 12px 14px;
  border: 1px solid color-mix(in srgb, var(--danger) 22%, transparent);
  border-radius: 8px;
  background: var(--danger-soft);
  color: var(--danger);
  font-size: 13px;
}
.apq-table {
  margin-top: 24px;
}
.apq-empty {
  padding: 28px;
  color: var(--dmuted);
  text-align: center;
}
.apq-usage {
  margin-bottom: 7px;
  font-family: var(--font-geist-mono), monospace;
  font-size: 13px;
  font-weight: 650;
}
`;
