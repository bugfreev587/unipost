"use client";

import { Fragment, useCallback, useEffect, useState, useRef } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { useWorkspaceId } from "@/lib/use-workspace-id";
import {
  listSocialAccounts, listSocialPosts, cancelSocialPost, listProfiles,
  type SocialAccount, type SocialPost, type Profile,
} from "@/lib/api";
import { Plus, Search, MoreHorizontal, Eye, Copy, Pencil, Send, XCircle, Calendar, ChevronDown, ChevronRight, ExternalLink } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";
import { CreatePostDrawer } from "@/components/posts/create-post/create-post-drawer";

type FilterTab = "all" | "published" | "scheduled" | "failed" | "draft";

const STATUS_BADGE: Record<string, { cls: string; label: string }> = {
  published: { cls: "dbadge-green", label: "published" },
  scheduled: { cls: "dbadge-blue", label: "scheduled" },
  processing: { cls: "dbadge-blue", label: "processing" },
  partial: { cls: "dbadge-amber", label: "partial" },
  failed: { cls: "dbadge-red", label: "failed" },
  draft: { cls: "dbadge-gray", label: "draft" },
  cancelled: { cls: "dbadge-gray", label: "cancelled" },
};

function statusBadge(status: string) {
  const b = STATUS_BADGE[status] || { cls: "dbadge-gray", label: status };
  return <span className={`dbadge ${b.cls}`}><span className="dbadge-dot" />{b.label}</span>;
}

// Extra CSS for this page
const CSS = `.dbadge-gray{background:color-mix(in srgb,var(--surface2) 82%,white);color:var(--dmuted);border:1px solid var(--dborder)}
.posts-header{display:flex;align-items:flex-start;justify-content:space-between;margin-bottom:24px}
.posts-filters{display:flex;align-items:center;gap:8px;margin-bottom:16px}
.posts-search{display:flex;align-items:center;gap:6px;background:var(--surface2);border:1px solid var(--dborder);border-radius:6px;padding:0 10px;height:32px;flex:0 1 240px}
.posts-search input{background:none;border:none;outline:none;color:var(--dtext);font-size:13px;font-family:inherit;width:100%}
.posts-search input::placeholder{color:var(--dmuted2)}
.posts-search svg{color:var(--dmuted2);flex-shrink:0}
.posts-select{background:var(--surface2);border:1px solid var(--dborder);border-radius:6px;padding:0 10px;height:32px;color:var(--dtext);font-size:13px;font-family:inherit;cursor:pointer;outline:none}
.posts-select:focus{border-color:var(--daccent)}
.posts-row{cursor:pointer;transition:background .1s}
.posts-row:hover{background:var(--surface2)}
.posts-caption{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:360px;font-size:14px;color:var(--dtext)}
.posts-plats{display:flex;gap:3px;align-items:center}
.posts-plats-more{font-size:10px;color:var(--dmuted2);font-weight:600}
.posts-time{font-size:13px;color:var(--dmuted)}
.posts-actions{position:relative}
.posts-actions-btn{background:none;border:1px solid transparent;border-radius:4px;padding:4px;cursor:pointer;color:var(--dmuted2);transition:all .1s;display:flex;align-items:center}
.posts-actions-btn:hover{background:var(--surface2);border-color:var(--dborder);color:var(--dtext)}
.posts-menu{position:absolute;right:0;top:100%;margin-top:4px;background:var(--surface-raised);border:1px solid var(--dborder);border-radius:8px;padding:4px;min-width:180px;z-index:20;box-shadow:0 12px 28px color-mix(in srgb,var(--shadow-color) 120%,transparent)}
.posts-menu-item{display:flex;align-items:center;gap:8px;padding:7px 10px;border-radius:5px;font-size:13px;color:var(--dmuted);cursor:pointer;transition:all .1s;border:none;background:none;width:100%;text-align:left;font-family:inherit}
.posts-menu-item:hover{background:var(--surface2);color:var(--dtext)}
.posts-menu-item svg{width:13px;height:13px;flex-shrink:0}
.posts-menu-item.danger{color:#ef4444}
.posts-menu-item.danger:hover{background:#ef444410}
.posts-empty{text-align:center;padding:60px 20px}
.posts-empty-title{font-size:16px;font-weight:600;color:var(--dtext);margin-bottom:6px}
.posts-empty-sub{font-size:13px;color:var(--dmuted)}
.posts-expand-cell{background:var(--surface);padding:18px 24px;border-bottom:1px solid var(--dborder)}
.posts-expand-layout{display:flex;flex-direction:column;gap:18px}
.posts-meta-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:12px}
.posts-meta-card{background:var(--surface2);border:1px solid var(--dborder);border-radius:10px;padding:12px 14px}
.posts-meta-label{font-size:10px;font-weight:700;letter-spacing:.08em;text-transform:uppercase;color:var(--dmuted2);margin-bottom:6px}
.posts-meta-value{font-size:13px;color:var(--dtext);line-height:1.5;word-break:break-word}
.posts-results-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:14px}
.posts-result-card{background:var(--surface2);border:1px solid var(--dborder);border-radius:10px;padding:14px}
.posts-result-head{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-bottom:8px}
.posts-result-title{display:flex;align-items:center;gap:8px;min-width:0}
.posts-result-name{font-size:13px;font-weight:600;color:var(--dtext);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.posts-result-meta{display:flex;flex-direction:column;gap:3px;margin-bottom:10px}
.posts-result-text{font-size:12px;color:var(--dmuted);line-height:1.5}
.posts-result-link{display:inline-flex;align-items:center;gap:4px;color:var(--daccent);text-decoration:none;font-size:12px;font-weight:500}
.posts-result-link:hover{text-decoration:underline}
.posts-error{font-size:11px;color:var(--danger);background:var(--danger-soft);border:1px solid color-mix(in srgb,var(--danger) 22%,transparent);border-radius:8px;padding:8px 10px;white-space:pre-wrap;word-break:break-word;font-family:var(--font-geist-mono),monospace;line-height:1.55;max-height:148px;overflow:auto}
.posts-hint{font-size:12px;color:var(--dtext);line-height:1.55}
.posts-hint-label{color:var(--dmuted)}
.posts-row-toggle{display:inline-flex;align-items:center;justify-content:center;width:20px;height:20px;color:var(--dmuted2)}
@media (max-width: 900px){.posts-expand-cell{padding:14px 16px}.posts-results-grid{grid-template-columns:1fr}}
`;

