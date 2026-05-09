"use client";

import { type CSSProperties, useEffect, useMemo, useState } from "react";
import { useParams, usePathname, useRouter, useSearchParams } from "next/navigation";
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
  Check,
  Copy,
  FileText,
  RefreshCw,
  Search,
  X,
  ExternalLink,
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

export default function LogsPage() {
  const { id: profileId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const [query, setQuery] = useState("");
  const [category, setCategory] = useState(() => searchParams.get("category") || "all");
  const [platform, setPlatform] = useState(() => searchParams.get("platform") || "all");
  const [accountFilter, setAccountFilter] = useState(() => searchParams.get("account") || "all");
  const [requestFilter, setRequestFilter] = useState(() => searchParams.get("request_id") || "");
  const [postFilter, setPostFilter] = useState(() => searchParams.get("post_id") || "");
  const [errorCodeFilter, setErrorCodeFilter] = useState(() => searchParams.get("error_code") || "");
  const [status, setStatus] = useState(() => searchParams.get("status") || "all");
  const [source, setSource] = useState(() => searchParams.get("source") || "all");
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
  const [drawerTab, setDrawerTab] = useState<"attributes" | "raw">("attributes");
  const [rawCopied, setRawCopied] = useState(false);

  const closeDetail = () => {
    setSelectedLogId(null);
    setSelectedLog(null);
  };

  const hasActiveFilter =
    Boolean(query.trim()) ||
    Boolean(requestFilter) ||
    Boolean(postFilter) ||
    Boolean(errorCodeFilter) ||
    category !== "all" ||
    platform !== "all" ||
    accountFilter !== "all" ||
    status !== "all" ||
    source !== "all";

  const resetFilters = () => {
    setQuery("");
    setRequestFilter("");
    setPostFilter("");
    setErrorCodeFilter("");
    setCategory("all");
    setPlatform("all");
    setAccountFilter("all");
    setStatus("all");
    setSource("all");
  };

  // Sync filter state into URL search params so the view is shareable
  // and survives a page refresh. Free-text search and time range stay
  // in-memory: search is transient input; time range is a relative value
  // that auto-recomputes per session.
  useEffect(() => {
    const params = new URLSearchParams();
    if (category !== "all") params.set("category", category);
    if (platform !== "all") params.set("platform", platform);
    if (accountFilter !== "all") params.set("account", accountFilter);
    if (status !== "all") params.set("status", status);
    if (source !== "all") params.set("source", source);
    if (requestFilter) params.set("request_id", requestFilter);
    if (postFilter) params.set("post_id", postFilter);
    if (errorCodeFilter) params.set("error_code", errorCodeFilter);
    const qs = params.toString();
    const target = qs ? `${pathname}?${qs}` : pathname;
    if (target !== `${pathname}${window.location.search}`) {
      router.replace(target, { scroll: false });
    }
  }, [category, platform, accountFilter, status, source, requestFilter, postFilter, errorCodeFilter, pathname, router]);

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
      setDrawerTab("attributes");
      setRawCopied(false);
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

  const copyRawLog = async () => {
    if (!selectedLog) return;
    try {
      await navigator.clipboard.writeText(JSON.stringify(selectedLog, null, 2));
      setRawCopied(true);
      window.setTimeout(() => setRawCopied(false), 1600);
    } catch {
      // ignore — clipboard permission denied or unavailable
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
    <div className="logs-page-fullheight">
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 16 }}>
        <div className="dt-page-title">Logs</div>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <button
            type="button"
            onClick={() => setLiveMode((value) => !value)}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 6,
              padding: "6px 12px",
              borderRadius: 10,
              border: liveMode ? "1px solid color-mix(in srgb, #10b981 28%, var(--dborder))" : "1px solid var(--dborder)",
              background: liveMode ? "color-mix(in srgb, #10b981 12%, var(--surface))" : "var(--surface)",
              color: "var(--dtext)",
              cursor: "pointer",
              fontSize: 13,
            }}
          >
            {liveMode ? <PauseCircle style={{ width: 13, height: 13 }} /> : <Radio style={{ width: 13, height: 13 }} />}
            <span style={{ color: "var(--dtext)", fontWeight: 600 }}>
              {liveMode ? "Pause" : "Live tail"}
            </span>
          </button>
          <button
            type="button"
            onClick={refresh}
            disabled={refreshing}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 6,
              padding: "6px 12px",
              borderRadius: 10,
              border: "1px solid var(--dborder)",
              background: "var(--surface)",
              color: "var(--dtext)",
              cursor: "pointer",
              fontSize: 13,
            }}
          >
            <RefreshCw style={{ width: 13, height: 13 }} className={refreshing ? "animate-spin" : ""} />
            <span style={{ color: "var(--dtext)", fontWeight: 600 }}>
              Refresh
            </span>
          </button>
        </div>
      </div>

      <div
        style={{
          display: "grid",
          gridTemplateColumns: "minmax(220px, 1.6fr) repeat(6, minmax(0, 1fr))",
          gap: 8,
          padding: 8,
          borderRadius: 12,
          border: "1px solid var(--dborder)",
          background: "var(--surface)",
        }}
      >
        <label style={{ position: "relative" }}>
          <Search style={{ position: "absolute", left: 12, top: 9, width: 14, height: 14, color: "var(--dmuted2)" }} />
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

      {hasActiveFilter && (
        <div style={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 6 }}>
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
          <button
            type="button"
            onClick={resetFilters}
            style={{
              marginLeft: 4,
              border: "none",
              background: "transparent",
              color: "var(--dmuted2)",
              fontSize: 12,
              fontWeight: 600,
              cursor: "pointer",
              textDecoration: "underline",
              padding: "2px 4px",
            }}
          >
            Clear all
          </button>
        </div>
      )}

      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 12,
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
          <MiniStat label="Errors" value={String(errorCount)} tone="error" />
          <MiniStat label="Warnings" value={String(warningCount)} tone="warning" />
          <MiniStat label="Publishing" value={String(publishCount)} />
          <MiniStat label="Retention" value={`${retentionDays}d`} />
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 10, fontSize: 12, color: "var(--dmuted2)" }}>
          {latestLog && !loading && (
            <span>Latest event {relativeTimeLabel(latestLog.ts)}</span>
          )}
          {liveMode && (
            <span style={{ color: liveConnected ? "#10b981" : "var(--dmuted2)", fontWeight: 600 }}>
              {liveConnected ? "Live tail connected" : "Connecting live tail…"}
            </span>
          )}
          <span>{logs.length} row{logs.length === 1 ? "" : "s"}</span>
        </div>
      </div>

      <div
        style={{
          flex: 1,
          minHeight: 0,
          display: "flex",
          flexDirection: "column",
          borderRadius: 22,
          border: CONSOLE_FRAME_BORDER,
          background: CONSOLE_FRAME_BACKGROUND,
          overflow: "hidden",
          position: "relative",
          boxShadow: "0 18px 50px color-mix(in srgb, var(--sidebar) 18%, transparent)",
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 16,
            padding: "14px 18px",
            borderBottom: CONSOLE_HEADER_BORDER,
            background: CONSOLE_HEADER_BACKGROUND,
            flexShrink: 0,
          }}
        >
          <div style={{ color: CONSOLE_TEXT_PRIMARY, fontSize: 14, fontWeight: 700 }}>Log stream</div>
          <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap", justifyContent: "flex-end" }}>
            <ConsoleBadge label={liveMode ? (liveConnected ? "live connected" : "connecting…") : "history mode"} tone={liveMode && liveConnected ? "success" : "neutral"} />
            <ConsoleBadge label={`${logs.length} rows`} tone="neutral" />
          </div>
        </div>

        <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
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
              const accountName = log.social_account_id ? (accountById.get(log.social_account_id)?.account_name || log.social_account_id) : "workspace";
              const requestRef = log.request_id || log.post_id || "—";
              return (
                <button
                  key={log.id}
                  type="button"
                  onClick={() => openLog(log.id)}
                  style={{
                    width: "100%",
                    display: "grid",
                    gridTemplateColumns: "170px minmax(0, 1fr)",
                    gap: 16,
                    padding: "10px 16px",
                    border: "none",
                    borderBottom: CONSOLE_ROW_BORDER,
                    background: selectedLogId === log.id ? CONSOLE_SELECTED_BG : "transparent",
                    textAlign: "left",
                    cursor: "pointer",
                  }}
                >
                  <div style={{ minWidth: 0, position: "relative", paddingLeft: 14 }}>
                    <span
                      style={{
                        position: "absolute",
                        left: 0,
                        top: 2,
                        bottom: 2,
                        width: 3,
                        borderRadius: 999,
                        background: tone.fg,
                      }}
                    />
                    <div style={{ color: CONSOLE_TEXT_PRIMARY, fontSize: 12.5 }}>{formatTimestamp(log.ts)}</div>
                    <div style={{ color: CONSOLE_TEXT_SUBTLE, fontSize: 11.5, marginTop: 2 }}>{relativeTimeLabel(log.ts)}</div>
                  </div>
                  <div style={{ minWidth: 0, display: "grid", gap: 6 }}>
                    <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                      <div style={{ minWidth: 0, color: CONSOLE_TEXT_PRIMARY, fontSize: 14, fontWeight: 600, fontFamily: "var(--font-geist-mono), monospace", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                        {log.action}
                      </div>
                      <span
                        style={{
                          display: "inline-flex",
                          alignItems: "center",
                          padding: "4px 8px",
                          borderRadius: 999,
                          border: `1px solid ${tone.border}`,
                          background: tone.bg,
                          color: tone.fg,
                          fontSize: 11,
                          fontWeight: 700,
                          textTransform: "capitalize",
                          flexShrink: 0,
                        }}
                      >
                        {log.status}
                      </span>
                    </div>
                    <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
                      <ConsoleBadge label={log.category.replaceAll("_", " ")} tone="neutral" />
                      <ConsoleBadge label={log.platform || "workspace"} tone="neutral" />
                      <ConsoleBadge label={accountName} tone="neutral" />
                      <ConsoleBadge label={requestRef} tone="neutral" />
                    </div>
                  </div>
                </button>
              );
            })}
          </div>
        )}
        </div>

        {selectedLogId !== null && (
          <aside
            className="logs-detail-drawer"
            role="dialog"
            aria-label="Log detail"
            style={{
              position: "absolute",
              top: 0,
              right: 0,
              bottom: 0,
              width: "45%",
              minWidth: 360,
              background: "var(--surface-raised, var(--surface))",
              borderLeft: "1px solid var(--dborder)",
              zIndex: 5,
              overflowY: "auto",
              padding: 18,
              display: "flex",
              flexDirection: "column",
              gap: 14,
              boxShadow: "-18px 0 44px color-mix(in srgb, var(--sidebar) 28%, transparent)",
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
                <DrawerTabs
                  active={drawerTab}
                  onChange={setDrawerTab}
                  rightSlot={
                    drawerTab === "raw" ? (
                      <button
                        type="button"
                        onClick={copyRawLog}
                        style={drawerCopyButtonStyle}
                        aria-label="Copy raw JSON"
                      >
                        {rawCopied ? (
                          <>
                            <Check style={{ width: 12, height: 12 }} />
                            Copied
                          </>
                        ) : (
                          <>
                            <Copy style={{ width: 12, height: 12 }} />
                            Copy
                          </>
                        )}
                      </button>
                    ) : null
                  }
                />
                {drawerTab === "raw" ? (
                  <pre style={drawerRawJsonStyle}>{JSON.stringify(selectedLog, null, 2)}</pre>
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
              </>
            )}
          </aside>
        )}
      </div>
    </div>
  );
}

