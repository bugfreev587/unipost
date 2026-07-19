import type { Metadata } from "next";
import type { PlatformConfig } from "./platforms";

export function buildPlatformMetadata(platform: PlatformConfig): Metadata {
  const canonical = `https://unipost.dev/${platform.slug}-api`;

  return {
    title: platform.seo.title,
    description: platform.seo.description,
    keywords: platform.seo.keywords,
    alternates: { canonical },
    openGraph: {
      title: `${platform.name} API for Developers | UniPost`,
      description: platform.seo.description,
      siteName: "UniPost",
      type: "website",
    },
  };
}
