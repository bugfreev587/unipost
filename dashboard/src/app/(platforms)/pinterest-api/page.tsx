import type { Metadata } from "next";
import PlatformPage from "../_components/platform-page";
import { pinterest } from "../_config/platforms";

export const metadata: Metadata = {
  title: pinterest.seo.title,
  description: pinterest.seo.description,
  keywords: pinterest.seo.keywords,
  openGraph: {
    title: `${pinterest.name} API for Developers | UniPost`,
    description: pinterest.seo.description,
    siteName: "UniPost",
    type: "website",
  },
};

export default function PinterestApiPage() {
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify([
            {
              "@context": "https://schema.org",
              "@type": "FAQPage",
              mainEntity: pinterest.faq.map((f) => ({
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
              description: pinterest.seo.description,
            },
          ]),
        }}
      />
      <PlatformPage cfg={pinterest} />
    </>
  );
}
