"use client";

import { useAuth } from "@clerk/nextjs";
import {
  AlertTriangle,
  Bug,
  CheckCircle2,
  Mail,
  Play,
  RotateCcw,
  Send,
  XCircle,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import {
  approveAdminErrorTriageBugPlan,
  createAdminErrorTriageRun,
  dismissAdminErrorTriageRecipient,
  getAdminErrorTriageRun,
  listAdminErrorTriageRuns,
  rerunAdminErrorTriageRun,
  sendAdminErrorTriageEmail,
  type ErrorTriageBugPlan,
  type ErrorTriageEmailDraft,
  type ErrorTriageItem,
  type ErrorTriageRunDetail,
  type ErrorTriageRunSummary,
} from "@/lib/api";

import { AdminShell, StatCard, fmtNumber, fmtRelative } from "../_components/admin-ui";

const workflowFinal = new Set(["completed", "dismissed"]);

export default function AdminErrorTriagePage() {
  const { getToken } = useAuth();
  const [runs, setRuns] = useState<ErrorTriageRunSummary[]>([]);
  const [selectedRunId, setSelectedRunId] = useState<string>("");
  const [detail, setDetail] = useState<ErrorTriageRunDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [actionKey, setActionKey] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const loadDetail = useCallback(async (runId: string, tokenOverride?: string) => {
    if (!runId) {
      setDetail(null);
      return;
    }
    setDetailLoading(true);
    try {
      const token = tokenOverride || await getToken();
      if (!token) throw new Error("Not authenticated");
      const res = await getAdminErrorTriageRun(token, runId);
      setDetail(res.data);
      setSelectedRunId(runId);
    } finally {
      setDetailLoading(false);
    }
  }, [getToken]);

  const loadRuns = useCallback(async (preferredRunId?: string) => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const res = await listAdminErrorTriageRuns(token, 30);
      setRuns(res.data);
      const nextRunId = preferredRunId || selectedRunId || res.data[0]?.id || "";
      if (nextRunId) {
        await loadDetail(nextRunId, token);
      } else {
        setDetail(null);
        setSelectedRunId("");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load error triage");
    } finally {
      setLoading(false);
    }
  }, [getToken, loadDetail, selectedRunId]);

  useEffect(() => {
    loadRuns();
  }, [loadRuns]);

  const activeItems = detail?.items || [];
  const latestRun = runs[0] || null;
  const emailRecipientCount = useMemo(
    () => activeItems.reduce((total, item) => total + (item.recipients?.length || 0), 0),
    [activeItems],
  );
  const pendingRecipientCount = useMemo(
    () => activeItems.reduce((total, item) => total + (item.recipients || []).filter((r) => r.status === "pending" || r.status === "send_failed").length, 0),
    [activeItems],
  );
  const bugPlanCount = useMemo(() => activeItems.filter((item) => item.action_kind === "bug_plan").length, [activeItems]);
  const needsReviewCount = useMemo(
    () => activeItems.filter((item) => item.workflow_status === "pending_review" || item.classification === "needs_human_review").length,
    [activeItems],
  );

  async function withAction(key: string, fn: () => Promise<void>) {
    setActionKey(key);
    setError(null);
    setNotice(null);
    try {
      await fn();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Action failed");
    } finally {
      setActionKey("");
    }
  }

  async function handleRunNow() {
    await withAction("run-now", async () => {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const res = await createAdminErrorTriageRun(token);
      setNotice("Run queued and completed.");
      await loadRuns(res.data.id);
    });
  }

  async function handleRerun(runId: string) {
    await withAction(`rerun:${runId}`, async () => {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const res = await rerunAdminErrorTriageRun(token, runId);
      setNotice("Run re-created for the same window.");
      await loadRuns(res.data.id);
    });
  }

  async function handleApproveBugPlan(item: ErrorTriageItem) {
    await withAction(`approve:${item.id}`, async () => {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      await approveAdminErrorTriageBugPlan(token, item.id, { admin_notes: "Bug plan approved from admin triage." });
      setNotice("Bug plan approved.");
      await loadDetail(selectedRunId, token);
      await loadRuns(selectedRunId);
    });
  }

  async function handleSend(item: ErrorTriageItem, recipientId: string) {
    await withAction(`send:${recipientId}`, async () => {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      await sendAdminErrorTriageEmail(token, item.id, recipientId);
      setNotice("Email sent.");
      await loadDetail(selectedRunId, token);
      await loadRuns(selectedRunId);
    });
  }

  async function handleDismiss(item: ErrorTriageItem, recipientId: string) {
    await withAction(`dismiss:${recipientId}`, async () => {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      await dismissAdminErrorTriageRecipient(token, item.id, recipientId, "Dismissed from admin triage.");
      setNotice("Recipient dismissed.");
      await loadDetail(selectedRunId, token);
      await loadRuns(selectedRunId);
    });
  }

  return (
    <AdminShell title="Error Triage" loading={loading || detailLoading} onRefresh={() => loadRuns(selectedRunId)}>
      <style>{triageCss}</style>

      {error ? (
        <div className="triage-alert triage-alert-error">
          <AlertTriangle strokeWidth={1.75} />
          <span>{error}</span>
        </div>
      ) : null}
      {notice ? (
        <div className="triage-alert triage-alert-ok">
          <CheckCircle2 strokeWidth={1.75} />
          <span>{notice}</span>
        </div>
      ) : null}

      <div className="ad-section-header">
        <div>
          <div className="ad-section-title">Daily triage runs</div>
          <div className="ad-section-meta">
            {latestRun ? `Latest ${fmtRelative(latestRun.created_at)} · ${formatWindow(latestRun)}` : "No runs recorded"}
          </div>
        </div>
        <div className="triage-actions">
          {selectedRunId ? (
            <button
              type="button"
              className="ad-btn ad-btn-ghost"
              onClick={() => handleRerun(selectedRunId)}
              disabled={!!actionKey || !selectedRunId}
            >
              <RotateCcw strokeWidth={1.75} />
              Re-run
            </button>
          ) : null}
          <button
            type="button"
            className="ad-btn triage-primary-btn"
            onClick={handleRunNow}
            disabled={!!actionKey}
          >
            <Play strokeWidth={1.75} />
            Run now
          </button>
        </div>
      </div>

      <div className="ad-stat-grid">
        <StatCard label="Failures" value={fmtNumber(detail?.run.failures_analyzed || 0)} sub={detail ? labelize(detail.run.health_status) : "selected run"} />
        <StatCard label="Items" value={fmtNumber(activeItems.length)} sub={`${fmtNumber(needsReviewCount)} need review`} />
        <StatCard label="Email Recipients" value={fmtNumber(emailRecipientCount)} sub={`${fmtNumber(pendingRecipientCount)} pending`} />
        <StatCard label="Bug Plans" value={fmtNumber(bugPlanCount)} sub="platform fixes" valueColor={bugPlanCount > 0 ? "accent" : undefined} />
      </div>

      <div className="triage-grid">
        <section className="triage-runs-panel">
          <div className="triage-panel-head">
            <span>Runs</span>
            <span className="ad-mono">{fmtNumber(runs.length)}</span>
          </div>
          <div className="triage-run-list">
            {loading && runs.length === 0 ? (
              <div className="triage-empty">Loading runs...</div>
            ) : runs.length === 0 ? (
              <div className="triage-empty">No triage runs yet.</div>
            ) : (
              runs.map((run) => (
                <button
                  key={run.id}
                  type="button"
                  className="triage-run-row"
                  data-active={run.id === selectedRunId}
                  onClick={() => loadDetail(run.id)}
                >
                  <span className="triage-run-top">
                    <span className="triage-run-window">{formatWindow(run)}</span>
                    <StatusBadge status={run.health_status} />
                  </span>
                  <span className="triage-run-bottom">
                    <span>{run.run_type}</span>
                    <span>{fmtNumber(run.items_total)} items</span>
                    <span>{fmtRelative(run.created_at)}</span>
                  </span>
                </button>
              ))
            )}
          </div>
        </section>

        <section className="triage-detail">
          {detailLoading && !detail ? (
            <div className="triage-empty triage-empty-wide">Loading run...</div>
          ) : !detail ? (
            <div className="triage-empty triage-empty-wide">Select a run.</div>
          ) : (
            <>
              <div className="triage-run-summary">
                <div>
                  <div className="triage-summary-title">{detail.run.summary || "Run summary unavailable"}</div>
                  <div className="ad-mono">
                    {formatDateTime(detail.run.window_start)} - {formatDateTime(detail.run.window_end)}
                  </div>
                </div>
                <StatusBadge status={detail.run.status === "failed" ? "needs_review" : detail.run.health_status} />
              </div>

              <div className="triage-items">
                {detail.items.length === 0 ? (
                  <div className="triage-empty triage-empty-wide">No actionable items in this run.</div>
                ) : (
                  detail.items.map((item) => (
                    <TriageItem
                      key={item.id}
                      item={item}
                      busyKey={actionKey}
                      onApprove={() => handleApproveBugPlan(item)}
                      onSend={(recipientId) => handleSend(item, recipientId)}
                      onDismiss={(recipientId) => handleDismiss(item, recipientId)}
                    />
                  ))
                )}
              </div>
            </>
          )}
        </section>
      </div>
    </AdminShell>
  );
}

