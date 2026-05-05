import type { Metadata } from "next";
import type { ReactNode } from "react";
import Link from "next/link";
import { UniPostLogo } from "@/components/brand/unipost-logo";
import { MarketingNav, MarketingCTA, MarketingCTALight } from "@/components/marketing/nav";
import { LandingAttribution } from "@/components/marketing/landing-attribution";
import { LandingCodeTabs } from "@/components/marketing/landing-code-tabs";

export const metadata: Metadata = {
  title: "UniPost | Unified Social Media API for Apps and AI Agents",
  description:
    "Add social publishing to your product with hosted Connect, validation, media uploads, per-platform posting, and delivery monitoring across X, Bluesky, LinkedIn, Instagram, Threads, TikTok, YouTube, and Pinterest.",
  alternates: {
    canonical: "https://unipost.dev/",
  },
};

const PLATFORM_ICONS: Record<string, ReactNode> = {
  X: <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z"/></svg>,
  Bluesky: <svg width="18" height="18" viewBox="0 0 600 530" fill="#0085ff"><path d="M135.7 44.3C202.3 94.8 273.6 197.2 300 249.6c26.4-52.4 97.7-154.8 164.3-205.3C520.4 1.5 588 -22.1 588 68.2c0 18 -10.4 151.2-16.5 172.8-21.2 75-98.6 94.1-167.9 82.6 121.1 20.7 151.8 89.2 85.3 157.8C390.5 584.2 310.2 500 300 481.4c-10.2 18.6-90.5 102.8-188.9 0C44.6 413.8 75.3 345.3 196.4 324.6c-69.3 11.5-146.7-7.6-167.9-82.6C22.4 220.4 12 87.2 12 69.2c0-90.3 67.6-66.7 123.7-24.9z"/></svg>,
  LinkedIn: <svg width="18" height="18" viewBox="0 0 24 24" fill="#0a66c2"><path d="M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 5.455v6.286zM5.337 7.433a2.062 2.062 0 01-2.063-2.065 2.064 2.064 0 112.063 2.065zm1.782 13.019H3.555V9h3.564v11.452zM22.225 0H1.771C.792 0 0 .774 0 1.729v20.542C0 23.227.792 24 1.771 24h20.451C23.2 24 24 23.227 24 22.271V1.729C24 .774 23.2 0 22.222 0h.003z"/></svg>,
  Instagram: <svg width="18" height="18" viewBox="0 0 24 24" fill="url(#ig)"><defs><radialGradient id="ig" cx="30%" cy="107%" r="150%"><stop offset="0%" stopColor="#fdf497"/><stop offset="5%" stopColor="#fdf497"/><stop offset="45%" stopColor="#fd5949"/><stop offset="60%" stopColor="#d6249f"/><stop offset="90%" stopColor="#285AEB"/></radialGradient></defs><path d="M12 2.163c3.204 0 3.584.012 4.85.07 3.252.148 4.771 1.691 4.919 4.919.058 1.265.069 1.645.069 4.849 0 3.205-.012 3.584-.069 4.849-.149 3.225-1.664 4.771-4.919 4.919-1.266.058-1.644.07-4.85.07-3.204 0-3.584-.012-4.849-.07-3.26-.149-4.771-1.699-4.919-4.92-.058-1.265-.07-1.644-.07-4.849 0-3.204.013-3.583.07-4.849.149-3.227 1.664-4.771 4.919-4.919 1.266-.057 1.645-.069 4.849-.069zM12 0C8.741 0 8.333.014 7.053.072 2.695.272.273 2.69.073 7.052.014 8.333 0 8.741 0 12c0 3.259.014 3.668.072 4.948.2 4.358 2.618 6.78 6.98 6.98C8.333 23.986 8.741 24 12 24c3.259 0 3.668-.014 4.948-.072 4.354-.2 6.782-2.618 6.979-6.98.059-1.28.073-1.689.073-4.948 0-3.259-.014-3.667-.072-4.947-.196-4.354-2.617-6.78-6.979-6.98C15.668.014 15.259 0 12 0zm0 5.838a6.162 6.162 0 100 12.324 6.162 6.162 0 000-12.324zM12 16a4 4 0 110-8 4 4 0 010 8zm6.406-11.845a1.44 1.44 0 100 2.881 1.44 1.44 0 000-2.881z"/></svg>,
  Threads: <svg width="18" height="18" viewBox="0 0 192 192" fill="currentColor"><path d="M141.537 88.988a66.667 66.667 0 0 0-2.518-1.143c-1.482-27.307-16.403-42.94-41.457-43.1h-.34c-14.986 0-27.449 6.396-35.12 18.036l13.779 9.452c5.73-8.695 14.724-10.548 21.348-10.548h.229c8.249.053 14.474 2.452 18.503 7.129 2.932 3.405 4.893 8.111 5.864 14.05-7.314-1.243-15.224-1.626-23.68-1.14-23.82 1.371-39.134 15.326-38.092 34.7.528 9.818 5.235 18.28 13.256 23.808 6.768 4.666 15.471 6.98 24.49 6.52 11.918-.607 21.27-5.003 27.79-13.066 4.947-6.116 8.1-13.908 9.532-23.619 5.708 3.45 9.953 8.063 12.37 13.676 4.106 9.533 4.349 25.194-7.865 37.315-10.724 10.64-23.618 15.254-38.399 15.358-16.388-.115-28.796-5.382-36.877-15.66-7.515-9.56-11.416-23.12-11.594-40.322.178-17.202 4.079-30.762 11.594-40.322 8.081-10.278 20.489-15.545 36.877-15.66 16.506.116 29.148 5.42 37.567 15.76 4.108 5.048 7.21 11.467 9.312 19.023l14.854-3.982c-2.605-9.463-6.641-17.573-12.159-24.356C152.088 14.14 136.308 7.353 116.379 7.2h-.069c-19.874.142-35.468 6.947-46.333 20.25C60.4 39.452 55.545 55.77 55.33 75.94l-.002.162.002.16c.215 20.17 5.07 36.488 14.645 48.49 10.865 13.303 26.459 20.108 46.333 20.25h.069c18.134-.119 33.577-5.86 45.916-17.068 16.456-14.938 17.617-36.986 12.28-49.39-3.835-8.908-11.151-16.063-21.036-20.544zm-36.844 51.014c-9.985.508-20.361-3.928-21.025-13.278-.477-6.732 4.746-14.243 24.298-15.368 2.132-.123 4.22-.183 6.263-.183 6.26 0 12.12.616 17.39 1.812-1.98 22.459-14.948 26.513-26.926 27.017z"/></svg>,
  TikTok: <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path d="M19.59 6.69a4.83 4.83 0 01-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 01-2.88 2.5 2.89 2.89 0 01-2.89-2.89 2.89 2.89 0 012.89-2.89c.28 0 .54.04.79.1v-3.5a6.37 6.37 0 00-.79-.05A6.34 6.34 0 003.15 15.2a6.34 6.34 0 0010.86 4.48 6.3 6.3 0 001.86-4.48V8.73a8.26 8.26 0 004.84 1.56V6.84a4.85 4.85 0 01-1.12-.15z"/></svg>,
  YouTube: <svg width="18" height="18" viewBox="0 0 24 24" fill="#ff0000"><path d="M23.498 6.186a3.016 3.016 0 00-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 00.502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 002.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 002.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"/></svg>,
  Pinterest: <svg width="18" height="18" viewBox="0 0 24 24" fill="#e60023"><path d="M12.017 0C5.396 0 0 5.383 0 12.014c0 4.895 2.918 9.104 7.111 10.956-.098-.93-.186-2.36.04-3.377.205-.872 1.314-5.55 1.314-5.55s-.336-.672-.336-1.664c0-1.56.91-2.726 2.038-2.726.958 0 1.42.718 1.42 1.577 0 .962-.614 2.398-.93 3.73-.264 1.114.562 2.022 1.666 2.022 2 0 3.536-2.11 3.536-5.156 0-2.693-1.935-4.576-4.7-4.576-3.2 0-5.08 2.4-5.08 4.88 0 .968.373 2.008.84 2.574a.338.338 0 01.077.323c-.084.355-.27 1.114-.307 1.27-.05.204-.165.247-.38.148-1.415-.658-2.3-2.724-2.3-4.386 0-3.57 2.594-6.85 7.478-6.85 3.925 0 6.976 2.8 6.976 6.548 0 3.904-2.46 7.047-5.875 7.047-1.147 0-2.227-.596-2.595-1.302l-.705 2.686c-.254.978-.94 2.202-1.4 2.95 1.053.324 2.168.5 3.333.5C18.624 24 24 18.617 24 12.014 24 5.383 18.624 0 12.017 0z"/></svg>,
};

