"use client";

// featureInDev is the single source of truth for dashboard-visible
// features that are still under development. Anything listed here must
// stay hidden from non-super-admin users until launch readiness, at
// which point we remove the feature name from this array.
export const FEATURES_IN_DEV = [
  "facebook_pages",
  "ai_assist_create_post_drawer",
] as const;

export type FeatureInDev = (typeof FEATURES_IN_DEV)[number];

const FEATURES_IN_DEV_SET = new Set<string>(FEATURES_IN_DEV);

export function isFeatureInDev(feature: string): feature is FeatureInDev {
  return FEATURES_IN_DEV_SET.has(feature);
}

// Development-only features are visible to internal testers on the
// SUPER_ADMINS allowlist. Everyone else should behave as if the
// feature doesn't exist yet.
export function isFeatureInDevEnabledForMe(
  feature: FeatureInDev,
  isSuperAdmin: boolean | undefined,
): boolean {
  return isFeatureInDev(feature) && !!isSuperAdmin;
}
