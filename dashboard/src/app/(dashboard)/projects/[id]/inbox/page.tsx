"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
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
  listSocialPosts,
  type InboxItem,
  type SocialAccount,
  type SocialPost,
} from "@/lib/api";
import { useWorkspaceId } from "@/lib/use-workspace-id";
import { PlatformIcon } from "@/components/platform-icons";
import {
  AlertTriangle,
  Archive,
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
  UserRound,
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

type ConversationGroup = {
  id: string;
  threadKey: string;
  source: "ig_comment" | "ig_dm" | "threads_reply";
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
  switch (source) {
    case "ig_comment":
      return "Comment";
    case "ig_dm":
      return "DM";
    case "threads_reply":
      return "Reply";
    default:
      return source;
  }
}

function sourceIcon(source: InboxItem["source"]) {
  switch (source) {
    case "ig_comment":
      return <MessageCircle style={{ width: 14, height: 14 }} />;
    case "ig_dm":
      return <Mail style={{ width: 14, height: 14 }} />;
    case "threads_reply":
      return <AtSign style={{ width: 14, height: 14 }} />;
    default:
      return <MessageSquare style={{ width: 14, height: 14 }} />;
  }
}

function byNewestActivity(a: ConversationGroup, b: ConversationGroup) {
  const priority = (status: ThreadStatus, unread: number) => {
    if (unread > 0) return 0;
    if (status === "open") return 1;
    if (status === "assigned") return 2;
    return 3;
  };
  const pa = priority(a.threadStatus, a.unreadCount);
  const pb = priority(b.threadStatus, b.unreadCount);
  if (pa !== pb) return pa - pb;
  return new Date(b.latestActivityAt).getTime() - new Date(a.latestActivityAt).getTime();
}

function groupItems(items: InboxItem[], source: ConversationGroup["source"]): ConversationGroup[] {
  const filtered = items.filter((item) => item.source === source);
  const map = new Map<string, InboxItem[]>();

  for (const item of filtered) {
    const key = `${item.social_account_id}:${source}:${item.thread_key || item.parent_external_id || item.author_id || item.external_id}`;
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
    const unreadCount = sorted.filter((item) => !item.is_read && !item.is_own).length;
    const title =
      source === "ig_dm"
        ? `@${firstInbound.author_name || firstInbound.author_id || "unknown"}`
        : firstInbound.body || "(no text)";
    const subtitle =
      source === "ig_dm"
        ? (latest.body || "(no text)")
        : `${sourceLabel(source)}s on @${latest.account_name || "account"}`;

    return {
      id: key,
      threadKey: latest.thread_key || key,
      source,
      title,
      subtitle,
      items: sorted,
      accountName: latest.account_name || undefined,
      accountPlatform: latest.account_platform || undefined,
      latestActivityAt: latest.received_at,
      unreadCount,
      parentExternalID: latest.parent_external_id,
      threadStatus: latest.thread_status || "open",
      assignedTo: latest.assigned_to,
      linkedPostID: latest.linked_post_id,
    };
  });
}

function buildCommentTree(items: InboxItem[], threadKey: string): CommentNode[] {
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

    if (parentNode && parentID !== threadKey) {
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

function StatusPill({ status, humanAgent = false }: { status: ThreadStatus; humanAgent?: boolean }) {
  const colors =
    status === "resolved"
      ? { bg: "rgba(255,255,255,.05)", border: "rgba(255,255,255,.08)", color: "var(--dmuted)" }
      : status === "assigned"
        ? { bg: "rgba(59,130,246,.10)", border: "rgba(59,130,246,.24)", color: "#93c5fd" }
        : { bg: "rgba(16,185,129,.10)", border: "rgba(16,185,129,.24)", color: "var(--daccent)" };

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
            background: "rgba(245,158,11,.10)",
            color: "#fbbf24",
            border: "1px solid rgba(245,158,11,.22)",
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
}: {
  icon: React.ReactNode;
  title: string;
  body: string;
  tone?: "neutral" | "warn" | "error";
}) {
  const styles =
    tone === "error"
      ? { bg: "rgba(239,68,68,.08)", border: "rgba(239,68,68,.18)", color: "#fca5a5" }
      : tone === "warn"
        ? { bg: "rgba(245,158,11,.08)", border: "rgba(245,158,11,.18)", color: "#fcd34d" }
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
      </div>
    </div>
  );
}

