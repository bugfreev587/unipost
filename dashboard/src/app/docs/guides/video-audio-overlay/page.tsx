import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";
import { ApiInlineLink } from "../../api/_components/doc-components";

const WORKFLOW_SNIPPETS = [
  {
    label: "cURL",
    lang: "bash",
    code: `# 1. Reserve and upload the video file.
curl -X POST "https://api.unipost.dev/v1/media" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "filename": "launch-clip.mp4",
    "content_type": "video/mp4"
  }'

# PUT the video bytes to data.upload_url, then poll:
curl "https://api.unipost.dev/v1/media/media_video_123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"

# 2. Reserve and upload the audio file.
curl -X POST "https://api.unipost.dev/v1/media" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "filename": "voiceover.mp3",
    "content_type": "audio/mpeg"
  }'

# PUT the audio bytes to data.upload_url, then poll GET /v1/media/{media_id}.

# 3. Create the overlay job.
curl -X POST "https://api.unipost.dev/v1/media/audio-overlays" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Idempotency-Key: overlay-user-42-launch-clip" \\
  -H "Content-Type: application/json" \\
  -d '{
    "video_media_id": "media_video_123",
    "audio_media_id": "media_audio_456",
    "mode": "mix",
    "video_volume": 35,
    "audio_volume": 100,
    "fit": "loop_to_video"
  }'

# 4. Poll until data.status is succeeded, then keep data.output_media_id.
curl "https://api.unipost.dev/v1/media/audio-overlays/mpj_123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"

# 5. Publish the processed video output.
curl -X POST "https://api.unipost.dev/v1/posts" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "caption": "Video with custom audio",
    "platform_posts": [
      {
        "account_id": "sa_tiktok_123",
        "media_ids": ["media_output_789"]
      }
    ]
  }'`,
  },
  {
    label: "Node.js SDK",
    lang: "javascript",
    code: `import { readFile } from "node:fs/promises";
import { UniPost } from "@unipost/sdk";

const client = new UniPost();

async function uploadAndWait({ path, filename, contentType }) {
  const fileBuffer = await readFile(path);
  const { mediaId, uploadUrl } = await client.media.upload({
    filename,
    contentType,
  });

  await fetch(uploadUrl, {
    method: "PUT",
    headers: { "Content-Type": contentType },
    body: fileBuffer,
  });

  let media = await client.media.get(mediaId);
  while (media.status === "pending") {
    await new Promise((resolve) => setTimeout(resolve, 1000));
    media = await client.media.get(mediaId);
  }

  if (media.status !== "uploaded" && media.status !== "attached") {
    throw new Error("Upload failed with media status " + media.status);
  }

  return mediaId;
}

const videoMediaId = await uploadAndWait({
  path: "./launch-clip.mp4",
  filename: "launch-clip.mp4",
  contentType: "video/mp4",
});

const audioMediaId = await uploadAndWait({
  path: "./voiceover.mp3",
  filename: "voiceover.mp3",
  contentType: "audio/mpeg",
});

const job = await client.media.audioOverlays.create({
  videoMediaId,
  audioMediaId,
  mode: "mix",
  videoVolume: 35,
  audioVolume: 100,
  fit: "loop_to_video",
}, {
  idempotencyKey: "overlay-user-42-launch-clip",
});

let current = job;
while (current.status === "queued" || current.status === "processing") {
  await new Promise((resolve) => setTimeout(resolve, 1500));
  current = await client.media.audioOverlays.get(job.id);
}

if (current.status !== "succeeded" || !current.outputMediaId) {
  throw new Error(current.error?.message || "Audio overlay failed");
}

await client.posts.create({
  caption: "Video with custom audio",
  platformPosts: [{
    accountId: "sa_tiktok_123",
    mediaIds: [current.outputMediaId],
  }],
});`,
  },
];

