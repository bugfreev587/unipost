import Link from "next/link";
import { DocsPage, DocsTable } from "../../../_components/docs-shell";
import { X_CREDIT_PLANS } from "@/data/x-credits-catalog.generated";
import { requirePublicDocsFeature } from "@/lib/public-feature-flags-server";

export default async function XCreditsGuidePage() {
  const publicFeatureFlags = await requirePublicDocsFeature("x_credits_billing_v1");

  return (
    <DocsPage
      eyebrow="X Guides"
      title="Plan and monitor X Credits"
      lead="Estimate managed-X usage, inspect the live monthly allowance, understand reset and safety caps, and handle hard-limit exhaustion without confusing X Credits with posts/month."
      className="docs-page-guide-redesign"
    >
      <div className="docs-guide-badges">
        <span className="docs-guide-badge">Managed X only</span>
        <span className="docs-guide-badge">Resets each billing period</span>
        <span className="docs-guide-badge">BYO does not consume UniPost Credits</span>
        <span className="docs-guide-badge">20 X posts/account/day still applies</span>
      </div>
      <p className="docs-guide-note">
        X Credits billing is in a controlled rollout. Until it is enabled for the workspace, managed X operations do
        not count against or block on the customer monthly balance, and
        <Link href="/docs/api/x-credits"> GET /v1/billing/x-credits</Link> returns
        <code> FEATURE_NOT_AVAILABLE</code>. Internal inbound cost safety and the independent publish cap still apply.
      </p>

      <h2 id="estimate">1. Estimate the operation mix</h2>
      <p>
        Start from the final X text. A conclusively URL-free X post uses 15 Credits; a post containing a URL or
        domain-like candidate is conservatively counted at 200 Credits. A complete Inbox comment interaction
        combines one received comment and one reply. A complete DM interaction combines one received DM and one sent DM.
      </p>
      <DocsTable
        columns={["Plan", "Included", "Normal posts", "URL posts", "Complete comments", "Complete DMs"]}
        rows={X_CREDIT_PLANS.map((plan) => [
          plan.label,
          plan.monthly_allowance == null ? "Custom" : plan.monthly_allowance.toLocaleString(),
          plan.capacity == null ? "Custom" : plan.capacity.normal_posts.toLocaleString(),
          plan.capacity == null ? "Custom" : plan.capacity.url_posts.toLocaleString(),
          !plan.inbox_eligible ? "Inbox not included" : plan.capacity == null ? "Custom" : plan.capacity.comment_interactions.toLocaleString(),
          !plan.inbox_eligible ? "Inbox not included" : plan.capacity == null ? "Custom" : plan.capacity.dm_interactions.toLocaleString(),
        ])}
      />
      <p className="docs-guide-note">
        Each operation column assumes the entire shared allowance is used for that operation. Real workloads mix
        operations, so calculate against the weighted total rather than adding the columns together.
      </p>

      <h2 id="inspect">2. Inspect the live allowance</h2>
      <p>
        Call <Link href="/docs/api/x-credits">GET /v1/billing/x-credits</Link> before large managed-X batches and
        display <code>monthly_remaining</code> plus <code>billing_period_end</code> in operator-facing UI.
      </p>
      <pre><code>{`curl "https://api.unipost.dev/v1/billing/x-credits" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`}</code></pre>

      <h2 id="validate">3. Validate before publishing</h2>
      <p>
        Use <Link href="/docs/api/posts/validate">POST /v1/posts/validate</Link> to catch request and platform
        validation errors before publishing. Validation does not consume X Credits. For the final publish contract,
        see <Link href="/docs/api/posts/create">POST /v1/posts</Link>.
      </p>

      <h2 id="exhaustion">4. Handle exhaustion</h2>
      <ol className="docs-checklist">
        <li>Branch on <code>x_monthly_usage_limit_exceeded</code>; do not parse the message.</li>
        <li>Stop retry loops for the same managed-X operation.</li>
        <li>Show the reset date and the current plan&apos;s upgrade or Enterprise contact path.</li>
        <li>Keep BYO X operations separate because they do not consume UniPost X Credits.</li>
        <li>For inbound delivery, also branch on <code>x_inbound_daily_cap_exceeded</code> and show the UTC reset boundary.</li>
      </ol>
      <p className="docs-guide-note">
        Managed-X work stops at the hard limit. The allowance does not override the independent safety cap of 20 X
        posts per connected account per UTC day, X rate limits, spam controls, or content-policy enforcement.
      </p>

      <h2 id="related">Related references</h2>
      <div className="docs-guide-next">
        <Link href="/docs/api/x-credits" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">API Reference</div>
          <div className="docs-guide-next-title">X Credits allowance</div>
          <div className="docs-guide-next-body">Fields, operation catalog, connection modes, and errors.</div>
        </Link>
        <Link href="/docs/api/posts/create" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">Publishing</div>
          <div className="docs-guide-next-title">Create post</div>
          <div className="docs-guide-next-body">Submit the final managed-X operation.</div>
        </Link>
        <Link href="/docs/guides/x/comments" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">Inbox</div>
          <div className="docs-guide-next-title">X comments</div>
          <div className="docs-guide-next-body">List, reply, sync, and handle managed-X boundaries.</div>
        </Link>
        {publicFeatureFlags.x_dms_v1 ? (
          <Link href="/docs/guides/x/direct-messages" className="docs-guide-next-card">
            <div className="docs-guide-next-kicker">Inbox</div>
            <div className="docs-guide-next-title">X direct messages</div>
            <div className="docs-guide-next-body">Work with private legacy DM threads safely.</div>
          </Link>
        ) : null}
        <Link href="/docs/api/posts/validate" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">Preflight</div>
          <div className="docs-guide-next-title">Validate post</div>
          <div className="docs-guide-next-body">Catch request errors without consuming X Credits.</div>
        </Link>
        <Link href="/docs/pricing" className="docs-guide-next-card">
          <div className="docs-guide-next-kicker">Plans</div>
          <div className="docs-guide-next-title">Compare included capacity</div>
          <div className="docs-guide-next-body">Review allowance and plan eligibility.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
