import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const read = (path) => readFileSync(new URL(`../${path}`, import.meta.url), "utf8");

test("admin feature flags page is super-admin-only and exposes both rollout switches", () => {
  const nav = read("src/app/admin/_components/admin-ui.tsx");
  const page = read("src/app/admin/feature-flags/page.tsx");

  assert.match(nav, /Feature Flags/);
  assert.match(nav, /\/admin\/feature-flags/);
  assert.ok(nav.indexOf("Object Storage") < nav.indexOf("Feature Flags"));
  assert.match(page, /requireSuperAdmin/);
  assert.match(page, /x_dms_v1/);
  assert.match(page, /x_credits_billing_v1/);
  assert.match(page, /available to regular users/i);
  assert.match(page, /Super Admin-owned workspaces/i);
  assert.match(page, /window\.confirm/);
});

test("feature flag clients use the backend authority surfaces", () => {
  const api = read("src/lib/api.ts");

  assert.match(api, /\/v1\/admin\/feature-flags/);
  assert.match(api, /\/v1\/me\/features/);
  assert.match(api, /\/v1\/public\/features/);
});

test("X Credits and X DMs customer UI are gated by evaluated flags", () => {
  const billing = read("src/app/(dashboard)/settings/billing/page.tsx");
  const inbox = read("src/app/(dashboard)/projects/[id]/inbox/page.tsx");
  const pricing = read("src/app/pricing/pricing-page-client.tsx");

  assert.match(billing, /x_credits_billing_v1/);
  assert.match(billing, /xCreditsEnabled/);
  assert.match(inbox, /x_dms_v1/);
  assert.match(inbox, /xDMsEnabled/);
  assert.match(inbox, /include_dms:\s*xDMsEnabled/);
  assert.match(pricing, /x_credits_billing_v1/);
  assert.match(pricing, /xCreditsEnabled/);
});
