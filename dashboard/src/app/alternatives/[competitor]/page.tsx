"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { UniPostLogo } from "@/components/brand/unipost-logo";
import { MarketingNav, MarketingCTA } from "@/components/marketing/nav";
import { UNIPOST } from "@/data/competitors/unipost";
import { getCompetitorBySlug } from "@/data/competitors";
import type { Competitor } from "@/data/competitors";

// ── Icons ──
function CheckIcon() { return <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2.2" width="14" height="14" style={{ flexShrink: 0 }}><path d="M3 8l4 4 6-7" /></svg>; }
function ArrowIcon() { return <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" width="14" height="14" style={{ flexShrink: 0 }}><path d="M3 8h10M9 4l4 4-4 4" /></svg>; }

// ── Comparison table data ──
type RowValue = boolean | string | number;
interface CompareSection { title: string; rows: { label: string; us: RowValue; them: RowValue }[] }

function buildSections(comp: Competitor): CompareSection[] {
  const u = UNIPOST;
  const c = comp;
  return [
    { title: "Pricing", rows: [
      { label: "Free tier", us: "✅ 100 posts/mo", them: c.pricing.freeTier ? `✅ ${c.pricing.freePostsPerMonth} posts/mo` : "❌ No free tier" },
      { label: "Starting price", us: `$${u.pricing.startingPrice}/month`, them: c.pricing.startingPrice ? `$${c.pricing.startingPrice}/month` : "Custom" },
      { label: "Pricing model", us: u.pricing.pricingModel, them: c.pricing.pricingModel },
      { label: "Enterprise plan", us: "Custom", them: c.pricing.enterprisePlan ? "Custom" : "❌" },
    ]},
    { title: "Platforms", rows: [
      { label: "Total platforms", us: u.platforms.total, them: c.platforms.total },
      ...["x", "bluesky", "linkedin", "instagram", "threads", "tiktok", "youtube", "facebook", "pinterest"].map((p) => ({
        label: p === "x" ? "X / Twitter" : p.charAt(0).toUpperCase() + p.slice(1),
        us: u.platforms[p],
        them: c.platforms[p],
      })),
    ]},
    { title: "Features", rows: [
      { label: "Scheduled posts", us: u.features.scheduledPosts, them: c.features.scheduledPosts },
      { label: "Post analytics", us: u.features.postAnalytics, them: c.features.postAnalytics },
      { label: "Webhooks", us: u.features.webhooks, them: c.features.webhooks },
      { label: "Media upload", us: u.features.mediaUpload, them: c.features.mediaUpload },
      { label: "Twitter threads", us: u.features.twitterThreads, them: c.features.twitterThreads },
      { label: "Bulk publishing", us: u.features.bulkPublishing, them: c.features.bulkPublishing },
      { label: "MCP Server", us: u.features.mcpServer, them: c.features.mcpServer },
      { label: "First comment", us: u.features.firstComment, them: c.features.firstComment },
      { label: "White-label (BYOC)", us: u.features.nativeMode, them: c.features.nativeMode },
      { label: "Quickstart mode", us: u.features.quickstartMode, them: c.features.quickstartMode },
    ]},
    { title: "Developer Experience", rows: [
      { label: "REST API", us: u.developerExperience.restApi, them: c.developerExperience.restApi },
      { label: "SDK", us: u.developerExperience.sdk, them: c.developerExperience.sdk },
      { label: "Docs quality", us: "⭐".repeat(u.developerExperience.docsQuality as number), them: "⭐".repeat(c.developerExperience.docsQuality as number) },
      { label: "MCP Server", us: u.developerExperience.mcpServer, them: c.developerExperience.mcpServer },
      { label: "Open source", us: u.developerExperience.openSource, them: c.developerExperience.openSource },
    ]},
    { title: "Compliance", rows: [
      { label: "SOC 2", us: u.compliance.soc2, them: c.compliance.soc2 },
      { label: "GDPR", us: u.compliance.gdpr, them: c.compliance.gdpr },
    ]},
  ];
}

function renderVal(v: RowValue): React.ReactNode {
  if (v === true) return <span className="alt-chk"><CheckIcon /></span>;
  if (v === false) return <span className="alt-x">✕</span>;
  if (v === "coming") return <span className="alt-coming">Coming</span>;
  return <span className="alt-val">{String(v)}</span>;
}

// ── Schema.org JSON-LD ──
function buildSchema(comp: Competitor) {
  return {
    "@context": "https://schema.org",
    "@type": ["WebPage", "FAQPage"],
    name: `UniPost vs ${comp.name}`,
    description: comp.seo.description,
    mainEntity: comp.faqs.map((f) => ({
      "@type": "Question",
      name: f.q,
      acceptedAnswer: { "@type": "Answer", text: f.a },
    })),
  };
}

// ── Styles ──
const CSS = `:root{--alt-bg:var(--app-bg);--alt-s1:var(--marketing-surface);--alt-s2:var(--marketing-surface-alt);--alt-s3:var(--marketing-surface-elevated);--alt-border:var(--marketing-border);--alt-b2:var(--marketing-border-strong);--alt-b3:var(--marketing-border-strong);--alt-text:var(--marketing-text);--alt-muted:var(--marketing-muted);--alt-muted2:var(--marketing-subtle);--alt-accent:var(--primary);--alt-adim:var(--success-soft);--alt-blue:var(--marketing-link);--alt-r:8px;--alt-mono:var(--font-fira-code),monospace;--alt-ui:var(--font-dm-sans),system-ui,sans-serif;--alt-nav-max:1480px;--alt-content-max:1100px;--alt-px:32px;--alt-section-py:96px}*{box-sizing:border-box;margin:0;padding:0}body{background:var(--alt-bg);color:var(--alt-text);font-family:var(--alt-ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}

/* NAV */
.alt-nav{position:sticky;top:0;z-index:50;border-bottom:1px solid var(--marketing-nav-border);background:var(--marketing-nav-bg);backdrop-filter:blur(12px);-webkit-backdrop-filter:blur(12px)}.alt-nav-inner{max-width:var(--alt-nav-max);margin:0 auto;padding:0 var(--alt-px);height:56px;display:flex;align-items:center;justify-content:space-between}.alt-logo{display:flex;align-items:center;gap:10px;text-decoration:none}.alt-logo-mark{width:28px;height:28px;background:var(--alt-accent);border-radius:7px;display:flex;align-items:center;justify-content:center}.alt-logo-mark svg{width:14px;height:14px;color:var(--primary-foreground)}.alt-logo-name{font-size:16px;font-weight:700;letter-spacing:-.4px;color:var(--alt-text)}.alt-nav-links{display:flex;gap:4px}.alt-nav-link{padding:6px 14px;font-size:14px;font-weight:500;color:var(--alt-muted);border-radius:var(--alt-r);transition:color .1s;text-decoration:none}.alt-nav-link:hover{color:var(--alt-text)}

/* BUTTONS */
.alt-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--alt-r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--alt-ui);text-decoration:none;white-space:nowrap}.alt-btn-primary{background:var(--alt-blue);color:#fff}.alt-btn-primary:hover{background:var(--marketing-link-hover);box-shadow:0 0 24px color-mix(in srgb,var(--marketing-link) 20%,transparent)}.alt-btn-ghost{background:transparent;color:var(--alt-muted);border-color:var(--alt-b2)}.alt-btn-ghost:hover{background:var(--alt-s2);color:var(--alt-text);border-color:var(--alt-b3)}.alt-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}

/* PAGE */
.alt-page{max-width:var(--alt-content-max);margin:0 auto;padding:0 var(--alt-px)}

/* BREADCRUMB */
.alt-bread{padding:24px 0 0;font-size:13px;color:var(--alt-muted2)}.alt-bread a{color:var(--alt-muted);text-decoration:none}.alt-bread a:hover{color:var(--alt-text)}.alt-bread-sep{margin:0 8px}

/* HERO */
.alt-hero{padding:64px 0 var(--alt-section-py);text-align:center;display:flex;flex-direction:column;align-items:center}.alt-hero-title{font-size:56px;font-weight:900;letter-spacing:-2px;line-height:1.08;color:var(--alt-text);margin-bottom:24px;max-width:800px;white-space:pre-line}.alt-hero-sub{font-size:17px;color:var(--alt-muted);max-width:600px;line-height:1.75;margin-bottom:40px}.alt-hero-actions{display:flex;gap:12px;margin-bottom:28px}.alt-hero-meta{display:flex;gap:20px;font-size:13px;color:var(--alt-muted2)}.alt-hero-meta-item{display:flex;align-items:center;gap:6px}.alt-hero-meta-item svg{color:var(--alt-accent)}

/* VERDICT */
.alt-verdict{margin-bottom:var(--alt-section-py)}.alt-verdict-card{background:var(--alt-s1);border:1px solid var(--alt-b2);border-radius:16px;padding:40px 48px}.alt-verdict-title{font-size:22px;font-weight:800;letter-spacing:-.3px;margin-bottom:28px;color:var(--alt-text)}.alt-verdict-grid{display:grid;grid-template-columns:1fr 1fr;gap:48px}.alt-verdict-col-title{font-size:14px;font-weight:700;color:var(--alt-accent);margin-bottom:16px}.alt-verdict-col-title.them{color:var(--alt-muted)}.alt-verdict-item{display:flex;align-items:flex-start;gap:10px;font-size:14px;color:var(--alt-text);margin-bottom:12px;line-height:1.5}.alt-verdict-item svg{color:var(--alt-accent);flex-shrink:0;margin-top:2px}

/* COMPARISON TABLE */
.alt-table-section{margin-bottom:var(--alt-section-py)}.alt-table-title{font-size:36px;font-weight:800;letter-spacing:-.8px;margin-bottom:32px;text-align:center}.alt-table-wrap{border:1px solid var(--alt-border);border-radius:14px;overflow:hidden;margin-bottom:24px}.alt-table-hdr{display:grid;grid-template-columns:2.5fr 1fr 1fr;background:var(--alt-s2);border-bottom:1px solid var(--alt-border)}.alt-th{padding:14px 24px;font-size:12.5px;font-weight:600;color:var(--alt-muted);letter-spacing:.03em}.alt-th.hl{color:var(--alt-accent)}.alt-section-hdr{display:grid;grid-template-columns:1fr;background:var(--alt-s1);border-bottom:1px solid var(--alt-border);padding:12px 24px}.alt-section-label{font-size:11px;font-weight:700;text-transform:uppercase;letter-spacing:.1em;color:var(--alt-muted2)}.alt-row{display:grid;grid-template-columns:2.5fr 1fr 1fr;border-bottom:1px solid var(--alt-border);transition:background .1s}.alt-row:last-child{border-bottom:none}.alt-row:hover{background:var(--alt-s2)}.alt-cell{padding:14px 24px;display:flex;align-items:center;font-size:14px}.alt-cell-label{color:var(--alt-text);font-weight:500}.alt-chk{color:var(--alt-accent)}.alt-chk svg{width:15px;height:15px}.alt-x{color:var(--alt-muted2);font-size:16px}.alt-coming{font-size:12px;color:var(--alt-muted);font-family:var(--alt-mono);background:var(--alt-s2);padding:2px 8px;border-radius:4px;border:1px solid var(--alt-border)}.alt-val{font-size:13.5px;color:var(--alt-text)}

/* PRICING COMPARISON */
.alt-pricing{margin-bottom:var(--alt-section-py)}.alt-pricing-title{font-size:36px;font-weight:800;letter-spacing:-.8px;margin-bottom:32px;text-align:center}.alt-pricing-grid{display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-bottom:24px}.alt-pricing-card{background:var(--alt-s1);border:1px solid var(--alt-border);border-radius:14px;padding:32px;position:relative}.alt-pricing-card.ours{border-color:var(--alt-accent)}.alt-pricing-card-badge{position:absolute;top:-12px;left:50%;transform:translateX(-50%);background:var(--alt-accent);color:var(--primary-foreground);font-size:11px;font-weight:700;padding:3px 14px;border-radius:20px;font-family:var(--alt-mono);white-space:nowrap}.alt-pricing-card-name{font-size:18px;font-weight:700;margin-bottom:20px}.alt-pricing-tier{display:flex;justify-content:space-between;align-items:center;padding:10px 0;border-bottom:1px solid var(--alt-border);font-size:14px}.alt-pricing-tier:last-child{border-bottom:none}.alt-pricing-tier-name{color:var(--alt-text)}.alt-pricing-tier-price{font-family:var(--alt-mono);font-weight:600;color:var(--alt-text)}.alt-pricing-note{font-size:14px;color:var(--alt-muted);text-align:center;max-width:600px;margin:0 auto;line-height:1.7}

/* MCP SECTION */
.alt-mcp{margin-bottom:var(--alt-section-py)}.alt-mcp-grid{display:grid;grid-template-columns:1.2fr 1fr;gap:48px;align-items:start}.alt-mcp-code{background:var(--alt-s2);border:1px solid var(--alt-border);border-radius:10px;padding:24px 28px;font-family:var(--alt-mono);font-size:13px;line-height:1.7;color:var(--alt-text);white-space:pre;overflow-x:auto}.alt-mcp-code-label{font-size:12px;color:var(--alt-muted);font-family:var(--alt-mono);margin-bottom:8px}.alt-mcp-points{list-style:none}.alt-mcp-point{display:flex;align-items:flex-start;gap:10px;font-size:14.5px;color:var(--alt-text);margin-bottom:14px;line-height:1.5}.alt-mcp-point svg{color:var(--alt-accent);flex-shrink:0;margin-top:3px}.alt-mcp-note{font-size:14px;color:var(--alt-muted);margin-top:20px;padding-top:20px;border-top:1px solid var(--alt-border)}

/* MIGRATION */
.alt-migrate{margin-bottom:var(--alt-section-py)}.alt-migrate-steps{display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-bottom:32px}.alt-migrate-step{background:var(--alt-s1);border:1px solid var(--alt-border);border-radius:12px;padding:28px 32px}.alt-migrate-step-num{font-family:var(--alt-mono);font-size:12px;color:var(--alt-accent);font-weight:600;margin-bottom:10px;letter-spacing:.05em}.alt-migrate-step-title{font-size:16px;font-weight:700;margin-bottom:8px}.alt-migrate-step-desc{font-size:13.5px;color:var(--alt-muted);line-height:1.6}.alt-migrate-code{font-family:var(--alt-mono);font-size:12.5px;background:var(--alt-s2);padding:3px 8px;border-radius:4px;color:var(--alt-text);margin-top:8px;display:inline-block}.alt-migrate-bottom{text-align:center}.alt-migrate-quote{font-size:15px;color:var(--alt-muted);font-style:italic;margin-bottom:20px}

/* FAQ */
.alt-faq{margin-bottom:var(--alt-section-py)}.alt-faq-title{font-size:36px;font-weight:800;letter-spacing:-.8px;margin-bottom:32px;text-align:center}.alt-faq-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.alt-faq-item{background:var(--alt-s1);border:1px solid var(--alt-border);border-radius:10px;padding:24px 26px;transition:border-color .15s}.alt-faq-item:hover{border-color:var(--alt-b2)}.alt-faq-q{font-size:15px;font-weight:600;margin-bottom:10px}.alt-faq-a{font-size:13.5px;color:var(--alt-muted);line-height:1.7}

/* CTA BANNER */
.alt-cta{background:var(--app-bg);border-top:1px solid var(--alt-border);padding:var(--alt-section-py) 0;margin-bottom:0}.alt-cta-inner{max-width:var(--alt-content-max);margin:0 auto;padding:0 var(--alt-px)}.alt-cta-card{background:var(--alt-s1);border:1px solid var(--alt-border);border-radius:16px;padding:72px 48px;text-align:center;position:relative;overflow:hidden;box-shadow:var(--marketing-shadow-soft)}.alt-cta-glow{position:absolute;width:400px;height:400px;border-radius:50%;background:var(--marketing-glow);top:50%;left:50%;transform:translate(-50%,-50%);pointer-events:none}.alt-cta-title{font-size:44px;font-weight:900;letter-spacing:-1.2px;margin-bottom:16px;position:relative}.alt-cta-sub{font-size:15px;color:var(--alt-muted);margin-bottom:36px;position:relative}.alt-cta-actions{display:flex;justify-content:center;gap:12px;position:relative}

/* FOOTER */
.alt-footer{width:100%;border-top:1px solid var(--alt-border);padding:48px 0}.alt-footer-inner{max-width:var(--alt-content-max);margin:0 auto;padding:0 var(--alt-px)}.alt-footer-top{display:grid;grid-template-columns:2fr 1fr 1fr 1fr 1fr;gap:48px;margin-bottom:48px}.alt-footer-logo{display:flex;align-items:center;gap:9px;margin-bottom:16px}.alt-footer-mark{width:26px;height:26px;background:var(--alt-accent);border-radius:6px;display:flex;align-items:center;justify-content:center}.alt-footer-mark svg{width:13px;height:13px;color:var(--primary-foreground)}.alt-footer-name{font-size:15px;font-weight:700;color:var(--alt-text)}.alt-footer-tagline{font-size:13px;color:var(--alt-muted);line-height:1.65;max-width:260px}.alt-footer-col-title{font-size:12px;font-weight:700;text-transform:uppercase;letter-spacing:.08em;color:var(--alt-muted2);margin-bottom:16px}.alt-footer-links{list-style:none}.alt-footer-link{font-size:13.5px;color:var(--alt-muted);margin-bottom:10px;cursor:pointer;transition:color .1s;display:block;text-decoration:none}.alt-footer-link:hover{color:var(--alt-text)}.alt-footer-bottom{border-top:1px solid var(--alt-border);padding-top:24px;display:flex;align-items:center;justify-content:space-between}.alt-footer-copy{font-size:13px;color:var(--alt-muted2)}.alt-footer-social{display:flex;gap:12px}.alt-footer-social-link{width:32px;height:32px;background:var(--alt-s2);border:1px solid var(--alt-border);border-radius:6px;display:flex;align-items:center;justify-content:center;color:var(--alt-muted);cursor:pointer;transition:all .15s;font-size:14px;text-decoration:none}.alt-footer-social-link:hover{background:var(--alt-s3);color:var(--alt-text);border-color:var(--alt-b2)}

/* SECTION TITLES */
.alt-section-eyebrow{font-size:11.5px;color:var(--alt-accent);text-transform:uppercase;letter-spacing:.12em;font-weight:700;margin-bottom:12px;font-family:var(--alt-mono);text-align:center}

/* LP-BTN COMPAT (MarketingNav/CTA use lp- classes) */
.lp-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--alt-r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--alt-ui);text-decoration:none;white-space:nowrap}.lp-btn-primary{background:var(--alt-blue);color:#fff}.lp-btn-primary:hover{background:var(--marketing-link-hover);box-shadow:0 0 24px color-mix(in srgb,var(--marketing-link) 20%,transparent)}.lp-btn-ghost{background:transparent;color:var(--alt-muted);border-color:var(--alt-b2)}.lp-btn-ghost:hover{background:var(--alt-s2);color:var(--alt-text);border-color:var(--alt-b3)}.lp-btn-outline{background:transparent;color:var(--alt-text);border-color:var(--alt-b2)}.lp-btn-outline:hover{background:var(--alt-s2);border-color:var(--alt-b3)}.lp-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}

/* RESPONSIVE */
@media(min-width:1600px){:root{--alt-content-max:1100px;--alt-px:40px}}
@media(max-width:1024px){:root{--alt-px:24px;--alt-section-py:72px}}
@media(max-width:768px){.alt-hero-title{font-size:36px}.alt-verdict-grid{grid-template-columns:1fr}.alt-table-hdr,.alt-row{grid-template-columns:2fr 1fr 1fr}.alt-pricing-grid{grid-template-columns:1fr}.alt-mcp-grid{grid-template-columns:1fr}.alt-migrate-steps{grid-template-columns:1fr}.alt-faq-grid{grid-template-columns:1fr}.alt-footer-top{grid-template-columns:1fr 1fr;gap:32px}.alt-footer-bottom{flex-direction:column;gap:12px;text-align:center}}
`;

const MCP_CONFIG = `// Claude Desktop config
{
  "mcpServers": {
    "unipost": {
      "url": "https://mcp.unipost.dev/sse",
      "headers": {
        "Authorization": "Bearer up_live_xxx"
      }
    }
  }
}`;

export default function AlternativePage() {
  const params = useParams();
  const slug = params.competitor as string;
  const comp = getCompetitorBySlug(slug);

  if (!comp) {
    return (
      <>
        <style dangerouslySetInnerHTML={{ __html: CSS }} />
        <div className="alt-page" style={{ padding: "200px 0", textAlign: "center" }}>
          <h1 style={{ fontSize: 36, fontWeight: 800, marginBottom: 16 }}>Competitor not found</h1>
          <p style={{ color: "var(--marketing-muted)", marginBottom: 32 }}>We don&apos;t have a comparison page for &ldquo;{slug}&rdquo; yet.</p>
          <Link href="/compare" className="alt-btn alt-btn-primary alt-btn-lg">View all comparisons →</Link>
        </div>
      </>
    );
  }

  const sections = buildSections(comp);
  const schema = buildSchema(comp);

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(schema) }} />

      {/* NAV */}
      <nav className="alt-nav">
        <div className="alt-nav-inner">
          <Link href="/" className="alt-logo">
            <UniPostLogo markSize={28} wordmarkColor="var(--alt-text)" />
          </Link>
          <div className="alt-nav-links">
            <Link href="/docs" className="alt-nav-link">Docs</Link>
            <Link href="/pricing" className="alt-nav-link">Pricing</Link>
            <Link href="/compare" className="alt-nav-link">Compare</Link>
          </div>
          <MarketingNav />
        </div>
      </nav>

      <div className="alt-page">
        {/* BREADCRUMB */}
        <div className="alt-bread">
          <Link href="/">UniPost</Link>
          <span className="alt-bread-sep">&gt;</span>
          <Link href="/compare">Alternatives</Link>
          <span className="alt-bread-sep">&gt;</span>
          <span>{comp.name} Alternative</span>
        </div>

        {/* HERO */}
        <div className="alt-hero">
          <h1 className="alt-hero-title">{comp.heroTitle}</h1>
          <p className="alt-hero-sub">{comp.heroSub}</p>
          <div className="alt-hero-actions">
            <MarketingCTA />
            <a href="#comparison" className="alt-btn alt-btn-ghost alt-btn-lg">See full comparison ↓</a>
          </div>
          <div className="alt-hero-meta">
            <div className="alt-hero-meta-item"><CheckIcon /><span>Free 100 posts/month</span></div>
            <div className="alt-hero-meta-item"><CheckIcon /><span>No credit card</span></div>
            <div className="alt-hero-meta-item"><CheckIcon /><span>9 platforms</span></div>
          </div>
        </div>

        {/* QUICK VERDICT */}
        <div className="alt-verdict">
          <div className="alt-verdict-card">
            <div className="alt-verdict-title">TL;DR — Which should you choose?</div>
            <div className="alt-verdict-grid">
              <div>
                <div className="alt-verdict-col-title">Choose UniPost if:</div>
                {comp.verdict.chooseUs.map((item) => (
                  <div key={item} className="alt-verdict-item"><ArrowIcon />{item}</div>
                ))}
              </div>
              <div>
                <div className="alt-verdict-col-title them">Choose {comp.name} if:</div>
                {comp.verdict.chooseThem.map((item) => (
                  <div key={item} className="alt-verdict-item"><ArrowIcon />{item}</div>
                ))}
              </div>
            </div>
          </div>
        </div>

        {/* COMPARISON TABLE */}
        <div className="alt-table-section" id="comparison">
          <h2 className="alt-table-title">UniPost vs {comp.name} — Full Comparison</h2>
          <div className="alt-table-wrap">
            <div className="alt-table-hdr">
              <div className="alt-th">Feature</div>
              <div className="alt-th hl">UniPost</div>
              <div className="alt-th">{comp.name}</div>
            </div>
            {sections.map((section) => (
              <div key={section.title}>
                <div className="alt-section-hdr">
                  <div className="alt-section-label">{section.title}</div>
                </div>
                {section.rows.map((row) => (
                  <div key={row.label} className="alt-row">
                    <div className="alt-cell alt-cell-label">{row.label}</div>
                    <div className="alt-cell">{renderVal(row.us)}</div>
                    <div className="alt-cell">{renderVal(row.them)}</div>
                  </div>
                ))}
              </div>
            ))}
          </div>
        </div>

        {/* PRICING COMPARISON */}
        <div className="alt-pricing">
          <h2 className="alt-pricing-title">Pricing comparison</h2>
          <div className="alt-pricing-grid">
            <div className="alt-pricing-card ours">
              <div className="alt-pricing-card-badge">Free tier available</div>
              <div className="alt-pricing-card-name">UniPost</div>
              {UNIPOST.pricing.tiers.map((t) => (
                <div key={t.label} className="alt-pricing-tier">
                  <span className="alt-pricing-tier-name">{t.label}</span>
                  <span className="alt-pricing-tier-price">{t.price !== null ? `$${t.price}/mo` : "Contact us"}</span>
                </div>
              ))}
            </div>
            <div className="alt-pricing-card">
              <div className="alt-pricing-card-name">{comp.name}</div>
              {comp.pricing.tiers.map((t) => (
                <div key={t.label} className="alt-pricing-tier">
                  <span className="alt-pricing-tier-name">{t.label}</span>
                  <span className="alt-pricing-tier-price">{t.price !== null ? `$${t.price}/mo` : "Contact us"}</span>
                </div>
              ))}
            </div>
          </div>
          <p className="alt-pricing-note">
            UniPost&apos;s pricing is product-tier based. Free includes the API and dashboard. Basic ($19/mo) bundles Inbox, Analytics, and one shared custom platform for Hosted Connect plus Platform Credentials. Growth ($59/mo) expands that to all supported platforms with optional attribution removal. Team ($149/mo) adds RBAC and team collaboration.
          </p>
        </div>

        {/* MCP SECTION */}
        <div className="alt-mcp">
          <div className="alt-section-eyebrow">{comp.features.mcpServer ? "MCP Comparison" : "UniPost Exclusive"}</div>
          <h2 className="alt-table-title">Built for the AI agent era</h2>
          <p className="alt-hero-sub" style={{ textAlign: "center", margin: "0 auto 48px" }}>
            {comp.features.mcpServer
              ? `Both UniPost and ${comp.name} ship a native MCP server so Claude, GPT, or any MCP-compatible agent can post on behalf of your users — no custom integration code.`
              : `UniPost ships a native MCP server so Claude, GPT, or any MCP-compatible agent can post on behalf of your users — no custom integration code.`}
          </p>
          <div className="alt-mcp-grid">
            <div>
              <div className="alt-mcp-code-label">{"// Claude Desktop config"}</div>
              <div className="alt-mcp-code">{MCP_CONFIG}</div>
            </div>
            <div>
              <div style={{ fontSize: 18, fontWeight: 700, marginBottom: 20 }}>What this means:</div>
              <ul className="alt-mcp-points">
                <li className="alt-mcp-point"><ArrowIcon />AI agents can list connected accounts</li>
                <li className="alt-mcp-point"><ArrowIcon />AI agents can publish posts</li>
                <li className="alt-mcp-point"><ArrowIcon />AI agents can read analytics</li>
                <li className="alt-mcp-point"><ArrowIcon />No custom integration code needed</li>
              </ul>
              <div className="alt-mcp-note">
                {comp.features.mcpServer
                  ? `${comp.name} also ships an MCP server. Compare the rest of the stack — pricing, free tier, bundled features — to decide which one fits.`
                  : `${comp.name} has no MCP Server support.`
                }
              </div>
            </div>
          </div>
        </div>

        {/* MIGRATION GUIDE */}
        <div className="alt-migrate">
          <div className="alt-section-eyebrow">Easy Switch</div>
          <h2 className="alt-table-title">Switching from {comp.name} to UniPost</h2>
          <div className="alt-migrate-steps">
            <div className="alt-migrate-step">
              <div className="alt-migrate-step-num">STEP 1</div>
              <div className="alt-migrate-step-title">Sign up for UniPost</div>
              <div className="alt-migrate-step-desc">Takes 2 minutes, free plan available</div>
            </div>
            <div className="alt-migrate-step">
              <div className="alt-migrate-step-num">STEP 2</div>
              <div className="alt-migrate-step-title">Connect your social accounts</div>
              <div className="alt-migrate-step-desc">Same platforms, same OAuth flow</div>
            </div>
            <div className="alt-migrate-step">
              <div className="alt-migrate-step-num">STEP 3</div>
              <div className="alt-migrate-step-title">Update your API calls</div>
              <div className="alt-migrate-step-desc">
                One endpoint change:
                <br /><code className="alt-migrate-code">FROM: {comp.migrationEndpoint.from}</code>
                <br /><code className="alt-migrate-code">TO: {comp.migrationEndpoint.to}</code>
              </div>
            </div>
            <div className="alt-migrate-step">
              <div className="alt-migrate-step-num">STEP 4</div>
              <div className="alt-migrate-step-title">Update field names</div>
              <div className="alt-migrate-step-desc">
                <code className="alt-migrate-code">FROM: {comp.migrationFields.from}</code>
                <br /><code className="alt-migrate-code">TO: {comp.migrationFields.to}</code>
              </div>
            </div>
          </div>
          <div className="alt-migrate-bottom">
            <p className="alt-migrate-quote">&ldquo;Most integrations can be migrated in under an hour.&rdquo;</p>
            <MarketingCTA />
          </div>
        </div>

        {/* FAQ */}
        <div className="alt-faq">
          <h2 className="alt-faq-title">Frequently asked questions</h2>
          <div className="alt-faq-grid">
            {comp.faqs.map((f) => (
              <div key={f.q} className="alt-faq-item">
                <div className="alt-faq-q">{f.q}</div>
                <div className="alt-faq-a">{f.a}</div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* CTA BANNER */}
      <div className="alt-cta">
        <div className="alt-cta-inner">
          <div className="alt-cta-card">
            <div className="alt-cta-glow" />
            <h2 className="alt-cta-title">Start for free. No {comp.name} contract needed.</h2>
            <p className="alt-cta-sub">Free plan · 100 posts/month · No credit card</p>
            <div className="alt-cta-actions">
              <MarketingCTA />
              <Link href="/pricing" className="alt-btn alt-btn-ghost alt-btn-lg">View Pricing →</Link>
            </div>
          </div>
        </div>
      </div>

    </>
  );
}
