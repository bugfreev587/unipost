import type { Metadata } from "next";
import SolutionsPageClient from "./solutions-page-client";

const description =
  "Social Media API Solutions for SaaS products, AI agents, ecommerce platforms, schedulers, agencies, and multi-account publishing workflows.";

export const metadata: Metadata = {
  title: "Social Media API Solutions | UniPost",
  description,
  alternates: {
    canonical: "https://unipost.dev/solutions",
  },
  openGraph: {
    title: "Social Media API Solutions",
    description,
    url: "https://unipost.dev/solutions",
    siteName: "UniPost",
    type: "website",
  },
};

export default function SolutionsPage() {
  return <SolutionsPageClient />;
}
