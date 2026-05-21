import { FacebookPageAnalyticsView } from "@/components/analytics/facebook-page-analytics-view";
import { FeatureFlagGate } from "@/components/feature-flag-gate";
import { FEATURE_FLAG_KEYS } from "@/lib/feature-flags";

export default async function FacebookPlatformAnalyticsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return (
    <FeatureFlagGate
      flag={FEATURE_FLAG_KEYS.facebookPageAnalytics}
      title="Facebook Page analytics is disabled"
      description="Facebook Page analytics is only enabled in environments where the Facebook Page analytics flag is on."
    >
      <FacebookPageAnalyticsView profileId={id} />
    </FeatureFlagGate>
  );
}
