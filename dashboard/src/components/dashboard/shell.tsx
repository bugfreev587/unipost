"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth, useUser, useClerk } from "@clerk/nextjs";
import { ThemeToggle } from "@/components/theme-toggle";
import { UniPostMark } from "@/components/brand/unipost-logo";
// useClerk kept for signOut
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { listProfiles, getWorkspace, getBilling, getMe, type Profile, type Workspace, type BillingInfo } from "@/lib/api";
import { buildContactPageHref } from "@/lib/support";
import {
  Key,
  Webhook,
  Send,
  ListTodo,
  BarChart3,
  ChevronDown,
  Settings,
  Shield,
  LogOut,
  Mail,
  Cable,
  Layers,
  MessageSquare,
  PanelLeftClose,
  PanelLeftOpen,
  GraduationCap,
  BookOpen,
} from "lucide-react";

// Feature flag: NEXT_PUBLIC_FEATURE_INBOX controls Inbox visibility.
// "true" = everyone, "user_id1,user_id2" = only those users, unset = hidden.
function isFeatureEnabled(envVar: string | undefined, userId: string | undefined, userEmail: string | undefined): boolean {
  if (!envVar) return false;
  if (envVar === "true") return true;
  const allowed = envVar.split(",").map((s) => s.trim().toLowerCase());
  if (userId && allowed.includes(userId)) return true;
  if (userEmail && allowed.includes(userEmail.toLowerCase())) return true;
  return false;
}

// Items with `featureFlag` are gated by the env var check.
const ALL_NAV_ITEMS = [
  { href: "/profile", label: "Profiles", icon: Layers },
  { href: "/accounts", label: "Connections", icon: Cable, submenu: [
    { href: "/accounts", label: "Quickstart" },
    { href: "/accounts/native", label: "White-label" },
    { href: "/users", label: "Developer App Users" },
  ]},
  { href: "/posts", label: "Posts", icon: Send, exactMatch: true },
  { href: "/posts/queue", label: "Queue", icon: ListTodo, exactMatch: true },
  { href: "/inbox", label: "Inbox", icon: MessageSquare, featureFlag: "INBOX" },
  { href: "/api-keys", label: "API Keys", icon: Key },
  { href: "/webhooks", label: "Webhooks", icon: Webhook },
  { href: "/analytics", label: "Analytics", icon: BarChart3, submenu: [
    { href: "/analytics", label: "Posts" },
    { href: "/analytics/api", label: "API" },
  ]},
];

const FEATURE_FLAGS: Record<string, string | undefined> = {
  INBOX: process.env.NEXT_PUBLIC_FEATURE_INBOX,
};

// Facebook Pages is in-development and only exposed to SUPER_ADMINS
// (internal team). The authoritative check lives server-side via the
// /v1/me response's is_super_admin field — consumers read that rather
// than consult an env var, so the allowlist stays in one place
// (SUPER_ADMINS) on the API.
export function isFacebookEnabledForMe(isSuperAdmin: boolean | undefined): boolean {
  return !!isSuperAdmin;
}

// Filter nav items based only on feature flags.
function filterNavItems(userId?: string, userEmail?: string) {
  return ALL_NAV_ITEMS.filter((item) => {
    // Feature flag gate
    if ("featureFlag" in item && item.featureFlag) {
      if (!isFeatureEnabled(FEATURE_FLAGS[item.featureFlag], userId, userEmail)) return false;
    }
    return true;
  }).map((item) => {
    if (!item.submenu) return item;
    const filteredSub = item.submenu;
    return { ...item, submenu: filteredSub.length > 0 ? filteredSub : undefined };
  }).filter((item) => item.submenu === undefined || item.submenu.length > 0);
}

