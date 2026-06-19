"use client";

import Link from "next/link";
import { InfoBox, type ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key, or a Clerk session JWT for dashboard callers." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "Configured platform credential rows for the authenticated workspace." },
  { name: "data[].platform", type: "string", description: "Platform key for this credential row." },
  { name: "data[].client_id", type: "string", description: "Public client ID. The client_secret is never returned by this endpoint." },
  { name: "data[].created_at", type: "string", description: "Timestamp for when the credentials were uploaded or replaced." },
  { name: "meta.total", type: "number", description: "Total configured platform credential rows returned by the list endpoint." },
  { name: "meta.limit", type: "number", description: "Applied list limit." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED" or "INTERNAL_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized" or "internal_error".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/platform-credentials" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": [
    {
      "platform": "tiktok",
      "client_id": "aweme_client_key",
      "created_at": "2026-06-01T10:00:00Z"
    }
  ],
  "meta": { "total": 1, "limit": 1 },
  "request_id": "req_123"
}`,
  },
];

export default function ListPlatformCredentialsPage() {
  return (
    <SingleEndpointReferencePage
      breadcrumbItems={[
        { label: "API Reference", href: "/docs/api" },
        { label: "Platform Credentials", href: "/docs/api/platform-credentials/create" },
        { label: "List credentials" },
      ]}
      section="core"
      title="List platform credentials"
      description={
        <>
          List the platform OAuth apps configured for the authenticated workspace. Use this endpoint to audit which platforms have workspace-owned credentials before creating new Connect Sessions.
        </>
      }
      method="GET"
      path="/v1/platform-credentials"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    >
      <InfoBox>
        This endpoint returns only the public <code>client_id</code>. UniPost never returns <code>client_secret</code> after upload; rotate the credential in the platform developer portal and upload it again if the original secret is lost.
      </InfoBox>
      <InfoBox>
        Need setup steps? Start with the <Link href="/docs/platform-credentials">Platform Credentials guide</Link>.
      </InfoBox>
    </SingleEndpointReferencePage>
  );
}
