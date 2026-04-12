"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth, useUser, useClerk } from "@clerk/nextjs";
import { OnboardingTourProvider, TourTriggerButton } from "@/components/dashboard/onboarding-tour";
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
  Mail,
  Cable,
  Layers,
} from "lucide-react";

// Each nav item can be tagged with the usage modes that require it.
// Items with no `modes` array are always shown.
const ALL_NAV_ITEMS = [
  { href: "/profile", label: "Profiles", icon: Layers },
  { href: "/accounts", label: "Connections", icon: Cable, submenu: [
    { href: "/accounts", label: "Accounts", modes: ["personal", "whitelabel"] },
    { href: "/users", label: "Connect Flow", modes: ["api"] },
    { href: "/accounts/native", label: "Credentials", modes: ["whitelabel"] },
  ]},
  { href: "/posts", label: "Posts", icon: Send, submenu: [
    { href: "/posts", label: "Overview" },
    { href: "/posts/queue", label: "Queue" },
  ]},
  { href: "/api-keys", label: "API Keys", icon: Key, modes: ["whitelabel", "api"] },
  { href: "/analytics", label: "Analytics", icon: BarChart3, submenu: [
    { href: "/analytics", label: "Posts" },
    { href: "/analytics/api", label: "API" },
  ]},
];

// Filter nav items based on workspace usage modes.
// Empty modes array = show everything.
function filterNavItems(modes: string[]) {
  if (modes.length === 0) return ALL_NAV_ITEMS;
  return ALL_NAV_ITEMS.filter((item) => {
    if (!("modes" in item) || !item.modes) return true;
    return item.modes.some((m: string) => modes.includes(m));
  }).map((item) => {
    if (!item.submenu) return item;
    const filteredSub = item.submenu.filter((sub) => {
      if (!("modes" in sub) || !sub.modes) return true;
      return (sub.modes as string[]).some((m: string) => modes.includes(m));
    });
    return { ...item, submenu: filteredSub.length > 0 ? filteredSub : undefined };
  }).filter((item) => item.submenu === undefined || item.submenu.length > 0);
}

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { getToken } = useAuth();
  const { user } = useUser();
  const { signOut, openUserProfile } = useClerk();
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [billing, setBilling] = useState<BillingInfo | null>(null);
  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  // Auto-expand submenus that match the current path on initial render
  const [expandedMenus, setExpandedMenus] = useState<Set<string>>(() => {
    const initial = new Set<string>();
    for (const item of ALL_NAV_ITEMS) {
      if (item.submenu && typeof window !== "undefined" && window.location.pathname.includes(item.href)) {
        initial.add(item.href);
      }
    }
    return initial;
  });
  const [isAdmin, setIsAdmin] = useState(false);

  const profileMatch = pathname.match(/^\/projects\/([^/]+)/);
  const urlProfileId = profileMatch?.[1];
  const profileId = urlProfileId ?? profiles[0]?.id;
  const currentProfile = profiles.find((p) => p.id === profileId);

  const navItems = filterNavItems(workspace?.usage_modes ?? []);

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
    <OnboardingTourProvider>
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
                <div className="dt-body-sm" style={{ fontWeight: 600, color: "var(--dtext)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{displayName}</div>
                <div className="dt-micro" style={{ color: "var(--daccent)" }}>{planName}</div>
              </div>
              <ChevronDown style={{ width: 14, height: 14, color: "var(--dmuted2)", flexShrink: 0 }} />
            </DropdownMenuTrigger>
            <DropdownMenuContent side="bottom" align="start" sideOffset={4} className="w-[210px]">
              <div className="dt-mono" style={{ padding: "10px 14px 8px", lineHeight: 1.5 }}>
                Signed in as<br />
                <span style={{ color: "var(--dtext)", fontWeight: 500 }}>{user?.primaryEmailAddress?.emailAddress || ""}</span>
              </div>
              <DropdownMenuSeparator />
              {/*
                Base UI's Menu.Item exposes onClick (NOT onSelect — that's
                Radix). Earlier handlers used onSelect and were silently
                ignored, which is why these items did nothing on click.
              */}
              <DropdownMenuItem onClick={() => router.push("/settings/account")} style={{ padding: "10px 14px" }}>
                <User style={{ width: 14, height: 14 }} /><span>Account</span>
              </DropdownMenuItem>
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
              {navItems.map((item) => {
                const active = isActive(item.href);
                const Icon = item.icon;
                const hasSubmenu = !!item.submenu;
                const submenuOpen = hasSubmenu && expandedMenus.has(item.href);

                const tourId = item.label.toLowerCase().replace(/\s+/g, "-");
                return (
                  <div key={item.href} data-tour={tourId}>
                    {hasSubmenu ? (
                      <button
                        onClick={() => setExpandedMenus(prev => {
                          const next = new Set(prev);
                          if (next.has(item.href)) next.delete(item.href);
                          else next.add(item.href);
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
                              className="dt-body-sm"
                              style={{
                                display: "block",
                                padding: "6px 12px",
                                borderRadius: 6,
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

        {/* ── Take a tour ── */}
        <div style={{ padding: "4px 10px" }}>
          <TourTriggerButton />
        </div>

        {/* ── Bottom: Workspace ── */}
        {workspace && (
          <div
            data-tour="workspace"
            style={{
              padding: "10px 10px",
              borderTop: "1px solid var(--dborder)",
              display: "flex",
              alignItems: "center",
              gap: 8,
            }}
          >
            <span
              className="dt-mono"
              style={{
                flex: 1,
                overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
              }}
            >
              {workspace.name}
            </span>
            <Link
              href="/settings"
              style={{
                display: "flex", alignItems: "center", justifyContent: "center",
                width: 28, height: 28, borderRadius: 6,
                color: "var(--dmuted)", transition: "background 0.1s, color 0.1s", flexShrink: 0,
              }}
              onMouseEnter={(e) => { e.currentTarget.style.background = "var(--surface2)"; e.currentTarget.style.color = "var(--dtext)"; }}
              onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; e.currentTarget.style.color = "var(--dmuted)"; }}
              title="Settings"
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
    </OnboardingTourProvider>
  );
}
