import { clerkMiddleware } from "@clerk/nextjs/server";
import { NextResponse } from "next/server";
import { warnIfFrontendHasClerkSecret } from "@/lib/clerk-env";

const APP_HOST = process.env.NEXT_PUBLIC_APP_HOST || "app.unipost.dev";

warnIfFrontendHasClerkSecret();

export default clerkMiddleware(async (auth, request) => {
  const hostname = request.headers.get("host") || "";
  const { pathname } = request.nextUrl;

  // Determine if this is the dashboard domain (app.unipost.dev)
  const isDashboard =
    hostname === APP_HOST ||
    hostname === "localhost:3000" ||
    hostname.startsWith("localhost:");

  // Public pages (no auth, available on both domains)
  const isPublicPage =
    pathname === "/terms" ||
    pathname === "/privacy" ||
    pathname.startsWith("/docs") ||
    pathname === "/pricing" ||
    pathname === "/solutions" ||
    pathname === "/compare" ||
    pathname.startsWith("/alternatives") ||
    pathname === "/contact" ||
    pathname.startsWith("/tools") ||
    pathname.endsWith("-api"); // platform landing pages: /twitter-api, /instagram-api, etc.
  if (isPublicPage) {
    return NextResponse.next();
  }

  if (!isDashboard) {
    // Landing page domain (unipost.dev) — rewrite to /marketing
    if (pathname === "/") {
      const url = request.nextUrl.clone();
      url.pathname = "/marketing";
      return NextResponse.rewrite(url);
    }
    // Other paths on landing domain → redirect to dashboard domain
    const url = new URL(pathname, `https://${APP_HOST}`);
    return NextResponse.redirect(url);
  }

  // Dashboard domain — block direct access to /marketing
  if (pathname.startsWith("/marketing")) {
    const url = request.nextUrl.clone();
    url.pathname = "/";
    return NextResponse.redirect(url);
  }

  // Dashboard domain — require auth for all routes
  await auth.protect();
});

export const config = {
  matcher: [
    "/((?!_next|[^?]*\\.(?:html?|css|js(?!on)|jpe?g|webp|png|gif|svg|ttf|woff2?|ico|csv|docx?|xlsx?|zip|webmanifest)).*)",
    "/(api|trpc)(.*)",
  ],
};
