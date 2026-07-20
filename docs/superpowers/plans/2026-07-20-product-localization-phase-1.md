# Product Localization Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship an isolated English/Spanish localization pilot for UniPost's public homepage and pricing path, with a language selector immediately after `Developer`, while replacing the incompatible password-based Clerk regression login with Clerk's development testing helper.

**Architecture:** Keep English public URLs unprefixed and add `/es` plus `/es/pricing` as the only released localized routes in this phase. `next-intl` owns message formatting and request locale context; the existing proxy remains the single host/auth boundary and adds only explicit locale-path recognition and locale-cookie persistence. Existing Dashboard URLs remain unchanged and no runtime translation model, backend locale persistence, email localization, or feature flag is introduced in this phase.

**Tech Stack:** Next.js 16.2 App Router, React 19.2, TypeScript, `next-intl` 4.13, Clerk 7, `@clerk/testing` 2.2, Playwright 1.60, Node test runner.

---

## Phase Boundary

This plan releases `en` and `es` only. It establishes registry entries for `vi`, `hu`, `zh-CN`, and `zh-TW`, but keeps them non-selectable until their reviewed catalogs and route coverage land in later plans.

Included:

- Clerk dev-instance authentication for the required Dashboard smoke test.
- Locale registry, cookie contract, message loading, and `next-intl` request configuration.
- English `/` and `/pricing`; Spanish `/es` and `/es/pricing`.
- Localized public navigation, homepage, pricing page, shared footer, metadata, canonicals, alternates, and sitemap entries.
- Desktop and responsive language selector after `Developer`, without flags.
- Catalog/routing/source-contract tests and public-route Playwright coverage.

Deferred:

- `users.locale`, `GET /v1/me`, and `PATCH /v1/me/locale`.
- Locale-free authenticated Dashboard catalog migration and account-settings preference.
- Transactional email localization.
- Vietnamese, Hungarian, Simplified Chinese, and Traditional Chinese release catalogs.
- AI translation automation and native-review state enforcement.

## File Structure

- `dashboard/src/i18n/locales.ts`: canonical frontend locale registry, released-locale guard, cookie name, and public pathname helpers.
- `dashboard/src/i18n/request.ts`: `next-intl` request configuration and namespace loading.
- `dashboard/src/i18n/navigation.ts`: locale-aware public href helpers used by the shared navigation.
- `dashboard/messages/{en,es}/{common,navigation,marketing,pricing}.json`: checked-in reviewed runtime catalogs.
- `dashboard/src/app/[locale]/layout.tsx`: validates released locale segments and establishes static locale context.
- `dashboard/src/app/[locale]/page.tsx`: Spanish homepage route using the existing marketing page implementation.
- `dashboard/src/app/[locale]/pricing/page.tsx`: Spanish pricing route using the existing pricing implementation.
- `dashboard/src/components/marketing/language-selector.tsx`: isolated interactive language menu.
- `dashboard/src/components/marketing/nav.tsx`: translated navigation/auth controls and selector placement.
- `dashboard/src/components/marketing/site-footer.tsx`: localized shared footer.
- `dashboard/src/app/marketing/page.tsx`: catalog-backed homepage copy.
- `dashboard/src/app/pricing/page.tsx`: localized metadata.
- `dashboard/src/app/pricing/pricing-page-client.tsx`: catalog-backed pricing copy.
- `dashboard/src/app/layout.tsx`: dynamic document language and `NextIntlClientProvider`.
- `dashboard/src/app/sitemap.ts`: English/Spanish route entries and alternates.
- `dashboard/src/proxy.ts`: locale path classification, English/Spanish negotiation, and cookie persistence while preserving Clerk/host behavior.
- `dashboard/tests/localization-contract.test.mjs`: registry, catalog, source, metadata, and proxy contracts.
- `dashboard/tests/regression/localization.spec.ts`: rendered English/Spanish public-route and selector behavior.
- `dashboard/tests/clerk-playwright-auth-source.test.mjs`: regression-auth source contract.
- `dashboard/tests/regression/clerk.setup.ts`: Clerk Testing Token setup project.
- `dashboard/tests/regression/dashboard.spec.ts`: passwordless Clerk test-helper login.
- `dashboard/playwright.regression.config.ts`: serial Clerk setup dependency and explicit dev credential validation.

