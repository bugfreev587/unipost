import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import test from "node:test";

const root = join(process.cwd(), "..");

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("all source validators exercise the additive X read and Credits SDK APIs", async () => {
  const validators = await Promise.all([
    source("scripts/sdk-validation/js/unipost-sdk-test.mjs"),
    source("scripts/sdk-validation/python/unipost_sdk_test.py"),
    source("scripts/sdk-validation/go/main.go"),
    source("scripts/sdk-validation/java/src/main/java/dev/unipost/validation/UnipostSdkTest.java"),
  ]);

  const languageContracts = [
    [
      /accounts\.getProfile\(/,
      /accounts\.listPosts\(/,
      /billing\.getXCredits\(/,
      /billing\.listXCreditEvents\(/,
    ],
    [
      /accounts\.get_profile\(/,
      /accounts\.list_posts\(/,
      /billing\.get_x_credits\(/,
      /billing\.list_x_credit_events\(/,
    ],
    [
      /Accounts\.Profile\(/,
      /Accounts\.ListPosts\(/,
      /Billing\.GetXCredits\(/,
      /Billing\.ListXCreditEvents\(/,
    ],
    [
      /accounts\(\)\.profile\(/,
      /accounts\(\)\.listPosts\(/,
      /billing\(\)\.getXCredits\(/,
      /billing\(\)\.listXCreditEvents\(/,
    ],
  ];

  validators.forEach((validator, index) => {
    for (const expected of languageContracts[index]) {
      assert.match(validator, expected);
    }
    assert.match(validator, /TEST_X_ACCOUNT_ID/);
    assert.match(validator, /TEST_EXTERNAL_USER_ID/);
    assert.match(validator, /REQUIRE_X_ACCOUNT_READ_ACCEPTANCE/);
    assert.match(validator, /request_id|RequestID|getRequestId/);
    assert.match(validator, /credits|Credits/);
    assert.match(validator, /replay|Replay/);
    assert.match(validator, /next_cursor|NextCursor/);
  });
});

test("source validation can pin every SDK PR head and fail closed on missing X fixtures", async () => {
  const [workflow, runner] = await Promise.all([
    source(".github/workflows/sdk-source-validation.yml"),
    source("scripts/sdk-source-validation/run-suite.sh"),
  ]);

  for (const name of ["sdk_js_ref", "sdk_python_ref", "sdk_go_ref", "sdk_java_ref"]) {
    assert.match(workflow, new RegExp(`${name}:`));
    assert.match(workflow, new RegExp(`inputs\\.${name}`));
  }
  assert.match(workflow, /default:\s*main/);
  assert.match(workflow, /TEST_X_ACCOUNT_ID/);
  assert.match(workflow, /TEST_EXTERNAL_USER_ID/);
  assert.match(workflow, /REQUIRE_X_ACCOUNT_READ_ACCEPTANCE/);
  assert.match(workflow, /UNIPOST_DEV_REGRESSION_API_KEY/);
  assert.match(workflow, /https:\/\/dev-api\.unipost\.dev/);
  assert.match(workflow, /TEST_ACCOUNT_ID:\s*\$\{\{\s*secrets\.REGRESSION_X_ACCOUNT_ID\s*\}\}/);
  assert.match(runner, /TEST_X_ACCOUNT_ID/);
  assert.match(runner, /TEST_EXTERNAL_USER_ID/);
  assert.match(runner, /REQUIRE_X_ACCOUNT_READ_ACCEPTANCE/);
  assert.match(runner, /required X account-read acceptance fixture/i);
  assert.doesNotMatch(runner, /resolve-x-fixture\.py/);
  assert.doesNotMatch(runner, /\/v1\/accounts/);
});

test("release safety requires the new SDK symbols and documents the 0.7.0 gates", async () => {
  const [release, bump, coverage, guide] = await Promise.all([
    source("scripts/release/create-sdk-release.sh"),
    source("scripts/release/bump-sdk-version.sh"),
    source("docs/sdk-api-coverage-matrix.md"),
    source("docs/sdk-release.md"),
  ]);

  for (const symbol of [
    "getProfile",
    "get_profile",
    "Accounts.Profile",
    "accounts().profile",
    "getXCredits",
    "get_x_credits",
    "Billing.GetXCredits",
    "billing().getXCredits",
  ]) {
    assert.match(release, new RegExp(symbol.replace(/[().]/g, "\\$&")));
  }

  for (const route of [
    "GET /v1/accounts/{id}/profile",
    "GET /v1/accounts/{id}/posts",
    "GET /v1/billing/x-credits",
    "GET /v1/billing/x-credits/events",
  ]) {
    assert.match(coverage, new RegExp(route.replace(/[/{}/-]/g, "\\$&")));
  }
  assert.match(coverage, /0\.7\.0/);
  assert.match(guide, /0\.7\.0/);
  assert.match(guide, /UNIPOST_DEV_ROOT/);
  assert.match(guide, /REQUIRE_X_ACCOUNT_READ_ACCEPTANCE/);
  assert.match(guide, /TEST_X_ACCOUNT_ID/);
  assert.match(guide, /TEST_EXTERNAL_USER_ID/);
  assert.match(guide, /github\.com\/unipost-dev\/sdk-go/);

  for (const versionContractPath of [
    "tests/release-workflows.test.ts",
    "tests/users.test.ts",
    "tests/test_release.py",
    "unipost/release_test.go",
    "src/test/java/dev/unipost/InboxTest.java",
  ]) {
    assert.match(
      bump,
      new RegExp(versionContractPath.replace(/[./]/g, "\\$&")),
      `version bump must update ${versionContractPath}`,
    );
    assert.match(
      release,
      new RegExp(versionContractPath.replace(/[./]/g, "\\$&")),
      `release commit must include ${versionContractPath}`,
    );
  }
});

test("X Credits reference shows complete 0.7.0 SDK envelopes for allowance and events", async () => {
  const reference = await source("dashboard/src/app/docs/api/x-credits/page.tsx");
  const allowanceStart = reference.indexOf("const ALLOWANCE_SNIPPETS");
  const eventsStart = reference.indexOf("const EVENT_SNIPPETS");
  const componentStart = reference.indexOf("export default async function XCreditsReferencePage");
  assert.notEqual(allowanceStart, -1);
  assert.notEqual(eventsStart, -1);
  assert.notEqual(componentStart, -1);

  const allowance = reference.slice(allowanceStart, eventsStart);
  const events = reference.slice(eventsStart, componentStart);

  function languageSnippet(block, lang) {
    const start = block.indexOf(`lang: "${lang}"`);
    const end = block.indexOf("\n  },", start);
    assert.notEqual(start, -1, `missing ${lang} snippet`);
    assert.notEqual(end, -1, `unterminated ${lang} snippet`);
    return block.slice(start, end);
  }

  assert.match(allowance, /lang: "curl"[\s\S]*\/v1\/billing\/x-credits/);
  assert.match(events, /lang: "curl"[\s\S]*\/v1\/billing\/x-credits\/events/);

  const languageContracts = [
    {
      lang: "javascript",
      allowance: [/billing\.getXCredits\(/, /allowance\.request_id/, /allowance\.data\.monthly_remaining/],
      events: [/billing\.listXCreditEvents\(/, /events\.request_id/, /events\.data/, /events\.meta\.next_cursor/],
    },
    {
      lang: "python",
      allowance: [/billing\.get_x_credits\(/, /allowance\.request_id/, /allowance\.data\.monthly_remaining/],
      events: [/billing\.list_x_credit_events\(/, /events\.request_id/, /events\.data/, /events\.meta\.next_cursor/],
    },
    {
      lang: "go",
      allowance: [/Billing\.GetXCredits\(/, /allowance\.RequestID/, /allowance\.Data\.MonthlyRemaining/],
      events: [/Billing\.ListXCreditEvents\(/, /events\.RequestID/, /events\.Data/, /events\.Meta\.NextCursor/],
    },
    {
      lang: "java",
      allowance: [
        /billing\(\)\.getXCredits\(/,
        /allowance\.path\("request_id"\)/,
        /allowance\.path\("data"\)\.path\("monthly_remaining"\)/,
      ],
      events: [
        /billing\(\)\.listXCreditEvents\(/,
        /events\.path\("request_id"\)/,
        /events\.path\("data"\)/,
        /events\.path\("meta"\)\.path\("next_cursor"\)/,
      ],
    },
  ];

  for (const contract of languageContracts) {
    const allowanceSnippet = languageSnippet(allowance, contract.lang);
    const eventsSnippet = languageSnippet(events, contract.lang);
    assert.match(allowanceSnippet, new RegExp(`lang: "${contract.lang}"`));
    assert.match(eventsSnippet, new RegExp(`lang: "${contract.lang}"`));
    for (const expected of contract.allowance) assert.match(allowanceSnippet, expected);
    for (const expected of contract.events) assert.match(eventsSnippet, expected);
  }

  for (const componentUse of [
    /<CodeTabs snippets=\{ALLOWANCE_SNIPPETS\} \/>/,
    /<CodeTabs snippets=\{EVENT_SNIPPETS\} \/>/,
  ]) {
    assert.match(reference, componentUse);
  }

  assert.match(reference, /requirePublicDocsFeature\("x_credits_billing_v1"\)/);
});
