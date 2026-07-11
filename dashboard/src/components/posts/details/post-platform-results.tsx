"use client";

import { useAuth } from "@clerk/nextjs";
import { ChevronDown, ChevronRight, Copy, ExternalLink, RotateCcw } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";

import { AccountDestinationIcon } from "@/components/account-destination-icon";
import {
  getSocialPostQueue,
  retrySocialPostResult,
  type PostDeliveryJob,
  type SocialPost,
} from "@/lib/api";
import { describePostResultFailure } from "@/lib/post-result-errors";

import { getJobsForResult, getQueueDiagnosticsState } from "./post-platform-results-model";
import { TimeMetricsPanel } from "./time-metrics-panel";

type PostPlatformResultsProps = {
  post: SocialPost;
  workspaceId: string;
  layout: "grid" | "stack";
  onRetryComplete?: () => void | Promise<void>;
};

export function PostPlatformResults({
  post,
  workspaceId,
  layout,
  onRetryComplete,
}: PostPlatformResultsProps) {
  const { getToken } = useAuth();
  const [jobs, setJobs] = useState<PostDeliveryJob[]>([]);
  const [jobsLoading, setJobsLoading] = useState(false);
  const [jobsError, setJobsError] = useState<string | null>(null);
  const queueRequestRef = useRef(0);
  const results = post.results || [];
  const shouldLoadQueue = results.length > 0;
  const resultQueueSignature = results
    .map((result) => `${result.id || result.social_account_id}:${result.status}:${result.published_at || ""}`)
    .join("|");
  const queueRefreshSignature = [
    post.status,
    post.queued_results_count,
    post.retrying_count,
    post.dead_count,
    resultQueueSignature,
  ].join("|");

  const loadQueue = useCallback(async (refreshSignature?: string) => {
    // The effect passes this key so queue-status changes deliberately
    // trigger a fresh request without coupling the request body to UI state.
    void refreshSignature;
    const requestId = ++queueRequestRef.current;
    if (!shouldLoadQueue) {
      setJobs([]);
      setJobsError(null);
      setJobsLoading(false);
      return;
    }
    setJobsLoading(true);
    setJobsError(null);
    try {
      const token = await getToken();
      if (!token || requestId !== queueRequestRef.current) return;
      const response = await getSocialPostQueue(token, post.id);
      if (requestId !== queueRequestRef.current) return;
      setJobs(response.data.jobs || []);
      setJobsError(null);
    } catch (error) {
      if (requestId !== queueRequestRef.current) return;
      setJobsError(error instanceof Error ? error.message : "Failed to load queue details");
    } finally {
      if (requestId === queueRequestRef.current) setJobsLoading(false);
    }
  }, [
    getToken,
    post.id,
    shouldLoadQueue,
  ]);

  useEffect(() => {
    void loadQueue(queueRefreshSignature);
    return () => {
      queueRequestRef.current += 1;
    };
  }, [loadQueue, queueRefreshSignature]);

  return (
    <>
      {results.length === 0 ? (
        <div className="posts-result-text">No platform results yet.</div>
      ) : (
        <div className={`posts-results-grid${layout === "stack" ? " is-stack" : ""}`}>
          {results.map((result, index) => (
            <PostResultCard
              key={result.id || result.social_account_id || `${result.platform || "platform"}-${index}`}
              post={post}
              result={result}
              workspaceId={workspaceId}
              jobs={getJobsForResult(jobs, result.id)}
              jobsLoading={jobsLoading}
              jobsError={jobsError}
              onRetryComplete={async () => {
                await loadQueue();
                await onRetryComplete?.();
              }}
            />
          ))}
        </div>
      )}
      <style jsx global>{PLATFORM_RESULTS_CSS}</style>
    </>
  );
}

