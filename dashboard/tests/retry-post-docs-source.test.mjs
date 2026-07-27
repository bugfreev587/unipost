import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  try {
    return await readFile(join(root, path), "utf8");
  } catch (error) {
    if (error?.code === "ENOENT") return "";
    throw error;
  }
}

function trailingBackslashCount(line = "") {
  return line.length - line.replace(/\\+$/, "").length;
}

test("Retry Post API Reference documents the endpoint contract and appears in Posts navigation", async () => {
  const [page, docsShell] = await Promise.all([
    source("src/app/docs/api/posts/retry/page.tsx"),
    source("src/app/docs/_components/docs-shell.tsx"),
  ]);

  assert.match(page, /SingleEndpointReferencePage/);
  assert.match(page, /title="Retry Post"/);
  assert.match(page, /method="POST"/);
  assert.match(page, /path="\/v1\/posts\/\{id\}\/results\/\{resultID\}\/retry"/);
  assert.match(page, /Authorization/);
  assert.match(page, /Bearer \$UNIPOST_API_KEY/);
  assert.match(page, /no request body/i);
  assert.match(page, /same result record/i);
  assert.doesNotMatch(page, /social_post_result/i);
  assert.match(page, /does not create a new result row/i);
  assert.match(page, /no idempotency key/i);
  assert.match(page, /GET \/v1\/posts\/\{id\}/);
  assert.match(page, /status[^]*queued/i);
  const apiCurlStart = page.split("\n").find((line) => line.includes("code: `curl -X POST"));
  assert.equal(trailingBackslashCount(apiCurlStart), 2, "API source should render one shell continuation");
  assert.match(page, /429/);
  assert.match(page, /RATE_LIMITED/);
  assert.match(page, /Retry-After/);
  assert.match(page, /upgrade_or_wait_then_retry/);
  assert.match(page, /TikTok publishing is temporarily unavailable on the Free plan/);

  for (const code of [
    "PLAN_PLATFORM_PUBLISHING_RESTRICTED",
    "RESULT_NOT_RETRYABLE",
    "QUEUE_JOB_ACTIVE",
    "SOCIAL_ACCOUNT_NOT_AVAILABLE",
    "MEDIA_REUPLOAD_REQUIRED",
    "POLICY_UNAVAILABLE",
  ]) {
    assert.match(page, new RegExp(code));
  }

  assert.match(
    docsShell,
    /label: "Retry Post", href: "\/docs\/api\/posts\/retry", method: "POST"/,
  );
});

test("Retry failed posts guide explains eligibility, TikTok capacity, workflow, and Guides navigation", async () => {
  const [guide, docsShell] = await Promise.all([
    source("src/app/docs/guides/posts/retry-failed-posts/page.tsx"),
    source("src/app/docs/_components/docs-shell.tsx"),
  ]);

  assert.match(guide, /title="Retry failed posts"/);
  assert.match(guide, /automatic retr/i);
  assert.match(guide, /manual retr/i);
  assert.match(guide, /manual_retry_allowed/);
  assert.match(guide, /pending[^]*running[^]*retrying/i);
  assert.match(guide, /disconnected[^]*reconnect_required/i);
  assert.match(guide, /uploaded[^]*retained/i);
  assert.match(guide, /reached_active_user_cap/);
  assert.match(guide, /is_retriable[^]*false/i);
  assert.match(guide, /Paid Plan[^]*capacity window/i);
  assert.match(guide, /Free Plan[^]*PLAN_PLATFORM_PUBLISHING_RESTRICTED/i);
  assert.match(guide, /does not request TikTok/i);
  assert.match(guide, /POST[^]*retry[^]*once/i);
  assert.match(guide, /poll[^]*GET \/v1\/posts\/\{id\}/i);
  const guideCurlStart = guide.split("\n").find((line) => line.includes("curl -fSs -X POST"));
  assert.equal(trailingBackslashCount(guideCurlStart), 2, "guide source should render one shell continuation");
  assert.match(guide, /media[^]*new publish request/i);
  assert.match(guide, /429/);
  assert.match(guide, /RATE_LIMITED/);
  assert.match(guide, /Retry-After/);

  for (const response of [
    "200",
    "402",
    "409",
    "429",
    "503",
    "RATE_LIMITED",
    "PLAN_PLATFORM_PUBLISHING_RESTRICTED",
    "RESULT_NOT_RETRYABLE",
    "QUEUE_JOB_ACTIVE",
    "SOCIAL_ACCOUNT_NOT_AVAILABLE",
    "MEDIA_REUPLOAD_REQUIRED",
    "POLICY_UNAVAILABLE",
  ]) {
    assert.match(guide, new RegExp(response));
  }

  assert.match(
    docsShell,
    /label: "Retry failed posts", href: "\/docs\/guides\/posts\/retry-failed-posts"/,
  );
});
