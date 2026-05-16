export const FEATURE_FLAG_KEYS = {
  tiktokAnalyticsScopes: "tiktok.analytics_scopes",
} as const;

export type FeatureFlagKey = typeof FEATURE_FLAG_KEYS[keyof typeof FEATURE_FLAG_KEYS];
