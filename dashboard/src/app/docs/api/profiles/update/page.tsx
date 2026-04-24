"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "profile_id", type: "string", description: "Profile ID such as pr_brand_us." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "branding_logo_url?", type: "string", description: "HTTPS logo URL for hosted Connect." },
  { name: "branding_display_name?", type: "string", description: "Display name shown on hosted Connect." },
  { name: "branding_primary_color?", type: "string", description: "Hex color such as #10b981." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Profile ID." },
  { name: "workspace_id", type: "string", description: "Owning workspace ID." },
  { name: "name", type: "string", description: "Profile name. Name remains dashboard-only editable today." },
  { name: "account_count", type: "number", description: "Connected account count for that profile." },
  { name: "branding_logo_url", type: "string | null", description: "Updated logo URL." },
  { name: "branding_display_name", type: "string | null", description: "Updated display name." },
  { name: "branding_primary_color", type: "string | null", description: "Updated brand color." },
  { name: "updated_at", type: "string", description: "Last update timestamp." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Common values include "UNAUTHORIZED", "NOT_FOUND", and "VALIDATION_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized", "not_found", or "validation_error".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X PATCH "https://api.unipost.dev/v1/profiles/pr_brand_us" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "branding_logo_url": "https://cdn.example.com/logo.png",
    "branding_display_name": "Brand US",
    "branding_primary_color": "#10b981"
  }'`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const profile = await client.profiles.update("pr_brand_us", {
  branding_display_name: "Brand US",
  branding_primary_color: "#10b981",
});

console.log(profile.branding_display_name);`,
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
    "updated_at": "2026-04-23T09:12:00Z"
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "422",
    code: `{
  "error": {
    "code": "VALIDATION_ERROR",
    "normalized_code": "validation_error",
    "message": "branding_primary_color must be a 6-digit hex color (e.g. #10b981)"
  },
  "request_id": "req_123"
}`,
  },
];

export default function UpdateProfilePage() {
  return (
    <SingleEndpointReferencePage
      section="profiles"
      title="Update profile"
      description="Updates one profile. Public API callers can rename the profile and update the hosted Connect branding fields in the same request."
      method="PATCH"
      path="/v1/profiles/:profile_id"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
