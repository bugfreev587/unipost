"use client";

import { useAuth, useUser } from "@clerk/nextjs";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";

import { getMe } from "@/lib/api";

export const fmtCents = (cents: number) => {
  const dollars = cents / 100;
  return dollars >= 1000 ? `$${(dollars / 1000).toFixed(1)}k` : `$${dollars.toFixed(0)}`;
};

export const fmtNumber = (n: number) => n.toLocaleString("en-US");

export const fmtRelative = (iso: string | null | undefined) => {
  if (!iso) return "Never";
  const then = new Date(iso).getTime();
  const diff = Date.now() - then;
  if (diff < 60_000) return "just now";
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`;
  if (diff < 30 * 86_400_000) return `${Math.floor(diff / 86_400_000)}d ago`;
  return new Date(iso).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
};

export const fmtDate = (iso: string) =>
  new Date(iso).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });

const NAV_ITEMS = [
  { label: "Dashboard", href: "/admin", section: "Overview", enabled: true },
  { label: "Users", href: "/admin/users", section: "Overview", enabled: true },
  { label: "Posts", href: "/admin/posts", section: "Overview", enabled: true },
  { label: "Billing", href: "/admin/billing", section: "Revenue", enabled: true },
  { label: "MRR", href: "/admin/mrr", section: "Revenue", enabled: true },
  { label: "Errors", href: "/admin/errors", section: "System", enabled: true },
  { label: "Settings", href: "/admin/settings", section: "System", enabled: true },
] as const;

function AdminSidebar() {
  const pathname = usePathname();
  const { user } = useUser();
  const userEmail = user?.primaryEmailAddress?.emailAddress?.toLowerCase() || "";
  const first = userEmail.charAt(0).toUpperCase() || "A";

  const sections = Array.from(new Set(NAV_ITEMS.map((item) => item.section)));

  return (
    <aside className="ad-sidebar">
      <Link href="/" className="ad-sb-logo">
        <div className="ad-sb-mark">
          <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
            <path d="M9 2L4 9h4l-1 5 5-7H8l1-5z" />
          </svg>
        </div>
        <span className="ad-sb-name">UniPost</span>
        <span className="ad-sb-badge">ADMIN</span>
      </Link>

      <nav className="ad-nav">
        {sections.map((section) => (
          <div key={section}>
            <div className="ad-nav-label">{section}</div>
            {NAV_ITEMS.filter((item) => item.section === section).map((item) => {
              const active = item.href === "/admin" ? pathname === "/admin" : pathname.startsWith(item.href);
              if (!item.enabled) {
                return (
                  <div key={item.label} className="ad-nav-item ad-nav-disabled">
                    {item.label}
                  </div>
                );
              }
              return (
                <Link
                  key={item.label}
                  href={item.href}
                  className={`ad-nav-item${active ? " ad-nav-active" : ""}`}
                >
                  {item.label}
                </Link>
              );
            })}
          </div>
        ))}
      </nav>

      <div className="ad-sb-footer">
        <div className="ad-sb-user">
          <div className="ad-sb-ava">{first}</div>
          <span className="ad-sb-email">{userEmail}</span>
        </div>
      </div>
    </aside>
  );
}

export function AdminShell({
  title,
  loading,
  onRefresh,
  children,
}: {
  title: string;
  loading?: boolean;
  onRefresh?: () => void;
  children: React.ReactNode;
}) {
  const { getToken } = useAuth();
  const { isLoaded: userLoaded } = useUser();
  const [isAdmin, setIsAdmin] = useState<boolean | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token) {
          if (!cancelled) setIsAdmin(false);
          return;
        }
        const res = await getMe(token);
        if (!cancelled) setIsAdmin(!!res.data.is_admin);
      } catch {
        if (!cancelled) setIsAdmin(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [getToken]);

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

  return (
    <div style={shellStyle}>
      <style>{adminCss}</style>
      <AdminSidebar />
      <main className="ad-main">
        <div className="ad-topbar">
          <div className="ad-bc">
            <span>Admin</span>
            <span className="ad-bc-sep">/</span>
            <span className="ad-bc-cur">{title}</span>
          </div>
          <div className="ad-topbar-right">
            <span style={{ fontSize: 11, color: "var(--dmuted)", fontFamily: "var(--font-geist-mono), monospace" }}>
              {loading ? "Loading…" : "Last updated: just now"}
            </span>
            {onRefresh ? (
              <button className="ad-btn ad-btn-ghost" onClick={onRefresh} disabled={loading}>
                ↻ Refresh
              </button>
            ) : null}
          </div>
        </div>

        <div className="ad-content">{children}</div>
      </main>
    </div>
  );
}

export function StatCard({
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
        style={{ color: subColor === "up" ? "var(--success)" : subColor === "down" ? "var(--danger)" : "var(--dmuted)" }}
      >
        {sub}
      </div>
    </div>
  );
}

export function PanelRow({ k, v }: { k: string; v: React.ReactNode }) {
  return (
    <div className="ad-panel-row">
      <span className="ad-panel-key">{k}</span>
      <span className="ad-panel-val">{v}</span>
    </div>
  );
}

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

export const adminCss = `
.ad-sidebar { width: 200px; min-width: 200px; background: var(--sidebar); border-right: 1px solid var(--dborder); display: flex; flex-direction: column; }
.ad-sb-logo { display: flex; align-items: center; gap: 8px; padding: 14px 14px 12px; border-bottom: 1px solid var(--dborder); color: inherit; text-decoration: none; transition: background-color 120ms ease, color 120ms ease; }
.ad-sb-logo:hover { background: var(--sidebar-accent); }
.ad-sb-logo:focus-visible { outline: none; box-shadow: inset 0 0 0 2px var(--focus-ring); }
.ad-sb-mark { width: 22px; height: 22px; background: #10b981; border-radius: 5px; display: flex; align-items: center; justify-content: center; }
.ad-sb-mark svg { width: 11px; height: 11px; color: var(--primary-foreground); }
.ad-sb-name { font-size: 13px; font-weight: 700; letter-spacing: -0.3px; }
.ad-sb-badge { font-size: 9px; font-weight: 700; background: var(--danger-soft); color: var(--danger); border: 1px solid color-mix(in srgb, var(--danger) 22%, transparent); border-radius: 3px; padding: 1px 5px; font-family: var(--font-geist-mono), monospace; letter-spacing: 0.05em; }
.ad-nav { padding: 10px 8px; flex: 1; }
.ad-nav-label { font-size: 9.5px; font-weight: 700; letter-spacing: 0.1em; text-transform: uppercase; color: var(--dmuted2); padding: 0 6px; margin: 10px 0 3px; }
.ad-nav-item { display: flex; align-items: center; gap: 7px; padding: 5px 7px; border-radius: 6px; color: var(--dmuted); font-size: 12px; margin-bottom: 1px; border: 1px solid transparent; text-decoration: none; }
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
.ad-section-meta { font-size: 11px; color: var(--dmuted); }
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
.ad-tbl-static tbody tr { cursor: default; }
.ad-tbl-static tbody tr:hover { background: transparent; }
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
.ad-stack { display: grid; gap: 10px; }
.ad-failure-card { border: 1px solid var(--dborder); border-radius: 8px; padding: 12px; background: var(--surface); }
.ad-failure-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 8px; }
.ad-failure-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; }
.ad-failure-title { font-size: 12.5px; color: var(--dtext); font-weight: 500; }
.ad-failure-message { font-size: 11.5px; color: var(--danger); background: var(--danger-soft); border: 1px solid color-mix(in srgb, var(--danger) 18%, transparent); border-radius: 6px; padding: 8px 9px; white-space: pre-wrap; word-break: break-word; }
.ad-failure-caption { font-size: 12px; color: var(--dtext); margin: 8px 0; white-space: pre-wrap; word-break: break-word; }
.ad-link { color: var(--daccent); text-decoration: none; }
.ad-link:hover { text-decoration: underline; }
`;
