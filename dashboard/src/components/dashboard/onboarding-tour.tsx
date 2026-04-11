"use client";

import { TourProvider, useTour } from "@reactour/tour";
import { useEffect } from "react";
import { Sparkles } from "lucide-react";

const TOUR_STORAGE_KEY = "unipost_tour_completed";
const TOUR_START_FLAG = "unipost_start_tour";
const FIRST_STEP_SELECTOR = '[data-tour="profiles"]';

const TOUR_STEPS = [
  {
    selector: '[data-tour="profiles"]',
    content: (
      <div>
        <div style={{ fontWeight: 700, fontSize: 15, marginBottom: 6, color: "#f4f4f5" }}>
          Profiles
        </div>
        <div style={{ fontSize: 13, lineHeight: 1.6, color: "#aaa" }}>
          Profiles organize your brand identities. Each profile has its own set of connected social accounts. Create separate profiles for different brands, products, or teams.
        </div>
      </div>
    ),
  },
  {
    selector: '[data-tour="connections"]',
    content: (
      <div>
        <div style={{ fontWeight: 700, fontSize: 15, marginBottom: 6, color: "#f4f4f5" }}>
          Connections
        </div>
        <div style={{ fontSize: 13, lineHeight: 1.6, color: "#aaa" }}>
          Connect your social media accounts here. UniPost supports Twitter/X, LinkedIn, Bluesky, Instagram, Threads, TikTok, and YouTube. Click <strong style={{ color: "#f4f4f5" }}>Accounts</strong> to get started.
        </div>
      </div>
    ),
  },
  {
    selector: '[data-tour="posts"]',
    content: (
      <div>
        <div style={{ fontWeight: 700, fontSize: 15, marginBottom: 6, color: "#f4f4f5" }}>
          Posts
        </div>
        <div style={{ fontSize: 13, lineHeight: 1.6, color: "#aaa" }}>
          Manage all your content from one place. View published, scheduled, and draft posts. Click <strong style={{ color: "#f4f4f5" }}>Create</strong> to compose a post for multiple platforms at once.
        </div>
      </div>
    ),
  },
  {
    selector: '[data-tour="api-keys"]',
    content: (
      <div>
        <div style={{ fontWeight: 700, fontSize: 15, marginBottom: 6, color: "#f4f4f5" }}>
          API Keys
        </div>
        <div style={{ fontSize: 13, lineHeight: 1.6, color: "#aaa" }}>
          Generate API keys to integrate UniPost into your own app. Use our SDKs (JavaScript, Python, Go) or call the REST API directly.
        </div>
      </div>
    ),
  },
  {
    selector: '[data-tour="analytics"]',
    content: (
      <div>
        <div style={{ fontWeight: 700, fontSize: 15, marginBottom: 6, color: "#f4f4f5" }}>
          Analytics
        </div>
        <div style={{ fontSize: 13, lineHeight: 1.6, color: "#aaa" }}>
          Track post performance across platforms and monitor your API usage, latency, and reliability — all in real time.
        </div>
      </div>
    ),
  },
  {
    selector: '[data-tour="workspace"]',
    content: (
      <div>
        <div style={{ fontWeight: 700, fontSize: 15, marginBottom: 6, color: "#f4f4f5" }}>
          Workspace
        </div>
        <div style={{ fontSize: 13, lineHeight: 1.6, color: "#aaa" }}>
          Your workspace is the top-level container for everything — API keys, billing, and posts. Click the gear icon to manage workspace settings.
        </div>
      </div>
    ),
  },
];

