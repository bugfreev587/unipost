"use client";

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@clerk/nextjs";
import { PublicSiteHeader, PricingCTA } from "@/components/marketing/nav";
import { getBilling, getPublicFeatureFlags, listProfiles } from "@/lib/api";
import { X_CREDIT_PLANS } from "@/data/x-credits-catalog.generated";

// ── Data ──
//
// Pricing redesign (May 2026): tiers are now product-stage based, not
// per-volume. See docs/prd-pricing-packaging-redesign.md. Plan IDs map
// 1:1 to plans.id from migration 058.
type TierFeatureKind = "include" | "exclude" | "headline";
type TierFeature = { kind: TierFeatureKind; text: string };

type Tier = {
  id: string;          // plans.id
  name: string;
  price: number | null;  // null = custom (Enterprise)
  priceLabel?: string;   // override for non-numeric display
  blurb: string;
  posts: string;       // human-formatted post quota
  features: TierFeature[];
  highlight?: boolean; // show "Most popular" ribbon
  cta?: string;        // CTA button label
};

const TIERS: Tier[] = [
  {
    id: "free",
    name: "Free",
    price: 0,
    blurb: "A permanent free plan for real API and dashboard testing.",
    posts: "100 posts/mo",
    features: [
      { kind: "headline", text: "Dashboard + API" },
      { kind: "include", text: "8 platforms (excludes X)" },
      { kind: "include", text: "MCP server (AI agent ready)" },
      { kind: "include", text: "Webhooks + scheduling (50 active)" },
      { kind: "include", text: "Media kept 1d / 2d after final status" },
      { kind: "include", text: "1 profile · 1 user" },
      { kind: "exclude", text: "Inbox" },
      { kind: "exclude", text: "Analytics" },
    ],
  },
  {
    id: "api",
    name: "API",
    price: 10,
    blurb: "For developers who only need the publishing API.",
    posts: "1,000 posts/mo",
    features: [
      { kind: "headline", text: "Dashboard + API + Analytics" },
      { kind: "include", text: "All 9 platforms incl. X" },
      { kind: "include", text: "Read-only Analytics API" },
      { kind: "include", text: "Webhooks + scheduling" },
      { kind: "include", text: "Media kept 2d / 4d after final status" },
      { kind: "include", text: "2 profiles · 1 user" },
      { kind: "exclude", text: "Inbox" },
    ],
  },
  {
    id: "basic",
    name: "Basic",
    price: 19,
    highlight: true,
    blurb: "Operating console for solo builders and creators.",
    posts: "2,500 posts/mo",
    features: [
      { kind: "headline", text: "Custom Hosted + Inbox + Analytics" },
      { kind: "include", text: "All 9 platforms incl. X" },
      { kind: "include", text: "Inbox: DMs + comments" },
      { kind: "include", text: "Full Analytics suite" },
      { kind: "include", text: "Hosted Connect + credentials for 1 platform" },
      { kind: "include", text: "Media kept 4d / 8d after final status" },
      { kind: "include", text: "5 profiles · 1 user" },
      { kind: "exclude", text: "Hide Powered by UniPost" },
    ],
  },
  {
    id: "growth",
    name: "Growth",
    price: 59,
    blurb: "Embed UniPost into your own product.",
    posts: "7,500 posts/mo",
    features: [
      { kind: "headline", text: "All-platform custom mode" },
      { kind: "include", text: "Everything in Basic" },
      { kind: "include", text: "Hosted Connect + credentials on all supported platforms" },
      { kind: "include", text: "Optional removal of Powered by UniPost" },
      { kind: "include", text: "Branded OAuth flow" },
      { kind: "include", text: "Media kept 15d / 30d after final status" },
      { kind: "include", text: "25 profiles · 3 users" },
    ],
  },
  {
    id: "team",
    name: "Team",
    price: 149,
    blurb: "For agencies and multi-operator teams.",
    posts: "Unlimited posts/mo",
    features: [
      { kind: "headline", text: "RBAC + per-member API keys" },
      { kind: "include", text: "Everything in Growth" },
      { kind: "include", text: "Roles: owner / admin / editor" },
      { kind: "include", text: "Audit log" },
      { kind: "include", text: "Media kept 30d / 60d after final status" },
      { kind: "include", text: "Unlimited profiles · unlimited users" },
      { kind: "include", text: "Priority support" },
    ],
  },
];

// Comparison matrix for the table further down. Columns are aligned
// with TIERS minus Enterprise (which lives in its own section below pricing cards).
type CompareCell = string | boolean;
type CompareRow = {
  name: string;
  sub?: string | null;
  free: CompareCell;
  api: CompareCell;
  basic: CompareCell;
  growth: CompareCell;
  team: CompareCell;
};