export default function VideoAudioOverlayGuidePage() {
  return (
    <DocsPage
      eyebrow="Publishing Guides"
      title="Overlay user audio onto a video"
      lead="Use this workflow when your customer has one video file and one audio file, and your product needs to publish a normal video with that audio already combined."
      className="docs-page-wide"
    >
      <div className="docs-callout docs-callout-warning">
        <strong>Important platform limitation:</strong> this guide does not attach arbitrary audio to an image post or carousel.
        TikTok and Instagram API publishing do not expose the same manual editor flow where a user picks multiple photos and adds a
        song. UniPost first creates a processed video, then the publish API receives that video as regular media.
      </div>

      <h2 id="when-to-use">When to use this guide</h2>
      <p>
        Use this guide when a user uploads a video plus a separate voiceover, music bed, narration, or other audio track.
        UniPost stores both inputs with <ApiInlineLink endpoint="POST /v1/media" />, creates an async processing job with{" "}
        <ApiInlineLink endpoint="POST /v1/media/audio-overlays" />, then publishes the returned <code>output_media_id</code>{" "}
        with <ApiInlineLink endpoint="POST /v1/posts" />.
      </p>
      <p>
        The resulting asset is a normal video. That keeps the final publish flow predictable across platforms and avoids asking the
        user to understand platform-specific music-library restrictions.
      </p>

      <h2 id="recommended-product-flow">Recommended product flow</h2>
      <DocsTable
        columns={["User-facing step", "What your app should show", "UniPost API work"]}
        rows={[
          [
            "Pick video and audio",
            "Show two file slots: the base video and the audio to add. Make ownership or licensing expectations explicit before upload.",
            "No API call yet.",
          ],
          [
            "Upload the video",
            "Keep the UI in an upload state until the video is usable. Do not ask the user to calculate file size.",
            <>
              Reserve with <ApiInlineLink endpoint="POST /v1/media" />, PUT bytes to <code>upload_url</code>, then poll{" "}
              <ApiInlineLink endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" /> until the media is uploaded.
            </>,
          ],
          [
            "Upload the audio",
            "Treat audio as an input file, not a publishable asset. Label it as the audio track for the generated video.",
            <>
              Use the same <ApiInlineLink endpoint="POST /v1/media" /> upload flow with an audio MIME type such as{" "}
              <code>audio/mpeg</code> or <code>audio/wav</code>.
            </>,
          ],
          [
            "Choose audio behavior",
            "Offer plain-language controls: keep original sound under the new audio, or replace the original sound entirely.",
            <>
              Send <code>mode: "mix"</code> or <code>mode: "replace"</code> to{" "}
              <ApiInlineLink endpoint="POST /v1/media/audio-overlays" />.
            </>,
          ],
          [
            "Generate processed video",
            "Show a processing state with a retry-safe action. The user should not have to repeat uploads if processing is still running.",
            <>
              Poll the audio overlay job with <code>GET /v1/media/audio-overlays/&#123;id&#125;</code> until{" "}
              <code>status</code> is <code>succeeded</code> or <code>failed</code>.
            </>,
          ],
          [
            "Publish output",
            "Use the generated video in the normal composer and make it clear this is the final video being posted.",
            <>
              Publish <code>output_media_id</code> through <ApiInlineLink endpoint="POST /v1/posts" /> as a regular video{" "}
              <code>media_id</code>.
            </>,
          ],
        ]}
      />

      <h2 id="audio-controls">Audio controls to expose</h2>
      <DocsTable
        columns={["User intent", "API settings", "UX copy"]}
        rows={[
          [
            "Add music or narration while keeping some original video sound",
            <>
              <code>mode: "mix"</code>, lower <code>video_volume</code>, keep <code>audio_volume</code> near full volume.
            </>,
            "Keep original sound",
          ],
          [
            "Replace the video's existing audio completely",
            <>
              <code>mode: "replace"</code>. <code>video_volume</code> is ignored for the final output.
            </>,
            "Replace original sound",
          ],
          [
            "Use only the beginning of a longer audio file",
            <code>fit: "trim_to_video"</code>,
            "Stop audio at the end of the video",
          ],
          [
            "Repeat a short audio file until the video ends",
            <code>fit: "loop_to_video"</code>,
            "Loop audio to match video length",
          ],
        ]}
      />

      <h2 id="steps">API steps</h2>
      <ol className="docs-step-list">
        <li>
          Reserve a video upload with <ApiInlineLink endpoint="POST /v1/media" />. The request can omit <code>size_bytes</code>;
          UniPost hydrates size after the upload lands.
        </li>
        <li>
          PUT the raw video bytes to the returned <code>upload_url</code>, then poll{" "}
          <ApiInlineLink endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" /> until the video is uploaded.
        </li>
        <li>
          Reserve and upload the audio file with the same media upload flow. Keep the returned audio <code>media_id</code> for
          processing only.
        </li>
        <li>
          Create the audio overlay job with <ApiInlineLink endpoint="POST /v1/media/audio-overlays" />. Use an{" "}
          <code>Idempotency-Key</code> tied to the user's generate action so browser retries do not create duplicate jobs.
        </li>
        <li>
          Poll <code>GET /v1/media/audio-overlays/&#123;id&#125;</code>. When <code>status</code> is <code>succeeded</code>,
          read <code>output_media_id</code>.
        </li>
        <li>
          Publish the final video by passing <code>output_media_id</code> in <code>media_ids</code> on{" "}
          <ApiInlineLink endpoint="POST /v1/posts" />.
        </li>
      </ol>

      <h2 id="example">Example workflow</h2>
      <DocsCodeTabs snippets={WORKFLOW_SNIPPETS} />

      <h2 id="failure-handling">Failure handling</h2>
      <p>
        Keep uploaded input media IDs so the user can retry processing without uploading the same files again. If the overlay job
        returns <code>status: "failed"</code>, show the job error message, keep the selected audio settings visible, and let the
        user retry with the same <code>video_media_id</code> and <code>audio_media_id</code>.
      </p>
      <p>
        If publishing fails after processing succeeds, do not rerun the overlay job. Reuse the same <code>output_media_id</code>
        and retry the publish flow according to the post error contract.
      </p>

      <h2 id="reference">Reference</h2>
      <div className="docs-next-grid">
        <Link href="/docs/api/media/reserve" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Reserve media upload</div>
          <div className="docs-next-body">Upload local video and audio files without asking users to provide file size.</div>
        </Link>
        <Link href="/docs/api/media/audio-overlays" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Create audio overlay</div>
          <div className="docs-next-body">Exact request body, fit modes, idempotency behavior, and job response shape.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
