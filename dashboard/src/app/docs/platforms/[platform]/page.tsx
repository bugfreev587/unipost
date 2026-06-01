import Link from "next/link";
import { notFound } from "next/navigation";
import { DocsCodeTabs, DocsPage, DocsRichText, DocsTable } from "../../_components/docs-shell";
import { ApiInlineLink } from "../../api/_components/doc-components";
import { PLATFORMS, type PlatformDoc, type PlatformSummary } from "./_data";
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

function PlatformApiExamples({
  examples,
  headingLevel = 2,
  title = "API Examples",
}: {
  examples: PlatformDoc["examples"];
  headingLevel?: 2 | 3;
  title?: string;
}) {
  const Heading = headingLevel === 3 ? "h3" : "h2";
  const renderExampleTitle = (title: string) => {
    const id = title.toLowerCase().replace(/[^a-z0-9]+/g, "-");

    if (headingLevel === 3) {
      return (
        <div
          id={id}
          style={{
            color: "var(--docs-text)",
            fontSize: 18,
            fontWeight: 680,
            letterSpacing: "-.015em",
            lineHeight: 1.3,
            margin: "22px 0 10px",
          }}
        >
          {title}
        </div>
      );
    }

    return (
      <h3 id={id}>
        {title}
      </h3>
    );
  };

  return (
    <>
      <Heading id="api-examples">{title}</Heading>
      <p className="docs-note">
        Each example calls <ApiInlineLink endpoint="POST /v1/posts" /> with Bearer
        auth. Swap the <code>account_ids</code> for your own, then copy the snippet
        for your language.
      </p>
      {examples.map((example) => (
        <div key={example.title}>
          {renderExampleTitle(example.title)}
          {example.note ? (
            <p className="docs-note">
              <DocsRichText text={example.note} />
            </p>
          ) : null}
          <DocsCodeTabs snippets={toExampleSnippets(example.body)} />
        </div>
      ))}
    </>
  );
}

function mediaSpecId(surface: string) {
  return `media-specs-${surface.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`;
}

function mediaSpecRows(spec: NonNullable<PlatformDoc["mediaSpecs"]>[number]) {
  return [
    ...(spec.text ?? []).map((row) => ["Text", row[0], row[1]] as const),
    ...(spec.image ?? []).map((row) => ["Image", row[0], row[1]] as const),
    ...(spec.video ?? []).map((row) => ["Video", row[0], row[1]] as const),
  ];
}

function MediaSpecsSection({ data }: { data: PlatformDoc }) {
  if (!data.mediaSpecs) return null;

  return (
    <>
      <h2 id="media-specs">Media specifications</h2>
      <p className="docs-note">
        Per-surface limits for text, images, and video. These are the source of
        truth UniPost uses for preflight validation and media optimization —
        treat hard-limit values as enforced and &quot;recommended&quot; values as
        platform guidance.
      </p>
      <div className="docs-surface-tabs" aria-label={`${data.title} media surfaces`}>
        {data.mediaSpecs.map((spec) => (
          <a className="docs-surface-tab" href={`#${mediaSpecId(spec.surface)}`} key={spec.surface}>
            {spec.surface}
          </a>
        ))}
      </div>
      {data.mediaSpecs.map((spec) => (
        <section className="docs-surface-panel" key={spec.surface}>
          <h3 id={mediaSpecId(spec.surface)}>{spec.surface}</h3>
          {spec.description ? (
            <p className="docs-note">{spec.description}</p>
          ) : null}
          <DocsTable
            columns={["Type", "Requirement", "Value"]}
            rows={mediaSpecRows(spec)}
          />
        </section>
      ))}
    </>
  );
}

