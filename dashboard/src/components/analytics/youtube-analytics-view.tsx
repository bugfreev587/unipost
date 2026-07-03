"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { CSSProperties } from "react";
import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import {
  Clock3,
  ExternalLink,
  Eye,
  ListVideo,
  MessageCircle,
  RefreshCw,
  ShieldCheck,
  ThumbsUp,
  Timer,
  TrendingUp,
  Users,
  Video,
  type LucideIcon,
} from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";
import {
  getAccountMetrics,
  getYouTubeAnalyticsSummary,
  getYouTubeAnalyticsTrend,
  getYouTubeAnalyticsVideos,
  listSocialAccounts,
  type AccountMetrics,
  type ApiResponse,
  type SocialAccount,
  type YouTubeAnalyticsSummary,
  type YouTubeAnalyticsTrend,
  type YouTubeAnalyticsTrendRow,
  type YouTubeAnalyticsVideoRow,
  type YouTubeAnalyticsVideos,
} from "@/lib/api";

const V1_REQUIRED_SCOPES = ["youtube.readonly"] as const;
const V2_REQUIRED_SCOPES = ["yt-analytics.readonly"] as const;
const REQUIRED_SCOPES = [...V1_REQUIRED_SCOPES, ...V2_REQUIRED_SCOPES] as const;

const tableStyle: CSSProperties = {
  width: "100%",
  borderCollapse: "collapse",
  fontSize: 13,
};

const thStyle: CSSProperties = {
  padding: "10px 12px",
  textAlign: "left",
  fontSize: 11,
  fontWeight: 600,
  letterSpacing: "0.06em",
  textTransform: "uppercase",
  color: "var(--dmuted2)",
  borderBottom: "1px solid var(--dborder)",
};

const thRightStyle: CSSProperties = {
  ...thStyle,
  textAlign: "right",
};

const tdStyle: CSSProperties = {
  padding: "12px",
  borderBottom: "1px solid var(--dborder)",
  color: "var(--dtext)",
  verticalAlign: "middle",
};

const tdRightStyle: CSSProperties = {
  ...tdStyle,
  textAlign: "right",
  fontFamily: "var(--font-geist-mono), monospace",
  fontSize: 12.5,
};

function formatNumber(n: number | undefined): string {
  if (!Number.isFinite(n || 0) || !n) return "0";
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1).replace(/\.0$/, "")}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1).replace(/\.0$/, "")}k`;
  return String(n);
}

function formatDate(value?: string): string {
  if (!value) return "-";
  const date = new Date(`${value}T00:00:00Z`);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function formatDateTime(value?: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return date.toLocaleString("en-US", { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
}

function formatWatchTime(minutes?: number): string {
  const value = Number.isFinite(minutes || 0) ? minutes || 0 : 0;
  if (value >= 60) return `${formatNumber(value / 60)}h`;
  return `${formatNumber(value)}m`;
}

function formatDuration(seconds?: number): string {
  const value = Math.max(0, Math.round(seconds || 0));
  if (!value) return "0s";
  const minutes = Math.floor(value / 60);
  const remainingSeconds = value % 60;
  if (!minutes) return `${remainingSeconds}s`;
  return `${minutes}m ${String(remainingSeconds).padStart(2, "0")}s`;
}

function recordNumber(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function hasScope(account: SocialAccount | undefined, scope: string): boolean {
  const values = account?.scope || [];
  const fullScope = `https://www.googleapis.com/auth/${scope}`;
  return values.some((value) => value === scope || value === fullScope || value.endsWith(`/auth/${scope}`));
}

function errorMessage(reason: unknown): string {
  return reason instanceof Error ? reason.message : "upstream error";
}

