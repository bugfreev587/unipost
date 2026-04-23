import { ImageResponse } from "next/og";

export const alt = "UniPost Platform API";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

function getPlatformMark(platformName: string) {
  return platformName
    .split(/\s+/)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("")
    .slice(0, 2);
}

export function createOgImage(platformName: string, brandColor: string) {
  return async function Image() {
    const platformMark = getPlatformMark(platformName);

    return new ImageResponse(
      (
        <div
          style={{
            background: "#000",
            width: "100%",
            height: "100%",
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            fontFamily: "system-ui, sans-serif",
          }}
        >
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: "16px",
              marginBottom: "32px",
            }}
          >
            <div
              style={{
                width: "64px",
                height: "64px",
                background: "#10b981",
                borderRadius: "14px",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                fontSize: "24px",
                color: "#000",
                fontWeight: 900,
                letterSpacing: "-0.04em",
                textTransform: "uppercase",
              }}
            >
              UP
            </div>
            <span style={{ fontSize: "28px", color: "#999", fontWeight: 600 }}>UniPost</span>
          </div>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: "18px",
              textAlign: "center",
              marginBottom: "20px",
            }}
          >
            <div
              style={{
                width: "92px",
                height: "92px",
                borderRadius: "24px",
                background: brandColor,
                color: "#fff",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                fontSize: "36px",
                fontWeight: 800,
                letterSpacing: "-0.03em",
                textTransform: "uppercase",
              }}
            >
              {platformMark}
            </div>
            <div
              style={{
                display: "flex",
                flexDirection: "column",
                alignItems: "flex-start",
              }}
            >
              <span
                style={{
                  fontSize: "72px",
                  fontWeight: 900,
                  color: "#f0f0f0",
                  letterSpacing: "-2px",
                  lineHeight: 1.05,
                }}
              >
                {platformName} API
              </span>
              <span
                style={{
                  marginTop: "12px",
                  fontSize: "24px",
                  fontWeight: 700,
                  color: brandColor,
                  letterSpacing: "0.08em",
                  textTransform: "uppercase",
                }}
              >
                Platform support
              </span>
            </div>
          </div>
          <div
            style={{
              fontSize: "72px",
              fontWeight: 900,
              color: "#10b981",
              letterSpacing: "-2px",
            }}
          >
            for Developers
          </div>
          <div
            style={{
              marginTop: "32px",
              fontSize: "22px",
              color: "#666",
            }}
          >
            Post programmatically · Free 100 posts/month · Setup in 5 min
          </div>
        </div>
      ),
      { ...size }
    );
  };
}