function PostResultCard({
  post,
  result,
  workspaceId,
  jobs,
  jobsLoading,
  jobsError,
  onRetryComplete,
}: {
  post: SocialPost;
  result: NonNullable<SocialPost["results"]>[number];
  workspaceId: string;
  jobs: PostDeliveryJob[];
  jobsLoading: boolean;
  jobsError: string | null;
  onRetryComplete?: () => void | Promise<void>;
}) {
  const { getToken } = useAuth();
  const [retrying, setRetrying] = useState(false);
  const [retryError, setRetryError] = useState<string | null>(null);

  const handleRetry = useCallback(async () => {
    if (!result.id || retrying) return;
    setRetrying(true);
    setRetryError(null);
    try {
      const token = await getToken();
      if (!token) return;
      await retrySocialPostResult(token, post.id, result.id);
      await onRetryComplete?.();
    } catch (error) {
      setRetryError(error instanceof Error ? error.message : "Retry failed");
    } finally {
      setRetrying(false);
    }
  }, [getToken, onRetryComplete, post.id, result.id, retrying]);

  const url = result.url
    ? result.url
    : result.external_id && result.platform
      ? postUrlFor(result.platform, result.external_id)
      : null;
  const failure = result.status === "failed" ? describePostResultFailure(result) : null;

  return (
    <div className="posts-result-card">
      <div className="posts-result-head">
        <div className="posts-result-title">
          <AccountDestinationIcon platform={result.platform || ""} size={15} />
          <span className="posts-result-name">{result.account_name || result.platform || "Unknown"}</span>
          <InlineStatusPill status={result.status} />
        </div>
        {url ? (
          <a
            href={url}
            target="_blank"
            rel="noreferrer"
            onClick={(event) => event.stopPropagation()}
            className="posts-result-link"
            title="Open original post"
          >
            <ExternalLink aria-hidden="true" />
            <span className="sr-only">Open original post</span>
          </a>
        ) : null}
      </div>

      <div className="posts-result-meta">
        {result.account_name ? <div className="posts-result-text">{result.platform || "Unknown"}</div> : null}
        <div className="posts-result-text">
          {result.published_at
            ? formatLongDate(result.published_at)
            : post.published_at
              ? formatLongDate(post.published_at)
              : "Not published yet"}
        </div>
      </div>

      {result.status === "failed" ? (
        <>
          {failure ? <div className="posts-error-title">{failure.title}</div> : null}
          <div className="posts-error">
            {failure?.message || result.error_message || "Publish failed (no error message reported)."}
          </div>
          {failure?.nextActionLabel ? (
            <div className="posts-hint posts-result-action-hint">
              <span className="posts-hint-label">Next: </span>
              {failure.actionHref ? (
                failure.actionHref.startsWith("http") ? (
                  <a href={failure.actionHref} target="_blank" rel="noreferrer" className="posts-result-link">
                    {failure.nextActionLabel}
                  </a>
                ) : (
                  <Link href={failure.actionHref.replace(":id", workspaceId)} className="posts-result-link">
                    {failure.nextActionLabel}
                  </Link>
                )
              ) : (
                failure.nextActionLabel
              )}
            </div>
          ) : null}
          {failure?.retryStatusLabel ? (
            <div className="posts-hint posts-result-retry-hint">
              <span className="posts-hint-label">Retry: </span>
              {failure.retryStatusLabel}
            </div>
          ) : null}
          {result.id && failure?.canRetry ? (
            <div className="posts-retry-row">
              <button type="button" onClick={handleRetry} disabled={retrying} className="posts-retry-btn">
                <RotateCcw className={retrying ? "is-spinning" : undefined} aria-hidden="true" />
                {retrying ? "Retrying…" : "Retry"}
              </button>
              {retryError ? <span className="posts-retry-error" role="alert">{retryError}</span> : null}
            </div>
          ) : null}
          {result.debug_curl ? <DebugCurlPanel curl={result.debug_curl} /> : null}
          <QueueDiagnostics jobs={jobs} loading={jobsLoading} error={jobsError} resultStatus={result.status} />
          <TimeMetricsPanel post={post} result={result} jobs={jobs} loading={jobsLoading} error={jobsError} />
          {result.submitted ? <SubmittedSettingsPanel platform={result.platform || ""} submitted={result.submitted} /> : null}
        </>
      ) : (
        <>
          <div className="posts-hint">
            {result.status === "published"
              ? "Published successfully."
              : result.status === "partial"
                ? "Partially completed. Review other platform cards for failures."
                : `Status: ${result.status}`}
            {result.external_id ? <div className="posts-result-text posts-result-id">ID: {result.external_id}</div> : null}
          </div>
          {result.status === "processing" && result.platform === "facebook" ? (
            <FacebookProcessingPanel publishStatus={result.publish_status} />
          ) : null}
          <QueueDiagnostics jobs={jobs} loading={jobsLoading} error={jobsError} resultStatus={result.status} />
          <TimeMetricsPanel post={post} result={result} jobs={jobs} loading={jobsLoading} error={jobsError} />
          {result.submitted ? <SubmittedSettingsPanel platform={result.platform || ""} submitted={result.submitted} /> : null}
        </>
      )}
    </div>
  );
}

