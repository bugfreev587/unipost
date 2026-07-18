"use client";

import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { useParams } from "next/navigation";
import {
  listInboxItems,
  getInboxUnreadCount,
  markInboxItemRead,
  markAllInboxRead,
  replyToInboxItem,
  syncInbox,
  updateInboxThreadState,
  listSocialAccounts,
  listSocialPostSummaries,
  getInboxMediaContext,
  getAccountCapabilities,
  getWorkspaceFeatureFlags,
  getXCreditsAllowance,
  getXInboxOutboundOperation,
  type InboxItem,
  type ApiFetchError,
  type SocialAccount,
  type SocialPostSummary,
  type IGMediaContext,
  type XInboxBackfillResult,
  type XInboxCapabilities,
  type XCreditsAllowance,
} from "@/lib/api";
import {
  canonicalInboxConversationKey,
  getInboxSourceDefinition,
  isInboxDMSource,
} from "@/lib/inbox-model";
import {
  evaluateXInboxEligibility,
  type XInboxEligibility,
} from "@/lib/x-inbox-eligibility";
import {
  beginXInboxOutboundOperation,
  classifyXInboxOutboundStatus,
  hashXInboxReplyBody,
  loadXInboxOutboundOperations,
  resolveXInboxOutboundOperation,
  saveXInboxOutboundOperations,
  updateXInboxOutboundOperation,
  type XInboxClientOutboundOperation,
  type XInboxClientOutboundStatus,
} from "@/lib/x-inbox-outbound-state";
import { useWorkspaceId } from "@/lib/use-workspace-id";
import { useInboxWebSocket } from "@/lib/use-inbox-ws";
import { buildContactPageHref } from "@/lib/support";
import { PlanGate } from "@/components/dashboard/plan-gate";
import { PlatformIcon } from "@/components/platform-icons";
import { isMetaDMReplyWindowClosed } from "./reply-window";
import {
  AlertTriangle,
  Archive,
  ArrowLeft,
  AtSign,
  CheckCheck,
  ChevronRight,
  Inbox as InboxIcon,
  Mail,
  MessageCircle,
  MessageSquare,
  RefreshCw,
  Search,
  Send,
  ShieldAlert,
} from "lucide-react";

type FilterTab = "comments" | "dms" | "threads";
type ThreadStatus = "open" | "assigned" | "resolved";

type SyncError = {
  account_id: string;
  platform: string;
  step: string;
  error: string;
};

type SyncResponse = {
  new_items?: number;
  accounts_checked?: number;
  errors?: SyncError[];
};

type XSyncState =
  | { kind: "idle" }
  | { kind: "estimate"; result: XInboxBackfillResult }
  | { kind: "pending"; result: XInboxBackfillResult; confirmationToken?: string }
  | { kind: "complete"; result: XInboxBackfillResult }
  | { kind: "error"; message: string };

type ConversationGroup = {
  id: string;
  threadKey: string;
  source: InboxItem["source"];
  title: string;
  subtitle: string;
  items: InboxItem[];
  accountName?: string;
  accountPlatform?: string;
  latestActivityAt: string;
  unreadCount: number;
  parentExternalID?: string;
  threadStatus: ThreadStatus;
  assignedTo?: string;
  linkedPostID?: string;
};

type CommentNode = {
  item: InboxItem;
  children: CommentNode[];
};

const COMMENT_THREAD_INDENT = 36;
// Theme-aware stroke for the comment-tree connectors. `--dborder2`
// maps to border-strong in both light and dark themes, so the line
// stays visible in either mode.
const COMMENT_THREAD_LINE_COLOR = "var(--dborder2)";
// Radius of the rounded bend where the vertical stroke turns into
// the horizontal stroke. Small enough to feel tight, big enough to
// read as a curve rather than a kink at most zoom levels.
const COMMENT_THREAD_BEND_RADIUS = 10;
const INBOX_RECENT_ITEM_LIMIT = 50;
const INBOX_UNREAD_ITEM_LIMIT = 500;

function xClientOutboundStatus(status: string): XInboxClientOutboundStatus {
  switch (status) {
    case "X_REMOTE_ACCEPTED_RECONCILING":
    case "remote_succeeded":
      return "remote_succeeded";
    case "X_USAGE_REVERSAL_PENDING":
    case "usage_reversal_pending":
    case "pending_recovery":
      return "usage_reversal_pending";
    case "X_WRITE_NEEDS_RECONCILIATION":
    case "needs_reconciliation":
      return "needs_reconciliation";
    case "X_WRITE_OUTCOME_PENDING":
    case "outcome_unknown":
      return "outcome_unknown";
    default:
      return "sending";
  }
}

function mergeInboxItems(...groups: InboxItem[][]): InboxItem[] {
  const byId = new Map<string, InboxItem>();
  for (const group of groups) {
    for (const item of group) {
      byId.set(item.id, item);
    }
  }
  return Array.from(byId.values()).sort(
    (a, b) => new Date(b.received_at).getTime() - new Date(a.received_at).getTime(),
  );
}

function initialsFromName(name?: string) {
  const value = (name || "?").trim();
  if (!value) return "?";
  return value.replace(/^@/, "").charAt(0).toUpperCase();
}

function Avatar({
  src,
  label,
  size = 36,
  dataId,
}: {
  src?: string;
  label?: string;
  size?: number;
  // Optional stable id written to a `data-comment-avatar` attribute
  // so the thread-line SVG overlay can find and measure this avatar.
  dataId?: string;
}) {
  if (src) {
    return (
      <img
        src={src}
        alt={label || "avatar"}
        data-comment-avatar={dataId}
        style={{
          width: size,
          height: size,
          borderRadius: "999px",
          objectFit: "cover",
          flexShrink: 0,
          background: "var(--surface2)",
          border: "1px solid var(--dborder)",
        }}
      />
    );
  }

  return (
    <div
      className="dt-mono"
      data-comment-avatar={dataId}
      style={{
        width: size,
        height: size,
        borderRadius: "999px",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        flexShrink: 0,
        background: "var(--surface2)",
        border: "1px solid var(--dborder2)",
        color: "var(--dmuted)",
        fontSize: size <= 28 ? 11 : 12,
      }}
    >
      {initialsFromName(label)}
    </div>
  );
}

function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diff = Math.max(0, now - then);
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(dateStr).toLocaleDateString();
}

function sourceLabel(source: InboxItem["source"]) {
  return getInboxSourceDefinition(source).shortLabel;
}

// platformFromSource maps an InboxItem.source back to the platform
// name used by PlatformIcon. Falls back to "instagram" for any
// unrecognized source so we never render a "missing icon" glyph —
// worst case the IG icon is wrong for a row, but it's still
// something recognizable.
function platformFromSource(source: InboxItem["source"] | ConversationGroup["source"]): string {
  return getInboxSourceDefinition(source).platform;
}

function sourceIcon(source: InboxItem["source"]) {
  switch (source) {
    case "ig_comment":
    case "youtube_comment":
    case "fb_comment":
    case "x_reply":
      return <MessageCircle style={{ width: 14, height: 14 }} />;
    case "ig_dm":
    case "fb_dm":
    case "x_dm":
      return <Mail style={{ width: 14, height: 14 }} />;
    case "threads_reply":
      return <AtSign style={{ width: 14, height: 14 }} />;
    default:
      return <MessageSquare style={{ width: 14, height: 14 }} />;
  }
}

function byNewestActivity(a: ConversationGroup, b: ConversationGroup) {
  // Only resolved threads sink to the bottom. Reading a message
  // (marking it read) must NOT reorder the list — only an explicit
  // "Resolve" click should move the group down.
  const priority = (status: ThreadStatus) => status === "resolved" ? 1 : 0;
  const pa = priority(a.threadStatus);
  const pb = priority(b.threadStatus);
  if (pa !== pb) return pa - pb;
  return new Date(b.latestActivityAt).getTime() - new Date(a.latestActivityAt).getTime();
}

// isDMSource returns true for any DM source. Several grouping +
// rendering branches below used to hard-code "ig_dm"; Facebook
// Messenger needs the exact same conversation-style handling, so
// the checks now route through this helper.
function isDMSource(source?: string | null): boolean {
  return isInboxDMSource(source);
}

function isMetaInboxPlatform(platform?: string | null): boolean {
  return platform === "instagram" || platform === "threads" || platform === "facebook";
}

function conversationRootKey(item: InboxItem, source: ConversationGroup["source"]) {
  if (isDMSource(source)) {
    return item.thread_key || item.parent_external_id || item.author_id || item.external_id;
  }

  if (source === "x_reply") {
    return item.thread_key || item.parent_external_id || item.external_id;
  }

  // For comment-style inbox items, prefer the internal linked post ID
  // when present. Facebook can surface different upstream identifiers
  // for the same underlying Page post across webhook vs sync paths;
  // grouping on linked_post_id keeps all comments on one published post
  // in the same left-hand conversation.
  if (item.linked_post_id) {
    return `post:${item.linked_post_id}`;
  }

  if (item.thread_key) return item.thread_key;
  if (!item.is_own) return item.external_id;
  return item.parent_external_id || item.external_id;
}

function groupItems(items: InboxItem[], source: ConversationGroup["source"]): ConversationGroup[] {
  const filtered = items.filter((item) => item.source === source);
  const map = new Map<string, InboxItem[]>();

  for (const item of filtered) {
    const key = canonicalInboxConversationKey(item, filtered);
    const existing = map.get(key) || [];
    existing.push(item);
    map.set(key, existing);
  }

  return Array.from(map.entries()).map(([key, groupedItems]) => {
    const sorted = [...groupedItems].sort(
      (a, b) => new Date(a.received_at).getTime() - new Date(b.received_at).getTime()
    );
    const latest = sorted[sorted.length - 1];
    const firstInbound = sorted.find((item) => !item.is_own) || sorted[0];
    const latestInbound =
      [...sorted].reverse().find((item) => !item.is_own) || latest;
    const unreadCount = sorted.filter((item) => !item.is_read && !item.is_own).length;
    const title =
      isDMSource(source)
        ? `@${firstInbound.author_name || firstInbound.author_id || "unknown"}`
        : ""; // enriched later with post caption
    const subtitle =
      isDMSource(source)
        ? (latest.body || "(no text)")
        : (latestInbound.body || latest.body || "(no text)");

    return {
      id: key,
      threadKey: firstInbound.thread_key || conversationRootKey(firstInbound, source),
      source,
      title,
      subtitle,
      items: sorted,
      accountName: latest.account_name || undefined,
      accountPlatform: latest.account_platform || undefined,
      latestActivityAt: isDMSource(source) ? latest.received_at : latestInbound.received_at,
      unreadCount,
      parentExternalID:
        isDMSource(source)
          ? latest.parent_external_id
          : firstInbound.parent_external_id || latest.parent_external_id,
      threadStatus: latest.thread_status || "open",
      assignedTo: latest.assigned_to,
      linkedPostID: latest.linked_post_id,
    };
  });
}

