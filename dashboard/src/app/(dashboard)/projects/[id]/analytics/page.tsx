"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import {
  listSocialPosts,
  getPostAnalytics,
  getAnalyticsSummary,
  getAnalyticsTrend,
  getAnalyticsByPlatform,
  type SocialPost,
  type SocialPostResult,
  type PostAnalytics,
  type AnalyticsSummary,
  type AnalyticsTrend,
  type PlatformAnalytics,
} from "@/lib/api";
import { PlatformIcon } from "@/components/platform-icons";
import {
  platformSupports,
  anyPlatformSupports,
  unsupportedReason,
  type MetricKey,
} from "@/lib/platform-capabilities";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  ResponsiveContainer,
} from "recharts";
import {
  BarChart3,
  Filter,
  RefreshCw,
  AlertTriangle,
  ChevronDown,
  ChevronRight,
  ExternalLink,
} from "lucide-react";

// ─── Helpers ───────────────────────────────────────────────────────────────

function formatNumber(n: number): string {
  if (!Number.isFinite(n) || n === 0) return "0";
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1).replace(/\.0$/, "") + "M";
  if (n >= 1_000) return (n / 1_000).toFixed(1).replace(/\.0$/, "") + "k";
  return n.toString();
}

function formatPercent(n: number): string {
  return (n * 100).toFixed(1) + "%";
}

// Renders a muted "N/A" with a tooltip explaining why a metric isn't
// available for a given platform. See dashboard/src/lib/platform-capabilities.ts.
function NACell({ platform, metric }: { platform: string; metric: MetricKey }) {
  return (
    <span
      title={unsupportedReason(platform, metric)}
      style={{ color: "var(--dmuted2)", cursor: "help", borderBottom: "1px dotted var(--dmuted2)" }}
    >
      N/A
    </span>
  );
}

function formatChange(n: number): { text: string; up: boolean; down: boolean } {
  if (n === 0) return { text: "--", up: false, down: false };
  const pct = (n * 100).toFixed(1) + "%";
  if (n > 0) return { text: `↑ ${pct}`, up: true, down: false };
  return { text: `↓ ${pct.replace("-", "")}`, up: false, down: true };
}

// Engagement rate color thresholds, PRD §11.2.
function engRateColor(rate: number): string {
  if (rate > 0.10) return "#10b981"; // green — excellent
  if (rate >= 0.05) return "var(--dtext)"; // default — normal
  if (rate >= 0.02) return "#f59e0b"; // yellow — low
  return "#ef4444"; // red — very low
}

type RangeKey = "7d" | "30d" | "90d" | "12m" | "custom";

function getRangeDates(
  range: RangeKey,
  custom: { start: string; end: string }
): { start: string; end: string } {
  const today = new Date();
  const fmt = (d: Date) => d.toISOString().slice(0, 10);
  if (range === "custom") {
    return { start: custom.start, end: custom.end };
  }
  const daysBack: Record<Exclude<RangeKey, "custom">, number> = {
    "7d": 7, "30d": 30, "90d": 90, "12m": 365,
  };
  const end = fmt(today);
  const start = new Date(today);
  start.setDate(start.getDate() - daysBack[range] + 1);
  return { start: fmt(start), end };
}

// Best-effort URL constructor for the per-post external link button.
// Returns null when the platform doesn't expose a stable post URL via its
// public ID (TikTok publish_id, for example, is opaque).
function postUrlFor(platform: string, externalId: string): string | null {
  switch (platform) {
    case "youtube":
      return `https://www.youtube.com/watch?v=${externalId}`;
    case "twitter":
      return `https://x.com/i/status/${externalId}`;
    case "instagram":
      return `https://www.instagram.com/p/${externalId}/`;
    case "threads":
      return `https://www.threads.net/post/${externalId}`;
    case "linkedin":
      if (externalId.startsWith("urn:li:")) {
        return `https://www.linkedin.com/feed/update/${externalId}/`;
      }
      return null;
    case "bluesky": {
      const m = externalId.match(/^at:\/\/(did:[^/]+)\/app\.bsky\.feed\.post\/(.+)$/);
      if (m) return `https://bsky.app/profile/${m[1]}/post/${m[2]}`;
      return null;
    }
    default:
      return null;
  }
}

// Sum a single post's per-account analytics rows into one combined metric set.
function sumPostMetrics(rows: PostAnalytics[]) {
  const acc = {
    impressions: 0, reach: 0, likes: 0, comments: 0, shares: 0,
    saves: 0, clicks: 0, video_views: 0,
  };
  rows.forEach((r) => {
    acc.impressions += r.impressions;
    acc.reach += r.reach;
    acc.likes += r.likes;
    acc.comments += r.comments;
    acc.shares += r.shares;
    acc.saves += r.saves;
    acc.clicks += r.clicks;
    acc.video_views += r.video_views;
  });
  const denom = acc.impressions || acc.video_views;
  const eng = denom > 0
    ? (acc.likes + acc.comments + acc.shares + acc.saves + acc.clicks) / denom
    : 0;
  return { ...acc, engagement_rate: eng };
}

// ─── Page ──────────────────────────────────────────────────────────────────

const PAGE_SIZE = 20;

const TREND_METRICS: { key: "posts" | "impressions" | "likes" | "comments" | "shares"; label: string; color: string }[] = [
  { key: "posts", label: "Posts", color: "#10b981" },
  { key: "impressions", label: "Impressions", color: "#0ea5e9" },
  { key: "likes", label: "Likes", color: "#f472b6" },
  { key: "comments", label: "Comments", color: "#a78bfa" },
  { key: "shares", label: "Shares", color: "#fb923c" },
];

