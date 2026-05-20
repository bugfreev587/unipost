import Link from "next/link";
import { notFound } from "next/navigation";
import { DocsCodeTabs, DocsPage, DocsRichText, DocsTable } from "../../_components/docs-shell";
import { ApiInlineLink } from "../../api/_components/doc-components";
import { PLATFORMS, type PlatformSummary } from "./_data";
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

console.log(post.id);`,
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

print(post["data"]["id"])`,
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
System.out.println(post.get("id").asText());`,
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

console.log(post.id);`,
  },
];

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
          <div className="docs-badge-row">
            {data.badges.map((badge) => (
              <span className="docs-badge" key={badge}>
                {badge}
              </span>
            ))}
          </div>
        </div>
      </div>

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
        <div className="docs-summary-card docs-summary-card-wide">
          <div className="docs-summary-label">Connection</div>
          <div className="docs-summary-copy">
            {data.summary.connection}
          </div>
        </div>
      </div>

      {platform === "instagram" ? (
        <>
          <h2 id="post-api-guide">Post API Guide</h2>
          <p className="docs-note">
            Start here when you want to publish an Instagram post through the API.
            Hosted media can go straight into <code>media_urls</code>. Local files
            need one upload step first, then the final publish call uses{" "}
            <code>media_ids</code>. <ApiInlineLink endpoint="POST /v1/posts" />{" "}
            returns <code>202</code>; read the final result with{" "}
            <ApiInlineLink endpoint="GET /v1/posts/:post_id" /> or webhooks.
          </p>
          <DocsTable
            columns={["Pattern", "API path", "When to use it"]}
            rows={[
              ["Hosted media URL", <ApiInlineLink key="hosted-create" endpoint="POST /v1/posts" />, "Your image or video already has a public URL. Pass it in platform_posts[].media_urls."],
              ["Local file upload", <ApiInlineLink key="local-reserve" endpoint="POST /v1/media" />, "Your app has raw file bytes. Reserve media, upload bytes, poll until uploaded, then publish with platform_posts[].media_ids."],
            ]}
          />
          <h3 id="local-file-flow">Local file flow</h3>
          <DocsTable
            columns={["Step", "API call", "Purpose"]}
            rows={[
              ["1", <ApiInlineLink key="connect" endpoint="POST /v1/oauth/connect" />, "Connect the Instagram Business or Creator account, then list accounts to keep its account_id."],
              ["2", <ApiInlineLink key="reserve" endpoint="POST /v1/media" />, "Reserve a media row and receive a presigned upload_url."],
              ["3", <code key="put">PUT upload_url</code>, "Upload the raw image or video bytes directly to storage."],
              ["4", <ApiInlineLink key="get-media" endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" />, "Poll until status is uploaded or attached."],
              ["5", <ApiInlineLink key="validate" endpoint="POST /v1/posts/validate" />, "Optional preflight check for Instagram media rules and account state."],
              ["6", <ApiInlineLink key="create" endpoint="POST /v1/posts" />, "Create the post with platform_posts[].media_ids and platform_options.mediaType."],
            ]}
          />
          <DocsCodeTabs snippets={INSTAGRAM_LOCAL_FILE_SNIPPETS} />
        </>
      ) : null}

      <h2 id="feature-matrix">Feature matrix</h2>
      <DocsTable columns={["Feature", "Support", "Notes"]} rows={data.capabilities} />

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

      {data.mediaSpecs ? (
        <>
          <h2 id="media-specs">Media specifications</h2>
          <p className="docs-note">
            Per-surface limits for text, images, and video. These are the source of
            truth UniPost uses for preflight validation and media optimization —
            treat hard-limit values as enforced and &quot;recommended&quot; values as
            platform guidance.
          </p>
          {data.mediaSpecs.map((spec) => (
            <section key={spec.surface}>
              <h3
                id={`media-specs-${spec.surface.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`}
              >
                {spec.surface}
              </h3>
              {spec.description ? (
                <p className="docs-note">{spec.description}</p>
              ) : null}
              {spec.text ? (
                <DocsTable
                  columns={["Text", "Value"]}
                  rows={spec.text.map((row) => [row[0], row[1]])}
                />
              ) : null}
              {spec.image ? (
                <DocsTable
                  columns={["Image", "Value"]}
                  rows={spec.image.map((row) => [row[0], row[1]])}
                />
              ) : null}
              {spec.video ? (
                <DocsTable
                  columns={["Video", "Value"]}
                  rows={spec.video.map((row) => [row[0], row[1]])}
                />
              ) : null}
            </section>
          ))}
        </>
      ) : null}

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

      <h2 id="api-examples">API examples</h2>
      <p className="docs-note">
        Each example calls <ApiInlineLink endpoint="POST /v1/posts" /> with Bearer
        auth. Swap the <code>account_ids</code> for your own, then copy the snippet
        for your language.
      </p>
      {data.examples.map((example) => (
        <div key={example.title}>
          <h3
            id={example.title.toLowerCase().replace(/[^a-z0-9]+/g, "-")}
          >
            {example.title}
          </h3>
          {example.note ? (
            <p className="docs-note">
              <DocsRichText text={example.note} />
            </p>
          ) : null}
          <DocsCodeTabs snippets={toExampleSnippets(example.body)} />
        </div>
      ))}

      <h2 id="limitations">Limitations</h2>
      <DocsTable columns={["Limitation", "Why"]} rows={data.limitations} />

      <h2 id="validation-errors">Validation errors</h2>
      <DocsTable columns={["Code", "What it means"]} rows={data.errors} />

      <h2 id="next-steps">Next steps</h2>
      <div className="docs-next-grid">
        <Link href="/docs/quickstart" className="docs-next-card">
          <div className="docs-next-kicker">Start publishing</div>
          <div className="docs-next-title">Quickstart</div>
          <div className="docs-next-body">
            Get an API key, connect this platform, and send your first post.
          </div>
        </Link>
        <Link href="/docs/api/posts/create" className="docs-next-card">
          <div className="docs-next-kicker">API reference</div>
          <div className="docs-next-title">Create post</div>
          <div className="docs-next-body">
            Full request / response schema for the publish endpoint.
          </div>
        </Link>
        <Link href="/docs/api/posts/validate" className="docs-next-card">
          <div className="docs-next-kicker">Preflight</div>
          <div className="docs-next-title">Validate post</div>
          <div className="docs-next-body">
            Catch caption and media issues before you hit publish.
          </div>
        </Link>
        <Link href="/docs/white-label" className="docs-next-card">
          <div className="docs-next-kicker">For customer accounts</div>
          <div className="docs-next-title">White-label</div>
          <div className="docs-next-body">
            Branded Connect flows that run against your own OAuth app.
          </div>
        </Link>
      </div>
    </DocsPage>
  );
}
