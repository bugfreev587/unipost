"use client";

import { PlatformAnalyticsEndpointPage } from "../../_components/platform-analytics-doc-pages";
import { platformAnalyticsDocs } from "../../_data/platform-analytics-docs";

export default function PinterestPostAnalyticsPage() {
  return <PlatformAnalyticsEndpointPage platform={platformAnalyticsDocs.pinterest} endpointId="postAnalytics" />;
}
