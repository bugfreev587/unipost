import "server-only";

import { notFound } from "next/navigation";
import type { UniPostFeatureFlagKey } from "@/lib/api";
import {
  CLOSED_PUBLIC_DOCS_FLAGS,
  normalizePublicDocsFeatureFlags,
  type PublicDocsFeatureFlags,
} from "@/lib/docs-feature-flags";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export async function getPublicDocsFeatureFlags(): Promise<PublicDocsFeatureFlags> {
  try {
    const response = await fetch(`${API_URL}/v1/public/features`, {
      cache: "no-store",
      headers: { Accept: "application/json" },
    });
    if (!response.ok) {
      return CLOSED_PUBLIC_DOCS_FLAGS;
    }

    const payload = await response.json() as { data?: { flags?: unknown } };
    return normalizePublicDocsFeatureFlags(payload.data?.flags);
  } catch {
    return CLOSED_PUBLIC_DOCS_FLAGS;
  }
}

export async function requirePublicDocsFeature(key: UniPostFeatureFlagKey) {
  const flags = await getPublicDocsFeatureFlags();
  if (!flags[key]) {
    notFound();
  }
  return flags;
}