### Task 1: Replace Password-Based Clerk Regression Login

**Files:**
- Create: `dashboard/tests/clerk-playwright-auth-source.test.mjs`
- Create: `dashboard/tests/regression/clerk.setup.ts`
- Modify: `dashboard/tests/regression/dashboard.spec.ts:1-8,223-262`
- Modify: `dashboard/playwright.regression.config.ts`
- Modify: `dashboard/package.json`

- [ ] **Step 1: Write the failing authentication source-contract test**

Create a Node test that reads the regression source and asserts:

```js
assert.match(source, /from "@clerk\/testing\/playwright"/);
assert.match(source, /clerk\.signIn\(\{/);
assert.match(source, /emailAddress: testEmail/);
assert.doesNotMatch(source, /DASHBOARD_TEST_PASSWORD/);
assert.doesNotMatch(source, /input\[type="password"\]/);
assert.match(config, /testMatch: \/clerk\\\.setup\\\.ts\$\//);
assert.match(config, /dependencies: \["clerk-setup"\]/);
```

- [ ] **Step 2: Run the source-contract test and confirm RED**

Run:

```bash
cd dashboard && node --test tests/clerk-playwright-auth-source.test.mjs
```

Expected: FAIL because the current test still references `DASHBOARD_TEST_PASSWORD` and does not import Clerk's Playwright helper.

- [ ] **Step 3: Add Clerk's setup project and passwordless sign-in**

Create `tests/regression/clerk.setup.ts`:

```ts
import {clerkSetup} from "@clerk/testing/playwright";
import {test as setup} from "@playwright/test";

setup.describe.configure({mode: "serial"});
setup("configure Clerk testing token", async () => {
  await clerkSetup();
});
```

Change `dashboard.spec.ts` to require only `DASHBOARD_TEST_EMAIL` and sign in with:

```ts
import {clerk} from "@clerk/testing/playwright";

async function signIn(page: Page, emailAddress: string) {
  await page.goto("/", {waitUntil: "domcontentloaded"});
  await clerk.signIn({page, emailAddress});
  await page.goto("/projects", {waitUntil: "networkidle"});
}
```

Replace `test.skip` with an explicit configuration error so a missing credential cannot appear as a successful suite:

```ts
if (!testEmail) {
  throw new Error("DASHBOARD_TEST_EMAIL is required for authenticated dashboard regression.");
}
```

Configure Playwright projects so `chromium` depends on `clerk-setup`, and add `test:clerk-auth-source` to `package.json`.

- [ ] **Step 4: Run the source contract and confirm GREEN**

Run:

```bash
cd dashboard && npm run test:clerk-auth-source
```

Expected: PASS.

- [ ] **Step 5: Run the authenticated smoke against dev**

Run with secrets injected by the local/CI secret store, never committed:

```bash
cd dashboard && \
DASHBOARD_BASE_URL=https://dev-app.unipost.dev \
DASHBOARD_TEST_EMAIL="$DASHBOARD_TEST_EMAIL" \
CLERK_PUBLISHABLE_KEY="$CLERK_PUBLISHABLE_KEY" \
CLERK_SECRET_KEY="$CLERK_SECRET_KEY" \
npm run test:regression:dashboard
```

Expected: all 62 tests pass with zero skipped tests. Any missing secret, skip, timeout, or failed route remains a hard stop.

- [ ] **Step 6: Commit the focused regression repair**

```bash
git add dashboard/tests/clerk-playwright-auth-source.test.mjs dashboard/tests/regression/clerk.setup.ts dashboard/tests/regression/dashboard.spec.ts dashboard/playwright.regression.config.ts dashboard/package.json
git commit -m "test: use Clerk helper for dashboard regression"
```

### Task 2: Add the Locale Registry and Message Runtime

