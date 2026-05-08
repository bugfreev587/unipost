"use client";

import { useEffect, useRef, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import type { IntegrationLog } from "@/lib/api";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

function getWsUrl(token: string): string {
  const base = API_URL.replace(/^http/, "ws");
  return `${base}/v1/logs/ws?token=${token}`;
}

export function useLogsWebSocket(
  enabled: boolean,
  onNewLog: (log: IntegrationLog) => void,
): { connected: boolean } {
  const { getToken } = useAuth();
  const wsRef = useRef<WebSocket | null>(null);
  const [connected, setConnected] = useState(false);
  const onNewLogRef = useRef(onNewLog);
  onNewLogRef.current = onNewLog;
  const mountedRef = useRef(true);
  const getTokenRef = useRef(getToken);
  getTokenRef.current = getToken;

  useEffect(() => {
    if (!enabled) return;

    mountedRef.current = true;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let attempt = 0;

    function clearReconnectTimer() {
      if (reconnectTimer !== null) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
    }

    function scheduleReconnect() {
      if (!mountedRef.current) return;
      if (typeof navigator !== "undefined" && navigator.onLine === false) return;
      if (reconnectTimer !== null) return;
      const delay = Math.min(2000 * 2 ** attempt, 30000);
      attempt++;
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        connect();
      }, delay);
    }

    async function connect() {
      if (!mountedRef.current || !enabled) return;
      if (typeof navigator !== "undefined" && navigator.onLine === false) return;

      if (wsRef.current) {
        wsRef.current.onclose = null;
        wsRef.current.close();
        wsRef.current = null;
      }

      try {
        const token = await getTokenRef.current();
        if (!token || !mountedRef.current) return;

        const ws = new WebSocket(getWsUrl(token));
        wsRef.current = ws;

        ws.onopen = () => {
          attempt = 0;
          setConnected(true);
        };

        ws.onmessage = (e) => {
          try {
            const msg = JSON.parse(e.data);
            if (msg.type === "logs.new" && msg.log) {
              onNewLogRef.current(msg.log as IntegrationLog);
            }
          } catch {
            // ignore malformed messages
          }
        };

        ws.onclose = () => {
          setConnected(false);
          wsRef.current = null;
          scheduleReconnect();
        };

        ws.onerror = () => {
          // reconnect on close
        };
      } catch {
        scheduleReconnect();
      }
    }

    function onOnline() {
      clearReconnectTimer();
      attempt = 0;
      connect();
    }
    function onOffline() {
      clearReconnectTimer();
      setConnected(false);
    }
    window.addEventListener("online", onOnline);
    window.addEventListener("offline", onOffline);

    const refreshInterval = setInterval(() => {
      if (reconnectTimer !== null) return;
      if (typeof navigator !== "undefined" && navigator.onLine === false) return;
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        attempt = 0;
        wsRef.current.onclose = null;
        wsRef.current.close();
        wsRef.current = null;
        connect();
      }
    }, 50_000);

    connect();

    return () => {
      mountedRef.current = false;
      clearReconnectTimer();
      clearInterval(refreshInterval);
      window.removeEventListener("online", onOnline);
      window.removeEventListener("offline", onOffline);
      if (wsRef.current) {
        wsRef.current.onclose = null;
        wsRef.current.close();
        wsRef.current = null;
      }
      setConnected(false);
    };
  }, [enabled]);

  return { connected };
}
