"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { CSSProperties, ReactNode } from "react";
import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import {
  Bookmark,
  ExternalLink,
  FileText,
  ImageIcon,
  MousePointerClick,
  RefreshCw,
  ShieldCheck,
  Users,
  type LucideIcon,
} from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";
import {
  getPostAnalytics,
  listPinterestBoards,
  listSocialAccounts,
  listSocialPosts,
  type ApiResponse,
  type PinterestBoard,
  type PostAnalytics,
  type SocialAccount,
  type SocialPost,
  type SocialPostResult,
} from "@/lib/api";

type PinterestPinRow = {
  title: string;
  status: string;
  externalId: string;
  url: string;
  boardId: string;
  impressions: number;
  saves: number;
  clicks: number;
  likes: number;
  comments: number;
  fetchedAt: string;
  publishedAt: string;
  failure: string;
};

const REQUIRED_SCOPES = ["pins:read", "boards:read", "user_accounts:read"] as const;

const tableStyle: CSSProperties = {
  width: "100%",
  borderCollapse: "collapse",
  fontSize: 13,
};

const thStyle: CSSProperties = {
  padding: "10px 12px",
  textAlign: "left",
  fontSize: 11,
  fontWeight: 600,
  letterSpacing: "0.06em",
  textTransform: "uppercase",
  color: "var(--dmuted2)",
  borderBottom: "1px solid var(--dborder)",
};

const thRightStyle: CSSProperties = {
  ...thStyle,
  textAlign: "right",
};

const tdStyle: CSSProperties = {
  padding: "12px",
  borderBottom: "1px solid var(--dborder)",
  color: "var(--dtext)",
  verticalAlign: "middle",
};

const tdRightStyle: CSSProperties = {
  ...tdStyle,
  textAlign: "right",
  fontFamily: "var(--font-geist-mono), monospace",
  fontSize: 12.5,
};

function formatNumber(n: number | undefined): string {
  if (!Number.isFinite(n || 0) || !n) return "0";
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1).replace(/\.0$/, "") + "M";
  if (n >= 1_000) return (n / 1_000).toFixed(1).replace(/\.0$/, "") + "k";
  return String(n);
}

function formatDate(value?: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return date.toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
}

function readString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function pinterestOptions(result: SocialPostResult): Record<string, unknown> {
  const submittedOptions = result.submitted?.platform_options;
  const nested = submittedOptions?.pinterest;
  if (nested && typeof nested === "object" && !Array.isArray(nested)) return nested as Record<string, unknown>;
  return submittedOptions || {};
}

