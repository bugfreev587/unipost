"use client";

import Link from "next/link";

export interface ToolCardData {
  icon: string;
  name: string;
  description: string;
  href: string;
  status: "live" | "coming_soon";
  badge?: string;
}

export function ToolCard({ tool }: { tool: ToolCardData }) {
  const isLive = tool.status === "live";
  const Tag = isLive ? Link : "div";

  return (
    <Tag
      href={tool.href}
      className="tl-card"
      style={!isLive ? { opacity: 0.5, cursor: "default" } : undefined}
    >
      <span className="tl-card-icon">{tool.icon}</span>
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <span className="tl-card-name">{tool.name}</span>
        {tool.badge && (
          <span
            className="tl-card-badge"
            style={
              tool.status === "coming_soon"
                ? { background: "#ffffff08", color: "#666", border: "1px solid #333" }
                : undefined
            }
          >
            {tool.badge}
          </span>
        )}
      </div>
      <span className="tl-card-desc">{tool.description}</span>
      {isLive && <span className="tl-card-cta">Try Free &rarr;</span>}
      {!isLive && <span className="tl-card-soon">Coming Soon</span>}
    </Tag>
  );
}
