import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.DASHBOARD_BASE_URL;
if (!baseURL) {
  throw new Error("DASHBOARD_BASE_URL is required for preview acceptance");
}

export default defineConfig({
  testDir: "./tests/regression",
  testMatch: "preview-environment.spec.ts",
  timeout: 45_000,
  expect: {
    timeout: 10_000,
  },
  fullyParallel: false,
  retries: 0,
  workers: 1,
  reporter: [
    ["list"],
    ["junit", { outputFile: "test-results/preview-junit.xml" }],
    ["html", { outputFolder: "playwright-report-preview", open: "never" }],
  ],
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    {
      name: "preview-chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
