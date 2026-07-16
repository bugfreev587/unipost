const META_DM_REPLY_WINDOW_MS = 24 * 60 * 60 * 1000;

export function isMetaDMReplyWindowClosed(
  source: string,
  lastInboundReceivedAt: string,
  now = Date.now(),
): boolean {
  if (source !== "ig_dm" && source !== "fb_dm") {
    return false;
  }

  const receivedAt = new Date(lastInboundReceivedAt).getTime();
  if (!Number.isFinite(receivedAt) || receivedAt > now) {
    return false;
  }

  return now - receivedAt > META_DM_REPLY_WINDOW_MS;
}
