import type { Metadata } from "next";
import Link from "next/link";
import { DocsPage, DocsTable } from "../../_components/docs-shell";

export const metadata: Metadata = {
  title: "Discord Webhook URL | UniPost Docs",
  description: "Step-by-step Discord UI guide to create a webhook URL for UniPost notifications.",
  keywords: ["discord webhook url", "unipost discord notifications", "discord integrations webhook"],
};

const STEPS = [
  {
    n: "1",
    title: "Open the channel settings",
    body: (
      <>
        Open Discord, find the text channel that should receive UniPost alerts, hover the channel name, and click the gear icon.
      </>
    ),
    img: "/docs/discord-webhook/step1.png",
    alt: "Discord channel list with the channel settings gear icon highlighted for the target channel.",
  },
  {
    n: "2",
    title: "Open Integrations and create the webhook",
    body: (
      <>
        In the settings sidebar, click <strong>Integrations</strong>, then <strong>Webhooks</strong>, then <strong>New Webhook</strong>.
      </>
    ),
    img: "/docs/discord-webhook/step2.png",
    alt: "Discord channel settings page with Integrations selected and Webhooks visible.",
  },
  {
    n: "3",
    title: "Copy the webhook URL",
    body: (
      <>
        Click the new webhook, optionally rename it, then click <strong>Copy Webhook URL</strong>.
      </>
    ),
    img: "/docs/discord-webhook/step3.png",
    alt: "Discord webhook details screen with the Copy Webhook URL button visible.",
  },
];

export default function DiscordWebhookPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Resources · Notifications"
      title="Discord Webhook URL"
      lead="Create a Discord channel webhook and paste it into UniPost. Three clicks in Discord, one paste in UniPost."
    >
      <div className="docs-badge-row">
        <span className="docs-badge">~1 min</span>
        <span className="docs-badge">Server-level only</span>
        <span className="docs-badge">One-time setup</span>
      </div>

      <h2 id="at-a-glance">At a glance</h2>
      <DocsTable
        columns={["Question", "Answer"]}
        rows={[
          ["Who needs it", "Anyone who wants UniPost alerts in a Discord channel"],
          ["Where the webhook lives", "Channel settings → Integrations → Webhooks"],
          ["Expected URL format", "`https://discord.com/api/webhooks/...`"],
          ["Permission required", "Manage Webhooks on the target channel"],
          ["Setup time", "About 1 minute"],
        ]}
      />

      <div className="docs-callout docs-callout-warning">
        <strong>Heads up:</strong> UniPost only accepts URLs starting with <code>https://discord.com/api/webhooks/</code>. Anything else is rejected at save time.
      </div>

      <h2 id="steps">Steps in Discord</h2>
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
        <li>Click <strong>Add channel → Discord Webhook</strong></li>
        <li>Paste the webhook URL from Discord</li>
        <li>Optional — add a label</li>
        <li>Save, then click <strong>Test</strong></li>
      </ul>
      <p className="docs-note">
        <strong>Next:</strong> after the channel shows as <strong>Verified</strong>, open the <Link href="/docs/resources/notifications#subscribe-events">Subscriptions</Link> table and turn on the alerts you want.
      </p>

      <h2 id="troubleshooting">Troubleshooting</h2>
      <DocsTable
        columns={["Symptom", "Likely cause", "Fix"]}
        rows={[
          ["UniPost rejects the URL at save", "URL does not start with `https://discord.com/api/webhooks/`", "Copy from the Webhook detail screen — not from a shared message or invite link"],
          ["Channel settings does not show Integrations", "You do not have Manage Webhooks on the server", "Ask a server admin to grant the permission or create the webhook for you"],
          ["Test delivered but Discord shows nothing", "Channel was deleted or the webhook revoked", "Regenerate the webhook URL in Discord and paste the new one into UniPost"],
        ]}
      />

      <h2 id="next-steps">Next steps</h2>
      <div className="docs-next-grid">
        <Link href="/docs/resources/notifications" className="docs-next-card">
          <div className="docs-next-kicker">Overview</div>
          <div className="docs-next-title">Notifications overview</div>
          <div className="docs-next-body">Channels, events, and which ones are on by default.</div>
        </Link>
        <Link href="/docs/resources/slack-webhook" className="docs-next-card">
          <div className="docs-next-kicker">Also available</div>
          <div className="docs-next-title">Slack Webhook URL</div>
          <div className="docs-next-body">Same setup shape for a Slack channel.</div>
        </Link>
        <a href="https://support.discord.com/hc/en-us/articles/228383668-Intro-to-Webhooks" target="_blank" rel="noreferrer" className="docs-next-card">
          <div className="docs-next-kicker">Discord docs</div>
          <div className="docs-next-title">Intro to Webhooks</div>
          <div className="docs-next-body">Discord&apos;s official reference for webhook behavior.</div>
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
