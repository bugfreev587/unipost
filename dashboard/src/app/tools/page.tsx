"use client";

import Link from "next/link";
import { MarketingNav, MarketingCTA } from "@/components/marketing/nav";

// ── Styles (same design language as /solutions) ──
const CSS = `@import url('https://fonts.googleapis.com/css2?family=DM+Sans:opsz,wght@9..40,300;9..40,400;9..40,500;9..40,600;9..40,700;9..40,800;9..40,900&family=Fira+Code:wght@400;500&display=swap');:root{--bg:#000;--s1:#0a0a0a;--s2:#111;--s3:#1a1a1a;--border:#1a1a1a;--b2:#242424;--b3:#2e2e2e;--text:#f0f0f0;--muted:#999;--muted2:#555;--accent:#10b981;--blue:#0ea5e9;--r:8px;--mono:'Fira Code',monospace;--ui:'DM Sans',system-ui,sans-serif;--nav-max:1480px;--content-max:1200px;--text-max:720px;--px:32px;--section-py:96px}*{box-sizing:border-box;margin:0;padding:0}body{background:var(--bg);color:var(--text);font-family:var(--ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}.tl-nav{position:sticky;top:0;z-index:50;width:100%;border-bottom:1px solid var(--border);background:#00000095;backdrop-filter:blur(12px);-webkit-backdrop-filter:blur(12px)}.tl-nav-inner{max-width:var(--nav-max);margin:0 auto;padding:0 var(--px);height:56px;display:flex;align-items:center;justify-content:space-between}.tl-logo{display:flex;align-items:center;gap:10px;text-decoration:none}.tl-logo-mark{width:28px;height:28px;background:var(--accent);border-radius:7px;display:flex;align-items:center;justify-content:center}.tl-logo-mark svg{width:14px;height:14px;color:#000}.tl-logo-name{font-size:16px;font-weight:700;letter-spacing:-.4px;color:var(--text)}.tl-nav-links{display:flex;align-items:center;gap:4px}.tl-nav-link{padding:6px 14px;font-size:14px;font-weight:500;color:var(--muted);cursor:pointer;border-radius:var(--r);transition:color .1s;text-decoration:none}.tl-nav-link:hover{color:var(--text)}.tl-nav-link.active{color:var(--text)}.lp-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--ui);text-decoration:none;white-space:nowrap}.lp-btn-primary{background:var(--blue);color:#000}.lp-btn-primary:hover{background:#38bdf8;box-shadow:0 0 24px #0ea5e930}.lp-btn-ghost{background:transparent;color:var(--muted);border-color:var(--b2)}.lp-btn-ghost:hover{background:var(--s2);color:var(--text);border-color:var(--b3)}.lp-btn-outline{background:transparent;color:var(--text);border-color:var(--b2)}.lp-btn-outline:hover{background:var(--s2);border-color:var(--b3)}.lp-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}.tl-page{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}.tl-hero{padding:var(--section-py) 0 56px;max-width:880px}.tl-eyebrow{font-size:11.5px;color:var(--accent);text-transform:uppercase;letter-spacing:.12em;font-weight:700;margin-bottom:18px;font-family:var(--mono)}.tl-hero-title{font-size:56px;font-weight:900;letter-spacing:-2px;line-height:1.05;color:var(--text);margin-bottom:24px}.tl-hero-title em{color:var(--accent);font-style:normal}.tl-hero-sub{font-size:18px;color:#aaa;line-height:1.7;max-width:680px}.tl-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:20px;padding:0 0 var(--section-py)}.tl-card{background:var(--s1);border:1px solid var(--b2);border-radius:14px;padding:32px 30px;display:flex;flex-direction:column;gap:14px;transition:all .2s;position:relative;text-decoration:none;color:inherit;min-height:260px}.tl-card:hover{border-color:#333;background:#0d0d0d;transform:translateY(-2px);box-shadow:0 8px 32px #00000040}.tl-card-badge{display:inline-flex;padding:4px 10px;border-radius:6px;font-size:11px;font-weight:700;text-transform:uppercase;letter-spacing:.06em;font-family:var(--mono);width:fit-content}.tl-card-icon{width:44px;height:44px;border-radius:10px;display:flex;align-items:center;justify-content:center;flex-shrink:0}.tl-card-title{font-size:20px;font-weight:700;letter-spacing:-.3px;color:var(--text);margin-top:4px}.tl-card-desc{font-size:14px;color:#999;line-height:1.65;flex:1}.tl-card-tags{display:flex;gap:6px;flex-wrap:wrap;margin-top:auto}.tl-card-tag{font-size:11px;color:var(--muted);background:var(--s3);border:1px solid var(--b2);border-radius:5px;padding:2px 8px;font-family:var(--mono)}.tl-card-soon{font-size:11px;font-weight:600;color:var(--muted2);text-transform:uppercase;letter-spacing:.08em;font-family:var(--mono);margin-top:auto}.tl-footer{width:100%;border-top:1px solid var(--border);padding:32px 0;margin-top:32px}.tl-footer-inner{max-width:var(--content-max);margin:0 auto;padding:0 var(--px);display:flex;align-items:center;justify-content:space-between;font-size:13px;color:var(--muted2)}.tl-footer-inner a{color:var(--blue);text-decoration:none}.tl-footer-inner a:hover{text-decoration:underline}@media(max-width:1024px){.tl-grid{grid-template-columns:1fr 1fr}.tl-hero-title{font-size:44px}}@media(max-width:680px){.tl-grid{grid-template-columns:1fr}.tl-hero-title{font-size:34px}}`;

function ZapIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" width="14" height="14">
      <path d="M9 2L4 9h4l-1 5 5-7H8l1-5z" />
    </svg>
  );
}

// ── Tool card data ──
interface ToolCardData {
  href: string;
  badge: string;
  badgeColor: string;
  iconBg: string;
  iconBorder: string;
  icon: React.ReactNode;
  title: string;
  desc: string;
  tags: string[];
  soon?: boolean;
}

const TOOLS: ToolCardData[] = [
  {
    href: "/tools/agentpost",
    badge: "Open Source",
    badgeColor: "#10b981",
    iconBg: "#10b98114",
    iconBorder: "#10b98126",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="#10b981" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" width="22" height="22">
        <path d="M12 2L2 7l10 5 10-5-10-5z" />
        <path d="M2 17l10 5 10-5" />
        <path d="M2 12l10 5 10-5" />
      </svg>
    ),
    title: "AgentPost",
    desc: "AI-native CLI that turns a one-line update into platform-perfect social posts and publishes everywhere. Describe what you shipped, Claude drafts per-platform copy, one keypress publishes.",
    tags: ["CLI", "AI", "MIT"],
  },
  {
    href: "#",
    badge: "Coming Soon",
    badgeColor: "#6366f1",
    iconBg: "#6366f114",
    iconBorder: "#6366f126",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="#6366f1" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" width="22" height="22">
        <rect x="2" y="3" width="20" height="14" rx="2" />
        <path d="M8 21h8" />
        <path d="M12 17v4" />
        <path d="M7 10l3 3 7-7" />
      </svg>
    ),
    title: "Connect Widget",
    desc: "Drop-in embeddable component for your app. Let your end users connect their social accounts with a single click — no backend code required.",
    tags: ["React", "Web Component", "Embeddable"],
    soon: true,
  },
  {
    href: "#",
    badge: "Coming Soon",
    badgeColor: "#f59e0b",
    iconBg: "#f59e0b14",
    iconBorder: "#f59e0b26",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="#f59e0b" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" width="22" height="22">
        <path d="M3 3v18h18" />
        <path d="M7 16l4-8 4 4 5-10" />
      </svg>
    ),
    title: "Analytics Explorer",
    desc: "Interactive dashboard for the /v1/analytics/rollup endpoint. Visualize publish volume by platform, account, and time — no SQL required.",
    tags: ["Dashboard", "Charts", "Rollup API"],
    soon: true,
  },
];

export default function ToolsPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />

      {/* Nav */}
      <nav className="tl-nav">
        <div className="tl-nav-inner">
          <div style={{ display: "flex", alignItems: "center", gap: 32 }}>
            <Link href="/" className="tl-logo">
              <span className="tl-logo-mark"><ZapIcon /></span>
              <span className="tl-logo-name">UniPost</span>
            </Link>
            <div className="tl-nav-links">
              <Link href="/solutions" className="tl-nav-link">Solutions</Link>
              <Link href="/tools" className="tl-nav-link active">Tools</Link>
              <Link href="/pricing" className="tl-nav-link">Pricing</Link>
              <Link href="/docs" className="tl-nav-link">Docs</Link>
            </div>
          </div>
          <MarketingNav />
        </div>
      </nav>

      {/* Hero */}
      <div className="tl-page">
        <div className="tl-hero">
          <div className="tl-eyebrow">Developer Tools</div>
          <h1 className="tl-hero-title">
            Tools built on <em>UniPost</em>
          </h1>
          <p className="tl-hero-sub">
            Open-source CLIs, embeddable widgets, and interactive dashboards
            that make the UniPost API easier to use. Each tool is standalone
            and MIT-licensed — use them directly or fork them as a starting
            point for your own integration.
          </p>
        </div>

        {/* Tool grid */}
        <div className="tl-grid">
          {TOOLS.map((tool) => (
            <ToolCard key={tool.title} tool={tool} />
          ))}
        </div>
      </div>

      {/* Footer */}
      <footer className="tl-footer">
        <div className="tl-footer-inner">
          <span>&copy; {new Date().getFullYear()} UniPost</span>
          <span>
            <a href="https://github.com/unipost-dev" target="_blank" rel="noopener noreferrer">
              GitHub
            </a>
          </span>
        </div>
      </footer>
    </>
  );
}

// ── Reusable ToolCard component ──
function ToolCard({ tool }: { tool: ToolCardData }) {
  const Wrapper = tool.soon ? "div" : Link;
  return (
    <Wrapper
      href={tool.href}
      className="tl-card"
      style={tool.soon ? { opacity: 0.6, pointerEvents: "none" as const } : undefined}
    >
      <span
        className="tl-card-badge"
        style={{
          background: tool.badgeColor + "14",
          color: tool.badgeColor,
          border: `1px solid ${tool.badgeColor}26`,
        }}
      >
        {tool.badge}
      </span>
      <span
        className="tl-card-icon"
        style={{
          background: tool.iconBg,
          border: `1px solid ${tool.iconBorder}`,
        }}
      >
        {tool.icon}
      </span>
      <span className="tl-card-title">{tool.title}</span>
      <span className="tl-card-desc">{tool.desc}</span>
      <span className="tl-card-tags">
        {tool.tags.map((t) => (
          <span key={t} className="tl-card-tag">{t}</span>
        ))}
      </span>
    </Wrapper>
  );
}
