import { expect, test } from "@playwright/test";

// The marketing landing page ("/") is only served on the landing host
// (e.g. unipost.dev), where the proxy rewrites "/" -> /marketing. On the
// app host (app.unipost.dev, the default baseURL) "/" is auth-gated and
// "/marketing" redirects back to it, so the landing page must be reached
// via an absolute URL on the landing host. Pricing is public on both
// hosts, so it can use a baseURL-relative path.
const appBaseURL = process.env.DASHBOARD_BASE_URL || "https://app.unipost.dev";
const landingBaseURL = appBaseURL.replace("://app.", "://");

const mobilePublicRoutes = [
  { path: `${landingBaseURL}/`, marker: /Post to every social platform/i },
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
