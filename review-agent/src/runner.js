import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { startNativeBrowserCapture } from "./native-capture.js";
import { validateScript } from "./script-contract.js";
import { buildReviewSessionCookie } from "./session-cookie.js";

const PAGE_VIDEO_ARTIFACT_NOTE = "Fallback artifact captures the page viewport only. It does not satisfy address-bar evidence requirements.";

export async function runScript(script, { dryRun = false, out = process.stdout, reporter = null, sessionToken = "", playwrightImpl = null, nativeCaptureImpl = startNativeBrowserCapture } = {}) {
  const valid = validateScript(script);
  if (dryRun) {
    out.write(`Review script ${valid.job_id} (${valid.steps.length} steps)\n`);
    for (const step of valid.steps) {
      out.write(`- ${step.id}: ${step.action}\n`);
    }
    return { status: "dry_run", jobId: valid.job_id };
  }

  const { chromium } = playwrightImpl || await importPlaywright();
  const contextOptions = buildBrowserContextOptions(valid);
  await mkdir(contextOptions.recordVideo.dir, { recursive: true });
  const browser = await chromium.launch({ headless: false });
  const context = await browser.newContext(contextOptions);
  let contextClosed = false;
  const sessionCookie = buildReviewSessionCookie(valid, sessionToken);
  if (sessionCookie) {
    await context.addCookies([sessionCookie]);
  }
  const page = await context.newPage();
  let nativeCapture = null;
  const markers = [];
  const recordingStartedAt = Date.now();
  let currentStepId = "";
  try {
    try {
      nativeCapture = await nativeCaptureImpl({ script: valid, browser, page, out });
    } catch (err) {
      throw new Error(`Native browser-window recording is required for address-bar evidence but could not start. ${err.message}`);
    }
    await reportEvent(reporter, "recording_started", "Recorder started", {
      job_id: valid.job_id,
      capture_mode: nativeCapture?.mode || "playwright-page-video",
      includes_address_bar: Boolean(nativeCapture?.includesAddressBar),
    }, out);
    for (const step of valid.steps) {
      currentStepId = step.id;
      if (step.marker) markers.push({ step_id: step.id, label: step.marker, elapsed_ms: Date.now() - recordingStartedAt });
      await reportEvent(reporter, "step_started", step.marker || step.id, { step_id: step.id, action: step.action }, out);
      if (step.action === "manual_pause") {
        await reportEvent(reporter, "manual_pause", step.marker || "Waiting for user", { step_id: step.id }, out);
      }
      await runStep(page, step, out);
      await reportEvent(reporter, "step_completed", step.marker || step.id, { step_id: step.id, action: step.action }, out);
    }
    const nativeVideo = nativeCapture ? await stopNativeCapture(nativeCapture, out) : null;
    const video = nativeVideo ? null : page.video?.();
    await context.close();
    contextClosed = true;
    const artifacts = await buildCompletionArtifacts({ markers, video, nativeVideo });
    const videoFileID = await uploadVideoArtifact(reporter, artifacts.video, out);
    const evidenceFileID = await uploadExecutionEvidenceArtifact(reporter, valid.job_id, artifacts, out);
    if (evidenceFileID) artifacts.execution_evidence = { file_id: evidenceFileID };
    await reportComplete(reporter, artifacts, out, videoFileID);
    return { status: "completed", jobId: valid.job_id };
  } catch (err) {
    let nativeVideo = null;
    try {
      if (nativeCapture) nativeVideo = await stopNativeCapture(nativeCapture, out);
    } catch (captureErr) {
      out.write("[recording warning] " + captureErr.message + "\n");
    }
    const video = nativeVideo ? null : page.video?.();
    try {
      if (!contextClosed) {
        await context.close();
        contextClosed = true;
      }
    } catch (closeErr) {
      out.write("[recording warning] " + closeErr.message + "\n");
    }
    const artifacts = await buildCompletionArtifacts({ markers, video, nativeVideo });
    await reportFail(reporter, err, { ...artifacts, last_step: currentStepId }, out);
    throw err;
  } finally {
    if (!contextClosed) {
      await context.close();
    }
    await browser.close();
  }
}

