"use client";

import { Laptop, Moon, Sun } from "lucide-react";

import { useTheme } from "@/components/theme-provider";

const OPTIONS = [
  { value: "light", label: "Light", icon: Sun },
  { value: "dark", label: "Dark", icon: Moon },
  { value: "system", label: "System", icon: Laptop },
] as const;

export function ThemeToggle() {
  const { theme, setTheme } = useTheme();

  return (
    <div className="theme-toggle" aria-label="Theme switcher" role="group">
      {OPTIONS.map((option) => {
        const Icon = option.icon;

        return (
          <button
            key={option.value}
            type="button"
            className="theme-toggle-btn"
            data-active={theme === option.value}
            onClick={() => setTheme(option.value)}
            aria-pressed={theme === option.value}
            title={option.label}
          >
            <Icon style={{ width: 14, height: 14 }} strokeWidth={1.9} />
          </button>
        );
      })}
    </div>
  );
}
