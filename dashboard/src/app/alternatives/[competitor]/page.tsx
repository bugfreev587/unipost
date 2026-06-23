import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ALL_COMPETITORS, getCompetitorBySlug } from "@/data/competitors";
import type { Competitor } from "@/data/competitors";
import AlternativePageClient from "./alternative-page-client";

type AlternativePageProps = {
  params: Promise<{ competitor: string }>;
};

export function generateStaticParams() {
  return ALL_COMPETITORS.map((competitor) => ({ competitor: competitor.slug }));
}

export async function generateMetadata({ params }: AlternativePageProps): Promise<Metadata> {
  const { competitor } = await params;
  const comp = getCompetitorBySlug(competitor);

  if (!comp) {
    return {};
  }

  const canonical = `https://unipost.dev/alternatives/${comp.slug}`;

  return {
    title: comp.seo.title,
    description: comp.seo.description,
    keywords: comp.seo.keywords,
    alternates: {
      canonical: canonical,
    },
    openGraph: {
      title: comp.seo.ogTitle,
      description: comp.seo.ogDescription,
      url: canonical,
      siteName: "UniPost",
      type: "website",
    },
  };
}

function buildAlternativeSchema(comp: Competitor) {
  const canonical = `https://unipost.dev/alternatives/${comp.slug}`;

  return {
    "@context": "https://schema.org",
    "@type": ["WebPage", "FAQPage"],
    name: `UniPost vs ${comp.name}`,
    description: comp.seo.description,
    url: canonical,
    breadcrumb: {
      "@type": "BreadcrumbList",
      itemListElement: [
        {
          "@type": "ListItem",
          position: 1,
          name: "UniPost",
          item: "https://unipost.dev",
        },
        {
          "@type": "ListItem",
          position: 2,
          name: "Compare Social Media APIs",
          item: "https://unipost.dev/compare",
        },
        {
          "@type": "ListItem",
          position: 3,
          name: `${comp.name} Alternative`,
          item: canonical,
        },
      ],
    },
    mainEntity: comp.faqs.map((faq) => ({
      "@type": "Question",
      name: faq.q,
      acceptedAnswer: { "@type": "Answer", text: faq.a },
    })),
  };
}

export default async function AlternativePage({ params }: AlternativePageProps) {
  const { competitor } = await params;
  const comp = getCompetitorBySlug(competitor);

  if (!comp) {
    notFound();
  }

  const schema = buildAlternativeSchema(comp);

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(schema) }}
      />
      <AlternativePageClient competitor={comp} />
    </>
  );
}
