"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "name", type: "string", description: "Unique profile name inside the workspace." },
  { name: "branding_logo_url?", type: "string", description: "Optional HTTPS logo URL for hosted Connect." },
  { name: "branding_display_name?", type: "string", description: "Optional hosted Connect display name." },
  { name: "branding_primary_color?", type: "string", description: "Optional hex color such as #10b981." },
];

const RESPONSE_201_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Profile ID." },
  { name: "workspace_id", type: "string", description: "Owning workspace ID." },
  { name: "name", type: "string", description: "Profile name." },
  { name: "account_count", type: "number", description: "Connected account count. New profiles start at 0." },
  { name: "branding_logo_url", type: "string | null", description: "Stored hosted Connect logo URL." },
  { name: "branding_display_name", type: "string | null", description: "Stored hosted Connect display name." },
  { name: "branding_primary_color", type: "string | null", description: "Stored hosted Connect brand color." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Common values include "UNAUTHORIZED", "VALIDATION_ERROR", and "PROFILE_NAME_TAKEN".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "profile_name_taken".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/profiles" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "name": "Brand US",
    "branding_display_name": "Brand US",
    "branding_primary_color": "#10b981"
  }'`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `const profile = await client.profiles.create({
  name: "Brand US",
  brandingDisplayName: "Brand US",
  brandingPrimaryColor: "#10b981",
});`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "201",
    code: `{
  "data": {
    "id": "pr_brand_us",
    "workspace_id": "ws_123",
    "name": "Brand US",
    "account_count": 0,
    "branding_display_name": "Brand US",
    "branding_primary_color": "#10b981",
    "created_at": "2026-04-23T16:00:00Z",
    "updated_at": "2026-04-23T16:00:00Z"
  }
}`,
  },
];

export default function CreateProfilePage() {
  return (
    <SingleEndpointReferencePage
      section="profiles"
      title="Create profile"
      description="Creates a new profile inside the workspace behind your API key. You can optionally set the hosted Connect branding fields in the same call."
      method="POST"
      path="/v1/profiles"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "201", fields: RESPONSE_201_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
