import { expect, test, type Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
import path from "node:path";

const testEmail = process.env.DASHBOARD_TEST_EMAIL;
const testPassword = process.env.DASHBOARD_TEST_PASSWORD;
const configuredProfileId = process.env.DASHBOARD_TEST_PROFILE_ID;

const publicRoutes = [
  { path: "/docs", marker: /UniPost|Dashboard|API/i },
  { path: "/pricing", marker: /Free|Basic|Growth|Team/i },
  { path: "/tools", marker: /TikTok Analytics|Instagram Analytics|Threads Analytics|Pinterest Analytics/i },
  { path: "/tools/tiktok-analytics", marker: /TikTok Analytics/i },
  { path: "/tools/instagram-analytics", marker: /Instagram Analytics/i },
  { path: "/tools/threads-analytics", marker: /Threads Analytics/i },
  { path: "/tools/pinterest-analytics", marker: /Pinterest Analytics/i },
  { path: "/docs/api", marker: /List analytics posts|Export analytics posts/i },
  { path: "/docs/api/analytics/posts-list", marker: /List analytics posts|GET\s+\/v1\/analytics\/posts/i },
  { path: "/docs/api/analytics/posts/export", marker: /Export analytics posts|GET\s+\/v1\/analytics\/posts\/export/i },
  { path: "/docs/api/analytics/rollup", marker: /Analytics rollup|GET\s+\/v1\/analytics\/rollup/i },
  { path: "/docs/api/analytics/platforms", marker: /Analytics platforms|GET\s+\/v1\/analytics\/platforms/i },
  { path: "/docs/api/analytics/platforms/detail", marker: /Get analytics platform|GET\s+\/v1\/analytics\/platforms\/\{platform\}/i },
  { path: "/docs/api/analytics/refresh", marker: /Request analytics refresh|POST\s+\/v1\/analytics\/refresh/i },
  { path: "/docs/api/api-metrics", marker: /API Metrics|GET\s+\/v1\/api-metrics\/overall/i },
];

test.describe("public dashboard surfaces", () => {
  for (const route of publicRoutes) {
    test(`${route.path} loads without server errors`, async ({ page }) => {
      const serverErrors: string[] = [];
      page.on("response", (response) => {
        if (response.status() >= 500) {
          serverErrors.push(`${response.status()} ${response.url()}`);
        }
      });

      await page.goto(route.path, { waitUntil: "domcontentloaded" });
      const surface = route.path.startsWith("/docs") ? page.locator("article").first() : page;
      await expect(surface.getByText(route.marker).first()).toBeVisible();
      expect(serverErrors).toEqual([]);
    });
  }

  test("/tools only shows live tools", async ({ page }) => {
    await page.goto("/tools", { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("link", { name: /TikTok Analytics/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /Instagram Analytics/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /Threads Analytics/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /Pinterest Analytics/i })).toBeVisible();
    await expect(page.getByText(/Coming Soon/i)).toHaveCount(0);
    await expect(page.getByText(/Thread Splitter|Caption Generator/i)).toHaveCount(0);
  });

  test("API Metrics docs stay wired into the API docs shell", async () => {
    const docsShellSource = await readFile(path.join(process.cwd(), "src/app/docs/_components/docs-shell.tsx"), "utf8");
    const apiMetricsPageSource = await readFile(path.join(process.cwd(), "src/app/docs/api/api-metrics/page.tsx"), "utf8");

    expect(docsShellSource).toContain('label: "API Metrics", href: "/docs/api/api-metrics", method: "GET"');
    expect(apiMetricsPageSource).toContain("<SingleEndpointReferencePage");
  });
});

test.describe("workspace-scoped developer routes", () => {
  const workspaceScopedPages = [
    "src/app/(dashboard)/projects/[id]/api-keys/page.tsx",
    "src/app/(dashboard)/projects/[id]/analytics/api/page.tsx",
    "src/app/(dashboard)/projects/[id]/credentials/page.tsx",
    "src/app/(dashboard)/projects/[id]/webhooks/page.tsx",
  ];

  for (const filePath of workspaceScopedPages) {
    test(`${filePath} does not block on profile lookup`, async () => {
      const source = await readFile(path.join(process.cwd(), filePath), "utf8");

      expect(source).not.toContain("useWorkspaceId");
      expect(source).not.toContain("getProfile(");
    });
  }
});

test.describe("platform credentials layout", () => {
  test("save action uses a compact button column", async () => {
    const pageSource = await readFile(path.join(process.cwd(), "src/app/(dashboard)/projects/[id]/credentials/page.tsx"), "utf8");
    const globalCss = await readFile(path.join(process.cwd(), "src/app/globals.css"), "utf8");

    expect(pageSource).toContain('className="platform-credential-form"');
    expect(pageSource).toContain('className="dbtn dbtn-primary platform-credential-save"');
    expect(globalCss).toMatch(/\.platform-credential-form\s*{[^}]*grid-template-columns:\s*minmax\(220px,\s*1fr\)\s+minmax\(220px,\s*1fr\)\s+max-content;/s);
    expect(globalCss).toMatch(/\.platform-credential-save\s*{[^}]*justify-self:\s*start;[^}]*min-width:\s*96px;/s);
  });
});

test.describe("admin AI keys layout", () => {
  test("uses a focused provider credential card instead of the dense status table", async () => {
    const pageSource = await readFile(path.join(process.cwd(), "src/app/admin/ai-keys/page.tsx"), "utf8");

    expect(pageSource).toContain('className="ai-hero-kicker"');
    expect(pageSource).toContain('className="ai-credential-card"');
    expect(pageSource).toContain('className="ai-provider-card"');
    expect(pageSource).toContain("Secrets stay server-side");
    expect(pageSource).not.toContain("Provider status");
  });
});

test.describe("authenticated dashboard smoke", () => {
  test.skip(!testEmail || !testPassword, "Set DASHBOARD_TEST_EMAIL and DASHBOARD_TEST_PASSWORD to enable authenticated dashboard regression.");

  test("core dashboard routes load and preserve feature-flag gating", async ({ page }) => {
    await signIn(page, testEmail!, testPassword!);
    const profileId = configuredProfileId || await resolveProfileId(page);

    await expectDashboardRoute(page, `/projects/${profileId}`);
    await expectDashboardRoute(page, `/projects/${profileId}/accounts`);
    await expectDashboardRoute(page, `/projects/${profileId}/posts`);
    await expectDashboardRoute(page, `/projects/${profileId}/analytics`);
    await expectDashboardRoute(page, `/projects/${profileId}/settings`);

    await page.goto(`/projects/${profileId}/analytics/platforms/tiktok`, { waitUntil: "networkidle" });
    await expect(page.getByText(/TikTok Analytics|TikTok analytics is disabled/).first()).toBeVisible();
  });
});

async function signIn(page: Page, email: string, password: string) {
  await page.goto("/", { waitUntil: "domcontentloaded" });
  if (!page.url().includes("clerk") && !page.url().includes("sign-in")) {
    await expect(page.getByText(/Navigate|Profiles|Posts|Dashboard/i).first()).toBeVisible();
    return;
  }

  await page.getByLabel(/email/i).fill(email);
  const continueButton = page.getByRole("button", { name: /continue|next/i });
  if (await continueButton.isVisible().catch(() => false)) {
    await continueButton.click();
  }
  await page.getByLabel(/password/i).fill(password);
  await page.getByRole("button", { name: /continue|sign in|log in/i }).click();
  await page.waitForURL((url) => !url.hostname.includes("clerk") && !url.pathname.includes("sign-in"), { timeout: 30_000 });
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

async function expectDashboardRoute(page: Page, path: string) {
  const failedRequests: string[] = [];
  page.on("response", (response) => {
    if (response.status() >= 500) {
      failedRequests.push(`${response.status()} ${response.url()}`);
    }
  });

  await page.goto(path, { waitUntil: "networkidle" });
  await expect(page.locator("body")).toContainText(/Navigate|Settings|Posts|Analytics|Connections|Profiles/);
  expect(failedRequests).toEqual([]);
}
