"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { CSSProperties } from "react";
import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import {
  BadgeCheck,
  ExternalLink,
  Heart,
  Link as LinkIcon,
  ListVideo,
  Play,
  RefreshCw,
  ShieldCheck,
  UserRoundCheck,
  Users,
  Video,
  type LucideIcon,
} from "lucide-react";
import {
  getAccountMetrics,
  getPostAnalytics,
  getTikTokProfile,
  getTikTokVideos,
  listSocialAccounts,
  listSocialPosts,
  type AccountMetrics,
  type SocialAccount,
  type TikTokProfile,
  type TikTokVideo,
} from "@/lib/api";
import { PlatformIcon } from "@/components/platform-icons";
import { buildTikTokPostRows, type TikTokPostRow } from "./tiktok-analytics-rows";

const REQUIRED_SCOPES = ["user.info.profile", "user.info.stats", "video.list"] as const;

const sectionLabelStyle: CSSProperties = {
  fontSize: 10,
  fontWeight: 600,
  letterSpacing: "0.08em",
  textTransform: "uppercase",
  color: "var(--dmuted2)",
  marginBottom: 8,
};

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

const previewProfile: TikTokProfile = {
  social_account_id: "sa_tiktok_preview",
  platform: "tiktok",
  open_id: "open_preview",
  display_name: "UniPost Demo",
  avatar_url: "",
  username: "unipost_demo",
  profile_web_link: "https://www.tiktok.com/@unipost_demo",
  profile_deep_link: "snssdk1233://user/profile/unipost_demo",
  bio_description: "Launch updates, product demos, and short-form publishing workflows.",
  is_verified: false,
  fetched_at: new Date().toISOString(),
};

const previewMetrics: AccountMetrics = {
  social_account_id: "sa_tiktok_preview",
  platform: "tiktok",
  follower_count: 12400,
  following_count: 328,
  post_count: 146,
  platform_specific: { likes_count: 86700, video_count: 146 },
  fetched_at: new Date().toISOString(),
};

const previewVideos: TikTokVideo[] = [
  { id: "7350123456789012345", title: "Launch workflow in 30 seconds", create_time: 1778544000, view_count: 8200, like_count: 612, comment_count: 38, share_count: 91 },
  { id: "7350123456789012311", title: "How UniPost schedules a TikTok", create_time: 1778284800, view_count: 5700, like_count: 433, comment_count: 27, share_count: 64 },
  { id: "7350123456789012290", title: "API-first creator workflow", create_time: 1777766400, view_count: 3900, like_count: 284, comment_count: 19, share_count: 41 },
];

const previewPostRows: TikTokPostRow[] = [
  { title: "Product launch recap", status: "Published", videoId: "7350123456789012345", views: 8200, likes: 612, comments: 38, shares: 91 },
  { title: "Creator API tutorial", status: "Published", videoId: "7350123456789012311", views: 5700, likes: 433, comments: 27, shares: 64 },
];

type TikTokAnalyticsViewProps = {
  profileId?: string;
  preview?: boolean;
};

function formatNumber(n: number | undefined): string {
  if (!Number.isFinite(n || 0) || !n) return "0";
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1).replace(/\.0$/, "") + "M";
  if (n >= 1_000) return (n / 1_000).toFixed(1).replace(/\.0$/, "") + "k";
  return String(n);
}

function formatDate(seconds?: number): string {
  if (!seconds) return "-";
  return new Date(seconds * 1000).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
}

function metricSpecificNumber(metrics: AccountMetrics | null, key: string): number {
  const raw = metrics?.platform_specific?.[key];
  return typeof raw === "number" ? raw : 0;
}

