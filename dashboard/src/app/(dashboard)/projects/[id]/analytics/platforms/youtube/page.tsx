import { YouTubeAnalyticsView } from "@/components/analytics/youtube-analytics-view";

export default async function YouTubePlatformAnalyticsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <YouTubeAnalyticsView profileId={id} />;
}
