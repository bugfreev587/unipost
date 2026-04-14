"use client";

import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { useWorkspaceId } from "@/lib/use-workspace-id";
import {
  listInboxItems,
  getInboxUnreadCount,
  markInboxItemRead,
  markAllInboxRead,
  replyToInboxItem,
  syncInbox,
  type InboxItem,
} from "@/lib/api";
import { PlatformIcon } from "@/components/platform-icons";
import {
  MessageSquare,
  RefreshCw,
  CheckCheck,
  Send,
  Mail,
  MessageCircle,
  AtSign,
} from "lucide-react";

const SOURCE_LABELS: Record<string, string> = {
  ig_comment: "Comment",
  ig_dm: "DM",
  threads_reply: "Reply",
};

const SOURCE_PLATFORM: Record<string, string> = {
  ig_comment: "instagram",
  ig_dm: "instagram",
  threads_reply: "threads",
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

function SourceIcon({ source }: { source: string }) {
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

type FilterSource = "" | "ig_comment" | "ig_dm" | "threads_reply";

export default function InboxPage() {
  const { getToken } = useAuth();
  const workspaceId = useWorkspaceId();
  const [items, setItems] = useState<InboxItem[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [filter, setFilter] = useState<FilterSource>("");
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [replyText, setReplyText] = useState("");
  const [replying, setReplying] = useState(false);

  const load = useCallback(async () => {
    if (!workspaceId) return;
    try {
      const token = await getToken();
      if (!token) return;
      const filters: { source?: string } = {};
      if (filter) filters.source = filter;
      const [itemsRes, countRes] = await Promise.all([
        listInboxItems(token, workspaceId, filters),
        getInboxUnreadCount(token, workspaceId),
      ]);
      setItems(itemsRes.data || []);
      setUnreadCount(countRes.data.count);
    } catch {
      /* silent */
    } finally {
      setLoading(false);
    }
  }, [workspaceId, getToken, filter]);

  useEffect(() => {
    setLoading(true);
    load();
  }, [load]);

  async function handleSync() {
    if (!workspaceId) return;
    setSyncing(true);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await syncInbox(token, workspaceId);
      if (res.data.new_items > 0) {
        await load();
      }
    } catch {
      /* silent */
    } finally {
      setSyncing(false);
    }
  }

  async function handleMarkAllRead() {
    if (!workspaceId) return;
    try {
      const token = await getToken();
      if (!token) return;
      await markAllInboxRead(token, workspaceId);
      setItems((prev) => prev.map((i) => ({ ...i, is_read: true })));
      setUnreadCount(0);
    } catch {
      /* silent */
    }
  }

  async function handleExpand(item: InboxItem) {
    if (expandedId === item.id) {
      setExpandedId(null);
      setReplyText("");
      return;
    }
    setExpandedId(item.id);
    setReplyText("");
    if (!item.is_read && workspaceId) {
      try {
        const token = await getToken();
        if (token) {
          await markInboxItemRead(token, workspaceId, item.id);
          setItems((prev) =>
            prev.map((i) => (i.id === item.id ? { ...i, is_read: true } : i))
          );
          setUnreadCount((c) => Math.max(0, c - 1));
        }
      } catch {
        /* silent */
      }
    }
  }

  async function handleReply(item: InboxItem) {
    if (!workspaceId || !replyText.trim()) return;
    setReplying(true);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await replyToInboxItem(token, workspaceId, item.id, replyText.trim());
      if (res.data) {
        setItems((prev) => {
          const idx = prev.findIndex((i) => i.id === item.id);
          if (idx === -1) return [res.data, ...prev];
          const copy = [...prev];
          copy.splice(idx + 1, 0, res.data);
          return copy;
        });
      }
      setReplyText("");
    } catch {
      /* silent */
    } finally {
      setReplying(false);
    }
  }

  const filterButtons: { label: string; value: FilterSource }[] = [
    { label: "All", value: "" },
    { label: "Comments", value: "ig_comment" },
    { label: "DMs", value: "ig_dm" },
    { label: "Threads", value: "threads_reply" },
  ];

  return (
    <div>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 24 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <MessageSquare style={{ width: 24, height: 24, color: "var(--daccent)" }} />
          <h1 className="dt-heading" style={{ margin: 0 }}>Inbox</h1>
          {unreadCount > 0 && (
            <span style={{
              background: "var(--daccent)",
              color: "var(--primary-foreground)",
              fontSize: 11,
              fontWeight: 700,
              borderRadius: 10,
              padding: "2px 8px",
              minWidth: 20,
              textAlign: "center",
            }}>
              {unreadCount}
            </span>
          )}
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          <button
            onClick={handleMarkAllRead}
            disabled={unreadCount === 0}
            className="dt-body-sm"
            style={{
              display: "flex", alignItems: "center", gap: 6,
              padding: "6px 12px", borderRadius: 6,
              border: "1px solid var(--dborder)",
              background: "transparent", cursor: "pointer",
              color: unreadCount === 0 ? "var(--dmuted2)" : "var(--dtext)",
              opacity: unreadCount === 0 ? 0.5 : 1,
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
              display: "flex", alignItems: "center", gap: 6,
              padding: "6px 12px", borderRadius: 6,
              border: "1px solid var(--dborder)",
              background: "var(--daccent)", color: "var(--primary-foreground)",
              cursor: syncing ? "wait" : "pointer",
            }}
          >
            <RefreshCw style={{ width: 14, height: 14, animation: syncing ? "spin 1s linear infinite" : "none" }} />
            {syncing ? "Syncing..." : "Sync"}
          </button>
        </div>
      </div>

      {/* Filters */}
      <div style={{ display: "flex", gap: 4, marginBottom: 16 }}>
        {filterButtons.map((f) => (
          <button
            key={f.value}
            onClick={() => setFilter(f.value)}
            className="dt-body-sm"
            style={{
              padding: "5px 14px",
              borderRadius: 6,
              border: "1px solid var(--dborder)",
              background: filter === f.value ? "var(--accent-dim)" : "transparent",
              color: filter === f.value ? "var(--daccent)" : "var(--dmuted)",
              cursor: "pointer",
              fontWeight: filter === f.value ? 600 : 400,
            }}
          >
            {f.label}
          </button>
        ))}
      </div>

      {/* List */}
      {loading ? (
        <div style={{ padding: 40, textAlign: "center", color: "var(--dmuted)" }}>
          Loading...
        </div>
      ) : items.length === 0 ? (
        <div style={{
          padding: 60, textAlign: "center", color: "var(--dmuted)",
          border: "1px dashed var(--dborder)", borderRadius: 8,
        }}>
          <MessageSquare style={{ width: 32, height: 32, margin: "0 auto 12px", opacity: 0.4 }} />
          <p className="dt-body" style={{ margin: 0 }}>No messages yet</p>
          <p className="dt-body-sm" style={{ margin: "4px 0 0", color: "var(--dmuted2)" }}>
            Click Sync to fetch comments and replies from your connected accounts
          </p>
        </div>
      ) : (
        <div style={{ border: "1px solid var(--dborder)", borderRadius: 8, overflow: "hidden" }}>
          {items.map((item, idx) => {
            const isExpanded = expandedId === item.id;
            return (
              <div key={item.id}>
                <button
                  onClick={() => handleExpand(item)}
                  style={{
                    width: "100%",
                    display: "flex",
                    alignItems: "flex-start",
                    gap: 12,
                    padding: "12px 16px",
                    border: "none",
                    borderBottom: idx < items.length - 1 && !isExpanded ? "1px solid var(--dborder)" : "none",
                    background: isExpanded ? "var(--accent-dim)" : item.is_read ? "transparent" : "var(--sidebar-accent)",
                    cursor: "pointer",
                    textAlign: "left",
                    fontFamily: "inherit",
                    transition: "background 0.1s",
                  }}
                  onMouseEnter={(e) => {
                    if (!isExpanded) e.currentTarget.style.background = "var(--sidebar-accent)";
                  }}
                  onMouseLeave={(e) => {
                    if (!isExpanded) e.currentTarget.style.background = item.is_read ? "transparent" : "var(--sidebar-accent)";
                  }}
                >
                  {/* Unread dot */}
                  <div style={{ width: 8, paddingTop: 6, flexShrink: 0 }}>
                    {!item.is_read && (
                      <div style={{
                        width: 8, height: 8, borderRadius: "50%",
                        background: "var(--daccent)",
                      }} />
                    )}
                  </div>

                  {/* Platform icon */}
                  <div style={{ flexShrink: 0, paddingTop: 2 }}>
                    <PlatformIcon
                      platform={item.account_platform || SOURCE_PLATFORM[item.source] || "instagram"}
                      size={20}
                    />
                  </div>

                  {/* Content */}
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 2 }}>
                      <span className="dt-body-sm" style={{
                        fontWeight: item.is_read ? 400 : 600,
                        color: "var(--dtext)",
                      }}>
                        {item.is_own ? "You" : (item.author_name || "Unknown")}
                      </span>
                      <span style={{
                        display: "inline-flex", alignItems: "center", gap: 3,
                        fontSize: 10, color: "var(--dmuted2)",
                        background: "var(--sidebar-accent)", borderRadius: 4,
                        padding: "1px 6px",
                      }}>
                        <SourceIcon source={item.source} />
                        {SOURCE_LABELS[item.source] || item.source}
                      </span>
                      {item.account_name && (
                        <span className="dt-mono" style={{ fontSize: 10, color: "var(--dmuted2)" }}>
                          @{item.account_name}
                        </span>
                      )}
                    </div>
                    <p className="dt-body-sm" style={{
                      margin: 0,
                      color: "var(--dmuted)",
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: isExpanded ? "normal" : "nowrap",
                    }}>
                      {item.body || "(no text)"}
                    </p>
                  </div>

                  {/* Time */}
                  <span className="dt-mono" style={{
                    fontSize: 11, color: "var(--dmuted2)",
                    whiteSpace: "nowrap", flexShrink: 0,
                  }}>
                    {timeAgo(item.received_at)}
                  </span>
                </button>

                {/* Expanded reply area */}
                {isExpanded && (
                  <div style={{
                    padding: "12px 16px 16px 52px",
                    borderBottom: idx < items.length - 1 ? "1px solid var(--dborder)" : "none",
                    background: "var(--accent-dim)",
                  }}>
                    {!item.is_own && (
                      <div style={{ display: "flex", gap: 8, marginTop: 4 }}>
                        <input
                          type="text"
                          value={replyText}
                          onChange={(e) => setReplyText(e.target.value)}
                          placeholder={`Reply to ${item.author_name || "this message"}...`}
                          onKeyDown={(e) => {
                            if (e.key === "Enter" && !e.shiftKey) {
                              e.preventDefault();
                              handleReply(item);
                            }
                          }}
                          style={{
                            flex: 1,
                            padding: "8px 12px",
                            borderRadius: 6,
                            border: "1px solid var(--dborder)",
                            background: "var(--sidebar)",
                            color: "var(--dtext)",
                            fontSize: 13,
                            fontFamily: "inherit",
                            outline: "none",
                          }}
                        />
                        <button
                          onClick={() => handleReply(item)}
                          disabled={replying || !replyText.trim()}
                          style={{
                            display: "flex", alignItems: "center", gap: 6,
                            padding: "8px 16px",
                            borderRadius: 6,
                            border: "none",
                            background: "var(--daccent)",
                            color: "var(--primary-foreground)",
                            cursor: replying || !replyText.trim() ? "not-allowed" : "pointer",
                            opacity: replying || !replyText.trim() ? 0.5 : 1,
                            fontSize: 13,
                            fontFamily: "inherit",
                          }}
                        >
                          <Send style={{ width: 14, height: 14 }} />
                          {replying ? "Sending..." : "Reply"}
                        </button>
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      <style>{`
        @keyframes spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
}
