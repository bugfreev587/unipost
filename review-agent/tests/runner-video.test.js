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

test("completion artifacts preserve split video segment paths", async () => {
  const artifacts = await runner.buildCompletionArtifacts({
    videoSegments: [
      { segment_key: "posting_part_1", local_path: "/tmp/part-1.mp4", scopes: ["video.upload"], duration_sec: 44, size_bytes: 42_000_000 },
      { segment_key: "posting_part_2", local_path: "/tmp/part-2.mp4", scopes: ["video.publish"], duration_sec: 39, size_bytes: 41_000_000 },
    ],
  });

  assert.equal(artifacts.video_segments.length, 2);
  assert.equal(artifacts.video_segments[0].format, "mp4");
  assert.equal(artifacts.video_segments[0].segment_key, "posting_part_1");
  assert.equal(artifacts.video_segments[0].size_bytes, 42_000_000);
});

test("policy link hold duration keeps external policy tabs visible for recording", () => {
  assert.equal(runner.policyLinkHoldDurationMs({}), 1200);
  assert.equal(runner.policyLinkHoldDurationMs({ policy_link_hold_ms: 0 }), 0);
  assert.equal(runner.policyLinkHoldDurationMs({ policy_link_hold_ms: 2500 }), 2500);
});

test("execution evidence preserves reviewer-facing segment metadata", () => {
  const evidence = runner.buildExecutionEvidence({
    jobId: "rvjob_evidence",
    artifacts: {
      video_segments: [{
        segment_key: "posting_part_1",
        title: "Content Posting Part 1 - Creator Info, Upload, and Content Details",
        filename: "tiktok-content-posting-part-1.mp4",
        local_path: "/tmp/tiktok-content-posting-part-1.mp4",
        format: "mp4",
        scopes: ["user.info.basic", "video.upload"],
        start_sec: 0.25,
        duration_sec: 42,
        size_bytes: 42_000_000,
        file_id: "review-artifacts/ws_1/rvjob_evidence/demo-video-posting_part_1.mp4",
      }],
    },
  });

  assert.equal(evidence.video_segments[0].filename, "tiktok-content-posting-part-1.mp4");
  assert.equal(evidence.video_segments[0].title, "Content Posting Part 1 - Creator Info, Upload, and Content Details");
  assert.equal(evidence.video_segments[0].start_sec, 0.25);
  assert.deepEqual(evidence.video_segments[0].scopes, ["user.info.basic", "video.upload"]);
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

test("runScript shows recorded section title overlays for review markers", async () => {
  const evaluations = [];
  const script = {
    job_id: "rvjob_marker_overlay",
    platform: "tiktok",
    agent_version: "0.1.0",
    start_url: "https://review.example.com/tiktok/posting",
    recording: { marker_overlay_ms: 1 },
    steps: [{ id: "creator_info", action: "assert_visible", selector: "[data-review-step='creator-info']", marker: "1. Retrieve Creator Info" }],
  };
  const page = {
    video: () => ({ path: async () => "/tmp/unipost-review-videos/marker-overlay.webm" }),
    evaluate: async (_fn, arg) => {
      evaluations.push(arg);
    },
    locator: (selector) => {
      assert.equal(selector, "[data-review-step='creator-info']");
      return {
        first: () => ({
          waitFor: async () => {},
        }),
      };
    },
  };
  const context = { addCookies: async () => {}, newPage: async () => page, close: async () => {} };
  const playwrightImpl = { chromium: { launch: async () => ({ newContext: async () => context, close: async () => {} }) } };
  const reporter = {
    event: async () => {},
    uploadArtifact: async (artifact) => artifact.artifactType === "demo_video"
      ? "review-artifacts/ws_1/rvjob_marker_overlay/demo-video.webm"
      : "review-artifacts/ws_1/rvjob_marker_overlay/execution-evidence.json",
    complete: async () => {},
    fail: async () => assert.fail("runScript should complete"),
  };

  await runner.runScript(script, { reporter, sessionToken: "rvsession_test", playwrightImpl, out: { write() {} } });

  assert.equal(evaluations.some((arg) => arg?.label === "1. Retrieve Creator Info" && arg?.stepId === "creator_info"), true);
  assert.equal(evaluations.some((arg) => arg?.remove === true), true);
});

test("runScript opens TikTok policy links in a temporary tab and closes them", async () => {
  let clickedSelector = "";
  let popupClosed = false;
  const script = {
    job_id: "rvjob_policy_link",
    platform: "tiktok",
    agent_version: "0.1.0",
    start_url: "https://review.example.com/tiktok/posting",
    recording: { policy_link_hold_ms: 0 },
    steps: [{ id: "open_music_policy", action: "open_link", selector: "[data-review-step='music-usage-confirmation-link']", value: "music-usage-confirmation" }],
  };
  const popup = {
    waitForLoadState: async (state) => assert.equal(state, "domcontentloaded"),
    url: () => "https://www.tiktok.com/legal/page/global/music-usage-confirmation/en",
    close: async () => {
      popupClosed = true;
    },
  };
  const page = {
    video: () => ({ path: async () => "/tmp/unipost-review-videos/policy-link.webm" }),
    waitForEvent: async (eventName) => {
      assert.equal(eventName, "popup");
      return popup;
    },
    locator: (selector) => ({
      click: async () => {
        clickedSelector = selector;
      },
    }),
  };
  const context = { addCookies: async () => {}, newPage: async () => page, close: async () => {} };
  const playwrightImpl = { chromium: { launch: async () => ({ newContext: async () => context, close: async () => {} }) } };
  const reporter = {
    event: async () => {},
    complete: async () => {},
    fail: async () => assert.fail("policy link step should complete"),
  };

  await runner.runScript(script, { reporter, sessionToken: "rvsession_test", playwrightImpl, nativeCaptureImpl: async () => null, out: { write() {} } });

  assert.equal(clickedSelector, "[data-review-step='music-usage-confirmation-link']");
  assert.equal(popupClosed, true);
});

test("runScript post-processes completed segments into uploadable 50MB video files", async () => {
  const uploaded = [];
  let postProcessInput;
  let completedArtifacts;
  const script = {
    job_id: "rvjob_segment_postprocess",
    platform: "tiktok",
    agent_version: "0.1.0",
    start_url: "https://review.example.com/tiktok/posting",
    recording: {
      window_width: 1920,
      window_height: 1080,
      show_address_bar: true,
      capture_mode: "native-browser-window",
      max_artifact_bytes: 50_000_000,
      split_automatically: true,
    },
    segments: [
      { key: "posting_part_1", title: "Posting Part 1", filename: "posting-part-1.mp4", scopes: ["user.info.basic", "video.upload"] },
      { key: "posting_part_2", title: "Posting Part 2", filename: "posting-part-2.mp4", scopes: ["video.publish"] },
    ],
    steps: [
      { id: "segment_posting_part_1", action: "emit_marker", marker: "Posting Part 1" },
      { id: "segment_posting_part_2", action: "emit_marker", marker: "Posting Part 2" },
    ],
  };
  const page = { video: () => ({ path: async () => assert.fail("native capture should provide the source video") }) };
  const context = { addCookies: async () => {}, newPage: async () => page, close: async () => {} };
  const playwrightImpl = { chromium: { launch: async () => ({ newContext: async () => context, close: async () => {} }) } };
  const reporter = {
    event: async () => {},
    uploadArtifact: async (artifact) => {
      uploaded.push(artifact);
      if (artifact.artifactType === "execution_evidence") return "review-artifacts/ws_1/rvjob_segment_postprocess/execution-evidence.json";
      return `review-artifacts/ws_1/rvjob_segment_postprocess/demo-video-${artifact.segmentKey}.mp4`;
    },
    complete: async (artifacts) => { completedArtifacts = artifacts; },
    fail: async () => assert.fail("runScript should complete"),
  };
  const nativeCaptureImpl = async () => ({
    mode: "macos-screencapture-region",
    localPath: "/tmp/unipost-review-videos/rvjob-full.mov",
    includesAddressBar: true,
    bounds: { left: 0, top: 0, width: 1920, height: 1080 },
    stop: async () => {},
  });
  const videoPostProcessImpl = async (input) => {
    postProcessInput = input;
    return [
      { segment_key: "posting_part_1", local_path: "/tmp/unipost-review-videos/posting-part-1.mp4", scopes: ["user.info.basic", "video.upload"], duration_sec: 35, size_bytes: 38_000_000 },
      { segment_key: "posting_part_2", local_path: "/tmp/unipost-review-videos/posting-part-2.mp4", scopes: ["video.publish"], duration_sec: 33, size_bytes: 37_000_000 },
    ];
  };

  await runner.runScript(script, { reporter, sessionToken: "rvsession_test", playwrightImpl, nativeCaptureImpl, videoPostProcessImpl, out: { write() {} } });

  assert.equal(postProcessInput.sourceVideo.local_path, "/tmp/unipost-review-videos/rvjob-full.mov");
  assert.equal(postProcessInput.maxBytes, 50_000_000);
  assert.equal(postProcessInput.segmentEvents.some((event) => event.key === "posting_part_1" && event.completed_elapsed_ms !== undefined), true);
  assert.deepEqual(uploaded.filter((artifact) => artifact.artifactType === "demo_video").map((artifact) => artifact.segmentKey), ["posting_part_1", "posting_part_2"]);
  assert.equal(completedArtifacts.video_segments[0].file_id, "review-artifacts/ws_1/rvjob_segment_postprocess/demo-video-posting_part_1.mp4");
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

test("runScript uploads split video segments with segment keys when present", async () => {
  const uploaded = [];
  const artifacts = await runner.buildCompletionArtifacts({
    videoSegments: [
      { segment_key: "analytics_part_1", local_path: "/tmp/analytics-1.mp4", scopes: ["user.info.profile"], duration_sec: 41, size_bytes: 40_000_000 },
      { segment_key: "analytics_part_2", local_path: "/tmp/analytics-2.mp4", scopes: ["user.info.stats"], duration_sec: 38, size_bytes: 39_000_000 },
    ],
  });
  const reporter = {
    uploadArtifact: async (artifact) => {
      uploaded.push(artifact);
      return `review-artifacts/ws_1/rvjob_segments/demo-video-${artifact.segmentKey}.mp4`;
    },
  };

  const videoFileID = await runner.uploadVideoArtifacts(reporter, artifacts, { write() {} });

  assert.equal(videoFileID, "review-artifacts/ws_1/rvjob_segments/demo-video-analytics_part_1.mp4");
  assert.deepEqual(uploaded.map((artifact) => artifact.segmentKey), ["analytics_part_1", "analytics_part_2"]);
  assert.equal(artifacts.video_segments[0].file_id, "review-artifacts/ws_1/rvjob_segments/demo-video-analytics_part_1.mp4");
});

test("prepareBrowserForNativeCapture waits for stale Chrome for Testing to quit", async () => {
  const calls = [];
  const waits = [];
  const runningStates = [true, true, false];
  const execFileImpl = async (_cmd, args) => {
    const script = args.join("\n");
    if (script.includes("exists process")) {
      calls.push("check");
      return { stdout: String(runningStates.shift()) };
    }
    calls.push("quit");
    return { stdout: "" };
  };

  await runner.prepareBrowserForNativeCapture({
    script: {
      recording: { show_address_bar: true, capture_mode: "native-browser-window" },
    },
    platform: "darwin",
    execFileImpl,
    delayImpl: async (ms) => { waits.push(ms); },
    out: { write() {} },
  });

  assert.deepEqual(calls, ["check", "quit", "check", "check"]);
  assert.deepEqual(waits, [250]);
});


test("runScript prefers native browser-window capture when address-bar evidence is required", async () => {
  let completionArtifacts;
  let uploadedContentType = "";
  let pageWasFront = false;
  const sequence = [];
  const script = {
    job_id: "rvjob_native_video",
    platform: "tiktok",
    agent_version: "0.1.0",
    start_url: "https://review.example.com/tiktok/posting",
    recording: { window_width: 1200, window_height: 900, show_address_bar: true, capture_mode: "native-browser-window" },
    steps: [{ id: "marker", action: "emit_marker", marker: "Open review app" }],
  };
  const page = {
    bringToFront: async () => { pageWasFront = true; },
    video: () => ({ path: async () => assert.fail("page video should not be used when native capture succeeds") }),
  };
  const context = { addCookies: async () => {}, newPage: async () => page, close: async () => {} };
  const playwrightImpl = { chromium: { launch: async () => {
    assert.deepEqual(sequence, ["prepare"], "native capture browser prep must run before launch");
    sequence.push("launch");
    return { newContext: async () => context, close: async () => {} };
  } } };
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
  const nativeCaptureImpl = async () => {
    assert.equal(pageWasFront, true, "review page should be frontmost before native capture starts");
    return {
      mode: "macos-screencapture-region",
      localPath: "/tmp/unipost-review-videos/rvjob-native.mov",
      includesAddressBar: true,
      bounds: { left: 80, top: 80, width: 1200, height: 900 },
      stop: async () => {},
    };
  };

  const prepareBrowserImpl = async () => { sequence.push("prepare"); };

  await runner.runScript(script, { reporter, sessionToken: "rvsession_test", playwrightImpl, nativeCaptureImpl, prepareBrowserImpl, out: { write() {} } });

  assert.deepEqual(sequence, ["prepare", "launch"]);
  assert.equal(completionArtifacts.video.capture_mode, "macos-screencapture-region");
  assert.equal(completionArtifacts.video.includes_address_bar, true);
  assert.equal(completionArtifacts.video.local_path, "/tmp/unipost-review-videos/rvjob-native.mov");
  assert.equal(uploadedContentType, "video/quicktime");
});

test("manual pause overlay waits for the page body before injecting instructions", async () => {
  let bodyReady = false;
  let completed = false;
  const events = [];
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
    event: async (eventType, message, metadata) => events.push({ eventType, message, metadata }),
    uploadArtifact: async (artifact) => artifact.artifactType === "demo_video"
      ? "review-artifacts/ws_1/rvjob_manual_pause/demo-video.webm"
      : "review-artifacts/ws_1/rvjob_manual_pause/execution-evidence.json",
    complete: async () => { completed = true; },
    fail: async () => assert.fail("manual pause should complete"),
  };

  await runner.runScript(script, { reporter, sessionToken: "rvsession_test", playwrightImpl, out: { write() {} } });

  assert.equal(completed, true);
  assert.deepEqual(
    events.filter((event) => event.eventType.startsWith("manual_pause")).map((event) => event.eventType),
    ["manual_pause_started", "manual_pause_completed"]
  );
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
