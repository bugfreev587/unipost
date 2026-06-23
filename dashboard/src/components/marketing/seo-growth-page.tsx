import Link from "next/link";
import {
  ArrowRight,
  CheckCircle2,
  Code2,
  FileText,
  Link2,
  ShieldCheck,
  Workflow,
} from "lucide-react";
import { MarketingCTA, PublicSiteHeader } from "@/components/marketing/nav";
import type { SeoGrowthPage } from "@/data/seo-growth-pages";

const CSS = `
:root{
  --sg-bg:var(--app-bg);
  --sg-surface:var(--marketing-surface);
  --sg-surface-alt:var(--marketing-surface-alt);
  --sg-surface-elevated:var(--marketing-surface-elevated);
  --sg-border:var(--marketing-border);
  --sg-border-strong:var(--marketing-border-strong);
  --sg-text:var(--marketing-text);
  --sg-muted:var(--marketing-muted);
  --sg-subtle:var(--marketing-subtle);
  --sg-link:var(--marketing-link);
  --sg-link-hover:var(--marketing-link-hover);
  --sg-success:var(--primary);
  --sg-mono:var(--font-fira-code), ui-monospace, SFMono-Regular, Menlo, monospace;
  --sg-ui:var(--font-dm-sans), system-ui, sans-serif;
  --sg-content:1180px;
  --sg-pad:32px;
}
*{box-sizing:border-box}
body{background:var(--sg-bg);color:var(--sg-text);font-family:var(--sg-ui);line-height:1.6;-webkit-font-smoothing:antialiased}
.sg-shell{background:linear-gradient(180deg,color-mix(in srgb,var(--sg-surface-alt) 55%,var(--sg-bg)),var(--sg-bg) 520px)}
.sg-main{width:100%}
.sg-section{padding:88px var(--sg-pad)}
.sg-section.tight{padding-top:54px;padding-bottom:54px}
.sg-inner{max-width:var(--sg-content);margin:0 auto}
.sg-hero{display:grid;grid-template-columns:minmax(0,1.05fr) minmax(360px,.95fr);gap:48px;align-items:center;padding:88px var(--sg-pad) 64px}
.sg-kicker{display:inline-flex;align-items:center;gap:8px;color:var(--sg-success);font-family:var(--sg-mono);font-size:12px;font-weight:700;text-transform:uppercase;letter-spacing:0;margin-bottom:18px}
.sg-title{font-size:58px;line-height:1.04;letter-spacing:0;font-weight:900;margin:0 0 20px;color:var(--sg-text);max-width:760px}
.sg-summary{font-size:18px;line-height:1.75;color:var(--sg-muted);max-width:680px;margin:0 0 30px}
.sg-actions{display:flex;flex-wrap:wrap;gap:12px;align-items:center}
.sg-btn{display:inline-flex;align-items:center;justify-content:center;gap:8px;min-height:46px;padding:11px 18px;border:1px solid var(--sg-border-strong);border-radius:8px;color:var(--sg-text);background:transparent;text-decoration:none;font-weight:700;font-size:14px;white-space:nowrap}
.sg-btn:hover{background:var(--sg-surface-alt);border-color:var(--sg-link);color:var(--sg-text)}
.sg-query{display:inline-flex;align-items:center;border:1px solid var(--sg-border);background:var(--sg-surface);border-radius:8px;padding:10px 12px;color:var(--sg-muted);font-size:13px;margin-top:24px}
.sg-query strong{color:var(--sg-text);font-weight:700;margin-left:6px}
.sg-code{border:1px solid var(--sg-border-strong);background:#07111f;border-radius:8px;overflow:hidden;box-shadow:var(--marketing-shadow-soft)}
.sg-code-head{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:12px 16px;background:#0b1728;border-bottom:1px solid rgba(255,255,255,.08);color:#dbeafe;font-size:13px}
.sg-code-head span{display:inline-flex;align-items:center;gap:8px;font-family:var(--sg-mono)}
.sg-code pre{margin:0;padding:22px 24px;overflow:auto;color:#dbeafe;font-family:var(--sg-mono);font-size:13px;line-height:1.75}
.sg-proof{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;margin-top:36px}
.sg-proof-item{display:flex;align-items:flex-start;gap:10px;border:1px solid var(--sg-border);background:var(--sg-surface);border-radius:8px;padding:14px;color:var(--sg-text);font-size:14px}
.sg-proof-item svg{width:18px;height:18px;color:var(--sg-success);flex:0 0 auto;margin-top:2px}
.sg-heading{max-width:760px;margin-bottom:34px}
.sg-eyebrow{font-family:var(--sg-mono);font-size:12px;font-weight:700;text-transform:uppercase;letter-spacing:0;color:var(--sg-success);margin-bottom:10px}
.sg-h2{font-size:34px;line-height:1.14;letter-spacing:0;margin:0 0 12px;color:var(--sg-text)}
.sg-lead{font-size:16px;line-height:1.75;color:var(--sg-muted);margin:0}
.sg-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:18px}
.sg-card{border:1px solid var(--sg-border);background:var(--sg-surface);border-radius:8px;padding:24px;min-height:220px}
.sg-card-icon{display:inline-flex;align-items:center;justify-content:center;width:36px;height:36px;border:1px solid color-mix(in srgb,var(--sg-success) 22%,transparent);background:color-mix(in srgb,var(--sg-success) 12%,transparent);border-radius:8px;color:var(--sg-success);margin-bottom:16px}
.sg-card-icon svg{width:19px;height:19px}
.sg-card h3{font-size:18px;line-height:1.25;margin:0 0 10px;color:var(--sg-text)}
.sg-card p{margin:0 0 16px;color:var(--sg-muted);font-size:14px;line-height:1.7}
.sg-list{display:grid;gap:10px;margin:0;padding:0;list-style:none}
.sg-list li{display:flex;gap:9px;color:var(--sg-muted);font-size:14px;line-height:1.55}
.sg-list li svg{width:15px;height:15px;color:var(--sg-success);flex:0 0 auto;margin-top:3px}
.sg-workflow{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:12px}
.sg-step{border:1px solid var(--sg-border);background:var(--sg-surface-alt);border-radius:8px;padding:18px}
.sg-step-num{font-family:var(--sg-mono);font-size:12px;color:var(--sg-success);font-weight:700;margin-bottom:10px}
.sg-step p{margin:0;color:var(--sg-muted);font-size:13.5px;line-height:1.6}
.sg-band{border-top:1px solid var(--sg-border);border-bottom:1px solid var(--sg-border);background:var(--sg-surface-alt)}
.sg-two{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);gap:22px;align-items:start}
.sg-panel{border:1px solid var(--sg-border);background:var(--sg-surface);border-radius:8px;padding:26px}
.sg-panel h3{display:flex;align-items:center;gap:10px;font-size:20px;margin:0 0 16px;color:var(--sg-text)}
.sg-panel h3 svg{width:20px;height:20px;color:var(--sg-success)}
.sg-links{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px}
.sg-link-card{display:flex;align-items:center;justify-content:space-between;gap:12px;border:1px solid var(--sg-border);background:var(--sg-surface);border-radius:8px;padding:16px;color:var(--sg-text);text-decoration:none;font-weight:700;font-size:14px}
.sg-link-card:hover{border-color:var(--sg-link);background:var(--sg-surface-elevated)}
.sg-link-card svg{width:17px;height:17px;color:var(--sg-link);flex:0 0 auto}
.sg-faq{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}
.sg-faq-item{border:1px solid var(--sg-border);background:var(--sg-surface);border-radius:8px;padding:20px}
.sg-faq-q{font-weight:800;color:var(--sg-text);font-size:15px;margin-bottom:8px}
.sg-faq-a{color:var(--sg-muted);font-size:14px;line-height:1.7;margin:0}
.sg-cta{padding:84px var(--sg-pad)}
.sg-cta-box{max-width:var(--sg-content);margin:0 auto;border:1px solid var(--sg-border);background:var(--sg-surface-elevated);border-radius:8px;padding:40px;display:flex;align-items:center;justify-content:space-between;gap:28px}
.sg-cta h2{margin:0 0 8px;font-size:30px;line-height:1.15;letter-spacing:0}
.sg-cta p{margin:0;color:var(--sg-muted);line-height:1.7;max-width:680px}
@media(max-width:1040px){.sg-hero{grid-template-columns:1fr}.sg-proof{grid-template-columns:repeat(2,minmax(0,1fr))}.sg-grid{grid-template-columns:1fr 1fr}.sg-workflow{grid-template-columns:1fr 1fr}.sg-links{grid-template-columns:1fr 1fr}}
@media(max-width:720px){:root{--sg-pad:20px}.sg-section{padding-top:62px;padding-bottom:62px}.sg-hero{padding-top:58px}.sg-title{font-size:38px}.sg-summary{font-size:16px}.sg-proof,.sg-grid,.sg-workflow,.sg-two,.sg-links,.sg-faq{grid-template-columns:1fr}.sg-code pre{font-size:12px}.sg-cta-box{align-items:flex-start;flex-direction:column;padding:28px}.sg-actions{width:100%}.sg-actions .sg-btn,.sg-actions .lp-btn{width:100%;justify-content:center}}
`;

