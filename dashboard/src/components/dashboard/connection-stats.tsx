"use client";

import { PlatformIcon } from "@/components/platform-icons";
import type { SocialAccount, Profile } from "@/lib/api";

// ── Quickstart Stats (BYO accounts) ──────────────────────────────────

interface QuickstartStatsProps {
  accounts: SocialAccount[];
  profiles: Profile[];
}

export function QuickstartStats({ accounts, profiles }: QuickstartStatsProps) {
  const byo = accounts.filter((a) => a.connection_type === "byo");
  const active = byo.filter((a) => a.status === "active");
  const needsReconnect = byo.filter((a) => a.status === "reconnect_required");

  // By platform
  const byPlatform = new Map<string, number>();
  for (const a of byo) {
    byPlatform.set(a.platform, (byPlatform.get(a.platform) || 0) + 1);
  }

  // By profile
  const byProfile = new Map<string, number>();
  for (const a of byo) {
    byProfile.set(a.profile_id, (byProfile.get(a.profile_id) || 0) + 1);
  }

  return (
    <div style={{
      display: "grid",
      gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))",
      gap: 12,
      padding: 18,
      background: "var(--surface1)",
      border: "1px solid var(--dborder)",
      borderRadius: 12,
      marginBottom: 24,
      boxShadow: "0 1px 2px color-mix(in srgb, var(--shadow-color) 58%, transparent)",
    }}>
      <StatCard label="Total Accounts" value={byo.length} />
      <StatCard
        label="Health"
        custom={
          <div style={{ display: "flex", gap: 12, marginTop: 4 }}>
            <span style={{ display: "flex", alignItems: "center", gap: 5, fontSize: 14 }}>
              <span style={{ width: 7, height: 7, borderRadius: "50%", background: "#10b981" }} />
              <span style={{ color: "var(--dtext)", fontWeight: 700 }}>{active.length}</span>
              <span style={{ color: "var(--dmuted)", fontSize: 12, fontWeight: 500 }}>active</span>
            </span>
            {needsReconnect.length > 0 && (
              <span style={{ display: "flex", alignItems: "center", gap: 5, fontSize: 14 }}>
                <span style={{ width: 7, height: 7, borderRadius: "50%", background: "#f59e0b" }} />
                <span style={{ color: "#f59e0b", fontWeight: 600 }}>{needsReconnect.length}</span>
                <span style={{ color: "var(--dmuted)", fontSize: 12, fontWeight: 500 }}>reconnect</span>
              </span>
            )}
          </div>
        }
      />
      <StatCard
        label="By Platform"
        custom={
          <div style={{ display: "grid", gridTemplateColumns: "repeat(3, auto)", gap: "8px 20px", marginTop: 6 }}>
            {[...byPlatform.entries()].sort((a, b) => b[1] - a[1]).map(([platform, count]) => (
              <span key={platform} style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 13 }}>
                <PlatformIcon platform={platform} size={14} />
                <span style={{ color: "var(--dtext)", fontWeight: 700 }}>{count}</span>
              </span>
            ))}
          </div>
        }
      />
      {profiles.length > 1 && (
        <StatCard
          label="By Profile"
          custom={
            <div style={{ display: "flex", flexDirection: "column", gap: 3, marginTop: 4 }}>
              {profiles.map((p) => (
                <div key={p.id} style={{ display: "flex", justifyContent: "space-between", fontSize: 13 }}>
                  <span style={{ color: "var(--dmuted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", maxWidth: 100 }}>{p.name}</span>
                  <span style={{ color: "var(--dtext)", fontWeight: 700, fontFamily: "var(--font-geist-mono), monospace" }}>{byProfile.get(p.id) || 0}</span>
                </div>
              ))}
            </div>
          }
        />
      )}
    </div>
  );
}

// ── Managed Users Stats ──────────────────────────────────────────────

interface ManagedUsersStatsProps {
  users: Array<{ account_count: number; platforms?: string[] }>;
  totalAccounts?: number;
}

export function ManagedUsersStats({ users, totalAccounts }: ManagedUsersStatsProps) {
  const total = totalAccounts ?? users.reduce((sum, u) => sum + u.account_count, 0);

  return (
    <div style={{
      display: "grid",
      gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))",
      gap: 12,
      padding: 18,
      background: "var(--surface1)",
      border: "1px solid var(--dborder)",
      borderRadius: 12,
      marginBottom: 24,
      boxShadow: "0 1px 2px color-mix(in srgb, var(--shadow-color) 58%, transparent)",
    }}>
      <StatCard label="Managed Users" value={users.length} />
      <StatCard label="Total Accounts" value={total} />
    </div>
  );
}

// ── White-label Stats ────────────────────────────────────────────────

interface WhiteLabelStatsProps {
  configuredCount: number;
  totalPlatforms: number;
}

export function WhiteLabelStats({ configuredCount, totalPlatforms }: WhiteLabelStatsProps) {
  return (
    <div style={{
      display: "grid",
      gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))",
      gap: 12,
      padding: 18,
      background: "var(--surface1)",
      border: "1px solid var(--dborder)",
      borderRadius: 12,
      marginBottom: 24,
      boxShadow: "0 1px 2px color-mix(in srgb, var(--shadow-color) 58%, transparent)",
    }}>
      <StatCard
        label="Platform Credentials"
        custom={
          <div style={{ marginTop: 4 }}>
            <span style={{ fontSize: 20, fontWeight: 700, color: "var(--dtext)" }}>{configuredCount}</span>
            <span style={{ fontSize: 13, color: "var(--dmuted)", marginLeft: 4 }}>/ {totalPlatforms} configured</span>
          </div>
        }
      />
    </div>
  );
}

// ── Shared StatCard ──────────────────────────────────────────────────

function StatCard({ label, value, custom }: { label: string; value?: number; custom?: React.ReactNode }) {
  return (
    <div style={{ padding: "10px 14px" }}>
      <div style={{ fontSize: 11, fontWeight: 700, textTransform: "uppercase", letterSpacing: "0.08em", color: "color-mix(in srgb, var(--dmuted) 82%, var(--dtext))", marginBottom: 6 }}>
        {label}
      </div>
      {custom ?? (
        <div style={{ fontSize: 30, lineHeight: "34px", fontWeight: 700, color: "var(--dtext)", letterSpacing: "-0.02em", marginTop: 2 }}>
          {value ?? 0}
        </div>
      )}
    </div>
  );
}
