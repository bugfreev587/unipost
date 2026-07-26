import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

const apiSource = readFileSync(resolve("src/lib/api.ts"), "utf8");
const adminBillingSource = readFileSync(resolve("src/app/admin/billing/page.tsx"), "utf8");
const pricingSource = readFileSync(resolve("src/app/pricing/pricing-page-client.tsx"), "utf8");
const settingsBillingSource = readFileSync(resolve("src/app/(dashboard)/settings/billing/page.tsx"), "utf8");

async function loadTrialFormatModule() {
  const source = readFileSync(resolve("src/lib/trial-format.ts"), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  });
  const dataUrl = `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`;
  return import(dataUrl);
}

async function loadAdminBillingFenceModule() {
  const sourceFile = ts.createSourceFile(
    "admin-billing-page.tsx",
    adminBillingSource,
    ts.ScriptTarget.ES2022,
    true,
    ts.ScriptKind.TSX,
  );
  const names = new Set(["isCurrentAdminBillingLoad", "reconcileRefreshRequiredLocks"]);
  const declarations = sourceFile.statements
    .filter((node) => ts.isFunctionDeclaration(node) && node.name && names.has(node.name.text))
    .map((node) => node.getText(sourceFile));
  assert.equal(declarations.length, names.size, "expected executable Admin Billing fence helpers");
  const compiled = ts.transpileModule(
    `${declarations.join("\n")}\nexport { isCurrentAdminBillingLoad, reconcileRefreshRequiredLocks };`,
    {
    compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
    },
  );
  const dataUrl = `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`;
  return import(dataUrl);
}

test("billing API exposes all managed trial contracts", () => {
  for (const value of [
    "WorkspaceTrial",
    "WorkspaceTrialHistoryEntry",
    "AdminWorkspaceTrial",
    "TrialMutationProjection",
    "free_to_paid",
    "paid_same_plan",
    "provisioning",
    "pending_activation",
    "checkout_pending",
    "scheduled",
    "active",
    "completed",
    "canceled",
    "revoked",
    "superseded",
    "failed",
    "getTrialHistory",
    "grantAdminTrial",
    "revokeAdminTrial",
    "cancelTrialRenewal",
    "changeTrialPlan",
  ]) {
    assert.match(apiSource, new RegExp(`\\b${value}\\b`));
  }
});

test("admin and history trial contracts include renewal timeline billing fields", () => {
  for (const name of ["WorkspaceTrialHistoryEntry", "AdminWorkspaceTrial"]) {
    const match = apiSource.match(new RegExp(`export interface ${name}[^\\{]*\\{([\\s\\S]*?)\\n\\}`));
    assert.ok(match, `missing ${name} interface`);
    assert.match(match[1], /post_trial_price_cents:\s*number;/, `${name} must expose post-trial price`);
    assert.match(match[1], /cancel_at_period_end:\s*boolean;/, `${name} must expose renewal cancellation`);
  }
});

test("trial API functions use the backend routes and exact mutation fields", () => {
  for (const route of [
    "/v1/billing/trials",
    "/v1/admin/workspaces/",
    "/trials/",
    "/revoke",
    "/cancel-renewal",
    "/v1/billing/change-plan",
  ]) {
    assert.ok(apiSource.includes(route), `missing route fragment ${route}`);
  }
  for (const field of [
    "duration_days",
    "post_trial_price_cents",
    "changing_plan_forfeits_trial",
    "terminal_reason",
    "failure_reason",
    "target_plan",
    "change_pending",
  ]) {
    assert.match(apiSource, new RegExp(`\\b${field}\\b`));
  }
});

