import type { CSSProperties } from "react";
import Link from "next/link";
import { ArrowRight, BarChart3, CheckCircle2, ExternalLink } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";

export type AnalyticsToolSlug = "tiktok" | "instagram" | "threads" | "pinterest";

export type AnalyticsMetric = {
  label: string;
  value: string;
  note: string;
};

export type AnalyticsTable = {
  title: string;
  description: string;
  headers: string[];
  rows: string[][];
};

export type AnalyticsToolConfig = {
  slug: AnalyticsToolSlug;
  platform: string;
  href: string;
  title: string;
  seoTitle: string;
  description: string;
  eyebrow: string;
  summary: string;
  accent: string;
  scopes: string[];
  metrics: AnalyticsMetric[];
  tables: AnalyticsTable[];
  docsHref: string;
};

export const analyticsTools: Record<AnalyticsToolSlug, AnalyticsToolConfig> = {
  tiktok: {
    slug: "tiktok",
    platform: "TikTok",
    href: "/tools/tiktok-analytics",
    title: "TikTok Analytics",
    seoTitle: "TikTok Analytics API and Dashboard Tool | UniPost",
    description:
      "Preview TikTok profile analytics, account stats, public videos, and UniPost-published post performance with UniPost.",
    eyebrow: "Platform Analytics",
    summary:
      "Connect TikTok once and inspect profile fields, follower stats, public videos, and post-level performance from the same UniPost analytics surface.",
    accent: "#111827",
    scopes: ["user.info.profile", "user.info.stats", "video.list"],
    docsHref: "/docs/api/analytics/posts",
    metrics: [
      { label: "Followers", value: "12.4k", note: "From user.info.stats" },
      { label: "Total Likes", value: "86.7k", note: "Account-level TikTok stats" },
      { label: "Public Videos", value: "146", note: "Inventory from video.list" },
      { label: "Recent Views", value: "17.8k", note: "Sample public video window" },
    ],
    tables: [
      {
        title: "Public Videos",
        description: "Owned videos returned by TikTok's approved video.list scope.",
        headers: ["Video", "Created", "Views", "Likes", "Comments", "Shares"],
        rows: [
          ["Launch workflow in 30 seconds", "May 12, 2026", "8.2k", "612", "38", "91"],
          ["How UniPost schedules a TikTok", "May 9, 2026", "5.7k", "433", "27", "64"],
          ["API-first creator workflow", "May 3, 2026", "3.9k", "284", "19", "41"],
        ],
      },
      {
        title: "UniPost-Published TikTok Posts",
        description: "Post analytics normalized back into UniPost after publishing.",
        headers: ["Post", "Status", "Video ID", "Views", "Likes", "Shares"],
        rows: [
          ["Product launch recap", "Published", "7350123456789012345", "8.2k", "612", "91"],
          ["Creator API tutorial", "Published", "7350123456789012311", "5.7k", "433", "64"],
        ],
      },
    ],
  },
  instagram: {
    slug: "instagram",
    platform: "Instagram",
    href: "/tools/instagram-analytics",
    title: "Instagram Analytics",
    seoTitle: "Instagram Analytics API for Business Accounts | UniPost",
    description:
      "Preview Instagram Business profile metrics, recent media insights, and UniPost-published Instagram post analytics.",
    eyebrow: "Business Account Insights",
    summary:
      "UniPost brings Instagram Business account data, recent media insights, and published post performance into one developer-friendly analytics view.",
    accent: "#c13584",
    scopes: ["instagram_business_basic", "instagram_business_manage_insights"],
    docsHref: "/docs/api/accounts/metrics",
    metrics: [
      { label: "Followers", value: "48.6k", note: "Business profile snapshot" },
      { label: "Media", value: "328", note: "Connected account inventory" },
      { label: "Recent Reach", value: "38.4k", note: "Recent media reach" },
      { label: "Saves", value: "1.2k", note: "Native media insight" },
    ],
    tables: [
      {
        title: "Recent Instagram Media",
        description: "Owned media with reach, saves, and engagement from Instagram Business Login.",
        headers: ["Media", "Date", "Reach", "Likes", "Comments", "Shares", "Saves"],
        rows: [
          ["Carousel launch notes", "May 18, 2026", "14.8k", "1.1k", "86", "143", "392"],
          ["Reel: API publishing flow", "May 15, 2026", "18.2k", "1.5k", "112", "208", "514"],
          ["Product screenshot drop", "May 10, 2026", "5.4k", "428", "31", "49", "146"],
        ],
      },
      {
        title: "UniPost-Published Instagram Posts",
        description: "Cross-platform post analytics cached after UniPost publishes to Instagram.",
        headers: ["Post", "Status", "External ID", "Reach", "Likes", "Comments"],
        rows: [
          ["Launch carousel", "Published", "ig_1806218473", "14.8k", "1.1k", "86"],
          ["API workflow Reel", "Published", "ig_1795519028", "18.2k", "1.5k", "112"],
        ],
      },
    ],
  },
  threads: {
    slug: "threads",
    platform: "Threads",
    href: "/tools/threads-analytics",
    title: "Threads Analytics",
    seoTitle: "Threads Analytics API and Post Insights | UniPost",
    description:
      "Preview Threads profile analytics, recent post performance, replies, reposts, quotes, and UniPost-published post metrics.",
    eyebrow: "Profile and Post Insights",
    summary:
      "Track Threads profile performance, recent posts, replies, reposts, quotes, and published content without building a separate Meta analytics layer.",
    accent: "#111827",
    scopes: ["threads_basic", "threads_manage_insights"],
    docsHref: "/docs/api/analytics/posts",
    metrics: [
      { label: "Followers", value: "21.9k", note: "Profile analytics" },
      { label: "Views", value: "42.7k", note: "Recent post window" },
      { label: "Replies", value: "386", note: "Conversation signal" },
      { label: "Reposts", value: "719", note: "Native Threads metric" },
    ],
    tables: [
      {
        title: "Recent Threads Posts",
        description: "Owned Threads posts with views, likes, replies, reposts, and quotes.",
        headers: ["Post", "Date", "Views", "Likes", "Replies", "Reposts", "Quotes"],
        rows: [
          ["Shipping analytics for builders", "May 19, 2026", "18.6k", "1.4k", "132", "284", "61"],
          ["What developers need after publish", "May 16, 2026", "13.1k", "927", "94", "213", "48"],
          ["One API, four analytics surfaces", "May 11, 2026", "11.0k", "688", "72", "222", "33"],
        ],
      },
      {
        title: "UniPost-Published Threads Posts",
        description: "Published Threads entries reconciled with UniPost post analytics.",
        headers: ["Post", "Status", "External ID", "Views", "Likes", "Replies"],
        rows: [
          ["Analytics API launch thread", "Published", "threads_1805129771", "18.6k", "1.4k", "132"],
          ["Developer workflow note", "Published", "threads_1805129820", "13.1k", "927", "94"],
        ],
      },
    ],
  },
  pinterest: {
    slug: "pinterest",
    platform: "Pinterest",
    href: "/tools/pinterest-analytics",
    title: "Pinterest Analytics",
    seoTitle: "Pinterest Analytics API for Pins and Boards | UniPost",
    description:
      "Preview Pinterest board inventory, Pin impressions, saves, outbound clicks, comments, and UniPost-published Pin analytics.",
    eyebrow: "Pin and Board Analytics",
    summary:
      "UniPost connects Pinterest boards, published Pins, impressions, saves, outbound clicks, and comments into one analytics workflow.",
    accent: "#bd081c",
    scopes: ["pins:read", "boards:read", "user_accounts:read"],
    docsHref: "/docs/api/analytics/posts",
    metrics: [
      { label: "Published Pins", value: "64", note: "UniPost-published pins" },
      { label: "Boards", value: "12", note: "Connected board inventory" },
      { label: "Impressions", value: "72.3k", note: "Production analytics API" },
      { label: "Outbound Clicks", value: "3.8k", note: "Traffic to destination URLs" },
    ],
    tables: [
      {
        title: "Pinterest Pin Performance",
        description: "Published Pin metrics from Pinterest production analytics.",
        headers: ["Pin", "Board", "Impressions", "Saves", "Outbound Clicks", "Comments"],
        rows: [
          ["Launch checklist graphic", "Product Marketing", "24.8k", "1.9k", "1.2k", "43"],
          ["API workflow diagram", "Developer Tools", "18.6k", "1.1k", "884", "31"],
          ["Content calendar template", "Planning", "28.9k", "2.4k", "1.7k", "58"],
        ],
      },
      {
        title: "UniPost-Published Pinterest Posts",
        description: "Pin analytics normalized back to the posts created in UniPost.",
        headers: ["Post", "Status", "Pin ID", "Impressions", "Saves", "Clicks"],
        rows: [
          ["Launch checklist", "Published", "1107111520928571145", "24.8k", "1.9k", "1.2k"],
          ["Developer workflow diagram", "Published", "1107111520928571188", "18.6k", "1.1k", "884"],
        ],
      },
    ],
  },
};

