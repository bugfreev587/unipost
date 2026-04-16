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
    ctaPath: (id) => `/projects/${id}/accounts?action=new`,
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
  // PRD §5.4: building_api users see connect → api_key → send_post.
  if (intent === "building_api") {
    const byId = new Map(steps.map((s) => [s.id, s]));
    const order: ActivationStepId[] = ["connect_account", "create_api_key", "send_post"];
    return order.map((id) => byId.get(id)!).filter(Boolean);
  }
  return steps;
}

export function ActivationCard({ profileId }: { profileId: string }) {
  const { getToken } = useAuth();
  const [loaded, setLoaded] = useState(false);
  const [hidden, setHidden] = useState(false);
  const [completed, setCompleted] = useState(false);
  const [dismissed, setDismissed] = useState(false);
  const [steps, setSteps] = useState<ActivationStep[]>([]);
  const [intent, setIntent] = useState<OnboardingIntent | null>(null);
  const [showDismissConfirm, setShowDismissConfirm] = useState(false);
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

      // Fire per-step completion events when a step transitions from
      // incomplete → complete (detects completion on this dashboard visit).
      for (const s of actRes.data.steps) {
        const prev = prevStepsRef.current.get(s.id);
        if (prev === false && s.completed) {
          track("activation_step_completed", { step_id: s.id });
        }
        prevStepsRef.current.set(s.id, s.completed);
      }

      // Log the "completed" milestone the first time we observe it.
      if (actRes.data.completed && !completionLoggedRef.current) {
        completionLoggedRef.current = true;
        // Only celebrate if we're transitioning (i.e., at least one prev
        // render had incomplete) — avoids celebrating on every load for
        // users who finished long ago.
        const wasAllDone = actRes.data.steps.every((s) => prevStepsRef.current.get(s.id) !== false);
        if (!wasAllDone && actRes.data.steps.some((s) => s.completed)) {
          track("activation_completed");
          setCelebrating(true);
          setTimeout(() => setHidden(true), 3000);
        }
      }

      setLoaded(true);
    } catch {
      /* silent — card just doesn't render */
    }
  }, [getToken]);

  useEffect(() => {
    load();
  }, [load]);

  // Poll when the user comes back from a CTA page so newly-completed
  // steps appear without needing a full reload.
  useEffect(() => {
    function onFocus() {
      load();
    }
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [load]);

  // Fire "card shown" exactly once per mount.
  useEffect(() => {
    if (!loaded || shownRef.current) return;
    if (completed || dismissed || hidden) return;
    shownRef.current = true;
    const completedCount = steps.filter((s) => s.completed).length;
    track("activation_card_shown", { completed_steps_count: completedCount });
  }, [loaded, completed, dismissed, hidden, steps]);

  async function handleDismiss() {
    setShowDismissConfirm(false);
    setHidden(true);
    const completedCount = steps.filter((s) => s.completed).length;
    track("activation_card_dismissed", { completed_steps_count: completedCount });
    try {
      const token = await getToken();
      if (token) await dismissActivation(token);
    } catch {
      /* silent — UI is already hidden */
    }
  }

  function handleStepClick(stepId: ActivationStepId) {
    track("activation_step_clicked", { step_id: stepId });
  }

  // Hide conditions: not loaded, completed+celebration-done, or dismissed.
  if (!loaded || hidden) return null;
  if ((completed && !celebrating) || dismissed) return null;

  const orderedSteps = orderSteps(steps, intent);
  const completedCount = orderedSteps.filter((s) => s.completed).length;
  const total = orderedSteps.length;
  const progressPct = total === 0 ? 0 : Math.round((completedCount / total) * 100);

  // Step state: the FIRST incomplete step is "active"; everything after it
  // is "locked"; completed steps are "completed". This matches PRD §5.3.
  function stepState(idx: number, step: ActivationStep): "completed" | "active" | "locked" {
    if (step.completed) return "completed";
    const firstIncomplete = orderedSteps.findIndex((s) => !s.completed);
    return idx === firstIncomplete ? "active" : "locked";
  }

  if (celebrating) {
    return (
      <div style={cardWrapStyle}>
        <div style={{ padding: 28, textAlign: "center" }}>
          <div style={{ fontSize: 32, marginBottom: 6 }}>🎉</div>
          <div className="dt-body" style={{ fontWeight: 700, color: "var(--dtext)" }}>
            You&apos;re all set!
          </div>
          <div className="dt-body-sm" style={{ color: "var(--dmuted)", marginTop: 4 }}>
            Explore the dashboard to make the most of UniPost.
          </div>
        </div>
      </div>
    );
  }

  return (
    <div style={cardWrapStyle}>
      {/* Header */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 14 }}>
        <div>
          <div className="dt-body" style={{ fontWeight: 700, color: "var(--dtext)", marginBottom: 2 }}>
            👋 Welcome to UniPost
          </div>
          <div className="dt-body-sm" style={{ color: "var(--dmuted)" }}>
            Let&apos;s get you posting in 2 minutes.
          </div>
        </div>
        <button
          type="button"
          onClick={() => setShowDismissConfirm(true)}
          aria-label="Dismiss"
          style={{
            width: 26, height: 26, borderRadius: 6,
            border: "none", background: "transparent",
            color: "var(--dmuted2)", cursor: "pointer",
            display: "flex", alignItems: "center", justifyContent: "center",
          }}
          onMouseEnter={(e) => { e.currentTarget.style.color = "var(--dtext)"; e.currentTarget.style.background = "rgba(255,255,255,.04)"; }}
          onMouseLeave={(e) => { e.currentTarget.style.color = "var(--dmuted2)"; e.currentTarget.style.background = "transparent"; }}
        >
          <X style={{ width: 14, height: 14 }} />
        </button>
      </div>

      {/* Progress bar */}
      <div style={{ marginBottom: 16 }}>
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
              {/* Icon */}
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

              {/* Text */}
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

              {/* CTA */}
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

      {/* Dismiss confirmation */}
      {showDismissConfirm && (
        <div
          onClick={() => setShowDismissConfirm(false)}
          style={{
            position: "fixed", inset: 0, zIndex: 1000,
            display: "flex", alignItems: "center", justifyContent: "center",
            background: "rgba(0,0,0,.55)", backdropFilter: "blur(4px)",
          }}
        >
          <div
            onClick={(e) => e.stopPropagation()}
            style={{
              width: 400, maxWidth: "calc(100vw - 32px)",
              background: "var(--dbackground)", border: "1px solid var(--dborder)",
              borderRadius: 12, padding: 22,
            }}
          >
            <div className="dt-body" style={{ fontWeight: 700, color: "var(--dtext)", marginBottom: 8 }}>
              Hide this guide?
            </div>
            <div className="dt-body-sm" style={{ color: "var(--dmuted)", marginBottom: 18 }}>
              You can always find setup help in Settings.
            </div>
            <div style={{ display: "flex", justifyContent: "flex-end", gap: 10 }}>
              <button
                type="button"
                onClick={() => setShowDismissConfirm(false)}
                className="dt-body-sm"
                style={{
                  padding: "8px 14px", borderRadius: 6,
                  border: "1px solid var(--dborder)", background: "transparent",
                  color: "var(--dtext)", cursor: "pointer", fontFamily: "inherit",
                }}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleDismiss}
                className="dt-body-sm"
                style={{
                  padding: "8px 14px", borderRadius: 6,
                  border: "none", background: "var(--dtext)",
                  color: "var(--dbackground)", cursor: "pointer", fontFamily: "inherit",
                  fontWeight: 600,
                }}
              >
                Hide
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

const cardWrapStyle: React.CSSProperties = {
  padding: 18,
  borderRadius: 14,
  background: "rgba(16,185,129,.03)",
  border: "1px solid rgba(16,185,129,.18)",
  marginBottom: 24,
};