type SortField = "published_at" | "impressions" | "likes" | "engagement";

export default function AnalyticsPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const { getToken } = useAuth();

  // Filters
  const [range, setRange] = useState<RangeKey>("30d");
  const [customRange, setCustomRange] = useState({ start: "", end: "" });
  const [platformFilter, setPlatformFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");
  const [trendMetric, setTrendMetric] = useState<typeof TREND_METRICS[number]["key"]>("posts");

  // Data
  const [summary, setSummary] = useState<AnalyticsSummary | null>(null);
  const [trend, setTrend] = useState<AnalyticsTrend | null>(null);
  const [byPlatform, setByPlatform] = useState<PlatformAnalytics[]>([]);
  const [posts, setPosts] = useState<SocialPost[]>([]);
  const [postAnalytics, setPostAnalytics] = useState<Record<string, PostAnalytics[]>>({});
  const [lastLoadedAt, setLastLoadedAt] = useState<Date | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  // Posts table state
  const [sortField, setSortField] = useState<SortField>("published_at");
  const [page, setPage] = useState(0);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  // Debounce timer for filter changes — coalesces rapid clicks.
  const debounceRef = useRef<NodeJS.Timeout | null>(null);

  const dateRange = useMemo(
    () => getRangeDates(range, customRange),
    [range, customRange]
  );

  // API params: snake_case dates plus platform/status filters that the
  // backend aggregation queries honor (empty / "all" disables them).
  const apiRange = useMemo(
    () => ({
      start_date: dateRange.start,
      end_date: dateRange.end,
      platform: platformFilter,
      status: statusFilter,
    }),
    [dateRange, platformFilter, statusFilter]
  );

  const reloadAll = useCallback(
    async (forceRefresh = false) => {
      try {
        const token = await getToken();
        if (!token) return;

        // Skip when custom range is half-filled.
        if (range === "custom" && (!customRange.start || !customRange.end)) {
          return;
        }

        if (forceRefresh) setRefreshing(true);
        else setLoading(true);

        // On a forceRefresh we MUST live-fetch per-post analytics BEFORE the
        // aggregation endpoints — the per-post `?refresh=1` calls update
        // post_analytics, which is what /summary, /trend, and /by-platform
        // read from. Doing it the other way leaves the user staring at stale
        // aggregations until they click Refresh again.
        //
        // On a normal load we don't need this ordering, since aggregations
        // and per-post both serve from the same cached table.
        if (forceRefresh) {
          const postsRes = await listSocialPosts(token, projectId);
          const published = (postsRes.data || []).filter((p) => p.status === "published");
          const results = await Promise.allSettled(
            published.map((p) => getPostAnalytics(token, projectId, p.id, { refresh: true }))
          );
          const map: Record<string, PostAnalytics[]> = {};
          results.forEach((r, i) => {
            if (r.status === "fulfilled" && r.value.data) {
              map[published[i].id] = r.value.data;
            }
          });
          setPosts(postsRes.data || []);
          setPostAnalytics(map);
        }

        const [summaryRes, trendRes, byPlatformRes, postsRes] = await Promise.all([
          getAnalyticsSummary(token, projectId, apiRange),
          getAnalyticsTrend(token, projectId, {
            ...apiRange,
            metric: "posts,impressions,likes,comments,shares",
          }),
          getAnalyticsByPlatform(token, projectId, apiRange),
          listSocialPosts(token, projectId),
        ]);

        setSummary(summaryRes.data);
        setTrend(trendRes.data);
        setByPlatform(byPlatformRes.data || []);
        setPosts(postsRes.data || []);
        setLastLoadedAt(new Date());

        // For non-refresh loads, fetch per-post analytics from cache after the
        // aggregations so the posts table can render the expand panels.
        if (!forceRefresh) {
          const published = (postsRes.data || []).filter((p) => p.status === "published");
          const results = await Promise.allSettled(
            published.map((p) => getPostAnalytics(token, projectId, p.id))
          );
          const map: Record<string, PostAnalytics[]> = {};
          results.forEach((r, i) => {
            if (r.status === "fulfilled" && r.value.data) {
              map[published[i].id] = r.value.data;
            }
          });
          setPostAnalytics(map);
        }
      } catch (err) {
        console.error("analytics: failed to load", err);
      } finally {
        setLoading(false);
        setRefreshing(false);
      }
    },
    [getToken, projectId, apiRange, range, customRange]
  );

  // Auto-load on filter change with 200ms debounce.
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => reloadAll(false), 200);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [reloadAll]);

  // ─── Derived: posts filtered by platform/status, joined with analytics ──

  const postRows = useMemo(() => {
    let filtered = posts;
    if (statusFilter !== "all") {
      filtered = filtered.filter((p) => p.status === statusFilter);
    }
    if (platformFilter !== "all") {
      filtered = filtered.filter((p) =>
        p.results?.some((r) => r.platform === platformFilter)
      );
    }

    const rows = filtered.map((post) => {
      const rows = postAnalytics[post.id] || [];
      const metrics = sumPostMetrics(rows);
      return { post, metrics, perAccount: rows };
    });

    rows.sort((a, b) => {
      switch (sortField) {
        case "impressions":
          return b.metrics.impressions - a.metrics.impressions;
        case "likes":
          return b.metrics.likes - a.metrics.likes;
        case "engagement":
          return b.metrics.engagement_rate - a.metrics.engagement_rate;
        case "published_at":
        default:
          return new Date(b.post.created_at).getTime() - new Date(a.post.created_at).getTime();
      }
    });

    return rows;
  }, [posts, postAnalytics, platformFilter, statusFilter, sortField]);

  const pagedRows = useMemo(
    () => postRows.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE),
    [postRows, page]
  );
  const totalPages = Math.max(1, Math.ceil(postRows.length / PAGE_SIZE));

  // Reset to page 0 when filters change so users don't end up on a phantom page.
  useEffect(() => {
    setPage(0);
  }, [platformFilter, statusFilter, sortField, range]);

  // ─── Render ────────────────────────────────────────────────────────────

  return (
    <>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 24 }}>
        <div>
          <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>Analytics</div>
          <div style={{ fontSize: 14, color: "#aaa", marginTop: 6 }}>
            Post performance and engagement metrics
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          {lastLoadedAt && (
            <span style={{ fontSize: 12, color: "var(--dmuted2)" }}>
              Last updated <RelativeTime date={lastLoadedAt} />
            </span>
          )}
          <button
            className="dbtn dbtn-ghost"
            onClick={() => reloadAll(true)}
            disabled={refreshing || loading}
            style={{ gap: 6 }}
            title="Force-refresh from each platform"
          >
            <RefreshCw
              style={{
                width: 13,
                height: 13,
                animation: refreshing ? "spin 1s linear infinite" : "none",
              }}
            />
            Refresh
          </button>
        </div>
      </div>

      {/* Filter Bar */}
      <FilterBar
        range={range}
        setRange={setRange}
        customRange={customRange}
        setCustomRange={setCustomRange}
        platformFilter={platformFilter}
        setPlatformFilter={setPlatformFilter}
        statusFilter={statusFilter}
        setStatusFilter={setStatusFilter}
        availablePlatforms={byPlatform.map((b) => b.platform)}
      />

      {loading && !summary && (
        <div style={{ color: "var(--dmuted)", padding: 40, textAlign: "center" }}>Loading...</div>
      )}

      {/* Empty state */}
      {!loading && summary && summary.posts.total === 0 && <EmptyState />}

      {summary && summary.posts.total > 0 && (
        <>
          {/* Failure-rate banner */}
          {summary.posts.failed_rate > 0.20 && (
            <FailureBanner rate={summary.posts.failed_rate} />
          )}

          {/* Layer 1: Summary Cards */}
          <SummaryCards summary={summary} />

          {/* Layer 2: Trend Chart */}
          {trend && (
            <TrendChart
              trend={trend}
              metric={trendMetric}
              setMetric={setTrendMetric}
            />
          )}

          {/* Layer 3: By Platform */}
          <ByPlatformTable rows={byPlatform} />

          {/* Layer 4: Posts Table */}
          <PostsTable
            rows={pagedRows}
            allRows={postRows}
            page={page}
            totalPages={totalPages}
            setPage={setPage}
            sortField={sortField}
            setSortField={setSortField}
            expanded={expanded}
            setExpanded={setExpanded}
          />
        </>
      )}

      <style>{`
        @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
      `}</style>
    </>
  );
}

