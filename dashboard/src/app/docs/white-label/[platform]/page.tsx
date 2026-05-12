import Link from "next/link";
import { notFound } from "next/navigation";
import { DocsPage, DocsTable } from "../../_components/docs-shell";
import { WHITE_LABEL_GUIDES } from "./_data";

export default async function WhiteLabelPlatformGuidePage({
  params,
}: {
  params: Promise<{ platform: string }>;
}) {
  const { platform } = await params;
  const guide = WHITE_LABEL_GUIDES[platform];
  if (!guide) notFound();

  return (
    <DocsPage
      eyebrow="White-label Guide"
      title={guide.title}
      lead={guide.lead}
      className="docs-page-wide"
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="wlp-top-callout">
        <strong>Fastest route to a first success:</strong> create the app in{" "}
        <a href={guide.portalUrl} target="_blank" rel="noreferrer noopener">
          {guide.portalName}
        </a>
        , add the callback URL shown on this page, paste the credentials into UniPost, then connect one real test account before you expand scope or review work.
      </div>

      <h2 id="at-a-glance">At a glance</h2>
      <DocsTable
        columns={["Question", "Answer"]}
        rows={[
          ["Create the app in", <a key="portal" href={guide.portalUrl} target="_blank" rel="noreferrer noopener">{guide.portalName}</a>],
          ["UniPost credential card", guide.dashboardCard],
          ["Client fields to copy", `${guide.clientIdLabel} + ${guide.clientSecretLabel}`],
          ["Best for", guide.bestFor],
          ["App review / approval", guide.appReview],
        ]}
      />

      <h2 id="before-you-start">Before you start</h2>
      <ul className="docs-checklist">
        {guide.beforeYouStart.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>

      <h2 id="callback-urls">Callback URLs to whitelist</h2>
      <p className="wlp-note">
        Copy these exactly into the platform developer console. Redirect mismatches are the fastest way to burn time during setup.
      </p>
      <div className="wlp-code-list">
        {guide.callbacks.map((callback) => (
          <code key={callback}>{callback}</code>
        ))}
      </div>

      {guide.consoleSteps ? (
        <>
          <h2 id="developer-console-walkthrough">Developer console walkthrough</h2>
          <p className="wlp-note">
            Follow this exact click path in the platform console to get the credential pair UniPost needs. This section is intentionally written for the first-time setup path, not for people who already know where everything lives.
          </p>
          <div className="wlp-steps">
            {guide.consoleSteps.map((step, index) => (
              <div key={step.title} className="wlp-step-card">
                <div className="wlp-step-num">{index + 1}</div>
                <div className="wlp-step-body">
                  <div className="wlp-step-title">{step.title}</div>
                  <div className="wlp-step-copy">{step.body}</div>
                </div>
              </div>
            ))}
          </div>
        </>
      ) : null}

      <h2 id="first-working-setup">First working setup</h2>
      <div className="wlp-steps">
        {guide.steps.map((step, index) => (
          <div key={step.title} className="wlp-step-card">
            <div className="wlp-step-num">{index + 1}</div>
            <div className="wlp-step-body">
              <div className="wlp-step-title">{step.title}</div>
              <div className="wlp-step-copy">{step.body}</div>
            </div>
          </div>
        ))}
      </div>

      <h2 id="what-to-paste-into-unipost">What to paste into UniPost</h2>
      <DocsTable
        columns={guide.fieldMap[0]}
        rows={guide.fieldMap.slice(1)}
      />
      <p className="wlp-note">
        Save the credentials first, then start a fresh connection attempt. Troubleshooting an OAuth flow against stale credentials usually creates false leads.
      </p>

      <h2 id="common-blockers">Common blockers</h2>
      <DocsTable
        columns={["Blocker", "What to do about it"]}
        rows={guide.gotchas}
      />

      <h2 id="definition-of-done">Definition of done</h2>
      <ul className="docs-checklist">
        {guide.doneChecklist.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>

      <h2 id="next-steps">Next steps</h2>
      <div className="wlp-next">
        <Link href="/docs/white-label" className="wlp-next-card">
          <div className="wlp-next-kicker">Overview</div>
          <div className="wlp-next-title">White-label overview</div>
          <div className="wlp-next-body">
            Return to the main white-label guide for architecture, pricing, and setup flow.
          </div>
        </Link>
        <Link href="/docs/api/white-label/credentials" className="wlp-next-card">
          <div className="wlp-next-kicker">API reference</div>
          <div className="wlp-next-title">Platform credentials endpoint</div>
          <div className="wlp-next-body">
            Field-by-field behavior for storing and deleting credential pairs in UniPost.
          </div>
        </Link>
        <Link href={guide.relatedPlatformHref} className="wlp-next-card">
          <div className="wlp-next-kicker">Platform details</div>
          <div className="wlp-next-title">{guide.relatedPlatformTitle}</div>
          <div className="wlp-next-body">
            Publishing rules, media limits, and platform-specific post behavior after setup is complete.
          </div>
        </Link>
      </div>
    </DocsPage>
  );
}

const styles = `
.wlp-top-callout{margin:6px 0 24px;padding:16px 18px;border-radius:16px;background:color-mix(in srgb, var(--docs-link) 7%, var(--docs-bg-elevated));border:1px solid color-mix(in srgb, var(--docs-link) 18%, var(--docs-border));font-size:14.5px;line-height:1.7;color:var(--docs-text-soft)}
.wlp-top-callout strong{color:var(--docs-text)}
.wlp-top-callout a{color:var(--docs-link)}
.wlp-note{font-size:14px;line-height:1.65;color:var(--docs-text-soft);margin:8px 0 14px}
.wlp-code-list{display:grid;gap:10px;margin:14px 0 8px}
.wlp-code-list code{display:block;padding:14px 16px;border-radius:14px;border:1px solid var(--docs-border);background:var(--docs-bg-elevated);font-family:var(--docs-mono);font-size:13px;line-height:1.6;color:var(--docs-text-soft);overflow:auto}
.wlp-steps{display:grid;gap:12px;margin:14px 0 8px}
.wlp-step-card{display:grid;grid-template-columns:38px 1fr;gap:14px;align-items:start;padding:16px 18px;border-radius:16px;border:1px solid var(--docs-border);background:var(--docs-bg-elevated)}
.wlp-step-num{width:30px;height:30px;border-radius:999px;display:inline-flex;align-items:center;justify-content:center;background:color-mix(in srgb, var(--docs-link) 14%, var(--docs-bg-muted));border:1px solid color-mix(in srgb, var(--docs-link) 22%, var(--docs-border));color:var(--docs-link);font-size:13px;font-weight:700}
.wlp-step-title{font-size:15px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text);margin-bottom:4px}
.wlp-step-copy{font-size:14px;line-height:1.68;color:var(--docs-text-soft)}
.wlp-next{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px;margin:14px 0 4px}
.wlp-next-card{display:flex;flex-direction:column;gap:6px;padding:16px 18px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated);text-decoration:none;color:inherit;transition:border-color .15s ease,transform .15s ease,box-shadow .15s ease}
.wlp-next-card:hover{border-color:color-mix(in srgb, var(--docs-link) 38%, var(--docs-border));transform:translateY(-1px);box-shadow:var(--docs-card-shadow);text-decoration:none}
.wlp-next-kicker{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint)}
.wlp-next-title{font-size:16px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text)}
.wlp-next-body{font-size:13.5px;line-height:1.6;color:var(--docs-text-soft)}
@media (max-width:960px){
  .wlp-next{grid-template-columns:1fr}
}
`;
