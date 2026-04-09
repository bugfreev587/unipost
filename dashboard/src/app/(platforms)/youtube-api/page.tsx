import type { Metadata } from "next";
import PlatformPage from "../_components/platform-page";
import { youtube } from "../_config/platforms";

export const metadata: Metadata = {
  title: youtube.seo.title,
  description: youtube.seo.description,
  keywords: youtube.seo.keywords,
  openGraph: {
    title: `${youtube.name} API for Developers | UniPost`,
    description: youtube.seo.description,
    siteName: "UniPost",
    type: "website",
  },
};

export default function YouTubeApiPage() {
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify([
            {
              "@context": "https://schema.org",
              "@type": "FAQPage",
              mainEntity: youtube.faq.map((f) => ({
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
              description: youtube.seo.description,
            },
          ]),
        }}
      />
      <PlatformPage cfg={youtube} />
    </>
  );
}
