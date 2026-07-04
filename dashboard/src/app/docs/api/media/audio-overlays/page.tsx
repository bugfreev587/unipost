"use client";

import { ApiInlineLink, EnumValues, InfoBox, type ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
  { name: "Idempotency-Key", type: "string", meta: "Optional header", description: "Use the same key to replay the original job instead of creating duplicate processing work. A different request body with the same key returns idempotency_conflict." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "video_media_id", type: "string", description: <>Uploaded video media ID from <ApiInlineLink endpoint="POST /v1/media" href="/docs/api/media/reserve" />. Must reference a video in the same workspace.</> },
  { name: "audio_media_id", type: "string", description: <>Uploaded audio media ID from <ApiInlineLink endpoint="POST /v1/media" href="/docs/api/media/reserve" />. Audio files are processing inputs and cannot be published directly.</> },
  { name: "mode?", type: "string", defaultValue: "mix", description: <>How UniPost combines audio. Omit it to mix the uploaded audio with the video's original audio.<EnumValues values={["mix", "replace"]} /></> },
  { name: "video_volume?", type: "number", defaultValue: "100", description: "Original video audio volume from 0 to 100. Used in mix mode." },
  { name: "audio_volume?", type: "number", defaultValue: "100", description: "Uploaded audio volume from 0 to 100." },
  { name: "audio_start_ms?", type: "number", defaultValue: "0", description: "Offset into the uploaded audio before mixing." },
  { name: "fit?", type: "string", defaultValue: "trim_to_video", description: <>How uploaded audio is fitted to the video duration.<EnumValues values={["trim_to_video", "loop_to_video"]} /></> },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Audio overlay job ID." },
  { name: "status", type: "string", description: <>Current job state.<EnumValues values={["queued", "processing", "succeeded", "failed"]} /></> },
  { name: "video_media_id", type: "string", description: "Input video media ID." },
  { name: "audio_media_id", type: "string", description: "Input audio media ID." },
  { name: "output_media_id", type: "string | null", description: "Processed video media ID. Present only after the job succeeds." },
  { name: "mode", type: "string", description: "Resolved audio mode." },
  { name: "fit", type: "string", description: "Resolved fit mode." },
  { name: "created_at", type: "string", description: "ISO timestamp when the job was created." },
  { name: "started_at", type: "string | null", description: "ISO timestamp when processing started." },
  { name: "completed_at", type: "string | null", description: "ISO timestamp when processing completed or failed." },
  { name: "error", type: "object | null", description: "Processing failure details when status is failed." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "VALIDATION_ERROR", "IDEMPOTENCY_CONFLICT", "NOT_FOUND", or "INTERNAL_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "validation_error" or "idempotency_conflict".' },
  { name: "error.issues[]", type: "array", description: "Field-level validation details for invalid media IDs, unsupported mode or fit, invalid volume, or invalid audio offset." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/media/audio-overlays" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Idempotency-Key: overlay-demo-001" \\
  -H "Content-Type: application/json" \\
  -d '{
    "video_media_id": "media_video_123",
    "audio_media_id": "media_audio_456",
    "mode": "mix",
    "video_volume": 70,
    "audio_volume": 100,
    "audio_start_ms": 0,
    "fit": "trim_to_video"
  }'`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const job = await client.media.audioOverlays.create({
  videoMediaId: "media_video_123",
  audioMediaId: "media_audio_456",
  mode: "mix",
  videoVolume: 70,
  audioVolume: 100,
  fit: "trim_to_video",
}, { idempotencyKey: "overlay-demo-001" });

let current = job;
while (current.status === "queued" || current.status === "processing") {
  await new Promise((resolve) => setTimeout(resolve, 1500));
  current = await client.media.audioOverlays.get(job.id);
}

if (current.status !== "succeeded") {
  throw new Error(current.error?.message || "audio overlay failed");
}

console.log(current.outputMediaId);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from time import sleep
from unipost import UniPost

client = UniPost()

job = client.media.audio_overlays.create(
    video_media_id="media_video_123",
    audio_media_id="media_audio_456",
    mode="replace",
    fit="loop_to_video",
    idempotency_key="overlay-demo-001",
)

while job.status in ("queued", "processing"):
    sleep(1.5)
    job = client.media.audio_overlays.get(job.id)

print(job.output_media_id)`,
  },
  {
    lang: "go",
    label: "Go",
    code: `job, err := client.Media.AudioOverlays.Create(ctx, &unipost.AudioOverlayCreateRequest{
  VideoMediaID: "media_video_123",
  AudioMediaID: "media_audio_456",
  Mode: "mix",
  Fit: "trim_to_video",
}, unipost.WithIdempotencyKey("overlay-demo-001"))
if err != nil {
  log.Fatal(err)
}

fmt.Println(job.ID)`,
  },
  {
    lang: "java",
    label: "Java",
    code: `var job = client.media().audioOverlays().create(Map.of(
    "video_media_id", "media_video_123",
    "audio_media_id", "media_audio_456",
    "mode", "mix",
    "fit", "trim_to_video"
), "overlay-demo-001");

System.out.println(job.get("id").asText());`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "202",
    code: `{
  "data": {
    "id": "mpj_123",
    "status": "queued",
    "video_media_id": "media_video_123",
    "audio_media_id": "media_audio_456",
    "output_media_id": null,
    "mode": "mix",
    "fit": "trim_to_video",
    "created_at": "2026-07-03T12:00:00Z",
    "started_at": null,
    "completed_at": null,
    "error": null
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "mpj_123",
    "status": "succeeded",
    "video_media_id": "media_video_123",
    "audio_media_id": "media_audio_456",
    "output_media_id": "media_output_789",
    "mode": "mix",
    "fit": "trim_to_video",
    "created_at": "2026-07-03T12:00:00Z",
    "started_at": "2026-07-03T12:00:04Z",
    "completed_at": "2026-07-03T12:00:21Z",
    "error": null
  },
  "request_id": "req_123"
}`,
  },
];

export default function AudioOverlayPage() {
  return (
    <SingleEndpointReferencePage
      section="publishing"
      title="Create audio overlay"
      description={<>Creates an async media-processing job that combines one uploaded video with one uploaded audio file. Poll the job until it returns <code>status: "succeeded"</code>, then publish <code>output_media_id</code> with <ApiInlineLink endpoint="POST /v1/posts" />.</>}
      guideLinks={[{ label: "Video + audio overlay", href: "/docs/guides/video-audio-overlay" }]}
      method="POST"
      path="/v1/media/audio-overlays"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "202", fields: RESPONSE_FIELDS },
        { code: "200", fields: RESPONSE_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    >
      <InfoBox>
        Use this endpoint for functionality, not platform music libraries. TikTok and Instagram API publishing do not let API clients attach arbitrary audio to image or carousel posts. UniPost creates a normal processed video instead, so the publish API receives a regular video <code>media_id</code>.
      </InfoBox>
    </SingleEndpointReferencePage>
  );
}