export function buildBrowserContextOptions(script, { videoDir = defaultVideoDir() } = {}) {
  const width = script.recording?.window_width || 1440;
  const height = script.recording?.window_height || 1000;
  return {
    viewport: { width, height },
    recordVideo: {
      dir: videoDir,
      size: { width, height },
    },
  };
}

export async function buildCompletionArtifacts({ markers = [], video = null, nativeVideo = null } = {}) {
  const artifacts = { markers };
  if (nativeVideo?.local_path) {
    artifacts.video = {
      format: videoFormat(nativeVideo.local_path),
      local_path: nativeVideo.local_path,
      capture_mode: nativeVideo.capture_mode,
      includes_address_bar: nativeVideo.includes_address_bar,
      bounds: nativeVideo.bounds,
    };
    return artifacts;
  }
  if (!video?.path) return artifacts;
  const localPath = await video.path();
  if (!localPath) return artifacts;
  artifacts.video = {
    format: videoFormat(localPath),
    local_path: localPath,
    capture_mode: "playwright-page-video",
    includes_address_bar: false,
    note: PAGE_VIDEO_ARTIFACT_NOTE,
  };
  return artifacts;
}

function defaultVideoDir() {
  return process.env.UNIPOST_REVIEW_VIDEO_DIR || path.resolve(process.cwd(), ".unipost-review-videos");
}

function videoFormat(localPath) {
  const ext = path.extname(localPath).replace(/^\./, "");
  return ext || "webm";
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

async function stopNativeCapture(nativeCapture, out) {
  await nativeCapture.stop();
  out.write("[recording] native browser-window capture finalized\n");
  return {
    local_path: nativeCapture.localPath,
    capture_mode: nativeCapture.mode,
    includes_address_bar: Boolean(nativeCapture.includesAddressBar),
    bounds: nativeCapture.bounds,
  };
}

async function uploadExecutionEvidenceArtifact(reporter, jobId, artifacts, out) {
  if (!reporter?.uploadArtifact) return "";
  const evidenceDir = defaultVideoDir();
  await mkdir(evidenceDir, { recursive: true });
  const evidencePath = path.join(evidenceDir, `${jobId}-execution-evidence.json`);
  const evidence = buildExecutionEvidence({ jobId, artifacts });
  await writeFile(evidencePath, JSON.stringify(evidence, null, 2), "utf8");
  const fileID = await reporter.uploadArtifact({
    artifactType: "execution_evidence",
    contentType: "application/json",
    path: evidencePath,
  });
  out.write("[artifact] uploaded execution evidence: " + fileID + "\n");
  return fileID;
}

export function buildExecutionEvidence({ jobId, artifacts = {} } = {}) {
  return {
    job_id: jobId || "",
    generated_at: new Date().toISOString(),
    markers: artifacts.markers || [],
    video: artifacts.video ? {
      format: artifacts.video.format,
      capture_mode: artifacts.video.capture_mode,
      includes_address_bar: Boolean(artifacts.video.includes_address_bar),
      file_id: artifacts.video.file_id || "",
      note: artifacts.video.note || "",
    } : null,
  };
}

async function uploadVideoArtifact(reporter, video, out) {
  if (!reporter?.uploadArtifact || !video?.local_path) return "";
  const contentType = videoContentType(video.format);
  const fileID = await reporter.uploadArtifact({
    artifactType: "demo_video",
    contentType,
    path: video.local_path,
  });
  out.write("[artifact] uploaded demo video: " + fileID + "\n");
  video.file_id = fileID;
  return fileID;
}

function videoContentType(format) {
  if (format === "mp4") return "video/mp4";
  if (format === "mov" || format === "quicktime") return "video/quicktime";
  return "video/webm";
}

async function reportEvent(reporter, eventType, message, metadata, out) {
  if (!reporter?.event) return;
  try {
    await reporter.event(eventType, message, metadata);
  } catch (err) {
    out.write("[report warning] " + err.message + "\n");
  }
}

async function reportComplete(reporter, artifacts, out, videoFileID = "") {
  if (!reporter?.complete) return;
  try {
    await reporter.complete(artifacts, videoFileID);
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
