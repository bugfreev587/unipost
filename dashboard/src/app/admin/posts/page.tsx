"use client";

import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { Check, Copy, ExternalLink, X } from "lucide-react";
import {
  type CSSProperties,
  type KeyboardEvent,
  type MouseEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
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
import { adminUserIdentifierLabel } from "@/lib/admin-privacy";

import { AdminShell, StatCard, bucketByLocalDayRange, fmtNumber, fmtRelative } from "../_components/admin-ui";
import { SearchHistoryInput } from "../_components/search-history-input";
import { fmtAdminPostTimelineDate, getAdminPostPublishTimeline } from "./timeline";

const STATUS_OPTIONS = ["all", "draft", "scheduled", "publishing", "published", "partial", "failed", "canceled", "archived"] as const;
const RESULT_STATUS_OPTIONS = ["all", "failed"] as const;
const PLATFORM_OPTIONS = ["all", "twitter", "linkedin", "instagram", "threads", "tiktok", "youtube", "bluesky", "facebook"] as const;
const SOURCE_OPTIONS = ["all", "ui", "dashboard", "api", "mcp"] as const;

const PERIOD_OPTIONS = [
  { value: "today", label: "Today" },
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
  { value: "90d", label: "Last 90 days" },
  { value: "this_month", label: "This Month" },
  { value: "last_month", label: "Last Month" },
  { value: "all", label: "All" },
] as const;

type PeriodValue = (typeof PERIOD_OPTIONS)[number]["value"];

// Card / section-meta wording for each period ("Last 30 days" etc.).
const PERIOD_SUBS: Record<PeriodValue, string> = {
  today: "Today",
  "7d": "Last 7 days",
  "30d": "Last 30 days",
  "90d": "Last 90 days",
  this_month: "This month",
  last_month: "Last month",
  all: "All time",
};

// Calendar periods send absolute [start, end) bounds computed in the
// admin's local timezone — the server can't know where local midnight
// or the first of the month falls. Rolling periods keep the days param.
function periodToRequestParams(period: PeriodValue): Pick<AdminPostListParams, "days" | "start_at" | "end_at" | "all"> {
  const now = new Date();
  switch (period) {
    case "7d":
      return { days: 7 };
    case "30d":
      return { days: 30 };
    case "90d":
      return { days: 90 };
    case "today": {
      const start = new Date(now);
      start.setHours(0, 0, 0, 0);
      return { start_at: start.toISOString() };
    }
    case "this_month":
      return { start_at: new Date(now.getFullYear(), now.getMonth(), 1).toISOString() };
    case "last_month":
      return {
        start_at: new Date(now.getFullYear(), now.getMonth() - 1, 1).toISOString(),
        end_at: new Date(now.getFullYear(), now.getMonth(), 1).toISOString(),
      };
    case "all":
      return { all: true };
  }
}

// Inclusive local-day range the chart should render for a period. "All"
// stretches back to the earliest event so no data falls off the chart.
function periodChartRange(period: PeriodValue, events: Array<{ created_at: string }>): { start: Date; end: Date } {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  switch (period) {
    case "today":
      return { start: today, end: today };
    case "7d":
    case "30d":
    case "90d": {
      const days = period === "7d" ? 7 : period === "30d" ? 30 : 90;
      const start = new Date(today);
      start.setDate(start.getDate() - (days - 1));
      return { start, end: today };
    }
    case "this_month":
      return { start: new Date(today.getFullYear(), today.getMonth(), 1), end: today };
    case "last_month":
      return {
        start: new Date(today.getFullYear(), today.getMonth() - 1, 1),
        end: new Date(today.getFullYear(), today.getMonth(), 0),
      };
    case "all": {
      let start = today;
      for (const e of events) {
        const d = new Date(e.created_at);
        if (d < start) start = d;
      }
      return { start, end: today };
    }
  }
}

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
  const [resultStatus, setResultStatus] = useState<(typeof RESULT_STATUS_OPTIONS)[number]>("all");
  const [platform, setPlatform] = useState<(typeof PLATFORM_OPTIONS)[number]>("all");
  const [source, setSource] = useState<(typeof SOURCE_OPTIONS)[number]>("all");
  const [userId, setUserId] = useState<string>("");
  const [workspaceId, setWorkspaceId] = useState<string>("");
  const [period, setPeriod] = useState<PeriodValue>("30d");
  const [hideUsers, setHideUsers] = useState(false);
  const [selectedPostId, setSelectedPostId] = useState<string | null>(null);
  const [drawerTab, setDrawerTab] = useState<"attributes" | "raw">("attributes");
  const [rawCopied, setRawCopied] = useState(false);
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
        result_status: resultStatus !== "all" ? resultStatus : undefined,
        platform: platform !== "all" ? platform : undefined,
        source: source !== "all" ? source : undefined,
        user_id: userId || undefined,
        workspace_id: workspaceId || undefined,
        ...periodToRequestParams(period),
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
  }, [getToken, period, platform, resultStatus, search, source, status, userId, workspaceId]);

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
    const { start, end } = periodChartRange(period, aggregates.events);
    return bucketByLocalDayRange(
      aggregates.events,
      start,
      end,
      (date) => ({ date, published: 0, failed: 0 }),
      (b, e) => {
        if (e.status === "published") b.published += 1;
        else if (e.status === "failed") b.failed += 1;
      },
      (e) => e.created_at,
    );
  }, [aggregates, period]);

  const selectedPost = useMemo(() => {
    if (!selectedPostId) return null;
    return posts.find((post, idx) => postKey(post, idx) === selectedPostId) ?? null;
  }, [posts, selectedPostId]);
  const selectedPostForDisplay = useMemo(() => {
    if (!selectedPost || !hideUsers) return selectedPost;
    return {
      ...selectedPost,
      user_id: adminUserIdentifierLabel(selectedPost.user_id, hideUsers),
      user_email: adminUserIdentifierLabel(selectedPost.user_email, hideUsers),
    };
  }, [hideUsers, selectedPost]);

  useEffect(() => {
    if (selectedPostId && !selectedPost) {
      setSelectedPostId(null);
    }
  }, [selectedPost, selectedPostId]);

  const openPostDetail = useCallback((post: AdminPostRow, idx: number) => {
    setSelectedPostId(postKey(post, idx));
    setDrawerTab("attributes");
    setRawCopied(false);
  }, []);

  const closeDetail = useCallback(() => {
    setSelectedPostId(null);
    setRawCopied(false);
  }, []);

  const copyRawPost = useCallback(async () => {
    if (!selectedPostForDisplay) return;
    await navigator.clipboard.writeText(JSON.stringify(selectedPostForDisplay, null, 2));
    setRawCopied(true);
    window.setTimeout(() => setRawCopied(false), 1200);
  }, [selectedPostForDisplay]);

  const stopLinkClick = useCallback((event: MouseEvent<HTMLAnchorElement>) => {
    event.stopPropagation();
  }, []);

  const selectedPublishTimeline = selectedPost ? getAdminPostPublishTimeline(selectedPost) : null;

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
        <SearchHistoryInput
          fieldKey="admin.posts.search"
          className="ad-search"
          placeholder="Search by user, workspace, caption, or post ID..."
          value={searchInput}
          onChange={setSearchInput}
          style={{ width: 320 }}
        />
        <select value={status} onChange={(e) => setStatus(e.target.value as typeof status)}>
          {STATUS_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All Statuses" : `Status: ${value}`}
            </option>
          ))}
        </select>
        <select value={resultStatus} onChange={(e) => setResultStatus(e.target.value as typeof resultStatus)}>
          {RESULT_STATUS_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All Deliveries" : "Has failed attempts"}
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
            <option key={u.id} value={u.id}>{`User: ${adminUserIdentifierLabel(u.email, hideUsers)}`}</option>
          ))}
        </select>
        <select value={workspaceId} onChange={(e) => setWorkspaceId(e.target.value)}>
          <option value="">All Workspaces</option>
          {workspaceOptions.map((w) => (
            <option key={w.id} value={w.id}>{`Workspace: ${w.name}`}</option>
          ))}
        </select>
        <select value={period} onChange={(e) => setPeriod(e.target.value as PeriodValue)}>
          {PERIOD_OPTIONS.map(({ value, label }) => (
            <option key={value} value={value}>
              {label}
            </option>
          ))}
        </select>
        <select value={hideUsers ? "hide" : "show"} onChange={(e) => setHideUsers(e.target.value === "hide")}>
          <option value="show">Privacy: Show Users</option>
          <option value="hide">Privacy: Hide Users</option>
        </select>
      </div>

      <div className="ad-stat-grid">
        <StatCard label="Posts" value={fmtNumber(total)} sub={PERIOD_SUBS[period]} />
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
            <div className="ad-section-meta">Published vs failed, {PERIOD_SUBS[period].toLowerCase()}</div>
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
              posts.map((post, idx) => {
                const id = postKey(post, idx);
                const selected = id === selectedPostId;
                const statusClass =
                  post.status === "failed" ? "ad-badge ad-b-blue" :
                  post.status === "published" ? "ad-badge ad-b-gray" :
                  "ad-badge ad-b-gray";
                const publishTimeline = getAdminPostPublishTimeline(post);
                return (
                  <tr
                    key={id}
                    role="button"
                    tabIndex={0}
                    aria-label={`Open post details for ${post.post_id}`}
                    aria-pressed={selected}
                    onClick={() => openPostDetail(post, idx)}
                    onKeyDown={(event) => handlePostKeyDown(event, () => openPostDetail(post, idx))}
                    style={{
                      cursor: "pointer",
                      outline: "none",
                      background: selected ? "color-mix(in srgb, var(--daccent) 9%, var(--surface))" : undefined,
                    }}
                  >
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
                      <Link href={`/admin/users?user=${post.user_id}`} className="ad-link" onClick={stopLinkClick}>
                        {adminUserIdentifierLabel(post.user_email, hideUsers)}
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

      {selectedPost ? (
        <aside
          className="posts-detail-drawer"
          role="dialog"
          aria-label="Post detail"
          style={drawerStyle}
        >
          <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12 }}>
            <div>
              <div style={{ fontSize: 18, fontWeight: 700 }}>Post detail</div>
              <div style={{ color: "var(--dmuted)", marginTop: 4 }}>
                {selectedPost.status} · {selectedPost.source}
              </div>
              <div className="ad-mono" style={{ marginTop: 5 }}>{selectedPost.post_id}</div>
            </div>
            <button type="button" onClick={closeDetail} style={iconButtonStyle} aria-label="Close post details">
              <X size={16} />
            </button>
          </div>

          <DrawerTabs
            active={drawerTab}
            onChange={setDrawerTab}
            rightSlot={
              drawerTab === "raw" ? (
                <button
                  type="button"
                  onClick={copyRawPost}
                  style={drawerCopyButtonStyle}
                  aria-label="Copy raw post JSON"
                >
                  {rawCopied ? (
                    <>
                      <Check size={12} />
                      Copied
                    </>
                  ) : (
                    <>
                      <Copy size={12} />
                      Copy
                    </>
                  )}
                </button>
              ) : null
            }
          />

          {drawerTab === "raw" ? (
            <pre style={drawerRawJsonStyle}>{JSON.stringify(selectedPostForDisplay, null, 2)}</pre>
          ) : (
            <div style={{ display: "grid", gap: 14 }}>
              <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                <FieldChip label="status" value={selectedPost.status} />
                <FieldChip label="source" value={selectedPost.source} />
                <FieldChip label="targets" value={String(selectedPost.platforms.length)} />
                <FieldChip label="created" value={formatDateTime(selectedPost.created_at)} />
              </div>

              <div style={sectionStyle}>
                <div style={sectionTitleStyle}>Caption</div>
                <div style={captionStyle}>{selectedPost.caption || "No caption"}</div>
              </div>

              <div style={sectionStyle}>
                <div style={sectionTitleStyle}>Context</div>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                  <FieldChip label="workspace" value={selectedPost.workspace_id} />
                  <FieldChip label="workspace_name" value={selectedPost.workspace_name} />
                  <FieldChip label="user" value={adminUserIdentifierLabel(selectedPost.user_id, hideUsers)} />
                  <FieldChip label="owner" value={adminUserIdentifierLabel(selectedPost.user_email, hideUsers)} />
                  <FieldChip label="post_id" value={selectedPost.post_id} />
                </div>
                <div style={{ marginTop: 12 }}>
                  <Link href={`/admin/users?user=${selectedPost.user_id}`} className="ad-link" style={drawerLinkButtonStyle}>
                    Inspect user
                    <ExternalLink size={14} />
                  </Link>
                </div>
              </div>

              <div style={sectionStyle}>
                <div style={sectionTitleStyle}>Targets</div>
                {selectedPost.platforms.length > 0 ? (
                  <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                    {selectedPost.platforms.map((item) => (
                      <FieldChip key={`${selectedPost.post_id}-${item}`} label="platform" value={item} />
                    ))}
                  </div>
                ) : (
                  <div style={{ color: "var(--dmuted2)", fontSize: 13 }}>No target platforms recorded.</div>
                )}
              </div>

              <div style={sectionStyle}>
                <div style={sectionTitleStyle}>Delivery</div>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                  <FieldChip label="total" value={fmtNumber(selectedPost.result_count)} />
                  <FieldChip label="published" value={fmtNumber(selectedPost.published_result_count)} />
                  <FieldChip label="failed" value={fmtNumber(selectedPost.failed_result_count)} />
                </div>
                <div className="ad-usage-bar" style={{ width: "100%", height: 8, marginTop: 12 }}>
                  <div
                    className={selectedPost.failed_result_count > 0 ? "ad-uf-r" : "ad-uf-g"}
                    style={{
                      width: `${selectedPost.result_count > 0 ? (selectedPost.published_result_count / selectedPost.result_count) * 100 : 0}%`,
                      height: "100%",
                    }}
                  />
                </div>
              </div>

              <div style={sectionStyle}>
                <div style={sectionTitleStyle}>Timeline</div>
                <div style={{ display: "grid", gap: 8 }}>
                  <KeyValue label="Created" value={formatDateTime(selectedPost.created_at)} />
                  <KeyValue label="Scheduled" value={formatDateTime(selectedPost.scheduled_at)} />
                  <KeyValue label="Published" value={formatDateTime(selectedPost.published_at)} />
                  {selectedPublishTimeline ? (
                    <KeyValue
                      label="Publish time"
                      value={`${selectedPublishTimeline.label} · ${fmtAdminPostTimelineDate(selectedPublishTimeline.at)}`}
                    />
                  ) : null}
                </div>
              </div>
            </div>
          )}
        </aside>
      ) : null}
    </AdminShell>
  );
}

