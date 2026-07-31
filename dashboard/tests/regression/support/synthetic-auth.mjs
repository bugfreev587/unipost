import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";

const CLERK_API_URL = "https://api.clerk.com/v1";
const SYNTHETIC_PREFIX = "codex-dashboard-regression-";

const STABLE_ENVIRONMENTS = new Map([
  ["app.unipost.dev", {
    name: "production",
    apiURL: "https://api.unipost.dev",
    clerkSecretPrefix: "sk_live_",
  }],
  ["dev-app.unipost.dev", {
    name: "development",
    apiURL: "https://dev-api.unipost.dev",
    clerkSecretPrefix: "sk_test_",
  }],
  ["staging-app.unipost.dev", {
    name: "staging",
    apiURL: "https://staging-api.unipost.dev",
    clerkSecretPrefix: "sk_test_",
  }],
]);

export function loadSyntheticAuthConfig(env = process.env) {
  const baseURL = stripTrailingSlash(required(env, "DASHBOARD_BASE_URL"));
  const clerkSecretKey = required(env, "DASHBOARD_TEST_CLERK_SECRET_KEY");
  const clerkPublishableKey = env.DASHBOARD_TEST_CLERK_PUBLISHABLE_KEY?.trim() || "";
  let parsedBaseURL;
  try {
    parsedBaseURL = new URL(baseURL);
  } catch {
    throw new Error("DASHBOARD_BASE_URL must be a valid absolute URL");
  }

  const stable = STABLE_ENVIRONMENTS.get(parsedBaseURL.hostname);
  const isPreview = parsedBaseURL.hostname.endsWith(".vercel.app");
  if (!stable && !isPreview) {
    throw new Error(`Authenticated Dashboard regression does not support host ${parsedBaseURL.hostname}`);
  }

  const environment = stable?.name ?? "preview";
  const clerkSecretPrefix = stable?.clerkSecretPrefix ?? "sk_test_";
  const clerkPublishablePrefix = clerkSecretPrefix.replace("sk_", "pk_");
  if (!clerkSecretKey.startsWith(clerkSecretPrefix)) {
    throw new Error(`${environment} Dashboard regression requires a ${clerkSecretPrefix} Clerk secret`);
  }
  if (clerkPublishableKey && !clerkPublishableKey.startsWith(clerkPublishablePrefix)) {
    throw new Error(`${environment} Dashboard regression requires a ${clerkPublishablePrefix} Clerk publishable key`);
  }

  const apiURL = stable?.apiURL ?? stripTrailingSlash(required(env, "EXPECTED_PREVIEW_API_URL"));
  if (isPreview) {
    let parsedAPIURL;
    try {
      parsedAPIURL = new URL(apiURL);
    } catch {
      throw new Error("EXPECTED_PREVIEW_API_URL must be a valid absolute URL");
    }
    if (!parsedAPIURL.hostname.endsWith(".up.railway.app")) {
      throw new Error("Preview Dashboard regression requires its isolated Railway API URL");
    }
  }

  return {
    environment,
    baseURL,
    apiURL,
    clerkSecretKey,
    clerkPublishableKey,
    vercelBypassSecret: env.VERCEL_AUTOMATION_BYPASS_SECRET?.trim() || "",
  };
}

export async function loadClerkPublishableKey(config, fetchImpl = fetch) {
  const headers = config.vercelBypassSecret
    ? { "x-vercel-protection-bypass": config.vercelBypassSecret }
    : undefined;
  const response = await fetchImpl(`${config.baseURL}/pricing`, headers ? { headers } : undefined);
  if (!response.ok) {
    throw new Error(`Could not load ${config.baseURL}/pricing to discover Clerk configuration (${response.status})`);
  }
  const html = await response.text();
  const publishableKey = html.match(/pk_(?:test|live)_[A-Za-z0-9_-]+/)?.[0];
  if (!publishableKey) {
    throw new Error("Deployed Dashboard did not expose a Clerk publishable key");
  }
  return publishableKey;
}

export async function createSyntheticClerkUser(config, fetchImpl = fetch, runtime = {}) {
  const now = runtime.now?.() ?? new Date();
  const uuid = (runtime.randomUUID?.() ?? randomUUID()).replaceAll("-", "").slice(0, 16);
  const timestamp = now.toISOString().replace(/[^0-9]/g, "");
  const email = `${SYNTHETIC_PREFIX}${timestamp}-${uuid}@example.com`;
  const user = await clerkRequest(config, "/users", {
    method: "POST",
    body: {
      email_address: [email],
      first_name: "Codex Dashboard",
      last_name: "Regression",
      skip_password_requirement: true,
      skip_legal_checks: true,
      public_metadata: { test_identity: "dashboard-regression" },
    },
  }, fetchImpl);
  if (!user?.id) {
    throw new Error("Clerk user creation did not return a user ID");
  }
  return { id: user.id, email };
}

export async function deleteSyntheticClerkUser(config, userID, fetchImpl = fetch) {
  if (!userID || !userID.startsWith("user_")) {
    throw new Error("Refusing to delete an invalid Clerk user ID");
  }
  await clerkRequest(config, `/users/${encodeURIComponent(userID)}`, {
    method: "DELETE",
    allow404: true,
  }, fetchImpl);
}