// Wrapper that auto-starts the tour for new users.
//
// Why this is more elaborate than `setTimeout(setIsOpen, 800)`:
// the dashboard root resolver (`/`) redirects through the (dashboard)
// layout to `/projects/[id]`, and the sidebar nav items — which the tour
// anchors to via `data-tour` attributes — only render once `profileId` is
// in the URL. So a plain mount-time timer can fire while the targets
// don't exist yet, leaving the tour silently broken.
//
// Instead we poll for the first selector to appear and only then open
// the tour. The setup page sets `unipost_start_tour` to force-trigger
// even if a stale `unipost_tour_completed` flag exists from a prior run.
function TourAutoStart() {
  const { setIsOpen } = useTour();

  useEffect(() => {
    const forced = sessionStorage.getItem(TOUR_START_FLAG) === "1";
    if (forced) {
      sessionStorage.removeItem(TOUR_START_FLAG);
      localStorage.removeItem(TOUR_STORAGE_KEY);
    } else if (localStorage.getItem(TOUR_STORAGE_KEY)) {
      return;
    }

    let cancelled = false;
    let attempts = 0;
    const MAX_ATTEMPTS = 60; // ~12s at 200ms cadence

    function tryOpen() {
      if (cancelled) return;
      if (document.querySelector(FIRST_STEP_SELECTOR)) {
        setIsOpen(true);
        return;
      }
      if (++attempts >= MAX_ATTEMPTS) return;
      window.setTimeout(tryOpen, 200);
    }

    const initial = window.setTimeout(tryOpen, 400);
    return () => {
      cancelled = true;
      window.clearTimeout(initial);
    };
  }, [setIsOpen]);

  return null;
}

// "Take a tour" button for the sidebar
export function TourTriggerButton() {
  const { setIsOpen } = useTour();

  return (
    <button
      onClick={() => setIsOpen(true)}
      style={{
        display: "flex",
        alignItems: "center",
        gap: 6,
        padding: "8px 10px",
        fontSize: 12,
        color: "var(--dmuted)",
        background: "transparent",
        border: "none",
        cursor: "pointer",
        fontFamily: "inherit",
        width: "100%",
        borderRadius: 6,
        transition: "all 0.1s",
      }}
      onMouseEnter={(e) => { e.currentTarget.style.color = "var(--dtext)"; e.currentTarget.style.background = "var(--surface2)"; }}
      onMouseLeave={(e) => { e.currentTarget.style.color = "var(--dmuted)"; e.currentTarget.style.background = "transparent"; }}
    >
      <Sparkles style={{ width: 14, height: 14 }} />
      Take a tour
    </button>
  );
}

// Provider wrapper
export function OnboardingTourProvider({ children }: { children: React.ReactNode }) {
  return (
    <TourProvider
      steps={TOUR_STEPS}
      onClickClose={({ setIsOpen }) => {
        setIsOpen(false);
        localStorage.setItem(TOUR_STORAGE_KEY, "true");
      }}
      afterOpen={() => {}}
      beforeClose={() => {
        localStorage.setItem(TOUR_STORAGE_KEY, "true");
      }}
      styles={{
        popover: (base) => ({
          ...base,
          background: "#1a1a1e",
          border: "1px solid #2e2e38",
          borderRadius: 12,
          padding: "20px 24px",
          boxShadow: "0 20px 60px rgba(0,0,0,0.5)",
          maxWidth: 360,
        }),
        maskArea: (base) => ({
          ...base,
          rx: 8,
        }),
        maskWrapper: (base) => ({
          ...base,
          color: "rgba(0,0,0,0.7)",
        }),
        badge: (base) => ({
          ...base,
          background: "#10b981",
          color: "#000",
          fontWeight: 700,
          fontSize: 11,
        }),
        controls: (base) => ({
          ...base,
          marginTop: 16,
        }),
        close: (base) => ({
          ...base,
          color: "#8a8a93",
          top: 12,
          right: 12,
        }),
        dot: (base, state) => ({
          ...base,
          background: (state as any)?.current ? "#10b981" : "#2e2e38",
          border: "none",
          width: 8,
          height: 8,
        }),
        button: (base) => ({
          ...base,
          background: "#10b981",
          color: "#000",
          fontWeight: 600,
          fontSize: 12,
          padding: "6px 16px",
          borderRadius: 6,
          border: "none",
        }),
      }}
      padding={{ mask: 6, popover: [8, 12] }}
      showBadge={true}
      showDots={true}
      showNavigation={true}
      showCloseButton={true}
    >
      <TourAutoStart />
      {children}
    </TourProvider>
  );
}
