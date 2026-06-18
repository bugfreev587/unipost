"use client";

import { PlatformAnalyticsOverviewPage } from "../_components/platform-analytics-doc-pages";
import { platformAnalyticsDocs } from "../_data/platform-analytics-docs";

export default function ThreadsAnalyticsDocsPage() {
  return <PlatformAnalyticsOverviewPage platform={platformAnalyticsDocs.threads} />;
}
