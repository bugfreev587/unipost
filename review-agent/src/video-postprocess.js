import { spawn } from "node:child_process";
import { mkdir, stat } from "node:fs/promises";
import path from "node:path";

const DEFAULT_MAX_BYTES = 50_000_000;
const OUTPUT_FILTER_1080P = "scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2";

export async function processReviewVideoSegments({
  sourceVideo,
  outputDir = defaultVideoDir(),
  maxBytes = DEFAULT_MAX_BYTES,
  segments = [],
  segmentEvents = [],
  ffmpegPath = defaultFFmpegPath(),
  spawnImpl = spawn,
  statImpl = stat,
  mkdirImpl = mkdir,
} = {}) {
  if (!sourceVideo?.local_path) return [];
  await mkdirImpl(outputDir, { recursive: true });
  const specs = buildSegmentClipSpecs({ outputDir, segments, segmentEvents });
  const results = [];
  for (const spec of specs) {
    await runFFmpeg(spawnImpl, ffmpegPath, buildFFmpegSegmentArgs({
      sourcePath: sourceVideo.local_path,
      spec,
      maxBytes,
    }));
    const info = await statImpl(spec.local_path);
    results.push({
      ...spec,
      format: "mp4",
      size_bytes: info.size,
    });
  }
  return results;
}

export function buildSegmentClipSpecs({ outputDir = defaultVideoDir(), segments = [], segmentEvents = [] } = {}) {
  const windows = new Map();
  for (const event of segmentEvents || []) {
    const key = event?.key;
    if (!key) continue;
    const current = windows.get(key) || {};
    if (Number.isFinite(event.started_elapsed_ms)) current.started_elapsed_ms = event.started_elapsed_ms;
    if (Number.isFinite(event.completed_elapsed_ms)) current.completed_elapsed_ms = event.completed_elapsed_ms;
    windows.set(key, current);
  }

  const specs = [];
  for (const segment of segments || []) {
    const key = segment?.key || "";
    const window = windows.get(key);
    if (!key || !window || !Number.isFinite(window.started_elapsed_ms) || !Number.isFinite(window.completed_elapsed_ms)) continue;
    if (window.completed_elapsed_ms <= window.started_elapsed_ms) continue;
    specs.push({
      segment_key: key,
      title: segment.title || key,
      scopes: Array.isArray(segment.scopes) ? segment.scopes : [],
      start_sec: roundSeconds(window.started_elapsed_ms / 1000),
      duration_sec: roundSeconds((window.completed_elapsed_ms - window.started_elapsed_ms) / 1000),
      local_path: path.join(outputDir, sanitizeSegmentFilename(segment.filename || `${key}.mp4`)),
    });
  }
  return specs;
}

export function buildFFmpegSegmentArgs({ sourcePath, spec, maxBytes = DEFAULT_MAX_BYTES } = {}) {
  const bitrateKbps = targetBitrateKbps(spec.duration_sec, maxBytes);
  return [
    "-y",
    "-ss", formatSeconds(spec.start_sec),
    "-i", sourcePath,
    "-t", formatSeconds(spec.duration_sec),
    "-vf", OUTPUT_FILTER_1080P,
    "-r", "30",
    "-c:v", "libx264",
    "-preset", "veryfast",
    "-b:v", `${bitrateKbps}k`,
    "-maxrate", `${bitrateKbps}k`,
    "-bufsize", `${bitrateKbps * 2}k`,
    "-movflags", "+faststart",
    "-an",
    "-fs", String(maxBytes),
    spec.local_path,
  ];
}

function targetBitrateKbps(durationSec, maxBytes) {
  const duration = Math.max(1, Number(durationSec) || 1);
  const budgetBits = Math.max(1_000_000, Number(maxBytes) || DEFAULT_MAX_BYTES) * 8 * 0.82;
  return Math.max(800, Math.min(4500, Math.floor(budgetBits / duration / 1000)));
}

function runFFmpeg(spawnImpl, command, args) {
  return new Promise((resolve, reject) => {
    const proc = spawnImpl(command, args, { stdio: ["ignore", "ignore", "pipe"] });
    let stderr = "";
    proc.stderr?.setEncoding?.("utf8");
    proc.stderr?.on?.("data", (chunk) => {
      stderr += String(chunk);
    });
    proc.once?.("error", reject);
    proc.once?.("exit", (code, signal) => {
      if (code === 0) {
        resolve();
        return;
      }
      reject(new Error(`ffmpeg segment export failed: ${code ?? signal ?? "unknown"} ${stderr}`.trim()));
    });
  });
}

function defaultFFmpegPath() {
  return process.env.UNIPOST_REVIEW_FFMPEG || "ffmpeg";
}

function defaultVideoDir() {
  return process.env.UNIPOST_REVIEW_VIDEO_DIR || path.resolve(process.cwd(), ".unipost-review-videos");
}

function sanitizeSegmentFilename(value) {
  const file = String(value || "").replace(/[^a-zA-Z0-9_.-]+/g, "-").replace(/^-+|-+$/g, "");
  if (!file) return "review-segment.mp4";
  return file.endsWith(".mp4") ? file : `${file}.mp4`;
}

function roundSeconds(value) {
  return Math.round(value * 1000) / 1000;
}

function formatSeconds(value) {
  return roundSeconds(value).toFixed(3);
}
