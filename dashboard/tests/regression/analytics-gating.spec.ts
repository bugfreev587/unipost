import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";
import path from "node:path";

test.describe("analytics route gating", () => {
  test("applies the paid-plan analytics gate at the route group layout", async () => {
    const layoutSource = await readFile(
      path.join(process.cwd(), "src/app/(dashboard)/projects/[id]/analytics/layout.tsx"),
      "utf8",
    );
    const postsPageSource = await readFile(
      path.join(process.cwd(), "src/app/(dashboard)/projects/[id]/analytics/page.tsx"),
      "utf8",
    );

    expect(layoutSource).toContain('import { PlanGate } from "@/components/dashboard/plan-gate";');
    expect(layoutSource).toContain('<PlanGate feature="analytics">');
    expect(postsPageSource).not.toContain('<PlanGate feature="analytics">');
  });
});
