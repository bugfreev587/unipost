export type TimeMetricPost = {
  created_at: string;
  scheduled_at?: string | null;
};

export type TimeMetricResult = {
  published_at?: string | null;
};

export type TimeMetricJob = {
  kind: "dispatch" | "retry" | string;
  attempts: number;
  created_at?: string | null;
  first_claimed_at?: string | null;
  platform_started_at?: string | null;
  finished_at?: string | null;
};

export type TimeMetricPhase = {
  key: "created" | "scheduled" | "queued" | "claimed" | "platform_started" | "finished" | "published";
  label: string;
  at: string | null;
  durationFromPreviousMs: number | null;
};

function timestampMs(value?: string | null): number | null {
  if (!value) return null;
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function validTimestamp(value?: string | null): string | null {
  return timestampMs(value) === null ? null : value || null;
}

function selectJobTimestamp(
  jobs: TimeMetricJob[],
  key: "created_at" | "first_claimed_at" | "platform_started_at" | "finished_at",
  mode: "earliest" | "latest",
): string | null {
  let selected: { at: string; ms: number } | null = null;
  for (const job of jobs) {
    const at = job[key];
    const ms = timestampMs(at);
    if (!at || ms === null) continue;
    if (!selected || (mode === "earliest" ? ms < selected.ms : ms > selected.ms)) {
      selected = { at, ms };
    }
  }
  return selected?.at ?? null;
}

export function getPlatformPostTotalDurationMs(post: TimeMetricPost, result: TimeMetricResult): number | null {
  const baseline = timestampMs(post.scheduled_at || post.created_at);
  const published = timestampMs(result.published_at);
  if (baseline === null || published === null || published < baseline) return null;
  return published - baseline;
}

export function getRetryCount(jobs: TimeMetricJob[]): number {
  return jobs.reduce((total, job) => {
    if (job.kind !== "retry" || !Number.isFinite(job.attempts) || job.attempts <= 0) return total;
    return total + Math.floor(job.attempts);
  }, 0);
}

export function buildTimeMetricPhases(
  post: TimeMetricPost,
  result: TimeMetricResult,
  jobs: TimeMetricJob[],
): TimeMetricPhase[] {
  const phaseInputs: Array<Omit<TimeMetricPhase, "durationFromPreviousMs">> = [
    { key: "created", label: "Post created", at: validTimestamp(post.created_at) },
  ];
  if (post.scheduled_at) {
    phaseInputs.push({ key: "scheduled", label: "Scheduled", at: validTimestamp(post.scheduled_at) });
  }
  phaseInputs.push(
    { key: "queued", label: "Job queued", at: selectJobTimestamp(jobs, "created_at", "earliest") },
    { key: "claimed", label: "First claimed", at: selectJobTimestamp(jobs, "first_claimed_at", "earliest") },
    { key: "platform_started", label: "Platform started", at: selectJobTimestamp(jobs, "platform_started_at", "earliest") },
    { key: "finished", label: "Job finished", at: selectJobTimestamp(jobs, "finished_at", "latest") },
    { key: "published", label: "Published", at: validTimestamp(result.published_at) },
  );

  let previousMs: number | null = null;
  return phaseInputs.map((phase) => {
    const currentMs = timestampMs(phase.at);
    let durationFromPreviousMs: number | null = null;
    if (currentMs !== null) {
      if (previousMs !== null && currentMs >= previousMs) {
        durationFromPreviousMs = currentMs - previousMs;
      }
      previousMs = currentMs;
    }
    return { ...phase, durationFromPreviousMs };
  });
}

export function formatTimeMetricDuration(ms: number | null): string {
  if (ms === null || !Number.isFinite(ms) || ms < 0) return "—";
  if (ms < 60_000) {
    const seconds = Math.round(ms / 100) / 10;
    return `${seconds}s`;
  }
  const totalSeconds = Math.round(ms / 1_000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}m ${seconds}s`;
}

export function formatTimeMetricTimestamp(iso: string | null): string {
  const ms = timestampMs(iso);
  if (ms === null) return "Not recorded";
  return new Date(ms).toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
    second: "2-digit",
    fractionalSecondDigits: 3,
  });
}
