import { expect, test } from "@playwright/test";

const routes = [
  {
    path: "/",
    locale: "en",
    marker: "Post to every social platform with one API",
    canonical: "https://unipost.dev/",
  },
  {
    path: "/es",
    locale: "es",
    marker: "Publica en todas las redes sociales con una sola API",
    canonical: "https://unipost.dev/es",
  },
  {
    path: "/pricing",
    locale: "en",
    marker: "Start free.",
    canonical: "https://unipost.dev/pricing",
  },
  {
    path: "/es/pricing",
    locale: "es",
    marker: "Empieza gratis.",
    canonical: "https://unipost.dev/es/pricing",
  },
] as const;

test.describe("localized public conversion path", () => {
  test.beforeEach(async ({ page }) => {
    const bypassSecret = process.env.VERCEL_AUTOMATION_BYPASS_SECRET;
    if (!bypassSecret) return;

    await page.request.get("/", {
      headers: {
        "x-vercel-protection-bypass": bypassSecret,
        "x-vercel-set-bypass-cookie": "true",
      },
    });
  });

  for (const route of routes) {
    test(`${route.path} renders localized copy and SEO`, async ({ page }) => {
      await page.goto(route.path, { waitUntil: "networkidle" });

      await expect(page.locator("html")).toHaveAttribute("lang", route.locale);
      await expect(page.getByText(route.marker).first()).toBeVisible();
      const canonical = await page.locator('link[rel="canonical"]').getAttribute("href");
      expect(canonical).not.toBeNull();
      expect(new URL(canonical!).href).toBe(new URL(route.canonical).href);
      await expect(page.locator('link[rel="alternate"][hreflang="en"]')).toHaveCount(1);
      await expect(page.locator('link[rel="alternate"][hreflang="es"]')).toHaveCount(1);
      await expect(page.locator('link[rel="alternate"][hreflang="x-default"]')).toHaveCount(1);

      const navText = await page.locator(".mk-nav-links").innerText();
      expect(navText).not.toMatch(/🇺🇸|🇪🇸/);
    });
  }

  test("selector is after Developer, keyboard accessible, and persists English", async ({ page }) => {
    await page.goto("/es/pricing", { waitUntil: "networkidle" });

    const developer = page.locator(".mk-nav-dropdown-trigger");
    const selector = page.getByRole("button", { name: "Elegir idioma" });
    await expect(selector).toBeVisible();
    expect(
      await developer.evaluate((node) =>
        Boolean(
          node.compareDocumentPosition(document.querySelector(".mk-language-trigger")!) &
            Node.DOCUMENT_POSITION_FOLLOWING,
        ),
      ),
    ).toBe(true);

    await selector.focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("menuitem", { name: "English" })).toBeVisible();
    await expect(page.getByRole("menuitem", { name: "Español" })).toHaveAttribute("aria-current", "true");
    await page.getByRole("menuitem", { name: "English" }).click();

    await expect(page).toHaveURL(/\/pricing$/);
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await expect(page.getByText("Start free.").first()).toBeVisible();
    const localeCookie = (await page.context().cookies()).find((cookie) => cookie.name === "unipost_locale");
    expect(localeCookie?.value).toBe("en");
  });

  test("private API paths on the landing host remain routed to the dashboard", async ({ page }) => {
    const response = await page.request.get("/api/private", { maxRedirects: 0 });

    expect(response.status()).toBe(307);
    expect(new URL(response.headers().location).hostname).toBe("app.unipost.dev");
  });

  for (const path of ["/vi", "/vi/pricing", "/fr", "/zh-cn/pricing"]) {
    test(`${path} returns 404 while the locale is unsupported`, async ({ page }) => {
      const response = await page.request.get(path, { maxRedirects: 0 });

      expect(response.status()).toBe(404);
    });
  }

  for (const route of routes) {
    test(`${route.path} has no mobile horizontal overflow`, async ({ page }) => {
      await page.setViewportSize({ width: 390, height: 844 });
      await page.goto(route.path, { waitUntil: "domcontentloaded" });
      const dimensions = await page.evaluate(() => ({
        clientWidth: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
      }));
      expect(dimensions.scrollWidth).toBeLessThanOrEqual(dimensions.clientWidth + 2);
    });
  }
});
