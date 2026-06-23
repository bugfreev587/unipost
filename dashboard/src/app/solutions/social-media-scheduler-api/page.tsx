import type { Metadata } from "next";
import { SeoGrowthPage } from "@/components/marketing/seo-growth-page";
import { getSolutionPage } from "@/data/seo-growth-pages";

const page = getSolutionPage("social-media-scheduler-api")!;

export const metadata: Metadata = {
  title: "Social Media Scheduler API | UniPost",
  description:
    "Build a social media scheduler API with account connection, media upload, scheduled posts, delivery status, and webhook-based retry workflows.",
  alternates: {
    canonical: "https://unipost.dev/solutions/social-media-scheduler-api",
  },
  openGraph: {
    title: "Social Media Scheduler API",
    description: "Power calendars, queues, approvals, retries, and scheduled publishing with UniPost.",
    url: "https://unipost.dev/solutions/social-media-scheduler-api",
    siteName: "UniPost",
    type: "website",
  },
};

export default function SocialMediaSchedulerApiPage() {
  const schema = [
    { "@context": "https://schema.org", "@type": "SoftwareApplication", name: "UniPost", applicationCategory: "DeveloperApplication", url: "https://unipost.dev/solutions/social-media-scheduler-api" },
    { "@context": "https://schema.org", "@type": "FAQPage", mainEntity: page.faqs.map((faq) => ({ "@type": "Question", name: faq.q, acceptedAnswer: { "@type": "Answer", text: faq.a } })) },
    { "@context": "https://schema.org", "@type": "BreadcrumbList", itemListElement: [{ "@type": "ListItem", position: 1, name: "UniPost", item: "https://unipost.dev" }, { "@type": "ListItem", position: 2, name: "Solutions", item: "https://unipost.dev/solutions" }, { "@type": "ListItem", position: 3, name: "Social Media Scheduler API", item: "https://unipost.dev/solutions/social-media-scheduler-api" }] },
  ];

  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(schema) }} />
      <SeoGrowthPage page={page} active="solutions" />
    </>
  );
}
