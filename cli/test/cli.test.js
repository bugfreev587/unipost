import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { main } from "../src/cli.js";

let activeFixture = null;

async function runCli(args, options = {}) {
  const env = {
    ...process.env,
    UNIPOST_API_KEY: "",
    UNIPOST_BASE_URL: "",
    NO_COLOR: "1",
    ...options.env,
  };
  let stdout = "";
  let stderr = "";

  const code = await main(args, {
    env,
    fetchImpl: options.fetchImpl || (activeFixture ? fixtureFetch : globalThis.fetch.bind(globalThis)),
    stdout: {
      write(chunk) {
        stdout += chunk;
      },
    },
    stderr: {
      write(chunk) {
        stderr += chunk;
      },
    },
  });

  return { code, stdout, stderr };
}

async function withServer(handler, callback) {
  const previousFixture = activeFixture;
  const fixture = { handler, requests: [] };
  activeFixture = fixture;
  try {
    return await callback("http://unipost-cli-test.local", fixture.requests);
  } finally {
    activeFixture = previousFixture;
  }
}

async function fixtureFetch(url, init = {}) {
  const fixture = activeFixture;
  const parsedUrl = new URL(url);
  const body = init.body ? String(init.body) : "";
  const req = {
    method: init.method || "GET",
    url: `${parsedUrl.pathname}${parsedUrl.search}`,
    headers: lowercaseHeaders(init.headers || {}),
    async *[Symbol.asyncIterator]() {
      if (body) {
        yield Buffer.from(body);
      }
    },
  };
  fixture.requests.push(req);

  return await new Promise((resolve, reject) => {
    let status = 200;
    let headers = {};
    const res = {
      writeHead(nextStatus, nextHeaders = {}) {
        status = nextStatus;
        headers = { ...headers, ...nextHeaders };
      },
      end(payload = "") {
        resolve(new Response(String(payload), { status, headers }));
      },
    };

    Promise.resolve(fixture.handler(req, res)).catch(reject);
  });
}

function lowercaseHeaders(headers) {
  if (headers instanceof Headers) {
    return Object.fromEntries([...headers.entries()].map(([key, value]) => [key.toLowerCase(), value]));
  }
  return Object.fromEntries(Object.entries(headers).map(([key, value]) => [key.toLowerCase(), value]));
}

function writeJson(res, status, body, headers = {}) {
  res.writeHead(status, {
    "Content-Type": "application/json",
    "X-Request-Id": "req_test_123",
    ...headers,
  });
  res.end(JSON.stringify(body));
}

async function readRequestJson(req) {
  let body = "";
  for await (const chunk of req) {
    body += chunk;
  }
  return body ? JSON.parse(body) : {};
}

async function withTempConfig(callback) {
  const dir = await mkdtemp(join(tmpdir(), "unipost-cli-test-"));
  try {
    return await callback(dir);
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
}

test("prints the CLI version", async () => {
  const result = await runCli(["--version"]);

  assert.equal(result.code, 0);
  assert.match(result.stdout.trim(), /^\d+\.\d+\.\d+/);
  assert.equal(result.stderr, "");
});

test("auth status --json fails with a stable envelope when credentials are missing", async () => {
  const result = await runCli(["auth", "status", "--json", "--base-url", "http://127.0.0.1:65534"]);

  assert.equal(result.code, 4);
  const body = JSON.parse(result.stdout);
  assert.equal(body.ok, false);
  assert.equal(body.error.code, "unauthorized");
  assert.equal(body.error.normalized_code, "unauthorized");
  assert.match(body.error.hint, /UNIPOST_API_KEY/);
  assert.equal(body.meta.command, "auth status");
  assert.equal(body.meta.base_url, "http://127.0.0.1:65534");
});

test("--output rejects unsupported formats with exit 2", async () => {
  const result = await runCli(["auth", "status", "--output", "xml"]);

  assert.equal(result.code, 2);
  assert.equal(result.stdout, "");
  assert.match(result.stderr, /--output must be one of/);
});

test("auth status --json validates the API key against /v1/workspace", async () => {
  await withServer((req, res) => {
    assert.equal(req.method, "GET");
    assert.equal(req.url, "/v1/workspace");
    assert.equal(req.headers.authorization, "Bearer up_test_valid");
    writeJson(res, 200, {
      data: { id: "ws_test", name: "Test Workspace" },
      request_id: "req_body_123",
    });
  }, async (baseUrl) => {
    const result = await runCli(["auth", "status", "--json", "--base-url", baseUrl], {
      env: { UNIPOST_API_KEY: "up_test_valid" },
    });

    assert.equal(result.code, 0);
    const body = JSON.parse(result.stdout);
    assert.equal(body.ok, true);
    assert.equal(body.data.authenticated, true);
    assert.equal(body.data.workspace.id, "ws_test");
    assert.equal(body.meta.request_id, "req_body_123");
    assert.equal(body.meta.command, "auth status");
  });
});

test("auth status preserves backend normalized errors and maps unauthorized to exit 4", async () => {
  await withServer((req, res) => {
    writeJson(res, 401, {
      error: {
        code: "UNAUTHORIZED",
        normalized_code: "unauthorized",
        message: "API key is invalid.",
      },
      request_id: "req_auth_fail",
    });
  }, async (baseUrl) => {
    const result = await runCli(["auth", "status", "--json", "--base-url", baseUrl], {
      env: { UNIPOST_API_KEY: "up_test_bad" },
    });

    assert.equal(result.code, 4);
    const body = JSON.parse(result.stdout);
    assert.equal(body.ok, false);
    assert.equal(body.error.code, "unauthorized");
    assert.equal(body.error.normalized_code, "unauthorized");
    assert.equal(body.error.message, "API key is invalid.");
    assert.equal(body.meta.request_id, "req_auth_fail");
  });
});

test("auth list/use tracks local workspace defaults without storing API keys", async () => {
  await withTempConfig(async (configDir) => {
    await withServer((req, res) => {
      assert.equal(req.method, "GET");
      assert.equal(req.url, "/v1/workspace");
      writeJson(res, 200, {
        data: { id: "ws_auth", name: "Auth Workspace" },
        request_id: "req_auth_list",
      });
    }, async (baseUrl) => {
      const env = {
        UNIPOST_API_KEY: "up_test_valid",
        UNIPOST_CONFIG_DIR: configDir,
      };

      const list = await runCli(["auth", "list", "--json", "--base-url", baseUrl], { env });
      assert.equal(list.code, 0);
      const listBody = JSON.parse(list.stdout);
      assert.equal(listBody.data.credentials[0].workspace_id, "ws_auth");
      assert.equal(listBody.data.credentials[0].credential_source, "env");
      assert.equal(JSON.stringify(listBody).includes("up_test_valid"), false);

      const use = await runCli(["auth", "use", "ws_auth", "--json", "--base-url", baseUrl], { env });
      assert.equal(use.code, 0);
      assert.equal(JSON.parse(use.stdout).data.default_workspace_id, "ws_auth");

      const config = JSON.parse(await readFile(join(configDir, "config.json"), "utf8"));
      assert.equal(config.default_workspace_id, "ws_auth");
      assert.equal(JSON.stringify(config).includes("up_test_valid"), false);
    });
  });
});

test("--field prints a selected value from the JSON envelope", async () => {
  await withServer((req, res) => {
    writeJson(res, 200, {
      data: { id: "ws_field", name: "Field Workspace" },
      request_id: "req_field",
    });
  }, async (baseUrl) => {
    const result = await runCli(["auth", "status", "--output", "json", "--field", "data.workspace.id", "--base-url", baseUrl], {
      env: { UNIPOST_API_KEY: "up_test_valid" },
    });

    assert.equal(result.code, 0);
    assert.equal(result.stdout, "ws_field\n");
    assert.equal(result.stderr, "");
  });
});

test("doctor --json reports CLI, API reachability, auth and workspace checks", async () => {
  await withServer((req, res) => {
    if (req.url === "/health") {
      writeJson(res, 200, { ok: true });
      return;
    }
    if (req.url === "/v1/workspace") {
      writeJson(res, 200, {
        data: { id: "ws_doctor", name: "Doctor Workspace" },
        request_id: "req_workspace",
      }, {
        "X-UniPost-RateLimit-Limit": "60",
        "X-UniPost-RateLimit-Remaining": "59",
      });
      return;
    }
    writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: "Not found" } });
  }, async (baseUrl) => {
    const result = await runCli(["doctor", "--json", "--base-url", baseUrl], {
      env: { UNIPOST_API_KEY: "up_test_valid" },
    });

    assert.equal(result.code, 0);
    const body = JSON.parse(result.stdout);
    assert.equal(body.ok, true);
    assert.deepEqual(body.data.checks.map((check) => check.id), [
      "cli_version",
      "api_reachability",
      "auth",
      "workspace",
      "rate_limit_headers",
      "request_id",
    ]);
    assert.equal(body.data.checks.every((check) => check.status === "pass"), true);
    assert.equal(body.data.workspace.id, "ws_doctor");
  });
});

