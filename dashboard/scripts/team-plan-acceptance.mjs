#!/usr/bin/env node

import assert from "node:assert/strict";
import { randomBytes } from "node:crypto";
import { spawnSync } from "node:child_process";
import { pathToFileURL } from "node:url";

const WORKSPACE_PREFIX = "codex-team-acceptance-";

const ENVIRONMENTS = {
  development: {
    apiUrl: "https://dev-api.unipost.dev",
    appUrl: "https://dev-app.unipost.dev",
    clerkKeyPrefix: "sk_test_",
  },
  staging: {
    apiUrl: "https://staging-api.unipost.dev",
    appUrl: "https://staging-app.unipost.dev",
    clerkKeyPrefix: "sk_test_",
  },
  production: {
    apiUrl: "https://api.unipost.dev",
    appUrl: "https://app.unipost.dev",
    clerkKeyPrefix: "sk_live_",
  },
};

export function loadAcceptanceConfig(env = process.env) {
  const environment = required(env, "TEAM_ACCEPTANCE_ENV");
  const expected = ENVIRONMENTS[environment];
  if (!expected) {
    throw new Error(`TEAM_ACCEPTANCE_ENV must be one of ${Object.keys(ENVIRONMENTS).join(", ")}`);
  }

  const apiUrl = stripTrailingSlash(required(env, "TEAM_ACCEPTANCE_API_URL"));
  const appUrl = stripTrailingSlash(required(env, "TEAM_ACCEPTANCE_APP_URL"));
  if (apiUrl !== expected.apiUrl || appUrl !== expected.appUrl) {
    throw new Error(
      `${environment} acceptance domain mismatch: expected ${expected.apiUrl} and ${expected.appUrl}`,
    );
  }

  const databaseUrl = required(env, "TEAM_ACCEPTANCE_DATABASE_URL");
  let databaseHost;
  try {
    const parsedDatabaseURL = new URL(databaseUrl);
    if (!parsedDatabaseURL.protocol.startsWith("postgres")) throw new Error("unsupported protocol");
    databaseHost = parsedDatabaseURL.hostname;
  } catch {
    throw new Error("TEAM_ACCEPTANCE_DATABASE_URL must be a valid Postgres URL");
  }
  if (databaseHost.endsWith(".railway.internal")) {
    throw new Error("TEAM_ACCEPTANCE_DATABASE_URL must use the environment's public database URL");
  }
  const clerkSecretKey = required(env, "TEAM_ACCEPTANCE_CLERK_SECRET_KEY");
  if (!clerkSecretKey.startsWith(expected.clerkKeyPrefix)) {
    throw new Error(`${environment} Clerk secret does not match the expected instance type`);
  }

  const emails = {
    owner: required(env, "TEAM_ACCEPTANCE_OWNER_EMAIL"),
    admin: required(env, "TEAM_ACCEPTANCE_ADMIN_EMAIL"),
    editor: required(env, "TEAM_ACCEPTANCE_EDITOR_EMAIL"),
  };
  if (new Set(Object.values(emails).map((value) => value.toLowerCase())).size !== 3) {
    throw new Error("Team acceptance owner/admin/editor identities must be distinct");
  }
  for (const [role, email] of Object.entries(emails)) {
    if (!email.toLowerCase().startsWith(WORKSPACE_PREFIX)) {
      throw new Error(`Team acceptance ${role} identity must start with ${WORKSPACE_PREFIX}`);
    }
  }

  return { environment, apiUrl, appUrl, databaseUrl, clerkSecretKey, emails };
}

export class CleanupLedger {
  #entries = [];

  track(kind, id, cleanup) {
    const entry = { kind, id, cleanup, cleaned: false, error: null };
    this.#entries.push(entry);
    return entry;
  }

