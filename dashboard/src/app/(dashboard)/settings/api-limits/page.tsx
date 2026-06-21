"use client";

// API Limits settings page — read-only display of the runtime
// safety caps the API enforces for this workspace. Sourced from
// GET /v1/limits which reads internal/ratelimit/plans.go, so the
// numbers shown here are exactly the numbers enforced.
//
// Three sections, one per admission control:
//
//   - Request rate     : token bucket on the API HTTP layer
//   - Enqueue throughput : sliding window on accepted post units
//   - Queue depth      : count of active delivery jobs
//
// Queue depth refreshes every 30s while the page is open; the
// other two are static per plan and only re-fetch on plan change
// (handled by the page-load fetch).

import { useEffect, useState } from "react";
import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { useCurrentWorkspace } from "@/lib/use-current-workspace";
import { getApiLimits, type ApiLimits } from "@/lib/api";
import { ArrowUpRight } from "lucide-react";

const QUEUE_REFRESH_MS = 30_000;

export default function ApiLimitsPage() {
  const { workspace } = useCurrentWorkspace();
  const workspaceId = workspace?.id ?? "";
  const { getToken } = useAuth();

  const [limits, setLimits] = useState<ApiLimits | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Two-phase load: first fetch on mount renders the static plan
  // values, then a background interval refreshes only the queue
  // depth so the page feels live without re-pinging the limits
  // map every 30s.
  useEffect(() => {
    if (!workspaceId) return;

    let cancelled = false;
    let interval: ReturnType<typeof setInterval> | null = null;

    const refresh = async () => {
      try {
        const token = await getToken();
        if (!token || cancelled) return;
        const res = await getApiLimits(token);
        if (cancelled) return;
        setLimits(res.data);
        setError(null);
      } catch (err) {
        if (cancelled) return;
        const msg = err instanceof Error ? err.message : "Failed to load limits";
        setError(msg);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    refresh();
    interval = setInterval(refresh, QUEUE_REFRESH_MS);

    return () => {
      cancelled = true;
      if (interval) clearInterval(interval);
    };
  }, [workspaceId, getToken]);

  if (loading && !limits) {
    return <div style={{ color: "var(--dmuted)" }}>Loading limits…</div>;
  }
  if (error && !limits) {
    return <div style={{ color: "#f87171" }}>Failed to load limits: {error}</div>;
  }
  if (!limits) return null;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 20, maxWidth: 720 }}>
      <p style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6, margin: 0 }}>
        Runtime safety limits applied to every API request from this workspace. They are
        independent of your monthly post quota and protect the system from accidental
        bursts. Numbers below are the values the API actually enforces.
      </p>

      <PlanCard planID={limits.plan_id} />

      <PlanPackagingCard limits={limits} />

      <LimitCard
        title="Request rate"
        primary={`${limits.request_rate_per_min} req/min`}
        secondary={`Burst ${limits.request_burst} requests`}
        body="A token bucket on every write API request. Tokens refill at the per-minute rate; the burst is the maximum number of back-to-back requests allowed before throttling kicks in."
      />

      <LimitCard
        title="Enqueue throughput"
        primary={`${limits.enqueue_posts_per_min} posts/min`}
        secondary={`${limits.enqueue_posts_per_5min} posts / 5 min`}
        body="Counts accepted post units across two sliding windows. A request is rejected if either window's cap would be exceeded — protects the queue from a few requests that each enqueue a lot of work."
      />

      <DepthCard
        current={limits.queue_depth_current}
        cap={limits.queue_depth_cap}
        managedCap={limits.managed_user_depth_cap}
      />

      <DailyCapsCard
        caps={limits.per_platform_daily_cap}
        twitterAllowed={limits.plan_allows_twitter}
      />

      <UpgradeFooter />
    </div>
  );
}