test("doctor --json exits 4 with failing auth checks when credentials are missing", async () => {
  await withServer((req, res) => {
    assert.equal(req.url, "/health");
    writeJson(res, 200, { ok: true });
  }, async (baseUrl) => {
    const result = await runCli(["doctor", "--json", "--base-url", baseUrl]);

    assert.equal(result.code, 4);
    const body = JSON.parse(result.stdout);
    assert.equal(body.ok, true);
    assert.equal(body.data.checks.find((check) => check.id === "auth").status, "fail");
    assert.equal(body.data.telemetry.enabled, false);
    assert.equal(body.meta.command, "doctor");
  });
});

test("doctor without --json keeps human diagnostic output when auth is missing", async () => {
  await withServer((req, res) => {
    assert.equal(req.url, "/health");
    writeJson(res, 200, { ok: true });
  }, async (baseUrl) => {
    const result = await runCli(["doctor", "--base-url", baseUrl]);

    assert.equal(result.code, 4);
    assert.match(result.stdout, /UniPost doctor/);
    assert.match(result.stdout, /FAIL auth/);
    assert.doesNotThrow(() => {
      assert.throws(() => JSON.parse(result.stdout));
    });
  });
});

test("doctor retries idempotent API checks using Retry-After before failing", async () => {
  let healthAttempts = 0;
  await withServer((req, res) => {
    if (req.url === "/health") {
      healthAttempts += 1;
      if (healthAttempts === 1) {
        writeJson(res, 429, {
          error: {
            code: "RATE_LIMITED",
            normalized_code: "request_rate_limited",
            message: "Retry later.",
          },
          request_id: "req_retry",
        }, { "Retry-After": "0" });
        return;
      }
      writeJson(res, 200, { ok: true });
      return;
    }
    if (req.url === "/v1/workspace") {
      writeJson(res, 200, {
        data: { id: "ws_retry", name: "Retry Workspace" },
        request_id: "req_retry_workspace",
      }, {
        "X-UniPost-RateLimit-Limit": "60",
        "X-UniPost-RateLimit-Remaining": "58",
      });
      return;
    }
    writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: "Not found" } });
  }, async (baseUrl) => {
    const result = await runCli(["doctor", "--json", "--base-url", baseUrl], {
      env: { UNIPOST_API_KEY: "up_test_valid" },
    });

    assert.equal(result.code, 0);
    assert.equal(healthAttempts, 2);
    const body = JSON.parse(result.stdout);
    assert.equal(body.data.checks.find((check) => check.id === "api_reachability").status, "pass");
  });
});

test("--no-telemetry overrides an enabled telemetry environment", async () => {
  await withServer((req, res) => {
    if (req.url === "/health") {
      writeJson(res, 200, { ok: true });
      return;
    }
    writeJson(res, 200, {
      data: { id: "ws_telemetry", name: "Telemetry Workspace" },
      request_id: "req_telemetry",
    }, {
      "X-UniPost-RateLimit-Limit": "60",
      "X-UniPost-RateLimit-Remaining": "59",
    });
  }, async (baseUrl) => {
    const result = await runCli(["doctor", "--json", "--no-telemetry", "--base-url", baseUrl], {
      env: {
        UNIPOST_API_KEY: "up_test_valid",
        UNIPOST_TELEMETRY: "1",
      },
    });

    assert.equal(result.code, 0);
    const body = JSON.parse(result.stdout);
    assert.equal(body.data.telemetry.enabled, false);
    assert.equal(body.meta.telemetry.reason, "disabled");
  });
});

