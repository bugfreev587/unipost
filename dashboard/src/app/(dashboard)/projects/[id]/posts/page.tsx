"use client";

import { useCallback, useEffect, useState, useRef } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { useWorkspaceId } from "@/lib/use-workspace-id";
import {
  listSocialAccounts, listSocialPosts, cancelSocialPost, listProfiles,
  type SocialAccount, type SocialPost, type Profile,
} from "@/lib/api";
import { Plus, Search, MoreHorizontal, Eye, Copy, Pencil, Send, XCircle, Calendar } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";
import { PostDetailDrawer } from "@/components/dashboard/post-detail-drawer";
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
const CSS = `.dbadge-gray{background:#ffffff08;color:#666;border:1px solid #333}
.posts-header{display:flex;align-items:flex-start;justify-content:space-between;margin-bottom:24px}
.posts-filters{display:flex;align-items:center;gap:8px;margin-bottom:16px}
.posts-search{display:flex;align-items:center;gap:6px;background:var(--surface2);border:1px solid var(--dborder);border-radius:6px;padding:0 10px;height:32px;flex:0 1 240px}
.posts-search input{background:none;border:none;outline:none;color:var(--dtext);font-size:12.5px;font-family:inherit;width:100%}
.posts-search input::placeholder{color:var(--dmuted2)}
.posts-search svg{color:var(--dmuted2);flex-shrink:0}
.posts-select{background:var(--surface2);border:1px solid var(--dborder);border-radius:6px;padding:0 10px;height:32px;color:var(--dtext);font-size:12.5px;font-family:inherit;cursor:pointer;outline:none}
.posts-select:focus{border-color:var(--daccent)}
.posts-row{cursor:pointer;transition:background .1s}
.posts-row:hover{background:var(--surface2)}
.posts-caption{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:360px;font-size:13.5px;color:var(--dtext)}
.posts-plats{display:flex;gap:3px;align-items:center}
.posts-plats-more{font-size:10px;color:var(--dmuted2);font-weight:600}
.posts-time{font-size:12.5px;color:var(--dmuted)}
.posts-actions{position:relative}
.posts-actions-btn{background:none;border:1px solid transparent;border-radius:4px;padding:4px;cursor:pointer;color:var(--dmuted2);transition:all .1s;display:flex;align-items:center}
.posts-actions-btn:hover{background:var(--surface2);border-color:var(--dborder);color:var(--dtext)}
.posts-menu{position:absolute;right:0;top:100%;margin-top:4px;background:var(--surface1);border:1px solid var(--dborder);border-radius:8px;padding:4px;min-width:180px;z-index:20;box-shadow:0 8px 24px #00000060}
.posts-menu-item{display:flex;align-items:center;gap:8px;padding:7px 10px;border-radius:5px;font-size:12.5px;color:var(--dmuted);cursor:pointer;transition:all .1s;border:none;background:none;width:100%;text-align:left;font-family:inherit}
.posts-menu-item:hover{background:var(--surface2);color:var(--dtext)}
.posts-menu-item svg{width:13px;height:13px;flex-shrink:0}
.posts-menu-item.danger{color:#ef4444}
.posts-menu-item.danger:hover{background:#ef444410}
.posts-empty{text-align:center;padding:60px 20px}
.posts-empty-title{font-size:15px;font-weight:600;color:var(--dtext);margin-bottom:6px}
.posts-empty-sub{font-size:13px;color:var(--dmuted)}`;

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
  const [drawerPost, setDrawerPost] = useState<SocialPost | null>(null);
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
      { icon: <Eye />, label: "View details", action: () => { setDrawerPost(post); setMenuOpen(null); } },
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
          <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>Posts</div>
          <div style={{ fontSize: 14, color: "#aaa", marginTop: 6 }}>Published and scheduled content</div>
        </div>
        <button className="dbtn dbtn-primary" style={{ gap: 5 }} onClick={() => setShowCreateModal(true)}>
          <Plus style={{ width: 14, height: 14 }} /> Create
        </button>
      </div>

      {/* Tabs */}
      <div className="dtabs" style={{ marginBottom: 16 }}>
        {(["all", "published", "scheduled", "failed", "draft"] as FilterTab[]).map((t) => (
          <div key={t} className={`dtab ${tab === t ? "active" : ""}`} onClick={() => setTab(t)}>
            {t === "all" ? "All" : t.charAt(0).toUpperCase() + t.slice(1)}
            {t === "failed" ? "s" : t === "draft" ? "s" : ""}
            <span style={{ fontSize: 10, color: "var(--dmuted2)", marginLeft: 4 }}>
              {tabCounts[t]}
            </span>
          </div>
        ))}
      </div>

      {/* Filters */}
      <div className="posts-filters">
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
              {filtered.map((post) => (
                <tr key={post.id} className="posts-row" onClick={() => setDrawerPost(post)}>
                  <td>
                    <span className="posts-caption" title={post.caption || undefined}>
                      {post.caption || "(no caption)"}
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
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Detail drawer */}
      {drawerPost && (
        <PostDetailDrawer
          post={drawerPost}
          onClose={() => setDrawerPost(null)}
          onDuplicate={() => { setDrawerPost(null); setShowCreateModal(true); }}
        />
      )}

      {/* Create post drawer */}
      <CreatePostDrawer
        open={showCreateModal}
        onOpenChange={setShowCreateModal}
        accounts={accounts}
        profiles={profiles}
        initialProfileId={profileId}
        workspaceId={workspaceId}
        getToken={getToken}
        onCreated={() => { loadData(); if (tab !== "all") setTab("all"); }}
      />
    </>
  );
}
