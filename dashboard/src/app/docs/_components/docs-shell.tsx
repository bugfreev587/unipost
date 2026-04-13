"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useMemo, useState } from "react";

type NavLeaf = {
  label: string;
  href: string;
  badge?: string;
};

type NavGroup = {
  title: string;
  items: NavLeaf[];
};

type NavSection = {
  title: string;
  description?: string;
  groups?: NavGroup[];
  items?: NavLeaf[];
};

type HeadingItem = {
  id: string;
  text: string;
  level: "h2" | "h3";
};

const APP_URL = process.env.NEXT_PUBLIC_APP_URL || "https://app.unipost.dev";

export const DOCS_NAV: NavSection[] = [
  {
    title: "Get Started",
    description: "Fastest path from API key to first publish.",
    items: [
      { label: "Quickstart", href: "/docs/quickstart" },
      { label: "SDKs", href: "/docs/sdk" },
      { label: "MCP", href: "/docs/mcp" },
      { label: "Pricing", href: "/docs/pricing" },
    ],
  },
  {
    title: "Platforms",
    description: "Per-platform support, constraints, and examples.",
    items: [
      { label: "Overview", href: "/docs/platforms" },
      { label: "Twitter/X", href: "/docs/platforms/twitter" },
      { label: "LinkedIn", href: "/docs/platforms/linkedin" },
      { label: "Instagram", href: "/docs/platforms/instagram" },
      { label: "Threads", href: "/docs/platforms/threads" },
      { label: "TikTok", href: "/docs/platforms/tiktok" },
      { label: "YouTube", href: "/docs/platforms/youtube" },
      { label: "Bluesky", href: "/docs/platforms/bluesky" },
    ],
  },
  {
    title: "API References",
    description: "Endpoint-level docs for auth, publishing, and analytics.",
    groups: [
      {
        title: "Core",
        items: [
          { label: "Overview", href: "/docs/api" },
          { label: "Authentication", href: "/docs/api/authentication" },
          { label: "Errors", href: "/docs/api/errors" },
        ],
      },
      {
        title: "Accounts",
        items: [
          { label: "Social Accounts", href: "/docs/api/accounts/list" },
          { label: "Connect Sessions", href: "/docs/api/connect/sessions" },
          { label: "Managed Users", href: "/docs/api/users" },
          { label: "Account Health", href: "/docs/api/accounts/health" },
        ],
      },
      {
        title: "Publishing",
        items: [
          { label: "Create Post", href: "/docs/api/posts/create" },
          { label: "Validate", href: "/docs/api/posts/validate" },
          { label: "Drafts", href: "/docs/api/posts/drafts" },
          { label: "Media", href: "/docs/api/media" },
        ],
      },
      {
        title: "Insights",
        items: [
          { label: "Analytics", href: "/docs/api/analytics" },
          { label: "Webhooks", href: "/docs/api/webhooks" },
          { label: "Billing", href: "/docs/api/billing" },
        ],
      },
    ],
  },
];

