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
  if (!Array.isArray(script.steps) || script.steps.length === 0) {
    throw new Error("script.steps must be a non-empty array");
  }
  script.steps.forEach((step, index) => validateStep(step, index));
  return script;
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
