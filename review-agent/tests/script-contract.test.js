import test from "node:test";
import assert from "node:assert/strict";
import { validateScript } from "../src/script-contract.js";
import { fetchReviewScript, requestNextReviewAction } from "../src/client.js";
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

test("validates policy link opening as a closed selector action", () => {
  const script = validateScript({
    ...validScript,
    steps: [{ id: "open_music_policy", action: "open_link", selector: "[data-review-step='music-usage-confirmation-link']", value: "music-usage-confirmation" }],
  });
  assert.equal(script.steps[0].action, "open_link");

  assert.throws(() => validateScript({
    ...validScript,
    steps: [{ id: "open_music_policy", action: "open_link", value: "music-usage-confirmation" }],
  }), /selector is required/);
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

test("requestNextReviewAction posts observations with bearer auth", async () => {
	const requests = [];
	const action = await requestNextReviewAction({
		token: "revtok_123",
		apiUrl: "https://api.example.com/",
		request: { goal: "Wait for the page", observation: { visible_text: "Loading" } },
		fetchImpl: async (url, init) => {
			requests.push({ url, init, body: JSON.parse(init.body) });
			return {
				ok: true,
				async json() {
					return { data: { action: "wait", hold_ms_after_action: 2000 } };
				},
			};
		},
	});

	assert.equal(action.action, "wait");
	assert.equal(requests[0].url, "https://api.example.com/v1/review/agent/next-action");
	assert.equal(requests[0].init.method, "POST");
	assert.equal(requests[0].init.headers.Authorization, "Bearer revtok_123");
	assert.equal(requests[0].body.observation.visible_text, "Loading");
});

test("doctor warns macOS users about screen recording permission", () => {
	const checks = runDoctor({ platform: "darwin", nodeVersion: "20.11.1" });
	assert.ok(checks.some((check) => check.id === "macos-screen-recording" && check.warning));
});

test("doctor fails when ffmpeg is unavailable for review video splitting", () => {
  const checks = runDoctor({ platform: "linux", nodeVersion: "20.11.1", ffmpegAvailable: false });
  assert.ok(checks.some((check) => check.id === "ffmpeg" && !check.ok && !check.warning));
});


test("requires native capture when script asks to show the browser address bar", () => {
  assert.throws(() => validateScript({
    ...validScript,
    recording: { show_address_bar: true, capture_mode: "playwright-page-video" },
  }), /requires native-browser-window/);

  const script = validateScript({
    ...validScript,
    recording: { show_address_bar: true, capture_mode: "native-browser-window" },
  });
  assert.equal(script.recording.capture_mode, "native-browser-window");
});

test("validates recording split constraints for TikTok artifact uploads", () => {
  const script = validateScript({
    ...validScript,
    recording: { max_artifact_bytes: 50_000_000, split_automatically: true },
  });
  assert.equal(script.recording.max_artifact_bytes, 50_000_000);

  assert.throws(() => validateScript({
    ...validScript,
    recording: { max_artifact_bytes: -1, split_automatically: true },
  }), /recording\.max_artifact_bytes/);
});

test("validates optional segment metadata", () => {
  const script = validateScript({
    ...validScript,
    segments: [{ key: "posting_part_1", title: "Posting Part 1", scopes: ["user.info.basic", "video.upload"] }],
  });
  assert.equal(script.segments[0].key, "posting_part_1");

  assert.throws(() => validateScript({
    ...validScript,
    segments: [{ key: "", title: "Missing key" }],
  }), /segments\[0\]\.key/);
});