function buildCommentTree(items: InboxItem[]): CommentNode[] {
  const nodeMap = new Map<string, CommentNode>();
  for (const item of items) {
    nodeMap.set(item.external_id, { item, children: [] });
  }

  const roots: CommentNode[] = [];
  for (const item of items) {
    const node = nodeMap.get(item.external_id);
    if (!node) continue;

    const parentID = item.parent_external_id;
    const parentNode = parentID ? nodeMap.get(parentID) : undefined;

    if (parentNode) {
      parentNode.children.push(node);
      continue;
    }

    roots.push(node);
  }

  const sortNodes = (nodes: CommentNode[]) => {
    nodes.sort(
      (a, b) =>
        new Date(a.item.received_at).getTime() - new Date(b.item.received_at).getTime()
    );
    nodes.forEach((node) => sortNodes(node.children));
  };
  sortNodes(roots);

  return roots;
}

// CommentThread owns the layout + SVG overlay for a threaded comment
// view. It renders every comment node normally via the provided render
// function, then reads the actual post-layout DOM positions of each
// avatar (tagged with data-comment-avatar="<item.id>") and draws one
// continuous <path> per parent→child edge.
//
// Because the paths come from measured coordinates instead of stacked
// per-row decorations, they can't end up "too long" or "too short"
// relative to the content — the geometry is always tangent to the
// actual avatars the user sees.
function CommentThread({
  tree,
  render,
}: {
  tree: CommentNode[];
  render: (node: CommentNode, depth?: number) => React.ReactNode;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [edges, setEdges] = useState<
    { id: string; fromX: number; fromY: number; toX: number; toY: number }[]
  >([]);
  const [size, setSize] = useState<{ w: number; h: number }>({ w: 0, h: 0 });

  // Flatten (parentID, childID) pairs from the tree so the layout
  // effect can loop in a fixed order regardless of structure depth.
  const edgePairs = useMemo(() => {
    const pairs: { parent: string; child: string }[] = [];
    const walk = (nodes: CommentNode[]) => {
      for (const node of nodes) {
        for (const child of node.children) {
          pairs.push({ parent: node.item.id, child: child.item.id });
          walk([child]);
        }
      }
    };
    walk(tree);
    return pairs;
  }, [tree]);

  useLayoutEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const measure = () => {
      const rect = container.getBoundingClientRect();
      setSize({ w: rect.width, h: rect.height });
      const map = new Map<string, DOMRect>();
      container
        .querySelectorAll<HTMLElement>("[data-comment-avatar]")
        .forEach((el) => {
          const id = el.getAttribute("data-comment-avatar");
          if (id) map.set(id, el.getBoundingClientRect());
        });
      const next: typeof edges = [];
      for (const { parent, child } of edgePairs) {
        const pr = map.get(parent);
        const cr = map.get(child);
        if (!pr || !cr) continue;
        next.push({
          id: `${parent}->${child}`,
          // Bottom-center of parent avatar.
          fromX: pr.left + pr.width / 2 - rect.left,
          fromY: pr.bottom - rect.top,
          // Left-center of child avatar.
          toX: cr.left - rect.left,
          toY: cr.top + cr.height / 2 - rect.top,
        });
      }
      setEdges(next);
    };

    measure();
    // Re-measure on content changes, font loads, image loads, etc.
    const ro = new ResizeObserver(measure);
    ro.observe(container);
    container
      .querySelectorAll<HTMLElement>("[data-comment-avatar]")
      .forEach((el) => ro.observe(el));
    // Images in the thread (avatars, bubble media previews) can push
    // layout around as they decode — schedule one more pass once the
    // browser has painted everything.
    const raf = requestAnimationFrame(measure);
    return () => {
      ro.disconnect();
      cancelAnimationFrame(raf);
    };
  }, [edgePairs]);

  return (
    <div ref={containerRef} style={{ position: "relative" }}>
      {/* SVG overlay — absolutely positioned so it doesn't affect
          layout. Pointer events pass through to the underlying DOM. */}
      <svg
        aria-hidden="true"
        width={size.w}
        height={size.h}
        style={{
          position: "absolute",
          left: 0,
          top: 0,
          pointerEvents: "none",
        }}
      >
        {edges.map((e) => {
          // Path recipe: vertical from parent bottom-center, rounded
          // quarter-arc bend, horizontal to child left-center. One
          // continuous stroke per edge — no seams, no stitching.
          const r = Math.min(
            COMMENT_THREAD_BEND_RADIUS,
            Math.abs(e.toX - e.fromX),
            Math.abs(e.toY - e.fromY)
          );
          const d = [
            `M ${e.fromX} ${e.fromY}`,
            `V ${e.toY - r}`,
            `Q ${e.fromX} ${e.toY} ${e.fromX + r} ${e.toY}`,
            `H ${e.toX}`,
          ].join(" ");
          return (
            <path
              key={e.id}
              d={d}
              stroke={COMMENT_THREAD_LINE_COLOR}
              strokeWidth={2}
              fill="none"
              strokeLinecap="round"
            />
          );
        })}
      </svg>
      {tree.map((node) => render(node, 0))}
    </div>
  );
}

