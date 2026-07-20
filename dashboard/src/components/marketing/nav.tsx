"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { useAuth, SignInButton, SignUpButton, UserButton } from "@clerk/nextjs";
import { ArrowRight, BookOpen, ChevronDown, History } from "lucide-react";
import { UniPostLogo } from "@/components/brand/unipost-logo";
import { LandingAttribution } from "@/components/marketing/landing-attribution";
import { LanguageSelector } from "@/components/marketing/language-selector";
import { appendLandingSessionId } from "@/lib/landing-attribution";
import { ThemeToggle } from "@/components/theme-toggle";
import {
  defaultLocale,
  isReleasedLocale,
  localizePublicPathname,
} from "@/i18n/locales";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

const APP_URL = process.env.NEXT_PUBLIC_APP_URL || "https://app.unipost.dev";
// New signups land on /welcome, where they enter their first name and
// optional organization name. That endpoint renames the workspace
// seeded by the Clerk user.created webhook ("{FirstName}'s Workspace"
// by default, or the org name if provided) before redirecting to the
// dashboard. The in-dashboard WelcomeModal (intent collection) runs
// after this, on the dashboard's first load.
const SIGN_UP_REDIRECT_URL = `${APP_URL}/welcome`;

const userButtonAppearance = {
  elements: {
    avatarBox: "w-8 h-8",
    userButtonPopoverCard: {
      color: "#1f2937",
    },
  },
};

const PUBLIC_NAV_ITEMS = [
  { href: "/solutions", labelKey: "solutions", key: "solutions" },
  { href: "/tools", labelKey: "tools", key: "tools" },
  { href: "/pricing", labelKey: "pricing", key: "pricing" },
  { href: "/blog", labelKey: "blog", key: "blog" },
] as const;

const DEVELOPER_NAV_ITEMS = [
  { href: "/docs", labelKey: "developer.docs", descriptionKey: "developer.docsDescription", icon: BookOpen },
  { href: "/changelog", labelKey: "developer.changeLogs", descriptionKey: "developer.changeLogsDescription", icon: History },
] as const;

type PublicNavKey = (typeof PUBLIC_NAV_ITEMS)[number]["key"] | "developer";

function useLandingRedirectUrls() {
  const [urls, setUrls] = useState({
    appUrl: APP_URL,
    signUpRedirectUrl: SIGN_UP_REDIRECT_URL,
  });

  useEffect(() => {
    let cancelled = false;
    queueMicrotask(() => {
      if (cancelled) return;
      setUrls({
        appUrl: appendLandingSessionId(APP_URL),
        signUpRedirectUrl: appendLandingSessionId(SIGN_UP_REDIRECT_URL),
      });
    });

    return () => {
      cancelled = true;
    };
  }, []);

  return urls;
}

export function MarketingNav() {
  const { isSignedIn, isLoaded } = useAuth();
  const { appUrl, signUpRedirectUrl } = useLandingRedirectUrls();
  const t = useTranslations("common");

  let controls: ReactNode;

  if (!isLoaded) {
    controls = <div style={{ display: "flex", alignItems: "center", gap: 8, height: 36 }} />;
  } else if (isSignedIn) {
    controls = (
      <div className="mk-auth-controls">
        <ThemeToggle />
        <a href={appUrl} className="mk-auth-btn mk-auth-btn-primary">
          {t("actions.goToDashboard")}
        </a>
        <UserButton appearance={userButtonAppearance} />
      </div>
    );
  } else {
    controls = (
      <div className="mk-auth-row">
        <ThemeToggle />
        <SignInButton mode="redirect" forceRedirectUrl={appUrl}>
          <button className="mk-auth-btn mk-auth-btn-ghost">
            {t("actions.signIn")}
          </button>
        </SignInButton>
        <SignUpButton mode="redirect" forceRedirectUrl={signUpRedirectUrl}>
          <button className="mk-auth-btn mk-auth-btn-primary">
            {t("actions.getStartedFree")}
          </button>
        </SignUpButton>
      </div>
    );
  }

  return (
    <>
      <LandingAttribution />
      {controls}
    </>
  );
}

