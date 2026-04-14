import Image from "next/image";

function joinClassNames(...parts: Array<string | undefined>) {
  return parts.filter(Boolean).join(" ");
}

export function UniPostMark({
  size = 28,
  className,
}: {
  size?: number;
  className?: string;
}) {
  return (
    <span
      className={joinClassNames("unipost-mark", className)}
      style={{
        position: "relative",
        display: "inline-flex",
        width: size,
        height: size,
        flexShrink: 0,
      }}
      aria-hidden="true"
    >
      <Image
        src="/brand/unipost-icon-dark.png"
        alt=""
        width={size}
        height={size}
        style={{ width: "100%", height: "100%", objectFit: "contain", borderRadius: Math.max(6, Math.round(size * 0.22)), display: "block" }}
        className="unipost-mark-dark"
        priority={size >= 64}
      />
      <Image
        src="/brand/unipost-icon-light.png"
        alt=""
        width={size}
        height={size}
        style={{
          width: "100%",
          height: "100%",
          objectFit: "contain",
          borderRadius: Math.max(6, Math.round(size * 0.22)),
          display: "none",
          position: "absolute",
          inset: 0,
        }}
        className="unipost-mark-light"
      />
      <style>{`
        html.light .unipost-mark-dark{display:none !important}
        html.light .unipost-mark-light{display:block !important}
      `}</style>
    </span>
  );
}

export function UniPostLogo({
  markSize = 28,
  className,
  wordmarkClassName,
  wordmarkColor = "currentColor",
  wordmark = "UniPost",
}: {
  markSize?: number;
  className?: string;
  wordmarkClassName?: string;
  wordmarkColor?: string;
  wordmark?: string;
}) {
  return (
    <span
      className={joinClassNames("unipost-logo", className)}
      style={{ display: "inline-flex", alignItems: "center", gap: 10, minWidth: 0 }}
    >
      <UniPostMark size={markSize} />
      <span
        className={joinClassNames("unipost-wordmark", wordmarkClassName)}
        style={{
          color: wordmarkColor,
          fontWeight: 700,
          fontSize: 16,
          lineHeight: 1,
          letterSpacing: "-0.045em",
          whiteSpace: "nowrap",
        }}
      >
        {wordmark}
      </span>
    </span>
  );
}
