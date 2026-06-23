import type { Metadata } from "next";
import Link from "next/link";
import {
  ArrowDown,
  ArrowRight,
  BookOpen,
  CheckCircle2,
  KeyRound,
  Plug,
} from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";
import { PublicSiteHeader } from "@/components/marketing/nav";

const APP_URL = process.env.NEXT_PUBLIC_APP_URL || "https://app.unipost.dev";
const START_BUILDING_URL = `${APP_URL}/welcome`;

export const metadata: Metadata = {
  title: "UniPost | Unified Social Media Posting API for Developers",
  description:
    "UniPost is a unified social media posting API for developers building customer account connection, media uploads, scheduling, webhooks, analytics, inbox, and delivery across nine social platforms.",
  alternates: {
    canonical: "https://unipost.dev/",
  },
  openGraph: {
    title: "UniPost | Unified Social Media Posting API for Developers",
    description:
      "Build social publishing into your product with one API for account connection, media uploads, scheduling, webhooks, analytics, inbox, and delivery across nine social platforms.",
    url: "https://unipost.dev/",
    siteName: "UniPost",
    type: "website",
  },
};

const PLATFORMS = [
  { name: "X", key: "twitter" },
  { name: "LinkedIn", key: "linkedin" },
  { name: "Instagram", key: "instagram" },
  { name: "TikTok", key: "tiktok" },
  { name: "Threads", key: "threads" },
  { name: "YouTube", key: "youtube" },
  { name: "Facebook", key: "facebook" },
  { name: "Pinterest", key: "pinterest" },
  { name: "Bluesky", key: "bluesky" },
] as const;

const WHY_ITEMS = [
  "Different OAuth flows",
  "Different media rules",
  "Different rate limits",
  "Different publishing APIs",
];

const HOW_STEPS = [
  {
    title: "Get API Key",
    body: "Create a UniPost API key from your workspace and use it to authenticate every publish, media, and account request.",
  },
  {
    title: "Connect accounts",
    body: "Send customers through hosted OAuth flows, then store the returned connected account IDs in your product.",
  },
  {
    title: "Publish content",
    body: "Submit text, media, or per-platform variants in one request and let UniPost handle validation and delivery.",
  },
];

const API_SURFACE = [
  {
    area: "Connect",
    method: "POST",
    path: "/v1/connect/sessions",
    href: "/docs/api/connect/sessions/create",
    body: "Create a hosted OAuth session for customer account onboarding.",
  },
  {
    area: "Posts",
    method: "POST",
    path: "/v1/posts",
    href: "/docs/api/posts/create",
    body: "Publish, schedule, or draft content across connected destinations.",
  },
  {
    area: "Analytics",
    method: "GET",
    path: "/v1/analytics/summary",
    href: "/docs/api/analytics/summary",
    body: "Fetch workspace-wide reporting totals and trend breakdowns.",
  },
  {
    area: "Webhooks",
    method: "POST",
    path: "/v1/webhooks",
    href: "/docs/api/webhooks/create",
    body: "Receive publish outcomes and account lifecycle events without polling.",
  },
  {
    area: "API keys",
    method: "POST",
    path: "/v1/api-keys",
    href: "/docs/api/api-keys/create",
    body: "Create workspace keys for server-side UniPost API access.",
  },
  {
    area: "Inbox",
    method: "GET",
    path: "/v1/inbox",
    href: "/docs/api/inbox",
    body: "Read comments, DMs, and reply workflows across supported social channels.",
  },
] as const;

const PUBLISH_SNIPPET = `await fetch("https://api.unipost.dev/v1/posts", {
  method: "POST",
  headers: {
    "Authorization": \`Bearer \${UNIPOST_API_KEY}\`,
    "Content-Type": "application/json"
  },
  body: JSON.stringify({
    caption: "Launching today",
    account_ids: [
      "sa_x_123",
      "sa_linkedin_456",
      "sa_threads_789"
    ]
  })
});`;

