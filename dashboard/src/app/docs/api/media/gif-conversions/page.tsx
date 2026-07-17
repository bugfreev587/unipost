"use client";

import { ApiInlineLink, EnumValues, InfoBox, type ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
  { name: "Idempotency-Key", type: "string", meta: "Optional POST header", description: "Replays the same normalized conversion without consuming another rolling fair-use unit. Reusing the key with different input returns idempotency_conflict." },
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
    code: `set -euo pipefail

JOB=$(curl -fSs -X POST "https://api.unipost.dev/v1/media/gif-conversions" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Idempotency-Key: gif-demo-001" \\
  -H "Content-Type: application/json" \\
  -d '{
    "gif_media_id": "media_gif_123",
    "background_color": "#FFFFFF"
  }')

JOB_ID=$(echo "$JOB" | jq -er '.data.id')

DEADLINE=$((SECONDS + 900))
while (( SECONDS < DEADLINE )); do
  CURRENT=$(curl -fSs "https://api.unipost.dev/v1/media/gif-conversions/$JOB_ID" \\
    -H "Authorization: Bearer $UNIPOST_API_KEY")
  STATUS=$(echo "$CURRENT" | jq -er '.data.status')
  case "$STATUS" in
    succeeded) echo "$CURRENT" | jq; exit 0 ;;
    failed) echo "$CURRENT" | jq >&2; exit 1 ;;
    queued|processing) sleep 2 ;;
    *) echo "Unexpected conversion status: $STATUS" >&2; echo "$CURRENT" | jq >&2; exit 1 ;;
  esac
done

echo "Timed out waiting for the GIF conversion; the server-side job was not cancelled. Continue polling JOB_ID=$JOB_ID." >&2
exit 1`,
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
    label: "GET 200",
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
        POST always returns 202 when a job is accepted, including an idempotent replay. GET returns 200 when the job can be read. Idempotency-Key applies only to POST.
      </InfoBox>
      <InfoBox>
        Published UniPost SDK packages do not yet include GIF conversion helpers. Use the REST example on this page until updated SDK versions are released.
      </InfoBox>
      <InfoBox>
        <code>universal_mp4_v1</code> is H.264, yuv420p, constant 30 FPS, silent, fast-start MP4 with even dimensions and no upscaling above a 1920-pixel longest edge. Transparent pixels use the selected background color. Static GIFs and short animations produce at least five seconds of video; short animations repeat complete cycles.
      </InfoBox>
      <InfoBox>
        Input limits: 50 MB compressed, 4096 pixels per dimension, 2,000 frames, 1.5 billion decoded pixels, and a 60-second animation cycle. Processing has a five-minute processing limit, and the converted MP4 cannot exceed the global 4 GB Media limit.
      </InfoBox>
      <InfoBox>
        The cURL example waits up to 15 minutes because queue time is separate from the five-minute server processing limit. A client timeout does not cancel the server-side job; keep the job ID and continue polling <code>GET /v1/media/gif-conversions/&#123;id&#125;</code>.
      </InfoBox>
      <InfoBox>
        Terminal job codes include <code>gif_dimensions_exceeded</code>, <code>gif_frame_count_exceeded</code>, <code>gif_decode_budget_exceeded</code>, <code>gif_duration_exceeded</code>, <code>gif_probe_failed</code>, <code>gif_decode_failed</code>, <code>processing_timeout</code>, <code>output_size_exceeded</code>, and <code>gif_conversion_failed</code>. Read <code>error.retryable</code> before deciding whether to submit a new job.
      </InfoBox>
      <InfoBox>
        Active Media Processing capacity is shared with Audio Overlay. New GIF jobs also have a Plan-based rolling 24-hour limit. Active inputs are protected from cleanup. After a successful conversion, UniPost retains both the input GIF and output MP4 for Free 1 day, API 2 days, Basic 4 days, Growth 15 days, and Team and Enterprise 30 days. After a failed conversion, no output Media exists; the input GIF is retained for Free 2 days, API 4 days, Basic 8 days, Growth 30 days, and Team and Enterprise 60 days. See <ApiInlineLink endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" /> for the shared Plan retention lifecycle.
      </InfoBox>
      <InfoBox>
        Plan limits are active Media Processing jobs / new GIF conversions in a rolling 24-hour window: Free: 1 active / 10 GIF conversions; API: 2 / 50; Basic: 2 / 100; Growth: 4 / 300; Team: 6 / 1,000; Enterprise: 6 / 1,000 by default, with contract overrides available.
      </InfoBox>
    </SingleEndpointReferencePage>
  );
}