function StatusPill({ status, humanAgent = false }: { status: ThreadStatus; humanAgent?: boolean }) {
  const colors =
    status === "resolved"
      ? { bg: "var(--surface2)", border: "var(--dborder2)", color: "var(--dmuted)" }
      : status === "assigned"
        ? { bg: "rgb(59 130 246 / 0.10)", border: "rgb(59 130 246 / 0.22)", color: "#2563eb" }
        : { bg: "rgb(16 185 129 / 0.10)", border: "rgb(16 185 129 / 0.22)", color: "var(--daccent)" };

  return (
    <div style={{ display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
      <span
        className="dt-mono"
        style={{
          fontSize: 10,
          textTransform: "uppercase",
          padding: "3px 8px",
          borderRadius: 999,
          background: colors.bg,
          color: colors.color,
          border: `1px solid ${colors.border}`,
        }}
      >
        {status}
      </span>
      {humanAgent ? (
        <span
          className="dt-mono"
        style={{
          fontSize: 10,
          textTransform: "uppercase",
          padding: "3px 8px",
          borderRadius: 999,
          background: "rgb(245 158 11 / 0.10)",
          color: "#b45309",
          border: "1px solid rgb(245 158 11 / 0.22)",
        }}
      >
          Human agent
        </span>
      ) : null}
    </div>
  );
}

function SyncStateCard({
  icon,
  title,
  body,
  tone = "neutral",
  actionHref,
  onAction,
  actionLabel = "Contact support",
  actionDisabled = false,
}: {
  icon: React.ReactNode;
  title: string;
  body: string;
  tone?: "neutral" | "warn" | "error";
  actionHref?: string;
  onAction?: () => void;
  actionLabel?: string;
  actionDisabled?: boolean;
}) {
  const styles =
    tone === "error"
      ? { bg: "rgb(239 68 68 / 0.08)", border: "rgb(239 68 68 / 0.18)", color: "#dc2626" }
      : tone === "warn"
        ? { bg: "rgb(245 158 11 / 0.08)", border: "rgb(245 158 11 / 0.18)", color: "#b45309" }
        : { bg: "var(--sidebar-accent)", border: "var(--dborder)", color: "var(--dmuted)" };

  return (
    <div
      style={{
        display: "flex",
        gap: 10,
        alignItems: "flex-start",
        padding: "12px 14px",
        borderRadius: 10,
        background: styles.bg,
        border: `1px solid ${styles.border}`,
      }}
    >
      <div style={{ color: styles.color, marginTop: 2 }}>{icon}</div>
      <div>
        <div className="dt-body-sm" style={{ fontWeight: 600, color: "var(--dtext)", marginBottom: 2 }}>
          {title}
        </div>
        <div className="dt-body-sm" style={{ color: "var(--dmuted)" }}>{body}</div>
        {onAction ? (
          <button
            type="button"
            className="dbtn dbtn-ghost"
            onClick={onAction}
            disabled={actionDisabled}
            style={{ marginTop: 10, fontSize: 12 }}
          >
            {actionLabel}
          </button>
        ) : actionHref ? (
          <a
            href={actionHref}
            style={{
              display: "inline-flex",
              marginTop: 10,
              color: "var(--dtext)",
              textDecoration: "none",
              fontSize: 12,
              fontWeight: 600,
            }}
          >
            {actionLabel}
          </a>
        ) : null}
      </div>
    </div>
  );
}

export default function InboxPage() {
  // Plan-gate (migration 059): Free + API workspaces see an upgrade
  // card instead of the inbox UI. Server-side enforcement is the
  // source of truth — this gate just shortcuts the UX.
  return (
    <PlanGate feature="inbox">
      <InboxPageInner />
    </PlanGate>
  );
}

function InboxPageInner() {
  const params = useParams<{ id: string }>();
  const profileId = Array.isArray(params?.id) ? params.id[0] : params?.id;
  const { getToken } = useAuth();
  const workspaceId = useWorkspaceId();

  const [items, setItems] = useState<InboxItem[]>([]);
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [socialPosts, setSocialPosts] = useState<SocialPostSummary[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [xSyncing, setXSyncing] = useState(false);
  const [tab, setTab] = useState<FilterTab>("comments");
  const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null);
  const [replyDrafts, setReplyDrafts] = useState<Record<string, string>>({});
  const [replyingGroupId, setReplyingGroupId] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [syncData, setSyncData] = useState<SyncResponse | null>(null);
  const [xSyncState, setXSyncState] = useState<XSyncState>({ kind: "idle" });
  const [xCapabilities, setXCapabilities] = useState<Record<string, XInboxCapabilities>>({});
  const [xCredits, setXCredits] = useState<XCreditsAllowance | null>(null);
  const [xDMsEnabled, setXDMsEnabled] = useState(false);
  const [xCreditsEnabled, setXCreditsEnabled] = useState(false);
  const [pageError, setPageError] = useState<string | null>(null);
  const [replyFeedback, setReplyFeedback] = useState<string | null>(null);
  const [xOutboundOperations, setXOutboundOperations] = useState<XInboxClientOutboundOperation[]>([]);
  const [mobileDetailOpen, setMobileDetailOpen] = useState(false);
  const [leftPaneWidth, setLeftPaneWidth] = useState(360);
  const isDragging = useRef(false);
  const xOutboundOperationsRef = useRef<XInboxClientOutboundOperation[]>([]);
  const containerRef = useRef<HTMLDivElement>(null);
  const [mediaContext, setMediaContext] = useState<Record<string, IGMediaContext>>({});

  const persistXOutboundOperations = useCallback((operations: XInboxClientOutboundOperation[]) => {
    xOutboundOperationsRef.current = operations;
    setXOutboundOperations(operations);
    if (typeof window !== "undefined") {
      saveXInboxOutboundOperations(window.localStorage, operations);
    }
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const operations = loadXInboxOutboundOperations(window.localStorage);
    xOutboundOperationsRef.current = operations;
    setXOutboundOperations(operations);
  }, []);

  const load = useCallback(async () => {
    if (!workspaceId) return;
    try {
      setPageError(null);
      const token = await getToken();
      if (!token) return;
      const [recentItemsRes, unreadRes, socialPostsRes, featureFlagsRes] = await Promise.all([
        listInboxItems(token, { limit: INBOX_RECENT_ITEM_LIMIT }),
        getInboxUnreadCount(token),
        listSocialPostSummaries(token),
        getWorkspaceFeatureFlags(token),
      ]);
      const nextXDMsEnabled = featureFlagsRes.data.flags.x_dms_v1;
      const nextXCreditsEnabled = featureFlagsRes.data.flags.x_credits_billing_v1;
      setXDMsEnabled(nextXDMsEnabled);
      setXCreditsEnabled(nextXCreditsEnabled);
      const unreadTotal = unreadRes.data.count;
      const unreadFetchLimit = Math.min(
        INBOX_UNREAD_ITEM_LIMIT,
        Math.max(INBOX_RECENT_ITEM_LIMIT, unreadTotal),
      );
      const unreadItemsRes = unreadTotal > 0
        ? await listInboxItems(token, {
            is_read: "false",
            is_own: "false",
            limit: unreadFetchLimit,
          })
        : null;
      const accountsRes = profileId ? await listSocialAccounts(token, profileId) : null;
      setItems(
        mergeInboxItems(recentItemsRes.data || [], unreadItemsRes?.data || [])
          .filter((item) => nextXDMsEnabled || item.source !== "x_dm"),
      );
      setUnreadCount(unreadTotal);
      setSocialPosts(socialPostsRes.data || []);
      if (accountsRes?.data) {
        setAccounts(accountsRes.data);
        const twitterAccounts = accountsRes.data.filter((account) => account.platform === "twitter");
        const [capabilityResults, creditsResult] = await Promise.all([
          Promise.allSettled(
            twitterAccounts.map(async (account) => ({
              accountId: account.id,
              capabilities: (await getAccountCapabilities(token, account.id)).data.x_inbox,
            })),
          ),
          nextXCreditsEnabled ? getXCreditsAllowance(token).catch(() => null) : Promise.resolve(null),
        ]);
        const nextCapabilities: Record<string, XInboxCapabilities> = {};
        for (const result of capabilityResults) {
          if (result.status === "fulfilled" && result.value.capabilities) {
            nextCapabilities[result.value.accountId] = result.value.capabilities;
          }
        }
        setXCapabilities(nextCapabilities);
        setXCredits(nextXCreditsEnabled ? creditsResult?.data || null : null);
      }
    } catch (err) {
      setPageError(err instanceof Error ? err.message : "Failed to load Inbox");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, getToken, profileId]);

  const reconcileXOutboundOperation = useCallback(async (
    operation: XInboxClientOutboundOperation,
    announce = true,
  ) => {
    if (!operation.operationId) return;
    try {
      const token = await getToken();
      if (!token) return;
      const response = await getXInboxOutboundOperation(token, operation.operationId);
      const classification = classifyXInboxOutboundStatus(response.data.status);
      if (classification.terminal) {
        persistXOutboundOperations(resolveXInboxOutboundOperation(
          xOutboundOperationsRef.current,
          operation.logicalKey,
        ));
        if (announce) setReplyFeedback("X reply completed and is now available in Inbox.");
        await load();
        return;
      }

      persistXOutboundOperations(updateXInboxOutboundOperation(
        xOutboundOperationsRef.current,
        operation.logicalKey,
        { status: xClientOutboundStatus(response.data.status) },
      ));
      if (announce) {
        setReplyFeedback(classification.manual
          ? "UniPost cannot safely determine whether X accepted this reply. Review X before sending anything again."
          : "X reply is still being reconciled. UniPost will not send it again while this operation is unresolved.");
      }
    } catch (err) {
      const apiError = err as ApiFetchError;
      if (apiError.status === 404) {
        persistXOutboundOperations(resolveXInboxOutboundOperation(
          xOutboundOperationsRef.current,
          operation.logicalKey,
        ));
        if (announce) setReplyFeedback("The prior X operation is no longer pending. Refresh Inbox before sending again.");
        await load();
        return;
      }
      if (announce) {
        setReplyFeedback(err instanceof Error ? err.message : "Could not refresh the X reply status");
      }
    }
  }, [getToken, load, persistXOutboundOperations]);

  useEffect(() => {
    setLoading(true);
    load();
  }, [load]);

  useEffect(() => {
    if (!workspaceId) return;
    let cancelled = false;
    const poll = async () => {
      const operations = xOutboundOperationsRef.current.filter(
        (operation) =>
          operation.workspaceId === workspaceId &&
          !!operation.operationId &&
          operation.status !== "needs_reconciliation",
      );
      for (const operation of operations) {
        if (cancelled) return;
        await reconcileXOutboundOperation(operation, false);
      }
    };
    void poll();
    const interval = window.setInterval(() => void poll(), 10_000);
    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, [workspaceId, reconcileXOutboundOperation, xOutboundOperations.length]);

  // Real-time: WebSocket pushes new items instantly.
  const { connected: wsConnected } = useInboxWebSocket(
    !!workspaceId,
    (newItem) => {
      if (!xDMsEnabled && newItem.source === "x_dm") return;
      setItems((prev) => {
        if (prev.some((i) => i.id === newItem.id)) return prev;
        return [...prev, newItem];
      });
      if (!newItem.is_own) setUnreadCount((c) => c + 1);
    },
    () => {
      // Background worker or manual sync found new items — reload all data
      load();
    }
  );

  // Fallback: poll every 30s only when WebSocket is not connected.
  useEffect(() => {
    if (wsConnected || !workspaceId) return;
    const interval = setInterval(() => load(), 30_000);
    return () => clearInterval(interval);
  }, [workspaceId, wsConnected, load]);

  // Enrich comment/thread group titles with post captions from mediaContext or socialPosts.
  const enrichGroupTitle = useCallback((group: ConversationGroup): ConversationGroup => {
    if (isDMSource(group.source) || group.title) return group;
    if (group.source === "x_reply") {
      return {
        ...group,
        title: group.accountName ? `@${group.accountName} on X` : "Conversation on X",
      };
    }
    const rootExternalID = group.parentExternalID || group.threadKey;
    // Try mediaContext first (fetched from IG API directly)
    if (rootExternalID && mediaContext[rootExternalID]?.caption) {
      return { ...group, title: mediaContext[rootExternalID].caption };
    }
    // Try socialPosts
    if (rootExternalID) {
      const post = socialPosts.find((p) =>
        (p.results || []).some((r) => r.external_id === rootExternalID)
      );
      if (post?.caption) {
        return { ...group, title: post.caption };
      }
    }
    return { ...group, title: group.accountName ? `@${group.accountName}` : "Post" };
  }, [mediaContext, socialPosts]);

  const commentsGroups = useMemo(() => [
    ...groupItems(items, "ig_comment"),
    ...groupItems(items, "youtube_comment"),
    ...groupItems(items, "fb_comment"),
    ...groupItems(items, "x_reply"),
  ].map(enrichGroupTitle), [items, enrichGroupTitle]);
  const dmGroups = useMemo(() => [
    ...groupItems(items, "ig_dm"),
    ...groupItems(items, "fb_dm"),
    ...(xDMsEnabled ? groupItems(items, "x_dm") : []),
  ], [items, xDMsEnabled]);
  const threadsGroups = useMemo(() => groupItems(items, "threads_reply").map(enrichGroupTitle), [items, enrichGroupTitle]);

  const activeGroups = useMemo(() => {
    const base =
      tab === "comments" ? commentsGroups : tab === "dms" ? dmGroups : threadsGroups;
    const q = search.trim().toLowerCase();
    const filtered = q
      ? base.filter((group) =>
          [group.title, group.subtitle, ...group.items.map((item) => `${item.author_name || ""} ${item.body || ""}`)]
            .join(" ")
            .toLowerCase()
            .includes(q)
        )
      : base;
    return [...filtered].sort((a, b) => byNewestActivity(a, b));
  }, [commentsGroups, dmGroups, threadsGroups, search, tab]);

  useEffect(() => {
    if (!activeGroups.length) {
      setSelectedGroupId(null);
      return;
    }
    if (!selectedGroupId || !activeGroups.some((group) => group.id === selectedGroupId)) {
      setSelectedGroupId(activeGroups[0].id);
    }
  }, [activeGroups, selectedGroupId]);

  const selectedGroup = activeGroups.find((group) => group.id === selectedGroupId) || null;
  const twitterAccounts = accounts.filter((account) => account.platform === "twitter");
  const xEligibilityByAccount = useMemo(() => {
    const result: Record<string, XInboxEligibility> = {};
    for (const account of twitterAccounts) {
      const capabilities = xCapabilities[account.id];
      if (capabilities) {
        result[account.id] = evaluateXInboxEligibility(account, capabilities);
      }
    }
    return result;
  }, [twitterAccounts, xCapabilities]);
  const xReconnectAccounts = twitterAccounts.filter(
    (account) => xEligibilityByAccount[account.id]?.reconnectRequired,
  );
  const xCredentialAccounts = twitterAccounts.filter(
    (account) => (xEligibilityByAccount[account.id]?.missingAppCredentials.length ?? 0) > 0,
  );
  const xDeliveryErrorAccounts = twitterAccounts.filter(
    (account) => xEligibilityByAccount[account.id]?.deliveryStatus === "error",
  );
  const xCapPaused = xCredits?.pause_paid_sources && xCredits.inbound_pause_reason === "daily_cap";
  const xAllowancePaused =
    xCredits?.pause_paid_sources && xCredits.inbound_pause_reason === "monthly_allowance";
  const hasWorkspaceXApp = twitterAccounts.some(
    (account) => xEligibilityByAccount[account.id]?.appMode === "workspace_x_app",
  );
  const xSyncPaidBlocked = (xCapPaused || xAllowancePaused) && !hasWorkspaceXApp;
  const workspaceXOutboundOperations = xOutboundOperations.filter(
    (operation) => operation.workspaceId === workspaceId,
  );
  const pendingXOutboundOperation = workspaceXOutboundOperations[0];
  const pendingXOutboundManual = pendingXOutboundOperation?.status === "needs_reconciliation";

  const reconnectAccounts = accounts.filter(
    (account) =>
      account.status === "reconnect_required" &&
      isMetaInboxPlatform(account.platform)
  );

  const missingPermissionErrors = (syncData?.errors || []).filter((error) =>
    /permission|scope|authorize|authorized|unsupported request/i.test(error.error)
  );
  const syncFailures = (syncData?.errors || []).filter(
    (error) => !missingPermissionErrors.includes(error)
  );

  const counts = {
    comments: commentsGroups.reduce((sum, group) => sum + group.unreadCount, 0),
    dms: dmGroups.reduce((sum, group) => sum + group.unreadCount, 0),
    threads: threadsGroups.reduce((sum, group) => sum + group.unreadCount, 0),
  };
  const metaAccountCount = accounts.filter((account) => isMetaInboxPlatform(account.platform)).length;

  async function handleSync() {
    if (!workspaceId) return;
    setSyncing(true);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await syncInbox(token);
      setSyncData((res.data as SyncResponse) || null);
      await load();
    } finally {
      setSyncing(false);
    }
  }

  async function handleXSync(confirmationToken?: string) {
    if (!workspaceId || twitterAccounts.length === 0) return;
    setXSyncing(true);
    setXSyncState({ kind: "idle" });
    try {
      const token = await getToken();
      if (!token) return;
      const response = await syncInbox(token, {
        x_backfill: {
          lookback_days: 7,
          max_items: 20,
          include_replies: true,
          include_dms: xDMsEnabled,
          confirmation_token: confirmationToken,
        },
      });
      const result = response.data as XInboxBackfillResult;
      if (result.status === "in_progress") {
        setXSyncState({ kind: "pending", result, confirmationToken });
      } else if (result.confirmation_required && result.confirmation_token) {
        setXSyncState({ kind: "estimate", result });
      } else {
        setXSyncState({ kind: "complete", result });
        await load();
      }
    } catch (err) {
      setXSyncState({
        kind: "error",
        message: err instanceof Error ? err.message : "X Inbox sync failed",
      });
    } finally {
      setXSyncing(false);
    }
  }

  async function handleMarkAllRead() {
    if (!workspaceId) return;
    const token = await getToken();
    if (!token) return;
    await markAllInboxRead(token);
    setItems((prev) => prev.map((item) => ({ ...item, is_read: true })));
    setUnreadCount(0);
    // Tell the sidebar (which lives outside this component tree) to
    // reset its badge immediately. Without this it'd wait up to 60s
    // for the next poll and the user would think the badge is stuck.
    if (typeof window !== "undefined") {
      window.dispatchEvent(new Event("inbox:mark-all-read"));
    }
  }

  async function openGroup(group: ConversationGroup) {
    setSelectedGroupId(group.id);
    setMobileDetailOpen(true);
    const unreadInbound = group.items.filter((item) => !item.is_read && !item.is_own);
    if (!workspaceId || unreadInbound.length === 0) return;
    const token = await getToken();
    if (!token) return;

    await Promise.all(
      unreadInbound.map((item) => markInboxItemRead(token, item.id).catch(() => undefined))
    );
    setItems((prev) =>
      prev.map((item) =>
        unreadInbound.some((candidate) => candidate.id === item.id)
          ? { ...item, is_read: true }
          : item
      )
    );
    setUnreadCount((count) => Math.max(0, count - unreadInbound.length));
    // Tell the sidebar so its badge decrements at the same instant
    // the per-tab counts do — see comment in markAllRead.
    if (typeof window !== "undefined") {
      window.dispatchEvent(
        new CustomEvent("inbox:mark-read", { detail: { count: unreadInbound.length } }),
      );
    }
  }

  async function handleReply(group: ConversationGroup, targetItem: InboxItem) {
    if (!workspaceId) return;
    // Check bottom DM input first, then per-message draft
    const draft = (replyDrafts["__dm_bottom__"] || replyDrafts[targetItem.id] || "").trim();
    if (!draft) return;
    const isX = targetItem.source === "x_reply" || targetItem.source === "x_dm";
    let xOperation: XInboxClientOutboundOperation | undefined;

    setReplyingGroupId(group.id);
    setReplyFeedback(null);
    try {
      const token = await getToken();
      if (!token) return;

      if (isX) {
        const bodyHash = await hashXInboxReplyBody(draft);
        const begun = beginXInboxOutboundOperation(
          xOutboundOperationsRef.current,
          {
            workspaceId,
            accountId: targetItem.social_account_id,
            source: targetItem.source as "x_reply" | "x_dm",
            targetItemId: targetItem.id,
            threadKey: group.threadKey,
            bodyHash,
          },
          () => `x-inbox-${crypto.randomUUID()}`,
        );
        xOperation = begun.operation;
        persistXOutboundOperations(begun.operations);

        // A previous response already supplied an operation id, so polling
        // is safer than issuing another POST. A changed body has a different
        // logical key and therefore starts a separate operation above.
        if (begun.reused && begun.operation.operationId) {
          await reconcileXOutboundOperation(begun.operation);
          return;
        }
      }

      const result = await replyToInboxItem(token, targetItem.id, draft, {
        idempotencyKey: xOperation?.idempotencyKey,
      });
      if (result.state === "completed") {
        if (xOperation) {
          persistXOutboundOperations(resolveXInboxOutboundOperation(
            xOutboundOperationsRef.current,
            xOperation.logicalKey,
          ));
        }
        setItems((prev) => prev.some((item) => item.id === result.data.id)
          ? prev
          : [...prev, result.data]);
        if (isX && xCreditsEnabled) {
          const credits = result.data.x_credits_counted ?? 0;
          const mode = result.data.x_credit_billing_mode === "workspace_x_app"
            ? "Workspace X app; no UniPost X Credits used."
            : `${credits.toLocaleString()} X Credits used.`;
          setReplyFeedback(`Sent on X. ${mode}`);
        } else if (isX) {
          setReplyFeedback("Sent on X.");
        }
        setReplyDrafts((prev) =>
          Object.fromEntries(Object.entries(prev).filter(([key]) => key !== targetItem.id && key !== "__dm_bottom__"))
        );
      } else if (xOperation) {
        const updated = updateXInboxOutboundOperation(
          xOutboundOperationsRef.current,
          xOperation.logicalKey,
          {
            status: xClientOutboundStatus(result.code),
            operationId: result.operation_id,
          },
        );
        persistXOutboundOperations(updated);
        setReplyFeedback(`${result.message} Retrying the same message will reuse this operation instead of sending twice.`);
      }
    } catch (err) {
      const apiError = err as ApiFetchError;
      if (xOperation && (apiError.status === undefined || apiError.status >= 500)) {
        persistXOutboundOperations(updateXInboxOutboundOperation(
          xOutboundOperationsRef.current,
          xOperation.logicalKey,
          { status: "outcome_unknown" },
        ));
        setReplyFeedback("The X response was interrupted, so the outcome is unknown. Retry the exact same message to reuse its idempotency key.");
      } else {
        if (xOperation) {
          persistXOutboundOperations(resolveXInboxOutboundOperation(
            xOutboundOperationsRef.current,
            xOperation.logicalKey,
          ));
        }
        setReplyFeedback(err instanceof Error ? err.message : "Reply failed");
      }
    } finally {
      setReplyingGroupId(null);
    }
  }

  async function handleSetThreadState(group: ConversationGroup, threadStatus: ThreadStatus) {
    if (!workspaceId || !group.items[0]) return;
    const token = await getToken();
    if (!token) return;

    const assignedTo =
      threadStatus === "assigned"
        ? group.assignedTo || "UniPost agent"
        : "";

    const distinctThreadLeaders = new Map<string, InboxItem>();
    for (const item of group.items) {
      const key = item.thread_key || item.parent_external_id || item.external_id;
      if (!distinctThreadLeaders.has(key)) {
        distinctThreadLeaders.set(key, item);
      }
    }

    await Promise.all(
      Array.from(distinctThreadLeaders.values()).map((item) =>
        updateInboxThreadState(token, item.id, {
          thread_status: threadStatus,
          assigned_to: assignedTo,
        })
      )
    );

    setItems((prev) =>
      prev.map((item) =>
        group.items.some((groupItem) => groupItem.id === item.id)
          ? {
              ...item,
              thread_status: threadStatus,
              assigned_to: assignedTo || undefined,
            }
          : item
      )
    );
  }

  const selectedPost = useMemo(() => {
    if (!selectedGroup) return null;
    if (selectedGroup.source === "x_reply") return null;
    const linkedPostID = selectedGroup.linkedPostID || selectedGroup.items.find((item) => item.linked_post_id)?.linked_post_id;
    if (linkedPostID) {
      const post = socialPosts.find((candidate) => candidate.id === linkedPostID);
      if (post) return post;
    }

    const rootExternalID =
      isDMSource(selectedGroup.source)
        ? selectedGroup.parentExternalID
        : selectedGroup.threadKey || selectedGroup.parentExternalID;

    if (rootExternalID) {
      return (
        socialPosts.find((candidate) =>
          (candidate.results || []).some((result) => result.external_id === rootExternalID)
        ) || null
      );
    }

    return null;
  }, [selectedGroup, socialPosts]);

  // Pre-fetch media context for all comment/thread groups to show post captions as titles.
  // Tracks attempted keys to prevent infinite retry on failed fetches.
  // Shared across both media-context effects so a key failure in one
  // effect doesn't re-fire in the other on the same render.
  const attemptedMediaKeys = useRef<Set<string>>(new Set());
  // Nonce incremented on `online` to force the fetch effects to re-run
  // after attemptedMediaKeys is cleared — otherwise React wouldn't see
  // any dep change and the retries would never fire.
  const [onlineNonce, setOnlineNonce] = useState(0);

  // When the browser goes back online, clear the attempted-keys cache
  // so failed fetches retry exactly once. Prevents a transient outage
  // from permanently disabling media-context for the session.
  useEffect(() => {
    function onOnline() {
      attemptedMediaKeys.current.clear();
      setOnlineNonce((n) => n + 1);
    }
    window.addEventListener("online", onOnline);
    return () => window.removeEventListener("online", onOnline);
  }, []);

  useEffect(() => {
    if (!workspaceId) return;
    const allGroups = [...commentsGroups, ...threadsGroups];
    const toFetch = allGroups.filter((g) => {
      if (g.source === "x_reply") return false;
      const key = g.parentExternalID || g.threadKey;
      return key && !mediaContext[key] && !attemptedMediaKeys.current.has(key) && g.items[0];
    });
    if (toFetch.length === 0) return;

    (async () => {
      const token = await getToken();
      if (!token) return;
      for (const group of toFetch.slice(0, 5)) {
        const key = group.parentExternalID || group.threadKey;
        if (!key || mediaContext[key] || attemptedMediaKeys.current.has(key)) continue;
        attemptedMediaKeys.current.add(key); // mark attempted BEFORE fetch
        try {
          const res = await getInboxMediaContext(token, group.items[0].id);
          if (res.data) {
            setMediaContext((prev) => ({ ...prev, [key!]: res.data }));
          }
        } catch (err) {
          // Log once per key (the Set guards future renders for this key).
          console.warn(`[inbox] media-context fetch failed for ${key}:`, err);
        }
      }
    })();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [commentsGroups.length, threadsGroups.length, workspaceId, onlineNonce]);

  // Fetch media context from platform API when post image isn't available locally.
  useEffect(() => {
    if (!selectedGroup || !workspaceId) return;
    if (isDMSource(selectedGroup.source)) return;
    if (selectedGroup.source === "x_reply") return;
    // Skip if selectedPost already has media_urls (image available locally).
    if (selectedPost && selectedPost.media_urls && selectedPost.media_urls.length > 0) return;
    const parentID = selectedGroup.parentExternalID || selectedGroup.threadKey;
    if (!parentID || mediaContext[parentID]) return;
    // Same attempted-keys guard as the pre-fetch effect above. Without this,
    // any re-render while the fetch is failing (WS reconnect churn, Clerk
    // token refresh) re-fires this effect into a console-flooding loop.
    if (attemptedMediaKeys.current.has(parentID)) return;
    const firstItem = selectedGroup.items[0];
    if (!firstItem) return;

    attemptedMediaKeys.current.add(parentID); // mark attempted BEFORE fetch
    (async () => {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await getInboxMediaContext(token, firstItem.id);
        if (res.data) {
          setMediaContext((prev) => ({ ...prev, [parentID]: res.data }));
        }
      } catch (err) {
        console.warn(`[inbox] media-context fetch failed for ${parentID}:`, err);
      }
    })();
  // onlineNonce in deps so we retry after the browser reconnects.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedGroup, selectedPost, workspaceId, onlineNonce]);

  const currentMediaContext = selectedGroup
    ? mediaContext[selectedGroup.parentExternalID || selectedGroup.threadKey || ""] || null
    : null;

  const detailStatus = selectedGroup ? selectedGroup.threadStatus || "open" : "open";
  const showHumanAgent = isDMSource(selectedGroup?.source);
  const commentTree = useMemo(
    () =>
      selectedGroup && !isDMSource(selectedGroup.source)
        ? buildCommentTree(selectedGroup.items)
        : [],
    [selectedGroup]
  );

  function handleDragStart(e: React.MouseEvent) {
    e.preventDefault();
    isDragging.current = true;
    const startX = e.clientX;
    const startWidth = leftPaneWidth;

    function onMouseMove(ev: MouseEvent) {
      if (!isDragging.current) return;
      const newWidth = Math.min(Math.max(startWidth + (ev.clientX - startX), 240), 600);
      setLeftPaneWidth(newWidth);
    }

    function onMouseUp() {
      isDragging.current = false;
      document.removeEventListener("mousemove", onMouseMove);
      document.removeEventListener("mouseup", onMouseUp);
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    }

    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    document.addEventListener("mousemove", onMouseMove);
    document.addEventListener("mouseup", onMouseUp);
  }

  function renderConversationItem(item: InboxItem, depth = 0) {
    if (!selectedGroup) return null;
    const draft = replyDrafts[item.id] || "";
    const replyOpen = Object.prototype.hasOwnProperty.call(replyDrafts, item.id);
    const isDM = isDMSource(selectedGroup.source);
    const avatarSrc = item.is_own ? item.account_avatar_url : item.author_avatar_url;
    // Meta's Graph API strips the `from` block for commenters who
    // haven't granted our app permission, so for fb_comment rows we
    // often have nothing to show. "Facebook user" is friendlier than
    // "unknown" and mirrors the fallback Meta itself uses in its
    // messenger + creator tools.
    const fallbackLabel = item.source === "fb_comment" ? "Facebook user" : "unknown";
    const avatarLabel = item.is_own ? selectedGroup.accountName || "You" : item.author_name || item.author_id || fallbackLabel;

    if (isDM) {
      // Compact IG-style DM bubble — no per-message actions, no username
      // between consecutive messages from same sender. Avatar only on
      // the last message in a run from the same author.
      const groupItems = selectedGroup.items;
      const idx = groupItems.indexOf(item);
      const prevItem = idx > 0 ? groupItems[idx - 1] : null;
      const nextItem = idx < groupItems.length - 1 ? groupItems[idx + 1] : null;
      const sameSenderAsPrev = prevItem && prevItem.is_own === item.is_own;
      const sameSenderAsNext = nextItem && nextItem.is_own === item.is_own;
      const showAvatar = !item.is_own && !sameSenderAsNext;

      return (
        <div
          key={item.id}
          style={{
            display: "flex",
            justifyContent: item.is_own ? "flex-end" : "flex-start",
            alignItems: "flex-end",
            gap: 8,
            marginTop: sameSenderAsPrev ? 1 : 10,
            paddingLeft: !item.is_own ? 0 : 40,
            paddingRight: item.is_own ? 0 : 40,
          }}
        >
          {!item.is_own ? (
            <div style={{ width: 28, flexShrink: 0 }}>
              {showAvatar ? <Avatar src={avatarSrc} label={avatarLabel} size={28} /> : null}
            </div>
          ) : null}
          <div
            style={{
              padding: "8px 14px",
              borderRadius: item.is_own
                ? (sameSenderAsNext ? "18px 18px 4px 18px" : "18px 18px 4px 18px")
                : (sameSenderAsNext ? "18px 18px 18px 4px" : "18px 18px 18px 4px"),
              ...(item.is_own
                ? sameSenderAsPrev && sameSenderAsNext
                  ? { borderRadius: "18px 4px 4px 18px" }
                  : sameSenderAsPrev
                    ? { borderRadius: "18px 4px 18px 18px" }
                    : sameSenderAsNext
                      ? { borderRadius: "18px 18px 4px 18px" }
                      : {}
                : sameSenderAsPrev && sameSenderAsNext
                  ? { borderRadius: "4px 18px 18px 4px" }
                  : sameSenderAsPrev
                    ? { borderRadius: "4px 18px 18px 18px" }
                    : sameSenderAsNext
                      ? { borderRadius: "18px 18px 18px 4px" }
                      : {}),
              background: item.is_own
                ? "var(--dtext)"
                : "var(--surface2)",
              color: item.is_own ? "var(--surface)" : "var(--dtext)",
              border: item.is_own ? "none" : "1px solid var(--dborder)",
              boxShadow: item.is_own ? "0 10px 24px rgb(15 23 42 / 0.12)" : "none",
              maxWidth: "70%",
              lineHeight: 1.45,
              fontSize: 13,
              wordBreak: "break-word",
              whiteSpace: "pre-wrap",
            }}
          >
            {item.body || "(no text)"}
          </div>
        </div>
      );
    }

    // Facebook-style comment bubble
    const avatarSize = depth > 0 ? 28 : 32;
    return (
      <div key={item.id} style={{ display: "flex", gap: 10, alignItems: "flex-start", maxWidth: "100%" }}>
        <Avatar src={avatarSrc} label={avatarLabel} size={avatarSize} dataId={item.id} />
        <div style={{ flex: 1, minWidth: 0 }}>
          {/* Bubble */}
          <div style={{
            display: "inline-block",
            padding: "8px 12px",
            borderRadius: 18,
            background: "var(--surface2)",
            border: "1px solid var(--dborder)",
            maxWidth: "100%",
          }}>
            <span className="dt-body-sm" style={{ fontWeight: 600, color: item.is_own ? "var(--daccent)" : "var(--dtext)", display: "block", marginBottom: 2, fontSize: 12 }}>
              {item.is_own ? "You" : (item.author_name || item.author_id || fallbackLabel)}
            </span>
            <span className="dt-body-sm" style={{ color: "var(--dtext)", whiteSpace: "pre-wrap", lineHeight: 1.5, fontSize: 13 }}>
              {item.body || "(no text)"}
            </span>
          </div>
          {/* Meta row below bubble */}
          <div style={{ display: "flex", alignItems: "center", gap: 12, paddingLeft: 4, marginTop: 3 }}>
            <span style={{ fontSize: 11, color: "var(--dmuted2)" }}>{timeAgo(item.received_at)}</span>
            {!item.is_own ? (
              <button
                onClick={() =>
                  setReplyDrafts((prev) =>
                    replyOpen
                      ? Object.fromEntries(Object.entries(prev).filter(([key]) => key !== item.id))
                      : { ...prev, [item.id]: "" }
                  )
                }
                style={{ border: "none", background: "transparent", color: "var(--dmuted)", cursor: "pointer", padding: 0, fontSize: 11, fontWeight: 600 }}
              >
                Reply
              </button>
            ) : null}
            {item.source === "x_reply" && item.url ? (
              <a
                href={item.url}
                target="_blank"
                rel="noopener noreferrer"
                style={{ fontSize: 11, color: "var(--dmuted)", fontWeight: 600 }}
              >
                View on X
              </a>
            ) : null}
            {/* Per-message Mark read / unread button removed — opening
                a conversation auto-marks every inbound message in it
                read (see openGroup), so the manual toggle was both
                redundant and lying (the toggle only mutated client
                state and never persisted to the server). */}
          </div>
          {/* Inline reply input */}
          {replyOpen ? (
            <div style={{ display: "flex", gap: 6, marginTop: 6 }}>
              <input
                value={draft}
                onChange={(e) => setReplyDrafts((prev) => ({ ...prev, [item.id]: e.target.value }))}
                aria-label={`Reply to ${item.author_name || "this comment"}`}
                placeholder={`Reply to ${item.author_name || "this comment"}...`}
                onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey && selectedGroup) { e.preventDefault(); handleReply(selectedGroup, item); } }}
                className="dt-body-sm"
                style={{
                  flex: 1, padding: "7px 12px", borderRadius: 18,
                  border: "1px solid var(--dborder)", background: "var(--sidebar)",
                  color: "var(--dtext)", outline: "none", fontSize: 12,
                }}
              />
              <button
                onClick={() => selectedGroup && handleReply(selectedGroup, item)}
                disabled={replyingGroupId === selectedGroup?.id || !draft.trim()}
                style={{
                  padding: "7px 12px", borderRadius: 18, border: "none",
                  background: "var(--daccent)", color: "var(--primary-foreground)",
                  cursor: !draft.trim() ? "not-allowed" : "pointer",
                  opacity: !draft.trim() ? 0.5 : 1, fontSize: 12,
                }}
              >
                {replyingGroupId === selectedGroup?.id ? "..." : "Reply"}
              </button>
            </div>
          ) : null}
        </div>
      </div>
    );
  }

  // Render a comment node as a plain indented row + recursive children.
  // No thread-line drawing here — that lives in the SVG overlay below,
  // which measures actual avatar positions after layout and draws one
  // continuous path per parent→child edge. This separation is why the
  // lines can't end up "too long" or "too short" relative to the
  // surrounding content: they're derived from measured coordinates, not
  // reassembled from per-row segments that have to meet by hand.
  function renderCommentNode(node: CommentNode, depth = 0) {
    return (
      <div key={node.item.id} style={{ marginTop: depth === 0 ? 10 : 8 }}>
        <div style={{ marginLeft: depth * COMMENT_THREAD_INDENT }}>
          {renderConversationItem(node.item, depth)}
        </div>
        {node.children.map((child) => renderCommentNode(child, depth + 1))}
      </div>
    );
  }

  return (
    <div className="inbox-page-fullheight">
      <div className="inbox-header" style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 24 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <div
            style={{
              width: 36,
              height: 36,
              borderRadius: 10,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              background: "rgba(16,185,129,.10)",
              border: "1px solid rgba(16,185,129,.18)",
            }}
          >
            <InboxIcon style={{ width: 18, height: 18, color: "var(--daccent)" }} />
          </div>
          <div>
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <h1 className="dt-heading" style={{ margin: 0 }}>Inbox</h1>
              <span
                className="dt-mono"
                title="The inbox is still under active development — some flows may change or break as we harden the comment & DM pipelines."
                style={{
                  fontSize: 10,
                  textTransform: "uppercase",
                  letterSpacing: "0.08em",
                  padding: "3px 8px",
                  borderRadius: 999,
                  background: "var(--warning-soft)",
                  color: "var(--warning)",
                  border: "1px solid color-mix(in srgb, var(--warning) 35%, transparent)",
                  lineHeight: 1,
                }}
              >
                Beta
              </span>
            </div>
            <p className="dt-body-sm" style={{ margin: "4px 0 0", color: "var(--dmuted)" }}>
              {metaAccountCount + twitterAccounts.length} inbox account{metaAccountCount + twitterAccounts.length === 1 ? "" : "s"} · {counts.comments + counts.dms + counts.threads} unread
            </p>
          </div>
        </div>
        <div className="inbox-header-actions" style={{ display: "flex", gap: 8 }}>
          <button
            onClick={handleMarkAllRead}
            disabled={unreadCount === 0}
            className="dt-body-sm"
            style={{
              display: "flex",
              alignItems: "center",
              gap: 6,
              padding: "8px 12px",
              borderRadius: 8,
              border: "1px solid var(--dborder)",
              background: "transparent",
              color: unreadCount === 0 ? "var(--dmuted2)" : "var(--dtext)",
              cursor: unreadCount === 0 ? "not-allowed" : "pointer",
            }}
          >
            <CheckCheck style={{ width: 14, height: 14 }} />
            Mark all read
          </button>
          {metaAccountCount > 0 ? (
            <button
              onClick={handleSync}
              disabled={syncing}
              className="dt-body-sm"
              style={{
                display: "flex",
                alignItems: "center",
                gap: 6,
                padding: "8px 12px",
                borderRadius: 8,
                border: "none",
                background: "var(--daccent)",
                color: "var(--primary-foreground)",
                cursor: syncing ? "wait" : "pointer",
              }}
            >
              <RefreshCw style={{ width: 14, height: 14, animation: syncing ? "spin 1s linear infinite" : "none" }} />
              {syncing ? "Syncing..." : "Sync Meta"}
            </button>
          ) : null}
          {twitterAccounts.length > 0 ? (
            <button
              onClick={() => handleXSync()}
              disabled={xSyncing || xSyncPaidBlocked}
              className="dt-body-sm"
              style={{
                display: "flex",
                alignItems: "center",
                gap: 6,
                padding: "8px 12px",
                borderRadius: 8,
                border: "1px solid var(--dborder2)",
                background: "var(--dtext)",
                color: "var(--surface)",
                cursor: xSyncing ? "wait" : "pointer",
                opacity: xSyncPaidBlocked ? 0.55 : 1,
              }}
            >
              <RefreshCw style={{ width: 14, height: 14, animation: xSyncing ? "spin 1s linear infinite" : "none" }} />
              {xSyncing ? "Estimating..." : "Sync X"}
            </button>
          ) : null}
        </div>
      </div>

      <div style={{ display: "grid", gap: 10, marginBottom: 16 }}>
        {pageError ? (
          <SyncStateCard
            icon={<AlertTriangle style={{ width: 16, height: 16 }} />}
            title="Inbox could not be fully loaded"
            body={pageError}
            tone="error"
            onAction={() => load()}
            actionLabel="Try again"
          />
        ) : null}
        {xReconnectAccounts.length > 0 ? (
          <SyncStateCard
            icon={<ShieldAlert style={{ width: 16, height: 16 }} />}
            title="Reconnect X for Inbox permissions"
            body={`${xReconnectAccounts.length} X account${xReconnectAccounts.length === 1 ? "" : "s"} can keep publishing, but ${xDMsEnabled ? "comments or DMs need" : "comments need"} the latest X scopes. Reconnect to grant the missing permissions.`}
            tone="warn"
            actionHref={profileId ? `/projects/${profileId}/accounts` : undefined}
            actionLabel="Review X connection"
          />
        ) : null}
        {xCredentialAccounts.length > 0 ? (
          <SyncStateCard
            icon={<ShieldAlert style={{ width: 16, height: 16 }} />}
            title="Complete workspace X app credentials"
            body={`${xCredentialAccounts.length} workspace X app connection${xCredentialAccounts.length === 1 ? " is" : "s are"} missing the app Bearer Token, Consumer Secret, Client ID, or Client Secret required for Inbox delivery. Publishing credentials are unchanged.`}
            tone="warn"
            actionHref={profileId ? `/projects/${profileId}/credentials` : undefined}
            actionLabel="Open platform credentials"
          />
        ) : null}
        {xDeliveryErrorAccounts.length > 0 ? (
          <SyncStateCard
            icon={<AlertTriangle style={{ width: 16, height: 16 }} />}
            title="X Inbox delivery needs attention"
            body={xDMsEnabled
              ? "UniPost could not activate X Inbox delivery for at least one account. Review the connection and integration logs before retrying sync."
              : "UniPost could not activate the X comments stream for at least one account. Review the connection and integration logs before retrying sync."}
            tone="error"
            actionHref={profileId ? `/projects/${profileId}/logs` : undefined}
            actionLabel="Review logs"
          />
        ) : null}
        {xCapPaused ? (
          <SyncStateCard
            icon={<Archive style={{ width: 16, height: 16 }} />}
            title="X inbound daily cap reached"
            body={`Paid X reads are paused until ${xCredits?.inbound_daily_reset_at ? new Date(xCredits.inbound_daily_reset_at).toLocaleString() : "the next UTC reset"}. Adjust the cap in Billing if you want to resume sooner.`}
            tone="warn"
            actionHref="/settings/billing"
            actionLabel="Manage X inbound cap"
          />
        ) : null}
        {xAllowancePaused ? (
          <SyncStateCard
            icon={<Archive style={{ width: 16, height: 16 }} />}
            title="Monthly X Credits exhausted"
            body="Managed X Inbox reads are paused until the billing period resets. Workspace X app connections remain BYO-billed."
            tone="error"
            actionHref="/settings/billing"
            actionLabel="Review X Credits"
          />
        ) : null}
        {xSyncState.kind === "estimate" ? (
          <SyncStateCard
            icon={<PlatformIcon platform="twitter" size={16} />}
            title="Confirm X Inbox sync"
            body={xCreditsEnabled
              ? `This 7-day backfill can use up to ${xSyncState.result.estimated_x_credits.toLocaleString()} X Credits across ${xSyncState.result.accounts_checked} account(s). The final charge is based on the server-returned result.`
              : `This 7-day X comments backfill will check ${xSyncState.result.accounts_checked} account(s).`}
            tone="warn"
            onAction={() => handleXSync(xSyncState.result.confirmation_token)}
            actionLabel={xSyncing ? "Syncing..." : "Confirm and sync"}
            actionDisabled={xSyncing}
          />
        ) : xSyncState.kind === "pending" ? (
          <SyncStateCard
            icon={<RefreshCw style={{ width: 16, height: 16 }} />}
            title="X Inbox sync is already running"
            body={`Operation ${xSyncState.result.confirmation_operation_id || ""} is in progress. Its execution lease runs until ${xSyncState.result.execution_lease_expires_at ? new Date(xSyncState.result.execution_lease_expires_at).toLocaleString() : "the server completes it"}.`}
            onAction={() => handleXSync(xSyncState.confirmationToken)}
            actionLabel="Refresh status"
          />
        ) : xSyncState.kind === "complete" ? (
          <SyncStateCard
            icon={<CheckCheck style={{ width: 16, height: 16 }} />}
            title="X Inbox sync complete"
            body={`${(xSyncState.result.accepted ?? 0).toLocaleString()} added, ${(xSyncState.result.duplicates ?? 0).toLocaleString()} duplicates, ${(xSyncState.result.suppressed ?? 0).toLocaleString()} suppressed.${xCreditsEnabled ? ` Estimated ceiling: ${xSyncState.result.estimated_x_credits.toLocaleString()} X Credits.` : ""}`}
          />
        ) : xSyncState.kind === "error" ? (
          <SyncStateCard
            icon={<AlertTriangle style={{ width: 16, height: 16 }} />}
            title="X Inbox sync failed"
            body={xSyncState.message}
            tone="error"
            onAction={() => handleXSync()}
            actionLabel="Try again"
          />
        ) : null}
        {pendingXOutboundOperation ? (
          <SyncStateCard
            icon={pendingXOutboundManual
              ? <AlertTriangle style={{ width: 16, height: 16 }} />
              : <RefreshCw style={{ width: 16, height: 16 }} />}
            title={pendingXOutboundManual
              ? "X reply requires manual review"
              : "X reply reconciliation in progress"}
            body={pendingXOutboundManual
              ? "UniPost cannot safely determine whether X accepted this reply. Check the X conversation and logs before sending anything again."
              : `${workspaceXOutboundOperations.length} X repl${workspaceXOutboundOperations.length === 1 ? "y is" : "ies are"} unresolved. Retrying an unchanged message reuses its saved idempotency key; editing the message creates a new operation.`}
            tone={pendingXOutboundManual ? "error" : "warn"}
            onAction={!pendingXOutboundManual && pendingXOutboundOperation.operationId
              ? () => reconcileXOutboundOperation(pendingXOutboundOperation)
              : undefined}
            actionHref={pendingXOutboundManual && profileId
              ? `/projects/${profileId}/logs`
              : undefined}
            actionLabel={pendingXOutboundManual ? "Review logs" : "Check status"}
          />
        ) : null}
        {replyFeedback ? (
          <div role="status" aria-live="polite">
            <SyncStateCard
              icon={<Send style={{ width: 16, height: 16 }} />}
              title="Reply status"
              body={replyFeedback}
            />
          </div>
        ) : null}
        {reconnectAccounts.length > 0 ? (
          <SyncStateCard
            icon={<ShieldAlert style={{ width: 16, height: 16 }} />}
            title="Reconnect required"
            body={`${reconnectAccounts.length} Instagram / Threads account${reconnectAccounts.length > 1 ? "s need" : " needs"} reconnect before inbox sync can fully load comments or replies.`}
            tone="warn"
            actionHref={buildContactPageHref({
              topic: "inbox-reconnect-required",
              source: "inbox-sync-card",
              workspace: workspaceId || undefined,
            })}
          />
        ) : null}
        {missingPermissionErrors.length > 0 ? (
          <SyncStateCard
            icon={<AlertTriangle style={{ width: 16, height: 16 }} />}
            title="Missing permission or outdated scope"
            body="At least one Meta account could not load comments or replies because its token does not have the required scopes. Reconnect the account with the latest Instagram / Threads permissions."
            tone="error"
            actionHref={buildContactPageHref({
              topic: "inbox-permission-error",
              source: "inbox-sync-card",
              workspace: workspaceId || undefined,
            })}
          />
        ) : null}
        {syncFailures.length > 0 ? (
          <SyncStateCard
            icon={<Archive style={{ width: 16, height: 16 }} />}
            title="Sync completed with errors"
            body={`UniPost checked ${syncData?.accounts_checked || 0} account(s), but ${syncFailures.length} sync step(s) failed. Review API logs for the exact Meta response.`}
            tone="neutral"
            actionHref={buildContactPageHref({
              topic: "inbox-sync-failure",
              source: "inbox-sync-card",
              workspace: workspaceId || undefined,
            })}
          />
        ) : null}
      </div>

      <div className="inbox-toolbar" style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 16, gap: 12 }}>
        <div role="tablist" aria-label="Inbox sources" style={{ display: "flex", gap: 4, padding: 4, borderRadius: 10, border: "1px solid var(--dborder)", background: "var(--sidebar)" }}>
          {([
            { key: "comments", label: "Comments", count: counts.comments },
            { key: "dms", label: "DMs", count: counts.dms },
            { key: "threads", label: "Threads", count: counts.threads },
          ] as const).map((item) => {
            const active = tab === item.key;
            return (
              <button
                key={item.key}
                role="tab"
                aria-selected={active}
                onClick={() => {
                  setTab(item.key);
                  setMobileDetailOpen(false);
                }}
                className="dt-body-sm"
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 8,
                  padding: "8px 12px",
                  borderRadius: 8,
                  border: "none",
                  cursor: "pointer",
                  background: active ? "var(--accent-dim)" : "transparent",
                  color: active ? "var(--daccent)" : "var(--dmuted)",
                  fontWeight: active ? 600 : 500,
                }}
              >
                {item.label}
                {item.count > 0 ? (
                  // Dark emerald-700 — distinct from the active tab's
                  // pale-green selection fill (which uses --accent-dim,
                  // close to mint) and from the brighter brand --daccent.
                  // Matches the sidebar's Inbox badge so the same number
                  // reads identically across surfaces.
                  <span
                    className="dt-mono"
                    style={{
                      fontSize: 10,
                      padding: "2px 6px",
                      borderRadius: 999,
                      background: "#047857",
                      color: "white",
                      fontWeight: 700,
                    }}
                  >
                    {item.count}
                  </span>
                ) : null}
              </button>
            );
          })}
        </div>

        <div style={{ position: "relative", width: 280, maxWidth: "100%" }}>
          <Search
            style={{
              position: "absolute",
              left: 12,
              top: "50%",
              transform: "translateY(-50%)",
              width: 14,
              height: 14,
              color: "var(--dmuted2)",
            }}
          />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            aria-label="Search Inbox conversations"
            placeholder={tab === "dms" ? "Search contacts or messages..." : "Search posts, authors, or comments..."}
            className="dt-body-sm"
            style={{
              width: "100%",
              padding: "10px 12px 10px 36px",
              borderRadius: 10,
              border: "1px solid var(--dborder)",
              background: "var(--sidebar)",
              color: "var(--dtext)",
              outline: "none",
            }}
          />
        </div>
      </div>

      <div
        ref={containerRef}
        className="inbox-master-detail"
        data-detail-open={mobileDetailOpen ? "true" : "false"}
        style={{ display: "flex", flex: 1, minHeight: 0, border: "1px solid var(--dborder)", borderRadius: 14, overflow: "hidden" }}
      >
        <div className="inbox-list-pane" style={{ width: leftPaneWidth, minWidth: 240, maxWidth: 600, minHeight: 0, flexShrink: 0, background: "var(--sidebar)", overflowY: "auto" }}>
          {loading ? (
            <div aria-label="Loading Inbox" style={{ padding: 16, display: "grid", gap: 12 }}>
              {[0, 1, 2, 3].map((index) => (
                <div key={index} className="inbox-skeleton-row" aria-hidden="true">
                  <span />
                  <div><span /><span /></div>
                </div>
              ))}
            </div>
          ) : activeGroups.length === 0 ? (
            <div style={{ padding: 24 }}>
              <SyncStateCard
                icon={<InboxIcon style={{ width: 16, height: 16 }} />}
                title="No conversations yet"
                body={
                  reconnectAccounts.length > 0 || missingPermissionErrors.length > 0
                    ? "Fix the account state above, then sync again."
                    : "UniPost will show new Instagram and YouTube comments, Instagram DMs, and Threads replies here once they arrive."
                }
              />
            </div>
          ) : (
            activeGroups.map((group) => {
              const active = group.id === selectedGroupId;
              const status = group.threadStatus || "open";
              return (
                <button
                  key={group.id}
                  onClick={() => openGroup(group)}
                  style={{
                    width: "100%",
                    textAlign: "left",
                    padding: "14px 16px",
                    border: "none",
                    borderBottom: "1px solid var(--dborder)",
                    background: active ? "var(--accent-dim)" : "transparent",
                    borderLeft: active ? "2px solid var(--daccent)" : "2px solid transparent",
                    cursor: "pointer",
                    fontFamily: "inherit",
                  }}
                >
                  <div style={{ display: "flex", alignItems: "flex-start", gap: 10 }}>
                    <div style={{ paddingTop: 2 }}>
                      <PlatformIcon platform={group.accountPlatform || platformFromSource(group.source)} size={18} />
                    </div>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
                        <span className="dt-body-sm" style={{ fontWeight: 600, color: "var(--dtext)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                          {isDMSource(group.source) ? group.title : group.title || `@${group.accountName || "post"}`}
                        </span>
                        <span className="dt-mono" style={{ fontSize: 10, color: "var(--dmuted2)" }}>
                          {sourceLabel(group.source)}
                        </span>
                        {group.unreadCount > 0 ? (
                          <span
                            className="dt-mono"
                            style={{
                              marginLeft: "auto",
                              fontSize: 10,
                              borderRadius: 999,
                              padding: "2px 6px",
                              background: "#047857",
                              color: "white",
                              fontWeight: 700,
                            }}
                          >
                            {group.unreadCount}
                          </span>
                        ) : null}
                      </div>
                      <div className="dt-body-sm" style={{ color: "var(--dmuted)", marginBottom: 8, display: "-webkit-box", WebkitLineClamp: 2, WebkitBoxOrient: "vertical", overflow: "hidden" }}>
                        {group.subtitle}
                      </div>
                      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8 }}>
                        <StatusPill status={status} humanAgent={isDMSource(group.source)} />
                        <span className="dt-mono" style={{ fontSize: 10, color: "var(--dmuted2)" }}>
                          {timeAgo(group.latestActivityAt)}
                        </span>
                      </div>
                    </div>
                    <ChevronRight style={{ width: 14, height: 14, color: "var(--dmuted2)", flexShrink: 0, marginTop: 4 }} />
                  </div>
                </button>
              );
            })
          )}
        </div>

        {/* Drag handle */}
        <div
          className="inbox-resize-handle"
          role="separator"
          aria-label="Resize conversation list"
          aria-orientation="vertical"
          aria-valuemin={240}
          aria-valuemax={600}
          aria-valuenow={leftPaneWidth}
          tabIndex={0}
          onMouseDown={handleDragStart}
          onKeyDown={(event) => {
            if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
            event.preventDefault();
            setLeftPaneWidth((width) =>
              Math.min(600, Math.max(240, width + (event.key === "ArrowRight" ? 24 : -24))),
            );
          }}
          style={{
            width: 4,
            cursor: "col-resize",
            background: "var(--dborder)",
            flexShrink: 0,
            transition: "background 0.15s",
          }}
          onMouseEnter={(e) => { e.currentTarget.style.background = "var(--daccent)"; }}
          onMouseLeave={(e) => { e.currentTarget.style.background = "var(--dborder)"; }}
        />

        <section className="inbox-detail-pane" aria-label="Conversation detail" style={{
          display: "flex",
          flexDirection: "column",
          minWidth: 0,
          minHeight: 0,
          flex: 1,
          background: "var(--surface)",
          overflowY: selectedGroup && !isDMSource(selectedGroup.source) ? "auto" : "hidden",
        }}>
          {!selectedGroup ? (
            <div style={{ padding: 28 }}>
              <SyncStateCard
                icon={<MessageSquare style={{ width: 16, height: 16 }} />}
                title="Select a conversation"
                body="Pick a post group, DM thread, or Threads reply from the left to open the Inbox detail view."
              />
            </div>
          ) : (
            <>
              <div style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: 12,
                padding: "18px 20px",
                borderBottom: "1px solid var(--dborder)",
                background: "var(--surface2)",
                position: !isDMSource(selectedGroup.source) ? "sticky" : "relative",
                top: 0,
                zIndex: 2,
              }}>
                <div>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 6 }}>
                    <button
                      type="button"
                      className="inbox-mobile-back"
                      aria-label="Back to conversation list"
                      onClick={() => setMobileDetailOpen(false)}
                    >
                      <ArrowLeft style={{ width: 15, height: 15 }} />
                    </button>
                    {sourceIcon(selectedGroup.source)}
                    <h2 className="dt-body" style={{ margin: 0, fontWeight: 700, color: "var(--dtext)" }}>
                      {isDMSource(selectedGroup.source) ? selectedGroup.title : `${selectedGroup.items.length} ${sourceLabel(selectedGroup.source).toLowerCase()}${selectedGroup.items.length === 1 ? "" : "s"}`}
                    </h2>
                  </div>
                  <p className="dt-body-sm" style={{ margin: 0, color: "var(--dmuted)" }}>
                    {selectedGroup.accountName ? `@${selectedGroup.accountName}` : "Connected account"} · last activity {timeAgo(selectedGroup.latestActivityAt)}
                  </p>
                </div>
                <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap", justifyContent: "flex-end" }}>
                  <StatusPill status={detailStatus} humanAgent={showHumanAgent} />
                  {selectedGroup.assignedTo ? (
                    <span className="dt-body-sm" style={{ color: "var(--dmuted)" }}>
                      Assigned to {selectedGroup.assignedTo}
                    </span>
                  ) : null}
                  {detailStatus !== "assigned" ? (
                    <button
                      onClick={() => handleSetThreadState(selectedGroup, "assigned")}
                      className="dt-body-sm"
                      style={{
                        padding: "8px 10px",
                        borderRadius: 8,
                        border: "1px solid var(--dborder)",
                        background: "transparent",
                        color: "var(--dtext)",
                        cursor: "pointer",
                      }}
                    >
                      Assign
                    </button>
                  ) : null}
                  <button
                    onClick={() => handleSetThreadState(selectedGroup, detailStatus === "resolved" ? "open" : "resolved")}
                    className="dt-body-sm"
                    style={{
                      padding: "8px 10px",
                      borderRadius: 8,
                      border: "1px solid var(--dborder)",
                      background: "transparent",
                      color: "var(--dtext)",
                      cursor: "pointer",
                    }}
                  >
                    {detailStatus === "resolved" ? "Re-open" : "Resolve"}
                  </button>
                </div>
              </div>

              {selectedGroup.source === "x_reply" ? (
                <div className="x-inbox-context">
                  <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    <PlatformIcon platform="twitter" size={18} />
                    <span className="dt-body-sm" style={{ fontWeight: 650, color: "var(--dtext)" }}>
                      X conversation
                    </span>
                  </div>
                  <p className="dt-body-sm" style={{ margin: 0, color: "var(--dmuted)", lineHeight: 1.6 }}>
                    Replies are grouped by X conversation ID {selectedGroup.threadKey}. Open any available permalink to review the public context on X.
                  </p>
                  {selectedGroup.items.find((item) => item.url)?.url ? (
                    <a
                      href={selectedGroup.items.find((item) => item.url)?.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="dt-body-sm"
                      style={{ color: "var(--daccent)", display: "inline-flex", alignItems: "center", gap: 6 }}
                    >
                      View conversation on X
                      <ChevronRight style={{ width: 13, height: 13 }} />
                    </a>
                  ) : null}
                </div>
              ) : !isDMSource(selectedGroup.source) ? (
                <div style={{ margin: "20px 20px 0", padding: 16, borderRadius: 12, border: "1px solid var(--dborder)", background: "var(--surface2)", position: "relative", overflow: "hidden", boxShadow: "0 10px 24px rgb(15 23 42 / 0.04)" }}>
                  <div style={{ position: "absolute", top: 0, left: 0, right: 0, height: 1, background: "linear-gradient(90deg, transparent, rgba(16,185,129,.45), transparent)" }} />
                  <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 8 }}>
                    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                      <PlatformIcon platform={selectedGroup.accountPlatform || platformFromSource(selectedGroup.source)} size={18} />
                      <span className="dt-body-sm" style={{ fontWeight: 600, color: "var(--dtext)" }}>
                        {selectedPost || currentMediaContext ? "Original post" : selectedGroup.accountName ? `@${selectedGroup.accountName}` : "Post context"}
                      </span>
                    </div>
                    {selectedPost?.published_at || selectedPost?.created_at ? (
                      <span className="dt-mono" style={{ fontSize: 10, color: "var(--dmuted2)" }}>
                        {selectedPost.published_at ? `published ${timeAgo(selectedPost.published_at)}` : `created ${timeAgo(selectedPost.created_at)}`}
                      </span>
                    ) : selectedGroup.parentExternalID ? (
                      <span className="dt-mono" style={{ fontSize: 10, color: "var(--dmuted2)" }}>
                        post id {selectedGroup.parentExternalID}
                      </span>
                    ) : null}
                  </div>
                  {selectedPost ? (
                    <div style={{ display: "grid", gap: 12 }}>
                      {selectedPost.media_urls && selectedPost.media_urls.length > 0 ? (
                        <div
                          style={{
                            width: 220,
                            maxWidth: "100%",
                            aspectRatio: "1 / 1",
                            borderRadius: 12,
                            overflow: "hidden",
                            border: "1px solid var(--dborder)",
                            background: "var(--surface3)",
                          }}
                        >
                          <img
                            src={selectedPost.media_urls[0]}
                            alt="Post media"
                            style={{ width: "100%", height: "100%", objectFit: "cover" }}
                          />
                        </div>
                      ) : currentMediaContext?.media_url ? (
                        <div
                          style={{
                            width: 220,
                            maxWidth: "100%",
                            aspectRatio: "1 / 1",
                            borderRadius: 12,
                            overflow: "hidden",
                            border: "1px solid var(--dborder)",
                            background: "var(--surface3)",
                          }}
                        >
                          <img
                            src={currentMediaContext.media_url}
                            alt="Post media"
                            style={{ width: "100%", height: "100%", objectFit: "cover" }}
                          />
                        </div>
                      ) : null}
                      <p className="dt-body-sm" style={{ margin: 0, color: "var(--dtext)", whiteSpace: "pre-wrap", lineHeight: 1.65 }}>
                        {selectedPost.caption || "(no caption)"}
                      </p>
                      <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
                        <span className="dt-mono" style={{ fontSize: 10, color: "var(--dmuted2)" }}>
                          {selectedPost.status}
                        </span>
                        <span className="dt-mono" style={{ fontSize: 10, color: "var(--dmuted2)" }}>
                          {(selectedPost.results || []).length} publish result{(selectedPost.results || []).length === 1 ? "" : "s"}
                        </span>
                        {selectedGroup.parentExternalID ? (
                          <span className="dt-mono" style={{ fontSize: 10, color: "var(--dmuted2)" }}>
                            external {selectedGroup.parentExternalID}
                          </span>
                        ) : null}
                      </div>
                    </div>
                  ) : currentMediaContext ? (
                    <div style={{ display: "grid", gap: 12 }}>
                      {currentMediaContext.media_url ? (
                        <div
                          style={{
                            width: 220,
                            maxWidth: "100%",
                            aspectRatio: "1 / 1",
                            borderRadius: 12,
                            overflow: "hidden",
                            border: "1px solid var(--dborder)",
                            background: "var(--surface3)",
                          }}
                        >
                          <img
                            src={currentMediaContext.media_url}
                            alt="Post media"
                            style={{ width: "100%", height: "100%", objectFit: "cover" }}
                          />
                        </div>
                      ) : null}
                      <p className="dt-body-sm" style={{ margin: 0, color: "var(--dtext)", whiteSpace: "pre-wrap", lineHeight: 1.65 }}>
                        {currentMediaContext.caption || "(no caption)"}
                      </p>
                      {currentMediaContext.permalink ? (
                        <a href={currentMediaContext.permalink} target="_blank" rel="noopener noreferrer"
                          className="dt-mono" style={{ fontSize: 10, color: "var(--daccent)" }}>
                          View on Instagram
                        </a>
                      ) : null}
                    </div>
                  ) : (
                    <p className="dt-body-sm" style={{ margin: 0, color: "var(--dmuted)" }}>
                      Loading post preview...
                    </p>
                  )}
                </div>
              ) : null}

              <div style={{
                padding: isDMSource(selectedGroup.source) ? "16px 14px" : "16px 20px 28px",
                display: "flex", flexDirection: "column",
                gap: 0,
                ...(isDMSource(selectedGroup.source)
                  ? { flex: 1, minHeight: 0, overflowY: "auto", justifyContent: "flex-end" }
                  : { flex: "0 0 auto", minHeight: "auto", overflow: "visible" }),
              }}>
                {isDMSource(selectedGroup.source)
                  ? selectedGroup.items.map((item) => renderConversationItem(item))
                  : (
                    <CommentThread tree={commentTree} render={renderCommentNode} />
                  )}
              </div>

              {isDMSource(selectedGroup.source) ? (() => {
                // Meta enforces the standard 24-hour reply window for
                // both Instagram and Facebook DMs. Keep the dashboard
                // aligned with the backend guard so users see the
                // actionable state before attempting a send.
                const lastInbound = [...selectedGroup.items].reverse().find((i) => !i.is_own);
                const windowClosed = lastInbound
                  ? isMetaDMReplyWindowClosed(selectedGroup.source, lastInbound.received_at)
                  : false;
                const draftReady = (replyDrafts["__dm_bottom__"] || "").trim().length > 0;
                const sendAllowed = !windowClosed && draftReady && !!lastInbound;
                return (
                  <div style={{ borderTop: "1px solid var(--dborder)", background: "var(--surface2)" }}>
                    {windowClosed ? (
                      <div style={{
                        padding: "8px 14px",
                        background: "color-mix(in srgb, var(--warning, #f59e0b) 12%, var(--surface2))",
                        borderBottom: "1px solid color-mix(in srgb, var(--warning, #f59e0b) 30%, transparent)",
                        fontSize: 12,
                        lineHeight: 1.45,
                        color: "var(--dtext)",
                      }}>
                        Reply window closed — this person last messaged over 24 hours ago. They need to message this account again before you can reply.
                      </div>
                    ) : null}
                    <div style={{ padding: "10px 14px", display: "flex", alignItems: "center", gap: 8 }}>
                      <input
                        value={replyDrafts["__dm_bottom__"] || ""}
                        onChange={(e) => setReplyDrafts((prev) => ({ ...prev, ["__dm_bottom__"]: e.target.value }))}
                        aria-label={selectedGroup.source === "x_dm" ? "Write an X direct message" : "Write a direct message"}
                        placeholder={windowClosed ? "24h reply window is closed" : "Message..."}
                        disabled={windowClosed}
                        onKeyDown={(e) => {
                          if (e.key === "Enter" && !e.shiftKey) {
                            e.preventDefault();
                            if (sendAllowed && lastInbound) {
                              handleReply(selectedGroup, lastInbound);
                            }
                          }
                        }}
                        className="dt-body-sm"
                        style={{
                          flex: 1,
                          padding: "10px 16px",
                          borderRadius: 999,
                          border: "1px solid var(--dborder)",
                          background: "var(--surface)",
                          color: "var(--dtext)",
                          outline: "none",
                          fontSize: 13,
                          opacity: windowClosed ? 0.55 : 1,
                          cursor: windowClosed ? "not-allowed" : "text",
                        }}
                      />
                      <button
                        aria-label={windowClosed ? "Cannot send: reply window closed" : "Send direct message"}
                        onClick={() => {
                          if (sendAllowed && lastInbound) {
                            handleReply(selectedGroup, lastInbound);
                          }
                        }}
                        disabled={replyingGroupId === selectedGroup.id || !sendAllowed}
                        title={windowClosed ? "Reply window closed" : undefined}
                        style={{
                          width: 36,
                          height: 36,
                          borderRadius: 999,
                          border: "none",
                          background: sendAllowed ? "var(--dtext)" : "var(--surface3)",
                          color: sendAllowed ? "var(--surface)" : "var(--dmuted2)",
                          boxShadow: sendAllowed ? "0 10px 24px rgb(15 23 42 / 0.12)" : "none",
                          cursor: sendAllowed ? "pointer" : "not-allowed",
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "center",
                          flexShrink: 0,
                          transition: "background .15s, color .15s",
                        }}
                      >
                        <Send style={{ width: 16, height: 16 }} />
                      </button>
                    </div>
                  </div>
                );
              })() : null}
            </>
          )}
        </section>
      </div>

      <style>{`
        @keyframes spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }

        .dm-message .dm-actions {
          opacity: 0;
          pointer-events: none;
          transition: opacity .15s ease;
        }

        .dm-message:hover .dm-actions {
          opacity: 1;
          pointer-events: auto;
        }
      `}</style>
    </div>
  );
}
