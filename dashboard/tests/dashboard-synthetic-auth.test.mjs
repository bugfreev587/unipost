import assert from "node:assert/strict";
import { test } from "node:test";

import {
  bootstrapSyntheticUser,
  cleanupSyntheticUser,
  createSyntheticClerkUser,
  deleteSyntheticClerkUser,
  loadClerkPublishableKey,
  loadSyntheticAuthConfig,
  runWithCleanup,
  signInSyntheticUser,
} from "./regression/support/synthetic-auth.mjs";

test("production requires a live Clerk secret", () => {
  assert.throws(
    () => loadSyntheticAuthConfig({
      DASHBOARD_BASE_URL: "https://app.unipost.dev",
      DASHBOARD_TEST_CLERK_SECRET_KEY: "sk_test_wrong",
    }),
    /production.*sk_live_/i,
  );

  const config = loadSyntheticAuthConfig({
    DASHBOARD_BASE_URL: "https://app.unipost.dev/",
    DASHBOARD_TEST_CLERK_SECRET_KEY: "sk_live_valid",
  });
  assert.equal(config.baseURL, "https://app.unipost.dev");
  assert.equal(config.apiURL, "https://api.unipost.dev");
});

test("development and isolated previews require a test Clerk secret", () => {
  const development = loadSyntheticAuthConfig({
    DASHBOARD_BASE_URL: "https://dev-app.unipost.dev",
    DASHBOARD_TEST_CLERK_SECRET_KEY: "sk_test_valid",
  });
  assert.equal(development.apiURL, "https://dev-api.unipost.dev");

  const preview = loadSyntheticAuthConfig({
    DASHBOARD_BASE_URL: "https://unipost-pr-42.vercel.app",
    DASHBOARD_TEST_CLERK_SECRET_KEY: "sk_test_valid",
    EXPECTED_PREVIEW_API_URL: "https://preview-api.up.railway.app/",
    VERCEL_AUTOMATION_BYPASS_SECRET: "preview-bypass",
  });
  assert.equal(preview.apiURL, "https://preview-api.up.railway.app");
  assert.equal(preview.vercelBypassSecret, "preview-bypass");

  assert.throws(
    () => loadSyntheticAuthConfig({
      DASHBOARD_BASE_URL: "https://unipost-pr-42.vercel.app",
      DASHBOARD_TEST_CLERK_SECRET_KEY: "sk_live_wrong",
      EXPECTED_PREVIEW_API_URL: "https://preview-api.up.railway.app",
    }),
    /preview.*sk_test_/i,
  );
});

test("missing authenticated regression configuration fails instead of skipping", () => {
  assert.throws(
    () => loadSyntheticAuthConfig({ DASHBOARD_BASE_URL: "https://app.unipost.dev" }),
    /DASHBOARD_TEST_CLERK_SECRET_KEY is required/,
  );
});

test("synthetic Clerk users are passwordless and carry a reserved identity", async () => {
  const config = loadSyntheticAuthConfig({
    DASHBOARD_BASE_URL: "https://dev-app.unipost.dev",
    DASHBOARD_TEST_CLERK_SECRET_KEY: "sk_test_valid",
  });
  let request;
  const identity = await createSyntheticClerkUser(config, async (url, init) => {
    request = { url, init, body: JSON.parse(init.body) };
    return jsonResponse({ id: "user_synthetic_1" }, 200);
  }, {
    now: () => new Date("2026-07-24T12:00:00Z"),
    randomUUID: () => "00000000-0000-4000-8000-000000000001",
  });

  assert.equal(request.url, "https://api.clerk.com/v1/users");
  assert.equal(request.init.method, "POST");
  assert.equal(request.init.headers.Authorization, "Bearer sk_test_valid");
  assert.equal(request.body.skip_password_requirement, true);
  assert.equal(request.body.skip_legal_checks, true);
  assert.equal(Object.hasOwn(request.body, "password"), false);
  assert.match(request.body.email_address[0], /^codex-dashboard-regression-/);
  assert.equal(request.body.public_metadata.test_identity, "dashboard-regression");
  assert.deepEqual(identity, {
    id: "user_synthetic_1",
    email: request.body.email_address[0],
  });
});

test("Clerk cleanup deletes only the tracked user and tolerates an existing 404", async () => {
  const config = loadSyntheticAuthConfig({
    DASHBOARD_BASE_URL: "https://dev-app.unipost.dev",
    DASHBOARD_TEST_CLERK_SECRET_KEY: "sk_test_valid",
  });
  const requests = [];
  await deleteSyntheticClerkUser(config, "user_synthetic_1", async (url, init) => {
    requests.push({ url, init });
    return new Response("", { status: 404 });
  });

  assert.deepEqual(requests, [{
    url: "https://api.clerk.com/v1/users/user_synthetic_1",
    init: {
      method: "DELETE",
      headers: { Authorization: "Bearer sk_test_valid" },
    },
  }]);
});

test("Clerk API errors never include the configured secret", async () => {
  const secretKey = "sk_test_do-not-leak";
  const config = loadSyntheticAuthConfig({
    DASHBOARD_BASE_URL: "https://dev-app.unipost.dev",
    DASHBOARD_TEST_CLERK_SECRET_KEY: secretKey,
  });

  await assert.rejects(
    createSyntheticClerkUser(config, async () => jsonResponse({ errors: [{ message: "rejected" }] }, 422)),
    (error) => {
      assert.doesNotMatch(error.message, new RegExp(secretKey));
      assert.match(error.message, /Clerk POST \/users returned 422/);
      return true;
    },
  );
});

