"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  getBilling,
  createCheckout,
  createPortal,
  type BillingInfo,
  type Plan,
} from "@/lib/api";

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
    return <div className="text-muted-foreground">Loading...</div>;
  }

  const usagePct = billing ? Math.min(billing.percentage, 100) : 0;
  const barColor =
    (billing?.percentage ?? 0) >= 100
      ? "bg-red-500"
      : (billing?.percentage ?? 0) >= 80
        ? "bg-yellow-500"
        : "bg-green-500";

  return (
    <div>
      {callbackStatus === "success" && (
        <div className="mb-6 p-4 rounded-md bg-green-50 border border-green-200 text-green-800 text-sm">
          Subscription updated successfully!
        </div>
      )}

      <h1 className="text-2xl font-bold mb-6">Billing</h1>

      {/* Current plan + usage */}
      <Card className="mb-8">
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-base">Current Plan</CardTitle>
              <CardDescription className="mt-1">
                {billing?.plan_name || "Free"} &middot; {billing?.period}
              </CardDescription>
            </div>
            <div className="flex items-center gap-2">
              <Badge variant={billing?.plan === "free" ? "secondary" : "default"}>
                {billing?.plan_name || "Free"}
              </Badge>
              {billing?.plan !== "free" && (
                <Button size="sm" variant="outline" onClick={handleManage}>
                  Manage Subscription
                </Button>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <div className="flex items-center justify-between text-sm">
              <span>
                Posts this month: {billing?.usage ?? 0} / {billing?.limit ?? 100}
              </span>
              <span className="text-muted-foreground">
                {Math.round(billing?.percentage ?? 0)}%
              </span>
            </div>
            <div className="w-full bg-muted rounded-full h-3">
              <div
                className={`h-3 rounded-full transition-all ${barColor}`}
                style={{ width: `${usagePct}%` }}
              />
            </div>
            {billing?.warning === "approaching_limit" && (
              <p className="text-sm text-yellow-600">
                You&apos;ve used {Math.round(billing.percentage)}% of your monthly
                posts. Upgrade to avoid interruption.
              </p>
            )}
            {billing?.warning === "over_limit" && (
              <p className="text-sm text-red-600">
                You&apos;ve exceeded your monthly limit. We&apos;re still processing
                your posts. Upgrade now to stay on track.
              </p>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Plans */}
      <h2 className="text-lg font-semibold mb-4">Plans</h2>
      <div className="grid gap-3">
        {PLANS.map((plan) => {
          const isCurrent = billing?.plan === plan.id;
          return (
            <Card
              key={plan.id}
              className={isCurrent ? "border-primary" : ""}
            >
              <CardHeader className="flex flex-row items-center justify-between space-y-0 py-4">
                <div>
                  <CardTitle className="text-base">{plan.name}</CardTitle>
                  <CardDescription>
                    {plan.post_limit.toLocaleString()} posts/month
                  </CardDescription>
                </div>
                {isCurrent ? (
                  <Badge>Current</Badge>
                ) : plan.id === "free" ? null : (
                  <Button
                    size="sm"
                    onClick={() => handleUpgrade(plan.id)}
                    disabled={upgrading === plan.id}
                  >
                    {upgrading === plan.id ? "..." : "Select"}
                  </Button>
                )}
              </CardHeader>
            </Card>
          );
        })}
      </div>
    </div>
  );
}
