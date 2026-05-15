import { TikTokAnalyticsView } from "@/components/analytics/tiktok-analytics-view";

export default async function TikTokPlatformAnalyticsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <TikTokAnalyticsView profileId={id} />;
}
