import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { integrationLogMatchesSearch } from "../src/lib/log-search.ts";

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