export function PinterestAnalyticsView({ profileId }: { profileId: string }) {
  const { getToken } = useAuth();
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [selectedAccountId, setSelectedAccountId] = useState("");
  const [boards, setBoards] = useState<PinterestBoard[]>([]);
  const [pinRows, setPinRows] = useState<PinterestPinRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");
  const [notices, setNotices] = useState<string[]>([]);

  const selectedAccount = useMemo(
    () => accounts.find((account) => account.id === selectedAccountId) || accounts[0],
    [accounts, selectedAccountId]
  );

  const scopeState = useMemo(() => {
    const storedScopes = selectedAccount?.scope || [];
    if (storedScopes.length === 0) return { unknown: true, missing: [] as string[] };
    const granted = new Set(storedScopes);
    return {
      unknown: false,
      missing: REQUIRED_SCOPES.filter((scope) => !granted.has(scope)),
    };
  }, [selectedAccount?.scope]);

  const totals = useMemo(() => ({
    pins: pinRows.length,
    boards: boards.length,
    impressions: pinRows.reduce((sum, row) => sum + row.impressions, 0),
    saves: pinRows.reduce((sum, row) => sum + row.saves, 0),
    clicks: pinRows.reduce((sum, row) => sum + row.clicks, 0),
    comments: pinRows.reduce((sum, row) => sum + row.comments, 0),
  }), [boards.length, pinRows]);

  const loadData = useCallback(async (opts?: { refreshAnalytics?: boolean }) => {
    try {
      setError("");
      setNotices([]);
      setRefreshing(true);
      const token = await getToken();
      if (!token) {
        setError("Session expired. Please sign in again.");
        return;
      }

      const accountRes = await listSocialAccounts(token, profileId, { platform: "pinterest" });
      const platformAccounts = accountRes.data || [];
      setAccounts(platformAccounts);
      const account = platformAccounts.find((a) => a.id === selectedAccountId) || platformAccounts[0];
      if (!account) {
        setBoards([]);
        setPinRows([]);
        return;
      }
      if (account.id !== selectedAccountId) setSelectedAccountId(account.id);

      const [boardsRes, postsRes] = await Promise.allSettled([
        listPinterestBoards(token, profileId, account.id),
        listSocialPosts(token),
      ]);

      const nextNotices: string[] = [];
      if (boardsRes.status === "fulfilled") {
        setBoards(boardsRes.value.data.boards || []);
        if (boardsRes.value.data.sandbox_mode) {
          nextNotices.push("This connection is still using Pinterest sandbox mode; production Pin analytics require the live Pinterest API.");
        }
      } else {
        setBoards([]);
        nextNotices.push(`Pinterest boards unavailable: ${boardsRes.reason instanceof Error ? boardsRes.reason.message : "upstream error"}`);
      }

      if (postsRes.status === "fulfilled") {
        const publishedPins = (postsRes.value.data || []).filter((post) => {
          if (!post.results?.some((result) => result.social_account_id === account.id && result.external_id)) return false;
          if (post.profile_ids?.length && !post.profile_ids.includes(profileId)) return false;
          return post.status === "published" || post.status === "partial";
        });
        const analyticsSettled = await Promise.allSettled(
          publishedPins.map((post) => getPostAnalytics(token, post.id, { refresh: !!opts?.refreshAnalytics }))
        );
        setPinRows(buildPinRows(publishedPins, analyticsSettled, account.id));
      } else {
        setPinRows([]);
        nextNotices.push(`UniPost Pin analytics unavailable: ${postsRes.reason instanceof Error ? postsRes.reason.message : "upstream error"}`);
      }
      setNotices(nextNotices);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load Pinterest analytics");
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [getToken, profileId, selectedAccountId]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const stats = [
    { label: "Published Pins", value: totals.pins, icon: ImageIcon },
    { label: "Boards", value: totals.boards, icon: Bookmark },
    { label: "Impressions", value: totals.impressions, icon: Users },
    { label: "Saves", value: totals.saves, icon: Bookmark },
    { label: "Outbound Clicks", value: totals.clicks, icon: MousePointerClick },
    { label: "Comments", value: totals.comments, icon: FileText },
  ];

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 20, marginBottom: 24 }}>
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 8 }}>
            <div className="platform-icon-wrap"><PlatformIcon platform="pinterest" /></div>
            <div className="dt-page-title">Pinterest Analytics</div>
          </div>
          <div className="dt-subtitle" style={{ maxWidth: 760 }}>
            Connected boards plus UniPost-published Pin performance from Pinterest's production analytics API.
          </div>
        </div>
        <button className="dbtn dbtn-ghost" type="button" onClick={() => loadData({ refreshAnalytics: true })} disabled={refreshing}>
          <RefreshCw style={{ width: 14, height: 14 }} />
          {refreshing ? "Refreshing..." : "Refresh"}
        </button>
      </div>

      {error && <Notice tone="danger" message={error} />}
      {notices.map((message) => <Notice key={message} tone="muted" message={message} />)}

      {accounts.length > 1 && (
        <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 16 }}>
          <span className="dt-label" style={{ color: "var(--dmuted2)" }}>Account</span>
          <select
            value={selectedAccount?.id || ""}
            onChange={(event) => setSelectedAccountId(event.target.value)}
            style={{ padding: "7px 10px", border: "1px solid var(--dborder)", borderRadius: 6, background: "var(--surface1)", color: "var(--dtext)" }}
          >
            {accounts.map((account) => (
              <option key={account.id} value={account.id}>{account.account_name || account.id}</option>
            ))}
          </select>
        </div>
      )}

      {loading ? (
        <div style={{ color: "var(--dmuted)", padding: 40, textAlign: "center" }}>Loading Pinterest analytics...</div>
      ) : !selectedAccount ? (
        <EmptyState />
      ) : (
        <>
          <ScopeReadiness unknown={scopeState.unknown} missingScopes={scopeState.missing} />
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(min(100%, 360px), 1fr))", gap: 16, marginBottom: 24 }}>
            <ProfilePanel account={selectedAccount} />
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(130px, 1fr))", gap: 12 }}>
              {stats.map((item) => <MetricCard key={item.label} {...item} />)}
            </div>
          </div>
          <BoardsTable boards={boards} />
          <PinsTable rows={pinRows} />
        </>
      )}
    </>
  );
}

