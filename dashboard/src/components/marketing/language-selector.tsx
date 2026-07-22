"use client";

import { Check, ChevronDown } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { usePathname } from "next/navigation";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  defaultLocale,
  isReleasedLocale,
  localeCookieName,
  localeRegistry,
  localizePublicPathname,
  releasedLocales,
  type ReleasedLocale,
} from "@/i18n/locales";

function persistLocaleCookie(locale: ReleasedLocale) {
  const hostname = window.location.hostname;
  const cookieDomain =
    hostname === "unipost.dev" || hostname.endsWith(".unipost.dev")
      ? "; Domain=.unipost.dev"
      : "";
  const secure = window.location.protocol === "https:" ? "; Secure" : "";
  document.cookie = `${localeCookieName}=${locale}; Path=/; Max-Age=31536000; SameSite=Lax${secure}${cookieDomain}`;
}

export function LanguageSelector() {
  const requestedLocale = useLocale();
  const locale = isReleasedLocale(requestedLocale) ? requestedLocale : defaultLocale;
  const pathname = usePathname();
  const t = useTranslations("navigation");

  function selectLocale(nextLocale: ReleasedLocale) {
    if (nextLocale === locale) return;

    persistLocaleCookie(nextLocale);

    // Locale lives in the shared root layout. A document navigation ensures
    // <html lang> and the next-intl provider are recreated for the new locale.
    window.location.assign(localizePublicPathname(pathname, nextLocale));
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <button
            type="button"
            className="mk-nav-link mk-language-trigger"
            aria-label={t("languageSelectorLabel")}
          />
        }
      >
        <span>{localeRegistry[locale].nativeName}</span>
        <ChevronDown aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        side="bottom"
        align="end"
        sideOffset={8}
        className="mk-nav-dropdown-content mk-language-menu"
      >
        {releasedLocales.map((nextLocale) => {
          const selected = nextLocale === locale;
          return (
            <DropdownMenuItem
              key={nextLocale}
              className="mk-language-item"
              aria-current={selected ? "true" : undefined}
              onClick={() => selectLocale(nextLocale)}
            >
              <span>{localeRegistry[nextLocale].nativeName}</span>
              {selected ? <Check aria-hidden="true" /> : null}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
