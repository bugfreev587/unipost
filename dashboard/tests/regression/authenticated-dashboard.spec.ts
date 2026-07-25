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

      await expectDashboardRoute(page, `/projects/${profileID}`);
      await expectDashboardRoute(page, `/projects/${profileID}/accounts`);
      await expectDashboardRoute(page, `/projects/${profileID}/posts`);
      await expectDashboardRoute(page, `/projects/${profileID}/analytics`);
      await expectDashboardRoute(page, `/projects/${profileID}/settings`);

      await expectPlanGatedAnalyticsRoute(page, `/projects/${profileID}/analytics/platforms/tiktok`);
      await expectPlanGatedAnalyticsRoute(page, `/projects/${profileID}/analytics/platforms/youtube`);
    },
  );
});

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
