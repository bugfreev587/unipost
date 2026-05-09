"use client";

import { type CSSProperties, useEffect, useMemo, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import {
  Search,
  RefreshCw,
  FileText,
  X,
  ChevronRight,
  ExternalLink,
} from "lucide-react";

import {
  type AdminIntegrationLog,
  getAdminIntegrationLog,
  listAdminIntegrationLogs,
} from "@/lib/api";
import { AdminShell } from "../_components/admin-ui";

type TimeRangeKey = "15m" | "1h" | "24h" | "7d" | "30d";

const CATEGORY_OPTIONS = [
  { value: "all", label: "All categories" },
  { value: "publishing", label: "Publishing" },
  { value: "api_request", label: "API requests" },
  { value: "oauth", label: "Account connection" },
  { value: "webhook", label: "Webhooks" },
  { value: "system", label: "System" },
];

const PLATFORM_OPTIONS = [
  { value: "all", label: "All platforms" },
  { value: "twitter", label: "X" },
  { value: "linkedin", label: "LinkedIn" },
  { value: "instagram", label: "Instagram" },
  { value: "threads", label: "Threads" },
  { value: "tiktok", label: "TikTok" },
  { value: "youtube", label: "YouTube" },
  { value: "bluesky", label: "Bluesky" },
  { value: "facebook", label: "Facebook" },
  { value: "pinterest", label: "Pinterest" },
];

const STATUS_OPTIONS = [
  { value: "all", label: "All statuses" },
  { value: "success", label: "Success" },
  { value: "warning", label: "Warning" },
  { value: "error", label: "Error" },
];

const SOURCE_OPTIONS = [
  { value: "all", label: "All sources" },
  { value: "api", label: "API" },
  { value: "dashboard", label: "Dashboard" },
  { value: "worker", label: "Worker" },
  { value: "webhook", label: "Webhook" },
  { value: "oauth", label: "OAuth" },
];

const TIME_RANGE_OPTIONS: Array<{ value: TimeRangeKey; label: string }> = [
  { value: "15m", label: "Last 15m" },
  { value: "1h", label: "Last 1h" },
  { value: "24h", label: "Last 24h" },
  { value: "7d", label: "Last 7d" },
  { value: "30d", label: "Last 30d" },
];

function rangeToISO(range: TimeRangeKey) {
  const now = new Date();
  const start = new Date(now);
  switch (range) {
    case "15m":
      start.setMinutes(start.getMinutes() - 15);
      break;
    case "1h":
      start.setHours(start.getHours() - 1);
      break;
    case "24h":
      start.setDate(start.getDate() - 1);
      break;
    case "7d":
      start.setDate(start.getDate() - 7);
      break;
    case "30d":
      start.setDate(start.getDate() - 30);
      break;
  }
  return { from: start.toISOString(), to: now.toISOString() };
}

function formatTimestamp(value: string) {
  return new Date(value).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function relativeTimeLabel(value: string) {
  const deltaMs = Date.now() - new Date(value).getTime();
  const sec = Math.max(1, Math.floor(deltaMs / 1000));
  if (sec < 60) return `${sec}s ago`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const day = Math.floor(hr / 24);
  return `${day}d ago`;
}

function retentionDaysForPlan(planId?: string) {
  switch ((planId || "").toLowerCase()) {
    case "free":
      return 1;
    case "api":
      return 7;
    case "basic":
      return 14;
    case "growth":
      return 30;
    case "team":
      return 90;
    case "enterprise":
      return 180;
    default:
      return 7;
  }
}

const CONSOLE_FRAME_BACKGROUND =
  "linear-gradient(180deg, color-mix(in srgb, var(--surface-raised, var(--surface)) 88%, var(--sidebar) 12%) 0%, color-mix(in srgb, var(--surface2) 92%, var(--sidebar) 8%) 100%)";
const CONSOLE_FRAME_BORDER = "1px solid color-mix(in srgb, var(--dborder) 76%, var(--sidebar) 24%)";
const CONSOLE_HEADER_BACKGROUND = "color-mix(in srgb, var(--surface) 68%, var(--sidebar) 32%)";
const CONSOLE_HEADER_BORDER = "1px solid color-mix(in srgb, var(--dborder) 66%, var(--sidebar) 34%)";
const CONSOLE_ROW_BORDER = "1px solid color-mix(in srgb, var(--dborder) 72%, transparent)";
const CONSOLE_SELECTED_BG = "color-mix(in srgb, var(--accent) 10%, var(--surface2))";
const CONSOLE_TEXT_PRIMARY = "color-mix(in srgb, var(--dtext) 94%, white 6%)";
const CONSOLE_TEXT_MUTED = "color-mix(in srgb, var(--dmuted) 92%, var(--dtext) 8%)";
const CONSOLE_TEXT_SUBTLE = "color-mix(in srgb, var(--dmuted2) 90%, var(--dtext) 10%)";
const DRAWER_PANEL_BACKGROUND = "color-mix(in srgb, var(--surface2) 82%, var(--sidebar) 18%)";
const DRAWER_PANEL_BORDER = "1px solid color-mix(in srgb, var(--dborder) 74%, var(--sidebar) 26%)";
const DRAWER_CODE_BACKGROUND = "color-mix(in srgb, var(--surface) 66%, var(--sidebar) 34%)";

function toneForStatus(status: AdminIntegrationLog["status"]) {
  switch (status) {
    case "error":
      return {
        fg: "#ef4444",
        bg: "color-mix(in srgb, #ef4444 14%, var(--surface))",
        border: "color-mix(in srgb, #ef4444 28%, var(--dborder))",
      };
    case "warning":
      return {
        fg: "#f59e0b",
        bg: "color-mix(in srgb, #f59e0b 14%, var(--surface))",
        border: "color-mix(in srgb, #f59e0b 28%, var(--dborder))",
      };
    default:
      return {
        fg: "#10b981",
        bg: "color-mix(in srgb, #10b981 14%, var(--surface))",
        border: "color-mix(in srgb, #10b981 28%, var(--dborder))",
      };
  }
}

function FieldChip({
  label,
  value,
  onClick,
}: {
  label: string;
  value: string;
  onClick?: () => void;
}) {
  const content = (
    <>
      <span style={{ color: "var(--dmuted2)" }}>{label}</span>
      <span style={{ fontFamily: "var(--font-geist-mono), monospace" }}>{value}</span>
    </>
  );
  const style = {
    display: "inline-flex",
    alignItems: "center",
    gap: 6,
    padding: "6px 10px",
    borderRadius: 999,
    border: "1px solid var(--dborder)",
    background: "var(--surface2)",
    color: "var(--dtext)",
    fontSize: 12,
    cursor: onClick ? "pointer" : "default",
  } as const;

  if (!onClick) {
    return <span style={style}>{content}</span>;
  }
  return (
    <button type="button" onClick={onClick} style={style}>
      {content}
    </button>
  );
}

function JSONBlock({ value }: { value: unknown }) {
  if (value === null || value === undefined) {
    return (
      <div className="dt-body-sm" style={{ color: "var(--dmuted2)" }}>
        None
      </div>
    );
  }
  return (
    <pre
      style={{
        margin: 0,
        padding: 14,
        borderRadius: 12,
        border: DRAWER_PANEL_BORDER,
        background: DRAWER_CODE_BACKGROUND,
        color: "var(--dtext)",
        fontSize: 12,
        lineHeight: 1.6,
        overflowX: "auto",
        whiteSpace: "pre-wrap",
        wordBreak: "break-word",
      }}
    >
      {JSON.stringify(value, null, 2)}
    </pre>
  );
}

function asObject(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function requestEnvelope(value: unknown) {
  const obj = asObject(value);
  if (!obj) return null;
  return {
    protocol: typeof obj.protocol === "string" ? obj.protocol : "",
    method: typeof obj.method === "string" ? obj.method : "",
    path: typeof obj.path === "string" ? obj.path : "",
    statusCode: typeof obj.status_code === "number" ? obj.status_code : undefined,
    query: obj.query,
    headers: obj.headers,
    payload: obj.payload,
  };
}

export default function AdminLogsPage() {
  const { getToken } = useAuth();

  const [query, setQuery] = useState("");
  const [workspaceFilter, setWorkspaceFilter] = useState("");
  const [category, setCategory] = useState("all");
  const [platform, setPlatform] = useState("all");
  const [requestFilter, setRequestFilter] = useState("");
  const [postFilter, setPostFilter] = useState("");
  const [errorCodeFilter, setErrorCodeFilter] = useState("");
  const [status, setStatus] = useState("all");
  const [source, setSource] = useState("all");
  const [timeRange, setTimeRange] = useState<TimeRangeKey>("7d");

  const [logs, setLogs] = useState<AdminIntegrationLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedLogId, setSelectedLogId] = useState<number | null>(null);
  const [selectedLog, setSelectedLog] = useState<AdminIntegrationLog | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const closeDetail = () => {
    setSelectedLogId(null);
    setSelectedLog(null);
  };

  const { from, to } = useMemo(() => rangeToISO(timeRange), [timeRange]);

  const loadLogs = async (showRefreshing = false) => {
    try {
      if (showRefreshing) setRefreshing(true);
      else setLoading(true);
      setError(null);

      const token = await getToken();
      if (!token) {
        throw new Error("Not authenticated");
      }

      const logsRes = await listAdminIntegrationLogs(token, {
        q: query || undefined,
        workspace_id: workspaceFilter || undefined,
        category: category === "all" ? undefined : category,
        platform: platform === "all" ? undefined : platform,
        status: status === "all" ? undefined : status,
        source: source === "all" ? undefined : source,
        request_id: requestFilter || undefined,
        post_id: postFilter || undefined,
        error_code: errorCodeFilter || undefined,
        from,
        to,
        limit: 150,
      });

      setLogs(logsRes.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load logs");
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    void loadLogs();
  }, [query, workspaceFilter, category, platform, status, source, requestFilter, postFilter, errorCodeFilter, from, to]);

  useEffect(() => {
    if (selectedLogId == null) return;
    let cancelled = false;
    (async () => {
      try {
        setDetailLoading(true);
        const token = await getToken();
        if (!token) throw new Error("Not authenticated");
        const res = await getAdminIntegrationLog(token, selectedLogId);
        if (!cancelled) setSelectedLog(res.data);
      } catch {
        if (!cancelled) setSelectedLog(null);
      } finally {
        if (!cancelled) setDetailLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [getToken, selectedLogId]);

  const errorCount = useMemo(() => logs.filter((log) => log.status === "error").length, [logs]);
  const warningCount = useMemo(() => logs.filter((log) => log.status === "warning").length, [logs]);
  const workspaceCount = useMemo(() => new Set(logs.map((log) => log.workspace_id)).size, [logs]);
  const latestLog = logs[0] || null;
  const retentionSummary = useMemo(() => {
    const plans = Array.from(new Set(logs.map((log) => (log.plan_id || "free").toLowerCase()).filter(Boolean)));
    if (plans.length === 1) {
      return `${retentionDaysForPlan(plans[0])}d`;
    }
    return "Varies";
  }, [logs]);

  const activeFilters = useMemo(() => {
    const entries: Array<{ key: string; label: string; clear: () => void }> = [];
    if (workspaceFilter) entries.push({ key: "workspace", label: workspaceFilter, clear: () => setWorkspaceFilter("") });
    if (requestFilter) entries.push({ key: "request_id", label: requestFilter, clear: () => setRequestFilter("") });
    if (postFilter) entries.push({ key: "post_id", label: postFilter, clear: () => setPostFilter("") });
    if (errorCodeFilter) entries.push({ key: "error_code", label: errorCodeFilter, clear: () => setErrorCodeFilter("") });
    if (category !== "all") entries.push({ key: "category", label: category, clear: () => setCategory("all") });
    if (platform !== "all") entries.push({ key: "platform", label: platform, clear: () => setPlatform("all") });
    if (status !== "all") entries.push({ key: "status", label: status, clear: () => setStatus("all") });
    if (source !== "all") entries.push({ key: "source", label: source, clear: () => setSource("all") });
    if (query) entries.push({ key: "search", label: query, clear: () => setQuery("") });
    return entries;
  }, [workspaceFilter, requestFilter, postFilter, errorCodeFilter, category, platform, status, source, query]);

  return (
    <AdminShell title="Logs" loading={loading || refreshing} onRefresh={() => void loadLogs(true)} requireSuperAdmin>
      <div style={{ display: "grid", gap: 14 }}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 16, alignItems: "flex-start", flexWrap: "wrap" }}>
          <div>
            <div style={{ fontSize: 28, fontWeight: 700, letterSpacing: "-0.03em" }}>Global Logs</div>
            <div style={{ color: "var(--dmuted)", marginTop: 4 }}>
              Search publishing, API, OAuth, and webhook events across all workspaces.
            </div>
          </div>
          <div style={{ color: "var(--dmuted)", fontSize: 12, textAlign: "right" }}>
            <div>Super admin only</div>
            <div>{latestLog ? `Latest event ${relativeTimeLabel(latestLog.ts)}` : "No recent events"}</div>
          </div>
        </div>

        <div style={toolbarWrapStyle}>
          <label style={searchStyle}>
            <Search size={16} />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search logs..."
              style={inputStyle}
            />
          </label>

          <input
            value={workspaceFilter}
            onChange={(e) => setWorkspaceFilter(e.target.value)}
            placeholder="Workspace ID"
            style={filterInputStyle}
          />

          <select value={category} onChange={(e) => setCategory(e.target.value)} style={selectStyle}>
            {CATEGORY_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>

          <select value={platform} onChange={(e) => setPlatform(e.target.value)} style={selectStyle}>
            {PLATFORM_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>

          <select value={status} onChange={(e) => setStatus(e.target.value)} style={selectStyle}>
            {STATUS_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>

          <select value={source} onChange={(e) => setSource(e.target.value)} style={selectStyle}>
            {SOURCE_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>

          <select value={timeRange} onChange={(e) => setTimeRange(e.target.value as TimeRangeKey)} style={selectStyle}>
            {TIME_RANGE_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>

          <button type="button" style={iconButtonStyle} onClick={() => void loadLogs(true)} aria-label="Refresh logs">
            <RefreshCw size={16} />
          </button>
        </div>

        {activeFilters.length > 0 ? (
          <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
            {activeFilters.map((filter) => (
              <button
                key={`${filter.key}:${filter.label}`}
                type="button"
                onClick={filter.clear}
                style={activeFilterChipStyle}
              >
                <span style={{ color: "var(--dmuted2)" }}>{filter.key}</span>
                <span>{filter.label}</span>
                <X size={12} />
              </button>
            ))}
          </div>
        ) : null}

        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 12,
            padding: "10px 14px",
            borderRadius: 14,
            border: "1px solid var(--dborder)",
            background: "var(--surface)",
          }}
        >
          <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
            <MiniStat label="Errors" value={String(errorCount)} tone="error" />
            <MiniStat label="Warnings" value={String(warningCount)} tone="warning" />
            <MiniStat label="Workspaces" value={String(workspaceCount)} />
            <MiniStat label="Retention" value={retentionSummary} />
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            {latestLog && !loading && (
              <div style={{ color: "var(--dmuted2)", fontSize: 12 }}>
                Latest event {relativeTimeLabel(latestLog.ts)}
              </div>
            )}
            <div style={{ color: "var(--dmuted2)", fontSize: 12 }}>
              {logs.length} row{logs.length === 1 ? "" : "s"}
            </div>
          </div>
        </div>

        {error ? (
          <div style={{ ...panelStyle, borderColor: "color-mix(in srgb, #ef4444 26%, var(--dborder))", color: "#ef4444" }}>
            {error}
          </div>
        ) : null}

        <div
          style={{
            ...tableShellStyle,
            borderRadius: 22,
            border: CONSOLE_FRAME_BORDER,
            background: CONSOLE_FRAME_BACKGROUND,
            boxShadow: "0 18px 50px color-mix(in srgb, var(--sidebar) 18%, transparent)",
          }}
        >
          <div style={tableHeaderStyle}>
            <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              <div style={{ color: CONSOLE_TEXT_PRIMARY, fontSize: 15, fontWeight: 700 }}>Global log stream</div>
              <div style={{ color: CONSOLE_TEXT_MUTED, fontSize: 12 }}>
                Cross-workspace support view. Filter by workspace ID to narrow to one customer.
              </div>
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap", justifyContent: "flex-end" }}>
              <ConsoleBadge label="super admin only" tone="success" />
              <ConsoleBadge label={`${logs.length} rows`} tone="neutral" />
            </div>
          </div>

          {loading ? (
            <div style={emptyStateStyle}>Loading logs…</div>
        ) : logs.length === 0 ? (
          <div style={emptyStateStyle}>
              <FileText size={32} color="var(--dmuted2)" />
              <div style={{ fontSize: 22, fontWeight: 600 }}>No logs found</div>
              <div style={{ color: "var(--dmuted)" }}>No matching logs in this time range.</div>
            </div>
        ) : (
          <div style={{ display: "grid" }}>
              {logs.map((log) => {
                const tone = toneForStatus(log.status);
                const workspaceLabel = log.workspace_name || log.workspace_id;
                return (
                  <button
                    key={log.id}
                    type="button"
                    onClick={() => setSelectedLogId(log.id)}
                    style={{
                      width: "100%",
                      display: "grid",
                      gridTemplateColumns: "170px minmax(0, 1fr)",
                      gap: 18,
                      padding: "14px 18px",
                      border: "none",
                      borderBottom: CONSOLE_ROW_BORDER,
                      background: selectedLogId === log.id ? CONSOLE_SELECTED_BG : "transparent",
                      textAlign: "left",
                      cursor: "pointer",
                    }}
                  >
                    <div style={{ minWidth: 0, position: "relative", paddingLeft: 16 }}>
                      <span
                        style={{
                          position: "absolute",
                          left: 0,
                          top: 2,
                          bottom: 2,
                          width: 4,
                          borderRadius: 999,
                          background: tone.fg,
                        }}
                      />
                      <div style={{ color: CONSOLE_TEXT_PRIMARY, fontSize: 13 }}>{formatTimestamp(log.ts)}</div>
                      <div style={{ color: CONSOLE_TEXT_SUBTLE, fontSize: 12, marginTop: 4 }}>{relativeTimeLabel(log.ts)}</div>
                    </div>
                    <div style={{ minWidth: 0, display: "grid", gap: 10 }}>
                      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12 }}>
                        <div style={{ minWidth: 0 }}>
                          <div style={{ color: CONSOLE_TEXT_PRIMARY, fontSize: 14, fontWeight: 600, fontFamily: "var(--font-geist-mono), monospace" }}>
                            {log.action}
                          </div>
                          <div style={{ color: CONSOLE_TEXT_MUTED, fontSize: 14, marginTop: 6, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                            {log.message}
                          </div>
                        </div>
                        <span style={{ ...statusBadgeStyle, color: tone.fg, background: tone.bg, borderColor: tone.border, flexShrink: 0 }}>
                          {log.status}
                        </span>
                      </div>
                      <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                        <ConsoleBadge label={workspaceLabel} tone="neutral" />
                        <ConsoleBadge label={log.workspace_id} tone="neutral" />
                        <ConsoleBadge label={log.plan_id || "free"} tone="neutral" />
                        <ConsoleBadge label={log.platform || "workspace"} tone="neutral" />
                        <ConsoleBadge label={log.request_id || log.post_id || "—"} tone="neutral" />
                      </div>
                      <div style={{ display: "flex", alignItems: "center", gap: 8, color: CONSOLE_TEXT_SUBTLE, fontSize: 12 }}>
                        <span>Open detail</span>
                        <ChevronRight size={14} />
                      </div>
                    </div>
                  </button>
                );
              })}
            </div>
          )}
        </div>

        {selectedLogId != null ? (
          <>
            <button
              type="button"
              aria-label="Close log detail"
              onClick={closeDetail}
              style={{
                position: "fixed",
                inset: 0,
                background: "rgba(0,0,0,0.35)",
                border: "none",
                zIndex: 70,
              }}
            />
            <aside
              style={{
                position: "fixed",
                top: 0,
                right: 0,
                bottom: 0,
                width: 520,
                maxWidth: "96vw",
                background: "var(--surface-raised, var(--surface))",
                borderLeft: "1px solid var(--dborder)",
                zIndex: 71,
                overflowY: "auto",
                padding: 22,
                display: "flex",
                flexDirection: "column",
                gap: 18,
              }}
            >
              <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12 }}>
                <div>
                  <div style={{ fontSize: 18, fontWeight: 700 }}>Log details</div>
                  <div style={{ color: "var(--dmuted)", marginTop: 4 }}>
                    {selectedLog ? selectedLog.action : "Loading event…"}
                  </div>
                </div>
                <button type="button" onClick={closeDetail} style={iconButtonStyle}>
                  <X size={16} />
                </button>
              </div>

              {detailLoading || !selectedLog ? (
                <div style={{ color: "var(--dmuted)" }}>Loading event details…</div>
              ) : (
                <div style={{ display: "grid", gap: 18 }}>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                  <FieldChip label="status" value={selectedLog.status} />
                  <FieldChip label="level" value={selectedLog.level} />
                  <FieldChip label="source" value={selectedLog.source} />
                  <FieldChip label="time" value={formatTimestamp(selectedLog.ts)} />
                </div>

                <div style={sectionStyle}>
                  <div style={sectionTitleStyle}>Context</div>
                  <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                    <FieldChip
                      label="workspace"
                      value={selectedLog.workspace_id}
                      onClick={() => setWorkspaceFilter(selectedLog.workspace_id || "")}
                    />
                    {selectedLog.plan_id && <FieldChip label="plan" value={selectedLog.plan_id} />}
                    {selectedLog.owner_email && <FieldChip label="owner" value={selectedLog.owner_email} />}
                    {selectedLog.platform && <FieldChip label="platform" value={selectedLog.platform} onClick={() => setPlatform(selectedLog.platform || "all")} />}
                    {selectedLog.request_id && <FieldChip label="request_id" value={selectedLog.request_id} onClick={() => setRequestFilter(selectedLog.request_id || "")} />}
                    {selectedLog.post_id && <FieldChip label="post_id" value={selectedLog.post_id} onClick={() => setPostFilter(selectedLog.post_id || "")} />}
                    {selectedLog.error_code && <FieldChip label="error_code" value={selectedLog.error_code} onClick={() => setErrorCodeFilter(selectedLog.error_code || "")} />}
                  </div>
                </div>

                <div style={sectionStyle}>
                  <div style={sectionTitleStyle}>Summary</div>
                  <div style={{ fontSize: 15, lineHeight: 1.6 }}>{selectedLog.message}</div>
                  <div style={{ marginTop: 12, display: "grid", gap: 8, color: "var(--dmuted)" }}>
                    {selectedLog.endpoint ? <div>Endpoint: <span style={{ color: "var(--dtext)" }}>{selectedLog.endpoint}</span></div> : null}
                    {selectedLog.method ? <div>Method: <span style={{ color: "var(--dtext)" }}>{selectedLog.method}</span></div> : null}
                    {selectedLog.http_status_code ? <div>HTTP status: <span style={{ color: "var(--dtext)" }}>{selectedLog.http_status_code}</span></div> : null}
                    {selectedLog.remote_status_code ? <div>Remote status: <span style={{ color: "var(--dtext)" }}>{selectedLog.remote_status_code}</span></div> : null}
                    {selectedLog.duration_ms ? <div>Duration: <span style={{ color: "var(--dtext)" }}>{selectedLog.duration_ms}ms</span></div> : null}
                  </div>
                </div>

                <div style={sectionStyle}>
                  <div style={sectionTitleStyle}>Metadata</div>
                  <JSONBlock value={selectedLog.metadata} />
                </div>

                <div style={sectionStyle}>
                  <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                    <div style={sectionTitleStyle}>Request</div>
                    {selectedLog.request_id ? (
                      <button type="button" onClick={() => setRequestFilter(selectedLog.request_id || "")} style={linkButtonStyle}>
                        Filter same request
                        <ChevronRight size={14} />
                      </button>
                    ) : null}
                  </div>
                  {(() => {
                    const envelope = requestEnvelope(selectedLog.request_payload);
                    if (!envelope) return <JSONBlock value={selectedLog.request_payload} />;
                    return (
                      <div style={{ display: "grid", gap: 10 }}>
                        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
                          {envelope.protocol && <KeyValue label="Protocol" value={envelope.protocol} />}
                          {envelope.method && <KeyValue label="Method" value={envelope.method} />}
                          {envelope.path && <KeyValue label="Path" value={envelope.path} />}
                        </div>
                        <div>
                          <div style={sectionTitleStyle}>Headers</div>
                          <JSONBlock value={envelope.headers} />
                        </div>
                        <div>
                          <div style={sectionTitleStyle}>Query</div>
                          <JSONBlock value={envelope.query} />
                        </div>
                        <div>
                          <div style={sectionTitleStyle}>Payload</div>
                          <JSONBlock value={envelope.payload} />
                        </div>
                      </div>
                    );
                  })()}
                </div>

                <div style={sectionStyle}>
                  <div style={sectionTitleStyle}>Response</div>
                  {(() => {
                    const envelope = requestEnvelope(selectedLog.response_payload);
                    if (!envelope) return <JSONBlock value={selectedLog.response_payload} />;
                    return (
                      <div style={{ display: "grid", gap: 10 }}>
                        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
                          {envelope.protocol && <KeyValue label="Protocol" value={envelope.protocol} />}
                          {envelope.statusCode !== undefined && <KeyValue label="Status code" value={String(envelope.statusCode)} />}
                        </div>
                        <div>
                          <div style={sectionTitleStyle}>Headers</div>
                          <JSONBlock value={envelope.headers} />
                        </div>
                        <div>
                          <div style={sectionTitleStyle}>Payload</div>
                          <JSONBlock value={envelope.payload} />
                        </div>
                      </div>
                    );
                  })()}
                </div>

                {selectedLog.workspace_id ? (
                  <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
                    <a href={`/projects?workspace=${selectedLog.workspace_id}`} style={linkButtonStyle}>
                      Open workspace context
                      <ExternalLink size={14} />
                    </a>
                  </div>
                ) : null}
                </div>
              )}
            </aside>
          </>
        ) : null}
      </div>
    </AdminShell>
  );
}

function KeyValue({ label, value }: { label: string; value: string }) {
  return (
    <div
      style={{
        padding: "10px 12px",
        borderRadius: 12,
        border: DRAWER_PANEL_BORDER,
        background: DRAWER_PANEL_BACKGROUND,
      }}
    >
      <div style={sectionTitleStyle}>{label}</div>
      <div style={{ color: "var(--dtext)", fontFamily: "var(--font-geist-mono), monospace", marginTop: 6 }}>{value}</div>
    </div>
  );
}

const toolbarWrapStyle: CSSProperties = {
  display: "flex",
  gap: 12,
  flexWrap: "wrap",
  alignItems: "center",
};

const searchStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 10,
  minWidth: 280,
  padding: "0 14px",
  height: 44,
  border: "1px solid var(--dborder)",
  borderRadius: 14,
  background: "var(--surface)",
  color: "var(--dmuted)",
};

const inputStyle: CSSProperties = {
  border: "none",
  outline: "none",
  background: "transparent",
  color: "var(--dtext)",
  width: "100%",
  fontSize: 14,
};

const filterInputStyle: CSSProperties = {
  minWidth: 200,
  height: 44,
  padding: "0 14px",
  borderRadius: 14,
  border: "1px solid var(--dborder)",
  background: "var(--surface)",
  color: "var(--dtext)",
  fontSize: 14,
};

const selectStyle: CSSProperties = {
  height: 44,
  padding: "0 14px",
  borderRadius: 14,
  border: "1px solid var(--dborder)",
  background: "var(--surface)",
  color: "var(--dtext)",
  fontSize: 14,
};

const iconButtonStyle: CSSProperties = {
  height: 44,
  width: 44,
  borderRadius: 14,
  border: "1px solid var(--dborder)",
  background: "var(--surface)",
  color: "var(--dtext)",
  display: "inline-flex",
  alignItems: "center",
  justifyContent: "center",
  cursor: "pointer",
};

const activeFilterChipStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 8,
  borderRadius: 999,
  border: "1px solid var(--dborder)",
  background: "var(--surface2)",
  color: "var(--dtext)",
  padding: "8px 12px",
  fontSize: 12,
  cursor: "pointer",
};

const panelStyle: CSSProperties = {
  border: "1px solid var(--dborder)",
  borderRadius: 18,
  padding: 16,
  background: "var(--surface)",
};

const tableShellStyle: CSSProperties = {
  border: "1px solid var(--dborder)",
  borderRadius: 18,
  overflow: "hidden",
  background: "var(--surface)",
};

const tableHeaderStyle: CSSProperties = {
  display: "flex",
  alignItems: "center",
  justifyContent: "space-between",
  gap: 16,
  padding: "16px 18px",
  borderBottom: CONSOLE_HEADER_BORDER,
  color: "var(--dmuted)",
  fontSize: 12,
};

const statusBadgeStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  justifyContent: "center",
  minWidth: 86,
  padding: "6px 10px",
  borderRadius: 999,
  border: "1px solid var(--dborder)",
  fontSize: 12,
  fontWeight: 600,
  textTransform: "capitalize",
};

function MiniStat({ label, value, tone = "neutral" }: { label: string; value: string; tone?: "neutral" | "error" | "warning" }) {
  const colors =
    tone === "error"
      ? { fg: "#ef4444", bg: "rgba(239,68,68,0.08)", border: "rgba(239,68,68,0.18)" }
      : tone === "warning"
        ? { fg: "#f59e0b", bg: "rgba(245,158,11,0.08)", border: "rgba(245,158,11,0.18)" }
        : { fg: "var(--dtext)", bg: "var(--surface2)", border: "var(--dborder)" };
  return (
    <div
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 8,
        padding: "8px 10px",
        borderRadius: 999,
        border: `1px solid ${colors.border}`,
        background: colors.bg,
      }}
    >
      <span style={{ color: "var(--dmuted2)", fontSize: 12, textTransform: "uppercase", letterSpacing: "0.08em" }}>{label}</span>
      <span style={{ color: colors.fg, fontFamily: "var(--font-geist-mono), monospace", fontWeight: 700 }}>{value}</span>
    </div>
  );
}

function ConsoleBadge({ label, tone = "neutral" }: { label: string; tone?: "neutral" | "success" }) {
  const style =
    tone === "success"
      ? {
          color: "color-mix(in srgb, #10b981 78%, var(--dtext) 22%)",
          bg: "color-mix(in srgb, #10b981 10%, var(--surface))",
          border: "color-mix(in srgb, #10b981 20%, var(--dborder))",
        }
      : {
          color: CONSOLE_TEXT_MUTED,
          bg: "color-mix(in srgb, var(--surface) 74%, var(--sidebar) 26%)",
          border: "color-mix(in srgb, var(--dborder) 68%, var(--sidebar) 32%)",
        };
  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        padding: "5px 8px",
        borderRadius: 999,
        border: `1px solid ${style.border}`,
        background: style.bg,
        color: style.color,
        fontSize: 12,
        lineHeight: 1,
      }}
    >
      {label}
    </span>
  );
}

