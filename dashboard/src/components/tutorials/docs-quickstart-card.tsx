"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import type { CSSProperties } from "react";
import { useAuth } from "@clerk/nextjs";
import { CheckCircle2, Lock, Play, RefreshCw, Sparkles } from "lucide-react";
import { getBootstrap, type TutorialId } from "@/lib/api";
import { TutorialHostProvider, useTutorialHost } from "./tutorial-host";
import { getTutorial, prerequisitesMet, stepCompleted } from "./registry";

const docsButtonLinkStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 8,
  borderRadius: 12,
  border: "1px solid color-mix(in srgb, var(--docs-accent) 22%, var(--docs-border))",
  background: "color-mix(in srgb, var(--docs-accent) 8%, var(--docs-surface))",
  color: "var(--docs-text)",
  padding: "10px 14px",
  fontWeight: 600,
  textDecoration: "none",
  fontSize: 14,
};

export function DocsQuickstartCard({
  tutorialId,
  fallbackHref,
}: {
  tutorialId: TutorialId;
  fallbackHref?: string;
}) {
  const { getToken, isLoaded, isSignedIn } = useAuth();
  const [profileId, setProfileId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!isLoaded) return;
    if (!isSignedIn) {
      setLoading(false);
      return;
    }

    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token || cancelled) return;
        const res = await getBootstrap(token);
        if (cancelled) return;
        setProfileId(res.data.last_profile_id ?? res.data.default_profile_id ?? null);
      } catch {
        if (!cancelled) setProfileId(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [getToken, isLoaded, isSignedIn]);

  const tutorial = getTutorial(tutorialId);
  if (!tutorial) return null;

  if (!isLoaded || loading) {
    return (
      <div className="docs-callout" style={{ marginBottom: 24 }}>
        <strong>Interactive quickstart</strong>
        <div style={{ marginTop: 6 }}>Loading your dashboard state…</div>
      </div>
    );
  }

  if (!isSignedIn) {
    return (
      <div className="docs-callout" style={{ marginBottom: 24 }}>
        <strong>Interactive quickstart</strong>
        <div style={{ marginTop: 6 }}>
          Sign in to run this quickstart inside your UniPost workspace.
        </div>
        <div style={{ marginTop: 14 }}>
          <Link href="/projects" style={docsButtonLinkStyle}>
            Open dashboard
          </Link>
        </div>
      </div>
    );
  }

  if (!profileId) {
    return (
      <div className="docs-callout" style={{ marginBottom: 24 }}>
        <strong>Interactive quickstart</strong>
        <div style={{ marginTop: 6 }}>
          Your workspace is still loading. Open the dashboard once, then come back here.
        </div>
        <div style={{ marginTop: 14 }}>
          <Link href="/projects" style={docsButtonLinkStyle}>
            Go to dashboard
          </Link>
        </div>
      </div>
    );
  }

  return (
    <TutorialHostProvider profileId={profileId}>
      <DocsQuickstartCardInner tutorialId={tutorialId} fallbackHref={fallbackHref} />
    </TutorialHostProvider>
  );
}

