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
