"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { CSSProperties, ReactNode } from "react";
import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import {
  BadgeCheck,
  ExternalLink,
  FileText,
  Heart,
  MessageCircle,
  RefreshCw,
  Repeat2,
  ShieldCheck,
  Users,
  type LucideIcon,
} from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";
import {
  getAccountMetrics,
  getInstagramMedia,
  getInstagramProfile,
  getPostAnalytics,
  getThreadsPosts,
  getThreadsProfile,
  listSocialAccounts,
  listSocialPosts,
  type AccountMetrics,
  type ApiResponse,
  type InstagramMedia,
  type InstagramMediaResponse,
  type InstagramProfile,
  type PostAnalytics,
  type SocialAccount,
  type SocialPost,
  type ThreadsPost,
  type ThreadsPostsResponse,
  type ThreadsProfile,
} from "@/lib/api";

type MetaAnalyticsPlatform = "instagram" | "threads";
type MetaProfile = InstagramProfile | ThreadsProfile;

type UniPostPostRow = {
  title: string;
  status: string;
  externalId: string;
  primaryMetric: number;
  likes: number;
  comments: number;
  shares: number;
};

const CONFIG = {
  instagram: {
    label: "Instagram",
    title: "Instagram Analytics",
    subtitle: "Business profile, account statistics, recent media, and UniPost-published post performance.",
    requiredScopes: ["instagram_business_basic", "instagram_business_manage_insights"],
    contentTitle: "Recent Instagram media",
    contentDesc: "Owned media returned by Instagram Business Login.",
    primaryMetric: "Reach",
    profileBase: "https://www.instagram.com/",
  },
  threads: {
    label: "Threads",
    title: "Threads Analytics",
    subtitle: "Profile, account insights, recent Threads posts, and UniPost-published post performance.",
    requiredScopes: ["threads_basic", "threads_manage_insights"],
    contentTitle: "Recent Threads posts",
    contentDesc: "Owned Threads posts returned by the Threads API.",
    primaryMetric: "Views",
    profileBase: "https://www.threads.net/@",
  },
} as const;

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
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1).replace(/\.0$/, "") + "M";
  if (n >= 1_000) return (n / 1_000).toFixed(1).replace(/\.0$/, "") + "k";
  return String(n);
}

function formatDate(value?: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return date.toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
}

