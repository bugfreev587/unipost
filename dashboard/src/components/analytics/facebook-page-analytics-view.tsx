"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { CSSProperties } from "react";
import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import {
  AlertTriangle,
  BarChart3,
  ExternalLink,
  FileText,
  Image as ImageIcon,
  MessageCircle,
  MousePointerClick,
  RefreshCw,
  Share2,
  ShieldCheck,
  ThumbsUp,
  UserRound,
  Users,
  Video,
  type LucideIcon,
} from "lucide-react";
import {
  getFacebookPageAnalytics,
  listSocialAccounts,
  type FacebookPageAnalytics,
  type FacebookPageAnalyticsPost,
  type SocialAccount,
} from "@/lib/api";
import { PlatformIcon } from "@/components/platform-icons";

const CSS = `
@keyframes fbpa-spin{from{transform:rotate(0deg)}to{transform:rotate(360deg)}}
.fbpa-layout{display:grid;grid-template-columns:minmax(0,1.42fr) minmax(320px,.8fr);gap:16px;align-items:start}
.fbpa-post-row{cursor:pointer;transition:background .12s ease}
.fbpa-post-row:hover td{background:color-mix(in srgb,var(--daccent) 7%,var(--surface))}
.fbpa-post-row.active td{background:color-mix(in srgb,var(--daccent) 10%,var(--surface));box-shadow:inset 3px 0 0 var(--daccent)}
.fbpa-thumb{width:42px;height:42px;border-radius:7px;background:var(--surface2);display:grid;place-items:center;overflow:hidden;flex-shrink:0;color:var(--dmuted2);border:1px solid var(--dborder)}
@media (max-width: 980px){.fbpa-layout{grid-template-columns:1fr}.fbpa-detail{position:static!important}}
`;