function DocsQuickstartCardInner({
  tutorialId,
  fallbackHref,
}: {
  tutorialId: TutorialId;
  fallbackHref?: string;
}) {
  const { state, startTutorial } = useTutorialHost();
  const tutorial = getTutorial(tutorialId);
  const tutorialState = state?.byId.get(tutorialId);

  const completed = !!tutorialState?.completed_at;
  const locked = !!(tutorial && state && !prerequisitesMet(tutorial, state.completedIds));
  const completedSteps = useMemo(() => {
    if (!tutorial || !state) return 0;
    if (completed) return tutorial.steps.length;
    return tutorial.steps.filter((step) => stepCompleted(step.signal, state.counts)).length;
  }, [completed, state, tutorial]);

  if (!tutorial) return null;

  const ctaLabel = completed ? "Replay interactive quickstart" : completedSteps > 0 ? "Resume interactive quickstart" : "Start interactive quickstart";
  const CtaIcon = completed ? RefreshCw : Play;
  const StateIcon = completed ? CheckCircle2 : locked ? Lock : Sparkles;
  const stateLabel = completed ? "Completed" : locked ? "Locked" : completedSteps > 0 ? "In progress" : "Interactive";

  return (
    <div
      style={{
        marginBottom: 24,
        borderRadius: 28,
        border: locked
          ? "1px solid var(--docs-border)"
          : "1px solid color-mix(in srgb, var(--docs-accent) 28%, var(--docs-border))",
        background: locked
          ? "var(--docs-bg-elevated)"
          : "linear-gradient(135deg, color-mix(in srgb, var(--docs-accent) 13%, var(--docs-bg-elevated) 87%), var(--docs-bg-elevated))",
        boxShadow: locked
          ? "0 10px 30px color-mix(in srgb, var(--shadow-color, rgba(15,23,42,1)) 6%, transparent)"
          : "0 18px 50px color-mix(in srgb, var(--shadow-color, rgba(15,23,42,1)) 10%, transparent)",
        overflow: "hidden",
      }}
    >
      <div
        style={{
          padding: 24,
          display: "flex",
          alignItems: "stretch",
          justifyContent: "space-between",
          gap: 24,
          flexWrap: "wrap",
        }}
      >
        <div style={{ minWidth: 0, flex: "1 1 520px" }}>
          <div
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 8,
              marginBottom: 14,
              padding: "6px 12px",
              borderRadius: 999,
              background: "var(--docs-bg-elevated)",
              border: "1px solid color-mix(in srgb, var(--docs-accent) 14%, var(--docs-border))",
              color: "var(--docs-text)",
              fontSize: 11,
              fontWeight: 800,
              letterSpacing: "0.1em",
              textTransform: "uppercase",
              boxShadow: "0 6px 18px color-mix(in srgb, var(--shadow-color, rgba(15,23,42,1)) 6%, transparent)",
            }}
          >
            <Sparkles style={{ width: 13, height: 13, color: "var(--docs-accent)" }} />
            Interactive quickstart
          </div>

          <div style={{ display: "flex", alignItems: "flex-start", gap: 16 }}>
            <div
              style={{
                width: 58,
                height: 58,
                flexShrink: 0,
                borderRadius: 18,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                background: locked
                  ? "var(--docs-bg-muted)"
                  : "linear-gradient(135deg, color-mix(in srgb, var(--docs-accent) 18%, var(--docs-bg-elevated) 82%), color-mix(in srgb, var(--docs-accent) 10%, var(--docs-bg-elevated) 90%))",
                border: locked
                  ? "1px solid var(--docs-border)"
                  : "1px solid color-mix(in srgb, var(--docs-accent) 22%, var(--docs-border))",
                boxShadow: locked
                  ? "none"
                  : "0 12px 28px color-mix(in srgb, var(--shadow-color, rgba(15,23,42,1)) 8%, transparent)",
              }}
            >
              <StateIcon
                style={{
                  width: 26,
                  height: 26,
                  color: completed
                    ? "#10b981"
                    : locked
                      ? "var(--docs-text-muted)"
                      : "var(--docs-accent)",
                }}
              />
            </div>

            <div style={{ minWidth: 0, flex: 1 }}>
              <div
                style={{
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 8,
                  marginBottom: 10,
                  padding: "4px 10px",
                  borderRadius: 999,
                  background: completed
                    ? "color-mix(in srgb, #10b981 14%, var(--docs-bg-elevated))"
                    : locked
                      ? "var(--docs-bg-muted)"
                      : "color-mix(in srgb, #3b82f6 14%, var(--docs-bg-elevated))",
                  border: completed
                    ? "1px solid rgba(16,185,129,.18)"
                    : locked
                      ? "1px solid var(--docs-border)"
                      : "1px solid rgba(59,130,246,.18)",
                  color: completed ? "#10b981" : locked ? "var(--docs-text-muted)" : "#3b82f6",
                  fontSize: 12,
                  fontWeight: 700,
                  letterSpacing: "0.06em",
                  textTransform: "uppercase",
                }}
              >
                {stateLabel}
              </div>
              <div
                style={{
                  fontSize: 32,
                  lineHeight: 1.05,
                  fontWeight: 800,
                  color: "var(--docs-text)",
                  marginBottom: 10,
                  letterSpacing: "-0.03em",
                }}
              >
                {tutorial.title}
              </div>
              <div
                style={{
                  color: "var(--docs-text-soft)",
                  lineHeight: 1.7,
                  marginBottom: 14,
                  fontSize: 18,
                  maxWidth: 760,
                }}
              >
                {tutorial.description}
              </div>
              <div
                style={{
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 10,
                  padding: "8px 12px",
                  borderRadius: 14,
                  background: "var(--docs-bg-elevated)",
                  border: "1px dashed color-mix(in srgb, var(--docs-accent) 26%, var(--docs-border))",
                  color: "var(--docs-text)",
                  fontSize: 14,
                  fontWeight: 600,
                }}
              >
                <span style={{ color: "var(--docs-text-muted)", fontWeight: 700, textTransform: "uppercase", letterSpacing: "0.06em", fontSize: 11 }}>
                  Progress
                </span>
                <span>{completedSteps} / {tutorial.steps.length} steps</span>
              </div>
            </div>
          </div>
        </div>

        <div
          style={{
            flex: "0 0 380px",
            minWidth: 300,
            alignSelf: "stretch",
            display: "flex",
            flexDirection: "column",
            justifyContent: "space-between",
            gap: 16,
            padding: 20,
            borderRadius: 24,
            border: "1px solid color-mix(in srgb, var(--docs-accent) 16%, var(--docs-border))",
            background: "var(--docs-bg-elevated)",
            boxShadow: "inset 0 1px 0 color-mix(in srgb, white 6%, transparent)",
          }}
        >
          <div
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 8,
              padding: "4px 0",
              color: "var(--docs-text-muted)",
              fontSize: 11,
              fontWeight: 800,
              letterSpacing: "0.12em",
              textTransform: "uppercase",
            }}
          >
            <Play style={{ width: 13, height: 13, color: "var(--docs-accent)" }} />
            Launch module
          </div>
          <div style={{ color: "var(--docs-text)", fontSize: 18, fontWeight: 700, lineHeight: 1.35 }}>
            Open the live quickstart and complete it step by step inside your workspace.
          </div>
          {locked && fallbackHref ? (
            <Link href={fallbackHref} style={{ ...docsButtonLinkStyle, width: "100%", justifyContent: "center", marginTop: 4 }}>
              Complete prerequisite first
            </Link>
          ) : (
            <button
              type="button"
              onClick={() => startTutorial(tutorialId)}
              style={{
                display: "inline-flex",
                alignItems: "center",
                justifyContent: "center",
                gap: 8,
                width: "100%",
                borderRadius: 16,
                border: "1px solid color-mix(in srgb, var(--docs-accent) 32%, var(--docs-border))",
                background: "var(--docs-accent)",
                color: "white",
                padding: "16px 18px",
                fontWeight: 700,
                cursor: "pointer",
                fontFamily: "inherit",
                fontSize: 17,
                boxShadow: "0 14px 30px color-mix(in srgb, var(--docs-accent) 22%, transparent)",
              }}
            >
              <CtaIcon style={{ width: 18, height: 18 }} />
              {ctaLabel}
            </button>
          )}
          <div
            style={{
              paddingTop: 4,
              color: "var(--docs-text-soft)",
              fontSize: 15,
              lineHeight: 1.65,
            }}
          >
            This is a live, playable quickstart. It opens the real walkthrough modal, keeps progress, and can be replayed anytime.
          </div>
        </div>
      </div>

      {locked && fallbackHref && (
        <div style={{ padding: "0 24px 24px", color: "var(--docs-text-muted)", fontSize: 14 }}>
          This quickstart unlocks after you complete the Dashboard Quickstart.
        </div>
      )}

      {!locked && tutorialId === "post_with_api" && (
        <div style={{ padding: "0 24px 24px", color: "var(--docs-text-muted)", fontSize: 14 }}>
          The API Quickstart creates a key, picks a real connected account, and sends a live test post.
        </div>
      )}
      {!locked && tutorialId === "quickstart" && (
        <div style={{ padding: "0 24px 24px", color: "var(--docs-text-muted)", fontSize: 14 }}>
          The Dashboard Quickstart walks through connecting an account and publishing your first post from the UI.
        </div>
      )}
    </div>
  );
}