const CSS = `
:root{
  --lp-bg:var(--app-bg);
  --lp-surface:var(--marketing-surface);
  --lp-surface-alt:var(--marketing-surface-alt);
  --lp-surface-elevated:var(--marketing-surface-elevated);
  --lp-border:var(--marketing-border);
  --lp-border-strong:var(--marketing-border-strong);
  --lp-text:var(--marketing-text);
  --lp-muted:var(--marketing-muted);
  --lp-subtle:var(--marketing-subtle);
  --lp-link:var(--marketing-link);
  --lp-link-hover:var(--marketing-link-hover);
  --lp-success:var(--marketing-auth-primary-bg);
  --lp-shadow:var(--marketing-shadow-soft);
  --lp-shadow-lg:var(--marketing-shadow-lg);
  --lp-content:1180px;
  --lp-wide:1320px;
  --lp-pad:32px;
  --lp-radius:8px;
  --lp-mono:var(--font-fira-code), ui-monospace, SFMono-Regular, Menlo, monospace;
  --lp-ui:var(--font-dm-sans), system-ui, sans-serif;
  --lp-ease:cubic-bezier(.16,1,.3,1);
}
*{box-sizing:border-box}
body{
  margin:0;
  background:var(--lp-bg);
  color:var(--lp-text);
  font-family:var(--lp-ui);
  -webkit-font-smoothing:antialiased;
}
.lp-shell{
  min-height:100vh;
  overflow-x:hidden;
  background:
    radial-gradient(circle at 82% 4%, color-mix(in srgb, var(--lp-link) 7%, transparent), transparent 34rem),
    linear-gradient(180deg, color-mix(in srgb, var(--lp-surface-alt) 52%, var(--lp-bg)) 0%, var(--lp-bg) 50%);
}
.lp-main{
  width:100%;
}
.lp-section{
  width:100%;
  padding:104px var(--lp-pad);
}
.lp-section.compact{
  padding-top:64px;
  padding-bottom:64px;
}
.lp-inner{
  max-width:var(--lp-content);
  margin:0 auto;
}
.lp-wide-inner{
  max-width:var(--lp-wide);
  margin:0 auto;
}
.lp-eyebrow{
  display:inline-flex;
  align-items:center;
  gap:8px;
  margin:0 0 16px;
  color:var(--lp-link);
  font-family:var(--lp-mono);
  font-size:12px;
  font-weight:700;
  text-transform:uppercase;
}
.lp-eyebrow::before{
  content:"";
  width:7px;
  height:7px;
  border-radius:50%;
  background:var(--lp-link);
  box-shadow:0 0 0 5px color-mix(in srgb, var(--lp-link) 12%, transparent);
}
.lp-section-head{
  max-width:760px;
  margin:0 0 34px;
  text-align:left;
}
.lp-section-head.center{
  margin-left:auto;
  margin-right:auto;
  text-align:center;
}
.lp-section-head.center .lp-eyebrow{
  justify-content:center;
}
.lp-section-head.center p{
  margin-left:auto;
  margin-right:auto;
}
.lp-section-head h2{
  margin:0;
  font-size:clamp(32px, 4vw, 48px);
  line-height:1.12;
  letter-spacing:0;
  font-weight:800;
}
.lp-section-head p{
  margin:16px 0 0;
  color:var(--lp-muted);
  font-size:17px;
  line-height:1.65;
}
.lp-btn{
  display:inline-flex;
  align-items:center;
  justify-content:center;
  gap:8px;
  min-height:46px;
  padding:11px 18px;
  border-radius:var(--lp-radius);
  border:1px solid transparent;
  font-family:var(--lp-ui);
  font-size:14px;
  font-weight:700;
  text-decoration:none;
  white-space:nowrap;
  max-width:100%;
  transition:background .22s var(--lp-ease), border-color .22s var(--lp-ease), color .22s var(--lp-ease), transform .22s var(--lp-ease), box-shadow .22s var(--lp-ease);
}
.lp-btn:hover{
  transform:translateY(-1px);
}
.lp-btn:active{
  transform:translateY(0) scale(.98);
}
.lp-btn-primary{
  background:var(--lp-link);
  color:#fff;
  box-shadow:0 14px 28px color-mix(in srgb, var(--lp-link) 16%, transparent);
}
.lp-btn-primary:hover{
  background:var(--lp-link-hover);
}
.lp-btn-primary svg{
  color:currentColor;
}
.lp-btn-outline{
  background:var(--lp-surface);
  color:var(--lp-text);
  border-color:var(--lp-border-strong);
}
.lp-btn-outline:hover{
  background:var(--lp-surface-alt);
}
.lp-hero{
  min-height:min(760px, calc(100dvh - 58px));
  padding:76px var(--lp-pad) 70px;
  display:flex;
  align-items:center;
  border-bottom:1px solid var(--lp-border);
  overflow:hidden;
}
.lp-hero-inner{
  width:100%;
  max-width:var(--lp-wide);
  margin:0 auto;
  position:relative;
}
.lp-hero-grid{
  display:grid;
  grid-template-columns:minmax(0, .96fr) minmax(420px, 1.04fr);
  gap:72px;
  align-items:center;
}
.lp-hero-copy{
  position:relative;
  z-index:2;
  max-width:680px;
  text-align:left;
  padding:18px 0 24px;
}
.lp-hero h1{
  margin:0;
  font-size:clamp(44px, 5.2vw, 74px);
  line-height:1.06;
  letter-spacing:0;
  font-weight:800;
}
.lp-hero-sub{
  max-width:660px;
  margin:22px 0 0;
  color:var(--lp-muted);
  font-size:18px;
  line-height:1.7;
}
.lp-hero-actions{
  display:flex;
  align-items:center;
  justify-content:flex-start;
  gap:12px;
  flex-wrap:wrap;
  margin-top:30px;
}
.lp-hero-note{
  margin:18px 0 0;
  color:var(--lp-subtle);
  font-size:13px;
}
.lp-model{
  display:grid;
  gap:18px;
  padding:28px;
  border:1px solid var(--lp-border);
  border-radius:24px;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--lp-surface) 96%, transparent), color-mix(in srgb, var(--lp-surface-alt) 74%, transparent));
  box-shadow:var(--lp-shadow-lg);
}
.lp-model-node{
  display:flex;
  align-items:center;
  justify-content:space-between;
  gap:20px;
  min-height:84px;
  padding:20px;
  border:1px solid var(--lp-border);
  border-radius:18px;
  background:var(--lp-surface);
}
.lp-model-node.api{
  border-color:color-mix(in srgb, var(--lp-link) 44%, var(--lp-border));
  background:color-mix(in srgb, var(--lp-link) 7%, var(--lp-surface));
}
.lp-model-label{
  display:flex;
  align-items:center;
  gap:12px;
  min-width:0;
}
.lp-model-label > span:last-child{
  min-width:0;
}
.lp-model-icon{
  width:40px;
  height:40px;
  display:flex;
  align-items:center;
  justify-content:center;
  flex-shrink:0;
  border-radius:12px;
  background:var(--lp-surface-alt);
  color:var(--lp-link);
}
.lp-model-title{
  display:block;
  font-size:16px;
  font-weight:800;
}
.lp-model-copy{
  display:block;
  margin-top:4px;
  color:var(--lp-muted);
  font-size:13px;
  line-height:1.45;
}
.lp-model-arrow{
  display:flex;
  justify-content:center;
  color:var(--lp-subtle);
}
.lp-model-platforms{
  display:grid;
  grid-template-columns:repeat(4, minmax(0, 1fr));
  gap:10px;
}
.lp-model-platform{
  display:flex;
  align-items:center;
  justify-content:center;
  gap:8px;
  min-height:48px;
  padding:9px;
  border:1px solid var(--lp-border);
  border-radius:14px;
  background:var(--lp-surface);
  color:var(--lp-text);
  font-size:12.5px;
  font-weight:800;
}
.lp-hero-stats{
  display:grid;
  grid-template-columns:repeat(3, minmax(0, 1fr));
  gap:12px;
  max-width:560px;
  margin-top:30px;
}
.lp-hero-stat{
  border-top:1px solid var(--lp-border-strong);
  padding-top:14px;
}
.lp-hero-stat strong{
  display:block;
  font-family:var(--lp-mono);
  font-size:24px;
  line-height:1;
}
.lp-hero-stat span{
  display:block;
  margin-top:7px;
  color:var(--lp-muted);
  font-size:12px;
  font-weight:700;
}
.lp-platform-strip{
  display:grid;
  grid-template-columns:repeat(9, minmax(0, 1fr));
  align-items:center;
  gap:12px;
}
.lp-platform-card{
  min-height:64px;
  display:flex;
  flex-direction:row;
  align-items:center;
  justify-content:center;
  gap:8px;
  padding:12px;
  border:1px solid var(--lp-border);
  border-radius:16px;
  background:var(--lp-surface);
  color:var(--lp-text);
  font-size:13px;
  font-weight:700;
  white-space:nowrap;
}
.lp-split{
  display:grid;
  grid-template-columns:minmax(0, .88fr) minmax(420px, 1.12fr);
  gap:58px;
  align-items:center;
}
.lp-problem-list{
  display:grid;
  gap:12px;
  margin-top:24px;
}
.lp-problem-item{
  display:flex;
  align-items:center;
  gap:12px;
  padding:15px 0;
  border-top:1px solid var(--lp-border);
  font-weight:700;
}
.lp-problem-item svg{
  color:var(--lp-link);
  flex-shrink:0;
}
.lp-unifies{
  padding:30px;
  border:1px solid var(--lp-border-strong);
  border-radius:18px;
  background:var(--lp-surface-elevated);
  box-shadow:var(--lp-shadow);
}
.lp-unifies-title{
  margin:0 0 18px;
  font-size:24px;
  line-height:1.2;
  font-weight:800;
}
.lp-unifies-row{
  display:grid;
  grid-template-columns:1fr;
  align-items:center;
  gap:12px;
}
.lp-unifies-box{
  min-height:126px;
  padding:18px;
  border:1px solid var(--lp-border);
  border-radius:14px;
  background:var(--lp-surface);
}
.lp-unifies-box strong{
  display:block;
  margin-bottom:12px;
  font-size:14px;
}
.lp-unifies-box span{
  display:block;
  color:var(--lp-muted);
  font-size:13px;
  line-height:1.55;
}
.lp-unifies-arrow{
  display:none;
  color:var(--lp-link);
}
.lp-steps{
  position:relative;
  display:grid;
  grid-template-columns:repeat(3, minmax(0, 1fr));
  gap:22px;
  align-items:stretch;
}
.lp-steps::before{
  content:"";
  position:absolute;
  left:12%;
  right:12%;
  top:64px;
  height:2px;
  background:linear-gradient(90deg, transparent, color-mix(in srgb, var(--lp-link) 42%, var(--lp-border-strong)), transparent);
}
.lp-step{
  position:relative;
  min-height:238px;
  padding:26px;
  border:1px solid var(--lp-border);
  border-radius:18px;
  background:var(--lp-surface);
  box-shadow:var(--lp-shadow);
}
.lp-step-number{
  width:62px;
  height:62px;
  display:flex;
  align-items:center;
  justify-content:center;
  margin-bottom:22px;
  border:1px solid color-mix(in srgb, var(--lp-link) 34%, var(--lp-border));
  border-radius:18px;
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--lp-link) 16%, transparent), transparent 62%),
    var(--lp-surface-alt);
  font-family:var(--lp-mono);
  color:color-mix(in srgb, var(--lp-link) 75%, var(--lp-text));
  font-size:28px;
  line-height:1;
  font-weight:700;
}
.lp-step h3{
  margin:0;
  font-size:18px;
  line-height:1.25;
  font-weight:800;
}
.lp-step p{
  margin:12px 0 0;
  color:var(--lp-muted);
  font-size:14px;
  line-height:1.6;
}
.lp-api-section{
  padding-top:28px;
  padding-bottom:72px;
}
.lp-api-section .lp-section-head{
  margin-bottom:24px;
}
.lp-api-list{
  max-width:980px;
  margin:0 auto;
  display:grid;
  gap:9px;
}
.lp-api-row{
  display:grid;
  grid-template-columns:140px minmax(230px, .78fr) minmax(0, 1fr) 24px;
  align-items:center;
  gap:18px;
  padding:15px 22px;
  border:1px solid var(--lp-border);
  border-radius:16px;
  background:var(--lp-surface);
  color:var(--lp-text);
  text-decoration:none;
  box-shadow:none;
  transition:transform .22s var(--lp-ease), border-color .22s var(--lp-ease), background .22s var(--lp-ease);
}
.lp-api-row:hover{
  transform:translateY(-2px);
  border-color:var(--lp-border-strong);
  background:var(--lp-surface-elevated);
}
.lp-api-area{
  color:var(--lp-muted);
  font-family:var(--lp-mono);
  font-size:12px;
  font-weight:800;
  text-transform:uppercase;
}
.lp-api-endpoint{
  display:flex;
  align-items:center;
  gap:10px;
  min-width:0;
  font-family:var(--lp-mono);
  font-size:17px;
  font-weight:700;
}
.lp-api-method{
  display:inline-flex;
  align-items:center;
  justify-content:center;
  min-width:54px;
  padding:5px 8px;
  border-radius:8px;
  background:color-mix(in srgb, var(--lp-link) 11%, var(--lp-surface-alt));
  color:var(--lp-link);
  font-size:12px;
  line-height:1;
}
.lp-api-method.post{
  background:color-mix(in srgb, var(--lp-success) 13%, var(--lp-surface-alt));
  color:color-mix(in srgb, var(--lp-success) 82%, var(--lp-text));
}
.lp-api-method.planned{
  background:var(--lp-surface-alt);
  color:var(--lp-muted);
}
.lp-api-path{
  overflow:hidden;
  text-overflow:ellipsis;
  white-space:nowrap;
}
.lp-api-body{
  margin:0;
  color:var(--lp-muted);
  font-size:14px;
  line-height:1.45;
}
.lp-api-link{
  display:flex;
  justify-content:flex-end;
  color:var(--lp-subtle);
}
.lp-code-layout{
  display:grid;
  grid-template-columns:minmax(0, .82fr) minmax(420px, 1.18fr);
  gap:28px;
  align-items:stretch;
}
.lp-code-copy{
  padding:32px;
  border:1px solid var(--lp-border);
  border-radius:18px;
  background:var(--lp-surface);
}
.lp-code-copy h2{
  margin:0;
  font-size:36px;
  line-height:1.15;
}
.lp-code-copy p{
  margin:16px 0 0;
  color:var(--lp-muted);
  font-size:16px;
  line-height:1.65;
}
.lp-code-window{
  min-width:0;
  overflow:hidden;
  border:1px solid #1e293b;
  border-radius:18px;
  background:#0f172a;
  box-shadow:var(--lp-shadow);
}
.lp-code-head{
  display:flex;
  align-items:center;
  justify-content:space-between;
  gap:12px;
  min-height:44px;
  padding:0 16px;
  border-bottom:1px solid #1e293b;
  color:#94a3b8;
  font-family:var(--lp-mono);
  font-size:12px;
}
.lp-code-dots{
  display:flex;
  gap:7px;
}
.lp-code-dots span{
  width:10px;
  height:10px;
  border-radius:50%;
  background:#334155;
}
.lp-code pre{
  display:block;
  margin:0;
  padding:22px;
  color:#dbeafe;
  font-family:var(--lp-mono);
  font-size:13px;
  line-height:1.75;
  overflow:auto;
  white-space:pre;
}
.lp-cta{
  padding:72px var(--lp-pad) 104px;
}
.lp-cta-inner{
  max-width:940px;
  margin:0 auto;
  padding:46px;
  border:1px solid var(--lp-border);
  border-radius:18px;
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--lp-link) 8%, transparent), transparent 56%),
    var(--lp-surface);
  box-shadow:var(--lp-shadow);
  text-align:center;
}
.lp-cta-inner .lp-eyebrow{
  justify-content:center;
}
.lp-cta h2{
  margin:0;
  font-size:46px;
  line-height:1.1;
  letter-spacing:0;
}
.lp-cta p{
  max-width:680px;
  margin:16px auto 0;
  color:var(--lp-muted);
  font-size:17px;
  line-height:1.65;
}
.lp-cta-actions{
  display:flex;
  justify-content:center;
  gap:12px;
  flex-wrap:wrap;
  margin-top:28px;
}
.lp-code-actions{
  justify-content:flex-start;
  margin-top:24px;
}
@media (max-width:1100px){
  .lp-hero{min-height:auto}
  .lp-hero-grid,
  .lp-split,
  .lp-api-row,
  .lp-code-layout{grid-template-columns:1fr}
  .lp-hero-copy{max-width:780px}
  .lp-platform-strip{grid-template-columns:repeat(3, minmax(0, 1fr))}
  .lp-steps{grid-template-columns:1fr}
  .lp-steps::before{display:none}
  .lp-api-link{justify-content:flex-start}
}
@media (max-width:760px){
  :root{--lp-pad:20px}
  .lp-section{padding-top:64px;padding-bottom:64px}
  .lp-hero{padding-top:44px;padding-bottom:42px}
  .lp-hero h1{font-size:clamp(38px, 11vw, 48px)}
  .lp-hero-sub{font-size:17px}
  .lp-hero-stats{grid-template-columns:1fr 1fr}
  .lp-hero-actions .lp-btn{flex:1 1 160px}
  .lp-model{padding:18px;border-radius:18px}
  .lp-model-platforms{grid-template-columns:repeat(2, minmax(0, 1fr))}
  .lp-section-head h2,
  .lp-code-copy h2,
  .lp-cta h2{font-size:32px}
  .lp-steps,
  .lp-platform-strip{grid-template-columns:1fr}
  .lp-platform-strip{grid-template-columns:repeat(2, minmax(0, 1fr))}
  .lp-model-platforms{grid-template-columns:repeat(2, minmax(0, 1fr))}
  .lp-code-layout{grid-template-columns:1fr}
  .lp-api-row{padding:17px}
  .lp-api-endpoint{
    align-items:flex-start;
    flex-direction:column;
    font-size:15px;
  }
  .lp-api-path{
    max-width:100%;
    white-space:normal;
    overflow-wrap:anywhere;
  }
  .lp-code pre{font-size:12px}
  .lp-step:nth-child(2),
  .lp-step:nth-child(3){margin-top:0}
  .lp-cta-inner{padding:32px 24px}
}
@media (max-width:480px){
  .lp-hero-actions{align-items:stretch}
  .lp-hero-actions .lp-btn{width:100%}
  .lp-hero-stats{grid-template-columns:1fr}
  .lp-model-node{align-items:flex-start;flex-direction:column;gap:12px}
  .lp-model-platform{justify-content:flex-start}
  .lp-platform-strip{grid-template-columns:1fr}
  .lp-code-copy{padding:24px}
}
@media (prefers-reduced-motion:reduce){
  .lp-btn,
  .lp-step{transition:none}
}
`;

