import Link from "next/link";
import { notFound } from "next/navigation";
import { DocsCodeTabs, DocsPage, DocsRichText, DocsTable } from "../../_components/docs-shell";
import { ApiInlineLink } from "../../api/_components/doc-components";
import { PLATFORMS, type PlatformDoc, type PlatformSummary } from "./_data";
import { toExampleSnippets } from "./_snippets";

const SUMMARY_LABELS: Record<keyof Omit<PlatformSummary, "connection">, string> = {
  publishing: "Publishing",
  scheduling: "Scheduling",
  analytics: "Analytics",
  inbox: "Inbox",
};

const INSTAGRAM_LOCAL_FILE_SNIPPETS = [
  {
    label: "Node SDK",
    lang: "javascript",
    code: `import { readFile } from "node:fs/promises";
import { UniPost } from "@unipost/sdk";

const client = new UniPost();

// Step 1: connect Instagram once, then keep the returned account_id.
const { auth_url: authUrl } = await client.connect.getConnectUrl({
  platform: "instagram",
});
console.log("Open this URL in a browser:", authUrl);

const { data: accounts } = await client.accounts.list({ platform: "instagram" });
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

// Step 5: optional preflight validation.
const validation = await client.posts.validate({
  platformPosts: [
    {
      accountId,
      caption: "New seasonal item is available today.",
      mediaIds: [mediaId],
      platformOptions: { mediaType: "feed" },
    },
  ],
});

if (!validation.valid) {
  throw new Error(JSON.stringify(validation.errors, null, 2));
}

// Step 6: create the Instagram post with media_ids.
const post = await client.posts.create({
  platformPosts: [
    {
      accountId,
      caption: "New seasonal item is available today.",
      mediaIds: [mediaId],
      platformOptions: { mediaType: "feed" },
    },
  ],
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

# Step 1: connect Instagram once, then keep the returned account_id.
connect = client.connect.get_connect_url(platform="instagram")
print("Open this URL in a browser:", connect.auth_url)

accounts = client.accounts.list(platform="instagram")
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

# Step 5: optional preflight validation.
validation = client.posts.validate(
    platform_posts=[
        {
            "account_id": account_id,
            "caption": "New seasonal item is available today.",
            "media_ids": [media_id],
            "platform_options": {"mediaType": "feed"},
        }
    ]
)
if not validation["data"]["valid"]:
    raise RuntimeError(validation["data"]["errors"])

# Step 6: create the Instagram post with media_ids.
post = client.posts.create(
    platform_posts=[
        {
            "account_id": account_id,
            "caption": "New seasonal item is available today.",
            "media_ids": [media_id],
            "platform_options": {"mediaType": "feed"},
        }
    ]
)

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

  // Step 1: connect Instagram once, then keep the returned account_id.
  connect, err := client.Connect.GetConnectURL(ctx, &unipost.GetConnectURLParams{
    Platform: "instagram",
  })
  if err != nil {
    log.Fatal(err)
  }
  fmt.Println("Open this URL in a browser:", connect.AuthURL)

  accounts, err := client.Accounts.List(ctx, &unipost.ListAccountsParams{
    Platform: "instagram",
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

  // Step 5: optional preflight validation.
  validation, err := client.Posts.Validate(ctx, &unipost.ValidatePostParams{
    PlatformPosts: []unipost.PlatformPost{
      {
        AccountID:       accountID,
        Caption:         "New seasonal item is available today.",
        MediaIDs:        []string{reservation.MediaID},
        PlatformOptions: map[string]any{"mediaType": "feed"},
      },
    },
  })
  if err != nil {
    log.Fatal(err)
  }
  if !validation.Valid {
    log.Fatalf("validation failed: %+v", validation.Errors)
  }

  // Step 6: create the Instagram post with media_ids.
  post, err := client.Posts.Create(ctx, &unipost.CreatePostParams{
    PlatformPosts: []unipost.PlatformPost{
      {
        AccountID:       accountID,
        Caption:         "New seasonal item is available today.",
        MediaIDs:        []string{reservation.MediaID},
        PlatformOptions: map[string]any{"mediaType": "feed"},
      },
    },
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

// Step 1: connect Instagram once, then keep the returned account_id.
var connect = client.connect().getConnectUrl(Map.of(
    "platform", "instagram"
));
System.out.println("Open this URL in a browser: " + connect.get("auth_url").asText());

var accounts = client.accounts().list(Map.of("platform", "instagram")).getData();
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

// Step 5: optional preflight validation.
var payload = Map.of(
    "platform_posts", List.of(
        Map.of(
            "account_id", accountId,
            "caption", "New seasonal item is available today.",
            "media_ids", List.of(mediaId),
            "platform_options", Map.of("mediaType", "feed")
        )
    )
);
var validation = client.posts().validate(payload);
if (!validation.get("valid").asBoolean()) {
    throw new RuntimeException(validation.get("errors").toString());
}

// Step 6: create the Instagram post with media_ids.
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
const headers = {
  Authorization: \`Bearer \${process.env.UNIPOST_API_KEY}\`,
  "Content-Type": "application/json",
};

async function unipost(path, options = {}) {
  const response = await fetch(\`\${API_BASE}\${path}\`, {
    ...options,
    headers: { ...headers, ...options.headers },
  });
  const body = await response.json();
  if (!response.ok) {
    throw new Error(JSON.stringify(body, null, 2));
  }
  return body.data;
}

// Step 1: connect Instagram once, then keep the returned account_id.
const connect = await unipost("/v1/oauth/connect", {
  method: "POST",
  body: JSON.stringify({ platform: "instagram" }),
});
console.log("Open this URL in a browser:", connect.auth_url);

const accounts = await unipost("/v1/accounts?platform=instagram");
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
      platform_options: { mediaType: "feed" },
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

// Step 6: create the Instagram post with media_ids.
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

const PLATFORM_SEQUENCE_MESSAGES: readonly {
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
  { from: 845, to: 1060, y: 610, label: "media_publish" },
  { from: 845, to: 1060, y: 650, label: "fetch permalink" },
  { from: 130, to: 455, y: 650, label: "GET /v1/posts/{post_id}" },
] as const;

function PlatformApiSequenceDiagram({ platformName }: { platformName: string }) {
  const actors = [
    { label: "Client", x: 130 },
    { label: "UniPost API", x: 455 },
    { label: "R2", x: 650 },
    { label: "Worker", x: 845 },
    { label: platformName, x: 1060 },
  ] as const;

  return (
    <div
      aria-label={`${platformName} post API call sequence`}
      style={{
        background: "#171717",
        border: "1px solid #333333",
        borderRadius: 16,
        margin: "14px 0 30px",
        overflowX: "auto",
      }}
    >
      <svg
        aria-labelledby="instagram-sequence-title"
        role="img"
        viewBox="0 0 1180 770"
        style={{
          display: "block",
          height: "auto",
          minWidth: 980,
          width: "100%",
          }}
        >
        <title id="instagram-sequence-title">{platformName} local file publishing sequence</title>
        <defs>
          <marker
            id="instagram-sequence-arrow"
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
        {PLATFORM_SEQUENCE_MESSAGES.map((message) => (
          <g key={`${message.label}-${message.y}`}>
            <line
              markerEnd="url(#instagram-sequence-arrow)"
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

function summaryBadge(value: "full" | "limited" | "none") {
  if (value === "full") return "Supported";
  if (value === "limited") return "Limited";
  return "Not supported";
}

function summaryTone(value: "full" | "limited" | "none") {
  if (value === "full") return "ok";
  if (value === "limited") return "warn";
  return "muted";
}

function PlatformApiExamples({
  examples,
  headingLevel = 2,
  title = "API Examples",
}: {
  examples: PlatformDoc["examples"];
  headingLevel?: 2 | 3;
  title?: string;
}) {
  const Heading = headingLevel === 3 ? "h3" : "h2";
  const renderExampleTitle = (title: string) => {
    const id = title.toLowerCase().replace(/[^a-z0-9]+/g, "-");

    if (headingLevel === 3) {
      return (
        <div
          id={id}
          style={{
            color: "var(--docs-text)",
            fontSize: 18,
            fontWeight: 680,
            letterSpacing: "-.015em",
            lineHeight: 1.3,
            margin: "22px 0 10px",
          }}
        >
          {title}
        </div>
      );
    }

    return (
      <h3 id={id}>
        {title}
      </h3>
    );
  };

  return (
    <>
      <Heading id="api-examples">{title}</Heading>
      <p className="docs-note">
        Each example calls <ApiInlineLink endpoint="POST /v1/posts" /> with Bearer
        auth. Swap the <code>account_ids</code> for your own, then copy the snippet
        for your language.
      </p>
      {examples.map((example) => (
        <div key={example.title}>
          {renderExampleTitle(example.title)}
          {example.note ? (
            <p className="docs-note">
              <DocsRichText text={example.note} />
            </p>
          ) : null}
          <DocsCodeTabs snippets={toExampleSnippets(example.body)} />
        </div>
      ))}
    </>
  );
}

function mediaSpecId(surface: string) {
  return `media-specs-${surface.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`;
}

function mediaSpecRows(spec: NonNullable<PlatformDoc["mediaSpecs"]>[number]) {
  return [
    ...(spec.text ?? []).map((row) => ["Text", row[0], row[1]] as const),
    ...(spec.image ?? []).map((row) => ["Image", row[0], row[1]] as const),
    ...(spec.video ?? []).map((row) => ["Video", row[0], row[1]] as const),
  ];
}

function MediaSpecsSection({ data }: { data: PlatformDoc }) {
  if (!data.mediaSpecs) return null;

  return (
    <>
      <h2 id="media-specs">Media specifications</h2>
      <p className="docs-note">
        Per-surface limits for text, images, and video. These are the source of
        truth UniPost uses for preflight validation and media optimization —
        treat hard-limit values as enforced and &quot;recommended&quot; values as
        platform guidance.
      </p>
      <div className="docs-surface-tabs" aria-label={`${data.title} media surfaces`}>
        {data.mediaSpecs.map((spec) => (
          <a className="docs-surface-tab" href={`#${mediaSpecId(spec.surface)}`} key={spec.surface}>
            {spec.surface}
          </a>
        ))}
      </div>
      {data.mediaSpecs.map((spec) => (
        <section className="docs-surface-panel" key={spec.surface}>
          <h3 id={mediaSpecId(spec.surface)}>{spec.surface}</h3>
          {spec.description ? (
            <p className="docs-note">{spec.description}</p>
          ) : null}
          <DocsTable
            columns={["Type", "Requirement", "Value"]}
            rows={mediaSpecRows(spec)}
          />
        </section>
      ))}
    </>
  );
}

