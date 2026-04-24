"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "profile_id", type: "string", description: "Profile ID such as pr_brand_us." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Profile ID." },
  { name: "workspace_id", type: "string", description: "Owning workspace ID." },
  { name: "name", type: "string", description: "Profile name." },
  { name: "account_count", type: "number", description: "Connected account count for that profile." },
  { name: "branding_logo_url", type: "string | null", description: "Optional hosted Connect logo URL." },
  { name: "branding_display_name", type: "string | null", description: "Optional hosted Connect display name." },
  { name: "branding_primary_color", type: "string | null", description: "Optional hosted Connect primary brand color." },
  { name: "created_at", type: "string", description: "Creation timestamp." },
  { name: "updated_at", type: "string", description: "Last update timestamp." },
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
    code: `curl "https://api.unipost.dev/v1/profiles/pr_brand_us" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const profile = await client.profiles.get("pr_brand_us");
console.log(profile.name);`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "pr_brand_us",
    "workspace_id": "ws_123",
    "name": "Brand US",
    "branding_logo_url": "https://cdn.example.com/logo.png",
    "branding_display_name": "Brand US",
    "branding_primary_color": "#10b981",
    "account_count": 2,
    "created_at": "2026-04-01T10:00:00Z",
    "updated_at": "2026-04-21T18:40:00Z"
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "404",
    code: `{
  "error": {
    "code": "NOT_FOUND",
    "normalized_code": "not_found",
    "message": "Profile not found"
  },
  "request_id": "req_123"
}`,
  },
];

export default function GetProfilePage() {
  return (
    <SingleEndpointReferencePage
      section="profiles"
      title="Get profile"
      description="Returns one profile that belongs to the workspace behind your API key."
      method="GET"
      path="/v1/profiles/:profile_id"
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
