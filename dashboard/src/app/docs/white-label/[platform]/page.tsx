"use client";

import Link from "next/link";
import { notFound, useParams } from "next/navigation";
import { useEffect, useState } from "react";
import { Check, Copy, X } from "lucide-react";
import { DocsCodeTabs, DocsPage, DocsTable, renderDocsRichContent } from "../../_components/docs-shell";
import { WHITE_LABEL_GUIDES } from "./_data";

function slugifyHeading(value: string) {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function CallbackUrlCard({ callback }: { callback: string }) {
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(callback);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1800);
    } catch {
      setCopied(false);
    }
  }

  return (
    <div className="wlp-callback-card">
      <code>{callback}</code>
      <button
        type="button"
        onClick={handleCopy}
        className={`wlp-copy-btn${copied ? " copied" : ""}`}
        aria-label="Copy callback URL"
      >
        {copied ? <Check size={14} /> : <Copy size={14} />}
        {copied ? "Copied" : "Copy"}
      </button>
    </div>
  );
}

export default function WhiteLabelPlatformGuidePage() {
  const params = useParams<{ platform: string }>();
  const guide = WHITE_LABEL_GUIDES[params.platform];
  const [zoomedImage, setZoomedImage] = useState<{ src: string; alt: string } | null>(null);
  if (!guide) notFound();

  useEffect(() => {
    if (!zoomedImage) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setZoomedImage(null);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [zoomedImage]);

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
          <CallbackUrlCard key={callback} callback={callback} />
        ))}
      </div>

      {guide.screenshotWalkthroughs ? (
        <>
          <h2 id="developer-console-walkthrough">Screenshot walkthrough</h2>
          {guide.screenshotWalkthroughs.map((group) => (
            <div key={group.title} className="wlp-shot-group">
              <h3 id={slugifyHeading(group.title)} className="wlp-shot-group-title">{group.title}</h3>
              {group.intro ? <p className="wlp-note">{group.intro}</p> : null}
              <div className="wlp-shot-list">
                {group.steps.map((step) => (
                  <div key={step.title} className="wlp-shot-card">
                    <div className="wlp-shot-title">{step.title}</div>
                    {step.caption ? <div className="wlp-shot-caption">{step.caption}</div> : null}
                    <button
                      type="button"
                      className="wlp-shot-trigger"
                      onClick={() => setZoomedImage({ src: step.image, alt: step.title })}
                      aria-label={`Open enlarged screenshot for ${step.title}`}
                    >
                      <img src={step.image} alt={step.title} className="wlp-shot-image" />
                    </button>
                    {step.snippets?.length ? (
                      <div className="wlp-step-code">
                        <DocsCodeTabs snippets={step.snippets} />
                      </div>
                    ) : null}
                  </div>
                ))}
              </div>
            </div>
          ))}
        </>
      ) : null}

      {!guide.screenshotWalkthroughs ? (
        <>
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
        </>
      ) : null}

      <h2 id="what-to-paste-into-unipost">What to paste into UniPost</h2>
      <DocsTable
        columns={guide.fieldMap[0]}
        rows={guide.fieldMap.slice(1)}
      />
      <p className="wlp-note">
        Save the credentials first, then start a fresh connection attempt. Troubleshooting an OAuth flow against stale credentials usually creates false leads.
      </p>

      {guide.verificationWorkflow ? (
        <>
          <h2 id="google-verification">{guide.verificationWorkflow.title}</h2>
          <p className="wlp-note">
            {guide.verificationWorkflow.intro}
          </p>
          <div className="wlp-steps">
            {guide.verificationWorkflow.steps.map((step, index) => (
              <div key={step.title} className="wlp-step-card">
                <div className="wlp-step-num">{index + 1}</div>
                <div className="wlp-step-body">
                  <div className="wlp-step-title">{step.title}</div>
                  <div className="wlp-step-copy">{renderDocsRichContent(step.body)}</div>
                </div>
              </div>
            ))}
          </div>
        </>
      ) : null}

      {guide.apiWorkflow ? (
        <>
          <h2 id="api-flow">{guide.apiWorkflow.title}</h2>
          <p className="wlp-note">
            {guide.apiWorkflow.intro}
          </p>
          <div className="wlp-steps">
            {guide.apiWorkflow.steps.map((step, index) => (
              <div key={step.title} className="wlp-step-card">
                <div className="wlp-step-num">{index + 1}</div>
                <div className="wlp-step-body">
                  <div className="wlp-step-title">{step.title}</div>
                  <div className="wlp-step-copy">{renderDocsRichContent(step.body)}</div>
                  {step.snippets?.length ? (
                    <div className="wlp-step-code">
                      <DocsCodeTabs snippets={step.snippets} />
                    </div>
                  ) : null}
                </div>
              </div>
            ))}
          </div>
        </>
      ) : null}

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

      {zoomedImage ? (
        <div
          className="wlp-lightbox"
          role="dialog"
          aria-modal="true"
          aria-label={zoomedImage.alt}
          onClick={() => setZoomedImage(null)}
        >
          <button
            type="button"
            className="wlp-lightbox-close"
            aria-label="Close enlarged screenshot"
            onClick={() => setZoomedImage(null)}
          >
            <X size={18} />
          </button>
          <div className="wlp-lightbox-inner" onClick={(event) => event.stopPropagation()}>
            <img src={zoomedImage.src} alt={zoomedImage.alt} className="wlp-lightbox-image" />
          </div>
        </div>
      ) : null}
    </DocsPage>
  );
}

