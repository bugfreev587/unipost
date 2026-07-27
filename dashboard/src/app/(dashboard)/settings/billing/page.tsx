"use client";

import { Suspense, useCallback, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { useCurrentWorkspace } from "@/lib/use-current-workspace";
import {
  getBilling,
  getTrialHistory,
  getWorkspaceFeatureFlags,
  getXCreditsAllowance,
  updateXInboundDailyCap,
  createCheckout,
  createPlanChangeSession,
  createPortal,
  cancelTrialRenewal,
  changeTrialPlan,
  type BillingInfo,
  type Plan,
  type WorkspaceTrialHistoryEntry,
  type XCreditsAllowance,
} from "@/lib/api";
import { X_CREDIT_OPERATIONS, X_CREDIT_PLANS } from "@/data/x-credits-catalog.generated";
import { formatPlanPostAllowance, formatPostUsage, usagePercentage } from "@/lib/billing-format";
import { isPlanVisibleInBilling } from "@/lib/plan-visibility";
import { buildContactPageHref, buildSupportMailto } from "@/lib/support";
import { formatWorkspaceTrial } from "@/lib/trial-format";
import { CheckCircle2, ExternalLink } from "lucide-react";

// Pricing redesign May 2026 (migration 058): tiers are now product-stage
// based, not per-volume. IDs match plans.id.
const PLANS: Plan[] = [
  { id: "free",   name: "Free",   price_cents: 0,     post_limit: 100,  pricing_model: "fixed" },
  { id: "api",    name: "API",    price_cents: 1000,  post_limit: 1000, pricing_model: "fixed" },
  { id: "basic",  name: "Basic",  price_cents: 1900,  post_limit: 2500, pricing_model: "fixed" },
  { id: "growth", name: "Growth", price_cents: 5900,  post_limit: 7500, pricing_model: "fixed" },
  { id: "team",   name: "Team",   price_cents: 14900, post_limit: -1,   pricing_model: "fixed" },
];

// Short blurbs surfaced on the upgrade card so customers see the
// product-stage difference instead of just a price/quota grid.
const PLAN_BLURBS: Record<string, string> = {
  free:   "Try the API and dashboard.",
  api:    "Dashboard + API + Analytics.",
  basic:  "Adds one custom platform, Inbox, and full Analytics.",
  growth: "Adds all-platform custom mode and optional attribution removal.",
  team:   "Adds RBAC and team collab.",
};

const BILLING_TRIAL_CSS = `
.trial-summary{margin-bottom:24px;padding:18px;border:1px solid color-mix(in srgb,var(--daccent) 28%,var(--dborder));border-radius:10px;background:color-mix(in srgb,var(--daccent) 6%,var(--dcard))}
.trial-summary-head,.trial-history-head,.trial-history-item-head{display:flex;align-items:flex-start;justify-content:space-between;gap:14px}
.trial-badge{display:inline-flex;align-items:center;min-height:22px;padding:3px 8px;border-radius:999px;background:color-mix(in srgb,var(--daccent) 10%,var(--dcard));font-size:10px;font-weight:700;letter-spacing:.05em;text-transform:uppercase;color:var(--daccent);white-space:nowrap}
.trial-timeline{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:10px;margin-top:14px}
.trial-timeline-item{padding-top:9px;border-top:1px solid var(--dborder);min-width:0}
.trial-timeline-label{font-size:10px;color:var(--dmuted);text-transform:uppercase;letter-spacing:.05em}
.trial-timeline-value{margin-top:3px;font-family:var(--font-geist-mono),monospace;font-size:12px;color:var(--dtext);overflow-wrap:anywhere}
.trial-actions{display:flex;flex-wrap:wrap;gap:8px;margin-top:14px}
.trial-history{margin-top:30px;padding-top:24px;border-top:1px solid var(--dborder)}
.trial-history-list{display:grid;gap:0;margin-top:12px;border-top:1px solid var(--dborder)}
.trial-history-item{padding:16px 0;border-bottom:1px solid var(--dborder)}
.trial-history-reason{margin-top:9px;font-size:12px;color:var(--dmuted)}
.trial-state{margin-top:12px;padding:14px;border:1px dashed var(--dborder);border-radius:8px;font-size:12.5px;color:var(--dmuted)}
.trial-skeleton{height:72px;background:linear-gradient(90deg,var(--dcard),color-mix(in srgb,var(--dcard) 82%,var(--dborder)),var(--dcard));background-size:200% 100%;animation:trial-shimmer 1.4s ease-in-out infinite}
@keyframes trial-shimmer{to{background-position:-200% 0}}
@media(max-width:680px){.trial-summary-head,.trial-history-head,.trial-history-item-head{flex-direction:column}.trial-timeline{grid-template-columns:1fr}.trial-actions .dbtn{width:100%;justify-content:center}.trial-history-item{padding:14px 0}}
@media(prefers-reduced-motion:reduce){.trial-skeleton{animation:none}}
`;

// The page default export wraps the content in a Suspense boundary.
// useSearchParams() on a statically prerenderable route (this one has
// no dynamic segments) triggers Next.js's "missing-suspense-with-csr-
// bailout" build error; the boundary lets the build skip CSR-reliant
// subtrees during static generation and render them on the client.
export default function BillingSettingsPage() {
  return (
    <Suspense fallback={<div style={{ color: "var(--dmuted)" }}>Loading...</div>}>
      <BillingSettingsContent />
    </Suspense>
  );
}

function BillingSettingsContent() {
  const { workspace, loading: workspaceLoading } = useCurrentWorkspace();
  const workspaceId = workspace?.id ?? "";
  const searchParams = useSearchParams();
  const { getToken } = useAuth();
  const [billing, setBilling] = useState<BillingInfo | null>(null);
  const [trialHistory, setTrialHistory] = useState<WorkspaceTrialHistoryEntry[]>([]);
  const [trialHistoryLoading, setTrialHistoryLoading] = useState(true);
  const [trialHistoryError, setTrialHistoryError] = useState<string | null>(null);
  const [xCredits, setXCredits] = useState<XCreditsAllowance | null>(null);
  const [xCreditsEnabled, setXCreditsEnabled] = useState(false);
  const [xCreditsLoading, setXCreditsLoading] = useState(true);
  const [xCreditsError, setXCreditsError] = useState<string | null>(null);
  const [xInboundCapDraft, setXInboundCapDraft] = useState("");
  const [xInboundCapAcknowledged, setXInboundCapAcknowledged] = useState(false);
  const [xInboundCapSaving, setXInboundCapSaving] = useState(false);
  const [loading, setLoading] = useState(true);
  const [upgrading, setUpgrading] = useState<string | null>(null);
  const [cancelingTrial, setCancelingTrial] = useState(false);
  const [billingError, setBillingError] = useState<{ message: string; topic: string } | null>(null);
  const callbackStatus = searchParams.get("status");
  const hasForfeitableTrial = Boolean(
    billing?.trial?.changing_plan_forfeits_trial &&
      (billing.trial.status === "scheduled" || billing.trial.status === "active"),
  );

  const loadBilling = useCallback(async () => {
    if (!workspaceId) return;
    try {
      setBillingError(null);
      setTrialHistoryError(null);
      setTrialHistoryLoading(true);
      setXCreditsError(null);
      setXCreditsLoading(true);
      const token = await getToken();
      if (!token) return;
      const [billingResult, historyResult, featureFlagsResult] = await Promise.allSettled([
        getBilling(token),
        getTrialHistory(token),
        getWorkspaceFeatureFlags(token),
      ]);
      if (billingResult.status === "rejected") {
        throw billingResult.reason;
      }
      setBilling(billingResult.value.data);
      if (historyResult.status === "fulfilled") {
        setTrialHistory(
          [...historyResult.value.data].sort(
            (left, right) => Date.parse(right.granted_at ?? "") - Date.parse(left.granted_at ?? ""),
          ),
        );
      } else {
        setTrialHistory([]);
        setTrialHistoryError(
          historyResult.reason instanceof Error
            ? historyResult.reason.message
            : "Trial history could not be loaded",
        );
      }
      const enabled =
        featureFlagsResult.status === "fulfilled" &&
        featureFlagsResult.value.data.flags.x_credits_billing_v1;
      setXCreditsEnabled(enabled);
      if (enabled) {
        try {
          const xCreditsResult = await getXCreditsAllowance(token);
          setXCredits(xCreditsResult.data);
          setXInboundCapDraft(
            xCreditsResult.data.inbound_daily_limit == null
              ? ""
              : String(xCreditsResult.data.inbound_daily_limit),
          );
        } catch (err) {
          setXCredits(null);
          setXCreditsError(
            err instanceof Error ? err.message : "Failed to load X Credits allowance",
          );
        }
      } else {
        setXCredits(null);
        setXCreditsError(null);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to load billing";
      console.error("Failed to load billing:", err);
      setBillingError({ message, topic: "billing-load-failure" });
    } finally {
      setTrialHistoryLoading(false);
      setXCreditsLoading(false);
      setLoading(false);
    }
  }, [getToken, workspaceId]);

  useEffect(() => {
    loadBilling();
  }, [loadBilling]);

  const upgradePlan = searchParams.get("upgrade");
  const [autoUpgradeTriggered, setAutoUpgradeTriggered] = useState(false);
  useEffect(() => {
    if (upgradePlan && !loading && billing && !autoUpgradeTriggered) {
      setAutoUpgradeTriggered(true);
      if (billing.plan !== upgradePlan) {
        handleUpgrade(upgradePlan);
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [upgradePlan, loading, billing, autoUpgradeTriggered]);

  async function handleUpgrade(planId: string) {
    if (!workspaceId) return;
    setUpgrading(planId);
    try {
      setBillingError(null);
      const token = await getToken();
      if (!token) return;
      if (hasForfeitableTrial && billing?.trial?.plan_id !== planId) {
        const confirmed = window.confirm(
          "Changing plans ends your trial immediately. The new plan starts billing now for a full billing period. Continue?",
        );
        if (!confirmed) return;
        await changeTrialPlan(token, planId);
        await loadBilling();
        return;
      }
      if (billing?.plan === "free") {
        const res = await createCheckout(token, planId);
        window.location.href = res.data.checkout_url;
        return;
      }
      const res = await createPlanChangeSession(token, planId);
      window.location.href = res.data.url;
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to start plan change";
      console.error("Failed:", err);
      setBillingError({ message, topic: "billing-upgrade-failure" });
      setUpgrading(null);
    }
  }

  async function handleCancelTrialRenewal() {
    if (!billing?.trial || cancelingTrial) return;
    const confirmed = window.confirm(
      "Cancel renewal? You will keep access through the trial end date and will not be charged afterward.",
    );
    if (!confirmed) return;
    setCancelingTrial(true);
    setBillingError(null);
    try {
      const token = await getToken();
      if (!token) return;
      await cancelTrialRenewal(token, billing.trial.id);
      await loadBilling();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to cancel trial renewal";
      setBillingError({ message, topic: "trial-cancel-renewal-failure" });
    } finally {
      setCancelingTrial(false);
    }
  }

  async function handleManage() {
    if (!workspaceId) return;
    try {
      setBillingError(null);
      const token = await getToken();
      if (!token) return;
      const res = await createPortal(token);
      window.location.href = res.data.portal_url;
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to open billing portal";
      console.error("Failed:", err);
      setBillingError({ message, topic: "billing-portal-failure" });
    }
  }

  async function handleSaveXInboundCap() {
    const parsed = Number(xInboundCapDraft);
    if (!Number.isInteger(parsed) || parsed < 0) {
      setXCreditsError("Daily cap must be a whole number of X Credits, zero or greater.");
      return;
    }
    setXInboundCapSaving(true);
    setXCreditsError(null);
    try {
      const token = await getToken();
      if (!token) return;
      await updateXInboundDailyCap(token, parsed, xInboundCapAcknowledged);
      setXInboundCapAcknowledged(false);
      await loadBilling();
    } catch (err) {
      setXCreditsError(err instanceof Error ? err.message : "Failed to update X inbound daily cap");
    } finally {
      setXInboundCapSaving(false);
    }
  }

  if (workspaceLoading || loading) {
    return (
      <div aria-live="polite" aria-busy="true">
        <div className="dt-body-sm" style={{ marginBottom: 10 }}>Loading billing and trial history…</div>
        <div className="trial-state trial-skeleton" aria-hidden="true" />
      </div>
    );
  }

  const used = billing?.completed_usage ?? billing?.usage ?? 0;
  const scheduled = billing?.scheduled_usage ?? 0;
  const held = billing?.quota_hold_usage ?? 0;
  const effectiveUsage = billing?.effective_usage ?? used + scheduled;
  const limit = billing?.limit ?? 100;
  const pct = billing
    ? Math.round(billing.effective_percentage ?? usagePercentage(effectiveUsage, limit))
    : 0;
  const barClass = pct >= 100 ? "bar-red" : pct >= 80 ? "bar-amber" : "bar-green";
  const xPlan = X_CREDIT_PLANS.find((plan) => plan.id === xCredits?.plan_id);
  const xAllowance = xCredits?.monthly_allowance;
  const xRemaining = xCredits?.monthly_remaining;
  const xCreditsPct = xAllowance && xAllowance > 0
    ? Math.min(100, Math.round(((xCredits?.monthly_used ?? 0) / xAllowance) * 100))
    : 0;
  const xResetDate = xCredits?.billing_period_end
    ? new Intl.DateTimeFormat(undefined, { dateStyle: "medium" }).format(new Date(xCredits.billing_period_end))
    : "";
  const xOperationCredits = Object.fromEntries(X_CREDIT_OPERATIONS.map((operation) => [operation.key, operation.credits]));
  const normalPostCredits = xOperationCredits["post.create"];
  const urlPostCredits = xOperationCredits["post.create_url"];
  const completeCommentCredits = xOperationCredits["post.mention.received"] + xOperationCredits["post.create"];
  const completeDMCredits = xOperationCredits["dm.received"] + xOperationCredits["dm.send"];
  const visiblePlans = PLANS.filter((plan) => isPlanVisibleInBilling(plan.id, billing?.plan));
  const formattedCurrentTrial = billing?.trial ? formatWorkspaceTrial(billing.trial) : null;

  return (
    <>
      <style>{BILLING_TRIAL_CSS}</style>
      {callbackStatus === "success" && (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 8,
            padding: "10px 14px",
            borderRadius: 6,
            background: "#10b98110",
            border: "1px solid #10b98125",
            fontSize: 13,
            color: "var(--daccent)",
            marginBottom: 20,
          }}
        >
          <CheckCircle2 style={{ width: 14, height: 14 }} /> Subscription updated.
        </div>
      )}

      {billingError && (
        <div
          style={{
            display: "flex",
            alignItems: "flex-start",
            justifyContent: "space-between",
            gap: 16,
            padding: "12px 14px",
            borderRadius: 8,
            background: "#ef444410",
            border: "1px solid #ef444425",
            fontSize: 13,
            color: "var(--danger)",
            marginBottom: 20,
          }}
        >
          <div>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>Billing action failed</div>
            <div>{billingError.message}</div>
          </div>
          <div style={{ display: "flex", gap: 8, flexShrink: 0 }}>
            <a
              href={buildSupportMailto({
                subject: "Billing action failed in dashboard",
                intro: "I ran into a billing-related failure in the dashboard.",
                details: [
                  `Workspace ID: ${workspaceId}`,
                  `Topic: ${billingError.topic}`,
                  `Error: ${billingError.message}`,
                ],
              })}
              className="dbtn dbtn-ghost"
              style={{ fontSize: 12 }}
            >
              Contact support
            </a>
            <a
              href={buildContactPageHref({
                topic: billingError.topic,
                source: "billing-settings",
                workspace: workspaceId,
                error: billingError.message,
              })}
              className="dbtn dbtn-ghost"
              style={{ fontSize: 12 }}
            >
              Open help center
            </a>
          </div>
        </div>
      )}

      <div
        style={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "flex-end",
          marginBottom: 20,
        }}
      >
        {billing?.plan !== "free" && (
          <button className="dbtn dbtn-ghost" onClick={handleManage}>
            Manage Subscription <ExternalLink style={{ width: 12, height: 12 }} />
          </button>
        )}
      </div>

      {billing?.trial && formattedCurrentTrial ? (
        <section className="trial-summary" aria-labelledby="managed-trial-heading">
          <div className="trial-summary-head">
            <div>
              <div id="managed-trial-heading" className="dt-label" style={{ marginBottom: 5 }}>Managed trial</div>
              <div className="dt-body" style={{ fontWeight: 700 }}>{formattedCurrentTrial.headline}</div>
              {formattedCurrentTrial.remainingDays !== null ? (
                <div className="dt-body-sm" style={{ marginTop: 4 }}>
                  {formattedCurrentTrial.remainingDays} day{formattedCurrentTrial.remainingDays === 1 ? "" : "s"} remaining
                </div>
              ) : null}
            </div>
            <span className="trial-badge">{formattedCurrentTrial.badge.label}</span>
          </div>
          <div className="trial-timeline">
            {formattedCurrentTrial.timeline.map((item) => (
              <div className="trial-timeline-item" key={`${item.label}-${item.value}`}>
                <div className="trial-timeline-label">{item.label}</div>
                <div className="trial-timeline-value">{item.value}</div>
              </div>
            ))}
          </div>
          {hasForfeitableTrial ? (
            <div className="trial-state" role="note">
              Changing plans ends your trial immediately. The selected plan starts billing for a full period.
            </div>
          ) : null}
          {(billing.trial.status === "scheduled" || billing.trial.status === "active") ? (
            <div className="trial-actions">
              <button
                type="button"
                className="dbtn dbtn-ghost"
                onClick={() => void handleCancelTrialRenewal()}
                disabled={cancelingTrial || billing.trial.cancel_at_period_end}
              >
                {billing.trial.cancel_at_period_end
                  ? "Renewal canceled"
                  : cancelingTrial
                    ? "Canceling…"
                    : "Cancel renewal"}
              </button>
            </div>
          ) : null}
        </section>
      ) : null}

      <div className="stat-grid">
        <div className="stat-card">
          <div className="dt-label" style={{ marginBottom: 8 }}>

            Effective Monthly Usage
          </div>
          <div style={{ fontFamily: "var(--font-geist-mono), monospace", fontSize: 22, fontWeight: 600, letterSpacing: -0.5 }}>

            {effectiveUsage.toLocaleString()}
          </div>
          <div style={{ margin: "8px 0 4px" }}>
            <div className="usage-bar-track">
              <div
                className={`usage-bar-fill ${barClass}`}
                style={{ width: `${Math.min(pct, 100)}%` }}
              />
            </div>
          </div>
          <div
            style={{
              fontSize: 12,
              color: pct >= 80 ? "var(--warning)" : "var(--dmuted)",
            }}
          >
            {formatPostUsage(effectiveUsage, limit)}
            {limit > 0 ? <> &middot; {pct}%</> : null}
          </div>
          <div style={{ marginTop: 10, display: "flex", flexWrap: "wrap", gap: "6px 14px", fontSize: 11, color: "var(--dmuted)" }}>
            <span>Published {used.toLocaleString()}</span>
            <span>Committed schedule {scheduled.toLocaleString()}</span>
            {held > 0 ? <span style={{ color: "var(--warning)" }}>On quota hold {held.toLocaleString()}</span> : null}
          </div>
        </div>
        <div className="stat-card">
          <div className="dt-label" style={{ marginBottom: 8 }}>

            Current Plan
          </div>
          <div style={{ fontFamily: "var(--font-geist-mono), monospace", fontSize: 22, fontWeight: 600, letterSpacing: -0.5 }}>

            {billing?.plan_name || "Free"}
          </div>
          <div style={{ fontSize: 12, color: "var(--dmuted)", marginTop: 4 }}>
            {billing?.period || ""}
          </div>
        </div>
        <div className="stat-card">
          <div className="dt-label" style={{ marginBottom: 8 }}>

            Scheduling
          </div>
          <div style={{ fontFamily: "var(--font-geist-mono), monospace", fontSize: 22, fontWeight: 600, letterSpacing: -0.5 }}>

            {billing?.scheduling_allowed === false ? "Paused" : "Available"}
          </div>
          <div style={{ fontSize: 12, color: "var(--dmuted)", marginTop: 4 }}>
            {billing?.scheduling_allowed === false
              ? "Publish now remains available"
              : `Subscription ${billing?.status || "active"}`}
          </div>
        </div>
      </div>

      {billing?.warning && (
        <div
          style={{
            padding: "10px 14px",
            borderRadius: 6,
            marginBottom: 20,
            background: billing.warning === "scheduled_quota_reached" || billing.warning === "over_limit" ? "#ef444410" : "#f59e0b10",
            border: `1px solid ${
              billing.warning === "scheduled_quota_reached" || billing.warning === "over_limit" ? "#ef444425" : "#f59e0b25"
            }`,
            fontSize: 13,
            color:
              billing.warning === "scheduled_quota_reached" || billing.warning === "over_limit" ? "var(--danger)" : "var(--warning)",
          }}
        >
          {billing.warning === "scheduled_quota_reached"
            ? billing.quota_hold_usage > 0
              ? `${billing.quota_hold_usage.toLocaleString()} scheduled units are on quota hold and will not publish automatically. Upgrade, cancel them, move them into a month with capacity, or publish them manually. New scheduled posts remain paused until the holds are resolved.`
              : `Monthly scheduling capacity is full. New scheduled posts are paused until ${billing.resets_at ? new Date(billing.resets_at).toLocaleDateString() : "the next billing month"} or until you upgrade. Immediate publishing remains available.`
            : billing.warning === "over_limit"
            ? billing.plan === "free"
              ? "Free monthly post quota reached. Upgrade to keep posting this month."
              : "Monthly post quota exceeded. Immediate publishing remains available, but new scheduled posts require available capacity."
            : `${pct}% of effective monthly quota is committed. Review upcoming posts or upgrade before scheduling capacity fills.`}
        </div>
      )}

      {xCreditsEnabled ? <section
        aria-labelledby="x-credits-heading"
        style={{
          marginBottom: 24,
          padding: 18,
          border: "1px solid var(--dborder)",
          borderRadius: 10,
          background: "var(--dcard)",
        }}
      >
        <div style={{ display: "flex", justifyContent: "space-between", gap: 16, alignItems: "flex-start", marginBottom: 14 }}>
          <div>
            <div id="x-credits-heading" className="dt-body" style={{ fontWeight: 700, marginBottom: 4 }}>X Credits</div>
            <div className="dt-body-sm">
              Included managed-X allowance. It is separate from posts/month and resets each billing period.
            </div>
          </div>
          <a href="/docs/guides/x/credits" style={{ fontSize: 12, color: "var(--daccent)", textDecoration: "underline", flexShrink: 0 }}>
            How usage works
          </a>
        </div>

        {xCreditsLoading ? (
          <div className="dt-body-sm">Loading X Credits...</div>
        ) : xCreditsError ? (
          <div style={{ color: "var(--danger)", fontSize: 13 }}>
            X Credits could not be loaded: {xCreditsError}
          </div>
        ) : xCredits?.monthly_allowance == null ? (
          <div>
            <div style={{ fontFamily: "var(--font-geist-mono), monospace", fontSize: 22, fontWeight: 650 }}>Custom</div>
            <div className="dt-body-sm" style={{ marginTop: 5 }}>
              Enterprise X Credits and inbound limits are defined by your contract. Contact your UniPost account team for capacity changes.
            </div>
          </div>
        ) : xCredits.monthly_allowance === 0 ? (
          <div>
            <div style={{ fontFamily: "var(--font-geist-mono), monospace", fontSize: 22, fontWeight: 650 }}>0 included</div>
            <div className="dt-body-sm" style={{ marginTop: 5 }}>
              This plan does not include managed X usage. Upgrade to a paid plan to publish through UniPost-managed X credentials.
            </div>
          </div>
        ) : (
          <div>
            <div style={{ display: "flex", justifyContent: "space-between", gap: 12, alignItems: "baseline" }}>
              <div style={{ fontFamily: "var(--font-geist-mono), monospace", fontSize: 22, fontWeight: 650 }}>
                {(xCredits.monthly_used ?? 0).toLocaleString()} / {xCredits.monthly_allowance.toLocaleString()}
              </div>
              <div style={{ fontSize: 12, color: "var(--dmuted)" }}>
                {(xRemaining ?? 0).toLocaleString()} remaining
              </div>
            </div>
            <div style={{ margin: "10px 0 6px" }} className="usage-bar-track">
              <div
                className={`usage-bar-fill ${xCreditsPct >= 100 ? "bar-red" : xCreditsPct >= 80 ? "bar-amber" : "bar-green"}`}
                style={{ width: `${xCreditsPct}%` }}
              />
            </div>
            <div style={{ fontSize: 12, color: "var(--dmuted)" }}>
              Resets {xResetDate || "at the end of the billing period"}.
            </div>
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "repeat(auto-fit, minmax(150px, 1fr))",
                gap: 8,
                marginTop: 14,
              }}
            >
              {[
                ["Normal X posts", Math.floor((xRemaining ?? 0) / normalPostCredits).toLocaleString()],
                ["Posts with URL", Math.floor((xRemaining ?? 0) / urlPostCredits).toLocaleString()],
                ["Complete comments", xPlan?.inbox_eligible ? Math.floor((xRemaining ?? 0) / completeCommentCredits).toLocaleString() : "Inbox not included"],
                ["Complete DMs", xPlan?.inbox_eligible ? Math.floor((xRemaining ?? 0) / completeDMCredits).toLocaleString() : "Inbox not included"],
              ].map(([label, value]) => (
                <div key={label} style={{ padding: "10px 11px", border: "1px solid var(--dborder)", borderRadius: 8 }}>
                  <div style={{ fontSize: 11, color: "var(--dmuted)", marginBottom: 3 }}>{label}</div>
                  <div style={{ fontFamily: "var(--font-geist-mono), monospace", fontSize: 15, fontWeight: 650 }}>{value}</div>
                </div>
              ))}
            </div>
          </div>
        )}

        {!xCreditsLoading && xCredits ? (
          <div style={{ marginTop: 16, paddingTop: 16, borderTop: "1px solid var(--dborder)" }}>
            <div style={{ display: "grid", gridTemplateColumns: "minmax(0, 1fr) minmax(220px, 0.7fr)", gap: 20, alignItems: "start" }} className="x-inbound-cap-grid">
              <div>
                <div className="dt-body-sm" style={{ fontWeight: 700, color: "var(--dtext)", marginBottom: 5 }}>
                  X inbound daily cap
                </div>
                <div className="dt-body-sm" style={{ color: "var(--dmuted)", lineHeight: 1.55 }}>
                  Stops new paid X comment and DM reads after the workspace reaches this UTC-day limit.
                  Today: {xCredits.inbound_daily_usage.toLocaleString()}
                  {xCredits.inbound_daily_limit == null
                    ? " used (no daily cap)."
                    : ` / ${xCredits.inbound_daily_limit.toLocaleString()} used.`}
                  {xCredits.inbound_events_suppressed > 0
                    ? ` ${xCredits.inbound_events_suppressed.toLocaleString()} event(s) suppressed today.`
                    : ""}
                </div>
                {xCredits.pause_paid_sources ? (
                  <div role="status" style={{ marginTop: 8, color: "var(--warning)", fontSize: 12, fontWeight: 600 }}>
                    Paid inbound X reads are paused: {xCredits.inbound_pause_reason === "daily_cap" ? "daily cap reached" : "monthly allowance reached"}.
                  </div>
                ) : null}
              </div>
              <div style={{ display: "grid", gap: 8 }}>
                <label htmlFor="x-inbound-daily-cap" className="dform-label">
                  Daily X Credits limit
                </label>
                <input
                  id="x-inbound-daily-cap"
                  inputMode="numeric"
                  min={0}
                  step={1}
                  value={xInboundCapDraft}
                  onChange={(event) => setXInboundCapDraft(event.target.value.replace(/[^\d]/g, ""))}
                  className="dform-input"
                  aria-describedby="x-inbound-cap-help"
                  placeholder="0"
                />
                <div id="x-inbound-cap-help" className="dt-micro" style={{ color: "var(--dmuted2)", textTransform: "none", letterSpacing: 0 }}>
                  Use 0 to pause paid inbound X reads. Raising the cap above your remaining monthly allowance requires acknowledgement.
                </div>
                <label style={{ display: "flex", alignItems: "flex-start", gap: 8, fontSize: 12, color: "var(--dmuted)", lineHeight: 1.45 }}>
                  <input
                    type="checkbox"
                    checked={xInboundCapAcknowledged}
                    onChange={(event) => setXInboundCapAcknowledged(event.target.checked)}
                    style={{ marginTop: 2 }}
                  />
                  I understand that a high daily cap can consume the remaining monthly X Credits quickly.
                </label>
                <button
                  type="button"
                  className="dbtn dbtn-primary"
                  onClick={handleSaveXInboundCap}
                  disabled={xInboundCapSaving || xInboundCapDraft === ""}
                  style={{ justifySelf: "start" }}
                >
                  {xInboundCapSaving ? "Saving…" : "Save inbound cap"}
                </button>
              </div>
            </div>
          </div>
        ) : null}

        <div style={{ marginTop: 14, paddingTop: 12, borderTop: "1px solid var(--dborder)", fontSize: 12, lineHeight: 1.6, color: "var(--dmuted)" }}>
          Managed-X work stops at the hard limit. The independent safety cap of 20 X posts per connected account per UTC day still applies.
          Bring-your-own X API connections do not consume this allowance. Comment and DM totals include the live X Inbox rollout.
        </div>
      </section> : null}

      <div style={{ marginBottom: 12 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 4 }}>
          <div className="dt-body" style={{ fontWeight: 600 }}>Upgrade Plan</div>
        </div>
        <div className="dt-body-sm">
          Plans are product-stage tiers (Free / Basic / Growth / Team). See the full feature matrix at <a href="/pricing" style={{ color: "var(--daccent)", textDecoration: "underline" }}>unipost.dev/pricing</a>.
        </div>
      </div>
      <div className="plan-cards">
        {visiblePlans.map((plan) => {
          const isCurrent = billing?.plan === plan.id;
          const isTrialPlan = billing?.trial?.plan_id === plan.id;
          const trialPlanCta = isTrialPlan && billing?.trial?.status === "pending_activation"
            ? `Start ${billing.trial.duration_days}-day free trial`
            : isTrialPlan && billing?.trial?.status === "checkout_pending"
              ? "Continue checkout"
              : isTrialPlan && billing?.trial?.status === "provisioning"
                ? "Activation in progress"
                : null;
          const currentPlanIndex = PLANS.findIndex((candidate) => candidate.id === billing?.plan);
          const planIndex = PLANS.findIndex((candidate) => candidate.id === plan.id);
          const price = plan.price_cents == null
            ? "Custom"
            : plan.price_cents === 0
              ? "$0"
              : `$${plan.price_cents / 100}`;
          const blurb = PLAN_BLURBS[plan.id] ?? "";
          return (
            <div key={plan.id} className={`plan-card ${isCurrent ? "current" : ""}`}>
              {isCurrent && (
                <div
                  style={{
                    fontSize: 10,
                    color: "var(--daccent)",
                    fontWeight: 600,
                    textTransform: "uppercase",
                    letterSpacing: "0.08em",
                    marginBottom: 6,
                  }}
                >
                  Current Plan
                </div>
              )}
              <div
                style={{
                  fontFamily: "var(--font-geist-mono), monospace",
                  fontSize: 13,
                  fontWeight: 700,
                  color: "var(--dtext)",
                  textTransform: "uppercase",
                  letterSpacing: "0.04em",
                  marginBottom: 4,
                }}
              >
                {plan.name}
              </div>
              <div
                style={{
                  fontFamily: "var(--font-geist-mono), monospace",
                  fontSize: 20,
                  fontWeight: 600,
                  color: "var(--dtext)",
                }}
              >
                {price}
                <span
                  style={{
                    fontSize: 12,
                    color: "var(--dmuted)",
                    fontWeight: 400,
                  }}
                >
                  /mo
                </span>
              </div>
              <div style={{ fontSize: 12, color: "var(--dmuted)", marginTop: 4 }}>
                {formatPlanPostAllowance(plan.post_limit)}
              </div>
              {blurb && (
                <div style={{ fontSize: 11.5, color: "var(--dmuted)", marginTop: 6, lineHeight: 1.4 }}>
                  {blurb}
                </div>
              )}
              {isTrialPlan && formattedCurrentTrial ? (
                <div style={{ marginTop: 8, fontSize: 11, color: "var(--daccent)", fontWeight: 650 }}>
                  {formattedCurrentTrial.badge.label} · {formattedCurrentTrial.headline}
                </div>
              ) : null}
              {(!isCurrent || trialPlanCta) && plan.id !== "free" && (
                <button
                  className="dbtn dbtn-ghost"
                  style={{
                    marginTop: 10,
                    width: "100%",
                    justifyContent: "center",
                    fontSize: 12,
                  }}
                  onClick={() => handleUpgrade(plan.id)}
                  disabled={upgrading === plan.id || billing?.trial?.status === "provisioning"}
                >
                  {upgrading === plan.id
                    ? "Working…"
                    : trialPlanCta
                      ? trialPlanCta
                    : hasForfeitableTrial
                      ? planIndex > currentPlanIndex ? "Upgrade now" : "Downgrade now"
                      : "Upgrade"}
                </button>
              )}
              {hasForfeitableTrial && !isTrialPlan && plan.id !== "free" ? (
                <div style={{ marginTop: 7, fontSize: 10.5, lineHeight: 1.4, color: "var(--dmuted)" }}>
                  Changing plans ends your trial immediately.
                </div>
              ) : null}
            </div>
          );
        })}
      </div>
      <div style={{ marginTop: 16, padding: "12px 14px", border: "1px dashed var(--dborder)", borderRadius: 8, fontSize: 12.5, color: "var(--dmuted)", lineHeight: 1.6 }}>
        Need custom terms or security review?{" "}
        <a href="mailto:support@unipost.dev" style={{ color: "var(--daccent)", textDecoration: "underline" }}>Contact us about Enterprise</a>.
      </div>

      <section className="trial-history" aria-labelledby="trial-history-heading">
        <div className="trial-history-head">
          <div>
            <div id="trial-history-heading" className="dt-body" style={{ fontWeight: 700 }}>Trial History</div>
            <div className="dt-body-sm" style={{ marginTop: 3 }}>Previous and current workspace trial grants, newest first.</div>
          </div>
        </div>
        {trialHistoryLoading ? (
          <div className="trial-state" aria-live="polite">Loading trial history…</div>
        ) : trialHistoryError ? (
          <div className="trial-state" role="alert">
            <strong>Trial history could not be loaded.</strong> {trialHistoryError}
            <button type="button" className="dbtn dbtn-ghost" style={{ marginTop: 10 }} onClick={() => void loadBilling()}>
              Try again
            </button>
          </div>
        ) : trialHistory.length === 0 ? (
          <div className="trial-state">No trial history yet.</div>
        ) : (
          <div className="trial-history-list">
            {trialHistory.map((entry) => {
              const formatted = formatWorkspaceTrial(entry);
              return (
                <article className="trial-history-item" key={entry.id}>
                  <div className="trial-history-item-head">
                    <div>
                      <div className="dt-body" style={{ fontWeight: 650 }}>{formatted.headline}</div>
                      <div className="dt-body-sm" style={{ marginTop: 3 }}>{entry.duration_days}-day {entry.plan_id.toUpperCase()} trial</div>
                    </div>
                    <span className="trial-badge">{formatted.badge.label}</span>
                  </div>
                  <div className="trial-timeline">
                    {formatted.timeline.map((item) => (
                      <div className="trial-timeline-item" key={`${item.label}-${item.value}`}>
                        <div className="trial-timeline-label">{item.label}</div>
                        <div className="trial-timeline-value">{item.value}</div>
                      </div>
                    ))}
                  </div>
                  {formatted.terminalReason ? (
                    <div className="trial-history-reason">Outcome: {formatted.terminalReason}</div>
                  ) : null}
                </article>
              );
            })}
          </div>
        )}
      </section>
    </>
  );
}
