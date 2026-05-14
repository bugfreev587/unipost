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
      <div className="docs-guide-intro">
        <div
          className="docs-guide-intro-icon"
          style={{ color: data.brandColor }}
        >
          {data.icon}
        </div>
        <div className="docs-guide-intro-body">
          <div className="docs-guide-intro-title">{data.tagline}</div>
          <div className="docs-badge-row">
            {data.badges.map((badge) => (
              <span className="docs-badge" key={badge}>
                {badge}
              </span>
            ))}
          </div>
        </div>
      </div>

      <h2 id="at-a-glance">At a glance</h2>
      <div className="docs-summary-grid">
        {(Object.keys(SUMMARY_LABELS) as (keyof typeof SUMMARY_LABELS)[]).map((key) => {
          const value = data.summary[key];
          return (
            <div key={key} className={`docs-summary-card tone-${summaryTone(value)}`}>
              <div className="docs-summary-label">{SUMMARY_LABELS[key]}</div>
              <div className="docs-summary-value">{summaryBadge(value)}</div>
            </div>
          );
        })}
        <div className="docs-summary-card docs-summary-card-wide">
          <div className="docs-summary-label">Connection</div>
          <div className="docs-summary-copy">
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
        <p className="docs-note">
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
          <p className="docs-note">
            Per-surface limits for text, images, and video. These are the source of
            truth UniPost uses for preflight validation and media optimization —
            treat hard-limit values as enforced and &quot;recommended&quot; values as
            platform guidance.
          </p>
          {data.mediaSpecs.map((spec) => (
            <section key={spec.surface}>
              <h3
                id={`media-specs-${spec.surface.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`}
              >
                {spec.surface}
              </h3>
              {spec.description ? (
                <p className="docs-note">{spec.description}</p>
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
            </section>
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
          {data.inbox.note ? <p className="docs-note">{data.inbox.note}</p> : null}
          <DocsTable
            columns={["Surface", "Support", "Notes"]}
            rows={data.inbox.rows}
          />
        </>
      ) : null}

      <h2 id="setup-modes">Connection modes</h2>
      <p className="docs-note">
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
      <p className="docs-note">
        Each example calls <ApiInlineLink endpoint="POST /v1/posts" /> with Bearer
        auth. Swap the <code>account_ids</code> for your own, then copy the snippet
        for your language.
      </p>
      {data.examples.map((example) => (
        <div key={example.title}>
          <h3
            id={example.title.toLowerCase().replace(/[^a-z0-9]+/g, "-")}
          >
            {example.title}
          </h3>
          {example.note ? (
            <p className="docs-note">
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
      <div className="docs-next-grid">
        <Link href="/docs/quickstart" className="docs-next-card">
          <div className="docs-next-kicker">Start publishing</div>
          <div className="docs-next-title">Quickstart</div>
          <div className="docs-next-body">
            Get an API key, connect this platform, and send your first post.
          </div>
        </Link>
        <Link href="/docs/api/posts/create" className="docs-next-card">
          <div className="docs-next-kicker">API reference</div>
          <div className="docs-next-title">Create post</div>
          <div className="docs-next-body">
            Full request / response schema for the publish endpoint.
          </div>
        </Link>
        <Link href="/docs/api/posts/validate" className="docs-next-card">
          <div className="docs-next-kicker">Preflight</div>
          <div className="docs-next-title">Validate post</div>
          <div className="docs-next-body">
            Catch caption and media issues before you hit publish.
          </div>
        </Link>
        <Link href="/docs/white-label" className="docs-next-card">
          <div className="docs-next-kicker">For customer accounts</div>
          <div className="docs-next-title">White-label</div>
          <div className="docs-next-body">
            Branded Connect flows that run against your own OAuth app.
          </div>
        </Link>
      </div>
    </DocsPage>
  );
}
