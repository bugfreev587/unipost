"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Check, Copy, ExternalLink, X } from "lucide-react";
import {
  type CSSProperties,
  type KeyboardEvent,
  type MouseEvent,
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";

import {
  listAdminPostFailures,
  type AdminPostFailureListParams,
  type AdminUserPostFailure,
} from "@/lib/api";

import { AdminShell, StatCard, fmtNumber, fmtRelative } from "../_components/admin-ui";
import { SearchHistoryInput } from "../_components/search-history-input";

const PLATFORM_OPTIONS = ["all", "twitter", "linkedin", "instagram", "threads", "tiktok", "youtube", "bluesky"] as const;
const SOURCE_OPTIONS = ["all", "dashboard", "api", "mcp"] as const;
const RANGE_OPTIONS = ["this_month", "7", "30", "90"] as const;
type FailureRange = typeof RANGE_OPTIONS[number];
type URLParamReader = {
  get(name: string): string | null;
};

function filtersFromURLParams(params: URLParamReader) {
  const platformParam = params.get("platform");
  const sourceParam = params.get("source");
  const daysParam = params.get("days");
  const periodIsThisMonth = params.get("period") === "this_month";
  const range = periodIsThisMonth
    ? "this_month"
    : RANGE_OPTIONS.includes(daysParam as FailureRange) && daysParam !== "this_month"
      ? daysParam as FailureRange
      : "30";
  return {
    search: params.get("search") || "",
    userId: params.get("user_id") || "",
    platform: PLATFORM_OPTIONS.includes(platformParam as typeof PLATFORM_OPTIONS[number])
      ? platformParam as typeof PLATFORM_OPTIONS[number]
      : "all",
    source: SOURCE_OPTIONS.includes(sourceParam as typeof SOURCE_OPTIONS[number])
      ? sourceParam as typeof SOURCE_OPTIONS[number]
      : "all",
    range,
  };
}

function initialFiltersFromURL() {
  if (typeof window === "undefined") {
    return { search: "", platform: "all" as const, source: "all" as const, range: "30" as FailureRange, userId: "" };
  }
  return filtersFromURLParams(new URLSearchParams(window.location.search));
}

function AdminErrorsContent() {
  const { getToken } = useAuth();
  const searchParams = useSearchParams();
  const [failures, setFailures] = useState<AdminUserPostFailure[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [initialFilters] = useState(() => initialFiltersFromURL());
  const [search, setSearch] = useState(initialFilters.search);
  const [searchInput, setSearchInput] = useState(initialFilters.search);
  const [userIdFilter, setUserIdFilter] = useState(initialFilters.userId);
  const [platform, setPlatform] = useState(initialFilters.platform);
  const [source, setSource] = useState(initialFilters.source);
  const [range, setRange] = useState<FailureRange>(initialFilters.range);
  const [selectedFailureId, setSelectedFailureId] = useState<string | null>(null);
  const [drawerTab, setDrawerTab] = useState<"attributes" | "raw">("attributes");
  const [rawCopied, setRawCopied] = useState(false);
  const limit = 100;

  const loadFailures = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const params: AdminPostFailureListParams = {
        search: search || undefined,
        user_id: userIdFilter || undefined,
        platform: platform !== "all" ? platform : undefined,
        source: source !== "all" ? source : undefined,
        period: range === "this_month" ? "this_month" : undefined,
        days: range !== "this_month" ? Number(range) : undefined,
        limit,
      };
      const res = await listAdminPostFailures(token, params);
      setFailures(res.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, [getToken, platform, range, search, source, userIdFilter]);

  useEffect(() => {
    loadFailures();
  }, [loadFailures]);

  useEffect(() => {
    const nextFilters = filtersFromURLParams(searchParams);
    setSearch(nextFilters.search);
    setSearchInput(nextFilters.search);
    setUserIdFilter(nextFilters.userId);
    setPlatform(nextFilters.platform);
    setSource(nextFilters.source);
    setRange(nextFilters.range);
    setSelectedFailureId(null);
  }, [searchParams]);

  useEffect(() => {
    const timer = setTimeout(() => setSearch(searchInput), 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  const uniqueUsers = useMemo(() => new Set(failures.map((item) => item.user_id)).size, [failures]);
  const uniqueWorkspaces = useMemo(() => new Set(failures.map((item) => item.workspace_id)).size, [failures]);
  const byPlatform = useMemo(() => {
    const counts = new Map<string, number>();
    failures.forEach((item) => {
      const key = item.platform || "parent";
      counts.set(key, (counts.get(key) || 0) + 1);
    });
    return [...counts.entries()].sort((a, b) => b[1] - a[1])[0] ?? null;
  }, [failures]);
  const bySource = useMemo(() => {
    const counts = new Map<string, number>();
    failures.forEach((item) => counts.set(item.source, (counts.get(item.source) || 0) + 1));
    return [...counts.entries()].sort((a, b) => b[1] - a[1])[0] ?? null;
  }, [failures]);
  const selectedFailure = useMemo(() => {
    if (!selectedFailureId) return null;
    return failures.find((failure, idx) => failureKey(failure, idx) === selectedFailureId) ?? null;
  }, [failures, selectedFailureId]);

  useEffect(() => {
    if (selectedFailureId && !selectedFailure) {
      setSelectedFailureId(null);
    }
  }, [selectedFailure, selectedFailureId]);

  const openFailureDetail = useCallback((failure: AdminUserPostFailure, idx: number) => {
    setSelectedFailureId(failureKey(failure, idx));
    setDrawerTab("attributes");
    setRawCopied(false);
  }, []);

  const closeDetail = useCallback(() => {
    setSelectedFailureId(null);
    setRawCopied(false);
  }, []);

  const copyRawFailure = useCallback(async () => {
    if (!selectedFailure) return;
    await navigator.clipboard.writeText(JSON.stringify(selectedFailure, null, 2));
    setRawCopied(true);
    window.setTimeout(() => setRawCopied(false), 1200);
  }, [selectedFailure]);

  const stopLinkClick = useCallback((event: MouseEvent<HTMLAnchorElement>) => {
    event.stopPropagation();
  }, []);
  const rangeLabel = range === "this_month" ? "this month" : `the last ${range} days`;

  return (
    <AdminShell title="Errors" loading={loading} onRefresh={loadFailures}>
      {error && (
        <div style={{ background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 22%, transparent)", borderRadius: 8, padding: 12, marginBottom: 16, color: "var(--danger)", fontSize: 13 }}>
          {error}
        </div>
      )}

      <div className="ad-section-header">
        <div className="ad-section-title">Publishing failures</div>
        <div className="ad-section-meta">
          Cross-tenant errors from {rangeLabel}{userIdFilter ? ` for ${userIdFilter.slice(0, 16)}` : ""}
        </div>
      </div>

      <div className="ad-stat-grid">
        <StatCard label="Failures" value={fmtNumber(failures.length)} sub="current filtered set" />
        <StatCard label="Affected Users" value={fmtNumber(uniqueUsers)} sub="distinct customers" />
        <StatCard label="Affected Workspaces" value={fmtNumber(uniqueWorkspaces)} sub="distinct workspaces" />
        <StatCard
          label="Top Bucket"
          value={byPlatform ? byPlatform[0] : "—"}
          sub={byPlatform ? `${fmtNumber(byPlatform[1])} failures` : bySource ? `${bySource[0]} source` : "—"}
          valueColor="accent"
        />
      </div>

      <div className="ad-filter-bar">
        <SearchHistoryInput
          fieldKey="admin.errors.search"
          className="ad-search"
          placeholder="Search by user, workspace, ID, caption, or error..."
          value={searchInput}
          onChange={setSearchInput}
          style={{ width: 320 }}
        />
        <select value={platform} onChange={(e) => setPlatform(e.target.value as typeof platform)}>
          {PLATFORM_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All Platforms" : `Platform: ${value}`}
            </option>
          ))}
        </select>
        <select value={source} onChange={(e) => setSource(e.target.value as typeof source)}>
          {SOURCE_OPTIONS.map((value) => (
            <option key={value} value={value}>
              {value === "all" ? "All Sources" : `Source: ${value}`}
            </option>
          ))}
        </select>
        <select value={range} onChange={(e) => setRange(e.target.value as FailureRange)}>
          <option value="this_month">This Month</option>
          {RANGE_OPTIONS.filter((value) => value !== "this_month").map((value) => (
            <option key={value} value={value}>
              Last {value} days
            </option>
          ))}
        </select>
        {userIdFilter ? (
          <span className="ad-badge ad-b-gray" title={userIdFilter}>
            User: {userIdFilter.slice(0, 16)}
          </span>
        ) : null}
      </div>

      <div style={errorsConsoleStyle}>
        <div className="ad-stack">
          {loading && failures.length === 0 ? (
            <div className="ad-failure-card" style={{ color: "var(--dmuted)", textAlign: "center" }}>Loading...</div>
          ) : failures.length === 0 ? (
            <div className="ad-failure-card" style={{ color: "var(--dmuted)", textAlign: "center" }}>
              No failures matched the current filters.
            </div>
          ) : (
            failures.map((failure, idx) => {
              const id = failureKey(failure, idx);
              const selected = id === selectedFailureId;
              const message = failure.error_message || failure.error_summary || "No error message recorded.";
              return (
                <article
                  key={id}
                  className="ad-failure-card"
                  role="button"
                  tabIndex={0}
                  aria-label={`Open error details for ${failure.user_email}`}
                  aria-pressed={selected}
                  onClick={() => openFailureDetail(failure, idx)}
                  onKeyDown={(event) => handleFailureKeyDown(event, () => openFailureDetail(failure, idx))}
                  style={{
                    cursor: "pointer",
                    outline: "none",
                    borderColor: selected ? "color-mix(in srgb, var(--danger) 38%, var(--dborder))" : undefined,
                    background: selected ? "color-mix(in srgb, var(--danger-soft) 42%, var(--surface))" : undefined,
                  }}
                >
                  <div className="ad-failure-head">
                    <div>
                      <div className="ad-failure-meta">
                        <span className="ad-badge ad-b-gray">{failure.platform || failure.post_status}</span>
                        <span className="ad-badge ad-b-blue">{failure.source}</span>
                        {failure.error_code ? <span className="ad-badge ad-b-red">{failure.error_code}</span> : null}
                        {failure.account_name ? <span style={{ fontSize: 11, color: "var(--dmuted)" }}>@{failure.account_name}</span> : null}
                      </div>
                      <div className="ad-failure-title" style={{ marginTop: 6 }}>
                        <Link href={`/admin/users?user=${failure.user_id}`} className="ad-link" onClick={stopLinkClick}>
                          {failure.user_email}
                        </Link>
                        <span style={{ color: "var(--dmuted2)" }}> · </span>
                        <span>{failure.workspace_name}</span>
                      </div>
                    </div>
                    <div style={{ textAlign: "right" }}>
                      <div style={{ fontSize: 11.5, color: "var(--dmuted)" }}>{fmtRelative(failure.created_at)}</div>
                      <div className="ad-mono" style={{ marginTop: 4 }}>{failure.post_id.slice(0, 16)}</div>
                    </div>
                  </div>

                  {failure.caption ? <div className="ad-failure-caption">{failure.caption}</div> : null}
                  <div className="ad-failure-message">{message}</div>

                  <div style={{ display: "flex", justifyContent: "space-between", gap: 12, marginTop: 8, alignItems: "center", flexWrap: "wrap" }}>
                    <div className="ad-mono">
                      workspace {failure.workspace_id.slice(0, 12)} · user {failure.user_id.slice(0, 12)}
                    </div>
                    <Link href={`/admin/users?user=${failure.user_id}`} className="ad-link" style={{ fontSize: 12 }} onClick={stopLinkClick}>
                      Inspect user →
                    </Link>
                  </div>
                </article>
              );
            })
          )}
        </div>

        {selectedFailure ? (
          <aside
            className="errors-detail-drawer"
            role="dialog"
            aria-label="Error detail"
            style={drawerStyle}
          >
            <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12 }}>
              <div>
                <div style={{ fontSize: 18, fontWeight: 700 }}>Error details</div>
                <div style={{ color: "var(--dmuted)", marginTop: 4 }}>
                  {selectedFailure.platform || selectedFailure.post_status} · {selectedFailure.source}
                </div>
              </div>
              <button type="button" onClick={closeDetail} style={iconButtonStyle} aria-label="Close error details">
                <X size={16} />
              </button>
            </div>

            <DrawerTabs
              active={drawerTab}
              onChange={setDrawerTab}
              rightSlot={
                drawerTab === "raw" ? (
                  <button
                    type="button"
                    onClick={copyRawFailure}
                    style={drawerCopyButtonStyle}
                    aria-label="Copy raw error JSON"
                  >
                    {rawCopied ? (
                      <>
                        <Check size={12} />
                        Copied
                      </>
                    ) : (
                      <>
                        <Copy size={12} />
                        Copy
                      </>
                    )}
                  </button>
                ) : null
              }
            />

            {drawerTab === "raw" ? (
              <pre style={drawerRawJsonStyle}>{JSON.stringify(selectedFailure, null, 2)}</pre>
            ) : (
              <div style={{ display: "grid", gap: 14 }}>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                  <FieldChip label="platform" value={selectedFailure.platform || "parent"} />
                  <FieldChip label="source" value={selectedFailure.source} />
                  <FieldChip label="status" value={selectedFailure.post_status} />
                  <FieldChip label="time" value={new Date(selectedFailure.created_at).toLocaleString()} />
                </div>

                <div style={sectionStyle}>
                  <div style={sectionTitleStyle}>Context</div>
                  <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                    <FieldChip label="workspace" value={selectedFailure.workspace_id} />
                    <FieldChip label="user" value={selectedFailure.user_id} />
                    <FieldChip label="owner" value={selectedFailure.user_email} />
                    {selectedFailure.account_name ? <FieldChip label="account" value={selectedFailure.account_name} /> : null}
                    <FieldChip label="post_id" value={selectedFailure.post_id} />
                    {selectedFailure.post_failure_id ? <FieldChip label="post_failure_id" value={selectedFailure.post_failure_id} /> : null}
                    {selectedFailure.social_post_result_id ? <FieldChip label="social_post_result_id" value={selectedFailure.social_post_result_id} /> : null}
                  </div>
                </div>

                {(selectedFailure.error_code || selectedFailure.failure_stage || selectedFailure.platform_error_code || selectedFailure.next_action || selectedFailure.is_retriable != null) ? (
                  <div style={sectionStyle}>
                    <div style={sectionTitleStyle}>Failure classification</div>
                    <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                      {selectedFailure.error_code ? <FieldChip label="error_code" value={selectedFailure.error_code} /> : null}
                      {selectedFailure.failure_stage ? <FieldChip label="failure_stage" value={selectedFailure.failure_stage} /> : null}
                      {selectedFailure.platform_error_code ? <FieldChip label="platform_error_code" value={selectedFailure.platform_error_code} /> : null}
                      {selectedFailure.next_action ? <FieldChip label="next_action" value={selectedFailure.next_action} /> : null}
                      {selectedFailure.is_retriable != null ? <FieldChip label="retriable" value={String(selectedFailure.is_retriable)} /> : null}
                    </div>
                  </div>
                ) : null}

                <div style={sectionStyle}>
                  <div style={sectionTitleStyle}>Summary</div>
                  {selectedFailure.caption ? (
                    <div style={{ fontSize: 14, lineHeight: 1.55, marginBottom: 12 }}>{selectedFailure.caption}</div>
                  ) : null}
                  <div style={{ fontSize: 14, lineHeight: 1.6, color: "var(--danger)" }}>
                    {selectedFailure.error_message || selectedFailure.error_summary || "No error message recorded."}
                  </div>
                </div>

                <div style={sectionStyle}>
                  <div style={sectionTitleStyle}>Debug curl</div>
                  {selectedFailure.debug_curl ? (
                    <pre style={drawerCodeBlockStyle}>{selectedFailure.debug_curl}</pre>
                  ) : (
                    <div style={{ color: "var(--dmuted2)", fontSize: 13 }}>No debug curl captured for this failure.</div>
                  )}
                </div>

                <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
                  <Link href={`/admin/users?user=${selectedFailure.user_id}`} className="ad-link" style={drawerLinkButtonStyle}>
                    Inspect user
                    <ExternalLink size={14} />
                  </Link>
                </div>
              </div>
            )}
          </aside>
        ) : null}
      </div>
    </AdminShell>
  );
}

export default function AdminErrorsPage() {
  return (
    <Suspense fallback={<AdminShell title="Errors" loading><div /></AdminShell>}>
      <AdminErrorsContent />
    </Suspense>
  );
}

function failureKey(failure: AdminUserPostFailure, idx: number) {
  if (failure.post_failure_id) return failure.post_failure_id;
  if (failure.social_post_result_id) return failure.social_post_result_id;
  return `${failure.post_id}-${failure.platform || "parent"}-${failure.created_at}-${idx}`;
}

function handleFailureKeyDown(event: KeyboardEvent<HTMLElement>, open: () => void) {
  if (event.key !== "Enter" && event.key !== " ") return;
  event.preventDefault();
  open();
}

function FieldChip({ label, value }: { label: string; value: string }) {
  return (
    <span style={fieldChipStyle}>
      <span style={{ color: "var(--dmuted2)" }}>{label}</span>
      <span style={{ fontFamily: "var(--font-geist-mono), monospace" }}>{value}</span>
    </span>
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

const errorsConsoleStyle: CSSProperties = {
  position: "relative",
  minHeight: 420,
};

const drawerStyle: CSSProperties = {
  position: "fixed",
  top: 0,
  right: 0,
  bottom: 0,
  width: "45%",
  minWidth: 380,
  maxWidth: 760,
  background: "var(--surface-raised, var(--surface))",
  borderLeft: "1px solid var(--dborder)",
  zIndex: 30,
  overflowY: "auto",
  padding: 18,
  display: "flex",
  flexDirection: "column",
  gap: 14,
  boxShadow: "-18px 0 44px color-mix(in srgb, var(--sidebar) 28%, transparent)",
};

const iconButtonStyle: CSSProperties = {
  height: 32,
  width: 32,
  borderRadius: 10,
  border: "1px solid var(--dborder)",
  background: "var(--surface2)",
  color: "var(--dtext)",
  display: "inline-flex",
  alignItems: "center",
  justifyContent: "center",
  cursor: "pointer",
};

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
    borderBottom: active ? "2px solid var(--danger)" : "2px solid transparent",
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

const sectionStyle: CSSProperties = {
  borderRadius: 14,
  border: "1px solid color-mix(in srgb, var(--dborder) 74%, var(--sidebar) 26%)",
  background: "color-mix(in srgb, var(--surface2) 82%, var(--sidebar) 18%)",
  padding: 14,
};

const sectionTitleStyle: CSSProperties = {
  fontSize: 12,
  fontWeight: 700,
  textTransform: "uppercase",
  letterSpacing: "0.12em",
  color: "var(--dmuted)",
  marginBottom: 10,
};

const fieldChipStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 6,
  padding: "6px 10px",
  borderRadius: 999,
  border: "1px solid var(--dborder)",
  background: "var(--surface2)",
  color: "var(--dtext)",
  fontSize: 12,
};

const drawerRawJsonStyle: CSSProperties = {
  margin: 0,
  padding: 14,
  borderRadius: 12,
  border: "1px solid color-mix(in srgb, var(--dborder) 74%, var(--sidebar) 26%)",
  background: "color-mix(in srgb, var(--surface) 66%, var(--sidebar) 34%)",
  color: "var(--dtext)",
  fontSize: 12,
  lineHeight: 1.6,
  overflow: "auto",
  whiteSpace: "pre",
  fontFamily: "var(--font-geist-mono), monospace",
};

const drawerCodeBlockStyle: CSSProperties = {
  ...drawerRawJsonStyle,
  whiteSpace: "pre-wrap",
  wordBreak: "break-all",
};

const drawerLinkButtonStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 6,
  fontSize: 13,
};