const COMPARE_ROWS: CompareRow[] = [
  { name: "Monthly posts", free: "100", api: "1,000", basic: "2,500", growth: "7,500", team: "Unlimited" },
  { name: "Monthly quota behavior", sub: "Immediate publishing stays soft on paid plans; new schedules pause when effective usage reaches 100%", free: "Hard cap", api: "Soft publish + schedule guard", basic: "Soft publish + schedule guard", growth: "Soft publish + schedule guard", team: "Unlimited" },
  { name: "Platforms", sub: "X (Twitter), Bluesky, LinkedIn, Instagram, Threads, TikTok, YouTube, Pinterest, Facebook", free: "8 (no X)", api: "9", basic: "9", growth: "9", team: "9" },
  { name: "Profiles", sub: "Brand groupings inside one workspace", free: "1", api: "2", basic: "5", growth: "25", team: "Unlimited" },
  { name: "Users", sub: "Team members on the workspace", free: "1", api: "1", basic: "1", growth: "3", team: "Unlimited" },
  { name: "Per-account daily safety caps", sub: "Protects connected accounts from spam flags — X 20/day, IG 100/day, FB 100/day, Threads 250/day, others 50/day", free: true, api: true, basic: true, growth: true, team: true },
  { name: "Posting API", sub: "REST API for publish / schedule / validate", free: true, api: true, basic: true, growth: true, team: true },
  { name: "MCP server", sub: "AI agent integration via MCP protocol", free: true, api: true, basic: true, growth: true, team: true },
  { name: "Webhooks", sub: "Real-time event notifications", free: true, api: true, basic: true, growth: true, team: true },
  { name: "Scheduling", sub: "Post at a future time", free: true, api: true, basic: true, growth: true, team: true },
  { name: "Active scheduled posts", sub: "Free has a separate backlog cap. Paid schedules consume monthly capacity when committed.", free: "50", api: "Within monthly capacity", basic: "Within monthly capacity", growth: "Within monthly capacity", team: "Unlimited" },
  { name: "Media retention after success", sub: "Uploaded UniPost media is kept after the parent post publishes successfully, then cleaned from storage.", free: "1 day", api: "2 days", basic: "4 days", growth: "15 days", team: "30 days" },
  { name: "Media retention after failed/partial/cancelled", sub: "Failed, partial, and cancelled posts share the same longer troubleshooting window. Scheduled, draft, and in-flight posts are retained.", free: "2 days", api: "4 days", basic: "8 days", growth: "30 days", team: "60 days" },
  { name: "Dashboard UI", sub: "Compose, account management, analytics, and workspace settings in browser", free: true, api: true, basic: true, growth: true, team: true },
  { name: "Inbox", sub: "DMs and comments from connected accounts", free: false, api: false, basic: true, growth: true, team: true },
  { name: "Analytics", sub: "Reach, impressions, engagement", free: false, api: "read-only API", basic: true, growth: true, team: true },
  { name: "Hosted Connect + Platform Credentials", sub: "Shared custom platform scope for branding and BYO developer credentials", free: false, api: false, basic: "1 shared platform", growth: "All supported", team: "All supported" },
  { name: "Hosted Connect attribution", sub: "\"Powered by UniPost\" on hosted onboarding", free: "Shown", api: "Shown", basic: "Shown", growth: "Optional", team: "Optional" },
  { name: "RBAC + per-member API keys", sub: "Roles: owner / admin / editor", free: false, api: false, basic: false, growth: false, team: true },
  { name: "Audit log", sub: "Membership and config-change history", free: false, api: false, basic: false, growth: false, team: true },
];

