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
    segments: [{ key: "posting_part_1", title: "Posting Part 1", scopes: ["video.upload"] }],
    segmentEvents: [{ key: "posting_part_1", title: "Posting Part 1", scopes: ["video.upload"], started_elapsed_ms: 0, completed_elapsed_ms: 1400 }],
    video: { path: async () => "/tmp/unipost-review-videos/rvjob-video.webm" },
  });

  assert.deepEqual(artifacts.markers, [{ step_id: "publish", label: "Publish test video", elapsed_ms: 1400 }]);
  assert.equal(artifacts.segments[0].key, "posting_part_1");
  assert.equal(artifacts.segment_events[0].completed_elapsed_ms, 1400);
  assert.deepEqual(artifacts.video, {
    format: "webm",
    local_path: "/tmp/unipost-review-videos/rvjob-video.webm",
    capture_mode: "playwright-page-video",
    includes_address_bar: false,
    note: "Fallback artifact captures the page viewport only. It does not satisfy address-bar evidence requirements.",
  });
});

test("runScript reports segment lifecycle events from script metadata", async () => {
  const events = [];
  const script = {
    job_id: "rvjob_segments",
    platform: "tiktok",
    agent_version: "0.1.0",
    start_url: "https://review.example.com/tiktok/posting",
    segments: [{ key: "posting_part_1", title: "Posting Part 1", filename: "part-1.mp4", scopes: ["user.info.basic"], estimated_duration_sec: 60 }],
    steps: [{ id: "segment_posting_part_1", action: "emit_marker", marker: "Posting Part 1" }],
  };
  const page = { video: () => ({ path: async () => "/tmp/unipost-review-videos/segments.webm" }) };
  const context = { addCookies: async () => {}, newPage: async () => page, close: async () => {} };
  const playwrightImpl = { chromium: { launch: async () => ({ newContext: async () => context, close: async () => {} }) } };
  const reporter = {
    event: async (eventType, message, metadata) => events.push({ eventType, message, metadata }),
    uploadArtifact: async (artifact) => artifact.artifactType === "demo_video"
      ? "review-artifacts/ws_1/rvjob_segments/demo-video.webm"
      : "review-artifacts/ws_1/rvjob_segments/execution-evidence.json",
    complete: async (artifacts) => {
      assert.equal(artifacts.segments[0].key, "posting_part_1");
      assert.equal(artifacts.segment_events.some((event) => event.key === "posting_part_1"), true);
    },
    fail: async () => assert.fail("runScript should complete"),
  };

  await runner.runScript(script, { reporter, sessionToken: "rvsession_test", playwrightImpl, out: { write() {} } });

  assert.equal(events.some((event) => event.eventType === "segment_started" && event.metadata.key === "posting_part_1"), true);
  assert.equal(events.some((event) => event.eventType === "segment_completed" && event.metadata.key === "posting_part_1"), true);
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

test("manual pause overlay waits for the page body before injecting instructions", async () => {
  let bodyReady = false;
  let completed = false;
  const script = {
    job_id: "rvjob_manual_pause",
    platform: "tiktok",
    agent_version: "0.1.0",
    start_url: "https://review.example.com/tiktok/posting",
    steps: [{ id: "wait_for_oauth", action: "manual_pause", overlay: "Log in to TikTok." }],
  };
  const page = {
    video: () => ({ path: async () => "/tmp/unipost-review-videos/manual-pause.webm" }),
    waitForLoadState: async () => {},
    locator: (selector) => {
      assert.equal(selector, "body");
      return {
        first: () => ({
          waitFor: async () => {
            bodyReady = true;
          },
        }),
      };
    },
    evaluate: async () => {
      assert.equal(bodyReady, true, "overlay injection should wait until body is ready");
    },
  };
  const context = { addCookies: async () => {}, newPage: async () => page, close: async () => {} };
  const playwrightImpl = { chromium: { launch: async () => ({ newContext: async () => context, close: async () => {} }) } };
  const reporter = {
    event: async () => {},
    uploadArtifact: async (artifact) => artifact.artifactType === "demo_video"
      ? "review-artifacts/ws_1/rvjob_manual_pause/demo-video.webm"
      : "review-artifacts/ws_1/rvjob_manual_pause/execution-evidence.json",
    complete: async () => { completed = true; },
    fail: async () => assert.fail("manual pause should complete"),
  };

  await runner.runScript(script, { reporter, sessionToken: "rvsession_test", playwrightImpl, out: { write() {} } });

  assert.equal(completed, true);
});

test("normalizes TikTok review OAuth URLs to content-posting scopes", () => {
  const originalAuthorize = "https://www.tiktok.com/v2/auth/authorize/?client_key=ck&redirect_uri=https%3A%2F%2Fdev-api.unipost.dev%2Fv1%2Fconnect%2Fcallback%2Ftiktok&response_type=code&scope=video.publish%2Cvideo.upload%2Cuser.info.basic%2Cvideo.list&state=rvstate_1";
  const normalizedAuthorize = runner.normalizeTikTokReviewOAuthURL(originalAuthorize);
  assert.equal(new URL(normalizedAuthorize).searchParams.get("scope"), "video.publish,video.upload,user.info.basic");

  const login = new URL("https://www.tiktok.com/login");
  login.searchParams.set("redirect_url", originalAuthorize);
  const normalizedLogin = runner.normalizeTikTokReviewOAuthURL(login.toString());
  const nested = new URL(new URL(normalizedLogin).searchParams.get("redirect_url"));
  assert.equal(nested.searchParams.get("scope"), "video.publish,video.upload,user.info.basic");
});

test("normalizes TikTok review OAuth URLs to requested analytics scopes", () => {
  const originalAuthorize = "https://www.tiktok.com/v2/auth/authorize/?client_key=ck&redirect_uri=https%3A%2F%2Fdev-api.unipost.dev%2Fv1%2Fconnect%2Fcallback%2Ftiktok&response_type=code&scope=video.publish%2Cvideo.upload%2Cuser.info.basic&state=rvstate_1";
  const normalizedAuthorize = runner.normalizeTikTokReviewOAuthURL(originalAuthorize, ["user.info.profile", "user.info.stats"]);
  assert.equal(new URL(normalizedAuthorize).searchParams.get("scope"), "user.info.profile,user.info.stats");
});

test("connect step fails clearly when TikTok skips OAuth consent", async () => {
  const events = [];
  let failedReason = "";
  const script = {
    job_id: "rvjob_oauth_skipped",
    platform: "tiktok",
    agent_version: "0.1.0",
    requested_scopes: ["user.info.profile", "user.info.stats"],
    start_url: "https://review.example.com/tiktok/analytics",
    steps: [{ id: "connect_tiktok", action: "click", selector: "[data-review-step='connect-tiktok']" }],
  };
  const page = {
    video: () => ({ path: async () => "/tmp/unipost-review-videos/oauth-skipped.webm" }),
    url: () => "https://review.example.com/tiktok/analytics?connect_status=success",
    waitForURL: async () => {
      throw new Error("timed out waiting for TikTok");
    },
    locator: () => ({ click: async () => {} }),
  };
  const context = { addCookies: async () => {}, newPage: async () => page, close: async () => {} };
  const playwrightImpl = { chromium: { launch: async () => ({ newContext: async () => context, close: async () => {} }) } };
  const reporter = {
    event: async (eventType, message, metadata) => events.push({ eventType, message, metadata }),
    fail: async (error) => {
      failedReason = error.message;
    },
  };

  await assert.rejects(
    () => runner.runScript(script, { reporter, sessionToken: "rvsession_test", playwrightImpl, nativeCaptureImpl: async () => null, out: { write() {} } }),
    /TikTok skipped the authorization page/
  );

  assert.equal(events.some((event) => event.eventType === "oauth_consent_skipped"), true);
  assert.match(failedReason, /Remove app access/);
});
