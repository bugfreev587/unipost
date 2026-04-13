import { useId } from "react";

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
  const gradientId = useId().replace(/:/g, "");

  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 128 128"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      aria-hidden="true"
    >
      <defs>
        <linearGradient id={gradientId} x1="16" y1="112" x2="112" y2="16" gradientUnits="userSpaceOnUse">
          <stop stopColor="#48A4FF" />
          <stop offset="1" stopColor="#A020F0" />
        </linearGradient>
      </defs>
      <rect x="16" y="48" width="56" height="56" rx="2" fill={`url(#${gradientId})`} />
      <rect x="44" y="28" width="56" height="56" rx="2" stroke={`url(#${gradientId})`} strokeWidth="8" />
      <rect x="52" y="56" width="30" height="30" rx="1.5" fill={`url(#${gradientId})`} />
      <rect x="62" y="66" width="10" height="10" rx="1" fill="#FFFFFF" />
    </svg>
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
