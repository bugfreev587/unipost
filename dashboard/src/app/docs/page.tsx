"use client";

import Link from "next/link";
import { useState, useEffect, useRef } from "react";
import { MarketingNav } from "@/components/marketing/nav";

const BASE = "https://api.unipost.dev";

const NAV_ITEMS = [
  ["overview", "Overview"],
  ["authentication", "Authentication"],
  ["quick-start", "Quick Start"],
  ["social-accounts", "Social Accounts"],
  ["social-posts", "Social Posts"],
  ["webhooks", "Webhooks"],
  ["oauth", "OAuth Flow"],
  ["billing", "Billing & Usage"],
  ["errors", "Error Handling"],
  ["platforms", "Supported Platforms"],
];

// ── Styles ──
const CSS = `@import url('https://fonts.googleapis.com/css2?family=DM+Sans:opsz,wght@9..40,300;9..40,400;9..40,500;9..40,600;9..40,700;9..40,800&family=Fira+Code:wght@400;500&display=swap');:root{--bg:#000;--s1:#0a0a0a;--s2:#111;--s3:#1a1a1a;--border:#1a1a1a;--b2:#242424;--b3:#2e2e2e;--text:#f0f0f0;--muted:#666;--muted2:#333;--accent:#10b981;--blue:#0ea5e9;--r:8px;--mono:'Fira Code',monospace;--ui:'DM Sans',system-ui,sans-serif;--nav-max:1480px;--content-max:1320px;--px:32px}*{box-sizing:border-box;margin:0;padding:0}body{background:var(--bg);color:var(--text);font-family:var(--ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}.doc-nav{position:sticky;top:0;z-index:50;border-bottom:1px solid var(--border);background:#00000095;backdrop-filter:blur(12px)}.doc-nav-inner{max-width:var(--nav-max);margin:0 auto;padding:0 var(--px);height:56px;display:flex;align-items:center;justify-content:space-between}.doc-logo{display:flex;align-items:center;gap:10px;text-decoration:none}.doc-logo-mark{width:28px;height:28px;background:var(--accent);border-radius:7px;display:flex;align-items:center;justify-content:center}.doc-logo-mark svg{width:14px;height:14px;color:#000}.doc-logo-name{font-size:16px;font-weight:700;letter-spacing:-.4px;color:var(--text)}.doc-nav-links{display:flex;gap:4px}.doc-nav-link{padding:6px 12px;font-size:13.5px;color:var(--muted);border-radius:var(--r);transition:color .1s;text-decoration:none}.doc-nav-link:hover{color:var(--text)}.doc-nav-link.active{color:var(--text);font-weight:500}.doc-layout{max-width:var(--content-max);margin:0 auto;padding:0 var(--px);display:flex;gap:48px;padding-top:40px;padding-bottom:96px}.doc-sidebar{width:200px;flex-shrink:0;position:sticky;top:96px;align-self:flex-start;max-height:calc(100vh - 120px);overflow-y:auto}.doc-sidebar-list{list-style:none}.doc-sidebar-item{margin-bottom:2px}.doc-sidebar-link{display:block;padding:5px 10px;font-size:13px;color:var(--muted);border-radius:6px;text-decoration:none;transition:all .1s}.doc-sidebar-link:hover{color:var(--text);background:var(--s2)}.doc-sidebar-link.active{color:var(--accent);background:var(--s2);font-weight:500}.doc-main{flex:1;min-width:0}.doc-title{font-size:36px;font-weight:800;letter-spacing:-.8px;margin-bottom:8px;color:var(--text)}.doc-subtitle{font-size:16px;color:var(--muted);margin-bottom:48px;line-height:1.7}.doc-section{scroll-margin-top:96px;margin-bottom:56px}.doc-section-title{font-size:22px;font-weight:700;letter-spacing:-.3px;margin-bottom:16px;color:var(--text)}.doc-section-title a{color:inherit;text-decoration:none}.doc-section-title a:hover{color:var(--accent)}.doc-p{font-size:14px;color:var(--muted);line-height:1.75;margin-bottom:16px}.doc-p a{color:var(--blue);text-decoration:none}.doc-p a:hover{text-decoration:underline}.doc-p code{font-family:var(--mono);font-size:12.5px;background:var(--s2);border:1px solid var(--border);padding:1px 6px;border-radius:4px;color:var(--text)}.doc-endpoint{border:1px solid var(--border);border-radius:10px;margin-bottom:24px;overflow:hidden}.doc-endpoint-header{display:flex;align-items:center;gap:10px;padding:12px 18px;background:var(--s2);border-bottom:1px solid var(--border)}.doc-method{padding:2px 8px;border-radius:4px;font-size:11px;font-weight:700;font-family:var(--mono)}.doc-method-get{background:#10b98120;color:var(--accent)}.doc-method-post{background:#0ea5e920;color:var(--blue)}.doc-method-patch{background:#f59e0b20;color:#f59e0b}.doc-method-delete{background:#ef444420;color:#ef4444}.doc-endpoint-path{font-family:var(--mono);font-size:13px;color:var(--text)}.doc-endpoint-auth{font-size:11px;color:var(--muted);margin-left:auto}.doc-endpoint-body{padding:18px;font-size:14px;color:var(--muted);line-height:1.7}.doc-code-wrap{margin:12px 0}.doc-code-label{font-size:11px;color:var(--muted);margin-bottom:4px}.doc-code{background:var(--s1);border:1px solid var(--border);border-radius:8px;padding:16px 20px;font-family:var(--mono);font-size:12.5px;line-height:1.7;color:#a0a0a0;overflow-x:auto;white-space:pre}.doc-param{display:flex;gap:8px;padding:4px 0;font-size:13px;align-items:baseline;flex-wrap:wrap}.doc-param-name{font-family:var(--mono);color:var(--blue);white-space:nowrap}.doc-param-type{font-size:11px;color:var(--muted2)}.doc-param-req{font-size:11px;color:#ef4444}.doc-param-desc{color:var(--muted)}.doc-param-desc code{font-family:var(--mono);font-size:11.5px;background:var(--s2);border:1px solid var(--border);padding:0 5px;border-radius:3px;color:var(--text)}.doc-callout{padding:14px 18px;border-radius:8px;font-size:13px;line-height:1.7;margin:16px 0}.doc-callout-info{background:var(--s2);border:1px solid var(--border);color:var(--muted)}.doc-callout-warn{background:#f59e0b08;border:1px solid #f59e0b20;color:#f59e0b}.doc-callout strong{color:var(--text)}.doc-grid-2{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin:16px 0}.doc-grid-card{border:1px solid var(--border);border-radius:8px;padding:14px 18px}.doc-grid-card-title{font-size:13px;font-weight:600;color:var(--text);margin-bottom:4px}.doc-grid-card-val{font-family:var(--mono);font-size:13px;color:var(--blue)}.doc-table{width:100%;border-collapse:collapse;margin:16px 0}.doc-table th{text-align:left;padding:10px 16px;font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:.04em;color:var(--muted);background:var(--s2);border-bottom:1px solid var(--border)}.doc-table td{padding:10px 16px;font-size:13px;border-bottom:1px solid var(--border);color:var(--muted)}.doc-table td code{font-family:var(--mono);color:#ef4444;font-size:12px}.doc-platform{border:1px solid var(--border);border-radius:8px;padding:14px 18px;margin-bottom:8px}.doc-platform-header{display:flex;align-items:center;justify-content:space-between;margin-bottom:4px}.doc-platform-name{font-size:14px;font-weight:600;color:var(--text)}.doc-platform-auth{font-size:11px;color:var(--muted)}.doc-platform-content{font-size:13px;color:var(--muted)}.doc-platform-note{font-size:11.5px;color:var(--muted2);margin-top:4px}.doc-footer{border-top:1px solid var(--border);padding:32px 0}.doc-footer-inner{max-width:1100px;margin:0 auto;padding:0 32px;font-size:13px;color:var(--muted)}.doc-footer-inner a{color:var(--blue);text-decoration:none}.doc-footer-inner a:hover{text-decoration:underline}::-webkit-scrollbar{width:5px;height:5px}::-webkit-scrollbar-track{background:transparent}::-webkit-scrollbar-thumb{background:var(--b2);border-radius:3px}.doc-sidebar-link.active{color:var(--accent);background:var(--s2);font-weight:500;border-left:2px solid var(--accent);border-radius:0 6px 6px 0;padding-left:8px}@media(min-width:1600px){:root{--nav-max:1560px;--content-max:1360px;--px:40px}}@media(max-width:1024px){:root{--nav-max:100%;--content-max:100%;--px:24px}}`;

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

