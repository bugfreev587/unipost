"use client";

// Shared modal chrome + default step-list body for tutorials.
//
// A tutorial's definition in registry.ts describes the steps; this
// component:
//   - Renders the modal header (title, description, close button)
//   - Renders the progress bar
//   - By default, renders the step list (completed -> active -> locked)
//     with CTA links to the step's ctaHref
//   - When the tutorial defines a custom renderBody (e.g. post_with_api)
//     delegates body rendering to that function
//
// Completion / dismissal:
//   - For completeOn: "all_steps_done", completion is auto-detected by
//     the host on count changes.
//   - For completeOn: "done_button", the body is responsible for calling
//     onRequestComplete.
//   - Close button (X/Esc/backdrop) calls onRequestClose. Dismissal
//     policy (mandatory re-pop vs permanent) is decided by the host.

import { useEffect } from "react";
import Link from "next/link";
import { Check, Lock, X, ArrowRight } from "lucide-react";
import {
  stepCompleted,
  type TutorialContext,
  type TutorialDefinition,
} from "./registry";

export function TutorialShell({
  tutorial,
  ctx,
  replayMode = false,
  onRequestClose,
  onRequestComplete,
}: {
  tutorial: TutorialDefinition;
  ctx: TutorialContext;
  replayMode?: boolean;
  onRequestClose: () => void;
  onRequestComplete: () => void;
}) {
  // Esc key closes the modal.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        onRequestClose();
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onRequestClose]);

  const stepsWithState = tutorial.steps.map((s) => ({
    ...s,
    completed: replayMode ? false : stepCompleted(s.signal, ctx.counts),
  }));
  const completedCount = stepsWithState.filter((s) => s.completed).length;
  const total = stepsWithState.length;
  const progressPct = total === 0 ? 0 : Math.round((completedCount / total) * 100);

  return (
    <Backdrop onBackdropClick={onRequestClose}>
      <div style={cardStyle}>
        {/* Header */}
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 16 }}>
          <div>
            <div className="dt-body" style={{ fontWeight: 700, color: "var(--dtext)", marginBottom: 3, fontSize: 17 }}>
              {tutorial.id === "quickstart" ? "👋 " : ""}{tutorial.title}
            </div>
            <div className="dt-body-sm" style={{ color: "var(--dmuted)" }}>
              {tutorial.description}
            </div>
            {replayMode && (
              <div className="dt-mono" style={{ fontSize: 10, color: "var(--daccent)", marginTop: 8, letterSpacing: "0.08em", textTransform: "uppercase" }}>
                Replay mode
              </div>
            )}
          </div>
          <button
            type="button"
            onClick={onRequestClose}
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
          <div style={{ width: "100%", height: 4, borderRadius: 999, background: "rgba(255,255,255,.06)", overflow: "hidden" }}>
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

        {/* Body */}
        {tutorial.renderBody ? (
          tutorial.renderBody({
            ctx,
            steps: stepsWithState,
            onRequestComplete,
            onRequestClose,
          })
        ) : (
          <DefaultStepList
            tutorial={tutorial}
            ctx={ctx}
            replayMode={replayMode}
            stepsWithState={stepsWithState}
            onStepCtaClick={onRequestClose}
          />
        )}
      </div>
    </Backdrop>
  );
}

