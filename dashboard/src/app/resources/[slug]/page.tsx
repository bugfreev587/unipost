import type { Metadata } from "next";
import { notFound } from "next/navigation";
import Link from "next/link";
import { ArrowRight, CheckCircle2, FileText } from "lucide-react";
import { PublicSiteHeader } from "@/components/marketing/nav";
import { SEO_RESOURCES, getSeoResource } from "@/data/seo-resources";

type ResourcePageProps = {
  params: Promise<{ slug: string }>;
};

export function generateStaticParams() {
  return SEO_RESOURCES.map((resource) => ({ slug: resource.slug }));
}

export async function generateMetadata({ params }: ResourcePageProps): Promise<Metadata> {
  const { slug } = await params;
  const resource = getSeoResource(slug);
  if (!resource) return {};

  const canonical = `https://unipost.dev/resources/${resource.slug}`;

  return {
    title: resource.title,
    description: resource.description,
    alternates: {
      canonical,
    },
    openGraph: {
      title: resource.title,
      description: resource.description,
      url: canonical,
      siteName: "UniPost",
      type: "article",
    },
  };
}

const CSS = `
:root{--rr-bg:var(--app-bg);--rr-s1:var(--marketing-surface);--rr-s2:var(--marketing-surface-alt);--rr-s3:var(--marketing-surface-elevated);--rr-border:var(--marketing-border);--rr-b2:var(--marketing-border-strong);--rr-text:var(--marketing-text);--rr-muted:var(--marketing-muted);--rr-subtle:var(--marketing-subtle);--rr-link:var(--marketing-link);--rr-success:var(--primary);--rr-mono:var(--font-fira-code),monospace;--rr-ui:var(--font-dm-sans),system-ui,sans-serif;--rr-content:1040px;--rr-pad:32px}
*{box-sizing:border-box}
body{background:var(--rr-bg);color:var(--rr-text);font-family:var(--rr-ui);line-height:1.6;-webkit-font-smoothing:antialiased}
.rr-shell{background:linear-gradient(180deg,color-mix(in srgb,var(--rr-s2) 56%,var(--rr-bg)),var(--rr-bg) 480px)}
.rr-main{padding:64px var(--rr-pad) 96px}
.rr-inner{max-width:var(--rr-content);margin:0 auto}
.rr-bread{font-size:13px;color:var(--rr-subtle);margin-bottom:42px}
.rr-bread a{color:var(--rr-muted);text-decoration:none}.rr-bread a:hover{color:var(--rr-text)}
.rr-kicker{display:inline-flex;gap:8px;align-items:center;font-family:var(--rr-mono);font-size:12px;font-weight:800;text-transform:uppercase;letter-spacing:0;color:var(--rr-success);margin-bottom:16px}
.rr-title{font-size:52px;line-height:1.06;letter-spacing:0;margin:0 0 18px;font-weight:900}
.rr-sub{font-size:18px;line-height:1.75;color:var(--rr-muted);margin:0 0 18px;max-width:780px}
.rr-date{font-family:var(--rr-mono);font-size:12px;color:var(--rr-subtle);margin-bottom:48px}
.rr-section{border-top:1px solid var(--rr-border);padding:42px 0}
.rr-section h2{font-size:30px;line-height:1.16;margin:0 0 10px;letter-spacing:0}
.rr-section p{margin:0 0 22px;color:var(--rr-muted);line-height:1.75}
.rr-table{display:grid;gap:10px}
.rr-row{display:grid;grid-template-columns:190px minmax(0,.8fr) minmax(0,1.2fr);gap:16px;border:1px solid var(--rr-border);background:var(--rr-s1);border-radius:8px;padding:16px}
.rr-row strong{color:var(--rr-text)}
.rr-row span{color:var(--rr-muted);font-size:14px;line-height:1.6}
.rr-faq{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}
.rr-faq-item{border:1px solid var(--rr-border);background:var(--rr-s1);border-radius:8px;padding:20px}
.rr-faq-q{display:flex;gap:8px;align-items:flex-start;font-weight:800;color:var(--rr-text);margin-bottom:8px}
.rr-faq-q svg{width:16px;height:16px;color:var(--rr-success);margin-top:3px}
.rr-faq-a{color:var(--rr-muted);line-height:1.7;margin:0;font-size:14px}
.rr-links{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;margin-top:28px}
.rr-link{display:flex;align-items:center;justify-content:space-between;gap:12px;border:1px solid var(--rr-border);background:var(--rr-s1);border-radius:8px;padding:15px;color:var(--rr-text);font-weight:800;text-decoration:none}
.rr-link:hover{border-color:var(--rr-link);background:var(--rr-s3)}
@media(max-width:760px){:root{--rr-pad:20px}.rr-title{font-size:36px}.rr-row{grid-template-columns:1fr}.rr-faq,.rr-links{grid-template-columns:1fr}}
`;

