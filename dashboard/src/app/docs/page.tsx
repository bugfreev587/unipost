"use client";

import Link from "next/link";
import { useState, useEffect, useRef } from "react";
import { MarketingNav } from "@/components/marketing/nav";

const BASE = "https://api.unipost.dev";

const NAV_ITEMS = [
  ["overview", "Overview"],
  ["authentication", "Authentication"],
  ["quick-start", "Quick Start"],
  ["mcp", "MCP / AI Agents"],
  ["capabilities", "Capabilities"],
  ["social-accounts", "Social Accounts"],
  ["social-posts", "Social Posts"],
  ["validate", "Validate (preflight)"],
  ["drafts", "Drafts & Preview"],
  ["threads", "Threads"],
  ["media", "Media library"],
  ["account-health", "Account Health"],
  ["analytics", "Analytics"],
  ["webhooks", "Webhooks"],
  ["oauth", "OAuth Flow"],
  ["billing", "Billing & Usage"],
  ["errors", "Error Handling"],
  ["platforms", "Supported Platforms"],
];

// ── Styles ──
const CSS = `@import url('https://fonts.googleapis.com/css2?family=DM+Sans:opsz,wght@9..40,300;9..40,400;9..40,500;9..40,600;9..40,700;9..40,800&family=Fira+Code:wght@400;500&display=swap');:root{--bg:#000;--s1:#0a0a0a;--s2:#111;--s3:#1a1a1a;--border:#1a1a1a;--b2:#242424;--b3:#2e2e2e;--text:#f0f0f0;--muted:#666;--muted2:#333;--accent:#10b981;--blue:#0ea5e9;--r:8px;--mono:'Fira Code',monospace;--ui:'DM Sans',system-ui,sans-serif;--nav-max:1480px;--content-max:1320px;--px:32px}*{box-sizing:border-box;margin:0;padding:0}body{background:var(--bg);color:var(--text);font-family:var(--ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}.doc-nav{position:sticky;top:0;z-index:50;border-bottom:1px solid var(--border);background:#00000095;backdrop-filter:blur(12px)}.doc-nav-inner{max-width:var(--nav-max);margin:0 auto;padding:0 var(--px);height:56px;display:flex;align-items:center;justify-content:space-between}.doc-logo{display:flex;align-items:center;gap:10px;text-decoration:none}.doc-logo-mark{width:28px;height:28px;background:var(--accent);border-radius:7px;display:flex;align-items:center;justify-content:center}.doc-logo-mark svg{width:14px;height:14px;color:#000}.doc-logo-name{font-size:16px;font-weight:700;letter-spacing:-.4px;color:var(--text)}.doc-nav-links{display:flex;gap:4px}.doc-nav-link{padding:6px 12px;font-size:13.5px;color:var(--muted);border-radius:var(--r);transition:color .1s;text-decoration:none}.doc-nav-link:hover{color:var(--text)}.doc-nav-link.active{color:var(--text);font-weight:500}.doc-layout{max-width:var(--content-max);margin:0 auto;padding:0 var(--px);display:flex;gap:48px;padding-top:40px;padding-bottom:96px}.doc-sidebar{width:200px;flex-shrink:0;position:sticky;top:96px;align-self:flex-start;max-height:calc(100vh - 120px);overflow-y:auto}.doc-sidebar-list{list-style:none}.doc-sidebar-item{margin-bottom:2px}.doc-sidebar-link{display:block;padding:6px 12px;font-size:13.5px;color:#999;border-radius:6px;text-decoration:none;transition:all .1s}.doc-sidebar-link:hover{color:var(--text);background:var(--s2)}.doc-sidebar-link.active{color:var(--accent);background:var(--s2);font-weight:500}.doc-main{flex:1;min-width:0}.doc-title{font-size:36px;font-weight:800;letter-spacing:-.8px;margin-bottom:8px;color:var(--text)}.doc-subtitle{font-size:17px;color:#aaa;margin-bottom:48px;line-height:1.7}.doc-section{scroll-margin-top:96px;margin-bottom:56px}.doc-section-title{font-size:24px;font-weight:700;letter-spacing:-.4px;margin-bottom:18px;color:var(--text)}.doc-section-title a{color:inherit;text-decoration:none}.doc-section-title a:hover{color:var(--accent)}.doc-p{font-size:15px;color:#bbb;line-height:1.8;margin-bottom:18px}.doc-p a{color:var(--blue);text-decoration:none}.doc-p a:hover{text-decoration:underline}.doc-p code{font-family:var(--mono);font-size:12.5px;background:var(--s2);border:1px solid var(--border);padding:1px 6px;border-radius:4px;color:var(--text)}.doc-endpoint{border:1px solid var(--border);border-radius:10px;margin-bottom:24px;overflow:hidden}.doc-endpoint-header{display:flex;align-items:center;gap:10px;padding:12px 18px;background:var(--s2);border-bottom:1px solid var(--border)}.doc-method{padding:2px 8px;border-radius:4px;font-size:11px;font-weight:700;font-family:var(--mono)}.doc-method-get{background:#10b98120;color:var(--accent)}.doc-method-post{background:#0ea5e920;color:var(--blue)}.doc-method-patch{background:#f59e0b20;color:#f59e0b}.doc-method-delete{background:#ef444420;color:#ef4444}.doc-endpoint-path{font-family:var(--mono);font-size:13px;color:var(--text)}.doc-endpoint-auth{font-size:11px;color:var(--muted);margin-left:auto}.doc-endpoint-body{padding:20px;font-size:15px;color:#bbb;line-height:1.75}.doc-code-wrap{margin:12px 0}.doc-code-label{font-size:11px;color:var(--muted);margin-bottom:4px}.doc-code-box{position:relative}.doc-code{display:flex;background:#161616;border:1px solid var(--b2);border-radius:8px;padding:18px 0;font-family:var(--mono);font-size:13px;line-height:1.75;color:#d4d4d4;overflow-x:auto;white-space:pre;margin:0}.doc-code-box-tabbed .doc-code{border-top:none;border-radius:0 0 8px 8px}.doc-code-gutter{display:flex;flex-direction:column;flex-shrink:0;padding:0 14px 0 18px;color:#555;text-align:right;user-select:none;border-right:1px solid #242424;margin-right:16px}.doc-code-ln{display:block}.doc-code-content{display:block;padding-right:22px;flex:1;min-width:0}.doc-code-copy{position:absolute;top:10px;right:10px;z-index:2;background:#1f1f1f;border:1px solid var(--b2);color:#aaa;font-family:var(--ui);font-size:11px;font-weight:500;padding:5px 10px;border-radius:6px;cursor:pointer;transition:all .12s;letter-spacing:.02em}.doc-code-copy:hover{color:var(--text);background:#2a2a2a;border-color:var(--b3)}.doc-code-tabs-bar{display:flex;gap:2px;background:#161616;border:1px solid var(--b2);border-bottom:1px solid #242424;border-radius:8px 8px 0 0;padding:8px 10px}.doc-code-tab{padding:5px 12px;border-radius:5px;font-size:12px;font-weight:500;color:#888;cursor:pointer;font-family:var(--mono);transition:all .12s;border:1px solid transparent;background:none}.doc-code-tab:hover{color:var(--text)}.doc-code-tab.active{background:#262626;color:var(--text);border-color:#2e2e2e}.doc-param{display:flex;gap:8px;padding:5px 0;font-size:14px;align-items:baseline;flex-wrap:wrap}.doc-param-name{font-family:var(--mono);color:var(--blue);white-space:nowrap}.doc-param-type{font-size:11px;color:var(--muted2)}.doc-param-req{font-size:11px;color:#ef4444}.doc-param-desc{color:#aaa}.doc-param-desc code{font-family:var(--mono);font-size:11.5px;background:var(--s2);border:1px solid var(--border);padding:0 5px;border-radius:3px;color:var(--text)}.doc-callout{padding:16px 20px;border-radius:8px;font-size:14px;line-height:1.7;margin:18px 0}.doc-callout-info{background:var(--s2);border:1px solid var(--border);color:#aaa}.doc-callout-warn{background:#f59e0b08;border:1px solid #f59e0b20;color:#f59e0b}.doc-callout strong{color:var(--text)}.doc-grid-2{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin:16px 0}.doc-grid-card{border:1px solid var(--border);border-radius:8px;padding:14px 18px}.doc-grid-card-title{font-size:13px;font-weight:600;color:var(--text);margin-bottom:4px}.doc-grid-card-val{font-family:var(--mono);font-size:13px;color:var(--blue)}.doc-table{width:100%;border-collapse:collapse;margin:16px 0}.doc-table th{text-align:left;padding:10px 16px;font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:.04em;color:var(--muted);background:var(--s2);border-bottom:1px solid var(--border)}.doc-table td{padding:12px 18px;font-size:14px;border-bottom:1px solid var(--border);color:#bbb}.doc-table td code{font-family:var(--mono);color:#ef4444;font-size:12px}.doc-platform{border:1px solid var(--border);border-radius:8px;padding:14px 18px;margin-bottom:8px}.doc-platform-header{display:flex;align-items:center;justify-content:space-between;margin-bottom:4px}.doc-platform-name{font-size:14px;font-weight:600;color:var(--text)}.doc-platform-auth{font-size:11px;color:var(--muted)}.doc-platform-content{font-size:14px;color:#aaa}.doc-platform-note{font-size:12px;color:#888;margin-top:4px}.doc-footer{border-top:1px solid var(--border);padding:32px 0}.doc-footer-inner{max-width:1100px;margin:0 auto;padding:0 32px;font-size:13px;color:var(--muted)}.doc-footer-inner a{color:var(--blue);text-decoration:none}.doc-footer-inner a:hover{text-decoration:underline}::-webkit-scrollbar{width:5px;height:5px}::-webkit-scrollbar-track{background:transparent}::-webkit-scrollbar-thumb{background:var(--b2);border-radius:3px}.doc-sidebar-link.active{color:var(--accent);background:var(--s2);font-weight:500;border-left:2px solid var(--accent);border-radius:0 6px 6px 0;padding-left:8px}@media(min-width:1600px){:root{--nav-max:1560px;--content-max:1360px;--px:40px}}@media(max-width:1024px){:root{--nav-max:100%;--content-max:100%;--px:24px}}`;

