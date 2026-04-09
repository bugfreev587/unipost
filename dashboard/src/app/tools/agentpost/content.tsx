"use client";

import Link from "next/link";
import { MarketingCTALight } from "@/components/marketing/nav";

// Page-specific styles (nav/footer/base come from layout.tsx)
const CSS = `.ap-hero{padding:var(--section-py) 0 56px;max-width:880px;text-align:center;margin:0 auto}
.ap-eyebrow{font-size:11.5px;color:var(--accent);text-transform:uppercase;letter-spacing:.12em;font-weight:700;margin-bottom:18px;font-family:var(--mono)}
.ap-hero-title{font-size:56px;font-weight:900;letter-spacing:-2px;line-height:1.05;color:var(--text);margin-bottom:24px}
.ap-hero-title em{color:var(--accent);font-style:normal}
.ap-hero-sub{font-size:18px;color:#aaa;line-height:1.7;max-width:680px;margin:0 auto 36px}
.ap-hero-actions{display:flex;align-items:center;justify-content:center;gap:12px;flex-wrap:wrap}
.ap-install{background:var(--s1);border:1px solid var(--b2);border-radius:10px;padding:20px 28px;font-family:var(--mono);font-size:14.5px;color:var(--accent);text-align:center;max-width:560px;margin:40px auto 0;letter-spacing:-.2px;user-select:all}
.ap-install span{color:var(--muted)}
.ap-demo{margin:56px auto var(--section-py);max-width:800px}
.ap-demo-window{background:var(--s1);border:1px solid var(--b2);border-radius:14px;overflow:hidden}
.ap-demo-bar{display:flex;align-items:center;gap:6px;padding:10px 14px;border-bottom:1px solid var(--b2);background:#080808}
.ap-demo-dot{width:10px;height:10px;border-radius:50%;background:var(--b3)}
.ap-demo-body{padding:24px;font-family:var(--mono);font-size:13px;line-height:1.8;color:var(--muted)}
.ap-demo-body .cmd{color:var(--accent)}
.ap-demo-body .prompt{color:var(--muted2)}
.ap-demo-body .output{color:#ccc}
.ap-demo-body .platform{color:#38bdf8}
.ap-demo-body .ok{color:#10b981}
.ap-features{display:grid;grid-template-columns:repeat(3,1fr);gap:20px;padding:0 0 var(--section-py)}
.ap-feat{background:var(--s1);border:1px solid var(--b2);border-radius:14px;padding:28px 26px;display:flex;flex-direction:column;gap:10px}
.ap-feat-title{font-size:16px;font-weight:700;color:var(--text);letter-spacing:-.2px}
.ap-feat-desc{font-size:13.5px;color:#999;line-height:1.65}
.ap-providers{padding:0 0 var(--section-py);text-align:center}
.ap-providers-title{font-size:28px;font-weight:800;letter-spacing:-.5px;margin-bottom:12px}
.ap-providers-sub{font-size:14.5px;color:var(--muted);margin-bottom:32px}
.ap-providers-grid{display:flex;justify-content:center;gap:16px;flex-wrap:wrap}
.ap-provider-card{background:var(--s1);border:1px solid var(--b2);border-radius:10px;padding:20px 32px;display:flex;flex-direction:column;align-items:center;gap:8px;min-width:140px}
.ap-provider-name{font-size:14px;font-weight:600;color:var(--text)}
.ap-provider-model{font-size:11.5px;color:var(--muted);font-family:var(--mono)}
@media(max-width:1024px){.ap-features{grid-template-columns:1fr 1fr}.ap-hero-title{font-size:44px}}
@media(max-width:680px){.ap-features{grid-template-columns:1fr}.ap-hero-title{font-size:34px}}`;

const FEATURES = [
  { title: "One line in, posts everywhere", desc: 'Type agentpost "shipped webhooks today" and the CLI drafts a per-platform post for every connected account. Twitter gets punchy, LinkedIn gets long-form, Bluesky gets casual.' },
  { title: "Preview before publishing", desc: "Ink-rendered cards in your terminal show each draft with character counts, platform labels, and green/yellow/red status. One keypress to publish all, Esc to cancel." },
  { title: "Three LLM providers", desc: "Anthropic Claude (default, prompt-tuned), OpenAI GPT-4o (JSON mode), and Google Gemini. Switch with agentpost init \u2014 your other keys are preserved." },
  { title: "The prompt is the product", desc: "The most important file is src/lib/prompt.ts. Platform-specific style guidance, hard rules (no buzzwords, no invented facts), and 4 few-shot examples. Fork and PR back what works." },
  { title: "Example agents included", desc: "changelog-bot publishes release notes on every git tag. rss-bridge polls any feed and posts new items. Both ship as GitHub Actions you can drop into your repo." },
  { title: "Free and hackable", desc: "MIT licensed. ~500 lines of TypeScript. No telemetry, no accounts, no growth hacks. The CLI is free; the infrastructure underneath (UniPost) is free up to 100 posts/month." },
];

const PROVIDERS = [
  { name: "Anthropic", model: "claude-opus-4-6", emoji: "\u{1F7E3}" },
  { name: "OpenAI", model: "gpt-4o", emoji: "\u{1F7E2}" },
  { name: "Google Gemini", model: "gemini-1.5-pro", emoji: "\u{1F535}" },
];

export function AgentPostContent() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <div className="tl-page">
        {/* Hero */}
        <div className="ap-hero">
          <div className="ap-eyebrow">Open Source Tool</div>
          <h1 className="ap-hero-title"><em>AgentPost</em></h1>
          <p className="ap-hero-sub">
            Describe what you shipped. AI posts it everywhere.
          </p>
          <div className="ap-hero-actions">
            <a href="https://github.com/unipost-dev/agentpost" target="_blank" rel="noopener noreferrer" className="lp-btn lp-btn-accent lp-btn-lg">
              View on GitHub
            </a>
            <a href="https://www.npmjs.com/package/@unipost/agentpost" target="_blank" rel="noopener noreferrer" className="lp-btn lp-btn-outline lp-btn-lg">
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
              <span className="ap-demo-dot" /><span className="ap-demo-dot" /><span className="ap-demo-dot" />
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
              <div className="output">Press <span className="ok">P</span> or <span className="ok">Enter</span> to publish, <span style={{ color: "#ef4444" }}>C</span> or <span style={{ color: "#ef4444" }}>Esc</span> to cancel.</div>
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
            Switch providers with <code style={{ color: "#10b981", fontFamily: "var(--mono)" }}>agentpost init</code>
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
        <div className="tl-cta">
          <div className="tl-cta-inner">
            <div className="tl-cta-glow" />
            <h2 className="tl-cta-title">Start posting in 60 seconds</h2>
            <p className="tl-cta-sub">
              Install the CLI, paste two API keys, and run your first post.
              Free up to 100 posts/month.
            </p>
            <div className="tl-cta-actions">
              <a href="https://github.com/unipost-dev/agentpost" target="_blank" rel="noopener noreferrer" className="lp-btn lp-btn-accent lp-btn-lg">
                Get Started
              </a>
              <MarketingCTALight />
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