test("shared formatter supports explicit UTC dates and stable remaining-day boundaries", async () => {
  const { formatWorkspaceTrial } = await loadTrialFormatModule();
  const trial = {
    id: "trial_1",
    kind: "paid_same_plan",
    plan_id: "growth",
    duration_days: 30,
    status: "scheduled",
    granted_at: "2026-07-24T23:30:00-07:00",
    scheduled_start_at: "2026-08-01T00:00:00Z",
    ends_at: "2026-08-31T00:00:00Z",
    post_trial_price_cents: 4900,
    cancel_at_period_end: false,
    changing_plan_forfeits_trial: true,
  };
  const formatted = formatWorkspaceTrial(
    trial,
    { now: "2026-08-29T12:00:00Z", timeZone: "UTC" },
  );

  assert.deepEqual(formatted.badge, { label: "Scheduled", tone: "info" });
  assert.equal(formatted.headline, "30-day Growth trial scheduled");
  assert.equal(formatted.remainingDays, null);
  assert.deepEqual(formatted.timeline, [
    { label: "Current period ends", value: "Aug 1, 2026" },
    { label: "Trial starts", value: "Aug 1, 2026" },
    { label: "Trial ends", value: "Aug 31, 2026" },
    { label: "Billing resumes", value: "Aug 31, 2026 · $49.00/month" },
  ]);
  assert.equal(formatted.terminalReason, null);

  const active = formatWorkspaceTrial(
    { ...trial, status: "active", started_at: "2026-08-01T00:00:00Z" },
    { now: "2026-08-29T12:00:00Z", timeZone: "UTC" },
  );
  assert.equal(active.remainingDays, 2);

  const renewalCanceled = formatWorkspaceTrial(
    { ...trial, status: "active", started_at: "2026-08-01T00:00:00Z", cancel_at_period_end: true },
    { now: "2026-08-29T12:00:00Z", timeZone: "UTC" },
  );
  assert.deepEqual(renewalCanceled.timeline, [
    { label: "Trial started", value: "Aug 1, 2026" },
    { label: "Trial ends", value: "Aug 31, 2026" },
    { label: "Access ends", value: "Aug 31, 2026" },
  ]);
});

test("shared formatter defaults to the viewer timezone", async () => {
  const { formatWorkspaceTrial } = await loadTrialFormatModule();
  const previousTimeZone = process.env.TZ;
  process.env.TZ = "America/Los_Angeles";
  try {
    const trial = {
      id: "trial_local",
      kind: "free_to_paid",
      plan_id: "api",
      duration_days: 7,
      status: "pending_activation",
      granted_at: "2026-07-25T01:00:00Z",
    };
    assert.deepEqual(formatWorkspaceTrial(trial).timeline, [
      { label: "Granted", value: "Jul 24, 2026" },
      { label: "Activation", value: "Complete checkout to start" },
    ]);
    assert.deepEqual(formatWorkspaceTrial(trial, { timeZone: "UTC" }).timeline, [
      { label: "Granted", value: "Jul 25, 2026" },
      { label: "Activation", value: "Complete checkout to start" },
    ]);
  } finally {
    if (previousTimeZone === undefined) delete process.env.TZ;
    else process.env.TZ = previousTimeZone;
  }
});

test("shared formatter normalizes terminal reasons without leaking internal failures", async () => {
  const { formatWorkspaceTrial } = await loadTrialFormatModule();

  const revoked = formatWorkspaceTrial(
    {
      id: "trial_2",
      kind: "free_to_paid",
      plan_id: "basic",
      duration_days: 14,
      status: "revoked",
      granted_at: "2026-07-24T00:00:00Z",
      revoked_at: "2026-07-25T00:00:00Z",
      terminal_reason: "offer_revoked",
    },
    { timeZone: "UTC" },
  );
  assert.equal(revoked.terminalReason, "Offer revoked");
  assert.deepEqual(revoked.timeline, [
    { label: "Granted", value: "Jul 24, 2026" },
    { label: "Revoked", value: "Jul 25, 2026" },
  ]);

  const unknown = formatWorkspaceTrial({
    id: "trial_future",
    kind: "future_kind",
    plan_id: "future_plan",
    duration_days: 10,
    status: "future_status",
  });
  assert.deepEqual(unknown.badge, { label: "Unknown", tone: "neutral" });
  assert.equal(unknown.headline, "Trial status unavailable");
  assert.equal(unknown.remainingDays, null);
  assert.equal(unknown.terminalReason, null);
});

