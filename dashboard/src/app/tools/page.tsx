import type { Metadata } from "next";
import { ToolCard, type ToolCardData } from "@/components/tools/ToolCard";

export const metadata: Metadata = {
  title: "Free Social Media Tools for Developers | UniPost",
  description:
    "Free tools for social media developers. Character counter, AI caption generator, thread splitter, and more. Built by UniPost.",
  alternates: { canonical: "https://unipost.dev/tools" },
  keywords: [
    "social media tools for developers",
    "free social media tools",
    "twitter character counter",
    "social media caption generator",
  ],
};

const TOOLS: ToolCardData[] = [
  {
    icon: "agentpost",
    name: "AgentPost",
    description: "AI-powered multi-platform social posting",
    href: "/tools/agentpost",
    status: "live",
    badge: "New",
  },
  {
    icon: "character-counter",
    name: "Character Counter",
    description: "Check post length for every platform",
    href: "/tools/character-counter",
    status: "live",
    badge: "New",
  },
  {
    icon: "tiktok",
    name: "TikTok Analytics",
    description: "Preview TikTok profile, video, and post analytics",
    href: "/tools/tiktok-analytics",
    status: "live",
  },
  {
    icon: "youtube",
    name: "YouTube Analytics",
    description: "Preview YouTube channel, trend, and top video reports",
    href: "/tools/youtube-analytics",
    status: "live",
  },
  {
    icon: "instagram",
    name: "Instagram Analytics",
    description: "Preview Instagram Business media and post insights",
    href: "/tools/instagram-analytics",
    status: "live",
  },
  {
    icon: "threads",
    name: "Threads Analytics",
    description: "Preview Threads profile and post performance",
    href: "/tools/threads-analytics",
    status: "live",
  },
  {
    icon: "pinterest",
    name: "Pinterest Analytics",
    description: "Preview Pin, board, save, and click analytics",
    href: "/tools/pinterest-analytics",
    status: "live",
  },
];

export default function ToolsPage() {
  return (
    <div className="tl-page">
      {/* Hero */}
      <div className="tl-hero">
        <div className="tl-eyebrow">Developer Tools</div>
        <h1 className="tl-hero-title">
          Free tools for social media <em>developers</em>
        </h1>
        <p className="tl-hero-sub">
          Built by UniPost. Free forever. No sign-up required.
        </p>
      </div>

      {/* Grid */}
      <div className="tl-grid">
        {TOOLS.map((t) => (
          <ToolCard key={t.name} tool={t} />
        ))}
      </div>

      {/* CTA */}
      <div className="tl-cta">
        <div className="tl-cta-inner">
          <div className="tl-cta-glow" />
          <h2 className="tl-cta-title">These tools are built on UniPost API</h2>
          <p className="tl-cta-sub">
            Need to post programmatically? UniPost gives your app a unified API
            to post to 9 platforms with one call.
          </p>
          <div className="tl-cta-actions">
            <a href="https://app.unipost.dev" className="lp-btn lp-btn-primary lp-btn-lg">
              Get API Access
            </a>
            <a href="/pricing" className="lp-btn lp-btn-outline lp-btn-lg">
              View Pricing
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}
