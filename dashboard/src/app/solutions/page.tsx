"use client";

import Link from "next/link";
import { MarketingNav, MarketingCTA } from "@/components/marketing/nav";

// ── Styles ──
const CSS = `:root{--bg:#000;--s1:#0a0a0a;--s2:#111;--s3:#1a1a1a;--border:#1a1a1a;--b2:#242424;--b3:#2e2e2e;--text:#f0f0f0;--muted:#999;--muted2:#555;--accent:#10b981;--blue:#0ea5e9;--r:8px;--mono:var(--font-fira-code),monospace;--ui:var(--font-dm-sans),system-ui,sans-serif;--nav-max:1480px;--content-max:1200px;--text-max:720px;--px:32px;--section-py:96px}*{box-sizing:border-box;margin:0;padding:0}body{background:var(--bg);color:var(--text);font-family:var(--ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}.sol-nav{position:sticky;top:0;z-index:50;width:100%;border-bottom:1px solid var(--border);background:#00000095;backdrop-filter:blur(12px);-webkit-backdrop-filter:blur(12px)}.sol-nav-inner{max-width:var(--nav-max);margin:0 auto;padding:0 var(--px);height:56px;display:flex;align-items:center;justify-content:space-between}.sol-logo{display:flex;align-items:center;gap:10px;text-decoration:none}.sol-logo-mark{width:28px;height:28px;background:var(--accent);border-radius:7px;display:flex;align-items:center;justify-content:center}.sol-logo-mark svg{width:14px;height:14px;color:#000}.sol-logo-name{font-size:16px;font-weight:700;letter-spacing:-.4px;color:var(--text)}.sol-nav-links{display:flex;align-items:center;gap:4px}.sol-nav-link{padding:6px 14px;font-size:14px;font-weight:500;color:var(--muted);cursor:pointer;border-radius:var(--r);transition:color .1s;text-decoration:none}.sol-nav-link:hover{color:var(--text)}.sol-nav-link.active{color:var(--text);font-weight:500}.lp-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--ui);text-decoration:none;white-space:nowrap}.lp-btn-primary{background:var(--blue);color:#000}.lp-btn-primary:hover{background:#38bdf8;box-shadow:0 0 24px #0ea5e930}.lp-btn-ghost{background:transparent;color:var(--muted);border-color:var(--b2)}.lp-btn-ghost:hover{background:var(--s2);color:var(--text);border-color:var(--b3)}.lp-btn-outline{background:transparent;color:var(--text);border-color:var(--b2)}.lp-btn-outline:hover{background:var(--s2);border-color:var(--b3)}.lp-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}.sol-page{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}.sol-hero{padding:var(--section-py) 0 56px;max-width:880px}.sol-eyebrow{font-size:11.5px;color:var(--accent);text-transform:uppercase;letter-spacing:.12em;font-weight:700;margin-bottom:18px;font-family:var(--mono)}.sol-hero-title{font-size:64px;font-weight:900;letter-spacing:-2px;line-height:1.05;color:var(--text);margin-bottom:24px}.sol-hero-title em{color:var(--accent);font-style:normal}.sol-hero-sub{font-size:18px;color:#aaa;line-height:1.7;max-width:680px}.sol-grid-section{padding:0 0 var(--section-py)}.sol-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:20px}.sol-card{background:var(--s1);border:1px solid var(--b2);border-radius:14px;padding:32px 30px;display:flex;flex-direction:column;gap:14px;transition:all .2s;position:relative;min-height:240px}.sol-card:hover{border-color:#333;background:#0d0d0d;transform:translateY(-2px);box-shadow:0 8px 32px #00000040}.sol-card-icon{width:44px;height:44px;border-radius:10px;background:#0ea5e914;border:1px solid #0ea5e926;display:flex;align-items:center;justify-content:center;color:var(--blue);flex-shrink:0}.sol-card-icon svg{width:20px;height:20px}.sol-card-title{font-size:18px;font-weight:700;letter-spacing:-.3px;color:var(--text);margin-top:6px}.sol-card-desc{font-size:14px;color:#999;line-height:1.65;flex:1}.sol-card-soon{font-size:11px;font-weight:600;color:var(--muted2);text-transform:uppercase;letter-spacing:.08em;font-family:var(--mono);margin-top:auto}.sol-cta{padding:0 0 var(--section-py)}.sol-cta-inner{background:#0d0d0d;border:1px solid var(--border);border-radius:16px;padding:64px 56px;text-align:center;position:relative;overflow:hidden}.sol-cta-glow{position:absolute;width:400px;height:400px;border-radius:50%;background:radial-gradient(circle,#10b98112,transparent 70%);top:50%;left:50%;transform:translate(-50%,-50%);pointer-events:none}.sol-cta-title{font-size:38px;font-weight:800;letter-spacing:-.8px;margin-bottom:14px;position:relative}.sol-cta-sub{font-size:15px;color:#aaa;margin-bottom:32px;position:relative;max-width:560px;margin-left:auto;margin-right:auto}.sol-cta-actions{display:flex;align-items:center;justify-content:center;gap:12px;position:relative;flex-wrap:wrap}.sol-footer{width:100%;border-top:1px solid var(--border);padding:32px 0;margin-top:32px}.sol-footer-inner{max-width:var(--content-max);margin:0 auto;padding:0 var(--px);display:flex;align-items:center;justify-content:space-between;font-size:13px;color:var(--muted2)}.sol-footer-inner a{color:var(--blue);text-decoration:none}.sol-footer-inner a:hover{text-decoration:underline}@media(min-width:1600px){:root{--nav-max:1560px;--content-max:1280px;--px:40px}}@media(max-width:1024px){:root{--nav-max:100%;--content-max:100%;--px:24px;--section-py:64px}.sol-grid{grid-template-columns:1fr 1fr}.sol-hero-title{font-size:48px}}@media(max-width:680px){.sol-grid{grid-template-columns:1fr}.sol-hero-title{font-size:38px}.sol-cta-inner{padding:48px 28px}.sol-cta-title{font-size:28px}}`;

function ZapIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" width="14" height="14">
      <path d="M9 2L4 9h4l-1 5 5-7H8l1-5z" />
    </svg>
  );
}

// Card icons (Lucide-style, stroked)
const ICONS: Record<string, React.ReactElement> = {
  saas: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="4" width="20" height="14" rx="2" />
      <path d="M2 10h20" />
      <path d="M8 21h8" />
      <path d="M12 18v3" />
    </svg>
  ),
  ai: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <rect x="4" y="6" width="16" height="14" rx="2" />
      <path d="M9 2v4" />
      <path d="M15 2v4" />
      <circle cx="9" cy="13" r="1" />
      <circle cx="15" cy="13" r="1" />
      <path d="M9 17h6" />
    </svg>
  ),
  ecommerce: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 3h2l3 13h11l2-9H7" />
      <circle cx="9" cy="20" r="1.5" />
      <circle cx="18" cy="20" r="1.5" />
    </svg>
  ),
  scheduler: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="5" width="18" height="16" rx="2" />
      <path d="M16 3v4" />
      <path d="M8 3v4" />
      <path d="M3 11h18" />
      <circle cx="12" cy="16" r="2" />
    </svg>
  ),
  multiAccount: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="9" cy="9" r="3.5" />
      <path d="M2.5 20a6.5 6.5 0 0 1 13 0" />
      <circle cx="17" cy="7" r="2.5" />
      <path d="M21.5 17a4.5 4.5 0 0 0-4.5-4.5" />
    </svg>
  ),
  agency: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 21h18" />
      <path d="M5 21V7l7-4 7 4v14" />
      <path d="M9 9h.01" />
      <path d="M15 9h.01" />
      <path d="M9 13h.01" />
      <path d="M15 13h.01" />
      <path d="M9 17h6" />
    </svg>
  ),
};

