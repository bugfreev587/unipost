import { completeReviewJob, failReviewJob, postReviewEvent } from "./client.js";

export function createAgentReporter({ token, apiUrl, fetchImpl = globalThis.fetch, now = Date.now } = {}) {
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
