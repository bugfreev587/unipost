"use client";

import { useEffect, useRef, useState } from "react";
import { X } from "lucide-react";
import type { OnboardingIntent } from "@/lib/api";

const OPTIONS: { value: Exclude<OnboardingIntent, "skipped">; title: string; desc: string }[] = [
  {
    value: "exploring",
    title: "Just exploring",
    desc: "Checking out what UniPost can do.",
  },
  {
    value: "own_accounts",
    title: "Publishing to my own social accounts",
    desc: "Connecting my own accounts to post from one place.",
  },
  {
    value: "building_api",
    title: "Building a product with UniPost API",
    desc: "Integrating UniPost's API into my app or service.",
  },
];

/**
 * Welcome modal for intent collection. Non-blocking — users can skip.
 * Shown once per user on first dashboard load (gated by onboarding_shown_at).
 *
 * Props:
 *   - open: whether the modal should render
 *   - initialIntent: pre-select a card (used when re-opened from Settings)
 *   - onSelect: called with the chosen intent when user clicks Continue
 *   - onSkip: called when user clicks Skip, Esc, or the close button
 */
export function WelcomeModal({
  open,
  initialIntent,
  onSelect,
  onSkip,
}: {
  open: boolean;
  initialIntent?: OnboardingIntent;
  onSelect: (intent: Exclude<OnboardingIntent, "skipped">) => void;
  onSkip: () => void;
}) {
  const [selected, setSelected] = useState<OnboardingIntent | null>(
    initialIntent && initialIntent !== "skipped" ? initialIntent : null
  );
  const firstRender = useRef(true);

  // Reset selection each time the modal reopens
  useEffect(() => {
    if (open) {
      setSelected(initialIntent && initialIntent !== "skipped" ? initialIntent : null);
    }
  }, [open, initialIntent]);

  // Esc key to skip
  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        onSkip();
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onSkip]);

  // Animation fade-in
  useEffect(() => {
    if (open) firstRender.current = false;
  }, [open]);

  if (!open) return null;

  return (
    <div
      style={{
        position: "fixed", inset: 0, zIndex: 1000,
        display: "flex", alignItems: "center", justifyContent: "center",
        background: "rgba(0, 0, 0, 0.55)",
        backdropFilter: "blur(4px)",
        animation: firstRender.current ? undefined : "welcome-fade 0.15s ease-out",
      }}
      // Backdrop click does NOT close (per PRD §5.1) — prevents accidental dismissal
      onClick={(e) => e.stopPropagation()}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="welcome-title"
        onClick={(e) => e.stopPropagation()}
        style={{
          width: 480,
          maxWidth: "calc(100vw - 32px)",
          background: "var(--surface-raised)",
          border: "1px solid var(--dborder)",
          borderRadius: 14,
          padding: 28,
          position: "relative",
          boxShadow: "0 24px 64px rgba(0,0,0,.4)",
        }}
      >
        {/* Close button */}
        <button
          type="button"
          onClick={onSkip}
          aria-label="Close"
          style={{
            position: "absolute", top: 14, right: 14,
            width: 28, height: 28, borderRadius: 8,
            border: "none", background: "transparent",
            color: "var(--dmuted)", cursor: "pointer",
            display: "flex", alignItems: "center", justifyContent: "center",
          }}
          onMouseEnter={(e) => { e.currentTarget.style.background = "var(--sidebar-accent)"; e.currentTarget.style.color = "var(--dtext)"; }}
          onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; e.currentTarget.style.color = "var(--dmuted)"; }}
        >
          <X style={{ width: 16, height: 16 }} />
        </button>

        {/* Header */}
        <h2 id="welcome-title" className="dt-heading" style={{ margin: 0, fontSize: 20, fontWeight: 700, color: "var(--dtext)" }}>
          What brings you to UniPost?
        </h2>
        <p className="dt-body-sm" style={{ margin: "8px 0 22px", color: "var(--dmuted)", lineHeight: 1.5 }}>
          Optional — helps us tailor your experience. You can change this later in Workspace Settings.
        </p>

        {/* Options */}
        <div style={{ display: "grid", gap: 10, marginBottom: 24 }}>
          {OPTIONS.map((opt) => {
            const active = selected === opt.value;
            return (
              <button
                key={opt.value}
                type="button"
                onClick={() => setSelected(opt.value)}
                style={{
                  display: "flex",
                  alignItems: "flex-start",
                  gap: 12,
                  padding: "14px 14px",
                  borderRadius: 10,
                  border: active ? "1px solid var(--daccent)" : "1px solid var(--dborder)",
                  background: active ? "var(--accent-dim)" : "var(--sidebar)",
                  cursor: "pointer",
                  textAlign: "left",
                  fontFamily: "inherit",
                  transition: "border-color 0.1s, background 0.1s",
                }}
                onMouseEnter={(e) => {
                  if (!active) {
                    e.currentTarget.style.borderColor = "rgba(16,185,129,.4)";
                  }
                }}
                onMouseLeave={(e) => {
                  if (!active) {
                    e.currentTarget.style.borderColor = "var(--dborder)";
                  }
                }}
              >
                {/* Radio circle */}
                <div
                  style={{
                    width: 18, height: 18, borderRadius: "50%",
                    border: active ? "5px solid var(--daccent)" : "2px solid var(--dmuted2)",
                    flexShrink: 0, marginTop: 2,
                    transition: "border-color 0.1s, border-width 0.1s",
                  }}
                />
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div className="dt-body-sm" style={{ fontWeight: 600, color: "var(--dtext)", marginBottom: 4 }}>
                    {opt.title}
                  </div>
                  <div className="dt-body-sm" style={{ color: "var(--dmuted)", lineHeight: 1.5 }}>
                    {opt.desc}
                  </div>
                </div>
              </button>
            );
          })}
        </div>

        {/* Actions */}
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12 }}>
          {/* Skip is always enabled and visually equal weight (per PRD §7) */}
          <button
            type="button"
            onClick={onSkip}
            className="dt-body-sm"
            style={{
              padding: "10px 18px",
              borderRadius: 8,
              border: "1px solid var(--dborder)",
              background: "transparent",
              color: "var(--dtext)",
              cursor: "pointer",
              fontWeight: 500,
              fontFamily: "inherit",
            }}
          >
            Skip
          </button>
          <button
            type="button"
            onClick={() => selected && selected !== "skipped" && onSelect(selected)}
            disabled={!selected || selected === "skipped"}
            className="dt-body-sm"
            style={{
              padding: "10px 18px",
              borderRadius: 8,
              border: "none",
              background: selected && selected !== "skipped" ? "var(--daccent)" : "rgba(255,255,255,.08)",
              color: selected && selected !== "skipped" ? "var(--primary-foreground)" : "var(--dmuted2)",
              cursor: selected && selected !== "skipped" ? "pointer" : "not-allowed",
              fontWeight: 600,
              fontFamily: "inherit",
            }}
          >
            Continue
          </button>
        </div>
      </div>

      <style>{`
        @keyframes welcome-fade {
          from { opacity: 0; }
          to { opacity: 1; }
        }
      `}</style>
    </div>
  );
}