  markClean(kind, id) {
    const entry = [...this.#entries].reverse().find((item) => item.kind === kind && item.id === id && !item.cleaned);
    if (entry) entry.cleaned = true;
  }

  remaining() {
    return this.#entries.filter((entry) => !entry.cleaned).map((entry) => `${entry.kind}:${entry.id}`);
  }

  async cleanupAll() {
    for (const entry of [...this.#entries].reverse()) {
      if (entry.cleaned) continue;
      try {
        await entry.cleanup();
        entry.cleaned = true;
      } catch (error) {
        entry.error = error instanceof Error ? error : new Error(String(error));
      }
    }
  }

  assertEmpty() {
    const failures = this.#entries.filter((entry) => !entry.cleaned);
    if (failures.length === 0) return;
    const detail = failures
      .map((entry) => `${entry.kind}:${entry.id} (${entry.error?.message ?? "not cleaned"})`)
      .join(", ");
    throw new Error(`Cleanup ledger is not empty: ${detail}`);
  }
}

export async function runWithCleanup(ledger, acceptance) {
  let acceptanceError;
  try {
    await acceptance();
  } catch (error) {
    acceptanceError = error;
  }

  await ledger.cleanupAll();
  let cleanupError;
  try {
    ledger.assertEmpty();
  } catch (error) {
    cleanupError = error;
  }

  if (acceptanceError && cleanupError) {
    throw new AggregateError([acceptanceError, cleanupError], `${acceptanceError.message}; ${cleanupError.message}`);
  }
  if (acceptanceError) throw acceptanceError;
  if (cleanupError) throw cleanupError;
}

async function main() {
  const config = loadAcceptanceConfig();
  const runID = new Date().toISOString().replace(/[-:.TZ]/g, "");
  const prefix = `${WORKSPACE_PREFIX}${runID}`;
  const password = `Codex-${randomBytes(18).toString("base64url")}!9a`;
  const ledger = new CleanupLedger();
  const state = { users: {}, sessions: {}, tokens: {}, profiles: [], apiKeys: [] };

  ledger.track("database_run", prefix, async () => cleanupDatabaseRun(config, prefix, state.users));

  await runWithCleanup(ledger, async () => {
    for (const role of ["owner", "admin", "editor"]) {
      const user = await clerkRequest(config, "/users", {
        method: "POST",
        body: {
          email_address: [config.emails[role]],
          password,
          first_name: "Codex Team Acceptance",
          last_name: role,
        },
      });
      state.users[role] = user.id;
      ledger.track("clerk_user", user.id, () => clerkRequest(config, `/users/${user.id}`, { method: "DELETE", allow404: true }));

      const session = await clerkRequest(config, "/sessions", {
        method: "POST",
        body: { user_id: user.id },
      });
      state.sessions[role] = session.id;
      ledger.track("clerk_session", session.id, () => clerkRequest(config, `/sessions/${session.id}/revoke`, { method: "POST", allow404: true }));
      const sessionToken = await clerkRequest(config, `/sessions/${session.id}/tokens`, { method: "POST" });
      state.tokens[role] = sessionToken.jwt;
      assert.ok(state.tokens[role], `Clerk did not return a ${role} session JWT`);
    }

    await apiRequest(config, state.tokens.owner, "/v1/me/bootstrap");
    const workspace = await apiRequest(config, state.tokens.owner, "/v1/workspace");
    await apiRequest(config, state.tokens.owner, "/v1/workspace", {
      method: "PATCH",
      body: { name: prefix },
    });
    provisionTeamWorkspace(config, workspace.id, state.users, config.emails);

    const confirmedWorkspace = await apiRequest(config, state.tokens.owner, "/v1/workspace");
    assert.equal(confirmedWorkspace.id, workspace.id);
    assert.equal(confirmedWorkspace.name, prefix);
    assert.ok(confirmedWorkspace.name.startsWith(WORKSPACE_PREFIX), "refusing to mutate a customer workspace");

    const limits = await apiRequest(config, state.tokens.owner, "/v1/limits");
    assertTeamLimits(limits);
    const gates = await apiRequest(config, state.tokens.owner, "/v1/me/plan-gates");
    for (const gate of ["inbox", "audit_log"]) {
      assert.equal(gates.plan_gates[gate], true, `Team gate ${gate} should be enabled`);
    }

    const originalProfiles = await apiRequest(config, state.tokens.owner, "/v1/profiles");
    const defaultProfile = originalProfiles[0];
    assert.ok(defaultProfile?.id, "Team acceptance workspace needs its bootstrap profile");
    for (let index = 1; index <= 25; index += 1) {
      const profile = await apiRequest(config, state.tokens.owner, "/v1/profiles", {
        method: "POST",
        expected: 201,
        body: { name: `${prefix}-profile-${String(index).padStart(2, "0")}` },
      });
      state.profiles.push(profile.id);
      ledger.track("profile", profile.id, () => apiRequest(config, state.tokens.owner, `/v1/profiles/${profile.id}`, { method: "DELETE", expected: 204 }));
    }
    const profilesBeyondGrowth = await apiRequest(config, state.tokens.owner, "/v1/profiles");
    assert.ok(profilesBeyondGrowth.length >= 26, "Team must create profiles beyond the Growth cap");

    for (const role of ["admin", "editor"]) {
      const invitation = await apiRequest(config, state.tokens.owner, "/v1/members/invite", {
        method: "POST",
        expected: 201,
        body: { email: config.emails[role], role },
      });
      ledger.track("invite", invitation.id, () => apiRequest(config, state.tokens.owner, `/v1/members/invites/${invitation.id}`, { method: "DELETE", expected: 204 }));
      const inviteToken = invitation.url?.split("/").pop();
      assert.ok(inviteToken, `${role} invitation did not include a token URL`);
      await apiRequest(config, state.tokens[role], `/v1/invites/${inviteToken}/accept`, { method: "POST" });
      ledger.markClean("invite", invitation.id);
      ledger.track("member", state.users[role], () => apiRequest(config, state.tokens.owner, `/v1/members/${state.users[role]}`, { method: "DELETE", expected: 204 }));
    }
    removeSyntheticOwnedWorkspaces(config, workspace.id, state.users);

    const adminMe = await apiRequest(config, state.tokens.admin, "/v1/me");
    const editorMe = await apiRequest(config, state.tokens.editor, "/v1/me");
    assert.equal(adminMe.role, "admin");
    assert.equal(editorMe.role, "editor");

    await apiRequest(config, state.tokens.editor, "/v1/members/invite", {
      method: "POST",
      body: { email: `${prefix}-forbidden@example.com`, role: "editor" },
      expected: 403,
    });
    await apiRequest(config, state.tokens.editor, "/v1/api-keys", {
      method: "POST",
      body: { name: `${prefix}-forbidden`, environment: "test" },
      expected: 403,
    });
    await apiRequest(config, state.tokens.owner, `/v1/members/${state.users.owner}`, { method: "DELETE", expected: 403 });
    await apiRequest(config, state.tokens.owner, `/v1/members/${state.users.owner}/role`, {
      method: "PATCH",
      body: { role: "admin" },
      expected: 403,
    });

    const ownerKey = await createTrackedAPIKey(config, ledger, state, state.tokens.owner, `${prefix}-owner-key`);
    const adminKey = await createTrackedAPIKey(config, ledger, state, state.tokens.admin, `${prefix}-admin-key`);
    const keyWorkspace = await apiRequest(config, ownerKey.key, "/v1/workspace");
    assert.equal(keyWorkspace.id, workspace.id);

    await apiRequest(config, state.tokens.owner, `/v1/members/${state.users.admin}/role`, {
      method: "PATCH",
      body: { role: "editor" },
    });
    await apiRequest(config, adminKey.key, "/v1/api-keys", {
      method: "POST",
      body: { name: `${prefix}-demoted-key`, environment: "test" },
      expected: 403,
    });
    await apiRequest(config, state.tokens.owner, `/v1/members/${state.users.admin}/role`, {
      method: "PATCH",
      body: { role: "admin" },
    });

    const clientSecret = `${prefix}-client-secret`;
    await apiRequest(config, state.tokens.admin, "/v1/platform-credentials", {
      method: "POST",
      expected: 201,
      body: { platform: "bluesky", client_id: `${prefix}-client`, client_secret: clientSecret },
    });
    ledger.track("platform_credential", "bluesky", () => apiRequest(config, state.tokens.admin, "/v1/platform-credentials/bluesky", { method: "DELETE", expected: 204 }));

    const auditEntries = await apiRequest(config, state.tokens.editor, "/v1/audit-log?days=1&limit=500");
    for (const action of [
      "MEMBER.INVITED",
      "MEMBER.JOINED",
      "MEMBER.ROLE_CHANGED",
      "API_KEY.CREATED",
      "PLATFORM_CREDENTIAL.CREATED",
    ]) {
      assert.ok(auditEntries.some((entry) => entry.action === action), `missing audit action ${action}`);
    }
    const serializedAudit = JSON.stringify(auditEntries);
    assert.equal(serializedAudit.includes(clientSecret), false, "audit log leaked a platform client secret");
    assert.equal(serializedAudit.includes(ownerKey.key), false, "audit log leaked a raw API key");
    assert.equal(serializedAudit.includes(adminKey.key), false, "audit log leaked a raw API key");

    await verifyDashboard(config, config.emails.owner, defaultProfile.id);

    console.log(`Team acceptance passed for ${config.environment}: ${prefix}`);
  });
}

async function createTrackedAPIKey(config, ledger, state, token, name) {
  const key = await apiRequest(config, token, "/v1/api-keys", {
    method: "POST",
    expected: 201,
    body: { name, environment: "test" },
  });
  state.apiKeys.push(key.id);
  ledger.track("api_key", key.id, () => apiRequest(config, state.tokens.owner, `/v1/api-keys/${key.id}`, { method: "DELETE", expected: 204 }));
  return key;
}

function assertTeamLimits(limits) {
  assert.equal(limits.plan_id, "team");
  for (const field of [
    "max_profiles",
    "max_members",
    "max_api_keys",
    "max_webhooks",
    "max_managed_accounts",
    "max_managed_users",
  ]) {
    assert.equal(limits[field], -1, `${field} must be unlimited for Team`);
  }
  for (const field of [
    "plan_allows_twitter",
    "plan_allows_inbox",
    "plan_allows_analytics",
    "plan_allows_audit_log",
    "plan_allows_white_label",
    "plan_allows_hosted_connect_branding",
    "plan_allows_hide_powered_by",
  ]) {
    assert.equal(limits[field], true, `${field} must be enabled for Team`);
  }
  assert.equal(limits.white_label_platform_limit, -1);
}

async function verifyDashboard(config, email, profileID) {
  const { chromium } = await import("@playwright/test");
  const previousClerkSecretKey = process.env.CLERK_SECRET_KEY;
  process.env.CLERK_SECRET_KEY = config.clerkSecretKey;
  const { clerk, clerkSetup } = await import("@clerk/testing/playwright");
  const publishableKey = await loadClerkPublishableKey(config.appUrl);
  await clerkSetup({ publishableKey, secretKey: config.clerkSecretKey });
  const browser = await chromium.launch();
  try {
    const page = await browser.newPage();
    const serverFailures = [];
    page.on("response", (response) => {
      if (response.status() >= 500) serverFailures.push(`${response.status()} ${response.url()}`);
    });
    await page.goto(`${config.appUrl}/pricing`, { waitUntil: "domcontentloaded" });
    await signInSyntheticUser(page, email, clerk.signIn);
    await page.goto(config.appUrl, { waitUntil: "domcontentloaded" });
    assert.equal(
      page.url().includes("clerk") || page.url().includes("sign-in"),
      false,
      "synthetic Clerk session JWT did not authenticate the dashboard",
    );

    const routes = [
      `/projects/${profileID}`,
      `/projects/${profileID}/analytics`,
      `/projects/${profileID}/inbox`,
      `/projects/${profileID}/api-keys`,
      `/projects/${profileID}/credentials`,
      "/settings/members",
      "/settings/audit-log",
    ];
    for (const route of routes) {
      await page.goto(`${config.appUrl}${route}`, { waitUntil: "networkidle" });
      const body = await page.locator("body").innerText();
      assert.ok(body.trim().length > 0, `${route} rendered an empty body`);
      assert.equal(/application error|internal server error/i.test(body), false, `${route} rendered an error state`);
      assert.equal(/Audit Log is a Team feature/.test(body), false, `${route} incorrectly gated Team Audit Log`);
    }
    assert.deepEqual(serverFailures, [], `dashboard produced 5xx responses: ${serverFailures.join(", ")}`);
  } finally {
    await browser.close();
    if (previousClerkSecretKey === undefined) delete process.env.CLERK_SECRET_KEY;
    else process.env.CLERK_SECRET_KEY = previousClerkSecretKey;
  }
}

export async function signInSyntheticUser(page, emailAddress, signIn) {
  await signIn({ page, emailAddress });
}

async function loadClerkPublishableKey(appUrl) {
  const response = await fetch(`${appUrl}/pricing`);
  assert.equal(response.ok, true, `could not load ${appUrl}/pricing to discover Clerk configuration`);
  return extractClerkPublishableKey(await response.text());
}

export function extractClerkPublishableKey(html) {
  const publishableKey = html.match(/pk_(?:test|live)_[A-Za-z0-9_-]+/)?.[0];
  assert.ok(publishableKey, "deployed dashboard did not expose a Clerk publishable key");
  return publishableKey;
}

async function apiRequest(config, token, path, options = {}) {
  const method = options.method ?? "GET";
  const expected = options.expected ?? 200;
  const response = await fetch(`${config.apiUrl}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${token}`,
      ...(options.body ? { "Content-Type": "application/json" } : {}),
    },
    body: options.body ? JSON.stringify(options.body) : undefined,
  });
  const text = await response.text();
  if (response.status !== expected) {
    throw new Error(`${method} ${path} returned ${response.status}, expected ${expected}: ${text.slice(0, 500)}`);
  }
  if (!text) return undefined;
  const payload = JSON.parse(text);
  return Object.hasOwn(payload, "data") ? payload.data : payload;
}

