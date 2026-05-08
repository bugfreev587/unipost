"use client";

import { type CSSProperties, useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import {
  listProfiles,
  listSocialAccounts,
  listIntegrationLogs,
  getIntegrationLog,
  getBilling,
  type IntegrationLog,
  type IntegrationLogListParams,
  type Profile,
  type SocialAccount,
  type BillingInfo,
} from "@/lib/api";
import { useLogsWebSocket } from "@/lib/use-logs-ws";
import {
  FileText,
  RefreshCw,
  Search,
  X,
  ExternalLink,
  ChevronRight,
  Radio,
  PauseCircle,
} from "lucide-react";

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

function toneForStatus(status: IntegrationLog["status"]) {
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
        border: "1px solid var(--dborder)",
        background: "color-mix(in srgb, var(--sidebar) 78%, var(--surface))",
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

export default function LogsPage() {
  const { id: profileId } = useParams<{ id: string }>();
  const { getToken } = useAuth();

  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("publishing");
  const [platform, setPlatform] = useState("all");
  const [accountFilter, setAccountFilter] = useState("all");
  const [requestFilter, setRequestFilter] = useState("");
  const [postFilter, setPostFilter] = useState("");
  const [errorCodeFilter, setErrorCodeFilter] = useState("");
  const [status, setStatus] = useState("all");
  const [source, setSource] = useState("all");
  const [timeRange, setTimeRange] = useState<TimeRangeKey>("7d");
  const [liveMode, setLiveMode] = useState(false);

  const [logs, setLogs] = useState<IntegrationLog[]>([]);
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [billing, setBilling] = useState<BillingInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedLogId, setSelectedLogId] = useState<number | null>(null);
  const [selectedLog, setSelectedLog] = useState<IntegrationLog | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const closeDetail = () => {
    setSelectedLogId(null);
    setSelectedLog(null);
  };

  const applyFieldFilter = (kind: "request" | "post" | "account" | "platform" | "error", value: string) => {
    if (!value) return;
    switch (kind) {
      case "request":
        setQuery("");
        setRequestFilter(value);
        setPostFilter("");
        setErrorCodeFilter("");
        setCategory("all");
        setStatus("all");
        setSource("all");
        setPlatform("all");
        setAccountFilter("all");
        break;
      case "post":
        setQuery("");
        setRequestFilter("");
        setPostFilter(value);
        setErrorCodeFilter("");
        setCategory("all");
        setStatus("all");
        setSource("all");
        setPlatform("all");
        setAccountFilter("all");
        break;
      case "account":
        setQuery("");
        setRequestFilter("");
        setPostFilter("");
        setErrorCodeFilter("");
        setCategory("all");
        setStatus("all");
        setSource("all");
        setPlatform("all");
        setAccountFilter(value);
        break;
      case "platform":
        setQuery("");
        setRequestFilter("");
        setPostFilter("");
        setErrorCodeFilter("");
        setCategory("all");
        setStatus("all");
        setSource("all");
        setAccountFilter("all");
        setPlatform(value);
        break;
      case "error":
        setQuery("");
        setRequestFilter("");
        setPostFilter("");
        setErrorCodeFilter(value);
        setCategory("all");
        setStatus("error");
        setSource("all");
        setPlatform("all");
        setAccountFilter("all");
        break;
    }
    closeDetail();
  };

  const profileNameById = useMemo(
    () => new Map(profiles.map((profile) => [profile.id, profile.name])),
    [profiles]
  );

  const accountById = useMemo(
    () => new Map(accounts.map((account) => [account.id, account])),
    [accounts]
  );

  const accountOptions = useMemo(() => {
    const sorted = [...accounts].sort((a, b) => {
      const aName = a.account_name || a.id;
      const bName = b.account_name || b.id;
      return aName.localeCompare(bName);
    });
    return [{ value: "all", label: "All accounts" }].concat(
      sorted.map((account) => ({
        value: account.id,
        label: `${account.account_name || account.id} · ${account.platform}`,
      }))
    );
  }, [accounts]);

  const params = useMemo<IntegrationLogListParams>(() => {
    const range = rangeToISO(timeRange);
    return {
      q: query.trim(),
      category,
      platform,
      social_account_id: accountFilter,
      request_id: requestFilter,
      post_id: postFilter,
      error_code: errorCodeFilter,
      status,
      source,
      from: range.from,
      to: range.to,
      limit: 150,
    };
  }, [accountFilter, category, errorCodeFilter, platform, postFilter, query, requestFilter, source, status, timeRange]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token) return;
        const profilesRes = await listProfiles(token);
        if (cancelled) return;
        setProfiles(profilesRes.data);
        try {
          const billingRes = await getBilling(token);
          if (!cancelled) setBilling(billingRes.data);
        } catch {
          // best effort
        }

        const accountRows = await Promise.all(
          profilesRes.data.map(async (profile) => {
            try {
              const res = await listSocialAccounts(token, profile.id);
              return res.data;
            } catch {
              return [] as SocialAccount[];
            }
          })
        );
        if (cancelled) return;
        setAccounts(accountRows.flat());
      } catch {
        if (cancelled) return;
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [getToken]);

  useEffect(() => {
    let cancelled = false;
    const timer = window.setTimeout(async () => {
      try {
        setLoading(true);
        setError(null);
        const token = await getToken();
        if (!token) return;
        const res = await listIntegrationLogs(token, params);
        if (cancelled) return;
        setLogs(res.data);
      } catch (err) {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Failed to load logs");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }, 220);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [getToken, params]);

  const refresh = async () => {
    try {
      setRefreshing(true);
      setError(null);
      const token = await getToken();
      if (!token) return;
      const res = await listIntegrationLogs(token, params);
      setLogs(res.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to refresh logs");
    } finally {
      setRefreshing(false);
    }
  };

  const openLog = async (id: number) => {
    try {
      setSelectedLogId(id);
      setSelectedLog(null);
      setDetailLoading(true);
      const token = await getToken();
      if (!token) return;
      const res = await getIntegrationLog(token, id);
      setSelectedLog(res.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load log detail");
    } finally {
      setDetailLoading(false);
    }
  };

  const selectedTone = selectedLog ? toneForStatus(selectedLog.status) : null;
  const errorCount = logs.filter((log) => log.status === "error").length;
  const warningCount = logs.filter((log) => log.status === "warning").length;
  const publishCount = logs.filter((log) => log.category === "publishing").length;
  const latestLog = logs[0];
  const retentionDays = retentionDaysForPlan(billing?.plan);

  const liveMatchesFilters = (log: IntegrationLog) => {
    if (category !== "all" && log.category !== category) return false;
    if (platform !== "all" && (log.platform || accountById.get(log.social_account_id || "")?.platform) !== platform) return false;
    if (accountFilter !== "all" && log.social_account_id !== accountFilter) return false;
    if (status !== "all" && log.status !== status) return false;
    if (source !== "all" && log.source !== source) return false;
    if (requestFilter && log.request_id !== requestFilter) return false;
    if (postFilter && log.post_id !== postFilter) return false;
    if (errorCodeFilter && log.error_code !== errorCodeFilter) return false;
    if (query.trim()) {
      const haystack = [
        log.message,
        log.action,
        log.request_id,
        log.post_id,
        log.error_code,
      ]
        .filter(Boolean)
        .join(" ")
        .toLowerCase();
      if (!haystack.includes(query.trim().toLowerCase())) return false;
    }
    return true;
  };

  const { connected: liveConnected } = useLogsWebSocket(liveMode, (log) => {
    if (!liveMatchesFilters(log)) return;
    setLogs((prev) => {
      const next = [log, ...prev.filter((item) => item.id !== log.id)];
      return next.slice(0, 150);
    });
  });

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 18 }}>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 16 }}>
        <div>
          <div className="dt-page-title">Logs</div>
          <div className="dt-body-sm" style={{ marginTop: 6, color: "var(--dmuted)" }}>
            Search workspace activity, publishing events, and integration failures. Results are isolated to this workspace.
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <button
            type="button"
            onClick={() => setLiveMode((value) => !value)}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 8,
              padding: "10px 14px",
              borderRadius: 12,
              border: liveMode ? "1px solid color-mix(in srgb, #10b981 28%, var(--dborder))" : "1px solid var(--dborder)",
              background: liveMode ? "color-mix(in srgb, #10b981 12%, var(--surface))" : "var(--surface)",
              color: "var(--dtext)",
              cursor: "pointer",
            }}
          >
            {liveMode ? <PauseCircle style={{ width: 14, height: 14 }} /> : <Radio style={{ width: 14, height: 14 }} />}
            <span className="dt-body-sm" style={{ color: "var(--dtext)", fontWeight: 600 }}>
              {liveMode ? "Pause live tail" : "Live tail"}
            </span>
          </button>
          <button
            type="button"
            onClick={refresh}
            disabled={refreshing}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 8,
              padding: "10px 14px",
              borderRadius: 12,
              border: "1px solid var(--dborder)",
              background: "var(--surface)",
              color: "var(--dtext)",
              cursor: "pointer",
            }}
          >
            <RefreshCw style={{ width: 14, height: 14 }} className={refreshing ? "animate-spin" : ""} />
            <span className="dt-body-sm" style={{ color: "var(--dtext)", fontWeight: 600 }}>
              Refresh
            </span>
          </button>
        </div>
      </div>

      <div
        style={{
          display: "grid",
          gridTemplateColumns: "minmax(220px, 1.6fr) repeat(6, minmax(0, 1fr))",
          gap: 10,
          padding: 14,
          borderRadius: 16,
          border: "1px solid var(--dborder)",
          background: "var(--surface)",
        }}
      >
        <label style={{ position: "relative" }}>
          <Search style={{ position: "absolute", left: 12, top: 11, width: 14, height: 14, color: "var(--dmuted2)" }} />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search logs, request IDs, or errors"
            style={inputStyle({ paddingLeft: 36 })}
          />
        </label>
        <select value={category} onChange={(e) => setCategory(e.target.value)} style={inputStyle()}>
          {CATEGORY_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </select>
        <select value={platform} onChange={(e) => setPlatform(e.target.value)} style={inputStyle()}>
          {PLATFORM_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </select>
        <select value={accountFilter} onChange={(e) => setAccountFilter(e.target.value)} style={inputStyle()}>
          {accountOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </select>
        <select value={status} onChange={(e) => setStatus(e.target.value)} style={inputStyle()}>
          {STATUS_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </select>
        <select value={source} onChange={(e) => setSource(e.target.value)} style={inputStyle()}>
          {SOURCE_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </select>
        <select value={timeRange} onChange={(e) => setTimeRange(e.target.value as TimeRangeKey)} style={inputStyle()}>
          {TIME_RANGE_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </select>
      </div>

      <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
        {query.trim() && <ActiveFilter label="search" value={query.trim()} onClear={() => setQuery("")} />}
        {requestFilter && <ActiveFilter label="request_id" value={requestFilter} onClear={() => setRequestFilter("")} />}
        {postFilter && <ActiveFilter label="post_id" value={postFilter} onClear={() => setPostFilter("")} />}
        {errorCodeFilter && <ActiveFilter label="error_code" value={errorCodeFilter} onClear={() => setErrorCodeFilter("")} />}
        {category !== "all" && <ActiveFilter label="category" value={category} onClear={() => setCategory("all")} />}
        {platform !== "all" && <ActiveFilter label="platform" value={platform} onClear={() => setPlatform("all")} />}
        {accountFilter !== "all" && (
          <ActiveFilter
            label="account"
            value={accountById.get(accountFilter)?.account_name || accountFilter}
            onClear={() => setAccountFilter("all")}
          />
        )}
        {status !== "all" && <ActiveFilter label="status" value={status} onClear={() => setStatus("all")} />}
        {source !== "all" && <ActiveFilter label="source" value={source} onClear={() => setSource("all")} />}
      </div>

      <div className="stat-grid" style={{ gridTemplateColumns: "repeat(4, minmax(0, 1fr))", marginTop: -4 }}>
        <div className="stat-card">
          <div className="dt-label" style={{ marginBottom: 8 }}>Errors</div>
          <div style={statValueStyle}>{errorCount}</div>
          <div className="dt-micro" style={{ marginTop: 4 }}>Current filtered set</div>
        </div>
        <div className="stat-card">
          <div className="dt-label" style={{ marginBottom: 8 }}>Warnings</div>
          <div style={statValueStyle}>{warningCount}</div>
          <div className="dt-micro" style={{ marginTop: 4 }}>Current filtered set</div>
        </div>
        <div className="stat-card">
          <div className="dt-label" style={{ marginBottom: 8 }}>Publishing events</div>
          <div style={statValueStyle}>{publishCount}</div>
          <div className="dt-micro" style={{ marginTop: 4 }}>Within selected range</div>
        </div>
        <div className="stat-card">
          <div className="dt-label" style={{ marginBottom: 8 }}>Retention</div>
          <div style={statValueStyle}>{retentionDays}d</div>
          <div className="dt-micro" style={{ marginTop: 4 }}>
            {billing ? `${billing.plan_name} plan history window` : "Based on current workspace plan"}
          </div>
        </div>
      </div>

      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 12,
          padding: "12px 14px",
          borderRadius: 14,
          border: "1px solid var(--dborder)",
          background: "color-mix(in srgb, var(--sidebar) 72%, var(--surface))",
        }}
      >
        <div className="dt-body-sm" style={{ color: "var(--dmuted)" }}>
          Workspace-scoped logs only. Use field filters to narrow by category, source, platform, and time range.
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          {latestLog && !loading && (
            <div className="dt-body-sm" style={{ color: "var(--dmuted2)" }}>
              Latest event {relativeTimeLabel(latestLog.ts)}
            </div>
          )}
          {liveMode && (
            <div className="dt-body-sm" style={{ color: liveConnected ? "#10b981" : "var(--dmuted2)", fontWeight: 600 }}>
              {liveConnected ? "Live tail connected" : "Connecting live tail…"}
            </div>
          )}
          <div className="dt-body-sm" style={{ color: "var(--dmuted2)" }}>
            {logs.length} row{logs.length === 1 ? "" : "s"}
          </div>
        </div>
      </div>

      <div
        style={{
          borderRadius: 18,
          border: "1px solid var(--dborder)",
          background: "var(--surface)",
          overflow: "hidden",
        }}
      >
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "170px 120px 210px 110px 220px 1fr 220px",
            gap: 12,
            padding: "14px 18px",
            borderBottom: "1px solid var(--dborder)",
            background: "color-mix(in srgb, var(--surface2) 88%, var(--surface))",
          }}
        >
          {["Time", "Category", "Action", "Status", "Account", "Summary", "Request / Post"].map((label) => (
            <div key={label} className="dt-label">{label}</div>
          ))}
        </div>

        {loading ? (
          <div style={emptyStateStyle}>
            <FileText style={{ width: 26, height: 26, color: "var(--dmuted2)" }} />
            <div className="dt-card-title" style={{ marginTop: 10 }}>Loading logs</div>
            <div className="dt-body-sm" style={{ marginTop: 6, color: "var(--dmuted)" }}>
              Fetching the latest workspace events.
            </div>
          </div>
        ) : error ? (
          <div style={emptyStateStyle}>
            <div className="dt-card-title" style={{ color: "#ef4444" }}>Failed to load logs</div>
            <div className="dt-body-sm" style={{ marginTop: 6, color: "var(--dmuted)" }}>{error}</div>
          </div>
        ) : logs.length === 0 ? (
          <div style={emptyStateStyle}>
            <FileText style={{ width: 26, height: 26, color: "var(--dmuted2)" }} />
            <div className="dt-card-title" style={{ marginTop: 10 }}>No logs found</div>
            <div className="dt-body-sm" style={{ marginTop: 6, color: "var(--dmuted)" }}>
              Try broadening the time range or removing one of the filters.
            </div>
          </div>
        ) : (
          <div>
            {logs.map((log) => {
              const tone = toneForStatus(log.status);
              return (
                <button
                  key={log.id}
                  type="button"
                  onClick={() => openLog(log.id)}
                  style={{
                    width: "100%",
                    display: "grid",
                    gridTemplateColumns: "170px 120px 210px 110px 220px 1fr 220px",
                    gap: 12,
                    padding: "13px 18px",
                    border: "none",
                    borderBottom: "1px solid var(--dborder)",
                    background: selectedLogId === log.id ? "color-mix(in srgb, var(--accent-glow) 60%, var(--surface))" : "transparent",
                    textAlign: "left",
                    cursor: "pointer",
                  }}
                >
                  <div style={{ minWidth: 0 }}>
                    <div className="dt-body-sm" style={{ color: "var(--dtext)" }}>{formatTimestamp(log.ts)}</div>
                    <div className="dt-micro" style={{ color: "var(--dmuted2)", marginTop: 2 }}>{relativeTimeLabel(log.ts)}</div>
                  </div>
                  <div className="dt-body-sm" style={{ color: "var(--dtext)", textTransform: "capitalize" }}>{log.category.replaceAll("_", " ")}</div>
                  <div className="dt-body-sm" style={{ color: "var(--dtext)", fontFamily: "var(--font-geist-mono), monospace", fontSize: 12 }}>
                    {log.action}
                  </div>
                  <div>
                    <span
                      style={{
                        display: "inline-flex",
                        alignItems: "center",
                        padding: "4px 8px",
                        borderRadius: 999,
                        border: `1px solid ${tone.border}`,
                        background: tone.bg,
                        color: tone.fg,
                        fontSize: 12,
                        fontWeight: 700,
                        textTransform: "capitalize",
                      }}
                    >
                      {log.status}
                    </span>
                  </div>
                  <div style={{ minWidth: 0 }}>
                    <div className="dt-body-sm" style={{ color: "var(--dtext)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                      {log.social_account_id ? (accountById.get(log.social_account_id)?.account_name || log.social_account_id) : "—"}
                    </div>
                    <div className="dt-micro" style={{ color: "var(--dmuted2)", marginTop: 2, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                      {log.platform || accountById.get(log.social_account_id || "")?.platform || "workspace"}
                    </div>
                  </div>
                  <div className="dt-body-sm" style={{ color: "var(--dmuted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {log.message}
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted2)", minWidth: 0 }}>
                    <span className="dt-body-sm" style={{ fontFamily: "var(--font-geist-mono), monospace", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                      {log.request_id || log.post_id || "—"}
                    </span>
                    <ChevronRight style={{ width: 14, height: 14, flexShrink: 0 }} />
                  </div>
                </button>
              );
            })}
          </div>
        )}
      </div>

      {selectedLogId !== null && (
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
              width: 480,
              maxWidth: "96vw",
              background: "var(--surface-raised)",
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
                <div className="dt-card-title">Log detail</div>
                <div className="dt-body-sm" style={{ marginTop: 4, color: "var(--dmuted)" }}>
                  {selectedLog ? selectedLog.action : "Loading detail…"}
                </div>
              </div>
              <button
                type="button"
                onClick={closeDetail}
                style={{
                  border: "1px solid var(--dborder)",
                  background: "var(--surface)",
                  borderRadius: 10,
                  padding: 8,
                  cursor: "pointer",
                  color: "var(--dmuted2)",
                }}
              >
                <X style={{ width: 16, height: 16 }} />
              </button>
            </div>

            {detailLoading || !selectedLog ? (
              <div className="dt-body-sm" style={{ color: "var(--dmuted)" }}>Loading detail…</div>
            ) : (
              <>
                <div
                  style={{
                    borderRadius: 16,
                    border: `1px solid ${selectedTone?.border}`,
                    background: selectedTone?.bg,
                    padding: 16,
                  }}
                >
                  <div className="dt-label" style={{ marginBottom: 8 }}>Summary</div>
                  <div className="dt-body-sm" style={{ color: "var(--dtext)", fontWeight: 600 }}>{selectedLog.message}</div>
                  <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 12 }}>
                    <FieldChip label="status" value={selectedLog.status} />
                    <FieldChip label="level" value={selectedLog.level} />
                    <FieldChip label="source" value={selectedLog.source} />
                    <FieldChip label="time" value={formatTimestamp(selectedLog.ts)} />
                  </div>
                </div>

                <section style={sectionStyle}>
                  <div className="dt-label" style={{ marginBottom: 10 }}>Correlation</div>
                  <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                    {selectedLog.request_id && (
                      <FieldChip
                        label="request_id"
                        value={selectedLog.request_id}
                        onClick={() => applyFieldFilter("request", selectedLog.request_id || "")}
                      />
                    )}
                    {selectedLog.post_id && (
                      <FieldChip
                        label="post_id"
                        value={selectedLog.post_id}
                        onClick={() => applyFieldFilter("post", selectedLog.post_id || "")}
                      />
                    )}
                    {selectedLog.social_account_id && (
                      <FieldChip
                        label="account"
                        value={selectedLog.social_account_id}
                        onClick={() => applyFieldFilter("account", selectedLog.social_account_id || "")}
                      />
                    )}
                    {selectedLog.profile_id && <FieldChip label="profile" value={selectedLog.profile_id} />}
                    {selectedLog.platform && (
                      <FieldChip
                        label="platform"
                        value={selectedLog.platform}
                        onClick={() => applyFieldFilter("platform", selectedLog.platform || "")}
                      />
                    )}
                    {selectedLog.error_code && (
                      <FieldChip
                        label="error_code"
                        value={selectedLog.error_code}
                        onClick={() => applyFieldFilter("error", selectedLog.error_code || "")}
                      />
                    )}
                  </div>
                </section>

                {(selectedLog.profile_id || selectedLog.social_account_id) && (
                  <section style={sectionStyle}>
                    <div className="dt-label" style={{ marginBottom: 10 }}>Resolved entities</div>
                    <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
                      {selectedLog.profile_id && (
                        <KeyValue
                          label="Profile"
                          value={profileNameById.get(selectedLog.profile_id) || selectedLog.profile_id}
                        />
                      )}
                      {selectedLog.social_account_id && (
                        <KeyValue
                          label="Account"
                          value={accountById.get(selectedLog.social_account_id)?.account_name || selectedLog.social_account_id}
                        />
                      )}
                    </div>
                  </section>
                )}

                {(selectedLog.endpoint || selectedLog.method || selectedLog.http_status_code || selectedLog.remote_status_code || selectedLog.duration_ms) && (
                  <section style={sectionStyle}>
                    <div className="dt-label" style={{ marginBottom: 10 }}>Request / HTTP</div>
                    <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
                      {selectedLog.endpoint && <KeyValue label="Endpoint" value={selectedLog.endpoint} />}
                      {selectedLog.method && <KeyValue label="Method" value={selectedLog.method} />}
                      {selectedLog.http_status_code !== undefined && <KeyValue label="HTTP status" value={String(selectedLog.http_status_code)} />}
                      {selectedLog.remote_status_code !== undefined && <KeyValue label="Remote status" value={String(selectedLog.remote_status_code)} />}
                      {selectedLog.duration_ms !== undefined && <KeyValue label="Duration" value={`${selectedLog.duration_ms} ms`} />}
                    </div>
                  </section>
                )}

                <section style={sectionStyle}>
                  <div className="dt-label" style={{ marginBottom: 10 }}>Metadata</div>
                  <JSONBlock value={selectedLog.metadata} />
                </section>

                <section style={sectionStyle}>
                  <div className="dt-label" style={{ marginBottom: 10 }}>Request</div>
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
                          <div className="dt-label" style={{ marginBottom: 8 }}>Headers</div>
                          <JSONBlock value={envelope.headers} />
                        </div>
                        <div>
                          <div className="dt-label" style={{ marginBottom: 8 }}>Query</div>
                          <JSONBlock value={envelope.query} />
                        </div>
                        <div>
                          <div className="dt-label" style={{ marginBottom: 8 }}>Payload</div>
                          <JSONBlock value={envelope.payload} />
                        </div>
                      </div>
                    );
                  })()}
                </section>

                <section style={sectionStyle}>
                  <div className="dt-label" style={{ marginBottom: 10 }}>Response</div>
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
                          <div className="dt-label" style={{ marginBottom: 8 }}>Headers</div>
                          <JSONBlock value={envelope.headers} />
                        </div>
                        <div>
                          <div className="dt-label" style={{ marginBottom: 8 }}>Payload</div>
                          <JSONBlock value={envelope.payload} />
                        </div>
                      </div>
                    );
                  })()}
                </section>

                {selectedLog.post_id && (
                  <a
                    href={`/projects/${profileId}/posts?post=${selectedLog.post_id}`}
                    style={{
                      display: "inline-flex",
                      alignItems: "center",
                      gap: 8,
                      textDecoration: "none",
                      color: "var(--dlink, var(--primary))",
                      fontSize: 14,
                      fontWeight: 600,
                    }}
                  >
                    Open related posts
                    <ExternalLink style={{ width: 14, height: 14 }} />
                  </a>
                )}
              </>
            )}
          </aside>
        </>
      )}
    </div>
  );
}