// ─── FilterBar ─────────────────────────────────────────────────────────────

function FilterBar({
  range, setRange, customRange, setCustomRange,
  platformFilter, setPlatformFilter,
  statusFilter, setStatusFilter,
  availablePlatforms,
}: {
  range: RangeKey;
  setRange: (r: RangeKey) => void;
  customRange: { start: string; end: string };
  setCustomRange: (c: { start: string; end: string }) => void;
  platformFilter: string;
  setPlatformFilter: (s: string) => void;
  statusFilter: string;
  setStatusFilter: (s: string) => void;
  availablePlatforms: string[];
}) {
  return (
    <div style={{
      display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap",
      padding: "10px 14px", marginBottom: 24,
      background: "var(--surface)", border: "1px solid var(--dborder)", borderRadius: 8,
    }}>
      <Filter style={{ width: 13, height: 13, color: "var(--dmuted2)", flexShrink: 0 }} />

      <FilterSelect
        label="Range"
        value={range}
        onChange={(v) => setRange(v as RangeKey)}
        options={[
          { value: "7d", label: "Last 7 days" },
          { value: "30d", label: "Last 30 days" },
          { value: "90d", label: "Last 90 days" },
          { value: "12m", label: "Last 12 months" },
          { value: "custom", label: "Custom range" },
        ]}
      />

      {range === "custom" && (
        <>
          <input
            type="date"
            className="dform-input"
            style={{ width: "auto", fontSize: 12, padding: "5px 8px" }}
            value={customRange.start}
            max={customRange.end || undefined}
            onChange={(e) => setCustomRange({ ...customRange, start: e.target.value })}
          />
          <span style={{ fontSize: 11, color: "var(--dmuted2)" }}>to</span>
          <input
            type="date"
            className="dform-input"
            style={{ width: "auto", fontSize: 12, padding: "5px 8px" }}
            value={customRange.end}
            min={customRange.start || undefined}
            onChange={(e) => setCustomRange({ ...customRange, end: e.target.value })}
          />
        </>
      )}

      <div style={{ width: 1, height: 20, background: "var(--dborder)" }} />

      <FilterSelect
        label="Platform"
        value={platformFilter}
        onChange={setPlatformFilter}
        options={[
          { value: "all", label: "All platforms" },
          ...availablePlatforms.map((p) => ({ value: p, label: p.charAt(0).toUpperCase() + p.slice(1) })),
        ]}
      />

      <FilterSelect
        label="Status"
        value={statusFilter}
        onChange={setStatusFilter}
        options={[
          { value: "all", label: "All statuses" },
          { value: "published", label: "Published" },
          { value: "scheduled", label: "Scheduled" },
          { value: "failed", label: "Failed" },
          { value: "partial", label: "Partial" },
        ]}
      />
    </div>
  );
}

