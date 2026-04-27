"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key or Clerk session JWT." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "name?", type: "string", description: "New workspace display name. Must be non-empty when provided." },
  { name: "per_account_monthly_limit?", type: "number", description: "Per-account monthly publish quota; 0 to 1,000,000." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Workspace ID." },
  { name: "name", type: "string", description: "Updated workspace name." },
  { name: "per_account_monthly_limit", type: "number | null", description: "Updated quota value." },
  { name: "usage_modes", type: "string[]", description: "Active usage modes." },
  { name: "created_at", type: "string", description: "Creation timestamp." },
  { name: "updated_at", type: "string", description: "Updated timestamp." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "VALIDATION_ERROR", "UNAUTHORIZED", or "INTERNAL_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "validation_error" or "unauthorized".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X PATCH "https://api.unipost.dev/v1/workspace" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{ "name": "Acme Inc." }'`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "ws_123",
    "name": "Acme Inc.",
    "per_account_monthly_limit": null,
    "usage_modes": ["publishing"],
    "created_at": "2026-01-04T10:00:00Z",
    "updated_at": "2026-04-27T18:00:00Z"
  }
}`,
  },
];

export default function UpdateWorkspacePage() {
  return (
    <SingleEndpointReferencePage
      section="core"
      title="Update workspace"
      description="Updates the workspace bound to the authenticated caller. Either field is optional; passing neither is a no-op that still returns the current state."
      method="PATCH"
      path="/v1/workspace"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
