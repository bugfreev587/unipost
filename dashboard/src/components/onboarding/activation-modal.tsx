"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { Check, Lock, X, ArrowRight } from "lucide-react";
import {
  getActivation,
  dismissActivation,
  getMe,
  type ActivationStep,
  type ActivationStepId,
  type OnboardingIntent,
} from "@/lib/api";
import { track } from "@/lib/analytics";

// Visual step definition — merged with server-side completion state
// at render time. Ordered per PRD default; `building_api` intent
// reorders (see orderSteps below).
type StepMeta = {
  id: ActivationStepId;
  title: string;
  description: string;
  hint?: string;
  ctaLabel: string;
  ctaPath: (profileId: string) => string;
};

const STEP_META: Record<ActivationStepId, StepMeta> = {
  connect_account: {
    id: "connect_account",
    title: "Connect your first account",
    description: "Bluesky, LinkedIn, Instagram, and more.",
    hint: "~30 seconds",
    ctaLabel: "Connect account",
    // first=1 tells the accounts page to filter the platform picker
    // to text/image platforms only (no TikTok/YouTube video-first flows).
    ctaPath: (id) => `/projects/${id}/accounts?action=new&first=1`,
  },
  send_post: {
    id: "send_post",
    title: "Send your first post",
    description: "Try it with any connected account.",
    ctaLabel: "Send test post",
    ctaPath: (id) => `/projects/${id}/posts?action=new&template=welcome`,
  },
  create_api_key: {
    id: "create_api_key",
    title: "Get your API key",
    description: "For building on UniPost API.",
    ctaLabel: "Create API key",
    ctaPath: (id) => `/projects/${id}/api-keys?action=new`,
  },
};

function orderSteps(
  steps: ActivationStep[],
  intent: OnboardingIntent | null
): ActivationStep[] {
  if (intent === "building_api") {
    const byId = new Map(steps.map((s) => [s.id, s]));
    const order: ActivationStepId[] = ["connect_account", "create_api_key", "send_post"];
    return order.map((id) => byId.get(id)!).filter(Boolean);
  }
  return steps;
}

/**
 * Activation modal — popped on the project dashboard root for new
 * users until they complete the first two steps (connect account,
 * send post).
 *
 * Behavior matrix:
 *   - Steps 0-1 completed: modal auto-opens on every /projects/[id]
 *     visit. User can close it in the moment (Esc / backdrop / X),
 *     but next visit it re-pops until steps 1+2 are done.
 *   - Steps 1+2 completed: modal pops once more showing step 3 as
 *     optional. Closing it (X, Esc, backdrop, or "I'll do this later")
 *     sets activation_guide_dismissed_at so it never re-appears.
 *   - All 3 completed: celebration state, then never again.
 *   - Dismissed or completed: never opens.
 */
