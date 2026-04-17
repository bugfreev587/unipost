import type { Metadata } from "next";
import Link from "next/link";
import { DocsPage } from "../../_components/docs-shell";

export const metadata: Metadata = {
  title: "How to get a Slack webhook URL | UniPost Docs",
  description: "Step-by-step Slack UI guide to create an incoming webhook URL for UniPost notifications.",
  keywords: ["slack webhook url", "unipost slack notifications", "slack incoming webhooks"],
};

function DocImage({ src, alt }: { src: string; alt: string }) {
  return (
    <div style={{ margin: "18px 0 28px" }}>
      <img
        src={src}
        alt={alt}
        style={{
          width: "100%",
          borderRadius: 18,
          border: "1px solid var(--docs-border)",
          display: "block",
          background: "var(--docs-panel)",
        }}
      />
    </div>
  );
}

export default function SlackWebhookPage() {
  return (
    <DocsPage
      eyebrow="Guide"
      title="How to get a Slack webhook URL"
      lead="Use this page if you want a Slack channel to receive UniPost notifications. These are the exact steps in Slack's app UI."
    >
      <h2 id="before-you-start">Before you start</h2>
      <p>
        You need access to the Slack workspace where notifications should post. The webhook URL is created from a Slack app with Incoming Webhooks enabled.
      </p>

      <h2 id="step-1">1. Open Slack apps and create a new app</h2>
      <ol className="docs-list">
        <li>Open <a href="https://api.slack.com/apps" target="_blank" rel="noreferrer">api.slack.com/apps</a>.</li>
        <li>Click <strong>Create an App</strong>.</li>
      </ol>
      <DocImage src="/docs/slack-webhook/step1.png" alt="Slack API Your Apps page with the Create an App button highlighted." />

      <h2 id="step-2">2. Choose From scratch</h2>
      <ol className="docs-list">
        <li>In the create app dialog, choose <strong>From scratch</strong>.</li>
      </ol>
      <DocImage src="/docs/slack-webhook/step2.png" alt="Slack create app dialog with From scratch highlighted." />

      <h2 id="step-3">3. Name the app and choose the workspace</h2>
      <ol className="docs-list">
        <li>Enter an app name.</li>
        <li>Select the Slack workspace where the webhook should live.</li>
        <li>Click <strong>Create App</strong>.</li>
      </ol>
      <DocImage src="/docs/slack-webhook/step3.png" alt="Slack dialog for naming the app and choosing a workspace." />

      <h2 id="step-4">4. Open Incoming Webhooks and enable it</h2>
      <ol className="docs-list">
        <li>In the app sidebar, click <strong>Incoming Webhooks</strong>.</li>
        <li>Turn on <strong>Activate Incoming Webhooks</strong>.</li>
      </ol>
      <DocImage src="/docs/slack-webhook/step4.png" alt="Slack app Incoming Webhooks settings page with the toggle enabled." />

      <h2 id="step-5">5. Add a new webhook</h2>
      <ol className="docs-list">
        <li>Scroll to <strong>Webhook URLs for Your Workspace</strong>.</li>
        <li>Click <strong>Add New Webhook</strong>.</li>
      </ol>
      <DocImage src="/docs/slack-webhook/step5.png" alt="Slack Incoming Webhooks page with the Add New Webhook button highlighted." />

      <h2 id="step-6">6. Choose the channel and allow access</h2>
      <ol className="docs-list">
        <li>Select the Slack channel where UniPost alerts should be posted.</li>
        <li>Click <strong>Allow</strong>.</li>
      </ol>
      <DocImage src="/docs/slack-webhook/step6.png" alt="Slack permission screen with channel selection and Allow button highlighted." />

      <h2 id="step-7">7. Copy the webhook URL</h2>
      <ol className="docs-list">
        <li>Back on the Incoming Webhooks page, find the webhook you just created.</li>
        <li>Click <strong>Copy</strong> to copy the webhook URL.</li>
      </ol>
      <DocImage src="/docs/slack-webhook/step7.png" alt="Slack Incoming Webhooks page showing a generated webhook URL and copy button." />
      <div className="docs-callout">
        <strong>Expected format:</strong> the copied URL should start with <code>https://hooks.slack.com/services/</code>.
      </div>

      <h2 id="add-to-unipost">8. Paste it into UniPost</h2>
      <ol className="docs-list">
        <li>Open <Link href="/settings/notifications">Settings &gt; Notifications</Link> in UniPost.</li>
        <li>Click <strong>Add channel &gt; Slack Webhook</strong>.</li>
        <li>Paste the webhook URL.</li>
        <li>Optional: add a label like <code>#ops-alerts</code>.</li>
        <li>Save, then click <strong>Test</strong>.</li>
      </ol>

      <h2 id="next-step">Next step</h2>
      <p>
        After the channel is marked <strong>Verified</strong>, go to the <Link href="/docs/api/notifications#subscribe-events">subscribe events section</Link> and turn on the alerts you want.
      </p>

      <p>
        Slack's official reference is here if you want it: <a href="https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/" target="_blank" rel="noreferrer">Sending messages using incoming webhooks</a>.
      </p>
    </DocsPage>
  );
}