function postKey(post: AdminPostRow, idx: number) {
  return `${post.post_id}-${idx}`;
}

function handlePostKeyDown(event: KeyboardEvent<HTMLElement>, open: () => void) {
  if (event.key !== "Enter" && event.key !== " ") return;
  event.preventDefault();
  open();
}

function formatDateTime(value?: string | null) {
  if (!value) return "—";
  return new Date(value).toLocaleString();
}

function FieldChip({ label, value }: { label: string; value: string }) {
  return (
    <span style={fieldChipStyle}>
      <span style={{ color: "var(--dmuted2)" }}>{label}</span>
      <span style={{ fontFamily: "var(--font-geist-mono), monospace" }}>{value}</span>
    </span>
  );
}

function KeyValue({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", gap: 12, fontSize: 13 }}>
      <span style={{ color: "var(--dmuted)" }}>{label}</span>
      <span style={{ color: "var(--dtext)", fontFamily: "var(--font-geist-mono), monospace", textAlign: "right" }}>{value}</span>
    </div>
  );
}

function DrawerTabs({
  active,
  onChange,
  rightSlot,
}: {
  active: "attributes" | "raw";
  onChange: (next: "attributes" | "raw") => void;
  rightSlot?: ReactNode;
}) {
  return (
    <div style={drawerTabBarStyle}>
      <div style={{ display: "flex", gap: 4 }}>
        <button type="button" onClick={() => onChange("attributes")} style={drawerTabButtonStyle(active === "attributes")}>
          Attributes
        </button>
        <button type="button" onClick={() => onChange("raw")} style={drawerTabButtonStyle(active === "raw")}>
          Raw Data
        </button>
      </div>
      {rightSlot}
    </div>
  );
}