function PlatformInputModeCards({
  mediaRequired,
  supportsManagedUploads,
}: {
  mediaRequired: boolean;
  supportsManagedUploads: boolean;
}) {
  return (
    <div className="docs-decision-grid">
      <article className="docs-decision-card">
        <div className="docs-decision-kicker">Direct publish</div>
        <div className="docs-decision-endpoint">
          <ApiInlineLink endpoint="POST /v1/posts" />
        </div>
        {mediaRequired ? (
          <p>
            Use this when your image or video already has a public URL. Pass hosted
            assets in <code>platform_posts[].media_urls</code>.
          </p>
        ) : (
          <p>
            Use this for text-only posts or when your media already has a public URL.
            Pass hosted assets in <code>platform_posts[].media_urls</code>.
          </p>
        )}
      </article>
      {supportsManagedUploads ? (
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
      ) : (
        <article className="docs-decision-card">
          <div className="docs-decision-kicker">Preflight first</div>
          <div className="docs-decision-endpoint">
            <ApiInlineLink endpoint="POST /v1/posts/validate" />
          </div>
          <p>
            Use validation before publish when platform limits, account state, or
            required metadata can block the request.
          </p>
        </article>
      )}
    </div>
  );
}

function PublishGuideSection({
  data,
  isInstagram,
  mediaRequired,
  supportsManagedUploads,
}: {
  data: PlatformDoc;
  isInstagram: boolean;
  mediaRequired: boolean;
  supportsManagedUploads: boolean;
}) {
  return (
    <>
      <h2 id="post-api-guide">Publish guide</h2>
      <h3 id="media-input-options">Choose input mode</h3>
      <p className="docs-note">
        Start here when you want to publish to {data.title} through the API.
        Direct publish uses <ApiInlineLink endpoint="POST /v1/posts" /> with{" "}
        {mediaRequired ? "hosted media URLs and platform-specific metadata" : "text, hosted media URLs, or platform-specific metadata"}.
        {supportsManagedUploads ? (
          <>
            {" "}Local files need one upload step first, then the final publish call uses <code>media_ids</code>.
          </>
        ) : null}{" "}
        <ApiInlineLink endpoint="POST /v1/posts" /> returns <code>202</code>; read
        the final result with <ApiInlineLink endpoint="GET /v1/posts/:post_id" /> or webhooks.
      </p>
      <PlatformInputModeCards
        mediaRequired={mediaRequired}
        supportsManagedUploads={supportsManagedUploads}
      />
      {supportsManagedUploads ? (
        <>
          <h3 id="local-file-flow">Local File Flow</h3>
          <p className="docs-note">
            The diagram shows the same local-file publish path as the table below:
            reserve media, upload to storage, confirm the media is ready, publish,
            then read or receive the asynchronous result.
          </p>
          <PlatformApiSequenceDiagram platformName={data.title} />
          <DocsTable
            columns={["Step", "API call", "Purpose"]}
            rows={[
              ["1", <ApiInlineLink key="connect" endpoint="POST /v1/oauth/connect" />, `Connect the ${data.title} account, then list accounts to keep its account_id.`],
              ["2", <ApiInlineLink key="reserve" endpoint="POST /v1/media" />, "Reserve a media row and receive a presigned upload_url."],
              ["3", <code key="put">PUT upload_url</code>, "Upload the raw image or video bytes directly to storage."],
              ["4", <ApiInlineLink key="get-media" endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" />, "Poll until status is uploaded or attached."],
              ["5", <ApiInlineLink key="validate" endpoint="POST /v1/posts/validate" />, "Optional preflight check for platform media rules, required metadata, and account state."],
              ["6", <ApiInlineLink key="create" endpoint="POST /v1/posts" />, "Create the post with platform_posts[].media_ids and any required platform_options."],
              [
                "7",
                <ApiInlineLink key="get-post" endpoint="GET /v1/posts/:post_id" />,
                <span key="get-post-purpose">
                  Get the async publishing status and result. You can also receive final status through webhooks; see{" "}
                  <Link href="/docs/api/posts/create#publishing-result">Publishing Result</Link>.
                </span>,
              ],
            ]}
          />
        </>
      ) : null}
      {isInstagram ? (
        <>
          <h3 id="code-examples">End-to-end local file example</h3>
          <DocsCodeTabs snippets={INSTAGRAM_LOCAL_FILE_SNIPPETS} />
        </>
      ) : null}
      <PlatformApiExamples
        examples={data.examples}
        headingLevel={3}
        title="Publish examples by surface"
      />
    </>
  );
}

function PlatformNextSteps({
  data,
  platform,
}: {
  data: PlatformDoc;
  platform: string;
}) {
  return (
    <div className="docs-next-grid">
      <Link href={`/docs/platforms/${platform}#api-examples`} className="docs-next-card">
        <div className="docs-next-kicker">Publish surfaces</div>
        <div className="docs-next-title">Start from a {data.title} example</div>
        <div className="docs-next-body">
          Pick the closest payload shape and swap in your own account, caption, and media.
        </div>
      </Link>
      <Link href="/docs/api/posts/create#publishing-result" className="docs-next-card">
        <div className="docs-next-kicker">Async result</div>
        <div className="docs-next-title">Track publishing status</div>
        <div className="docs-next-body">
          Read the final post result after UniPost accepts the publish request.
        </div>
      </Link>
      <Link href="/docs/api/webhooks" className="docs-next-card">
        <div className="docs-next-kicker">Push delivery</div>
        <div className="docs-next-title">Set up developer webhooks</div>
        <div className="docs-next-body">
          Receive post.published, post.partial, and post.failed events in your
          backend.
        </div>
      </Link>
      <Link href="/docs/white-label" className="docs-next-card">
        <div className="docs-next-kicker">Customer accounts</div>
        <div className="docs-next-title">Plan account connection</div>
        <div className="docs-next-body">
          Choose Quickstart or White-label based on who owns the social accounts.
        </div>
      </Link>
    </div>
  );
}

export default async function PlatformDetailPage({
  params,
}: {
  params: Promise<{ platform: string }>;
}) {
  const { platform } = await params;
  const data = PLATFORMS[platform];
  if (!data) notFound();

  const supportsManagedUploads = data.requirements.some(
    (row) => row[0].includes("media_urls") || row[0].includes("media_ids"),
  );
  const mediaRequired = data.requirements.some(
    (row) => row[0].includes("media_urls") && String(row[1]).toLowerCase() === "required",
  );
  const isInstagram = platform === "instagram";

  return (
    <DocsPage
      eyebrow="Platform Guide"
      title={data.title}
      lead={data.lead}
      className="docs-page-wide"
    >
      <div className="docs-guide-intro">
        <div
          className="docs-guide-intro-icon"
          style={{ color: data.brandColor }}
        >
          {data.icon}
        </div>
        <div className="docs-guide-intro-body">
          <div className="docs-guide-intro-title">{data.tagline}</div>
        </div>
      </div>

      {data.setupNote ? (
        <div className="docs-callout docs-callout-warning docs-callout-compact">
          <strong>{data.title} account requirement</strong>
          {data.setupNote}
        </div>
      ) : null}

      <h2 id="at-a-glance">At a glance</h2>
      <div className="docs-summary-grid">
        {(Object.keys(SUMMARY_LABELS) as (keyof typeof SUMMARY_LABELS)[]).map((key) => {
          const value = data.summary[key];
          return (
            <div key={key} className={`docs-summary-card tone-${summaryTone(value)}`}>
              <div className="docs-summary-label">{SUMMARY_LABELS[key]}</div>
              <div className="docs-summary-value">{summaryBadge(value)}</div>
            </div>
          );
        })}
      </div>
      <div className="docs-summary-connection">
        <div className="docs-summary-label">Connection</div>
        <div className="docs-summary-copy">
          {data.summary.connection}
        </div>
      </div>

      <h2 id="feature-matrix">Feature matrix</h2>
      <DocsTable columns={["Feature", "Support", "Notes"]} rows={data.capabilities} />

      <h2 id="limitations">Known constraints</h2>
      <DocsTable columns={["Limitation", "Why"]} rows={data.limitations} />

      <PublishGuideSection
        data={data}
        isInstagram={isInstagram}
        mediaRequired={mediaRequired}
        supportsManagedUploads={supportsManagedUploads}
      />

      <h2 id="media-requirements">Media &amp; field requirements</h2>
      <DocsTable
        columns={["Field", "Required", "Limits", "Notes"]}
        rows={data.requirements}
      />
      {supportsManagedUploads ? (
        <p className="docs-note">
          Hosted URLs: pass the public URL in <code>media_urls</code>. Local files:
          reserve an upload with <ApiInlineLink endpoint="POST /v1/media" />, PUT the
          bytes to the returned <code>upload_url</code>, then publish with{" "}
          <code>media_ids</code>. Full flow in{" "}
          <Link href="/docs/api/media">Media API</Link>.
        </p>
      ) : null}

      <MediaSpecsSection data={data} />

      {data.options ? (
        <>
          <h2 id="platform-options">Platform-specific options</h2>
          <DocsTable columns={["Option", "Values", "Notes"]} rows={data.options} />
        </>
      ) : null}

      <h2 id="analytics">Analytics</h2>
      <DocsTable columns={["Metric", "Support", "Notes"]} rows={data.analytics} />

      {data.inbox ? (
        <>
          <h2 id="inbox">Inbox</h2>
          {data.inbox.note ? <p className="docs-note">{data.inbox.note}</p> : null}
          <DocsTable
            columns={["Surface", "Support", "Notes"]}
            rows={data.inbox.rows}
          />
        </>
      ) : null}

      <h2 id="setup-modes">Connection modes</h2>
      <p className="docs-note">
        Pick the setup that matches how the account is owned. Quickstart is fastest
        when you publish to your own accounts; White-label is required when your
        customers bring their own accounts through a branded flow. Full setup
        details in <Link href="/docs/quickstart">Quickstart</Link> and{" "}
        <Link href="/docs/white-label">White-label</Link>.
      </p>
      <DocsTable
        columns={["Mode", "Best for", "App / credentials", "Availability"]}
        rows={data.setup}
      />
      {data.setupNote ? <p className="docs-note">{data.setupNote}</p> : null}

      <h2 id="validation-errors">Validation errors</h2>
      <DocsTable columns={["Code", "What it means"]} rows={data.errors} />

      <h2 id="next-steps">Next steps</h2>
      <PlatformNextSteps data={data} platform={platform} />
    </DocsPage>
  );
}
