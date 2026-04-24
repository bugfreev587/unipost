import Link from "next/link";
import { notFound } from "next/navigation";
import { DocsCodeTabs, DocsPage, DocsRichText, DocsTable } from "../../_components/docs-shell";
import { ApiInlineLink } from "../../api/_components/doc-components";
import { PLATFORMS, type PlatformSummary } from "./_data";
import { toExampleSnippets } from "./_snippets";

const SUMMARY_LABELS: Record<keyof Omit<PlatformSummary, "connection">, string> = {
  publishing: "Publishing",
  scheduling: "Scheduling",
  analytics: "Analytics",
  inbox: "Inbox",
};

function summaryBadge(value: "full" | "limited" | "none") {
  if (value === "full") return "Supported";
  if (value === "limited") return "Limited";
  return "Not supported";
}

function summaryTone(value: "full" | "limited" | "none") {
  if (value === "full") return "ok";
  if (value === "limited") return "warn";
  return "muted";
}

export default async function PlatformDetailPage({
  params,
}: {
  params: Promise<{ platform: string }>;
}) {
  const { platform } = await params;
  const data = PLATFORMS[platform];
  if (!data) notFound();

  const supportsManagedUploads = data.requirements.some(
    (row) => row[0].includes("media_urls") || row[0].includes("media_ids"),
  );

  return (
    <DocsPage
      eyebrow="Platform Guide"
      title={data.title}
      lead={data.lead}
      className="docs-page-wide"
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="plat-hero">
        <div
          className="plat-hero-icon"
          style={{ ["--plat-brand" as never]: data.brandColor }}
        >
          {data.icon}
        </div>
        <div className="plat-hero-body">
          <div className="plat-hero-tagline">{data.tagline}</div>
          <div className="plat-hero-badges">
            {data.badges.map((badge) => (
              <span className="plat-badge" key={badge}>
                {badge}
              </span>
            ))}
          </div>
        </div>
      </div>

      <h2 id="at-a-glance">At a glance</h2>
      <div className="plat-summary-grid">
        {(Object.keys(SUMMARY_LABELS) as (keyof typeof SUMMARY_LABELS)[]).map((key) => {
          const value = data.summary[key];
          return (
            <div key={key} className={`plat-summary-card tone-${summaryTone(value)}`}>
              <div className="plat-summary-label">{SUMMARY_LABELS[key]}</div>
              <div className="plat-summary-value">{summaryBadge(value)}</div>
            </div>
          );
        })}
        <div className="plat-summary-card plat-summary-wide">
          <div className="plat-summary-label">Connection</div>
          <div className="plat-summary-value plat-summary-value-text">
            {data.summary.connection}
          </div>
        </div>
      </div>

      <h2 id="feature-matrix">Feature matrix</h2>
      <DocsTable columns={["Feature", "Support", "Notes"]} rows={data.capabilities} />

      <h2 id="media-requirements">Media &amp; field requirements</h2>
      <DocsTable
        columns={["Field", "Required", "Limits", "Notes"]}
        rows={data.requirements}
      />
      {supportsManagedUploads ? (
        <p className="plat-note">
          Hosted URLs: pass the public URL in <code>media_urls</code>. Local files:
          reserve an upload with <ApiInlineLink endpoint="POST /v1/media" />, PUT the
          bytes to the returned <code>upload_url</code>, then publish with{" "}
          <code>media_ids</code>. Full flow in{" "}
          <Link href="/docs/api/media">Media API</Link>.
        </p>
      ) : null}

      {data.mediaSpecs ? (
        <>
          <h2 id="media-specs">Media specifications</h2>
          <p className="plat-note">
            Per-surface limits for text, images, and video. These are the source of
            truth UniPost uses for preflight validation and media optimization —
            treat hard-limit values as enforced and &quot;recommended&quot; values as
            platform guidance.
          </p>
          {data.mediaSpecs.map((spec) => (
            <div key={spec.surface} className="plat-spec-block">
              <h3
                id={`media-specs-${spec.surface.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`}
                className="plat-spec-heading"
              >
                {spec.surface}
              </h3>
              {spec.description ? (
                <p className="plat-note">{spec.description}</p>
              ) : null}
              {spec.text ? (
                <DocsTable
                  columns={["Text", "Value"]}
                  rows={spec.text.map((row) => [row[0], row[1]])}
                />
              ) : null}
              {spec.image ? (
                <DocsTable
                  columns={["Image", "Value"]}
                  rows={spec.image.map((row) => [row[0], row[1]])}
                />
              ) : null}
              {spec.video ? (
                <DocsTable
                  columns={["Video", "Value"]}
                  rows={spec.video.map((row) => [row[0], row[1]])}
                />
              ) : null}
            </div>
          ))}
        </>
      ) : null}

      {data.options ? (
        <>
          <h2 id="platform-options">Platform-specific options</h2>
          <DocsTable columns={["Option", "Values", "Notes"]} rows={data.options} />
        </>
      ) : null}

      <h2 id="analytics">Analytics</h2>
      <DocsTable columns={["Metric", "Support", "Notes"]} rows={data.analytics} />

      {data.inbox ? (
        <>
          <h2 id="inbox">Inbox</h2>
          {data.inbox.note ? <p className="plat-note">{data.inbox.note}</p> : null}
          <DocsTable
            columns={["Surface", "Support", "Notes"]}
            rows={data.inbox.rows}
          />
        </>
      ) : null}

      <h2 id="setup-modes">Connection modes</h2>
      <p className="plat-note">
        Pick the setup that matches how the account is owned. Quickstart is fastest
        when you publish to your own accounts; White-label is required when your
        customers bring their own accounts through a branded flow. Full setup
        details in <Link href="/docs/quickstart">Quickstart</Link> and{" "}
        <Link href="/docs/white-label">White-label</Link>.
      </p>
      <DocsTable
        columns={["Mode", "Best for", "App / credentials", "Availability"]}
        rows={data.setup}
      />

      <h2 id="api-examples">API examples</h2>
      <p className="plat-note">
        Each example calls <ApiInlineLink endpoint="POST /v1/posts" /> with Bearer
        auth. Swap the <code>account_ids</code> for your own, then copy the snippet
        for your language.
      </p>
      {data.examples.map((example) => (
        <div key={example.title}>
          <h3
            id={example.title.toLowerCase().replace(/[^a-z0-9]+/g, "-")}
            className="plat-example-title"
          >
            {example.title}
          </h3>
          {example.note ? (
            <p className="plat-note">
              <DocsRichText text={example.note} />
            </p>
          ) : null}
          <DocsCodeTabs snippets={toExampleSnippets(example.body)} />
        </div>
      ))}

      <h2 id="limitations">Limitations</h2>
      <DocsTable columns={["Limitation", "Why"]} rows={data.limitations} />

      <h2 id="validation-errors">Validation errors</h2>
      <DocsTable columns={["Code", "What it means"]} rows={data.errors} />

      <h2 id="next-steps">Next steps</h2>
      <div className="plat-next">
        <Link href="/docs/quickstart" className="plat-next-card">
          <div className="plat-next-kicker">Start publishing</div>
          <div className="plat-next-title">Quickstart</div>
          <div className="plat-next-body">
            Get an API key, connect this platform, and send your first post.
          </div>
        </Link>
        <Link href="/docs/api/posts/create" className="plat-next-card">
          <div className="plat-next-kicker">API reference</div>
          <div className="plat-next-title">Create post</div>
          <div className="plat-next-body">
            Full request / response schema for the publish endpoint.
          </div>
        </Link>
        <Link href="/docs/api/posts/validate" className="plat-next-card">
          <div className="plat-next-kicker">Preflight</div>
          <div className="plat-next-title">Validate post</div>
          <div className="plat-next-body">
            Catch caption and media issues before you hit publish.
          </div>
        </Link>
        <Link href="/docs/white-label" className="plat-next-card">
          <div className="plat-next-kicker">For customer accounts</div>
          <div className="plat-next-title">White-label</div>
          <div className="plat-next-body">
            Branded Connect flows that run against your own OAuth app.
          </div>
        </Link>
      </div>
    </DocsPage>
  );
}

