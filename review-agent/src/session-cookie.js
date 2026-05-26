export function buildReviewSessionCookie(script, sessionToken) {
  if (!sessionToken) return null;
  const cookieName = script?.review_session?.cookie_name || "__unipost_review_session";
  const cookie = {
    name: cookieName,
    value: sessionToken,
    url: script.start_url,
    sameSite: "Lax",
    secure: true,
  };
  const expiresAt = script?.review_session?.expires_at;
  if (expiresAt) {
    const timestamp = Date.parse(expiresAt);
    if (Number.isFinite(timestamp)) {
      cookie.expires = Math.floor(timestamp / 1000);
    }
  }
  return cookie;
}
