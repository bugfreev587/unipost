"use client";

import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { useCallback, useEffect, useMemo, useState } from "react";
import { RotateCcwIcon, XIcon } from "lucide-react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { PlatformIcon } from "@/components/platform-icons";
import {
  getAdminUserSignups,
  getAdminUser,
  getAdminUserPostFailures,
  getAdminUserScheduledPosts,
  listAdminUsers,
  resetAdminUserPostQuota,
  resetAdminUserScheduledQuota,
  type AdminUserDetail,
  type AdminUserQuotaResetResult,
  type AdminUserSignupTrend,
  type AdminUserListParams,
  type AdminUserPostFailure,
  type AdminUserRow,
  type AdminUserScheduledPost,
} from "@/lib/api";
import { adminUserIdentifierLabel } from "@/lib/admin-privacy";
import { formatPostUsage, usagePercentage } from "@/lib/billing-format";
import { countryDisplay, countryNameFromCode } from "@/lib/countries";

import { CountryDonut } from "../_components/country-donut";
import { AdminShell, PanelRow, bucketByLocalDay, fmtCents, fmtDate, fmtNumber, fmtRelative } from "../_components/admin-ui";
import { SearchHistoryInput } from "../_components/search-history-input";

function CountryBadge({ code }: { code?: string | null }) {
  const name = countryNameFromCode(code);
  if (!name) return <span style={{ color: "var(--dmuted2)", fontSize: 11 }}>—</span>;
  return <span className="ad-badge ad-b-gray" title={countryDisplay(code)}>{name}</span>;
}

function adminUserFailedPostsHref(userId: string) {
  return `/admin/errors?user_id=${encodeURIComponent(userId)}&period=this_month`;
}

