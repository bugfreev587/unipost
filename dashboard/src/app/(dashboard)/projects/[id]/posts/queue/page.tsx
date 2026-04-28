"use client";

import { useEffect, useMemo, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { AlertCircle, Archive, Loader2, RotateCcw, StopCircle } from "lucide-react";
import {
  cancelPostDeliveryJob,
  dismissPostDeliveryJob,
  getApiLimits,
  getPostDeliveryJobsSummary,
  listPostDeliveryJobs,
  listSocialPostSummaries,
  retryPostDeliveryJobNow,
  type ApiLimits,
  type PostDeliveryJob,
  type PostDeliveryJobsSummary,
  type SocialPostSummary,
} from "@/lib/api";
import { useWorkspaceId } from "@/lib/use-workspace-id";

const STATUS_BADGE: Record<string, string> = {
  pending: "dbadge-blue",
  running: "dbadge-blue",
  retrying: "dbadge-amber",
  dead: "dbadge-red",
};

const ACTIVE_JOB_STATES = new Set(["pending", "running", "retrying"]);

const CSS = `
.queue-shell{display:flex;flex-direction:column;gap:18px}
.queue-summary{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:12px}
.queue-card{background:var(--surface2);border:1px solid var(--dborder);border-radius:12px;padding:14px}
.queue-card-label{font-size:11px;letter-spacing:.08em;text-transform:uppercase;color:var(--dmuted2);margin-bottom:8px}
.queue-card-value{font-size:26px;font-weight:700;color:var(--dtext)}
.queue-group{background:var(--surface);border:1px solid var(--dborder);border-radius:14px;overflow:hidden}
.queue-group-head{display:flex;align-items:center;justify-content:space-between;gap:14px;padding:16px 18px;border-bottom:1px solid var(--dborder);background:var(--surface2)}
.queue-group-title{font-size:14px;font-weight:600;color:var(--dtext)}
.queue-group-sub{font-size:12px;color:var(--dmuted);margin-top:4px}
.queue-table{width:100%;border-collapse:collapse}
.queue-table th,.queue-table td{padding:12px 18px;text-align:left;border-bottom:1px solid var(--dborder);font-size:13px}
.queue-table th{font-size:11px;letter-spacing:.08em;text-transform:uppercase;color:var(--dmuted2);background:var(--surface2)}
.queue-table td{color:var(--dtext);vertical-align:top}
.queue-table tr:last-child td{border-bottom:none}
.queue-actions{display:flex;gap:8px;flex-wrap:wrap;justify-content:flex-end;white-space:nowrap}
.queue-error-cell{display:flex;flex-direction:column;align-items:flex-start;gap:4px;max-width:380px}
.queue-error-text--collapsed{display:-webkit-box;-webkit-line-clamp:1;-webkit-box-orient:vertical;overflow:hidden;text-overflow:ellipsis;line-height:1.45;max-width:100%;word-break:break-all}
.queue-error-text--expanded{white-space:pre-wrap;word-break:break-word;line-height:1.45;max-width:100%}
.queue-error-toggle{padding:0;border:0;background:transparent;color:var(--dmuted);font-size:11px;font-weight:600;cursor:pointer;text-decoration:underline dotted;text-underline-offset:2px}
.queue-error-toggle:hover{color:var(--dtext)}
.queue-btn{display:inline-flex;align-items:center;gap:6px;height:30px;padding:0 10px;border-radius:8px;border:1px solid var(--dborder);background:var(--surface2);color:var(--dtext);font-size:12px;font-weight:600;cursor:pointer}
.queue-btn:disabled{opacity:.5;cursor:not-allowed}
.queue-empty,.queue-loading{display:flex;flex-direction:column;align-items:center;justify-content:center;min-height:260px;text-align:center;border:1px dashed var(--dborder);border-radius:16px;color:var(--dmuted)}
.queue-error{display:flex;align-items:flex-start;gap:10px;padding:14px 16px;border:1px solid color-mix(in srgb,var(--danger) 30%,var(--dborder));border-radius:12px;background:var(--danger-soft);color:var(--danger)}
@media (max-width: 900px){.queue-summary{grid-template-columns:repeat(2,minmax(0,1fr))}.queue-table{display:block;overflow:auto}}
`;

// CapacityBanner shows the workspace's active queue depth against
// the plan's cap, so a customer who's about to hit the depth limit
// sees it before the next publish surfaces a 429. Non-blocking —
// purely informational. Uses the same green/amber/red threshold
// scheme as the API Limits settings page.
function CapacityBanner({ current, cap }: { current: number; cap: number }) {
  const pct = cap > 0 ? Math.min(100, (current / cap) * 100) : 0;
  const barColor = pct >= 90 ? "#f87171" : pct >= 60 ? "#fbbf24" : "var(--daccent)";
  return (
    <div
      style={{
        border: "1px solid var(--dborder)",
        borderRadius: 12,
        padding: "12px 16px",
        background: "var(--surface2)",
      }}
    >
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", gap: 12 }}>
        <div style={{ fontSize: 12, color: "var(--dmuted2)", letterSpacing: ".08em", textTransform: "uppercase" }}>
          Active capacity
        </div>
        <div style={{ fontSize: 13, color: "var(--dtext)", fontWeight: 600 }}>
          {current.toLocaleString()} / {cap.toLocaleString()}
        </div>
      </div>
      <div
        style={{
          marginTop: 8,
          height: 6,
          background: "var(--dborder)",
          borderRadius: 3,
          overflow: "hidden",
        }}
      >
        <div
          style={{
            width: `${pct}%`,
            height: "100%",
            background: barColor,
            transition: "width 0.4s ease, background 0.2s",
          }}
        />
      </div>
    </div>
  );
}

function badge(status: string) {
  const cls = STATUS_BADGE[status] || "dbadge-gray";
  return <span className={`dbadge ${cls}`}>{status}</span>;
}

function ErrorCell({ message }: { message: string | null | undefined }) {
  const [expanded, setExpanded] = useState(false);
  if (!message) return <>—</>;
  return (
    <div className="queue-error-cell">
      <div className={expanded ? "queue-error-text--expanded" : "queue-error-text--collapsed"}>
        {message}
      </div>
      <button
        type="button"
        className="queue-error-toggle"
        onClick={() => setExpanded((v) => !v)}
      >
        {expanded ? "Show less" : "Show more"}
      </button>
    </div>
  );
}

// Dead jobs come in two flavors and the UI used to render both as "—"
// in the Next Retry column, which made non-retriable failures look
// like the queue had silently given up. Split them so each case is
// obvious at a glance.
function nextRetryCell(job: PostDeliveryJob) {
  if (job.next_run_at) {
    return new Date(job.next_run_at).toLocaleString();
  }
  if (job.state === "dead") {
    if (job.attempts >= job.max_attempts) {
      return (
        <span
          style={{ color: "var(--dmuted)" }}
          title="All retry attempts were used. Use Retry now to start a fresh attempt."
        >
          Retries exhausted
        </span>
      );
    }
    return (
      <span
        style={{ color: "var(--danger)" }}
        title="The platform returned a non-retriable error (bad input, permission denied, app not yet audited, etc.). The queue stopped early because retrying would just repeat the same failure. Fix the underlying issue and use Retry now."
      >
        Won&apos;t retry — non-retriable
      </span>
    );
  }
  return "—";
}

function attemptsCell(job: PostDeliveryJob) {
  const label = `${job.attempts}/${job.max_attempts}`;
  if (job.state === "dead" && job.attempts < job.max_attempts) {
    return (
      <span title="Stopped early — the platform returned a non-retriable error">
        {label}{" "}
        <span style={{ color: "var(--dmuted2)", fontSize: 11 }}>(stopped)</span>
      </span>
    );
  }
  return label;
}

export default function QueuePage() {
  const workspaceId = useWorkspaceId();
  const { getToken } = useAuth();
  const [jobs, setJobs] = useState<PostDeliveryJob[]>([]);
  const [posts, setPosts] = useState<SocialPostSummary[]>([]);
  const [summary, setSummary] = useState<PostDeliveryJobsSummary | null>(null);
  const [limits, setLimits] = useState<ApiLimits | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busyJobId, setBusyJobId] = useState("");

  useEffect(() => {
    if (!workspaceId) return;
    let cancelled = false;

    const load = async () => {
      try {
        const token = await getToken();
        if (!token || cancelled) return;
        // ApiLimits failure is non-fatal — the page still renders the
        // queue without the capacity banner if the limits endpoint
        // hiccups. Fold it in via Promise.allSettled so one slow
        // limits read doesn't stall the visible queue table.
        const [jobsRes, summaryRes, postsRes, limitsRes] = await Promise.allSettled([
          listPostDeliveryJobs(token),
          getPostDeliveryJobsSummary(token),
          listSocialPostSummaries(token),
          getApiLimits(token),
        ]);
        if (cancelled) return;
        if (jobsRes.status === "fulfilled") setJobs(jobsRes.value.data);
        if (summaryRes.status === "fulfilled") setSummary(summaryRes.value.data);
        if (postsRes.status === "fulfilled") setPosts(postsRes.value.data);
        if (limitsRes.status === "fulfilled") setLimits(limitsRes.value.data);
        // Surface the first hard failure if everything-but-limits
        // failed; limits alone failing is silent.
        const fatal = [jobsRes, summaryRes, postsRes].find((r) => r.status === "rejected");
        if (fatal && fatal.status === "rejected") {
          setError(fatal.reason instanceof Error ? fatal.reason.message : "Failed to load queue");
        } else {
          setError("");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    load();
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") load();
    }, 8000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [workspaceId, getToken]);

  const grouped = useMemo(() => {
      const byId = new Map(posts.map((post) => [post.id, post]));
    const groups = new Map<string, { post?: SocialPostSummary; jobs: PostDeliveryJob[] }>();
    for (const job of jobs) {
      const current = groups.get(job.post_id) || { post: byId.get(job.post_id), jobs: [] };
      current.jobs.push(job);
      groups.set(job.post_id, current);
    }
    return Array.from(groups.entries()).map(([postId, value]) => {
      const visibleJobsByResult = new Map<string, PostDeliveryJob>();
      for (const job of value.jobs) {
        const current = visibleJobsByResult.get(job.social_post_result_id);
        if (!current) {
          visibleJobsByResult.set(job.social_post_result_id, job);
          continue;
        }

        const currentIsActive = ACTIVE_JOB_STATES.has(current.state);
        const nextIsActive = ACTIVE_JOB_STATES.has(job.state);
        if (nextIsActive && !currentIsActive) {
          visibleJobsByResult.set(job.social_post_result_id, job);
          continue;
        }
        if (nextIsActive === currentIsActive) {
          const currentUpdatedAt = new Date(current.updated_at).getTime();
          const nextUpdatedAt = new Date(job.updated_at).getTime();
          if (nextUpdatedAt > currentUpdatedAt) {
            visibleJobsByResult.set(job.social_post_result_id, job);
          }
        }
      }

      return {
        postId,
        post: value.post,
        jobs: Array.from(visibleJobsByResult.values()).sort((a, b) => {
          const aTime = new Date(a.updated_at).getTime();
          const bTime = new Date(b.updated_at).getTime();
          return bTime - aTime;
        }),
      };
    });
  }, [jobs, posts]);

  const runAction = async (job: PostDeliveryJob, action: "retry" | "cancel" | "dismiss") => {
    try {
      setBusyJobId(job.id);
      const token = await getToken();
      if (!token || !workspaceId) return;
      if (action === "retry") {
        await retryPostDeliveryJobNow(token, job.id);
      } else if (action === "cancel") {
        await cancelPostDeliveryJob(token, job.id);
      } else {
        await dismissPostDeliveryJob(token, job.id);
      }
      const [jobsRes, summaryRes] = await Promise.all([
        listPostDeliveryJobs(token),
        getPostDeliveryJobsSummary(token),
      ]);
      setJobs(jobsRes.data);
      setSummary(summaryRes.data);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Queue action failed");
    } finally {
      setBusyJobId("");
    }
  };

  return (
    <div className="queue-shell">
      <style>{CSS}</style>
      <div>
        <div className="dt-title">Queue</div>
        <div className="dt-subtitle">Platform deliveries currently pending, running, processing, or retrying.</div>
      </div>

      {limits && limits.queue_depth_cap > 0 && (
        <CapacityBanner
          current={limits.queue_depth_current}
          cap={limits.queue_depth_cap}
        />
      )}

      {summary && (
        <div className="queue-summary">
          <div className="queue-card"><div className="queue-card-label">Pending</div><div className="queue-card-value">{summary.pending_count}</div></div>
          <div className="queue-card"><div className="queue-card-label">Running</div><div className="queue-card-value">{summary.running_count}</div></div>
          <div className="queue-card"><div className="queue-card-label">Retrying</div><div className="queue-card-value">{summary.retrying_count}</div></div>
          <div className="queue-card"><div className="queue-card-label">Dead</div><div className="queue-card-value">{summary.dead_count}</div></div>
          <div className="queue-card"><div className="queue-card-label">Recovered Today</div><div className="queue-card-value">{summary.recovered_today_count}</div></div>
        </div>
      )}

      {error && (
        <div className="queue-error">
          <AlertCircle style={{ width: 16, height: 16, marginTop: 2 }} />
          <div>{error}</div>
        </div>
      )}

      {loading ? (
        <div className="queue-loading">
          <Loader2 style={{ width: 20, height: 20, animation: "spin 1s linear infinite", marginBottom: 10 }} />
          <div>Loading queue…</div>
        </div>
      ) : grouped.length === 0 ? (
        <div className="queue-empty">
          <div style={{ fontSize: 16, fontWeight: 600, color: "var(--dtext)", marginBottom: 6 }}>No active queue items</div>
          <div style={{ fontSize: 13 }}>Queued, running, retrying, and dead deliveries will show up here.</div>
        </div>
      ) : (
        grouped.map(({ postId, post, jobs: postJobs }) => (
          <section key={postId} className="queue-group">
            <div className="queue-group-head">
              <div>
                <div className="queue-group-title">{post?.caption || "Untitled post"}</div>
                <div className="queue-group-sub">
                  Post {postId.slice(0, 8)} • {post?.status || "unknown"} • {postJobs.length} delivery {postJobs.length === 1 ? "unit" : "units"}
                </div>
              </div>
            </div>
            <table className="queue-table">
              <thead>
                <tr>
                  <th>Platform</th>
                  <th>Status</th>
                  <th>Stage</th>
                  <th>Attempts</th>
                  <th>Next Retry</th>
                  <th>Error</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {postJobs.map((job) => (
                  <tr key={job.id}>
                    <td>{job.platform || "unknown"}</td>
                    <td>{badge(job.state)}</td>
                    <td>{job.failure_stage || (job.kind === "retry" ? "Retry queue" : job.state === "running" ? "Dispatching" : "Queued")}</td>
                    <td>{attemptsCell(job)}</td>
                    <td>{nextRetryCell(job)}</td>
                    <td><ErrorCell message={job.last_error} /></td>
                    <td>
                      <div className="queue-actions">
                        {job.state === "dead" && (
                          <>
                            <button className="queue-btn" disabled={busyJobId === job.id} onClick={() => runAction(job, "retry")}>
                              <RotateCcw style={{ width: 13, height: 13 }} />
                              Retry now
                            </button>
                            <button
                              className="queue-btn"
                              disabled={busyJobId === job.id}
                              onClick={() => runAction(job, "dismiss")}
                              title="Archive this delivery from the queue. The failure record stays in analytics."
                            >
                              <Archive style={{ width: 13, height: 13 }} />
                              Dismiss
                            </button>
                          </>
                        )}
                        {(job.state === "pending" || job.state === "retrying") && (
                          <button className="queue-btn" disabled={busyJobId === job.id} onClick={() => runAction(job, "cancel")}>
                            <StopCircle style={{ width: 13, height: 13 }} />
                            Cancel
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
        ))
      )}
    </div>
  );
}