test("completion zsh prints a shell completion script without calling the API", async () => {
  const result = await runCli(["completion", "zsh"]);

  assert.equal(result.code, 0);
  assert.match(result.stdout, /#compdef unipost/);
  assert.match(result.stdout, /auth status/);
  assert.match(result.stdout, /examples mcp\.claude-code/);
  assert.match(result.stdout, /agent execute/);
  assert.match(result.stdout, /agent mcp-test/);
  assert.match(result.stdout, /agent install/);
  assert.match(result.stdout, /--plan/);
  assert.match(result.stdout, /--client/);
  assert.match(result.stdout, /--limit/);
  assert.match(result.stdout, /--cursor/);
  assert.match(result.stdout, /--all/);
  assert.equal(result.stderr, "");
});

test("profiles create/use/list persists a local default profile without storing secrets", async () => {
  await withTempConfig(async (configDir) => {
    const bodies = [];
    await withServer(async (req, res) => {
      if (req.method === "POST" && req.url === "/v1/profiles") {
        bodies.push(await readRequestJson(req));
        writeJson(res, 201, {
          data: {
            id: "pr_studio",
            workspace_id: "ws_test",
            name: "Studio",
            account_count: 0,
          },
          request_id: "req_profile_create",
        });
        return;
      }
      if (req.method === "GET" && req.url === "/v1/profiles") {
        writeJson(res, 200, {
          data: [
            {
              id: "pr_studio",
              workspace_id: "ws_test",
              name: "Studio",
              account_count: 1,
            },
          ],
          meta: { total: 1, limit: 1 },
          request_id: "req_profile_list",
        });
        return;
      }
      writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: "Not found" } });
    }, async (baseUrl) => {
      const env = {
        UNIPOST_API_KEY: "up_test_valid",
        UNIPOST_CONFIG_DIR: configDir,
      };

      const create = await runCli(["profiles", "create", "--name", "Studio", "--json", "--base-url", baseUrl], { env });
      assert.equal(create.code, 0);
      assert.deepEqual(bodies[0], { name: "Studio" });
      assert.equal(JSON.parse(create.stdout).data.profile.id, "pr_studio");

      const use = await runCli(["profiles", "use", "pr_studio", "--json", "--base-url", baseUrl], { env });
      assert.equal(use.code, 0);
      assert.equal(JSON.parse(use.stdout).data.default_profile_id, "pr_studio");

      const config = JSON.parse(await readFile(join(configDir, "config.json"), "utf8"));
      assert.equal(config.default_profile_id, "pr_studio");
      assert.equal(JSON.stringify(config).includes("up_test_valid"), false);

      const list = await runCli(["profiles", "list", "--json", "--base-url", baseUrl], { env });
      assert.equal(list.code, 0);
      const body = JSON.parse(list.stdout);
      assert.equal(body.data.profiles[0].id, "pr_studio");
      assert.equal(body.meta.pagination.total, 1);
    });
  });
});

test("connect create/get/wait uses real endpoints and normalizes canceled status aliases", async () => {
  const seenBodies = [];
  let sessionReads = 0;
  await withServer(async (req, res) => {
    if (req.method === "GET" && req.url === "/v1/profiles") {
      writeJson(res, 200, {
        data: [{ id: "pr_default", name: "Default", workspace_id: "ws_test", account_count: 0 }],
        request_id: "req_profiles",
      });
      return;
    }
    if (req.method === "POST" && req.url === "/v1/connect/sessions") {
      seenBodies.push(await readRequestJson(req));
      writeJson(res, 201, {
        data: {
          id: "cs_test",
          platform: "linkedin",
          status: "pending",
          url: "https://dev-app.unipost.dev/connect/linkedin?session=cs_test",
          expires_at: "2026-06-02T20:00:00Z",
        },
        request_id: "req_connect_create",
      });
      return;
    }
    if (req.method === "GET" && req.url === "/v1/connect/sessions/cs_test") {
      sessionReads += 1;
      writeJson(res, 200, {
        data: sessionReads === 1 ? {
          id: "cs_test",
          platform: "linkedin",
          status: "cancelled",
        } : {
          id: "cs_test",
          platform: "linkedin",
          status: "completed",
          completed_social_account_id: "sa_linkedin",
          managed_account_id: "sa_linkedin",
          completed_at: "2026-06-02T19:00:00Z",
        },
        request_id: "req_connect_get",
      });
      return;
    }
    writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: "Not found" } });
  }, async (baseUrl) => {
    const env = { UNIPOST_API_KEY: "up_test_valid", UNIPOST_TEST_POLL_MS: "1" };
    const create = await runCli(["connect", "create", "--platform", "linkedin", "--json", "--base-url", baseUrl], { env });
    assert.equal(create.code, 0);
    assert.deepEqual(seenBodies[0], {
      platform: "linkedin",
      profile_id: "pr_default",
      external_user_id: "cli-local-user",
      allow_quickstart_creds: true,
    });
    assert.equal(JSON.parse(create.stdout).data.session.id, "cs_test");

    const get = await runCli(["connect", "get", "cs_test", "--json", "--base-url", baseUrl], { env });
    assert.equal(get.code, 0);
    assert.equal(JSON.parse(get.stdout).data.session.status, "canceled");

    const wait = await runCli(["connect", "wait", "cs_test", "--timeout", "2", "--json", "--base-url", baseUrl], { env });
    assert.equal(wait.code, 0);
    const body = JSON.parse(wait.stdout);
    assert.equal(body.data.session.status, "completed");
    assert.equal(body.data.session.completed_social_account_id, "sa_linkedin");
  });
});

test("accounts list and accounts get use the list endpoint with local filtering", async () => {
  const urls = [];
  await withServer((req, res) => {
    urls.push(req.url);
    if (req.method === "GET" && req.url === "/v1/accounts") {
      writeJson(res, 200, {
        data: [
          { id: "sa_1", platform: "linkedin", account_name: "Team", profile_id: "pr_1", status: "active" },
          { id: "sa_2", platform: "twitter", account_name: "News", profile_id: "pr_1", status: "active" },
        ],
        meta: { total: 2, limit: 2 },
        request_id: "req_accounts",
      });
      return;
    }
    writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: "Not found" } });
  }, async (baseUrl) => {
    const env = { UNIPOST_API_KEY: "up_test_valid" };
    const list = await runCli(["accounts", "list", "--json", "--base-url", baseUrl], { env });
    assert.equal(list.code, 0);
    assert.equal(JSON.parse(list.stdout).data.accounts.length, 2);

    const get = await runCli(["accounts", "get", "sa_2", "--json", "--base-url", baseUrl], { env });
    assert.equal(get.code, 0);
    assert.equal(JSON.parse(get.stdout).data.account.platform, "twitter");
    assert.deepEqual(urls, ["/v1/accounts", "/v1/accounts"]);
  });
});

test("posts validate and draft send the legacy account_ids request shape", async () => {
  const bodies = [];
  await withServer(async (req, res) => {
    if (req.method === "POST" && (req.url === "/v1/posts/validate" || req.url === "/v1/posts")) {
      bodies.push({ url: req.url, body: await readRequestJson(req) });
      writeJson(res, req.url === "/v1/posts" ? 201 : 200, {
        data: req.url === "/v1/posts/validate" ? {
          valid: true,
          errors: [],
          warnings: [],
        } : {
          id: "post_draft",
          caption: "Launch update",
          status: "draft",
          profile_ids: ["pr_1"],
        },
        request_id: "req_posts",
      });
      return;
    }
    writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: "Not found" } });
  }, async (baseUrl) => {
    const env = { UNIPOST_API_KEY: "up_test_valid" };
    const validate = await runCli(["posts", "validate", "--account", "sa_1", "--caption", "Launch update", "--json", "--base-url", baseUrl], { env });
    assert.equal(validate.code, 0);
    assert.equal(JSON.parse(validate.stdout).data.validation.valid, true);

    const draft = await runCli(["posts", "draft", "--account", "sa_1", "--caption", "Launch update", "--json", "--base-url", baseUrl], { env });
    assert.equal(draft.code, 0);
    assert.equal(JSON.parse(draft.stdout).data.post.id, "post_draft");
    assert.deepEqual(bodies, [
      { url: "/v1/posts/validate", body: { caption: "Launch update", account_ids: ["sa_1"] } },
      { url: "/v1/posts", body: { caption: "Launch update", account_ids: ["sa_1"], status: "draft" } },
    ]);
  });
});

