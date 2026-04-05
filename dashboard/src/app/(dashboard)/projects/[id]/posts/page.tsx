"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Label } from "@/components/ui/label";
import {
  listSocialAccounts, listSocialPosts, createSocialPost, type SocialAccount, type SocialPost,
} from "@/lib/api";
import { Send, CheckCircle2, XCircle, Clock, AlertCircle } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";

type FilterTab = "all" | "published" | "scheduled" | "failed";

export default function PostsPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [posts, setPosts] = useState<SocialPost[]>([]);
  const [loading, setLoading] = useState(true);
  const [caption, setCaption] = useState("");
  const [selectedAccounts, setSelectedAccounts] = useState<string[]>([]);
  const [posting, setPosting] = useState(false);
  const [postResult, setPostResult] = useState<SocialPost | null>(null);
  const [filter, setFilter] = useState<FilterTab>("all");
  const [expandedPost, setExpandedPost] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const [accountsRes, postsRes] = await Promise.all([listSocialAccounts(token, projectId), listSocialPosts(token, projectId)]);
      setAccounts(accountsRes.data); setPosts(postsRes.data);
    } catch (err) { console.error("Failed to load:", err); } finally { setLoading(false); }
  }, [getToken, projectId]);

  useEffect(() => { loadData(); }, [loadData]);

  function toggleAccount(id: string) {
    setSelectedAccounts((prev) => prev.includes(id) ? prev.filter((a) => a !== id) : [...prev, id]);
  }

  async function handlePost() {
    if (!caption.trim() || selectedAccounts.length === 0) return;
    setPosting(true); setPostResult(null);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await createSocialPost(token, projectId, { caption: caption.trim(), account_ids: selectedAccounts });
      setPostResult(res.data); setCaption(""); setSelectedAccounts([]); loadData();
    } catch (err) { console.error("Failed to post:", err); } finally { setPosting(false); }
  }

  const filtered = filter === "all" ? posts : posts.filter((p) => p.status === filter);

  function statusBadge(status: string) {
    const cls = status === "published" ? "dbadge-green" : status === "scheduled" ? "dbadge-blue" : status === "partial" ? "dbadge-amber" : "dbadge-red";
    return <span className={`dbadge ${cls}`}><span className="dbadge-dot" />{status}</span>;
  }

  if (loading) return <div style={{ color: "var(--dmuted)" }}>Loading...</div>;

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>Posts</div>
          <div style={{ fontSize: 14, color: "#aaa", marginTop: 6 }}>Published and scheduled social media posts</div>
        </div>
      </div>

      {/* Compose */}
      <div className="settings-section" style={{ marginBottom: 24 }}>
        <div className="settings-section-header">
          Compose
          <span style={{ fontFamily: "var(--font-geist-mono), monospace", fontSize: 11, color: "var(--dmuted2)" }}>{caption.length} chars</span>
        </div>
        <div className="settings-section-body">
          <textarea
            className="dform-input"
            style={{ minHeight: 80, resize: "none", marginBottom: 12 }}
            placeholder="What would you like to share?"
            value={caption}
            onChange={(e) => setCaption(e.target.value)}
          />
          {accounts.length === 0 ? (
            <div style={{ fontSize: 12, color: "var(--dmuted2)" }}>
              No accounts. <a href={`/projects/${projectId}/accounts`} style={{ color: "var(--daccent)" }}>Connect one</a>
            </div>
          ) : (
            <>
              <Label className="dform-label">Post to</Label>
              <div style={{ display: "flex", flexWrap: "wrap", gap: 6, marginBottom: 12 }}>
                {accounts.map((a) => {
                  const sel = selectedAccounts.includes(a.id);
                  return (
                    <button
                      key={a.id}
                      type="button"
                      onClick={() => toggleAccount(a.id)}
                      className={sel ? "dbtn dbtn-primary" : "dbtn dbtn-ghost"}
                      style={{ padding: "4px 10px", fontSize: 12, gap: 5 }}
                    >
                      <PlatformIcon platform={a.platform} size={12} />
                      {a.account_name || a.platform}
                    </button>
                  );
                })}
              </div>
            </>
          )}
          <div style={{ display: "flex", justifyContent: "flex-end" }}>
            <button className="dbtn dbtn-primary" onClick={handlePost} disabled={posting || !caption.trim() || selectedAccounts.length === 0}>
              <Send style={{ width: 13, height: 13 }} />
              {posting ? "Sending..." : "Send Post"}
            </button>
          </div>
        </div>
      </div>

      {/* Result */}
      {postResult && (
        <div style={{ padding: "10px 14px", borderRadius: 6, marginBottom: 20, background: postResult.status === "published" ? "#10b98110" : postResult.status === "partial" ? "#f59e0b10" : "#ef444410", border: `1px solid ${postResult.status === "published" ? "#10b98125" : postResult.status === "partial" ? "#f59e0b25" : "#ef444425"}` }}>
          <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 6 }}>{statusBadge(postResult.status)}</div>
          {postResult.results?.map((r, i) => (
            <div key={i} style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 12, marginTop: 4 }}>
              {statusBadge(r.status)}
              <span style={{ color: "var(--dmuted)" }}>{r.platform || "unknown"}</span>
              {r.external_id && <span className="mono" style={{ fontSize: 10 }}>{r.external_id}</span>}
              {r.error_message && <span style={{ color: "var(--danger)", fontSize: 11 }}>{r.error_message}</span>}
            </div>
          ))}
        </div>
      )}

      {/* Tabs + table */}
      {posts.length > 0 && (
        <>
          <div className="dtabs">
            {(["all", "published", "scheduled", "failed"] as FilterTab[]).map((t) => (
              <div key={t} className={`dtab ${filter === t ? "active" : ""}`} onClick={() => setFilter(t)}>
                {t.charAt(0).toUpperCase() + t.slice(1)}
              </div>
            ))}
          </div>

          {filtered.length === 0 ? (
            <div className="empty-state" style={{ padding: 40 }}>
              <div style={{ fontSize: 14, fontWeight: 500, color: "var(--dtext)", marginBottom: 4 }}>No {filter} posts</div>
              <div style={{ fontSize: 12.5, color: "var(--dmuted)" }}>Posts in this status will appear here.</div>
            </div>
          ) : (
            <div className="table-wrap">
              <table>
                <thead><tr><th>Caption</th><th>Platforms</th><th>Status</th><th>Created</th></tr></thead>
                <tbody>
                  {filtered.map((post) => (
                    <tr key={post.id} onClick={() => setExpandedPost(expandedPost === post.id ? null : post.id)} style={{ cursor: "pointer" }}>
                      <td style={{ maxWidth: 300 }}>
                        <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                          {post.caption || "(no caption)"}
                        </span>
                        {expandedPost === post.id && (
                          <div style={{ marginTop: 8, padding: 10, background: "var(--bg)", border: "1px solid var(--dborder)", borderRadius: 6, whiteSpace: "pre-wrap", fontSize: 12, color: "var(--dmuted)" }}>
                            {post.caption}
                            {post.results && post.results.length > 0 && (
                              <div style={{ marginTop: 8, borderTop: "1px solid var(--dborder)", paddingTop: 8 }}>
                                {post.results.map((r, i) => (
                                  <div key={i} style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 4 }}>
                                    {statusBadge(r.status)}
                                    <span>{r.platform}</span>
                                    {r.error_message && <span style={{ color: "var(--danger)", fontSize: 11 }}>{r.error_message}</span>}
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>
                        )}
                      </td>
                      <td>
                        <div style={{ display: "flex", gap: 4 }}>
                          {post.results?.map((r, i) => (
                            <PlatformIcon key={i} platform={r.platform || ""} size={14} />
                          ))}
                        </div>
                      </td>
                      <td>{statusBadge(post.status)}</td>
                      <td style={{ color: "var(--dmuted)" }}>
                        {new Date(post.created_at).toLocaleDateString("en-US", { month: "short", day: "numeric" })}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </>
  );
}
