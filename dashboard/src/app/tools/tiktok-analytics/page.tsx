import type { Metadata } from "next";
import { TikTokAnalyticsView } from "@/components/analytics/tiktok-analytics-view";

export const metadata: Metadata = {
  title: "TikTok Analytics Dashboard Preview | UniPost",
  description:
    "Local preview of UniPost's TikTok analytics dashboard surface for user.info.profile, user.info.stats, and video.list.",
};

export default function PublicTikTokAnalyticsPreview() {
  return (
    <main
      className="dashboard-typography"
      style={{
        padding: "28px 32px 72px",
        background:
          "radial-gradient(circle at top right, color-mix(in srgb, var(--accent-glow) 100%, transparent), transparent 28%), var(--app-bg)",
      }}
    >
      <div className="dashboard-page-frame">
        <TikTokAnalyticsView preview />
      </div>
    </main>
  );
}
