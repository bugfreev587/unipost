"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { completeOnboarding } from "@/lib/api";

// "later" is a sentinel: it's mutually exclusive with the real modes, and on
// submit it gets stripped so the workspace is saved with `usage_modes: []`
// (which the dashboard treats as "show every feature").
const LATER_ID = "later";

const USE_CASES = [
  {
    id: "personal",
    title: "Post to my own accounts",
    description: "Connect your social accounts and publish to all of them in one click. Perfect for creators, indie hackers, and small teams.",
    icon: (
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="M22 2L11 13" /><path d="M22 2l-7 20-4-9-9-4 20-7z" />
      </svg>
    ),
  },
  {
    id: "whitelabel",
    title: "Post with my own app credentials",
    description: "Use your own OAuth apps for each platform. Your brand shows up during authorization instead of UniPost.",
    icon: (
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <rect width="18" height="18" x="3" y="3" rx="2" /><path d="M3 9h18" /><path d="M9 21V9" />
      </svg>
    ),
  },
  {
    id: "api",
    title: "Build an app on UniPost API",
    description: "Integrate UniPost into your product. Your customers connect their accounts through a hosted OAuth flow, and you post on their behalf via API.",
    icon: (
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <polyline points="16 18 22 12 16 6" /><polyline points="8 6 2 12 8 18" />
      </svg>
    ),
  },
  {
    id: LATER_ID,
    title: "I'll decide later",
    description: "Not sure yet? Show me everything for now. I can narrow this down anytime in Workspace Settings.",
    icon: (
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="10" /><polyline points="12 6 12 12 16 14" />
      </svg>
    ),
  },
];

export default function SetupPage() {
  const router = useRouter();
  const { getToken } = useAuth();
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [submitting, setSubmitting] = useState(false);

  function toggle(id: string) {
    setSelected((prev) => {
      // "later" is mutually exclusive with the real modes — picking it
      // clears everything else, and picking anything else clears "later".
      if (id === LATER_ID) {
        return prev.has(LATER_ID) ? new Set() : new Set([LATER_ID]);
      }
      const next = new Set(prev);
      next.delete(LATER_ID);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function handleContinue() {
    if (selected.size === 0) return;
    setSubmitting(true);
    try {
      const token = await getToken();
      if (!token) return;
      const stored = sessionStorage.getItem("onboarding_data");
      const data = stored ? JSON.parse(stored) : { first_name: "User" };
      // Strip the sentinel before sending — backend stores [] which the
      // dashboard reads as "show all features".
      const modes = [...selected].filter((m) => m !== LATER_ID);
      await completeOnboarding(token, {
        first_name: data.first_name,
        org_name: data.org_name,
        usage_modes: modes,
      });
      sessionStorage.removeItem("onboarding_data");
      // Signal the dashboard shell to auto-start the product tour once the
      // sidebar nav has rendered. The shell only mounts those `data-tour`
      // anchors after the `/` resolver redirects to `/projects/[id]`, so a
      // plain mount-time timer can fire before the targets exist.
      sessionStorage.setItem("unipost_start_tour", "1");
      localStorage.removeItem("unipost_tour_completed");
      router.push("/");
    } catch (err) {
      console.error("Onboarding failed:", err);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div style={{ width: "100%", maxWidth: 560, padding: "40px 24px" }}>
      <div style={{ textAlign: "center", marginBottom: 40 }}>
        <h1 style={{ fontSize: 28, fontWeight: 700, color: "#ededed", letterSpacing: -0.5, marginBottom: 8 }}>
          How will you use UniPost?
        </h1>
        <p style={{ fontSize: 14, color: "#888", lineHeight: 1.6 }}>
          Select all that apply. This helps us show you the right tools.<br />
          You can change this anytime in Workspace Settings.
        </p>
      </div>

      <div style={{ display: "flex", flexDirection: "column", gap: 12, marginBottom: 32 }}>
        {USE_CASES.map((uc) => {
          const active = selected.has(uc.id);
          return (
            <button
              key={uc.id}
              type="button"
              onClick={() => toggle(uc.id)}
              style={{
                display: "flex", alignItems: "flex-start", gap: 16,
                padding: "18px 20px", borderRadius: 12, textAlign: "left",
                background: active ? "#10b98110" : "#111113",
                border: `1.5px solid ${active ? "#10b981" : "#22222a"}`,
                cursor: "pointer", transition: "all 0.14s",
                outline: "none",
              }}
            >
              {/* Checkbox */}
              <div style={{
                width: 22, height: 22, borderRadius: 6, flexShrink: 0, marginTop: 2,
                border: `2px solid ${active ? "#10b981" : "#33333a"}`,
                background: active ? "#10b981" : "transparent",
                display: "flex", alignItems: "center", justifyContent: "center",
                transition: "all 0.14s",
              }}>
                {active && (
                  <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                    <path d="M2.5 6l2.5 2.5 4.5-5" stroke="#000" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                )}
              </div>

              {/* Icon */}
              <div style={{ color: active ? "#10b981" : "#555", flexShrink: 0, transition: "color 0.14s" }}>
                {uc.icon}
              </div>

              {/* Text */}
              <div>
                <div style={{ fontSize: 15, fontWeight: 600, color: "#ededed", marginBottom: 4 }}>
                  {uc.title}
                </div>
                <div style={{ fontSize: 13, color: "#888", lineHeight: 1.5 }}>
                  {uc.description}
                </div>
              </div>
            </button>
          );
        })}
      </div>

      <button
        type="button"
        onClick={handleContinue}
        disabled={selected.size === 0 || submitting}
        style={{
          width: "100%", padding: "12px 0", fontSize: 14, fontWeight: 600,
          background: selected.size > 0 && !submitting ? "#10b981" : "#1a1a1a",
          color: selected.size > 0 && !submitting ? "#000" : "#555",
          border: "none", borderRadius: 8,
          cursor: selected.size > 0 && !submitting ? "pointer" : "not-allowed",
          transition: "background 0.14s, color 0.14s",
        }}
      >
        {submitting ? "Setting up..." : "Get Started"}
      </button>
    </div>
  );
}
