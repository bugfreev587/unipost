import { expect, test } from "@playwright/test";

// The marketing landing page is only served on the landing host (e.g.
// unipost.dev), where src/proxy.ts rewrites "/" -> /marketing. On the app
// host (app.unipost.dev, the default baseURL) "/" is auth-gated and
// "/marketing" redirects back to it, so the landing page must be reached
// via an absolute URL on the landing host. The dedicated local regression
// gate uses the RFC localhost names dev-app.localhost and dev.localhost on one
// server, while the ordinary single-host CI build still skips this assertion.
// Pricing is public on every host, so it stays baseURL-relative.
const appBaseURL = process.env.DASHBOARD_BASE_URL || "https://app.unipost.dev";
const landingBaseURL = appBaseURL
  .replace("://staging-app.", "://staging.")
  .replace("://dev-app.", "://dev.")
  .replace("://app.", "://");
const landingHostTestable = landingBaseURL !== appBaseURL;

const mobilePublicRoutes = [
  {
    path: `${landingBaseURL}/`,
    marker: /Post to every social platform/i,
    requiresLandingHost: true,
  },
  { path: "/pricing", marker: /Start free/i },
  {
    path: "/docs/api/posts/retry",
    marker: /Queue one new delivery attempt for a failed per-destination result/i,
  },
  {
    path: "/docs/guides/posts/retry-failed-posts",
    marker: /Decide whether UniPost will retry a failed destination automatically/i,
  },
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

      if (route.path === "/pricing") {
        const planCards = page.locator(".pr-card");
        await expect(planCards).toHaveCount(4);
        const cardBounds = await planCards.evaluateAll((cards) =>
          cards.map((card) => {
            const rect = card.getBoundingClientRect();
            return { left: rect.left, right: rect.right, width: rect.width };
          }),
        );
        for (const bounds of cardBounds) {
          expect(bounds.left).toBeGreaterThanOrEqual(0);
          expect(bounds.right).toBeLessThanOrEqual(392);
          expect(bounds.width).toBeGreaterThan(300);
        }
      }
    });
  }
});