export function getAnalyticsTool(slug: AnalyticsToolSlug): AnalyticsToolConfig {
  return analyticsTools[slug];
}

export function PublicAnalyticsToolPage({ tool }: { tool: AnalyticsToolConfig }) {
  const relatedTools = Object.values(analyticsTools).filter((item) => item.slug !== tool.slug);
  const accentStyle = { "--at-accent": tool.accent } as CSSProperties;

  return (
    <main className="at-page" style={accentStyle}>
      <style dangerouslySetInnerHTML={{ __html: ANALYTICS_TOOL_CSS }} />

      <section className="at-hero">
        <div className="at-hero-copy">
          <div className="at-eyebrow">
            <span className="at-icon-pill">
              <PlatformIcon platform={tool.slug} size={18} />
            </span>
            {tool.eyebrow}
          </div>
          <h1>{tool.title} for UniPost developers</h1>
          <p>{tool.summary}</p>
          <div className="at-actions">
            <a href="https://app.unipost.dev/welcome" className="lp-btn lp-btn-primary lp-btn-lg">
              Start Building
            </a>
            <Link href={tool.docsHref} className="lp-btn lp-btn-outline lp-btn-lg">
              Read Analytics Docs
            </Link>
          </div>
        </div>

        <div className="at-hero-panel" aria-label={`${tool.platform} analytics sample`}>
          <div className="at-panel-top">
            <div className="at-window-dots" aria-hidden="true">
              <span />
              <span />
              <span />
            </div>
            <span>{tool.platform} analytics sample</span>
          </div>
          <div className="at-panel-body">
            <div className="at-panel-title-row">
              <div>
                <div className="at-panel-label">Posts Overview</div>
                <div className="at-panel-title">Cross-platform performance, platform-native details</div>
              </div>
              <BarChart3 aria-hidden="true" />
            </div>
            <div className="at-metric-grid">
              {tool.metrics.map((metric) => (
                <div className="at-metric" key={metric.label}>
                  <span>{metric.label}</span>
                  <strong>{metric.value}</strong>
                  <small>{metric.note}</small>
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      <section className="at-section at-scope-section">
        <div>
          <div className="at-section-label">Permissions</div>
          <h2>Uses the scopes your analytics surface actually needs.</h2>
        </div>
        <div className="at-scope-list">
          {tool.scopes.map((scope) => (
            <span key={scope}>
              <CheckCircle2 aria-hidden="true" />
              {scope}
            </span>
          ))}
        </div>
      </section>

      <section className="at-section">
        <div className="at-section-heading">
          <div className="at-section-label">Sample Data</div>
          <h2>What the {tool.platform} analytics view can show</h2>
          <p>
            UniPost keeps cross-platform post analytics normalized while preserving native
            metrics that only exist on each social network.
          </p>
        </div>

        <div className="at-table-stack">
          {tool.tables.map((table) => (
            <div className="at-table-card" key={table.title}>
              <div className="at-table-head">
                <div>
                  <h3>{table.title}</h3>
                  <p>{table.description}</p>
                </div>
              </div>
              <div className="at-table-wrap">
                <table>
                  <thead>
                    <tr>
                      {table.headers.map((header) => (
                        <th key={header}>{header}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {table.rows.map((row, rowIndex) => (
                      <tr key={`${table.title}-${rowIndex}`}>
                        {row.map((cell, cellIndex) => (
                          <td key={`${table.title}-${rowIndex}-${cellIndex}`}>{cell}</td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className="at-section at-related-section">
        <div>
          <div className="at-section-label">More Platform Analytics</div>
          <h2>Compare the rest of the UniPost analytics surfaces.</h2>
        </div>
        <div className="at-related-grid">
          {relatedTools.map((item) => (
            <Link href={item.href} className="at-related-link" key={item.slug}>
              <span>
                <PlatformIcon platform={item.slug} size={16} />
                {item.title}
              </span>
              <ArrowRight aria-hidden="true" />
            </Link>
          ))}
        </div>
      </section>

      <section className="at-cta">
        <div>
          <div className="at-section-label">Build With UniPost</div>
          <h2>Publishing and analytics belong in the same developer workflow.</h2>
          <p>
            Connect accounts, publish posts, and read performance through one API surface
            instead of maintaining one integration per platform.
          </p>
        </div>
        <div className="at-actions">
          <a href="https://app.unipost.dev/welcome" className="lp-btn lp-btn-primary lp-btn-lg">
            Start Building
          </a>
          <Link href="/docs/api/analytics" className="lp-btn lp-btn-outline lp-btn-lg">
            Analytics API
            <ExternalLink aria-hidden="true" />
          </Link>
        </div>
      </section>
    </main>
  );
}

const ANALYTICS_TOOL_CSS = `
.at-page{max-width:1180px;margin:0 auto;padding:0 var(--tl-px) var(--tl-section-py);--at-line:color-mix(in srgb,var(--at-accent) 18%,var(--tl-border));--at-soft:color-mix(in srgb,var(--at-accent) 8%,transparent)}
.at-hero{display:grid;grid-template-columns:minmax(0,1.02fr) minmax(380px,.98fr);gap:48px;align-items:center;padding:88px 0 64px}
.at-hero-copy h1{font-size:clamp(36px,5vw,64px);line-height:.98;letter-spacing:-.04em;font-weight:900;color:var(--tl-text);margin:0 0 22px}
.at-hero-copy p{font-size:17px;line-height:1.72;color:var(--tl-muted);max-width:650px;margin:0 0 30px}
.at-eyebrow{display:inline-flex;align-items:center;gap:10px;font-family:var(--tl-mono);font-size:11.5px;letter-spacing:.12em;text-transform:uppercase;color:var(--at-accent);font-weight:700;margin-bottom:22px}
.at-icon-pill{width:34px;height:34px;border-radius:10px;border:1px solid var(--at-line);background:var(--at-soft);display:inline-flex;align-items:center;justify-content:center;color:var(--tl-text)}
.at-actions{display:flex;align-items:center;gap:12px;flex-wrap:wrap}
.at-hero-panel{border:1px solid var(--at-line);border-radius:16px;background:var(--tl-s1);box-shadow:var(--tl-card-shadow);overflow:hidden}
.at-panel-top{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:12px 16px;border-bottom:1px solid var(--tl-border);color:var(--tl-muted2);font-size:12px;font-family:var(--tl-mono)}
.at-window-dots{display:flex;gap:6px}.at-window-dots span{width:9px;height:9px;border-radius:50%;background:var(--tl-b3)}
.at-panel-body{padding:24px}
.at-panel-title-row{display:flex;align-items:flex-start;justify-content:space-between;gap:20px;margin-bottom:22px}.at-panel-title-row svg{width:22px;height:22px;color:var(--at-accent)}
.at-panel-label{font-family:var(--tl-mono);font-size:11px;letter-spacing:.1em;text-transform:uppercase;color:var(--tl-muted2);margin-bottom:8px}
.at-panel-title{font-size:22px;line-height:1.18;font-weight:800;color:var(--tl-text);letter-spacing:-.02em}
.at-metric-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));border-top:1px solid var(--tl-border);border-left:1px solid var(--tl-border)}
.at-metric{min-height:122px;padding:16px;border-right:1px solid var(--tl-border);border-bottom:1px solid var(--tl-border);display:flex;flex-direction:column;justify-content:space-between;background:var(--tl-s2)}
.at-metric span{font-size:11px;text-transform:uppercase;letter-spacing:.08em;color:var(--tl-muted2);font-weight:700}.at-metric strong{font-size:28px;line-height:1;font-family:var(--tl-mono);letter-spacing:0;color:var(--tl-text)}.at-metric small{font-size:12px;color:var(--tl-muted);line-height:1.45}
.at-section{padding:54px 0;border-top:1px solid var(--tl-border)}
.at-scope-section,.at-related-section,.at-cta{display:grid;grid-template-columns:minmax(0,.8fr) minmax(320px,1.2fr);gap:32px;align-items:start}
.at-section-label{font-family:var(--tl-mono);font-size:11px;letter-spacing:.12em;text-transform:uppercase;color:var(--at-accent);font-weight:700;margin-bottom:12px}
.at-section h2,.at-section-heading h2,.at-cta h2{font-size:clamp(26px,3vw,40px);line-height:1.08;letter-spacing:-.035em;color:var(--tl-text);font-weight:850;margin:0}
.at-section-heading{max-width:720px;margin-bottom:28px}.at-section-heading p,.at-cta p{font-size:15px;line-height:1.7;color:var(--tl-muted);margin:14px 0 0}
.at-scope-list{display:flex;flex-wrap:wrap;gap:10px}.at-scope-list span{display:inline-flex;align-items:center;gap:8px;border:1px solid var(--at-line);background:var(--at-soft);border-radius:999px;padding:9px 12px;font-family:var(--tl-mono);font-size:12px;color:var(--tl-text)}.at-scope-list svg{width:14px;height:14px;color:var(--at-accent)}
.at-table-stack{display:grid;gap:22px}.at-table-card{border:1px solid var(--tl-border);border-radius:14px;background:var(--tl-s1);overflow:hidden;box-shadow:var(--tl-card-shadow)}
.at-table-head{padding:20px 22px;border-bottom:1px solid var(--tl-border)}.at-table-head h3{font-size:18px;letter-spacing:-.02em;color:var(--tl-text);margin:0 0 6px}.at-table-head p{font-size:13px;line-height:1.55;color:var(--tl-muted);margin:0}
.at-table-wrap{overflow-x:auto}.at-table-wrap table{width:100%;border-collapse:collapse;min-width:720px}.at-table-wrap th{padding:12px 14px;text-align:left;font-size:11px;letter-spacing:.08em;text-transform:uppercase;color:var(--tl-muted2);border-bottom:1px solid var(--tl-border);font-weight:750}.at-table-wrap td{padding:14px;border-bottom:1px solid var(--tl-border);font-size:13px;color:var(--tl-text);white-space:nowrap}.at-table-wrap td:not(:first-child){font-family:var(--tl-mono);font-size:12.5px;color:var(--tl-muted)}
.at-related-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:12px}.at-related-link{display:flex;align-items:center;justify-content:space-between;gap:16px;border:1px solid var(--tl-border);border-radius:10px;padding:14px 15px;background:var(--tl-s1);text-decoration:none;color:var(--tl-text);transition:transform .16s,border-color .16s,background .16s}.at-related-link:hover{transform:translateY(-1px);border-color:var(--at-line);background:var(--tl-s3)}.at-related-link span{display:inline-flex;align-items:center;gap:9px;font-weight:700}.at-related-link svg{width:16px;height:16px;color:var(--tl-muted2)}
.at-cta{border:1px solid var(--at-line);border-radius:16px;background:var(--tl-s3);padding:34px;margin-top:20px;box-shadow:var(--tl-card-shadow)}
.at-cta .at-actions{justify-content:flex-end}
@media(max-width:900px){.at-hero,.at-scope-section,.at-related-section,.at-cta{grid-template-columns:1fr}.at-hero{padding:56px 0 42px}.at-hero-panel{min-width:0}.at-cta .at-actions{justify-content:flex-start}}
@media(max-width:560px){.at-metric-grid{grid-template-columns:1fr}.at-panel-body{padding:18px}.at-cta{padding:24px}.at-page{padding-left:20px;padding-right:20px}}
`;
