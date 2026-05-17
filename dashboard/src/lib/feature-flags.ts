export const FEATURE_FLAG_KEYS = {
  tiktokAnalyticsScopes: "tiktok.analytics_scopes",
  attributionUtmSignupBindingV1: "attribution.utm_signup_binding_v1",
} as const;

export type FeatureFlagKey = typeof FEATURE_FLAG_KEYS[keyof typeof FEATURE_FLAG_KEYS];
