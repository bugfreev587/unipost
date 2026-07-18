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
  assert.doesNotMatch(page, /window\.confirm/);
  assert.match(page, /DialogContent/);
  assert.match(page, /DialogTitle/);
  assert.match(page, /DialogDescription/);
  assert.match(page, /DialogFooter/);
  assert.match(page, /pendingChange/);
  assert.match(page, /Cancel/);
  assert.match(page, /Turn \{pendingChange\.enabled \? "ON" : "OFF"\}/);
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

test("X API references and guidance disclose controlled availability and keep bidirectional links", () => {
  const dmGuide = read("src/app/docs/guides/x/direct-messages/page.tsx");
  const inboxReference = read("src/app/docs/api/inbox/page.tsx");
  const inboxListReference = read("src/app/docs/api/inbox/list/page.tsx");
  const creditsGuide = read("src/app/docs/guides/x/credits/page.tsx");
  const creditsReference = read("src/app/docs/api/x-credits/page.tsx");

  assert.match(dmGuide, /Controlled availability/);
  assert.match(dmGuide, /OAuth 2\.0 user-token subscription path returns 403/);
  assert.match(inboxReference, /FEATURE_NOT_AVAILABLE/);
  assert.match(creditsGuide, /controlled rollout/i);
  assert.match(creditsReference, /FEATURE_NOT_AVAILABLE/);
  assert.match(dmGuide, /\/docs\/api\/inbox/);
  assert.match(inboxListReference, /\/docs\/guides\/x\/direct-messages/);
  assert.match(creditsGuide, /\/docs\/api\/x-credits/);
  assert.match(creditsReference, /\/docs\/guides\/x\/credits/);
});
