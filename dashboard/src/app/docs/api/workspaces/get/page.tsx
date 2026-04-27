"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Clerk session", type: "Browser session", meta: "Cookie/session", description: "Dashboard-authenticated route. This endpoint does not use a Bearer API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "workspace_id", type: "string", description: "Workspace to fetch." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Workspace ID." },
  { name: "name", type: "string", description: "Human-readable workspace name." },
  { name: "per_account_monthly_limit", type: "number | null", description: "Optional per-account monthly publish quota." },
  { name: "usage_modes", type: "string[]", description: 'Active usage modes such as "publishing" or "agentic".' },
  { name: "created_at", type: "string", description: "Creation timestamp." },
  { name: "updated_at", type: "string", description: "Last update timestamp." },
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
    code: `curl "https://api.unipost.dev/v1/workspaces/ws_123" \\
  -H "Cookie: __session=<clerk-session-cookie>"`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "ws_123",
    "name": "Acme",
    "per_account_monthly_limit": null,
    "usage_modes": ["publishing"],
    "created_at": "2026-01-04T10:00:00Z",
    "updated_at": "2026-04-23T18:00:00Z"
  }
}`,
  },
];

export default function GetWorkspacePage() {
  return (
    <SingleEndpointReferencePage
      section="core"
      title="Get workspace"
      description="Returns one workspace by ID. The caller must own the workspace; otherwise the route returns 404. This is a Clerk session route used by the dashboard, not a Bearer API key route."
      method="GET"
      path="/v1/workspaces/:workspace_id"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