const PLATFORMS = [
  { name: "X", slug: "twitter" },
  { name: "Bluesky", slug: "bluesky" },
  { name: "LinkedIn", slug: "linkedin" },
  { name: "Instagram", slug: "instagram" },
  { name: "Threads", slug: "threads" },
  { name: "TikTok", slug: "tiktok" },
  { name: "YouTube", slug: "youtube" },
  { name: "Pinterest", slug: "pinterest" },
];

const HERO_POINTS = [
  "Hosted Connect flows for end-user social accounts",
  "Per-platform captions, media handling, and validation",
  "Delivery jobs, account health, webhooks, and analytics",
];

const USE_CASES = [
  {
    eyebrow: "For SaaS products",
    title: "Add social publishing without building the plumbing.",
    body: "Ship account onboarding, draft validation, previews, publishing, and delivery monitoring as product features instead of a quarter-long infrastructure project.",
  },
  {
    eyebrow: "For AI agents",
    title: "Give agents a safe path to publish.",
    body: "Use per-platform payloads, validation, idempotency, and preview links so AI systems can draft and publish without spraying the same caption everywhere.",
  },
  {
    eyebrow: "For ops teams",
    title: "Keep one control plane for every destination.",
    body: "Track usage, reconnect unhealthy accounts, inspect failed deliveries, and keep customer-facing posting flows inside one consistent API surface.",
  },
];

const PRIMITIVES = [
  {
    title: "Hosted Connect",
    desc: "Onboard customer accounts through a branded OAuth flow while UniPost handles token exchange and refresh.",
    chips: ["Connect sessions", "external_user_id mapping", "White-label"],
  },
  {
    title: "Validation + Preview",
    desc: "Catch platform issues before send, then share a read-only preview link with teammates, customers, or AI review steps.",
    chips: ["POST /v1/posts/validate", "Preview links", "Per-platform checks"],
  },
  {
    title: "Media Pipeline",
    desc: "Use public asset URLs or reserve local uploads through UniPost so image and video workflows stay predictable.",
    chips: ["POST /v1/media", "upload_url", "media_ids"],
  },
  {
    title: "Per-platform Delivery",
    desc: "Publish one campaign across multiple accounts while still controlling captions, media shape, and platform-specific options.",
    chips: ["platform_posts[]", "Account-level results", "Retries"],
  },
];

const FLOW_STEPS = [
  {
    step: "01",
    title: "Onboard the user account",
    body: "Create a Connect session for a specific end user and let UniPost own the OAuth handshake and token lifecycle.",
  },
  {
    step: "02",
    title: "Validate and prepare the payload",
    body: "Run preflight validation, upload local media when needed, and generate a preview if the content needs review before publishing.",
  },
  {
    step: "03",
    title: "Publish and monitor outcomes",
    body: "Send the post, watch delivery jobs, listen for webhooks, and surface account health and analytics back inside your own app.",
  },
];

const STATS = [
  { number: "8", label: "platforms supported now" },
  { number: "100", label: "free posts every month" },
  { number: "1", label: "API for connect, publish, and monitor" },
  { number: "24h", label: "default preview link lifetime" },
];

const MODES = [
  {
    badge: "Quickstart",
    badgeColor: "#22c55e",
    title: "Go live before you apply for every platform credential.",
    desc: "Use UniPost's developer credentials and start integrating immediately while you validate your product and onboarding flow.",
    features: [
      "Fastest path from API key to real posts",
      "No platform approval process to get started",
      "Ideal for prototyping, pilots, and early customer validation",
    ],
  },
  {
    badge: "White-label",
    badgeColor: "#38bdf8",
    title: "Move the social surface under your own brand.",
    desc: "Bring your own credentials and present your product throughout the hosted onboarding experience when you are ready to own the full flow.",
    features: [
      "OAuth screens show your app, not a generic tool",
      "Credential ownership stays with your product team",
      "Best fit for customer-facing SaaS at scale",
    ],
  },
];

const FAQS = [
  {
    q: "What is UniPost in one sentence?",
    a: "UniPost is a unified social media API that lets apps and AI agents connect end-user accounts, validate drafts, upload media, publish per-platform content, and monitor delivery across multiple networks.",
  },
  {
    q: "Who is this built for?",
    a: "Teams building customer-facing products, AI agents, internal automations, and workflow tools that need to publish on behalf of many users without owning every OAuth and media edge case themselves.",
  },
  {
    q: "Do I need to handle local media uploads myself?",
    a: "No. If you already host the file publicly, send the URL directly. If the file is local, reserve an upload through POST /v1/media, upload the bytes to the returned URL, then publish using media_ids.",
  },
  {
    q: "What counts as a post on the free plan?",
    a: "One successful publish to one social account counts as one post. A campaign that publishes to three accounts counts as three posts. Failed posts do not count against quota.",
  },
];

function CheckIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2.2" width="14" height="14" style={{ flexShrink: 0 }}>
      <path d="M3 8l4 4 6-7" />
    </svg>
  );
}

const CSS = `
:root{
  --lp-page-max:1360px;
  --lp-nav-max:1480px;
  --lp-text-max:680px;
  --lp-pad:32px;
  --lp-section:112px;
  --lp-emerald:#34d399;
  --lp-sky:#38bdf8;
  --lp-amber:#f59e0b;
  --lp-shadow:0 24px 80px rgba(0,0,0,.18);
  --lp-soft-shadow:0 16px 36px rgba(15,23,42,.08);
}
html.dark{
  --lp-bg:#050816;
  --lp-bg-2:#081120;
  --lp-surface:#0c1728;
  --lp-surface-2:#0f1d33;
  --lp-surface-3:#13233c;
  --lp-border:rgba(255,255,255,.09);
  --lp-border-2:rgba(255,255,255,.14);
  --lp-text:#f8fafc;
  --lp-muted:#a9b6ca;
  --lp-soft:#dbe5f4;
  --lp-chip:#d5e0ef;
  --lp-overlay:rgba(5,8,22,.72);
  --lp-hero-glow:radial-gradient(circle at top left, rgba(56,189,248,.18), transparent 42%),radial-gradient(circle at 90% 10%, rgba(52,211,153,.16), transparent 32%),linear-gradient(180deg, #071021 0%, #050816 42%, #071020 100%);
  --lp-panel:linear-gradient(180deg, rgba(12,23,40,.94), rgba(8,16,30,.98));
  --lp-code-bg:#0b1220;
  --lp-code-text:#d7e4f3;
  --lp-code-gutter:#5e7089;
}
html.light{
  --lp-bg:#f5f9ff;
  --lp-bg-2:#edf5fd;
  --lp-surface:#ffffff;
  --lp-surface-2:#f7fbff;
  --lp-surface-3:#edf4fb;
  --lp-border:rgba(15,23,42,.10);
  --lp-border-2:rgba(15,23,42,.16);
  --lp-text:#0f172a;
  --lp-muted:#56657a;
  --lp-soft:#223046;
  --lp-chip:#334155;
  --lp-overlay:rgba(245,249,255,.78);
  --lp-hero-glow:radial-gradient(circle at top left, rgba(56,189,248,.18), transparent 42%),radial-gradient(circle at 90% 10%, rgba(52,211,153,.18), transparent 30%),linear-gradient(180deg, #f8fbff 0%, #eef6ff 44%, #f5f9ff 100%);
  --lp-panel:linear-gradient(180deg, rgba(255,255,255,.98), rgba(244,249,255,.98));
  --lp-code-bg:#0f172a;
  --lp-code-text:#dbe6f6;
  --lp-code-gutter:#64748b;
}
*{box-sizing:border-box}
body{
  background:var(--lp-bg);
  color:var(--lp-text);
  font-family:var(--font-dm-sans),system-ui,sans-serif;
  -webkit-font-smoothing:antialiased;
}
.lp-shell{
  min-height:100vh;
  background:
    radial-gradient(circle at 15% 0%, rgba(52,211,153,.09), transparent 26%),
    radial-gradient(circle at 82% 4%, rgba(56,189,248,.10), transparent 24%),
    var(--lp-bg);
}
.lp-nav{
  position:sticky;
  top:0;
  z-index:50;
  width:100%;
  border-bottom:1px solid var(--lp-border);
  background:var(--lp-overlay);
  backdrop-filter:blur(18px);
  -webkit-backdrop-filter:blur(18px);
}
.lp-nav-inner{
  max-width:var(--lp-nav-max);
  margin:0 auto;
  padding:0 var(--lp-pad);
  height:62px;
  display:grid;
  grid-template-columns:minmax(0,1fr) auto minmax(0,1fr);
  align-items:center;
  gap:16px;
}
.lp-nav-links{
  display:flex;
  align-items:center;
  justify-content:center;
  gap:4px;
}
.lp-nav-link{
  padding:8px 14px;
  border-radius:999px;
  color:var(--lp-muted);
  text-decoration:none;
  font-size:14px;
  font-weight:600;
  transition:all .14s;
}
.lp-nav-link:hover{
  color:var(--lp-text);
  background:rgba(255,255,255,.04);
}
.lp-nav-dropdown{position:relative}
.lp-nav-dropdown-trigger{
  display:inline-flex;
  align-items:center;
  background:none;
  border:none;
  font-family:inherit;
}
.lp-nav-dropdown-menu{
  position:absolute;
  top:100%;
  right:0;
  margin-top:8px;
  min-width:260px;
  border-radius:18px;
  padding:8px;
  background:var(--lp-surface);
  border:1px solid var(--lp-border-2);
  opacity:0;
  visibility:hidden;
  transform:translateY(-6px);
  transition:all .16s ease;
  box-shadow:0 24px 50px rgba(15,23,42,.18);
}
.lp-nav-dropdown:hover .lp-nav-dropdown-menu{
  opacity:1;
  visibility:visible;
  transform:translateY(0);
}
.lp-nav-dropdown-item{
  display:flex;
  gap:12px;
  align-items:flex-start;
  padding:12px;
  border-radius:14px;
  color:var(--lp-text);
  text-decoration:none;
  transition:background .12s;
}
.lp-nav-dropdown-item:hover{background:var(--lp-surface-2)}
.lp-nav-dropdown-icon{
  width:36px;
  height:36px;
  border-radius:12px;
  background:var(--lp-surface-3);
  border:1px solid var(--lp-border);
  display:flex;
  align-items:center;
  justify-content:center;
  color:var(--lp-emerald);
  flex-shrink:0;
}
.lp-nav-dropdown-label{
  font-size:14px;
  font-weight:700;
  margin-bottom:3px;
}
.lp-nav-dropdown-desc{
  font-size:12.5px;
  line-height:1.45;
  color:var(--lp-muted);
}
.lp-page{
  max-width:var(--lp-page-max);
  margin:0 auto;
  padding:0 var(--lp-pad) 120px;
}
.lp-btn{
  display:inline-flex;
  align-items:center;
  justify-content:center;
  gap:8px;
  min-height:48px;
  padding:12px 22px;
  border-radius:999px;
  font-size:14px;
  font-weight:700;
  text-decoration:none;
  border:1px solid transparent;
  transition:all .15s;
}
.lp-btn-primary{
  background:linear-gradient(135deg, var(--lp-emerald), #7cf3be);
  color:#03110a;
  box-shadow:0 18px 40px rgba(52,211,153,.24);
}
.lp-btn-primary:hover{
  transform:translateY(-1px);
  box-shadow:0 24px 44px rgba(52,211,153,.30);
}
.lp-btn-outline{
  color:var(--lp-text);
  border-color:var(--lp-border-2);
  background:rgba(255,255,255,.03);
}
.lp-btn-outline:hover{
  background:rgba(255,255,255,.06);
  border-color:rgba(255,255,255,.22);
}
.lp-btn-subtle{
  color:var(--lp-soft);
  border-color:var(--lp-border);
  background:transparent;
}
.lp-btn-subtle:hover{
  background:rgba(255,255,255,.03);
}
.lp-section{
  padding-top:var(--lp-section);
}
.lp-eyebrow{
  display:inline-flex;
  align-items:center;
  gap:10px;
  padding:8px 14px;
  border-radius:999px;
  border:1px solid rgba(52,211,153,.24);
  background:rgba(52,211,153,.09);
  color:var(--lp-emerald);
  font-size:12px;
  font-weight:800;
  letter-spacing:.08em;
  text-transform:uppercase;
}
.lp-eyebrow-dot{
  width:7px;
  height:7px;
  border-radius:50%;
  background:currentColor;
  box-shadow:0 0 0 6px rgba(52,211,153,.12);
}
.lp-hero{
  padding:40px 0 24px;
}
.lp-hero-shell{
  display:grid;
  grid-template-columns:minmax(0,1.02fr) minmax(400px,.98fr);
  gap:36px;
  align-items:center;
  padding:48px;
  border:1px solid var(--lp-border);
  border-radius:32px;
  background:var(--lp-hero-glow);
  overflow:hidden;
  position:relative;
  box-shadow:var(--lp-shadow);
}
.lp-hero-copy{
  position:relative;
  z-index:1;
  display:flex;
  flex-direction:column;
  align-items:center;
  text-align:center;
}
.lp-hero-title{
  margin:22px 0 18px;
  max-width:720px;
  font-size:clamp(48px, 6vw, 84px);
  line-height:.98;
  letter-spacing:-.06em;
  font-weight:900;
}
.lp-hero-title strong{
  display:block;
  color:var(--lp-emerald);
  font-weight:900;
}
.lp-hero-sub{
  max-width:var(--lp-text-max);
  color:var(--lp-soft);
  font-size:18px;
  line-height:1.75;
}
.lp-hero-actions{
  display:flex;
  align-items:center;
  justify-content:center;
  gap:12px;
  flex-wrap:wrap;
  margin:32px 0 24px;
}
.lp-hero-proof{
  display:grid;
  grid-template-columns:repeat(2,minmax(0,1fr));
  gap:12px;
  max-width:640px;
  width:100%;
}
.lp-hero-proof-card{
  display:flex;
  gap:12px;
  align-items:flex-start;
  padding:14px 16px;
  border-radius:18px;
  background:rgba(255,255,255,.05);
  border:1px solid var(--lp-border);
}
.lp-hero-proof-card svg{
  color:var(--lp-emerald);
  margin-top:2px;
}
.lp-hero-proof-label{
  font-size:13px;
  line-height:1.55;
  color:var(--lp-chip);
}
.lp-hero-visual{
  position:relative;
  min-height:520px;
}
.lp-visual-stack{
  position:absolute;
  inset:0;
}
.lp-visual-card{
  position:absolute;
  border:1px solid var(--lp-border);
  background:var(--lp-panel);
  border-radius:24px;
  box-shadow:var(--lp-shadow);
  overflow:hidden;
}
.lp-visual-card-main{
  right:0;
  top:16px;
  width:min(100%, 520px);
  padding:20px;
}
.lp-visual-card-secondary{
  left:16px;
  bottom:18px;
  width:min(78%, 360px);
  padding:18px;
}
.lp-visual-card-float{
  right:38px;
  bottom:0;
  width:240px;
  padding:16px;
}
.lp-visual-topline{
  display:flex;
  align-items:center;
  justify-content:space-between;
  margin-bottom:18px;
}
.lp-visual-topline strong{
  font-size:14px;
  letter-spacing:-.02em;
}
.lp-visual-pill{
  padding:6px 10px;
  border-radius:999px;
  background:rgba(56,189,248,.14);
  color:var(--lp-sky);
  font-size:11px;
  font-weight:800;
  text-transform:uppercase;
  letter-spacing:.08em;
}
.lp-visual-lanes{
  display:grid;
  gap:14px;
}
.lp-lane{
  padding:16px;
  border-radius:18px;
  background:rgba(255,255,255,.04);
  border:1px solid var(--lp-border);
}
.lp-lane-label{
  display:flex;
  align-items:center;
  justify-content:space-between;
  gap:12px;
  margin-bottom:10px;
  font-size:13px;
  font-weight:700;
}
.lp-lane-sub{
  color:var(--lp-muted);
  font-size:12.5px;
  line-height:1.55;
}
.lp-lane-platforms{
  display:flex;
  flex-wrap:wrap;
  gap:8px;
  margin-top:12px;
}
.lp-mini-chip{
  display:inline-flex;
  align-items:center;
  gap:7px;
  padding:7px 10px;
  border-radius:999px;
  background:rgba(255,255,255,.06);
  border:1px solid var(--lp-border);
  color:var(--lp-chip);
  font-size:12px;
  font-weight:600;
}
.lp-signal-list{
  display:grid;
  gap:10px;
}
.lp-signal-item{
  display:flex;
  align-items:center;
  justify-content:space-between;
  gap:12px;
  padding:12px 14px;
  border-radius:16px;
  background:rgba(255,255,255,.04);
  border:1px solid var(--lp-border);
  font-size:13px;
}
.lp-signal-meta{
  display:flex;
  flex-direction:column;
  gap:4px;
}
.lp-signal-meta strong{
  font-size:13px;
}
.lp-signal-meta span{
  color:var(--lp-muted);
  font-size:12px;
}
.lp-signal-badge{
  padding:5px 10px;
  border-radius:999px;
  font-size:11px;
  font-weight:800;
  letter-spacing:.08em;
  text-transform:uppercase;
}
.lp-signal-badge.ok{background:rgba(52,211,153,.16);color:var(--lp-emerald)}
.lp-signal-badge.warn{background:rgba(245,158,11,.16);color:var(--lp-amber)}
.lp-code-card{
  border-radius:18px;
  overflow:hidden;
  border:1px solid rgba(255,255,255,.08);
  background:var(--lp-code-bg);
}
.lp-code-card-head{
  display:flex;
  align-items:center;
  gap:8px;
  padding:12px 14px;
  border-bottom:1px solid rgba(255,255,255,.06);
}
.lp-code-dot{
  width:9px;
  height:9px;
  border-radius:50%;
  background:rgba(255,255,255,.26);
}
.lp-code-card-body{
  padding:14px 16px 18px;
  color:var(--lp-code-text);
  font-family:var(--font-fira-code),monospace;
  font-size:12px;
  line-height:1.8;
  white-space:pre;
}
.lp-code-accent{color:#7dd3fc}
.lp-code-green{color:#86efac}
.lp-code-amber{color:#fcd34d}
.lp-platforms{
  padding-top:28px;
  display:flex;
  flex-direction:column;
  align-items:center;
}
.lp-platforms-label{
  font-size:12px;
  color:var(--lp-muted);
  text-transform:uppercase;
  letter-spacing:.12em;
  font-weight:800;
  margin-bottom:18px;
  text-align:center;
}
.lp-platform-row{
  display:flex;
  flex-wrap:wrap;
  justify-content:center;
  gap:12px;
}
.lp-platform-chip{
  display:inline-flex;
  align-items:center;
  gap:10px;
  padding:10px 16px;
  border-radius:999px;
  text-decoration:none;
  color:var(--lp-chip);
  border:1px solid var(--lp-border);
  background:var(--lp-surface);
  transition:all .14s;
  font-size:14px;
  font-weight:700;
}
.lp-platform-chip:hover{
  transform:translateY(-1px);
  color:var(--lp-text);
  border-color:var(--lp-border-2);
}
.lp-section-head{
  max-width:720px;
  margin:0 auto 28px;
  text-align:center;
  display:flex;
  flex-direction:column;
  align-items:center;
  margin-bottom:28px;
}
.lp-section-head h2{
  font-size:clamp(32px,4vw,54px);
  line-height:1.02;
  letter-spacing:-.05em;
  font-weight:900;
  margin:16px 0 12px;
}
.lp-section-head p{
  color:var(--lp-soft);
  font-size:17px;
  line-height:1.75;
  max-width:680px;
}
.lp-usecase-grid{
  display:grid;
  grid-template-columns:repeat(3,minmax(0,1fr));
  gap:18px;
}
.lp-usecase-card,
.lp-primitive-card,
.lp-flow-step,
.lp-mode-card,
.lp-faq-item{
  background:var(--lp-surface);
  border:1px solid var(--lp-border);
  border-radius:24px;
}
.lp-usecase-card{
  padding:26px;
  min-height:230px;
  box-shadow:var(--lp-soft-shadow);
}
.lp-card-eyebrow{
  color:var(--lp-sky);
  font-size:12px;
  letter-spacing:.08em;
  text-transform:uppercase;
  font-weight:800;
  margin-bottom:12px;
}
.lp-usecase-card h3,
.lp-primitive-card h3,
.lp-flow-step h3,
.lp-mode-card h3,
.lp-faq-item h3{
  font-size:24px;
  line-height:1.15;
  letter-spacing:-.04em;
  font-weight:800;
  margin-bottom:12px;
}
.lp-usecase-card p,
.lp-primitive-card p,
.lp-flow-step p,
.lp-mode-card p,
.lp-faq-item p{
  color:var(--lp-soft);
  font-size:15px;
  line-height:1.75;
}
.lp-primitives-grid{
  display:grid;
  grid-template-columns:repeat(2,minmax(0,1fr));
  gap:18px;
}
.lp-primitive-card{
  padding:26px;
}
.lp-chip-row{
  display:flex;
  flex-wrap:wrap;
  gap:10px;
  margin-top:18px;
}
.lp-chip{
  display:inline-flex;
  align-items:center;
  gap:8px;
  padding:8px 11px;
  border-radius:999px;
  background:var(--lp-surface-2);
  border:1px solid var(--lp-border);
  color:var(--lp-chip);
  font-size:12px;
  font-family:var(--font-fira-code),monospace;
  font-weight:600;
}
.lp-step-list{
  display:grid;
  grid-template-columns:repeat(3,minmax(0,1fr));
  gap:18px;
}
.lp-flow-step{
  padding:22px;
}
.lp-flow-step-number{
  display:inline-flex;
  margin-bottom:12px;
  color:var(--lp-emerald);
  font-family:var(--font-fira-code),monospace;
  font-size:13px;
  font-weight:800;
}
.lp-stats{
  display:grid;
  grid-template-columns:repeat(4,minmax(0,1fr));
  gap:14px;
}
.lp-stat{
  padding:24px;
  border-radius:20px;
  background:var(--lp-surface);
  border:1px solid var(--lp-border);
  box-shadow:var(--lp-soft-shadow);
}
.lp-stat strong{
  display:block;
  margin-bottom:8px;
  color:var(--lp-emerald);
  font-size:40px;
  line-height:1;
  letter-spacing:-.06em;
  font-weight:900;
}
.lp-stat span{
  color:var(--lp-soft);
  font-size:14px;
  line-height:1.6;
}
.lp-code-shell{
  display:grid;
  grid-template-columns:minmax(0,.86fr) minmax(0,1.14fr);
  gap:22px;
  align-items:start;
}
.lp-code-left{
  padding:24px;
  border-radius:24px;
  background:var(--lp-surface);
  border:1px solid var(--lp-border);
  box-shadow:var(--lp-soft-shadow);
}
.lp-code-left ul{
  list-style:none;
  display:grid;
  gap:16px;
  margin-top:22px;
}
.lp-code-left li{
  display:flex;
  gap:12px;
  align-items:flex-start;
}
.lp-code-left li svg{
  color:var(--lp-emerald);
  margin-top:3px;
}
.lp-code-left li strong{
  display:block;
  font-size:15px;
  margin-bottom:4px;
}
.lp-code-left li span{
  color:var(--lp-muted);
  font-size:13.5px;
  line-height:1.6;
}
.lp-code-right{
  overflow:hidden;
  border-radius:24px;
  border:1px solid var(--lp-border);
  box-shadow:var(--lp-shadow);
}
.lp-code-tabs-bar{
  display:flex;
  gap:6px;
  padding:14px 16px;
  background:var(--lp-surface);
  border-bottom:1px solid var(--lp-border);
}
.lp-code-tab{
  border:1px solid transparent;
  background:transparent;
  color:var(--lp-muted);
  padding:7px 12px;
  border-radius:999px;
  font-family:var(--font-fira-code),monospace;
  font-size:12px;
  font-weight:700;
  cursor:pointer;
}
.lp-code-tab.active{
  color:var(--lp-text);
  background:var(--lp-surface-2);
  border-color:var(--lp-border);
}
.lp-editor{
  background:var(--lp-code-bg);
}
.lp-editor-code{
  padding:18px 0 22px;
  margin:0;
  overflow:auto;
  font-family:var(--font-fira-code),monospace;
  font-size:13px;
  line-height:1.75;
}
.lp-editor-line{
  display:flex;
  padding-right:18px;
}
.lp-editor-line:hover{background:rgba(255,255,255,.03)}
.lp-editor-ln{
  width:48px;
  padding-right:14px;
  text-align:right;
  color:var(--lp-code-gutter);
  flex-shrink:0;
  user-select:none;
}
.lp-editor-text{
  color:var(--lp-code-text);
  white-space:pre;
}
.lp-modes-grid{
  display:grid;
  grid-template-columns:repeat(2,minmax(0,1fr));
  gap:18px;
}
.lp-mode-card{
  padding:30px;
  box-shadow:var(--lp-soft-shadow);
}
.lp-mode-badge{
  display:inline-flex;
  align-items:center;
  gap:8px;
  padding:8px 14px;
  border-radius:999px;
  margin-bottom:18px;
  font-family:var(--font-fira-code),monospace;
  font-size:12px;
  font-weight:800;
  text-transform:uppercase;
  letter-spacing:.08em;
}
.lp-mode-card ul{
  list-style:none;
  display:grid;
  gap:12px;
  margin:20px 0 24px;
}
.lp-mode-card li{
  display:flex;
  gap:10px;
  color:var(--lp-soft);
  font-size:14px;
  line-height:1.65;
}
.lp-mode-card li svg{
  margin-top:4px;
  flex-shrink:0;
}
.lp-faq-grid{
  display:grid;
  grid-template-columns:repeat(2,minmax(0,1fr));
  gap:18px;
}
.lp-faq-item{
  padding:24px;
}
.lp-cta{
  padding-top:var(--lp-section);
}
.lp-cta-panel{
  position:relative;
  overflow:hidden;
  border-radius:32px;
  border:1px solid var(--lp-border);
  background:linear-gradient(135deg, rgba(52,211,153,.11), rgba(56,189,248,.10) 45%, var(--lp-surface) 100%);
  padding:44px 40px;
  box-shadow:var(--lp-shadow);
}
.lp-cta-panel h2{
  max-width:760px;
  font-size:clamp(34px,4vw,58px);
  line-height:1.02;
  letter-spacing:-.05em;
  font-weight:900;
  margin-bottom:14px;
}
.lp-cta-panel p{
  max-width:720px;
  color:var(--lp-soft);
  font-size:17px;
  line-height:1.75;
}
.lp-cta-actions{
  display:flex;
  gap:12px;
  flex-wrap:wrap;
  margin-top:28px;
}
.lp-footnote{
  margin-top:18px;
  color:var(--lp-muted);
  font-size:13px;
  line-height:1.7;
}
.lp-footnote a{
  color:var(--lp-text);
}
@media (max-width:1200px){
  :root{--lp-pad:24px}
  .lp-hero-shell,
  .lp-code-shell{
    grid-template-columns:1fr;
  }
  .lp-hero-visual{
    min-height:480px;
  }
}
@media (max-width:980px){
  :root{--lp-section:88px}
  .lp-nav-links{display:none}
  .lp-hero-shell{padding:32px}
  .lp-primitives-grid,
  .lp-usecase-grid,
  .lp-modes-grid,
  .lp-faq-grid,
  .lp-stats,
  .lp-step-list{
    grid-template-columns:1fr 1fr;
  }
  .lp-hero-proof{
    grid-template-columns:1fr;
  }
}
@media (max-width:720px){
  :root{--lp-pad:18px;--lp-section:72px}
  .lp-page{padding-bottom:88px}
  .lp-nav-inner{height:58px;display:flex;justify-content:space-between}
  .lp-hero{
    padding-top:24px;
  }
  .lp-hero-shell{
    padding:24px 20px;
    border-radius:24px;
  }
  .lp-hero-title{
    font-size:42px;
  }
  .lp-hero-sub{
    font-size:16px;
  }
  .lp-hero-visual{
    min-height:440px;
  }
  .lp-visual-card-main,
  .lp-visual-card-secondary,
  .lp-visual-card-float{
    position:relative;
    width:100%;
    left:auto;
    right:auto;
    top:auto;
    bottom:auto;
  }
  .lp-visual-stack{
    display:grid;
    gap:12px;
    position:static;
  }
  .lp-primitives-grid,
  .lp-usecase-grid,
  .lp-modes-grid,
  .lp-faq-grid,
  .lp-stats,
  .lp-step-list{
    grid-template-columns:1fr;
  }
  .lp-section-head h2{
    font-size:34px;
  }
  .lp-cta-panel{
    padding:28px 22px;
    border-radius:24px;
  }
}
`;

