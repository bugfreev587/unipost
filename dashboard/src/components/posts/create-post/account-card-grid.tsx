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
      <div
        className="rounded-lg border border-dashed py-6 px-4 text-center"
        style={{ borderColor: "var(--dborder2)", background: "color-mix(in srgb, var(--surface2) 72%, transparent)" }}
      >
        <p className="mb-1 text-sm" style={{ color: "var(--dmuted)" }}>No accounts connected.</p>
        <p className="text-[12px]" style={{ color: "var(--dmuted2)" }}>
          Connect an account in Settings &rarr;
        </p>
      </div>
    );
  }

  return (
    <div>
      {onToggleAll && (
        <div className="flex items-center justify-between mb-3">
          <label className="text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
            Post to
          </label>
          <button
            type="button"
            className="text-[10.5px] font-mono tracking-[0.02em] transition-colors"
            style={{ color: "var(--dmuted)" }}
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
        <div className="mt-3 text-[11px] font-mono" style={{ color: "var(--dmuted2)" }}>
          connected to <span style={{ color: "var(--dmuted)" }}>{profileName}</span> profile
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
  const brandColor = PLATFORM_BRAND_COLORS[account.platform] || "var(--dmuted)";
  const label = PLATFORM_LABELS[account.platform] || account.platform;

  return (
    <button
      type="button"
      onClick={() => onToggle(account.id)}
      className="relative rounded-lg border p-2.5 text-left transition-all duration-[180ms] [transition-timing-function:cubic-bezier(0.16,1,0.3,1)]"
      style={
        selected
          ? {
              borderColor: "var(--daccent)",
              background: "linear-gradient(180deg, color-mix(in srgb, var(--primary) 12%, transparent) 0%, color-mix(in srgb, var(--surface) 94%, var(--primary)) 100%)",
              boxShadow: "0 0 0 1px color-mix(in srgb, var(--primary) 70%, transparent), 0 0 16px -4px color-mix(in srgb, var(--primary) 20%, transparent)",
            }
          : {
              borderColor: "var(--dborder)",
              background: "var(--surface2)",
            }
      }
      onMouseEnter={(e) => {
        if (!selected) {
          e.currentTarget.style.borderColor = "var(--dborder2)";
          e.currentTarget.style.background = "var(--surface3)";
        }
      }}
      onMouseLeave={(e) => {
        if (!selected) {
          e.currentTarget.style.borderColor = "var(--dborder)";
          e.currentTarget.style.background = "var(--surface2)";
        }
      }}
    >
      <div className="flex items-center gap-2">
        <div
          className="w-5 h-5 rounded flex items-center justify-center flex-shrink-0"
          style={{ background: `${brandColor}20`, color: brandColor }}
        >
          <PlatformIcon platform={account.platform} size={10} />
        </div>
        <div className="min-w-0">
          <div className="text-[10px] uppercase tracking-[0.1em] font-mono leading-none" style={{ color: "var(--dmuted2)" }}>
            {label}
          </div>
          <div className="mt-0.5 truncate text-[11.5px] leading-[1.3]" style={{ color: "var(--dtext)", fontWeight: 500 }}>
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
  duplicateIds?: Set<string>;
  onRemove: (id: string) => void;
}

export function PostToGrid({
  accounts,
  selectedIds,
  duplicateIds,
  onRemove,
}: PostToGridProps) {
  const selectedAccounts = accounts.filter((a) => selectedIds.has(a.id));

  if (selectedAccounts.length === 0) {
    return (
      <div
        className="rounded-lg border border-dashed py-3 px-4 text-center"
        style={{ borderColor: "var(--dborder2)", background: "color-mix(in srgb, var(--surface2) 72%, transparent)" }}
      >
        <p className="text-[12px]" style={{ color: "var(--dmuted2)" }}>
          Select accounts above to post to.
        </p>
      </div>
    );
  }

  return (
    <div>
      <div className="flex flex-wrap gap-1.5">
        {selectedAccounts.map((account) => (
          <PostToChip
            key={account.id}
            account={account}
            isDuplicate={duplicateIds?.has(account.id) ?? false}
            onRemove={onRemove}
          />
        ))}
      </div>
      {duplicateIds && duplicateIds.size > 0 && (
        <div className="mt-2 text-[10px] font-mono" style={{ color: "var(--warning)" }}>
          Duplicate accounts detected — only one post per platform account will be sent.
        </div>
      )}
    </div>
  );
}

function PostToChip({
  account,
  isDuplicate,
  onRemove,
}: {
  isDuplicate: boolean;
  account: SocialAccount;
  onRemove: (id: string) => void;
}) {
  const brandColor = PLATFORM_BRAND_COLORS[account.platform] || "var(--dmuted)";

  return (
    <div
      className="flex items-center gap-1.5 rounded-md border px-2 py-1 text-[11px]"
      style={{
        background: "var(--surface2)",
        borderColor: isDuplicate
          ? "color-mix(in srgb, var(--warning) 40%, transparent)"
          : "color-mix(in srgb, var(--primary) 30%, transparent)",
        opacity: isDuplicate ? 0.58 : 1,
      }}
    >
      <div
        className="w-3.5 h-3.5 rounded flex items-center justify-center flex-shrink-0"
        style={{ background: `${brandColor}20`, color: brandColor }}
      >
        <PlatformIcon platform={account.platform} size={8} />
      </div>
      <span
        className={`truncate max-w-[80px] ${isDuplicate ? "line-through" : ""}`}
        style={{ color: isDuplicate ? "var(--dmuted)" : "var(--dtext)" }}
      >
        {account.account_name || account.platform}
      </span>
      {isDuplicate && (
        <span className="text-[9px] flex-shrink-0" style={{ color: "var(--warning)" }}>DUP</span>
      )}
      <button
        type="button"
        onClick={() => onRemove(account.id)}
        className="transition-colors flex-shrink-0 ml-0.5"
        style={{ color: "var(--dmuted2)" }}
      >
        <X className="w-3 h-3" />
      </button>
    </div>
  );
}
