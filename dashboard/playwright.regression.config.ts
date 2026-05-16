import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.DASHBOARD_BASE_URL || "https://app.unipost.dev";
const startLocalServer = process.env.DASHBOARD_WEB_SERVER === "1";

export default defineConfig({
  testDir: "./tests/regression",
  timeout: 45_000,
  expect: {
    timeout: 10_000,
  },
  fullyParallel: true,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: process.env.CI
    ? [["list"], ["html", { outputFolder: "playwright-report", open: "never" }]]
    : "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  webServer: startLocalServer
    ? {
        command: "npm run start -- --hostname 0.0.0.0 --port 3000",
        url: `${baseURL}/docs`,
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
      }
    : undefined,
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
