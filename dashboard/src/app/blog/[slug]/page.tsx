import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { Fragment, type ReactNode } from "react";
import { BlogCover } from "@/app/blog/_components/blog-cover";
import { blogPosts, countBlogWords, getBlogPost, type BlogBlock, type BlogPost } from "@/lib/blog";

const APP_URL = process.env.NEXT_PUBLIC_APP_URL || "https://app.unipost.dev";
const START_BUILDING_URL = `${APP_URL}/welcome`;

type BlogPostPageProps = {
  params: Promise<{ slug: string }>;
};

export function generateStaticParams() {
  return blogPosts.map((post) => ({ slug: post.slug }));
}

export async function generateMetadata({ params }: BlogPostPageProps): Promise<Metadata> {
  const { slug } = await params;
  const post = getBlogPost(slug);

  if (!post) {
    return {};
  }

  const url = `https://unipost.dev/blog/${post.slug}`;
  const ogImage = `${url}/opengraph-image`;

  return {
    title: post.seoTitle,
    description: post.description,
    keywords: post.keywords,
    alternates: {
      canonical: url,
    },
    openGraph: {
      title: post.seoTitle,
      description: post.description,
      url,
      siteName: "UniPost",
      type: "article",
      publishedTime: post.publishedAt,
      modifiedTime: post.updatedAt,
      authors: [post.author],
      images: [{ url: ogImage, width: 1200, height: 630, alt: post.title }],
    },
    twitter: {
      card: "summary_large_image",
      title: post.seoTitle,
      description: post.description,
      images: [ogImage],
    },
  };
}

export default async function BlogPostPage({ params }: BlogPostPageProps) {
  const { slug } = await params;
  const post = getBlogPost(slug);

  if (!post) {
    notFound();
  }

  const url = `https://unipost.dev/blog/${post.slug}`;
  const ogImage = `${url}/opengraph-image`;
  const wordCount = countBlogWords(post);

  const articleJsonLd = {
    "@context": "https://schema.org",
    "@type": "TechArticle",
    headline: post.title,
    description: post.description,
    image: [ogImage],
    datePublished: post.publishedAt,
    dateModified: post.updatedAt,
    inLanguage: "en-US",
    wordCount,
    keywords: post.keywords.join(", "),
    author: {
      "@type": "Organization",
      name: post.author,
      url: "https://unipost.dev",
    },
    publisher: {
      "@type": "Organization",
      name: "UniPost",
      url: "https://unipost.dev",
      logo: {
        "@type": "ImageObject",
        url: "https://unipost.dev/brand/unipost-icon-128.png",
      },
    },
    mainEntityOfPage: url,
  };

  const faqJsonLd = buildFaqJsonLd(post);

  return (
    <article className="blog-article-wrap">
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(articleJsonLd) }}
      />
      {faqJsonLd ? (
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(faqJsonLd) }}
        />
      ) : null}
      <Link href="/blog" className="blog-back">Back to blog</Link>
      <header>
        <div className="blog-article-kicker">
          <span>{post.category}</span>
          <span>{formatDate(post.publishedAt)}</span>
          <span>{post.readingTime}</span>
        </div>
        <h1 className="blog-article-title">{post.title}</h1>
        <p className="blog-article-desc">{post.description}</p>
      </header>
      <div className="blog-article-cover">
        <BlogCover />
      </div>

      <div className="blog-body">
        {post.blocks.map((block, index) => (
          <BlogContentBlock key={`${block.type}-${index}`} block={block} />
        ))}
      </div>

      <aside className="blog-cta">
        <h2>Build social publishing without owning every platform integration.</h2>
        <p>
          UniPost gives your app hosted account connection, media handling, validation,
          publishing, delivery status, and webhooks through one API.
        </p>
        <div className="blog-cta-actions">
          <a href={START_BUILDING_URL} className="lp-btn lp-btn-primary lp-btn-lg">Start Building</a>
          <Link href="/docs" className="lp-btn lp-btn-outline lp-btn-lg">Read Docs</Link>
        </div>
      </aside>
    </article>
  );
}