const CSS = `
:root{--docs-bg:#050505;--docs-panel:#0b0b0b;--docs-panel-2:#101010;--docs-border:#1d1d1d;--docs-border-2:#292929;--docs-text:#f7f7f5;--docs-muted:#a6a6a0;--docs-muted-2:#6f6f69;--docs-accent:#22c55e;--docs-blue:#67b7ff;--docs-warm:#efe7d7;--docs-radius:14px;--docs-radius-sm:10px;--docs-shadow:0 18px 50px rgba(0,0,0,.28);--docs-mono:var(--font-fira-code),monospace;--docs-ui:var(--font-dm-sans),system-ui,sans-serif}
*{box-sizing:border-box}
body{background:var(--docs-bg);color:var(--docs-text);font-family:var(--docs-ui);-webkit-font-smoothing:antialiased}
.docs-shell{min-height:100vh;background:
radial-gradient(circle at top left, rgba(34,197,94,.08), transparent 30%),
linear-gradient(180deg, #090909 0%, #050505 38%, #050505 100%)}
.docs-topbar{position:sticky;top:0;z-index:50;border-bottom:1px solid rgba(255,255,255,.05);background:rgba(5,5,5,.78);backdrop-filter:blur(14px)}
.docs-topbar-inner{max-width:1560px;margin:0 auto;padding:0 28px;height:62px;display:flex;align-items:center;justify-content:space-between;gap:18px}
.docs-brand{display:flex;align-items:center;gap:12px;text-decoration:none;color:inherit;min-width:0}
.docs-brand-mark{width:30px;height:30px;border-radius:9px;background:linear-gradient(135deg,#22c55e,#7dd3fc);display:flex;align-items:center;justify-content:center;color:#041108;box-shadow:0 10px 26px rgba(34,197,94,.18)}
.docs-brand-copy{display:flex;flex-direction:column;gap:2px;min-width:0}
.docs-brand-name{display:block;font-size:15px;font-weight:700;letter-spacing:-.02em;line-height:1.2}
.docs-brand-context{display:block;font-size:12px;line-height:1.45;color:var(--docs-muted)}
.docs-topbar-links{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
.docs-topbar-link{padding:8px 12px;border-radius:999px;font-size:13px;color:var(--docs-muted);text-decoration:none;transition:all .14s}
.docs-topbar-link:hover{color:var(--docs-text);background:rgba(255,255,255,.04)}
.docs-topbar-link.active{color:var(--docs-text);background:rgba(255,255,255,.06)}
.docs-layout{max-width:1560px;margin:0 auto;padding:32px 28px 80px;display:grid;grid-template-columns:260px minmax(0,1fr) 240px;gap:32px}
.docs-sidebar,.docs-toc{position:sticky;top:94px;align-self:start;max-height:calc(100vh - 120px);overflow:auto;padding-bottom:16px}
.docs-sidebar-card,.docs-toc-card{background:rgba(255,255,255,.02);border:1px solid rgba(255,255,255,.05);border-radius:18px;padding:16px 14px;box-shadow:var(--docs-shadow)}
.docs-sidebar-section{padding:12px 10px 10px;border:1px solid rgba(255,255,255,.05);border-radius:16px;background:linear-gradient(180deg,rgba(255,255,255,.03),rgba(255,255,255,.015));margin-bottom:14px}
.docs-sidebar-section:last-child{margin-bottom:0}
.docs-sidebar-section-header{padding:2px 2px 10px;margin-bottom:4px;border-bottom:1px solid rgba(255,255,255,.05)}
.docs-section-label{padding:0;font-size:11px;font-weight:700;letter-spacing:.1em;text-transform:uppercase;color:var(--docs-muted-2)}
.docs-section-desc{margin-top:6px;font-size:12.5px;line-height:1.55;color:var(--docs-muted-2)}
.docs-nav-group-title{padding:12px 10px 6px;font-size:11px;font-weight:700;letter-spacing:.08em;text-transform:uppercase;color:var(--docs-muted-2)}
.docs-nav-link{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:8px 10px;border-radius:10px;font-size:13.5px;color:var(--docs-muted);text-decoration:none;transition:all .12s}
.docs-nav-link:hover{color:var(--docs-text);background:rgba(255,255,255,.04)}
.docs-nav-link.active{color:var(--docs-text);background:rgba(34,197,94,.08);box-shadow:inset 0 0 0 1px rgba(34,197,94,.18)}
.docs-nav-badge{font-size:10px;font-family:var(--docs-mono);padding:2px 6px;border-radius:999px;background:rgba(255,255,255,.06);color:var(--docs-muted-2)}
.docs-main{min-width:0}
.docs-page{background:rgba(255,255,255,.02);border:1px solid rgba(255,255,255,.05);border-radius:24px;padding:42px 46px;box-shadow:var(--docs-shadow)}
.docs-eyebrow{display:inline-flex;align-items:center;gap:8px;padding:6px 10px;border-radius:999px;background:rgba(255,255,255,.04);border:1px solid rgba(255,255,255,.05);font-size:11px;font-weight:700;letter-spacing:.08em;text-transform:uppercase;color:var(--docs-muted);margin-bottom:18px}
.docs-page h1{font-size:44px;line-height:1.02;letter-spacing:-.05em;margin:0 0 14px;color:var(--docs-warm)}
.docs-lead{font-size:18px;line-height:1.7;color:var(--docs-muted);margin:0 0 34px;max-width:760px}
.docs-page h2,.docs-page h3{scroll-margin-top:96px}
.docs-page h2{font-size:24px;line-height:1.2;letter-spacing:-.03em;margin:40px 0 14px;color:var(--docs-text)}
.docs-page h3{font-size:18px;line-height:1.3;letter-spacing:-.02em;margin:28px 0 12px;color:var(--docs-text)}
.docs-page p{font-size:15px;line-height:1.8;color:var(--docs-muted);margin:0 0 16px}
.docs-page a{color:var(--docs-blue);text-decoration:none}
.docs-page a:hover{text-decoration:underline}
.docs-page code{font-family:var(--docs-mono);font-size:12.5px;background:rgba(255,255,255,.04);border:1px solid rgba(255,255,255,.06);padding:1px 6px;border-radius:7px;color:var(--docs-text)}
.docs-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin:22px 0}
.docs-card{background:var(--docs-panel);border:1px solid var(--docs-border);border-radius:18px;padding:18px}
.docs-card h3{margin-top:0}
.docs-card-title{font-size:15px;font-weight:700;color:var(--docs-text);margin-bottom:8px}
.docs-card p{margin-bottom:0;font-size:14px}
.docs-list{margin:0 0 16px;padding-left:18px;color:var(--docs-muted)}
.docs-list li{margin-bottom:8px;line-height:1.7}
.docs-table-wrap{overflow:auto;margin:18px 0}
.docs-table{width:100%;border-collapse:collapse;min-width:620px;border:1px solid var(--docs-border);border-radius:16px;overflow:hidden;background:var(--docs-panel)}
.docs-table th{font-size:11px;font-weight:700;letter-spacing:.08em;text-transform:uppercase;color:var(--docs-muted-2);text-align:left;padding:12px 14px;background:var(--docs-panel-2);border-bottom:1px solid var(--docs-border)}
.docs-table td{padding:14px;color:var(--docs-muted);font-size:14px;border-bottom:1px solid var(--docs-border)}
.docs-table tr:last-child td{border-bottom:none}
.docs-code{margin:18px 0;padding:18px 20px;border-radius:18px;background:#0f1011;border:1px solid #212325;color:#ddded7;font-family:var(--docs-mono);font-size:13px;line-height:1.75;overflow:auto}
.docs-callout{margin:20px 0;padding:16px 18px;border-radius:16px;background:rgba(34,197,94,.06);border:1px solid rgba(34,197,94,.16);color:var(--docs-muted)}
.docs-callout strong{color:var(--docs-text)}
.docs-toc-title{padding:6px 8px 10px;font-size:11px;font-weight:700;letter-spacing:.1em;text-transform:uppercase;color:var(--docs-muted-2)}
.docs-toc-link{display:block;padding:7px 8px;border-radius:8px;font-size:12.5px;color:var(--docs-muted);text-decoration:none;transition:all .12s}
.docs-toc-link:hover{color:var(--docs-text);background:rgba(255,255,255,.04)}
.docs-toc-link.active{color:var(--docs-text);background:rgba(255,255,255,.05)}
.docs-toc-link.level-h3{padding-left:18px;color:var(--docs-muted-2)}
.docs-empty-toc{padding:8px;color:var(--docs-muted-2);font-size:12.5px;line-height:1.6}
@media (max-width:1240px){.docs-layout{grid-template-columns:240px minmax(0,1fr)}.docs-toc{display:none}}
@media (max-width:900px){.docs-topbar-inner{padding:0 18px;height:auto;min-height:62px;align-items:flex-start;flex-direction:column;justify-content:center;padding-top:10px;padding-bottom:10px}.docs-topbar-links{width:100%}.docs-layout{grid-template-columns:1fr;padding:20px 16px 56px}.docs-sidebar{display:none}.docs-page{padding:28px 22px;border-radius:20px}.docs-page h1{font-size:34px}.docs-grid{grid-template-columns:1fr}}
`;

