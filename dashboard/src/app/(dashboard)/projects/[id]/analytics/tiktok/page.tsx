import { redirect } from "next/navigation";

export default async function LegacyTikTokAnalyticsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  redirect(`/projects/${id}/analytics/platforms/tiktok`);
}
