import test from "node:test";
import assert from "node:assert/strict";
import { createApiFetchError } from "../src/lib/api.ts";

test("createApiFetchError preserves actionable validation details", () => {
  const err = createApiFetchError(400, {
    error: {
      code: "VALIDATION_ERROR",
      normalized_code: "validation_error",
      message: "request failed pre-publish validation",
      hint: "Fix the listed validation issues and retry.",
      next_action: "fix_request",
      is_retriable: false,
      docs_url: "https://unipost.dev/docs/api/posts/validate",
      issues: [
        {
          platform_post_index: 0,
          account_id: "acc_tiktok",
          platform: "tiktok",
          field: "platform_options.tiktok.title",
          code: "exceeds_max_length",
          message: "TikTok photo title must be 90 characters or fewer.",
          actual: 117,
          limit: 90,
          severity: "error",
        },
      ],
    },
    request_id: "req_tiktok_title",
  });

  assert.equal(err.status, 400);
  assert.equal(err.code, "validation_error");
  assert.equal(err.rawCode, "VALIDATION_ERROR");
  assert.equal(err.hint, "Fix the listed validation issues and retry.");
  assert.equal(err.nextAction, "fix_request");
  assert.equal(err.isRetriable, false);
  assert.equal(err.docsUrl, "https://unipost.dev/docs/api/posts/validate");
  assert.equal(err.requestId, "req_tiktok_title");
  assert.equal(err.issues?.length, 1);
  assert.match(err.message, /request failed pre-publish validation/);
  assert.match(err.message, /platform_options\.tiktok\.title/);
  assert.match(err.message, /90 characters or fewer/);
});

