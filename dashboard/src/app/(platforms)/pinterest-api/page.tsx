import PlatformPage from "../_components/platform-page";
import { buildPlatformMetadata } from "../_config/metadata";
import { pinterest } from "../_config/platforms";

export const metadata = buildPlatformMetadata(pinterest);

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
