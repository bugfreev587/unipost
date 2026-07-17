"use client";

import { ApiInlineLink, EnumValues, InfoBox, type ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];
const BODY_FIELDS: ApiFieldItem[] = [
  { name: "filename", type: "string", description: "Original file name for the asset." },
  { name: "content_type", type: "string", description: <>MIME type such as <code>image/jpeg</code>, <code>video/mp4</code>, or <code>audio/mpeg</code>. Audio uploads are accepted as media-processing inputs; publish the processed video output, not the raw audio media ID.</> },
  { name: "size_bytes?", type: "number", description: <>Optional upload size in bytes. If provided, it must be greater than <code>0</code>. If your client does not know the byte length yet, omit <code>size_bytes</code>; UniPost hydrates the actual size from storage after upload. If the asset already has a public URL, skip <ApiInlineLink endpoint="POST /v1/media" /> and send that URL in <code>platform_posts[].media_urls</code> on <ApiInlineLink endpoint="POST /v1/posts" />.</> },
];
const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Media library ID to use in later publish calls." },
  { name: "upload_url", type: "string", description: "Presigned storage URL for the raw file bytes." },
  { name: "status", type: "string", description: <>Media lifecycle state. Reserve responses start as pending; publish calls should wait for uploaded.<EnumValues values={["pending", "uploaded", "attached", "deleted"]} /></> },
];
const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
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
    "content_type": "image/jpeg"
  }'`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { readFile } from "node:fs/promises";
import { UniPost } from "@unipost/sdk";

const client = new UniPost();
const fileBuffer = await readFile("photo.jpg");

const { mediaId, uploadUrl } = await client.media.upload({
  filename: "photo.jpg",
  contentType: "image/jpeg",
});

await fetch(uploadUrl, {
  method: "PUT",
  headers: { "Content-Type": "image/jpeg" },
  body: fileBuffer,
});

let media = await client.media.get(mediaId);
while (media.status === "pending") {
  await new Promise((resolve) => setTimeout(resolve, 1000));
  media = await client.media.get(mediaId);
}

if (media.status !== "uploaded" && media.status !== "attached") {
  throw new Error(\`media upload failed with status \${media.status}\`);
}

console.log(mediaId);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost
import requests

client = UniPost()

reservation = client.media.upload(
  filename="photo.jpg",
  content_type="image/jpeg",
)

with open("photo.jpg", "rb") as f:
  requests.put(
    reservation["data"]["upload_url"],
    data=f,
    headers={"Content-Type": "image/jpeg"},
  )

print(reservation["data"]["id"])`,
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

  reservation, err := client.Media.Upload(context.Background(), &unipost.MediaUploadRequest{
    Filename:    "photo.jpg",
    ContentType: "image/jpeg",
  })
  if err != nil {
    log.Fatal(err)
  }

  // PUT raw bytes to reservation.UploadURL with your HTTP client of choice.
  // Poll GET /v1/media/{media_id} until status is uploaded before publishing.
  fmt.Println(reservation.MediaID)
}`,
  },
  {
    lang: "java",
    label: "Java",
    code: `import dev.unipost.UniPost;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpRequest.BodyPublishers;
import java.net.http.HttpResponse.BodyHandlers;
import java.nio.file.Path;
import java.util.Map;

UniPost client = new UniPost();

var reservation = client.media().upload(Map.of(
    "filename", "photo.jpg",
    "content_type", "image/jpeg"
));

var mediaId = reservation.get("id").asText();
var uploadUrl = reservation.get("upload_url").asText();

HttpClient.newHttpClient().send(
    HttpRequest.newBuilder(URI.create(uploadUrl))
        .header("Content-Type", "image/jpeg")
        .PUT(BodyPublishers.ofFile(Path.of("photo.jpg")))
        .build(),
    BodyHandlers.discarding()
);

System.out.println(mediaId);`,
  },
];
const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "media_123",
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
    "normalized_code": "unauthorized",
    "message": "Missing or invalid API key."
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "422",
    code: `{
  "error": {
    "code": "VALIDATION_ERROR",
    "normalized_code": "validation_error",
    "message": "size_bytes, when provided, must be greater than 0 when reserving an upload with POST /v1/media. If you do not know the raw byte length yet, omit size_bytes; UniPost will hydrate it from storage after upload.",
    "issues": [
      {
        "field": "size_bytes",
        "code": "below_min_length",
        "message": "size_bytes, when provided, must be greater than 0 when reserving an upload with POST /v1/media. If you do not know the raw byte length yet, omit size_bytes; UniPost will hydrate it from storage after upload.",
        "actual": 0,
        "limit": 1,
        "severity": "error"
      }
    ]
  },
  "request_id": "req_123"
}`,
  },
];

export default function ReserveMediaPage() {
  return (
    <SingleEndpointReferencePage
      section="publishing"
      title="Reserve media upload"
      description={<>Creates a media library row and returns a presigned upload URL for local files or raw bytes. Hosted public URLs do not need this step; send them directly in <code>platform_posts[].media_urls</code> on <ApiInlineLink endpoint="POST /v1/posts" />. After the PUT succeeds, poll <ApiInlineLink endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" /> until status is uploaded before using the media ID in <ApiInlineLink endpoint="POST /v1/posts" />.</>}
      method="POST"
      path="/v1/media"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "201", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
        { code: "503", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    >
      <InfoBox>
        <strong>Size is optional:</strong> clients no longer need to calculate file length before reserving an upload.
        When <code>size_bytes</code> is omitted, the pending media response can show <code>size_bytes: 0</code>;
        <ApiInlineLink endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" /> returns the hydrated size after the PUT lands.
      </InfoBox>
      <InfoBox>
        <strong>Retention follows post status:</strong> UniPost keeps uploaded media for scheduled, draft,
        queued, publishing, and active Media Processing jobs. After the parent post or processing job finishes, media is retained by plan:
        Free 1 day after success or 2 days after failed/partial/cancelled, API 2/4 days, Basic 4/8 days,
        Growth 15/30 days, and Team or Enterprise 30/60 days. Reused media is deleted only after all Post and Processing usages are due.
      </InfoBox>
      <InfoBox>
        To turn an uploaded <code>image/gif</code> into video media, use <ApiInlineLink endpoint="POST /v1/media/gif-conversions" href="/docs/api/media/gif-conversions" /> after the upload is ready.
      </InfoBox>
      <InfoBox>
        <strong>TikTok file_upload note:</strong> upload the complete video file to UniPost with this
        endpoint, then publish with <code>media_ids</code>. UniPost handles TikTok&apos;s downstream
        chunk sizing and sequential transfer automatically, including videos larger than <code>64 MB</code>.
        Do not split the file or provide TikTok <code>chunk_size</code> / <code>total_chunk_count</code> yourself.
      </InfoBox>
    </SingleEndpointReferencePage>
  );
}
