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
      <div className="docs-badge-row">
        <span className="docs-badge">Email ready</span>
        <span className="docs-badge">Slack · Discord</span>
        <span className="docs-badge">4 event types</span>
        <span className="docs-badge">Dashboard-managed</span>
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
      <div className="docs-step-flow">
        <div className="docs-step-row">
          <div className="docs-step-number">1</div>
          <div>
            <div className="docs-step-title">Create a channel in UniPost</div>
            <div className="docs-step-copy">Open <Link href="/settings/notifications">Settings → Notifications</Link>.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">2</div>
          <div>
            <div className="docs-step-title">For Slack or Discord, create a webhook URL first</div>
            <div className="docs-step-copy">Use the <Link href="/docs/resources/slack-webhook">Slack guide</Link> or <Link href="/docs/resources/discord-webhook">Discord guide</Link>.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">3</div>
          <div>
            <div className="docs-step-title">Paste the URL into UniPost</div>
            <div className="docs-step-copy">Add channel → Slack / Discord Webhook → paste.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">4</div>
          <div>
            <div className="docs-step-title">Click Test</div>
            <div className="docs-step-copy">Confirm delivery before you trust production alerts to it.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">5</div>
          <div>
            <div className="docs-step-title">Turn on events</div>
            <div className="docs-step-copy">Use the <strong>Subscriptions</strong> table to pick which alerts go where.</div>
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
      <p className="docs-note">New workspaces are subscribed to all four events on the auto-provisioned email channel by default.</p>

      <h2 id="email-setup">Email</h2>
      <p className="docs-note">Email is the default channel. UniPost creates it from your signup email automatically — there is nothing to add.</p>
      <ul className="docs-checklist">
        <li>Open <Link href="/settings/notifications">Settings → Notifications</Link></li>
        <li>Find the built-in <strong>Email</strong> channel showing your signup email</li>
        <li>Use <strong>Test</strong> if you want to confirm delivery</li>
        <li>Turn events on in the <strong>Subscriptions</strong> table</li>
      </ul>
      <p className="docs-note">
        <strong>Current rule:</strong> UniPost only sends email to your signup email. You do not need to add it manually.
      </p>

      <h2 id="subscribe-events">Subscribe to events</h2>
      <ul className="docs-checklist">
        <li>Make sure the channel shows as <strong>Verified</strong></li>
        <li>Scroll to the <strong>Subscriptions</strong> table</li>
        <li>Find the event — e.g. <code>post.failed</code></li>
        <li>Tick the checkbox under the channel where the alert should land</li>
      </ul>
      <div className="docs-callout docs-callout-warning">
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
      <div className="docs-next-grid">
        <Link href="/docs/resources/slack-webhook" className="docs-next-card">
          <div className="docs-next-kicker">Channel setup</div>
          <div className="docs-next-title">Get a Slack webhook URL</div>
          <div className="docs-next-body">Exact UI steps in Slack to create an incoming webhook.</div>
        </Link>
        <Link href="/docs/resources/discord-webhook" className="docs-next-card">
          <div className="docs-next-kicker">Channel setup</div>
          <div className="docs-next-title">Get a Discord webhook URL</div>
          <div className="docs-next-body">Exact UI steps in Discord to create a channel webhook.</div>
        </Link>
        <Link href="/docs/api/webhooks" className="docs-next-card">
          <div className="docs-next-kicker">For machines</div>
          <div className="docs-next-title">Developer webhooks</div>
          <div className="docs-next-body">Push delivery for post status, account events, and other machine consumers.</div>
        </Link>
        <Link href="/settings/notifications" className="docs-next-card">
          <div className="docs-next-kicker">Configure</div>
          <div className="docs-next-title">Open Settings → Notifications</div>
          <div className="docs-next-body">Add a channel, test delivery, and manage subscriptions.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
