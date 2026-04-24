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
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="dw-badges">
        <span className="dw-badge">~1 min</span>
        <span className="dw-badge">Server-level only</span>
        <span className="dw-badge">One-time setup</span>
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

      <div className="docs-callout">
        <strong>Heads up:</strong> UniPost only accepts URLs starting with <code>https://discord.com/api/webhooks/</code>. Anything else is rejected at save time.
      </div>

      <h2 id="steps">Steps in Discord</h2>
      <ol className="dw-steps">
        {STEPS.map((step) => (
          <li key={step.n} className="dw-step">
            <div className="dw-step-head">
              <div className="dw-step-num">{step.n}</div>
              <div className="dw-step-title">{step.title}</div>
            </div>
            <div className="dw-step-body">{step.body}</div>
            <div className="dw-step-img">
              <img src={step.img} alt={step.alt} />
            </div>
          </li>
        ))}
      </ol>

      <h2 id="paste-into-unipost">Paste the URL into UniPost</h2>
      <ul className="dw-checklist">
        <li>Open <Link href="/settings/notifications">Settings → Notifications</Link></li>
        <li>Click <strong>Add channel → Discord Webhook</strong></li>
        <li>Paste the webhook URL from Discord</li>
        <li>Optional — add a label</li>
        <li>Save, then click <strong>Test</strong></li>
      </ul>
      <div className="docs-callout">
        <strong>Next:</strong> after the channel shows as <strong>Verified</strong>, open the <Link href="/docs/resources/notifications#subscribe-events">Subscriptions</Link> table and turn on the alerts you want.
      </div>

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
      <div className="dw-next">
        <Link href="/docs/resources/notifications" className="dw-next-card">
          <div className="dw-next-kicker">Overview</div>
          <div className="dw-next-title">Notifications overview</div>
          <div className="dw-next-body">Channels, events, and which ones are on by default.</div>
        </Link>
        <Link href="/docs/resources/slack-webhook" className="dw-next-card">
          <div className="dw-next-kicker">Also available</div>
          <div className="dw-next-title">Slack Webhook URL</div>
          <div className="dw-next-body">Same setup shape for a Slack channel.</div>
        </Link>
        <a href="https://support.discord.com/hc/en-us/articles/228383668-Intro-to-Webhooks" target="_blank" rel="noreferrer" className="dw-next-card">
          <div className="dw-next-kicker">Discord docs</div>
          <div className="dw-next-title">Intro to Webhooks</div>
          <div className="dw-next-body">Discord's official reference for webhook behavior.</div>
        </a>
        <Link href="/settings/notifications" className="dw-next-card">
          <div className="dw-next-kicker">Configure</div>
          <div className="dw-next-title">Open Notifications settings</div>
          <div className="dw-next-body">Add the channel and manage subscriptions.</div>
        </Link>
      </div>
    </DocsPage>
  );
}

const styles = `
.dw-badges{display:flex;flex-wrap:wrap;gap:6px;margin:2px 0 18px}
.dw-badge{display:inline-flex;align-items:center;padding:4px 11px;border-radius:999px;background:var(--docs-bg-muted);border:1px solid var(--docs-border);color:var(--docs-text);font-size:11.5px;font-weight:600;letter-spacing:.01em}
.dw-steps{list-style:none;padding:0;margin:14px 0 6px;display:grid;grid-template-columns:1fr;gap:18px}
.dw-step{padding:18px 20px 20px;border:1px solid var(--docs-border);border-radius:18px;background:var(--docs-bg-elevated);box-shadow:0 1px 0 rgba(255,255,255,.02)}
.dw-step-head{display:flex;align-items:center;gap:12px;margin-bottom:8px}
.dw-step-num{display:inline-flex;align-items:center;justify-content:center;width:30px;height:30px;border-radius:999px;background:color-mix(in srgb, var(--docs-link) 14%, var(--docs-bg-muted));color:var(--docs-link);font-size:14px;font-weight:700;border:1px solid color-mix(in srgb, var(--docs-link) 22%, var(--docs-border));flex:none}
.dw-step-title{font-size:17px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text)}
.dw-step-body{font-size:14.5px;line-height:1.7;color:var(--docs-text-soft);margin-bottom:12px}
.dw-step-body code{font-family:var(--docs-mono);font-size:12.5px}
.dw-step-img{border:1px solid var(--docs-border);border-radius:14px;overflow:hidden;background:var(--docs-bg-muted)}
.dw-step-img img{display:block;width:100%;height:auto}
.dw-checklist{list-style:none;padding:0;margin:10px 0 14px;display:grid;grid-template-columns:1fr;gap:4px}
.dw-checklist li{position:relative;padding-left:22px;font-size:14px;line-height:1.7;color:var(--docs-text-soft)}
.dw-checklist li::before{content:"";position:absolute;left:0;top:9px;width:12px;height:12px;border-radius:4px;border:1.5px solid color-mix(in srgb, var(--docs-link) 45%, var(--docs-border-strong));background:color-mix(in srgb, var(--docs-link) 14%, transparent)}
.dw-checklist li code{font-family:var(--docs-mono);font-size:12.5px}
.dw-next{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin:14px 0 4px}
.dw-next-card{display:flex;flex-direction:column;gap:6px;padding:16px 18px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated);text-decoration:none;color:inherit;transition:border-color .15s ease,transform .15s ease,box-shadow .15s ease}
.dw-next-card:hover{border-color:color-mix(in srgb, var(--docs-link) 38%, var(--docs-border));transform:translateY(-1px);box-shadow:var(--docs-card-shadow);text-decoration:none}
.dw-next-kicker{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint)}
.dw-next-title{font-size:16px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text)}
.dw-next-body{font-size:13.5px;line-height:1.6;color:var(--docs-text-soft)}
@media (max-width:960px){
  .dw-next{grid-template-columns:1fr}
}
`;
