"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Clerk session", type: "Browser session", meta: "Cookie/session", description: "Dashboard-authenticated route. This endpoint does not use a Bearer API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "workspace_id", type: "string", description: "Workspace that will own the new API key." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "name", type: "string", description: "Human-readable label for the key." },
  { name: "environment?", type: '"production" | "test"', description: 'Defaults to "production" when omitted.' },
  { name: "expires_at?", type: "string", description: "Optional RFC3339 expiration timestamp." },
];

const RESPONSE_201_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "API key record ID." },
  { name: "name", type: "string", description: "Human-readable key label." },
  { name: "key", type: "string", description: "Full plaintext API key. Returned only once at creation time." },
  { name: "prefix", type: "string", description: "Safe prefix for future display." },
  { name: "environment", type: "string", description: 'Either "production" or "test".' },
  { name: "created_at", type: "string", description: "Creation timestamp." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "VALIDATION_ERROR", "UNAUTHORIZED", or "NOT_FOUND".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "validation_error", "unauthorized", or "not_found".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/workspaces/ws_123/api-keys" \\
  -H "Cookie: __session=<clerk-session-cookie>" \\
  -H "Content-Type: application/json" \\
  -d '{
    "name": "Production backend",
    "environment": "production"
  }'`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "201",
    code: `{
  "data": {
    "id": "key_123",
    "name": "Production backend",
    "key": "up_live_abc123secret",
    "prefix": "up_live_abc1",
    "environment": "production",
    "created_at": "2026-04-23T18:00:00Z"
  },
  "request_id": "req_123"
}`,
  },
];

export default function CreateApiKeyPage() {
  return (
    <SingleEndpointReferencePage
      section="core"
      title="Create API key"
      description="Creates a new API key for one workspace in the dashboard. The plaintext key is only returned in this creation response."
      method="POST"
      path="/v1/workspaces/:workspace_id/api-keys"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "201", fields: RESPONSE_201_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
