import type { Metadata } from "next";
import { SeoGrowthPage } from "@/components/marketing/seo-growth-page";
import { getMoneyPage } from "@/data/seo-growth-pages";

const page = getMoneyPage("social-media-publishing-api")!;

export const metadata: Metadata = {
  title: "Social Media Publishing API for SaaS Products | UniPost",
  description:
    "A social media publishing API for SaaS teams that need account connection, media workflows, scheduling, approvals, status tracking, and webhooks.",
  alternates: {
    canonical: "https://unipost.dev/social-media-publishing-api",
  },
  openGraph: {
    title: "Social Media Publishing API for SaaS Products",
    description:
      "Embed account connection, media workflows, scheduling, approvals, and delivery status into your SaaS.",
    url: "https://unipost.dev/social-media-publishing-api",
    siteName: "UniPost",
    type: "website",
  },
};

export default function SocialMediaPublishingApiPage() {
  const schema = [
    {
      "@context": "https://schema.org",
      "@type": "SoftwareApplication",
      name: "UniPost",
      applicationCategory: "DeveloperApplication",
      operatingSystem: "Web",
      offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
      description: metadata.description,
      url: "https://unipost.dev/social-media-publishing-api",
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
        { "@type": "ListItem", position: 2, name: "Social Media Publishing API", item: "https://unipost.dev/social-media-publishing-api" },
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