function TriageItem({
  item,
  busyKey,
  onApprove,
  onSend,
  onDismiss,
}: {
  item: ErrorTriageItem;
  busyKey: string;
  onApprove: () => void;
  onSend: (recipientId: string) => void;
  onDismiss: (recipientId: string) => void;
}) {
  const draft = item.email_draft_json;
  const bugPlan = item.bug_plan_json;
  const hasEmail = hasEmailDraft(draft);
  const hasBug = hasBugPlan(bugPlan);
  const sendableWorkflow = item.workflow_status === "ready" || item.workflow_status === "partially_completed";

  return (
    <article className="triage-item">
      <div className="triage-item-head">
        <div>
          <div className="triage-item-meta">
            <span className={`ad-badge ${classificationClass(item.classification)}`}>{labelize(item.classification)}</span>
            <span className="ad-badge ad-b-gray">{labelize(item.action_kind)}</span>
            <span className="ad-badge ad-b-gray">{labelize(item.workflow_status)}</span>
            {item.duplicate_of_item_id ? <span className="ad-badge ad-b-gray">duplicate</span> : null}
          </div>
          <div className="triage-item-title">{item.ai_summary || "No summary recorded."}</div>
        </div>
        <div className="triage-confidence">
          <span>{Math.round((item.confidence || 0) * 100)}%</span>
          <small>confidence</small>
        </div>
      </div>

      <div className="triage-facts">
        <Fact label="Platform" value={item.platform || "parent"} />
        <Fact label="Source" value={item.source || "-"} />
        <Fact label="Code" value={item.platform_error_code || item.error_code || "-"} />
        <Fact label="Stage" value={item.failure_stage || "-"} />
        <Fact label="Users" value={fmtNumber(item.affected_user_count)} />
        <Fact label="Posts" value={fmtNumber(item.affected_post_count)} />
      </div>

      {hasBug && bugPlan ? (
        <div className="triage-subsection">
          <div className="triage-subhead">
            <Bug strokeWidth={1.75} />
            <span>{bugPlan.title || "Bug plan"}</span>
          </div>
          <PlanRow label="Impact" value={bugPlan.impact} />
          <PlanRow label="Area" value={bugPlan.suspected_area} />
          <PlanRow label="Fix" value={bugPlan.proposed_fix} />
          <PlanRow label="Validation" value={bugPlan.validation_plan} />
          <PlanRow label="Rollback" value={bugPlan.rollback_plan} />
          {bugPlan.evidence?.length ? (
            <div className="triage-evidence-list">
              {bugPlan.evidence.map((line, index) => <span key={`${line}-${index}`}>{line}</span>)}
            </div>
          ) : null}
          {!workflowFinal.has(item.workflow_status) ? (
            <button type="button" className="ad-btn triage-primary-btn" onClick={onApprove} disabled={busyKey === `approve:${item.id}` || !!busyKey}>
              <CheckCircle2 strokeWidth={1.75} />
              Approve
            </button>
          ) : null}
        </div>
      ) : null}

      {hasEmail && draft ? (
        <div className="triage-subsection">
          <div className="triage-subhead">
            <Mail strokeWidth={1.75} />
            <span>{draft.subject || "Email draft"}</span>
          </div>
          <div className="triage-email-body">{draft.body}</div>
          <div className="triage-recipient-list">
            {(item.recipients || []).map((recipient) => {
              const canSend = sendableWorkflow && (recipient.status === "pending" || recipient.status === "send_failed");
              return (
                <div key={recipient.id} className="triage-recipient-row">
                  <div className="triage-recipient-main">
                    <span>{recipient.current_email || recipient.email_snapshot}</span>
                    <small>{recipient.status === "send_failed" ? "send failed" : labelize(recipient.status)}</small>
                  </div>
                  <div className="triage-recipient-actions">
                    <button
                      type="button"
                      className="ad-btn ad-btn-ghost"
                      onClick={() => onDismiss(recipient.id)}
                      disabled={!canSend || busyKey === `dismiss:${recipient.id}` || !!busyKey}
                    >
                      <XCircle strokeWidth={1.75} />
                      Dismiss
                    </button>
                    <button
                      type="button"
                      className="ad-btn triage-primary-btn"
                      onClick={() => onSend(recipient.id)}
                      disabled={!canSend || busyKey === `send:${recipient.id}` || !!busyKey}
                    >
                      <Send strokeWidth={1.75} />
                      Send
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      ) : null}
    </article>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="triage-fact">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function PlanRow({ label, value }: { label: string; value?: string }) {
  if (!value) return null;
  return (
    <div className="triage-plan-row">
      <span>{label}</span>
      <p>{value}</p>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const cls = status === "needs_review" || status === "failed"
    ? "ad-badge triage-b-danger"
    : status === "actionable_items" || status === "running"
      ? "ad-badge triage-b-warn"
      : "ad-badge triage-b-ok";
  return <span className={cls}>{labelize(status)}</span>;
}

function hasBugPlan(plan: ErrorTriageBugPlan | null | undefined) {
  if (!plan) return false;
  return Boolean(plan.title || plan.impact || plan.proposed_fix || plan.validation_plan || plan.rollback_plan || plan.evidence?.length);
}

function hasEmailDraft(draft: ErrorTriageEmailDraft | null | undefined) {
  if (!draft) return false;
  return Boolean(draft.subject || draft.body);
}

function classificationClass(classification: string) {
  if (classification === "unipost_bug") return "triage-b-danger";
  if (classification === "user_action_needed") return "triage-b-warn";
  if (classification === "transient_no_action") return "triage-b-ok";
  return "ad-b-gray";
}

function labelize(value: string) {
  return value.replaceAll("_", " ");
}

function formatWindow(run: ErrorTriageRunSummary) {
  const start = new Date(run.window_start);
  const end = new Date(run.window_end);
  const opts: Intl.DateTimeFormatOptions = { month: "short", day: "numeric" };
  return `${start.toLocaleDateString("en-US", opts)} to ${end.toLocaleDateString("en-US", opts)}`;
}

function formatDateTime(value: string) {
  return new Date(value).toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

const triageCss = `
.triage-alert {
  display: flex;
  align-items: center;
  gap: 8px;
  border-radius: 8px;
  padding: 10px 12px;
  margin-bottom: 14px;
  font-size: 12.5px;
  border: 1px solid var(--dborder);
}
.triage-alert svg { width: 16px; height: 16px; flex-shrink: 0; }
.triage-alert-error {
  background: var(--danger-soft);
  border-color: color-mix(in srgb, var(--danger) 22%, transparent);
  color: var(--danger);
}
.triage-alert-ok {
  background: var(--success-soft);
  border-color: color-mix(in srgb, var(--success) 22%, transparent);
  color: var(--success);
}
.triage-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.triage-actions svg,
.triage-primary-btn svg,
.triage-subhead svg,
.triage-recipient-actions svg {
  width: 14px;
  height: 14px;
}
.triage-primary-btn {
  background: var(--daccent);
  color: var(--primary-foreground);
  border-color: color-mix(in srgb, var(--daccent) 82%, transparent);
}
.triage-primary-btn:hover:not(:disabled) {
  background: color-mix(in srgb, var(--daccent) 88%, var(--dtext));
}
.triage-grid {
  display: grid;
  grid-template-columns: minmax(280px, 360px) minmax(0, 1fr);
  gap: 14px;
  align-items: start;
}
.triage-runs-panel {
  border: 1px solid var(--dborder);
  border-radius: 8px;
  background: var(--surface);
  overflow: hidden;
}
.triage-panel-head {
  height: 38px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 12px;
  border-bottom: 1px solid var(--dborder);
  background: var(--surface2);
  font-size: 12px;
  font-weight: 600;
}
.triage-run-list {
  display: grid;
  max-height: calc(100dvh - 260px);
  overflow-y: auto;
}
.triage-run-row {
  display: grid;
  gap: 7px;
  padding: 12px;
  border: 0;
  border-bottom: 1px solid var(--dborder);
  background: transparent;
  color: inherit;
  text-align: left;
  font-family: inherit;
  cursor: pointer;
}
.triage-run-row:hover {
  background: var(--surface2);
}
.triage-run-row[data-active="true"] {
  background: color-mix(in srgb, var(--accent-dim) 52%, transparent);
  box-shadow: inset 3px 0 0 var(--daccent);
}
.triage-run-top,
.triage-run-bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.triage-run-bottom {
  color: var(--dmuted);
  font-size: 11px;
  font-family: var(--font-geist-mono), monospace;
}
.triage-run-window {
  font-size: 12.5px;
  font-weight: 600;
}
.triage-detail {
  min-width: 0;
}
.triage-run-summary {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  border-top: 1px solid var(--dborder);
  border-bottom: 1px solid var(--dborder);
  padding: 13px 0;
  margin-bottom: 12px;
}
.triage-summary-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--dtext);
  margin-bottom: 3px;
}
.triage-items {
  display: grid;
  gap: 12px;
}
.triage-item {
  border: 1px solid var(--dborder);
  border-radius: 8px;
  background: var(--surface);
  padding: 14px;
}
.triage-item-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}
.triage-item-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-bottom: 8px;
}
.triage-item-title {
  font-size: 13px;
  font-weight: 560;
  color: var(--dtext);
}
.triage-confidence {
  display: grid;
  justify-items: end;
  flex-shrink: 0;
  font-family: var(--font-geist-mono), monospace;
}
.triage-confidence span {
  font-size: 20px;
  font-weight: 700;
}
.triage-confidence small {
  font-size: 10px;
  color: var(--dmuted);
}
.triage-facts {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 8px;
  margin: 13px 0 8px;
}
.triage-fact {
  display: grid;
  gap: 2px;
  min-width: 0;
  border-top: 1px solid var(--dborder);
  padding-top: 7px;
}
.triage-fact span {
  color: var(--dmuted);
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: .06em;
}
.triage-fact strong {
  color: var(--dtext);
  font-family: var(--font-geist-mono), monospace;
  font-size: 11px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.triage-subsection {
  border-top: 1px solid var(--dborder);
  margin-top: 12px;
  padding-top: 12px;
}
.triage-subhead {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 12.5px;
  font-weight: 650;
  margin-bottom: 8px;
}
.triage-plan-row {
  display: grid;
  grid-template-columns: 86px minmax(0, 1fr);
  gap: 10px;
  padding: 6px 0;
  border-top: 1px solid color-mix(in srgb, var(--dborder) 70%, transparent);
}
.triage-plan-row:first-of-type {
  border-top: 0;
}
.triage-plan-row span {
  color: var(--dmuted);
  font-size: 11px;
}
.triage-plan-row p {
  margin: 0;
  color: var(--dtext);
  font-size: 12px;
  line-height: 1.5;
}
.triage-evidence-list {
  display: grid;
  gap: 4px;
  margin: 7px 0 10px;
}
.triage-evidence-list span {
  color: var(--dmuted);
  font-size: 11.5px;
  font-family: var(--font-geist-mono), monospace;
}
.triage-email-body {
  white-space: pre-wrap;
  color: var(--dtext);
  background: var(--surface2);
  border: 1px solid var(--dborder);
  border-radius: 6px;
  padding: 10px 11px;
  font-size: 12px;
  line-height: 1.55;
}
.triage-recipient-list {
  display: grid;
  gap: 0;
  margin-top: 10px;
  border-top: 1px solid var(--dborder);
}
.triage-recipient-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 9px 0;
  border-bottom: 1px solid var(--dborder);
}
.triage-recipient-main {
  display: grid;
  min-width: 0;
}
.triage-recipient-main span {
  color: var(--dtext);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.triage-recipient-main small {
  color: var(--dmuted);
  font-size: 11px;
  font-family: var(--font-geist-mono), monospace;
}
.triage-recipient-actions {
  display: flex;
  align-items: center;
  gap: 7px;
  flex-shrink: 0;
}
.triage-b-danger {
  background: var(--danger-soft);
  color: var(--danger);
  border: 1px solid color-mix(in srgb, var(--danger) 22%, transparent);
}
.triage-b-warn {
  background: var(--warning-soft);
  color: var(--warning);
  border: 1px solid color-mix(in srgb, var(--warning) 24%, transparent);
}
.triage-b-ok {
  background: var(--success-soft);
  color: var(--success);
  border: 1px solid color-mix(in srgb, var(--success) 22%, transparent);
}
.triage-empty {
  padding: 18px 12px;
  color: var(--dmuted);
  font-size: 12px;
  text-align: center;
}
.triage-empty-wide {
  border: 1px solid var(--dborder);
  border-radius: 8px;
  background: var(--surface);
}
@media (max-width: 1080px) {
  .triage-facts {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
@media (max-width: 860px) {
  .triage-grid {
    grid-template-columns: 1fr;
  }
  .triage-run-list {
    max-height: 320px;
  }
  .triage-item-head,
  .triage-run-summary,
  .triage-recipient-row {
    align-items: flex-start;
    flex-direction: column;
  }
  .triage-confidence {
    justify-items: start;
  }
  .triage-recipient-actions {
    width: 100%;
    justify-content: flex-end;
    flex-wrap: wrap;
  }
}
@media (max-width: 560px) {
  .triage-facts {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .triage-plan-row {
    grid-template-columns: 1fr;
    gap: 2px;
  }
}
`;
