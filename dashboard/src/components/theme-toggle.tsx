"use client";

import { useSyncExternalStore } from "react";
import { Moon, Sun } from "lucide-react";

import { useTheme } from "@/components/theme-provider";

function subscribeToClientSnapshot() {
  return () => {};
}

function getClientSnapshot() {
  return true;
}

function getServerSnapshot() {
  return false;
}

export function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme();
  const mounted = useSyncExternalStore(subscribeToClientSnapshot, getClientSnapshot, getServerSnapshot);
  const isDark = mounted && resolvedTheme === "dark";
  const Icon = isDark ? Moon : Sun;
  const nextTheme = resolvedTheme === "dark" ? "light" : "dark";
  let label = "Toggle theme";
  if (mounted) {
    label = isDark ? "Switch to light theme" : "Switch to dark theme";
  }

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