export function TikTokAnalyticsView({ profileId, preview = false }: TikTokAnalyticsViewProps) {
  const { getToken } = useAuth();
  const [accounts, setAccounts] = useState<SocialAccount[]>(preview ? [{
    id: "sa_tiktok_preview",
    profile_id: profileId || "preview",
    platform: "tiktok",
    account_name: "UniPost Demo",
    connected_at: new Date().toISOString(),
    status: "active",
    connection_type: "byo",
    scope: [...REQUIRED_SCOPES, "user.info.basic", "video.publish", "video.upload"],
  }] : []);
  const [selectedAccountId, setSelectedAccountId] = useState("sa_tiktok_preview");
  const [profile, setProfile] = useState<TikTokProfile | null>(preview ? previewProfile : null);
  const [metrics, setMetrics] = useState<AccountMetrics | null>(preview ? previewMetrics : null);
  const [videos, setVideos] = useState<TikTokVideo[]>(preview ? previewVideos : []);
  const [postRows, setPostRows] = useState<TikTokPostRow[]>(preview ? previewPostRows : []);
  const [loading, setLoading] = useState(!preview);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");

  const selectedAccount = useMemo(
    () => accounts.find((account) => account.id === selectedAccountId) || accounts[0],
    [accounts, selectedAccountId]
  );

  const missingScopes = useMemo(() => {
    const granted = new Set(selectedAccount?.scope || []);
    return REQUIRED_SCOPES.filter((scope) => !granted.has(scope));
  }, [selectedAccount?.scope]);

  const loadData = useCallback(async () => {
    if (preview || !profileId) return;
    try {
      setError("");
      setLoading((wasLoading) => wasLoading || accounts.length === 0);
      setRefreshing(true);
      const token = await getToken();
      if (!token) return;

      const accountRes = await listSocialAccounts(token, profileId, { platform: "tiktok" });
      const tiktokAccounts = accountRes.data || [];
      setAccounts(tiktokAccounts);
      const account = tiktokAccounts.find((a) => a.id === selectedAccountId) || tiktokAccounts[0];
      if (!account) {
        setProfile(null);
        setMetrics(null);
        setVideos([]);
        setPostRows([]);
        return;
      }
      if (account.id !== selectedAccountId) setSelectedAccountId(account.id);

      const [profileRes, metricsRes, videosRes, postsRes] = await Promise.all([
        getTikTokProfile(token, profileId, account.id),
        getAccountMetrics(token, profileId, account.id),
        getTikTokVideos(token, profileId, account.id, { limit: 20 }),
        listSocialPosts(token),
      ]);

      setProfile(profileRes.data);
      setMetrics(metricsRes.data);
      setVideos(videosRes.data.videos || []);

      const published = (postsRes.data || []).filter((post) =>
        post.status === "published" &&
        post.results?.some((result) => result.social_account_id === account.id)
      );
      const analyticsSettled = await Promise.allSettled(
        published.map((post) => getPostAnalytics(token, post.id))
      );
      setPostRows(buildTikTokPostRows(published, analyticsSettled, account.id));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load TikTok analytics");
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [accounts.length, getToken, preview, profileId, selectedAccountId]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const stats = [
    { label: "Followers", value: metrics?.follower_count || 0, icon: Users, scope: "user.info.stats" },
    { label: "Following", value: metrics?.following_count || 0, icon: UserRoundCheck, scope: "user.info.stats" },
    { label: "Total Likes", value: metricSpecificNumber(metrics, "likes_count"), icon: Heart, scope: "user.info.stats" },
    { label: "Public Videos", value: metrics?.post_count || 0, icon: ListVideo, scope: "user.info.stats" },
  ];

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 20, marginBottom: 24 }}>
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 8 }}>
            <div className="platform-icon-wrap"><PlatformIcon platform="tiktok" /></div>
            <div className="dt-page-title">TikTok Analytics</div>
          </div>
          <div className="dt-subtitle" style={{ maxWidth: 720 }}>
            Platform-specific profile, account statistics, public videos, and UniPost-published post performance.
          </div>
        </div>
        <button className="dbtn dbtn-ghost" type="button" onClick={() => loadData()} disabled={preview || refreshing}>
          <RefreshCw style={{ width: 14, height: 14 }} />
          {refreshing ? "Refreshing..." : "Refresh"}
        </button>
      </div>

      {preview && (
        <div style={{ marginBottom: 16, padding: "10px 12px", border: "1px solid var(--dborder)", borderRadius: 8, color: "var(--dmuted)", background: "var(--surface2)", fontSize: 13 }}>
          Local preview with sample data. The dashboard route uses live TikTok endpoints after the account reconnects with analytics scopes.
        </div>
      )}

      {error && (
        <div style={{ marginBottom: 16, padding: "10px 12px", border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)", borderRadius: 8, color: "var(--danger)", background: "var(--danger-soft)", fontSize: 13 }}>
          {error}
        </div>
      )}

      {!preview && accounts.length > 1 && (
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
        <div style={{ color: "var(--dmuted)", padding: 40, textAlign: "center" }}>Loading TikTok analytics...</div>
      ) : !selectedAccount ? (
        <div style={{ textAlign: "center", padding: 60, color: "var(--dmuted)" }}>
          <Video style={{ width: 40, height: 40, margin: "0 auto 12px", opacity: 0.3 }} />
          <div style={{ fontSize: 15, fontWeight: 600, color: "var(--dtext)", marginBottom: 6 }}>No TikTok account connected</div>
          <div style={{ fontSize: 13 }}>Connect a TikTok account before using platform analytics.</div>
        </div>
      ) : (
        <>
          <ScopeReadiness missingScopes={missingScopes} />
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(min(100%, 360px), 1fr))", gap: 16, marginBottom: 24 }}>
            <ProfilePanel profile={profile} account={selectedAccount} />
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(130px, 1fr))", gap: 12 }}>
              {stats.map((item) => <MetricCard key={item.label} {...item} />)}
            </div>
          </div>
          <VideosTable videos={videos} />
          <TikTokPostsTable rows={postRows} />
        </>
      )}
    </>
  );
}

