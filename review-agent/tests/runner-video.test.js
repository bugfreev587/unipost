import test from "node:test";
import assert from "node:assert/strict";
import * as runner from "../src/runner.js";

test("browser context options record a page video at the scripted viewport size", () => {
  const options = runner.buildBrowserContextOptions({
    job_id: "rvjob_video",
    platform: "tiktok",
    agent_version: "0.1.0",
    start_url: "https://review.example.com/tiktok/posting",
    recording: { window_width: 1280, window_height: 720 },
    steps: [{ id: "open", action: "goto", url: "https://review.example.com/tiktok/posting" }],
  }, { videoDir: "/tmp/unipost-review-videos" });

  assert.deepEqual(options.viewport, { width: 1280, height: 720 });
  assert.deepEqual(options.recordVideo, {
    dir: "/tmp/unipost-review-videos",
    size: { width: 1280, height: 720 },
  });
});

test("completion artifacts include the recorded video path and marker timeline", async () => {
  const artifacts = await runner.buildCompletionArtifacts({
    markers: [{ step_id: "publish", label: "Publish test video", elapsed_ms: 1400 }],
    video: { path: async () => "/tmp/unipost-review-videos/rvjob-video.webm" },
  });

  assert.deepEqual(artifacts.markers, [{ step_id: "publish", label: "Publish test video", elapsed_ms: 1400 }]);
  assert.deepEqual(artifacts.video, {
    format: "webm",
    local_path: "/tmp/unipost-review-videos/rvjob-video.webm",
    capture_mode: "playwright-page-video",
    note: "Beta artifact captures the page viewport. Native browser-window capture is required before claiming address-bar coverage.",
  });
});


test("runScript uses the recording context and reports the finalized video artifact", async () => {
  let contextOptions;
  let completionArtifacts;
  const script = {
    job_id: "rvjob_run_video",
    platform: "tiktok",
    agent_version: "0.1.0",
    start_url: "https://review.example.com/tiktok/posting",
    recording: { window_width: 1024, window_height: 768 },
    steps: [{ id: "marker", action: "emit_marker", marker: "Open review app" }],
  };
  const page = { video: () => ({ path: async () => "/tmp/unipost-review-videos/run-video.webm" }) };
  const context = {
    addCookies: async () => {},
    newPage: async () => page,
    close: async () => {},
  };
  const playwrightImpl = {
    chromium: {
      launch: async () => ({
        newContext: async (options) => {
          contextOptions = options;
          return context;
        },
        close: async () => {},
      }),
    },
  };
  const reporter = {
    event: async () => {},
    complete: async (artifacts) => { completionArtifacts = artifacts; },
    fail: async () => assert.fail("runScript should complete"),
  };

  await runner.runScript(script, {
    reporter,
    sessionToken: "rvsession_test",
    playwrightImpl,
    out: { write() {} },
  });

  assert.equal(contextOptions.recordVideo.size.width, 1024);
  assert.equal(contextOptions.recordVideo.size.height, 768);
  assert.equal(completionArtifacts.video.local_path, "/tmp/unipost-review-videos/run-video.webm");
  assert.equal(completionArtifacts.markers[0].step_id, "marker");
  assert.equal(typeof completionArtifacts.markers[0].elapsed_ms, "number");
});
