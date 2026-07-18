import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";

const read = (path) => readFile(new URL(`../../${path}`, import.meta.url), "utf8");

test("AGENTS enforces exclusive branch and worktree ownership", async () => {
  const agents = await read("AGENTS.md");
  assert.match(agents, /one exclusive branch and one exclusive worktree/i);
  assert.match(agents, /must never use another conversation's branch or worktree/i);
  assert.match(agents, /verify the absolute worktree path and current branch/i);
});

test("AGENTS requires preview acceptance before dev", async () => {
  const agents = await read("AGENTS.md");
  assert.match(agents, /Draft pull request.*to `dev`/s);
  assert.match(agents, /Railway PR Environment/);
  assert.match(agents, /Vercel Preview/);
  assert.match(agents, /must not merge.*`dev`.*Preview Acceptance.*success/is);
});

test("AGENTS treats every non-success regression result as a hard stop", async () => {
  const agents = await read("AGENTS.md");
  for (const result of ["fails", "errors", "times out", "is cancelled", "is skipped", "cannot start"]) {
    assert.ok(agents.includes(result), `missing hard-stop result: ${result}`);
  }
  assert.match(agents, /exact failure message and relevant log excerpt/i);
});

test("CI covers every integration branch without production runtime targets", async () => {
  const workflow = await read(".github/workflows/ci.yml");
  for (const branch of ["dev", "staging", "main"]) {
    assert.ok(workflow.includes(`- ${branch}`), `CI does not cover ${branch}`);
  }
  assert.doesNotMatch(workflow, /https:\/\/api\.unipost\.dev/);
  assert.doesNotMatch(workflow, /pk_live_/);
  assert.match(workflow, /NEXT_PUBLIC_API_URL: http:\/\/localhost:8080/);
  assert.match(
    workflow,
    /NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY:.*NEXT_PUBLIC_CLERK_DEVELOPMENT_PUBLISHABLE_KEY/,
  );
});

test("Preview Acceptance is fail-closed and tied to the exact PR head", async () => {
  const workflow = await read(".github/workflows/preview-acceptance.yml");
  for (const event of [
    "opened",
    "synchronize",
    "reopened",
    "ready_for_review",
    "labeled",
    "unlabeled",
    "closed",
  ]) {
    assert.ok(workflow.includes(event), `Preview workflow omits ${event}`);
  }
  assert.match(workflow, /github\.event\.pull_request\.head\.sha/);
  assert.match(workflow, /github\.event\.pull_request\.head\.repo\.full_name == github\.repository/);
  assert.match(workflow, /startsWith\(github\.event\.pull_request\.head\.ref, 'dev-'\)/);
  assert.match(workflow, /vercel@50\.26\.1/);
  assert.match(workflow, /RAILWAY_API_TOKEN:.*secrets\.RAILWAY_API_TOKEN/);
  assert.match(workflow, /railway-deployments\.mjs/);
  assert.doesNotMatch(
    workflow,
    /--cwd dashboard/,
    "the Vercel project already has dashboard as its Root Directory",
  );
  assert.match(workflow, /test:regression:preview/);
  assert.match(workflow, /preview-failure-drill/);
  assert.match(workflow, /if: always\(\)/);
  assert.match(workflow, /if: failure\(\)/);
  assert.match(workflow, /alias rm/);
  assert.doesNotMatch(workflow, /https:\/\/api\.unipost\.dev/);
  assert.doesNotMatch(workflow, /pk_live_/);
});

test("ordinary dashboard regression excludes deployed preview-only acceptance", async () => {
  const config = await read("dashboard/playwright.regression.config.ts");
  assert.match(
    config,
    /testIgnore:\s*["']preview-environment\.spec\.ts["']/,
    "dashboard regression would collect the preview-only spec without its required deployment identity",
  );
});
