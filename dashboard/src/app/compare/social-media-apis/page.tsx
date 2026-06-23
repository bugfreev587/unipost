import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, CheckCircle2, Info, Scale } from "lucide-react";
import { MarketingCTA, PublicSiteHeader } from "@/components/marketing/nav";
import { ALL_COMPETITORS } from "@/data/competitors";
import { UNIPOST } from "@/data/competitors/unipost";

export const metadata: Metadata = {
  title: "Best Unified Social Media APIs for Developers | UniPost",
  description:
    "Compare unified social media APIs for posting, scheduling, webhooks, platform coverage, pricing model, white-label support, and AI-agent workflows.",
  alternates: {
    canonical: "https://unipost.dev/compare/social-media-apis",
  },
  openGraph: {
    title: "Best Unified Social Media APIs for Developers",
    description:
      "A UniPost-authored buying guide for developers comparing social media APIs.",
    url: "https://unipost.dev/compare/social-media-apis",
    siteName: "UniPost",
    type: "website",
  },
};

const CSS = `
:root{--sca-bg:var(--app-bg);--sca-s1:var(--marketing-surface);--sca-s2:var(--marketing-surface-alt);--sca-s3:var(--marketing-surface-elevated);--sca-border:var(--marketing-border);--sca-b2:var(--marketing-border-strong);--sca-text:var(--marketing-text);--sca-muted:var(--marketing-muted);--sca-subtle:var(--marketing-subtle);--sca-link:var(--marketing-link);--sca-success:var(--primary);--sca-mono:var(--font-fira-code),monospace;--sca-ui:var(--font-dm-sans),system-ui,sans-serif;--sca-content:1180px;--sca-pad:32px}
*{box-sizing:border-box}
body{background:var(--sca-bg);color:var(--sca-text);font-family:var(--sca-ui);line-height:1.6;-webkit-font-smoothing:antialiased}
.sca-shell{background:linear-gradient(180deg,color-mix(in srgb,var(--sca-s2) 58%,var(--sca-bg)),var(--sca-bg) 520px)}
.sca-main{width:100%}
.sca-section{padding:82px var(--sca-pad)}
.sca-section.tight{padding-top:48px;padding-bottom:48px}
.sca-inner{max-width:var(--sca-content);margin:0 auto}
.sca-hero{padding:88px var(--sca-pad) 56px;text-align:center;max-width:940px;margin:0 auto}
.sca-kicker{display:inline-flex;align-items:center;gap:8px;color:var(--sca-success);font-family:var(--sca-mono);font-size:12px;font-weight:800;text-transform:uppercase;letter-spacing:0;margin-bottom:18px}
.sca-title{font-size:56px;line-height:1.05;letter-spacing:0;margin:0 0 20px;font-weight:900}
.sca-sub{font-size:18px;line-height:1.75;color:var(--sca-muted);margin:0 auto 30px;max-width:760px}
.sca-actions{display:flex;justify-content:center;gap:12px;flex-wrap:wrap}
.sca-btn{display:inline-flex;align-items:center;justify-content:center;gap:8px;min-height:46px;padding:11px 18px;border:1px solid var(--sca-b2);border-radius:8px;color:var(--sca-text);background:transparent;text-decoration:none;font-weight:700;font-size:14px}
.sca-btn:hover{border-color:var(--sca-link);background:var(--sca-s2)}
.sca-note{display:flex;gap:10px;align-items:flex-start;border:1px solid var(--sca-border);background:var(--sca-s1);border-radius:8px;padding:16px;color:var(--sca-muted);font-size:14px;line-height:1.7;margin-top:30px;text-align:left}
.sca-note svg{width:18px;height:18px;color:var(--sca-success);margin-top:2px;flex:0 0 auto}
.sca-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px}
.sca-card{border:1px solid var(--sca-border);background:var(--sca-s1);border-radius:8px;padding:22px;min-height:270px}
.sca-card.featured{border-color:color-mix(in srgb,var(--sca-success) 40%,var(--sca-border));background:var(--sca-s3)}
.sca-card h2{font-size:20px;margin:0 0 8px}
.sca-card p{margin:0 0 16px;color:var(--sca-muted);font-size:14px;line-height:1.7}
.sca-meta{display:grid;gap:8px;margin:0 0 18px}
.sca-meta-row{display:flex;justify-content:space-between;gap:12px;border-bottom:1px solid var(--sca-border);padding-bottom:8px;font-size:13px;color:var(--sca-muted)}
.sca-meta-row strong{color:var(--sca-text);font-weight:800;text-align:right}
.sca-list{display:grid;gap:9px;margin:0;padding:0;list-style:none}
.sca-list li{display:flex;gap:8px;color:var(--sca-muted);font-size:13.5px;line-height:1.55}
.sca-list svg{width:15px;height:15px;color:var(--sca-success);flex:0 0 auto;margin-top:3px}
.sca-table{border:1px solid var(--sca-border);border-radius:8px;overflow:auto;background:var(--sca-s1)}
.sca-row{display:grid;grid-template-columns:1.4fr repeat(4,minmax(150px,1fr));min-width:860px;border-bottom:1px solid var(--sca-border)}
.sca-row:last-child{border-bottom:0}
.sca-cell{padding:13px 16px;color:var(--sca-muted);font-size:14px}
.sca-head .sca-cell{font-size:12px;font-weight:800;text-transform:uppercase;letter-spacing:0;color:var(--sca-subtle);background:var(--sca-s2)}
.sca-cell strong{color:var(--sca-text)}
.sca-band{border-top:1px solid var(--sca-border);border-bottom:1px solid var(--sca-border);background:var(--sca-s2)}
.sca-two{display:grid;grid-template-columns:1fr 1fr;gap:18px}
.sca-panel{border:1px solid var(--sca-border);background:var(--sca-s1);border-radius:8px;padding:24px}
.sca-panel h2{font-size:24px;line-height:1.2;margin:0 0 12px}
.sca-panel p{margin:0;color:var(--sca-muted);line-height:1.7}
.sca-link-card{display:flex;align-items:center;justify-content:space-between;gap:12px;border:1px solid var(--sca-border);background:var(--sca-s1);border-radius:8px;padding:16px;color:var(--sca-text);text-decoration:none;font-weight:800}
.sca-link-card:hover{border-color:var(--sca-link);background:var(--sca-s3)}
.sca-links{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}
@media(max-width:1040px){.sca-grid{grid-template-columns:1fr 1fr}.sca-two,.sca-links{grid-template-columns:1fr}}
@media(max-width:720px){:root{--sca-pad:20px}.sca-title{font-size:38px}.sca-grid{grid-template-columns:1fr}.sca-actions .lp-btn,.sca-actions .sca-btn{width:100%;justify-content:center}}
`;

