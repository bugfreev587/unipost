import Link from "next/link";
import { DocsPage } from "../_components/docs-shell";
import { ApiInlineLink } from "../api/_components/doc-components";
import {
  PublishingInputModeCards,
  PublishingLocalFileExample,
  PublishingLocalFileFlow,
} from "../_components/publishing-guide";

export default function PublishingGuidePage() {
  return (
    <DocsPage
      eyebrow="API Guide"
      title="Publishing guide"
      lead="Use this guide when your integration is ready to create posts through the UniPost API. It covers the shared publish path for hosted URLs, local media uploads, async publishing results, and the handoff to platform-specific options."
      className="docs-page-wide"
    >
      <h2 id="choose-input-mode">Choose input mode</h2>
      <p className="docs-note">
        UniPost accepts the same two media input patterns across supported
        platforms. Use <code>media_urls</code> when your asset is already hosted,
        or use <code>media_ids</code> when your app starts from local file bytes.
        Platform pages still define which media counts, formats, surfaces, and
        options are valid for each destination.
      </p>
      <PublishingInputModeCards />

      <PublishingLocalFileFlow />

      <PublishingLocalFileExample />

      <h2 id="publishing-result">Publishing result</h2>
      <p>
        <ApiInlineLink endpoint="POST /v1/posts" /> accepts immediate publish
        requests asynchronously and returns once UniPost has queued the work. Poll{" "}
        <ApiInlineLink endpoint="GET /v1/posts/:post_id" /> when your UI needs to
        show a live status, or subscribe to <Link href="/docs/api/webhooks">developer webhooks</Link>{" "}
        when your backend should receive final outcomes without polling.
      </p>
      <p>
        The aggregate post status tells you whether the whole request is still
        pending, succeeded, partially succeeded, or failed. Inspect per-destination
        results when a cross-post includes multiple platform accounts.
      </p>

      <h2 id="platform-specific-payloads">Platform-specific payloads</h2>
      <p>
        The publish flow is shared, but payload details are still platform-specific.
        For example, Instagram uses <code>platform_options.mediaType</code> to
        select feed, reels, stories, or carousel publishing; YouTube uses upload
        metadata and privacy options; TikTok has video and photo-post constraints.
      </p>
      <div className="docs-next-grid">
        <Link href="/docs/platforms/instagram#api-examples" className="docs-next-card">
          <div className="docs-next-kicker">Examples</div>
          <div className="docs-next-title">Instagram payloads</div>
          <div className="docs-next-body">Feed, reels, stories, and carousel examples.</div>
        </Link>
        <Link href="/docs/platforms/tiktok#api-examples" className="docs-next-card">
          <div className="docs-next-kicker">Examples</div>
          <div className="docs-next-title">TikTok payloads</div>
          <div className="docs-next-body">Video and photo carousel examples.</div>
        </Link>
        <Link href="/docs/platforms/youtube#api-examples" className="docs-next-card">
          <div className="docs-next-kicker">Examples</div>
          <div className="docs-next-title">YouTube payloads</div>
          <div className="docs-next-body">Hosted video, media library, and scheduled upload examples.</div>
        </Link>
        <Link href="/docs/platforms" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">All platform guides</div>
          <div className="docs-next-body">Media specs, constraints, analytics, inbox, and account connection modes.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
