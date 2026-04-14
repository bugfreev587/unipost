import type { Metadata } from "next";
import type { ReactNode } from "react";
import Link from "next/link";
import { UniPostLogo } from "@/components/brand/unipost-logo";
import { MarketingNav, MarketingCTA, MarketingCTALight } from "@/components/marketing/nav";
import { LandingAttribution } from "@/components/marketing/landing-attribution";
import { LandingCodeTabs } from "@/components/marketing/landing-code-tabs";
import { LandingHeroRotation } from "@/components/marketing/landing-hero-rotation";

export const metadata: Metadata = {
  title: "UniPost | Unified Social Media API for Developers",
  description:
    "Onboard user accounts, validate drafts, publish per-platform content, and track analytics across X, Bluesky, LinkedIn, Instagram, Threads, TikTok, and YouTube.",
  alternates: {
    canonical: "https://unipost.dev/",
  },
};

// ── Data ──
const PLATFORM_ICONS: Record<string, ReactNode> = {
  X: <svg width="18" height="18" viewBox="0 0 24 24" fill="#ffffff"><path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z"/></svg>,
  Bluesky: <svg width="18" height="18" viewBox="0 0 600 530" fill="#0085ff"><path d="M135.7 44.3C202.3 94.8 273.6 197.2 300 249.6c26.4-52.4 97.7-154.8 164.3-205.3C520.4 1.5 588 -22.1 588 68.2c0 18 -10.4 151.2-16.5 172.8-21.2 75-98.6 94.1-167.9 82.6 121.1 20.7 151.8 89.2 85.3 157.8C390.5 584.2 310.2 500 300 481.4c-10.2 18.6-90.5 102.8-188.9 0C44.6 413.8 75.3 345.3 196.4 324.6c-69.3 11.5-146.7-7.6-167.9-82.6C22.4 220.4 12 87.2 12 69.2c0-90.3 67.6-66.7 123.7-24.9z"/></svg>,
  LinkedIn: <svg width="18" height="18" viewBox="0 0 24 24" fill="#0a66c2"><path d="M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 5.455v6.286zM5.337 7.433a2.062 2.062 0 01-2.063-2.065 2.064 2.064 0 112.063 2.065zm1.782 13.019H3.555V9h3.564v11.452zM22.225 0H1.771C.792 0 0 .774 0 1.729v20.542C0 23.227.792 24 1.771 24h20.451C23.2 24 24 23.227 24 22.271V1.729C24 .774 23.2 0 22.222 0h.003z"/></svg>,
  Instagram: <svg width="18" height="18" viewBox="0 0 24 24" fill="url(#ig)"><defs><radialGradient id="ig" cx="30%" cy="107%" r="150%"><stop offset="0%" stopColor="#fdf497"/><stop offset="5%" stopColor="#fdf497"/><stop offset="45%" stopColor="#fd5949"/><stop offset="60%" stopColor="#d6249f"/><stop offset="90%" stopColor="#285AEB"/></radialGradient></defs><path d="M12 2.163c3.204 0 3.584.012 4.85.07 3.252.148 4.771 1.691 4.919 4.919.058 1.265.069 1.645.069 4.849 0 3.205-.012 3.584-.069 4.849-.149 3.225-1.664 4.771-4.919 4.919-1.266.058-1.644.07-4.85.07-3.204 0-3.584-.012-4.849-.07-3.26-.149-4.771-1.699-4.919-4.92-.058-1.265-.07-1.644-.07-4.849 0-3.204.013-3.583.07-4.849.149-3.227 1.664-4.771 4.919-4.919 1.266-.057 1.645-.069 4.849-.069zM12 0C8.741 0 8.333.014 7.053.072 2.695.272.273 2.69.073 7.052.014 8.333 0 8.741 0 12c0 3.259.014 3.668.072 4.948.2 4.358 2.618 6.78 6.98 6.98C8.333 23.986 8.741 24 12 24c3.259 0 3.668-.014 4.948-.072 4.354-.2 6.782-2.618 6.979-6.98.059-1.28.073-1.689.073-4.948 0-3.259-.014-3.667-.072-4.947-.196-4.354-2.617-6.78-6.979-6.98C15.668.014 15.259 0 12 0zm0 5.838a6.162 6.162 0 100 12.324 6.162 6.162 0 000-12.324zM12 16a4 4 0 110-8 4 4 0 010 8zm6.406-11.845a1.44 1.44 0 100 2.881 1.44 1.44 0 000-2.881z"/></svg>,
  Threads: <svg width="18" height="18" viewBox="0 0 192 192" fill="#ffffff"><path d="M141.537 88.988a66.667 66.667 0 0 0-2.518-1.143c-1.482-27.307-16.403-42.94-41.457-43.1h-.34c-14.986 0-27.449 6.396-35.12 18.036l13.779 9.452c5.73-8.695 14.724-10.548 21.348-10.548h.229c8.249.053 14.474 2.452 18.503 7.129 2.932 3.405 4.893 8.111 5.864 14.05-7.314-1.243-15.224-1.626-23.68-1.14-23.82 1.371-39.134 15.326-38.092 34.7.528 9.818 5.235 18.28 13.256 23.808 6.768 4.666 15.471 6.98 24.49 6.52 11.918-.607 21.27-5.003 27.79-13.066 4.947-6.116 8.1-13.908 9.532-23.619 5.708 3.45 9.953 8.063 12.37 13.676 4.106 9.533 4.349 25.194-7.865 37.315-10.724 10.64-23.618 15.254-38.399 15.358-16.388-.115-28.796-5.382-36.877-15.66-7.515-9.56-11.416-23.12-11.594-40.322.178-17.202 4.079-30.762 11.594-40.322 8.081-10.278 20.489-15.545 36.877-15.66 16.506.116 29.148 5.42 37.567 15.76 4.108 5.048 7.21 11.467 9.312 19.023l14.854-3.982c-2.605-9.463-6.641-17.573-12.159-24.356C152.088 14.14 136.308 7.353 116.379 7.2h-.069c-19.874.142-35.468 6.947-46.333 20.25C60.4 39.452 55.545 55.77 55.33 75.94l-.002.162.002.16c.215 20.17 5.07 36.488 14.645 48.49 10.865 13.303 26.459 20.108 46.333 20.25h.069c18.134-.119 33.577-5.86 45.916-17.068 16.456-14.938 17.617-36.986 12.28-49.39-3.835-8.908-11.151-16.063-21.036-20.544zm-36.844 51.014c-9.985.508-20.361-3.928-21.025-13.278-.477-6.732 4.746-14.243 24.298-15.368 2.132-.123 4.22-.183 6.263-.183 6.26 0 12.12.616 17.39 1.812-1.98 22.459-14.948 26.513-26.926 27.017z"/></svg>,
  TikTok: <svg width="18" height="18" viewBox="0 0 24 24" fill="#ffffff"><path d="M19.59 6.69a4.83 4.83 0 01-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 01-2.88 2.5 2.89 2.89 0 01-2.89-2.89 2.89 2.89 0 012.89-2.89c.28 0 .54.04.79.1v-3.5a6.37 6.37 0 00-.79-.05A6.34 6.34 0 003.15 15.2a6.34 6.34 0 0010.86 4.48 6.3 6.3 0 001.86-4.48V8.73a8.26 8.26 0 004.84 1.56V6.84a4.85 4.85 0 01-1.12-.15z"/></svg>,
  YouTube: <svg width="18" height="18" viewBox="0 0 24 24" fill="#ff0000"><path d="M23.498 6.186a3.016 3.016 0 00-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 00.502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 002.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 002.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"/></svg>,
};