export function YouTubeAnalyticsView({ profileId }: { profileId: string }) {
  const { getToken } = useAuth();
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [selectedAccountId, setSelectedAccountId] = useState("");
  const [metrics, setMetrics] = useState<AccountMetrics | null>(null);
  const [summary, setSummary] = useState<YouTubeAnalyticsSummary | null>(null);
  const [trend, setTrend] = useState<YouTubeAnalyticsTrend | null>(null);
  const [videos, setVideos] = useState<YouTubeAnalyticsVideos | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");
  const [notices, setNotices] = useState<string[]>([]);

  const selectedAccount = useMemo(
    () => accounts.find((account) => account.id === selectedAccountId) || accounts[0],
    [accounts, selectedAccountId]
  );

  const scopeState = useMemo(() => {
    const scopes = selectedAccount?.scope || [];
    if (scopes.length === 0) {
      return { unknown: true, missingV1: [] as string[], missingV2: [] as string[] };
    }
    return {
      unknown: false,
      missingV1: V1_REQUIRED_SCOPES.filter((scope) => !hasScope(selectedAccount, scope)),
      missingV2: V2_REQUIRED_SCOPES.filter((scope) => !hasScope(selectedAccount, scope)),
    };
  }, [selectedAccount]);

  const loadData = useCallback(async () => {
    try {
      setError("");
      setNotices([]);
      setLoading((wasLoading) => wasLoading || accounts.length === 0);
      setRefreshing(true);

      const token = await getToken();
      if (!token) return;

      const accountRes = await listSocialAccounts(token, profileId, { platform: "youtube" });
      const youtubeAccounts = accountRes.data || [];
      setAccounts(youtubeAccounts);
      const account = youtubeAccounts.find((item) => item.id === selectedAccountId) || youtubeAccounts[0];
      if (!account) {
        setMetrics(null);
        setSummary(null);
        setTrend(null);
        setVideos(null);
        return;
      }
      if (account.id !== selectedAccountId) setSelectedAccountId(account.id);

      const settled = await Promise.allSettled([
        getAccountMetrics(token, profileId, account.id),
        getYouTubeAnalyticsSummary(token, profileId, account.id),
        getYouTubeAnalyticsTrend(token, profileId, account.id),
        getYouTubeAnalyticsVideos(token, profileId, account.id, { limit: 25 }),
      ]) as [
        PromiseSettledResult<ApiResponse<AccountMetrics>>,
        PromiseSettledResult<ApiResponse<YouTubeAnalyticsSummary>>,
        PromiseSettledResult<ApiResponse<YouTubeAnalyticsTrend>>,
        PromiseSettledResult<ApiResponse<YouTubeAnalyticsVideos>>,
      ];

      const [metricsRes, summaryRes, trendRes, videosRes] = settled;
      const nextNotices: string[] = [];

      if (metricsRes.status === "fulfilled") {
        setMetrics(metricsRes.value.data);
      } else {
        setMetrics(null);
        nextNotices.push(`Basic channel metrics unavailable: ${errorMessage(metricsRes.reason)}`);
      }

      if (summaryRes.status === "fulfilled") {
        setSummary(summaryRes.value.data);
      } else {
        setSummary(null);
        nextNotices.push(`Analytics report unavailable: ${errorMessage(summaryRes.reason)}`);
      }

      if (trendRes.status === "fulfilled") {
        setTrend(trendRes.value.data);
      } else {
        setTrend(null);
        nextNotices.push(`Daily trend unavailable: ${errorMessage(trendRes.reason)}`);
      }

      if (videosRes.status === "fulfilled") {
        setVideos(videosRes.value.data);
      } else {
        setVideos(null);
        nextNotices.push(`Top videos unavailable: ${errorMessage(videosRes.reason)}`);
      }

      setNotices(nextNotices);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load YouTube Analytics");
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [accounts.length, getToken, profileId, selectedAccountId]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const lifetimeViews = recordNumber(metrics?.platform_specific?.view_count);
  const analyticsMetrics = summary?.metrics;
  const stats = [
    { label: "Subscribers", value: metrics?.follower_count || 0, icon: Users, caption: "V1 account metrics" },
    { label: "Public videos", value: metrics?.post_count || 0, icon: ListVideo, caption: "YouTube Data API" },
    { label: "Channel views", value: lifetimeViews, icon: Eye, caption: "Lifetime channel views" },
    { label: "Watch time", value: formatWatchTime(analyticsMetrics?.estimated_minutes_watched), icon: Clock3, caption: "V2 report range" },
  ];

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 20, marginBottom: 24 }}>
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 8 }}>
            <div className="platform-icon-wrap"><PlatformIcon platform="youtube" /></div>
            <div className="dt-page-title">YouTube Analytics</div>
          </div>
          <div className="dt-subtitle" style={{ maxWidth: 760 }}>
            Basic channel metrics from the YouTube Data API plus owner-authorized YouTube Analytics API reports.
          </div>
        </div>
        <button className="dbtn dbtn-ghost" type="button" onClick={() => loadData()} disabled={refreshing}>
          <RefreshCw style={{ width: 14, height: 14 }} />
          {refreshing ? "Refreshing..." : "Refresh"}
        </button>
      </div>

      {error && <Notice tone="danger" message={error} />}
      {notices.map((message) => <Notice key={message} tone="muted" message={message} />)}

      {accounts.length > 1 && (
        <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 16 }}>
          <span className="dt-label" style={{ color: "var(--dmuted2)" }}>Account</span>
          <select
            value={selectedAccount?.id || ""}
            onChange={(event) => setSelectedAccountId(event.target.value)}
            style={{ padding: "7px 10px", border: "1px solid var(--dborder)", borderRadius: 6, background: "var(--surface1)", color: "var(--dtext)" }}
          >
            {accounts.map((account) => (
              <option key={account.id} value={account.id}>{account.account_name || account.id}</option>
            ))}
          </select>
        </div>
      )}

      {loading ? (
        <div style={{ color: "var(--dmuted)", padding: 40, textAlign: "center" }}>Loading YouTube Analytics...</div>
      ) : !selectedAccount ? (
        <EmptyState />
      ) : (
        <>
          <ScopeReadiness unknown={scopeState.unknown} missingV1={scopeState.missingV1} missingV2={scopeState.missingV2} />
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(min(100%, 360px), 1fr))", gap: 16, marginBottom: 24 }}>
            <ChannelPanel account={selectedAccount} metrics={metrics} summary={summary} />
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(130px, 1fr))", gap: 12 }}>
              {stats.map((item) => <MetricCard key={item.label} {...item} />)}
            </div>
          </div>
          <AnalyticsReport summary={summary} />
          <TrendTable rows={trend?.rows || []} />
          <TopVideosTable rows={videos?.videos || []} />
        </>
      )}
    </>
  );
}

