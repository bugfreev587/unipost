import { expect, test, type APIRequestContext } from "@playwright/test";

const dashboardBaseURL = process.env.DASHBOARD_BASE_URL;
const automationBypassSecret =
  process.env.VERCEL_AUTOMATION_BYPASS_SECRET;

if (!dashboardBaseURL || !automationBypassSecret) {
  throw new Error(
    "DASHBOARD_BASE_URL and VERCEL_AUTOMATION_BYPASS_SECRET are required",
  );
}

const bypassHeaders = {
  "x-vercel-protection-bypass": automationBypassSecret,
};
const productionOrigin = "https://unipost.dev";
const expectedTitle = "UniPost | Social Media Posting API for Developers";
const expectedDescription =
  "UniPost gives developers one API to connect customer social accounts, upload media, schedule posts, and publish across major social platforms.";

function normalizePath(pathname: string): string {
  if (pathname === "/") {
    return pathname;
  }
  return pathname.replace(/\/+$/, "");
}

function canonicalFromHTML(html: string): string | null {
  return html.match(
    /<link[^>]+rel=["']canonical["'][^>]+href=["']([^"']+)["']/i,
  )?.[1] ??
    html.match(
      /<link[^>]+href=["']([^"']+)["'][^>]+rel=["']canonical["']/i,
    )?.[1] ??
    null;
}

function robotsFromHTML(html: string): string | null {
  return html.match(
    /<meta[^>]+name=["']robots["'][^>]+content=["']([^"']+)["']/i,
  )?.[1] ??
    html.match(
      /<meta[^>]+content=["']([^"']+)["'][^>]+name=["']robots["']/i,
    )?.[1] ??
    null;
}

async function fetchPreviewRoute(
  request: APIRequestContext,
  pathname: string,
) {
  return request.get(pathname, {
    headers: bypassHeaders,
    maxRedirects: 0,
  });
}

test("homepage renders the protected developer API metadata", async ({ page }) => {
  await page.setExtraHTTPHeaders(bypassHeaders);
  await page.goto("/", { waitUntil: "domcontentloaded" });

  await expect(page).toHaveTitle(expectedTitle);
  await expect(page.locator('meta[name="description"]')).toHaveAttribute(
    "content",
    expectedDescription,
  );
  const canonical = await page
    .locator('link[rel="canonical"]')
    .getAttribute("href");
  expect(canonical).not.toBeNull();
  expect(new URL(canonical!).href).toBe(new URL(productionOrigin).href);
  await expect(page.locator('meta[property="og:title"]')).toHaveAttribute(
    "content",
    expectedTitle,
  );
  await expect(page.locator('meta[property="og:description"]')).toHaveAttribute(
    "content",
    expectedDescription,
  );
  await expect(page.locator('meta[name="twitter:card"]')).toHaveAttribute(
    "content",
    "summary",
  );
  await expect(page.locator('meta[name="twitter:title"]')).toHaveAttribute(
    "content",
    expectedTitle,
  );
  await expect(page.locator('meta[name="twitter:description"]')).toHaveAttribute(
    "content",
    expectedDescription,
  );
});

test("every deployed sitemap entry is directly indexable", async ({ request }) => {
  const sitemapResponse = await fetchPreviewRoute(request, "/sitemap.xml");
  expect(sitemapResponse.status()).toBe(200);
  expect(sitemapResponse.headers()["content-type"]).toContain("xml");

  const xml = await sitemapResponse.text();
  const sitemapURLs = [...xml.matchAll(/<loc>([^<]+)<\/loc>/g)].map(
    (match) => match[1],
  );
  expect(sitemapURLs.length).toBeGreaterThan(0);
  expect(new Set(sitemapURLs).size).toBe(sitemapURLs.length);

  for (let index = 0; index < sitemapURLs.length; index += 8) {
    const batch = sitemapURLs.slice(index, index + 8);
    const results = await Promise.all(
      batch.map(async (sitemapURL) => {
        const productionURL = new URL(sitemapURL);
        expect(productionURL.origin, sitemapURL).toBe(productionOrigin);

        const response = await fetchPreviewRoute(
          request,
          `${productionURL.pathname}${productionURL.search}`,
        );
        const html = await response.text();
        return {
          sitemapURL,
          productionURL,
          status: response.status(),
          html,
        };
      }),
    );

    for (const result of results) {
      expect(result.status, result.sitemapURL).toBe(200);
      expect(
        robotsFromHTML(result.html)?.toLowerCase() ?? "",
        result.sitemapURL,
      ).not.toContain("noindex");

      const canonical = canonicalFromHTML(result.html);
      if (canonical) {
        const canonicalURL = new URL(canonical);
        expect(canonicalURL.origin, result.sitemapURL).toBe(productionOrigin);
        expect(
          normalizePath(canonicalURL.pathname),
          result.sitemapURL,
        ).toBe(normalizePath(result.productionURL.pathname));
      }
    }
  }
});
