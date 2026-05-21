import { FacebookPageAnalyticsView } from "@/components/analytics/facebook-page-analytics-view";

export default async function FacebookPlatformAnalyticsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return <FacebookPageAnalyticsView profileId={id} />;
}
