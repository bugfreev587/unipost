import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { test } from "node:test";

const dashboardRoot = process.cwd();
const namespaces = ["common", "navigation", "marketing", "pricing"];

function flattenMessages(value, prefix = "") {
  return Object.entries(value).flatMap(([key, child]) => {
    const nextKey = prefix ? `${prefix}.${key}` : key;
    return child && typeof child === "object" && !Array.isArray(child)
      ? flattenMessages(child, nextKey)
      : [[nextKey, child]];
  });
}

function placeholders(value) {
  return [...String(value).matchAll(/\{([\w]+)(?:,[^}]*)?\}/g)]
    .map((match) => match[1])
    .sort();
}

async function readCatalog(locale) {
  const catalog = {};
  for (const namespace of namespaces) {
    catalog[namespace] = JSON.parse(
      await readFile(path.join(dashboardRoot, "messages", locale, `${namespace}.json`), "utf8"),
    );
  }
  return catalog;
}

test("locale registry declares planned and released locales", async () => {
  const source = await readFile(path.join(dashboardRoot, "src/i18n/locales.ts"), "utf8");

  assert.match(source, /releasedLocales\s*=\s*\["en",\s*"es"\]/);
  assert.match(source, /plannedLocales\s*=\s*\["en",\s*"es",\s*"vi",\s*"hu",\s*"zh-CN",\s*"zh-TW"\]/);
  assert.match(source, /localeCookieName\s*=\s*"unipost_locale"/);
  assert.match(source, /"zh-CN"/);
  assert.match(source, /"zh-TW"/);
  assert.doesNotMatch(source, /he(?:brew)?/i);
});

test("English and Spanish catalogs have non-empty key and placeholder parity", async () => {
  const [english, spanish] = await Promise.all([readCatalog("en"), readCatalog("es")]);
  const englishEntries = flattenMessages(english);
  const spanishEntries = flattenMessages(spanish);

  assert.deepEqual(
    englishEntries.map(([key]) => key),
    spanishEntries.map(([key]) => key),
  );

  const spanishByKey = new Map(spanishEntries);
  for (const [key, englishValue] of englishEntries) {
    const spanishValue = spanishByKey.get(key);
    assert.equal(typeof englishValue, "string", `${key} must be a string in English`);
    assert.equal(typeof spanishValue, "string", `${key} must be a string in Spanish`);
    assert.ok(englishValue.trim(), `${key} must not be empty in English`);
    assert.ok(spanishValue.trim(), `${key} must not be empty in Spanish`);
    assert.deepEqual(placeholders(englishValue), placeholders(spanishValue), `${key} placeholders differ`);
  }

  assert.equal(spanish.navigation.languageName, "Español");
});

test("localized public routes persist locale without changing dashboard paths", async () => {
  const [proxy, rootLayout, localeLayout, localeHome, localePricing] = await Promise.all([
    readFile(path.join(dashboardRoot, "src/proxy.ts"), "utf8"),
    readFile(path.join(dashboardRoot, "src/app/layout.tsx"), "utf8"),
    readFile(path.join(dashboardRoot, "src/app/[locale]/layout.tsx"), "utf8"),
    readFile(path.join(dashboardRoot, "src/app/[locale]/page.tsx"), "utf8"),
    readFile(path.join(dashboardRoot, "src/app/[locale]/pricing/page.tsx"), "utf8"),
  ]);

  assert.match(proxy, /localeCookieName/);
  assert.match(proxy, /stripLocalePrefix/);
  assert.match(proxy, /maxAge:\s*60 \* 60 \* 24 \* 365/);
  assert.match(proxy, /sameSite:\s*"lax"/);
  assert.match(proxy, /secure:\s*request\.nextUrl\.protocol === "https:"/);
  assert.match(proxy, /pathname\.startsWith\("\/api"\)/);
  assert.match(localeLayout, /isReleasedLocale/);
  assert.match(localeLayout, /setRequestLocale\(locale\)/);
  assert.match(localeLayout, /notFound\(\)/);
  assert.match(localeHome, /MarketingPage/);
  assert.match(localePricing, /PricingPageClient/);
  assert.match(rootLayout, /getLocale\(\)/);
  assert.match(rootLayout, /NextIntlClientProvider/);
  assert.match(rootLayout, /lang=\{locale\}/);

  const dashboardFiles = await readFile(
    path.join(dashboardRoot, "src/app/(dashboard)/layout.tsx"),
    "utf8",
  );
  assert.doesNotMatch(dashboardFiles, /\[locale\]|localizePublicPathname/);
});

