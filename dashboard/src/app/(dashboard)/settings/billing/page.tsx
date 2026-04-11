"use client";

import { Suspense, useCallback, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { useCurrentWorkspace } from "@/lib/use-current-workspace";
import { getBilling, createCheckout, createPortal, type BillingInfo, type Plan } from "@/lib/api";
import { CheckCircle2, ExternalLink } from "lucide-react";

const PLANS: Plan[] = [
  { id: "free", name: "Free", price_cents: 0, post_limit: 100 },
  { id: "p10", name: "Starter", price_cents: 1000, post_limit: 1000 },
  { id: "p25", name: "Pro", price_cents: 2500, post_limit: 2500 },
  { id: "p50", name: "Growth", price_cents: 5000, post_limit: 5000 },
  { id: "p75", name: "Scale", price_cents: 7500, post_limit: 10000 },
  { id: "p150", name: "Business", price_cents: 15000, post_limit: 20000 },
  { id: "p300", name: "Enterprise", price_cents: 30000, post_limit: 40000 },
  { id: "p500", name: "Enterprise+", price_cents: 50000, post_limit: 100000 },
  { id: "p1000", name: "Custom", price_cents: 100000, post_limit: 200000 },
];

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
  const [loading, setLoading] = useState(true);
  const [upgrading, setUpgrading] = useState<string | null>(null);
  const callbackStatus = searchParams.get("status");

  const loadBilling = useCallback(async () => {
    if (!workspaceId) return;
    try {
      const token = await getToken();
      if (!token) return;
      const res = await getBilling(token, workspaceId);
      setBilling(res.data);
    } catch (err) {
      console.error("Failed to load billing:", err);
    } finally {
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
      const token = await getToken();
      if (!token) return;
      const res = await createCheckout(token, workspaceId, planId);
      window.location.href = res.data.checkout_url;
    } catch (err) {
      console.error("Failed:", err);
      setUpgrading(null);
    }
  }

  async function handleManage() {
    if (!workspaceId) return;
    try {
      const token = await getToken();
      if (!token) return;
      const res = await createPortal(token, workspaceId);
      window.location.href = res.data.portal_url;
    } catch (err) {
      console.error("Failed:", err);
    }
  }

  if (workspaceLoading || loading) {
    return <div style={{ color: "var(--dmuted)" }}>Loading...</div>;
  }

  const used = billing?.usage ?? 0;
  const limit = billing?.limit ?? 100;
  const pct = billing ? Math.round(billing.percentage) : 0;
  const barClass = pct >= 100 ? "bar-red" : pct >= 80 ? "bar-amber" : "bar-green";

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
            fontSize: 12.5,
            color: "var(--daccent)",
            marginBottom: 20,
          }}
        >
          <CheckCircle2 style={{ width: 14, height: 14 }} /> Subscription updated.
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
          <div
            style={{
              fontSize: 11,
              color: "var(--dmuted)",
              textTransform: "uppercase",
              letterSpacing: "0.06em",
              fontWeight: 600,
              marginBottom: 8,
            }}
          >
            Posts This Month
          </div>
          <div
            style={{
              fontFamily: "var(--font-geist-mono), monospace",
              fontSize: 22,
              fontWeight: 600,
              color: "var(--dtext)",
              letterSpacing: -0.5,
            }}
          >
            {used.toLocaleString()}
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
              fontSize: 11.5,
              color: pct >= 80 ? "var(--warning)" : "var(--dmuted)",
            }}
          >
            {used.toLocaleString()} / {limit.toLocaleString()} posts &middot; {pct}%
          </div>
        </div>
        <div className="stat-card">
          <div
            style={{
              fontSize: 11,
              color: "var(--dmuted)",
              textTransform: "uppercase",
              letterSpacing: "0.06em",
              fontWeight: 600,
              marginBottom: 8,
            }}
          >
            Current Plan
          </div>
          <div
            style={{
              fontFamily: "var(--font-geist-mono), monospace",
              fontSize: 22,
              fontWeight: 600,
              color: "var(--dtext)",
              letterSpacing: -0.5,
            }}
          >
            {billing?.plan_name || "Free"}
          </div>
          <div style={{ fontSize: 11.5, color: "var(--dmuted)", marginTop: 4 }}>
            {billing?.period || ""}
          </div>
        </div>
        <div className="stat-card">
          <div
            style={{
              fontSize: 11,
              color: "var(--dmuted)",
              textTransform: "uppercase",
              letterSpacing: "0.06em",
              fontWeight: 600,
              marginBottom: 8,
            }}
          >
            Status
          </div>
          <div
            style={{
              fontFamily: "var(--font-geist-mono), monospace",
              fontSize: 22,
              fontWeight: 600,
              color: "var(--dtext)",
              letterSpacing: -0.5,
            }}
          >
            {billing?.status || "active"}
          </div>
          <div style={{ fontSize: 11.5, color: "var(--dmuted)", marginTop: 4 }}>
            Unlimited accounts
          </div>
        </div>
      </div>

      {billing?.warning && (
        <div
          style={{
            padding: "10px 14px",
            borderRadius: 6,
            marginBottom: 20,
            background: billing.warning === "over_limit" ? "#ef444410" : "#f59e0b10",
            border: `1px solid ${
              billing.warning === "over_limit" ? "#ef444425" : "#f59e0b25"
            }`,
            fontSize: 12.5,
            color:
              billing.warning === "over_limit" ? "var(--danger)" : "var(--warning)",
          }}
        >
          {billing.warning === "over_limit"
            ? "Monthly limit exceeded. Upgrade to continue posting."
            : `${pct}% of monthly limit used. Consider upgrading.`}
        </div>
      )}

      <div style={{ marginBottom: 12 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 4 }}>
          <div style={{ fontSize: 14, fontWeight: 600, color: "var(--dtext)" }}>Upgrade Plan</div>
          {billing?.trial_eligible && billing?.plan === "free" && (
            <span className="dbadge dbadge-green" style={{ fontSize: 10 }}>
              14-day free trial
            </span>
          )}
        </div>
        <div style={{ color: "var(--dmuted)", fontSize: 12.5 }}>
          All plans include the same features. Only post volume differs.
        </div>
      </div>
      <div className="plan-cards">
        {PLANS.slice(0, 6).map((plan) => {
          const isCurrent = billing?.plan === plan.id;
          const price = plan.price_cents === 0 ? "$0" : `$${plan.price_cents / 100}`;
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
              <div style={{ fontSize: 11.5, color: "var(--dmuted)", marginTop: 4 }}>
                {plan.post_limit.toLocaleString()} posts
              </div>
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
      {PLANS.length > 6 && (
        <div className="plan-cards" style={{ marginTop: 10 }}>
          {PLANS.slice(6).map((plan) => {
            const isCurrent = billing?.plan === plan.id;
            const price = `$${plan.price_cents / 100}`;
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
                <div style={{ fontSize: 11.5, color: "var(--dmuted)", marginTop: 4 }}>
                  {plan.post_limit.toLocaleString()} posts
                </div>
                {!isCurrent && (
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
      )}
    </>
  );
}
