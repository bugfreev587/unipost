import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.DASHBOARD_BASE_URL || "https://app.unipost.dev";
const startLocalServer = process.env.DASHBOARD_WEB_SERVER === "1";
const authenticatedRegressionEnabled = Boolean(
  process.env.DASHBOARD_TEST_EMAIL &&
    process.env.CLERK_SECRET_KEY &&
    process.env.CLERK_SECRET_KEY !== "sk_test_dummy",
);

export default defineConfig({
  testDir: "./tests/regression",
  testIgnore: "preview-environment.spec.ts",
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
    ...(authenticatedRegressionEnabled
      ? [
          {
            name: "clerk-setup",
            testMatch: /clerk\.setup\.ts$/,
          },
          {
            name: "authenticated-dashboard",
            testMatch: /dashboard-authenticated\.spec\.ts$/,
            dependencies: ["clerk-setup"],
            use: { ...devices["Desktop Chrome"] },
          },
        ]
      : []),
    {
      name: "chromium",
      testIgnore: [
        "preview-environment.spec.ts",
        "localization.spec.ts",
        /clerk\.setup\.ts$/,
        /dashboard-authenticated\.spec\.ts$/,
      ],
      ...(authenticatedRegressionEnabled
        ? { dependencies: ["authenticated-dashboard"] }
        : {}),
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