function buildPinRows(
  posts: SocialPost[],
  analyticsSettled: PromiseSettledResult<ApiResponse<PostAnalytics[]>>[],
  accountId: string
): PinterestPinRow[] {
  return posts.map((post, index) => {
    const result = post.results?.find((item) => item.social_account_id === accountId);
    const analytics = analyticsSettled[index];
    const row = analytics.status === "fulfilled"
      ? analytics.value.data.find((item) => item.social_account_id === accountId)
      : undefined;
    const options = result ? pinterestOptions(result) : {};
    const externalId = row?.external_id || result?.external_id || "";
    const title = readString(options.title) || post.caption || "Pinterest Pin";
    return {
      title,
      status: result?.status || post.status,
      externalId: externalId || "-",
      url: result?.url || (externalId ? `https://www.pinterest.com/pin/${externalId}/` : ""),
      boardId: readString(options.board_id) || "-",
      impressions: row?.impressions || 0,
      saves: row?.saves || 0,
      clicks: row?.clicks || 0,
      likes: row?.likes || 0,
      comments: row?.comments || 0,
      fetchedAt: row?.fetched_at || "",
      publishedAt: result?.published_at || post.published_at || post.created_at,
      failure: analytics.status === "rejected"
        ? analytics.reason instanceof Error ? analytics.reason.message : "analytics unavailable"
        : row?.last_failure_reason || "",
    };
  });
}

function Notice({ tone, message }: { tone: "danger" | "muted"; message: string }) {
  const danger = tone === "danger";
  return (
    <div style={{
      marginBottom: 16,
      padding: "10px 12px",
      border: danger ? "1px solid color-mix(in srgb, var(--danger) 24%, transparent)" : "1px solid var(--dborder)",
      borderRadius: 8,
      color: danger ? "var(--danger)" : "var(--dmuted)",
      background: danger ? "var(--danger-soft)" : "var(--surface2)",
      fontSize: 13,
    }}>
      {message}
    </div>
  );
}

function ScopeReadiness({
  unknown,
  missingScopes,
}: {
  unknown: boolean;
  missingScopes: readonly string[];
}) {
  const ready = missingScopes.length === 0;
  const body = unknown
    ? "Stored scope data is unavailable for this connection; live Pinterest API calls verify access."
    : ready ? REQUIRED_SCOPES.join(", ") : `Missing: ${missingScopes.join(", ")}`;
  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 16, padding: "12px 14px", borderRadius: 8, border: "1px solid var(--dborder)", background: "var(--surface1)" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 10, minWidth: 0 }}>
        <ShieldCheck style={{ width: 18, height: 18, color: ready ? "var(--success)" : "var(--warning)" }} />
        <div>
          <div style={{ color: "var(--dtext)", fontSize: 13, fontWeight: 650 }}>{ready ? "Pinterest permissions ready" : "Reconnect required for analytics"}</div>
          <div style={{ color: "var(--dmuted)", fontSize: 12, marginTop: 3 }}>{body}</div>
        </div>
      </div>
      <span className={`dbadge ${ready ? "dbadge-green" : "dbadge-amber"}`}><span className="dbadge-dot" />{ready ? "Ready" : "Reconnect"}</span>
    </div>
  );
}

