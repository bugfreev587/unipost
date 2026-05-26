import { validateScript } from "./script-contract.js";
import { buildReviewSessionCookie } from "./session-cookie.js";

export async function runScript(script, { dryRun = false, out = process.stdout, reporter = null, sessionToken = "" } = {}) {
  const valid = validateScript(script);
  if (dryRun) {
    out.write(`Review script ${valid.job_id} (${valid.steps.length} steps)\n`);
    for (const step of valid.steps) {
      out.write(`- ${step.id}: ${step.action}\n`);
    }
    return { status: "dry_run", jobId: valid.job_id };
  }

  const { chromium } = await importPlaywright();
  const browser = await chromium.launch({ headless: false });
  const context = await browser.newContext({
    viewport: {
      width: valid.recording?.window_width || 1440,
      height: valid.recording?.window_height || 1000,
    },
    recordVideo: undefined,
  });
  const sessionCookie = buildReviewSessionCookie(valid, sessionToken);
  if (sessionCookie) {
    await context.addCookies([sessionCookie]);
  }
  const page = await context.newPage();
  const markers = [];
  let currentStepId = "";
  await reportEvent(reporter, "recording_started", "Recorder started", { job_id: valid.job_id }, out);
  try {
    for (const step of valid.steps) {
      currentStepId = step.id;
      if (step.marker) markers.push({ step_id: step.id, label: step.marker });
      await reportEvent(reporter, "step_started", step.marker || step.id, { step_id: step.id, action: step.action }, out);
      if (step.action === "manual_pause") {
        await reportEvent(reporter, "manual_pause", step.marker || "Waiting for user", { step_id: step.id }, out);
      }
      await runStep(page, step, out);
      await reportEvent(reporter, "step_completed", step.marker || step.id, { step_id: step.id, action: step.action }, out);
    }
    await reportComplete(reporter, { markers }, out);
    return { status: "completed", jobId: valid.job_id };
  } catch (err) {
    await reportFail(reporter, err, { last_step: currentStepId, markers }, out);
    throw err;
  } finally {
    await context.close();
    await browser.close();
  }
}

async function importPlaywright() {
  try {
    return await import("playwright");
  } catch (err) {
    throw new Error(`Playwright is required for recording. Run: npx playwright install chromium. Details: ${err.message}`);
  }
}

async function runStep(page, step, out) {
  if (step.marker) {
    out.write(`[marker] ${step.marker}\n`);
  }
  switch (step.action) {
    case "emit_marker":
      return;
    case "goto":
      await page.goto(step.url, { waitUntil: "domcontentloaded" });
      return;
    case "click":
      await page.locator(step.selector).click();
      return;
    case "fill":
      await page.locator(step.selector).fill(step.value || "");
      return;
    case "assert_visible":
      await page.locator(step.selector).first().waitFor({ state: "visible", timeout: 30000 });
      return;
    case "assert_url_contains":
      if (!page.url().includes(step.value || step.text || "")) {
        throw new Error(`URL assertion failed for ${step.id}: ${page.url()}`);
      }
      return;
    case "manual_pause":
      await showOverlay(page, step.overlay || "Complete the manual step, then UniPost will continue.");
      if (step.resume_when_url_contains) {
        await page.waitForURL((url) => url.toString().includes(step.resume_when_url_contains), { timeout: 10 * 60 * 1000 });
      }
      await hideOverlay(page);
      return;
    case "wait_for_navigation":
      await page.waitForLoadState("domcontentloaded");
      return;
    case "wait_for_network_idle":
      await page.waitForLoadState("networkidle");
      return;
    case "screenshot":
      await page.screenshot({ path: step.value || `${step.id}.png`, fullPage: true });
      return;
    default:
      throw new Error(`unsupported action after validation: ${step.action}`);
  }
}

async function showOverlay(page, text) {
  await page.evaluate((message) => {
    const previous = document.querySelector("[data-unipost-review-overlay]");
    if (previous) previous.remove();
    const overlay = document.createElement("div");
    overlay.setAttribute("data-unipost-review-overlay", "true");
    overlay.textContent = message;
    Object.assign(overlay.style, {
      position: "fixed",
      left: "24px",
      right: "24px",
      bottom: "24px",
      zIndex: "2147483647",
      padding: "14px 16px",
      borderRadius: "8px",
      background: "#111827",
      color: "white",
      fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, sans-serif",
      fontSize: "14px",
      lineHeight: "1.45",
      boxShadow: "0 18px 60px rgba(0,0,0,.28)",
    });
    document.body.appendChild(overlay);
  }, text);
}

async function hideOverlay(page) {
  await page.evaluate(() => document.querySelector("[data-unipost-review-overlay]")?.remove());
}

async function reportEvent(reporter, eventType, message, metadata, out) {
  if (!reporter?.event) return;
  try {
    await reporter.event(eventType, message, metadata);
  } catch (err) {
    out.write("[report warning] " + err.message + "\n");
  }
}

async function reportComplete(reporter, artifacts, out) {
  if (!reporter?.complete) return;
  try {
    await reporter.complete(artifacts);
  } catch (err) {
    out.write("[report warning] " + err.message + "\n");
  }
}

async function reportFail(reporter, err, artifacts, out) {
  if (!reporter?.fail) return;
  try {
    await reporter.fail(err, artifacts);
  } catch (reportErr) {
    out.write("[report warning] " + reportErr.message + "\n");
  }
}
