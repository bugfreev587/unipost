import assert from "node:assert/strict";
import { test } from "node:test";

import { createPreviewManifest } from "./write-manifest.mjs";

test("creates a non-secret manifest for one preview SHA", () => {
  assert.deepEqual(createPreviewManifest({
    sha: "a".repeat(40),
    branch: "dev-preview-environment-guardrails",
    apiURL: "https://api-pr-42.up.railway.app",
  }), {
    sha: "a".repeat(40),
    branch: "dev-preview-environment-guardrails",
    apiURL: "https://api-pr-42.up.railway.app",
  });
});

test("creates a non-secret manifest for a hotfix sync branch", () => {
  assert.deepEqual(createPreviewManifest({
    sha: "b".repeat(40),
    branch: "hotfix-public-canonical-20260719",
    apiURL: "https://api-pr-229.up.railway.app",
  }), {
    sha: "b".repeat(40),
    branch: "hotfix-public-canonical-20260719",
    apiURL: "https://api-pr-229.up.railway.app",
  });
});

test("creates a non-secret manifest for an explicitly staging-based Codex branch", () => {
  assert.deepEqual(createPreviewManifest({
    sha: "c".repeat(40),
    branch: "codex/staging-tiktok-free-publishing-restriction",
    apiURL: "https://api-pr-270.up.railway.app",
  }), {
    sha: "c".repeat(40),
    branch: "codex/staging-tiktok-free-publishing-restriction",
    apiURL: "https://api-pr-270.up.railway.app",
  });
});

test("creates a non-secret manifest for the PR 270 hardening branch", () => {
  assert.deepEqual(createPreviewManifest({
    sha: "d".repeat(40),
    branch: "codex/pr270-review-hardening",
    apiURL: "https://api-pr-282.up.railway.app",
  }), {
    sha: "d".repeat(40),
    branch: "codex/pr270-review-hardening",
    apiURL: "https://api-pr-282.up.railway.app",
  });
});

test("rejects persistent API targets", () => {
  for (const apiURL of [
    "https://api.unipost.dev",
    "https://dev-api.unipost.dev",
    "https://staging-api.unipost.dev",
    "https://unipost-dev.up.railway.app",
  ]) {
    assert.throws(
      () => createPreviewManifest({
        sha: "a".repeat(40),
        branch: "dev-preview-environment-guardrails",
        apiURL,
      }),
      /ephemeral Railway/,
    );
  }
});

test("rejects malformed SHAs, branches, and non-HTTPS API URLs", () => {
  assert.throws(
    () => createPreviewManifest({
      sha: "abc",
      branch: "dev-preview-environment-guardrails",
      apiURL: "https://api-pr-42.up.railway.app",
    }),
    /40-character/,
  );
  assert.throws(
    () => createPreviewManifest({
      sha: "a".repeat(40),
      branch: "main",
      apiURL: "https://api-pr-42.up.railway.app",
    }),
    /dev-|hotfix-|codex\/staging-/,
  );
  assert.throws(
    () => createPreviewManifest({
      sha: "a".repeat(40),
      branch: "dev-preview-environment-guardrails",
      apiURL: "http://api-pr-42.up.railway.app",
    }),
    /ephemeral Railway/,
  );
});
