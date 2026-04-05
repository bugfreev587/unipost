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
  ShieldCheck,
  CreditCard,
  Settings,
  ArrowRight,
  Activity,
} from "lucide-react";

const QUICK_NAV = [
  { href: "/posts", label: "Posts", desc: "Send and manage posts", icon: Send },
  { href: "/accounts", label: "Accounts", desc: "Connected platforms", icon: Users },
  { href: "/api-keys", label: "API Keys", desc: "Manage API access", icon: Key },
  { href: "/credentials", label: "Credentials", desc: "Native mode (BYOC)", icon: ShieldCheck },
  { href: "/billing", label: "Billing", desc: "Plan and usage", icon: CreditCard },
  { href: "/settings", label: "Settings", desc: "Project config", icon: Settings },
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
            listSocialAccounts(token, id).catch(() => ({ data: [] })),
            listSocialPosts(token, id).catch(() => ({ data: [] })),
            listApiKeys(token, id).catch(() => ({ data: [] })),
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
        <div className="h-6 w-48 bg-muted rounded animate-pulse" />
        <div className="grid grid-cols-4 gap-3">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-20 rounded-lg bg-muted/50 animate-pulse" />
          ))}
        </div>
      </div>
    );
  }

  if (!project) {
    return (
      <div className="text-destructive text-sm">Project not found</div>
    );
  }

  const stats = [
    { label: "Posts", value: posts.length, href: `/projects/${id}/posts` },
    { label: "Accounts", value: accounts.length, href: `/projects/${id}/accounts` },
    { label: "API Keys", value: keys.length, href: `/projects/${id}/api-keys` },
    {
      label: "Usage",
      value: billing ? `${billing.usage}/${billing.limit}` : "---",
      href: `/projects/${id}/billing`,
    },
  ];

  const usagePct = billing ? Math.min(billing.percentage, 100) : 0;

  return (
    <div>
      {/* Header */}
      <div className="flex items-start justify-between mb-8 animate-fade-up">
        <div>
          <div className="flex items-center gap-3 mb-1">
            <h1 className="text-xl font-semibold tracking-tight">
              {project.name}
            </h1>
            <Badge variant="secondary" className="text-[11px] font-normal">
              {project.mode}
            </Badge>
          </div>
          <p className="mono-data text-muted-foreground text-[11px]">
            {project.id}
          </p>
        </div>
        {billing && (
          <div className="text-right">
            <p className="text-[11px] text-muted-foreground mb-1">
              {billing.plan_name} plan
            </p>
            <Badge
              variant={billing.plan === "free" ? "secondary" : "default"}
              className="text-[11px]"
            >
              {billing.plan_name}
            </Badge>
          </div>
        )}
      </div>

      {/* Stats row */}
      <div
        className="grid grid-cols-4 gap-3 mb-8 animate-fade-up"
        style={{ animationDelay: "60ms" }}
      >
        {stats.map((stat) => (
          <Link key={stat.label} href={stat.href}>
            <div className="card-hover rounded-lg border border-border bg-card px-4 py-3 hover:border-foreground/15">
              <p className="text-[11px] text-muted-foreground font-medium uppercase tracking-wider mb-1">
                {stat.label}
              </p>
              <p className="text-lg font-semibold tracking-tight mono-data">
                {stat.value}
              </p>
            </div>
          </Link>
        ))}
      </div>

      {/* Usage bar */}
      {billing && (
        <div
          className="rounded-lg border border-border bg-card p-4 mb-8 animate-fade-up"
          style={{ animationDelay: "120ms" }}
        >
          <div className="flex items-center justify-between mb-2.5">
            <div className="flex items-center gap-2">
              <Activity className="w-3.5 h-3.5 text-muted-foreground" />
              <span className="text-[13px] font-medium">Monthly Usage</span>
            </div>
            <span className="mono-data text-[12px] text-muted-foreground">
              {billing.usage} / {billing.limit} posts
            </span>
          </div>
          <div className="w-full bg-muted rounded-full h-1.5">
            <div
              className={`h-1.5 rounded-full transition-all duration-500 ${
                usagePct >= 100
                  ? "bg-destructive"
                  : usagePct >= 80
                    ? "bg-amber"
                    : "bg-foreground/70"
              }`}
              style={{ width: `${usagePct}%` }}
            />
          </div>
          {billing.warning && (
            <p
              className={`text-[12px] mt-2 ${
                billing.warning === "over_limit"
                  ? "text-destructive"
                  : "text-amber"
              }`}
            >
              {billing.warning === "over_limit"
                ? "You've exceeded your monthly limit. Upgrade to continue posting."
                : `${Math.round(billing.percentage)}% of monthly limit used.`}
            </p>
          )}
        </div>
      )}

      {/* Quick nav grid */}
      <div
        className="animate-fade-up"
        style={{ animationDelay: "180ms" }}
      >
        <p className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground mb-3">
          Manage
        </p>
        <div className="grid grid-cols-2 gap-2">
          {QUICK_NAV.map((item) => {
            const Icon = item.icon;
            return (
              <Link key={item.href} href={`/projects/${id}${item.href}`}>
                <div className="group card-hover flex items-center gap-3.5 px-4 py-3 rounded-lg border border-border bg-card hover:border-foreground/15">
                  <div className="w-8 h-8 rounded-md bg-muted flex items-center justify-center shrink-0">
                    <Icon
                      className="w-4 h-4 text-muted-foreground"
                      strokeWidth={1.75}
                    />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-[13px] font-medium">{item.label}</p>
                    <p className="text-[11px] text-muted-foreground">
                      {item.desc}
                    </p>
                  </div>
                  <ArrowRight className="w-3.5 h-3.5 text-muted-foreground/30 group-hover:text-muted-foreground/60 transition-colors shrink-0" />
                </div>
              </Link>
            );
          })}
        </div>
      </div>
    </div>
  );
}
