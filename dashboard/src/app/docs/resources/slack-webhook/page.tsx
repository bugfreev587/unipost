import type { Metadata } from "next";
import Link from "next/link";
import { DocsPage, DocsTable } from "../../_components/docs-shell";

export const metadata: Metadata = {
  title: "Slack Webhook URL | UniPost Docs",
  description: "Step-by-step Slack UI guide to create an incoming webhook URL for UniPost notifications.",
  keywords: ["slack webhook url", "unipost slack notifications", "slack incoming webhooks"],
};

const STEPS = [
  {
    n: "1",
    title: "Open Slack apps and create a new app",
    body: (
      <>
        Open <a href="https://api.slack.com/apps" target="_blank" rel="noreferrer">api.slack.com/apps</a> and click <strong>Create an App</strong>.
      </>
    ),
    img: "/docs/slack-webhook/step1.png",
    alt: "Slack API Your Apps page with the Create an App button highlighted.",
  },
  {
    n: "2",
    title: "Choose From scratch",
    body: <>In the create-app dialog, choose <strong>From scratch</strong>.</>,
    img: "/docs/slack-webhook/step2.png",
    alt: "Slack create app dialog with From scratch highlighted.",
  },
  {
    n: "3",
    title: "Name the app and choose the workspace",
    body: <>Enter a name, pick the Slack workspace, then click <strong>Create App</strong>.</>,
    img: "/docs/slack-webhook/step3.png",
    alt: "Slack dialog for naming the app and choosing a workspace.",
  },
  {
    n: "4",
    title: "Open Incoming Webhooks and enable it",
    body: <>In the app sidebar, click <strong>Incoming Webhooks</strong>, then turn on <strong>Activate Incoming Webhooks</strong>.</>,
    img: "/docs/slack-webhook/step4.png",
    alt: "Slack app Incoming Webhooks settings page with the toggle enabled.",
  },
  {
    n: "5",
    title: "Add a new webhook",
    body: <>Scroll to <strong>Webhook URLs for Your Workspace</strong> and click <strong>Add New Webhook</strong>.</>,
    img: "/docs/slack-webhook/step5.png",
    alt: "Slack Incoming Webhooks page with the Add New Webhook button highlighted.",
  },
  {
    n: "6",
    title: "Choose the channel and allow access",
    body: <>Select the Slack channel that should receive UniPost alerts, then click <strong>Allow</strong>.</>,
    img: "/docs/slack-webhook/step6.png",
    alt: "Slack permission screen with channel selection and Allow button highlighted.",
  },
  {
    n: "7",
    title: "Copy the webhook URL",
    body: <>Back on the Incoming Webhooks page, click <strong>Copy</strong> next to the webhook you just created.</>,
    img: "/docs/slack-webhook/step7.png",
    alt: "Slack Incoming Webhooks page showing a generated webhook URL and copy button.",
  },
];

