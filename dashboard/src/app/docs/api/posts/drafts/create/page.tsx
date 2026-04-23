"use client";

import type { ApiFieldItem } from "../../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];
const BODY_FIELDS: ApiFieldItem[] = [
  { name: "platform_posts", type: "array", description: "Draft content grouped by destination account." },
  { name: "status", type: '"draft"', description: "Creates a saved draft instead of publishing immediately." },
];
const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Draft post ID." },
  { name: "status", type: "string", description: 'Stored as "draft" until explicitly published.' },
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

const draft = await client.posts.create({
  status: "draft",
  platformPosts: [
    {
      accountId: "sa_twitter_1",
      caption: "Work in progress",
    },
  ],
});`,
  },
];
const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "post_abc123",
    "status": "draft"
  }
}`,
  },
];

export default function CreateDraftPage() {
  return (
    <SingleEndpointReferencePage
      section="publishing"
      title="Create draft"
      description="Creates a saved draft post. Use it when content should be reviewed or approved before it is published."
      method="POST"
      path="/v1/social-posts"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
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