const FAQS = [
  { q: "Why Free, API, Basic, Growth, Team?", a: "Each plan corresponds to a stage of how you use UniPost — evaluating the API, running it as your only integration point, using the dashboard as your operating console, embedding it into your product, or running it as a multi-operator team. Pick by which of those describes you, not by raw post volume." },
  { q: "What counts as a post?", a: "One successful publish to a single connected social account. Posting the same content to 3 platforms counts as 3 posts. Failed or cancelled posts are never counted." },
  { q: "Is there a free trial?", a: "The Free plan is the trial: 100 posts/month, no credit card, and no time limit. Paid plans do not include a separate time-limited trial." },
  { q: "Why is X (Twitter) not on the Free plan?", a: "The X API has the highest per-call cost of any platform we support, and the Free plan's 100-post quota is too small to absorb that cost without distorting our pricing for everyone else. Free workspaces can read existing X data; new X publishes and connections require any paid plan starting at $10/mo." },
  { q: "Why are there per-account daily limits?", a: "To protect your customers' accounts from being flagged for spam by the platforms themselves. Each connected account has its own daily ceiling — X 20/day, Instagram 100/day, Facebook 100/day, Threads 250/day, others 50/day. Limits reset at 00:00 UTC. Failed posts never count toward the cap." },
  { q: "Can I change plans anytime?", a: "Yes. Upgrade instantly from your billing dashboard. Downgrades apply at the start of the next billing cycle. No lock-in, no cancellation fees." },
  { q: "What happens if I go over my monthly post quota?", a: "Free workspaces stop accepting new publish requests once the 100-post monthly quota is reached. On API, Basic, and Growth, immediate publishing remains available, while new scheduled posts pause once completed plus committed scheduled usage reaches 100%. Cancel or move scheduled posts, wait for the monthly reset, or upgrade to restore scheduling capacity. Team includes unlimited monthly posts." },
  { q: "Can I schedule posts on the Free plan?", a: "Yes. Free workspaces can schedule posts, but they can hold up to 50 undeleted parent posts in scheduled status at once. Published, failed, partial, draft, and cancelled posts do not count toward that active scheduled backlog. Paid plans do not have an active scheduled-post cap." },
  { q: "How long does UniPost keep uploaded media after a post finishes?", a: "Scheduled, draft, queued, publishing, and processing posts keep their media. Once the parent post reaches a final state, uploaded media is retained by plan: Free 1 day after success or 2 days after failed/partial/cancelled; API 2/4 days; Basic 4/8 days; Growth 15/30 days; Team 30/60 days." },
  { q: "What's the difference between API and Basic?", a: "API includes the dashboard, publishing API, MCP server, and read-only Analytics API. Basic adds Inbox for DMs/comments, full Analytics, and one shared custom platform for Hosted Connect branding plus Platform Credentials. Same publishing API on both." },
  { q: "When do I need Growth?", a: "When Basic's one shared custom platform is no longer enough and you need Hosted Connect branding or BYO platform credentials across all supported platforms, or you want to remove \"Powered by UniPost\" from hosted onboarding." },
  { q: "When do I need Team?", a: "When multiple people need to log in and collaborate, with role-based permissions, per-member API keys, and an audit log. Typical fit: agencies managing multiple client brands, internal marketing teams." },
  { q: "What does unlimited Team usage mean?", a: "Team has no monthly UniPost post quota. Platform safety limits, third-party API quotas, abuse controls, and shared-infrastructure fairness still apply. Customers needing capacity planning, SLA, or custom platform-volume terms should use Enterprise." },
  { q: "When do I need Enterprise instead of Team?", a: "Use Enterprise when you need dedicated support, capacity planning, custom platform-volume terms, SLA, security review, or procurement. It is a sales-led contract plan, not a sixth self-serve card." },
  { q: "Can Enterprise increase third-party platform quotas?", a: "Enterprise can help plan, isolate, and contract around usage, but it cannot override platform-owned rate limits, app review, spam controls, or content policy enforcement. Custom means contract-defined terms and is not a smaller quota than Team." },
  { q: "How does UniPost compare to Ayrshare, Zernio, or PostForMe?", a: "UniPost self-serve plans are based on product stage and monthly post capacity, not per connected social account. For an embedded app with 100 end users connecting 2 social accounts each, UniPost Growth is $59/mo if total usage fits under 7,500 posts/month; Zernio's current account-meter pricing is $418/mo for those 200 connected accounts. PostForMe is open-source at $10/mo — UniPost API matches that price and adds a permanent free tier and an Inbox. See full comparisons at unipost.dev/alternatives." },
];

const COMPARE_VALUE_KEYS: Record<string, string> = {
  Unlimited: "comparison.values.unlimited",
  "Hard cap": "comparison.values.hardCap",
  "Soft publish + schedule guard": "comparison.values.softGuard",
  "8 (no X)": "comparison.values.noX",
  "Within monthly capacity": "comparison.values.withinMonthlyCapacity",
  "1 day": "comparison.values.oneDay",
  "2 days": "comparison.values.twoDays",
  "4 days": "comparison.values.fourDays",
  "8 days": "comparison.values.eightDays",
  "15 days": "comparison.values.fifteenDays",
  "30 days": "comparison.values.thirtyDays",
  "60 days": "comparison.values.sixtyDays",
  "read-only API": "comparison.values.readOnlyApi",
  "1 shared platform": "comparison.values.oneSharedPlatform",
  "All supported": "comparison.values.allSupported",
  Shown: "comparison.values.shown",
  Optional: "comparison.values.optional",
};

