import type { Metadata } from "next";
import Link from "next/link";
import { BlogCover } from "@/app/blog/_components/blog-cover";
import { MarketingCTA } from "@/components/marketing/nav";
import { blogPosts } from "@/lib/blog";

export const metadata: Metadata = {
  title: "UniPost Blog | Social Media API Engineering Guides",
  description:
    "Technical guides about social media publishing APIs, multi-platform posting, account connection, media handling, webhooks, and AI agent publishing.",
  alternates: {
    canonical: "https://unipost.dev/blog",
  },
  openGraph: {
    title: "UniPost Blog",
    description:
      "Technical guides about social media publishing APIs, multi-platform posting, account connection, media handling, webhooks, and AI agent publishing.",
    url: "https://unipost.dev/blog",
    siteName: "UniPost",
    type: "website",
  },
};

export default function BlogIndexPage() {
  return (
    <div className="blog-page">
      <section className="blog-hero">
        <h1 className="blog-title">Get the most out of UniPost</h1>
        <p className="blog-sub">
          Technical guides, integration tutorials, and product notes for developers
          building social publishing, scheduling, and AI content workflows.
        </p>
      </section>

      <section className="blog-grid" aria-label="Blog posts">
        {blogPosts.map((post) => (
          <Link key={post.slug} href={`/blog/${post.slug}`} className="blog-card">
            <div className="blog-card-media">
              <BlogCover compact />
            </div>
            <div className="blog-card-body">
              <div className="blog-card-kicker">
                <span>{formatDate(post.publishedAt)}</span>
                <span>•</span>
                <span>{post.author}</span>
                <span>•</span>
                <span>{post.readingTime}</span>
              </div>
              <h2 className="blog-card-title">{post.title}</h2>
              <p className="blog-card-excerpt">{post.excerpt}</p>
              <span className="blog-card-arrow">Read post <span>→</span></span>
            </div>
          </Link>
        ))}
      </section>

      <section className="blog-index-cta">
        <h2>Start coding today</h2>
        <p>
          Connect accounts, publish posts, and track delivery across social platforms
          without rebuilding every integration yourself.
        </p>
        <div className="blog-index-cta-actions">
          <MarketingCTA label="Start Building" />
          <Link href="/docs" className="lp-btn lp-btn-outline lp-btn-lg">Read Docs</Link>
        </div>
      </section>
    </div>
  );
}

function formatDate(date: string) {
  return new Intl.DateTimeFormat("en", {
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  }).format(new Date(date));
}