const drawerStyle: CSSProperties = {
  position: "fixed",
  top: 0,
  right: 0,
  bottom: 0,
  width: "min(92vw, 760px)",
  background: "var(--surface-raised, var(--surface))",
  borderLeft: "1px solid var(--dborder)",
  zIndex: 30,
  overflowY: "auto",
  padding: 18,
  display: "flex",
  flexDirection: "column",
  gap: 14,
  boxShadow: "-18px 0 44px color-mix(in srgb, var(--sidebar) 28%, transparent)",
};

const iconButtonStyle: CSSProperties = {
  height: 32,
  width: 32,
  borderRadius: 10,
  border: "1px solid var(--dborder)",
  background: "var(--surface2)",
  color: "var(--dtext)",
  display: "inline-flex",
  alignItems: "center",
  justifyContent: "center",
  cursor: "pointer",
};

const drawerTabBarStyle: CSSProperties = {
  display: "flex",
  alignItems: "center",
  justifyContent: "space-between",
  gap: 8,
  borderBottom: "1px solid var(--dborder)",
  paddingBottom: 8,
};

function drawerTabButtonStyle(active: boolean): CSSProperties {
  return {
    background: "transparent",
    border: "none",
    padding: "6px 4px",
    fontSize: 13,
    fontWeight: 600,
    color: active ? "var(--dtext)" : "var(--dmuted2)",
    borderBottom: active ? "2px solid var(--daccent)" : "2px solid transparent",
    cursor: "pointer",
    marginBottom: -9,
  };
}

const drawerCopyButtonStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 6,
  padding: "4px 10px",
  borderRadius: 8,
  border: "1px solid var(--dborder)",
  background: "var(--surface)",
  color: "var(--dtext)",
  fontSize: 12,
  fontWeight: 600,
  cursor: "pointer",
};

const sectionStyle: CSSProperties = {
  borderRadius: 14,
  border: "1px solid color-mix(in srgb, var(--dborder) 74%, var(--sidebar) 26%)",
  background: "color-mix(in srgb, var(--surface2) 82%, var(--sidebar) 18%)",
  padding: 14,
};

const sectionTitleStyle: CSSProperties = {
  fontSize: 12,
  fontWeight: 700,
  textTransform: "uppercase",
  letterSpacing: "0.12em",
  color: "var(--dmuted)",
  marginBottom: 10,
};

const fieldChipStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 6,
  padding: "6px 10px",
  borderRadius: 999,
  border: "1px solid var(--dborder)",
  background: "var(--surface2)",
  color: "var(--dtext)",
  fontSize: 12,
};

const captionStyle: CSSProperties = {
  fontSize: 14,
  lineHeight: 1.6,
  color: "var(--dtext)",
  whiteSpace: "pre-wrap",
  wordBreak: "break-word",
};

const drawerRawJsonStyle: CSSProperties = {
  margin: 0,
  padding: 14,
  borderRadius: 12,
  border: "1px solid color-mix(in srgb, var(--dborder) 74%, var(--sidebar) 26%)",
  background: "color-mix(in srgb, var(--surface) 66%, var(--sidebar) 34%)",
  color: "var(--dtext)",
  fontSize: 12,
  lineHeight: 1.6,
  overflow: "auto",
  whiteSpace: "pre",
  fontFamily: "var(--font-geist-mono), monospace",
};

const drawerLinkButtonStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 6,
  fontSize: 13,
};