function ProfilePanel({ account }: { account: SocialAccount }) {
  const username = (account.account_name || "").replace(/^@/, "").trim();
  const displayName = username ? `@${username}` : account.account_name || "Pinterest account";
  const profileUrl = username ? `https://www.pinterest.com/${username}/` : "";

  return (
    <div className="settings-section" style={{ marginBottom: 0 }}>
      <div className="settings-section-header">
        <div>
          <div className="settings-section-title">Profile</div>
          <div className="settings-section-desc">Pinterest user account connected through OAuth</div>
        </div>
      </div>
      <div className="settings-section-body">
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 16 }}>
          <ProfileAvatar src={account.account_avatar_url || ""} label={displayName} />
          <div style={{ minWidth: 0 }}>
            <div style={{ color: "var(--dtext)", fontWeight: 700 }}>{displayName}</div>
            <div style={{ color: "var(--dmuted)", fontSize: 13 }}>{account.external_account_id || account.id}</div>
          </div>
        </div>
        {profileUrl && (
          <Link href={profileUrl} target="_blank" rel="noreferrer" style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--daccent)", fontSize: 13, textDecoration: "none", minWidth: 0 }}>
            <ExternalLink style={{ width: 14, height: 14, flexShrink: 0 }} />
            <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{profileUrl}</span>
          </Link>
        )}
      </div>
    </div>
  );
}

function ProfileAvatar({ src, label }: { src: string; label: string }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [src]);
  const showImage = src && !failed;
  return (
    <div style={{ width: 48, height: 48, borderRadius: "50%", background: "linear-gradient(135deg, #7f1d1d, #be123c)", display: "grid", placeItems: "center", color: "white", fontWeight: 700, overflow: "hidden", flexShrink: 0 }}>
      {showImage ? (
        <img src={src} alt="" onError={() => setFailed(true)} style={{ width: "100%", height: "100%", objectFit: "cover", display: "block" }} />
      ) : (
        <span>{label.replace(/^@/, "").slice(0, 1).toUpperCase() || "P"}</span>
      )}
    </div>
  );
}

function MetricCard({ label, value, icon: Icon }: { label: string; value: number; icon: LucideIcon }) {
  return (
    <div style={{ border: "1px solid var(--dborder)", borderRadius: 8, background: "var(--surface1)", padding: 14, minHeight: 92 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8, marginBottom: 12 }}>
        <div style={{ color: "var(--dmuted)", fontSize: 12 }}>{label}</div>
        <Icon style={{ width: 15, height: 15, color: "var(--dmuted2)" }} />
      </div>
      <div style={{ color: "var(--dtext)", fontSize: 24, fontWeight: 750, fontFamily: "var(--font-geist-mono), monospace" }}>{formatNumber(value)}</div>
    </div>
  );
}

function BoardsTable({ boards }: { boards: PinterestBoard[] }) {
  return (
    <ContentSection title="Pinterest boards" desc="Boards visible to the connected account. Pins published through UniPost must target one board.">
      <table style={tableStyle}>
        <thead>
          <tr>
            <th style={thStyle}>Board</th>
            <th style={thStyle}>Board ID</th>
          </tr>
        </thead>
        <tbody>
          {boards.length === 0 ? (
            <tr><td colSpan={2} style={{ ...tdStyle, textAlign: "center", color: "var(--dmuted2)", padding: 24 }}>No Pinterest boards returned.</td></tr>
          ) : boards.map((board) => (
            <tr key={board.id}>
              <td style={tdStyle}>{board.name || "Untitled board"}</td>
              <td style={tdStyle}><span style={{ color: "var(--dmuted)", fontFamily: "var(--font-geist-mono), monospace", fontSize: 12 }}>{board.id}</span></td>
            </tr>
          ))}
        </tbody>
      </table>
    </ContentSection>
  );
}