test("Phase 3 posts create --from-file --dry-run validates without publishing", async () => {
  await withTempConfig(async (configDir) => {
    const postPath = join(configDir, "post.json");
    await writeFile(postPath, JSON.stringify({
      account_ids: ["sa_1"],
      caption: "Dry run launch",
      media_ids: ["med_1"],
      scheduled_at: "2026-06-10T09:00:00Z",
      idempotency_key: "dry-run-key",
    }));

    const requests = [];
    await withServer(async (req, res) => {
      requests.push({ url: req.url, method: req.method, body: await readRequestJson(req) });
      if (req.method === "POST" && req.url === "/v1/posts/validate") {
        writeJson(res, 200, {
          data: { valid: true, errors: [], warnings: [], normalized: true },
          request_id: "req_dry_run",
        });
        return;
      }
      writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: "Not found" } });
    }, async (baseUrl) => {
      const result = await runCli(["posts", "create", "--from-file", postPath, "--dry-run", "--json", "--base-url", baseUrl], {
        env: { UNIPOST_API_KEY: "up_test_valid" },
      });

      assert.equal(result.code, 0);
      const body = JSON.parse(result.stdout);
      assert.equal(body.data.dry_run, true);
      assert.equal(body.data.validation.valid, true);
      assert.deepEqual(body.data.payload.account_ids, ["sa_1"]);
      assert.deepEqual(requests, [{
        url: "/v1/posts/validate",
        method: "POST",
        body: {
          account_ids: ["sa_1"],
          caption: "Dry run launch",
          media_ids: ["med_1"],
          scheduled_at: "2026-06-10T09:00:00Z",
          idempotency_key: "dry-run-key",
        },
      }]);
    });
  });
});

test("Phase 3 live post creation is blocked without confirmation and idempotency", async () => {
  await withServer(async (req, res) => {
    const body = await readRequestJson(req);
    assert.equal(req.method, "POST");
    assert.equal(req.url, "/v1/posts");
    assert.equal(req.headers["idempotency-key"], "idem_live");
    assert.equal(req.headers["x-unipost-cli-command"], "posts create");
    assert.equal(req.headers["x-unipost-agent-name"], "codex");
    assert.deepEqual(body, { caption: "Live launch", account_ids: ["sa_1"] });
    writeJson(res, 201, {
      data: { id: "post_live", caption: "Live launch", status: "publishing" },
      request_id: "req_live_create",
    });
  }, async (baseUrl) => {
    const env = { UNIPOST_API_KEY: "up_test_valid" };

    const missingYes = await runCli([
      "posts", "create", "--account", "sa_1", "--caption", "Live launch", "--non-interactive", "--json", "--base-url", baseUrl,
    ], { env });
    assert.equal(missingYes.code, 9);
    assert.equal(JSON.parse(missingYes.stdout).error.normalized_code, "unsafe_action_blocked");

    const missingIdempotency = await runCli([
      "posts", "create", "--account", "sa_1", "--caption", "Live launch", "--yes", "--non-interactive", "--json", "--base-url", baseUrl,
    ], { env });
    assert.equal(missingIdempotency.code, 3);
    assert.equal(JSON.parse(missingIdempotency.stdout).error.normalized_code, "missing_required_input");

    const ok = await runCli([
      "posts", "create", "--account", "sa_1", "--caption", "Live launch", "--yes", "--idempotency-key", "idem_live", "--agent-name", "codex", "--json", "--base-url", baseUrl,
    ], { env });
    assert.equal(ok.code, 0);
    assert.equal(JSON.parse(ok.stdout).data.post.id, "post_live");
  });
});

test("Phase 3 posts schedule maps to create with scheduled_at", async () => {
  await withServer(async (req, res) => {
    const body = await readRequestJson(req);
    assert.equal(req.method, "POST");
    assert.equal(req.url, "/v1/posts");
    assert.equal(req.headers["idempotency-key"], "idem_sched");
    assert.deepEqual(body, {
      caption: "Scheduled launch",
      account_ids: ["sa_1"],
      scheduled_at: "2026-06-10T09:00:00Z",
    });
    writeJson(res, 201, {
      data: {
        id: "post_scheduled",
        caption: "Scheduled launch",
        status: "scheduled",
        scheduled_at: "2026-06-10T09:00:00Z",
      },
      request_id: "req_schedule",
    });
  }, async (baseUrl) => {
    const result = await runCli([
      "posts", "schedule", "--account", "sa_1", "--caption", "Scheduled launch", "--at", "2026-06-10T09:00:00Z", "--yes", "--idempotency-key", "idem_sched", "--json", "--base-url", baseUrl,
    ], {
      env: { UNIPOST_API_KEY: "up_test_valid" },
    });

    assert.equal(result.code, 0);
    const body = JSON.parse(result.stdout);
    assert.equal(body.data.post.status, "scheduled");
    assert.equal(body.data.post.scheduled_at, "2026-06-10T09:00:00Z");
  });
});

test("Phase 3 posts wait cancel and retry use canonical lifecycle endpoints", async () => {
  let postReads = 0;
  await withServer((req, res) => {
    if (req.method === "GET" && req.url === "/v1/posts/post_1") {
      postReads += 1;
      writeJson(res, 200, {
        data: postReads === 1 ? {
          id: "post_1",
          status: "publishing",
          results: [{ id: "res_1", status: "pending" }],
        } : {
          id: "post_1",
          status: "partial",
          results: [{ id: "res_1", status: "failed" }],
        },
        request_id: "req_post_get",
      });
      return;
    }
    if (req.method === "POST" && req.url === "/v1/posts/post_1/cancel") {
      writeJson(res, 200, { data: { id: "post_1", status: "cancelled" }, request_id: "req_cancel" });
      return;
    }
    if (req.method === "POST" && req.url === "/v1/posts/post_1/results/res_1/retry") {
      writeJson(res, 202, { data: { id: "job_1", status: "queued" }, request_id: "req_retry" });
      return;
    }
    writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: "Not found" } });
  }, async (baseUrl) => {
    const env = { UNIPOST_API_KEY: "up_test_valid", UNIPOST_TEST_POLL_MS: "1" };

    const wait = await runCli(["posts", "wait", "post_1", "--timeout", "2", "--json", "--base-url", baseUrl], { env });
    assert.equal(wait.code, 0);
    const waitBody = JSON.parse(wait.stdout);
    assert.equal(waitBody.data.post.status, "partial");
    assert.equal(waitBody.data.attempts, 2);

    const cancelBlocked = await runCli(["posts", "cancel", "post_1", "--json", "--base-url", baseUrl], { env });
    assert.equal(cancelBlocked.code, 9);

    const cancel = await runCli(["posts", "cancel", "post_1", "--yes", "--json", "--base-url", baseUrl], { env });
    assert.equal(cancel.code, 0);
    assert.equal(JSON.parse(cancel.stdout).data.post.status, "canceled");

    const retry = await runCli(["posts", "retry", "post_1", "--result", "res_1", "--yes", "--json", "--base-url", baseUrl], { env });
    assert.equal(retry.code, 0);
    assert.equal(JSON.parse(retry.stdout).data.retry.status, "queued");
  });
});