const SOLUTIONS = [
  {
    key: "saas",
    title: "SaaS Products",
    desc: "Add native social posting and scheduling to your SaaS without distracting your team from core product work. One API replaces six integrations.",
  },
  {
    key: "ai",
    title: "AI Content Generation",
    desc: "Close the loop between AI generation and social distribution. Let your AI agents publish across platforms via REST API or our native MCP server.",
  },
  {
    key: "ecommerce",
    title: "E-commerce Platforms",
    desc: "Drive sales with automated product launches, restock alerts, and promo campaigns shared across every social platform your sellers use.",
  },
  {
    key: "scheduler",
    title: "Social Media Schedulers",
    desc: "Build a custom scheduler app on top of one unified API. We handle OAuth, token refresh, media uploads, and platform quirks — you ship the UX.",
  },
  {
    key: "multiAccount",
    title: "Multi-Account Management",
    desc: "Manage dozens or hundreds of social accounts affordably. White-label lets your customers see your brand on OAuth, while you scale on a flat rate.",
  },
  {
    key: "agency",
    title: "Agencies & Creator Tools",
    desc: "Help agencies and creators publish on behalf of their clients without juggling six developer accounts. White-label friendly, audit-log ready.",
  },
];

export default function SolutionsPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />

      {/* NAV */}
      <nav className="sol-nav">
        <div className="sol-nav-inner">
          <Link href="/" className="sol-logo">
            <div className="sol-logo-mark"><ZapIcon /></div>
            <span className="sol-logo-name">UniPost</span>
          </Link>
          <div className="sol-nav-links">
            <Link href="/solutions" className="sol-nav-link active">Solutions</Link>
            <Link href="/tools" className="sol-nav-link">Tools</Link>
            <Link href="/docs" className="sol-nav-link">Docs</Link>
            <Link href="/pricing" className="sol-nav-link">Pricing</Link>
          </div>
          <MarketingNav />
        </div>
      </nav>

      <div className="sol-page">
        {/* HERO */}
        <section className="sol-hero">
          <div className="sol-eyebrow">Solutions</div>
          <h1 className="sol-hero-title">
            One API. Every social platform.<br />
            <em>Built for every use case.</em>
          </h1>
          <p className="sol-hero-sub">
            From SaaS products and AI agents to e-commerce and creator tools — see how teams use
            UniPost to ship social features without the infrastructure overhead.
          </p>
        </section>

        {/* GRID */}
        <section className="sol-grid-section">
          <div className="sol-grid">
            {SOLUTIONS.map((s) => (
              <div key={s.key} className="sol-card">
                <div className="sol-card-icon">{ICONS[s.key]}</div>
                <h3 className="sol-card-title">{s.title}</h3>
                <p className="sol-card-desc">{s.desc}</p>
                <div className="sol-card-soon">More details coming soon</div>
              </div>
            ))}
          </div>
        </section>

        {/* CTA */}
        <section className="sol-cta">
          <div className="sol-cta-inner">
            <div className="sol-cta-glow" />
            <h2 className="sol-cta-title">Don&apos;t see your use case?</h2>
            <p className="sol-cta-sub">
              UniPost works for any product that needs to publish to social platforms.
              Get started for free, or talk to us about your scenario.
            </p>
            <div className="sol-cta-actions">
              <MarketingCTA />
              <Link href="/docs" className="lp-btn lp-btn-outline lp-btn-lg">View Docs →</Link>
            </div>
          </div>
        </section>
      </div>

      <footer className="sol-footer">
        <div className="sol-footer-inner">
          <span>© {new Date().getFullYear()} UniPost</span>
          <span>
            <Link href="/terms">Terms</Link>{" · "}
            <Link href="/privacy">Privacy</Link>
          </span>
        </div>
      </footer>
    </>
  );
}
