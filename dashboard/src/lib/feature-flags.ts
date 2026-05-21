export const FEATURE_FLAG_KEYS = {
  tiktokAnalyticsScopes: "tiktok.analytics_scopes",
  facebookPageAnalytics: "facebook.page_analytics",
  attributionUtmSignupBindingV1: "attribution.utm_signup_binding_v1",
  inbox: "inbox",
} as const;

export type FeatureFlagKey = typeof FEATURE_FLAG_KEYS[keyof typeof FEATURE_FLAG_KEYS];
