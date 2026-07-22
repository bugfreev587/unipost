# Inbox App Integration Guide Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct the deployed Inbox API overview and publish a production integration guide that lets app owners connect social accounts, isolate every managed user's Inbox operations, relay real-time events safely, and expose a separate owner/admin aggregate.

**Architecture:** Keep the work documentation-only. Add one server-rendered Next.js guide that reuses `DocsPage`, `DocsCodeTabs`, `DocsTable`, and `ApiInlineLink`; extend the existing Inbox overview rather than creating duplicate endpoint pages; wire the guide into the existing sidebar-derived keyword search, AI search index, Guides index, and sitemap. Enforce the security-critical contract with a source test before each implementation slice.

**Tech Stack:** Next.js 16 App Router, React 19 server components, TypeScript, existing docs components and Tailwind/CSS shell, Node.js built-in test runner.

---

### Task 1: Add the failing Inbox documentation contract

**Files:**
- Create: `dashboard/tests/inbox-integration-guide-source.test.mjs`

- [ ] **Step 1: Write the failing source test**

Create a test file that reads missing files as an empty string so the RED state is an assertion failure rather than an unhandled `ENOENT`. The test must assert:

```js
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import test from "node:test";

const root = process.cwd();

async function source(path) {
  try {
    return await readFile(join(root, path), "utf8");
  } catch (error) {
    if (error?.code === "ENOENT") return "";
    throw error;
  }
}

const guidePath = "src/app/docs/guides/inbox-integration/page.tsx";
const overviewPath = "src/app/docs/api/inbox/page.tsx";

test("Inbox API overview matches the deployed router and auth contract", async () => {
  const overview = await source(overviewPath);
  for (const endpoint of [
    "GET /v1/inbox",
    "GET /v1/inbox/unread-count",
    "GET /v1/inbox/{id}",
    "POST /v1/inbox/{id}/read",
    "POST /v1/inbox/mark-all-read",
    "POST /v1/inbox/{id}/reply",
    "POST /v1/inbox/{id}/thread-state",
    "GET /v1/inbox/{id}/media-context",
    "POST /v1/inbox/sync",
    "GET /v1/inbox/x-outbound-operations/{requestID}",
    "GET /v1/inbox/ws",
  ]) assert.match(overview, new RegExp(endpoint.replaceAll("/", "\\/")));
  assert.match(overview, /selected scope/i);
  assert.match(overview, /INBOX_SCOPE_LOOKUP_FAILED/);
  assert.match(overview, /scope[^\n]{0,160}before[^\n]{0,160}plan/i);
  assert.match(overview, /Clerk[^\n]{0,160}token[^\n]{0,160}query/i);
  assert.match(overview, /API key[^\n]{0,160}Authorization[^\n]{0,160}header/i);
  assert.match(overview, /\/docs\/guides\/inbox-integration/);
});

test("Inbox integration guide derives stable managed-user scope on the server", async () => {
  const guide = await source(guidePath);
  assert.match(guide, /https:\/\/unipost\.dev\/docs\/guides\/inbox-integration/);
  assert.match(guide, /app_usr_7f4c91/);
  assert.match(guide, /external_user_id/);
  assert.match(guide, /opaque/i);
  assert.match(guide, /immutable/i);
  assert.match(guide, /email[^\n]{0,120}(?:mutable|primary)/i);
  assert.match(guide, /authenticated app user/i);
  assert.match(guide, /server-side/i);
  assert.match(guide, /inbox_scope=managed_user/);
  assert.match(guide, /inbox_scope=workspace/);
  assert.doesNotMatch(guide, /youtube_comment/);
});

test("Inbox integration guide covers connect, reads, writes, realtime, errors, and A-B acceptance", async () => {
  const guide = await source(guidePath);
  for (const endpoint of [
    "POST /v1/connect/sessions",
    "GET /v1/inbox",
    "GET /v1/inbox/unread-count",
    "GET /v1/inbox/{id}",
    "POST /v1/inbox/{id}/read",
    "POST /v1/inbox/mark-all-read",
    "POST /v1/inbox/{id}/reply",
    "POST /v1/inbox/{id}/thread-state",
    "POST /v1/inbox/sync",
    "GET /v1/inbox/ws",
  ]) assert.match(guide, new RegExp(endpoint.replaceAll("/", "\\/")));
  assert.match(guide, /npm install ws/);
  assert.match(guide, /Authorization[^\n]{0,160}Bearer/);
  assert.match(guide, /browser[^\n]{0,180}(?:never|must not)[^\n]{0,180}API key/i);
  assert.match(guide, /default[^\n]{0,80}50[^\n]{0,80}(?:cap|maximum)[^\n]{0,80}500/i);
  assert.match(guide, /INBOX_SCOPE_LOOKUP_FAILED/);
  assert.match(guide, /bounded exponential backoff/i);
  assert.match(guide, /ownership conflict/i);
  assert.match(guide, /409/);
  assert.match(guide, /x_backfill/);
  assert.match(guide, /confirmation_token/);
  assert.match(guide, /X Credits/);
  assert.match(guide, /synthetic user A/i);
  assert.match(guide, /synthetic user B/i);
  assert.match(guide, /cross-scope/i);
  assert.match(guide, /remove[^\n]{0,100}synthetic/i);
});

test("Inbox integration guide is discoverable everywhere", async () => {
  const [guideIndex, shell, sitemap, aiIndex] = await Promise.all([
    source("src/app/docs/guides/page.tsx"),
    source("src/app/docs/_components/docs-shell.tsx"),
    source("src/app/sitemap.ts"),
    source("src/lib/docs-ai-search-index.ts"),
  ]);
  for (const corpus of [guideIndex, shell, sitemap, aiIndex]) {
    assert.match(corpus, /\/docs\/guides\/inbox-integration/);
  }
  assert.match(shell, /Inbox Guides/);
  assert.match(aiIndex, /managed user[^\n]{0,200}inbox|inbox[^\n]{0,200}managed user/i);
});
```

