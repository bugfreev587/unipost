import type { Metadata } from "next";
import PlatformPage from "../_components/platform-page";
import { linkedin } from "../_config/platforms";

export const metadata: Metadata = {
  title: linkedin.seo.title,
  description: linkedin.seo.description,
  keywords: linkedin.seo.keywords,
  openGraph: {
    title: `${linkedin.name} API for Developers | UniPost`,
    description: linkedin.seo.description,
    siteName: "UniPost",
    type: "website",
  },
};

export default function LinkedInApiPage() {
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify([
            {
              "@context": "https://schema.org",
              "@type": "FAQPage",
              mainEntity: linkedin.faq.map((f) => ({
                "@type": "Question",
                name: f.q,
                acceptedAnswer: { "@type": "Answer", text: f.a },
              })),
            },
            {
              "@context": "https://schema.org",
              "@type": "SoftwareApplication",
              name: "UniPost",
              applicationCategory: "DeveloperApplication",
              operatingSystem: "Web",
              offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
              description: linkedin.seo.description,
            },
          ]),
        }}
      />
      <PlatformPage cfg={linkedin} />
    </>
  );
}
