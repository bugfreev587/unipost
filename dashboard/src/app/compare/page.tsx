import type { Metadata } from "next";
import ComparePageClient from "./compare-page-client";

const description =
  "Compare Social Media APIs for pricing, supported platforms, webhooks, analytics, white-label publishing, MCP support, and developer experience.";

export const metadata: Metadata = {
  title: "Compare Social Media APIs | UniPost",
  description,
  alternates: {
    canonical: "https://unipost.dev/compare",
  },
  openGraph: {
    title: "Compare Social Media APIs",
    description,
    url: "https://unipost.dev/compare",
    siteName: "UniPost",
    type: "website",
  },
};

export default function ComparePage() {
  return <ComparePageClient />;
}
