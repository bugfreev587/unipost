import assert from "node:assert/strict";
import test from "node:test";

import { describePostResultFailure } from "../src/lib/post-result-errors.ts";

test("describePostResultFailure prefers structured publish failure fields", () => {
  const detail = describePostResultFailure({
    status: "failed",
    platform: "tiktok",
    error_message: "invalid_params",
    error_code: "platform_request_invalid",
    failure_stage: "platform_publish_init",
    platform_error_code: "invalid_params",
    is_retriable: false,
    next_action: "review_platform_options",
  });

  assert.equal(detail.title, "TikTok rejected the publish request");
  assert.match(detail.message, /invalid_params/i);
  assert.equal(detail.nextActionLabel, "Review platform settings");
  assert.equal(detail.canRetry, false);
});

test("describePostResultFailure keeps legacy string fallback", () => {
  const detail = describePostResultFailure({
    status: "failed",
    platform: "threads",
    error_message: "token expired while publishing",
  });

  assert.equal(detail.title, "Reconnect account");
  assert.match(detail.message, /token expired/);
  assert.equal(detail.nextActionLabel, "Reconnect account");
  assert.equal(detail.canRetry, false);
});