function FilterSelect({ label, value, onChange, options }: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 5 }}>
      <span style={{ fontSize: 10, fontWeight: 600, color: "var(--dmuted2)", textTransform: "uppercase", letterSpacing: "0.05em" }}>
        {label}
      </span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        style={{
          fontSize: 12, padding: "5px 8px",
          background: "var(--surface2)", color: "var(--dtext)",
          border: "1px solid var(--dborder2)", borderRadius: 6,
          cursor: "pointer", outline: "none", fontFamily: "inherit",
        }}
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>{o.label}</option>
        ))}
      </select>
    </div>
  );
}

// ─── Failure-rate banner ───────────────────────────────────────────────────

function FailureBanner({ rate }: { rate: number }) {
  const severe = rate > 0.50;
  const color = severe ? "#ef4444" : "#f59e0b";
  const message = severe
    ? "High failure rate detected. Check your account connections."
    : "Some posts failed. Review failed posts below.";

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 10,
        padding: "12px 16px",
        marginBottom: 20,
        background: severe ? "#ef444410" : "#f59e0b10",
        border: `1px solid ${severe ? "#ef444440" : "#f59e0b40"}`,
        borderRadius: 8,
        fontSize: 13,
        color,
      }}
    >
      <AlertTriangle style={{ width: 16, height: 16, flexShrink: 0 }} />
      <span>
        <strong>{formatPercent(rate)} failure rate.</strong> {message}
      </span>
    </div>
  );
}

// ─── SummaryCards ──────────────────────────────────────────────────────────

function SummaryCards({ summary }: { summary: AnalyticsSummary }) {
  const { posts, engagement, vs_previous_period: delta } = summary;
  const failedRateColor = posts.failed_rate > 0.20 ? "#f59e0b" : posts.failed_rate > 0.50 ? "#ef4444" : "var(--dmuted)";

  return (
    <>
      {/* Row 1: Post counts */}
      <div style={{ fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--dmuted2)", marginBottom: 8 }}>
        Post Overview
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 12, marginBottom: 24 }}>
        <KPICard label="Total Posts" value={formatNumber(posts.total)} />
        <KPICard
          label="Published"
          value={formatNumber(posts.published)}
          color="#10b981"
        />
        <KPICard
          label="Failed"
          value={formatNumber(posts.failed)}
          color={posts.failed > 0 ? "#ef4444" : "var(--dmuted)"}
          subtext={posts.failed > 0 ? `${formatPercent(posts.failed_rate)} rate` : undefined}
          subtextColor={failedRateColor}
        />
      </div>

      {/* Row 2: Engagement totals */}
      <div style={{ fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--dmuted2)", marginBottom: 8 }}>
        Engagement
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 12, marginBottom: 28 }}>
        <KPICard
          label="Total Impressions"
          value={engagement.impressions === 0 ? "--" : formatNumber(engagement.impressions)}
          change={delta.impressions_change}
          footnote="Twitter / LinkedIn / Threads only"
        />
        <KPICard
          label="Total Likes"
          value={engagement.likes === 0 ? "--" : formatNumber(engagement.likes)}
          change={delta.likes_change}
        />
        <KPICard
          label="Avg Engagement Rate"
          value={engagement.impressions === 0 ? "--" : formatPercent(engagement.engagement_rate)}
          color={engagement.impressions === 0 ? undefined : engRateColor(engagement.engagement_rate)}
          change={delta.engagement_change}
          footnote="Based on platforms exposing impressions"
        />
      </div>
    </>
  );
}

function KPICard({ label, value, color, change, subtext, subtextColor, footnote }: {
  label: string;
  value: string;
  color?: string;
  change?: number;
  subtext?: string;
  subtextColor?: string;
  footnote?: string;
}) {
  const ch = change !== undefined ? formatChange(change) : null;
  return (
    <div className="stat-card">
      <div style={{
        fontSize: 11,
        color: "var(--dmuted)",
        textTransform: "uppercase",
        letterSpacing: "0.06em",
        fontWeight: 600,
        marginBottom: 8,
      }}>
        {label}
      </div>
      <div style={{
        fontFamily: "var(--font-geist-mono), monospace",
        fontSize: 22,
        fontWeight: 600,
        color: color || "var(--dtext)",
        letterSpacing: -0.5,
        fontVariantNumeric: "tabular-nums",
        marginBottom: 4,
      }}>
        {value}
      </div>
      {ch && (
        <div style={{
          fontSize: 11,
          color: ch.up ? "#10b981" : ch.down ? "#ef4444" : "var(--dmuted2)",
        }}>
          {ch.text === "--" ? ch.text : `${ch.text} vs prev`}
        </div>
      )}
      {subtext && (
        <div style={{ fontSize: 11, color: subtextColor || "var(--dmuted2)" }}>
          ⚠ {subtext}
        </div>
      )}
      {footnote && (
        <div style={{ fontSize: 10, color: "var(--dmuted2)", marginTop: 2, fontStyle: "italic" }}>
          {footnote}
        </div>
      )}
    </div>
  );
}

