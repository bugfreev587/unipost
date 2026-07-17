"use client";

import { ApiInlineLink, EnumValues, InfoBox, type ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
  { name: "Idempotency-Key", type: "string", meta: "Optional header", description: "Replays the same normalized conversion without consuming another rolling fair-use unit. Reusing the key with different input returns idempotency_conflict." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "gif_media_id", type: "string", description: <>An uploaded <code>image/gif</code> Media ID from <ApiInlineLink endpoint="POST /v1/media" href="/docs/api/media/reserve" />. The actual stored object must belong to the current workspace and be 50 MB or smaller.</> },
  { name: "background_color?", type: "string", defaultValue: "#FFFFFF", description: "Opaque six-digit #RRGGBB color used behind transparent pixels. UniPost normalizes it to uppercase." },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "GIF conversion job ID." },
  { name: "kind", type: "string", description: <><EnumValues values={["gif_to_mp4"]} /></> },
  { name: "status", type: "string", description: <>Current public job state.<EnumValues values={["queued", "processing", "succeeded", "failed"]} /></> },
  { name: "gif_media_id", type: "string", description: "Input GIF Media ID." },
  { name: "background_color", type: "string", description: "Normalized opaque background color." },
  { name: "output_profile", type: "string", description: <><EnumValues values={["universal_mp4_v1"]} /></> },
  { name: "output_media_id", type: "string | null", description: "Converted video Media ID. Present only after status is succeeded." },
  { name: "created_at", type: "string", description: "ISO timestamp when the job was created." },
  { name: "started_at", type: "string | null", description: "ISO timestamp when the worker first claimed the job." },
  { name: "completed_at", type: "string | null", description: "ISO timestamp when processing succeeded or failed." },
  { name: "error", type: "object | null", description: "Stable code, customer-safe message, and retryable flag for a failed job." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: "Stable creation error such as media_not_found, gif_media_required, gif_size_exceeded, media_processing_capacity_exceeded, or gif_conversion_rate_limit_exceeded." },
  { name: "error.details.reset_at", type: "string", description: "Present for the rolling 24-hour conversion limit." },
  { name: "Retry-After", type: "integer", meta: "Response header", description: "Seconds before the caller should retry a 429 response." },
  { name: "request_id", type: "string", description: "Request identifier for support and diagnostics." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `JOB=$(curl -sS -X POST "https://api.unipost.dev/v1/media/gif-conversions" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Idempotency-Key: gif-demo-001" \\
  -H "Content-Type: application/json" \\
  -d '{
    "gif_media_id": "media_gif_123",
    "background_color": "#FFFFFF"
  }')

JOB_ID=$(echo "$JOB" | jq -r '.data.id')

while true; do
  CURRENT=$(curl -sS "https://api.unipost.dev/v1/media/gif-conversions/$JOB_ID" \\
    -H "Authorization: Bearer $UNIPOST_API_KEY")
  STATUS=$(echo "$CURRENT" | jq -r '.data.status')
  case "$STATUS" in
    succeeded|failed) echo "$CURRENT" | jq; break ;;
  esac
  sleep 2
done`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();
const job = await client.media.gifConversions.create({
  gifMediaId: "media_gif_123",
  backgroundColor: "#FFFFFF",
}, { idempotencyKey: "gif-demo-001" });

const completed = await client.media.gifConversions.wait(job.id, {
  pollIntervalMs: 2000,
  timeoutMs: 300000,
});

if (completed.status !== "succeeded") {
  throw new Error(completed.error?.message || "GIF conversion failed");
}
console.log(completed.outputMediaId);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()
job = client.media.gif_conversions.create(
    gif_media_id="media_gif_123",
    background_color="#FFFFFF",
    idempotency_key="gif-demo-001",
)
completed = client.media.gif_conversions.wait(
    job.id,
    poll_interval=2.0,
    timeout=300.0,
)
print(completed.output_media_id)`,
  },
  {
    lang: "go",
    label: "Go",
    code: `job, err := client.Media.GIFConversions.Create(ctx, &unipost.GIFConversionCreateRequest{
  GIFMediaID: "media_gif_123",
  BackgroundColor: "#FFFFFF",
}, unipost.WithIdempotencyKey("gif-demo-001"))
if err != nil {
  log.Fatal(err)
}

completed, err := client.Media.GIFConversions.Wait(ctx, job.ID, unipost.WithPollInterval(2*time.Second))
if err != nil {
  log.Fatal(err)
}
fmt.Println(completed.OutputMediaID)`,
  },
  {
    lang: "java",
    label: "Java",
    code: `var job = client.media().gifConversions().create(
    new GifConversionRequest("media_gif_123", "#FFFFFF"),
    "gif-demo-001"
);

var completed = client.media().gifConversions().waitFor(
    job.id(), Duration.ofSeconds(2), Duration.ofMinutes(5)
);
System.out.println(completed.outputMediaId());`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "202",
    code: `{
  "data": {
    "id": "mpj_123",
    "kind": "gif_to_mp4",
    "status": "queued",
    "gif_media_id": "media_gif_123",
    "background_color": "#FFFFFF",
    "output_profile": "universal_mp4_v1",
    "output_media_id": null,
    "created_at": "2026-07-17T12:00:00Z",
    "started_at": null,
    "completed_at": null,
    "error": null
  }
}`,
  },
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "mpj_123",
    "kind": "gif_to_mp4",
    "status": "succeeded",
    "gif_media_id": "media_gif_123",
    "background_color": "#FFFFFF",
    "output_profile": "universal_mp4_v1",
    "output_media_id": "media_mp4_456",
    "created_at": "2026-07-17T12:00:00Z",
    "started_at": "2026-07-17T12:00:02Z",
    "completed_at": "2026-07-17T12:00:06Z",
    "error": null
  }
}`,
  },
];

export default function GIFConversionsPage() {
  return (
    <SingleEndpointReferencePage
      section="publishing"
      title="Convert GIF to MP4"
      description={<>Creates an asynchronous GIF-to-MP4 job. Poll <code>GET /v1/media/gif-conversions/&#123;id&#125;</code>, then publish the successful <code>output_media_id</code> as a normal video with <ApiInlineLink endpoint="POST /v1/posts" />.</>}
      guideLinks={[{ label: "Publish GIFs", href: "/docs/guides/publish-gifs" }]}
      method="POST"
      path="/v1/media/gif-conversions"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "202", fields: RESPONSE_FIELDS },
        { code: "200", fields: RESPONSE_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "429", fields: ERROR_FIELDS },
        { code: "503", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    >
      <InfoBox>
        Conversion and publishing are separate operations. This endpoint never edits a draft or publishes automatically. X and Facebook can receive the original GIF directly; use the MP4 output for destinations that require video.
      </InfoBox>
      <InfoBox>
        <code>universal_mp4_v1</code> is H.264, yuv420p, constant 30 FPS, silent, fast-start MP4 with even dimensions and no upscaling above a 1920-pixel longest edge. Transparent pixels use the selected background color.
      </InfoBox>
      <InfoBox>
        Active Media Processing capacity is shared with Audio Overlay. New GIF jobs also have a Plan-based rolling 24-hour limit. Input and output Media follow Plan retention after the job reaches a terminal state; active inputs are protected from cleanup.
      </InfoBox>
    </SingleEndpointReferencePage>
  );
}
