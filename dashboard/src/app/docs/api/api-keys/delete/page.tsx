"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Clerk session", type: "Browser session", meta: "Cookie/session", description: "Dashboard-authenticated route. This endpoint does not use a Bearer API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "workspace_id", type: "string", description: "Workspace that owns the API key." },
  { name: "key_id", type: "string", description: "API key record ID to revoke." },
];

const RESPONSE_204_FIELDS: ApiFieldItem[] = [
  { name: "status", type: "204 No Content", description: "The API key was revoked successfully and no response body is returned." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED", "NOT_FOUND", or "INTERNAL_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized", "not_found", or "internal_error".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X DELETE "https://api.unipost.dev/v1/workspaces/ws_123/api-keys/key_123" \\
  -H "Cookie: __session=<clerk-session-cookie>"`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "404",
    code: `{
  "error": {
    "code": "NOT_FOUND",
    "normalized_code": "not_found",
    "message": "API key not found"
  },
  "request_id": "req_123"
}`,
  },
];

export default function DeleteApiKeyPage() {
  return (
    <SingleEndpointReferencePage
      section="core"
      title="Delete API key"
      description="Revokes one API key from a workspace in the dashboard. After deletion, the key can no longer authenticate requests."
      method="DELETE"
      path="/v1/workspaces/:workspace_id/api-keys/:key_id"
      requestSections={[
        { title: "Authentication", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "204", fields: RESPONSE_204_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
