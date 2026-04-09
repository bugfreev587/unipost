import type { Metadata } from "next";
import { ToolCard, type ToolCardData } from "@/components/tools/ToolCard";

export const metadata: Metadata = {
  title: "Free Social Media Tools for Developers | UniPost",
  description:
    "Free tools for social media developers. Character counter, AI caption generator, thread splitter, and more. Built by UniPost.",
  keywords: [
    "social media tools for developers",
    "free social media tools",
    "twitter character counter",
    "social media caption generator",
  ],
};

const TOOLS: ToolCardData[] = [
  {
    icon: "\u{1F916}",
    name: "AgentPost",
    description: "AI-powered multi-platform social posting",
    href: "/tools/agentpost",
    status: "live",
    badge: "New",
  },
  {
    icon: "\u{1F4CF}",
    name: "Character Counter",
    description: "Check post length for every platform",
    href: "/tools/character-counter",
    status: "live",
    badge: "New",
  },
  {
    icon: "\u{1F9F5}",
    name: "Thread Splitter",
    description: "Split long text into Twitter threads",
    href: "/tools/thread-splitter",
    status: "coming_soon",
  },
  {
    icon: "\u270D\uFE0F",
    name: "Caption Generator",
    description: "AI-written captions for every platform",
    href: "/tools/caption-generator",
    status: "coming_soon",
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
            to post to 7 platforms with one call.
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
