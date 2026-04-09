"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { MarketingNav, MarketingCTA, MarketingCTALight } from "@/components/marketing/nav";

// ── Rotating subtitle hook (slide animation) ──
function useRotatingText<T>(items: T[], interval = 2500) {
  const [index, setIndex] = useState(0);
  const [phase, setPhase] = useState<"visible" | "exit" | "enter">("visible");
  useEffect(() => {
    const timer = setInterval(() => {
      setPhase("exit");
      setTimeout(() => {
        setIndex((i) => (i + 1) % items.length);
        setPhase("enter");
        setTimeout(() => setPhase("visible"), 50);
      }, 500);
    }, interval);
    return () => clearInterval(timer);
  }, [items, interval]);
  return { item: items[index], phase };
}

// ── Data ──
const ROTATING_ITEMS = [
  { text: "AI content generators", color: "#a78bfa" },
  { text: "social schedulers", color: "#38bdf8" },
  { text: "SaaS products", color: "#10b981" },
  { text: "marketing tools", color: "#fb923c" },
  { text: "e-commerce platforms", color: "#f472b6" },
];
const PLATFORM_ICONS: Record<string, React.ReactNode> = {
  Bluesky: <svg width="18" height="18" viewBox="0 0 600 530" fill="#0085ff"><path d="M135.7 44.3C202.3 94.8 273.6 197.2 300 249.6c26.4-52.4 97.7-154.8 164.3-205.3C520.4 1.5 588 -22.1 588 68.2c0 18 -10.4 151.2-16.5 172.8-21.2 75-98.6 94.1-167.9 82.6 121.1 20.7 151.8 89.2 85.3 157.8C390.5 584.2 310.2 500 300 481.4c-10.2 18.6-90.5 102.8-188.9 0C44.6 413.8 75.3 345.3 196.4 324.6c-69.3 11.5-146.7-7.6-167.9-82.6C22.4 220.4 12 87.2 12 69.2c0-90.3 67.6-66.7 123.7-24.9z"/></svg>,
  LinkedIn: <svg width="18" height="18" viewBox="0 0 24 24" fill="#0a66c2"><path d="M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 5.455v6.286zM5.337 7.433a2.062 2.062 0 01-2.063-2.065 2.064 2.064 0 112.063 2.065zm1.782 13.019H3.555V9h3.564v11.452zM22.225 0H1.771C.792 0 0 .774 0 1.729v20.542C0 23.227.792 24 1.771 24h20.451C23.2 24 24 23.227 24 22.271V1.729C24 .774 23.2 0 22.222 0h.003z"/></svg>,
  Instagram: <svg width="18" height="18" viewBox="0 0 24 24" fill="url(#ig)"><defs><radialGradient id="ig" cx="30%" cy="107%" r="150%"><stop offset="0%" stopColor="#fdf497"/><stop offset="5%" stopColor="#fdf497"/><stop offset="45%" stopColor="#fd5949"/><stop offset="60%" stopColor="#d6249f"/><stop offset="90%" stopColor="#285AEB"/></radialGradient></defs><path d="M12 2.163c3.204 0 3.584.012 4.85.07 3.252.148 4.771 1.691 4.919 4.919.058 1.265.069 1.645.069 4.849 0 3.205-.012 3.584-.069 4.849-.149 3.225-1.664 4.771-4.919 4.919-1.266.058-1.644.07-4.85.07-3.204 0-3.584-.012-4.849-.07-3.26-.149-4.771-1.699-4.919-4.92-.058-1.265-.07-1.644-.07-4.849 0-3.204.013-3.583.07-4.849.149-3.227 1.664-4.771 4.919-4.919 1.266-.057 1.645-.069 4.849-.069zM12 0C8.741 0 8.333.014 7.053.072 2.695.272.273 2.69.073 7.052.014 8.333 0 8.741 0 12c0 3.259.014 3.668.072 4.948.2 4.358 2.618 6.78 6.98 6.98C8.333 23.986 8.741 24 12 24c3.259 0 3.668-.014 4.948-.072 4.354-.2 6.782-2.618 6.979-6.98.059-1.28.073-1.689.073-4.948 0-3.259-.014-3.667-.072-4.947-.196-4.354-2.617-6.78-6.979-6.98C15.668.014 15.259 0 12 0zm0 5.838a6.162 6.162 0 100 12.324 6.162 6.162 0 000-12.324zM12 16a4 4 0 110-8 4 4 0 010 8zm6.406-11.845a1.44 1.44 0 100 2.881 1.44 1.44 0 000-2.881z"/></svg>,
  Threads: <svg width="18" height="18" viewBox="0 0 192 192" fill="#ffffff"><path d="M141.537 88.988a66.667 66.667 0 0 0-2.518-1.143c-1.482-27.307-16.403-42.94-41.457-43.1h-.34c-14.986 0-27.449 6.396-35.12 18.036l13.779 9.452c5.73-8.695 14.724-10.548 21.348-10.548h.229c8.249.053 14.474 2.452 18.503 7.129 2.932 3.405 4.893 8.111 5.864 14.05-7.314-1.243-15.224-1.626-23.68-1.14-23.82 1.371-39.134 15.326-38.092 34.7.528 9.818 5.235 18.28 13.256 23.808 6.768 4.666 15.471 6.98 24.49 6.52 11.918-.607 21.27-5.003 27.79-13.066 4.947-6.116 8.1-13.908 9.532-23.619 5.708 3.45 9.953 8.063 12.37 13.676 4.106 9.533 4.349 25.194-7.865 37.315-10.724 10.64-23.618 15.254-38.399 15.358-16.388-.115-28.796-5.382-36.877-15.66-7.515-9.56-11.416-23.12-11.594-40.322.178-17.202 4.079-30.762 11.594-40.322 8.081-10.278 20.489-15.545 36.877-15.66 16.506.116 29.148 5.42 37.567 15.76 4.108 5.048 7.21 11.467 9.312 19.023l14.854-3.982c-2.605-9.463-6.641-17.573-12.159-24.356C152.088 14.14 136.308 7.353 116.379 7.2h-.069c-19.874.142-35.468 6.947-46.333 20.25C60.4 39.452 55.545 55.77 55.33 75.94l-.002.162.002.16c.215 20.17 5.07 36.488 14.645 48.49 10.865 13.303 26.459 20.108 46.333 20.25h.069c18.134-.119 33.577-5.86 45.916-17.068 16.456-14.938 17.617-36.986 12.28-49.39-3.835-8.908-11.151-16.063-21.036-20.544zm-36.844 51.014c-9.985.508-20.361-3.928-21.025-13.278-.477-6.732 4.746-14.243 24.298-15.368 2.132-.123 4.22-.183 6.263-.183 6.26 0 12.12.616 17.39 1.812-1.98 22.459-14.948 26.513-26.926 27.017z"/></svg>,
  TikTok: <svg width="18" height="18" viewBox="0 0 24 24" fill="#ffffff"><path d="M19.59 6.69a4.83 4.83 0 01-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 01-2.88 2.5 2.89 2.89 0 01-2.89-2.89 2.89 2.89 0 012.89-2.89c.28 0 .54.04.79.1v-3.5a6.37 6.37 0 00-.79-.05A6.34 6.34 0 003.15 15.2a6.34 6.34 0 0010.86 4.48 6.3 6.3 0 001.86-4.48V8.73a8.26 8.26 0 004.84 1.56V6.84a4.85 4.85 0 01-1.12-.15z"/></svg>,
  YouTube: <svg width="18" height="18" viewBox="0 0 24 24" fill="#ff0000"><path d="M23.498 6.186a3.016 3.016 0 00-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 00.502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 002.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 002.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"/></svg>,
};