- [ ] **Step 2: Run the test and verify RED**

Run from `dashboard/`:

```bash
node --test tests/inbox-integration-guide-source.test.mjs
```

Expected: FAIL because the new guide does not exist and the overview is missing several router endpoints and accuracy notes.

- [ ] **Step 3: Commit the verified failing contract**

```bash
git add dashboard/tests/inbox-integration-guide-source.test.mjs
git commit -m "test(docs): define inbox integration guide contract"
```

### Task 2: Correct the Inbox API overview

**Files:**
- Modify: `dashboard/src/app/docs/api/inbox/page.tsx`
- Test: `dashboard/tests/inbox-integration-guide-source.test.mjs`
- Test: `dashboard/tests/x-inbox-docs-source.test.mjs`

- [ ] **Step 1: Expand the endpoint inventory and selected-scope wording**

Add the five missing router entries to `ENDPOINT_FIELDS` and replace “current workspace” with “selected Inbox scope” where the response can be managed-user scoped:

```tsx
{ name: "POST /v1/inbox/{id}/read", type: "write", description: "Mark one item in the selected Inbox scope as read." },
{ name: "POST /v1/inbox/mark-all-read", type: "write", description: "Mark every item in the selected Inbox scope as read." },
{ name: "GET /v1/inbox/{id}/media-context", type: "read", description: "Fetch scoped media context for a supported Inbox item." },
{ name: "GET /v1/inbox/x-outbound-operations/{requestID}", type: "read", description: "Inspect the durable outcome of an X reply operation." },
{ name: "GET /v1/inbox/ws", type: "realtime", description: "Subscribe to events for one explicit Inbox scope." },
```

- [ ] **Step 2: Publish authentication, ordering, error, and guide-link notes**

Add prose next to the fields, not a new component:

```tsx
<p>
  Customer backends authenticate WebSocket upgrades with the API key in the
  <code> Authorization</code> header. UniPost&apos;s Dashboard uses its Clerk session
  token in the <code>token</code> query field. Never place an API key in a URL.
</p>
<p>
  Authentication and explicit scope resolution run before the Inbox plan gate.
  A malformed or unauthorized scope therefore returns before a possible 402.
</p>
<p>
  For an end-to-end server integration, follow the <Link href="/docs/guides/inbox-integration">Inbox integration guide</Link>.
</p>
```

Extend the public error list with `INBOX_SCOPE_LOOKUP_FAILED` and describe it as a transient pre-handler `500` that permits retrying the same request with bounded exponential backoff. Do not label every `5xx` retryable.

- [ ] **Step 3: Run focused tests and verify GREEN for the overview slice**

```bash
node --test tests/inbox-integration-guide-source.test.mjs tests/x-inbox-docs-source.test.mjs
```

Expected: the overview test passes; the guide/discovery tests still fail because those files are not implemented.

- [ ] **Step 4: Commit the overview correction**

```bash
git add dashboard/src/app/docs/api/inbox/page.tsx
git commit -m "docs(inbox): correct API overview contract"
```

### Task 3: Build the production app-owner integration guide

**Files:**
- Create: `dashboard/src/app/docs/guides/inbox-integration/page.tsx`
- Test: `dashboard/tests/inbox-integration-guide-source.test.mjs`

- [ ] **Step 1: Create the server-rendered page and canonical metadata**

Use only installed components and dependencies:

```tsx
import type { Metadata } from "next";
import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";
import { ApiInlineLink } from "../../api/_components/doc-components";

export const metadata: Metadata = {
  alternates: { canonical: "https://unipost.dev/docs/guides/inbox-integration" },
};
```

The page must use `DocsPage` with `eyebrow="Inbox Guides"`, title `Integrate UniPost Inbox into your app`, the existing guide-redesign class, and badges for server-only API keys, managed-user scope, owner/admin aggregate, and production verification.

- [ ] **Step 2: Add the identity and Connect Session workflow**

Document `app_usr_7f4c91` as the same opaque, immutable identifier across connection and Inbox access. The Connect example must call `POST /v1/connect/sessions` from the app backend and must derive the value from the authenticated app session:

```ts
type AuthenticatedAppUser = { id: "app_usr_7f4c91"; role: "user" | "owner" | "admin" };

async function createConnectSession(appUser: AuthenticatedAppUser) {
  const response = await fetch("https://api.unipost.dev/v1/connect/sessions", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${process.env.UNIPOST_API_KEY}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      platform: "instagram",
      profile_id: "pr_app_inbox",
      external_user_id: appUser.id,
      return_url: "https://app.example.com/integrations/complete",
    }),
  });
  if (!response.ok) throw new Error(`Connect Session failed: ${response.status}`);
  return response.json();
}
```

Explain that mutable email is optional reconciliation data, not the primary identifier. Document the hosted Connect HTTP `409` ownership-conflict message and instruct the app to resolve ownership rather than auto-reassign.

- [ ] **Step 3: Add one reusable managed-user backend boundary**

Show a TypeScript helper that receives an already authenticated app user, appends `inbox_scope=managed_user` and the derived `external_user_id`, reads `UNIPOST_API_KEY` only from the server environment, and accepts only a relative Inbox path controlled by server code. Explicitly state that browser query/body fields never choose UniPost scope.

- [ ] **Step 4: Add scoped read and write examples**

Cover and link these operations:

```text
GET /v1/inbox?limit=50
GET /v1/inbox/unread-count
GET /v1/inbox/{id}
POST /v1/inbox/{id}/read
POST /v1/inbox/mark-all-read
POST /v1/inbox/{id}/reply
POST /v1/inbox/{id}/thread-state
POST /v1/inbox/sync
```

State that `limit` defaults to 50, caps at 500, and is not cursor pagination. Every item-level operation must preserve scoped `404` without revealing whether an ID belongs to another managed user. Separate ordinary non-X sync from `x_backfill`; document `estimated_x_credits`, `confirmation_required`, and repeating the exact request with `confirmation_token`.