// ─── TrendChart ────────────────────────────────────────────────────────────

function TrendChart({
  trend, metric, setMetric,
}: {
  trend: AnalyticsTrend;
  metric: typeof TREND_METRICS[number]["key"];
  setMetric: (m: typeof TREND_METRICS[number]["key"]) => void;
}) {
  const active = TREND_METRICS.find((m) => m.key === metric)!;

  // recharts wants an array of {name, value} per row.
  const data = useMemo(() => {
    const series = trend.series[metric] || [];
    return trend.dates.map((date, i) => ({
      date,
      value: series[i] ?? 0,
    }));
  }, [trend, metric]);

  return (
    <>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 8 }}>
        <div style={{ fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--dmuted2)" }}>
          Trend
        </div>
        <div style={{ display: "flex", gap: 4 }}>
          {TREND_METRICS.map((m) => (
            <button
              key={m.key}
              onClick={() => setMetric(m.key)}
              style={{
                fontSize: 11,
                padding: "4px 10px",
                borderRadius: 6,
                background: m.key === metric ? "var(--surface2)" : "transparent",
                color: m.key === metric ? "var(--dtext)" : "var(--dmuted)",
                border: `1px solid ${m.key === metric ? "var(--dborder2)" : "transparent"}`,
                cursor: "pointer",
                fontFamily: "inherit",
                fontWeight: 500,
              }}
            >
              {m.label}
            </button>
          ))}
        </div>
      </div>
      <div className="settings-section" style={{ marginBottom: 28 }}>
        <div className="settings-section-body" style={{ padding: 16 }}>
          <ResponsiveContainer width="100%" height={200}>
            <LineChart data={data} margin={{ top: 8, right: 16, bottom: 0, left: -16 }}>
              <CartesianGrid stroke="var(--dborder)" strokeDasharray="3 3" vertical={false} />
              <XAxis
                dataKey="date"
                tick={{ fill: "var(--dmuted2)", fontSize: 11 }}
                tickFormatter={(d: string) => {
                  const date = new Date(d);
                  return date.toLocaleDateString("en-US", { month: "short", day: "numeric" });
                }}
                interval="preserveStartEnd"
                minTickGap={40}
                axisLine={{ stroke: "var(--dborder)" }}
                tickLine={false}
              />
              <YAxis
                tick={{ fill: "var(--dmuted2)", fontSize: 11 }}
                axisLine={false}
                tickLine={false}
                tickFormatter={(v: number) => formatNumber(v)}
                width={48}
              />
              <Tooltip
                contentStyle={{
                  background: "var(--surface2)",
                  border: "1px solid var(--dborder2)",
                  borderRadius: 6,
                  fontSize: 12,
                  color: "var(--dtext)",
                }}
                labelStyle={{ color: "var(--dmuted)", marginBottom: 4 }}
                formatter={(v) => [formatNumber(Number(v)), active.label]}
              />
              <Line
                type="monotone"
                dataKey="value"
                stroke={active.color}
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 4 }}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>
    </>
  );
}

// ─── ByPlatformTable ──────────────────────────────────────────────────────

