import assert from "node:assert/strict";
import http from "node:http";
import { test } from "node:test";
import { apiRequest, canonicalizeApiPath, UniPostApiError } from "../dist/api-client.js";

async function withServer(handler, run) {
  const server = http.createServer(handler);
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  const baseUrl = `http://127.0.0.1:${address.port}`;
  try {
    await run(baseUrl);
  } finally {
    await new Promise((resolve, reject) => {
      server.close((err) => (err ? reject(err) : resolve()));
    });
  }
}

test("canonicalizeApiPath maps legacy social-posts routes to canonical posts routes", () => {
  assert.equal(canonicalizeApiPath("/v1/social-posts"), "/v1/posts");
  assert.equal(canonicalizeApiPath("/v1/social-posts/validate"), "/v1/posts/validate");
  assert.equal(canonicalizeApiPath("/v1/social-posts/post_123/analytics"), "/v1/posts/post_123/analytics");
  assert.equal(canonicalizeApiPath("/v1/social-accounts"), "/v1/social-accounts");
});

test("apiRequest throws structured UniPostApiError for non-2xx API responses", async () => {
  await withServer(
    (_req, res) => {
      res.writeHead(422, { "Content-Type": "application/json", "X-Request-Id": "req_header" });
      res.end(
        JSON.stringify({
          error: {
            code: "VALIDATION_ERROR",
            normalized_code: "validation_error",
            message: "request failed pre-publish validation",
            issues: [{ field: "caption", code: "exceeds_max_length", message: "Caption is too long." }],
          },
          request_id: "req_body",
        })
      );
    },
    async (baseUrl) => {
      try {
        await apiRequest(baseUrl, "/v1/posts", "up_test", { method: "POST", body: "{}" });
        assert.fail("expected apiRequest to throw");
      } catch (err) {
        assert.ok(err instanceof UniPostApiError);
        assert.equal(err.status, 422);
        assert.equal(err.code, "VALIDATION_ERROR");
        assert.equal(err.normalizedCode, "validation_error");
        assert.equal(err.requestId, "req_body");
        assert.deepEqual(err.issues, [
          { field: "caption", code: "exceeds_max_length", message: "Caption is too long." },
        ]);
        assert.match(err.message, /request failed pre-publish validation/);
        assert.match(err.message, /normalized_code=validation_error/);
        assert.match(err.message, /request_id=req_body/);
        assert.match(err.message, /caption: Caption is too long\./);
      }
    }
  );
});

test("apiRequest sends canonical route and bearer token", async () => {
  let seenUrl = "";
  let seenAuth = "";
  await withServer(
    (req, res) => {
      seenUrl = req.url || "";
      seenAuth = req.headers.authorization || "";
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ data: { ok: true } }));
    },
    async (baseUrl) => {
      const body = await apiRequest(baseUrl, "/v1/social-posts/validate", "up_test", {
        method: "POST",
        body: "{}",
      });
      assert.deepEqual(body, { data: { ok: true } });
    }
  );
  assert.equal(seenUrl, "/v1/posts/validate");
  assert.equal(seenAuth, "Bearer up_test");
});