export default async function ResourcePage({ params }: ResourcePageProps) {
  const { slug } = await params;
  const resource = getSeoResource(slug);
  if (!resource) {
    notFound();
  }

  const canonical = `https://unipost.dev/resources/${resource.slug}`;
  const schema = [
    {
      "@context": "https://schema.org",
      "@type": "Article",
      headline: resource.h1,
      description: resource.description,
      dateModified: resource.lastVerified,
      url: canonical,
    },
    {
      "@context": "https://schema.org",
      "@type": "FAQPage",
      mainEntity: resource.faqs.map((faq) => ({
        "@type": "Question",
        name: faq.q,
        acceptedAnswer: { "@type": "Answer", text: faq.a },
      })),
    },
    {
      "@context": "https://schema.org",
      "@type": "BreadcrumbList",
      itemListElement: [
        { "@type": "ListItem", position: 1, name: "UniPost", item: "https://unipost.dev" },
        { "@type": "ListItem", position: 2, name: "Resources", item: "https://unipost.dev/resources" },
        { "@type": "ListItem", position: 3, name: resource.h1, item: canonical },
      ],
    },
  ];

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(schema) }} />
      <div className="rr-shell">
        <PublicSiteHeader active="developer" />
        <main className="rr-main">
          <article className="rr-inner">
            <div className="rr-bread">
              <Link href="/">UniPost</Link> / <Link href="/resources">Resources</Link> / {resource.h1}
            </div>
            <div className="rr-kicker"><FileText aria-hidden="true" />{resource.eyebrow}</div>
            <h1 className="rr-title">{resource.h1}</h1>
            <p className="rr-sub">{resource.summary}</p>
            <div className="rr-date">Last verified {resource.lastVerified}</div>

            {resource.sections.map((section) => (
              <section key={section.title} className="rr-section">
                <h2>{section.title}</h2>
                <p>{section.body}</p>
                <div className="rr-table">
                  {section.rows.map((row) => (
                    <div key={row.label} className="rr-row">
                      <strong>{row.label}</strong>
                      <span>{row.value}</span>
                      <span>{row.note}</span>
                    </div>
                  ))}
                </div>
              </section>
            ))}

            <section className="rr-section">
              <h2>FAQ</h2>
              <div className="rr-faq">
                {resource.faqs.map((faq) => (
                  <div key={faq.q} className="rr-faq-item">
                    <div className="rr-faq-q"><CheckCircle2 aria-hidden="true" />{faq.q}</div>
                    <p className="rr-faq-a">{faq.a}</p>
                  </div>
                ))}
              </div>
              <div className="rr-links">
                <Link href="/social-media-api" className="rr-link">
                  <span>Unified API page</span>
                  <ArrowRight aria-hidden="true" />
                </Link>
                <Link href="/social-media-posting-api" className="rr-link">
                  <span>Posting API page</span>
                  <ArrowRight aria-hidden="true" />
                </Link>
                <Link href="/compare/social-media-apis" className="rr-link">
                  <span>Compare APIs</span>
                  <ArrowRight aria-hidden="true" />
                </Link>
              </div>
            </section>
          </article>
        </main>
      </div>
    </>
  );
}
