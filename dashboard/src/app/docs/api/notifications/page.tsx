import type { Metadata } from "next";
import Link from "next/link";
import { DocsCode, DocsPage, DocsTable } from "../../_components/docs-shell";

export const metadata: Metadata = {
  title: "Notifications | UniPost Docs",
  description: "Configure UniPost notification channels, supported events, and subscriptions from Settings > Notifications.",
  keywords: ["unipost notifications", "slack webhook notifications", "discord webhook notifications", "settings notifications"],
};

const createSlackWebhook = `POST /v1/me/notifications/channels
Authorization: Bearer <dashboard-session-token>
Content-Type: application/json

{
  "kind": "slack_webhook",
  "url": "https://hooks.slack.com/services/T000/B000/XXXX",
  "label": "#ops-alerts"
}`;

const subscribeEvent = `PUT /v1/me/notifications/subscriptions
Authorization: Bearer <dashboard-session-token>
Content-Type: application/json

{
  "event_type": "post.failed",
  "channel_id": "notif_ch_123",
  "enabled": true
}`;

export default function NotificationsPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Notifications"
      lead="UniPost notifications are the human-facing alert system behind Settings > Notifications. Use them when a workspace owner should receive email, Slack, or Discord alerts for failures, billing issues, and account problems without building a webhook consumer."
    >
      <h2 id="overview">Overview</h2>
      <p>
        Open the dashboard at <code>/settings/notifications</code> to manage channels and event subscriptions. The page has two parts: <strong>Channels</strong>, where you add delivery destinations, and <strong>Subscriptions</strong>, where you choose which verified channels receive each event.
      </p>
      <div className="docs-callout">
        <strong>Default behavior:</strong> on the first bootstrap for a user with an email on file, UniPost automatically creates a verified email channel pointing at the signup email and subscribes it to every notification event that currently ships with <code>default_on=true</code>.
      </div>

      <h2 id="supported-channels">Supported channel types</h2>
      <DocsTable
        columns={["Channel", "Status", "How to configure it", "Verification behavior"]}
        rows={[
          ["Email", "Supported in Settings", "Enter an email address in Add channel > Email.", "Only the signup email is auto-verified today. Other email addresses can be created but remain unverified and cannot be subscribed yet."],
          ["Slack webhook", "Supported in Settings", "Create an Incoming Webhook in Slack, then paste the full https://hooks.slack.com/services/... URL.", "Auto-verified immediately after creation."],
          ["Discord webhook", "Supported in Settings", "Create a channel webhook in Discord, then paste the full https://discord.com/api/webhooks/... URL.", "Auto-verified immediately after creation."],
          ["SMS", "Not available yet", "Modeled in the backend only.", "Not wired for delivery."],
          ["In-app", "Not available yet", "Modeled in the backend only.", "Not wired for delivery."],
        ]}
      />

      <h3 id="add-email">Add an email channel</h3>
      <p>
        In <code>Settings &gt; Notifications</code>, click <strong>Add channel</strong>, choose <strong>Email</strong>, and enter the address. In the current implementation, the practical path is to use the Clerk signup email, because that address is auto-verified and immediately eligible for subscriptions. A different email address can be stored, but it stays <strong>Unverified</strong> and will not appear in the subscription matrix until a verification flow exists.
      </p>

      <h3 id="add-slack">Add a Slack webhook channel</h3>
      <p>
        Create an Incoming Webhook in the Slack workspace and channel where alerts should land, copy the full webhook URL, then add it under <strong>Add channel &gt; Slack Webhook</strong>. You can optionally set a label such as <code>#ops-alerts</code> or <code>#billing</code> so the channel is easier to recognize in the settings table.
      </p>
      <p>
        UniPost validates the URL format and accepts only URLs starting with <code>https://hooks.slack.com/</code>. After saving, the channel is marked verified automatically and can be tested immediately from the <strong>Test</strong> button.
      </p>

      <h3 id="add-discord">Add a Discord webhook channel</h3>
      <p>
        Create a webhook on the target Discord channel, copy the full webhook URL, then add it under <strong>Add channel &gt; Discord Webhook</strong>. Optional labels work the same way as Slack and are useful if you keep multiple Discord destinations.
      </p>
      <p>
        UniPost validates the URL format and accepts only URLs starting with <code>https://discord.com/api/webhooks/</code>. The channel is auto-verified after creation and can also be tested from the settings page.
      </p>

      <h2 id="settings-flow">How to add a channel in Settings</h2>
      <ol className="docs-list">
        <li>Open <Link href="/settings/notifications">Settings &gt; Notifications</Link>.</li>
        <li>Click <strong>Add channel</strong>.</li>
        <li>Choose <strong>Email</strong>, <strong>Slack Webhook</strong>, or <strong>Discord Webhook</strong>.</li>
        <li>Enter the email address or webhook URL. For Slack and Discord, optionally add a label.</li>
        <li>Save the channel.</li>
        <li>Use <strong>Test</strong> to confirm delivery before subscribing events.</li>
      </ol>
      <p>
        Deleting a channel also removes its related subscriptions. The subscriptions matrix only shows <strong>verified</strong> channels, so if a newly added channel is missing there, check whether it is still marked <strong>Unverified</strong>.
      </p>

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
        These four events are the current source of truth for the Settings page. They are also the events used when UniPost seeds default subscriptions for a newly provisioned signup email channel.
      </p>

      <h2 id="subscribe-events">How to subscribe events</h2>
      <p>
        After at least one verified channel exists, the <strong>Subscriptions</strong> section renders an event-by-channel matrix. Each checkbox represents one subscription row. Turning a checkbox on calls the notifications subscription API and enables delivery for that exact pair of <code>event_type</code> and <code>channel_id</code>. Turning it off keeps the row but sets <code>enabled=false</code>.
      </p>
      <ol className="docs-list">
        <li>Make sure the destination channel is verified.</li>
        <li>Go to the <strong>Subscriptions</strong> table in <code>/settings/notifications</code>.</li>
        <li>Find the event row you care about.</li>
        <li>Enable the checkbox under the channel column where you want the alert delivered.</li>
      </ol>
      <div className="docs-callout">
        <strong>Important:</strong> notification fanout only targets subscriptions that are both <code>enabled=true</code> and attached to a verified, non-deleted channel.
      </div>

      <h2 id="delivery-behavior">Delivery behavior</h2>
      <DocsTable
        columns={["Channel", "Message format", "Test message", "Retry behavior"]}
        rows={[
          ["Email", "Rendered UniPost email template per event", "Supported from the channel row", "Pending deliveries retry after 1 minute, 5 minutes, and 30 minutes before being marked dead."],
          ["Slack webhook", "Plain-text Slack webhook message", "Supported from the channel row", "Same retry schedule as email."],
          ["Discord webhook", "Plain-text Discord webhook message with username UniPost", "Supported from the channel row", "Same retry schedule as email."],
        ]}
      />

      <h2 id="api-endpoints">Settings API endpoints</h2>
      <DocsTable
        columns={["Endpoint", "Purpose"]}
        rows={[
          ["GET /v1/me/notifications/events", "List the events catalog shown in Settings."],
          ["GET /v1/me/notifications/channels", "List configured channels."],
          ["POST /v1/me/notifications/channels", "Create a channel."],
          ["POST /v1/me/notifications/channels/{id}/test", "Send a test notification through the real provider path."],
          ["DELETE /v1/me/notifications/channels/{id}", "Soft-delete a channel and stop future sends."],
          ["GET /v1/me/notifications/subscriptions", "List existing subscriptions."],
          ["PUT /v1/me/notifications/subscriptions", "Create or update one event/channel subscription."],
          ["DELETE /v1/me/notifications/subscriptions/{id}", "Delete a subscription row."],
        ]}
      />

      <h3 id="api-create-channel">Create a channel by API</h3>
      <p>
        The dashboard UI uses account-scoped <code>/v1/me/notifications/*</code> routes. If you are extending the product UI or automating setup from a signed-in session, create channels with the same endpoints.
      </p>
      <DocsCode code={createSlackWebhook} language="http" />

      <h3 id="api-subscribe">Subscribe to an event by API</h3>
      <p>
        Subscriptions are stored per event and per channel. The same event can fan out to multiple channels, and the same channel can be enabled for multiple events.
      </p>
      <DocsCode code={subscribeEvent} language="http" />

      <h2 id="recommended-setup">Recommended setup</h2>
      <p>
        For most teams, the clean starting point is to leave the auto-provisioned signup email enabled for all four default events, then add one Slack or Discord webhook for shared operational visibility. Use email for personal ownership and Slack or Discord for team-level response.
      </p>
    </DocsPage>
  );
}
