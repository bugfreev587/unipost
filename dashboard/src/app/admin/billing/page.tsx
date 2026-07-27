"use client";

import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  grantAdminTrial,
  listAdminBilling,
  revokeAdminTrial,
  setAdminWorkspacePlan,
  type AdminBillingListParams,
  type AdminBillingRow,
  type TrialStatus,
} from "@/lib/api";
import { formatPostUsage, usagePercentage } from "@/lib/billing-format";
import { formatWorkspaceTrial } from "@/lib/trial-format";
import { ConfirmModal } from "@/components/confirm-modal";

import { AdminShell, StatCard, fmtCents, fmtNumber, fmtRelative } from "../_components/admin-ui";

const STATUS_OPTIONS = ["all", "active", "past_due", "canceled", "trialing"] as const;
const PLAN_OPTIONS = ["all", "free", "api", "basic", "growth", "team", "enterprise"] as const;
const PLAN_FLIP_OPTIONS = ["free", "api", "basic", "growth", "team", "enterprise"] as const;
const DAY_OPTIONS = [30, 90, 180] as const;
const TRIAL_PLAN_OPTIONS = [
  { id: "api", label: "API" },
  { id: "basic", label: "Basic" },
  { id: "growth", label: "Growth" },
  { id: "team", label: "Team" },
] as const;
const OPEN_TRIAL_STATUSES = new Set<TrialStatus>(["provisioning", "pending_activation", "checkout_pending", "scheduled", "active"]);
const REVOCABLE_TRIAL_STATUSES = new Set<TrialStatus>(["pending_activation", "checkout_pending"]);
const TRIAL_STATUS_LABELS: Record<TrialStatus, string> = {
  provisioning: "Provisioning",
  pending_activation: "Pending activation",
  checkout_pending: "Checkout pending",
  scheduled: "Scheduled",
  active: "Active",
  completed: "Completed",
  canceled: "Canceled",
  revoked: "Revoked",
  superseded: "Superseded",
  failed: "Failed",
};
type WorkspaceMutationState = "plan" | "trial" | "refresh_required";
type BillingLoadOptions = {
  clearRefreshRequiredFor?: string;
  clearAllRefreshRequired?: boolean;
  mutationEpoch?: number;
};
type TrialConfirmation =
  | {
      kind: "grant";
      planId: string;
      durationDays: number;
      timeline: Array<{ label: string; value: string }>;
    }
  | {
      kind: "revoke";
      grantId: string;
      planId: string;
    };
const REFRESH_REQUIRED_MESSAGE = "The trial was updated, but Billing could not refresh. Refresh Billing before making another change.";
const PLAN_REFRESH_REQUIRED_MESSAGE = "Plan changed, but Billing could not refresh. Refresh Billing before making another change.";
const SUPERSEDED_LOAD_MESSAGE = "Billing request superseded by a newer refresh.";

function isCurrentAdminBillingLoad(
  requestSequence: number,
  latestRequestSequence: number,
  requestMutationEpoch: number,
  currentMutationEpoch: number,
): boolean {
  return requestSequence === latestRequestSequence && requestMutationEpoch === currentMutationEpoch;
}

function reconcileRefreshRequiredLocks(
  current: Record<string, WorkspaceMutationState>,
  options: BillingLoadOptions,
  workspaceMutationEpochs: Record<string, number>,
): Record<string, WorkspaceMutationState> {
  if (options.clearAllRefreshRequired) {
    return Object.fromEntries(Object.entries(current).filter(([, mutation]) => mutation !== "refresh_required"));
  }

  const workspaceId = options.clearRefreshRequiredFor;
  if (
    !workspaceId
    || options.mutationEpoch === undefined
    || workspaceMutationEpochs[workspaceId] !== options.mutationEpoch
    || current[workspaceId] !== "refresh_required"
  ) {
    return current;
  }
  const next = { ...current };
  delete next[workspaceId];
  return next;
}

