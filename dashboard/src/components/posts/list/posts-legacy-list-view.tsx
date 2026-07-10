"use client";

import { Fragment, useCallback, useEffect, useState, useRef } from "react";
import Link from "next/link";
import { useParams, useSearchParams, useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { useWorkspaceId } from "@/lib/use-workspace-id";
import {
  listSocialAccounts, listAllSocialPosts, cancelSocialPost, archiveSocialPost, restoreSocialPost, deleteSocialPost, retrySocialPostResult, rescheduleSocialPost,
  getActivation, getSocialPostQueue, listProfiles,
  type SocialAccount, type SocialPost, type Profile, type PostDeliveryJob,
} from "@/lib/api";
import { Plus, Search, MoreHorizontal, Copy, Pencil, Send, XCircle, Calendar, ChevronDown, ChevronRight, ExternalLink, Archive, Trash2, RotateCcw } from "lucide-react";
import { AccountDestinationIcon } from "@/components/account-destination-icon";
import { CreatePostDrawer } from "@/components/posts/create-post/create-post-drawer";
import { clearStoredReplay, readStoredReplay } from "@/components/tutorials/replay-storage";
import {
  clearStoredQuickstartSelectedAccountId,
  consumeStoredQuickstartSelectedAccountId,
} from "@/components/tutorials/quickstart-selection-storage";
import { describePostResultFailure } from "@/lib/post-result-errors";
import { TimeMetricsPanel } from "./time-metrics-panel";

type FilterTab = "all" | "published" | "scheduled" | "failed" | "draft" | "archived";

const STATUS_BADGE: Record<string, { cls: string; label: string }> = {
  published: { cls: "dbadge-green", label: "published" },
  scheduled: { cls: "dbadge-blue", label: "scheduled" },
  queued: { cls: "dbadge-blue", label: "queued" },
  queued_retry: { cls: "dbadge-amber", label: "queued retry" },
  waiting_retry: { cls: "dbadge-amber", label: "waiting retry" },
  reserved: { cls: "dbadge-blue", label: "reserved" },
  dispatching: { cls: "dbadge-blue", label: "dispatching" },
  retrying: { cls: "dbadge-amber", label: "retrying" },
  processing: { cls: "dbadge-blue", label: "processing" },
  partial: { cls: "dbadge-amber", label: "partial" },
  failed: { cls: "dbadge-red", label: "failed" },
  draft: { cls: "dbadge-gray", label: "draft" },
  cancelled: { cls: "dbadge-gray", label: "cancelled" },
};

const POSTS_POLL_INTERVAL_MS = 8000;

function statusBadge(status: string) {
  const b = STATUS_BADGE[status] || { cls: "dbadge-gray", label: status };
  return <span className={`dbadge ${b.cls}`}><span className="dbadge-dot" />{b.label}</span>;
}

function queueHint(post: SocialPost) {
  if ((post.queued_results_count ?? 0) > 0) {
    const count = post.queued_results_count ?? 0;
    return `${count} deliver${count === 1 ? "y" : "ies"} queued`;
  }
  if ((post.retrying_count ?? 0) > 0) {
    const count = post.retrying_count ?? 0;
    return `${count} deliver${count === 1 ? "y" : "ies"} retrying`;
  }
  if ((post.dead_count ?? 0) > 0 && post.status !== "failed") {
    const count = post.dead_count ?? 0;
    return `${count} dead deliver${count === 1 ? "y" : "ies"}`;
  }
  return "";
}

function sourceBadge(source: SocialPost["source"] | undefined) {
  const s = source || "ui";
  const cls = s === "api" ? "dbadge-blue" : "dbadge-gray";
  return <span className={`dbadge ${cls}`}>{s.toUpperCase()}</span>;
}

function profileLabel(post: SocialPost, profiles: Profile[]) {
  const ids = post.profile_ids ?? [];
  if (ids.length === 0) return <span style={{ color: "var(--dmuted2)", fontSize: 13 }}>—</span>;
  const names = ids.map((pid) => profiles.find((p) => p.id === pid)?.name || pid.slice(0, 8));
  const text = names.join(", ");
  return (
    <span title={text} style={{ fontSize: 13, color: "var(--dmuted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", display: "inline-block", maxWidth: 130 }}>
      {text}
    </span>
  );
}

// Extra CSS for this page
const CSS = `.dbadge-gray{background:color-mix(in srgb,var(--surface2) 82%,white);color:var(--dmuted);border:1px solid var(--dborder)}
.posts-header{display:flex;align-items:flex-start;justify-content:space-between;margin-bottom:28px;gap:20px}
.posts-header-actions{display:flex;align-items:center;gap:12px;flex-wrap:wrap;justify-content:flex-end}
.posts-bulk-btn{display:inline-flex;align-items:center;gap:7px;padding:9px 14px;border-radius:10px;border:1px solid var(--dborder);background:var(--surface2);color:var(--dtext);font-size:14px;font-weight:600;font-family:inherit;cursor:pointer;transition:all .12s}
.posts-bulk-btn:hover:not(:disabled){background:var(--surface3);border-color:var(--dborder2)}
.posts-bulk-btn:disabled{opacity:.45;cursor:not-allowed}
.posts-bulk-btn.danger{color:var(--danger);border-color:color-mix(in srgb,var(--danger) 26%,var(--dborder))}
.posts-bulk-btn.danger:hover:not(:disabled){background:var(--danger-soft);border-color:color-mix(in srgb,var(--danger) 38%,var(--dborder))}
.posts-selection-hint{font-size:13px;color:var(--dmuted2);min-width:108px;text-align:right;line-height:1.45}
.posts-filters{display:flex;align-items:center;gap:10px;margin-bottom:18px}
.posts-search{display:flex;align-items:center;gap:8px;background:var(--surface2);border:1px solid var(--dborder);border-radius:10px;padding:0 12px;height:38px;flex:0 1 260px}
.posts-search input{background:none;border:none;outline:none;color:var(--dtext);font-size:14px;font-family:inherit;width:100%}
.posts-search input::placeholder{color:var(--dmuted2)}
.posts-search svg{color:var(--dmuted2);flex-shrink:0}
.posts-select{background:var(--surface2);border:1px solid var(--dborder);border-radius:10px;padding:0 12px;height:38px;color:var(--dtext);font-size:14px;font-family:inherit;cursor:pointer;outline:none}
.posts-select:focus{border-color:var(--daccent)}
.posts-view-switch{display:inline-flex;align-items:center;gap:7px;height:38px;padding:0 13px;border-radius:10px;border:1px solid var(--dborder);background:var(--surface2);color:var(--dtext);font-size:14px;font-weight:650;text-decoration:none;transition:all .12s}
.posts-view-switch:hover{background:var(--surface3);border-color:var(--dborder2)}
.posts-row{cursor:pointer;transition:background .12s ease, box-shadow .12s ease}
/* Hover state — needs to read clearly in both themes. The flat
   --surface2 value we used before changed luminance by only a few
   percent and was barely visible. Switch to an accent-tinted mix
   so the hover pulses with the brand green at low opacity, which
   stays legible against either light (white) or dark (#0f0f0f)
   surfaces. The inset box-shadow on every cell adds a subtle bar
   on the left edge as an extra affordance — applied per-cell
   because <tr> doesn't reliably accept box-shadow across browsers. */
.posts-row:hover>td{background:color-mix(in srgb, var(--daccent) 9%, var(--surface))}
.posts-row:hover>td:first-child{box-shadow:inset 3px 0 0 var(--daccent)}
.posts-caption{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:420px;font-size:15px;line-height:1.5;color:var(--dtext);font-weight:500}
.posts-plats{display:flex;gap:3px;align-items:center}
.posts-plats-more{font-size:11.5px;color:var(--dmuted2);font-weight:600}
.posts-time{font-size:14px;color:var(--dmuted);line-height:1.5}
.posts-actions{position:relative}
.posts-actions-btn{background:none;border:1px solid transparent;border-radius:4px;padding:4px;cursor:pointer;color:var(--dmuted2);transition:all .1s;display:flex;align-items:center}
.posts-actions-btn:hover{background:var(--surface2);border-color:var(--dborder);color:var(--dtext)}
.posts-menu{position:fixed;background:var(--surface-raised);border:1px solid var(--dborder);border-radius:12px;padding:5px;min-width:190px;z-index:9999;box-shadow:0 16px 34px color-mix(in srgb,var(--shadow-color) 120%,transparent)}
.posts-menu-item{display:flex;align-items:center;gap:8px;padding:8px 11px;border-radius:8px;font-size:14px;color:var(--dmuted);cursor:pointer;transition:all .1s;border:none;background:none;width:100%;text-align:left;font-family:inherit}
.posts-menu-item:hover{background:var(--surface2);color:var(--dtext)}
.posts-menu-item svg{width:13px;height:13px;flex-shrink:0}
.posts-menu-item.danger{color:#ef4444}
.posts-menu-item.danger:hover{background:#ef444410}
.posts-tooltip-anchor{position:relative;display:inline-flex}
.posts-tooltip{position:absolute;left:50%;bottom:calc(100% + 10px);transform:translateX(-50%) translateY(4px);padding:8px 10px;border-radius:10px;border:1px solid var(--dborder);background:color-mix(in srgb,var(--surface-raised) 96%,black);color:var(--dtext);font-size:13px;line-height:1.5;white-space:nowrap;box-shadow:0 14px 30px color-mix(in srgb,var(--shadow-color) 120%,transparent);opacity:0;pointer-events:none;transition:opacity .12s,transform .12s;z-index:40}
.posts-tooltip-anchor:hover .posts-tooltip,.posts-tooltip-anchor:focus-within .posts-tooltip{opacity:1;transform:translateX(-50%) translateY(0)}
.posts-tooltip::after{content:"";position:absolute;left:50%;top:100%;transform:translateX(-50%);border:6px solid transparent;border-top-color:color-mix(in srgb,var(--surface-raised) 96%,black)}
.posts-select-cell{width:42px}
.posts-checkbox{appearance:none;width:16px;height:16px;border-radius:4px;border:1px solid var(--dborder2);background:var(--surface2);cursor:pointer;display:inline-flex;align-items:center;justify-content:center;position:relative;transition:all .12s}
.posts-checkbox:hover{border-color:var(--daccent)}
.posts-checkbox:checked{background:var(--daccent);border-color:var(--daccent)}
.posts-checkbox:checked::after{content:"";width:8px;height:5px;border-left:2px solid #03120e;border-bottom:2px solid #03120e;transform:rotate(-45deg);margin-top:-1px}
.posts-empty{text-align:center;padding:68px 20px}
.posts-empty-title{font-size:18px;font-weight:650;color:var(--dtext);margin-bottom:8px}
.posts-empty-sub{font-size:14px;line-height:1.6;color:var(--dmuted)}
.posts-expand-cell{background:var(--surface);padding:22px 24px;border-bottom:1px solid var(--dborder)}
.posts-expand-layout{display:flex;flex-direction:column;gap:22px}
.posts-meta-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:14px}
.posts-meta-card{background:var(--surface2);border:1px solid var(--dborder);border-radius:14px;padding:15px 16px}
.posts-meta-label{font-size:11.5px;font-weight:700;letter-spacing:.1em;text-transform:uppercase;color:var(--dmuted2);margin-bottom:8px}
.posts-meta-value{font-size:14px;color:var(--dtext);line-height:1.65;word-break:break-word}
.posts-results-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:16px}
.posts-result-card{background:var(--surface2);border:1px solid var(--dborder);border-radius:14px;padding:16px}
.posts-result-head{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-bottom:10px}
.posts-result-title{display:flex;align-items:center;gap:8px;min-width:0}
.posts-result-name{font-size:14px;font-weight:650;color:var(--dtext);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.posts-result-meta{display:flex;flex-direction:column;gap:4px;margin-bottom:12px}
.posts-result-text{font-size:13.5px;color:var(--dmuted);line-height:1.6}
.posts-result-link{display:inline-flex;align-items:center;gap:4px;color:var(--daccent);text-decoration:none;font-size:13px;font-weight:600}
.posts-result-link:hover{text-decoration:underline}
.posts-error-title{font-size:12px;font-weight:800;color:var(--danger);margin-bottom:6px}
.posts-error{font-size:12px;color:var(--danger);background:var(--danger-soft);border:1px solid color-mix(in srgb,var(--danger) 22%,transparent);border-radius:10px;padding:10px 12px;white-space:pre-wrap;word-break:break-word;font-family:var(--font-geist-mono),monospace;line-height:1.6;max-height:148px;overflow:auto}
.posts-hint{font-size:14px;color:var(--dtext);line-height:1.65}
.posts-hint-label{color:var(--dmuted)}
.posts-debug-panel{border:1px solid var(--dborder);border-radius:10px;background:var(--surface1)}
.posts-debug-toggle{display:flex;align-items:center;gap:6px;width:100%;padding:9px 11px;font-size:11.5px;font-weight:600;color:var(--dmuted);background:transparent;border:0;cursor:pointer;font-family:var(--font-geist-mono),monospace;text-transform:uppercase;letter-spacing:.08em}
.posts-debug-toggle:hover{color:var(--dtext)}
.posts-debug-body{border-top:1px solid var(--dborder);padding:10px 11px}
.posts-debug-actions{display:flex;justify-content:flex-end;margin-bottom:6px}
.posts-debug-copy{display:inline-flex;align-items:center;gap:4px;padding:4px 8px;font-size:11px;color:var(--dmuted);background:var(--surface2);border:1px solid var(--dborder);border-radius:7px;cursor:pointer;font-family:var(--font-geist-mono),monospace}
.posts-debug-copy:hover{color:var(--dtext);border-color:var(--daccent)}
.posts-debug-pre{font-size:11.5px;line-height:1.6;color:var(--dtext);background:var(--surface2);border:1px solid var(--dborder);border-radius:8px;padding:11px 12px;max-height:320px;overflow:auto;white-space:pre-wrap;word-break:break-all;font-family:var(--font-geist-mono),monospace}
.posts-submitted-panel{border:1px solid var(--dborder);border-radius:10px;background:var(--surface1)}
.posts-submitted-body{border-top:1px solid var(--dborder);padding:11px 12px}
.posts-submitted-list{display:grid;grid-template-columns:max-content 1fr;gap:6px 14px;margin:0}
.posts-submitted-row{display:contents}
.posts-submitted-row dt{font-size:11.5px;color:var(--dmuted2);text-transform:uppercase;letter-spacing:.08em;font-weight:600}
.posts-submitted-row dd{font-size:13px;color:var(--dtext);margin:0;word-break:break-word;white-space:pre-wrap;line-height:1.55}
.posts-time-metrics-panel{border:1px solid var(--dborder);border-radius:10px;background:var(--surface1);overflow:hidden}
.posts-time-metrics-toggle{justify-content:flex-start}
.posts-time-metrics-total{margin-left:auto;padding:3px 7px;border-radius:999px;background:color-mix(in srgb,var(--daccent) 12%,var(--surface2));color:var(--daccent);font-size:11px;letter-spacing:0;text-transform:none}
.posts-time-metrics-body{border-top:1px solid var(--dborder);padding:12px}
.posts-time-metrics-notice{margin-bottom:10px;padding:8px 10px;border:1px solid var(--dborder);border-radius:8px;background:var(--surface2);color:var(--dmuted);font-size:12px;line-height:1.5}
.posts-time-metrics-summary{display:grid;grid-template-columns:minmax(0,1.4fr) repeat(2,minmax(0,1fr));gap:10px;margin-bottom:14px}
.posts-time-metrics-summary>div{border-left:2px solid color-mix(in srgb,var(--daccent) 34%,var(--dborder));padding-left:9px;min-width:0}
.posts-time-metrics-summary span{display:block;margin-bottom:3px;color:var(--dmuted2);font-size:10px;font-weight:650;letter-spacing:.07em;text-transform:uppercase}
.posts-time-metrics-summary strong{display:block;color:var(--dtext);font-family:var(--font-geist-mono),monospace;font-size:12px;font-weight:650;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.posts-time-metrics-timeline{position:relative;display:flex;flex-direction:column}
.posts-time-metrics-timeline::before{content:"";position:absolute;top:12px;bottom:12px;left:4px;width:1px;background:var(--dborder)}
.posts-time-metrics-event{position:relative;display:grid;grid-template-columns:10px minmax(0,1fr) max-content;gap:9px;align-items:center;min-height:38px}
.posts-time-metrics-dot{position:relative;z-index:1;width:7px;height:7px;border:2px solid var(--surface1);border-radius:999px;background:var(--dmuted2);box-shadow:0 0 0 1px var(--dmuted2)}
.posts-time-metrics-event.is-final .posts-time-metrics-dot{background:var(--success);box-shadow:0 0 0 1px var(--success)}
.posts-time-metrics-event-copy{display:flex;flex-direction:column;gap:2px;min-width:0}
.posts-time-metrics-event-label{color:var(--dtext);font-size:11.5px;font-weight:650}
.posts-time-metrics-event-time{overflow:hidden;color:var(--dmuted2);font-family:var(--font-geist-mono),monospace;font-size:10.5px;text-overflow:ellipsis;white-space:nowrap}
.posts-time-metrics-gap{padding:3px 6px;border-radius:6px;background:var(--surface2);color:var(--dmuted);font-family:var(--font-geist-mono),monospace;font-size:10.5px;white-space:nowrap}
.posts-queue-panel{border:1px solid var(--dborder);border-radius:10px;background:var(--surface1);margin-top:10px}
.posts-queue-body{border-top:1px solid var(--dborder);padding:11px 12px}
.posts-queue-grid{display:grid;grid-template-columns:max-content 1fr;gap:6px 14px;margin:0}
.posts-queue-grid dt{font-size:11.5px;color:var(--dmuted2);text-transform:uppercase;letter-spacing:.08em;font-weight:600}
.posts-queue-grid dd{font-size:13px;color:var(--dtext);margin:0;word-break:break-word;white-space:pre-wrap;line-height:1.55}
.posts-queue-empty{font-size:13px;color:var(--dmuted);line-height:1.55}
.posts-queue-loading{font-size:12px;color:var(--dmuted2);font-family:var(--font-geist-mono),monospace}
.posts-queue-timeline{display:flex;flex-direction:column;gap:8px;margin-top:12px}
.posts-queue-event{padding:9px 10px;border:1px solid var(--dborder);border-radius:9px;background:var(--surface2)}
.posts-queue-event-top{display:flex;align-items:center;justify-content:space-between;gap:8px;margin-bottom:5px}
.posts-queue-event-meta{font-size:11.5px;color:var(--dmuted2);font-family:var(--font-geist-mono),monospace}
.posts-queue-event-error{font-size:12px;color:var(--dmuted);line-height:1.5;white-space:pre-wrap;word-break:break-word}
.posts-retry-row{display:flex;align-items:center;gap:10px;margin-top:10px}
.posts-retry-btn{display:inline-flex;align-items:center;gap:6px;padding:7px 12px;font-size:13px;font-weight:600;color:var(--dtext);background:var(--surface2);border:1px solid var(--dborder);border-radius:8px;cursor:pointer;transition:border-color 140ms,background 140ms}
.posts-retry-btn:hover:not(:disabled){border-color:var(--daccent);background:color-mix(in srgb,var(--daccent) 12%,var(--surface2))}
.posts-retry-btn:disabled{opacity:.55;cursor:not-allowed}
.posts-retry-error{font-size:11.5px;color:var(--danger);font-family:var(--font-geist-mono),monospace}
.posts-fb-phases{display:flex;flex-direction:column;gap:5px;margin-top:8px;padding:10px 11px;background:var(--surface1);border:1px solid var(--dborder);border-radius:8px}
.posts-fb-phase{display:flex;align-items:center;justify-content:space-between;font-size:12.5px}
.posts-fb-phase-label{color:var(--dmuted)}
.posts-fb-phase-status{font-family:var(--font-geist-mono),monospace;font-size:11px;text-transform:uppercase;letter-spacing:.05em;color:var(--dmuted2)}
.posts-fb-phase-complete{color:var(--success,#10b981)}
.posts-fb-phase-progress{color:var(--primary)}
.posts-fb-phase-error{color:var(--danger)}
@keyframes spin{from{transform:rotate(0deg)}to{transform:rotate(360deg)}}
.posts-row-toggle{display:inline-flex;align-items:center;justify-content:center;width:20px;height:20px;color:var(--dmuted2)}
.posts-dialog-backdrop{position:fixed;inset:0;background:rgba(0,0,0,.62);backdrop-filter:blur(2px);display:flex;align-items:center;justify-content:center;padding:20px;z-index:120}
.posts-dialog{width:min(100%,440px);background:var(--surface-raised);border:1px solid var(--dborder);border-radius:18px;padding:24px;box-shadow:0 28px 70px color-mix(in srgb,var(--shadow-color) 120%,transparent)}
.posts-dialog-title{font-size:20px;font-weight:700;color:var(--dtext);margin-bottom:10px;letter-spacing:-.02em}
.posts-dialog-body{font-size:15px;color:var(--dmuted);line-height:1.7;margin-bottom:20px}
.posts-dialog-field{display:flex;flex-direction:column;gap:8px;margin-bottom:18px}
.posts-dialog-label{font-size:12px;font-weight:700;letter-spacing:.08em;text-transform:uppercase;color:var(--dmuted2)}
.posts-dialog-input{height:44px;border-radius:12px;border:1px solid var(--dborder);background:var(--surface2);padding:0 12px;color:var(--dtext);font-size:14px;font-family:inherit;outline:none}
.posts-dialog-input:focus{border-color:var(--daccent)}
.posts-dialog-error{margin:-6px 0 16px;font-size:12.5px;color:var(--danger);line-height:1.55}
.posts-dialog-actions{display:flex;justify-content:flex-end;gap:10px}
@media (max-width: 900px){.posts-expand-cell{padding:14px 16px}.posts-results-grid{grid-template-columns:1fr}.posts-time-metrics-summary{grid-template-columns:1fr}.posts-time-metrics-event{grid-template-columns:10px minmax(0,1fr)}}
`;

type ConfirmAction =
  | { kind: "archive"; ids: string[] }
  | { kind: "delete"; ids: string[] }
  | { kind: "restore"; ids: string[] };

function toDateTimeLocalValue(value?: string | null) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function minScheduleTimeLocal() {
  return toDateTimeLocalValue(new Date(Date.now() + 60 * 1000).toISOString());
}

type PostsLegacyListViewProps = {
  showCalendarLink?: boolean;
};

export function PostsLegacyListView({ showCalendarLink = false }: PostsLegacyListViewProps) {
  const { id: profileId } = useParams<{ id: string }>();
  const workspaceId = useWorkspaceId();
  const { getToken } = useAuth();
  const router = useRouter();
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [posts, setPosts] = useState<SocialPost[]>([]);
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<FilterTab>("all");
  const [search, setSearch] = useState("");
  const [platformFilter, setPlatformFilter] = useState("all");
  const [menuOpen, setMenuOpen] = useState<string | null>(null);
  const [menuPos, setMenuPos] = useState<{ top: number; left: number } | null>(null);
  const [expandedPostId, setExpandedPostId] = useState<string | null>(null);
  const [pendingExpandedPostId, setPendingExpandedPostId] = useState<string | null>(null);
  // Auto-open the create drawer when arriving from activation modal
  // (?action=new&template=welcome). See activation-modal.tsx STEP_META.send_post.
  const searchParams = useSearchParams();
  const focusPostId = searchParams.get("post");
  const [showCreateModal, setShowCreateModal] = useState(searchParams.get("action") === "new");
  const initialCaption = searchParams.get("template") === "welcome" ? "Hello from UniPost 👋" : "";
  const replaySelectedAccountId = initialCaption ? readStoredReplay()?.selectedAccountId : undefined;
  const [quickstartSelectedAccountId] = useState<string | undefined>(() =>
    initialCaption && !replaySelectedAccountId
      ? consumeStoredQuickstartSelectedAccountId() ?? undefined
      : undefined
  );
  const [selectedPostIds, setSelectedPostIds] = useState<Set<string>>(new Set());
  const [confirmAction, setConfirmAction] = useState<ConfirmAction | null>(null);
  const [actionBusy, setActionBusy] = useState(false);
  const [reschedulePost, setReschedulePost] = useState<SocialPost | null>(null);
  const [rescheduleValue, setRescheduleValue] = useState("");
  const [rescheduleBusy, setRescheduleBusy] = useState(false);
  const [rescheduleError, setRescheduleError] = useState("");
  const menuRef = useRef<HTMLDivElement>(null);
  const rowRefs = useRef<Record<string, HTMLTableRowElement | null>>({});

  const clearTutorialParams = useCallback(() => {
    const url = new URL(window.location.href);
    url.searchParams.delete("action");
    url.searchParams.delete("template");
    router.replace(url.pathname + (url.search ? url.search : ""));
  }, [router]);

  const loadData = useCallback(async (opts?: { silent?: boolean }) => {
    if (!workspaceId) return; // wait for workspace resolution
    if (!opts?.silent) setLoading(true);
    try {
      const token = await getToken();
      if (!token) return;
      const [a, p, pr] = await Promise.all([
        listSocialAccounts(token, profileId),
        listAllSocialPosts(token),
        listProfiles(token).catch(() => ({ data: [] as Profile[] })),
      ]);
      setAccounts(a.data);
      setPosts(p.data);
      setProfiles(pr.data);
      return p.data;
    } catch (err) {
      console.error("Failed to load:", err);
    } finally {
      if (!opts?.silent) setLoading(false);
    }
  }, [getToken, profileId, workspaceId]);

  useEffect(() => { loadData(); }, [loadData]);

  useEffect(() => {
    if (!workspaceId) return;
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") {
        loadData({ silent: true });
      }
    }, POSTS_POLL_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [workspaceId, loadData]);

  useEffect(() => {
    if (!focusPostId || expandedPostId === focusPostId) return;
    if (!posts.some((post) => post.id === focusPostId)) return;
    setExpandedPostId(focusPostId);
    setPendingExpandedPostId(focusPostId);
  }, [expandedPostId, focusPostId, posts]);

  useEffect(() => {
    if (!pendingExpandedPostId) return;
    if (!posts.some((post) => post.id === pendingExpandedPostId)) return;
    setExpandedPostId(pendingExpandedPostId);
    window.requestAnimationFrame(() => {
      rowRefs.current[pendingExpandedPostId]?.scrollIntoView({
        behavior: "smooth",
        block: "start",
      });
    });
    setPendingExpandedPostId(null);
  }, [pendingExpandedPostId, posts]);

  // Close menu on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setMenuOpen(null);
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  async function handleCancel(postId: string) {
    try {
      const token = await getToken();
      if (!token) return;
      await cancelSocialPost(token, postId);
      loadData();
    } catch (err) { console.error("Cancel failed:", err); }
    setMenuOpen(null);
  }

  function openRescheduleDialog(post: SocialPost) {
    setReschedulePost(post);
    setRescheduleValue(toDateTimeLocalValue(post.scheduled_at));
    setRescheduleError("");
    setMenuOpen(null);
  }

  function closeRescheduleDialog() {
    if (rescheduleBusy) return;
    setReschedulePost(null);
    setRescheduleValue("");
    setRescheduleError("");
  }

  async function submitReschedule() {
    if (!reschedulePost) return;
    if (!rescheduleValue) {
      setRescheduleError("Pick a new scheduled time.");
      return;
    }
    const nextTime = new Date(rescheduleValue);
    if (Number.isNaN(nextTime.getTime())) {
      setRescheduleError("Enter a valid scheduled time.");
      return;
    }
    if (nextTime.getTime() < Date.now() + 60 * 1000) {
      setRescheduleError("Choose a time at least 60 seconds in the future.");
      return;
    }
    try {
      const token = await getToken();
      if (!token) return;
      setRescheduleBusy(true);
      setRescheduleError("");
      await rescheduleSocialPost(token, reschedulePost.id, nextTime.toISOString());
      await loadData();
      setReschedulePost(null);
      setRescheduleValue("");
    } catch (err) {
      console.error("Reschedule failed:", err);
      setRescheduleError(err instanceof Error ? err.message : "Failed to reschedule post.");
    } finally {
      setRescheduleBusy(false);
    }
  }

  async function runConfirmAction(action: ConfirmAction) {
    try {
      const token = await getToken();
      if (!token) return;
      setActionBusy(true);
      if (action.kind === "archive") {
        await Promise.all(action.ids.map((id) => archiveSocialPost(token, id)));
      }
      if (action.kind === "restore") {
        await Promise.all(action.ids.map((id) => restoreSocialPost(token, id)));
      }
      if (action.kind === "delete") {
        await Promise.all(action.ids.map((id) => deleteSocialPost(token, id)));
      }
      setSelectedPostIds((current) => {
        const next = new Set(current);
        action.ids.forEach((id) => next.delete(id));
        return next;
      });
      if (expandedPostId && action.ids.includes(expandedPostId)) setExpandedPostId(null);
      setConfirmAction(null);
      await loadData();
    } catch (err) {
      console.error("Post action failed:", err);
    } finally {
      setActionBusy(false);
    }
  }

  function requestArchive(ids: string[]) {
    if (ids.length === 0) return;
    setConfirmAction({ kind: "archive", ids });
    setMenuOpen(null);
  }

  function requestRestore(ids: string[]) {
    if (ids.length === 0) return;
    setConfirmAction({ kind: "restore", ids });
    setMenuOpen(null);
  }

  function requestDelete(ids: string[]) {
    if (ids.length === 0) return;
    setConfirmAction({ kind: "delete", ids });
    setMenuOpen(null);
  }

  function toggleSelected(postId: string, checked: boolean) {
    setSelectedPostIds((current) => {
      const next = new Set(current);
      if (checked) next.add(postId);
      else next.delete(postId);
      return next;
    });
  }

  // Filter logic
  const filtered = posts.filter((p) => {
    const isArchived = Boolean(p.archived_at);
    if (tab === "archived") return isArchived;
    if (isArchived) return false;
    // Tab filter
    if (tab === "published" && p.status !== "published") return false;
    if (tab === "scheduled" && p.status !== "scheduled") return false;
    if (tab === "failed" && p.status !== "failed" && p.status !== "partial") return false;
    if (tab === "draft" && p.status !== "draft") return false;
    // Search
    if (search && !(p.caption || "").toLowerCase().includes(search.toLowerCase())) return false;
    // Platform filter
    if (platformFilter !== "all") {
      const resultPlatforms = p.results?.map((r) => r.platform).filter(Boolean) || [];
      const fallbackPlatforms = p.target_platforms || [];
      const hasPlatform = [...new Set([...resultPlatforms, ...fallbackPlatforms])].includes(platformFilter);
      if (!hasPlatform) return false;
    }
    return true;
  });

  const tabCounts = {
    all: posts.filter((p) => !p.archived_at).length,
    published: posts.filter((p) => !p.archived_at && p.status === "published").length,
    scheduled: posts.filter((p) => !p.archived_at && p.status === "scheduled").length,
    failed: posts.filter((p) => !p.archived_at && (p.status === "failed" || p.status === "partial")).length,
    draft: posts.filter((p) => !p.archived_at && p.status === "draft").length,
    archived: posts.filter((p) => Boolean(p.archived_at)).length,
  };

  function getTime(post: SocialPost) {
    const d = post.status === "scheduled" ? post.scheduled_at : post.published_at || post.created_at;
    if (!d) return "";
    return new Date(d).toLocaleDateString("en-US", { month: "short", day: "numeric" });
  }

  function platformIcons(post: SocialPost) {
    const resultPlatforms = post.results?.map((r) => r.platform).filter(Boolean) || [];
    const platforms = [...new Set([...resultPlatforms, ...(post.target_platforms || [])])];
    const show = platforms.slice(0, 4);
    const more = platforms.length - show.length;
    return (
      <div className="posts-plats">
        {show.map((p) => <AccountDestinationIcon key={p} platform={p!} size={14} />)}
        {more > 0 && <span className="posts-plats-more">+{more}</span>}
      </div>
    );
  }

  function actionsMenu(post: SocialPost) {
    const isArchived = Boolean(post.archived_at);
    const items: { icon: React.ReactNode; label: string; action: () => void; danger?: boolean; tooltip?: string }[] = [];
    if (isArchived) {
      items.push({ icon: <RotateCcw />, label: "Restore", action: () => requestRestore([post.id]) });
    } else {
      items.push({ icon: <Archive />, label: "Archive", action: () => requestArchive([post.id]) });
    }
    if (post.status === "draft") {
      items.push({ icon: <Pencil />, label: "Edit", action: () => { setMenuOpen(null); } });
      items.push({ icon: <Send />, label: "Publish now", action: () => { setMenuOpen(null); } });
      items.push({ icon: <Calendar />, label: "Schedule", action: () => { setMenuOpen(null); } });
    }
    if (post.status === "scheduled") {
      items.push({ icon: <Calendar />, label: "Edit scheduled time", action: () => openRescheduleDialog(post) });
      items.push({ icon: <XCircle />, label: "Cancel", action: () => handleCancel(post.id), danger: true });
    }
    items.push({
      icon: <Trash2 />,
      label: "Delete",
      action: () => requestDelete([post.id]),
      danger: true,
      tooltip: "Deletes from UniPost only. Published posts stay live on social platforms.",
    });
    return items;
  }

  if (loading) return <div style={{ color: "var(--dmuted)", padding: 20 }}>Loading...</div>;

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />

      {/* Header */}
      <div className="posts-header">
        <div>
          <div className="dt-page-title">Posts</div>
          <div className="dt-subtitle">Published and scheduled content</div>
        </div>
        <div className="posts-header-actions">
          <div className="posts-selection-hint">
            {selectedPostIds.size > 0 ? `${selectedPostIds.size} selected` : "Select posts"}
          </div>
          {tab === "archived" ? (
            <button className="posts-bulk-btn" disabled={selectedPostIds.size === 0} onClick={() => requestRestore([...selectedPostIds])}>
              <RotateCcw style={{ width: 14, height: 14 }} />
              Restore
            </button>
          ) : (
            <button className="posts-bulk-btn" disabled={selectedPostIds.size === 0} onClick={() => requestArchive([...selectedPostIds])}>
              <Archive style={{ width: 14, height: 14 }} />
              Archive
            </button>
          )}
          <HoverHint text="Deletes from UniPost only. Published posts stay live on social platforms.">
            <button className="posts-bulk-btn danger" disabled={selectedPostIds.size === 0} onClick={() => requestDelete([...selectedPostIds])}>
              <Trash2 style={{ width: 14, height: 14 }} />
              Delete
            </button>
          </HoverHint>
          <button className="dbtn dbtn-primary" style={{ gap: 5 }} onClick={() => setShowCreateModal(true)}>
            <Plus style={{ width: 14, height: 14 }} /> Create
          </button>
        </div>
      </div>

      {/* Tabs + search + platform filter — single row */}
      <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 16, flexWrap: "wrap" }}>
        <div className="dtabs" style={{ marginBottom: 0 }}>
          {(["all", "published", "scheduled", "failed", "draft", "archived"] as FilterTab[]).map((t) => (
            <div key={t} className={`dtab ${tab === t ? "active" : ""}`} onClick={() => setTab(t)}>
              {t === "all" ? "All" : t.charAt(0).toUpperCase() + t.slice(1)}
              {t === "draft" ? "s" : ""}
              <span style={{ fontSize: 10, color: "var(--dmuted2)", marginLeft: 4 }}>
                {tabCounts[t]}
              </span>
            </div>
          ))}
        </div>
        <div style={{ flex: 1 }} />
        <div className="posts-search">
          <Search style={{ width: 13, height: 13 }} />
          <input placeholder="Search posts..." value={search} onChange={(e) => setSearch(e.target.value)} />
        </div>
        <select className="posts-select" value={platformFilter} onChange={(e) => setPlatformFilter(e.target.value)}>
          <option value="all">All platforms</option>
          {["twitter", "linkedin", "instagram", "threads", "tiktok", "youtube", "bluesky"].map((p) => (
            <option key={p} value={p}>{p.charAt(0).toUpperCase() + p.slice(1)}</option>
          ))}
        </select>
        {showCalendarLink ? (
          <Link className="posts-view-switch" href={`/projects/${profileId}/posts`}>
            <Calendar style={{ width: 15, height: 15 }} />
            Calendar View
          </Link>
        ) : null}
      </div>

      {/* Table */}
      {filtered.length === 0 ? (
        <div className="posts-empty">
          <div className="posts-empty-title">
            {posts.length === 0 ? "No posts yet" : `No ${tab === "all" ? "" : tab + " "}posts found`}
          </div>
          <div className="posts-empty-sub">
            {posts.length === 0
              ? "Create your first post to get started."
              : "Try adjusting your filters."}
          </div>
        </div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th className="posts-select-cell">
                  <input
                    type="checkbox"
                    className="posts-checkbox"
                    checked={filtered.length > 0 && filtered.every((post) => selectedPostIds.has(post.id))}
                    onChange={(e) => {
                      const checked = e.target.checked;
                      setSelectedPostIds((current) => {
                        const next = new Set(current);
                        filtered.forEach((post) => {
                          if (checked) next.add(post.id);
                          else next.delete(post.id);
                        });
                        return next;
                      });
                    }}
                  />
                </th>
                <th>Caption</th>
                <th style={{ width: 100 }}>Platforms</th>
                <th style={{ width: 70 }}>Source</th>
                <th style={{ width: 140 }}>Profile</th>
                <th style={{ width: 110 }}>Status</th>
                <th style={{ width: 100 }}>Time</th>
                <th style={{ width: 48 }}></th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((post) => {
                const isExpanded = expandedPostId === post.id;
                return (
                  <Fragment key={post.id}>
                    <tr
                      ref={(node) => {
                        rowRefs.current[post.id] = node;
                      }}
                      className="posts-row"
                      onClick={() => setExpandedPostId((current) => current === post.id ? null : post.id)}
                    >
                      <td className="posts-select-cell" onClick={(e) => e.stopPropagation()}>
                        <input
                          type="checkbox"
                          className="posts-checkbox"
                          checked={selectedPostIds.has(post.id)}
                          onChange={(e) => toggleSelected(post.id, e.target.checked)}
                        />
                      </td>
                      <td>
                        <span style={{ display: "flex", alignItems: "center", gap: 10 }}>
                          <span className="posts-row-toggle">
                            {isExpanded ? (
                              <ChevronDown style={{ width: 14, height: 14 }} />
                            ) : (
                              <ChevronRight style={{ width: 14, height: 14 }} />
                            )}
                          </span>
                          <span className="posts-caption" title={post.caption || undefined}>
                            {post.caption || "(no caption)"}
                          </span>
                        </span>
                      </td>
                      <td>{platformIcons(post)}</td>
                      <td>{sourceBadge(post.source)}</td>
                      <td>{profileLabel(post, profiles)}</td>
                      <td>
                        <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                          {statusBadge(post.status)}
                          {queueHint(post) ? (
                            <span style={{ fontSize: 11, color: "var(--dmuted2)", lineHeight: 1.3 }}>{queueHint(post)}</span>
                          ) : null}
                        </div>
                      </td>
                      <td><span className="posts-time">{getTime(post)}</span></td>
                      <td>
                        <div className="posts-actions" ref={menuOpen === post.id ? menuRef : undefined}>
                          <button
                            className="posts-actions-btn"
                            onClick={(e) => {
                              e.stopPropagation();
                              if (menuOpen === post.id) {
                                setMenuOpen(null);
                                setMenuPos(null);
                              } else {
                                const rect = e.currentTarget.getBoundingClientRect();
                                setMenuPos({ top: rect.bottom + 4, left: rect.right - 180 });
                                setMenuOpen(post.id);
                              }
                            }}
                          >
                            <MoreHorizontal style={{ width: 16, height: 16 }} />
                          </button>
                          {menuOpen === post.id && menuPos && (
                            <div className="posts-menu" style={{ top: menuPos.top, left: menuPos.left }}>
                              {actionsMenu(post).map((item) => {
                                const button = (
                                  <button
                                    key={item.label}
                                    className={`posts-menu-item${item.danger ? " danger" : ""}`}
                                    onClick={(e) => { e.stopPropagation(); item.action(); }}
                                  >
                                    {item.icon}
                                    {item.label}
                                  </button>
                                );
                                return item.tooltip ? (
                                  <HoverHint key={item.label} text={item.tooltip}>
                                    {button}
                                  </HoverHint>
                                ) : button;
                              })}
                            </div>
                          )}
                        </div>
                      </td>
                    </tr>
                    {isExpanded && (
                      <tr>
                        <td colSpan={8} className="posts-expand-cell">
                          <div className="posts-expand-layout">
                            <div className="posts-meta-grid">
                              <MetaCard label="Caption" value={post.caption || "(no caption)"} />
                              <MetaCard label="Mode" value={post.scheduled_at ? "Scheduled" : post.status === "draft" ? "Draft" : "Immediate"} />
                              <MetaCard label="Status" value={post.status} />
                              <MetaCard label="Created" value={formatLongDate(post.created_at)} />
                              <MetaCard label="Scheduled" value={post.scheduled_at ? formatLongDate(post.scheduled_at) : "—"} />
                              <MetaCard label="Published" value={post.published_at ? formatLongDate(post.published_at) : "—"} />
                            </div>
                            <div>
                              <div className="posts-meta-label" style={{ marginBottom: 10 }}>Platform Results</div>
                              <PostResultsGrid
                                post={post}
                                workspaceId={workspaceId}
                                onRetryComplete={async () => {
                                  await loadData();
                                }}
                              />
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {reschedulePost ? (
        <div className="posts-dialog-backdrop" onClick={closeRescheduleDialog}>
          <div className="posts-dialog" onClick={(e) => e.stopPropagation()}>
            <div className="posts-dialog-title">Edit Scheduled Time</div>
            <div className="posts-dialog-body">
              Update when <span style={{ color: "var(--dtext)", fontWeight: 600 }}>{reschedulePost.caption || "this post"}</span> should be published.
            </div>
            <div className="posts-dialog-field">
              <label className="posts-dialog-label" htmlFor="reschedule-at">Scheduled Time</label>
              <input
                id="reschedule-at"
                className="posts-dialog-input"
                type="datetime-local"
                value={rescheduleValue}
                min={minScheduleTimeLocal()}
                onChange={(e) => setRescheduleValue(e.target.value)}
                disabled={rescheduleBusy}
              />
            </div>
            {rescheduleError ? <div className="posts-dialog-error">{rescheduleError}</div> : null}
            <div className="posts-dialog-actions">
              <button className="dbtn dbtn-ghost" onClick={closeRescheduleDialog} disabled={rescheduleBusy}>
                Cancel
              </button>
              <button className="dbtn dbtn-primary" onClick={() => { void submitReschedule(); }} disabled={rescheduleBusy || !rescheduleValue}>
                {rescheduleBusy ? "Saving..." : "Save"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {/* Create post drawer */}
      <CreatePostDrawer
        open={showCreateModal}
        onOpenChange={(open) => {
          setShowCreateModal(open);
          if (!open && searchParams.get("action") === "new") {
            clearStoredReplay();
            clearStoredQuickstartSelectedAccountId();
            clearTutorialParams();
          }
        }}
        accounts={accounts}
        workspaceId={workspaceId}
        getToken={getToken}
        onCreated={async (postId) => {
          if (tab !== "all") setTab("all");
          if (postId) {
            setPendingExpandedPostId(postId);
            setExpandedPostId(postId);
          }
          await loadData({ silent: true });
          // During activation (arrived via ?action=new&template=welcome),
          // bounce back to /projects/[id] so the Welcome modal re-pops
          // with step 2 ✓ and step 3 (optional) visible.
          if (initialCaption === "") return;
          try {
            const token = await getToken();
            if (!token) return;
            const res = await getActivation(token);
            if (!res.data.completed && !res.data.dismissed) {
              router.push(`/projects/${profileId}`);
            }
          } catch { /* silent */ }
        }}
        initialCaption={initialCaption}
        preselectAllAccounts={false}
        preselectedAccountIds={
          replaySelectedAccountId
            ? [replaySelectedAccountId]
            : quickstartSelectedAccountId
              ? [quickstartSelectedAccountId]
              : undefined
        }
      />

      {confirmAction ? (
        <div className="posts-dialog-backdrop" onClick={() => setConfirmAction(null)}>
          <div className="posts-dialog" onClick={(e) => e.stopPropagation()}>
            <div className="posts-dialog-title">
              {confirmAction.kind === "archive" ? "Archive posts?" : confirmAction.kind === "restore" ? "Restore posts?" : "Delete posts?"}
            </div>
            <div className="posts-dialog-body">
              {confirmAction.kind === "archive"
                ? `Archive ${confirmAction.ids.length} post${confirmAction.ids.length === 1 ? "" : "s"} from the overview list? You can still find them in the Archived tab.`
                : confirmAction.kind === "restore"
                  ? `Restore ${confirmAction.ids.length} archived post${confirmAction.ids.length === 1 ? "" : "s"} back to the main overview?`
                  : `Delete ${confirmAction.ids.length} post${confirmAction.ids.length === 1 ? "" : "s"} from UniPost? This removes the post from your UniPost dashboard and analytics only. It does not delete the published post on social platforms.`}
            </div>
            <div className="posts-dialog-actions">
              <button className="dbtn dbtn-ghost" onClick={() => setConfirmAction(null)}>
                Cancel
              </button>
              <button
                className={confirmAction.kind === "delete" ? "posts-bulk-btn danger" : "posts-bulk-btn"}
                disabled={actionBusy}
                onClick={() => { void runConfirmAction(confirmAction); }}
              >
                {actionBusy ? "Working..." : confirmAction.kind === "archive" ? "Archive" : confirmAction.kind === "restore" ? "Restore" : "Delete"}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </>
  );
}

function MetaCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="posts-meta-card">
      <div className="posts-meta-label">{label}</div>
      <div className="posts-meta-value">{value}</div>
    </div>
  );
}

function HoverHint({ children, text }: { children: React.ReactNode; text: string }) {
  return (
    <span className="posts-tooltip-anchor">
      {children}
      <span className="posts-tooltip" role="tooltip">{text}</span>
    </span>
  );
}

function PostResultsGrid({
  post,
  workspaceId,
  onRetryComplete,
}: {
  post: SocialPost;
  workspaceId: string;
  onRetryComplete?: () => void | Promise<void>;
}) {
  const { getToken } = useAuth();
  const [jobs, setJobs] = useState<PostDeliveryJob[] | null>(null);
  const [jobsLoading, setJobsLoading] = useState(false);
  const [jobsError, setJobsError] = useState<string | null>(null);
  const results = post.results || [];
  const shouldLoadQueue = results.length > 0;
  const resultQueueSignature = results
    .map((result) => `${result.id || result.social_account_id}:${result.status}:${result.published_at || ""}`)
    .join("|");

  useEffect(() => {
    let cancelled = false;
    if (!shouldLoadQueue) {
      setJobs(null);
      setJobsError(null);
      setJobsLoading(false);
      return;
    }

    const loadQueue = async () => {
      setJobsLoading(true);
      try {
        const token = await getToken();
        if (!token || cancelled) return;
        const res = await getSocialPostQueue(token, post.id);
        if (cancelled) return;
        setJobs(res.data.jobs || []);
        setJobsError(null);
      } catch (err) {
        if (cancelled) return;
        setJobsError(err instanceof Error ? err.message : "Failed to load queue details");
      } finally {
        if (!cancelled) setJobsLoading(false);
      }
    };

    void loadQueue();
    return () => {
      cancelled = true;
    };
  }, [
    getToken,
    post.id,
    post.status,
    post.queued_results_count,
    post.retrying_count,
    post.dead_count,
    shouldLoadQueue,
    resultQueueSignature,
  ]);

  if (results.length === 0) {
    return <div className="posts-result-text">No platform results yet.</div>;
  }
  return (
    <div className="posts-results-grid">
      {results.map((result) => (
        <PostResultCard
          key={result.social_account_id}
          post={post}
          workspaceId={workspaceId}
          result={result}
          jobs={(jobs || []).filter((job) => job.social_post_result_id === result.id)}
          jobsLoading={jobsLoading}
          jobsError={jobsError}
          onRetryComplete={onRetryComplete}
        />
      ))}
    </div>
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
      // Parent status may have flipped to published/partial —
      // reload the whole list so every row reflects the latest
      // derivation instead of surgically patching one card.
      await onRetryComplete?.();
    } catch (err) {
      setRetryError(err instanceof Error ? err.message : "Retry failed");
    } finally {
      setRetrying(false);
    }
  }, [result.id, retrying, getToken, workspaceId, post.id, onRetryComplete]);
  // Prefer the platform-provided URL (set by the adapter, e.g. Threads
  // permalink from the Graph API). Fall back to postUrlFor only if the
  // adapter didn't return one — important for Threads, whose public
  // URL uses shortcodes that aren't derivable from the numeric post ID.
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
            onClick={(e) => e.stopPropagation()}
            className="posts-result-link"
            title="Open original post"
          >
            <ExternalLink style={{ width: 12, height: 12 }} />
          </a>
        ) : null}
      </div>

      <div className="posts-result-meta">
        {result.account_name ? <div className="posts-result-text">{result.platform || "Unknown"}</div> : null}
        <div className="posts-result-text">
          {result.published_at ? formatLongDate(result.published_at) : post.published_at ? formatLongDate(post.published_at) : "Not published yet"}
        </div>
      </div>

      {result.status === "failed" ? (
        <>
          {failure ? (
            <div className="posts-error-title">{failure.title}</div>
          ) : null}
          <div className="posts-error">
            {failure?.message || result.error_message || "Publish failed (no error message reported)."}
          </div>
          {failure?.nextActionLabel ? (
            <div className="posts-hint" style={{ marginTop: 10 }}>
              <span className="posts-hint-label">Next: </span>
              {failure.actionHref ? (
                <>
                  {failure.actionHref.startsWith("http") ? (
                    <a href={failure.actionHref} target="_blank" rel="noreferrer" className="posts-result-link">
                      {failure.nextActionLabel}
                    </a>
                  ) : (
                    <Link href={failure.actionHref.replace(":id", workspaceId)} className="posts-result-link">
                      {failure.nextActionLabel}
                    </Link>
                  )}
                </>
              ) : failure.nextActionLabel}
            </div>
          ) : null}
          {failure?.retryStatusLabel ? (
            <div className="posts-hint" style={{ marginTop: 8 }}>
              <span className="posts-hint-label">Retry: </span>
              {failure.retryStatusLabel}
            </div>
          ) : null}
          {/* Retry button — only meaningful for failed rows. Fire-
              and-reload: on success the whole posts list is refetched
              because the parent post's status may have flipped from
              failed → partial / published. */}
          {result.id && failure?.canRetry ? (
            <div className="posts-retry-row">
              <button
                type="button"
                onClick={handleRetry}
                disabled={retrying}
                className="posts-retry-btn"
              >
                <RotateCcw style={{ width: 12, height: 12, animation: retrying ? "spin 1s linear infinite" : undefined }} />
                {retrying ? "Retrying…" : "Retry"}
              </button>
              {retryError ? (
                <span className="posts-retry-error">{retryError}</span>
              ) : null}
            </div>
          ) : null}
          {result.debug_curl ? <DebugCurlPanel curl={result.debug_curl} /> : null}
          <QueueDiagnostics jobs={jobs} loading={jobsLoading} error={jobsError} />
          <TimeMetricsPanel post={post} result={result} jobs={jobs} loading={jobsLoading} error={jobsError} />
          {result.submitted ? (
            <SubmittedSettingsPanel platform={result.platform || ""} submitted={result.submitted} />
          ) : null}
        </>
      ) : (
        <>
          <div className="posts-hint">
            {result.status === "published" ? "Published successfully." : result.status === "partial" ? "Partially completed. Review other platform cards for failures." : `Status: ${result.status}`}
            {result.external_id ? <div className="posts-result-text" style={{ marginTop: 10 }}>ID: {result.external_id}</div> : null}
          </div>
          {result.status === "processing" && result.platform === "facebook" ? (
            <FacebookProcessingPanel publishStatus={result.publish_status} />
          ) : null}
          <QueueDiagnostics jobs={jobs} loading={jobsLoading} error={jobsError} />
          <TimeMetricsPanel post={post} result={result} jobs={jobs} loading={jobsLoading} error={jobsError} />
          {result.submitted ? (
            <SubmittedSettingsPanel platform={result.platform || ""} submitted={result.submitted} />
          ) : null}
        </>
      )}
    </div>
  );
}

// FacebookProcessingPanel shows the three-phase lifecycle for a FB
// video post that's still being processed by Meta at view time:
// Uploading → Processing → Publishing. Renders whatever phase data
// /v1/social-posts/{id} refreshed server-side. The "status is
// processing" banner reassures the user that published will flip
// automatically once FB is done (via Get's re-poll logic).
function FacebookProcessingPanel({ publishStatus }: { publishStatus?: Record<string, unknown> }) {
  const phases: Array<{ label: string; status: string }> = [
    { label: "Uploading", status: (publishStatus?.uploading_phase_status as string) || "—" },
    { label: "Processing", status: (publishStatus?.processing_phase_status as string) || "—" },
    { label: "Publishing", status: (publishStatus?.publishing_phase_status as string) || "—" },
  ];
  return (
    <div className="posts-hint" style={{ marginTop: 10 }}>
      <div style={{ marginBottom: 6 }}>
        Facebook is still processing this video. The post will appear on the Page once all phases complete — usually within a few minutes for short clips.
      </div>
      <div className="posts-fb-phases">
        {phases.map((p) => (
          <div key={p.label} className="posts-fb-phase">
            <span className="posts-fb-phase-label">{p.label}</span>
            <span className={`posts-fb-phase-status ${statusToClass(p.status)}`}>{p.status}</span>
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

// DebugCurlPanel renders the captured curl dump from a failed publish.
// Collapsed by default in the user view so the error stays scannable;
// admins get the always-expanded variant via `defaultOpen`. Includes
// a "Copy" button because users are expected to paste this into their
// terminal when opening a support ticket.
function DebugCurlPanel({ curl, defaultOpen = false }: { curl: string; defaultOpen?: boolean }) {
  const [open, setOpen] = useState(defaultOpen);
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(curl);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard can be blocked by browser policy — fall back to a
      // manual select. The <pre> below is already selectable.
    }
  }, [curl]);

  return (
    <div className="posts-debug-panel" style={{ marginTop: 10 }}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="posts-debug-toggle"
        aria-expanded={open}
      >
        {open ? <ChevronDown style={{ width: 12, height: 12 }} /> : <ChevronRight style={{ width: 12, height: 12 }} />}
        <span>Debug request ({curl.split("\n# Request ").length - 1 || 1} HTTP call{curl.includes("\n# Request 2") ? "s" : ""})</span>
      </button>
      {open ? (
        <div className="posts-debug-body">
          <div className="posts-debug-actions">
            <button type="button" onClick={handleCopy} className="posts-debug-copy">
              <Copy style={{ width: 11, height: 11 }} />
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
}: {
  jobs: PostDeliveryJob[];
  loading: boolean;
  error: string | null;
}) {
  const [open, setOpen] = useState(true);
  if (loading && jobs.length === 0) {
    return (
      <div className="posts-queue-panel">
        <button type="button" className="posts-debug-toggle" aria-expanded>
          <ChevronDown style={{ width: 12, height: 12 }} />
          <span>Queue diagnostics</span>
        </button>
        <div className="posts-queue-body">
          <div className="posts-queue-loading">Loading queue details…</div>
        </div>
      </div>
    );
  }
  if (error && jobs.length === 0) {
    return (
      <div className="posts-queue-panel">
        <button type="button" className="posts-debug-toggle" aria-expanded>
          <ChevronDown style={{ width: 12, height: 12 }} />
          <span>Queue diagnostics</span>
        </button>
        <div className="posts-queue-body">
          <div className="posts-queue-empty">{error}</div>
        </div>
      </div>
    );
  }
  if (jobs.length === 0) return null;

  const sorted = [...jobs].sort((a, b) => {
    const aTime = Date.parse(a.updated_at) || 0;
    const bTime = Date.parse(b.updated_at) || 0;
    return bTime - aTime;
  });
  const active = sorted.find((job) => job.state === "pending" || job.state === "running" || job.state === "retrying");
  const latest = active || sorted[0];
  const timeline = sorted.slice(0, 3);

  return (
    <div className="posts-queue-panel" style={{ marginTop: 10 }}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="posts-debug-toggle"
        aria-expanded={open}
      >
        {open ? <ChevronDown style={{ width: 12, height: 12 }} /> : <ChevronRight style={{ width: 12, height: 12 }} />}
        <span>Queue diagnostics ({jobs.length})</span>
      </button>
      {open ? (
        <div className="posts-queue-body">
          <dl className="posts-queue-grid">
            <dt>Current state</dt>
            <dd>{latest.state}</dd>
            <dt>Delivery phase</dt>
            <dd>{humanizeCode(latest.delivery_phase || latest.state)}</dd>
            <dt>Queue lane</dt>
            <dd>{latest.kind === "retry" ? "Retry queue" : "Initial dispatch"}</dd>
            <dt>Attempts</dt>
            <dd>{latest.attempts}/{latest.max_attempts}</dd>
            <dt>Queued at</dt>
            <dd>{formatLongDate(latest.queued_at || latest.created_at)}</dd>
            <dt>Last update</dt>
            <dd>{formatLongDate(latest.updated_at)}</dd>
            {latest.next_run_at ? (
              <>
                <dt>Next retry</dt>
                <dd>{formatLongDate(latest.next_run_at)}</dd>
              </>
            ) : null}
            {latest.last_attempt_at ? (
              <>
                <dt>Last attempt</dt>
                <dd>{formatLongDate(latest.last_attempt_at)}</dd>
              </>
            ) : null}
            {latest.first_claimed_at ? (
              <>
                <dt>First claimed</dt>
                <dd>{formatLongDate(latest.first_claimed_at)}</dd>
              </>
            ) : null}
            {latest.platform_started_at ? (
              <>
                <dt>Platform started</dt>
                <dd>{formatLongDate(latest.platform_started_at)}</dd>
              </>
            ) : null}
            {latest.finished_at ? (
              <>
                <dt>Finished</dt>
                <dd>{formatLongDate(latest.finished_at)}</dd>
              </>
            ) : null}
            {typeof latest.queue_wait_ms === "number" ? (
              <>
                <dt>Queue wait</dt>
                <dd>{formatDurationMs(latest.queue_wait_ms)}</dd>
              </>
            ) : null}
            {typeof latest.worker_wait_ms === "number" ? (
              <>
                <dt>Worker wait</dt>
                <dd>{formatDurationMs(latest.worker_wait_ms)}</dd>
              </>
            ) : null}
            {typeof latest.platform_duration_ms === "number" ? (
              <>
                <dt>Platform duration</dt>
                <dd>{formatDurationMs(latest.platform_duration_ms)}</dd>
              </>
            ) : null}
            {latest.failure_stage ? (
              <>
                <dt>Failure stage</dt>
                <dd>{humanizeCode(latest.failure_stage)}</dd>
              </>
            ) : null}
            {latest.error_code ? (
              <>
                <dt>Internal code</dt>
                <dd>{latest.error_code}</dd>
              </>
            ) : null}
            {latest.platform_error_code ? (
              <>
                <dt>Platform code</dt>
                <dd>{latest.platform_error_code}</dd>
              </>
            ) : null}
            {latest.last_error ? (
              <>
                <dt>Worker note</dt>
                <dd>{latest.last_error}</dd>
              </>
            ) : null}
          </dl>

          {timeline.length > 1 ? (
            <div className="posts-queue-timeline">
              {timeline.map((job) => (
                <div key={job.id} className="posts-queue-event">
                  <div className="posts-queue-event-top">
                    <span>{statusBadge(job.delivery_phase || job.state)}</span>
                    <span className="posts-queue-event-meta">
                      {job.kind === "retry" ? "retry" : "dispatch"} · {formatLongDate(job.updated_at)}
                    </span>
                  </div>
                  {job.last_error ? (
                    <div className="posts-queue-event-error">{job.last_error}</div>
                  ) : null}
                </div>
              ))}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

// SubmittedSettingsPanel renders what the user actually sent for a
// given account — per-account caption override, media counts, and the
// platform-specific options that gated publishing (TikTok privacy +
// toggles, YouTube category/visibility, Instagram media_type, etc.).
// Collapsed by default; users expand it only when they want to review
// their own choices after the fact.
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
    <div className="posts-submitted-panel" style={{ marginTop: 10 }}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="posts-debug-toggle"
        aria-expanded={open}
      >
        {open ? <ChevronDown style={{ width: 12, height: 12 }} /> : <ChevronRight style={{ width: 12, height: 12 }} />}
        <span>Submitted settings ({rows.length})</span>
      </button>
      {open ? (
        <div className="posts-submitted-body">
          <dl className="posts-submitted-list">
            {rows.map((row) => (
              <div key={row.label} className="posts-submitted-row">
                <dt>{row.label}</dt>
                <dd>{row.value}</dd>
              </div>
            ))}
          </dl>
        </div>
      ) : null}
    </div>
  );
}

// buildSubmittedRows flattens a per-platform options blob into the
// label/value pairs we render. Each platform has its own block so field
// names look like what the user saw in the compose form (e.g. "Only me"
// rather than "SELF_ONLY"). Anything we don't know how to pretty-print
// falls through to a generic key/value dump so nothing gets silently
// dropped.
function buildSubmittedRows(
  platform: string,
  submitted: NonNullable<NonNullable<SocialPost["results"]>[number]["submitted"]>
): Array<{ label: string; value: React.ReactNode }> {
  const rows: Array<{ label: string; value: React.ReactNode }> = [];

  if (submitted.caption) {
    rows.push({ label: "Caption override", value: submitted.caption });
  }
  const mediaCount = (submitted.media_urls?.length || 0) + (submitted.media_ids?.length || 0);
  if (mediaCount > 0) {
    rows.push({ label: "Media attached", value: `${mediaCount} file${mediaCount === 1 ? "" : "s"}` });
  }
  if (submitted.first_comment) {
    rows.push({ label: "First comment", value: submitted.first_comment });
  }
  if (typeof submitted.thread_position === "number" && submitted.thread_position > 0) {
    rows.push({ label: "Thread position", value: String(submitted.thread_position) });
  }

  const opts = submitted.platform_options;
  if (!opts) return rows;

  switch (platform) {
    case "tiktok":
      pushTikTokRows(rows, opts);
      break;
    case "youtube":
      pushYouTubeRows(rows, opts);
      break;
    case "instagram":
      pushInstagramRows(rows, opts);
      break;
    case "linkedin":
      pushLinkedInRows(rows, opts);
      break;
    default:
      pushGenericRows(rows, opts);
  }
  return rows;
}

const TIKTOK_PRIVACY_LABELS: Record<string, string> = {
  PUBLIC_TO_EVERYONE: "Everyone",
  MUTUAL_FOLLOW_FRIENDS: "Friends",
  FOLLOWER_OF_CREATOR: "Followers",
  SELF_ONLY: "Only me",
};

function pushTikTokRows(
  rows: Array<{ label: string; value: React.ReactNode }>,
  opts: Record<string, unknown>
) {
  if (typeof opts.privacy_level === "string") {
    rows.push({ label: "Who can view", value: TIKTOK_PRIVACY_LABELS[opts.privacy_level] || opts.privacy_level });
  }
  // TikTok's API uses disable_* with inverted semantics; render as the
  // user-facing "Allow ..." phrasing so the panel matches the compose UI.
  const interactions: string[] = [];
  if (opts.disable_comment === false) interactions.push("Comment");
  if (opts.disable_duet === false) interactions.push("Duet");
  if (opts.disable_stitch === false) interactions.push("Stitch");
  if (interactions.length > 0) {
    rows.push({ label: "Allow interactions", value: interactions.join(", ") });
  } else if (
    opts.disable_comment === true ||
    opts.disable_duet === true ||
    opts.disable_stitch === true
  ) {
    rows.push({ label: "Allow interactions", value: "All disabled" });
  }
  if (opts.brand_organic_toggle === true || opts.brand_content_toggle === true) {
    const labels: string[] = [];
    if (opts.brand_organic_toggle === true) labels.push("Your Brand (Promotional content)");
    if (opts.brand_content_toggle === true) labels.push("Branded Content (Paid partnership)");
    rows.push({ label: "Commercial disclosure", value: labels.join(" + ") });
  }
}

function pushYouTubeRows(
  rows: Array<{ label: string; value: React.ReactNode }>,
  opts: Record<string, unknown>
) {
  if (typeof opts.title === "string" && opts.title) {
    rows.push({ label: "Video title", value: opts.title });
  }
  if (typeof opts.privacy_status === "string") {
    rows.push({ label: "Visibility", value: opts.privacy_status });
  }
  if (typeof opts.category_id === "string") {
    rows.push({ label: "Category", value: opts.category_id });
  }
  if (opts.shorts === true) {
    rows.push({ label: "Posted as", value: "Shorts" });
  }
  if (typeof opts.made_for_kids === "boolean") {
    rows.push({ label: "Made for kids", value: opts.made_for_kids ? "Yes" : "No" });
  }
  if (Array.isArray(opts.tags) && opts.tags.length > 0) {
    rows.push({ label: "Tags", value: (opts.tags as unknown[]).join(", ") });
  }
  if (typeof opts.publish_at === "string" && opts.publish_at) {
    rows.push({ label: "Scheduled for", value: opts.publish_at });
  }
  if (typeof opts.playlist_id === "string" && opts.playlist_id) {
    rows.push({ label: "Playlist", value: opts.playlist_id });
  }
  if (opts.contains_synthetic_media === true) {
    rows.push({ label: "AI-generated content", value: "Yes" });
  }
}

function pushInstagramRows(
  rows: Array<{ label: string; value: React.ReactNode }>,
  opts: Record<string, unknown>
) {
  if (typeof opts.mediaType === "string") {
    rows.push({ label: "Media type", value: opts.mediaType });
  } else if (typeof opts.media_type === "string") {
    rows.push({ label: "Media type", value: opts.media_type });
  }
}

function pushLinkedInRows(
  rows: Array<{ label: string; value: React.ReactNode }>,
  opts: Record<string, unknown>
) {
  if (typeof opts.visibility === "string") {
    rows.push({ label: "Visibility", value: opts.visibility });
  }
}

function pushGenericRows(
  rows: Array<{ label: string; value: React.ReactNode }>,
  opts: Record<string, unknown>
) {
  for (const [key, value] of Object.entries(opts)) {
    if (value === null || value === undefined || value === "" || value === false) continue;
    rows.push({ label: key, value: typeof value === "object" ? JSON.stringify(value) : String(value) });
  }
}

function postUrlFor(platform: string, externalId: string): string | null {
  switch (platform) {
    case "youtube":
      return `https://www.youtube.com/watch?v=${externalId}`;
    case "twitter":
      return `https://x.com/i/status/${externalId}`;
    case "instagram":
      return `https://www.instagram.com/p/${externalId}/`;
    case "threads":
      return `https://www.threads.net/post/${externalId}`;
    case "linkedin":
      if (externalId.startsWith("urn:li:")) return `https://www.linkedin.com/feed/update/${externalId}/`;
      return null;
    case "bluesky": {
      const match = externalId.match(/^at:\/\/(did:[^/]+)\/app\.bsky\.feed\.post\/(.+)$/);
      return match ? `https://bsky.app/profile/${match[1]}/post/${match[2]}` : null;
    }
    default:
      return null;
  }
}

function formatLongDate(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
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
  const remMinutes = minutes % 60;
  return remMinutes > 0 ? `${hours}h ${remMinutes}m` : `${hours}h`;
}

function humanizeCode(value: string): string {
  return value
    .split("_")
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function InlineStatusPill({ status }: { status: string }) {
  return statusBadge(status);
}