export default function SlackWebhookPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Resources · Notifications"
      title="Slack Webhook URL"
      lead="Create a Slack incoming webhook and paste it into UniPost. Seven clicks in Slack, one paste in UniPost."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="sw-badges">
        <span className="sw-badge">~2 min</span>
        <span className="sw-badge">No admin required</span>
        <span className="sw-badge">One-time setup</span>
      </div>

      <h2 id="at-a-glance">At a glance</h2>
      <DocsTable
        columns={["Question", "Answer"]}
        rows={[
          ["Who needs it", "Anyone who wants UniPost alerts in a Slack channel"],
          ["Where the webhook lives", "A Slack app with Incoming Webhooks enabled"],
          ["Expected URL format", "`https://hooks.slack.com/services/...`"],
          ["Setup time", "~2 minutes total"],
          ["Reversible?", "Yes — delete the webhook in Slack or remove the channel in UniPost"],
        ]}
      />

      <div className="docs-callout">
        <strong>Heads up:</strong> UniPost only accepts URLs starting with <code>https://hooks.slack.com/</code>. Anything else is rejected at save time.
      </div>

      <h2 id="steps">Steps in Slack</h2>
      <ol className="sw-steps">
        {STEPS.map((step) => (
          <li key={step.n} className="sw-step">
            <div className="sw-step-head">
              <div className="sw-step-num">{step.n}</div>
              <div className="sw-step-title">{step.title}</div>
            </div>
            <div className="sw-step-body">{step.body}</div>
            <div className="sw-step-img">
              <img src={step.img} alt={step.alt} />
            </div>
          </li>
        ))}
      </ol>

      <h2 id="paste-into-unipost">Paste the URL into UniPost</h2>
      <ul className="docs-checklist">
        <li>Open <Link href="/settings/notifications">Settings → Notifications</Link></li>
        <li>Click <strong>Add channel → Slack Webhook</strong></li>
        <li>Paste the webhook URL from Slack</li>
        <li>Optional — add a label such as <code>#ops-alerts</code></li>
        <li>Save, then click <strong>Test</strong></li>
      </ul>
      <div className="docs-callout">
        <strong>Next:</strong> after the channel shows as <strong>Verified</strong>, open the <Link href="/docs/resources/notifications#subscribe-events">Subscriptions</Link> table and turn on the alerts you want.
      </div>

      <h2 id="troubleshooting">Troubleshooting</h2>
      <DocsTable
        columns={["Symptom", "Likely cause", "Fix"]}
        rows={[
          ["UniPost rejects the URL at save", "URL does not start with `https://hooks.slack.com/`", "Copy the Webhook URL from Slack's Incoming Webhooks page — not the OAuth or app URL"],
          ["Test sends but nothing shows in Slack", "The Slack channel was archived or the webhook was revoked", "Open the Slack app's Incoming Webhooks page and confirm the webhook still exists"],
          ["Only some events land", "Event subscription is off for that channel", "Toggle the event on in the Subscriptions table in UniPost"],
        ]}
      />

      <h2 id="next-steps">Next steps</h2>
      <div className="sw-next">
        <Link href="/docs/resources/notifications" className="sw-next-card">
          <div className="sw-next-kicker">Overview</div>
          <div className="sw-next-title">Notifications overview</div>
          <div className="sw-next-body">Channels, events, and which ones are on by default.</div>
        </Link>
        <Link href="/docs/resources/discord-webhook" className="sw-next-card">
          <div className="sw-next-kicker">Also available</div>
          <div className="sw-next-title">Discord Webhook URL</div>
          <div className="sw-next-body">Same setup shape for a Discord channel.</div>
        </Link>
        <a href="https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/" target="_blank" rel="noreferrer" className="sw-next-card">
          <div className="sw-next-kicker">Slack docs</div>
          <div className="sw-next-title">Incoming Webhooks reference</div>
          <div className="sw-next-body">Slack's official deep-dive on webhook payloads and limits.</div>
        </a>
        <Link href="/settings/notifications" className="sw-next-card">
          <div className="sw-next-kicker">Configure</div>
          <div className="sw-next-title">Open Notifications settings</div>
          <div className="sw-next-body">Add the channel and manage subscriptions.</div>
        </Link>
      </div>
    </DocsPage>
  );
}

const styles = `
.sw-badges{display:flex;flex-wrap:wrap;gap:6px;margin:2px 0 18px}
.sw-badge{display:inline-flex;align-items:center;padding:4px 11px;border-radius:999px;background:var(--docs-bg-muted);border:1px solid var(--docs-border);color:var(--docs-text);font-size:11.5px;font-weight:600;letter-spacing:.01em}
.sw-steps{list-style:none;padding:0;margin:14px 0 6px;display:grid;grid-template-columns:1fr;gap:18px;counter-reset:sw-step}
.sw-step{padding:18px 20px 20px;border:1px solid var(--docs-border);border-radius:18px;background:var(--docs-bg-elevated);box-shadow:0 1px 0 rgba(255,255,255,.02)}
.sw-step-head{display:flex;align-items:center;gap:12px;margin-bottom:8px}
.sw-step-num{display:inline-flex;align-items:center;justify-content:center;width:30px;height:30px;border-radius:999px;background:color-mix(in srgb, var(--docs-link) 14%, var(--docs-bg-muted));color:var(--docs-link);font-size:14px;font-weight:700;border:1px solid color-mix(in srgb, var(--docs-link) 22%, var(--docs-border));flex:none}
.sw-step-title{font-size:17px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text)}
.sw-step-body{font-size:14.5px;line-height:1.7;color:var(--docs-text-soft);margin-bottom:12px}
.sw-step-body code{font-family:var(--docs-mono);font-size:12.5px}
.sw-step-img{border:1px solid var(--docs-border);border-radius:14px;overflow:hidden;background:var(--docs-bg-muted)}
.sw-step-img img{display:block;width:100%;height:auto}
.sw-next{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin:14px 0 4px}
.sw-next-card{display:flex;flex-direction:column;gap:6px;padding:16px 18px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated);text-decoration:none;color:inherit;transition:border-color .15s ease,transform .15s ease,box-shadow .15s ease}
.sw-next-card:hover{border-color:color-mix(in srgb, var(--docs-link) 38%, var(--docs-border));transform:translateY(-1px);box-shadow:var(--docs-card-shadow);text-decoration:none}
.sw-next-kicker{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint)}
.sw-next-title{font-size:16px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text)}
.sw-next-body{font-size:13.5px;line-height:1.6;color:var(--docs-text-soft)}
@media (max-width:960px){
  .sw-next{grid-template-columns:1fr}
}
`;
