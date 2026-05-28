import { mkdir, writeFile } from "node:fs/promises";
import { execFile } from "node:child_process";
import path from "node:path";
import { promisify } from "node:util";
import { runAIGuidedScript } from "./ai-runner.js";
import { startNativeBrowserCapture } from "./native-capture.js";
import { validateScript } from "./script-contract.js";
import { buildReviewSessionCookie } from "./session-cookie.js";
import { processReviewVideoSegments } from "./video-postprocess.js";

const PAGE_VIDEO_ARTIFACT_NOTE = "Fallback artifact captures the page viewport only. It does not satisfy address-bar evidence requirements.";
const execFileAsync = promisify(execFile);

export async function runScript(script, { dryRun = false, out = process.stdout, reporter = null, sessionToken = "", manualOAuthHandoff = manualOAuthHandoffEnabled(), manualOAuthPollMs = null, manualOAuthTimeoutMs = null, playwrightImpl = null, nativeCaptureImpl = startNativeBrowserCapture, videoPostProcessImpl = processReviewVideoSegments, prepareBrowserImpl = prepareBrowserForNativeCapture, aiGuided = false, aiRunnerImpl = runAIGuidedScript, token = "", apiUrl = "", uploadFilePath = "" } = {}) {
  const valid = validateScript(script);
  if (dryRun) {
    out.write(`Review script ${valid.job_id} (${valid.steps.length} steps)\n`);
    for (const step of valid.steps) {
      out.write(`- ${step.id}: ${step.action}\n`);
    }
    return { status: "dry_run", jobId: valid.job_id };
  }

  await prepareBrowserImpl({ script: valid, out });
  const { chromium } = playwrightImpl || await importPlaywright();
  const contextOptions = buildBrowserContextOptions(valid);
  if (contextOptions.recordVideo?.dir) {
    await mkdir(contextOptions.recordVideo.dir, { recursive: true });
  }
  const browser = await chromium.launch(browserLaunchOptions());
  const context = await browser.newContext(contextOptions);
  let contextClosed = false;
  const sessionCookie = buildReviewSessionCookie(valid, sessionToken);
  if (sessionCookie) {
    await context.addCookies([sessionCookie]);
  }
  const page = await context.newPage();
  await bringPageToFront(page);
  await preloadNativeCapturePage(page, valid);
  let nativeCapture = null;
  const markers = [];
  const segmentMap = buildSegmentMap(valid.segments || []);
  const segmentEvents = [];
  const recordingStartedAt = Date.now();
  let currentStepId = "";
  let activeSegment = null;
  const runtime = { manualOAuthCompleted: false };
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
      if (isSegmentMarker(step)) {
        if (activeSegment) {
          const completedAt = Date.now() - recordingStartedAt;
          segmentEvents.push({ ...activeSegment, completed_elapsed_ms: completedAt });
          await reportEvent(reporter, "segment_completed", activeSegment.title, activeSegment, out);
        }
        activeSegment = segmentMetadataForStep(step, segmentMap, recordingStartedAt);
        segmentEvents.push(activeSegment);
        await reportEvent(reporter, "segment_started", activeSegment.title, activeSegment, out);
      }
      if (step.marker && step.action !== "manual_pause") {
        await showMarkerOverlay(page, {
          stepId: step.id,
          label: step.marker,
          durationMs: markerOverlayDurationMs(valid.recording || {}),
        });
      }
      await reportEvent(reporter, "step_started", step.marker || step.id, { step_id: step.id, action: step.action }, out);
      if (step.action === "manual_pause") {
        await reportEvent(reporter, "manual_pause_started", step.marker || "Waiting for user", { step_id: step.id }, out);
      }
      if (aiGuided) {
        await aiRunnerImpl({
          script: { ...valid, steps: [step] },
          page,
          token,
          apiUrl,
          reporter,
          uploadFilePath,
        });
      } else {
        await runStep(page, step, out, { reporter, script: valid, manualOAuthHandoff, manualOAuthPollMs, manualOAuthTimeoutMs, runtime });
      }
      if (step.action === "manual_pause") {
        await reportEvent(reporter, "manual_pause_completed", step.marker || "User step completed", { step_id: step.id }, out);
      }
      await reportEvent(reporter, "step_completed", step.marker || step.id, { step_id: step.id, action: step.action }, out);
    }
    if (activeSegment) {
      const completedAt = Date.now() - recordingStartedAt;
      segmentEvents.push({ ...activeSegment, completed_elapsed_ms: completedAt });
      await reportEvent(reporter, "segment_completed", activeSegment.title, activeSegment, out);
    }
    const nativeVideo = nativeCapture ? await stopNativeCapture(nativeCapture, out) : null;
    const video = nativeVideo ? null : page.video?.();
    await context.close();
    contextClosed = true;
    const artifacts = await buildCompletionArtifacts({ markers, segments: valid.segments || [], segmentEvents, video, nativeVideo });
    await maybePostProcessVideoSegments({ script: valid, artifacts, segmentEvents, videoPostProcessImpl, out });
    const videoFileID = await uploadVideoArtifacts(reporter, artifacts, out);
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
    const artifacts = await buildCompletionArtifacts({ markers, segments: valid.segments || [], segmentEvents, video, nativeVideo });
    await reportFail(reporter, err, { ...artifacts, last_step: currentStepId }, out);
    throw err;
  } finally {
    if (!contextClosed) {
      await context.close();
    }
    await browser.close();
  }
}

