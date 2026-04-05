"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  getBilling,
  createCheckout,
  createPortal,
  type BillingInfo,
  type Plan,
} from "@/lib/api";
import { Activity, CheckCircle2, ExternalLink } from "lucide-react";

const PLANS: Plan[] = [
  { id: "free", name: "Free", price_cents: 0, post_limit: 100 },
  { id: "p10", name: "$10/mo", price_cents: 1000, post_limit: 1000 },
  { id: "p25", name: "$25/mo", price_cents: 2500, post_limit: 2500 },
  { id: "p50", name: "$50/mo", price_cents: 5000, post_limit: 5000 },
  { id: "p75", name: "$75/mo", price_cents: 7500, post_limit: 10000 },
  { id: "p150", name: "$150/mo", price_cents: 15000, post_limit: 20000 },
  { id: "p300", name: "$300/mo", price_cents: 30000, post_limit: 40000 },
  { id: "p500", name: "$500/mo", price_cents: 50000, post_limit: 100000 },
  { id: "p1000", name: "$1000/mo", price_cents: 100000, post_limit: 200000 },
];

export default function BillingPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const searchParams = useSearchParams();
  const { getToken } = useAuth();
  const [billing, setBilling] = useState<BillingInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [upgrading, setUpgrading] = useState<string | null>(null);

  const callbackStatus = searchParams.get("status");

  const loadBilling = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await getBilling(token, projectId);
      setBilling(res.data);
    } catch (err) {
      console.error("Failed to load billing:", err);
    } finally {
      setLoading(false);
    }
  }, [getToken, projectId]);

  useEffect(() => {
    loadBilling();
  }, [loadBilling]);

  async function handleUpgrade(planId: string) {
    setUpgrading(planId);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await createCheckout(token, projectId, planId);
      window.location.href = res.data.checkout_url;
    } catch (err) {
      console.error("Failed to create checkout:", err);
      setUpgrading(null);
    }
  }

  async function handleManage() {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await createPortal(token, projectId);
      window.location.href = res.data.portal_url;
    } catch (err) {
      console.error("Failed to open portal:", err);
    }
  }

  if (loading) {
    return (
      <div className="space-y-4">
        <div className="h-6 w-32 bg-muted rounded animate-pulse" />
        <div className="h-32 rounded-lg bg-muted/50 animate-pulse" />
      </div>
    );
  }

  const usagePct = billing ? Math.min(billing.percentage, 100) : 0;

  return (
    <div>
      {callbackStatus === "success" && (
        <div className="mb-6 flex items-center gap-2 px-4 py-3 rounded-lg border border-foreground/10 bg-foreground/[0.02] text-[13px] animate-fade-up">
          <CheckCircle2 className="w-4 h-4 text-foreground/60 shrink-0" />
          Subscription updated successfully.
        </div>
      )}

      <div className="flex items-center justify-between mb-6 animate-fade-up">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Billing</h1>
          <p className="text-[13px] text-muted-foreground mt-1">
            Manage your plan and track usage.
          </p>
        </div>
        {billing?.plan !== "free" && (
          <Button
            size="sm"
            variant="outline"
            onClick={handleManage}
            className="gap-1.5"
          >
            Manage Subscription
            <ExternalLink className="w-3 h-3" />
          </Button>
        )}
      </div>

      {/* Current plan + usage */}
      <div
        className="rounded-lg border border-border bg-card p-5 mb-8 animate-fade-up"
        style={{ animationDelay: "60ms" }}
      >
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2.5">
            <Activity className="w-4 h-4 text-muted-foreground" />
            <span className="text-[13px] font-medium">Current Plan</span>
          </div>
          <div className="flex items-center gap-2">
            <Badge
              variant={billing?.plan === "free" ? "secondary" : "default"}
              className="text-[11px]"
            >
              {billing?.plan_name || "Free"}
            </Badge>
            {billing?.period && (
              <span className="mono-data text-[11px] text-muted-foreground">
                {billing.period}
              </span>
            )}
          </div>
        </div>

        {/* Usage bar */}
        <div className="space-y-2">
          <div className="flex items-center justify-between text-[12px]">
            <span className="text-muted-foreground">
              Posts this month
            </span>
            <span className="mono-data">
              {billing?.usage ?? 0}{" "}
              <span className="text-muted-foreground">
                / {billing?.limit ?? 100}
              </span>
            </span>
          </div>
          <div className="w-full bg-muted rounded-full h-1.5">
            <div
              className={`h-1.5 rounded-full transition-all duration-500 ${
                usagePct >= 100
                  ? "bg-destructive"
                  : usagePct >= 80
                    ? "bg-amber"
                    : "bg-foreground/70"
              }`}
              style={{ width: `${usagePct}%` }}
            />
          </div>
          {billing?.warning === "approaching_limit" && (
            <p className="text-[12px] text-amber">
              {Math.round(billing.percentage)}% of your monthly limit used.
              Consider upgrading.
            </p>
          )}
          {billing?.warning === "over_limit" && (
            <p className="text-[12px] text-destructive">
              Monthly limit exceeded. Upgrade now to continue posting.
            </p>
          )}
        </div>
      </div>

      {/* Plans */}
      <div className="animate-fade-up" style={{ animationDelay: "120ms" }}>
        <p className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground mb-3">
          Plans
        </p>
        <div className="space-y-1.5">
          {PLANS.map((plan) => {
            const isCurrent = billing?.plan === plan.id;
            return (
              <div
                key={plan.id}
                className={`flex items-center justify-between px-4 py-3 rounded-lg border bg-card ${
                  isCurrent
                    ? "border-foreground/20"
                    : "border-border"
                }`}
              >
                <div className="flex items-center gap-4">
                  <div className="min-w-[80px]">
                    <p className="text-[13px] font-medium">{plan.name}</p>
                  </div>
                  <span className="mono-data text-[12px] text-muted-foreground">
                    {plan.post_limit.toLocaleString()} posts/mo
                  </span>
                </div>
                {isCurrent ? (
                  <Badge variant="secondary" className="text-[10px]">
                    Current
                  </Badge>
                ) : plan.id === "free" ? null : (
                  <Button
                    size="sm"
                    variant="outline"
                    className="h-7 text-[12px]"
                    onClick={() => handleUpgrade(plan.id)}
                    disabled={upgrading === plan.id}
                  >
                    {upgrading === plan.id ? "..." : "Select"}
                  </Button>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