const styles = `
.plat-hero{display:flex;align-items:center;gap:18px;margin:6px 0 28px;padding:16px 18px;border:1px solid var(--docs-border);border-radius:18px;background:var(--docs-bg-elevated);box-shadow:0 1px 0 rgba(255,255,255,.02)}
.plat-hero-icon{display:inline-flex;align-items:center;justify-content:center;width:56px;height:56px;border-radius:16px;background:color-mix(in srgb, var(--plat-brand, var(--docs-link)) 12%, var(--docs-bg-muted));color:var(--plat-brand, var(--docs-text));flex:none}
.plat-hero-body{display:flex;flex-direction:column;gap:10px;min-width:0}
.plat-hero-tagline{font-size:15px;line-height:1.55;color:var(--docs-text-soft)}
.plat-hero-badges{display:flex;flex-wrap:wrap;gap:6px}
.plat-badge{display:inline-flex;align-items:center;padding:3px 10px;border-radius:999px;background:var(--docs-bg-muted);border:1px solid var(--docs-border);color:var(--docs-text);font-size:11.5px;font-weight:600;letter-spacing:.01em}
.plat-summary-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;margin:18px 0 22px}
.plat-summary-card{padding:14px 16px;border:1px solid var(--docs-border);border-radius:14px;background:var(--docs-bg-elevated);min-width:0}
.plat-summary-wide{grid-column:span 4}
.plat-summary-label{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint);margin-bottom:6px}
.plat-summary-value{font-size:15px;font-weight:700;letter-spacing:-.01em;color:var(--docs-text)}
.plat-summary-value-text{font-size:14px;font-weight:500;color:var(--docs-text-soft);line-height:1.55}
.tone-ok .plat-summary-value{color:#16a34a}
.tone-warn .plat-summary-value{color:#d97706}
.tone-muted .plat-summary-value{color:var(--docs-text-faint)}
.plat-note{font-size:14px;line-height:1.65;color:var(--docs-text-soft);margin:10px 0 18px;max-width:none}
.plat-example-title{margin-top:22px;margin-bottom:8px;font-size:15.5px;letter-spacing:-.015em}
.plat-spec-block{margin:6px 0 22px}
.plat-spec-block .docs-table-wrap+.docs-table-wrap{margin-top:10px}
.plat-spec-heading{margin-top:22px;margin-bottom:8px;font-size:15.5px;letter-spacing:-.015em}
.plat-next{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin:18px 0 4px}
.plat-next-card{display:flex;flex-direction:column;gap:6px;padding:16px 18px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated);text-decoration:none;color:inherit;transition:border-color .15s ease,transform .15s ease,box-shadow .15s ease}
.plat-next-card:hover{border-color:color-mix(in srgb, var(--docs-link) 38%, var(--docs-border));transform:translateY(-1px);box-shadow:var(--docs-card-shadow);text-decoration:none}
.plat-next-kicker{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint)}
.plat-next-title{font-size:16px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text)}
.plat-next-body{font-size:13.5px;line-height:1.6;color:var(--docs-text-soft)}
@media (max-width:960px){
  .plat-hero{flex-direction:column;align-items:flex-start}
  .plat-summary-grid{grid-template-columns:repeat(2,minmax(0,1fr))}
  .plat-summary-wide{grid-column:span 2}
  .plat-next{grid-template-columns:1fr}
}
`;
