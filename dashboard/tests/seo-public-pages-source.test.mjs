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
  it("homepage metadata protects the developer API search intent", () => {
    const source = read("src/app/marketing/page.tsx");
    assert.match(
      source,
      /const HOMEPAGE_TITLE = "UniPost \| Social Media Posting API for Developers"/,
    );
    assert.match(
      source,
      /UniPost gives developers one API to connect customer social accounts, upload media, schedule posts, and publish across major social platforms\./,
    );
    assert.doesNotMatch(source, /const HOMEPAGE_TITLE = "Unipost"/);
    assert.doesNotMatch(
      source,
      /const HOMEPAGE_TITLE = "Rewrite homepage title and meta description for query relevance"/,
    );
    assert.match(
      source,
      /const HOMEPAGE_URL = "https:\/\/unipost\.dev\/"/,
    );
    assert.match(source, /alternates:\s*{[^}]*canonical:\s*HOMEPAGE_URL[^}]*}/);
    assert.match(
      source,
      /openGraph:\s*{[^}]*title:\s*HOMEPAGE_TITLE,[^}]*description:\s*HOMEPAGE_DESCRIPTION,[^}]*url:\s*HOMEPAGE_URL,[^}]*}/,
    );
    assert.match(
      source,
      /twitter:\s*{[^}]*card:\s*"summary",[^}]*title:\s*HOMEPAGE_TITLE,[^}]*description:\s*HOMEPAGE_DESCRIPTION,[^}]*}/,
    );
    assert.match(source, /Post to every social platform with one API/);
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

  it("about page platform grid renders official platform icons before labels", () => {
    const source = read("src/app/about/page.tsx");
    assert.match(source, /import \{ PlatformIcon \} from "@\/components\/platform-icons"/);
    assert.match(source, /const ABOUT_PLATFORMS = \[/);
    for (const platform of ["twitter", "linkedin", "instagram", "tiktok", "threads", "youtube", "facebook", "pinterest", "bluesky"]) {
      assert.match(source, new RegExp(`platform: "${platform}"`));
    }
    assert.match(source, /<PlatformIcon platform=\{platform\.platform\}/);
    assert.match(source, /<span className="about-platform-name">\{platform\.name\}<\/span>/);
  });
});

describe("audited public routes expose self-referencing canonicals", () => {
  const platformRoutes = [
    ["src/app/(platforms)/bluesky-api/page.tsx", "bluesky"],
    ["src/app/(platforms)/instagram-api/page.tsx", "instagram"],
    ["src/app/(platforms)/linkedin-api/page.tsx", "linkedin"],
    ["src/app/(platforms)/pinterest-api/page.tsx", "pinterest"],
    ["src/app/(platforms)/threads-api/page.tsx", "threads"],
    ["src/app/(platforms)/tiktok-api/page.tsx", "tiktok"],
    ["src/app/(platforms)/twitter-api/page.tsx", "twitter"],
    ["src/app/(platforms)/youtube-api/page.tsx", "youtube"],
  ];

  it("builds every platform page metadata through the canonical helper", () => {
    const helperPath = "src/app/(platforms)/_config/metadata.ts";
    assert.equal(existsSync(join(root, helperPath)), true);
    const helper = read(helperPath);
    assert.match(helper, /const canonical = `https:\/\/unipost\.dev\/\$\{platform\.slug\}-api`/);
    assert.match(helper, /alternates:\s*{\s*canonical\s*}/s);

    for (const [routePath, platformName] of platformRoutes) {
      const source = read(routePath);
      assert.match(source, new RegExp(`buildPlatformMetadata\\(${platformName}\\)`));
    }
  });

  it("declares exact self-canonicals for audited docs routes", () => {
    const routes = [
      ["src/app/docs/page.tsx", "https://unipost.dev/docs"],
      ["src/app/docs/api/inbox/list/page.tsx", "https://unipost.dev/docs/api/inbox/list"],
      ["src/app/docs/api/inbox/reply/page.tsx", "https://unipost.dev/docs/api/inbox/reply"],
      ["src/app/docs/api/inbox/sync/page.tsx", "https://unipost.dev/docs/api/inbox/sync"],
      ["src/app/docs/guides/x/comments/page.tsx", "https://unipost.dev/docs/guides/x/comments"],
      ["src/app/docs/guides/x/reconnect-permissions/page.tsx", "https://unipost.dev/docs/guides/x/reconnect-permissions"],
    ];

    for (const [routePath, canonical] of routes) {
      const source = read(routePath);
      assert.equal(source.includes(canonical), true);
      assert.match(source, /alternates:\s*{\s*canonical:/s);
    }
  });

  it("declares exact self-canonicals for audited legal and tools routes", () => {
    const routes = [
      ["src/app/privacy/page.tsx", "https://unipost.dev/privacy"],
      ["src/app/terms/page.tsx", "https://unipost.dev/terms"],
      ["src/app/tools/page.tsx", "https://unipost.dev/tools"],
      ["src/app/tools/agentpost/page.tsx", "https://unipost.dev/tools/agentpost"],
      ["src/app/tools/character-counter/page.tsx", "https://unipost.dev/tools/character-counter"],
    ];

    for (const [routePath, canonical] of routes) {
      const source = read(routePath);
      assert.equal(source.includes(canonical), true);
      assert.match(source, /alternates:\s*{\s*canonical:/s);
    }
  });
});
