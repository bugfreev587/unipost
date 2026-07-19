import PlatformPage from "../_components/platform-page";
import { buildPlatformMetadata } from "../_config/metadata";
import { bluesky } from "../_config/platforms";

export const metadata = buildPlatformMetadata(bluesky);

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
