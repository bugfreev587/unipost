"use client";

import { useEffect, useMemo, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { AlertCircle, Loader2, RotateCcw, StopCircle } from "lucide-react";
import {
  cancelPostDeliveryJob,
  getPostDeliveryJobsSummary,
  listPostDeliveryJobs,
  listSocialPosts,
  retryPostDeliveryJobNow,
  type PostDeliveryJob,
  type PostDeliveryJobsSummary,
  type SocialPost,
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
.queue-actions{display:flex;gap:8px;flex-wrap:wrap}
.queue-btn{display:inline-flex;align-items:center;gap:6px;height:30px;padding:0 10px;border-radius:8px;border:1px solid var(--dborder);background:var(--surface2);color:var(--dtext);font-size:12px;font-weight:600;cursor:pointer}
.queue-btn:disabled{opacity:.5;cursor:not-allowed}
.queue-empty,.queue-loading{display:flex;flex-direction:column;align-items:center;justify-content:center;min-height:260px;text-align:center;border:1px dashed var(--dborder);border-radius:16px;color:var(--dmuted)}
.queue-error{display:flex;align-items:flex-start;gap:10px;padding:14px 16px;border:1px solid color-mix(in srgb,var(--danger) 30%,var(--dborder));border-radius:12px;background:var(--danger-soft);color:var(--danger)}
@media (max-width: 900px){.queue-summary{grid-template-columns:repeat(2,minmax(0,1fr))}.queue-table{display:block;overflow:auto}}
`;

function badge(status: string) {
  const cls = STATUS_BADGE[status] || "dbadge-gray";
  return <span className={`dbadge ${cls}`}>{status}</span>;
}

export default function QueuePage() {
  const workspaceId = useWorkspaceId();
  const { getToken } = useAuth();
  const [jobs, setJobs] = useState<PostDeliveryJob[]>([]);
  const [posts, setPosts] = useState<SocialPost[]>([]);
  const [summary, setSummary] = useState<PostDeliveryJobsSummary | null>(null);
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
        const [jobsRes, summaryRes, postsRes] = await Promise.all([
          listPostDeliveryJobs(token, workspaceId),
          getPostDeliveryJobsSummary(token, workspaceId),
          listSocialPosts(token, workspaceId),
        ]);
        if (cancelled) return;
        setJobs(jobsRes.data);
        setSummary(summaryRes.data);
        setPosts(postsRes.data);
        setError("");
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "Failed to load queue");
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
    const groups = new Map<string, { post?: SocialPost; jobs: PostDeliveryJob[] }>();
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

  const runAction = async (job: PostDeliveryJob, action: "retry" | "cancel") => {
    try {
      setBusyJobId(job.id);
      const token = await getToken();
      if (!token || !workspaceId) return;
      if (action === "retry") {
        await retryPostDeliveryJobNow(token, workspaceId, job.id);
      } else {
        await cancelPostDeliveryJob(token, workspaceId, job.id);
      }
      const [jobsRes, summaryRes] = await Promise.all([
        listPostDeliveryJobs(token, workspaceId),
        getPostDeliveryJobsSummary(token, workspaceId),
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
                    <td>{job.attempts}/{job.max_attempts}</td>
                    <td>{job.next_run_at ? new Date(job.next_run_at).toLocaleString() : "—"}</td>
                    <td style={{ maxWidth: 360 }}>{job.last_error || "—"}</td>
                    <td>
                      <div className="queue-actions">
                        {job.state === "dead" && (
                          <button className="queue-btn" disabled={busyJobId === job.id} onClick={() => runAction(job, "retry")}>
                            <RotateCcw style={{ width: 13, height: 13 }} />
                            Retry now
                          </button>
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
