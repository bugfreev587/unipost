// Single source of truth for which metrics each social platform exposes via
// its public API. Used by the analytics UI to render "N/A" with an
// explanation instead of a misleading "--" or "0" for unsupported metrics.
//
// Keep this in sync with the platform adapters under `api/internal/platform/`.

export type Platform =
  | "bluesky"
  | "linkedin"
  | "instagram"
  | "threads"
  | "tiktok"
  | "twitter"
  | "youtube";

export type MetricKey =
  | "impressions"
  | "reach"
  | "likes"
  | "comments"
  | "shares"
  | "saves"
  | "clicks"
  | "video_views";

type Caps = Record<MetricKey, boolean>;

// Snapshot reflecting the platform adapter behavior as of 2026-04.
// See `api/internal/platform/<platform>.go` for the live source.
export const PLATFORM_METRICS: Record<Platform, Caps> = {
  twitter: {
    impressions: true,
    reach: false,
    likes: true,
    comments: true,
    shares: true,
    saves: false,
    clicks: false,
    video_views: false,
  },
  linkedin: {
    impressions: true,
    reach: true,
    likes: true,
    comments: true,
    shares: true,
    saves: false,
    clicks: true,
    video_views: false,
  },
  threads: {
    impressions: true,
    reach: false,
    likes: true,
    comments: true,
    shares: true,
    saves: false,
    clicks: false,
    video_views: false,
  },
  instagram: {
    // Removed in Graph API v22 (April 2024) for IMAGE / CAROUSEL.
    impressions: false,
    reach: true,
    likes: true,
    comments: true,
    shares: true,
    saves: true,
    clicks: false,
    video_views: false,
  },
  bluesky: {
    impressions: false,
    reach: false,
    likes: true,
    comments: true,
    shares: true,
    saves: false,
    clicks: false,
    video_views: false,
  },
  youtube: {
    impressions: false,
    reach: false,
    likes: true,
    comments: true,
    shares: false,
    saves: false,
    clicks: false,
    video_views: true,
  },
  tiktok: {
    impressions: false,
    reach: false,
    likes: true,
    comments: true,
    shares: true,
    saves: false,
    clicks: false,
    // TikTok exposes view_count (= video plays), not display impressions.
    video_views: true,
  },
};

export function platformSupports(platform: string, metric: MetricKey): boolean {
  const p = platform.toLowerCase() as Platform;
  return PLATFORM_METRICS[p]?.[metric] ?? false;
}

// True if at least one of the given platforms exposes the metric.
export function anyPlatformSupports(platforms: string[], metric: MetricKey): boolean {
  return platforms.some((p) => platformSupports(p, metric));
}

// Human-readable explanation for why a (platform, metric) pair is unsupported.
// Used as a tooltip on "N/A" cells.
export function unsupportedReason(platform: string, metric: MetricKey): string {
  const p = platform.toLowerCase();
  const Pname = p.charAt(0).toUpperCase() + p.slice(1);

  if (metric === "impressions") {
    switch (p) {
      case "instagram":
        return "Instagram removed organic impressions in Graph API v22 (April 2024)";
      case "bluesky":
        return "Bluesky API doesn't expose impression data";
      case "youtube":
        return "YouTube Data API doesn't expose impressions for individual videos";
      case "tiktok":
        return "TikTok exposes view_count (video plays), not display impressions";
      default:
        return `${Pname} doesn't expose impressions via API`;
    }
  }
  return `${Pname} doesn't expose ${metric} via API`;
}