const PLATFORMS = [
  { name: "X", slug: "twitter" }, { name: "Bluesky", slug: "bluesky" }, { name: "LinkedIn", slug: "linkedin" }, { name: "Instagram", slug: "instagram" },
  { name: "Threads", slug: "threads" }, { name: "TikTok", slug: "tiktok" }, { name: "YouTube", slug: "youtube" },
];
const FEATURES = [
  { number: "01", title: "Connect end users, not just your own accounts", desc: "UniPost is built for products that onboard customer social accounts. Create branded Connect flows, map accounts to your own external_user_id, and keep the OAuth surface out of your app.", code: `POST /v1/connect/sessions\n{\n  "platform": "linkedin",\n  "external_user_id": "user_123",\n  "return_url": "https://app.example.com/settings"\n}` },
  { number: "02", title: "Validate, preview, then publish", desc: "Catch bad payloads before publish time, generate read-only preview links for review, and ship with more confidence when you are posting across multiple platforms and users.", code: `POST /v1/social-posts/validate\nPOST /v1/drafts\nGET  /v1/public/drafts/{id}?token=...` },
  { number: "03", title: "AI and automation ready by default", desc: "Per-platform captions, idempotency keys, MCP support, and bulk publish are already in the product. UniPost fits AI agents and workflow systems without forcing them into a single-caption model.", code: `{\n  "platform_posts": [\n    { "account_id": "sa_x", "caption": "short version" },\n    { "account_id": "sa_li", "caption": "longer LinkedIn version" }\n  ],\n  "idempotency_key": "launch-001"\n}` },
];
const CAPABILITIES = [
  {
    title: "Multi-tenant Connect",
    desc: "Create a public Connect link for each end user, store your own external_user_id, and let UniPost handle the OAuth exchange and token refresh lifecycle.",
    points: ["Public connect sessions", "external_user_id mapping", "Hosted OAuth flow"],
  },
  {
    title: "Draft validation + preview",
    desc: "Validate posts before publish, generate shareable preview links, and catch character-limit or thread-shape issues before your users hit send.",
    points: ["Preflight validation", "Read-only preview links", "Per-platform counters"],
  },
  {
    title: "Branded white-label onboarding",
    desc: "Paid plans can inject customer branding into the hosted Connect surface so your product owns the onboarding experience instead of handing trust to a generic tool.",
    points: ["Logo + display name", "Primary brand color", "Your app in the flow"],
  },
  {
    title: "Operational analytics",
    desc: "Track what was published, what failed, which accounts need attention, and how usage trends over time instead of debugging platform behavior blind.",
    points: ["Analytics rollups", "Account health", "Usage warnings"],
  },
];
const PROOF_STEPS = [
  {
    step: "01",
    title: "Create a Connect session",
    body: "Your app starts a hosted onboarding flow for a specific end user and platform.",
  },
  {
    step: "02",
    title: "Preview or validate content",
    body: "Generate per-platform drafts, validate the payload, and share a preview link before publishing.",
  },
  {
    step: "03",
    title: "Publish and monitor",
    body: "Send platform_posts, receive webhooks, and monitor analytics, usage, and account health in one place.",
  },
];
const MODES = [
  { badge: "Quickstart Mode", badgeColor: "#10b981", title: "Start posting in minutes", desc: "Use UniPost's developer credentials. No platform approval process, no waiting.", features: ["Instant access to all 7 platforms", "No developer approval needed", "OAuth shows 'UniPost' branding", "Available on all plans including Free"], ctaVariant: "ghost" },
  { badge: "White-label", badgeColor: "#3b82f6", title: "Your brand, your credentials", desc: "Bring your own platform credentials. Users see your app name during OAuth.", features: ["OAuth shows your app name", "Complete credential ownership", "Professional user experience", "Available on all paid plans"], ctaVariant: "primary" },
];
const FAQS = [
  { q: "Why UniPost over direct platform APIs?", a: "We handle OAuth, token refresh, media processing, and platform-specific quirks — reducing integration time from weeks to hours." },
  { q: "What counts as a post?", a: "One successful publish to a single social account. Posting to 3 platforms counts as 3 posts. Failed posts are never counted." },
  { q: "What's the difference between Quickstart and White-label?", a: "Quickstart uses UniPost's credentials so you start immediately. White-label lets you brand the hosted Connect experience so your end users see your product during onboarding." },
  { q: "Do I need to handle OAuth flows?", a: "No. UniPost handles the entire OAuth flow. Your users connect once through our hosted flow, and you get a simple account_id to use in API calls." },
];