const sectionLabelStyle: CSSProperties = {
  fontSize: 10,
  fontWeight: 700,
  letterSpacing: 0,
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
  fontWeight: 700,
  letterSpacing: 0,
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

type FacebookPageAnalyticsViewProps = {
  profileId: string;
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
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("en-US", { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
}

function postTitle(post: FacebookPageAnalyticsPost): string {
  const text = post.message.trim();
  if (text) return text;
  if (post.media_type === "video") return "Facebook video post";
  if (post.media_type === "image") return "Facebook photo post";
  if (post.media_type === "link") return "Facebook link post";
  return "Facebook Page post";
}

export function FacebookPageAnalyticsView({ profileId }: FacebookPageAnalyticsViewProps) {
  const { getToken } = useAuth();
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [selectedAccountId, setSelectedAccountId] = useState("");
  const [data, setData] = useState<FacebookPageAnalytics | null>(null);
  const [selectedPostId, setSelectedPostId] = useState("");
  const [days, setDays] = useState(28);
  const [limit, setLimit] = useState(12);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");

  const selectedAccount = useMemo(
    () => accounts.find((account) => account.id === selectedAccountId) || accounts[0],
    [accounts, selectedAccountId]
  );

  const selectedPost = useMemo(
    () => data?.posts.find((post) => post.id === selectedPostId) || data?.posts[0],
    [data?.posts, selectedPostId]
  );

  const loadData = useCallback(async () => {
    try {
      setError("");
      setLoading((wasLoading) => wasLoading || accounts.length === 0);
      setRefreshing(true);
      const token = await getToken();
      if (!token) return;

      const accountRes = await listSocialAccounts(token, profileId, { platform: "facebook" });
      const facebookAccounts = accountRes.data || [];
      setAccounts(facebookAccounts);
      const account = facebookAccounts.find((item) => item.id === selectedAccountId) || facebookAccounts[0];
      if (!account) {
        setData(null);
        setSelectedPostId("");
        return;
      }
      if (account.id !== selectedAccountId) setSelectedAccountId(account.id);

      const analyticsRes = await getFacebookPageAnalytics(token, profileId, account.id, { days, limit });
      const next = analyticsRes.data;
      setData(next);
      setSelectedPostId((current) => next.posts.some((post) => post.id === current) ? current : next.posts[0]?.id || "");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load Facebook Page analytics");
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [accounts.length, days, getToken, limit, profileId, selectedAccountId]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const topStats = [
    { label: "Followers", value: data?.page?.followers_count || 0, icon: Users },
    { label: "Page Likes", value: data?.page?.fan_count || 0, icon: ThumbsUp },
    { label: "Posts Loaded", value: data?.posts.length || 0, icon: FileText },
    { label: "Post Engagement", value: data?.insights?.post_engagements || sumEngagement(data?.posts || []), icon: BarChart3 },
  ];

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 20, marginBottom: 24 }}>
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 8 }}>
            <div className="platform-icon-wrap"><PlatformIcon platform="facebook" /></div>
            <div className="dt-page-title">Facebook Page Analytics</div>
          </div>
          <div className="dt-subtitle" style={{ maxWidth: 760 }}>
            Page profile, published Page content, and engagement data for connected Facebook Pages.
          </div>
        </div>
        <button className="dbtn dbtn-ghost" type="button" onClick={() => loadData()} disabled={refreshing || loading}>
          <RefreshCw style={{ width: 14, height: 14, animation: refreshing ? "fbpa-spin 1s linear infinite" : "none" }} />
          {refreshing ? "Refreshing..." : "Refresh"}
        </button>
      </div>

      {error && (
        <div style={{ marginBottom: 16, padding: "10px 12px", border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)", borderRadius: 8, color: "var(--danger)", background: "var(--danger-soft)", fontSize: 13 }}>
          {error}
        </div>
      )}

      <Controls
        accounts={accounts}
        selectedAccountId={selectedAccount?.id || ""}
        setSelectedAccountId={setSelectedAccountId}
        days={days}
        setDays={setDays}
        limit={limit}
        setLimit={setLimit}
      />

      {loading ? (
        <div style={{ color: "var(--dmuted)", padding: 40, textAlign: "center" }}>Loading Facebook Page analytics...</div>
      ) : !selectedAccount ? (
        <EmptyFacebookState profileId={profileId} />
      ) : (
        <>
          <ScopeReadiness
            grantedScopes={data?.granted_scopes || selectedAccount.scope || []}
            requiredScopes={data?.required_scopes || ["pages_read_engagement"]}
            recommendedScopes={data?.recommended_scopes || ["read_insights"]}
            insightsError={data?.insights_error}
            readAccessVerified={Boolean(data?.page || data?.posts.length)}
          />

          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(150px, 1fr))", gap: 12, marginBottom: 20 }}>
            {topStats.map((item) => <MetricTile key={item.label} {...item} />)}
          </div>

          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(min(100%, 360px), 1fr))", gap: 16, marginBottom: 24 }}>
            <PageProfilePanel data={data} account={selectedAccount} />
            <PageInsightsPanel data={data} days={days} />
          </div>

          <div className="fbpa-layout">
            <PagePostsTable
              posts={data?.posts || []}
              selectedPostId={selectedPost?.id || ""}
              onSelect={setSelectedPostId}
            />
            <PostDetailPanel post={selectedPost} />
          </div>
        </>
      )}
    </>
  );
}

function Controls({
  accounts,
  selectedAccountId,
  setSelectedAccountId,
  days,
  setDays,
  limit,
  setLimit,
}: {
  accounts: SocialAccount[];
  selectedAccountId: string;
  setSelectedAccountId: (id: string) => void;
  days: number;
  setDays: (days: number) => void;
  limit: number;
  setLimit: (limit: number) => void;
}) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap", marginBottom: 16, padding: "10px 12px", borderRadius: 8, border: "1px solid var(--dborder)", background: "var(--surface1)" }}>
      {accounts.length > 1 && (
        <FilterSelect
          label="Page"
          value={selectedAccountId}
          onChange={setSelectedAccountId}
          options={accounts.map((account) => ({ value: account.id, label: account.account_name || account.id }))}
        />
      )}
      <FilterSelect
        label="Range"
        value={String(days)}
        onChange={(value) => setDays(Number(value))}
        options={[
          { value: "7", label: "Last 7 days" },
          { value: "28", label: "Last 28 days" },
          { value: "90", label: "Last 90 days" },
        ]}
      />
      <FilterSelect
        label="Posts"
        value={String(limit)}
        onChange={(value) => setLimit(Number(value))}
        options={[
          { value: "8", label: "8 latest" },
          { value: "12", label: "12 latest" },
          { value: "25", label: "25 latest" },
        ]}
      />
    </div>
  );
}

