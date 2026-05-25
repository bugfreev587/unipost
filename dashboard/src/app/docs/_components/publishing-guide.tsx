import Link from "next/link";
import { DocsCodeTabs, DocsTable } from "./docs-shell";
import { ApiInlineLink } from "../api/_components/doc-components";

const LOCAL_FILE_SNIPPETS = [
  {
    label: "Node SDK",
    lang: "javascript",
    code: `import { readFile } from "node:fs/promises";
import { UniPost } from "@unipost/sdk";

const client = new UniPost();
const platform = "instagram";
const platformOptions = { mediaType: "feed" };

// Step 1: connect the platform account once, then keep the returned account_id.
const { auth_url: authUrl } = await client.connect.getConnectUrl({ platform });
console.log("Open this URL in a browser:", authUrl);

const { data: accounts } = await client.accounts.list({ platform });
const accountId = accounts[0].id;

// Step 2: reserve media and get a presigned upload URL.
const fileBuffer = await readFile("campaign-photo.jpg");
const { mediaId, uploadUrl } = await client.media.upload({
  filename: "campaign-photo.jpg",
  contentType: "image/jpeg",
  sizeBytes: fileBuffer.byteLength,
});

// Step 3: PUT raw bytes to the upload_url.
await fetch(uploadUrl, {
  method: "PUT",
  headers: { "Content-Type": "image/jpeg" },
  body: fileBuffer,
});

// Step 4: poll until the media row is ready to publish.
let media = await client.media.get(mediaId);
while (media.status === "pending") {
  await new Promise((resolve) => setTimeout(resolve, 1000));
  media = await client.media.get(mediaId);
}

if (media.status !== "uploaded" && media.status !== "attached") {
  throw new Error(\`media upload failed with status \${media.status}\`);
}

const platformPost = {
  accountId,
  caption: "New seasonal item is available today.",
  mediaIds: [mediaId],
  platformOptions,
};

// Step 5: optional preflight validation.
const validation = await client.posts.validate({
  platformPosts: [platformPost],
});

if (!validation.valid) {
  throw new Error(JSON.stringify(validation.errors, null, 2));
}

// Step 6: create the post with media_ids.
const post = await client.posts.create({
  platformPosts: [platformPost],
});

console.log(post.id);

// Step 7: get the asynchronous publishing status and result.
const publishingResult = await client.posts.get(post.id);
console.log(publishingResult.data.status);
console.log(publishingResult.data.results);`,
  },
  {
    label: "Python SDK",
    lang: "python",
    code: `from pathlib import Path
import time
import requests
from unipost import UniPost

client = UniPost()
platform = "instagram"
platform_options = {"mediaType": "feed"}

# Step 1: connect the platform account once, then keep the returned account_id.
connect = client.connect.get_connect_url(platform=platform)
print("Open this URL in a browser:", connect.auth_url)

accounts = client.accounts.list(platform=platform)
account_id = accounts["data"][0]["id"]

# Step 2: reserve media and get a presigned upload URL.
file_path = Path("campaign-photo.jpg")
file_bytes = file_path.read_bytes()
reservation = client.media.upload(
    filename=file_path.name,
    content_type="image/jpeg",
    size_bytes=len(file_bytes),
)
media_id = reservation["data"]["media_id"]
upload_url = reservation["data"]["upload_url"]

# Step 3: PUT raw bytes to the upload_url.
requests.put(
    upload_url,
    data=file_bytes,
    headers={"Content-Type": "image/jpeg"},
)

# Step 4: poll until the media row is ready to publish.
media = client.media.get(media_id)
while media["data"]["status"] == "pending":
    time.sleep(1)
    media = client.media.get(media_id)

if media["data"]["status"] not in ("uploaded", "attached"):
    raise RuntimeError(f"media upload failed with status {media['data']['status']}")

platform_post = {
    "account_id": account_id,
    "caption": "New seasonal item is available today.",
    "media_ids": [media_id],
    "platform_options": platform_options,
}

# Step 5: optional preflight validation.
validation = client.posts.validate(platform_posts=[platform_post])
if not validation["data"]["valid"]:
    raise RuntimeError(validation["data"]["errors"])

# Step 6: create the post with media_ids.
post = client.posts.create(platform_posts=[platform_post])
print(post["data"]["id"])

# Step 7: get the asynchronous publishing status and result.
publishing_result = client.posts.get(post["data"]["id"])
print(publishing_result["data"]["status"])
print(publishing_result["data"]["results"])`,
  },
  {
    label: "Go SDK",
    lang: "go",
    code: `package main

import (
  "bytes"
  "context"
  "fmt"
  "log"
  "net/http"
  "os"
  "time"

  "github.com/unipost-dev/sdk-go/unipost"
)

func main() {
  ctx := context.Background()
  client := unipost.NewClient()
  platform := "instagram"
  platformOptions := map[string]any{"mediaType": "feed"}

  // Step 1: connect the platform account once, then keep the returned account_id.
  connect, err := client.Connect.GetConnectURL(ctx, &unipost.GetConnectURLParams{
    Platform: platform,
  })
  if err != nil {
    log.Fatal(err)
  }
  fmt.Println("Open this URL in a browser:", connect.AuthURL)

  accounts, err := client.Accounts.List(ctx, &unipost.ListAccountsParams{
    Platform: platform,
  })
  if err != nil {
    log.Fatal(err)
  }
  accountID := accounts[0].ID

  // Step 2: reserve media and get a presigned upload URL.
  fileBytes, err := os.ReadFile("campaign-photo.jpg")
  if err != nil {
    log.Fatal(err)
  }
  reservation, err := client.Media.Upload(ctx, &unipost.MediaUploadRequest{
    Filename:    "campaign-photo.jpg",
    ContentType: "image/jpeg",
    SizeBytes:   int64(len(fileBytes)),
  })
  if err != nil {
    log.Fatal(err)
  }

  // Step 3: PUT raw bytes to the upload_url.
  req, err := http.NewRequestWithContext(ctx, http.MethodPut, reservation.UploadURL, bytes.NewReader(fileBytes))
  if err != nil {
    log.Fatal(err)
  }
  req.Header.Set("Content-Type", "image/jpeg")
  resp, err := http.DefaultClient.Do(req)
  if err != nil {
    log.Fatal(err)
  }
  resp.Body.Close()

  // Step 4: poll until the media row is ready to publish.
  media, err := client.Media.Get(ctx, reservation.MediaID)
  if err != nil {
    log.Fatal(err)
  }
  for media.Status == "pending" {
    time.Sleep(time.Second)
    media, err = client.Media.Get(ctx, reservation.MediaID)
    if err != nil {
      log.Fatal(err)
    }
  }
  if media.Status != "uploaded" && media.Status != "attached" {
    log.Fatalf("media upload failed with status %s", media.Status)
  }

  platformPost := unipost.PlatformPost{
    AccountID:       accountID,
    Caption:         "New seasonal item is available today.",
    MediaIDs:        []string{reservation.MediaID},
    PlatformOptions: platformOptions,
  }

  // Step 5: optional preflight validation.
  validation, err := client.Posts.Validate(ctx, &unipost.ValidatePostParams{
    PlatformPosts: []unipost.PlatformPost{platformPost},
  })
  if err != nil {
    log.Fatal(err)
  }
  if !validation.Valid {
    log.Fatalf("validation failed: %+v", validation.Errors)
  }

  // Step 6: create the post with media_ids.
  post, err := client.Posts.Create(ctx, &unipost.CreatePostParams{
    PlatformPosts: []unipost.PlatformPost{platformPost},
  })
  if err != nil {
    log.Fatal(err)
  }
  fmt.Println(post.ID)

  // Step 7: get the asynchronous publishing status and result.
  publishingResult, err := client.Posts.Get(ctx, post.ID)
  if err != nil {
    log.Fatal(err)
  }
  fmt.Println(publishingResult.Status)
  fmt.Println(publishingResult.Results)
}`,
  },
  {
    label: "Java SDK",
    lang: "java",
    code: `import dev.unipost.UniPost;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpRequest.BodyPublishers;
import java.net.http.HttpResponse.BodyHandlers;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;

UniPost client = new UniPost();
var platform = "instagram";
var platformOptions = Map.of("mediaType", "feed");

// Step 1: connect the platform account once, then keep the returned account_id.
var connect = client.connect().getConnectUrl(Map.of("platform", platform));
System.out.println("Open this URL in a browser: " + connect.get("auth_url").asText());

var accounts = client.accounts().list(Map.of("platform", platform)).getData();
var accountId = accounts.get(0).get("id").asText();

// Step 2: reserve media and get a presigned upload URL.
var filePath = Path.of("campaign-photo.jpg");
var fileBytes = Files.readAllBytes(filePath);
var reservation = client.media().upload(Map.of(
    "filename", "campaign-photo.jpg",
    "content_type", "image/jpeg",
    "size_bytes", fileBytes.length
));
var mediaId = reservation.get("media_id").asText();
var uploadUrl = reservation.get("upload_url").asText();

// Step 3: PUT raw bytes to the upload_url.
HttpClient.newHttpClient().send(
    HttpRequest.newBuilder(URI.create(uploadUrl))
        .header("Content-Type", "image/jpeg")
        .PUT(BodyPublishers.ofByteArray(fileBytes))
        .build(),
    BodyHandlers.discarding()
);

// Step 4: poll until the media row is ready to publish.
var media = client.media().get(mediaId);
while (media.get("status").asText().equals("pending")) {
    Thread.sleep(1000);
    media = client.media().get(mediaId);
}
var status = media.get("status").asText();
if (!status.equals("uploaded") && !status.equals("attached")) {
    throw new RuntimeException("media upload failed with status " + status);
}

var platformPost = Map.of(
    "account_id", accountId,
    "caption", "New seasonal item is available today.",
    "media_ids", List.of(mediaId),
    "platform_options", platformOptions
);
var payload = Map.of("platform_posts", List.of(platformPost));

// Step 5: optional preflight validation.
var validation = client.posts().validate(payload);
if (!validation.get("valid").asBoolean()) {
    throw new RuntimeException(validation.get("errors").toString());
}

// Step 6: create the post with media_ids.
var post = client.posts().create(payload);
System.out.println(post.get("id").asText());

// Step 7: get the asynchronous publishing status and result.
var publishingResult = client.posts().get(post.get("id").asText());
System.out.println(publishingResult.get("status").asText());
System.out.println(publishingResult.get("results"));`,
  },
  {
    label: "REST API",
    lang: "javascript",
    code: `import { readFile } from "node:fs/promises";

const API_BASE = "https://api.unipost.dev";
const platform = "instagram";
const platformOptions = { mediaType: "feed" };
const jsonHeaders = {
  Authorization: \`Bearer \${process.env.UNIPOST_API_KEY}\`,
  "Content-Type": "application/json",
};

async function unipost(path, options = {}) {
  const response = await fetch(\`\${API_BASE}\${path}\`, {
    ...options,
    headers: { ...jsonHeaders, ...options.headers },
  });
  const body = await response.json();
  if (!response.ok) {
    throw new Error(JSON.stringify(body, null, 2));
  }
  return body.data;
}

// Step 1: connect the platform account once, then keep the returned account_id.
const connect = await unipost("/v1/oauth/connect", {
  method: "POST",
  body: JSON.stringify({ platform }),
});
console.log("Open this URL in a browser:", connect.auth_url);

const accounts = await unipost(\`/v1/accounts?platform=\${platform}\`);
const accountId = accounts[0].id;

// Step 2: reserve media and get a presigned upload URL.
const fileBuffer = await readFile("campaign-photo.jpg");
const reservation = await unipost("/v1/media", {
  method: "POST",
  body: JSON.stringify({
    filename: "campaign-photo.jpg",
    content_type: "image/jpeg",
    size_bytes: fileBuffer.byteLength,
  }),
});

// Step 3: PUT raw bytes to the upload_url.
await fetch(reservation.upload_url, {
  method: "PUT",
  headers: { "Content-Type": "image/jpeg" },
  body: fileBuffer,
});

// Step 4: poll until the media row is ready to publish.
let media = await unipost(\`/v1/media/\${reservation.media_id}\`);
while (media.status === "pending") {
  await new Promise((resolve) => setTimeout(resolve, 1000));
  media = await unipost(\`/v1/media/\${reservation.media_id}\`);
}
if (media.status !== "uploaded" && media.status !== "attached") {
  throw new Error(\`media upload failed with status \${media.status}\`);
}

const postPayload = {
  platform_posts: [
    {
      account_id: accountId,
      caption: "New seasonal item is available today.",
      media_ids: [reservation.media_id],
      platform_options: platformOptions,
    },
  ],
};

// Step 5: optional preflight validation.
const validation = await unipost("/v1/posts/validate", {
  method: "POST",
  body: JSON.stringify(postPayload),
});
if (!validation.valid) {
  throw new Error(JSON.stringify(validation.errors, null, 2));
}

// Step 6: create the post with media_ids.
const post = await unipost("/v1/posts", {
  method: "POST",
  body: JSON.stringify(postPayload),
});
console.log(post.id);

// Step 7: get the asynchronous publishing status and result.
const publishingResult = await unipost(\`/v1/posts/\${post.id}\`);
console.log(publishingResult.status);
console.log(publishingResult.results);`,
  },
];

