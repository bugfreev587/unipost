import type { Metadata } from "next";
import Link from "next/link";
import { DocsPage, DocsTable } from "../../_components/docs-shell";

export const metadata: Metadata = {
  title: "Set up Notifications | UniPost Docs",
  description: "Quick setup guide for UniPost notifications: create email, Slack, and Discord channels and subscribe to events.",
  keywords: ["unipost notifications", "slack webhook notifications", "discord webhook notifications", "settings notifications"],
};

export default function NotificationsPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Set up Notifications"
      lead="Use Notifications when you want UniPost to send human-facing alerts to email, Slack, or Discord from the dashboard. This is separate from developer webhooks for your own backend receivers."
    >
      <div className="docs-callout">
        <strong>Not the same as developer webhooks:</strong> if you need machine-to-machine delivery for post status or account events, use <Link href="/docs/api/webhooks">Webhooks</Link>. Notifications are dashboard-managed channels for people.
      </div>

      <h2 id="quick-start">Quick start</h2>
      <p>
        Open <Link href="/settings/notifications">Settings &gt; Notifications</Link>. Setup always works like this:
      </p>
      <ol className="docs-list">
        <li>Create a channel in UniPost.</li>
        <li>For Slack or Discord, first create a webhook URL in Slack or Discord.</li>
        <li>Paste that URL into UniPost.</li>
        <li>Click <strong>Test</strong>.</li>
        <li>Turn on the events you want in the <strong>Subscriptions</strong> table.</li>
      </ol>
      <div className="docs-callout">
        <strong>Tip:</strong> if you just want something working fast, add a Slack webhook first. It is created once in Slack, pasted once into UniPost, and becomes verified immediately.
      </div>

      <h2 id="supported-channels">Supported channel types</h2>
      <DocsTable
        columns={["Channel", "Use now?", "What you need", "Notes"]}
        rows={[
          ["Email", "Yes", "Nothing to create. UniPost uses your signup email automatically.", "You only need to turn events on or off."],
          ["Slack webhook", "Yes", "Create an Incoming Webhook URL in Slack, then paste it into UniPost.", "URL must start with https://hooks.slack.com/."],
          ["Discord webhook", "Yes", "Create a webhook URL in Discord, then paste it into UniPost.", "URL must start with https://discord.com/api/webhooks/."],
          ["SMS", "No", "Not available yet.", "Modeled in backend only."],
          ["In-app", "No", "Not available yet.", "Modeled in backend only."],
        ]}
      />

      <h2 id="email-setup">Email setup</h2>
      <p>
        Email is the default channel. UniPost automatically creates it from your signup email.
      </p>
      <ol className="docs-list">
        <li>Open <code>Settings &gt; Notifications</code>.</li>
        <li>Look for the built-in <strong>Email</strong> channel that shows your signup email address.</li>
        <li>Use <strong>Test</strong> if you want to confirm delivery.</li>
        <li>Turn events on or off in the <strong>Subscriptions</strong> table.</li>
      </ol>
      <div className="docs-callout">
        <strong>Current rule:</strong> UniPost only sends email notifications to your signup email. You do not need to add it manually.
      </div>

      <h2 id="slack-setup">Slack setup</h2>
      <p>
        If you want the exact Slack UI flow, use this page: <Link href="/docs/api/slack-webhook">How to get a Slack webhook URL</Link>.
      </p>
      <p>
        Short version:
      </p>
      <ol className="docs-list">
        <li>Open <a href="https://api.slack.com/apps" target="_blank" rel="noreferrer">api.slack.com/apps</a> and create a new app.</li>
        <li>Open <strong>Incoming Webhooks</strong> in the app sidebar and turn it on.</li>
        <li>Click <strong>Add New Webhook</strong>, choose the channel, then click <strong>Allow</strong>.</li>
        <li>Copy the generated webhook URL.</li>
      </ol>
      <p>
        Then in UniPost:
      </p>
      <ol className="docs-list">
        <li>Open <code>Settings &gt; Notifications</code>.</li>
        <li>Click <strong>Add channel &gt; Slack Webhook</strong>.</li>
        <li>Paste the URL from Slack.</li>
        <li>Optional: add a label like <code>#ops-alerts</code>.</li>
        <li>Save, then click <strong>Test</strong>.</li>
      </ol>
      <div className="docs-callout">
        <strong>Slack URL format:</strong> UniPost only accepts URLs starting with <code>https://hooks.slack.com/</code>.
      </div>

      <h2 id="discord-setup">Discord setup</h2>
      <p>
        If Discord's official guide feels too vague, use our shorter guide here: <Link href="/docs/api/discord-webhook">How to get a Discord webhook URL</Link>.
      </p>
      <p>
        Short version:
      </p>
      <ol className="docs-list">
        <li>Open the Discord channel where you want alerts, then click the channel gear icon.</li>
        <li>Open <strong>Integrations &gt; Webhooks</strong>, then click <strong>New Webhook</strong>.</li>
        <li>Open that webhook and click <strong>Copy Webhook URL</strong>.</li>
      </ol>
      <p>
        Then in UniPost:
      </p>
      <ol className="docs-list">
        <li>Open <code>Settings &gt; Notifications</code>.</li>
        <li>Click <strong>Add channel &gt; Discord Webhook</strong>.</li>
        <li>Paste the URL from Discord.</li>
        <li>Optional: add a label.</li>
        <li>Save, then click <strong>Test</strong>.</li>
      </ol>
      <div className="docs-callout">
        <strong>Discord URL format:</strong> UniPost only accepts URLs starting with <code>https://discord.com/api/webhooks/</code>.
      </div>

      <h2 id="supported-events">Supported events</h2>
      <DocsTable
        columns={["Event type", "Default on", "Severity", "What triggers it"]}
        rows={[
          ["post.failed", "Yes", "High", "A post could not be delivered to the target platform."],
          ["account.disconnected", "Yes", "High", "A connected social account lost access and needs to be reconnected before future posts can succeed."],
          ["billing.usage_80pct", "Yes", "Medium", "The workspace has reached 80% of its monthly plan quota."],
          ["billing.payment_failed", "Yes", "Critical", "A Stripe subscription payment failed."],
        ]}
      />

      <p>
        These are the events currently available in <code>Settings &gt; Notifications</code>. New users with an auto-provisioned signup email channel are subscribed to all four by default.
      </p>

      <h2 id="subscribe-events">How to subscribe events</h2>
      <ol className="docs-list">
        <li>Make sure the channel shows as <strong>Verified</strong>.</li>
        <li>Scroll to the <strong>Subscriptions</strong> table.</li>
        <li>Find the event you want, such as <code>post.failed</code>.</li>
        <li>Turn on the checkbox under the channel where you want alerts delivered.</li>
      </ol>
      <div className="docs-callout">
        <strong>Important:</strong> only verified channels appear in the subscriptions table.
      </div>
    </DocsPage>
  );
}
