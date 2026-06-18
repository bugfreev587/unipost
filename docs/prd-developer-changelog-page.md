# Developer Navigation And Change Logs Page PRD
Status: Planning
Owner: Marketing / Developer Experience
Created: 2026-06-18

---

## 1. Background

UniPost 现在的公开 landing navigation 里有一个直接指向 `/docs` 的 `Docs` 链接。这个入口对已经知道要看文档的开发者有效，但它没有表达 UniPost 更完整的 developer surface，也没有给新访客一个清晰信号：UniPost 正在持续发布产品能力、API 能力和 SDK 能力。

用户提出将 landing page 上的 `Docs` 改成 `Developer`，并在下拉里放 `Docs` 和 `Change Logs`。这是一个合理的产品方向：

- `Developer` 比 `Docs` 更像一个长期产品栏目，可以承载 docs、API reference、SDK、CLI、MCP、release history 等未来内容。
- `Change Logs` 可以成为公开的产品进展页，让用户知道 UniPost 不是一次性 landing page，而是一个正在迭代的开发者平台。
- 对 SDK 发布尤其重要。SDK 版本号如果只藏在 package registry 或 GitHub tag 里，集成者不容易感知。Change Logs 应把 SDK version 作为一等信息展示。

---

## 2. Product Goals

1. 将公开站点顶部导航中的 `Docs` 升级为 `Developer` 下拉入口。
2. 在 `Developer` 下拉中提供两个首批入口：
   - `Docs` -> `/docs`
   - `Change Logs` -> `/changelog`
3. 新增一个公开的 Change Logs 页面，记录 UniPost 的重要发布，而不是所有小修小补。
4. 让访客在 10 秒内感知三件事：
   - UniPost 是一个仍在稳定迭代的产品。
   - UniPost 的能力发展有清晰轨迹。
   - SDK、API、Dashboard、平台支持等发布都有可追踪版本和链接。
5. SDK 发布时必须显示 SDK package name 和 version number，例如 `@unipost/sdk-js v0.4.0`。
6. 页面要适合未来持续维护，不依赖工程师每次改复杂布局才能新增一条发布记录。

---

## 3. Non-goals

- 不做 public roadmap。Change Logs 只记录已经发布或已经公开可用的内容。
- 不做 status page。故障、incident、downtime、provider outage 不进入此页面。
- 不做完整 release notes 系统。v1 不需要评论、RSS、订阅、邮件通知或后台 CMS。
- 不记录每个 commit、bugfix、copy tweak 或内部运维变更。
- 不把用户私有 workspace logs、developer logs 或 API traffic logs 混进公开 Change Logs。
- 不在 v1 强制引入 feature flag。若后续实现时需要灰度导航或控制发布时间，再按仓库规则使用 Unleash。

---

## 4. Target Users

### 4.1 New Developer Evaluating UniPost

Jobs:

- 判断 UniPost 是否还在维护。
- 判断 API、SDK、platform support 是否有足够迭代速度。
- 快速找到最近发布的能力和相关 docs。

### 4.2 Existing Customer Or Integrator

Jobs:

- 查看最近有没有影响自己集成的 API、SDK、Dashboard 或 platform capability 更新。
- 找到 SDK 最新版本号和升级入口。
- 回看某个能力大概是什么时候发布的。

### 4.3 Internal GTM / Support

Jobs:

- 给潜在客户发送一个可信的产品进展链接。
- 在 support 中引用某个发布条目，而不是临时解释。
- 确认对外发布口径和 docs 链接。

---

## 5. Information Architecture

### 5.1 Public Navigation

Top-level nav should change from:

```text
Solutions | Tools | Pricing | Blog | Docs
```

to:

```text
Solutions | Tools | Pricing | Blog | Developer
```

`Developer` is a dropdown menu, not a direct-only link.

Dropdown items:

| Label | URL | Description |
| --- | --- | --- |
| Docs | `/docs` | Quickstarts, API reference, platform guides, SDK, CLI, MCP |
| Change Logs | `/changelog` | Major product updates, API releases, SDK versions |

