"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Clerk session", type: "Browser session", meta: "Cookie/session", description: "Dashboard-authenticated route. This endpoint does not use a Bearer API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "workspace_id", type: "string", description: "Workspace that owns the API keys." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "API keys currently attached to this workspace." },
  { name: "data[].id", type: "string", description: "API key record ID." },
  { name: "data[].name", type: "string", description: "Human-readable label shown in the dashboard." },
  { name: "data[].prefix", type: "string", description: "Safe key prefix, such as up_live_abcd." },
  { name: "data[].environment", type: "string", description: 'Either "production" or "test".' },
  { name: "data[].created_at", type: "string", description: "Creation timestamp." },
  { name: "data[].last_used_at", type: "string | null", description: "Last request timestamp, if the key has been used." },
  { name: "data[].expires_at", type: "string | null", description: "Optional expiration timestamp." },
  { name: "meta.total", type: "number", description: "Total API keys returned for this workspace." },
  { name: "meta.limit", type: "number", description: "Applied list limit for this response." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED" or "NOT_FOUND".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized" or "not_found".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/workspaces/ws_123/api-keys" \\
  -H "Cookie: __session=<clerk-session-cookie>"`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": [
    {
      "id": "key_123",
      "name": "Production backend",
      "prefix": "up_live_abcd",
      "environment": "production",
      "created_at": "2026-04-23T18:00:00Z",
      "last_used_at": "2026-04-23T18:15:00Z",
      "expires_at": null
    }
  ],
  "meta": {
    "total": 1,
    "limit": 1
  },
  "request_id": "req_123"
}`,
  },
];

export default function ListApiKeysPage() {
  return (
    <SingleEndpointReferencePage
      section="core"
      title="List API keys"
      description="Lists the API keys owned by one workspace in the dashboard. This is a Clerk session route used by the UniPost app, not a Bearer API key route."
      method="GET"
      path="/v1/workspaces/:workspace_id/api-keys"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
