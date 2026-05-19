import { ImageResponse } from "next/og";
import { getBlogPost, blogPosts } from "@/lib/blog";

export const alt = "UniPost blog post";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export function generateStaticParams() {
  return blogPosts.map((post) => ({ slug: post.slug }));
}

export default async function Image({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const post = getBlogPost(slug);
  const title = post?.title || "UniPost";
  const category = post?.category || "Engineering";

  return new ImageResponse(
    (
      <div
        style={{
          height: "100%",
          width: "100%",
          display: "flex",
          flexDirection: "column",
          justifyContent: "space-between",
          padding: "72px",
          background:
            "radial-gradient(circle at 80% 20%, rgba(37,99,235,0.45), transparent 45%), linear-gradient(135deg, #050816 0%, #0a1224 55%, #111827 100%)",
          color: "#f7f7f5",
          fontFamily:
            'system-ui, -apple-system, "Segoe UI", Roboto, sans-serif',
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
          <div
            style={{
              width: 44,
              height: 44,
              borderRadius: 12,
              background: "#f7f7f5",
              color: "#0b0d10",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              fontSize: 26,
              fontWeight: 900,
            }}
          >
            U
          </div>
          <div style={{ fontSize: 26, fontWeight: 800 }}>UniPost</div>
          <div
            style={{
              marginLeft: "auto",
              fontSize: 18,
              fontWeight: 700,
              padding: "8px 14px",
              border: "1px solid rgba(255,255,255,0.18)",
              borderRadius: 999,
              color: "#cbd5e1",
              textTransform: "uppercase",
              letterSpacing: 1.5,
            }}
          >
            {category}
          </div>
        </div>

        <div style={{ display: "flex", flexDirection: "column", gap: 28 }}>
          <div
            style={{
              fontSize: title.length > 70 ? 56 : 68,
              fontWeight: 800,
              lineHeight: 1.08,
              letterSpacing: -1,
              color: "#ffffff",
              maxWidth: 1040,
            }}
          >
            {title}
          </div>
          <div
            style={{
              fontSize: 22,
              fontWeight: 600,
              color: "#94a3b8",
              fontFamily:
                'ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace',
            }}
          >
            unipost.dev/blog
          </div>
        </div>

        <div
          style={{
            display: "flex",
            gap: 10,
            flexWrap: "wrap",
            fontSize: 15,
            fontWeight: 700,
            color: "#dbeafe",
          }}
        >
          {[
            "One API",
            "9 platforms",
            "Hosted OAuth",
            "Validation",
            "Webhooks",
          ].map((chip) => (
            <div
              key={chip}
              style={{
                padding: "8px 14px",
                border: "1px solid rgba(255,255,255,0.16)",
                background: "rgba(255,255,255,0.06)",
                borderRadius: 999,
              }}
            >
              {chip}
            </div>
          ))}
        </div>
      </div>
    ),
    size,
  );
}