export async function clerkRequest(config, path, options = {}, fetchImpl = fetch) {
  const method = options.method ?? "GET";
  const response = await fetchImpl(`https://api.clerk.com/v1${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${config.clerkSecretKey}`,
      ...(!["GET", "HEAD"].includes(method.toUpperCase()) ? { "Content-Type": "application/json" } : {}),
    },
    body: options.body ? JSON.stringify(options.body) : undefined,
  });
  if (options.allow404 && response.status === 404) return undefined;
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`Clerk ${options.method ?? "GET"} ${path} returned ${response.status}: ${text.slice(0, 500)}`);
  }
  return text ? JSON.parse(text) : undefined;
}

function provisionTeamWorkspace(config, workspaceID, users, emails) {
  runPSQL(config, `
    INSERT INTO users (id, email, name)
    VALUES (:'admin_id', :'admin_email', 'Codex Team Acceptance admin'),
           (:'editor_id', :'editor_email', 'Codex Team Acceptance editor')
    ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email, name = EXCLUDED.name;
    INSERT INTO subscriptions (workspace_id, plan_id, status)
    VALUES (:'workspace_id', 'team', 'active')
    ON CONFLICT (workspace_id) DO UPDATE
      SET plan_id = 'team', status = 'active', updated_at = NOW();
    DELETE FROM workspaces
    WHERE user_id IN (:'admin_id', :'editor_id') AND id <> :'workspace_id';
  `, {
    workspace_id: workspaceID,
    admin_id: users.admin,
    editor_id: users.editor,
    admin_email: emails.admin,
    editor_email: emails.editor,
  });
}