const SEQUENCE_MESSAGES: readonly {
  from: number;
  label: string;
  response?: boolean;
  to: number;
  y: number;
}[] = [
  { from: 130, to: 455, y: 150, label: "POST /v1/media" },
  { from: 455, to: 130, y: 195, label: "media_id + upload_url, status=pending", response: true },
  { from: 130, to: 650, y: 250, label: "PUT bytes to upload_url" },
  { from: 130, to: 455, y: 300, label: "GET /v1/media/{media_id}" },
  { from: 455, to: 130, y: 345, label: "status=uploaded", response: true },
  { from: 130, to: 455, y: 395, label: "POST /v1/posts or /v1/posts/validate" },
  { from: 455, to: 130, y: 440, label: "202 accepted, post/result/job ids", response: true },
  { from: 845, to: 1060, y: 520, label: "create media container" },
  { from: 845, to: 1060, y: 565, label: "wait for container ready" },
  { from: 845, to: 1060, y: 610, label: "publish to platform" },
  { from: 845, to: 1060, y: 650, label: "fetch permalink" },
  { from: 130, to: 455, y: 650, label: "GET /v1/posts/{post_id}" },
] as const;

export function PublishingInputModeCards() {
  return (
    <div className="docs-decision-grid">
      <article className="docs-decision-card">
        <div className="docs-decision-kicker">Hosted media URL</div>
        <div className="docs-decision-endpoint">
          <ApiInlineLink endpoint="POST /v1/posts" />
        </div>
        <p>
          Use this when your image or video already has a public URL. Pass hosted
          assets in <code>platform_posts[].media_urls</code>.
        </p>
      </article>
      <article className="docs-decision-card">
        <div className="docs-decision-kicker">Local file upload</div>
        <div className="docs-decision-endpoint">
          <ApiInlineLink endpoint="POST /v1/media" />
          <span>then</span>
          <ApiInlineLink endpoint="POST /v1/posts" />
        </div>
        <p>
          Use this when your app has raw file bytes. Reserve media, PUT the bytes,
          poll until uploaded, then publish with <code>platform_posts[].media_ids</code>.
        </p>
        <a className="docs-decision-link" href="#local-file-flow">
          See the 7-step flow
        </a>
      </article>
    </div>
  );
}