The canonical page URL should be `/changelog`. The nav label and page title can remain `Change Logs` to match the requested product wording. If users type `/change-logs`, the implementation may redirect it to `/changelog`, but `/changelog` should be canonical for SEO and convention.

### 5.2 Active State

`Developer` should be active when the current route starts with:

- `/docs`
- `/changelog`
- `/change-logs` if an alias route exists

### 5.3 Footer

Footer v1 can keep `Docs` under Product, but the preferred follow-up is to add or rename a footer column to `Developer` with:

- Docs
- Change Logs
- API Reference
- SDK
- CLI

This footer adjustment is not required for the first implementation, but the PRD recommends it because the top navigation will now frame these links as a developer surface.

---

## 6. Change Logs Page Content Strategy

The page should publish high-signal release entries only. A good rule:

- Include a release if a user could reasonably care about it during evaluation, integration, upgrade, or support.
- Exclude a release if it is only an internal refactor, tiny copy change, dependency update, or non-user-visible cleanup.

Recommended release categories:

| Category | Include Examples |
| --- | --- |
| API | New endpoints, new payload fields, breaking behavior changes, new webhook events |
| SDK | New package release, major SDK method coverage, auth change, generated types |
| Dashboard | Major workflow changes, new analytics pages, account connection improvements |
| Platform | New supported social network, expanded support for a network, OAuth capability changes |
| Developer Experience | Docs IA changes, CLI, MCP, local testing tools, agent integration |
| Reliability | Queueing, delivery tracking, retries, observability, logs visible to customers |

Recommended cadence:

- Add entries when a major release ships.
- If several smaller but related improvements ship in the same week, group them into one entry.
- If there are no major releases in a month, do not fabricate updates. A sparse but truthful changelog is more credible than a noisy one.

---

## 7. Page Structure

### 7.1 First View

The first viewport should immediately show the product history surface, not a marketing explanation page.

Required elements:

- Eyebrow: `Product updates`
- H1: `Change Logs`
- Short lead: explain that this page tracks major UniPost product, API, SDK, and platform releases.
- Latest release highlight: a compact summary of the most recent major release, with date, category, title, optional SDK version, and link.
- A visible start of the release table or timeline below the hero so the user understands this is a real history page.

Tone:

- Concrete and factual.
- Avoid exaggerated launch language.
- Avoid pretending every small fix is a milestone.

### 7.2 Release Table

The core page should be a polished table on desktop and stacked release rows on mobile.

Desktop columns:

| Column | Purpose |
| --- | --- |
| Date | Release date, displayed as `YYYY-MM-DD` or `Month YYYY` if only a month-level release is appropriate |
| Release | Human-readable title and 1 to 2 sentence summary |
| Area | Category pill, such as `API`, `SDK`, `Dashboard`, `Platform`, `DX`, `Reliability` |
| SDK | Package and version pills when applicable, otherwise `-` |
| Links | Docs, API reference, blog post, package registry, or migration guide |

Mobile behavior:

- Each release becomes one stacked row.
- Date and category appear at top.
- Title and summary come next.
- SDK versions appear as inline pills.
- Links appear as compact text buttons.
- No horizontal scrolling should be required.

### 7.3 Filters

v1 should include simple client-side filters only if the data set is large enough at launch. If launch has fewer than 10 entries, filters can be visually present but should not dominate the page.

Recommended filters:

- All
- API
- SDK
- Dashboard
- Platform
- DX

Filtering should be progressive enhancement. The full release list must still be visible and crawlable without relying on client-side JavaScript.

### 7.4 Release Detail Density

Each entry should be short. Suggested structure:

```text
Title
One-sentence summary of what changed and why it matters.
Optional second sentence for migration, SDK version, or platform scope.
```

Avoid long paragraphs. If a release needs detailed explanation, link to Docs or Blog.

---

## 8. Data Model

The first implementation should use a typed local data source, not a CMS.

