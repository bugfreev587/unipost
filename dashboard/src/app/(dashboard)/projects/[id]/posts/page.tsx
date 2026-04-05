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
import { Send, CheckCircle2, XCircle, Clock, AlertCircle } from "lucide-react";

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

  if (loading) {
    return (
      <div className="space-y-4">
        <div className="h-6 w-32 bg-muted rounded animate-pulse" />
        <div className="h-48 rounded-lg bg-muted/50 animate-pulse" />
      </div>
    );
  }

  const charCount = caption.length;

  return (
    <div>
      <div className="mb-6 animate-fade-up">
        <h1 className="text-xl font-semibold tracking-tight">Posts</h1>
        <p className="text-[13px] text-muted-foreground mt-1">
          Compose and send posts to connected social accounts.
        </p>
      </div>

      {/* Compose */}
      <div
        className="rounded-lg border border-border bg-card p-5 mb-6 animate-fade-up"
        style={{ animationDelay: "60ms" }}
      >
        <div className="flex items-center justify-between mb-4">
          <Label className="text-[13px] font-medium">Compose</Label>
          <span className="mono-data text-[11px] text-muted-foreground">
            {charCount} chars
          </span>
        </div>

        <textarea
          className="w-full min-h-[100px] rounded-md border border-input bg-background px-3 py-2.5 text-[14px] placeholder:text-muted-foreground/60 resize-none focus-visible:ring-1 focus-visible:ring-ring transition-shadow"
          placeholder="What would you like to share?"
          value={caption}
          onChange={(e) => setCaption(e.target.value)}
        />

        {/* Account selector */}
        <div className="mt-4">
          {accounts.length === 0 ? (
            <p className="text-[12px] text-muted-foreground">
              No connected accounts.{" "}
              <a
                href={`/projects/${projectId}/accounts`}
                className="text-foreground underline underline-offset-2"
              >
                Connect one
              </a>{" "}
              to start posting.
            </p>
          ) : (
            <>
              <Label className="text-[12px] text-muted-foreground mb-2 block">
                Post to
              </Label>
              <div className="flex flex-wrap gap-2">
                {accounts.map((account) => {
                  const selected = selectedAccounts.includes(account.id);
                  return (
                    <button
                      key={account.id}
                      type="button"
                      onClick={() => toggleAccount(account.id)}
                      className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md border text-[12px] font-medium transition-all cursor-pointer ${
                        selected
                          ? "border-foreground/20 bg-foreground/[0.04] text-foreground"
                          : "border-border bg-transparent text-muted-foreground hover:border-foreground/10 hover:text-foreground"
                      }`}
                    >
                      <span
                        className={`w-1.5 h-1.5 rounded-full ${
                          selected ? "bg-foreground" : "bg-muted-foreground/30"
                        }`}
                      />
                      {account.account_name || account.platform}
                      <span className="text-muted-foreground/60">
                        {account.platform}
                      </span>
                    </button>
                  );
                })}
              </div>
            </>
          )}
        </div>

        {/* Submit */}
        <div className="mt-4 flex items-center justify-between">
          <span className="text-[11px] text-muted-foreground">
            {selectedAccounts.length > 0 &&
              `${selectedAccounts.length} account${selectedAccounts.length > 1 ? "s" : ""} selected`}
          </span>
          <Button
            size="sm"
            onClick={handlePost}
            disabled={posting || !caption.trim() || selectedAccounts.length === 0}
            className="gap-1.5"
          >
            <Send className="w-3.5 h-3.5" />
            {posting ? "Sending..." : "Send Post"}
          </Button>
        </div>
      </div>

      {/* Result banner */}
      {postResult && (
        <div
          className={`rounded-lg border p-4 mb-6 animate-fade-up ${
            postResult.status === "published"
              ? "border-foreground/10 bg-foreground/[0.02]"
              : postResult.status === "partial"
                ? "border-amber/30 bg-amber/5"
                : "border-destructive/30 bg-destructive/5"
          }`}
        >
          <div className="flex items-center gap-2 mb-3">
            {postResult.status === "published" ? (
              <CheckCircle2 className="w-4 h-4 text-foreground/70" />
            ) : postResult.status === "partial" ? (
              <AlertCircle className="w-4 h-4 text-amber" />
            ) : (
              <XCircle className="w-4 h-4 text-destructive" />
            )}
            <span className="text-[13px] font-medium">
              {postResult.status === "published"
                ? "Published"
                : postResult.status === "partial"
                  ? "Partially Published"
                  : "Failed"}
            </span>
          </div>
          <div className="space-y-1.5">
            {postResult.results?.map((result, i) => (
              <div key={i} className="flex items-center gap-2 text-[12px]">
                <Badge
                  variant={
                    result.status === "published" ? "default" : "destructive"
                  }
                  className="text-[10px]"
                >
                  {result.status}
                </Badge>
                <span className="text-muted-foreground">
                  {result.platform || "unknown"}
                </span>
                {result.external_id && (
                  <span className="mono-data text-[10px] text-muted-foreground/60 truncate max-w-[250px]">
                    {result.external_id}
                  </span>
                )}
                {result.error_message && (
                  <span className="text-destructive text-[11px]">
                    {result.error_message}
                  </span>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Post history */}
      {posts.length > 0 && (
        <div className="animate-fade-up" style={{ animationDelay: "120ms" }}>
          <p className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground mb-3">
            Recent Posts
          </p>
          <div className="space-y-1.5">
            {posts.map((post) => (
              <div
                key={post.id}
                className="flex items-center justify-between px-4 py-2.5 rounded-md border border-border bg-card"
              >
                <div className="flex items-center gap-3 min-w-0 flex-1">
                  <div className="shrink-0">
                    {post.status === "published" ? (
                      <CheckCircle2 className="w-3.5 h-3.5 text-foreground/40" />
                    ) : post.status === "failed" ? (
                      <XCircle className="w-3.5 h-3.5 text-destructive/60" />
                    ) : (
                      <Clock className="w-3.5 h-3.5 text-muted-foreground/40" />
                    )}
                  </div>
                  <p className="text-[13px] truncate min-w-0">
                    {post.caption || "(no caption)"}
                  </p>
                </div>
                <div className="flex items-center gap-3 shrink-0 ml-4">
                  <Badge
                    variant={
                      post.status === "published"
                        ? "secondary"
                        : post.status === "failed"
                          ? "destructive"
                          : "secondary"
                    }
                    className="text-[10px]"
                  >
                    {post.status}
                  </Badge>
                  <span className="mono-data text-[11px] text-muted-foreground">
                    {new Date(post.created_at).toLocaleDateString("en-US", {
                      month: "short",
                      day: "numeric",
                      hour: "2-digit",
                      minute: "2-digit",
                    })}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
