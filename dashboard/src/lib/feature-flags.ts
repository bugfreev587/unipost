export const FEATURE_FLAG_KEYS = {
  tiktokAnalyticsScopes: "tiktok.analytics_scopes",
  attributionUtmSignupBindingV1: "attribution.utm_signup_binding_v1",
  appReviewAutopilotV1: "app_review.autopilot_v1",
  postsCalendarViewV1: "posts.calendar_view_v1",
} as const;

export type FeatureFlagKey = typeof FEATURE_FLAG_KEYS[keyof typeof FEATURE_FLAG_KEYS];
