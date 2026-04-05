"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { MarketingNav, MarketingCTA, MarketingCTALight } from "@/components/marketing/nav";

// ── Rotating subtitle hook (slide animation) ──
function useRotatingText(items: string[], interval = 2500) {
  const [index, setIndex] = useState(0);
  const [phase, setPhase] = useState<"in" | "out" | "idle">("idle");
  useEffect(() => {
    const timer = setInterval(() => {
      setPhase("out");
      setTimeout(() => {
        setIndex((i) => (i + 1) % items.length);
        setPhase("in");
        setTimeout(() => setPhase("idle"), 400);
      }, 400);
    }, interval);
    return () => clearInterval(timer);
  }, [items, interval]);
  return { text: items[index], phase };
}

// ── Data ──
const ROTATING_ITEMS = ["AI content generators", "social schedulers", "SaaS products", "marketing tools", "e-commerce platforms"];
const PLATFORMS = [
  { icon: "🦋", name: "Bluesky" }, { icon: "💼", name: "LinkedIn" }, { icon: "📸", name: "Instagram" },
  { icon: "🧵", name: "Threads" }, { icon: "🎵", name: "TikTok" }, { icon: "▶️", name: "YouTube" },
];
const FEATURES = [
  { number: "01", title: "One API, every platform", desc: "Stop maintaining 6 different integrations. One endpoint, one auth token, one response format. Post to Bluesky, LinkedIn, Instagram, Threads, TikTok, and YouTube with a single call.", code: `POST /v1/social-posts\n{\n  "caption": "Hello from UniPost!",\n  "account_ids": ["sa_instagram", "sa_linkedin"],\n  "scheduled_at": "2026-04-07T09:00:00Z"\n}` },
  { number: "02", title: "Token management, handled", desc: "OAuth flows, token refresh, expiry handling — all managed automatically. Your users connect once, and UniPost handles everything in the background forever.", code: `// No token refresh code needed.\n// UniPost handles it automatically.\n\nGET /v1/social-accounts\n→ Always returns valid, active accounts` },
  { number: "03", title: "AI Agent ready via MCP", desc: "The first unified social API with native MCP Server support. Let Claude, GPT, or any AI agent post on behalf of your users — no code required.", code: `// Claude Desktop config\n{\n  "mcpServers": {\n    "unipost": {\n      "url": "https://mcp.unipost.dev/sse",\n      "headers": {\n        "Authorization": "Bearer up_live_xxx"\n      }\n    }\n  }\n}` },
];
const CODE_SNIPPETS: Record<string, string> = {
  js: `const response = await fetch(\n  'https://api.unipost.dev/v1/social-posts',\n  {\n    method: 'POST',\n    headers: {\n      'Authorization': 'Bearer up_live_xxxx',\n      'Content-Type':  'application/json',\n    },\n    body: JSON.stringify({\n      caption:     'Hello from UniPost! 🚀',\n      account_ids: ['sa_instagram_123', 'sa_linkedin_456'],\n    }),\n  }\n);\n\nconst { data } = await response.json();\nconsole.log(data.id); // post_abc123`,
  python: `import requests\n\nresponse = requests.post(\n    'https://api.unipost.dev/v1/social-posts',\n    headers={\n        'Authorization': 'Bearer up_live_xxxx',\n        'Content-Type':  'application/json',\n    },\n    json={\n        'caption':     'Hello from UniPost! 🚀',\n        'account_ids': ['sa_instagram_123', 'sa_linkedin_456'],\n    }\n)\n\ndata = response.json()['data']\nprint(data['id'])  # post_abc123`,
  go: `req, _ := http.NewRequest("POST",\n    "https://api.unipost.dev/v1/social-posts",\n    strings.NewReader(\`{\n      "caption":     "Hello from UniPost! 🚀",\n      "account_ids": ["sa_instagram_123", "sa_linkedin_456"]\n    }\`),\n)\n\nreq.Header.Set("Authorization", "Bearer up_live_xxxx")\nreq.Header.Set("Content-Type",  "application/json")\n\nresp, _ := http.DefaultClient.Do(req)\n// resp.StatusCode == 200 ✓`,
  curl: `curl -X POST https://api.unipost.dev/v1/social-posts \\\\\n  -H "Authorization: Bearer up_live_xxxx" \\\\\n  -H "Content-Type: application/json" \\\\\n  -d '{\n    "caption":     "Hello from UniPost! 🚀",\n    "account_ids": ["sa_instagram_123", "sa_linkedin_456"]\n  }'`,
};
const MODES = [
  { badge: "Quickstart Mode", badgeColor: "#10b981", title: "Start posting in minutes", desc: "Use UniPost's developer credentials. No platform approval process, no waiting.", features: ["Instant access to all 6 platforms", "No developer approval needed", "OAuth shows 'UniPost' branding", "Available on all plans including Free"], ctaVariant: "ghost" },
  { badge: "Native Mode", badgeColor: "#3b82f6", title: "Your brand, your credentials", desc: "Bring your own platform credentials. Users see your app name during OAuth.", features: ["OAuth shows your app name", "Complete credential ownership", "Professional user experience", "Available on all paid plans"], ctaVariant: "primary" },
];
const FAQS = [
  { q: "Why UniPost over direct platform APIs?", a: "We handle OAuth, token refresh, media processing, and platform-specific quirks — reducing integration time from weeks to hours." },
  { q: "What counts as a post?", a: "One successful publish to a single social account. Posting to 3 platforms counts as 3 posts. Failed posts are never counted." },
  { q: "What's the difference between Quickstart and Native mode?", a: "Quickstart uses UniPost's credentials so you start immediately. Native mode lets you bring your own credentials so users see your app name." },
  { q: "Do I need to handle OAuth flows?", a: "No. UniPost handles the entire OAuth flow. Your users connect once through our hosted flow, and you get a simple account_id to use in API calls." },
];