test("Phase 3 read commands expose stable JSON for posts media and analytics", async () => {
  await withServer((req, res) => {
    if (req.method === "GET" && req.url === "/v1/posts?status=failed&limit=2&cursor=cur_1") {
      writeJson(res, 200, {
        data: [{ id: "post_failed", status: "failed" }],
        meta: { total: 1, limit: 2, next_cursor: "cur_2" },
        request_id: "req_posts_list",
      });
      return;
    }
    if (req.method === "GET" && req.url === "/v1/posts/post_failed") {
      writeJson(res, 200, { data: { id: "post_failed", status: "failed" }, request_id: "req_post_get" });
      return;
    }
    if (req.method === "GET" && req.url === "/v1/posts/post_failed/analytics") {
      writeJson(res, 200, { data: { post_id: "post_failed", impressions: 42 }, request_id: "req_post_analytics" });
      return;
    }
    if (req.method === "GET" && req.url === "/v1/media/med_1") {
      writeJson(res, 200, { data: { id: "med_1", status: "ready" }, request_id: "req_media_get" });
      return;
    }
    if (req.method === "GET" && req.url === "/v1/analytics/summary?from=2026-06-01&to=2026-06-30") {
      writeJson(res, 200, { data: { posts: { total: 3 } }, request_id: "req_analytics_summary" });
      return;
    }
    if (req.method === "GET" && req.url === "/v1/analytics/platforms") {
      writeJson(res, 200, { data: [{ platform: "linkedin", posts: 2 }], request_id: "req_analytics_platforms" });
      return;
    }
    if (req.method === "GET" && req.url === "/v1/analytics/platforms/linkedin?from=2026-06-01") {
      writeJson(res, 200, { data: { platform: "linkedin", posts: 2 }, request_id: "req_analytics_platform" });
      return;
    }
    writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: `Not found: ${req.url}` } });
  }, async (baseUrl) => {
    const env = { UNIPOST_API_KEY: "up_test_valid" };

    const list = await runCli(["posts", "list", "--status", "failed", "--limit", "2", "--cursor", "cur_1", "--json", "--base-url", baseUrl], { env });
    assert.equal(list.code, 0);
    assert.equal(JSON.parse(list.stdout).data.posts[0].status, "failed");
    assert.equal(JSON.parse(list.stdout).meta.pagination.next_cursor, "cur_2");

    const get = await runCli(["posts", "get", "post_failed", "--json", "--base-url", baseUrl], { env });
    assert.equal(get.code, 0);
    assert.equal(JSON.parse(get.stdout).data.post.id, "post_failed");

    const postAnalytics = await runCli(["posts", "analytics", "post_failed", "--json", "--base-url", baseUrl], { env });
    assert.equal(postAnalytics.code, 0);
    assert.equal(JSON.parse(postAnalytics.stdout).data.analytics.impressions, 42);

    const media = await runCli(["media", "get", "med_1", "--json", "--base-url", baseUrl], { env });
    assert.equal(media.code, 0);
    assert.equal(JSON.parse(media.stdout).data.media.status, "ready");

    const summary = await runCli(["analytics", "summary", "--from", "2026-06-01", "--to", "2026-06-30", "--json", "--base-url", baseUrl], { env });
    assert.equal(summary.code, 0);
    assert.equal(JSON.parse(summary.stdout).data.summary.posts.total, 3);

    const platforms = await runCli(["analytics", "platforms", "--json", "--base-url", baseUrl], { env });
    assert.equal(platforms.code, 0);
    assert.equal(JSON.parse(platforms.stdout).data.platforms[0].platform, "linkedin");

    const platform = await runCli(["analytics", "platform", "linkedin", "--from", "2026-06-01", "--json", "--base-url", baseUrl], { env });
    assert.equal(platform.code, 0);
    assert.equal(JSON.parse(platform.stdout).data.platform.platform, "linkedin");
  });
});

test("Phase 4 accounts diagnostics read health capabilities and metrics endpoints", async () => {
  await withServer((req, res) => {
    if (req.method === "GET" && req.url === "/v1/accounts/sa_1/health") {
      writeJson(res, 200, {
        data: {
          social_account_id: "sa_1",
          platform: "linkedin",
          status: "degraded",
          last_error: { code: "rate_limited", message: "Too many requests" },
        },
        request_id: "req_health",
      });
      return;
    }
    if (req.method === "GET" && req.url === "/v1/accounts/sa_1/capabilities") {
      writeJson(res, 200, {
        data: {
          account_id: "sa_1",
          platform: "linkedin",
          capability: { text: { max_length: 3000 }, media: { images: { max_count: 9 } } },
        },
        request_id: "req_capabilities",
      });
      return;
    }
    if (req.method === "GET" && req.url === "/v1/accounts/sa_1/metrics") {
      writeJson(res, 200, {
        data: {
          social_account_id: "sa_1",
          platform: "linkedin",
          follower_count: 123,
          following_count: 45,
          post_count: 6,
        },
        request_id: "req_metrics",
      });
      return;
    }
    writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: `Not found: ${req.url}` } });
  }, async (baseUrl) => {
    const env = { UNIPOST_API_KEY: "up_test_valid" };

    const health = await runCli(["accounts", "health", "--account", "sa_1", "--json", "--base-url", baseUrl], { env });
    assert.equal(health.code, 0);
    const healthBody = JSON.parse(health.stdout);
    assert.equal(healthBody.data.health.status, "degraded");
    assert.equal(healthBody.data.health.last_error.code, "rate_limited");

    const capabilities = await runCli(["accounts", "capabilities", "--account", "sa_1", "--json", "--base-url", baseUrl], { env });
    assert.equal(capabilities.code, 0);
    const capabilitiesBody = JSON.parse(capabilities.stdout);
    assert.equal(capabilitiesBody.data.capabilities.platform, "linkedin");
    assert.equal(capabilitiesBody.data.capabilities.capability.media.images.max_count, 9);

    const metrics = await runCli(["accounts", "metrics", "--account", "sa_1", "--json", "--base-url", baseUrl], { env });
    assert.equal(metrics.code, 0);
    const metricsBody = JSON.parse(metrics.stdout);
    assert.equal(metricsBody.data.metrics.follower_count, 123);
  });
});

