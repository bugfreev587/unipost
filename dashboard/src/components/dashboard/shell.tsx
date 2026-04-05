"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth, UserButton } from "@clerk/nextjs";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuLabel,
} from "@/components/ui/dropdown-menu";
import { listProjects, type Project } from "@/lib/api";
import {
  Key,
  Users,
  Send,
  CreditCard,
  Settings,
  ChevronDown,
  Plus,
  FolderOpen,
  Zap,
} from "lucide-react";

const PROJECT_NAV = [
  { href: "/api-keys", label: "API Keys", icon: Key },
  { href: "/accounts", label: "Accounts", icon: Users },
  { href: "/posts", label: "Posts", icon: Send },
  { href: "/billing", label: "Billing", icon: CreditCard },
  { href: "/settings", label: "Settings", icon: Settings },
];

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { getToken } = useAuth();
  const [projects, setProjects] = useState<Project[]>([]);

  const projectMatch = pathname.match(/^\/projects\/([^/]+)/);
  const projectId = projectMatch?.[1];
  const currentProject = projects.find((p) => p.id === projectId);

  const loadProjects = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await listProjects(token);
      setProjects(res.data);
    } catch {
      // silent
    }
  }, [getToken]);

  useEffect(() => {
    loadProjects();
  }, [loadProjects]);

  function isActive(href: string) {
    if (!projectId) return false;
    const full = `/projects/${projectId}${href}`;
    return pathname.startsWith(full);
  }

  return (
    <div className="flex min-h-screen bg-[#0a0a0a]">
      {/* ── Sidebar ── */}
      <aside className="w-[220px] shrink-0 flex flex-col border-r border-[#1e1e1e] bg-[#0a0a0a]">
        {/* Logo */}
        <div className="h-[52px] flex items-center px-4 border-b border-[#1e1e1e]">
          <Link href="/" className="flex items-center gap-2.5">
            <div className="w-[22px] h-[22px] rounded bg-emerald flex items-center justify-center">
              <Zap className="w-3 h-3 text-emerald-foreground" strokeWidth={2.5} />
            </div>
            <span className="text-[14px] font-semibold text-[#e5e5e5] tracking-tight">
              UniPost
            </span>
          </Link>
        </div>

        {/* Project selector */}
        <div className="px-3 pt-3 pb-1">
          <DropdownMenu>
            <DropdownMenuTrigger
              className="w-full flex items-center justify-between px-2.5 py-2 rounded-md bg-[#111111] border border-[#1e1e1e] hover:border-[#2a2a2a] transition-colors cursor-pointer"
            >
              <div className="flex items-center gap-2 min-w-0">
                <div className="w-5 h-5 rounded bg-[#1a1a1a] flex items-center justify-center shrink-0">
                  <span className="text-[10px] font-semibold text-[#737373]">
                    {currentProject?.name?.charAt(0).toUpperCase() || "P"}
                  </span>
                </div>
                <span className="text-[12px] font-medium text-[#d4d4d4] truncate">
                  {currentProject?.name || "Select project"}
                </span>
              </div>
              <ChevronDown className="w-3 h-3 text-[#525252] shrink-0" />
            </DropdownMenuTrigger>
            <DropdownMenuContent side="bottom" align="start" sideOffset={4} className="w-[196px]">
              <DropdownMenuLabel>Projects</DropdownMenuLabel>
              {projects.map((p) => (
                <DropdownMenuItem
                  key={p.id}
                  onSelect={() => router.push(`/projects/${p.id}`)}
                  className={p.id === projectId ? "bg-accent" : ""}
                >
                  <div className="w-4 h-4 rounded bg-[#1a1a1a] flex items-center justify-center shrink-0">
                    <span className="text-[9px] font-bold text-[#525252]">
                      {p.name.charAt(0).toUpperCase()}
                    </span>
                  </div>
                  <span className="truncate">{p.name}</span>
                </DropdownMenuItem>
              ))}
              <DropdownMenuSeparator />
              <DropdownMenuItem onSelect={() => router.push("/projects/new")}>
                <Plus className="w-3.5 h-3.5 text-emerald" />
                <span>New Project</span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        {/* Navigation */}
        <nav className="flex-1 px-3 py-3 space-y-0.5 overflow-y-auto">
          {projectId ? (
            <>
              <div className="px-2.5 pt-1 pb-2">
                <span className="text-[10px] font-medium uppercase tracking-[0.1em] text-[#525252]">
                  Navigate
                </span>
              </div>
              {PROJECT_NAV.map((item) => {
                const active = isActive(item.href);
                const Icon = item.icon;
                return (
                  <Link
                    key={item.href}
                    href={`/projects/${projectId}${item.href}`}
                    data-active={active}
                    className={`nav-item flex items-center gap-2.5 px-2.5 py-[7px] rounded-md text-[13px] transition-colors ${
                      active
                        ? "bg-[#141414] text-[#e5e5e5] font-medium"
                        : "text-[#737373] hover:text-[#a3a3a3] hover:bg-[#111111]"
                    }`}
                  >
                    <Icon className="w-[15px] h-[15px] shrink-0" strokeWidth={1.75} />
                    <span>{item.label}</span>
                  </Link>
                );
              })}
            </>
          ) : (
            <Link
              href="/"
              data-active={pathname === "/"}
              className={`nav-item flex items-center gap-2.5 px-2.5 py-[7px] rounded-md text-[13px] transition-colors ${
                pathname === "/"
                  ? "bg-[#141414] text-[#e5e5e5] font-medium"
                  : "text-[#737373] hover:text-[#a3a3a3] hover:bg-[#111111]"
              }`}
            >
              <FolderOpen className="w-[15px] h-[15px] shrink-0" strokeWidth={1.75} />
              <span>Projects</span>
            </Link>
          )}
        </nav>

        {/* User section */}
        <div className="px-4 py-3 border-t border-[#1e1e1e] flex items-center gap-2.5">
          <UserButton
            appearance={{
              elements: {
                avatarBox: "w-6 h-6",
              },
            }}
          />
          <span className="text-[11px] text-[#525252] truncate">Account</span>
        </div>
      </aside>

      {/* ── Main content ── */}
      <main className="flex-1 min-w-0 bg-[#0a0a0a]">
        {/* Top bar */}
        <div className="h-[52px] border-b border-[#1e1e1e] flex items-center px-8">
          <Breadcrumb pathname={pathname} currentProject={currentProject} />
        </div>
        {/* Content */}
        <div className="px-8 py-6 max-w-[960px]">{children}</div>
      </main>
    </div>
  );
}

