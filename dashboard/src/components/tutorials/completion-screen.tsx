"use client";

// Celebration screen shown after a tutorial completes. Reused by every
// tutorial (quickstart, post_with_api, ...). Renders:
//   - Confetti animation (~4s, then dismissable)
//   - Title + "Congratulations, you finished <Tutorial>" message
//   - Completed tutorials list (checkmarks)
//   - Remaining tutorials (with Start CTAs) — all optional
//   - Optional handoff CTA for the next suggested tutorial
//   - "Done" button that closes the screen
//
// Fires on first completion only — replays via the /tutorials page
// skip the celebration and show a small toast instead.

import { useEffect } from "react";
import { Check, ArrowRight } from "lucide-react";
import { Backdrop, cardStyle } from "./tutorial-shell";
import {
  TUTORIAL_REGISTRY,
  prerequisitesMet,
  type TutorialDefinition,
} from "./registry";
import type { TutorialId } from "@/lib/api";

export function CompletionScreen({
  completedTutorialId,
  completedIds,
  onStartTutorial,
  onDone,
}: {
  completedTutorialId: TutorialId;
  completedIds: Set<TutorialId>;
  onStartTutorial: (id: TutorialId) => void;
  onDone: () => void;
}) {
  const tutorial = TUTORIAL_REGISTRY.find((t) => t.id === completedTutorialId);
  if (!tutorial) return null;

  const allCompleted = TUTORIAL_REGISTRY.filter((t) => completedIds.has(t.id));
  const remaining = TUTORIAL_REGISTRY.filter(
    (t) => !completedIds.has(t.id) && prerequisitesMet(t, completedIds),
  );

  // Prefer the handoff target on top of the remaining list.
  const handoffTarget =
    tutorial.handoff && remaining.find((t) => t.id === tutorial.handoff!.tutorialId);

  return (
    <Backdrop onBackdropClick={onDone}>
      <div style={{ ...cardStyle, position: "relative", overflow: "hidden" }}>
        <ConfettiCanvas />

        {/* Header */}
        <div style={{ textAlign: "center", marginBottom: 20, position: "relative", zIndex: 1 }}>
          <div style={{ fontSize: 40, marginBottom: 4 }}>🎉</div>
          <div className="dt-body" style={{ fontWeight: 700, color: "var(--dtext)", fontSize: 20 }}>
            Congratulations!
          </div>
          <div className="dt-body-sm" style={{ color: "var(--dmuted)", marginTop: 6 }}>
            You finished the <strong style={{ color: "var(--dtext)" }}>{tutorial.title}</strong> tutorial.
          </div>
        </div>

        {/* Completed list */}
        {allCompleted.length > 0 && (
          <div style={{ marginBottom: 18, position: "relative", zIndex: 1 }}>
            <div className="dt-mono" style={{ fontSize: 10, fontWeight: 600, color: "var(--dmuted2)", letterSpacing: "0.08em", textTransform: "uppercase", marginBottom: 8 }}>
              Completed
            </div>
            <div style={{ display: "grid", gap: 6 }}>
              {allCompleted.map((t) => (
                <div key={t.id} style={{
                  display: "flex", alignItems: "center", gap: 10,
                  padding: "8px 12px", borderRadius: 8,
                  background: "rgba(16,185,129,.05)",
                  border: "1px solid rgba(16,185,129,.15)",
                }}>
                  <div style={{
                    width: 18, height: 18, borderRadius: "50%",
                    background: "var(--daccent)", color: "var(--primary-foreground)",
                    display: "flex", alignItems: "center", justifyContent: "center",
                    flexShrink: 0,
                  }}>
                    <Check style={{ width: 11, height: 11 }} strokeWidth={3} />
                  </div>
                  <div className="dt-body-sm" style={{ color: "var(--dtext)", fontWeight: 500 }}>
                    {t.title}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Remaining list */}
        {remaining.length > 0 && (
          <div style={{ marginBottom: 18, position: "relative", zIndex: 1 }}>
            <div className="dt-mono" style={{ fontSize: 10, fontWeight: 600, color: "var(--dmuted2)", letterSpacing: "0.08em", textTransform: "uppercase", marginBottom: 8 }}>
              Up next — optional
            </div>
            {handoffTarget && tutorial.handoff && (
              <div style={{ marginBottom: 10 }}>
                <div className="dt-body-sm" style={{ color: "var(--dtext)", marginBottom: 8 }}>
                  {tutorial.handoff.prompt}
                </div>
                <button
                  type="button"
                  onClick={() => onStartTutorial(handoffTarget.id)}
                  className="dt-body-sm"
                  style={{
                    display: "inline-flex", alignItems: "center", gap: 6,
                    padding: "8px 14px", borderRadius: 8,
                    background: "var(--daccent)", color: "var(--primary-foreground)",
                    border: "none", cursor: "pointer", fontWeight: 600,
                    fontFamily: "inherit",
                  }}
                >
                  {tutorial.handoff.ctaLabel}
                  <ArrowRight style={{ width: 13, height: 13 }} />
                </button>
              </div>
            )}
            <div style={{ display: "grid", gap: 6 }}>
              {remaining
                .filter((t) => !handoffTarget || t.id !== handoffTarget.id)
                .map((t) => (
                  <button
                    key={t.id}
                    type="button"
                    onClick={() => onStartTutorial(t.id)}
                    style={{
                      display: "flex", alignItems: "center", gap: 10,
                      padding: "10px 12px", borderRadius: 8,
                      background: "transparent",
                      border: "1px solid var(--dborder)",
                      cursor: "pointer", fontFamily: "inherit",
                      textAlign: "left", width: "100%",
                    }}
                    onMouseEnter={(e) => { e.currentTarget.style.background = "var(--sidebar-accent)"; }}
                    onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; }}
                  >
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div className="dt-body-sm" style={{ color: "var(--dtext)", fontWeight: 500 }}>
                        {t.title}
                      </div>
                      <div className="dt-body-sm" style={{ color: "var(--dmuted)", fontSize: 12 }}>
                        {t.description}
                      </div>
                    </div>
                    <ArrowRight style={{ width: 13, height: 13, color: "var(--dmuted2)", flexShrink: 0 }} />
                  </button>
                ))}
            </div>
          </div>
        )}

        <div style={{ textAlign: "right", position: "relative", zIndex: 1 }}>
          <button
            type="button"
            onClick={onDone}
            className="dt-body-sm"
            style={{
              padding: "8px 16px", borderRadius: 8,
              background: "var(--sidebar-accent)", color: "var(--dtext)",
              border: "1px solid var(--dborder)", cursor: "pointer",
              fontFamily: "inherit", fontWeight: 500,
            }}
          >
            Done
          </button>
        </div>
      </div>
    </Backdrop>
  );
}

// Lightweight confetti (no deps). Matches the /welcome page's style.
function ConfettiCanvas() {
  useEffect(() => {
    const canvas = document.getElementById("tutorial-confetti") as HTMLCanvasElement | null;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight;

    const colors = ["#10b981", "#a78bfa", "#38bdf8", "#fb923c", "#f472b6", "#fbbf24"];
    const particles: Array<{
      x: number; y: number; w: number; h: number;
      color: string; vx: number; vy: number; rot: number; vr: number;
      opacity: number;
    }> = [];
    for (let i = 0; i < 120; i++) {
      particles.push({
        x: Math.random() * canvas.width,
        y: Math.random() * canvas.height - canvas.height,
        w: Math.random() * 8 + 4,
        h: Math.random() * 6 + 3,
        color: colors[Math.floor(Math.random() * colors.length)],
        vx: (Math.random() - 0.5) * 3,
        vy: Math.random() * 4 + 2,
        rot: Math.random() * Math.PI * 2,
        vr: (Math.random() - 0.5) * 0.1,
        opacity: 1,
      });
    }
    let frame: number;
    function animate() {
      ctx!.clearRect(0, 0, canvas!.width, canvas!.height);
      let alive = false;
      for (const p of particles) {
        p.x += p.vx;
        p.y += p.vy;
        p.rot += p.vr;
        p.vy += 0.05;
        if (p.y > canvas!.height * 0.7) p.opacity -= 0.02;
        if (p.opacity <= 0) continue;
        alive = true;
        ctx!.save();
        ctx!.globalAlpha = Math.max(0, p.opacity);
        ctx!.translate(p.x, p.y);
        ctx!.rotate(p.rot);
        ctx!.fillStyle = p.color;
        ctx!.fillRect(-p.w / 2, -p.h / 2, p.w, p.h);
        ctx!.restore();
      }
      if (alive) frame = requestAnimationFrame(animate);
    }
    frame = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(frame);
  }, []);

  return (
    <canvas
      id="tutorial-confetti"
      style={{
        position: "fixed", top: 0, left: 0, width: "100vw", height: "100vh",
        pointerEvents: "none", zIndex: 0,
      }}
    />
  );
}

// Unused param suppressor (used in TutorialDefinition but accessed via
// registry here — keep type export consistent).
export type { TutorialDefinition };
