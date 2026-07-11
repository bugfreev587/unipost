export type QueueDiagnosticsKind = "loading" | "unavailable" | "not_queued" | "no_history" | "ready";

const NOT_QUEUED_STATUSES = new Set(["pending", "scheduled"]);

export function getJobsForResult<T extends { social_post_result_id: string }>(
  jobs: T[],
  resultId?: string,
): T[] {
  if (!resultId) return [];
  return jobs.filter((job) => job.social_post_result_id === resultId);
}

export function getQueueDiagnosticsState(
  jobs: Array<{ id: string }>,
  loading: boolean,
  error: string | null,
  resultStatus: string,
): { kind: QueueDiagnosticsKind; label: string } {
  if (error) {
    return { kind: "unavailable", label: "Queue diagnostics · Unavailable" };
  }
  if (loading && jobs.length === 0) {
    return { kind: "loading", label: "Queue diagnostics" };
  }
  if (jobs.length === 0) {
    return NOT_QUEUED_STATUSES.has(resultStatus)
      ? { kind: "not_queued", label: "Queue diagnostics · Not queued yet" }
      : { kind: "no_history", label: "Queue diagnostics · No history" };
  }
  return { kind: "ready", label: `Queue diagnostics (${jobs.length})` };
}