function ZapIcon() { return <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" width="14" height="14"><path d="M9 2L4 9h4l-1 5 5-7H8l1-5z" /></svg>; }

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section id={id} className="doc-section">
      <h2 className="doc-section-title"><a href={`#${id}`}>{title}</a></h2>
      {children}
    </section>
  );
}

function Endpoint({ method, path, auth, children }: { method: string; path: string; auth: string; children: React.ReactNode }) {
  const cls = `doc-method doc-method-${method.toLowerCase()}`;
  return (
    <div className="doc-endpoint">
      <div className="doc-endpoint-header">
        <span className={cls}>{method}</span>
        <code className="doc-endpoint-path">{path}</code>
        <span className="doc-endpoint-auth">{auth}</span>
      </div>
      <div className="doc-endpoint-body">{children}</div>
    </div>
  );
}

type LangId = "js" | "python" | "go" | "curl";
const LANGS: { id: LangId; label: string }[] = [
  { id: "js", label: "JavaScript" },
  { id: "python", label: "Python" },
  { id: "go", label: "Go" },
  { id: "curl", label: "cURL" },
];

type Snippets = Record<LangId, string>;

// ── API request snippets (one set per example) ──
const SN_LIST_ACCOUNTS: Snippets = {
  js: `const response = await fetch(\n  '${BASE}/v1/social-accounts',\n  {\n    headers: {\n      'Authorization': 'Bearer up_live_your_key',\n    },\n  }\n);\n\nconst { data } = await response.json();`,
  python: `import requests\n\nresponse = requests.get(\n    '${BASE}/v1/social-accounts',\n    headers={\n        'Authorization': 'Bearer up_live_your_key',\n    },\n)\n\ndata = response.json()['data']`,
  go: `req, _ := http.NewRequest("GET",\n    "${BASE}/v1/social-accounts", nil)\nreq.Header.Set("Authorization", "Bearer up_live_your_key")\n\nresp, _ := http.DefaultClient.Do(req)`,
  curl: `curl ${BASE}/v1/social-accounts \\\n  -H "Authorization: Bearer up_live_your_key"`,
};

const SN_CONNECT_BLUESKY: Snippets = {
  js: `const response = await fetch(\n  '${BASE}/v1/social-accounts/connect',\n  {\n    method: 'POST',\n    headers: {\n      'Authorization': 'Bearer up_live_your_key',\n      'Content-Type':  'application/json',\n    },\n    body: JSON.stringify({\n      platform: 'bluesky',\n      credentials: {\n        handle:       'alice.bsky.social',\n        app_password: 'xxxx-xxxx-xxxx-xxxx',\n      },\n    }),\n  }\n);\n\nconst { data } = await response.json();`,
  python: `import requests\n\nresponse = requests.post(\n    '${BASE}/v1/social-accounts/connect',\n    headers={\n        'Authorization': 'Bearer up_live_your_key',\n        'Content-Type':  'application/json',\n    },\n    json={\n        'platform': 'bluesky',\n        'credentials': {\n            'handle':       'alice.bsky.social',\n            'app_password': 'xxxx-xxxx-xxxx-xxxx',\n        },\n    },\n)\n\ndata = response.json()['data']`,
  go: `body := strings.NewReader(\`{\n  "platform": "bluesky",\n  "credentials": {\n    "handle":       "alice.bsky.social",\n    "app_password": "xxxx-xxxx-xxxx-xxxx"\n  }\n}\`)\n\nreq, _ := http.NewRequest("POST",\n    "${BASE}/v1/social-accounts/connect", body)\nreq.Header.Set("Authorization", "Bearer up_live_your_key")\nreq.Header.Set("Content-Type",  "application/json")\n\nresp, _ := http.DefaultClient.Do(req)`,
  curl: `curl -X POST ${BASE}/v1/social-accounts/connect \\\n  -H "Authorization: Bearer up_live_your_key" \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "platform": "bluesky",\n    "credentials": {\n      "handle":       "alice.bsky.social",\n      "app_password": "xxxx-xxxx-xxxx-xxxx"\n    }\n  }'`,
};

const SN_DELETE_ACCOUNT: Snippets = {
  js: `const response = await fetch(\n  '${BASE}/v1/social-accounts/sa_abc123',\n  {\n    method: 'DELETE',\n    headers: {\n      'Authorization': 'Bearer up_live_your_key',\n    },\n  }\n);`,
  python: `import requests\n\nresponse = requests.delete(\n    '${BASE}/v1/social-accounts/sa_abc123',\n    headers={\n        'Authorization': 'Bearer up_live_your_key',\n    },\n)`,
  go: `req, _ := http.NewRequest("DELETE",\n    "${BASE}/v1/social-accounts/sa_abc123", nil)\nreq.Header.Set("Authorization", "Bearer up_live_your_key")\n\nresp, _ := http.DefaultClient.Do(req)`,
  curl: `curl -X DELETE ${BASE}/v1/social-accounts/sa_abc123 \\\n  -H "Authorization: Bearer up_live_your_key"`,
};

const SN_CREATE_POST: Snippets = {
  js: `const response = await fetch(\n  '${BASE}/v1/social-posts',\n  {\n    method: 'POST',\n    headers: {\n      'Authorization': 'Bearer up_live_xxxx',\n      'Content-Type':  'application/json',\n    },\n    body: JSON.stringify({\n      caption:     'Hello from UniPost! 🚀',\n      account_ids: ['sa_instagram_123', 'sa_linkedin_456'],\n    }),\n  }\n);\n\nconst { data } = await response.json();\nconsole.log(data.id); // post_abc123`,
  python: `import requests\n\nresponse = requests.post(\n    '${BASE}/v1/social-posts',\n    headers={\n        'Authorization': 'Bearer up_live_xxxx',\n        'Content-Type':  'application/json',\n    },\n    json={\n        'caption':     'Hello from UniPost! 🚀',\n        'account_ids': ['sa_instagram_123', 'sa_linkedin_456'],\n    },\n)\n\ndata = response.json()['data']\nprint(data['id'])  # post_abc123`,
  go: `body := strings.NewReader(\`{\n  "caption":     "Hello from UniPost! 🚀",\n  "account_ids": ["sa_instagram_123", "sa_linkedin_456"]\n}\`)\n\nreq, _ := http.NewRequest("POST",\n    "${BASE}/v1/social-posts", body)\nreq.Header.Set("Authorization", "Bearer up_live_xxxx")\nreq.Header.Set("Content-Type",  "application/json")\n\nresp, _ := http.DefaultClient.Do(req)\n// resp.StatusCode == 200 ✓`,
  curl: `curl -X POST ${BASE}/v1/social-posts \\\n  -H "Authorization: Bearer up_live_xxxx" \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "caption":     "Hello from UniPost! 🚀",\n    "account_ids": ["sa_instagram_123", "sa_linkedin_456"]\n  }'`,
};

const SN_PLATFORM_POSTS: Snippets = {
  js: `// Different caption per platform — recommended shape.
const response = await fetch(
  '${BASE}/v1/social-posts',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer up_live_xxxx',
      'Content-Type':  'application/json',
    },
    body: JSON.stringify({
      platform_posts: [
        { account_id: 'sa_twitter_1',  caption: 'shipped 🚀' },
        { account_id: 'sa_linkedin_1', caption: 'Today we shipped a meaningful improvement to...' },
        { account_id: 'sa_bluesky_1',  caption: 'shipped! 💫' },
      ],
      idempotency_key: 'launch-2026-04-08-001',
    }),
  }
);`,
  python: `import requests

response = requests.post(
    '${BASE}/v1/social-posts',
    headers={
        'Authorization': 'Bearer up_live_xxxx',
        'Content-Type':  'application/json',
    },
    json={
        'platform_posts': [
            {'account_id': 'sa_twitter_1',  'caption': 'shipped 🚀'},
            {'account_id': 'sa_linkedin_1', 'caption': 'Today we shipped...'},
            {'account_id': 'sa_bluesky_1',  'caption': 'shipped! 💫'},
        ],
        'idempotency_key': 'launch-2026-04-08-001',
    },
)`,
  go: `body := strings.NewReader(\`{
  "platform_posts": [
    { "account_id": "sa_twitter_1",  "caption": "shipped 🚀" },
    { "account_id": "sa_linkedin_1", "caption": "Today we shipped..." },
    { "account_id": "sa_bluesky_1",  "caption": "shipped! 💫" }
  ],
  "idempotency_key": "launch-2026-04-08-001"
}\`)
req, _ := http.NewRequest("POST", "${BASE}/v1/social-posts", body)
req.Header.Set("Authorization", "Bearer up_live_xxxx")
req.Header.Set("Content-Type",  "application/json")`,
  curl: `curl -X POST ${BASE}/v1/social-posts \\
  -H "Authorization: Bearer up_live_xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform_posts": [
      { "account_id": "sa_twitter_1",  "caption": "shipped 🚀" },
      { "account_id": "sa_linkedin_1", "caption": "Today we shipped..." },
      { "account_id": "sa_bluesky_1",  "caption": "shipped! 💫" }
    ],
    "idempotency_key": "launch-2026-04-08-001"
  }'`,
};

