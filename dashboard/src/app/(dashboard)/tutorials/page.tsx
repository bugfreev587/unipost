"use client";

// /tutorials — central hub listing all available tutorials with
// completion state. Users land here via the GraduationCap icon in
// the sidebar, or via the "Start tutorial" CTAs on the celebration
// screen.
//
// Each tutorial card shows:
//   - title + description
//   - state badge (Completed / In Progress / Not Started / Locked)
//   - action button (Start / Resume / Replay / Unlock prereq)
//
// Clicking Start/Resume/Replay opens the tutorial's modal (via
// TutorialHostProvider's context). Because quickstart is mandatory
// on the profile page, most users reach this page only after it's
// already complete.

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Check, Lock, Play, RefreshCw, GraduationCap } from "lucide-react";
import { TutorialHostProvider, useTutorialHost } from "@/components/tutorials/tutorial-host";
import {
  TUTORIAL_REGISTRY,
  prerequisitesMet,
  type TutorialDefinition,
} from "@/components/tutorials/registry";
import { getBootstrap, type TutorialId } from "@/lib/api";

export default function TutorialsPage() {
  const { getToken } = useAuth();
  const router = useRouter();
  const [profileId, setProfileId] = useState<string | null>(null);
  const [profileLoaded, setProfileLoaded] = useState(false);

  useEffect(() => {
    (async () => {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await getBootstrap(token);
        setProfileId(res.data.default_profile_id || res.data.last_profile_id || null);
      } catch { /* silent */ }
      finally {
        setProfileLoaded(true);
      }
    })();
  }, [getToken]);

  if (!profileLoaded) {
    return <div style={{ padding: 32, color: "var(--dmuted)" }}>Loading…</div>;
  }

  // If the user has no profile yet (shouldn't happen post-onboarding,
  // but defensive), bounce them to /projects.
  if (!profileId) {
    router.replace("/projects");
    return null;
  }

  return (
    <TutorialHostProvider profileId={profileId}>
      <TutorialsView />
    </TutorialHostProvider>
  );
}

function TutorialsView() {
  const { state, startTutorial } = useTutorialHost();

  if (!state) {
    return <div style={{ padding: 32, color: "var(--dmuted)" }}>Loading tutorials…</div>;
  }

  return (
    <div style={{ maxWidth: 720 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 8 }}>
        <GraduationCap style={{ width: 24, height: 24, color: "var(--daccent)" }} strokeWidth={1.75} />
        <div className="dt-page-title">Tutorials</div>
      </div>
      <div className="dt-body" style={{ color: "var(--dmuted)", marginBottom: 28 }}>
        Hands-on walkthroughs to help you get the most out of UniPost.
      </div>

      <div style={{ display: "grid", gap: 12 }}>
        {TUTORIAL_REGISTRY.map((t) => (
          <TutorialCard
            key={t.id}
            tutorial={t}
            completed={state.completedIds.has(t.id)}
            locked={!prerequisitesMet(t, state.completedIds)}
            onStart={() => startTutorial(t.id)}
          />
        ))}
      </div>
    </div>
  );
}

function TutorialCard({
  tutorial,
  completed,
  locked,
  onStart,
}: {
  tutorial: TutorialDefinition;
  completed: boolean;
  locked: boolean;
  onStart: () => void;
}) {
  const badge = completed
    ? { label: "Completed", bg: "rgba(16,185,129,.10)", border: "rgba(16,185,129,.22)", color: "var(--daccent)" }
    : locked
      ? { label: "Locked", bg: "rgba(255,255,255,.04)", border: "var(--dborder)", color: "var(--dmuted2)" }
      : tutorial.required
        ? { label: "Required", bg: "rgba(251,146,60,.10)", border: "rgba(251,146,60,.25)", color: "#fb923c" }
        : { label: "Optional", bg: "rgba(255,255,255,.03)", border: "var(--dborder)", color: "var(--dmuted)" };

  const ctaLabel = completed ? "Replay" : locked ? "Locked" : "Start";
  const CtaIcon = completed ? RefreshCw : locked ? Lock : Play;

  return (
    <div
      style={{
        padding: 18,
        borderRadius: 12,
        border: "1px solid var(--dborder)",
        background: "var(--surface)",
        display: "flex", alignItems: "flex-start", gap: 16,
        opacity: locked ? 0.6 : 1,
      }}
    >
      <div style={{
        width: 40, height: 40, flexShrink: 0,
        borderRadius: 10,
        display: "flex", alignItems: "center", justifyContent: "center",
        background: completed ? "rgba(16,185,129,.10)" : "var(--sidebar-accent)",
        color: completed ? "var(--daccent)" : "var(--dmuted)",
      }}>
        {completed ? (
          <Check style={{ width: 18, height: 18 }} strokeWidth={2.5} />
        ) : locked ? (
          <Lock style={{ width: 16, height: 16 }} />
        ) : (
          <GraduationCap style={{ width: 20, height: 20 }} strokeWidth={1.75} />
        )}
      </div>

      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 4 }}>
          <div className="dt-body" style={{ fontWeight: 600, color: "var(--dtext)" }}>
            {tutorial.title}
          </div>
          <span className="dt-mono" style={{
            fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase",
            padding: "2px 8px", borderRadius: 4,
            background: badge.bg, border: `1px solid ${badge.border}`, color: badge.color,
          }}>
            {badge.label}
          </span>
        </div>
        <div className="dt-body-sm" style={{ color: "var(--dmuted)", marginBottom: 10 }}>
          {tutorial.description}
        </div>
        {locked && tutorial.prerequisites && (
          <div className="dt-body-sm" style={{ color: "var(--dmuted2)", fontSize: 12 }}>
            Complete {tutorial.prerequisites.map((id) => {
              const prereq = TUTORIAL_REGISTRY.find((t) => t.id === id as TutorialId);
              return prereq?.title || id;
            }).join(", ")} first.
          </div>
        )}
      </div>

      <button
        type="button"
        onClick={onStart}
        disabled={locked}
        className="dt-body-sm"
        style={{
          padding: "8px 14px", borderRadius: 8,
          border: "none",
          background: completed ? "transparent" : "var(--daccent)",
          color: completed ? "var(--dtext)" : "var(--primary-foreground)",
          cursor: locked ? "not-allowed" : "pointer",
          fontFamily: "inherit", fontWeight: 600,
          display: "inline-flex", alignItems: "center", gap: 6,
          flexShrink: 0,
          ...(completed && { border: "1px solid var(--dborder)" }),
        }}
      >
        <CtaIcon style={{ width: 13, height: 13 }} />
        {ctaLabel}
      </button>
    </div>
  );
}
