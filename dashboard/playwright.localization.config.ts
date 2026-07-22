import { defineConfig, devices } from "@playwright/test";

const port = process.env.DASHBOARD_LOCALIZATION_PORT || "3107";
const baseURL = process.env.DASHBOARD_LANDING_BASE_URL || `http://landing.localhost:${port}`;
const startLocalServer = process.env.DASHBOARD_WEB_SERVER === "1";

export default defineConfig({
  testDir: "./tests/regression",
  testMatch: "localization.spec.ts",
  timeout: 45_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  reporter: "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  webServer: startLocalServer
    ? {
        command: `npm run start -- --hostname 0.0.0.0 --port ${port}`,
        url: `http://localhost:${port}/pricing`,
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
      }
    : undefined,
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