test("Admin Billing exposes safe row-level trial grant controls", () => {
  for (const value of [
    "GrantTrialForm",
    "grantAdminTrial",
    "revokeAdminTrial",
    "Grant Trial",
    "API",
    "Basic",
    "Growth",
    "Team",
    "Pending activation",
    "Checkout pending",
    "Scheduled",
    "Active",
    "Failed",
    "Revoke",
    "Granting…",
    "Revoking…",
    "Use 1–730 whole days.",
    "role=\"alert\"",
  ]) {
    assert.ok(adminBillingSource.includes(value), `missing Admin Billing trial UI contract: ${value}`);
  }

  assert.match(adminBillingSource, /type="number"[\s\S]*min=\{1\}[\s\S]*max=\{730\}[\s\S]*step=\{1\}/);
  assert.match(adminBillingSource, /row\.plan_id === "free"[\s\S]*<select/);
  assert.match(adminBillingSource, /formatWorkspaceTrial\(/);
  assert.match(adminBillingSource, /pending_activation[\s\S]*checkout_pending/);
  assert.match(adminBillingSource, /disabled=\{[\s\S]*(?:busy|hasOpenTrial)/);
});

test("Admin Billing uses UniPost confirmation modals for trial mutations", () => {
  const start = adminBillingSource.indexOf("function GrantTrialForm(");
  const end = adminBillingSource.indexOf("function PlanFlipMenu(");
  assert.ok(start >= 0 && end > start, "expected GrantTrialForm source boundary");
  const grantTrialFormSource = adminBillingSource.slice(start, end);

  assert.match(adminBillingSource, /import \{ ConfirmModal \} from "@\/components\/confirm-modal"/);
  assert.doesNotMatch(grantTrialFormSource, /(?:window\.)?confirm\s*\(/);
  assert.match(grantTrialFormSource, /title="Grant trial"[\s\S]*confirmLabel="Grant Trial"/);
  assert.match(grantTrialFormSource, /title="Revoke trial offer"[\s\S]*confirmLabel="Revoke"[\s\S]*variant="danger"/);
  assert.match(grantTrialFormSource, /onCancel=\{\(\) => setConfirmation\(null\)\}/);
});

test("Admin Billing serializes workspace mutations and fails closed after refresh errors", () => {
  for (const value of [
    "WorkspaceMutationState",
    "workspaceMutations",
    "setWorkspaceMutation",
    "refresh_required",
    "Refresh required",
    "Refresh Billing before making another change.",
  ]) {
    assert.ok(adminBillingSource.includes(value), `missing Admin Billing mutation safety contract: ${value}`);
  }

  assert.match(adminBillingSource, /<PlanFlipMenu[\s\S]*hasOpenTrial=\{[\s\S]*workspaceMutation=\{/);
  assert.match(adminBillingSource, /function PlanFlipMenu\([\s\S]*hasOpenTrial[\s\S]*workspaceMutation[\s\S]*const disabled = busy \|\| hasOpenTrial \|\| !!workspaceMutation/);
  assert.match(adminBillingSource, /function GrantTrialForm\([\s\S]*workspaceMutation[\s\S]*if \(busy \|\| hasOpenTrial \|\| workspaceMutation\)/);
  assert.match(adminBillingSource, /await onChanged\([^)]*\);[\s\S]*setSuccess\(/);
  assert.match(adminBillingSource, /mutationSucceeded[\s\S]*"refresh_required"/);
});

test("Admin Billing ignores stale loads and scopes refresh-required unlocks", async () => {
  for (const value of [
    "loadRequestSequence",
    "requestSequence",
    "Billing request superseded by a newer refresh.",
    "clearRefreshRequiredFor",
    "clearAllRefreshRequired",
  ]) {
    assert.ok(adminBillingSource.includes(value), `missing Admin Billing stale-load fence: ${value}`);
  }

  assert.match(adminBillingSource, /if \(!isCurrentAdminBillingLoad\([\s\S]*throw new Error\(SUPERSEDED_LOAD_MESSAGE\);[\s\S]*setRows\(/);
  assert.match(adminBillingSource, /isCurrentAdminBillingLoad\([\s\S]*setError\(/);
  assert.match(adminBillingSource, /requestSequence === loadRequestSequence\.current[\s\S]*setLoading\(false\)/);
  assert.match(adminBillingSource, /onChanged=\{\(mutationEpoch\) => loadBilling\(\{ clearRefreshRequiredFor: row\.workspace_id, mutationEpoch \}\)\}/);
  assert.match(adminBillingSource, /loadBilling\(\{ clearAllRefreshRequired: true \}\)/);

  const { isCurrentAdminBillingLoad, reconcileRefreshRequiredLocks } = await loadAdminBillingFenceModule();
  const requestA = { sequence: 1, mutationEpoch: 0 };
  const latestRequestSequence = 2; // mutation refresh B started and then failed
  const currentMutationEpoch = 1;
  let rows = ["before mutation"];
  let locks = { workspace_1: "refresh_required" };

  const requestACanApply = isCurrentAdminBillingLoad(
    requestA.sequence,
    latestRequestSequence,
    requestA.mutationEpoch,
    currentMutationEpoch,
  );
  if (requestACanApply) {
    rows = ["stale request A"];
    locks = reconcileRefreshRequiredLocks(locks, { clearAllRefreshRequired: true }, { workspace_1: 1 });
  }
  assert.equal(requestACanApply, false);
  assert.deepEqual(rows, ["before mutation"]);
  assert.deepEqual(locks, { workspace_1: "refresh_required" });

  const twoLocks = { workspace_1: "refresh_required", workspace_2: "refresh_required" };
  assert.deepEqual(
    reconcileRefreshRequiredLocks(
      twoLocks,
      { clearRefreshRequiredFor: "workspace_1", mutationEpoch: 2 },
      { workspace_1: 3 },
    ),
    twoLocks,
    "an older mutation epoch must not unlock a newer workspace mutation",
  );
  assert.deepEqual(
    reconcileRefreshRequiredLocks(
      twoLocks,
      { clearRefreshRequiredFor: "workspace_1", mutationEpoch: 3 },
      { workspace_1: 3 },
    ),
    { workspace_2: "refresh_required" },
    "a matching mutation refresh may unlock only its workspace",
  );
});

test("Pricing decorates only the granted tier and preserves trial-aware plan actions", () => {
  for (const value of [
    "WorkspaceTrial",
    "formatWorkspaceTrial",
    "Start ${trial.duration_days}-day free trial",
    "Continue checkout",
    "Changing plans ends your trial immediately",
    "changeTrialPlan",
    "createCheckout",
    "Trial action failed",
  ]) {
    assert.ok(pricingSource.includes(value), `missing Pricing trial contract: ${value}`);
  }

  assert.match(pricingSource, /const isTrialTier = trial\?\.plan_id === t\.id/);
  assert.match(pricingSource, /trial\.status === "pending_activation"[\s\S]*createCheckout\(token, planId\)/);
  assert.match(pricingSource, /trial\.status === "checkout_pending"[\s\S]*createCheckout\(token, planId\)/);
  assert.ok(
    readFileSync(resolve("src/lib/trial-format.ts"), "utf8").includes("Activation in progress"),
    "shared formatter must expose the checkout-pending status label",
  );
  assert.match(pricingSource, /hasForfeitableTrial[\s\S]*window\.confirm[\s\S]*changeTrialPlan/);
  assert.doesNotMatch(pricingSource, /Paid plans do not include a separate time-limited trial/);
});

test("Settings Billing renders the managed trial timeline, controls, and durable history", () => {
  for (const value of [
    "getTrialHistory",
    "formatWorkspaceTrial",
    "Managed trial",
    "Cancel renewal",
    "Changing plans ends your trial immediately",
    "changeTrialPlan",
    "cancelTrialRenewal",
    "Trial History",
    "Loading trial history",
    "Trial history could not be loaded",
    "No trial history yet",
    "terminalReason",
  ]) {
    assert.ok(settingsBillingSource.includes(value), `missing Settings Billing trial contract: ${value}`);
  }

  assert.match(settingsBillingSource, /Promise\.allSettled\(\[[\s\S]*getBilling\(token\)[\s\S]*getTrialHistory\(token\)/);
  assert.match(settingsBillingSource, /\[\.\.\.historyResult\.value\.data\]\.sort/);
  assert.match(settingsBillingSource, /hasForfeitableTrial[\s\S]*window\.confirm[\s\S]*changeTrialPlan/);
  assert.match(settingsBillingSource, /cancelTrialRenewal\(token,\s*billing\.trial\.id\)/);
  assert.match(settingsBillingSource, /className="trial-history-list"/);
});

test("Settings Billing dispatches free, paid, and managed-trial plan changes to exclusive flows", () => {
  assert.match(
    apiSource,
    /export async function createPlanChangeSession\([\s\S]*?\): Promise<ApiResponse<\{ url: string \}>>[\s\S]*?\/v1\/billing\/plan-change-session[\s\S]*?JSON\.stringify\(\{ plan_id: planId \}\)/,
  );
  assert.ok(settingsBillingSource.includes("createPlanChangeSession"));
  assert.match(
    settingsBillingSource,
    /hasForfeitableTrial[\s\S]*?changeTrialPlan\(token, planId\)[\s\S]*?if \(billing\?\.plan === "free"\)[\s\S]*?createCheckout\(token, planId\)[\s\S]*?checkout_url[\s\S]*?createPlanChangeSession\(token, planId\)[\s\S]*?res\.data\.url/,
  );
  assert.match(pricingSource, /settings\/billing\?upgrade=\$\{encodeURIComponent\(planId\)\}/);
  assert.doesNotMatch(
    settingsBillingSource,
    /createPlanChangeSession\(token, planId\)[\s\S]{0,300}setBilling\(/,
    "Portal redirect must wait for webhook-backed billing reload instead of mutating the plan optimistically",
  );
});
