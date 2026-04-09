import type { Metadata } from "next";
import PlatformPage from "../_components/platform-page";
import { threads } from "../_config/platforms";

export const metadata: Metadata = {
  title: threads.seo.title,
  description: threads.seo.description,
  keywords: threads.seo.keywords,
  openGraph: {
    title: `${threads.name} API for Developers | UniPost`,
    description: threads.seo.description,
    siteName: "UniPost",
    type: "website",
  },
};

export default function ThreadsApiPage() {
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify([
            {
              "@context": "https://schema.org",
              "@type": "FAQPage",
              mainEntity: threads.faq.map((f) => ({
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
              description: threads.seo.description,
            },
          ]),
        }}
      />
      <PlatformPage cfg={threads} />
    </>
  );
}
