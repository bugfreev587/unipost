import type { Metadata } from "next";
import { getAnalyticsTool, PublicAnalyticsToolPage } from "../_components/public-analytics-tool";

const tool = getAnalyticsTool("pinterest");

export const metadata: Metadata = {
  title: tool.seoTitle,
  description: tool.description,
  alternates: {
    canonical: `https://unipost.dev${tool.href}`,
  },
};

export default function PinterestAnalyticsToolPage() {
  return <PublicAnalyticsToolPage tool={tool} />;
}
