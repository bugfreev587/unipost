"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useAuth, useUser } from "@clerk/nextjs";
import {
  getAdminStats,
  getAdminUser,
  getMe,
  listAdminUsers,
  type AdminStats,
  type AdminUserDetail,
  type AdminUserListParams,
  type AdminUserRow,
} from "@/lib/api";

// ── platform → emoji (mockup uses these as identicons) ───────────────
const PLATFORM_ICON: Record<string, string> = {
  bluesky: "🦋",
  linkedin: "💼",
  instagram: "📸",
  threads: "🧵",
  facebook: "👤",
  tiktok: "🎵",
  youtube: "▶️",
  twitter: "🐦",
};
const platformEmoji = (p: string) => PLATFORM_ICON[p.toLowerCase()] || "•";

// ── format helpers ───────────────────────────────────────────────────
const fmtCents = (cents: number) => {
  const dollars = cents / 100;
  return dollars >= 1000
    ? `$${(dollars / 1000).toFixed(1)}k`
    : `$${dollars.toFixed(0)}`;
};
const fmtNumber = (n: number) => n.toLocaleString("en-US");
const fmtRelative = (iso: string | null | undefined) => {
  if (!iso) return "Never";
  const then = new Date(iso).getTime();
  const diff = Date.now() - then;
  if (diff < 60_000) return "just now";
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`;
  if (diff < 30 * 86_400_000) return `${Math.floor(diff / 86_400_000)}d ago`;
  return new Date(iso).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
};
const fmtDate = (iso: string) =>
  new Date(iso).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });

// ─────────────────────────────────────────────────────────────────────

export default function AdminPage() {
  const { getToken } = useAuth();
  const { user, isLoaded: userLoaded } = useUser();

  const [stats, setStats] = useState<AdminStats | null>(null);
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
  const [detailLoading, setDetailLoading] = useState(false);

  // Admin gate is resolved server-side via /v1/me against ADMIN_USERS.
  // null = still checking, true/false = resolved. Backend independently
  // enforces the same allowlist on /v1/admin/* — this client check is
  // just UX so non-admins see a friendly 403 instead of empty cards.
  const userEmail = user?.primaryEmailAddress?.emailAddress?.toLowerCase() || "";
  const [isAdmin, setIsAdmin] = useState<boolean | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token) { if (!cancelled) setIsAdmin(false); return; }
        const res = await getMe(token);
        if (!cancelled) setIsAdmin(!!res.data.is_admin);
      } catch {
        if (!cancelled) setIsAdmin(false);
      }
    })();
    return () => { cancelled = true; };
  }, [getToken]);

  const loadAll = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const [statsRes, usersRes] = await Promise.all([
        getAdminStats(token),
        listAdminUsers(token, { search, plan, sort, limit, offset }),
      ]);
      setStats(statsRes.data);
      setUsers(usersRes.data);
      setTotal(usersRes.meta?.total ?? usersRes.data.length);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, [getToken, search, plan, sort, offset]);

  useEffect(() => {
    if (userLoaded && isAdmin === true) loadAll();
  }, [userLoaded, isAdmin, loadAll]);

  // Debounce search input
  useEffect(() => {
    const t = setTimeout(() => {
      setSearch(searchInput);
      setOffset(0);
    }, 300);
    return () => clearTimeout(t);
  }, [searchInput]);

  // Reset offset when filters change
  useEffect(() => {
    setOffset(0);
  }, [plan, sort]);

  async function openUser(id: string) {
    setSelectedUserId(id);
    setDetail(null);
    setDetailLoading(true);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await getAdminUser(token, id);
      setDetail(res.data);
    } catch (e) {
      console.error(e);
    } finally {
      setDetailLoading(false);
    }
  }

  function closeDetail() {
    setSelectedUserId(null);
    setDetail(null);
  }

  // Conversion %
  const conversionPct = useMemo(() => {
    if (!stats || stats.total_users === 0) return 0;
    return (stats.paid_users / stats.total_users) * 100;
  }, [stats]);

  const failedPct = useMemo(() => {
    if (!stats || stats.posts_this_month === 0) return 0;
    return (stats.posts_failed_this_month / stats.posts_this_month) * 100;
  }, [stats]);

  const signups7dDelta = useMemo(() => {
    if (!stats || stats.prev_signups_7d === 0) return null;
    const change = ((stats.new_signups_7d - stats.prev_signups_7d) / stats.prev_signups_7d) * 100;
    return change;
  }, [stats]);

  // ── Gating: still loading user / admin check, or not admin ────────
  if (!userLoaded || isAdmin === null) {
    return (
      <div style={{ ...shellStyle, alignItems: "center", justifyContent: "center" }}>
        <div style={{ color: "var(--dmuted)" }}>Loading…</div>
      </div>
    );
  }

  if (!isAdmin) {
    return (
      <div style={{ ...shellStyle, alignItems: "center", justifyContent: "center", flexDirection: "column", gap: 8 }}>
        <div style={{ fontSize: 18, fontWeight: 600 }}>403 — Not authorized</div>
        <div style={{ fontSize: 13, color: "var(--dmuted)" }}>This page is restricted to admins.</div>
      </div>
    );
  }

  // ── Page ──────────────────────────────────────────────────────────
  return (
    <div style={shellStyle}>
      <style>{adminCss}</style>

      {/* Sidebar */}
      <aside className="ad-sidebar">
        <div className="ad-sb-logo">
          <div className="ad-sb-mark">
            <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
              <path d="M9 2L4 9h4l-1 5 5-7H8l1-5z" />
            </svg>
          </div>
          <span className="ad-sb-name">UniPost</span>
          <span className="ad-sb-badge">ADMIN</span>
        </div>
        <nav className="ad-nav">
          <div className="ad-nav-label">Overview</div>
          <div className="ad-nav-item ad-nav-active">Dashboard</div>
          <div className="ad-nav-item ad-nav-disabled">Users</div>
          <div className="ad-nav-item ad-nav-disabled">Posts</div>
          <div className="ad-nav-label">Revenue</div>
          <div className="ad-nav-item ad-nav-disabled">Billing</div>
          <div className="ad-nav-item ad-nav-disabled">MRR</div>
          <div className="ad-nav-label">System</div>
          <div className="ad-nav-item ad-nav-disabled">Errors</div>
          <div className="ad-nav-item ad-nav-disabled">Settings</div>
        </nav>
        <div className="ad-sb-footer">
          <div className="ad-sb-user">
            <div className="ad-sb-ava">{userEmail.charAt(0).toUpperCase()}</div>
            <span className="ad-sb-email">{userEmail}</span>
          </div>
        </div>
      </aside>

      {/* Main */}
      <main className="ad-main">
        <div className="ad-topbar">
          <div className="ad-bc">
            <span>Admin</span>
            <span className="ad-bc-sep">/</span>
            <span className="ad-bc-cur">Dashboard</span>
          </div>
          <div className="ad-topbar-right">
            <span style={{ fontSize: 11, color: "var(--dmuted)", fontFamily: "var(--font-geist-mono), monospace" }}>
              {loading ? "Loading…" : "Last updated: just now"}
            </span>
            <button className="ad-btn ad-btn-ghost" onClick={loadAll} disabled={loading}>
              ↻ Refresh
            </button>
          </div>
        </div>

        <div className="ad-content">
          {error && (
            <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
              {error}
            </div>
          )}

          {/* Stat grid — row 1 */}
          <div className="ad-stat-grid">
            <StatCard
              label="Total Users"
              value={stats ? fmtNumber(stats.total_users) : "—"}
              sub={stats && stats.new_users_this_month > 0 ? `↑ ${stats.new_users_this_month} this month` : "—"}
              subColor="up"
            />
            <StatCard
              label="Paid Users"
              value={stats ? fmtNumber(stats.paid_users) : "—"}
              sub={stats ? `${conversionPct.toFixed(1)}% conversion` : "—"}
            />
            <StatCard
              label="MRR"
              value={stats ? fmtCents(stats.mrr_cents) : "—"}
              valueColor="accent"
              sub="—"
            />
            <StatCard
              label="Posts This Month"
              value={stats ? fmtNumber(stats.posts_this_month) : "—"}
              sub={
                stats ? (
                  <>
                    Failed rate: <span style={{ color: failedPct > 5 ? "var(--danger)" : "var(--warning)" }}>{failedPct.toFixed(1)}%</span>
                  </>
                ) : (
                  "—"
                )
              }
            />
          </div>

          {/* Stat grid — row 2 */}
          <div className="ad-stat-grid" style={{ marginBottom: 24 }}>
            <StatCard
              label="Active Workspaces"
              value={stats ? fmtNumber(stats.active_workspaces) : "—"}
              sub={stats && stats.total_users > 0 ? `avg ${(stats.active_workspaces / stats.total_users).toFixed(1)} / user` : "—"}
            />
            <StatCard
              label="Platform Connections"
              value={stats ? fmtNumber(stats.platform_connections) : "—"}
              sub="—"
            />
            <StatCard
              label="New Signups (7d)"
              value={stats ? fmtNumber(stats.new_signups_7d) : "—"}
              sub={
                signups7dDelta != null
                  ? `${signups7dDelta >= 0 ? "↑" : "↓"} ${Math.abs(signups7dDelta).toFixed(0)}% vs prev week`
                  : "—"
              }
              subColor={signups7dDelta != null && signups7dDelta >= 0 ? "up" : "down"}
            />
            <StatCard
              label="Churn (30d)"
              value={stats ? fmtNumber(stats.churn_30d) : "—"}
              sub="last 30 days"
              subColor={stats && stats.churn_30d > 0 ? "down" : undefined}
            />
          </div>

          {/* User table */}
          <div className="ad-section-header">
            <div className="ad-section-title">Users</div>
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
                                <div key={p} className="ad-plat-dot" title={p}>{platformEmoji(p)}</div>
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

            {/* Slide-in detail panel */}
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
                      {detail.name && <PanelRow k="Name" v={detail.name} />}
                      <PanelRow k="Last post" v={fmtRelative(detail.last_post_at)} />
                    </div>

                    <div className="ad-panel-section">
                      <div className="ad-panel-section-title">Billing</div>
                      <PanelRow
                        k="Total MRR"
                        v={
                          detail.mrr_cents > 0 ? (
                            <span className="ad-mrr-chip">{fmtCents(detail.mrr_cents)}/mo</span>
                          ) : (
                            <span className="ad-badge ad-b-gray">Free</span>
                          )
                        }
                      />
                      <PanelRow
                        k="Posts used (this month)"
                        v={
                          <span style={{ color: detail.post_limit > 0 && detail.posts_used_this_month / detail.post_limit > 0.7 ? "var(--warning)" : undefined }}>
                            {fmtNumber(detail.posts_used_this_month)} / {fmtNumber(detail.post_limit || 100)}
                          </span>
                        }
                      />
                    </div>

                    <div className="ad-panel-section">
                      <div className="ad-panel-section-title">Usage</div>
                      <PanelRow k="Workspaces" v={String(detail.workspace_count)} />
                      <PanelRow k="API Keys" v={String(detail.api_key_count)} />
                      <PanelRow
                        k="Connected platforms"
                        v={
                          detail.platforms.length > 0
                            ? detail.platforms.map(platformEmoji).join(" ")
                            : "—"
                        }
                      />
                      <PanelRow k="Total posts (all time)" v={fmtNumber(detail.total_posts)} />
                      <PanelRow
                        k="Failed posts (30d)"
                        v={<span style={{ color: detail.failed_posts_30d > 0 ? "var(--warning)" : undefined }}>{fmtNumber(detail.failed_posts_30d)}</span>}
                      />
                    </div>

                    {detail.workspaces.length > 0 && (
                      <div className="ad-panel-section">
                        <div className="ad-panel-section-title">Workspaces ({detail.workspaces.length})</div>
                        {detail.workspaces.map((p) => (
                          <div key={p.id} style={{ padding: "8px 0", borderBottom: "1px solid #1f1f1f" }}>
                            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline" }}>
                              <span style={{ fontSize: 12, fontWeight: 500 }}>{p.name}</span>
                              <span className={`ad-badge ${p.price_cents > 0 ? "ad-b-blue" : "ad-b-gray"}`}>
                                {p.price_cents > 0 ? `$${(p.price_cents / 100).toFixed(0)}/mo` : "Free"}
                              </span>
                            </div>
                            <div className="ad-mono" style={{ marginTop: 2 }}>
                              {p.id.slice(0, 16)} · {fmtNumber(p.posts_used)} / {fmtNumber(p.post_limit)} posts · {p.platform_count} platforms
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </>
                ) : null}
              </div>
            )}
          </div>

          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: 12, padding: "0 2px" }}>
            <span style={{ fontSize: 12, color: "var(--dmuted)" }}>
              Showing {users.length === 0 ? 0 : offset + 1}–{offset + users.length} of {fmtNumber(total)} users
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
        </div>
      </main>
    </div>
  );
}

// ── small components ─────────────────────────────────────────────────

function StatCard({
  label,
  value,
  valueColor,
  sub,
  subColor,
}: {
  label: string;
  value: string;
  valueColor?: "accent";
  sub?: React.ReactNode;
  subColor?: "up" | "down";
}) {
  return (
    <div className="ad-stat-card">
      <div className="ad-stat-label">{label}</div>
      <div className="ad-stat-value" style={{ color: valueColor === "accent" ? "var(--daccent)" : undefined }}>
        {value}
      </div>
      <div
        className="ad-stat-sub"
        style={{
          color: subColor === "up" ? "var(--success)" : subColor === "down" ? "var(--danger)" : "var(--dmuted)",
        }}
      >
        {sub}
      </div>
    </div>
  );
}

function PanelRow({ k, v }: { k: string; v: React.ReactNode }) {
  return (
    <div className="ad-panel-row">
      <span className="ad-panel-key">{k}</span>
      <span className="ad-panel-val">{v}</span>
    </div>
  );
}

// ── styles (scoped via .ad-* prefix) ─────────────────────────────────

const shellStyle: React.CSSProperties = {
  display: "flex",
  height: "100vh",
  minHeight: 700,
  background: "var(--bg)",
  color: "var(--dtext)",
  fontFamily: "var(--font-dm-sans), var(--font-geist-sans), system-ui, sans-serif",
  fontSize: 13,
  lineHeight: 1.5,
};

const adminCss = `
.ad-sidebar { width: 200px; min-width: 200px; background: var(--sidebar); border-right: 1px solid var(--dborder); display: flex; flex-direction: column; }
.ad-sb-logo { display: flex; align-items: center; gap: 8px; padding: 14px 14px 12px; border-bottom: 1px solid var(--dborder); }
.ad-sb-mark { width: 22px; height: 22px; background: #10b981; border-radius: 5px; display: flex; align-items: center; justify-content: center; }
.ad-sb-mark svg { width: 11px; height: 11px; color: var(--primary-foreground); }
.ad-sb-name { font-size: 13px; font-weight: 700; letter-spacing: -0.3px; }
.ad-sb-badge { font-size: 9px; font-weight: 700; background: var(--danger-soft); color: var(--danger); border: 1px solid color-mix(in srgb, var(--danger) 22%, transparent); border-radius: 3px; padding: 1px 5px; font-family: var(--font-geist-mono), monospace; letter-spacing: 0.05em; }
.ad-nav { padding: 10px 8px; flex: 1; }
.ad-nav-label { font-size: 9.5px; font-weight: 700; letter-spacing: 0.1em; text-transform: uppercase; color: var(--dmuted2); padding: 0 6px; margin: 10px 0 3px; }
.ad-nav-item { display: flex; align-items: center; gap: 7px; padding: 5px 7px; border-radius: 6px; color: var(--dmuted); font-size: 12px; margin-bottom: 1px; border: 1px solid transparent; }
.ad-nav-active { background: var(--accent-dim); color: var(--daccent); border-color: color-mix(in srgb, var(--primary) 18%, transparent); font-weight: 500; }
.ad-nav-disabled { color: var(--dmuted2); cursor: not-allowed; }
.ad-sb-footer { padding: 8px; border-top: 1px solid var(--dborder); }
.ad-sb-user { display: flex; align-items: center; gap: 7px; padding: 5px 7px; border-radius: 6px; }
.ad-sb-ava { width: 22px; height: 22px; border-radius: 50%; background: linear-gradient(135deg, #10b981, #059669); display: flex; align-items: center; justify-content: center; font-size: 9px; font-weight: 700; color: var(--primary-foreground); flex-shrink: 0; }
.ad-sb-email { font-size: 11px; color: var(--dmuted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.ad-main { flex: 1; display: flex; flex-direction: column; overflow: hidden; min-width: 0; }
.ad-topbar { height: 44px; border-bottom: 1px solid var(--dborder); display: flex; align-items: center; padding: 0 20px; gap: 8px; flex-shrink: 0; justify-content: space-between; }
.ad-bc { display: flex; align-items: center; gap: 5px; font-size: 12px; color: var(--dmuted); }
.ad-bc-sep { color: var(--dmuted2); }
.ad-bc-cur { color: var(--dtext); font-weight: 500; }
.ad-topbar-right { display: flex; align-items: center; gap: 8px; }
.ad-content { flex: 1; overflow-y: auto; padding: 20px 24px; }

.ad-btn { display: inline-flex; align-items: center; gap: 5px; padding: 5px 12px; border-radius: 6px; font-size: 12px; font-weight: 500; cursor: pointer; border: 1px solid transparent; font-family: inherit; white-space: nowrap; }
.ad-btn-ghost { background: transparent; color: var(--dmuted); border-color: var(--dborder2); }
.ad-btn-ghost:hover:not(:disabled) { background: var(--surface2); color: var(--dtext); }
.ad-btn:disabled { opacity: 0.4; cursor: not-allowed; }

.ad-stat-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; margin-bottom: 20px; }
.ad-stat-card { background: var(--surface); border: 1px solid var(--dborder); border-radius: 8px; padding: 14px 16px; }
.ad-stat-label { font-size: 10px; color: var(--dmuted); text-transform: uppercase; letter-spacing: 0.07em; font-weight: 600; margin-bottom: 6px; }
.ad-stat-value { font-family: var(--font-geist-mono), monospace; font-size: 22px; font-weight: 700; color: var(--dtext); letter-spacing: -0.5px; }
.ad-stat-sub { font-size: 11px; margin-top: 3px; }

.ad-section-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; }
.ad-section-title { font-size: 14px; font-weight: 600; letter-spacing: -0.2px; }

.ad-filter-bar { display: flex; align-items: center; gap: 8px; margin-bottom: 14px; }
.ad-search { background: var(--surface2); border: 1px solid var(--dborder2); border-radius: 6px; color: var(--dtext); font-size: 12px; padding: 6px 10px; font-family: inherit; outline: none; width: 220px; }
.ad-search:focus { border-color: color-mix(in srgb, var(--primary) 32%, transparent); box-shadow: 0 0 0 3px var(--focus-ring); }
.ad-search::placeholder { color: var(--dmuted2); }
.ad-filter-bar select { background: var(--surface2); border: 1px solid var(--dborder2); border-radius: 6px; color: var(--dtext); font-size: 12px; padding: 5px 10px; font-family: inherit; outline: none; cursor: pointer; }
.ad-filter-bar select:focus { border-color: color-mix(in srgb, var(--primary) 32%, transparent); box-shadow: 0 0 0 3px var(--focus-ring); }

.ad-tbl-wrap { border: 1px solid var(--dborder); border-radius: 8px; overflow: hidden; background: var(--surface); }
.ad-tbl-wrap table { width: 100%; border-collapse: collapse; }
.ad-tbl-wrap thead { background: var(--surface2); }
.ad-tbl-wrap th { padding: 8px 12px; text-align: left; font-size: 10.5px; font-weight: 600; letter-spacing: 0.06em; text-transform: uppercase; color: var(--dmuted); border-bottom: 1px solid var(--dborder); white-space: nowrap; }
.ad-tbl-wrap td { padding: 10px 12px; font-size: 12px; border-bottom: 1px solid var(--dborder); color: var(--dtext); }
.ad-tbl-wrap tr:last-child td { border-bottom: none; }
.ad-tbl-wrap tbody tr { cursor: pointer; }
.ad-tbl-wrap tbody tr:hover { background: var(--surface2); }

.ad-mono { font-family: var(--font-geist-mono), monospace; font-size: 11px; color: var(--dmuted); }
.ad-badge { display: inline-flex; align-items: center; gap: 4px; padding: 2px 7px; border-radius: 20px; font-size: 10.5px; font-weight: 600; font-family: var(--font-geist-mono), monospace; }
.ad-b-blue { background: var(--info-soft); color: var(--info); border: 1px solid color-mix(in srgb, var(--info) 22%, transparent); }
.ad-b-gray { background: var(--surface2); color: var(--dmuted); border: 1px solid var(--dborder2); }
.ad-mrr-chip { display: inline-flex; align-items: center; gap: 4px; font-size: 11px; color: var(--success); font-family: var(--font-geist-mono), monospace; background: var(--success-soft); border: 1px solid color-mix(in srgb, var(--success) 20%, transparent); padding: 1px 6px; border-radius: 3px; }

.ad-plat-icons { display: flex; gap: 3px; flex-wrap: wrap; }
.ad-plat-dot { width: 18px; height: 18px; border-radius: 4px; display: flex; align-items: center; justify-content: center; font-size: 10px; background: var(--surface2); border: 1px solid var(--dborder2); }

.ad-usage-bar { height: 3px; background: var(--surface3); border-radius: 2px; overflow: hidden; margin-top: 3px; width: 60px; }
.ad-usage-fill { height: 100%; border-radius: 2px; }
.ad-uf-g { background: #10b981; }
.ad-uf-a { background: #f59e0b; }
.ad-uf-r { background: #ef4444; }

.ad-detail-panel { position: absolute; right: 0; top: 0; bottom: 0; width: 360px; background: var(--surface-raised); border-left: 1px solid var(--dborder); padding: 20px; overflow-y: auto; z-index: 10; animation: ad-slideIn 0.18s ease-out; box-shadow: -12px 0 32px color-mix(in srgb, var(--shadow-color) 90%, transparent); }
@keyframes ad-slideIn { from { transform: translateX(20px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
.ad-panel-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.ad-panel-title { font-size: 14px; font-weight: 600; word-break: break-all; }
.ad-close-btn { background: none; border: none; color: var(--dmuted); cursor: pointer; font-size: 16px; padding: 2px; }
.ad-close-btn:hover { color: var(--dtext); }
.ad-panel-section { margin-bottom: 18px; }
.ad-panel-section-title { font-size: 10px; text-transform: uppercase; letter-spacing: 0.1em; color: var(--dmuted2); font-weight: 700; margin-bottom: 10px; }
.ad-panel-row { display: flex; align-items: flex-start; justify-content: space-between; padding: 7px 0; border-bottom: 1px solid var(--dborder); gap: 12px; }
.ad-panel-row:last-child { border-bottom: none; }
.ad-panel-key { font-size: 12px; color: var(--dmuted); }
.ad-panel-val { font-size: 12px; color: var(--dtext); text-align: right; max-width: 200px; word-break: break-all; }
`;