const SN_CREATE_VIDEO_POST: Snippets = {
  js: `const response = await fetch(\n  '${BASE}/v1/social-posts',\n  {\n    method: 'POST',\n    headers: {\n      'Authorization': 'Bearer up_live_your_key',\n      'Content-Type':  'application/json',\n    },\n    body: JSON.stringify({\n      caption:     'Launch day 🎬',\n      account_ids: ['sa_youtube_123', 'sa_tiktok_456'],\n      media_urls:  ['https://example.com/video.mp4'],\n      platform_options: {\n        youtube: { privacy_status: 'public' },\n        tiktok:  { privacy_level:  'PUBLIC_TO_EVERYONE' },\n      },\n    }),\n  }\n);`,
  python: `import requests\n\nresponse = requests.post(\n    '${BASE}/v1/social-posts',\n    headers={\n        'Authorization': 'Bearer up_live_your_key',\n        'Content-Type':  'application/json',\n    },\n    json={\n        'caption':     'Launch day 🎬',\n        'account_ids': ['sa_youtube_123', 'sa_tiktok_456'],\n        'media_urls':  ['https://example.com/video.mp4'],\n        'platform_options': {\n            'youtube': {'privacy_status': 'public'},\n            'tiktok':  {'privacy_level':  'PUBLIC_TO_EVERYONE'},\n        },\n    },\n)`,
  go: `body := strings.NewReader(\`{\n  "caption":     "Launch day 🎬",\n  "account_ids": ["sa_youtube_123", "sa_tiktok_456"],\n  "media_urls":  ["https://example.com/video.mp4"],\n  "platform_options": {\n    "youtube": { "privacy_status": "public" },\n    "tiktok":  { "privacy_level":  "PUBLIC_TO_EVERYONE" }\n  }\n}\`)\n\nreq, _ := http.NewRequest("POST",\n    "${BASE}/v1/social-posts", body)\nreq.Header.Set("Authorization", "Bearer up_live_your_key")\nreq.Header.Set("Content-Type",  "application/json")\n\nresp, _ := http.DefaultClient.Do(req)`,
  curl: `curl -X POST ${BASE}/v1/social-posts \\\n  -H "Authorization: Bearer up_live_your_key" \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "caption": "Launch day 🎬",\n    "account_ids": ["sa_youtube_123", "sa_tiktok_456"],\n    "media_urls": ["https://example.com/video.mp4"],\n    "platform_options": {\n      "youtube": { "privacy_status": "public" },\n      "tiktok":  { "privacy_level": "PUBLIC_TO_EVERYONE" }\n    }\n  }'`,
};

function CodeTabs({ snippets, title }: { snippets: Snippets; title?: string }) {
  const [lang, setLang] = useState<LangId>("js");
  const [copied, setCopied] = useState(false);
  const code = snippets[lang];
  const lines = code.split("\n");
  const onCopy = async () => {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {}
  };
  return (
    <div className="doc-code-wrap">
      {title && <div className="doc-code-label">{title}</div>}
      <div className="doc-code-tabs-bar">
        {LANGS.map((l) => (
          <button
            key={l.id}
            type="button"
            className={`doc-code-tab ${lang === l.id ? "active" : ""}`}
            onClick={() => setLang(l.id)}
          >
            {l.label}
          </button>
        ))}
      </div>
      <div className="doc-code-box doc-code-box-tabbed">
        <button type="button" className="doc-code-copy" onClick={onCopy} aria-label="Copy code">
          {copied ? "Copied" : "Copy"}
        </button>
        <pre className="doc-code">
          <span className="doc-code-gutter" aria-hidden="true">
            {lines.map((_, i) => (
              <span key={i} className="doc-code-ln">{i + 1}</span>
            ))}
          </span>
          <code className="doc-code-content">{code}</code>
        </pre>
      </div>
    </div>
  );
}

