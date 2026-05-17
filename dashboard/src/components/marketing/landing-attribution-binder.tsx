"use client";

import { useEffect } from "react";
import { useAuth } from "@clerk/nextjs";
import { bindLandingAttributionSession } from "@/lib/api";
import { getExistingLandingSessionId } from "@/lib/landing-attribution";

const BOUND_SESSION_KEY = "unipost-landing-session-bound";

export function LandingAttributionBinder() {
  const { getToken, isLoaded, isSignedIn } = useAuth();

  useEffect(() => {
    if (!isLoaded || !isSignedIn) return;

    const sessionId = getExistingLandingSessionId();
    if (!sessionId) return;

    try {
      if (window.localStorage.getItem(BOUND_SESSION_KEY) === sessionId) return;
    } catch {
      // Binding can still run; localStorage is just a client-side dedupe.
    }

    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token || cancelled) return;
        await bindLandingAttributionSession(token, sessionId);
        if (cancelled) return;
        try {
          window.localStorage.setItem(BOUND_SESSION_KEY, sessionId);
        } catch {
          // Best-effort dedupe only.
        }
      } catch {
        // Attribution must never block product usage.
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [getToken, isLoaded, isSignedIn]);

  return null;
}
