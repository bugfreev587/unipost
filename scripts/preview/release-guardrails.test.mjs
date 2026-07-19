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

test("CI makes the dashboard SEO regression blocking", async () => {
  const workflow = await read(".github/workflows/ci.yml");
  assert.match(
    workflow,
    /Run dashboard SEO source regression[\s\S]*npm run test:seo/,
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
  assert.match(workflow, /startsWith\(github\.event\.pull_request\.head\.ref, 'hotfix-'\)/);
  assert.match(workflow, /vercel@50\.26\.1/);
  assert.match(workflow, /--prebuilt[\s\S]*--archive=tgz/);
  assert.match(workflow, /github\.run_id/);
  assert.match(workflow, /github\.run_attempt/);
  assert.match(workflow, /RAILWAY_API_TOKEN:.*secrets\.RAILWAY_API_TOKEN/);
  assert.match(workflow, /RAILWAY_PROJECT_ID:.*vars\.RAILWAY_PROJECT_ID/);
  assert.match(workflow, /railway-deployments\.mjs/);
  assert.doesNotMatch(
    workflow,
    /--cwd dashboard/,
    "the Vercel project already has dashboard as its Root Directory",
  );
  assert.match(workflow, /test:regression:preview/);
  assert.match(
    workflow,
    /Run deployed preview regression[\s\S]*VERCEL_AUTOMATION_BYPASS_SECRET:.*secrets\.VERCEL_AUTOMATION_BYPASS_SECRET/,
  );
  assert.doesNotMatch(workflow, /vercel-share-link\.mjs/);
  assert.doesNotMatch(workflow, /VERCEL_SHAREABLE_URL/);
  assert.match(
    workflow,
    /DASHBOARD_BASE_URL: \$\{\{ steps\.vercel\.outputs\.deployment_url \}\}/,
  );
  assert.match(workflow, /vercel-alias-cleanup\.mjs/);
  assert.match(
    workflow,
    /cleanup-preview-alias:[\s\S]*ref: \$\{\{ github\.event\.pull_request\.head\.sha \}\}/,
  );
  assert.match(workflow, /preview-failure-drill/);
  assert.match(workflow, /if: always\(\)/);
  assert.match(workflow, /if: failure\(\)/);
  assert.doesNotMatch(workflow, /https:\/\/api\.unipost\.dev/);
  assert.doesNotMatch(workflow, /pk_live_/);

  const previewConfig = await read("dashboard/playwright.preview.config.ts");
  assert.doesNotMatch(previewConfig, /VERCEL_SHAREABLE_URL/);
  assert.doesNotMatch(previewConfig, /extraHTTPHeaders/);
  assert.match(previewConfig, /seo-preview\.spec\.ts/);

  const previewTest = await read("dashboard/tests/regression/preview-environment.spec.ts");
  assert.doesNotMatch(previewTest, /shareableURL/);
  assert.match(previewTest, /x-vercel-protection-bypass/);
  assert.match(previewTest, /x-vercel-set-bypass-cookie/);

  const seoPreviewTest = await read(
    "dashboard/tests/regression/seo-preview.spec.ts",
  );
  assert.match(seoPreviewTest, /\/sitemap\.xml/);
  assert.match(seoPreviewTest, /maxRedirects:\s*0/);
  assert.match(seoPreviewTest, /noindex/i);
  assert.match(seoPreviewTest, /UniPost \| Social Media Posting API for Developers/);

  const proxy = await read("dashboard/src/proxy.ts");
  assert.match(proxy, /pathname === "\/__unipost-preview\.json"/);
});

test("ordinary dashboard regression excludes deployed preview-only acceptance", async () => {
  const config = await read("dashboard/playwright.regression.config.ts");
  assert.match(config, /testIgnore:\s*\[[\s\S]*preview-environment\.spec\.ts/);
  assert.match(config, /testIgnore:\s*\[[\s\S]*seo-preview\.spec\.ts/);
});
