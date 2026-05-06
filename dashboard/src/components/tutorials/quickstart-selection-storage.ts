"use client";

const QUICKSTART_SELECTION_STORAGE_KEY = "unipost.quickstart_selected_account";

export function readStoredQuickstartSelectedAccountId(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return window.sessionStorage.getItem(QUICKSTART_SELECTION_STORAGE_KEY);
  } catch {
    return null;
  }
}

export function writeStoredQuickstartSelectedAccountId(accountId: string): void {
  if (typeof window === "undefined") return;
  try {
    window.sessionStorage.setItem(QUICKSTART_SELECTION_STORAGE_KEY, accountId);
  } catch {
    /* silent */
  }
}

export function clearStoredQuickstartSelectedAccountId(): void {
  if (typeof window === "undefined") return;
  try {
    window.sessionStorage.removeItem(QUICKSTART_SELECTION_STORAGE_KEY);
  } catch {
    /* silent */
  }
}

export function consumeStoredQuickstartSelectedAccountId(): string | null {
  const value = readStoredQuickstartSelectedAccountId();
  if (value) {
    clearStoredQuickstartSelectedAccountId();
  }
  return value;
}
