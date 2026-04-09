"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

// ── Sidebar nav data ──
interface NavItem { label: string; href: string; method?: string }
interface NavGroup { title: string; items: NavItem[] }

const NAV: NavGroup[] = [
  {
    title: "Posts",
    items: [
      { label: "Create post", href: "/docs/api/posts/create", method: "POST" },
      { label: "List posts", href: "/docs/api/posts/list", method: "GET" },
      { label: "Get post", href: "/docs/api/posts/get", method: "GET" },
      { label: "Post analytics", href: "/docs/api/posts/analytics", method: "GET" },
      { label: "Bulk publish", href: "/docs/api/posts/bulk", method: "POST" },
    ],
  },
  {
    title: "Accounts",
    items: [
      { label: "List accounts", href: "/docs/api/accounts/list", method: "GET" },
      { label: "Connect account", href: "/docs/api/accounts/connect", method: "POST" },
      { label: "Account health", href: "/docs/api/accounts/health", method: "GET" },
      { label: "Disconnect", href: "/docs/api/accounts/disconnect", method: "DELETE" },
    ],
  },
  {
    title: "Connect (Managed)",
    items: [
      { label: "Create session", href: "/docs/api/connect/sessions", method: "POST" },
    ],
  },
  {
    title: "Analytics",
    items: [
      { label: "Summary", href: "/docs/api/analytics/summary", method: "GET" },
      { label: "Trend", href: "/docs/api/analytics/trend", method: "GET" },
      { label: "By platform", href: "/docs/api/analytics/by-platform", method: "GET" },
      { label: "Rollup", href: "/docs/api/analytics/rollup", method: "GET" },
    ],
  },
  {
    title: "Media",
    items: [
      { label: "Upload media", href: "/docs/api/media/upload", method: "POST" },
      { label: "Get media", href: "/docs/api/media/get", method: "GET" },
    ],
  },
  {
    title: "Other",
    items: [
      { label: "Capabilities", href: "/docs/api/capabilities", method: "GET" },
      { label: "Usage", href: "/docs/api/usage", method: "GET" },
      { label: "Webhooks", href: "/docs/api/webhooks" },
    ],
  },
];

const CSS = `@import url('https://fonts.googleapis.com/css2?family=DM+Sans:opsz,wght@9..40,300;9..40,400;9..40,500;9..40,600;9..40,700;9..40,800;9..40,900&family=Fira+Code:wght@400;500&display=swap');
:root{--bg:#000;--s1:#0a0a0a;--s2:#111;--s3:#1a1a1a;--border:#1a1a1a;--b2:#242424;--b3:#2e2e2e;--text:#f0f0f0;--muted:#999;--muted2:#555;--accent:#10b981;--blue:#0ea5e9;--r:8px;--mono:'Fira Code',monospace;--ui:'DM Sans',system-ui,sans-serif}
*{box-sizing:border-box;margin:0;padding:0}
body{background:var(--bg);color:var(--text);font-family:var(--ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}

.api-layout{display:flex;min-height:100vh}
.api-sidebar{width:260px;flex-shrink:0;border-right:1px solid var(--border);background:#050505;position:fixed;top:0;left:0;bottom:0;overflow-y:auto;padding:20px 0;z-index:40}
.api-sidebar-logo{display:flex;align-items:center;gap:9px;padding:0 20px 20px;text-decoration:none;border-bottom:1px solid var(--border);margin-bottom:12px}
.api-sidebar-logo-mark{width:24px;height:24px;background:var(--accent);border-radius:6px;display:flex;align-items:center;justify-content:center}
.api-sidebar-logo-mark svg{width:12px;height:12px;color:#000}
.api-sidebar-logo-name{font-size:14px;font-weight:700;color:var(--text);letter-spacing:-.3px}
.api-sidebar-logo-badge{font-size:10px;font-weight:600;color:var(--accent);background:#10b98114;border:1px solid #10b98126;padding:1px 6px;border-radius:4px;font-family:var(--mono)}
.api-sidebar-back{display:flex;align-items:center;gap:6px;padding:8px 20px;font-size:12.5px;color:var(--muted2);text-decoration:none;transition:color .1s}
.api-sidebar-back:hover{color:var(--muted)}
.api-nav-group{padding:8px 0}
.api-nav-group-title{font-size:10.5px;font-weight:700;text-transform:uppercase;letter-spacing:.08em;color:var(--muted2);padding:6px 20px;margin-bottom:2px}
.api-nav-item{display:flex;align-items:center;gap:8px;padding:6px 20px;font-size:13px;color:var(--muted);text-decoration:none;transition:all .1s;border-left:2px solid transparent;margin-left:-1px}
.api-nav-item:hover{color:var(--text);background:#ffffff04}
.api-nav-item.active{color:var(--text);background:#10b98108;border-left-color:var(--accent);font-weight:600}
.api-nav-method{font-size:9.5px;font-weight:700;font-family:var(--mono);padding:1px 5px;border-radius:3px;letter-spacing:.03em;flex-shrink:0;min-width:32px;text-align:center}
.api-nav-method.GET{background:#10b98114;color:#10b981}
.api-nav-method.POST{background:#3b82f614;color:#3b82f6}
.api-nav-method.DELETE{background:#ef444414;color:#ef4444}
.api-nav-method.PUT,.api-nav-method.PATCH{background:#f59e0b14;color:#f59e0b}

.api-content{margin-left:260px;flex:1;padding:40px 48px 80px;max-width:900px}

@media(max-width:900px){
  .api-sidebar{display:none}
  .api-content{margin-left:0;padding:24px 20px 60px}
}`;

function ZapIcon() {
  return <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" width="12" height="12"><path d="M9 2L4 9h4l-1 5 5-7H8l1-5z" /></svg>;
}

export default function ApiDocsLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <div className="api-layout">
        {/* Sidebar */}
        <aside className="api-sidebar">
          <Link href="/" className="api-sidebar-logo">
            <span className="api-sidebar-logo-mark"><ZapIcon /></span>
            <span className="api-sidebar-logo-name">UniPost</span>
            <span className="api-sidebar-logo-badge">API</span>
          </Link>
          <Link href="/docs" className="api-sidebar-back">&larr; Full docs</Link>

          {NAV.map(group => (
            <div key={group.title} className="api-nav-group">
              <div className="api-nav-group-title">{group.title}</div>
              {group.items.map(item => (
                <Link
                  key={item.href}
                  href={item.href}
                  className={`api-nav-item${pathname === item.href ? " active" : ""}`}
                >
                  {item.method && <span className={`api-nav-method ${item.method}`}>{item.method}</span>}
                  {item.label}
                </Link>
              ))}
            </div>
          ))}
        </aside>

        {/* Content */}
        <main className="api-content">
          {children}
        </main>
      </div>
    </>
  );
}
