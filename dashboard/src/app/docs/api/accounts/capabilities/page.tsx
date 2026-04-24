"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "account_id", type: "string", description: "Connected account ID such as sa_instagram_123." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "schema_version", type: "string", description: "Capability schema version." },
  { name: "account_id", type: "string", description: "Resolved account ID." },
  { name: "platform", type: "string", description: "Platform name for this account." },
  { name: "capability.display_name", type: "string", description: "Human-friendly platform name." },
  { name: "capability.text", type: "object", description: "Caption length and threading rules." },
  { name: "capability.media", type: "object", description: "Image/video limits and file format rules." },
  { name: "capability.thread", type: "object", description: "Whether reply-chain style threading is supported." },
  { name: "capability.scheduling", type: "object", description: "Whether UniPost can schedule posts for this platform." },
  { name: "capability.first_comment", type: "object", description: "Whether first comments are supported and any extra limits." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Common values include "UNAUTHORIZED" and "NOT_FOUND".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized" or "not_found".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/accounts/sa_instagram_123/capabilities" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const caps = await client.accounts.capabilities("sa_instagram_123");
console.log(caps.capability.media.images.max_count);`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "schema_version": "1.5",
    "account_id": "sa_instagram_123",
    "platform": "instagram",
    "capability": {
      "display_name": "Instagram",
      "text": {
        "max_length": 2200,
        "min_length": 0,
        "required": false,
        "supports_threads": false
      },
      "media": {
        "requires_media": true,
        "allow_mixed": true
      },
      "thread": { "supported": false },
      "scheduling": { "supported": true },
      "first_comment": { "supported": true, "max_length": 2200 }
    }
  }
}`,
  },
  {
    lang: "json",
    label: "404",
    code: `{
  "error": {
    "code": "NOT_FOUND",
    "normalized_code": "not_found",
    "message": "Account not found"
  },
  "request_id": "req_123"
}`,
  },
];

export default function AccountCapabilitiesPage() {
  return (
    <SingleEndpointReferencePage
      section="accounts"
      title="Get account capabilities"
      description="Returns the publishing capability map for the platform behind one connected account. Use it to drive client-side validation or UI affordances before you call create post."
      method="GET"
      path="/v1/accounts/:account_id/capabilities"
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