export default function PostsPage() {
  const { id: profileId } = useParams<{ id: string }>();
  const workspaceId = useWorkspaceId();
  const { getToken } = useAuth();
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [posts, setPosts] = useState<SocialPost[]>([]);
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<FilterTab>("all");
  const [search, setSearch] = useState("");
  const [platformFilter, setPlatformFilter] = useState("all");
  const [menuOpen, setMenuOpen] = useState<string | null>(null);
  const [expandedPostId, setExpandedPostId] = useState<string | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  const loadData = useCallback(async () => {
    if (!workspaceId) return; // wait for workspace resolution
    try {
      const token = await getToken();
      if (!token) return;
      const [a, p] = await Promise.all([
        listSocialAccounts(token, profileId),
        listSocialPosts(token, workspaceId),
      ]);
      setAccounts(a.data);
      setPosts(p.data);
      // Load profiles separately — don't block posts/accounts if it fails
      try {
        const pr = await listProfiles(token);
        setProfiles(pr.data);
      } catch (profileErr) {
        console.error("Failed to load profiles:", profileErr);
      }
    } catch (err) {
      console.error("Failed to load:", err);
    } finally {
      setLoading(false);
    }
  }, [getToken, profileId, workspaceId]);

  useEffect(() => { loadData(); }, [loadData]);

  // Close menu on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setMenuOpen(null);
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  async function handleCancel(postId: string) {
    try {
      const token = await getToken();
      if (!token) return;
      await cancelSocialPost(token, postId);
      loadData();
    } catch (err) { console.error("Cancel failed:", err); }
    setMenuOpen(null);
  }

  async function handleDuplicate(post: SocialPost) {
    // TODO: open create modal pre-filled with post content
    setMenuOpen(null);
  }

  // Filter logic
  const filtered = posts.filter((p) => {
    // Tab filter
    if (tab === "published" && p.status !== "published") return false;
    if (tab === "scheduled" && p.status !== "scheduled") return false;
    if (tab === "failed" && p.status !== "failed" && p.status !== "partial") return false;
    if (tab === "draft" && p.status !== "draft") return false;
    // Search
    if (search && !(p.caption || "").toLowerCase().includes(search.toLowerCase())) return false;
    // Platform filter
    if (platformFilter !== "all") {
      const hasPlatform = p.results?.some((r) => r.platform === platformFilter);
      if (!hasPlatform) return false;
    }
    return true;
  });

  const tabCounts = {
    all: posts.length,
    published: posts.filter((p) => p.status === "published").length,
    scheduled: posts.filter((p) => p.status === "scheduled").length,
    failed: posts.filter((p) => p.status === "failed" || p.status === "partial").length,
    draft: posts.filter((p) => p.status === "draft").length,
  };

  function getTime(post: SocialPost) {
    const d = post.status === "scheduled" ? post.scheduled_at : post.published_at || post.created_at;
    if (!d) return "";
    return new Date(d).toLocaleDateString("en-US", { month: "short", day: "numeric" });
  }

  function platformIcons(post: SocialPost) {
    const platforms = [...new Set(post.results?.map((r) => r.platform).filter(Boolean) || [])];
    const show = platforms.slice(0, 4);
    const more = platforms.length - show.length;
    return (
      <div className="posts-plats">
        {show.map((p) => <PlatformIcon key={p} platform={p!} size={14} />)}
        {more > 0 && <span className="posts-plats-more">+{more}</span>}
      </div>
    );
  }

  function actionsMenu(post: SocialPost) {
    const items: { icon: React.ReactNode; label: string; action: () => void; danger?: boolean }[] = [
      { icon: <Eye />, label: "View details", action: () => { setExpandedPostId((current) => current === post.id ? null : post.id); setMenuOpen(null); } },
      { icon: <Copy />, label: "Duplicate", action: () => handleDuplicate(post) },
    ];
    if (post.status === "draft") {
      items.push({ icon: <Pencil />, label: "Edit", action: () => { setMenuOpen(null); } });
      items.push({ icon: <Send />, label: "Publish now", action: () => { setMenuOpen(null); } });
      items.push({ icon: <Calendar />, label: "Schedule", action: () => { setMenuOpen(null); } });
      items.push({ icon: <XCircle />, label: "Delete", action: () => { setMenuOpen(null); }, danger: true });
    }
    if (post.status === "scheduled") {
      items.push({ icon: <Calendar />, label: "Edit scheduled time", action: () => { setMenuOpen(null); } });
      items.push({ icon: <XCircle />, label: "Cancel", action: () => handleCancel(post.id), danger: true });
    }
    return items;
  }

  if (loading) return <div style={{ color: "var(--dmuted)", padding: 20 }}>Loading...</div>;

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />

      {/* Header */}
      <div className="posts-header">
        <div>
          <div className="dt-page-title">Posts</div>
          <div className="dt-subtitle">Published and scheduled content</div>
        </div>
        <button className="dbtn dbtn-primary" style={{ gap: 5 }} onClick={() => setShowCreateModal(true)}>
          <Plus style={{ width: 14, height: 14 }} /> Create
        </button>
      </div>

      {/* Tabs + search + platform filter — single row */}
      <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 16, flexWrap: "wrap" }}>
        <div className="dtabs" style={{ marginBottom: 0 }}>
          {(["all", "published", "scheduled", "failed", "draft"] as FilterTab[]).map((t) => (
            <div key={t} className={`dtab ${tab === t ? "active" : ""}`} onClick={() => setTab(t)}>
              {t === "all" ? "All" : t.charAt(0).toUpperCase() + t.slice(1)}
              {t === "draft" ? "s" : ""}
              <span style={{ fontSize: 10, color: "var(--dmuted2)", marginLeft: 4 }}>
                {tabCounts[t]}
              </span>
            </div>
          ))}
        </div>
        <div style={{ flex: 1 }} />
        <div className="posts-search">
          <Search style={{ width: 13, height: 13 }} />
          <input placeholder="Search posts..." value={search} onChange={(e) => setSearch(e.target.value)} />
        </div>
        <select className="posts-select" value={platformFilter} onChange={(e) => setPlatformFilter(e.target.value)}>
          <option value="all">All platforms</option>
          {["twitter", "linkedin", "instagram", "threads", "tiktok", "youtube", "bluesky"].map((p) => (
            <option key={p} value={p}>{p.charAt(0).toUpperCase() + p.slice(1)}</option>
          ))}
        </select>
      </div>

      {/* Table */}
      {filtered.length === 0 ? (
        <div className="posts-empty">
          <div className="posts-empty-title">
            {posts.length === 0 ? "No posts yet" : `No ${tab === "all" ? "" : tab + " "}posts found`}
          </div>
          <div className="posts-empty-sub">
            {posts.length === 0
              ? "Create your first post to get started."
              : "Try adjusting your filters."}
          </div>
        </div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Caption</th>
                <th style={{ width: 100 }}>Platforms</th>
                <th style={{ width: 110 }}>Status</th>
                <th style={{ width: 100 }}>Time</th>
                <th style={{ width: 48 }}></th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((post) => {
                const isExpanded = expandedPostId === post.id;
                return (
                  <Fragment key={post.id}>
                    <tr className="posts-row" onClick={() => setExpandedPostId((current) => current === post.id ? null : post.id)}>
                      <td>
                        <span style={{ display: "flex", alignItems: "center", gap: 10 }}>
                          <span className="posts-row-toggle">
                            {isExpanded ? (
                              <ChevronDown style={{ width: 14, height: 14 }} />
                            ) : (
                              <ChevronRight style={{ width: 14, height: 14 }} />
                            )}
                          </span>
                          <span className="posts-caption" title={post.caption || undefined}>
                            {post.caption || "(no caption)"}
                          </span>
                        </span>
                      </td>
                      <td>{platformIcons(post)}</td>
                      <td>{statusBadge(post.status)}</td>
                      <td><span className="posts-time">{getTime(post)}</span></td>
                      <td>
                        <div className="posts-actions" ref={menuOpen === post.id ? menuRef : undefined}>
                          <button
                            className="posts-actions-btn"
                            onClick={(e) => { e.stopPropagation(); setMenuOpen(menuOpen === post.id ? null : post.id); }}
                          >
                            <MoreHorizontal style={{ width: 16, height: 16 }} />
                          </button>
                          {menuOpen === post.id && (
                            <div className="posts-menu">
                              {actionsMenu(post).map((item) => (
                                <button
                                  key={item.label}
                                  className={`posts-menu-item${item.danger ? " danger" : ""}`}
                                  onClick={(e) => { e.stopPropagation(); item.action(); }}
                                >
                                  {item.icon}
                                  {item.label}
                                </button>
                              ))}
                            </div>
                          )}
                        </div>
                      </td>
                    </tr>
                    {isExpanded && (
                      <tr>
                        <td colSpan={5} className="posts-expand-cell">
                          <div className="posts-expand-layout">
                            <div className="posts-meta-grid">
                              <MetaCard label="Caption" value={post.caption || "(no caption)"} />
                              <MetaCard label="Mode" value={post.scheduled_at ? "Scheduled" : post.status === "draft" ? "Draft" : "Immediate"} />
                              <MetaCard label="Status" value={post.status} />
                              <MetaCard label="Created" value={formatLongDate(post.created_at)} />
                              <MetaCard label="Published" value={post.published_at ? formatLongDate(post.published_at) : "—"} />
                            </div>
                            <div>
                              <div className="posts-meta-label" style={{ marginBottom: 10 }}>Platform Results</div>
                              <PostResultsGrid post={post} workspaceId={workspaceId} />
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Create post drawer */}
      <CreatePostDrawer
        open={showCreateModal}
        onOpenChange={setShowCreateModal}
        accounts={accounts}
        workspaceId={workspaceId}
        getToken={getToken}
        onCreated={() => { loadData(); if (tab !== "all") setTab("all"); }}
      />
    </>
  );
}

function MetaCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="posts-meta-card">
      <div className="posts-meta-label">{label}</div>
      <div className="posts-meta-value">{value}</div>
    </div>
  );
}

function PostResultsGrid({ post, workspaceId }: { post: SocialPost; workspaceId: string }) {
  const results = post.results || [];
  if (results.length === 0) {
    return <div className="posts-result-text">No platform results yet.</div>;
  }
  return (
    <div className="posts-results-grid">
      {results.map((result) => (
        <PostResultCard key={result.social_account_id} post={post} workspaceId={workspaceId} result={result} />
      ))}
    </div>
  );
}

function PostResultCard({
  post,
  result,
  workspaceId,
}: {
  post: SocialPost;
  result: NonNullable<SocialPost["results"]>[number];
  workspaceId: string;
}) {
  const url = result.external_id && result.platform ? postUrlFor(result.platform, result.external_id) : null;
  const hint = result.status === "failed" ? categorizeError(result.error_message || "") : null;

  return (
    <div className="posts-result-card">
      <div className="posts-result-head">
        <div className="posts-result-title">
          <PlatformIcon platform={result.platform || ""} size={15} />
          <span className="posts-result-name">{result.account_name || result.platform || "Unknown"}</span>
          <InlineStatusPill status={result.status} />
        </div>
        {url ? (
          <a
            href={url}
            target="_blank"
            rel="noreferrer"
            onClick={(e) => e.stopPropagation()}
            className="posts-result-link"
            title="Open original post"
          >
            <ExternalLink style={{ width: 12, height: 12 }} />
          </a>
        ) : null}
      </div>

      <div className="posts-result-meta">
        {result.account_name ? <div className="posts-result-text">{result.platform || "Unknown"}</div> : null}
        <div className="posts-result-text">
          {result.published_at ? formatLongDate(result.published_at) : post.published_at ? formatLongDate(post.published_at) : "Not published yet"}
        </div>
      </div>

      {result.status === "failed" ? (
        <>
          <div className="posts-error">
            {result.error_message || "Publish failed (no error message reported)."}
          </div>
          {hint ? (
            <div className="posts-hint" style={{ marginTop: 10 }}>
              <span className="posts-hint-label">{hint.label}: </span>
              {hint.body}
              {hint.action ? (
                <>
                  {" "}
                  {hint.action.href.startsWith("http") ? (
                    <a href={hint.action.href} target="_blank" rel="noreferrer" className="posts-result-link">
                      {hint.action.label}
                    </a>
                  ) : (
                    <Link href={hint.action.href.replace(":id", workspaceId)} className="posts-result-link">
                      {hint.action.label}
                    </Link>
                  )}
                </>
              ) : null}
            </div>
          ) : null}
        </>
      ) : (
        <div className="posts-hint">
          {result.status === "published" ? "Published successfully." : result.status === "partial" ? "Partially completed. Review other platform cards for failures." : `Status: ${result.status}`}
          {result.external_id ? <div className="posts-result-text" style={{ marginTop: 10 }}>ID: {result.external_id}</div> : null}
        </div>
      )}
    </div>
  );
}

function postUrlFor(platform: string, externalId: string): string | null {
  switch (platform) {
    case "youtube":
      return `https://www.youtube.com/watch?v=${externalId}`;
    case "twitter":
      return `https://x.com/i/status/${externalId}`;
    case "instagram":
      return `https://www.instagram.com/p/${externalId}/`;
    case "threads":
      return `https://www.threads.net/post/${externalId}`;
    case "linkedin":
      if (externalId.startsWith("urn:li:")) return `https://www.linkedin.com/feed/update/${externalId}/`;
      return null;
    case "bluesky": {
      const match = externalId.match(/^at:\/\/(did:[^/]+)\/app\.bsky\.feed\.post\/(.+)$/);
      return match ? `https://bsky.app/profile/${match[1]}/post/${match[2]}` : null;
    }
    default:
      return null;
  }
}

function formatLongDate(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

type ErrorHint = {
  label: string;
  body: string;
  action?: { label: string; href: string };
};

function categorizeError(error: string): ErrorHint | null {
  const e = error.toLowerCase();
  if (e.includes("account is disconnected") || e.includes("account not found")) {
    return { label: "What to do", body: "The connected social account was disconnected before this post was published.", action: { label: "Reconnect on Accounts page", href: "/projects/:id/accounts" } };
  }
  if (e.includes("token") && (e.includes("expired") || e.includes("invalid") || e.includes("revoked") || e.includes("unauthorized"))) {
    return { label: "What to do", body: "The platform access token is no longer valid.", action: { label: "Reconnect on Accounts page", href: "/projects/:id/accounts" } };
  }
  if (e.includes("rate limit") || e.includes("too many requests") || e.includes("429")) {
    return { label: "What to do", body: "The platform rate-limited this request. Wait a few minutes and retry." };
  }
  if (e.includes("instagram requires at least one")) {
    return { label: "What to do", body: "Instagram does not support text-only posts. Attach at least one image or video." };
  }
  if (e.includes("duplicate") || e.includes("duplicate_post")) {
    return { label: "Likely cause", body: "The platform rejected this post because the content looks like a duplicate of a recent post." };
  }
  return null;
}

function InlineStatusPill({ status }: { status: string }) {
  return statusBadge(status);
}
