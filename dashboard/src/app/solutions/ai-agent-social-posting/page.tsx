import type { Metadata } from "next";
import { SeoGrowthPage } from "@/components/marketing/seo-growth-page";
import { getSolutionPage } from "@/data/seo-growth-pages";

const page = getSolutionPage("ai-agent-social-posting")!;

export const metadata: Metadata = {
  title: "AI Agent Social Posting API | UniPost",
  description:
    "Let AI agents publish safely through a social posting API with connected account controls, MCP support, media handling, webhooks, and audit-friendly status.",
  alternates: {
    canonical: "https://unipost.dev/solutions/ai-agent-social-posting",
  },
  openGraph: {
    title: "AI Agent Social Posting API",
    description: "Give agents a narrow, controllable publishing tool instead of raw social credentials.",
    url: "https://unipost.dev/solutions/ai-agent-social-posting",
    siteName: "UniPost",
    type: "website",
  },
};

export default function AiAgentSocialPostingPage() {
  const schema = [
    { "@context": "https://schema.org", "@type": "SoftwareApplication", name: "UniPost", applicationCategory: "DeveloperApplication", url: "https://unipost.dev/solutions/ai-agent-social-posting" },
    { "@context": "https://schema.org", "@type": "FAQPage", mainEntity: page.faqs.map((faq) => ({ "@type": "Question", name: faq.q, acceptedAnswer: { "@type": "Answer", text: faq.a } })) },
    { "@context": "https://schema.org", "@type": "BreadcrumbList", itemListElement: [{ "@type": "ListItem", position: 1, name: "UniPost", item: "https://unipost.dev" }, { "@type": "ListItem", position: 2, name: "Solutions", item: "https://unipost.dev/solutions" }, { "@type": "ListItem", position: 3, name: "AI Agent Social Posting", item: "https://unipost.dev/solutions/ai-agent-social-posting" }] },
  ];

  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(schema) }} />
      <SeoGrowthPage page={page} active="solutions" />
    </>
  );
}
