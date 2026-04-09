"use client";

import { useState } from "react";
import Link from "next/link";
import { MarketingNav, MarketingCTA, MarketingCTALight } from "@/components/marketing/nav";
import type { PlatformConfig } from "../_config/platforms";
import { ALL_PLATFORMS } from "../_config/platforms";

// ── Platform brand SVG icons (from landing page) ──
const PLATFORM_ICONS: Record<string, React.ReactNode> = {
  bluesky: <svg width="18" height="18" viewBox="0 0 600 530" fill="#0085ff"><path d="M135.7 44.3C202.3 94.8 273.6 197.2 300 249.6c26.4-52.4 97.7-154.8 164.3-205.3C520.4 1.5 588 -22.1 588 68.2c0 18 -10.4 151.2-16.5 172.8-21.2 75-98.6 94.1-167.9 82.6 121.1 20.7 151.8 89.2 85.3 157.8C390.5 584.2 310.2 500 300 481.4c-10.2 18.6-90.5 102.8-188.9 0C44.6 413.8 75.3 345.3 196.4 324.6c-69.3 11.5-146.7-7.6-167.9-82.6C22.4 220.4 12 87.2 12 69.2c0-90.3 67.6-66.7 123.7-24.9z"/></svg>,
  linkedin: <svg width="18" height="18" viewBox="0 0 24 24" fill="#0a66c2"><path d="M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 5.455v6.286zM5.337 7.433a2.062 2.062 0 01-2.063-2.065 2.064 2.064 0 112.063 2.065zm1.782 13.019H3.555V9h3.564v11.452zM22.225 0H1.771C.792 0 0 .774 0 1.729v20.542C0 23.227.792 24 1.771 24h20.451C23.2 24 24 23.227 24 22.271V1.729C24 .774 23.2 0 22.222 0h.003z"/></svg>,
  instagram: <svg width="18" height="18" viewBox="0 0 24 24" fill="url(#pp-ig)"><defs><radialGradient id="pp-ig" cx="30%" cy="107%" r="150%"><stop offset="0%" stopColor="#fdf497"/><stop offset="5%" stopColor="#fdf497"/><stop offset="45%" stopColor="#fd5949"/><stop offset="60%" stopColor="#d6249f"/><stop offset="90%" stopColor="#285AEB"/></radialGradient></defs><path d="M12 2.163c3.204 0 3.584.012 4.85.07 3.252.148 4.771 1.691 4.919 4.919.058 1.265.069 1.645.069 4.849 0 3.205-.012 3.584-.069 4.849-.149 3.225-1.664 4.771-4.919 4.919-1.266.058-1.644.07-4.85.07-3.204 0-3.584-.012-4.849-.07-3.26-.149-4.771-1.699-4.919-4.92-.058-1.265-.07-1.644-.07-4.849 0-3.204.013-3.583.07-4.849.149-3.227 1.664-4.771 4.919-4.919 1.266-.057 1.645-.069 4.849-.069zM12 0C8.741 0 8.333.014 7.053.072 2.695.272.273 2.69.073 7.052.014 8.333 0 8.741 0 12c0 3.259.014 3.668.072 4.948.2 4.358 2.618 6.78 6.98 6.98C8.333 23.986 8.741 24 12 24c3.259 0 3.668-.014 4.948-.072 4.354-.2 6.782-2.618 6.979-6.98.059-1.28.073-1.689.073-4.948 0-3.259-.014-3.667-.072-4.947-.196-4.354-2.617-6.78-6.979-6.98C15.668.014 15.259 0 12 0zm0 5.838a6.162 6.162 0 100 12.324 6.162 6.162 0 000-12.324zM12 16a4 4 0 110-8 4 4 0 010 8zm6.406-11.845a1.44 1.44 0 100 2.881 1.44 1.44 0 000-2.881z"/></svg>,
  threads: <svg width="18" height="18" viewBox="0 0 24 24" fill="#ffffff"><path d="M12.186 24h-.007C5.965 24 2.615 20.483 2.615 14.832V9.168C2.615 3.517 5.965 0 12.186 0h.007c4.486 0 7.457 1.9 8.907 4.581l-3.182 1.822C17.032 4.857 15.1 3.618 12.193 3.618c-3.862 0-5.96 2.587-5.96 5.55v5.664c0 2.963 2.098 5.55 5.96 5.55 2.111 0 3.662-.6 4.608-1.594.757-.793 1.247-1.935 1.352-3.353h-5.96v-3.4h9.652c.073.49.117.99.117 1.528C21.962 20.2 18.791 24 12.186 24z"/></svg>,
  tiktok: <svg width="18" height="18" viewBox="0 0 24 24" fill="#ffffff"><path d="M19.59 6.69a4.83 4.83 0 01-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 01-2.88 2.5 2.89 2.89 0 01-2.89-2.89 2.89 2.89 0 012.89-2.89c.28 0 .54.04.79.1v-3.5a6.37 6.37 0 00-.79-.05A6.34 6.34 0 003.15 15.2a6.34 6.34 0 0010.86 4.48 6.3 6.3 0 001.86-4.48V8.73a8.26 8.26 0 004.84 1.56V6.84a4.85 4.85 0 01-1.12-.15z"/></svg>,
  youtube: <svg width="18" height="18" viewBox="0 0 24 24" fill="#ff0000"><path d="M23.498 6.186a3.016 3.016 0 00-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 00.502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 002.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 002.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"/></svg>,
  twitter: <svg width="18" height="18" viewBox="0 0 24 24" fill="#ffffff"><path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z"/></svg>,
};

