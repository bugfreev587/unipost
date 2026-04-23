"use client";

import type { ApiFieldItem } from "../../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];
const PATH_FIELDS: ApiFieldItem[] = [
  { name: "post_id", type: "string", description: "Draft post ID to publish." },
];
const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Published post ID." },
  { name: "status", type: "string", description: 'Transitions from "draft" to a publish state.' },
  { name: "results", type: "array", description: "Per-platform publish results." },
];
const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: "Machine-readable error code." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
];
const SNIPPETS = [
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

await client.posts.publish("post_abc123");`,
  },
];
const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "post_abc123",
    "status": "published",
    "results": [
      {
        "platform": "twitter",
        "status": "published"
      }
    ]
  }
}`,
  },
];

export default function PublishDraftPage() {
  return (
    <SingleEndpointReferencePage
      section="publishing"
      title="Publish draft"
      description="Publishes a draft post that already exists in UniPost. Use it when a human or automation has approved the saved content."
      method="POST"
      path="/v1/social-posts/:post_id/publish"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