**Files:**
- Create: `dashboard/src/i18n/locales.ts`
- Create: `dashboard/src/i18n/request.ts`
- Create: `dashboard/src/i18n/navigation.ts`
- Create: `dashboard/messages/en/common.json`
- Create: `dashboard/messages/en/navigation.json`
- Create: `dashboard/messages/en/marketing.json`
- Create: `dashboard/messages/en/pricing.json`
- Create: `dashboard/messages/es/common.json`
- Create: `dashboard/messages/es/navigation.json`
- Create: `dashboard/messages/es/marketing.json`
- Create: `dashboard/messages/es/pricing.json`
- Create: `dashboard/tests/localization-contract.test.mjs`
- Modify: `dashboard/next.config.ts`
- Modify: `dashboard/package.json`
- Modify: `dashboard/package-lock.json`

- [ ] **Step 1: Write failing registry and catalog contracts**

The Node contract test must assert that:

```js
assert.deepEqual(releasedLocales, ["en", "es"]);
assert.deepEqual(plannedLocales, ["en", "es", "vi", "hu", "zh-CN", "zh-TW"]);
assert.equal(localeCookieName, "unipost_locale");
assert.deepEqual(flattenKeys(enCatalog), flattenKeys(esCatalog));
assert.equal(esCatalog.navigation.languageName, "Español");
```

It must also reject empty translated values and validate that ICU placeholders match for every English/Spanish key.

- [ ] **Step 2: Run the contract and confirm RED**

Run:

```bash
cd dashboard && node --test tests/localization-contract.test.mjs
```

Expected: FAIL because the registry and catalogs do not exist.

- [ ] **Step 3: Install and configure `next-intl`**

Run:

```bash
cd dashboard && npm install next-intl@4.13.2
```

Wrap the existing config without changing its Turbopack root:

```ts
import createNextIntlPlugin from "next-intl/plugin";

const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");
export default withNextIntl(nextConfig);
```

- [ ] **Step 4: Implement the canonical registry**

Use a typed registry with all planned locales and only English/Spanish released:

```ts
export const localeRegistry = {
  en: {nativeName: "English", direction: "ltr", released: true, fallback: null},
  es: {nativeName: "Español", direction: "ltr", released: true, fallback: "en"},
  vi: {nativeName: "Tiếng Việt", direction: "ltr", released: false, fallback: "en"},
  hu: {nativeName: "Magyar", direction: "ltr", released: false, fallback: "en"},
  "zh-CN": {nativeName: "简体中文", direction: "ltr", released: false, fallback: "en"},
  "zh-TW": {nativeName: "繁體中文", direction: "ltr", released: false, fallback: "en"}
} as const;

export const releasedLocales = ["en", "es"] as const;
export const defaultLocale = "en" as const;
export const localeCookieName = "unipost_locale" as const;
```

`isReleasedLocale`, `stripLocalePrefix`, `localizePublicPathname`, and `isLocalizedPublicPathname` must be pure typed helpers. Only `/` and `/pricing` are localized in this phase; other destinations return their English canonical pathname.

- [ ] **Step 5: Add namespace catalogs and request loading**

`request.ts` must validate `requestLocale`, fall back to English, load only `common`, `navigation`, `marketing`, and `pricing`, and return a merged message object. English and Spanish files must have identical semantic keys and ICU variables. Product/platform names, API paths, code examples, plan currency, and identifiers remain unchanged.

- [ ] **Step 6: Run the contract and confirm GREEN**

Run:

```bash
cd dashboard && node --test tests/localization-contract.test.mjs
```

Expected: PASS with locale order, key parity, non-empty values, and placeholder parity all confirmed.

- [ ] **Step 7: Commit the locale runtime**

```bash
git add dashboard/package.json dashboard/package-lock.json dashboard/next.config.ts dashboard/src/i18n dashboard/messages dashboard/tests/localization-contract.test.mjs
git commit -m "feat: add English and Spanish locale runtime"
```

### Task 3: Add Localized Public Routes Without Changing Dashboard URLs

**Files:**
- Create: `dashboard/src/app/[locale]/layout.tsx`
- Create: `dashboard/src/app/[locale]/page.tsx`
- Create: `dashboard/src/app/[locale]/pricing/page.tsx`
- Modify: `dashboard/src/app/layout.tsx`
- Modify: `dashboard/src/proxy.ts`
- Modify: `dashboard/tests/localization-contract.test.mjs`

- [ ] **Step 1: Add failing route/proxy source contracts**

Assert that the proxy:

```js
assert.match(proxy, /localeCookieName/);
assert.match(proxy, /stripLocalePrefix/);
assert.match(proxy, /maxAge: 60 \* 60 \* 24 \* 365/);
assert.match(proxy, /sameSite: "lax"/);
assert.match(proxy, /secure: request\.nextUrl\.protocol === "https:"/);
assert.doesNotMatch(proxy, /pathname\.startsWith\("\/api"\).*locale/);
```

Assert that `[locale]` validates with `isReleasedLocale`, calls `setRequestLocale`, and that no locale segment is introduced under `src/app/(dashboard)`.

- [ ] **Step 2: Run the contract and confirm RED**

Run `npm run test:localization-contract` and expect the missing localized routes/proxy behavior to fail.

- [ ] **Step 3: Add the locale segment layout and route wrappers**

The locale layout must await params, call `notFound()` for unreleased locales, call `setRequestLocale(locale)`, and render its children. The page wrappers reuse the existing marketing/pricing components instead of copying their layout or CSS.

- [ ] **Step 4: Compose locale behavior into the existing proxy**

Preserve the current decision order for static exclusions, public docs/API exemptions, landing-host rewrites, Dashboard-host protection, and Clerk. Add these behaviors only for the localized manifest:

1. `/es` and `/es/pricing` are public on marketing and app hosts.
2. Explicit `/es...` writes `unipost_locale=es` for one year.
3. Selecting English writes `unipost_locale=en`; English stays unprefixed.
4. `/`, `/pricing`, `/es`, and `/es/pricing` never receive Dashboard locale segments.
5. API, Clerk callback, OAuth callback, static asset, and Dashboard project paths are never locale rewritten.
6. Unsupported locale-looking prefixes are not treated as released locale routes.

- [ ] **Step 5: Make the root document locale-aware**

Make `RootLayout` async, resolve `getLocale()`, set `<html lang={locale}>`, and wrap the existing content with `NextIntlClientProvider`. Retain ClerkProvider, theme initialization, analytics, cookie consent, and all existing font classes.

- [ ] **Step 6: Run contracts and build**

Run:

```bash
cd dashboard && npm run test:localization-contract && npm run build
```

Expected: contracts pass and Next.js generates `/[locale]` plus `/[locale]/pricing` without changing authenticated route paths.

- [ ] **Step 7: Commit routing integration**

```bash
git add dashboard/src/app/[locale] dashboard/src/app/layout.tsx dashboard/src/proxy.ts dashboard/tests/localization-contract.test.mjs
git commit -m "feat: add localized public routes"
```

### Task 4: Add the Accessible Language Selector in the Approved Header Position

**Files:**
- Create: `dashboard/src/components/marketing/language-selector.tsx`
- Modify: `dashboard/src/components/marketing/nav.tsx`
- Modify: `dashboard/src/app/globals.css`
- Modify: `dashboard/tests/localization-contract.test.mjs`
- Create: `dashboard/tests/regression/localization.spec.ts`

- [ ] **Step 1: Write failing placement and accessibility tests**

The source contract must compare source offsets and require:

```js
assert.ok(nav.indexOf("<LanguageSelector") > nav.indexOf("Developer"));
assert.ok(nav.indexOf("<LanguageSelector") < nav.indexOf("<MarketingNav"));
assert.match(selector, /aria-label=/);
assert.match(selector, /aria-current=/);
assert.match(selector, /router\.replace/);
assert.doesNotMatch(selector, /flag|🇺🇸|🇪🇸/i);
```

The Playwright test must verify the Spanish route renders `Español`, opens a menu with `English` and `Español`, supports keyboard activation, and switching to English lands on the unprefixed equivalent.

- [ ] **Step 2: Run the tests and confirm RED**

Run the Node contract and the localization Playwright spec against a local production build. Expected: FAIL because the selector and Spanish routes are not rendered yet.

- [ ] **Step 3: Implement an isolated client selector**

Use the existing dropdown primitives and a native-name text trigger. Keep client state inside the selector leaf. On selection:

```ts
document.cookie = `${localeCookieName}=${nextLocale}; Path=/; Max-Age=31536000; SameSite=Lax${cookieDomain}`;
router.replace(localizePublicPathname(pathname, nextLocale));
router.refresh();
```