// ── Icons ──
function CheckIcon({ color = "currentColor" }: { color?: string }) {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke={color} strokeWidth="2.2" width="14" height="14" style={{ flexShrink: 0 }}>
      <path d="M3 8l4 4 6-7" />
    </svg>
  );
}
function ZapIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" width="14" height="14">
      <path d="M9 2L4 9h4l-1 5 5-7H8l1-5z" />
    </svg>
  );
}

// ── CSS ──
// Reuses lp- variables from the marketing landing page (same font, color, spacing system).
// Adds pp- prefix for platform-page-specific sections matching the reference HTML design.
const CSS = `
@import url('https://fonts.googleapis.com/css2?family=DM+Sans:opsz,wght@9..40,300;9..40,400;9..40,500;9..40,600;9..40,700;9..40,800;9..40,900&family=Fira+Code:wght@400;500&display=swap');
:root{
  --bg:#000;--s1:#0a0a0a;--s2:#111;--s3:#1a1a1a;
  --border:#1a1a1a;--b2:#242424;--b3:#2e2e2e;
  --text:#f0f0f0;--muted:#999;--muted2:#555;
  --accent:#10b981;--adim:#10b98112;
  --blue:#0ea5e9;--blue-dim:#0ea5e912;
  --danger:#ef4444;
  --r:8px;--mono:'Fira Code',monospace;--ui:'DM Sans',system-ui,sans-serif;
  --content-max:960px;--nav-max:1480px;--px:32px;--section-py:64px;
}

/* NAV — same as marketing */
.pp-nav{position:sticky;top:0;z-index:50;width:100%;border-bottom:1px solid var(--border);background:#00000095;backdrop-filter:blur(12px);-webkit-backdrop-filter:blur(12px)}
.pp-nav-inner{max-width:var(--nav-max);margin:0 auto;padding:0 var(--px);height:56px;display:flex;align-items:center;justify-content:space-between}
.pp-logo{display:flex;align-items:center;gap:10px;text-decoration:none}
.pp-logo-mark{width:28px;height:28px;background:var(--accent);border-radius:7px;display:flex;align-items:center;justify-content:center}
.pp-logo-mark svg{width:14px;height:14px;color:#000}
.pp-logo-name{font-size:16px;font-weight:700;letter-spacing:-.4px;color:var(--text)}
.pp-nav-links{display:flex;align-items:center;gap:4px}
.pp-nav-link{padding:6px 14px;font-size:14px;font-weight:500;color:var(--muted);cursor:pointer;border-radius:var(--r);transition:color .1s;text-decoration:none}
.pp-nav-link:hover{color:var(--text)}

/* Buttons — reuse lp- names for MarketingNav compat */
.lp-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--ui);text-decoration:none;white-space:nowrap}
.lp-btn-primary{background:var(--blue);color:#000}
.lp-btn-primary:hover{background:#38bdf8;box-shadow:0 0 24px #0ea5e930}
.lp-btn-ghost{background:transparent;color:var(--muted);border-color:var(--b2)}
.lp-btn-ghost:hover{background:var(--s2);color:var(--text);border-color:var(--b3)}
.lp-btn-outline{background:transparent;color:var(--text);border-color:var(--b2)}
.lp-btn-outline:hover{background:var(--s2);border-color:var(--b3)}
.lp-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}

/* PAGE */
.pp-page{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}

/* HERO */
.pp-hero{padding:72px 0 64px;text-align:center;position:relative}
.pp-plat-badge{display:inline-flex;align-items:center;gap:8px;padding:8px 16px;background:var(--s1);border:1px solid var(--border);border-radius:24px;margin-bottom:28px;font-size:13px;color:var(--muted)}
.pp-plat-icon{width:32px;height:32px;border-radius:8px;display:flex;align-items:center;justify-content:center;font-size:18px}
.pp-hero-title{font-size:56px;font-weight:900;letter-spacing:-1.5px;line-height:1.08;margin-bottom:16px;white-space:pre-line}
.pp-hero-title em{color:var(--accent);font-style:normal}
.pp-hero-sub{font-size:15px;color:var(--muted);max-width:520px;margin:0 auto 32px;line-height:1.75}
.pp-hero-actions{display:flex;align-items:center;justify-content:center;gap:10px;margin-bottom:20px}
.pp-hero-meta{display:flex;align-items:center;justify-content:center;gap:16px;font-size:12px;color:var(--muted2)}
.pp-hero-meta-item{display:flex;align-items:center;gap:5px}
.pp-hero-meta-item svg{width:12px;height:12px;color:var(--accent)}

/* WAITLIST */
.pp-waitlist-badge{display:inline-flex;align-items:center;gap:6px;padding:6px 16px;background:#f59e0b14;border:1px solid #f59e0b30;border-radius:20px;font-size:12.5px;color:#f59e0b;font-weight:600;font-family:var(--mono);margin-bottom:12px}

/* SCREENSHOT PLACEHOLDER */
.pp-screenshot{background:var(--s1);border:1px solid var(--border);border-radius:12px;margin:40px 0;overflow:hidden}
.pp-sp-header{background:var(--s2);border-bottom:1px solid var(--border);padding:10px 16px;display:flex;align-items:center;gap:8px}
.pp-sp-dot{width:10px;height:10px;border-radius:50%}
.pp-sp-body{padding:32px;text-align:center;min-height:200px;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:10px}
.pp-sp-icon{font-size:32px;opacity:.3}
.pp-sp-label{font-size:12px;color:var(--muted2);font-family:var(--mono)}

/* VIDEO PLACEHOLDER */
.pp-video-placeholder{background:var(--s1);border:1px dashed var(--b2);border-radius:10px;padding:28px;text-align:center;display:flex;flex-direction:column;align-items:center;gap:8px}
.pp-vp-icon{width:44px;height:44px;background:var(--s2);border:1px solid var(--b2);border-radius:50%;display:flex;align-items:center;justify-content:center;font-size:18px}
.pp-vp-label{font-size:12px;font-weight:600;color:var(--muted)}
.pp-vp-desc{font-size:11px;color:var(--muted2);max-width:260px;line-height:1.6;text-align:center}

/* SECTIONS */
.pp-section{padding:var(--section-py) 0}
.pp-section-label{font-size:11px;color:var(--accent);text-transform:uppercase;letter-spacing:.12em;font-weight:700;margin-bottom:10px;font-family:var(--mono);text-align:center}
.pp-section-title{font-size:36px;font-weight:800;letter-spacing:-.6px;margin-bottom:10px;line-height:1.1;text-align:center}
.pp-section-sub{font-size:14px;color:var(--muted);max-width:480px;line-height:1.7;margin-bottom:40px;text-align:center;margin-left:auto;margin-right:auto}

/* FEATURE CARDS */
.pp-feat-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:12px}
.pp-feat-card{background:var(--s1);border:1px solid var(--border);border-radius:10px;padding:20px;transition:border-color .15s}
.pp-feat-card:hover{border-color:var(--b2)}
.pp-feat-icon{width:36px;height:36px;background:var(--adim);border:1px solid #10b98118;border-radius:8px;display:flex;align-items:center;justify-content:center;margin-bottom:14px;font-size:16px}
.pp-feat-title{font-size:13.5px;font-weight:600;margin-bottom:6px}
.pp-feat-desc{font-size:12px;color:var(--muted);line-height:1.6}

/* CODE */
.pp-code-wrap{background:var(--s1);border:1px solid var(--border);border-radius:10px;overflow:hidden}
.pp-code-topbar{background:var(--s2);border-bottom:1px solid var(--border);padding:10px 16px;display:flex;align-items:center;gap:8px}
.pp-c-dot{width:10px;height:10px;border-radius:50%}
.pp-c-tabs{display:flex;gap:2px;margin-left:12px}
.pp-c-tab{padding:3px 10px;border-radius:4px;font-size:11.5px;color:var(--muted);cursor:pointer;font-family:var(--mono);border:none;background:none;transition:all .1s}
.pp-c-tab.active{background:var(--s3);color:var(--text)}
.pp-c-tab:hover{color:var(--text)}
.pp-code-body{padding:20px 22px;font-family:var(--mono);font-size:12px;line-height:1.75;color:#888;overflow-x:auto;white-space:pre}
.pp-code-layout{display:grid;grid-template-columns:1fr 1fr;gap:24px;align-items:start}

/* ALTERNATING */
.pp-alt-item{display:grid;grid-template-columns:1fr 1fr;gap:56px;align-items:center;margin-bottom:64px}
.pp-alt-item.reverse{direction:rtl}
.pp-alt-item.reverse>*{direction:ltr}
.pp-alt-num{font-family:var(--mono);font-size:11px;color:var(--muted2);font-weight:600;letter-spacing:.1em;margin-bottom:14px}
.pp-alt-title{font-size:24px;font-weight:700;letter-spacing:-.4px;margin-bottom:10px}
.pp-alt-desc{font-size:13.5px;color:var(--muted);line-height:1.75}

/* BEFORE AFTER */
.pp-ba-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px}
.pp-ba-card{background:var(--s1);border:1px solid var(--border);border-radius:10px;padding:22px}
.pp-ba-title{font-size:14px;font-weight:600;margin-bottom:16px;display:flex;align-items:center;gap:8px}
.pp-ba-list{list-style:none;padding:0;margin:0}
.pp-ba-item{display:flex;align-items:flex-start;gap:8px;font-size:13px;margin-bottom:10px;line-height:1.4}
.pp-ba-x{color:var(--danger);font-weight:700;flex-shrink:0}
.pp-ba-check{color:var(--accent);font-weight:700;flex-shrink:0}
.pp-ba-bad{color:var(--muted)}
.pp-ba-good{color:var(--text)}

/* MODES */
.pp-modes-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px}
.pp-mode-card{background:var(--s1);border:1px solid var(--border);border-radius:10px;padding:24px;transition:border-color .15s}
.pp-mode-badge{display:inline-flex;align-items:center;gap:5px;padding:3px 10px;border-radius:20px;font-size:11px;font-weight:700;font-family:var(--mono);margin-bottom:14px}
.pp-mb-green{background:var(--adim);color:var(--accent);border:1px solid #10b98120}
.pp-mb-blue{background:#3b82f612;color:#60a5fa;border:1px solid #3b82f620}
.pp-mode-title{font-size:18px;font-weight:700;margin-bottom:8px}
.pp-mode-desc{font-size:13px;color:var(--muted);line-height:1.65;margin-bottom:16px}
.pp-mode-feat{display:flex;align-items:flex-start;gap:8px;font-size:12.5px;color:var(--muted);margin-bottom:6px}
.pp-mode-feat svg{width:12px;height:12px;color:var(--accent);flex-shrink:0;margin-top:2px}
.pp-app-pw-card{background:var(--s1);border:1px solid var(--border);border-radius:10px;padding:28px;max-width:600px}

/* METRICS */
.pp-metrics-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:10px;margin-bottom:32px}
.pp-metric-card{background:var(--s1);border:1px solid var(--border);border-radius:8px;padding:14px 16px}
.pp-metric-label{font-size:10px;color:var(--muted);text-transform:uppercase;letter-spacing:.07em;font-weight:600;margin-bottom:6px}
.pp-metric-val{font-family:var(--mono);font-size:20px;font-weight:700;color:var(--accent)}

/* FAQ */
.pp-faq-grid{display:grid;grid-template-columns:1fr 1fr;gap:10px}
.pp-faq-item{background:var(--s1);border:1px solid var(--border);border-radius:9px;padding:18px}
.pp-faq-q{font-size:13px;font-weight:600;margin-bottom:7px}
.pp-faq-a{font-size:12px;color:var(--muted);line-height:1.65}

/* CTA */
.pp-cta-section{padding:0 0 64px}
.pp-cta-inner{background:var(--s1);border:1px solid var(--border);border-radius:14px;padding:56px;text-align:center;position:relative;overflow:hidden}
.pp-cta-glow{position:absolute;width:400px;height:400px;border-radius:50%;background:radial-gradient(circle,#10b98110,transparent 70%);top:50%;left:50%;transform:translate(-50%,-50%);pointer-events:none}
.pp-cta-title{font-size:36px;font-weight:900;letter-spacing:-.8px;margin-bottom:12px;position:relative}
.pp-cta-sub{font-size:14px;color:var(--muted);margin-bottom:28px;position:relative}
.pp-cta-actions{display:flex;align-items:center;justify-content:center;gap:10px;position:relative}

/* ALSO PLATFORMS */
.pp-also{text-align:center;padding:0 0 48px}
.pp-also-label{font-size:12px;color:var(--muted2);margin-bottom:14px;text-transform:uppercase;letter-spacing:.08em;font-weight:600}
.pp-also-chips{display:flex;align-items:center;justify-content:center;gap:8px;flex-wrap:wrap}
.pp-also-chip{display:flex;align-items:center;gap:6px;padding:6px 12px;background:var(--s2);border:1px solid var(--border);border-radius:20px;font-size:12px;color:var(--muted);text-decoration:none;transition:all .15s}
.pp-also-chip:hover{color:var(--text);border-color:var(--b2)}

/* FOOTER */
.pp-footer{border-top:1px solid var(--border);padding:48px 0}
.pp-footer-inner{max-width:var(--content-max);margin:0 auto;padding:0 var(--px);display:flex;align-items:center;justify-content:space-between}
.pp-footer-logo{display:flex;align-items:center;gap:7px}
.pp-footer-mark{width:20px;height:20px;background:var(--accent);border-radius:4px;display:flex;align-items:center;justify-content:center}
.pp-footer-mark svg{width:10px;height:10px;color:#000}
.pp-footer-name{font-size:13px;font-weight:700}
.pp-footer-links{display:flex;gap:16px}
.pp-footer-link{font-size:12px;color:var(--muted);text-decoration:none;cursor:pointer}
.pp-footer-link:hover{color:var(--text)}
.pp-footer-copy{font-size:12px;color:var(--muted2)}

/* RESPONSIVE */
@media(max-width:768px){
  .pp-hero-title{font-size:36px}
  .pp-feat-grid{grid-template-columns:1fr}
  .pp-code-layout{grid-template-columns:1fr}
  .pp-alt-item,.pp-alt-item.reverse{grid-template-columns:1fr;direction:ltr;gap:24px}
  .pp-ba-grid{grid-template-columns:1fr}
  .pp-modes-grid{grid-template-columns:1fr}
  .pp-metrics-grid{grid-template-columns:1fr 1fr}
  .pp-faq-grid{grid-template-columns:1fr}
  .pp-hero-meta{flex-direction:column;gap:8px}
  .pp-footer-inner{flex-direction:column;gap:16px;text-align:center}
  .pp-section-title{font-size:28px}
}
`;