function markSvg() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" width="15" height="15">
      <path d="M9 2 4 9h4l-1 5 5-7H8l1-5Z" />
    </svg>
  );
}

function isLeafActive(current: string, href: string) {
  return current === href.split("#")[0];
}

function isTopLevelActive(current: string, href: string) {
  if (href.includes("#")) {
    return current === href.split("#")[0];
  }
  if (href === "/docs") {
    return current === "/docs" || current.startsWith("/docs/");
  }
  return current === href || current.startsWith(`${href}/`);
}

export function DocsShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const [activeHeading, setActiveHeading] = useState("");

  const headings =
    typeof document === "undefined"
      ? ([] as HeadingItem[])
      : Array.from(document.querySelectorAll<HTMLElement>(".docs-page h2[id], .docs-page h3[id]"))
          .map((node) => ({
            id: node.id,
            text: node.textContent?.trim() || "",
            level: node.tagName.toLowerCase() as "h2" | "h3",
          }))
          .filter((item) => item.id && item.text);

  useEffect(() => {
    const headingNodes = Array.from(
      document.querySelectorAll<HTMLElement>(".docs-page h2[id], .docs-page h3[id]")
    );

    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (visible[0]?.target instanceof HTMLElement) {
          setActiveHeading(visible[0].target.id);
        }
      },
      { rootMargin: "-100px 0px -60% 0px", threshold: [0.1, 1] }
    );

    headingNodes.forEach((node) => observer.observe(node));
    return () => observer.disconnect();
  }, [pathname]);

  const topLinks = useMemo(
    () => [
      { label: "Docs", href: "/docs" },
      { label: "API", href: "/docs/api" },
      { label: "Dashboard", href: APP_URL, external: true },
      { label: "Pricing", href: "/pricing" },
    ],
    []
  );

  return (
    <div className="docs-shell">
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <header className="docs-topbar">
        <div className="docs-topbar-inner">
          <Link href="/docs" className="docs-brand">
            <span className="docs-brand-mark">{markSvg()}</span>
            <span className="docs-brand-copy">
              <span className="docs-brand-name">UniPost Docs</span>
              <span className="docs-brand-context">Build social publishing, account onboarding, and analytics.</span>
            </span>
          </Link>
          <nav className="docs-topbar-links">
            {topLinks.map((link) => (
              link.external ? (
                <a
                  key={link.label}
                  href={link.href}
                  className="docs-topbar-link"
                >
                  {link.label}
                </a>
              ) : (
                <Link
                  key={link.href}
                  href={link.href}
                  className={`docs-topbar-link${isTopLevelActive(pathname, link.href) ? " active" : ""}`}
                >
                  {link.label}
                </Link>
              )
            ))}
          </nav>
        </div>
      </header>

      <div className="docs-layout">
        <aside className="docs-sidebar">
          <div className="docs-sidebar-card">
            {DOCS_NAV.map((section) => (
              <section key={section.title} className="docs-sidebar-section">
                <div className="docs-sidebar-section-header">
                  <div className="docs-section-label">{section.title}</div>
                  {section.description ? <div className="docs-section-desc">{section.description}</div> : null}
                </div>
                {section.items?.map((item) => (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={`docs-nav-link${isLeafActive(pathname, item.href) ? " active" : ""}`}
                  >
                    <span>{item.label}</span>
                    {item.badge ? <span className="docs-nav-badge">{item.badge}</span> : null}
                  </Link>
                ))}
                {section.groups?.map((group) => (
                  <div key={group.title}>
                    <div className="docs-nav-group-title">{group.title}</div>
                    {group.items.map((item) => (
                      <Link
                        key={item.href}
                        href={item.href}
                        className={`docs-nav-link${isLeafActive(pathname, item.href) ? " active" : ""}`}
                      >
                        <span>{item.label}</span>
                        {item.badge ? <span className="docs-nav-badge">{item.badge}</span> : null}
                      </Link>
                    ))}
                  </div>
                ))}
              </section>
            ))}
          </div>
        </aside>

        <main className="docs-main">{children}</main>

        <aside className="docs-toc">
          <div className="docs-toc-card">
            <div className="docs-toc-title">On This Page</div>
            {headings.length === 0 ? (
              <div className="docs-empty-toc">This page is a navigation hub. Open a guide or reference page to see section links here.</div>
            ) : (
              headings.map((heading) => (
                <a
                  key={heading.id}
                  href={`#${heading.id}`}
                  className={`docs-toc-link level-${heading.level}${activeHeading === heading.id ? " active" : ""}`}
                >
                  {heading.text}
                </a>
              ))
            )}
          </div>
        </aside>
      </div>
    </div>
  );
}

