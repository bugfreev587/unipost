"use client";

import {
  type PlatformLimit,
  countCharacters,
  getCountStatus,
  STATUS_COLORS,
} from "./platform-limits";

interface PlatformCardProps {
  platform: PlatformLimit;
  text: string;
}

export function PlatformCard({ platform, text }: PlatformCardProps) {
  const count = countCharacters(text, platform.countingMethod);
  const status = getCountStatus(count, platform.maxLength);
  const color = STATUS_COLORS[status];
  const pct = Math.min((count / platform.maxLength) * 100, 100);
  const remaining = platform.maxLength - count;

  const statusLabel =
    status === "over"
      ? `${Math.abs(remaining)} over`
      : status === "warning"
        ? `${remaining} left`
        : "OK";

  const statusIcon =
    status === "over" ? "\u274C" : status === "warning" ? "\u26A0\uFE0F" : "\u2705";

  return (
    <div
      style={{
        background: "#0f0f0f",
        border: "1px solid #1a1a1a",
        borderRadius: 12,
        padding: "20px 22px",
        display: "flex",
        flexDirection: "column",
        gap: 10,
        transition: "border-color 0.15s",
        borderColor: status === "over" ? "#ef444440" : "#1a1a1a",
      }}
    >
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ fontSize: 18 }}>{platform.icon}</span>
          <span style={{ fontSize: 14, fontWeight: 700, color: "#f0f0f0" }}>
            {platform.name}
          </span>
        </div>
        <span
          style={{
            fontSize: 12,
            fontFamily: "var(--mono, 'Fira Code', monospace)",
            color,
            fontWeight: 600,
          }}
        >
          {statusIcon} {statusLabel}
        </span>
      </div>

      {/* Count */}
      <div
        style={{
          fontSize: 13,
          fontFamily: "var(--mono, 'Fira Code', monospace)",
          color: "#999",
        }}
      >
        <span style={{ color }}>{count.toLocaleString()}</span>
        <span> / {platform.maxLength.toLocaleString()}</span>
      </div>

      {/* Progress bar */}
      <div
        style={{
          height: 6,
          background: "#1a1a1a",
          borderRadius: 3,
          overflow: "hidden",
        }}
      >
        <div
          style={{
            height: "100%",
            width: `${pct}%`,
            background: color,
            borderRadius: 3,
            transition: "width 0.1s, background 0.15s",
          }}
        />
      </div>
    </div>
  );
}
