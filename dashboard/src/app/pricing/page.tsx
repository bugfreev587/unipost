import type { Metadata } from "next";
import PricingPageClient from "./pricing-page-client";

const description =
  "Start free with 100 posts/month, then scale UniPost's unified social media API, dashboard, analytics, webhooks, and inbox with product-stage pricing.";

export const metadata: Metadata = {
  title: "UniPost Pricing | Social Media API Plans",
  description,
  alternates: {
    canonical: "https://unipost.dev/pricing",
  },
  openGraph: {
    title: "UniPost Pricing",
    description,
    url: "https://unipost.dev/pricing",
    siteName: "UniPost",
    type: "website",
  },
};

export default function PricingPage() {
  return <PricingPageClient />;
}
