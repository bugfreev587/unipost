import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.DASHBOARD_BASE_URL;
const bypassSecret = process.env.VERCEL_AUTOMATION_BYPASS_SECRET;
if (!baseURL || !bypassSecret) {
  throw new Error(
    "DASHBOARD_BASE_URL and VERCEL_AUTOMATION_BYPASS_SECRET are required for preview acceptance",
  );
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
    extraHTTPHeaders: {
      "x-vercel-protection-bypass": bypassSecret,
      "x-vercel-set-bypass-cookie": "true",
    },
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