export default function LandingPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <LandingAttribution />

      <div className="lp-shell">
        <nav className="lp-nav">
          <div className="lp-nav-inner">
            <Link href="/" aria-label="UniPost home" style={{ textDecoration: "none" }}>
              <UniPostLogo markSize={28} wordmarkColor="var(--lp-text)" />
            </Link>

            <div className="lp-nav-links">
              <Link href="/solutions" className="lp-nav-link">Solutions</Link>
              <Link href="/tools" className="lp-nav-link">Tools</Link>
              <Link href="/pricing" className="lp-nav-link">Pricing</Link>
              <div className="lp-nav-dropdown">
                <button className="lp-nav-link lp-nav-dropdown-trigger">
                  Docs
                  <svg width="10" height="10" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" style={{ marginLeft: 5 }}>
                    <path d="M4 6l4 4 4-4" />
                  </svg>
                </button>
                <div className="lp-nav-dropdown-menu">
                  <Link href="/docs/quickstart" className="lp-nav-dropdown-item">
                    <span className="lp-nav-dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /><line x1="16" y1="13" x2="8" y2="13" /><line x1="16" y1="17" x2="8" y2="17" /></svg></span>
                    <span><div className="lp-nav-dropdown-label">Quickstart</div><div className="lp-nav-dropdown-desc">Go from API key to your first published post.</div></span>
                  </Link>
                  <Link href="/docs/sdk" className="lp-nav-dropdown-item">
                    <span className="lp-nav-dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="M9 18h6" /><path d="M10 22h4" /><path d="M12 2a7 7 0 0 0-4 12.75V17a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1v-2.25A7 7 0 0 0 12 2Z" /></svg></span>
                    <span><div className="lp-nav-dropdown-label">SDKs</div><div className="lp-nav-dropdown-desc">Use UniPost from JavaScript, Python, and Go.</div></span>
                  </Link>
                  <Link href="/docs/mcp" className="lp-nav-dropdown-item">
                    <span className="lp-nav-dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><circle cx="12" cy="12" r="3" /><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" /></svg></span>
                    <span><div className="lp-nav-dropdown-label">MCP Server</div><div className="lp-nav-dropdown-desc">Connect AI agents to UniPost tools and workflows.</div></span>
                  </Link>
                  <Link href="/docs/api" className="lp-nav-dropdown-item">
                    <span className="lp-nav-dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="M8 6h12" /><path d="M8 12h12" /><path d="M8 18h12" /><path d="M3 6h.01" /><path d="M3 12h.01" /><path d="M3 18h.01" /></svg></span>
                    <span><div className="lp-nav-dropdown-label">API Reference</div><div className="lp-nav-dropdown-desc">Inspect endpoints, schemas, and example payloads.</div></span>
                  </Link>
                </div>
              </div>
            </div>

            <MarketingNav />
          </div>
        </nav>

        <main className="lp-page">
          <section className="lp-hero">
            <div className="lp-hero-shell">
              <div className="lp-hero-copy">
                <div className="lp-eyebrow">
                  <span className="lp-eyebrow-dot" />
                  Unified social media API for apps and AI agents
                </div>
                <h1 className="lp-hero-title">
                  Add social publishing to your product,
                  <strong>not your roadmap.</strong>
                </h1>
                <p className="lp-hero-sub">
                  UniPost gives your app one place to onboard customer social accounts, validate drafts, upload media,
                  publish platform-specific posts, and monitor delivery across X, Bluesky, LinkedIn, Instagram, Threads,
                  TikTok, YouTube, and Pinterest.
                </p>
                <div className="lp-hero-actions">
                  <MarketingCTA className="lp-btn lp-btn-primary" />
                  <Link href="/docs" className="lp-btn lp-btn-outline">View Docs</Link>
                  <Link href="/pricing" className="lp-btn lp-btn-subtle">See Pricing</Link>
                </div>
                <div className="lp-hero-proof">
                  {HERO_POINTS.map((point) => (
                    <div key={point} className="lp-hero-proof-card">
                      <CheckIcon />
                      <div className="lp-hero-proof-label">{point}</div>
                    </div>
                  ))}
                  <div className="lp-hero-proof-card">
                    <CheckIcon />
                    <div className="lp-hero-proof-label">Free plan includes 100 posts per month with no credit card.</div>
                  </div>
                </div>
              </div>

              <div className="lp-hero-visual" aria-hidden="true">
                <div className="lp-visual-stack">
                  <div className="lp-visual-card lp-visual-card-main">
                    <div className="lp-visual-topline">
                      <strong>Product workflow</strong>
                      <span className="lp-visual-pill">One API</span>
                    </div>
                    <div className="lp-visual-lanes">
                      <div className="lp-lane">
                        <div className="lp-lane-label">
                          <span>Connect end-user accounts</span>
                          <span style={{ color: "var(--lp-emerald)" }}>hosted OAuth</span>
                        </div>
                        <div className="lp-lane-sub">Create a Connect session, map it to your own external user, and keep the OAuth complexity out of your frontend.</div>
                      </div>
                      <div className="lp-lane">
                        <div className="lp-lane-label">
                          <span>Shape content per platform</span>
                          <span style={{ color: "var(--lp-sky)" }}>platform_posts[]</span>
                        </div>
                        <div className="lp-lane-sub">Use different captions, media, and platform options instead of forcing one generic payload everywhere.</div>
                        <div className="lp-lane-platforms">
                          {PLATFORMS.slice(0, 5).map((platform) => (
                            <span key={platform.name} className="lp-mini-chip">
                              {PLATFORM_ICONS[platform.name]}
                              {platform.name}
                            </span>
                          ))}
                        </div>
                      </div>
                      <div className="lp-lane">
                        <div className="lp-lane-label">
                          <span>Observe delivery and health</span>
                          <span style={{ color: "var(--lp-amber)" }}>ops ready</span>
                        </div>
                        <div className="lp-lane-sub">Watch job state, handle retries, reconnect failed accounts, and surface health back into your own product.</div>
                      </div>
                    </div>
                  </div>

                  <div className="lp-visual-card lp-visual-card-secondary">
                    <div className="lp-visual-topline">
                      <strong>Delivery snapshot</strong>
                      <span className="lp-visual-pill">Live</span>
                    </div>
                    <div className="lp-signal-list">
                      <div className="lp-signal-item">
                        <div className="lp-signal-meta">
                          <strong>LinkedIn launch post</strong>
                          <span>validated · scheduled · delivered</span>
                        </div>
                        <span className="lp-signal-badge ok">ok</span>
                      </div>
                      <div className="lp-signal-item">
                        <div className="lp-signal-meta">
                          <strong>Pinterest campaign</strong>
                          <span>media uploaded · waiting on board selection</span>
                        </div>
                        <span className="lp-signal-badge warn">review</span>
                      </div>
                      <div className="lp-signal-item">
                        <div className="lp-signal-meta">
                          <strong>Threads draft</strong>
                          <span>preview shared with customer before publish</span>
                        </div>
                        <span className="lp-signal-badge ok">ready</span>
                      </div>
                    </div>
                  </div>

                  <div className="lp-visual-card lp-visual-card-float">
                    <div className="lp-code-card">
                      <div className="lp-code-card-head">
                        <span className="lp-code-dot" />
                        <span className="lp-code-dot" />
                        <span className="lp-code-dot" />
                      </div>
                      <div className="lp-code-card-body">{`POST /v1/posts
{
  "platform_posts": [
    { "account_id": "sa_ln_01" },
    { "account_id": "sa_pin_02" }
  ]
}`}</div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div className="lp-platforms">
              <div className="lp-platforms-label">Supported Platforms</div>
              <div className="lp-platform-row">
                {PLATFORMS.map((platform) => (
                  <Link key={platform.name} href={`/${platform.slug}-api`} className="lp-platform-chip">
                    {PLATFORM_ICONS[platform.name]}
                    {platform.name}
                  </Link>
                ))}
              </div>
            </div>
          </section>

          <section className="lp-section">
            <div className="lp-section-head">
              <div className="lp-eyebrow"><span className="lp-eyebrow-dot" />Built for real product flows</div>
              <h2>Not a posting wrapper. A full social layer.</h2>
              <p>
                The hard part is not firing a single request. It is safely onboarding accounts, shaping content per
                platform, and staying operational when your users publish at scale.
              </p>
            </div>
            <div className="lp-usecase-grid">
              {USE_CASES.map((item) => (
                <div key={item.title} className="lp-usecase-card">
                  <div className="lp-card-eyebrow">{item.eyebrow}</div>
                  <h3>{item.title}</h3>
                  <p>{item.body}</p>
                </div>
              ))}
            </div>
          </section>

          <section className="lp-section">
            <div className="lp-section-head">
              <div className="lp-eyebrow"><span className="lp-eyebrow-dot" />Core primitives</div>
              <h2>The pieces you actually need to ship.</h2>
              <p>
                UniPost covers the boring-but-critical surfaces that usually leak into your app architecture once
                customer accounts, local media, or AI-generated content enter the picture.
              </p>
            </div>
            <div className="lp-primitives-grid">
              {PRIMITIVES.map((item) => (
                <div key={item.title} className="lp-primitive-card">
                  <h3>{item.title}</h3>
                  <p>{item.desc}</p>
                  <div className="lp-chip-row">
                    {item.chips.map((chip) => (
                      <span key={chip} className="lp-chip">{chip}</span>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </section>

          <section className="lp-section">
            <div className="lp-section-head">
              <div className="lp-eyebrow"><span className="lp-eyebrow-dot" />How it fits</div>
              <h2>One clean workflow from onboarding to delivery.</h2>
              <p>
                The winning flow is not “generate one caption and spray it everywhere.” It is connecting the user,
                validating the payload, then publishing with visibility into what happened next.
              </p>
            </div>
            <div className="lp-step-list">
              {FLOW_STEPS.map((step) => (
                <div key={step.step} className="lp-flow-step">
                  <span className="lp-flow-step-number">{step.step}</span>
                  <h3>{step.title}</h3>
                  <p>{step.body}</p>
                </div>
              ))}
            </div>
          </section>

          <section className="lp-section">
            <div className="lp-stats">
              {STATS.map((stat) => (
                <div key={stat.label} className="lp-stat">
                  <strong>{stat.number}</strong>
                  <span>{stat.label}</span>
                </div>
              ))}
            </div>
          </section>

          <section className="lp-section">
            <div className="lp-section-head">
              <div className="lp-eyebrow"><span className="lp-eyebrow-dot" />Developer experience</div>
              <h2>Built for per-platform control, not one generic caption.</h2>
              <p>
                Keep one API shape while still giving your product or agent enough structure to send different content
                to different destinations, handle local media, and stay idempotent.
              </p>
            </div>
            <div className="lp-code-shell">
              <div className="lp-code-left">
                <h3 style={{ fontSize: 28, lineHeight: 1.1, letterSpacing: "-.04em", fontWeight: 850 }}>
                  The API surface matches how modern products actually publish.
                </h3>
                <ul>
                  <li>
                    <CheckIcon />
                    <div>
                      <strong>Per-platform payloads</strong>
                      <span>Use <code>platform_posts[]</code> when different networks need different captions, media, or options.</span>
                    </div>
                  </li>
                  <li>
                    <CheckIcon />
                    <div>
                      <strong>Media that starts local</strong>
                      <span>Reserve uploads with <code>POST /v1/media</code>, then publish with returned <code>media_ids</code>.</span>
                    </div>
                  </li>
                  <li>
                    <CheckIcon />
                    <div>
                      <strong>Safer automation</strong>
                      <span>Validation, preview links, idempotency keys, and structured result data make agentic workflows much less brittle.</span>
                    </div>
                  </li>
                </ul>
              </div>
              <div className="lp-code-right">
                <LandingCodeTabs />
              </div>
            </div>
          </section>

          <section className="lp-section">
            <div className="lp-section-head">
              <div className="lp-eyebrow"><span className="lp-eyebrow-dot" />Adoption path</div>
              <h2>Start quickly. Then take ownership of the surface.</h2>
              <p>
                UniPost lets you validate the posting experience early, then graduate into your own branded OAuth and
                credential stack when the product is ready.
              </p>
            </div>
            <div className="lp-modes-grid">
              {MODES.map((mode, index) => (
                <div key={mode.badge} className="lp-mode-card">
                  <div
                    className="lp-mode-badge"
                    style={{
                      background: `${mode.badgeColor}18`,
                      color: mode.badgeColor,
                      border: `1px solid ${mode.badgeColor}30`,
                    }}
                  >
                    {mode.badge}
                  </div>
                  <h3>{mode.title}</h3>
                  <p>{mode.desc}</p>
                  <ul>
                    {mode.features.map((feature) => (
                      <li key={feature}>
                        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke={mode.badgeColor} strokeWidth="2.2"><path d="M3 8l4 4 6-7" /></svg>
                        <span>{feature}</span>
                      </li>
                    ))}
                  </ul>
                  {index === 0 ? <MarketingCTA className="lp-btn lp-btn-primary" /> : <MarketingCTALight />}
                </div>
              ))}
            </div>
          </section>

          <section className="lp-section">
            <div className="lp-section-head">
              <div className="lp-eyebrow"><span className="lp-eyebrow-dot" />FAQ</div>
              <h2>Questions teams usually ask before they integrate.</h2>
            </div>
            <div className="lp-faq-grid">
              {FAQS.map((item) => (
                <div key={item.q} className="lp-faq-item">
                  <h3>{item.q}</h3>
                  <p>{item.a}</p>
                </div>
              ))}
            </div>
          </section>

          <section className="lp-cta">
            <div className="lp-cta-panel">
              <h2>Build the social layer once, then keep shipping product.</h2>
              <p>
                Start on the free plan, wire up Connect and publish flows, and move into white-label onboarding and
                higher volume when your product is ready.
              </p>
              <div className="lp-cta-actions">
                <MarketingCTA className="lp-btn lp-btn-primary" />
                <Link href="/docs/quickstart" className="lp-btn lp-btn-outline">Start with Quickstart</Link>
              </div>
              <div className="lp-footnote">
                Want to inspect the agent path too? Try <Link href="/tools/agentpost">AgentPost</Link> or browse the{" "}
                <Link href="/compare">comparison pages</Link>.
              </div>
            </div>
          </section>
        </main>
      </div>
    </>
  );
}
