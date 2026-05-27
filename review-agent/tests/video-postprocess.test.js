import test from "node:test";
import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { buildSegmentClipSpecs, processReviewVideoSegments } from "../src/video-postprocess.js";

test("buildSegmentClipSpecs converts segment lifecycle events into clip windows", () => {
  const specs = buildSegmentClipSpecs({
    outputDir: "/tmp/unipost-review-videos",
    segments: [
      { key: "posting_part_1", title: "Posting Part 1", filename: "posting-part-1.mp4", scopes: ["video.upload"] },
      { key: "posting_part_2", title: "Posting Part 2", filename: "posting-part-2.mp4", scopes: ["video.publish"] },
    ],
    segmentEvents: [
      { key: "posting_part_1", started_elapsed_ms: 250 },
      { key: "posting_part_1", started_elapsed_ms: 250, completed_elapsed_ms: 3250 },
      { key: "posting_part_2", started_elapsed_ms: 3250 },
      { key: "posting_part_2", started_elapsed_ms: 3250, completed_elapsed_ms: 7800 },
    ],
  });

  assert.equal(specs.length, 2);
  assert.deepEqual(specs[0], {
    segment_key: "posting_part_1",
    title: "Posting Part 1",
    scopes: ["video.upload"],
    start_sec: 0.25,
    duration_sec: 3,
    local_path: "/tmp/unipost-review-videos/posting-part-1.mp4",
  });
  assert.equal(specs[1].duration_sec, 4.55);
});

test("processReviewVideoSegments invokes ffmpeg with 1080p mp4 output capped under TikTok's 50MB limit", async () => {
  const calls = [];
  const spawnImpl = (command, args) => {
    calls.push({ command, args });
    const proc = new EventEmitter();
    proc.stderr = new EventEmitter();
    proc.stderr.setEncoding = () => {};
    queueMicrotask(() => proc.emit("exit", 0));
    return proc;
  };

  const results = await processReviewVideoSegments({
    sourceVideo: { local_path: "/tmp/source.mov" },
    outputDir: "/tmp/unipost-review-videos",
    maxBytes: 50_000_000,
    segments: [{ key: "analytics_part_1", title: "Analytics Part 1", filename: "analytics-part-1.mp4", scopes: ["user.info.stats"] }],
    segmentEvents: [
      { key: "analytics_part_1", started_elapsed_ms: 0 },
      { key: "analytics_part_1", started_elapsed_ms: 0, completed_elapsed_ms: 42_000 },
    ],
    spawnImpl,
    statImpl: async () => ({ size: 42_000_000 }),
    mkdirImpl: async () => {},
  });

  assert.equal(results[0].segment_key, "analytics_part_1");
  assert.equal(results[0].format, "mp4");
  assert.equal(results[0].size_bytes, 42_000_000);
  assert.equal(calls[0].command, "ffmpeg");
  assert.deepEqual(calls[0].args.slice(0, 8), ["-y", "-ss", "0.000", "-i", "/tmp/source.mov", "-t", "42.000", "-vf"]);
  assert.equal(calls[0].args.includes("scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2"), true);
  assert.equal(calls[0].args.includes("-fs"), true);
  assert.equal(calls[0].args.includes("50000000"), true);
});