export function PublishingSequenceDiagram() {
  const actors = [
    { label: "Client", x: 130 },
    { label: "UniPost API", x: 455 },
    { label: "Storage", x: 650 },
    { label: "Worker", x: 845 },
    { label: "Platform", x: 1060 },
  ] as const;

  return (
    <div
      aria-label="UniPost post API call sequence"
      style={{
        background: "#171717",
        border: "1px solid #333333",
        borderRadius: 16,
        margin: "14px 0 30px",
        overflowX: "auto",
      }}
    >
      <svg
        aria-labelledby="publishing-sequence-title"
        role="img"
        viewBox="0 0 1180 770"
        style={{
          display: "block",
          height: "auto",
          minWidth: 980,
          width: "100%",
        }}
      >
        <title id="publishing-sequence-title">Local file publishing sequence</title>
        <defs>
          <marker
            id="publishing-sequence-arrow"
            markerHeight="10"
            markerUnits="strokeWidth"
            markerWidth="10"
            orient="auto"
            refX="9"
            refY="5"
          >
            <path d="M 0 0 L 10 5 L 0 10 z" fill="#d4d4d8" />
          </marker>
        </defs>
        <rect fill="#171717" height="770" rx="16" width="1180" x="0" y="0" />
        {actors.map((actor) => (
          <line
            key={`${actor.label}-lifeline`}
            stroke="#a3a3a3"
            strokeWidth="2"
            x1={actor.x}
            x2={actor.x}
            y1="98"
            y2="690"
          />
        ))}
        {SEQUENCE_MESSAGES.map((message) => (
          <g key={`${message.label}-${message.y}`}>
            <line
              markerEnd="url(#publishing-sequence-arrow)"
              stroke="#d4d4d8"
              strokeDasharray={message.response ? "4 5" : undefined}
              strokeWidth="2"
              x1={message.from}
              x2={message.to}
              y1={message.y}
              y2={message.y}
            />
            <text
              fill="#f4f4f5"
              fontFamily="var(--docs-ui)"
              fontSize="17"
              textAnchor="middle"
              x={(message.from + message.to) / 2}
              y={message.y - 12}
            >
              {message.label}
            </text>
          </g>
        ))}
        {actors.map((actor) => (
          <g key={`${actor.label}-top`}>
            <rect
              fill="#303030"
              height="62"
              rx="4"
              stroke="#c7c7c7"
              width="150"
              x={actor.x - 75}
              y="36"
            />
            <text
              dominantBaseline="middle"
              fill="#f4f4f5"
              fontFamily="var(--docs-ui)"
              fontSize="18"
              textAnchor="middle"
              x={actor.x}
              y="67"
            >
              {actor.label}
            </text>
          </g>
        ))}
        {actors.map((actor) => (
          <g key={`${actor.label}-bottom`}>
            <rect
              fill="#303030"
              height="62"
              rx="4"
              stroke="#c7c7c7"
              width="150"
              x={actor.x - 75}
              y="690"
            />
            <text
              dominantBaseline="middle"
              fill="#f4f4f5"
              fontFamily="var(--docs-ui)"
              fontSize="18"
              textAnchor="middle"
              x={actor.x}
              y="721"
            >
              {actor.label}
            </text>
          </g>
        ))}
      </svg>
    </div>
  );
}

