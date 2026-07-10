export type QueueDiagnosticsKind = "loading" | "unavailable" | "not_queued" | "ready";

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
): { kind: QueueDiagnosticsKind; label: string } {
  if (loading && jobs.length === 0) {
    return { kind: "loading", label: "Queue diagnostics" };
  }
  if (error && jobs.length === 0) {
    return { kind: "unavailable", label: "Queue diagnostics · Unavailable" };
  }
  if (jobs.length === 0) {
    return { kind: "not_queued", label: "Queue diagnostics · Not queued yet" };
  }
  return { kind: "ready", label: `Queue diagnostics (${jobs.length})` };
}
