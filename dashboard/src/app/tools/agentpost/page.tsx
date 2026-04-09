"use client";

import Link from "next/link";
import { MarketingNav, MarketingCTA, MarketingCTALight } from "@/components/marketing/nav";

// ── Styles ──
const CSS = `@import url('https://fonts.googleapis.com/css2?family=DM+Sans:opsz,wght@9..40,300;9..40,400;9..40,500;9..40,600;9..40,700;9..40,800;9..40,900&family=Fira+Code:wght@400;500&display=swap');:root{--bg:#000;--s1:#0a0a0a;--s2:#111;--s3:#1a1a1a;--border:#1a1a1a;--b2:#242424;--b3:#2e2e2e;--text:#f0f0f0;--muted:#999;--muted2:#555;--accent:#10b981;--blue:#0ea5e9;--r:8px;--mono:'Fira Code',monospace;--ui:'DM Sans',system-ui,sans-serif;--nav-max:1480px;--content-max:1200px;--text-max:720px;--px:32px;--section-py:96px}*{box-sizing:border-box;margin:0;padding:0}body{background:var(--bg);color:var(--text);font-family:var(--ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}.ap-nav{position:sticky;top:0;z-index:50;width:100%;border-bottom:1px solid var(--border);background:#00000095;backdrop-filter:blur(12px);-webkit-backdrop-filter:blur(12px)}.ap-nav-inner{max-width:var(--nav-max);margin:0 auto;padding:0 var(--px);height:56px;display:flex;align-items:center;justify-content:space-between}.ap-logo{display:flex;align-items:center;gap:10px;text-decoration:none}.ap-logo-mark{width:28px;height:28px;background:var(--accent);border-radius:7px;display:flex;align-items:center;justify-content:center}.ap-logo-mark svg{width:14px;height:14px;color:#000}.ap-logo-name{font-size:16px;font-weight:700;letter-spacing:-.4px;color:var(--text)}.ap-nav-links{display:flex;align-items:center;gap:4px}.ap-nav-link{padding:6px 14px;font-size:14px;font-weight:500;color:var(--muted);cursor:pointer;border-radius:var(--r);transition:color .1s;text-decoration:none}.ap-nav-link:hover{color:var(--text)}.ap-nav-link.active{color:var(--text)}.lp-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--ui);text-decoration:none;white-space:nowrap}.lp-btn-primary{background:var(--accent);color:#000}.lp-btn-primary:hover{background:#34d399;box-shadow:0 0 24px #10b98130}.lp-btn-ghost{background:transparent;color:var(--muted);border-color:var(--b2)}.lp-btn-ghost:hover{background:var(--s2);color:var(--text);border-color:var(--b3)}.lp-btn-outline{background:transparent;color:var(--text);border-color:var(--b2)}.lp-btn-outline:hover{background:var(--s2);border-color:var(--b3)}.lp-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}.ap-page{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}.ap-hero{padding:var(--section-py) 0 56px;max-width:880px;text-align:center;margin:0 auto}.ap-eyebrow{font-size:11.5px;color:var(--accent);text-transform:uppercase;letter-spacing:.12em;font-weight:700;margin-bottom:18px;font-family:var(--mono)}.ap-hero-title{font-size:56px;font-weight:900;letter-spacing:-2px;line-height:1.05;color:var(--text);margin-bottom:24px}.ap-hero-title em{color:var(--accent);font-style:normal}.ap-hero-sub{font-size:18px;color:#aaa;line-height:1.7;max-width:680px;margin:0 auto 36px}.ap-hero-actions{display:flex;align-items:center;justify-content:center;gap:12px;flex-wrap:wrap}.ap-install{background:var(--s1);border:1px solid var(--b2);border-radius:10px;padding:20px 28px;font-family:var(--mono);font-size:14.5px;color:var(--accent);text-align:center;max-width:560px;margin:40px auto 0;letter-spacing:-.2px;user-select:all}.ap-install span{color:var(--muted)}.ap-demo{margin:56px auto var(--section-py);max-width:800px}.ap-demo-window{background:var(--s1);border:1px solid var(--b2);border-radius:14px;overflow:hidden}.ap-demo-bar{display:flex;align-items:center;gap:6px;padding:10px 14px;border-bottom:1px solid var(--b2);background:#080808}.ap-demo-dot{width:10px;height:10px;border-radius:50%;background:var(--b3)}.ap-demo-body{padding:24px;font-family:var(--mono);font-size:13px;line-height:1.8;color:var(--muted)}.ap-demo-body .cmd{color:var(--accent)}.ap-demo-body .prompt{color:var(--muted2)}.ap-demo-body .output{color:#ccc}.ap-demo-body .platform{color:#38bdf8}.ap-demo-body .ok{color:#10b981}.ap-features{display:grid;grid-template-columns:repeat(3,1fr);gap:20px;padding:0 0 var(--section-py)}.ap-feat{background:var(--s1);border:1px solid var(--b2);border-radius:14px;padding:28px 26px;display:flex;flex-direction:column;gap:10px}.ap-feat-title{font-size:16px;font-weight:700;color:var(--text);letter-spacing:-.2px}.ap-feat-desc{font-size:13.5px;color:#999;line-height:1.65}.ap-providers{padding:0 0 var(--section-py);text-align:center}.ap-providers-title{font-size:28px;font-weight:800;letter-spacing:-.5px;margin-bottom:12px}.ap-providers-sub{font-size:14.5px;color:var(--muted);margin-bottom:32px}.ap-providers-grid{display:flex;justify-content:center;gap:16px;flex-wrap:wrap}.ap-provider-card{background:var(--s1);border:1px solid var(--b2);border-radius:10px;padding:20px 32px;display:flex;flex-direction:column;align-items:center;gap:8px;min-width:140px}.ap-provider-name{font-size:14px;font-weight:600;color:var(--text)}.ap-provider-model{font-size:11.5px;color:var(--muted);font-family:var(--mono)}.ap-cta{padding:0 0 var(--section-py)}.ap-cta-inner{background:#0d0d0d;border:1px solid var(--border);border-radius:16px;padding:64px 56px;text-align:center;position:relative;overflow:hidden}.ap-cta-glow{position:absolute;width:400px;height:400px;border-radius:50%;background:radial-gradient(circle,#10b98112,transparent 70%);top:50%;left:50%;transform:translate(-50%,-50%);pointer-events:none}.ap-cta-title{font-size:38px;font-weight:800;letter-spacing:-.8px;margin-bottom:14px;position:relative}.ap-cta-sub{font-size:15px;color:#aaa;margin-bottom:32px;position:relative;max-width:560px;margin-left:auto;margin-right:auto}.ap-cta-actions{display:flex;align-items:center;justify-content:center;gap:12px;position:relative;flex-wrap:wrap}.ap-footer{width:100%;border-top:1px solid var(--border);padding:32px 0;margin-top:32px}.ap-footer-inner{max-width:var(--content-max);margin:0 auto;padding:0 var(--px);display:flex;align-items:center;justify-content:space-between;font-size:13px;color:var(--muted2)}.ap-footer-inner a{color:var(--accent);text-decoration:none}.ap-footer-inner a:hover{text-decoration:underline}@media(max-width:1024px){.ap-features{grid-template-columns:1fr 1fr}.ap-hero-title{font-size:44px}}@media(max-width:680px){.ap-features{grid-template-columns:1fr}.ap-hero-title{font-size:34px}.ap-cta-inner{padding:48px 28px}.ap-cta-title{font-size:28px}}`;

function ZapIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" width="14" height="14">
      <path d="M9 2L4 9h4l-1 5 5-7H8l1-5z" />
    </svg>
  );
}

const FEATURES = [
  {
    title: "One line in, posts everywhere",
    desc: 'Type agentpost "shipped webhooks today" and the CLI drafts a per-platform post for every connected account. Twitter gets punchy, LinkedIn gets long-form, Bluesky gets casual.',
  },
  {
    title: "Preview before publishing",
    desc: "Ink-rendered cards in your terminal show each draft with character counts, platform labels, and green/yellow/red status. One keypress to publish all, Esc to cancel.",
  },
  {
    title: "Three LLM providers",
    desc: "Anthropic Claude (default, prompt-tuned), OpenAI GPT-4o (JSON mode), and Google Gemini. Switch with agentpost init — your other keys are preserved.",
  },
  {
    title: "The prompt is the product",
    desc: "The most important file is src/lib/prompt.ts. Platform-specific style guidance, hard rules (no buzzwords, no invented facts), and 4 few-shot examples. Fork and PR back what works.",
  },
  {
    title: "Example agents included",
    desc: "changelog-bot publishes release notes on every git tag. rss-bridge polls any feed and posts new items. Both ship as GitHub Actions you can drop into your repo.",
  },
  {
    title: "Free and hackable",
    desc: "MIT licensed. ~500 lines of TypeScript. No telemetry, no accounts, no growth hacks. The CLI is free; the infrastructure underneath (UniPost) is free up to 100 posts/month.",
  },
];

