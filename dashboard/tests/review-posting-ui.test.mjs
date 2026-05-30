import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const clientPath = path.join(root, "src/app/tiktok/posting/tiktok-review-posting-client.tsx");
const appReviewPath = path.join(root, "src/app/(dashboard)/projects/[id]/accounts/app-review/page.tsx");

test("TikTok review posting page keeps video selection manual and removable", async () => {
  const source = await readFile(clientPath, "utf8");

  assert.match(source, /const selectedVideoURL = form\.videoSelected \? videoURL : ""/);
  assert.match(source, /data-review-step="clear-video"/);
  assert.match(source, /No video selected/);
  assert.match(source, /src=\{selectedVideoURL\}/);
  const connectStep = source.indexOf('data-review-step="connect-tiktok"');
  assert.notEqual(connectStep, -1);
  const connectTag = source.slice(source.lastIndexOf("<a", connectStep), source.indexOf(">", connectStep) + 1);
  assert.match(connectTag, /target="_blank"/);
  assert.doesNotMatch(source, /Uploaded to UniPost media storage" : videoHost/);
});

test("App Review Autopilot exposes scope steps and a customer-domain manual launch URL", async () => {
  const source = await readFile(appReviewPath, "utf8");

  assert.match(source, /Manual review workspace/);
  assert.match(source, /buildReviewLaunchURL/);
  assert.match(source, /\/tiktok\/posting\/session/);
  assert.match(source, /\/tiktok\/analytics\/session/);
  assert.match(source, /reviewSessionTokenFromCommand/);
  assert.match(source, /segment\.steps\.map/);
});
