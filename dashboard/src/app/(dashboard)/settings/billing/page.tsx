"use client";

import { Suspense, useCallback, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { useCurrentWorkspace } from "@/lib/use-current-workspace";
import {
  getBilling,
  getXCreditsAllowance,
  createCheckout,
  createPortal,
  type BillingInfo,
  type Plan,
  type XCreditsAllowance,
} from "@/lib/api";
import { X_CREDIT_OPERATIONS, X_CREDIT_PLANS } from "@/data/x-credits-catalog.generated";
import { formatPlanPostAllowance, formatPostUsage, usagePercentage } from "@/lib/billing-format";
import { buildContactPageHref, buildSupportMailto } from "@/lib/support";
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
  const [xCredits, setXCredits] = useState<XCreditsAllowance | null>(null);
  const [xCreditsLoading, setXCreditsLoading] = useState(true);
  const [xCreditsError, setXCreditsError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [upgrading, setUpgrading] = useState<string | null>(null);
  const [billingError, setBillingError] = useState<{ message: string; topic: string } | null>(null);
  const callbackStatus = searchParams.get("status");

  const loadBilling = useCallback(async () => {
    if (!workspaceId) return;
    try {
      setBillingError(null);
      setXCreditsError(null);
      setXCreditsLoading(true);
      const token = await getToken();
      if (!token) return;
      const [billingResult, xCreditsResult] = await Promise.allSettled([
        getBilling(token),
        getXCreditsAllowance(token),
      ]);
      if (billingResult.status === "rejected") {
        throw billingResult.reason;
      }
      setBilling(billingResult.value.data);
      if (xCreditsResult.status === "fulfilled") {
        setXCredits(xCreditsResult.value.data);
      } else {
        setXCreditsError(
          xCreditsResult.reason instanceof Error
            ? xCreditsResult.reason.message
            : "Failed to load X Credits allowance",
        );
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to load billing";
      console.error("Failed to load billing:", err);
      setBillingError({ message, topic: "billing-load-failure" });
    } finally {
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
      const res = await createCheckout(token, planId);
      window.location.href = res.data.checkout_url;
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to start billing checkout";
      console.error("Failed:", err);
      setBillingError({ message, topic: "billing-upgrade-failure" });
      setUpgrading(null);
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

  if (workspaceLoading || loading) {
    return <div style={{ color: "var(--dmuted)" }}>Loading...</div>;
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

  return (
    <>
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

      <section
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

        <div style={{ marginTop: 14, paddingTop: 12, borderTop: "1px solid var(--dborder)", fontSize: 12, lineHeight: 1.6, color: "var(--dmuted)" }}>
          Managed-X work stops at the hard limit. The independent safety cap of 20 X posts per connected account per UTC day still applies.
          Bring-your-own X API connections do not consume this allowance. Comment and DM examples are capacity
          planning for the phased X Inbox rollout.
        </div>
      </section>

      <div style={{ marginBottom: 12 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 4 }}>
          <div className="dt-body" style={{ fontWeight: 600 }}>Upgrade Plan</div>
        </div>
        <div className="dt-body-sm">
          Plans are product-stage tiers (Free / API / Basic / Growth / Team). See the full feature matrix at <a href="/pricing" style={{ color: "var(--daccent)", textDecoration: "underline" }}>unipost.dev/pricing</a>.
        </div>
      </div>
      <div className="plan-cards">
        {PLANS.map((plan) => {
          const isCurrent = billing?.plan === plan.id;
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
              {!isCurrent && plan.id !== "free" && (
                <button
                  className="dbtn dbtn-ghost"
                  style={{
                    marginTop: 10,
                    width: "100%",
                    justifyContent: "center",
                    fontSize: 12,
                  }}
                  onClick={() => handleUpgrade(plan.id)}
                  disabled={upgrading === plan.id}
                >
                  {upgrading === plan.id ? "..." : "Upgrade"}
                </button>
              )}
            </div>
          );
        })}
      </div>
      <div style={{ marginTop: 16, padding: "12px 14px", border: "1px dashed var(--dborder)", borderRadius: 8, fontSize: 12.5, color: "var(--dmuted)", lineHeight: 1.6 }}>
        Need custom terms or security review?{" "}
        <a href="mailto:support@unipost.dev" style={{ color: "var(--daccent)", textDecoration: "underline" }}>Contact us about Enterprise</a>.
      </div>
    </>
  );
}
