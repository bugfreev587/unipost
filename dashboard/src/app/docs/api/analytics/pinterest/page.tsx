"use client";

import { PlatformAnalyticsOverviewPage } from "../_components/platform-analytics-doc-pages";
import { platformAnalyticsDocs } from "../_data/platform-analytics-docs";

export default function PinterestAnalyticsDocsPage() {
  return <PlatformAnalyticsOverviewPage platform={platformAnalyticsDocs.pinterest} />;
}
