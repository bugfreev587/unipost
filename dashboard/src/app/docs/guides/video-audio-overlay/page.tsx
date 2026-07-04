import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";
import { ApiInlineLink } from "../../api/_components/doc-components";

const WORKFLOW_SNIPPETS = [
  {
    label: "cURL",
    lang: "bash",
    code: `# Step 1: Upload the video input.
# POST /v1/media creates a media row and returns a presigned upload_url.
curl -X POST "https://api.unipost.dev/v1/media" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "filename": "launch-clip.mp4",
    "content_type": "video/mp4"
  }'

# Upload the video file bytes to data.upload_url, then poll GET /v1/media/{media_id}
# until UniPost marks the video as uploaded. upload_url is the file upload destination,
# not another UniPost JSON endpoint.
curl "https://api.unipost.dev/v1/media/media_video_123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"

# Step 2: Upload the audio input.
# Audio media is an input for processing; do not publish the raw audio media_id.
curl -X POST "https://api.unipost.dev/v1/media" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "filename": "voiceover.mp3",
    "content_type": "audio/mpeg"
  }'

# Upload the audio file bytes to data.upload_url, then poll GET /v1/media/{media_id}
# until UniPost marks the audio as uploaded.

# Step 3: Generate the overlay video.
# mode=mix keeps some original video sound; mode=replace removes it.
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

# Poll the processing job until it finishes. When status is succeeded,
# keep data.output_media_id for publishing.
curl "https://api.unipost.dev/v1/media/audio-overlays/mpj_123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"

# Step 4: Publish the processed video.
# output_media_id is a normal video media_id for POST /v1/posts.
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

  // POST /v1/media reserves the upload. SDK 0.5.0+ does not require sizeBytes.
  const { mediaId, uploadUrl } = await client.media.upload({
    filename,
    contentType,
  });

  // Upload raw bytes to the presigned upload_url returned by UniPost.
  await fetch(uploadUrl, {
    method: "PUT",
    headers: { "Content-Type": contentType },
    body: fileBuffer,
  });

  // Poll GET /v1/media/{media_id} until the input file is usable.
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

// Step 1: Upload the video input.
const videoMediaId = await uploadAndWait({
  path: "./launch-clip.mp4",
  filename: "launch-clip.mp4",
  contentType: "video/mp4",
});

// Step 2: Upload the audio input.
const audioMediaId = await uploadAndWait({
  path: "./voiceover.mp3",
  filename: "voiceover.mp3",
  contentType: "audio/mpeg",
});

// Step 3: Generate the overlay video.
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

// Poll the processing job until it finishes.
let current = job;
while (current.status === "queued" || current.status === "processing") {
  await new Promise((resolve) => setTimeout(resolve, 1500));
  current = await client.media.audioOverlays.get(job.id);
}

if (current.status !== "succeeded" || !current.outputMediaId) {
  throw new Error(current.error?.message || "Audio overlay failed");
}

// Step 4: Publish the processed video.
await client.posts.create({
  caption: "Video with custom audio",
  platformPosts: [{
    accountId: "sa_tiktok_123",
    mediaIds: [current.outputMediaId],
  }],
});`,
  },
  {
    label: "Python SDK",
    lang: "python",
    code: `from pathlib import Path
from time import sleep

import requests
from unipost import UniPost

client = UniPost()

def upload_and_wait(path: str, content_type: str) -> str:
    file_path = Path(path)

    # POST /v1/media reserves the upload. SDK 0.5.0+ does not require size_bytes.
    reservation = client.media.upload(
        filename=file_path.name,
        content_type=content_type,
    )

    # Upload raw bytes to the presigned upload_url returned by UniPost.
    requests.put(
        reservation.upload_url,
        data=file_path.read_bytes(),
        headers={"Content-Type": content_type},
    )

    # Poll GET /v1/media/{media_id} until the input file is usable.
    media = client.media.get(reservation.media_id)
    while media.status == "pending":
        sleep(1)
        media = client.media.get(reservation.media_id)

    if media.status not in ("uploaded", "attached"):
        raise RuntimeError(f"Upload failed with media status {media.status}")

    return reservation.media_id

# Step 1: Upload the video input.
video_media_id = upload_and_wait("./launch-clip.mp4", "video/mp4")

# Step 2: Upload the audio input.
audio_media_id = upload_and_wait("./voiceover.mp3", "audio/mpeg")

# Step 3: Generate the overlay video.
job = client.media.audio_overlays.create(
    video_media_id=video_media_id,
    audio_media_id=audio_media_id,
    mode="mix",
    video_volume=35,
    audio_volume=100,
    fit="loop_to_video",
    idempotency_key="overlay-user-42-launch-clip",
)

# Poll the processing job until it finishes.
while job.status in ("queued", "processing"):
    sleep(1.5)
    job = client.media.audio_overlays.get(job.id)

if job.status != "succeeded" or not job.output_media_id:
    raise RuntimeError(job.error.message if job.error else "Audio overlay failed")

# Step 4: Publish the processed video.
client.posts.create(
    caption="Video with custom audio",
    platform_posts=[
        {
            "account_id": "sa_tiktok_123",
            "media_ids": [job.output_media_id],
        }
    ],
)`,
  },
  {
    label: "Go SDK",
    lang: "go",
    code: `package main

import (
  "bytes"
  "context"
  "fmt"
  "io"
  "net/http"
  "os"
  "time"

  "github.com/unipost-dev/sdk-go/unipost"
)

func uploadAndWait(ctx context.Context, client *unipost.Client, path string, contentType string) (string, error) {
  data, err := os.ReadFile(path)
  if err != nil {
    return "", err
  }

  // POST /v1/media reserves the upload. SDK 0.5.0+ does not require SizeBytes.
  reserved, err := client.Media.Upload(ctx, &unipost.MediaUploadRequest{
    Filename:    path,
    ContentType: contentType,
  })
  if err != nil {
    return "", err
  }

  // Upload raw bytes to the presigned upload_url returned by UniPost.
  req, err := http.NewRequestWithContext(ctx, http.MethodPut, reserved.UploadURL, bytes.NewReader(data))
  if err != nil {
    return "", err
  }
  req.Header.Set("Content-Type", contentType)

  resp, err := http.DefaultClient.Do(req)
  if err != nil {
    return "", err
  }
  defer resp.Body.Close()
  _, _ = io.Copy(io.Discard, resp.Body)
  if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    return "", fmt.Errorf("upload failed with status %s", resp.Status)
  }

  // Poll GET /v1/media/{media_id} until the input file is usable.
  media, err := client.Media.Get(ctx, reserved.MediaID)
  if err != nil {
    return "", err
  }
  for media.Status == "pending" {
    time.Sleep(time.Second)
    media, err = client.Media.Get(ctx, reserved.MediaID)
    if err != nil {
      return "", err
    }
  }
  if media.Status != "uploaded" && media.Status != "attached" {
    return "", fmt.Errorf("upload failed with media status %s", media.Status)
  }

  return reserved.MediaID, nil
}

func main() {
  ctx := context.Background()
  client := unipost.NewClient()

  // Step 1: Upload the video input.
  videoMediaID, err := uploadAndWait(ctx, client, "launch-clip.mp4", "video/mp4")
  if err != nil {
    panic(err)
  }

  // Step 2: Upload the audio input.
  audioMediaID, err := uploadAndWait(ctx, client, "voiceover.mp3", "audio/mpeg")
  if err != nil {
    panic(err)
  }

  videoVolume := int32(35)
  audioVolume := int32(100)

  // Step 3: Generate the overlay video.
  job, err := client.Media.AudioOverlays.Create(ctx, &unipost.AudioOverlayCreateRequest{
    VideoMediaID: videoMediaID,
    AudioMediaID: audioMediaID,
    Mode:         "mix",
    VideoVolume:  &videoVolume,
    AudioVolume:  &audioVolume,
    Fit:          "loop_to_video",
  }, unipost.WithIdempotencyKey("overlay-user-42-launch-clip"))
  if err != nil {
    panic(err)
  }

  // Poll the processing job until it finishes.
  for job.Status == "queued" || job.Status == "processing" {
    time.Sleep(1500 * time.Millisecond)
    job, err = client.Media.AudioOverlays.Get(ctx, job.ID)
    if err != nil {
      panic(err)
    }
  }
  if job.Status != "succeeded" || job.OutputMediaID == nil {
    panic("audio overlay failed")
  }

  // Step 4: Publish the processed video.
  _, err = client.Posts.Create(ctx, &unipost.CreatePostParams{
    Caption: "Video with custom audio",
    PlatformPosts: []unipost.CreatePostPlatform{{
      AccountID: "sa_tiktok_123",
      MediaIDs:  []string{*job.OutputMediaID},
    }},
  })
  if err != nil {
    panic(err)
  }
}`,
  },
  {
    label: "Java SDK",
    lang: "java",
    code: `import dev.unipost.UniPost;
import com.fasterxml.jackson.databind.JsonNode;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpRequest.BodyPublishers;
import java.net.http.HttpResponse.BodyHandlers;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;

UniPost client = new UniPost();
HttpClient http = HttpClient.newHttpClient();

String uploadAndWait(String path, String contentType) throws Exception {
    Path file = Path.of(path);

    // POST /v1/media reserves the upload. SDK 0.5.0+ does not require size_bytes.
    JsonNode reservation = client.media().upload(Map.of(
        "filename", file.getFileName().toString(),
        "content_type", contentType
    ));

    String mediaId = reservation.path("media_id").asText();
    if (mediaId.isEmpty()) {
        mediaId = reservation.path("id").asText();
    }

    // Upload raw bytes to the presigned upload_url returned by UniPost.
    http.send(
        HttpRequest.newBuilder(URI.create(reservation.path("upload_url").asText()))
            .header("Content-Type", contentType)
            .PUT(BodyPublishers.ofFile(file))
            .build(),
        BodyHandlers.discarding()
    );

    // Poll GET /v1/media/{media_id} until the input file is usable.
    JsonNode media = client.media().get(mediaId);
    while (media.path("status").asText().equals("pending")) {
        Thread.sleep(1000);
        media = client.media().get(mediaId);
    }
    if (!List.of("uploaded", "attached").contains(media.path("status").asText())) {
        throw new IllegalStateException("Upload failed with media status " + media.path("status").asText());
    }

    return mediaId;
}

// Step 1: Upload the video input.
String videoMediaId = uploadAndWait("launch-clip.mp4", "video/mp4");

// Step 2: Upload the audio input.
String audioMediaId = uploadAndWait("voiceover.mp3", "audio/mpeg");

// Step 3: Generate the overlay video.
JsonNode job = client.media().audioOverlays().create(Map.of(
    "video_media_id", videoMediaId,
    "audio_media_id", audioMediaId,
    "mode", "mix",
    "video_volume", 35,
    "audio_volume", 100,
    "fit", "loop_to_video"
), "overlay-user-42-launch-clip");

// Poll the processing job until it finishes.
while (job.path("status").asText().equals("queued") ||
       job.path("status").asText().equals("processing")) {
    Thread.sleep(1500);
    job = client.media().audioOverlays().get(job.path("id").asText());
}
if (!job.path("status").asText().equals("succeeded")) {
    throw new IllegalStateException("Audio overlay failed");
}

// Step 4: Publish the processed video.
client.posts().create(Map.of(
    "caption", "Video with custom audio",
    "platform_posts", List.of(Map.of(
        "account_id", "sa_tiktok_123",
        "media_ids", List.of(job.path("output_media_id").asText())
    ))
));`,
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
      <div className="docs-callout docs-callout-tip">
        <strong>SDK prerequisite:</strong> use UniPost SDK version <code>0.5.0</code> or later for this workflow.
        Starting in <code>0.5.0</code>, the official SDKs no longer force callers to provide <code>sizeBytes</code> or{" "}
        <code>size_bytes</code> when reserving media uploads. Older SDK versions may still make your app calculate file size
        before calling <ApiInlineLink endpoint="POST /v1/media" />.
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
            "Keep the UI in an upload state until the video is usable. File size is optional: provide it when your app already knows the byte length, or omit it when your app does not know it yet.",
            <>
              Reserve with <ApiInlineLink endpoint="POST /v1/media" />, upload bytes to the returned <code>upload_url</code>,
              then poll <ApiInlineLink endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" /> until the media is
              uploaded. The returned upload_url is not another UniPost JSON endpoint; it is the presigned file upload destination.
            </>,
          ],
          [
            "Upload the audio",
            "Treat audio as an input file, not a publishable asset. Label it as the audio track for the generated video.",
            <>
              Use the same <ApiInlineLink endpoint="POST /v1/media" /> upload flow with an audio MIME type such as{" "}
              <code>audio/mpeg</code> or <code>audio/wav</code>, then poll{" "}
              <ApiInlineLink endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" /> until the audio media is uploaded.
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
      <h3 id="step-1-upload-video">Step 1: Upload the video input</h3>
      <p>
        Reserve a video upload with <ApiInlineLink endpoint="POST /v1/media" />. File size is optional: provide it when your app
        already knows the byte length, or omit it when your app does not know it yet. UniPost hydrates <code>size_bytes</code>{" "}
        after the upload lands.
      </p>
      <p>
        Upload the raw video bytes to the returned <code>upload_url</code>, then poll{" "}
        <ApiInlineLink endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" /> until the video is uploaded. The returned
        upload_url is not another UniPost JSON endpoint; it is the presigned destination for the actual file bytes.
      </p>

      <h3 id="step-2-upload-audio">Step 2: Upload the audio input</h3>
      <p>
        Reserve and upload the audio file with the same media upload flow and an audio MIME type such as <code>audio/mpeg</code>{" "}
        or <code>audio/wav</code>. Keep the returned audio <code>media_id</code> for processing only; do not publish the raw audio
        media as the post asset.
      </p>

      <h3 id="step-3-generate-overlay">Step 3: Generate the overlay video</h3>
      <p>
        Create the audio overlay job with <ApiInlineLink endpoint="POST /v1/media/audio-overlays" />. Use <code>mode: "mix"</code>{" "}
        to keep some original video sound, or <code>mode: "replace"</code> to remove the original sound. Use an{" "}
        <code>Idempotency-Key</code> tied to the user's generate action so browser retries do not create duplicate jobs.
      </p>
      <p>
        Poll <code>GET /v1/media/audio-overlays/&#123;id&#125;</code>. When <code>status</code> is <code>succeeded</code>, read{" "}
        <code>output_media_id</code>. That media ID points to the processed video that already contains the requested audio.
      </p>

      <h3 id="step-4-publish-post">Step 4: Publish the processed video</h3>
      <p>
        Publish the final video by passing <code>output_media_id</code> in <code>media_ids</code> on{" "}
        <ApiInlineLink endpoint="POST /v1/posts" />. From the publish API's perspective, this is a regular video media asset.
      </p>

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
