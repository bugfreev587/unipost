"use client";

import { useEffect, useRef, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import type { InboxItem } from "@/lib/api";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

function getWsUrl(workspaceId: string, token: string): string {
  const base = API_URL.replace(/^http/, "ws");
  return `${base}/v1/workspaces/${workspaceId}/inbox/ws?token=${token}`;
}

/**
 * Connects to the inbox WebSocket and calls onNewItem when a new
 * inbox item arrives. Falls back gracefully — returns `connected`
 * so the caller can poll as a fallback when WS is unavailable.
 */
export function useInboxWebSocket(
  workspaceId: string | null,
  onNewItem: (item: InboxItem) => void,
  onSyncComplete?: () => void
): { connected: boolean } {
  const { getToken } = useAuth();
  const wsRef = useRef<WebSocket | null>(null);
  const [connected, setConnected] = useState(false);
  const onNewItemRef = useRef(onNewItem);
  onNewItemRef.current = onNewItem;
  const onSyncCompleteRef = useRef(onSyncComplete);
  onSyncCompleteRef.current = onSyncComplete;
  const mountedRef = useRef(true);
  const getTokenRef = useRef(getToken);
  getTokenRef.current = getToken;

  useEffect(() => {
    if (!workspaceId) return;

    mountedRef.current = true;
    let reconnectTimer: ReturnType<typeof setTimeout>;
    let attempt = 0;

    async function connect() {
      if (!mountedRef.current || !workspaceId) return;

      // Close any existing connection
      if (wsRef.current) {
        wsRef.current.onclose = null;
        wsRef.current.close();
        wsRef.current = null;
      }

      try {
        // Always get a fresh token on each connect attempt
        const token = await getTokenRef.current();
        if (!token || !mountedRef.current) return;

        const ws = new WebSocket(getWsUrl(workspaceId, token));
        wsRef.current = ws;

        ws.onopen = () => {
          attempt = 0;
          setConnected(true);
        };

        ws.onmessage = (e) => {
          try {
            const msg = JSON.parse(e.data);
            if (msg.type === "inbox.new_item" && msg.item) {
              onNewItemRef.current(msg.item as InboxItem);
            } else if (msg.type === "inbox.sync_complete") {
              onSyncCompleteRef.current?.();
            }
          } catch {
            // ignore malformed messages
          }
        };

        ws.onclose = () => {
          setConnected(false);
          wsRef.current = null;
          if (!mountedRef.current) return;
          // Reconnect with backoff: 2s, 4s, 8s, max 30s
          const delay = Math.min(2000 * 2 ** attempt, 30000);
          attempt++;
          reconnectTimer = setTimeout(connect, delay);
        };

        ws.onerror = () => {
          // onclose fires after onerror — reconnect happens there
        };
      } catch {
        if (!mountedRef.current) return;
        const delay = Math.min(2000 * 2 ** attempt, 30000);
        attempt++;
        reconnectTimer = setTimeout(connect, delay);
      }
    }

    // Reconnect every 50 seconds to refresh the Clerk JWT (expires ~60s)
    const refreshInterval = setInterval(() => {
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        attempt = 0; // reset backoff since this is a planned reconnect
        wsRef.current.onclose = null;
        wsRef.current.close();
        wsRef.current = null;
        connect();
      }
    }, 50_000);

    connect();

    return () => {
      mountedRef.current = false;
      clearTimeout(reconnectTimer);
      clearInterval(refreshInterval);
      if (wsRef.current) {
        wsRef.current.onclose = null;
        wsRef.current.close();
        wsRef.current = null;
      }
      setConnected(false);
    };
  }, [workspaceId]); // only re-run when workspace changes

  return { connected };
}
