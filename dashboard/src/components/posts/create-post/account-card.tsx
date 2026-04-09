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

  return (
    <button
      type="button"
      onClick={() => onToggle(account.id)}
      className={cn(
        "relative rounded-lg p-3 text-left transition-all duration-[180ms] [transition-timing-function:cubic-bezier(0.16,1,0.3,1)]",
        "bg-[#17171a] border",
        selected
          ? "border-[#10b981] shadow-[0_0_0_1px_#10b981,0_0_24px_-4px_rgba(16,185,129,0.15)]"
          : "border-[#22222a] hover:border-[#2e2e38] hover:bg-[#1c1c20]"
      )}
      style={
        selected
          ? { background: "linear-gradient(180deg, rgba(16,185,129,0.15) 0%, rgba(16,185,129,0.03) 100%)" }
          : undefined
      }
    >
      {/* Check mark */}
      <div
        className={cn(
          "absolute top-1.5 right-1.5 w-4 h-4 rounded-full bg-[#10b981] flex items-center justify-center",
          "transition-all duration-[160ms] [transition-timing-function:cubic-bezier(0.16,1,0.3,1)]",
          selected ? "opacity-100 scale-100" : "opacity-0 scale-[0.6]"
        )}
      >
        <Check className="w-[9px] h-[9px] text-[#0a0a0b]" strokeWidth={3} />
      </div>

      {/* Platform icon + label */}
      <div className="flex items-center gap-2 mb-1.5">
        <div
          className="w-6 h-6 rounded flex items-center justify-center"
          style={{ background: `${brandColor}20`, color: brandColor }}
        >
          <PlatformIcon platform={account.platform} size={11} />
        </div>
        <span className="text-[10px] uppercase tracking-wider text-[#55555c] font-mono">
          {label}
        </span>
      </div>

      {/* Handle */}
      <div className="text-[12px] truncate text-[#f4f4f5]">
        {account.account_name || account.external_user_email || account.platform}
      </div>
    </button>
  );
}