function ScopeReadiness({ missingScopes }: { missingScopes: readonly string[] }) {
  const ready = missingScopes.length === 0;
  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 16, padding: "12px 14px", borderRadius: 10, border: "1px solid var(--dborder)", background: "var(--surface1)" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 10, minWidth: 0 }}>
        <ShieldCheck style={{ width: 18, height: 18, color: ready ? "var(--success)" : "var(--warning)" }} />
        <div>
          <div style={{ color: "var(--dtext)", fontSize: 13, fontWeight: 650 }}>{ready ? "Analytics scopes ready" : "Reconnect required for analytics"}</div>
          <div style={{ color: "var(--dmuted)", fontSize: 12, marginTop: 3 }}>
            {ready ? REQUIRED_SCOPES.join(", ") : `Missing: ${missingScopes.join(", ")}`}
          </div>
        </div>
      </div>
      <span className={`dbadge ${ready ? "dbadge-green" : "dbadge-amber"}`}><span className="dbadge-dot" />{ready ? "Ready" : "Reconnect"}</span>
    </div>
  );
}

function ProfilePanel({ profile, account }: { profile: TikTokProfile | null; account: SocialAccount }) {
  const displayName = profile?.display_name || account.account_name || "TikTok account";
  const username = profile?.username || account.account_name || "";
  const avatarUrl = profile?.avatar_url || account.account_avatar_url || "";
  const normalizedUsername = username.replace(/^@/, "").trim();
  const canonicalProfileUrl = normalizedUsername ? `https://www.tiktok.com/@${normalizedUsername}` : "";
  const profileWebLink = canonicalProfileUrl || profile?.profile_web_link || "";
  const profileWebLabel = canonicalProfileUrl || profile?.profile_web_link || "";
  const profileDeepLink = profile?.profile_deep_link || "";
  return (
    <div className="settings-section" style={{ marginBottom: 0 }}>
      <div className="settings-section-header">
        <div>
          <div className="settings-section-title">Profile</div>
          <div className="settings-section-desc">Powered by user.info.profile</div>
        </div>
      </div>
      <div className="settings-section-body">
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 16 }}>
          <ProfileAvatar src={avatarUrl} label={displayName} />
          <div style={{ minWidth: 0 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 6, color: "var(--dtext)", fontWeight: 700 }}>
              {displayName}
              {profile?.is_verified ? <BadgeCheck style={{ width: 15, height: 15, color: "var(--info)" }} /> : null}
            </div>
            <div style={{ color: "var(--dmuted)", fontSize: 13 }}>{normalizedUsername ? `@${normalizedUsername}` : account.id}</div>
          </div>
        </div>
        <div style={{ color: "var(--dtext)", fontSize: 13, lineHeight: 1.55, marginBottom: 14 }}>
          {profile?.bio_description || "No TikTok bio returned yet."}
        </div>
        <div style={{ display: "grid", gap: 9 }}>
          {profileWebLink && (
            <Link href={profileWebLink} target="_blank" rel="noreferrer" style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--daccent)", fontSize: 13, textDecoration: "none" }}>
              <ExternalLink style={{ width: 14, height: 14 }} />
              <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{profileWebLabel}</span>
            </Link>
          )}
          {profileDeepLink && (
            <Link href={profileDeepLink} target="_blank" rel="noreferrer" style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)", fontSize: 13, minWidth: 0, textDecoration: "none" }}>
              <LinkIcon style={{ width: 14, height: 14, flexShrink: 0 }} />
              <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{profileDeepLink}</span>
            </Link>
          )}
          <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)", fontSize: 13 }}>
            <BadgeCheck style={{ width: 14, height: 14 }} />
            {profile?.is_verified ? "Verified account" : "Not verified"}
          </div>
        </div>
      </div>
    </div>
  );
}

