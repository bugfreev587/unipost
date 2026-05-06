import Link from "next/link";
import { DocsPage } from "../_components/docs-shell";
import { DocsQuickstartCard } from "@/components/tutorials/docs-quickstart-card";

export default function DashboardQuickstartPage() {
  return (
    <DocsPage
      eyebrow="Getting Started"
      title="Dashboard Quickstart"
      lead="Use the UniPost dashboard to connect social accounts, draft content, customize per platform, and publish your first post without writing code."
    >
      <DocsQuickstartCard tutorialId="quickstart" />

      <h2 id="what-youll-do">What you&apos;ll do</h2>
      <ul className="docs-checklist">
        <li>Connect at least one social account</li>
        <li>Open the create-post drawer in the dashboard</li>
        <li>Write a caption, add media, and customize per platform</li>
        <li>Publish now, save a draft, or schedule the post</li>
      </ul>

      <h2 id="step-1-connect-accounts">1. Connect your social accounts</h2>
      <p>
        Go to your workspace in the UniPost dashboard and open the connections area. Start by connecting the platform accounts you want to publish to first.
      </p>
      <p>
        If you are unsure which destinations are currently supported, check <Link href="/docs/platforms">Platforms</Link>.
      </p>

      <h2 id="step-2-open-compose">2. Open the create-post flow</h2>
      <p>
        Once at least one account is connected, open the create-post drawer from the posts area of the dashboard. This is where you write the main caption, upload media, and decide which connected accounts should receive the post.
      </p>

      <h2 id="step-3-customize-per-platform">3. Customize per platform</h2>
      <p>
        UniPost lets you compose once and then override the caption or platform-specific fields for each destination. This is useful when the same campaign needs a different tone on LinkedIn, X, Instagram, or TikTok.
      </p>

      <h2 id="step-4-choose-how-to-publish">4. Choose how to publish</h2>
      <ul className="docs-step-list">
        <li><strong>Publish now</strong> if the post is ready immediately.</li>
        <li><strong>Save draft</strong> if you want to come back later.</li>
        <li><strong>Schedule</strong> if you want it to go out at a specific time.</li>
        <li><strong>Queue</strong> if your workflow uses queued publishing.</li>
      </ul>

      <h2 id="step-5-publish-your-first-post">5. Publish your first post</h2>
      <p>
        Review the selected accounts, media, and any per-platform customization, then publish. If the composer reports validation issues, resolve them in the drawer before trying again.
      </p>

      <div className="docs-callout">
        <strong>Building an integration instead?</strong> Start with the <Link href="/docs/quickstart">API Quickstart</Link> if you want to publish programmatically rather than through the dashboard UI.
      </div>
    </DocsPage>
  );
}