function Code({ children, title }: { children: string; title?: string }) {
  return (
    <div className="doc-code-wrap">
      {title && <div className="doc-code-label">{title}</div>}
      <pre className="doc-code">{children}</pre>
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
            <Code title="Example">{`curl ${BASE}/v1/social-accounts \\\n  -H "Authorization: Bearer up_live_your_api_key_here"`}</Code>
            <p className="doc-p"><strong>Key format:</strong> <code>up_live_</code> (production) or <code>up_test_</code> (test)</p>
            <p className="doc-p"><strong>Security:</strong> Keys are shown only once at creation. Store them securely — never commit to version control.</p>
          </Section>

          <Section id="quick-start" title="Quick Start">
            <p className="doc-p">Get posting in 3 steps:</p>
            <p className="doc-p"><strong>1. Connect a social account</strong></p>
            <Code>{`curl -X POST ${BASE}/v1/social-accounts/connect \\\n  -H "Authorization: Bearer up_live_your_key" \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "platform": "bluesky",\n    "credentials": {\n      "handle": "yourname.bsky.social",\n      "app_password": "xxxx-xxxx-xxxx-xxxx"\n    }\n  }'`}</Code>
            <p className="doc-p"><strong>2. Get your account ID from the response</strong></p>
            <Code>{`{\n  "data": {\n    "id": "sa_abc123",\n    "platform": "bluesky",\n    "account_name": "yourname.bsky.social",\n    "status": "active"\n  }\n}`}</Code>
            <p className="doc-p"><strong>3. Create a post</strong></p>
            <Code>{`curl -X POST ${BASE}/v1/social-posts \\\n  -H "Authorization: Bearer up_live_your_key" \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "caption": "Hello from UniPost!",\n    "account_ids": ["sa_abc123"]\n  }'`}</Code>
          </Section>

          <Section id="social-accounts" title="Social Accounts">
            <p className="doc-p">Connect, list, and disconnect social media accounts.</p>
            <Endpoint method="POST" path="/v1/social-accounts/connect" auth="API Key">
              <p className="doc-p">Connect a new social media account. For Bluesky, provide credentials directly. For OAuth platforms, use the <a href="#oauth">OAuth flow</a>.</p>
              <Param name="platform" type="string" required>Platform: <code>bluesky</code></Param>
              <Param name="credentials" type="object" required>Platform-specific credentials</Param>
              <Code title="Example: Connect Bluesky">{`curl -X POST ${BASE}/v1/social-accounts/connect \\\n  -H "Authorization: Bearer up_live_your_key" \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "platform": "bluesky",\n    "credentials": {\n      "handle": "alice.bsky.social",\n      "app_password": "xxxx-xxxx-xxxx-xxxx"\n    }\n  }'`}</Code>
              <Code title="Response (201)">{`{\n  "data": {\n    "id": "sa_abc123",\n    "platform": "bluesky",\n    "account_name": "alice.bsky.social",\n    "connected_at": "2026-04-02T10:00:00Z",\n    "status": "active"\n  }\n}`}</Code>
            </Endpoint>
            <Endpoint method="GET" path="/v1/social-accounts" auth="API Key">
              <p className="doc-p">List all connected social accounts.</p>
              <Code title="Example">{`curl ${BASE}/v1/social-accounts \\\n  -H "Authorization: Bearer up_live_your_key"`}</Code>
            </Endpoint>
            <Endpoint method="DELETE" path="/v1/social-accounts/{id}" auth="API Key">
              <p className="doc-p">Disconnect a social account and invalidate its tokens.</p>
              <Code title="Example">{`curl -X DELETE ${BASE}/v1/social-accounts/sa_abc123 \\\n  -H "Authorization: Bearer up_live_your_key"`}</Code>
            </Endpoint>
          </Section>

          <Section id="social-posts" title="Social Posts">
            <p className="doc-p">Create, list, get, and delete social media posts. Posts can be published to multiple accounts simultaneously.</p>
            <Endpoint method="POST" path="/v1/social-posts" auth="API Key">
              <p className="doc-p">Create and publish a post to one or more connected accounts. Posts are published concurrently — one failure won&apos;t block others.</p>
              <Param name="caption" type="string" required>The text content</Param>
              <Param name="account_ids" type="string[]" required>Array of social account IDs</Param>
              <Param name="media_urls" type="string[]">Array of media URLs</Param>
              <p className="doc-p"><strong>Response Headers</strong></p>
              <Param name="X-UniPost-Usage" type="header">Current usage, e.g. <code>450/1000</code></Param>
              <Param name="X-UniPost-Warning" type="header">Warning: <code>approaching_limit</code> or <code>over_limit</code></Param>
              <Code title="Example">{`curl -X POST ${BASE}/v1/social-posts \\\n  -H "Authorization: Bearer up_live_your_key" \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "caption": "Hello from UniPost! 🚀",\n    "account_ids": ["sa_bluesky_123", "sa_linkedin_456"],\n    "media_urls": ["https://example.com/image.jpg"]\n  }'`}</Code>
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
          </Section>

          <Section id="webhooks" title="Webhooks">
            <p className="doc-p">Register webhook endpoints for real-time notifications.</p>
            <Endpoint method="POST" path="/v1/webhooks" auth="API Key">
              <Param name="url" type="string" required>HTTPS endpoint URL</Param>
              <Param name="events" type="string[]" required>Events: <code>post.published</code>, <code>post.failed</code>, <code>account.connected</code>, <code>account.disconnected</code></Param>
              <Param name="secret" type="string" required>HMAC-SHA256 signing secret</Param>
              <Code title="Webhook payload">{`{\n  "event": "post.published",\n  "timestamp": "2026-04-02T12:00:01Z",\n  "data": {\n    "post_id": "post_xyz789",\n    "platform": "bluesky",\n    "external_id": "at://..."\n  }\n}`}</Code>
              <p className="doc-p">Verify with <code>X-UniPost-Signature</code> header: <code>HMAC-SHA256(secret, body)</code></p>
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
            {[
              { name: "Bluesky", auth: "App Password", content: "Text, Images", notes: "Generate at bsky.app → Settings → App Passwords" },
              { name: "LinkedIn", auth: "OAuth", content: "Text, Links", notes: "Requires Share on LinkedIn product" },
              { name: "Instagram", auth: "OAuth", content: "Images (required)", notes: "Business or Creator account required" },
              { name: "Threads", auth: "OAuth", content: "Text, Images", notes: "Uses Meta developer app" },
              { name: "TikTok", auth: "OAuth", content: "Video (required)", notes: "MP4/H.264, min 3 seconds" },
              { name: "YouTube", auth: "OAuth", content: "Video (required)", notes: "YouTube Data API v3" },
            ].map((p) => (
              <div key={p.name} className="doc-platform">
                <div className="doc-platform-header">
                  <span className="doc-platform-name">{p.name}</span>
                  <span className="doc-platform-auth">{p.auth}</span>
                </div>
                <div className="doc-platform-content">Content: {p.content}</div>
                <div className="doc-platform-note">{p.notes}</div>
              </div>
            ))}
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
