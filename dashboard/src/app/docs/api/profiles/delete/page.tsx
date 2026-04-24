"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "profile_id", type: "string", description: "Profile ID to delete." },
];

const RESPONSE_204_FIELDS: ApiFieldItem[] = [
  { name: "status", type: "204 No Content", description: "Returned when the profile is deleted successfully." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Common values include "DEFAULT_PROFILE_PROTECTED", "PROFILE_NOT_EMPTY", and "NOT_FOUND".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "default_profile_protected" or "profile_not_empty".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X DELETE "https://api.unipost.dev/v1/profiles/pr_brand_us" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `await client.profiles.delete("pr_brand_us");`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "409",
    code: `{
  "error": {
    "code": "PROFILE_NOT_EMPTY",
    "normalized_code": "profile_not_empty",
    "message": "Profile has connected accounts; disconnect them before deleting the profile"
  },
  "request_id": "req_123"
}`,
  },
];

export default function DeleteProfilePage() {
  return (
    <SingleEndpointReferencePage
      section="profiles"
      title="Delete profile"
      description="Deletes one profile. Only empty non-default profiles can be deleted; profiles with connected accounts must be cleaned up first."
      method="DELETE"
      path="/v1/profiles/:profile_id"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "204", fields: RESPONSE_204_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