function FilterSelect({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <label style={{ display: "flex", alignItems: "center", gap: 6 }}>
      <span style={{ fontSize: 10, fontWeight: 700, color: "var(--dmuted2)", textTransform: "uppercase" }}>{label}</span>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        style={{
          fontSize: 12,
          padding: "6px 8px",
          background: "var(--surface2)",
          color: "var(--dtext)",
          border: "1px solid var(--dborder2)",
          borderRadius: 6,
          cursor: "pointer",
          outline: "none",
          fontFamily: "inherit",
        }}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>{option.label}</option>
        ))}
      </select>
    </label>
  );
}

function ScopeReadiness({
  grantedScopes,
  requiredScopes,
  recommendedScopes,
  insightsError,
  readAccessVerified,
}: {
  grantedScopes: string[];
  requiredScopes: string[];
  recommendedScopes: string[];
  insightsError?: string;
  readAccessVerified?: boolean;
}) {
  const granted = new Set(grantedScopes);
  const missingRequired = requiredScopes.filter((scope) => !granted.has(scope));
  const missingRecommended = recommendedScopes.filter((scope) => !granted.has(scope));
  const ready = readAccessVerified || missingRequired.length === 0;
  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 16, padding: "12px 14px", borderRadius: 8, border: "1px solid var(--dborder)", background: "var(--surface1)" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 10, minWidth: 0 }}>
        <ShieldCheck style={{ width: 18, height: 18, color: ready ? "var(--success)" : "var(--warning)" }} />
        <div>
          <div style={{ color: "var(--dtext)", fontSize: 13, fontWeight: 650 }}>{ready ? "Facebook Page reads ready" : "Reconnect required for Page reads"}</div>
          <div style={{ color: "var(--dmuted)", fontSize: 12, marginTop: 3 }}>
            {readAccessVerified
              ? "Verified by live Page profile and published post reads"
              : missingRequired.length > 0
              ? `Missing: ${missingRequired.join(", ")}`
              : missingRecommended.length > 0
                ? `Recommended for Page Insights: ${missingRecommended.join(", ")}`
                : requiredScopes.concat(recommendedScopes).join(", ")}
          </div>
          {insightsError && (
            <div style={{ color: "var(--warning)", fontSize: 12, marginTop: 4 }}>
              Page Insights unavailable: {insightsError}
            </div>
          )}
        </div>
      </div>
      <span className={`dbadge ${ready ? "dbadge-green" : "dbadge-amber"}`}><span className="dbadge-dot" />{ready ? "Ready" : "Reconnect"}</span>
    </div>
  );
}

function MetricTile({ label, value, icon: Icon }: { label: string; value: number; icon: LucideIcon }) {
  return (
    <div style={{ border: "1px solid var(--dborder)", background: "var(--surface1)", borderRadius: 8, padding: "14px 16px", minHeight: 96, display: "flex", flexDirection: "column", justifyContent: "space-between" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10 }}>
        <span style={{ color: "var(--dmuted2)", fontSize: 10, fontWeight: 700, textTransform: "uppercase" }}>{label}</span>
        <Icon style={{ width: 16, height: 16, color: "var(--dmuted2)" }} />
      </div>
      <div style={{ fontSize: 24, fontWeight: 700, color: "var(--dtext)", fontFamily: "var(--font-geist-mono), monospace" }}>{formatNumber(value)}</div>
    </div>
  );
}