// ── Icons ──
function CheckIcon({ className = "" }: { className?: string }) { return <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2.2" width="15" height="15" style={{ flexShrink: 0 }}><path d="M3 8l4 4 6-7" /></svg>; }
function XIcon({ className = "" }: { className?: string }) { return <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" width="15" height="15" style={{ flexShrink: 0 }}><path d="M4 8h8" /></svg>; }

// ── Styles ──
const CSS = `:root{--pr-bg:var(--app-bg);--pr-s1:var(--marketing-surface);--pr-s2:var(--marketing-surface-alt);--pr-s3:var(--marketing-surface-elevated);--pr-border:var(--marketing-border);--pr-b2:var(--marketing-border-strong);--pr-b3:var(--marketing-border-strong);--pr-text:var(--marketing-text);--pr-muted:var(--marketing-muted);--pr-muted2:var(--marketing-subtle);--pr-accent:var(--primary);--pr-adim:var(--success-soft);--pr-blue:var(--marketing-link);--pr-shadow-soft:var(--marketing-shadow-soft);--pr-shadow-lg:var(--marketing-shadow-lg);--pr-r:8px;--pr-mono:var(--font-fira-code),monospace;--pr-ui:var(--font-dm-sans),system-ui,sans-serif;--pr-content-max:1400px;--pr-px:32px}*{box-sizing:border-box;margin:0;padding:0}body{background:var(--pr-bg);color:var(--pr-text);font-family:var(--pr-ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}.pr-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--pr-r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--pr-ui);text-decoration:none;white-space:nowrap}.pr-btn-primary{background:var(--pr-blue);color:#fff}.pr-btn-primary:hover{background:var(--marketing-link-hover)}.pr-btn-ghost{background:transparent;color:var(--pr-muted);border-color:var(--pr-b2)}.pr-btn-ghost:hover{background:var(--pr-s2);color:var(--pr-text);border-color:var(--pr-b3)}.pr-btn-tier{width:100%;justify-content:center;padding:10px;font-size:13.5px;border-radius:9px;background:var(--pr-s2);color:var(--pr-text);border-color:var(--pr-b2)}.pr-btn-tier:hover{background:var(--pr-s3);border-color:var(--pr-b3)}.pr-btn-tier-hi{background:var(--pr-blue);color:#fff;border-color:var(--pr-blue);font-weight:700}.pr-btn-tier-hi:hover{background:var(--marketing-link-hover);border-color:var(--marketing-link-hover)}.pr-btn-ent{background:transparent;color:var(--pr-text);border-color:var(--pr-b2);padding:10px 24px;font-size:14px;flex-shrink:0}.pr-btn-ent:hover{background:var(--pr-s2);border-color:var(--pr-b3)}.pr-page{max-width:var(--pr-content-max);margin:0 auto;padding:0 var(--pr-px) 96px}.pr-hero{padding:96px 0 56px;text-align:center}.pr-hero-title{font-size:64px;font-weight:900;letter-spacing:-2px;line-height:1.04;color:var(--pr-text);margin-bottom:20px}.pr-hero-sub{font-size:17px;color:var(--pr-muted);max-width:640px;margin:0 auto 12px;line-height:1.7}.pr-hero-altlink{font-size:13px;color:var(--pr-muted2);margin-top:18px}.pr-hero-altlink a{color:var(--pr-blue);text-decoration:underline}.pr-cards{display:grid;grid-template-columns:repeat(5,1fr);gap:14px;margin-bottom:18px}.pr-card{background:var(--pr-s1);border:1px solid var(--pr-border);border-radius:14px;padding:24px 20px;display:flex;flex-direction:column;box-shadow:var(--pr-shadow-soft);position:relative}.pr-card.hi{border-color:var(--pr-accent);box-shadow:0 0 0 1px var(--pr-accent),var(--pr-shadow-soft)}.pr-card.current{border-color:var(--pr-accent);box-shadow:0 0 0 1px var(--pr-accent),var(--pr-shadow-soft)}.pr-ribbon{position:absolute;top:-11px;left:50%;transform:translateX(-50%);background:var(--pr-accent);color:#fff;font-size:10.5px;font-weight:700;padding:3px 10px;border-radius:20px;font-family:var(--pr-mono);letter-spacing:.04em;white-space:nowrap}.pr-current-badge{position:absolute;top:-11px;left:50%;transform:translateX(-50%);background:var(--pr-accent);color:#fff;font-size:10.5px;font-weight:700;padding:3px 10px;border-radius:20px;font-family:var(--pr-mono);letter-spacing:.04em;white-space:nowrap}.pr-tname{font-size:14px;font-weight:700;color:var(--pr-text);letter-spacing:.02em;margin-bottom:8px;text-transform:uppercase;font-family:var(--pr-mono)}.pr-tprice{font-size:32px;font-weight:900;letter-spacing:-1px;color:var(--pr-text);line-height:1;font-family:var(--pr-mono)}.pr-tprice .mo{font-size:14px;font-weight:400;color:var(--pr-muted);letter-spacing:0;margin-left:2px}.pr-tprice.custom{font-size:22px;letter-spacing:-.5px}.pr-tposts{font-size:12.5px;color:var(--pr-muted);font-family:var(--pr-mono);margin-top:6px}.pr-tblurb{font-size:13px;color:var(--pr-muted);line-height:1.55;margin:14px 0 16px;min-height:40px}.pr-tdivider{height:1px;background:var(--pr-border);margin-bottom:14px}.pr-tfeats{flex:1;margin-bottom:18px}.pr-tfeat{display:flex;align-items:flex-start;gap:8px;font-size:12.5px;color:var(--pr-text);margin-bottom:9px;line-height:1.4}.pr-tfeat svg{width:13px;height:13px;flex-shrink:0;margin-top:2px}.pr-tfeat .chk{color:var(--pr-accent)}.pr-tfeat .chk-no{color:var(--pr-muted2)}.pr-tfeat.dim{color:var(--pr-muted)}.pr-tfeat.headline{font-weight:700;color:var(--pr-text);padding-bottom:8px;border-bottom:1px dashed var(--pr-border);margin-bottom:11px}.pr-tfeat.headline svg{display:none}.pr-soft{background:var(--pr-s1);border:1px solid var(--pr-border);border-radius:14px;padding:22px 26px;margin-bottom:64px;display:flex;gap:18px;align-items:flex-start;box-shadow:var(--pr-shadow-soft)}.pr-soft-icon{width:40px;height:40px;flex-shrink:0;background:var(--pr-adim);border:1px solid color-mix(in srgb,var(--pr-accent) 18%,transparent);border-radius:10px;display:flex;align-items:center;justify-content:center}.pr-soft-icon svg{width:18px;height:18px;color:var(--pr-accent)}.pr-soft-title{font-size:15px;font-weight:700;margin-bottom:6px;color:var(--pr-text)}.pr-soft-desc{font-size:13.5px;color:var(--pr-muted);line-height:1.65}.pr-soft-mono{font-family:var(--pr-mono);font-size:12px;color:var(--pr-text);background:var(--pr-s2);border:1px solid var(--pr-border);padding:2px 6px;border-radius:4px}.pr-compare{margin-bottom:64px}.pr-compare-title{font-size:36px;font-weight:800;letter-spacing:-.5px;margin-bottom:28px;text-align:center;color:var(--pr-text)}.pr-compare-wrap{border:1px solid var(--pr-border);border-radius:14px;overflow:hidden;box-shadow:var(--pr-shadow-soft)}.pr-compare-hdr{display:grid;grid-template-columns:2fr repeat(5,1fr);background:var(--pr-s2);border-bottom:1px solid var(--pr-border)}.pr-ch{padding:16px 20px;font-size:13px;font-weight:700;color:var(--pr-muted);letter-spacing:.06em;text-transform:uppercase;font-family:var(--pr-mono)}.pr-ch.hl{color:var(--pr-accent)}.pr-compare-row{display:grid;grid-template-columns:2fr repeat(5,1fr);border-bottom:1px solid var(--pr-border);transition:background .1s}.pr-compare-row:last-child{border-bottom:none}.pr-compare-row:hover{background:var(--pr-s2)}.pr-cr{padding:18px 20px;display:flex;align-items:center}.pr-cr-feat{flex-direction:column;align-items:flex-start}.pr-cr-name{font-size:15.5px;font-weight:600;color:var(--pr-text);line-height:1.35}.pr-cr-sub{font-size:13px;color:var(--pr-muted);margin-top:5px;line-height:1.5}.pr-chk{color:var(--pr-accent)}.pr-cr svg.pr-chk{width:22px;height:22px;flex-shrink:0;stroke-width:2.6}.pr-dash{color:var(--pr-muted2);font-size:24px;line-height:1;font-weight:400}.pr-cr-val{font-family:var(--pr-mono);font-size:14px;color:var(--pr-text);font-weight:500}.pr-cr-val.hl{color:var(--pr-accent);font-weight:700}.pr-ent{background:var(--pr-s1);border:1px solid var(--pr-border);border-radius:14px;padding:32px 36px;display:flex;align-items:center;justify-content:space-between;gap:32px;margin-bottom:64px;box-shadow:var(--pr-shadow-soft)}.pr-ent-title{font-size:20px;font-weight:700;letter-spacing:-.3px;margin-bottom:8px;color:var(--pr-text)}.pr-ent-desc{font-size:13.5px;color:var(--pr-muted);line-height:1.65;max-width:520px}.pr-ent-chips{display:flex;flex-wrap:wrap;gap:8px;margin-top:14px}.pr-ent-chip{display:flex;align-items:center;gap:6px;font-size:12px;color:var(--pr-muted);background:var(--pr-s2);border:1px solid var(--pr-border);padding:4px 10px;border-radius:20px}.pr-ent-chip svg{width:11px;height:11px;color:var(--pr-accent)}.pr-faq-title{font-size:32px;font-weight:800;letter-spacing:-.5px;margin-bottom:24px;text-align:center;color:var(--pr-text)}.pr-faq-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:64px}.pr-faq-item{background:var(--pr-s1);border:1px solid var(--pr-border);border-radius:12px;padding:22px 24px;transition:border-color .15s;box-shadow:var(--pr-shadow-soft)}.pr-faq-item:hover{border-color:var(--pr-b2)}.pr-faq-q{font-size:14.5px;font-weight:600;margin-bottom:9px;color:var(--pr-text)}.pr-faq-a{font-size:13px;color:var(--pr-muted);line-height:1.7}@media(min-width:1700px){:root{--pr-content-max:1500px;--pr-px:40px}}@media(max-width:1300px){.pr-cards{grid-template-columns:repeat(3,1fr)}.pr-compare-hdr,.pr-compare-row{grid-template-columns:1.6fr repeat(5,1fr)}}@media(max-width:1024px){:root{--pr-content-max:100%;--pr-px:24px}.pr-cards{grid-template-columns:repeat(2,1fr)}.pr-faq-grid{grid-template-columns:1fr}.pr-ent{flex-direction:column;align-items:flex-start}.pr-compare-wrap{overflow-x:auto}.pr-compare-hdr,.pr-compare-row{grid-template-columns:1.4fr repeat(5,minmax(110px,1fr));min-width:780px}}@media(max-width:680px){.pr-page{padding-bottom:72px}.pr-hero{padding:64px 0 40px}.pr-hero-title{font-size:42px}.pr-cards{grid-template-columns:1fr}.pr-soft,.pr-ent{padding:22px}.pr-compare-title,.pr-faq-title{font-size:26px}}`;

const MOBILE_CSS = `
.pr-page{overflow-x:hidden}
.pr-btn{max-width:100%}
.pr-ent{margin-bottom:18px}
.pr-ent-copy{flex:1;min-width:0}
.pr-ent-desc{max-width:none}
.pr-ent+.pr-soft{margin-bottom:18px}
.pr-ent-chip{background:color-mix(in srgb,var(--pr-accent) 10%,#fff);border-color:color-mix(in srgb,var(--pr-accent) 28%,var(--pr-border));color:color-mix(in srgb,var(--pr-text) 72%,var(--pr-accent));font-weight:650;box-shadow:inset 0 1px 0 rgba(255,255,255,.72)}
.pr-ent-chip svg{color:var(--pr-accent)}
.pr-compare-wrap{max-width:100%;overflow-x:auto;-webkit-overflow-scrolling:touch}
@media(max-width:680px){
  :root{--pr-px:18px}
  .pr-hero-title{font-size:clamp(34px,10vw,42px);letter-spacing:-1px}
  .pr-hero-sub{font-size:16px}
  .pr-card{padding:22px 18px}
  .pr-soft{flex-direction:column;gap:14px}
  .pr-ent{gap:20px}
  .pr-btn-ent{width:100%;justify-content:center}
  .pr-ent-chips{display:grid;grid-template-columns:1fr}
  .pr-faq-item{padding:18px}
  .pr-compare-wrap{margin-inline:-18px;border-left:0;border-right:0;border-radius:0}
  .pr-compare-hdr,.pr-compare-row{min-width:720px}
}
@media(max-width:420px){
  .pr-btn{white-space:normal;text-align:center}
  .pr-soft-mono{overflow-wrap:anywhere}
}
`;

const X_CREDITS_CSS = `
.pr-xcredits{margin:0 0 64px}
.pr-xcredits-head{max-width:760px;margin:0 auto 24px;text-align:center}
.pr-xcredits-title{font-size:36px;font-weight:800;letter-spacing:-.5px;color:var(--pr-text);margin-bottom:10px}
.pr-xcredits-copy{font-size:14px;line-height:1.7;color:var(--pr-muted)}
.pr-xcredits-wrap{border:1px solid var(--pr-border);border-radius:14px;overflow:auto;box-shadow:var(--pr-shadow-soft);background:var(--pr-s1)}
.pr-xcredits-table{width:100%;min-width:880px;border-collapse:collapse}
.pr-xcredits-table th,.pr-xcredits-table td{padding:16px 18px;border-bottom:1px solid var(--pr-border);text-align:left;vertical-align:top}
.pr-xcredits-table th{font-family:var(--pr-mono);font-size:11px;letter-spacing:.05em;text-transform:uppercase;color:var(--pr-muted);background:var(--pr-s2)}
.pr-xcredits-table td{font-family:var(--pr-mono);font-size:13px;color:var(--pr-text)}
.pr-xcredits-table tr:last-child td{border-bottom:0}
.pr-xcredits-plan{font-family:var(--pr-ui);font-weight:750}
.pr-xcredits-muted{display:block;margin-top:3px;font-family:var(--pr-ui);font-size:11.5px;line-height:1.45;color:var(--pr-muted)}
.pr-xcredits-note{margin-top:14px;font-size:12.5px;line-height:1.65;color:var(--pr-muted)}
.pr-xcredits-cards{display:none}
.pr-xcredits-card{border:1px solid var(--pr-border);border-radius:12px;padding:15px;background:var(--pr-s1)}
.pr-xcredits-card.current{border-color:var(--pr-accent);box-shadow:0 0 0 1px var(--pr-accent)}
.pr-xcredits-card-head{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-bottom:12px}
.pr-xcredits-current{font-family:var(--pr-mono);font-size:10px;letter-spacing:.06em;text-transform:uppercase;color:var(--pr-accent)}
.pr-xcredits-card-grid{display:grid;grid-template-columns:1fr 1fr;gap:10px}
.pr-xcredits-card-label{display:block;font-size:10.5px;color:var(--pr-muted);margin-bottom:2px}
.pr-xcredits-card-value{font-family:var(--pr-mono);font-size:13px;color:var(--pr-text)}
@media(max-width:680px){.pr-xcredits-title{font-size:26px}.pr-xcredits-head{text-align:left}.pr-xcredits{margin-bottom:48px}.pr-xcredits-wrap{display:none}.pr-xcredits-cards{display:grid;gap:10px}}
`;

// ── Component ──
export default function PricingPage() {
  const [currentPlan, setCurrentPlan] = useState<string | null>(null);
  const [profileId, setProjectId] = useState<string | null>(null);
  const [xCreditsEnabled, setXCreditsEnabled] = useState(false);
  const { isSignedIn, getToken } = useAuth();
  const t = useTranslations("pricing");

  const APP_URL = "https://app.unipost.dev";

  const loadPlan = useCallback(async () => {
    if (!isSignedIn) return;
    try {
      const token = await getToken();
      if (!token) return;
      const profiles = await listProfiles(token);
      if (!profiles.data || profiles.data.length === 0) return;
      const pid = profiles.data[0].id;
      setProjectId(pid);
      const billing = await getBilling(token);
      setCurrentPlan(billing.data.plan);
    } catch {
      // CORS / network — no current plan info
    }
  }, [isSignedIn, getToken]);

  useEffect(() => {
    const timer = window.setTimeout(() => { void loadPlan(); }, 0);
    return () => window.clearTimeout(timer);
  }, [loadPlan]);

  useEffect(() => {
    let cancelled = false;
    void getPublicFeatureFlags()
      .then((response) => {
        if (!cancelled) {
          setXCreditsEnabled(response.data.flags.x_credits_billing_v1);
        }
      })
      .catch(() => {
        // Conservative default: do not advertise a disabled or unknown rollout.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <style dangerouslySetInnerHTML={{ __html: MOBILE_CSS }} />
      <style dangerouslySetInnerHTML={{ __html: X_CREDITS_CSS }} />
      <PublicSiteHeader active="pricing" />

      <div className="pr-page">
        {/* HERO */}
        <div className="pr-hero">
          <h1 className="pr-hero-title">{t("hero.titleLine1")}<br />{t("hero.titleLine2")}<br />{t("hero.titleLine3")}</h1>
          <p className="pr-hero-sub">{t("hero.body")}</p>
          <p className="pr-hero-altlink">
            {t("hero.compare")}{" "}
            <Link href="/alternatives/postforme">vs PostForMe</Link>
            {" · "}
            <Link href="/alternatives/zernio">vs Zernio</Link>
            {" · "}
            <Link href="/alternatives/ayrshare">vs Ayrshare</Link>
          </p>
        </div>

        {/* CARDS */}
        <div className="pr-cards">
          {TIERS.map((tier) => {
            const isCurrent = currentPlan === tier.id;
            const buttonHref = isCurrent
              ? (profileId ? `${APP_URL}/projects/${profileId}/billing` : APP_URL)
              : profileId
                ? `${APP_URL}/projects/${profileId}/billing?upgrade=${tier.id}`
                : undefined;
            const ctaLabel = isCurrent
              ? t("labels.goToDashboard")
              : tier.price === 0
                ? t("labels.getStarted")
                : t("labels.choosePlan");
            return (
              <div key={tier.id} className={`pr-card ${tier.highlight ? "hi" : ""} ${isCurrent ? "current" : ""}`}>
                {tier.highlight && !isCurrent && <div className="pr-ribbon">{t("labels.mostPopular")}</div>}
                {isCurrent && <div className="pr-current-badge">{t("labels.currentPlan")}</div>}
                <div className="pr-tname">{t(`plans.${tier.id}.name`)}</div>
                {tier.price === null ? (
                  <div className="pr-tprice custom">{tier.priceLabel ?? t("labels.custom")}</div>
                ) : (
                  <div className="pr-tprice">${tier.price}<span className="mo">{t("labels.perMonth")}</span></div>
                )}
                <div className="pr-tposts">{t(`plans.${tier.id}.posts`)}</div>
                <div className="pr-tblurb">{t(`plans.${tier.id}.blurb`)}</div>
                <div className="pr-tdivider" />
                <div className="pr-tfeats">
                  {tier.features.map((f, i) => (
                    <div key={i} className={`pr-tfeat ${f.kind === "headline" ? "headline" : ""} ${f.kind === "exclude" ? "dim" : ""}`}>
                      {f.kind === "include" && <CheckIcon className="chk" />}
                      {f.kind === "exclude" && <XIcon className="chk-no" />}
                      {t(`plans.${tier.id}.features.${i}`)}
                    </div>
                  ))}
                </div>
                {buttonHref ? (
                  <PricingCTA className={`pr-btn pr-btn-tier ${tier.highlight ? "pr-btn-tier-hi" : ""}`} label={ctaLabel} href={buttonHref} />
                ) : (
                  <PricingCTA className={`pr-btn pr-btn-tier ${tier.highlight ? "pr-btn-tier-hi" : ""}`} label={ctaLabel} />
                )}
              </div>
            );
          })}
        </div>

        {/* Enterprise */}
        <div className="pr-ent">
          <div className="pr-ent-copy">
            <div className="pr-ent-title">{t("enterprise.title")}</div>
            <div className="pr-ent-desc">{t("enterprise.description")}</div>
            <div className="pr-ent-chips">{[0, 1, 2, 3].map((index) => (<div key={index} className="pr-ent-chip"><CheckIcon />{t(`enterprise.chips.${index}`)}</div>))}</div>
          </div>
          <a href="mailto:support@unipost.dev" className="pr-btn pr-btn-ent">{t("enterprise.contact")}</a>
        </div>

        {/* Embedded apps */}
        <div className="pr-soft">
          <div className="pr-soft-icon"><svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" width="20" height="20"><path d="M4 8h8M8 4v8" /><circle cx="8" cy="8" r="6.5" /></svg></div>
          <div>
            <div className="pr-soft-title">{t("embedded.title")}</div>
            <div className="pr-soft-desc">{t("embedded.description")} <Link href="/alternatives/zernio">{t("embedded.link")}</Link>.</div>
          </div>
        </div>

        {/* Quota behavior */}
        <div className="pr-soft">
          <div className="pr-soft-icon"><svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" width="20" height="20"><circle cx="8" cy="8" r="6.5" /><path d="M8 5v3M8 10v1" /></svg></div>
          <div>
            <div className="pr-soft-title">{t("quota.title")}</div>
            <div className="pr-soft-desc">{t("quota.description")}</div>
          </div>
        </div>

        {xCreditsEnabled ? <section className="pr-xcredits" aria-labelledby="x-credits-capacity">
          <div className="pr-xcredits-head">
            <h2 id="x-credits-capacity" className="pr-xcredits-title">{t("xCredits.title")}</h2>
            <p className="pr-xcredits-copy">{t("xCredits.description")}</p>
          </div>
          <div className="pr-xcredits-wrap">
            <table className="pr-xcredits-table">
              <thead>
                <tr>
                  <th>{t("xCredits.plan")}</th>
                  <th>{t("xCredits.included")}</th>
                  <th>{t("xCredits.normalPosts")}</th>
                  <th>{t("xCredits.urlPosts")}</th>
                  <th>{t("xCredits.comments")}</th>
                  <th>{t("xCredits.dms")}</th>
                </tr>
              </thead>
              <tbody>
                {X_CREDIT_PLANS.map((plan) => (
                  <tr key={plan.id} className={currentPlan === plan.id ? "current" : ""}>
                    <td><span className="pr-xcredits-plan">{plan.label}</span></td>
                    <td>{plan.monthly_allowance == null ? t("labels.custom") : plan.monthly_allowance.toLocaleString()}</td>
                    <td>{plan.capacity?.normal_posts.toLocaleString() ?? t("labels.custom")}</td>
                    <td>{plan.capacity?.url_posts.toLocaleString() ?? t("labels.custom")}</td>
                    <td>
                      {plan.inbox_eligible ? plan.capacity?.comment_interactions.toLocaleString() ?? t("labels.custom") : t("xCredits.inboxNotIncluded")}
                      <span className="pr-xcredits-muted">{t("xCredits.commentNote")}</span>
                    </td>
                    <td>
                      {plan.inbox_eligible ? plan.capacity?.dm_interactions.toLocaleString() ?? t("labels.custom") : t("xCredits.inboxNotIncluded")}
                      <span className="pr-xcredits-muted">{t("xCredits.dmNote")}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="pr-xcredits-cards">
            {X_CREDIT_PLANS.map((plan) => (
              <article key={plan.id} className={`pr-xcredits-card ${currentPlan === plan.id ? "current" : ""}`}>
                <div className="pr-xcredits-card-head">
                  <span className="pr-xcredits-plan">{plan.label}</span>
                  {currentPlan === plan.id && <span className="pr-xcredits-current">{t("labels.currentPlan")}</span>}
                </div>
                <div className="pr-xcredits-card-grid">
                  <div><span className="pr-xcredits-card-label">{t("xCredits.included")}</span><span className="pr-xcredits-card-value">{plan.monthly_allowance == null ? t("labels.custom") : plan.monthly_allowance.toLocaleString()}</span></div>
                  <div><span className="pr-xcredits-card-label">{t("xCredits.normalPosts")}</span><span className="pr-xcredits-card-value">{plan.capacity?.normal_posts.toLocaleString() ?? t("labels.custom")}</span></div>
                  <div><span className="pr-xcredits-card-label">{t("xCredits.urlPosts")}</span><span className="pr-xcredits-card-value">{plan.capacity?.url_posts.toLocaleString() ?? t("labels.custom")}</span></div>
                  <div><span className="pr-xcredits-card-label">{t("xCredits.comments")}</span><span className="pr-xcredits-card-value">{plan.inbox_eligible ? plan.capacity?.comment_interactions.toLocaleString() ?? t("labels.custom") : t("xCredits.inboxNotIncluded")}</span></div>
                  <div><span className="pr-xcredits-card-label">{t("xCredits.dms")}</span><span className="pr-xcredits-card-value">{plan.inbox_eligible ? plan.capacity?.dm_interactions.toLocaleString() ?? t("labels.custom") : t("xCredits.inboxNotIncluded")}</span></div>
                </div>
              </article>
            ))}
          </div>
          <p className="pr-xcredits-note">{t("xCredits.note")}</p>
        </section> : null}

        {/* Compare */}
        <div className="pr-compare">
          <h2 className="pr-compare-title">{t("labels.comparePlans")}</h2>
          <div className="pr-compare-wrap">
            <div className="pr-compare-hdr">
              <div className="pr-ch">{t("comparison.capability")}</div>
              <div className="pr-ch">Free</div>
              <div className="pr-ch">API</div>
              <div className="pr-ch hl">Basic</div>
              <div className="pr-ch">Growth</div>
              <div className="pr-ch">Team</div>
            </div>
            {COMPARE_ROWS.map((row, rowIndex) => (
              <div key={row.name} className="pr-compare-row">
                <div className="pr-cr pr-cr-feat">
                  <span className="pr-cr-name">{t(`comparison.rows.${rowIndex}.name`)}</span>
                  {row.sub && <span className="pr-cr-sub">{t(`comparison.rows.${rowIndex}.sub`)}</span>}
                </div>
                {(["free", "api", "basic", "growth", "team"] as const).map((col) => {
                  const v = row[col];
                  const valueKey = typeof v === "string" ? COMPARE_VALUE_KEYS[v] : undefined;
                  const renderedValue = valueKey ? t(valueKey) : v;
                  return (
                    <div key={col} className="pr-cr">
                      {v === true ? <CheckIcon className="pr-chk" />
                        : v === false ? <span className="pr-dash">—</span>
                        : <span className={`pr-cr-val ${col === "basic" ? "hl" : ""}`}>{renderedValue}</span>}
                    </div>
                  );
                })}
              </div>
            ))}
          </div>
        </div>

        {/* FAQ */}
        <h2 className="pr-faq-title">{t("labels.frequentlyAskedQuestions")}</h2>
        <div className="pr-faq-grid">
          {FAQS.map((f, index) => (<div key={f.q} className="pr-faq-item"><div className="pr-faq-q">{t(`faq.${index}.question`)}</div><div className="pr-faq-a">{t(`faq.${index}.answer`)}</div></div>))}
        </div>
      </div>
    </>
  );
}