function Notice({ tone, message }: { tone: "danger" | "muted"; message: string }) {
  const danger = tone === "danger";
  return (
    <div style={{
      marginBottom: 16,
      padding: "10px 12px",
      border: `1px solid ${danger ? "color-mix(in srgb, var(--danger) 24%, transparent)" : "var(--dborder)"}`,
      borderRadius: 8,
      color: danger ? "var(--danger)" : "var(--dmuted)",
      background: danger ? "var(--danger-soft)" : "var(--surface2)",
      fontSize: 13,
    }}>
      {message}
    </div>
  );
}

function EmptyState() {
  return (
    <div style={{ textAlign: "center", padding: 60, color: "var(--dmuted)" }}>
      <Video style={{ width: 40, height: 40, margin: "0 auto 12px", opacity: 0.3 }} />
      <div style={{ fontSize: 15, fontWeight: 600, color: "var(--dtext)", marginBottom: 6 }}>No YouTube account connected</div>
      <div style={{ fontSize: 13 }}>Connect a YouTube channel before using platform analytics.</div>
    </div>
  );
}

function ScopeReadiness({
  unknown,
  missingV1,
  missingV2,
}: {
  unknown: boolean;
  missingV1: readonly string[];
  missingV2: readonly string[];
}) {
  const ready = !unknown && missingV1.length === 0 && missingV2.length === 0;
  const title = unknown
    ? "Scope status unavailable"
    : missingV1.length > 0
      ? "Reconnect required for basic YouTube metrics"
      : missingV2.length > 0
        ? "Reconnect required for YouTube Analytics"
        : "YouTube analytics scopes ready";
  const detail = unknown
    ? REQUIRED_SCOPES.join(", ")
    : ready
      ? REQUIRED_SCOPES.join(", ")
      : `Missing: ${[...missingV1, ...missingV2].join(", ")}`;

  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 16, padding: "12px 14px", borderRadius: 8, border: "1px solid var(--dborder)", background: "var(--surface1)" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 10, minWidth: 0 }}>
        <ShieldCheck style={{ width: 18, height: 18, color: ready ? "var(--success)" : "var(--warning)" }} />
        <div>
          <div style={{ color: "var(--dtext)", fontSize: 13, fontWeight: 650 }}>{title}</div>
          <div style={{ color: "var(--dmuted)", fontSize: 12, marginTop: 3 }}>{detail}</div>
        </div>
      </div>
      <span className={`dbadge ${ready ? "dbadge-green" : "dbadge-amber"}`}><span className="dbadge-dot" />{ready ? "Ready" : "Reconnect"}</span>
    </div>
  );
}

