import Link from "next/link";
import { DocsPage, DocsTable } from "../_components/docs-shell";
import { ApiInlineLink } from "../api/_components/doc-components";

export default function DocsPricingPage() {
  return (
    <DocsPage
      breadcrumbItems={[
        { label: "Using the API", href: "/docs/quickstart" },
        { label: "Plans and limits" },
      ]}
      title="Plans and limits"
      lead="Use this reference to understand how UniPost plans affect API behavior: monthly post quotas, active scheduled backlog, media retention, platform access, and the errors your integration should handle."
      className="docs-page-guide-redesign"
    >
      <div className="docs-guide-badges">
        <span className="docs-guide-badge">Free quota: 100 posts/month</span>
        <span className="docs-guide-badge">Free scheduled: 50 active posts</span>
        <span className="docs-guide-badge">Media retention: status driven</span>
        <span className="docs-guide-badge">Paid scheduled: unlimited</span>
      </div>
      <p className="docs-guide-note">
        <strong>This is the developer reference.</strong>{" "}
        For buyer-facing plan comparison, price cards, and upgrade decisions,
        open the <Link href="/pricing">full pricing page</Link>.
      </p>

      <h2 id="plan-behavior">Plan behavior</h2>
      <p className="docs-guide-note">
        The public pricing page explains who each plan is for. This table focuses on
        the behavior your application sees when it calls the API.
      </p>
      <DocsTable
        columns={["Plan", "Posts/month", "Quota behavior", "Active scheduled", "API behavior"]}
        rows={[
          ["Free", "100", "Hard cap", "50", "Dashboard + API. Publishing to X is not available on Free."],
          ["API", "1,000", "Soft overage", "Unlimited", "Dashboard + API + MCP, all 9 platforms including X, read-only Analytics API."],
          ["Basic", "2,500", "Soft overage", "Unlimited", "Adds Inbox, full Analytics, and one shared custom platform slot."],
          ["Growth", "7,500", "Soft overage", "Unlimited", "Adds Hosted Connect branding and Platform Credentials across supported platforms."],
          ["Team", "Unlimited", "Unlimited", "Unlimited", "Adds RBAC, per-member API keys, audit log, unlimited profiles and members."],
          ["Enterprise", "Custom", "Contract", "Unlimited", "SLA, dedicated support, security review, and contract flexibility."],
        ]}
      />

      <h2 id="usage-limits">Usage limits</h2>
      <p className="docs-guide-note">
        Free workspaces enforce hard limits. Paid self-serve plans are designed to
        keep production integrations running while usage is reviewed.
      </p>
      <ul className="docs-checklist">
        <li>
          <strong>Monthly post quota:</strong> Free stops accepting new publish
          requests after 100 posts/month. API, Basic, and Growth use soft overage:
          posting continues for now, with usage warnings and upgrade guidance
          instead of surprise billing.
        </li>
        <li>
          <strong>Active scheduled posts:</strong> Free workspaces can hold up to
          50 undeleted parent posts in scheduled status. Published, failed,
          partial, draft, and cancelled posts do not count toward this cap.
        </li>
        <li>
          <strong>Paid scheduling:</strong> API, Basic, Growth, Team, and
          Enterprise do not cap active scheduled backlog.
        </li>
        <li>
          <strong>Safety caps:</strong> each connected account still has a daily
          platform-safety ceiling. Failed posts do not count toward these safety
          caps.
        </li>
      </ul>

      <h2 id="media-retention">Media retention</h2>
      <p className="docs-guide-note">
        UniPost keeps media while a post can still need it. Cleanup starts only
        after the parent post reaches a final status, and reused media is deleted
        only after all post usages for that media are due.
      </p>
      <DocsTable
        columns={["Plan", "After success", "After failed, partial, or cancelled"]}
        rows={[
          ["Free", "1 day", "2 days"],
          ["API", "2 days", "4 days"],
          ["Basic", "4 days", "8 days"],
          ["Growth", "15 days", "30 days"],
          ["Team", "30 days", "60 days"],
          ["Enterprise", "30 days", "60 days unless your contract says otherwise"],
        ]}
      />
      <p className="docs-guide-note">
        <strong>Scheduled posts keep their media.</strong>
        R2 cleanup is driven by UniPost post state, not by object age. Scheduled,
        draft, queued, publishing, and processing posts keep uploaded media until
        they finish.
      </p>

      <h2 id="api-errors">API errors to handle</h2>
      <p className="docs-guide-note">
        Plan limits surface as normalized API errors. Your integration should
        branch on <code>normalized_code</code>, not only on the human-readable
        message.
      </p>
      <DocsTable
        columns={["Scenario", "HTTP", "Normalized code", "Where it appears"]}
        rows={[
          [
            "Free monthly post quota exceeded",
            "402",
            "`plan_limit_exceeded`",
            <ApiInlineLink key="quota" endpoint="POST /v1/posts" />,
          ],
          [
            "Free active scheduled backlog exceeded",
            "402",
            "`plan_scheduled_post_limit_exceeded`",
            <ApiInlineLink key="scheduled" endpoint="POST /v1/posts" />,
          ],
        ]}
      />
      <p className="docs-guide-note">
        See <Link href="/docs/api/posts/create#errors">Create post errors</Link>{" "}
        for response examples and the scheduled-post idempotency behavior.
      </p>

      <h2 id="changing-plans">Changing plans</h2>
      <p className="docs-guide-note">
        Upgrades take effect immediately and prorate. Downgrades take effect at
        the start of the next billing cycle. Plan changes do not invalidate API
        keys or disconnect social accounts.
      </p>

      <h2 id="next-steps">Next steps</h2>
      <div className="docs-guide-next">
        <Link href="/pricing" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">Pricing</div>
          <div className="docs-guide-next-title">Compare plan prices</div>
          <div className="docs-guide-next-body">Open the buyer-facing pricing page for plan cards and FAQs.</div>
        </Link>
        <Link href="/docs/api/posts/create#errors" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">API</div>
          <div className="docs-guide-next-title">Create post errors</div>
          <div className="docs-guide-next-body">Handle monthly quota and Free scheduled cap errors.</div>
        </Link>
        <Link href="/docs/api/media/reserve" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">Media</div>
          <div className="docs-guide-next-title">Reserve uploads</div>
          <div className="docs-guide-next-body">Upload local media and understand status-driven retention.</div>
        </Link>
        <Link href="/docs/publishing" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">Guide</div>
          <div className="docs-guide-next-title">Publishing guide</div>
          <div className="docs-guide-next-body">Follow the end-to-end publish path for hosted URLs and media IDs.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