function recordNumber(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function usernameFromProfile(profile: MetaProfile | null, account: SocialAccount | undefined) {
  return (profile?.username || account?.account_name || "").replace(/^@/, "").trim();
}

export function MetaPlatformAnalyticsView({
  profileId,
  platform,
}: {
  profileId: string;
  platform: MetaAnalyticsPlatform;
}) {
  const { getToken } = useAuth();
  const config = CONFIG[platform];
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [selectedAccountId, setSelectedAccountId] = useState("");
  const [profile, setProfile] = useState<MetaProfile | null>(null);
  const [metrics, setMetrics] = useState<AccountMetrics | null>(null);
  const [instagramMedia, setInstagramMedia] = useState<InstagramMedia[]>([]);
  const [threadsPosts, setThreadsPosts] = useState<ThreadsPost[]>([]);
  const [postRows, setPostRows] = useState<UniPostPostRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");
  const [notices, setNotices] = useState<string[]>([]);

  const selectedAccount = useMemo(
    () => accounts.find((account) => account.id === selectedAccountId) || accounts[0],
    [accounts, selectedAccountId]
  );

  const scopeState = useMemo(() => {
    const storedScopes = selectedAccount?.scope || [];
    if (storedScopes.length === 0) return { unknown: true, missing: [] as string[] };
    const granted = new Set(storedScopes);
    return {
      unknown: false,
      missing: config.requiredScopes.filter((scope) => !granted.has(scope)),
    };
  }, [config.requiredScopes, selectedAccount?.scope]);

  const loadData = useCallback(async () => {
    try {
      setError("");
      setNotices([]);
      setLoading((wasLoading) => wasLoading || accounts.length === 0);
      setRefreshing(true);
      const token = await getToken();
      if (!token) return;

      const accountRes = await listSocialAccounts(token, profileId, { platform });
      const platformAccounts = accountRes.data || [];
      setAccounts(platformAccounts);
      const account = platformAccounts.find((a) => a.id === selectedAccountId) || platformAccounts[0];
      if (!account) {
        setProfile(null);
        setMetrics(null);
        setInstagramMedia([]);
        setThreadsPosts([]);
        setPostRows([]);
        return;
      }
      if (account.id !== selectedAccountId) setSelectedAccountId(account.id);

      const profilePromise: Promise<ApiResponse<MetaProfile>> = platform === "instagram"
        ? getInstagramProfile(token, profileId, account.id)
        : getThreadsProfile(token, profileId, account.id);
      const contentPromise = platform === "instagram"
        ? getInstagramMedia(token, profileId, account.id, { limit: 20 })
        : getThreadsPosts(token, profileId, account.id, { limit: 20 });

      const [profileRes, metricsRes, contentRes, postsRes] = await Promise.allSettled([
        profilePromise,
        getAccountMetrics(token, profileId, account.id),
        contentPromise,
        listSocialPosts(token),
      ]);

      const nextNotices: string[] = [];
      if (profileRes.status === "fulfilled") {
        setProfile(profileRes.value.data);
      } else {
        setProfile(null);
        nextNotices.push(`Profile unavailable: ${profileRes.reason instanceof Error ? profileRes.reason.message : "upstream error"}`);
      }

      if (metricsRes.status === "fulfilled") {
        setMetrics(metricsRes.value.data);
      } else {
        setMetrics(null);
        nextNotices.push(`Account metrics unavailable: ${metricsRes.reason instanceof Error ? metricsRes.reason.message : "upstream error"}`);
      }

      if (contentRes.status === "fulfilled") {
        if (platform === "instagram") {
          setInstagramMedia((contentRes.value.data as InstagramMediaResponse).media || []);
          setThreadsPosts([]);
        } else {
          setThreadsPosts((contentRes.value.data as ThreadsPostsResponse).posts || []);
          setInstagramMedia([]);
        }
      } else {
        setInstagramMedia([]);
        setThreadsPosts([]);
        nextNotices.push(`Recent content unavailable: ${contentRes.reason instanceof Error ? contentRes.reason.message : "upstream error"}`);
      }

      if (postsRes.status === "fulfilled") {
        const published = (postsRes.value.data || []).filter((post) =>
          post.status === "published" &&
          post.results?.some((result) => result.social_account_id === account.id)
        );
        const analyticsSettled = await Promise.allSettled(
          published.map((post) => getPostAnalytics(token, post.id))
        );
        setPostRows(buildPostRows(platform, published, analyticsSettled, account.id));
      } else {
        setPostRows([]);
        nextNotices.push(`UniPost post analytics unavailable: ${postsRes.reason instanceof Error ? postsRes.reason.message : "upstream error"}`);
      }
      setNotices(nextNotices);
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to load ${config.label} analytics`);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [accounts.length, config.label, getToken, platform, profileId, selectedAccountId]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const nativeTotals = useMemo(() => {
    if (platform === "instagram") {
      return {
        reach: instagramMedia.reduce((sum, item) => sum + (item.reach || 0), 0),
        likes: instagramMedia.reduce((sum, item) => sum + (item.like_count || 0), 0),
        comments: instagramMedia.reduce((sum, item) => sum + (item.comments_count || 0), 0),
        shares: instagramMedia.reduce((sum, item) => sum + (item.shares || 0), 0),
        saves: instagramMedia.reduce((sum, item) => sum + (item.saves || 0), 0),
      };
    }
    return {
      views: threadsPosts.reduce((sum, item) => sum + (item.views || 0), 0),
      likes: threadsPosts.reduce((sum, item) => sum + (item.likes || 0), 0),
      replies: threadsPosts.reduce((sum, item) => sum + (item.replies || 0), 0),
      shares: threadsPosts.reduce((sum, item) => sum + (item.shares || 0), 0),
    };
  }, [instagramMedia, platform, threadsPosts]);

  const stats = platform === "instagram"
    ? [
        { label: "Followers", value: metrics?.follower_count || ("followers_count" in (profile || {}) ? (profile as InstagramProfile).followers_count : 0), icon: Users },
        { label: "Following", value: metrics?.following_count || ("follows_count" in (profile || {}) ? (profile as InstagramProfile).follows_count : 0), icon: Users },
        { label: "Media", value: metrics?.post_count || ("media_count" in (profile || {}) ? (profile as InstagramProfile).media_count : 0), icon: FileText },
        { label: "Recent Reach", value: nativeTotals.reach || 0, icon: BadgeCheck },
      ]
    : [
        { label: "Followers", value: metrics?.follower_count || 0, icon: Users },
        { label: "Views", value: recordNumber(metrics?.platform_specific?.views) || nativeTotals.views || 0, icon: BadgeCheck },
        { label: "Replies", value: recordNumber(metrics?.platform_specific?.replies) || nativeTotals.replies || 0, icon: MessageCircle },
        { label: "Shares", value: recordNumber(metrics?.platform_specific?.reposts) + recordNumber(metrics?.platform_specific?.quotes) || nativeTotals.shares || 0, icon: Repeat2 },
      ];

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 20, marginBottom: 24 }}>
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 8 }}>
            <div className="platform-icon-wrap"><PlatformIcon platform={platform} /></div>
            <div className="dt-page-title">{config.title}</div>
          </div>
          <div className="dt-subtitle" style={{ maxWidth: 760 }}>{config.subtitle}</div>
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
        <div style={{ color: "var(--dmuted)", padding: 40, textAlign: "center" }}>Loading {config.label} analytics...</div>
      ) : !selectedAccount ? (
        <EmptyState platform={platform} label={config.label} />
      ) : (
        <>
          <ScopeReadiness requiredScopes={config.requiredScopes} unknown={scopeState.unknown} missingScopes={scopeState.missing} />
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(min(100%, 360px), 1fr))", gap: 16, marginBottom: 24 }}>
            <ProfilePanel platform={platform} profile={profile} account={selectedAccount} />
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(130px, 1fr))", gap: 12 }}>
              {stats.map((item) => <MetricCard key={item.label} {...item} />)}
            </div>
          </div>
          {platform === "instagram" ? (
            <InstagramMediaTable rows={instagramMedia} />
          ) : (
            <ThreadsPostsTable rows={threadsPosts} />
          )}
          <UniPostPostsTable platform={platform} primaryLabel={config.primaryMetric} rows={postRows} />
        </>
      )}
    </>
  );
}

function buildPostRows(
  platform: MetaAnalyticsPlatform,
  posts: SocialPost[],
  analyticsSettled: PromiseSettledResult<ApiResponse<PostAnalytics[]>>[],
  accountId: string
): UniPostPostRow[] {
  return posts.map((post, index) => {
    const analytics = analyticsSettled[index];
    const row = analytics.status === "fulfilled"
      ? analytics.value.data.find((item) => item.social_account_id === accountId)
      : undefined;
    const result = post.results?.find((item) => item.social_account_id === accountId);
    return {
      title: post.caption || `Untitled ${platform} post`,
      status: result?.status || post.status,
      externalId: row?.external_id || result?.external_id || "-",
      primaryMetric: platform === "instagram" ? row?.reach || 0 : row?.impressions || row?.views || 0,
      likes: row?.likes || 0,
      comments: row?.comments || 0,
      shares: row?.shares || 0,
    };
  });
}

function Notice({ tone, message }: { tone: "danger" | "muted"; message: string }) {
  const danger = tone === "danger";
  return (
    <div style={{
      marginBottom: 16,
      padding: "10px 12px",
      border: danger ? "1px solid color-mix(in srgb, var(--danger) 24%, transparent)" : "1px solid var(--dborder)",
      borderRadius: 8,
      color: danger ? "var(--danger)" : "var(--dmuted)",
      background: danger ? "var(--danger-soft)" : "var(--surface2)",
      fontSize: 13,
    }}>
      {message}
    </div>
  );
}

function ScopeReadiness({
  requiredScopes,
  unknown,
  missingScopes,
}: {
  requiredScopes: readonly string[];
  unknown: boolean;
  missingScopes: readonly string[];
}) {
  const ready = missingScopes.length === 0;
  const body = unknown
    ? "Stored scope data is unavailable for this connection; live Meta API calls verify access."
    : ready ? requiredScopes.join(", ") : `Missing: ${missingScopes.join(", ")}`;
  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 16, padding: "12px 14px", borderRadius: 8, border: "1px solid var(--dborder)", background: "var(--surface1)" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 10, minWidth: 0 }}>
        <ShieldCheck style={{ width: 18, height: 18, color: ready ? "var(--success)" : "var(--warning)" }} />
        <div>
          <div style={{ color: "var(--dtext)", fontSize: 13, fontWeight: 650 }}>{ready ? "Analytics permissions ready" : "Reconnect required for analytics"}</div>
          <div style={{ color: "var(--dmuted)", fontSize: 12, marginTop: 3 }}>{body}</div>
        </div>
      </div>
      <span className={`dbadge ${ready ? "dbadge-green" : "dbadge-amber"}`}><span className="dbadge-dot" />{ready ? "Ready" : "Reconnect"}</span>
    </div>
  );
}

function ProfilePanel({
  platform,
  profile,
  account,
}: {
  platform: MetaAnalyticsPlatform;
  profile: MetaProfile | null;
  account: SocialAccount;
}) {
  const config = CONFIG[platform];
  const username = usernameFromProfile(profile, account);
  const displayName = username ? `@${username}` : account.account_name || config.label;
  const avatarUrl = platform === "instagram"
    ? (profile as InstagramProfile | null)?.profile_picture_url || account.account_avatar_url || ""
    : (profile as ThreadsProfile | null)?.threads_profile_picture_url || account.account_avatar_url || "";
  const profileUrl = username ? `${config.profileBase}${username}` : "";

  return (
    <div className="settings-section" style={{ marginBottom: 0 }}>
      <div className="settings-section-header">
        <div>
          <div className="settings-section-title">Profile</div>
          <div className="settings-section-desc">Powered by {config.requiredScopes[0]}</div>
        </div>
      </div>
      <div className="settings-section-body">
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 16 }}>
          <ProfileAvatar src={avatarUrl} label={displayName} />
          <div style={{ minWidth: 0 }}>
            <div style={{ color: "var(--dtext)", fontWeight: 700 }}>{displayName}</div>
            <div style={{ color: "var(--dmuted)", fontSize: 13 }}>{account.id}</div>
          </div>
        </div>
        {profileUrl && (
          <Link href={profileUrl} target="_blank" rel="noreferrer" style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--daccent)", fontSize: 13, textDecoration: "none", minWidth: 0 }}>
            <ExternalLink style={{ width: 14, height: 14, flexShrink: 0 }} />
            <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{profileUrl}</span>
          </Link>
        )}
      </div>
    </div>
  );
}

function ProfileAvatar({ src, label }: { src: string; label: string }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [src]);
  const showImage = src && !failed;
  return (
    <div style={{ width: 48, height: 48, borderRadius: "50%", background: "linear-gradient(135deg, #0f172a, #0f766e)", display: "grid", placeItems: "center", color: "white", fontWeight: 700, overflow: "hidden", flexShrink: 0 }}>
      {showImage ? (
        <img src={src} alt="" onError={() => setFailed(true)} style={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }} />
      ) : (
        <span>{label.replace(/^@/, "").slice(0, 1).toUpperCase() || "U"}</span>
      )}
    </div>
  );
}

function MetricCard({ label, value, icon: Icon }: { label: string; value: number; icon: LucideIcon }) {
  return (
    <div style={{ border: "1px solid var(--dborder)", borderRadius: 8, background: "var(--surface1)", padding: 14, minHeight: 92 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8, marginBottom: 12 }}>
        <div style={{ color: "var(--dmuted)", fontSize: 12 }}>{label}</div>
        <Icon style={{ width: 15, height: 15, color: "var(--dmuted2)" }} />
      </div>
      <div style={{ color: "var(--dtext)", fontSize: 24, fontWeight: 750, fontFamily: "var(--font-geist-mono), monospace" }}>{formatNumber(value)}</div>
    </div>
  );
}

function InstagramMediaTable({ rows }: { rows: InstagramMedia[] }) {
  return (
    <ContentSection title={CONFIG.instagram.contentTitle} desc={CONFIG.instagram.contentDesc}>
      <table style={tableStyle}>
        <thead>
          <tr>
            <th style={thStyle}>Media</th>
            <th style={thStyle}>Date</th>
            <th style={thRightStyle}>Reach</th>
            <th style={thRightStyle}>Likes</th>
            <th style={thRightStyle}>Comments</th>
            <th style={thRightStyle}>Shares</th>
            <th style={thRightStyle}>Saves</th>
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr><td colSpan={7} style={{ ...tdStyle, textAlign: "center", color: "var(--dmuted2)", padding: 24 }}>No Instagram media returned.</td></tr>
          ) : rows.map((row) => (
            <tr key={row.id}>
              <td style={tdStyle}><ContentLink href={row.permalink} text={row.caption || row.media_type || "Instagram media"} /></td>
              <td style={tdStyle}>{formatDate(row.timestamp)}</td>
              <td style={tdRightStyle}>{formatNumber(row.reach)}</td>
              <td style={tdRightStyle}>{formatNumber(row.like_count)}</td>
              <td style={tdRightStyle}>{formatNumber(row.comments_count)}</td>
              <td style={tdRightStyle}>{formatNumber(row.shares)}</td>
              <td style={tdRightStyle}>{formatNumber(row.saves)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </ContentSection>
  );
}

function ThreadsPostsTable({ rows }: { rows: ThreadsPost[] }) {
  return (
    <ContentSection title={CONFIG.threads.contentTitle} desc={CONFIG.threads.contentDesc}>
      <table style={tableStyle}>
        <thead>
          <tr>
            <th style={thStyle}>Post</th>
            <th style={thStyle}>Date</th>
            <th style={thRightStyle}>Views</th>
            <th style={thRightStyle}>Likes</th>
            <th style={thRightStyle}>Replies</th>
            <th style={thRightStyle}>Reposts</th>
            <th style={thRightStyle}>Quotes</th>
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr><td colSpan={7} style={{ ...tdStyle, textAlign: "center", color: "var(--dmuted2)", padding: 24 }}>No Threads posts returned.</td></tr>
          ) : rows.map((row) => (
            <tr key={row.id}>
              <td style={tdStyle}><ContentLink href={row.permalink} text={row.text || row.media_type || "Threads post"} /></td>
              <td style={tdStyle}>{formatDate(row.timestamp)}</td>
              <td style={tdRightStyle}>{formatNumber(row.views)}</td>
              <td style={tdRightStyle}>{formatNumber(row.likes)}</td>
              <td style={tdRightStyle}>{formatNumber(row.replies)}</td>
              <td style={tdRightStyle}>{formatNumber(row.reposts)}</td>
              <td style={tdRightStyle}>{formatNumber(row.quotes)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </ContentSection>
  );
}

function UniPostPostsTable({ platform, primaryLabel, rows }: { platform: MetaAnalyticsPlatform; primaryLabel: string; rows: UniPostPostRow[] }) {
  return (
    <ContentSection title={`UniPost-published ${CONFIG[platform].label} posts`} desc="Rows from the cross-platform post analytics cache for this connected account.">
      <table style={tableStyle}>
        <thead>
          <tr>
            <th style={thStyle}>Post</th>
            <th style={thStyle}>Status</th>
            <th style={thStyle}>External ID</th>
            <th style={thRightStyle}>{primaryLabel}</th>
            <th style={thRightStyle}>Likes</th>
            <th style={thRightStyle}>Comments</th>
            <th style={thRightStyle}>Shares</th>
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr><td colSpan={7} style={{ ...tdStyle, textAlign: "center", color: "var(--dmuted2)", padding: 24 }}>No UniPost-published {CONFIG[platform].label} posts returned.</td></tr>
          ) : rows.map((row, index) => (
            <tr key={`${row.externalId}-${index}`}>
              <td style={tdStyle}>{row.title}</td>
              <td style={tdStyle}>{row.status}</td>
              <td style={tdStyle}><span style={{ color: "var(--dmuted)", fontFamily: "var(--font-geist-mono), monospace", fontSize: 12 }}>{row.externalId}</span></td>
              <td style={tdRightStyle}>{formatNumber(row.primaryMetric)}</td>
              <td style={tdRightStyle}>{formatNumber(row.likes)}</td>
              <td style={tdRightStyle}>{formatNumber(row.comments)}</td>
              <td style={tdRightStyle}>{formatNumber(row.shares)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </ContentSection>
  );
}

function ContentSection({ title, desc, children }: { title: string; desc: string; children: ReactNode }) {
  return (
    <div className="settings-section" style={{ marginBottom: 24 }}>
      <div className="settings-section-header">
        <div>
          <div className="settings-section-title">{title}</div>
          <div className="settings-section-desc">{desc}</div>
        </div>
      </div>
      <div className="settings-section-body" style={{ overflowX: "auto" }}>
        {children}
      </div>
    </div>
  );
}

function ContentLink({ href, text }: { href?: string; text: string }) {
  const label = text.length > 80 ? `${text.slice(0, 77)}...` : text;
  if (!href) return <span>{label}</span>;
  return (
    <Link href={href} target="_blank" rel="noreferrer" style={{ color: "var(--dtext)", textDecoration: "none", display: "inline-flex", alignItems: "center", gap: 6, maxWidth: 360 }}>
      <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{label}</span>
      <ExternalLink style={{ width: 13, height: 13, color: "var(--dmuted2)", flexShrink: 0 }} />
    </Link>
  );
}

function EmptyState({ platform, label }: { platform: MetaAnalyticsPlatform; label: string }) {
  return (
    <div style={{ textAlign: "center", padding: 60, color: "var(--dmuted)" }}>
      <Heart style={{ width: 40, height: 40, margin: "0 auto 12px", opacity: 0.3 }} />
      <div style={{ fontSize: 15, fontWeight: 600, color: "var(--dtext)", marginBottom: 6 }}>No {label} account connected</div>
      <div style={{ fontSize: 13 }}>Connect a {CONFIG[platform].label} account before using platform analytics.</div>
    </div>
  );
}
