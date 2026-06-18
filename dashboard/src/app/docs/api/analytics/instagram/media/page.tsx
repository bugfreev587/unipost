"use client";

import { PlatformAnalyticsEndpointPage } from "../../_components/platform-analytics-doc-pages";
import { platformAnalyticsDocs } from "../../_data/platform-analytics-docs";

export default function InstagramMediaAnalyticsPage() {
  return <PlatformAnalyticsEndpointPage platform={platformAnalyticsDocs.instagram} endpointId="media" />;
}
