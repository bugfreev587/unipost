import { validateScript } from "./script-contract.js";

export async function runScript(script, { dryRun = false, out = process.stdout } = {}) {
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
  const page = await context.newPage();
  try {
    for (const step of valid.steps) {
      await runStep(page, step, out);
    }
    return { status: "completed", jobId: valid.job_id };
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