function PageProfilePanel({ data, account }: { data: FacebookPageAnalytics | null; account: SocialAccount }) {
  const page = data?.page;
  const displayName = page?.name || account.account_name || "Facebook Page";
  const handle = page?.username ? `@${page.username}` : page?.id || account.external_account_id || "-";
  return (
    <div className="settings-section" style={{ marginBottom: 0 }}>
      <div className="settings-section-header">
        <div>
          <div className="settings-section-title">Page Profile</div>
          <div className="settings-section-desc">Name, avatar, category, and public Page link</div>
        </div>
      </div>
      <div className="settings-section-body">
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 16 }}>
          <PageAvatar src={page?.picture_url || ""} label={displayName} />
          <div style={{ minWidth: 0 }}>
            <div style={{ color: "var(--dtext)", fontWeight: 700, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{displayName}</div>
            <div style={{ color: "var(--dmuted)", fontSize: 13 }}>{handle}</div>
          </div>
        </div>
        <div style={{ display: "grid", gap: 8, fontSize: 13 }}>
          <InfoLine label="Category" value={page?.category || "-"} />
          <InfoLine label="Verification" value={page?.verification_status || "-"} />
          <InfoLine label="Page ID" value={page?.id || account.external_account_id || "-"} mono />
          {page?.about ? <InfoLine label="About" value={page.about} /> : null}
          {page?.link ? (
            <Link href={page.link} target="_blank" rel="noreferrer" style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "var(--daccent)", textDecoration: "none", fontSize: 13 }}>
              <ExternalLink style={{ width: 13, height: 13 }} />
              Open Facebook Page
            </Link>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function PageInsightsPanel({ data, days }: { data: FacebookPageAnalytics | null; days: number }) {
  const insights = data?.insights;
  return (
    <div className="settings-section" style={{ marginBottom: 0 }}>
      <div className="settings-section-header">
        <div>
          <div className="settings-section-title">Page Insights</div>
          <div className="settings-section-desc">Last {days} days, when Meta returns Page-level insights</div>
        </div>
      </div>
      <div className="settings-section-body">
        {insights?.below_100_likes_notice ? (
          <div style={{ display: "flex", alignItems: "flex-start", gap: 10, padding: "10px 12px", borderRadius: 8, border: "1px solid var(--dborder)", background: "var(--surface2)", color: "var(--dmuted)", fontSize: 13, lineHeight: 1.55 }}>
            <AlertTriangle style={{ width: 16, height: 16, color: "var(--warning)", flexShrink: 0, marginTop: 1 }} />
            Meta only returns some Page Insights after the Page crosses its likes threshold.
          </div>
        ) : data?.insights_error ? (
          <div style={{ display: "flex", alignItems: "flex-start", gap: 10, padding: "10px 12px", borderRadius: 8, border: "1px solid color-mix(in srgb, var(--warning) 28%, var(--dborder))", background: "var(--surface2)", color: "var(--dmuted)", fontSize: 13, lineHeight: 1.55 }}>
            <AlertTriangle style={{ width: 16, height: 16, color: "var(--warning)", flexShrink: 0, marginTop: 1 }} />
            {data.insights_error}
          </div>
        ) : (
          <div style={{ display: "grid", gridTemplateColumns: "repeat(3, minmax(0, 1fr))", gap: 10 }}>
            <MiniMetric label="Follows" value={insights?.follows || 0} />
            <MiniMetric label="Views" value={insights?.views ?? insights?.impressions ?? 0} />
            <MiniMetric label="Engagements" value={insights?.post_engagements || 0} />
          </div>
        )}
        <div style={{ marginTop: 12, color: "var(--dmuted2)", fontSize: 12 }}>
          Fetched {data?.fetched_at ? formatDate(data.fetched_at) : "-"}
        </div>
      </div>
    </div>
  );
}

function PagePostsTable({
  posts,
  selectedPostId,
  onSelect,
}: {
  posts: FacebookPageAnalyticsPost[];
  selectedPostId: string;
  onSelect: (id: string) => void;
}) {
  return (
    <div>
      <div style={sectionLabelStyle}>Published Page Posts</div>
      <div className="settings-section">
        <div className="settings-section-body">
          <table style={tableStyle}>
            <thead>
              <tr>
                <th style={thStyle}>Post</th>
                <th style={thStyle}>Published</th>
                <th style={thRightStyle}>Likes</th>
                <th style={thRightStyle}>Comments</th>
                <th style={thRightStyle}>Shares</th>
                <th style={thRightStyle}>Clicks</th>
              </tr>
            </thead>
            <tbody>
              {posts.map((post) => (
                <tr
                  key={post.id}
                  className={`fbpa-post-row${post.id === selectedPostId ? " active" : ""}`}
                  onClick={() => onSelect(post.id)}
                >
                  <td style={tdStyle}>
                    <div style={{ display: "flex", alignItems: "center", gap: 10, minWidth: 0 }}>
                      <PostThumb post={post} />
                      <div style={{ minWidth: 0 }}>
                        <div style={{ color: "var(--dtext)", fontWeight: 650, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", maxWidth: 360 }}>{postTitle(post)}</div>
                        <div style={{ color: "var(--dmuted2)", fontSize: 12, fontFamily: "var(--font-geist-mono), monospace" }}>{post.id}</div>
                      </div>
                    </div>
                  </td>
                  <td style={{ ...tdStyle, color: "var(--dmuted)", fontWeight: 500 }}>{formatDate(post.created_time)}</td>
                  <td style={tdRightStyle}>{formatNumber(post.likes)}</td>
                  <td style={tdRightStyle}>{formatNumber(post.comments)}</td>
                  <td style={tdRightStyle}>{formatNumber(post.shares)}</td>
                  <td style={tdRightStyle}>{formatNumber(post.clicks)}</td>
                </tr>
              ))}
              {posts.length === 0 && (
                <tr><td colSpan={6} style={{ ...tdStyle, textAlign: "center", color: "var(--dmuted2)", padding: 24 }}>No Facebook Page posts returned.</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

function PostDetailPanel({ post }: { post?: FacebookPageAnalyticsPost }) {
  return (
    <div className="fbpa-detail" style={{ position: "sticky", top: 18 }}>
      <div style={sectionLabelStyle}>Post Detail</div>
      <div className="settings-section">
        <div className="settings-section-body">
          {!post ? (
            <div style={{ color: "var(--dmuted2)", fontSize: 13, padding: 24, textAlign: "center" }}>No post selected.</div>
          ) : (
            <>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 12 }}>
                <span className="dbadge dbadge-blue"><span className="dbadge-dot" />{post.media_type || "post"}</span>
                {post.permalink_url ? (
                  <Link href={post.permalink_url} target="_blank" rel="noreferrer" style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "var(--daccent)", textDecoration: "none", fontSize: 13, fontWeight: 650 }}>
                    Open post
                    <ExternalLink style={{ width: 13, height: 13 }} />
                  </Link>
                ) : null}
              </div>
              {post.media_url ? (
                <div style={{ marginBottom: 14, borderRadius: 8, overflow: "hidden", border: "1px solid var(--dborder)", background: "var(--surface2)", aspectRatio: "16 / 9" }}>
                  <img src={post.media_url} alt="" style={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }} />
                </div>
              ) : null}
              <div style={{ color: "var(--dtext)", fontSize: 14, lineHeight: 1.65, whiteSpace: "pre-wrap", wordBreak: "break-word", marginBottom: 14 }}>
                {post.message || "(no message)"}
              </div>
              <div style={{ display: "grid", gap: 8, marginBottom: 14, fontSize: 13 }}>
                <InfoLine label="Published" value={formatDate(post.created_time)} />
                <InfoLine label="Post ID" value={post.id} mono />
              </div>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(2, minmax(0, 1fr))", gap: 10 }}>
                <DetailMetric label="Likes" value={post.likes} icon={ThumbsUp} />
                <DetailMetric label="Comments" value={post.comments} icon={MessageCircle} />
                <DetailMetric label="Shares" value={post.shares} icon={Share2} />
                <DetailMetric label="Clicks" value={post.clicks} icon={MousePointerClick} />
                <DetailMetric label="Video Views" value={post.video_views} icon={Video} />
                <DetailMetric label="Engagement" value={post.engagement_total} icon={BarChart3} />
              </div>
              {post.metrics_unavailable_reason ? (
                <div style={{ marginTop: 12, padding: "9px 10px", borderRadius: 8, border: "1px solid color-mix(in srgb, var(--warning) 28%, var(--dborder))", color: "var(--dmuted)", background: "var(--surface2)", fontSize: 12, lineHeight: 1.5 }}>
                  Some post insights were unavailable: {post.metrics_unavailable_reason}
                </div>
              ) : null}
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function EmptyFacebookState({ profileId }: { profileId: string }) {
  return (
    <div style={{ textAlign: "center", padding: 60, color: "var(--dmuted)" }}>
      <UserRound style={{ width: 40, height: 40, margin: "0 auto 12px", opacity: 0.35 }} />
      <div style={{ fontSize: 15, fontWeight: 650, color: "var(--dtext)", marginBottom: 6 }}>No Facebook Page connected</div>
      <div style={{ fontSize: 13, marginBottom: 14 }}>Connect a Facebook Page before using platform analytics.</div>
      <Link href={`/projects/${profileId}/accounts`} className="dbtn dbtn-primary" style={{ display: "inline-flex", textDecoration: "none" }}>
        Connect Page
      </Link>
    </div>
  );
}

function PageAvatar({ src, label }: { src: string; label: string }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [src]);
  const showImage = src && !failed;
  return (
    <div style={{ width: 50, height: 50, borderRadius: 8, background: "#1877f2", display: "grid", placeItems: "center", color: "white", fontWeight: 800, overflow: "hidden", flexShrink: 0 }}>
      {showImage ? (
        <img src={src} alt="" onError={() => setFailed(true)} style={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }} />
      ) : (
        label.slice(0, 2).toUpperCase()
      )}
    </div>
  );
}

function PostThumb({ post }: { post: FacebookPageAnalyticsPost }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [post.media_url]);
  const showImage = post.media_url && !failed;
  const Icon = post.media_type === "video" ? Video : post.media_type === "image" ? ImageIcon : FileText;
  return (
    <div className="fbpa-thumb">
      {showImage ? (
        <img src={post.media_url} alt="" onError={() => setFailed(true)} style={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }} />
      ) : (
        <Icon style={{ width: 16, height: 16 }} />
      )}
    </div>
  );
}

function InfoLine({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div style={{ display: "grid", gridTemplateColumns: "96px minmax(0, 1fr)", gap: 10, alignItems: "baseline" }}>
      <span style={{ color: "var(--dmuted2)", fontSize: 12 }}>{label}</span>
      <span style={{ color: "var(--dtext)", fontSize: 13, lineHeight: 1.55, wordBreak: "break-word", fontFamily: mono ? "var(--font-geist-mono), monospace" : undefined }}>{value || "-"}</span>
    </div>
  );
}

function MiniMetric({ label, value }: { label: string; value: number }) {
  return (
    <div style={{ border: "1px solid var(--dborder)", borderRadius: 8, background: "var(--surface2)", padding: "12px" }}>
      <div style={{ color: "var(--dmuted2)", fontSize: 11, fontWeight: 650, marginBottom: 8 }}>{label}</div>
      <div style={{ color: "var(--dtext)", fontFamily: "var(--font-geist-mono), monospace", fontSize: 20, fontWeight: 700 }}>{formatNumber(value)}</div>
    </div>
  );
}

function DetailMetric({ label, value, icon: Icon }: { label: string; value: number; icon: LucideIcon }) {
  return (
    <div style={{ border: "1px solid var(--dborder)", borderRadius: 8, background: "var(--surface2)", padding: "10px 11px" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8, marginBottom: 8 }}>
        <span style={{ color: "var(--dmuted2)", fontSize: 11, fontWeight: 650 }}>{label}</span>
        <Icon style={{ width: 14, height: 14, color: "var(--dmuted2)" }} />
      </div>
      <div style={{ color: "var(--dtext)", fontFamily: "var(--font-geist-mono), monospace", fontWeight: 700 }}>{formatNumber(value)}</div>
    </div>
  );
}

function sumEngagement(posts: FacebookPageAnalyticsPost[]): number {
  return posts.reduce((sum, post) => sum + post.engagement_total, 0);
}
