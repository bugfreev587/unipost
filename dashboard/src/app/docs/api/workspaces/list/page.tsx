"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Clerk session", type: "Browser session", meta: "Cookie/session", description: "Dashboard-authenticated route. This endpoint does not use a Bearer API key." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "Workspaces owned by the authenticated user." },
  { name: "data[].id", type: "string", description: "Workspace ID. Use this value as the workspace_id path param on routes like /v1/workspaces/:workspace_id/api-keys." },
  { name: "data[].name", type: "string", description: "Human-readable workspace name." },
  { name: "data[].per_account_monthly_limit", type: "number | null", description: "Optional per-account monthly publish quota." },
  { name: "data[].usage_modes", type: "string[]", description: 'Active usage modes such as "publishing" or "agentic".' },
  { name: "data[].created_at", type: "string", description: "Creation timestamp." },
  { name: "data[].updated_at", type: "string", description: "Last update timestamp." },
  { name: "meta.total", type: "number", description: "Total number of workspaces returned." },
  { name: "meta.limit", type: "number", description: "Applied list limit for this response." },
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
    code: `curl "https://api.unipost.dev/v1/workspaces" \\
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
      "id": "ws_123",
      "name": "Acme",
      "per_account_monthly_limit": null,
      "usage_modes": ["publishing"],
      "created_at": "2026-01-04T10:00:00Z",
      "updated_at": "2026-04-23T18:00:00Z"
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

export default function ListWorkspacesPage() {
  return (
    <SingleEndpointReferencePage
      section="core"
      title="List workspaces"
      description="Returns the workspaces owned by the authenticated dashboard user. Use it to discover the workspace_id required by other dashboard routes such as the API keys endpoints. This is a Clerk session route, not a Bearer API key route — Bearer-authenticated calls do not need a workspace_id because the API key already binds the request to one workspace."
      method="GET"
      path="/v1/workspaces"
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
    />
  );
}