function ByPlatformTable({ rows }: { rows: PlatformAnalytics[] }) {
  const sorted = [...rows].sort((a, b) => b.impressions - a.impressions);

  return (
    <>
      <div style={{ fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--dmuted2)", marginBottom: 8 }}>
        By Platform
      </div>
      <div className="settings-section" style={{ marginBottom: 28 }}>
        <div className="settings-section-body">
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
            <thead>
              <tr>
                <th style={thStyle}>Platform</th>
                <th style={thRightStyle}>Posts</th>
                <th style={thRightStyle}>Impressions</th>
                <th style={thRightStyle}>Likes</th>
                <th style={thRightStyle}>Comments</th>
                <th style={thRightStyle}>Shares</th>
                <th style={thRightStyle}>Avg Eng.</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((r) => (
                <tr key={r.platform}>
                  <td style={tdStyle}>
                    <span style={{ display: "flex", alignItems: "center", gap: 8 }}>
                      <PlatformIcon platform={r.platform} size={14} />
                      <span style={{ fontWeight: 500, color: "var(--dtext)", textTransform: "capitalize" }}>{r.platform}</span>
                    </span>
                  </td>
                  <td style={tdRight}>{formatNumber(r.posts)}</td>
                  <td style={tdRight}>
                    {!platformSupports(r.platform, "impressions")
                      ? <NACell platform={r.platform} metric="impressions" />
                      : r.impressions === 0 ? "--" : formatNumber(r.impressions)}
                  </td>
                  <td style={tdRight}>{r.likes === 0 ? "--" : formatNumber(r.likes)}</td>
                  <td style={tdRight}>{r.comments === 0 ? "--" : formatNumber(r.comments)}</td>
                  <td style={tdRight}>{r.shares === 0 ? "--" : formatNumber(r.shares)}</td>
                  <td style={tdRight}>
                    {!platformSupports(r.platform, "impressions") ? (
                      <NACell platform={r.platform} metric="impressions" />
                    ) : (
                      <span style={{ color: r.impressions === 0 ? "var(--dmuted2)" : engRateColor(r.engagement_rate) }}>
                        {r.impressions === 0 ? "--" : formatPercent(r.engagement_rate)}
                      </span>
                    )}
                  </td>
                </tr>
              ))}
              {sorted.length === 0 && (
                <tr>
                  <td colSpan={7} style={{ ...tdStyle, textAlign: "center", color: "var(--dmuted2)" }}>
                    No platform data
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </>
  );
}

// ─── PostsTable ────────────────────────────────────────────────────────────

type PostRow = {
  post: SocialPost;
  metrics: ReturnType<typeof sumPostMetrics>;
  perAccount: PostAnalytics[];
};

function PostsTable({
  rows, allRows, page, totalPages, setPage,
  sortField, setSortField,
  expanded, setExpanded,
}: {
  rows: PostRow[];
  allRows: PostRow[];
  page: number;
  totalPages: number;
  setPage: (p: number) => void;
  sortField: SortField;
  setSortField: (s: SortField) => void;
  expanded: Set<string>;
  setExpanded: (s: Set<string>) => void;
}) {
  const toggleRow = (id: string) => {
    const next = new Set(expanded);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setExpanded(next);
  };

  return (
    <>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 8 }}>
        <div style={{ fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--dmuted2)" }}>
          Posts
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <span style={{ fontSize: 10, fontWeight: 600, color: "var(--dmuted2)", textTransform: "uppercase", letterSpacing: "0.05em" }}>
            Sort
          </span>
          <select
            value={sortField}
            onChange={(e) => setSortField(e.target.value as SortField)}
            style={{
              fontSize: 12, padding: "5px 8px",
              background: "var(--surface2)", color: "var(--dtext)",
              border: "1px solid var(--dborder2)", borderRadius: 6,
              cursor: "pointer", outline: "none", fontFamily: "inherit",
            }}
          >
            <option value="published_at">Newest first</option>
            <option value="impressions">Impressions</option>
            <option value="likes">Likes</option>
            <option value="engagement">Engagement</option>
          </select>
        </div>
      </div>

      <div className="settings-section">
        <div className="settings-section-body">
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
            <thead>
              <tr>
                <th style={{ ...thStyle, width: 28 }}></th>
                <th style={thStyle}>Caption</th>
                <th style={thStyle}>Platforms</th>
                <th style={thStyle}>Status</th>
                <th style={thRightStyle}>Impressions</th>
                <th style={thRightStyle}>Likes</th>
                <th style={thRightStyle}>Eng.</th>
                <th style={thStyle}>Published</th>
              </tr>
            </thead>
            <tbody>
              {rows.map(({ post, metrics, perAccount }) => {
                const isExpanded = expanded.has(post.id);
                const platforms = Array.from(new Set((post.results || []).map((r) => r.platform).filter(Boolean) as string[]));
                return (
                  <FragmentRow
                    key={post.id}
                    post={post}
                    metrics={metrics}
                    perAccount={perAccount}
                    platforms={platforms}
                    isExpanded={isExpanded}
                    onToggle={() => toggleRow(post.id)}
                  />
                );
              })}
              {rows.length === 0 && (
                <tr>
                  <td colSpan={8} style={{ ...tdStyle, textAlign: "center", color: "var(--dmuted2)" }}>
                    No posts in this range
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: 12 }}>
          <span style={{ fontSize: 12, color: "var(--dmuted2)" }}>
            Showing {page * PAGE_SIZE + 1}–{Math.min((page + 1) * PAGE_SIZE, allRows.length)} of {allRows.length}
          </span>
          <div style={{ display: "flex", gap: 6 }}>
            <button
              className="dbtn dbtn-ghost"
              disabled={page === 0}
              onClick={() => setPage(page - 1)}
              style={{ fontSize: 12, padding: "4px 10px" }}
            >
              Previous
            </button>
            <button
              className="dbtn dbtn-ghost"
              disabled={page + 1 >= totalPages}
              onClick={() => setPage(page + 1)}
              style={{ fontSize: 12, padding: "4px 10px" }}
            >
              Next
            </button>
          </div>
        </div>
      )}
    </>
  );
}

function FragmentRow({
  post, metrics, perAccount, platforms, isExpanded, onToggle,
}: {
  post: SocialPost;
  metrics: ReturnType<typeof sumPostMetrics>;
  perAccount: PostAnalytics[];
  platforms: string[];
  isExpanded: boolean;
  onToggle: () => void;
}) {
  const failed = post.status === "failed";
  const noImpressionPlatform = platforms.length > 0 && !anyPlatformSupports(platforms, "impressions");
  return (
    <>
      <tr
        onClick={onToggle}
        style={{ cursor: "pointer" }}
      >
        <td style={{ ...tdStyle, paddingRight: 0 }}>
          {isExpanded ? (
            <ChevronDown style={{ width: 14, height: 14, color: "var(--dmuted2)" }} />
          ) : (
            <ChevronRight style={{ width: 14, height: 14, color: "var(--dmuted2)" }} />
          )}
        </td>
        <td style={{ ...tdStyle, maxWidth: 280 }}>
          <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: "var(--dtext)", fontWeight: 500 }}>
            {post.caption || "(no caption)"}
          </span>
        </td>
        <td style={tdStyle}>
          <span style={{ display: "flex", alignItems: "center", gap: 6 }}>
            {platforms.map((p) => (
              <PlatformIcon key={p} platform={p} size={14} />
            ))}
          </span>
        </td>
        <td style={tdStyle}>
          <StatusPill status={post.status} />
        </td>
        <td style={tdRight}>
          {failed ? "--" : noImpressionPlatform ? (
            <span
              title="None of this post's platforms expose impressions via API"
              style={{ color: "var(--dmuted2)", cursor: "help", borderBottom: "1px dotted var(--dmuted2)" }}
            >
              N/A
            </span>
          ) : metrics.impressions === 0 ? "--" : formatNumber(metrics.impressions)}
        </td>
        <td style={tdRight}>{failed || metrics.likes === 0 ? "--" : formatNumber(metrics.likes)}</td>
        <td style={tdRight}>
          {failed ? (
            <span style={{ color: "var(--dmuted2)" }}>--</span>
          ) : noImpressionPlatform ? (
            <span
              title="Engagement rate needs impressions, which none of this post's platforms expose"
              style={{ color: "var(--dmuted2)", cursor: "help", borderBottom: "1px dotted var(--dmuted2)" }}
            >
              N/A
            </span>
          ) : metrics.impressions === 0 ? (
            <span style={{ color: "var(--dmuted2)" }}>--</span>
          ) : (
            <span style={{ color: engRateColor(metrics.engagement_rate) }}>
              {formatPercent(metrics.engagement_rate)}
            </span>
          )}
        </td>
        <td style={{ ...tdStyle, fontSize: 12, color: "var(--dmuted)" }}>
          {new Date(post.created_at).toLocaleDateString("en-US", { month: "short", day: "numeric" })}
        </td>
      </tr>
      {isExpanded && (
        <tr>
          <td colSpan={8} style={{ background: "var(--surface)", padding: "16px 24px", borderBottom: "1px solid var(--dborder)" }}>
            <PostExpandPanel results={post.results || []} perAccount={perAccount} />
          </td>
        </tr>
      )}
    </>
  );
}

