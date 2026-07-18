import type { UniPostFeatureFlagKey } from "@/lib/api";

export type PublicDocsFeatureFlags = Record<UniPostFeatureFlagKey, boolean>;

export const CLOSED_PUBLIC_DOCS_FLAGS: PublicDocsFeatureFlags = {
  x_dms_v1: false,
  x_credits_billing_v1: false,
};

const DOCS_PATH_FEATURES: Readonly<Record<string, UniPostFeatureFlagKey>> = {
  "/docs/guides/x/direct-messages": "x_dms_v1",
  "/docs/guides/x/credits": "x_credits_billing_v1",
  "/docs/api/x-credits": "x_credits_billing_v1",
};

export function normalizePublicDocsFeatureFlags(value: unknown): PublicDocsFeatureFlags {
  if (!value || typeof value !== "object") {
    return CLOSED_PUBLIC_DOCS_FLAGS;
  }

  const flags = value as Partial<Record<UniPostFeatureFlagKey, unknown>>;
  return {
    x_dms_v1: flags.x_dms_v1 === true,
    x_credits_billing_v1: flags.x_credits_billing_v1 === true,
  };
}

export function isDocsPathAvailable(path: string, flags: PublicDocsFeatureFlags) {
  const requiredFlag = DOCS_PATH_FEATURES[path];
  return requiredFlag ? flags[requiredFlag] : true;
}

export function filterDocsNavigation<T extends { href: string }>(
  items: readonly T[],
  flags: PublicDocsFeatureFlags,
) {
  return items.filter((item) => isDocsPathAvailable(item.href, flags));
}

export function filterDocsSearchIndex<T extends { href: string }>(
  items: readonly T[],
  flags: PublicDocsFeatureFlags,
) {
  return filterDocsNavigation(items, flags);
}

export function filterDocsSearchChunks<
  T extends { path: string; required_feature?: UniPostFeatureFlagKey },
>(
  items: readonly T[],
  flags: PublicDocsFeatureFlags,
) {
  return items.filter((item) => (
    isDocsPathAvailable(item.path, flags)
    && (!item.required_feature || flags[item.required_feature])
  ));
}
