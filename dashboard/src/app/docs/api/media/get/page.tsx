"use client";

import { ApiInlineLink, EnumValues, InfoBox, type ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];
const PATH_FIELDS: ApiFieldItem[] = [
  { name: "media_id", type: "string", description: "Media library ID returned from the reserve call." },
];
const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Media library ID." },
  { name: "status", type: "string", description: <>Media lifecycle state. Poll until uploaded before passing the ID to a publish request.<EnumValues values={["pending", "uploaded", "attached", "deleted"]} /></>, },
  { name: "content_type", type: "string", description: "Resolved media MIME type." },
  { name: "size_bytes", type: "number", description: "Stored file size in bytes." },
];
const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED" or "NOT_FOUND".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized" or "not_found".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];
const SNIPPETS = [
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

const client = new UniPost();

const media = await client.media.get("media_123");
if (media.status === "uploaded" || media.status === "attached") {
  console.log("ready to publish");
}`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

media = client.media.get("media_123")
print(media["data"]["status"])`,
  },
  {
    lang: "go",
    label: "Go",
    code: `package main

import (
  "context"
  "fmt"
  "log"

  "github.com/unipost-dev/sdk-go/unipost"
)

func main() {
  client := unipost.NewClient()

  media, err := client.Media.Get(context.Background(), "media_123")
  if err != nil {
    log.Fatal(err)
  }

  fmt.Println(media.Status)
}`,
  },
  {
    lang: "java",
    label: "Java",
    code: `import dev.unipost.UniPost;

UniPost client = new UniPost();

var media = client.media().get("media_123");
System.out.println(media.get("status").asText());`,
  },
];
const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "media_123",
    "status": "uploaded",
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
    "normalized_code": "not_found",
    "message": "Media not found."
  },
  "request_id": "req_123"
}`,
  },
];

export default function GetMediaPage() {
  return (
    <SingleEndpointReferencePage
      section="publishing"
      title="Get media"
      description="Returns the current state of one media library asset. Use it to check whether an upload is ready before publishing with media IDs."
      method="GET"
      path="/v1/media/:media_id"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    >
      <InfoBox>
        <strong>Deleted media:</strong> after a post reaches a final status, UniPost keeps uploaded media
        for the plan retention window, then removes the R2 object and the media row after every usage
        for that media is due. Scheduled, draft, queued, publishing, and active Media Processing jobs keep their
        media until they finish. GIF conversion inputs and successful MP4 outputs follow the same lifecycle.
      </InfoBox>
      <InfoBox>
        An uploaded <code>image/gif</code> can be passed to <ApiInlineLink endpoint="POST /v1/media/gif-conversions" href="/docs/api/media/gif-conversions" />. Poll the conversion job itself for <code>output_media_id</code>; this endpoint only reports the Media row.
      </InfoBox>
    </SingleEndpointReferencePage>
  );
}
