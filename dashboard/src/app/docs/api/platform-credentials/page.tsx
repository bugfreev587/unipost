import Link from "next/link";
import type { ReactNode } from "react";
import {
  ApiAccordion,
  ApiEndpointCard,
  ApiFieldList,
  ApiInlineLink,
  ApiReferenceGrid,
  ApiReferencePage,
  CodeTabs,
  InfoBox,
  MethodBadge,
  type ApiFieldItem,
} from "../_components/doc-components";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key, or a Clerk session JWT for dashboard callers." },
];

const CREATE_BODY_FIELDS: ApiFieldItem[] = [
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

const DELETE_PATH_FIELDS: ApiFieldItem[] = [
  { name: "platform", type: "string", description: "Platform key whose stored OAuth app credentials should be deleted." },
];

const CREATE_RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "data.platform", type: "string", description: "Platform key that was created or replaced." },
  { name: "data.client_id", type: "string", description: "Public client ID that was saved. The client_secret is never returned." },
  { name: "data.created_at", type: "string", description: "Timestamp for the saved credential row." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const LIST_RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "Configured platform credential rows for the authenticated workspace." },
  { name: "data[].platform", type: "string", description: "Platform key for this credential row." },
  { name: "data[].client_id", type: "string", description: "Public client ID. The client_secret is never returned by this endpoint." },
  { name: "data[].created_at", type: "string", description: "Timestamp for when the credentials were uploaded or replaced." },
  { name: "meta.total", type: "number", description: "Total configured platform credential rows returned by the list endpoint." },
  { name: "meta.limit", type: "number", description: "Applied list limit." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const DELETE_RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "status", type: "204 No Content", description: "Credentials were deleted if they existed; no response body is returned." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED", "PLAN_FEATURE_NOT_AVAILABLE", "VALIDATION_ERROR", or "INTERNAL_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "plan_feature_not_available" or "validation_error".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const REQUEST_SNIPPETS = [
  {
    lang: "curl",
    label: "Create",
    code: `curl -X POST "https://api.unipost.dev/v1/platform-credentials" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform": "tiktok",
    "client_id": "aweme_client_key",
    "client_secret": "aweme_client_secret"
  }'`,
  },
  {
    lang: "curl",
    label: "List",
    code: `curl "https://api.unipost.dev/v1/platform-credentials" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "curl",
    label: "Delete",
    code: `curl -X DELETE "https://api.unipost.dev/v1/platform-credentials/tiktok" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
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
  {
    lang: "text",
    label: "204",
    code: "No response body",
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

function EndpointSection({
  method,
  path,
  title,
  children,
}: {
  method: "GET" | "POST" | "DELETE";
  path: string;
  title: string;
  children: ReactNode;
}) {
  return (
    <ApiEndpointCard method={method} path={path}>
      <div style={{ padding: "18px", borderBottom: "1px solid var(--docs-border)" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 11, flexWrap: "wrap", marginBottom: 8 }}>
          <MethodBadge method={method} />
          <code style={{ fontFamily: "var(--docs-mono)", fontSize: 14, color: "var(--docs-text)", overflowWrap: "anywhere" }}>{path}</code>
        </div>
        <h2 style={{ fontSize: 17, lineHeight: 1.35, margin: 0, color: "var(--docs-text)", fontWeight: 720 }}>{title}</h2>
      </div>
      <div style={{ padding: "18px", display: "grid", gap: 18 }}>
        {children}
      </div>
    </ApiEndpointCard>
  );
}

export default function PlatformCredentialsPage() {
  return (
    <ApiReferencePage
      breadcrumbItems={[
        { label: "API Reference", href: "/docs/api" },
        { label: "Platform Credentials" },
      ]}
      section="core"
      title="Platform Credentials"
      description={
        <>
          Upload, inspect, and remove workspace-owned OAuth app credentials. Platform Credentials are separate from Hosted Connect branding: credentials control the upstream platform app identity and quota, while <Link href="/docs/white-label">Hosted Connect</Link> controls UniPost&apos;s pre-OAuth page. See the <Link href="/docs/platform-credentials">Platform Credentials guide</Link> for platform setup steps.
        </>
      }
    >
      <ApiReferenceGrid
        left={
          <div style={{ display: "grid", gap: 16 }}>
            <InfoBox>
              Open <strong style={{ color: "var(--docs-text)" }}>Developer - Platform Credentials</strong> in the dashboard for manual setup, or use these endpoints for scripts and admin tooling. Basic supports one platform credential slot; Growth and Team support all supported platforms. Connect Sessions can use UniPost&apos;s shared OAuth app only when <code>allow_quickstart_creds</code> is true and no workspace credential exists.
            </InfoBox>

            <EndpointSection method="POST" path="/v1/platform-credentials" title="Upload credentials">
              <ApiFieldList title="Authorization" items={AUTH_FIELDS} />
              <ApiFieldList title="Request Body" items={CREATE_BODY_FIELDS} />
              <ApiAccordion title="201 Response Body" defaultOpen>
                <ApiFieldList items={CREATE_RESPONSE_FIELDS} />
              </ApiAccordion>
              <ApiAccordion title="401 / 402 / 422 / 500 Error Body">
                <ApiFieldList items={ERROR_FIELDS} />
              </ApiAccordion>
              <InfoBox>
                A successful upload replaces any previous credentials for the same platform. New Connect Sessions for that platform will use these credentials unless the session explicitly allows the shared quickstart fallback.
              </InfoBox>
            </EndpointSection>

            <EndpointSection method="GET" path="/v1/platform-credentials" title="List configured platforms">
              <ApiFieldList title="Authorization" items={AUTH_FIELDS} />
              <ApiAccordion title="200 Response Body" defaultOpen>
                <ApiFieldList items={LIST_RESPONSE_FIELDS} />
              </ApiAccordion>
              <ApiAccordion title="401 / 500 Error Body">
                <ApiFieldList items={ERROR_FIELDS} />
              </ApiAccordion>
              <InfoBox>
                This endpoint returns the public <code>client_id</code> only. There is no API that reads back <code>client_secret</code>.
              </InfoBox>
            </EndpointSection>

            <EndpointSection method="DELETE" path="/v1/platform-credentials/:platform" title="Remove credentials">
              <ApiFieldList title="Authorization" items={AUTH_FIELDS} />
              <ApiFieldList title="Path Params" items={DELETE_PATH_FIELDS} />
              <ApiAccordion title="204 Response" defaultOpen>
                <ApiFieldList items={DELETE_RESPONSE_FIELDS} />
              </ApiAccordion>
              <ApiAccordion title="401 / 500 Error Body">
                <ApiFieldList items={ERROR_FIELDS} />
              </ApiAccordion>
              <InfoBox>
                Deleting credentials affects future OAuth flows only. Existing connected accounts continue publishing with their stored tokens. Future <ApiInlineLink endpoint="POST /v1/connect/sessions" /> calls for this platform fail unless credentials are uploaded again or the session uses <code>allow_quickstart_creds=true</code>.
              </InfoBox>
            </EndpointSection>
          </div>
        }
        right={
          <div style={{ display: "grid", gap: 14, alignContent: "start" }}>
            <CodeTabs snippets={REQUEST_SNIPPETS} />
            <CodeTabs snippets={RESPONSE_SNIPPETS} />
          </div>
        }
      />
    </ApiReferencePage>
  );
}
