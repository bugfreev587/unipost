"use client";

// Hosted preview page for shareable draft links. Renders one column
// per platform_post entry with the per-platform caption, attached
// media, character counter against the platform's documented limit,
// and a thread-position badge for multi-tweet drafts.
//
// Token verification happens on the API side — this page is purely a
// renderer. A bad/expired/missing token surfaces as the API's 401
// which we display as a friendly error.
//
// Lives at /preview/[id] on the dashboard origin (per Sprint 2
// review B3); a dedicated preview.unipost.dev subdomain is deferred.

import { useEffect, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { countChars } from "@/lib/charcount";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "https://api.unipost.dev";

type PlatformPost = {
  account_id: string;
  platform: string;
  account_name?: string;
  caption: string;
  caption_length: number;
  caption_max: number;
  media_urls: string[];
  thread_position?: number;
};

type PreviewPayload = {
  post_id: string;
  status: string;
  created_at: string;
  scheduled_at?: string;
  platform_posts: PlatformPost[];
};

export default function PreviewPage() {
  const { id } = useParams<{ id: string }>();
  const search = useSearchParams();
  const token = search.get("token") || "";
  const [data, setData] = useState<PreviewPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      if (!token) {
        setError("Missing preview token in URL.");
        setLoading(false);
        return;
      }
      try {
        const res = await fetch(
          `${API_URL}/v1/public/drafts/${encodeURIComponent(id)}?token=${encodeURIComponent(token)}`,
        );
        if (!res.ok) {
          // Body shape is { error: { code, message } }.
          const body = await res.json().catch(() => ({}));
          const msg = body?.error?.message || `Preview API returned ${res.status}`;
          if (!cancelled) setError(msg);
        } else {
          const body = await res.json();
          if (!cancelled) setData(body.data);
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [id, token]);

  if (loading) {
    return (
      <div style={{ padding: 48, fontFamily: "var(--font-sans, sans-serif)", color: "#888" }}>
        Loading preview…
      </div>
    );
  }
  if (error) {
    return (
      <div style={{ padding: 48, fontFamily: "var(--font-sans, sans-serif)", maxWidth: 640 }}>
        <h1 style={{ fontSize: 24, fontWeight: 700, marginBottom: 12 }}>
          Preview unavailable
        </h1>
        <p style={{ color: "#666", lineHeight: 1.6 }}>{error}</p>
        <p style={{ color: "#999", marginTop: 24, fontSize: 13 }}>
          Preview links expire 24 hours after they&rsquo;re created. Ask the
          author for a fresh link.
        </p>
      </div>
    );
  }
  if (!data) return null;

  return (
    <div
      style={{
        minHeight: "100vh",
        background: "#0a0a0a",
        color: "#f0f0f0",
        padding: "48px 24px",
        fontFamily: "var(--font-sans, system-ui, sans-serif)",
      }}
    >
      <div style={{ maxWidth: 1280, margin: "0 auto" }}>
        <header style={{ marginBottom: 32 }}>
          <h1 style={{ fontSize: 28, fontWeight: 700, marginBottom: 4 }}>
            Draft preview
          </h1>
          <p style={{ color: "#888", fontSize: 14 }}>
            {data.platform_posts.length} platform
            {data.platform_posts.length === 1 ? "" : "s"} · created{" "}
            {new Date(data.created_at).toLocaleString()}
            {data.scheduled_at && (
              <> · scheduled for {new Date(data.scheduled_at).toLocaleString()}</>
            )}
          </p>
        </header>

        <div
          style={{
            display: "grid",
            gridTemplateColumns: `repeat(auto-fill, minmax(320px, 1fr))`,
            gap: 16,
          }}
        >
          {data.platform_posts.map((pp, i) => (
            <PlatformCard key={i} post={pp} />
          ))}
        </div>

        <footer style={{ marginTop: 48, textAlign: "center", color: "#555", fontSize: 12 }}>
          Powered by{" "}
          <a href="https://unipost.dev" style={{ color: "#888" }}>
            UniPost
          </a>{" "}
          · Read-only · Link expires in 24h
        </footer>
      </div>
    </div>
  );
}

function PlatformCard({ post }: { post: PlatformPost }) {
  // Sprint 3 PR9: per-platform char counter using the local helper
  // (Twitter URL collapsing, Bluesky grapheme count, others raw
  // length). Falls back to the API's pre-computed values when the
  // platform isn't recognized.
  const local = countChars(post.platform, post.caption);
  const captionLength = local.limit > 0 ? local.used : post.caption_length;
  const captionMax = local.limit > 0 ? local.limit : post.caption_max;
  const overLimit = captionMax > 0 && captionLength > captionMax;
  return (
    <div
      style={{
        background: "#141414",
        border: "1px solid #262626",
        borderRadius: 12,
        padding: 18,
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          marginBottom: 12,
        }}
      >
        <div>
          <div
            style={{
              fontSize: 14,
              fontWeight: 600,
              textTransform: "capitalize",
            }}
          >
            {post.platform}
          </div>
          {post.account_name && (
            <div style={{ fontSize: 12, color: "#888" }}>{post.account_name}</div>
          )}
        </div>
        {post.thread_position && post.thread_position > 0 && (
          <span
            style={{
              fontSize: 11,
              fontWeight: 600,
              color: "#10b981",
              background: "#10b98110",
              border: "1px solid #10b98130",
              padding: "2px 8px",
              borderRadius: 999,
            }}
          >
            {post.thread_position}/N
          </span>
        )}
      </div>

      <p
        style={{
          fontSize: 14,
          lineHeight: 1.55,
          color: "#e8e8e8",
          whiteSpace: "pre-wrap",
          wordBreak: "break-word",
          marginBottom: 12,
        }}
      >
        {post.caption || (
          <span style={{ color: "#555", fontStyle: "italic" }}>(no caption)</span>
        )}
      </p>

      {post.media_urls && post.media_urls.length > 0 && (
        <div
          style={{
            display: "grid",
            gridTemplateColumns:
              post.media_urls.length === 1
                ? "1fr"
                : "repeat(2, 1fr)",
            gap: 6,
            marginBottom: 12,
          }}
        >
          {post.media_urls.map((u, i) => (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              key={i}
              src={u}
              alt=""
              style={{
                width: "100%",
                aspectRatio: "1",
                objectFit: "cover",
                borderRadius: 8,
                background: "#222",
              }}
              onError={(e) => {
                // Soft fail for broken / pending media
                (e.currentTarget as HTMLImageElement).style.display = "none";
              }}
            />
          ))}
        </div>
      )}

      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          fontSize: 12,
          color: overLimit ? "#ef4444" : "#666",
          paddingTop: 10,
          borderTop: "1px solid #262626",
        }}
        title="Approximate. Each platform counts characters slightly differently."
      >
        <span>
          {captionLength.toLocaleString()} / {captionMax.toLocaleString() || "∞"}
        </span>
        <span style={{ color: "#444", fontSize: 10 }}>per-platform</span>
      </div>
    </div>
  );
}
