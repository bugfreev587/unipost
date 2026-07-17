import Link from "next/link";
import { DocsPage, DocsTable } from "../_components/docs-shell";
import { ApiInlineLink } from "../api/_components/doc-components";
import { X_CREDIT_PLANS } from "@/data/x-credits-catalog.generated";

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
          ["Enterprise", "Custom", "Contract", "Unlimited", "Contract-defined usage terms, capacity planning, SLA, dedicated support, and security review."],
        ]}
      />

      <h2 id="x-credits">Included X Credits</h2>
      <p className="docs-guide-note">
        <strong>X Credits are separate from posts/month.</strong>{" "}
        They apply to managed X API usage after the X Credits billing rollout is enabled, reset each billing period,
        and stop new managed-X work at the hard limit. Until then, regular workspaces do not consume or block on the
        customer monthly balance, although internal cost-safety limits remain active.
        Bring-your-own X API connections do not consume UniPost X Credits. See the{" "}
        <Link href="/docs/api/x-credits">X Credits API Reference</Link> and the{" "}
        <Link href="/docs/guides/x/credits">X Credits planning guide</Link>.
      </p>
      <DocsTable
        columns={["Plan", "Credits", "Normal posts", "URL posts", "Complete comments", "Complete DMs"]}
        rows={X_CREDIT_PLANS.map((plan) => [
          plan.label,
          plan.monthly_allowance == null ? "Custom" : plan.monthly_allowance.toLocaleString(),
          plan.capacity == null ? "Custom" : plan.capacity.normal_posts.toLocaleString(),
          plan.capacity == null ? "Custom" : plan.capacity.url_posts.toLocaleString(),
          !plan.inbox_eligible
            ? "Inbox not included"
            : plan.capacity == null
              ? "Custom"
              : plan.capacity.comment_interactions.toLocaleString(),
          !plan.inbox_eligible
            ? "Inbox not included"
            : plan.capacity == null
              ? "Custom"
              : plan.capacity.dm_interactions.toLocaleString(),
        ])}
      />
      <p className="docs-guide-note">
        Each operation column assumes the full shared allowance is spent on that one operation type. A complete
        comment means one received comment plus one reply; a complete DM means one received DM plus one sent DM.
        Comment and DM figures are capacity planning for phased X Inbox support and do not indicate API availability
        before that production phase ships. The independent safety cap of 20 X posts per connected account per UTC day still applies.
      </p>

      <h2 id="usage-limits">Usage limits</h2>
      <p className="docs-guide-note">
        Free workspaces enforce hard limits. Paid self-serve plans are designed to
        keep production integrations running while usage is reviewed.
      </p>
      <dl className="docs-guide-key-values">
        <div className="docs-guide-key-item">
          <dt className="docs-guide-key-label">Monthly post quota</dt>
          <dd className="docs-guide-key-copy">
            Free stops accepting new publish requests after 100 posts/month. API,
            Basic, and Growth use soft overage: posting continues for now, with
            usage warnings and upgrade guidance instead of surprise billing.
            Team has no monthly UniPost post quota. Enterprise Custom means contract-defined terms and may include no UniPost monthly post quota, custom capacity terms, or account-specific guarantees.
          </dd>
        </div>
        <div className="docs-guide-key-item">
          <dt className="docs-guide-key-label">Active scheduled posts</dt>
          <dd className="docs-guide-key-copy">
            Free workspaces can hold up to 50 undeleted parent posts in scheduled
            status. Published, failed, partial, draft, and cancelled posts do not
            count toward this cap.
          </dd>
        </div>
        <div className="docs-guide-key-item">
          <dt className="docs-guide-key-label">Paid scheduling</dt>
          <dd className="docs-guide-key-copy">
            API, Basic, Growth, Team, and Enterprise do not cap active scheduled
            backlog.
          </dd>
        </div>
        <div className="docs-guide-key-item">
          <dt className="docs-guide-key-label">Safety caps</dt>
          <dd className="docs-guide-key-copy">
            Each connected account still has a daily platform-safety ceiling.
            Failed posts do not count toward these safety caps. Enterprise contracts can plan around high-volume platform usage, but they cannot override platform-owned rate limits, app review, spam controls, or content policy enforcement.
          </dd>
        </div>
      </dl>

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