test("Phase 4 media upload reserves uploads and waits for canonical readiness", async () => {
  await withTempConfig(async (configDir) => {
    const mediaPath = join(configDir, "clip.mp4");
    await writeFile(mediaPath, "media bytes");
    let mediaReads = 0;

    await withServer(async (req, res) => {
      if (req.method === "POST" && req.url === "/v1/media") {
        const body = await readRequestJson(req);
        assert.equal(body.filename, "clip.mp4");
        assert.equal(body.content_type, "video/mp4");
        assert.equal(body.size_bytes, 11);
        assert.match(body.content_hash, /^[a-f0-9]{64}$/);
        writeJson(res, 201, {
          data: {
            id: "med_1",
            status: "pending",
            content_type: "video/mp4",
            size_bytes: 11,
            upload_url: "http://unipost-cli-test.local/uploads/med_1",
          },
          request_id: "req_media_create",
        });
        return;
      }
      if (req.method === "PUT" && req.url === "/uploads/med_1") {
        assert.equal(req.headers["content-type"], "video/mp4");
        res.writeHead(200);
        res.end("");
        return;
      }
      if (req.method === "GET" && req.url === "/v1/media/med_1") {
        mediaReads += 1;
        writeJson(res, 200, {
          data: {
            id: "med_1",
            status: mediaReads === 1 ? "pending" : "uploaded",
            content_type: "video/mp4",
            size_bytes: 11,
          },
          request_id: "req_media_get",
        });
        return;
      }
      writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: `Not found: ${req.url}` } });
    }, async (baseUrl) => {
      const env = { UNIPOST_API_KEY: "up_test_valid", UNIPOST_TEST_POLL_MS: "1" };

      const upload = await runCli(["media", "upload", mediaPath, "--json", "--base-url", baseUrl], { env });
      assert.equal(upload.code, 0);
      const uploadBody = JSON.parse(upload.stdout);
      assert.equal(uploadBody.data.media.id, "med_1");
      assert.equal(uploadBody.data.media.status, "ready");
      assert.equal(uploadBody.data.ready, true);
      assert.equal(uploadBody.data.media.content_type, "video/mp4");

      mediaReads = 0;
      const wait = await runCli(["media", "wait", "med_1", "--timeout", "2", "--json", "--base-url", baseUrl], { env });
      assert.equal(wait.code, 0);
      const waitBody = JSON.parse(wait.stdout);
      assert.equal(waitBody.data.media.status, "ready");
      assert.equal(waitBody.data.ready, true);
      assert.equal(waitBody.data.attempts, 2);
    });
  });
});

test("Phase 4 agent capabilities include advanced diagnostics and media operations", async () => {
  const capabilities = await runCli(["agent", "capabilities", "--json"]);
  assert.equal(capabilities.code, 0);
  const body = JSON.parse(capabilities.stdout);
  assert.equal(body.data.catalog_version, "2026-06-03.phase5");
  assert.ok(body.data.commands.includes("accounts health"));
  assert.ok(body.data.commands.includes("accounts capabilities"));
  assert.ok(body.data.commands.includes("accounts metrics"));
  assert.ok(body.data.commands.includes("media upload"));
  assert.ok(body.data.commands.includes("media wait"));
  assert.ok(body.data.intents.some((intent) => intent.name === "diagnose_account"));
  assert.ok(body.data.intents.some((intent) => intent.name === "upload_media"));
});

test("Phase 4 agent plan supports account diagnostics and media upload intents", async () => {
  await withTempConfig(async (configDir) => {
    const uploadPlanPath = join(configDir, "upload-plan.json");
    await writeFile(uploadPlanPath, JSON.stringify({
      file_path: "/tmp/clip.mp4",
      content_type: "video/mp4",
    }));

    const diagnose = await runCli(["agent", "plan", "--intent", "diagnose_account", "--account", "sa_1", "--json"]);
    assert.equal(diagnose.code, 0);
    const diagnoseBody = JSON.parse(diagnose.stdout);
    assert.equal(diagnoseBody.data.intent, "diagnose_account");
    assert.equal(diagnoseBody.data.safety_level, "read_only");
    assert.deepEqual(diagnoseBody.data.missing_inputs, []);
    assert.deepEqual(diagnoseBody.data.actions.map((action) => action.canonical_action), [
      "accounts.health",
      "accounts.capabilities",
      "accounts.metrics",
    ]);

    const upload = await runCli(["agent", "plan", "--intent", "upload_media", "--from-file", uploadPlanPath, "--json"]);
    assert.equal(upload.code, 0);
    const uploadBody = JSON.parse(upload.stdout);
    assert.equal(uploadBody.data.intent, "upload_media");
    assert.equal(uploadBody.data.safety_level, "setup_write");
    assert.equal(uploadBody.data.safe_to_execute_without_user, false);
    assert.deepEqual(uploadBody.data.required_user_confirmations, ["approve_local_file_upload"]);
    assert.deepEqual(uploadBody.data.actions.map((action) => action.canonical_action), ["media.upload", "media.wait"]);
    assert.equal(uploadBody.data.actions[0].args.file_path, "/tmp/clip.mp4");
    assert.equal(uploadBody.data.actions[0].args["--content-type"], "video/mp4");
  });
});

test("Phase 5 examples and client configs expose MCP ecosystem setup", async () => {
  const example = await runCli(["examples", "mcp.claude-code", "--json"]);
  assert.equal(example.code, 0);
  const exampleBody = JSON.parse(example.stdout);
  assert.equal(exampleBody.data.example, "mcp.claude-code");
  assert.match(exampleBody.data.content, /claude mcp add unipost/);
  assert.match(exampleBody.data.content, /mcp\.unipost\.dev\/mcp/);
  assert.match(exampleBody.data.content, /agent mcp-test/);

  const cursorConfig = await runCli(["agent", "mcp-config", "cursor", "--json"]);
  assert.equal(cursorConfig.code, 0);
  const cursorBody = JSON.parse(cursorConfig.stdout);
  assert.equal(cursorBody.data.client, "cursor");
  assert.equal(cursorBody.data.transport, "streamable_http");
  assert.equal(cursorBody.data.config.mcpServers.unipost.url, "https://mcp.unipost.dev/mcp");
  assert.equal(cursorBody.data.config.mcpServers.unipost.headers.Authorization, "Bearer ${UNIPOST_API_KEY}");

  const claudeConfig = await runCli(["agent", "mcp-config", "claude-code", "--json"]);
  assert.equal(claudeConfig.code, 0);
  assert.match(JSON.parse(claudeConfig.stdout).data.content, /claude mcp add unipost/);

  const install = await runCli(["agent", "install", "--client", "codex", "--json"]);
  assert.equal(install.code, 0);
  const installBody = JSON.parse(install.stdout);
  assert.equal(installBody.data.client, "codex");
  assert.equal(installBody.data.mode, "instructions");
  assert.ok(installBody.data.files.some((file) => file.path.endsWith("agent-packages/codex/SKILL.md")));
  assert.match(installBody.data.instructions, /agent capabilities/);
});