The selected menu item uses `aria-current="true"`, a text-safe selected indicator, visible focus, and at least a 40px touch target. No flag, globe emoji, neon glow, new card treatment, or continuous animation is added.

- [ ] **Step 4: Insert it immediately after Developer**

Keep the layout order:

```tsx
<DeveloperMenu />
<LanguageSelector />
<MarketingNav />
```

On narrow screens, allow the existing navigation row to wrap or collapse without horizontal overflow; the language control remains before authentication actions.

- [ ] **Step 5: Run selector contracts and Playwright**

Run:

```bash
cd dashboard && npm run test:localization-contract
cd dashboard && DASHBOARD_WEB_SERVER=1 DASHBOARD_BASE_URL=http://localhost:3000 npm run test:regression:localization
```

Expected: selector placement, keyboard behavior, English/Spanish path switching, cookie persistence, and no-horizontal-overflow checks pass.

- [ ] **Step 6: Commit the selector**

```bash
git add dashboard/src/components/marketing/language-selector.tsx dashboard/src/components/marketing/nav.tsx dashboard/src/app/globals.css dashboard/tests/localization-contract.test.mjs dashboard/tests/regression/localization.spec.ts dashboard/package.json
git commit -m "feat: add public language selector"
```

### Task 5: Localize Homepage, Pricing, Footer, and SEO

**Files:**
- Modify: `dashboard/messages/en/{common,navigation,marketing,pricing}.json`
- Modify: `dashboard/messages/es/{common,navigation,marketing,pricing}.json`
- Modify: `dashboard/src/components/marketing/nav.tsx`
- Modify: `dashboard/src/components/marketing/site-footer.tsx`
- Modify: `dashboard/src/app/marketing/page.tsx`
- Modify: `dashboard/src/app/pricing/page.tsx`
- Modify: `dashboard/src/app/pricing/pricing-page-client.tsx`
- Modify: `dashboard/src/app/[locale]/page.tsx`
- Modify: `dashboard/src/app/[locale]/pricing/page.tsx`
- Modify: `dashboard/src/app/sitemap.ts`
- Modify: `dashboard/tests/localization-contract.test.mjs`
- Modify: `dashboard/tests/regression/localization.spec.ts`

- [ ] **Step 1: Add failing copy and SEO contracts**

Require the Spanish rendered routes to contain Spanish hero, pricing, CTA, and footer markers while preserving `UniPost`, platform names, `/v1/...` paths, and USD prices. Assert:

- `/` canonical is `https://unipost.dev/`.
- `/es` canonical is `https://unipost.dev/es`.
- `/pricing` canonical is `https://unipost.dev/pricing`.
- `/es/pricing` canonical is `https://unipost.dev/es/pricing`.
- Each pair emits `en`, `es`, and `x-default` alternates.
- Sitemap includes exactly those four localized-manifest URLs with matching alternates.

- [ ] **Step 2: Run contracts and confirm RED**

Run `npm run test:localization-contract` and the localization Playwright spec. Expected: FAIL on untranslated copy and missing alternates.

- [ ] **Step 3: Move English source copy into semantic catalog keys**

Replace user-visible literals in the in-scope components with `useTranslations`/`getTranslations`. Keep content structures typed in code while storing labels, headings, descriptions, FAQ questions/answers, and CTA text under semantic keys such as:

```text
navigation.developer.label
marketing.hero.title
marketing.workflow.connect.body
pricing.hero.title
pricing.plans.growth.description
pricing.faq.billing.question
common.actions.getStartedFree
```

Do not translate source code, API routes, brand/platform names, identifiers, plan currencies, or user-generated examples.

- [ ] **Step 4: Add complete Spanish values with parity**

Populate every released key in Spanish. Preserve ICU placeholders exactly. Use neutral international Spanish, sentence case, concise CTA wording, `API` unchanged, and `UniPost` unchanged. Mark the catalog header metadata as `reviewStatus: "product-approved"` only after the user accepts the rendered preview; until then the locale remains present in Preview but the PR remains Draft.

- [ ] **Step 5: Add localized metadata and sitemap alternates**

Use `generateMetadata` with `getTranslations` for route-specific title/description/Open Graph values. Emit canonical and language alternates only for English/Spanish routes that truly contain translated content; use English as `x-default`.

