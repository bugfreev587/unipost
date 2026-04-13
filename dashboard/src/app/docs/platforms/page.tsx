import Link from "next/link";
import { DocsPage, DocsTable } from "../_components/docs-shell";

const PLATFORM_MATRIX = [
  ["Bluesky", "Yes", "Yes", "Yes", "Yes", "No", "Yes"],
  ["Twitter/X", "Yes", "Yes", "Yes", "Yes", "Yes", "Yes"],
  ["LinkedIn", "Yes", "Yes", "Yes", "No", "Yes", "Yes"],
  ["Instagram", "Media-first", "Yes", "Yes", "No", "Yes", "Yes"],
  ["Threads", "Yes", "Yes", "Yes", "Yes", "No", "Yes"],
  ["TikTok", "No", "Carousel only", "Yes", "No", "No", "Partial"],
  ["YouTube", "No", "No", "Yes", "No", "No", "Yes"],
];

export default function PlatformsPage() {
  return (
    <DocsPage
      eyebrow="Platforms"
      title="Platform guides built for implementation, not browsing."
      lead="Every platform page follows the same structure so a developer can answer the practical questions quickly: what UniPost supports there, what the platform requires, what the request should look like, and what validation errors to expect."
    >
      <h2 id="support-matrix">Support matrix</h2>
      <DocsTable
        columns={["Platform", "Text", "Images", "Video", "Threads", "First comment", "Analytics"]}
        rows={PLATFORM_MATRIX}
      />

      <h2 id="how-to-read">How to read these guides</h2>
      <DocsTable
        columns={["Section", "Why it exists"]}
        rows={[
          ["Overview", "Quickly decide whether the platform fits your publishing workflow"],
          ["Capabilities", "See what UniPost supports there today"],
          ["Requirements", "Understand field requirements, limits, and mixing rules"],
          ["Examples", "Copy a concrete request for text, image, video, or thread publishing"],
          ["Common errors", "Understand what UniPost validates before the request hits the platform"],
        ]}
      />

      <h2 id="cross-platform-rules">Cross-platform rules to remember</h2>
      <ul className="docs-list">
        <li><code>platform_posts[]</code> is the recommended request shape when different platforms need different copy or media.</li>
        <li>Most networks reject mixed image and video in a single post. Instagram and Threads only allow mixing inside carousel-style containers.</li>
        <li><code>thread_position</code> is the preferred way to model multi-post conversational flows. Do not assume every platform supports it.</li>
        <li><code>first_comment</code> is a platform-specific feature, not a universal one.</li>
      </ul>

      <h2 id="platform-list">Platform list</h2>
      <div className="docs-grid">
        {[
          ["Twitter/X", "twitter", "Best for text, threads, and first-comment style replies."],
          ["LinkedIn", "linkedin", "Best for longer-form captions and professional announcements."],
          ["Instagram", "instagram", "Media-first publishing with image, video, and carousel flows."],
          ["Threads", "threads", "Short conversational posts with strong thread support."],
          ["TikTok", "tiktok", "Video-led publishing with platform-specific privacy and upload controls."],
          ["YouTube", "youtube", "Single-video publishing with privacy, Shorts, category, and tag options."],
          ["Bluesky", "bluesky", "Short-form posts with strong support for threads and direct account setup."],
        ].map(([label, slug, summary]) => (
          <div className="docs-card" key={slug}>
            <div className="docs-card-title">{label}</div>
            <p>{summary}</p>
            <p><Link href={`/docs/platforms/${slug}`}>Open guide</Link></p>
          </div>
        ))}
      </div>
    </DocsPage>
  );
}
