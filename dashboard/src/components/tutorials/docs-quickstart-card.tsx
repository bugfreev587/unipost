"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import type { CSSProperties } from "react";
import { useAuth } from "@clerk/nextjs";
import { CheckCircle2, Lock, Play, RefreshCw } from "lucide-react";
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

  return (
    <div
      className="docs-callout"
      style={{
        marginBottom: 24,
        border: "1px solid var(--docs-border)",
        background: "color-mix(in srgb, var(--docs-surface) 84%, var(--docs-accent) 16%)",
      }}
    >
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 16, flexWrap: "wrap" }}>
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
                ? "rgba(16,185,129,.10)"
                : locked
                  ? "rgba(255,255,255,.05)"
                  : "rgba(59,130,246,.10)",
              border: completed
                ? "1px solid rgba(16,185,129,.18)"
                : locked
                  ? "1px solid var(--docs-border)"
                  : "1px solid rgba(59,130,246,.18)",
              color: completed ? "#10b981" : locked ? "var(--docs-text-muted)" : "#60a5fa",
              fontSize: 12,
              fontWeight: 700,
              letterSpacing: "0.04em",
              textTransform: "uppercase",
            }}
          >
            {completed ? <CheckCircle2 style={{ width: 13, height: 13 }} /> : locked ? <Lock style={{ width: 13, height: 13 }} /> : <Play style={{ width: 13, height: 13 }} />}
            {completed ? "Completed" : locked ? "Locked" : completedSteps > 0 ? "In progress" : "Interactive"}
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: "var(--docs-text)", marginBottom: 8 }}>
            {tutorial.title}
          </div>
          <div style={{ color: "var(--docs-text-soft)", lineHeight: 1.7, marginBottom: 10 }}>
            {tutorial.description}
          </div>
          <div style={{ color: "var(--docs-text-muted)", fontSize: 14 }}>
            Progress: {completedSteps} / {tutorial.steps.length} steps
          </div>
        </div>

        <div style={{ display: "flex", flexDirection: "column", alignItems: "flex-start", gap: 10 }}>
          {locked && fallbackHref ? (
            <Link href={fallbackHref} style={docsButtonLinkStyle}>
              Complete prerequisite first
            </Link>
          ) : (
            <button
              type="button"
              onClick={() => startTutorial(tutorialId)}
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: 8,
                borderRadius: 12,
                border: "1px solid color-mix(in srgb, var(--docs-accent) 32%, var(--docs-border))",
                background: "var(--docs-accent)",
                color: "white",
                padding: "10px 14px",
                fontWeight: 600,
                cursor: "pointer",
                fontFamily: "inherit",
                fontSize: 14,
              }}
            >
              <CtaIcon style={{ width: 14, height: 14 }} />
              {ctaLabel}
            </button>
          )}
          <div style={{ color: "var(--docs-text-muted)", fontSize: 13, maxWidth: 280 }}>
            Runs the live quickstart flow inside your workspace without leaving this page.
          </div>
        </div>
      </div>

      {locked && fallbackHref && (
        <div style={{ marginTop: 14, color: "var(--docs-text-muted)", fontSize: 14 }}>
          This quickstart unlocks after you complete the Dashboard Quickstart.
        </div>
      )}

      {!locked && tutorialId === "post_with_api" && (
        <div style={{ marginTop: 14, color: "var(--docs-text-muted)", fontSize: 14 }}>
          The API Quickstart creates a key, picks a real connected account, and sends a live test post.
        </div>
      )}
      {!locked && tutorialId === "quickstart" && (
        <div style={{ marginTop: 14, color: "var(--docs-text-muted)", fontSize: 14 }}>
          The Dashboard Quickstart walks through connecting an account and publishing your first post from the UI.
        </div>
      )}
    </div>
  );
}