- [ ] **Step 6: Run contract, build, and localized route tests**

Run:

```bash
cd dashboard && npm run test:localization-contract
cd dashboard && npm run build
cd dashboard && DASHBOARD_WEB_SERVER=1 DASHBOARD_BASE_URL=http://localhost:3000 npm run test:regression:localization
```

Expected: catalog parity, metadata, sitemap, rendered English/Spanish content, selector, focus behavior, and responsive overflow checks all pass.

- [ ] **Step 7: Commit the localized pilot content**

```bash
git add dashboard/messages dashboard/src/components/marketing dashboard/src/app/marketing/page.tsx dashboard/src/app/pricing dashboard/src/app/[locale] dashboard/src/app/sitemap.ts dashboard/tests
git commit -m "feat: localize public conversion path in Spanish"
```

### Task 6: Full Local Verification and Preview Handoff

**Files:**
- Modify only if verification exposes an in-scope defect.

- [ ] **Step 1: Audit ownership before tests**

Run:

```bash
pwd
git branch --show-current
git rev-parse HEAD
git status --short
```

Expected path: `/Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-product-localization-implementation`; expected branch: `dev-product-localization-implementation`.

- [ ] **Step 2: Run local CI-equivalent checks**

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./...
cd dashboard && npm run test:clerk-auth-source
cd dashboard && npm run test:localization-contract
cd dashboard && npm run build
cd dashboard && DASHBOARD_BASE_URL=https://dev-app.unipost.dev DASHBOARD_TEST_EMAIL="$DASHBOARD_TEST_EMAIL" CLERK_PUBLISHABLE_KEY="$CLERK_PUBLISHABLE_KEY" CLERK_SECRET_KEY="$CLERK_SECRET_KEY" npm run test:regression:dashboard
```

Expected: every command succeeds and the Dashboard suite has zero skipped tests.

- [ ] **Step 3: Perform local browser acceptance**

Check desktop and mobile widths for `/`, `/es`, `/pricing`, and `/es/pricing`. Verify selector position, native names, menu focus order, no flags, no horizontal overflow, current-language state, URL transitions, refreshed copy, `<html lang>`, canonical, and alternates.

- [ ] **Step 4: Audit branch content before push**

List exact commits and files unique to `origin/dev`:

```bash
git log --oneline origin/dev..HEAD
git diff --name-status origin/dev...HEAD
```

Stop if any unrelated, unidentified, unfinished, or unaccepted file is present.

- [ ] **Step 5: Push only the owned task branch and open a Draft PR to `dev`**

```bash
git push -u origin dev-product-localization-implementation
gh pr create --draft --base dev --head dev-product-localization-implementation
```

Do not merge. Monitor GitHub CI, Railway PR Environment, Vercel Preview, deployed regression, and browser acceptance on the exact PR head SHA.

- [ ] **Step 6: Run Preview Acceptance on the exact SHA**

Use the Vercel Preview URL wired to the Railway PR API. Verify English and Spanish routes plus authenticated Dashboard smoke with the development Clerk instance. Any failed, skipped, missing, timed-out, or wrong-SHA result is a hard stop with complete evidence reporting.

- [ ] **Step 7: Request product-language acceptance**

Keep the PR Draft until the user accepts the Spanish rendered copy and selector behavior. Product acceptance changes translation review metadata from preview-only to product-approved. Merging to `dev` is outside this step until every mandatory gate succeeds.

## Self-Review

- Spec coverage: Phase 1 covers the approved Spanish pilot, public selector position, persisted locale cookie, localized homepage/pricing shell, SEO, and passwordless Clerk regression. Backend preference, Dashboard catalogs, email, and remaining languages are explicitly assigned to later independent plans.
- Placeholder scan: the plan contains no implementation placeholders; each deferred item is a deliberate phase boundary rather than unfinished Phase 1 work.
- Type consistency: locale identifiers are consistently `en | es` for released runtime behavior and the canonical planned registry retains `vi | hu | zh-CN | zh-TW` as unreleased entries. `DASHBOARD_TEST_PASSWORD` is removed everywhere; `DASHBOARD_TEST_EMAIL`, `CLERK_PUBLISHABLE_KEY`, and `CLERK_SECRET_KEY` are the only auth inputs for this suite.
