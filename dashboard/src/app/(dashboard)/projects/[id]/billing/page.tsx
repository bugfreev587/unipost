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
import { Activity, ExternalLink, CheckCircle2, Zap } from "lucide-react";

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
        <div className="h-5 w-24 bg-[#111111] rounded animate-pulse" />
        <div className="h-32 rounded-lg bg-[#111111] border border-[#1e1e1e] animate-pulse" />
      </div>
    );
  }

  const usagePct = billing ? Math.min(billing.percentage, 100) : 0;
  const usageColor =
    usagePct >= 100
      ? "bg-destructive"
      : usagePct >= 80
        ? "bg-amber-status"
        : "bg-emerald";

  return (
    <div>
      {callbackStatus === "success" && (
        <div className="mb-5 flex items-center gap-2 px-4 py-3 rounded-lg bg-emerald/5 border border-emerald/10 text-[13px] text-emerald animate-enter">
          <CheckCircle2 className="w-4 h-4 shrink-0" />
          Subscription updated.
        </div>
      )}

      <div className="flex items-center justify-between mb-6 animate-enter">
        <div>
          <h1 className="text-[18px] font-semibold text-[#e5e5e5] tracking-tight">
            Billing
          </h1>
          <p className="text-[13px] text-[#525252] mt-0.5">
            Plan, usage, and subscription management.
          </p>
        </div>
        {billing?.plan !== "free" && (
          <Button
            size="sm"
            variant="outline"
            onClick={handleManage}
            className="gap-1.5 border-[#1e1e1e] text-[#a3a3a3] hover:text-[#e5e5e5] hover:border-[#2a2a2a]"
          >
            Manage
            <ExternalLink className="w-3 h-3" />
          </Button>
        )}
      </div>

      {/* Current plan card */}
      <div className="rounded-lg bg-[#111111] border border-[#1e1e1e] p-5 mb-6 animate-enter" style={{ animationDelay: "50ms" }}>
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2.5">
            <Zap className="w-4 h-4 text-emerald" />
            <span className="text-[14px] font-semibold text-[#e5e5e5]">
              {billing?.plan_name || "Free"}
            </span>
            <Badge
              variant="secondary"
              className={`text-[9px] border-0 ${
                billing?.plan === "free"
                  ? "bg-[#1a1a1a] text-[#525252]"
                  : "bg-emerald/10 text-emerald"
              }`}
            >
              {billing?.status || "active"}
            </Badge>
          </div>
          {billing?.period && (
            <span className="mono text-[11px] text-[#3a3a3a]">
              {billing.period}
            </span>
          )}
        </div>

        {/* Usage bar */}
        <div className="space-y-2">
          <div className="flex items-center justify-between text-[12px]">
            <span className="text-[#525252]">Posts this month</span>
            <span className="mono text-[#a3a3a3]">
              {billing?.usage ?? 0}
              <span className="text-[#3a3a3a]"> / {billing?.limit ?? 100}</span>
              <span className="text-[#2a2a2a] ml-1.5">
                ({Math.round(billing?.percentage ?? 0)}%)
              </span>
            </span>
          </div>
          <div className="w-full bg-[#1a1a1a] rounded-full h-2">
            <div
              className={`h-2 rounded-full transition-all duration-700 ease-out ${usageColor}`}
              style={{ width: `${usagePct}%` }}
            />
          </div>
          {billing?.warning === "approaching_limit" && (
            <p className="text-[11px] text-amber-status flex items-center gap-1.5">
              <Activity className="w-3 h-3" />
              {Math.round(billing.percentage)}% used. Consider upgrading.
            </p>
          )}
          {billing?.warning === "over_limit" && (
            <p className="text-[11px] text-destructive flex items-center gap-1.5">
              <Activity className="w-3 h-3" />
              Limit exceeded. Upgrade to continue posting.
            </p>
          )}
        </div>
      </div>

      {/* Plans grid — horizontal scroll */}
      <div className="animate-enter" style={{ animationDelay: "100ms" }}>
        <p className="text-[10px] font-medium uppercase tracking-[0.1em] text-[#525252] mb-3 px-0.5">
          Available Plans
        </p>
        <div className="flex gap-2 overflow-x-auto pb-2 -mx-1 px-1">
          {PLANS.map((plan) => {
            const isCurrent = billing?.plan === plan.id;
            const price = plan.price_cents === 0 ? "Free" : `$${plan.price_cents / 100}`;
            return (
              <div
                key={plan.id}
                className={`shrink-0 w-[140px] rounded-lg border p-4 flex flex-col ${
                  isCurrent
                    ? "bg-emerald/5 border-emerald/20"
                    : "bg-[#111111] border-[#1e1e1e] hover:border-[#2a2a2a]"
                } transition-colors`}
              >
                <p className="text-[12px] font-medium text-[#d4d4d4] mb-1">
                  {plan.name}
                </p>
                <p className="mono text-[18px] font-bold text-[#e5e5e5] tracking-tight mb-0.5">
                  {price}
                  {plan.price_cents > 0 && (
                    <span className="text-[10px] font-normal text-[#3a3a3a]">/mo</span>
                  )}
                </p>
                <p className="mono text-[10px] text-[#3a3a3a] mb-3">
                  {plan.post_limit.toLocaleString()} posts
                </p>
                {isCurrent ? (
                  <Badge variant="secondary" className="text-[9px] bg-emerald/10 text-emerald border-0 w-fit">
                    Current
                  </Badge>
                ) : plan.id === "free" ? null : (
                  <Button
                    size="sm"
                    variant="outline"
                    className="text-[11px] h-7 border-[#1e1e1e] text-[#737373] hover:text-[#e5e5e5] hover:border-emerald/30"
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