// ── Icons ──
function CheckIcon() { return <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2.2" width="14" height="14" style={{ flexShrink: 0 }}><path d="M3 8l4 4 6-7" /></svg>; }
function ZapIcon() { return <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" width="14" height="14"><path d="M9 2L4 9h4l-1 5 5-7H8l1-5z" /></svg>; }

// ── Styles ──
const CSS = `@import url('https://fonts.googleapis.com/css2?family=DM+Sans:opsz,wght@9..40,300;9..40,400;9..40,500;9..40,600;9..40,700;9..40,800;9..40,900&family=Fira+Code:wght@400;500&display=swap');:root{--bg:#000;--s1:#0a0a0a;--s2:#111;--s3:#1a1a1a;--border:#1a1a1a;--b2:#242424;--b3:#2e2e2e;--text:#f0f0f0;--muted:#666;--muted2:#333;--accent:#10b981;--adim:#10b98112;--blue:#0ea5e9;--blue-dim:#0ea5e912;--r:8px;--mono:'Fira Code',monospace;--ui:'DM Sans',system-ui,sans-serif;--page-max:1100px;--page-padding:48px}*{box-sizing:border-box;margin:0;padding:0}body{background:var(--bg);color:var(--text);font-family:var(--ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}.lp-nav{position:sticky;top:0;z-index:50;width:100%;border-bottom:1px solid var(--border);background:#00000095;backdrop-filter:blur(12px);-webkit-backdrop-filter:blur(12px)}.lp-nav-inner{max-width:var(--page-max,1100px);margin:0 auto;padding:0 var(--page-padding,48px);height:56px;display:flex;align-items:center;justify-content:space-between}.lp-logo{display:flex;align-items:center;gap:10px;text-decoration:none}.lp-logo-mark{width:28px;height:28px;background:var(--accent);border-radius:7px;display:flex;align-items:center;justify-content:center}.lp-logo-mark svg{width:14px;height:14px;color:#000}.lp-logo-name{font-size:16px;font-weight:700;letter-spacing:-.4px;color:var(--text)}.lp-nav-links{display:flex;align-items:center;gap:4px}.lp-nav-link{padding:6px 14px;font-size:14px;font-weight:500;color:var(--muted);cursor:pointer;border-radius:var(--r);transition:color .1s;text-decoration:none}.lp-nav-link:hover{color:var(--text)}.lp-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--ui);text-decoration:none;white-space:nowrap}.lp-btn-primary{background:var(--blue);color:#000}.lp-btn-primary:hover{background:#38bdf8;box-shadow:0 0 24px #0ea5e930}.lp-btn-ghost{background:transparent;color:var(--muted);border-color:var(--b2)}.lp-btn-ghost:hover{background:var(--s2);color:var(--text);border-color:var(--b3)}.lp-btn-outline{background:transparent;color:var(--text);border-color:var(--b2)}.lp-btn-outline:hover{background:var(--s2);border-color:var(--b3)}.lp-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}.lp-btn svg{width:14px;height:14px}.lp-page{max-width:var(--page-max,1100px);margin:0 auto;padding:0 var(--page-padding,48px)}.lp-hero{padding:120px 0 96px;text-align:center;display:flex;flex-direction:column;align-items:center}.lp-hero-badge{display:inline-flex;align-items:center;gap:7px;padding:5px 14px;border-radius:20px;background:var(--adim);border:1px solid #10b98120;font-size:12.5px;color:var(--accent);font-weight:600;margin-bottom:32px;font-family:var(--mono)}.lp-hero-badge-dot{width:6px;height:6px;border-radius:50%;background:var(--accent);animation:lp-pulse 2s infinite}@keyframes lp-pulse{0%,100%{opacity:1}50%{opacity:.3}}.lp-hero-title{font-size:76px;font-weight:900;letter-spacing:-2.5px;line-height:1;color:var(--text);margin-bottom:24px;max-width:900px;text-align:center}.lp-hero-title em{color:var(--accent);font-style:normal}.lp-hero-rotate-wrap{font-size:76px;font-weight:900;letter-spacing:-2.5px;line-height:1;color:var(--muted);margin-bottom:36px;text-align:center;overflow:hidden;position:relative;height:1.1em}.lp-hero-rotate-text{display:inline-block;transition:transform .4s cubic-bezier(.4,0,.2,1),opacity .4s cubic-bezier(.4,0,.2,1)}.lp-hero-rotate-text.idle{transform:translateX(0);opacity:1}.lp-hero-rotate-text.out{transform:translateX(-80px);opacity:0}.lp-hero-rotate-text.in{transform:translateX(80px);opacity:0;animation:slide-in .4s cubic-bezier(.4,0,.2,1) forwards}@keyframes slide-in{from{transform:translateX(80px);opacity:0}to{transform:translateX(0);opacity:1}}.lp-hero-sub{font-size:17px;color:var(--muted);max-width:520px;line-height:1.75;margin-bottom:44px;text-align:center}.lp-hero-actions{display:flex;align-items:center;gap:12px;margin-bottom:28px;justify-content:center}.lp-hero-meta{font-size:13px;color:var(--muted2);display:flex;align-items:center;gap:16px;justify-content:center}.lp-hero-meta-item{display:flex;align-items:center;gap:6px}.lp-hero-meta-item svg{width:13px;height:13px;color:var(--accent)}.lp-platforms{padding:0 0 96px}.lp-plat-label{font-size:11.5px;color:var(--muted2);text-transform:uppercase;letter-spacing:.12em;font-weight:700;margin-bottom:24px;font-family:var(--mono);text-align:center}.lp-plat-row{display:flex;align-items:center;gap:12px;flex-wrap:wrap;justify-content:center}.lp-plat-chip{display:flex;align-items:center;gap:9px;padding:9px 18px;background:var(--s1);border:1px solid var(--border);border-radius:24px;font-size:14px;font-weight:500;color:var(--muted);transition:all .15s;cursor:default}.lp-plat-chip:hover{border-color:var(--b2);color:var(--text);background:var(--s2)}.lp-plat-icon{font-size:18px}.lp-code-section{padding:0 0 96px}.lp-section-eyebrow{font-size:11.5px;color:var(--accent);text-transform:uppercase;letter-spacing:.12em;font-weight:700;margin-bottom:12px;font-family:var(--mono);text-align:center}.lp-section-title{font-size:44px;font-weight:800;letter-spacing:-1px;margin-bottom:12px;line-height:1.1;text-align:center}.lp-section-sub{font-size:16px;color:var(--muted);max-width:520px;line-height:1.7;margin-bottom:48px;text-align:center;margin-left:auto;margin-right:auto}.lp-code-wrap{background:var(--s1);border:1px solid var(--border);border-radius:12px;overflow:hidden}.lp-code-topbar{background:var(--s2);border-bottom:1px solid var(--border);padding:12px 20px;display:flex;align-items:center;gap:10px}.lp-code-dot{width:11px;height:11px;border-radius:50%}.lp-code-dot-r{background:#ef444440}.lp-code-dot-y{background:#f59e0b40}.lp-code-dot-g{background:#10b98140}.lp-code-tabs{display:flex;gap:2px;margin-left:14px}.lp-code-tab{padding:4px 12px;border-radius:5px;font-size:12.5px;font-weight:500;color:var(--muted);cursor:pointer;font-family:var(--mono);transition:all .1s;border:1px solid transparent;background:none}.lp-code-tab:hover{color:var(--text)}.lp-code-tab.active{background:var(--s3);color:var(--text);border-color:var(--b2)}.lp-code-body{padding:28px 32px;font-family:var(--mono);font-size:13.5px;line-height:1.75;overflow-x:auto;color:#a0a0a0;white-space:pre}.lp-features{padding:0 0 96px}.lp-feat-list{display:flex;flex-direction:column;gap:80px}.lp-feat-item{display:grid;grid-template-columns:1fr 1fr;gap:64px;align-items:center}.lp-feat-item.reverse{direction:rtl}.lp-feat-item.reverse>*{direction:ltr}.lp-feat-number{font-family:var(--mono);font-size:11px;color:var(--muted2);font-weight:600;letter-spacing:.1em;margin-bottom:16px}.lp-feat-title{font-size:32px;font-weight:800;letter-spacing:-.6px;line-height:1.15;margin-bottom:16px}.lp-feat-desc{font-size:15px;color:var(--muted);line-height:1.75}.lp-feat-code{background:var(--s1);border:1px solid var(--border);border-radius:10px;padding:24px 28px;font-family:var(--mono);font-size:13px;line-height:1.7;color:#888;white-space:pre;overflow-x:auto}.lp-stats{padding:0 0 96px}.lp-stats-inner{border:1px solid var(--border);border-radius:14px;display:grid;grid-template-columns:repeat(4,1fr);overflow:hidden}.lp-stat{padding:40px 36px;border-right:1px solid var(--border)}.lp-stat:last-child{border-right:none}.lp-stat-num{font-family:var(--mono);font-size:40px;font-weight:700;color:var(--accent);letter-spacing:-1px;margin-bottom:8px}.lp-stat-label{font-size:14px;color:var(--muted);line-height:1.5}.lp-modes{padding:0 0 96px}.lp-modes-grid{display:grid;grid-template-columns:1fr 1fr;gap:16px}.lp-mode-card{background:var(--s1);border:1px solid var(--border);border-radius:14px;padding:36px;transition:all .2s}.lp-mode-card:hover{border-color:var(--b2);background:var(--s2)}.lp-mode-badge{display:inline-flex;align-items:center;gap:6px;padding:4px 12px;border-radius:20px;font-size:12px;font-weight:700;font-family:var(--mono);margin-bottom:20px}.lp-mode-title{font-size:24px;font-weight:700;letter-spacing:-.4px;margin-bottom:12px}.lp-mode-desc{font-size:14px;color:var(--muted);line-height:1.7;margin-bottom:24px}.lp-mode-feats{list-style:none;margin-bottom:28px}.lp-mode-feat{display:flex;align-items:flex-start;gap:10px;font-size:14px;color:var(--muted);margin-bottom:9px}.lp-mode-feat svg{width:14px;height:14px;color:var(--accent);flex-shrink:0;margin-top:3px}.lp-faq{padding:0 0 96px}.lp-faq-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.lp-faq-item{background:var(--s1);border:1px solid var(--border);border-radius:10px;padding:24px 26px;transition:border-color .15s}.lp-faq-item:hover{border-color:var(--b2)}.lp-faq-q{font-size:15px;font-weight:600;margin-bottom:10px}.lp-faq-a{font-size:13.5px;color:var(--muted);line-height:1.7}.lp-cta{padding:0 0 96px}.lp-cta-inner{background:var(--s1);border:1px solid var(--border);border-radius:16px;padding:80px 64px;text-align:center;position:relative;overflow:hidden}.lp-cta-glow{position:absolute;width:400px;height:400px;border-radius:50%;background:radial-gradient(circle,#10b98110,transparent 70%);top:50%;left:50%;transform:translate(-50%,-50%);pointer-events:none}.lp-cta-title{font-size:52px;font-weight:900;letter-spacing:-1.5px;margin-bottom:16px;position:relative}.lp-cta-sub{font-size:16px;color:var(--muted);margin-bottom:40px;position:relative}.lp-cta-actions{display:flex;align-items:center;justify-content:center;gap:12px;position:relative}.lp-footer{width:100%;border-top:1px solid var(--border);padding:48px 0}.lp-footer-inner{max-width:var(--page-max,1100px);margin:0 auto;padding:0 var(--page-padding,48px)}.lp-footer-top{display:grid;grid-template-columns:2fr 1fr 1fr 1fr;gap:48px;margin-bottom:48px}.lp-footer-logo{display:flex;align-items:center;gap:9px;margin-bottom:16px}.lp-footer-mark{width:26px;height:26px;background:var(--accent);border-radius:6px;display:flex;align-items:center;justify-content:center}.lp-footer-mark svg{width:13px;height:13px;color:#000}.lp-footer-name{font-size:15px;font-weight:700;color:var(--text)}.lp-footer-tagline{font-size:13px;color:var(--muted);line-height:1.65;max-width:260px}.lp-footer-col-title{font-size:12px;font-weight:700;text-transform:uppercase;letter-spacing:.08em;color:var(--muted2);margin-bottom:16px}.lp-footer-links{list-style:none}.lp-footer-link{font-size:13.5px;color:var(--muted);margin-bottom:10px;cursor:pointer;transition:color .1s;display:block;text-decoration:none}.lp-footer-link:hover{color:var(--text)}.lp-footer-bottom{border-top:1px solid var(--border);padding-top:24px;display:flex;align-items:center;justify-content:space-between}.lp-footer-copy{font-size:13px;color:var(--muted2)}.lp-footer-social{display:flex;gap:12px}.lp-footer-social-link{width:32px;height:32px;background:var(--s2);border:1px solid var(--border);border-radius:6px;display:flex;align-items:center;justify-content:center;color:var(--muted);cursor:pointer;transition:all .15s;font-size:14px;text-decoration:none}.lp-footer-social-link:hover{background:var(--s3);color:var(--text);border-color:var(--b2)}@media(max-width:1024px){:root{--page-padding:32px}}@media(max-width:768px){:root{--page-padding:20px}}`;

export default function LandingPage() {
  const { text: rotatingText, phase } = useRotatingText(ROTATING_ITEMS);
  const [activeLang, setActiveLang] = useState("js");
  const langs = [{ id: "js", label: "JavaScript" }, { id: "python", label: "Python" }, { id: "go", label: "Go" }, { id: "curl", label: "cURL" }];

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />

      {/* NAV */}
      <nav className="lp-nav">
        <div className="lp-nav-inner">
          <Link href="/" className="lp-logo">
            <div className="lp-logo-mark"><ZapIcon /></div>
            <span className="lp-logo-name">UniPost</span>
          </Link>
          <div className="lp-nav-links">
            <Link href="/docs" className="lp-nav-link">Docs</Link>
            <Link href="/pricing" className="lp-nav-link">Pricing</Link>
          </div>
          <MarketingNav />
        </div>
      </nav>

      <div className="lp-page">
        {/* HERO */}
        <div className="lp-hero">
          <div className="lp-hero-badge"><span className="lp-hero-badge-dot" />Now supporting 6 platforms</div>
          <h1 className="lp-hero-title">Ship social media<br />integrations for <em>your</em></h1>
          <div className="lp-hero-rotate-wrap" aria-live="polite"><span className={`lp-hero-rotate-text ${phase}`}>{rotatingText}.</span></div>
          <p className="lp-hero-sub">UniPost gives your app a unified API to post, schedule, and analyze across all major social platforms. Ship in hours, not weeks.</p>
          <div className="lp-hero-actions">
            <MarketingCTA />
            <Link href="/docs" className="lp-btn lp-btn-outline lp-btn-lg">View Docs →</Link>
          </div>
          <div className="lp-hero-meta">
            <div className="lp-hero-meta-item"><CheckIcon /><span>Free plan · 100 posts/month</span></div>
            <div className="lp-hero-meta-item"><CheckIcon /><span>No credit card required</span></div>
            <div className="lp-hero-meta-item"><CheckIcon /><span>6 platforms supported</span></div>
          </div>
        </div>

        {/* PLATFORMS */}
        <div className="lp-platforms">
          <div className="lp-plat-label">Supported Platforms</div>
          <div className="lp-plat-row">
            {PLATFORMS.map((p) => (<div key={p.name} className="lp-plat-chip"><span className="lp-plat-icon">{p.icon}</span>{p.name}</div>))}
          </div>
        </div>

        {/* CODE DEMO */}
        <div className="lp-code-section">
          <div className="lp-section-eyebrow">Simple by Design</div>
          <h2 className="lp-section-title">Post everywhere.<br />With one API call.</h2>
          <p className="lp-section-sub">Drop-in REST calls replace dozens of separate APIs. Our example code gets you live the same day.</p>
          <div className="lp-code-wrap">
            <div className="lp-code-topbar">
              <div className="lp-code-dot lp-code-dot-r" /><div className="lp-code-dot lp-code-dot-y" /><div className="lp-code-dot lp-code-dot-g" />
              <div className="lp-code-tabs">
                {langs.map((l) => (<button key={l.id} className={`lp-code-tab ${activeLang === l.id ? "active" : ""}`} onClick={() => setActiveLang(l.id)}>{l.label}</button>))}
              </div>
            </div>
            <pre className="lp-code-body">{CODE_SNIPPETS[activeLang]}</pre>
          </div>
        </div>

        {/* FEATURES */}
        <div className="lp-features">
          <div className="lp-section-eyebrow">Why UniPost</div>
          <h2 className="lp-section-title">Everything you need.<br />Nothing you don&apos;t.</h2>
          <p className="lp-section-sub" style={{ marginBottom: 64 }}>Stop maintaining 6 different API integrations. One integration handles everything.</p>
          <div className="lp-feat-list">
            {FEATURES.map((f, i) => (
              <div key={f.number} className={`lp-feat-item ${i % 2 !== 0 ? "reverse" : ""}`}>
                <div><div className="lp-feat-number">{f.number}</div><h3 className="lp-feat-title">{f.title}</h3><p className="lp-feat-desc">{f.desc}</p></div>
                <pre className="lp-feat-code">{f.code}</pre>
              </div>
            ))}
          </div>
        </div>

        {/* STATS */}
        <div className="lp-stats">
          <div className="lp-stats-inner">
            {[{ num: "6", label: "Platforms supported" }, { num: "1", label: "API call to post everywhere" }, { num: "<1h", label: "Average integration time" }, { num: "∞", label: "Social accounts per project" }].map((s) => (
              <div key={s.num} className="lp-stat"><div className="lp-stat-num">{s.num}</div><div className="lp-stat-label">{s.label}</div></div>
            ))}
          </div>
        </div>

        {/* MODES */}
        <div className="lp-modes">
          <div className="lp-section-eyebrow">Two Modes</div>
          <h2 className="lp-section-title" style={{ marginBottom: 12 }}>Start fast. Scale properly.</h2>
          <p className="lp-section-sub">Choose how UniPost integrates into your product. Switch anytime.</p>
          <div className="lp-modes-grid">
            {MODES.map((m) => (
              <div key={m.badge} className="lp-mode-card">
                <div className="lp-mode-badge" style={{ background: m.badgeColor + "15", border: `1px solid ${m.badgeColor}25`, color: m.badgeColor }}>{m.badge}</div>
                <h3 className="lp-mode-title">{m.title}</h3>
                <p className="lp-mode-desc">{m.desc}</p>
                <ul className="lp-mode-feats">{m.features.map((f) => (<li key={f} className="lp-mode-feat"><CheckIcon />{f}</li>))}</ul>
                <MarketingCTALight />
              </div>
            ))}
          </div>
        </div>

        {/* FAQ */}
        <div className="lp-faq">
          <div className="lp-section-eyebrow" style={{ textAlign: "center" }}>FAQ</div>
          <h2 className="lp-section-title" style={{ textAlign: "center", marginBottom: 48 }}>Common questions</h2>
          <div className="lp-faq-grid">
            {FAQS.map((f) => (<div key={f.q} className="lp-faq-item"><div className="lp-faq-q">{f.q}</div><div className="lp-faq-a">{f.a}</div></div>))}
          </div>
        </div>

        {/* CTA */}
        <div className="lp-cta">
          <div className="lp-cta-inner">
            <div className="lp-cta-glow" />
            <h2 className="lp-cta-title">Start building today</h2>
            <p className="lp-cta-sub">Free plan includes 100 posts/month. No credit card required.</p>
            <div className="lp-cta-actions">
              <MarketingCTA />
              <Link href="/docs" className="lp-btn lp-btn-outline lp-btn-lg">Read the Docs →</Link>
            </div>
          </div>
        </div>
      </div>

      {/* FOOTER */}
      <footer className="lp-footer">
        <div className="lp-footer-inner">
          <div className="lp-footer-top">
            <div><div className="lp-footer-logo"><div className="lp-footer-mark"><ZapIcon /></div><span className="lp-footer-name">UniPost</span></div><p className="lp-footer-tagline">Unified social media API for developers. Post to 6 platforms with one API call.</p></div>
            <div><div className="lp-footer-col-title">Product</div><ul className="lp-footer-links"><li><Link href="/" className="lp-footer-link">Overview</Link></li><li><Link href="/pricing" className="lp-footer-link">Pricing</Link></li><li><Link href="/docs" className="lp-footer-link">Docs</Link></li></ul></div>
            <div><div className="lp-footer-col-title">Developers</div><ul className="lp-footer-links"><li><Link href="/docs" className="lp-footer-link">API Reference</Link></li><li><Link href="/docs" className="lp-footer-link">MCP Server</Link></li></ul></div>
            <div><div className="lp-footer-col-title">Legal</div><ul className="lp-footer-links"><li><Link href="/privacy" className="lp-footer-link">Privacy</Link></li><li><Link href="/terms" className="lp-footer-link">Terms</Link></li></ul></div>
          </div>
          <div className="lp-footer-bottom">
            <div className="lp-footer-copy">&copy; 2026 UniPost. All rights reserved.</div>
            <div className="lp-footer-social">
              <a href="https://x.com/unipostdev" className="lp-footer-social-link" target="_blank" rel="noopener noreferrer">𝕏</a>
            </div>
          </div>
        </div>
      </footer>
    </>
  );
}
