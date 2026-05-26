import type { Metadata } from "next";
import { getAnalyticsTool, PublicAnalyticsToolPage } from "../_components/public-analytics-tool";

const tool = getAnalyticsTool("threads");

export const metadata: Metadata = {
  title: tool.seoTitle,
  description: tool.description,
  alternates: {
    canonical: `https://unipost.dev${tool.href}`,
  },
};

export default function ThreadsAnalyticsToolPage() {
  return <PublicAnalyticsToolPage tool={tool} />;
}
