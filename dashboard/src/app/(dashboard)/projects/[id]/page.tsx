"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import {
  getProject,
  getBilling,
  listSocialAccounts,
  listSocialPosts,
  listApiKeys,
  type Project,
  type BillingInfo,
  type SocialAccount,
  type SocialPost,
  type ApiKey,
} from "@/lib/api";
import {
  Key,
  Users,
  Send,
  CreditCard,
  Settings,
  ArrowUpRight,
  Activity,
} from "lucide-react";

const NAV_CARDS = [
  { href: "/api-keys", label: "API Keys", desc: "Manage access tokens", icon: Key },
  { href: "/accounts", label: "Accounts", desc: "Connected platforms", icon: Users },
  { href: "/posts", label: "Posts", desc: "Send & track posts", icon: Send },
  { href: "/billing", label: "Billing", desc: "Plan & usage", icon: CreditCard },
  { href: "/settings", label: "Settings", desc: "Configuration", icon: Settings },
];

export default function ProjectOverviewPage() {
  const { id } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [project, setProject] = useState<Project | null>(null);
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
        const [projectRes, billingRes, accountsRes, postsRes, keysRes] =
          await Promise.all([
            getProject(token, id),
            getBilling(token, id).catch(() => null),
            listSocialAccounts(token, id).catch(() => ({ data: [] as SocialAccount[] })),
            listSocialPosts(token, id).catch(() => ({ data: [] as SocialPost[] })),
            listApiKeys(token, id).catch(() => ({ data: [] as ApiKey[] })),
          ]);
        setProject(projectRes.data);
        if (billingRes) setBilling(billingRes.data);
        setAccounts(accountsRes.data);
        setPosts(postsRes.data);
        setKeys(keysRes.data);
      } catch (err) {
        console.error("Failed to load project:", err);
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [getToken, id]);

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="h-5 w-40 bg-[#111111] rounded animate-pulse" />
        <div className="grid grid-cols-4 gap-3">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-[80px] rounded-lg bg-[#111111] border border-[#1e1e1e] animate-pulse" />
          ))}
        </div>
      </div>
    );
  }

  if (!project) {
    return <p className="text-[13px] text-destructive">Project not found.</p>;
  }

  const usagePct = billing ? Math.min(billing.percentage, 100) : 0;
  const usageColor =
    usagePct >= 100
      ? "bg-destructive"
      : usagePct >= 80
        ? "bg-amber-status"
        : "bg-emerald";

  const stats = [
    { label: "API Keys", value: keys.length },
    { label: "Accounts", value: accounts.length },
    { label: "Posts", value: posts.length },
    {
      label: "Usage",
      value: billing ? `${billing.usage}/${billing.limit}` : "—",
    },
  ];

  return (
    <div>
      {/* Header */}
      <div className="flex items-start justify-between mb-6 animate-enter">
        <div>
          <div className="flex items-center gap-2.5">
            <h1 className="text-[18px] font-semibold text-[#e5e5e5] tracking-tight">
              {project.name}
            </h1>
            <Badge variant="secondary" className="text-[10px] bg-[#1a1a1a] text-[#737373] border-0">
              {project.mode}
            </Badge>
          </div>
          <p className="mono text-[11px] text-[#3a3a3a] mt-1">{project.id}</p>
        </div>
        {billing && (
          <Badge
            variant={billing.plan === "free" ? "secondary" : "default"}
            className={`text-[10px] ${billing.plan === "free" ? "bg-[#1a1a1a] text-[#737373] border-0" : "bg-emerald/10 text-emerald border-emerald/20"}`}
          >
            {billing.plan_name}
          </Badge>
        )}
      </div>

      {/* Stats row */}
      <div
        className="grid grid-cols-4 gap-3 mb-6 animate-enter"
        style={{ animationDelay: "50ms" }}
      >
        {stats.map((stat) => (
          <div
            key={stat.label}
            className="rounded-lg bg-[#111111] border border-[#1e1e1e] px-4 py-3"
          >
            <p className="text-[10px] font-medium uppercase tracking-[0.1em] text-[#525252] mb-1.5">
              {stat.label}
            </p>
            <p className="mono text-[18px] font-semibold text-[#e5e5e5] tracking-tight">
              {stat.value}
            </p>
          </div>
        ))}
      </div>

      {/* Usage bar */}
      {billing && (
        <div
          className="rounded-lg bg-[#111111] border border-[#1e1e1e] p-4 mb-6 animate-enter"
          style={{ animationDelay: "100ms" }}
        >
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <Activity className="w-3.5 h-3.5 text-[#525252]" />
              <span className="text-[12px] font-medium text-[#a3a3a3]">
                Monthly Usage
              </span>
            </div>
            <span className="mono text-[11px] text-[#525252]">
              {billing.usage} / {billing.limit} posts &middot;{" "}
              {Math.round(billing.percentage)}%
            </span>
          </div>
          <div className="w-full bg-[#1a1a1a] rounded-full h-1.5">
            <div
              className={`h-1.5 rounded-full transition-all duration-700 ease-out ${usageColor}`}
              style={{ width: `${usagePct}%` }}
            />
          </div>
          {billing.warning && (
            <p
              className={`text-[11px] mt-2 ${
                billing.warning === "over_limit" ? "text-destructive" : "text-amber-status"
              }`}
            >
              {billing.warning === "over_limit"
                ? "Monthly limit exceeded. Upgrade to continue."
                : "Approaching monthly limit. Consider upgrading."}
            </p>
          )}
        </div>
      )}

      {/* Quick nav */}
      <div className="animate-enter" style={{ animationDelay: "150ms" }}>
        <p className="text-[10px] font-medium uppercase tracking-[0.1em] text-[#525252] mb-3 px-0.5">
          Manage
        </p>
        <div className="grid grid-cols-2 gap-2">
          {NAV_CARDS.map((item) => {
            const Icon = item.icon;
            return (
              <Link key={item.href} href={`/projects/${id}${item.href}`}>
                <div className="group flex items-center gap-3.5 px-4 py-3 rounded-lg bg-[#111111] border border-[#1e1e1e] hover:border-[#2a2a2a] transition-colors">
                  <Icon
                    className="w-4 h-4 text-[#3a3a3a] group-hover:text-[#525252] transition-colors shrink-0"
                    strokeWidth={1.75}
                  />
                  <div className="flex-1 min-w-0">
                    <p className="text-[13px] font-medium text-[#d4d4d4]">
                      {item.label}
                    </p>
                    <p className="text-[11px] text-[#3a3a3a]">{item.desc}</p>
                  </div>
                  <ArrowUpRight className="w-3.5 h-3.5 text-[#1e1e1e] group-hover:text-[#3a3a3a] transition-colors shrink-0" />
                </div>
              </Link>
            );
          })}
        </div>
      </div>
    </div>
  );
}
