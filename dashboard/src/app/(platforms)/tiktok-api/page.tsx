import type { Metadata } from "next";
import PlatformPage from "../_components/platform-page";
import { tiktok } from "../_config/platforms";

export const metadata: Metadata = {
  title: tiktok.seo.title,
  description: tiktok.seo.description,
  keywords: tiktok.seo.keywords,
  openGraph: {
    title: `${tiktok.name} API for Developers | UniPost`,
    description: tiktok.seo.description,
    siteName: "UniPost",
    type: "website",
  },
};

export default function TikTokApiPage() {
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify([
            {
              "@context": "https://schema.org",
              "@type": "FAQPage",
              mainEntity: tiktok.faq.map((f) => ({
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
              description: tiktok.seo.description,
            },
          ]),
        }}
      />
      <PlatformPage cfg={tiktok} />
    </>
  );
}