function Breadcrumb({
  pathname,
  currentProject,
}: {
  pathname: string;
  currentProject?: Project;
}) {
  const segments = pathname.split("/").filter(Boolean);
  const crumbs: { label: string; href: string }[] = [];

  crumbs.push({ label: "Projects", href: "/" });

  if (segments[0] === "projects" && segments[1]) {
    const id = segments[1];
    crumbs.push({
      label: currentProject?.name || truncateId(id),
      href: `/projects/${id}`,
    });
    if (segments[2]) {
      const sub = segments[2];
      const nav = PROJECT_NAV.find((n) => n.href === `/${sub}`);
      crumbs.push({
        label: nav?.label || capitalize(sub),
        href: `/projects/${id}/${sub}`,
      });
    }
  }

  return (
    <div className="flex items-center gap-1.5 text-[13px]">
      {crumbs.map((crumb, i) => (
        <span key={crumb.href} className="flex items-center gap-1.5">
          {i > 0 && (
            <span className="text-[#2a2a2a] select-none">/</span>
          )}
          {i === crumbs.length - 1 ? (
            <span className="text-[#d4d4d4] font-medium">{crumb.label}</span>
          ) : (
            <Link
              href={crumb.href}
              className="text-[#525252] hover:text-[#a3a3a3] transition-colors"
            >
              {crumb.label}
            </Link>
          )}
        </span>
      ))}
    </div>
  );
}

function truncateId(id: string) {
  return id.length <= 10 ? id : id.slice(0, 8) + "\u2026";
}

function capitalize(s: string) {
  return s.charAt(0).toUpperCase() + s.slice(1).replace(/-/g, " ");
}
