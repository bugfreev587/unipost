"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { UserButton } from "@clerk/nextjs";

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  return (
    <div className="flex min-h-screen">
      <aside className="w-64 border-r bg-muted/30 p-6 flex flex-col">
        <Link href="/" className="text-xl font-bold mb-8">
          UniPost
        </Link>
        <nav className="flex flex-col gap-1 flex-1">
          <Link
            href="/"
            className={`px-3 py-2 rounded-md text-sm font-medium transition-colors ${
              pathname === "/"
                ? "bg-primary text-primary-foreground"
                : "text-muted-foreground hover:bg-muted hover:text-foreground"
            }`}
          >
            Projects
          </Link>
        </nav>
        <div className="pt-4 border-t">
          <UserButton />
        </div>
      </aside>
      <main className="flex-1 p-8">{children}</main>
    </div>
  );
}
