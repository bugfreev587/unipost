import { clerkMiddleware } from "@clerk/nextjs/server";
import { NextResponse } from "next/server";

const APP_HOST = process.env.NEXT_PUBLIC_APP_HOST || "app.unipost.dev";
const COUNTRY_COOKIE = "unipost_country";

function countryCodeFromHeaders(headers: Headers) {
  for (const header of [
    "x-vercel-ip-country",
    "cf-ipcountry",
    "cloudfront-viewer-country",
    "x-country-code",
  ]) {
    const value = (headers.get(header) || "").trim().toUpperCase();
    if (/^[A-Z]{2}$/.test(value) && value !== "XX" && value !== "T1") {
      return value;
    }
  }
  return "";
}

function withCountryCookie(response: NextResponse, request: Request) {
  const country = countryCodeFromHeaders(request.headers);
  if (!country) return response;
  response.cookies.set(COUNTRY_COOKIE, country, {
    path: "/",
    sameSite: "lax",
    maxAge: 60 * 60 * 24 * 30,
  });
  return response;
}

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
    pathname === "/sitemap.xml" ||
    pathname === "/robots.txt" ||
    pathname.startsWith("/docs") ||
    pathname.startsWith("/preview") ||
    pathname.startsWith("/review/") ||
    pathname === "/pricing" ||
    pathname === "/about" ||
    pathname === "/changelog" ||
    pathname.startsWith("/blog") ||
    pathname === "/solutions" ||
    pathname === "/compare" ||
    pathname.startsWith("/alternatives") ||
    pathname === "/contact" ||
    pathname.startsWith("/tools") ||
    // Hosted white-label Connect pages. End users arrive here from the
    // customer's own product — they don't have UniPost accounts and
    // shouldn't be asked for one. The page authenticates via the
    // `session=<id>&state=<oauth_state>` pair in the URL (verified
    // server-side against /v1/public/connect/sessions).
    pathname.startsWith("/connect") ||
    // Customer-domain app review recording surface. The local review
    // agent authenticates it with a short-lived review-session cookie.
    pathname.startsWith("/tiktok/posting") ||
    pathname.endsWith("-api"); // platform landing pages: /twitter-api, /instagram-api, etc.

  const isPublicDocsApi =
    pathname === "/api/docs/answer" ||
    pathname === "/api/docs/feedback";

  if (isPublicPage || isPublicDocsApi) {
    return withCountryCookie(NextResponse.next(), request);
  }

  if (!isDashboard) {
    // Landing page domain (unipost.dev) — rewrite to /marketing
    if (pathname === "/") {
      const url = request.nextUrl.clone();
      url.pathname = "/marketing";
      return withCountryCookie(NextResponse.rewrite(url), request);
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
