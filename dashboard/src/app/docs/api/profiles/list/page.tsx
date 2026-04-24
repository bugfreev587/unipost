"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "Profiles that belong to the workspace behind the API key." },
  { name: "data[].id", type: "string", description: "Profile ID." },
  { name: "data[].workspace_id", type: "string", description: "Workspace that owns the profile." },
  { name: "data[].name", type: "string", description: "Human-readable profile name." },
  { name: "data[].branding_logo_url", type: "string | null", description: "Optional hosted Connect logo URL." },
  { name: "data[].branding_display_name", type: "string | null", description: "Optional hosted Connect display name." },
  { name: "data[].branding_primary_color", type: "string | null", description: "Optional hosted Connect primary brand color." },
  { name: "data[].account_count", type: "number", description: "Connected account count for that profile." },
  { name: "meta.total", type: "number", description: "Total number of profiles in the workspace." },
  { name: "meta.limit", type: "number", description: "Applied list size for this response." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/profiles" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const { data: profiles } = await client.profiles.list();
console.log(profiles.map((profile) => profile.name));`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": [
    {
      "id": "pr_brand_us",
      "workspace_id": "ws_123",
      "name": "Brand US",
      "branding_logo_url": "https://cdn.example.com/logo.png",
      "branding_display_name": "Brand US",
      "branding_primary_color": "#10b981",
      "account_count": 2,
      "created_at": "2026-04-01T10:00:00Z",
      "updated_at": "2026-04-21T18:40:00Z"
    }
  ],
  "meta": {
    "total": 1,
    "limit": 1
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "401",
    code: `{
  "error": {
    "code": "UNAUTHORIZED",
    "normalized_code": "unauthorized",
    "message": "Missing or invalid API key."
  },
  "request_id": "req_123"
}`,
  },
];

export default function ListProfilesPage() {
  return (
    <SingleEndpointReferencePage
      section="profiles"
      title="List profiles"
      description="Returns the profiles that belong to your workspace. Profiles are the brand containers that sit under one workspace and drive hosted Connect branding."
      method="GET"
      path="/v1/profiles"
      requestSections={[{ title: "Authorization", items: AUTH_FIELDS }]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