function PlanPackagingCard({ limits }: { limits: ApiLimits }) {
  const rows = [
    {
      label: "API keys",
      detail: "Active, non-revoked keys",
      current: limits.current_api_keys,
      max: limits.max_api_keys,
    },
    {
      label: "Webhook endpoints",
      detail: "Active endpoints",
      current: limits.current_webhooks,
      max: limits.max_webhooks,
    },
    {
      label: "Managed accounts",
      detail: "Successful Hosted Connect accounts",
      current: limits.current_managed_accounts,
      max: limits.max_managed_accounts,
    },
    {
      label: "Managed users",
      detail: "Distinct completed external_user_id values",
      current: limits.current_managed_users,
      max: limits.max_managed_users,
    },
  ];

  return (
    <Card>
      <CardTitle>Plan packaging limits</CardTitle>
      <p style={{ margin: "6px 0 12px", fontSize: 12, color: "var(--dmuted)", lineHeight: 1.6 }}>
        Free plan caps apply to new API keys, active webhooks, and successful Hosted
        Connect completions. Connect Session create attempts are not capped by plan.
      </p>
      <div style={{ display: "flex", flexDirection: "column", gap: 0 }}>
        {rows.map((row) => (
          <PlanLimitRow key={row.label} {...row} />
        ))}
      </div>
    </Card>
  );
}

function PlanLimitRow({
  label,
  detail,
  current,
  max,
}: {
  label: string;
  detail: string;
  current: number;
  max: number;
}) {
  const capped = max >= 0;
  const atLimit = capped && current >= max;

  return (
    <div
      style={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        gap: 16,
        padding: "10px 0",
        borderTop: "1px solid var(--dborder)",
      }}
    >
      <div style={{ minWidth: 0 }}>
        <div style={{ fontSize: 13, color: "var(--dtext)", fontWeight: 600 }}>{label}</div>
        <div style={{ fontSize: 12, color: "var(--dmuted)", marginTop: 2 }}>{detail}</div>
      </div>
      <div
        style={{
          color: atLimit ? "#f87171" : "var(--dtext)",
          fontFamily: "var(--font-mono, ui-monospace)",
          fontSize: 13,
          fontWeight: 600,
          whiteSpace: "nowrap",
        }}
      >
        {formatPlanLimit(current, max)}
      </div>
    </div>
  );
}

function formatPlanLimit(current: number, max: number) {
  if (max < 0) return `${current.toLocaleString()} / Unlimited`;
  return `${current.toLocaleString()} / ${max.toLocaleString()}`;
}

function DailyCapsCard({
  caps,
  twitterAllowed,
}: {
  caps: Record<string, number>;
  twitterAllowed: boolean;
}) {
  const display = [
    { key: "twitter", label: "X / Twitter" },
    { key: "instagram", label: "Instagram" },
    { key: "facebook", label: "Facebook Page" },
    { key: "threads", label: "Threads" },
    { key: "linkedin", label: "LinkedIn" },
    { key: "bluesky", label: "Bluesky" },
    { key: "tiktok", label: "TikTok" },
    { key: "youtube", label: "YouTube" },
    { key: "pinterest", label: "Pinterest" },
  ];

  return (
    <Card>
      <CardTitle>Per-account daily safety caps</CardTitle>
      <p style={{ margin: "6px 0 12px", fontSize: 12, color: "var(--dmuted)", lineHeight: 1.6 }}>
        Each connected account can publish at most this many successful posts per UTC day.
        The cap exists to keep the account from being flagged for spam by the platform itself
        — failed posts never count, and limits reset at 00:00 UTC.
      </p>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "6px 16px" }}>
        {display.map((p) => {
          const cap = caps?.[p.key];
          if (cap == null) return null;
          const gated = p.key === "twitter" && !twitterAllowed;
          return (
            <div
              key={p.key}
              style={{
                display: "flex",
                justifyContent: "space-between",
                fontSize: 13,
                color: gated ? "var(--dmuted)" : "var(--dtext)",
              }}
            >
              <span>
                {p.label}
                {gated && (
                  <span style={{ marginLeft: 6, fontSize: 11, color: "var(--daccent)" }}>
                    · paid plan
                  </span>
                )}
              </span>
              <span style={{ fontFamily: "var(--font-mono, ui-monospace)" }}>
                {cap}/day
              </span>
            </div>
          );
        })}
      </div>
    </Card>
  );
}

