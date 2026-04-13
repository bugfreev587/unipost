"use client";

import Link from "next/link";
import { UniPostLogo } from "@/components/brand/unipost-logo";
import { MarketingNav, MarketingCTA } from "@/components/marketing/nav";
import { UNIPOST } from "@/data/competitors/unipost";
import { ALL_COMPETITORS } from "@/data/competitors";

// ── Icons ──
function CheckIcon() { return <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2.2" width="14" height="14" style={{ flexShrink: 0 }}><path d="M3 8l4 4 6-7" /></svg>; }

type RowValue = boolean | string | number;
function renderVal(v: RowValue): React.ReactNode {
  if (v === true) return <span className="cmp-chk"><CheckIcon /></span>;
  if (v === false) return <span className="cmp-x">✕</span>;
  if (v === "coming") return <span className="cmp-coming">Coming</span>;
  return <span className="cmp-val">{String(v)}</span>;
}

interface OverviewRow { label: string; unipost: RowValue; values: RowValue[] }
const OVERVIEW_ROWS: OverviewRow[] = [
  { label: "Free tier", unipost: "✅ 100/mo", values: ALL_COMPETITORS.map((c) => c.pricing.freeTier ? `✅ ${c.pricing.freePostsPerMonth}/mo` : "❌") },
  { label: "Starting price", unipost: `$${UNIPOST.pricing.startingPrice}/mo`, values: ALL_COMPETITORS.map((c) => c.pricing.startingPrice ? `$${c.pricing.startingPrice}/mo` : "Custom") },
  { label: "Total platforms", unipost: UNIPOST.platforms.total, values: ALL_COMPETITORS.map((c) => c.platforms.total) },
  { label: "X / Twitter", unipost: UNIPOST.platforms.x, values: ALL_COMPETITORS.map((c) => c.platforms.x) },
  { label: "Bluesky", unipost: UNIPOST.platforms.bluesky, values: ALL_COMPETITORS.map((c) => c.platforms.bluesky) },
  { label: "MCP Server", unipost: UNIPOST.features.mcpServer, values: ALL_COMPETITORS.map((c) => c.features.mcpServer) },
  { label: "Webhooks", unipost: UNIPOST.features.webhooks, values: ALL_COMPETITORS.map((c) => c.features.webhooks) },
  { label: "Post analytics", unipost: UNIPOST.features.postAnalytics, values: ALL_COMPETITORS.map((c) => c.features.postAnalytics) },
  { label: "Scheduled posts", unipost: UNIPOST.features.scheduledPosts, values: ALL_COMPETITORS.map((c) => c.features.scheduledPosts) },
  { label: "White-label (BYOC)", unipost: UNIPOST.features.nativeMode, values: ALL_COMPETITORS.map((c) => c.features.nativeMode) },
  { label: "Quickstart mode", unipost: UNIPOST.features.quickstartMode, values: ALL_COMPETITORS.map((c) => c.features.quickstartMode) },
  { label: "First comment", unipost: UNIPOST.features.firstComment, values: ALL_COMPETITORS.map((c) => c.features.firstComment) },
  { label: "SOC 2", unipost: UNIPOST.compliance.soc2, values: ALL_COMPETITORS.map((c) => c.compliance.soc2) },
  { label: "GDPR", unipost: UNIPOST.compliance.gdpr, values: ALL_COMPETITORS.map((c) => c.compliance.gdpr) },
  { label: "Open source", unipost: UNIPOST.developerExperience.openSource, values: ALL_COMPETITORS.map((c) => c.developerExperience.openSource) },
];

const CSS = `:root{--bg:#000;--s1:#0a0a0a;--s2:#111;--s3:#1a1a1a;--border:#1a1a1a;--b2:#242424;--b3:#2e2e2e;--text:#fff;--muted:#b0b0b0;--muted2:#777;--accent:#10b981;--adim:#10b98112;--blue:#0ea5e9;--r:8px;--mono:var(--font-fira-code),monospace;--ui:var(--font-dm-sans),system-ui,sans-serif;--nav-max:1480px;--content-max:1200px;--px:32px;--section-py:96px}*{box-sizing:border-box;margin:0;padding:0}body{background:var(--bg);color:var(--text);font-family:var(--ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}

/* NAV */
.cmp-nav{position:sticky;top:0;z-index:50;border-bottom:1px solid var(--border);background:#00000095;backdrop-filter:blur(12px);-webkit-backdrop-filter:blur(12px)}.cmp-nav-inner{max-width:var(--nav-max);margin:0 auto;padding:0 var(--px);height:56px;display:flex;align-items:center;justify-content:space-between}.cmp-logo{display:flex;align-items:center;gap:10px;text-decoration:none}.cmp-logo-mark{width:28px;height:28px;background:var(--accent);border-radius:7px;display:flex;align-items:center;justify-content:center}.cmp-logo-mark svg{width:14px;height:14px;color:#000}.cmp-logo-name{font-size:16px;font-weight:700;letter-spacing:-.4px;color:var(--text)}.cmp-nav-links{display:flex;gap:4px}.cmp-nav-link{padding:6px 14px;font-size:14px;font-weight:500;color:var(--muted);border-radius:var(--r);transition:color .1s;text-decoration:none}.cmp-nav-link:hover{color:var(--text)}

/* BUTTONS */
.cmp-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--ui);text-decoration:none;white-space:nowrap}.cmp-btn-primary{background:var(--blue);color:#000}.cmp-btn-primary:hover{background:#38bdf8;box-shadow:0 0 24px #0ea5e930}.cmp-btn-ghost{background:transparent;color:var(--muted);border-color:var(--b2)}.cmp-btn-ghost:hover{background:var(--s2);color:var(--text);border-color:var(--b3)}.cmp-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}

/* PAGE */
.cmp-page{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}

/* HERO */
.cmp-hero{padding:96px 0 var(--section-py);text-align:center;display:flex;flex-direction:column;align-items:center}.cmp-hero-title{font-size:56px;font-weight:900;letter-spacing:-2px;line-height:1.08;margin-bottom:20px}.cmp-hero-sub{font-size:17px;color:#bbb;max-width:600px;line-height:1.75;margin-bottom:40px}

/* COMPETITOR CARDS */
.cmp-cards{display:grid;grid-template-columns:repeat(3,1fr);gap:16px;margin-bottom:var(--section-py)}.cmp-card{background:var(--s1);border:1px solid var(--b2);border-radius:14px;padding:32px;transition:all .2s;text-decoration:none;color:var(--text)}.cmp-card:hover{border-color:var(--accent);transform:translateY(-2px);box-shadow:0 8px 32px #10b98110}.cmp-card-name{font-size:20px;font-weight:800;letter-spacing:-.3px;margin-bottom:8px}.cmp-card-desc{font-size:13.5px;color:var(--muted);line-height:1.6;margin-bottom:20px}.cmp-card-highlights{list-style:none;margin-bottom:24px}.cmp-card-hl{display:flex;align-items:flex-start;gap:8px;font-size:13px;color:#ccc;margin-bottom:8px;line-height:1.4}.cmp-card-hl svg{color:var(--accent);flex-shrink:0;margin-top:2px}.cmp-card-link{font-size:13px;font-weight:600;color:var(--accent);font-family:var(--mono)}

/* TABLE */
.cmp-table-section{margin-bottom:var(--section-py)}.cmp-table-title{font-size:36px;font-weight:800;letter-spacing:-.8px;margin-bottom:32px;text-align:center}.cmp-table-wrap{border:1px solid var(--border);border-radius:14px;overflow:hidden;overflow-x:auto}.cmp-table-hdr{display:grid;grid-template-columns:2fr 1fr 1fr 1fr 1fr;background:var(--s2);border-bottom:1px solid var(--border);min-width:700px}.cmp-th{padding:14px 20px;font-size:12.5px;font-weight:600;color:var(--muted);letter-spacing:.03em}.cmp-th.hl{color:var(--accent)}.cmp-th a{color:inherit;text-decoration:none}.cmp-th a:hover{color:var(--text)}.cmp-row{display:grid;grid-template-columns:2fr 1fr 1fr 1fr 1fr;border-bottom:1px solid var(--border);transition:background .1s;min-width:700px}.cmp-row:last-child{border-bottom:none}.cmp-row:hover{background:var(--s2)}.cmp-cell{padding:12px 20px;display:flex;align-items:center;font-size:14px}.cmp-cell-label{font-weight:500;color:var(--text)}.cmp-chk{color:var(--accent)}.cmp-chk svg{width:15px;height:15px}.cmp-x{color:var(--muted2);font-size:16px}.cmp-coming{font-size:12px;color:var(--muted);font-family:var(--mono);background:var(--s2);padding:2px 8px;border-radius:4px;border:1px solid var(--border)}.cmp-val{font-size:13.5px;color:var(--text)}

/* CTA */
.cmp-cta{background:#080808;border-top:1px solid #161616;padding:var(--section-py) 0}.cmp-cta-inner{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}.cmp-cta-card{background:#0f0f0f;border:1px solid #1a1a1a;border-radius:16px;padding:72px 48px;text-align:center;position:relative;overflow:hidden}.cmp-cta-glow{position:absolute;width:400px;height:400px;border-radius:50%;background:radial-gradient(circle,#10b98110,transparent 70%);top:50%;left:50%;transform:translate(-50%,-50%);pointer-events:none}.cmp-cta-title{font-size:44px;font-weight:900;letter-spacing:-1.2px;margin-bottom:16px;position:relative}.cmp-cta-sub{font-size:15px;color:var(--muted);margin-bottom:36px;position:relative}.cmp-cta-actions{display:flex;justify-content:center;gap:12px;position:relative}

/* FOOTER */
.cmp-footer{width:100%;border-top:1px solid var(--border);padding:48px 0}.cmp-footer-inner{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}.cmp-footer-top{display:grid;grid-template-columns:2fr 1fr 1fr 1fr 1fr;gap:48px;margin-bottom:48px}.cmp-footer-logo{display:flex;align-items:center;gap:9px;margin-bottom:16px}.cmp-footer-mark{width:26px;height:26px;background:var(--accent);border-radius:6px;display:flex;align-items:center;justify-content:center}.cmp-footer-mark svg{width:13px;height:13px;color:#000}.cmp-footer-name{font-size:15px;font-weight:700;color:var(--text)}.cmp-footer-tagline{font-size:13px;color:#bbb;line-height:1.65;max-width:260px}.cmp-footer-col-title{font-size:12px;font-weight:700;text-transform:uppercase;letter-spacing:.08em;color:var(--muted2);margin-bottom:16px}.cmp-footer-links{list-style:none}.cmp-footer-link{font-size:13.5px;color:#bbb;margin-bottom:10px;cursor:pointer;transition:color .1s;display:block;text-decoration:none}.cmp-footer-link:hover{color:var(--text)}.cmp-footer-bottom{border-top:1px solid var(--border);padding-top:24px;display:flex;align-items:center;justify-content:space-between}.cmp-footer-copy{font-size:13px;color:var(--muted2)}.cmp-footer-social{display:flex;gap:12px}.cmp-footer-social-link{width:32px;height:32px;background:var(--s2);border:1px solid var(--border);border-radius:6px;display:flex;align-items:center;justify-content:center;color:var(--muted);cursor:pointer;transition:all .15s;font-size:14px;text-decoration:none}.cmp-footer-social-link:hover{background:var(--s3);color:var(--text);border-color:var(--b2)}

/* LP-BTN COMPAT (MarketingNav/CTA use lp- classes) */
.lp-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--ui);text-decoration:none;white-space:nowrap}.lp-btn-primary{background:var(--blue);color:#000}.lp-btn-primary:hover{background:#38bdf8;box-shadow:0 0 24px #0ea5e930}.lp-btn-ghost{background:transparent;color:var(--muted);border-color:var(--b2)}.lp-btn-ghost:hover{background:var(--s2);color:var(--text);border-color:var(--b3)}.lp-btn-outline{background:transparent;color:var(--text);border-color:var(--b2)}.lp-btn-outline:hover{background:var(--s2);border-color:var(--b3)}.lp-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}

@media(min-width:1600px){:root{--content-max:1200px;--px:40px}}
@media(max-width:1024px){:root{--px:24px;--section-py:72px}}
@media(max-width:768px){.cmp-hero-title{font-size:36px}.cmp-cards{grid-template-columns:1fr}.cmp-table-hdr,.cmp-row{min-width:700px}.cmp-footer-top{grid-template-columns:1fr 1fr;gap:32px}.cmp-footer-bottom{flex-direction:column;gap:12px;text-align:center}}
`;

const COMPETITORS_LIST = ALL_COMPETITORS;

export default function ComparePage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />

      {/* NAV */}
      <nav className="cmp-nav">
        <div className="cmp-nav-inner">
          <Link href="/" className="cmp-logo">
            <UniPostLogo markSize={28} wordmarkColor="var(--text)" />
          </Link>
          <div className="cmp-nav-links">
            <Link href="/docs" className="cmp-nav-link">Docs</Link>
            <Link href="/pricing" className="cmp-nav-link">Pricing</Link>
            <Link href="/compare" className="cmp-nav-link" style={{ color: "var(--text)" }}>Compare</Link>
          </div>
          <MarketingNav />
        </div>
      </nav>

      <div className="cmp-page">
        {/* HERO */}
        <div className="cmp-hero">
          <h1 className="cmp-hero-title">UniPost vs every social media API</h1>
          <p className="cmp-hero-sub">See how UniPost compares to other social media APIs. Free tier, simple pricing, and native MCP Server support.</p>
        </div>

        {/* COMPETITOR CARDS */}
        <div className="cmp-cards">
          {COMPETITORS_LIST.map((c) => (
            <Link key={c.slug} href={`/alternatives/${c.slug}`} className="cmp-card">
              <div className="cmp-card-name">UniPost vs {c.name}</div>
              <p className="cmp-card-desc">{c.tagline}</p>
              <ul className="cmp-card-highlights">
                {c.verdict.chooseUs.slice(0, 3).map((h) => (
                  <li key={h} className="cmp-card-hl"><CheckIcon />{h}</li>
                ))}
              </ul>
              <span className="cmp-card-link">Full comparison →</span>
            </Link>
          ))}
        </div>

        {/* OVERVIEW TABLE */}
        <div className="cmp-table-section">
          <h2 className="cmp-table-title">Quick comparison</h2>
          <div className="cmp-table-wrap">
            <div className="cmp-table-hdr">
              <div className="cmp-th">Feature</div>
              <div className="cmp-th hl">UniPost</div>
              {COMPETITORS_LIST.map((c) => (
                <div key={c.slug} className="cmp-th"><Link href={`/alternatives/${c.slug}`}>{c.name}</Link></div>
              ))}
            </div>
            {OVERVIEW_ROWS.map((row) => (
              <div key={row.label} className="cmp-row">
                <div className="cmp-cell cmp-cell-label">{row.label}</div>
                <div className="cmp-cell">{renderVal(row.unipost)}</div>
                {row.values.map((v, i) => (
                  <div key={i} className="cmp-cell">{renderVal(v)}</div>
                ))}
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* CTA */}
      <div className="cmp-cta">
        <div className="cmp-cta-inner">
          <div className="cmp-cta-card">
            <div className="cmp-cta-glow" />
            <h2 className="cmp-cta-title">Start building for free</h2>
            <p className="cmp-cta-sub">Free plan · 100 posts/month · No credit card</p>
            <div className="cmp-cta-actions">
              <MarketingCTA />
              <Link href="/pricing" className="cmp-btn cmp-btn-ghost cmp-btn-lg">View Pricing →</Link>
            </div>
          </div>
        </div>
      </div>

      {/* FOOTER */}
      <footer className="cmp-footer">
        <div className="cmp-footer-inner">
          <div className="cmp-footer-top">
            <div>
              <div className="cmp-footer-logo"><UniPostLogo markSize={26} wordmarkColor="var(--text)" wordmarkClassName="cmp-footer-name" /></div>
              <p className="cmp-footer-tagline">Unified social media API for developers. Post to 7 platforms with one API call.</p>
            </div>
            <div>
              <div className="cmp-footer-col-title">Product</div>
              <ul className="cmp-footer-links">
                <li><Link href="/" className="cmp-footer-link">Overview</Link></li>
                <li><Link href="/pricing" className="cmp-footer-link">Pricing</Link></li>
                <li><Link href="/docs" className="cmp-footer-link">Docs</Link></li>
              </ul>
            </div>
            <div>
              <div className="cmp-footer-col-title">Compare</div>
              <ul className="cmp-footer-links">
                {COMPETITORS_LIST.map((c) => (
                  <li key={c.slug}><Link href={`/alternatives/${c.slug}`} className="cmp-footer-link">vs {c.name}</Link></li>
                ))}
                <li><Link href="/compare" className="cmp-footer-link">All Comparisons →</Link></li>
              </ul>
            </div>
            <div>
              <div className="cmp-footer-col-title">Legal</div>
              <ul className="cmp-footer-links">
                <li><Link href="/privacy" className="cmp-footer-link">Privacy</Link></li>
                <li><Link href="/terms" className="cmp-footer-link">Terms</Link></li>
              </ul>
            </div>
          </div>
          <div className="cmp-footer-bottom">
            <div className="cmp-footer-copy">&copy; 2026 UniPost. All rights reserved.</div>
            <div className="cmp-footer-social">
              <a href="https://x.com/unipostdev" className="cmp-footer-social-link" target="_blank" rel="noopener noreferrer">𝕏</a>
            </div>
          </div>
        </div>
      </footer>
    </>
  );
}