test("publishable key discovery supports protected Preview requests", async () => {
  const config = loadSyntheticAuthConfig({
    DASHBOARD_BASE_URL: "https://unipost-pr-42.vercel.app",
    DASHBOARD_TEST_CLERK_SECRET_KEY: "sk_test_valid",
    EXPECTED_PREVIEW_API_URL: "https://preview-api.up.railway.app",
    VERCEL_AUTOMATION_BYPASS_SECRET: "preview-bypass",
  });
  let request;
  const publishableKey = await loadClerkPublishableKey(config, async (url, init) => {
    request = { url, init };
    return new Response('<script>window.key="pk_test_Y2xlcmsuZXhhbXBsZSQ"</script>', { status: 200 });
  });

  assert.equal(publishableKey, "pk_test_Y2xlcmsuZXhhbXBsZSQ");
  assert.deepEqual(request, {
    url: "https://unipost-pr-42.vercel.app/pricing",
    init: { headers: { "x-vercel-protection-bypass": "preview-bypass" } },
  });
});

test("browser sign-in uses Clerk's email overload and restores the secret environment", async () => {
  const config = loadSyntheticAuthConfig({
    DASHBOARD_BASE_URL: "https://dev-app.unipost.dev",
    DASHBOARD_TEST_CLERK_SECRET_KEY: "sk_test_valid",
  });
  const page = {
    async goto(url, options) {
      assert.equal(url, "https://dev-app.unipost.dev/pricing");
      assert.deepEqual(options, { waitUntil: "domcontentloaded" });
    },
  };
  const calls = [];
  const originalSecret = process.env.CLERK_SECRET_KEY;
  process.env.CLERK_SECRET_KEY = "original-secret";
  try {
    await signInSyntheticUser(page, config, {
      id: "user_synthetic_1",
      email: "codex-dashboard-regression@example.com",
    }, {
      loadPublishableKey: async () => "pk_test_example",
      clerkSetup: async (options) => calls.push({ setup: options, activeSecret: process.env.CLERK_SECRET_KEY }),
      signIn: async (options) => calls.push({ signIn: options, activeSecret: process.env.CLERK_SECRET_KEY }),
    });
  } finally {
    if (originalSecret === undefined) delete process.env.CLERK_SECRET_KEY;
    else process.env.CLERK_SECRET_KEY = originalSecret;
  }

  assert.deepEqual(calls, [
    {
      setup: { publishableKey: "pk_test_example", secretKey: "sk_test_valid", dotenv: false },
      activeSecret: "sk_test_valid",
    },
    {
      signIn: { page, emailAddress: "codex-dashboard-regression@example.com" },
      activeSecret: "sk_test_valid",
    },
  ]);
  assert.equal(process.env.CLERK_SECRET_KEY, originalSecret);
});

test("bootstrap uses the active Clerk token before resolving a profile", async () => {
  const config = loadSyntheticAuthConfig({
    DASHBOARD_BASE_URL: "https://dev-app.unipost.dev",
    DASHBOARD_TEST_CLERK_SECRET_KEY: "sk_test_valid",
  });
  const calls = [];
  const page = {
    async evaluate() {
      return { id: "session_1", token: "session-token" };
    },
  };
  const result = await bootstrapSyntheticUser(page, config, async (url, init) => {
    calls.push({ url, init });
    if (url.endsWith("/v1/me/bootstrap")) return jsonResponse({ data: { created: true } }, 200);
    return jsonResponse({ data: [{ id: "profile_synthetic_1" }] }, 200);
  });

  assert.deepEqual(result, { token: "session-token", profileID: "profile_synthetic_1", bootstrapped: true });
  assert.deepEqual(calls.map((call) => [call.url, call.init.method]), [
    ["https://dev-api.unipost.dev/v1/me/bootstrap", "GET"],
    ["https://dev-api.unipost.dev/v1/profiles", "GET"],
  ]);
  assert.equal(calls[0].init.headers.Authorization, "Bearer session-token");
});

test("cleanup uses synchronous /v1/me after bootstrap", async () => {
  const config = loadSyntheticAuthConfig({
    DASHBOARD_BASE_URL: "https://dev-app.unipost.dev",
    DASHBOARD_TEST_CLERK_SECRET_KEY: "sk_test_valid",
  });
  const calls = [];
  await cleanupSyntheticUser(config, { id: "user_synthetic_1" }, {
    token: "session-token",
    bootstrapped: true,
  }, {
    fetchImpl: async (url, init) => {
      calls.push({ url, init });
      return new Response(null, { status: 204 });
    },
    deleteClerkUser: async () => assert.fail("Clerk fallback must not run after confirmed /v1/me cleanup"),
  });

  assert.deepEqual(calls, [{
    url: "https://dev-api.unipost.dev/v1/me",
    init: {
      method: "DELETE",
      headers: { Authorization: "Bearer session-token" },
      body: undefined,
    },
  }]);
});

test("cleanup falls back to the exact Clerk user before bootstrap", async () => {
  const deleted = [];
  await cleanupSyntheticUser(
    { clerkSecretKey: "sk_test_valid" },
    { id: "user_synthetic_1" },
    { bootstrapped: false },
    { deleteClerkUser: async (_config, userID) => deleted.push(userID) },
  );
  assert.deepEqual(deleted, ["user_synthetic_1"]);
});

test("cleanup runs after failure and preserves both errors", async () => {
  const calls = [];
  await assert.rejects(
    runWithCleanup(
      async () => {
        calls.push("cleanup");
        throw new Error("cleanup failed");
      },
      async () => {
        calls.push("acceptance");
        throw new Error("acceptance failed");
      },
    ),
    (error) => {
      assert.equal(error instanceof AggregateError, true);
      assert.match(error.message, /acceptance failed.*cleanup failed/);
      return true;
    },
  );
  assert.deepEqual(calls, ["acceptance", "cleanup"]);
});

function jsonResponse(payload, status) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
