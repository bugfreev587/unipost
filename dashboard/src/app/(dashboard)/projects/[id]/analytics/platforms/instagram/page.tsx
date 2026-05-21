import { MetaPlatformAnalyticsView } from "@/components/analytics/meta-platform-analytics-view";

export default async function InstagramPlatformAnalyticsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <MetaPlatformAnalyticsView profileId={id} platform="instagram" />;
}