function DefaultStepList({
  tutorial,
  ctx,
  replayMode = false,
  stepsWithState,
  onStepCtaClick,
}: {
  tutorial: TutorialDefinition;
  ctx: TutorialContext;
  replayMode?: boolean;
  stepsWithState: ReadonlyArray<{
    id: string;
    title: string;
    description: string;
    hint?: string;
    ctaLabel?: string;
    ctaHref?: (ctx: TutorialContext) => string;
    completed: boolean;
  }>;
  onStepCtaClick: () => void;
}) {
  // First incomplete step is active, rest are locked.
  const firstIncompleteIdx = stepsWithState.findIndex((s) => !s.completed);
  const stepState = (idx: number, completed: boolean): "completed" | "active" | "locked" => {
    if (replayMode) return "active";
    if (completed) return "completed";
    return idx === firstIncompleteIdx ? "active" : "locked";
  };

  // Suppress unused-param lint warning (tutorial arg kept for parity with
  // renderBody signature — useful if we later gate CTAs on tutorial flags).
  void tutorial;

  return (
    <div style={{ display: "grid", gap: 8 }}>
      {stepsWithState.map((step, idx) => {
        const state = stepState(idx, step.completed);
        return (
          <div
            key={step.id}
            style={{
              display: "flex", alignItems: "center", gap: 12,
              padding: "12px 14px", borderRadius: 10,
              border: state === "active" ? "1px solid rgba(16,185,129,.25)" : "1px solid var(--dborder)",
              background: state === "active" ? "rgba(16,185,129,.04)" : "transparent",
              opacity: state === "locked" ? 0.5 : 1,
            }}
          >
            <div style={{ flexShrink: 0 }}>
              {state === "completed" ? (
                <div style={{ width: 22, height: 22, borderRadius: "50%", background: "var(--daccent)", color: "var(--primary-foreground)", display: "flex", alignItems: "center", justifyContent: "center" }}>
                  <Check style={{ width: 14, height: 14 }} strokeWidth={3} />
                </div>
              ) : state === "locked" ? (
                <div style={{ width: 22, height: 22, borderRadius: "50%", border: "1px solid var(--dborder)", display: "flex", alignItems: "center", justifyContent: "center", color: "var(--dmuted2)" }}>
                  <Lock style={{ width: 11, height: 11 }} />
                </div>
              ) : (
                <div style={{ width: 22, height: 22, borderRadius: "50%", border: "2px solid var(--daccent)" }} />
              )}
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div className="dt-body-sm" style={{
                fontWeight: 600,
                color: state === "completed" ? "var(--dmuted)" : "var(--dtext)",
                textDecoration: state === "completed" ? "line-through" : "none",
                marginBottom: 2,
              }}>
                {step.title}
              </div>
              <div className="dt-body-sm" style={{ color: "var(--dmuted)" }}>
                {state === "locked"
                  ? `Unlocks after step ${idx}`
                  : state === "completed"
                    ? "Completed"
                    : step.description}
                {state === "active" && step.hint && (
                  <span className="dt-mono" style={{ fontSize: 10, color: "var(--dmuted2)", marginLeft: 8 }}>
                    {step.hint}
                  </span>
                )}
              </div>
            </div>
            {state === "active" && step.ctaHref && step.ctaLabel && (
              <Link
                href={step.ctaHref(ctx)}
                onClick={onStepCtaClick}
                className="dt-body-sm"
                style={{
                  display: "inline-flex", alignItems: "center", gap: 6,
                  padding: "8px 14px", borderRadius: 8,
                  background: "var(--daccent)", color: "var(--primary-foreground)",
                  textDecoration: "none", fontWeight: 600, whiteSpace: "nowrap",
                  flexShrink: 0,
                }}
              >
                {step.ctaLabel}
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
  );
}

export function Backdrop({ children, onBackdropClick }: { children: React.ReactNode; onBackdropClick: () => void }) {
  return (
    <div
      onClick={onBackdropClick}
      style={{
        position: "fixed", inset: 0, zIndex: 900,
        display: "flex", alignItems: "center", justifyContent: "center",
        // Heavy overlay + blur so page content behind doesn't read
        // through the modal. The tutorials page has a lot of visible
        // text that bled through at 0.55 + blur(4).
        background: "rgba(0, 0, 0, 0.72)",
        backdropFilter: "blur(10px)",
        WebkitBackdropFilter: "blur(10px)",
        animation: "tutorial-fade 0.15s ease-out",
        padding: 16,
      }}
    >
      <div onClick={(e) => e.stopPropagation()}>{children}</div>
      <style>{`@keyframes tutorial-fade { from { opacity: 0; } to { opacity: 1; } }`}</style>
    </div>
  );
}

export const cardStyle: React.CSSProperties = {
  width: 640,
  maxWidth: "calc(100vw - 32px)",
  padding: 22,
  borderRadius: 14,
  overflow: "hidden",
  background: "var(--surface-raised)",
  border: "1px solid var(--dborder)",
  boxShadow: "0 24px 64px rgba(0,0,0,.4)",
};
