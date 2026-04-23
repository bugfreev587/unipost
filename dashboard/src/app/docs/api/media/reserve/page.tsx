"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];
const BODY_FIELDS: ApiFieldItem[] = [
  { name: "filename", type: "string", description: "Original file name for the asset." },
  { name: "content_type", type: "string", description: "MIME type such as image/jpeg or video/mp4." },
  { name: "size_bytes", type: "number", description: "Expected upload size in bytes." },
];
const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "media_id", type: "string", description: "Media library ID to use in later publish calls." },
  { name: "upload_url", type: "string", description: "Presigned storage URL for the raw file bytes." },
  { name: "status", type: "string", description: 'Initial state, usually "pending".' },
];
const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
];
const SNIPPETS = [
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
const RESPONSE_SNIPPETS = [
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

export default function ReserveMediaPage() {
  return (
    <SingleEndpointReferencePage
      section="publishing"
      title="Reserve media upload"
      description="Creates a media library row and returns a presigned upload URL. Use it before uploading raw file bytes into UniPost-managed storage."
      method="POST"
      path="/v1/media"
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
