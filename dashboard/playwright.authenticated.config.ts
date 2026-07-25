import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.DASHBOARD_BASE_URL;
if (!baseURL) {
  throw new Error("DASHBOARD_BASE_URL is required for authenticated Dashboard regression");
}

export default defineConfig({
  testDir: "./tests/regression",
  testMatch: "authenticated-dashboard.spec.ts",
  timeout: 90_000,
  expect: {
    timeout: 15_000,
  },
  fullyParallel: false,
  retries: 0,
  workers: 1,
  reporter: process.env.CI
    ? [["list"], ["html", { outputFolder: "playwright-report-authenticated", open: "never" }]]
    : "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    {
      name: "authenticated-chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
