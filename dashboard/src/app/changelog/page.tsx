import type { Metadata } from "next";
import Link from "next/link";
import { PublicSiteHeader } from "@/components/marketing/nav";
import {
  categoryLabels,
  changelogCategories,
  changelogReleases,
  impactLabels,
  type ChangelogRelease,
} from "./releases";

export const metadata: Metadata = {
  title: "UniPost Change Logs | Product Updates and SDK Releases",
  description:
    "Track major UniPost product updates, API releases, SDK versions, platform support, and developer experience improvements.",
  alternates: {
    canonical: "https://unipost.dev/changelog",
  },
  openGraph: {
    title: "UniPost Change Logs",
    description: "Major product, API, SDK, and platform releases from UniPost.",
    url: "https://unipost.dev/changelog",
    siteName: "UniPost",
    type: "website",
  },
};

const changelogReleaseRows = [...changelogReleases].sort((a, b) => b.date.localeCompare(a.date));
const latestRelease = changelogReleaseRows[0];
const latestSdk = changelogReleaseRows.flatMap((release) => release.sdkVersions ?? [])[0];

const categoryCounts = changelogCategories
  .map((category) => ({
    category,
    count: changelogReleaseRows.filter((release) => release.category === category).length,
  }))
  .filter((item) => item.count > 0);

function formatReleaseDate(release: ChangelogRelease) {
  return release.displayDate ?? release.date;
}

function ReleaseBadges({ release }: { release: ChangelogRelease }) {
  const impactLabel = impactLabels[release.impact];

  return (
    <span className="cl-badges">
      <span className={`cl-pill cl-pill-${release.category}`}>{categoryLabels[release.category]}</span>
      <span className={`cl-pill cl-pill-impact cl-pill-impact-${release.impact}`}>{impactLabel}</span>
      {release.isBreaking ? <span className="cl-pill cl-pill-breaking">Breaking</span> : null}
    </span>
  );
}

function ReleaseLinks({ links, compact = false }: { links: ChangelogRelease["links"]; compact?: boolean }) {
  return (
    <span className={compact ? "cl-links compact" : "cl-links"}>
      {links.map((link) => (
        <Link key={link.href} href={link.href} className="cl-link">
          {link.label}
        </Link>
      ))}
    </span>
  );
}

function SourceLinks({ links }: { links: ChangelogRelease["sourceLinks"] }) {
  return (
    <span className="cl-source-links">
      {links.map((link) => (
        <Link key={link.href} href={link.href} className="cl-source-link">
          {link.label}
        </Link>
      ))}
    </span>
  );
}

function SdkPills({ release }: { release: ChangelogRelease }) {
  if (!release.sdkVersions?.length) return <span className="cl-empty">-</span>;

  return (
    <span className="cl-sdk-list">
      {release.sdkVersions.map((sdk) => (
        <Link key={`${sdk.ecosystem}-${sdk.packageName}-${sdk.version}`} href={sdk.href} className="cl-sdk-pill">
          <span>{sdk.ecosystem}</span>
          <code>{sdk.packageName}</code>
          <strong>v{sdk.version}</strong>
        </Link>
      ))}
    </span>
  );
}

function ReleaseCard({ release }: { release: ChangelogRelease }) {
  return (
    <article className="cl-release-card" id={release.id}>
      <div className="cl-release-meta">
        <time dateTime={release.date}>{formatReleaseDate(release)}</time>
        <ReleaseBadges release={release} />
      </div>
      <h2>{release.title}</h2>
      <p>{release.summary}</p>
      <div className="cl-release-card-grid">
        <div>
          <span className="cl-mobile-label">SDK</span>
          <SdkPills release={release} />
        </div>
        <div>
          <span className="cl-mobile-label">Links</span>
          <ReleaseLinks links={release.links} compact />
        </div>
      </div>
      <div className="cl-source-row">
        <span>Verified by</span>
        <SourceLinks links={release.sourceLinks} />
      </div>
    </article>
  );
}

