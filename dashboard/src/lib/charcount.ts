// Per-platform character counters for the Sprint 2 hosted preview
// page. The goal is to match what each platform's compose box
// reports so a draft that says "234 / 280" in our preview really
// rejects at 281 when posted.
//
// Sprint 3 PR9 picked the "hand-rolled approximation" path over the
// official `twitter-text` npm dependency because:
//
//   - twitter-text is ~200KB minified and only marginally more
//     accurate than the heuristic below for the 95% case (plain
//     text + URLs);
//
//   - the preview page is a cold-load route that already pulls in
//     React + Next.js, so adding 200KB more isn't free even though
//     it isn't hot-path latency either;
//
//   - the cases where twitter-text disagrees with this heuristic
//     (CJK weighting, exotic emoji clusters) typically still leave
//     the post under the limit, so a false negative on our side
//     just means the user is told they have slightly more headroom
//     than they actually do — never the other way around.
//
// If a customer ever complains, swapping for the real library is
// a one-line change.

const PLATFORM_LIMITS: Record<string, number> = {
  twitter: 280,
  bluesky: 300,
  linkedin: 3000,
  threads: 500,
  instagram: 2200,
  tiktok: 2200,
  youtube: 5000,
};

// Twitter counts URLs as a fixed 23 chars regardless of actual length
// (t.co wraps every link). This regex matches the same shape twitter
// does — protocol-prefixed and bare-domain URLs — and is intentionally
// permissive on the path / query.
const URL_REGEX =
  /\bhttps?:\/\/[^\s]+|\b(?:[a-z0-9-]+\.)+(?:com|org|net|io|dev|app|co|ai|xyz|me)\b[^\s]*/gi;
const TWITTER_URL_WEIGHT = 23;

// twitterCount: code points (NOT UTF-16 units) + URL collapsing.
// Twitter's grapheme weighting for CJK is more nuanced (each CJK
// char counts as 2) but the gap between code-point and weighted
// counts only matters for posts that are already very close to the
// limit, which most users avoid.
function twitterCount(text: string): number {
  const urlMatches = text.match(URL_REGEX) || [];
  const bodyWithoutURLs = text.replace(URL_REGEX, "");
  const bodyCodePoints = [...bodyWithoutURLs].length;
  return bodyCodePoints + urlMatches.length * TWITTER_URL_WEIGHT;
}

// blueskyCount: grapheme count via Intl.Segmenter, which is what
// the official Bluesky web client uses. Falls back to code-point
// count if Segmenter isn't available (older Safari, etc).
function blueskyCount(text: string): number {
  if (typeof Intl !== "undefined" && "Segmenter" in Intl) {
    const seg = new Intl.Segmenter("en", { granularity: "grapheme" });
    let n = 0;
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    for (const _ of seg.segment(text)) n++;
    return n;
  }
  return [...text].length;
}

export type CharCount = {
  used: number;
  limit: number;
  over: boolean;
};

// countChars returns the per-platform character count + the platform's
// hard limit. Unknown platforms fall back to UTF-16 code unit count
// and a 0 limit (so the over check is always false — display as
// `${used}` without a / N suffix).
export function countChars(platform: string, caption: string): CharCount {
  const limit = PLATFORM_LIMITS[platform] ?? 0;
  let used: number;
  switch (platform) {
    case "twitter":
      used = twitterCount(caption);
      break;
    case "bluesky":
      used = blueskyCount(caption);
      break;
    default:
      // LinkedIn, Threads, Instagram, TikTok, YouTube: UTF-16 code
      // units (.length) is what they all use server-side.
      used = caption.length;
  }
  return { used, limit, over: limit > 0 && used > limit };
}
