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
const COOKIE_KEY = "unipost-theme";

const ThemeContext = createContext<ThemeContextValue | null>(null);

function getCookieTheme(): ResolvedTheme | null {
  if (typeof document === "undefined") return null;
  const match = document.cookie.match(/(?:^|; )unipost-theme=(light|dark)(?:;|$)/);
  return match?.[1] === "light" || match?.[1] === "dark" ? match[1] : null;
}

function persistTheme(theme: ResolvedTheme) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(STORAGE_KEY, theme);
  const host = window.location.hostname;
  const domain =
    host === "unipost.dev" || host.endsWith(".unipost.dev")
      ? "; domain=.unipost.dev"
      : "";
  document.cookie = `${COOKIE_KEY}=${theme}; path=/; max-age=31536000; samesite=lax${domain}`;
}

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
    const cookieTheme = getCookieTheme();
    if (cookieTheme) {
      return cookieTheme;
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
    persistTheme(nextTheme);
  }, []);

  useEffect(() => {
    const onStorage = (event: StorageEvent) => {
      if (event.key !== STORAGE_KEY) return;
      if (event.newValue === "light" || event.newValue === "dark") {
        setThemeState(event.newValue);
      }
    };

    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
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
