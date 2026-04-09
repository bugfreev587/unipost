// Single source of truth for character limits across all 7 platforms.
// Used by the Character Counter tool and the AgentPost web composer.
//
// Keep these in sync with the UniPost API's capabilities endpoint
// (api/internal/platform/capabilities.go). The counting methods
// match the validate path in the Go backend:
//   - twitter:  weighted (URLs → 23 chars, CJK → 2 chars each)
//   - bluesky:  grapheme segmenter (emoji = 1, not 2+ code points)
//   - standard: string.length (UTF-16 code units)

export interface PlatformLimit {
  platform: string;
  icon: string;
  name: string;
  maxLength: number;
  countingMethod: "standard" | "twitter" | "grapheme";
}

export const PLATFORM_LIMITS: PlatformLimit[] = [
  { platform: "twitter",   icon: "\uD835\uDD4F", name: "X / Twitter",  maxLength: 280,  countingMethod: "twitter" },
  { platform: "linkedin",  icon: "\uD83D\uDCBC", name: "LinkedIn",     maxLength: 3000, countingMethod: "standard" },
  { platform: "instagram", icon: "\uD83D\uDCF8", name: "Instagram",    maxLength: 2200, countingMethod: "standard" },
  { platform: "threads",   icon: "\uD83E\uDDF5", name: "Threads",      maxLength: 500,  countingMethod: "standard" },
  { platform: "tiktok",    icon: "\uD83C\uDFB5", name: "TikTok",       maxLength: 2200, countingMethod: "standard" },
  { platform: "youtube",   icon: "\u25B6\uFE0F",  name: "YouTube",      maxLength: 5000, countingMethod: "standard" },
  { platform: "bluesky",   icon: "\uD83E\uDD8B", name: "Bluesky",      maxLength: 300,  countingMethod: "grapheme" },
];

// Twitter's weighted character counting:
// - URLs (http/https) → 23 chars each
// - Characters in Unicode ranges for CJK → 2 chars each
// - Everything else → 1 char each
// Simplified version — the full twitter-text library is 200KB and
// handles edge cases (t.co wrapping, emoji variation selectors) that
// don't matter for a character counter tool.
export function twitterWeightedCount(text: string): number {
  // Replace URLs with 23-char placeholders before counting.
  const urlRegex = /https?:\/\/[^\s]+/g;
  const withoutUrls = text.replace(urlRegex, "");
  const urlCount = (text.match(urlRegex) || []).length;

  let count = urlCount * 23;
  for (const char of withoutUrls) {
    const code = char.codePointAt(0)!;
    // CJK Unified Ideographs + CJK Compatibility + Katakana + Hiragana + etc.
    if (
      (code >= 0x1100 && code <= 0x11FF) ||  // Hangul Jamo
      (code >= 0x2E80 && code <= 0x9FFF) ||  // CJK ranges
      (code >= 0xAC00 && code <= 0xD7AF) ||  // Hangul Syllables
      (code >= 0xF900 && code <= 0xFAFF) ||  // CJK Compatibility Ideographs
      (code >= 0xFE30 && code <= 0xFE4F) ||  // CJK Compatibility Forms
      (code >= 0xFF00 && code <= 0xFFEF) ||  // Halfwidth/Fullwidth Forms
      (code >= 0x20000 && code <= 0x2FA1F)   // CJK Unified Ideographs Extension B+
    ) {
      count += 2;
    } else {
      count += 1;
    }
  }
  return count;
}

// Bluesky counts grapheme clusters, not code points. The key
// difference: a compound emoji like 👨‍👩‍👧‍👦 is 1 grapheme
// despite being 7 code points.
export function graphemeCount(text: string): number {
  if (typeof Intl !== "undefined" && Intl.Segmenter) {
    return [...new Intl.Segmenter().segment(text)].length;
  }
  // Fallback for environments without Intl.Segmenter — use spread
  // which splits on code points (close enough for ASCII/Latin text,
  // over-counts compound emoji by 1-3 but never under-counts).
  return [...text].length;
}

// Unified counting function.
export function countCharacters(text: string, method: PlatformLimit["countingMethod"]): number {
  switch (method) {
    case "twitter":
      return twitterWeightedCount(text);
    case "grapheme":
      return graphemeCount(text);
    default:
      return text.length;
  }
}

// Status thresholds: green < 80%, yellow 80-99%, red >= 100%.
export type CountStatus = "ok" | "warning" | "over";

export function getCountStatus(count: number, max: number): CountStatus {
  const pct = count / max;
  if (pct >= 1) return "over";
  if (pct >= 0.8) return "warning";
  return "ok";
}

export const STATUS_COLORS: Record<CountStatus, string> = {
  ok: "#10b981",
  warning: "#f59e0b",
  over: "#ef4444",
};
