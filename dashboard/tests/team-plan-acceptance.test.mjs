import assert from "node:assert/strict";
import { test } from "node:test";

import {
  CleanupLedger,
  clerkRequest,
  loadAcceptanceConfig,
  runWithCleanup,
} from "../scripts/team-plan-acceptance.mjs";

const identities = {
  TEAM_ACCEPTANCE_OWNER_EMAIL: "owner@example.com",
  TEAM_ACCEPTANCE_ADMIN_EMAIL: "admin@example.com",
  TEAM_ACCEPTANCE_EDITOR_EMAIL: "editor@example.com",
};

test("production rejects staging domains", () => {
  assert.throws(
    () => loadAcceptanceConfig({
      TEAM_ACCEPTANCE_ENV: "production",
      TEAM_ACCEPTANCE_API_URL: "https://staging-api.unipost.dev",
      TEAM_ACCEPTANCE_APP_URL: "https://staging-app.unipost.dev",
      TEAM_ACCEPTANCE_DATABASE_URL: "postgresql://example.invalid/db",
      TEAM_ACCEPTANCE_CLERK_SECRET_KEY: "sk_live_test",
      ...identities,
    }),
    /production.*domain/i,
  );
});

test("staging rejects production domains", () => {
  assert.throws(
    () => loadAcceptanceConfig({
      TEAM_ACCEPTANCE_ENV: "staging",
      TEAM_ACCEPTANCE_API_URL: "https://api.unipost.dev",
      TEAM_ACCEPTANCE_APP_URL: "https://app.unipost.dev",
      TEAM_ACCEPTANCE_DATABASE_URL: "postgresql://example.invalid/db",
      TEAM_ACCEPTANCE_CLERK_SECRET_KEY: "sk_test_test",
      ...identities,
    }),
    /staging.*domain/i,
  );
});

test("release mode requires owner, admin, and editor identities", () => {
  const base = {
    TEAM_ACCEPTANCE_ENV: "staging",
    TEAM_ACCEPTANCE_API_URL: "https://staging-api.unipost.dev",
    TEAM_ACCEPTANCE_APP_URL: "https://staging-app.unipost.dev",
    TEAM_ACCEPTANCE_DATABASE_URL: "postgresql://example.invalid/db",
    TEAM_ACCEPTANCE_CLERK_SECRET_KEY: "sk_test_test",
  };

  for (const missing of Object.keys(identities)) {
    const env = { ...base, ...identities };
    delete env[missing];
    assert.throws(() => loadAcceptanceConfig(env), new RegExp(missing));
  }
});

test("release mode rejects a Railway-private database host", () => {
  assert.throws(
    () => loadAcceptanceConfig({
      TEAM_ACCEPTANCE_ENV: "staging",
      TEAM_ACCEPTANCE_API_URL: "https://staging-api.unipost.dev",
      TEAM_ACCEPTANCE_APP_URL: "https://staging-app.unipost.dev",
      TEAM_ACCEPTANCE_DATABASE_URL: "postgresql://postgres:secret@postgres.railway.internal:5432/railway",
      TEAM_ACCEPTANCE_CLERK_SECRET_KEY: "sk_test_test",
      TEAM_ACCEPTANCE_OWNER_EMAIL: "codex-team-acceptance-owner@example.com",
      TEAM_ACCEPTANCE_ADMIN_EMAIL: "codex-team-acceptance-admin@example.com",
      TEAM_ACCEPTANCE_EDITOR_EMAIL: "codex-team-acceptance-editor@example.com",
    }),
    /public.*database/i,
  );
});

test("Clerk POST without a payload still sends JSON content type", async () => {
  let requestInit;
  await clerkRequest(
    { clerkSecretKey: "sk_test_test" },
    "/sessions/sess_test/tokens",
    { method: "POST" },
    async (_url, init) => {
      requestInit = init;
      return new Response('{"jwt":"token"}', {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    },
  );

  assert.equal(requestInit.headers["Content-Type"], "application/json");
});

test("cleanup runs after a failed acceptance assertion", async () => {
  const ledger = new CleanupLedger();
  const calls = [];
  ledger.track("profile", "profile_1", async () => calls.push("profile_1"));

  await assert.rejects(
    runWithCleanup(ledger, async () => {
      throw new Error("acceptance failed");
    }),
    /acceptance failed/,
  );
  assert.deepEqual(calls, ["profile_1"]);
  assert.deepEqual(ledger.remaining(), []);
});

test("a non-empty cleanup ledger fails the run", async () => {
  const ledger = new CleanupLedger();
  ledger.track("api_key", "key_1", async () => {
    throw new Error("revoke failed");
  });

  await assert.rejects(
    runWithCleanup(ledger, async () => {}),
    /cleanup.*api_key:key_1.*revoke failed/i,
  );
  assert.deepEqual(ledger.remaining(), ["api_key:key_1"]);
});
