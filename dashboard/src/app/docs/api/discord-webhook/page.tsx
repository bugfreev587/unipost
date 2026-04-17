import type { Metadata } from "next";
import Link from "next/link";
import { DocsPage } from "../../_components/docs-shell";

export const metadata: Metadata = {
  title: "How to get a Discord webhook URL | UniPost Docs",
  description: "Step-by-step Discord UI guide to create a webhook URL for UniPost notifications.",
  keywords: ["discord webhook url", "unipost discord notifications", "discord integrations webhook"],
};

export default function DiscordWebhookPage() {
  return (
    <DocsPage
      eyebrow="Guide"
      title="How to get a Discord webhook URL"
      lead="Use this page if you want a Discord channel to receive UniPost notifications. These are the exact UI steps inside Discord."
    >
      <h2 id="before-you-start">Before you start</h2>
      <p>
        Pick the Discord text channel where you want UniPost alerts to appear. You will create the webhook from that channel's settings page.
      </p>

      <h2 id="step-1">1. Open the channel settings</h2>
      <ol className="docs-list">
        <li>Open Discord.</li>
        <li>Find the text channel that should receive UniPost alerts.</li>
        <li>Hover the channel name and click the gear icon.</li>
      </ol>
      <p>
        This opens the settings page for that specific channel.
      </p>
      <div style={{ margin: "18px 0 28px" }}>
        <img
          src="/docs/discord-webhook/step1.png"
          alt="Discord channel list with the channel settings gear icon highlighted for the target channel."
          style={{
            width: "100%",
            borderRadius: 18,
            border: "1px solid var(--docs-border)",
            display: "block",
            background: "var(--docs-panel)",
          }}
        />
      </div>

      <h2 id="step-2">2. Open Integrations and create the webhook</h2>
      <ol className="docs-list">
        <li>In the channel settings sidebar, click <strong>Integrations</strong>.</li>
        <li>Click <strong>Webhooks</strong>.</li>
        <li>Click <strong>New Webhook</strong>.</li>
      </ol>
      <p>
        Discord will create a new webhook entry for that channel.
      </p>
      <div style={{ margin: "18px 0 28px" }}>
        <img
          src="/docs/discord-webhook/step2.png"
          alt="Discord channel settings page with Integrations selected and Webhooks visible."
          style={{
            width: "100%",
            borderRadius: 18,
            border: "1px solid var(--docs-border)",
            display: "block",
            background: "var(--docs-panel)",
          }}
        />
      </div>

      <h2 id="step-3">3. Copy the webhook URL</h2>
      <ol className="docs-list">
        <li>Click the new webhook you just created.</li>
        <li>If needed, rename it so it is easy to recognize later.</li>
        <li>Click <strong>Copy Webhook URL</strong>.</li>
      </ol>
      <div style={{ margin: "18px 0 20px" }}>
        <img
          src="/docs/discord-webhook/step3.png"
          alt="Discord webhook details screen with the Copy Webhook URL button visible."
          style={{
            width: "100%",
            borderRadius: 18,
            border: "1px solid var(--docs-border)",
            display: "block",
            background: "var(--docs-panel)",
          }}
        />
      </div>
      <div className="docs-callout">
        <strong>Expected format:</strong> the copied URL should start with <code>https://discord.com/api/webhooks/</code>.
      </div>

      <h2 id="add-to-unipost">4. Paste it into UniPost</h2>
      <ol className="docs-list">
        <li>Open <Link href="/settings/notifications">Settings &gt; Notifications</Link> in UniPost.</li>
        <li>Click <strong>Add channel &gt; Discord Webhook</strong>.</li>
        <li>Paste the webhook URL.</li>
        <li>Optional: add a label.</li>
        <li>Save, then click <strong>Test</strong>.</li>
      </ol>

      <h2 id="next-step">Next step</h2>
      <p>
        After the channel is marked <strong>Verified</strong>, go to the <Link href="/docs/api/notifications#subscribe-events">subscribe events section</Link> and turn on the alerts you want.
      </p>

      <p>
        Discord's official reference is here if you want it: <a href="https://support.discord.com/hc/en-us/articles/228383668-Intro-to-Webhooks" target="_blank" rel="noreferrer">Intro to Webhooks</a>.
      </p>
    </DocsPage>
  );
}
