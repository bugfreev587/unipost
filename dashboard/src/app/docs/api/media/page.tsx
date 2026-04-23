"use client";

import {
  ApiReferencePage,
  ApiReferenceGrid,
  ApiEndpointCard,
  ApiAccordion,
  ApiFieldList,
  CodeTabs,
  type ApiFieldItem,
} from "../_components/doc-components";

const AUTH_FIELDS: ApiFieldItem[] = [
  {
    name: "Authorization",
    type: "Bearer <token>",
    meta: "In header",
    description: "Workspace API key.",
  },
];

const CREATE_BODY_FIELDS: ApiFieldItem[] = [
  {
    name: "filename",
    type: "string",
    description: "Original file name for the asset.",
  },
  {
    name: "content_type",
    type: "string",
    description: "MIME type such as image/jpeg or video/mp4.",
  },
  {
    name: "size_bytes",
    type: "number",
    description: "Expected upload size in bytes.",
  },
];

const CREATE_RESPONSE_FIELDS: ApiFieldItem[] = [
  {
    name: "media_id",
    type: "string",
    description: "Media library ID to use in later publish calls.",
  },
  {
    name: "upload_url",
    type: "string",
    description: "Presigned storage URL for the raw file bytes.",
  },
  {
    name: "status",
    type: "string",
    description: 'Initial state, usually "pending".',
  },
];

const GET_PATH_FIELDS: ApiFieldItem[] = [
  {
    name: "media_id",
    type: "string",
    description: "Media library ID returned from the reserve call.",
  },
];

const GET_RESPONSE_FIELDS: ApiFieldItem[] = [
  {
    name: "id",
    type: "string",
    description: "Media library ID.",
  },
  {
    name: "status",
    type: "string",
    description: 'Media processing state such as "pending" or "ready".',
  },
  {
    name: "content_type",
    type: "string",
    description: "Resolved media MIME type.",
  },
  {
    name: "size_bytes",
    type: "number",
    description: "Stored file size in bytes.",
  },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  {
    name: "error.code",
    type: "string",
    description: 'Usually "UNAUTHORIZED" or "NOT_FOUND".',
  },
  {
    name: "error.message",
    type: "string",
    description: "Human-readable error message.",
  },
];

const CREATE_SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/media" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "filename": "photo.jpg",
    "content_type": "image/jpeg",
    "size_bytes": 284192
  }'`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const { mediaId, uploadUrl } = await client.media.upload({
  filename: "photo.jpg",
  contentType: "image/jpeg",
  sizeBytes: 284192,
});

await fetch(uploadUrl, {
  method: "PUT",
  body: fileBuffer,
});

console.log(mediaId);`,
  },
];

const GET_SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/media/media_123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const media = await client.media.get("media_123");
console.log(media.status);`,
  },
];

const CREATE_RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "media_id": "media_123",
    "upload_url": "https://storage.example.com/...",
    "status": "pending"
  }
}`,
  },
  {
    lang: "json",
    label: "401",
    code: `{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Missing or invalid API key."
  }
}`,
  },
];

const GET_RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "media_123",
    "status": "ready",
    "content_type": "image/jpeg",
    "size_bytes": 284192
  }
}`,
  },
  {
    lang: "json",
    label: "404",
    code: `{
  "error": {
    "code": "NOT_FOUND",
    "message": "Media not found."
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
  extraResponses,
  snippets,
  responseSnippets,
}: {
  label: string;
  method: string;
  path: string;
  requestTitle: string;
  requestFields: ApiFieldItem[];
  responseFields: ApiFieldItem[];
  extraResponses: Array<{ code: string; fields: ApiFieldItem[] }>;
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
              {extraResponses.map((item) => (
                <ApiAccordion key={item.code} title={item.code}>
                  <ApiFieldList items={item.fields} />
                </ApiAccordion>
              ))}
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

export default function MediaPage() {
  return (
    <ApiReferencePage
      section="publishing"
      title="Media"
      description="Two-step upload flow backed by UniPost storage. Use media IDs when your app cannot rely on public asset URLs or needs a stable upload-to-publish workflow."
    >
      <div style={{ display: "grid", gap: 34 }}>
        <EndpointBlock
          label="Reserve upload"
          method="POST"
          path="/v1/media"
          requestTitle="Request Body"
          requestFields={CREATE_BODY_FIELDS}
          responseFields={CREATE_RESPONSE_FIELDS}
          extraResponses={[{ code: "401", fields: ERROR_FIELDS }]}
          snippets={CREATE_SNIPPETS}
          responseSnippets={CREATE_RESPONSE_SNIPPETS}
        />

        <EndpointBlock
          label="Get media"
          method="GET"
          path="/v1/media/:media_id"
          requestTitle="Path Params"
          requestFields={GET_PATH_FIELDS}
          responseFields={GET_RESPONSE_FIELDS}
          extraResponses={[
            { code: "401", fields: ERROR_FIELDS },
            { code: "404", fields: ERROR_FIELDS },
          ]}
          snippets={GET_SNIPPETS}
          responseSnippets={GET_RESPONSE_SNIPPETS}
        />
      </div>
    </ApiReferencePage>
  );
}
