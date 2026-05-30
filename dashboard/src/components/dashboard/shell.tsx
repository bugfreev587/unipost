"use client";

import { useEffect, useState, useSyncExternalStore } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth, useUser, useClerk } from "@clerk/nextjs";
import { UniPostMark } from "@/components/brand/unipost-logo";
import { useTheme } from "@/components/theme-provider";
import { isFeatureInDevEnabledForMe } from "@/lib/features-in-dev";
import { FEATURE_FLAG_KEYS } from "@/lib/feature-flags";
import { useFeatureFlags } from "@/lib/use-feature-flags";
// useClerk kept for signOut
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { listProfiles, getWorkspace, getBilling, getMe, type Profile, type Workspace, type BillingInfo } from "@/lib/api";
import { useGlobalInboxUnreadCount } from "@/lib/use-inbox-unread";
import { buildContactPageHref } from "@/lib/support";
import { shouldLoadGlobalInboxUnreadCount } from "@/components/dashboard/inbox-unread-gate";
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
  FileText,
  PanelLeftClose,
  PanelLeftOpen,
  BookOpen,
  Sun,
  Moon,
  type LucideIcon,
} from "lucide-react";

type NavSubItem = {
  href: string;
  label: string;
  backendFlag?: string;
  backendFlagsAny?: string[];
  showWhenAdmin?: boolean;
};

type NavItem = {
  href: string;
  label: string;
  icon: LucideIcon;
  exactMatch?: boolean;
  backendFlag?: string;
  backendFlagsAny?: string[];
  showWhenAdmin?: boolean;
  submenu?: NavSubItem[];
};

// Items with `backendFlag` are gated by /v1/me/features.
const ALL_NAV_ITEMS: NavItem[] = [
  { href: "/profile", label: "Profiles", icon: Layers },
  { href: "/accounts", label: "Connections", icon: Cable, submenu: [
    { href: "/accounts", label: "Quickstart" },
    { href: "/accounts/native", label: "White-label" },
    { href: "/accounts/app-review", label: "App Review", backendFlag: FEATURE_FLAG_KEYS.appReviewAutopilotV1 },
    { href: "/users", label: "Developer App Users" },
  ]},
  { href: "/posts", label: "Posts", icon: Send, exactMatch: true },
  { href: "/posts/queue", label: "Queue", icon: ListTodo, exactMatch: true },
  { href: "/inbox", label: "Inbox", icon: MessageSquare },
  { href: "/api-keys", label: "API Keys", icon: Key },
  { href: "/webhooks", label: "Webhooks", icon: Webhook },
  { href: "/logs", label: "Logs", icon: FileText },
  { href: "/analytics", label: "Analytics", icon: BarChart3, submenu: [
    { href: "/analytics", label: "Posts" },
    { href: "/analytics/platforms", label: "Platforms" },
    { href: "/analytics/api", label: "API" },
  ]},
];

export function isFacebookEnabledForMe(isSuperAdmin: boolean | undefined): boolean {
  return isFeatureInDevEnabledForMe("facebook_pages", isSuperAdmin);
}

function subscribeToClientSnapshot() {
  return () => {};
}

function getClientSnapshot() {
  return true;
}

function getServerSnapshot() {
  return false;
}