const emptyStateStyle: CSSProperties = {
  minHeight: 280,
  display: "grid",
  placeItems: "center",
  gap: 12,
  padding: 32,
  color: "var(--dmuted)",
  textAlign: "center",
};

const drawerShellStyle: CSSProperties = {
  border: "1px solid var(--dborder)",
  borderRadius: 20,
  background: "var(--surface)",
  overflow: "hidden",
};

const drawerHeaderStyle: CSSProperties = {
  padding: 20,
  borderBottom: "1px solid var(--dborder)",
  display: "flex",
  justifyContent: "space-between",
  alignItems: "flex-start",
  gap: 16,
};

const sectionStyle: CSSProperties = {
  display: "grid",
  gap: 12,
  padding: 14,
  borderRadius: 16,
  border: DRAWER_PANEL_BORDER,
  background: DRAWER_PANEL_BACKGROUND,
};

const sectionTitleStyle: CSSProperties = {
  fontSize: 13,
  fontWeight: 700,
  textTransform: "uppercase",
  letterSpacing: "0.12em",
  color: "var(--dmuted)",
};

const linkButtonStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 8,
  color: "var(--accent)",
  fontWeight: 600,
  fontSize: 13,
  background: "transparent",
  border: "none",
  padding: 0,
  cursor: "pointer",
  textDecoration: "none",
};
