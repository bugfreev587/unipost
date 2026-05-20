"use client";

import { ApiInlineLink, EnumValues, type ApiFieldItem } from "../../_components/doc-components";
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
    "content_type": "image/jpeg",
    "size_bytes": 284192
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
  sizeBytes: fileBuffer.byteLength,
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
  size_bytes=284192,
)

with open("photo.jpg", "rb") as f:
  requests.put(
    reservation["data"]["upload_url"],
    data=f,
    headers={"Content-Type": "image/jpeg"},
  )

print(reservation["data"]["media_id"])`,
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
    SizeBytes:   284192,
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
    "content_type", "image/jpeg",
    "size_bytes", 284192
));

var mediaId = reservation.get("media_id").asText();
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
    "normalized_code": "unauthorized",
    "message": "Missing or invalid API key."
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
      description={<>Creates a media library row and returns a presigned upload URL. After the PUT succeeds, poll <ApiInlineLink endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" /> until status is uploaded before using the media ID in <ApiInlineLink endpoint="POST /v1/posts" />.</>}
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
    />
  );
}
