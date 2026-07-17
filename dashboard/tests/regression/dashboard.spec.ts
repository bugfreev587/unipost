import { expect, test, type Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
import path from "node:path";

const testEmail = process.env.DASHBOARD_TEST_EMAIL;
const testPassword = process.env.DASHBOARD_TEST_PASSWORD;
const configuredProfileId = process.env.DASHBOARD_TEST_PROFILE_ID;

const publicRoutes = [
  { path: "/docs", marker: /UniPost|Dashboard|API/i },
  { path: "/docs/guides", marker: /Task guides|Analytics guides/i },
  { path: "/docs/guides/analytics", marker: /Analytics Guides|Unified-first Analytics/i },
  { path: "/docs/guides/analytics/tiktok-followers", marker: /Get TikTok followers|user\.info\.stats/i },
  { path: "/docs/guides/x/credits", marker: /Plan and monitor X Credits|Estimate the operation mix/i },
  { path: "/pricing", marker: /Start free|Compare plans/i },
  { path: "/tools", marker: /TikTok Analytics|YouTube Analytics|Instagram Analytics|Threads Analytics|Pinterest Analytics/i },
  { path: "/tools/tiktok-analytics", marker: /TikTok Analytics/i },
  { path: "/tools/youtube-analytics", marker: /YouTube Analytics/i },
  { path: "/tools/instagram-analytics", marker: /Instagram Analytics/i },
  { path: "/tools/threads-analytics", marker: /Threads Analytics/i },
  { path: "/tools/pinterest-analytics", marker: /Pinterest Analytics/i },
  { path: "/docs/api", marker: /List analytics posts|Export analytics posts/i },
  { path: "/docs/api/x-credits", marker: /X Credits|GET\s+\/v1\/billing\/x-credits/i },
  { path: "/docs/api/analytics/posts-list", marker: /List analytics posts|GET\s+\/v1\/analytics\/posts/i },
  { path: "/docs/api/analytics/posts/export", marker: /Export analytics posts|GET\s+\/v1\/analytics\/posts\/export/i },
  { path: "/docs/api/analytics/rollup", marker: /Analytics rollup|GET\s+\/v1\/analytics\/rollup/i },
  { path: "/docs/api/analytics/platforms", marker: /Platform capabilities|GET\s+\/v1\/analytics\/platforms/i },
  { path: "/docs/api/analytics/platforms/detail", marker: /Get platform summary|GET\s+\/v1\/analytics\/platforms\/\{platform\}/i },
  { path: "/docs/api/analytics/youtube", marker: /YouTube Analytics|yt-analytics\.readonly/i },
  { path: "/docs/api/analytics/youtube/summary", marker: /Get YouTube analytics summary|GET\s+\/v1\/accounts\/:account_id\/youtube\/analytics\/summary/i },
  { path: "/docs/api/analytics/youtube/trend", marker: /Get YouTube analytics trend|GET\s+\/v1\/accounts\/:account_id\/youtube\/analytics\/trend/i },
  { path: "/docs/api/analytics/youtube/videos", marker: /Get YouTube analytics top videos|GET\s+\/v1\/accounts\/:account_id\/youtube\/analytics\/videos/i },
  { path: "/docs/api/analytics/refresh", marker: /Request analytics refresh|POST\s+\/v1\/analytics\/refresh/i },
  { path: "/docs/api/api-metrics", marker: /API Metrics|GET\s+\/v1\/api-metrics\/overall/i },
  { path: "/docs/api/api-metrics/overall", marker: /Overall|GET\s+\/v1\/api-metrics\/overall/i },
  { path: "/docs/api/api-metrics/summary", marker: /Summary|GET\s+\/v1\/api-metrics\/summary/i },
  { path: "/docs/api/api-metrics/trend", marker: /Trend|GET\s+\/v1\/api-metrics\/trend/i },
  { path: "/docs/api/api-metrics/status-codes", marker: /Status-Code|GET\s+\/v1\/api-metrics\/status-codes/i },
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
    await expect(page.getByRole("link", { name: /YouTube Analytics/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /Instagram Analytics/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /Threads Analytics/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /Pinterest Analytics/i })).toBeVisible();
    await expect(page.getByText(/Coming Soon/i)).toHaveCount(0);
    await expect(page.getByText(/Thread Splitter|Caption Generator/i)).toHaveCount(0);
  });

  test("API Metrics docs stay wired into the API docs shell", async () => {
    const docsShellSource = await readFile(path.join(process.cwd(), "src/app/docs/_components/docs-shell.tsx"), "utf8");
    const apiMetricsPageSource = await readFile(path.join(process.cwd(), "src/app/docs/api/api-metrics/page.tsx"), "utf8");

    expect(docsShellSource).toContain('label: "API Metrics",');
    expect(docsShellSource).toContain('label: "Overall", href: "/docs/api/api-metrics/overall", method: "GET"');
    expect(docsShellSource).toContain('label: "Summary", href: "/docs/api/api-metrics/summary", method: "GET"');
    expect(docsShellSource).toContain('label: "Trend", href: "/docs/api/api-metrics/trend", method: "GET"');
    expect(docsShellSource).toContain('label: "Status-Code", href: "/docs/api/api-metrics/status-codes", method: "GET"');
    expect(docsShellSource).not.toContain('label: "API Metrics", href: "/docs/api/api-metrics", method: "GET"');
    expect(apiMetricsPageSource).toContain('redirect("/docs/api/api-metrics/overall")');
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

test.describe("admin errors details drawer", () => {
  test("lets admins open a failure detail drawer with copyable raw data", async () => {
    const pageSource = await readFile(path.join(process.cwd(), "src/app/admin/errors/page.tsx"), "utf8");

    expect(pageSource).toContain("openFailureDetail");
    expect(pageSource).toContain('role="dialog"');
    expect(pageSource).toContain("Error details");
    expect(pageSource).toContain("Raw Data");
    expect(pageSource).toContain("copyRawFailure");
    expect(pageSource).toContain("navigator.clipboard.writeText");
  });

  test("uses URL filters as the initial failure query", async () => {
    const pageSource = await readFile(path.join(process.cwd(), "src/app/admin/errors/page.tsx"), "utf8");

    expect(pageSource).toContain("useState(() => initialFiltersFromURL())");
    expect(pageSource).toContain("useState(initialFilters.search)");
    expect(pageSource).toContain("useState(initialFilters.platform)");
    expect(pageSource).toContain("useState(initialFilters.source)");
    expect(pageSource).toContain("useState<FailureRange>(initialFilters.range)");
    expect(pageSource).toContain("useState(initialFilters.userId)");
    expect(pageSource).toContain('params.get("period") === "this_month"');
    expect(pageSource).toContain('params.get("user_id")');
    expect(pageSource).not.toContain("const initial = initialFiltersFromURL();");
  });
});

test.describe("admin email notifications", () => {
  test("wires quota reminder email notifications into admin navigation and API client", async () => {
    const navSource = await readFile(path.join(process.cwd(), "src/app/admin/_components/admin-ui.tsx"), "utf8");
    const pageSource = await readFile(path.join(process.cwd(), "src/app/admin/email/page.tsx"), "utf8");
    const apiSource = await readFile(path.join(process.cwd(), "src/lib/api.ts"), "utf8");

    expect(navSource).toContain('label: "Email"');
    expect(navSource).toContain('href: "/admin/email"');
    expect(pageSource).toContain("listAdminEmailNotifications");
    expect(pageSource).toContain("trigger_event");
    expect(pageSource).toContain('fieldKey="admin.email.search"');
    expect(apiSource).toContain("AdminEmailNotificationRow");
    expect(apiSource).toContain('| "admin.email.search"');
    expect(apiSource).toContain("/v1/admin/email-notifications");
  });
});

test.describe("authenticated dashboard smoke", () => {
  test.skip(!testEmail || !testPassword, "Set DASHBOARD_TEST_EMAIL and DASHBOARD_TEST_PASSWORD to enable authenticated dashboard regression.");

  test("core dashboard routes load and preserve plan-gated navigation", async ({ page }) => {
    await signIn(page, testEmail!, testPassword!);
    const profileId = configuredProfileId || await resolveProfileId(page);

    await expectDashboardRoute(page, `/projects/${profileId}`);
    await expectDashboardRoute(page, `/projects/${profileId}/accounts`);
    await expectDashboardRoute(page, `/projects/${profileId}/posts`);
    await expectDashboardRoute(page, `/projects/${profileId}/analytics`);
    await expectDashboardRoute(page, `/projects/${profileId}/settings`);

    await page.goto(`/projects/${profileId}/analytics/platforms/tiktok`, { waitUntil: "networkidle" });
    await expect(page.getByText("TikTok Analytics").first()).toBeVisible();

    await page.goto(`/projects/${profileId}/analytics/platforms/youtube`, { waitUntil: "networkidle" });
    await expect(page.getByText("YouTube Analytics").first()).toBeVisible();
  });
});

async function signIn(page: Page, email: string, password: string) {
  await page.goto("/", { waitUntil: "domcontentloaded" });
  if (!page.url().includes("clerk") && !page.url().includes("sign-in")) {
    await expect(page.getByText(/Navigate|Profiles|Posts|Dashboard/i).first()).toBeVisible();
    return;
  }

  await page.getByLabel(/email/i).fill(email);
  const continueButton = page.locator('button[data-localization-key="formButtonPrimary"]');
  if (await continueButton.isVisible().catch(() => false)) {
    await continueButton.click();
  }
  await page.locator('input[type="password"]').fill(password);
  await page.locator('button[data-localization-key="formButtonPrimary"]').click();
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
