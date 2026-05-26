"use client";

import Link from "next/link";
import { BarChart3, Bot, Ruler, type LucideIcon } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";

export type ToolIconKey =
  | "agentpost"
  | "character-counter"
  | "tiktok"
  | "instagram"
  | "threads"
  | "pinterest";

const ICONS: Partial<Record<ToolIconKey, LucideIcon>> = {
  agentpost: Bot,
  "character-counter": Ruler,
};

export interface ToolCardData {
  icon: ToolIconKey;
  name: string;
  description: string;
  href: string;
  status: "live";
  badge?: string;
}

export function ToolCard({ tool }: { tool: ToolCardData }) {
  return (
    <Link
      href={tool.href}
      className="tl-card"
    >
      <span className="tl-card-icon">
        <ToolIcon icon={tool.icon} />
      </span>
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <span className="tl-card-name">{tool.name}</span>
        {tool.badge && (
          <span
            className="tl-card-badge"
          >
            {tool.badge}
          </span>
        )}
      </div>
      <span className="tl-card-desc">{tool.description}</span>
      <span className="tl-card-cta">Try Free &rarr;</span>
    </Link>
  );
}

function ToolIcon({ icon }: { icon: ToolIconKey }) {
  const Icon = ICONS[icon] || BarChart3;
  if (icon === "tiktok" || icon === "instagram" || icon === "threads" || icon === "pinterest") {
    return <PlatformIcon platform={icon} size={24} />;
  }
  return <Icon aria-hidden="true" />;
}