function ProfileAvatar({ src, label }: { src: string; label: string }) {
  const [failedSrc, setFailedSrc] = useState("");
  const showImage = src && failedSrc !== src;
  return (
    <div style={{ width: 48, height: 48, borderRadius: "50%", background: "linear-gradient(135deg, #111827, #0f766e)", display: "grid", placeItems: "center", color: "white", fontWeight: 700, overflow: "hidden", flexShrink: 0 }}>
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

function MetricCard({ label, value, scope, icon: Icon }: { label: string; value: number; scope: string; icon: LucideIcon }) {
  return (
    <div style={{ border: "1px solid var(--dborder)", background: "var(--surface1)", borderRadius: 10, padding: "14px 16px", minHeight: 104, display: "flex", flexDirection: "column", justifyContent: "space-between" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10 }}>
        <span style={{ color: "var(--dmuted2)", fontSize: 10, fontWeight: 600, letterSpacing: "0.06em", textTransform: "uppercase" }}>{label}</span>
        <Icon style={{ width: 16, height: 16, color: "var(--dmuted2)" }} />
      </div>
      <div style={{ fontSize: 24, fontWeight: 700, color: "var(--dtext)", letterSpacing: 0, fontFamily: "var(--font-geist-mono), monospace" }}>{formatNumber(value)}</div>
      <div style={{ color: "var(--dmuted2)", fontSize: 11, fontFamily: "var(--font-geist-mono), monospace" }}>{scope}</div>
    </div>
  );
}

function VideosTable({ videos }: { videos: TikTokVideo[] }) {
  return (
    <div style={{ marginBottom: 28 }}>
      <div style={sectionLabelStyle}>Public Videos</div>
      <div className="settings-section">
        <div className="settings-section-body">
        <table style={tableStyle}>
          <thead>
            <tr>
              <th style={thStyle}>Video</th>
              <th style={thStyle}>Created</th>
              <th style={thRightStyle}>Views</th>
              <th style={thRightStyle}>Likes</th>
              <th style={thRightStyle}>Comments</th>
              <th style={thRightStyle}>Shares</th>
            </tr>
          </thead>
          <tbody>
            {videos.map((video) => (
              <tr key={video.id}>
                <td style={tdStyle}>
                  <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                    <VideoThumb video={video} />
                    <div>
                      <div style={{ color: "var(--dtext)", fontWeight: 600 }}>{video.title || video.video_description || "Untitled TikTok video"}</div>
                      <div style={{ color: "var(--dmuted2)", fontSize: 12, fontFamily: "var(--font-geist-mono), monospace" }}>{video.id}</div>
                    </div>
                  </div>
                </td>
                <td style={{ ...tdStyle, color: "var(--dmuted)", fontWeight: 500 }}>{formatDate(video.create_time)}</td>
                <td style={tdRightStyle}>{formatNumber(video.view_count)}</td>
                <td style={tdRightStyle}>{formatNumber(video.like_count)}</td>
                <td style={tdRightStyle}>{formatNumber(video.comment_count)}</td>
                <td style={tdRightStyle}>{formatNumber(video.share_count)}</td>
              </tr>
            ))}
            {videos.length === 0 && (
              <tr><td colSpan={6} style={{ ...tdStyle, textAlign: "center", color: "var(--dmuted2)", padding: 24 }}>No public TikTok videos returned.</td></tr>
            )}
          </tbody>
        </table>
        </div>
      </div>
    </div>
  );
}

function VideoThumb({ video }: { video: TikTokVideo }) {
  const [failedSrc, setFailedSrc] = useState("");
  const showImage = video.cover_image_url && failedSrc !== video.cover_image_url;
  return (
    <div style={{ width: 38, height: 38, borderRadius: 6, background: "var(--surface2)", display: "grid", placeItems: "center", color: "var(--dmuted)", overflow: "hidden", flexShrink: 0 }}>
      {showImage ? (
        <img
          src={video.cover_image_url}
          alt=""
          onError={() => setFailedSrc(video.cover_image_url || "")}
          style={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }}
        />
      ) : (
        <Play style={{ width: 15, height: 15 }} />
      )}
    </div>
  );
}

function TikTokPostsTable({ rows }: { rows: TikTokPostRow[] }) {
  return (
    <div>
      <div style={sectionLabelStyle}>UniPost Published Posts</div>
      <div className="settings-section">
        <div className="settings-section-body">
        <table style={tableStyle}>
          <thead>
            <tr>
              <th style={thStyle}>Post</th>
              <th style={thStyle}>Status</th>
              <th style={thStyle}>Video ID</th>
              <th style={thRightStyle}>Views</th>
              <th style={thRightStyle}>Likes</th>
              <th style={thRightStyle}>Comments</th>
              <th style={thRightStyle}>Shares</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={`${row.title}-${row.videoId}`}>
                <td style={{ ...tdStyle, fontWeight: 600 }}>{row.title}</td>
                <td style={tdStyle}><span className="dbadge dbadge-green"><span className="dbadge-dot" />{row.status}</span></td>
                <td style={{ ...tdStyle, color: "var(--dmuted2)", fontFamily: "var(--font-geist-mono), monospace", fontSize: 12 }}>{row.videoId}</td>
                <td style={tdRightStyle}>{formatNumber(row.views)}</td>
                <td style={tdRightStyle}>{formatNumber(row.likes)}</td>
                <td style={tdRightStyle}>{formatNumber(row.comments)}</td>
                <td style={tdRightStyle}>{formatNumber(row.shares)}</td>
              </tr>
            ))}
            {rows.length === 0 && (
              <tr><td colSpan={7} style={{ ...tdStyle, textAlign: "center", color: "var(--dmuted2)", padding: 24 }}>No UniPost-published TikTok analytics yet.</td></tr>
            )}
          </tbody>
        </table>
        </div>
      </div>
    </div>
  );
}
