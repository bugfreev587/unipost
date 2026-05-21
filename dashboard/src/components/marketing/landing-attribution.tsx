"use client";

import { useEffect } from "react";
import { recordLandingVisit } from "@/lib/api";
import {
  firstQueryValue,
  getOrCreateLandingSessionId,
  getStoredLandingSource,
  normalizeLandingSource,
  storeLandingSource,
} from "@/lib/landing-attribution";

function landingCountryCode() {
  const match = document.cookie
    .split("; ")
    .find((part) => part.startsWith("unipost_country="));
  let code = "";
  if (match) {
    try {
      code = decodeURIComponent(match.split("=").slice(1).join("=")).trim().toUpperCase();
    } catch {
      code = "";
    }
  }
  return /^[A-Z]{2}$/.test(code) && code !== "XX" && code !== "T1" ? code : undefined;
}

export function LandingAttribution() {
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const rawSource = firstQueryValue(params, "r", "utm_source", "s");
    const sourceFromUrl = normalizeLandingSource(rawSource);
    const source = sourceFromUrl ?? getStoredLandingSource();
    const sessionId = getOrCreateLandingSessionId();
    const attribution = {
      r: params.get("r") || undefined,
      utm_source: firstQueryValue(params, "utm_source", "s"),
      utm_medium: firstQueryValue(params, "utm_medium", "m"),
      utm_campaign: firstQueryValue(params, "utm_campaign", "c"),
    };

    if (sourceFromUrl) {
      storeLandingSource(sourceFromUrl);
    }

    void recordLandingVisit({
      path: window.location.pathname || "/",
      source,
      session_id: sessionId,
      referrer: document.referrer || undefined,
      country_code: landingCountryCode(),
      raw_query: window.location.search ? window.location.search.slice(1) : undefined,
      attribution,
    }).catch(() => {
      // Keep attribution non-blocking for the landing page.
    });
  }, []);

  return null;
}