const styles = `
.wlp-top-callout{margin:6px 0 24px;padding:16px 18px;border-radius:16px;background:color-mix(in srgb, var(--docs-link) 7%, var(--docs-bg-elevated));border:1px solid color-mix(in srgb, var(--docs-link) 18%, var(--docs-border));font-size:14.5px;line-height:1.7;color:var(--docs-text-soft)}
.wlp-top-callout strong{color:var(--docs-text)}
.wlp-top-callout a{color:var(--docs-link)}
.wlp-note{font-size:14px;line-height:1.65;color:var(--docs-text-soft);margin:8px 0 14px}
.wlp-code-list{display:grid;gap:10px;margin:14px 0 8px}
.wlp-callback-card{display:flex;align-items:center;justify-content:space-between;gap:14px;padding:14px 16px;border-radius:14px;border:1px solid var(--docs-border);background:var(--docs-bg-elevated)}
.wlp-callback-card code{display:block;min-width:0;overflow:auto;font-family:var(--docs-mono);font-size:13px;line-height:1.6;color:var(--docs-text-soft)}
.wlp-copy-btn{display:inline-flex;align-items:center;gap:8px;border:1px solid var(--docs-border);background:var(--docs-bg-muted);color:var(--docs-text-soft);border-radius:10px;padding:8px 12px;font-size:12px;font-weight:700;cursor:pointer;flex-shrink:0;transition:border-color .15s ease,background .15s ease,color .15s ease}
.wlp-copy-btn:hover{border-color:color-mix(in srgb, var(--docs-link) 28%, var(--docs-border));color:var(--docs-text)}
.wlp-copy-btn.copied{color:var(--docs-link);border-color:color-mix(in srgb, var(--docs-link) 36%, var(--docs-border));background:color-mix(in srgb, var(--docs-link) 8%, var(--docs-bg-muted))}
.wlp-shot-group{margin:14px 0 22px}
.wlp-shot-group-title{font-size:20px;line-height:1.3;letter-spacing:-.02em;color:var(--docs-text);margin:0 0 8px}
.wlp-shot-list{display:grid;gap:18px;margin:14px 0 8px}
.wlp-shot-card{padding:16px 16px 18px;border-radius:18px;border:1px solid var(--docs-border);background:var(--docs-bg-elevated)}
.wlp-shot-trigger{display:block;width:100%;padding:0;border:none;background:transparent;cursor:zoom-in}
.wlp-shot-title{font-size:16px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text);margin-bottom:8px}
.wlp-shot-caption{font-size:13px;line-height:1.6;color:var(--docs-text-soft);margin-bottom:12px}
.wlp-shot-caption code{font-family:var(--docs-mono);font-size:12px}
.wlp-shot-image{display:block;width:100%;height:auto;border-radius:14px;border:1px solid var(--docs-border-strong);background:#111}
.wlp-steps{display:grid;gap:12px;margin:14px 0 8px}
.wlp-step-card{display:grid;grid-template-columns:38px 1fr;gap:14px;align-items:start;padding:16px 18px;border-radius:16px;border:1px solid var(--docs-border);background:var(--docs-bg-elevated)}
.wlp-step-num{width:30px;height:30px;border-radius:999px;display:inline-flex;align-items:center;justify-content:center;background:color-mix(in srgb, var(--docs-link) 14%, var(--docs-bg-muted));border:1px solid color-mix(in srgb, var(--docs-link) 22%, var(--docs-border));color:var(--docs-link);font-size:13px;font-weight:700}
.wlp-step-title{font-size:15px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text);margin-bottom:4px}
.wlp-step-copy{font-size:14px;line-height:1.68;color:var(--docs-text-soft)}
.wlp-step-code{margin-top:14px}
.wlp-next{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px;margin:14px 0 4px}
.wlp-next-card{display:flex;flex-direction:column;gap:6px;padding:16px 18px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated);text-decoration:none;color:inherit;transition:border-color .15s ease,transform .15s ease,box-shadow .15s ease}
.wlp-next-card:hover{border-color:color-mix(in srgb, var(--docs-link) 38%, var(--docs-border));transform:translateY(-1px);box-shadow:var(--docs-card-shadow);text-decoration:none}
.wlp-next-kicker{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint)}
.wlp-next-title{font-size:16px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text)}
.wlp-next-body{font-size:13.5px;line-height:1.6;color:var(--docs-text-soft)}
.wlp-lightbox{position:fixed;inset:0;z-index:80;display:flex;align-items:center;justify-content:center;padding:28px;background:rgba(5,10,18,.82);backdrop-filter:blur(10px)}
.wlp-lightbox-inner{max-width:min(1480px,100%);max-height:100%;display:flex;align-items:center;justify-content:center}
.wlp-lightbox-image{display:block;max-width:100%;max-height:calc(100vh - 56px);border-radius:18px;border:1px solid var(--docs-border-strong);box-shadow:0 30px 80px rgba(0,0,0,.4);background:#111}
.wlp-lightbox-close{position:absolute;top:18px;right:18px;display:inline-flex;align-items:center;justify-content:center;width:40px;height:40px;border-radius:999px;border:1px solid color-mix(in srgb, #fff 14%, transparent);background:rgba(17,22,31,.82);color:#fff;cursor:pointer}
.wlp-lightbox-close:hover{background:rgba(24,30,42,.92)}
@media (max-width:960px){
  .wlp-callback-card{flex-direction:column;align-items:stretch}
  .wlp-copy-btn{justify-content:center}
  .wlp-next{grid-template-columns:1fr}
}
`;
