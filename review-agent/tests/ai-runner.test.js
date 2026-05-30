import test from "node:test";
import assert from "node:assert/strict";

import { runAIGuidedScript } from "../src/ai-runner.js";

test("runAIGuidedScript observes page, requests action, executes allowed click, and reports evidence", async () => {
  const calls = [];
  const page = {
    url: () => "https://review.example.com/tiktok/posting",
    title: async () => "TailTales",
    locator: (selector) => ({
      allInnerTexts: async () => ["Connect TikTok"],
      click: async () => calls.push(`click:${selector}`),
    }),
    evaluate: async () => [{ role: "button", text: "Connect TikTok", selector_hint: "[data-review-step='connect-tiktok']" }],
    waitForTimeout: async (ms) => calls.push(`wait:${ms}`),
  };

  await runAIGuidedScript({
    script: {
      job_id: "rvjob_ai",
      steps: [{ id: "connect_tiktok", marker: "Start TikTok OAuth", goal: "Connect TikTok" }],
    },
    page,
    nextActionImpl: async () => ({
      action: "click",
      target: { selector: "[data-review-step='connect-tiktok']" },
      hold_ms_after_action: 2000,
    }),
    reporter: { event: async (type) => calls.push(`event:${type}`) },
  });

  assert.deepEqual(calls, [
    "event:ai_observation_captured",
    "click:[data-review-step='connect-tiktok']",
    "wait:2000",
    "event:ai_action_completed",
  ]);
});

test("runAIGuidedScript rejects unsupported local actions", async () => {
  const page = {
    url: () => "https://review.example.com",
    title: async () => "",
    locator: () => ({ allInnerTexts: async () => [] }),
    evaluate: async () => [],
  };

  await assert.rejects(
    () =>
      runAIGuidedScript({
        script: { job_id: "rvjob_ai", steps: [{ id: "bad", goal: "bad" }] },
        page,
        nextActionImpl: async () => ({ action: "eval", value: "alert(1)" }),
        reporter: { event: async () => {} },
      }),
    /not supported/,
  );
});
