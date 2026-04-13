import Link from "next/link";
import { DocsPage, DocsTable } from "../_components/docs-shell";

export default function PlatformsPage() {
  return (
    <DocsPage
      eyebrow="Platforms"
      title="Platform guides built for implementation, not browsing."
      lead="Each platform page answers the same practical questions: what UniPost supports there, what the platform requires, what the request should look like, and what validation errors to expect."
    >
      <h2 id="how-platform-pages-work">How platform pages work</h2>
      <DocsTable
        columns={["Section", "Why it exists"]}
        rows={[
          ["Overview", "Quickly see whether the platform supports text, images, video, threads, first comments, and analytics"],
          ["Requirements", "Know the exact constraints before you build the request"],
          ["Examples", "Copy a request for text, image, video, or thread publishing"],
          ["Errors", "Understand what UniPost validates before the request hits the platform"],
        ]}
      />

      <h2 id="platform-list">Platform list</h2>
      <div className="docs-grid">
        {[
          ["Twitter/X", "twitter"],
          ["LinkedIn", "linkedin"],
          ["Instagram", "instagram"],
          ["Threads", "threads"],
          ["TikTok", "tiktok"],
          ["YouTube", "youtube"],
          ["Bluesky", "bluesky"],
        ].map(([label, slug]) => (
          <div className="docs-card" key={slug}>
            <div className="docs-card-title">{label}</div>
            <p>Support matrix, content rules, and example requests for {label}.</p>
            <p><Link href={`/docs/platforms/${slug}`}>Open guide</Link></p>
          </div>
        ))}
      </div>
    </DocsPage>
  );
}
