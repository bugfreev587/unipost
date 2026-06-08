import type { MetadataRoute } from "next";
import { staticBlogPosts } from "@/lib/blog";

const BASE = "https://unipost.dev";

export default function sitemap(): MetadataRoute.Sitemap {
  const now = new Date();

  const staticPages: MetadataRoute.Sitemap = [
    { url: BASE, lastModified: now, changeFrequency: "weekly", priority: 1 },
    { url: `${BASE}/pricing`, lastModified: now, changeFrequency: "monthly", priority: 0.8 },
    { url: `${BASE}/blog`, lastModified: now, changeFrequency: "weekly", priority: 0.7 },
    { url: `${BASE}/docs`, lastModified: now, changeFrequency: "weekly", priority: 0.9 },
    { url: `${BASE}/solutions`, lastModified: now, changeFrequency: "monthly", priority: 0.7 },
    { url: `${BASE}/privacy`, lastModified: now, changeFrequency: "yearly", priority: 0.3 },
    { url: `${BASE}/terms`, lastModified: now, changeFrequency: "yearly", priority: 0.3 },
  ];

  const platformSlugs = [
    "instagram",
    "linkedin",
    "twitter",
    "tiktok",
    "youtube",
    "bluesky",
    "threads",
  ];

  const platformPages: MetadataRoute.Sitemap = platformSlugs.map((slug) => ({
    url: `${BASE}/${slug}-api`,
    lastModified: now,
    changeFrequency: "weekly" as const,
    priority: 0.8,
  }));

  const toolPages: MetadataRoute.Sitemap = [
    "/tools",
    "/tools/agentpost",
    "/tools/character-counter",
    "/tools/tiktok-analytics",
    "/tools/instagram-analytics",
    "/tools/threads-analytics",
    "/tools/pinterest-analytics",
  ].map((path) => ({
    url: `${BASE}${path}`,
    lastModified: now,
    changeFrequency: "weekly" as const,
    priority: path === "/tools" ? 0.7 : 0.6,
  }));

  const blogPages: MetadataRoute.Sitemap = staticBlogPosts.map((post) => ({
    url: `${BASE}/blog/${post.slug}`,
    lastModified: new Date(post.updatedAt),
    changeFrequency: "monthly" as const,
    priority: 0.6,
  }));

  return [...staticPages, ...platformPages, ...toolPages, ...blogPages];
}
