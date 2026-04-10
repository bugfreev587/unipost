"use client";

import type { SocialAccount } from "@/lib/api";
import { AccountCard } from "./account-card";

interface AccountCardGridProps {
  accounts: SocialAccount[];
  selectedIds: Set<string>;
  onToggle: (id: string) => void;
  onToggleAll: () => void;
  profileName?: string;
}

export function AccountCardGrid({
  accounts,
  selectedIds,
  onToggle,
  onToggleAll,
  profileName,
}: AccountCardGridProps) {
  if (accounts.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-[#22222a] bg-[#0a0a0b]/40 py-10 px-6 text-center">
        <p className="text-sm text-[#8a8a93] mb-1">No accounts connected yet.</p>
        <p className="text-[13px] text-[#55555c]">
          Connect an account in Settings &rarr;
        </p>
      </div>
    );
  }

  return (
    <div>
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

      <div className="grid grid-cols-2 gap-2">
        {accounts.map((account) => (
          <AccountCard
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