function FacebookProcessingPanel({ publishStatus }: { publishStatus?: Record<string, unknown> }) {
  const phases = [
    { label: "Uploading", status: (publishStatus?.uploading_phase_status as string) || "—" },
    { label: "Processing", status: (publishStatus?.processing_phase_status as string) || "—" },
    { label: "Publishing", status: (publishStatus?.publishing_phase_status as string) || "—" },
  ];
  return (
    <div className="posts-hint posts-fb-processing">
      <div>Facebook is still processing this video. The post will appear on the Page once all phases complete.</div>
      <div className="posts-fb-phases">
        {phases.map((phase) => (
          <div key={phase.label} className="posts-fb-phase">
            <span className="posts-fb-phase-label">{phase.label}</span>
            <span className={`posts-fb-phase-status ${statusToClass(phase.status)}`}>{phase.status}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function statusToClass(status: string): string {
  if (status === "complete") return "posts-fb-phase-complete";
  if (status === "in_progress") return "posts-fb-phase-progress";
  if (status === "error") return "posts-fb-phase-error";
  return "";
}

function DebugCurlPanel({ curl }: { curl: string }) {
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(curl);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      // The request remains selectable if clipboard permission is blocked.
    }
  }, [curl]);

  return (
    <div className="posts-debug-panel posts-panel-spacing">
      <PanelToggle open={open} onToggle={() => setOpen((value) => !value)}>
        Debug request ({curl.split("\n# Request ").length - 1 || 1} HTTP call{curl.includes("\n# Request 2") ? "s" : ""})
      </PanelToggle>
      {open ? (
        <div className="posts-debug-body">
          <div className="posts-debug-actions">
            <button type="button" onClick={handleCopy} className="posts-debug-copy">
              <Copy aria-hidden="true" />
              {copied ? "Copied" : "Copy"}
            </button>
          </div>
          <pre className="posts-debug-pre">{curl}</pre>
        </div>
      ) : null}
    </div>
  );
}

function QueueDiagnostics({
  jobs,
  loading,
  error,
  resultStatus,
}: {
  jobs: PostDeliveryJob[];
  loading: boolean;
  error: string | null;
  resultStatus: string;
}) {
  const [open, setOpen] = useState(false);
  const state = getQueueDiagnosticsState(jobs, loading, error, resultStatus);
  const sorted = [...jobs].sort((a, b) => (Date.parse(b.updated_at) || 0) - (Date.parse(a.updated_at) || 0));
  const active = sorted.find((job) => ["pending", "running", "retrying"].includes(job.state));
  const latest = active || sorted[0];
  const timeline = sorted.slice(0, 3);

  return (
    <div className="posts-queue-panel">
      <PanelToggle open={open} onToggle={() => setOpen((value) => !value)}>{state.label}</PanelToggle>
      {open ? (
        <div className="posts-queue-body">
          {state.kind === "loading" ? <div className="posts-queue-loading">Loading queue details…</div> : null}
          {state.kind === "unavailable" ? <div className="posts-queue-empty">{error}</div> : null}
          {state.kind === "not_queued" ? <div className="posts-queue-empty">No delivery job has been queued for this result.</div> : null}
          {state.kind === "no_history" ? <div className="posts-queue-empty">No delivery job history is available for this result.</div> : null}
          {state.kind === "ready" && latest ? (
            <>
              <dl className="posts-queue-grid">
                <dt>Current state</dt><dd>{latest.state}</dd>
                <dt>Delivery phase</dt><dd>{humanizeCode(latest.delivery_phase || latest.state)}</dd>
                <dt>Queue lane</dt><dd>{latest.kind === "retry" ? "Retry queue" : "Initial dispatch"}</dd>
                <dt>Attempts</dt><dd>{latest.attempts}/{latest.max_attempts}</dd>
                <dt>Queued at</dt><dd>{formatLongDate(latest.queued_at || latest.created_at)}</dd>
                <dt>Last update</dt><dd>{formatLongDate(latest.updated_at)}</dd>
                {latest.next_run_at ? <><dt>Next retry</dt><dd>{formatLongDate(latest.next_run_at)}</dd></> : null}
                {latest.last_attempt_at ? <><dt>Last attempt</dt><dd>{formatLongDate(latest.last_attempt_at)}</dd></> : null}
                {latest.first_claimed_at ? <><dt>First claimed</dt><dd>{formatLongDate(latest.first_claimed_at)}</dd></> : null}
                {latest.platform_started_at ? <><dt>Platform started</dt><dd>{formatLongDate(latest.platform_started_at)}</dd></> : null}
                {latest.finished_at ? <><dt>Finished</dt><dd>{formatLongDate(latest.finished_at)}</dd></> : null}
                {typeof latest.queue_wait_ms === "number" ? <><dt>Queue wait</dt><dd>{formatDurationMs(latest.queue_wait_ms)}</dd></> : null}
                {typeof latest.worker_wait_ms === "number" ? <><dt>Worker wait</dt><dd>{formatDurationMs(latest.worker_wait_ms)}</dd></> : null}
                {typeof latest.platform_duration_ms === "number" ? <><dt>Platform duration</dt><dd>{formatDurationMs(latest.platform_duration_ms)}</dd></> : null}
                {latest.failure_stage ? <><dt>Failure stage</dt><dd>{humanizeCode(latest.failure_stage)}</dd></> : null}
                {latest.error_code ? <><dt>Internal code</dt><dd>{latest.error_code}</dd></> : null}
                {latest.platform_error_code ? <><dt>Platform code</dt><dd>{latest.platform_error_code}</dd></> : null}
                {latest.last_error ? <><dt>Worker note</dt><dd>{latest.last_error}</dd></> : null}
              </dl>
              {timeline.length > 1 ? (
                <div className="posts-queue-timeline">
                  {timeline.map((job) => (
                    <div key={job.id} className="posts-queue-event">
                      <div className="posts-queue-event-top">
                        <InlineStatusPill status={job.delivery_phase || job.state} />
                        <span className="posts-queue-event-meta">{job.kind === "retry" ? "retry" : "dispatch"} · {formatLongDate(job.updated_at)}</span>
                      </div>
                      {job.last_error ? <div className="posts-queue-event-error">{job.last_error}</div> : null}
                    </div>
                  ))}
                </div>
              ) : null}
            </>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function PanelToggle({ open, onToggle, children }: { open: boolean; onToggle: () => void; children: React.ReactNode }) {
  return (
    <button type="button" onClick={onToggle} className="posts-debug-toggle" aria-expanded={open}>
      {open ? <ChevronDown aria-hidden="true" /> : <ChevronRight aria-hidden="true" />}
      <span>{children}</span>
    </button>
  );
}

function SubmittedSettingsPanel({
  platform,
  submitted,
}: {
  platform: string;
  submitted: NonNullable<NonNullable<SocialPost["results"]>[number]["submitted"]>;
}) {
  const [open, setOpen] = useState(false);
  const rows = buildSubmittedRows(platform, submitted);
  if (rows.length === 0) return null;
  return (
    <div className="posts-submitted-panel posts-panel-spacing">
      <PanelToggle open={open} onToggle={() => setOpen((value) => !value)}>Submitted settings ({rows.length})</PanelToggle>
      {open ? (
        <div className="posts-submitted-body">
          <dl className="posts-submitted-list">
            {rows.map((row) => <div key={row.label} className="posts-submitted-row"><dt>{row.label}</dt><dd>{row.value}</dd></div>)}
          </dl>
        </div>
      ) : null}
    </div>
  );
}

function buildSubmittedRows(
  platform: string,
  submitted: NonNullable<NonNullable<SocialPost["results"]>[number]["submitted"]>,
): Array<{ label: string; value: React.ReactNode }> {
  const rows: Array<{ label: string; value: React.ReactNode }> = [];
  if (submitted.caption) rows.push({ label: "Caption override", value: submitted.caption });
  const mediaCount = (submitted.media_urls?.length || 0) + (submitted.media_ids?.length || 0);
  if (mediaCount > 0) rows.push({ label: "Media attached", value: `${mediaCount} file${mediaCount === 1 ? "" : "s"}` });
  if (submitted.first_comment) rows.push({ label: "First comment", value: submitted.first_comment });
  if (typeof submitted.thread_position === "number" && submitted.thread_position > 0) rows.push({ label: "Thread position", value: String(submitted.thread_position) });
  const options = submitted.platform_options;
  if (!options) return rows;
  if (platform === "tiktok") pushTikTokRows(rows, options);
  else if (platform === "youtube") pushYouTubeRows(rows, options);
  else if (platform === "instagram") pushInstagramRows(rows, options);
  else if (platform === "linkedin") pushLinkedInRows(rows, options);
  else pushGenericRows(rows, options);
  return rows;
}

const TIKTOK_PRIVACY_LABELS: Record<string, string> = {
  PUBLIC_TO_EVERYONE: "Everyone",
  MUTUAL_FOLLOW_FRIENDS: "Friends",
  FOLLOWER_OF_CREATOR: "Followers",
  SELF_ONLY: "Only me",
};

type SubmittedRow = { label: string; value: React.ReactNode };

function pushTikTokRows(rows: SubmittedRow[], options: Record<string, unknown>) {
  if (typeof options.privacy_level === "string") rows.push({ label: "Who can view", value: TIKTOK_PRIVACY_LABELS[options.privacy_level] || options.privacy_level });
  const interactions: string[] = [];
  if (options.disable_comment === false) interactions.push("Comment");
  if (options.disable_duet === false) interactions.push("Duet");
  if (options.disable_stitch === false) interactions.push("Stitch");
  if (interactions.length > 0) rows.push({ label: "Allow interactions", value: interactions.join(", ") });
  else if (options.disable_comment === true || options.disable_duet === true || options.disable_stitch === true) rows.push({ label: "Allow interactions", value: "All disabled" });
  if (options.brand_organic_toggle === true || options.brand_content_toggle === true) {
    const labels: string[] = [];
    if (options.brand_organic_toggle === true) labels.push("Your Brand (Promotional content)");
    if (options.brand_content_toggle === true) labels.push("Branded Content (Paid partnership)");
    rows.push({ label: "Commercial disclosure", value: labels.join(" + ") });
  }
}

function pushYouTubeRows(rows: SubmittedRow[], options: Record<string, unknown>) {
  if (typeof options.title === "string" && options.title) rows.push({ label: "Video title", value: options.title });
  if (typeof options.privacy_status === "string") rows.push({ label: "Visibility", value: options.privacy_status });
  if (typeof options.category_id === "string") rows.push({ label: "Category", value: options.category_id });
  if (options.shorts === true) rows.push({ label: "Posted as", value: "Shorts" });
  if (typeof options.made_for_kids === "boolean") rows.push({ label: "Made for kids", value: options.made_for_kids ? "Yes" : "No" });
  if (Array.isArray(options.tags) && options.tags.length > 0) rows.push({ label: "Tags", value: options.tags.join(", ") });
  if (typeof options.publish_at === "string" && options.publish_at) rows.push({ label: "Scheduled for", value: options.publish_at });
  if (typeof options.playlist_id === "string" && options.playlist_id) rows.push({ label: "Playlist", value: options.playlist_id });
  if (options.contains_synthetic_media === true) rows.push({ label: "AI-generated content", value: "Yes" });
}

function pushInstagramRows(rows: SubmittedRow[], options: Record<string, unknown>) {
  const mediaType = typeof options.mediaType === "string" ? options.mediaType : typeof options.media_type === "string" ? options.media_type : null;
  if (mediaType) rows.push({ label: "Media type", value: mediaType });
}

function pushLinkedInRows(rows: SubmittedRow[], options: Record<string, unknown>) {
  if (typeof options.visibility === "string") rows.push({ label: "Visibility", value: options.visibility });
}

function pushGenericRows(rows: SubmittedRow[], options: Record<string, unknown>) {
  for (const [key, value] of Object.entries(options)) {
    if (value === null || value === undefined || value === "" || value === false) continue;
    rows.push({ label: key, value: typeof value === "object" ? JSON.stringify(value) : String(value) });
  }
}

function postUrlFor(platform: string, externalId: string): string | null {
  switch (platform) {
    case "youtube": return `https://www.youtube.com/watch?v=${externalId}`;
    case "twitter": return `https://x.com/i/status/${externalId}`;
    case "instagram": return `https://www.instagram.com/p/${externalId}/`;
    case "threads": return `https://www.threads.net/post/${externalId}`;
    case "linkedin": return externalId.startsWith("urn:li:") ? `https://www.linkedin.com/feed/update/${externalId}/` : null;
    case "bluesky": {
      const match = externalId.match(/^at:\/\/(did:[^/]+)\/app\.bsky\.feed\.post\/(.+)$/);
      return match ? `https://bsky.app/profile/${match[1]}/post/${match[2]}` : null;
    }
    default: return null;
  }
}

function formatLongDate(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleString("en-US", { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
}

function formatDurationMs(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return "—";
  if (ms < 1000) return `${Math.round(ms)} ms`;
  const seconds = ms / 1000;
  if (seconds < 60) return `${seconds.toFixed(seconds < 10 ? 1 : 0)} sec`;
  const minutes = Math.floor(seconds / 60);
  const remaining = Math.round(seconds % 60);
  if (minutes < 60) return remaining > 0 ? `${minutes}m ${remaining}s` : `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  return minutes % 60 > 0 ? `${hours}h ${minutes % 60}m` : `${hours}h`;
}

function humanizeCode(value: string): string {
  return value.split("_").filter(Boolean).map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(" ");
}

const STATUS_BADGES: Record<string, { className: string; label: string }> = {
  published: { className: "is-success", label: "published" },
  succeeded: { className: "is-success", label: "succeeded" },
  failed: { className: "is-danger", label: "failed" },
  dead: { className: "is-danger", label: "dead" },
  retrying: { className: "is-warning", label: "retrying" },
  waiting_retry: { className: "is-warning", label: "waiting retry" },
  partial: { className: "is-warning", label: "partial" },
  processing: { className: "is-info", label: "processing" },
  running: { className: "is-info", label: "running" },
  pending: { className: "is-neutral", label: "pending" },
};

function InlineStatusPill({ status }: { status: string }) {
  const badge = STATUS_BADGES[status] || { className: "is-neutral", label: humanizeCode(status) };
  return <span className={`posts-result-status ${badge.className}`}><span aria-hidden="true" />{badge.label}</span>;
}

const PLATFORM_RESULTS_CSS = `
.posts-results-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:16px}
.posts-results-grid.is-stack{grid-template-columns:minmax(0,1fr)}
.posts-result-card{min-width:0;background:var(--surface2);border:1px solid var(--dborder);border-radius:14px;padding:16px}
.posts-result-head{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-bottom:10px}
.posts-result-title{display:flex;align-items:center;gap:8px;min-width:0}
.posts-result-name{font-size:14px;font-weight:650;color:var(--dtext);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.posts-result-meta{display:flex;flex-direction:column;gap:4px;margin-bottom:12px}
.posts-result-text{font-size:13.5px;color:var(--dmuted);line-height:1.6;overflow-wrap:anywhere}
.posts-result-link{display:inline-flex;align-items:center;gap:4px;color:var(--daccent);text-decoration:none;font-size:13px;font-weight:600}
.posts-result-link>svg{width:14px;height:14px}
.posts-result-link:hover{text-decoration:underline}
.posts-result-status{display:inline-flex;align-items:center;gap:6px;padding:4px 9px;border-radius:999px;border:1px solid var(--dborder);font-family:var(--font-geist-mono),monospace;font-size:11px;line-height:1;text-transform:lowercase;white-space:nowrap}
.posts-result-status>span{width:6px;height:6px;border-radius:999px;background:currentColor}
.posts-result-status.is-success{color:var(--success);background:color-mix(in srgb,var(--success) 11%,var(--surface2));border-color:color-mix(in srgb,var(--success) 24%,var(--dborder))}
.posts-result-status.is-danger{color:var(--danger);background:var(--danger-soft);border-color:color-mix(in srgb,var(--danger) 24%,var(--dborder))}
.posts-result-status.is-warning{color:var(--warning,#f59e0b);background:color-mix(in srgb,var(--warning,#f59e0b) 10%,var(--surface2));border-color:color-mix(in srgb,var(--warning,#f59e0b) 24%,var(--dborder))}
.posts-result-status.is-info{color:var(--primary);background:color-mix(in srgb,var(--primary) 10%,var(--surface2));border-color:color-mix(in srgb,var(--primary) 24%,var(--dborder))}
.posts-result-status.is-neutral{color:var(--dmuted);background:var(--surface1)}
.posts-error-title{font-size:12px;font-weight:800;color:var(--danger);margin-bottom:6px}
.posts-error{font-size:12px;color:var(--danger);background:var(--danger-soft);border:1px solid color-mix(in srgb,var(--danger) 22%,transparent);border-radius:10px;padding:10px 12px;white-space:pre-wrap;word-break:break-word;font-family:var(--font-geist-mono),monospace;line-height:1.6;max-height:148px;overflow:auto}
.posts-hint{font-size:14px;color:var(--dtext);line-height:1.65}.posts-hint-label{color:var(--dmuted)}
.posts-result-action-hint{margin-top:10px}.posts-result-retry-hint{margin-top:8px}.posts-result-id{margin-top:10px}.posts-fb-processing{margin-top:10px}
.posts-panel-spacing{margin-top:10px}.posts-debug-panel,.posts-submitted-panel,.posts-time-metrics-panel,.posts-queue-panel{border:1px solid var(--dborder);border-radius:10px;background:var(--surface1);overflow:hidden}
.posts-queue-panel{margin-top:10px}
.posts-debug-toggle{display:flex;align-items:center;gap:6px;width:100%;padding:10px 12px;font-size:11.5px;font-weight:650;color:var(--dmuted);background:transparent;border:0;cursor:pointer;font-family:var(--font-geist-mono),monospace;text-transform:uppercase;letter-spacing:.08em;text-align:left}
.posts-debug-toggle>svg{width:12px;height:12px;flex:0 0 auto}.posts-debug-toggle:hover{color:var(--dtext)}
.posts-debug-body,.posts-submitted-body,.posts-time-metrics-body,.posts-queue-body{border-top:1px solid var(--dborder);padding:11px 12px}
.posts-debug-actions{display:flex;justify-content:flex-end;margin-bottom:6px}.posts-debug-copy{display:inline-flex;align-items:center;gap:4px;padding:4px 8px;font-size:11px;color:var(--dmuted);background:var(--surface2);border:1px solid var(--dborder);border-radius:7px;cursor:pointer;font-family:var(--font-geist-mono),monospace}.posts-debug-copy>svg{width:11px;height:11px}
.posts-debug-pre{font-size:11.5px;line-height:1.6;color:var(--dtext);background:var(--surface2);border:1px solid var(--dborder);border-radius:8px;padding:11px 12px;max-height:320px;overflow:auto;white-space:pre-wrap;word-break:break-all;font-family:var(--font-geist-mono),monospace}
.posts-submitted-list,.posts-queue-grid{display:grid;grid-template-columns:max-content minmax(0,1fr);gap:6px 14px;margin:0}.posts-submitted-row{display:contents}.posts-submitted-row dt,.posts-queue-grid dt{font-size:11.5px;color:var(--dmuted2);text-transform:uppercase;letter-spacing:.08em;font-weight:600}.posts-submitted-row dd,.posts-queue-grid dd{font-size:13px;color:var(--dtext);margin:0;word-break:break-word;white-space:pre-wrap;line-height:1.55}
.posts-time-metrics-toggle{justify-content:flex-start}.posts-time-metrics-total{margin-left:auto;padding:3px 7px;border-radius:999px;background:color-mix(in srgb,var(--daccent) 12%,var(--surface2));color:var(--daccent);font-size:11px;letter-spacing:0;text-transform:none}.posts-time-metrics-notice{margin-bottom:10px;padding:8px 10px;border:1px solid var(--dborder);border-radius:8px;background:var(--surface2);color:var(--dmuted);font-size:12px;line-height:1.5}
.posts-time-metrics-summary{display:grid;grid-template-columns:minmax(0,1.4fr) repeat(2,minmax(0,1fr));gap:10px;margin-bottom:14px}.posts-time-metrics-summary>div{border-left:2px solid color-mix(in srgb,var(--daccent) 34%,var(--dborder));padding-left:9px;min-width:0}.posts-time-metrics-summary span{display:block;margin-bottom:3px;color:var(--dmuted2);font-size:10px;font-weight:650;letter-spacing:.07em;text-transform:uppercase}.posts-time-metrics-summary strong{display:block;color:var(--dtext);font-family:var(--font-geist-mono),monospace;font-size:12px;font-weight:650;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.posts-time-metrics-timeline{position:relative;display:flex;flex-direction:column}.posts-time-metrics-event{position:relative;display:grid;grid-template-columns:10px minmax(0,1fr) max-content;gap:9px;align-items:center;min-height:38px}.posts-time-metrics-event:not(:last-child)::before{content:"";position:absolute;z-index:0;top:7px;bottom:-7px;left:4px;width:1px;background:var(--dborder)}.posts-time-metrics-dot{position:relative;z-index:1;align-self:start;width:7px;height:7px;margin-top:3px;border:2px solid var(--surface1);border-radius:999px;background:var(--dmuted2);box-shadow:0 0 0 1px var(--dmuted2)}.posts-time-metrics-event.is-final .posts-time-metrics-dot{background:var(--success);box-shadow:0 0 0 1px var(--success)}.posts-time-metrics-event-copy{display:flex;flex-direction:column;gap:2px;min-width:0}.posts-time-metrics-event-label{color:var(--dtext);font-size:11.5px;font-weight:650}.posts-time-metrics-event-time{overflow:hidden;color:var(--dmuted2);font-family:var(--font-geist-mono),monospace;font-size:10.5px;text-overflow:ellipsis;white-space:nowrap}.posts-time-metrics-gap{padding:3px 6px;border-radius:6px;background:var(--surface2);color:var(--dmuted);font-family:var(--font-geist-mono),monospace;font-size:10.5px;white-space:nowrap}
.posts-queue-empty{font-size:13px;color:var(--dmuted);line-height:1.55}.posts-queue-loading{font-size:12px;color:var(--dmuted2);font-family:var(--font-geist-mono),monospace}.posts-queue-timeline{display:flex;flex-direction:column;gap:8px;margin-top:12px}.posts-queue-event{padding:9px 10px;border:1px solid var(--dborder);border-radius:9px;background:var(--surface2)}.posts-queue-event-top{display:flex;align-items:center;justify-content:space-between;gap:8px;margin-bottom:5px}.posts-queue-event-meta{font-size:11.5px;color:var(--dmuted2);font-family:var(--font-geist-mono),monospace;text-align:right}.posts-queue-event-error{font-size:12px;color:var(--dmuted);line-height:1.5;white-space:pre-wrap;word-break:break-word}
.posts-retry-row{display:flex;align-items:center;gap:10px;margin-top:10px}.posts-retry-btn{display:inline-flex;align-items:center;gap:6px;padding:7px 12px;font-size:13px;font-weight:600;color:var(--dtext);background:var(--surface2);border:1px solid var(--dborder);border-radius:8px;cursor:pointer;transition:border-color 140ms,background 140ms}.posts-retry-btn>svg{width:12px;height:12px}.posts-retry-btn:hover:not(:disabled){border-color:var(--daccent);background:color-mix(in srgb,var(--daccent) 12%,var(--surface2))}.posts-retry-btn:disabled{opacity:.55;cursor:not-allowed}.posts-retry-error{font-size:11.5px;color:var(--danger);font-family:var(--font-geist-mono),monospace}.is-spinning{animation:posts-results-spin 1s linear infinite}
.posts-fb-phases{display:flex;flex-direction:column;gap:5px;margin-top:8px;padding:10px 11px;background:var(--surface1);border:1px solid var(--dborder);border-radius:8px}.posts-fb-phase{display:flex;align-items:center;justify-content:space-between;font-size:12.5px}.posts-fb-phase-label{color:var(--dmuted)}.posts-fb-phase-status{font-family:var(--font-geist-mono),monospace;font-size:11px;text-transform:uppercase;letter-spacing:.05em;color:var(--dmuted2)}.posts-fb-phase-complete{color:var(--success)}.posts-fb-phase-progress{color:var(--primary)}.posts-fb-phase-error{color:var(--danger)}
@keyframes posts-results-spin{from{transform:rotate(0deg)}to{transform:rotate(360deg)}}
@media (max-width:900px){.posts-results-grid{grid-template-columns:minmax(0,1fr)}.posts-time-metrics-summary{grid-template-columns:minmax(0,1fr)}.posts-time-metrics-event{grid-template-columns:10px minmax(0,1fr)}.posts-time-metrics-gap{grid-column:2}.posts-result-title{flex-wrap:wrap}}
`;