test("Phase 5 mcp-test validates auth and reports shared catalog readiness", async () => {
  await withServer((req, res) => {
    if (req.method === "GET" && req.url === "/v1/workspace") {
      writeJson(res, 200, { data: { id: "ws_mcp", name: "MCP Workspace" }, request_id: "req_mcp_test" });
      return;
    }
    writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: "Not found" } });
  }, async (baseUrl) => {
    const result = await runCli(["agent", "mcp-test", "--json", "--base-url", baseUrl], {
      env: { UNIPOST_API_KEY: "up_test_valid" },
    });

    assert.equal(result.code, 0);
    const body = JSON.parse(result.stdout);
    assert.equal(body.data.authenticated, true);
    assert.equal(body.data.workspace.id, "ws_mcp");
    assert.equal(body.data.mcp.endpoint, "https://mcp.unipost.dev/mcp");
    assert.equal(body.data.catalog_version, "2026-06-03.phase5");
  });
});

test("Phase 5 agent execute runs only structured safe actions and rejects live writes", async () => {
  await withTempConfig(async (configDir) => {
    const safePlanPath = join(configDir, "safe-plan.json");
    await writeFile(safePlanPath, JSON.stringify({
      ok: true,
      data: {
        intent: "create_draft_post",
        actions: [
          {
            canonical_action: "posts.validate",
            args: { "--account": "sa_1", "--caption": "Safe draft", "--json": true },
          },
          {
            canonical_action: "posts.draft",
            args: { "--account": "sa_1", "--caption": "Safe draft", "--json": true },
          },
        ],
      },
    }));
    const livePlanPath = join(configDir, "live-plan.json");
    await writeFile(livePlanPath, JSON.stringify({
      ok: true,
      data: {
        intent: "plan_publish_post",
        actions: [
          {
            canonical_action: "posts.validate",
            args: { "--account": "sa_1", "--caption": "Publish me", "--json": true },
          },
          {
            canonical_action: "posts.create",
            safety_level: "read_only",
            display_command: "echo unsafe",
            args: { "--account": "sa_1", "--caption": "Publish me", "--json": true },
          },
        ],
      },
    }));

    const bodies = [];
    await withServer(async (req, res) => {
      if (req.method === "POST" && req.url === "/v1/posts/validate") {
        bodies.push({ url: req.url, body: await readRequestJson(req) });
        writeJson(res, 200, { data: { valid: true, warnings: [] }, request_id: "req_validate" });
        return;
      }
      if (req.method === "POST" && req.url === "/v1/posts") {
        bodies.push({ url: req.url, body: await readRequestJson(req) });
        writeJson(res, 201, { data: { id: "post_draft", status: "draft" }, request_id: "req_draft" });
        return;
      }
      writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: `Not found: ${req.url}` } });
    }, async (baseUrl) => {
      const env = { UNIPOST_API_KEY: "up_test_valid" };
      const executed = await runCli(["agent", "execute", "--plan", safePlanPath, "--json", "--base-url", baseUrl], { env });
      assert.equal(executed.code, 0);
      const executedBody = JSON.parse(executed.stdout);
      assert.deepEqual(executedBody.data.executed_actions, ["posts.validate", "posts.draft"]);
      assert.equal(executedBody.data.results[1].data.post.id, "post_draft");
      assert.deepEqual(bodies.map((entry) => entry.url), ["/v1/posts/validate", "/v1/posts"]);
      assert.equal(bodies[1].body.status, "draft");
      bodies.length = 0;

      const rejected = await runCli(["agent", "execute", "--plan", livePlanPath, "--json", "--base-url", baseUrl], { env });
      assert.equal(rejected.code, 9);
      const rejectedBody = JSON.parse(rejected.stdout);
      assert.equal(rejectedBody.ok, false);
      assert.equal(rejectedBody.error.code, "requires_explicit_publish_command");
      assert.deepEqual(bodies, []);
    });
  });
});

test("Phase 3 agent plan returns executable steps and required confirmations", async () => {
  await withTempConfig(async (configDir) => {
    const postPath = join(configDir, "plan-post.json");
    await writeFile(postPath, JSON.stringify({
      account_ids: ["sa_1"],
      caption: "Plan launch",
      scheduled_at: "2026-06-10T09:00:00Z",
    }));

    const plan = await runCli(["agent", "plan", "--intent", "plan_publish_post", "--from-file", postPath, "--json"]);
    assert.equal(plan.code, 0);
    const body = JSON.parse(plan.stdout);
    assert.equal(body.data.intent, "plan_publish_post");
    assert.equal(body.data.safe_to_execute_without_user, false);
    assert.deepEqual(body.data.missing_inputs, []);
    assert.deepEqual(body.data.required_user_confirmations, ["approve_live_publish"]);
    assert.deepEqual(body.data.actions.map((action) => action.canonical_action), ["posts.validate", "posts.create_dry_run", "posts.create"]);
    assert.equal(body.data.actions[1].args["--dry-run"], true);
    assert.equal(body.data.actions[1].args["--from-file"], postPath);
    assert.equal(body.data.actions[2].args["--schedule-at"], "2026-06-10T09:00:00Z");

    const alias = await runCli(["agent", "plan-publish", "--from-file", postPath, "--json"]);
    assert.equal(alias.code, 0);
    assert.equal(JSON.parse(alias.stdout).data.intent, "plan_publish_post");

    const missing = await runCli(["agent", "plan", "--intent", "create_draft_post", "--caption", "Only caption", "--json"]);
    assert.equal(missing.code, 0);
    assert.deepEqual(JSON.parse(missing.stdout).data.missing_inputs, ["account_ids"]);
  });
});