function ChannelPanel({
  account,
  metrics,
  summary,
}: {
  account: SocialAccount;
  metrics: AccountMetrics | null;
  summary: YouTubeAnalyticsSummary | null;
}) {
  const channelName = account.account_name || "YouTube channel";
  const channelUrl = account.external_account_id ? `https://www.youtube.com/channel/${account.external_account_id}` : "";
  const hiddenSubscriberCount = Boolean(metrics?.platform_specific?.hidden_subscriber_count);
  const subscriberRounded = Boolean(metrics?.platform_specific?.subscriber_count_rounded);

  return (
    <div className="settings-section" style={{ marginBottom: 0 }}>
      <div className="settings-section-header">
        <div>
          <div className="settings-section-title">Basic channel metrics</div>
          <div className="settings-section-desc">V1 from GET /v1/accounts/{"{account_id}"}/metrics</div>
        </div>
      </div>
      <div className="settings-section-body">
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 16 }}>
          <ChannelAvatar src={account.account_avatar_url || ""} label={channelName} />
          <div style={{ minWidth: 0 }}>
            <div style={{ color: "var(--dtext)", fontWeight: 700 }}>{channelName}</div>
            <div style={{ color: "var(--dmuted)", fontSize: 12, fontFamily: "var(--font-geist-mono), monospace", overflow: "hidden", textOverflow: "ellipsis" }}>
              {account.external_account_id || account.id}
            </div>
          </div>
        </div>
        <div style={{ display: "grid", gap: 9, color: "var(--dmuted)", fontSize: 13 }}>
          <div>Last metrics fetch: {formatDateTime(metrics?.fetched_at)}</div>
          <div>Analytics window: {summary ? `${summary.start_date} to ${summary.end_date}` : "Unavailable until YouTube Analytics responds"}</div>
          <div>Subscriber count: {hiddenSubscriberCount ? "Hidden by channel settings" : subscriberRounded ? "Rounded by YouTube" : "Exact when YouTube returns it"}</div>
          {channelUrl && (
            <Link href={channelUrl} target="_blank" rel="noreferrer" style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--daccent)", textDecoration: "none" }}>
              <ExternalLink style={{ width: 14, height: 14 }} />
              Open channel
            </Link>
          )}
        </div>
      </div>
    </div>
  );
}

function ChannelAvatar({ src, label }: { src: string; label: string }) {
  const [failedSrc, setFailedSrc] = useState("");
  const showImage = src && failedSrc !== src;
  return (
    <div style={{ width: 48, height: 48, borderRadius: "50%", background: "linear-gradient(135deg, #dc2626, #111827)", display: "grid", placeItems: "center", color: "white", fontWeight: 700, overflow: "hidden", flexShrink: 0 }}>
      {showImage ? (
        <img
          src={src}
          alt=""
          onError={() => setFailedSrc(src)}
          style={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }}
        />
      ) : (
        label.slice(0, 2).toUpperCase()
      )}
    </div>
  );
}

function MetricCard({ label, value, caption, icon: Icon }: { label: string; value: number | string; caption: string; icon: LucideIcon }) {
  const rendered = typeof value === "number" ? formatNumber(value) : value;
  return (
    <div style={{ border: "1px solid var(--dborder)", background: "var(--surface1)", borderRadius: 8, padding: "14px 16px", minHeight: 104, display: "flex", flexDirection: "column", justifyContent: "space-between" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10 }}>
        <span style={{ color: "var(--dmuted2)", fontSize: 10, fontWeight: 600, letterSpacing: "0.06em", textTransform: "uppercase" }}>{label}</span>
        <Icon style={{ width: 16, height: 16, color: "var(--dmuted2)" }} />
      </div>
      <div style={{ fontSize: 24, fontWeight: 700, color: "var(--dtext)", letterSpacing: 0, fontFamily: "var(--font-geist-mono), monospace" }}>{rendered}</div>
      <div style={{ color: "var(--dmuted2)", fontSize: 11 }}>{caption}</div>
    </div>
  );
}

