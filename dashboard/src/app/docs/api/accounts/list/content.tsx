"use client";

import {
  Breadcrumbs, EndpointHeader, DocSection, ParamTable, CodeTabs, ResponseBlock,
  ErrorTable, RelatedEndpoints, InfoBox,
  type ParamRow, type ErrorCodeRow,
} from "../../_components/doc-components";

const QUERY_PARAMS: ParamRow[] = [
  { name: "platform", type: "string", required: false, description: 'Filter by platform: "twitter", "linkedin", "instagram", "threads", "tiktok", "youtube", "bluesky".' },
  { name: "external_user_id", type: "string", required: false, description: "Filter by the external_user_id set during a Connect flow. Useful for looking up accounts onboarded by a specific end user." },
];

const ERRORS: ErrorCodeRow[] = [
  { code: "UNAUTHORIZED", http: 401, description: "Missing or invalid API key." },
  { code: "INTERNAL_ERROR", http: 500, description: "Server error." },
];

const SNIPPETS = [
  { lang: "js", label: "JavaScript", code: `const response = await fetch(
  'https://api.unipost.dev/v1/social-accounts',
  {
    headers: {
      'Authorization': 'Bearer up_live_xxxx',
    },
  }
);

const { data } = await response.json();
for (const account of data) {
  console.log(account.id, account.platform, account.status);
}` },
  { lang: "python", label: "Python", code: `import requests

response = requests.get(
    'https://api.unipost.dev/v1/social-accounts',
    headers={'Authorization': 'Bearer up_live_xxxx'}
)

for account in response.json()['data']:
    print(account['id'], account['platform'], account['status'])` },
  { lang: "curl", label: "cURL", code: `curl https://api.unipost.dev/v1/social-accounts \\
  -H "Authorization: Bearer up_live_xxxx"

# Filter by platform
curl "https://api.unipost.dev/v1/social-accounts?platform=instagram" \\
  -H "Authorization: Bearer up_live_xxxx"

# Filter by external user (Connect flow)
curl "https://api.unipost.dev/v1/social-accounts?external_user_id=user_abc" \\
  -H "Authorization: Bearer up_live_xxxx"` },
];

const RESPONSE = `{
  "data": [
    {
      "id": "sa_instagram_123",
      "platform": "instagram",
      "account_name": "magicxiaobo416",
      "account_avatar_url": "https://...",
      "status": "active",
      "connection_type": "byo",
      "connected_at": "2026-04-02T10:00:00Z",
      "external_user_id": null
    },
    {
      "id": "sa_linkedin_456",
      "platform": "linkedin",
      "account_name": "Xiaobo Yu",
      "account_avatar_url": "https://...",
      "status": "active",
      "connection_type": "managed",
      "connected_at": "2026-04-05T14:30:00Z",
      "external_user_id": "user_abc"
    }
  ]
}`;

const RESPONSE_FIELDS: ParamRow[] = [
  { name: "id", type: "string", required: false, description: "Social account ID. Use this as account_id in POST /v1/social-posts." },
  { name: "platform", type: "string", required: false, description: "Platform name: twitter, linkedin, instagram, threads, tiktok, youtube, bluesky." },
  { name: "account_name", type: "string?", required: false, description: "Human-readable handle or display name from the platform." },
  { name: "status", type: "string", required: false, description: '"active" (ready to post), "reconnect_required" (token expired, needs re-auth).' },
  { name: "connection_type", type: "string", required: false, description: '"byo" (White-label — your own credentials) or "managed" (connected via UniPost Connect flow).' },
  { name: "connected_at", type: "string", required: false, description: "ISO 8601 timestamp when the account was connected." },
  { name: "external_user_id", type: "string?", required: false, description: "Your identifier for the end user, set during a Connect session. Null for BYO accounts." },
];