test("agent capabilities and context expose stable machine-readable discovery", async () => {
  const capabilities = await runCli(["agent", "capabilities", "--json"]);
  assert.equal(capabilities.code, 0);
  const capabilitiesBody = JSON.parse(capabilities.stdout);
  assert.equal(capabilitiesBody.data.catalog_version, "2026-06-03.phase5");
  const draftIntent = capabilitiesBody.data.intents.find((intent) => intent.name === "create_draft_post");
  assert.equal(draftIntent.safety_level, "draft_write");
  assert.equal(draftIntent.canonical_action, "posts.draft");
  assert.deepEqual(draftIntent.input_schema.required, ["account_ids", "caption"]);

  await withServer((req, res) => {
    if (req.url === "/v1/workspace") {
      writeJson(res, 200, { data: { id: "ws_agent", name: "Agent Workspace" }, request_id: "req_ws" });
      return;
    }
    if (req.url === "/v1/profiles") {
      writeJson(res, 200, { data: [{ id: "pr_agent", name: "Agent", account_count: 1 }], request_id: "req_profiles" });
      return;
    }
    if (req.url === "/v1/accounts") {
      writeJson(res, 200, { data: [{ id: "sa_agent", platform: "linkedin", account_name: "Agent", status: "active" }], request_id: "req_accounts" });
      return;
    }
    writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: "Not found" } });
  }, async (baseUrl) => {
    const context = await runCli(["agent", "context", "--json", "--base-url", baseUrl], {
      env: { UNIPOST_API_KEY: "up_test_valid" },
    });
    assert.equal(context.code, 0);
    const body = JSON.parse(context.stdout);
    assert.equal(body.data.workspace.id, "ws_agent");
    assert.equal(body.data.profiles[0].id, "pr_agent");
    assert.equal(body.data.accounts[0].id, "sa_agent");
    assert.equal(body.data.grounding.account_count, 1);
  });
});

test("agent guide and mcp-config return client-specific setup content", async () => {
  const guide = await runCli(["agent", "guide", "--client", "codex", "--json"]);
  assert.equal(guide.code, 0);
  const guideBody = JSON.parse(guide.stdout);
  assert.equal(guideBody.data.client, "codex");
  assert.match(guideBody.data.recommended_prompt, /posts validate/);

  const claudeConfig = await runCli(["agent", "mcp-config", "claude-code", "--json"]);
  assert.equal(claudeConfig.code, 0);
  const claudeBody = JSON.parse(claudeConfig.stdout);
  assert.equal(claudeBody.data.endpoint, "https://mcp.unipost.dev/mcp");
  assert.match(claudeBody.data.content, /claude mcp add unipost/);

  const codexConfig = await runCli(["agent", "mcp-config", "codex"]);
  assert.equal(codexConfig.code, 0);
  assert.match(codexConfig.stdout, /\[mcp_servers\.unipost\]/);
  assert.match(codexConfig.stdout, /UNIPOST_API_KEY/);
});

test("agent bootstrap diagnoses missing auth and succeeds with API-key fallback", async () => {
  const missing = await runCli(["agent", "bootstrap", "--client", "codex", "--json"]);
  assert.equal(missing.code, 4);
  const missingBody = JSON.parse(missing.stdout);
  assert.equal(missingBody.ok, true);
  assert.equal(missingBody.data.authenticated, false);
  assert.match(missingBody.data.next_actions[0], /UNIPOST_API_KEY/);

  await withServer((req, res) => {
    if (req.url === "/v1/workspace") {
      writeJson(res, 200, { data: { id: "ws_boot", name: "Boot Workspace" } });
      return;
    }
    if (req.url === "/v1/profiles") {
      writeJson(res, 200, { data: [{ id: "pr_boot", name: "Boot", account_count: 1 }] });
      return;
    }
    if (req.url === "/v1/accounts") {
      writeJson(res, 200, { data: [{ id: "sa_boot", platform: "linkedin", account_name: "Boot", status: "active" }] });
      return;
    }
    writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: "Not found" } });
  }, async (baseUrl) => {
    const ok = await runCli(["agent", "bootstrap", "--client", "codex", "--json", "--base-url", baseUrl], {
      env: { UNIPOST_API_KEY: "up_test_valid" },
    });
    assert.equal(ok.code, 0);
    const body = JSON.parse(ok.stdout);
    assert.equal(body.data.authenticated, true);
    assert.equal(body.data.ready_for_draft, true);
    assert.equal(body.data.client, "codex");
  });
});

test("examples posts.create generates dependency-free cURL and native fetch snippets", async () => {
  const curl = await runCli(["examples", "posts.create", "--lang", "curl", "--account", "sa_1", "--caption", "Hello", "--json"]);
  assert.equal(curl.code, 0);
  assert.match(JSON.parse(curl.stdout).data.code, /curl/);
  assert.match(JSON.parse(curl.stdout).data.code, /"account_ids":\["sa_1"\]/);

  const node = await runCli(["examples", "posts.create", "--lang", "node", "--account", "sa_1", "--caption", "Hello", "--json"]);
  assert.equal(node.code, 0);
  const body = JSON.parse(node.stdout);
  assert.match(body.data.code, /fetch/);
  assert.match(body.data.code, /process.env.UNIPOST_API_KEY/);
});

test("init and quickstart summarize the first-run state without creating a live post", async () => {
  await withTempConfig(async (configDir) => {
    const bodies = [];
    await withServer(async (req, res) => {
      if (req.method === "GET" && req.url === "/v1/workspace") {
        writeJson(res, 200, { data: { id: "ws_init", name: "Init Workspace" }, request_id: "req_workspace" });
        return;
      }
      if (req.method === "GET" && req.url === "/v1/profiles") {
        writeJson(res, 200, { data: [], meta: { total: 0, limit: 0 }, request_id: "req_profiles" });
        return;
      }
      if (req.method === "POST" && req.url === "/v1/profiles") {
        bodies.push(await readRequestJson(req));
        writeJson(res, 201, { data: { id: "pr_new", name: "New Brand", account_count: 0 }, request_id: "req_profile_create" });
        return;
      }
      if (req.method === "GET" && req.url === "/v1/accounts") {
        writeJson(res, 200, { data: [], meta: { total: 0, limit: 0 }, request_id: "req_accounts" });
        return;
      }
      writeJson(res, 404, { error: { code: "NOT_FOUND", normalized_code: "not_found", message: "Not found" } });
    }, async (baseUrl) => {
      const env = {
        UNIPOST_API_KEY: "up_test_valid",
        UNIPOST_CONFIG_DIR: configDir,
      };
      const init = await runCli(["init", "--json", "--base-url", baseUrl], { env });
      assert.equal(init.code, 0);
      assert.equal(JSON.parse(init.stdout).data.workspace.id, "ws_init");
      assert.equal(JSON.parse(init.stdout).data.profiles.length, 0);

      const quickstart = await runCli(["quickstart", "--name", "New Brand", "--json", "--base-url", baseUrl], { env });
      assert.equal(quickstart.code, 0);
      const body = JSON.parse(quickstart.stdout);
      assert.equal(body.data.profile.id, "pr_new");
      assert.equal(body.data.accounts.length, 0);
      assert.equal(body.data.live_publish_created, false);
      assert.deepEqual(bodies, [{ name: "New Brand" }]);
    });
  });
});
