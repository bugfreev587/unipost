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
const latestSdkRelease = changelogReleaseRows.find((release) => release.sdkVersions?.length);
const latestSdks = latestSdkRelease?.sdkVersions ?? [];

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
    <span className="chg-badges">
      <span className={`chg-pill chg-pill-${release.category}`}>{categoryLabels[release.category]}</span>
      <span className={`chg-pill chg-pill-impact chg-pill-impact-${release.impact}`}>{impactLabel}</span>
      {release.isBreaking ? <span className="chg-pill chg-pill-breaking">Breaking</span> : null}
    </span>
  );
}

function ReleaseLinks({ links, compact = false }: { links: ChangelogRelease["links"]; compact?: boolean }) {
  return (
    <span className={compact ? "chg-links compact" : "chg-links"}>
      {links.map((link) => (
        <Link key={link.href} href={link.href} className="chg-link">
          {link.label}
        </Link>
      ))}
    </span>
  );
}

function SourceLinks({ links }: { links: ChangelogRelease["sourceLinks"] }) {
  return (
    <span className="chg-source-links">
      {links.map((link) => (
        <Link key={link.href} href={link.href} className="chg-source-link">
          {link.label}
        </Link>
      ))}
    </span>
  );
}

function SdkPills({ release }: { release: ChangelogRelease }) {
  if (!release.sdkVersions?.length) return <span className="chg-empty">-</span>;

  return (
    <span className="chg-sdk-list">
      {release.sdkVersions.map((sdk) => (
        <Link key={`${sdk.ecosystem}-${sdk.packageName}-${sdk.version}`} href={sdk.href} className="chg-sdk-pill">
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
    <article className="chg-release-card" id={release.id}>
      <div className="chg-release-meta">
        <time dateTime={release.date}>{formatReleaseDate(release)}</time>
        <ReleaseBadges release={release} />
      </div>
      <h2>{release.title}</h2>
      <p>{release.summary}</p>
      <div className="chg-release-card-grid">
        <div>
          <span className="chg-mobile-label">SDK</span>
          <SdkPills release={release} />
        </div>
        <div>
          <span className="chg-mobile-label">Links</span>
          <ReleaseLinks links={release.links} compact />
        </div>
      </div>
      <div className="chg-source-row">
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
      <main className="chg-page">
        <section className="chg-hero" aria-labelledby="changelog-title">
          <div className="chg-hero-copy">
            <p className="chg-eyebrow">Product updates</p>
            <h1 id="changelog-title">Change Logs</h1>
            <p className="chg-lead">
              Major UniPost product, API, SDK, and platform releases. This page stays intentionally high-signal so
              teams can see what changed, when it shipped, and where to verify it.
            </p>
            {latestRelease ? (
              <Link href={`#${latestRelease.id}`} className="chg-latest-release">
                <span>Latest release</span>
                <strong>{latestRelease.title}</strong>
                <small>{formatReleaseDate(latestRelease)} / {categoryLabels[latestRelease.category]}</small>
              </Link>
            ) : null}
          </div>

          <aside className="chg-index-panel" aria-label="Release index">
            <div className="chg-index-topline">
              <span>{changelogReleaseRows.length}</span>
              <p>verified releases tracked</p>
            </div>
            <div className="chg-index-counts">
              {categoryCounts.map(({ category, count }) => (
                <div key={category}>
                  <span>{categoryLabels[category]}</span>
                  <strong>{count}</strong>
                </div>
              ))}
            </div>
            {latestSdks.length ? (
              <div className="chg-index-sdk">
                <span>Latest SDKs</span>
                <div className="chg-index-sdk-list">
                  {latestSdks.map((sdk) => (
                    <Link
                      key={`${sdk.ecosystem}-${sdk.packageName}-${sdk.version}-index`}
                      href={sdk.href}
                      className="chg-index-sdk-item"
                      title={sdk.installCommand ?? `${sdk.packageName} v${sdk.version}`}
                    >
                      <code>{sdk.ecosystem}</code>
                      <strong>v{sdk.version}</strong>
                    </Link>
                  ))}
                </div>
              </div>
            ) : null}
          </aside>
        </section>

        <section className="chg-history" aria-labelledby="release-history-title">
          <div className="chg-section-head">
            <p className="chg-eyebrow">Release history</p>
            <h2 id="release-history-title">A public record of meaningful changes</h2>
          </div>

          <div className="chg-release-table-wrap">
            <table className="chg-release-table">
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
                      <a href={`#${release.id}`} className="chg-date-link">
                        <time dateTime={release.date}>{formatReleaseDate(release)}</time>
                      </a>
                    </td>
                    <td data-label="Release">
                      <div className="chg-release-copy">
                        <h3>{release.title}</h3>
                        <p>{release.summary}</p>
                        <div className="chg-source-row table-source">
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

          <div className="chg-mobile-list" aria-label="Release history mobile list">
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
  --chg-bg:var(--app-bg);
  --chg-surface:var(--marketing-surface);
  --chg-surface-alt:var(--marketing-surface-alt);
  --chg-surface-elevated:var(--marketing-surface-elevated);
  --chg-border:var(--marketing-border);
  --chg-border-strong:var(--marketing-border-strong);
  --chg-text:var(--marketing-text);
  --chg-muted:var(--marketing-muted);
  --chg-subtle:var(--marketing-subtle);
  --chg-link:var(--marketing-link);
  --chg-link-hover:var(--marketing-link-hover);
  --chg-accent:var(--marketing-auth-primary-bg);
  --chg-shadow:var(--marketing-shadow-soft);
  --chg-ui:var(--font-dm-sans),system-ui,sans-serif;
  --chg-mono:var(--font-fira-code),ui-monospace,SFMono-Regular,Menlo,monospace;
}
.chg-page{width:100%;background:var(--chg-bg);color:var(--chg-text);font-family:var(--chg-ui);overflow-x:hidden}
.chg-hero{max-width:1320px;margin:0 auto;padding:86px 32px 42px;display:grid;grid-template-columns:minmax(0,1.38fr) minmax(320px,.62fr);gap:42px;align-items:end}
.chg-eyebrow{margin:0 0 14px;font-family:var(--chg-mono);font-size:12px;font-weight:650;letter-spacing:.08em;text-transform:uppercase;color:var(--chg-link)}
.chg-hero h1{margin:0;font-size:64px;line-height:1.02;letter-spacing:0;font-weight:850;color:var(--chg-text)}
.chg-lead{max-width:720px;margin:22px 0 0;font-size:18px;line-height:1.75;color:var(--chg-muted)}
.chg-latest-release{display:grid;gap:5px;max-width:520px;margin-top:34px;padding:18px 20px;border:1px solid var(--chg-border);border-radius:12px;background:var(--chg-surface);box-shadow:var(--chg-shadow);text-decoration:none;color:inherit;transition:border-color .16s ease,transform .16s ease}
.chg-latest-release:hover{border-color:color-mix(in srgb,var(--chg-link) 34%,var(--chg-border));transform:translateY(-1px)}
.chg-latest-release span{font-family:var(--chg-mono);font-size:11px;text-transform:uppercase;letter-spacing:.08em;color:var(--chg-subtle)}
.chg-latest-release strong{font-size:18px;color:var(--chg-text)}
.chg-latest-release small{font-family:var(--chg-mono);font-size:12px;color:var(--chg-muted)}
.chg-index-panel{border:1px solid var(--chg-border);border-radius:12px;background:var(--chg-surface);box-shadow:var(--chg-shadow);padding:22px;display:grid;gap:20px}
.chg-index-topline{display:grid;gap:4px}
.chg-index-topline span{font-family:var(--chg-mono);font-size:42px;line-height:1;font-weight:700;color:var(--chg-text)}
.chg-index-topline p{margin:0;color:var(--chg-muted);font-size:13px}
.chg-index-counts{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));border-top:1px solid var(--chg-border);border-left:1px solid var(--chg-border)}
.chg-index-counts div{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:11px 12px;border-right:1px solid var(--chg-border);border-bottom:1px solid var(--chg-border);background:var(--chg-surface-alt)}
.chg-index-counts span{font-size:12px;color:var(--chg-muted)}
.chg-index-counts strong{font-family:var(--chg-mono);font-size:13px;color:var(--chg-text)}
.chg-index-sdk{display:grid;gap:9px;padding:14px 0 0;border-top:1px solid var(--chg-border)}
.chg-index-sdk span{font-size:12px;color:var(--chg-subtle)}
.chg-index-sdk-list{display:flex;flex-wrap:wrap;gap:7px}
.chg-index-sdk-item{display:inline-flex;align-items:center;gap:6px;padding:7px 9px;border:1px solid var(--chg-border);border-radius:8px;background:var(--chg-surface-alt);text-decoration:none}
.chg-index-sdk-item code{font-family:var(--chg-mono);font-size:11.5px;color:var(--chg-text)}
.chg-index-sdk-item strong{font-family:var(--chg-mono);font-size:11.5px;color:var(--chg-link)}
.chg-index-sdk-item:hover{border-color:color-mix(in srgb,var(--chg-link) 28%,var(--chg-border))}
.chg-history{max-width:1320px;margin:0 auto;padding:46px 32px 112px}
.chg-section-head{display:grid;gap:4px;margin-bottom:22px}
.chg-section-head h2{margin:0;font-size:30px;line-height:1.18;letter-spacing:0;font-weight:780;color:var(--chg-text)}
.chg-release-table-wrap{border:1px solid var(--chg-border);border-radius:12px;overflow:hidden;background:var(--chg-surface);box-shadow:var(--chg-shadow)}
.chg-release-table{width:100%;border-collapse:collapse;table-layout:fixed}
.chg-release-table th{padding:13px 16px;border-bottom:1px solid var(--chg-border);background:var(--chg-surface-alt);font-family:var(--chg-mono);font-size:11px;letter-spacing:.08em;text-transform:uppercase;color:var(--chg-subtle);text-align:left}
.chg-release-table td{padding:18px 16px;border-bottom:1px solid var(--chg-border);vertical-align:top}
.chg-release-table tr:last-child td{border-bottom:0}
.chg-release-table th:nth-child(1),.chg-release-table td:nth-child(1){width:132px}
.chg-release-table th:nth-child(2),.chg-release-table td:nth-child(2){width:39%}
.chg-release-table th:nth-child(3),.chg-release-table td:nth-child(3){width:170px}
.chg-release-table th:nth-child(4),.chg-release-table td:nth-child(4){width:210px}
.chg-date-link{font-family:var(--chg-mono);font-size:12.5px;color:var(--chg-muted);text-decoration:none}
.chg-date-link:hover{color:var(--chg-link)}
.chg-release-copy{display:grid;gap:8px}
.chg-release-copy h3,.chg-release-card h2{margin:0;font-size:17px;line-height:1.35;letter-spacing:0;font-weight:760;color:var(--chg-text)}
.chg-release-copy p,.chg-release-card p{margin:0;font-size:14px;line-height:1.65;color:var(--chg-muted)}
.chg-badges{display:flex;flex-wrap:wrap;gap:6px;align-items:flex-start}
.chg-pill{display:inline-flex;align-items:center;min-height:24px;padding:4px 8px;border-radius:999px;border:1px solid var(--chg-border);background:var(--chg-surface-alt);font-family:var(--chg-mono);font-size:11px;font-weight:650;line-height:1;color:var(--chg-muted)}
.chg-pill-api,.chg-pill-sdk,.chg-pill-dx{color:var(--chg-link);border-color:color-mix(in srgb,var(--chg-link) 22%,var(--chg-border));background:color-mix(in srgb,var(--chg-link) 8%,var(--chg-surface-alt))}
.chg-pill-reliability{color:#12805c;border-color:color-mix(in srgb,#12805c 24%,var(--chg-border));background:color-mix(in srgb,#12805c 8%,var(--chg-surface-alt))}
.chg-pill-platform{color:#8a5c00;border-color:color-mix(in srgb,#8a5c00 24%,var(--chg-border));background:color-mix(in srgb,#8a5c00 8%,var(--chg-surface-alt))}
.chg-pill-dashboard{color:#8b4a62;border-color:color-mix(in srgb,#8b4a62 24%,var(--chg-border));background:color-mix(in srgb,#8b4a62 8%,var(--chg-surface-alt))}
.chg-pill-impact{color:var(--chg-text);background:transparent}
.chg-pill-breaking{color:#b42318;border-color:color-mix(in srgb,#b42318 28%,var(--chg-border));background:color-mix(in srgb,#b42318 9%,var(--chg-surface-alt))}
.chg-sdk-list,.chg-links,.chg-source-links{display:flex;flex-wrap:wrap;gap:7px}
.chg-sdk-pill{display:inline-flex;align-items:center;gap:6px;max-width:100%;padding:6px 8px;border-radius:8px;border:1px solid var(--chg-border);background:var(--chg-surface-alt);text-decoration:none;color:inherit}
.chg-sdk-pill span,.chg-sdk-pill strong{font-family:var(--chg-mono);font-size:11px;color:var(--chg-link)}
.chg-sdk-pill code{font-family:var(--chg-mono);font-size:11.5px;color:var(--chg-text);overflow-wrap:anywhere}
.chg-link,.chg-source-link{font-size:13px;line-height:1.45;color:var(--chg-link);text-decoration:none}
.chg-link:hover,.chg-source-link:hover{color:var(--chg-link-hover);text-decoration:underline}
.chg-source-row{display:flex;flex-wrap:wrap;gap:8px 10px;align-items:center;margin-top:4px}
.chg-source-row span:first-child{font-family:var(--chg-mono);font-size:10.5px;text-transform:uppercase;letter-spacing:.08em;color:var(--chg-subtle)}
.chg-source-link{font-size:12px;color:var(--chg-subtle)}
.chg-empty{font-family:var(--chg-mono);font-size:12px;color:var(--chg-subtle)}
.chg-mobile-list{display:none}
.chg-mobile-label{display:block;margin-bottom:7px;font-family:var(--chg-mono);font-size:10.5px;text-transform:uppercase;letter-spacing:.08em;color:var(--chg-subtle)}
.chg-release-card{display:grid;gap:12px;padding:18px;border:1px solid var(--chg-border);border-radius:12px;background:var(--chg-surface);box-shadow:var(--chg-shadow)}
.chg-release-meta{display:grid;gap:8px}
.chg-release-meta time{font-family:var(--chg-mono);font-size:12px;color:var(--chg-muted)}
.chg-release-card-grid{display:grid;gap:14px}
@media(max-width:980px){
  .chg-hero{grid-template-columns:1fr;padding:64px 24px 34px}
  .chg-hero h1{font-size:48px}
  .chg-history{padding:34px 24px 84px}
  .chg-release-table-wrap{display:none}
  .chg-mobile-list{display:grid;gap:14px}
}
@media(max-width:620px){
  .chg-hero{padding:46px 18px 28px}
  .chg-hero h1{font-size:40px}
  .chg-lead{font-size:16px}
  .chg-history{padding:28px 18px 72px}
  .chg-section-head h2{font-size:24px}
  .chg-index-counts{grid-template-columns:1fr}
  .chg-latest-release{padding:16px}
  .chg-link,.chg-source-link{overflow-wrap:anywhere}
}
`;