function KeyValue({ label, value }: { label: string; value: string }) {
  return (
    <div
      style={{
        padding: "10px 12px",
        borderRadius: 12,
        border: "1px solid var(--dborder)",
        background: "var(--surface2)",
      }}
    >
      <div className="dt-label" style={{ marginBottom: 6 }}>{label}</div>
      <div className="dt-body-sm" style={{ color: "var(--dtext)", fontFamily: "var(--font-geist-mono), monospace" }}>{value}</div>
    </div>
  );
}

function ActiveFilter({
  label,
  value,
  onClear,
}: {
  label: string;
  value: string;
  onClear: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClear}
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 8,
        padding: "6px 10px",
        borderRadius: 999,
        border: "1px solid var(--dborder)",
        background: "color-mix(in srgb, var(--accent-glow) 58%, var(--surface))",
        color: "var(--dtext)",
        cursor: "pointer",
      }}
      title="Clear filter"
    >
      <span style={{ color: "var(--dmuted2)", fontSize: 12 }}>{label}</span>
      <span style={{ fontSize: 12, fontWeight: 600 }}>{value}</span>
      <X style={{ width: 12, height: 12, color: "var(--dmuted2)" }} />
    </button>
  );
}

function inputStyle(extra?: CSSProperties): CSSProperties {
  return {
    width: "100%",
    height: 38,
    borderRadius: 12,
    border: "1px solid var(--dborder)",
    background: "var(--surface2)",
    color: "var(--dtext)",
    padding: "0 12px",
    fontSize: 14,
    outline: "none",
    ...extra,
  };
}

const emptyStateStyle: CSSProperties = {
  display: "flex",
  minHeight: 340,
  alignItems: "center",
  justifyContent: "center",
  flexDirection: "column",
  padding: 24,
};

const sectionStyle: CSSProperties = {
  display: "flex",
  flexDirection: "column",
  gap: 8,
};

const statValueStyle: CSSProperties = {
  fontFamily: "var(--font-geist-mono), monospace",
  fontSize: 22,
  fontWeight: 600,
  letterSpacing: -0.5,
  color: "var(--dtext)",
};
