"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { PricingNav, PricingCTA } from "@/components/marketing/nav";
import { listProjects, getBilling } from "@/lib/api";

// ── Data ──
const TIERS = [
  { id: "p10",   label: "1,000 Social Posts", posts: "1,000", price: 10 },
  { id: "p25",   label: "2,500 Social Posts", posts: "2,500", price: 25 },
  { id: "p50",   label: "5,000 Social Posts", posts: "5,000", price: 50 },
  { id: "p75",   label: "10,000 Social Posts", posts: "10,000", price: 75 },
  { id: "p150",  label: "20,000 Social Posts", posts: "20,000", price: 150 },
  { id: "p300",  label: "40,000 Social Posts", posts: "40,000", price: 300 },
  { id: "p500",  label: "100,000 Social Posts", posts: "100,000", price: 500 },
  { id: "p1000", label: "200,000 Social Posts", posts: "200,000", price: 1000 },
];
const FEATURES_FREE = [
  { text: "100 posts per month", included: true }, { text: "Unlimited social accounts", included: true },
  { text: "All 7 platforms", included: true }, { text: "Webhooks", included: true },
  { text: "Analytics", included: true }, { text: "Scheduled posts", included: true },
  { text: "MCP Server (AI Agent)", included: true }, { text: "Unlimited API keys", included: true },
  { text: "Unlimited team members", included: true }, { text: "Quickstart mode", included: true },
  { text: "Native mode", included: false },
];
const FEATURES_PAID = [
  { text: "posts per month", dynamic: true, included: true }, { text: "Unlimited social accounts", included: true },
  { text: "All 7 platforms", included: true }, { text: "Webhooks", included: true },
  { text: "Analytics", included: true }, { text: "Scheduled posts", included: true },
  { text: "MCP Server (AI Agent)", included: true }, { text: "Unlimited API keys", included: true },
  { text: "Unlimited team members", included: true }, { text: "Quickstart mode", included: true },
  { text: "Native mode", included: true },
];
const COMPARE_ROWS = [
  { name: "Post volume", sub: null, free: "100/mo", paid: "dynamic" },
  { name: "All 7 platforms", sub: "X, Bluesky, LinkedIn, Instagram, Threads, TikTok, YouTube", free: true, paid: true },
  { name: "Unlimited social accounts", sub: null, free: true, paid: true },
  { name: "Unlimited API keys", sub: null, free: true, paid: true },
  { name: "Unlimited team members", sub: null, free: true, paid: true },
  { name: "Webhooks", sub: "Real-time event notifications", free: true, paid: true },
  { name: "Analytics", sub: "Unified metrics from all platforms", free: true, paid: true },
  { name: "Scheduled posts", sub: "Post at a future time", free: true, paid: true },
  { name: "MCP Server", sub: "AI Agent integration via MCP protocol", free: true, paid: true },
  { name: "Quickstart mode", sub: "Use UniPost credentials", free: true, paid: true },
  { name: "Native mode", sub: "Bring your own credentials", free: false, paid: true },
];
const FAQS = [
  { q: "What counts as a post?", a: "One successful publish to a single social account. Posting the same content to 3 platforms counts as 3 posts. Failed or cancelled posts are never counted." },
  { q: "Can I change plans anytime?", a: "Yes. Upgrade instantly from your billing dashboard. Downgrades apply at the start of the next billing cycle. No lock-in, no cancellation fees." },
  { q: "What's the difference between Quickstart and Native mode?", a: "Quickstart uses UniPost's platform credentials so you can start immediately — users see 'UniPost' during OAuth. Native mode lets you bring your own credentials so users see your app name. Native requires a paid plan." },
  { q: "Do unused posts roll over to the next month?", a: "No, post quotas reset at the start of each billing cycle. However, if you exceed your limit we won't cut you off — we'll reach out about upgrading instead." },
  { q: "What happens if I go over my plan?", a: "We won't shut you down or block your posts. If your usage consistently exceeds the limit, we'll reach out about upgrading. You'll never experience a hard stop that breaks your users' experience." },
  { q: "Is there a free trial for paid plans?", a: "The Free plan is your trial — 100 posts/month with no credit card required. Upgrade when you need more volume. There's no time limit on the free tier." },
];

