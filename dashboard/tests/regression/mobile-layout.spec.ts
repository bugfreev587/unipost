import { expect, test } from "@playwright/test";

// The marketing landing page is only served on the landing host (e.g.
// unipost.dev), where src/proxy.ts rewrites "/" -> /marketing. On the app
// host (app.unipost.dev, the default baseURL) "/" is auth-gated and
// "/marketing" redirects back to it, so the landing page must be reached
// via an absolute URL on the landing host. The CI "Dashboard build" job
// runs a single local server that the proxy always treats as the app host,
// so no distinct landing host exists there and the landing assertion is
// skipped. Pricing is public on every host, so it stays baseURL-relative.
const appBaseURL = process.env.DASHBOARD_BASE_URL || "https://app.unipost.dev";
const landingBaseURL = appBaseURL.replace("://app.", "://");
const landingHostTestable =
  landingBaseURL !== appBaseURL && !/localhost|127\.0\.0\.1/.test(appBaseURL);

const mobilePublicRoutes = [
  {
    path: `${landingBaseURL}/`,
    marker: /Post to every social platform/i,
    requiresLandingHost: true,
  },
  { path: "/pricing", marker: /Start free/i },
];

test.describe("mobile public layout", () => {
  test.use({
    viewport: { width: 390, height: 844 },
    isMobile: true,
  });

  for (const route of mobilePublicRoutes) {
    test(`${route.path} avoids mobile horizontal overflow`, async ({ page }) => {
      test.skip(
        Boolean(route.requiresLandingHost) && !landingHostTestable,
        "Landing page is served only on a distinct landing host; the local CI server is the app host.",
      );

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
