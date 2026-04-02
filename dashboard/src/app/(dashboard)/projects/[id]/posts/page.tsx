"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  listSocialAccounts,
  listSocialPosts,
  createSocialPost,
  type SocialAccount,
  type SocialPost,
} from "@/lib/api";

export default function PostsPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [posts, setPosts] = useState<SocialPost[]>([]);
  const [loading, setLoading] = useState(true);

  // Post form state
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
    return <div className="text-muted-foreground">Loading...</div>;
  }

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Posts</h1>

      {/* Post form */}
      <Card className="mb-8">
        <CardHeader>
          <CardTitle className="text-base">Test Post</CardTitle>
          <CardDescription>
            Send a test post to your connected accounts
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="caption">Caption</Label>
            <textarea
              id="caption"
              className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              placeholder="Hello from UniPost!"
              value={caption}
              onChange={(e) => setCaption(e.target.value)}
            />
          </div>

          {accounts.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No connected accounts. Connect an account first.
            </p>
          ) : (
            <div className="space-y-2">
              <Label>Post to</Label>
              <div className="space-y-2">
                {accounts.map((account) => (
                  <label
                    key={account.id}
                    className="flex items-center gap-2 cursor-pointer"
                  >
                    <input
                      type="checkbox"
                      checked={selectedAccounts.includes(account.id)}
                      onChange={() => toggleAccount(account.id)}
                      className="rounded border-input"
                    />
                    <span className="text-sm">
                      {account.account_name || account.id} ({account.platform})
                    </span>
                  </label>
                ))}
              </div>
            </div>
          )}

          <Button
            onClick={handlePost}
            disabled={
              posting ||
              !caption.trim() ||
              selectedAccounts.length === 0
            }
          >
            {posting ? "Sending..." : "Send Post"}
          </Button>
        </CardContent>
      </Card>

      {/* Post result */}
      {postResult && (
        <Card className="mb-8 border-green-200 bg-green-50/50">
          <CardHeader>
            <CardTitle className="text-base">
              {postResult.status === "published"
                ? "Published Successfully"
                : postResult.status === "partial"
                  ? "Partially Published"
                  : "Failed"}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {postResult.results?.map((result, i) => (
              <div key={i} className="flex items-center gap-2 text-sm">
                <Badge
                  variant={
                    result.status === "published" ? "default" : "destructive"
                  }
                >
                  {result.status}
                </Badge>
                <span>{result.platform || "unknown"}</span>
                {result.external_id && (
                  <span className="text-muted-foreground font-mono text-xs truncate max-w-[300px]">
                    {result.external_id}
                  </span>
                )}
                {result.error_message && (
                  <span className="text-destructive text-xs">
                    {result.error_message}
                  </span>
                )}
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {/* Post history */}
      {posts.length > 0 && (
        <div>
          <h2 className="text-lg font-semibold mb-4">Recent Posts</h2>
          <div className="space-y-3">
            {posts.map((post) => (
              <Card key={post.id}>
                <CardHeader className="pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-sm font-medium truncate max-w-[400px]">
                      {post.caption || "(no caption)"}
                    </CardTitle>
                    <Badge
                      variant={
                        post.status === "published"
                          ? "default"
                          : post.status === "failed"
                            ? "destructive"
                            : "secondary"
                      }
                    >
                      {post.status}
                    </Badge>
                  </div>
                  <CardDescription>
                    {new Date(post.created_at).toLocaleString()}
                  </CardDescription>
                </CardHeader>
              </Card>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
