"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const TABS = [
  { href: "/settings/account", label: "Account" },
  { href: "/settings/workspace", label: "Workspace" },
  { href: "/settings/dashboard", label: "Dashboard" },
  { href: "/settings/notifications", label: "Notifications" },
  { href: "/settings/billing", label: "Billing" },
];

export default function SettingsLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  return (
    <div>
      <div style={{ marginBottom: 20 }}>
        <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>
          Settings
        </div>
        <div style={{ fontSize: 14, color: "#aaa", marginTop: 6 }}>
          Manage your account, workspace, and billing.
        </div>
      </div>

      <div
        style={{
          display: "flex",
          gap: 4,
          borderBottom: "1px solid var(--dborder)",
          marginBottom: 24,
        }}
      >
        {TABS.map((tab) => {
          const active = pathname === tab.href || pathname.startsWith(`${tab.href}/`);
          return (
            <Link
              key={tab.href}
              href={tab.href}
              style={{
                padding: "10px 16px",
                fontSize: 13,
                fontWeight: active ? 600 : 500,
                color: active ? "var(--dtext)" : "var(--dmuted)",
                textDecoration: "none",
                borderBottom: active ? "2px solid var(--daccent)" : "2px solid transparent",
                marginBottom: -1,
                transition: "color 0.1s, border-color 0.1s",
              }}
              onMouseEnter={(e) => {
                if (!active) e.currentTarget.style.color = "var(--dtext)";
              }}
              onMouseLeave={(e) => {
                if (!active) e.currentTarget.style.color = "var(--dmuted)";
              }}
            >
              {tab.label}
            </Link>
          );
        })}
      </div>

      {children}
    </div>
  );
}
