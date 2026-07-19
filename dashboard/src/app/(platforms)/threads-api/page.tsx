import PlatformPage from "../_components/platform-page";
import { buildPlatformMetadata } from "../_config/metadata";
import { threads } from "../_config/platforms";

export const metadata = buildPlatformMetadata(threads);

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
