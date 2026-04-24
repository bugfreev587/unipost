import type { Metadata } from "next";
import Link from "next/link";
import { DocsPage, DocsTable } from "../../_components/docs-shell";

export const metadata: Metadata = {
  title: "Notifications Overview | UniPost Docs",
  description: "Overview of UniPost notification channels and events — email, Slack, Discord, and the event subscriptions you can turn on.",
  keywords: ["unipost notifications", "slack webhook notifications", "discord webhook notifications", "settings notifications"],
};

export default function NotificationsOverviewPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Resources · Notifications"
      title="Notifications Overview"
      lead="Channels and events that deliver UniPost alerts to humans. Email is ready to go, Slack and Discord need a webhook URL."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="nt-badges">
        <span className="nt-badge">Email ready</span>
        <span className="nt-badge">Slack · Discord</span>
        <span className="nt-badge">4 event types</span>
        <span className="nt-badge">Dashboard-managed</span>
      </div>

      <div className="docs-callout">
        <strong>Not the same as developer webhooks.</strong> If you need machine-to-machine delivery for post status or account events, use <Link href="/docs/api/webhooks">Developer webhooks</Link>. Notifications are dashboard-managed channels for people.
      </div>

      <h2 id="at-a-glance">At a glance</h2>
      <DocsTable
        columns={["Question", "Answer"]}
        rows={[
          ["Who is it for", "Operators, on-call engineers, and founders who want human-readable alerts"],
          ["Where to configure", <Link key="nt-settings" href="/settings/notifications">Settings → Notifications</Link>],
          ["Channels ready today", "Email, Slack, Discord"],
          ["Events ready today", "`post.failed`, `account.disconnected`, `billing.usage_80pct`, `billing.payment_failed`"],
          ["Setup time", "~2 minutes per channel"],
        ]}
      />

      <h2 id="setup-flow">How setup works</h2>
      <div className="nt-flow">
        <div className="nt-flow-step">
          <div className="nt-flow-num">1</div>
          <div className="nt-flow-body">
            <div className="nt-flow-title">Create a channel in UniPost</div>
            <div className="nt-flow-sub">Open <Link href="/settings/notifications">Settings → Notifications</Link>.</div>
          </div>
        </div>
        <div className="nt-flow-step">
          <div className="nt-flow-num">2</div>
          <div className="nt-flow-body">
            <div className="nt-flow-title">For Slack or Discord, create a webhook URL first</div>
            <div className="nt-flow-sub">Use the <Link href="/docs/resources/slack-webhook">Slack guide</Link> or <Link href="/docs/resources/discord-webhook">Discord guide</Link>.</div>
          </div>
        </div>
        <div className="nt-flow-step">
          <div className="nt-flow-num">3</div>
          <div className="nt-flow-body">
            <div className="nt-flow-title">Paste the URL into UniPost</div>
            <div className="nt-flow-sub">Add channel → Slack / Discord Webhook → paste.</div>
          </div>
        </div>
        <div className="nt-flow-step">
          <div className="nt-flow-num">4</div>
          <div className="nt-flow-body">
            <div className="nt-flow-title">Click Test</div>
            <div className="nt-flow-sub">Confirm delivery before you trust production alerts to it.</div>
          </div>
        </div>
        <div className="nt-flow-step">
          <div className="nt-flow-num">5</div>
          <div className="nt-flow-body">
            <div className="nt-flow-title">Turn on events</div>
            <div className="nt-flow-sub">Use the <strong>Subscriptions</strong> table to pick which alerts go where.</div>
          </div>
        </div>
      </div>

      <h2 id="channels">Supported channels</h2>
      <DocsTable
        columns={["Channel", "Available", "What you need", "Format rule"]}
        rows={[
          ["Email", "Yes", "Nothing — UniPost uses your signup email automatically", "—"],
          ["Slack webhook", "Yes", "Slack Incoming Webhook URL", "Must start with `https://hooks.slack.com/`"],
          ["Discord webhook", "Yes", "Discord channel webhook URL", "Must start with `https://discord.com/api/webhooks/`"],
          ["SMS", "No", "Modeled in backend only", "—"],
          ["In-app", "No", "Modeled in backend only", "—"],
        ]}
      />

      <h2 id="events">Supported events</h2>
      <DocsTable
        columns={["Event", "Severity", "Default on", "What triggers it"]}
        rows={[
          ["`post.failed`", "High", "Yes", "A post could not be delivered to the target platform"],
          ["`account.disconnected`", "High", "Yes", "A connected social account lost access and needs to be reconnected"],
          ["`billing.usage_80pct`", "Medium", "Yes", "The workspace has reached 80% of its monthly plan quota"],
          ["`billing.payment_failed`", "Critical", "Yes", "A Stripe subscription payment failed"],
        ]}
      />
      <p className="nt-note">New workspaces are subscribed to all four events on the auto-provisioned email channel by default.</p>

      <h2 id="email-setup">Email</h2>
      <p className="nt-note">Email is the default channel. UniPost creates it from your signup email automatically — there is nothing to add.</p>
      <ul className="docs-checklist">
        <li>Open <Link href="/settings/notifications">Settings → Notifications</Link></li>
        <li>Find the built-in <strong>Email</strong> channel showing your signup email</li>
        <li>Use <strong>Test</strong> if you want to confirm delivery</li>
        <li>Turn events on in the <strong>Subscriptions</strong> table</li>
      </ul>
      <div className="docs-callout">
        <strong>Current rule:</strong> UniPost only sends email to your signup email. You do not need to add it manually.
      </div>

      <h2 id="subscribe-events">Subscribe to events</h2>
      <ul className="docs-checklist">
        <li>Make sure the channel shows as <strong>Verified</strong></li>
        <li>Scroll to the <strong>Subscriptions</strong> table</li>
        <li>Find the event — e.g. <code>post.failed</code></li>
        <li>Tick the checkbox under the channel where the alert should land</li>
      </ul>
      <div className="docs-callout">
        <strong>Important:</strong> only verified channels appear in the subscriptions table.
      </div>

      <h2 id="limitations">Limitations</h2>
      <DocsTable
        columns={["Limitation", "Reason"]}
        rows={[
          ["Email only delivers to your signup address", "To keep the verified-identity surface simple — custom addresses are on the roadmap"],
          ["SMS and in-app are not available yet", "Modeled in backend but not wired to a provider"],
          [
            "Webhook events are different from notifications",
            <span key="nt-webhooks">Use <Link href="/docs/api/webhooks">Developer webhooks</Link> for machine delivery</span>,
          ],
        ]}
      />

      <h2 id="next-steps">Next steps</h2>
      <div className="nt-next">
        <Link href="/docs/resources/slack-webhook" className="nt-next-card">
          <div className="nt-next-kicker">Channel setup</div>
          <div className="nt-next-title">Get a Slack webhook URL</div>
          <div className="nt-next-body">Exact UI steps in Slack to create an incoming webhook.</div>
        </Link>
        <Link href="/docs/resources/discord-webhook" className="nt-next-card">
          <div className="nt-next-kicker">Channel setup</div>
          <div className="nt-next-title">Get a Discord webhook URL</div>
          <div className="nt-next-body">Exact UI steps in Discord to create a channel webhook.</div>
        </Link>
        <Link href="/docs/api/webhooks" className="nt-next-card">
          <div className="nt-next-kicker">For machines</div>
          <div className="nt-next-title">Developer webhooks</div>
          <div className="nt-next-body">Push delivery for post status, account events, and other machine consumers.</div>
        </Link>
        <Link href="/settings/notifications" className="nt-next-card">
          <div className="nt-next-kicker">Configure</div>
          <div className="nt-next-title">Open Settings → Notifications</div>
          <div className="nt-next-body">Add a channel, test delivery, and manage subscriptions.</div>
        </Link>
      </div>
    </DocsPage>
  );
}

