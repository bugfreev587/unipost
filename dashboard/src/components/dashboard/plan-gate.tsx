"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { getApiLimits } from "@/lib/api";
import { Lock } from "lucide-react";

// PlanGate renders an upgrade card when the active workspace's plan
// doesn't unlock a given feature. Used on Inbox + Analytics pages so
// Free / API users see a clear upgrade CTA instead of a 402 toast.
//
// Server-side enforcement is the source of truth (the matching API
// endpoints return 402 PLAN_FEATURE_NOT_AVAILABLE regardless of what
// the dashboard does). This component is a UX shortcut, not a
// security boundary.

type Feature = "inbox" | "analytics";

const FEATURE_COPY: Record<Feature, { title: string; minTier: string; blurb: string }> = {
  inbox: {
    title: "Inbox is a paid plan feature",
    minTier: "Basic ($19/mo)",
    blurb: "Inbox unifies DMs and comments from your connected accounts. It is included on Basic, Growth, and Team.",
  },
  analytics: {
    title: "Analytics is a paid plan feature",
    minTier: "API ($10/mo)",
    blurb: "Analytics surfaces reach, impressions, and engagement across every connected account. It is included on every paid plan.",
  },
};

export function PlanGate({
  feature,
  children,
}: {
  feature: Feature;
  children: React.ReactNode;
}) {
  const { getToken } = useAuth();
  const [allowed, setAllowed] = useState<boolean | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token || cancelled) return;
        const res = await getApiLimits(token);
        if (cancelled) return;
        const ok =
          feature === "inbox"
            ? res.data.plan_allows_inbox
            : res.data.plan_allows_analytics;
        setAllowed(Boolean(ok));
      } catch {
        // Fail-open on the dashboard: show the feature, let the
        // server-side 402 surface if the plan really doesn't allow.
        if (!cancelled) setAllowed(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [getToken, feature]);

  if (allowed === null) {
    return <div style={{ color: "var(--dmuted)", padding: 16 }}>Loading…</div>;
  }
  if (allowed) return <>{children}</>;

  const copy = FEATURE_COPY[feature];
  return (
    <div
      style={{
        margin: "32px auto",
        maxWidth: 520,
        border: "1px solid var(--dborder)",
        borderRadius: 12,
        padding: 28,
        background: "var(--dcard, transparent)",
        textAlign: "center",
      }}
    >
      <div
        style={{
          width: 44,
          height: 44,
          margin: "0 auto 16px",
          borderRadius: 10,
          background: "var(--success-soft, color-mix(in srgb, var(--daccent) 14%, transparent))",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          color: "var(--daccent)",
        }}
      >
        <Lock size={20} />
      </div>
      <div style={{ fontSize: 17, fontWeight: 700, color: "var(--dtext)", marginBottom: 8 }}>
        {copy.title}
      </div>
      <div style={{ fontSize: 13.5, color: "var(--dmuted)", lineHeight: 1.6, marginBottom: 18 }}>
        {copy.blurb}
        <br />
        <span style={{ fontFamily: "var(--font-mono, ui-monospace)", fontSize: 12.5 }}>
          Minimum tier: {copy.minTier}
        </span>
      </div>
      <Link
        href="/settings/billing"
        className="dbtn dbtn-primary"
        style={{ display: "inline-flex", padding: "8px 18px", fontSize: 13.5 }}
      >
        Upgrade plan
      </Link>
    </div>
  );
}
