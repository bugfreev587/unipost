"use client";

import { ApiInlineLink, InfoBox, type ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key, or a Clerk session JWT for dashboard callers." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "platform", type: "string", description: "Platform key whose stored OAuth app credentials should be deleted." },
];

const RESPONSE_204_FIELDS: ApiFieldItem[] = [
  { name: "status", type: "204 No Content", description: "Credentials were deleted if they existed; no response body is returned." },
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
    code: `curl -X DELETE "https://api.unipost.dev/v1/platform-credentials/tiktok" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "text",
    label: "204",
    code: "No response body",
  },
];

export default function DeletePlatformCredentialsPage() {
  return (
    <SingleEndpointReferencePage
      breadcrumbItems={[
        { label: "API Reference", href: "/docs/api" },
        { label: "Platform Credentials", href: "/docs/api/platform-credentials/create" },
        { label: "Delete credentials" },
      ]}
      section="core"
      title="Delete platform credentials"
      description="Delete the stored OAuth app credentials for one upstream platform in the authenticated workspace."
      method="DELETE"
      path="/v1/platform-credentials/:platform"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "204", fields: RESPONSE_204_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    >
      <InfoBox>
        Deleting credentials affects future OAuth flows only. Existing connected accounts continue publishing with their stored tokens. Future <ApiInlineLink endpoint="POST /v1/connect/sessions" /> calls for this platform fail unless credentials are uploaded again or the session uses <code>allow_quickstart_creds=true</code>.
      </InfoBox>
    </SingleEndpointReferencePage>
  );
}