export function PublicSiteHeader({ active }: { active?: PublicNavKey }) {
  const requestedLocale = useLocale();
  const locale = isReleasedLocale(requestedLocale) ? requestedLocale : defaultLocale;
  const t = useTranslations("navigation");

  return (
    <nav className="mk-nav">
      <div className="mk-nav-inner">
        <div className="mk-nav-left">
          <Link href={localizePublicPathname("/", locale)} className="mk-logo">
            <UniPostLogo markSize={28} wordmarkColor="var(--marketing-text)" />
          </Link>
        </div>
        <div className="mk-nav-links">
          {PUBLIC_NAV_ITEMS.map((item) => (
            <Link
              key={item.key}
              href={localizePublicPathname(item.href, locale)}
              className={`mk-nav-link${active === item.key ? " active" : ""}`}
            >
              {t(item.labelKey)}
            </Link>
          ))}
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <button
                  type="button"
                  className={`mk-nav-link mk-nav-dropdown-trigger${active === "developer" ? " active" : ""}`}
                />
              }
            >
              <span>{t("developer.label")}</span>
              <ChevronDown aria-hidden="true" />
            </DropdownMenuTrigger>
            <DropdownMenuContent
              side="bottom"
              align="center"
              sideOffset={8}
              className="mk-nav-dropdown-content"
            >
              {DEVELOPER_NAV_ITEMS.map((item) => {
                const Icon = item.icon;
                return (
                  <DropdownMenuItem
                    key={item.href}
                    render={<Link href={item.href} className="mk-nav-dropdown-item" />}
                  >
                    <Icon aria-hidden="true" />
                    <span>
                      <strong>{t(item.labelKey)}</strong>
                      <small>{t(item.descriptionKey)}</small>
                    </span>
                  </DropdownMenuItem>
                );
              })}
            </DropdownMenuContent>
          </DropdownMenu>
          <LanguageSelector />
        </div>
        <MarketingNav />
      </div>
    </nav>
  );
}

export function MarketingCTA({
  className = "lp-btn lp-btn-primary lp-btn-lg",
  label,
  showArrow = false,
}: {
  className?: string;
  label?: string;
  showArrow?: boolean;
} = {}) {
  const { isSignedIn, isLoaded } = useAuth();
  const { appUrl, signUpRedirectUrl } = useLandingRedirectUrls();
  const t = useTranslations("common");
  const resolvedLabel = label ?? t("actions.getStartedFree");

  if (!isLoaded) return <div style={{ height: 48 }} />;

  if (isSignedIn) {
    return (
      <a href={appUrl} className={className}>
        {t("actions.goToDashboard")}
        {showArrow ? <ArrowRight size={17} aria-hidden="true" /> : null}
      </a>
    );
  }

  return (
    <SignUpButton mode="redirect" forceRedirectUrl={signUpRedirectUrl}>
      <button className={className} style={{ cursor: "pointer" }}>
        {resolvedLabel}
        {showArrow ? <ArrowRight size={17} aria-hidden="true" /> : null}
      </button>
    </SignUpButton>
  );
}

export function MarketingCTALight() {
  const { isSignedIn, isLoaded } = useAuth();
  const { appUrl, signUpRedirectUrl } = useLandingRedirectUrls();
  const t = useTranslations("common");

  if (!isLoaded) return <div style={{ height: 48 }} />;

  if (isSignedIn) {
    return (
      <a href={appUrl} className="lp-btn lp-btn-outline lp-btn-lg">
        {t("actions.goToDashboard")}
      </a>
    );
  }

  return (
    <SignUpButton mode="redirect" forceRedirectUrl={signUpRedirectUrl}>
      <button className="lp-btn lp-btn-outline lp-btn-lg" style={{ cursor: "pointer" }}>
        {t("actions.signUpFree")}
      </button>
    </SignUpButton>
  );
}

/* Pricing page variants — same logic, pr- prefix classes */
export function PricingNav() {
  return <MarketingNav />;
}

export function PricingCTA({ className = "pr-btn-free", label, href }: { className?: string; label?: string; href?: string }) {
  const { isSignedIn, isLoaded } = useAuth();
  const { appUrl, signUpRedirectUrl } = useLandingRedirectUrls();
  const t = useTranslations("common");

  if (!isLoaded) return <div style={{ height: 44 }} />;

  // If explicit label + href provided, use them directly
  if (label && href) {
    return (
      <a href={href} className={`pr-btn ${className}`}>
        {label}
      </a>
    );
  }

  if (isSignedIn) {
    return (
      <a href={appUrl} className={`pr-btn ${className}`}>
        {className.includes("paid") ? t("actions.upgrade") : t("actions.goToDashboard")}
      </a>
    );
  }

  return (
    <SignUpButton mode="redirect" forceRedirectUrl={signUpRedirectUrl}>
      <button className={`pr-btn ${className}`} style={{ cursor: "pointer" }}>
        {className.includes("paid") ? t("actions.getStarted") : t("actions.getStartedFree")}
      </button>
    </SignUpButton>
  );
}