export function ActivationModal({ profileId }: { profileId: string }) {
  const { getToken } = useAuth();
  const [loaded, setLoaded] = useState(false);
  const [open, setOpen] = useState(false);
  const [completed, setCompleted] = useState(false);
  const [dismissed, setDismissed] = useState(false);
  const [steps, setSteps] = useState<ActivationStep[]>([]);
  const [intent, setIntent] = useState<OnboardingIntent | null>(null);
  const [celebrating, setCelebrating] = useState(false);
  const shownRef = useRef(false);
  const completionLoggedRef = useRef(false);
  const prevStepsRef = useRef<Map<ActivationStepId, boolean>>(new Map());

  const load = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const [actRes, meRes] = await Promise.all([getActivation(token), getMe(token)]);
      setCompleted(actRes.data.completed);
      setDismissed(actRes.data.dismissed);
      setSteps(actRes.data.steps);
      setIntent(meRes.data.onboarding_intent ?? null);

      // Detect step transitions incomplete → complete and fire events.
      for (const s of actRes.data.steps) {
        const prev = prevStepsRef.current.get(s.id);
        if (prev === false && s.completed) {
          track("activation_step_completed", { step_id: s.id });
        }
        prevStepsRef.current.set(s.id, s.completed);
      }

      if (actRes.data.completed && !completionLoggedRef.current) {
        completionLoggedRef.current = true;
        const wasAllDone = actRes.data.steps.every((s) => prevStepsRef.current.get(s.id) !== false);
        if (!wasAllDone) {
          track("activation_completed");
          setCelebrating(true);
          setOpen(true);
          setTimeout(() => {
            setOpen(false);
            setCelebrating(false);
          }, 3000);
        }
      } else if (!actRes.data.completed && !actRes.data.dismissed) {
        // Not completed, not dismissed → pop the modal.
        setOpen(true);
      }

      setLoaded(true);
    } catch {
      /* silent */
    }
  }, [getToken]);

  useEffect(() => {
    load();
  }, [load]);

  // Re-poll on window focus so completing a step on another tab / coming
  // back from a CTA page reflects immediately.
  useEffect(() => {
    function onFocus() {
      load();
    }
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [load]);

  // Esc key closes the modal (treated as in-the-moment skip).
  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        handleClose();
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  // Fire "modal shown" once per actual open transition.
  useEffect(() => {
    if (!loaded || !open || celebrating || shownRef.current) return;
    shownRef.current = true;
    const completedCount = steps.filter((s) => s.completed).length;
    track("activation_card_shown", { completed_steps_count: completedCount });
  }, [loaded, open, celebrating, steps]);

  async function handleClose() {
    setOpen(false);
    const completedCount = steps.filter((s) => s.completed).length;
    const firstTwoDone = completedCount >= 2;
    track("activation_card_dismissed", { completed_steps_count: completedCount });

    // Auto-persist dismissal once the user has done the first 2 steps
    // (the critical activation). The remaining step 3 is optional.
    if (firstTwoDone) {
      try {
        const token = await getToken();
        if (token) await dismissActivation(token);
      } catch {
        /* silent */
      }
      setDismissed(true);
    }
    // Else: temporary dismiss — the modal re-opens on the next
    // `/projects/[id]` visit via the load effect.
    shownRef.current = false;
  }

  function handleStepClick(stepId: ActivationStepId) {
    track("activation_step_clicked", { step_id: stepId });
    // Close the modal so the destination page (accounts / posts /
    // api-keys) is visible with its auto-opened drawer/dialog.
    setOpen(false);
    shownRef.current = false;
  }

  if (!loaded || !open) return null;
  if (dismissed && !celebrating) return null;

  const orderedSteps = orderSteps(steps, intent);
  const completedCount = orderedSteps.filter((s) => s.completed).length;
  const total = orderedSteps.length;
  const progressPct = total === 0 ? 0 : Math.round((completedCount / total) * 100);
  const firstTwoDone = completedCount >= 2;

  // The FIRST incomplete step is active; everything after is locked;
  // completed steps are displayed with a check.
  function stepState(idx: number, step: ActivationStep): "completed" | "active" | "locked" {
    if (step.completed) return "completed";
    const firstIncomplete = orderedSteps.findIndex((s) => !s.completed);
    return idx === firstIncomplete ? "active" : "locked";
  }

  if (celebrating) {
    return (
      <Backdrop onBackdropClick={handleClose}>
        <div style={{ ...cardStyle, padding: 28, textAlign: "center" }}>
          <div style={{ fontSize: 40, marginBottom: 8 }}>🎉</div>
          <div className="dt-body" style={{ fontWeight: 700, color: "var(--dtext)", fontSize: 18 }}>
            You&apos;re all set!
          </div>
          <div className="dt-body-sm" style={{ color: "var(--dmuted)", marginTop: 6 }}>
            Explore the dashboard to make the most of UniPost.
          </div>
        </div>
      </Backdrop>
    );
  }

  return (
    <Backdrop onBackdropClick={handleClose}>
      <div style={cardStyle}>
        {/* Header */}
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 16 }}>
          <div>
            <div className="dt-body" style={{ fontWeight: 700, color: "var(--dtext)", marginBottom: 3, fontSize: 17 }}>
              👋 Welcome to UniPost
            </div>
            <div className="dt-body-sm" style={{ color: "var(--dmuted)" }}>
              Let&apos;s get you posting in 2 minutes.
            </div>
          </div>
          <button
            type="button"
            onClick={handleClose}
            aria-label="Close"
            style={{
              width: 28, height: 28, borderRadius: 8,
              border: "none", background: "transparent",
              color: "var(--dmuted)", cursor: "pointer",
              display: "flex", alignItems: "center", justifyContent: "center",
            }}
            onMouseEnter={(e) => { e.currentTarget.style.color = "var(--dtext)"; e.currentTarget.style.background = "var(--sidebar-accent)"; }}
            onMouseLeave={(e) => { e.currentTarget.style.color = "var(--dmuted)"; e.currentTarget.style.background = "transparent"; }}
          >
            <X style={{ width: 14, height: 14 }} />
          </button>
        </div>

        {/* Progress bar */}
        <div style={{ marginBottom: 18 }}>
          <div style={{
            width: "100%", height: 4, borderRadius: 999,
            background: "rgba(255,255,255,.06)", overflow: "hidden",
          }}>
            <div style={{
              width: `${progressPct}%`, height: "100%",
              background: "var(--daccent)",
              transition: "width 0.3s ease",
            }} />
          </div>
          <div className="dt-mono" style={{ fontSize: 10, color: "var(--dmuted2)", marginTop: 6 }}>
            {completedCount} of {total} completed
          </div>
        </div>

        {/* Steps */}
        <div style={{ display: "grid", gap: 8 }}>
          {orderedSteps.map((step, idx) => {
            const meta = STEP_META[step.id];
            const state = stepState(idx, step);

            return (
              <div
                key={step.id}
                style={{
                  display: "flex", alignItems: "center", gap: 12,
                  padding: "12px 14px",
                  borderRadius: 10,
                  border: state === "active" ? "1px solid rgba(16,185,129,.25)" : "1px solid var(--dborder)",
                  background: state === "active" ? "rgba(16,185,129,.04)" : "transparent",
                  opacity: state === "locked" ? 0.5 : 1,
                }}
              >
                <div style={{ flexShrink: 0 }}>
                  {state === "completed" ? (
                    <div style={{
                      width: 22, height: 22, borderRadius: "50%",
                      background: "var(--daccent)", color: "var(--primary-foreground)",
                      display: "flex", alignItems: "center", justifyContent: "center",
                    }}>
                      <Check style={{ width: 14, height: 14 }} strokeWidth={3} />
                    </div>
                  ) : state === "locked" ? (
                    <div style={{
                      width: 22, height: 22, borderRadius: "50%",
                      border: "1px solid var(--dborder)",
                      display: "flex", alignItems: "center", justifyContent: "center",
                      color: "var(--dmuted2)",
                    }}>
                      <Lock style={{ width: 11, height: 11 }} />
                    </div>
                  ) : (
                    <div style={{
                      width: 22, height: 22, borderRadius: "50%",
                      border: "2px solid var(--daccent)",
                    }} />
                  )}
                </div>

                <div style={{ flex: 1, minWidth: 0 }}>
                  <div className="dt-body-sm" style={{
                    fontWeight: 600,
                    color: state === "completed" ? "var(--dmuted)" : "var(--dtext)",
                    textDecoration: state === "completed" ? "line-through" : "none",
                    marginBottom: 2,
                  }}>
                    {meta.title}
                    {step.id === "create_api_key" && state !== "completed" && (
                      <span className="dt-mono" style={{
                        fontSize: 9, color: "var(--dmuted2)",
                        marginLeft: 8, padding: "1px 6px", borderRadius: 4,
                        border: "1px solid var(--dborder)",
                      }}>
                        OPTIONAL
                      </span>
                    )}
                  </div>
                  <div className="dt-body-sm" style={{ color: "var(--dmuted)" }}>
                    {state === "locked"
                      ? `Unlocks after step ${idx}`
                      : state === "completed"
                        ? "Completed"
                        : meta.description}
                    {state === "active" && meta.hint && (
                      <span className="dt-mono" style={{ fontSize: 10, color: "var(--dmuted2)", marginLeft: 8 }}>
                        {meta.hint}
                      </span>
                    )}
                  </div>
                </div>

                {state === "active" && (
                  <Link
                    href={meta.ctaPath(profileId)}
                    onClick={() => handleStepClick(step.id)}
                    className="dt-body-sm"
                    style={{
                      display: "inline-flex", alignItems: "center", gap: 6,
                      padding: "8px 14px", borderRadius: 8,
                      background: "var(--daccent)", color: "var(--primary-foreground)",
                      textDecoration: "none", fontWeight: 600, whiteSpace: "nowrap",
                      flexShrink: 0,
                    }}
                  >
                    {meta.ctaLabel}
                    <ArrowRight style={{ width: 13, height: 13 }} />
                  </Link>
                )}
                {state === "completed" && (
                  <span className="dt-mono" style={{
                    fontSize: 10, color: "var(--daccent)",
                    padding: "4px 10px", borderRadius: 6,
                    background: "rgba(16,185,129,.10)",
                    border: "1px solid rgba(16,185,129,.22)",
                    flexShrink: 0,
                  }}>
                    COMPLETED
                  </span>
                )}
              </div>
            );
          })}
        </div>

        {/* Once the first two steps are done, step 3 is optional — surface
            an explicit "I'll do this later" so the intent is clear. */}
        {firstTwoDone && (
          <div style={{ marginTop: 16, textAlign: "right" }}>
            <button
              type="button"
              onClick={handleClose}
              className="dt-body-sm"
              style={{
                border: "none", background: "transparent",
                color: "var(--dmuted)", cursor: "pointer", fontFamily: "inherit",
                padding: "6px 10px",
              }}
            >
              I&apos;ll do this later
            </button>
          </div>
        )}
      </div>
    </Backdrop>
  );
}

function Backdrop({ children, onBackdropClick }: { children: React.ReactNode; onBackdropClick: () => void }) {
  return (
    <div
      onClick={onBackdropClick}
      style={{
        position: "fixed", inset: 0, zIndex: 900,
        display: "flex", alignItems: "center", justifyContent: "center",
        background: "rgba(0, 0, 0, 0.55)",
        backdropFilter: "blur(4px)",
        animation: "activation-fade 0.15s ease-out",
        padding: 16,
      }}
    >
      <div onClick={(e) => e.stopPropagation()}>
        {children}
      </div>
      <style>{`
        @keyframes activation-fade {
          from { opacity: 0; }
          to { opacity: 1; }
        }
      `}</style>
    </div>
  );
}

const cardStyle: React.CSSProperties = {
  width: 540,
  maxWidth: "calc(100vw - 32px)",
  padding: 22,
  borderRadius: 14,
  background: "var(--dbackground)",
  border: "1px solid var(--dborder)",
  boxShadow: "0 24px 64px rgba(0,0,0,.4)",
};