Recommended fields:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `id` | string | yes | Stable slug for anchors |
| `date` | string | yes | ISO date if known, otherwise month-level metadata plus display label |
| `title` | string | yes | Short release title |
| `summary` | string | yes | 1 to 2 sentences |
| `category` | enum | yes | `api`, `sdk`, `dashboard`, `platform`, `dx`, `reliability` |
| `sdkVersions` | array | no | Package name, version, registry URL |
| `links` | array | no | Label and URL |
| `impact` | enum | yes | `new`, `improved`, `changed`, `fixed` |
| `isBreaking` | boolean | yes | Defaults to false |

Example entry shape for implementation:

```ts
{
  id: "sdk-js-publish-support",
  date: "2026-06-18",
  title: "JavaScript SDK publish helpers",
  summary: "The JavaScript SDK adds typed helpers for creating drafts, publishing posts, and reading delivery results.",
  category: "sdk",
  sdkVersions: [
    { packageName: "@unipost/sdk-js", version: "0.4.0", href: "https://www.npmjs.com/package/@unipost/sdk-js" }
  ],
  links: [
    { label: "SDK docs", href: "/docs/sdk" },
    { label: "Create Post API", href: "/docs/api/posts/create" }
  ],
  impact: "new",
  isBreaking: false
}
```

This example is a schema example, not a claim that this exact SDK version has shipped. Launch content must use verified release dates and verified package versions.

---

## 9. Initial Launch Content Requirements

The initial page should not launch empty. It should start with 5 to 8 verified major release entries.

Recommended seed themes to verify before publishing:

1. Developer docs and API reference improvements.
2. Hosted Connect / Connect Sessions improvements.
3. Platform Credentials or white-label connection flow improvements.
4. Public API metrics, logs, or delivery observability improvements.
5. TikTok analytics API or platform analytics release.
6. Inbox, comments, DMs, or webhooks capability release.
7. CLI, MCP, or SDK release if actually shipped.
8. Major platform support expansion such as Pinterest, Bluesky, Threads, YouTube, or Facebook support if release dates can be verified.

Content rule:

- Every launch entry must be traceable to a merged PR, docs page, release tag, package registry version, or dated internal release note.
- If a release date cannot be verified exactly, use a month-level display such as `June 2026`, but keep the underlying data structured.
- SDK version numbers must never be guessed. If the package version cannot be verified, omit the SDK pill for that entry.

---

## 10. Visual Direction

The page should feel like a developer product history, not a blog index.

Design principles:

- Quiet, precise, and information-dense enough to scan.
- Use table structure, timeline accents, and compact labels instead of large decorative cards.
- Keep the first screen useful: latest release plus visible history.
- Prefer neutral surfaces, crisp borders, and restrained accent color.
- Use monospace styling for SDK package names, versions, API paths, and dates.
- Avoid a generic three-card feature row.
- Avoid decorative gradients, oversized hero copy, and marketing filler.

Suggested visual pattern:

- Header section with asymmetric layout:
  - Left: title, lead, latest release.
  - Right: compact "release index" panel showing counts by area and latest SDK version if available.
- Main section:
  - Full-width release table with sticky-ish visual rhythm from date labels.
  - Category pills with one accent system.
  - SDK version pills in monospace.
- Mobile:
  - One-column stacked list with strong date/category hierarchy.

---

## 11. SEO And Metadata

Canonical URL:

```text
https://unipost.dev/changelog
```

Recommended metadata:

- Title: `UniPost Change Logs | Product Updates and SDK Releases`
- Description: `Track major UniPost product updates, API releases, SDK versions, platform support, and developer experience improvements.`
- Open Graph title: `UniPost Change Logs`
- Open Graph description: `Major product, API, SDK, and platform releases from UniPost.`

Sitemap:

- Add `/changelog` with a weekly or monthly change frequency.

Indexing:

- Page should be indexable.
- Release anchors should be linkable, for example `/changelog#sdk-js-publish-support`.

---

## 12. Accessibility Requirements

Navigation dropdown:

- Must work with keyboard.
- Must expose correct expanded/collapsed state.
- Must not rely on hover only.
- Escape should close the menu.
- Clicking outside should close the menu.
- Mobile should provide an accessible tap path to `Docs` and `Change Logs`.

