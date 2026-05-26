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
    includes_address_bar: false,
    note: "Fallback artifact captures the page viewport only. It does not satisfy address-bar evidence requirements.",
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


test("runScript uploads the finalized video and execution evidence before completing the job", async () => {
  let completedVideoFileID = "";
  const uploaded = [];
  const script = {
    job_id: "rvjob_upload_video",
    platform: "tiktok",
    agent_version: "0.1.0",
    start_url: "https://review.example.com/tiktok/posting",
    steps: [{ id: "marker", action: "emit_marker", marker: "Open review app" }],
  };
  const page = { video: () => ({ path: async () => "/tmp/unipost-review-videos/run-video.webm" }) };
  const context = { addCookies: async () => {}, newPage: async () => page, close: async () => {} };
  const playwrightImpl = { chromium: { launch: async () => ({ newContext: async () => context, close: async () => {} }) } };
  const reporter = {
    event: async () => {},
    uploadArtifact: async (artifact) => {
      uploaded.push(artifact);
      if (artifact.artifactType === "execution_evidence") return "review-artifacts/ws_1/rvjob_upload_video/execution-evidence.json";
      return "review-artifacts/ws_1/rvjob_upload_video/demo-video.webm";
    },
    complete: async (artifacts, videoFileID) => {
      completedVideoFileID = videoFileID;
      assert.equal(artifacts.execution_evidence.file_id, "review-artifacts/ws_1/rvjob_upload_video/execution-evidence.json");
    },
    fail: async () => assert.fail("runScript should complete"),
  };

  await runner.runScript(script, { reporter, sessionToken: "rvsession_test", playwrightImpl, out: { write() {} } });

  assert.equal(uploaded[0].artifactType, "demo_video");
  assert.equal(uploaded[0].path, "/tmp/unipost-review-videos/run-video.webm");
  assert.equal(uploaded[1].artifactType, "execution_evidence");
  assert.equal(uploaded[1].contentType, "application/json");
  assert.equal(completedVideoFileID, "review-artifacts/ws_1/rvjob_upload_video/demo-video.webm");
});


test("runScript prefers native browser-window capture when address-bar evidence is required", async () => {
  let completionArtifacts;
  let uploadedContentType = "";
  const script = {
    job_id: "rvjob_native_video",
    platform: "tiktok",
    agent_version: "0.1.0",
    start_url: "https://review.example.com/tiktok/posting",
    recording: { window_width: 1200, window_height: 900, show_address_bar: true, capture_mode: "native-browser-window" },
    steps: [{ id: "marker", action: "emit_marker", marker: "Open review app" }],
  };
  const page = { video: () => ({ path: async () => assert.fail("page video should not be used when native capture succeeds") }) };
  const context = { addCookies: async () => {}, newPage: async () => page, close: async () => {} };
  const playwrightImpl = { chromium: { launch: async () => ({ newContext: async () => context, close: async () => {} }) } };
  const reporter = {
    event: async () => {},
    uploadArtifact: async (artifact) => {
      if (artifact.artifactType === "demo_video") {
        uploadedContentType = artifact.contentType;
        return "review-artifacts/ws_1/rvjob_native_video/demo-video.mov";
      }
      return "review-artifacts/ws_1/rvjob_native_video/execution-evidence.json";
    },
    complete: async (artifacts) => { completionArtifacts = artifacts; },
    fail: async () => assert.fail("runScript should complete"),
  };
  const nativeCaptureImpl = async () => ({
    mode: "macos-screencapture-region",
    localPath: "/tmp/unipost-review-videos/rvjob-native.mov",
    includesAddressBar: true,
    bounds: { left: 80, top: 80, width: 1200, height: 900 },
    stop: async () => {},
  });

  await runner.runScript(script, { reporter, sessionToken: "rvsession_test", playwrightImpl, nativeCaptureImpl, out: { write() {} } });

  assert.equal(completionArtifacts.video.capture_mode, "macos-screencapture-region");
  assert.equal(completionArtifacts.video.includes_address_bar, true);
  assert.equal(completionArtifacts.video.local_path, "/tmp/unipost-review-videos/rvjob-native.mov");
  assert.equal(uploadedContentType, "video/quicktime");
});
