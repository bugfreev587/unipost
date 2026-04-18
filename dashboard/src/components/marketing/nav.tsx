"use client";

import { useAuth, SignInButton, SignUpButton, UserButton } from "@clerk/nextjs";
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

const authRowStyle = { display: "flex", alignItems: "center", gap: 8 } as const;

const authGhostButtonStyle = {
  cursor: "pointer",
  display: "inline-flex",
  alignItems: "center",
  justifyContent: "center",
  padding: "8px 14px",
  borderRadius: "999px",
  border: "1px solid rgba(255,255,255,.08)",
  background: "transparent",
  color: "#a6a6a0",
  fontSize: "13px",
  fontWeight: 600,
  lineHeight: 1,
  transition: "all .14s",
} as const;

const authPrimaryButtonStyle = {
  cursor: "pointer",
  display: "inline-flex",
  alignItems: "center",
  justifyContent: "center",
  padding: "8px 14px",
  borderRadius: "999px",
  border: "1px solid transparent",
  background: "#22c55e",
  color: "#041108",
  fontSize: "13px",
  fontWeight: 600,
  lineHeight: 1,
  textDecoration: "none",
  boxShadow: "0 10px 24px rgba(34,197,94,.22)",
  transition: "all .14s",
} as const;

export function MarketingNav() {
  const { isSignedIn, isLoaded } = useAuth();

  if (!isLoaded) {
    return <div style={{ display: "flex", alignItems: "center", gap: 8, height: 36 }} />;
  }

  if (isSignedIn) {
    return (
      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
        <ThemeToggle />
        <a href={APP_URL} style={authPrimaryButtonStyle}>
          Go to Dashboard
        </a>
        <UserButton appearance={userButtonAppearance} />
      </div>
    );
  }

  return (
    <div style={authRowStyle}>
      <ThemeToggle />
      <SignInButton mode="redirect" forceRedirectUrl={APP_URL}>
        <button style={authGhostButtonStyle}>
          Sign in
        </button>
      </SignInButton>
      <SignUpButton mode="redirect" forceRedirectUrl={SIGN_UP_REDIRECT_URL}>
        <button style={authPrimaryButtonStyle}>
          Get Started Free
        </button>
      </SignUpButton>
    </div>
  );
}

export function MarketingCTA({ className = "lp-btn lp-btn-primary lp-btn-lg" }: { className?: string } = {}) {
  const { isSignedIn, isLoaded } = useAuth();

  if (!isLoaded) return <div style={{ height: 48 }} />;

  if (isSignedIn) {
    return (
      <a href={APP_URL} className={className}>
        Go to Dashboard
      </a>
    );
  }

  return (
    <SignUpButton mode="redirect" forceRedirectUrl={SIGN_UP_REDIRECT_URL}>
      <button className={className} style={{ cursor: "pointer" }}>
        Get Started Free
      </button>
    </SignUpButton>
  );
}

export function MarketingCTALight() {
  const { isSignedIn, isLoaded } = useAuth();

  if (!isLoaded) return <div style={{ height: 48 }} />;

  if (isSignedIn) {
    return (
      <a href={APP_URL} className="lp-btn lp-btn-outline lp-btn-lg">
        Go to Dashboard
      </a>
    );
  }

  return (
    <SignUpButton mode="redirect" forceRedirectUrl={SIGN_UP_REDIRECT_URL}>
      <button className="lp-btn lp-btn-outline lp-btn-lg" style={{ cursor: "pointer" }}>
        Sign Up Free
      </button>
    </SignUpButton>
  );
}

/* Pricing page variants — same logic, pr- prefix classes */
export function PricingNav() {
  const { isSignedIn, isLoaded } = useAuth();

  if (!isLoaded) {
    return <div style={{ display: "flex", alignItems: "center", gap: 8, height: 36 }} />;
  }

  if (isSignedIn) {
    return (
      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
        <a href={APP_URL} className="pr-btn pr-btn-primary">
          Go to Dashboard
        </a>
        <UserButton appearance={userButtonAppearance} />
      </div>
    );
  }

  return (
    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
      <SignInButton mode="redirect" forceRedirectUrl={APP_URL}>
        <button className="pr-btn pr-btn-ghost" style={{ cursor: "pointer" }}>
          Sign in
        </button>
      </SignInButton>
      <SignUpButton mode="redirect" forceRedirectUrl={SIGN_UP_REDIRECT_URL}>
        <button className="pr-btn pr-btn-primary" style={{ cursor: "pointer" }}>
          Get Started Free
        </button>
      </SignUpButton>
    </div>
  );
}

export function PricingCTA({ className = "pr-btn-free", label, href }: { className?: string; label?: string; href?: string }) {
  const { isSignedIn, isLoaded } = useAuth();

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
      <a href={APP_URL} className={`pr-btn ${className}`}>
        {className.includes("paid") ? "Upgrade" : "Go to Dashboard"}
      </a>
    );
  }

  return (
    <SignUpButton mode="redirect" forceRedirectUrl={SIGN_UP_REDIRECT_URL}>
      <button className={`pr-btn ${className}`} style={{ cursor: "pointer" }}>
        {className.includes("paid") ? "Get Started" : "Get Started Free"}
      </button>
    </SignUpButton>
  );
}
