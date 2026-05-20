import Link from "next/link";
import { DocsPage, DocsTable } from "../_components/docs-shell";

export default function DocsPricingPage() {
  return (
    <DocsPage
      eyebrow="Get Started"
      title="Pricing"
      lead="UniPost is priced by product stage, not raw post volume. Pick the tier that matches how you use the product — Free / API / Basic / Growth / Team / Enterprise — then scale within it."
    >
      <h2 id="when-to-read">When to read this page</h2>
      <p>Pricing influences architecture decisions: which surfaces you build against, whether you need white-label Connect, and how you split traffic across plans. Read this early if you&apos;re evaluating where UniPost fits in your product.</p>

      <h2 id="ladder">The ladder</h2>
      <DocsTable
        columns={["Tier", "Price", "Posts/mo", "What it unlocks"]}
        rows={[
          ["Free",   "$0",     "100",    "API + dashboard + 8 platforms (no X). Try without a credit card."],
          ["API",    "$10",    "1,000",  "Dashboard + API + MCP, all 9 platforms incl. X, and read-only Analytics API. Inbox is not included."],
          ["Basic",  "$19",    "2,500",  "Adds white-label for 1 platform, Inbox (DMs + comments), and full Analytics. Hosted onboarding still shows Powered by UniPost."],
          ["Growth", "$59",    "7,500",  "Adds full white-label / native mode across supported platforms plus optional removal of Powered by UniPost. 25 profiles, 3 team members."],
          ["Team",   "$149",   "25,000", "Adds RBAC (owner/admin/editor), per-member API keys, audit log, unlimited profiles + members."],
          ["Enterprise", "Custom", "Custom", "Custom volume, SLA, dedicated support, contract flexibility. Contact us."],
        ]}
      />

      <h2 id="picking-a-tier">Picking a tier</h2>
      <DocsTable
        columns={["You are", "Tier"]}
        rows={[
          ["Evaluating UniPost or building a hobby project", <span key="f">Free</span>],
          ["Building with the dashboard, REST API, MCP, and lightweight analytics", <span key="a">API</span>],
          ["Running UniPost as your day-to-day operating console (compose, Inbox, Analytics)", <span key="b">Basic</span>],
          ["Embedding UniPost into your own SaaS with one branded platform", <span key="b2">Basic</span>],
          ["Embedding UniPost into your own SaaS across multiple platforms or without UniPost attribution", <span key="g">Growth</span>],
          ["Running an agency or multi-operator team with role-based access", <span key="t">Team</span>],
        ]}
      />

      <h2 id="usage-controls">Usage and safety controls</h2>
      <p>Two limits apply to every plan:</p>
      <ul>
        <li><strong>Monthly post quota</strong> — the post number in the table above. Free is a hard cap: once a Free workspace reaches 100 posts/month, new publish requests are rejected until the next month or an upgrade. Paid plans use soft overage: posting continues over the cap, but sustained overage triggers an upgrade conversation. No surprise billing.</li>
        <li><strong>Per-account daily safety caps</strong> — each connected account has a daily ceiling to keep it from being flagged as a spam bot by the platform itself: X 20/day, Instagram 100/day, Facebook 100/day, Threads 250/day, others 50/day. UTC-day window. Failed posts don&apos;t count.</li>
      </ul>

      <h2 id="free-plan">Free plan and trials</h2>
      <p>The Free plan is the public trial path: 100 posts/month, no credit card, and no time limit. Paid plans do not include a separate time-limited trial; upgrade when you need X publishing, higher volume, Inbox, Analytics, white-label, or team workflows.</p>

      <h2 id="full-pricing">Full pricing</h2>
      <p>Side-by-side feature comparison and FAQs:</p>
      <p><Link href="/pricing">Open the full pricing page</Link></p>

      <h2 id="changing-tiers">Changing tiers</h2>
      <p>Upgrades take effect immediately and prorate. Downgrades take effect at the start of the next billing cycle. Plan changes do not invalidate API keys or disconnect social accounts.</p>
    </DocsPage>
  );
}