const LANGS = [
  { id: "js", label: "JavaScript" },
  { id: "python", label: "Python" },
  { id: "curl", label: "cURL" },
] as const;

export default function PlatformPage({ cfg }: { cfg: PlatformConfig }) {
  const [activeLang, setActiveLang] = useState<string>("js");
  const others = ALL_PLATFORMS.filter((p) => p.slug !== cfg.slug);
  const isWaitlist = cfg.waitlist && process.env.NEXT_PUBLIC_INSTAGRAM_ENABLED !== "true";

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />

      {/* NAV */}
      <nav className="pp-nav">
        <div className="pp-nav-inner">
          <Link href="/" className="pp-logo">
            <div className="pp-logo-mark"><ZapIcon /></div>
            <span className="pp-logo-name">UniPost</span>
          </Link>
          <div className="pp-nav-links">
            <Link href="/solutions" className="pp-nav-link">Solutions</Link>
            <Link href="/docs" className="pp-nav-link">Docs</Link>
            <Link href="/pricing" className="pp-nav-link">Pricing</Link>
          </div>
          <MarketingNav />
        </div>
      </nav>

      <div className="pp-page">
        {/* ─── HERO ─── */}
        <div className="pp-hero">
          <div className="pp-plat-badge">
            <div className="pp-plat-icon" style={{ background: cfg.brandColor + "15" }}>{PLATFORM_ICONS[cfg.slug] ?? cfg.icon}</div>
            <span>{cfg.name} API</span>
            <span style={{ color: "var(--muted2)" }}>·</span>
            <span style={{ color: "var(--accent)", fontSize: 12, fontFamily: "var(--mono)" }}>7 platforms supported</span>
          </div>

          {isWaitlist && (
            <div className="pp-waitlist-badge">Early Access — Join the Waitlist</div>
          )}

          <h1 className="pp-hero-title">
            {cfg.heroTitle.split("\n").map((line, i) => (
              <span key={i}>
                {i === 0 ? line : <><br /><em>{line}</em></>}
              </span>
            ))}
          </h1>
          <p className="pp-hero-sub">{cfg.heroSub}</p>

          <div className="pp-hero-actions">
            {isWaitlist ? (
              <Link href="/contact" className="lp-btn lp-btn-primary lp-btn-lg">Join Waitlist</Link>
            ) : (
              <MarketingCTA />
            )}
            <Link href={`/docs`} className="lp-btn lp-btn-ghost lp-btn-lg">
              View {cfg.name} Docs →
            </Link>
          </div>

          <div className="pp-hero-meta">
            <div className="pp-hero-meta-item">
              <CheckIcon color="var(--accent)" />
              Free 100 posts/month
            </div>
            <div className="pp-hero-meta-item">
              <CheckIcon color="var(--accent)" />
              No credit card required
            </div>
            <div className="pp-hero-meta-item">
              <CheckIcon color="var(--accent)" />
              Setup in 5 minutes
            </div>
          </div>

          {/* Screenshot placeholder */}
          <div className="pp-screenshot">
            <div className="pp-sp-header">
              <div className="pp-sp-dot" style={{ background: "#ef444440" }} />
              <div className="pp-sp-dot" style={{ background: "#f59e0b40" }} />
              <div className="pp-sp-dot" style={{ background: "#10b98140" }} />
              <span style={{ fontSize: 11, color: "var(--muted2)", marginLeft: 8, fontFamily: "var(--mono)" }}>
                app.unipost.dev — Accounts
              </span>
            </div>
            <div className="pp-sp-body">
              <div className="pp-sp-icon">{cfg.icon}</div>
              <div className="pp-sp-label">SCREENSHOT PLACEHOLDER</div>
            </div>
          </div>
        </div>

        {/* ─── CAPABILITIES ─── */}
        <div className="pp-section">
          <div className="pp-section-label">Capabilities</div>
          <h2 className="pp-section-title">
            Everything you need<br />to build with {cfg.name}
          </h2>
          <p className="pp-section-sub">
            Post any content type, read metrics, and manage accounts — all through a unified API.
          </p>
          <div className="pp-feat-grid">
            {cfg.capabilities.map((c) => (
              <div key={c.title} className="pp-feat-card">
                <div className="pp-feat-icon">{c.icon}</div>
                <div className="pp-feat-title">{c.title}</div>
                <div className="pp-feat-desc">{c.desc}</div>
              </div>
            ))}
          </div>
        </div>

        {/* ─── CODE EXAMPLE ─── */}
        <div className="pp-section" style={{ paddingTop: 0 }}>
          <div className="pp-section-label">Simple by Design</div>
          <h2 className="pp-section-title">
            Post to {cfg.name}<br />in 3 lines of code
          </h2>
          <div className="pp-code-layout">
            <div className="pp-code-wrap">
              <div className="pp-code-topbar">
                <div className="pp-c-dot" style={{ background: "#ef444440" }} />
                <div className="pp-c-dot" style={{ background: "#f59e0b40" }} />
                <div className="pp-c-dot" style={{ background: "#10b98140" }} />
                <div className="pp-c-tabs">
                  {LANGS.map((l) => (
                    <button
                      key={l.id}
                      className={`pp-c-tab ${activeLang === l.id ? "active" : ""}`}
                      onClick={() => setActiveLang(l.id)}
                    >
                      {l.label}
                    </button>
                  ))}
                </div>
              </div>
              <div className="pp-code-body">
                {cfg.codeExample[activeLang as keyof typeof cfg.codeExample]}
              </div>
            </div>

            {/* Video placeholder */}
            <div className="pp-video-placeholder" style={{ minHeight: 320, justifyContent: "center" }}>
              <div className="pp-vp-icon">▶️</div>
              <div className="pp-vp-label">DEMO VIDEO PLACEHOLDER</div>
              <div className="pp-vp-desc">
                Dashboard connect → API post → view on {cfg.name}
              </div>
            </div>
          </div>
        </div>

        {/* ─── PLATFORM FEATURES (ALTERNATING) ─── */}
        <div className="pp-section">
          <div className="pp-section-label">Platform Features</div>
          <h2 className="pp-section-title">
            Built for {cfg.name}&apos;s<br />unique requirements
          </h2>
          <div style={{ marginTop: 40 }}>
            {cfg.alternatingFeatures.map((f, i) => (
              <div key={f.num} className={`pp-alt-item ${i % 2 !== 0 ? "reverse" : ""}`}>
                <div>
                  <div className="pp-alt-num">{f.num}</div>
                  <div className="pp-alt-title">{f.title}</div>
                  <div className="pp-alt-desc">{f.desc}</div>
                </div>
                <div className="pp-screenshot" style={{ margin: 0 }}>
                  <div className="pp-sp-header">
                    <div className="pp-sp-dot" style={{ background: "#ef444440" }} />
                    <div className="pp-sp-dot" style={{ background: "#f59e0b40" }} />
                    <div className="pp-sp-dot" style={{ background: "#10b98140" }} />
                  </div>
                  <div className="pp-sp-body" style={{ minHeight: 140 }}>
                    <div className="pp-sp-icon">{f.placeholderIcon}</div>
                    <div className="pp-sp-label">{f.placeholderLabel}</div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* ─── WHY NOT DIRECT API ─── */}
        <div className="pp-section" style={{ paddingTop: 0 }}>
          <div className="pp-section-label">Why UniPost</div>
          <h2 className="pp-section-title">Why not use {cfg.name} API directly?</h2>
          <div className="pp-ba-grid" style={{ marginTop: 28 }}>
            <div className="pp-ba-card">
              <div className="pp-ba-title">Without UniPost</div>
              <ul className="pp-ba-list">
                {cfg.whyNot.without.map((item) => (
                  <li key={item} className="pp-ba-item">
                    <span className="pp-ba-x">&times;</span>
                    <span className="pp-ba-bad">{item}</span>
                  </li>
                ))}
              </ul>
            </div>
            <div className="pp-ba-card" style={{ borderColor: "var(--b2)" }}>
              <div className="pp-ba-title">With UniPost</div>
              <ul className="pp-ba-list">
                {cfg.whyNot.with.map((item) => (
                  <li key={item} className="pp-ba-item">
                    <span className="pp-ba-check">&#10003;</span>
                    <span className="pp-ba-good">{item}</span>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </div>

        {/* ─── MODES ─── */}
        <div className="pp-section" style={{ paddingTop: 0 }}>
          {cfg.modes.type === "dual" ? (
            <>
              <div className="pp-section-label">Two Modes</div>
              <h2 className="pp-section-title">Two ways to connect {cfg.name}</h2>
              <div className="pp-modes-grid" style={{ marginTop: 28 }}>
                <div className="pp-mode-card">
                  <div className="pp-mode-badge pp-mb-green">⚡ Quickstart Mode</div>
                  <div className="pp-mode-title">Start in 5 minutes</div>
                  <div className="pp-mode-desc">{cfg.modes.quickstartDesc}</div>
                  {cfg.modes.quickstartFeats.map((f) => (
                    <div key={f} className="pp-mode-feat">
                      <CheckIcon color="var(--accent)" />{f}
                    </div>
                  ))}
                </div>
                <div className="pp-mode-card">
                  <div className="pp-mode-badge pp-mb-blue">🔑 Native Mode</div>
                  <div className="pp-mode-title">Your brand, your credentials</div>
                  <div className="pp-mode-desc">{cfg.modes.nativeDesc}</div>
                  {cfg.modes.nativeFeats.map((f) => (
                    <div key={f} className="pp-mode-feat">
                      <CheckIcon color="var(--accent)" />{f}
                    </div>
                  ))}
                </div>
              </div>
            </>
          ) : (
            <>
              <div className="pp-section-label">Connection</div>
              <h2 className="pp-section-title">Connect {cfg.name} with an App Password</h2>
              <div className="pp-app-pw-card" style={{ marginTop: 28 }}>
                <div className="pp-mode-desc">{cfg.modes.desc}</div>
                {cfg.modes.features.map((f) => (
                  <div key={f} className="pp-mode-feat">
                    <CheckIcon color="var(--accent)" />{f}
                  </div>
                ))}
              </div>
            </>
          )}
        </div>

        {/* ─── ANALYTICS ─── */}
        <div className="pp-section" style={{ paddingTop: 0 }}>
          <div className="pp-section-label">Analytics</div>
          <h2 className="pp-section-title">Track {cfg.name} performance</h2>
          <p className="pp-section-sub">
            Get unified metrics from {cfg.name} and every connected platform in one API response.
          </p>
          <div className="pp-metrics-grid">
            {cfg.metrics.map((m) => (
              <div key={m.label} className="pp-metric-card">
                <div className="pp-metric-label">{m.label}</div>
                <div className="pp-metric-val">{m.sampleValue}</div>
              </div>
            ))}
          </div>
          <div className="pp-screenshot">
            <div className="pp-sp-header">
              <div className="pp-sp-dot" style={{ background: "#ef444440" }} />
              <div className="pp-sp-dot" style={{ background: "#f59e0b40" }} />
              <div className="pp-sp-dot" style={{ background: "#10b98140" }} />
              <span style={{ fontSize: 11, color: "var(--muted2)", marginLeft: 8, fontFamily: "var(--mono)" }}>
                app.unipost.dev — Analytics
              </span>
            </div>
            <div className="pp-sp-body">
              <div className="pp-sp-icon">📊</div>
              <div className="pp-sp-label">ANALYTICS SCREENSHOT PLACEHOLDER</div>
            </div>
          </div>
        </div>

        {/* ─── FAQ ─── */}
        <div className="pp-section" style={{ paddingTop: 0 }}>
          <div className="pp-section-label">FAQ</div>
          <h2 className="pp-section-title" style={{ marginBottom: 28 }}>Common questions</h2>
          <div className="pp-faq-grid">
            {cfg.faq.map((f) => (
              <div key={f.q} className="pp-faq-item">
                <div className="pp-faq-q">{f.q}</div>
                <div className="pp-faq-a">{f.a}</div>
              </div>
            ))}
          </div>
        </div>

        {/* ─── ALSO PLATFORMS ─── */}
        <div className="pp-also">
          <div className="pp-also-label">Also post to these platforms</div>
          <div className="pp-also-chips">
            {others.map((p) => (
              <Link key={p.slug} href={`/${p.slug}-api`} className="pp-also-chip">
                <span style={{ display: "flex", alignItems: "center" }}>{PLATFORM_ICONS[p.slug] ?? p.icon}</span> {p.name}
              </Link>
            ))}
          </div>
        </div>

        {/* ─── CTA ─── */}
        <div className="pp-cta-section">
          <div className="pp-cta-inner">
            <div className="pp-cta-glow" />
            <h2 className="pp-cta-title">Start posting to {cfg.name} today</h2>
            <p className="pp-cta-sub">Free plan includes 100 posts/month. No credit card required.</p>
            <div className="pp-cta-actions">
              {isWaitlist ? (
                <Link href="/contact" className="lp-btn lp-btn-primary lp-btn-lg">Join Waitlist</Link>
              ) : (
                <MarketingCTA />
              )}
              <Link href={`/docs`} className="lp-btn lp-btn-ghost lp-btn-lg">
                View {cfg.name} Docs →
              </Link>
            </div>
          </div>
        </div>
      </div>

      {/* FOOTER */}
      <footer className="pp-footer">
        <div className="pp-footer-inner">
          <div className="pp-footer-logo">
            <div className="pp-footer-mark"><ZapIcon /></div>
            <span className="pp-footer-name">UniPost</span>
          </div>
          <div className="pp-footer-links">
            <Link href="/docs" className="pp-footer-link">Docs</Link>
            <Link href="/pricing" className="pp-footer-link">Pricing</Link>
            <Link href="/privacy" className="pp-footer-link">Privacy</Link>
            <Link href="/terms" className="pp-footer-link">Terms</Link>
          </div>
          <div className="pp-footer-copy">&copy; 2026 UniPost</div>
        </div>
      </footer>
    </>
  );
}
