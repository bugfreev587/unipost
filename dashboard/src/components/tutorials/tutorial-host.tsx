"use client";

// TutorialHost — single entry point that the dashboard shell / profile
// page mounts. Responsible for:
//   - Loading tutorial state + counts (GET /v1/me/tutorials)
//   - Deciding which tutorial (if any) should auto-open
//   - Orchestrating completion + celebration transitions
//   - Exposing imperative controls via context so other parts of the UI
//     (the /tutorials page, the sidebar "Start tutorial" CTA) can open
//     a specific tutorial on demand
//
// Auto-open behavior:
//   - Mandatory tutorials (required=true) auto-pop on the profile page
//     until completed. Dismissing temporarily hides the modal until the
//     next profile visit (the host clears dismissed_at on mount).
//   - Optional tutorials only open when explicitly started.

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useAuth } from "@clerk/nextjs";
import { useRouter } from "next/navigation";
import {
  getTutorials,
  completeTutorial,
  dismissTutorial,
  reopenTutorial,
  type TutorialId,
  type TutorialsCounts,
  type TutorialState,
} from "@/lib/api";
import { track } from "@/lib/analytics";
import { TutorialShell } from "./tutorial-shell";
import { CompletionScreen } from "./completion-screen";
import {
  TUTORIAL_REGISTRY,
  getTutorial,
  prerequisitesMet,
  stepCompleted,
  type TutorialContext,
} from "./registry";

// ── Context ──────────────────────────────────────────────────────────

type TutorialHostControls = {
  // Start a tutorial (opens the modal immediately).
  startTutorial: (id: TutorialId) => void;
  // Force a state refresh (after the user completes a step via the UI).
  refresh: () => void;
  // Current state, exposed so /tutorials can render the completion grid
  // without a duplicate fetch.
  state: TutorialHostState | null;
};

type TutorialHostState = {
  loaded: boolean;
  counts: TutorialsCounts;
  byId: Map<TutorialId, TutorialState>;
  completedIds: Set<TutorialId>;
};

const TutorialHostContext = createContext<TutorialHostControls | null>(null);

export function useTutorialHost(): TutorialHostControls {
  const ctx = useContext(TutorialHostContext);
  if (!ctx) throw new Error("useTutorialHost must be used inside <TutorialHostProvider>");
  return ctx;
}

// ── Provider ─────────────────────────────────────────────────────────

