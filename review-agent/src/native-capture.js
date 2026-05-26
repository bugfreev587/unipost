import { spawn } from "node:child_process";
import { mkdir } from "node:fs/promises";
import path from "node:path";

const DEFAULT_CAPTURE_MODE = "native-browser-window";

export async function startNativeBrowserCapture({
  script,
  browser,
  page,
  outputDir = defaultNativeVideoDir(),
  platform = process.platform,
  spawnImpl = spawn,
  out = process.stdout,
  startDelayMs = 900,
} = {}) {
  const recording = script?.recording || {};
  const requestedMode = recording.capture_mode || (recording.show_address_bar ? DEFAULT_CAPTURE_MODE : "playwright-page-video");
  if (requestedMode === "playwright-page-video" || !recording.show_address_bar) {
    return null;
  }
  if (platform !== "darwin") {
    throw new Error("native browser-window capture currently requires macOS; use playwright-page-video on this platform");
  }

  await mkdir(outputDir, { recursive: true });
  const bounds = await resolveChromiumWindowBounds({ browser, page, recording });
  const filePath = path.join(outputDir, `${safeFilePart(script.job_id || "review")}-browser-window.mov`);
  const proc = spawnImpl("screencapture", [
    "-v",
    "-x",
    "-R",
    `${bounds.left},${bounds.top},${bounds.width},${bounds.height}`,
    filePath,
  ], { stdio: ["ignore", "ignore", "pipe"] });

  let stderr = "";
  proc.stderr?.on?.("data", (chunk) => {
    stderr += chunk.toString();
  });
  await waitForNativeCaptureStart(proc, () => stderr, startDelayMs);
  out.write(`[recording] native browser-window capture started: ${bounds.left},${bounds.top},${bounds.width},${bounds.height}\n`);

  return {
    mode: "macos-screencapture-region",
    localPath: filePath,
    includesAddressBar: true,
    bounds,
    async stop() {
      return stopCaptureProcess(proc, () => stderr);
    },
  };
}

export async function resolveChromiumWindowBounds({ browser, page, recording = {} } = {}) {
  if (!page?.context) {
    throw new Error("browser page is required to resolve native capture bounds");
  }
  const session = await page.context().newCDPSession(page);
  const result = await session.send("Browser.getWindowForTarget");
  const windowID = result.windowId;
  const targetBounds = {
    left: recording.window_left ?? 80,
    top: recording.window_top ?? 80,
    width: recording.window_width || result.bounds?.width || 1440,
    height: recording.window_height || result.bounds?.height || 1000,
    windowState: "normal",
  };
  if (windowID !== undefined) {
    await session.send("Browser.setWindowBounds", {
      windowId: windowID,
      bounds: targetBounds,
    });
  }
  return {
    left: Math.max(0, Number(targetBounds.left) || 0),
    top: Math.max(0, Number(targetBounds.top) || 0),
    width: Math.max(320, Number(targetBounds.width) || 1440),
    height: Math.max(240, Number(targetBounds.height) || 1000),
  };
}

function waitForNativeCaptureStart(proc, stderrText, delayMs = 900) {
  return new Promise((resolve, reject) => {
    let settled = false;
    const timer = setTimeout(() => {
      if (settled) return;
      settled = true;
      cleanup();
      resolve();
    }, delayMs);
    const onError = (err) => {
      if (settled) return;
      settled = true;
      cleanup();
      reject(err);
    };
    const onExit = (code, signal) => {
      if (settled) return;
      settled = true;
      cleanup();
      reject(new Error(`native capture exited before recording started: ${code ?? signal ?? "unknown"} ${stderrText()}`.trim()));
    };
    const cleanup = () => {
      clearTimeout(timer);
      proc.off?.("error", onError);
      proc.off?.("exit", onExit);
    };
    proc.once?.("error", onError);
    proc.once?.("exit", onExit);
  });
}

function stopCaptureProcess(proc, stderrText) {
  return new Promise((resolve, reject) => {
    let settled = false;
    const finish = (code, signal) => {
      if (settled) return;
      settled = true;
      cleanup();
      if (code === 0 || signal === "SIGINT" || signal === "SIGTERM" || code === null) {
        resolve();
        return;
      }
      reject(new Error(`native capture failed: ${code} ${stderrText()}`.trim()));
    };
    const onError = (err) => {
      if (settled) return;
      settled = true;
      cleanup();
      reject(err);
    };
    const cleanup = () => {
      proc.off?.("exit", finish);
      proc.off?.("error", onError);
    };
    proc.once?.("exit", finish);
    proc.once?.("error", onError);
    proc.kill?.("SIGINT");
    setTimeout(() => {
      if (!settled) proc.kill?.("SIGTERM");
    }, 3000).unref?.();
  });
}

function defaultNativeVideoDir() {
  return process.env.UNIPOST_REVIEW_VIDEO_DIR || path.resolve(process.cwd(), ".unipost-review-videos");
}

function safeFilePart(value) {
  return String(value).replace(/[^a-zA-Z0-9_-]+/g, "-").replace(/^-+|-+$/g, "") || "review";
}
