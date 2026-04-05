"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { UserButton } from "@clerk/nextjs";
import {
  FolderOpen,
  Key,
  Users,
  Send,
  ShieldCheck,
  CreditCard,
  Settings,
  ChevronLeft,
  Layers,
} from "lucide-react";

const PROJECT_NAV = [
  { href: "", label: "Overview", icon: Layers },
  { href: "/posts", label: "Posts", icon: Send },
  { href: "/accounts", label: "Accounts", icon: Users },
  { href: "/api-keys", label: "API Keys", icon: Key },
  { href: "/credentials", label: "Credentials", icon: ShieldCheck },
  { href: "/billing", label: "Billing", icon: CreditCard },
  { href: "/settings", label: "Settings", icon: Settings },
];

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  // Extract project context from path
  const projectMatch = pathname.match(/^\/projects\/([^/]+)/);
  const projectId = projectMatch?.[1];
  const isProjectPage = !!projectId;

  function isActive(href: string) {
    if (!projectId) return pathname === href;
    const full = `/projects/${projectId}${href}`;
    // Exact match for overview, prefix match for sub-pages
    return href === "" ? pathname === full : pathname.startsWith(full);
  }

  return (
    <div className="flex min-h-screen">
      {/* Sidebar */}
      <aside className="w-[220px] shrink-0 bg-sidebar text-sidebar-foreground flex flex-col border-r border-sidebar-border">
        {/* Logo */}
        <div className="h-14 flex items-center px-5 border-b border-sidebar-border">
          <Link href="/" className="flex items-center gap-2 group">
            <div className="w-6 h-6 rounded bg-amber text-amber-foreground flex items-center justify-center text-xs font-bold">
              U
            </div>
            <span className="text-[15px] font-semibold text-sidebar-primary tracking-tight">
              UniPost
            </span>
          </Link>
        </div>

        {/* Navigation */}
        <nav className="flex-1 px-3 py-4 space-y-1 overflow-y-auto">
          {isProjectPage ? (
            <>
              {/* Back to projects */}
              <Link
                href="/"
                className="sidebar-link flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-[13px] text-sidebar-foreground/60 hover:text-sidebar-foreground hover:bg-sidebar-accent mb-3 transition-colors"
              >
                <ChevronLeft className="w-3.5 h-3.5" />
                <span>All Projects</span>
              </Link>

              {/* Project section label */}
              <div className="px-2.5 pb-2">
                <span className="text-[10px] font-medium uppercase tracking-[0.08em] text-sidebar-foreground/40">
                  Project
                </span>
              </div>

              {/* Project nav items */}
              {PROJECT_NAV.map((item) => {
                const active = isActive(item.href);
                const Icon = item.icon;
                return (
                  <Link
                    key={item.href}
                    href={`/projects/${projectId}${item.href}`}
                    data-active={active}
                    className={`sidebar-link flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-[13px] font-medium transition-colors ${
                      active
                        ? "bg-sidebar-accent text-sidebar-accent-foreground"
                        : "text-sidebar-foreground hover:text-sidebar-accent-foreground hover:bg-sidebar-accent/60"
                    }`}
                  >
                    <Icon className="w-4 h-4 shrink-0" strokeWidth={1.75} />
                    <span>{item.label}</span>
                  </Link>
                );
              })}
            </>
          ) : (
            <>
              <Link
                href="/"
                data-active={pathname === "/"}
                className={`sidebar-link flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-[13px] font-medium transition-colors ${
                  pathname === "/"
                    ? "bg-sidebar-accent text-sidebar-accent-foreground"
                    : "text-sidebar-foreground hover:text-sidebar-accent-foreground hover:bg-sidebar-accent/60"
                }`}
              >
                <FolderOpen className="w-4 h-4 shrink-0" strokeWidth={1.75} />
                <span>Projects</span>
              </Link>
            </>
          )}
        </nav>

        {/* User */}
        <div className="px-4 py-3 border-t border-sidebar-border flex items-center gap-3">
          <UserButton
            appearance={{
              elements: {
                avatarBox: "w-7 h-7",
              },
            }}
          />
          <span className="text-[12px] text-sidebar-foreground/50 truncate">
            Account
          </span>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 min-w-0">
        <div className="h-14 border-b border-border flex items-center px-8">
          <Breadcrumb pathname={pathname} />
        </div>
        <div className="px-8 py-6 max-w-5xl">{children}</div>
      </main>
    </div>
  );
}

function Breadcrumb({ pathname }: { pathname: string }) {
  const segments = pathname.split("/").filter(Boolean);
  const crumbs: { label: string; href: string }[] = [];

  if (segments.length === 0) {
    crumbs.push({ label: "Projects", href: "/" });
  } else if (segments[0] === "projects") {
    crumbs.push({ label: "Projects", href: "/" });
    if (segments[1]) {
      const id = segments[1];
      crumbs.push({ label: truncateId(id), href: `/projects/${id}` });
      if (segments[2]) {
        const sub = segments[2];
        const label = PROJECT_NAV.find(
          (n) => n.href === `/${sub}` || (sub === "new" && n.href === "")
        )?.label;
        crumbs.push({
          label: label || capitalize(sub),
          href: `/projects/${id}/${sub}`,
        });
      }
    }
  }

  return (
    <div className="flex items-center gap-1.5 text-[13px]">
      {crumbs.map((crumb, i) => (
        <span key={crumb.href} className="flex items-center gap-1.5">
          {i > 0 && (
            <span className="text-muted-foreground/40 select-none">/</span>
          )}
          {i === crumbs.length - 1 ? (
            <span className="text-foreground font-medium">{crumb.label}</span>
          ) : (
            <Link
              href={crumb.href}
              className="text-muted-foreground hover:text-foreground transition-colors"
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
  if (id.length <= 12) return id;
  return id.slice(0, 8) + "...";
}

function capitalize(s: string) {
  return s.charAt(0).toUpperCase() + s.slice(1).replace(/-/g, " ");
}
