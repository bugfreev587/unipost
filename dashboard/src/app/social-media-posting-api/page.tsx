import type { Metadata } from "next";
import { SeoGrowthPage } from "@/components/marketing/seo-growth-page";
import { getMoneyPage } from "@/data/seo-growth-pages";

const page = getMoneyPage("social-media-posting-api")!;

export const metadata: Metadata = {
  title: "Social Media Posting API for Developers | UniPost",
  description:
    "Add a social media posting API to your app with one POST /v1/posts call for text, media, scheduling, post status, webhooks, and platform-specific options.",
  alternates: {
    canonical: "https://unipost.dev/social-media-posting-api",
  },
  openGraph: {
    title: "Social Media Posting API for Developers",
    description:
      "Create, schedule, and track social posts through one developer API.",
    url: "https://unipost.dev/social-media-posting-api",
    siteName: "UniPost",
    type: "website",
  },
};

export default function SocialMediaPostingApiPage() {
  const schema = [
    {
      "@context": "https://schema.org",
      "@type": "SoftwareApplication",
      name: "UniPost",
      applicationCategory: "DeveloperApplication",
      operatingSystem: "Web",
      offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
      description: metadata.description,
      url: "https://unipost.dev/social-media-posting-api",
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
        { "@type": "ListItem", position: 2, name: "Social Media Posting API", item: "https://unipost.dev/social-media-posting-api" },
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