export async function signInSyntheticUser(page, config, identity, dependencies = {}) {
  const clerkModule = dependencies.clerkSetup && dependencies.signIn
    ? null
    : await import("@clerk/testing/playwright");
  const clerkSetup = dependencies.clerkSetup ?? clerkModule.clerkSetup;
  const signIn = dependencies.signIn ?? clerkModule.clerk.signIn;
  const loadPublishableKey = dependencies.loadPublishableKey ?? loadClerkPublishableKey;
  await establishPreviewBypassCookie(page, config);
  const publishableKey = config.clerkPublishableKey || await loadPublishableKey(config);

  await withClerkSecret(config.clerkSecretKey, async () => {
    await clerkSetup({
      publishableKey,
      secretKey: config.clerkSecretKey,
      dotenv: false,
    });
    await page.goto(`${config.baseURL}/pricing`, { waitUntil: "domcontentloaded" });
    await signIn({ page, emailAddress: identity.email });
  });
}

async function establishPreviewBypassCookie(page, config) {
  if (!config.vercelBypassSecret) return;
  const response = await page.request.get(`${config.baseURL}/pricing`, {
    headers: {
      "x-vercel-protection-bypass": config.vercelBypassSecret,
      "x-vercel-set-bypass-cookie": "true",
    },
    maxRedirects: 0,
  });
  if (!response.ok() && ![307, 308].includes(response.status())) {
    throw new Error(`Could not establish Vercel Preview access (${response.status()})`);
  }
}

export async function withClerkSecret(secretKey, operation) {
  const previous = process.env.CLERK_SECRET_KEY;
  process.env.CLERK_SECRET_KEY = secretKey;
  try {
    return await operation();
  } finally {
    if (previous === undefined) delete process.env.CLERK_SECRET_KEY;
    else process.env.CLERK_SECRET_KEY = previous;
  }
}

export async function readActiveClerkSession(page) {
  const session = await page.evaluate(async () => {
    const activeSession = window.Clerk?.session;
    return {
      id: activeSession?.id ?? null,
      token: activeSession ? await activeSession.getToken() : null,
    };
  });
  assert.ok(session.id && session.token, "Clerk browser sign-in did not create an active session");
  return session;
}

export async function bootstrapSyntheticUser(page, config, fetchImpl = fetch) {
  const session = await readActiveClerkSession(page);
  await apiRequest(config, session.token, "/v1/me/bootstrap", { fetchImpl });
  const profiles = await apiRequest(config, session.token, "/v1/profiles", { fetchImpl });
  const profileID = profiles?.find((profile) => typeof profile?.id === "string")?.id;
  if (!profileID) {
    throw new Error("Synthetic Dashboard user did not receive a default profile after bootstrap");
  }
  return { token: session.token, profileID, bootstrapped: true };
}

export async function cleanupSyntheticUser(config, identity, state = {}, dependencies = {}) {
  const fetchImpl = dependencies.fetchImpl ?? fetch;
  const deleteClerkUser = dependencies.deleteClerkUser ?? deleteSyntheticClerkUser;
  if (!state.bootstrapped || !state.token) {
    await deleteClerkUser(config, identity.id, fetchImpl);
    return;
  }

  try {
    await apiRequest(config, state.token, "/v1/me", {
      method: "DELETE",
      expected: 204,
      fetchImpl,
    });
  } catch (accountDeleteError) {
    try {
      await deleteClerkUser(config, identity.id, fetchImpl);
    } catch (clerkFallbackError) {
      throw new AggregateError(
        [accountDeleteError, clerkFallbackError],
        `${accountDeleteError.message}; Clerk fallback failed: ${clerkFallbackError.message}`,
      );
    }
    throw accountDeleteError;
  }
}

export async function apiRequest(config, token, path, options = {}) {
  const method = options.method ?? "GET";
  const expected = options.expected ?? 200;
  const fetchImpl = options.fetchImpl ?? fetch;
  const response = await fetchImpl(`${config.apiURL}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${token}`,
      ...(options.body === undefined ? {} : { "Content-Type": "application/json" }),
    },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });
  const text = await response.text();
  if (response.status !== expected) {
    throw new Error(`${method} ${path} returned ${response.status}, expected ${expected}: ${text.slice(0, 500)}`);
  }
  if (!text) return undefined;
  const payload = JSON.parse(text);
  return Object.hasOwn(payload, "data") ? payload.data : payload;
}

export async function runWithCleanup(cleanup, acceptance) {
  let acceptanceError;
  try {
    await acceptance();
  } catch (error) {
    acceptanceError = error;
  }

  let cleanupError;
  try {
    await cleanup();
  } catch (error) {
    cleanupError = error;
  }

  if (acceptanceError && cleanupError) {
    throw new AggregateError(
      [acceptanceError, cleanupError],
      `${acceptanceError.message}; ${cleanupError.message}`,
    );
  }
  if (acceptanceError) throw acceptanceError;
  if (cleanupError) throw cleanupError;
}

async function clerkRequest(config, path, options = {}, fetchImpl = fetch) {
  const method = options.method ?? "GET";
  const response = await fetchImpl(`${CLERK_API_URL}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${config.clerkSecretKey}`,
      ...(options.body === undefined ? {} : { "Content-Type": "application/json" }),
    },
    ...(options.body === undefined ? {} : { body: JSON.stringify(options.body) }),
  });
  if (options.allow404 && response.status === 404) return undefined;
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`Clerk ${method} ${path} returned ${response.status}: ${text.slice(0, 500)}`);
  }
  return text ? JSON.parse(text) : undefined;
}

function required(env, name) {
  const value = env[name]?.trim();
  if (!value) throw new Error(`${name} is required for authenticated Dashboard regression`);
  return value;
}

function stripTrailingSlash(value) {
  return value.replace(/\/+$/, "");
}