function navItemIsActive(pathname: string, profileId: string | undefined, itemHref: string, exactMatch?: boolean) {
  if (!profileId) return false;
  const fullHref = `/projects/${profileId}${itemHref}`;
  return exactMatch ? pathname === fullHref : pathname.startsWith(fullHref);
}

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { getToken } = useAuth();
  const { user } = useUser();
  const { signOut } = useClerk();
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [billing, setBilling] = useState<BillingInfo | null>(null);
  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  // Only one submenu should be expanded at a time.
  const [expandedMenu, setExpandedMenu] = useState<string | null>(() => {
    if (typeof window === "undefined") return null;
    for (const item of ALL_NAV_ITEMS) {
      if (item.submenu && window.location.pathname.includes(item.href)) {
        return item.href;
      }
    }
    return null;
  });
  const [isAdmin, setIsAdmin] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  const profileMatch = pathname.match(/^\/projects\/([^/]+)/);
  const urlProfileId = profileMatch?.[1];
  const profileId = urlProfileId ?? profiles[0]?.id;
  const currentProfile = profiles.find((p) => p.id === profileId);

  const navItems = filterNavItems(user?.id, user?.primaryEmailAddress?.emailAddress);

  // Auto-expand the submenu that matches the current URL on navigation,
  // but only when the pathname actually changes — not on every render.
  // This prevents the effect from overriding manual submenu toggling.
  useEffect(() => {
    const activeSubmenuParent = navItems.find((item) => item.submenu && pathname.includes(item.href));
    if (activeSubmenuParent) {
      setExpandedMenu(activeSubmenuParent.href);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pathname]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await listProfiles(token);
        if (!cancelled) setProfiles(res.data);
      } catch { /* silent */ }
    })();

    return () => { cancelled = true; };
  }, [getToken]);

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
        if (cancelled) return;
        setIsAdmin(!!res.data.is_admin);
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

  const displayName = user?.firstName || user?.username || "User";
  const planName = billing?.plan_name || "Free";
  const avatarUrl = user?.imageUrl;
  const supportHref = buildContactPageHref({
    topic: "dashboard-help",
    source: "sidebar",
  });

  return (
    <div style={{ display: "flex", height: "100vh", overflow: "hidden" }}>
      {/* ── SIDEBAR ── */}
      <aside
        style={{
          width: sidebarCollapsed ? 0 : 220,
          minWidth: sidebarCollapsed ? 0 : 220,
          background: "linear-gradient(180deg, color-mix(in srgb, var(--sidebar) 98%, transparent), color-mix(in srgb, var(--surface2) 80%, transparent))",
          borderRight: sidebarCollapsed ? "none" : "1px solid var(--dborder)",
          display: "flex", flexDirection: "column",
          overflow: "hidden",
          transition: "width 0.2s, min-width 0.2s",
        }}
      >
        {/* ── Top: User profile ── */}
        <div style={{ padding: "14px 10px", borderBottom: "1px solid var(--dborder)", display: "flex", alignItems: "center", gap: 8 }}>
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <button
                  style={{
                    flex: 1, minWidth: 0, display: "flex", alignItems: "center", gap: 10,
                    padding: "6px 8px", borderRadius: 8, border: "none",
                    background: "transparent", cursor: "pointer",
                    transition: "background 0.1s", textAlign: "left",
                  }}
                  onMouseEnter={(e) => { e.currentTarget.style.background = "var(--sidebar-accent)"; }}
                  onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; }}
                />
              }
            >
              {avatarUrl ? (
                <img src={avatarUrl} alt="" style={{ width: 32, height: 32, borderRadius: "50%", flexShrink: 0, objectFit: "cover" }} />
              ) : (
                <div style={{ width: 32, height: 32, borderRadius: "50%", background: "linear-gradient(135deg, var(--daccent), var(--primary-hover))", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 12, fontWeight: 700, color: "var(--primary-foreground)", flexShrink: 0 }}>
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
                Account lives under /settings/account (sidebar Settings entry)
                and Theme toggles from the bottom of the sidebar — kept out
                of this menu to avoid duplicating controls.
                Base UI's Menu.Item exposes onClick (NOT onSelect — that's
                Radix). Earlier handlers used onSelect and were silently
                ignored, which is why these items did nothing on click.
              */}
              <DropdownMenuItem onClick={() => router.push("/contact")} style={{ padding: "10px 14px" }}>
                <Mail style={{ width: 14, height: 14 }} /><span>Contact us</span>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => signOut({ redirectUrl: "https://unipost.dev" })} style={{ padding: "10px 14px" }}>
                <LogOut style={{ width: 14, height: 14 }} /><span>Log out</span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <button
            onClick={() => setSidebarCollapsed(true)}
            title="Collapse sidebar"
            style={{
              flexShrink: 0,
              width: 32, height: 32, borderRadius: 8,
              border: "none", background: "transparent",
              color: "var(--dmuted)", cursor: "pointer",
              display: "flex", alignItems: "center", justifyContent: "center",
            }}
            onMouseEnter={(e) => { e.currentTarget.style.color = "var(--dtext)"; e.currentTarget.style.background = "var(--sidebar-accent)"; }}
            onMouseLeave={(e) => { e.currentTarget.style.color = "var(--dmuted)"; e.currentTarget.style.background = "transparent"; }}
          >
            <PanelLeftClose style={{ width: 18, height: 18 }} />
          </button>
        </div>

        {/* ── Middle: Nav items ── */}
        <nav style={{ padding: "16px 10px 8px", flex: 1, overflowY: "auto" }}>
          {profileId ? (
            <>
              <div style={{ fontSize: 10, fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--dmuted2)", padding: "0 6px", marginBottom: 4 }}>
                Navigate
              </div>
              {navItems.map((item) => {
                const active = navItemIsActive(pathname, profileId, item.href, "exactMatch" in item ? item.exactMatch : undefined);
                const Icon = item.icon;
                const hasSubmenu = !!item.submenu;
                const submenuOpen = hasSubmenu && expandedMenu === item.href;

                return (
                  <div key={item.href}>
                    {hasSubmenu ? (
                      <button
                        onClick={() => setExpandedMenu((current) => current === item.href ? null : item.href)}
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
                                color: subActive ? "var(--daccent)" : "var(--dmuted)",
                                textDecoration: "none",
                                transition: "all 0.1s",
                                marginBottom: 2,
                                background: subActive ? "var(--accent-dim)" : "transparent",
                              }}
                              onMouseEnter={(e) => { if (!subActive) { e.currentTarget.style.color = "var(--dtext)"; e.currentTarget.style.background = "var(--sidebar-accent)"; } }}
                              onMouseLeave={(e) => { if (!subActive) { e.currentTarget.style.color = "var(--dmuted)"; e.currentTarget.style.background = "transparent"; } }}
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
              <UniPostMark size={14} />
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

        <div style={{ padding: "0 10px 10px" }}>
          <Link
            href={supportHref}
            style={{
              display: "flex",
              alignItems: "flex-start",
              gap: 10,
              padding: "12px 12px",
              borderRadius: 12,
              border: "1px solid rgba(59,130,246,0.24)",
              background: "linear-gradient(180deg, rgba(59,130,246,0.14), rgba(59,130,246,0.08))",
              color: "var(--dtext)",
              textDecoration: "none",
            }}
          >
            <div
              style={{
                width: 32,
                height: 32,
                borderRadius: 10,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                background: "rgba(59,130,246,0.18)",
                color: "#93c5fd",
                flexShrink: 0,
              }}
            >
              <Mail style={{ width: 16, height: 16 }} strokeWidth={1.75} />
            </div>
            <div style={{ minWidth: 0 }}>
              <div className="dt-body-sm" style={{ fontWeight: 600, color: "var(--dtext)" }}>
                Contact support
              </div>
              <div className="dt-micro" style={{ color: "var(--dmuted)", lineHeight: 1.5, marginTop: 2 }}>
                Get help with failed actions, billing, or account issues.
              </div>
            </div>
          </Link>
        </div>

        {/* ── Bottom actions: tutorials + theme ── */}
        <div style={{ padding: "4px 10px", display: "flex", alignItems: "center", justifyContent: "flex-end", gap: 8 }}>
          <a
            href="https://unipost.dev/docs"
            target="_blank"
            rel="noopener noreferrer"
            title="Docs"
            aria-label="Docs"
            style={{
              display: "inline-flex", alignItems: "center", justifyContent: "center",
              width: 30, height: 30, borderRadius: 8,
              border: "1px solid var(--dborder)",
              background: "transparent",
              color: "var(--dmuted)",
              transition: "background 0.1s, color 0.1s",
            }}
            onMouseEnter={(e) => { e.currentTarget.style.color = "var(--dtext)"; e.currentTarget.style.background = "var(--sidebar-accent)"; }}
            onMouseLeave={(e) => {
              e.currentTarget.style.color = "var(--dmuted)";
              e.currentTarget.style.background = "transparent";
            }}
          >
            <BookOpen style={{ width: 16, height: 16 }} strokeWidth={1.75} />
          </a>
          <Link
            href="/tutorials"
            title="Tutorials"
            aria-label="Tutorials"
            data-active={pathname.startsWith("/tutorials")}
            style={{
              display: "inline-flex", alignItems: "center", justifyContent: "center",
              width: 30, height: 30, borderRadius: 8,
              border: "1px solid var(--dborder)",
              background: pathname.startsWith("/tutorials") ? "var(--sidebar-accent)" : "transparent",
              color: pathname.startsWith("/tutorials") ? "var(--daccent)" : "var(--dmuted)",
              transition: "background 0.1s, color 0.1s",
            }}
            onMouseEnter={(e) => { e.currentTarget.style.color = "var(--dtext)"; e.currentTarget.style.background = "var(--sidebar-accent)"; }}
            onMouseLeave={(e) => {
              const active = pathname.startsWith("/tutorials");
              e.currentTarget.style.color = active ? "var(--daccent)" : "var(--dmuted)";
              e.currentTarget.style.background = active ? "var(--sidebar-accent)" : "transparent";
            }}
          >
            <GraduationCap style={{ width: 16, height: 16 }} strokeWidth={1.75} />
          </Link>
          <ThemeToggle />
        </div>

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
              onMouseEnter={(e) => { e.currentTarget.style.background = "var(--sidebar-accent)"; e.currentTarget.style.color = "var(--dtext)"; }}
              onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; e.currentTarget.style.color = "var(--dmuted)"; }}
              title="Settings"
            >
              <Settings style={{ width: 14, height: 14 }} strokeWidth={1.75} />
            </Link>
          </div>
        )}
      </aside>

      {/* ── MAIN ── */}
      <main
        style={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
          position: "relative",
          background:
            "radial-gradient(circle at top right, color-mix(in srgb, var(--accent-glow) 100%, transparent), transparent 28%), var(--app-bg)",
        }}
      >
        {sidebarCollapsed && (
          <button
            onClick={() => setSidebarCollapsed(false)}
            title="Expand sidebar"
            style={{
              position: "absolute", top: 14, left: 14, zIndex: 10,
              width: 32, height: 32, borderRadius: 8,
              border: "1px solid var(--dborder)",
              background: "var(--sidebar)",
              color: "var(--dmuted)", cursor: "pointer",
              display: "flex", alignItems: "center", justifyContent: "center",
              boxShadow: "0 2px 8px rgba(0,0,0,.15)",
            }}
            onMouseEnter={(e) => { e.currentTarget.style.color = "var(--dtext)"; }}
            onMouseLeave={(e) => { e.currentTarget.style.color = "var(--dmuted)"; }}
          >
            <PanelLeftOpen style={{ width: 16, height: 16 }} />
          </button>
        )}
        <div style={{ flex: 1, overflowY: "auto", padding: "28px 32px 40px" }}>
          {children}
        </div>
      </main>
    </div>
  );
}