// PostExpandPanel renders one card per social_account_id that the post was
// dispatched to. The source of truth for which accounts exist is the post's
// own results[] (always present after a publish attempt, including failed
// ones), joined in with PostAnalytics rows by social_account_id when the
// analytics worker has fetched them. This means:
//   - Failed accounts show their error_message even when there are no metrics.
//   - Published accounts always render even before analytics has run, with a
//     "Analytics not yet available" placeholder until metrics arrive.
function PostExpandPanel({
  results,
  perAccount,
}: {
  results: SocialPostResult[];
  perAccount: PostAnalytics[];
}) {
  if (results.length === 0) {
    return <div style={{ fontSize: 12, color: "var(--dmuted2)" }}>No accounts attached to this post.</div>;
  }

  // Build a quick lookup so we can attach metrics to the matching result row
  // without an O(n²) scan.
  const metricsByAccount = new Map<string, PostAnalytics>();
  for (const m of perAccount) {
    metricsByAccount.set(m.social_account_id, m);
  }

  return (
    <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(240px, 1fr))", gap: 16 }}>
      {results.map((res) => {
        const platform = res.platform || "";
        const metrics = metricsByAccount.get(res.social_account_id);
        const url = res.external_id ? postUrlFor(platform, res.external_id) : null;
        const isFailed = res.status === "failed";

        return (
          <div
            key={res.social_account_id}
            style={{
              padding: 14,
              background: "var(--surface2)",
              border: "1px solid var(--dborder)",
              borderRadius: 8,
              opacity: isFailed ? 0.95 : 1,
            }}
          >
            {/* Header: platform + status pill + (optional) external link */}
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 10, gap: 8 }}>
              <span style={{ display: "flex", alignItems: "center", gap: 6, minWidth: 0 }}>
                <PlatformIcon platform={platform} size={14} />
                <span style={{ fontWeight: 600, color: "var(--dtext)", textTransform: "capitalize", fontSize: 13 }}>
                  {platform || "unknown"}
                </span>
                <StatusPill status={res.status} />
              </span>
              {url && (
                <a
                  href={url}
                  target="_blank"
                  rel="noreferrer"
                  onClick={(e) => e.stopPropagation()}
                  style={{ color: "var(--dmuted)", display: "inline-flex", flexShrink: 0 }}
                  title="Open original post"
                >
                  <ExternalLink style={{ width: 12, height: 12 }} />
                </a>
              )}
            </div>

            {/* Failed: surface the platform's error message instead of metrics */}
            {isFailed && (
              <div
                style={{
                  fontSize: 12,
                  color: "#ef4444",
                  background: "#ef444410",
                  border: "1px solid #ef444430",
                  borderRadius: 6,
                  padding: "8px 10px",
                  whiteSpace: "pre-wrap",
                  wordBreak: "break-word",
                  fontFamily: "var(--mono, monospace)",
                  lineHeight: 1.5,
                }}
              >
                {res.error_message || "Publish failed (no error message reported)."}
              </div>
            )}

            {/* Published with no analytics yet: short placeholder */}
            {!isFailed && !metrics && (
              <div style={{ fontSize: 12, color: "var(--dmuted2)", padding: "4px 0" }}>
                Published. Analytics not yet available — refresh to fetch.
              </div>
            )}

            {/* Published with analytics: existing metric rows */}
            {!isFailed && metrics && (
              <>
                <MetricLine
                  label="Impressions"
                  value={metrics.impressions}
                  na={!platformSupports(platform, "impressions")}
                  naReason={unsupportedReason(platform, "impressions")}
                />
                {metrics.reach > 0 && <MetricLine label="Reach" value={metrics.reach} />}
                <MetricLine label="Likes" value={metrics.likes} />
                <MetricLine label="Comments" value={metrics.comments} />
                <MetricLine label="Shares" value={metrics.shares} />
                {metrics.saves > 0 && <MetricLine label="Saves" value={metrics.saves} />}
                {metrics.clicks > 0 && <MetricLine label="Clicks" value={metrics.clicks} />}
                {metrics.video_views > 0 && <MetricLine label="Video Views" value={metrics.video_views} />}
                <div style={{
                  marginTop: 8,
                  paddingTop: 8,
                  borderTop: "1px solid var(--dborder)",
                  display: "flex",
                  justifyContent: "space-between",
                  fontSize: 12,
                }}>
                  <span style={{ color: "var(--dmuted)" }}>Engagement</span>
                  {!platformSupports(platform, "impressions") ? (
                    <span
                      title={unsupportedReason(platform, "impressions")}
                      style={{ color: "var(--dmuted2)", fontWeight: 600, cursor: "help", borderBottom: "1px dotted var(--dmuted2)" }}
                    >
                      N/A
                    </span>
                  ) : (
                    <span style={{ color: metrics.impressions === 0 ? "var(--dmuted2)" : engRateColor(metrics.engagement_rate), fontWeight: 600 }}>
                      {metrics.impressions === 0 ? "--" : formatPercent(metrics.engagement_rate)}
                    </span>
                  )}
                </div>
              </>
            )}
          </div>
        );
      })}
    </div>
  );
}