Release table:

- Use semantic table markup on desktop where practical.
- If mobile uses stacked rows, preserve meaningful labels for screen readers.
- Links need descriptive labels.
- Category color must not be the only indicator.
- Focus states must be visible.

---

## 13. Analytics

Track these events if the existing analytics system supports public marketing events:

| Event | When |
| --- | --- |
| `developer_nav_opened` | User opens Developer dropdown |
| `developer_nav_docs_clicked` | User clicks Docs from the dropdown |
| `developer_nav_changelog_clicked` | User clicks Change Logs from the dropdown |
| `changelog_filter_selected` | User changes release category filter |
| `changelog_release_link_clicked` | User clicks a release docs/package/blog link |

If no public marketing analytics pattern exists, implementation can skip custom events and rely on page analytics.

---

## 14. Implementation Notes

Suggested implementation surfaces:

- `dashboard/src/components/marketing/nav.tsx`
  - Replace `Docs` nav item with accessible `Developer` dropdown.
  - Keep `Docs` as a dropdown item.
  - Add `Change Logs`.
- `dashboard/src/app/changelog/page.tsx`
  - New public page.
  - Reuse marketing shell and public site header.
  - Add route metadata.
- `dashboard/src/app/sitemap.ts`
  - Add `/changelog`.
- Optional:
  - Add `/change-logs` redirect to `/changelog`.
  - Update footer to include `Change Logs`.

The page should use a local typed data file or colocated constant for v1. If release entries become frequent, move to MDX or a lightweight content collection later.

---

## 15. Validation Plan

Local validation for implementation:

1. From `dashboard/`, run `npm run build`.
2. Because this touches public navigation and marketing routing, run dashboard regression tests if Playwright browsers are installed:

```bash
npm run test:regression:dashboard
```

Manual validation:

- Desktop:
  - Open landing page.
  - Confirm `Developer` dropdown opens and closes.
  - Confirm `Docs` navigates to `/docs`.
  - Confirm `Change Logs` navigates to `/changelog`.
  - Confirm `/changelog` renders the latest release and release table.
- Mobile:
  - Confirm nav remains usable.
  - Confirm release rows do not overflow horizontally.
- Keyboard:
  - Tab to Developer.
  - Open menu.
  - Navigate to both menu items.
  - Escape closes menu.
- SEO:
  - Confirm title, description, canonical URL, and sitemap entry.
- Content:
  - Confirm every published release entry has verified date, category, links, and SDK version when applicable.

Remote validation after pushing to `origin/dev`:

- Wait for the development deployment to finish.
- Test against `https://dev.unipost.dev`, not production.
- Verify the public landing nav and `https://dev.unipost.dev/changelog`.

---

## 16. Acceptance Criteria

1. Public landing nav shows `Developer` instead of direct `Docs`.
2. `Developer` dropdown contains `Docs` and `Change Logs`.
3. `/docs` remains reachable from the public nav.
4. `/changelog` is a public, indexable page.
5. The page includes a latest-release area and a release table/history list.
6. Release rows include date, title, summary, area, links, and SDK version pills when applicable.
7. SDK version numbers are displayed only when verified.
8. The page launches with 5 to 8 verified major releases.
9. Desktop and mobile layouts are polished and do not overflow.
10. Keyboard and screen-reader access for the dropdown and release links is handled.
11. Build passes.
12. After merge to `dev`, the dev deployment is validated at the development domain.

---

## 17. Product Recommendation

This change is worth doing.

The main product value is not just adding a new page. It reframes UniPost from "a docs-backed API product" into "a developer platform with visible momentum." For a product like UniPost, where trust matters before someone wires social publishing into their own app, a credible Change Logs page can reduce uncertainty.

The key is restraint. The page should not feel like marketing theater or a fake activity feed. It should be a concise public ledger of meaningful releases, especially API, SDK, platform, and reliability improvements. If maintained honestly, it becomes a trust asset for evaluation, sales, support, and existing integrators.
