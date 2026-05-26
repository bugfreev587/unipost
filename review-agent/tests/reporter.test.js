import test from "node:test";
import assert from "node:assert/strict";
import { createAgentReporter } from "../src/reporter.js";

test("reporter posts elapsed agent events with bearer auth", async () => {
  const requests = [];
  const reporter = createAgentReporter({
    token: "revtok_live",
    apiUrl: "https://api.example.com",
    now: (() => {
      const ticks = [1000, 1042];
      return () => ticks.shift() ?? 1042;
    })(),
    fetchImpl: async (url, init) => {
      requests.push({ url, init, body: JSON.parse(init.body) });
      return { ok: true, async json() { return { data: { ok: true } }; } };
    },
  });

  await reporter.event("recording_started", "Recorder started", { step_id: "open_review_app" });

  assert.equal(requests[0].url, "https://api.example.com/v1/review/agent/events");
  assert.equal(requests[0].init.headers.Authorization, "Bearer revtok_live");
  assert.equal(requests[0].body.elapsed_ms, 42);
  assert.equal(requests[0].body.metadata.step_id, "open_review_app");
});

test("reporter completes and fails jobs through agent endpoints", async () => {
  const paths = [];
  const reporter = createAgentReporter({
    token: "revtok_live",
    apiUrl: "https://api.example.com/",
    now: () => 2000,
    fetchImpl: async (url) => {
      paths.push(new URL(url).pathname);
      return { ok: true, async json() { return { data: { status: "ok" } }; } };
    },
  });

  await reporter.complete({ markers: [] });
  await reporter.fail(new Error("redirect URI mismatch"), { last_step: "wait_for_oauth" });

  assert.deepEqual(paths, ["/v1/review/agent/complete", "/v1/review/agent/fail"]);
});


test("uploads review artifacts before completion", async () => {
  const requests = [];
  const reporter = createAgentReporter({
    token: "revtok_live",
    apiUrl: "https://api.example.com",
    now: () => 2000,
    fetchImpl: async (url, init = {}) => {
      requests.push({ url, init, body: typeof init.body === "string" ? JSON.parse(init.body) : null });
      if (url === "https://api.example.com/v1/review/agent/artifacts") {
        return { ok: true, async json() { return { data: { file_id: "review-artifacts/ws_1/rvjob_1/demo-video.webm", upload_url: "https://uploads.example.com/video", method: "PUT", headers: { "Content-Type": "video/webm" } } }; } };
      }
      if (url === "https://uploads.example.com/video") {
        return { ok: true, async text() { return ""; } };
      }
      return { ok: true, async json() { return { data: { status: "ok" } }; } };
    },
    readFileImpl: async (filePath) => Buffer.from("video-bytes:" + filePath),
    statImpl: async () => ({ size: 11 }),
  });

  const fileID = await reporter.uploadArtifact({
    artifactType: "demo_video",
    contentType: "video/webm",
    path: "/tmp/demo.webm",
  });

  assert.equal(fileID, "review-artifacts/ws_1/rvjob_1/demo-video.webm");
  assert.equal(requests[0].url, "https://api.example.com/v1/review/agent/artifacts");
  assert.equal(requests[0].body.size_bytes, 11);
  assert.equal(requests[1].url, "https://uploads.example.com/video");
  assert.equal(requests[1].init.method, "PUT");
  assert.equal(requests[1].init.headers["Content-Type"], "video/webm");
});
