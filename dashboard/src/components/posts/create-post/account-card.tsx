"use client";

import { Check } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";
import { PLATFORM_LABELS, PLATFORM_BRAND_COLORS } from "./use-create-post-form";
import type { SocialAccount } from "@/lib/api";
import { cn } from "@/lib/utils";

interface AccountCardProps {
  account: SocialAccount;
  selected: boolean;
  onToggle: (id: string) => void;
}

export function AccountCard({ account, selected, onToggle }: AccountCardProps) {
  const brandColor = PLATFORM_BRAND_COLORS[account.platform] || "#888";
  const label = PLATFORM_LABELS[account.platform] || account.platform;
  const selectedBackground = "linear-gradient(180deg, color-mix(in srgb, var(--primary) 14%, var(--surface-raised)) 0%, color-mix(in srgb, var(--primary) 4%, var(--surface-raised)) 100%)";

  return (
    <button
      type="button"
      onClick={() => onToggle(account.id)}
      className={cn(
        "relative rounded-lg p-3 text-left transition-all duration-[180ms] [transition-timing-function:cubic-bezier(0.16,1,0.3,1)]",
        "border",
        selected
          ? "shadow-[0_0_0_1px_color-mix(in_srgb,var(--primary)_70%,transparent),0_0_24px_-4px_color-mix(in_srgb,var(--primary)_18%,transparent)]"
          : "hover:shadow-[0_0_0_1px_color-mix(in_srgb,var(--border-strong)_90%,transparent)]"
      )}
      style={
        selected
          ? { background: selectedBackground, borderColor: "var(--primary)" }
          : { background: "var(--surface-raised)", borderColor: "var(--border-soft)" }
      }
    >
      {/* Check mark */}
      <div
        className={cn(
          "absolute top-1.5 right-1.5 flex h-4 w-4 items-center justify-center rounded-full",
          "transition-all duration-[160ms] [transition-timing-function:cubic-bezier(0.16,1,0.3,1)]",
          selected ? "opacity-100 scale-100" : "opacity-0 scale-[0.6]"
        )}
        style={{ background: "var(--primary)" }}
      >
        <Check className="h-[9px] w-[9px]" style={{ color: "var(--primary-foreground)" }} strokeWidth={3} />
      </div>

      {/* Platform icon + label */}
      <div className="flex items-center gap-2 mb-1.5">
        <div
          className="w-6 h-6 rounded flex items-center justify-center"
          style={{ background: `${brandColor}20`, color: brandColor }}
        >
          <PlatformIcon platform={account.platform} size={11} />
        </div>
        <span className="font-mono text-[10px] uppercase tracking-wider" style={{ color: "var(--dmuted2)" }}>
          {label}
        </span>
      </div>

      {/* Handle */}
      <div className="truncate text-[12px]" style={{ color: "var(--dtext)" }}>
        {account.account_name || account.external_user_email || account.platform}
      </div>
    </button>
  );
}
