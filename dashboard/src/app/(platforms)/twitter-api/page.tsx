import type { Metadata } from "next";
import PlatformPage from "../_components/platform-page";
import { twitter } from "../_config/platforms";

export const metadata: Metadata = {
  title: twitter.seo.title,
  description: twitter.seo.description,
  keywords: twitter.seo.keywords,
  openGraph: {
    title: `${twitter.name} API for Developers | UniPost`,
    description: twitter.seo.description,
    siteName: "UniPost",
    type: "website",
  },
};

export default function TwitterApiPage() {
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify([
            {
              "@context": "https://schema.org",
              "@type": "FAQPage",
              mainEntity: twitter.faq.map((f) => ({
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
              description: twitter.seo.description,
            },
          ]),
        }}
      />
      <PlatformPage cfg={twitter} />
    </>
  );
}