export default function AdminBillingPage() {
  const { getToken } = useAuth();
  const [rows, setRows] = useState<AdminBillingRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [workspaceMutations, setWorkspaceMutations] = useState<Record<string, WorkspaceMutationState>>({});
  const loadRequestSequence = useRef(0);
  const billingMutationEpoch = useRef(0);
  const workspaceMutationEpochs = useRef<Record<string, number>>({});

  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState<(typeof STATUS_OPTIONS)[number]>("all");
  const [plan, setPlan] = useState<(typeof PLAN_OPTIONS)[number]>("all");
  const [days, setDays] = useState<(typeof DAY_OPTIONS)[number]>(90);
  const limit = 100;

  const setWorkspaceMutation = useCallback((workspaceId: string, state: WorkspaceMutationState | null): number => {
    if (state === "plan" || state === "trial") {
      billingMutationEpoch.current += 1;
      workspaceMutationEpochs.current[workspaceId] = (workspaceMutationEpochs.current[workspaceId] ?? 0) + 1;
    }
    setWorkspaceMutations((current) => {
      if (state) return { ...current, [workspaceId]: state };
      if (!(workspaceId in current)) return current;
      const next = { ...current };
      delete next[workspaceId];
      return next;
    });
    return workspaceMutationEpochs.current[workspaceId] ?? 0;
  }, []);

  const loadBilling = useCallback(async (options: BillingLoadOptions = {}) => {
    const requestSequence = ++loadRequestSequence.current;
    const requestMutationEpoch = billingMutationEpoch.current;
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
      if (!isCurrentAdminBillingLoad(
        requestSequence,
        loadRequestSequence.current,
        requestMutationEpoch,
        billingMutationEpoch.current,
      )) {
        throw new Error(SUPERSEDED_LOAD_MESSAGE);
      }
      setRows(res.data);
      setWorkspaceMutations((current) => reconcileRefreshRequiredLocks(current, options, workspaceMutationEpochs.current));
    } catch (e) {
      const loadError = e instanceof Error ? e : new Error("Failed to load");
      if (isCurrentAdminBillingLoad(
        requestSequence,
        loadRequestSequence.current,
        requestMutationEpoch,
        billingMutationEpoch.current,
      )) {
        setError(loadError.message);
      }
      throw loadError;
    } finally {
      if (requestSequence === loadRequestSequence.current) setLoading(false);
    }
  }, [days, getToken, plan, search, status]);

  const refreshBilling = useCallback(() => {
    void loadBilling({ clearAllRefreshRequired: true }).catch(() => undefined);
  }, [loadBilling]);

  useEffect(() => {
    void loadBilling().catch(() => undefined);
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
    <AdminShell title="Billing" loading={loading} onRefresh={refreshBilling}>
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
              <th>Trial</th>
              <th>Status</th>
              <th>Usage</th>
              <th>Renewal</th>
              <th>Stripe</th>
              <th>Updated</th>
            </tr>
          </thead>
          <tbody>
            {loading && rows.length === 0 ? (
              <tr><td colSpan={9} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>Loading…</td></tr>
            ) : rows.length === 0 ? (
              <tr><td colSpan={9} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>No billing rows matched the current filters.</td></tr>
            ) : (
              rows.map((row) => {
                const usagePct = usagePercentage(row.posts_used, row.post_limit);
                const usageClass = usagePct >= 90 ? "ad-uf-r" : usagePct >= 70 ? "ad-uf-a" : "ad-uf-g";
                const hasOpenTrial = isOpenTrial(row.trial);
                const workspaceMutation = workspaceMutations[row.workspace_id];
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
                      <PlanFlipMenu
                        workspaceId={row.workspace_id}
                        currentPlan={row.plan_id}
                        hasOpenTrial={hasOpenTrial}
                        workspaceMutation={workspaceMutation}
                        setWorkspaceMutation={setWorkspaceMutation}
                        onChanged={(mutationEpoch) => loadBilling({ clearRefreshRequiredFor: row.workspace_id, mutationEpoch })}
                      />
                    </td>
                    <td className="abt-trial-cell">
                      <GrantTrialForm
                        row={row}
                        workspaceMutation={workspaceMutation}
                        setWorkspaceMutation={setWorkspaceMutation}
                        onChanged={(mutationEpoch) => loadBilling({ clearRefreshRequiredFor: row.workspace_id, mutationEpoch })}
                      />
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
                      </div>
                    </td>
                    <td>
                      <div style={{ fontSize: 11.5 }}>{formatPostUsage(row.posts_used, row.post_limit)}</div>
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
      <style jsx global>{`
        .abt-trial-cell { min-width: 292px; vertical-align: top; }
        .abt-form { display: grid; gap: 8px; min-width: 268px; }
        .abt-fields { display: grid; grid-template-columns: minmax(118px, 1fr) 82px; gap: 8px; align-items: end; }
        .abt-field { display: grid; gap: 4px; min-width: 0; }
        .abt-label { color: var(--dmuted); font-size: 10px; font-weight: 600; letter-spacing: .04em; text-transform: uppercase; }
        .abt-control { width: 100%; min-height: 30px; border: 1px solid var(--dborder2); border-radius: 6px; background: var(--surface2); color: var(--dtext); font: inherit; font-size: 12px; padding: 5px 8px; outline: none; }
        .abt-control:focus-visible, .abt-button:focus-visible { border-color: color-mix(in srgb, var(--primary) 42%, transparent); box-shadow: 0 0 0 3px var(--focus-ring); }
        .abt-readonly { display: flex; align-items: center; font-weight: 600; text-transform: capitalize; }
        .abt-helper { color: var(--dmuted2); font-size: 10px; line-height: 1.35; }
        .abt-timeline { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 3px 8px; margin: 0; padding-top: 7px; border-top: 1px solid var(--dborder); }
        .abt-timeline dt { color: var(--dmuted2); font-size: 10px; }
        .abt-timeline dd { color: var(--dmuted); font-family: var(--font-geist-mono), monospace; font-size: 10px; margin: 0; min-width: 0; }
        .abt-actions { display: flex; flex-wrap: wrap; gap: 6px; }
        .abt-button { min-height: 30px; border: 1px solid var(--dborder2); border-radius: 6px; background: var(--surface2); color: var(--dtext); cursor: pointer; font: inherit; font-size: 11px; font-weight: 600; padding: 5px 10px; transition: transform .18s cubic-bezier(.16, 1, .3, 1), background-color .18s cubic-bezier(.16, 1, .3, 1); }
        .abt-button:hover:not(:disabled) { background: var(--surface3); }
        .abt-button:active:not(:disabled) { transform: translateY(1px); }
        .abt-button:disabled { cursor: not-allowed; opacity: .45; }
        .abt-button-primary { border-color: transparent; background: var(--daccent); color: var(--primary-foreground); }
        .abt-button-primary:hover:not(:disabled) { background: color-mix(in srgb, var(--daccent) 88%, var(--dtext)); }
        .abt-button-danger { background: var(--danger-soft); border-color: color-mix(in srgb, var(--danger) 20%, transparent); color: var(--danger); }
        .abt-current { display: grid; gap: 7px; }
        .abt-current-head { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; }
        .abt-current-title { color: var(--dtext); font-size: 11px; font-weight: 600; }
        .abt-badge { display: inline-flex; border: 1px solid var(--dborder2); border-radius: 999px; font-family: var(--font-geist-mono), monospace; font-size: 10px; font-weight: 600; padding: 2px 7px; }
        .abt-tone-neutral { background: var(--surface2); color: var(--dmuted); }
        .abt-tone-info { background: var(--info-soft); border-color: color-mix(in srgb, var(--info) 22%, transparent); color: var(--info); }
        .abt-tone-success { background: var(--success-soft); border-color: color-mix(in srgb, var(--success) 20%, transparent); color: var(--success); }
        .abt-tone-warning { background: color-mix(in srgb, #f59e0b 10%, transparent); border-color: color-mix(in srgb, #f59e0b 24%, transparent); color: #b45309; }
        .abt-tone-danger { background: var(--danger-soft); border-color: color-mix(in srgb, var(--danger) 20%, transparent); color: var(--danger); }
        .abt-message { border-left: 2px solid currentColor; font-size: 10.5px; line-height: 1.4; padding-left: 7px; }
        .abt-error { color: var(--danger); }
        .abt-success { color: var(--success); }
        @media (max-width: 860px) {
          .abt-trial-cell { min-width: 276px; }
          .abt-form { min-width: 252px; }
        }
      `}</style>
    </AdminShell>
  );
}

function trialPlanLabel(planId: string): string {
  return TRIAL_PLAN_OPTIONS.find((plan) => plan.id === planId)?.label ?? planId;
}

function isOpenTrial(trial?: AdminBillingRow["trial"]): boolean {
  return !!trial && OPEN_TRIAL_STATUSES.has(trial.status);
}

function formatTrialDate(value: Date | string): string {
  const date = value instanceof Date ? value : new Date(value);
  if (!Number.isFinite(date.getTime())) return "Date unavailable";
  return new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric", year: "numeric" }).format(date);
}

function proposedTrialTimeline(currentPlan: string, durationDays: number, currentPeriodEnd?: string) {
  if (currentPlan === "free") {
    return [
      { label: "Trial starts", value: "When the user completes checkout" },
      { label: "Trial ends", value: `${durationDays} day${durationDays === 1 ? "" : "s"} after checkout` },
      { label: "Billing begins", value: "At trial end unless canceled" },
    ];
  }

  const startsAt = currentPeriodEnd ? new Date(currentPeriodEnd) : null;
  const validStart = startsAt && Number.isFinite(startsAt.getTime()) ? startsAt : null;
  const endsAt = validStart ? new Date(validStart.getTime() + durationDays * 24 * 60 * 60 * 1000) : null;
  return [
    { label: "Trial starts", value: validStart ? formatTrialDate(validStart) : "After the current billing period" },
    { label: "Trial ends", value: endsAt ? formatTrialDate(endsAt) : `${durationDays} days after it starts` },
    { label: "Billing resumes", value: `On ${trialPlanLabel(currentPlan)} at trial end` },
  ];
}

function TrialTimeline({ items }: { items: { label: string; value: string }[] }) {
  return (
    <dl className="abt-timeline">
      {items.map((item) => (
        <div key={`${item.label}:${item.value}`} style={{ display: "contents" }}>
          <dt>{item.label}</dt>
          <dd>{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}

function GrantTrialForm({
  row,
  workspaceMutation,
  setWorkspaceMutation,
  onChanged,
}: {
  row: AdminBillingRow;
  workspaceMutation?: WorkspaceMutationState;
  setWorkspaceMutation: (workspaceId: string, state: WorkspaceMutationState | null) => number;
  onChanged: (mutationEpoch: number) => Promise<void>;
}) {
  const { getToken } = useAuth();
  const [targetPlan, setTargetPlan] = useState<(typeof TRIAL_PLAN_OPTIONS)[number]["id"]>("basic");
  const [durationDays, setDurationDays] = useState("30");
  const [busy, setBusy] = useState<"grant" | "revoke" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [confirmation, setConfirmation] = useState<TrialConfirmation | null>(null);

  const currentTrial = row.trial;
  const hasOpenTrial = isOpenTrial(currentTrial);
  const canRevoke = !!currentTrial && REVOCABLE_TRIAL_STATUSES.has(currentTrial.status);
  const interactionLocked = !!busy || !!workspaceMutation;
  const grantPlanIsEligible = row.plan_id === "free" || TRIAL_PLAN_OPTIONS.some((plan) => plan.id === row.plan_id);
  const selectedPlan = row.plan_id === "free" ? targetPlan : row.plan_id;
  const parsedDays = Number(durationDays);
  const validDays = Number.isInteger(parsedDays) && parsedDays >= 1 && parsedDays <= 730;
  const durationError = durationDays.trim() === "" ? "Days are required." : validDays ? null : "Use 1–730 whole days.";
  const proposal = proposedTrialTimeline(row.plan_id, validDays ? parsedDays : 0, row.current_period_end);
  const formattedTrial = currentTrial ? formatWorkspaceTrial(currentTrial) : null;
  const visibleError = error === REFRESH_REQUIRED_MESSAGE && workspaceMutation !== "refresh_required" ? null : error;

  const requestGrant = () => {
    if (busy || hasOpenTrial || workspaceMutation) return;
    setError(null);
    setSuccess(null);

    if (!validDays) {
      setError("Enter a whole number of days from 1 to 730.");
      return;
    }
    if (!TRIAL_PLAN_OPTIONS.some((plan) => plan.id === selectedPlan)) {
      setError("Trials are available only for API, Basic, Growth, and Team plans.");
      return;
    }

    setConfirmation({
      kind: "grant",
      planId: selectedPlan,
      durationDays: parsedDays,
      timeline: proposal,
    });
  };

  const grant = async (request: Extract<TrialConfirmation, { kind: "grant" }>) => {
    if (busy || hasOpenTrial || workspaceMutation) return;
    setBusy("grant");
    const mutationEpoch = setWorkspaceMutation(row.workspace_id, "trial");
    let mutationSucceeded = false;
    let keepLocked = false;
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const response = await grantAdminTrial(token, row.workspace_id, {
        plan_id: request.planId,
        duration_days: request.durationDays,
      });
      mutationSucceeded = true;
      await onChanged(mutationEpoch);
      setSuccess(`${TRIAL_STATUS_LABELS[response.data.status]} trial recorded.`);
    } catch (e) {
      if (mutationSucceeded) {
        keepLocked = true;
        setWorkspaceMutation(row.workspace_id, "refresh_required");
        setError(REFRESH_REQUIRED_MESSAGE);
      } else {
        setError(e instanceof Error ? e.message : "Failed to grant trial");
      }
    } finally {
      setBusy(null);
      if (!keepLocked) setWorkspaceMutation(row.workspace_id, null);
    }
  };

  const requestRevoke = () => {
    if (!currentTrial || !canRevoke || busy || workspaceMutation) return;
    setError(null);
    setSuccess(null);
    setConfirmation({
      kind: "revoke",
      grantId: currentTrial.id,
      planId: currentTrial.plan_id,
    });
  };

  const revoke = async (request: Extract<TrialConfirmation, { kind: "revoke" }>) => {
    if (busy || workspaceMutation) return;
    setBusy("revoke");
    const mutationEpoch = setWorkspaceMutation(row.workspace_id, "trial");
    setError(null);
    setSuccess(null);
    let mutationSucceeded = false;
    let keepLocked = false;
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      await revokeAdminTrial(token, row.workspace_id, request.grantId);
      mutationSucceeded = true;
      await onChanged(mutationEpoch);
      setSuccess("Trial offer revoked.");
    } catch (e) {
      if (mutationSucceeded) {
        keepLocked = true;
        setWorkspaceMutation(row.workspace_id, "refresh_required");
        setError(REFRESH_REQUIRED_MESSAGE);
      } else {
        setError(e instanceof Error ? e.message : "Failed to revoke trial");
      }
    } finally {
      setBusy(null);
      if (!keepLocked) setWorkspaceMutation(row.workspace_id, null);
    }
  };

  return (
    <div className="abt-form" aria-busy={!!busy}>
      {currentTrial && formattedTrial ? (
        <div className="abt-current">
          <div className="abt-current-head">
            <span className={`abt-badge abt-tone-${formattedTrial.badge.tone}`}>
              {TRIAL_STATUS_LABELS[currentTrial.status]}
            </span>
            <span className="abt-current-title">{formattedTrial.headline}</span>
          </div>
          {formattedTrial.timeline.length > 0 ? <TrialTimeline items={formattedTrial.timeline} /> : null}
          {formattedTrial.terminalReason ? <div className="abt-helper">{formattedTrial.terminalReason}</div> : null}
        </div>
      ) : null}

      <div className="abt-fields">
        <div className="abt-field">
          {row.plan_id === "free" ? (
            <>
              <label className="abt-label" htmlFor={`trial-plan-${row.workspace_id}`}>Trial plan</label>
              <select
                id={`trial-plan-${row.workspace_id}`}
                className="abt-control"
                value={targetPlan}
                onChange={(event) => {
                  setTargetPlan(event.target.value as typeof targetPlan);
                  setError(null);
                  setSuccess(null);
                }}
                disabled={interactionLocked || hasOpenTrial}
              >
                {TRIAL_PLAN_OPTIONS.map((plan) => <option key={plan.id} value={plan.id}>{plan.label}</option>)}
              </select>
            </>
          ) : (
            <>
              <span id={`trial-plan-label-${row.workspace_id}`} className="abt-label">Trial plan</span>
              <div className="abt-control abt-readonly" aria-labelledby={`trial-plan-label-${row.workspace_id}`}>
                {trialPlanLabel(row.plan_id)}
              </div>
            </>
          )}
        </div>
        <div className="abt-field">
          <label className="abt-label" htmlFor={`trial-days-${row.workspace_id}`}>Days</label>
          <input
            id={`trial-days-${row.workspace_id}`}
            className="abt-control"
            type="number"
            min={1}
            max={730}
            step={1}
            inputMode="numeric"
            value={durationDays}
            onChange={(event) => {
              setDurationDays(event.target.value);
              setError(null);
              setSuccess(null);
            }}
            aria-describedby={`${durationError ? `trial-days-error-${row.workspace_id} ` : ""}trial-help-${row.workspace_id}`}
            aria-invalid={!validDays}
            disabled={interactionLocked || hasOpenTrial}
          />
          {durationError ? <span id={`trial-days-error-${row.workspace_id}`} className="abt-helper abt-error" role="alert">{durationError}</span> : null}
        </div>
      </div>

      <div id={`trial-help-${row.workspace_id}`} className="abt-helper">
        {workspaceMutation === "refresh_required"
          ? "Refresh required. Refresh Billing to load the confirmed workspace state."
          : hasOpenTrial
          ? "This workspace already has an open trial."
          : row.plan_id === "free"
            ? "The user must complete checkout before access starts."
            : grantPlanIsEligible
              ? "The current paid plan is fixed for this trial."
              : "This plan is not eligible for an admin trial."}
      </div>

      {!hasOpenTrial && validDays && grantPlanIsEligible ? <TrialTimeline items={proposal} /> : null}

      <div className="abt-actions">
        <button
          type="button"
          className="abt-button abt-button-primary"
          onClick={requestGrant}
          disabled={interactionLocked || hasOpenTrial || !validDays || !grantPlanIsEligible}
        >
          {busy === "grant" ? "Granting…" : hasOpenTrial ? "Trial already open" : "Grant Trial"}
        </button>
        {canRevoke ? (
          <button type="button" className="abt-button abt-button-danger" onClick={requestRevoke} disabled={interactionLocked}>
            {busy === "revoke" ? "Revoking…" : "Revoke"}
          </button>
        ) : null}
      </div>

      {visibleError ? <div className="abt-message abt-error" role="alert">{visibleError}</div> : null}
      {success ? <div className="abt-message abt-success" role="status" aria-live="polite">{success}</div> : null}

      {confirmation?.kind === "grant" ? (
        <ConfirmModal
          open
          title="Grant trial"
          message={(
            <div style={{ display: "grid", gap: 12 }}>
              <div>
                Grant a {confirmation.durationDays}-day {trialPlanLabel(confirmation.planId)} trial to {row.workspace_name}?
              </div>
              <TrialTimeline items={confirmation.timeline} />
            </div>
          )}
          confirmLabel="Grant Trial"
          confirmDisabled={interactionLocked}
          onConfirm={() => {
            const request = confirmation;
            setConfirmation(null);
            void grant(request);
          }}
          onCancel={() => setConfirmation(null)}
        />
      ) : null}

      {confirmation?.kind === "revoke" ? (
        <ConfirmModal
          open
          title="Revoke trial offer"
          message={(
            <div>
              Revoke the {trialPlanLabel(confirmation.planId)} trial offer for {row.workspace_name}? The user will no longer be able to activate it.
            </div>
          )}
          confirmLabel="Revoke"
          variant="danger"
          confirmDisabled={interactionLocked}
          onConfirm={() => {
            const request = confirmation;
            setConfirmation(null);
            void revoke(request);
          }}
          onCancel={() => setConfirmation(null)}
        />
      ) : null}
    </div>
  );
}

// PlanFlipMenu is a tiny inline dropdown that lets an admin change a
// workspace's plan without going through Stripe. Used for QA of the
// plan-feature gates (Inbox, Analytics, profile cap, daily-cap, X
// publishing). It is locked during managed trials and all other billing
// mutations so this bypass cannot race Stripe-backed changes.
function PlanFlipMenu({
  workspaceId,
  currentPlan,
  hasOpenTrial,
  workspaceMutation,
  setWorkspaceMutation,
  onChanged,
}: {
  workspaceId: string;
  currentPlan: string;
  hasOpenTrial: boolean;
  workspaceMutation?: WorkspaceMutationState;
  setWorkspaceMutation: (workspaceId: string, state: WorkspaceMutationState | null) => number;
  onChanged: (mutationEpoch: number) => Promise<void>;
}) {
  const { getToken } = useAuth();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onPick = async (plan: string) => {
    if (plan === currentPlan || busy || hasOpenTrial || workspaceMutation) return;
    if (!confirm(`Change this workspace from "${currentPlan}" to "${plan}"? This bypasses Stripe entirely.`)) return;
    setBusy(true);
    const mutationEpoch = setWorkspaceMutation(workspaceId, "plan");
    setError(null);
    let mutationSucceeded = false;
    let keepLocked = false;
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      await setAdminWorkspacePlan(token, workspaceId, plan);
      mutationSucceeded = true;
      await onChanged(mutationEpoch);
    } catch (e) {
      if (mutationSucceeded) {
        keepLocked = true;
        setWorkspaceMutation(workspaceId, "refresh_required");
        setError(PLAN_REFRESH_REQUIRED_MESSAGE);
      } else {
        setError(e instanceof Error ? e.message : "Failed to flip plan");
      }
    } finally {
      setBusy(false);
      if (!keepLocked) setWorkspaceMutation(workspaceId, null);
    }
  };

  const disabled = busy || hasOpenTrial || !!workspaceMutation;
  const visibleError = error === PLAN_REFRESH_REQUIRED_MESSAGE && workspaceMutation !== "refresh_required" ? null : error;
  const title = workspaceMutation === "refresh_required"
    ? "Refresh Billing before changing this plan"
    : hasOpenTrial
      ? "Plan flip is disabled while a managed trial is open"
      : workspaceMutation
        ? "Another billing change is in progress"
        : "Flip plan (admin only — bypasses Stripe)";

  return (
    <div style={{ marginTop: 6 }}>
      <select
        value={currentPlan}
        onChange={(e) => void onPick(e.target.value)}
        disabled={disabled}
        style={{
          fontFamily: "var(--font-mono, ui-monospace)",
          fontSize: 11,
          padding: "3px 6px",
          background: "var(--surface2)",
          border: "1px solid var(--dborder2)",
          borderRadius: 4,
          color: "var(--dmuted)",
          cursor: disabled ? "not-allowed" : "pointer",
        }}
        title={title}
      >
        {PLAN_FLIP_OPTIONS.map((p) => (
          <option key={p} value={p}>
            {p === currentPlan ? `→ ${p}` : `flip to ${p}`}
          </option>
        ))}
      </select>
      {visibleError && <div style={{ fontSize: 10.5, color: "var(--danger)", marginTop: 2 }}>{visibleError}</div>}
    </div>
  );
}
