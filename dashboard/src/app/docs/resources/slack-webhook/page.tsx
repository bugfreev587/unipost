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
      <div className="docs-badge-row">
        <span className="docs-badge">~2 min</span>
        <span className="docs-badge">No admin required</span>
        <span className="docs-badge">One-time setup</span>
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

      <div className="docs-callout docs-callout-warning">
        <strong>Heads up:</strong> UniPost only accepts URLs starting with <code>https://hooks.slack.com/</code>. Anything else is rejected at save time.
      </div>

      <h2 id="steps">Steps in Slack</h2>
      <ol className="docs-screenshot-steps">
        {STEPS.map((step) => (
          <li key={step.n} className="docs-screenshot-step">
            <div className="docs-screenshot-step-head">
              <div className="docs-screenshot-step-number">{step.n}</div>
              <div className="docs-screenshot-step-title">{step.title}</div>
            </div>
            <div className="docs-screenshot-step-body">{step.body}</div>
            <div className="docs-screenshot-step-image">
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
      <p className="docs-note">
        <strong>Next:</strong> after the channel shows as <strong>Verified</strong>, open the <Link href="/docs/resources/notifications#subscribe-events">Subscriptions</Link> table and turn on the alerts you want.
      </p>

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
      <div className="docs-next-grid">
        <Link href="/docs/resources/notifications" className="docs-next-card">
          <div className="docs-next-kicker">Overview</div>
          <div className="docs-next-title">Notifications overview</div>
          <div className="docs-next-body">Channels, events, and which ones are on by default.</div>
        </Link>
        <Link href="/docs/resources/discord-webhook" className="docs-next-card">
          <div className="docs-next-kicker">Also available</div>
          <div className="docs-next-title">Discord Webhook URL</div>
          <div className="docs-next-body">Same setup shape for a Discord channel.</div>
        </Link>
        <a href="https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/" target="_blank" rel="noreferrer" className="docs-next-card">
          <div className="docs-next-kicker">Slack docs</div>
          <div className="docs-next-title">Incoming Webhooks reference</div>
          <div className="docs-next-body">Slack&apos;s official deep-dive on webhook payloads and limits.</div>
        </a>
        <Link href="/settings/notifications" className="docs-next-card">
          <div className="docs-next-kicker">Configure</div>
          <div className="docs-next-title">Open Notifications settings</div>
          <div className="docs-next-body">Add the channel and manage subscriptions.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
