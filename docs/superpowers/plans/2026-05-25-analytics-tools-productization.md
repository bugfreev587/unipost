# Analytics Tools Productization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Productize public analytics tool pages for TikTok, Instagram, Threads, and Pinterest, clean the `/tools` index, and publish a UniPost Analytics overview blog post.

**Architecture:** Add a small config-driven public analytics tool layer under `dashboard/src/app/tools/_components`. Each public platform route imports one config, exports route metadata, and renders a static sample analytics surface without Clerk or live account data. Existing blog and sitemap data flows are extended rather than replaced.

**Tech Stack:** Next.js App Router, React Server Components, existing `/tools` CSS, `lucide-react`, Playwright regression smoke tests.

---

### Task 1: Regression Tests First

**Files:**
- Modify: `dashboard/tests/regression/dashboard.spec.ts`

- [ ] **Step 1: Add failing public route coverage**

Update `publicRoutes` to include all four analytics tool pages and `/tools`:

```ts
const publicRoutes = [
  { path: "/docs", marker: /UniPost|Dashboard|API/i },
  { path: "/pricing", marker: /Free|Basic|Growth|Team/i },
  { path: "/tools", marker: /TikTok Analytics|Instagram Analytics|Threads Analytics|Pinterest Analytics/i },
  { path: "/tools/tiktok-analytics", marker: /TikTok Analytics/i },
  { path: "/tools/instagram-analytics", marker: /Instagram Analytics/i },
  { path: "/tools/threads-analytics", marker: /Threads Analytics/i },
  { path: "/tools/pinterest-analytics", marker: /Pinterest Analytics/i },
];
```

Add a test that verifies `/tools` has no coming-soon cards:

```ts
test("/tools only shows live tools", async ({ page }) => {
  await page.goto("/tools", { waitUntil: "domcontentloaded" });
  await expect(page.getByRole("link", { name: /TikTok Analytics/i })).toBeVisible();
  await expect(page.getByRole("link", { name: /Instagram Analytics/i })).toBeVisible();
  await expect(page.getByRole("link", { name: /Threads Analytics/i })).toBeVisible();
  await expect(page.getByRole("link", { name: /Pinterest Analytics/i })).toBeVisible();
  await expect(page.getByText(/Coming Soon/i)).toHaveCount(0);
  await expect(page.getByText(/Thread Splitter|Caption Generator/i)).toHaveCount(0);
});
```

- [ ] **Step 2: Verify red**

Run:

```bash
cd dashboard && DASHBOARD_BASE_URL=https://unipost.dev npx playwright test --config=playwright.regression.config.ts --grep "instagram-analytics|threads-analytics|pinterest-analytics|only shows live tools"
```

Expected: tests fail because the three new analytics routes are missing and `/tools` still contains coming-soon cards.

### Task 2: Shared Public Analytics Tool Component

**Files:**
- Create: `dashboard/src/app/tools/_components/public-analytics-tool.tsx`

- [ ] **Step 1: Create typed platform config and renderer**

Create a server-safe component exporting these exact public types and functions:

```ts
export type AnalyticsToolSlug = "tiktok" | "instagram" | "threads" | "pinterest";
export type AnalyticsMetric = { label: string; value: string; note: string };
export type AnalyticsTable = { title: string; description: string; headers: string[]; rows: string[][] };
export type AnalyticsToolConfig = {
  slug: AnalyticsToolSlug;
  platform: string;
  href: string;
  title: string;
  seoTitle: string;
  description: string;
  eyebrow: string;
  summary: string;
  accent: string;
  scopes: string[];
  metrics: AnalyticsMetric[];
  tables: AnalyticsTable[];
  docsHref: string;
};

export const analyticsTools: Record<AnalyticsToolSlug, AnalyticsToolConfig>;
export function getAnalyticsTool(slug: AnalyticsToolSlug): AnalyticsToolConfig;
export function PublicAnalyticsToolPage({ tool }: { tool: AnalyticsToolConfig }): JSX.Element;
```

The implementation must define all four `analyticsTools` entries in the same file. Each entry must include complete sample `metrics` and `tables` arrays, no runtime fetching, and no empty fallback copy. The component should render a productized page with hero, metric cards, scope list, sample analytics tables, related platform links, and CTAs to `https://app.unipost.dev/welcome` and `/docs/api/analytics`.

- [ ] **Step 2: Keep the renderer static**

Do not import `useAuth`, dashboard data-fetching APIs, or authenticated dashboard components. Use sample data only.

### Task 3: Public Routes and Metadata

**Files:**
- Modify: `dashboard/src/app/tools/tiktok-analytics/page.tsx`
- Create: `dashboard/src/app/tools/instagram-analytics/page.tsx`
- Create: `dashboard/src/app/tools/threads-analytics/page.tsx`
- Create: `dashboard/src/app/tools/pinterest-analytics/page.tsx`

- [ ] **Step 1: Replace TikTok preview wrapper**

Render:

```tsx
import type { Metadata } from "next";
import { getAnalyticsTool, PublicAnalyticsToolPage } from "../_components/public-analytics-tool";

const tool = getAnalyticsTool("tiktok");

export const metadata: Metadata = {
  title: tool.seoTitle,
  description: tool.description,
  alternates: { canonical: `https://unipost.dev${tool.href}` },
};

