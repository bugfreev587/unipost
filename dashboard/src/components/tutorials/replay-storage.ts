// Per-tab storage for an in-progress tutorial session.
//
// The modal-driven flow may navigate the browser away (OAuth callback
// for the Connect step) and lose all React state. We mirror just
// enough state into sessionStorage so the host can restore the modal —
// and the OAuth-callback page can detect an in-progress quickstart and
// bounce the user back to where the host is mounted.

import type { TutorialId, TutorialsCounts } from "@/lib/api";

const REPLAY_STORAGE_KEY = "unipost.tutorial_replay";

export type StoredReplay = {
  id: TutorialId;
  replay: boolean;
  countsSnapshot?: TutorialsCounts;
  selectedAccountId?: string;
};

export function readStoredReplay(): StoredReplay | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.sessionStorage.getItem(REPLAY_STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Partial<StoredReplay>;
    if (!parsed.id || typeof parsed.replay !== "boolean") return null;
    if (parsed.replay && !parsed.countsSnapshot) return null;
    return parsed as StoredReplay;
  } catch {
    return null;
  }
}

export function writeStoredReplay(value: StoredReplay): void {
  if (typeof window === "undefined") return;
  try {
    window.sessionStorage.setItem(REPLAY_STORAGE_KEY, JSON.stringify(value));
  } catch {
    /* silent — storage unavailable */
  }
}

export function clearStoredReplay(): void {
  if (typeof window === "undefined") return;
  try {
    window.sessionStorage.removeItem(REPLAY_STORAGE_KEY);
  } catch {
    /* silent */
  }
}
