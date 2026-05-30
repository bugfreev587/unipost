"use client";

import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import {
  listAdminPosts,
  listAdminPostsAggregates,
  type AdminPostListParams,
  type AdminPostRow,
  type AdminPostsAggregates,
} from "@/lib/api";

import { AdminShell, StatCard, bucketByLocalDay, fmtNumber, fmtRelative } from "../_components/admin-ui";
import { fmtAdminPostTimelineDate, getAdminPostPublishTimeline } from "./timeline";

const STATUS_OPTIONS = ["all", "draft", "scheduled", "publishing", "published", "failed", "canceled", "archived"] as const;
const PLATFORM_OPTIONS = ["all", "twitter", "linkedin", "instagram", "threads", "tiktok", "youtube", "bluesky", "facebook"] as const;
const SOURCE_OPTIONS = ["all", "ui", "dashboard", "api", "mcp"] as const;
const DAY_OPTIONS = [7, 30, 90] as const;

const PLATFORM_COLORS: Record<string, string> = {
  twitter: "#0f172a",
  linkedin: "#0a66c2",
  instagram: "#e1306c",
  threads: "#000000",
  tiktok: "#000000",
  youtube: "#ff0000",
  bluesky: "#0085ff",
  facebook: "#1877f2",
};

export default function AdminPostsPage() {
  const { getToken } = useAuth();
  const [posts, setPosts] = useState<AdminPostRow[]>([]);
  const [aggregates, setAggregates] = useState<AdminPostsAggregates | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [status, setStatus] = useState<(typeof STATUS_OPTIONS)[number]>("all");
  const [platform, setPlatform] = useState<(typeof PLATFORM_OPTIONS)[number]>("all");
  const [source, setSource] = useState<(typeof SOURCE_OPTIONS)[number]>("all");
  const [userId, setUserId] = useState<string>("");
  const [workspaceId, setWorkspaceId] = useState<string>("");
  const [days, setDays] = useState<(typeof DAY_OPTIONS)[number]>(30);
  // Filter dropdown options accumulate across loads so picking one filter
  // doesn't strand the others — once we've seen a user/workspace we keep
  // them selectable until a hard refresh.
  const [userOptions, setUserOptions] = useState<Array<{ id: string; email: string }>>([]);
  const [workspaceOptions, setWorkspaceOptions] = useState<Array<{ id: string; name: string }>>([]);
  const limit = 100;

  const loadAll = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const baseParams: AdminPostListParams = {
        search: search || undefined,
        status: status !== "all" ? status : undefined,
        platform: platform !== "all" ? platform : undefined,
        source: source !== "all" ? source : undefined,
        user_id: userId || undefined,
        workspace_id: workspaceId || undefined,
        days,
      };
      // Issue both calls in parallel — same filter set, separate
      // round trips because the row list is LIMITed but the
      // aggregates need to count the full filtered universe.
      const [listRes, aggRes] = await Promise.all([
        listAdminPosts(token, { ...baseParams, limit }),
        listAdminPostsAggregates(token, baseParams),
      ]);
      setPosts(listRes.data);
      setAggregates(aggRes.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, [days, getToken, platform, search, source, status, userId, workspaceId]);

  useEffect(() => {
    loadAll();
  }, [loadAll]);

  useEffect(() => {
    const timer = setTimeout(() => setSearch(searchInput), 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  // Accumulate user + workspace dropdown options as we see them across
  // loads. Without this, picking a user filter would shrink the user
  // dropdown to a single option, making it impossible to switch users
  // without first clearing.
  useEffect(() => {
    if (posts.length === 0) return;
    setUserOptions((prev) => {
      const map = new Map(prev.map((u) => [u.id, u.email]));
      for (const p of posts) map.set(p.user_id, p.user_email);
      return Array.from(map.entries())
        .map(([id, email]) => ({ id, email }))
        .sort((a, b) => a.email.localeCompare(b.email));
    });
    setWorkspaceOptions((prev) => {
      const map = new Map(prev.map((w) => [w.id, w.name]));
      for (const p of posts) map.set(p.workspace_id, p.workspace_name);
      return Array.from(map.entries())
        .map(([id, name]) => ({ id, name }))
        .sort((a, b) => a.name.localeCompare(b.name));
    });
  }, [posts]);

  const total = aggregates?.total_posts ?? 0;
  const published = aggregates?.by_status?.published ?? 0;
  const failed = aggregates?.by_status?.failed ?? 0;
  const scheduled = aggregates?.by_status?.scheduled ?? 0;
  const uniqueUsers = aggregates?.unique_users ?? 0;

  // Bucket the raw published/failed events into local-day buckets so a
  // late-evening publish doesn't land on the next UTC date in the chart.
  const dailyRows = useMemo(() => {
    if (!aggregates) return [] as { date: string; published: number; failed: number }[];
    return bucketByLocalDay(
      aggregates.events,
      days,
      (date) => ({ date, published: 0, failed: 0 }),
      (b, e) => {
        if (e.status === "published") b.published += 1;
        else if (e.status === "failed") b.failed += 1;
      },
      (e) => e.created_at,
    );
  }, [aggregates, days]);

  return (
    <AdminShell title="Posts" loading={loading} onRefresh={loadAll}>
      {error && (
        <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
          {error}
        </div>
      )}

      <div className="ad-section-header">
        <div className="ad-section-title">Publishing activity</div>
        <div className="ad-section-meta">Cross-tenant post volume and delivery state. All numbers respect the filters below.</div>
      </div>

      {/* Filter bar moved above the cards so it gates the entire view —
          headline cards, per-platform cards, time-series chart, and the
          row table are all driven by the same filter state. */}
      <div className="ad-filter-bar" style={{ marginBottom: 16 }}>
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
        <select value={userId} onChange={(e) => setUserId(e.target.value)}>
          <option value="">All Users</option>
          {userOptions.map((u) => (
            <option key={u.id} value={u.id}>{`User: ${u.email}`}</option>
          ))}
        </select>
        <select value={workspaceId} onChange={(e) => setWorkspaceId(e.target.value)}>
          <option value="">All Workspaces</option>
          {workspaceOptions.map((w) => (
            <option key={w.id} value={w.id}>{`Workspace: ${w.name}`}</option>
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

      <div className="ad-stat-grid">
        <StatCard label="Posts" value={fmtNumber(total)} sub={`Last ${days} days`} />
        <StatCard
          label="Published"
          value={fmtNumber(published)}
          sub={total > 0 ? `${((published / total) * 100).toFixed(0)}% of current set` : "—"}
        />
        <StatCard
          label="Failed"
          value={fmtNumber(failed)}
          subColor={failed > 0 ? "down" : undefined}
          sub={total > 0 ? `${((failed / total) * 100).toFixed(0)}% of current set` : "—"}
        />
        <StatCard
          label="Scheduled"
          value={fmtNumber(scheduled)}
          sub={`${fmtNumber(uniqueUsers)} users in current set`}
          valueColor="accent"
        />
      </div>

      {/* Per-platform row — RESULT-level so a multi-platform post that
          partially succeeds shows up correctly in each platform's
          numbers rather than getting hidden under "partial" status. */}
      {aggregates && aggregates.by_platform.length > 0 && (
        <>
          <div className="ad-section-header" style={{ marginTop: 24 }}>
            <div className="ad-section-title" style={{ fontSize: 14 }}>By platform</div>
            <div className="ad-section-meta">Result-level — one platform attempt per row of social_post_results</div>
          </div>
          <div className="ad-stat-grid" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(180px, 1fr))" }}>
            {aggregates.by_platform.map((p) => (
              <div key={p.platform} className="ad-stat-card">
                <div className="ad-stat-label" style={{ display: "flex", alignItems: "center", gap: 6 }}>
                  <span
                    aria-hidden
                    style={{
                      display: "inline-block",
                      width: 8,
                      height: 8,
                      borderRadius: 2,
                      background: PLATFORM_COLORS[p.platform] ?? "var(--dmuted)",
                    }}
                  />
                  {p.platform}
                </div>
                <div className="ad-stat-value" style={{ fontSize: 22 }}>
                  {fmtNumber(p.published)}
                  <span style={{ color: "var(--dmuted2)", fontWeight: 400, fontSize: 13 }}> / </span>
                  <span style={{ color: p.failed > 0 ? "var(--danger)" : "var(--dmuted)" }}>
                    {fmtNumber(p.failed)}
                  </span>
                </div>
                <div className="ad-stat-sub" style={{ color: "var(--dmuted)" }}>
                  ok / failed · {fmtNumber(p.total)} attempts
                </div>
              </div>
            ))}
          </div>
        </>
      )}

      {/* Time series — published vs failed by day. Counts come from the
          parent post status (matches the headline cards) so totals
          reconcile across the page. */}
      {aggregates && dailyRows.length > 0 && (
        <>
          <div className="ad-section-header" style={{ marginTop: 24 }}>
            <div className="ad-section-title" style={{ fontSize: 14 }}>Posts per day</div>
            <div className="ad-section-meta">Published vs failed, last {days} days</div>
          </div>
          <div
            style={{
              background: "var(--surface-raised)",
              border: "1px solid var(--dborder)",
              borderRadius: 12,
              padding: 16,
              height: 280,
            }}
          >
            <ResponsiveContainer width="100%" height="100%">
              <BarChart
                data={dailyRows}
                margin={{ top: 8, right: 16, left: 0, bottom: 4 }}
                barGap={4}
                barCategoryGap="24%"
              >
                <CartesianGrid strokeDasharray="3 3" stroke="var(--dborder)" />
                <XAxis
                  dataKey="date"
                  tick={{ fontSize: 11, fill: "var(--dmuted)" }}
                  tickFormatter={(v: string) => v.slice(5)}
                  stroke="var(--dborder)"
                />
                <YAxis
                  allowDecimals={false}
                  tick={{ fontSize: 11, fill: "var(--dmuted)" }}
                  stroke="var(--dborder)"
                />
                <Tooltip
                  contentStyle={{
                    background: "var(--surface-raised)",
                    border: "1px solid var(--dborder)",
                    borderRadius: 8,
                    fontSize: 12,
                  }}
                  labelStyle={{ color: "var(--dtext)" }}
                />
                <Bar
                  dataKey="published"
                  fill="var(--success)"
                  radius={[4, 4, 0, 0]}
                  name="Published"
                />
                <Bar
                  dataKey="failed"
                  fill="var(--danger)"
                  radius={[4, 4, 0, 0]}
                  name="Failed"
                />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </>
      )}

      <div className="ad-section-header" style={{ marginTop: 24 }}>
        <div className="ad-section-title" style={{ fontSize: 14 }}>
          Posts {posts.length === limit ? `(showing first ${limit})` : `(${posts.length})`}
        </div>
        <div className="ad-section-meta">Most recent first</div>
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
              <th>Publish Time</th>
              <th>Delivery</th>
            </tr>
          </thead>
          <tbody>
            {loading && posts.length === 0 ? (
              <tr><td colSpan={9} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>Loading…</td></tr>
            ) : posts.length === 0 ? (
              <tr><td colSpan={9} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>No posts matched the current filters.</td></tr>
            ) : (
              posts.map((post) => {
                const statusClass =
                  post.status === "failed" ? "ad-badge ad-b-blue" :
                  post.status === "published" ? "ad-badge ad-b-gray" :
                  "ad-badge ad-b-gray";
                const publishTimeline = getAdminPostPublishTimeline(post);
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
                    </td>
                    <td style={{ whiteSpace: "nowrap", minWidth: 132 }}>
                      {publishTimeline ? (
                        <>
                          <div>{fmtRelative(publishTimeline.at)}</div>
                          <div className="ad-mono" style={{ marginTop: 3 }}>
                            {publishTimeline.label} · {fmtAdminPostTimelineDate(publishTimeline.at)}
                          </div>
                        </>
                      ) : (
                        <span style={{ color: "var(--dmuted2)", fontSize: 11 }}>—</span>
                      )}
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