// Filter nav items based on backend feature flags plus internal admin-only surfaces.
function filterNavItems(backendFlags?: Record<string, boolean>, isAdmin = false) {
  return ALL_NAV_ITEMS.filter((item) => {
    const adminAllowed = isAdmin && item.showWhenAdmin;
    if (item.backendFlag && !backendFlags?.[item.backendFlag] && !adminAllowed) return false;
    if (item.backendFlagsAny && !item.backendFlagsAny.some((flag) => backendFlags?.[flag]) && !adminAllowed) return false;
    return true;
  }).map((item) => {
    if (!item.submenu) return item;
    const filteredSub = item.submenu.filter((sub) => {
      const adminAllowed = isAdmin && sub.showWhenAdmin;
      if (sub.backendFlag && !backendFlags?.[sub.backendFlag] && !adminAllowed) return false;
      if (sub.backendFlagsAny && !sub.backendFlagsAny.some((flag) => backendFlags?.[flag]) && !adminAllowed) return false;
      return true;
    });
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
  const { resolvedTheme, setTheme } = useTheme();
  const { flags: backendFeatureFlags, planGates } = useFeatureFlags();
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [billing, setBilling] = useState<BillingInfo | null>(null);
  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  const themeMounted = useSyncExternalStore(subscribeToClientSnapshot, getClientSnapshot, getServerSnapshot);
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

  // Global inbox unread badge: the Inbox surface is public, but the
  // unread network work should only start once the current plan allows
  // Inbox. /v1/me/features already carries this plan gate, so the shell
  // avoids adding a /v1/limits waterfall to every dashboard page.
  // Disabled = 0 returned, no network calls, no WS connection.
  const planAllowsInbox = planGates.inbox ?? false;
  const inboxUnreadCount = useGlobalInboxUnreadCount(
    shouldLoadGlobalInboxUnreadCount({ profileId, planAllowsInbox }),
  );

  const navItems = filterNavItems(backendFeatureFlags, isAdmin);

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
        const res = await getBilling(token);
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
        const res = await getWorkspace(token);
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
  const settingsActive = pathname.startsWith("/settings");
  const themeIsDark = themeMounted && resolvedTheme === "dark";
  const ThemeIcon = themeIsDark ? Moon : Sun;
  const nextTheme = resolvedTheme === "dark" ? "light" : "dark";
  let themeLabel = "Toggle theme";
  if (themeMounted) {
    themeLabel = themeIsDark ? "Switch to light theme" : "Switch to dark theme";
  }

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
                        style={{ display: "flex", alignItems: "center", gap: 10, justifyContent: "flex-start" }}
                      >
                        <Icon style={{ width: 18, height: 18 }} strokeWidth={1.75} />
                        <span>{item.label}</span>
                        {item.href === "/inbox" && inboxUnreadCount > 0 ? (
                          // Notification-red unread badge — distinct
                          // from the nav-item's "active" green so users
                          // see a new comment/DM at a glance even when
                          // they're on the Inbox tab. Caps at 99+ so a
                          // wide badge doesn't push the layout around.
                          <span
                            aria-label={`${inboxUnreadCount} unread`}
                            style={{
                              marginLeft: "auto",
                              minWidth: 18,
                              height: 18,
                              padding: "0 6px",
                              borderRadius: 999,
                              // Dark emerald-700 — distinct from the
                              // active-tab pale-green AND from the
                              // brand --daccent (which is brighter),
                              // so the count reads cleanly on the
                              // sidebar's neutral background AND on
                              // the active-row's tinted background.
                              background: "#047857",
                              color: "white",
                              fontSize: 10,
                              fontWeight: 700,
                              lineHeight: "18px",
                              textAlign: "center",
                            }}
                          >
                            {inboxUnreadCount > 99 ? "99+" : inboxUnreadCount}
                          </span>
                        ) : null}
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
            title="Get help with failed actions, billing, or account issues"
            style={{
              display: "flex",
              alignItems: "center",
              gap: 8,
              padding: "8px 10px",
              borderRadius: 10,
              border: "1px solid color-mix(in srgb, var(--daccent) 22%, var(--dborder))",
              background: "color-mix(in srgb, var(--daccent) 8%, transparent)",
              color: "var(--dtext)",
              textDecoration: "none",
              fontSize: 13,
              fontWeight: 500,
              transition: "background 0.12s ease, border-color 0.12s ease",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.background = "color-mix(in srgb, var(--daccent) 14%, transparent)";
              e.currentTarget.style.borderColor = "color-mix(in srgb, var(--daccent) 38%, var(--dborder))";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.background = "color-mix(in srgb, var(--daccent) 8%, transparent)";
              e.currentTarget.style.borderColor = "color-mix(in srgb, var(--daccent) 22%, var(--dborder))";
            }}
          >
            <Mail
              style={{ width: 14, height: 14, color: "var(--daccent)", flexShrink: 0 }}
              strokeWidth={1.75}
            />
            <span style={{ flex: 1, minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              Contact support
            </span>
          </Link>
        </div>

        {/* ── Bottom actions: docs ── */}
        <div style={{ padding: "4px 10px 10px", display: "flex", flexDirection: "column", alignItems: "stretch", gap: 8 }}>
          <a
            href="https://unipost.dev/docs"
            target="_blank"
            rel="noopener noreferrer"
            title="Open docs"
            aria-label="Open docs"
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "flex-start",
              gap: 8,
              minWidth: 0,
              width: "100%",
              height: 36,
              padding: "0 14px",
              borderRadius: 12,
              border: "1px solid color-mix(in srgb, var(--daccent) 12%, var(--dborder))",
              background: "color-mix(in srgb, var(--surface) 86%, transparent)",
              color: "var(--dtext)",
              textDecoration: "none",
              fontSize: 13,
              fontWeight: 600,
              transition: "transform 0.12s ease, background 0.12s ease, border-color 0.12s ease, box-shadow 0.12s ease",
              boxShadow: "0 0 0 rgba(0,0,0,0)",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.transform = "translateY(-1px)";
              e.currentTarget.style.background = "color-mix(in srgb, var(--daccent) 10%, var(--surface))";
              e.currentTarget.style.borderColor = "color-mix(in srgb, var(--daccent) 34%, var(--dborder))";
              e.currentTarget.style.boxShadow = "0 10px 24px rgba(0,0,0,.22)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.transform = "translateY(0)";
              e.currentTarget.style.background = "color-mix(in srgb, var(--surface) 86%, transparent)";
              e.currentTarget.style.borderColor = "color-mix(in srgb, var(--daccent) 12%, var(--dborder))";
              e.currentTarget.style.boxShadow = "0 0 0 rgba(0,0,0,0)";
            }}
          >
            <BookOpen style={{ width: 16, height: 16, color: "var(--dmuted)" }} strokeWidth={1.75} />
            <span>Docs</span>
          </a>
        </div>

        {/* ── Bottom: Workspace + settings/theme icons ── */}
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
                minWidth: 0,
                overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
              }}
            >
              {workspace.name}
            </span>
            <Link
              href="/settings"
              title="Open settings"
              aria-label="Open settings"
              style={{
                display: "inline-flex",
                alignItems: "center",
                justifyContent: "center",
                flexShrink: 0,
                width: 28,
                height: 28,
                borderRadius: 8,
                border: settingsActive
                  ? "1px solid color-mix(in srgb, var(--daccent) 32%, var(--dborder))"
                  : "1px solid var(--dborder)",
                background: settingsActive
                  ? "color-mix(in srgb, var(--daccent) 10%, var(--surface))"
                  : "color-mix(in srgb, var(--surface) 82%, transparent)",
                color: settingsActive ? "var(--daccent)" : "var(--dmuted)",
                textDecoration: "none",
                transition: "background 0.12s ease, border-color 0.12s ease, color 0.12s ease",
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = "color-mix(in srgb, var(--daccent) 8%, var(--surface))";
                e.currentTarget.style.borderColor = "color-mix(in srgb, var(--daccent) 28%, var(--dborder))";
                e.currentTarget.style.color = settingsActive ? "var(--daccent)" : "var(--dtext)";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = settingsActive
                  ? "color-mix(in srgb, var(--daccent) 10%, var(--surface))"
                  : "color-mix(in srgb, var(--surface) 82%, transparent)";
                e.currentTarget.style.borderColor = settingsActive
                  ? "color-mix(in srgb, var(--daccent) 32%, var(--dborder))"
                  : "var(--dborder)";
                e.currentTarget.style.color = settingsActive ? "var(--daccent)" : "var(--dmuted)";
              }}
            >
              <Settings style={{ width: 14, height: 14 }} strokeWidth={1.75} />
            </Link>
            <button
              type="button"
              onClick={() => setTheme(nextTheme)}
              title={themeLabel}
              aria-label={themeLabel}
              style={{
                display: "inline-flex",
                alignItems: "center",
                justifyContent: "center",
                flexShrink: 0,
                width: 28,
                height: 28,
                borderRadius: 8,
                border: "1px solid var(--dborder)",
                background: "color-mix(in srgb, var(--surface) 82%, transparent)",
                color: "var(--dmuted)",
                cursor: "pointer",
                transition: "background 0.12s ease, border-color 0.12s ease, color 0.12s ease",
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = "color-mix(in srgb, var(--daccent) 8%, var(--surface))";
                e.currentTarget.style.borderColor = "color-mix(in srgb, var(--daccent) 28%, var(--dborder))";
                e.currentTarget.style.color = "var(--dtext)";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = "color-mix(in srgb, var(--surface) 82%, transparent)";
                e.currentTarget.style.borderColor = "var(--dborder)";
                e.currentTarget.style.color = "var(--dmuted)";
              }}
            >
              <ThemeIcon style={{ width: 14, height: 14 }} strokeWidth={1.75} />
            </button>
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
