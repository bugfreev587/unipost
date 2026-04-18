"use client";

import { useAuth } from "@clerk/nextjs";
import { useCallback, useEffect, useMemo, useState } from "react";

import { PlatformIcon } from "@/components/platform-icons";
import {
  getAdminUser,
  getAdminUserPostFailures,
  listAdminUsers,
  type AdminUserDetail,
  type AdminUserListParams,
  type AdminUserPostFailure,
  type AdminUserRow,
} from "@/lib/api";

import { AdminShell, PanelRow, fmtCents, fmtDate, fmtNumber, fmtRelative } from "../_components/admin-ui";

export default function AdminUsersPage() {
  const { getToken } = useAuth();
  const [users, setUsers] = useState<AdminUserRow[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [plan, setPlan] = useState<NonNullable<AdminUserListParams["plan"]>>("all");
  const [sort, setSort] = useState<NonNullable<AdminUserListParams["sort"]>>("newest");
  const [offset, setOffset] = useState(0);
  const limit = 50;

  const [selectedUserId, setSelectedUserId] = useState<string | null>(null);
  const [detail, setDetail] = useState<AdminUserDetail | null>(null);
  const [postFailures, setPostFailures] = useState<AdminUserPostFailure[]>([]);
  const [detailLoading, setDetailLoading] = useState(false);

  const loadUsers = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const usersRes = await listAdminUsers(token, { search, plan, sort, limit, offset });
      setUsers(usersRes.data);
      setTotal(usersRes.meta?.total ?? usersRes.data.length);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, [getToken, limit, offset, plan, search, sort]);

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
  }, [plan, sort]);

  async function openUser(id: string) {
    setSelectedUserId(id);
    setDetail(null);
    setPostFailures([]);
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
  }

  const selectedRangeLabel = useMemo(() => {
    if (users.length === 0) return "0";
    return `${offset + 1}–${offset + users.length}`;
  }, [offset, users.length]);

  return (
    <AdminShell title="Users" loading={loading} onRefresh={loadUsers}>
      {error && (
        <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
          {error}
        </div>
      )}

      <div className="ad-section-header">
        <div className="ad-section-title">Users</div>
        <div className="ad-section-meta">Cross-tenant customer listing</div>
      </div>

      <div className="ad-filter-bar">
        <input
          className="ad-search"
          placeholder="Search by email or ID..."
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
        />
        <select value={plan} onChange={(e) => setPlan(e.target.value as typeof plan)}>
          <option value="all">All Plans</option>
          <option value="free">Free</option>
          <option value="paid">Paid</option>
        </select>
        <select value={sort} onChange={(e) => setSort(e.target.value as typeof sort)}>
          <option value="newest">Sort: Newest</option>
          <option value="mrr">Sort: MRR ↓</option>
          <option value="usage">Sort: Usage ↓</option>
          <option value="last_active">Sort: Last Active</option>
        </select>
      </div>

      <div className="ad-tbl-wrap" style={{ position: "relative" }}>
        <table>
          <thead>
            <tr>
              <th>User</th>
              <th>Plan</th>
              <th>MRR</th>
              <th>Workspaces</th>
              <th>API Keys</th>
              <th>Platforms</th>
              <th>Posts Used</th>
              <th>Last Active</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {loading && users.length === 0 ? (
              <tr><td colSpan={9} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>Loading…</td></tr>
            ) : users.length === 0 ? (
              <tr><td colSpan={9} style={{ padding: 24, color: "var(--dmuted)", textAlign: "center" }}>No users found</td></tr>
            ) : (
              users.map((u) => {
                const usagePct = u.post_limit > 0 ? Math.min(100, (u.posts_used / u.post_limit) * 100) : 0;
                const usageClass = usagePct >= 90 ? "ad-uf-r" : usagePct >= 70 ? "ad-uf-a" : "ad-uf-g";
                return (
                  <tr key={u.id} onClick={() => openUser(u.id)}>
                    <td>
                      <div style={{ fontWeight: 500 }}>{u.email}</div>
                      <div className="ad-mono">{u.id.slice(0, 16)}</div>
                    </td>
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
                      <div style={{ fontSize: 11.5 }}>{fmtNumber(u.posts_used)} / {fmtNumber(u.post_limit || 100)}</div>
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
              <div className="ad-panel-title">{detail?.email || "Loading…"}</div>
              <button className="ad-close-btn" onClick={closeDetail}>✕</button>
            </div>

            {detailLoading && !detail ? (
              <div style={{ color: "var(--dmuted)", fontSize: 13 }}>Loading…</div>
            ) : detail ? (
              <>
                <div className="ad-panel-section">
                  <div className="ad-panel-section-title">Account</div>
                  <PanelRow k="User ID" v={<span className="ad-mono">{detail.id}</span>} />
                  <PanelRow k="Signed up" v={fmtDate(detail.created_at)} />
                  {detail.name ? <PanelRow k="Name" v={detail.name} /> : null}
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
                    v={<span style={{ color: detail.post_limit > 0 && detail.posts_used_this_month / detail.post_limit > 0.7 ? "var(--warning)" : undefined }}>{fmtNumber(detail.posts_used_this_month)} / {fmtNumber(detail.post_limit || 100)}</span>}
                  />
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
                          {workspace.id.slice(0, 16)} · {fmtNumber(workspace.posts_used)} / {fmtNumber(workspace.post_limit)} posts · {workspace.platform_count} platforms
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

      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: 12, padding: "0 2px" }}>
        <span style={{ fontSize: 12, color: "var(--dmuted)" }}>
          Showing {selectedRangeLabel} of {fmtNumber(total)} users
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