const PLATFORMS = [
  { name: "Bluesky", slug: "bluesky" }, { name: "LinkedIn", slug: "linkedin" }, { name: "Instagram", slug: "instagram" },
  { name: "Threads", slug: "threads" }, { name: "TikTok", slug: "tiktok" }, { name: "YouTube", slug: "youtube" },
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
const CSS = `@import url('https://fonts.googleapis.com/css2?family=DM+Sans:opsz,wght@9..40,300;9..40,400;9..40,500;9..40,600;9..40,700;9..40,800;9..40,900&family=Fira+Code:wght@400;500&display=swap');:root{--bg:#000;--s1:#0a0a0a;--s2:#111;--s3:#1a1a1a;--border:#1a1a1a;--b2:#242424;--b3:#2e2e2e;--text:#f0f0f0;--muted:#999;--muted2:#555;--accent:#10b981;--adim:#10b98112;--blue:#0ea5e9;--blue-dim:#0ea5e912;--r:8px;--mono:'Fira Code',monospace;--ui:'DM Sans',system-ui,sans-serif;--nav-max:1480px;--content-max:1320px;--text-max:720px;--px:32px;--section-py:112px}*{box-sizing:border-box;margin:0;padding:0}body{background:var(--bg);color:var(--text);font-family:var(--ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}.lp-nav{position:sticky;top:0;z-index:50;width:100%;border-bottom:1px solid var(--border);background:#00000095;backdrop-filter:blur(12px);-webkit-backdrop-filter:blur(12px)}.lp-nav-inner{max-width:var(--nav-max);margin:0 auto;padding:0 var(--px);height:56px;display:flex;align-items:center;justify-content:space-between}.lp-logo{display:flex;align-items:center;gap:10px;text-decoration:none}.lp-logo-mark{width:28px;height:28px;background:var(--accent);border-radius:7px;display:flex;align-items:center;justify-content:center}.lp-logo-mark svg{width:14px;height:14px;color:#000}.lp-logo-name{font-size:16px;font-weight:700;letter-spacing:-.4px;color:var(--text)}.lp-nav-links{display:flex;align-items:center;gap:4px}.lp-nav-link{padding:6px 14px;font-size:14px;font-weight:500;color:var(--muted);cursor:pointer;border-radius:var(--r);transition:color .1s;text-decoration:none}.lp-nav-link:hover{color:var(--text)}.lp-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--ui);text-decoration:none;white-space:nowrap}.lp-btn-primary{background:var(--blue);color:#000}.lp-btn-primary:hover{background:#38bdf8;box-shadow:0 0 24px #0ea5e930}.lp-btn-ghost{background:transparent;color:var(--muted);border-color:var(--b2)}.lp-btn-ghost:hover{background:var(--s2);color:var(--text);border-color:var(--b3)}.lp-btn-outline{background:transparent;color:var(--text);border-color:var(--b2)}.lp-btn-outline:hover{background:var(--s2);border-color:var(--b3)}.lp-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}.lp-btn svg{width:14px;height:14px}.lp-page{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}.lp-hero{padding:var(--section-py) 0;text-align:center;display:flex;flex-direction:column;align-items:center}.lp-hero-badge{display:inline-flex;align-items:center;gap:7px;padding:5px 14px;border-radius:20px;background:var(--adim);border:1px solid #10b98120;font-size:12.5px;color:var(--accent);font-weight:600;margin-bottom:32px;font-family:var(--mono)}.lp-hero-badge-dot{width:6px;height:6px;border-radius:50%;background:var(--accent);animation:lp-pulse 2s infinite}@keyframes lp-pulse{0%,100%{opacity:1}50%{opacity:.3}}.lp-hero-title{font-size:76px;font-weight:900;letter-spacing:-2.5px;line-height:1;color:var(--text);margin-bottom:24px;max-width:900px;text-align:center}.lp-hero-title em{color:var(--accent);font-style:normal}.lp-hero-rotate-wrap{font-size:60px;font-weight:800;letter-spacing:-2px;line-height:1;margin-bottom:36px;text-align:center;height:70px;width:100%;max-width:var(--text-max);display:flex;align-items:center;justify-content:center;overflow:hidden;position:relative}.lp-hero-rotate-text{position:absolute;display:inline-block;transition:transform .5s cubic-bezier(.4,0,.2,1),opacity .5s cubic-bezier(.4,0,.2,1);will-change:transform,opacity;white-space:nowrap}.lp-hero-rotate-text.visible{transform:translateY(0);opacity:1}.lp-hero-rotate-text.exit{transform:translateY(-40px);opacity:0}.lp-hero-rotate-text.enter{transform:translateY(40px);opacity:0;transition:none}.lp-hero-sub{font-size:17px;color:#aaa;max-width:var(--text-max);line-height:1.75;margin-bottom:44px;text-align:center}.lp-hero-actions{display:flex;align-items:center;gap:12px;margin-bottom:28px;justify-content:center}.lp-hero-meta{font-size:13px;color:var(--muted2);display:flex;align-items:center;gap:16px;justify-content:center}.lp-hero-meta-item{display:flex;align-items:center;gap:6px}.lp-hero-meta-item svg{width:13px;height:13px;color:var(--accent)}.lp-platforms{padding:0 0 var(--section-py)}.lp-plat-label{font-size:11.5px;color:#777;text-transform:uppercase;letter-spacing:.12em;font-weight:700;margin-bottom:24px;font-family:var(--mono);text-align:center}.lp-plat-row{display:flex;align-items:center;gap:12px;flex-wrap:wrap;justify-content:center}.lp-plat-chip{display:flex;align-items:center;gap:9px;padding:9px 18px;background:var(--s1);border:1px solid var(--b2);border-radius:24px;font-size:14px;font-weight:500;color:#ccc;transition:all .15s;cursor:default}.lp-plat-chip:hover{border-color:var(--b2);color:var(--text);background:var(--s2)}.lp-plat-icon{display:flex;align-items:center;justify-content:center;width:18px;height:18px;flex-shrink:0}.lp-code-section{padding:0 0 var(--section-py)}.lp-section-eyebrow{font-size:11.5px;color:var(--accent);text-transform:uppercase;letter-spacing:.12em;font-weight:700;margin-bottom:12px;font-family:var(--mono);text-align:center}.lp-section-title{font-size:44px;font-weight:800;letter-spacing:-1px;margin-bottom:12px;line-height:1.1;text-align:center}.lp-section-sub{font-size:16px;color:#aaa;max-width:var(--text-max);line-height:1.7;margin-bottom:48px;text-align:center;margin-left:auto;margin-right:auto}.lp-integ-grid{display:grid;grid-template-columns:1fr 1.3fr;gap:48px;align-items:start}.lp-integ-left{padding-top:16px}.lp-integ-title{font-size:32px;font-weight:800;letter-spacing:-.6px;line-height:1.2;color:var(--text);margin-bottom:40px}.lp-integ-cards{display:flex;flex-direction:column;gap:0}.lp-integ-card{display:flex;gap:14px;align-items:flex-start;padding:16px 0}.lp-integ-card-icon{width:36px;height:36px;border-radius:8px;background:var(--s2);border:1px solid var(--b2);display:flex;align-items:center;justify-content:center;flex-shrink:0;color:var(--accent)}.lp-integ-card-title{font-size:15px;font-weight:700;color:var(--text);margin-bottom:4px}.lp-integ-card-desc{font-size:13.5px;color:#aaa;line-height:1.5;margin-bottom:6px}.lp-integ-card-link{font-size:12.5px;color:var(--accent);text-decoration:none;font-weight:600;font-family:var(--mono)}.lp-integ-card-link:hover{text-decoration:underline}.lp-integ-card-divider{height:1px;background:var(--border);margin:4px 0}.lp-integ-right{}.lp-code-tabs-bar{display:flex;gap:2px;margin-bottom:0;background:var(--s2);border:1px solid var(--border);border-bottom:none;border-radius:10px 10px 0 0;padding:10px 16px}.lp-code-tab{padding:5px 14px;border-radius:6px;font-size:12.5px;font-weight:500;color:var(--muted);cursor:pointer;font-family:var(--mono);transition:all .1s;border:1px solid transparent;background:none}.lp-code-tab:hover{color:var(--text)}.lp-code-tab.active{background:var(--s3);color:var(--text);border-color:var(--b2)}.lp-editor{background:#1e1e2e;border:1px solid var(--border);border-top:1px solid var(--border);border-radius:0 0 10px 10px;overflow:hidden}.lp-editor-code{padding:20px 0;margin:0;font-family:var(--mono);font-size:13px;line-height:1.8;overflow-x:auto}.lp-editor-line{display:flex;padding:0 20px 0 0}.lp-editor-line:hover{background:#ffffff06}.lp-editor-ln{width:44px;text-align:right;padding-right:16px;color:#555;user-select:none;flex-shrink:0;font-size:12px}.lp-editor-text{color:#cdd6f4;white-space:pre}.lp-features{padding:0 0 var(--section-py)}.lp-feat-list{display:flex;flex-direction:column;gap:80px}.lp-feat-item{display:grid;grid-template-columns:1fr 1fr;gap:64px;align-items:center}.lp-feat-item.reverse{direction:rtl}.lp-feat-item.reverse>*{direction:ltr}.lp-feat-number{font-family:var(--mono);font-size:11px;color:var(--muted2);font-weight:600;letter-spacing:.1em;margin-bottom:16px}.lp-feat-title{font-size:32px;font-weight:800;letter-spacing:-.6px;line-height:1.15;margin-bottom:16px}.lp-feat-desc{font-size:15px;color:#aaa;line-height:1.75}.lp-feat-code{background:var(--s1);border:1px solid var(--border);border-radius:10px;padding:24px 28px;font-family:var(--mono);font-size:13px;line-height:1.7;color:#b0b0b0;white-space:pre;overflow-x:auto}.lp-stats{padding:0 0 var(--section-py)}.lp-stats-inner{border:1px solid var(--border);border-radius:14px;display:grid;grid-template-columns:repeat(4,1fr);overflow:hidden}.lp-stat{padding:40px 36px;border-right:1px solid var(--border)}.lp-stat:last-child{border-right:none}.lp-stat-num{font-family:var(--mono);font-size:40px;font-weight:700;color:var(--accent);letter-spacing:-1px;margin-bottom:8px}.lp-stat-label{font-size:14px;color:#aaa;line-height:1.5}.lp-modes{padding:0 0 var(--section-py)}.lp-modes-grid{display:grid;grid-template-columns:1fr 1fr;gap:16px}.lp-mode-card{background:var(--s1);border:1px solid var(--b2);border-radius:16px;padding:40px;transition:all .25s;position:relative;overflow:hidden}.lp-mode-card::before{content:'';position:absolute;top:0;left:0;right:0;height:3px;border-radius:16px 16px 0 0}.lp-mode-card.mode-quickstart::before{background:linear-gradient(90deg,#10b981,#34d399)}.lp-mode-card.mode-native::before{background:linear-gradient(90deg,#3b82f6,#60a5fa)}.lp-mode-card:hover{border-color:#333;transform:translateY(-2px);box-shadow:0 8px 32px #00000040}.lp-mode-card.mode-quickstart:hover{box-shadow:0 8px 32px #10b98110}.lp-mode-card.mode-native:hover{box-shadow:0 8px 32px #3b82f610}.lp-mode-badge{display:inline-flex;align-items:center;gap:7px;padding:6px 16px;border-radius:8px;font-size:13px;font-weight:700;font-family:var(--mono);margin-bottom:24px;letter-spacing:.02em}.lp-mode-icon{width:28px;height:28px;border-radius:7px;display:flex;align-items:center;justify-content:center;flex-shrink:0}.lp-mode-title{font-size:26px;font-weight:800;letter-spacing:-.5px;margin-bottom:12px;line-height:1.2}.lp-mode-desc{font-size:14.5px;color:#aaa;line-height:1.7;margin-bottom:28px}.lp-mode-feats{list-style:none;margin-bottom:32px}.lp-mode-feat{display:flex;align-items:flex-start;gap:11px;font-size:14px;color:#ccc;margin-bottom:11px}.lp-mode-feat svg{width:14px;height:14px;color:var(--accent);flex-shrink:0;margin-top:3px}.lp-faq-band{background:#0c0c0c;border-top:1px solid #161616;padding:var(--section-py) 0}.lp-faq-band-inner{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}.lp-faq-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.lp-faq-item{background:var(--s1);border:1px solid var(--border);border-radius:10px;padding:24px 26px;transition:border-color .15s}.lp-faq-item:hover{border-color:var(--b2)}.lp-faq-q{font-size:15px;font-weight:600;margin-bottom:10px}.lp-faq-a{font-size:13.5px;color:#aaa;line-height:1.7}.lp-cta-band{background:#080808;padding:var(--section-py) 0}.lp-cta-band-inner{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}.lp-cta-inner{background:#0f0f0f;border:1px solid #1a1a1a;border-radius:16px;padding:80px 64px;text-align:center;position:relative;overflow:hidden}.lp-cta-glow{position:absolute;width:400px;height:400px;border-radius:50%;background:radial-gradient(circle,#10b98110,transparent 70%);top:50%;left:50%;transform:translate(-50%,-50%);pointer-events:none}.lp-cta-title{font-size:52px;font-weight:900;letter-spacing:-1.5px;margin-bottom:16px;position:relative}.lp-cta-sub{font-size:16px;color:#aaa;margin-bottom:40px;position:relative}.lp-cta-actions{display:flex;align-items:center;justify-content:center;gap:12px;position:relative}.lp-footer{width:100%;border-top:1px solid var(--border);padding:48px 0}.lp-footer-inner{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}.lp-footer-top{display:grid;grid-template-columns:2fr 1fr 1fr 1fr;gap:48px;margin-bottom:48px}.lp-footer-logo{display:flex;align-items:center;gap:9px;margin-bottom:16px}.lp-footer-mark{width:26px;height:26px;background:var(--accent);border-radius:6px;display:flex;align-items:center;justify-content:center}.lp-footer-mark svg{width:13px;height:13px;color:#000}.lp-footer-name{font-size:15px;font-weight:700;color:var(--text)}.lp-footer-tagline{font-size:13px;color:#aaa;line-height:1.65;max-width:260px}.lp-footer-col-title{font-size:12px;font-weight:700;text-transform:uppercase;letter-spacing:.08em;color:var(--muted2);margin-bottom:16px}.lp-footer-links{list-style:none}.lp-footer-link{font-size:13.5px;color:#aaa;margin-bottom:10px;cursor:pointer;transition:color .1s;display:block;text-decoration:none}.lp-footer-link:hover{color:var(--text)}.lp-footer-bottom{border-top:1px solid var(--border);padding-top:24px;display:flex;align-items:center;justify-content:space-between}.lp-footer-copy{font-size:13px;color:var(--muted2)}.lp-footer-social{display:flex;gap:12px}.lp-footer-social-link{width:32px;height:32px;background:var(--s2);border:1px solid var(--border);border-radius:6px;display:flex;align-items:center;justify-content:center;color:var(--muted);cursor:pointer;transition:all .15s;font-size:14px;text-decoration:none}.lp-footer-social-link:hover{background:var(--s3);color:var(--text);border-color:var(--b2)}@media(min-width:1600px){:root{--nav-max:1560px;--content-max:1360px;--px:40px}}@media(max-width:1024px){:root{--nav-max:100%;--content-max:100%;--px:24px;--section-py:80px}}`;

export default function LandingPage() {
  const { item: rotatingItem, phase: rotatePhase } = useRotatingText(ROTATING_ITEMS);
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
            <Link href="/solutions" className="lp-nav-link">Solutions</Link>
            <Link href="/tools" className="lp-nav-link">Tools</Link>
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
          <div className="lp-hero-rotate-wrap" aria-live="polite"><span className={`lp-hero-rotate-text ${rotatePhase}`} style={{ color: rotatingItem.color }}>{rotatingItem.text}.</span></div>
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
            {PLATFORMS.map((p) => (<Link key={p.name} href={`/${p.slug}-api`} className="lp-plat-chip" style={{ textDecoration: "none" }}><span className="lp-plat-icon">{PLATFORM_ICONS[p.name]}</span>{p.name}</Link>))}
          </div>
        </div>

        {/* CODE DEMO — two-column layout */}
        <div className="lp-code-section">
          <div className="lp-integ-grid">
            {/* Left: title + integration cards */}
            <div className="lp-integ-left">
              <h2 className="lp-integ-title">One API call. Six platforms. Zero token headaches.</h2>

              <div className="lp-integ-cards">
                <div className="lp-integ-card">
                  <div className="lp-integ-card-icon">
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><circle cx="12" cy="12" r="3"/><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/></svg>
                  </div>
                  <div>
                    <div className="lp-integ-card-title">REST API</div>
                    <div className="lp-integ-card-desc">Simple, single point of entry for every platform.</div>
                    <Link href="/docs" className="lp-integ-card-link">Docs ↗</Link>
                  </div>
                </div>
                <div className="lp-integ-card-divider" />
                <div className="lp-integ-card">
                  <div className="lp-integ-card-icon">
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                  </div>
                  <div>
                    <div className="lp-integ-card-title">Webhooks</div>
                    <div className="lp-integ-card-desc">Real-time account connections and post status.</div>
                    <Link href="/docs#webhooks" className="lp-integ-card-link">Docs ↗</Link>
                  </div>
                </div>
              </div>
            </div>

            {/* Right: code editor */}
            <div className="lp-integ-right">
              <div className="lp-code-tabs-bar">
                {langs.map((l) => (<button key={l.id} className={`lp-code-tab ${activeLang === l.id ? "active" : ""}`} onClick={() => setActiveLang(l.id)}>{l.label}</button>))}
              </div>
              <div className="lp-editor">
                <pre className="lp-editor-code">{CODE_SNIPPETS[activeLang].split("\n").map((line, i) => (
                  <div key={i} className="lp-editor-line">
                    <span className="lp-editor-ln">{i + 1}</span>
                    <span className="lp-editor-text">{line}</span>
                  </div>
                ))}</pre>
              </div>
            </div>
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
            <div><div className="lp-footer-col-title">Platforms</div><ul className="lp-footer-links">{PLATFORMS.map((p) => (<li key={p.slug}><Link href={`/${p.slug}-api`} className="lp-footer-link">{p.name}</Link></li>))}</ul></div>
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
