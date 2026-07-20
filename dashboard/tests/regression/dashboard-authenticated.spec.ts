import { clerk } from "@clerk/testing/playwright";
import { expect, test, type Page } from "@playwright/test";

const testEmail = process.env.DASHBOARD_TEST_EMAIL;
const configuredProfileId = process.env.DASHBOARD_TEST_PROFILE_ID;

if (!testEmail) {
  throw new Error("DASHBOARD_TEST_EMAIL is required for authenticated dashboard regression.");
}

test.describe("authenticated dashboard smoke", () => {
  test("core dashboard routes load and preserve plan-gated navigation", async ({ page }) => {
    await signIn(page, testEmail);
    const profileId = configuredProfileId || (await resolveProfileId(page));

    await expectDashboardRoute(page, `/projects/${profileId}`);
    await expectDashboardRoute(page, `/projects/${profileId}/accounts`);
    await expectDashboardRoute(page, `/projects/${profileId}/posts`);
    await expectDashboardRoute(page, `/projects/${profileId}/analytics`);
    await expectDashboardRoute(page, `/projects/${profileId}/settings`);

    await page.goto(`/projects/${profileId}/analytics/platforms/tiktok`, {
      waitUntil: "networkidle",
    });
    await expect(page.getByText("TikTok Analytics").first()).toBeVisible();

    await page.goto(`/projects/${profileId}/analytics/platforms/youtube`, {
      waitUntil: "networkidle",
    });
    await expect(page.getByText("YouTube Analytics").first()).toBeVisible();
  });
});

async function signIn(page: Page, emailAddress: string) {
  await page.goto("/pricing", { waitUntil: "domcontentloaded" });
  await clerk.signIn({ page, emailAddress });
  await page.goto("/projects", { waitUntil: "networkidle" });
}

async function resolveProfileId(page: Page): Promise<string> {
  await page.goto("/projects", { waitUntil: "networkidle" });
  const projectLink = page.locator('a[href^="/projects/"]').first();
  await expect(projectLink).toBeVisible();
  const href = await projectLink.getAttribute("href");
  const profileId = href?.match(/^\/projects\/([^/]+)/)?.[1];
  if (!profileId) throw new Error("Could not resolve dashboard profile id from /projects");
  return profileId;
}

async function expectDashboardRoute(page: Page, routePath: string) {
  const failedRequests: string[] = [];
  page.on("response", (response) => {
    if (response.status() >= 500) {
      failedRequests.push(`${response.status()} ${response.url()}`);
    }
  });

  await page.goto(routePath, { waitUntil: "networkidle" });
  await expect(page.locator("body")).toContainText(
    /Navigate|Settings|Posts|Analytics|Connections|Profiles/,
  );
  expect(failedRequests).toEqual([]);
}
