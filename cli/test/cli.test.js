import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import http from "node:http";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";

const __dirname = dirname(fileURLToPath(import.meta.url));
const cliBin = join(__dirname, "..", "bin", "unipost.js");

function runCli(args, options = {}) {
  const env = {
    ...process.env,
    UNIPOST_API_KEY: "",
    UNIPOST_BASE_URL: "",
    NO_COLOR: "1",
    ...options.env,
  };

  return new Promise((resolve) => {
    const child = spawn(process.execPath, [cliBin, ...args], { env });
    let stdout = "";
    let stderr = "";

    child.stdout.on("data", (chunk) => {
      stdout += chunk;
    });
    child.stderr.on("data", (chunk) => {
      stderr += chunk;
    });
    child.on("close", (code) => {
      resolve({ code, stdout, stderr });
    });
  });
}

async function withServer(handler, callback) {
  const requests = [];
  const server = http.createServer(async (req, res) => {
    requests.push(req);
    await handler(req, res);
  });

  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const { port } = server.address();
  try {
    return await callback(`http://127.0.0.1:${port}`, requests);
  } finally {
    await new Promise((resolve) => server.close(resolve));
  }
}

function writeJson(res, status, body, headers = {}) {
  res.writeHead(status, {
    "Content-Type": "application/json",
    "X-Request-Id": "req_test_123",
    ...headers,
  });
  res.end(JSON.stringify(body));
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
  assert.match(result.stdout, /--client/);
  assert.match(result.stdout, /--limit/);
  assert.match(result.stdout, /--cursor/);
  assert.match(result.stdout, /--all/);
  assert.equal(result.stderr, "");
});
