"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { useAuth, SignInButton, SignUpButton, UserButton } from "@clerk/nextjs";
import { UniPostLogo } from "@/components/brand/unipost-logo";
import { LandingAttribution } from "@/components/marketing/landing-attribution";
import { appendLandingSessionId } from "@/lib/landing-attribution";
import { ThemeToggle } from "@/components/theme-toggle";

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
  { href: "/solutions", label: "Solutions", key: "solutions" },
  { href: "/tools", label: "Tools", key: "tools" },
  { href: "/pricing", label: "Pricing", key: "pricing" },
  { href: "/docs", label: "Docs", key: "docs" },
] as const;

type PublicNavKey = (typeof PUBLIC_NAV_ITEMS)[number]["key"];

function useLandingRedirectUrls() {
  const [urls, setUrls] = useState({
    appUrl: APP_URL,
    signUpRedirectUrl: SIGN_UP_REDIRECT_URL,
  });

  useEffect(() => {
    setUrls({
      appUrl: appendLandingSessionId(APP_URL),
      signUpRedirectUrl: appendLandingSessionId(SIGN_UP_REDIRECT_URL),
    });
  }, []);

  return urls;
}

export function MarketingNav() {
  const { isSignedIn, isLoaded } = useAuth();
  const { appUrl, signUpRedirectUrl } = useLandingRedirectUrls();

  let controls: ReactNode;

  if (!isLoaded) {
    controls = <div style={{ display: "flex", alignItems: "center", gap: 8, height: 36 }} />;
  } else if (isSignedIn) {
    controls = (
      <div className="mk-auth-controls">
        <ThemeToggle />
        <a href={appUrl} className="mk-auth-btn mk-auth-btn-primary">
          Go to Dashboard
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
            Sign in
          </button>
        </SignInButton>
        <SignUpButton mode="redirect" forceRedirectUrl={signUpRedirectUrl}>
          <button className="mk-auth-btn mk-auth-btn-primary">
            Get Started Free
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
  return (
    <nav className="mk-nav">
      <div className="mk-nav-inner">
        <div className="mk-nav-left">
          <Link href="/" className="mk-logo">
            <UniPostLogo markSize={28} wordmarkColor="var(--marketing-text)" />
          </Link>
        </div>
        <div className="mk-nav-links">
          {PUBLIC_NAV_ITEMS.map((item) => (
            <Link
              key={item.key}
              href={item.href}
              className={`mk-nav-link${active === item.key ? " active" : ""}`}
            >
              {item.label}
            </Link>
          ))}
        </div>
        <MarketingNav />
      </div>
    </nav>
  );
}

export function MarketingCTA({ className = "lp-btn lp-btn-primary lp-btn-lg" }: { className?: string } = {}) {
  const { isSignedIn, isLoaded } = useAuth();
  const { appUrl, signUpRedirectUrl } = useLandingRedirectUrls();

  if (!isLoaded) return <div style={{ height: 48 }} />;

  if (isSignedIn) {
    return (
      <a href={appUrl} className={className}>
        Go to Dashboard
      </a>
    );
  }

  return (
    <SignUpButton mode="redirect" forceRedirectUrl={signUpRedirectUrl}>
      <button className={className} style={{ cursor: "pointer" }}>
        Get Started Free
      </button>
    </SignUpButton>
  );
}

export function MarketingCTALight() {
  const { isSignedIn, isLoaded } = useAuth();
  const { appUrl, signUpRedirectUrl } = useLandingRedirectUrls();

  if (!isLoaded) return <div style={{ height: 48 }} />;

  if (isSignedIn) {
    return (
      <a href={appUrl} className="lp-btn lp-btn-outline lp-btn-lg">
        Go to Dashboard
      </a>
    );
  }

  return (
    <SignUpButton mode="redirect" forceRedirectUrl={signUpRedirectUrl}>
      <button className="lp-btn lp-btn-outline lp-btn-lg" style={{ cursor: "pointer" }}>
        Sign Up Free
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
        {className.includes("paid") ? "Upgrade" : "Go to Dashboard"}
      </a>
    );
  }

  return (
    <SignUpButton mode="redirect" forceRedirectUrl={signUpRedirectUrl}>
      <button className={`pr-btn ${className}`} style={{ cursor: "pointer" }}>
        {className.includes("paid") ? "Get Started" : "Get Started Free"}
      </button>
    </SignUpButton>
  );
}
