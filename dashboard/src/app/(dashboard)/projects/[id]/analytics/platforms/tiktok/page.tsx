import { FeatureFlagGate } from "@/components/feature-flag-gate";
import { TikTokAnalyticsView } from "@/components/analytics/tiktok-analytics-view";
import { FEATURE_FLAG_KEYS } from "@/lib/feature-flags";

export default async function TikTokPlatformAnalyticsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <FeatureFlagGate
      flag={FEATURE_FLAG_KEYS.tiktokAnalyticsScopes}
      title="TikTok analytics is disabled"
      description="TikTok platform analytics is only enabled in environments where the analytics scopes flag is on."
    >
      <TikTokAnalyticsView profileId={id} />
    </FeatureFlagGate>
  );
}
