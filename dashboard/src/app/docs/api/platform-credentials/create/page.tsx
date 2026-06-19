"use client";

import Link from "next/link";
import { ApiInlineLink, InfoBox, type ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key, or a Clerk session JWT for dashboard callers." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  {
    name: "platform",
    type: "string",
    description: (
      <>
        Platform key for the upstream OAuth app.
        <br />
        Values: <code>facebook</code>, <code>instagram</code>, <code>linkedin</code>, <code>pinterest</code>, <code>tiktok</code>, <code>youtube</code>, or <code>twitter</code>.
      </>
    ),
  },
  { name: "client_id", type: "string", description: "Client ID, App ID, or Client Key copied from the platform developer portal." },
  { name: "client_secret", type: "string", description: "Client secret copied from the platform developer portal. UniPost stores it encrypted at rest and never returns client_secret from read endpoints." },
];

const RESPONSE_201_FIELDS: ApiFieldItem[] = [
  { name: "data.platform", type: "string", description: "Platform key that was created or replaced." },
  { name: "data.client_id", type: "string", description: "Public client ID that was saved. The client_secret is never returned." },
  { name: "data.created_at", type: "string", description: "Timestamp for the saved credential row." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED", "PLAN_FEATURE_NOT_AVAILABLE", "VALIDATION_ERROR", or "INTERNAL_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "plan_feature_not_available" or "validation_error".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/platform-credentials" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform": "tiktok",
    "client_id": "aweme_client_key",
    "client_secret": "aweme_client_secret"
  }'`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "201",
    code: `{
  "data": {
    "platform": "tiktok",
    "client_id": "aweme_client_key",
    "created_at": "2026-06-01T10:00:00Z"
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "402",
    code: `{
  "error": {
    "code": "PLAN_FEATURE_NOT_AVAILABLE",
    "normalized_code": "plan_feature_not_available",
    "message": "White-label credentials require the Basic plan or higher - upgrade at unipost.dev/pricing"
  },
  "request_id": "req_123"
}`,
  },
];

export default function CreatePlatformCredentialsPage() {
  return (
    <SingleEndpointReferencePage
      breadcrumbItems={[
        { label: "API Reference", href: "/docs/api" },
        { label: "Platform Credentials", href: "/docs/api/platform-credentials/create" },
        { label: "Upload credentials" },
      ]}
      section="core"
      title="Upload platform credentials"
      description={
        <>
          Upload or replace workspace-owned OAuth app credentials for one upstream platform. Platform Credentials are separate from Hosted Connect branding: credentials control the platform app identity and quota used by future <ApiInlineLink endpoint="POST /v1/connect/sessions" /> OAuth flows, while <Link href="/docs/white-label">Hosted Connect</Link> controls the pre-OAuth branding layer.
        </>
      }
      method="POST"
      path="/v1/platform-credentials"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "201", fields: RESPONSE_201_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "402", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    >
      <InfoBox>
        A successful upload replaces any previous credentials for the same platform. Basic supports one platform credential slot; Growth and Team support all supported platforms. See the <Link href="/docs/platform-credentials">Platform Credentials guide</Link> for platform setup steps.
      </InfoBox>
    </SingleEndpointReferencePage>
  );
}
