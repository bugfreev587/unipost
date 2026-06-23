import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, it } from "node:test";

const root = new URL("..", import.meta.url).pathname;

function read(path) {
  return readFileSync(join(root, path), "utf8");
}

function firstMeaningfulLine(source) {
  return source
    .split(/\r?\n/)
    .map((line) => line.trim())
    .find((line) => line && !line.startsWith("//")) || "";
}

describe("blog pages inherit the global light and dark theme", () => {
  it("scopes blog colors to the global theme classes instead of forcing dark mode", () => {
    const source = read("src/app/blog/layout.tsx");

    assert.doesNotMatch(source, /:root\{--blog-bg:#/);
    assert.doesNotMatch(source, /body\{background:var\(--blog-bg\)/);
    assert.doesNotMatch(source, /\.mk-nav\{background:#/);
    assert.match(source, /\.blog-shell\{[^}]*--blog-bg:var\(--app-bg\)/s);
    assert.match(source, /\.light \.blog-shell\{/);
    assert.match(source, /\.dark \.blog-shell\{/);
  });
});

describe("public commercial pages expose SEO metadata from server routes", () => {
  const staticRoutes = [
    {
      name: "pricing",
      path: "src/app/pricing/page.tsx",
      titleNeedle: "UniPost Pricing",
    },
    {
      name: "compare",
      path: "src/app/compare/page.tsx",
      titleNeedle: "Compare Social Media APIs",
    },
    {
      name: "solutions",
      path: "src/app/solutions/page.tsx",
      titleNeedle: "Social Media API Solutions",
    },
  ];

  for (const route of staticRoutes) {
    it(`${route.name} page is a server route with metadata`, () => {
      const source = read(route.path);
      assert.notEqual(firstMeaningfulLine(source), '"use client";');
      assert.match(source, /export const metadata\s*:\s*Metadata\s*=/);
      assert.match(source, new RegExp(route.titleNeedle));
      assert.match(source, /alternates:\s*{\s*canonical:/s);
      assert.match(source, /openGraph:\s*{/);
    });
  }

  it("competitor alternatives are static server routes with generated metadata", () => {
    const source = read("src/app/alternatives/[competitor]/page.tsx");
    assert.notEqual(firstMeaningfulLine(source), '"use client";');
    assert.match(source, /export function generateStaticParams/);
    assert.match(source, /export (async )?function generateMetadata/);
    assert.match(source, /canonical:/);
    assert.match(source, /openGraph:/);
    assert.doesNotMatch(source, /useParams/);
  });
});

describe("P1 money and solution pages are crawlable server routes", () => {
  const moneyRoutes = [
    {
      path: "src/app/social-media-api/page.tsx",
      canonical: "https://unipost.dev/social-media-api",
      titleNeedle: "Unified Social Media API",
    },
    {
      path: "src/app/social-media-posting-api/page.tsx",
      canonical: "https://unipost.dev/social-media-posting-api",
      titleNeedle: "Social Media Posting API",
    },
    {
      path: "src/app/social-media-publishing-api/page.tsx",
      canonical: "https://unipost.dev/social-media-publishing-api",
      titleNeedle: "Social Media Publishing API",
    },
  ];

  for (const route of moneyRoutes) {
    it(`${route.canonical} exposes metadata and JSON-LD`, () => {
      assert.equal(existsSync(join(root, route.path)), true);
      const source = read(route.path);
      assert.notEqual(firstMeaningfulLine(source), '"use client";');
      assert.match(source, /export const metadata\s*:\s*Metadata\s*=/);
      assert.match(source, new RegExp(route.titleNeedle));
      assert.match(source, new RegExp(route.canonical.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
      assert.match(source, /application\/ld\+json/);
      assert.match(source, /FAQPage/);
      assert.match(source, /SoftwareApplication/);
      assert.match(source, /BreadcrumbList/);
    });
  }

  it("money page data includes concrete API, OAuth, media, webhook, constraints, and CTAs", () => {
    const source = read("src/data/seo-growth-pages.ts");
    for (const slug of ["social-media-api", "social-media-posting-api", "social-media-publishing-api"]) {
      assert.match(source, new RegExp(`slug: "${slug}"`));
    }
    for (const needle of [
      "POST /v1/posts",
      "account_ids",
      "OAuth",
      "media",
      "webhooks",
      "platform constraints",
      "/docs/api/posts/create",
      "/pricing",
    ]) {
      assert.match(source, new RegExp(needle));
    }
  });

  const solutionSlugs = [
    "social-media-scheduler-api",
    "ai-agent-social-posting",
    "saas-social-publishing",
    "white-label-social-media-api",
  ];

  for (const slug of solutionSlugs) {
    it(`/solutions/${slug} exposes metadata and workflow content`, () => {
      const routePath = `src/app/solutions/${slug}/page.tsx`;
      assert.equal(existsSync(join(root, routePath)), true);
      const source = read(routePath);
      assert.notEqual(firstMeaningfulLine(source), '"use client";');
      assert.match(source, /export const metadata\s*:\s*Metadata\s*=/);
      assert.match(source, new RegExp(`https://unipost.dev/solutions/${slug}`));
      assert.match(source, /application\/ld\+json/);
      assert.match(source, /FAQPage/);
      assert.match(source, /BreadcrumbList/);
    });
  }

  it("solution data maps workflows to UniPost primitives", () => {
    const source = read("src/data/seo-growth-pages.ts");
    for (const needle of [
      "Connect customer accounts",
      "Upload or reference media",
      "Publish or schedule posts",
      "Track results",
      "Handle webhooks and errors",
      "platform-specific differences",
    ]) {
      assert.match(source, new RegExp(needle));
    }
  });
});

describe("comparison and original GEO assets are explicit", () => {
  it("best social media APIs comparison page targets category evaluation", () => {
    const source = read("src/app/compare/social-media-apis/page.tsx");
    assert.notEqual(firstMeaningfulLine(source), '"use client";');
    assert.match(source, /export const metadata\s*:\s*Metadata\s*=/);
    assert.match(source, /Best Unified Social Media APIs/);
    assert.match(source, /https:\/\/unipost\.dev\/compare\/social-media-apis/);
    assert.match(source, /application\/ld\+json/);
    assert.match(source, /ItemList/);
    assert.match(source, /FAQPage/);
  });

  it("competitor data includes best-fit and source discipline", () => {
    const files = [
      ["src/data/competitors/postforme.ts", ["Last verified: 2026-06-23", "sourceLinks", "bestFit", "about", "open-source", "$10/mo"]],
      ["src/data/competitors/zernio.ts", ["Last verified: 2026-06-23", "sourceLinks", "bestFit", "connected social account"]],
      ["src/data/competitors/ayrshare.ts", ["Last verified: 2026-06-23", "sourceLinks", "bestFit", "profile"]],
    ];

    for (const [file, needles] of files) {
      const source = read(file);
      for (const needle of needles) {
        assert.match(source, new RegExp(needle.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
      }
    }

    const client = read("src/app/alternatives/[competitor]/alternative-page-client.tsx");
    assert.match(client, /bestFit/);
    assert.match(client, /Best fit/);
    assert.match(client, /Source notes/);
  });

  it("resource pages cover the original GEO asset set", () => {
    const route = read("src/app/resources/[slug]/page.tsx");
    assert.notEqual(firstMeaningfulLine(route), '"use client";');
    assert.match(route, /export function generateStaticParams/);
    assert.match(route, /export (async )?function generateMetadata/);
    assert.match(route, /application\/ld\+json/);
    assert.match(route, /BreadcrumbList/);

    const source = read("src/data/seo-resources.ts");
    for (const slug of [
      "social-media-api-platform-requirements",
      "platform-posting-constraints",
      "social-media-oauth-app-review",
      "media-upload-limits",
      "unified-api-cost-calculator",
    ]) {
      assert.match(source, new RegExp(`slug: "${slug}"`));
    }
  });

  it("ads, measurement, and distribution execution doc exists", () => {
    const source = read("../docs/seo-geo-search-execution.md");
    for (const needle of [
      "Campaign 1: Unified Social Media API",
      "negative keyword",
      "utm_campaign",
      "signup count by landing page",
      "developer directories",
      "external account required",
    ]) {
      assert.match(source, new RegExp(needle));
    }
  });
});

describe("crawl surfaces are explicit", () => {
  it("robots.txt is public and points at the sitemap", () => {
    assert.equal(existsSync(join(root, "src/app/robots.ts")), true);
    const robots = read("src/app/robots.ts");
    assert.match(robots, /MetadataRoute\.Robots/);
    assert.match(robots, /userAgent:\s*"\*"/);
    assert.match(robots, /allow:\s*"\/"/);
    assert.match(robots, /https:\/\/unipost\.dev\/sitemap\.xml/);

    const proxy = read("src/proxy.ts");
    assert.match(proxy, /pathname === "\/robots\.txt"/);
  });

  it("sitemap includes existing commercial and platform pages", () => {
    const sitemap = read("src/app/sitemap.ts");
    assert.match(sitemap, /"pinterest"/);
    assert.match(sitemap, /"\/about"/);
    assert.match(sitemap, /"\/compare"/);
    assert.match(sitemap, /\/alternatives\/\$\{slug\}/);
    assert.match(sitemap, /MONEY_PAGES/);
    assert.match(sitemap, /SOLUTION_PAGES/);
    assert.match(sitemap, /SEO_RESOURCES/);
    assert.match(sitemap, /\/social-media-api/);
    assert.match(sitemap, /\/compare\/social-media-apis/);
  });

  it("proxy keeps SEO subpaths public on the landing domain", () => {
    const proxy = read("src/proxy.ts");
    assert.match(proxy, /pathname\.startsWith\("\/solutions"\)/);
    assert.match(proxy, /pathname\.startsWith\("\/compare"\)/);
    assert.match(proxy, /pathname\.startsWith\("\/resources"\)/);
  });
});

describe("homepage and about page carry entity SEO intent", () => {
  it("homepage metadata owns brand plus one-api positioning", () => {
    const source = read("src/app/marketing/page.tsx");
    assert.match(source, /UniPost \| Unified Social Media Posting API for Developers/);
    assert.match(source, /Post to every social platform with one API/);
    assert.match(source, /openGraph:\s*{/);
  });

  it("about page exists with entity metadata and structured data", () => {
    assert.equal(existsSync(join(root, "src/app/about/page.tsx")), true);
    const source = read("src/app/about/page.tsx");
    assert.match(source, /About UniPost \| Unified Social Media API for Developers/);
    assert.match(source, /A developer-first social media publishing API/);
    assert.match(source, /application\/ld\+json/);
    assert.match(source, /Organization/);
    assert.match(source, /SoftwareApplication/);
    assert.match(source, /BreadcrumbList/);
  });
});
