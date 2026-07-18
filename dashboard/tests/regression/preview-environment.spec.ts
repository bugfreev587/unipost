import { expect, test } from "@playwright/test";

const expectedSHA = process.env.EXPECTED_PREVIEW_SHA;
const expectedAPIURL = process.env.EXPECTED_PREVIEW_API_URL?.replace(/\/+$/, "");
const dashboardBaseURL = process.env.DASHBOARD_BASE_URL;
const automationBypassSecret =
  process.env.VERCEL_AUTOMATION_BYPASS_SECRET;

if (
  !expectedSHA ||
  !expectedAPIURL ||
  !dashboardBaseURL ||
  !automationBypassSecret
) {
  throw new Error(
    "EXPECTED_PREVIEW_SHA, EXPECTED_PREVIEW_API_URL, DASHBOARD_BASE_URL, and VERCEL_AUTOMATION_BYPASS_SECRET are required",
  );
}

test("frontend and API are the same isolated preview pair", async ({ page }) => {
  const dashboardHost = new URL(dashboardBaseURL).hostname;
  expect(dashboardHost).toMatch(/\.vercel\.app$/);
  expect([
    "app.unipost.dev",
    "dev-app.unipost.dev",
    "staging-app.unipost.dev",
  ]).not.toContain(dashboardHost);

  const manifestResponse = await page.request.get("/__unipost-preview.json", {
    headers: {
      "x-vercel-protection-bypass": automationBypassSecret,
      "x-vercel-set-bypass-cookie": "true",
    },
  });
  expect(manifestResponse.ok()).toBeTruthy();
  expect(manifestResponse.headers()["content-type"]).toContain("application/json");
  const manifest = await manifestResponse.json();
  expect(manifest.sha).toBe(expectedSHA);
  expect(manifest.apiURL).toBe(expectedAPIURL);
  expect(new URL(manifest.apiURL).hostname).toMatch(/\.up\.railway\.app$/);

  const serverErrors: string[] = [];
  page.on("response", (response) => {
    if (response.status() >= 500) {
      serverErrors.push(`${response.status()} ${response.url()}`);
    }
  });

  await page.goto("/docs", { waitUntil: "domcontentloaded" });
  await expect(page.locator("article").first()).toContainText(/UniPost|API/);

  const dashboardOrigin = new URL(dashboardBaseURL).origin;
  const corsProbe = await page.request.get(`${manifest.apiURL}/health`, {
    headers: { Origin: dashboardOrigin },
  });
  expect(corsProbe.ok()).toBeTruthy();
  expect(corsProbe.headers()["access-control-allow-origin"]).toBe(
    dashboardOrigin,
  );

  const health = await page.evaluate(async (apiURL) => {
    const response = await fetch(`${apiURL}/health`, {
      credentials: "include",
      headers: { Accept: "application/json" },
    });
    return {
      ok: response.ok,
      status: response.status,
    };
  }, manifest.apiURL);

  expect(health).toEqual({
    ok: true,
    status: 200,
  });
  expect(serverErrors).toEqual([]);
});
