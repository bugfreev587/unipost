import test from "node:test";
import assert from "node:assert/strict";
import { validateScript } from "../src/script-contract.js";
import { fetchReviewScript } from "../src/client.js";
import { runDoctor } from "../src/doctor.js";

const validScript = {
  job_id: "rvjob_1",
  platform: "tiktok",
  agent_version: "0.1.0",
  start_url: "https://review.example.com/tiktok/posting",
  steps: [
    { id: "open", action: "goto", url: "https://review.example.com/tiktok/posting" },
    { id: "publish", action: "click", selector: "[data-review-step='publish-tiktok']" },
  ],
};

test("validates the closed review script contract", () => {
  assert.equal(validateScript(validScript).job_id, "rvjob_1");
});

test("rejects arbitrary JavaScript actions", () => {
  assert.throws(() => validateScript({ ...validScript, steps: [{ id: "bad", action: "eval", value: "alert(1)" }] }), /not allowed/);
});

test("fetchReviewScript sends the bearer token and validates the response", async () => {
  let authHeader = "";
  const script = await fetchReviewScript({
    token: "revtok_123",
    apiUrl: "https://api.example.com",
    fetchImpl: async (_url, init) => {
      authHeader = init.headers.Authorization;
      return {
        ok: true,
        async json() { return { data: validScript }; },
      };
    },
  });
  assert.equal(script.job_id, "rvjob_1");
  assert.equal(authHeader, "Bearer revtok_123");
});

test("doctor warns macOS users about screen recording permission", () => {
  const checks = runDoctor({ platform: "darwin", nodeVersion: "20.11.1" });
  assert.ok(checks.some((check) => check.id === "macos-screen-recording" && check.warning));
});