export function DocsPage({
  eyebrow,
  title,
  lead,
  children,
}: {
  eyebrow?: string;
  title: string;
  lead: string;
  children: React.ReactNode;
}) {
  return (
    <article className="docs-page">
      {eyebrow ? <div className="docs-eyebrow">{eyebrow}</div> : null}
      <h1>{title}</h1>
      <p className="docs-lead">{lead}</p>
      {children}
    </article>
  );
}

export function DocsTable({
  columns,
  rows,
}: {
  columns: readonly string[];
  rows: readonly (readonly string[])[];
}) {
  return (
    <div className="docs-table-wrap">
      <table className="docs-table">
        <thead>
          <tr>
            {columns.map((column) => (
              <th key={column}>{column}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr key={index}>
              {row.map((cell, cellIndex) => (
                <td key={cellIndex}>{cell}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function DocsCode({ code }: { code: string }) {
  return <pre className="docs-code"><code>{code}</code></pre>;
}

export function DocsCodeTabs({
  snippets,
}: {
  snippets: Array<{ label: string; code: string }>;
}) {
  const [active, setActive] = useState(0);

  return (
    <div className="docs-card" style={{ padding: 0, overflow: "hidden" }}>
      <div style={{ display: "flex", gap: 6, padding: 12, borderBottom: "1px solid var(--docs-border)", background: "var(--docs-panel-2)", flexWrap: "wrap" }}>
        {snippets.map((snippet, index) => (
          <button
            key={snippet.label}
            type="button"
            onClick={() => setActive(index)}
            style={{
              padding: "7px 12px",
              borderRadius: 10,
              border: "1px solid",
              borderColor: index === active ? "rgba(34,197,94,.24)" : "rgba(255,255,255,.06)",
              background: index === active ? "rgba(34,197,94,.08)" : "rgba(255,255,255,.03)",
              color: index === active ? "var(--docs-text)" : "var(--docs-muted)",
              fontSize: 12.5,
              fontFamily: "var(--docs-mono)",
              cursor: "pointer",
            }}
          >
            {snippet.label}
          </button>
        ))}
      </div>
      <pre className="docs-code" style={{ margin: 0, border: "none", borderRadius: 0 }}>
        <code>{snippets[active].code}</code>
      </pre>
    </div>
  );
}
