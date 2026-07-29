import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  integrationLogMatchesSearch,
  logDetailUnavailableMessage,
} from "../src/lib/log-search.ts";

const log = {
  message: "Hosted Connect failed during authorization.",
  action: "account.connect.callback_failed",
  metadata: { connect_session_id: "cs_exact", external_user_id: "customer_42" },
};

test("matches complete Hosted Connect metadata IDs", () => {
  assert.equal(integrationLogMatchesSearch(log, "cs_exact"), true);
  assert.equal(integrationLogMatchesSearch(log, "customer_42"), true);
  assert.equal(integrationLogMatchesSearch(log, "CS_EXACT"), false);
  assert.equal(integrationLogMatchesSearch(log, "cs_ex"), false);
  assert.equal(integrationLogMatchesSearch(log, "customer"), false);
});

test("preserves substring search for existing text fields", () => {
  assert.equal(integrationLogMatchesSearch(log, "callback_fail"), true);
  assert.equal(integrationLogMatchesSearch(log, "authorization"), true);
  assert.equal(integrationLogMatchesSearch(log, "missing"), false);
});

test("accepts canonical request log metadata omissions", () => {
  const canonicalLog = {
    message: "GET /v1/posts returned 500.",
    action: "api.request.failed",
    request_id: "req_canonical_01",
    metadata: undefined,
  };

  assert.equal(integrationLogMatchesSearch(canonicalLog, "api.request"), true);
  assert.equal(integrationLogMatchesSearch(canonicalLog, "REQ_CANONICAL"), true);
  assert.equal(integrationLogMatchesSearch(canonicalLog, "not-present"), false);
});

test("explains canonical request detail absence without changing legacy details", () => {
  assert.equal(
    logDetailUnavailableMessage({
      id: "request:00000000000000000001_example",
      category: "api_request",
      status: "error",
      detail_status: "expired",
    }),
    "Detail payload expired. The structured log metadata remains available.",
  );
  assert.equal(
    logDetailUnavailableMessage({
      id: "opaque-value-that-must-not-be-parsed",
      category: "api_request",
      status: "success",
    }),
    "Detailed payloads are retained only for failed API requests.",
  );
  assert.equal(logDetailUnavailableMessage({ id: 101, category: "api_request", status: "success" }), null);
  assert.equal(logDetailUnavailableMessage({ id: "integration:101", category: "oauth", status: "success" }), null);
});

test("Developer Logs documentation describes Hosted Connect outcomes", () => {
  const corpus = [
    readFileSync(new URL("../src/app/docs/api/logs/page.tsx", import.meta.url), "utf8"),
    readFileSync(new URL("../src/app/docs/api/logs/list/page.tsx", import.meta.url), "utf8"),
  ].join("\n");

  for (const expected of [
    "account.connect.callback_succeeded",
    "account.connect.callback_failed",
    "account.connect.callback_cancelled",
    "facebook_page_not_available",
    "facebook_page_permission_required",
    "facebook_authorization_failed",
    "connect_session_id",
    "external_user_id",
  ]) {
    assert.match(corpus, new RegExp(expected));
  }
});