function removeSyntheticOwnedWorkspaces(config, workspaceID, users) {
  runPSQL(config, `
    DELETE FROM workspaces
    WHERE user_id IN (:'admin_id', :'editor_id') AND id <> :'workspace_id';
  `, { workspace_id: workspaceID, admin_id: users.admin, editor_id: users.editor });
}

function cleanupDatabaseRun(config, prefix, users) {
  const userIDs = Object.values(users);
  if (userIDs.length > 0) {
    runPSQL(config, "DELETE FROM users WHERE id = ANY(string_to_array(:'user_ids', ','));", {
      user_ids: userIDs.join(","),
    });
  }
  const remaining = Number(runPSQL(config, `
    SELECT COUNT(*)
    FROM (
      SELECT id FROM workspaces WHERE name LIKE :'prefix' || '%'
      UNION ALL
      SELECT id FROM profiles WHERE name LIKE :'prefix' || '%'
      UNION ALL
      SELECT id FROM api_keys WHERE name LIKE :'prefix' || '%'
      UNION ALL
      SELECT id FROM workspace_invites WHERE email LIKE :'prefix' || '%'
    ) leftovers;
  `, { prefix }).trim() || "0");
  assert.equal(remaining, 0, `${remaining} removable database artifacts remain for ${prefix}`);
}

function runPSQL(config, sql, variables = {}) {
  const args = ["--no-psqlrc", "--set=ON_ERROR_STOP=1", "--tuples-only", "--no-align"];
  for (const [key, value] of Object.entries(variables)) {
    if (!/^[a-z_]+$/.test(key)) throw new Error(`Unsafe psql variable name: ${key}`);
    args.push(`--set=${key}=${value}`);
  }
  args.push("--dbname", config.databaseUrl);
  const result = spawnSync("psql", args, { input: sql, encoding: "utf8", timeout: 30_000 });
  if (result.error || result.status !== 0) {
    const detail = (result.stderr || result.error?.message || "unknown psql error").replaceAll(config.databaseUrl, "[REDACTED_DATABASE_URL]");
    throw new Error(`Acceptance database operation failed: ${detail.trim()}`);
  }
  return result.stdout;
}

function required(env, name) {
  const value = env[name]?.trim();
  if (!value) throw new Error(`${name} is required for release acceptance`);
  return value;
}

function stripTrailingSlash(value) {
  return value.replace(/\/$/, "");
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    console.error(error);
    process.exitCode = 1;
  });
}
