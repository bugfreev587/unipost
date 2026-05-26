import test from "node:test";
import assert from "node:assert/strict";
import { buildReviewSessionCookie } from "../src/session-cookie.js";

test("builds a customer-domain review session cookie", () => {
  const cookie = buildReviewSessionCookie({
    start_url: "https://review.example.com/tiktok/posting",
    review_session: { cookie_name: "__unipost_review_session", expires_at: "2026-05-26T21:00:00Z" },
  }, "revsess_live");

  assert.equal(cookie.name, "__unipost_review_session");
  assert.equal(cookie.value, "revsess_live");
  assert.equal(cookie.url, "https://review.example.com/tiktok/posting");
  assert.equal(cookie.expires, 1779829200);
});
