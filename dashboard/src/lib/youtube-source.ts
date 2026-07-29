export const YOUTUBE_HOME_URL = "https://www.youtube.com/";

const YOUTUBE_CONTENT_HOSTS = new Set([
  "youtube.com",
  "www.youtube.com",
  "m.youtube.com",
  "youtu.be",
]);

export function buildYouTubeChannelUrl(rawChannelId?: string | null): string | null {
  const channelId = rawChannelId?.trim();
  if (!channelId || channelId.toLowerCase().startsWith("disconnected:")) return null;
  return `https://www.youtube.com/channel/${encodeURIComponent(channelId)}`;
}

export function normalizeYouTubeContentUrl(rawUrl?: string | null): string | null {
  const value = rawUrl?.trim();
  if (!value) return null;
  try {
    const parsed = new URL(value);
    if (parsed.protocol !== "https:" || !YOUTUBE_CONTENT_HOSTS.has(parsed.hostname.toLowerCase())) return null;
    return parsed.toString();
  } catch {
    return null;
  }
}
