"use client";

import { X } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";
import { PLATFORM_LABELS, PLATFORM_BRAND_COLORS } from "./use-create-post-form";
import type { SocialAccount } from "@/lib/api";

// --- Connected Accounts section (clickable cards to select/deselect) ---

interface ConnectedAccountsGridProps {
  accounts: SocialAccount[];
  selectedIds: Set<string>;
  onToggle: (id: string) => void;
  onToggleAll?: () => void;
  profileName?: string;
}

// Alias for backward compatibility with the drawer
export { ConnectedAccountsGrid as AccountCardGrid };

export function ConnectedAccountsGrid({
  accounts,
  selectedIds,
  onToggle,
  onToggleAll,
  profileName,
}: ConnectedAccountsGridProps) {
  if (accounts.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-[#22222a] bg-[#0a0a0b]/40 py-6 px-4 text-center">
        <p className="text-sm text-[#8a8a93] mb-1">No accounts connected.</p>
        <p className="text-[12px] text-[#55555c]">
          Connect an account in Settings &rarr;
        </p>
      </div>
    );
  }

  return (
    <div>
      {onToggleAll && (
        <div className="flex items-center justify-between mb-3">
          <label className="text-xs uppercase tracking-wider text-[#55555c] font-medium">
            Post to
          </label>
          <button
            type="button"
            className="text-[11px] text-[#8a8a93] hover:text-[#f4f4f5] font-mono transition-colors"
            onClick={onToggleAll}
          >
            toggle all
          </button>
        </div>
      )}
      <div className="grid grid-cols-2 gap-2">
        {accounts.map((account) => (
          <AccountCardSmall
            key={account.id}
            account={account}
            selected={selectedIds.has(account.id)}
            onToggle={onToggle}
          />
        ))}
      </div>
      {profileName && (
        <div className="mt-3 text-[11px] text-[#55555c] font-mono">
          connected to <span className="text-[#8a8a93]">{profileName}</span> profile
        </div>
      )}
    </div>
  );
}

// Compact account card for Connected Accounts grid
function AccountCardSmall({
  account,
  selected,
  onToggle,
}: {
  account: SocialAccount;
  selected: boolean;
  onToggle: (id: string) => void;
}) {
  const brandColor = PLATFORM_BRAND_COLORS[account.platform] || "#888";
  const label = PLATFORM_LABELS[account.platform] || account.platform;

  return (
    <button
      type="button"
      onClick={() => onToggle(account.id)}
      className={`relative rounded-lg p-2.5 text-left transition-all duration-[180ms] [transition-timing-function:cubic-bezier(0.16,1,0.3,1)] bg-[#17171a] border ${
        selected
          ? "border-[#10b981] shadow-[0_0_0_1px_#10b981,0_0_16px_-4px_rgba(16,185,129,0.15)]"
          : "border-[#22222a] hover:border-[#2e2e38] hover:bg-[#1c1c20]"
      }`}
      style={
        selected
          ? { background: "linear-gradient(180deg, rgba(16,185,129,0.12) 0%, rgba(16,185,129,0.02) 100%)" }
          : undefined
      }
    >
      <div className="flex items-center gap-2">
        <div
          className="w-5 h-5 rounded flex items-center justify-center flex-shrink-0"
          style={{ background: `${brandColor}20`, color: brandColor }}
        >
          <PlatformIcon platform={account.platform} size={10} />
        </div>
        <div className="min-w-0">
          <div className="text-[10px] uppercase tracking-wider text-[#55555c] font-mono leading-none">
            {label}
          </div>
          <div className="text-[11px] truncate text-[#f4f4f5] mt-0.5">
            {account.account_name || account.external_user_email || account.platform}
          </div>
        </div>
      </div>
    </button>
  );
}

// --- Post To section (selected account chips with X to unselect) ---

interface PostToGridProps {
  accounts: SocialAccount[];
  selectedIds: Set<string>;
  onRemove: (id: string) => void;
}

export function PostToGrid({
  accounts,
  selectedIds,
  onRemove,
}: PostToGridProps) {
  const selectedAccounts = accounts.filter((a) => selectedIds.has(a.id));

  if (selectedAccounts.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-[#22222a] bg-[#0a0a0b]/40 py-3 px-4 text-center">
        <p className="text-[12px] text-[#55555c]">
          Select accounts above to post to.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-wrap gap-1.5">
      {selectedAccounts.map((account) => (
        <PostToChip key={account.id} account={account} onRemove={onRemove} />
      ))}
    </div>
  );
}

function PostToChip({
  account,
  onRemove,
}: {
  account: SocialAccount;
  onRemove: (id: string) => void;
}) {
  const brandColor = PLATFORM_BRAND_COLORS[account.platform] || "#888";

  return (
    <div className="flex items-center gap-1.5 rounded-md bg-[#17171a] border border-[#10b981]/30 px-2 py-1 text-[11px]">
      <div
        className="w-3.5 h-3.5 rounded flex items-center justify-center flex-shrink-0"
        style={{ background: `${brandColor}20`, color: brandColor }}
      >
        <PlatformIcon platform={account.platform} size={8} />
      </div>
      <span className="text-[#f4f4f5] truncate max-w-[80px]">
        {account.account_name || account.platform}
      </span>
      <button
        type="button"
        onClick={() => onRemove(account.id)}
        className="text-[#55555c] hover:text-[#f4f4f5] transition-colors flex-shrink-0 ml-0.5"
      >
        <X className="w-3 h-3" />
      </button>
    </div>
  );
}
