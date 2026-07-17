import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("pricing keeps Team support and Enterprise support distinct", async () => {
  const pricing = await source("src/app/pricing/pricing-page-client.tsx");

  assert.match(pricing, /id: "team"[\s\S]*?text: "Priority support"/);
  assert.match(pricing, /When do I need Enterprise instead of Team\?[\s\S]*?need dedicated support/);
  assert.match(pricing, /className="pr-ent-desc">Dedicated support, capacity planning/);
  assert.doesNotMatch(pricing, /Enterprise when you need priority support/);
});

test("pricing advertises the Team-only Audit Log contract", async () => {
  const pricing = await source("src/app/pricing/pricing-page-client.tsx");

  assert.match(pricing, /id: "team"[\s\S]*?text: "Audit log"/);
  assert.match(
    pricing,
    /name: "Audit log"[\s\S]*?free: false[\s\S]*?api: false[\s\S]*?basic: false[\s\S]*?growth: false[\s\S]*?team: true/,
  );
});

test("Team member mutations and Audit Log stay protected at the router", async () => {
  const routes = await source("../api/cmd/api/main.go");

  for (const protectedRoute of [
    'r.With(auth.RequireRole(auth.RoleAdmin)).Post("/v1/members/invite"',
    'r.With(auth.RequireRole(auth.RoleAdmin)).Delete("/v1/members/invites/{id}"',
    'r.With(auth.RequireRole(auth.RoleAdmin)).Patch("/v1/members/{userID}/role"',
    'r.With(auth.RequireRole(auth.RoleAdmin)).Delete("/v1/members/{userID}"',
    'r.With(auth.RequireRole(auth.RoleOwner)).Post("/v1/members/{userID}/transfer-ownership"',
    'r.With(handler.RequirePlanAuditLog(quotaChecker)).Get("/v1/audit-log"',
  ]) {
    assert.ok(routes.includes(protectedRoute), `missing route protection: ${protectedRoute}`);
  }
});
