import type { CSSProperties } from "react";
import { Link2 } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";

export function AccountDestinationIcon({ platform, size = 14 }: { platform: string; size?: number }) {
  if (platform === "youtube") {
    const style: CSSProperties = {
      display: "inline-flex",
      alignItems: "center",
      justifyContent: "center",
      width: size,
      height: size,
      flexShrink: 0,
      color: "currentColor",
    };

    return (
      <span style={style} aria-hidden="true" data-youtube-destination-icon>
        <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
          <rect
            x="3"
            y="5"
            width="18"
            height="14"
            rx="4"
            stroke="currentColor"
            strokeWidth="1.8"
          />
          <path
            d="M10 9.25L15 12L10 14.75Z"
            stroke="currentColor"
            strokeWidth="1.6"
            strokeLinejoin="round"
          />
        </svg>
      </span>
    );
  }

  if (!platform) {
    const style: CSSProperties = {
      display: "inline-flex",
      alignItems: "center",
      justifyContent: "center",
      width: size,
      height: size,
      flexShrink: 0,
      color: "currentColor",
    };

    return (
      <span style={style} aria-hidden="true">
        <Link2 width={size} height={size} strokeWidth={2} />
      </span>
    );
  }

  return <PlatformIcon platform={platform} size={size} />;
}
