import { PinterestAnalyticsView } from "@/components/analytics/pinterest-analytics-view";

export default async function PinterestPlatformAnalyticsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <PinterestAnalyticsView profileId={id} />;
}
