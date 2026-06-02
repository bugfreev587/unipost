import type { CSSProperties } from "react";
import { Link2, Video } from "lucide-react";
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
      <span style={style} aria-hidden="true">
        <Video width={size} height={size} strokeWidth={2} />
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
