import type { MetadataRoute } from "next";
import { ALL_COMPETITORS } from "@/data/competitors";
import { MONEY_PAGES, SOLUTION_PAGES } from "@/data/seo-growth-pages";
import { SEO_RESOURCES } from "@/data/seo-resources";
import { staticBlogPosts } from "@/lib/blog";

const BASE = "https://unipost.dev";

export default function sitemap(): MetadataRoute.Sitemap {
  const now = new Date();

  const staticRoutes = [
    { path: "", changeFrequency: "weekly", priority: 1 },
    { path: "/pricing", changeFrequency: "monthly", priority: 0.8 },
    { path: "/about", changeFrequency: "monthly", priority: 0.7 },
    { path: "/compare", changeFrequency: "monthly", priority: 0.8 },
    { path: "/compare/social-media-apis", changeFrequency: "monthly", priority: 0.8 },
    { path: "/blog", changeFrequency: "weekly", priority: 0.7 },
    { path: "/docs", changeFrequency: "weekly", priority: 0.9 },
    { path: "/changelog", changeFrequency: "weekly", priority: 0.7 },
    { path: "/solutions", changeFrequency: "monthly", priority: 0.7 },
    { path: "/resources", changeFrequency: "monthly", priority: 0.7 },
    { path: "/privacy", changeFrequency: "yearly", priority: 0.3 },
    { path: "/terms", changeFrequency: "yearly", priority: 0.3 },
  ] as const;

  const staticPages: MetadataRoute.Sitemap = staticRoutes.map((route) => ({
    url: `${BASE}${route.path}`,
    lastModified: now,
    changeFrequency: route.changeFrequency,
    priority: route.priority,
  }));

  const platformSlugs = [
    "instagram",
    "linkedin",
    "twitter",
    "tiktok",
    "youtube",
    "pinterest",
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

  const alternativePages: MetadataRoute.Sitemap = ALL_COMPETITORS.map(({ slug }) => ({
    url: `${BASE}/alternatives/${slug}`,
    lastModified: now,
    changeFrequency: "monthly" as const,
    priority: 0.7,
  }));

  const moneyPagePriority: Record<string, number> = {
    "/social-media-api": 0.9,
    "/social-media-posting-api": 0.9,
    "/social-media-publishing-api": 0.85,
  };

  const moneyPages: MetadataRoute.Sitemap = MONEY_PAGES.map((page) => ({
    url: `${BASE}${page.path}`,
    lastModified: now,
    changeFrequency: "monthly" as const,
    priority: moneyPagePriority[page.path] ?? 0.8,
  }));

  const solutionPages: MetadataRoute.Sitemap = SOLUTION_PAGES.map((page) => ({
    url: `${BASE}${page.path}`,
    lastModified: now,
    changeFrequency: "monthly" as const,
    priority: 0.75,
  }));

  const resourcePages: MetadataRoute.Sitemap = SEO_RESOURCES.map((resource) => ({
    url: `${BASE}/resources/${resource.slug}`,
    lastModified: new Date(resource.lastVerified),
    changeFrequency: "monthly" as const,
    priority: 0.7,
  }));

  return [
    ...staticPages,
    ...moneyPages,
    ...solutionPages,
    ...resourcePages,
    ...platformPages,
    ...toolPages,
    ...alternativePages,
    ...blogPages,
  ];
}
