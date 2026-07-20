import { createNavigation } from "next-intl/navigation";
import { defineRouting } from "next-intl/routing";
import { defaultLocale, releasedLocales } from "./locales";

export const routing = defineRouting({
  locales: releasedLocales,
  defaultLocale,
  localePrefix: "as-needed",
});

export const { Link, redirect, usePathname, useRouter, getPathname } = createNavigation(routing);
