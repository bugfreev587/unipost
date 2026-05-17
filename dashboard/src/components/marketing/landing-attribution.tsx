"use client";

import { useEffect } from "react";
import { recordLandingVisit } from "@/lib/api";
import {
  getOrCreateLandingSessionId,
  getStoredLandingSource,
  normalizeLandingSource,
  storeLandingSource,
} from "@/lib/landing-attribution";

export function LandingAttribution() {
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const sourceFromUrl = normalizeLandingSource(params.get("r"));
    const source = sourceFromUrl ?? getStoredLandingSource();
    const sessionId = getOrCreateLandingSessionId();
    const attribution = {
      r: params.get("r") || undefined,
      utm_source: params.get("utm_source") || undefined,
      utm_medium: params.get("utm_medium") || undefined,
      utm_campaign: params.get("utm_campaign") || undefined,
    };

    if (sourceFromUrl) {
      storeLandingSource(sourceFromUrl);
    }

    void recordLandingVisit({
      path: window.location.pathname || "/",
      source,
      session_id: sessionId,
      referrer: document.referrer || undefined,
      raw_query: window.location.search ? window.location.search.slice(1) : undefined,
      attribution,
    }).catch(() => {
      // Keep attribution non-blocking for the landing page.
    });
  }, []);

  return null;
}