export default function TikTokAnalyticsToolPage() {
  return <PublicAnalyticsToolPage tool={tool} />;
}
```

- [ ] **Step 2: Add Instagram, Threads, and Pinterest wrappers**

Create the same wrapper pattern for `instagram`, `threads`, and `pinterest`, changing only the slug and component name.

### Task 4: Tools Index Cleanup

**Files:**
- Modify: `dashboard/src/app/tools/page.tsx`
- Modify: `dashboard/src/components/tools/ToolCard.tsx`
- Modify: `dashboard/src/app/tools/layout.tsx`

- [ ] **Step 1: Convert tool icons from emoji text to mapped icons**

In `ToolCard.tsx`, replace `icon: string` with:

```ts
export type ToolIconKey = "agentpost" | "character-counter" | "tiktok" | "instagram" | "threads" | "pinterest";
```

Map these keys to `lucide-react` icons and `PlatformIcon`. Render icons in the existing `.tl-card-icon` span.

- [ ] **Step 2: Update `TOOLS`**

Keep AgentPost and Character Counter, add four analytics pages, and remove Thread Splitter and Caption Generator:

```ts
const TOOLS: ToolCardData[] = [
  { icon: "agentpost", name: "AgentPost", description: "AI-powered multi-platform social posting", href: "/tools/agentpost", status: "live", badge: "New" },
  { icon: "character-counter", name: "Character Counter", description: "Check post length for every platform", href: "/tools/character-counter", status: "live", badge: "New" },
  { icon: "tiktok", name: "TikTok Analytics", description: "Preview TikTok profile, video, and post analytics", href: "/tools/tiktok-analytics", status: "live" },
  { icon: "instagram", name: "Instagram Analytics", description: "Preview Instagram Business media and post insights", href: "/tools/instagram-analytics", status: "live" },
  { icon: "threads", name: "Threads Analytics", description: "Preview Threads profile and post performance", href: "/tools/threads-analytics", status: "live" },
  { icon: "pinterest", name: "Pinterest Analytics", description: "Preview Pin, board, save, and click analytics", href: "/tools/pinterest-analytics", status: "live" },
];
```

- [ ] **Step 3: Adjust tools grid CSS**

Change the tools grid to support six live cards cleanly:

```css
.tl-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,280px),1fr));gap:20px;padding:0 0 var(--tl-section-py)}
.tl-card-icon{width:34px;height:34px;display:inline-flex;align-items:center;justify-content:center;color:var(--tl-text)}
```

### Task 5: Blog Post and Sitemap

**Files:**
- Modify: `dashboard/src/lib/blog.ts`
- Modify: `dashboard/src/app/sitemap.ts`

- [ ] **Step 1: Add blog post data**

Add a new first item in `blogPosts` with:

```ts
slug: "social-media-analytics-api",
title: "Social Media Analytics API: Posts Overview and Platform Insights in UniPost",
seoTitle: "Social Media Analytics API for TikTok, Instagram, Threads, Pinterest",
description: "How UniPost combines cross-platform post analytics with native platform analytics for TikTok, Instagram, Threads, and Pinterest.",
publishedAt: "2026-05-25",
updatedAt: "2026-05-25",
readingTime: "6 min read",
category: "Analytics",
author: "UniPost",
```

The blocks must cover Posts Overview, Platform Analytics, platform metric differences, API docs links, and links to the four tool pages.

- [ ] **Step 2: Add tools to sitemap**

In `dashboard/src/app/sitemap.ts`, add public tool URLs:

```ts
const toolPages: MetadataRoute.Sitemap = [
  "/tools",
  "/tools/agentpost",
  "/tools/character-counter",
  "/tools/tiktok-analytics",
  "/tools/instagram-analytics",
  "/tools/threads-analytics",
  "/tools/pinterest-analytics",
].map((path) => ({
  url: `${BASE}${path}`,
  lastModified: now,
  changeFrequency: "weekly" as const,
  priority: path === "/tools" ? 0.7 : 0.6,
}));
```

Return `toolPages` with the existing sitemap arrays.

### Task 6: Green Verification

**Files:**
- Test only

- [ ] **Step 1: Run the targeted regression tests**

Run:

```bash
cd dashboard && DASHBOARD_BASE_URL=https://unipost.dev npx playwright test --config=playwright.regression.config.ts --grep "tools|analytics"
```

Expected after deployment only: public route tests pass on the target URL. For local pre-deploy validation, use a local server instead.

- [ ] **Step 2: Run local build**

Run:

```bash
cd dashboard && npm run build
```

Expected: Next.js production build succeeds.

- [ ] **Step 3: Run local regression if possible**

Run:

```bash
cd dashboard && DASHBOARD_WEB_SERVER=1 DASHBOARD_BASE_URL=http://localhost:3000 npm run test:regression:dashboard
```

Expected: public route tests pass. Authenticated tests remain skipped unless credentials are configured.

- [ ] **Step 4: Inspect changed files**

Run:

```bash
git diff --stat
git status --short
```

Expected: only dashboard files and docs plan/spec files from this task are staged/changed, plus any pre-existing unrelated user changes remain unstaged.