const styles = `
.nt-badges{display:flex;flex-wrap:wrap;gap:6px;margin:2px 0 18px}
.nt-badge{display:inline-flex;align-items:center;padding:4px 11px;border-radius:999px;background:var(--docs-bg-muted);border:1px solid var(--docs-border);color:var(--docs-text);font-size:11.5px;font-weight:600;letter-spacing:.01em}
.nt-note{font-size:14px;line-height:1.65;color:var(--docs-text-soft);margin:6px 0 14px;max-width:none}
.nt-note code{font-family:var(--docs-mono);font-size:12.5px}
.nt-flow{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;margin:14px 0 6px}
.nt-flow-step{display:grid;grid-template-columns:36px 1fr;gap:14px;align-items:start;padding:14px 16px;border:1px solid var(--docs-border);border-radius:14px;background:var(--docs-bg-elevated)}
.nt-flow-num{display:inline-flex;align-items:center;justify-content:center;width:28px;height:28px;border-radius:999px;background:color-mix(in srgb, var(--docs-link) 14%, var(--docs-bg-muted));color:var(--docs-link);font-size:13px;font-weight:700;border:1px solid color-mix(in srgb, var(--docs-link) 22%, var(--docs-border))}
.nt-flow-title{font-size:15px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text);margin-bottom:3px}
.nt-flow-sub{font-size:13.5px;line-height:1.6;color:var(--docs-text-soft)}
.nt-next{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin:14px 0 4px}
.nt-next-card{display:flex;flex-direction:column;gap:6px;padding:16px 18px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated);text-decoration:none;color:inherit;transition:border-color .15s ease,transform .15s ease,box-shadow .15s ease}
.nt-next-card:hover{border-color:color-mix(in srgb, var(--docs-link) 38%, var(--docs-border));transform:translateY(-1px);box-shadow:var(--docs-card-shadow);text-decoration:none}
.nt-next-kicker{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint)}
.nt-next-title{font-size:16px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text)}
.nt-next-body{font-size:13.5px;line-height:1.6;color:var(--docs-text-soft)}
@media (max-width:960px){
  .nt-flow{grid-template-columns:1fr}
  .nt-next{grid-template-columns:1fr}
}
`;
