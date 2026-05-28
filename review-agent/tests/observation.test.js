import test from "node:test";
import assert from "node:assert/strict";

import { collectPageObservation, redactObservation } from "../src/observation.js";

test("redactObservation removes password-like visible text and DOM hints", () => {
  const obs = redactObservation({
    visible_text: "Password hunter2 token abc123 Connect TikTok",
    dom_hints: [
      { role: "textbox", text: "Password", selector_hint: "input[type=password]" },
      { role: "button", text: "Connect TikTok", selector_hint: "[data-review-step='connect-tiktok']" },
    ],
  });

  assert.equal(obs.visible_text.includes("hunter2"), false);
  assert.equal(obs.visible_text.includes("abc123"), false);
  assert.deepEqual(obs.dom_hints, [
    { role: "button", text: "Connect TikTok", selector_hint: "[data-review-step='connect-tiktok']" },
  ]);
});

test("collectPageObservation captures url title text and review-step hints", async () => {
  const page = {
    url: () => "https://review.example.com/tiktok/posting",
    title: async () => "TailTales",
    locator: () => ({ allInnerTexts: async () => ["Connect TikTok", "Upload video"] }),
    evaluate: async () => [
      { role: "button", text: "Connect TikTok", selector_hint: "[data-review-step='connect-tiktok']" },
    ],
  };

  const obs = await collectPageObservation(page, { jobId: "rvjob_1", stepKey: "connect_tiktok" });

  assert.equal(obs.job_id, "rvjob_1");
  assert.equal(obs.current_url, "https://review.example.com/tiktok/posting");
  assert.equal(obs.page_title, "TailTales");
  assert.equal(obs.visible_text, "Connect TikTok\nUpload video");
  assert.equal(obs.dom_hints[0].selector_hint, "[data-review-step='connect-tiktok']");
});