export async function prepareBrowserForNativeCapture({ script = {}, out = process.stdout, platform = process.platform, execFileImpl = execFileAsync, delayImpl = delay } = {}) {
  const recording = script.recording || {};
  const requestedMode = recording.capture_mode || (recording.show_address_bar ? "native-browser-window" : "playwright-page-video");
  if (platform !== "darwin" || requestedMode !== "native-browser-window" || !recording.show_address_bar) return;
  try {
    if (!await isChromeForTestingRunning(execFileImpl)) return;
    await execFileImpl("osascript", ["-e", 'tell application "Google Chrome for Testing" to quit']);
    for (let attempt = 0; attempt < 20; attempt += 1) {
      if (!await isChromeForTestingRunning(execFileImpl)) {
        out.write("[browser] cleared stale Chrome for Testing windows before native capture\n");
        return;
      }
      await delayImpl(250);
    }
    out.write("[browser warning] Chrome for Testing was still running after quit request; recording may capture the wrong window\n");
  } catch (err) {
    out.write("[browser warning] could not clear stale Chrome for Testing windows: " + err.message + "\n");
  }
}

async function isChromeForTestingRunning(execFileImpl) {
  const result = await execFileImpl("osascript", [
    "-e",
    'tell application "System Events" to exists process "Google Chrome for Testing"',
  ]);
  return String(result?.stdout || "").trim() === "true";
}

export function buildBrowserContextOptions(script, { videoDir = defaultVideoDir() } = {}) {
  const width = script.recording?.window_width || 1440;
  const height = script.recording?.window_height || 1000;
  const options = {
    viewport: { width, height },
  };
  if (!requiresNativeAddressBarCapture(script.recording || {})) {
    options.recordVideo = {
      dir: videoDir,
      size: { width, height },
    };
  }
  return options;
}

function requiresNativeAddressBarCapture(recording = {}) {
  const requestedMode = recording.capture_mode || (recording.show_address_bar ? "native-browser-window" : "playwright-page-video");
  return Boolean(recording.show_address_bar) && requestedMode === "native-browser-window";
}