function formatFreeTier(pricing: { freeTier: boolean; freePostsPerMonth: number | string; freeTierLabel?: string }) {
  if (!pricing.freeTier) return "No free tier";
  if (pricing.freeTierLabel) return pricing.freeTierLabel;
  return `${pricing.freePostsPerMonth}/mo`;
}

const rows = [
  ["Pricing model", UNIPOST.pricing.pricingModel, ...ALL_COMPETITORS.map((c) => c.pricing.pricingModel)],
  ["Free tier", "100 posts/mo", ...ALL_COMPETITORS.map((c) => formatFreeTier(c.pricing))],
  ["Supported platforms", String(UNIPOST.platforms.total), ...ALL_COMPETITORS.map((c) => String(c.platforms.total))],
  ["White-label path", "Growth native mode", ...ALL_COMPETITORS.map((c) => (c.features.nativeMode ? "Available" : "Not public"))],
  ["MCP / AI agents", "Native MCP", ...ALL_COMPETITORS.map((c) => (c.features.mcpServer ? "Supported" : "Not public"))],
  ["Best fit", "Embedded SaaS and AI workflows", ...ALL_COMPETITORS.map((c) => c.bestFit.competitor)],
];

export default function SocialMediaApisComparisonPage() {
  const schema = [
    {
      "@context": "https://schema.org",
      "@type": "ItemList",
      name: "Best Unified Social Media APIs",
      itemListElement: [
        { "@type": "ListItem", position: 1, name: "UniPost", url: "https://unipost.dev" },
        ...ALL_COMPETITORS.map((competitor, index) => ({
          "@type": "ListItem",
          position: index + 2,
          name: competitor.name,
          url: `https://unipost.dev/alternatives/${competitor.slug}`,
        })),
      ],
    },
    {
      "@context": "https://schema.org",
      "@type": "FAQPage",
      mainEntity: [
        {
          "@type": "Question",
          name: "What is the best unified social media API?",
          acceptedAnswer: {
            "@type": "Answer",
            text: "The best API depends on platform coverage, pricing model, white-label requirements, AI-agent support, and whether you prefer managed infrastructure or self-hosting.",
          },
        },
        {
          "@type": "Question",
          name: "Is this comparison neutral?",
          acceptedAnswer: {
            "@type": "Answer",
            text: "No. This is a UniPost-authored buying guide that explains when UniPost is a good fit and when another vendor may be better.",
          },
        },
      ],
    },
  ];

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(schema) }} />
      <div className="sca-shell">
        <PublicSiteHeader active="developer" />
        <main className="sca-main">
          <section className="sca-hero">
            <div className="sca-kicker">
              <Scale aria-hidden="true" />
              Buying guide
            </div>
            <h1 className="sca-title">Best unified social media APIs for developers</h1>
            <p className="sca-sub">
              A practical comparison of social media APIs for teams evaluating posting,
              scheduling, webhooks, platform coverage, pricing, white-label account connection,
              and AI-agent workflows.
            </p>
            <div className="sca-actions">
              <MarketingCTA />
              <Link href="/compare" className="sca-btn">
                All comparisons
                <ArrowRight aria-hidden="true" />
              </Link>
            </div>
            <div className="sca-note">
              <Info aria-hidden="true" />
              <span>
                This guide is written by UniPost. It is not pretending to be a neutral third-party
                ranking. Use it to understand fit, tradeoffs, and what to verify before choosing a vendor.
              </span>
            </div>
          </section>

          <section className="sca-section tight">
            <div className="sca-inner sca-grid">
              <article className="sca-card featured">
                <h2>UniPost</h2>
                <p>{UNIPOST.tagline}</p>
                <div className="sca-meta">
                  <div className="sca-meta-row"><span>Pricing</span><strong>{UNIPOST.pricing.pricingModel}</strong></div>
                  <div className="sca-meta-row"><span>Platforms</span><strong>{UNIPOST.platforms.total}</strong></div>
                  <div className="sca-meta-row"><span>MCP</span><strong>Native</strong></div>
                </div>
                <ul className="sca-list">
                  <li><CheckCircle2 aria-hidden="true" /><span>Best for embedded SaaS, AI agents, and predictable self-serve pricing.</span></li>
                  <li><CheckCircle2 aria-hidden="true" /><span>Choose another vendor if you need a network UniPost does not support yet.</span></li>
                </ul>
              </article>
              {ALL_COMPETITORS.map((competitor) => (
                <article key={competitor.slug} className="sca-card">
                  <h2>{competitor.name}</h2>
                  <p>{competitor.tagline}</p>
                  <div className="sca-meta">
                    <div className="sca-meta-row"><span>Pricing</span><strong>{competitor.pricing.pricingModel}</strong></div>
                    <div className="sca-meta-row"><span>Platforms</span><strong>{competitor.platforms.total}</strong></div>
                    <div className="sca-meta-row"><span>Best fit</span><strong>{competitor.bestFit.competitor}</strong></div>
                  </div>
                  <Link href={`/alternatives/${competitor.slug}`} className="sca-link-card">
                    <span>Compare with UniPost</span>
                    <ArrowRight aria-hidden="true" />
                  </Link>
                </article>
              ))}
            </div>
          </section>

          <section className="sca-section sca-band">
            <div className="sca-inner">
              <div className="sca-table">
                <div className="sca-row sca-head">
                  <div className="sca-cell">Criteria</div>
                  <div className="sca-cell">UniPost</div>
                  {ALL_COMPETITORS.map((competitor) => (
                    <div key={competitor.slug} className="sca-cell">{competitor.name}</div>
                  ))}
                </div>
                {rows.map((row) => (
                  <div key={row[0]} className="sca-row">
                    {row.map((cell, index) => (
                      <div key={`${row[0]}-${index}`} className="sca-cell">
                        {index === 0 ? <strong>{cell}</strong> : cell}
                      </div>
                    ))}
                  </div>
                ))}
              </div>
            </div>
          </section>

          <section className="sca-section">
            <div className="sca-inner sca-two">
              <div className="sca-panel">
                <h2>How to choose</h2>
                <p>
                  Start with the workflow you are embedding: account connection, media upload,
                  scheduled publishing, status tracking, webhooks, analytics, and white-label
                  requirements. Then compare pricing against the number of customers, profiles,
                  connected accounts, and monthly posts you expect.
                </p>
              </div>
              <div className="sca-panel">
                <h2>What to verify</h2>
                <p>
                  Verify platform coverage, native app review requirements, webhook detail,
                  media constraints, pricing meters, support response time, and whether the
                  provider exposes enough errors for your product and support team.
                </p>
              </div>
            </div>
          </section>

          <section className="sca-section sca-band">
            <div className="sca-inner sca-links">
              <Link href="/social-media-api" className="sca-link-card">
                <span>Unified API page</span>
                <ArrowRight aria-hidden="true" />
              </Link>
              <Link href="/resources/unified-api-cost-calculator" className="sca-link-card">
                <span>Cost calculator</span>
                <ArrowRight aria-hidden="true" />
              </Link>
              <Link href="/resources/social-media-api-platform-requirements" className="sca-link-card">
                <span>Platform matrix</span>
                <ArrowRight aria-hidden="true" />
              </Link>
            </div>
          </section>
        </main>
      </div>
    </>
  );
}