export default function AdminUsersPage() {
  const { getToken } = useAuth();
  const [users, setUsers] = useState<AdminUserRow[]>([]);
  const [signups, setSignups] = useState<AdminUserSignupTrend | null>(null);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [plan, setPlan] = useState<NonNullable<AdminUserListParams["plan"]>>("all");
  const [activity, setActivity] = useState<NonNullable<AdminUserListParams["activity"]>>("all");
  const [sort, setSort] = useState<NonNullable<AdminUserListParams["sort"]>>("newest");
  const [hideUsers, setHideUsers] = useState(false);
  const [offset, setOffset] = useState(0);
  const limit = 50;

  const [selectedUserId, setSelectedUserId] = useState<string | null>(null);
  const [detail, setDetail] = useState<AdminUserDetail | null>(null);
  const [postFailures, setPostFailures] = useState<AdminUserPostFailure[]>([]);
  const [detailLoading, setDetailLoading] = useState(false);
  const [scheduledDrawerUser, setScheduledDrawerUser] = useState<AdminUserRow | null>(null);
  const [scheduledPosts, setScheduledPosts] = useState<AdminUserScheduledPost[]>([]);
  const [scheduledDrawerLoading, setScheduledDrawerLoading] = useState(false);
  const [scheduledDrawerError, setScheduledDrawerError] = useState<string | null>(null);
  const [quotaResetPending, setQuotaResetPending] = useState<AdminUserQuotaResetResult["quota_kind"] | null>(null);
  const [quotaResetMessage, setQuotaResetMessage] = useState<string | null>(null);

  const loadUsers = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const [usersRes, signupsRes] = await Promise.all([
        listAdminUsers(token, { search, plan, activity, sort, limit, offset }),
        getAdminUserSignups(token, 30),
      ]);
      setUsers(usersRes.data);
      setTotal(usersRes.meta?.total ?? usersRes.data.length);
      setSignups(signupsRes.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, [activity, getToken, limit, offset, plan, search, sort]);

  useEffect(() => {
    loadUsers();
  }, [loadUsers]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const requestedUserId = new URLSearchParams(window.location.search).get("user");
    if (!requestedUserId) return;
    void openUser(requestedUserId);
  }, []);

  useEffect(() => {
    const t = setTimeout(() => {
      setSearch(searchInput);
      setOffset(0);
    }, 300);
    return () => clearTimeout(t);
  }, [searchInput]);

  useEffect(() => {
    setOffset(0);
  }, [activity, plan, sort]);

  async function openUser(id: string) {
    setSelectedUserId(id);
    setDetail(null);
    setPostFailures([]);
    setQuotaResetMessage(null);
    setDetailLoading(true);
    try {
      const token = await getToken();
      if (!token) return;
      const [userRes, failuresRes] = await Promise.all([
        getAdminUser(token, id),
        getAdminUserPostFailures(token, id, { days: 30, limit: 25 }),
      ]);
      setDetail(userRes.data);
      setPostFailures(failuresRes.data);
    } catch (e) {
      console.error(e);
    } finally {
      setDetailLoading(false);
    }
  }

  function closeDetail() {
    setSelectedUserId(null);
    setDetail(null);
    setPostFailures([]);
    setQuotaResetPending(null);
    setQuotaResetMessage(null);
  }

  async function handleQuotaReset(kind: AdminUserQuotaResetResult["quota_kind"]) {
    if (!selectedUserId) return;
    setQuotaResetPending(kind);
    setQuotaResetMessage(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const reset =
        kind === "scheduled"
          ? await resetAdminUserScheduledQuota(token, selectedUserId)
          : await resetAdminUserPostQuota(token, selectedUserId);
      const label = kind === "scheduled" ? "Schedule quota" : "Post quota";
      const message = `${label} reset for ${reset.data.period}. Previous usage: ${fmtNumber(reset.data.previous_usage)} across ${fmtNumber(reset.data.affected_workspaces)} workspaces.`;
      await Promise.all([openUser(selectedUserId), loadUsers()]);
      setQuotaResetMessage(message);
    } catch (e) {
      setQuotaResetMessage(e instanceof Error ? e.message : "Failed to reset quota");
    } finally {
      setQuotaResetPending(null);
    }
  }

  async function openScheduledPosts(u: AdminUserRow) {
    setScheduledDrawerUser(u);
    setScheduledPosts([]);
    setScheduledDrawerError(null);
    setScheduledDrawerLoading(true);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const res = await getAdminUserScheduledPosts(token, u.id);
      setScheduledPosts(res.data);
    } catch (e) {
      setScheduledDrawerError(e instanceof Error ? e.message : "Failed to load scheduled posts");
    } finally {
      setScheduledDrawerLoading(false);
    }
  }

  function closeScheduledPosts() {
    setScheduledDrawerUser(null);
    setScheduledPosts([]);
    setScheduledDrawerError(null);
    setScheduledDrawerLoading(false);
  }

  const selectedRangeLabel = useMemo(() => {
    if (users.length === 0) return "0";
    return `${offset + 1}–${offset + users.length}`;
  }, [offset, users.length]);
  const totalUserLabel = activity === "active" ? "active users" : "users";

  // Bucket the raw signup timestamps into local-day buckets so a 11pm
  // PT signup shows up under the same day a Pacific viewer would expect,
  // not the next UTC calendar day.
  const signupRows = useMemo(() => {
    if (!signups) return [] as { date: string; count: number }[];
    return bucketByLocalDay(
      signups.events,
      signups.range_days,
      (date) => ({ date, count: 0 }),
      (b) => { b.count += 1; },
      (iso) => iso,
    );
  }, [signups]);
  const signupTotal = useMemo(
    () => signupRows.reduce((sum, r) => sum + r.count, 0),
    [signupRows],
  );

  return (
    <AdminShell title="Users" loading={loading} onRefresh={loadUsers}>
      <style>{usersCss}</style>
      {error && (
        <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
          {error}
        </div>
      )}

      <div className="ad-section-header">
        <div className="ad-section-title">Users</div>
        <div className="ad-section-meta">Cross-tenant customer listing</div>
      </div>

      <div className="au-signup-grid">
        <div className="au-chart-panel">
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", marginBottom: 10 }}>
            <div style={{ fontSize: 14, fontWeight: 600 }}>Signups per day</div>
            <div style={{ fontSize: 12, color: "var(--dmuted)" }}>
              Last {signups?.range_days ?? 30} days
              {signups ? ` · ${fmtNumber(signupTotal)} total` : ""}
            </div>
          </div>
          <div className="au-chart-body">
            {signups && signupRows.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={signupRows} margin={{ top: 8, right: 16, left: 0, bottom: 4 }}>
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
                    formatter={(value) => [fmtNumber(Number(value ?? 0)), "Signups"]}
                  />
                  <Bar dataKey="count" name="Signups" fill="var(--daccent)" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            ) : (
              <div style={{ height: "100%", display: "flex", alignItems: "center", justifyContent: "center", color: "var(--dmuted)", fontSize: 13 }}>
                {loading ? "Loading chart…" : "No signup data yet"}
              </div>
            )}
          </div>
        </div>
        <CountryDonut
          title="User countries"
          subtitle={`Last ${signups?.range_days ?? 30} days`}
          rows={signups?.countries ?? []}
          loading={loading}
          valueLabel="users"
        />
      </div>

      <div className="ad-filter-bar">
        <SearchHistoryInput
          fieldKey="admin.users.search"
          className="ad-search"
          placeholder="Search by email or ID..."
          value={searchInput}
          onChange={setSearchInput}
        />
        <select value={plan} onChange={(e) => setPlan(e.target.value as typeof plan)}>
          <option value="all">All Plans</option>
          <option value="free">Free</option>
          <option value="paid">Paid</option>
        </select>
        <select value={activity} onChange={(e) => setActivity(e.target.value as typeof activity)}>
          <option value="all">All Users</option>
          <option value="active">Active Users</option>
        </select>
        <select value={sort} onChange={(e) => setSort(e.target.value as typeof sort)}>
          <option value="newest">Sort: Newest</option>
          <option value="mrr">Sort: MRR ↓</option>
          <option value="usage">Sort: Usage ↓</option>
          <option value="last_active">Sort: Last Active</option>
        </select>
        <select value={hideUsers ? "hide" : "show"} onChange={(e) => setHideUsers(e.target.value === "hide")}>
          <option value="show">Privacy: Show Users</option>
          <option value="hide">Privacy: Hide Users</option>
        </select>
      </div>

      <div className={`ad-tbl-wrap ad-tbl-static au-users-table-wrap ${selectedUserId ? "au-users-table-wrap-detail-open" : ""}`}>
        <table>
          <thead>
            <tr>
              <th>User</th>
              <th>Sign Up</th>
              <th>Country</th>
              <th>Plan</th>
              <th>MRR</th>
              <th>Workspaces</th>
              <th>API Keys</th>
              <th>Platforms</th>
              <th>Scheduled</th>
              <th>Failed</th>
              <th>Posts Used</th>
              <th>Last Active</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {loading && users.length === 0 ? (
              <tr><td colSpan={13} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>Loading…</td></tr>
            ) : users.length === 0 ? (
              <tr><td colSpan={13} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>No users found</td></tr>
            ) : (
              users.map((u) => {
                const usagePct = usagePercentage(u.posts_used, u.post_limit);
                const usageClass = usagePct >= 90 ? "ad-uf-r" : usagePct >= 70 ? "ad-uf-a" : "ad-uf-g";
                const scheduledCount = u.scheduled_posts ?? 0;
                return (
                  <tr key={u.id}>
                    <td>
                      <div style={{ fontWeight: 500 }}>{adminUserIdentifierLabel(u.email, hideUsers)}</div>
                      <div className="ad-mono">{adminUserIdentifierLabel(u.id.slice(0, 16), hideUsers)}</div>
                    </td>
                    <td style={{ color: "var(--dmuted)", fontSize: 11.5 }}>
                      <div>{fmtDate(u.created_at)}</div>
                      <div style={{ fontSize: 11, color: "var(--dmuted2)" }}>{fmtRelative(u.created_at)}</div>
                    </td>
                    <td><CountryBadge code={u.signup_country_code} /></td>
                    <td>
                      {u.is_paid ? (
                        <span className="ad-badge ad-b-blue">{fmtCents(u.mrr_cents)}/mo</span>
                      ) : (
                        <span className="ad-badge ad-b-gray">Free</span>
                      )}
                    </td>
                    <td>
                      {u.mrr_cents > 0 ? (
                        <span className="ad-mrr-chip">{fmtCents(u.mrr_cents)}</span>
                      ) : (
                        <span style={{ color: "var(--dmuted2)", fontSize: 11 }}>—</span>
                      )}
                    </td>
                    <td>{u.workspace_count}</td>
                    <td>{u.api_key_count}</td>
                    <td>
                      {u.platforms.length > 0 ? (
                        <div className="ad-plat-icons">
                          {u.platforms.map((p) => (
                            <div key={p} className="ad-plat-dot" title={p}><PlatformIcon platform={p} size={14} /></div>
                          ))}
                        </div>
                      ) : (
                        <span style={{ color: "var(--dmuted2)", fontSize: 11 }}>—</span>
                      )}
                    </td>
                    <td>
                      {scheduledCount > 0 ? (
                        <button
                          type="button"
                          className="ad-link au-scheduled-link"
                          aria-label={`View ${fmtNumber(u.scheduled_posts ?? 0)} scheduled posts for ${adminUserIdentifierLabel(u.email, hideUsers)}`}
                          onClick={(e) => {
                            e.stopPropagation();
                            void openScheduledPosts(u);
                          }}
                        >
                          {fmtNumber(u.scheduled_posts ?? 0)}
                        </button>
                      ) : (
                        <span className="au-scheduled-zero">{fmtNumber(u.scheduled_posts ?? 0)}</span>
                      )}
                    </td>
                    <td>
                      {u.failed_posts_this_month > 0 ? (
                        <Link href={adminUserFailedPostsHref(u.id)} className="ad-link au-failed-link">
                          {fmtNumber(u.failed_posts_this_month)}
                        </Link>
                      ) : (
                        <span className="au-failed-zero">0</span>
                      )}
                    </td>
                    <td>
                      <div style={{ fontSize: 11.5 }}>{formatPostUsage(u.posts_used, u.post_limit || 100)}</div>
                      <div className="ad-usage-bar">
                        <div className={`ad-usage-fill ${usageClass}`} style={{ width: `${usagePct}%` }} />
                      </div>
                    </td>
                    <td style={{ color: "var(--dmuted)", fontSize: 11.5 }}>{fmtRelative(u.last_post_at)}</td>
                    <td>
                      <button
                        className="ad-btn ad-btn-ghost"
                        style={{ padding: "3px 8px", fontSize: 11 }}
                        onClick={(e) => {
                          e.stopPropagation();
                          openUser(u.id);
                        }}
                      >
                        View
                      </button>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>

        {selectedUserId && (
          <div className="ad-detail-panel">
            <div className="ad-panel-header">
              <div className="ad-panel-title">{detail?.email ? adminUserIdentifierLabel(detail.email, hideUsers) : "Loading…"}</div>
              <button className="ad-close-btn" onClick={closeDetail}>✕</button>
            </div>

            {detailLoading && !detail ? (
              <div style={{ color: "var(--dmuted)", fontSize: 13 }}>Loading…</div>
            ) : detail ? (
              <>
                <div className="ad-panel-section">
                  <div className="ad-panel-section-title">Account</div>
                  <PanelRow k="User ID" v={<span className="ad-mono">{adminUserIdentifierLabel(detail.id, hideUsers)}</span>} />
                  <PanelRow k="Signed up" v={fmtDate(detail.created_at)} />
                  <PanelRow k="Signup country" v={countryDisplay(detail.signup_country_code)} />
                  {detail.name ? <PanelRow k="Name" v={adminUserIdentifierLabel(detail.name, hideUsers)} /> : null}
                  <PanelRow k="Last post" v={fmtRelative(detail.last_post_at)} />
                </div>

                <div className="ad-panel-section">
                  <div className="ad-panel-section-title">Billing</div>
                  <PanelRow
                    k="Total MRR"
                    v={detail.mrr_cents > 0 ? <span className="ad-mrr-chip">{fmtCents(detail.mrr_cents)}/mo</span> : <span className="ad-badge ad-b-gray">Free</span>}
                  />
                  <PanelRow
                    k="Posts used (this month)"
                    v={<span style={{ color: detail.post_limit > 0 && detail.posts_used_this_month / detail.post_limit > 0.7 ? "var(--warning)" : undefined }}>{formatPostUsage(detail.posts_used_this_month, detail.post_limit || 100)}</span>}
                  />
                </div>

                <div className="ad-panel-section">
                  <div className="ad-panel-section-title">Posts quota reset</div>
                  <div className="au-quota-reset-actions">
                    <button
                      type="button"
                      className="ad-btn ad-btn-ghost au-quota-reset-btn"
                      disabled={quotaResetPending !== null}
                      onClick={() => void handleQuotaReset("scheduled")}
                    >
                      <RotateCcwIcon size={13} aria-hidden="true" />
                      {quotaResetPending === "scheduled" ? "Resetting schedule..." : "Reset schedule quota"}
                    </button>
                    <button
                      type="button"
                      className="ad-btn ad-btn-ghost au-quota-reset-btn"
                      disabled={quotaResetPending !== null}
                      onClick={() => void handleQuotaReset("post")}
                    >
                      <RotateCcwIcon size={13} aria-hidden="true" />
                      {quotaResetPending === "post" ? "Resetting posts..." : "Reset post quota"}
                    </button>
                  </div>
                  {quotaResetMessage ? (
                    <div className="au-quota-reset-message">{quotaResetMessage}</div>
                  ) : null}
                </div>

                <div className="ad-panel-section">
                  <div className="ad-panel-section-title">Usage</div>
                  <PanelRow k="Workspaces" v={String(detail.workspace_count)} />
                  <PanelRow k="API Keys" v={String(detail.api_key_count)} />
                  <PanelRow
                    k="Connected platforms"
                    v={detail.platforms.length > 0 ? <span style={{ display: "inline-flex", gap: 4 }}>{detail.platforms.map((p) => <PlatformIcon key={p} platform={p} size={14} />)}</span> : "—"}
                  />
                  <PanelRow k="Total posts (all time)" v={fmtNumber(detail.total_posts)} />
                  <PanelRow k="Failed posts (30d)" v={<span style={{ color: detail.failed_posts_30d > 0 ? "var(--warning)" : undefined }}>{fmtNumber(detail.failed_posts_30d)}</span>} />
                </div>

                {detail.workspaces.length > 0 ? (
                  <div className="ad-panel-section">
                    <div className="ad-panel-section-title">Workspaces ({detail.workspaces.length})</div>
                    {detail.workspaces.map((workspace) => (
                      <div key={workspace.id} style={{ padding: "8px 0", borderBottom: "1px solid #1f1f1f" }}>
                        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline" }}>
                          <span style={{ fontSize: 12, fontWeight: 500 }}>{workspace.name}</span>
                          <span className={`ad-badge ${workspace.price_cents > 0 ? "ad-b-blue" : "ad-b-gray"}`}>
                            {workspace.price_cents > 0 ? `$${(workspace.price_cents / 100).toFixed(0)}/mo` : "Free"}
                          </span>
                        </div>
                        <div className="ad-mono" style={{ marginTop: 2 }}>
                          {workspace.id.slice(0, 16)} · {formatPostUsage(workspace.posts_used, workspace.post_limit)} · {workspace.platform_count} platforms
                        </div>
                      </div>
                    ))}
                  </div>
                ) : null}

                <div className="ad-panel-section">
                  <div className="ad-panel-section-title">Failed posts (30d)</div>
                  {postFailures.length === 0 ? (
                    <div style={{ fontSize: 12, color: "var(--dmuted)" }}>
                      No failure details found for this user in the last 30 days.
                    </div>
                  ) : (
                    <div style={{ display: "grid", gap: 10 }}>
                      {postFailures.map((failure, idx) => {
                        const message = failure.error_message || failure.error_summary || "No error message recorded.";
                        return (
                          <div
                            key={`${failure.post_id}-${failure.platform || "parent"}-${idx}`}
                            style={{ border: "1px solid var(--dborder)", borderRadius: 8, padding: 10, background: "var(--surface)" }}
                          >
                            <div style={{ display: "flex", justifyContent: "space-between", gap: 8, marginBottom: 6 }}>
                              <div style={{ display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
                                <span className="ad-badge ad-b-gray">{failure.platform || failure.post_status}</span>
                                {failure.account_name ? <span style={{ fontSize: 11, color: "var(--dmuted)" }}>@{failure.account_name}</span> : null}
                                <span style={{ fontSize: 11, color: "var(--dmuted2)" }}>{failure.workspace_name}</span>
                              </div>
                              <span style={{ fontSize: 11, color: "var(--dmuted)" }}>{fmtRelative(failure.created_at)}</span>
                            </div>
                            {failure.caption ? (
                              <div style={{ fontSize: 12, color: "var(--dtext)", marginBottom: 6 }}>
                                {failure.caption}
                              </div>
                            ) : null}
                            <div
                              style={{
                                fontSize: 11.5,
                                color: "var(--danger)",
                                background: "var(--danger-soft)",
                                border: "1px solid color-mix(in srgb, var(--danger) 18%, transparent)",
                                borderRadius: 6,
                                padding: "8px 9px",
                                whiteSpace: "pre-wrap",
                                wordBreak: "break-word",
                              }}
                            >
                              {message}
                            </div>
                            <div className="ad-mono" style={{ marginTop: 6 }}>
                              post {failure.post_id.slice(0, 16)} · source {failure.source}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              </>
            ) : null}
          </div>
        )}
      </div>

      {scheduledDrawerUser ? (
        <div className="au-scheduled-layer">
          <button
            type="button"
            className="au-scheduled-backdrop"
            aria-label="Close scheduled posts drawer"
            onClick={closeScheduledPosts}
          />
          <aside
            className="au-scheduled-drawer"
            role="dialog"
            aria-modal="true"
            aria-labelledby="au-scheduled-title"
          >
            <div className="au-scheduled-drawer-header">
              <div>
                <div id="au-scheduled-title" className="au-scheduled-drawer-title">Scheduled posts</div>
                <div className="au-scheduled-drawer-subtitle">{adminUserIdentifierLabel(scheduledDrawerUser.email, hideUsers)}</div>
              </div>
              <button
                type="button"
                className="ad-close-btn au-scheduled-close"
                aria-label="Close scheduled posts drawer"
                onClick={closeScheduledPosts}
              >
                <XIcon size={14} aria-hidden="true" />
              </button>
            </div>

            <div className="au-scheduled-drawer-body">
              {scheduledDrawerLoading ? (
                <div className="au-scheduled-skeleton-list" aria-label="Loading scheduled posts">
                  {[0, 1, 2].map((item) => (
                    <div key={item} className="au-scheduled-skeleton">
                      <div />
                      <span />
                      <span />
                    </div>
                  ))}
                </div>
              ) : scheduledDrawerError ? (
                <div className="au-scheduled-state au-scheduled-error">
                  {scheduledDrawerError}
                </div>
              ) : scheduledPosts.length === 0 ? (
                <div className="au-scheduled-state">
                  No scheduled posts found for this user.
                </div>
              ) : (
                <div className="au-scheduled-list">
                  {scheduledPosts.map((post) => (
                    <div key={post.post_id} className="au-scheduled-post">
                      <div className="au-scheduled-post-head">
                        <div className="au-scheduled-post-title">{post.title}</div>
                        <div className="au-scheduled-platforms" aria-label="Platforms">
                          {post.platforms.length > 0 ? (
                            post.platforms.map((platform) => (
                              <PlatformIcon key={platform} platform={platform} size={14} />
                            ))
                          ) : (
                            <span className="au-scheduled-no-platform">—</span>
                          )}
                        </div>
                      </div>
                      <div className="au-scheduled-post-meta">
                        <div>
                          <span>Created</span>
                          <strong>{formatDateTime(post.created_at)}</strong>
                        </div>
                        <div>
                          <span>Publishes</span>
                          <strong>{formatDateTime(post.scheduled_at)}</strong>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </aside>
        </div>
      ) : null}

      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: 12, padding: "0 2px" }}>
        <span style={{ fontSize: 12, color: "var(--dmuted)" }}>
          Showing {selectedRangeLabel} of {fmtNumber(total)} {totalUserLabel}
        </span>
        <div style={{ display: "flex", gap: 6 }}>
          <button
            className="ad-btn ad-btn-ghost"
            style={{ padding: "4px 10px" }}
            disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - limit))}
          >
            ← Prev
          </button>
          <button
            className="ad-btn ad-btn-ghost"
            style={{ padding: "4px 10px" }}
            disabled={offset + limit >= total}
            onClick={() => setOffset(offset + limit)}
          >
            Next →
          </button>
        </div>
      </div>
    </AdminShell>
  );
}

function formatDateTime(value?: string | null) {
  if (!value) return "—";
  return new Date(value).toLocaleString();
}

const usersCss = `
.au-signup-grid {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(300px, 0.85fr);
  gap: 10px;
  margin-bottom: 16px;
}
.au-chart-panel {
  background: var(--surface-raised);
  border: 1px solid var(--dborder);
  border-radius: 8px;
  padding: 14px 16px 16px;
  min-height: 280px;
}
.au-chart-body {
  height: 230px;
}
.au-users-table-wrap {
  position: relative;
}
.au-users-table-wrap-detail-open {
  min-height: clamp(420px, calc(100dvh - 260px), 640px);
}
.au-scheduled-link {
  appearance: none;
  background: transparent;
  border: 0;
  cursor: pointer;
  font: inherit;
  padding: 0;
  color: var(--daccent);
  font-weight: 650;
}
.au-scheduled-link:active {
  transform: translateY(1px);
}
.au-scheduled-zero {
  color: var(--dmuted2);
}
.au-failed-link {
  color: var(--danger);
  font-weight: 650;
}
.au-failed-zero {
  color: var(--dmuted2);
}
.au-quota-reset-actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}
.au-quota-reset-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 30px;
  padding: 5px 8px;
  font-size: 11px;
  white-space: nowrap;
}
.au-quota-reset-btn:active:not(:disabled) {
  transform: translateY(1px);
}
.au-quota-reset-message {
  margin-top: 8px;
  padding: 8px 9px;
  border: 1px solid color-mix(in srgb, var(--success) 24%, transparent);
  border-radius: 8px;
  background: color-mix(in srgb, var(--success) 11%, transparent);
  color: var(--dtext);
  font-size: 11.5px;
  line-height: 1.45;
}
.au-scheduled-layer {
  position: fixed;
  inset: 0;
  z-index: 40;
  pointer-events: none;
}
.au-scheduled-backdrop {
  position: absolute;
  inset: 0;
  pointer-events: auto;
  border: 0;
  padding: 0;
  background: rgba(8, 8, 8, 0.38);
  animation: au-scheduled-fade-in 180ms cubic-bezier(0.16, 1, 0.3, 1);
}
.au-scheduled-drawer {
  position: absolute;
  top: 0;
  right: 0;
  width: min(460px, calc(100vw - 24px));
  height: 100dvh;
  pointer-events: auto;
  overflow-y: auto;
  background: var(--surface-raised);
  border-left: 1px solid var(--dborder);
  box-shadow: -24px 0 52px -28px rgba(0, 0, 0, 0.7);
  padding: 18px;
  animation: au-scheduled-drawer-in 260ms cubic-bezier(0.16, 1, 0.3, 1);
}
.au-scheduled-drawer-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--dborder);
}
.au-scheduled-drawer-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--dtext);
  line-height: 1.25;
}
.au-scheduled-drawer-subtitle {
  margin-top: 3px;
  font-size: 12px;
  color: var(--dmuted);
  word-break: break-word;
}
.au-scheduled-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  padding: 0;
}
.au-scheduled-drawer-body {
  padding-top: 14px;
}
.au-scheduled-list,
.au-scheduled-skeleton-list {
  display: grid;
  gap: 10px;
}
.au-scheduled-post {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--dborder);
  border-radius: 8px;
  background: var(--surface);
}
.au-scheduled-post-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}
.au-scheduled-post-title {
  min-width: 0;
  color: var(--dtext);
  font-size: 13px;
  font-weight: 650;
  line-height: 1.4;
  word-break: break-word;
}
.au-scheduled-platforms {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 5px;
  min-width: 56px;
  color: var(--dtext);
}
.au-scheduled-no-platform {
  color: var(--dmuted2);
  font-size: 12px;
}
.au-scheduled-post-meta {
  display: grid;
  gap: 6px;
}
.au-scheduled-post-meta > div {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  font-size: 11.5px;
}
.au-scheduled-post-meta span {
  color: var(--dmuted);
}
.au-scheduled-post-meta strong {
  color: var(--dtext);
  font-family: var(--font-geist-mono), monospace;
  font-weight: 500;
  text-align: right;
}
.au-scheduled-state {
  border: 1px solid var(--dborder);
  border-radius: 8px;
  padding: 14px;
  background: var(--surface);
  color: var(--dmuted);
  font-size: 13px;
}
.au-scheduled-error {
  color: var(--danger);
  background: var(--danger-soft);
  border-color: color-mix(in srgb, var(--danger) 20%, transparent);
}
.au-scheduled-skeleton {
  display: grid;
  gap: 8px;
  padding: 12px;
  border: 1px solid var(--dborder);
  border-radius: 8px;
  background: var(--surface);
}
.au-scheduled-skeleton div,
.au-scheduled-skeleton span {
  display: block;
  height: 10px;
  border-radius: 999px;
  background: linear-gradient(90deg, var(--surface2), var(--dborder), var(--surface2));
  background-size: 200% 100%;
  animation: au-scheduled-shimmer 1.1s ease-in-out infinite;
}
.au-scheduled-skeleton div {
  width: 76%;
}
.au-scheduled-skeleton span {
  width: 52%;
}
.au-scheduled-skeleton span:last-child {
  width: 68%;
}
@keyframes au-scheduled-drawer-in {
  from { transform: translateX(28px); opacity: 0.86; }
  to { transform: translateX(0); opacity: 1; }
}
@keyframes au-scheduled-fade-in {
  from { opacity: 0; }
  to { opacity: 1; }
}
@keyframes au-scheduled-shimmer {
  from { background-position: 200% 0; }
  to { background-position: -200% 0; }
}
@media (max-width: 1120px) {
  .au-signup-grid {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 860px) {
  .au-users-table-wrap-detail-open {
    min-height: 0;
  }
}
`;
