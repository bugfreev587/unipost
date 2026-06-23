import type { Metadata } from "next";
import { SeoGrowthPage } from "@/components/marketing/seo-growth-page";
import { getSolutionPage } from "@/data/seo-growth-pages";

const page = getSolutionPage("saas-social-publishing")!;

export const metadata: Metadata = {
  title: "Social Publishing API for SaaS | UniPost",
  description:
    "Embed social publishing in your SaaS product with hosted account connection, media workflows, scheduling, status tracking, and webhooks.",
  alternates: {
    canonical: "https://unipost.dev/solutions/saas-social-publishing",
  },
  openGraph: {
    title: "Social Publishing API for SaaS",
    description: "Let customers connect social accounts and publish from your app.",
    url: "https://unipost.dev/solutions/saas-social-publishing",
    siteName: "UniPost",
    type: "website",
  },
};

export default function SaasSocialPublishingPage() {
  const schema = [
    { "@context": "https://schema.org", "@type": "SoftwareApplication", name: "UniPost", applicationCategory: "DeveloperApplication", url: "https://unipost.dev/solutions/saas-social-publishing" },
    { "@context": "https://schema.org", "@type": "FAQPage", mainEntity: page.faqs.map((faq) => ({ "@type": "Question", name: faq.q, acceptedAnswer: { "@type": "Answer", text: faq.a } })) },
    { "@context": "https://schema.org", "@type": "BreadcrumbList", itemListElement: [{ "@type": "ListItem", position: 1, name: "UniPost", item: "https://unipost.dev" }, { "@type": "ListItem", position: 2, name: "Solutions", item: "https://unipost.dev/solutions" }, { "@type": "ListItem", position: 3, name: "SaaS Social Publishing", item: "https://unipost.dev/solutions/saas-social-publishing" }] },
  ];

  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(schema) }} />
      <SeoGrowthPage page={page} active="solutions" />
    </>
  );
}
