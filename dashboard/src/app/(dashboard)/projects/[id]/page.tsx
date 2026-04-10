"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import Link from "next/link";
import {
  getProfile,
  getBilling,
  listSocialAccounts,
  listSocialPosts,
  listApiKeys,
  type Profile,
  type BillingInfo,
  type SocialAccount,
  type SocialPost,
  type ApiKey,
} from "@/lib/api";
import { Key, Users, Send, BarChart3, CreditCard, Settings, ChevronRight } from "lucide-react";

const NAV_CARDS = [
  { href: "/api-keys", label: "API Keys", desc: "Manage access tokens", icon: Key },
  { href: "/accounts", label: "Accounts", desc: "Connected platforms", icon: Users },
  { href: "/posts", label: "Posts", desc: "Send & track posts", icon: Send },
  { href: "/analytics", label: "Analytics", desc: "Post performance metrics", icon: BarChart3 },
  { href: "/billing", label: "Billing", desc: "Plan & usage", icon: CreditCard },
  { href: "/settings", label: "Settings", desc: "Configuration", icon: Settings },
];

export default function ProfileOverviewPage() {
  const { id } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [profile, setProfile] = useState<Profile | null>(null);
  const [billing, setBilling] = useState<BillingInfo | null>(null);
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [posts, setPosts] = useState<SocialPost[]>([]);
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function load() {
      try {
        const token = await getToken();
        if (!token) return;
        const [profileRes, billingRes, accountsRes, postsRes, keysRes] =
          await Promise.all([
            getProfile(token, id),
            getBilling(token, id).catch(() => null),
            listSocialAccounts(token, id).catch(() => ({ data: [] as SocialAccount[] })),
            listSocialPosts(token, id).catch(() => ({ data: [] as SocialPost[] })),
            listApiKeys(token, id).catch(() => ({ data: [] as ApiKey[] })),
          ]);
        setProfile(profileRes.data);
        if (billingRes) setBilling(billingRes.data);
        setAccounts(accountsRes.data);
        setPosts(postsRes.data);
        setKeys(keysRes.data);
      } catch (err) {
        console.error("Failed to load profile:", err);
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [getToken, id]);

  if (loading) {
    return <div style={{ color: "var(--dmuted)" }}>Loading...</div>;
  }

  if (!profile) {
    return <div style={{ color: "var(--danger)" }}>Profile not found.</div>;
  }

  const usagePct = billing ? Math.min(billing.percentage, 100) : 0;
  const barClass = usagePct >= 100 ? "bar-red" : usagePct >= 80 ? "bar-amber" : "bar-green";

  return (
    <>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>
              {profile.name}
            </div>
          </div>
          <div style={{ fontFamily: "var(--font-geist-mono), monospace", fontSize: 11, color: "var(--dmuted2)", marginTop: 4 }}>
            {profile.id}
          </div>
        </div>
        {billing && (
          <span className={`dbadge ${billing.plan === "free" ? "dbadge-amber" : "dbadge-green"}`}>
            {billing.plan_name}
          </span>
        )}
      </div>

      {/* Stats */}
      <div className="stat-grid">
        <div className="stat-card">
          <div style={{ fontSize: 11, color: "var(--dmuted)", textTransform: "uppercase", letterSpacing: "0.06em", fontWeight: 600, marginBottom: 8 }}>
            Posts This Month
          </div>
          <div style={{ fontFamily: "var(--font-geist-mono), monospace", fontSize: 22, fontWeight: 600, color: "var(--dtext)", letterSpacing: -0.5 }}>
            {billing?.usage ?? 0}
          </div>
          {billing && (
            <>
              <div style={{ margin: "8px 0 4px" }}>
                <div className="usage-bar-track">
                  <div className={`usage-bar-fill ${barClass}`} style={{ width: `${usagePct}%` }} />
                </div>
              </div>
              <div style={{ fontSize: 11.5, color: usagePct >= 80 ? "var(--warning)" : "var(--dmuted)" }}>
                {billing.usage} / {billing.limit} &middot; {Math.round(billing.percentage)}%
              </div>
            </>
          )}
        </div>
        <div className="stat-card">
          <div style={{ fontSize: 11, color: "var(--dmuted)", textTransform: "uppercase", letterSpacing: "0.06em", fontWeight: 600, marginBottom: 8 }}>
            Connected Accounts
          </div>
          <div style={{ fontFamily: "var(--font-geist-mono), monospace", fontSize: 22, fontWeight: 600, color: "var(--dtext)", letterSpacing: -0.5 }}>
            {accounts.length}
          </div>
          <div style={{ fontSize: 11.5, color: "var(--dmuted)", marginTop: 4 }}>Unlimited on all plans</div>
        </div>
        <div className="stat-card">
          <div style={{ fontSize: 11, color: "var(--dmuted)", textTransform: "uppercase", letterSpacing: "0.06em", fontWeight: 600, marginBottom: 8 }}>
            API Keys
          </div>
          <div style={{ fontFamily: "var(--font-geist-mono), monospace", fontSize: 22, fontWeight: 600, color: "var(--dtext)", letterSpacing: -0.5 }}>
            {keys.length}
          </div>
          <div style={{ fontSize: 11.5, color: "var(--dmuted)", marginTop: 4 }}>{posts.length} total posts</div>
        </div>
      </div>

      {/* Quick nav */}
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
        {NAV_CARDS.map((item) => {
          const Icon = item.icon;
          return (
            <Link
              key={item.href}
              href={`/projects/${id}${item.href}`}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 12,
                padding: "14px 16px",
                background: "var(--surface)",
                border: "1px solid var(--dborder)",
                borderRadius: 8,
                textDecoration: "none",
                color: "inherit",
                transition: "border-color 0.15s, background 0.15s",
              }}
              onMouseEnter={(e) => { e.currentTarget.style.borderColor = "var(--dborder2)"; e.currentTarget.style.background = "var(--surface2)"; }}
              onMouseLeave={(e) => { e.currentTarget.style.borderColor = "var(--dborder)"; e.currentTarget.style.background = "var(--surface)"; }}
            >
              <Icon style={{ width: 16, height: 16, color: "var(--dmuted2)" }} strokeWidth={1.75} />
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: 13, fontWeight: 500, color: "var(--dtext)" }}>{item.label}</div>
                <div style={{ fontSize: 11.5, color: "var(--dmuted2)" }}>{item.desc}</div>
              </div>
              <ChevronRight style={{ width: 14, height: 14, color: "var(--dmuted2)" }} />
            </Link>
          );
        })}
      </div>
    </>
  );
}
