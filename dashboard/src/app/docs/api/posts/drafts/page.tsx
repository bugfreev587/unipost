"use client";

import {
  ApiReferencePage,
  ApiReferenceGrid,
  ApiEndpointCard,
  ApiAccordion,
  ApiFieldList,
  CodeTabs,
  type ApiFieldItem,
} from "../../_components/doc-components";

const AUTH_FIELDS: ApiFieldItem[] = [
  {
    name: "Authorization",
    type: "Bearer <token>",
    meta: "In header",
    description: "Workspace API key.",
  },
];

const DRAFT_BODY_FIELDS: ApiFieldItem[] = [
  {
    name: "platform_posts",
    type: "array",
    description: "Draft content grouped by destination account.",
  },
  {
    name: "status",
    type: '"draft"',
    description: "Creates a saved draft instead of publishing immediately.",
  },
];

const DRAFT_RESPONSE_FIELDS: ApiFieldItem[] = [
  {
    name: "id",
    type: "string",
    description: "Draft post ID.",
  },
  {
    name: "status",
    type: "string",
    description: 'Stored as "draft" until explicitly published.',
  },
];

const PUBLISH_PATH_FIELDS: ApiFieldItem[] = [
  {
    name: "post_id",
    type: "string",
    description: "Draft post ID to publish.",
  },
];

const PUBLISH_RESPONSE_FIELDS: ApiFieldItem[] = [
  {
    name: "id",
    type: "string",
    description: "Published post ID.",
  },
  {
    name: "status",
    type: "string",
    description: 'Transitions from "draft" to a publish state.',
  },
  {
    name: "results",
    type: "array",
    description: "Per-platform publish results.",
  },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  {
    name: "error.code",
    type: "string",
    description: "Machine-readable error code.",
  },
  {
    name: "error.message",
    type: "string",
    description: "Human-readable error message.",
  },
];

const DRAFT_SNIPPETS = [
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

const PUBLISH_SNIPPETS = [
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

const DRAFT_RESPONSE_SNIPPETS = [
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

const PUBLISH_RESPONSE_SNIPPETS = [
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

function EndpointBlock({
  label,
  method,
  path,
  requestTitle,
  requestFields,
  responseFields,
  snippets,
  responseSnippets,
}: {
  label: string;
  method: string;
  path: string;
  requestTitle: string;
  requestFields: ApiFieldItem[];
  responseFields: ApiFieldItem[];
  snippets: { lang: string; label: string; code: string }[];
  responseSnippets: { lang: string; label: string; code: string }[];
}) {
  const methodColor = method === "GET" ? "#10b981" : "#3b82f6";

  return (
    <section style={{ display: "grid", gap: 18 }}>
      <div style={{ fontSize: 18, fontWeight: 700, color: "var(--docs-text)" }}>{label}</div>
      <ApiReferenceGrid
        left={
          <div style={{ display: "grid", gap: 16 }}>
            <ApiEndpointCard method={method} path={path}>
              <div style={{ padding: "16px 18px" }}>
                <span style={{ fontFamily: "var(--docs-mono)", fontSize: 15, fontWeight: 700, color: methodColor, marginRight: 12 }}>{method}</span>
                <code style={{ fontFamily: "var(--docs-mono)", fontSize: 15, color: "var(--docs-text)" }}>{path}</code>
              </div>
            </ApiEndpointCard>

            <ApiEndpointCard method={method} path={path}>
              <div style={{ padding: "18px", borderBottom: "1px solid var(--docs-border)" }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Authorization</div>
                <ApiFieldList items={AUTH_FIELDS} />
              </div>
              <div style={{ padding: "18px" }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>{requestTitle}</div>
                <ApiFieldList items={requestFields} />
              </div>
            </ApiEndpointCard>

            <ApiEndpointCard method={method} path={path}>
              <div style={{ padding: "18px 18px 4px" }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Response Body</div>
              </div>
              <ApiAccordion title="200">
                <ApiFieldList items={responseFields} />
              </ApiAccordion>
              <ApiAccordion title="401">
                <ApiFieldList items={ERROR_FIELDS} />
              </ApiAccordion>
            </ApiEndpointCard>
          </div>
        }
        right={
          <div style={{ display: "grid", gap: 14, alignContent: "start" }}>
            <CodeTabs snippets={snippets} />
            <CodeTabs snippets={responseSnippets} />
          </div>
        }
      />
    </section>
  );
}

export default function DraftsPage() {
  return (
    <ApiReferencePage
      section="publishing"
      title="Drafts and preview links"
      description="Drafts are real social-post rows stored with status draft. Use them when content should be saved, reviewed, and published later instead of going live immediately."
    >
      <div style={{ display: "grid", gap: 34 }}>
        <EndpointBlock
          label="Create draft"
          method="POST"
          path="/v1/social-posts"
          requestTitle="Request Body"
          requestFields={DRAFT_BODY_FIELDS}
          responseFields={DRAFT_RESPONSE_FIELDS}
          snippets={DRAFT_SNIPPETS}
          responseSnippets={DRAFT_RESPONSE_SNIPPETS}
        />

        <EndpointBlock
          label="Publish draft"
          method="POST"
          path="/v1/social-posts/:post_id/publish"
          requestTitle="Path Params"
          requestFields={PUBLISH_PATH_FIELDS}
          responseFields={PUBLISH_RESPONSE_FIELDS}
          snippets={PUBLISH_SNIPPETS}
          responseSnippets={PUBLISH_RESPONSE_SNIPPETS}
        />

        <ApiEndpointCard method="GET" path="preview-links">
          <div style={{ padding: "18px" }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Preview links</div>
            <div style={{ fontSize: 15, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
              Preview-link generation is not in the SDK yet. Use SDK draft creation and publish today, and keep preview-link generation on the REST endpoint if your workflow needs signed review links.
            </div>
          </div>
        </ApiEndpointCard>
      </div>
    </ApiReferencePage>
  );
}
