import Link from "next/link";
import { ArrowRight, BarChart3, CheckCircle2, Clock, Video } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";

export default async function AnalyticsPlatformsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 20, marginBottom: 24 }}>
        <div>
          <div className="dt-page-title">Platform Analytics</div>
          <div className="dt-subtitle" style={{ maxWidth: 720 }}>
            Platform-specific account insights and extended metrics beyond the cross-platform Posts view.
          </div>
        </div>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(260px, 1fr))", gap: 14 }}>
        <Link
          href={`/projects/${id}/analytics/platforms/tiktok`}
          style={{
            display: "block",
            textDecoration: "none",
            color: "inherit",
            border: "1px solid var(--dborder)",
            borderRadius: 8,
            background: "var(--surface1)",
            padding: 16,
          }}
        >
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 18 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <div className="platform-icon-wrap"><PlatformIcon platform="tiktok" /></div>
              <div>
                <div style={{ color: "var(--dtext)", fontWeight: 700 }}>TikTok</div>
                <div style={{ color: "var(--dmuted)", fontSize: 12 }}>Profile, stats, videos</div>
              </div>
            </div>
            <ArrowRight style={{ width: 16, height: 16, color: "var(--dmuted2)" }} />
          </div>
          <div style={{ display: "grid", gap: 9, fontSize: 13 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
              <CheckCircle2 style={{ width: 14, height: 14, color: "var(--success)" }} />
              user.info.profile / user.info.stats / video.list
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
              <Video style={{ width: 14, height: 14 }} />
              Public video inventory and post-level metrics
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
              <Clock style={{ width: 14, height: 14 }} />
              Live data after account reconnect
            </div>
          </div>
        </Link>

        <div style={{ border: "1px dashed var(--dborder2)", borderRadius: 8, background: "var(--surface1)", padding: 16, color: "var(--dmuted)" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 10 }}>
            <BarChart3 style={{ width: 18, height: 18 }} />
            <div style={{ color: "var(--dtext)", fontWeight: 700 }}>More platforms</div>
          </div>
          <div style={{ fontSize: 13, lineHeight: 1.55 }}>
            Facebook Page Insights, YouTube channel stats, Instagram account insights, and X account metrics can use this same drilldown pattern later.
          </div>
        </div>
      </div>
    </>
  );
}