export default function ChangelogPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <PublicSiteHeader active="developer" />
      <main className="cl-page">
        <section className="cl-hero" aria-labelledby="changelog-title">
          <div className="cl-hero-copy">
            <p className="cl-eyebrow">Product updates</p>
            <h1 id="changelog-title">Change Logs</h1>
            <p className="cl-lead">
              Major UniPost product, API, SDK, and platform releases. This page stays intentionally high-signal so
              teams can see what changed, when it shipped, and where to verify it.
            </p>
            {latestRelease ? (
              <Link href={`#${latestRelease.id}`} className="cl-latest-release">
                <span>Latest release</span>
                <strong>{latestRelease.title}</strong>
                <small>{formatReleaseDate(latestRelease)} / {categoryLabels[latestRelease.category]}</small>
              </Link>
            ) : null}
          </div>

          <aside className="cl-index-panel" aria-label="Release index">
            <div className="cl-index-topline">
              <span>{changelogReleaseRows.length}</span>
              <p>verified releases tracked</p>
            </div>
            <div className="cl-index-counts">
              {categoryCounts.map(({ category, count }) => (
                <div key={category}>
                  <span>{categoryLabels[category]}</span>
                  <strong>{count}</strong>
                </div>
              ))}
            </div>
            {latestSdk ? (
              <div className="cl-index-sdk">
                <span>Latest SDK</span>
                <code>{latestSdk.packageName}</code>
                <strong>v{latestSdk.version}</strong>
              </div>
            ) : null}
          </aside>
        </section>

        <section className="cl-history" aria-labelledby="release-history-title">
          <div className="cl-section-head">
            <p className="cl-eyebrow">Release history</p>
            <h2 id="release-history-title">A public record of meaningful changes</h2>
          </div>

          <div className="cl-release-table-wrap">
            <table className="cl-release-table">
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Release</th>
                  <th>Area / Impact</th>
                  <th>SDK</th>
                  <th>Links</th>
                </tr>
              </thead>
              <tbody>
                {changelogReleaseRows.map((release) => (
                  <tr key={release.id} id={`${release.id}-row`}>
                    <td data-label="Date">
                      <a href={`#${release.id}`} className="cl-date-link">
                        <time dateTime={release.date}>{formatReleaseDate(release)}</time>
                      </a>
                    </td>
                    <td data-label="Release">
                      <div className="cl-release-copy">
                        <h3>{release.title}</h3>
                        <p>{release.summary}</p>
                        <div className="cl-source-row table-source">
                          <span>Verified by</span>
                          <SourceLinks links={release.sourceLinks} />
                        </div>
                      </div>
                    </td>
                    <td data-label="Area / Impact">
                      <ReleaseBadges release={release} />
                    </td>
                    <td data-label="SDK">
                      <SdkPills release={release} />
                    </td>
                    <td data-label="Links">
                      <ReleaseLinks links={release.links} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="cl-mobile-list" aria-label="Release history mobile list">
            {changelogReleaseRows.map((release) => (
              <ReleaseCard key={release.id} release={release} />
            ))}
          </div>
        </section>
      </main>
    </>
  );
}

const CSS = `
:root{
  --cl-bg:var(--app-bg);
  --cl-surface:var(--marketing-surface);
  --cl-surface-alt:var(--marketing-surface-alt);
  --cl-surface-elevated:var(--marketing-surface-elevated);
  --cl-border:var(--marketing-border);
  --cl-border-strong:var(--marketing-border-strong);
  --cl-text:var(--marketing-text);
  --cl-muted:var(--marketing-muted);
  --cl-subtle:var(--marketing-subtle);
  --cl-link:var(--marketing-link);
  --cl-link-hover:var(--marketing-link-hover);
  --cl-accent:var(--marketing-auth-primary-bg);
  --cl-shadow:var(--marketing-shadow-soft);
  --cl-ui:var(--font-dm-sans),system-ui,sans-serif;
  --cl-mono:var(--font-fira-code),ui-monospace,SFMono-Regular,Menlo,monospace;
}
.cl-page{width:100%;background:var(--cl-bg);color:var(--cl-text);font-family:var(--cl-ui);overflow-x:hidden}
.cl-hero{max-width:1320px;margin:0 auto;padding:86px 32px 42px;display:grid;grid-template-columns:minmax(0,1.38fr) minmax(320px,.62fr);gap:42px;align-items:end}
.cl-eyebrow{margin:0 0 14px;font-family:var(--cl-mono);font-size:12px;font-weight:650;letter-spacing:.08em;text-transform:uppercase;color:var(--cl-link)}
.cl-hero h1{margin:0;font-size:64px;line-height:1.02;letter-spacing:0;font-weight:850;color:var(--cl-text)}
.cl-lead{max-width:720px;margin:22px 0 0;font-size:18px;line-height:1.75;color:var(--cl-muted)}
.cl-latest-release{display:grid;gap:5px;max-width:520px;margin-top:34px;padding:18px 20px;border:1px solid var(--cl-border);border-radius:12px;background:var(--cl-surface);box-shadow:var(--cl-shadow);text-decoration:none;color:inherit;transition:border-color .16s ease,transform .16s ease}
.cl-latest-release:hover{border-color:color-mix(in srgb,var(--cl-link) 34%,var(--cl-border));transform:translateY(-1px)}
.cl-latest-release span{font-family:var(--cl-mono);font-size:11px;text-transform:uppercase;letter-spacing:.08em;color:var(--cl-subtle)}
.cl-latest-release strong{font-size:18px;color:var(--cl-text)}
.cl-latest-release small{font-family:var(--cl-mono);font-size:12px;color:var(--cl-muted)}
.cl-index-panel{border:1px solid var(--cl-border);border-radius:12px;background:var(--cl-surface);box-shadow:var(--cl-shadow);padding:22px;display:grid;gap:20px}
.cl-index-topline{display:grid;gap:4px}
.cl-index-topline span{font-family:var(--cl-mono);font-size:42px;line-height:1;font-weight:700;color:var(--cl-text)}
.cl-index-topline p{margin:0;color:var(--cl-muted);font-size:13px}
.cl-index-counts{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));border-top:1px solid var(--cl-border);border-left:1px solid var(--cl-border)}
.cl-index-counts div{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:11px 12px;border-right:1px solid var(--cl-border);border-bottom:1px solid var(--cl-border);background:var(--cl-surface-alt)}
.cl-index-counts span{font-size:12px;color:var(--cl-muted)}
.cl-index-counts strong{font-family:var(--cl-mono);font-size:13px;color:var(--cl-text)}
.cl-index-sdk{display:grid;gap:4px;padding:14px 0 0;border-top:1px solid var(--cl-border)}
.cl-index-sdk span{font-size:12px;color:var(--cl-subtle)}
.cl-index-sdk code{font-family:var(--cl-mono);font-size:13px;color:var(--cl-text)}
.cl-index-sdk strong{font-family:var(--cl-mono);font-size:13px;color:var(--cl-link)}
.cl-history{max-width:1320px;margin:0 auto;padding:46px 32px 112px}
.cl-section-head{display:grid;gap:4px;margin-bottom:22px}
.cl-section-head h2{margin:0;font-size:30px;line-height:1.18;letter-spacing:0;font-weight:780;color:var(--cl-text)}
.cl-release-table-wrap{border:1px solid var(--cl-border);border-radius:12px;overflow:hidden;background:var(--cl-surface);box-shadow:var(--cl-shadow)}
.cl-release-table{width:100%;border-collapse:collapse;table-layout:fixed}
.cl-release-table th{padding:13px 16px;border-bottom:1px solid var(--cl-border);background:var(--cl-surface-alt);font-family:var(--cl-mono);font-size:11px;letter-spacing:.08em;text-transform:uppercase;color:var(--cl-subtle);text-align:left}
.cl-release-table td{padding:18px 16px;border-bottom:1px solid var(--cl-border);vertical-align:top}
.cl-release-table tr:last-child td{border-bottom:0}
.cl-release-table th:nth-child(1),.cl-release-table td:nth-child(1){width:132px}
.cl-release-table th:nth-child(2),.cl-release-table td:nth-child(2){width:39%}
.cl-release-table th:nth-child(3),.cl-release-table td:nth-child(3){width:170px}
.cl-release-table th:nth-child(4),.cl-release-table td:nth-child(4){width:210px}
.cl-date-link{font-family:var(--cl-mono);font-size:12.5px;color:var(--cl-muted);text-decoration:none}
.cl-date-link:hover{color:var(--cl-link)}
.cl-release-copy{display:grid;gap:8px}
.cl-release-copy h3,.cl-release-card h2{margin:0;font-size:17px;line-height:1.35;letter-spacing:0;font-weight:760;color:var(--cl-text)}
.cl-release-copy p,.cl-release-card p{margin:0;font-size:14px;line-height:1.65;color:var(--cl-muted)}
.cl-badges{display:flex;flex-wrap:wrap;gap:6px;align-items:flex-start}
.cl-pill{display:inline-flex;align-items:center;min-height:24px;padding:4px 8px;border-radius:999px;border:1px solid var(--cl-border);background:var(--cl-surface-alt);font-family:var(--cl-mono);font-size:11px;font-weight:650;line-height:1;color:var(--cl-muted)}
.cl-pill-api,.cl-pill-sdk,.cl-pill-dx{color:var(--cl-link);border-color:color-mix(in srgb,var(--cl-link) 22%,var(--cl-border));background:color-mix(in srgb,var(--cl-link) 8%,var(--cl-surface-alt))}
.cl-pill-reliability{color:#12805c;border-color:color-mix(in srgb,#12805c 24%,var(--cl-border));background:color-mix(in srgb,#12805c 8%,var(--cl-surface-alt))}
.cl-pill-platform{color:#8a5c00;border-color:color-mix(in srgb,#8a5c00 24%,var(--cl-border));background:color-mix(in srgb,#8a5c00 8%,var(--cl-surface-alt))}
.cl-pill-dashboard{color:#8b4a62;border-color:color-mix(in srgb,#8b4a62 24%,var(--cl-border));background:color-mix(in srgb,#8b4a62 8%,var(--cl-surface-alt))}
.cl-pill-impact{color:var(--cl-text);background:transparent}
.cl-pill-breaking{color:#b42318;border-color:color-mix(in srgb,#b42318 28%,var(--cl-border));background:color-mix(in srgb,#b42318 9%,var(--cl-surface-alt))}
.cl-sdk-list,.cl-links,.cl-source-links{display:flex;flex-wrap:wrap;gap:7px}
.cl-sdk-pill{display:inline-flex;align-items:center;gap:6px;max-width:100%;padding:6px 8px;border-radius:8px;border:1px solid var(--cl-border);background:var(--cl-surface-alt);text-decoration:none;color:inherit}
.cl-sdk-pill span,.cl-sdk-pill strong{font-family:var(--cl-mono);font-size:11px;color:var(--cl-link)}
.cl-sdk-pill code{font-family:var(--cl-mono);font-size:11.5px;color:var(--cl-text);overflow-wrap:anywhere}
.cl-link,.cl-source-link{font-size:13px;line-height:1.45;color:var(--cl-link);text-decoration:none}
.cl-link:hover,.cl-source-link:hover{color:var(--cl-link-hover);text-decoration:underline}
.cl-source-row{display:flex;flex-wrap:wrap;gap:8px 10px;align-items:center;margin-top:4px}
.cl-source-row span:first-child{font-family:var(--cl-mono);font-size:10.5px;text-transform:uppercase;letter-spacing:.08em;color:var(--cl-subtle)}
.cl-source-link{font-size:12px;color:var(--cl-subtle)}
.cl-empty{font-family:var(--cl-mono);font-size:12px;color:var(--cl-subtle)}
.cl-mobile-list{display:none}
.cl-mobile-label{display:block;margin-bottom:7px;font-family:var(--cl-mono);font-size:10.5px;text-transform:uppercase;letter-spacing:.08em;color:var(--cl-subtle)}
.cl-release-card{display:grid;gap:12px;padding:18px;border:1px solid var(--cl-border);border-radius:12px;background:var(--cl-surface);box-shadow:var(--cl-shadow)}
.cl-release-meta{display:grid;gap:8px}
.cl-release-meta time{font-family:var(--cl-mono);font-size:12px;color:var(--cl-muted)}
.cl-release-card-grid{display:grid;gap:14px}
@media(max-width:980px){
  .cl-hero{grid-template-columns:1fr;padding:64px 24px 34px}
  .cl-hero h1{font-size:48px}
  .cl-history{padding:34px 24px 84px}
  .cl-release-table-wrap{display:none}
  .cl-mobile-list{display:grid;gap:14px}
}
@media(max-width:620px){
  .cl-hero{padding:46px 18px 28px}
  .cl-hero h1{font-size:40px}
  .cl-lead{font-size:16px}
  .cl-history{padding:28px 18px 72px}
  .cl-section-head h2{font-size:24px}
  .cl-index-counts{grid-template-columns:1fr}
  .cl-latest-release{padding:16px}
  .cl-link,.cl-source-link{overflow-wrap:anywhere}
}
`;
