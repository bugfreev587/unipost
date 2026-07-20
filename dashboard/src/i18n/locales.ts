export const plannedLocales = ["en", "es", "vi", "hu", "zh-CN", "zh-TW"] as const;
export const releasedLocales = ["en", "es"] as const;

export type PlannedLocale = (typeof plannedLocales)[number];
export type ReleasedLocale = (typeof releasedLocales)[number];

export const defaultLocale = "en" as const satisfies ReleasedLocale;
export const localeCookieName = "unipost_locale" as const;

export const localeRegistry = {
  en: { nativeName: "English", direction: "ltr", released: true, fallback: null },
  es: { nativeName: "Español", direction: "ltr", released: true, fallback: "en" },
  vi: { nativeName: "Tiếng Việt", direction: "ltr", released: false, fallback: "en" },
  hu: { nativeName: "Magyar", direction: "ltr", released: false, fallback: "en" },
  "zh-CN": { nativeName: "简体中文", direction: "ltr", released: false, fallback: "en" },
  "zh-TW": { nativeName: "繁體中文", direction: "ltr", released: false, fallback: "en" },
} as const satisfies Record<PlannedLocale, {
  nativeName: string;
  direction: "ltr" | "rtl";
  released: boolean;
  fallback: ReleasedLocale | null;
}>;

const localizedPublicPaths = new Set(["/", "/pricing"]);

export function isReleasedLocale(value: string | null | undefined): value is ReleasedLocale {
  return releasedLocales.includes(value as ReleasedLocale);
}

export function isPlannedLocale(value: string | null | undefined): value is PlannedLocale {
  return plannedLocales.includes(value as PlannedLocale);
}

export function isLocaleLikePrefix(value: string | null | undefined) {
  return typeof value === "string" && /^[a-z]{2}(?:-[a-z]{2})?$/i.test(value);
}

export function stripLocalePrefix(pathname: string) {
  const match = pathname.match(/^\/([^/]+)(\/.*)?$/);
  if (!match || !isReleasedLocale(match[1])) {
    return { locale: defaultLocale, pathname } as const;
  }

  return {
    locale: match[1],
    pathname: match[2] || "/",
  } as const;
}

export function isLocalizedPublicPathname(pathname: string) {
  const stripped = stripLocalePrefix(pathname);
  return localizedPublicPaths.has(stripped.pathname);
}

export function isLocalizedPublicBasePathname(pathname: string) {
  return localizedPublicPaths.has(pathname);
}

export function localizePublicPathname(pathname: string, locale: ReleasedLocale) {
  const hashIndex = pathname.indexOf("#");
  const searchIndex = pathname.indexOf("?");
  const suffixIndex = [hashIndex, searchIndex].filter((index) => index >= 0).sort((a, b) => a - b)[0];
  const barePathname = suffixIndex === undefined ? pathname : pathname.slice(0, suffixIndex);
  const suffix = suffixIndex === undefined ? "" : pathname.slice(suffixIndex);
  const stripped = stripLocalePrefix(barePathname).pathname;

  if (!localizedPublicPaths.has(stripped)) return `${stripped}${suffix}`;
  if (locale === defaultLocale) return `${stripped}${suffix}`;
  return `${stripped === "/" ? `/${locale}` : `/${locale}${stripped}`}${suffix}`;
}
