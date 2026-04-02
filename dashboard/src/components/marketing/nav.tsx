"use client";

import { useAuth, SignInButton, SignUpButton } from "@clerk/nextjs";

const APP_URL = process.env.NEXT_PUBLIC_APP_URL || "https://app.unipost.dev";

export function MarketingNav() {
  const { isSignedIn, isLoaded } = useAuth();

  if (!isLoaded) {
    return <div className="flex items-center gap-4 h-9" />;
  }

  if (isSignedIn) {
    return (
      <a
        href={APP_URL}
        className="inline-flex items-center justify-center rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-800 transition-colors"
      >
        Go to Dashboard
      </a>
    );
  }

  return (
    <div className="flex items-center gap-4">
      <SignInButton mode="redirect" fallbackRedirectUrl={APP_URL}>
        <button className="text-sm font-medium text-zinc-600 hover:text-zinc-900 transition-colors cursor-pointer">
          Log in
        </button>
      </SignInButton>
      <SignUpButton mode="redirect" fallbackRedirectUrl={APP_URL}>
        <button className="inline-flex items-center justify-center rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-800 transition-colors cursor-pointer">
          Get Started
        </button>
      </SignUpButton>
    </div>
  );
}

export function MarketingCTA() {
  const { isSignedIn, isLoaded } = useAuth();

  if (!isLoaded) {
    return <div className="h-12" />;
  }

  if (isSignedIn) {
    return (
      <a
        href={APP_URL}
        className="inline-flex items-center justify-center rounded-md bg-zinc-900 px-8 py-3 text-base font-medium text-white hover:bg-zinc-800 transition-colors"
      >
        Go to Dashboard
      </a>
    );
  }

  return (
    <SignUpButton mode="redirect" fallbackRedirectUrl={APP_URL}>
      <button className="inline-flex items-center justify-center rounded-md bg-zinc-900 px-8 py-3 text-base font-medium text-white hover:bg-zinc-800 transition-colors cursor-pointer">
        Get Started Free
      </button>
    </SignUpButton>
  );
}

export function MarketingCTALight() {
  const { isSignedIn, isLoaded } = useAuth();

  if (!isLoaded) {
    return <div className="h-12" />;
  }

  if (isSignedIn) {
    return (
      <a
        href={APP_URL}
        className="inline-flex items-center justify-center rounded-md bg-white px-8 py-3 text-base font-medium text-zinc-900 hover:bg-zinc-100 transition-colors"
      >
        Go to Dashboard
      </a>
    );
  }

  return (
    <SignUpButton mode="redirect" fallbackRedirectUrl={APP_URL}>
      <button className="inline-flex items-center justify-center rounded-md bg-white px-8 py-3 text-base font-medium text-zinc-900 hover:bg-zinc-100 transition-colors cursor-pointer">
        Sign Up Free
      </button>
    </SignUpButton>
  );
}
