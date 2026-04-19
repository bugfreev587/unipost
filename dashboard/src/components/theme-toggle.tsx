"use client";

import { Moon, Sun } from "lucide-react";

import { useTheme } from "@/components/theme-provider";

export function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme();
  const isDark = resolvedTheme === "dark";
  const Icon = isDark ? Moon : Sun;
  const nextTheme = isDark ? "light" : "dark";
  const label = isDark ? "Switch to light theme" : "Switch to dark theme";

  return (
    <div className="theme-picker">
      <button
        type="button"
        className="theme-picker-trigger"
        aria-label={label}
        title={label}
        onClick={() => setTheme(nextTheme)}
      >
        <Icon style={{ width: 15, height: 15 }} strokeWidth={1.9} />
      </button>
    </div>
  );
}
