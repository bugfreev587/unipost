// Canonical supported Connect platform set for the Developer → App Users
// surface.
//
// The App Users list previously hard-coded a four-platform subset
// (twitter/linkedin/bluesky/youtube) in three places: the SQL aggregate, the
// API response mapping, and the badge renderer. An App User who connected
// TikTok therefore reported the correct account_count while rendering only one
// platform icon. Everything that renders or iterates platform_counts now reads
// this list, so a future Connect platform is added in one place.

import type { ConnectSessionPlatform } from "./api";

/** Supported Connect platforms, in badge render order. */
export const MANAGED_USER_PLATFORMS: readonly ConnectSessionPlatform[] = [
  "twitter",
  "linkedin",
  "bluesky",
  "youtube",
  "tiktok",
  "instagram",
  "threads",
  "facebook",
  "pinterest",
];

const PLATFORM_LABELS: Record<ConnectSessionPlatform, string> = {
  twitter: "X",
  linkedin: "LinkedIn",
  bluesky: "Bluesky",
  youtube: "YouTube",
  tiktok: "TikTok",
  instagram: "Instagram",
  threads: "Threads",
  facebook: "Facebook",
  pinterest: "Pinterest",
};

/**
 * Human-readable platform name. Falls back to the raw value so an account
 * stored under a platform the dashboard does not know about still renders
 * something meaningful instead of blank.
 */
export function platformDisplayName(platform: string): string {
  return PLATFORM_LABELS[platform as ConnectSessionPlatform] ?? platform;
}

/**
 * Human-readable connection type. `managed` accounts were onboarded through a
 * hosted Connect flow; `byo` accounts use the customer's own platform app.
 */
export function connectionTypeLabel(connectionType: string): string {
  if (connectionType === "managed") return "UniPost-managed (Connect)";
  if (connectionType === "byo") return "Own platform app (BYO)";
  return connectionType;
}
