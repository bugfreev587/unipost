"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { FolderOpen, RefreshCw } from "lucide-react";
import { getBootstrap } from "@/lib/api";

const AUTH_LOAD_TIMEOUT_MS = 4500;
const BOOTSTRAP_TIMEOUT_MS = 8000;

function withTimeout<T>(promise: Promise<T>, timeoutMs: number, label: string): Promise<T> {
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(() => {
      reject(new Error(`${label} timed out`));
    }, timeoutMs);

    promise.then(
      (value) => {
        window.clearTimeout(timer);
        resolve(value);
      },
      (error) => {
        window.clearTimeout(timer);
        reject(error);
      },
    );
  });
}

// Dashboard root resolver. Routes the user to the project they were
// last working on (or their auto-created Default if it's their first
// login). Falls back to /projects when bootstrap returns nothing —
// the only realistic cause is "Clerk webhook hasn't synced this user
// yet", which is self-healing on the next page load.
//
// We render this as a client-side redirect rather than a Server
// Component because the dashboard talks to the Go API with a Clerk
// session token sourced from `useAuth().getToken()` — replicating
// that on the server would mean threading Clerk's server-side helpers
// for one redirect, which isn't worth it.
export default function DashboardRootPage() {
  const router = useRouter();
  const { getToken, isLoaded, isSignedIn } = useAuth();
  const [authTimedOut, setAuthTimedOut] = useState(false);
  const [bootstrapTimedOut, setBootstrapTimedOut] = useState(false);
  const [retryCount, setRetryCount] = useState(0);

  useEffect(() => {
    if (isLoaded) {
      setAuthTimedOut(false);
      return;
    }

    const timer = window.setTimeout(() => {
      setAuthTimedOut(true);
    }, AUTH_LOAD_TIMEOUT_MS);

    return () => window.clearTimeout(timer);
  }, [isLoaded, retryCount]);

  useEffect(() => {
    if (!isLoaded) return;

    setAuthTimedOut(false);
    setBootstrapTimedOut(false);

    if (!isSignedIn) {
      setAuthTimedOut(true);
      return;
    }

    let cancelled = false;
    (async () => {
      try {
        const token = await withTimeout(getToken(), BOOTSTRAP_TIMEOUT_MS, "Clerk token");
        if (cancelled) return;
        if (!token) {
          router.replace("/projects");
          return;
        }
        const res = await withTimeout(getBootstrap(token), BOOTSTRAP_TIMEOUT_MS, "Dashboard bootstrap");
        if (cancelled) return;
        // Intent-collection redesign: onboarding is no longer a blocking
        // step. The Welcome modal on the dashboard handles intent collection
        // non-blockingly. Always route straight to the user's project.
        const target = res.data.last_profile_id ?? res.data.default_profile_id;
        if (target) {
          router.replace(`/projects/${target}`);
        } else {
          router.replace("/projects");
        }
      } catch (error) {
        if (cancelled) return;
        if (error instanceof Error && error.message.includes("timed out")) {
          setBootstrapTimedOut(true);
          return;
        }
        router.replace("/projects");
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [isLoaded, isSignedIn, getToken, router, retryCount]);

  if (authTimedOut || bootstrapTimedOut) {
    return (
      <div
        aria-live="polite"
        style={{
          maxWidth: 520,
          border: "1px solid var(--dborder)",
          borderRadius: 8,
          background: "var(--surface)",
          padding: 18,
          color: "var(--dtext)",
        }}
      >
        <div style={{ fontSize: 15, fontWeight: 650, marginBottom: 6 }}>Dashboard is taking longer than expected</div>
        <div style={{ color: "var(--dmuted)", fontSize: 13, lineHeight: "20px", marginBottom: 14 }}>
          Your session is still protected, but the dashboard could not finish loading this route.
        </div>
        <div style={{ display: "flex", flexWrap: "wrap", gap: 10 }}>
          <button
            type="button"
            className="dbtn"
            onClick={() => {
              setAuthTimedOut(false);
              setBootstrapTimedOut(false);
              setRetryCount((count) => count + 1);
            }}
          >
            <RefreshCw style={{ width: 13, height: 13 }} /> Retry loading dashboard
          </button>
          <Link href="/projects" className="dbtn dbtn-primary">
            <FolderOpen style={{ width: 13, height: 13 }} /> Open profiles
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div style={{ color: "var(--dmuted)", fontSize: 14, lineHeight: "20px" }}>Loading...</div>
  );
}