export default function InboxPage() {
  const params = useParams<{ id: string }>();
  const profileId = Array.isArray(params?.id) ? params.id[0] : params?.id;
  const { getToken } = useAuth();
  const workspaceId = useWorkspaceId();

  const [items, setItems] = useState<InboxItem[]>([]);
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [socialPosts, setSocialPosts] = useState<SocialPost[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [tab, setTab] = useState<FilterTab>("comments");
  const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null);
  const [replyDrafts, setReplyDrafts] = useState<Record<string, string>>({});
  const [replyingGroupId, setReplyingGroupId] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [syncData, setSyncData] = useState<SyncResponse | null>(null);

  const load = useCallback(async () => {
    if (!workspaceId) return;
    try {
      const token = await getToken();
      if (!token) return;
      const [itemsRes, unreadRes, socialPostsRes] = await Promise.all([
        listInboxItems(token, workspaceId),
        getInboxUnreadCount(token, workspaceId),
        listSocialPosts(token, workspaceId),
      ]);
      const accountsRes = profileId ? await listSocialAccounts(token, profileId) : null;
      setItems(itemsRes.data || []);
      setUnreadCount(unreadRes.data.count);
      setSocialPosts(socialPostsRes.data || []);
      if (accountsRes?.data) setAccounts(accountsRes.data);
    } finally {
      setLoading(false);
    }
  }, [workspaceId, getToken, profileId]);

  useEffect(() => {
    setLoading(true);
    load();
  }, [load]);

  const commentsGroups = useMemo(() => groupItems(items, "ig_comment"), [items]);
  const dmGroups = useMemo(() => groupItems(items, "ig_dm"), [items]);
  const threadsGroups = useMemo(() => groupItems(items, "threads_reply"), [items]);

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

  const reconnectAccounts = accounts.filter(
    (account) =>
      account.status === "reconnect_required" &&
      (account.platform === "instagram" || account.platform === "threads")
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

  async function handleSync() {
    if (!workspaceId) return;
    setSyncing(true);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await syncInbox(token, workspaceId);
      setSyncData((res.data as SyncResponse) || null);
      await load();
    } finally {
      setSyncing(false);
    }
  }

  async function handleMarkAllRead() {
    if (!workspaceId) return;
    const token = await getToken();
    if (!token) return;
    await markAllInboxRead(token, workspaceId);
    setItems((prev) => prev.map((item) => ({ ...item, is_read: true })));
    setUnreadCount(0);
  }

  async function openGroup(group: ConversationGroup) {
    setSelectedGroupId(group.id);
    const unreadInbound = group.items.filter((item) => !item.is_read && !item.is_own);
    if (!workspaceId || unreadInbound.length === 0) return;
    const token = await getToken();
    if (!token) return;

    await Promise.all(
      unreadInbound.map((item) => markInboxItemRead(token, workspaceId, item.id).catch(() => undefined))
    );
    setItems((prev) =>
      prev.map((item) =>
        unreadInbound.some((candidate) => candidate.id === item.id)
          ? { ...item, is_read: true }
          : item
      )
    );
    setUnreadCount((count) => Math.max(0, count - unreadInbound.length));
  }

  async function handleReply(group: ConversationGroup, targetItem: InboxItem) {
    if (!workspaceId) return;
    const draft = (replyDrafts[targetItem.id] || "").trim();
    if (!draft) return;

    setReplyingGroupId(group.id);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await replyToInboxItem(token, workspaceId, targetItem.id, draft);
      if (res.data) {
        setItems((prev) => [...prev, res.data]);
      }
      setReplyDrafts((prev) => ({ ...prev, [targetItem.id]: "" }));
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

    await updateInboxThreadState(token, workspaceId, group.items[0].id, {
      thread_status: threadStatus,
      assigned_to: assignedTo,
    });

    setItems((prev) =>
      prev.map((item) =>
        item.social_account_id === group.items[0].social_account_id &&
        item.source === group.source &&
        item.thread_key === group.threadKey
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
    const linkedPostID = selectedGroup.linkedPostID || selectedGroup.items.find((item) => item.linked_post_id)?.linked_post_id;
    if (linkedPostID) {
      const post = socialPosts.find((candidate) => candidate.id === linkedPostID);
      if (post) return post;
    }

    if (selectedGroup.parentExternalID) {
      return (
        socialPosts.find((candidate) =>
          (candidate.results || []).some((result) => result.external_id === selectedGroup.parentExternalID)
        ) || null
      );
    }

    return null;
  }, [selectedGroup, socialPosts]);

  const detailStatus = selectedGroup ? selectedGroup.threadStatus || "open" : "open";
  const showHumanAgent = selectedGroup?.source === "ig_dm";
  const commentTree = useMemo(
    () =>
      selectedGroup && selectedGroup.source !== "ig_dm"
        ? buildCommentTree(selectedGroup.items, selectedGroup.threadKey)
        : [],
    [selectedGroup]
  );

  function renderConversationItem(item: InboxItem, depth = 0) {
    if (!selectedGroup) return null;
    const draft = replyDrafts[item.id] || "";

    return (
      <div key={item.id} style={{ display: "grid", gap: 10, marginLeft: depth * 28 }}>
        <div
          style={{
            padding: 14,
            borderRadius: 14,
            border: item.is_own ? "1px solid rgba(16,185,129,.18)" : "1px solid var(--dborder)",
            background: item.is_own ? "rgba(16,185,129,.08)" : "rgba(255,255,255,.02)",
          }}
        >
          <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
            {item.is_own ? <UserRound style={{ width: 14, height: 14, color: "var(--daccent)" }} /> : null}
            <span className="dt-body-sm" style={{ fontWeight: 600, color: item.is_own ? "var(--daccent)" : "var(--dtext)" }}>
              {item.is_own ? "You" : `@${item.author_name || item.author_id || "unknown"}`}
            </span>
            {item.is_own ? (
              <span className="dt-mono" style={{ fontSize: 10, padding: "2px 6px", borderRadius: 999, background: "rgba(16,185,129,.15)", color: "var(--daccent)" }}>
                you
              </span>
            ) : null}
            {depth > 0 ? (
              <span className="dt-mono" style={{ fontSize: 10, color: "var(--dmuted2)" }}>
                reply
              </span>
            ) : null}
            <span className="dt-mono" style={{ fontSize: 10, color: "var(--dmuted2)", marginLeft: "auto" }}>
              {timeAgo(item.received_at)}
            </span>
          </div>
          <div className="dt-body-sm" style={{ color: "var(--dtext)", whiteSpace: "pre-wrap", lineHeight: 1.65 }}>
            {item.body || "(no text)"}
          </div>

          {!item.is_own ? (
            <div style={{ marginTop: 12 }}>
              <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 8 }}>
                <button
                  onClick={() => setReplyDrafts((prev) => ({ ...prev, [item.id]: prev[item.id] || "" }))}
                  className="dt-body-sm"
                  style={{ border: "none", background: "transparent", color: "var(--daccent)", cursor: "pointer", padding: 0 }}
                >
                  Reply
                </button>
                <button
                  onClick={() =>
                    setItems((prev) => prev.map((candidate) => candidate.id === item.id ? { ...candidate, is_read: !candidate.is_read } : candidate))
                  }
                  className="dt-body-sm"
                  style={{ border: "none", background: "transparent", color: "var(--dmuted)", cursor: "pointer", padding: 0 }}
                >
                  {item.is_read ? "Mark unread" : "Mark read"}
                </button>
              </div>

              <div style={{ display: "flex", gap: 8 }}>
                <input
                  value={draft}
                  onChange={(e) => setReplyDrafts((prev) => ({ ...prev, [item.id]: e.target.value }))}
                  placeholder={`Reply to @${item.author_name || item.author_id || "this user"}...`}
                  className="dt-body-sm"
                  style={{
                    flex: 1,
                    padding: "10px 12px",
                    borderRadius: 10,
                    border: "1px solid var(--dborder)",
                    background: "var(--sidebar)",
                    color: "var(--dtext)",
                    outline: "none",
                  }}
                />
                <button
                  onClick={() => handleReply(selectedGroup, item)}
                  disabled={replyingGroupId === selectedGroup.id || !draft.trim()}
                  className="dt-body-sm"
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 6,
                    padding: "10px 14px",
                    borderRadius: 10,
                    border: "none",
                    background: "var(--daccent)",
                    color: "var(--primary-foreground)",
                    cursor: replyingGroupId === selectedGroup.id || !draft.trim() ? "not-allowed" : "pointer",
                    opacity: replyingGroupId === selectedGroup.id || !draft.trim() ? 0.5 : 1,
                  }}
                >
                  <Send style={{ width: 14, height: 14 }} />
                  {replyingGroupId === selectedGroup.id ? "Sending..." : "Send"}
                </button>
              </div>
            </div>
          ) : null}
        </div>
      </div>
    );
  }

  return (
    <div>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 24 }}>
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
            <h1 className="dt-heading" style={{ margin: 0 }}>Inbox</h1>
            <p className="dt-body-sm" style={{ margin: "4px 0 0", color: "var(--dmuted)" }}>
              {accounts.filter((a) => a.platform === "instagram" || a.platform === "threads").length} Meta accounts · {unreadCount} unread
            </p>
          </div>
        </div>
        <div style={{ display: "flex", gap: 8 }}>
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
            {syncing ? "Syncing..." : "Sync"}
          </button>
        </div>
      </div>

      <div style={{ display: "grid", gap: 10, marginBottom: 16 }}>
        {reconnectAccounts.length > 0 ? (
          <SyncStateCard
            icon={<ShieldAlert style={{ width: 16, height: 16 }} />}
            title="Reconnect required"
            body={`${reconnectAccounts.length} Instagram / Threads account${reconnectAccounts.length > 1 ? "s need" : " needs"} reconnect before inbox sync can fully load comments or replies.`}
            tone="warn"
          />
        ) : null}
        {missingPermissionErrors.length > 0 ? (
          <SyncStateCard
            icon={<AlertTriangle style={{ width: 16, height: 16 }} />}
            title="Missing permission or outdated scope"
            body="At least one Meta account could not load comments or replies because its token does not have the required scopes. Reconnect the account with the latest Instagram / Threads permissions."
            tone="error"
          />
        ) : null}
        {syncFailures.length > 0 ? (
          <SyncStateCard
            icon={<Archive style={{ width: 16, height: 16 }} />}
            title="Sync completed with errors"
            body={`UniPost checked ${syncData?.accounts_checked || 0} account(s), but ${syncFailures.length} sync step(s) failed. Review API logs for the exact Meta response.`}
            tone="neutral"
          />
        ) : null}
      </div>

      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 16, gap: 12 }}>
        <div style={{ display: "flex", gap: 4, padding: 4, borderRadius: 10, border: "1px solid var(--dborder)", background: "var(--sidebar)" }}>
          {([
            { key: "comments", label: "Comments", count: counts.comments },
            { key: "dms", label: "DMs", count: counts.dms },
            { key: "threads", label: "Threads", count: counts.threads },
          ] as const).map((item) => {
            const active = tab === item.key;
            return (
              <button
                key={item.key}
                onClick={() => setTab(item.key)}
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
                  <span
                    className="dt-mono"
                    style={{
                      fontSize: 10,
                      padding: "2px 6px",
                      borderRadius: 999,
                      background: active ? "rgba(16,185,129,.16)" : "rgba(255,255,255,.06)",
                      color: active ? "var(--daccent)" : "var(--dmuted2)",
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

      <div style={{ display: "grid", gridTemplateColumns: "360px minmax(0, 1fr)", gap: 0, border: "1px solid var(--dborder)", borderRadius: 14, overflow: "hidden", minHeight: 620 }}>
        <aside style={{ borderRight: "1px solid var(--dborder)", background: "var(--sidebar)" }}>
          {loading ? (
            <div style={{ padding: 24, color: "var(--dmuted)" }}>Loading inbox...</div>
          ) : activeGroups.length === 0 ? (
            <div style={{ padding: 24 }}>
              <SyncStateCard
                icon={<InboxIcon style={{ width: 16, height: 16 }} />}
                title="No conversations yet"
                body={
                  reconnectAccounts.length > 0 || missingPermissionErrors.length > 0
                    ? "Fix the account state above, then sync again."
                    : "UniPost will show new Instagram comments, DMs, and Threads replies here once they arrive."
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
                      <PlatformIcon platform={group.accountPlatform || (group.source === "threads_reply" ? "threads" : "instagram")} size={18} />
                    </div>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
                        <span className="dt-body-sm" style={{ fontWeight: 600, color: "var(--dtext)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                          {group.source === "ig_dm" ? group.title : group.accountName ? `@${group.accountName}` : group.title}
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
                              background: "var(--daccent)",
                              color: "var(--primary-foreground)",
                            }}
                          >
                            {group.unreadCount}
                          </span>
                        ) : null}
                      </div>
                      <div className="dt-body-sm" style={{ color: "var(--dmuted)", marginBottom: 8, display: "-webkit-box", WebkitLineClamp: 2, WebkitBoxOrient: "vertical", overflow: "hidden" }}>
                        {group.source === "ig_dm" ? group.subtitle : group.title}
                      </div>
                      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8 }}>
                        <StatusPill status={status} humanAgent={group.source === "ig_dm"} />
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
        </aside>

        <section style={{ display: "flex", flexDirection: "column", minWidth: 0, background: "var(--surface)" }}>
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
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, padding: "18px 20px", borderBottom: "1px solid var(--dborder)", background: "rgba(0,0,0,.15)" }}>
                <div>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 6 }}>
                    {sourceIcon(selectedGroup.source)}
                    <h2 className="dt-body" style={{ margin: 0, fontWeight: 700, color: "var(--dtext)" }}>
                      {selectedGroup.source === "ig_dm" ? selectedGroup.title : `${selectedGroup.items.filter((item) => !item.is_own).length} ${sourceLabel(selectedGroup.source).toLowerCase()}${selectedGroup.items.filter((item) => !item.is_own).length === 1 ? "" : "s"}`}
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

              {selectedGroup.source !== "ig_dm" ? (
                <div style={{ margin: "20px 20px 0", padding: 16, borderRadius: 12, border: "1px solid var(--dborder)", background: "rgba(255,255,255,.02)", position: "relative", overflow: "hidden" }}>
                  <div style={{ position: "absolute", top: 0, left: 0, right: 0, height: 1, background: "linear-gradient(90deg, transparent, rgba(16,185,129,.45), transparent)" }} />
                  <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 8 }}>
                    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                      <PlatformIcon platform={selectedGroup.accountPlatform || (selectedGroup.source === "threads_reply" ? "threads" : "instagram")} size={18} />
                      <span className="dt-body-sm" style={{ fontWeight: 600, color: "var(--dtext)" }}>
                        {selectedPost ? "Original post" : selectedGroup.accountName ? `@${selectedGroup.accountName}` : "Post context"}
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
                    <div style={{ display: "grid", gap: 10 }}>
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
                  ) : (
                    <p className="dt-body-sm" style={{ margin: 0, color: "var(--dmuted)" }}>
                      UniPost could not map this conversation to a stored post yet. The inbox item is real, but the original post preview is unavailable until the comment or reply links back to a UniPost post result.
                    </p>
                  )}
                </div>
              ) : null}

              <div style={{ flex: 1, overflowY: "auto", padding: 20, display: "grid", gap: 14 }}>
                {selectedGroup.source === "ig_dm"
                  ? selectedGroup.items.map((item) => renderConversationItem(item))
                  : commentTree.map(function renderNode(node, depth = 0) {
                      return (
                        <div key={node.item.id} style={{ display: "grid", gap: 10 }}>
                          {renderConversationItem(node.item, depth)}
                          {node.children.length > 0 ? (
                            <div style={{ display: "grid", gap: 10, paddingLeft: 18, borderLeft: "1px solid rgba(255,255,255,.08)" }}>
                              {node.children.map((child) => renderNode(child, depth + 1))}
                            </div>
                          ) : null}
                        </div>
                      );
                    })}
              </div>

              {selectedGroup.source === "ig_dm" ? (
                <div style={{ borderTop: "1px solid var(--dborder)", padding: 16, background: "rgba(0,0,0,.12)" }}>
                  <div className="dt-body-sm" style={{ color: "var(--dmuted2)" }}>
                    Human-agent workflow: DMs should visibly support assigned, open, and resolved states during the Meta review demo.
                  </div>
                </div>
              ) : null}
            </>
          )}
        </section>
      </div>

      <style>{`
        @keyframes spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
}