function Code({ children, title }: { children: string; title?: string }) {
  const [copied, setCopied] = useState(false);
  const onCopy = async () => {
    try {
      await navigator.clipboard.writeText(children);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {}
  };
  const lines = children.split("\n");
  return (
    <div className="doc-code-wrap">
      {title && <div className="doc-code-label">{title}</div>}
      <div className="doc-code-box">
        <button type="button" className="doc-code-copy" onClick={onCopy} aria-label="Copy code">
          {copied ? "Copied" : "Copy"}
        </button>
        <pre className="doc-code">
          <span className="doc-code-gutter" aria-hidden="true">
            {lines.map((_, i) => (
              <span key={i} className="doc-code-ln">{i + 1}</span>
            ))}
          </span>
          <code className="doc-code-content">{children}</code>
        </pre>
      </div>
    </div>
  );
}

function Param({ name, type, required, children }: { name: string; type: string; required?: boolean; children: React.ReactNode }) {
  return (
    <div className="doc-param">
      <code className="doc-param-name">{name}</code>
      <span className="doc-param-type">{type}</span>
      {required && <span className="doc-param-req">required</span>}
      <span className="doc-param-desc">— {children}</span>
    </div>
  );
}

export default function DocsPage() {
  const [activeSection, setActiveSection] = useState("overview");

  // Intersection Observer to highlight the sidebar link for the section in view
  useEffect(() => {
    const sectionIds = NAV_ITEMS.map(([id]) => id);
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setActiveSection(entry.target.id);
          }
        }
      },
      { rootMargin: "-100px 0px -60% 0px", threshold: 0 }
    );
    for (const id of sectionIds) {
      const el = document.getElementById(id);
      if (el) observer.observe(el);
    }
    return () => observer.disconnect();
  }, []);

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />

      {/* Nav */}
      <nav className="doc-nav">
        <div className="doc-nav-inner">
          <Link href="/" className="doc-logo">
            <div className="doc-logo-mark"><ZapIcon /></div>
            <span className="doc-logo-name">UniPost</span>
          </Link>
          <div className="doc-nav-links">
            <Link href="/solutions" className="doc-nav-link">Solutions</Link>
            <Link href="/docs" className="doc-nav-link active">Docs</Link>
            <Link href="/pricing" className="doc-nav-link">Pricing</Link>
          </div>
          <MarketingNav />
        </div>
      </nav>

      <div className="doc-layout">
        {/* Sidebar */}
        <nav className="doc-sidebar">
          <ul className="doc-sidebar-list">
            {NAV_ITEMS.map(([id, label]) => (
              <li key={id} className="doc-sidebar-item">
                <a
                  href={`#${id}`}
                  className={`doc-sidebar-link ${activeSection === id ? "active" : ""}`}
                  onClick={() => setActiveSection(id)}
                >
                  {label}
                </a>
              </li>
            ))}
          </ul>
        </nav>

        {/* Content */}
        <div className="doc-main">
          <h1 className="doc-title">UniPost API Documentation</h1>
          <p className="doc-subtitle">One API to post across all major social platforms.</p>

          <Section id="overview" title="Overview">
            <p className="doc-p">
              UniPost is a unified social media API that lets developers integrate posting capabilities
              into their products without dealing with each platform individually. Connect social accounts
              once, then publish content to Bluesky, LinkedIn, Instagram, Threads, TikTok, and YouTube
              through a single API call.
            </p>
            <div className="doc-grid-2">
              <div className="doc-grid-card">
                <div className="doc-grid-card-title">Base URL</div>
                <div className="doc-grid-card-val">{BASE}</div>
              </div>
              <div className="doc-grid-card">
                <div className="doc-grid-card-title">Response Format</div>
                <div className="doc-grid-card-val">JSON</div>
              </div>
            </div>
            <div className="doc-callout doc-callout-info">
              <strong>All responses follow this structure:</strong>
              <Code>{`// Success\n{ "data": { ... }, "meta": { "total": 10, "page": 1, "per_page": 20 } }\n\n// Error\n{ "error": { "code": "UNAUTHORIZED", "message": "Invalid API key" } }`}</Code>
            </div>
          </Section>

          <Section id="authentication" title="Authentication">
            <p className="doc-p">
              All API requests require a Bearer token in the <code>Authorization</code> header.
              Create API keys in your project dashboard at <a href="https://app.unipost.dev">app.unipost.dev</a>.
            </p>
            <CodeTabs title="Example" snippets={SN_LIST_ACCOUNTS} />
            <p className="doc-p"><strong>Key format:</strong> <code>up_live_</code> (production) or <code>up_test_</code> (test)</p>
            <p className="doc-p"><strong>Security:</strong> Keys are shown only once at creation. Store them securely — never commit to version control.</p>
          </Section>

          <Section id="quick-start" title="Quick Start">
            <p className="doc-p">Get posting in 3 steps:</p>
            <p className="doc-p"><strong>1. Connect a social account</strong></p>
            <CodeTabs snippets={SN_CONNECT_BLUESKY} />
            <p className="doc-p"><strong>2. Get your account ID from the response</strong></p>
            <Code>{`{\n  "data": {\n    "id": "sa_abc123",\n    "platform": "bluesky",\n    "account_name": "yourname.bsky.social",\n    "status": "active"\n  }\n}`}</Code>
            <p className="doc-p"><strong>3. Create a post</strong></p>
            <CodeTabs snippets={SN_CREATE_POST} />
          </Section>

          <Section id="mcp" title="MCP / AI Agents">
            <p className="doc-p">
              UniPost provides a native <a href="https://modelcontextprotocol.io">Model Context Protocol (MCP)</a> server
              that lets AI agents — like Claude, GPT, or any MCP-compatible client — post to social media, check analytics,
              and manage your accounts through natural language.
            </p>

            <div className="doc-grid-2">
              <div className="doc-grid-card">
                <div className="doc-grid-card-title">MCP Endpoint</div>
                <div className="doc-grid-card-val">https://mcp.unipost.dev/mcp</div>
              </div>
              <div className="doc-grid-card">
                <div className="doc-grid-card-title">Transport</div>
                <div className="doc-grid-card-val">Streamable HTTP</div>
              </div>
            </div>

            <p className="doc-p"><strong>Available Tools</strong></p>
            <table className="doc-table">
              <thead><tr><th>Tool</th><th>Description</th></tr></thead>
              <tbody>
                {[
                  ["unipost_list_accounts", "List all connected social media accounts"],
                  ["unipost_create_post", "Create and publish a post to one or more accounts"],
                  ["unipost_get_post", "Get the status and details of a published post"],
                  ["unipost_get_analytics", "Get engagement metrics for a published post"],
                  ["unipost_list_posts", "List recent posts filtered by status"],
                ].map(([tool, desc]) => (
                  <tr key={tool}><td><code>{tool}</code></td><td>{desc}</td></tr>
                ))}
              </tbody>
            </table>

            <p className="doc-p"><strong>Claude Desktop</strong></p>
            <p className="doc-p">
              Add to your <code>claude_desktop_config.json</code>. Requires the <code>mcp-remote</code> bridge
              (auto-installed via npx). If <code>npx</code> is not found, replace with the full path
              (e.g. <code>/opt/homebrew/bin/npx</code>).
            </p>
            <Code>{`{
  "mcpServers": {
    "unipost": {
      "command": "npx",
      "args": [
        "-y",
        "mcp-remote",
        "https://mcp.unipost.dev/mcp",
        "--header",
        "Authorization:Bearer YOUR_API_KEY",
        "--transport",
        "http-only"
      ]
    }
  }
}`}</Code>

            <p className="doc-p"><strong>Claude Code</strong></p>
            <Code>{`claude mcp add unipost \\
  -t http \\
  --header "Authorization:Bearer YOUR_API_KEY" \\
  -- "https://mcp.unipost.dev/mcp"`}</Code>

            <p className="doc-p"><strong>Cursor / Windsurf</strong></p>
            <p className="doc-p">Add to your MCP settings JSON:</p>
            <Code>{`{
  "mcpServers": {
    "unipost": {
      "url": "https://mcp.unipost.dev/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_API_KEY"
      }
    }
  }
}`}</Code>

            <p className="doc-p"><strong>Test with cURL</strong></p>
            <Code>{`curl -X POST https://mcp.unipost.dev/mcp \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -H "Accept: application/json, text/event-stream" \\
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2025-03-26",
      "capabilities": {},
      "clientInfo": { "name": "test", "version": "0.1.0" }
    }
  }'`}</Code>

            <div className="doc-callout doc-callout-info">
              <strong>Legacy SSE endpoint:</strong> <code>https://mcp.unipost.dev/sse</code> is also available
              for clients that only support SSE transport. The Streamable HTTP endpoint (<code>/mcp</code>) is recommended.
            </div>
          </Section>

          <Section id="capabilities" title="Capabilities">
            <p className="doc-p">
              The capabilities map is the source of truth for what each
              network accepts on the publish side: caption length,
              image / video count caps, file size hints, threading,
              first-comment support. LLM-driven clients should fetch
              this once at session start so generated content respects
              every per-platform limit before hitting publish.
            </p>

            <Endpoint method="GET" path="/v1/platforms/capabilities" auth="None — public, cacheable">
              <p className="doc-p">
                Static map of all supported platforms. Versioned via the
                top-level <code>schema_version</code> field; bumps follow
                semver semantics (1.0 → 1.1 was an additive change in
                Sprint 2, adding <code>text.supports_threads</code>).
              </p>
              <p className="doc-p">
                Returned with <code>Cache-Control: public, max-age=3600</code>{" "}
                so CDNs and clients can cache it freely between deploys.
              </p>
              <Code title="Example">{`curl ${BASE}/v1/platforms/capabilities`}</Code>
              <Code title="Response (200)">{`{
  "data": {
    "schema_version": "1.1",
    "platforms": {
      "twitter": {
        "display_name": "Twitter / X",
        "text": { "max_length": 280, "min_length": 0, "required": false, "supports_threads": true },
        "media": {
          "requires_media": false,
          "allow_mixed": false,
          "images": { "max_count": 4, "max_file_size_bytes": 5242880, "allowed_formats": ["jpg","png","webp","gif"] },
          "videos": { "max_count": 1, "max_duration_seconds": 140, "max_file_size_bytes": 536870912, "allowed_formats": ["mp4","mov"] }
        },
        "thread":       { "supported": true },
        "scheduling":   { "supported": true },
        "first_comment":{ "supported": false }
      },
      "instagram": { /* ... */ },
      "tiktok":    { /* ... */ },
      "youtube":   { /* ... */ },
      "threads":   { /* ... */ },
      "linkedin":  { /* ... */ },
      "bluesky":   { /* ... */ }
    }
  }
}`}</Code>
            </Endpoint>

            <Endpoint method="GET" path="/v1/social-accounts/{id}/capabilities" auth="API Key">
              <p className="doc-p">
                Per-account variant. Same shape, but scoped to one
                connected account so a client doesn&apos;t need to
                map account_id → platform itself. Returns 404 for
                accounts not in the calling project.
              </p>
              <Code title="Example">{`curl -H "Authorization: Bearer up_live_xxx" \\
  ${BASE}/v1/social-accounts/8558370d-b957-450c-a399-e2c0838a441a/capabilities`}</Code>
            </Endpoint>
          </Section>

          <Section id="social-accounts" title="Social Accounts">
            <p className="doc-p">Connect, list, and disconnect social media accounts.</p>
            <Endpoint method="POST" path="/v1/social-accounts/connect" auth="API Key">
              <p className="doc-p">Connect a new social media account. For Bluesky, provide credentials directly. For OAuth platforms, use the <a href="#oauth">OAuth flow</a>.</p>
              <Param name="platform" type="string" required>Platform: <code>bluesky</code></Param>
              <Param name="credentials" type="object" required>Platform-specific credentials</Param>
              <CodeTabs title="Example: Connect Bluesky" snippets={SN_CONNECT_BLUESKY} />
              <Code title="Response (201)">{`{\n  "data": {\n    "id": "sa_abc123",\n    "platform": "bluesky",\n    "account_name": "alice.bsky.social",\n    "connected_at": "2026-04-02T10:00:00Z",\n    "status": "active"\n  }\n}`}</Code>
            </Endpoint>
            <Endpoint method="GET" path="/v1/social-accounts" auth="API Key">
              <p className="doc-p">List all connected social accounts.</p>
              <CodeTabs title="Example" snippets={SN_LIST_ACCOUNTS} />
            </Endpoint>
            <Endpoint method="DELETE" path="/v1/social-accounts/{id}" auth="API Key">
              <p className="doc-p">Disconnect a social account and invalidate its tokens.</p>
              <CodeTabs title="Example" snippets={SN_DELETE_ACCOUNT} />
            </Endpoint>
          </Section>

          <Section id="social-posts" title="Social Posts">
            <p className="doc-p">
              Create, list, get, and delete social media posts. Two
              request shapes: pass <strong>exactly one</strong> of{" "}
              <code>platform_posts[]</code> (per-account captions,
              recommended for AgentPost-style flows) or the legacy{" "}
              <code>caption + account_ids</code> (one caption fanned
              out to N accounts). Both still work — the legacy shape
              is expanded server-side into the same internal
              representation, so behavior parity is preserved.
            </p>
            <Endpoint method="POST" path="/v1/social-posts" auth="API Key">
              <p className="doc-p">
                Create and publish a post. Posts are published concurrently
                — one failure won&apos;t block others.
              </p>

              <p className="doc-p"><strong>Shape A — `platform_posts[]` (recommended)</strong></p>
              <Param name="platform_posts" type="object[]" required>
                Array of per-account posts with their own caption,
                media, and platform options. Same <code>account_id</code>
                can appear twice (e.g. a 2-tweet thread on the same
                Twitter account).
              </Param>
              <Param name="platform_posts[].account_id" type="string" required>Social account ID</Param>
              <Param name="platform_posts[].caption" type="string" required>The text content for THIS account</Param>
              <Param name="platform_posts[].media_urls" type="string[]">Publicly reachable URLs (image / video / gif sniffed by extension)</Param>
              <Param name="platform_posts[].media_ids" type="string[]">
                IDs returned by the media library (see <a href="#media">Media library</a>). Resolved
                to presigned download URLs server-side and merged with <code>media_urls</code>.
              </Param>
              <Param name="platform_posts[].platform_options" type="object">
                Platform-specific options (privacy_status for YouTube,
                privacy_level for TikTok, etc — see below).
              </Param>
              <Param name="platform_posts[].thread_position" type="number">
                1-indexed position in a multi-post thread. All entries
                with the same <code>account_id</code> and any non-zero
                <code>thread_position</code> form one thread. Twitter only
                in v0.3; Bluesky / Threads land in a future release. See{" "}
                <a href="#threads">Threads</a>.
              </Param>

              <p className="doc-p"><strong>Shape B — legacy</strong></p>
              <Param name="caption" type="string">The text content (used for every account_id)</Param>
              <Param name="account_ids" type="string[]">Array of social account IDs</Param>
              <Param name="media_urls" type="string[]">Shared media URLs (deprecated — prefer per-post media)</Param>
              <Param name="platform_options" type="object">
                Per-platform overrides keyed by platform name.
              </Param>

              <p className="doc-p"><strong>Common fields</strong></p>
              <Param name="scheduled_at" type="string">RFC3339 timestamp. If set, the post is queued and published by the scheduler at that time. Must be in the future.</Param>
              <Param name="idempotency_key" type="string">
                Optional retry-safe key. The same{" "}
                <code>(project_id, idempotency_key)</code> within 24h
                returns the original response unchanged with an{" "}
                <code>Idempotent-Replay: true</code> response header.
                <strong> No duplicate platform posts are created.</strong>
              </Param>
              <Param name="status" type="string">
                Set to <code>&quot;draft&quot;</code> to persist without
                publishing. See <a href="#drafts">Drafts &amp; Preview</a>.
              </Param>
              <p className="doc-p"><strong>Media handling</strong></p>
              <p className="doc-p">
                <code>media_urls</code> is a flat list of publicly reachable URLs.
                The API sniffs each item&apos;s kind from its file extension
                (<code>.jpg</code>, <code>.png</code>, <code>.webp</code>, <code>.heic</code> →
                image; <code>.mp4</code>, <code>.mov</code>, <code>.webm</code> → video; <code>.gif</code> → gif).
                Each adapter then decides how to use the items — single image, carousel, video, etc. See{" "}
                <a href="#platforms">Supported Platforms</a> below for what each network accepts and how many items.
              </p>

              <p className="doc-p"><strong>Platform options</strong></p>
              <p className="doc-p">Pass overrides under <code>platform_options.&lt;platform&gt;</code>. Unknown keys are ignored.</p>

              <p className="doc-p"><em>YouTube</em></p>
              <Param name="platform_options.youtube.privacy_status" type="string">
                <code>private</code> (default), <code>public</code>, or <code>unlisted</code>.
              </Param>
              <Param name="platform_options.youtube.shorts" type="boolean">
                When <code>true</code>, appends <code>#Shorts</code> to the title and description so YouTube
                surfaces the upload in the Shorts shelf. Combine with a 9:16 source video under 60 seconds.
              </Param>
              <Param name="platform_options.youtube.category_id" type="string">
                YouTube category id (e.g. <code>22</code> for People &amp; Blogs, <code>10</code> for Music).
              </Param>
              <Param name="platform_options.youtube.tags" type="string[]">
                Tag list set on <code>snippet.tags</code>.
              </Param>

              <p className="doc-p"><em>TikTok</em></p>
              <Param name="platform_options.tiktok.privacy_level" type="string">
                <code>SELF_ONLY</code> (default), <code>PUBLIC_TO_EVERYONE</code>, <code>MUTUAL_FOLLOW_FRIENDS</code>, or <code>FOLLOWER_OF_CREATOR</code>. Note: TikTok forces <code>SELF_ONLY</code> for apps that are still in sandbox/unaudited mode regardless of what you send.
              </Param>
              <Param name="platform_options.tiktok.upload_mode" type="string">
                <code>pull_from_url</code> (default — TikTok pulls the video from your CDN, no proxy)
                or <code>file_upload</code> (we download then push the bytes to TikTok). Use{" "}
                <code>file_upload</code> only when the source URL isn&apos;t on a domain registered with your
                TikTok developer portal.
              </Param>
              <Param name="platform_options.tiktok.photo_cover_index" type="number">
                Zero-based index into <code>media_urls</code> selecting which image is shown as the carousel cover.
                Defaults to <code>0</code>.
              </Param>
              <p className="doc-p"><strong>Response Headers</strong></p>
              <Param name="X-UniPost-Usage" type="header">Current usage, e.g. <code>450/1000</code></Param>
              <Param name="X-UniPost-Warning" type="header">Warning: <code>approaching_limit</code> or <code>over_limit</code></Param>
              <CodeTabs title="Example: per-platform captions (recommended)" snippets={SN_PLATFORM_POSTS} />
              <CodeTabs title="Example: legacy fan-out shape" snippets={SN_CREATE_POST} />
              <CodeTabs title="Example: video to YouTube as public + TikTok" snippets={SN_CREATE_VIDEO_POST} />
              <Code title="Response (200)">{`{\n  "data": {\n    "id": "post_xyz789",\n    "caption": "Hello from UniPost! 🚀",\n    "status": "published",\n    "results": [\n      { "platform": "bluesky", "status": "published", "external_id": "at://..." },\n      { "platform": "linkedin", "status": "published", "external_id": "urn:li:share:..." }\n    ]\n  }\n}`}</Code>
              <div className="doc-callout doc-callout-warn">
                <strong>Post status values:</strong> <code>published</code> (all succeeded), <code>partial</code> (some failed), <code>failed</code> (all failed)
              </div>
            </Endpoint>
            <Endpoint method="GET" path="/v1/social-posts/{id}" auth="API Key">
              <p className="doc-p">Get a post with per-account results.</p>
            </Endpoint>
            <Endpoint method="GET" path="/v1/social-posts" auth="API Key">
              <p className="doc-p">List all posts, ordered by creation date (newest first).</p>
            </Endpoint>
            <Endpoint method="DELETE" path="/v1/social-posts/{id}" auth="API Key">
              <p className="doc-p">Delete a post. Attempts to delete from all platforms.</p>
            </Endpoint>
            <Endpoint method="GET" path="/v1/social-posts" auth="API Key">
              <p className="doc-p">
                List posts with optional filters and cursor-based pagination
                over <code>(created_at DESC, id DESC)</code>.
              </p>
              <Param name="status" type="query"><code>draft</code>, <code>scheduled</code>, <code>published</code>, <code>partial</code>, <code>failed</code></Param>
              <Param name="platform" type="query">Filter by destination platform key</Param>
              <Param name="account_id" type="query">Filter by social account id</Param>
              <Param name="created_after" type="query">RFC3339 lower bound</Param>
              <Param name="created_before" type="query">RFC3339 upper bound</Param>
              <Param name="limit" type="query">1–100, default 25</Param>
              <Param name="cursor" type="query">Opaque cursor returned in <code>meta.next_cursor</code></Param>
              <Code title="Response (200)">{`{
  "data": [ /* SocialPost[] */ ],
  "meta": { "next_cursor": "eyJjcmVhdGVkX2F0Ijoi..." }
}`}</Code>
            </Endpoint>
          </Section>

          <Section id="validate" title="Validate (preflight)">
            <p className="doc-p">
              Pure preflight: same request body as <code>POST /v1/social-posts</code>,
              but no DB writes and no platform API calls. Use it to confirm an LLM-drafted
              post will pass server-side validation before charging the user&apos;s quota.
              Always returns 200 — the body carries fatal / non-fatal errors per platform.
              p95 budget &lt; 50ms.
            </p>
            <Endpoint method="POST" path="/v1/social-posts/validate" auth="API Key">
              <p className="doc-p">
                Accepts both Shape A (<code>platform_posts[]</code>) and Shape B
                (<code>caption + account_ids</code>). Validates account existence,
                connection state, capability limits (caption length, media counts,
                file sizes via <code>media_ids</code> from the Media library),
                threading rules, and <code>scheduled_at</code>.
              </p>
              <Code title="Response (200)">{`{
  "data": {
    "ok": false,
    "errors": [
      {
        "account_id": "sa_twitter_1",
        "platform":   "twitter",
        "code":       "caption_too_long",
        "message":    "caption is 312 chars, twitter max is 280",
        "fatal":      true
      },
      {
        "account_id": "sa_instagram_1",
        "platform":   "instagram",
        "code":       "account_disconnected",
        "message":    "account requires reconnect",
        "fatal":      false
      }
    ]
  }
}`}</Code>
              <p className="doc-p">
                <strong>Fatal vs non-fatal.</strong> Fatal codes (<code>caption_too_long</code>,
                <code> too_many_media</code>, <code>media_id_not_in_project</code>,
                <code> unknown_account</code>, <code>scheduled_in_past</code>, …) block
                publish. Non-fatal codes (e.g. <code>account_disconnected</code>) are surfaced
                so the client can warn the user but don&apos;t prevent the request from being
                accepted by <code>POST /v1/social-posts</code>.
              </p>
            </Endpoint>
          </Section>

          <Section id="drafts" title="Drafts & Preview">
            <p className="doc-p">
              Drafts are real <code>social_posts</code> rows with{" "}
              <code>status=&quot;draft&quot;</code> — no separate table, no separate
              API surface. Create one by passing <code>status: &quot;draft&quot;</code>
              on <code>POST /v1/social-posts</code>; it&apos;s persisted but not dispatched
              to any platform.
            </p>
            <p className="doc-p">
              Each draft gets a tamper-proof preview URL signed with HMAC-SHA256
              (the same <code>ENCRYPTION_KEY</code> used by the rest of the API).
              Tokens are scoped to one post id and expire after 7 days.
            </p>
            <Endpoint method="POST" path="/v1/social-posts/{id}/publish" auth="API Key">
              <p className="doc-p">
                Promote a draft to live. Uses an optimistic lock
                (<code>UPDATE … WHERE status=&apos;draft&apos; RETURNING …</code>) so a
                concurrent second call loses with <code>409 ALREADY_PUBLISHED</code>{" "}
                instead of double-posting. The body is empty; the post is dispatched
                using the captions / media that were saved on the draft.
              </p>
              <Code title="Response (200)">{`{
  "data": {
    "id":     "post_xyz789",
    "status": "published",
    "results": [ /* one entry per account */ ]
  }
}`}</Code>
            </Endpoint>
            <Endpoint method="GET" path="/v1/social-posts/{id}/preview-url" auth="API Key">
              <p className="doc-p">
                Returns a signed URL pointing at <code>app.unipost.dev/preview/{"{id}"}</code>.
                Tokens carry the post id in their <code>aud</code> claim and are rejected
                for any other id.
              </p>
              <Code title="Response (200)">{`{
  "data": {
    "url":        "https://app.unipost.dev/preview/post_xyz789?token=eyJhbGciOi...",
    "expires_at": "2026-04-15T10:00:00Z"
  }
}`}</Code>
            </Endpoint>
            <Endpoint method="GET" path="/v1/social-posts/{id}/preview" auth="Signed token">
              <p className="doc-p">
                Public, no API key. Validates the JWT signature + audience and returns
                a render-friendly view of the post (per-account caption, resolved media URLs,
                platform metadata). Powers the dashboard preview page.
              </p>
            </Endpoint>
          </Section>

          <Section id="threads" title="Threads">
            <p className="doc-p">
              Multi-post threads use <code>thread_position</code> on the{" "}
              <code>platform_posts[]</code> shape: every entry sharing the same{" "}
              <code>account_id</code> with a non-zero <code>thread_position</code> forms
              one thread, dispatched in ascending order. Threads on different accounts
              run in parallel; entries within one thread are sequential because each
              reply must reference the prior tweet&apos;s id.
            </p>
            <div className="doc-callout doc-callout-info">
              <strong>Platform support (v0.3):</strong> Twitter / X only.
              Bluesky and Threads thread support is on the roadmap. Sending a
              <code> thread_position</code> for an unsupported platform is
              rejected by <code>/validate</code> with{" "}
              <code>thread_unsupported</code>.
            </div>
            <Code title="3-tweet thread on one account">{`{
  "platform_posts": [
    { "account_id": "sa_twitter_1", "caption": "1/ I want to talk about why...",  "thread_position": 1 },
    { "account_id": "sa_twitter_1", "caption": "2/ The first reason is...",       "thread_position": 2 },
    { "account_id": "sa_twitter_1", "caption": "3/ And finally — try it here →",  "thread_position": 3 }
  ]
}`}</Code>
            <p className="doc-p">
              The handler orchestrates the dispatch: tweet 1 publishes, the returned
              tweet id is plumbed into <code>opts[&quot;in_reply_to_tweet_id&quot;]</code>
              for tweet 2, and so on. If any tweet in the chain fails the rest are
              skipped and the post status is <code>partial</code>; already-posted
              replies are not rolled back (Twitter doesn&apos;t support that).
            </p>
          </Section>

          <Section id="media" title="Media library">
            <p className="doc-p">
              Two-step presigned upload backed by Cloudflare R2. Use this when you
              don&apos;t have a public CDN handy, or when you want UniPost to enforce
              size / content-type limits before the bytes ever touch a platform API.
              Uploaded media is referenced from posts via <code>media_ids</code>.
            </p>
            <Endpoint method="POST" path="/v1/media" auth="API Key">
              <p className="doc-p">
                Reserve a media row and get a presigned PUT URL. The row starts in{" "}
                <code>pending</code> status until the bytes land in R2.
              </p>
              <Param name="filename" type="string" required>Original filename, used for the storage key + extension sniffing</Param>
              <Param name="content_type" type="string" required>MIME type — must be on the allowlist (image/jpeg, image/png, image/webp, image/gif, video/mp4, video/quicktime)</Param>
              <Param name="size_bytes" type="number" required>Declared size — must respect per-type caps</Param>
              <Code title="Response (201)">{`{
  "data": {
    "id":             "med_abc123",
    "upload_url":     "https://r2.example.com/...&X-Amz-Signature=...",
    "upload_method":  "PUT",
    "upload_headers": { "Content-Type": "image/jpeg" },
    "expires_at":     "2026-04-08T11:00:00Z"
  }
}`}</Code>
            </Endpoint>
            <Endpoint method="PUT" path="(presigned R2 URL)" auth="Signed">
              <p className="doc-p">
                Upload the bytes directly to R2 using the URL + headers from the previous
                step. UniPost is not in the data path; the presigned URL expires after 15 minutes.
              </p>
              <Code title="cURL">{`curl -X PUT "$UPLOAD_URL" \\
  -H "Content-Type: image/jpeg" \\
  --data-binary @photo.jpg`}</Code>
            </Endpoint>
            <Endpoint method="GET" path="/v1/media/{id}" auth="API Key">
              <p className="doc-p">
                Returns the media row. <strong>Lazy hydration:</strong> the first call
                after upload HEADs the R2 object, copies its real size and content-type,
                and flips status from <code>pending</code> → <code>uploaded</code>.
                No R2 webhooks required.
              </p>
              <Code title="Response (200)">{`{
  "data": {
    "id":           "med_abc123",
    "status":       "uploaded",
    "content_type": "image/jpeg",
    "size_bytes":   284192,
    "filename":     "photo.jpg",
    "created_at":   "2026-04-08T10:42:00Z"
  }
}`}</Code>
            </Endpoint>
            <Endpoint method="GET" path="/v1/media" auth="API Key">
              <p className="doc-p">List media rows for the project. Cursor pagination, same shape as <code>/v1/social-posts</code>.</p>
            </Endpoint>
            <Endpoint method="DELETE" path="/v1/media/{id}" auth="API Key">
              <p className="doc-p">Delete the row and the underlying R2 object. Posts that reference the media keep their existing platform-side copies.</p>
            </Endpoint>
            <p className="doc-p">
              <strong>Using media in posts.</strong> Pass{" "}
              <code>media_ids: [&quot;med_abc123&quot;]</code> on a{" "}
              <code>platform_posts[]</code> entry. The publish handler resolves each
              id to a short-lived presigned download URL and merges them with any
              <code> media_urls</code> on the same entry before handing off to the adapter.
            </p>
          </Section>

          <Section id="account-health" title="Account Health">
            <p className="doc-p">
              Returns liveness state for one connected account: when its token was
              last refreshed, what scopes it currently holds, and whether the most
              recent publish attempt succeeded. Use this to decide if a reconnect
              banner should be shown in your UI.
            </p>
            <Endpoint method="GET" path="/v1/social-accounts/{id}/health" auth="API Key">
              <Code title="Response (200)">{`{
  "data": {
    "id":                  "sa_twitter_1",
    "platform":            "twitter",
    "status":              "active",
    "token_refreshed_at":  "2026-04-07T22:11:04Z",
    "scopes":              ["tweet.read", "tweet.write", "users.read", "media.write", "offline.access"],
    "last_publish_at":     "2026-04-08T08:30:11Z",
    "last_publish_status": "published",
    "last_publish_error":  null
  }
}`}</Code>
            </Endpoint>
            <p className="doc-p">
              <code>status</code> mirrors the value from <code>GET /v1/social-accounts</code>{" "}
              (<code>active</code> or <code>reconnect_required</code>). The latter is set
              automatically when a publish fails with a token error so the dashboard can
              prompt the user before they next try to post.
            </p>
          </Section>

          <Section id="analytics" title="Analytics">
            <p className="doc-p">
              Read aggregated and per-post engagement metrics across all platforms with a unified field set.
              All endpoints accept <code>start_date</code> and <code>end_date</code> in <code>YYYY-MM-DD</code> form;
              defaults are the last 30 days. Date range filtering is by post <code>created_at</code> in UTC.
            </p>
            <p className="doc-p">
              <strong>Unified metric fields:</strong> <code>impressions</code>, <code>reach</code>, <code>likes</code>,{" "}
              <code>comments</code>, <code>shares</code>, <code>saves</code>, <code>clicks</code>,{" "}
              <code>video_views</code>. Fields a given platform doesn&apos;t expose are returned as <code>0</code>.
              <code>engagement_rate</code> is computed as{" "}
              <code>(likes + comments + shares + saves + clicks) / impressions</code>, rounded to 4 decimals.
            </p>

            <Endpoint method="GET" path="/v1/analytics/summary" auth="API Key">
              <p className="doc-p">Aggregated post counts, engagement totals, and period-over-period change for the requested window.</p>
              <Param name="start_date" type="query">YYYY-MM-DD. Defaults to 30 days ago (UTC).</Param>
              <Param name="end_date" type="query">YYYY-MM-DD, inclusive. Defaults to today (UTC).</Param>
              <p className="doc-p">
                <strong>vs_previous_period</strong> compares the requested window against the same-length window
                immediately preceding it (e.g. last 30 days vs days 60–30 ago). Returns 0 when the previous window has no data.
              </p>
              <Code title="Example">{`curl ${BASE}/v1/analytics/summary?start_date=2026-03-07&end_date=2026-04-06 \\\n  -H "Authorization: Bearer up_live_your_key"`}</Code>
              <Code title="Response (200)">{`{
  "data": {
    "period": { "start": "2026-03-07", "end": "2026-04-06" },
    "posts": {
      "total": 62,
      "published": 31,
      "scheduled": 0,
      "failed": 31,
      "failed_rate": 0.5
    },
    "engagement": {
      "impressions": 48234,
      "reach": 0,
      "likes": 2891,
      "comments": 456,
      "shares": 789,
      "saves": 0,
      "clicks": 234,
      "video_views": 0,
      "engagement_rate": 0.0884
    },
    "vs_previous_period": {
      "impressions_change": 0.08,
      "likes_change": 0.15,
      "engagement_change": -0.02
    }
  }
}`}</Code>
            </Endpoint>

            <Endpoint method="GET" path="/v1/analytics/trend" auth="API Key">
              <p className="doc-p">Daily time series for the requested window. Days with no posts are zero-filled.</p>
              <Param name="start_date" type="query">YYYY-MM-DD. Defaults to 30 days ago.</Param>
              <Param name="end_date" type="query">YYYY-MM-DD, inclusive. Defaults to today.</Param>
              <Param name="metric" type="query">
                CSV of any of: <code>posts</code>, <code>impressions</code>, <code>likes</code>, <code>comments</code>, <code>shares</code>.
                Defaults to <code>posts,impressions,likes</code>.
              </Param>
              <Code title="Example">{`curl ${BASE}/v1/analytics/trend?start_date=2026-04-01&end_date=2026-04-06&metric=posts,impressions \\\n  -H "Authorization: Bearer up_live_your_key"`}</Code>
              <Code title="Response (200)">{`{
  "data": {
    "dates": ["2026-04-01", "2026-04-02", "2026-04-03", "2026-04-04", "2026-04-05", "2026-04-06"],
    "series": {
      "posts":       [2, 1, 3, 0, 2, 1],
      "impressions": [800, 400, 1200, 0, 900, 350]
    }
  }
}`}</Code>
            </Endpoint>

            <Endpoint method="GET" path="/v1/analytics/by-platform" auth="API Key">
              <p className="doc-p">Per-platform aggregates plus an account count for the requested window.</p>
              <Param name="start_date" type="query">YYYY-MM-DD.</Param>
              <Param name="end_date" type="query">YYYY-MM-DD, inclusive.</Param>
              <Code title="Response (200)">{`{
  "data": [
    {
      "platform": "instagram",
      "posts": 2,
      "accounts": 1,
      "impressions": 3200,
      "reach": 2100,
      "likes": 189,
      "comments": 34,
      "shares": 12,
      "saves": 18,
      "clicks": 0,
      "video_views": 0,
      "engagement_rate": 0.0791
    }
  ]
}`}</Code>
            </Endpoint>

            <Endpoint method="GET" path="/v1/social-posts/{id}/analytics" auth="API Key">
              <p className="doc-p">
                Per-post metrics broken down by social account (one entry per account the post was published to).
                Cached for 1 hour; the <code>AnalyticsRefreshWorker</code> keeps cached rows fresh in the background.
              </p>
              <Code title="Example">{`curl ${BASE}/v1/social-posts/post_xyz789/analytics \\\n  -H "Authorization: Bearer up_live_your_key"`}</Code>
            </Endpoint>
          </Section>

          <Section id="webhooks" title="Webhooks">
            <p className="doc-p">Register webhook endpoints for real-time notifications.</p>
            <Endpoint method="POST" path="/v1/webhooks" auth="API Key">
              <Param name="url" type="string" required>HTTPS endpoint URL</Param>
              <Param name="events" type="string[]" required>Events: <code>post.published</code>, <code>post.failed</code>, <code>account.connected</code>, <code>account.disconnected</code></Param>
              <Param name="secret" type="string" required>HMAC-SHA256 signing secret</Param>
              <Code title="Webhook payload">{`{\n  "event": "post.published",\n  "timestamp": "2026-04-02T12:00:01Z",\n  "data": {\n    "post_id": "post_xyz789",\n    "platform": "bluesky",\n    "external_id": "at://..."\n  }\n}`}</Code>
              <p className="doc-p">
                <strong>Signature header:</strong> <code>X-UniPost-Signature: t=&lt;unix_ts&gt;,v1=&lt;hex&gt;</code>.
                Compute <code>v1 = HMAC-SHA256(secret, t + &quot;.&quot; + body)</code> and reject any
                request whose <code>t</code> is more than 5 minutes old to defeat replay attacks.
                Use a constant-time comparison.
              </p>
              <div className="doc-callout doc-callout-warn">
                <strong>Breaking change (Sprint 1, PR8):</strong> the signature header was previously{" "}
                <code>HMAC-SHA256(secret, body)</code> with no timestamp. Existing receivers must update
                to the timestamped <code>t=…,v1=…</code> scheme — the body-only form is no longer sent.
              </div>
            </Endpoint>
          </Section>

          <Section id="oauth" title="OAuth Flow">
            <p className="doc-p">For OAuth platforms (LinkedIn, Instagram, Threads, TikTok, YouTube), get an authorization URL and redirect the user.</p>
            <Endpoint method="GET" path="/v1/oauth/connect/{platform}" auth="API Key">
              <Param name="platform" type="path" required><code>linkedin</code>, <code>instagram</code>, <code>threads</code>, <code>tiktok</code>, <code>youtube</code></Param>
              <Param name="redirect_url" type="query">Post-auth redirect URL</Param>
              <Code title="Response">{`{ "data": { "auth_url": "https://www.linkedin.com/oauth/v2/authorization?..." } }`}</Code>
              <p className="doc-p">After authorization, user is redirected with <code>?status=success&amp;account_name=...</code> or <code>?status=error</code>.</p>
            </Endpoint>
          </Section>

          <Section id="billing" title="Billing & Usage">
            <p className="doc-p">UniPost uses soft-block quotas. Exceeding your limit won&apos;t interrupt service.</p>
            <div className="doc-grid-2" style={{ gridTemplateColumns: "repeat(4, 1fr)" }}>
              {[["Free", "$0/mo", "100"], ["Starter", "$10/mo", "1,000"], ["Growth", "$50/mo", "5,000"], ["Scale", "$150/mo", "20,000"]].map(([name, price, posts]) => (
                <div key={name} className="doc-grid-card" style={{ textAlign: "center" }}>
                  <div className="doc-grid-card-title">{name}</div>
                  <div className="doc-grid-card-val">{price}</div>
                  <div style={{ fontSize: 11, color: "var(--muted2)", marginTop: 2 }}>{posts} posts</div>
                </div>
              ))}
            </div>
            <p className="doc-p" style={{ fontSize: 12 }}>Additional tiers: $25, $75, $300, $500, $1000/mo. <Link href="/pricing">Full pricing →</Link></p>
            <Code title="Usage headers">{`X-UniPost-Usage: 450/1000\nX-UniPost-Warning: approaching_limit  # at 80%+\nX-UniPost-Warning: over_limit         # at 100%+`}</Code>
          </Section>

          <Section id="errors" title="Error Handling">
            <table className="doc-table">
              <thead><tr><th>Code</th><th>Status</th><th>Description</th></tr></thead>
              <tbody>
                {[["UNAUTHORIZED", "401", "Invalid or missing API key"], ["FORBIDDEN", "403", "No access to resource"], ["NOT_FOUND", "404", "Resource not found"], ["VALIDATION_ERROR", "422", "Invalid parameters"], ["INTERNAL_ERROR", "500", "Server error"]].map(([code, status, desc]) => (
                  <tr key={code}><td><code>{code}</code></td><td>{status}</td><td>{desc}</td></tr>
                ))}
              </tbody>
            </table>
          </Section>

          <Section id="platforms" title="Supported Platforms">
            <p className="doc-p">
              Each platform&apos;s adapter decides how to use the items in <code>media_urls</code>.
              The matrix below summarises what every network accepts in a single
              <code> POST /v1/social-posts </code> call. Symbols: <strong>✅</strong> supported,{" "}
              <strong>❌</strong> not supported, <strong>—</strong> not applicable.
            </p>

            <table className="doc-table">
              <thead>
                <tr>
                  <th>Platform</th>
                  <th>Auth</th>
                  <th>Text&nbsp;only</th>
                  <th>Image</th>
                  <th>Multi&#8209;image</th>
                  <th>Video</th>
                  <th>Mix img+vid</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td><strong>Bluesky</strong></td>
                  <td>App password</td>
                  <td>✅</td>
                  <td>✅</td>
                  <td>✅ (≤4)</td>
                  <td>✅ (1)</td>
                  <td>❌</td>
                </tr>
                <tr>
                  <td><strong>Twitter / X</strong></td>
                  <td>OAuth 2.0 (PKCE)</td>
                  <td>✅</td>
                  <td>✅</td>
                  <td>✅ (≤4)</td>
                  <td>✅ (1)</td>
                  <td>❌</td>
                </tr>
                <tr>
                  <td><strong>LinkedIn</strong></td>
                  <td>OAuth</td>
                  <td>✅</td>
                  <td>✅ native</td>
                  <td>✅ (≤9)</td>
                  <td>✅ (1)</td>
                  <td>❌</td>
                </tr>
                <tr>
                  <td><strong>Instagram</strong></td>
                  <td>OAuth</td>
                  <td>❌</td>
                  <td>✅</td>
                  <td>✅ Carousel (≤10)</td>
                  <td>✅ Reels (1)</td>
                  <td>✅ in carousel</td>
                </tr>
                <tr>
                  <td><strong>Threads</strong></td>
                  <td>OAuth</td>
                  <td>✅</td>
                  <td>✅</td>
                  <td>✅ Carousel (≤20)</td>
                  <td>✅ (1)</td>
                  <td>✅ in carousel</td>
                </tr>
                <tr>
                  <td><strong>TikTok</strong></td>
                  <td>OAuth</td>
                  <td>❌</td>
                  <td>✅ Photo (≤35)</td>
                  <td>✅ Photo (≤35)</td>
                  <td>✅ (1)</td>
                  <td>❌</td>
                </tr>
                <tr>
                  <td><strong>YouTube</strong></td>
                  <td>OAuth</td>
                  <td>❌</td>
                  <td>❌</td>
                  <td>❌</td>
                  <td>✅ (1)</td>
                  <td>—</td>
                </tr>
              </tbody>
            </table>

            <div className="doc-callout doc-callout-info">
              <strong>Mixing rules.</strong> Most networks reject mixed image+video in a single post.
              Instagram and Threads allow mixing only inside their carousel containers (<code>media_type=CAROUSEL</code>).
              All limits above are enforced server-side — sending more items than the cap returns a <code>VALIDATION_ERROR</code>.
            </div>

            <p className="doc-p">
              Below: per-platform request examples and the response shape you should expect. All examples assume
              <code> POST /v1/social-posts </code> with <code>Authorization: Bearer up_live_xxx</code> and
              <code> Content-Type: application/json</code>; only the JSON body is shown for brevity.
            </p>

            <h3 className="doc-section-title" style={{ fontSize: 18, marginTop: 32, marginBottom: 12 }}>Bluesky</h3>
            <p className="doc-p">App-password auth. Up to 4 images per post or exactly 1 video. Text-only posts are allowed.</p>
            <Code title="Image post (1–4 images)">{`{
  "caption": "Photos from the trip ✈️",
  "account_ids": ["sa_bluesky_1"],
  "media_urls": [
    "https://cdn.example.com/photo1.jpg",
    "https://cdn.example.com/photo2.jpg"
  ]
}`}</Code>
            <Code title="Video post">{`{
  "caption": "Behind the scenes",
  "account_ids": ["sa_bluesky_1"],
  "media_urls": ["https://cdn.example.com/clip.mp4"]
}`}</Code>
            <Code title="Response">{`{
  "data": {
    "id": "post_xyz789",
    "status": "published",
    "results": [
      {
        "platform": "bluesky",
        "status": "published",
        "external_id": "at://did:plc:abc/app.bsky.feed.post/3kxyz123"
      }
    ]
  }
}`}</Code>

            <h3 className="doc-section-title" style={{ fontSize: 18, marginTop: 32, marginBottom: 12 }}>Twitter / X</h3>
            <p className="doc-p">
              OAuth 2.0. Up to 4 images per tweet, or 1 video, or 1 GIF. Requires the <code>media.write</code> scope.
              Video uploads use the chunked <code>/2/media/upload</code> path; large files may take several seconds to process.
            </p>
            <Code title="Image post (1–4 images)">{`{
  "caption": "Launching today 🚀",
  "account_ids": ["sa_twitter_1"],
  "media_urls": [
    "https://cdn.example.com/hero.jpg",
    "https://cdn.example.com/screenshot.png"
  ]
}`}</Code>
            <Code title="Video post">{`{
  "caption": "Watch the demo 👇",
  "account_ids": ["sa_twitter_1"],
  "media_urls": ["https://cdn.example.com/demo.mp4"]
}`}</Code>
            <Code title="Response">{`{
  "data": {
    "id": "post_xyz789",
    "status": "published",
    "results": [
      {
        "platform": "twitter",
        "status": "published",
        "external_id": "1789012345678901234"
      }
    ]
  }
}`}</Code>

            <h3 className="doc-section-title" style={{ fontSize: 18, marginTop: 32, marginBottom: 12 }}>LinkedIn</h3>
            <p className="doc-p">
              OAuth. Native uploads via the Assets API: up to 9 images per post, OR exactly 1 video. Mixing image
              and video in a single share is not allowed. Plain text posts are supported by omitting <code>media_urls</code>.
            </p>
            <Code title="Multi-image post (≤9)">{`{
  "caption": "Recap of our launch event",
  "account_ids": ["sa_linkedin_1"],
  "media_urls": [
    "https://cdn.example.com/event-1.jpg",
    "https://cdn.example.com/event-2.jpg",
    "https://cdn.example.com/event-3.jpg"
  ]
}`}</Code>
            <Code title="Video post">{`{
  "caption": "Customer story 🎬",
  "account_ids": ["sa_linkedin_1"],
  "media_urls": ["https://cdn.example.com/story.mp4"]
}`}</Code>
            <Code title="Response">{`{
  "data": {
    "id": "post_xyz789",
    "status": "published",
    "results": [
      {
        "platform": "linkedin",
        "status": "published",
        "external_id": "urn:li:share:7180000000000000000"
      }
    ]
  }
}`}</Code>

            <h3 className="doc-section-title" style={{ fontSize: 18, marginTop: 32, marginBottom: 12 }}>Instagram</h3>
            <p className="doc-p">
              OAuth. Business or Creator account required. Single image, single video as <strong>Reels</strong>,
              or 2–10 items as a <strong>carousel</strong> (images, videos, or a mix). Text-only posts are not allowed.
            </p>
            <Code title="Single image">{`{
  "caption": "Sunset 🌅",
  "account_ids": ["sa_instagram_1"],
  "media_urls": ["https://cdn.example.com/sunset.jpg"]
}`}</Code>
            <Code title="Reels (single video)">{`{
  "caption": "30-second intro 🎬",
  "account_ids": ["sa_instagram_1"],
  "media_urls": ["https://cdn.example.com/intro.mp4"]
}`}</Code>
            <Code title="Carousel (2–10 mixed items)">{`{
  "caption": "Product walkthrough",
  "account_ids": ["sa_instagram_1"],
  "media_urls": [
    "https://cdn.example.com/cover.jpg",
    "https://cdn.example.com/detail.jpg",
    "https://cdn.example.com/clip.mp4"
  ]
}`}</Code>
            <Code title="Response">{`{
  "data": {
    "id": "post_xyz789",
    "status": "published",
    "results": [
      {
        "platform": "instagram",
        "status": "published",
        "external_id": "17900000000000000"
      }
    ]
  }
}`}</Code>

            <h3 className="doc-section-title" style={{ fontSize: 18, marginTop: 32, marginBottom: 12 }}>Threads</h3>
            <p className="doc-p">
              OAuth (Meta developer app). Text-only, single image, single video, or 2–20 items in a carousel
              (mixed image+video allowed inside the carousel container). Video posts wait ~30 seconds before
              publishing per Meta&apos;s recommendation.
            </p>
            <Code title="Text-only">{`{
  "caption": "Just shipped a new release ✨",
  "account_ids": ["sa_threads_1"]
}`}</Code>
            <Code title="Single video">{`{
  "caption": "Behind the scenes",
  "account_ids": ["sa_threads_1"],
  "media_urls": ["https://cdn.example.com/bts.mp4"]
}`}</Code>
            <Code title="Carousel (2–20 items)">{`{
  "caption": "Conference highlights",
  "account_ids": ["sa_threads_1"],
  "media_urls": [
    "https://cdn.example.com/talk-1.jpg",
    "https://cdn.example.com/talk-2.jpg",
    "https://cdn.example.com/keynote.mp4"
  ]
}`}</Code>
            <Code title="Response">{`{
  "data": {
    "id": "post_xyz789",
    "status": "published",
    "results": [
      {
        "platform": "threads",
        "status": "published",
        "external_id": "17900000000000000"
      }
    ]
  }
}`}</Code>

            <h3 className="doc-section-title" style={{ fontSize: 18, marginTop: 32, marginBottom: 12 }}>TikTok</h3>
            <p className="doc-p">
              OAuth. Either a single video (default flow uses <code>PULL_FROM_URL</code> so TikTok pulls from your CDN)
              or a photo carousel of up to 35 images. Text-only posts and image+video mixing are not supported.
              Source URLs must be on a domain registered in your TikTok developer portal — otherwise set
              <code> upload_mode: "file_upload" </code> in <code>platform_options.tiktok</code> to fall back to the
              proxy upload path.
            </p>
            <Code title="Video post">{`{
  "caption": "How we built it",
  "account_ids": ["sa_tiktok_1"],
  "media_urls": ["https://cdn.example.com/build.mp4"],
  "platform_options": {
    "tiktok": {
      "privacy_level": "PUBLIC_TO_EVERYONE"
    }
  }
}`}</Code>
            <Code title="Photo carousel">{`{
  "caption": "Lookbook 📸",
  "account_ids": ["sa_tiktok_1"],
  "media_urls": [
    "https://cdn.example.com/look-1.jpg",
    "https://cdn.example.com/look-2.jpg",
    "https://cdn.example.com/look-3.jpg"
  ],
  "platform_options": {
    "tiktok": {
      "privacy_level": "PUBLIC_TO_EVERYONE",
      "photo_cover_index": 0
    }
  }
}`}</Code>
            <Code title="Response">{`{
  "data": {
    "id": "post_xyz789",
    "status": "published",
    "results": [
      {
        "platform": "tiktok",
        "status": "published",
        "external_id": "v0..."
      }
    ]
  }
}`}</Code>

            <h3 className="doc-section-title" style={{ fontSize: 18, marginTop: 32, marginBottom: 12 }}>YouTube</h3>
            <p className="doc-p">
              OAuth. Exactly 1 video per post (long-form or Shorts). The default privacy is <code>private</code> —
              set <code>privacy_status: &quot;public&quot;</code> to publish immediately. The <code>shorts: true</code>
              flag appends <code>#Shorts</code> to the title and description so YouTube routes the upload into the
              Shorts shelf (the actual 9:16 / &lt;60 s constraint must be satisfied by the source video).
            </p>
            <Code title="Long-form video">{`{
  "caption": "Quarterly product update",
  "account_ids": ["sa_youtube_1"],
  "media_urls": ["https://cdn.example.com/update.mp4"],
  "platform_options": {
    "youtube": {
      "privacy_status": "public",
      "category_id": "22",
      "tags": ["product", "quarterly", "update"]
    }
  }
}`}</Code>
            <Code title="Shorts">{`{
  "caption": "30s feature demo",
  "account_ids": ["sa_youtube_1"],
  "media_urls": ["https://cdn.example.com/demo-vertical.mp4"],
  "platform_options": {
    "youtube": {
      "privacy_status": "public",
      "shorts": true
    }
  }
}`}</Code>
            <Code title="Response">{`{
  "data": {
    "id": "post_xyz789",
    "status": "published",
    "results": [
      {
        "platform": "youtube",
        "status": "published",
        "external_id": "dQw4w9WgXcQ"
      }
    ]
  }
}`}</Code>

            <div className="doc-callout doc-callout-warn">
              <strong>Multi-platform fan-out.</strong> When <code>account_ids</code> spans accounts on multiple
              networks, the API publishes to each one concurrently and returns a single <code>results</code> array
              with one entry per account. Per-account failures don&apos;t block the others — the post status will be
              <code> published</code>, <code>partial</code>, or <code>failed</code> depending on the mix.
              If you need to send platform-specific media (e.g. a horizontal video for YouTube and a vertical clip
              for TikTok), issue separate <code>POST /v1/social-posts</code> calls.
            </div>
          </Section>

          {/* Footer */}
          <div style={{ borderTop: "1px solid var(--border)", paddingTop: 32, marginTop: 56, fontSize: 13, color: "var(--muted)" }}>
            <p>Need help? Contact <a href="mailto:support@unipost.dev" style={{ color: "var(--blue)", textDecoration: "none" }}>support@unipost.dev</a></p>
            <p style={{ marginTop: 6 }}>
              <Link href="/terms" style={{ color: "var(--blue)", textDecoration: "none" }}>Terms</Link>
              {" · "}
              <Link href="/privacy" style={{ color: "var(--blue)", textDecoration: "none" }}>Privacy</Link>
            </p>
          </div>
        </div>
      </div>
    </>
  );
}