const PROVIDERS = [
  { name: "Anthropic", model: "claude-opus-4-6", emoji: "🟣" },
  { name: "OpenAI", model: "gpt-4o", emoji: "🟢" },
  { name: "Google Gemini", model: "gemini-1.5-pro", emoji: "🔵" },
];

export default function AgentPostPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />

      {/* Nav */}
      <nav className="ap-nav">
        <div className="ap-nav-inner">
          <div style={{ display: "flex", alignItems: "center", gap: 32 }}>
            <Link href="/" className="ap-logo">
              <span className="ap-logo-mark"><ZapIcon /></span>
              <span className="ap-logo-name">UniPost</span>
            </Link>
            <div className="ap-nav-links">
              <Link href="/solutions" className="ap-nav-link">Solutions</Link>
              <Link href="/tools" className="ap-nav-link active">Tools</Link>
              <Link href="/pricing" className="ap-nav-link">Pricing</Link>
              <Link href="/docs" className="ap-nav-link">Docs</Link>
            </div>
          </div>
          <MarketingNav />
        </div>
      </nav>

      {/* Hero */}
      <div className="ap-page">
        <div className="ap-hero">
          <div className="ap-eyebrow">Open Source Tool</div>
          <h1 className="ap-hero-title">
            <em>AgentPost</em>
          </h1>
          <p className="ap-hero-sub">
            The AI-native CLI that turns a one-line update into platform-perfect
            social posts and publishes them everywhere — Twitter, LinkedIn,
            Bluesky, Threads, Instagram — in one command.
          </p>
          <div className="ap-hero-actions">
            <a
              href="https://github.com/unipost-dev/agentpost"
              target="_blank"
              rel="noopener noreferrer"
              className="lp-btn lp-btn-primary lp-btn-lg"
            >
              View on GitHub
            </a>
            <a
              href="https://www.npmjs.com/package/@unipost/agentpost"
              target="_blank"
              rel="noopener noreferrer"
              className="lp-btn lp-btn-outline lp-btn-lg"
            >
              npm package
            </a>
          </div>
          <div className="ap-install">
            <span>$</span> npm install -g @unipost/agentpost
          </div>
        </div>

        {/* Terminal demo */}
        <div className="ap-demo">
          <div className="ap-demo-window">
            <div className="ap-demo-bar">
              <span className="ap-demo-dot" />
              <span className="ap-demo-dot" />
              <span className="ap-demo-dot" />
            </div>
            <div className="ap-demo-body">
              <div><span className="prompt">$</span> <span className="cmd">agentpost &quot;shipped webhooks today&quot;</span></div>
              <br />
              <div className="output">Loading your accounts and capabilities...</div>
              <div className="output">Generating drafts for 3 accounts via Anthropic Claude (claude-opus-4-6)...</div>
              <br />
              <div><span className="platform">twitter</span> <span className="output">@yuxiaobohit</span></div>
              <div className="output" style={{ paddingLeft: 16 }}>webhooks are live. for anyone who&#39;s been waiting on</div>
              <div className="output" style={{ paddingLeft: 16 }}>the receipt-side of an integration: it&#39;s done.</div>
              <div style={{ paddingLeft: 16, color: "#10b981", fontSize: 11 }}>127/280 chars</div>
              <br />
              <div><span className="platform">linkedin</span> <span className="output">Xiaobo Yu</span></div>
              <div className="output" style={{ paddingLeft: 16 }}>Webhooks shipped today. This was the most-requested</div>
              <div className="output" style={{ paddingLeft: 16 }}>feature from the last quarter...</div>
              <div style={{ paddingLeft: 16, color: "#10b981", fontSize: 11 }}>312 chars</div>
              <br />
              <div><span className="platform">bluesky</span> <span className="output">@xiaobo.bsky.social</span></div>
              <div className="output" style={{ paddingLeft: 16 }}>webhooks shipped today finally</div>
              <div style={{ paddingLeft: 16, color: "#10b981", fontSize: 11 }}>30/300 chars</div>
              <br />
              <div className="output">Press <span className="ok">P</span> or <span className="ok">Enter</span> to publish all, <span style={{ color: "#ef4444" }}>C</span> or <span style={{ color: "#ef4444" }}>Esc</span> to cancel.</div>
              <br />
              <div><span className="ok">✓ twitter @yuxiaobohit</span></div>
              <div><span className="ok">✓ linkedin Xiaobo Yu</span></div>
              <div><span className="ok">✓ bluesky @xiaobo.bsky.social</span></div>
            </div>
          </div>
        </div>

        {/* Features */}
        <div className="ap-features">
          {FEATURES.map((f) => (
            <div key={f.title} className="ap-feat">
              <div className="ap-feat-title">{f.title}</div>
              <div className="ap-feat-desc">{f.desc}</div>
            </div>
          ))}
        </div>

        {/* Providers */}
        <div className="ap-providers">
          <div className="ap-providers-title">Three AI providers, one prompt</div>
          <div className="ap-providers-sub">
            Switch providers with <code style={{ color: "#10b981", fontFamily: "var(--mono)" }}>agentpost init</code> — your other keys are preserved on disk.
          </div>
          <div className="ap-providers-grid">
            {PROVIDERS.map((p) => (
              <div key={p.name} className="ap-provider-card">
                <span style={{ fontSize: 28 }}>{p.emoji}</span>
                <span className="ap-provider-name">{p.name}</span>
                <span className="ap-provider-model">{p.model}</span>
              </div>
            ))}
          </div>
        </div>

        {/* CTA */}
        <div className="ap-cta">
          <div className="ap-cta-inner">
            <div className="ap-cta-glow" />
            <h2 className="ap-cta-title">Start posting in 60 seconds</h2>
            <p className="ap-cta-sub">
              Install the CLI, paste two API keys, and run your first post.
              Free up to 100 posts/month on the UniPost free tier.
            </p>
            <div className="ap-cta-actions">
              <a
                href="https://github.com/unipost-dev/agentpost"
                target="_blank"
                rel="noopener noreferrer"
                className="lp-btn lp-btn-primary lp-btn-lg"
              >
                Get Started
              </a>
              <MarketingCTALight />
            </div>
          </div>
        </div>
      </div>

      {/* Footer */}
      <footer className="ap-footer">
        <div className="ap-footer-inner">
          <span>&copy; {new Date().getFullYear()} UniPost</span>
          <span>
            <Link href="/tools" style={{ color: "var(--accent)" }}>
              &larr; All tools
            </Link>
          </span>
        </div>
      </footer>
    </>
  );
}