export function SeoGrowthPage({ page, active }: { page: SeoGrowthPage; active?: "solutions" }) {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <div className="sg-shell">
        <PublicSiteHeader active={active} />
        <main className="sg-main">
          <section className="sg-hero sg-inner">
            <div>
              <div className="sg-kicker">
                <ShieldCheck aria-hidden="true" />
                {page.eyebrow}
              </div>
              <h1 className="sg-title">{page.h1}</h1>
              <p className="sg-summary">{page.summary}</p>
              <div className="sg-actions">
                <MarketingCTA />
                <Link href={page.secondaryCta.href} className="sg-btn">
                  {page.secondaryCta.label}
                  <ArrowRight aria-hidden="true" />
                </Link>
              </div>
              <div className="sg-query">
                Primary query:<strong>{page.primaryQuery}</strong>
              </div>
            </div>
            <div className="sg-code">
              <div className="sg-code-head">
                <span>
                  <Code2 aria-hidden="true" />
                  POST /v1/posts
                </span>
                <span>UniPost API</span>
              </div>
              <pre>{page.codeExample}</pre>
            </div>
          </section>

          <section className="sg-section tight">
            <div className="sg-inner">
              <div className="sg-proof">
                {page.proofPoints.map((point) => (
                  <div key={point} className="sg-proof-item">
                    <CheckCircle2 aria-hidden="true" />
                    <span>{point}</span>
                  </div>
                ))}
              </div>
            </div>
          </section>

          <section className="sg-section">
            <div className="sg-inner">
              <div className="sg-heading">
                <div className="sg-eyebrow">Workflow</div>
                <h2 className="sg-h2">From connected account to published post</h2>
                <p className="sg-lead">
                  These are the same primitives whether you are building a scheduler, SaaS workflow,
                  AI agent, or white-label social publishing layer.
                </p>
              </div>
              <div className="sg-workflow">
                {page.workflow.map((step, index) => (
                  <div key={step} className="sg-step">
                    <div className="sg-step-num">STEP {index + 1}</div>
                    <p>{step}</p>
                  </div>
                ))}
              </div>
            </div>
          </section>

          <section className="sg-section sg-band">
            <div className="sg-inner">
              <div className="sg-grid">
                {page.sections.map((section, index) => (
                  <article key={section.title} className="sg-card">
                    <div className="sg-card-icon">
                      {index === 0 ? <Workflow aria-hidden="true" /> : index === 1 ? <FileText aria-hidden="true" /> : <Link2 aria-hidden="true" />}
                    </div>
                    <h3>{section.title}</h3>
                    <p>{section.body}</p>
                    <ul className="sg-list">
                      {section.bullets.map((bullet) => (
                        <li key={bullet}>
                          <CheckCircle2 aria-hidden="true" />
                          <span>{bullet}</span>
                        </li>
                      ))}
                    </ul>
                  </article>
                ))}
              </div>
            </div>
          </section>

          <section className="sg-section">
            <div className="sg-inner sg-two">
              <div className="sg-panel">
                <h3>
                  <ShieldCheck aria-hidden="true" />
                  Honest limitations
                </h3>
                <ul className="sg-list">
                  {page.limitations.map((item) => (
                    <li key={item}>
                      <CheckCircle2 aria-hidden="true" />
                      <span>{item}</span>
                    </li>
                  ))}
                </ul>
              </div>
              <div className="sg-panel">
                <h3>
                  <Link2 aria-hidden="true" />
                  Related paths
                </h3>
                <div className="sg-links">
                  {page.relatedLinks.map((link) => (
                    <Link key={link.href} href={link.href} className="sg-link-card">
                      <span>{link.label}</span>
                      <ArrowRight aria-hidden="true" />
                    </Link>
                  ))}
                </div>
              </div>
            </div>
          </section>

          <section className="sg-section sg-band">
            <div className="sg-inner">
              <div className="sg-heading">
                <div className="sg-eyebrow">FAQ</div>
                <h2 className="sg-h2">Common evaluation questions</h2>
              </div>
              <div className="sg-faq">
                {page.faqs.map((faq) => (
                  <div key={faq.q} className="sg-faq-item">
                    <div className="sg-faq-q">{faq.q}</div>
                    <p className="sg-faq-a">{faq.a}</p>
                  </div>
                ))}
              </div>
            </div>
          </section>

          <section className="sg-cta">
            <div className="sg-cta-box">
              <div>
                <h2>Ship social publishing without rebuilding every native API.</h2>
                <p>Start with the docs, test on the free plan, then choose the pricing tier that fits your usage.</p>
              </div>
              <div className="sg-actions">
                <MarketingCTA />
                <Link href="/pricing" className="sg-btn">
                  Pricing
                  <ArrowRight aria-hidden="true" />
                </Link>
              </div>
            </div>
          </section>
        </main>
      </div>
    </>
  );
}
