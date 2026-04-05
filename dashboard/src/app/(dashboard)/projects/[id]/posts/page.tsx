"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  listSocialAccounts,
  listSocialPosts,
  createSocialPost,
  type SocialAccount,
  type SocialPost,
} from "@/lib/api";
import {
  Send,
  CheckCircle2,
  XCircle,
  Clock,
  AlertCircle,
  ChevronDown,
  ChevronUp,
} from "lucide-react";

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
      const [accountsRes, postsRes] = await Promise.all([
        listSocialAccounts(token, projectId),
        listSocialPosts(token, projectId),
      ]);
      setAccounts(accountsRes.data);
      setPosts(postsRes.data);
    } catch (err) {
      console.error("Failed to load data:", err);
    } finally {
      setLoading(false);
    }
  }, [getToken, projectId]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  function toggleAccount(id: string) {
    setSelectedAccounts((prev) =>
      prev.includes(id) ? prev.filter((a) => a !== id) : [...prev, id]
    );
  }

  async function handlePost() {
    if (!caption.trim() || selectedAccounts.length === 0) return;
    setPosting(true);
    setPostResult(null);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await createSocialPost(token, projectId, {
        caption: caption.trim(),
        account_ids: selectedAccounts,
      });
      setPostResult(res.data);
      setCaption("");
      setSelectedAccounts([]);
      loadData();
    } catch (err) {
      console.error("Failed to create post:", err);
    } finally {
      setPosting(false);
    }
  }

  const filteredPosts =
    filter === "all" ? posts : posts.filter((p) => p.status === filter);

  const filterCounts = {
    all: posts.length,
    published: posts.filter((p) => p.status === "published").length,
    scheduled: posts.filter((p) => p.status === "scheduled").length,
    failed: posts.filter((p) => p.status === "failed").length,
  };

  function statusBadge(status: string) {
    const map: Record<string, { class: string; icon: typeof CheckCircle2 }> = {
      published: { class: "bg-emerald/10 text-emerald", icon: CheckCircle2 },
      scheduled: { class: "bg-[#3b82f6]/10 text-[#3b82f6]", icon: Clock },
      failed: { class: "bg-destructive/10 text-destructive", icon: XCircle },
      partial: { class: "bg-amber-status/10 text-amber-status", icon: AlertCircle },
    };
    const s = map[status] || map.scheduled;
    const Icon = s.icon;
    return (
      <Badge variant="secondary" className={`text-[10px] border-0 gap-1 ${s.class}`}>
        <Icon className="w-2.5 h-2.5" />
        {status}
      </Badge>
    );
  }

  if (loading) {
    return (
      <div className="space-y-4">
        <div className="h-5 w-24 bg-[#111111] rounded animate-pulse" />
        <div className="h-44 rounded-lg bg-[#111111] border border-[#1e1e1e] animate-pulse" />
      </div>
    );
  }

  return (
    <div>
      <div className="mb-6 animate-enter">
        <h1 className="text-[18px] font-semibold text-[#e5e5e5] tracking-tight">
          Posts
        </h1>
        <p className="text-[13px] text-[#525252] mt-0.5">
          Compose, send, and track posts.
        </p>
      </div>

      {/* Compose */}
      <div className="rounded-lg bg-[#111111] border border-[#1e1e1e] p-5 mb-6 animate-enter" style={{ animationDelay: "50ms" }}>
        <div className="flex items-center justify-between mb-3">
          <Label className="text-[12px] font-medium text-[#a3a3a3]">Compose</Label>
          <span className="mono text-[10px] text-[#3a3a3a]">
            {caption.length} chars
          </span>
        </div>
        <textarea
          className="w-full min-h-[90px] rounded-md bg-[#0a0a0a] border border-[#1e1e1e] px-3 py-2.5 text-[13px] text-[#e5e5e5] placeholder:text-[#2a2a2a] resize-none focus:border-emerald/30 transition-colors"
          placeholder="What would you like to share?"
          value={caption}
          onChange={(e) => setCaption(e.target.value)}
        />

        {/* Account chips */}
        <div className="mt-3">
          {accounts.length === 0 ? (
            <p className="text-[12px] text-[#3a3a3a]">
              No accounts.{" "}
              <a href={`/projects/${projectId}/accounts`} className="text-emerald hover:underline">
                Connect one
              </a>
            </p>
          ) : (
            <>
              <Label className="text-[11px] text-[#3a3a3a] mb-2 block">Post to</Label>
              <div className="flex flex-wrap gap-1.5">
                {accounts.map((account) => {
                  const sel = selectedAccounts.includes(account.id);
                  return (
                    <button
                      key={account.id}
                      type="button"
                      onClick={() => toggleAccount(account.id)}
                      className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md border text-[11px] font-medium transition-all cursor-pointer ${
                        sel
                          ? "border-emerald/30 bg-emerald/5 text-emerald"
                          : "border-[#1e1e1e] text-[#525252] hover:border-[#2a2a2a]"
                      }`}
                    >
                      <span className={`w-1.5 h-1.5 rounded-full ${sel ? "bg-emerald" : "bg-[#2a2a2a]"}`} />
                      {account.account_name || account.platform}
                    </button>
                  );
                })}
              </div>
            </>
          )}
        </div>

        <div className="mt-4 flex items-center justify-between">
          <span className="text-[11px] text-[#3a3a3a]">
            {selectedAccounts.length > 0 &&
              `${selectedAccounts.length} account${selectedAccounts.length > 1 ? "s" : ""}`}
          </span>
          <Button
            size="sm"
            onClick={handlePost}
            disabled={posting || !caption.trim() || selectedAccounts.length === 0}
            className="gap-1.5 bg-emerald text-emerald-foreground hover:bg-emerald/90"
          >
            <Send className="w-3.5 h-3.5" />
            {posting ? "Sending..." : "Send"}
          </Button>
        </div>
      </div>

      {/* Result banner */}
      {postResult && (
        <div className={`rounded-lg border p-4 mb-6 animate-enter ${
          postResult.status === "published"
            ? "bg-emerald/5 border-emerald/10"
            : postResult.status === "partial"
              ? "bg-amber-status/5 border-amber-status/10"
              : "bg-destructive/5 border-destructive/10"
        }`}>
          <div className="flex items-center gap-2 mb-2">
            {statusBadge(postResult.status)}
          </div>
          {postResult.results?.map((r, i) => (
            <div key={i} className="flex items-center gap-2 text-[12px] mt-1.5">
              {statusBadge(r.status)}
              <span className="text-[#737373]">{r.platform || "unknown"}</span>
              {r.external_id && (
                <span className="mono text-[10px] text-[#2a2a2a] truncate max-w-[200px]">
                  {r.external_id}
                </span>
              )}
              {r.error_message && (
                <span className="text-[11px] text-destructive">{r.error_message}</span>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Filter tabs + post table */}
      {posts.length > 0 && (
        <div className="animate-enter" style={{ animationDelay: "100ms" }}>
          {/* Tabs */}
          <div className="flex items-center gap-1 mb-3">
            {(["all", "published", "scheduled", "failed"] as FilterTab[]).map((tab) => (
              <button
                key={tab}
                onClick={() => setFilter(tab)}
                className={`px-2.5 py-1 rounded-md text-[11px] font-medium transition-colors cursor-pointer ${
                  filter === tab
                    ? "bg-[#1a1a1a] text-[#e5e5e5]"
                    : "text-[#3a3a3a] hover:text-[#525252]"
                }`}
              >
                {tab.charAt(0).toUpperCase() + tab.slice(1)}
                {filterCounts[tab] > 0 && (
                  <span className="ml-1 text-[#2a2a2a]">{filterCounts[tab]}</span>
                )}
              </button>
            ))}
          </div>

          {/* Table */}
          <div className="rounded-lg border border-[#1e1e1e] overflow-hidden">
            <div className="grid grid-cols-[1fr_120px_100px_48px] gap-4 px-4 py-2.5 bg-[#0d0d0d] border-b border-[#1e1e1e]">
              <span className="text-[10px] font-medium uppercase tracking-[0.1em] text-[#3a3a3a]">Caption</span>
              <span className="text-[10px] font-medium uppercase tracking-[0.1em] text-[#3a3a3a]">Status</span>
              <span className="text-[10px] font-medium uppercase tracking-[0.1em] text-[#3a3a3a]">Created</span>
              <span />
            </div>
            {filteredPosts.length === 0 ? (
              <div className="px-4 py-8 text-center text-[12px] text-[#3a3a3a]">
                No {filter === "all" ? "" : filter + " "}posts.
              </div>
            ) : (
              filteredPosts.map((post) => {
                const isExpanded = expandedPost === post.id;
                return (
                  <div key={post.id} className="border-b border-[#1e1e1e] last:border-b-0">
                    <div
                      className="table-row grid grid-cols-[1fr_120px_100px_48px] gap-4 items-center px-4 py-2.5 cursor-pointer"
                      onClick={() => setExpandedPost(isExpanded ? null : post.id)}
                    >
                      <p className="text-[13px] text-[#d4d4d4] truncate">
                        {post.caption || "(no caption)"}
                      </p>
                      {statusBadge(post.status)}
                      <span className="mono text-[11px] text-[#3a3a3a]">
                        {new Date(post.created_at).toLocaleDateString("en-US", {
                          month: "short",
                          day: "numeric",
                        })}
                      </span>
                      <div className="flex justify-end">
                        {isExpanded ? (
                          <ChevronUp className="w-3.5 h-3.5 text-[#3a3a3a]" />
                        ) : (
                          <ChevronDown className="w-3.5 h-3.5 text-[#2a2a2a]" />
                        )}
                      </div>
                    </div>

                    {/* Expanded detail */}
                    {isExpanded && (
                      <div className="px-4 pb-3 pt-0">
                        <div className="rounded-md bg-[#0a0a0a] border border-[#1a1a1a] p-3">
                          <p className="text-[12px] text-[#a3a3a3] mb-3 whitespace-pre-wrap">
                            {post.caption}
                          </p>
                          {post.results && post.results.length > 0 && (
                            <div className="space-y-1.5">
                              <p className="text-[10px] font-medium uppercase tracking-[0.1em] text-[#3a3a3a] mb-1">
                                Per-platform results
                              </p>
                              {post.results.map((r, i) => (
                                <div key={i} className="flex items-center gap-2 text-[12px]">
                                  {statusBadge(r.status)}
                                  <span className="text-[#525252]">{r.platform || "unknown"}</span>
                                  {r.external_id && (
                                    <span className="mono text-[10px] text-[#2a2a2a] truncate max-w-[200px]">
                                      {r.external_id}
                                    </span>
                                  )}
                                  {r.error_message && (
                                    <span className="text-[11px] text-destructive">
                                      {r.error_message}
                                    </span>
                                  )}
                                </div>
                              ))}
                            </div>
                          )}
                          <p className="mono text-[10px] text-[#2a2a2a] mt-2">
                            {new Date(post.created_at).toLocaleString()}
                          </p>
                        </div>
                      </div>
                    )}
                  </div>
                );
              })
            )}
          </div>
        </div>
      )}
    </div>
  );
}