// ── Icons ──
function ZapIcon() { return <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" width="14" height="14"><path d="M9 2L4 9h4l-1 5 5-7H8l1-5z" /></svg>; }
function CheckIcon({ className = "" }: { className?: string }) { return <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2.2" width="15" height="15" style={{ flexShrink: 0 }}><path d="M3 8l4 4 6-7" /></svg>; }
function ChevronIcon() { return <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" width="14" height="14" style={{ flexShrink: 0 }}><path d="M4 6l4 4 4-4" /></svg>; }

// ── Styles ──
const CSS = `@import url('https://fonts.googleapis.com/css2?family=DM+Sans:opsz,wght@9..40,300;9..40,400;9..40,500;9..40,600;9..40,700;9..40,800;9..40,900&family=Fira+Code:wght@400;500&display=swap');:root{--bg:#000;--s1:#0a0a0a;--s2:#111;--s3:#1a1a1a;--border:#1a1a1a;--b2:#242424;--b3:#2e2e2e;--text:#f0f0f0;--muted:#666;--muted2:#333;--accent:#10b981;--adim:#10b98112;--blue:#0ea5e9;--r:8px;--mono:'Fira Code',monospace;--ui:'DM Sans',system-ui,sans-serif;--nav-max:1480px;--content-max:1320px;--px:32px}*{box-sizing:border-box;margin:0;padding:0}body{background:var(--bg);color:var(--text);font-family:var(--ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}.pr-nav{position:sticky;top:0;z-index:50;border-bottom:1px solid var(--border);background:#00000095;backdrop-filter:blur(12px);-webkit-backdrop-filter:blur(12px)}.pr-nav-inner{max-width:var(--nav-max);margin:0 auto;padding:0 var(--px);height:56px;display:flex;align-items:center;justify-content:space-between}.pr-logo{display:flex;align-items:center;gap:10px;text-decoration:none}.pr-logo-mark{width:28px;height:28px;background:var(--accent);border-radius:7px;display:flex;align-items:center;justify-content:center}.pr-logo-mark svg{width:14px;height:14px;color:#000}.pr-logo-name{font-size:16px;font-weight:700;letter-spacing:-.4px;color:var(--text)}.pr-nav-links{display:flex;gap:4px}.pr-nav-link{padding:6px 14px;font-size:14px;font-weight:500;color:var(--muted);cursor:pointer;border-radius:var(--r);transition:color .1s;text-decoration:none}.pr-nav-link:hover{color:var(--text)}.pr-nav-link.active{color:var(--text);font-weight:500}.pr-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--ui);text-decoration:none;white-space:nowrap}.pr-btn-primary{background:var(--blue);color:#000}.pr-btn-primary:hover{background:#38bdf8}.pr-btn-ghost{background:transparent;color:var(--muted);border-color:var(--b2)}.pr-btn-ghost:hover{background:var(--s2);color:var(--text);border-color:var(--b3)}.pr-btn-outline{background:transparent;color:var(--text);border-color:var(--b2)}.pr-btn-outline:hover{background:var(--s2);border-color:var(--b3)}.pr-btn-free{background:var(--s2);color:var(--text);border-color:var(--b2);width:100%;justify-content:center;padding:11px;font-size:14px;border-radius:9px}.pr-btn-free:hover{background:var(--s3);border-color:var(--b3)}.pr-btn-paid{background:var(--blue);color:#000;width:100%;justify-content:center;padding:11px;font-size:14px;border-radius:9px;font-weight:700}.pr-btn-paid:hover{background:#38bdf8}.pr-btn-ent{background:transparent;color:var(--text);border-color:var(--b2);padding:10px 24px;font-size:14px;flex-shrink:0}.pr-btn-ent:hover{background:var(--s2);border-color:var(--b3)}.pr-page{max-width:var(--content-max);margin:0 auto;padding:0 var(--px) 96px}.pr-hero{padding:96px 0 72px;text-align:center}.pr-hero-title{font-size:72px;font-weight:900;letter-spacing:-2px;line-height:1.04;color:var(--text);margin-bottom:20px}.pr-hero-sub{font-size:17px;color:var(--muted);max-width:480px;margin:0 auto 48px;line-height:1.75}.pr-cards{display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-bottom:16px}.pr-card{background:var(--s1);border:1px solid var(--border);border-radius:16px;padding:32px;display:flex;flex-direction:column}.pr-card.paid{border-color:var(--b2)}.pr-card.current-plan{border-color:var(--accent);position:relative}.pr-current-badge{position:absolute;top:-12px;left:50%;transform:translateX(-50%);background:var(--accent);color:#000;font-size:11px;font-weight:700;padding:3px 14px;border-radius:20px;font-family:var(--mono);letter-spacing:.03em;white-space:nowrap}.pr-card-top{display:flex;align-items:flex-start;justify-content:space-between;margin-bottom:8px}.pr-price{font-size:48px;font-weight:900;letter-spacing:-1.5px;color:var(--text);line-height:1;font-family:var(--mono)}.pr-price .mo{font-size:16px;font-weight:400;color:var(--muted);letter-spacing:0}.pr-card-sub{font-size:14px;color:var(--muted);margin-bottom:24px}.pr-divider{height:1px;background:var(--border);margin-bottom:24px}.pr-feats{flex:1;margin-bottom:24px}.pr-feat{display:flex;align-items:flex-start;gap:11px;font-size:14px;color:var(--text);margin-bottom:12px;line-height:1.4}.pr-feat svg{width:15px;height:15px;flex-shrink:0;margin-top:2px}.pr-feat .chk{color:var(--accent)}.pr-feat .chk-no{color:var(--muted2)}.pr-feat.dim{color:var(--muted)}.pr-sel-wrap{position:relative}.pr-sel-btn{display:flex;align-items:center;gap:10px;padding:10px 16px;background:var(--s2);border:1px solid var(--b2);border-radius:9px;cursor:pointer;font-size:13.5px;font-weight:500;color:var(--text);transition:all .15s;white-space:nowrap;min-width:180px;justify-content:space-between;font-family:var(--ui)}.pr-sel-btn:hover{border-color:var(--b3);background:var(--s3)}.pr-sel-btn svg{width:14px;height:14px;color:var(--muted);flex-shrink:0;transition:transform .2s}.pr-sel-btn.open svg{transform:rotate(180deg)}.pr-dropdown{position:absolute;top:calc(100% + 8px);right:0;background:var(--s2);border:1px solid var(--b2);border-radius:12px;padding:5px;min-width:200px;z-index:30;box-shadow:0 16px 40px #0008;animation:pr-drop .15s ease}@keyframes pr-drop{from{opacity:0;transform:translateY(-4px)}to{opacity:1;transform:translateY(0)}}.pr-drop-item{padding:9px 14px;border-radius:7px;cursor:pointer;font-size:13.5px;color:var(--muted);display:flex;align-items:center;justify-content:space-between;transition:all .1s}.pr-drop-item:hover{background:var(--s3);color:var(--text)}.pr-drop-item.active{color:var(--text);font-weight:500}.pr-drop-item svg{width:13px;height:13px;color:var(--accent)}.pr-soft{background:var(--s1);border:1px solid var(--border);border-radius:14px;padding:28px 32px;margin-bottom:16px;display:flex;gap:20px;align-items:flex-start}.pr-soft-icon{width:44px;height:44px;flex-shrink:0;background:var(--adim);border:1px solid #10b98118;border-radius:10px;display:flex;align-items:center;justify-content:center}.pr-soft-icon svg{width:20px;height:20px;color:var(--accent)}.pr-soft-title{font-size:16px;font-weight:700;margin-bottom:7px;color:var(--text)}.pr-soft-desc{font-size:14px;color:var(--muted);line-height:1.7}.pr-soft-mono{font-family:var(--mono);font-size:12.5px;color:var(--text);background:var(--s2);border:1px solid var(--border);padding:2px 7px;border-radius:4px}.pr-compare{margin-bottom:64px}.pr-compare-title{font-size:36px;font-weight:800;letter-spacing:-.6px;margin-bottom:28px;text-align:center;color:var(--text)}.pr-compare-wrap{border:1px solid var(--border);border-radius:14px;overflow:hidden}.pr-compare-hdr{display:grid;grid-template-columns:2.5fr 1fr 1fr;background:var(--s2);border-bottom:1px solid var(--border)}.pr-ch{padding:14px 24px;font-size:12.5px;font-weight:600;color:var(--muted);letter-spacing:.03em}.pr-ch.hl{color:var(--accent)}.pr-compare-row{display:grid;grid-template-columns:2.5fr 1fr 1fr;border-bottom:1px solid var(--border);transition:background .1s}.pr-compare-row:last-child{border-bottom:none}.pr-compare-row:hover{background:var(--s2)}.pr-cr{padding:16px 24px;display:flex;align-items:center}.pr-cr-feat{flex-direction:column;align-items:flex-start}.pr-cr-name{font-size:14px;font-weight:600;color:var(--text)}.pr-cr-sub{font-size:12px;color:var(--muted);margin-top:2px}.pr-chk{color:var(--accent)}.pr-cr svg{width:15px;height:15px;flex-shrink:0}.pr-dash{color:var(--muted2);font-size:20px;line-height:1}.pr-cr-val{font-family:var(--mono);font-size:13px}.pr-ent{background:var(--s1);border:1px solid var(--border);border-radius:14px;padding:36px 40px;display:flex;align-items:center;justify-content:space-between;gap:32px;margin-bottom:64px}.pr-ent-title{font-size:22px;font-weight:700;letter-spacing:-.3px;margin-bottom:8px;color:var(--text)}.pr-ent-desc{font-size:14px;color:var(--muted);line-height:1.65;max-width:480px}.pr-ent-chips{display:flex;flex-wrap:wrap;gap:8px;margin-top:16px}.pr-ent-chip{display:flex;align-items:center;gap:6px;font-size:12.5px;color:var(--muted);background:var(--s2);border:1px solid var(--border);padding:4px 11px;border-radius:20px}.pr-ent-chip svg{width:11px;height:11px;color:var(--accent)}.pr-faq-title{font-size:36px;font-weight:800;letter-spacing:-.6px;margin-bottom:28px;text-align:center;color:var(--text)}.pr-faq-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:64px}.pr-faq-item{background:var(--s1);border:1px solid var(--border);border-radius:12px;padding:24px 26px;transition:border-color .15s}.pr-faq-item:hover{border-color:var(--b2)}.pr-faq-q{font-size:15px;font-weight:600;margin-bottom:10px;color:var(--text)}.pr-faq-a{font-size:13.5px;color:var(--muted);line-height:1.7}.pr-footer{border-top:1px solid var(--border);padding:32px 0}.pr-footer-inner{max-width:var(--content-max);margin:0 auto;padding:0 var(--px);display:flex;align-items:center;justify-content:space-between}.pr-foot-logo{display:flex;align-items:center;gap:9px}.pr-foot-mark{width:24px;height:24px;background:var(--accent);border-radius:5px;display:flex;align-items:center;justify-content:center}.pr-foot-mark svg{width:12px;height:12px;color:#000}.pr-foot-name{font-size:14px;font-weight:700;color:var(--text)}.pr-foot-links{display:flex;gap:20px}.pr-foot-link{font-size:13px;color:var(--muted);cursor:pointer;text-decoration:none}.pr-foot-link:hover{color:var(--text)}.pr-foot-copy{font-size:13px;color:var(--muted2)}@media(min-width:1600px){:root{--nav-max:1560px;--content-max:1360px;--px:40px}}@media(max-width:1024px){:root{--nav-max:100%;--content-max:100%;--px:24px}}`;

export default function PricingPage() {
  const [selectedTier, setSelectedTier] = useState(0);
  const [dropOpen, setDropOpen] = useState(false);
  const [currentPlan, setCurrentPlan] = useState<string | null>(null);
  const [projectId, setProjectId] = useState<string | null>(null);
  const [trialEligible, setTrialEligible] = useState(true);
  const dropRef = useRef<HTMLDivElement>(null);
  const tier = TIERS[selectedTier];
  const { isSignedIn, getToken } = useAuth();

  const APP_URL = "https://app.unipost.dev";

  // Fetch current plan if signed in (fails silently on marketing domain due to CORS)
  const loadPlan = useCallback(async () => {
    if (!isSignedIn) return;
    try {
      const token = await getToken();
      if (!token) return;
      const projects = await listProjects(token);
      if (!projects.data || projects.data.length === 0) return;
      const pid = projects.data[0].id;
      setProjectId(pid);
      const billing = await getBilling(token, pid);
      const plan = billing.data.plan;
      setCurrentPlan(plan);
      setTrialEligible(billing.data.trial_eligible);
      // Set dropdown to current plan if it's a paid tier
      const tierIdx = TIERS.findIndex((t) => t.id === plan);
      if (tierIdx !== -1) {
        setSelectedTier(tierIdx);
      }
    } catch {
      // CORS or network error on unipost.dev — no current plan info
    }
  }, [isSignedIn, getToken]);

  useEffect(() => { loadPlan(); }, [loadPlan]);

  useEffect(() => {
    function handler(e: MouseEvent) {
      if (dropRef.current && !dropRef.current.contains(e.target as Node)) setDropOpen(false);
    }
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />

      {/* NAV */}
      <nav className="pr-nav">
        <div className="pr-nav-inner">
          <Link href="/" className="pr-logo"><div className="pr-logo-mark"><ZapIcon /></div><span className="pr-logo-name">UniPost</span></Link>
          <div className="pr-nav-links">
            <Link href="/solutions" className="pr-nav-link">Solutions</Link>
            <Link href="/docs" className="pr-nav-link">Docs</Link>
            <Link href="/pricing" className="pr-nav-link active">Pricing</Link>
          </div>
          <PricingNav />
        </div>
      </nav>

      <div className="pr-page">
        {/* HERO */}
        <div className="pr-hero">
          <h1 className="pr-hero-title">Predictable pricing<br />without surprises.</h1>
          <p className="pr-hero-sub">All plans include every feature. The only difference is how many posts you need per month.</p>
        </div>

        {/* CARDS */}
        <div className="pr-cards">
          {/* Free */}
          <div className={`pr-card ${currentPlan === "free" ? "current-plan" : ""}`}>
            {currentPlan === "free" && <div className="pr-current-badge">Current Plan</div>}
            <div className="pr-card-top"><div className="pr-price">$0<span className="mo">/mo</span></div></div>
            <div className="pr-card-sub">Everything you need to get started.</div>
            <div className="pr-divider" />
            <div className="pr-feats">
              {FEATURES_FREE.map((f) => (
                <div key={f.text} className={`pr-feat ${!f.included ? "dim" : ""}`}>
                  {f.included ? <CheckIcon className="chk" /> : <svg className="chk-no" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" width="15" height="15" style={{ flexShrink: 0 }}><path d="M4 8h8" /></svg>}
                  {f.text}
                </div>
              ))}
            </div>
            <div style={{ marginTop: "auto", paddingTop: 8 }}>
              {currentPlan === "free" ? (
                <PricingCTA className="pr-btn-free" label="Go to Dashboard" href={APP_URL} />
              ) : (
                <PricingCTA className="pr-btn-free" />
              )}
            </div>
          </div>
          {/* Paid */}
          <div className={`pr-card paid ${currentPlan === tier.id ? "current-plan" : ""}`}>
            {currentPlan === tier.id && <div className="pr-current-badge">Current Plan</div>}
            <div className="pr-card-top">
              <div className="pr-price">${tier.price}<span className="mo">/mo</span></div>
              <div className="pr-sel-wrap" ref={dropRef}>
                <button className={`pr-sel-btn ${dropOpen ? "open" : ""}`} onClick={() => setDropOpen(!dropOpen)}>
                  <span>{tier.label}</span>
                  {TIERS.some((t) => t.id === currentPlan) && (
                    <span style={{ fontSize: 10, color: "var(--accent)", marginRight: 2 }}>
                      {TIERS.find((t) => t.id === currentPlan)?.id === tier.id ? "✓" : ""}
                    </span>
                  )}
                  <ChevronIcon />
                </button>
                {dropOpen && (
                  <div className="pr-dropdown">
                    {TIERS.map((t, i) => (
                      <div key={t.label} className={`pr-drop-item ${i === selectedTier ? "active" : ""}`} onClick={() => { setSelectedTier(i); setDropOpen(false); }}>
                        <span>{t.label}</span>
                        <span style={{ display: "flex", alignItems: "center", gap: 6 }}>
                          {t.id === currentPlan && <span style={{ fontSize: 10, color: "var(--accent)", fontFamily: "var(--mono)" }}>Current</span>}
                          {i === selectedTier && <CheckIcon />}
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
            <div className="pr-card-sub">Everything you need to build and scale.</div>
            <div className="pr-divider" />
            <div className="pr-feats">
              {FEATURES_PAID.map((f) => (
                <div key={f.text} className="pr-feat">
                  <CheckIcon className="chk" />
                  {f.dynamic ? `Up to ${tier.posts} successful posts per month` : f.text}
                </div>
              ))}
            </div>
            {trialEligible && currentPlan !== tier.id && (
              <div style={{ fontSize: 13, color: "var(--accent)", fontWeight: 600, textAlign: "center", marginBottom: 8, fontFamily: "var(--mono)" }}>
                14-day free trial included
              </div>
            )}
            <div style={{ marginTop: "auto", paddingTop: 8 }}>
              {currentPlan === tier.id ? (
                <PricingCTA className="pr-btn-paid" label="Go to Dashboard" href={projectId ? `${APP_URL}/projects/${projectId}/billing` : APP_URL} />
              ) : projectId ? (
                <PricingCTA className="pr-btn-paid" label="Get Started" href={`${APP_URL}/projects/${projectId}/billing?upgrade=${tier.id}`} />
              ) : (
                <PricingCTA className="pr-btn-paid" />
              )}
            </div>
          </div>
        </div>

        {/* Soft block */}
        <div className="pr-soft">
          <div className="pr-soft-icon"><svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" width="20" height="20"><circle cx="8" cy="8" r="6.5" /><path d="M8 5v3M8 10v1" /></svg></div>
          <div>
            <div className="pr-soft-title">What happens when I go over my plan?</div>
            <div className="pr-soft-desc">We want you to succeed, and not be afraid of surprise charges. If you unexpectedly go over the limits of your plan, we won&apos;t shut you down or charge you extra automatically. API responses include <span className="pr-soft-mono">X-UniPost-Usage</span> and <span className="pr-soft-mono">X-UniPost-Warning</span> headers so you can monitor usage programmatically.</div>
          </div>
        </div>

        {/* Compare */}
        <div className="pr-compare">
          <h2 className="pr-compare-title">Compare plans</h2>
          <div className="pr-compare-wrap">
            <div className="pr-compare-hdr"><div className="pr-ch">Feature</div><div className="pr-ch">Free</div><div className="pr-ch hl">Paid</div></div>
            {COMPARE_ROWS.map((row) => (
              <div key={row.name} className="pr-compare-row">
                <div className="pr-cr pr-cr-feat"><span className="pr-cr-name">{row.name}</span>{row.sub && <span className="pr-cr-sub">{row.sub}</span>}</div>
                <div className="pr-cr">{row.free === true ? <CheckIcon className="pr-chk" /> : row.free === false ? <span className="pr-dash">—</span> : <span className="pr-cr-val" style={{ color: "var(--muted)" }}>{row.free}</span>}</div>
                <div className="pr-cr">{row.paid === "dynamic" ? <span className="pr-cr-val" style={{ color: "var(--accent)" }}>1,000 — 200,000/mo</span> : row.paid === true ? <CheckIcon className="pr-chk" /> : <span className="pr-dash">—</span>}</div>
              </div>
            ))}
          </div>
        </div>

        {/* Enterprise */}
        <div className="pr-ent">
          <div>
            <div className="pr-ent-title">Need more than 200,000 posts/month?</div>
            <div className="pr-ent-desc">Contact us for a tailored Enterprise plan with custom volume, SLA guarantees, dedicated support, and flexible contract terms.</div>
            <div className="pr-ent-chips">{["Custom post volume", "99.9% SLA", "Dedicated support", "Custom contract", "On-premise option"].map((c) => (<div key={c} className="pr-ent-chip"><CheckIcon />{c}</div>))}</div>
          </div>
          <a href="mailto:support@unipost.dev" className="pr-btn pr-btn-ent">Contact Sales →</a>
        </div>

        {/* FAQ */}
        <h2 className="pr-faq-title">Frequently asked questions</h2>
        <div className="pr-faq-grid">
          {FAQS.map((f) => (<div key={f.q} className="pr-faq-item"><div className="pr-faq-q">{f.q}</div><div className="pr-faq-a">{f.a}</div></div>))}
        </div>
      </div>

      {/* Footer */}
      <footer className="pr-footer">
        <div className="pr-footer-inner">
          <div className="pr-foot-logo"><div className="pr-foot-mark"><ZapIcon /></div><span className="pr-foot-name">UniPost</span></div>
          <div className="pr-foot-links">
            <Link href="/docs" className="pr-foot-link">Docs</Link>
            <Link href="/pricing" className="pr-foot-link">Pricing</Link>
            <Link href="/privacy" className="pr-foot-link">Privacy</Link>
            <Link href="/terms" className="pr-foot-link">Terms</Link>
          </div>
          <div className="pr-foot-copy">&copy; 2026 UniPost. All rights reserved.</div>
        </div>
      </footer>
    </>
  );
}
