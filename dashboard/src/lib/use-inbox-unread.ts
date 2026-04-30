"use client";

// useGlobalInboxUnreadCount drives the sidebar's "Inbox" badge.
//
// Why this exists separately from the inbox page's own state: the
// sidebar is always mounted while the inbox page is mounted only when
// you navigate there, but the badge has to update in real time across
// the whole dashboard so users see new comments / DMs without having
// to open the inbox first. Routing the count through the same /v1/inbox/ws
// stream the inbox page uses (`useInboxWebSocket`) keeps the two views
// authoritative without a custom server-side push channel.
//
// Two WebSocket connections per tab when you're ON the inbox page (one
// here, one inside the inbox page) is a known and acceptable cost — a
// React Context to share a single connection would be a bigger refactor.

import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { getInboxUnreadCount } from "@/lib/api";
import { useInboxWebSocket } from "@/lib/use-inbox-ws";

export function useGlobalInboxUnreadCount(enabled: boolean): number {
  const { getToken } = useAuth();
  const [count, setCount] = useState(0);

  // Authoritative refetch — pulled out so we can call it on mount,
  // on a refresh interval, and after every `inbox.sync_complete`
  // event from the websocket. The caller never sees an error from
  // this; a transient API failure just leaves the previous count in
  // place until the next attempt succeeds.
  const refetch = useCallback(async () => {
    if (!enabled) return;
    const token = await getToken();
    if (!token) return;
    try {
      const res = await getInboxUnreadCount(token);
      if (res?.data && typeof res.data.count === "number") {
        setCount(res.data.count);
      }
    } catch {
      // Swallowed — see comment above.
    }
  }, [enabled, getToken]);

  useEffect(() => {
    if (!enabled) return;
    // refetch() is async — the setCount call inside happens after
    // the network round-trip resolves, not synchronously during this
    // effect, so the cascading-render rule doesn't apply. The lint
    // rule is body-shape-based and can't see across the await chain.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    refetch();
    // 60-second poll catches "marked read in another tab" without
    // a dedicated read-event push.
    const interval = setInterval(refetch, 60_000);
    return () => clearInterval(interval);
  }, [enabled, refetch]);

  // Same-tab cross-tree push: the inbox page emits these events when
  // the user opens a conversation (auto-marking unread items read) or
  // clicks "Mark all read". Without this listener the sidebar would
  // wait up to 60s for the next poll and the badge would feel stuck.
  // BroadcastChannel would also work cross-tab but window events are
  // enough for the in-tab case the user is currently asking about.
  useEffect(() => {
    if (!enabled) return;
    function onMarkRead(e: Event) {
      const evt = e as CustomEvent<{ count: number }>;
      const delta = evt.detail?.count ?? 0;
      if (delta > 0) {
        setCount((c) => Math.max(0, c - delta));
      }
    }
    function onMarkAllRead() {
      setCount(0);
    }
    window.addEventListener("inbox:mark-read", onMarkRead);
    window.addEventListener("inbox:mark-all-read", onMarkAllRead);
    return () => {
      window.removeEventListener("inbox:mark-read", onMarkRead);
      window.removeEventListener("inbox:mark-all-read", onMarkAllRead);
    };
  }, [enabled]);

  // WebSocket fan-in. Inbound items bump the count optimistically;
  // sync-complete events trigger an authoritative refetch so the
  // count converges even if we missed a marked-read event.
  useInboxWebSocket(
    enabled,
    (item) => {
      if (!item.is_own && !item.is_read) {
        setCount((c) => c + 1);
      }
    },
    () => {
      refetch();
    },
  );

  // Returning `enabled ? count : 0` keeps the rendered value at zero
  // when the hook is disabled (e.g. feature flag off, no profile yet),
  // without resetting `count` synchronously inside the effect. The
  // synchronous-setState-in-effect pattern triggers cascading renders;
  // a derived return is the cheaper, lint-clean equivalent.
  return enabled ? count : 0;
}
