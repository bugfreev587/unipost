import { readFile, stat } from "node:fs/promises";
import { completeReviewJob, createReviewArtifactUpload, failReviewJob, postReviewEvent, putReviewArtifact } from "./client.js";

export function createAgentReporter({ token, apiUrl, fetchImpl = globalThis.fetch, now = Date.now, readFileImpl = readFile, statImpl = stat } = {}) {
  const startedAt = now();
  const elapsed = () => Math.max(0, now() - startedAt);
  return {
    event(eventType, message, metadata = {}) {
      return postReviewEvent({
        token,
        apiUrl,
        fetchImpl,
        event: {
          event_type: eventType,
          message,
          metadata,
          elapsed_ms: elapsed(),
        },
      });
    },
    async uploadArtifact({ artifactType, contentType, path }) {
      const info = await statImpl(path);
      const upload = await createReviewArtifactUpload({
        token,
        apiUrl,
        fetchImpl,
        artifact: {
          artifact_type: artifactType,
          content_type: contentType,
          size_bytes: info.size,
        },
      });
      const bytes = await readFileImpl(path);
      await putReviewArtifact({ upload, bytes, fetchImpl });
      return upload.file_id;
    },
    complete(artifacts = {}, videoFileId = "") {
      return completeReviewJob({
        token,
        apiUrl,
        fetchImpl,
        data: {
          video_file_id: videoFileId,
          artifacts: { ...artifacts, elapsed_ms: elapsed() },
        },
      });
    },
    fail(error, artifacts = {}) {
      const message = error instanceof Error ? error.message : String(error || "review recording failed");
      return failReviewJob({
        token,
        apiUrl,
        fetchImpl,
        data: {
          failure_reason: message,
          artifacts: { ...artifacts, elapsed_ms: elapsed() },
        },
      });
    },
  };
}
