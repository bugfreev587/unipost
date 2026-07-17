import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";
import path from "node:path";

test.describe("Team plan security gates", () => {
  test("keeps Audit Log gated in the API and dashboard", async () => {
    const planGateSource = await readFile(
      path.join(process.cwd(), "src/components/dashboard/plan-gate.tsx"),
      "utf8",
    );
    const auditPageSource = await readFile(
      path.join(process.cwd(), "src/app/(dashboard)/settings/audit-log/page.tsx"),
      "utf8",
    );
    const apiRoutesSource = await readFile(
      path.join(process.cwd(), "../api/cmd/api/main.go"),
      "utf8",
    );

    expect(planGateSource).toContain('type Feature = "inbox" | "analytics" | "audit_log";');
    expect(planGateSource).toContain('feature === "audit_log"');
    expect(planGateSource).toContain("res.data.plan_allows_audit_log");
    expect(auditPageSource).toContain('import { PlanGate } from "@/components/dashboard/plan-gate";');
    expect(auditPageSource).toContain('<PlanGate feature="audit_log">');
    expect(apiRoutesSource).toContain(
      'r.With(handler.RequirePlanAuditLog(quotaChecker)).Get("/v1/audit-log", auditHandler.List)',
    );
  });
});