function PinsTable({ rows }: { rows: PinterestPinRow[] }) {
  return (
    <ContentSection title="UniPost-published Pins" desc="Pin-level analytics fetched from Pinterest for content published through UniPost. Use Refresh to request fresh platform metrics.">
      <table style={tableStyle}>
        <thead>
          <tr>
            <th style={thStyle}>Pin</th>
            <th style={thStyle}>Published</th>
            <th style={thStyle}>Board ID</th>
            <th style={thRightStyle}>Impressions</th>
            <th style={thRightStyle}>Saves</th>
            <th style={thRightStyle}>Clicks</th>
            <th style={thRightStyle}>Likes</th>
            <th style={thRightStyle}>Comments</th>
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr><td colSpan={8} style={{ ...tdStyle, textAlign: "center", color: "var(--dmuted2)", padding: 24 }}>No UniPost-published Pinterest Pins returned.</td></tr>
          ) : rows.map((row, index) => (
            <tr key={`${row.externalId}-${index}`}>
              <td style={tdStyle}>
                <div style={{ display: "grid", gap: 4, minWidth: 220 }}>
                  <ContentLink href={row.url} text={row.title} />
                  <span style={{ color: "var(--dmuted2)", fontFamily: "var(--font-geist-mono), monospace", fontSize: 11 }}>{row.externalId}</span>
                  {row.failure && <span style={{ color: "var(--warning)", fontSize: 12 }}>{row.failure}</span>}
                </div>
              </td>
              <td style={tdStyle}>{formatDate(row.publishedAt)}</td>
              <td style={tdStyle}><span style={{ color: "var(--dmuted)", fontFamily: "var(--font-geist-mono), monospace", fontSize: 12 }}>{row.boardId}</span></td>
              <td style={tdRightStyle}>{formatNumber(row.impressions)}</td>
              <td style={tdRightStyle}>{formatNumber(row.saves)}</td>
              <td style={tdRightStyle}>{formatNumber(row.clicks)}</td>
              <td style={tdRightStyle}>{formatNumber(row.likes)}</td>
              <td style={tdRightStyle}>{formatNumber(row.comments)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </ContentSection>
  );
}

function ContentSection({ title, desc, children }: { title: string; desc: string; children: ReactNode }) {
  return (
    <div className="settings-section" style={{ marginBottom: 24 }}>
      <div className="settings-section-header">
        <div>
          <div className="settings-section-title">{title}</div>
          <div className="settings-section-desc">{desc}</div>
        </div>
      </div>
      <div className="settings-section-body" style={{ overflowX: "auto" }}>
        {children}
      </div>
    </div>
  );
}

function ContentLink({ href, text }: { href?: string; text: string }) {
  const label = text.length > 80 ? `${text.slice(0, 77)}...` : text;
  if (!href) return <span>{label}</span>;
  return (
    <Link href={href} target="_blank" rel="noreferrer" style={{ color: "var(--dtext)", textDecoration: "none", display: "inline-flex", alignItems: "center", gap: 6, maxWidth: 360 }}>
      <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{label}</span>
      <ExternalLink style={{ width: 13, height: 13, color: "var(--dmuted2)", flexShrink: 0 }} />
    </Link>
  );
}

function EmptyState() {
  return (
    <div style={{ textAlign: "center", padding: 60, color: "var(--dmuted)" }}>
      <ImageIcon style={{ width: 40, height: 40, margin: "0 auto 12px", opacity: 0.3 }} />
      <div style={{ fontSize: 15, fontWeight: 600, color: "var(--dtext)", marginBottom: 6 }}>No Pinterest account connected</div>
      <div style={{ fontSize: 13 }}>Connect a Pinterest account before using platform analytics.</div>
    </div>
  );
}