function MetricLine({ label, value, na, naReason }: {
  label: string;
  value: number;
  na?: boolean;
  naReason?: string;
}) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", fontSize: 12, padding: "2px 0" }}>
      <span style={{ color: "var(--dmuted)" }}>{label}</span>
      {na ? (
        <span
          title={naReason}
          style={{ color: "var(--dmuted2)", cursor: "help", borderBottom: "1px dotted var(--dmuted2)" }}
        >
          N/A
        </span>
      ) : (
        <span style={{ color: "var(--dtext)", fontVariantNumeric: "tabular-nums" }}>{formatNumber(value)}</span>
      )}
    </div>
  );
}

function StatusPill({ status }: { status: string }) {
  const styles: Record<string, { bg: string; color: string; label: string }> = {
    published: { bg: "#10b98120", color: "#10b981", label: "published" },
    scheduled: { bg: "#0ea5e920", color: "#0ea5e9", label: "scheduled" },
    failed:    { bg: "#ef444420", color: "#ef4444", label: "failed" },
    partial:   { bg: "#f59e0b20", color: "#f59e0b", label: "partial" },
    publishing:{ bg: "#a78bfa20", color: "#a78bfa", label: "publishing" },
  };
  const s = styles[status] || { bg: "var(--surface2)", color: "var(--dmuted)", label: status };
  return (
    <span style={{
      display: "inline-block",
      padding: "2px 8px",
      borderRadius: 4,
      background: s.bg,
      color: s.color,
      fontSize: 11,
      fontWeight: 600,
      textTransform: "uppercase",
      letterSpacing: "0.04em",
    }}>
      {s.label}
    </span>
  );
}

// ─── EmptyState ────────────────────────────────────────────────────────────

const CURL_EXAMPLE = `curl -X POST https://api.unipost.dev/v1/social-posts \\
  -H "Authorization: Bearer up_live_xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "caption": "Hello from UniPost! 🚀",
    "account_ids": ["sa_instagram_123"]
  }'`;

function EmptyState() {
  return (
    <div className="empty-state" style={{ padding: 60, textAlign: "center" }}>
      <BarChart3 style={{ width: 32, height: 32, color: "var(--dmuted2)", marginBottom: 12 }} />
      <div style={{ fontSize: 14, fontWeight: 500, color: "var(--dtext)", marginBottom: 4 }}>
        No posts yet
      </div>
      <div style={{ fontSize: 12.5, color: "var(--dmuted)", marginBottom: 24 }}>
        Start posting via the API to see your analytics here.
      </div>
      <pre style={{
        display: "inline-block",
        textAlign: "left",
        background: "var(--surface2)",
        border: "1px solid var(--dborder)",
        borderRadius: 8,
        padding: "16px 20px",
        fontSize: 12,
        fontFamily: "var(--font-geist-mono), monospace",
        color: "var(--dtext)",
        lineHeight: 1.65,
        margin: 0,
      }}>
        {CURL_EXAMPLE}
      </pre>
    </div>
  );
}

// ─── RelativeTime ──────────────────────────────────────────────────────────

function RelativeTime({ date }: { date: Date }) {
  const [, force] = useState(0);
  useEffect(() => {
    const t = setInterval(() => force((n) => n + 1), 30_000);
    return () => clearInterval(t);
  }, []);
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000);
  if (seconds < 60) return <>just now</>;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return <>{minutes}m ago</>;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return <>{hours}h ago</>;
  return <>{Math.floor(hours / 24)}d ago</>;
}

// ─── Styles ────────────────────────────────────────────────────────────────

const thStyle: React.CSSProperties = {
  textAlign: "left",
  fontSize: 10,
  fontWeight: 600,
  color: "var(--dmuted)",
  textTransform: "uppercase",
  letterSpacing: "0.06em",
  padding: "0 12px 10px",
  borderBottom: "1px solid var(--dborder)",
};

const thRightStyle: React.CSSProperties = {
  ...thStyle,
  textAlign: "right",
};

const tdStyle: React.CSSProperties = {
  padding: "10px 12px",
  borderBottom: "1px solid var(--dborder)",
  color: "var(--dmuted)",
  fontVariantNumeric: "tabular-nums",
};

const tdRight: React.CSSProperties = {
  ...tdStyle,
  textAlign: "right",
};
