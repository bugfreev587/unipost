"use client";

import { useEffect } from "react";
import { recordLandingVisit } from "@/lib/api";

const LANDING_SESSION_KEY = "unipost-landing-session-id";
const LANDING_SOURCE_KEY = "unipost-landing-source";
const LANDING_SOURCE_AT_KEY = "unipost-landing-source-at";
const LANDING_SOURCE_TTL_MS = 7 * 24 * 60 * 60 * 1000;
const KNOWN_LANDING_SOURCES = new Set(["x", "rd", "ih", "ph", "o", "direct"]);

function generateLandingSessionId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }

  return `lp_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

function normalizeLandingSource(raw: string | null) {
  const source = raw?.trim().toLowerCase();
  return source && KNOWN_LANDING_SOURCES.has(source) ? source : undefined;
}

function getStoredLandingSource() {
  try {
    const source = normalizeLandingSource(window.localStorage.getItem(LANDING_SOURCE_KEY));
    const rawAt = window.localStorage.getItem(LANDING_SOURCE_AT_KEY);
    const savedAt = rawAt ? Number(rawAt) : 0;

    if (!source || !savedAt || Date.now() - savedAt > LANDING_SOURCE_TTL_MS) {
      window.localStorage.removeItem(LANDING_SOURCE_KEY);
      window.localStorage.removeItem(LANDING_SOURCE_AT_KEY);
      return undefined;
    }

    return source;
  } catch {
    return undefined;
  }
}

function getLandingSessionId() {
  try {
    const existing = window.localStorage.getItem(LANDING_SESSION_KEY);
    if (existing) return existing;

    const next = generateLandingSessionId();
    window.localStorage.setItem(LANDING_SESSION_KEY, next);
    return next;
  } catch {
    return generateLandingSessionId();
  }
}

export function LandingAttribution() {
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const sourceFromUrl = normalizeLandingSource(params.get("r"));
    const source = sourceFromUrl ?? getStoredLandingSource();
    const sessionId = getLandingSessionId();

    if (sourceFromUrl) {
      try {
        window.localStorage.setItem(LANDING_SOURCE_KEY, sourceFromUrl);
        window.localStorage.setItem(LANDING_SOURCE_AT_KEY, String(Date.now()));
      } catch {
        // Keep attribution non-blocking for the landing page.
      }
    }

    void recordLandingVisit({
      path: window.location.pathname || "/",
      source,
      session_id: sessionId,
      referrer: document.referrer || undefined,
    }).catch(() => {
      // Keep attribution non-blocking for the landing page.
    });
  }, []);

  return null;
}