test("language selector is accessible and follows Developer in the public header", async () => {
  const [nav, selector, styles] = await Promise.all([
    readFile(path.join(dashboardRoot, "src/components/marketing/nav.tsx"), "utf8"),
    readFile(path.join(dashboardRoot, "src/components/marketing/language-selector.tsx"), "utf8"),
    readFile(path.join(dashboardRoot, "src/app/globals.css"), "utf8"),
  ]);

  assert.ok(nav.indexOf("<LanguageSelector") > nav.indexOf("Developer"));
  assert.ok(nav.indexOf("<LanguageSelector") < nav.lastIndexOf("<MarketingNav"));
  assert.match(selector, /aria-label=/);
  assert.match(selector, /aria-current=/);
  assert.match(selector, /window\.location\.assign/);
  assert.doesNotMatch(selector, /router\.replace/);
  assert.match(selector, /localeCookieName/);
  assert.doesNotMatch(selector, /flag|🇺🇸|🇪🇸/i);
  assert.match(styles, /\.mk-language-trigger/);
  assert.match(styles, /min-height:\s*40px/);
});

test("localized conversion pages use catalogs and publish correct SEO alternates", async () => {
  const [nav, footer, marketing, pricingClient, pricingPage, localeHome, localePricing, sitemap] =
    await Promise.all([
      readFile(path.join(dashboardRoot, "src/components/marketing/nav.tsx"), "utf8"),
      readFile(path.join(dashboardRoot, "src/components/marketing/site-footer.tsx"), "utf8"),
      readFile(path.join(dashboardRoot, "src/app/marketing/page.tsx"), "utf8"),
      readFile(path.join(dashboardRoot, "src/app/pricing/pricing-page-client.tsx"), "utf8"),
      readFile(path.join(dashboardRoot, "src/app/pricing/page.tsx"), "utf8"),
      readFile(path.join(dashboardRoot, "src/app/[locale]/page.tsx"), "utf8"),
      readFile(path.join(dashboardRoot, "src/app/[locale]/pricing/page.tsx"), "utf8"),
      readFile(path.join(dashboardRoot, "src/app/sitemap.ts"), "utf8"),
    ]);

  assert.match(nav, /useTranslations\("navigation"\)/);
  assert.match(footer, /useTranslations\("common"\)/);
  assert.match(marketing, /getTranslations\("marketing"\)/);
  assert.match(pricingClient, /useTranslations\("pricing"\)/);
  assert.match(pricingPage, /generateMetadata/);
  assert.match(localeHome, /generateMetadata/);
  assert.match(localePricing, /generateMetadata/);
  assert.match(sitemap, /"https:\/\/unipost\.dev\/es"/);
  assert.match(sitemap, /"https:\/\/unipost\.dev\/es\/pricing"/);
  assert.match(sitemap, /"x-default"/);

  const spanish = await readCatalog("es");
  assert.match(spanish.marketing.hero.title, /Publica/);
  assert.match(spanish.pricing.hero.titleLine1, /Empieza/);
  assert.match(spanish.common.footer.descriptionLine1, /API unificada/);
});

test("ordinary dashboard regression excludes landing-only localization acceptance", async () => {
  const config = await readFile(
    path.join(dashboardRoot, "playwright.regression.config.ts"),
    "utf8",
  );

  assert.match(config, /testIgnore/);
  assert.match(config, /localization\.spec\.ts/);
});