function DrawerTabs({
  active,
  onChange,
  rightSlot,
}: {
  active: "attributes" | "raw";
  onChange: (next: "attributes" | "raw") => void;
  rightSlot?: React.ReactNode;
}) {
  return (
    <div style={drawerTabBarStyle}>
      <div style={{ display: "flex", gap: 4 }}>
        <button type="button" onClick={() => onChange("attributes")} style={drawerTabButtonStyle(active === "attributes")}>
          Attributes
        </button>
        <button type="button" onClick={() => onChange("raw")} style={drawerTabButtonStyle(active === "raw")}>
          Raw Data
        </button>
      </div>
      {rightSlot}
    </div>
  );
}

const drawerTabBarStyle: CSSProperties = {
  display: "flex",
  alignItems: "center",
  justifyContent: "space-between",
  gap: 8,
  borderBottom: "1px solid var(--dborder)",
  paddingBottom: 8,
};

function drawerTabButtonStyle(active: boolean): CSSProperties {
  return {
    background: "transparent",
    border: "none",
    padding: "6px 4px",
    fontSize: 13,
    fontWeight: 600,
    color: active ? "var(--dtext)" : "var(--dmuted2)",
    borderBottom: active ? "2px solid var(--accent)" : "2px solid transparent",
    cursor: "pointer",
    marginBottom: -9,
  };
}

const drawerCopyButtonStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 6,
  padding: "4px 10px",
  borderRadius: 8,
  border: "1px solid var(--dborder)",
  background: "var(--surface)",
  color: "var(--dtext)",
  fontSize: 12,
  fontWeight: 600,
  cursor: "pointer",
};

const drawerRawJsonStyle: CSSProperties = {
  margin: 0,
  padding: 14,
  borderRadius: 12,
  border: DRAWER_PANEL_BORDER,
  background: DRAWER_CODE_BACKGROUND,
  color: "var(--dtext)",
  fontSize: 12,
  lineHeight: 1.6,
  overflow: "auto",
  whiteSpace: "pre",
  fontFamily: "var(--font-geist-mono), monospace",
};

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
    height: 32,
    borderRadius: 10,
    border: "1px solid var(--dborder)",
    background: "var(--surface2)",
    color: "var(--dtext)",
    padding: "0 10px",
    fontSize: 13,
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
  gap: 10,
  padding: 14,
  borderRadius: 16,
  border: DRAWER_PANEL_BORDER,
  background: DRAWER_PANEL_BACKGROUND,
};

const statValueStyle: CSSProperties = {
  fontFamily: "var(--font-geist-mono), monospace",
  fontSize: 22,
  fontWeight: 600,
  letterSpacing: -0.5,
  color: "var(--dtext)",
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
        gap: 6,
        padding: "4px 9px",
        borderRadius: 999,
        border: `1px solid ${colors.border}`,
        background: colors.bg,
        fontSize: 11,
      }}
    >
      <span style={{ color: "var(--dmuted2)", textTransform: "uppercase", letterSpacing: "0.08em" }}>{label}</span>
      <span style={{ color: colors.fg, fontFamily: "var(--font-geist-mono), monospace", fontWeight: 700, fontSize: 12 }}>{value}</span>
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
