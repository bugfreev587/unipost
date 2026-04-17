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
      lead="Use Notifications when you want UniPost to send alerts to email, Slack, or Discord from the dashboard. This page focuses on the fastest setup path."
    >
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
          ["Email", "Yes", "Enter an email address in UniPost.", "Only the signup email is auto-verified today."],
          ["Slack webhook", "Yes", "Create an Incoming Webhook URL in Slack, then paste it into UniPost.", "URL must start with https://hooks.slack.com/."],
          ["Discord webhook", "Yes", "Create a webhook URL in Discord, then paste it into UniPost.", "URL must start with https://discord.com/api/webhooks/."],
          ["SMS", "No", "Not available yet.", "Modeled in backend only."],
          ["In-app", "No", "Not available yet.", "Modeled in backend only."],
        ]}
      />

      <h2 id="email-setup">Email setup</h2>
      <p>
        Fastest path:
      </p>
      <ol className="docs-list">
        <li>Open <code>Settings &gt; Notifications</code>.</li>
        <li>Click <strong>Add channel &gt; Email</strong>.</li>
        <li>Enter your email address and save.</li>
      </ol>
      <div className="docs-callout">
        <strong>Current limitation:</strong> only the signup email is auto-verified. Other email addresses can be added, but they stay <code>Unverified</code> and will not appear in the subscriptions table yet.
      </div>

      <h2 id="slack-setup">Slack setup</h2>
      <p>
        Official Slack guide: <a href="https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/" target="_blank" rel="noreferrer">Sending messages using incoming webhooks</a>.
      </p>
      <p>
        In Slack, do this:
      </p>
      <ol className="docs-list">
        <li>Open <a href="https://api.slack.com/apps" target="_blank" rel="noreferrer">api.slack.com/apps</a> and click <strong>Create New App</strong>.</li>
        <li>Choose <strong>From scratch</strong>, enter an app name, pick your Slack workspace, and create the app.</li>
        <li>In the app sidebar, open <strong>Incoming Webhooks</strong>.</li>
        <li>Turn on <strong>Activate Incoming Webhooks</strong>.</li>
        <li>Click <strong>Add New Webhook to Workspace</strong>.</li>
        <li>Pick the Slack channel where UniPost alerts should post, then click <strong>Allow</strong> or <strong>Authorize</strong>.</li>
        <li>Copy the generated URL. It should look like <code>https://hooks.slack.com/services/...</code>.</li>
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
        Official Discord guide: <a href="https://support.discord.com/hc/en-us/articles/228383668-Intro-to-Webhooks" target="_blank" rel="noreferrer">Intro to Webhooks</a>.
      </p>
      <p>
        In Discord, do this:
      </p>
      <ol className="docs-list">
        <li>Open your Discord server.</li>
        <li>Go to <strong>Server Settings &gt; Integrations</strong>.</li>
        <li>Click <strong>Create Webhook</strong>.</li>
        <li>Choose the text channel where UniPost alerts should post.</li>
        <li>Name the webhook.</li>
        <li>Click <strong>Copy Webhook URL</strong>.</li>
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
