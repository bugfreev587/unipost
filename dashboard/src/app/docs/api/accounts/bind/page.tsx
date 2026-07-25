"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "account_id", type: "string", description: "An existing public account binding ID in this workspace." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "profile_id", type: "string", description: "Target Profile that should reuse the verified physical connection." },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "data.id", type: "string", description: "Stable public account ID for the target Profile binding." },
  { name: "data.profile_id", type: "string", description: "Target Profile ID." },
  { name: "data.shared_connection", type: "boolean", description: "True for a successfully shared connection." },
  { name: "data.bound_profile_ids", type: "string[]", description: "Caller-visible Profiles currently bound to the physical connection." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Common values include "NOT_FOUND", "ACCOUNT_OWNERSHIP_CONFLICT", and "ACCOUNT_RECONNECT_REQUIRED".' },
  { name: "error.message", type: "string", description: "Human-readable failure message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [{
  lang: "curl",
  label: "cURL",
  code: `curl -X POST "https://api.unipost.dev/v1/accounts/sa_twitter_development/bindings" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"profile_id":"pr_staging"}'`,
}];

const RESPONSE_SNIPPETS = [{
  lang: "json",
  label: "201",
  code: `{
  "data": {
    "id": "sa_twitter_staging",
    "profile_id": "pr_staging",
    "platform": "twitter",
    "shared_connection": true,
    "bound_profile_ids": ["pr_development", "pr_staging"]
  },
  "request_id": "req_123"
}`,
}];

export default function BindAccountPage() {
  return (
    <SingleEndpointReferencePage
      section="accounts"
      title="Bind account to Profile"
      description="Creates or reactivates a Profile-scoped public account binding for an existing verified social connection. Publish requests continue to use only the returned account_id; the physical connection identifier is never accepted or returned."
      method="POST"
      path="/v1/accounts/:account_id/bindings"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "201", fields: RESPONSE_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
