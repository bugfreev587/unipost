"use client";

import { useEffect, useState } from "react";
import { getPublicFeatureFlags } from "@/lib/api";
import {
  CLOSED_PUBLIC_DOCS_FLAGS,
  normalizePublicDocsFeatureFlags,
  type PublicDocsFeatureFlags,
} from "@/lib/docs-feature-flags";

export function usePublicDocsFeatureFlags() {
  const [flags, setFlags] = useState<PublicDocsFeatureFlags>(CLOSED_PUBLIC_DOCS_FLAGS);

  useEffect(() => {
    let cancelled = false;
    void getPublicFeatureFlags()
      .then((response) => {
        if (!cancelled) {
          setFlags(normalizePublicDocsFeatureFlags(response.data.flags));
        }
      })
      .catch(() => {
        if (!cancelled) {
          setFlags(CLOSED_PUBLIC_DOCS_FLAGS);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return flags;
}
