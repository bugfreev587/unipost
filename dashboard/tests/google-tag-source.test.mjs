import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, it } from "node:test";

const root = new URL("..", import.meta.url).pathname;

function read(path) {
  return readFileSync(join(root, path), "utf8");
}

describe("sitewide Google tag", () => {
  it("loads the GA4 tag once from the root App Router layout", () => {
    const source = read("src/app/layout.tsx");

    assert.match(source, /import Script from "next\/script";/);
    assert.match(source, /const GOOGLE_TAG_ID = "G-W2D6215V56";/);
    assert.match(source, /googletagmanager\.com\/gtag\/js\?id=\$\{GOOGLE_TAG_ID\}/);
    assert.match(source, /window\.dataLayer = window\.dataLayer \|\| \[\];/);
    assert.match(source, /function gtag\(\) \{ window\.dataLayer\.push\(arguments\); \}/);
    assert.match(source, /gtag\("js", new Date\(\)\);/);
    assert.match(source, /gtag\("config", "\$\{GOOGLE_TAG_ID\}"\);/);
    assert.match(source, /<Script[\s\S]*id="google-tag-loader"[\s\S]*strategy="afterInteractive"/);
    assert.match(source, /<Script[\s\S]*id="google-tag-init"[\s\S]*strategy="afterInteractive"/);
    assert.equal((source.match(/GOOGLE_TAG_ID/g) || []).length, 3);
  });

  it("loads the CookieYes consent banner before interactive scripts", () => {
    const source = read("src/app/layout.tsx");

    assert.match(source, /const COOKIEYES_SCRIPT_SRC = "https:\/\/cdn-cookieyes\.com\/client_data\/2e7c92a4d9dcb072ba8cdf03\/script\.js";/);
    assert.match(source, /<Script[\s\S]*id="cookieyes"[\s\S]*type="text\/javascript"[\s\S]*src=\{COOKIEYES_SCRIPT_SRC\}[\s\S]*strategy="beforeInteractive"/);
    assert.equal((source.match(/id="cookieyes"/g) || []).length, 1);
    assert.ok(source.indexOf('id="cookieyes"') < source.indexOf('id="google-tag-loader"'));
  });
});
