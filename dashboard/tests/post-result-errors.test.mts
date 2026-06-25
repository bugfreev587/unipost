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
    error_source: "platform",
    error_temporality: "permanent",
    provider_error: {
      provider: "tiktok",
      http_status: 400,
      code: "invalid_params",
    },
    retry_policy: {
      is_retriable: false,
      will_retry: false,
      retry_state: "not_retriable",
      manual_retry_allowed: false,
      reason: "classification_not_retriable",
    },
  });

  assert.equal(detail.title, "TikTok rejected the publish request");
  assert.match(detail.message, /TikTok rejected the request/);
  assert.match(detail.message, /Provider error: invalid_params, HTTP 400/);
  assert.equal(detail.nextActionLabel, "Review platform settings");
  assert.equal(detail.retryStatusLabel, "Automatic retry is not scheduled.");
  assert.equal(detail.canRetry, false);
});

test("describePostResultFailure separates automatic retry from manual retry", () => {
  const detail = describePostResultFailure({
    status: "failed",
    platform: "instagram",
    error_message: `publish failed (500): {"error":{"message":"An unexpected error has occurred. Please retry your request later.","type":"OAuthException","is_transient":true,"code":2}}`,
    error_code: "temporary_platform_error",
    is_retriable: true,
    next_action: "retry_later",
    error_source: "platform",
    error_temporality: "temporary",
    provider_error: {
      provider: "meta",
      http_status: 500,
      code: "2",
      type: "OAuthException",
      is_transient: true,
    },
    retry_policy: {
      is_retriable: true,
      will_retry: true,
      retry_state: "scheduled",
      next_run_at: "2026-06-23T22:00:30Z",
      attempts_made: 1,
      max_attempts: 5,
      attempts_remaining: 4,
      manual_retry_allowed: false,
    },
  });

  assert.match(detail.message, /Instagram had a temporary official-platform error/);
  assert.match(detail.message, /Provider error: 2, HTTP 500/);
  assert.match(detail.message, /structured error payload/);
  assert.match(detail.retryStatusLabel || "", /UniPost will retry automatically/);
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
