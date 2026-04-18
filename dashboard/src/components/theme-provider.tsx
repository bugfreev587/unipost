"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

type ThemePreference = "light" | "dark" | "system";
type ResolvedTheme = "light" | "dark";

type ThemeContextValue = {
  theme: ResolvedTheme;
  resolvedTheme: ResolvedTheme;
  setTheme: (theme: ResolvedTheme) => void;
};

const STORAGE_KEY = "unipost-theme";

const ThemeContext = createContext<ThemeContextValue | null>(null);

function getSystemTheme(): ResolvedTheme {
  if (typeof window === "undefined") {
    return "dark";
  }

  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ResolvedTheme>(() => {
    if (typeof window === "undefined") {
      return "dark";
    }
    const storedTheme = window.localStorage.getItem(STORAGE_KEY);
    if (storedTheme === "light" || storedTheme === "dark") {
      return storedTheme;
    }
    return getSystemTheme();
  });

  const applyTheme = useCallback((nextResolvedTheme: ResolvedTheme) => {
    const root = document.documentElement;

    root.classList.toggle("dark", nextResolvedTheme === "dark");
    root.classList.toggle("light", nextResolvedTheme === "light");
    root.style.colorScheme = nextResolvedTheme;
    root.dataset.theme = nextResolvedTheme;
  }, []);

  useEffect(() => {
    applyTheme(theme);
  }, [applyTheme, theme]);

  const setTheme = useCallback((nextTheme: ResolvedTheme) => {
    setThemeState(nextTheme);
    window.localStorage.setItem(STORAGE_KEY, nextTheme);
  }, []);

  const value = useMemo<ThemeContextValue>(() => ({
    theme,
    resolvedTheme: theme,
    setTheme,
  }), [theme, setTheme]);

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const context = useContext(ThemeContext);
  if (!context) {
    throw new Error("useTheme must be used within ThemeProvider");
  }

  return context;
}
