import PlatformPage from "../_components/platform-page";
import { buildPlatformMetadata } from "../_config/metadata";
import { instagram } from "../_config/platforms";

export const metadata = buildPlatformMetadata(instagram);

export default function InstagramApiPage() {
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify([
            {
              "@context": "https://schema.org",
              "@type": "FAQPage",
              mainEntity: instagram.faq.map((f) => ({
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
              description: instagram.seo.description,
            },
          ]),
        }}
      />
      <PlatformPage cfg={instagram} />
    </>
  );
}
