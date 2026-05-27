export const ALLOWED_ACTIONS = Object.freeze(new Set([
  "goto",
  "click",
  "fill",
  "assert_visible",
  "assert_url_contains",
  "manual_pause",
  "wait_for_navigation",
  "wait_for_network_idle",
  "screenshot",
  "emit_marker",
]));

const SELECTOR_ACTIONS = new Set(["click", "fill", "assert_visible"]);
const ALLOWED_CAPTURE_MODES = new Set(["native-browser-window", "playwright-page-video"]);

export function validateScript(script) {
  if (!script || typeof script !== "object") {
    throw new Error("script must be an object");
  }
  if (!isNonEmptyString(script.job_id)) {
    throw new Error("script.job_id is required");
  }
  if (script.platform !== "tiktok") {
    throw new Error(`unsupported platform: ${script.platform || ""}`);
  }
  if (!isNonEmptyString(script.agent_version)) {
    throw new Error("script.agent_version is required");
  }
  if (!isNonEmptyString(script.start_url) || !isHttpURL(script.start_url)) {
    throw new Error("script.start_url must be an http(s) URL");
  }
  validateRecording(script.recording || {});
  if (!Array.isArray(script.steps) || script.steps.length === 0) {
    throw new Error("script.steps must be a non-empty array");
  }
  if (script.segments !== undefined) {
    if (!Array.isArray(script.segments)) {
      throw new Error("script.segments must be an array");
    }
    script.segments.forEach((segment, index) => validateSegment(segment, index));
  }
  script.steps.forEach((step, index) => validateStep(step, index));
  return script;
}

function validateSegment(segment, index) {
  if (!segment || typeof segment !== "object") {
    throw new Error(`segments[${index}] must be an object`);
  }
  if (!isNonEmptyString(segment.key)) {
    throw new Error(`segments[${index}].key is required`);
  }
  if (!isNonEmptyString(segment.title)) {
    throw new Error(`segments[${index}].title is required`);
  }
  if (segment.scopes !== undefined && !Array.isArray(segment.scopes)) {
    throw new Error(`segments[${index}].scopes must be an array`);
  }
}

export function validateStep(step, index) {
  if (!step || typeof step !== "object") {
    throw new Error(`steps[${index}] must be an object`);
  }
  if (!isNonEmptyString(step.id)) {
    throw new Error(`steps[${index}].id is required`);
  }
  if (!ALLOWED_ACTIONS.has(step.action)) {
    throw new Error(`steps[${index}].action is not allowed: ${step.action || ""}`);
  }
  if (step.action === "goto" && (!isNonEmptyString(step.url) || !isHttpURL(step.url))) {
    throw new Error(`steps[${index}].url must be an http(s) URL`);
  }
  if (SELECTOR_ACTIONS.has(step.action) && !isNonEmptyString(step.selector)) {
    throw new Error(`steps[${index}].selector is required for ${step.action}`);
  }
}

function validateRecording(recording) {
  if (recording.capture_mode && !ALLOWED_CAPTURE_MODES.has(recording.capture_mode)) {
    throw new Error(`recording.capture_mode is not allowed: ${recording.capture_mode}`);
  }
  if (recording.show_address_bar && recording.capture_mode === "playwright-page-video") {
    throw new Error("recording.show_address_bar requires native-browser-window capture");
  }
  if (recording.max_artifact_bytes !== undefined && (!Number.isFinite(recording.max_artifact_bytes) || recording.max_artifact_bytes <= 0)) {
    throw new Error("recording.max_artifact_bytes must be a positive number");
  }
  if (recording.split_automatically !== undefined && typeof recording.split_automatically !== "boolean") {
    throw new Error("recording.split_automatically must be a boolean");
  }
}

function isNonEmptyString(value) {
  return typeof value === "string" && value.trim().length > 0;
}

function isHttpURL(value) {
  try {
    const parsed = new URL(value);
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}
