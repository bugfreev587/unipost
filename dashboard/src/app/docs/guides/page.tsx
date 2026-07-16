import Link from "next/link";
import { DocsPage } from "../_components/docs-shell";

export default function GuidesIndexPage() {
  return (
    <DocsPage
      eyebrow="Guides"
      title="Task guides for UniPost integrations"
      lead="Use Guides when you know the outcome you want and need the shortest path to the right UniPost API. API Reference stays focused on endpoint contracts."
      className="docs-page-wide"
    >
      <div className="docs-grid">
        <Link href="/docs/guides/publish-gifs" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Publish GIFs</div>
          <p>Publish a hosted or local GIF to X and Facebook, compare platform support, and prepare for upcoming conversion workflows.</p>
        </Link>
        <Link href="/docs/guides/x/comments" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">X comments</div>
          <p>List eligible X replies, send idempotent responses, and run a bounded public-reply backfill.</p>
        </Link>
        <Link href="/docs/guides/x/direct-messages" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">X direct messages</div>
          <p>Receive and respond to legacy X DM events while protecting private conversation data.</p>
        </Link>
        <Link href="/docs/guides/x/reconnect-permissions" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Reconnect X Inbox permissions</div>
          <p>Inspect capability state, complete workspace-app credentials, and grant the current X scopes.</p>
        </Link>
        <Link href="/docs/guides/x/credits" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">X Credits</div>
          <p>Estimate managed-X usage, inspect the monthly allowance, and handle hard-limit exhaustion.</p>
        </Link>
        <Link href="/docs/guides/platform-options" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Platform options examples</div>
          <p>Copy safe platform_posts[] options for YouTube, Instagram, TikTok, Facebook, and Pinterest.</p>
        </Link>
        <Link href="/docs/guides/instagram-stories" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Instagram Stories</div>
          <p>Publish a single Instagram Story with the strict platform_posts[] shape and avoid feed-post fallback.</p>
        </Link>
        <Link href="/docs/guides/video-audio-overlay" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Video + audio overlay</div>
          <p>Upload a user's video and audio, generate a combined video, then publish the processed output.</p>
        </Link>
        <Link href="/docs/guides/analytics" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Analytics guides</div>
          <p>Get account metrics, TikTok followers, UniPost-published post analytics, exports, and reconnect guidance.</p>
        </Link>
        <Link href="/docs/publishing" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Publishing guide</div>
          <p>Create posts through the shared publishing flow, including hosted URLs, local media uploads, and status handling.</p>
        </Link>
        <Link href="/docs/connect-sessions" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Connect Sessions</div>
          <p>Let customers connect their own social accounts through UniPost's hosted OAuth flow.</p>
        </Link>
        <Link href="/docs/local-connect-test" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Local Connect testing</div>
          <p>Create a Connect Session from your terminal and open the OAuth flow locally.</p>
        </Link>
      </div>

      <h2 id="how-guides-work">How Guides work</h2>
      <p>
        Guides answer workflow questions first, then link back to API Reference for exact request, response, and error details.
        If you already know the endpoint, start in <Link href="/docs/api">API Reference</Link>.
      </p>

      <h2 id="start-with-publishing">Start with publishing workflows</h2>
      <p>
        If a user wants to publish a GIF, start with{" "}
        <Link href="/docs/guides/publish-gifs">Publish GIFs</Link>. It shows the current X and Facebook workflow,
        platform-wide support status, local media upload steps, and the planned GIF-to-MP4 path.
      </p>
      <p>
        If a user wants an Instagram Story rather than a normal feed post, start with{" "}
        <Link href="/docs/guides/instagram-stories">Instagram Stories</Link>. It shows the strict request shape,
        validation behavior, and common error codes for story publishing.
      </p>
      <p>
        If a user brings their own video and audio files, use{" "}
        <Link href="/docs/guides/video-audio-overlay">Video + audio overlay</Link>. It shows how the upload, processing,
        and publish APIs fit together from the user's point of view.
      </p>

      <h2 id="start-with-analytics">Start with Analytics</h2>
      <p>
        The Analytics guide set is built around common questions that are hard to answer from endpoint names alone, such as
        which UniPost API returns TikTok followers and which fields to read from the response.
      </p>
      <p>
        Start here: <Link href="/docs/guides/analytics">Analytics guides</Link>
      </p>
    </DocsPage>
  );
}
