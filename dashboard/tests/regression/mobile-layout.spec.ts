import { expect, test } from "@playwright/test";

// The marketing landing page is served on a separate host, where src/proxy.ts
// rewrites "/" -> /marketing. A *.localhost alias lets the same local server
// exercise both the app-host and landing-host paths without skipping coverage.
const appBaseURL = process.env.DASHBOARD_BASE_URL || "https://app.unipost.dev";

function resolveLandingBaseURL(baseURL: string) {
  const url = new URL(baseURL);

  const hostnames: Record<string, string> = {
    "app.unipost.dev": "unipost.dev",
    "dev-app.unipost.dev": "dev.unipost.dev",
    "staging-app.unipost.dev": "staging.unipost.dev",
    localhost: "landing.localhost",
    "127.0.0.1": "landing.localhost",
  };

  url.hostname = hostnames[url.hostname] || url.hostname;
  return url.origin;
}

const landingBaseURL =
  process.env.DASHBOARD_LANDING_BASE_URL || resolveLandingBaseURL(appBaseURL);

const mobilePublicRoutes = [
  {
    path: `${landingBaseURL}/`,
    marker: /Post to every social platform/i,
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
