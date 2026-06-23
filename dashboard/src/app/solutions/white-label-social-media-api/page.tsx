import type { Metadata } from "next";
import { SeoGrowthPage } from "@/components/marketing/seo-growth-page";
import { getSolutionPage } from "@/data/seo-growth-pages";

const page = getSolutionPage("white-label-social-media-api")!;

export const metadata: Metadata = {
  title: "White Label Social Media API | UniPost",
  description:
    "Use a white-label social media API path with customer-owned credentials, hosted account connection, media upload, publishing, webhooks, and status tracking.",
  alternates: {
    canonical: "https://unipost.dev/solutions/white-label-social-media-api",
  },
  openGraph: {
    title: "White Label Social Media API",
    description: "Use native mode and platform credentials for branded account connection workflows.",
    url: "https://unipost.dev/solutions/white-label-social-media-api",
    siteName: "UniPost",
    type: "website",
  },
};

export default function WhiteLabelSocialMediaApiPage() {
  const schema = [
    { "@context": "https://schema.org", "@type": "SoftwareApplication", name: "UniPost", applicationCategory: "DeveloperApplication", url: "https://unipost.dev/solutions/white-label-social-media-api" },
    { "@context": "https://schema.org", "@type": "FAQPage", mainEntity: page.faqs.map((faq) => ({ "@type": "Question", name: faq.q, acceptedAnswer: { "@type": "Answer", text: faq.a } })) },
    { "@context": "https://schema.org", "@type": "BreadcrumbList", itemListElement: [{ "@type": "ListItem", position: 1, name: "UniPost", item: "https://unipost.dev" }, { "@type": "ListItem", position: 2, name: "Solutions", item: "https://unipost.dev/solutions" }, { "@type": "ListItem", position: 3, name: "White Label Social Media API", item: "https://unipost.dev/solutions/white-label-social-media-api" }] },
  ];

  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(schema) }} />
      <SeoGrowthPage page={page} active="solutions" />
    </>
  );
}
