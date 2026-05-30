import { expect, test } from "@playwright/test";

const mobilePublicRoutes = [
  { path: "/", marker: /Post to every social platform/i },
  { path: "/pricing", marker: /Start free/i },
];

test.describe("mobile public layout", () => {
  test.use({
    viewport: { width: 390, height: 844 },
    isMobile: true,
  });

  for (const route of mobilePublicRoutes) {
    test(`${route.path} avoids mobile horizontal overflow`, async ({ page }) => {
      await page.goto(route.path, { waitUntil: "domcontentloaded" });
      await expect(page.getByText(route.marker).first()).toBeVisible();

      const layout = await page.evaluate(() => {
        const root = document.documentElement;
        const nav = document.querySelector(".mk-nav");
        return {
          clientWidth: root.clientWidth,
          scrollWidth: root.scrollWidth,
          navHeight: nav?.getBoundingClientRect().height ?? 0,
        };
      });

      expect(layout.scrollWidth).toBeLessThanOrEqual(layout.clientWidth + 2);
      expect(layout.navHeight).toBeLessThanOrEqual(112);
    });
  }
});