function AnalyticsReport({ summary }: { summary: YouTubeAnalyticsSummary | null }) {
  const metrics = summary?.metrics;
  const reportStats = [
    { label: "Views", value: metrics?.views || 0, icon: Eye },
    { label: "Likes", value: metrics?.likes || 0, icon: ThumbsUp },
    { label: "Comments", value: metrics?.comments || 0, icon: MessageCircle },
    { label: "Avg duration", value: formatDuration(metrics?.average_view_duration), icon: Timer },
    { label: "Subscribers gained", value: metrics?.subscribers_gained || 0, icon: TrendingUp },
    { label: "Subscribers lost", value: metrics?.subscribers_lost || 0, icon: Users },
  ];

  return (
    <div style={{ marginBottom: 28 }}>
      <div style={{ color: "var(--dmuted2)", fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", marginBottom: 8 }}>Analytics report</div>
      <div className="settings-section">
        <div className="settings-section-body">
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(135px, 1fr))", gap: 12 }}>
            {reportStats.map((item) => <MetricCard key={item.label} label={item.label} value={item.value} caption="yt-analytics.readonly" icon={item.icon} />)}
          </div>
          {!summary && (
            <div style={{ color: "var(--dmuted2)", fontSize: 13, marginTop: 14 }}>Reconnect with yt-analytics.readonly to load date-ranged YouTube Analytics reports.</div>
          )}
        </div>
      </div>
    </div>
  );
}

function TrendTable({ rows }: { rows: YouTubeAnalyticsTrendRow[] }) {
  return (
    <div style={{ marginBottom: 28 }}>
      <div style={{ color: "var(--dmuted2)", fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", marginBottom: 8 }}>Daily trend</div>
      <div className="settings-section">
        <div className="settings-section-body">
          <table style={tableStyle}>
            <thead>
              <tr>
                <th style={thStyle}>Date</th>
                <th style={thRightStyle}>Views</th>
                <th style={thRightStyle}>Watch time</th>
                <th style={thRightStyle}>Likes</th>
                <th style={thRightStyle}>Comments</th>
                <th style={thRightStyle}>Subscribers gained</th>
              </tr>
            </thead>
            <tbody>
              {rows.slice(0, 31).map((row) => (
                <tr key={row.date}>
                  <td style={{ ...tdStyle, fontWeight: 600 }}>{formatDate(row.date)}</td>
                  <td style={tdRightStyle}>{formatNumber(row.metrics.views)}</td>
                  <td style={tdRightStyle}>{formatWatchTime(row.metrics.estimated_minutes_watched)}</td>
                  <td style={tdRightStyle}>{formatNumber(row.metrics.likes)}</td>
                  <td style={tdRightStyle}>{formatNumber(row.metrics.comments)}</td>
                  <td style={tdRightStyle}>{formatNumber(row.metrics.subscribers_gained)}</td>
                </tr>
              ))}
              {rows.length === 0 && (
                <tr><td colSpan={6} style={{ ...tdStyle, textAlign: "center", color: "var(--dmuted2)", padding: 24 }}>No YouTube Analytics daily rows returned.</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

function TopVideosTable({ rows }: { rows: YouTubeAnalyticsVideoRow[] }) {
  return (
    <div>
      <div style={{ color: "var(--dmuted2)", fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", marginBottom: 8 }}>Top videos</div>
      <div className="settings-section">
        <div className="settings-section-body">
          <table style={tableStyle}>
            <thead>
              <tr>
                <th style={thStyle}>Video</th>
                <th style={thRightStyle}>Views</th>
                <th style={thRightStyle}>Watch time</th>
                <th style={thRightStyle}>Avg duration</th>
                <th style={thRightStyle}>Likes</th>
                <th style={thRightStyle}>Comments</th>
                <th style={thRightStyle}>Shares</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={row.video_id}>
                  <td style={tdStyle}>
                    <Link href={`https://www.youtube.com/watch?v=${row.video_id}`} target="_blank" rel="noreferrer" style={{ display: "inline-flex", alignItems: "center", gap: 8, color: "var(--daccent)", textDecoration: "none" }}>
                      <Video style={{ width: 14, height: 14 }} />
                      <span style={{ fontFamily: "var(--font-geist-mono), monospace", fontSize: 12 }}>{row.video_id}</span>
                    </Link>
                  </td>
                  <td style={tdRightStyle}>{formatNumber(row.metrics.views)}</td>
                  <td style={tdRightStyle}>{formatWatchTime(row.metrics.estimated_minutes_watched)}</td>
                  <td style={tdRightStyle}>{formatDuration(row.metrics.average_view_duration)}</td>
                  <td style={tdRightStyle}>{formatNumber(row.metrics.likes)}</td>
                  <td style={tdRightStyle}>{formatNumber(row.metrics.comments)}</td>
                  <td style={tdRightStyle}>{formatNumber(row.metrics.shares)}</td>
                </tr>
              ))}
              {rows.length === 0 && (
                <tr><td colSpan={7} style={{ ...tdStyle, textAlign: "center", color: "var(--dmuted2)", padding: 24 }}>No YouTube Analytics video rows returned.</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
