"use client";

export const LANDING_SESSION_KEY = "unipost-landing-session-id";
export const LANDING_SESSION_QUERY_KEY = "lsid";
export const LANDING_SOURCE_KEY = "unipost-landing-source";
export const LANDING_SOURCE_AT_KEY = "unipost-landing-source-at";

const LANDING_SOURCE_TTL_MS = 30 * 24 * 60 * 60 * 1000;
const LANDING_SESSION_COOKIE = "unipost-landing-session-id";
const LANDING_SESSION_COOKIE_MAX_AGE = 90 * 24 * 60 * 60;
const KNOWN_LANDING_SOURCES = new Set(["x", "rd", "ih", "ph", "google", "meta", "microsoft", "o", "direct"]);

export function generateLandingSessionId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }

  return `lp_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

export function normalizeLandingSource(raw: string | null | undefined) {
  const source = raw?.trim().toLowerCase();
  return source && KNOWN_LANDING_SOURCES.has(source) ? source : undefined;
}

function landingCookieDomain() {
  if (typeof window === "undefined") return "";
  const host = window.location.hostname;
  if (host === "unipost.dev" || host.endsWith(".unipost.dev")) {
    return "; domain=.unipost.dev";
  }
  return "";
}

export function writeLandingSessionCookie(sessionId: string) {
  if (typeof document === "undefined" || !sessionId) return;
  document.cookie = `${LANDING_SESSION_COOKIE}=${encodeURIComponent(sessionId)}; path=/; max-age=${LANDING_SESSION_COOKIE_MAX_AGE}; samesite=lax${landingCookieDomain()}`;
}

function readLandingSessionCookie() {
  if (typeof document === "undefined") return undefined;
  const match = document.cookie.match(new RegExp(`(?:^|; )${LANDING_SESSION_COOKIE}=([^;]*)`));
  return match ? decodeURIComponent(match[1]) : undefined;
}

export function getStoredLandingSource() {
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

export function storeLandingSource(source: string) {
  try {
    window.localStorage.setItem(LANDING_SOURCE_KEY, source);
    window.localStorage.setItem(LANDING_SOURCE_AT_KEY, String(Date.now()));
  } catch {
    // Attribution must stay non-blocking.
  }
}

export function getExistingLandingSessionId() {
  try {
    const fromQuery = new URLSearchParams(window.location.search).get(LANDING_SESSION_QUERY_KEY);
    const fromLocalStorage = window.localStorage.getItem(LANDING_SESSION_KEY);
    const sessionId = fromQuery || fromLocalStorage || readLandingSessionCookie();
    if (!sessionId) return undefined;

    window.localStorage.setItem(LANDING_SESSION_KEY, sessionId);
    writeLandingSessionCookie(sessionId);
    return sessionId;
  } catch {
    return readLandingSessionCookie();
  }
}

export function getOrCreateLandingSessionId() {
  const existing = getExistingLandingSessionId();
  if (existing) return existing;

  const next = generateLandingSessionId();
  try {
    window.localStorage.setItem(LANDING_SESSION_KEY, next);
  } catch {
    // Cookie still gives subdomain handoff a chance.
  }
  writeLandingSessionCookie(next);
  return next;
}

export function appendLandingSessionId(rawUrl: string) {
  if (typeof window === "undefined") return rawUrl;
  try {
    const sessionId = getOrCreateLandingSessionId();
    const url = new URL(rawUrl, window.location.origin);
    url.searchParams.set(LANDING_SESSION_QUERY_KEY, sessionId);
    return url.toString();
  } catch {
    return rawUrl;
  }
}