function BlogContentBlock({ block }: { block: BlogBlock }) {
  if (block.type === "lead") {
    return <p className="lead">{renderInline(block.text)}</p>;
  }

  if (block.type === "paragraph") {
    return <p>{renderInline(block.text)}</p>;
  }

  if (block.type === "heading") {
    return <h2 id={slugifyHeading(block.text)}>{block.text}</h2>;
  }

  if (block.type === "summary") {
    return (
      <aside className="blog-summary" aria-label={block.title || "Summary"}>
        {block.title ? <div className="blog-summary-title">{block.title}</div> : null}
        <ul>
          {block.items.map((item, i) => (
            <li key={i}>{renderInline(item)}</li>
          ))}
        </ul>
      </aside>
    );
  }

  if (block.type === "list") {
    return (
      <ul>
        {block.items.map((item, i) => (
          <li key={i}>{renderInline(item)}</li>
        ))}
      </ul>
    );
  }

  if (block.type === "code") {
    return (
      <pre className="blog-code" aria-label={`${block.language} code example`}>
        <code>{block.code}</code>
      </pre>
    );
  }

  if (block.type === "table") {
    return (
      <figure className="blog-table-wrap">
        <table className="blog-table">
          <thead>
            <tr>
              {block.headers.map((header) => (
                <th key={header}>{header}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {block.rows.map((row, ri) => (
              <tr key={ri}>
                {row.map((cell, ci) => (
                  <td key={ci}>{renderInline(cell)}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
        {block.caption ? <figcaption>{block.caption}</figcaption> : null}
      </figure>
    );
  }

  if (block.type === "faq") {
    return (
      <div className="blog-faq">
        {block.items.map((item, i) => (
          <details key={i} className="blog-faq-item">
            <summary>{item.question}</summary>
            <p>{renderInline(item.answer)}</p>
          </details>
        ))}
      </div>
    );
  }

  return (
    <div className="blog-note">
      <div className="blog-note-title">{block.title}</div>
      <p>{renderInline(block.text)}</p>
    </div>
  );
}

// Tiny inline renderer: parses [text](url) markdown-style links and `code` spans.
// Internal links (starting with "/") render as Next.js <Link>; others render as <a>.
function renderInline(input: string): ReactNode {
  const tokens: ReactNode[] = [];
  const pattern = /\[([^\]]+)\]\(([^)]+)\)|`([^`]+)`/g;
  let last = 0;
  let key = 0;
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(input)) !== null) {
    if (match.index > last) {
      tokens.push(<Fragment key={key++}>{input.slice(last, match.index)}</Fragment>);
    }
    if (match[1] && match[2]) {
      const text = match[1];
      const href = match[2];
      if (href.startsWith("/")) {
        tokens.push(
          <Link href={href} key={key++}>
            {text}
          </Link>,
        );
      } else {
        const isExternal = /^https?:\/\//.test(href);
        tokens.push(
          <a
            href={href}
            key={key++}
            {...(isExternal ? { target: "_blank", rel: "noopener noreferrer" } : {})}
          >
            {text}
          </a>,
        );
      }
    } else if (match[3]) {
      tokens.push(<code key={key++}>{match[3]}</code>);
    }
    last = pattern.lastIndex;
  }
  if (last < input.length) {
    tokens.push(<Fragment key={key++}>{input.slice(last)}</Fragment>);
  }
  return tokens.length > 0 ? <>{tokens}</> : input;
}

function buildFaqJsonLd(post: BlogPost) {
  const faqBlock = post.blocks.find((b) => b.type === "faq");
  if (!faqBlock || faqBlock.type !== "faq" || faqBlock.items.length === 0) {
    return null;
  }
  return {
    "@context": "https://schema.org",
    "@type": "FAQPage",
    mainEntity: faqBlock.items.map((item) => ({
      "@type": "Question",
      name: item.question,
      acceptedAnswer: {
        "@type": "Answer",
        text: item.answer,
      },
    })),
  };
}

function slugifyHeading(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, "")
    .trim()
    .replace(/\s+/g, "-")
    .slice(0, 64);
}

function formatDate(date: string) {
  return new Intl.DateTimeFormat("en", {
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  }).format(new Date(date));
}