export function PublishingLocalFileFlow() {
  return (
    <>
      <h2 id="local-file-flow">Local file flow</h2>
      <p className="docs-note">
        The diagram and table show the same path: reserve media, upload to
        storage, confirm the media is ready, publish, then read or receive the
        asynchronous result.
      </p>
      <PublishingSequenceDiagram />
      <DocsTable
        columns={["Step", "API call", "Purpose"]}
        rows={[
          [
            "1",
            <span key="connect">
              <ApiInlineLink endpoint="POST /v1/oauth/connect" /> or{" "}
              <ApiInlineLink endpoint="POST /v1/connect/sessions" />
            </span>,
            "Connect the platform account, then keep the returned account_id or completed managed_account_id.",
          ],
          ["2", <ApiInlineLink key="reserve" endpoint="POST /v1/media" />, "Reserve a media row and receive a presigned upload_url."],
          ["3", <code key="put">PUT upload_url</code>, "Upload the raw image or video bytes directly to storage."],
          ["4", <ApiInlineLink key="get-media" endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" />, "Poll until status is uploaded or attached."],
          ["5", <ApiInlineLink key="validate" endpoint="POST /v1/posts/validate" />, "Optional preflight check for platform media rules, required metadata, and account state."],
          ["6", <ApiInlineLink key="create" endpoint="POST /v1/posts" />, "Create the post with platform_posts[].media_ids and any required platform_options."],
          [
            "7",
            <ApiInlineLink key="get-post" endpoint="GET /v1/posts/:post_id" />,
            <span key="get-post-purpose">
              Get the async publishing status and result. You can also receive
              final status through webhooks; see{" "}
              <Link href="/docs/api/posts/create#publishing-result">Publishing Result</Link>.
            </span>,
          ],
        ]}
      />
    </>
  );
}

export function PublishingLocalFileExample() {
  return (
    <>
      <h2 id="code-examples">End-to-end local file example</h2>
      <p className="docs-note">
        These examples use Instagram feed publishing as the concrete surface.
        Change <code>platform</code>, <code>account_id</code>, and{" "}
        <code>platform_options</code> to match the platform-specific guide you are
        implementing.
      </p>
      <DocsCodeTabs snippets={LOCAL_FILE_SNIPPETS} />
    </>
  );
}