function PlanCard({ planID }: { planID: string }) {
  return (
    <Card>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div>
          <CardTitle>Current plan</CardTitle>
          <div style={{ fontSize: 14, color: "var(--dtext)", marginTop: 4 }}>
            <span style={{ fontFamily: "var(--font-mono, ui-monospace)", fontWeight: 600 }}>
              {planID}
            </span>
          </div>
        </div>
        <Link
          href="/settings/billing"
          style={{
            fontSize: 12,
            color: "var(--dmuted)",
            textDecoration: "none",
            display: "inline-flex",
            alignItems: "center",
            gap: 4,
          }}
        >
          Manage <ArrowUpRight size={12} />
        </Link>
      </div>
    </Card>
  );
}

function LimitCard({
  title,
  primary,
  secondary,
  body,
}: {
  title: string;
  primary: string;
  secondary?: string;
  body: string;
}) {
  return (
    <Card>
      <CardTitle>{title}</CardTitle>
      <div style={{ display: "flex", alignItems: "baseline", gap: 12, marginTop: 6 }}>
        <span style={{ fontSize: 22, fontWeight: 700, color: "var(--dtext)" }}>{primary}</span>
        {secondary && (
          <span style={{ fontSize: 13, color: "var(--dmuted)" }}>· {secondary}</span>
        )}
      </div>
      <p style={{ margin: "10px 0 0", fontSize: 12, color: "var(--dmuted)", lineHeight: 1.6 }}>
        {body}
      </p>
    </Card>
  );
}

function DepthCard({
  current,
  cap,
  managedCap,
}: {
  current: number;
  cap: number;
  managedCap: number;
}) {
  const pct = cap > 0 ? Math.min(100, (current / cap) * 100) : 0;
  // Bar color tracks utilization: green under 60%, amber 60-90%, red over 90%.
  // Same thresholds the billing usage warning uses (see quota.Checker).
  const barColor = pct >= 90 ? "#f87171" : pct >= 60 ? "#fbbf24" : "var(--daccent)";

  return (
    <Card>
      <CardTitle>Queue depth</CardTitle>
      <div style={{ display: "flex", alignItems: "baseline", gap: 12, marginTop: 6 }}>
        <span style={{ fontSize: 22, fontWeight: 700, color: "var(--dtext)" }}>
          {current.toLocaleString()} / {cap.toLocaleString()}
        </span>
        <span style={{ fontSize: 13, color: "var(--dmuted)" }}>active jobs</span>
      </div>
      <div
        style={{
          marginTop: 10,
          height: 6,
          background: "var(--dborder)",
          borderRadius: 3,
          overflow: "hidden",
        }}
      >
        <div
          style={{
            width: `${pct}%`,
            height: "100%",
            background: barColor,
            transition: "width 0.4s ease, background 0.2s",
          }}
        />
      </div>
      <p style={{ margin: "10px 0 0", fontSize: 12, color: "var(--dmuted)", lineHeight: 1.6 }}>
        A job is "active" while pending / running / retrying. New publish requests are
        rejected when the workspace is at the cap — wait for the queue to drain or upgrade.
        Refreshes every 30 seconds.
      </p>
      <p style={{ margin: "8px 0 0", fontSize: 12, color: "var(--dmuted)", lineHeight: 1.6 }}>
        Per managed end user (Connect customers): up to{" "}
        <span style={{ color: "var(--dtext)", fontWeight: 600 }}>{managedCap}</span> active
        jobs.
      </p>
    </Card>
  );
}

function UpgradeFooter() {
  return (
    <div style={{ fontSize: 12, color: "var(--dmuted)", marginTop: 4 }}>
      Need higher limits?{" "}
      <Link href="/settings/billing" style={{ color: "var(--daccent)", textDecoration: "none" }}>
        Upgrade your plan
      </Link>
      .
    </div>
  );
}

function Card({ children }: { children: React.ReactNode }) {
  return (
    <div
      style={{
        border: "1px solid var(--dborder)",
        borderRadius: 8,
        padding: 16,
        background: "var(--dcard, transparent)",
      }}
    >
      {children}
    </div>
  );
}

function CardTitle({ children }: { children: React.ReactNode }) {
  return (
    <div style={{ fontSize: 13, fontWeight: 600, color: "var(--dtext)" }}>{children}</div>
  );
}