export async function buildCompletionArtifacts({ markers = [], segments = [], segmentEvents = [], videoSegments = [], video = null, nativeVideo = null } = {}) {
  const artifacts = { markers };
  if (segments.length) artifacts.segments = segments;
  if (segmentEvents.length) artifacts.segment_events = segmentEvents;
  if (videoSegments.length) {
    artifacts.video_segments = videoSegments
      .filter((segment) => segment?.local_path)
      .map((segment) => ({
        ...segment,
        segment_key: segment.segment_key || segment.key || "",
        format: segment.format || videoFormat(segment.local_path),
      }));
  }
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

async function maybePostProcessVideoSegments({ script, artifacts, segmentEvents, videoPostProcessImpl, out }) {
  if (!videoPostProcessImpl || !artifacts?.video?.local_path || !script?.recording?.split_automatically) return;
  if (!Array.isArray(script.segments) || script.segments.length === 0) return;
  const videoSegments = await videoPostProcessImpl({
    sourceVideo: artifacts.video,
    segments: script.segments,
    segmentEvents,
    maxBytes: recordingMaxArtifactBytes(script.recording),
    out,
  });
  if (Array.isArray(videoSegments) && videoSegments.length > 0) {
    artifacts.video_segments = videoSegments.map((segment) => ({
      ...segment,
      segment_key: segment.segment_key || segment.key || "",
      format: segment.format || videoFormat(segment.local_path || ""),
    }));
  }
}

function recordingMaxArtifactBytes(recording = {}) {
  const configured = Number(recording.max_artifact_bytes || 0);
  if (Number.isFinite(configured) && configured > 0) return configured;
  return 50_000_000;
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

async function runStep(page, step, out, { reporter = null, script = {}, manualOAuthHandoff = false, manualOAuthPollMs = null, manualOAuthTimeoutMs = null, runtime = {} } = {}) {
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
      if (step.id === "connect_tiktok" && manualOAuthHandoff) {
        await runManualTikTokOAuthHandoff(page, step, out, { reporter, script, pollMs: manualOAuthPollMs, timeoutMs: manualOAuthTimeoutMs, runtime });
        return;
      }
      await page.locator(step.selector).click();
      if (step.id === "connect_tiktok") {
        const oauth = await normalizeTikTokReviewOAuthScopes(page, out, script.requested_scopes || []);
        if (oauth.seen) {
          await reportEvent(reporter, "oauth_consent_seen", "TikTok OAuth authorization page was shown", { step_id: step.id, scopes: oauth.scopes }, out);
        }
        if (oauth.skipped) {
          await reportEvent(reporter, "oauth_consent_skipped", "TikTok skipped the authorization page", { step_id: step.id, scopes: oauth.scopes }, out);
          throw new Error("TikTok skipped the authorization page because this account is already authorized. Remove app access in TikTok mobile settings, then record again.");
        }
      }
      await holdForReview(page, script.recording || {});
      return;
    case "fill":
      await page.locator(step.selector).fill(step.value || "");
      await holdForReview(page, script.recording || {});
      return;
    case "assert_visible":
      await page.locator(step.selector).first().waitFor({ state: "visible", timeout: 30000 });
      await holdForReview(page, script.recording || {});
      return;
    case "open_link":
      await openLinkForReview(page, step, out, reporter, script.recording || {});
      await holdForReview(page, script.recording || {});
      return;
    case "assert_url_contains":
      if (!page.url().includes(step.value || step.text || "")) {
        throw new Error(`URL assertion failed for ${step.id}: ${page.url()}`);
      }
      return;
    case "manual_pause":
      if (runtime.manualOAuthCompleted && step.id === "wait_for_oauth") {
        return;
      }
      await showOverlay(page, step.overlay || "Complete the manual step, then UniPost will continue.");
      if (step.resume_when_url_contains) {
        await page.waitForURL((url) => url.toString().includes(step.resume_when_url_contains), { timeout: manualPauseTimeoutMs() });
      }
      await hideOverlay(page);
      return;
    case "wait_for_navigation":
      await page.waitForLoadState("domcontentloaded");
      await holdForReview(page, script.recording || {});
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

async function openLinkForReview(page, step, out, reporter, recording = {}) {
  const expectedURLPart = step.value || step.text || step.url || "";
  const popupPromise = typeof page.waitForEvent === "function"
    ? page.waitForEvent("popup", { timeout: 8000 }).catch(() => null)
    : Promise.resolve(null);
  const [popup] = await Promise.all([
    popupPromise,
    page.locator(step.selector).click(),
  ]);
  const targetPage = popup || page;
  if (typeof targetPage.waitForLoadState === "function") {
    await targetPage.waitForLoadState("domcontentloaded", { timeout: 15000 }).catch(() => {});
  }
  const actualURL = typeof targetPage.url === "function" ? targetPage.url() : "";
  if (expectedURLPart && !actualURL.includes(expectedURLPart)) {
    throw new Error(`Policy link assertion failed for ${step.id}: expected URL containing ${expectedURLPart}, got ${actualURL}`);
  }
  await reportEvent(reporter, "external_policy_opened", step.marker || "Opened TikTok policy link", {
    step_id: step.id,
    selector: step.selector,
    url: actualURL,
    expected_url_part: expectedURLPart,
  }, out);
  await delay(policyLinkHoldDurationMs(recording));
  if (popup && typeof popup.close === "function") {
    await popup.close();
    await bringPageToFront(page);
    return;
  }
  if (!popup && typeof page.goBack === "function") {
    await page.goBack({ waitUntil: "domcontentloaded" }).catch(() => {});
    await bringPageToFront(page);
  }
}

async function bringPageToFront(page) {
  if (typeof page?.bringToFront === "function") {
    await page.bringToFront();
  }
}

async function preloadNativeCapturePage(page, script) {
  if (!requiresNativeAddressBarCapture(script.recording || {})) return;
  if (typeof page?.goto !== "function") return;
  const firstStep = Array.isArray(script.steps) ? script.steps[0] : null;
  const preloadURL = firstStep?.action === "goto" ? firstStep.url : script.start_url;
  if (!preloadURL) return;
  await page.goto(preloadURL, { waitUntil: "domcontentloaded" });
  await bringPageToFront(page);
}

export function policyLinkHoldDurationMs(recording = {}) {
  const configured = Number(recording.policy_link_hold_ms);
  if (Number.isFinite(configured) && configured >= 0) return configured;
  return 1200;
}

async function showOverlay(page, text) {
  await page.waitForLoadState("domcontentloaded", { timeout: 15000 }).catch(() => {});
  await page.locator("body").first().waitFor({ state: "attached", timeout: 15000 }).catch(() => {});
  await page.evaluate((message) => {
    const previous = document.querySelector("[data-unipost-review-overlay]");
    if (previous) previous.remove();
    const mount = document.body || document.documentElement;
    if (!mount) {
      throw new Error("review overlay cannot be mounted before the document is ready");
    }
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
    mount.appendChild(overlay);
  }, text);
}

async function hideOverlay(page) {
  await page.evaluate(() => document.querySelector("[data-unipost-review-overlay]")?.remove());
}

async function showMarkerOverlay(page, { stepId, label, durationMs }) {
  if (!page?.evaluate) return;
  await page.evaluate(({ stepId, label }) => {
    const previous = document.querySelector("[data-unipost-review-marker]");
    if (previous) previous.remove();
    const mount = document.body || document.documentElement;
    if (!mount) return;
    const overlay = document.createElement("div");
    overlay.setAttribute("data-unipost-review-marker", "true");
    overlay.setAttribute("data-review-marker-step", stepId);
    const kicker = document.createElement("div");
    kicker.textContent = "TikTok App Review Demo";
    const title = document.createElement("div");
    title.textContent = label;
    overlay.append(kicker, title);
    Object.assign(overlay.style, {
      position: "fixed",
      left: "28px",
      top: "28px",
      zIndex: "2147483646",
      maxWidth: "760px",
      padding: "16px 18px",
      borderRadius: "8px",
      background: "rgba(17,24,39,.94)",
      color: "#fff",
      fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, sans-serif",
      boxShadow: "0 20px 56px rgba(15,23,42,.30)",
      border: "1px solid rgba(255,255,255,.18)",
    });
    Object.assign(kicker.style, {
      fontSize: "12px",
      letterSpacing: ".08em",
      textTransform: "uppercase",
      color: "rgba(255,255,255,.72)",
      fontWeight: "700",
      marginBottom: "5px",
    });
    Object.assign(title.style, {
      fontSize: "26px",
      lineHeight: "1.18",
      fontWeight: "760",
    });
    mount.appendChild(overlay);
  }, { stepId, label });
  await delay(durationMs);
  await page.evaluate(({ stepId }) => {
    const overlay = document.querySelector(`[data-unipost-review-marker][data-review-marker-step="${stepId}"]`);
    overlay?.remove();
  }, { stepId, remove: true });
}

function markerOverlayDurationMs(recording = {}) {
  const configured = Number(recording.marker_overlay_ms || "");
  if (Number.isFinite(configured) && configured >= 0) return configured;
  return 1300;
}

async function holdForReview(page, recording = {}) {
  const ms = stepHoldDurationMs(recording);
  if (ms <= 0) return;
  if (typeof page?.waitForTimeout === "function") {
    await page.waitForTimeout(ms);
    return;
  }
  await delay(ms);
}

function stepHoldDurationMs(recording = {}) {
  const configured = Number(recording.step_hold_ms || "");
  if (Number.isFinite(configured) && configured >= 0) return configured;
  return 1800;
}

function delay(ms) {
  if (!ms) return Promise.resolve();
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function manualPauseTimeoutMs() {
  const configured = Number(process.env.UNIPOST_REVIEW_MANUAL_PAUSE_TIMEOUT_MS || "");
  if (Number.isFinite(configured) && configured > 0) return configured;
  return 30 * 60 * 1000;
}

export function browserLaunchOptions({ browserChannel = process.env.UNIPOST_REVIEW_BROWSER_CHANNEL || "" } = {}) {
  const channel = String(browserChannel || "").trim();
  return {
    headless: false,
    ...(channel ? { channel } : {}),
  };
}

async function runManualTikTokOAuthHandoff(page, step, out, { reporter = null, script = {}, pollMs = null, timeoutMs = null, runtime = {} } = {}) {
  const authorizeURL = await manualTikTokAuthorizeURL(page, step.selector);
  if (!authorizeURL) {
    throw new Error("Manual TikTok OAuth handoff could not find the authorize URL on the review page.");
  }
  const scopes = normalizeRequestedScopes(script.requested_scopes || []);
  out.write("[oauth] Open this URL in a real Chrome Incognito window and complete TikTok login/authorization:\n");
  out.write(authorizeURL + "\n");
  out.write("[oauth] UniPost will keep recording this review window and continue after the account is connected.\n");
  await reportEvent(reporter, "manual_oauth_handoff_started", "Manual TikTok OAuth handoff started", {
    step_id: step.id,
    scopes,
    authorize_host: safeURLHost(authorizeURL),
  }, out);
  await showOverlay(page, "Open the TikTok authorization URL from the terminal in a real Chrome Incognito window. Log in and approve access there; UniPost will continue automatically after the review page detects the connected account.");
  await waitForManualTikTokConnection(page, { pollMs, timeoutMs });
  runtime.manualOAuthCompleted = true;
  await bringPageToFront(page);
  await hideOverlay(page);
  await reportEvent(reporter, "manual_oauth_handoff_completed", "Manual TikTok OAuth handoff completed", {
    step_id: step.id,
    scopes,
  }, out);
}

async function manualTikTokAuthorizeURL(page, selector) {
  let href = "";
  try {
    href = await page.locator(selector).getAttribute("href", { timeout: 10000 });
  } catch {
    return "";
  }
  if (!href || href === "#") return "";
  try {
    return new URL(href, typeof page.url === "function" ? page.url() : undefined).toString();
  } catch {
    return "";
  }
}

async function waitForManualTikTokConnection(page, { pollMs = null, timeoutMs = null } = {}) {
  const deadline = Date.now() + manualOAuthHandoffTimeoutMs(timeoutMs);
  while (Date.now() <= deadline) {
    if (await reviewPageShowsConnectedTikTok(page)) return;
    if (typeof page.reload === "function") {
      await page.reload({ waitUntil: "domcontentloaded" }).catch(() => {});
    }
    await delay(manualOAuthHandoffPollMs(pollMs));
  }
  throw new Error("Timed out waiting for TikTok OAuth to complete in the manual Incognito handoff. Confirm the TikTok authorization finished and retry the recording.");
}

async function reviewPageShowsConnectedTikTok(page) {
  const panelSelectors = [
    "[data-review-step='connect-tiktok-panel']",
    "[data-review-step='analytics-loading']",
  ];
  for (const selector of panelSelectors) {
    try {
      const text = await page.locator(selector).first().innerText({ timeout: 1000 });
      if (/TikTok account connected/i.test(text)) return true;
    } catch {
      // Try the next reviewer page shape.
    }
  }
  try {
    const connect = page.locator("[data-review-step='connect-tiktok']");
    if (typeof connect.count === "function" && await connect.count() === 0) return true;
  } catch {
    // Absence alone is not conclusive unless Playwright reports a zero count.
  }
  return false;
}

function manualOAuthHandoffEnabled(value = process.env.UNIPOST_REVIEW_MANUAL_OAUTH_HANDOFF || "") {
  if (value === true) return true;
  const normalized = String(value || "").trim().toLowerCase();
  return normalized === "1" || normalized === "true" || normalized === "yes";
}

function manualOAuthHandoffPollMs(configured) {
  const value = configured ?? process.env.UNIPOST_REVIEW_MANUAL_OAUTH_POLL_MS;
  const parsed = Number(value || "");
  if (Number.isFinite(parsed) && parsed > 0) return parsed;
  return 2000;
}

function manualOAuthHandoffTimeoutMs(configured) {
  const value = configured ?? process.env.UNIPOST_REVIEW_MANUAL_OAUTH_TIMEOUT_MS;
  const parsed = Number(value || "");
  if (Number.isFinite(parsed) && parsed > 0) return parsed;
  return 30 * 60 * 1000;
}

function safeURLHost(rawURL) {
  try {
    return new URL(rawURL).host;
  } catch {
    return "";
  }
}

async function normalizeTikTokReviewOAuthScopes(page, out, requestedScopes = []) {
  const scopes = normalizeRequestedScopes(requestedScopes);
  try {
    await page.waitForURL((url) => url.hostname.endsWith("tiktok.com"), { timeout: 15000 });
    const current = page.url();
    const normalized = normalizeTikTokReviewOAuthURL(current, scopes);
    if (normalized && normalized !== current) {
      out.write("[oauth] normalized TikTok review scopes for this review plan\n");
      await page.goto(normalized, { waitUntil: "domcontentloaded" });
    }
    return { seen: true, skipped: false, scopes };
  } catch (err) {
    const current = typeof page.url === "function" ? page.url() : "";
    if (current.includes("connect_status=success")) {
      return { seen: false, skipped: true, scopes };
    }
    out.write("[oauth warning] " + err.message + "\n");
    return { seen: false, skipped: false, scopes };
  }
}

export function normalizeTikTokReviewOAuthURL(rawURL, requestedScopes = []) {
  let parsed;
  try {
    parsed = new URL(rawURL);
  } catch {
    return "";
  }
  if (!parsed.hostname.endsWith("tiktok.com")) return "";
  const reviewScopes = normalizeRequestedScopes(requestedScopes).join(",");
  if (parsed.pathname.startsWith("/v2/auth/authorize")) {
    if (parsed.searchParams.get("scope") === reviewScopes) return rawURL;
    parsed.searchParams.set("scope", reviewScopes);
    return parsed.toString();
  }
  const redirectURL = parsed.searchParams.get("redirect_url");
  if (!redirectURL) return "";
  let nested;
  try {
    nested = new URL(redirectURL);
  } catch {
    return "";
  }
  if (!nested.hostname.endsWith("tiktok.com") || !nested.pathname.startsWith("/v2/auth/authorize")) return "";
  if (nested.searchParams.get("scope") === reviewScopes) return rawURL;
  nested.searchParams.set("scope", reviewScopes);
  parsed.searchParams.set("redirect_url", nested.toString());
  return parsed.toString();
}

export function normalizeRequestedScopes(scopes = []) {
  const selected = [...new Set((Array.isArray(scopes) ? scopes : [])
    .map((scope) => String(scope || "").trim())
    .filter(Boolean))];
  if (selected.length === 0) return ["video.publish", "video.upload", "user.info.basic"];
  return selected;
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
    segments: artifacts.segments || [],
    segment_events: artifacts.segment_events || [],
    video: artifacts.video ? {
      format: artifacts.video.format,
      capture_mode: artifacts.video.capture_mode,
      includes_address_bar: Boolean(artifacts.video.includes_address_bar),
      file_id: artifacts.video.file_id || "",
      note: artifacts.video.note || "",
    } : null,
    video_segments: (artifacts.video_segments || []).map((segment) => ({
      segment_key: segment.segment_key || "",
      title: segment.title || "",
      filename: segment.filename || "",
      format: segment.format || "",
      scopes: segment.scopes || [],
      start_sec: segment.start_sec || 0,
      duration_sec: segment.duration_sec || 0,
      resolution: segment.resolution || "",
      size_bytes: segment.size_bytes || 0,
      file_id: segment.file_id || "",
    })),
  };
}

function buildSegmentMap(segments) {
  const map = new Map();
  for (const segment of segments || []) {
    if (segment?.key) map.set(segment.key, segment);
  }
  return map;
}

function isSegmentMarker(step) {
  return step?.action === "emit_marker" && typeof step.id === "string" && step.id.startsWith("segment_");
}

function segmentMetadataForStep(step, segmentMap, recordingStartedAt) {
  const key = step.id.replace(/^segment_/, "");
  const segment = segmentMap.get(key) || { key, title: step.marker || key, scopes: [] };
  return {
    key,
    title: segment.title || step.marker || key,
    filename: segment.filename || "",
    scopes: Array.isArray(segment.scopes) ? segment.scopes : [],
    estimated_duration_sec: segment.estimated_duration_sec || 0,
    started_elapsed_ms: Date.now() - recordingStartedAt,
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

export async function uploadVideoArtifacts(reporter, artifacts, out) {
  if (!reporter?.uploadArtifact) return "";
  if (Array.isArray(artifacts?.video_segments) && artifacts.video_segments.length > 0) {
    let firstFileID = "";
    for (const segment of artifacts.video_segments) {
      if (!segment?.local_path) continue;
      const format = segment.format || videoFormat(segment.local_path);
      const fileID = await reporter.uploadArtifact({
        artifactType: "demo_video",
        segmentKey: segment.segment_key || segment.key || "",
        contentType: videoContentType(format),
        path: segment.local_path,
      });
      out.write("[artifact] uploaded demo video segment: " + fileID + "\n");
      segment.file_id = fileID;
      if (!firstFileID) firstFileID = fileID;
    }
    return firstFileID;
  }
  return uploadVideoArtifact(reporter, artifacts?.video, out);
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