export function ListAccountsContent() {
  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify({
        "@context": "https://schema.org", "@type": "TechArticle",
        name: "UniPost API — GET /v1/social-accounts",
        description: "List connected social media accounts",
        url: "https://unipost.dev/docs/api/accounts/list",
        author: { "@type": "Organization", name: "UniPost" }, dateModified: "2026-04-09",
      })}} />

      <Breadcrumbs items={[
        { label: "Docs", href: "/docs" },
        { label: "API Reference" },
        { label: "Accounts" },
        { label: "List Accounts" },
      ]} />

      <EndpointHeader
        method="GET"
        path="/v1/social-accounts"
        description="List all connected social media accounts in the current workspace. Returns active accounts by default; disconnected accounts are excluded unless queried by ID."
        badges={["Requires Auth", "Rate Limited"]}
      />

      <DocSection id="overview" title="Overview">
        <p style={{ fontSize: 14.5, color: "var(--docs-text-soft)", lineHeight: 1.7, marginBottom: 12 }}>
          After connecting social accounts (either via the dashboard or the Connect flow), this endpoint returns them with their platform, status, and the <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>id</code> you need for publishing.
        </p>
        <p style={{ fontSize: 14.5, color: "var(--docs-text-soft)", lineHeight: 1.7 }}>
          Filter by <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>platform</code> to list only Instagram accounts, or by <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>external_user_id</code> to find accounts onboarded by a specific end user via Connect.
        </p>
      </DocSection>

      <DocSection id="authentication" title="Authentication">
        <div style={{ background: "var(--docs-bg-muted)", border: "1px solid var(--docs-border)", borderRadius: 10, padding: "18px 22px" }}>
          <code style={{ fontSize: 14, fontFamily: "var(--docs-mono)", color: "var(--docs-text)" }}>Authorization: Bearer up_live_xxxx</code>
        </div>
      </DocSection>

      <DocSection id="request" title="Request">
        <div style={{ fontSize: 13, fontFamily: "var(--docs-mono)", color: "var(--docs-text-muted)", marginBottom: 16 }}>
          <span style={{ color: "var(--docs-accent)", fontWeight: 700 }}>GET</span>{" "}
          <span style={{ color: "var(--docs-text)" }}>https://api.unipost.dev/v1/social-accounts</span>
        </div>
        <ParamTable params={QUERY_PARAMS} title="Query Parameters" />
      </DocSection>

      <DocSection id="examples" title="Examples">
        <CodeTabs snippets={SNIPPETS} />
      </DocSection>

      <DocSection id="response" title="Response">
        <ResponseBlock title="200 — Success" code={RESPONSE} />
        <ParamTable params={RESPONSE_FIELDS} title="Response fields" />
        <InfoBox>
          <strong style={{ color: "var(--docs-link)" }}>connection_type explained</strong><br />
          <code>byo</code> = White-label (your own platform credentials, OAuth shows your app name).<br />
          <code>managed</code> = Connected via UniPost Connect flow (end-user OAuth through UniPost&apos;s hosted page).
        </InfoBox>
      </DocSection>

      <DocSection id="errors" title="Error Codes">
        <ErrorTable errors={ERRORS} />
      </DocSection>

      <DocSection id="related" title="Related Endpoints">
        <RelatedEndpoints items={[
          { method: "POST", path: "/v1/social-posts", label: "Create post", href: "/docs/api/posts/create" },
          { method: "GET", path: "/v1/social-accounts/:id/health", label: "Account health", href: "/docs/api/accounts/health" },
          { method: "POST", path: "/v1/connect/sessions", label: "Create Connect session", href: "/docs/api/connect/sessions" },
          { method: "DELETE", path: "/v1/social-accounts/:id", label: "Disconnect account", href: "/docs/api/accounts/list" },
        ]} />
      </DocSection>

      <div style={{ marginTop: 48, paddingTop: 24, borderTop: "1px solid var(--docs-border)", fontSize: 13, color: "var(--docs-text-faint)" }}>
        <a href="/docs" style={{ color: "var(--docs-link)", textDecoration: "none" }}>&larr; View full docs</a>
        <span style={{ margin: "0 12px" }}>|</span>
        <span>Last updated: April 2026 &middot; API v1</span>
      </div>
    </>
  );
}
