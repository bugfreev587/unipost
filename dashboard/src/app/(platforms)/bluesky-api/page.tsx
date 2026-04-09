import type { Metadata } from "next";
import PlatformPage from "../_components/platform-page";
import { bluesky } from "../_config/platforms";

export const metadata: Metadata = {
  title: bluesky.seo.title,
  description: bluesky.seo.description,
  keywords: bluesky.seo.keywords,
  openGraph: {
    title: `${bluesky.name} API for Developers | UniPost`,
    description: bluesky.seo.description,
    siteName: "UniPost",
    type: "website",
  },
};

export default function BlueskyApiPage() {
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify([
            {
              "@context": "https://schema.org",
              "@type": "FAQPage",
              mainEntity: bluesky.faq.map((f) => ({
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
              description: bluesky.seo.description,
            },
          ]),
        }}
      />
      <PlatformPage cfg={bluesky} />
    </>
  );
}
