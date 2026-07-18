import { expect, test } from "@playwright/test";

const expectedSHA = process.env.EXPECTED_PREVIEW_SHA;
const expectedAPIURL = process.env.EXPECTED_PREVIEW_API_URL?.replace(/\/+$/, "");
const dashboardBaseURL = process.env.DASHBOARD_BASE_URL;

if (!expectedSHA || !expectedAPIURL || !dashboardBaseURL) {
  throw new Error(
    "EXPECTED_PREVIEW_SHA, EXPECTED_PREVIEW_API_URL, and DASHBOARD_BASE_URL are required",
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

  const manifestResponse = await page.request.get("/__unipost-preview.json");
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

  const health = await page.evaluate(async (apiURL) => {
    const response = await fetch(`${apiURL}/health`, {
      credentials: "include",
      headers: { Accept: "application/json" },
    });
    return {
      ok: response.ok,
      status: response.status,
      allowedOrigin: response.headers.get("access-control-allow-origin"),
    };
  }, manifest.apiURL);

  expect(health).toEqual({
    ok: true,
    status: 200,
    allowedOrigin: new URL(dashboardBaseURL).origin,
  });
  expect(serverErrors).toEqual([]);
});