export function TutorialHostProvider({
  profileId,
  children,
}: {
  profileId: string | null;
  children: React.ReactNode;
}) {
  const { getToken } = useAuth();
  const router = useRouter();
  const [state, setState] = useState<TutorialHostState | null>(null);

  // Which tutorial is currently open (null = no modal shown).
  const [activeTutorialId, setActiveTutorialId] = useState<TutorialId | null>(null);
  // When transitioning from tutorial → celebration, we keep showing the
  // celebration until the user clicks Done.
  const [celebratingId, setCelebratingId] = useState<TutorialId | null>(null);
  // Track which completions we've already celebrated in this session to
  // avoid re-firing on every refresh.
  const celebratedRef = useRef<Set<TutorialId>>(new Set());
  // Track previous step-completion states so we can fire analytics on
  // transitions.
  const prevStepsRef = useRef<Map<string, boolean>>(new Map());
  // Avoid auto-popping on profile page more than once per mount.
  const autoOpenedRef = useRef(false);

  const load = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await getTutorials(token);
      const byId = new Map<TutorialId, TutorialState>();
      const completedIds = new Set<TutorialId>();
      for (const t of res.data.tutorials) {
        byId.set(t.id, t);
        if (t.completed_at) completedIds.add(t.id);
      }

      // Detect step transitions (incomplete → complete) for analytics.
      for (const tut of TUTORIAL_REGISTRY) {
        for (const step of tut.steps) {
          const key = `${tut.id}:${step.id}`;
          const nowDone = stepCompleted(step.signal, res.data.counts);
          const prev = prevStepsRef.current.get(key);
          if (prev === false && nowDone) {
            track("tutorial_step_completed", {
              tutorial_id: tut.id,
              step_id: step.id,
            });
          }
          prevStepsRef.current.set(key, nowDone);
        }
      }

      setState({
        loaded: true,
        counts: res.data.counts,
        byId,
        completedIds,
      });
    } catch {
      /* silent — tutorials are non-critical */
    }
  }, [getToken]);

  useEffect(() => {
    load();
  }, [load]);

  // Re-poll on window focus so completing a step on another tab (or
  // coming back from a CTA page) reflects immediately.
  useEffect(() => {
    function onFocus() {
      load();
    }
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [load]);

  // Auto-open the mandatory tutorial on the profile page if it's not
  // complete. Clears any prior dismissal so the modal pops fresh.
  useEffect(() => {
    if (!state || !profileId || autoOpenedRef.current || activeTutorialId) return;
    if (celebratingId) return;

    const quickstart = getTutorial("quickstart");
    if (!quickstart) return;
    const quickstartState = state.byId.get("quickstart");
    if (quickstartState?.completed_at) return;

    // Clear dismissal on profile visit so the mandatory modal re-pops.
    (async () => {
      if (quickstartState?.dismissed_at) {
        try {
          const token = await getToken();
          if (token) await reopenTutorial(token, "quickstart");
        } catch { /* silent */ }
      }
      autoOpenedRef.current = true;
      setActiveTutorialId("quickstart");
      track("tutorial_opened", { tutorial_id: "quickstart", trigger: "auto" });
    })();
  }, [state, profileId, activeTutorialId, celebratingId, getToken]);

  // Auto-complete tutorials whose completeOn is "all_steps_done" when
  // all signals are met. Fires celebration on first completion per tab.
  useEffect(() => {
    if (!state) return;
    for (const tut of TUTORIAL_REGISTRY) {
      if (tut.completeOn !== "all_steps_done") continue;
      const already = state.byId.get(tut.id)?.completed_at;
      if (already) continue;
      const allDone = tut.steps.every((s) => stepCompleted(s.signal, state.counts));
      if (!allDone) continue;

      (async () => {
        try {
          const token = await getToken();
          if (!token) return;
          await completeTutorial(token, tut.id);
          if (!celebratedRef.current.has(tut.id)) {
            celebratedRef.current.add(tut.id);
            track("tutorial_completed", { tutorial_id: tut.id });
            setCelebratingId(tut.id);
            setActiveTutorialId(null);
          }
          load();
        } catch { /* silent */ }
      })();
    }
  }, [state, getToken, load]);

  // ── Controls ────────────────────────────────────────────────────────

  const startTutorial = useCallback(
    (id: TutorialId) => {
      setCelebratingId(null);
      setActiveTutorialId(id);
      track("tutorial_opened", { tutorial_id: id, trigger: "manual" });
    },
    [],
  );

  const handleClose = useCallback(
    async (id: TutorialId) => {
      setActiveTutorialId(null);
      track("tutorial_dismissed", { tutorial_id: id });
      try {
        const token = await getToken();
        if (token) await dismissTutorial(token, id);
      } catch { /* silent */ }
      // Refresh so dismissed_at is reflected in state for the
      // /tutorials page.
      load();
    },
    [getToken, load],
  );

  const handleManualComplete = useCallback(
    async (id: TutorialId) => {
      setActiveTutorialId(null);
      try {
        const token = await getToken();
        if (token) await completeTutorial(token, id);
        track("tutorial_completed", { tutorial_id: id });
      } catch { /* silent */ }
      await load();
      if (!celebratedRef.current.has(id)) {
        celebratedRef.current.add(id);
        setCelebratingId(id);
      }
    },
    [getToken, load],
  );

  const handleCelebrationDone = useCallback(() => {
    setCelebratingId(null);
    if (profileId) {
      router.push(`/projects/${profileId}/posts`);
    }
  }, [profileId, router]);

  const activeTutorial = activeTutorialId ? getTutorial(activeTutorialId) : undefined;
  const tutorialCtx: TutorialContext | null = useMemo(() => {
    if (!state || !profileId) return null;
    return { profileId, counts: state.counts };
  }, [state, profileId]);

  const controls: TutorialHostControls = {
    startTutorial,
    refresh: load,
    state,
  };

  // Gate rendering: only show modals if prerequisites are met.
  const canShowActive =
    activeTutorial &&
    tutorialCtx &&
    state &&
    prerequisitesMet(activeTutorial, state.completedIds);

  return (
    <TutorialHostContext.Provider value={controls}>
      {children}
      {canShowActive && activeTutorial && tutorialCtx && (
        <TutorialShell
          tutorial={activeTutorial}
          ctx={tutorialCtx}
          onRequestClose={() => handleClose(activeTutorial.id)}
          onRequestComplete={() => handleManualComplete(activeTutorial.id)}
        />
      )}
      {celebratingId && state && (
        <CompletionScreen
          completedTutorialId={celebratingId}
          completedIds={state.completedIds}
          onStartTutorial={(id) => {
            setCelebratingId(null);
            startTutorial(id);
          }}
          onDone={handleCelebrationDone}
        />
      )}
    </TutorialHostContext.Provider>
  );
}