function PlatformNextSteps({
  data,
  platform,
}: {
  data: PlatformDoc;
  platform: string;
}) {
  return (
    <div className="docs-next-grid">
      <Link href="/docs/publishing" className="docs-next-card">
        <div className="docs-next-kicker">Shared flow</div>
        <div className="docs-next-title">Follow the Publishing guide</div>
        <div className="docs-next-body">
          Choose hosted URLs or local file uploads, then track the async result.
        </div>
      </Link>
      <Link href={`/docs/platforms/${platform}#api-examples`} className="docs-next-card">
        <div className="docs-next-kicker">Payload shape</div>
        <div className="docs-next-title">Start from a {data.title} example</div>
        <div className="docs-next-body">
          Pick the closest payload shape and swap in your own account, caption, and media.
        </div>
      </Link>
      <Link href="/docs/api/posts/create#publishing-result" className="docs-next-card">
        <div className="docs-next-kicker">Async result</div>
        <div className="docs-next-title">Track publishing status</div>
        <div className="docs-next-body">
          Read the final post result after UniPost accepts the publish request.
        </div>
      </Link>
      <Link href="/docs/api/webhooks" className="docs-next-card">
        <div className="docs-next-kicker">Push delivery</div>
        <div className="docs-next-title">Set up developer webhooks</div>
        <div className="docs-next-body">
          Receive post.published, post.partial, and post.failed events in your
          backend.
        </div>
      </Link>
      <Link href="/docs/connect-sessions" className="docs-next-card">
        <div className="docs-next-kicker">Customer accounts</div>
        <div className="docs-next-title">Plan account connection</div>
        <div className="docs-next-body">
          Use Connect Sessions for customer-owned accounts, with shared OAuth
          fallback or workspace Platform Credentials.
        </div>
      </Link>
    </div>
  );
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
        </div>
      </div>

      {data.setupNote ? (
        <div className="docs-callout docs-callout-warning docs-callout-compact">
          <strong>{data.title} account requirement</strong>
          {data.setupNote}
        </div>
      ) : null}

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
      </div>
      <div className="docs-summary-connection">
        <div className="docs-summary-label">Connection</div>
        <div className="docs-summary-copy">
          {data.summary.connection}
        </div>
      </div>

      <h2 id="feature-matrix">Feature matrix</h2>
      <DocsTable columns={["Feature", "Support", "Notes"]} rows={data.capabilities} />

      <h2 id="limitations">Known constraints</h2>
      <DocsTable columns={["Limitation", "Why"]} rows={data.limitations} />

      <h2 id="post-api-guide">Publishing</h2>
      <div className="docs-callout docs-callout-info docs-callout-compact">
        <strong>Ready to publish?</strong>
        Use the shared <Link href="/docs/publishing">Publishing guide</Link> for
        hosted URLs, local file uploads, preflight validation, and async publish
        status. Then use the {data.title} examples below for platform-specific
        payload shape.
      </div>

      <PlatformApiExamples
        examples={data.examples}
        headingLevel={2}
        title="Publish examples by surface"
      />

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
          <code>media_ids</code>. Full flow in the{" "}
          <Link href="/docs/publishing#local-file-flow">Publishing guide</Link>.
        </p>
      ) : null}

      <MediaSpecsSection data={data} />

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
        when you publish to your own accounts; Connect Sessions are for
        customer-owned account onboarding; Hosted Connect branding controls the
        UniPost-hosted pre-OAuth page, while Platform Credentials control the
        upstream app identity and quota. Full setup details in <Link href="/docs/quickstart">Quickstart</Link>,{" "}
        <Link href="/docs/connect-sessions">Connect Sessions</Link>, and{" "}
        <Link href="/docs/white-label">White-label</Link>.
      </p>
      <DocsTable
        columns={["Mode", "Best for", "App / credentials", "Availability"]}
        rows={data.setup}
      />
      {data.setupNote ? <p className="docs-note">{data.setupNote}</p> : null}

      <h2 id="validation-errors">Validation errors</h2>
      <DocsTable columns={["Code", "What it means"]} rows={data.errors} />

      <h2 id="next-steps">Next steps</h2>
      <PlatformNextSteps data={data} platform={platform} />
    </DocsPage>
  );
}
