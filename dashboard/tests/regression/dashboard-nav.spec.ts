import { expect, test } from "@playwright/test";
import { projectNavSubItemIsActive } from "../../src/lib/dashboard-nav";

test.describe("dashboard sidebar route matching", () => {
  test("keeps analytics posts exact so platforms does not double-select posts", () => {
    const pathname = "/projects/profile-1/analytics/platforms";

    expect(
      projectNavSubItemIsActive(pathname, "profile-1", "/analytics", {
        exactMatch: true,
      }),
    ).toBe(false);
    expect(
      projectNavSubItemIsActive(pathname, "profile-1", "/analytics/platforms"),
    ).toBe(true);
  });

  test("keeps platform detail routes under the platforms submenu", () => {
    expect(
      projectNavSubItemIsActive(
        "/projects/profile-1/analytics/platforms/instagram",
        "profile-1",
        "/analytics/platforms",
      ),
    ).toBe(true);
  });
});
