import type { Metadata } from "next";
import { SeoGrowthPage } from "@/components/marketing/seo-growth-page";
import { getMoneyPage } from "@/data/seo-growth-pages";

const page = getMoneyPage("social-media-api")!;

export const metadata: Metadata = {
  title: "Unified Social Media API for Developers | UniPost",
  description:
    "Use UniPost as a unified social media API for account connection, media upload, multi-platform publishing, webhooks, and status tracking across nine networks.",
  alternates: {
    canonical: "https://unipost.dev/social-media-api",
  },
  openGraph: {
    title: "Unified Social Media API for Developers",
    description:
      "One API for social account connection, media upload, publishing, webhooks, and status tracking.",
    url: "https://unipost.dev/social-media-api",
    siteName: "UniPost",
    type: "website",
  },
};

export default function SocialMediaApiPage() {
  const schema = [
    {
      "@context": "https://schema.org",
      "@type": "SoftwareApplication",
      name: "UniPost",
      applicationCategory: "DeveloperApplication",
      operatingSystem: "Web",
      offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
      description: metadata.description,
      url: "https://unipost.dev/social-media-api",
    },
    {
      "@context": "https://schema.org",
      "@type": "FAQPage",
      mainEntity: page.faqs.map((faq) => ({
        "@type": "Question",
        name: faq.q,
        acceptedAnswer: { "@type": "Answer", text: faq.a },
      })),
    },
    {
      "@context": "https://schema.org",
      "@type": "BreadcrumbList",
      itemListElement: [
        { "@type": "ListItem", position: 1, name: "UniPost", item: "https://unipost.dev" },
        { "@type": "ListItem", position: 2, name: "Unified Social Media API", item: "https://unipost.dev/social-media-api" },
      ],
    },
  ];

  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(schema) }} />
      <SeoGrowthPage page={page} />
    </>
  );
}
