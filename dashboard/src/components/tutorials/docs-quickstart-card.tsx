"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import type { CSSProperties } from "react";
import { useAuth } from "@clerk/nextjs";
import { CheckCircle2, Lock, Play, RefreshCw, Sparkles } from "lucide-react";
import { getBootstrap, type TutorialId } from "@/lib/api";
import { TutorialHostProvider, useTutorialHost } from "./tutorial-host";
import { getTutorial, prerequisitesMet, stepCompleted } from "./registry";

const AUTH_LOAD_TIMEOUT_MS = 4500;
const BOOTSTRAP_TIMEOUT_MS = 8000;

const docsButtonLinkStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 8,
  borderRadius: 7,
  border: "1px solid color-mix(in srgb, var(--docs-accent) 22%, var(--docs-border))",
  background: "color-mix(in srgb, var(--docs-accent) 7%, var(--docs-bg-elevated))",
  color: "var(--docs-text)",
  padding: "9px 13px",
  fontWeight: 600,
  textDecoration: "none",
  fontSize: 13,
};

function withTimeout<T>(promise: Promise<T>, timeoutMs: number, label: string): Promise<T> {
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(() => {
      reject(new Error(`${label} timed out`));
    }, timeoutMs);

    promise.then(
      (value) => {
        window.clearTimeout(timer);
        resolve(value);
      },
      (error) => {
        window.clearTimeout(timer);
        reject(error);
      },
    );
  });
}

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
  const [authLoadTimedOut, setAuthLoadTimedOut] = useState(false);
  const [retryCount, setRetryCount] = useState(0);

  useEffect(() => {
    if (isLoaded) {
      setAuthLoadTimedOut(false);
      return;
    }

    const timer = window.setTimeout(() => {
      setAuthLoadTimedOut(true);
      setLoading(false);
    }, AUTH_LOAD_TIMEOUT_MS);

    return () => window.clearTimeout(timer);
  }, [isLoaded, retryCount]);

  useEffect(() => {
    if (!isLoaded) return;

    setAuthLoadTimedOut(false);
    setProfileId(null);

    if (!isSignedIn) {
      setLoading(false);
      return;
    }

    let cancelled = false;
    setLoading(true);

    (async () => {
      try {
        const token = await withTimeout(getToken(), BOOTSTRAP_TIMEOUT_MS, "Clerk token");
        if (cancelled) return;
        if (!token) {
          setProfileId(null);
          setLoading(false);
          return;
        }
        const res = await withTimeout(getBootstrap(token), BOOTSTRAP_TIMEOUT_MS, "Dashboard bootstrap");
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
  }, [getToken, isLoaded, isSignedIn, retryCount]);

  const tutorial = getTutorial(tutorialId);
  if (!tutorial) return null;

  if (!isLoaded && authLoadTimedOut) {
    return (
      <div className="docs-callout" style={{ marginBottom: 24 }}>
        <strong>Interactive quickstart</strong>
        <div style={{ marginTop: 6 }}>
          We could not confirm your dashboard session from this page.
        </div>
        <div style={{ display: "flex", flexWrap: "wrap", gap: 10, marginTop: 14 }}>
          <button
            type="button"
            style={{ ...docsButtonLinkStyle, cursor: "pointer", fontFamily: "inherit" }}
            onClick={() => {
              setLoading(true);
              setAuthLoadTimedOut(false);
              setRetryCount((value) => value + 1);
            }}
          >
            Retry quickstart
          </button>
          <Link href="/projects" style={docsButtonLinkStyle}>
            Open dashboard
          </Link>
        </div>
      </div>
    );
  }

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
        marginBottom: 20,
        overflow: "hidden",
        borderRadius: 8,
        border: locked
          ? "1px solid var(--docs-border)"
          : "1px solid color-mix(in srgb, var(--docs-accent) 28%, var(--docs-border))",
        background: locked
          ? "var(--docs-bg-elevated)"
          : "color-mix(in srgb, var(--docs-accent) 3%, var(--docs-bg-elevated))",
        boxShadow: "none",
      }}
    >
      <div
        style={{
          padding: 16,
          display: "flex",
          alignItems: "stretch",
          justifyContent: "space-between",
          gap: 16,
          flexWrap: "wrap",
        }}
      >
        <div style={{ minWidth: 0, flex: "1 1 320px" }}>
          <div
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 6,
              marginBottom: 10,
              padding: "4px 9px",
              borderRadius: 6,
              background: "var(--docs-bg-elevated)",
              border: "1px solid color-mix(in srgb, var(--docs-accent) 14%, var(--docs-border))",
              color: "var(--docs-text)",
              fontSize: 10.5,
              fontWeight: 800,
              letterSpacing: "0.1em",
              textTransform: "uppercase",
            }}
          >
            <Sparkles style={{ width: 12, height: 12, color: "var(--docs-accent)" }} />
            Interactive quickstart
          </div>

          <div style={{ display: "flex", alignItems: "flex-start", gap: 12 }}>
            <div
              style={{
                width: 40,
                height: 40,
                flexShrink: 0,
                borderRadius: 8,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                background: locked
                  ? "var(--docs-bg-muted)"
                  : "linear-gradient(135deg, color-mix(in srgb, var(--docs-accent) 18%, var(--docs-bg-elevated) 82%), color-mix(in srgb, var(--docs-accent) 10%, var(--docs-bg-elevated) 90%))",
                border: locked
                  ? "1px solid var(--docs-border)"
                  : "1px solid color-mix(in srgb, var(--docs-accent) 22%, var(--docs-border))",
              }}
            >
              <StateIcon
                style={{
                  width: 18,
                  height: 18,
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
                  gap: 6,
                  marginBottom: 6,
                  padding: "2px 7px",
                  borderRadius: 5,
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
                  fontSize: 11,
                  fontWeight: 700,
                  letterSpacing: "0.04em",
                  textTransform: "uppercase",
                }}
              >
                {stateLabel}
              </div>
              <div
                style={{
                  fontSize: 18,
                  lineHeight: 1.25,
                  fontWeight: 740,
                  color: "var(--docs-text)",
                  marginBottom: 6,
                  letterSpacing: "-0.015em",
                }}
              >
                {tutorial.title}
              </div>
              <div
                style={{
                  color: "var(--docs-text-soft)",
                  lineHeight: 1.55,
                  marginBottom: 10,
                  fontSize: 13.5,
                  maxWidth: 760,
                }}
              >
                {tutorial.description}
              </div>
              <div
                style={{
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 8,
                  padding: "5px 9px",
                  borderRadius: 7,
                  background: "var(--docs-bg-elevated)",
                  border: "1px dashed color-mix(in srgb, var(--docs-accent) 26%, var(--docs-border))",
                  color: "var(--docs-text)",
                  fontSize: 12.5,
                  fontWeight: 600,
                }}
              >
                <span style={{ color: "var(--docs-text-muted)", fontWeight: 700, textTransform: "uppercase", letterSpacing: "0.06em", fontSize: 10.5 }}>
                  Progress
                </span>
                <span>{completedSteps} / {tutorial.steps.length} steps</span>
              </div>
            </div>
          </div>
        </div>

        <div
          style={{
            flex: "1 1 260px",
            maxWidth: 360,
            alignSelf: "stretch",
            display: "flex",
            flexDirection: "column",
            justifyContent: "space-between",
            gap: 10,
            padding: 13,
            borderRadius: 8,
            border: "1px solid color-mix(in srgb, var(--docs-accent) 16%, var(--docs-border))",
            background: "var(--docs-bg-elevated)",
          }}
        >
          <div
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 6,
              color: "var(--docs-text-muted)",
              fontSize: 10.5,
              fontWeight: 800,
              letterSpacing: "0.12em",
              textTransform: "uppercase",
            }}
          >
            <Play style={{ width: 12, height: 12, color: "var(--docs-accent)" }} />
            Launch module
          </div>
          <div style={{ color: "var(--docs-text)", fontSize: 14, fontWeight: 700, lineHeight: 1.4 }}>
            Open the live quickstart and complete it step by step inside your workspace.
          </div>
          {locked && fallbackHref ? (
            <Link href={fallbackHref} style={{ ...docsButtonLinkStyle, width: "100%", justifyContent: "center" }}>
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
                borderRadius: 7,
                border: "1px solid color-mix(in srgb, var(--docs-accent) 32%, var(--docs-border))",
                background: "var(--docs-accent)",
                color: "white",
                padding: "9px 13px",
                fontWeight: 700,
                cursor: "pointer",
                fontFamily: "inherit",
                fontSize: 13,
              }}
            >
              <CtaIcon style={{ width: 15, height: 15 }} />
              {ctaLabel}
            </button>
          )}
          <div
            style={{
              color: "var(--docs-text-soft)",
              fontSize: 12.5,
              lineHeight: 1.55,
            }}
          >
            This is a live, playable quickstart. It opens the real walkthrough modal, keeps progress, and can be replayed anytime.
          </div>
        </div>
      </div>

      {locked && fallbackHref && (
        <div style={{ padding: "0 18px 14px", color: "var(--docs-text-muted)", fontSize: 13 }}>
          This quickstart unlocks after you complete the Dashboard Quickstart.
        </div>
      )}

      {!locked && tutorialId === "post_with_api" && (
        <div style={{ padding: "0 18px 14px", color: "var(--docs-text-muted)", fontSize: 13 }}>
          Quickstart Mode creates a key, picks a real connected account, and sends a live test post.
        </div>
      )}
      {!locked && tutorialId === "quickstart" && (
        <div style={{ padding: "0 18px 14px", color: "var(--docs-text-muted)", fontSize: 13 }}>
          The Dashboard Quickstart walks through connecting an account and publishing your first post from the UI.
        </div>
      )}
    </div>
  );
}
