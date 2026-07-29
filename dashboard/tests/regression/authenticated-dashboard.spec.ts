import { expect, test, type Page } from "@playwright/test";

import {
  bootstrapSyntheticUser,
  cleanupSyntheticUser,
  createSyntheticClerkUser,
  loadSyntheticAuthConfig,
  runWithCleanup,
  signInSyntheticUser,
} from "./support/synthetic-auth.mjs";

const config = loadSyntheticAuthConfig();

test("core dashboard routes load and preserve plan-gated navigation with a passwordless Clerk ticket", async ({ page }) => {
  let identity: Awaited<ReturnType<typeof createSyntheticClerkUser>> | undefined;
  let authState: Awaited<ReturnType<typeof bootstrapSyntheticUser>> | undefined;

  await runWithCleanup(
    async () => {
      if (!identity) return;
      await cleanupSyntheticUser(config, identity, authState);
    },
    async () => {
      identity = await createSyntheticClerkUser(config);
      await signInSyntheticUser(page, config, identity);
      authState = await bootstrapSyntheticUser(page, config);
      const profileID = authState.profileID;
      await installYouTubeAccountFixture(page, profileID);

      await expectDashboardRoute(page, `/projects/${profileID}`);
      await expectYouTubeAccountIdentity(page, profileID);
      await expectDashboardRoute(page, `/projects/${profileID}/posts`);
      await expectDashboardRoute(page, `/projects/${profileID}/analytics`);
      await expectDashboardRoute(page, `/projects/${profileID}/settings`);

      await expectPlanGatedAnalyticsRoute(page, `/projects/${profileID}/analytics/platforms/tiktok`);
      await expectPlanGatedAnalyticsRoute(page, `/projects/${profileID}/analytics/platforms/youtube`);
    },
  );
});

async function installYouTubeAccountFixture(page: Page, profileID: string) {
  await page.route(`**/v1/profiles/${profileID}/accounts*`, async (route) => {
    if (route.request().method() !== "GET") {
      await route.continue();
      return;
    }

    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: [
          {
            id: "youtube-audit-fixture",
            profile_id: profileID,
            platform: "youtube",
            account_name: "UniPost Audit Channel",
            external_account_id: "UCunipostAuditFixture",
            account_avatar_url: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='80' height='80'%3E%3Crect width='80' height='80' rx='40' fill='%23334155'/%3E%3Ctext x='40' y='48' text-anchor='middle' font-size='24' fill='white'%3EUA%3C/text%3E%3C/svg%3E",
            connected_at: "2026-07-29T12:00:00.000Z",
            status: "active",
            connection_type: "managed",
            scope: ["youtube.readonly", "yt-analytics.readonly"],
          },
        ],
      }),
    });
  });
}

async function expectYouTubeAccountIdentity(page: Page, profileID: string) {
  await expectDashboardRoute(page, `/projects/${profileID}/accounts`);

  const identity = page.locator("[data-youtube-channel-identity]");
  const sourceLink = identity.locator("[data-youtube-source-link]");
  const status = page.locator("[data-unipost-account-status]");
  await expect(identity).toContainText("UniPost Audit Channel");
  await expect(sourceLink).toContainText("Source: YouTube");
  await expect(sourceLink).toHaveAttribute(
    "href",
    "https://www.youtube.com/channel/UCunipostAuditFixture",
  );
  await expect(sourceLink).toHaveAttribute("rel", "noopener noreferrer");
  await expect(status).toContainText("Active");
  await expect(page.locator('path[fill="#ff0000"]')).toHaveCount(0);

  await sourceLink.focus();
  await expect(sourceLink).toBeFocused();
  expect(await sourceLink.evaluate((element) => getComputedStyle(element).outlineStyle)).not.toBe("none");

  await page.setViewportSize({ width: 390, height: 844 });
  await expect(identity).toBeVisible();
  await expect(sourceLink).toBeVisible();
  await page.setViewportSize({ width: 1280, height: 900 });
}

async function expectDashboardRoute(page: Page, path: string) {
  const failedRequests: string[] = [];
  const recordFailure = (response: { status(): number; url(): string }) => {
    if (response.status() >= 500) failedRequests.push(`${response.status()} ${response.url()}`);
  };
  page.on("response", recordFailure);
  try {
    await page.goto(path, { waitUntil: "domcontentloaded" });
    await expect(page.locator("body")).toContainText(/Navigate|Settings|Posts|Analytics|Connections|Profiles/);
    expect(failedRequests).toEqual([]);
  } finally {
    page.off("response", recordFailure);
  }
}

async function expectPlanGatedAnalyticsRoute(page: Page, path: string) {
  await expectDashboardRoute(page, path);
  await expect(page).toHaveURL(new RegExp(`${path.replaceAll("/", "\\/")}$`));
  await expect(page.getByText("Analytics is a paid plan feature", { exact: true })).toBeVisible();
}
