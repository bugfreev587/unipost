import { spawnSync } from "node:child_process";

export function runDoctor({ platform = process.platform, nodeVersion = process.versions.node, ffmpegAvailable = detectFFmpeg() } = {}) {
  const checks = [];
  const major = Number(String(nodeVersion).split(".")[0]);
  checks.push({
    id: "node",
    ok: Number.isFinite(major) && major >= 20,
    message: `Node.js ${nodeVersion}`,
    remediation: "Install Node.js 20 or newer.",
  });
  checks.push({
    id: "playwright",
    ok: true,
    message: "Playwright will be loaded when recording starts.",
    remediation: "If browser launch fails, run: npx playwright install chromium",
  });
  checks.push({
    id: "ffmpeg",
    ok: Boolean(ffmpegAvailable),
    message: ffmpegAvailable ? "ffmpeg is available for MP4 segment export." : "ffmpeg is required to split review recordings into TikTok-ready MP4 files.",
    remediation: "Install ffmpeg or set UNIPOST_REVIEW_FFMPEG to an ffmpeg binary path.",
  });
  if (platform === "darwin") {
    checks.push({
      id: "macos-screen-recording",
      ok: false,
      warning: true,
      message: "macOS requires Screen Recording permission for the terminal app that runs this command.",
      remediation: "Open System Settings -> Privacy & Security -> Screen Recording, enable your terminal, then restart it.",
    });
  }
  return checks;
}

function detectFFmpeg() {
  const binary = process.env.UNIPOST_REVIEW_FFMPEG || "ffmpeg";
  const result = spawnSync(binary, ["-version"], { stdio: "ignore" });
  return !result.error && result.status === 0;
}

export function printDoctor(checks, out = process.stdout) {
  for (const check of checks) {
    const mark = check.ok ? "OK" : check.warning ? "WARN" : "FAIL";
    out.write(`${mark} ${check.id}: ${check.message}\n`);
    if (!check.ok && check.remediation) {
      out.write(`  ${check.remediation}\n`);
    }
  }
}