export default function LandingPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <div className="lp-shell">
        <PublicSiteHeader />
        <main className="lp-main">
          <section className="lp-hero">
            <div className="lp-hero-inner">
              <div className="lp-hero-grid">
                <div className="lp-hero-copy">
                  <p className="lp-eyebrow">Unified social publishing API</p>
                  <h1>Post to every social platform with one API</h1>
                  <p className="lp-hero-sub">
                    Connect customer accounts, upload media, and publish to X, LinkedIn,
                    Instagram, TikTok, Threads, YouTube, and more through one unified API.
                  </p>
                  <div className="lp-hero-actions">
                    <a href={START_BUILDING_URL} className="lp-btn lp-btn-primary">
                      Start Building
                      <ArrowRight size={17} />
                    </a>
                    <Link href="/docs" className="lp-btn lp-btn-outline">
                      <BookOpen size={17} />
                      View Docs
                    </Link>
                  </div>
                  <p className="lp-hero-note">Built for developers adding social publishing to apps, workflows, and agents.</p>
                </div>

                <div className="lp-model" aria-label="UniPost publishing model">
                  <div className="lp-model-node">
                    <div className="lp-model-label">
                      <span className="lp-model-icon"><Plug size={20} /></span>
                      <span>
                        <span className="lp-model-title">Your app</span>
                        <span className="lp-model-copy">Scheduling tools, SaaS products, internal workflows, and agents.</span>
                      </span>
                    </div>
                  </div>
                  <div className="lp-model-arrow"><ArrowDown size={22} /></div>
                  <div className="lp-model-node api">
                    <div className="lp-model-label">
                      <span className="lp-model-icon"><KeyRound size={20} /></span>
                      <span>
                        <span className="lp-model-title">UniPost API</span>
                        <span className="lp-model-copy">Connect accounts, upload media, publish posts, and track delivery.</span>
                      </span>
                    </div>
                  </div>
                  <div className="lp-model-arrow"><ArrowDown size={22} /></div>
                  <div className="lp-model-platforms">
                    {PLATFORMS.slice(0, 8).map((platform) => (
                      <div className="lp-model-platform" key={platform.key}>
                        <PlatformIcon platform={platform.key} size={19} />
                        {platform.name}
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section className="lp-section compact" aria-label="Supported platforms">
            <div className="lp-wide-inner">
              <div className="lp-platform-strip">
                {PLATFORMS.map((platform) => (
                  <div className="lp-platform-card" key={platform.key}>
                    <PlatformIcon platform={platform.key} size={22} />
                    {platform.name}
                  </div>
                ))}
              </div>
            </div>
          </section>

          <section className="lp-section">
            <div className="lp-inner lp-split">
              <div>
                <div className="lp-section-head left">
                  <p className="lp-eyebrow">Why UniPost</p>
                  <h2>Stop maintaining separate social media integrations</h2>
                  <p>
                    Every platform has its own connection model, content constraints, media behavior,
                    publish lifecycle, and failure modes. UniPost turns those differences into one API.
                  </p>
                </div>
                <div className="lp-problem-list">
                  {WHY_ITEMS.map((item) => (
                    <div className="lp-problem-item" key={item}>
                      <CheckCircle2 size={18} />
                      {item}
                    </div>
                  ))}
                </div>
              </div>

              <div className="lp-unifies">
                <h3 className="lp-unifies-title">One product feature instead of nine integration projects</h3>
                <div className="lp-unifies-row">
                  <div className="lp-unifies-box">
                    <strong>Before UniPost</strong>
                    <span>Custom OAuth, media validation, retry logic, result tracking, and platform-specific code for each network.</span>
                  </div>
                  <div className="lp-unifies-arrow"><ArrowRight size={24} /></div>
                  <div className="lp-unifies-box">
                    <strong>With UniPost</strong>
                    <span>Connect accounts once, publish by account ID, and monitor delivery with one API and webhook surface.</span>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section className="lp-section">
            <div className="lp-wide-inner">
              <div className="lp-section-head center">
                <p className="lp-eyebrow">How it works</p>
                <h2>Get an API key, connect accounts, publish content</h2>
                <p>
                  The production path is three steps: authenticate your app, connect customer
                  accounts, then send publish requests through one API.
                </p>
              </div>
              <div className="lp-steps">
                {HOW_STEPS.map((step, index) => (
                  <div className="lp-step" key={step.title}>
                    <div className="lp-step-number">{String(index + 1).padStart(2, "0")}</div>
                    <h3>{step.title}</h3>
                    <p>{step.body}</p>
                  </div>
                ))}
              </div>
            </div>
          </section>

          <section className="lp-section lp-api-section">
            <div className="lp-wide-inner">
              <div className="lp-section-head center">
                <p className="lp-eyebrow">What you can do</p>
                <h2>One API surface for the social layer</h2>
                <p>
                  Pick the endpoint for the job: onboard accounts, publish posts,
                  report analytics, receive delivery events, and handle inbox workflows.
                </p>
              </div>
              <div className="lp-api-list">
                {API_SURFACE.map((item) => (
                  <Link href={item.href} className="lp-api-row" key={item.area}>
                    <div className="lp-api-area">{item.area}</div>
                    <div className="lp-api-endpoint">
                      <span className={`lp-api-method ${item.method.toLowerCase()}`}>{item.method}</span>
                      <span className="lp-api-path">{item.path}</span>
                    </div>
                    <p className="lp-api-body">{item.body}</p>
                    <span className="lp-api-link" aria-hidden="true">
                      <ArrowRight size={18} />
                    </span>
                  </Link>
                ))}
              </div>
            </div>
          </section>

          <section className="lp-section">
            <div className="lp-inner lp-code-layout">
              <div className="lp-code-copy">
                <p className="lp-eyebrow">Developer quickstart</p>
                <h2>A publish call should feel boring</h2>
                <p>
                  Once accounts are connected, your app sends one request. UniPost handles the
                  platform-specific rules behind it.
                </p>
                <div className="lp-hero-actions lp-code-actions">
                  <Link href="/docs/quickstart" className="lp-btn lp-btn-primary">Read Quickstart</Link>
                  <Link href="/docs/api/posts/create" className="lp-btn lp-btn-outline">Create Post API</Link>
                </div>
              </div>
              <div className="lp-code-window">
                <div className="lp-code-head">
                  <div className="lp-code-dots"><span /><span /><span /></div>
                  <span>publish.ts</span>
                </div>
                <div className="lp-code">
                  <pre><code>{PUBLISH_SNIPPET}</code></pre>
                </div>
              </div>
            </div>
          </section>

          <section className="lp-cta">
            <div className="lp-cta-inner">
              <p className="lp-eyebrow">Start building</p>
              <h2>Build the social layer once</h2>
              <p>
                Add account connection, media upload, multi-platform publishing, and delivery monitoring
                without maintaining every social integration yourself.
              </p>
              <div className="lp-cta-actions">
                <a href={START_BUILDING_URL} className="lp-btn lp-btn-primary">
                  Start Building
                  <ArrowRight size={17} />
                </a>
                <Link href="/alternatives/zernio" className="lp-btn lp-btn-outline">
                  Compare alternatives
                </Link>
              </div>
            </div>
          </section>
        </main>
      </div>
    </>
  );
}
