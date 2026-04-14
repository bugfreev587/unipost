"use client";

import { useEffect, useRef, useCallback, useState } from "react";
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
  onNewItem: (item: InboxItem) => void
): { connected: boolean } {
  const { getToken } = useAuth();
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttempt = useRef(0);
  const [connected, setConnected] = useState(false);
  const onNewItemRef = useRef(onNewItem);
  onNewItemRef.current = onNewItem;

  const connect = useCallback(async () => {
    if (!workspaceId) return;
    try {
      const token = await getToken();
      if (!token) return;

      const ws = new WebSocket(getWsUrl(workspaceId, token));
      wsRef.current = ws;

      ws.onopen = () => {
        reconnectAttempt.current = 0;
        setConnected(true);
      };

      ws.onmessage = (e) => {
        try {
          const msg = JSON.parse(e.data);
          if (msg.type === "inbox.new_item" && msg.item) {
            onNewItemRef.current(msg.item as InboxItem);
          }
        } catch {
          // ignore malformed messages
        }
      };

      ws.onclose = () => {
        setConnected(false);
        wsRef.current = null;
        // Exponential backoff: 1s, 2s, 4s, 8s, max 30s
        const delay = Math.min(1000 * 2 ** reconnectAttempt.current, 30000);
        reconnectAttempt.current++;
        setTimeout(connect, delay);
      };

      ws.onerror = () => {
        // onclose will fire after onerror
      };
    } catch {
      // Token fetch failed — retry after delay
      const delay = Math.min(1000 * 2 ** reconnectAttempt.current, 30000);
      reconnectAttempt.current++;
      setTimeout(connect, delay);
    }
  }, [workspaceId, getToken]);

  useEffect(() => {
    connect();
    return () => {
      reconnectAttempt.current = 999; // prevent reconnect after unmount
      wsRef.current?.close();
      wsRef.current = null;
      setConnected(false);
    };
  }, [connect]);

  return { connected };
}
