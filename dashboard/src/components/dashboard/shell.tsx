"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth, useUser, useClerk } from "@clerk/nextjs";
// useClerk kept for signOut
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { listProjects, getBilling, createProject, type Project, type BillingInfo } from "@/lib/api";
import {
  Key,
  Users,
  Send,
  BarChart3,
  ChevronDown,
  ChevronRight,
  ChevronsUpDown,
  Settings,
  Zap,
  LogOut,
  User,
  CreditCard,
  Mail,
  X,
  Plus,
} from "lucide-react";

const NAV_ITEMS = [
  { href: "/api-keys", label: "API Keys", icon: Key },
  { href: "/accounts", label: "Accounts", icon: Users, submenu: [
    { href: "/accounts", label: "Quickstart Mode" },
    { href: "/accounts/native", label: "Native Mode" },
  ]},
  { href: "/posts", label: "Posts", icon: Send },
  { href: "/analytics", label: "Analytics", icon: BarChart3 },
];

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { getToken } = useAuth();
  const { user } = useUser();
  const { signOut, openUserProfile } = useClerk();
  const [projects, setProjects] = useState<Project[]>([]);
  const [billing, setBilling] = useState<BillingInfo | null>(null);
  const [projectSwitcherOpen, setProjectSwitcherOpen] = useState(false);
  const [newProjectName, setNewProjectName] = useState("");
  const [creatingProject, setCreatingProject] = useState(false);
  const [accountsExpanded, setAccountsExpanded] = useState(false);

  const projectMatch = pathname.match(/^\/projects\/([^/]+)/);
  const projectId = projectMatch?.[1];
  const currentProject = projects.find((p) => p.id === projectId);

  const loadProjects = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await listProjects(token);
      setProjects(res.data);
    } catch { /* silent */ }
  }, [getToken]);

  useEffect(() => { loadProjects(); }, [loadProjects]);

  useEffect(() => {
    async function loadBilling() {
      if (!projectId) return;
      try {
        const token = await getToken();
        if (!token) return;
        const res = await getBilling(token, projectId);
        setBilling(res.data);
      } catch { /* silent */ }
    }
    loadBilling();
  }, [projectId, getToken]);

  function isActive(href: string) {
    if (!projectId) return false;
    return pathname.startsWith(`/projects/${projectId}${href}`);
  }

  const displayName = user?.firstName || user?.username || "User";
  const planName = billing?.plan_name || "Free";
  const avatarUrl = user?.imageUrl;

  async function handleCreateProject() {
    if (!newProjectName.trim()) return;
    setCreatingProject(true);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await createProject(token, { name: newProjectName.trim() });
      setNewProjectName("");
      setProjectSwitcherOpen(false);
      await loadProjects();
      router.push(`/projects/${res.data.id}`);
    } catch { /* silent */ }
    finally { setCreatingProject(false); }
  }

  return (
    <div style={{ display: "flex", height: "100vh", overflow: "hidden" }}>
      {/* ── SIDEBAR ── */}
      <aside
        style={{
          width: 220, minWidth: 220,
          background: "var(--surface)",
          borderRight: "1px solid var(--dborder)",
          display: "flex", flexDirection: "column",
          overflow: "hidden",
        }}
      >
        {/* ── Top: User profile ── */}
        <div style={{ padding: "14px 10px", borderBottom: "1px solid var(--dborder)" }}>
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <button
                  style={{
                    width: "100%", display: "flex", alignItems: "center", gap: 10,
                    padding: "6px 8px", borderRadius: 8, border: "none",
                    background: "transparent", cursor: "pointer",
                    transition: "background 0.1s", textAlign: "left",
                  }}
                  onMouseEnter={(e) => { e.currentTarget.style.background = "var(--surface2)"; }}
                  onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; }}
                />
              }
            >
              {avatarUrl ? (
                <img src={avatarUrl} alt="" style={{ width: 32, height: 32, borderRadius: "50%", flexShrink: 0, objectFit: "cover" }} />
              ) : (
                <div style={{ width: 32, height: 32, borderRadius: "50%", background: "linear-gradient(135deg, var(--daccent), #059669)", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 12, fontWeight: 700, color: "#000", flexShrink: 0 }}>
                  {displayName.charAt(0).toUpperCase()}
                </div>
              )}
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: 13, fontWeight: 600, color: "var(--dtext)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{displayName}</div>
                <div style={{ fontSize: 11, fontWeight: 500, color: "var(--daccent)" }}>{planName}</div>
              </div>
              <ChevronDown style={{ width: 14, height: 14, color: "var(--dmuted2)", flexShrink: 0 }} />
            </DropdownMenuTrigger>
            <DropdownMenuContent side="bottom" align="start" sideOffset={4} className="w-[210px]">
              <div style={{ padding: "10px 14px 8px", fontSize: 12, color: "var(--dmuted)", lineHeight: 1.5 }}>
                Signed in as<br />
                <span style={{ color: "var(--dtext)", fontWeight: 500 }}>{user?.primaryEmailAddress?.emailAddress || ""}</span>
              </div>
              <DropdownMenuSeparator />
              {/*
                Base UI's Menu.Item exposes onClick (NOT onSelect — that's
                Radix). Earlier handlers used onSelect and were silently
                ignored, which is why these items did nothing on click.
              */}
              <DropdownMenuItem onClick={() => router.push("/account")} style={{ padding: "10px 14px" }}>
                <User style={{ width: 14, height: 14 }} /><span>Account</span>
              </DropdownMenuItem>
              {projectId && (
                <DropdownMenuItem onClick={() => router.push(`/projects/${projectId}/billing`)} style={{ padding: "10px 14px" }}>
                  <CreditCard style={{ width: 14, height: 14 }} /><span>Billing</span>
                </DropdownMenuItem>
              )}
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => router.push("/contact")} style={{ padding: "10px 14px" }}>
                <Mail style={{ width: 14, height: 14 }} /><span>Contact us</span>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => signOut({ redirectUrl: "https://unipost.dev" })} style={{ padding: "10px 14px" }}>
                <LogOut style={{ width: 14, height: 14 }} /><span>Log out</span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        {/* ── Middle: Nav items ── */}
        <nav style={{ padding: "16px 10px 8px", flex: 1, overflowY: "auto" }}>
          {projectId ? (
            <>
              <div style={{ fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--dmuted2)", padding: "0 6px", marginBottom: 4 }}>
                Navigate
              </div>
              {NAV_ITEMS.map((item) => {
                const active = isActive(item.href);
                const Icon = item.icon;
                const hasSubmenu = !!item.submenu;
                const submenuOpen = hasSubmenu && (accountsExpanded || pathname.startsWith(`/projects/${projectId}/accounts`));

                return (
                  <div key={item.href}>
                    {hasSubmenu ? (
                      <button
                        onClick={() => setAccountsExpanded(!submenuOpen)}
                        className="sidebar-nav-item"
                        style={{ width: "100%", border: "none", background: "transparent", cursor: "pointer", fontFamily: "inherit", justifyContent: "space-between" }}
                      >
                        <span style={{ display: "flex", alignItems: "center", gap: 10 }}>
                          <Icon style={{ width: 18, height: 18 }} strokeWidth={1.75} />
                          {item.label}
                        </span>
                        <ChevronDown style={{
                          width: 14, height: 14, color: "var(--dmuted)",
                          transition: "transform 0.2s",
                          transform: submenuOpen ? "rotate(0deg)" : "rotate(-90deg)",
                        }} />
                      </button>
                    ) : (
                      <Link
                        href={`/projects/${projectId}${item.href}`}
                        data-active={active}
                        className="sidebar-nav-item"
                      >
                        <Icon style={{ width: 18, height: 18 }} strokeWidth={1.75} />
                        {item.label}
                      </Link>
                    )}
                    {hasSubmenu && submenuOpen && item.submenu && (
                      <div style={{ paddingLeft: 28, marginBottom: 4 }}>
                        {item.submenu.map((sub) => {
                          const subActive = pathname === `/projects/${projectId}${sub.href}`;
                          return (
                            <Link
                              key={sub.href}
                              href={`/projects/${projectId}${sub.href}`}
                              style={{
                                display: "block",
                                padding: "6px 12px",
                                borderRadius: 6,
                                fontSize: 13,
                                fontWeight: subActive ? 600 : 400,
                                color: subActive ? "var(--daccent)" : "#888",
                                textDecoration: "none",
                                transition: "all 0.1s",
                                marginBottom: 2,
                                background: subActive ? "var(--accent-dim)" : "transparent",
                              }}
                              onMouseEnter={(e) => { if (!subActive) { e.currentTarget.style.color = "var(--dtext)"; e.currentTarget.style.background = "var(--surface2)"; } }}
                              onMouseLeave={(e) => { if (!subActive) { e.currentTarget.style.color = "#888"; e.currentTarget.style.background = "transparent"; } }}
                            >
                              {sub.label}
                            </Link>
                          );
                        })}
                      </div>
                    )}
                  </div>
                );
              })}
            </>
          ) : (
            <Link href="/" data-active={pathname === "/"} className="sidebar-nav-item">
              <Zap style={{ width: 14, height: 14 }} strokeWidth={1.75} />
              Projects
            </Link>
          )}
        </nav>

        {/* ── Bottom: Current project + settings + switcher ── */}
        {currentProject && (
          <div
            style={{
              padding: "12px 10px",
              borderTop: "1px solid var(--dborder)",
              display: "flex",
              alignItems: "center",
              gap: 8,
            }}
          >
            <div className="project-initial" style={{ width: 28, height: 28, fontSize: 12 }}>
              {currentProject.name.charAt(0).toUpperCase()}
            </div>
            <span
              style={{
                flex: 1, fontSize: 13, fontWeight: 600, color: "var(--dtext)",
                overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
              }}
            >
              {currentProject.name}
            </span>
            <Link
              href={`/projects/${projectId}/settings`}
              style={{
                display: "flex", alignItems: "center", justifyContent: "center",
                width: 30, height: 30, borderRadius: 6,
                color: "var(--dmuted)", transition: "background 0.1s, color 0.1s", flexShrink: 0,
              }}
              onMouseEnter={(e) => { e.currentTarget.style.background = "var(--surface2)"; e.currentTarget.style.color = "var(--dtext)"; }}
              onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; e.currentTarget.style.color = "var(--dmuted)"; }}
              title="Settings"
            >
              <Settings style={{ width: 16, height: 16 }} strokeWidth={1.75} />
            </Link>
            <button
              onClick={() => setProjectSwitcherOpen(true)}
              style={{
                display: "flex", alignItems: "center", justifyContent: "center",
                width: 30, height: 30, borderRadius: 6, border: "none", background: "transparent",
                color: "var(--dmuted)", cursor: "pointer", transition: "background 0.1s, color 0.1s", flexShrink: 0,
              }}
              onMouseEnter={(e) => { e.currentTarget.style.background = "var(--surface2)"; e.currentTarget.style.color = "var(--dtext)"; }}
              onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; e.currentTarget.style.color = "var(--dmuted)"; }}
              title="Switch project"
            >
              <ChevronsUpDown style={{ width: 16, height: 16 }} strokeWidth={1.75} />
            </button>
          </div>
        )}
      </aside>

      {/* ── PROJECT SWITCHER OVERLAY ── */}
      {projectSwitcherOpen && (
        <div
          style={{
            position: "fixed", inset: 0, zIndex: 100,
            background: "#000000cc",
            display: "flex", alignItems: "center", justifyContent: "center",
          }}
          onClick={() => setProjectSwitcherOpen(false)}
        >
          <div
            style={{
              background: "var(--surface)",
              border: "1px solid var(--dborder2)",
              borderRadius: 16,
              width: 520, maxWidth: "90vw",
              maxHeight: "80vh",
              boxShadow: "0 24px 60px #00000080",
              animation: "slideUp 0.2s ease",
              overflow: "hidden",
              display: "flex", flexDirection: "column",
            }}
            onClick={(e) => e.stopPropagation()}
          >
            {/* Header */}
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "20px 24px 16px" }}>
              <div>
                <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 6 }}>
                  <div style={{ width: 28, height: 28, background: "var(--daccent)", borderRadius: 7, display: "flex", alignItems: "center", justifyContent: "center" }}>
                    <Zap style={{ width: 14, height: 14, color: "#000" }} strokeWidth={2.5} />
                  </div>
                  <span style={{ fontSize: 15, fontWeight: 700, color: "var(--dtext)" }}>UniPost</span>
                </div>
                <h2 style={{ fontSize: 22, fontWeight: 800, letterSpacing: -0.5, color: "var(--dtext)" }}>Your Projects</h2>
              </div>
              <button
                onClick={() => setProjectSwitcherOpen(false)}
                style={{
                  width: 32, height: 32, borderRadius: 8, border: "none",
                  background: "transparent", color: "var(--dmuted)", cursor: "pointer",
                  display: "flex", alignItems: "center", justifyContent: "center",
                  transition: "background 0.1s, color 0.1s",
                }}
                onMouseEnter={(e) => { e.currentTarget.style.background = "var(--surface2)"; e.currentTarget.style.color = "var(--dtext)"; }}
                onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; e.currentTarget.style.color = "var(--dmuted)"; }}
              >
                <X style={{ width: 18, height: 18 }} />
              </button>
            </div>

            {/* Project list */}
            <div style={{ padding: "0 24px 20px", overflowY: "auto", flex: 1 }}>
              <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                {projects.map((p) => {
                  const isCurrent = p.id === projectId;
                  return (
                    <button
                      key={p.id}
                      onClick={() => {
                        router.push(`/projects/${p.id}`);
                        setProjectSwitcherOpen(false);
                      }}
                      style={{
                        display: "flex", alignItems: "center", gap: 12,
                        padding: "12px 16px", borderRadius: 10,
                        border: `1px solid ${isCurrent ? "var(--daccent)" + "40" : "var(--dborder2)"}`,
                        background: isCurrent ? "var(--accent-dim)" : "var(--surface2)",
                        cursor: "pointer", transition: "all 0.15s",
                        textAlign: "left", width: "100%",
                      }}
                      onMouseEnter={(e) => { if (!isCurrent) e.currentTarget.style.borderColor = "#333"; e.currentTarget.style.background = "var(--surface3)"; }}
                      onMouseLeave={(e) => { if (!isCurrent) e.currentTarget.style.borderColor = "var(--dborder2)"; e.currentTarget.style.background = isCurrent ? "var(--accent-dim)" : "var(--surface2)"; }}
                    >
                      <div className="project-initial" style={{ width: 28, height: 28, fontSize: 12 }}>
                        {p.name.charAt(0).toUpperCase()}
                      </div>
                      <span style={{ flex: 1, fontSize: 14, fontWeight: 500, color: "var(--dtext)" }}>
                        {p.name}
                      </span>
                      {isCurrent && (
                        <span style={{
                          fontSize: 11, fontWeight: 600, color: "var(--daccent)",
                          fontFamily: "var(--font-geist-mono), monospace",
                        }}>
                          Current
                        </span>
                      )}
                    </button>
                  );
                })}

                {/* Create new project */}
                <div
                  style={{
                    display: "flex", alignItems: "center", gap: 8,
                    padding: "10px 16px", borderRadius: 10,
                    border: "1px dashed var(--dborder2)",
                    background: "transparent",
                  }}
                >
                  <Plus style={{ width: 16, height: 16, color: "var(--daccent)", flexShrink: 0 }} />
                  <input
                    placeholder="Create a new project..."
                    value={newProjectName}
                    onChange={(e) => setNewProjectName(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && handleCreateProject()}
                    style={{
                      flex: 1, border: "none", background: "transparent",
                      color: "var(--dtext)", fontSize: 13, outline: "none",
                      fontFamily: "inherit",
                    }}
                  />
                  {newProjectName.trim() && (
                    <button
                      onClick={handleCreateProject}
                      disabled={creatingProject}
                      className="dbtn dbtn-primary"
                      style={{ padding: "4px 12px", fontSize: 11 }}
                    >
                      {creatingProject ? "..." : "Create"}
                    </button>
                  )}
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ── MAIN ── */}
      <main style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
        <div style={{ flex: 1, overflowY: "auto", padding: "40px 48px" }}>
          {children}
        </div>
      </main>
    </div>
  );
}

