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

function formatFreeTier(pricing: { freeTier: boolean; freePostsPerMonth: number | string; freeTierLabel?: string }) {
  if (!pricing.freeTier) return "No free tier";
  if (pricing.freeTierLabel) return pricing.freeTierLabel;
  return `${pricing.freePostsPerMonth}/mo`;
}

interface OverviewRow { label: string; unipost: RowValue; values: RowValue[] }
const OVERVIEW_ROWS: OverviewRow[] = [
  { label: "Free tier", unipost: "100/mo", values: ALL_COMPETITORS.map((c) => formatFreeTier(c.pricing)) },
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

const CSS = `:root{--cmp-bg:var(--app-bg);--cmp-s1:var(--marketing-surface);--cmp-s2:var(--marketing-surface-alt);--cmp-s3:var(--marketing-surface-elevated);--cmp-border:var(--marketing-border);--cmp-b2:var(--marketing-border-strong);--cmp-b3:var(--marketing-border-strong);--cmp-text:var(--marketing-text);--cmp-muted:var(--marketing-muted);--cmp-muted2:var(--marketing-subtle);--cmp-accent:var(--primary);--cmp-adim:var(--success-soft);--cmp-blue:var(--marketing-link);--cmp-r:8px;--cmp-mono:var(--font-fira-code),monospace;--cmp-ui:var(--font-dm-sans),system-ui,sans-serif;--cmp-nav-max:1480px;--cmp-content-max:1200px;--cmp-px:32px;--cmp-section-py:96px}*{box-sizing:border-box;margin:0;padding:0}body{background:var(--cmp-bg);color:var(--cmp-text);font-family:var(--cmp-ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}

/* NAV */
.cmp-nav{position:sticky;top:0;z-index:50;border-bottom:1px solid var(--marketing-nav-border);background:var(--marketing-nav-bg);backdrop-filter:blur(12px);-webkit-backdrop-filter:blur(12px)}.cmp-nav-inner{max-width:var(--cmp-nav-max);margin:0 auto;padding:0 var(--cmp-px);height:56px;display:flex;align-items:center;justify-content:space-between}.cmp-logo{display:flex;align-items:center;gap:10px;text-decoration:none}.cmp-logo-mark{width:28px;height:28px;background:var(--cmp-accent);border-radius:7px;display:flex;align-items:center;justify-content:center}.cmp-logo-mark svg{width:14px;height:14px;color:var(--primary-foreground)}.cmp-logo-name{font-size:16px;font-weight:700;letter-spacing:-.4px;color:var(--cmp-text)}.cmp-nav-links{display:flex;gap:4px}.cmp-nav-link{padding:6px 14px;font-size:14px;font-weight:500;color:var(--cmp-muted);border-radius:var(--cmp-r);transition:color .1s;text-decoration:none}.cmp-nav-link:hover{color:var(--cmp-text)}

/* BUTTONS */
.cmp-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--cmp-r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--cmp-ui);text-decoration:none;white-space:nowrap}.cmp-btn-primary{background:var(--cmp-blue);color:#fff}.cmp-btn-primary:hover{background:var(--marketing-link-hover);box-shadow:0 0 24px color-mix(in srgb,var(--marketing-link) 20%,transparent)}.cmp-btn-ghost{background:transparent;color:var(--cmp-muted);border-color:var(--cmp-b2)}.cmp-btn-ghost:hover{background:var(--cmp-s2);color:var(--cmp-text);border-color:var(--cmp-b3)}.cmp-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}

/* PAGE */
.cmp-page{width:100%;max-width:var(--cmp-content-max);margin:0 auto;padding:0 var(--cmp-px)}

/* HERO */
.cmp-hero{padding:96px 0 var(--cmp-section-py);text-align:center;display:flex;flex-direction:column;align-items:center}.cmp-hero-title{font-size:56px;font-weight:900;letter-spacing:-2px;line-height:1.08;margin-bottom:20px}.cmp-hero-sub{font-size:17px;color:var(--cmp-muted);max-width:600px;line-height:1.75;margin-bottom:40px}

/* COMPETITOR CARDS */
.cmp-cards{display:grid;grid-template-columns:repeat(3,1fr);gap:16px;margin-bottom:var(--cmp-section-py)}.cmp-card{background:var(--cmp-s1);border:1px solid var(--cmp-b2);border-radius:14px;padding:32px;transition:all .2s;text-decoration:none;color:var(--cmp-text)}.cmp-card:hover{border-color:var(--cmp-accent);transform:translateY(-2px);box-shadow:0 8px 32px var(--cmp-accent-glow)}.cmp-card-name{font-size:20px;font-weight:800;letter-spacing:-.3px;margin-bottom:8px}.cmp-card-desc{font-size:13.5px;color:var(--cmp-muted);line-height:1.6;margin-bottom:20px}.cmp-card-highlights{list-style:none;margin-bottom:24px}.cmp-card-hl{display:flex;align-items:flex-start;gap:8px;font-size:13px;color:var(--cmp-text);margin-bottom:8px;line-height:1.4}.cmp-card-hl svg{color:var(--cmp-accent);flex-shrink:0;margin-top:2px}.cmp-card-link{font-size:13px;font-weight:600;color:var(--cmp-accent);font-family:var(--cmp-mono)}

/* TABLE */
.cmp-table-section{margin-bottom:var(--cmp-section-py)}.cmp-table-title{font-size:36px;font-weight:800;letter-spacing:-.8px;margin-bottom:32px;text-align:center}.cmp-table-wrap{max-width:100%;border:1px solid var(--cmp-border);border-radius:14px;overflow:hidden;overflow-x:auto}.cmp-table-hdr{display:grid;grid-template-columns:2fr 1fr 1fr 1fr 1fr;background:var(--cmp-s2);border-bottom:1px solid var(--cmp-border);min-width:700px}.cmp-th{padding:14px 20px;font-size:12.5px;font-weight:600;color:var(--cmp-muted);letter-spacing:.03em}.cmp-th.hl{color:var(--cmp-accent)}.cmp-th a{color:inherit;text-decoration:none}.cmp-th a:hover{color:var(--cmp-text)}.cmp-row{display:grid;grid-template-columns:2fr 1fr 1fr 1fr 1fr;border-bottom:1px solid var(--cmp-border);transition:background .1s;min-width:700px}.cmp-row:last-child{border-bottom:none}.cmp-row:hover{background:var(--cmp-s2)}.cmp-cell{padding:12px 20px;display:flex;align-items:center;font-size:14px}.cmp-cell-label{font-weight:500;color:var(--cmp-text)}.cmp-chk{color:var(--cmp-accent)}.cmp-chk svg{width:15px;height:15px}.cmp-x{color:var(--cmp-muted2);font-size:16px}.cmp-coming{font-size:12px;color:var(--cmp-muted);font-family:var(--cmp-mono);background:var(--cmp-s2);padding:2px 8px;border-radius:4px;border:1px solid var(--cmp-border)}.cmp-val{font-size:13.5px;color:var(--cmp-text)}

/* CTA */
.cmp-cta{background:var(--app-bg);border-top:1px solid var(--cmp-border);padding:var(--cmp-section-py) 0}.cmp-cta-inner{max-width:var(--cmp-content-max);margin:0 auto;padding:0 var(--cmp-px)}.cmp-cta-card{background:var(--cmp-s1);border:1px solid var(--cmp-border);border-radius:16px;padding:72px 48px;text-align:center;position:relative;overflow:hidden;box-shadow:var(--marketing-shadow-soft)}.cmp-cta-glow{position:absolute;width:400px;height:400px;border-radius:50%;background:var(--marketing-glow);top:50%;left:50%;transform:translate(-50%,-50%);pointer-events:none}.cmp-cta-title{font-size:44px;font-weight:900;letter-spacing:-1.2px;margin-bottom:16px;position:relative}.cmp-cta-sub{font-size:15px;color:var(--cmp-muted);margin-bottom:36px;position:relative}.cmp-cta-actions{display:flex;justify-content:center;gap:12px;position:relative}

/* FOOTER */
.cmp-footer{width:100%;border-top:1px solid var(--cmp-border);padding:48px 0}.cmp-footer-inner{max-width:var(--cmp-content-max);margin:0 auto;padding:0 var(--cmp-px)}.cmp-footer-top{display:grid;grid-template-columns:2fr 1fr 1fr 1fr 1fr;gap:48px;margin-bottom:48px}.cmp-footer-logo{display:flex;align-items:center;gap:9px;margin-bottom:16px}.cmp-footer-mark{width:26px;height:26px;background:var(--cmp-accent);border-radius:6px;display:flex;align-items:center;justify-content:center}.cmp-footer-mark svg{width:13px;height:13px;color:var(--primary-foreground)}.cmp-footer-name{font-size:15px;font-weight:700;color:var(--cmp-text)}.cmp-footer-tagline{font-size:13px;color:var(--cmp-muted);line-height:1.65;max-width:260px}.cmp-footer-col-title{font-size:12px;font-weight:700;text-transform:uppercase;letter-spacing:.08em;color:var(--cmp-muted2);margin-bottom:16px}.cmp-footer-links{list-style:none}.cmp-footer-link{font-size:13.5px;color:var(--cmp-muted);margin-bottom:10px;cursor:pointer;transition:color .1s;display:block;text-decoration:none}.cmp-footer-link:hover{color:var(--cmp-text)}.cmp-footer-bottom{border-top:1px solid var(--cmp-border);padding-top:24px;display:flex;align-items:center;justify-content:space-between}.cmp-footer-copy{font-size:13px;color:var(--cmp-muted2)}.cmp-footer-social{display:flex;gap:12px}.cmp-footer-social-link{width:32px;height:32px;background:var(--cmp-s2);border:1px solid var(--cmp-border);border-radius:6px;display:flex;align-items:center;justify-content:center;color:var(--cmp-muted);cursor:pointer;transition:all .15s;font-size:14px;text-decoration:none}.cmp-footer-social-link:hover{background:var(--cmp-s3);color:var(--cmp-text);border-color:var(--cmp-b2)}

/* LP-BTN COMPAT (MarketingNav/CTA use lp- classes) */
.lp-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--cmp-r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--cmp-ui);text-decoration:none;white-space:nowrap}.lp-btn-primary{background:var(--cmp-blue);color:#fff}.lp-btn-primary:hover{background:var(--marketing-link-hover);box-shadow:0 0 24px color-mix(in srgb,var(--marketing-link) 20%,transparent)}.lp-btn-ghost{background:transparent;color:var(--cmp-muted);border-color:var(--cmp-b2)}.lp-btn-ghost:hover{background:var(--cmp-s2);color:var(--cmp-text);border-color:var(--cmp-b3)}.lp-btn-outline{background:transparent;color:var(--cmp-text);border-color:var(--cmp-b2)}.lp-btn-outline:hover{background:var(--cmp-s2);border-color:var(--cmp-b3)}.lp-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}

@media(min-width:1600px){:root{--cmp-content-max:1200px;--cmp-px:40px}}
@media(max-width:1024px){:root{--cmp-px:24px;--cmp-section-py:72px}}
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
            <UniPostLogo markSize={28} wordmarkColor="var(--cmp-text)" />
          </Link>
          <div className="cmp-nav-links">
            <Link href="/docs" className="cmp-nav-link">Docs</Link>
            <Link href="/pricing" className="cmp-nav-link">Pricing</Link>
            <Link href="/compare" className="cmp-nav-link" style={{ color: "var(--cmp-text)" }}>Compare</Link>
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

        <div
          style={{
            margin: "-48px 0 var(--cmp-section-py)",
            padding: 24,
            border: "1px solid var(--cmp-border)",
            borderRadius: 14,
            background: "var(--cmp-s1)",
          }}
        >
          <div
            style={{
              fontSize: 12,
              fontWeight: 700,
              letterSpacing: ".08em",
              textTransform: "uppercase",
              color: "var(--cmp-muted2)",
              marginBottom: 8,
            }}
          >
            Embedded pricing example
          </div>
          <p style={{ margin: 0, color: "var(--cmp-muted)", lineHeight: 1.7 }}>
            If your app has 100 users and each connects 2 social accounts, Zernio's
            current connected-account pricing is $418/mo. UniPost Growth is $59/mo
            when the same app fits under 7,500 posts/month and Growth feature limits.
          </p>
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

    </>
  );
}
