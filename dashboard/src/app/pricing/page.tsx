import type { Metadata } from "next";
import { getLocale, getTranslations } from "next-intl/server";
import PricingPageClient from "./pricing-page-client";

export async function generateMetadata(): Promise<Metadata> {
  const locale = await getLocale();
  const t = await getTranslations("pricing");
  const canonical =
    locale === "es" ? "https://unipost.dev/es/pricing" : "https://unipost.dev/pricing";

  return {
    title: t("metadata.title"),
    description: t("metadata.description"),
    alternates: {
      canonical,
      languages: {
        en: "https://unipost.dev/pricing",
        es: "https://unipost.dev/es/pricing",
        "x-default": "https://unipost.dev/pricing",
      },
    },
    openGraph: {
      title: t("metadata.openGraphTitle"),
      description: t("metadata.description"),
      url: canonical,
      siteName: "UniPost",
      type: "website",
    },
  };
}

export default function PricingPage() {
  return <PricingPageClient />;
}