// ── Icons ──
function CheckIcon() { return <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2.2" width="14" height="14" style={{ flexShrink: 0 }}><path d="M3 8l4 4 6-7" /></svg>; }

// ── Styles ──
const CSS = `:root{--bg:#000;--s1:#0a0a0a;--s2:#111;--s3:#1a1a1a;--border:#1a1a1a;--b2:#242424;--b3:#2e2e2e;--text:#fff;--muted:#b0b0b0;--muted2:#777;--accent:#10b981;--adim:#10b98112;--blue:#0ea5e9;--blue-dim:#0ea5e912;--r:8px;--mono:var(--font-fira-code),monospace;--ui:var(--font-dm-sans),system-ui,sans-serif;--nav-max:1480px;--content-max:1320px;--text-max:720px;--px:32px;--section-py:112px}*{box-sizing:border-box;margin:0;padding:0}body{background:var(--bg);color:var(--text);font-family:var(--ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}.lp-nav{position:sticky;top:0;z-index:50;width:100%;border-bottom:1px solid var(--border);background:#00000095;backdrop-filter:blur(12px);-webkit-backdrop-filter:blur(12px)}.lp-nav-inner{max-width:var(--nav-max);margin:0 auto;padding:0 var(--px);height:56px;display:flex;align-items:center;justify-content:space-between}.lp-logo{display:flex;align-items:center;gap:10px;text-decoration:none}.lp-logo-mark{width:28px;height:28px;background:var(--accent);border-radius:7px;display:flex;align-items:center;justify-content:center}.lp-logo-mark svg{width:14px;height:14px;color:#000}.lp-logo-name{font-size:16px;font-weight:700;letter-spacing:-.4px;color:var(--text)}.lp-nav-links{display:flex;align-items:center;gap:4px}.lp-nav-link{padding:6px 14px;font-size:14px;font-weight:500;color:var(--muted);cursor:pointer;border-radius:var(--r);transition:color .1s;text-decoration:none}.lp-nav-link:hover{color:var(--text)}.lp-nav-dropdown{position:relative}.lp-nav-dropdown-trigger{display:inline-flex;align-items:center;background:none;border:none;font-family:var(--ui)}.lp-nav-dropdown-menu{position:absolute;top:100%;right:0;margin-top:8px;min-width:260px;background:var(--s1);border:1px solid var(--b2);border-radius:10px;padding:6px;opacity:0;visibility:hidden;transform:translateY(-4px);transition:all .15s ease;z-index:100;box-shadow:0 8px 32px #00000060}.lp-nav-dropdown:hover .lp-nav-dropdown-menu{opacity:1;visibility:visible;transform:translateY(0)}.lp-nav-dropdown-item{display:flex;align-items:flex-start;gap:12px;padding:10px 12px;border-radius:7px;text-decoration:none;color:var(--text);transition:background .1s}.lp-nav-dropdown-item:hover{background:var(--s2)}.lp-nav-dropdown-icon{width:32px;height:32px;background:var(--s2);border:1px solid var(--b2);border-radius:7px;display:flex;align-items:center;justify-content:center;flex-shrink:0;color:var(--accent)}.lp-nav-dropdown-item:hover .lp-nav-dropdown-icon{background:var(--s3)}.lp-nav-dropdown-label{font-size:13.5px;font-weight:600;margin-bottom:2px}.lp-nav-dropdown-desc{font-size:12px;color:var(--muted);line-height:1.4}.lp-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--ui);text-decoration:none;white-space:nowrap}.lp-btn-primary{background:var(--blue);color:#000}.lp-btn-primary:hover{background:#38bdf8;box-shadow:0 0 24px #0ea5e930}.lp-btn-ghost{background:transparent;color:var(--muted);border-color:var(--b2)}.lp-btn-ghost:hover{background:var(--s2);color:var(--text);border-color:var(--b3)}.lp-btn-outline{background:transparent;color:var(--text);border-color:var(--b2)}.lp-btn-outline:hover{background:var(--s2);border-color:var(--b3)}.lp-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}.lp-btn-hero-primary{padding:12px 24px;border-radius:999px;background:#22c55e;color:#041108;border-color:transparent;font-size:15px;font-weight:700;box-shadow:0 14px 30px rgba(34,197,94,.22)}.lp-btn-hero-primary:hover{background:#4ade80;box-shadow:0 18px 34px rgba(34,197,94,.28)}.lp-btn-hero-outline{padding:12px 24px;border-radius:999px;background:transparent;color:var(--text);border-color:rgba(255,255,255,.08);font-size:15px;font-weight:600}.lp-btn-hero-outline:hover{background:rgba(255,255,255,.04);border-color:rgba(255,255,255,.12)}.lp-btn-hero-lg{min-height:48px}.lp-btn svg{width:14px;height:14px}.lp-page{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}.lp-hero{padding:var(--section-py) 0;text-align:center;display:flex;flex-direction:column;align-items:center}.lp-hero-badge{display:inline-flex;align-items:center;gap:7px;padding:5px 14px;border-radius:20px;background:var(--adim);border:1px solid #10b98120;font-size:12.5px;color:var(--accent);font-weight:600;margin-bottom:32px;font-family:var(--mono)}.lp-hero-badge-dot{width:6px;height:6px;border-radius:50%;background:var(--accent);animation:lp-pulse 2s infinite}@keyframes lp-pulse{0%,100%{opacity:1}50%{opacity:.3}}.lp-hero-title{font-size:76px;font-weight:900;letter-spacing:-2.5px;line-height:1;color:var(--text);margin-bottom:24px;max-width:900px;text-align:center}.lp-hero-title em{color:var(--accent);font-style:normal}.lp-hero-rotate-wrap{font-size:60px;font-weight:800;letter-spacing:-2px;line-height:1;margin-bottom:36px;text-align:center;height:70px;width:100%;max-width:var(--text-max);display:flex;align-items:center;justify-content:center;overflow:hidden;position:relative}.lp-hero-rotate-text{position:absolute;display:inline-block;transition:transform .5s cubic-bezier(.4,0,.2,1),opacity .5s cubic-bezier(.4,0,.2,1);will-change:transform,opacity;white-space:nowrap}.lp-hero-rotate-text.visible{transform:translateY(0);opacity:1}.lp-hero-rotate-text.exit{transform:translateY(-40px);opacity:0}.lp-hero-rotate-text.enter{transform:translateY(40px);opacity:0;transition:none}.lp-hero-sub{font-size:17px;color:#bbb;max-width:var(--text-max);line-height:1.75;margin-bottom:44px;text-align:center}.lp-hero-actions{display:flex;align-items:center;gap:12px;margin-bottom:28px;justify-content:center}.lp-hero-meta{font-size:13px;color:var(--muted2);display:flex;align-items:center;gap:16px;justify-content:center}.lp-hero-meta-item{display:flex;align-items:center;gap:6px}.lp-hero-meta-item svg{width:13px;height:13px;color:var(--accent)}.lp-platforms{padding:0 0 var(--section-py)}.lp-plat-label{font-size:11.5px;color:#777;text-transform:uppercase;letter-spacing:.12em;font-weight:700;margin-bottom:24px;font-family:var(--mono);text-align:center}.lp-plat-row{display:flex;align-items:center;gap:12px;flex-wrap:wrap;justify-content:center}.lp-plat-chip{display:flex;align-items:center;gap:9px;padding:9px 18px;background:var(--s1);border:1px solid var(--b2);border-radius:24px;font-size:14px;font-weight:500;color:#ccc;transition:all .15s;cursor:default}.lp-plat-chip:hover{border-color:var(--b2);color:var(--text);background:var(--s2)}.lp-plat-icon{display:flex;align-items:center;justify-content:center;width:18px;height:18px;flex-shrink:0}.lp-code-section{padding:0 0 var(--section-py)}.lp-section-eyebrow{font-size:11.5px;color:var(--accent);text-transform:uppercase;letter-spacing:.12em;font-weight:700;margin-bottom:12px;font-family:var(--mono);text-align:center}.lp-section-title{font-size:44px;font-weight:800;letter-spacing:-1px;margin-bottom:12px;line-height:1.1;text-align:center}.lp-section-sub{font-size:16px;color:#bbb;max-width:var(--text-max);line-height:1.7;margin-bottom:48px;text-align:center;margin-left:auto;margin-right:auto}.lp-integ-grid{display:grid;grid-template-columns:1fr 1.3fr;gap:48px;align-items:start}.lp-integ-left{padding-top:16px}.lp-integ-title{font-size:32px;font-weight:800;letter-spacing:-.6px;line-height:1.2;color:var(--text);margin-bottom:40px}.lp-integ-cards{display:flex;flex-direction:column;gap:0}.lp-integ-card{display:flex;gap:14px;align-items:flex-start;padding:16px 0}.lp-integ-card-icon{width:36px;height:36px;border-radius:8px;background:var(--s2);border:1px solid var(--b2);display:flex;align-items:center;justify-content:center;flex-shrink:0;color:var(--accent)}.lp-integ-card-title{font-size:15px;font-weight:700;color:var(--text);margin-bottom:4px}.lp-integ-card-desc{font-size:13.5px;color:#bbb;line-height:1.5;margin-bottom:6px}.lp-integ-card-link{font-size:12.5px;color:var(--accent);text-decoration:none;font-weight:600;font-family:var(--mono)}.lp-integ-card-link:hover{text-decoration:underline}.lp-integ-card-divider{height:1px;background:var(--border);margin:4px 0}.lp-integ-right{}.lp-code-tabs-bar{display:flex;gap:2px;margin-bottom:0;background:var(--s2);border:1px solid var(--border);border-bottom:none;border-radius:10px 10px 0 0;padding:10px 16px}.lp-code-tab{padding:5px 14px;border-radius:6px;font-size:12.5px;font-weight:500;color:var(--muted);cursor:pointer;font-family:var(--mono);transition:all .1s;border:1px solid transparent;background:none}.lp-code-tab:hover{color:var(--text)}.lp-code-tab.active{background:var(--s3);color:var(--text);border-color:var(--b2)}.lp-editor{background:#1e1e2e;border:1px solid var(--border);border-top:1px solid var(--border);border-radius:0 0 10px 10px;overflow:hidden}.lp-editor-code{padding:20px 0;margin:0;font-family:var(--mono);font-size:13px;line-height:1.8;overflow-x:auto}.lp-editor-line{display:flex;padding:0 20px 0 0}.lp-editor-line:hover{background:#ffffff06}.lp-editor-ln{width:44px;text-align:right;padding-right:16px;color:#555;user-select:none;flex-shrink:0;font-size:12px}.lp-editor-text{color:#cdd6f4;white-space:pre}.lp-features{padding:0 0 var(--section-py)}.lp-feat-list{display:flex;flex-direction:column;gap:80px}.lp-feat-item{display:grid;grid-template-columns:1fr 1fr;gap:64px;align-items:center}.lp-feat-item.reverse{direction:rtl}.lp-feat-item.reverse>*{direction:ltr}.lp-feat-number{font-family:var(--mono);font-size:11px;color:var(--muted2);font-weight:600;letter-spacing:.1em;margin-bottom:16px}.lp-feat-title{font-size:32px;font-weight:800;letter-spacing:-.6px;line-height:1.15;margin-bottom:16px}.lp-feat-desc{font-size:15px;color:#bbb;line-height:1.75}.lp-feat-code{background:var(--s1);border:1px solid var(--border);border-radius:10px;padding:24px 28px;font-family:var(--mono);font-size:13px;line-height:1.7;color:#b0b0b0;white-space:pre;overflow-x:auto}.lp-stats{padding:0 0 var(--section-py)}.lp-stats-inner{border:1px solid var(--border);border-radius:14px;display:grid;grid-template-columns:repeat(4,1fr);overflow:hidden}.lp-stat{padding:40px 36px;border-right:1px solid var(--border)}.lp-stat:last-child{border-right:none}.lp-stat-num{font-family:var(--mono);font-size:40px;font-weight:700;color:var(--accent);letter-spacing:-1px;margin-bottom:8px}.lp-stat-label{font-size:14px;color:#bbb;line-height:1.5}.lp-modes{padding:0 0 var(--section-py)}.lp-modes-grid{display:grid;grid-template-columns:1fr 1fr;gap:16px}.lp-mode-card{background:var(--s1);border:1px solid var(--b2);border-radius:16px;padding:40px;transition:all .25s;position:relative;overflow:hidden}.lp-mode-card::before{content:'';position:absolute;top:0;left:0;right:0;height:3px;border-radius:16px 16px 0 0}.lp-mode-card.mode-quickstart::before{background:linear-gradient(90deg,#10b981,#34d399)}.lp-mode-card.mode-native::before{background:linear-gradient(90deg,#3b82f6,#60a5fa)}.lp-mode-card:hover{border-color:#333;transform:translateY(-2px);box-shadow:0 8px 32px #00000040}.lp-mode-card.mode-quickstart:hover{box-shadow:0 8px 32px #10b98110}.lp-mode-card.mode-native:hover{box-shadow:0 8px 32px #3b82f610}.lp-mode-badge{display:inline-flex;align-items:center;gap:7px;padding:6px 16px;border-radius:8px;font-size:13px;font-weight:700;font-family:var(--mono);margin-bottom:24px;letter-spacing:.02em}.lp-mode-icon{width:28px;height:28px;border-radius:7px;display:flex;align-items:center;justify-content:center;flex-shrink:0}.lp-mode-title{font-size:26px;font-weight:800;letter-spacing:-.5px;margin-bottom:12px;line-height:1.2}.lp-mode-desc{font-size:14.5px;color:#bbb;line-height:1.7;margin-bottom:28px}.lp-mode-feats{list-style:none;margin-bottom:32px}.lp-mode-feat{display:flex;align-items:flex-start;gap:11px;font-size:14px;color:#ccc;margin-bottom:11px}.lp-mode-feat svg{width:14px;height:14px;color:var(--accent);flex-shrink:0;margin-top:3px}.lp-faq-band{background:#0c0c0c;border-top:1px solid #161616;padding:var(--section-py) 0}.lp-faq-band-inner{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}.lp-faq-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.lp-faq-item{background:var(--s1);border:1px solid var(--border);border-radius:10px;padding:24px 26px;transition:border-color .15s}.lp-faq-item:hover{border-color:var(--b2)}.lp-faq-q{font-size:15px;font-weight:600;margin-bottom:10px}.lp-faq-a{font-size:13.5px;color:#bbb;line-height:1.7}.lp-cta-band{background:#080808;padding:var(--section-py) 0}.lp-cta-band-inner{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}.lp-cta-inner{background:#0f0f0f;border:1px solid #1a1a1a;border-radius:16px;padding:80px 64px;text-align:center;position:relative;overflow:hidden}.lp-cta-glow{position:absolute;width:400px;height:400px;border-radius:50%;background:radial-gradient(circle,#10b98110,transparent 70%);top:50%;left:50%;transform:translate(-50%,-50%);pointer-events:none}.lp-cta-title{font-size:52px;font-weight:900;letter-spacing:-1.5px;margin-bottom:16px;position:relative}.lp-cta-sub{font-size:16px;color:#bbb;margin-bottom:40px;position:relative}.lp-cta-actions{display:flex;align-items:center;justify-content:center;gap:12px;position:relative}.lp-footer{width:100%;border-top:1px solid var(--border);padding:48px 0}.lp-footer-inner{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}.lp-footer-top{display:grid;grid-template-columns:2fr 1fr 1fr 1fr;gap:48px;margin-bottom:48px}.lp-footer-logo{display:flex;align-items:center;gap:9px;margin-bottom:16px}.lp-footer-mark{width:26px;height:26px;background:var(--accent);border-radius:6px;display:flex;align-items:center;justify-content:center}.lp-footer-mark svg{width:13px;height:13px;color:#000}.lp-footer-name{font-size:15px;font-weight:700;color:var(--text)}.lp-footer-tagline{font-size:13px;color:#bbb;line-height:1.65;max-width:260px}.lp-footer-col-title{font-size:12px;font-weight:700;text-transform:uppercase;letter-spacing:.08em;color:var(--muted2);margin-bottom:16px}.lp-footer-links{list-style:none}.lp-footer-link{font-size:13.5px;color:#bbb;margin-bottom:10px;cursor:pointer;transition:color .1s;display:block;text-decoration:none}.lp-footer-link:hover{color:var(--text)}.lp-footer-bottom{border-top:1px solid var(--border);padding-top:24px;display:flex;align-items:center;justify-content:space-between}.lp-footer-copy{font-size:13px;color:var(--muted2)}.lp-footer-social{display:flex;gap:12px}.lp-footer-social-link{width:32px;height:32px;background:var(--s2);border:1px solid var(--border);border-radius:6px;display:flex;align-items:center;justify-content:center;color:var(--muted);cursor:pointer;transition:all .15s;font-size:14px;text-decoration:none}.lp-footer-social-link:hover{background:var(--s3);color:var(--text);border-color:var(--b2)}@media(min-width:1600px){:root{--nav-max:1560px;--content-max:1360px;--px:40px}}@media(max-width:1024px){:root{--nav-max:100%;--content-max:100%;--px:24px;--section-py:80px}}`;

export default function LandingPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <LandingAttribution />

      {/* NAV */}
      <nav className="lp-nav">
        <div className="lp-nav-inner">
          <Link href="/" className="lp-logo">
            <UniPostLogo markSize={28} wordmarkColor="var(--text)" />
          </Link>
          <div className="lp-nav-links">
            <Link href="/solutions" className="lp-nav-link">Solutions</Link>
            <Link href="/tools" className="lp-nav-link">Tools</Link>
            <Link href="/pricing" className="lp-nav-link">Pricing</Link>
            <div className="lp-nav-dropdown">
              <button className="lp-nav-link lp-nav-dropdown-trigger">
                Docs
                <svg width="10" height="10" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" style={{ marginLeft: 4 }}><path d="M4 6l4 4 4-4"/></svg>
              </button>
              <div className="lp-nav-dropdown-menu">
                <Link href="/docs/quickstart" className="lp-nav-dropdown-item">
                  <span className="lp-nav-dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg></span>
                  <span><div className="lp-nav-dropdown-label">Quickstart</div><div className="lp-nav-dropdown-desc">Go from API key to your first published post</div></span>
                </Link>
                <Link href="/docs/sdk" className="lp-nav-dropdown-item">
                  <span className="lp-nav-dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="M9 18h6"/><path d="M10 22h4"/><path d="M12 2a7 7 0 0 0-4 12.75V17a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1v-2.25A7 7 0 0 0 12 2Z"/></svg></span>
                  <span><div className="lp-nav-dropdown-label">SDKs</div><div className="lp-nav-dropdown-desc">Use UniPost from JavaScript, Python, and Go</div></span>
                </Link>
                <Link href="/docs/mcp" className="lp-nav-dropdown-item">
                  <span className="lp-nav-dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><circle cx="12" cy="12" r="3"/><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/></svg></span>
                  <span><div className="lp-nav-dropdown-label">MCP Server</div><div className="lp-nav-dropdown-desc">Connect AI agents to UniPost tools and workflows</div></span>
                </Link>
                <Link href="/docs/api" className="lp-nav-dropdown-item">
                  <span className="lp-nav-dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="M8 6h12"/><path d="M8 12h12"/><path d="M8 18h12"/><path d="M3 6h.01"/><path d="M3 12h.01"/><path d="M3 18h.01"/></svg></span>
                  <span><div className="lp-nav-dropdown-label">API Reference</div><div className="lp-nav-dropdown-desc">Browse endpoints, request shapes, and examples</div></span>
                </Link>
              </div>
            </div>
          </div>
          <MarketingNav />
        </div>
      </nav>

      <div className="lp-page">
        {/* HERO */}
        <div className="lp-hero">
          <div className="lp-hero-badge"><span className="lp-hero-badge-dot" />Now supporting 7 platforms</div>
          <h1 className="lp-hero-title">Onboard accounts.<br />Publish everywhere.<br /><em>Stay in control.</em></h1>
          <LandingHeroRotation />
          <p className="lp-hero-sub">UniPost is the social infrastructure layer for SaaS products and AI agents: branded Connect flows, per-platform publishing, draft preview, validation, webhooks, and analytics across every major network.</p>
          <div className="lp-hero-actions">
            <MarketingCTA className="lp-btn lp-btn-hero-primary lp-btn-hero-lg" />
            <Link href="/docs" className="lp-btn lp-btn-hero-outline lp-btn-hero-lg">View Docs →</Link>
          </div>
          <div className="lp-hero-meta">
            <div className="lp-hero-meta-item"><CheckIcon /><span>Free plan · 100 posts/month</span></div>
            <div className="lp-hero-meta-item"><CheckIcon /><span>Hosted Connect + white-label</span></div>
            <div className="lp-hero-meta-item"><CheckIcon /><span>Validate, preview, and publish</span></div>
          </div>
        </div>

        {/* PLATFORMS */}
        <div className="lp-platforms">
          <div className="lp-plat-label">Supported Platforms</div>
          <div className="lp-plat-row">
            {PLATFORMS.map((p) => (<Link key={p.name} href={`/${p.slug}-api`} className="lp-plat-chip" style={{ textDecoration: "none" }}><span className="lp-plat-icon">{PLATFORM_ICONS[p.name]}</span>{p.name}</Link>))}
          </div>
        </div>

        <div style={{ padding: "0 0 var(--section-py)" }}>
          <div className="lp-section-eyebrow">Built For Products</div>
          <h2 className="lp-section-title" style={{ marginBottom: 12 }}>More than a posting wrapper.</h2>
          <p className="lp-section-sub" style={{ marginBottom: 36 }}>
            UniPost already has the primitives you need when your product is publishing on behalf of end users, not just your own internal team.
          </p>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(2, minmax(0, 1fr))", gap: 16 }}>
            {CAPABILITIES.map((item) => (
              <div key={item.title} style={{ background: "var(--s1)", border: "1px solid var(--border)", borderRadius: 14, padding: 24 }}>
                <h3 style={{ fontSize: 20, fontWeight: 800, marginBottom: 10, letterSpacing: "-0.3px" }}>{item.title}</h3>
                <p style={{ fontSize: 14.5, color: "#bbb", lineHeight: 1.75, marginBottom: 14 }}>{item.desc}</p>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                  {item.points.map((point) => (
                    <span key={point} style={{ fontSize: 12, color: "var(--text)", background: "var(--s2)", border: "1px solid var(--b2)", borderRadius: 999, padding: "6px 10px", fontFamily: "var(--mono)" }}>
                      {point}
                    </span>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* CODE DEMO — two-column layout */}
        <div className="lp-code-section">
          <div className="lp-integ-grid">
            {/* Left: title + integration cards */}
            <div className="lp-integ-left">
              <h2 className="lp-integ-title">Per-platform publishing for real products.</h2>

              <div className="lp-integ-cards">
                <div className="lp-integ-card">
                  <div className="lp-integ-card-icon">
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><circle cx="12" cy="12" r="3"/><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/></svg>
                  </div>
                  <div>
                    <div className="lp-integ-card-title">Recommended request shape</div>
                    <div className="lp-integ-card-desc">Use `platform_posts[]` when AI or product logic needs different copy per destination.</div>
                    <Link href="/docs" className="lp-integ-card-link">Docs ↗</Link>
                  </div>
                </div>
                <div className="lp-integ-card-divider" />
                <div className="lp-integ-card">
                  <div className="lp-integ-card-icon">
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                  </div>
                  <div>
                    <div className="lp-integ-card-title">AgentPost-ready</div>
                    <div className="lp-integ-card-desc">The same API shape powers preview, AI-generated drafts, and per-platform publishing flows.</div>
                    <Link href="/tools/agentpost" className="lp-integ-card-link">See AgentPost ↗</Link>
                  </div>
                </div>
              </div>
            </div>

            {/* Right: code editor */}
            <LandingCodeTabs />
          </div>
        </div>

        {/* FEATURES */}
        <div className="lp-features">
          <div className="lp-section-eyebrow">Why UniPost</div>
          <h2 className="lp-section-title">Infrastructure for SaaS teams and AI agents.</h2>
          <p className="lp-section-sub" style={{ marginBottom: 64 }}>Not just posting. UniPost covers onboarding, validation, preview, publish, and monitoring.</p>
          <div className="lp-feat-list">
            {FEATURES.map((f, i) => (
              <div key={f.number} className={`lp-feat-item ${i % 2 !== 0 ? "reverse" : ""}`}>
                <div><div className="lp-feat-number">{f.number}</div><h3 className="lp-feat-title">{f.title}</h3><p className="lp-feat-desc">{f.desc}</p></div>
                <pre className="lp-feat-code">{f.code}</pre>
              </div>
            ))}
          </div>
        </div>

        <div style={{ padding: "0 0 var(--section-py)" }}>
          <div className="lp-section-eyebrow">Proof</div>
          <h2 className="lp-section-title" style={{ marginBottom: 12 }}>How UniPost fits into your product.</h2>
          <p className="lp-section-sub" style={{ marginBottom: 36 }}>
            The winning workflow is not “generate one caption and spray it everywhere.” It is onboarding users cleanly, validating drafts, then publishing with visibility.
          </p>
          <div style={{ display: "grid", gridTemplateColumns: "1.1fr .9fr", gap: 18, alignItems: "stretch" }}>
            <div style={{ background: "linear-gradient(180deg,#101010,#0a0a0a)", border: "1px solid var(--border)", borderRadius: 16, padding: 24 }}>
              <div style={{ fontSize: 12, color: "var(--accent)", fontFamily: "var(--mono)", marginBottom: 14 }}>Branded Connect Flow</div>
              <div style={{ background: "#ffffff", color: "#111", borderRadius: 14, padding: 24, maxWidth: 420, boxShadow: "0 16px 50px #00000040" }}>
                <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 18 }}>
                  <div style={{ width: 42, height: 42, borderRadius: 10, background: "#0f172a", color: "#fff", display: "flex", alignItems: "center", justifyContent: "center", fontWeight: 800 }}>A</div>
                  <div>
                    <div style={{ fontWeight: 700 }}>Acme Social</div>
                    <div style={{ fontSize: 12, color: "#666" }}>Powered by UniPost</div>
                  </div>
                </div>
                <h3 style={{ fontSize: 22, fontWeight: 800, marginBottom: 8 }}>Connect your LinkedIn account</h3>
                <p style={{ fontSize: 14, lineHeight: 1.6, color: "#555", marginBottom: 18 }}>
                  Your users see your brand during onboarding while UniPost handles the OAuth mechanics and token lifecycle underneath.
                </p>
                <div style={{ background: "#111827", color: "#fff", borderRadius: 10, textAlign: "center", padding: "12px 16px", fontWeight: 700 }}>
                  Continue with LinkedIn
                </div>
              </div>
            </div>
            <div style={{ display: "grid", gap: 12 }}>
              {PROOF_STEPS.map((item) => (
                <div key={item.step} style={{ background: "var(--s1)", border: "1px solid var(--border)", borderRadius: 14, padding: 20 }}>
                  <div style={{ fontSize: 12, fontFamily: "var(--mono)", color: "var(--accent)", marginBottom: 8 }}>{item.step}</div>
                  <div style={{ fontSize: 18, fontWeight: 800, marginBottom: 8 }}>{item.title}</div>
                  <p style={{ fontSize: 14, color: "#bbb", lineHeight: 1.7 }}>{item.body}</p>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* STATS */}
        <div className="lp-stats">
          <div className="lp-stats-inner">
            {[{ num: "7", label: "Platforms supported" }, { num: "1", label: "Connect + publish stack" }, { num: "24h", label: "Preview links lifetime" }, { num: "∞", label: "Social accounts per project" }].map((s) => (
              <div key={s.num} className="lp-stat"><div className="lp-stat-num">{s.num}</div><div className="lp-stat-label">{s.label}</div></div>
            ))}
          </div>
        </div>

        {/* MODES */}
        <div className="lp-modes">
          <div className="lp-section-eyebrow">Two Modes</div>
          <h2 className="lp-section-title" style={{ marginBottom: 12 }}>Start fast. Then make it yours.</h2>
          <p className="lp-section-sub">Quickstart gets your product live fast. White-label turns the onboarding surface into your own brand.</p>
          <div className="lp-modes-grid">
            {MODES.map((m, i) => (
              <div key={m.badge} className={`lp-mode-card ${i === 0 ? "mode-quickstart" : "mode-native"}`}>
                <div className="lp-mode-badge" style={{ background: m.badgeColor + "18", border: `1px solid ${m.badgeColor}30`, color: m.badgeColor }}>
                  <span className="lp-mode-icon" style={{ background: m.badgeColor + "20" }}>
                    {i === 0 ? (
                      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke={m.badgeColor} strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M9 2L4 9h4l-1 5 5-7H8l1-5z"/></svg>
                    ) : (
                      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke={m.badgeColor} strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><rect x="3" y="3" width="10" height="10" rx="2"/><path d="M7 7l2 2M9 7l-2 2"/></svg>
                    )}
                  </span>
                  {m.badge}
                </div>
                <h3 className="lp-mode-title">{m.title}</h3>
                <p className="lp-mode-desc">{m.desc}</p>
                <ul className="lp-mode-feats">{m.features.map((f) => (
                  <li key={f} className="lp-mode-feat">
                    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke={m.badgeColor} strokeWidth="2.2" style={{ flexShrink: 0, marginTop: 3 }}><path d="M3 8l4 4 6-7"/></svg>
                    {f}
                  </li>
                ))}</ul>
                <MarketingCTALight />
              </div>
            ))}
          </div>
        </div>

      </div>

      {/* FAQ — full-width dark band */}
      <div className="lp-faq-band">
          <div className="lp-faq-band-inner">
            <div className="lp-section-eyebrow" style={{ textAlign: "center" }}>FAQ</div>
            <h2 className="lp-section-title" style={{ textAlign: "center", marginBottom: 48 }}>Common questions</h2>
          <div className="lp-faq-grid">
            {FAQS.map((f) => (<div key={f.q} className="lp-faq-item"><div className="lp-faq-q">{f.q}</div><div className="lp-faq-a">{f.a}</div></div>))}
          </div>
        </div>
      </div>

      {/* CTA — full-width darker band */}
      <div className="lp-cta-band">
        <div className="lp-cta-band-inner">
          <div className="lp-cta-inner">
            <div className="lp-cta-glow" />
            <h2 className="lp-cta-title">Build the social layer once.</h2>
            <p className="lp-cta-sub">Start with the free plan, wire up Connect and publish flows, then scale into white-label onboarding and higher volume when your product is ready.</p>
            <div className="lp-cta-actions">
              <MarketingCTA />
              <Link href="/docs" className="lp-btn lp-btn-outline lp-btn-lg">Read the Docs →</Link>
            </div>
            <p style={{ fontSize: 13, color: "#555", marginTop: 20, position: "relative" }}>Need to prove the AI-native path too? <Link href="/tools/agentpost" style={{ color: "#999", textDecoration: "underline" }}>Try AgentPost</Link> or <Link href="/compare" style={{ color: "#999", textDecoration: "underline" }}>see competitor comparisons →</Link></p>
          </div>
        </div>
      </div>

    </>
  );
}
