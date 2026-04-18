"use client";

import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { useCallback, useEffect, useMemo, useState } from "react";

import { listAdminPosts, type AdminPostListParams, type AdminPostRow } from "@/lib/api";

import { AdminShell, StatCard, fmtNumber, fmtRelative } from "../_components/admin-ui";

const STATUS_OPTIONS = ["all", "draft", "scheduled", "publishing", "published", "failed", "canceled", "archived"] as const;
const PLATFORM_OPTIONS = ["all", "twitter", "linkedin", "instagram", "threads", "tiktok", "youtube", "bluesky"] as const;
const SOURCE_OPTIONS = ["all", "ui", "dashboard", "api", "mcp"] as const;
const DAY_OPTIONS = [7, 30, 90] as const;

export default function AdminPostsPage() {
  const { getToken } = useAuth();
  const [posts, setPosts] = useState<AdminPostRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState<(typeof STATUS_OPTIONS)[number]>("all");
  const [platform, setPlatform] = useState<(typeof PLATFORM_OPTIONS)[number]>("all");
  const [source, setSource] = useState<(typeof SOURCE_OPTIONS)[number]>("all");
  const [days, setDays] = useState<(typeof DAY_OPTIONS)[number]>(30);
  const limit = 100;

  const loadPosts = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const params: AdminPostListParams = {
        search: search || undefined,
        status: status !== "all" ? status : undefined,
        platform: platform !== "all" ? platform : undefined,
        source: source !== "all" ? source : undefined,
        days,
        limit,
      };
      const res = await listAdminPosts(token, params);
      setPosts(res.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, [days, getToken, platform, search, source, status]);

  useEffect(() => {
    loadPosts();
  }, [loadPosts]);

  useEffect(() => {
    const timer = setTimeout(() => setSearch(searchInput), 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  const failedCount = useMemo(() => posts.filter((post) => post.status === "failed").length, [posts]);
  const scheduledCount = useMemo(() => posts.filter((post) => post.status === "scheduled").length, [posts]);
  const publishedCount = useMemo(() => posts.filter((post) => post.status === "published").length, [posts]);
  const affectedUsers = useMemo(() => new Set(posts.map((post) => post.user_id)).size, [posts]);

  return (
    <AdminShell title="Posts" loading={loading} onRefresh={loadPosts}>
      {error && (
        <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
          {error}
        </div>
      )}

      <div className="ad-section-header">
        <div className="ad-section-title">Publishing activity</div>
        <div className="ad-section-meta">Recent cross-tenant post volume and delivery state</div>
      </div>

      <div className="ad-stat-grid">
        <StatCard label="Posts" value={fmtNumber(posts.length)} sub={`Last ${days} days`} />
        <StatCard label="Published" value={fmtNumber(publishedCount)} sub={posts.length > 0 ? `${((publishedCount / posts.length) * 100).toFixed(0)}% of current set` : "—"} />
        <StatCard label="Failed" value={fmtNumber(failedCount)} subColor={failedCount > 0 ? "down" : undefined} sub={posts.length > 0 ? `${((failedCount / posts.length) * 100).toFixed(0)}% of current set` : "—"} />
        <StatCard label="Scheduled" value={fmtNumber(scheduledCount)} sub={`${fmtNumber(affectedUsers)} users in current set`} valueColor="accent" />
      </div>

      <div className="ad-filter-bar">
        <input
          className="ad-search"
          placeholder="Search by user, workspace, caption, or post ID..."
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          style={{ width: 320 }}
        />
        <select value={status} onChange={(e) => setStatus(e.target.value as typeof status)}>
          {STATUS_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All Statuses" : `Status: ${value}`}
            </option>
          ))}
        </select>
        <select value={platform} onChange={(e) => setPlatform(e.target.value as typeof platform)}>
          {PLATFORM_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All Platforms" : `Platform: ${value}`}
            </option>
          ))}
        </select>
        <select value={source} onChange={(e) => setSource(e.target.value as typeof source)}>
          {SOURCE_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All Sources" : `Source: ${value}`}
            </option>
          ))}
        </select>
        <select value={days} onChange={(e) => setDays(Number(e.target.value) as typeof days)}>
          {DAY_OPTIONS.map((value) => (
            <option key={value} value={value}>
              Last {value} days
            </option>
          ))}
        </select>
      </div>

      <div className="ad-tbl-wrap ad-tbl-static">
        <table>
          <thead>
            <tr>
              <th>Post</th>
              <th>Status</th>
              <th>Targets</th>
              <th>Source</th>
              <th>Workspace</th>
              <th>User</th>
              <th>Created</th>
              <th>Delivery</th>
            </tr>
          </thead>
          <tbody>
            {loading && posts.length === 0 ? (
              <tr><td colSpan={8} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>Loading…</td></tr>
            ) : posts.length === 0 ? (
              <tr><td colSpan={8} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>No posts matched the current filters.</td></tr>
            ) : (
              posts.map((post) => {
                const statusClass =
                  post.status === "failed" ? "ad-badge ad-b-blue" :
                  post.status === "published" ? "ad-badge ad-b-gray" :
                  "ad-badge ad-b-gray";
                return (
                  <tr key={post.post_id}>
                    <td style={{ minWidth: 280 }}>
                      <div style={{ fontWeight: 500 }}>
                        {post.caption?.slice(0, 110) || "No caption"}
                      </div>
                      <div className="ad-mono" style={{ marginTop: 3 }}>
                        {post.post_id.slice(0, 16)}
                      </div>
                    </td>
                    <td>
                      <span className={statusClass} style={post.status === "failed" ? { background: "var(--danger-soft)", color: "var(--danger)", borderColor: "color-mix(in srgb, var(--danger) 20%, transparent)" } : post.status === "published" ? { background: "var(--success-soft)", color: "var(--success)", borderColor: "color-mix(in srgb, var(--success) 20%, transparent)" } : undefined}>
                        {post.status}
                      </span>
                    </td>
                    <td>
                      {post.platforms.length > 0 ? (
                        <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
                          {post.platforms.map((item) => (
                            <span key={`${post.post_id}-${item}`} className="ad-badge ad-b-gray">{item}</span>
                          ))}
                        </div>
                      ) : (
                        <span style={{ color: "var(--dmuted2)", fontSize: 11 }}>—</span>
                      )}
                    </td>
                    <td><span className="ad-badge ad-b-gray">{post.source}</span></td>
                    <td>{post.workspace_name}</td>
                    <td>
                      <Link href={`/admin/users?user=${post.user_id}`} className="ad-link">
                        {post.user_email}
                      </Link>
                    </td>
                    <td style={{ whiteSpace: "nowrap" }}>
                      <div>{fmtRelative(post.created_at)}</div>
                      {post.scheduled_at ? <div className="ad-mono" style={{ marginTop: 3 }}>sched {fmtRelative(post.scheduled_at)}</div> : null}
                    </td>
                    <td>
                      <div style={{ fontSize: 11.5 }}>
                        {fmtNumber(post.published_result_count)} ok / {fmtNumber(post.failed_result_count)} failed
                      </div>
                      <div className="ad-usage-bar" style={{ width: 88, marginTop: 5 }}>
                        <div
                          className={post.failed_result_count > 0 ? "ad-uf-r" : "ad-uf-g"}
                          style={{
                            width: `${post.result_count > 0 ? (post.published_result_count / post.result_count) * 100 : 0}%`,
                            height: "100%",
                          }}
                        />
                      </div>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </AdminShell>
  );
}
