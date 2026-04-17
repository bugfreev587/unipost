import { ImageResponse } from "next/og";
import { youtube } from "../_config/platforms";

export const alt = "YouTube API for Developers | UniPost";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default async function Image() {
  const cfg = youtube;
  return new ImageResponse(
    (
      <div style={{ background: "#000", width: "100%", height: "100%", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", fontFamily: "system-ui, sans-serif" }}>
        <div style={{ display: "flex", alignItems: "center", gap: "16px", marginBottom: "32px" }}>
          <div style={{ width: "64px", height: "64px", background: "#10b981", borderRadius: "14px", display: "flex", alignItems: "center", justifyContent: "center", fontSize: "32px", color: "#000", fontWeight: 900 }}>⚡</div>
          <span style={{ fontSize: "28px", color: "#999", fontWeight: 600 }}>UniPost</span>
        </div>
        <div style={{ fontSize: "72px", fontWeight: 900, color: "#f0f0f0", letterSpacing: "-2px", textAlign: "center", lineHeight: 1.1, marginBottom: "20px" }}>{`${cfg.icon} ${cfg.name} API`}</div>
        <div style={{ fontSize: "72px", fontWeight: 900, color: "#10b981", letterSpacing: "-2px" }}>for Developers</div>
        <div style={{ marginTop: "32px", fontSize: "22px", color: "#666" }}>Post programmatically · Free 100 posts/month · Setup in 5 min</div>
      </div>
    ),
    { ...size }
  );
}
