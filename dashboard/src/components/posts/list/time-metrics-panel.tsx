"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";

import type { PostDeliveryJob, SocialPost } from "@/lib/api";
import {
  buildTimeMetricPhases,
  formatTimeMetricDuration,
  formatTimeMetricTimestamp,
  getPlatformPostTotalDurationMs,
  getRetryCount,
} from "./time-metrics";

type TimeMetricsPanelProps = {
  post: SocialPost;
  result: NonNullable<SocialPost["results"]>[number];
  jobs: PostDeliveryJob[];
  loading: boolean;
  error: string | null;
};

export function TimeMetricsPanel({ post, result, jobs, loading, error }: TimeMetricsPanelProps) {
  const [open, setOpen] = useState(false);
  const totalMs = getPlatformPostTotalDurationMs(post, result);
  const phases = buildTimeMetricPhases(post, result, jobs);
  const retryCount = getRetryCount(jobs);

  return (
    <div className="posts-time-metrics-panel" style={{ marginTop: 10 }}>
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        className="posts-debug-toggle posts-time-metrics-toggle"
        aria-expanded={open}
      >
        {open ? <ChevronDown style={{ width: 12, height: 12 }} /> : <ChevronRight style={{ width: 12, height: 12 }} />}
        <span>Time Metrics</span>
        <span className="posts-time-metrics-total">{formatTimeMetricDuration(totalMs)}</span>
      </button>

      {open ? (
        <div className="posts-time-metrics-body">
          {loading && jobs.length === 0 ? (
            <div className="posts-time-metrics-notice">Loading detailed job timing…</div>
          ) : null}
          {error ? (
            <div className="posts-time-metrics-notice">Detailed job timing unavailable: {error}</div>
          ) : null}

          <div className="posts-time-metrics-summary">
            <div>
              <span>Total publishing time</span>
              <strong>{formatTimeMetricDuration(totalMs)}</strong>
            </div>
            <div>
              <span>Baseline</span>
              <strong>{post.scheduled_at ? "Scheduled" : "Created"}</strong>
            </div>
            <div>
              <span>Retry count</span>
              <strong>{retryCount}</strong>
            </div>
          </div>

          <div className="posts-time-metrics-timeline">
            {phases.map((phase) => (
              <div key={phase.key} className={`posts-time-metrics-event${phase.key === "published" ? " is-final" : ""}`}>
                <span className="posts-time-metrics-dot" aria-hidden="true" />
                <div className="posts-time-metrics-event-copy">
                  <span className="posts-time-metrics-event-label">{phase.label}</span>
                  <span className="posts-time-metrics-event-time">{formatTimeMetricTimestamp(phase.at)}</span>
                </div>
                <span className="posts-time-metrics-gap">
                  {formatTimeMetricDuration(phase.durationFromPreviousMs)}
                </span>
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}
