"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth, UserButton, useUser } from "@clerk/nextjs";
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
  ChevronRight,
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
  const { user } = useUser();
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

  const pageLabel = PROJECT_NAV.find((n) => isActive(n.href))?.label;

  return (
    <div style={{ display: "flex", height: "100vh", overflow: "hidden" }}>
      {/* ── SIDEBAR ── */}
      <aside
        style={{
          width: 220,
          minWidth: 220,
          background: "var(--surface)",
          borderRight: "1px solid var(--dborder)",
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
      >
        {/* Logo */}
        <Link
          href="/"
          style={{
            display: "flex",
            alignItems: "center",
            gap: 9,
            padding: "16px 16px 14px",
            borderBottom: "1px solid var(--dborder)",
            textDecoration: "none",
          }}
        >
          <div
            style={{
              width: 26,
              height: 26,
              background: "var(--daccent)",
              borderRadius: 6,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              flexShrink: 0,
              boxShadow: "0 0 12px var(--accent-dim)",
            }}
          >
            <Zap style={{ width: 14, height: 14, color: "#000" }} strokeWidth={2.5} />
          </div>
          <span
            style={{
              fontSize: 14,
              fontWeight: 600,
              letterSpacing: -0.3,
              color: "var(--dtext)",
            }}
          >
            UniPost
          </span>
        </Link>

        {/* Project selector */}
        <DropdownMenu>
          <DropdownMenuTrigger render={<button className="project-selector" />}>
            <div className="project-initial">
              {currentProject?.name?.charAt(0).toUpperCase() || "P"}
            </div>
            <span
              style={{
                fontSize: 12.5,
                fontWeight: 500,
                color: "var(--dtext)",
                flex: 1,
                whiteSpace: "nowrap",
                overflow: "hidden",
                textOverflow: "ellipsis",
              }}
            >
              {currentProject?.name || "Select project"}
            </span>
            <ChevronRight
              style={{ width: 12, height: 12, color: "var(--dmuted2)", flexShrink: 0 }}
            />
          </DropdownMenuTrigger>
          <DropdownMenuContent
            side="bottom"
            align="start"
            sideOffset={4}
            className="w-[196px]"
          >
            <DropdownMenuLabel>Projects</DropdownMenuLabel>
            {projects.map((p) => (
              <DropdownMenuItem
                key={p.id}
                onSelect={() => router.push(`/projects/${p.id}`)}
                className={p.id === projectId ? "bg-accent" : ""}
              >
                <span
                  style={{
                    width: 16,
                    height: 16,
                    borderRadius: 3,
                    background: "var(--accent-dim)",
                    display: "inline-flex",
                    alignItems: "center",
                    justifyContent: "center",
                    fontSize: 9,
                    fontWeight: 700,
                    color: "var(--daccent)",
                    flexShrink: 0,
                  }}
                >
                  {p.name.charAt(0).toUpperCase()}
                </span>
                <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {p.name}
                </span>
              </DropdownMenuItem>
            ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => router.push("/projects/new")}>
              <Plus style={{ width: 14, height: 14, color: "var(--daccent)" }} />
              <span>New Project</span>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

        {/* Nav */}
        <nav
          style={{
            padding: "16px 10px 8px",
            flex: 1,
            overflowY: "auto",
          }}
        >
          {projectId ? (
            <>
              <div
                style={{
                  fontSize: 10,
                  fontWeight: 600,
                  letterSpacing: "0.08em",
                  textTransform: "uppercase",
                  color: "var(--dmuted2)",
                  padding: "0 6px",
                  marginBottom: 4,
                }}
              >
                Navigate
              </div>
              {PROJECT_NAV.map((item) => {
                const active = isActive(item.href);
                const Icon = item.icon;
                return (
                  <Link
                    key={item.href}
                    href={`/projects/${projectId}${item.href}`}
                    data-active={active}
                    className="sidebar-nav-item"
                  >
                    <Icon style={{ width: 14, height: 14 }} strokeWidth={1.75} />
                    {item.label}
                  </Link>
                );
              })}
            </>
          ) : (
            <Link
              href="/"
              data-active={pathname === "/"}
              className="sidebar-nav-item"
            >
              <FolderOpen style={{ width: 14, height: 14 }} strokeWidth={1.75} />
              Projects
            </Link>
          )}
        </nav>

        {/* User */}
        <div
          style={{
            padding: 10,
            borderTop: "1px solid var(--dborder)",
          }}
        >
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 9,
              padding: "6px 8px",
              borderRadius: 6,
            }}
          >
            <UserButton
              appearance={{ elements: { avatarBox: "w-6 h-6" } }}
            />
            <span
              style={{
                fontSize: 12,
                color: "var(--dmuted)",
                flex: 1,
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}
            >
              {user?.primaryEmailAddress?.emailAddress || "Account"}
            </span>
          </div>
        </div>
      </aside>

      {/* ── MAIN ── */}
      <main style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
        {/* Topbar */}
        <div
          style={{
            height: 44,
            borderBottom: "1px solid var(--dborder)",
            display: "flex",
            alignItems: "center",
            padding: "0 24px",
            gap: 6,
            flexShrink: 0,
          }}
        >
          <Breadcrumb
            pathname={pathname}
            projectName={currentProject?.name}
            pageLabel={pageLabel}
          />
        </div>
        {/* Content */}
        <div style={{ flex: 1, overflowY: "auto", padding: "28px 32px" }}>
          {children}
        </div>
      </main>
    </div>
  );
}

function Breadcrumb({
  pathname,
  projectName,
  pageLabel,
}: {
  pathname: string;
  projectName?: string;
  pageLabel?: string;
}) {
  const segments = pathname.split("/").filter(Boolean);

  return (
    <div style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 12.5, color: "var(--dmuted)" }}>
      <Link href="/" style={{ color: "var(--dmuted)", textDecoration: "none" }}>
        Projects
      </Link>
      {segments[0] === "projects" && segments[1] && (
        <>
          <span style={{ color: "var(--dmuted2)" }}>/</span>
          <Link
            href={`/projects/${segments[1]}`}
            style={{ color: "var(--dmuted)", textDecoration: "none" }}
          >
            {projectName || segments[1].slice(0, 8)}
          </Link>
          {(segments[2] || pageLabel) && (
            <>
              <span style={{ color: "var(--dmuted2)" }}>/</span>
              <span style={{ color: "var(--dtext)", fontWeight: 500 }}>
                {pageLabel || capitalize(segments[2])}
              </span>
            </>
          )}
        </>
      )}
    </div>
  );
}

function capitalize(s: string) {
  return s ? s.charAt(0).toUpperCase() + s.slice(1).replace(/-/g, " ") : "";
}
