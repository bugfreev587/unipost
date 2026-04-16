// Tutorial registry — single source of truth for all tutorials.
//
// Each tutorial describes:
//   - id / title / description: metadata used by /tutorials and the
//     shell header
//   - required: true means the tutorial auto-pops on the profile page
//     until complete (resume from first incomplete step); false means
//     the user only sees it via the /tutorials page or a handoff CTA
//   - steps: declarative step list, each with a completion signal
//   - completeOn: "all_steps_done" auto-completes when every step's
//     signal is met; "done_button" requires an explicit Complete call
//     (used for tutorials whose final step is a one-shot UI action,
//     like sending a post from the code-block modal)
//   - handoff: tutorial to suggest after completion on the celebration
//     screen (optional)
//
// The runtime shell (tutorial-shell.tsx) renders the chrome; each
// tutorial provides its own body via the `render` function.

import type { TutorialId, TutorialsCounts } from "@/lib/api";
import type { ReactNode } from "react";

export type TutorialStepSignal =
  | { kind: "count"; name: keyof TutorialsCounts; threshold?: number }
  | { kind: "manual" };

export type TutorialStep = {
  id: string;
  title: string;
  description: string;
  hint?: string;
  ctaLabel?: string;
  ctaHref?: (ctx: TutorialContext) => string;
  signal: TutorialStepSignal;
};

export type TutorialContext = {
  profileId: string;
  counts: TutorialsCounts;
};

export type TutorialDefinition = {
  id: TutorialId;
  title: string;
  description: string;
  required: boolean;
  prerequisites?: TutorialId[];
  steps: TutorialStep[];
  completeOn: "all_steps_done" | "done_button";
  // Optional handoff on the celebration screen.
  handoff?: {
    tutorialId: TutorialId;
    prompt: string;
    ctaLabel: string;
  };
  // Optional custom body renderer. When provided, the shell uses this
  // instead of the default step-list UI (needed for post_with_api which
  // renders code blocks and a live Send button inside the modal).
  renderBody?: (props: TutorialBodyProps) => ReactNode;
};

export type TutorialBodyProps = {
  ctx: TutorialContext;
  steps: ReadonlyArray<TutorialStep & { completed: boolean }>;
  onRequestComplete: () => void;
  onRequestClose: () => void;
};

// Determines whether a step is complete given current counts.
export function stepCompleted(signal: TutorialStepSignal, counts: TutorialsCounts): boolean {
  if (signal.kind === "count") {
    const threshold = signal.threshold ?? 1;
    return counts[signal.name] >= threshold;
  }
  // "manual" — completion for these is tracked at the tutorial level,
  // not per step, so the step is considered done only when the
  // tutorial itself is complete. Step-list rendering for manual-step
  // tutorials is expected to be custom (renderBody).
  return false;
}

// Registry of all tutorials. Components import this list to render the
// /tutorials page; the shell uses it to resolve a tutorial by id.
//
// Quickstart is defined inline because its step list is trivial. The
// post_with_api definition pulls in a custom body from its own module
// to keep the code-block UI colocated.
export const TUTORIAL_REGISTRY: TutorialDefinition[] = [
  {
    id: "quickstart",
    title: "Quickstart",
    description: "Connect your first account and send your first post.",
    required: true,
    completeOn: "all_steps_done",
    steps: [
      {
        id: "connect_account",
        title: "Connect your first account",
        description: "Bluesky, LinkedIn, Instagram, and more.",
        hint: "~30 seconds",
        ctaLabel: "Connect account",
        ctaHref: ({ profileId }) =>
          `/projects/${profileId}/accounts?action=new&first=1`,
        signal: { kind: "count", name: "connected_accounts" },
      },
      {
        id: "send_post",
        title: "Send your first post",
        description: "Try it with any connected account.",
        ctaLabel: "Send test post",
        ctaHref: ({ profileId }) =>
          `/projects/${profileId}/posts?action=new&template=welcome`,
        signal: { kind: "count", name: "posts_sent" },
      },
    ],
    handoff: {
      tutorialId: "post_with_api",
      prompt: "Want to send a post via the API next?",
      ctaLabel: "Try API tutorial",
    },
  },
  {
    id: "post_with_api",
    title: "Send a post via the API",
    description: "Create an API key and post programmatically.",
    required: false,
    prerequisites: ["quickstart"],
    completeOn: "done_button",
    steps: [
      {
        id: "create_api_key",
        title: "Create an API key",
        description: "Used to authenticate requests from your app.",
        signal: { kind: "count", name: "api_keys" },
      },
      {
        id: "send_via_api",
        title: "Send a post via API",
        description: "Run the code below or copy it into your app.",
        signal: { kind: "manual" },
      },
    ],
    // renderBody is attached at registration time below to avoid a
    // circular import (post-with-api/body.tsx imports TutorialBodyProps
    // from this file).
  },
];

export function getTutorial(id: TutorialId): TutorialDefinition | undefined {
  return TUTORIAL_REGISTRY.find((t) => t.id === id);
}

// Resolves whether a tutorial's prerequisites are satisfied.
export function prerequisitesMet(
  tutorial: TutorialDefinition,
  completedIds: Set<TutorialId>
): boolean {
  if (!tutorial.prerequisites || tutorial.prerequisites.length === 0) return true;
  return tutorial.prerequisites.every((id) => completedIds.has(id));
}