- [ ] **Step 5: Add backend WebSocket relay and aggregate route**

Include `npm install ws` and a backend-only example using:

```ts
import WebSocket from "ws";

const url = new URL("wss://api.unipost.dev/v1/inbox/ws");
url.searchParams.set("inbox_scope", "managed_user");
url.searchParams.set("external_user_id", appUser.id);
const socket = new WebSocket(url, {
  headers: { Authorization: `Bearer ${process.env.UNIPOST_API_KEY}` },
});
```

Relay only to the app channel already authorized for `appUser.id`. On reconnect, refresh list and unread count. For aggregate access, require the app session role to be owner/admin and use only `inbox_scope=workspace` with no `external_user_id`; note the UniPost key must be creator-bound to a current UniPost owner/admin.

- [ ] **Step 6: Add the error table, security checklist, and synthetic A/B acceptance**

Use `DocsTable` for 400/401/402/403/404/409/500 behavior. Explain scope-before-plan ordering and restrict automatic retry guidance to `INBOX_SCOPE_LOOKUP_FAILED` with bounded exponential backoff. Add a checklist covering API-key secrecy, authenticated identity derivation, separate aggregate handlers, private-body logging, same idempotency key after uncertain X writes, and fail-closed behavior.

The acceptance section must name synthetic user A and synthetic user B and verify:

```text
A list excludes B; B list excludes A.
Cross-scope get, read, reply, and thread-state return 404.
Workspace aggregate sees both.
A's real-time channel does not receive B's exact event.
All synthetic fixtures are removed and residual counts are zero.
```

- [ ] **Step 7: Run the focused test**

```bash
node --test tests/inbox-integration-guide-source.test.mjs
```

Expected: guide behavior tests pass; discovery test still fails until Task 4.

- [ ] **Step 8: Commit the guide**

```bash
git add dashboard/src/app/docs/guides/inbox-integration/page.tsx
git commit -m "docs(inbox): add production app integration guide"
```

### Task 4: Wire guide discovery and search

**Files:**
- Modify: `dashboard/src/app/docs/guides/page.tsx`
- Modify: `dashboard/src/app/docs/_components/docs-shell.tsx`
- Modify: `dashboard/src/app/sitemap.ts`
- Modify: `dashboard/src/lib/docs-ai-search-index.ts`
- Test: `dashboard/tests/inbox-integration-guide-source.test.mjs`
- Test: `dashboard/tests/docs-ai-search-evals.test.mjs`

- [ ] **Step 1: Put Inbox integration first on the Guides index**

Add this card as the first item inside `.docs-grid`:

```tsx
<Link href="/docs/guides/inbox-integration" className="docs-card" style={{ textDecoration: "none" }}>
  <div className="docs-card-title">Inbox integration</div>
  <p>Connect each app user, isolate reads and writes, relay real-time events, and build an owner/admin aggregate.</p>
</Link>
```

- [ ] **Step 2: Add a dedicated Inbox Guides sidebar group**

Insert before Analytics Guides:

```ts
{
  title: "Inbox Guides",
  description: "Connect app users and keep every managed user's conversations isolated.",
  items: [
    { label: "Inbox integration", href: "/docs/guides/inbox-integration" },
    { label: "X comments", href: "/docs/guides/x/comments" },
    { label: "X direct messages", href: "/docs/guides/x/direct-messages" },
    { label: "Reconnect X permissions", href: "/docs/guides/x/reconnect-permissions" },
  ],
},
```

Remove those three workflow entries from the existing X Guides group, leaving X Credits there, to avoid duplicate sidebar search records.

- [ ] **Step 3: Add sitemap coverage**

Add `/docs/guides/inbox-integration` to `staticRoutes` with monthly change frequency and priority `0.7`.

- [ ] **Step 4: Add an AI retrieval evaluation before implementation and verify it fails**

Add this test to `docs-ai-search-evals.test.mjs` before adding the chunks:

```js
test("managed-user Inbox integration questions rank the production guide", () => {
  const { answer } = answerFor("How do I isolate each managed user's Inbox in my app?");
  assert.equal(answer.sources[0]?.path, "/docs/guides/inbox-integration");
  assert.match(answer.answer, /external_user_id/);
  assert.match(answer.answer, /managed_user/);
  assert.match(answer.answer, /server/i);
});
```

Run it, confirm the expected ranking failure, then add the index chunks and rerun.

- [ ] **Step 5: Add grounded AI search coverage**

Add two non-feature-gated `DOCS_AI_INDEX` chunks for the managed-user integration and owner/admin aggregate. Both must include task-shaped tags and endpoint aliases; the managed-user chunk must explain stable `external_user_id`, server-only key handling, and explicit scope, while the aggregate chunk must explain app role enforcement plus creator-bound UniPost owner/admin authorization.

- [ ] **Step 6: Run discovery and search tests**

```bash
node --test tests/inbox-integration-guide-source.test.mjs tests/docs-ai-search-evals.test.mjs tests/docs-ai-search-implementation-source.test.mjs
```

Expected: PASS.

- [ ] **Step 7: Commit discovery changes**

```bash
git add dashboard/src/app/docs/guides/page.tsx dashboard/src/app/docs/_components/docs-shell.tsx dashboard/src/app/sitemap.ts dashboard/src/lib/docs-ai-search-index.ts dashboard/tests/docs-ai-search-evals.test.mjs
git commit -m "docs(inbox): add guide discovery and search"
```

### Task 5: Complete local verification and Preview Acceptance

**Files:**
- Modify only if verification exposes a defect: files from Tasks 1-4

- [ ] **Step 1: Run all affected source tests**

```bash
node --test \
  tests/inbox-integration-guide-source.test.mjs \
  tests/x-inbox-docs-source.test.mjs \
  tests/docs-ai-search-evals.test.mjs \
  tests/docs-ai-search-implementation-source.test.mjs \
  tests/api-reference-overview-source.test.mjs \
  tests/seo-public-pages-source.test.mjs
```

Expected: PASS with no skipped or cancelled tests.

- [ ] **Step 2: Run the production Dashboard build**

```bash
npm run build
```

Expected: Next.js build succeeds and lists `/docs/guides/inbox-integration` as a generated public route.

- [ ] **Step 3: Inspect the local rendered pages**

Start the built app and verify desktop/mobile rendering for:

```text
/docs/api/inbox
/docs/guides
/docs/guides/inbox-integration
```

Check the navigation group, breadcrumb, table overflow, code tabs, headings/TOC, canonical, cross-links, dark theme, and absence of horizontal overflow. Authenticated Clerk Dashboard regression remains deferred because no authenticated Dashboard code is touched.

- [ ] **Step 4: Audit branch contents before push**

```bash
git log --oneline origin/staging..HEAD
git diff --name-status origin/staging...HEAD
git diff --check origin/staging...HEAD
```

Expected: only the approved design/plan, Inbox docs, source tests, and discovery files are unique to the branch.

- [ ] **Step 5: Push the owned branch and open a Draft PR to staging**

Push only `hotfix-inbox-comments-idor`, open a Draft PR targeting `staging`, and record its exact head SHA. Do not update `staging` directly.

- [ ] **Step 6: Complete Preview Acceptance on the exact head SHA**

Wait for GitHub CI, Railway PR Environment, Vercel Preview, and deployed regression. Open the Vercel Preview and repeat the three public-page checks at desktop and mobile widths. Any failed, skipped, timed-out, cancelled, or wrong-SHA result is a hard stop.

- [ ] **Step 7: Promote only after every gate passes**

Audit the unique commits/files again, mark the PR ready, merge to `staging`, wait for the staging deployment, and verify the same pages on `https://staging.unipost.dev`. Then promote `staging` to `main`, verify production, and finally audit and synchronize the complete staging state back to `dev` as explicitly approved. Stop on any unidentified staging content or merge conflict.
