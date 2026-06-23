import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, FileText } from "lucide-react";
import { PublicSiteHeader } from "@/components/marketing/nav";
import { SEO_RESOURCES } from "@/data/seo-resources";

export const metadata: Metadata = {
  title: "Social Media API Resources | UniPost",
  description:
    "Practical resources for evaluating social media APIs, including platform requirements, posting constraints, OAuth review, media workflows, and cost planning.",
  alternates: {
    canonical: "https://unipost.dev/resources",
  },
  openGraph: {
    title: "Social Media API Resources",
    description:
      "Platform matrices, OAuth review guides, media workflow notes, and cost planning for social media API teams.",
    url: "https://unipost.dev/resources",
    siteName: "UniPost",
    type: "website",
  },
};

const CSS = `
:root{--res-bg:var(--app-bg);--res-s1:var(--marketing-surface);--res-s2:var(--marketing-surface-alt);--res-s3:var(--marketing-surface-elevated);--res-border:var(--marketing-border);--res-b2:var(--marketing-border-strong);--res-text:var(--marketing-text);--res-muted:var(--marketing-muted);--res-subtle:var(--marketing-subtle);--res-link:var(--marketing-link);--res-success:var(--primary);--res-mono:var(--font-fira-code),monospace;--res-ui:var(--font-dm-sans),system-ui,sans-serif;--res-content:1120px;--res-pad:32px}
*{box-sizing:border-box}
body{background:var(--res-bg);color:var(--res-text);font-family:var(--res-ui);line-height:1.6;-webkit-font-smoothing:antialiased}
.res-shell{background:linear-gradient(180deg,color-mix(in srgb,var(--res-s2) 56%,var(--res-bg)),var(--res-bg) 500px)}
.res-main{padding:86px var(--res-pad) 96px}
.res-inner{max-width:var(--res-content);margin:0 auto}
.res-kicker{font-family:var(--res-mono);font-size:12px;font-weight:800;text-transform:uppercase;letter-spacing:0;color:var(--res-success);margin-bottom:16px}
.res-title{font-size:54px;line-height:1.06;letter-spacing:0;margin:0 0 18px;font-weight:900}
.res-sub{font-size:18px;line-height:1.75;color:var(--res-muted);margin:0 0 42px;max-width:780px}
.res-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px}
.res-card{display:flex;flex-direction:column;gap:14px;min-height:240px;border:1px solid var(--res-border);background:var(--res-s1);border-radius:8px;padding:24px;color:var(--res-text);text-decoration:none}
.res-card:hover{border-color:var(--res-link);background:var(--res-s3)}
.res-card-top{display:flex;align-items:center;justify-content:space-between;gap:16px}
.res-card-icon{display:inline-flex;align-items:center;justify-content:center;width:36px;height:36px;border:1px solid color-mix(in srgb,var(--res-success) 24%,transparent);background:color-mix(in srgb,var(--res-success) 12%,transparent);border-radius:8px;color:var(--res-success)}
.res-card h2{font-size:21px;line-height:1.2;margin:0}
.res-card p{margin:0;color:var(--res-muted);line-height:1.7;font-size:14.5px}
.res-date{font-family:var(--res-mono);font-size:12px;color:var(--res-subtle);margin-top:auto}
@media(max-width:760px){:root{--res-pad:20px}.res-main{padding-top:58px}.res-title{font-size:38px}.res-grid{grid-template-columns:1fr}}
`;

export default function ResourcesPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <div className="res-shell">
        <PublicSiteHeader active="developer" />
        <main className="res-main">
          <div className="res-inner">
            <div className="res-kicker">Original resources</div>
            <h1 className="res-title">Social media API resources for product teams</h1>
            <p className="res-sub">
              Practical references for platform requirements, posting constraints, OAuth review,
              media workflows, and the engineering cost of native integrations.
            </p>
            <div className="res-grid">
              {SEO_RESOURCES.map((resource) => (
                <Link key={resource.slug} href={`/resources/${resource.slug}`} className="res-card">
                  <div className="res-card-top">
                    <span className="res-card-icon"><FileText aria-hidden="true" /></span>
                    <ArrowRight aria-hidden="true" />
                  </div>
                  <h2>{resource.h1}</h2>
                  <p>{resource.summary}</p>
                  <div className="res-date">Last verified {resource.lastVerified}</div>
                </Link>
              ))}
            </div>
          </div>
        </main>
      </div>
    </>
  );
}
