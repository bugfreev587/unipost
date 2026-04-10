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
import { listProfiles, getWorkspace, getBilling, getMe, type Profile, type Workspace, type BillingInfo } from "@/lib/api";
import {
  Key,
  Users,
  Send,
  BarChart3,
  ChevronDown,
  ChevronsUpDown,
  Settings,
  Shield,
  Zap,
  LogOut,
  User,
  CreditCard,
  Mail,
} from "lucide-react";

const NAV_ITEMS = [
  { href: "/api-keys", label: "API Keys", icon: Key },
  { href: "/accounts", label: "Accounts", icon: Users, submenu: [
    { href: "/accounts", label: "Quickstart Mode" },
    { href: "/accounts/native", label: "White-label" },
  ]},
  { href: "/users", label: "Managed Users", icon: User },
  { href: "/posts", label: "Posts", icon: Send, submenu: [
    { href: "/posts", label: "Overview" },
    { href: "/posts/queue", label: "Queue" },
  ]},
  { href: "/analytics", label: "Analytics", icon: BarChart3 },
];

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { getToken } = useAuth();
  const { user } = useUser();
  const { signOut, openUserProfile } = useClerk();
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [billing, setBilling] = useState<BillingInfo | null>(null);
  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  const [expandedMenus, setExpandedMenus] = useState<Set<string>>(new Set());
  const [isAdmin, setIsAdmin] = useState(false);

  const profileMatch = pathname.match(/^\/projects\/([^/]+)/);
  const profileId = profileMatch?.[1];
  const currentProfile = profiles.find((p) => p.id === profileId);

  const loadProfiles = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await listProfiles(token);
      setProfiles(res.data);
    } catch { /* silent */ }
  }, [getToken]);

  useEffect(() => { loadProfiles(); }, [loadProfiles]);

  // Resolve admin status from the backend ADMIN_USERS allowlist. We
  // intentionally don't read a NEXT_PUBLIC_* env var here — keeping the
  // allowlist server-side means rotating it doesn't require a frontend
  // rebuild, and there's only one source of truth.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await getMe(token);
        if (!cancelled) setIsAdmin(!!res.data.is_admin);
      } catch { /* silent — non-admins simply don't see the link */ }
    })();
    return () => { cancelled = true; };
  }, [getToken]);

  useEffect(() => {
    async function loadBilling() {
      if (!currentProfile?.workspace_id) return;
      try {
        const token = await getToken();
        if (!token) return;
        const res = await getBilling(token, currentProfile.workspace_id);
        setBilling(res.data);
      } catch { /* silent */ }
    }
    loadBilling();
  }, [currentProfile?.workspace_id, getToken]);

  useEffect(() => {
    async function loadWorkspace() {
      if (!currentProfile?.workspace_id) return;
      try {
        const token = await getToken();
        if (!token) return;
        const res = await getWorkspace(token, currentProfile.workspace_id);
        setWorkspace(res.data);
      } catch { /* silent */ }
    }
    loadWorkspace();
  }, [currentProfile?.workspace_id, getToken]);

  function isActive(href: string) {
    if (!profileId) return false;
    return pathname.startsWith(`/projects/${profileId}${href}`);
  }

  const displayName = user?.firstName || user?.username || "User";
  const planName = billing?.plan_name || "Free";
  const avatarUrl = user?.imageUrl;

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
              {profileId && (
                <DropdownMenuItem onClick={() => router.push(`/projects/${profileId}/billing`)} style={{ padding: "10px 14px" }}>
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
          {profileId ? (
            <>
              <div style={{ fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--dmuted2)", padding: "0 6px", marginBottom: 4 }}>
                Navigate
              </div>
              {NAV_ITEMS.map((item) => {
                const active = isActive(item.href);
                const Icon = item.icon;
                const hasSubmenu = !!item.submenu;
                const submenuOpen = hasSubmenu && (
                  expandedMenus.has(item.href) ||
                  pathname.startsWith(`/projects/${profileId}${item.href}`)
                );

                return (
                  <div key={item.href}>
                    {hasSubmenu ? (
                      <button
                        onClick={() => setExpandedMenus(prev => {
                          const next = new Set(prev);
                          if (submenuOpen) next.delete(item.href); else next.add(item.href);
                          return next;
                        })}
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
                        href={`/projects/${profileId}${item.href}`}
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
                          const subActive = pathname === `/projects/${profileId}${sub.href}`;
                          return (
                            <Link
                              key={sub.href}
                              href={`/projects/${profileId}${sub.href}`}
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
            <Link href="/projects" data-active={pathname === "/projects"} className="sidebar-nav-item">
              <Zap style={{ width: 14, height: 14 }} strokeWidth={1.75} />
              Profiles
            </Link>
          )}

          {isAdmin && (
            <Link
              href="/admin"
              data-active={pathname.startsWith("/admin")}
              className="sidebar-nav-item"
            >
              <Shield style={{ width: 14, height: 14 }} strokeWidth={1.75} />
              Admin
            </Link>
          )}
        </nav>

        {/* ── Bottom: Current profile + switcher ── */}
        {currentProfile && (
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
              {currentProfile.name.charAt(0).toUpperCase()}
            </div>
            <span
              style={{
                flex: 1, fontSize: 13, fontWeight: 600, color: "var(--dtext)",
                overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
              }}
            >
              {currentProfile.name}
            </span>
            <Link
              href={`/projects/${profileId}/settings`}
              style={{
                display: "flex", alignItems: "center", justifyContent: "center",
                width: 30, height: 30, borderRadius: 6,
                color: "var(--dmuted)", transition: "background 0.1s, color 0.1s", flexShrink: 0,
              }}
              onMouseEnter={(e) => { e.currentTarget.style.background = "var(--surface2)"; e.currentTarget.style.color = "var(--dtext)"; }}
              onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; e.currentTarget.style.color = "var(--dmuted)"; }}
              title="Profile settings"
            >
              <Settings style={{ width: 16, height: 16 }} strokeWidth={1.75} />
            </Link>
            <Link
              href="/projects"
              style={{
                display: "flex", alignItems: "center", justifyContent: "center",
                width: 30, height: 30, borderRadius: 6, color: "var(--dmuted)",
                transition: "background 0.1s, color 0.1s", flexShrink: 0,
              }}
              onMouseEnter={(e) => { e.currentTarget.style.background = "var(--surface2)"; e.currentTarget.style.color = "var(--dtext)"; }}
              onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; e.currentTarget.style.color = "var(--dmuted)"; }}
              title="Switch profile"
            >
              <ChevronsUpDown style={{ width: 16, height: 16 }} strokeWidth={1.75} />
            </Link>
          </div>
        )}

        {/* ── Bottom: Workspace ── */}
        {workspace && (
          <div
            style={{
              padding: "10px 10px",
              borderTop: "1px solid var(--dborder)",
              display: "flex",
              alignItems: "center",
              gap: 8,
            }}
          >
            <span
              style={{
                flex: 1, fontSize: 12, fontWeight: 500, color: "var(--dmuted)",
                overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
              }}
            >
              {workspace.name}
            </span>
            <Link
              href="/workspace/settings"
              style={{
                display: "flex", alignItems: "center", justifyContent: "center",
                width: 28, height: 28, borderRadius: 6,
                color: "var(--dmuted)", transition: "background 0.1s, color 0.1s", flexShrink: 0,
              }}
              onMouseEnter={(e) => { e.currentTarget.style.background = "var(--surface2)"; e.currentTarget.style.color = "var(--dtext)"; }}
              onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; e.currentTarget.style.color = "var(--dmuted)"; }}
              title="Workspace settings"
            >
              <Settings style={{ width: 14, height: 14 }} strokeWidth={1.75} />
            </Link>
          </div>
        )}
      </aside>

      {/* ── MAIN ── */}
      <main style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
        <div style={{ flex: 1, overflowY: "auto", padding: "40px 48px" }}>
          {children}
        </div>
      </main>
    </div>
  );
}
