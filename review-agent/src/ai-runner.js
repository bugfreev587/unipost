import { requestNextReviewAction } from "./client.js";
import { collectPageObservation } from "./observation.js";

const LOCAL_ALLOWED_ACTIONS = new Set([
  "navigate",
  "click",
  "type",
  "upload_file",
  "scroll",
  "wait",
  "assert",
  "pause_for_user",
  "open_link",
  "return_to_review_page",
]);

export async function runAIGuidedScript({
  script,
  page,
  token = "",
  apiUrl = "",
  reporter = null,
  nextActionImpl = null,
  uploadFilePath = "",
} = {}) {
  const nextAction =
    nextActionImpl ||
    ((request) =>
      requestNextReviewAction({
        token,
        apiUrl,
        request,
      }));

  for (const step of script?.steps || []) {
    const observation = await collectPageObservation(page, { jobId: script.job_id, stepKey: step.id });
    await reporter?.event?.("ai_observation_captured", step.marker || step.id, { step_id: step.id });
    const action = await nextAction({
      step_key: step.id,
      goal: step.goal || step.marker || step.id,
      observation,
    });

    await executeAIAction(page, action, { uploadFilePath });
    const hold = Number(action.hold_ms_after_action || 1800);
    if (typeof page.waitForTimeout === "function" && hold > 0) {
      await page.waitForTimeout(Math.min(30000, hold));
    }
    await reporter?.event?.("ai_action_completed", action.action, { step_id: step.id, action: action.action });
  }
}

export async function executeAIAction(page, action = {}, { uploadFilePath = "" } = {}) {
  if (!LOCAL_ALLOWED_ACTIONS.has(action.action)) {
    throw new Error(`AI action ${action.action || ""} is not supported by the local agent`);
  }

  switch (action.action) {
    case "click":
      await page.locator(requiredSelector(action)).click();
      return;
    case "type":
      await page.locator(requiredSelector(action)).fill(action.value || "");
      return;
    case "upload_file":
      await page.locator(requiredSelector(action)).setInputFiles(action.value || uploadFilePath);
      return;
    case "scroll":
      await page.mouse?.wheel?.(0, Number(action.value || 600));
      return;
    case "wait":
      await page.waitForTimeout?.(Number(action.value || action.hold_ms_after_action || 1500));
      return;
    case "navigate":
      await page.goto(requiredValue(action, "navigate URL"), { waitUntil: "domcontentloaded" });
      return;
    case "assert":
      await page.locator(requiredSelector(action)).first().waitFor({ state: "visible", timeout: 30000 });
      return;
    case "pause_for_user":
      return;
    case "open_link":
      await page.locator(requiredSelector(action)).click();
      return;
    case "return_to_review_page":
      await page.bringToFront?.();
      return;
    default:
      throw new Error(`AI action ${action.action || ""} has no executor`);
  }
}

function requiredSelector(action) {
  const selector = String(action.target?.selector || "").trim();
  if (!selector) {
    throw new Error(`AI action ${action.action || ""} requires target.selector`);
  }
  return selector;
}

function requiredValue(action, label) {
  const value = String(action.value || "").trim();
  if (!value) {
    throw new Error(`AI action ${action.action || ""} requires ${label}`);
  }
  return value;
}
