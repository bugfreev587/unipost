"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import {
  listSocialPosts, listSocialAccounts, getPostAnalytics,
  type SocialPost, type SocialAccount, type PostAnalytics,
} from "@/lib/api";
import { PlatformIcon } from "@/components/platform-icons";
import { BarChart3, Eye, Heart, MessageCircle, Share2, TrendingUp, RefreshCw } from "lucide-react";

type PlatformStats = {
  platform: string;
  posts: number;
  views: number;
  likes: number;
  comments: number;
  shares: number;
  engagementRate: number;
};

export default function AnalyticsPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [posts, setPosts] = useState<SocialPost[]>([]);
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [allAnalytics, setAllAnalytics] = useState<PostAnalytics[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingAnalytics, setLoadingAnalytics] = useState(false);
  const [analyticsLoaded, setAnalyticsLoaded] = useState(false);

  const loadData = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const [postsRes, accountsRes] = await Promise.all([
        listSocialPosts(token, projectId),
        listSocialAccounts(token, projectId),
      ]);
      setPosts(postsRes.data || []);
      setAccounts(accountsRes.data || []);
    } catch (err) { console.error("Failed to load:", err); }
    finally { setLoading(false); }
  }, [getToken, projectId]);

  useEffect(() => { loadData(); }, [loadData]);

  async function loadAllAnalytics() {
    setLoadingAnalytics(true);
    try {
      const token = await getToken();
      if (!token) return;
      const published = posts.filter((p) => p.status === "published");
      const results = await Promise.allSettled(
        published.map((p) => getPostAnalytics(token, projectId, p.id))
      );
      const all: PostAnalytics[] = [];
      results.forEach((r) => {
        if (r.status === "fulfilled" && r.value.data) {
          all.push(...r.value.data);
        }
      });
      setAllAnalytics(all);
      setAnalyticsLoaded(true);
    } catch (err) { console.error("Failed to load analytics:", err); }
    finally { setLoadingAnalytics(false); }
  }

  // Derived stats
  const totalPosts = posts.length;
  const published = posts.filter((p) => p.status === "published").length;
  const scheduled = posts.filter((p) => p.status === "scheduled").length;
  const failed = posts.filter((p) => p.status === "failed").length;
  const partial = posts.filter((p) => p.status === "partial").length;

  // Platform post counts from results
  const platformPostCounts: Record<string, number> = {};
  posts.forEach((p) => {
    p.results?.forEach((r) => {
      if (r.platform) {
        platformPostCounts[r.platform] = (platformPostCounts[r.platform] || 0) + 1;
      }
    });
  });

  // Platform analytics aggregates
  const platformStats: PlatformStats[] = [];
  if (analyticsLoaded) {
    const byPlatform: Record<string, { views: number; likes: number; comments: number; shares: number; count: number }> = {};
    allAnalytics.forEach((a) => {
      if (!byPlatform[a.platform]) {
        byPlatform[a.platform] = { views: 0, likes: 0, comments: 0, shares: 0, count: 0 };
      }
      const s = byPlatform[a.platform];
      s.views += a.views;
      s.likes += a.likes;
      s.comments += a.comments;
      s.shares += a.shares;
      s.count++;
    });
    Object.entries(byPlatform).forEach(([platform, s]) => {
      const total = s.likes + s.comments + s.shares;
      platformStats.push({
        platform,
        posts: s.count,
        views: s.views,
        likes: s.likes,
        comments: s.comments,
        shares: s.shares,
        engagementRate: s.views > 0 ? total / s.views : 0,
      });
    });
    platformStats.sort((a, b) => b.views - a.views);
  }

  // Totals
  const totalViews = allAnalytics.reduce((s, a) => s + a.views, 0);
  const totalLikes = allAnalytics.reduce((s, a) => s + a.likes, 0);
  const totalComments = allAnalytics.reduce((s, a) => s + a.comments, 0);
  const totalShares = allAnalytics.reduce((s, a) => s + a.shares, 0);
  const totalEngagement = totalViews > 0 ? (totalLikes + totalComments + totalShares) / totalViews : 0;

  // Per-post analytics (for the table)
  const postAnalyticsMap: Record<string, PostAnalytics[]> = {};
  allAnalytics.forEach((a) => {
    if (!postAnalyticsMap[a.post_id]) postAnalyticsMap[a.post_id] = [];
    postAnalyticsMap[a.post_id].push(a);
  });

  if (loading) return <div style={{ color: "var(--dmuted)" }}>Loading...</div>;

  return (
    <>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>Analytics</div>
          <div style={{ fontSize: 14, color: "#aaa", marginTop: 6 }}>Post performance and engagement metrics</div>
        </div>
        {!analyticsLoaded && published > 0 && (
          <button
            className="dbtn dbtn-primary"
            onClick={loadAllAnalytics}
            disabled={loadingAnalytics}
            style={{ gap: 6 }}
          >
            <RefreshCw style={{ width: 13, height: 13, animation: loadingAnalytics ? "spin 1s linear infinite" : "none" }} />
            {loadingAnalytics ? "Fetching..." : "Load Analytics"}
          </button>
        )}
        {analyticsLoaded && (
          <button
            className="dbtn dbtn-ghost"
            onClick={loadAllAnalytics}
            disabled={loadingAnalytics}
            style={{ gap: 6 }}
          >
            <RefreshCw style={{ width: 13, height: 13, animation: loadingAnalytics ? "spin 1s linear infinite" : "none" }} />
            Refresh
          </button>
        )}
      </div>

      {/* KPI Cards - Post Status */}
      <div style={{ fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--dmuted2)", marginBottom: 8 }}>
        Post Overview
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 12, marginBottom: 28 }}>
        <KPICard label="Total Posts" value={totalPosts} />
        <KPICard label="Published" value={published} color="var(--daccent)" />
        <KPICard label="Scheduled" value={scheduled} color="var(--info)" />
        <KPICard label="Failed" value={failed + partial} color={failed > 0 ? "var(--danger)" : "var(--dmuted)"} />
      </div>

      {/* KPI Cards - Engagement (only if analytics loaded) */}
      {analyticsLoaded && (
        <>
          <div style={{ fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--dmuted2)", marginBottom: 8 }}>
            Engagement Totals
          </div>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(5, 1fr)", gap: 12, marginBottom: 28 }}>
            <KPICard label="Views" value={totalViews} icon={<Eye style={{ width: 14, height: 14 }} />} format />
            <KPICard label="Likes" value={totalLikes} icon={<Heart style={{ width: 14, height: 14 }} />} format />
            <KPICard label="Comments" value={totalComments} icon={<MessageCircle style={{ width: 14, height: 14 }} />} format />
            <KPICard label="Shares" value={totalShares} icon={<Share2 style={{ width: 14, height: 14 }} />} format />
            <KPICard label="Engagement" value={totalEngagement} icon={<TrendingUp style={{ width: 14, height: 14 }} />} percent />
          </div>
        </>
      )}

      {/* Platform Breakdown */}
      <div style={{ fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--dmuted2)", marginBottom: 8 }}>
        By Platform
      </div>
      {!analyticsLoaded ? (
        <div className="settings-section" style={{ marginBottom: 28 }}>
          <div className="settings-section-body">
            <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
              <thead>
                <tr>
                  <th style={thStyle}>Platform</th>
                  <th style={{ ...thStyle, textAlign: "right" }}>Posts</th>
                  <th style={{ ...thStyle, textAlign: "right" }}>Accounts</th>
                </tr>
              </thead>
              <tbody>
                {Object.entries(platformPostCounts).sort((a, b) => b[1] - a[1]).map(([p, count]) => (
                  <tr key={p}>
                    <td style={tdStyle}>
                      <span style={{ display: "flex", alignItems: "center", gap: 8 }}>
                        <PlatformIcon platform={p} size={14} />
                        <span style={{ fontWeight: 500, color: "var(--dtext)" }}>{p}</span>
                      </span>
                    </td>
                    <td style={{ ...tdStyle, textAlign: "right", fontVariantNumeric: "tabular-nums" }}>{count}</td>
                    <td style={{ ...tdStyle, textAlign: "right", fontVariantNumeric: "tabular-nums" }}>
                      {accounts.filter((a) => a.platform === p).length}
                    </td>
                  </tr>
                ))}
                {Object.keys(platformPostCounts).length === 0 && (
                  <tr><td colSpan={3} style={{ ...tdStyle, textAlign: "center", color: "var(--dmuted2)" }}>No posts yet</td></tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      ) : (
        <div className="settings-section" style={{ marginBottom: 28 }}>
          <div className="settings-section-body">
            <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
              <thead>
                <tr>
                  <th style={thStyle}>Platform</th>
                  <th style={{ ...thStyle, textAlign: "right" }}>Posts</th>
                  <th style={{ ...thStyle, textAlign: "right" }}>Views</th>
                  <th style={{ ...thStyle, textAlign: "right" }}>Likes</th>
                  <th style={{ ...thStyle, textAlign: "right" }}>Comments</th>
                  <th style={{ ...thStyle, textAlign: "right" }}>Shares</th>
                  <th style={{ ...thStyle, textAlign: "right" }}>Eng. Rate</th>
                </tr>
              </thead>
              <tbody>
                {platformStats.map((s) => (
                  <tr key={s.platform}>
                    <td style={tdStyle}>
                      <span style={{ display: "flex", alignItems: "center", gap: 8 }}>
                        <PlatformIcon platform={s.platform} size={14} />
                        <span style={{ fontWeight: 500, color: "var(--dtext)" }}>{s.platform}</span>
                      </span>
                    </td>
                    <td style={tdRight}>{s.posts}</td>
                    <td style={tdRight}>{s.views.toLocaleString()}</td>
                    <td style={tdRight}>{s.likes.toLocaleString()}</td>
                    <td style={tdRight}>{s.comments.toLocaleString()}</td>
                    <td style={tdRight}>{s.shares.toLocaleString()}</td>
                    <td style={tdRight}>
                      <span style={{ color: s.engagementRate > 0.05 ? "var(--daccent)" : "var(--dmuted)" }}>
                        {(s.engagementRate * 100).toFixed(1)}%
                      </span>
                    </td>
                  </tr>
                ))}
                {platformStats.length === 0 && (
                  <tr><td colSpan={7} style={{ ...tdStyle, textAlign: "center", color: "var(--dmuted2)" }}>No analytics data</td></tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Per-Post Metrics Table */}
      {analyticsLoaded && Object.keys(postAnalyticsMap).length > 0 && (
        <>
          <div style={{ fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--dmuted2)", marginBottom: 8 }}>
            By Post
          </div>
          <div className="settings-section">
            <div className="settings-section-body">
              <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
                <thead>
                  <tr>
                    <th style={thStyle}>Caption</th>
                    <th style={thStyle}>Platform</th>
                    <th style={{ ...thStyle, textAlign: "right" }}>Views</th>
                    <th style={{ ...thStyle, textAlign: "right" }}>Likes</th>
                    <th style={{ ...thStyle, textAlign: "right" }}>Comments</th>
                    <th style={{ ...thStyle, textAlign: "right" }}>Shares</th>
                    <th style={{ ...thStyle, textAlign: "right" }}>Eng.</th>
                  </tr>
                </thead>
                <tbody>
                  {posts
                    .filter((p) => postAnalyticsMap[p.id])
                    .map((post) =>
                      postAnalyticsMap[post.id].map((a, i) => (
                        <tr key={`${post.id}-${i}`}>
                          {i === 0 ? (
                            <td style={{ ...tdStyle, maxWidth: 200 }} rowSpan={postAnalyticsMap[post.id].length}>
                              <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: "var(--dtext)", fontWeight: 500 }}>
                                {post.caption || "(no caption)"}
                              </span>
                              <span style={{ fontSize: 11, color: "var(--dmuted2)" }}>
                                {new Date(post.created_at).toLocaleDateString("en-US", { month: "short", day: "numeric" })}
                              </span>
                            </td>
                          ) : null}
                          <td style={tdStyle}>
                            <span style={{ display: "flex", alignItems: "center", gap: 6 }}>
                              <PlatformIcon platform={a.platform} size={12} />
                              <span>{a.platform}</span>
                            </span>
                          </td>
                          <td style={tdRight}>{a.views.toLocaleString()}</td>
                          <td style={tdRight}>{a.likes.toLocaleString()}</td>
                          <td style={tdRight}>{a.comments.toLocaleString()}</td>
                          <td style={tdRight}>{a.shares.toLocaleString()}</td>
                          <td style={tdRight}>
                            <span style={{ color: a.engagement_rate > 0.05 ? "var(--daccent)" : "var(--dmuted)" }}>
                              {(a.engagement_rate * 100).toFixed(1)}%
                            </span>
                          </td>
                        </tr>
                      ))
                    )}
                </tbody>
              </table>
            </div>
          </div>
        </>
      )}

      {/* Empty state */}
      {totalPosts === 0 && (
        <div className="empty-state" style={{ padding: 60, textAlign: "center" }}>
          <BarChart3 style={{ width: 32, height: 32, color: "var(--dmuted2)", marginBottom: 12 }} />
          <div style={{ fontSize: 14, fontWeight: 500, color: "var(--dtext)", marginBottom: 4 }}>No posts yet</div>
          <div style={{ fontSize: 12.5, color: "var(--dmuted)" }}>
            Create your first post to start tracking analytics.
          </div>
        </div>
      )}

      <style>{`
        @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
      `}</style>
    </>
  );
}

// -- Sub-components --

function KPICard({ label, value, color, icon, format, percent }: {
  label: string;
  value: number;
  color?: string;
  icon?: React.ReactNode;
  format?: boolean;
  percent?: boolean;
}) {
  let display: string;
  if (percent) {
    display = (value * 100).toFixed(1) + "%";
  } else if (format) {
    display = value >= 1000000 ? (value / 1000000).toFixed(1) + "M"
      : value >= 1000 ? (value / 1000).toFixed(1) + "K"
      : value.toString();
  } else {
    display = value.toString();
  }

  return (
    <div className="stat-card">
      <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 8 }}>
        {icon && <span style={{ color: "var(--dmuted2)" }}>{icon}</span>}
        <span style={{ fontSize: 11, color: "var(--dmuted)", textTransform: "uppercase", letterSpacing: "0.06em", fontWeight: 600 }}>
          {label}
        </span>
      </div>
      <div style={{
        fontFamily: "var(--font-geist-mono), monospace",
        fontSize: 22, fontWeight: 600,
        color: color || "var(--dtext)",
        letterSpacing: -0.5,
        fontVariantNumeric: "tabular-nums",
      }}>
        {display}
      </div>
    </div>
  );
}

// -- Styles --

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
